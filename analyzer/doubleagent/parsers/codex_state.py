"""Read Codex's SQLite projections, tracing log and desktop state for `RolloutParser`.

The files sit beside `sessions/` under the Codex home of Codex and its forks. The projections
and the global state are secondary sources: Codex writes them from the rollout, the store of
record, so `dedup.py` drops a row of theirs a rollout row repeats and keeps the rest (a thread
whose rollout is gone). `logs_<N>.sqlite` is a source of its own.

Fields the parser depends on:

- `state_5.sqlite` (and its copies under `db-backups/`): `threads` (`THREAD_COLUMNS`, each read
  when the table has it), `remote_control_enrollments` (`ENROLLMENT_COLUMNS`) and
  `guardian_review_feedback` (`id`, a UUIDv7, `thread_id`, and `record`'s
  `reviewed_thread_id`, `reviewed_turn_id`, `target_item_id`, `reviewer_thread_id`, `model`,
  `status`, `decision`, `action`);
- `thread_history_1.sqlite`: `thread_turns` (`TURN_COLUMNS`) and `thread_items`
  (`ITEM_COLUMNS`), whose `item_json` is the app server's camelCase `ThreadItem`, renamed to
  the rollout's turn-item form (`TURN_ITEM_TYPES`, `TURN_ITEM_KEYS`) for `codex.item_rows`;
- `logs_<N>.sqlite` table `logs`: `ts`, `ts_nanos`, `target`, `thread_id` and
  `feedback_log_body` (`message` in the first migration), read for the `/feedback` upload
  row, the desktop app's `ExecApproval`, `PatchApproval` and
  `CommandExecutionRequestApproval` lines and the `guardian_review{...}` spans;
- `.codex-global-state.json`: the `electron-persisted-atom-state` keys `prompt-history`,
  `thread-descriptions-v1`, `thread-user-activity-times-v1`,
  `heartbeat-thread-permissions-by-id`, the deleted-thread markers and the host settings, and
  the top-level `codex-mobile-has-connected-device`,
  `electron-local-remote-control-environment-id`, `thread-workspace-root-hints` and
  `thread-projectless-output-directories`.

A first prompt from `first_user_message` or `prompt-history` has no timestamp. Validated against
codex source at 3e23877 and 4e7b9e2 and the ChatGPT app; see `research/codex-cli.md`.
"""

from __future__ import annotations

import json
import re
import sqlite3
from collections.abc import Iterator
from pathlib import Path
from typing import TYPE_CHECKING

from ..inputs import Artifact, error_reason
from ..model import GIT_REMOTE_PREFIX, SCRATCH_PREFIX, Row, compact
from ..sqlite_util import is_sqlite, open_copy, table_names
from ..timeutil import epoch_ms, to_utc
from .base import Options, error_rows, file_mtime, setting_value
from .codex import guardian_text, guardian_verdict, item_rows

if TYPE_CHECKING:
    from .codex import RolloutParser

STATE_DATABASE = "state_5.sqlite"
THREAD_HISTORY_DATABASE = "thread_history_1.sqlite"

THREAD_COLUMNS = (
    "id",
    "rollout_path",
    "created_at",
    "created_at_ms",
    "cwd",
    "title",
    "name",
    "git_branch",
    "git_origin_url",
    "model",
    "archived",
    "source",
    "thread_source",
    "first_user_message",
)
TURN_COLUMNS = (
    "thread_id",
    "turn_id",
    "rollout_ordinal",
    "rollout_end_ordinal",
    "status",
    "started_at",
    "completed_at",
    "duration_ms",
)
ITEM_COLUMNS = (
    "thread_id",
    "turn_id",
    "item_id",
    "rollout_ordinal",
    "created_at_ms",
    "started_at_ms",
    "completed_at_ms",
    "item_json",
)
# camelCase `ThreadItem` types and keys to the rollout's turn-item form
# that `item_rows` reads.
TURN_ITEM_TYPES = {
    "commandExecution": "CommandExecution",
    "mcpToolCall": "McpToolCall",
    "dynamicToolCall": "DynamicToolCall",
    "fileChange": "FileChange",
    "functionCallOutput": "FunctionCallOutput",
    "webSearch": "WebSearch",
    "plan": "Plan",
    "subAgentActivity": "SubAgentActivity",
    "imageGeneration": "ImageGeneration",
}
TURN_ITEM_KEYS = {
    "aggregatedOutput": "aggregated_output",
    "exitCode": "exit_code",
    "contentItems": "content_items",
    "agentPath": "agent_path",
    "agentThreadId": "agent_thread_id",
}
SKIPPED_ITEMS = ("hookPrompt", "contextCompaction")
# `key=value` in a tracing body: a bare value or a quoted string.
LOG_FIELD_PATTERN = re.compile(
    r'(?:^|[ \t])([A-Za-z_][A-Za-z0-9_.]*)=("(?:[^"\\]|\\.)*"|[^ \t\r\n]+)'
)


