"""Letta Code transcripts, settings, channel routes, scheduled tasks and interrupted turns.

Letta keeps conversations on the Letta server by default; these local files are read, under
`.letta/` unless said:

* `lc-local-backend/conversations/<base64url key>/messages.jsonl` (local backend): pi's session
  format, read with `pi.read_session` and `pi.entry_rows` (header `id`, `cwd`; entries
  `message`, `compaction`); only the last copy of each `message.id` is kept. Schema 1 files
  (one bare message per line, no header) are timed by `message.metadata.created_at`.
* `transcripts/<agent>/<conversation>/transcript.jsonl` (secondary): `kind` (`user`,
  `assistant`, `reasoning`, `error`, `tool_call` with `name`, `argsText`, `resultText`,
  `resultOk`), `text` and `captured_at`, stamped once per turn when the turn ends, hence
  `TURN_TOLERANCE_SECONDS`. The project comes from `sessions.jsonl` by time, a heuristic.
* `sessions.jsonl`: `agent_id`, `session_id`, `timestamp` (ms), `project`, `model`,
  `provider`, `message_count`, `tool_call_count`, `exit_reason`, `duration.wall_ms`.
* `settings.json`, the legacy `.config/letta/settings.json` and a project's
  `.letta/settings.json` and `.letta/settings.local.json`: `sessionsByServer` (`agentId`,
  `conversationId`), `permissions` (`PERMISSION_KEYS`) and `agents[]` (`agentId`, `baseUrl`).
  No other key is read: `env` and the token keys hold credentials.
* `channels/<channel>/routing.{json,yaml}` `routes[]`, `crons.json` `tasks[]`,
  `runs/<task id>.jsonl` and `listener-state/<24 hex>/<agent>_<conversation>.json` (`agentId`,
  `conversationId`, `runId`, `toolCallIds`, `unstartedToolCallIds`, `results`,
  `workingDirectory`, `actingUserId`); the row texts name every field they read.

A transcript of an agent that does not start `agent-local-` is a server conversation and gets
a `session synced:` row. Validated against source, letta-ai/letta-code at 77faf36e and
2a46615d (0.34.17), with a synthetic fixture; no real install. See `research/letta.md`.
"""

from __future__ import annotations

import base64
import binascii
import json
import re
from collections.abc import Iterator

from ..inputs import Artifact
from ..model import Row, compact
from ..timeutil import to_utc
from .base import (
    Options,
    Parser,
    error_rows,
    file_bytes,
    file_mtime,
    iter_jsonl,
    setting_value,
    text_of,
)
from .pi import SessionFile, args_summary, entry_rows, model_text, read_session

TRANSCRIPT_PATTERN = re.compile(r"^\.letta/transcripts/([^/]+)/([^/]+)/transcript\.jsonl$")
LOCAL_PATTERN = re.compile(r"^\.letta/lc-local-backend/conversations/([^/]+)/messages\.jsonl$")
SESSIONS_REL = ".letta/sessions.jsonl"
SETTINGS_REL = ".letta/settings.json"
LOCAL_SETTINGS_REL = ".letta/settings.local.json"
LEGACY_SETTINGS_REL = ".config/letta/settings.json"
CRONS_REL = ".letta/crons.json"
ROUTING_PATTERN = re.compile(r"^\.letta/channels/([^/]+)/routing\.(?:json|yaml)$")
RUNS_PATTERN = re.compile(r"^\.letta/runs/[^/]+\.jsonl$")
LISTENER_PATTERN = re.compile(r"^\.letta/listener-state/[0-9a-f]{24}/[^/]+\.json$")
# The keys of a settings file's `permissions` object (PermissionRules).
PERMISSION_KEYS = ("mode", "allow", "deny", "ask", "alwaysAsk", "additionalDirectories")
LOCAL_AGENT_PREFIX = "agent-local-"
LETTA_CLOUD = "api.letta.com"
# The window a transcript line may lag its messages.jsonl twin: a turn's
# whole duration, since captured_at is stamped at the turn's end.
TURN_TOLERANCE_SECONDS = 3600.0


def decode_key(name: str) -> str:
    """Return `conversation:<id>` or `default:<agent id>` from a base64url name, or `""`."""
    try:
        return base64.urlsafe_b64decode(name + "=" * (-len(name) % 4)).decode("utf-8")
    except (binascii.Error, ValueError, UnicodeDecodeError):
        return ""


