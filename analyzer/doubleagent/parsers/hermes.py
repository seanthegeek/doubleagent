r"""Hermes Agent sessions.

Hermes keeps every conversation of a profile, whatever the front end, in one SQLite database
in OpenAI chat-completions message shape. Reads, under `<hermes>` (`.hermes<suffix>` or
`AppData/Local/hermes<suffix>`, the suffix empty or appended with no separator, then optionally
`profiles/<name>`; empty when the base is a Docker or Podman volume's `_data`): `state.db`
(primary); the secondary copies `state-snapshots/<id>/state.db`, `sessions/<id>.jsonl`,
`.hermes_history`, `cron/output/<job>/<time>.md`, `response_store.db` and a project's
`trajectory_samples.jsonl` and `failed_trajectories.jsonl`; and `pastes/pending.json`,
`config.yaml` (`command_allowlist` only), `[platforms/]pairing/<platform>-approved.json` and
the desktop app's `connections.json` (in `Hermes<suffix>`). Yields transcript rows and model,
permission, relay, deletion, scratch-directory, debug-upload and remote-backend `system` rows.

Fields the parser depends on:

* `sessions`: `id`, `source`, `model`, `parent_session_id`, `started_at`, `end_reason`,
  `cwd`, `git_branch`, `git_repo_root`, `title`, `archived`, `chat_id`, `chat_type`,
  `thread_id`, `user_id`, `display_name`, `origin_json`, `model_config` (`yolo_mode`).
* `messages`: `id`, `session_id`, `role`, `content` (`\x00json:` + JSON for parts),
  `tool_calls`, `tool_call_id`, `tool_name`, `timestamp`, `reasoning`, `reasoning_content`,
  `active`, `display_kind`; `session_model_usage` (`model`, `billing_provider`, `task`,
  `first_seen`), `state_meta` (`key`, `value`), `conversation_generations`,
  `hosted_room_events` and `hosted_rooms`.
* `responses` (`response_id`, `data`), `conversations` (`name`, `response_id`).

Validated against source, NousResearch/hermes-agent at 6bb377a (the schema 31 shapes at
8b66a51), with a synthetic fixture; no real install. Times without a zone (the history and
cron files) are emitted as if UTC. See `research/hermes.md`.
"""

from __future__ import annotations

import json
import re
import sqlite3
from collections.abc import Iterator

from ..dedup import TOLERANCE_SECONDS
from ..inputs import Artifact
from ..model import Row, compact
from ..sqlite_util import open_copy, sqlite_problem, table_names
from ..textlines import SPACE_OR_TAB, split_lines
from ..timeutil import to_utc
from .aider import read_input_history
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_mtime,
    iter_jsonl,
    line_text,
    setting_value,
    text_of,
    track_change,
)

# The Hermes home a path starts with: `.hermes<suffix>/` or
# `AppData/Local/hermes<suffix>/` under a user's home, `HERMES_DATA_DIR_SUFFIX`
# appended with no separator, or nothing when the collection base is a Docker or
# Podman named volume's `_data` (the `hermes|*hermes*` volume line), which holds
# the Hermes home itself; either one optionally followed by `profiles/<name>/`.
HOME = r"(?:(?:\.hermes[^/]*|AppData/Local/hermes[^/]*)/)?(?:profiles/[^/]+/)?"
DATABASE_PATTERN = re.compile(r"^%sstate\.db$" % HOME)
JSONL_PATTERN = re.compile(r"^%ssessions/[^/]+\.jsonl$" % HOME)
SNAPSHOT_PATTERN = re.compile(r"^%sstate-snapshots/([^/]+)/state\.db$" % HOME)
# A YAML comment starts at `#` after a space or tab, a mapping value at `:` before one.
YAML_COMMENT_PATTERN = re.compile(r"[ \t]#")
YAML_MAPPING_PATTERN = re.compile(r":[ \t]")
SNAPSHOT_TIME_PATTERN = re.compile(r"^(\d{4})(\d{2})(\d{2})-(\d{2})(\d{2})(\d{2})(?:-|$)")
PASTES_PATTERN = re.compile(r"^%spastes/pending\.json$" % HOME)
# A paste.rs upload's `expire_at` is its upload time plus this (hermes_cli/debug.py).
PASTE_LIFETIME = 21600
# Read from hermes-agent 6bb377a: the standing permission files,
# the prompt history, the cron run archive, the API server's response store,
# the trajectory files a run with --save-trajectories leaves in its working
# directory (project files; the base is the project) and the desktop app's
# connection registry in its Electron userData directory.
CONFIG_PATTERN = re.compile(r"^%sconfig\.yaml$" % HOME)
PAIRING_PATTERN = re.compile(r"^%s((?:platforms/)?pairing)/([^/]+)-approved\.json$" % HOME)
HISTORY_PATTERN = re.compile(r"^%s\.hermes_history$" % HOME)
CRON_PATTERN = re.compile(r"^%scron/output/([^/]+)/([^/]+)\.md$" % HOME)
CRON_TIME_PATTERN = re.compile(r"^(\d{4}-\d\d-\d\d)_(\d\d)-(\d\d)-(\d\d)$")
RESPONSES_PATTERN = re.compile(r"^%sresponse_store\.db$" % HOME)
TRAJECTORY_PATTERN = re.compile(r"^(?:trajectory_samples|failed_trajectories)\.jsonl$")
CONNECTIONS_PATTERN = re.compile(
    r"^(?:\.config|Library/Application Support|AppData/Roaming)/Hermes[^/]*/connections\.json$"
)
# The classic CLI's history entries and the cron run files carry host-local
# times with no zone (prompt_toolkit's `datetime.now()`, `hermes_time.now()`
# formatted without an offset), emitted as if UTC; the window that pairs them
# with the store's UTC times spans every zone offset.
LOCAL_TOLERANCE = 14 * 3600 + TOLERANCE_SECONDS
# gateway/config.py Platform values that are messaging platforms: every one
# but `local` (the CLI) and `api_server` (the OpenAI-compatible HTTP API).
MESSAGING = frozenset(
    (
        "telegram",
        "discord",
        "whatsapp",
        "whatsapp_cloud",
        "slack",
        "signal",
        "mattermost",
        "matrix",
        "email",
        "sms",
        "dingtalk",
        "webhook",
        "msgraph_webhook",
        "feishu",
        "wecom",
        "wecom_callback",
        "weixin",
        "bluebubbles",
        "qqbot",
        "yuanbao",
        "relay",
    )
)
NOT_RELAYED = frozenset(("local", "api_server"))
RETIRED_PREFIX = "gateway.retired_session.v1."
WORKTREE_PATTERN = re.compile(r"^(.*)[/\\]\.worktrees[/\\][^/\\]+[/\\]?$")
WORKTREE_BRANCHES = ("hermes/", "hermes-subagent/")
# The cron run document's length frames (cron/scheduler_prompt.py).
CRON_PROMPT_FRAME_PATTERN = re.compile(r"(?m)^\*\*Prompt Characters:\*\* ([0-9]+)\n## Prompt\n\n")
CRON_RESPONSE_FRAME_PATTERN = re.compile(r"\*\*Response Characters:\*\* ([0-9]+)\n## Response\n\n")
CRON_PROMPT_HEADING = "## Prompt\n\n"
CRON_RESPONSE_HEADING = "## Response\n\n"
# ShareGPT `from` values of a trajectory entry (agent/agent_runtime_helpers.py).
TRAJECTORY_TYPES = {"human": "user", "gpt": "assistant", "tool": "tool_result"}