def _select(connection: sqlite3.Connection, table: str, wanted: tuple[str, ...]) -> list[dict]:
    """Return every row of `table` as a dict of the `wanted` columns it has, plus `_rowid`.

    A column a migration has not added is absent from the dict.
    """
    have = {info[1] for info in connection.execute("pragma table_info(%s)" % table)}
    columns = [column for column in wanted if column in have]
    sql = "select rowid as _rowid, %s from %s order by rowid" % (", ".join(columns), table)
    return [dict(row) for row in connection.execute(sql)]


def _created(thread: dict):
    milliseconds = thread.get("created_at_ms")
    return milliseconds if milliseconds is not None else thread.get("created_at")


def read_threads(connection: sqlite3.Connection) -> list[dict]:
    """The `threads` rows in creation order, or [] without the table."""
    if "threads" not in table_names(connection):
        return []
    rows = _select(connection, "threads", THREAD_COLUMNS)
    rows.sort(key=lambda thread: (_created(thread) or 0, str(thread.get("id") or "")))
    return rows


def _is_subagent(thread: dict) -> bool:
    source = str(thread.get("source") or "")
    return thread.get("thread_source") == "subagent" or (
        source.startswith("{") and '"subagent"' in source
    )


def _thread_text(thread: dict) -> str:
    text = "thread record: %s title=%s rollout=%s" % (
        thread.get("id") or "",
        thread.get("title") or "",
        thread.get("rollout_path") or "",
    )
    for key, label in (("name", "name"), ("git_branch", "branch"), ("model", "model")):
        if thread.get(key):
            text += " %s=%s" % (label, thread[key])
    if thread.get("archived"):
        text += " archived"
    return text


def _stamp(parser: RolloutParser, artifact: Artifact, thread: dict | None, session_id: str) -> Row:
    """Return a row of thread `session_id` with the project and branch of its `threads` row.

    `model` stays empty: the `threads` row holds the model of the thread's last turn, not of
    each event.
    """
    row = parser.base_row(artifact)
    row.session_id = session_id
    if thread:
        row.project_path = str(thread.get("cwd") or "")
        row.git_branch = str(thread.get("git_branch") or "")
    return row


def _problem(parser: RolloutParser, artifact: Artifact, text: str) -> Row:
    row = parser.base_row(artifact)
    row.turn_type = "system"
    row.text = text
    return row


def parse_state(parser: RolloutParser, artifact: Artifact, opts: Options) -> Iterator[Row]:
    """Yield the rows of `state_5.sqlite`, or of a copy under `db-backups/`.

    A thread record and the first prompt per thread come first, then a `remote control:` row per
    enrollment, then a `permission decision:` row per guardian review feedback record.
    """
    if not is_sqlite(artifact.disk_path):
        yield _problem(parser, artifact, "parser: not a SQLite database")
        return
    try:
        with open_copy(artifact.disk_path) as connection:
            threads = read_threads(connection)
            enrollments = read_enrollments(connection)
            feedback = read_guardian_feedback(connection)
    except sqlite3.DatabaseError as error:
        yield _problem(parser, artifact, "parser: SQLite error: %s" % error)
        return
    yield from _thread_rows(parser, artifact, threads)
    for enrollment in enrollments:
        row = _problem(parser, artifact, _enrollment_text(enrollment))
        row.source_line = int(enrollment["_rowid"])
        row.timestamp_utc = to_utc(enrollment.get("updated_at"))
        yield row
    by_id = {str(thread.get("id") or ""): thread for thread in threads}
    yield from _guardian_feedback_rows(parser, artifact, feedback, by_id)


def read_guardian_feedback(connection: sqlite3.Connection) -> list[dict]:
    """Return the `guardian_review_feedback` rows in rowid order, or [] without the table."""
    if "guardian_review_feedback" not in table_names(connection):
        return []
    return _select(connection, "guardian_review_feedback", ("id", "thread_id", "record"))


def uuid7_time(value: object) -> str:
    """Return the capture time in a UUIDv7's first 48 bits as a timestamp, else ""."""
    hexes = str(value or "").replace("-", "")
    if len(hexes) != 32 or hexes[12] != "7":
        return ""
    try:
        return to_utc(int(hexes[:12], 16) / 1000)
    except ValueError:
        return ""


def guardian_feedback_text(record: dict) -> str:
    """Return the `permission decision:` text of a failed guardian review's feedback record.

    A `denied` record whose `decision` is the reviewer's JSON verdict reads exactly as the rollout
    reader's row for that verdict, so the two pair; any other is `<status> (auto_review feedback
    model=<m>) exec <item> turn=<turn> reviewer <thread>: <decision> action: <action>`.
    """
    item = str(record.get("target_item_id") or "-")
    reviewer = str(record.get("reviewer_thread_id") or "-")
    verdict = guardian_verdict(record.get("decision"))
    if record.get("status") == "denied" and verdict:
        return guardian_text(verdict, item, reviewer)
    return (
        "permission decision: %s (auto_review feedback model=%s) exec %s turn=%s reviewer %s:"
        " %s action: %s"
        % (
            record.get("status") or "-",
            record.get("model") or "-",
            item,
            record.get("reviewed_turn_id") or "-",
            reviewer,
            record.get("decision") or "-",
            record.get("action") or "",
        )
    )


