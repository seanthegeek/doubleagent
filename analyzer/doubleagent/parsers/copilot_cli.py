"""Parse the session state of the GitHub Copilot CLI (`copilot`).

Reads, under `.copilot` (or `.local/state/.copilot` and `.config/.copilot` on installs from
before the XDG move): `session-state/<id>/events.jsonl` (the primary store),
`session-state/<id>/workspace.yaml`, and as secondary sources `session-store.db`,
`session-state/<id>/rewind-file-snapshots/index.json` and `command-history-state.json`; the
standing grants of `permissions-config.json` and `settings.json` (JSON with comments);
`sidebar-sessions-state/*.json` (a session-to-cwd join, no rows); and `logs/process-*.log`.

Fields the parser depends on:

- event envelope: `type`, `timestamp`, `data`, `agentId` (`parentId` is not read: a rewind
  truncates the file); `data`: `context.cwd`, `context.branch`, `copilotVersion`, `producer`,
  `content`, `role`, `source`, `isAutopilotContinuation`, `model`, `reasoningText`,
  `reasoningBlocks`, `toolRequests[]`, `toolCallId`, `toolName`, `arguments`, `success`,
  `result`, `error`, `shellExecution.exitCode`, `permissionRequest`, `permissionMode`,
  `agentMode`, `decisionSource`, `previousMode`, `mode`, `newMode`, `newModel`,
  `previousModel`, `cause`, `reasoningEffort`, `chosenModel`, `routingMethod`, `infoType`,
  `message`, `url`, `tip`, `modelCall.model`, `summaryContent`, `reason`, `hookType`, `input`,
  `output`, `parentToolCallId`, `shutdownType`, `errorReason`, `codeChanges.filesModified`;
- `workspace.yaml` top-level scalars: `id`, `cwd`, `name`, `user_named`, `fork_count`,
  `summary_count`, `created_at`, `client_name`, `mc_session_id`, `mc_task_id`,
  `mc_last_event_id`, `remote_steerable`;
- `session-store.db` tables `sessions`, `turns`, `session_files`, `session_refs`,
  `checkpoints` and `forge_trajectory_events` (every column possibly NULL).

Validated against the shipped event schema, bundle and runtime addon of
`@github/copilot-darwin-arm64` 1.0.98 and real installs. The Copilot Desktop app's bundled CLI
1.0.95-3 writes `session.permissions_changed` with the `ALLOW_ALL_KEYS` instead of `mode`. See
`research/copilot-cli.md`.
"""

from __future__ import annotations

import json
import os
import re
import sqlite3
from collections.abc import Callable, Iterator
from pathlib import Path

from ..inputs import Artifact
from ..model import SCRATCH_PREFIX, Row, compact
from ..sqlite_util import is_sqlite, open_copy, table_names
from ..textlines import JSON_SPACE, SPACE_OR_TAB, split_lines
from ..timeutil import to_utc
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_mtime,
    iter_jsonl,
    setting_value,
    track_change,
)

HOME = r"(?:\.copilot|\.local/state/\.copilot|\.config/\.copilot)"
SESSION_DIR = HOME + r"/session-state/([0-9a-f-]{36})/"
EVENTS_PATTERN = re.compile("^" + SESSION_DIR + r"events\.jsonl$")
WORKSPACE_PATTERN = re.compile("^" + SESSION_DIR + r"workspace\.yaml$")
SNAPSHOTS_PATTERN = re.compile("^" + SESSION_DIR + r"rewind-file-snapshots/index\.json$")
STORE_PATTERN = re.compile("^(" + HOME + r")/session-store\.db$")
HISTORY_PATTERN = re.compile("^" + HOME + r"/command-history-state\.json$")
PERMISSIONS_PATTERN = re.compile("^" + HOME + r"/permissions-config\.json$")
SETTINGS_PATTERN = re.compile("^" + HOME + r"/settings\.json$")
SIDEBAR_PATTERN = re.compile("^" + HOME + r"/sidebar-sessions-state/[0-9a-f]{64}\.json$")
LOG_PATTERN = re.compile("^" + HOME + r"/logs/process-\d+-\d+\.log$")
HOME_PREFIX_PATTERN = re.compile("^(" + HOME + ")/")

# `turns.timestamp` and the snapshot time are stamped at the end of a turn
# (or at a time not measured against the prompt's), so the prompt and the
# reply of the log are up to a turn's length earlier.
TURN_TOLERANCE_SECONDS = 3600.0