JSON_PREFIX = "\x00json:"
# The TUI gateway's model-switch marker (tui_gateway/server.py:1169-1209 at
# 6bb377a): `<prefix><model>[ via provider <provider>]<end> ...]`.
SWITCH_PREFIX = "[System: The active model for this chat has changed to "
SWITCH_END = ". From this point forward,"
SWITCH_PROVIDER = " via provider "
ROLE_TYPES = {"user": "user", "assistant": "assistant", "tool": "tool_result", "system": "system"}


def decode_content(value):
    """Return `messages.content` as stored: text, or the parts behind the JSON sentinel."""
    if isinstance(value, str) and value.startswith(JSON_PREFIX):
        try:
            return json.loads(value[len(JSON_PREFIX) :])
        except ValueError:
            return value[len(JSON_PREFIX) :]
    return value


def content_text(value) -> str:
    """Return the text of a `messages.content` value, image parts as `[image]`."""
    content = decode_content(value)
    if isinstance(content, list):
        content = [
            {"type": "image"}
            if isinstance(block, dict) and block.get("type") in ("image_url", "input_image")
            else block
            for block in content
        ]
    return text_of(content)


def _json(value, default):
    """Decode a JSON text, returning `default` for an empty or undecodable value."""
    if value is None or value == "":
        return default
    if not isinstance(value, str):
        return value
    try:
        return json.loads(value)
    except ValueError:
        return default


def model_switch(content) -> tuple[str, str] | None:
    """Return (model, provider) named by a model-switch marker, or None when the text is not one.

    The provider is `""` when the marker names none.
    """
    if not isinstance(content, str) or not content.startswith(SWITCH_PREFIX):
        return None
    head, separator, _ = content[len(SWITCH_PREFIX) :].partition(SWITCH_END)
    if not separator:
        return None
    model, _, provider = head.partition(SWITCH_PROVIDER)
    return (model, provider) if model else None


def tool_calls(message: dict, line) -> tuple[list, list]:
    """Return the `tool_calls` of `message` as (calls, errors).

    A value that is not valid JSON, is not a list (a lone object is taken as a one-call list) or
    holds an entry that is not an object gives an `errors` entry carrying the value as stored, for
    `error_rows`; the entries that are objects are still returned.
    """
    value = message.get("tool_calls")
    if value is None or value == "":
        return [], []
    raw = value if isinstance(value, str) else compact_json(value)
    label = "unparseable record %s" % line
    if isinstance(value, str):
        try:
            value = json.loads(value)
        except ValueError as error:
            return [], [(line, "%s (tool_calls is not valid JSON: %s)" % (label, error), raw)]
    if isinstance(value, dict):
        value = [value]
    if not isinstance(value, list):
        return [], [(line, "%s (tool_calls is not a list)" % label, raw)]
    calls = [call for call in value if isinstance(call, dict)]
    if len(calls) != len(value):
        return calls, [(line, "%s (tool_calls entry is not an object)" % label, raw)]
    return calls, []


def call_summary(arguments) -> str:
    """Return the `command`, `path`, `file_path`, `url` or `query` argument, else the arguments."""
    args = _json(arguments, arguments)
    if isinstance(args, dict):
        for k in ("command", "path", "file_path", "url", "query"):
            if args.get(k) not in (None, ""):
                return str(args[k])
        return compact_json(args)
    return args if isinstance(args, str) else compact_json(args)


def _join(*parts: str) -> str:
    """Join the non-empty parts with ` | `."""
    return " | ".join(part for part in parts if part)


def _dash(value) -> str:
    """A row text field: the value as text, `-` when it is absent or empty."""
    if value is None or value == "":
        return "-"
    return setting_value(value)


def terminal_decision(content) -> str:
    """Return the `permission decision:` text, less its label, of a `terminal` tool result.

    The result's JSON records the approval outcome (tools/terminal_tool.py:1101-1126 at 6bb377a):
    `approved (<note>)`, `blocked <error>` with ` user_summary=<summary>` when a person was asked,
    or `pending pattern=<pattern_key> smart_denied=<bool>`; `""` for any other result.
    """
    value = decode_content(content)
    if isinstance(value, str):
        try:
            value = json.loads(value)
        except ValueError:
            return ""
    if not isinstance(value, dict):
        return ""
    if isinstance(value.get("approval"), str) and value["approval"]:
        return "approved (%s)" % value["approval"]
    status = value.get("status")
    if status == "blocked":
        text = "blocked %s" % _dash(value.get("error"))
        if value.get("user_summary"):
            text += " user_summary=%s" % setting_value(value["user_summary"])
        return text
    if status == "pending_approval":
        return "pending pattern=%s smart_denied=%s" % (
            _dash(value.get("pattern_key")),
            setting_value(bool(value.get("smart_denied"))),
        )
    return ""