def sanitize(segment: str) -> str:
    """Return the transcript directory name Letta derives from an id."""
    name = re.sub(r"[^a-zA-Z0-9._-]", "_", segment).strip()
    return name or "unknown"


def _args_text(args) -> str:
    """Return a transcript `argsText` as the pi reader renders the same call's arguments.

    That is `args_summary` of the parsed JSON, else the text itself.
    """
    if not isinstance(args, str):
        return args_summary(args)
    try:
        return args_summary(json.loads(args))
    except ValueError:
        return args


def read_settings(path, errors: list) -> dict | None:
    """Return a Letta settings file as a dict, or None when it is unreadable or not an object.

    A file that cannot be read is its unreadable entry in `errors` (`file_bytes`); one that
    is not a JSON object leaves `errors` as it was.
    """
    content = file_bytes(path, errors)
    if content is None:
        return None
    try:
        document = json.loads(content.decode("utf-8", errors="replace"))
    except ValueError:
        return None
    return document if isinstance(document, dict) else None


def _value(value) -> str:
    """Return a record value as row text, `-` when absent or empty."""
    if value is None or value == "":
        return "-"
    return setting_value(value)


def _bare_messages(session: SessionFile) -> SessionFile:
    """Wrap the bare pi-ai messages of a schema 1 file as session entries."""
    out = SessionFile()
    out.errors = session.errors
    for line_number, record in session.entries:
        meta = record.get("metadata")
        if not isinstance(meta, dict):
            meta = {}
        out.entries.append(
            (
                line_number,
                {"type": "message", "timestamp": meta.get("created_at"), "message": record},
            )
        )
    return out


def _last_snapshots(session: SessionFile) -> SessionFile:
    """Keep only the last entry for each `message.id`."""
    last: dict[str, int] = {}
    for i, (_, record) in enumerate(session.entries):
        message = record.get("message")
        if isinstance(message, dict) and message.get("id"):
            last[str(message.get("id"))] = i
    out = SessionFile()
    out.header, out.header_line, out.errors = session.header, session.header_line, session.errors
    for i, (line_number, record) in enumerate(session.entries):
        message = record.get("message")
        if isinstance(message, dict) and message.get("id") and last[str(message.get("id"))] != i:
            continue
        out.entries.append((line_number, record))
    return out


def _session_project(sessions: list[tuple[str, str]], timestamp: str) -> str:
    """Return the project of a transcript line captured at `timestamp`.

    That of the agent's earliest `sessions.jsonl` line at or after it (the exit line of the CLI
    session the turn belongs to), else, when no line is later, that of the latest line before it.
    """
    if not timestamp:
        return ""
    before = ""
    for session_time, project in sessions:
        if session_time >= timestamp:
            return project
        before = project
    return before


