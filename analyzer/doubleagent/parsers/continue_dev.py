"""Parse the sessions, events and logs of Continue, its IDE extensions and the `cn` CLI.

The module is not called `continue.py` because `continue` is a Python keyword. Reads, under
`.continue/`: `sessions/<sessionId>.json` and the `sessions/sessions.json` index (rows only for
what the index lost), `dev_data/[<schema>/]` `chatInteraction`, `toolUsage`, `editInteraction`,
`editOutcome` and `chatFeedback` (secondary) `.jsonl`, `input_history.json` (secondary),
`permissions.yaml` and `logs/cn*.log`.

Fields the parser depends on:

- session file: `sessionId`, `title`, `workspaceDirectory`, `mode`, `chatModelTitle`, `usage`,
  `history[]` items' `message` (`role`, `content`, `toolCalls[].id`, `function.name`,
  `function.arguments`, `toolCallId`), `toolCallStates[]` (`toolCallId`, `toolCall`, `status`,
  `output`), `promptLogs[].modelTitle` and `reasoning` (`text`, `startAt` epoch ms);
- index: `sessionId`, `title`, `dateCreated` (epoch ms as a string), `workspaceDirectory`;
- `dev_data` lines: `timestamp`, `sessionId`, `prompt`, `completion`, `modelTitle`,
  `modelName`, `feedback`, `filepath`, `accepted`, `succeeded`, `previousCodeLines`,
  `newCodeLines`, `toolCallId`, `functionName`, `functionParams`, `toolCallArgs`, `output`;
- `input_history.json`: `text`, `timestamp` (epoch ms); `permissions.yaml`: the `allow`, `ask`
  and `exclude` lists only; `cn.log`: the `[metadata] POST` and result lines, the storage-sync
  skip and `Added policy rule to permissions.yaml:` lines.

Session files carry no per-message timestamp: every row of a session inherits the index's
`dateCreated` (empty when the index lacks the session), except thinking rows at
`reasoning.startAt`; `source_line` is the position in `history[]`. A `cn.log` time is host-local
with no zone and is emitted as if it were UTC. Validated against continuedev/continue source at
5522c6f with a synthetic fixture; see `research/continue.md`.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterator

from ..errors import unreadable_message
from ..inputs import Artifact
from ..model import Row, compact
from ..textlines import SPACE_OR_TAB, split_lines
from ..timeutil import to_utc
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_bytes,
    file_mtime,
    iter_jsonl,
    iter_lines,
    line_text,
    setting_value,
    text_of,
    track_change,
)
from .cline_legacy import iter_array_at

SESSION_PATTERN = re.compile(r"^(?:.*/)?\.continue/sessions/([^/]+)\.json$")
DEV_DATA_PATTERN = re.compile(
    r"^(?:.*/)?\.continue/dev_data/(?:[^/]+/)?"
    r"(chatInteraction|toolUsage|editInteraction|editOutcome|chatFeedback)\.jsonl$"
)
INDEX_NAME = "sessions.json"
CN_LOG_PATTERN = re.compile(r"^\.continue/logs/cn[0-9]*\.log$")
INPUT_HISTORY_REL = ".continue/input_history.json"
PERMISSIONS_REL = ".continue/permissions.yaml"
# The secondary sources' window. Session rows carry the session's creation
# time, not the prompt's, and a rating can follow its reply by any time,
# so a history entry or a chatFeedback line pairs with its twin on text
# and scope whatever the time: ten years is in effect no bound.
ANY_TIME_SECONDS = 10 * 366 * 86400.0
# `cn review` worktree basename, `cn-review-<epoch ms>-<n>`
# (extensions/cli/src/commands/review/worktree.ts:15-20).
REVIEW_WORKTREE_PATTERN = re.compile(r"(?:^|[/\\])cn-review-[0-9]+-[0-9]+[/\\]?$")
# The CLI's denial texts in a canceled state's output, by `denialReason`
# (extensions/cli/src/stream/streamChatResponse.helpers.ts:519-527).
DENIAL_REASONS = {
    "Permission denied by user": "user",
    "Command blocked by security policy": "policy",
}
# The output item the extension gives a call its policy disables
# (gui/src/redux/thunks/evaluateToolPolicies.ts:93-117).
POLICY_VIOLATION = "Security Policy Violation"
POLICY_ADDED_PATTERN = re.compile(r"^Added policy rule to permissions\.yaml: (.*)$")
# permissions.yaml: the three list keys the CLI reads and writes
# (permissionsYamlLoader.ts:15-19, :162-171; policyWriter.ts:57-66), each
# empty (`[]`, `null`, `~` or nothing) or followed by `- <rule>` lines.
YAML_KEY_PATTERN = re.compile(r"^(allow|ask|exclude):[ \t]*(\[[ \t]*\]|null|~)?[ \t]*(?:#.*)?$")
YAML_ITEM_PATTERN = re.compile(r"^[ \t]*-[ \t]+(.*?)[ \t]*$")
LOG_LINE_PATTERN = re.compile(
    r"^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[([0-9a-fA-F]{8})\] \[[A-Za-z]+\]: (.*)$",
    re.ASCII,
)
METADATA_POST_PATTERN = re.compile(r"^\[metadata\] POST (\S+)(?: (\{.*\}))?$")
# `[metadata]` result lines (extensions/cli/src/util/metadata.ts:144-180):
# the status word, then the URL, which ends `/metadata`, followed on a
# `Failed` line by the response as JSON.
METADATA_RESULT_PATTERN = re.compile(
    r"^\[metadata\] (Success|Unexpected response|Auth required|Failed|Error)\b.* "
    r"(\S+/metadata)(?: \{.*\})?$"
)
AGENT_URL_PATTERN = re.compile(r"/agents/([^/]+)/metadata$")
STORAGE_SKIPPED = "Storage sync requested but no Continue API key"
HEADER_KEYS = ("sessionId", "title", "workspaceDirectory", "mode", "chatModelTitle")


def read_index(path) -> tuple[dict[str, dict], list]:
    """Return sessionId -> entry for the `sessions.json` index at `path`, and its errors.

    The errors, for `error_rows`, are an unreadable file, a file that is not a JSON array, an
    element that does not decode (with the text from it to the end of the file) and an element
    that is not an object with a `sessionId`. Entries keyed `session_id`, a pre-1.0 format the
    extension filters out, are skipped without one. An absent file or a symlink is an empty index
    with no error. Shared with the PearAI parser, whose fork keeps the same index.
    """
    out: dict[str, dict] = {}
    errors: list = []
    try:
        if not path.is_file() or path.is_symlink():
            return out, errors
    except OSError as error:
        errors.append((0, unreadable_message(path, error)))
        return out, errors
    content = file_bytes(path, errors)
    if content is None:
        return out, errors
    text = content.decode("utf-8", errors="replace")
    if text.startswith("\ufeff"):
        text = text[1:]
    end: list = []
    count = 0
    for index, element in iter_array_at(text, 0, errors, end):
        count = index
        if isinstance(element, dict) and element.get("sessionId"):
            out[str(element["sessionId"])] = element
        elif not (isinstance(element, dict) and element.get("session_id")):
            errors.append(
                (
                    index,
                    "unparseable record %d (not an object with a sessionId)" % index,
                    compact_json(element),
                )
            )
    if end and text[end[0] :].strip(" \t\r\n"):
        errors.append((count + 1, "unparseable data after the JSON array", ""))
    return out, errors


def yaml_scalar(text: str) -> str | None:
    """Return a YAML sequence item as a string, or None for anything but a scalar on its line.

    A double-quoted item is decoded as JSON, whose escapes it shares; in a single-quoted one `''`
    is one quote; a plain one loses its ` #` comment. A nested sequence or mapping, a flow
    collection, an alias, a block scalar or an unclosed quote is None.
    """
    if text.startswith('"'):
        match = re.match(r'^("(?:[^"\\]|\\.)*")[ \t]*(?:#.*)?$', text)
        if not match:
            return None
        try:
            value = json.loads(match.group(1))
        except ValueError:
            return None
        return value if isinstance(value, str) else None
    if text.startswith("'"):
        match = re.match(r"^'((?:[^']|'')*)'[ \t]*(?:#.*)?$", text)
        return match.group(1).replace("''", "'") if match else None
    if not text or text[0] in "[]{}&*!|>%@`-?:,#":
        return None
    value = re.sub(r"[ \t]+#.*$", "", text)
    # A mapping value starts at `:` before a space or tab.
    return None if re.search(r":[ \t]", value) or value.endswith(":") else value


def read_permissions(text: str) -> tuple[list[tuple[int, str, str]], str]:
    """Return the (line number, list, rule) rules of a `permissions.yaml`, and a problem.

    The problem is "", or, with no rules, why the file is not the flat shape the CLI writes
    (top-level `allow`, `ask` and `exclude` keys, each empty or followed by `- <rule>` lines),
    since a reader of that shape alone cannot tell what any other YAML construct grants.
    """
    entries: list[tuple[int, str, str]] = []
    current = ""
    for line_number, line in enumerate(split_lines(text), 1):
        if line_number == 1 and line.startswith("\ufeff"):
            line = line[1:]
        stripped = line.strip(SPACE_OR_TAB)
        if not stripped or stripped.startswith("#") or stripped == "---":
            continue
        key = YAML_KEY_PATTERN.match(line)
        if key:
            current = "" if key.group(2) else key.group(1)
            continue
        item = YAML_ITEM_PATTERN.match(line)
        value = yaml_scalar(item.group(1)) if item and current else None
        if value is None:
            return [], "unsupported YAML at line %d" % line_number
        entries.append((line_number, current, value))
    return entries, ""


def message_text(content) -> str:
    """Flatten a ChatMessage content: image parts become `[image]`."""
    if isinstance(content, list):
        content = [
            {"type": "text", "text": "[image]"}
            if isinstance(part, dict) and part.get("type") == "imageUrl"
            else part
            for part in content
        ]
    return text_of(content)


def output_text(output) -> str:
    """Text of a ContextItem[] tool output."""
    if not isinstance(output, list):
        return text_of(output)
    parts = []
    for item in output:
        if isinstance(item, dict):
            content = item.get("content")
            parts.append(
                content
                if isinstance(content, str)
                else compact_json(content)
                if content is not None
                else str(item.get("name") or "")
            )
        else:
            parts.append(str(item))
    return "\n".join(part for part in parts if part)


def state_decision(status: str, output, text: str) -> str:
    """Return the decision a `ToolCallState` records, as `permission decision:` text, or "".

    A `canceled` state gets `reason=user` or `reason=policy` when its output is one of the CLI's
    denial texts; an `errored` one with the extension's `Security Policy Violation` output is
    `errored reason=policy`.
    """
    if status == "canceled":
        reason = DENIAL_REASONS.get(text.strip())
        return "canceled reason=%s" % reason if reason else "canceled"
    violation = isinstance(output, list) and any(
        isinstance(item, dict) and item.get("name") == POLICY_VIOLATION for item in output
    )
    return "errored reason=policy" if status == "errored" and violation else ""


def load_session(text: str) -> tuple[dict, str]:
    """Parse a session file into its object and a problem message, "" when it parsed.

    On failure, recover the header keys and every `history[]` item that decodes before the damage.
    """
    try:
        document = json.loads(text)
        if isinstance(document, dict):
            return document, ""
        return {}, "session file is not a JSON object"
    except ValueError as error:
        first_error = str(error)
    decoder = json.JSONDecoder()
    start = text.find("{")
    if start >= 0:
        try:
            document, end = decoder.raw_decode(text, start)
            if isinstance(document, dict):
                return document, "trailing data after the session object at offset %d" % end
        except ValueError:
            pass
    out: dict = {}
    for key in HEADER_KEYS:
        match = re.search(r'"%s"[ \t\n\r]*:[ \t\n\r]*("(?:[^"\\]|\\.)*")' % key, text)
        if match:
            try:
                out[key] = json.loads(match.group(1))
            except ValueError:
                pass
    history: list[dict] = []
    match = re.search(r'"history"[ \t\n\r]*:[ \t\n\r]*\[', text)
    if match:
        position = match.end()
        while True:
            while position < len(text) and text[position] in " \t\r\n,":
                position += 1
            if position >= len(text) or text[position] == "]":
                break
            try:
                item, position = decoder.raw_decode(text, position)
            except ValueError:
                break
            if isinstance(item, dict):
                history.append(item)
    out["history"] = history
    return out, "session file did not parse (%s); recovered %d history item(s)" % (
        first_error,
        len(history),
    )


class ContinueParser(Parser):
    """Parse Continue's session files, `dev_data` events, CLI state files and CLI logs."""

    agent = "continue"
    name = "continue"

    def __init__(self) -> None:
        """Start with an empty cache of session indexes."""
        self._index_cache: dict[str, tuple[dict[str, dict], list]] = {}

    has_secondary = True

    def wants(self, artifact: Artifact) -> bool:
        # sessions.json too, for the rows that say what the index lost
        """Return whether the artifact is one of the files this parser reads."""
        return bool(
            SESSION_PATTERN.match(artifact.rel)
            or DEV_DATA_PATTERN.match(artifact.rel)
            or CN_LOG_PATTERN.match(artifact.rel)
            or artifact.rel in (INPUT_HISTORY_REL, PERMISSIONS_REL)
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return whether the artifact is the CLI's prompt history or a `chatFeedback` file.

        Both repeat a prompt of a session or a `chatInteraction` line.
        """
        if artifact.rel == INPUT_HISTORY_REL:
            return True
        match = DEV_DATA_PATTERN.match(artifact.rel)
        return bool(match and match.group(1) == "chatFeedback")

    def secondary_tolerance(self, artifact: Artifact) -> float:
        """Return `ANY_TIME_SECONDS`: a secondary row's time says nothing of its twin's.

        A history entry is stamped when the prompt was submitted while the session's rows carry its
        creation time; a rating is stamped when it was clicked, any time after the reply.
        """
        return ANY_TIME_SECONDS

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one artifact, by the file it is."""
        if CN_LOG_PATTERN.match(artifact.rel):
            yield from self._parse_cn_log(artifact)
            return
        if artifact.rel == INPUT_HISTORY_REL:
            yield from self._parse_input_history(artifact, opts)
            return
        if artifact.rel == PERMISSIONS_REL:
            yield from self._parse_permissions(artifact)
            return
        match = DEV_DATA_PATTERN.match(artifact.rel)
        if match and match.group(1) in ("chatInteraction", "chatFeedback"):
            yield from self._parse_chat_interaction(artifact, opts, match.group(1))
        elif match and match.group(1) == "editInteraction":
            yield from self._parse_edit_interaction(artifact, opts)
        elif match and match.group(1) == "editOutcome":
            yield from self._parse_edit_outcome(artifact)
        elif match:
            yield from self._parse_tool_usage(artifact, opts)
        elif artifact.disk_path.name == INDEX_NAME:
            yield from error_rows(self, artifact, self.index_errors(artifact.disk_path))
        else:
            yield from self._parse_session(artifact, opts)

    # ---- sessions.json --------------------------------------------------------

    def _read_index(self, path) -> tuple[dict[str, dict], list]:
        """Return `read_index` of `path`, cached per file for the session files that share it."""
        key = str(path)
        if key not in self._index_cache:
            self._index_cache[key] = read_index(path)
        return self._index_cache[key]

    def _index(self, sessions_dir) -> dict[str, dict]:
        """Return sessionId -> `sessions.json` entry for the index beside the session files.

        The result is empty when the index is absent or unreadable; what it lost is
        reported once, by `index_errors` on the index's own artifact.
        """
        return self._read_index(sessions_dir / INDEX_NAME)[0]

    def index_errors(self, path) -> list:
        """Return the errors `read_index` found in the index at `path`, a fresh list each call."""
        return list(self._read_index(path)[1])

    # ---- session files --------------------------------------------------------

    def _parse_session(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        match = SESSION_PATTERN.match(artifact.rel)
        assert match is not None  # wants() accepted only matching paths
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors, match.group(1))
            return
        raw = content.decode("utf-8", errors="replace")
        session, problem = load_session(raw)
        session_id = session.get("sessionId")
        if not session_id:
            session_id = match.group(1)
        session_id = str(session_id)
        entry = self._index(artifact.disk_path.parent).get(session_id, {})
        timestamp = to_utc(entry.get("dateCreated"))
        project = str(session.get("workspaceDirectory") or entry.get("workspaceDirectory") or "")
        session_model = str(session.get("chatModelTitle") or "")
        history = session.get("history")
        if not isinstance(history, list):
            history = []

        def base(position: int, model: str = "") -> Row:
            row = self.base_row(artifact)
            row.source_line = position
            row.timestamp_utc = timestamp
            row.session_id = session_id
            row.project_path = project
            row.model = model
            return row

        row = base(0, session_model)
        row.turn_type = "system"
        usage = session.get("usage")
        if not isinstance(usage, dict):
            usage = {}
        bits = ["session start: %s" % (session.get("title") or entry.get("title") or "")]
        if session.get("mode"):
            bits.append("mode=%s" % session["mode"])
        bits.append("items=%d" % len(history))
        for field in ("promptTokens", "completionTokens", "totalCost"):
            if usage.get(field) is not None:
                bits.append("%s=%s" % (field, usage[field]))
        if not timestamp:
            bits.append("dateCreated unknown")
        row.text = compact(" ".join(bits))
        yield row
        models: dict = {}
        change = track_change("model", models, session_id, session_model)
        if change:
            row = base(0, session_model)
            row.turn_type = "system"
            row.text = change
            yield row
        mode = session.get("mode")
        if isinstance(mode, str) and mode:
            # the extension's mode at the last save; `plan` limits the tools
            row = base(0, session_model)
            row.turn_type = "system"
            row.text = "permission mode: " + mode
            yield row
        if REVIEW_WORKTREE_PATTERN.search(project):
            row = base(0, session_model)
            row.turn_type = "system"
            row.text = "scratch directory: %s (cn review worktree)" % project
            yield row

        # Tool results normally arrive as a later `tool` message; a call that
        # never got one (canceled, errored) keeps its output in toolCallStates.
        answered = set()
        for item in history:
            message = item.get("message") if isinstance(item, dict) else None
            if (
                isinstance(message, dict)
                and message.get("role") == "tool"
                and message.get("toolCallId")
            ):
                answered.add(str(message["toolCallId"]))
        names: dict[str, str] = {}

        for i, item in enumerate(history, 1):
            if not isinstance(item, dict):
                continue
            message = item.get("message")
            if not isinstance(message, dict):
                continue
            role = message.get("role")
            model = session_model
            logs = item.get("promptLogs")
            if isinstance(logs, list):
                for prompt_log in logs:
                    if isinstance(prompt_log, dict) and prompt_log.get("modelTitle"):
                        model = str(prompt_log["modelTitle"])
                        change = track_change("model", models, session_id, model)
                        if change:
                            row = base(i, model)
                            row.turn_type = "system"
                            row.text = change
                            yield row
            reasoning = item.get("reasoning")
            if opts.include_thinking and isinstance(reasoning, dict) and reasoning.get("text"):
                row = base(i, model)
                row.turn_type = "thinking"
                row.timestamp_utc = to_utc(reasoning.get("startAt")) or timestamp
                yield self._fill(row, reasoning.get("text"), opts)
            text = message_text(message.get("content"))
            if role == "user":
                if text:
                    row = base(i)
                    row.turn_type = "user"
                    yield self._fill(row, text, opts)
            elif role == "assistant":
                if text:
                    row = base(i, model)
                    row.turn_type = "assistant"
                    yield self._fill(row, text, opts)
                calls = message.get("toolCalls")
                states = item.get("toolCallStates")
                states = (
                    [item_state for item_state in states if isinstance(item_state, dict)]
                    if isinstance(states, list)
                    else []
                )
                if not isinstance(calls, list) or not calls:
                    calls = [
                        item_state.get("toolCall")
                        for item_state in states
                        if isinstance(item_state.get("toolCall"), dict)
                    ]
                for call in calls:
                    if not isinstance(call, dict):
                        continue
                    function = call.get("function")
                    if not isinstance(function, dict):
                        function = {}
                    call_id = str(call.get("id") or "")
                    name = str(function.get("name") or "")
                    names[call_id] = name
                    arguments = function.get("arguments")
                    row = base(i, model)
                    row.turn_type = "tool_use"
                    row.tool_name = name
                    row.tool_use_id = call_id
                    yield self._fill(
                        row,
                        arguments
                        if isinstance(arguments, str)
                        else compact_json(arguments)
                        if arguments is not None
                        else "",
                        opts,
                    )
                for state in states:
                    call_id = str(state.get("toolCallId") or "")
                    if not call_id:
                        continue
                    output = output_text(state.get("output"))
                    status = str(state.get("status") or "")
                    if call_id not in answered and (
                        output or status not in ("", "done", "generated", "generating", "calling")
                    ):
                        row = base(i, model)
                        row.turn_type = "tool_result"
                        row.tool_use_id = call_id
                        row.tool_name = names.get(call_id, "")
                        yield self._fill(
                            row,
                            "[%s] %s" % (status, output) if status and status != "done" else output,
                            opts,
                        )
                    decision = state_decision(status, state.get("output"), output)
                    if decision:
                        row = base(i, model)
                        row.turn_type = "system"
                        row.tool_use_id = call_id
                        row.tool_name = names.get(call_id, "")
                        row.text = "permission decision: " + decision
                        yield row
            elif role == "thinking":
                if opts.include_thinking and text:
                    row = base(i, model)
                    row.turn_type = "thinking"
                    yield self._fill(row, text, opts)
            elif role == "tool":
                call_id = str(message.get("toolCallId") or "")
                row = base(i)
                row.turn_type = "tool_result"
                row.tool_use_id = call_id
                row.tool_name = names.get(call_id, "")
                yield self._fill(row, text, opts)
            elif text:
                row = base(i)
                row.turn_type = "system"
                yield self._fill(
                    row, "%s: %s" % (role or "unknown", text) if role != "system" else text, opts
                )

        if problem:
            row = base(0)
            row.turn_type = "system"
            row.text = "parser: " + problem
            yield row

    # ---- dev_data -------------------------------------------------------------

    def _dev_index(self, artifact: Artifact) -> dict[str, dict]:
        # .continue/dev_data/<schema>/x.jsonl or .continue/dev_data/x.jsonl
        directory = artifact.disk_path.parent
        base = directory.parent if directory.parent.name == ".continue" else directory.parent.parent
        return self._index(base / "sessions")

    def _parse_chat_interaction(
        self, artifact: Artifact, opts: Options, event: str = "chatInteraction"
    ) -> Iterator[Row]:
        """Yield the `user` and `assistant` rows of a `chatInteraction` or `chatFeedback` line.

        A `chatFeedback` line adds `chat feedback: feedback=<true|false|-> model=<modelTitle|->`.
        """
        errors: list = []
        index = self._dev_index(artifact)
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            session_id = str(record.get("sessionId") or "")
            entry = index.get(session_id, {})
            model = str(record.get("modelTitle") or record.get("modelName") or "")

            def row_for(
                turn: str,
                record=record,
                line_number=line_number,
                session_id=session_id,
                entry=entry,
            ) -> Row:
                row = self.base_row(artifact)
                row.source_line = line_number
                row.timestamp_utc = to_utc(record.get("timestamp"))
                row.session_id = session_id
                row.project_path = str(entry.get("workspaceDirectory") or "")
                row.turn_type = turn
                return row

            for turn, key in (("user", "prompt"), ("assistant", "completion")):
                text = record.get(key)
                if not text:
                    continue
                row = row_for(turn)
                if turn == "assistant":
                    row.model = model
                yield self._fill(row, text if isinstance(text, str) else compact_json(text), opts)
            if event == "chatFeedback":
                row = row_for("system")
                row.model = model
                feedback = record.get("feedback")
                row.text = "chat feedback: feedback=%s model=%s" % (
                    "-" if feedback is None else setting_value(feedback),
                    model or "-",
                )
                yield row
        yield from error_rows(self, artifact, errors)

    def _parse_edit_interaction(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield an edit request: an `edit interaction:` row, the prompt and the edited file."""
        errors: list = []
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            model = str(record.get("modelTitle") or record.get("modelName") or "")
            filepath = record.get("filepath")
            parts = [("system", "edit interaction: file=%s" % (filepath or "-"))]
            parts += [
                (turn_type, record.get(key))
                for turn_type, key in (("user", "prompt"), ("assistant", "completion"))
            ]
            for turn, text in parts:
                if not text:
                    continue
                row = self.base_row(artifact)
                row.source_line = line_number
                row.timestamp_utc = to_utc(record.get("timestamp"))
                row.turn_type = turn
                row.model = "" if turn == "user" else model
                yield self._fill(row, text if isinstance(text, str) else compact_json(text), opts)
        yield from error_rows(self, artifact, errors)

    def _parse_edit_outcome(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the user's accept or reject of an edit as a `permission decision: edit` row.

        The prompt, completion and code are not put in rows: the `editInteraction` line and the
        session already hold the request.
        """
        errors: list = []
        for line_number, record in iter_jsonl(artifact.disk_path, errors):

            def field_value(key: str, record=record) -> str:
                value = record.get(key)
                return "-" if value is None or value == "" else setting_value(value)

            row = self.base_row(artifact)
            row.source_line = line_number
            row.timestamp_utc = to_utc(record.get("timestamp"))
            row.turn_type = "system"
            row.model = str(record.get("modelTitle") or record.get("modelName") or "")
            row.text = "permission decision: edit accepted=%s file=%s lines %s -> %s" % (
                field_value("accepted"),
                field_value("filepath"),
                field_value("previousCodeLines"),
                field_value("newCodeLines"),
            )
            yield row
        yield from error_rows(self, artifact, errors)

    # ---- CLI state files -----------------------------------------------------

    def _parse_input_history(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield one `user` row per `{text, timestamp}` entry of the CLI's prompt history.

        The rows carry no session or project, since the file names none;
        `source_line` is the entry's 1-based position.
        """
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors)
            return
        text = content.decode("utf-8", errors="replace")
        if text.startswith("﻿"):
            text = text[1:]
        end: list = []
        count = 0
        for position, entry in iter_array_at(text, 0, errors, end):
            count = position
            prompt = entry.get("text") if isinstance(entry, dict) else None
            if not isinstance(entry, dict) or not isinstance(prompt, str):
                errors.append(
                    (
                        position,
                        "unparseable record %d (not an object with a text)" % position,
                        compact_json(entry),
                    )
                )
                continue
            row = self.base_row(artifact)
            row.source_line = position
            row.timestamp_utc = to_utc(entry.get("timestamp"))
            row.turn_type = "user"
            yield self._fill(row, prompt, opts)
        if end and text[end[0] :].strip(" \t\r\n"):
            errors.append((count + 1, "unparseable data after the JSON array", ""))
        yield from error_rows(self, artifact, errors)

    def _parse_permissions(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `permission setting:` row per rule of `permissions.yaml`, at the file's mtime.

        A file outside the shape `read_permissions` reads is one `parser:` row.
        """
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors)
            return
        text = content.decode("utf-8", errors="replace")
        entries, problem = read_permissions(text)
        if problem:
            message = "%s (only allow, ask and exclude lists of strings are read)" % problem
            yield from error_rows(self, artifact, [(0, message, "")])
            return
        timestamp = file_mtime(artifact)
        for line_number, key, rule in entries:
            row = self.base_row(artifact)
            row.source_line = line_number
            row.timestamp_utc = timestamp
            row.turn_type = "system"
            row.text = compact(
                "permission setting: %s=%s (from %s, file mtime)" % (key, rule, artifact.rel)
            )
            yield row

    def _parse_tool_usage(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            timestamp = to_utc(record.get("timestamp"))
            call_id = str(record.get("toolCallId") or "")
            name = str(record.get("functionName") or "")
            row = self.base_row(artifact)
            row.source_line = line_number
            row.timestamp_utc = timestamp
            row.turn_type = "tool_use"
            row.tool_name = name
            row.tool_use_id = call_id
            arguments = record.get("toolCallArgs")
            if not arguments and record.get("functionParams") is not None:
                arguments = compact_json(record.get("functionParams"))
            yield self._fill(
                row,
                arguments
                if isinstance(arguments, str)
                else compact_json(arguments)
                if arguments is not None
                else "",
                opts,
            )
            row = self.base_row(artifact)
            row.source_line = line_number
            row.timestamp_utc = timestamp
            row.turn_type = "tool_result"
            row.tool_name = name
            row.tool_use_id = call_id
            yield self._fill(
                row,
                "accepted=%s succeeded=%s %s"
                % (
                    compact_json(record.get("accepted")),
                    compact_json(record.get("succeeded")),
                    output_text(record.get("output")),
                ),
                opts,
            )
            if "accepted" in record:
                row = self.base_row(artifact)
                row.source_line = line_number
                row.timestamp_utc = timestamp
                row.turn_type = "system"
                row.tool_name = name
                row.tool_use_id = call_id
                row.text = "permission decision: accepted=%s" % setting_value(record["accepted"])
                yield row
        yield from error_rows(self, artifact, errors)

    # ---- logs/cn.log ---------------------------------------------------------

    def _parse_cn_log(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the `agent metadata posted:` and `storage upload requested` rows of a `cn` log."""
        source = "logs/" + artifact.disk_path.name
        # Each post as [process id, url, fields JSON, status, row]; a post
        # waits in `pending` for the result line of its process and URL.
        posts: list[list] = []
        pending: list[list] = []
        rows: list[Row] = []
        errors: list = []
        for line_number, raw in iter_lines(artifact.disk_path, errors):
            match = LOG_LINE_PATTERN.match(line_text(raw))
            if not match:
                continue
            when, process_id, message = match.groups()
            if STORAGE_SKIPPED in message:
                text = "storage upload requested, skipped (from %s, log line time, %s)"
                rows.append(
                    self._log_row(artifact, line_number, when, text % (source, "host-local"))
                )
                continue
            added = POLICY_ADDED_PATTERN.match(message)
            if added:
                text = "permission decision: allow always rule=%s (from %s, %s)" % (
                    added.group(1),
                    source,
                    "log line time, host-local",
                )
                rows.append(self._log_row(artifact, line_number, when, text))
                continue
            post = METADATA_POST_PATTERN.match(message)
            if post:
                row = self._log_row(artifact, line_number, when, "")
                posts.append([process_id, post.group(1), post.group(2), "-", row])
                pending.append(posts[-1])
                rows.append(row)
                continue
            result = METADATA_RESULT_PATTERN.match(message)
            if result:
                self._settle(pending, process_id, result.group(2), result.group(1))
        for _process_id, url, fields, status, row in posts:
            row.text = self._post_text(url, fields, status, source)
        yield from rows
        yield from error_rows(self, artifact, errors)

    @staticmethod
    def _settle(pending: list[list], process_id: str, url: str, status: str) -> None:
        """Give the oldest pending post of process `pid` to `url` its status."""
        for entry in pending:
            if entry[0] == process_id and entry[1] == url:
                entry[3] = status
                pending.remove(entry)
                return

    def _log_row(self, artifact: Artifact, line_number: int, when: str, text: str) -> Row:
        row = self.base_row(artifact)
        row.source_line = line_number
        row.timestamp_utc = to_utc(when)
        row.turn_type = "system"
        row.text = text
        return row

    @staticmethod
    def _post_text(url: str, fields: str | None, status: str, source: str) -> str:
        try:
            document = json.loads(fields) if fields else {}
        except ValueError:
            document = {}
        document = document if isinstance(document, dict) else {}
        agent = document.get("agentId")
        if not isinstance(agent, str) or not agent:
            match = AGENT_URL_PATTERN.search(url)
            agent = match.group(1) if match else "-"
        keys = document.get("metadataKeys")
        keys = ",".join(str(key) for key in keys) if isinstance(keys, list) and keys else "-"
        return "agent metadata posted: agent %s keys %s %s (from %s, log line time, host-local)" % (
            agent,
            keys,
            status,
            source,
        )

    @staticmethod
    def _fill(row: Row, text, opts: Options) -> Row:
        row.text = compact(text)
        return row
