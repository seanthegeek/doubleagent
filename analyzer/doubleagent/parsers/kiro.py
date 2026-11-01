"""Amazon Q Developer CLI conversations, the open-source predecessor of the Kiro CLI.

Reads, relative to the home: `data.sqlite3` under `.local/share`, `Library/Application
Support` or `AppData/Local`, in `amazon-q/` or `kiro-cli/` (tables `conversations(key, value)`,
one serialised `ConversationState` per working directory, and the legacy Fig shell `history`;
`auth_kv` and `state` hold tokens and are never read); `/save` exports, a `.json` whose
top-level object has `conversation_id` and `history`; `.aws/amazonq/.cli_bash_history`, the
prompt's rustyline history (secondary); agent configurations
`{.aws/amazonq,.amazonq}/cli-agents/<name>.json`; and the delegate tool's
`.amazonq/.subagents/<agent>.json`. Kiro CLI's `.kiro/sessions/` files are not read.

Fields the parser depends on:

* `ConversationState`: `conversation_id`, `history[]` of `{user, assistant,
  request_metadata}`, `next_message`, `latest_summary`, `model_info.model_id`, legacy
  `model`, `checkpoint_manager.checkpoints[]` (`tag`, `timestamp`, `description`,
  `tool_name`, `history_snapshot`) and `tangent_state.main_history`.
* `user.content` (`Prompt`, `ToolUseResults`, `CancelledToolUses`), `user.timestamp`,
  `user.env_context.env_state.current_working_directory`; `assistant` (`Response`,
  `ToolUse` with `tool_uses[]` `id`, `name`, `orig_name`, `args`); `request_metadata`
  (`request_start_timestamp_ms`, `stream_end_timestamp_ms`, `model_id`,
  `message_meta_tags`).
* `history` columns `command`, `shell`, `session_id`, `cwd`, `start_time` or `time`,
  `duration`, `hostname`, `exit_code`; agent `allowedTools` and `toolsSettings`; delegate
  `agent`, `task`, `status`, `launched_at`, `completed_at`, `pid`, `exit_code`, `output`,
  `summary`, `cwd`.

Validated against source, aws/amazon-q-developer-cli at 15cc8f3 (1.19.7), with a synthetic
fixture; no real install. A blob cut mid-write is salvaged entry by entry. See
`research/kiro.md`.
"""

from __future__ import annotations

import json
import re
import sqlite3
from collections.abc import Iterator

from ..inputs import Artifact
from ..model import Row, compact
from ..sqlite_util import open_copy, sqlite_problem, table_names
from ..timeutil import to_utc
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_bytes,
    file_mtime,
    iter_lines,
    line_text,
    setting_value,
    track_change,
)

DATABASE_PATTERN = re.compile(
    r"^(?:\.local/share|Library/Application Support|AppData/Local)/(?:amazon-q|kiro-cli)/data\.sqlite3$"
)
SESSIONS_PATTERN = re.compile(r"^\.kiro/sessions/")
# The chat prompt's rustyline history (Q `paths.rs` `CLI_BASH_HISTORY`).
HISTORY_PATTERN = re.compile(r"^\.aws/amazonq/\.cli_bash_history$")
# Agent configurations, global and per project (Q `paths.rs` `AGENTS_DIR`).
AGENT_PATTERN = re.compile(r"^(?:\.aws/amazonq|\.amazonq)/cli-agents/[^/]+\.json$")
# The delegate tool's per-agent task records (Q `paths.rs` `SUBAGENTS_DIR`).
SUBAGENT_PATTERN = re.compile(r"^\.amazonq/\.subagents/[^/]+\.json$")
# The header rustyline writes first in a multi-line-aware history file.
HISTORY_V2 = "#V2"
# The fixed prompts of a CancelledToolUses message (Q `cli/chat/mod.rs`).
DENY_TEXT = "I deny this tool request. Ask a follow up question clarifying the expected action"
INTERRUPT_TEXT = "The user interrupted the tool execution."
DENY_RULE_PATTERN = re.compile(
    r"^Tool use with (.+) was rejected because the arguments supplied were forbidden$", re.S
)
# The agent the delegate tool runs with --trust-all-tools (Q `cli/agent/mod.rs`).
DEFAULT_AGENT_NAME = "q_cli_default"
SNIFF_BYTES = 4096