ARG_KEYS = ("command", "url", "query", "path", "input")
NOTE_TYPES = (
    "session.task_complete",
    "system.notification",
    "session.schedule_created",
    "skill.invoked",
    "session.handoff",
)
NOTE_PREFIXES = ("subagent.",)
# The Copilot Desktop app's form of `session.permissions_changed`.
ALLOW_ALL_KEYS = (
    "previousAllowAllPermissions",
    "allowAllPermissions",
    "previousAllowAllPermissionMode",
    "allowAllPermissionMode",
)
NOTE_KEYS = ("summary", "content", "prompt", "message", "reason", "name", "agentName")
# `permission.completed` result keys appended to the decision row, in order.
DECISION_KEYS = (
    ("locationKey", "location"),
    ("approval", "approval"),
    ("directories", "directories"),
    ("feedback", "feedback"),
    ("rules", "rules"),
    ("reason", "reason"),
    ("path", "path"),
    ("message", "message"),
)
CHECKPOINT_COLUMNS = (
    "overview",
    "history",
    "work_done",
    "technical_details",
    "important_files",
    "next_steps",
)
SIGNED_IN_PATTERN = re.compile(r"Signed in to (\S+) as (.+?)\.?[ \t]*$")
WORKSPACE_LINE_PATTERN = re.compile(r"Workspace initialized: ([0-9a-f-]{36})")
LOG_TIME_PATTERN = re.compile(r"^([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z)[ \t]")


def _dict(value) -> dict:
    return value if isinstance(value, dict) else {}


def _str(value) -> str:
    if value is None:
        return ""
    return value if isinstance(value, str) else compact_json(value)


def args_text(arguments) -> str:
    """Return the `tool_use` text of a call's arguments.

    A string argument (a `custom` grammar tool such as `apply_patch`) is kept as it is, else the
    first of `ARG_KEYS` is taken, else compact JSON.
    """
    if isinstance(arguments, str):
        return arguments
    if isinstance(arguments, dict):
        for key in ARG_KEYS:
            value = arguments.get(key)
            if value not in (None, ""):
                return _str(value)
        return compact_json(arguments) if arguments else ""
    return _str(arguments)


def result_text(data: dict) -> str:
    """Return the `tool_result` text of a `tool.execution_complete` event.

    `detailedContent` is the complete result and `content` the concise one sent to the model,
    except that a successful skill invocation puts the concise text in `detailedContent`, so the
    longer of the two is taken, `detailedContent` on a tie.
    """
    result = _dict(data.get("result"))
    detailed = _str(result.get("detailedContent"))
    concise = _str(result.get("content"))
    text = concise if len(concise) > len(detailed) else detailed
    code = _dict(data.get("shellExecution")).get("exitCode")
    if code not in (None, 0, "0") and not isinstance(code, bool):
        text = "[exit %s] %s" % (code, text)
    if data.get("success") is False:
        message = _str(_dict(data.get("error")).get("message"))
        if message:
            text = text + "\n" + message if text else message
    return text


def request_target(request: dict) -> str:
    """Return what a `permissionRequest` asks about, or "".

    It is the command, URL, file or path, or `<server>/<tool>` for an MCP tool.
    """
    for key in ("fullCommandText", "url", "fileName", "path"):
        value = request.get(key)
        if value not in (None, ""):
            return _str(value)
    server, tool = _str(request.get("serverName")), _str(request.get("toolName"))
    if server or tool:
        return "%s/%s" % (server, tool) if server else tool
    return ""


def decision_text(data: dict) -> str:
    """`permission decision: <kind> source=<decisionSource or -> ...`."""
    result = _dict(data.get("result"))
    text = "permission decision: %s source=%s" % (
        _str(result.get("kind")) or "-",
        _str(data.get("decisionSource")) or "-",
    )
    for key, label in DECISION_KEYS:
        if result.get(key) not in (None, "", [], {}):
            text += " %s=%s" % (label, setting_value(result.get(key)))
    return text


def strip_jsonc(text: str) -> str:
    """Return JSON with `//` and `/* */` comments and trailing commas as plain JSON.

    Strings are copied untouched, so a URL's `//` and a `,]` inside one
    survive. A trailing comma is one followed by JSON whitespace (space, tab,
    LF, CR) and comments only, then `}` or `]`.
    """
    out: list[str] = []
    i, length = 0, len(text)
    while i < length:
        character = text[i]
        if character in "}]":
            last = len(out) - 1
            while last >= 0 and out[last] in JSON_SPACE:
                last -= 1
            if last >= 0 and out[last] == ",":
                del out[last]
            out.append(character)
            i += 1
        elif character == '"':
            j = i + 1
            while j < length and text[j] != '"':
                j += 2 if text[j] == "\\" else 1
            out.append(text[i : j + 1])
            i = j + 1
        elif text.startswith("//", i):
            j = text.find("\n", i)
            i = length if j < 0 else j
        elif text.startswith("/*", i):
            j = text.find("*/", i + 2)
            i = length if j < 0 else j + 2
        else:
            out.append(character)
            i += 1
    return "".join(out)


