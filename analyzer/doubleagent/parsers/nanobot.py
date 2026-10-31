"""nanobot sessions, memory history, WebUI mirror, CLI history, settings and run records.

Reads, under a state directory `.nanobot` or `.nanobot-<instance>`:

* `sessions/<workspace-id>/sessions.sqlite3`, the store of record from 0.3.6: `sessions`
  (`key`, `created_at`, `updated_at`, `metadata`) and `messages` (`session_key`, `position`,
  `record`, one JSONL message line), mapped as the JSONL files below.
* `sessions/<workspace-id>/<base64 key>.jsonl`, `jsonl-backup/` and `.migration-conflicts/`
  copies and legacy `sessions/<key>.jsonl`; the copies a database imported are secondary.
  Line 1 is `_type` `metadata` (`key`, `created_at`, `updated_at`, `metadata`
  `_nanobot_model_preset`, `last_channel`, `workspace_scope`); the rest are OpenAI chat
  messages (`role`, `content`, `timestamp`, `tool_calls[]`, `reasoning_content`,
  `tool_call_id`, `name`, `_hidden_history`, `_command`). `.workspace` is the project path.
* `memory/history.jsonl` (`cursor`, `timestamp`, `content`, `session_key`), primary.
* Secondary: `webui/<key>.jsonl` and its `.segments/` (`event`, `chat_id`, `created_at_ms`,
  see `_webui_events`), `history/cli_history`, `cron/runs/*.json` and
  `triggers/runs/*.json` under a workspace, and `.webui_session_index.json`.
* `config.json` (only `CONFIG_TOOL_KEYS`, `CONFIG_EXEC_KEYS` and `channels.<name>.allowFrom`;
  it holds API keys) and `pairing.json` (`approved`, `pending`).

Every timestamp nanobot writes in the session files is naive host local time, emitted as if
UTC; the WebUI and run records carry true UTC, hence `WEBUI_TOLERANCE_SECONDS`. The model is
the session's preset (`model: <preset> (preset)`) or a `/model` switch's reply; a messaging
session key gives relay rows.

Validated against source, HKUDS/nanobot at acdae3d and, for the SQLite store, the `--home`
history path, relay, switch, permission and run rows, at 909b277 (version 0.3.5 in
`pyproject.toml`, the store named as 0.3.6), with a synthetic fixture; no real install. See
`research/nanobot.md`.
"""

from __future__ import annotations

import base64
import json
import re
import sqlite3
from collections.abc import Generator, Iterable, Iterator
from pathlib import Path

from ..inputs import Artifact, error_reason
from ..model import Row, compact
from ..sqlite_util import is_sqlite, open_copy, table_names
from ..timeutil import to_utc
from .aider import read_input_history
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_mtime,
    iter_jsonl,
    setting_value,
    text_of,
    track_change,
)
from .openclaw import tool_args_text

