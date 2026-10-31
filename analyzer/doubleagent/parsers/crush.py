"""Parse Crush's per-project SQLite database, its project registry and its permission settings.

Reads `<project>/.crush/crush.db` (WAL mode; `artifact.rel` is `.crush/crush.db` under a project
or a home, and `project_path` is the directory holding `.crush`, since the database records no
working directory), `projects.json` under `.local/share/crush` or `AppData/Local/crush`, every
`crush.json` scope and the `crushrc` shell configs.

Fields the parser depends on:

- `sessions`: `id`, `parent_session_id`, `title`, `message_count`, `prompt_tokens`,
  `completion_tokens`, `cost`, `channel`, `created_at`, `updated_at` (epoch seconds);
- `messages`: `id`, `session_id`, `role`, `parts` (a JSON array of `{type, data}`), `model`,
  `provider`, `prism_model_id`, `created_at`, `finished_at`; part `data`: `text` and `hidden`,
  `thinking` and `started_at`, `id`, `name`, `input`, `tool_call_id`, `content`, `is_error`,
  `mime_type`, `metadata.hook`, `reason`, `time`, `message`, `details`, `command`, `output`,
  `exit_code`, `url`, `Path`, `MIMEType`;
- `mcp_enabled_servers.name`, `mcp_disabled_servers.name`, `files` (`session_id`, `path`,
  `version`, the `content` length, `created_at`) and `read_files` (`session_id`, `path`,
  `read_at`);
- `projects.json`: `projects[]` `path`, `data_dir`, `last_accessed`; `crush.json`:
  `permissions.allowed_tools`, `options.disabled_tools`, `mcp.<name>.disabled_tools` and
  `enabled_tools`, `hooks.<event>`; `crushrc`: `permissions allow` and `permissions deny` lines.

Parts carry no timestamp: they inherit the message's `created_at`, except tool calls and
results, at `finished_at` or the finish part's `time`. A `finish` part is a row only when the
turn ended abnormally or carries a message. No git branch is recorded. Validated against
charmbracelet/crush source at ca6ae26; the Prism model, permission, relay, deletion and file
rows rest on source at 7f799fb alone. See `research/crush.md`.
"""

from __future__ import annotations

import html
import json
import re
import shlex
import sqlite3
from collections.abc import Iterator

from ..inputs import Artifact
from ..model import Row, compact
from ..sqlite_util import is_sqlite, open_copy, table_names
from ..timeutil import to_utc
from .base import Options, Parser, compact_json, error_rows, file_mtime, setting_value

DATABASE_PATTERN = re.compile(r"(^|/)\.crush/crush\.db$")
PROJECTS_PATTERN = re.compile(r"^(\.local/share/crush|AppData/Local/crush)/projects\.json$")
ORIGINAL_PATTERN = re.compile(r"^(.*)[\\/]\.crush[\\/]crush\.db$")
# Every crush.json scope the catalog collects: the global config, the global
# data file, the workspace file in a data directory, and a project's own.
CONFIG_PATTERN = re.compile(
    r"^(\.config/crush/crush\.json|\.local/share/crush/crush\.json"
    r"|AppData/Local/crush/crush\.json|\.crush/crush\.json|\.?crush\.json)$"
)
# The global shell config beside crush.json, and a project's crushrc or .crushrc.
CRUSHRC_PATTERN = re.compile(r"^(\.config/crush/crushrc|\.?crushrc)$")
CONFIG_ORIGINAL_PATTERN = re.compile(r"^(.*)[\\/]\.crush[\\/]crush\.json$")
CHANNEL_PATTERN = re.compile(r"^<channel[ \t\r\n]([^>]*)>")
ATTRIBUTE_PATTERN = re.compile(r'([A-Za-z_][\w.-]*)="([^"]*)"')
DENIED = "User denied permission"
HOOK_PREFIXES = (("Tool call blocked by hook.", "deny"), ("Turn halted by hook.", "halt"))
MCP_TABLES = ("mcp_enabled_servers", "mcp_disabled_servers")

