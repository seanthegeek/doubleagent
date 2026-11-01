"""Claude Desktop local agent sessions (Cowork, the Code tab, scheduled tasks).

Reads, under the app-data directory (`BASE`: `Claude[-3p]` under `Library/Application
Support`, `.config`, `AppData/Roaming` or `AppData/Local`, `AppData/Local/Claude-Data`, or
the MSIX build's `AppData/Local/Packages/<package>/LocalCache/Roaming/Claude`): the Claude
Code transcripts under `local-agent-mode-sessions/<acct>/<org>/[agent/]<session dir>/.claude/
projects/` and `claude-code-sessions/<acct>/<org>/` (including `imported-staging/`), read by
`ClaudeCodeParser._parse_session`; each session dir's `audit.jsonl` (a secondary source);
the session records `local_<uuid>.json` (a secondary source); `scheduled-tasks.json`,
`git-worktrees.json` and `remote-session-spaces.json`. It yields the transcript rows with
the record's folder as `project_path`, the audit log's user, assistant, tool, model,
permission, `result` and `init` rows, and system rows for records, grants, scheduled
tasks, worktrees and remote folder grants.

Fields read: the audit line's `type`, `subtype`, `timestamp`, `_audit_timestamp`,
`session_id`, `cwd`, `model`, `client_platform`, `isSynthetic`, `message.model`,
`message.content` blocks, `tool_name`, `tool_input`, `decision`, `granted`, `source`,
`scheduled_task_id`, `matched_ops`, `num_turns`, `is_error`; the record's `sessionId`,
`cliSessionId`, `sessionType`, `title`, `createdAt`, `lastActivityAt`, `model`,
`userSelectedFolders`, `originCwd`, `cwd`, `branch`, `initialMessage`, `error`,
`errorCategory`, `errorAt`, `permissionMode`, `effort`, the Remote Control keys and the
grant keys `GRANT_LISTS`, `GRANT_KEYS` and `COWORK_GRANTS` name; a task's `id`,
`cronExpression`, `fireAt`, `enabled`, `createdAt`, `cwd`, `userSelectedFolders`,
`permissionMode`, `model`; a worktree's `path`, `branch`, `sourceBranch`, `baseRepo`,
`leasedBy`, `createdAt`; and the spaces file's `entries[].sessionId` and `folders[]`.
Every `createdAt`, `errorAt` and `fireAt` is epoch milliseconds.

Closed source: the shapes come from the app bundle 2.31226.1 (`app.asar`, macOS), with
top-level keys observed on installs of that version on macOS, Windows 11 and Ubuntu. See
`research/claude-desktop.md`.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterator
from pathlib import Path

from ..inputs import Artifact
from ..model import SCRATCH_PREFIX, Row, compact
from ..timeutil import to_utc
from .base import (
    Options,
    compact_json,
    error_rows,
    file_bytes,
    file_mtime,
    iter_jsonl,
    text_of,
    track_change,
)
from .claude_code import ClaudeCodeParser, result_text, tool_summary

BASE = (
    r"^(?:Library/Application Support/Claude(?:-3p)?"
    r"|\.config/Claude(?:-3p)?"
    r"|AppData/Roaming/Claude(?:-3p)?"
    r"|AppData/Local/Claude(?:-3p)?"
    r"|AppData/Local/Claude-Data(?:/[^/]+)?"
    r"|AppData/Local/Packages/(?:AnthropicPBC\.)?Claude_[^/]+/LocalCache/Roaming/Claude)/"
)
_TREES = r"(?:local-agent-mode-sessions|claude-code-sessions)/[^/]+/[^/]+/"
# Group `sess` is the session dir, relative to the home.
TRANSCRIPT_PATTERN = re.compile(
    r"(?P<sess>"
    + BASE
    + _TREES
    + r"(?:agent/)?[^/]+)/\.claude/projects/[^/]+/"
    + r"(?:[^/]+\.jsonl|[^/]+/subagents/(?:[^/]+/)*agent-[^/]+\.jsonl)$"
)
STAGED_PATTERN = re.compile(
    BASE + r"claude-code-sessions/[^/]+/[^/]+/imported-staging/[^/]+\.jsonl$"
)
AUDIT_PATTERN = re.compile(
    r"(?P<sess>" + BASE + r"local-agent-mode-sessions/[^/]+/[^/]+/(?:agent/)?[^/]+)/audit\.jsonl$"
)
RECORD_PATTERN = re.compile(
    BASE + r"(?P<tree>" + _TREES + r")(?:agent/)?local_[0-9a-f-]{36}\.json$"
)
SCHEDULED_TASKS_PATTERN = re.compile(BASE + _TREES + r"scheduled-tasks\.json$")
WORKTREE_PATTERN = re.compile(BASE + r"git-worktrees\.json$")
SPACES_PATTERN = re.compile(
    BASE + r"local-agent-mode-sessions/[^/]+/[^/]+/remote-session-spaces\.json$"
)

LONG_PATTERN = re.compile(r"^local_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
SHORT_PATTERN = re.compile(r"^[0-9a-f]{8}$")


def _bool(value) -> str:
    """`true`/`false` as in the JSON, the value as text otherwise."""
    if value is None:
        return ""
    return json.dumps(value) if isinstance(value, bool) else str(value)


def _flag(value) -> str:
    """`true` only for a JSON true; absent or anything else is `false`."""
    return "true" if value is True else "false"


def _str(value) -> str:
    return "" if value is None else str(value)


def _value(value) -> str:
    """A grant value as text: a string as it is, anything else as JSON."""
    return value if isinstance(value, str) else compact_json(value)


# Session-record keys that hold per-session permission grants: lists read one
# row per entry, then keys read one row each.
GRANT_LISTS = ("alwaysAllowedReasons", "sessionPermissionUpdates")
GRANT_KEYS = (
    "chromePermissionMode",
    "chromeAllowedDomains",
    "cuAllowedApps",
    "cuGrantFlags",
    "cuFlagsGrantedAt",
)
# Cowork tool, network and folder grants ("arrays and objects" in the bundle;
# which key is which type is not determined): a list is one row per entry, an
# object one row per member, anything else one row.
COWORK_GRANTS = (
    "approvedToolNames",
    "webFetchAllowedUrls",
    "egressAllowedDomains",
    "userApprovedFileAccessPaths",
    "enabledMcpTools",
    "fileDeleteApprovedMounts",
)
# Envelope keys of an audit line, left out of a generic row's JSON.
AUDIT_ENVELOPE = frozenset(
    (
        "type",
        "subtype",
        "uuid",
        "session_id",
        "timestamp",
        "_audit_timestamp",
        "_audit_hmac",
        "_audit_redactions",
    )
)


def _read_json(path: Path, errors: list):
    """Return the parsed file, or None when it cannot be read or parsed.

    A file that cannot be read adds its unreadable entry to `errors`.
    """
    data = file_bytes(path, errors)
    if data is None:
        return None
    try:
        return json.loads(data.decode("utf-8", errors="replace"))
    except ValueError:
        return None


def _plain_dir(path: Path) -> bool:
    return not path.is_symlink() and path.is_dir()


def _plain_file(path: Path) -> bool:
    return not path.is_symlink() and path.is_file()


def _org_dir(session_dir: Path) -> Path:
    return session_dir.parent.parent if session_dir.parent.name == "agent" else session_dir.parent


def _find_record(session_dir: Path) -> dict | None:
    """Return the session record of a session dir, or None.

    The dir is named `local_<uuid>` or with the UUID's first eight hex digits, and the
    record is found in `<org>/` or `<org>/agent/`, never through a symlink.
    """
    org = _org_dir(session_dir)
    name = session_dir.name
    if not (LONG_PATTERN.match(name) or SHORT_PATTERN.match(name)):
        return None
    if org.is_symlink() or session_dir.parent.is_symlink():
        return None
    for directory in (org, org / "agent"):
        if not _plain_dir(directory):
            continue
        if LONG_PATTERN.match(name):
            candidates = [directory / (name + ".json")]
        else:
            try:
                candidates = sorted(directory.glob("local_%s-*.json" % name))
            except OSError:
                candidates = []
        for candidate in candidates:
            if not LONG_PATTERN.match(candidate.name[: -len(".json")]) or not _plain_file(
                candidate
            ):
                continue
            # The record is an artifact `_record` parses in full and reports there.
            record = _read_json(candidate, [])
            if isinstance(record, dict):
                return record
    return None


def _jsonl_files(directory: Path, name: str = "*.jsonl") -> list[Path]:
    """Return the regular files `directory/*/<name>`, through no symlink."""
    if not _plain_dir(directory):
        return []
    out = []
    try:
        for slug in sorted(directory.iterdir()):
            if _plain_dir(slug):
                out.extend(path for path in sorted(slug.glob(name)) if _plain_file(path))
    except OSError:
        return out
    return out


def _has_transcript(session_dir: Path) -> bool:
    claude = session_dir / ".claude"
    return (
        _plain_dir(session_dir) and _plain_dir(claude) and bool(_jsonl_files(claude / "projects"))
    )


def _folder(record: dict | None) -> str:
    if not record:
        return ""
    folders = record.get("userSelectedFolders")
    if isinstance(folders, list) and folders and folders[0]:
        return str(folders[0])
    return ""


class ClaudeDesktopParser(ClaudeCodeParser):
    """Parse Claude Desktop's session transcripts, audit logs and session files."""

    agent = "claude-desktop"
    name = "claude-desktop"
    has_secondary = True
    # Not inherited from ClaudeCodeParser: the desktop's sync state is not
    # established, so its sessions keep `synced` "" (a `bridge-session`
    # record in one of its transcripts still gives `session synced:`).
    records_sync = False

    def wants(self, artifact: Artifact) -> bool:
        """Return True for a transcript, audit log, record or desktop state file."""
        rel = artifact.rel
        return any(
            pattern.match(rel)
            for pattern in (
                TRANSCRIPT_PATTERN,
                STAGED_PATTERN,
                AUDIT_PATTERN,
                RECORD_PATTERN,
                SCHEDULED_TASKS_PATTERN,
                WORKTREE_PATTERN,
                SPACES_PATTERN,
            )
        )

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Claude Desktop file."""
        rel = artifact.rel
        match = TRANSCRIPT_PATTERN.match(rel)
        if match:
            yield from self._transcript(
                artifact, opts, self._session_dir(artifact, match.group("sess"))
            )
        elif STAGED_PATTERN.match(rel):
            yield from self._transcript(artifact, opts, None)
        elif AUDIT_PATTERN.match(rel):
            match = AUDIT_PATTERN.match(rel)
            assert match is not None
            yield from self._audit(artifact, opts, self._session_dir(artifact, match.group("sess")))
        elif RECORD_PATTERN.match(rel):
            yield from self._record(artifact)
        elif SCHEDULED_TASKS_PATTERN.match(rel):
            yield from self._scheduled(artifact)
        elif WORKTREE_PATTERN.match(rel):
            yield from self._worktrees(artifact)
        elif SPACES_PATTERN.match(rel):
            yield from self._spaces(artifact)

    @staticmethod
    def _session_dir(artifact: Artifact, session_rel: str) -> Path:
        return artifact.home.disk_path / session_rel

    def _system(self, artifact: Artifact, text: str, line: int = 0) -> Row:
        row = self.base_row(artifact)
        row.turn_type = "system"
        row.source_line = line
        row.text = compact(text)
        return row

    # -- transcripts -----------------------------------------------------------

    def _transcript(
        self, artifact: Artifact, opts: Options, session_dir: Path | None
    ) -> Iterator[Row]:
        folder = _folder(_find_record(session_dir)) if session_dir is not None else ""
        for row in self._parse_session(artifact, opts):
            # A `scratch directory:` row names its directory, not the folder.
            if folder and not (row.turn_type == "system" and row.text.startswith(SCRATCH_PREFIX)):
                row.project_path = folder
            yield row

    # -- audit.jsonl -----------------------------------------------------------

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for an audit log or a session record."""
        return bool(AUDIT_PATTERN.match(artifact.rel) or RECORD_PATTERN.match(artifact.rel))

    @staticmethod
    def _init_cwd(artifact: Artifact) -> str:
        """Return the `cwd` of the audit log's first `system` `init` line, or "".

        It is the CLI's working directory, which its transcript records as the JSONL `cwd`.
        """
        for _, line in iter_jsonl(artifact.disk_path, []):
            if line.get("type") == "system" and line.get("subtype") == "init":
                cwd = _str(line.get("cwd"))
                if cwd:
                    return cwd
        return ""

    def _audit(self, artifact: Artifact, opts: Options, session_dir: Path) -> Iterator[Row]:
        record = _find_record(session_dir)
        project = (
            _folder(record)
            or self._init_cwd(artifact)
            or (_str(record.get("cwd")) if record else "")
        )
        cli_id = _str(record.get("cliSessionId")) if record else ""
        errors: list = []
        models: dict = {}
        last_session_id = cli_id
        for line_number, line in iter_jsonl(artifact.disk_path, errors):
            session_id = cli_id or _str(line.get("session_id"))
            last_session_id = session_id or last_session_id
            timestamp = to_utc(line.get("timestamp")) or to_utc(line.get("_audit_timestamp"))

            def make_row(
                turn_type: str,
                text: str,
                line_number=line_number,
                timestamp=timestamp,
                session_id=session_id,
            ) -> Row:
                new_row = self.base_row(artifact)
                new_row.source_line = line_number
                new_row.timestamp_utc = timestamp
                new_row.session_id = session_id
                new_row.project_path = project
                new_row.turn_type = turn_type
                new_row.text = compact(text)
                return new_row

            line_type = line.get("type")
            subtype = line.get("subtype")
            if line_type == "system" and subtype in (
                "permission_request",
                "permission_response",
                "permission_auto_approved",
                "permission_auto_denied",
            ):
                tool = _str(line.get("tool_name"))
                if subtype == "permission_request":
                    tool_input = line.get("tool_input")
                    text = "permission request: %s %s" % (
                        tool,
                        "" if tool_input is None else tool_summary(tool, tool_input),
                    )
                elif subtype == "permission_response":
                    text = "permission response: %s %s granted=%s" % (
                        tool,
                        _str(line.get("decision")),
                        _bool(line.get("granted")),
                    )
                else:
                    word = "approved" if subtype == "permission_auto_approved" else "denied"
                    why = line.get("source") or line.get("scheduled_task_id")
                    if not why and line.get("matched_ops") is not None:
                        why = compact_json(line.get("matched_ops"))
                    text = "permission auto-%s: %s" % (word, tool)
                    if why:
                        text += " (%s)" % why
                row = make_row("system", text)
                row.tool_name = tool
                yield row
            elif line_type == "result":
                yield make_row(
                    "system",
                    "result: %s turns=%s error=%s"
                    % (
                        _str(line.get("subtype")),
                        _str(line.get("num_turns")),
                        _bool(line.get("is_error")),
                    ),
                )
            elif line_type == "system" and subtype == "init":
                # The session's model, even when a transcript exists.
                model = _str(line.get("model"))
                text = track_change("model", models, session_id, model)
                if text:
                    row = make_row("system", text)
                    row.model = model
                    yield row
                parts = ["%s=%s" % (k, line.get(k)) for k in ("cwd", "model") if line.get(k)]
                yield make_row("system", "init: " + " ".join(parts) if parts else "init")
            elif line_type == "system" and subtype == "model_refusal_fallback":
                # The subtype is in the bundle, its fields are not recorded:
                # print them as they are.
                rest = {k: value for k, value in line.items() if k not in AUDIT_ENVELOPE}
                yield make_row("system", "model_refusal_fallback: " + compact_json(rest))
            elif line_type == "user":
                yield from self._audit_user(line, make_row)
            elif line_type == "assistant":
                model = _str((line.get("message") or {}).get("model"))
                text = track_change("model", models, session_id, model)
                if text:
                    row = make_row("system", text)
                    row.model = model
                    yield row
                yield from self._audit_assistant(line, make_row, opts)
        yield from error_rows(self, artifact, errors, last_session_id, project)

    @staticmethod
    def _audit_user(line: dict, make_row) -> Iterator[Row]:
        content = (line.get("message") or {}).get("content")
        blocks = content if isinstance(content, list) else [{"type": "text", "text": content}]
        typed = line.get("client_platform") == "desktop_app" and not line.get("isSynthetic")
        for block in blocks:
            if not isinstance(block, dict):
                continue
            block_type = block.get("type")
            if block_type == "tool_result":
                text = result_text(block.get("content"))
                if block.get("is_error"):
                    text = "[error] " + text
                row = make_row("tool_result", text)
                row.tool_use_id = _str(block.get("tool_use_id"))
                yield row
            elif block_type in ("text", "image"):
                text = text_of([block])
                if not text:
                    continue
                if typed:
                    yield make_row("user", text)
                elif line.get("isSynthetic"):
                    yield make_row("system", "context: notification: " + text)
                else:
                    yield make_row("system", "context: " + text)

    @staticmethod
    def _audit_assistant(line: dict, make_row, opts: Options) -> Iterator[Row]:
        message = line.get("message") or {}
        model = _str(message.get("model"))
        content = message.get("content")
        blocks = content if isinstance(content, list) else [{"type": "text", "text": content}]
        for block in blocks:
            if not isinstance(block, dict):
                continue
            block_type = block.get("type")
            if block_type == "tool_use":
                name = _str(block.get("name"))
                row = make_row("tool_use", tool_summary(name, block.get("input")))
                row.tool_name = name
                row.tool_use_id = _str(block.get("id"))
            elif block_type == "text" and block.get("text"):
                row = make_row("assistant", _str(block.get("text")))
            elif block_type == "thinking" and opts.include_thinking and block.get("thinking"):
                row = make_row("thinking", _str(block.get("thinking")))
            elif block_type == "redacted_thinking" and opts.include_thinking:
                row = make_row("thinking", "[redacted]")
            else:
                continue
            row.model = model
            yield row

    # -- session records -------------------------------------------------------

    def _record(self, artifact: Artifact) -> Iterator[Row]:
        match = RECORD_PATTERN.match(artifact.rel)
        assert match is not None
        code = match.group("tree").startswith("claude-code-sessions/")
        errors: list = []
        record = _read_json(artifact.disk_path, errors)
        if errors:
            yield from error_rows(self, artifact, errors)
            return
        if not isinstance(record, dict):
            yield self._system(artifact, "parser: session record is not a JSON object", 1)
            return
        cli_id = _str(record.get("cliSessionId"))
        project = _folder(record) or _str(record.get("originCwd")) or _str(record.get("cwd"))
        created = to_utc(record.get("createdAt"))
        model = _str(record.get("model"))
        title = _str(record.get("title"))

        def make_row(turn_type: str, text: str, timestamp: str = created) -> Row:
            new_row = self.base_row(artifact)
            new_row.source_line = 1
            new_row.timestamp_utc = timestamp
            new_row.session_id = cli_id
            new_row.project_path = project
            new_row.git_branch = _str(record.get("branch")) if code else ""
            new_row.model = model
            new_row.turn_type = turn_type
            new_row.text = compact(text)
            return new_row

        mode_effort = " permission_mode=%s effort=%s" % (
            _str(record.get("permissionMode")) or "-",
            _str(record.get("effort")) or "-",
        )
        if code:
            spawn = record.get("remoteControlSpawn")
            bridges = record.get("bridgeSessionIds")
            yield make_row(
                "system",
                'desktop code session: "%s" remote_control=%s bridges=%d '
                "spawned_remotely=%s steered_remotely=%s"
                % (
                    title,
                    _flag(record.get("remoteControlUserEnabled")),
                    len(bridges) if isinstance(bridges, list) else 0,
                    _flag(isinstance(spawn, dict) and bool(spawn)),
                    _flag(record.get("steeredByRemoteClient")),
                )
                + mode_effort,
            )
            update_quit = record.get("remoteControlLiveAtUpdateQuit")
            quit_at = update_quit.get("at") if isinstance(update_quit, dict) else None
            if isinstance(quit_at, (int, float)) and not isinstance(quit_at, bool):
                yield make_row("system", "remote control live at update quit", to_utc(quit_at))
        else:
            yield make_row(
                "system",
                'desktop session: %s "%s"' % (_str(record.get("sessionType")), title) + mode_effort,
            )
        yield from self._grants(record, make_row, created)
        initial = _str(record.get("initialMessage"))
        if initial and not code:
            # Cowork: always, with no time of its own (a secondary source).
            yield make_row("user", initial, "")
        elif initial and not self._session_collected(artifact, record):
            # Code tab: its transcript is claude-code's, another agent, so
            # the rule could not pair the two; only when it is absent.
            yield make_row("user", initial)
        if record.get("error"):
            category = _str(record.get("errorCategory"))
            error = _str(record.get("error"))
            yield make_row(
                "system",
                "session error: %s" % ("%s: %s" % (category, error) if category else error),
                to_utc(record.get("errorAt")),
            )

    @staticmethod
    def _grants(record: dict, make_row, created: str) -> Iterator[Row]:
        """Yield one `permission grant:` row per grant entry or key of a session record.

        The record keeps no time per grant, so the rows take `lastActivityAt` (the grant held
        by then), else `createdAt`, and say which.
        """
        last = to_utc(record.get("lastActivityAt"))
        timestamp, field = (last, "lastActivityAt") if last else (created, "createdAt")
        for key in GRANT_LISTS:
            entries = record.get(key)
            if isinstance(entries, list):
                for entry in entries:
                    yield make_row(
                        "system",
                        "permission grant: %s %s (as of %s)" % (key, _value(entry), field),
                        timestamp,
                    )
        for key in GRANT_KEYS:
            value = record.get(key)
            if value is None or value == "" or value == [] or value == {}:
                continue
            yield make_row(
                "system",
                "permission grant: %s=%s (as of %s)" % (key, _value(value), field),
                timestamp,
            )
        for key in COWORK_GRANTS:
            value = record.get(key)
            if isinstance(value, list):
                for entry in value:
                    yield make_row(
                        "system",
                        "permission grant: %s %s (as of %s)" % (key, _value(entry), field),
                        timestamp,
                    )
            elif isinstance(value, dict):
                for k, entry in value.items():
                    yield make_row(
                        "system",
                        "permission grant: %s.%s=%s (as of %s)" % (key, k, _value(entry), field),
                        timestamp,
                    )
            elif value is not None and value != "":
                yield make_row(
                    "system",
                    "permission grant: %s=%s (as of %s)" % (key, _value(value), field),
                    timestamp,
                )

    @staticmethod
    def _session_collected(artifact: Artifact, record: dict) -> bool:
        """Return whether a transcript or audit log of the record's session is in the input.

        It is looked for in the session dir (full or short name, beside the record or in its
        org dir), the org's `imported-staging/`, and for the Code tab `~/.claude/projects/*/`.
        """
        record_dir = artifact.disk_path.parent
        org = record_dir.parent if record_dir.name == "agent" else record_dir
        session_id = _str(record.get("sessionId")) or artifact.disk_path.stem
        names = [session_id]
        if LONG_PATTERN.match(session_id):
            names.append(session_id[len("local_") : len("local_") + 8])
        for directory in dict.fromkeys((record_dir, org)):
            for name in names:
                session_dir = directory / name
                if not _plain_dir(session_dir):
                    continue
                if _plain_file(session_dir / "audit.jsonl") or _has_transcript(session_dir):
                    return True
        cli_id = _str(record.get("cliSessionId"))
        if cli_id:
            staging = org / "imported-staging"
            if _plain_dir(staging) and _plain_file(staging / (cli_id + ".jsonl")):
                return True
            claude = artifact.home.disk_path / ".claude"
            if _plain_dir(claude) and _jsonl_files(claude / "projects", cli_id + ".jsonl"):
                return True
        return False

    # -- scheduled tasks and worktrees -----------------------------------------

    def _scheduled(self, artifact: Artifact) -> Iterator[Row]:
        errors: list = []
        doc = _read_json(artifact.disk_path, errors)
        if errors:
            yield from error_rows(self, artifact, errors)
            return
        if not isinstance(doc, dict):
            yield self._system(artifact, "parser: unreadable scheduled-tasks.json", 1)
            return
        # A fresh install's file has the markers but no task list yet.
        tasks = doc.get("scheduledTasks")
        if not isinstance(tasks, list):
            return
        for i, task in enumerate(tasks, 1):
            if not isinstance(task, dict):
                continue
            when = task.get("cronExpression") or to_utc(task.get("fireAt"))
            row = self._system(
                artifact,
                "scheduled task: %s %s enabled=%s permission_mode=%s model=%s"
                % (
                    _str(task.get("id")),
                    _str(when),
                    _bool(task.get("enabled")),
                    _str(task.get("permissionMode")) or "-",
                    _str(task.get("model")) or "-",
                ),
                i,
            )
            row.timestamp_utc = to_utc(task.get("createdAt"))
            row.model = _str(task.get("model"))
            row.project_path = _str(task.get("cwd")) or _folder(task)
            yield row

    def _worktrees(self, artifact: Artifact) -> Iterator[Row]:
        errors: list = []
        doc = _read_json(artifact.disk_path, errors)
        if errors:
            yield from error_rows(self, artifact, errors)
            return
        if not isinstance(doc, dict):
            yield self._system(artifact, "parser: unreadable git-worktrees.json", 1)
            return
        # A fresh install's file is `{"schemaVersion": 2}`, with no `worktrees`
        # key until the Code tab creates one.
        trees = doc.get("worktrees")
        if not isinstance(trees, dict):
            return
        for i, worktree in enumerate(trees.values(), 1):
            if not isinstance(worktree, dict):
                continue
            text = "worktree created: %s branch=%s from=%s" % (
                _str(worktree.get("path")),
                _str(worktree.get("branch")),
                _str(worktree.get("sourceBranch")),
            )
            if worktree.get("leasedBy"):
                text += " leased by %s" % worktree.get("leasedBy")
            row = self._system(artifact, text, i)
            row.timestamp_utc = to_utc(worktree.get("createdAt"))
            row.project_path = _str(worktree.get("baseRepo"))
            row.git_branch = _str(worktree.get("branch"))
            yield row

    # -- remote-session-spaces.json --------------------------------------------

    def _spaces(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `permission setting:` row per folder granted to a remote Cowork session.

        The folders are `entries[].folders[]`, keyed by the entry's `sessionId`, the
        server's id, which is kept in the text; `session_id` stays empty.
        """
        doc, bad = self._settings_doc(artifact, "remote-session-spaces.json")
        if doc is None:
            if bad is not None:
                yield bad
            return
        entries = doc.get("entries")
        if not isinstance(entries, list):
            return
        timestamp = file_mtime(artifact)
        for entry in entries:
            if not isinstance(entry, dict):
                continue
            folders = entry.get("folders")
            if not isinstance(folders, list):
                continue
            session_id = _str(entry.get("sessionId"))
            key = "entries.%s.folders" % session_id if session_id else "entries.folders"
            for folder in folders:
                if folder in (None, ""):
                    continue
                yield self._standing_row(artifact, timestamp, _str(folder), key, folder)
