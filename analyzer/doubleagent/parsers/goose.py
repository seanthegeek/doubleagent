"""Goose CLI and desktop sessions.

Reads, relative to the home (`<data>` is `.local/share/goose` or
`AppData/Roaming/Block/goose/data`, `<state>` is `.local/state/goose` or the Windows data
directory, `<config>` is `.config/goose` or `AppData/Roaming/Block/goose/config`):
`<data>/sessions/sessions.db`, the legacy `<data>/sessions/<YYYYMMDD_HHMMSS>.jsonl`, the
request logs `<state>/logs/llm_request.<N>.jsonl` and the prompt histories
`<state>/history.txt` and `.config/goose/history.txt` (both secondary), and the standing files
`<config>/permission.yaml`, `<config>/roaming_trust.json` and `<data>/schedule.json`. Yields
session start rows, one row per content block, model, permission, relay and scheduled-task
`system` rows, and `parser:` rows for stored values that do not decode.

Fields the parser depends on:

* `sessions`: `id`, `name`, `description`, `session_type`, `working_dir`, `created_at`,
  `provider_name`, `model_config_json.model_name`, `parent_session_id`, `schedule_id`,
  `archived_at`, `goose_mode`; every column is read with a default.
* `messages`: `id`, `session_id`, `role`, `content_json`, `created_timestamp`,
  `metadata_json` (`userVisible`, `inference.provider`, `.requestedModel`, `.resolvedModel`).
* Content blocks by `type`: `text`, `image`, `document`, `toolRequest` (`_meta`
  `goose.executable`), `toolResponse`, `thinking`, `redactedThinking`,
  `toolConfirmationRequest`, `actionRequired` (`data.actionType`), `systemNotification`,
  `error`.
* Legacy and request-log lines: `id`, `role`, `created`, `content`, `metadata`,
  `model_config.model_name`, `input.messages`, `data`, `error`.

Validated against source, aaif-goose/goose at b23d21c (1.57.0), with a synthetic fixture; no real
install. The older shapes it still reads (the legacy JSONL sessions, a bare content array in
`toolResult.value`, the `toolConfirmationRequest` block) are under Format versions and Record
schema of `research/goose.md`, which this module follows.
"""

from __future__ import annotations

import json
import re
import sqlite3
from collections.abc import Iterator

from ..inputs import Artifact
from ..model import Row, compact
from ..sqlite_util import is_sqlite, open_copy, table_names
from ..textlines import SPACE_OR_TAB, without_terminator
from ..timeutil import to_utc
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

DATA = r"(?:\.local/share/goose|AppData/Roaming/Block/goose/data)"
STATE = r"(?:\.local/state/goose|AppData/Roaming/Block/goose/data)"
CONFIG = r"(?:\.config/goose|AppData/Roaming/Block/goose/config)"
DATABASE_PATTERN = re.compile(r"^%s/sessions/sessions\.db$" % DATA)
LEGACY_PATTERN = re.compile(r"^%s/sessions/[^/]+\.jsonl$" % DATA)
LLM_PATTERN = re.compile(r"^%s/logs/llm_request\.[^/]+\.jsonl$" % STATE)
HISTORY_PATTERN = re.compile(r"^(?:%s|\.config/goose)/history\.txt$" % STATE)
PERMISSION_PATTERN = re.compile(r"^%s/permission\.yaml$" % CONFIG)
TRUST_PATTERN = re.compile(r"^%s/roaming_trust\.json$" % CONFIG)
SCHEDULE_PATTERN = re.compile(r"^%s/schedule\.json$" % DATA)
# The request log stamps each streamed chunk when it arrives, while
# `sessions.db` stamps the assembled message with the later of its first
# chunk's time and the session's newest stored message.
LLM_TOLERANCE_SECONDS = 600.0
TOOL_EXECUTABLE_KEY = "goose.executable"
# A gateway session's working directory, `<config>/gateway/<platform>/<user_id>`.
GATEWAY_DIRECTORY_PATTERN = re.compile(r"[/\\]gateway[/\\]([^/\\]+)[/\\]([^/\\]+)[/\\]?$")