class LettaParser(Parser):
    """Parse Letta Code's local conversations, transcripts, settings and task records."""

    agent = "letta"
    name = "letta"
    has_secondary = True
    records_sync = True
    records_relay = True

    def wants(self, artifact: Artifact) -> bool:
        """Return True for the Letta files this parser reads."""
        rel = artifact.rel
        return (
            rel in (SESSIONS_REL, SETTINGS_REL, LOCAL_SETTINGS_REL, LEGACY_SETTINGS_REL, CRONS_REL)
            or bool(TRANSCRIPT_PATTERN.match(rel))
            or bool(LOCAL_PATTERN.match(rel))
            or bool(ROUTING_PATTERN.match(rel))
            or bool(RUNS_PATTERN.match(rel))
            or bool(LISTENER_PATTERN.match(rel))
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for the transcripts, which repeat the local backend's messages."""
        return bool(TRANSCRIPT_PATTERN.match(artifact.rel))

    def secondary_tolerance(self, artifact: Artifact) -> float:
        """Return the window a transcript line may lag its `messages.jsonl` twin."""
        return TURN_TOLERANCE_SECONDS

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Letta file."""
        if artifact.rel == SESSIONS_REL:
            yield from self._parse_sessions(artifact, opts)
            return
        if artifact.rel in (SETTINGS_REL, LOCAL_SETTINGS_REL, LEGACY_SETTINGS_REL):
            yield from self._parse_settings(artifact)
            return
        if artifact.rel == CRONS_REL:
            yield from self._parse_crons(artifact)
            return
        if RUNS_PATTERN.match(artifact.rel):
            yield from self._parse_runs(artifact)
            return
        if LISTENER_PATTERN.match(artifact.rel):
            yield from self._parse_interrupted(artifact)
            return
        route = ROUTING_PATTERN.match(artifact.rel)
        if route:
            yield from self._parse_routing(artifact, route.group(1))
            return
        match = LOCAL_PATTERN.match(artifact.rel)
        if match:
            yield from self._parse_local(artifact, match.group(1), opts)
            return
        match = TRANSCRIPT_PATTERN.match(artifact.rel)
        assert match is not None  # wants() accepted only matching paths
        yield from self._parse_transcript(artifact, match.group(1), match.group(2), opts)

    # -- local backend ----------------------------------------------------

    def _parse_local(self, artifact: Artifact, dirname: str, opts: Options) -> Iterator[Row]:
        session = read_session(artifact.disk_path)
        key = decode_key(dirname)
        session_id = str(session.header.get("id") or key.partition(":")[2])
        cwd = str(session.header.get("cwd") or "")
        if (
            not session.header
            and session.entries
            and all(
                "role" in record and record.get("type") != "message"
                for _, record in session.entries
            )
        ):
            session = _bare_messages(session)
        else:
            session = _last_snapshots(session)
        if session.header:
            row = self.base_row(artifact)
            row.source_line = session.header_line
            row.timestamp_utc = to_utc(session.header.get("timestamp"))
            row.session_id, row.project_path, row.turn_type = session_id, cwd, "system"
            row.text = compact(
                "letta local-backend conversation start: %s cwd=%s" % (key or session_id, cwd),
            )
            yield row
        yield from entry_rows(self, artifact, session, opts, session_id, cwd)
        yield from error_rows(self, artifact, session.errors, session_id, cwd)

    # -- reflection transcript ------------------------------------------

    def _sessions(self, artifact: Artifact, agent_dir: str) -> list[tuple[str, str]]:
        """Return (timestamp, project) of this agent's `sessions.jsonl` lines, sorted by time."""
        sessions_path = artifact.home.disk_path / SESSIONS_REL
        if (
            sessions_path.is_symlink()
            or not sessions_path.is_file()
            or (artifact.home.disk_path / ".letta").is_symlink()
        ):
            return []
        out = []
        for _, record in iter_jsonl(sessions_path, []):
            if sanitize(str(record.get("agent_id") or "")) == agent_dir and record.get("project"):
                timestamp = to_utc(record.get("timestamp"))
                if timestamp:
                    out.append((timestamp, str(record.get("project"))))
        return sorted(out)

    @staticmethod
    def _server(artifact: Artifact, agent_dir: str) -> str:
        """Return the server of a transcript's agent.

        The `baseUrl` of its `agents[]` entry in `~/.letta/settings.json`, `api.letta.com` when the
        entry has none, `-` when there is no entry or no file.
        """
        settings_path = artifact.home.disk_path / SETTINGS_REL
        if (
            settings_path.is_symlink()
            or not settings_path.is_file()
            or (artifact.home.disk_path / ".letta").is_symlink()
        ):
            return "-"
        # A pre-scan: this parser reports the settings file when it parses that file itself.
        agents = (read_settings(settings_path, []) or {}).get("agents")
        for entry in agents if isinstance(agents, list) else []:
            if isinstance(entry, dict) and sanitize(str(entry.get("agentId") or "")) == agent_dir:
                return str(entry.get("baseUrl") or LETTA_CLOUD)
        return "-"

    def _parse_transcript(
        self, artifact: Artifact, agent_dir: str, conversation_dir: str, opts: Options
    ) -> Iterator[Row]:
        sessions = self._sessions(artifact, agent_dir)
        server_side = not agent_dir.startswith(LOCAL_AGENT_PREFIX)
        errors: list = []
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            timestamp = to_utc(record.get("captured_at"))
            project = _session_project(sessions, timestamp)

            def base() -> Row:
                row = self.base_row(artifact)
                row.source_line = line_number
                row.timestamp_utc = timestamp
                row.session_id = conversation_dir
                row.project_path = project
                return row

            if server_side:
                server_side = False
                row = base()
                row.turn_type = "system"
                row.text = compact(
                    "session synced: server conversation %s agent %s server %s "
                    "(from .letta/transcripts path, first captured_at)"
                    % (conversation_dir, agent_dir, self._server(artifact, agent_dir))
                )
                yield row

            kind = record.get("kind")
            text = str(record.get("text") or "")
            if kind in ("user", "assistant"):
                row = base()
                row.turn_type = kind
                row.text = compact(text)
                yield row
            elif kind == "reasoning":
                if opts.include_thinking:
                    row = base()
                    row.turn_type = "thinking"
                    row.text = compact(text)
                    yield row
            elif kind == "error":
                row = base()
                row.turn_type = "system"
                row.text = compact("error: " + text)
                yield row
            elif kind == "tool_call":
                name = str(record.get("name") or "")
                row = base()
                row.turn_type, row.tool_name = "tool_use", name
                row.text = compact(_args_text(record.get("argsText")))
                yield row
                if "resultText" in record or "resultOk" in record:
                    result = str(record.get("resultText") or "")
                    if record.get("resultOk") is False:
                        result = "[error] " + result
                    row = base()
                    row.turn_type, row.tool_name = "tool_result", name
                    row.text = compact(result)
                    yield row
        yield from error_rows(self, artifact, errors, conversation_dir)

    # -- settings.json and settings.local.json ---------------------------

    def _parse_settings(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the sync and permission rows of a settings file, at its modification time.

        One sync row per `sessionsByServer` entry whose key does not start `local:` (not in the
        legacy file, which Letta reads only for its permissions), then one permission row per entry
        of `permissions`; one `parser:` row when the file is not a JSON object.
        """
        errors: list = []
        document = read_settings(artifact.disk_path, errors)
        project = artifact.home.original if artifact.home.project else ""
        if errors:
            yield from error_rows(self, artifact, errors, "", project)
            return
        timestamp = file_mtime(artifact)

        def base() -> Row:
            row = self.base_row(artifact)
            row.source_line, row.turn_type, row.timestamp_utc = 1, "system", timestamp
            row.project_path = project
            return row

        if document is None:
            row = base()
            row.text = "parser: %s is not a JSON object" % artifact.rel
            yield row
            return
        servers = document.get("sessionsByServer")
        if artifact.rel == LEGACY_SETTINGS_REL or not isinstance(servers, dict):
            servers = {}
        for key, entry in servers.items():
            if str(key).startswith("local:"):
                continue
            entry = entry if isinstance(entry, dict) else {}
            row = base()
            row.session_id = str(entry.get("conversationId") or "")
            row.text = compact(
                "session synced: last session on %s agent %s conversation %s "
                "(from %s, file mtime)"
                % (
                    key,
                    entry.get("agentId") or "-",
                    entry.get("conversationId") or "-",
                    artifact.rel,
                )
            )
            yield row
        permissions = document.get("permissions")
        if not isinstance(permissions, dict):
            permissions = {}
        for key in PERMISSION_KEYS:
            value = permissions.get(key)
            entries = value if isinstance(value, list) else [] if value is None else [value]
            for entry in entries:
                row = base()
                row.text = compact(
                    "permission setting: permissions.%s=%s (from %s, file mtime)"
                    % (key, setting_value(entry), artifact.rel)
                )
                yield row

    # -- channels/<channel>/routing.json -----------------------------------

    def _parse_routing(self, artifact: Artifact, channel: str) -> Iterator[Row]:
        """Yield one `session relayed:` row per route, at its `createdAt`, else the file's mtime."""
        errors: list = []
        document = read_settings(artifact.disk_path, errors)
        if errors:
            yield from error_rows(self, artifact, errors)
            return
        routes = document.get("routes") if document is not None else None
        if not isinstance(routes, list):
            row = self.base_row(artifact)
            row.source_line, row.turn_type = 1, "system"
            row.text = "parser: %s is not a JSON object with a routes list" % artifact.rel
            yield row
            return
        mtime = file_mtime(artifact)
        for route in routes:
            if not isinstance(route, dict):
                continue
            created = to_utc(route.get("createdAt"))
            row = self.base_row(artifact)
            row.source_line, row.turn_type = 1, "system"
            row.timestamp_utc = created or mtime
            row.session_id = str(route.get("conversationId") or "")
            row.text = compact(
                "session relayed: %s chat %s type %s thread %s account %s agent %s "
                "enabled=%s outbound=%s detached=%s updated=%s (from %s, %s)"
                % (
                    channel,
                    _value(route.get("chatId")),
                    _value(route.get("chatType")),
                    _value(route.get("threadId")),
                    _value(route.get("accountId")),
                    _value(route.get("agentId")),
                    _value(route.get("enabled")),
                    _value(route.get("outboundEnabled")),
                    _value(route.get("detached")),
                    _value(to_utc(route.get("updatedAt"))),
                    artifact.rel,
                    "createdAt" if created else "file mtime",
                )
            )
            yield row

    # -- crons.json and runs/<task id>.jsonl ---------------------------------

    def _parse_crons(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `scheduled task:` row per task, at its `created_at`, else the file's mtime."""
        errors: list = []
        document = read_settings(artifact.disk_path, errors)
        if errors:
            yield from error_rows(self, artifact, errors)
            return
        if document is None:
            row = self.base_row(artifact)
            row.source_line, row.turn_type = 1, "system"
            row.text = "parser: %s is not a JSON object" % artifact.rel
            yield row
            return
        tasks = document.get("tasks")
        mtime = file_mtime(artifact)
        for task in tasks if isinstance(tasks, list) else ():
            if not isinstance(task, dict):
                continue
            created = to_utc(task.get("created_at"))
            conversation = str(task.get("conversation_id") or "")
            row = self.base_row(artifact)
            row.source_line, row.turn_type = 1, "system"
            row.timestamp_utc = created or mtime
            row.session_id = "" if conversation == "new" else conversation
            row.text = compact(
                "scheduled task: %s name=%s status=%s cron=%s timezone=%s recurring=%s "
                "scheduled_for=%s agent=%s conversation=%s fire_count=%s last_fired=%s "
                "last_run=%s outcome=%s reason=%s error=%s cancel_reason=%s "
                "(from %s, %s) description: %s prompt: %s"
                % (
                    _value(task.get("id")),
                    _value(task.get("name")),
                    _value(task.get("status")),
                    _value(task.get("cron")),
                    _value(task.get("timezone")),
                    _value(task.get("recurring")),
                    _value(to_utc(task.get("scheduled_for"))),
                    _value(task.get("agent_id")),
                    _value(conversation),
                    _value(task.get("fire_count")),
                    _value(to_utc(task.get("last_fired_at"))),
                    _value(to_utc(task.get("last_run_at"))),
                    _value(task.get("last_run_outcome")),
                    _value(task.get("last_run_reason")),
                    _value(task.get("last_run_error")),
                    _value(task.get("cancel_reason")),
                    artifact.rel,
                    "created_at" if created else "file mtime",
                    _value(task.get("description")),
                    _value(task.get("prompt")),
                )
            )
            yield row

    def _parse_runs(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `scheduled task run:` row per line, at its `ts`."""
        errors: list = []
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            row = self.base_row(artifact)
            row.source_line, row.turn_type = line_number, "system"
            row.timestamp_utc = to_utc(record.get("ts"))
            row.session_id = str(record.get("conversationId") or "")
            row.text = compact(
                "scheduled task run: %s action=%s status=%s outcome=%s reason=%s agent=%s "
                "conversation=%s run=%s scheduled_for=%s fired=%s missed=%s summary=%s "
                "error=%s (from %s, ts)"
                % (
                    _value(record.get("jobId")),
                    _value(record.get("action")),
                    _value(record.get("status")),
                    _value(record.get("outcome")),
                    _value(record.get("reason")),
                    _value(record.get("agentId")),
                    _value(record.get("conversationId")),
                    _value(record.get("runId")),
                    _value(to_utc(record.get("scheduledFor"))),
                    _value(to_utc(record.get("firedAt"))),
                    _value(record.get("missedCount")),
                    _value(record.get("summary")),
                    _value(record.get("error")),
                    artifact.rel,
                )
            )
            yield row
        yield from error_rows(self, artifact, errors)

    # -- listener-state/<hash>/<agent>_<conversation>.json -------------------

    def _parse_interrupted(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the rows of an interrupted remote-listener turn, at the file's modification time.

        One `interrupted turn:` row, then per `results` entry a `permission decision:` row (an
        `ApprovalReturn`, with `approve`) or a `tool_result` row (a `ToolReturn`, with `status`),
        joined on `tool_call_id`; the record holds no time.
        """
        errors: list = []
        document = read_settings(artifact.disk_path, errors)
        if errors:
            yield from error_rows(self, artifact, errors)
            return
        timestamp = file_mtime(artifact)
        if document is None:
            row = self.base_row(artifact)
            row.source_line, row.turn_type, row.timestamp_utc = 1, "system", timestamp
            row.text = "parser: %s is not a JSON object" % artifact.rel
            yield row
            return
        session_id = str(document.get("conversationId") or "")
        cwd = str(document.get("workingDirectory") or "")

        def base() -> Row:
            row = self.base_row(artifact)
            row.source_line, row.turn_type, row.timestamp_utc = 1, "system", timestamp
            row.session_id, row.project_path = session_id, cwd
            return row

        def ids(key: str) -> str:
            value = document.get(key)
            items = [str(entry) for entry in value] if isinstance(value, list) else []
            return ",".join(items) or "-"

        results = document.get("results")
        results = (
            [entry for entry in results if isinstance(entry, dict)]
            if isinstance(results, list)
            else []
        )
        row = base()
        row.text = compact(
            "interrupted turn: agent %s conversation %s run %s tool_calls=%s unstarted=%s "
            "results=%d working_directory=%s acting_user=%s (from %s, file mtime)"
            % (
                _value(document.get("agentId")),
                _value(session_id),
                _value(document.get("runId")),
                ids("toolCallIds"),
                ids("unstartedToolCallIds"),
                len(results),
                _value(cwd),
                _value(document.get("actingUserId")),
                artifact.rel,
            )
        )
        yield row
        for result in results:
            row = base()
            row.tool_use_id = str(result.get("tool_call_id") or "")
            if "approve" in result:
                row.text = compact(
                    "permission decision: %s reason=%s (from %s, interrupted turn, file mtime)"
                    % (
                        "approve" if result.get("approve") is True else "deny",
                        _value(result.get("reason")),
                        artifact.rel,
                    )
                )
            else:
                text = text_of(result.get("tool_return"))
                row.turn_type = "tool_result"
                row.text = compact(("[error] " if result.get("status") == "error" else "") + text)
            yield row

    # -- sessions.jsonl --------------------------------------------------

    def _parse_sessions(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield one `letta session` row per `sessions.jsonl` line, then its model row."""
        errors: list = []
        seen: dict = {}  # the model per CLI session, for the `model:` rows
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            row = self.base_row(artifact)
            row.source_line = line_number
            row.timestamp_utc = to_utc(record.get("timestamp"))
            row.session_id = str(record.get("session_id") or "")
            row.project_path = str(record.get("project") or "")
            row.model = str(record.get("model") or "")
            row.turn_type = "system"
            duration = record.get("duration")
            if not isinstance(duration, dict):
                duration = {}
            if "exit_reason" in record or "message_count" in record:
                text = (
                    "letta session end: agent=%s exit_reason=%s messages=%s tool_calls=%s wall_ms=%s"
                    % (
                        record.get("agent_id") or "",
                        record.get("exit_reason") or "",
                        record.get("message_count", ""),
                        record.get("tool_call_count", ""),
                        duration.get("wall_ms", ""),
                    )
                )
            else:
                text = "letta session start: agent=%s provider=%s model=%s project=%s" % (
                    record.get("agent_id") or "",
                    record.get("provider") or "",
                    row.model,
                    row.project_path,
                )
            row.text = compact(text)
            yield row
            model_message = model_text(
                seen, row.session_id, row.model, str(record.get("provider") or "")
            )
            if model_message:
                model_row = self.base_row(artifact)
                model_row.source_line, model_row.turn_type = line_number, "system"
                model_row.timestamp_utc, model_row.session_id = row.timestamp_utc, row.session_id
                model_row.project_path, model_row.model = row.project_path, row.model
                model_row.text = compact(model_message)
                yield model_row
        yield from error_rows(self, artifact, errors)