def read_flat_yaml(path: Path) -> dict[str, str | None]:
    """Return the top-level `key: value` scalars of a YAML file.

    `null` and `~` are None and quoted values are unquoted. `workspace.yaml` holds only scalars, so
    a nested or multi-line value is skipped.
    """
    out: dict[str, str | None] = {}
    try:
        lines = split_lines(path.read_bytes().decode("utf-8", errors="replace"))
    except OSError:
        return out
    for line in lines:
        if not line or line[0] in " \t#-" or ":" not in line:
            continue
        key, _, value = line.partition(":")
        key, value = key.rstrip(SPACE_OR_TAB), value.strip(SPACE_OR_TAB)
        if value in ("", "null", "~"):
            out[key] = None
        elif value.startswith('"'):
            try:
                out[key] = str(json.loads(value))
            except ValueError:
                out[key] = value.strip('"')
        elif value.startswith("'") and value.endswith("'") and len(value) > 1:
            out[key] = value[1:-1].replace("''", "'")
        else:
            out[key] = value
    return out


def _read_json(path: Path):
    try:
        return json.loads(path.read_bytes().decode("utf-8", errors="replace"))
    except (OSError, ValueError):
        return None


class CopilotCliParser(Parser):
    """Parse the Copilot CLI's events, session metadata, stores, grants and logs."""

    agent = "copilot-cli"
    name = "copilot-cli"
    has_secondary = True
    records_sync = True

    def __init__(self) -> None:
        """Start with an empty cache of sidebar session directories."""
        self._sidebar_cache: dict[Path, dict[str, str]] = {}

    def wants(self, artifact: Artifact) -> bool:
        """Return whether the artifact is one of the files this parser reads."""
        rel = artifact.rel
        return any(
            pattern.match(rel)
            for pattern in (
                EVENTS_PATTERN,
                WORKSPACE_PATTERN,
                SNAPSHOTS_PATTERN,
                STORE_PATTERN,
                HISTORY_PATTERN,
                PERMISSIONS_PATTERN,
                SETTINGS_PATTERN,
                SIDEBAR_PATTERN,
                LOG_PATTERN,
            )
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return whether the artifact repeats events of the session logs."""
        rel = artifact.rel
        return bool(
            STORE_PATTERN.match(rel) or HISTORY_PATTERN.match(rel) or SNAPSHOTS_PATTERN.match(rel)
        )

    def secondary_tolerance(self, artifact: Artifact) -> float:
        """Return the pairing window: a turn's length for the store and the snapshots."""
        if STORE_PATTERN.match(artifact.rel) or SNAPSHOTS_PATTERN.match(artifact.rel):
            return TURN_TOLERANCE_SECONDS
        return super().secondary_tolerance(artifact)

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one artifact, by the file it is."""
        rel = artifact.rel
        match = EVENTS_PATTERN.match(rel)
        if match:
            yield from self._parse_events(artifact, match.group(1), opts)
        elif match := WORKSPACE_PATTERN.match(rel):
            yield from self._parse_workspace(artifact, match.group(1))
        elif match := SNAPSHOTS_PATTERN.match(rel):
            yield from self._parse_snapshots(artifact, match.group(1))
        elif STORE_PATTERN.match(rel):
            yield from self._parse_store(artifact)
        elif HISTORY_PATTERN.match(rel):
            yield from self._parse_history(artifact)
        elif PERMISSIONS_PATTERN.match(rel):
            yield from self._parse_permissions(artifact)
        elif SETTINGS_PATTERN.match(rel):
            yield from self._parse_settings(artifact)
        elif LOG_PATTERN.match(rel):
            yield from self._parse_log(artifact)
        # sidebar-sessions-state files give no rows of their own; they are
        # read as a join by `_sidebar`.

    # -- helpers ------------------------------------------------------------

    def _row(self, artifact: Artifact, line: int, turn: str, text: str, **columns) -> Row:
        row = self.base_row(artifact)
        row.source_line = line
        row.turn_type = turn
        row.text = compact(text)
        for field, value in columns.items():
            setattr(row, field, value or "")
        return row

    def _problem(self, artifact: Artifact, text: str, timestamp: str = "") -> Row:
        return self._row(artifact, 1, "system", text, timestamp_utc=timestamp)

    def _sidebar(self, artifact: Artifact) -> dict[str, str]:
        """Return session id -> cwd from the sidebar files of the Copilot home `artifact` is in.

        The result is {} when there are none; symlinks are not read.
        """
        match = HOME_PREFIX_PATTERN.match(artifact.rel)
        if not match:
            return {}
        directory = artifact.home.disk_path / match.group(1) / "sidebar-sessions-state"
        if directory not in self._sidebar_cache:
            found: dict[str, str] = {}
            if directory.is_dir() and not directory.is_symlink():
                for path in sorted(directory.glob("*.json")):
                    if path.is_symlink() or not path.is_file():
                        continue
                    document = _dict(_read_json(path))
                    cwd = _str(document.get("cwd"))
                    ids = document.get("sessionIds")
                    if cwd and isinstance(ids, list):
                        for session_id in ids:
                            found.setdefault(_str(session_id), cwd)
            self._sidebar_cache[directory] = found
        return self._sidebar_cache[directory]

    @staticmethod
    def _workspace_cwd(session_dir: Path) -> str:
        path = session_dir / "workspace.yaml"
        if path.is_symlink() or not path.is_file():
            return ""
        return read_flat_yaml(path).get("cwd") or ""

    def _fallback_project(self, artifact: Artifact, session_dir: Path, session_id: str) -> str:
        return self._workspace_cwd(session_dir) or self._sidebar(artifact).get(session_id, "")

    # -- events.jsonl -------------------------------------------------------

    def _parse_events(self, artifact: Artifact, session_id: str, opts: Options) -> Iterator[Row]:
        errors: list = []
        project = self._fallback_project(artifact, artifact.disk_path.parent, session_id)
        state = {"project": project, "branch": "", "model": ""}
        names: dict[str, str] = {}
        requested: set[str] = set()
        models: dict = {}
        modes: dict = {}
        permission = {"mode": "", "agent": ""}

        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            event_type = _str(record.get("type"))
            data = _dict(record.get("data"))
            timestamp = to_utc(record.get("timestamp"))

            def row(
                turn: str,
                text: str,
                tool_name: str = "",
                tool_use_id: str = "",
                model: str = "",
                line_number=line_number,
                timestamp=timestamp,
            ) -> Row:
                return self._row(
                    artifact,
                    line_number,
                    turn,
                    text,
                    timestamp_utc=timestamp,
                    session_id=session_id,
                    project_path=state["project"],
                    git_branch=state["branch"],
                    model=model or state["model"],
                    tool_name=tool_name,
                    tool_use_id=tool_use_id,
                )

            if event_type in ("session.start", "session.resume", "session.context_changed"):
                context = _dict(data.get("context")) if "context" in data else data
                if context.get("cwd"):
                    state["project"] = _str(context.get("cwd"))
                if "branch" in context or event_type == "session.context_changed":
                    state["branch"] = _str(context.get("branch"))
                if event_type == "session.start":
                    yield row(
                        "system",
                        "session start: copilot %s producer=%s cwd=%s"
                        % (
                            _str(data.get("copilotVersion")) or "-",
                            _str(data.get("producer")) or "-",
                            _str(context.get("cwd")) or "-",
                        ),
                    )
                elif event_type == "session.resume":
                    yield row("system", "session resumed")
            elif event_type == "system.message":
                label = "developer message" if data.get("role") == "developer" else "system prompt"
                yield row("system", "%s: %s" % (label, _str(data.get("content"))))
            elif event_type == "user.message":
                yield self._user_row(row, record, data)
            elif event_type == "assistant.message":
                yield from self._assistant_rows(row, data, opts, names, requested)
            elif event_type == "assistant.reasoning":
                if opts.include_thinking and data.get("content"):
                    yield row("thinking", _str(data.get("content")))
            elif event_type in ("tool.execution_start", "tool.user_requested"):
                call_id = _str(data.get("toolCallId"))
                name = _str(data.get("toolName"))
                if call_id and name:
                    names.setdefault(call_id, name)
                if event_type == "tool.execution_start" and call_id in requested:
                    continue
                requested.add(call_id)
                text = args_text(data.get("arguments"))
                if event_type == "tool.user_requested":
                    text = "user requested: " + text
                yield row("tool_use", text, name, call_id, _str(data.get("model")))
            elif event_type == "tool.execution_complete":
                call_id = _str(data.get("toolCallId"))
                yield row(
                    "tool_result",
                    result_text(data),
                    names.get(call_id, ""),
                    call_id,
                    _str(data.get("model")),
                )
            elif event_type.startswith("permission.") or event_type in (
                "session.permissions_changed",
                "session.mode_changed",
            ):
                yield from self._permission_rows(
                    row, event_type, data, names, modes, permission, session_id
                )
            elif event_type in ("session.model_change", "session.auto_mode_resolved"):
                yield from self._model_rows(row, event_type, data, models, state, session_id)
            elif event_type == "session.info":
                yield row("system", self._info_text(data))
            elif event_type == "model.message":
                call_model = _str(_dict(data.get("modelCall")).get("model"))
                content = _str(_dict(data.get("message")).get("content"))
                yield row(
                    "system", "utility model %s: %s" % (call_model, content), model=call_model
                )
            elif event_type == "session.compaction_complete":
                yield row("system", "compaction summary: %s" % _str(data.get("summaryContent")))
            elif event_type in ("abort", "session.error", "session.warning"):
                text = _str(data.get("reason")) or _str(data.get("message")) or compact_json(data)
                yield row("system", "%s: %s" % (event_type, text))
            elif event_type in ("hook.start", "hook.end"):
                yield row(
                    "system",
                    self._hook_text(event_type, data),
                    tool_use_id=_str(data.get("parentToolCallId")),
                )
            elif event_type in NOTE_TYPES or event_type.startswith(NOTE_PREFIXES):
                main = next(
                    (_str(data[key]) for key in NOTE_KEYS if data.get(key) not in (None, "")), ""
                )
                yield row("system", "%s: %s" % (event_type, main or compact_json(data)))
            elif event_type == "session.shutdown":
                yield row("system", self._shutdown_text(data))
            # assistant.turn_*, session.usage_*, the other model.* events and
            # unknown types are bookkeeping: no row.

        for error_row in error_rows(self, artifact, errors, session_id, state["project"]):
            error_row.git_branch = state["branch"]
            yield error_row

    @staticmethod
    def _user_row(row: Callable[..., Row], record: dict, data: dict) -> Row:
        """`user` for a typed prompt, else `system` with what sent it."""
        content = _str(data.get("content"))
        source = _str(data.get("source"))
        if data.get("isAutopilotContinuation"):
            label = "auto-continuation"
        elif record.get("agentId"):
            label = "subagent task"
        elif source and source != "user":
            label = source
        else:
            return row("user", content)
        return row("system", "%s: %s" % (label, content))

    @staticmethod
    def _assistant_rows(
        row: Callable[..., Row], data: dict, opts: Options, names: dict, requested: set
    ) -> Iterator[Row]:
        model = _str(data.get("model"))
        content = _str(data.get("content"))
        if content.strip():
            yield row("assistant", content, model=model)
        if opts.include_thinking:
            thought = _str(data.get("reasoningText"))
            if not thought:
                parts = []
                for block in _dict(data.get("reasoningBlocks")).get("blocks") or []:
                    for summary in _dict(block).get("summary") or []:
                        text = _dict(summary).get("text")
                        if isinstance(text, str) and text:
                            parts.append(text)
                thought = "\n\n".join(parts)
            if thought:
                yield row("thinking", thought, model=model)
        for request in data.get("toolRequests") or []:
            request = _dict(request)
            call_id = _str(request.get("toolCallId"))
            name = _str(request.get("name"))
            if call_id:
                names[call_id] = name
                requested.add(call_id)
            yield row("tool_use", args_text(request.get("arguments")), name, call_id, model)

    @staticmethod
    def _permission_rows(
        row: Callable[..., Row],
        event_type: str,
        data: dict,
        names: dict,
        modes: dict,
        permission: dict,
        session_id: str,
    ) -> Iterator[Row]:
        """Yield the permission request, decision and mode rows of one event.

        The tracked mode is `<permissionMode> agent=<agentMode>`; the two change events update their
        half of it.
        """
        if event_type == "permission.requested":
            request = _dict(data.get("permissionRequest"))
            call_id = _str(request.get("toolCallId"))
            text = "permission request: %s" % (_str(request.get("kind")) or "-")
            target = request_target(request)
            if target:
                text += " " + target
            if request.get("intention") not in (None, ""):
                text += " intention=%s" % _str(request.get("intention"))
            yield row("system", text, names.get(call_id, ""), call_id)
            if data.get("permissionMode"):
                permission["mode"] = _str(data.get("permissionMode"))
            if data.get("agentMode"):
                permission["agent"] = _str(data.get("agentMode"))
            if permission["mode"]:
                value = permission["mode"] + (
                    " agent=%s" % permission["agent"] if permission["agent"] else ""
                )
                mode_text = track_change("permission mode", modes, session_id, value)
                if mode_text:
                    yield row("system", mode_text, names.get(call_id, ""), call_id)
        elif event_type == "permission.completed":
            call_id = _str(data.get("toolCallId"))
            yield row("system", decision_text(data), names.get(call_id, ""), call_id)
        elif event_type == "permission.carriedForward":
            call_id = _str(data.get("toolCallId"))
            yield row(
                "system",
                "permission decision: carried-forward source=%s"
                % (_str(data.get("decisionSource")) or "-"),
                names.get(call_id, ""),
                call_id,
            )
        elif event_type == "session.permissions_changed":
            allow_all = (
                "mode" not in data
                and "previousMode" not in data
                and any(key in data for key in ALLOW_ALL_KEYS)
            )
            if allow_all:
                old = _str(data.get("previousAllowAllPermissionMode")) or "-"
                new = _str(data.get("allowAllPermissionMode"))
                yield row("system", "permission mode: allow-all %s -> %s" % (old, new or "-"))
                permission["mode"] = "allow-all %s" % new if new else permission["mode"]
            else:
                old, new = _str(data.get("previousMode")) or "-", _str(data.get("mode")) or "-"
                yield row("system", "permission mode: %s -> %s" % (old, new))
                permission["mode"] = _str(data.get("mode"))
            if permission["mode"]:
                modes[session_id] = permission["mode"] + (
                    " agent=%s" % permission["agent"] if permission["agent"] else ""
                )
        elif event_type == "session.mode_changed":
            old, new = _str(data.get("previousMode")) or "-", _str(data.get("newMode")) or "-"
            yield row("system", "agent mode: %s -> %s" % (old, new))
            permission["agent"] = _str(data.get("newMode"))
            if permission["mode"]:
                modes[session_id] = permission["mode"] + (
                    " agent=%s" % permission["agent"] if permission["agent"] else ""
                )

    @staticmethod
    def _model_rows(
        row: Callable[..., Row],
        event_type: str,
        data: dict,
        models: dict,
        state: dict,
        session_id: str,
    ) -> Iterator[Row]:
        """Yield `model: <id>` at the first model and `model: <old> -> <new>` after.

        A change to the same model is kept, since a change event is a decision even
        when it resolves to the same model.
        """
        if event_type == "session.auto_mode_resolved":
            new = _str(data.get("chosenModel"))
            suffix = " (auto, %s)" % (_str(data.get("routingMethod")) or "-")
        else:
            new = _str(data.get("newModel"))
            suffix = ""
            for key, label in (("cause", "cause"), ("source", "source")):
                if data.get(key) not in (None, ""):
                    suffix += " %s=%s" % (label, _str(data.get(key)))
            if data.get("reasoningEffort") not in (None, ""):
                suffix += " effort=%s" % _str(data.get("reasoningEffort"))
        if not new:
            return
        old = models.get(session_id) or _str(data.get("previousModel"))
        models[session_id] = new
        state["model"] = new
        text = "model: %s -> %s" % (old, new) if old else "model: %s" % new
        yield row("system", text + suffix, model=new)

    @staticmethod
    def _hook_text(event_type: str, data: dict) -> str:
        """Return the text of a `hook.start` or `hook.end` event.

        The text is `hook <hookType> start: <input>`, or `hook <hookType> end:
        success=<b> output=<output>` and ` error=<error>` when present; each payload is
        compact JSON in full, `{}` when absent.
        """
        hook = _str(data.get("hookType")) or "-"
        if event_type == "hook.start":
            payload = data.get("input")
            return "hook %s start: %s" % (hook, "{}" if payload is None else compact_json(payload))
        success = data.get("success")
        output = data.get("output")
        text = "hook %s end: success=%s output=%s" % (
            hook,
            setting_value(success) if isinstance(success, bool) else "-",
            "{}" if output is None else compact_json(output),
        )
        if data.get("error") is not None:
            text += " error=%s" % compact_json(data.get("error"))
        return text

    @staticmethod
    def _info_text(data: dict) -> str:
        """Return `session.info <infoType>: <message>` and the `url` and `tip` present."""
        text = "session.info %s: %s" % (_str(data.get("infoType")), _str(data.get("message")))
        for key in ("url", "tip"):
            if data.get(key) not in (None, ""):
                text += " %s=%s" % (key, _str(data.get(key)))
        return text

    @staticmethod
    def _shutdown_text(data: dict) -> str:
        files = _dict(data.get("codeChanges")).get("filesModified")
        if isinstance(files, list):
            files_text = ", ".join(_str(path) for path in files) or "none"
        else:
            files_text = _str(files) or "none"
        parts = ["session end:", _str(data.get("shutdownType")) or "-"]
        if data.get("errorReason") not in (None, ""):
            parts.append(_str(data.get("errorReason")))
        parts.append("files modified: %s" % files_text)
        return " ".join(parts)

    # -- workspace.yaml -----------------------------------------------------

    def _parse_workspace(self, artifact: Artifact, session_id: str) -> Iterator[Row]:
        metadata = read_flat_yaml(artifact.disk_path)
        session_id = metadata.get("id") or session_id
        columns = {
            "timestamp_utc": to_utc(metadata.get("created_at")),
            "session_id": session_id,
            "project_path": metadata.get("cwd") or self._sidebar(artifact).get(session_id, ""),
        }
        if metadata.get("name"):
            yield self._row(
                artifact, 1, "system", "session name: %s" % metadata.get("name"), **columns
            )
        yield self._row(
            artifact,
            1,
            "system",
            "session metadata: user_named=%s forks=%s compactions=%s client=%s"
            % (
                metadata.get("user_named") or "-",
                metadata.get("fork_count") or "-",
                metadata.get("summary_count") or "-",
                metadata.get("client_name") or "",
            ),
            **columns,
        )
        # A session the app pre-created and never prompted (Copilot
        # Desktop's pool and probe sessions) has no events.jsonl; its cwd
        # is the agent's, not a project.
        if metadata.get("cwd") and not os.path.lexists(artifact.disk_path.parent / "events.jsonl"):
            yield self._row(
                artifact,
                1,
                "system",
                "%s %s (working directory of a session the app pre-created and never"
                " prompted: no events.jsonl)" % (SCRATCH_PREFIX, metadata.get("cwd")),
                **columns,
            )
        if metadata.get("mc_session_id"):
            yield self._row(
                artifact,
                1,
                "system",
                "session synced: remote session %s task %s last event %s remote_steerable=%s"
                " (from workspace.yaml, file mtime)"
                % tuple(
                    metadata.get(key) or "-"
                    for key in (
                        "mc_session_id",
                        "mc_task_id",
                        "mc_last_event_id",
                        "remote_steerable",
                    )
                ),
                **{**columns, "timestamp_utc": file_mtime(artifact)},
            )

    # -- rewind-file-snapshots/index.json -----------------------------------

    def _parse_snapshots(self, artifact: Artifact, session_id: str) -> Iterator[Row]:
        document = _read_json(artifact.disk_path)
        if not isinstance(document, dict):
            yield self._problem(artifact, "parser: rewind snapshot index is not a JSON object")
            return
        session_dir = artifact.disk_path.parent.parent
        project = self._fallback_project(artifact, session_dir, session_id)
        for i, snapshot in enumerate(document.get("snapshots") or [], 1):
            snapshot = _dict(snapshot)
            columns = {
                "timestamp_utc": to_utc(snapshot.get("timestamp")),
                "session_id": session_id,
                "project_path": project,
            }
            if snapshot.get("userMessage") not in (None, ""):
                yield self._row(artifact, i, "user", _str(snapshot.get("userMessage")), **columns)
            paths = [
                _str(_dict(entry).get("path"))
                for entry in _dict(snapshot.get("files")).values()
                if _dict(entry).get("path")
            ]
            text = "rewind snapshot: %s files +%s -%s" % (
                _str(snapshot.get("fileCount")) or "0",
                _str(snapshot.get("linesAdded")) or "0",
                _str(snapshot.get("linesRemoved")) or "0",
            )
            if paths:
                text += ": " + ", ".join(paths)
            yield self._row(artifact, i, "system", text, **columns)

    # -- session-store.db ---------------------------------------------------

    def _parse_store(self, artifact: Artifact) -> Iterator[Row]:
        if not is_sqlite(artifact.disk_path):
            yield self._problem(artifact, "parser: not a SQLite database")
            return
        try:
            with open_copy(artifact.disk_path) as connection:
                rows = list(self._store_rows(artifact, connection))
        except sqlite3.DatabaseError as error:
            yield self._problem(artifact, "parser: SQLite error: %s" % error)
            return
        yield from rows

    @staticmethod
    def _select(connection: sqlite3.Connection, table: str, order: str) -> list[dict]:
        sql = "select rowid as _rowid, * from %s order by %s" % (table, order)
        return [dict(row) for row in connection.execute(sql)]

    def _store_rows(self, artifact: Artifact, connection: sqlite3.Connection) -> Iterator[Row]:
        tables = table_names(connection)
        sidebar = self._sidebar(artifact)
        sessions: dict[str, dict] = {}
        if "sessions" in tables:
            for session in self._select(connection, "sessions", "created_at, rowid"):
                sessions[_str(session.get("id"))] = session

        def stamp(session_id: str, line, turn: str, text: str, timestamp, **columns) -> Row:
            session = sessions.get(session_id, {})
            return self._row(
                artifact,
                int(line or 0),
                turn,
                text,
                timestamp_utc=to_utc(timestamp),
                session_id=session_id,
                project_path=_str(session.get("cwd")) or sidebar.get(session_id, ""),
                git_branch=_str(session.get("branch")),
                **columns,
            )

        for session_id, session in sessions.items():
            if session.get("summary") not in (None, ""):
                yield stamp(
                    session_id,
                    session["_rowid"],
                    "system",
                    "session name: %s" % session["summary"],
                    session.get("created_at"),
                )
        if "turns" in tables:
            for turn_record in self._select(connection, "turns", "session_id, turn_index, rowid"):
                session_id = _str(turn_record.get("session_id"))
                for column, turn in (("user_message", "user"), ("assistant_response", "assistant")):
                    if turn_record.get(column) not in (None, ""):
                        yield stamp(
                            session_id,
                            turn_record["_rowid"],
                            turn,
                            _str(turn_record[column]),
                            turn_record.get("timestamp"),
                        )
        if "session_files" in tables:
            for file_record in self._select(connection, "session_files", "first_seen_at, rowid"):
                text = "file changed: %s tool=%s" % (
                    _str(file_record.get("file_path")),
                    _str(file_record.get("tool_name")) or "-",
                )
                yield stamp(
                    _str(file_record.get("session_id")),
                    file_record["_rowid"],
                    "system",
                    text,
                    file_record.get("first_seen_at"),
                )
        if "session_refs" in tables:
            for ref in self._select(connection, "session_refs", "created_at, rowid"):
                text = "reference: %s %s" % (_str(ref.get("ref_type")), _str(ref.get("ref_value")))
                yield stamp(
                    _str(ref.get("session_id")),
                    ref["_rowid"],
                    "system",
                    text,
                    ref.get("created_at"),
                )
        if "checkpoints" in tables:
            for checkpoint in self._select(connection, "checkpoints", "created_at, rowid"):
                lines = [
                    "checkpoint %s: %s"
                    % (_str(checkpoint.get("checkpoint_number")), _str(checkpoint.get("title")))
                ]
                for column in CHECKPOINT_COLUMNS:
                    if checkpoint.get(column) not in (None, ""):
                        lines.append("%s: %s" % (column, _str(checkpoint[column])))
                yield stamp(
                    _str(checkpoint.get("session_id")),
                    checkpoint["_rowid"],
                    "system",
                    "\n".join(lines),
                    checkpoint.get("created_at"),
                )
        if "forge_trajectory_events" in tables:
            yield from self._forge_rows(connection, stamp)

    def _forge_rows(
        self, connection: sqlite3.Connection, stamp: Callable[..., Row]
    ) -> Iterator[Row]:
        """Yield the `forge_trajectory_events` rows, read from the bundle's shape alone.

        A row with a `command` is a `tool_use`, one with an `output` or an `exit_code` a
        `tool_result`, both joined on `tool_call_id`; every column may be NULL.
        """
        for event in self._select(connection, "forge_trajectory_events", "created_at, rowid"):
            session_id = _str(event.get("session_id"))
            call_id = _str(event.get("tool_call_id"))
            if event.get("command") not in (None, ""):
                yield stamp(
                    session_id,
                    event["_rowid"],
                    "tool_use",
                    _str(event.get("command")),
                    event.get("created_at"),
                    tool_use_id=call_id,
                )
            code = event.get("exit_code")
            if event.get("output") is not None or code is not None:
                text = _str(event.get("output"))
                if code not in (None, 0, "0"):
                    text = "[exit %s] %s" % (code, text)
                yield stamp(
                    session_id,
                    event["_rowid"],
                    "tool_result",
                    text,
                    event.get("created_at"),
                    tool_use_id=call_id,
                )

    # -- command-history-state.json -----------------------------------------

    def _parse_history(self, artifact: Artifact) -> Iterator[Row]:
        document = _read_json(artifact.disk_path)
        if not isinstance(document, dict):
            yield self._problem(artifact, "parser: command history is not a JSON object")
            return
        for i, entry in enumerate(document.get("commandHistory") or [], 1):
            if isinstance(entry, str) and entry.strip():
                yield self._row(artifact, i, "user", entry)

    # -- standing permissions -----------------------------------------------

    def _setting(
        self, artifact: Artifact, timestamp: str, key: str, value, project: str = ""
    ) -> Row:
        text = "permission setting: %s=%s (from %s, file mtime)" % (
            key,
            setting_value(value),
            artifact.rel,
        )
        return self._row(artifact, 1, "system", text, timestamp_utc=timestamp, project_path=project)

    def _parse_permissions(self, artifact: Artifact) -> Iterator[Row]:
        timestamp = file_mtime(artifact)
        document = _read_json(artifact.disk_path)
        if not isinstance(document, dict):
            yield self._problem(
                artifact, "parser: permissions-config.json is not a JSON object", timestamp
            )
            return
        for path, entry in _dict(document.get("locations")).items():
            entry = _dict(entry)
            for key in ("tool_approvals", "allowed_directories"):
                items = entry.get(key)
                if not isinstance(items, list):
                    continue
                for item in items:
                    yield self._setting(
                        artifact, timestamp, "locations.%s.%s" % (path, key), item, path
                    )

    def _parse_settings(self, artifact: Artifact) -> Iterator[Row]:
        timestamp = file_mtime(artifact)
        try:
            raw = artifact.disk_path.read_bytes().decode("utf-8", errors="replace")
            document = json.loads(strip_jsonc(raw))
        except (OSError, ValueError):
            document = None
        if not isinstance(document, dict):
            yield self._problem(artifact, "parser: settings.json is not a JSON object", timestamp)
            return
        for key in ("allowedUrls", "deniedUrls"):
            items = document.get(key)
            if isinstance(items, list):
                for item in items:
                    yield self._setting(artifact, timestamp, key, item)

    # -- logs/process-*.log -------------------------------------------------

    def _parse_log(self, artifact: Artifact) -> Iterator[Row]:
        try:
            lines = split_lines(artifact.disk_path.read_bytes().decode("utf-8", errors="replace"))
        except OSError:
            return
        sessions = {
            match.group(1)
            for line in lines
            for match in [WORKSPACE_LINE_PATTERN.search(line)]
            if match
        }
        session_id = next(iter(sessions)) if len(sessions) == 1 else ""
        for line_number, line in enumerate(lines, 1):
            match = SIGNED_IN_PATTERN.search(line)
            if not match:
                continue
            time_match = LOG_TIME_PATTERN.match(line)
            yield self._row(
                artifact,
                line_number,
                "system",
                "signed in: %s as %s" % (match.group(1), match.group(2)),
                timestamp_utc=to_utc(time_match.group(1)) if time_match else "",
                session_id=session_id,
            )