NORMAL_FINISH = ("", "end_turn", "tool_use", "stop")


def _join(*parts: str) -> str:
    return " | ".join(part for part in parts if part)


def _str(value) -> str:
    return value if isinstance(value, str) else ""


def _model_text(seen: dict, session_id: str, provider: str, requested: str, prism: str) -> str:
    """Return the `model:` row text for one message, or "" when the session's model is unchanged.

    `seen` maps a session id to its last (provider, requested, served) triple. The served model is
    `prism` when set, else `requested`; a message without `prism` that repeats the last provider
    and request keeps the last served model rather than counting as a change.
    """
    old = seen.get(session_id)
    served = prism
    if not served:
        if old and old[:2] == (provider, requested):
            return ""
        served = requested
    if not served:
        return ""
    new = (provider, requested, served)
    if old == new:
        return ""
    seen[session_id] = new
    text = "model: %s -> %s" % (old[2], served) if old else "model: " + served
    if provider:
        text += " provider=" + provider
    if requested and requested != served:
        text += " requested=" + requested
    return text


def _metadata(value) -> dict:
    """A tool result's `metadata`, stored as a JSON string, as a dict."""
    if isinstance(value, str) and value:
        try:
            value = json.loads(value)
        except ValueError:
            return {}
    return value if isinstance(value, dict) else {}


def _permission_text(data: dict) -> str:
    """Return the `permission decision:` text for one `tool_result` part's data, or "".

    A user denial is the error content `User denied permission`; a hook's decision is
    `metadata.hook`, and an error result that starts with a hook's block or halt message stands
    for it when the metadata is missing. A hook decision of `none` without a halt is not one.
    """
    content = _str(data.get("content"))
    if data.get("is_error") and content == DENIED:
        return "permission decision: denied by user"
    hook = _metadata(data.get("metadata")).get("hook")
    if isinstance(hook, dict):
        decision = "halt" if hook.get("halt") else _str(hook.get("decision"))
        if decision not in ("allow", "deny", "halt"):
            return ""
        hooks = [entry for entry in hook.get("hooks") or [] if isinstance(entry, dict)]
        if decision == "halt":
            chosen = [entry for entry in hooks if entry.get("halt")]
        else:
            chosen = [entry for entry in hooks if entry.get("decision") == decision]
        names = ",".join(
            _str(entry.get("name")) for entry in chosen or hooks if _str(entry.get("name"))
        )
        text = "permission decision: %s by hook" % decision
        if names:
            text += " " + names
        if _str(hook.get("reason")):
            text += " reason=" + hook["reason"]
        return text
    if data.get("is_error"):
        for prefix, decision in HOOK_PREFIXES:
            if content.startswith(prefix):
                return "permission decision: %s by hook" % decision
    return ""


def _relay_text(text: str, message_id: str) -> tuple[str, str]:
    """Return the `session relayed:` text of a message an MCP channel pushed, and its relay key.

    The message starts `<channel source="<server>" <meta>>`; the key is the part of the text that
    identifies the relay (source, sender, group). Any other message gives ("", "").
    """
    match = CHANNEL_PATTERN.match(text)
    if not match:
        return "", ""
    attributes = {
        attribute: html.unescape(value)
        for attribute, value in ATTRIBUTE_PATTERN.findall(match.group(1))
    }
    key = "session relayed: channel %s" % attributes.get("source", "")
    for name in ("sender", "group"):
        if attributes.get(name):
            key += " %s %s" % (name, attributes[name])
    return key + " (from crush.db message %s)" % message_id, key


def project_of(original: str) -> str:
    """Return the project directory of a `crush.db` from its original path, or ""."""
    match = ORIGINAL_PATTERN.match(original or "")
    return match.group(1) if match else ""


