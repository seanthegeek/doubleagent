"""Parse OpenHands conversations (Agent Canvas, Agent Server and the `openhands` CLI).

Follows `research/openhands.md`. Validated against source, OpenHands/software-agent-sdk
at b347047 and 33d8aa2 (security verdicts, standing settings, model switches and
token usage, worktree scratch rows), OpenHands/OpenHands-CLI at 0403f1a (prompt
history) and OpenHands/OpenHands at 7fbb48c (0.62.0, legacy layout), with a
synthetic fixture; no real install. Files read, under `.openhands/`:

- `[agent-canvas/[dev_]]conversations/<hex id>/events/event-<index>-<id>.json`,
  one event object per file: `kind`, `id`, `timestamp`, `source` and the
  per-kind fields `_event_rows` reads (`llm_message`, `action`,
  `observation`, `tool_name`, `tool_call_id`, `security_risk`, `risks`,
  `llm_response_id`, ...). The conversation's `meta.json` (`title`,
  `created_at`, parent and fork ids, `workspace.working_dir`) and
  `base_state.json` (`agent.llm.model`, `confirmation_policy`,
  `security_analyzer`, `blocked_actions`, `stats...token_usages[]`) are read
  once per conversation; their secret fields are never read.
- `base_state.json` as its own file: the standing `permission setting:` rows
  and a `model:` row for a switch made over REST, at the file's time.
- `projects/<sha256 of cwd>/prompt_history.json`, the CLI's prompts, a
  secondary source (a file repeating the conversations' prompts).
- Legacy 0.x `sessions/<sid>/events/<n>.json`: one row per session, not parsed.

Event times are naive host local time. When `meta.json` has a UTC
`created_at`, the host's offset is the first event's time less `created_at`,
floored to the quarter hour after `SKEW`; without it (the CLI writes none)
the times are emitted as if UTC, and the header row says which. The session
id is the directory name; `project_path` is `workspace.working_dir`; no git
branch is recorded; `source_line` is 1.
"""

from __future__ import annotations

import hashlib
import json
import re
from collections.abc import Iterator
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from pathlib import Path

from ..errors import unreadable_message
from ..inputs import Artifact
from ..model import Row, compact
from ..textlines import ASCII_WHITESPACE
from ..timeutil import to_utc
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_bytes,
    file_mtime,
    setting_value,
    text_of,
)
from .cline_legacy import iter_json_array

EVENT_PATTERN = re.compile(
    r"^\.openhands/(?:agent-canvas/(?:dev_)?conversations|conversations)/"
    r"([0-9a-fA-F]{32})/events/event-(\d{5,})-[0-9a-fA-F-]{8,}\.json$"
)
EVENT_NAME_PATTERN = re.compile(r"^event-(\d{5,})-[0-9a-fA-F-]{8,}\.json$")
STATE_PATTERN = re.compile(
    r"^\.openhands/(?:agent-canvas/(?:dev_)?conversations|conversations)/"
    r"([0-9a-fA-F]{32})/base_state\.json$"
)
HISTORY_PATTERN = re.compile(r"^\.openhands/projects/([0-9a-f]{64})/prompt_history\.json$")
CONVERSATION_GLOBS = (
    "agent-canvas/dev_conversations/*/base_state.json",
    "agent-canvas/conversations/*/base_state.json",
    "conversations/*/base_state.json",
    "agent-canvas/dev_conversations/*/meta.json",
    "agent-canvas/conversations/*/meta.json",
)
# The Agent Server's default `conversation_worktree_root`; macOS resolves
# /tmp to /private/tmp.
WORKTREE_ROOTS = ("/tmp/conversation-worktrees/", "/private/tmp/conversation-worktrees/")
SWITCH_KINDS = ("SwitchLLMObservation", "ClassifyAndSwitchLLMObservation")
LEGACY_PATTERN = re.compile(
    r"^\.openhands/(?:sessions|users/[^/]+/conversations)/([^/]+)/events/(\d+)\.json$"
)