def yaml_scalar(value: str) -> str | None:
    """Return a one-line YAML scalar as a string, or None for anything else.

    Double-quoted (JSON escapes), single-quoted (`''` for a quote) and plain scalars (a trailing `
    #` comment removed) are read; a mapping, a flow collection, an alias, a block scalar or an
    unterminated quote gives None.
    """
    if value[:1] in ("'", '"'):
        quote, i = value[0], 1
        while i < len(value):
            # a backslash escape in a double-quoted scalar, `''` in a single-quoted one
            escaped = value[i] == "\\" if quote == '"' else value[i : i + 2] == "''"
            if escaped:
                i += 2
            elif value[i] == quote:
                break
            else:
                i += 1
        rest = value[i + 1 :].strip(SPACE_OR_TAB)
        if i >= len(value) or (rest and not rest.startswith("#")):
            return None
        if quote == "'":
            return value[1:i].replace("''", "'")
        try:
            out = json.loads(value[: i + 1])
        except ValueError:
            return None
        return out if isinstance(out, str) else None
    plain = YAML_COMMENT_PATTERN.split(value, maxsplit=1)[0].rstrip(SPACE_OR_TAB)
    if (
        not plain
        or plain[0] in "[]{}&*!|>%@`#,"
        or YAML_MAPPING_PATTERN.search(plain)
        or plain.endswith(":")
    ):
        return None
    return plain


def read_flat_list(lines: list[str], key: str) -> tuple[list[tuple[int, str]], str]:
    """Return the strings of the top-level YAML block list `key` and the first problem.

    The strings come as (line number, string); the problem names the first line that is not a `-
    <scalar>` item of the list, `""` when there is none. An absent key, `key:` with no items and
    `key: []` give no entries.
    """
    entries: list[tuple[int, str]] = []
    start = 0
    for line_number, line in enumerate(lines, 1):
        if line.startswith(key + ":"):
            start = line_number
            rest = YAML_COMMENT_PATTERN.split(line[len(key) + 1 :], maxsplit=1)[0].strip(
                SPACE_OR_TAB
            )
            if rest not in ("", "[]", "null", "~"):
                return [], "line %d: %s is not a block list" % (line_number, key)
            break
    if not start:
        return [], ""
    for line_number in range(start + 1, len(lines) + 1):
        line = lines[line_number - 1]
        stripped = line.strip(SPACE_OR_TAB)
        if not stripped or stripped.startswith("#"):
            continue
        if line[0] not in SPACE_OR_TAB and not stripped.startswith("-"):
            break  # the next top-level key
        item = stripped[2:].strip(SPACE_OR_TAB)
        value = yaml_scalar(item) if stripped.startswith("- ") else None
        if value is None:
            return entries, "line %d: not a string item of %s" % (line_number, key)
        entries.append((line_number, value))
    return entries, ""


def split_cron_doc(document: str) -> tuple[str, str] | None:
    """Return (prompt, response) of a cron run document, or None.

    The length frames (`**Prompt Characters:** <n>`, `**Response Characters:** <n>`) are used when
    the writer stamped them, else the first `## Prompt` and the last `## Response` heading; a
    document with neither (a failed, blocked or script-gated run) gives None.
    """
    frame = CRON_PROMPT_FRAME_PATTERN.search(document)
    if frame and document.find(CRON_PROMPT_HEADING) == frame.end() - len(CRON_PROMPT_HEADING):
        end = frame.end() + int(frame.group(1))
        reply = CRON_RESPONSE_FRAME_PATTERN.match(document, end + 2)
        if reply:
            prompt = document[frame.end() : end]
            return prompt, document[reply.end() : reply.end() + int(reply.group(1))]
    start = document.find(CRON_PROMPT_HEADING)
    cut = document.rfind(CRON_RESPONSE_HEADING)
    if start < 0 or cut <= start:
        return None
    prompt = document[start + len(CRON_PROMPT_HEADING) : cut]
    if prompt.endswith("\n\n"):
        prompt = prompt[:-2]
    reply_text = document[cut + len(CRON_RESPONSE_HEADING) :]
    if reply_text.endswith("\n"):
        reply_text = reply_text[:-1]
    return prompt, reply_text