def _guardian_feedback_rows(
    parser: RolloutParser, artifact: Artifact, feedback: list[dict], threads: dict[str, dict]
) -> Iterator[Row]:
    """Yield one `permission decision:` row per `guardian_review_feedback` record.

    The row is at the UUIDv7 id's time, for the reviewed thread, with the governed item as
    `tool_use_id`; a `record` that is not a JSON object is a bad-record row after the others.
    """
    errors: list = []
    for feedback_row in feedback:
        rowid = int(feedback_row["_rowid"])
        blob = feedback_row.get("record")
        raw = blob.decode("utf-8", errors="replace") if isinstance(blob, bytes) else str(blob)
        try:
            record = json.loads(raw)
        except ValueError as error:
            errors.append((rowid, "unparseable record %d (%s)" % (rowid, error), raw))
            continue
        if not isinstance(record, dict):
            errors.append((rowid, "unparseable record %d (not a JSON object)" % rowid, raw))
            continue
        session_id = str(record.get("reviewed_thread_id") or feedback_row.get("thread_id") or "")
        row = _stamp(parser, artifact, threads.get(session_id), session_id)
        row.source_line = rowid
        row.turn_type = "system"
        row.timestamp_utc = uuid7_time(feedback_row.get("id"))
        row.tool_use_id = str(record.get("target_item_id") or "")
        row.text = compact(guardian_feedback_text(record))
        yield row
    yield from error_rows(parser, artifact, errors)


ENROLLMENT_COLUMNS = (
    "remote_control_enabled",
    "server_id",
    "environment_id",
    "server_name",
    "app_server_client_name",
    "account_id",
    "updated_at",
)


def read_enrollments(connection: sqlite3.Connection) -> list[dict]:
    """Return the `remote_control_enrollments` rows in rowid order, or [] without the table."""
    if "remote_control_enrollments" not in table_names(connection):
        return []
    return _select(connection, "remote_control_enrollments", ENROLLMENT_COLUMNS)


def _enrollment_text(record: dict) -> str:
    def field(key: str) -> str:
        value = record.get(key)
        return "-" if value in (None, "") else str(value)

    return (
        "remote control: enabled=%s server=%s environment=%s host=%s client=%s account=%s"
        % tuple(field(column) for column in ENROLLMENT_COLUMNS[:6])
    )


def _thread_rows(parser: RolloutParser, artifact: Artifact, threads: list[dict]) -> Iterator[Row]:
    """A thread record and the first prompt per `threads` row."""
    for thread in threads:
        session_id = str(thread.get("id") or "")
        row = _stamp(parser, artifact, thread, session_id)
        row.source_line = int(thread["_rowid"])
        row.turn_type = "system"
        row.timestamp_utc = to_utc(_created(thread))
        row.text = compact(_thread_text(thread))
        yield row
        origin = compact(thread.get("git_origin_url"))
        if origin:
            remote = _stamp(parser, artifact, thread, session_id)
            remote.source_line = row.source_line
            remote.turn_type = "system"
            remote.timestamp_utc = row.timestamp_utc
            remote.text = GIT_REMOTE_PREFIX + origin
            yield remote
        first = compact(thread.get("first_user_message"))
        if not first:
            continue
        prompt = _stamp(parser, artifact, thread, session_id)
        prompt.source_line = row.source_line
        if _is_subagent(thread):
            prompt.turn_type = "system"
            prompt.text = "subagent task: " + first
        else:
            prompt.turn_type = "user"
            prompt.text = first
        yield prompt


def _sibling_threads(artifact: Artifact) -> dict[str, dict]:
    """Return the `threads` rows of the `state_5.sqlite` beside `artifact`, by id.

    The result is {} when there is no such database or it cannot be read.
    """
    path = artifact.disk_path.parent / STATE_DATABASE
    if not is_sqlite(path):
        return {}
    try:
        with open_copy(path) as connection:
            return {str(thread.get("id") or ""): thread for thread in read_threads(connection)}
    except sqlite3.DatabaseError:
        return {}


def to_turn_item(item: dict) -> dict:
    """A camelCase `ThreadItem` in the rollout's turn-item form."""
    item_type = str(item.get("type") or "")
    if item_type == "sleep":
        return {"type": "Extension", "kind": "clock.sleep", "durationMs": item.get("durationMs")}
    out = {TURN_ITEM_KEYS.get(key, key): value for key, value in item.items()}
    out["type"] = TURN_ITEM_TYPES.get(item_type, item_type)
    changes = item.get("changes")
    if item_type == "fileChange" and isinstance(changes, list):
        out["changes"] = {
            str(change.get("path")): change
            for change in changes
            if isinstance(change, dict) and change.get("path")
        }
    return out