_CONVERSATION_ID_PATTERN = re.compile(r'"conversation_id"[ \t\n\r]*:[ \t\n\r]*"((?:[^"\\]|\\.)*)"')
_HISTORY_PATTERN = re.compile(r'"history"[ \t\n\r]*:[ \t\n\r]*\[')


def _sniff_export(path) -> bool:
    """Return True when a file starts like a `/save` export (an object with `conversation_id`).

    This is file selection, not a parse: a file that cannot be read is not
    recognised, so it is never offered to the parser and gives no row.
    """
    try:
        with open(path, "rb") as handle:
            head = handle.read(SNIFF_BYTES)
    except OSError:
        return False
    return head.lstrip()[:1] == b"{" and b'"conversation_id"' in head


def salvage(text: str) -> tuple[dict | None, str]:
    """Parse a `ConversationState`, salvaging what precedes the damage when it does not parse.

    On failure, `conversation_id` and every complete `history` entry before the damage are
    recovered. Returns (state or None, error message), the message empty when the whole document
    parsed.
    """
    try:
        state = json.loads(text)
        if isinstance(state, dict):
            return state, ""
        return None, "not a JSON object"
    except ValueError as error:
        message = str(error)
    match = _HISTORY_PATTERN.search(text)
    if not match:
        return None, message
    state = {"history": []}
    conversation_match = _CONVERSATION_ID_PATTERN.search(text, 0, match.start())
    if conversation_match:
        try:
            state["conversation_id"] = json.loads('"%s"' % conversation_match.group(1))
        except ValueError:
            pass
    decoder = json.JSONDecoder()
    i = match.end()
    length = len(text)
    while i < length:
        while i < length and text[i] in " \t\r\n,":
            i += 1
        if i >= length or text[i] == "]":
            break
        try:
            entry, i = decoder.raw_decode(text, i)
        except ValueError:
            break
        if isinstance(entry, dict):
            state["history"].append(entry)
    return state, message


def _variant(value) -> tuple[str, dict]:
    """Unpack a serde externally tagged enum, `{"Tag": {...}}`, as (tag, body)."""
    if isinstance(value, dict) and len(value) == 1:
        tag, body = next(iter(value.items()))
        return str(tag), body if isinstance(body, dict) else {}
    return "", {}


def result_text(result: dict) -> str:
    """Return the text of a tool use result, its `status` in brackets after it."""
    parts: list[str] = []
    for block in result.get("content") or []:
        if not isinstance(block, dict):
            parts.append(str(block))
        elif isinstance(block.get("Text"), str):
            parts.append(block["Text"])
        elif "Json" in block:
            parts.append(compact_json(block["Json"]))
        else:
            parts.append(compact_json(block))
    text = "\n".join(parts)
    status = result.get("status")
    if status:
        text = "%s [%s]" % (text, status) if text else "[%s]" % status
    return text


def unescape_history(line: str) -> str:
    r"""Return one entry of a rustyline `#V2` history file: `\n` a line feed, `\\` a backslash.

    A line with any other escape, or a trailing backslash, is kept exactly as written, as rustyline
    15's `load_from` keeps it.
    """
    out: list[str] = []
    i = 0
    length = len(line)
    while i < length:
        char = line[i]
        if char != "\\":
            out.append(char)
            i += 1
            continue
        next_char = line[i + 1] if i + 1 < length else ""
        if next_char == "n":
            out.append("\n")
        elif next_char == "\\":
            out.append("\\")
        else:
            return line
        i += 2
    return "".join(out)


def decision_text(prompt) -> str:
    """Return the `permission decision:` text of a CancelledToolUses message's `prompt`.

    The fixed denial text of a bare `n`, a deny rule of the agent's `toolsSettings`, an interrupt,
    or the user's own words typed at the approval prompt.
    """
    if not isinstance(prompt, str) or not prompt:
        return "permission decision: cancelled"
    if prompt == DENY_TEXT:
        return "permission decision: denied by=n"
    if prompt == INTERRUPT_TEXT:
        return "permission decision: cancelled by=interrupt"
    match = DENY_RULE_PATTERN.match(prompt)
    if match:
        return "permission decision: denied by=deny-rule tool=%s" % match.group(1)
    return "permission decision: denied by=user-text"


def _entry_key(entry: dict) -> str:
    """Return what identifies a history entry across the copies one blob holds.

    The user content and time and the assistant message: a snapshot entry is a clone of the history
    entry, so the two are equal on these.
    """
    user = entry.get("user")
    user = user if isinstance(user, dict) else {}
    return compact_json([user.get("content"), user.get("timestamp"), entry.get("assistant")])