def epoch(value) -> str:
    """Return a `created_timestamp` as UTC.

    The value is in seconds, but some clients write milliseconds: Goose divides anything above 1e10
    by 1000 (session_manager.rs:851-864 at aaif-goose/goose b23d21c).
    """
    if isinstance(value, (int, float)) and not isinstance(value, bool) and value > 10_000_000_000:
        value = value / 1000.0
    return to_utc(value)


def _stored_json(value, kind: type) -> tuple[object, str, str]:
    """Decode a JSON column of `sessions.db`.

    `kind` is the type Goose writes there (`list` for `content_json`, `dict` otherwise); a BLOB is
    read as UTF-8 with replacement. Returns (decoded value, reason, stored text): (None, "", "") for
    a NULL or empty column, and (None, reason, the stored text in full) for a value that does not
    decode or decodes to another type than `kind` (a JSON `null` excepted).
    """
    if value is None or value == "" or value == b"":
        return None, "", ""
    if isinstance(value, bytes):
        value = value.decode("utf-8", errors="replace")
    if not isinstance(value, str):
        return None, "not JSON text", str(value)
    try:
        decoded = json.loads(value)
    except ValueError as error:
        return None, str(error), value
    if decoded is not None and not isinstance(decoded, kind):
        return None, "not a JSON %s" % ("array" if kind is list else "object"), value
    return decoded, "", value


def _join(*parts: str) -> str:
    """Join the non-empty parts with ` | `."""
    return " | ".join(part for part in parts if part)


def gateway_text(session: dict) -> str:
    """Return the `session relayed:` text of a `gateway` session, `""` for any other.

    The platform and user id come from `working_dir` (`<config>/gateway/<platform>/<user_id>`), else
    the platform from `name`; the display name comes from `name` (`<platform>/<display name or user
    id>`), `-` when it is the user id or absent.
    """
    if session.get("session_type") != "gateway":
        return ""
    match = GATEWAY_DIRECTORY_PATTERN.search(str(session.get("working_dir") or ""))
    platform, user_id = match.groups() if match else ("", "")
    name_platform, _, display = str(session.get("name") or "").partition("/")
    platform = platform or name_platform
    if display == user_id:
        display = ""
    return (
        "session relayed: gateway %s user %s display %s"
        " (from sessions.db session_type gateway, created_at)"
        % (platform or "-", user_id or "-", display or "-")
    )


def _model(provider: str, name: str) -> str:
    """Return `<provider>/<name>`, or the name alone when the provider is unknown."""
    return "%s/%s" % (provider, name) if provider and name else name or ""


def result_text(tool_result) -> tuple[str, bool]:
    """Return (text, is_error) of a `toolResponse.toolResult`."""
    if not isinstance(tool_result, dict):
        return compact_json(tool_result), False
    if tool_result.get("status") == "error":
        return str(tool_result.get("error") or ""), True
    value = tool_result.get("value")
    if isinstance(value, list):  # older rows: bare content array
        return text_of(value), False
    if isinstance(value, dict):
        text = text_of(value.get("content"))
        if not text and value.get("structuredContent") is not None:
            text = compact_json(value["structuredContent"])
        return text, bool(value.get("isError"))
    return "", False


def unescape_history(line: str) -> str:
    r"""Undo the backslash escapes of a rustyline `#V2` history line (`\n` is a line feed)."""
    out: list[str] = []
    i = 0
    while i < len(line):
        char = line[i]
        if char == "\\" and i + 1 < len(line):
            next_char = line[i + 1]
            out.append("\n" if next_char == "n" else next_char)
            i += 2
            continue
        out.append(char)
        i += 1
    return "".join(out)


def _unquote(value: str) -> str:
    """Return a YAML scalar as `serde_yaml` writes it: plain, single-quoted or double-quoted."""
    if len(value) >= 2 and value[0] == value[-1] == "'":
        return value[1:-1].replace("''", "'")
    if len(value) >= 2 and value[0] == value[-1] == '"':
        try:
            decoded = json.loads(value)
        except ValueError:
            return value[1:-1]
        return decoded if isinstance(decoded, str) else value[1:-1]
    return value