class CrushParser(Parser):
    """Parse Crush's databases, project registry and permission settings."""

    agent = "crush"
    name = "crush"
    records_relay = True

    def wants(self, artifact: Artifact) -> bool:
        """Return whether the artifact is one of the files this parser reads."""
        rel = artifact.rel
        return bool(
            DATABASE_PATTERN.search(rel)
            or PROJECTS_PATTERN.match(rel)
            or CONFIG_PATTERN.match(rel)
            or CRUSHRC_PATTERN.match(rel)
        )

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one artifact, by the file it is."""
        if PROJECTS_PATTERN.match(artifact.rel):
            yield from self._parse_projects(artifact, opts)
        elif CONFIG_PATTERN.match(artifact.rel):
            yield from self._parse_config(artifact)
        elif CRUSHRC_PATTERN.match(artifact.rel):
            yield from self._parse_crushrc(artifact)
        else:
            yield from self._parse_database(artifact, opts)

    def _system(self, artifact: Artifact, text: str, line: int = 0) -> Row:
        row = self.base_row(artifact)
        row.turn_type = "system"
        row.source_line = line
        row.text = text
        return row

    # -- projects.json ---------------------------------------------------------------
    def _parse_projects(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        try:
            with open(artifact.disk_path, "rb") as handle:
                data = json.loads(handle.read().decode("utf-8", errors="replace"))
        except OSError as error:
            reason = error.strerror or error
            yield self._system(artifact, "parser: projects.json could not be read: %s" % reason)
            return
        except ValueError as error:
            yield self._system(artifact, "parser: projects.json did not parse: %s" % error)
            return
        projects = data.get("projects") if isinstance(data, dict) else None
        for position, project in enumerate(projects if isinstance(projects, list) else [], 1):
            if not isinstance(project, dict):
                continue
            row = self._system(artifact, "", position)
            row.timestamp_utc = to_utc(project.get("last_accessed"))
            row.project_path = str(project.get("path") or "")
            row.text = compact(
                _join(
                    "crush project last accessed",
                    "data_dir=%s" % project["data_dir"] if project.get("data_dir") else "",
                ),
            )
            yield row

    # -- crush.json and crushrc ------------------------------------------------------
    def _config_project(self, artifact: Artifact) -> str:
        """Return the project a config file belongs to, or "" for a global file.

        A workspace `crush.json` belongs to the directory holding its `.crush`, a project-catalog
        file to its project directory.
        """
        match = CONFIG_ORIGINAL_PATTERN.match(artifact.original or "")
        if match:
            return match.group(1)
        return artifact.home.original if artifact.home.project else ""

    def _setting_row(
        self, artifact: Artifact, timestamp: str, key: str, value, where: str, line: int = 1
    ) -> Row:
        row = self._system(artifact, "", line)
        row.timestamp_utc = timestamp
        row.project_path = self._config_project(artifact)
        row.text = compact(
            "permission setting: %s=%s (from %s, file mtime)" % (key, setting_value(value), where)
        )
        return row

    def _parse_config(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `permission setting:` row per standing entry of a crush.json.

        The entries are `permissions.allowed_tools`, `options.disabled_tools`,
        `mcp.<name>.disabled_tools` and `enabled_tools`, and each hook under `hooks.<event>`.
        Nothing else is read, so no key is printed.
        """
        timestamp = file_mtime(artifact)
        try:
            document = json.loads(artifact.disk_path.read_bytes().decode("utf-8", errors="replace"))
        except (OSError, ValueError):
            document = None
        if not isinstance(document, dict):
            row = self._system(artifact, "parser: %s is not a JSON object" % artifact.rel, 1)
            row.timestamp_utc = timestamp
            row.project_path = self._config_project(artifact)
            yield row
            return
        lists = []
        for section, key in (("permissions", "allowed_tools"), ("options", "disabled_tools")):
            block = document.get(section)
            if isinstance(block, dict):
                lists.append(("%s.%s" % (section, key), block.get(key)))
        mcp = document.get("mcp")
        for name, server in mcp.items() if isinstance(mcp, dict) else []:
            if isinstance(server, dict):
                for key in ("disabled_tools", "enabled_tools"):
                    lists.append(("mcp.%s.%s" % (name, key), server.get(key)))
        hooks = document.get("hooks")
        for event, entries in hooks.items() if isinstance(hooks, dict) else []:
            lists.append(("hooks.%s" % event, entries))
        for key, entries in lists:
            for entry in entries if isinstance(entries, list) else []:
                yield self._setting_row(artifact, timestamp, key, entry, artifact.rel)

    def _parse_crushrc(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `permission setting:` row per tool on a crushrc `permissions` line.

        Allow adds to `permissions.allowed_tools`, deny to `options.disabled_tools`. Only lines
        whose first word is `permissions` are read, as written, whether or not a condition around
        them runs; such a line that does not split into shell words (an unclosed quote) is a bad
        line.
        """
        try:
            lines = artifact.disk_path.read_bytes().decode("utf-8", errors="replace")
        except OSError as error:
            yield self._system(artifact, "parser: %s could not be read: %s" % (artifact.rel, error))
            return
        timestamp = file_mtime(artifact)
        keys = {"allow": "permissions.allowed_tools", "deny": "options.disabled_tools"}
        errors: list = []
        for line_number, line in enumerate(lines.split("\n"), 1):
            line = line[:-1] if line.endswith("\r") else line
            if not line.lstrip().startswith("permissions"):
                continue
            try:
                words = shlex.split(line, comments=True)
            except ValueError as error:
                errors.append((line_number, "unparseable line (%s)" % error, line))
                continue
            if len(words) < 3 or words[0] != "permissions" or words[1] not in keys:
                continue
            where = "%s line %d" % (artifact.rel, line_number)
            for tool in words[2:]:
                yield self._setting_row(
                    artifact, timestamp, keys[words[1]], tool, where, line_number
                )
        yield from error_rows(self, artifact, errors, "", self._config_project(artifact))

    # -- .crush/crush.db ---------------------------------------------------------------
    def _parse_database(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        project_path = project_of(artifact.original)
        if not is_sqlite(artifact.disk_path):
            row = self._system(artifact, "parser: not a SQLite database")
            row.project_path = project_path
            yield row
            return
        try:
            with open_copy(artifact.disk_path) as connection:
                tables = table_names(connection)
                if "messages" not in tables:
                    return
                if "sessions" in tables:
                    yield from self._session_rows(artifact, connection, project_path)
                yield from self._mcp_rows(artifact, connection, tables, project_path)
                models: dict = {}
                relayed: set = set()
                for message_row in connection.execute(
                    "select rowid as _rowid, * from messages order by created_at, rowid"
                ):
                    yield from self._message_rows(
                        artifact, dict(message_row), project_path, opts, models, relayed
                    )
                yield from self._file_rows(artifact, connection, tables, project_path)
        except sqlite3.DatabaseError as error:
            row = self._system(artifact, "parser: SQLite error: %s" % error)
            row.project_path = project_path
            yield row

    def _session_rows(self, artifact: Artifact, connection, project_path: str) -> Iterator[Row]:
        """Yield the session start, relay and deletion rows of the `sessions` table.

        A `session deleted:` row is written per parent a sub-agent session names that has no
        `sessions` row: the delete removes one id, and `parent_session_id` has no foreign key.
        """
        sessions = [
            dict(session)
            for session in connection.execute(
                "select rowid as _rowid, * from sessions order by created_at, rowid"
            )
        ]
        ids = {str(session.get("id") or "") for session in sessions}
        for session in sessions:
            session_id = str(session.get("id") or "")
            row = self._system(artifact, "", session["_rowid"])
            row.session_id = session_id
            row.project_path = project_path
            row.timestamp_utc = to_utc(session.get("created_at"))
            row.text = compact(
                _join(
                    "session start",
                    session.get("title") or "",
                    "parent=%s" % session["parent_session_id"]
                    if session.get("parent_session_id")
                    else "",
                    "%s messages" % session["message_count"]
                    if session.get("message_count") is not None
                    else "",
                    "tokens in=%s out=%s"
                    % (session.get("prompt_tokens") or 0, session.get("completion_tokens") or 0),
                    "cost=%s" % session["cost"] if session.get("cost") else "",
                ),
            )
            yield row
            if _str(session.get("channel")):
                row = self._system(artifact, "", session["_rowid"])
                row.session_id = session_id
                row.project_path = project_path
                row.timestamp_utc = to_utc(session.get("updated_at")) or to_utc(
                    session.get("created_at")
                )
                row.text = compact(
                    "session relayed: channel %s (from crush.db sessions.channel, as of updated_at)"
                    % session["channel"]
                )
                yield row
        orphans: set = set()
        for session in sessions:
            parent = _str(session.get("parent_session_id"))
            if not parent or parent in ids or parent in orphans:
                continue
            orphans.add(parent)
            row = self._system(artifact, "", session["_rowid"])
            row.session_id = parent
            row.project_path = project_path
            row.timestamp_utc = to_utc(session.get("updated_at")) or to_utc(
                session.get("created_at")
            )
            row.text = compact(
                "session deleted: %s (parent_session_id of sub-agent session %s names no "
                "session in crush.db; time is the sub-agent's updated_at)"
                % (parent, session.get("id"))
            )
            yield row

    def _mcp_rows(self, artifact: Artifact, connection, tables, project_path: str) -> Iterator[Row]:
        """Yield one `permission setting:` row per MCP server turned on or off for the project.

        The rows are at the database file's modification time.
        """
        for table in MCP_TABLES:
            if table not in tables:
                continue
            for record in connection.execute(
                "select rowid as _rowid, name from %s order by rowid" % table
            ):
                row = self._system(artifact, "", record["_rowid"])
                row.timestamp_utc = file_mtime(artifact)
                row.project_path = project_path
                row.text = compact(
                    "permission setting: %s=%s (from %s, file mtime)"
                    % (table, record["name"], artifact.rel)
                )
                yield row

    def _file_rows(
        self, artifact: Artifact, connection, tables, project_path: str
    ) -> Iterator[Row]:
        """Yield a `file version:` row per `files` snapshot and a `file read:` row per read.

        A snapshot row gives the path, version and the stored content's length, never the content.
        """
        if "files" in tables:
            for record in connection.execute(
                "select rowid as _rowid, session_id, path, version, length(content) as size,"
                " created_at from files order by created_at, rowid"
            ):
                row = self._system(artifact, "", record["_rowid"])
                row.session_id = str(record["session_id"] or "")
                row.project_path = project_path
                row.timestamp_utc = to_utc(record["created_at"])
                row.text = compact(
                    "file version: %s version=%s bytes=%s (from crush.db files)"
                    % (record["path"], record["version"], record["size"] or 0)
                )
                yield row
        if "read_files" in tables:
            for record in connection.execute(
                "select rowid as _rowid, session_id, path, read_at from read_files"
                " order by read_at, rowid"
            ):
                row = self._system(artifact, "", record["_rowid"])
                row.session_id = str(record["session_id"] or "")
                row.project_path = project_path
                row.timestamp_utc = to_utc(record["read_at"])
                row.text = compact(
                    "file read: %s (from crush.db read_files, read_at)" % record["path"]
                )
                yield row

    def _message_rows(
        self,
        artifact: Artifact,
        message: dict,
        project_path: str,
        opts: Options,
        models: dict,
        relayed: set,
    ) -> Iterator[Row]:
        role = str(message.get("role") or "")
        provider = _str(message.get("provider"))
        requested, prism = _str(message.get("model")), _str(message.get("prism_model_id"))
        model_id = prism or requested
        model = "%s/%s" % (provider, model_id) if provider and model_id else model_id
        created = to_utc(message.get("created_at"))
        session_id = str(message.get("session_id") or "")

        def base(timestamp: str = "") -> Row:
            row = self.base_row(artifact)
            row.source_line = message["_rowid"]
            row.session_id = str(message.get("session_id") or "")
            row.project_path = project_path
            row.model = model
            row.timestamp_utc = timestamp or created
            return row

        text = _model_text(models, session_id, provider, requested, prism)
        if text:
            row = base()
            row.turn_type = "system"
            row.text = text
            yield row
        raw = message.get("parts")
        if isinstance(raw, bytes):
            raw = raw.decode("utf-8", errors="replace")
        elif raw is not None and not isinstance(raw, str):
            raw = str(raw)
        try:
            parts = json.loads(raw or "[]")
            if not isinstance(parts, list):
                raise ValueError("not a JSON array")
        except ValueError as error:
            reason = "unparseable record %s (message %s parts: %s)" % (
                message["_rowid"],
                message.get("id"),
                error,
            )
            errors = [(message["_rowid"], reason, raw)]
            yield from error_rows(self, artifact, errors, session_id, project_path)
            return
        finished = to_utc(message.get("finished_at")) if message.get("finished_at") else ""
        for part in parts:
            if isinstance(part, dict) and part.get("type") == "finish":
                finish_time = (part.get("data") or {}).get("time")
                if finish_time and not finished:
                    finished = to_utc(finish_time)
        text_type = role if role in ("user", "assistant") else "system"

        for part in parts:
            if not isinstance(part, dict):
                continue
            after: list = []  # rows that follow this part's own row
            kind = part.get("type")
            data = part.get("data")
            if not isinstance(data, dict):
                data = {}
            row = base()
            if kind == "text":
                row.turn_type = text_type
                text = data.get("text") or ""
                if role not in ("user", "assistant"):
                    text = "%s: %s" % (role, text)
                if data.get("hidden"):
                    text = "[hidden] " + text
                relay, key = _relay_text(text, str(message.get("id") or ""))
                if relay and role == "user" and (session_id, key) not in relayed:
                    relayed.add((session_id, key))
                    extra = base()
                    extra.turn_type = "system"
                    extra.text = compact(relay)
                    after.append(extra)
            elif kind == "reasoning":
                if not opts.include_thinking or not data.get("thinking"):
                    continue
                row.turn_type = "thinking"
                row.timestamp_utc = to_utc(data.get("started_at")) or created
                text = data.get("thinking")
            elif kind == "tool_call":
                row.turn_type = "tool_use"
                row.timestamp_utc = finished or created
                row.tool_name = str(data.get("name") or "")
                row.tool_use_id = str(data.get("id") or "")
                text = (
                    data.get("input")
                    if isinstance(data.get("input"), str)
                    else compact_json(data.get("input"))
                )
            elif kind == "tool_result":
                row.turn_type = "tool_result"
                row.timestamp_utc = finished or created
                row.tool_name = str(data.get("name") or "")
                row.tool_use_id = str(data.get("tool_call_id") or "")
                text = data.get("content") or ""
                if not text and data.get("mime_type"):
                    text = "[%s]" % data["mime_type"]
                if data.get("is_error"):
                    text = "[error] " + text
                decision = _permission_text(data)
                if decision:
                    extra = base(row.timestamp_utc)
                    extra.turn_type = "system"
                    extra.tool_name, extra.tool_use_id = row.tool_name, row.tool_use_id
                    extra.text = compact(decision)
                    after.append(extra)
            elif kind == "finish":
                reason = str(data.get("reason") or "")
                if reason in NORMAL_FINISH and not data.get("message"):
                    continue
                row.turn_type = "system"
                row.timestamp_utc = to_utc(data.get("time")) if data.get("time") else created
                text = _join(
                    "finish: " + reason, data.get("message") or "", data.get("details") or ""
                )
            elif kind == "shell_command":
                row.turn_type = "system"
                text = _join(
                    "shell: " + str(data.get("command") or ""),
                    "exit=%s" % data["exit_code"] if "exit_code" in data else "",
                    data.get("output") or "",
                )
            elif kind == "image_url":
                row.turn_type = text_type
                url = str(data.get("url") or "")
                text = "[image] " + (url.split(",", 1)[0] if url.startswith("data:") else url)
            elif kind == "binary":
                row.turn_type = text_type
                text = "[binary] " + _join(
                    str(data.get("Path") or data.get("path") or ""),
                    str(data.get("MIMEType") or data.get("mime_type") or ""),
                )
            else:
                row.turn_type = "system"
                text = "%s: %s" % (kind, compact_json(data))
            row.text = compact(text)
            yield row
            yield from after