QUARTER = 900
SKEW = 60
MAX_OFFSET = 14 * 3600


def _dict_or_empty(value) -> dict:
    return value if isinstance(value, dict) else {}


def _join(*parts: str) -> str:
    return " | ".join(part for part in parts if part)


def _load_json(path: Path, errors: list) -> tuple[object, str]:
    """Return the decoded JSON of `path` and "", or None and why it did not decode.

    A file that exists but cannot be read gives None and "" after its
    `unreadable_message` entry in `errors`. A missing file, a symlink or a
    directory is None and "" with no entry, since the CLI writes no
    `meta.json` and that is not an error.
    """
    try:
        if path.is_symlink() or not path.is_file():
            return None, ""
    except OSError as error:
        errors.append((0, unreadable_message(path, error)))
        return None, ""
    data = file_bytes(path, errors)
    if data is None:
        return None, ""
    try:
        return json.loads(data.decode("utf-8", errors="replace")), ""
    except ValueError as error:
        return None, "could not be decoded (%s)" % error


def _read_json(path: Path):
    """Return the decoded JSON of a file this parser also parses in full, else None.

    A pre-scan: the event and conversation files it reads are reported when
    they are parsed as artifacts or loaded as a conversation's state.
    """
    return _load_json(path, [])[0]


def _is_file(path: Path) -> bool:
    try:
        return not path.is_symlink() and path.is_file()
    except OSError:
        return False


def _load_object(path: Path, problems: list) -> dict:
    """Return the JSON object in `path`, else {}.

    A file that exists but cannot be read appends its whole-file entry
    (0, `unreadable_message`) to `problems`; one that does not decode or is
    not an object appends (1, `<name> <reason>`).
    """
    read_errors: list = []
    value, reason = _load_json(path, read_errors)
    if read_errors:
        problems.extend(read_errors)
        return {}
    if not reason and not isinstance(value, dict) and (value is not None or _is_file(path)):
        reason = "is not a JSON object"  # a list, a scalar or a literal null
    if reason:
        problems.append((1, "%s %s" % (path.name, reason)))
    return value if isinstance(value, dict) else {}


def _aware(value) -> datetime | None:
    """Return a UTC datetime from a string `to_utc` accepts, else None."""
    timestamp = to_utc(value) if isinstance(value, str) else ""
    if not timestamp:
        return None
    return datetime.fromisoformat(timestamp[:-1]).replace(tzinfo=timezone.utc)


def _naive(value) -> datetime | None:
    """Return a naive datetime when the timestamp has no zone, else None."""
    if not isinstance(value, str) or not value:
        return None
    try:
        parsed = datetime.fromisoformat(
            re.sub(r"(\.[0-9]{6})[0-9]+", r"\1", value.strip(ASCII_WHITESPACE))
        )
    except ValueError:
        return None
    return parsed if parsed.tzinfo is None else None