class KiroParser(Parser):
    """Parse the Amazon Q Developer CLI database, exports, prompt history and agent files."""

    agent = "kiro"
    name = "kiro"
    has_secondary = True

    def wants(self, artifact: Artifact) -> bool:
        """Return True for the database, history and agent files and for `/save` exports."""
        rel = artifact.rel
        if DATABASE_PATTERN.match(rel) or HISTORY_PATTERN.match(rel):
            return True
        if AGENT_PATTERN.match(rel) or SUBAGENT_PATTERN.match(rel):
            return True
        if SESSIONS_PATTERN.match(rel) or not rel.endswith(".json"):
            return False
        return _sniff_export(artifact.disk_path)

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for the prompt history, which repeats the conversations' prompts."""
        return bool(HISTORY_PATTERN.match(artifact.rel))

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Amazon Q Developer CLI file."""
        rel = artifact.rel
        if DATABASE_PATTERN.match(rel):
            yield from self._parse_database(artifact, opts)
        elif HISTORY_PATTERN.match(rel):
            yield from self._parse_history(artifact)
        elif AGENT_PATTERN.match(rel):
            yield from self._parse_agent_config(artifact)
        elif SUBAGENT_PATTERN.match(rel):
            yield from self._parse_delegate(artifact)
        else:
            yield from self._parse_export(artifact, opts)

    # -- .cli_bash_history ---------------------------------------------------------
    def _parse_history(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `user` row per history entry, with no timestamp, session or project."""
        errors: list = []
        multiline = False
        for line_number, raw in iter_lines(artifact.disk_path, errors):
            try:
                raw.decode("utf-8")
            except UnicodeDecodeError:
                errors.append((line_number, "unparseable line (not valid UTF-8)", line_text(raw)))
                continue
            line = line_text(raw)
            if line_number == 1 and line == HISTORY_V2:
                multiline = True
                continue
            if not line:
                continue
            row = self.base_row(artifact)
            row.source_line = line_number
            row.turn_type = "user"
            row.text = compact(unescape_history(line) if multiline else line)
            yield row
        yield from error_rows(self, artifact, errors)

    # -- cli-agents/<name>.json ---------------------------------------------------
    def _json_doc(self, artifact: Artifact, what: str) -> tuple[dict | None, Row | None]:
        """Return the file as a JSON object, or a `system` row when it is unreadable or not one."""
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            return None, next(error_rows(self, artifact, errors))
        try:
            document = json.loads(content.decode("utf-8", errors="replace"))
        except ValueError:
            document = None
        if isinstance(document, dict):
            return document, None
        row = self.base_row(artifact)
        row.source_line = 1
        row.turn_type = "system"
        row.timestamp_utc = file_mtime(artifact)
        row.text = "parser: %s is not a JSON object" % what
        return None, row

    def _parse_agent_config(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `permission setting:` row per `allowedTools` entry and `toolsSettings` key."""
        document, bad = self._json_doc(artifact, "agent configuration")
        if document is None:
            if bad is not None:
                yield bad
            return
        timestamp = file_mtime(artifact)
        project = artifact.home.original if artifact.home.project else ""

        def setting(key: str, value) -> Row:
            row = self.base_row(artifact)
            row.source_line = 1
            row.turn_type = "system"
            row.timestamp_utc = timestamp
            row.project_path = project
            row.text = compact(
                "permission setting: %s=%s (from %s, file mtime)"
                % (key, setting_value(value), artifact.rel)
            )
            return row

        allowed = document.get("allowedTools")
        if isinstance(allowed, list):
            for entry in allowed:
                yield setting("allowedTools", entry)
        tools = document.get("toolsSettings")
        if isinstance(tools, dict):
            for tool, conf in tools.items():
                if isinstance(conf, dict):
                    for key, value in conf.items():
                        yield setting("toolsSettings.%s.%s" % (tool, key), value)
                else:
                    yield setting("toolsSettings.%s" % tool, conf)

    # -- .amazonq/.subagents/<agent>.json -----------------------------------------
    def _parse_delegate(self, artifact: Artifact) -> Iterator[Row]:
        """Yield a delegate task's `launched` row and, once it ended, its status row.

        The status row, at `completed_at`, holds the exit code, summary and output.
        """
        document, bad = self._json_doc(artifact, "delegate record")
        if document is None:
            if bad is not None:
                yield bad
            return
        cwd = str(document.get("cwd") or "")
        if not cwd or cwd == "Unknown":
            cwd = artifact.home.original if artifact.home.project else ""
        agent = str(document.get("agent") or "")

        def when(value) -> str:
            valid = isinstance(value, (int, float)) and not isinstance(value, bool) and value > 0
            return to_utc(value) if valid else ""

        def event(timestamp: str, text: str) -> Row:
            row = self.base_row(artifact)
            row.source_line = 1
            row.turn_type = "system"
            row.timestamp_utc = timestamp
            row.project_path = cwd
            row.text = compact(text)
            return row

        trust = " trust_all_tools=true" if agent == DEFAULT_AGENT_NAME else ""
        yield event(
            when(document.get("launched_at")),
            "delegate task launched: agent=%s pid=%s%s: %s"
            % (agent, document.get("pid", ""), trust, document.get("task") or ""),
        )
        status = str(document.get("status") or "")
        completed = document.get("completed_at")
        if completed is None and status in ("", "running"):
            return
        exit_code = document.get("exit_code")
        text = "delegate task %s: agent=%s exit_code=%s" % (
            status or "ended",
            agent,
            "" if exit_code is None else exit_code,
        )
        if document.get("summary"):
            text += "\nsummary: %s" % document["summary"]
        if document.get("output"):
            text += "\noutput: %s" % document["output"]
        yield event(when(completed), text)

    # -- data.sqlite3 -------------------------------------------------------------
    def _parse_database(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of `data.sqlite3`, or one row for a database that cannot be read."""
        problem = sqlite_problem(artifact.disk_path)
        if problem:
            yield from error_rows(self, artifact, [(0, problem)])
            return
        try:
            with open_copy(artifact.disk_path) as connection:
                rows = list(self._database_rows(artifact, opts, connection))
        except sqlite3.DatabaseError as error:
            yield from error_rows(self, artifact, [(0, "SQLite error: %s" % error)])
            return
        yield from rows

    def _database_rows(self, artifact: Artifact, opts: Options, connection) -> Iterator[Row]:
        """Yield the conversation and legacy shell history rows of an open database."""
        tables = table_names(connection)
        if "conversations" in tables:
            for position, table_row in enumerate(
                connection.execute("select key, value from conversations order by rowid"), 1
            ):
                value = table_row["value"]
                if isinstance(value, bytes):
                    value = value.decode("utf-8", errors="replace")
                yield from self._conversation(
                    artifact, opts, str(value or ""), str(table_row["key"] or ""), position
                )
        if "history" in tables:
            yield from self._shell_history(artifact, opts, connection)

    def _shell_history(self, artifact: Artifact, opts: Options, connection) -> Iterator[Row]:
        """Yield one `shell history:` row per legacy Fig shell history row."""
        for position, history_row in enumerate(
            connection.execute("select * from history order by rowid"), 1
        ):
            history_row = dict(history_row)
            row = self.base_row(artifact)
            row.source_line = position
            start = (
                history_row.get("start_time")
                if "start_time" in history_row
                else history_row.get("time")
            )
            row.timestamp_utc = to_utc(start)
            row.session_id = str(history_row.get("session_id") or "")
            row.project_path = str(history_row.get("cwd") or "")
            row.turn_type = "system"
            extras = [
                "%s=%s" % (k, history_row[k])
                for k in ("exit_code", "shell", "hostname", "duration")
                if history_row.get(k) not in (None, "")
            ]
            text = "shell history: %s" % (history_row.get("command") or "")
            if extras:
                text += " (%s)" % " ".join(extras)
            row.text = compact(text)
            yield row

    # -- /save exports ------------------------------------------------------------
    def _parse_export(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of a `/save` export."""
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors)
            return
        yield from self._conversation(
            artifact, opts, content.decode("utf-8", errors="replace"), "", 0
        )

    # -- ConversationState --------------------------------------------------------
    def _conversation(
        self, artifact: Artifact, opts: Options, text: str, key: str, line: int
    ) -> Iterator[Row]:
        """Yield the rows of one serialised `ConversationState`.

        Entries without a time of their own inherit the previous row's; the entries that only a
        checkpoint snapshot or the tangent-mode copy still holds follow the history's rows, marked
        recovered.
        """
        state, problem = salvage(text)
        if state is None or not isinstance(state.get("history"), list):
            if problem:
                row = self.base_row(artifact)
                row.source_line = line
                row.project_path = key
                row.turn_type = "system"
                row.text = "parser: unparseable conversation: %s" % problem
                yield row
            return
        session_id = str(state.get("conversation_id") or "")
        info = state.get("model_info")
        state_model = ""
        state_model_source = ""
        if isinstance(info, dict) and info.get("model_id"):
            state_model, state_model_source = str(info["model_id"]), "model_info"
        elif isinstance(state.get("model"), str):
            state_model, state_model_source = state["model"], "model"
        project = key or self._first_cwd(state)
        names: dict[str, str] = {}
        seen_model: dict[str, str] = {}
        last_timestamp = [""]

        def base(row_time: str, model: str = "") -> Row:
            row = self.base_row(artifact)
            row.source_line = line
            if row_time:
                last_timestamp[0] = row_time
            row.timestamp_utc = row_time or last_timestamp[0]
            row.session_id = session_id
            row.project_path = project
            row.model = model
            return row

        def fill(row: Row, content: str) -> Row:
            row.text = compact(content)
            return row

        def entry_rows(entry: dict, seen_model: dict[str, str]) -> Iterator[Row]:
            meta = entry.get("request_metadata")
            meta = meta if isinstance(meta, dict) else {}
            model = str(meta.get("model_id") or state_model)
            start_time = to_utc(meta.get("request_start_timestamp_ms"))
            end_time = to_utc(meta.get("stream_end_timestamp_ms")) or start_time
            user = entry.get("user")
            user_time = to_utc(user.get("timestamp")) if isinstance(user, dict) else ""
            text = track_change("model", seen_model, session_id, model)
            if text:
                if not meta.get("model_id"):
                    text += " source=%s" % state_model_source
                row = base(user_time or start_time or end_time, model)
                row.turn_type = "system"
                yield fill(row, text)
            if isinstance(user, dict):
                user_time = user_time or start_time
                yield from self._user(user.get("content"), user_time, model, names, base, fill)
            assistant = entry.get("assistant")
            tag, body = _variant(assistant)
            if tag in ("Response", "ToolUse"):
                content = body.get("content")
                if content:
                    yield self._assistant(base(end_time, model), content, fill)
                for use in body.get("tool_uses") or []:
                    if not isinstance(use, dict):
                        continue
                    tool_id = str(use.get("id") or "")
                    tool_name = str(use.get("name") or use.get("orig_name") or "")
                    names[tool_id] = tool_name
                    row = base(end_time, model)
                    row.turn_type = "tool_use"
                    row.tool_name = tool_name
                    row.tool_use_id = tool_id
                    args = use.get("args")
                    arguments_text = args if isinstance(args, str) else compact_json(args)
                    orig = use.get("orig_name")
                    if orig and orig != tool_name:
                        arguments_text = "orig_name=%s %s" % (orig, arguments_text)
                    yield fill(row, arguments_text)
            tags = meta.get("message_meta_tags") or []
            if isinstance(tags, list) and "Compact" in tags:
                row = base(end_time, model)
                row.turn_type = "system"
                yield fill(row, "compact: conversation history summarised")

        seen_entries: set[str] = set()
        for entry in state["history"]:
            if isinstance(entry, dict):
                seen_entries.add(_entry_key(entry))
                yield from entry_rows(entry, seen_model)

        # Copies of the history inside the same blob: each checkpoint's
        # history_snapshot (kept through /checkpoint restore) and the main
        # conversation held aside in tangent mode. Only entries the history
        # no longer holds are yielded, marked recovered.
        seen_copy_model: dict[str, str] = {}
        for head, tail, timestamp, entries in self._history_copies(state):
            fresh = []
            for entry in entries:
                if isinstance(entry, dict) and _entry_key(entry) not in seen_entries:
                    seen_entries.add(_entry_key(entry))
                    fresh.append(entry)
            if not fresh:
                continue
            row = base(timestamp)
            row.turn_type = "system"
            yield fill(row, "%s recovered=%d entries%s" % (head, len(fresh), tail))
            for entry in fresh:
                for row in entry_rows(entry, seen_copy_model):
                    if row.turn_type != "system":
                        row.recovered = "yes"
                    yield row

        summary = state.get("latest_summary")
        if isinstance(summary, list) and summary:
            summary_meta = summary[1] if len(summary) > 1 and isinstance(summary[1], dict) else {}
            timestamp = to_utc(summary_meta.get("stream_end_timestamp_ms")) or to_utc(
                summary_meta.get("request_start_timestamp_ms")
            )
            row = base(timestamp, str(summary_meta.get("model_id") or ""))
            row.turn_type = "system"
            yield fill(
                row,
                "summary: %s"
                % (summary[0] if isinstance(summary[0], str) else compact_json(summary[0])),
            )

        pending = state.get("next_message")
        if isinstance(pending, dict):
            tag, body = _variant(pending.get("content"))
            prompt = body.get("prompt") if tag in ("Prompt", "CancelledToolUses") else None
            row = base(to_utc(pending.get("timestamp")))
            row.turn_type = "system"
            yield fill(
                row,
                "pending message (not sent): %s" % (prompt or compact_json(pending.get("content"))),
            )

        if problem:
            row = self.base_row(artifact)
            row.source_line = line
            row.session_id = session_id
            row.project_path = project
            row.turn_type = "system"
            row.text = "parser: truncated conversation, %d history entries recovered: %s" % (
                len(state["history"]),
                problem,
            )
            yield row

    @staticmethod
    def _assistant(row: Row, content, fill) -> Row:
        """Return `row` as the `assistant` row of a response's content."""
        row.turn_type = "assistant"
        return fill(row, content if isinstance(content, str) else compact_json(content))

    @staticmethod
    def _history_copies(state: dict) -> Iterator[tuple[str, str, str, list]]:
        """Yield (row text before and after the entry count, timestamp, entries) per history copy.

        Each checkpoint's `history_snapshot`, in checkpoint order, then
        `tangent_state.main_history`. The tangent row has no time of its own (`tangent_start_time`
        is not read).
        """
        manager = state.get("checkpoint_manager")
        checkpoints = manager.get("checkpoints") if isinstance(manager, dict) else None
        for checkpoint in checkpoints if isinstance(checkpoints, list) else []:
            if not isinstance(checkpoint, dict) or not isinstance(
                checkpoint.get("history_snapshot"), list
            ):
                continue
            head = "checkpoint snapshot: tag=%s" % (checkpoint.get("tag") or "")
            if checkpoint.get("tool_name"):
                head += " tool=%s" % checkpoint["tool_name"]
            tail = ": %s" % checkpoint["description"] if checkpoint.get("description") else ""
            yield head, tail, to_utc(checkpoint.get("timestamp")), checkpoint["history_snapshot"]
        tangent = state.get("tangent_state")
        if isinstance(tangent, dict) and isinstance(tangent.get("main_history"), list):
            head = "tangent mode: main conversation held aside,"
            yield head, "", "", tangent["main_history"]

    @staticmethod
    def _user(
        content, timestamp: str, model: str, names: dict[str, str], base, fill
    ) -> Iterator[Row]:
        """Yield the rows of a history entry's `user` content."""
        tag, body = _variant(content)
        prompt = body.get("prompt")
        if tag in ("Prompt", "CancelledToolUses") and prompt:
            row = base(timestamp, model)
            row.turn_type = "user"
            yield fill(row, prompt)
        if tag == "Prompt" and isinstance(prompt, str):
            match = DENY_RULE_PATTERN.match(prompt)
            if match:
                # the rejection reached the model as a plain prompt: it names
                # the tool but not the call
                row = base(timestamp, model)
                row.turn_type = "system"
                row.tool_name = match.group(1)
                yield fill(row, decision_text(prompt))
        if tag in ("ToolUseResults", "CancelledToolUses"):
            for result in body.get("tool_use_results") or []:
                if not isinstance(result, dict):
                    continue
                row = base(timestamp, model)
                row.turn_type = "tool_result"
                row.tool_use_id = str(result.get("tool_use_id") or "")
                row.tool_name = names.get(row.tool_use_id, "")
                text = result_text(result)
                if tag == "CancelledToolUses":
                    text = "cancelled %s" % text if text else "cancelled"
                yield fill(row, text)
                if tag == "CancelledToolUses":
                    decision = base(timestamp, model)
                    decision.turn_type = "system"
                    decision.tool_use_id = row.tool_use_id
                    decision.tool_name = row.tool_name
                    yield fill(decision, decision_text(prompt))

    @staticmethod
    def _first_cwd(state: dict) -> str:
        """Return the first `current_working_directory` recorded in the history, or `""`."""
        for entry in state.get("history") or []:
            try:
                cwd = entry["user"]["env_context"]["env_state"]["current_working_directory"]
            except (KeyError, TypeError):
                continue
            if cwd:
                return str(cwd)
        return ""