def permission_entries(path, errors: list) -> Iterator[tuple[int, str, str, str]]:
    """Yield (line, category, level, tool) for each tool in `permission.yaml`.

    `serde_yaml` writes a map from category (`user`, `smart_approve`) to `always_allow`,
    `ask_before` and `never_allow`, each a block list of tool names or `[]`
    (config/permission.rs:18-44, 257-275 at aaif-goose/goose b23d21c). Only that nesting is read:
    any other line is appended to `errors` as (line, message, raw), as `iter_jsonl` does.
    """
    category = level = ""
    with open(path, "rb") as handle:
        for line_number, raw in enumerate(handle, 1):
            text = without_terminator(raw.decode("utf-8", errors="replace"))
            body = text.strip(SPACE_OR_TAB)
            if not body or body.startswith("#") or body in ("---", "{}"):
                continue
            indent = len(text) - len(text.lstrip(" "))
            if body.startswith("- ") or body == "-":
                tool = _unquote(body[1:].strip(SPACE_OR_TAB))
                if category and level and tool:
                    yield line_number, category, level, tool
                    continue
            elif body.endswith(":") and indent == 0:
                category, level = _unquote(body[:-1].strip(SPACE_OR_TAB)), ""
                continue
            elif indent > 0 and category:
                key, separator, value = body.partition(":")
                key = key.strip(SPACE_OR_TAB)
                if separator and value.strip(SPACE_OR_TAB) in ("", "[]") and key:
                    level = _unquote(key)
                    continue
            errors.append(
                (
                    line_number,
                    "unparseable line (not a category, level or list item)",
                    line_text(raw),
                )
            )


def _audience(block: dict):
    """Return the `annotations.audience` of a content block, or None."""
    annotations = block.get("annotations")
    return annotations.get("audience") if isinstance(annotations, dict) else None


def merge_delta(blocks: list, new: list) -> None:
    """Append one streamed chunk's content blocks to the message assembled so far.

    As `Conversation::push` does for chunks with the same message id
    (goose-provider-types/src/conversation.rs:75-119 at aaif-goose/goose b23d21c): a one-block text
    chunk extends a last text block of the same audience, a one-block thinking chunk extends a last
    thinking block that is unsigned or has its signature, and anything else is appended.
    """
    if len(new) == 1 and blocks and isinstance(blocks[-1], dict) and isinstance(new[0], dict):
        last, block = blocks[-1], new[0]
        kind = last.get("type")
        if kind == "text" == block.get("type") and _audience(last) == _audience(block):
            blocks[-1] = dict(last, text=str(last.get("text") or "") + str(block.get("text") or ""))
            return
        if kind == "thinking" == block.get("type") and (
            not last.get("signature") or block.get("signature") == last.get("signature")
        ):
            merged = dict(
                last, thinking=str(last.get("thinking") or "") + str(block.get("thinking") or "")
            )
            if block.get("signature"):
                merged["signature"] = block["signature"]
            blocks[-1] = merged
            return
    blocks.extend(new)


def schedule_text(job: dict, rel: str) -> str:
    """Return the `scheduled task:` text of one `schedule.json` job.

    The job is a `ScheduledJob` (scheduler/common.rs:166-187 at aaif-goose/goose b23d21c); `-`
    stands for an absent value, and `parameters` is added only when the job has any.
    """

    def value(key: str) -> str:
        stored = job.get(key)
        return "-" if stored is None or stored == "" else setting_value(stored)

    parameters = job.get("parameters")
    return (
        "scheduled task: %s %s paused=%s running=%s source=%s last_run=%s"
        " current_session=%s recipe_base_dir=%s%s (from %s, file mtime)"
        % (
            value("id"),
            value("cron"),
            setting_value(bool(job.get("paused"))),
            setting_value(bool(job.get("currently_running"))),
            value("source"),
            to_utc(job.get("last_run")) or value("last_run"),
            value("current_session_id"),
            value("recipe_base_dir"),
            " parameters=%s" % compact_json(parameters) if parameters else "",
            rel,
        )
    )


