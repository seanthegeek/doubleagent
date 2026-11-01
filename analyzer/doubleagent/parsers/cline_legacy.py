"""Shared reader of the per-task layout Cline introduced and Roo Code inherited.

Subclassed by the `cline`, `roo-code` and `pearai` parsers; others reuse its JSON readers.
Reads, under a storage root (the editor's `User/globalStorage/<extension id>`, Cline's
`~/.cline/data`, Roo's `~/.vscode-mock/global-storage`), `tasks/<taskId>/`:

* `ui_messages.json`, the preferred transcript, every record timestamped: a JSON array of
  `ClineMessage` `{ts, type, ask, say, text, reasoning, images, partial, modelInfo}`,
  `text` being JSON for `tool`, `api_req_started`, `use_mcp_server`, `followup`,
  `plan_mode_respond`, `browser_action` and `browser_action_result`, plus Roo's
  `contextCondense.summary` and `batchFiles[].path`.
* `api_conversation_history.json` (Roo's older `claude_messages.json`), read only when no
  `ui_messages.json` sits beside it: Anthropic `MessageParam` `{role, content}` with
  `text`, `tool_use`, `tool_result`, `thinking`, `image` and `file` blocks, plus Roo's `ts`,
  `isSummary` and `type: "reasoning"` records. Records without `ts` take the session
  start: a millisecond task id, else the earliest `task_metadata.json` time, else the task
  index `ts`.
* `task_metadata.json`: `files_in_context[]` (`path`, `record_state`, `record_source`,
  `*_date`), `model_usage[]` (`ts`, `model_id`, `model_provider_id`, `mode`) and
  `environment_history[]`, one `system` row per dated event and `model:` and
  `permission mode:` rows on a change.
* The task index `HistoryItem` (`id`, `ts`, `task`, token counts, `totalCost`, `size`, the
  project key and Cline's `modelId`), found by each subclass's `history_item()`.

A record that does not decode stops the array walk and becomes one `parser: unparseable
record <N> (<reason>): <raw>` row holding the text from that record to the end of the file.
Validated against source (cline/cline at 39ff2359, RooCodeInc/Roo-Code at b867ec91). See
`research/cline.md` and `research/roo-code.md`.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterator
from pathlib import Path

from ..errors import unreadable_message
from ..inputs import Artifact
from ..model import Row, compact
from ..timeutil import to_utc
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_bytes,
    tail_text,
    text_of,
    track_change,
)
from .claude_code import tool_summary

UI_FILE = "ui_messages.json"
API_FILES = ("api_conversation_history.json", "claude_messages.json")
METADATA_FILE = "task_metadata.json"

USER_KINDS = ("task", "user_feedback", "user_feedback_diff")
ASSISTANT_KINDS = (
    "text",
    "completion_result",
    "plan_completion_result",
    "followup",
    "plan_mode_respond",
    "act_mode_respond",
)
TOOL_USE_KINDS = (
    "tool",
    "command",
    "use_mcp_server",
    "browser_action_launch",
    "browser_action",
    "new_task",
    "use_subagents",
    "condense",
    "summarize_task",
)
TOOL_RESULT_KINDS = (
    "command_output",
    "mcp_server_response",
    "browser_action_result",
    "codebase_search_result",
    "subtask_result",
)
TOOL_NAMES = {
    "command": "execute_command",
    "browser_action_launch": "browser_action",
    "browser_action": "browser_action",
}


# ---- tolerant JSON readers ----------------------------------------------------

_WHITESPACE = " \t\r\n"


def _skip_whitespace(text: str, offset: int) -> int:
    while offset < len(text) and text[offset] in _WHITESPACE:
        offset += 1
    return offset


def iter_array_at(
    text: str, offset: int, errors: list, end: list | None = None
) -> Iterator[tuple[int, object]]:
    """Yield (1-based position, element) from the JSON array at `text[offset]`.

    A truncated or corrupt element stops the walk and is recorded in `errors` as
    (position, `unparseable record <position> (<reason>)`, the text from the element's
    first character to the end, less a trailing line terminator); the elements before it
    are kept. A structural error (not an array, no closing bracket) is (position, message,
    ""). When `end` is given, the index after the closing bracket is appended to it.
    """
    decoder = json.JSONDecoder()
    offset = _skip_whitespace(text, offset)
    if offset >= len(text) or text[offset] != "[":
        errors.append((1, "not a JSON array", ""))
        return
    offset = _skip_whitespace(text, offset + 1)
    if offset < len(text) and text[offset] == "]":
        if end is not None:
            end.append(offset + 1)
        return
    position = 0
    while True:
        position += 1
        start = _skip_whitespace(text, offset)
        try:
            value, offset = decoder.raw_decode(text, start)
        except ValueError as error:
            errors.append(
                (position, "unparseable record %d (%s)" % (position, error), tail_text(text, start))
            )
            return
        yield position, value
        offset = _skip_whitespace(text, offset)
        if offset < len(text) and text[offset] == ",":
            offset += 1
            continue
        if offset < len(text) and text[offset] == "]":
            if end is not None:
                end.append(offset + 1)
            return
        errors.append((position + 1, "array cut off after record %d" % position, ""))
        return


def read_text(path: Path, errors: list) -> str | None:
    """Return a file's text without a BOM, or None.

    A file that cannot be read gives None after its `file_bytes` entry in
    `errors`. A symlink, a missing path and anything but a regular file give
    None silently: the walk offers only regular files, so only a side file
    the parser looks for beside an artifact meets them, and an absent side
    file is optional.
    """
    file_path = Path(path)
    try:
        if file_path.is_symlink() or not file_path.is_file():
            return None
    except OSError as error:
        errors.append((0, unreadable_message(file_path, error)))
        return None
    data = file_bytes(file_path, errors)
    if data is None:
        return None
    text = data.decode("utf-8", errors="replace")
    return text[1:] if text.startswith("\ufeff") else text


def iter_json_array(path: Path, errors: list) -> Iterator[tuple[int, object]]:
    """Yield the elements of a file holding one JSON array.

    A file that cannot be read, a truncation and data appended after the
    array are reported through `errors`.
    """
    text = read_text(path, errors)
    if text is None:
        return
    end: list = []
    count = 0
    for position, value in iter_array_at(text, 0, errors, end):
        count = position
        yield position, value
    if end and text[end[0] :].strip(_WHITESPACE):
        errors.append((count + 1, "unparseable data after the JSON array", ""))


def load_json(path: Path, errors: list):
    """Return a file parsed as JSON, or None.

    Only a file that cannot be read adds an entry to `errors` (`read_text`);
    for text that is not JSON the caller writes its own message.
    """
    text = read_text(path, errors)
    if text is None:
        return None
    try:
        return json.loads(text)
    except ValueError:
        return None


def loads_or_none(text) -> dict | None:
    """Return `text` parsed as a JSON object, or None."""
    if not isinstance(text, str) or not text.lstrip().startswith("{"):
        return None
    try:
        value = json.loads(text)
    except ValueError:
        return None
    return value if isinstance(value, dict) else None


def _join(*parts) -> str:
    return " | ".join(str(part) for part in parts if part not in (None, "", [], {}))


def _fmt_counts(record: dict, keys) -> str:
    return " ".join("%s=%s" % (k, record[k]) for k in keys if record.get(k) not in (None, ""))


# ---- task context ---------------------------------------------------------------


class TaskContext:
    """Hold what one task directory knows about itself.

    That is its id, project, models and a start time for records that carry none.
    """

    def __init__(self, task_id: str, item: dict | None, metadata: dict | None, project_key: str):
        """Read the task's project and models from its index item and metadata."""
        self.task_id = task_id
        item = item if isinstance(item, dict) else {}
        metadata = metadata if isinstance(metadata, dict) else {}
        self.project_path = str(item.get(project_key) or "")
        self.default_model = str(item.get("modelId") or "")
        usage = metadata.get("model_usage")
        self.model_usage: list[tuple[float, str]] = sorted(
            (float(usage_entry.get("ts") or 0), str(usage_entry.get("model_id") or ""))
            for usage_entry in (usage if isinstance(usage, list) else [])
            if isinstance(usage_entry, dict) and usage_entry.get("model_id")
        )
        self.start = self._start(task_id, item, metadata)

    @staticmethod
    def _start(task_id: str, item: dict, metadata: dict) -> str:
        if re.match(r"^\d{12,14}$", task_id):
            return to_utc(int(task_id))
        stamps = []
        for key in ("model_usage", "environment_history"):
            for entry in metadata.get(key) or []:
                if isinstance(entry, dict) and isinstance(entry.get("ts"), (int, float)):
                    stamps.append(entry["ts"])
        if stamps:
            return to_utc(min(stamps))
        return to_utc(item.get("ts"))

    def model_at(self, timestamp) -> str:
        """Return the model in use at an epoch-millisecond time, else the task's default."""
        if self.model_usage:
            chosen = self.model_usage[0][1]
            if isinstance(timestamp, (int, float)):
                for used_at, model_id in self.model_usage:
                    if used_at <= timestamp:
                        chosen = model_id
            return chosen
        return self.default_model