def _joined(values, key: str = "") -> str:
    """Strings of a list, or the `key` of its dict members, joined by newlines."""
    if not isinstance(values, list):
        return ""
    parts = []
    for value in values:
        if key and isinstance(value, dict) and value.get("type") == "text":
            value = value.get(key)
        if isinstance(value, str) and value:
            parts.append(value)
    return "\n".join(parts)


def thread_item_rows(item: dict, item_row: dict) -> list[tuple[str, object, dict, str]]:
    """Return the rows of one `thread_items` row whose decoded `item_json` is `item`.

    Each row is (turn_type, timestamp, {tool_name, tool_use_id}, text).
    """
    item_type = str(item.get("type") or "")
    created = item_row.get("created_at_ms")
    started = (
        item_row.get("started_at_ms") if item_row.get("started_at_ms") is not None else created
    )
    completed = (
        item_row.get("completed_at_ms") if item_row.get("completed_at_ms") is not None else created
    )
    if item_type == "userMessage":
        text = _joined(item.get("content"), "text")
        return [("user", completed, {}, text)] if text else []
    if item_type == "agentMessage":
        text = str(item.get("text") or "")
        return [("assistant", completed, {}, text)] if text else []
    if item_type == "reasoning":
        text = _joined(item.get("summary")) or _joined(item.get("content"))
        return [("thinking", completed, {}, text)] if text else []
    if item_type in SKIPPED_ITEMS:
        return []
    return item_rows(to_turn_item(item), {"started_at_ms": started, "completed_at_ms": completed})


def _events(turns: list[dict], items: list[dict]) -> list[tuple]:
    """Return the turn starts, items and turn ends in rollout order, thread by thread.

    Each entry is a sort key ending in (kind, record).
    """
    last_item: dict = {}
    for item in items:
        key = (item.get("thread_id"), item.get("turn_id"))
        last_item[key] = max(last_item.get(key, -1), item.get("rollout_ordinal") or 0)
    events = []
    for turn in turns:
        start = turn.get("rollout_ordinal") or 0
        end = turn.get("rollout_end_ordinal")
        if end is None:
            end = max(start, last_item.get((turn.get("thread_id"), turn.get("turn_id")), start))
        events.append((str(turn.get("thread_id") or ""), start, 0, turn["_rowid"], "start", turn))
        events.append((str(turn.get("thread_id") or ""), end, 2, turn["_rowid"], "end", turn))
    for item in items:
        ordinal = item.get("rollout_ordinal") or 0
        events.append((str(item.get("thread_id") or ""), ordinal, 1, item["_rowid"], "item", item))
    events.sort(key=lambda event: event[:4])
    return events


def _turn_row(kind: str, turn: dict) -> tuple[object, str] | None:
    """(timestamp, text) of a turn's start or end row, or None."""
    turn_id = turn.get("turn_id") or ""
    if kind == "start":
        return turn.get("started_at"), "task_started turn=%s" % turn_id
    if turn.get("status") in ("completed", "failed"):
        text = "task_complete turn=%s duration_ms=%s" % (turn_id, turn.get("duration_ms") or "")
        return turn.get("completed_at"), text
    return None


def _decode_item(
    rowid: int, value, thread: dict | None, session_id: str, item_errors: list
) -> dict | None:
    """Return the `ThreadItem` object in a `thread_items` row's `item_json`, or None.

    A value that does not decode to a JSON object goes to `item_errors` as (thread id, its
    `threads` row, entry), the entry holding the rowid, `unparseable record <rowid> (<reason>)`
    and the stored value as text, so the row is reported in full under its own thread.
    """
    if value is None:
        item_errors.append(
            (session_id, thread, (rowid, "unparseable record %d (no item_json)" % rowid, ""))
        )
        return None
    raw = value.decode("utf-8", "replace") if isinstance(value, bytes) else str(value)
    try:
        item = json.loads(raw)
    except ValueError as error:
        reason = str(error)
    else:
        if isinstance(item, dict):
            return item
        reason = "not a JSON object"
    item_errors.append(
        (session_id, thread, (rowid, "unparseable record %d (%s)" % (rowid, reason), raw))
    )
    return None