def _format_offset(seconds: int) -> str:
    sign = "+" if seconds >= 0 else "-"
    seconds = abs(seconds)
    return "UTC%s%02d:%02d" % (sign, seconds // 3600, seconds % 3600 // 60)


@dataclass
class _Conversation:
    session_id: str
    project_path: str = ""
    model: str = ""
    offset: int | None = None  # seconds east of UTC, when anchored
    first_event: str = ""  # file name of the lowest-index event
    header: str = ""
    created: str = ""
    # Whole-file `errors` entries for a meta.json or base_state.json that
    # exists but cannot be read or did not decode.
    problems: list = field(default_factory=list)
    # The model in force at the conversation's start: the model of the first
    # response before any switch, else `base_state.json` `agent.llm.model`.
    initial_model: str = ""
    # Per event file name: a `model:` row written before or after the
    # event's own rows, and the model in force at that event.
    model_before: dict[str, str] = field(default_factory=dict)
    model_after: dict[str, tuple[str, str]] = field(default_factory=dict)
    event_model: dict[str, str] = field(default_factory=dict)
    # A `model: <old> -> <new>` row for the base_state.json artifact when its
    # `agent.llm.model` differs from the last model the events show.
    final_model_row: str = ""
    # ActionEvent id -> (tool_name, tool_call_id), for the security
    # analyzer's verdicts and the blocked actions, which name action ids.
    actions: dict[str, tuple[str, str]] = field(default_factory=dict)
    # `permission setting:` rows from base_state.json, with their join.
    settings: list[tuple[str, str, str]] = field(default_factory=list)
    scratch: str = ""


class OpenHandsParser(Parser):
    """Parse OpenHands event files, conversation state, CLI prompt history and legacy sessions."""

    agent = "openhands"
    name = "openhands"

    has_secondary = True

    def __init__(self) -> None:
        """Start with an empty cache of loaded conversations."""
        self._cache: dict[tuple, _Conversation] = {}

    def wants(self, artifact: Artifact) -> bool:
        """Return True for an event file, `base_state.json`, prompt history or legacy event."""
        rel = artifact.rel
        return bool(
            EVENT_PATTERN.match(rel)
            or LEGACY_PATTERN.match(rel)
            or STATE_PATTERN.match(rel)
            or HISTORY_PATTERN.match(rel)
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for the CLI's prompt history."""
        return bool(HISTORY_PATTERN.match(artifact.rel))

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one OpenHands file; a conversation's first event adds its header."""
        if LEGACY_PATTERN.match(artifact.rel):
            yield from self._parse_legacy(artifact, opts)
            return
        if HISTORY_PATTERN.match(artifact.rel):
            yield from self._parse_history(artifact)
            return
        if STATE_PATTERN.match(artifact.rel):
            yield from self._parse_state(artifact)
            return
        conversation = self._conversation(artifact.disk_path.parent.parent)
        name = artifact.disk_path.name
        if name == conversation.first_event:
            row = self._row(artifact, conversation, "system")
            row.timestamp_utc = conversation.created
            row.text = compact(conversation.header)
            yield row
            for text in (
                "model: " + conversation.initial_model if conversation.initial_model else "",
                conversation.scratch,
            ):
                if text:
                    row = self._row(artifact, conversation, "system")
                    row.timestamp_utc = conversation.created
                    row.model = conversation.initial_model or conversation.model
                    row.text = text
                    yield row
            yield from error_rows(
                self,
                artifact,
                conversation.problems,
                conversation.session_id,
                conversation.project_path,
            )
        read_errors: list = []
        event, reason = _load_json(artifact.disk_path, read_errors)
        if read_errors:
            yield from error_rows(
                self, artifact, read_errors, conversation.session_id, conversation.project_path
            )
            return
        if not isinstance(event, dict):
            row = self._row(artifact, conversation, "system")
            row.text = "parser: event file %s" % (reason or "is not a JSON object")
            yield row
            return
        timestamp = self._timestamp(event.get("timestamp"), conversation)
        model = conversation.event_model.get(name, conversation.initial_model or conversation.model)
        rows: list[Row] = []
        if name in conversation.model_before:
            rows.append(self._row(artifact, conversation, "system"))
            rows[-1].text = conversation.model_before[name]
        rows.extend(self._event_rows(artifact, conversation, event, opts))
        for row in rows:
            row.timestamp_utc = timestamp
            row.model = model
            yield row
        if name in conversation.model_after:
            row = self._row(artifact, conversation, "system")
            row.text, row.model = conversation.model_after[name]
            row.timestamp_utc = timestamp
            yield row

    # -- base_state.json: standing permissions and a model set over REST ----------------
    def _parse_state(self, artifact: Artifact) -> Iterator[Row]:
        conversation = self._conversation(artifact.disk_path.parent)
        timestamp = file_mtime(artifact)
        texts = list(conversation.settings)
        if conversation.final_model_row:
            texts.append((conversation.final_model_row, "", ""))
        for text, tool_name, tool_use_id in texts:
            row = self._row(artifact, conversation, "system")
            row.timestamp_utc = timestamp
            row.text = text
            row.tool_name, row.tool_use_id = tool_name, tool_use_id
            yield row

    # -- CLI prompt history (secondary) -----------------------------------------------
    def _parse_history(self, artifact: Artifact) -> Iterator[Row]:
        match = HISTORY_PATTERN.match(artifact.rel)
        assert match is not None  # parse() routes only matching paths here
        project = _project_for_hash(artifact.disk_path.parent.parent.parent, match.group(1))
        errors: list = []
        for index, entry in iter_json_array(artifact.disk_path, errors):
            if not isinstance(entry, dict):
                message = "unparseable record %d (not a JSON object)" % index
                errors.append((index, message, compact_json(entry)))
                continue
            text = entry.get("text")
            if not isinstance(text, str) or not text:
                continue
            row = self.base_row(artifact)
            row.turn_type = "user"
            row.project_path = project
            row.source_line = index
            stamp = entry.get("timestamp")
            row.timestamp_utc = to_utc(stamp) if isinstance(stamp, str) else ""
            row.text = text
            yield row
        yield from error_rows(self, artifact, errors, "", project)

    # -- conversation context ---------------------------------------------------------
    def _conversation(self, conversation_dir: Path) -> _Conversation:
        def mtime(path: Path) -> int:
            try:
                return path.stat().st_mtime_ns
            except OSError:
                return 0

        key = (
            str(conversation_dir),
            mtime(conversation_dir / "meta.json"),
            mtime(conversation_dir / "base_state.json"),
            mtime(conversation_dir / "events"),
        )
        conversation = self._cache.get(key)
        if conversation is None:
            conversation = self._load_conversation(conversation_dir)
            if len(self._cache) > 256:
                self._cache.clear()
            self._cache[key] = conversation
        return conversation

    def _load_conversation(self, conversation_dir: Path) -> _Conversation:
        conversation = _Conversation(session_id=conversation_dir.name)
        meta = _load_object(conversation_dir / "meta.json", conversation.problems)
        state = _load_object(conversation_dir / "base_state.json", conversation.problems)
        conversation.project_path = str(
            _dict_or_empty(meta.get("workspace")).get("working_dir")
            or _dict_or_empty(state.get("workspace")).get("working_dir")
            or ""
        )
        conversation.model = str(
            _dict_or_empty(_dict_or_empty(state.get("agent")).get("llm")).get("model") or ""
        )

        events: list[tuple[int, str]] = []
        try:
            for path in (conversation_dir / "events").iterdir():
                match = EVENT_NAME_PATTERN.match(path.name)
                if match and not path.is_symlink():
                    events.append((int(match.group(1)), path.name))
        except OSError:
            pass
        events.sort()
        conversation.first_event = events[0][1] if events else ""
        loaded = [
            (file_name, _dict_or_empty(_read_json(conversation_dir / "events" / file_name)))
            for _, file_name in events
        ]
        first = loaded[0][1] if loaded else {}
        _track_models(conversation, state, loaded)
        conversation.settings = _settings(state, conversation.actions)
        conversation.scratch = _scratch(conversation.project_path)

        created = _aware(meta.get("created_at"))
        naive_first = _naive(first.get("timestamp"))
        if created is not None and naive_first is not None:
            diff = (naive_first.replace(tzinfo=timezone.utc) - created).total_seconds()
            offset = int((diff + SKEW) // QUARTER) * QUARTER
            if abs(offset) <= MAX_OFFSET:
                conversation.offset = offset
        if created is not None:
            conversation.created = to_utc(meta.get("created_at"))
        else:
            conversation.created = self._timestamp(first.get("timestamp"), conversation)

        if conversation.offset is not None:
            basis = (
                "event times are host local, corrected by %s from meta.json created_at"
                % _format_offset(conversation.offset)
            )
        elif naive_first is not None:
            basis = "event times are host local with no zone, emitted as if UTC"
        else:
            basis = ""
        conversation.header = _join(
            "conversation start",
            str(meta.get("title") or ""),
            "id=%s" % meta["id"] if meta.get("id") else "",
            "parent=%s" % meta["parent_conversation_id"]
            if meta.get("parent_conversation_id")
            else "",
            "forked_from=%s" % meta["forked_from_conversation_id"]
            if meta.get("forked_from_conversation_id")
            else "",
            "model=%s" % (conversation.initial_model or conversation.model)
            if conversation.initial_model or conversation.model
            else "",
            "status=%s" % state["execution_status"] if state.get("execution_status") else "",
            "%d events" % len(events),
            basis,
        )
        return conversation

    @staticmethod
    def _timestamp(value, conversation: _Conversation) -> str:
        naive = _naive(value)
        if naive is not None and conversation.offset is not None:
            return to_utc((naive - timedelta(seconds=conversation.offset)).isoformat() + "+00:00")
        return to_utc(value) if isinstance(value, str) else ""

    def _row(self, artifact: Artifact, conversation: _Conversation, turn_type: str) -> Row:
        row = self.base_row(artifact)
        row.session_id = conversation.session_id
        row.project_path = conversation.project_path
        row.model = conversation.model
        row.turn_type = turn_type
        row.source_line = 1
        return row

    # -- events -----------------------------------------------------------------------
    def _event_rows(
        self, artifact: Artifact, conversation: _Conversation, event: dict, opts: Options
    ) -> Iterator[Row]:
        kind = str(event.get("kind") or "")

        def make_row(turn_type: str, text: str, tool_name: str = "", tool_use_id: str = "") -> Row:
            row = self._row(artifact, conversation, turn_type)
            row.tool_name, row.tool_use_id = tool_name, tool_use_id
            row.text = compact(text)
            return row

        if kind == "MessageEvent":
            message = _dict_or_empty(event.get("llm_message"))
            role = str(message.get("role") or event.get("source") or "")
            if opts.include_thinking:
                thinking = _thinking(message)
                if thinking:
                    yield make_row("thinking", thinking)
            text = text_of(message.get("content"))
            if role == "tool":
                yield make_row(
                    "tool_result",
                    text,
                    str(message.get("name") or ""),
                    str(message.get("tool_call_id") or ""),
                )
            elif text or role in ("user", "system"):
                yield make_row(role if role in ("user", "assistant", "system") else "system", text)
            for call in message.get("tool_calls") or []:
                call = _dict_or_empty(call)
                function = _dict_or_empty(call.get("function"))
                yield make_row(
                    "tool_use",
                    str(call.get("arguments") or function.get("arguments") or ""),
                    str(call.get("name") or function.get("name") or ""),
                    str(call.get("id") or ""),
                )
        elif kind == "ActionEvent":
            if opts.include_thinking:
                thinking = _join(text_of(event.get("thought")), _thinking(event))
                if thinking:
                    yield make_row("thinking", thinking)
            risk = event.get("security_risk")
            yield make_row(
                "tool_use",
                _action_text(event)
                + (" security_risk=%s" % risk if risk not in (None, "") else ""),
                str(event.get("tool_name") or ""),
                str(event.get("tool_call_id") or ""),
            )
        elif kind == "ObservationEvent":
            observation = _dict_or_empty(event.get("observation"))
            text = text_of(observation.get("content"))
            if observation.get("is_error"):
                text = "[error] " + text
            yield make_row(
                "tool_result",
                text,
                str(event.get("tool_name") or ""),
                str(event.get("tool_call_id") or ""),
            )
        elif kind == "UserRejectObservation":
            tool_name, tool_use_id = (
                str(event.get("tool_name") or ""),
                str(event.get("tool_call_id") or ""),
            )
            yield make_row(
                "tool_result",
                "[rejected by %s] %s"
                % (event.get("rejection_source") or "user", event.get("rejection_reason") or ""),
                tool_name,
                tool_use_id,
            )
            yield make_row(
                "system",
                _join_space(
                    "permission decision: rejected rejection_source=%s"
                    % (event.get("rejection_source") or "-"),
                    str(event.get("rejection_reason") or ""),
                ),
                tool_name,
                tool_use_id,
            )
        elif kind == "AgentErrorEvent":
            yield make_row(
                "tool_result",
                "[error] %s" % (event.get("error") or ""),
                str(event.get("tool_name") or ""),
                str(event.get("tool_call_id") or ""),
            )
        elif kind == "ACPToolCallEvent":
            name, call_id = (
                str(event.get("tool_kind") or "acp"),
                str(event.get("tool_call_id") or ""),
            )
            raw_input = event.get("raw_input")
            yield make_row(
                "tool_use",
                _join(
                    str(event.get("title") or ""),
                    compact_json(raw_input) if raw_input is not None else "",
                ),
                name,
                call_id,
            )
            out = event.get("raw_output")
            out_text = _join(
                out if isinstance(out, str) else compact_json(out) if out is not None else "",
                text_of(event.get("content")) if isinstance(event.get("content"), list) else "",
            )
            if out_text or event.get("status") in ("completed", "failed"):
                if event.get("is_error") or event.get("status") == "failed":
                    out_text = "[error] " + out_text
                yield make_row("tool_result", out_text, name, call_id)
        elif kind == "SystemPromptEvent":
            tools = event.get("tools")
            yield make_row(
                "system",
                "system prompt, %d tools: %s"
                % (
                    len(tools) if isinstance(tools, list) else 0,
                    text_of(event.get("system_prompt")),
                ),
            )
        elif kind in ("Condensation", "CondensationSummaryEvent"):
            forgotten = event.get("forgotten_event_ids")
            yield make_row(
                "system",
                _join(
                    "%s:" % kind,
                    "%d events forgotten" % len(forgotten) if isinstance(forgotten, list) else "",
                    str(event.get("summary") or ""),
                ),
            )
        elif kind == "HookExecutionEvent":
            yield make_row(
                "system",
                _join(
                    "hook %s: %s"
                    % (event.get("hook_event_type") or "", event.get("hook_command") or ""),
                    "exit=%s" % event["exit_code"] if event.get("exit_code") is not None else "",
                    "blocked" if event.get("blocked") else "",
                    str(event.get("stderr") or ""),
                ),
            )
        elif kind == "ConversationStateUpdateEvent":
            # The full_state value is a whole ConversationState, secrets
            # included; only its key is reported.
            value = event.get("value")
            shown = (
                ""
                if isinstance(value, (dict, list)) or event.get("key") == "full_state"
                else str(value)
            )
            yield make_row(
                "system",
                "state update: %s%s" % (event.get("key") or "", "=" + shown if shown else ""),
            )
        elif kind == "SecurityAnalysisEvent" and _dict_or_empty(event.get("risks")):
            details = _dict_or_empty(event.get("details"))
            for action_id, risk in _dict_or_empty(event.get("risks")).items():
                tool_name, tool_use_id = conversation.actions.get(str(action_id), ("", ""))
                detail = details.get(action_id)
                yield make_row(
                    "system",
                    _join_space(
                        "permission decision: analyzed risk=%s analyzer=%s policy=%s"
                        % (risk, event.get("analyzer") or "-", event.get("policy") or "-"),
                        "" if tool_use_id else "action_id=%s" % action_id,
                        "details=%s" % compact_json(detail) if detail else "",
                    ),
                    tool_name,
                    tool_use_id,
                )
        else:
            rest = {
                key: item
                for key, item in event.items()
                if key not in ("kind", "id", "timestamp", "source", "parent_id")
            }
            yield make_row(
                "system", _join("%s:" % (kind or "event"), compact_json(rest) if rest else "")
            )

    # -- legacy 0.x ---------------------------------------------------------------------
    def _parse_legacy(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        match = LEGACY_PATTERN.match(artifact.rel)
        assert match is not None  # parse() routes only matching paths here
        mine = int(match.group(2))
        numbers = []
        try:
            for path in artifact.disk_path.parent.iterdir():
                if path.suffix == ".json" and path.stem.isdigit() and not path.is_symlink():
                    numbers.append(int(path.stem))
        except OSError:
            pass
        if numbers and mine != min(numbers):
            return
        read_errors: list = []
        event = _dict_or_empty(_load_json(artifact.disk_path, read_errors)[0])
        row = self.base_row(artifact)
        row.session_id = match.group(1)
        row.turn_type = "system"
        row.source_line = 1
        row.timestamp_utc = (
            to_utc(event.get("timestamp")) if isinstance(event.get("timestamp"), str) else ""
        )
        row.text = compact(
            "legacy OpenHands 0.x session, %d event files, not parsed; "
            "times are host local, emitted as if UTC" % len(numbers),
        )
        yield row
        yield from error_rows(self, artifact, read_errors, row.session_id)


def _project_for_hash(root: Path, digest: str) -> str:
    """Return the working directory under `root` whose SHA-256 is `digest`, else "".

    `root` is the `.openhands` directory; the directories are the
    conversations' `workspace.working_dir`, and the digest is the name the
    CLI gives a project's directory under `projects/`.
    """
    for pattern in CONVERSATION_GLOBS:
        try:
            paths = sorted(root.glob(pattern))
        except OSError:
            continue
        for path in paths:
            working_dir = _dict_or_empty(_dict_or_empty(_read_json(path)).get("workspace")).get(
                "working_dir"
            )
            if isinstance(working_dir, str) and working_dir and _sha256(working_dir) == digest:
                return working_dir
    return ""


def _sha256(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def _scratch(working_dir: str) -> str:
    """Return the `scratch directory:` text for a directory under the default worktree root."""
    for root in WORKTREE_ROOTS:
        if working_dir.startswith(root) and working_dir[len(root) :].strip("/"):
            return (
                "scratch directory: %s (conversation worktree under the Agent Server's "
                "default conversation_worktree_root /tmp/conversation-worktrees)" % working_dir
            )
    return ""


def _track_models(conversation: _Conversation, state: dict, loaded: list[tuple[str, dict]]) -> None:
    """Fill the conversation's model rows and ActionEvent map from its events in index order.

    Each response's model comes from `stats.usage_to_metrics.*.token_usages[]`
    joined on the event's `llm_response_id`, the agent's own switches from
    the `SWITCH_KINDS` observations' `active_model`, and a final row when
    `agent.llm.model` differs from the last model the events show, since a
    switch over REST leaves no event.
    """
    by_response: dict[str, str] = {}
    for metrics in _dict_or_empty(
        _dict_or_empty(state.get("stats")).get("usage_to_metrics")
    ).values():
        for usage in _dict_or_empty(metrics).get("token_usages") or []:
            usage = _dict_or_empty(usage)
            response_id, model = usage.get("response_id"), usage.get("model")
            if isinstance(response_id, str) and response_id and isinstance(model, str) and model:
                by_response.setdefault(response_id, model)

    initial, switched = "", False
    for _, event in loaded:
        if _switch(event)[0]:
            switched = True
            break
        initial = by_response.get(str(event.get("llm_response_id") or ""), "")
        if initial:
            break
    if not initial and not switched:
        initial = conversation.model
    conversation.initial_model = current = initial
    for file_name, event in loaded:
        if event.get("kind") == "ActionEvent" and event.get("id"):
            conversation.actions[str(event["id"])] = (
                str(event.get("tool_name") or ""),
                str(event.get("tool_call_id") or ""),
            )
        model = by_response.get(str(event.get("llm_response_id") or ""), "")
        if model and model != current:
            conversation.model_before[file_name] = _model_text(current, model)
            current = model
        conversation.event_model[file_name] = current
        new, suffix = _switch(event)
        if new and new != current:
            conversation.model_after[file_name] = (_model_text(current, new) + suffix, new)
            current = new
    if conversation.model and current and conversation.model != current:
        conversation.final_model_row = (
            "%s (from base_state.json agent.llm, file mtime)"
            % _model_text(current, conversation.model)
        )


def _model_text(old: str, new: str) -> str:
    return "model: %s -> %s" % (old, new) if old else "model: %s" % new


def _switch(event: dict) -> tuple[str, str]:
    """Return (new model, ` profile=<name>[ class=<class>]`) of a model switch, else ("", "")."""
    observation = _dict_or_empty(event.get("observation"))
    if event.get("kind") != "ObservationEvent" or observation.get("kind") not in SWITCH_KINDS:
        return "", ""
    new = observation.get("active_model")
    if observation.get("is_error") or not isinstance(new, str) or not new:
        return "", ""
    profile = observation.get("profile_name") or observation.get("model")
    suffix = " profile=%s" % profile if isinstance(profile, str) and profile else ""
    if observation.get("chosen_class"):
        suffix += " class=%s" % observation["chosen_class"]
    return new, suffix


def _settings(state: dict, actions: dict[str, tuple[str, str]]) -> list[tuple[str, str, str]]:
    """Return (text, tool_name, tool_use_id) of each `permission setting:` row of base_state.json.

    The rows are the confirmation policy, the security analyzer (`none` when
    the key is absent, since the file is written with `exclude_none`) and one
    per hook-blocked action, joined to its tool call when the events name it.
    """
    if not state:
        return []
    tail = " (from base_state.json, file mtime)"
    out: list[tuple[str, str, str]] = []
    policy = _dict_or_empty(state.get("confirmation_policy"))
    if policy.get("kind"):
        text = "permission setting: confirmation_policy=%s" % policy["kind"]
        if policy.get("threshold"):
            text += " threshold=%s" % policy["threshold"]
        if "confirm_unknown" in policy:
            text += " confirm_unknown=%s" % setting_value(policy["confirm_unknown"])
        out.append((text + tail, "", ""))
    analyzer = _dict_or_empty(state.get("security_analyzer")).get("kind") or "none"
    out.append(("permission setting: security_analyzer=%s%s" % (analyzer, tail), "", ""))
    for action_id, reason in _dict_or_empty(state.get("blocked_actions")).items():
        tool_name, tool_use_id = actions.get(str(action_id), ("", ""))
        text = "permission setting: blocked_actions.%s=%s%s" % (
            action_id,
            setting_value(reason),
            tail,
        )
        out.append((text, tool_name, tool_use_id))
    return out


def _join_space(*parts: str) -> str:
    return " ".join(part for part in parts if part)


def _thinking(record: dict) -> str:
    parts = []
    if isinstance(record.get("reasoning_content"), str):
        parts.append(record["reasoning_content"])
    for block in record.get("thinking_blocks") or []:
        if isinstance(block, dict) and isinstance(block.get("thinking"), str):
            parts.append(block["thinking"])
    return _join(*parts)


def _action_text(event: dict) -> str:
    """Return a terminal action's command, a file edit's `<command> <path>`, else the arguments."""
    action = _dict_or_empty(event.get("action"))
    command, path = action.get("command"), action.get("path")
    if isinstance(path, str) and path:
        return "%s %s" % (command, path) if isinstance(command, str) and command else path
    if isinstance(command, str) and command:
        return command
    args = _dict_or_empty(event.get("tool_call")).get("arguments")
    if isinstance(args, str) and args:
        return args
    rest = {key: value for key, value in action.items() if key != "kind"}
    return compact_json(rest) if rest else str(action.get("kind") or "")