# ---- parser -----------------------------------------------------------------------


class LegacyTaskParser(Parser):
    """Parse the per-task layout; the base of the Cline-family parsers.

    Subclasses set ROOT (a regex for the storage root, home-relative) and PROJECT_KEY,
    and implement history_item().
    """

    ROOT = ""
    PROJECT_KEY = ""

    def __init__(self) -> None:
        """Compile the task-file pattern under ROOT and start an empty file cache."""
        self._task_rx = re.compile(
            r"^%s/tasks/([^/]+)/(ui_messages|api_conversation_history|claude_messages|task_metadata)\.json$"
            % self.ROOT,
            re.I,
        )
        self._cache: dict[tuple, tuple[object, list]] = {}

    def history_item(self, task_dir: Path, root: Path, task_id: str) -> dict | None:
        """Return the task index entry of a task, or None; subclasses implement it."""
        raise NotImplementedError

    def cached_json(self, path: Path, errors: list):
        """Return a file parsed as JSON, cached by path, modification time and size.

        A missing file gives None silently. A file that cannot be read adds
        its entry to `errors` on every call, the cached ones included.
        """
        try:
            file_stat = Path(path).lstat()
        except (FileNotFoundError, NotADirectoryError):
            return None
        except OSError as error:
            errors.append((0, unreadable_message(path, error)))
            return None
        key = (str(path), file_stat.st_mtime_ns, file_stat.st_size)
        if key not in self._cache:
            read_errors: list = []
            self._cache[key] = (load_json(path, read_errors), read_errors)
        value, read_errors = self._cache[key]
        errors.extend(read_errors)
        return value

    def api_content_rows(
        self, message: dict, position: int, base_turn: str, include_thinking: bool
    ) -> Iterator[tuple[str, str, str, str]]:
        """Yield the rows of one API history message from its native content blocks.

        PearAI's Roo fork overrides it for XML tool calls.
        """
        return content_rows(message.get("content"), base_turn, include_thinking)

    def task_match(self, artifact: Artifact):
        """Return the match of a task file under ROOT, or None."""
        return self._task_rx.match(artifact.rel)

    def context(self, artifact: Artifact) -> TaskContext:
        """Return the `TaskContext` of the task directory holding `artifact`."""
        task_dir = artifact.disk_path.parent
        root = task_dir.parent.parent
        task_id = task_dir.name
        item = self.history_item(task_dir, root, task_id)
        # A pre-scan: the same parser reports task_metadata.json when it parses the file.
        metadata = load_json(task_dir / METADATA_FILE, [])
        return TaskContext(task_id, item, metadata, self.PROJECT_KEY)

    def parse_task_file(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one task file."""
        name = artifact.disk_path.name.lower()
        context = self.context(artifact)
        if name == UI_FILE:
            yield from self._parse_ui(artifact, context, opts)
        elif name in API_FILES:
            if (artifact.disk_path.parent / UI_FILE).is_file():
                return
            yield from self._parse_api(artifact, context, opts)
        elif name == METADATA_FILE:
            yield from self._parse_metadata(artifact, context, opts)

    def _row(
        self,
        artifact: Artifact,
        context: TaskContext,
        position: int,
        timestamp: str,
        model: str = "",
    ) -> Row:
        row = self.base_row(artifact)
        row.source_line = position
        row.timestamp_utc = timestamp
        row.session_id = context.task_id
        row.project_path = context.project_path
        row.model = model
        return row

    # -- ui_messages.json

    def _parse_ui(self, artifact: Artifact, context: TaskContext, opts: Options) -> Iterator[Row]:
        errors: list = []
        model = ""
        last_tool_id = ""
        for position, message in iter_json_array(artifact.disk_path, errors):
            if not isinstance(message, dict):
                continue
            info = message.get("modelInfo")
            if isinstance(info, dict) and info.get("modelId"):
                model = str(info["modelId"])
            timestamp = message.get("ts")
            row_model = model or context.model_at(timestamp)
            kind = message.get("say") if message.get("type") == "say" else message.get("ask")
            kind = str(
                kind or message.get("say") or message.get("ask") or message.get("type") or ""
            )
            reasoning = message.get("reasoning")
            if (
                opts.include_thinking
                and isinstance(reasoning, str)
                and reasoning
                and kind != "reasoning"
            ):
                row = self._row(artifact, context, position, to_utc(timestamp), row_model)
                row.turn_type = "thinking"
                row.text = compact(reasoning)
                yield row
            mapped = map_ui(kind, message, position == 1)
            if mapped is None:
                continue
            turn, text, tool_name = mapped
            if turn == "thinking" and not opts.include_thinking:
                continue
            row = self._row(artifact, context, position, to_utc(timestamp), row_model)
            row.turn_type = turn
            if turn == "tool_use":
                last_tool_id = str(timestamp if timestamp is not None else position)
                row.tool_use_id = last_tool_id
                row.tool_name = tool_name
            elif turn == "tool_result":
                row.tool_use_id = last_tool_id
                row.tool_name = tool_name
            if message.get("partial"):
                text = text + " [partial]" if text else "[partial]"
            row.text = compact(text)
            yield row
        yield from error_rows(self, artifact, errors, context.task_id, context.project_path)

    # -- api_conversation_history.json

    def _parse_api(self, artifact: Artifact, context: TaskContext, opts: Options) -> Iterator[Row]:
        errors: list = []
        for position, message in iter_json_array(artifact.disk_path, errors):
            if not isinstance(message, dict):
                continue
            timestamp = to_utc(message.get("ts")) or context.start
            model = context.model_at(message.get("ts"))
            role = message.get("role")
            base_turn = (
                "user" if role == "user" else "assistant" if role == "assistant" else "system"
            )
            if message.get("type") == "reasoning":
                if opts.include_thinking:
                    row = self._row(artifact, context, position, timestamp, model)
                    row.turn_type = "thinking"
                    row.text = compact(
                        text_of(message.get("summary"))
                        or message.get("text")
                        or message.get("reasoning_content")
                        or "[encrypted reasoning]",
                    )
                    yield row
                continue
            if message.get("isSummary") or message.get("isTruncationMarker"):
                row = self._row(artifact, context, position, timestamp, model)
                row.turn_type = "system"
                label = "summary" if message.get("isSummary") else "truncation marker"
                row.text = compact("%s: %s" % (label, text_of(message.get("content"))))
                yield row
                continue
            if (
                opts.include_thinking
                and isinstance(message.get("reasoning_content"), str)
                and message["reasoning_content"]
            ):
                row = self._row(artifact, context, position, timestamp, model)
                row.turn_type = "thinking"
                row.text = compact(message["reasoning_content"])
                yield row
            for turn, text, name, tool_id in self.api_content_rows(
                message, position, base_turn, opts.include_thinking
            ):
                row = self._row(artifact, context, position, timestamp, model)
                row.turn_type = turn
                row.tool_name = name
                row.tool_use_id = tool_id
                row.text = compact(text)
                yield row
        yield from error_rows(self, artifact, errors, context.task_id, context.project_path)

    # -- task_metadata.json

    def _parse_metadata(
        self, artifact: Artifact, context: TaskContext, opts: Options
    ) -> Iterator[Row]:
        errors: list = []
        data = load_json(artifact.disk_path, errors)
        if not isinstance(data, dict):
            yield from error_rows(
                self,
                artifact,
                errors or [(0, "task_metadata.json is not a JSON object", "")],
                context.task_id,
                context.project_path,
            )
            return
        events = []
        seen = set()
        for position, entry in enumerate(data.get("files_in_context") or [], 1):
            if not isinstance(entry, dict):
                continue
            for key in sorted(k for k in entry if k.endswith("_date")):
                timestamp = entry.get(key)
                if not isinstance(timestamp, (int, float)):
                    continue
                signature = (entry.get("path"), key, timestamp)
                if signature in seen:
                    continue
                seen.add(signature)
                events.append(
                    (
                        timestamp,
                        position,
                        "",
                        "file %s: %s (source=%s state=%s)"
                        % (
                            key[:-5],
                            entry.get("path") or "",
                            entry.get("record_source") or "",
                            entry.get("record_state") or "",
                        ),
                    )
                )
        for position, entry in enumerate(data.get("model_usage") or [], 1):
            if isinstance(entry, dict):
                # Text is decided after sorting: a model_usage entry becomes a
                # `model:` row when the model changes and a `permission mode:`
                # row when the plan/act mode changes.
                events.append(
                    (entry.get("ts") or 0, position, str(entry.get("model_id") or ""), entry)
                )
        for position, entry in enumerate(data.get("environment_history") or [], 1):
            if isinstance(entry, dict):
                events.append(
                    (
                        entry.get("ts") or 0,
                        position,
                        "",
                        "environment: %s"
                        % _fmt_counts(
                            entry,
                            (
                                "os_name",
                                "os_version",
                                "os_arch",
                                "host_name",
                                "host_version",
                                "cline_version",
                            ),
                        ),
                    )
                )
        events.sort(key=lambda event: event[0] if isinstance(event[0], (int, float)) else 0)
        models: dict = {}
        modes: dict = {}
        for timestamp, position, model, text in events:
            if isinstance(text, dict):
                usage = text
                texts = []
                change = track_change("model", models, context.task_id, model)
                if change:
                    extra = _fmt_counts(usage, ("model_provider_id", "mode"))
                    texts.append("%s %s" % (change, extra) if extra else change)
                mode = track_change(
                    "permission mode", modes, context.task_id, str(usage.get("mode") or "")
                )
                if mode:
                    texts.append(mode)
            else:
                texts = [text]
            for event_text in texts:
                row = self._row(artifact, context, position, to_utc(timestamp), model)
                row.turn_type = "system"
                row.text = compact(event_text)
                yield row

    # -- task index entries

    def history_row(self, artifact: Artifact, item: dict, position: int, opts: Options) -> Row:
        """Return the `task:` row of one task index entry."""
        row = self.base_row(artifact)
        row.source_line = position
        row.timestamp_utc = to_utc(item.get("ts"))
        row.session_id = str(item.get("id") or "")
        row.project_path = str(item.get(self.PROJECT_KEY) or "")
        row.model = str(item.get("modelId") or "")
        row.turn_type = "system"
        extra = _fmt_counts(
            item,
            (
                "tokensIn",
                "tokensOut",
                "cacheWrites",
                "cacheReads",
                "totalCost",
                "size",
                "apiProvider",
                "mode",
                "status",
                "apiConfigName",
                "parentTaskId",
                "rootTaskId",
            ),
        )
        row.text = compact("task: %s | %s" % (item.get("task") or "", extra))
        return row


def map_ui(kind: str, message: dict, first: bool) -> tuple[str, str, str] | None:
    """Return (turn_type, text, tool_name) for one ClineMessage, or None to skip it."""
    text = message.get("text")
    text = text if isinstance(text, str) else ("" if text is None else compact_json(text))
    images = message.get("images")
    image_count = len(images) if isinstance(images, list) else 0
    if kind in USER_KINDS or (kind == "text" and first):
        if image_count:
            text = _join(text, "[%d image(s)]" % image_count)
        return ("user", text, "") if text else None
    if kind == "reasoning":
        return ("thinking", text, "") if text else None
    if kind in ASSISTANT_KINDS:
        parsed = loads_or_none(text)
        if parsed is not None:
            choices = parsed.get("options") or parsed.get("suggest")
            if isinstance(choices, list):
                choices = "; ".join(
                    choice
                    if isinstance(choice, str)
                    else str(choice.get("answer") or compact_json(choice))
                    if isinstance(choice, dict)
                    else str(choice)
                    for choice in choices
                )
            text = _join(parsed.get("question") or parsed.get("response") or "", choices or "")
        return ("assistant", text, "") if text else None
    if kind in TOOL_USE_KINDS:
        name, body = tool_use_text(kind, text)
        return ("tool_use", body, name) if body or name else None
    if kind in TOOL_RESULT_KINDS:
        if kind == "browser_action_result":
            parsed = loads_or_none(text) or {}
            text = _join(
                parsed.get("currentUrl"),
                parsed.get("logs"),
                "[screenshot]" if parsed.get("screenshot") else "",
            )
        name = {
            "command_output": "execute_command",
            "browser_action_result": "browser_action",
            "codebase_search_result": "codebaseSearch",
            "subtask_result": "new_task",
        }.get(kind, "")
        return ("tool_result", text, name) if text else None
    if kind in ("api_req_started", "api_req_finished", "api_req_deleted"):
        parsed = loads_or_none(text) or {}
        detail = _fmt_counts(
            parsed,
            (
                "tokensIn",
                "tokensOut",
                "cacheWrites",
                "cacheReads",
                "cost",
                "apiProtocol",
                "cancelReason",
                "streamingFailedMessage",
            ),
        )
        return ("system", "%s: %s" % (kind, detail) if detail else kind, "")
    if kind == "condense_context":
        condense = message.get("contextCondense") or {}
        if isinstance(condense, dict) and condense:
            detail = _fmt_counts(condense, ("prevContextTokens", "newContextTokens", "cost"))
            return (
                "system",
                "condense_context: %s | %s" % (detail, condense.get("summary") or ""),
                "",
            )
    if kind == "checkpoint_created" and message.get("lastCheckpointHash"):
        return ("system", "checkpoint_created: %s" % message["lastCheckpointHash"], "")
    return ("system", "%s: %s" % (kind, text) if text else kind, "")


def tool_use_text(kind: str, text: str) -> tuple[str, str]:
    """Return (tool name, text) of a `tool_use` ClineMessage."""
    if kind == "tool":
        parsed = loads_or_none(text)
        if parsed is None:
            return "tool", text
        batch = parsed.get("batchFiles")
        paths = (
            "; ".join(str(entry.get("path")) for entry in batch if isinstance(entry, dict))
            if isinstance(batch, list)
            else ""
        )
        body = _join(
            parsed.get("path"),
            paths,
            parsed.get("regex"),
            parsed.get("filePattern"),
            parsed.get("query"),
            parsed.get("mode"),
            parsed.get("reason"),
            parsed.get("diff") or parsed.get("content"),
        )
        return str(parsed.get("tool") or "tool"), body
    if kind == "use_mcp_server":
        parsed = loads_or_none(text)
        if parsed is None:
            return "use_mcp_server", text
        name = parsed.get("toolName") or parsed.get("type") or "use_mcp_server"
        return str(name), _join(
            parsed.get("serverName"), parsed.get("uri"), parsed.get("arguments")
        )
    if kind == "browser_action":
        parsed = loads_or_none(text)
        if parsed is not None:
            return "browser_action", _join(
                parsed.get("action"), parsed.get("coordinate"), parsed.get("text")
            )
    if kind == "use_subagents":
        parsed = loads_or_none(text)
        if parsed is not None and isinstance(parsed.get("prompts"), list):
            return "use_subagents", " ; ".join(str(prompt) for prompt in parsed["prompts"])
    return TOOL_NAMES.get(kind, kind), text


def content_rows(
    content, base_turn: str, include_thinking: bool
) -> Iterator[tuple[str, str, str, str]]:
    """Yield (turn_type, text, tool_name, tool_use_id) per block of an Anthropic message.

    The legacy API history and Cline's SDK messages share it.
    """
    if isinstance(content, str):
        if content:
            yield base_turn, content, "", ""
        return
    if not isinstance(content, list):
        if content not in (None, ""):
            yield base_turn, compact_json(content), "", ""
        return
    for block in content:
        if isinstance(block, str):
            if block:
                yield base_turn, block, "", ""
            continue
        if not isinstance(block, dict):
            continue
        block_type = block.get("type")
        if block_type == "text":
            if block.get("text"):
                yield base_turn, str(block["text"]), "", ""
        elif block_type == "tool_use":
            name = str(block.get("name") or "")
            yield (
                "tool_use",
                tool_summary(name, block.get("input")),
                name,
                str(block.get("id") or block.get("call_id") or ""),
            )
        elif block_type == "tool_result":
            result_content = block.get("content")
            body = (
                text_of(result_content)
                if isinstance(result_content, (str, list)) or result_content is None
                else compact_json(result_content)
            )
            if block.get("is_error"):
                body = "[error] " + body
            yield (
                "tool_result",
                body,
                str(block.get("name") or ""),
                str(block.get("tool_use_id") or ""),
            )
        elif block_type in ("thinking", "reasoning"):
            if include_thinking:
                yield "thinking", str(block.get("thinking") or block.get("text") or ""), "", ""
        elif block_type == "redacted_thinking":
            if include_thinking:
                yield "thinking", "[redacted thinking]", "", ""
        elif block_type == "image":
            yield base_turn, "[image]", "", ""
        elif block_type == "file":
            yield base_turn, _join("[file]", block.get("path")), "", ""
        elif block_type:
            yield base_turn, "[%s]" % block_type, "", ""
