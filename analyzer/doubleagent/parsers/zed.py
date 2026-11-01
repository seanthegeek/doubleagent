"""Parse Zed agent panel threads, the sidebar database and the permission settings.

Follows `research/zed.md`. Validated against source, zed-industries/zed at
a84689073d29 and 9cd14daad768 (settings, `db.sqlite` stores, denials, scratch
directories and drafts), with a synthetic fixture; no real install. Paths are
under the data directory (`DATA_DIR`) or the config directory (`CONFIG_DIR`):

- `threads/threads.db`, table `threads(id, summary, updated_at, data_type,
  data, parent_id, folder_paths, created_at)`; `data` is JSON, zstd-compressed
  when `data_type` is `zstd`. The thread JSON (`version` 0.3.0): `title`,
  `updated_at`, `initial_project_snapshot` (`timestamp`,
  `worktree_snapshots[].worktree_path` and `git_state.{current_branch,
  remote_url}`), `model.{provider, model}`, `subagent_context`, `profile`,
  `sandbox_grants`, `sandboxed_terminal_temp_dir`, `draft_prompt` and
  `messages[]`, externally tagged `User`, `Agent` (`content[]`,
  `tool_results`), `Resume` or `Compaction`. Versions 0.1.0 and 0.2.0
  (`role`, `segments[]`, `tool_uses[]`, `tool_results[]`) and the
  unversioned legacy shape are read on a best-effort basis.
- `db/0-<channel>/db.sqlite`: `sidebar_threads`, `trusted_worktrees`, the
  `agent_draft_prompts` and `created_git_worktrees` namespaces of
  `scoped_kv_store`, `archived_git_worktrees` with
  `thread_archived_worktrees`, and `sidebar_terminal_threads`: `system` rows.
- `settings.json` and `global_settings.json` (JSONC): the standing
  permission keys `_settings_entries` lists, in every override block.

Messages carry no time: each row takes the thread's `updated_at`, and the
thread-start, `git remote:` and `model:` rows the snapshot's time. The blob
holds one model per thread, so a switch is not recorded. Denials are read
from failed tool results' texts (`USER_DENIAL`, `AUTO_DENIAL_PATTERN`); an
approval leaves no record. `source_line` is the SQLite `rowid`.
"""

from __future__ import annotations

import io
import json
import re
import sqlite3
import uuid
from collections.abc import Iterator

import zstandard

from ..inputs import Artifact
from ..model import GIT_REMOTE_PREFIX, Row, compact
from ..sqlite_util import open_copy, sqlite_problem, table_names
from ..timeutil import to_utc
from .base import Options, Parser, compact_json, error_rows, file_mtime, setting_value
from .gemini_cli import parse_jsonc

DATA_DIR = r"(?:\.local/share/zed|Library/Application Support/Zed|AppData/Local/Zed|\.var/app/dev\.zed\.Zed[A-Za-z]*/data/zed)"
SANDBOX_FLAGS = ("network_any_host", "allow_fs_write_all", "unsandboxed", "sandbox_fallback")
CONFIG_DIR = r"(?:\.config/zed|AppData/Roaming/Zed|\.var/app/dev\.zed\.Zed[A-Za-z]*/config/zed)"
THREADS_PATTERN = re.compile(r"^" + DATA_DIR + r"/threads/threads\.db$")
SIDEBAR_PATTERN = re.compile(r"^" + DATA_DIR + r"/db/0-[^/]+/db\.sqlite$")
SETTINGS_PATTERN = re.compile(r"^" + CONFIG_DIR + r"/(?:global_)?settings\.json$")
# Release-channel and platform override blocks of a user settings file
# (`ReleaseChannelOverrides`, `PlatformOverrides`), each a whole settings
# object of its own.
SETTINGS_OVERRIDES = ("dev", "nightly", "preview", "stable", "macos", "linux", "windows")
USER_DENIAL = "Permission to run tool denied by user"
# The texts `ToolPermissionDecision::Deny` carries when the rules deny a
# call with no prompt (crates/agent/src/tool_permissions.rs at 9cd14da).
AUTO_DENIAL_PATTERN = re.compile(
    r"(?:Blocked by global default: deny"
    r"|Command blocked by security rule for \S+ tool"
    r"|\S+ tool is disabled"
    r"|Blocked by built-in security rule\..*"
    r"|The terminal command could not be approved because .*"
    r"|The \S+ tool cannot run because \d+ regex patterns? failed to compile\..*"
    r'|The .+ shell does not support "always allow" patterns .*)',
    re.S,
)