class GooseParser(Parser):
    """Parse Goose's session database, legacy sessions, request logs, histories and settings."""

    agent = "goose"
    name = "goose"
    has_secondary = True
    records_relay = True

    def wants(self, artifact: Artifact) -> bool:
        """Return True for the Goose files this parser reads."""
        rel = artifact.rel
        return any(
            pattern.match(rel)
            for pattern in (
                DATABASE_PATTERN,
                LEGACY_PATTERN,
                LLM_PATTERN,
                HISTORY_PATTERN,
                PERMISSION_PATTERN,
                TRUST_PATTERN,
                SCHEDULE_PATTERN,
            )
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for the prompt histories and request logs, which repeat `sessions.db`."""
        return bool(HISTORY_PATTERN.match(artifact.rel) or LLM_PATTERN.match(artifact.rel))

    def secondary_tolerance(self, artifact: Artifact) -> float:
        """Return `LLM_TOLERANCE_SECONDS` for a request log, else the default window."""
        if LLM_PATTERN.match(artifact.rel):
            return LLM_TOLERANCE_SECONDS
        return super().secondary_tolerance(artifact)

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Goose file."""
        rel = artifact.rel
        if DATABASE_PATTERN.match(rel):
            yield from self._parse_database(artifact, opts)
        elif LEGACY_PATTERN.match(rel):
            yield from self._parse_legacy(artifact, opts)
        elif LLM_PATTERN.match(rel):
            yield from self._parse_llm_log(artifact, opts)
        elif PERMISSION_PATTERN.match(rel):
            yield from self._parse_permission_yaml(artifact)
        elif TRUST_PATTERN.match(rel):
            yield from self._parse_roaming_trust(artifact)
        elif SCHEDULE_PATTERN.match(rel):
            yield from self._parse_schedule(artifact)
        else:
            yield from self._parse_history(artifact, opts)

    def _row(
        self,
        artifact: Artifact,
        line: int,
        session_id: str = "",
        project_path: str = "",
        model: str = "",
        timestamp: str = "",
    ) -> Row:
        row = self.base_row(artifact)
        row.source_line = line
        row.session_id = session_id
        row.project_path = project_path
        row.model = model
        row.timestamp_utc = timestamp
        return row

    @staticmethod
    def _system_copy(base: Row, text: str) -> Row:
        """A `system` row with `base`'s attribution, time and model."""
        row = Row(**base.__dict__)
        row.turn_type = "system"
        row.tool_name = ""
        row.tool_use_id = ""
        row.text = text
        return row

    # -- content blocks, shared by every store ------------------------------------------
    def _block_rows(
        self,
        artifact: Artifact,
        base: Row,
        role: str,
        content,
        user_visible: bool,
        tool_names: dict[str, str],
        opts: Options,
        answered: set[str] | None = None,
    ) -> Iterator[Row]:
        """Yield the rows of a message's content blocks.

        `answered` holds the ids of the tool requests the session's `toolConfirmationResponse`
        blocks answer; when it is given (only for `sessions.db`), a `toolRequest` whose `_meta`
        holds `goose.executable` `false` and no answer is followed by its `permission decision: not
        executable` row.
        """
        if isinstance(content, str):
            text_block: dict = {"type": "text", "text": content}
            content = [text_block]
        if not isinstance(content, list):
            return
        text_type = role if role in ("user", "assistant") else "system"
        for block in content:
            if not isinstance(block, dict):
                continue
            kind = block.get("type")
            row = Row(**base.__dict__)
            if kind == "text":
                row.turn_type = text_type
                text = block.get("text") or ""
                if role not in ("user", "assistant"):
                    text = "%s: %s" % (role, text)
                if not user_visible:
                    text = "[hidden] " + text
            elif kind in ("image", "document"):
                row.turn_type = text_type
                text = "[%s] %s" % (
                    kind,
                    _join(str(block.get("name") or ""), str(block.get("mimeType") or "")),
                )
            elif kind == "toolRequest":
                row.turn_type = "tool_use"
                row.tool_use_id = str(block.get("id") or "")
                call = block.get("toolCall") or {}
                if call.get("status") == "error":
                    text = "[error] " + str(call.get("error") or "")
                else:
                    value = call.get("value") or {}
                    row.tool_name = str(value.get("name") or "")
                    text = compact_json(value.get("arguments") or {})
                tool_names[row.tool_use_id] = row.tool_name
            elif kind == "toolResponse":
                row.turn_type = "tool_result"
                row.tool_use_id = str(block.get("id") or "")
                row.tool_name = tool_names.get(row.tool_use_id, "")
                text, is_error = result_text(block.get("toolResult"))
                if is_error:
                    text = "[error] " + text
            elif kind in ("thinking", "redactedThinking"):
                if not opts.include_thinking:
                    continue
                row.turn_type = "thinking"
                text = (
                    (block.get("thinking") or "") if kind == "thinking" else "[redacted thinking]"
                )
            elif kind == "toolConfirmationRequest":
                row.turn_type = "system"
                row.tool_name = str(block.get("toolName") or "")
                row.tool_use_id = str(block.get("id") or "")
                text = _join(
                    "confirmation requested: " + row.tool_name,
                    str(block.get("prompt") or ""),
                    compact_json(block.get("arguments") or {}),
                )
            elif kind == "systemNotification":
                row.turn_type = "system"
                text = "%s: %s" % (
                    block.get("notificationType") or "notification",
                    block.get("msg") or "",
                )
            elif kind == "error":
                row.turn_type = "system"
                text = "error %s: %s" % (block.get("kind") or "", block.get("message") or "")
            elif kind == "actionRequired":
                row.turn_type = "system"
                text = self._action_text(row, block.get("data"), tool_names)
            else:
                row.turn_type = "system"
                text = compact_json(block)
            row.text = compact(text)
            yield row
            if kind == "toolRequest" and answered is not None:
                verdict = block.get("_meta")
                if (
                    isinstance(verdict, dict)
                    and verdict.get(TOOL_EXECUTABLE_KEY) is False
                    and row.tool_use_id not in answered
                ):
                    denied = Row(**row.__dict__)
                    denied.turn_type = "system"
                    denied.text = "permission decision: not executable (goose.executable=false)"
                    yield denied

    @staticmethod
    def _action_text(row: Row, data, tool_names: dict[str, str]) -> str:
        """Return the text of an `actionRequired` block, setting `row`'s tool join.

        A tool confirmation is a `permission request:` and its answer a `permission decision:`
        (message.rs:194-220 at aaif-goose/goose b23d21c); an elicitation keeps the generic `action
        required:` row.
        """
        kind = data.get("actionType") if isinstance(data, dict) else None
        if isinstance(data, dict) and kind == "toolConfirmation":
            row.tool_name = str(data.get("toolName") or "")
            row.tool_use_id = str(data.get("id") or "")
            return _join(
                "permission request: " + row.tool_name,
                str(data.get("prompt") or ""),
                compact_json(data.get("arguments") or {}),
            )
        if isinstance(data, dict) and kind == "toolConfirmationResponse":
            row.tool_use_id = str(data.get("id") or "")
            row.tool_name = tool_names.get(row.tool_use_id, "")
            return "permission decision: %s (toolConfirmationResponse)" % setting_value(
                data.get("permission") or "-"
            )
        return "action required: " + compact_json(data)

    # -- sessions.db ---------------------------------------------------------------------
    def _parse_database(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        if not is_sqlite(artifact.disk_path):
            row = self._row(artifact, 0)
            row.turn_type = "system"
            row.text = "parser: not a SQLite database"
            yield row
            return
        try:
            with open_copy(artifact.disk_path) as connection:
                tables = table_names(connection)
                if "messages" not in tables:
                    return
                sessions: dict[str, dict] = {}
                seen_model: dict[str, str] = {}
                session_models: dict[str, str] = {}
                seen_mode: dict[str, str] = {}
                if "sessions" in tables:
                    for session in connection.execute(
                        "select rowid as _rowid, * from sessions order by created_at, rowid"
                    ):
                        session = dict(session)
                        config, reason, raw = _stored_json(session.get("model_config_json"), dict)
                        config_errors = []
                        if reason:
                            config_errors.append(
                                (
                                    session["_rowid"],
                                    "unparseable record %d (sessions.model_config_json: %s)"
                                    % (session["_rowid"], reason),
                                    raw,
                                )
                            )
                        model_name = str(
                            (config.get("model_name") if isinstance(config, dict) else "") or ""
                        )
                        session_id = str(session.get("id") or "")
                        session_models[session_id] = _model(
                            session.get("provider_name") or "", model_name
                        )
                        sessions[session_id] = session
                        row = self._row(
                            artifact,
                            session["_rowid"],
                            session_id,
                            session.get("working_dir") or "",
                            session_models[session_id],
                            to_utc(session.get("created_at")),
                        )
                        row.turn_type = "system"
                        row.text = compact(
                            _join(
                                "session start",
                                session.get("name") or "",
                                session.get("description") or "",
                                "type=%s" % session["session_type"]
                                if session.get("session_type")
                                else "",
                                "parent=%s" % session["parent_session_id"]
                                if session.get("parent_session_id")
                                else "",
                                "schedule=%s" % session["schedule_id"]
                                if session.get("schedule_id")
                                else "",
                                "archived=%s" % session["archived_at"]
                                if session.get("archived_at")
                                else "",
                            ),
                        )
                        yield row
                        yield from error_rows(
                            self,
                            artifact,
                            config_errors,
                            session_id,
                            session.get("working_dir") or "",
                        )
                        text = gateway_text(session)
                        if text:
                            yield self._system_copy(row, text)
                        text = track_change("model", seen_model, session_id, model_name)
                        if text:
                            if session.get("provider_name"):
                                text += " provider=%s" % session["provider_name"]
                            yield self._system_copy(row, text)
                        text = track_change(
                            "permission mode",
                            seen_mode,
                            session_id,
                            str(session.get("goose_mode") or ""),
                        )
                        if text:
                            yield self._system_copy(row, text)
                answered = self._answered(connection)
                tool_names: dict[str, str] = {}
                for message in connection.execute(
                    "select * from messages order by session_id, created_timestamp, id"
                ):
                    message = dict(message)
                    session_id = str(message.get("session_id") or "")
                    session = sessions.get(session_id, {})
                    project_path = session.get("working_dir") or ""
                    record = message.get("id") or 0
                    message_errors: list = []
                    meta, reason, raw = _stored_json(message.get("metadata_json"), dict)
                    if reason:
                        message_errors.append(
                            (
                                record,
                                "unparseable record %s (messages.metadata_json: %s)"
                                % (record, reason),
                                raw,
                            )
                        )
                    meta = meta if isinstance(meta, dict) else {}
                    inference = meta.get("inference") or {}
                    model = _model(
                        inference.get("provider") or "",
                        inference.get("resolvedModel") or inference.get("requestedModel") or "",
                    ) or session_models.get(session_id, "")
                    base = self._row(
                        artifact,
                        record,
                        session_id,
                        project_path,
                        model,
                        epoch(message.get("created_timestamp")) or to_utc(message.get("timestamp")),
                    )
                    resolved = str(
                        inference.get("resolvedModel") or inference.get("requestedModel") or ""
                    )
                    text = track_change("model", seen_model, session_id, resolved)
                    if text:
                        if inference.get("provider"):
                            text += " provider=%s" % inference["provider"]
                        requested = str(inference.get("requestedModel") or "")
                        if requested and requested != resolved:
                            text += " requested=%s" % requested
                        yield self._system_copy(base, text)
                    yield from error_rows(self, artifact, message_errors, session_id, project_path)
                    content, reason, raw = _stored_json(message.get("content_json"), list)
                    if content is None and not reason:
                        reason = "not a JSON array" if raw else "empty"
                    if reason:
                        yield from error_rows(
                            self,
                            artifact,
                            [
                                (
                                    record,
                                    "unparseable record %s (messages.content_json: %s)"
                                    % (record, reason),
                                    raw,
                                )
                            ],
                            session_id,
                            project_path,
                        )
                        continue
                    yield from self._block_rows(
                        artifact,
                        base,
                        str(message.get("role") or ""),
                        content,
                        meta.get("userVisible", True) is not False,
                        tool_names,
                        opts,
                        answered.get(session_id, set()),
                    )
        except sqlite3.DatabaseError as error:
            row = self._row(artifact, 0)
            row.turn_type = "system"
            row.text = "parser: SQLite error: %s" % error
            yield row

    @staticmethod
    def _answered(connection) -> dict[str, set[str]]:
        """Map each session id to the tool request ids its `toolConfirmationResponse` blocks answer.

        A pre-scan: a `content_json` that does not decode is skipped here and reported by the full
        read of the same row.
        """
        answered: dict[str, set[str]] = {}
        for session_id, stored in connection.execute(
            "select session_id, content_json from messages"
            " where content_json like '%toolConfirmationResponse%'"
        ):
            content, _, _ = _stored_json(stored, list)
            for block in content if isinstance(content, list) else []:
                data = block.get("data") if isinstance(block, dict) else None
                if isinstance(data, dict) and data.get("actionType") == "toolConfirmationResponse":
                    answered.setdefault(str(session_id or ""), set()).add(str(data.get("id") or ""))
        return answered

    # -- legacy sessions/<YYYYMMDD_HHMMSS>.jsonl ------------------------------------------
    def _parse_legacy(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        session_id = artifact.disk_path.name[: -len(".jsonl")]
        project_path = ""
        tool_names: dict[str, str] = {}
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            if "role" not in record and "content" not in record:
                session_id = str(record.get("id") or session_id)
                project_path = str(record.get("working_dir") or "")
                row = self._row(
                    artifact,
                    line_number,
                    session_id,
                    project_path,
                    timestamp=to_utc(record.get("created_at")),
                )
                row.turn_type = "system"
                row.text = compact(
                    _join(
                        "session start (legacy)",
                        str(record.get("description") or ""),
                        "%s messages" % record["message_count"]
                        if "message_count" in record
                        else "",
                    ),
                )
                yield row
                continue
            meta = record.get("metadata")
            if not isinstance(meta, dict):
                meta = {}
            base = self._row(
                artifact,
                line_number,
                session_id,
                project_path,
                timestamp=epoch(record.get("created")),
            )
            yield from self._block_rows(
                artifact,
                base,
                str(record.get("role") or ""),
                record.get("content"),
                meta.get("userVisible", True) is not False,
                tool_names,
                opts,
            )
        yield from error_rows(self, artifact, errors, session_id, project_path)

    # -- logs/llm_request.*.jsonl ---------------------------------------------------------
    def _parse_llm_log(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        request: Row | None = None
        pending: list[Row] = []
        model = ""
        tool_names: dict[str, str] = {}
        # The message being assembled from streamed chunks: its first chunk's
        # row (time, line), its id, its role and its content blocks so far.
        current: tuple[Row, str, str, list] | None = None

        def flush() -> None:
            nonlocal current
            if current is not None:
                base, _, role, blocks = current
                pending.extend(
                    self._block_rows(artifact, base, role, blocks, True, tool_names, opts)
                )
                current = None

        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            if "input" in record or "model_config" in record:
                flush()
                config = record.get("model_config") or {}
                request_input = record.get("input")
                if not isinstance(request_input, dict):
                    request_input = {}
                model = str(config.get("model_name") or request_input.get("model") or "")
                messages = request_input.get("messages")
                if not isinstance(messages, list):
                    messages = []
                last_user = next(
                    (
                        text_of(message.get("content"))
                        for message in reversed(messages)
                        if isinstance(message, dict) and message.get("role") == "user"
                    ),
                    "",
                )
                request = self._row(artifact, line_number, model=model)
                request.turn_type = "system"
                request.text = compact(
                    _join(
                        "llm request",
                        "%d messages" % len(messages),
                        "last user: " + last_user if last_user else "",
                    ),
                )
                continue
            data = record.get("data")
            if isinstance(data, dict) and "content" in data:
                content = data.get("content")
                if isinstance(content, str):
                    content = [{"type": "text", "text": content}]
                if not isinstance(content, list) or not content:
                    continue  # a usage or metadata chunk adds no content
                message_id = str(data.get("id") or "")
                if current is not None and message_id and current[1] == message_id:
                    merge_delta(current[3], content)
                    continue
                flush()
                base = self._row(
                    artifact, line_number, model=model, timestamp=epoch(data.get("created"))
                )
                if request is not None and not request.timestamp_utc:
                    request.timestamp_utc = base.timestamp_utc
                current = (base, message_id, str(data.get("role") or "assistant"), list(content))
            elif record.get("error"):
                flush()
                row = self._row(artifact, line_number, model=model)
                row.turn_type = "system"
                row.text = compact(
                    "llm error: "
                    + (
                        record["error"]
                        if isinstance(record["error"], str)
                        else compact_json(record["error"])
                    ),
                )
                pending.append(row)
        flush()
        if request is not None:
            yield request
        yield from pending
        yield from error_rows(self, artifact, errors)

    # -- standing files: permission.yaml, roaming_trust.json, schedule.json -------------
    def _standing_row(self, artifact: Artifact, line: int, timestamp: str, text: str) -> Row:
        row = self._row(artifact, line, timestamp=timestamp)
        row.turn_type = "system"
        row.text = compact(text)
        return row

    def _read_json_file(self, artifact: Artifact) -> tuple[object, str]:
        """Return (document, error message) of a whole-file JSON store."""
        try:
            raw = artifact.disk_path.read_bytes().decode("utf-8", errors="replace")
        except OSError as error:
            return None, "unreadable %s (%s)" % (artifact.rel, error)
        try:
            return json.loads(raw), ""
        except ValueError as error:
            return None, "%s is not JSON (%s)" % (artifact.rel, error)

    def _parse_permission_yaml(self, artifact: Artifact) -> Iterator[Row]:
        errors: list = []
        timestamp = file_mtime(artifact)
        for line_number, category, level, tool in permission_entries(artifact.disk_path, errors):
            yield self._standing_row(
                artifact,
                line_number,
                timestamp,
                "permission setting: %s.%s=%s (from %s, file mtime)"
                % (category, level, tool, artifact.rel),
            )
        yield from error_rows(self, artifact, errors)

    def _parse_roaming_trust(self, artifact: Artifact) -> Iterator[Row]:
        document, message = self._read_json_file(artifact)
        timestamp = file_mtime(artifact)
        if not message and not isinstance(document, dict):
            message = "%s is not a JSON object" % artifact.rel
        if message or not isinstance(document, dict):
            yield self._standing_row(artifact, 0, timestamp, "parser: " + message)
            return
        for key in ("allowed", "revoked_keys"):
            values = document.get(key)
            for peer in values if isinstance(values, list) else []:
                yield self._standing_row(
                    artifact,
                    1,
                    timestamp,
                    "permission setting: roaming_trust.%s=%s (from %s, file mtime)"
                    % (key, setting_value(peer), artifact.rel),
                )

    def _parse_schedule(self, artifact: Artifact) -> Iterator[Row]:
        document, message = self._read_json_file(artifact)
        timestamp = file_mtime(artifact)
        if not message and not isinstance(document, list):
            message = "%s is not a JSON array" % artifact.rel
        if message or not isinstance(document, list):
            yield self._standing_row(artifact, 0, timestamp, "parser: " + message)
            return
        errors: list = []
        for i, job in enumerate(document, 1):
            if not isinstance(job, dict):
                errors.append(
                    (0, "unparseable record %d (not a JSON object)" % i, compact_json(job))
                )
                continue
            yield self._standing_row(artifact, 1, timestamp, schedule_text(job, artifact.rel))
        yield from error_rows(self, artifact, errors)

    # -- history.txt ------------------------------------------------------------------------
    def _parse_history(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        with open(artifact.disk_path, "rb") as handle:
            for line_number, raw in enumerate(handle, 1):
                line = raw.decode("utf-8", errors="replace").rstrip("\r\n")
                if not line or (line_number == 1 and line.startswith("#V")):
                    continue
                row = self._row(artifact, line_number)
                row.turn_type = "user"
                row.text = compact(unescape_history(line))
                yield row