STATE = r"\.nanobot(?:-[^/]+)?"
SESSION_PATTERN = re.compile(
    r"^"
    + STATE
    + r"/sessions/(?:(?P<ws>[0-9a-f]{32})/(?P<sub>\.migration-conflicts/|jsonl-backup/)?)?"
    r"(?P<stem>[^/]+)\.jsonl$"
)
DATABASE_PATTERN = re.compile(r"^" + STATE + r"/sessions/[0-9a-f]{32}/sessions\.sqlite3$")
DATABASE_NAME = "sessions.sqlite3"
HISTORY_PATTERN = re.compile(r"^" + STATE + r"/(?:.+/)?memory/history\.jsonl$")
WEBUI_PATTERN = re.compile(
    r"^" + STATE + r"/webui/(?:(?P<seg>[^/]+)\.segments/\d{6}|(?P<stem>[^/]+))\.jsonl$"
)
CLI_HISTORY_PATTERN = re.compile(r"^" + STATE + r"/history/cli_history$")
CONFIG_PATTERN = re.compile(r"^" + STATE + r"/config\.json$")
PAIRING_PATTERN = re.compile(r"^" + STATE + r"/pairing\.json$")
SIDEBAR_PATTERN = re.compile(r"^" + STATE + r"/sessions/[0-9a-f]{32}/\.webui_session_index\.json$")
# A cron or trigger run record: under a workspace inside the state directory
# (`.nanobot/workspace/cron/runs/`), under the state directory itself (the
# legacy `cron/jobs.json` location), or at the root of a project workspace
# the collector collected (the bare form, accepted only for a project base).
RUN_PATTERN = re.compile(
    r"^(?:" + STATE + r"/(?P<ws>.+/)?)?(?P<kind>cron|triggers)/runs/[^/]+\.json$"
)
ORIGINAL_RUN_PATTERN = re.compile(r"^(.*)[\\/](?:cron|triggers)[\\/]runs[\\/][^\\/]+$")
WEBUI_PREFIX = "websocket:"
# The host's UTC offset (at most 14 hours) plus an hour of turn: the WebUI
# transcript and the run records carry true UTC, the session files local time.
WEBUI_TOLERANCE_SECONDS = 15 * 3600.0
# Session key prefixes of the channels that carry a chat through a messaging
# platform (each channel's `name`, nanobot/channels/*/runtime.py at 909b277);
# `websocket` (the WebUI), `cli`, `api`, `heartbeat`, `dream:` and
# `unified:default` are local.
MESSAGING_CHANNELS = frozenset(
    {
        "dingtalk",
        "discord",
        "email",
        "feishu",
        "linear",
        "matrix",
        "mattermost",
        "mochat",
        "msteams",
        "napcat",
        "qq",
        "signal",
        "slack",
        "telegram",
        "wecom",
        "weixin",
        "whatsapp",
        "whatsapp_agent",
    }
)
# `config.json` settings that grant or restrict what the agent may do: each
# tuple is the spellings nanobot accepts for one key (camelCase, as it saves
# them, snake_case and any legacy alias); the row names the key as the file
# spells it, and a list gives one row per entry.
CONFIG_TOOL_KEYS = (
    ("restrictToWorkspace", "restrict_to_workspace"),
    ("ssrfWhitelist", "ssrf_whitelist"),
    (
        "webuiAllowLocalServiceAccess",
        "webui_allow_local_service_access",
        "allowLocalPreviewAccess",
        "allow_local_preview_access",
    ),
    ("webuiAllowRemotePackageInstall", "webui_allow_remote_package_install"),
)
CONFIG_EXEC_KEYS = (
    ("enable",),
    ("sandbox",),
    ("allowPatterns", "allow_patterns"),
    ("denyPatterns", "deny_patterns"),
    ("sandboxRoBinds", "sandbox_ro_binds"),
    ("sandboxRwBinds", "sandbox_rw_binds"),
    ("allowedEnvKeys", "allowed_env_keys"),
)
# The `/model <preset>` command and its reply (nanobot/command/builtin.py).
MODEL_COMMAND_PATTERN = re.compile(r"^/model\s+(\S.*?)\s*$", re.S)
SWITCH_PATTERN = re.compile(r"^Switched model preset to `([^`]*)`")
RESOLVED_PATTERN = re.compile(r"^- Model: `([^`]*)`", re.M)
# Stream chunks (folded into their end records) and the turn boundary.
WEBUI_SILENT = frozenset({"delta", "reasoning_delta", "turn_end"})
ORIGINAL_MEMORY_PATTERN = re.compile(r"^(.*)[\\/]memory[\\/]history\.jsonl$")


def decode_stem(stem: str) -> str:
    """Return the session key of a file stem.

    The stem is URL-safe base64 without padding; the legacy lossy `<channel>_<chat>` form is
    returned unchanged.
    """
    try:
        key = base64.urlsafe_b64decode(stem + "=" * (-len(stem) % 4)).decode("utf-8")
    except (ValueError, UnicodeDecodeError):
        return stem
    if base64.urlsafe_b64encode(key.encode("utf-8")).decode("ascii").rstrip("=") != stem:
        return stem
    return key


def _read_marker(path: Path) -> str:
    """Return the stripped text of a `.workspace` file, `""` when it cannot be read."""
    try:
        if path.is_symlink() or not path.is_file():
            return ""
        return path.read_text(encoding="utf-8", errors="replace").strip()
    except OSError:
        return ""


def _json_object(raw: object, rowid: int, label: str, errors: list) -> dict | None:
    """Return a stored JSON text decoded to an object, or None when it does not decode to one.

    On failure (rowid, `<label> (<reason>)`, the value as text) is appended to `errors`.
    """
    text = raw.decode("utf-8", errors="replace") if isinstance(raw, bytes) else str(raw)
    try:
        value = json.loads(text)
    except ValueError as error:
        errors.append((rowid, "%s (%s)" % (label, error), text))
        return None
    if not isinstance(value, dict):
        errors.append((rowid, "%s (not a JSON object)" % label, text))
        return None
    return value


def relay_texts(session_id: str, last_channel: object) -> list[str]:
    """Return the `session relayed:` row texts of a session.

    One for a session key `<channel>:<chat_id>` whose channel is a messaging platform, and one for a
    `metadata.last_channel` route of a messaging platform that differs from the key.
    """
    texts = []
    channel, _, chat = session_id.partition(":")
    if channel in MESSAGING_CHANNELS and chat:
        texts.append("session relayed: %s chat %s (from session key)" % (channel, chat))
    if isinstance(last_channel, str) and last_channel != session_id:
        channel, _, chat = last_channel.partition(":")
        if channel in MESSAGING_CHANNELS and chat:
            texts.append(
                "session relayed: %s chat %s (from metadata.last_channel)" % (channel, chat)
            )
    return texts