def decompress(blob: bytes) -> bytes:
    """Return a zstd frame's content, streaming it when the header records no size."""
    decompressor = zstandard.ZstdDecompressor()
    try:
        return decompressor.decompress(blob)
    except zstandard.ZstdError:
        # No content size in the frame header: stream it instead.
        out = io.BytesIO()
        with decompressor.stream_reader(io.BytesIO(blob)) as reader:
            while True:
                chunk = reader.read(65536)
                if not chunk:
                    break
                out.write(chunk)
        return out.getvalue()


def record_json(value) -> str:
    """Return a decoded blob item as error row text, in full.

    The text is compact JSON in the blob's own key order, non-ASCII kept.
    """
    try:
        return json.dumps(value, ensure_ascii=False, separators=(",", ":"))
    except (TypeError, ValueError):
        return str(value)


def content_text(content) -> str:
    """Return the text of a tool result `content`.

    The content is a list of `{"Text": ...}` or `{"Image": ...}` items, a
    single such item, or a plain string.
    """
    if content is None:
        return ""
    if isinstance(content, str):
        return content
    if isinstance(content, dict):
        content = [content]
    if not isinstance(content, list):
        return compact_json(content)
    parts = []
    for item in content:
        if isinstance(item, str):
            parts.append(item)
        elif isinstance(item, dict):
            if isinstance(item.get("Text"), str):
                parts.append(item["Text"])
            elif "Image" in item:
                parts.append("[image]")
            elif isinstance(item.get("text"), str):
                parts.append(item["text"])
            else:
                parts.append(compact_json(item))
    return "\n".join(parts)


def mention_text(mention) -> str:
    """Return a `Mention` content item as `[mention] <uri>`."""
    if not isinstance(mention, dict):
        return "[mention]"
    uri = mention.get("uri")
    return "[mention] " + (uri if isinstance(uri, str) else compact_json(uri))


def tool_input_text(tool_use: dict) -> str:
    """Return a tool use's `input` as text, else its `raw_input`."""
    tool_input = tool_use.get("input")
    if (
        isinstance(tool_input, dict)
        and set(tool_input) <= {"type", "value"}
        and "value" in tool_input
    ):
        tool_input = tool_input["value"]
    if tool_input not in (None, "", {}):
        return tool_input if isinstance(tool_input, str) else compact_json(tool_input)
    raw = tool_use.get("raw_input")
    return raw if isinstance(raw, str) else ""


def first_folder(folder_paths) -> str:
    """Return the first of the newline-joined `folder_paths`, else ""."""
    if not isinstance(folder_paths, str):
        return ""
    for path in folder_paths.split("\n"):
        if path.strip():
            return path
    return ""


def acp_blocks_text(blocks) -> str:
    """Return the text of a draft prompt's ACP `ContentBlock`s.

    A `text` block gives its text, `resource_link` and `resource` their URI,
    `image` and `audio` a marker; another block is compact JSON.
    """
    if isinstance(blocks, dict):
        blocks = [blocks]
    if not isinstance(blocks, list):
        return compact_json(blocks)
    parts = []
    for block in blocks:
        kind = block.get("type") if isinstance(block, dict) else None
        if isinstance(block, str):
            parts.append(block)
        elif kind == "text" and isinstance(block.get("text"), str):
            parts.append(block["text"])
        elif kind == "resource_link" and isinstance(block.get("uri"), str):
            parts.append("[mention] " + block["uri"])
        elif kind == "resource" and isinstance(block.get("resource"), dict):
            uri = block["resource"].get("uri")
            parts.append("[mention] " + (uri if isinstance(uri, str) else compact_json(uri)))
        elif kind in ("image", "audio"):
            parts.append("[%s]" % kind)
        else:
            parts.append(compact_json(block))
    return "\n".join(parts)


def denial_text(result: dict) -> str:
    """Return the `permission decision:` text of a failed tool result Zed wrote for a denial.

    Any other result gives "": an approval leaves no record, so only denials
    have a row.
    """
    if not result.get("is_error"):
        return ""
    text = content_text(result.get("content")).strip()
    if text == USER_DENIAL:
        return "permission decision: denied"
    if AUTO_DENIAL_PATTERN.fullmatch(text):
        return "permission decision: auto_denied reason=" + text
    return ""


def thread_key(blob) -> str:
    """Return a `sidebar_threads.thread_id` (16 raw UUID bytes) as a hyphenated UUID.

    That is the string the key-value store keys drafts by.
    """
    if isinstance(blob, (bytes, bytearray)) and len(blob) == 16:
        return str(uuid.UUID(bytes=bytes(blob)))
    return str(blob or "")