def parse_thread_history(parser: RolloutParser, artifact: Artifact, opts: Options) -> Iterator[Row]:
    """Rows of `thread_history_1.sqlite`: turns and their items."""
    if not is_sqlite(artifact.disk_path):
        yield _problem(parser, artifact, "parser: not a SQLite database")
        return
    try:
        with open_copy(artifact.disk_path) as connection:
            tables = table_names(connection)
            turns = (
                _select(connection, "thread_turns", TURN_COLUMNS)
                if "thread_turns" in tables
                else []
            )
            items = (
                _select(connection, "thread_items", ITEM_COLUMNS)
                if "thread_items" in tables
                else []
            )
    except sqlite3.DatabaseError as error:
        yield _problem(parser, artifact, "parser: SQLite error: %s" % error)
        return
    threads = _sibling_threads(artifact)
    item_errors: list[tuple[str, dict | None, tuple[int, str, str]]] = []
    tasked: set = set()  # subagent threads whose first prompt was seen
    for session_id, _ordinal, _order, rowid, kind, record in _events(turns, items):
        thread = threads.get(session_id)
        if kind != "item":
            turn = _turn_row(kind, record)
            if turn is None:
                continue
            row = _stamp(parser, artifact, thread, session_id)
            row.source_line = int(rowid)
            row.turn_type = "system"
            row.timestamp_utc = to_utc(turn[0])
            row.text = compact(turn[1])
            yield row
            continue
        item = _decode_item(int(rowid), record.get("item_json"), thread, session_id, item_errors)
        if item is None:
            continue
        for turn_type, timestamp, fields, text in thread_item_rows(item, record):
            if turn_type == "thinking" and not opts.include_thinking:
                continue
            row = _stamp(parser, artifact, thread, session_id)
            row.source_line = int(rowid)
            row.turn_type = turn_type
            row.timestamp_utc = to_utc(timestamp)
            row.tool_name = fields.get("tool_name", "")
            row.tool_use_id = fields.get("tool_use_id", "")
            if turn_type == "user" and thread and _is_subagent(thread) and session_id not in tasked:
                row.turn_type = "system"
                text = "subagent task: " + text
            if turn_type == "user":
                tasked.add(session_id)
            row.text = compact(text)
            yield row
    for session_id, thread, entry in item_errors:
        project = str(thread.get("cwd") or "") if thread else ""
        yield from error_rows(parser, artifact, [entry], session_id, project)


LOGS_TARGET = "codex_feedback"
LOGS_MESSAGE = "feedback event uploaded to Sentry"
NO_THREAD_PREFIX = "no-active-thread-"


def _log_fields(body: str) -> dict[str, str]:
    """Return the `key=value` fields of a `feedback_log_body`, a quoted value unquoted."""
    out: dict[str, str] = {}
    for match in LOG_FIELD_PATTERN.finditer(body):
        value = match.group(2)
        if len(value) >= 2 and value[0] == value[-1] == '"':
            value = value[1:-1]
        out[match.group(1)] = value
    return out


# The user's per-call approval decisions of the ChatGPT desktop app, as
# Codex Desktop 0.163.0-alpha.10.2 logs them (research/codex-cli.md,
# Other stores): the core's submission of the decision, with the thread and
# the governed item's id, and the app server's response to the approval
# request, with only a request id.
EXEC_APPROVAL_TARGET = "codex_core::session::handlers"
EXEC_APPROVAL_MARK = "op=ExecApproval {"
EXEC_APPROVAL_PATTERN = re.compile(
    r'op=ExecApproval \{ id: "([^"]*)"(?:, turn_id: (?:Some\("([^"]*)"\)|None))?'
    r".*?decision: ([A-Za-z_]+)"
)
# An `apply_patch` approval, logged by the same `Submission` line with the
# governed `FileChange` item's id and no turn (research/codex-cli.md,
# Standing grants and approval records).
PATCH_APPROVAL_MARK = "op=PatchApproval {"
PATCH_APPROVAL_PATTERN = re.compile(r'op=PatchApproval \{ id: "([^"]*)".*?decision: ([A-Za-z_]+)')
REQUEST_APPROVAL_TARGET = "codex_app_server::outgoing_message"
REQUEST_APPROVAL_MARK = "CommandExecutionRequestApproval {"
REQUEST_APPROVAL_PATTERN = re.compile(
    r'CommandExecutionRequestApproval \{ request_id: (?:[A-Za-z]+\()?"?([^,)"]*)"?\)?,'
    r".*?decision: ([A-Za-z_]+)"
)
GUARDIAN_SPAN_MARK = "guardian_review{"
GUARDIAN_SPAN_PATTERN = re.compile(r"guardian_review\{([^}]*)\}")
LOG_KINDS = (
    (LOGS_TARGET, LOGS_MESSAGE, "feedback"),
    (EXEC_APPROVAL_TARGET, EXEC_APPROVAL_MARK, "exec"),
    (EXEC_APPROVAL_TARGET, PATCH_APPROVAL_MARK, "patch"),
    (REQUEST_APPROVAL_TARGET, REQUEST_APPROVAL_MARK, "request"),
)


def _logs_shape(connection: sqlite3.Connection) -> tuple[str, str] | None:
    """Return the body column and the select list of `ts`, `ts_nanos` and `thread_id` of `logs`.

    The result is None without the table or its body and target columns.
    """
    if "logs" not in table_names(connection):
        return None
    have = {info[1] for info in connection.execute("pragma table_info(logs)")}
    body = "feedback_log_body" if "feedback_log_body" in have else "message"
    if body not in have or "target" not in have:
        return None
    columns = ", ".join(
        "%s as %s" % (column if column in have else "null", column)
        for column in ("ts", "ts_nanos", "thread_id")
    )
    return body, columns