class HermesParser(Parser):
    """Parse Hermes's session databases, their copies, histories and settings."""

    agent = "hermes"
    name = "hermes"
    has_secondary = True
    records_relay = True

    WANTED = (
        DATABASE_PATTERN,
        SNAPSHOT_PATTERN,
        JSONL_PATTERN,
        PASTES_PATTERN,
        CONFIG_PATTERN,
        PAIRING_PATTERN,
        HISTORY_PATTERN,
        CRON_PATTERN,
        RESPONSES_PATTERN,
        TRAJECTORY_PATTERN,
        CONNECTIONS_PATTERN,
    )
    SECONDARY = (
        SNAPSHOT_PATTERN,
        JSONL_PATTERN,
        HISTORY_PATTERN,
        CRON_PATTERN,
        RESPONSES_PATTERN,
        TRAJECTORY_PATTERN,
    )

    def wants(self, artifact: Artifact) -> bool:
        """Return True for the Hermes files this parser reads."""
        return any(pattern.match(artifact.rel) for pattern in self.WANTED)

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for the files that repeat the events of `state.db`."""
        return any(pattern.match(artifact.rel) for pattern in self.SECONDARY)

    def secondary_tolerance(self, artifact: Artifact) -> float:
        """Return `LOCAL_TOLERANCE` for the history and cron run files, else the default window."""
        if HISTORY_PATTERN.match(artifact.rel) or CRON_PATTERN.match(artifact.rel):
            return LOCAL_TOLERANCE
        return TOLERANCE_SECONDS

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Hermes file."""
        rel = artifact.rel
        snapshot = SNAPSHOT_PATTERN.match(rel)
        if snapshot:
            yield self._snapshot_row(artifact, snapshot.group(1))
            yield from self._parse_database(artifact, opts)
        elif DATABASE_PATTERN.match(rel):
            yield from self._parse_database(artifact, opts)
        elif PASTES_PATTERN.match(rel):
            yield from self._parse_pastes(artifact)
        elif CONFIG_PATTERN.match(rel):
            yield from self._parse_config(artifact)
        elif pairing := PAIRING_PATTERN.match(rel):
            yield from self._parse_pairing(artifact, pairing)
        elif HISTORY_PATTERN.match(rel):
            yield from self._parse_history(artifact)
        elif cron := CRON_PATTERN.match(rel):
            yield from self._parse_cron(artifact, cron)
        elif RESPONSES_PATTERN.match(rel):
            yield from self._parse_responses(artifact, opts)
        elif TRAJECTORY_PATTERN.match(rel):
            yield from self._parse_trajectory(artifact)
        elif CONNECTIONS_PATTERN.match(rel):
            yield from self._parse_connections(artifact)
        else:
            yield from self._parse_jsonl(artifact, opts)

    def _snapshot_row(self, artifact: Artifact, snap_id: str) -> Row:
        """Return the `system` row naming a quick snapshot, at the UTC time its id starts with.

        `backup.py` names a snapshot `%Y%m%d-%H%M%S[-label]`.
        """
        row = self._system(artifact, 0, {}, "state snapshot %s: older copy of state.db" % snap_id)
        match = SNAPSHOT_TIME_PATTERN.match(snap_id)
        if match:
            row.timestamp_utc = to_utc("%s-%s-%sT%s:%s:%sZ" % match.groups())
        return row

    def _row(self, artifact: Artifact, line: int, session: dict, timestamp: str = "") -> Row:
        row = self.base_row(artifact)
        row.source_line = line
        row.session_id = str(session.get("id") or "")
        row.project_path = str(session.get("cwd") or session.get("git_repo_root") or "")
        row.git_branch = str(session.get("git_branch") or "")
        row.model = str(session.get("model") or "")
        row.timestamp_utc = timestamp
        return row

    def _system(self, artifact: Artifact, line: int, session: dict, text: str) -> Row:
        row = self._row(artifact, line, session)
        row.turn_type = "system"
        row.text = text
        return row

    def _message_rows(
        self,
        artifact: Artifact,
        message: dict,
        session: dict,
        line: int,
        tool_names: dict[str, str],
        opts: Options,
        switches: dict[str, str],
        usage: dict[str, list[dict]] | None = None,
    ) -> Iterator[Row]:
        timestamp = to_utc(message.get("timestamp"))
        role = str(message.get("role") or "")
        if message.get("display_kind") == "model_switch":
            yield self._switch_row(artifact, message, session, line, switches, usage or {})
            return
        marker = "[rewound] " if message.get("active", 1) in (0, "0", False) else ""

        def make_row(turn_type: str, text: str) -> Row:
            row = self._row(artifact, line, session, timestamp)
            row.turn_type = turn_type
            row.text = compact(marker + text)
            return row

        if opts.include_thinking:
            reasoning = message.get("reasoning") or message.get("reasoning_content")
            if reasoning:
                yield make_row("thinking", str(reasoning))
        text = content_text(message.get("content"))
        turn_type = ROLE_TYPES.get(role, "system")
        if turn_type == "tool_result":
            row = make_row("tool_result", text)
            row.tool_use_id = str(message.get("tool_call_id") or "")
            row.tool_name = str(message.get("tool_name") or tool_names.get(row.tool_use_id, ""))
            yield row
            decision = (
                terminal_decision(message.get("content")) if row.tool_name == "terminal" else ""
            )
            if decision:
                permission = self._row(artifact, line, session, timestamp)
                permission.turn_type = "system"
                permission.text = compact("permission decision: " + decision)
                permission.tool_name, permission.tool_use_id = row.tool_name, row.tool_use_id
                yield permission
        elif text:
            if turn_type == "system" and role not in ("", "system"):
                text = "%s: %s" % (role, text)
            yield make_row(turn_type, text)
        calls, errors = tool_calls(message, line)
        for call in calls:
            function = call.get("function")
            if not isinstance(function, dict):
                function = {}
            row = make_row("tool_use", call_summary(function.get("arguments")))
            row.tool_use_id = str(call.get("id") or call.get("call_id") or "")
            row.tool_name = str(function.get("name") or "")
            tool_names[row.tool_use_id] = row.tool_name
            yield row
        yield from error_rows(
            self,
            artifact,
            errors,
            str(session.get("id") or ""),
            str(session.get("cwd") or session.get("git_repo_root") or ""),
        )

    def _switch_row(
        self,
        artifact: Artifact,
        message: dict,
        session: dict,
        line: int,
        switches: dict[str, str],
        usage: dict[str, list[dict]],
    ) -> Row:
        """Return the `system` row of a TUI gateway model-switch marker.

        The marker names only the new model: the old one is the session's previous marker's, else
        the last main-loop `session_model_usage` model first seen before it, else unknown. A later
        switch sets the marker's `active` to 0, which is not a rewind, so the row has no `[rewound]`
        prefix; a marker whose text does not parse is a `model_switch: <text>` row.
        """
        row = self._row(artifact, line, session, to_utc(message.get("timestamp")))
        row.turn_type = "system"
        session_id = row.session_id
        parsed = model_switch(message.get("content"))
        if parsed is None:
            row.text = compact("model_switch: " + content_text(message.get("content")))
            return row
        model, provider = parsed
        old = switches.get(session_id, "")
        switch_time = message.get("timestamp")
        if not old and isinstance(switch_time, (int, float)):
            for usage_row in usage.get(session_id) or []:
                first = usage_row.get("first_seen")
                if isinstance(first, (int, float)) and first < switch_time:
                    old = str(usage_row.get("model") or "")
        switches[session_id] = model
        row.text = "model: %s -> %s" % (old, model) if old and old != model else "model: " + model
        if provider:
            row.text += " provider=%s" % provider
        row.model = model
        return row

    def _model_rows(
        self,
        artifact: Artifact,
        session: dict,
        start: Row,
        usage: dict[str, list[dict]],
        seen: dict[str, str],
    ) -> Iterator[Row]:
        session_id = start.session_id
        uses = usage.get(session_id) or []
        if not uses:
            text = track_change("model", seen, session_id, str(session.get("model") or ""))
            if text:
                yield self._copy(start, text, str(session.get("model") or ""))
            return
        for usage_row in uses:
            model = str(usage_row.get("model") or "")
            text = track_change("model", seen, session_id, model)
            if not text:
                continue
            if usage_row.get("billing_provider"):
                text += " provider=%s" % usage_row["billing_provider"]
            row = self._copy(start, text, model)
            row.source_line = usage_row["_rowid"]
            row.timestamp_utc = to_utc(usage_row.get("first_seen")) or start.timestamp_utc
            yield row

    @staticmethod
    def _copy(base: Row, text: str, model: str) -> Row:
        row = Row(**base.__dict__)
        row.turn_type = "system"
        row.model = model
        row.text = text
        return row

    # -- state.db ------------------------------------------------------------------------
    def _parse_database(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        problem = sqlite_problem(artifact.disk_path)
        if problem:
            yield self._system(artifact, 0, {}, "parser: " + problem)
            return
        try:
            with open_copy(artifact.disk_path) as connection:
                tables = table_names(connection)
                if "messages" not in tables:
                    return
                # (session_id, project_path, error entry) per record that does
                # not decode, reported after the file's other rows.
                record_errors: list[tuple[str, str, tuple]] = []
                sessions: dict[str, dict] = {}
                usage = self._usage(connection, tables)
                if "sessions" in tables:
                    seen_model: dict[str, str] = {}
                    for session_row in connection.execute(
                        "select rowid as _rowid, * from sessions order by started_at, rowid"
                    ):
                        session_row = dict(session_row)
                        sessions[str(session_row.get("id") or "")] = session_row
                        yield from self._session_rows(
                            artifact, session_row, usage, seen_model, record_errors
                        )
                tool_names: dict[str, str] = {}
                switches: dict[str, str] = {}
                for message in connection.execute("select * from messages order by timestamp, id"):
                    message = dict(message)
                    session_id = str(message.get("session_id") or "")
                    session = sessions.get(session_id) or {"id": session_id}
                    yield from self._message_rows(
                        artifact,
                        message,
                        session,
                        message.get("id") or 0,
                        tool_names,
                        opts,
                        switches,
                        usage,
                    )
                if "state_meta" in tables:
                    yield from self._retired_rows(artifact, connection, sessions, record_errors)
                if "conversation_generations" in tables:
                    yield from self._generation_rows(artifact, connection)
                if "hosted_room_events" in tables:
                    yield from self._room_rows(artifact, connection, tables, record_errors)
                for session_id, project, entry in record_errors:
                    yield from error_rows(self, artifact, [entry], session_id, project)
        except sqlite3.DatabaseError as error:
            yield self._system(artifact, 0, {}, "parser: SQLite error: %s" % error)

    @staticmethod
    def _usage(connection, tables) -> dict[str, list[dict]]:
        """Return the main-loop `session_model_usage` rows per session, by `first_seen`."""
        usage: dict[str, list[dict]] = {}
        if "session_model_usage" in tables:
            for usage_row in connection.execute(
                "select rowid as _rowid, * from session_model_usage order by first_seen, rowid"
            ):
                usage_row = dict(usage_row)
                if not usage_row.get("task"):
                    usage.setdefault(str(usage_row.get("session_id") or ""), []).append(usage_row)
        return usage

    @staticmethod
    def _column_json(session: dict, column: str, record_errors: list) -> dict:
        """Return `sessions.<column>` decoded as a JSON object, `{}` when it is empty.

        A value that does not decode to an object is appended to `record_errors` with the value as
        stored, and gives `{}`.
        """
        raw = session.get(column)
        if raw is None or raw == "":
            return {}
        session_id = str(session.get("id") or "")
        project = str(session.get("cwd") or session.get("git_repo_root") or "")
        label = "unparseable record %s (sessions.%s" % (session.get("_rowid"), column)
        try:
            value = json.loads(raw) if isinstance(raw, str) else raw
        except ValueError as error:
            record_errors.append(
                (session_id, project, (session.get("_rowid"), "%s: %s)" % (label, error), raw))
            )
            return {}
        if not isinstance(value, dict):
            entry = (session.get("_rowid"), "%s: not a JSON object)" % label, str(raw))
            record_errors.append((session_id, project, entry))
            return {}
        return value

    def _session_rows(
        self,
        artifact: Artifact,
        session: dict,
        usage: dict[str, list[dict]],
        seen_model: dict[str, str],
        record_errors: list,
    ) -> Iterator[Row]:
        """Yield a session's start row, its model rows, and its relay, `/yolo` and worktree rows."""
        origin = self._column_json(session, "origin_json", record_errors)
        config = self._column_json(session, "model_config", record_errors)
        imported = origin.get("imported_from")
        imported_text = ""
        if isinstance(imported, dict):
            imported_text = "imported from %s %s session %s" % (
                _dash(imported.get("tool")),
                _dash(imported.get("path")),
                _dash(imported.get("foreign_session_id")),
            )
        start = self._system(
            artifact,
            session["_rowid"],
            session,
            compact(
                _join(
                    "session start",
                    str(session.get("title") or ""),
                    "source=%s" % session["source"] if session.get("source") else "",
                    "parent=%s" % session["parent_session_id"]
                    if session.get("parent_session_id")
                    else "",
                    "end=%s" % session["end_reason"] if session.get("end_reason") else "",
                    "archived=1" if session.get("archived") in (1, "1", True) else "",
                    imported_text,
                ),
            ),
        )
        start.timestamp_utc = to_utc(session.get("started_at"))
        yield start
        yield from self._model_rows(artifact, session, start, usage, seen_model)
        source = str(session.get("source") or "")
        if source in MESSAGING or (session.get("chat_id") and source not in NOT_RELAYED):
            yield self._copy(
                start,
                "session relayed: %s chat %s type %s thread %s user %s name %s display %s"
                " (from state.db sessions, started_at)"
                % (
                    source,
                    _dash(session.get("chat_id")),
                    _dash(session.get("chat_type")),
                    _dash(session.get("thread_id")),
                    _dash(session.get("user_id")),
                    _dash(origin.get("user_name")),
                    _dash(session.get("display_name") or origin.get("chat_name")),
                ),
                start.model,
            )
        if "yolo_mode" in config:
            yield self._copy(
                start,
                "permission setting: yolo_mode=%s (from state.db sessions.model_config, started_at)"
                % setting_value(config["yolo_mode"]),
                start.model,
            )
        cwd = str(session.get("cwd") or "")
        tree = WORKTREE_PATTERN.match(cwd)
        branch = str(session.get("git_branch") or "")
        if tree and branch.startswith(WORKTREE_BRANCHES):
            yield self._copy(
                start,
                "scratch directory: %s (git worktree the agent created in %s, branch %s)"
                % (cwd, tree.group(1), branch),
                start.model,
            )

    def _retired_rows(
        self, artifact, connection, sessions: dict, record_errors: list
    ) -> Iterator[Row]:
        """Yield one `session deleted:` row per `state_meta` deletion fence, at its `retired_at`.

        The key is `gateway.retired_session.v1.<session id>` (hermes_state_mutation_retirement.py at
        6bb377a); ` sweep` follows the id when a sweep, not a delete, retired it (`expires`).
        """
        for table_row in connection.execute(
            "select rowid as _rowid, key, value from state_meta where key like ? order by rowid",
            (RETIRED_PREFIX + "%",),
        ):
            session_id = str(table_row["key"])[len(RETIRED_PREFIX) :]
            session = sessions.get(session_id) or {"id": session_id}
            project = str(session.get("cwd") or session.get("git_repo_root") or "")
            try:
                value = json.loads(table_row["value"] or "")
                reason = "not a JSON object"
            except (TypeError, ValueError) as error:
                value, reason = None, str(error)
            if not isinstance(value, dict):
                label = "unparseable record %s (state_meta.value: %s)" % (
                    table_row["_rowid"],
                    reason,
                )
                record_errors.append(
                    (
                        session_id,
                        project,
                        (table_row["_rowid"], label, str(table_row["value"] or "")),
                    )
                )
                continue
            row = self._system(
                artifact,
                table_row["_rowid"],
                session,
                "session deleted: %s%s (from state.db state_meta gateway.retired_session.v1,"
                " retired_at)" % (session_id, " sweep" if value.get("expires") else ""),
            )
            row.timestamp_utc = to_utc(value.get("retired_at"))
            yield row

    def _generation_rows(self, artifact, connection) -> Iterator[Row]:
        """Yield one `relay peer:` row per `conversation_generations` row, with no time.

        Hermes never deletes these rows, so they name every messaging chat that had a conversation.
        """
        for table_row in connection.execute(
            "select rowid as _rowid, source, session_key, generation"
            " from conversation_generations order by rowid"
        ):
            yield self._system(
                artifact,
                table_row["_rowid"],
                {},
                "relay peer: %s %s generation %s (from state.db conversation_generations)"
                % (
                    _dash(table_row["source"]),
                    _dash(table_row["session_key"]),
                    _dash(table_row["generation"]),
                ),
            )

    def _room_rows(self, artifact, connection, tables, record_errors: list) -> Iterator[Row]:
        """Yield one `system` row per Bot Mode room event, at its `created_at`.

        The events are `hosted_room_events` (gateway/hosted_rooms.py at 6bb377a): the text of a
        `message.user` or `message.member` event, the payload as compact JSON for the other kinds.
        Whether a room message is also a `messages` row is not known, so these are never `user`
        rows.
        """
        names: dict[str, str] = {}
        if "hosted_rooms" in tables:
            for table_row in connection.execute("select room_id, name from hosted_rooms"):
                names[str(table_row["room_id"])] = str(table_row["name"] or "")
        for table_row in connection.execute(
            "select rowid as _rowid, * from hosted_room_events order by created_at, room_id, seq"
        ):
            table_row = dict(table_row)
            parsed = [
                self._room_json(table_row, column, record_errors)
                for column in ("actor_json", "payload_json")
            ]
            actor, payload = parsed
            if actor is None or payload is None:
                continue
            kind = str(table_row.get("kind") or "")
            room = str(table_row.get("room_id") or "")
            body = payload.get("text") if kind in ("message.user", "message.member") else None
            row = self._system(
                artifact,
                table_row["_rowid"],
                {},
                "bot mode room %s (%s) seq %s %s actor %s:%s (%s): %s"
                % (
                    _dash(names.get(room)),
                    room,
                    _dash(table_row.get("seq")),
                    kind,
                    _dash(actor.get("kind")),
                    _dash(actor.get("id")),
                    _dash(actor.get("display_name")),
                    body if isinstance(body, str) else compact_json(payload),
                ),
            )
            row.timestamp_utc = to_utc(table_row.get("created_at"))
            yield row

    @staticmethod
    def _room_json(room_event: dict, column: str, record_errors: list) -> dict | None:
        """Return `hosted_room_events.<column>` of an event as a JSON object.

        None, with an entry in `record_errors`, when the value does not decode to one.
        """
        try:
            value = json.loads(room_event.get(column) or "")
            reason = "not a JSON object"
        except (TypeError, ValueError) as error:
            value, reason = None, str(error)
        if isinstance(value, dict):
            return value
        label = "unparseable record %s (hosted_room_events.%s: %s)" % (
            room_event["_rowid"],
            column,
            reason,
        )
        record_errors.append(
            ("", "", (room_event["_rowid"], label, str(room_event.get(column) or "")))
        )
        return None

    # -- sessions/<id>.jsonl -------------------------------------------------------------
    def _parse_jsonl(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list[tuple] = []
        session = {"id": artifact.disk_path.name[: -len(".jsonl")]}
        tool_names: dict[str, str] = {}
        switches: dict[str, str] = {}
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            yield from self._message_rows(
                artifact, record, session, line_number, tool_names, opts, switches
            )
        yield from error_rows(
            self,
            artifact,
            errors,
            str(session.get("id") or ""),
            str(session.get("cwd") or session.get("git_repo_root") or ""),
        )

    # -- pastes/pending.json -------------------------------------------------------------
    def _parse_pastes(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `debug report uploaded:` row per `pastes/pending.json` entry.

        The time is `expire_at` minus the six-hour paste lifetime. A file that is not a JSON list is
        one `parser:` row, and an entry that is not an object with a `url` its own row.
        """
        try:
            document = json.loads(artifact.disk_path.read_bytes().decode("utf-8", errors="replace"))
        except (OSError, ValueError) as error:
            yield from error_rows(
                self, artifact, [(0, "pastes/pending.json did not parse (%s)" % error, "")]
            )
            return
        if not isinstance(document, list):
            yield from error_rows(
                self, artifact, [(0, "pastes/pending.json is not a JSON list", "")]
            )
            return
        errors: list = []
        for position, entry in enumerate(document, 1):
            if not isinstance(entry, dict) or not entry.get("url"):
                errors.append(
                    (
                        position,
                        "unparseable record %d (not an object with a url)" % position,
                        compact_json(entry),
                    )
                )
                continue
            row = self._system(
                artifact,
                position,
                {},
                "debug report uploaded: %s (from pastes/pending.json, expire_at minus 6 hours)"
                % entry["url"],
            )
            expire = entry.get("expire_at")
            if isinstance(expire, (int, float)) and not isinstance(expire, bool):
                row.timestamp_utc = to_utc(expire - PASTE_LIFETIME)
            yield row
        yield from error_rows(self, artifact, errors)

    def _read_json(self, artifact: Artifact, name: str, errors: list):
        """Return the JSON document of `artifact`, or None with a whole-file entry in `errors`."""
        try:
            return json.loads(artifact.disk_path.read_bytes().decode("utf-8", errors="replace"))
        except (OSError, ValueError) as error:
            errors.append((0, "%s did not parse (%s)" % (name, error), ""))
            return None

    def _setting(self, artifact: Artifact, line: int, text: str, timestamp: str) -> Row:
        row = self._system(artifact, line, {}, text)
        row.timestamp_utc = timestamp
        return row

    # -- config.yaml ---------------------------------------------------------------------
    def _parse_config(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `permission setting: command_allowlist=<pattern>` row per allowlist entry.

        The entries are the top-level `command_allowlist` block list that an "always" answer writes
        (tools/approval.py:411-424 at 6bb377a); the nested `approvals` settings are not read. Any
        other construct under the key is one whole-file `parser:` row after the entries before it.
        """
        errors: list = []
        try:
            text = artifact.disk_path.read_bytes().decode("utf-8", errors="replace")
        except OSError as error:
            errors.append((0, "config.yaml could not be read (%s)" % error, ""))
            text = ""
        entries, problem = read_flat_list(split_lines(text), "command_allowlist")
        timestamp = file_mtime(artifact) if entries else ""
        for line_number, pattern in entries:
            yield self._setting(
                artifact,
                line_number,
                "permission setting: command_allowlist=%s (from config.yaml, file mtime)" % pattern,
                timestamp,
            )
        if problem:
            errors.append(
                (0, "config.yaml command_allowlist is not a flat list (%s)" % problem, "")
            )
        yield from error_rows(self, artifact, errors)

    # -- pairing/<platform>-approved.json ------------------------------------------------
    def _parse_pairing(self, artifact: Artifact, match: re.Match) -> Iterator[Row]:
        """Yield one `permission setting: pairing.<platform>.<user id>` row per approved user.

        The users are those the gateway's owner approved to drive the agent (gateway/pairing.py:
        432-438 at 6bb377a), each at its `approved_at`.
        """
        name = "%s/%s-approved.json" % (match.group(1), match.group(2))
        errors: list = []
        document = self._read_json(artifact, name, errors)
        if document is not None and not isinstance(document, dict):
            errors.append((0, "%s is not a JSON object" % name, ""))
            document = None
        for position, (user, info) in enumerate((document or {}).items(), 1):
            if not isinstance(info, dict):
                label = "unparseable record %d (not an object)" % position
                errors.append((position, label, compact_json({user: info})))
                continue
            yield self._setting(
                artifact,
                position,
                "permission setting: pairing.%s.%s user_name=%s (from %s, approved_at)"
                % (match.group(2), user, _dash(info.get("user_name")), name),
                to_utc(info.get("approved_at")),
            )
        yield from error_rows(self, artifact, errors)

    # -- .hermes_history -----------------------------------------------------------------
    def _parse_history(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the prompt history of the classic CLI and the TUI, with no session or project.

        The file is prompt_toolkit's `FileHistory`, read with the Aider parser's
        `read_input_history`: one `user` row per entry at its time, or a `slash command:` row for an
        entry that starts with `/`. The TUI writes UTC times, the classic CLI host-local ones.
        """
        errors: list = []
        for line_number, timestamp, text in read_input_history(artifact.disk_path, errors):
            row = self.base_row(artifact)
            row.source_line = line_number
            row.timestamp_utc = to_utc(timestamp)
            if text.startswith("/"):
                row.turn_type, row.text = "system", compact("slash command: " + text)
            else:
                row.turn_type, row.text = "user", compact(text)
            yield row
        yield from error_rows(self, artifact, errors)

    # -- cron/output/<job>/<time>.md -----------------------------------------------------
    def _parse_cron(self, artifact: Artifact, match: re.Match) -> Iterator[Row]:
        """Yield the rows of a cron run's document, all at the file name's time.

        A `cron run: job <job id>` row (cron/scheduler.py:2219-2227 and 2622-2628 at 6bb377a), then
        the prompt as `user` and the response as `assistant`, or the whole document as a `cron run
        output:` row when it has no prompt and response sections. The time is Hermes' configured
        zone or the host's, written as if UTC; there is no session id.
        """
        clock = CRON_TIME_PATTERN.match(match.group(2))
        timestamp = to_utc("%sT%s:%s:%s" % clock.groups()) if clock else ""
        errors: list = []
        try:
            document = artifact.disk_path.read_bytes().decode("utf-8", errors="replace")
        except OSError as error:
            errors.append((0, "cron run file could not be read (%s)" % error, ""))
            document = ""
        rows = [("system", "cron run: job %s (from cron/output, file name time, host-local)")]
        parts = split_cron_doc(document) if document else None
        if parts:
            rows += [("user", parts[0]), ("assistant", parts[1])]
        elif document:
            rows.append(("system", "cron run output: " + document))
        for position, (turn_type, text) in enumerate(rows):
            row = self.base_row(artifact)
            row.source_line = position
            row.timestamp_utc = timestamp
            row.turn_type = turn_type
            row.text = compact(text % match.group(1) if position == 0 else text)
            yield row
        yield from error_rows(self, artifact, errors)

    # -- response_store.db ---------------------------------------------------------------
    def _parse_responses(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of the API server's Responses store.

        Per `responses` row (gateway/platforms/api_server_response_store.py at 6bb377a), by the
        response's `created_at`: an `api server response` row, then the messages of its
        `conversation_history` that no earlier response of the session held at the same position,
        read as `state.db` messages with `data.session_id` and no time of their own.
        """
        problem = sqlite_problem(artifact.disk_path)
        if problem:
            yield self._system(artifact, 0, {}, "parser: " + problem)
            return
        record_errors: list = []
        try:
            with open_copy(artifact.disk_path) as connection:
                tables = table_names(connection)
                if "responses" not in tables:
                    yield self._system(artifact, 0, {}, "parser: no responses table")
                    return
                names: dict[str, str] = {}
                if "conversations" in tables:
                    for table_row in connection.execute(
                        "select name, response_id from conversations"
                    ):
                        names[str(table_row["response_id"])] = str(table_row["name"])
                stored = self._responses(connection, record_errors)
        except sqlite3.DatabaseError as error:
            yield self._system(artifact, 0, {}, "parser: SQLite error: %s" % error)
            return
        seen: dict[str, set] = {}
        tool_names: dict[str, str] = {}
        switches: dict[str, str] = {}
        for _, rowid, response_id, data in stored:
            session_id = str(data.get("session_id") or "")
            response = data.get("response") if isinstance(data.get("response"), dict) else {}
            header = self._system(
                artifact,
                rowid,
                {"id": session_id},
                "api server response %s status=%s model=%s conversation=%s"
                " (from response_store.db responses, created_at)"
                % (response_id, _dash(response.get("status")), _dash(response.get("model")),
                   _dash(names.get(response_id))),
            )  # fmt: skip
            header.timestamp_utc = to_utc(response.get("created_at"))
            yield header
            history = data.get("conversation_history")
            for i, message in enumerate(history if isinstance(history, list) else []):
                if not isinstance(message, dict):
                    label = "unparseable record %s (responses.data conversation_history %d" % (
                        rowid,
                        i,
                    )
                    record_errors.append((rowid, label + ": not an object)", compact_json(message)))
                    continue
                key = (i, compact_json(message))
                if key in seen.setdefault(session_id, set()):
                    continue
                seen[session_id].add(key)
                yield from self._message_rows(
                    artifact, message, {"id": session_id}, rowid, tool_names, opts, switches
                )
        yield from error_rows(self, artifact, record_errors)

    @staticmethod
    def _responses(connection, record_errors: list) -> list[tuple]:
        """Return (created_at, rowid, response_id, data) per `responses` row whose `data` decodes.

        The list is sorted by the response's `created_at`.
        """
        out = []
        for table_row in connection.execute(
            "select rowid as _rowid, response_id, data from responses"
        ):
            try:
                data = json.loads(table_row["data"] or "")
                reason = "not a JSON object"
            except (TypeError, ValueError) as error:
                data, reason = None, str(error)
            if not isinstance(data, dict):
                label = "unparseable record %s (responses.data: %s)" % (table_row["_rowid"], reason)
                record_errors.append((table_row["_rowid"], label, str(table_row["data"] or "")))
                continue
            response = data.get("response")
            created = response.get("created_at") if isinstance(response, dict) else None
            if not isinstance(created, (int, float)) or isinstance(created, bool):
                created = 0
            out.append((created, table_row["_rowid"], str(table_row["response_id"] or ""), data))
        out.sort(key=lambda entry: (entry[0], entry[1]))
        return out

    # -- trajectory_samples.jsonl, failed_trajectories.jsonl -------------------------------
    def _parse_trajectory(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the rows of a `--save-trajectories` file in the run's working directory.

        Per line (agent/trajectory.py:37-41 at 6bb377a) a `trajectory:` row at the line's
        `timestamp` (local time, written as if UTC), then one row per ShareGPT entry of
        `conversations` with no time of its own, typed by `from` (`TRAJECTORY_TYPES`, `system` as a
        `trajectory system prompt:` row). No session; the project is the collected directory.
        """
        errors: list = []
        project = artifact.home.original if artifact.home.project else ""
        name = artifact.disk_path.name
        with open(artifact.disk_path, "rb") as handle:
            for line_number, raw in enumerate(handle, 1):
                if not raw.strip():
                    continue
                try:
                    record = json.loads(raw.decode("utf-8", errors="replace"))
                    reason = "not a JSON object"
                except ValueError as error:
                    record, reason = None, str(error)
                if not isinstance(record, dict):
                    errors.append((line_number, "unparseable line (%s)" % reason, line_text(raw)))
                    continue
                conversations = record.get("conversations")
                if not isinstance(conversations, list):
                    reason = "conversations is not a list"
                    errors.append((line_number, "unparseable line (%s)" % reason, line_text(raw)))
                    continue
                head = self.base_row(artifact)
                head.source_line, head.project_path, head.turn_type = line_number, project, "system"
                head.model = str(record.get("model") or "")
                head.timestamp_utc = to_utc(record.get("timestamp"))
                head.text = "trajectory: model=%s completed=%s (from %s, timestamp host-local)" % (
                    _dash(record.get("model")),
                    _dash(record.get("completed")),
                    name,
                )
                yield head
                for entry in conversations:
                    row = self.base_row(artifact)
                    row.source_line, row.project_path, row.model = line_number, project, head.model
                    sender = entry.get("from") if isinstance(entry, dict) else None
                    value = text_of(entry.get("value")) if isinstance(entry, dict) else ""
                    if sender in TRAJECTORY_TYPES:
                        row.turn_type, row.text = TRAJECTORY_TYPES[sender], compact(value)
                    elif sender == "system":
                        row.turn_type = "system"
                        row.text = compact("trajectory system prompt: " + value)
                    else:
                        row.turn_type = "system"
                        row.text = compact("%s: %s" % (_dash(sender), value or compact_json(entry)))
                    yield row
        yield from error_rows(self, artifact, errors, "", project)

    # -- Hermes desktop connections.json -----------------------------------------------
    def _parse_connections(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the desktop app's remote backends and quarantined entries, at the file's mtime.

        One `remote backend:` row per connection that is not the app's own `local` runtime
        (apps/desktop/electron/connection-registry.ts at 6bb377a), with ` org=`, ` name=` and `
        profile=` when recorded, and one `remote backend quarantined by the app:` row per entry the
        app could not load. Tokens and headers are never printed.
        """
        errors: list = []
        document = self._read_json(artifact, "connections.json", errors)
        connections = document.get("connections") if isinstance(document, dict) else None
        if document is not None and not isinstance(connections, list):
            errors.append((0, "connections.json has no connections list", ""))
        timestamp = file_mtime(artifact)
        tail = " (from connections.json, file mtime)"
        for position, connection in enumerate(
            connections if isinstance(connections, list) else [], 1
        ):
            if not isinstance(connection, dict):
                errors.append(
                    (
                        position,
                        "unparseable record %d (not an object)" % position,
                        compact_json(connection),
                    )
                )
                continue
            if connection.get("kind") == "local":
                continue
            target = connection.get("url")
            if not target and connection.get("host"):
                target = "%s%s%s" % (
                    "%s@" % connection["user"] if connection.get("user") else "",
                    connection["host"],
                    ":%s" % connection["port"] if connection.get("port") else "",
                )
            text = "remote backend: %s %s %s" % (
                _dash(connection.get("kind")),
                _dash(connection.get("label")),
                _dash(target),
            )
            for key, label in (("org", "org"), ("name", "name"), ("remoteProfile", "profile")):
                if connection.get(key):
                    text += " %s=%s" % (label, setting_value(connection[key]))
            yield self._setting(artifact, position, text + tail, timestamp)
        quarantined = document.get("quarantined") if isinstance(document, dict) else None
        for entry in quarantined if isinstance(quarantined, list) else []:
            reason = entry.get("reason") if isinstance(entry, dict) else None
            text = "remote backend quarantined by the app: %s" % _dash(reason)
            yield self._setting(artifact, 0, text + tail, timestamp)
        yield from error_rows(self, artifact, errors)