def flatten_setting(key: str, value) -> Iterator[tuple[str, object]]:
    """Yield (dotted key, value) per leaf of a settings value, one per list entry."""
    if isinstance(value, dict):
        for member, member_value in value.items():
            yield from flatten_setting("%s.%s" % (key, member), member_value)
    elif isinstance(value, list):
        for entry in value:
            yield key, entry
    else:
        yield key, value


class ZedParser(Parser):
    """Parse Zed's thread database, sidebar database and user settings files."""

    agent = "zed"
    name = "zed"

    def wants(self, artifact: Artifact) -> bool:
        """Return True for `threads.db`, a channel's `db.sqlite` or a user settings file."""
        rel = artifact.rel
        return bool(
            THREADS_PATTERN.match(rel) or SIDEBAR_PATTERN.match(rel) or SETTINGS_PATTERN.match(rel)
        )

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Zed file."""
        if SETTINGS_PATTERN.match(artifact.rel):
            yield from self._parse_settings(artifact)
            return
        problem = sqlite_problem(artifact.disk_path)
        if problem:
            yield self._file_error(artifact, problem)
            return
        try:
            if THREADS_PATTERN.match(artifact.rel):
                yield from self._parse_threads(artifact, opts)
            else:
                yield from self._parse_sidebar(artifact)
        except sqlite3.DatabaseError as error:
            yield self._file_error(artifact, "SQLite error: %s" % error)

    def _file_error(self, artifact: Artifact, message: str) -> Row:
        """Return the one `parser: <message>` row for a file that cannot be read."""
        row = self.base_row(artifact)
        row.turn_type = "system"
        row.text = "parser: " + message
        return row

    def _state_row(
        self,
        artifact: Artifact,
        text: str,
        timestamp: str,
        source_line: int,
        session_id: str = "",
        project: str = "",
    ) -> Row:
        """Return a `system` row of `artifact` with the given text and context."""
        row = self.base_row(artifact)
        row.source_line = source_line
        row.session_id = session_id
        row.project_path = project
        row.timestamp_utc = timestamp
        row.turn_type = "system"
        row.text = compact(text)
        return row

    # -- settings.json, global_settings.json ----------------------------------------
    def _parse_settings(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `permission setting:` row per standing permission, at the file's mtime."""
        try:
            raw = artifact.disk_path.read_bytes()
        except OSError as error:
            yield self._file_error(artifact, "unreadable: %s" % error)
            return
        if not raw.strip():
            return
        try:
            doc = parse_jsonc(raw)
        except ValueError:
            doc = None
        if not isinstance(doc, dict):
            yield self._file_error(artifact, "%s is not a JSON object" % artifact.rel)
            return
        timestamp = file_mtime(artifact)
        for key, value in self._settings_entries(doc):
            text = "permission setting: %s=%s (from %s, file mtime)" % (
                key,
                setting_value(value),
                artifact.rel,
            )
            yield self._state_row(artifact, text, timestamp, 1)

    @staticmethod
    def _settings_entries(doc: dict) -> Iterator[tuple[str, object]]:
        """Yield (dotted key, value) per standing permission entry of a settings object.

        The entries are those of `agent.tool_permissions`,
        `agent.sandbox_permissions`, the legacy `agent.always_allow_tool_actions`,
        `agent_servers.<id>.default_mode` and `session.trust_all_worktrees`, in
        the top level, each release-channel and platform override block and
        each `profiles.<name>.settings`, the key prefixed with the block's.
        """
        layers: list[tuple[str, dict]] = [("", doc)]
        for name in SETTINGS_OVERRIDES:
            if isinstance(doc.get(name), dict):
                layers.append((name + ".", doc[name]))
        profiles = doc.get("profiles")
        if isinstance(profiles, dict):
            for name, profile in profiles.items():
                if isinstance(profile, dict) and isinstance(profile.get("settings"), dict):
                    layers.append(("profiles.%s.settings." % name, profile["settings"]))
        for prefix, block in layers:
            agent = block.get("agent")
            if isinstance(agent, dict):
                for section_name in ("tool_permissions", "sandbox_permissions"):
                    if section_name in agent:
                        yield from flatten_setting(
                            prefix + "agent." + section_name, agent[section_name]
                        )
                if "always_allow_tool_actions" in agent:
                    legacy = agent["always_allow_tool_actions"]
                    yield prefix + "agent.always_allow_tool_actions", legacy
            servers = block.get("agent_servers")
            if isinstance(servers, dict):
                for session_id, config in servers.items():
                    if isinstance(config, dict) and "default_mode" in config:
                        yield (
                            prefix + "agent_servers.%s.default_mode" % session_id,
                            config["default_mode"],
                        )
            session = block.get("session")
            if isinstance(session, dict) and "trust_all_worktrees" in session:
                yield prefix + "session.trust_all_worktrees", session["trust_all_worktrees"]

    # -- db/0-<channel>/db.sqlite -------------------------------------------------
    def _parse_sidebar(self, artifact: Artifact) -> Iterator[Row]:
        errors: list = []
        with open_copy(artifact.disk_path) as connection:
            tables = table_names(connection)
            sidebar = self._select(
                connection,
                tables,
                "sidebar_threads",
                "select rowid as _rowid, * from sidebar_threads order by updated_at",
            )
            trusted = self._select(
                connection,
                tables,
                "trusted_worktrees",
                "select rowid as _rowid, * from trusted_worktrees order by rowid",
            )
            key_values = self._select(
                connection,
                tables,
                "scoped_kv_store",
                "select rowid as _rowid, namespace, key, value"
                " from scoped_kv_store where namespace in ('agent_draft_prompts',"
                " 'created_git_worktrees') order by rowid",
            )
            link = "thread_archived_worktrees" in tables
            archived = self._select(
                connection,
                tables,
                "archived_git_worktrees",
                "select a.rowid as _rowid, a.*, t.thread_id as _thread from"
                " archived_git_worktrees a left join thread_archived_worktrees t"
                " on t.archived_worktree_id = a.id order by a.id"
                if link
                else "select rowid as _rowid, *, null as _thread from archived_git_worktrees"
                " order by id",
            )
            terminals = self._select(
                connection,
                tables,
                "sidebar_terminal_threads",
                "select rowid as _rowid, * from sidebar_terminal_threads order by created_at",
            )
        by_key = {thread_key(row.get("thread_id")): row for row in sidebar}
        for row in sidebar:
            yield from self._sidebar_row(artifact, row)
        for row in trusted:
            yield self._trusted_row(artifact, row)
        for row in key_values:
            if row["namespace"] == "agent_draft_prompts":
                yield from self._draft(artifact, row, by_key, errors)
            else:
                yield from self._created_worktree(artifact, row, sidebar, archived, errors)
        for row in archived:
            yield self._archived_worktree(artifact, row, by_key)
        for row in terminals:
            title = row.get("custom_title") or row.get("title") or ""
            text = "sidebar terminal: %s cwd=%s" % (title, row.get("working_directory") or "-")
            project = first_folder(row.get("folder_paths"))
            timestamp = to_utc(row.get("created_at"))
            yield self._state_row(artifact, text, timestamp, row["_rowid"], "", project)
        yield from error_rows(self, artifact, errors)

    @staticmethod
    def _select(connection: sqlite3.Connection, tables: set, table: str, sql: str) -> list[dict]:
        """Return the rows of `sql` as dicts, or [] when `table` does not exist."""
        if table not in tables:
            return []
        return [dict(row) for row in connection.execute(sql)]

    def _sidebar_row(self, artifact: Artifact, row: dict) -> Iterator[Row]:
        """Yield an external agent's thread or a native thread's sidebar state.

        An external agent's thread has no transcript in `threads.db`; a draft
        (no `session_id`) has no row of its own, only its draft prompt.
        """
        title = row.get("title_override") or row.get("title") or ""
        project = first_folder(row.get("folder_paths"))
        session_id = str(row.get("session_id") or "")
        if row.get("agent_id"):
            text = "external agent thread (no transcript in threads.db): agent=%s title=%s%s" % (
                row["agent_id"],
                title,
                " archived" if row.get("archived") else "",
            )
            timestamp = to_utc(row.get("interacted_at") or row.get("updated_at"))
            yield self._state_row(artifact, text, timestamp, row["_rowid"], session_id, project)
        elif session_id:
            text = "sidebar thread: title=%s archived=%s interacted_at=%s main_worktree=%s" % (
                title,
                "true" if row.get("archived") else "false",
                to_utc(row.get("interacted_at")) or "-",
                first_folder(row.get("main_worktree_paths")) or "-",
            )
            timestamp = to_utc(row.get("updated_at"))
            yield self._state_row(artifact, text, timestamp, row["_rowid"], session_id, project)

    def _trusted_row(self, artifact: Artifact, row: dict) -> Row:
        """Return the `permission setting:` row of one trusted project path."""
        path = str(row.get("absolute_path") or "")
        host = str(row.get("host_name") or "")
        if host and row.get("user_name"):
            host = "%s@%s" % (row["user_name"], host)
        text = "permission setting: trusted_worktrees.%s=true%s (from %s trusted_worktrees," % (
            path,
            " host=%s" % host if host else "",
            artifact.rel,
        )
        timestamp = file_mtime(artifact)
        return self._state_row(artifact, text + " file mtime)", timestamp, row["_rowid"], "", path)

    def _draft(self, artifact: Artifact, row: dict, by_key: dict, errors: list) -> Iterator[Row]:
        """Yield `unsent draft: <text>` for an `agent_draft_prompts` entry.

        The row is at the draft's sidebar `updated_at`, or at the file's
        modification time, saying so, when the draft has no sidebar row.
        """
        rowid, key, value = row["_rowid"], str(row["key"]), row["value"]
        try:
            blocks = json.loads(value)
        except (TypeError, ValueError) as error:
            label = "unparseable record %d (agent_draft_prompts %s: %s: %s)" % (
                rowid,
                key,
                type(error).__name__,
                error,
            )
            errors.append((rowid, label, str(value)))
            return
        text = acp_blocks_text(blocks)
        if not text.strip():
            return
        side = by_key.get(key)
        if side is not None:
            timestamp = to_utc(side.get("updated_at"))
            project = first_folder(side.get("folder_paths"))
            yield self._state_row(artifact, "unsent draft: " + text, timestamp, rowid, "", project)
            return
        text = "unsent draft: %s (from %s agent_draft_prompts, file mtime)" % (text, artifact.rel)
        yield self._state_row(artifact, text, file_mtime(artifact), rowid)

    def _created_worktree(
        self, artifact: Artifact, row: dict, sidebar: list, archived: list, errors: list
    ) -> Iterator[Row]:
        """Yield `scratch directory:` for a `created_git_worktrees` entry, at its creation time.

        `project_path` is the worktree; the main repository comes from the
        sidebar or the archived worktrees.
        """
        rowid, value = row["_rowid"], row["value"]
        host, _, path = str(row["key"]).partition("\n")
        try:
            record = json.loads(value)
            seconds = record["created_at_seconds"] + record.get("created_at_subsec_nanos", 0) / 1e9
        except (TypeError, ValueError, KeyError, AttributeError) as error:
            label = "unparseable record %d (created_git_worktrees: %s: %s)" % (
                rowid,
                type(error).__name__,
                error,
            )
            errors.append((rowid, label, str(value)))
            return
        main = next(
            (
                first_folder(entry.get("main_worktree_paths"))
                for entry in sidebar
                if entry.get("main_worktree_paths")
                and path in str(entry.get("folder_paths") or "").split("\n")
            ),
            "",
        ) or next(
            (
                str(archive["main_repo_path"])
                for archive in archived
                if archive["worktree_path"] == path
            ),
            "",
        )
        text = "scratch directory: %s (Zed-created git worktree%s%s) (from %s" % (
            path,
            " of %s" % main if main else "",
            " host=%s" % host if host and host != "local" else "",
            artifact.rel,
        )
        text += " created_git_worktrees, created_at)"
        yield self._state_row(artifact, text, to_utc(seconds), rowid, "", path)

    def _archived_worktree(self, artifact: Artifact, row: dict, by_key: dict) -> Row:
        """Return the row of one archived worktree and one thread linked to it."""
        side = by_key.get(thread_key(row.get("_thread"))) if row.get("_thread") else None
        session_id = str((side or {}).get("session_id") or "")
        text = (
            "archived worktree: %s of %s branch=%s staged=%s unstaged=%s original=%s"
            " ref=refs/archived-worktrees/%s (from %s archived_git_worktrees, file mtime)"
        ) % (
            row.get("worktree_path") or "",
            row.get("main_repo_path") or "",
            row.get("branch_name") or "-",
            row.get("staged_commit_hash") or "-",
            row.get("unstaged_commit_hash") or "-",
            row.get("original_commit_hash") or "-",
            row.get("id"),
            artifact.rel,
        )
        project = str(row.get("main_repo_path") or "")
        timestamp = file_mtime(artifact)
        return self._state_row(artifact, text, timestamp, row["_rowid"], session_id, project)

    # -- threads/threads.db -------------------------------------------------------
    def _parse_threads(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        with open_copy(artifact.disk_path) as connection:
            if "threads" not in table_names(connection):
                return
            for row in connection.execute(
                "select rowid as _rowid, * from threads order by updated_at"
            ):
                yield from self._thread_rows(artifact, dict(row), opts)

    def _thread_rows(self, artifact: Artifact, record: dict, opts: Options) -> Iterator[Row]:
        session_id = str(record.get("id") or "")
        project = first_folder(record.get("folder_paths"))
        updated = to_utc(record.get("updated_at"))

        def base(
            turn_type: str, text: str, timestamp: str = "", model: str = "", branch: str = ""
        ) -> Row:
            row = self.base_row(artifact)
            row.source_line = record["_rowid"]
            row.session_id = session_id
            row.project_path = project
            row.git_branch = branch
            row.timestamp_utc = timestamp or updated
            row.model = model
            row.turn_type = turn_type
            row.text = compact(text)
            return row

        thread, error, raw = self._decode(record)
        if thread is None:
            label = "unparseable record %d (%s)" % (record["_rowid"], error)
            yield from error_rows(
                self, artifact, [(record["_rowid"], label, raw)], session_id, project
            )
            return

        snapshot = thread.get("initial_project_snapshot")
        if not isinstance(snapshot, dict):
            snapshot = {}
        worktrees = [
            worktree
            for worktree in (snapshot.get("worktree_snapshots") or [])
            if isinstance(worktree, dict)
        ]
        if not project and worktrees:
            project = str(worktrees[0].get("worktree_path") or "")
        branches = [
            (worktree.get("worktree_path") == project, str(worktree["git_state"]["current_branch"]))
            for worktree in worktrees
            if isinstance(worktree.get("git_state"), dict)
            and worktree["git_state"].get("current_branch")
        ]
        branch = max(branches, key=lambda pair: pair[0])[1] if branches else ""
        # The project's own snapshot, else the first one; never another
        # worktree's, whose remote is another repository's.
        own = [
            worktree for worktree in worktrees if worktree.get("worktree_path") == project
        ] or worktrees[:1]
        remote = ""
        if own and isinstance(own[0].get("git_state"), dict):
            url = own[0]["git_state"].get("remote_url")
            remote = url.strip() if isinstance(url, str) else ""
        model_info = thread.get("model")
        if not isinstance(model_info, dict):
            model_info = {}
        model = "/".join(
            str(part) for part in (model_info.get("provider"), model_info.get("model")) if part
        )
        if not updated:
            updated = to_utc(thread.get("updated_at"))

        parent = record.get("parent_id") or (
            (thread.get("subagent_context") or {}).get("parent_thread_id")
            if isinstance(thread.get("subagent_context"), dict)
            else ""
        )
        start_timestamp = (
            to_utc(snapshot.get("timestamp")) or to_utc(record.get("created_at")) or updated
        )
        yield base(
            "system",
            " ".join(
                part
                for part in (
                    "thread start:",
                    str(thread.get("title") or record.get("summary") or ""),
                    "version=%s" % thread["version"] if thread.get("version") else "",
                    "parent=%s" % parent if parent else "",
                )
                if part
            ),
            start_timestamp,
            model,
            branch,
        )
        if remote:
            yield base("system", GIT_REMOTE_PREFIX + remote, start_timestamp, model, branch)
        if model_info.get("model"):
            text = "model: %s" % model_info["model"]
            if model_info.get("provider"):
                text += " provider=%s" % model_info["provider"]
            yield base("system", text, start_timestamp, model, branch)
        for key, value in self._grants(thread):
            yield base(
                "system",
                "permission setting: %s=%s (thread blob, as of updated_at)"
                % (key, setting_value(value)),
                "",
                model,
                branch,
            )

        temp_dir = thread.get("sandboxed_terminal_temp_dir")
        if isinstance(temp_dir, str) and temp_dir:
            # Its own project_path and no session_id, so the thread stays
            # counted under its project.
            text = (
                "scratch directory: %s (sandboxed terminal temp directory of thread %s)"
                " (thread blob, as of updated_at)" % (temp_dir, session_id)
            )
            yield self._state_row(artifact, text, updated, record["_rowid"], "", temp_dir)

        errors: list = []
        for kind, text, tool_name, tool_id in self._messages(thread, record["_rowid"], errors):
            if kind == "thinking" and not opts.include_thinking:
                continue
            if not text and kind not in ("tool_use", "tool_result"):
                continue
            row = base(kind, text, "", model, branch)
            row.tool_name = tool_name
            row.tool_use_id = tool_id
            yield row
        draft = thread.get("draft_prompt")
        if draft:
            text = acp_blocks_text(draft)
            if text.strip():
                text = "unsent draft: %s (thread blob, as of updated_at)" % text
                yield base("system", text, "", model, branch)
        yield from error_rows(self, artifact, errors, session_id, project)

    @staticmethod
    def _grants(thread: dict) -> Iterator[tuple[str, object]]:
        """Yield (dotted key, value) per thread-scoped grant.

        The grants are the agent profile, each granted write path and network
        host, and each sandbox escape flag that is set (`DbSandboxGrants`,
        db.rs:91-122 at a84689073d29).
        """
        if thread.get("profile") not in (None, ""):
            yield "profile", thread["profile"]
        grants = thread.get("sandbox_grants")
        if not isinstance(grants, dict):
            return
        for key in ("write_paths", "network_hosts"):
            entries = grants.get(key)
            if isinstance(entries, list):
                for entry in entries:
                    yield "sandbox_grants.%s" % key, entry
        for key in SANDBOX_FLAGS:
            if grants.get(key) is True:
                yield "sandbox_grants.%s" % key, True

    @staticmethod
    def _decode(row: dict) -> tuple[dict | None, str, str]:
        """Return (thread, "", "") for a blob holding a JSON object, else (None, reason, raw).

        A `json` row's raw value is its text, decoded UTF-8 with
        `errors="replace"`; a `zstd` row's is its compressed bytes as
        lowercase hex, since they have no faithful text form and a truncated
        frame can decompress to nothing, and its reason ends `stored zstd
        blob as hex`.
        """
        data = row.get("data")
        if data is None:
            return None, "no data", ""
        binary = isinstance(data, (bytes, bytearray))
        stored = bytes(data) if binary else str(data).encode("utf-8")
        is_zstd = str(row.get("data_type") or "").lower() == "zstd"
        try:
            raw = decompress(stored) if is_zstd else stored
            thread = json.loads(raw.decode("utf-8", errors="replace"))
            if not isinstance(thread, dict):
                raise ValueError("not a JSON object")
        except Exception as error:  # noqa: BLE001 - ZstdError, the stream reader's errors, ValueError
            if is_zstd:
                reason = "%s: %s; stored zstd blob as hex" % (type(error).__name__, error)
                return None, reason, stored.hex()
            return (
                None,
                "%s: %s" % (type(error).__name__, error),
                stored.decode("utf-8", errors="replace"),
            )
        return thread, "", ""

    def _messages(
        self, thread: dict, rowid: int, errors: list
    ) -> Iterator[tuple[str, str, str, str]]:
        """Yield (turn_type, text, tool_name, tool_use_id) in transcript order.

        A `messages[]` item, or a content, segment, tool use or tool result
        item inside one, of a shape not recognised is appended to `errors` as
        (rowid, `unparseable record <N> (<reason>)`, the item as JSON), N the
        message's 1-based position in `messages[]`.
        """
        messages = thread.get("messages")
        if messages is None:
            return
        if not isinstance(messages, list):
            reason = "unparseable record 0 (messages is not a list)"
            errors.append((rowid, reason, record_json(messages)))
            return

        def bad(position: int, reason: str, item) -> None:
            errors.append(
                (rowid, "unparseable record %d (%s)" % (position, reason), record_json(item))
            )

        for position, message in enumerate(messages, 1):
            if isinstance(message, str):
                yield "system", message.lower(), "", ""
            elif not isinstance(message, dict):
                bad(position, "unknown message variant", message)
            elif "User" in message:
                yield from self._user(message["User"], position, bad)
            elif "Agent" in message:
                yield from self._agent(message["Agent"], position, bad)
            elif "Compaction" in message:
                compaction = message["Compaction"]
                if isinstance(compaction, dict) and isinstance(compaction.get("Summary"), str):
                    yield "system", "compaction: " + compaction["Summary"], "", ""
                elif isinstance(compaction, dict) and "ProviderNative" in compaction:
                    yield "system", "compaction: provider native", "", ""
                else:
                    bad(position, "unknown Compaction variant", message)
            elif "role" in message:
                yield from self._legacy(message, position, bad)
            else:
                bad(position, "unknown message variant", message)

    @staticmethod
    def _user(user_message, position: int, bad) -> Iterator[tuple[str, str, str, str]]:
        # One row per prompt: text, mentions and images of one User message
        # are joined so sessions.jsonl counts prompts, not content items. A
        # content object of an unknown variant stays in the prompt text as
        # JSON; an item that is not an object or string goes to `bad`.
        if not isinstance(user_message, dict):
            bad(position, "User message is not an object", {"User": user_message})
            return
        content = user_message.get("content")
        if isinstance(content, (str, dict)):
            content = [content]
        elif content is not None and not isinstance(content, list):
            bad(position, "User content is not a list", content)
            content = []
        parts = []
        for k, item in enumerate(content or []):
            if isinstance(item, str):
                parts.append(item)
            elif not isinstance(item, dict):
                bad(position, "unknown User content variant, content[%d]" % k, item)
            else:
                if isinstance(item.get("Text"), str):
                    parts.append(item["Text"])
                elif "Mention" in item:
                    parts.append(mention_text(item["Mention"]))
                elif "Image" in item:
                    parts.append("[image]")
                else:
                    parts.append(compact_json(item))
        yield "user", "\n".join(parts), "", ""

    @staticmethod
    def _agent(agent_message, position: int, bad) -> Iterator[tuple[str, str, str, str]]:
        if not isinstance(agent_message, dict):
            bad(position, "Agent message is not an object", {"Agent": agent_message})
            return
        results = agent_message.get("tool_results")
        if results is not None and not isinstance(results, dict):
            bad(position, "Agent tool_results is not an object", results)
        if not isinstance(results, dict):
            results = {}
        done = set()

        def result(key, tool_result: dict):
            text = content_text(tool_result.get("content"))
            if not text and tool_result.get("output") is not None:
                text = (
                    content_text(tool_result["output"])
                    if not isinstance(tool_result["output"], dict)
                    else compact_json(tool_result["output"])
                )
            if tool_result.get("is_error"):
                text = "[error] " + text
            name, tool_use_id = (
                str(tool_result.get("tool_name") or ""),
                str(tool_result.get("tool_use_id") or key),
            )
            yield "tool_result", text, name, tool_use_id
            decision = denial_text(tool_result)
            if decision:
                yield "system", decision, name, tool_use_id

        content = agent_message.get("content")
        if isinstance(content, (str, dict)):
            content = [content]
        elif content is not None and not isinstance(content, list):
            bad(position, "Agent content is not a list", content)
            content = []
        for k, item in enumerate(content or []):
            if isinstance(item, str):
                yield "assistant", item, "", ""
            elif not isinstance(item, dict):
                bad(position, "unknown Agent content variant, content[%d]" % k, item)
            elif isinstance(item.get("Text"), str):
                yield "assistant", item["Text"], "", ""
            elif "Thinking" in item:
                thinking = item["Thinking"]
                yield (
                    "thinking",
                    (thinking.get("text") if isinstance(thinking, dict) else str(thinking)) or "",
                    "",
                    "",
                )
            elif "RedactedThinking" in item:
                yield "thinking", "[redacted thinking]", "", ""
            elif isinstance(item.get("ToolUse"), dict):
                tool_use = item["ToolUse"]
                tool_use_id = str(tool_use.get("id") or "")
                yield (
                    "tool_use",
                    tool_input_text(tool_use),
                    str(tool_use.get("name") or ""),
                    tool_use_id,
                )
                # The result follows its call, so the timeline reads in order.
                if isinstance(results.get(tool_use_id), dict):
                    done.add(tool_use_id)
                    yield from result(tool_use_id, results[tool_use_id])
            elif "Image" in item:
                yield "assistant", "[image]", "", ""
            else:
                bad(position, "unknown Agent content variant, content[%d]" % k, item)
        for key, tool_result in results.items():
            if key in done:
                continue
            if isinstance(tool_result, dict):
                yield from result(key, tool_result)
            else:
                bad(position, "Agent tool_results entry %s is not an object" % key, tool_result)

    @staticmethod
    def _legacy(message: dict, position: int, bad) -> Iterator[tuple[str, str, str, str]]:
        role = str(message.get("role") or "").lower()
        kind = "user" if role == "user" else "assistant" if role == "assistant" else "system"
        if isinstance(message.get("text"), str):
            yield (
                kind,
                message["text"] if kind != "system" else "%s: %s" % (role, message["text"]),
                "",
                "",
            )
        for k, seg in enumerate(message.get("segments") or []):
            segment_type = str(seg.get("type") or "").lower() if isinstance(seg, dict) else ""
            if segment_type == "text":
                yield kind, str(seg.get("text") or ""), "", ""
            elif segment_type == "thinking":
                yield "thinking", str(seg.get("text") or ""), "", ""
            elif segment_type == "redactedthinking":
                yield "thinking", "[redacted thinking]", "", ""
            else:
                bad(position, "unknown segment variant, segments[%d]" % k, seg)
        for k, tool_use in enumerate(message.get("tool_uses") or []):
            if not isinstance(tool_use, dict):
                bad(position, "tool_uses[%d] is not an object" % k, tool_use)
            else:
                yield (
                    "tool_use",
                    tool_input_text(tool_use),
                    str(tool_use.get("name") or ""),
                    str(tool_use.get("id") or ""),
                )
        for k, result in enumerate(message.get("tool_results") or []):
            if not isinstance(result, dict):
                bad(position, "tool_results[%d] is not an object" % k, result)
            else:
                text = content_text(result.get("content"))
                if result.get("is_error"):
                    text = "[error] " + text
                tool_use_id = str(result.get("tool_use_id") or "")
                yield "tool_result", text, "", tool_use_id
                decision = denial_text(result)
                if decision:
                    yield "system", decision, "", tool_use_id


__all__ = [
    "ZedParser",
    "acp_blocks_text",
    "content_text",
    "decompress",
    "denial_text",
    "first_folder",
    "record_json",
    "tool_input_text",
]