def _log_rows(connection: sqlite3.Connection) -> list[dict]:
    """Return the `logs` rows the parser reads, in id order, or [] without the table.

    Each row has a `kind`, `feedback`, `exec`, `patch` or `request`: the first `LOG_KINDS` entry
    whose target and marker the row has.
    """
    shape = _logs_shape(connection)
    if shape is None:
        return []
    body, columns = shape
    where = " or ".join("(target = ? and instr(%s, ?) > 0)" % body for _entry in LOG_KINDS)
    sql = "select rowid as _rowid, %s, target, %s as body from logs where %s order by rowid" % (
        columns,
        body,
        where,
    )
    arguments = [value for target, mark, _kind in LOG_KINDS for value in (target, mark)]
    rows = [dict(row) for row in connection.execute(sql, arguments)]
    for row in rows:
        body = str(row.get("body") or "")
        row["kind"] = next(
            (kind for target, mark, kind in LOG_KINDS if row["target"] == target and mark in body),
            "",
        )
    return rows


def guardian_calls(home: Path, parent: str, start: str, end: str) -> list[str]:
    """Return the distinct `tool_call_id`s of thread `parent`'s guardian review spans, in log order.

    The spans are read from the `logs_<N>.sqlite` files in the Codex home `home`, logged from 2
    seconds before `start` to 2 seconds after `end` (both timeline timestamps). The result is []
    when there are none or no log can be read.
    """
    start_ms, end_ms = epoch_ms(start), epoch_ms(end)
    if start_ms is None or end_ms is None:
        return []
    calls: list[str] = []
    for path in sorted(home.glob("logs_*.sqlite")):
        if not is_sqlite(path):
            continue
        try:
            with open_copy(path) as connection:
                shape = _logs_shape(connection)
                if shape is None:
                    continue
                sql = "select %s from logs where instr(%s, ?) > 0 and ts between ? and ?" % (
                    shape[0],
                    shape[0],
                )
                arguments = (GUARDIAN_SPAN_MARK, start_ms // 1000 - 2, end_ms // 1000 + 2)
                bodies = [
                    str(row[0] or "")
                    for row in connection.execute(sql + " order by rowid", arguments)
                ]
        except sqlite3.DatabaseError:
            continue
        for body in bodies:
            for match in GUARDIAN_SPAN_PATTERN.finditer(body):
                fields = _log_fields(match.group(1))
                call = fields.get("tool_call_id", "")
                if fields.get("thread_id") == parent and call and call not in calls:
                    calls.append(call)
    return calls


def _log_time(record: dict) -> str:
    timestamp = record.get("ts")
    if not isinstance(timestamp, (int, float)) or isinstance(timestamp, bool):
        return ""
    nanoseconds = record.get("ts_nanos")
    if (
        isinstance(nanoseconds, int)
        and not isinstance(nanoseconds, bool)
        and 0 <= nanoseconds < 1_000_000_000
    ):
        return to_utc(timestamp + nanoseconds / 1e9)
    return to_utc(timestamp)


def _near(first, second) -> bool:
    """Whether two `ts` values are numbers at most 2 seconds apart."""
    numbers = all(
        isinstance(value, (int, float)) and not isinstance(value, bool) for value in (first, second)
    )
    return numbers and abs(first - second) <= 2


def _approval_match(record: dict, kind: str, errors: list) -> re.Match | None:
    """Return the approval pattern's match in a `logs` row of `kind`, or None.

    A row of that kind the pattern cannot read goes to `errors` as (rowid, message, body).
    """
    if record["kind"] != kind:
        return None
    body = str(record.get("body") or "")
    pattern = {"exec": EXEC_APPROVAL_PATTERN, "patch": PATCH_APPROVAL_PATTERN}.get(
        kind, REQUEST_APPROVAL_PATTERN
    )
    match = pattern.search(body)
    if not match:
        errors.append(
            (int(record["_rowid"]), "unreadable approval log row (%s)" % record["target"], body)
        )
    return match


def _decisions(records: list[dict], errors: list) -> dict[int, tuple[str, str, str]]:
    """Return (session id, tool_use_id, text less its source suffix) of each approval row, by rowid.

    A row the patterns cannot read goes to `errors`. A request row with an exec row within 2
    seconds is left out, since the exec row names the thread and the governed item; the two use
    different words for one decision (`denied` and `approved`, `Decline` and `Accept`). A patch row
    stands alone: the request rows read are command approvals only.
    """
    out: dict[int, tuple[str, str, str]] = {}
    exec_approvals: list[list] = []  # [ts, decision, paired]
    for record in records:
        match = _approval_match(record, "exec", errors)
        if not match:
            continue
        body = str(record.get("body") or "")
        session_id = _log_fields(body).get("thread_id") or str(record.get("thread_id") or "")
        item, turn, decision = match.group(1), match.group(2) or "-", match.group(3)
        text = "permission decision: %s exec %s turn=%s" % (decision, item, turn)
        out[int(record["_rowid"])] = (session_id, item, text)
        exec_approvals.append([record.get("ts"), decision, False])
    for record in records:
        match = _approval_match(record, "patch", errors)
        if not match:
            continue
        body = str(record.get("body") or "")
        session_id = _log_fields(body).get("thread_id") or str(record.get("thread_id") or "")
        text = "permission decision: %s patch %s" % (match.group(2), match.group(1))
        out[int(record["_rowid"])] = (session_id, match.group(1), text)
    for record in records:
        match = _approval_match(record, "request", errors)
        if not match:
            continue
        twin = next(
            (
                candidate
                for candidate in exec_approvals
                if not candidate[2] and _near(candidate[0], record.get("ts"))
            ),
            None,
        )
        if twin is not None:
            twin[2] = True
            continue
        text = "permission decision: %s request %s" % (match.group(2), match.group(1))
        out[int(record["_rowid"])] = ("", "", text)
    return out


def _feedback_text(record: dict, name: str) -> tuple[str, str]:
    """(session id, text) of a sent `/feedback` report's row."""
    fields = _log_fields(str(record.get("body") or ""))
    session_id = fields.get("thread_id") or str(record.get("thread_id") or "")
    if session_id.startswith(NO_THREAD_PREFIX):
        session_id = ""
    text = (
        "session synced: feedback report uploaded to Sentry, attachments %s failed %s"
        " (from %s logs, ts)"
        % (fields.get("uploaded_attachments") or "-", fields.get("attachments_failed") or "-", name)
    )
    return session_id, text


def parse_logs(parser: RolloutParser, artifact: Artifact) -> Iterator[Row]:
    """Yield the rows of `logs_<N>.sqlite`: sent `/feedback` reports and approval decisions."""
    if not is_sqlite(artifact.disk_path):
        yield _problem(parser, artifact, "parser: not a SQLite database")
        return
    try:
        with open_copy(artifact.disk_path) as connection:
            records = _log_rows(connection)
    except sqlite3.DatabaseError as error:
        yield _problem(parser, artifact, "parser: SQLite error: %s" % error)
        return
    threads = _sibling_threads(artifact) if records else {}
    errors: list = []
    decisions = _decisions(records, errors)
    name = artifact.disk_path.name
    for record in records:
        rowid = int(record["_rowid"])
        tool_use_id = ""
        if record["kind"] == "feedback":
            session_id, text = _feedback_text(record, name)
        elif rowid in decisions:
            session_id, tool_use_id, text = decisions[rowid]
            text += " (from %s logs, ts)" % name
        else:
            continue
        row = _stamp(parser, artifact, threads.get(session_id) if session_id else None, session_id)
        row.source_line = rowid
        row.turn_type = "system"
        row.timestamp_utc = _log_time(record)
        row.tool_use_id = tool_use_id
        row.text = text
        yield row
    yield from error_rows(parser, artifact, errors)


# `.codex-global-state.json`, the ChatGPT desktop app's own state, as Codex
# Desktop 0.163.0-alpha.10.2 writes it (research/codex-cli.md, Other stores).
# The key holding the app's persisted UI atoms, and the keys read inside it
# and at the top level.
ATOM_KEY = "electron-persisted-atom-state"
PROMPT_HISTORY = "prompt-history"
DESCRIPTIONS = "thread-descriptions-v1"
ACTIVITY = "thread-user-activity-times-v1"
PERMISSIONS = "heartbeat-thread-permissions-by-id"
DELETED_PREFIX = "codex-writing-block-deleted-thread-v1:"
HOST_SETTINGS = (
    "agent-mode-by-host-id",
    "preferred-non-full-access-agent-mode-by-host-id",
)
SELECTION_PREFIX = "permission-selection-by-host-id:"
ROOT_HINTS = "thread-workspace-root-hints"
OUTPUT_DIRS = "thread-projectless-output-directories"
MOBILE_DEVICE = "codex-mobile-has-connected-device"
REMOTE_ENVIRONMENT = "electron-local-remote-control-environment-id"


def _as_object(value) -> dict:
    """`value` when it is a JSON object, or a string holding one; else {}."""
    if isinstance(value, str):
        try:
            value = json.loads(value)
        except ValueError:
            return {}
    return value if isinstance(value, dict) else {}


def _activity_by_thread(atom: dict) -> dict[str, object]:
    """Last activity time per thread id, from keys `["local","<id>"]`."""
    out: dict[str, object] = {}
    for key, value in _as_object(atom.get(ACTIVITY)).items():
        try:
            parts = json.loads(key)
        except ValueError:
            continue
        if isinstance(parts, list) and len(parts) == 2 and isinstance(parts[1], str):
            out[parts[1]] = value
    return out


def _prompt_entries(atom: dict) -> Iterator[tuple[str, int, str]]:
    """Yield (thread id, 1-based index, markdown) of each `prompt-history` entry.

    An entry of the `global` list, the composer's history across threads, has no thread id and is
    left out when a thread's list in the same file has its text, since two rows of one file never
    pair in deduplication.
    """
    lists = {
        str(key): [
            (i, entry["markdown"])
            for i, entry in enumerate(entries, 1)
            if isinstance(entry, dict)
            and isinstance(entry.get("markdown"), str)
            and entry["markdown"].strip()
        ]
        for key, entries in _as_object(atom.get(PROMPT_HISTORY)).items()
        if isinstance(entries, list)
    }
    threaded = {text for key, pairs in lists.items() if key != "global" for _index, text in pairs}
    for key, pairs in lists.items():
        for i, text in pairs:
            if key != "global":
                yield key, i, text
            elif text not in threaded:
                yield "", i, text


def _standing_text(entry: dict, suffix: str) -> str:
    profile = entry.get("activePermissionProfile")
    if isinstance(profile, dict):
        profile = profile.get("id")
    pairs = (
        ("approval", entry.get("approvalPolicy")),
        ("sandbox", entry.get("sandboxPolicy")),
        ("reviewer", entry.get("approvalsReviewer")),
        ("profile_id", profile),
    )
    return "permission mode (standing): %s %s" % (
        " ".join(
            "%s=%s" % (label, "-" if value in (None, "") else setting_value(value))
            for label, value in pairs
        ),
        suffix,
    )


def _global_state_texts(document: dict, rel: str) -> Iterator[tuple[str, str]]:
    """(session id, text) of each file-mtime `system` row of the global state."""
    atom = _as_object(document.get(ATOM_KEY))

    def source(key: str) -> str:
        return "(from %s %s, file mtime)" % (rel, key)

    for key, value in atom.items():
        if key.startswith(DELETED_PREFIX) and value is True:
            session_id = key[len(DELETED_PREFIX) :]
            yield (
                session_id,
                "session deleted: %s %s" % (session_id, source(DELETED_PREFIX.rstrip(":"))),
            )
    descriptions = _as_object(atom.get(DESCRIPTIONS))
    activity = _activity_by_thread(atom)
    for session_id in list(descriptions) + [
        thread_id for thread_id in activity if thread_id not in descriptions
    ]:
        text = descriptions.get(session_id)
        yield (
            session_id,
            "thread description: %s last_activity=%s %s"
            % (
                text if isinstance(text, str) and text else "-",
                to_utc(activity.get(session_id)) or "-",
                source(DESCRIPTIONS),
            ),
        )
    for session_id, entry in _as_object(atom.get(PERMISSIONS)).items():
        if isinstance(entry, dict):
            yield session_id, _standing_text(entry, source(PERMISSIONS))
    for key in HOST_SETTINGS:
        for host, value in _as_object(atom.get(key)).items():
            setting = "%s.%s=%s" % (key, host, setting_value(value))
            yield "", "permission setting: %s (from %s, file mtime)" % (setting, rel)
    for key, value in atom.items():
        if key.startswith(SELECTION_PREFIX):
            setting = "%s=%s" % (key, setting_value(value))
            yield "", "permission setting: %s (from %s, file mtime)" % (setting, rel)
    if MOBILE_DEVICE in document or REMOTE_ENVIRONMENT in document:
        yield (
            "",
            "remote control: mobile device connected=%s environment=%s %s"
            % (
                setting_value(document[MOBILE_DEVICE]) if MOBILE_DEVICE in document else "-",
                document.get(REMOTE_ENVIRONMENT) or "-",
                source(MOBILE_DEVICE),
            ),
        )
    for session_id, path in _as_object(document.get(OUTPUT_DIRS)).items():
        if isinstance(path, str) and path:
            yield (
                session_id,
                "%s %s (projectless thread %s) %s"
                % (SCRATCH_PREFIX, path, session_id, source(OUTPUT_DIRS)),
            )


def parse_global_state(parser: RolloutParser, artifact: Artifact) -> Iterator[Row]:
    """Yield the rows of `.codex-global-state.json`.

    The app's prompt history comes first, as `user` rows with no timestamp, then the `system` rows
    at the file's modification time.
    """
    try:
        with open(artifact.disk_path, "rb") as handle:
            document = json.loads(handle.read().decode("utf-8", errors="replace"))
    except (OSError, ValueError) as error:
        yield _problem(
            parser, artifact, "parser: unreadable global state: %s" % error_reason(error)
        )
        return
    if not isinstance(document, dict):
        yield _problem(parser, artifact, "parser: global state is not a JSON object")
        return
    roots = _as_object(document.get(ROOT_HINTS))
    outputs = _as_object(document.get(OUTPUT_DIRS))

    def stamp(session_id: str) -> Row:
        row = _stamp(parser, artifact, None, session_id)
        row.project_path = (
            str(roots.get(session_id) or outputs.get(session_id) or "") if session_id else ""
        )
        return row

    for session_id, index, text in _prompt_entries(_as_object(document.get(ATOM_KEY))):
        row = stamp(session_id)
        row.turn_type = "user"
        row.source_line = index
        row.text = compact(text)
        yield row
    mtime = file_mtime(artifact)
    for session_id, text in _global_state_texts(document, artifact.rel):
        row = stamp(session_id)
        row.turn_type = "system"
        row.source_line = 1
        row.timestamp_utc = mtime
        row.text = compact(text)
        if text.startswith(SCRATCH_PREFIX):
            # The row names the projectless directory itself, whatever
            # root hint the thread has.
            row.project_path = str(outputs[session_id])
        yield row