def _first_key(mapping: dict, names: tuple[str, ...]) -> tuple[str, object] | None:
    """Return (name, value) of the first of `names` that `mapping` holds, else None."""
    for name in names:
        if name in mapping:
            return name, mapping[name]
    return None


def _has_switch(records: list[tuple[int, dict]]) -> bool:
    """Return True when a session records a `/model` switch reply."""
    return any(
        record.get("role") == "assistant"
        and record.get("_command")
        and SWITCH_PATTERN.match(text_of(record.get("content")))
        for _, record in records
    )


class NanobotParser(Parser):
    """Parse nanobot's session stores, their copies, histories, settings and run records."""

    agent = "nanobot"
    name = "nanobot"
    has_secondary = True
    records_relay = True

    def wants(self, artifact: Artifact) -> bool:
        """Return True for the nanobot files this parser reads."""
        return bool(
            SESSION_PATTERN.match(artifact.rel)
            or HISTORY_PATTERN.match(artifact.rel)
            or WEBUI_PATTERN.match(artifact.rel)
            or DATABASE_PATTERN.match(artifact.rel)
            or CLI_HISTORY_PATTERN.match(artifact.rel)
            or CONFIG_PATTERN.match(artifact.rel)
            or PAIRING_PATTERN.match(artifact.rel)
            or SIDEBAR_PATTERN.match(artifact.rel)
            or self._is_run(artifact)
        )

    @staticmethod
    def _is_run(artifact: Artifact) -> bool:
        """Return True for a cron or trigger run record.

        The bare `cron/runs/` form counts only at the root of a project base, where the project
        catalog put it.
        """
        match = RUN_PATTERN.match(artifact.rel)
        if match is None:
            return False
        return artifact.rel.startswith(".nanobot") or artifact.home.project

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for the files that repeat the session store's events."""
        return (
            bool(
                WEBUI_PATTERN.match(artifact.rel)
                or CLI_HISTORY_PATTERN.match(artifact.rel)
                or SIDEBAR_PATTERN.match(artifact.rel)
            )
            or self._is_run(artifact)
            or self._frozen(artifact)
        )

    @staticmethod
    def _namespace(artifact: Artifact) -> Path | None:
        """Return the `sessions/<workspace-id>` directory of a namespaced session file, or None.

        The file's own directory, or its parent for a `.migration-conflicts/` or `jsonl-backup/`
        copy; None for a legacy global file or any other path.
        """
        match = SESSION_PATTERN.match(artifact.rel)
        if not match or not match.group("ws"):
            return None
        return artifact.disk_path.parent.parent if match.group("sub") else artifact.disk_path.parent

    @classmethod
    def _frozen(cls, artifact: Artifact) -> bool:
        """Return True for a session JSONL that the namespace's `sessions.sqlite3` imported.

        A `<stem>.jsonl` of the namespace or a `jsonl-backup/` copy, when the database is beside it;
        a `.migration-conflicts/` copy and a legacy global file were never imported.
        """
        match = SESSION_PATTERN.match(artifact.rel)
        namespace = cls._namespace(artifact)
        if match is None or namespace is None or match.group("sub") == ".migration-conflicts/":
            return False
        database = namespace / DATABASE_NAME
        return not database.is_symlink() and database.is_file()

    def secondary_tolerance(self, artifact: Artifact) -> float:
        """Return `WEBUI_TOLERANCE_SECONDS` for the true-UTC sources, else the default window."""
        if WEBUI_PATTERN.match(artifact.rel) or self._is_run(artifact):
            return WEBUI_TOLERANCE_SECONDS
        return super().secondary_tolerance(artifact)

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one nanobot file."""
        if SESSION_PATTERN.match(artifact.rel):
            yield from self._parse_session(artifact, opts)
        elif DATABASE_PATTERN.match(artifact.rel):
            yield from self._parse_database(artifact, opts)
        elif WEBUI_PATTERN.match(artifact.rel):
            yield from self._parse_webui(artifact, opts)
        elif CLI_HISTORY_PATTERN.match(artifact.rel):
            yield from self._parse_cli_history(artifact)
        elif CONFIG_PATTERN.match(artifact.rel):
            yield from self._parse_config(artifact)
        elif PAIRING_PATTERN.match(artifact.rel):
            yield from self._parse_pairing(artifact)
        elif SIDEBAR_PATTERN.match(artifact.rel):
            yield from self._parse_sidebar(artifact)
        elif RUN_PATTERN.match(artifact.rel):
            yield from self._parse_run(artifact)
        else:
            yield from self._parse_history(artifact, opts)

    def _parse_session(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        match = SESSION_PATTERN.match(artifact.rel)
        assert match is not None  # parse() routes only matching paths here
        namespace = self._namespace(artifact)
        project = _read_marker(namespace / ".workspace") if namespace else ""
        errors: list = []
        session_id = yield from self._session_rows(
            artifact,
            opts,
            decode_stem(match.group("stem")),
            project,
            iter_jsonl(artifact.disk_path, errors),
        )
        yield from error_rows(self, artifact, errors, session_id, project)

    def _session_rows(
        self,
        artifact: Artifact,
        opts: Options,
        session_id: str,
        project: str,
        records: Iterable[tuple[int, dict]],
    ) -> Generator[Row, None, str]:
        """Yield the rows of one session from its (source_line, record) pairs.

        The metadata record comes first, then the messages, as a session JSONL holds them and as
        `_parse_database` rebuilds them from `sessions.sqlite3`. Returns the session id, the
        metadata `key` when there is one.
        """
        model = ""
        last_timestamp = ""
        models: dict = {}
        records = list(records)
        # With a `/model` switch in the session, the metadata's preset is the
        # last switch's, not the session's first: the switch rows name the
        # presets instead, and the rows before the first switch name none.
        switched = _has_switch(records)
        preset = ""  # the preset the last switch set, "" before any
        requested = ""  # the preset name of the last `/model <name>`

        def make_row(
            source_line: int, turn_type: str, text: str, tool_name: str = "", tool_use_id: str = ""
        ) -> Row:
            row = self.base_row(artifact)
            row.source_line = source_line
            row.session_id = session_id
            row.project_path = project
            row.timestamp_utc = last_timestamp
            row.model = model
            row.turn_type = turn_type
            row.tool_name = tool_name
            row.tool_use_id = tool_use_id
            row.text = compact(text)
            return row

        for line_number, record in records:
            record_type = record.get("_type")
            if record_type == "metadata":
                if record.get("key"):
                    session_id = str(record["key"])
                meta = record.get("metadata")
                if not isinstance(meta, dict):
                    meta = {}
                model = "" if switched else str(meta.get("_nanobot_model_preset") or "")
                last_timestamp = to_utc(record.get("created_at")) or to_utc(
                    record.get("updated_at")
                )
                yield make_row(
                    line_number,
                    "system",
                    " ".join(
                        part
                        for part in (
                            "session start: key=%s" % session_id,
                            "last_channel=%s" % meta["last_channel"]
                            if meta.get("last_channel")
                            else "",
                            "updated_at=%s" % record["updated_at"]
                            if record.get("updated_at")
                            else "",
                            "workspace=%s" % project if project else "",
                        )
                        if part
                    ),
                )
                for text in relay_texts(session_id, meta.get("last_channel")):
                    yield make_row(line_number, "system", text)
                text = track_change("model", models, session_id, model)
                if text:
                    yield make_row(line_number, "system", text + " (preset)")
                scope = meta.get("workspace_scope")
                if isinstance(scope, dict) and (
                    scope.get("access_mode") or scope.get("project_path")
                ):
                    yield make_row(
                        line_number,
                        "system",
                        "permission mode: access_mode=%s project=%s"
                        " (from session metadata workspace_scope)"
                        % (scope.get("access_mode") or "", scope.get("project_path") or ""),
                    )
                continue
            if record_type is not None and "role" not in record:
                continue  # provider_state and future record types
            last_timestamp = to_utc(record.get("timestamp")) or last_timestamp
            role = record.get("role")
            text = text_of(record.get("content"))
            if role == "user":
                if record.get("_hidden_history"):
                    yield make_row(line_number, "system", "summarised history: " + text)
                else:
                    if record.get("_command"):
                        match = MODEL_COMMAND_PATTERN.match(text)
                        requested = match.group(1) if match else requested
                    yield make_row(line_number, "user", text)
            elif role == "assistant":
                reasoning = record.get("reasoning_content")
                if opts.include_thinking and isinstance(reasoning, str) and reasoning:
                    yield make_row(line_number, "thinking", reasoning)
                if text:
                    yield make_row(line_number, "assistant", text)
                match = SWITCH_PATTERN.match(text) if record.get("_command") else None
                if match:
                    resolved = RESOLVED_PATTERN.search(text)
                    switch = "model: %s%s (preset)%s%s" % (
                        preset + " -> " if preset else "",
                        match.group(1),
                        " requested=%s" % requested if requested else "",
                        " resolved=%s" % resolved.group(1) if resolved else "",
                    )
                    preset = model = match.group(1)
                    requested = ""
                    yield make_row(line_number, "system", switch)
                for tool_call in record.get("tool_calls") or []:
                    if not isinstance(tool_call, dict):
                        continue
                    function = tool_call.get("function")
                    if not isinstance(function, dict):
                        function = {}
                    yield make_row(
                        line_number,
                        "tool_use",
                        tool_args_text(function.get("arguments")),
                        str(function.get("name") or ""),
                        str(tool_call.get("id") or ""),
                    )
            elif role == "tool":
                yield make_row(
                    line_number,
                    "tool_result",
                    text,
                    str(record.get("name") or ""),
                    str(record.get("tool_call_id") or ""),
                )
            else:
                yield make_row(line_number, "system", "%s: %s" % (role or "message", text))
        return session_id

    def _parse_database(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of every session of a `sessions.sqlite3`, orphaned messages included."""
        project = _read_marker(artifact.disk_path.parent / ".workspace")
        if not is_sqlite(artifact.disk_path):
            yield self._file_row(artifact, project, "parser: not a SQLite database")
            return
        messages: dict[str, list[tuple[int, object, object]]] = {}
        try:
            with open_copy(artifact.disk_path) as connection:
                missing = {"sessions", "messages"} - table_names(connection)
                if missing:
                    text = "parser: no %s table in the session database" % " or ".join(
                        sorted(missing)
                    )
                    yield self._file_row(artifact, project, text)
                    return
                sessions = [
                    tuple(table_row)
                    for table_row in connection.execute(
                        "SELECT rowid, key, created_at, updated_at, metadata FROM sessions"
                        " ORDER BY created_at, key"
                    )
                ]
                for rowid, key, position, record in connection.execute(
                    "SELECT rowid, session_key, position, record FROM messages"
                    " ORDER BY session_key, position"
                ):
                    messages.setdefault(str(key), []).append((rowid, position, record))
        except sqlite3.DatabaseError as error:
            yield self._file_row(artifact, project, "parser: SQLite error: %s" % error)
            return
        known = {str(session[1]) for session in sessions}
        # Messages whose session row is gone (nanobot enforces the foreign
        # key only on its own connections) still yield their rows.
        heads: list[tuple[str, tuple | None]] = [(str(session[1]), session) for session in sessions]
        heads += [(k, None) for k in sorted(messages) if k not in known]
        for key, head in heads:
            errors: list = []
            records = self._database_records(head, messages.get(key, []), errors)
            session_id = yield from self._session_rows(artifact, opts, key, project, records)
            yield from error_rows(self, artifact, errors, session_id, project)

    @staticmethod
    def _database_records(
        head: tuple | None, messages: list[tuple[int, object, object]], errors: list
    ) -> Iterator[tuple[int, dict]]:
        """Yield (rowid, record) pairs of one session of `sessions.sqlite3`.

        Its `sessions` row as the JSONL metadata record, when the row exists, then each
        `messages.record` in `position` order. A value that is not a JSON object is appended to
        `errors` and skipped.
        """
        if head is not None:
            rowid, key, created_at, updated_at, raw = head
            meta = _json_object(raw, rowid, "unparseable session metadata", errors)
            yield (
                rowid,
                {
                    "_type": "metadata",
                    "key": key,
                    "created_at": created_at,
                    "updated_at": updated_at,
                    "metadata": meta or {},
                },
            )
        for rowid, position, raw in messages:
            record = _json_object(raw, rowid, "unparseable record %s" % position, errors)
            if record is not None:
                yield rowid, record

    def _file_row(self, artifact: Artifact, project: str, text: str) -> Row:
        """Return a whole-file `parser:` row."""
        row = self.base_row(artifact)
        row.project_path = project
        row.turn_type = "system"
        row.text = text
        return row

    def _parse_history(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield one `memory history` row per line of `memory/history.jsonl`."""
        match = ORIGINAL_MEMORY_PATTERN.match(artifact.original or "")
        project = match.group(1) if match else ""
        errors: list = []
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            row = self.base_row(artifact)
            row.source_line = line_number
            row.session_id = str(record.get("session_key") or "")
            row.project_path = project
            row.timestamp_utc = to_utc(record.get("timestamp"))
            row.turn_type = "system"
            row.text = compact(
                "memory history #%s: %s" % (record.get("cursor", ""), record.get("content") or ""),
            )
            yield row
        yield from error_rows(self, artifact, errors, "", project)

    @staticmethod
    def _webui_workspace(artifact: Artifact, session_id: str) -> str:
        """Return the `.workspace` of the session file for `session_id`, if exactly one has one."""
        match = WEBUI_PATTERN.match(artifact.rel)
        assert match is not None  # parse() routes only matching paths here
        state = artifact.disk_path.parents[2 if match.group("seg") else 1]
        stem = base64.urlsafe_b64encode(session_id.encode("utf-8")).decode("ascii").rstrip("=")
        found: set[str] = set()
        try:
            for session_file in (state / "sessions").glob("*/%s.jsonl" % stem):
                if (
                    not (session_file.is_symlink() or session_file.parent.is_symlink())
                    and session_file.is_file()
                ):
                    found.add(_read_marker(session_file.parent / ".workspace"))
        except OSError:
            return ""
        found.discard("")
        return found.pop() if len(found) == 1 else ""

    def _parse_webui(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of a WebUI transcript file, its session id `websocket:<chat_id>`."""
        match = WEBUI_PATTERN.match(artifact.rel)
        assert match is not None  # parse() routes only matching paths here
        stem = match.group("seg") or match.group("stem")
        fallback = stem
        if stem.startswith("websocket_"):
            fallback = WEBUI_PREFIX + stem[len("websocket_") :]
        projects: dict[str, str] = {}
        errors: list = []

        def make_row(
            source_line: int,
            row_session_id: str,
            timestamp: object,
            event: tuple[str, str, str, str],
        ) -> Row:
            turn_type, text, tool_name, tool_use_id = event
            if row_session_id not in projects:
                projects[row_session_id] = self._webui_workspace(artifact, row_session_id)
            row = self.base_row(artifact)
            row.source_line = source_line
            row.session_id = row_session_id
            row.project_path = projects[row_session_id]
            row.timestamp_utc = to_utc(timestamp)
            row.turn_type = turn_type
            row.tool_name = tool_name
            row.tool_use_id = tool_use_id
            row.text = compact(text)
            return row

        session_id = fallback
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            chat = record.get("chat_id")
            session_id = WEBUI_PREFIX + chat if isinstance(chat, str) and chat else fallback
            for event in self._webui_events(record, opts):
                yield make_row(line_number, session_id, record.get("created_at_ms"), event)
        if errors:
            if session_id not in projects:
                projects[session_id] = self._webui_workspace(artifact, session_id)
            yield from error_rows(self, artifact, errors, session_id, projects[session_id])

    @staticmethod
    def _webui_events(record: dict, opts: Options) -> Iterator[tuple[str, str, str, str]]:
        """Yield (turn_type, text, tool_name, tool_use_id) per row of one WebUI record.

        `user`, `message` (by `kind`: tool events joined on `call_id`, a breadcrumb only without
        them, reasoning, else assistant text), `stream_end` unless `merge_next`, `reasoning_end`,
        finished `file_edit` edits, `fork_marker` and `context_compaction`; `WEBUI_SILENT` events
        give nothing and any other event a `webui event:` row.
        """
        event = record.get("event")
        text = record.get("text") if isinstance(record.get("text"), str) else ""
        if event == "user":
            media = [
                str(media_path) for media_path in record.get("media_paths") or [] if media_path
            ]
            yield (
                "user",
                text or " ".join("[media: %s]" % media_path for media_path in media),
                "",
                "",
            )
        elif event == "message":
            kind = record.get("kind")
            tool_events = [
                item for item in record.get("tool_events") or [] if isinstance(item, dict)
            ]
            for entry in tool_events:
                name, call_id = str(entry.get("name") or ""), str(entry.get("call_id") or "")
                if entry.get("phase") == "start":
                    yield "tool_use", tool_args_text(entry.get("arguments")), name, call_id
                elif entry.get("phase") == "end":
                    result = entry.get("result")
                    body = result if isinstance(result, str) else compact_json(result)
                    yield "tool_result", body, name, call_id
                elif entry.get("phase") == "error":
                    yield "tool_result", str(entry.get("error") or ""), name, call_id
            if kind == "reasoning":
                if opts.include_thinking and text:
                    yield "thinking", text, "", ""
            elif kind in ("tool_hint", "progress"):
                if text and not tool_events:
                    yield "system", "%s: %s" % (str(kind).replace("_", " "), text), "", ""
            elif text:
                yield "assistant", text, "", ""
        elif event == "stream_end":
            if text and not record.get("merge_next"):
                yield "assistant", text, "", ""
        elif event == "reasoning_end":
            if opts.include_thinking and text:
                yield "thinking", text, "", ""
        elif event == "file_edit":
            for entry in record.get("edits") or []:
                if isinstance(entry, dict) and entry.get("phase") != "start":
                    yield (
                        "system",
                        "file edit: %s %s +%s -%s%s"
                        % (
                            entry.get("status") or entry.get("phase") or "",
                            entry.get("absolute_path") or entry.get("path") or "",
                            entry.get("added", 0),
                            entry.get("deleted", 0),
                            " (%s)" % entry["operation"] if entry.get("operation") else "",
                        ),
                        str(entry.get("tool") or ""),
                        str(entry.get("call_id") or ""),
                    )
        elif event == "fork_marker":
            yield "system", "webui fork marker: the fork starts here", "", ""
        elif event == "context_compaction":
            yield "system", "context compaction: %s" % (record.get("phase") or ""), "", ""
        elif isinstance(event, str) and event and event not in WEBUI_SILENT:
            yield "system", "webui event: %s" % event, "", ""

    def _load_object(self, artifact: Artifact, what: str) -> tuple[dict | None, Row | None]:
        """Return the JSON object in `artifact`, or None and the `parser:` row that says why."""
        try:
            value = json.loads(artifact.disk_path.read_text(encoding="utf-8", errors="replace"))
        except OSError as error:
            return None, self._file_row(
                artifact, "", "parser: unreadable %s: %s" % (what, error_reason(error))
            )
        except ValueError as error:
            return None, self._file_row(
                artifact, "", "parser: %s did not parse: %s" % (what, error)
            )
        if not isinstance(value, dict):
            return None, self._file_row(artifact, "", "parser: %s is not a JSON object" % what)
        return value, None

    def _setting_row(
        self, artifact: Artifact, timestamp: str, text: str, session_id: str = ""
    ) -> Row:
        """Return a `system` row at `timestamp`."""
        row = self.base_row(artifact)
        row.timestamp_utc = timestamp
        row.session_id = session_id
        row.turn_type = "system"
        row.text = compact(text)
        return row

    def _parse_config(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `permission setting:` row per standing setting of `config.json`, at its mtime.

        Only the settings that grant or restrict the agent are read, named as the file spells them;
        the file holds API keys.
        """
        data, problem = self._load_object(artifact, "config.json")
        if data is None:
            yield from [problem] if problem else []
            return
        timestamp = file_mtime(artifact)
        tail = " (from %s, file mtime)" % artifact.rel
        tools = data.get("tools")
        tools = tools if isinstance(tools, dict) else {}
        settings: list[tuple[str, object]] = []
        for names in CONFIG_TOOL_KEYS:
            hit = _first_key(tools, names)
            if hit:
                settings.append(("tools.%s" % hit[0], hit[1]))
        exec_config = tools.get("exec")
        exec_config = exec_config if isinstance(exec_config, dict) else {}
        for names in CONFIG_EXEC_KEYS:
            hit = _first_key(exec_config, names)
            if hit:
                settings.append(("tools.exec.%s" % hit[0], hit[1]))
        channels = data.get("channels")
        for name, config in (channels if isinstance(channels, dict) else {}).items():
            if isinstance(config, dict):
                hit = _first_key(config, ("allowFrom", "allow_from"))
                if hit:
                    settings.append(("channels.%s.%s" % (name, hit[0]), hit[1]))
        for key, value in settings:
            for entry in value if isinstance(value, list) else [value]:
                yield self._setting_row(
                    artifact,
                    timestamp,
                    "permission setting: %s=%s%s" % (key, setting_value(entry), tail),
                )

    def _parse_pairing(self, artifact: Artifact) -> Iterator[Row]:
        """Yield a row per approved chat sender, at the file's mtime, and per pending request.

        A pending request's row is at its `created_at`; its pairing code is not shown.
        """
        data, problem = self._load_object(artifact, "pairing.json")
        if data is None:
            yield from [problem] if problem else []
            return
        timestamp = file_mtime(artifact)
        approved = data.get("approved")
        for channel, senders in (approved if isinstance(approved, dict) else {}).items():
            for sender in senders if isinstance(senders, list) else []:
                yield self._setting_row(
                    artifact,
                    timestamp,
                    "permission setting: pairing.approved.%s=%s (from %s, file mtime)"
                    % (channel, setting_value(sender), artifact.rel),
                )
        pending = data.get("pending")
        for info in (pending if isinstance(pending, dict) else {}).values():
            if not isinstance(info, dict):
                continue
            yield self._setting_row(
                artifact,
                to_utc(info.get("created_at")),
                "permission setting: pairing.pending.%s=%s expires_at=%s%s (from %s, created_at)"
                % (
                    info.get("channel") or "",
                    setting_value(info.get("sender_id") or ""),
                    to_utc(info.get("expires_at")),
                    " delivery_status=%s" % info["delivery_status"]
                    if info.get("delivery_status")
                    else "",
                    artifact.rel,
                ),
            )

    def _parse_sidebar(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `webui sidebar entry:` row per cached entry, at its `created_at`."""
        data, problem = self._load_object(artifact, ".webui_session_index.json")
        if data is None:
            yield from [problem] if problem else []
            return
        entries = data.get("sessions")
        if not isinstance(entries, list):
            yield self._file_row(
                artifact, "", "parser: .webui_session_index.json has no sessions list"
            )
            return
        errors: list = []
        for i, entry in enumerate(entries, 1):
            if not isinstance(entry, dict):
                errors.append(
                    (0, "unparseable record %d (not a JSON object)" % i, compact_json(entry))
                )
                continue
            key = str(entry.get("key") or "")
            text = " ".join(
                part
                for part in (
                    "webui sidebar entry: key=%s" % key,
                    "title=%s" % entry["title"] if entry.get("title") else "",
                    "preview=%s" % entry["preview"] if entry.get("preview") else "",
                    "model_preset=%s" % entry["model_preset"] if entry.get("model_preset") else "",
                    "updated_at=%s" % entry["updated_at"] if entry.get("updated_at") else "",
                    "(from .webui_session_index.json version %s)" % data.get("version", ""),
                )
                if part
            )
            yield self._setting_row(artifact, to_utc(entry.get("created_at")), text, key)
        yield from error_rows(self, artifact, errors)

    def _parse_run(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the rows of a cron or trigger run record.

        The prompt as the session stored it (`user`) at the run's start, the reply (`assistant`) and
        a `cron run:` or `trigger run:` row with the run's status at `updated_at_ms`.
        """
        match = RUN_PATTERN.match(artifact.rel)
        assert match is not None  # parse() routes only matching paths here
        kind = match.group("kind")
        project = ""
        if match.group("ws") or artifact.home.project:
            original_match = ORIGINAL_RUN_PATTERN.match(artifact.original or "")
            project = original_match.group(1) if original_match else ""
        record, problem = self._load_object(artifact, "%s run record" % kind)
        if record is None:
            if problem:
                problem.project_path = project
                yield problem
            return
        session_id = str(record.get("session_key") or "")
        end = to_utc(record.get("updated_at_ms"))

        def make_row(timestamp: str, turn_type: str, text: str) -> Row:
            row = self._setting_row(artifact, timestamp, text, session_id)
            row.turn_type = turn_type
            row.project_path = project
            return row

        if kind == "cron":
            prompt_vars = record.get("prompt_vars")
            message = prompt_vars.get("message") if isinstance(prompt_vars, dict) else None
            if record.get("job_name") and isinstance(message, str):
                prompt = "Scheduled cron job triggered: %s\n\n%s" % (record["job_name"], message)
            else:
                prompt = str(record.get("rendered_prompt") or "")
            # `run_id` is `<job id>:<epoch ms at the run's start>:<hex>`.
            parts = str(record.get("run_id") or "").rsplit(":", 2)
            start = (
                to_utc(int(parts[1]))
                if len(parts) == 3 and parts[1].isascii() and parts[1].isdigit()
                else end
            )
            status = "cron run: job %s (%s) run %s status=%s" % (
                record.get("job_name") or "",
                record.get("job_id") or "",
                record.get("run_id") or "",
                record.get("status") or "",
            )
        else:
            label = str(record.get("trigger_name") or "").strip() or str(
                record.get("trigger_id") or ""
            )
            prompt = "Local trigger received: %s\n\n%s" % (label, record.get("content") or "")
            start = to_utc(record.get("created_at_ms")) or end
            status = "trigger run: trigger %s (%s) delivery %s status=%s attempts=%s" % (
                record.get("trigger_name") or "",
                record.get("trigger_id") or "",
                record.get("delivery_id") or record.get("run_id") or "",
                record.get("status") or "",
                record.get("attempts", ""),
            )
        if prompt.strip():
            yield make_row(start, "user", prompt)
        if isinstance(record.get("response"), str) and record["response"]:
            yield make_row(end, "assistant", record["response"])
        error = record.get("error") or record.get("last_error")
        yield make_row(end, "system", status + (" error=%s" % error if error else ""))

    def _parse_cli_history(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `user` row per entry of `history/cli_history`, with no session or project."""
        errors: list = []
        for line_number, timestamp, text in read_input_history(artifact.disk_path, errors):
            row = self.base_row(artifact)
            row.source_line = line_number
            row.timestamp_utc = to_utc(timestamp)
            row.turn_type = "user"
            row.text = compact(text)
            yield row
        yield from error_rows(self, artifact, errors)


__all__ = ["NanobotParser", "decode_stem"]
