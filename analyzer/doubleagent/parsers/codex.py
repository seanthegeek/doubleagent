"""Parse Codex CLI rollouts, and the rollout format its forks share.

Reads, under `~/.codex` (`RolloutParser.home_prefix` for a fork), the rollouts under
`sessions/` and `archived_sessions/` (`rollout-*.jsonl`, or `.jsonl.zst` decoded with
`zstandard`, a truncated frame keeping the lines before the cut), `history.jsonl` (secondary),
`config.toml` and `rules/*.rules` (standing grants), `memories/rollout_summaries/*.md`, a
fork's import ledger (`ledger_name`) and, for Codex only, `automations/*/automation.toml`.
`codex_state.py` reads `state_5.sqlite` and its `db-backups/` copies, `thread_history_1.sqlite`,
`logs_<N>.sqlite` and `.codex-global-state.json`. Yields user, assistant, thinking, tool_use,
tool_result and the `system` rows of the research document's Parser plan.

Fields the parser depends on:

- every rollout line: `timestamp`, `type`, `payload`, `metadata.inherited_user_message`;
- `session_meta`: `id`, `session_id`, `timestamp`, `cwd`, `originator`, `cli_version`,
  `model_provider`, `git.branch`, `git.repository_url`, `creator_account_id`,
  `creator_user_id`, `parent_thread_id`, `thread_source`, `source.subagent`, `agent_path`,
  `agent_role`, `agent_nickname`, `forked_from_id`;
- `turn_context`: `cwd`, `model` and the `POLICY_FIELDS` keys;
- `response_item`: `type`, `role`, `content`, `name`, `call_id`, `arguments`, `input`,
  `output`, `action`, `tools`, `author`, `recipient`, `other_recipients`, `revised_prompt`,
  `status`, `summary`, `internal_chat_message_metadata_passthrough.content_item_kinds`;
- `event_msg`: `turn_id`, `duration_ms`, `turn_attribution`, `last_agent_message`,
  `thread_settings`, `turnId`, `goal`, `reason`, `error`, `num_turns`, and `item_completed`'s
  `item`, `started_at_ms`, `completed_at_ms`; `compacted`: `message`, `replacement_history`;
  `inter_agent_communication`: `id`, `content`, `encrypted_content`; `realtime_item`: `type`,
  `role`, `text`; `history.jsonl`: `session_id`, `text`, `ts` (epoch seconds).

Validated against real installs and codex source at 3e23877. At 4e7b9e2 `turn_context` has no
`network`, so `network=` reads `-` on rollouts of such builds. See `research/codex-cli.md`.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterator
from typing import cast

import zstandard

from ..errors import DoubleAgentError, unreadable_message
from ..inputs import Artifact, error_reason
from ..model import GIT_REMOTE_PREFIX, Row, compact
from ..textlines import ASCII_SPACE, split_lines
from ..timeutil import to_utc
from ..toml_util import load_config_toml, load_toml
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_bytes,
    file_mtime,
    iter_jsonl,
    line_text,
    setting_value,
    text_of,
    track_change,
)

ROLLOUT_TMPL = r"^%s/(?:archived_)?sessions/(?:.*/)?rollout-[^/]*\.jsonl(?:\.zst)?$"
AUTOMATION_TMPL = r"^%s/automations/[^/]+/automation\.toml$"
LOGS_TMPL = r"^%s/logs_[0-9]+\.sqlite$"
RULES_TMPL = r"^%s/rules/[^/]+\.rules$"
MEMORY_SUMMARY_TMPL = r"^%s/memories/rollout_summaries/[^/]+\.md$"
BACKUP_STATE_TMPL = r"^%s/db-backups/[^/]+/state_5\.sqlite$"
ROLLOUT_PATTERN = re.compile(ROLLOUT_TMPL % re.escape(".codex"))
HISTORY_REL = ".codex/history.jsonl"
GLOBAL_STATE_NAME = ".codex-global-state.json"
UUID = r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"
# `rollout-<time>-<thread id>.jsonl`, or `...-<thread id>_<rollout id>.jsonl`
# for a reverted thread; group 1 is the thread id either way.
UUID_TAIL_PATTERN = re.compile(r"(%s)(?:_%s)?\.jsonl(?:\.zst)?$" % (UUID, UUID))

CALL_TYPES = ("function_call", "custom_tool_call", "local_shell_call", "web_search_call")
OUTPUT_TYPES = ("function_call_output", "custom_tool_call_output", "local_shell_call_output")


def call_summary(payload: dict) -> str:
    """Return the text of a tool call: its command, query or arguments."""
    payload_type = payload.get("type")
    if payload_type == "local_shell_call":
        action = payload.get("action") or {}
        command = action.get("command")
        if isinstance(command, list):
            return " ".join(str(part) for part in command)
        return str(command or compact_json(action))
    if payload_type == "web_search_call":
        action = payload.get("action") or {}
        return str(action.get("query") or compact_json(action))
    for key in ("arguments", "input"):
        value = payload.get(key)
        if value not in (None, ""):
            return value if isinstance(value, str) else compact_json(value)
    return compact_json(
        {
            field: value
            for field, value in payload.items()
            if field not in ("type", "id", "call_id", "name", "status")
        }
    )


def call_name(payload: dict) -> str:
    """Return the tool name of a call: `shell`, `web_search` or the call's `name`."""
    payload_type = payload.get("type")
    if payload_type == "local_shell_call":
        return "shell"
    if payload_type == "web_search_call":
        return "web_search"
    return str(payload.get("name") or payload_type or "")


def _decode_line(line_number: int, raw: bytes, errors: list) -> dict | None:
    stripped = raw.strip()
    if not stripped:
        return None
    try:
        record = json.loads(stripped.decode("utf-8", errors="replace"))
    except ValueError as error:
        errors.append((line_number, "unparseable line (%s)" % error, line_text(raw)))
        return None
    return record if isinstance(record, dict) else None


class TruncatedFrame(DoubleAgentError):
    """The zstd stream ends inside a frame."""

    pass


def iter_zst_chunks(handle) -> Iterator[bytes]:
    """Yield the decompressed bytes of every zstd frame in `handle`, streamed.

    Raises:
        zstandard.ZstdError: the input is corrupt.
        TruncatedFrame: the file ends inside a frame, where the stream reader would end
            silently.
    """
    decompressor = zstandard.ZstdDecompressor()
    frame_decoder = decompressor.decompressobj()
    open_frame = False
    while True:
        data = handle.read(1 << 16)
        if not data:
            break
        while data:
            out = frame_decoder.decompress(data)
            open_frame = True
            if out:
                yield out
            if getattr(frame_decoder, "eof", False):
                data = frame_decoder.unused_data
                frame_decoder = decompressor.decompressobj()
                open_frame = False
            else:
                data = b""
    if open_frame and hasattr(frame_decoder, "eof"):
        raise TruncatedFrame("file ends inside a zstd frame")


def iter_zst_jsonl(path, errors: list) -> Iterator[tuple[int, dict]]:
    """Yield (line number, record) of a zstd-compressed JSONL file, streamed, like `iter_jsonl`.

    A corrupt or truncated frame becomes one entry in `errors`; the lines decoded before it are
    still yielded, and a line cut by the damage is a bad line. A file that cannot be opened is
    one entry (0, `unreadable_message`), and one that fails while being read is one entry at the
    last line read, after the lines before it, as `iter_lines` does. Nothing raises.
    """
    line_number = 0
    buffer = b""
    try:
        with open(path, "rb") as handle:
            try:
                for chunk in iter_zst_chunks(handle):
                    buffer += chunk
                    lines = buffer.split(b"\n")
                    buffer = lines.pop()
                    for raw in lines:
                        line_number += 1
                        record = _decode_line(line_number, raw, errors)
                        if record is not None:
                            yield line_number, record
            except (zstandard.ZstdError, TruncatedFrame) as error:
                errors.append((line_number + 1, "zstd: %s" % error, ""))
    except OSError as error:
        # line_number is 0 when the open failed, else the last line read.
        errors.append((line_number, unreadable_message(path, error)))
        return
    if buffer.strip():
        line_number += 1
        record = _decode_line(line_number, buffer, errors)
        if record is not None:
            yield line_number, record


# Turn items that only repeat a response_item message, reasoning or the
# `compacted` line, under an id of their own (`items.rs` `new_item_id`).
RESPONSE_CARRIED_ITEMS = (
    "UserMessage",
    "AgentMessage",
    "Reasoning",
    "HookPrompt",
    "ContextCompaction",
)
# Content kinds of a user-role message block that the person supplied.
USER_KINDS = ("", "unknown")
# Wrappers the harness puts around the user-role context it injects, as
# (name, start, end); Codex recognises the same blocks with
# CONTEXTUAL_USER_FRAGMENT_MATCHERS (`core/src/context/contextual_user_message.rs`
# at 3e23877; citations in research/codex-cli.md, Parser plan). Used only for
# rollouts without content kinds.
CONTEXT_MARKERS = (
    ("agents_md_instructions", "# AGENTS.md instructions", "</INSTRUCTIONS>"),
    ("environment_context", "<environment_context>", "</environment_context>"),
    (
        "agent_message_board_notification",
        "<agent_message_board_notification>",
        "</agent_message_board_notification>",
    ),
    ("skill", "<skill>", "</skill>"),
    ("user_shell_command", "<user_shell_command>", "</user_shell_command>"),
    ("turn_aborted", "<turn_aborted>", "</turn_aborted>"),
    ("subagent_notification", "<subagent_notification>", "</subagent_notification>"),
    ("recommended_plugins", "<recommended_plugins>", "</recommended_plugins>"),
)


def context_marker(text: str) -> str:
    """Return the name of the harness wrapper `text` is, or "".

    Like Codex's own `matches_marked_text`, the block must start with the open marker after
    leading whitespace and end with the close marker before trailing whitespace, ignoring ASCII
    case, so a tag typed mid-message does not count.
    """
    body = text.strip().lower()
    for name, start, end in CONTEXT_MARKERS:
        if body.startswith(start.lower()) and body.endswith(end.lower()):
            return name
    return ""


def user_parts(payload: dict) -> list[tuple[str, str]]:
    """Return the (kind, text) runs of a `role: "user"` message.

    The kind is "" for input the person supplied (`user.*`, or `unknown`, which Codex itself
    treats as possibly the user's) and the content kind for harness context. Without a
    `content_item_kinds` list as long as `content`, each block is classified by
    `context_marker`, and the kind is the marker name.
    """
    content = payload.get("content")
    if isinstance(content, str):
        content = [{"type": "input_text", "text": content}]
    if not isinstance(content, list):
        text = text_of(content)
        return [("", text)] if text else []
    metadata = payload.get("internal_chat_message_metadata_passthrough")
    kinds = metadata.get("content_item_kinds") if isinstance(metadata, dict) else None
    use_kinds = isinstance(kinds, list) and len(kinds) == len(content)
    parts: list[tuple[str, str]] = []
    for i, block in enumerate(content):
        text = text_of([block])
        if not text:
            continue
        if use_kinds:
            kind = str(cast(list, kinds)[i] or "")
            key = "" if kind.startswith("user.") or kind in USER_KINDS else kind
        else:
            key = context_marker(text)
        if parts and parts[-1][0] == key:
            parts[-1] = (key, parts[-1][1] + "\n" + text)
        else:
            parts.append((key, text))
    return parts


def _parent_of(metadata: dict) -> str:
    """Return the parent thread of a subagent's `session_meta`, or "".

    It is `parent_thread_id`, else `source.subagent.thread_spawn.parent_thread_id`, else a root
    `session_id` that differs from `id` when the thread is not a fork.
    """
    parent = metadata.get("parent_thread_id")
    if parent:
        return str(parent)
    source = metadata.get("source")
    subagent = source.get("subagent") if isinstance(source, dict) else None
    spawn = subagent.get("thread_spawn") if isinstance(subagent, dict) else None
    if isinstance(spawn, dict) and spawn.get("parent_thread_id"):
        return str(spawn["parent_thread_id"])
    root = metadata.get("session_id")
    if (
        root
        and metadata.get("id")
        and root != metadata.get("id")
        and not metadata.get("forked_from_id")
    ):
        return str(root)
    return ""


def _is_subagent(metadata: dict) -> bool:
    source = metadata.get("source")
    return metadata.get("thread_source") == "subagent" or (
        isinstance(source, dict) and "subagent" in source
    )


def _prescan(records: list) -> tuple[set, dict]:
    """Return the ids every response_item carries and the exit code of each command item.

    The exit codes are keyed by the `CommandExecution` turn item's id, which is the call's
    `call_id`.
    """
    carried: set = set()
    exit_codes: dict = {}
    for _line_number, record in records:
        payload = record.get("payload")
        if not isinstance(payload, dict):
            continue
        if record.get("type") == "response_item":
            for key in ("id", "call_id"):
                if payload.get(key):
                    carried.add(str(payload[key]))
        elif record.get("type") == "event_msg" and payload.get("type") == "item_completed":
            item = payload.get("item")
            if (
                isinstance(item, dict)
                and item.get("type") == "CommandExecution"
                and item.get("exit_code") is not None
            ):
                exit_codes[str(item.get("id") or "")] = item["exit_code"]
    carried.discard("")
    return carried, exit_codes


# `turn_context` and `thread_settings_applied` policy fields, in the order of
# the `permission mode:` row, as (label, payload key); research/codex-cli.md,
# Record schema. New fields go at the end, since the row text is an interface.
POLICY_FIELDS = (
    ("approval", "approval_policy"),
    ("sandbox", "sandbox_policy"),
    ("profile", "permission_profile"),
    ("network", "network"),
    ("reviewer", "approvals_reviewer"),
    ("profile_id", "active_permission_profile"),
)
POLICY_KEYS = tuple(key for _label, key in POLICY_FIELDS)
# The fields a `thread_settings_applied` event is the authority for: the
# thread's approval policy, reviewer and access level. Its `permission_profile`
# is the thread's, without the roots a turn adds, and it has no
# `sandbox_policy`; both stay with the last `turn_context`.
SETTINGS_POLICY_KEYS = ("approval_policy", "approvals_reviewer", "active_permission_profile")


def _policy_value(key: str, value) -> object:
    """Return a policy field as the composite shows it.

    An `active_permission_profile` object shows as its `id`, any other value as it is.
    """
    if key == "active_permission_profile" and isinstance(value, dict):
        return value.get("id")
    return value


def policy_text(payload: dict) -> str:
    """Return the composite permission mode of a policy payload, or "" when it has no field.

    The text is `approval=<p> sandbox=<s> profile=<pp> network=<n> reviewer=<r> profile_id=<id>`,
    with `-` for a missing field.
    """
    pairs = [(label, _policy_value(key, payload.get(key))) for label, key in POLICY_FIELDS]
    if all(value in (None, "") for _label, value in pairs):
        return ""
    return " ".join(
        "%s=%s" % (label, "-" if value in (None, "") else setting_value(value))
        for label, value in pairs
    )


def settings_model(settings: dict) -> str:
    """Return the model a `thread_settings` object names, or "".

    It is `model`, else `collaboration_mode.settings.model`.
    """
    if settings.get("model"):
        return str(settings["model"])
    mode = settings.get("collaboration_mode")
    inner = mode.get("settings") if isinstance(mode, dict) else None
    if isinstance(inner, dict) and inner.get("model"):
        return str(inner["model"])
    return ""


def guardian_verdict(message) -> dict | None:
    """Return the verdict a guardian review's `last_agent_message` holds, else None.

    The verdict is a JSON object string with `outcome` (and `risk_level`, `user_authorization`,
    `rationale`).
    """
    if not isinstance(message, str):
        return None
    try:
        verdict = json.loads(message)
    except ValueError:
        return None
    if isinstance(verdict, dict) and verdict.get("outcome"):
        return verdict
    return None


def guardian_text(verdict: dict, call: str, reviewer: str) -> str:
    """The `permission decision:` text of a guardian review verdict."""
    return (
        "permission decision: %s (auto_review risk_level=%s user_authorization=%s) exec %s"
        " reviewer %s: %s"
        % (
            verdict.get("outcome"),
            verdict.get("risk_level") or "-",
            verdict.get("user_authorization") or "-",
            call,
            reviewer,
            verdict.get("rationale") or "",
        )
    )


def _is_guardian(metadata: dict) -> bool:
    """Return whether a `session_meta` is a guardian review thread's.

    A guardian review thread has `thread_source` `guardian_review`, or a `source.subagent.other`
    of `guardian`.
    """
    if metadata.get("thread_source") == "guardian_review":
        return True
    source = metadata.get("source")
    subagent = source.get("subagent") if isinstance(source, dict) else None
    return isinstance(subagent, dict) and subagent.get("other") == "guardian"


def _with_exit(text: str, code) -> str:
    if code in (None, 0):
        return text
    return "[exit %s] %s" % (code, text)


def _tool_names(tools) -> str:
    if not isinstance(tools, list):
        return compact_json(tools)
    names = [
        str(tool.get("name")) if isinstance(tool, dict) and tool.get("name") else compact_json(tool)
        for tool in tools
    ]
    return ", ".join(names)


def _agent_label(payload: dict) -> str:
    recipients = [str(payload.get("recipient") or "")]
    others = payload.get("other_recipients")
    if isinstance(others, list):
        recipients += [str(recipient) for recipient in others]
    return "agent message %s -> %s: " % (payload.get("author") or "", ", ".join(recipients))


def _agent_text(payload: dict) -> str:
    content = payload.get("content")
    text = text_of(content)
    if isinstance(content, list) and any(
        isinstance(block, dict) and block.get("type") == "encrypted_content" for block in content
    ):
        text = (text + " [encrypted]").strip()
    return text


def _result_text(value) -> str:
    """Text of an MCP `CallToolResult` or dynamic tool output."""
    if isinstance(value, dict):
        if "content" in value:
            return text_of(value.get("content"))
        return compact_json(value)
    return text_of(value)


def item_rows(item: dict, payload: dict) -> list[tuple[str, object, dict, str]]:
    """Return the rows for an `item_completed` turn item that no response_item carries.

    Each row is (turn_type, timestamp, {tool_name, tool_use_id}, text). A tool item becomes a
    tool_use at `started_at_ms` and a tool_result at `completed_at_ms`; anything else one system
    row.
    """
    item_type = str(item.get("type") or "")
    item_id = str(item.get("id") or "")
    started = payload.get("started_at_ms")
    completed = payload.get("completed_at_ms")

    def pair(name: str, call: str, result: str) -> list:
        ids = {"tool_name": name, "tool_use_id": item_id}
        return [
            ("tool_use", started, ids, call),
            ("tool_result", completed, {"tool_use_id": item_id}, result),
        ]

    def failure() -> str:
        error = item.get("error")
        if isinstance(error, dict):
            return "[error] %s" % (error.get("message") or compact_json(error))
        return "[error] %s" % error if error else ""

    if item_type == "CommandExecution":
        command = item.get("command")
        call = (
            " ".join(str(part) for part in command)
            if isinstance(command, list)
            else str(command or "")
        )
        out = _with_exit(str(item.get("aggregated_output") or ""), item.get("exit_code"))
        return pair("shell", call, out)
    if item_type == "McpToolCall":
        name = "%s.%s" % (item.get("server") or "", item.get("tool") or "")
        result = _result_text(item.get("result")) if item.get("result") else failure()
        return pair(name, compact_json(item.get("arguments")), result)
    if item_type == "DynamicToolCall":
        result = _result_text(item.get("content_items")) or failure()
        return pair(str(item.get("tool") or ""), compact_json(item.get("arguments")), result)
    if item_type == "FileChange":
        changes = item.get("changes")
        paths = ", ".join(sorted(changes)) if isinstance(changes, dict) else ""
        out = "\n".join(str(item.get(key)) for key in ("stdout", "stderr") if item.get(key))
        return pair("apply_patch", paths, out or str(item.get("status") or ""))
    if item_type == "FunctionCallOutput":
        ids = {"tool_name": str(item.get("name") or ""), "tool_use_id": item_id}
        return [("tool_result", completed, ids, text_of(item.get("output")))]
    if item_type == "WebSearch" or (item_type == "Extension" and item.get("kind") == "web.search"):
        results = item.get("results")
        return pair("web_search", str(item.get("query") or ""), compact_json(results or []))
    if item_type == "Extension" and item.get("kind") == "clock.sleep":
        return [("system", completed, {}, "sleep: %s ms" % item.get("durationMs", ""))]
    if item_type in ("ImageGeneration", "Extension"):
        prompt = item.get("revised_prompt") or item.get("revisedPrompt") or ""
        saved = item.get("saved_path") or item.get("savedPath") or ""
        text = "image generation: %s status=%s" % (prompt, item.get("status") or "")
        if item_type == "Extension" and item.get("kind") != "image_gen.generation":
            rest = {key: value for key, value in item.items() if key not in ("type", "id")}
            text = "item Extension: " + compact_json(rest)
        elif saved:
            text += " saved=%s" % saved
        return [("system", completed, {}, text)]
    if item_type == "Plan":
        return [("system", completed, {}, "plan: %s" % (item.get("text") or ""))]
    if item_type == "SubAgentActivity":
        return [
            (
                "system",
                completed,
                {},
                "subagent activity: %s %s %s"
                % (
                    item.get("kind") or "",
                    item.get("agent_path") or "",
                    item.get("agent_thread_id") or "",
                ),
            )
        ]
    rest = {key: value for key, value in item.items() if key not in ("type", "id", "result")}
    return [("system", completed, {}, "item %s: %s" % (item_type, compact_json(rest)))]


def goal_text(payload: dict) -> str:
    """Return the `thread goal:` text of a `thread_goal_updated` event.

    The event's keys are camelCase (`turnId`, `goal.objective`, `goal.status`).
    """
    goal = payload.get("goal")
    goal = goal if isinstance(goal, dict) else {}
    return "thread goal: status=%s turn=%s objective: %s" % (
        goal.get("status") or "-",
        payload.get("turnId") or "-",
        goal.get("objective") or "",
    )


def rule_statements(text: str, errors: list) -> list[tuple[int, str]]:
    """Return (first line, text) of each top-level statement of a Starlark exec policy file.

    A statement (`prefix_rule(...)`, `network_rule(...)` or any other call) is joined onto one
    line, comments dropped. One still open at the end of the file (an unclosed bracket or string)
    goes to `errors` as (line, message, the statement's lines as written).
    """
    out: list[tuple[int, str]] = []
    depth = 0
    quote = ""
    start = 0
    parts: list[str] = []
    raw: list[str] = []
    for line_number, line in enumerate(split_lines(text), 1):
        kept = ""
        i = 0
        while i < len(line):
            character = line[i]
            if quote:
                if character == "\\":
                    kept += line[i : i + 2]
                    i += 2
                    continue
                if character == quote:
                    quote = ""
            elif character == "#":
                break
            elif character in "\"'":
                quote = character
            elif character in "([{":
                depth += 1
            elif character in ")]}":
                depth -= 1
            kept += character
            i += 1
        if kept.strip(ASCII_SPACE) or parts:
            if not parts:
                start = line_number
            parts.append(kept.strip(ASCII_SPACE))
            raw.append(line)
        if parts and depth <= 0 and not quote:
            out.append((start, " ".join(part for part in parts if part)))
            parts, raw, depth = [], [], 0
    if parts:
        errors.append((start, "unterminated rule", "\n".join(raw)))
    return out


# Standing permission keys of Codex's `config.toml`, as dotted paths with
# `*` for one table name (research/codex-cli.md, Standing grants and approval
# records): the policy defaults, a project's trust level, and the
# `approval_mode` an "approve for good" answer writes for an MCP or app tool.
CONFIG_PERMISSION_PATHS = (
    ("approval_policy",),
    ("approvals_reviewer",),
    ("sandbox_mode",),
    ("default_permissions",),
    ("projects", "*", "trust_level"),
    ("mcp_servers", "*", "tools", "*", "approval_mode"),
    ("plugins", "*", "mcp_servers", "*", "tools", "*", "approval_mode"),
    ("apps", "*", "tools", "*", "approval_mode"),
)


def config_settings(document: dict) -> list[tuple[list[str], object]]:
    """Return (key parts, value) of every `CONFIG_PERMISSION_PATHS` entry `document` holds.

    The entries come path by path in that order, tables in file order.
    """
    found: list[tuple[list[str], object]] = []

    def walk(node: object, path: tuple[str, ...], parts: list[str]) -> None:
        if not path:
            if node is not None and not isinstance(node, dict):
                found.append((parts, node))
            return
        if not isinstance(node, dict):
            return
        if path[0] == "*":
            for key, child in node.items():
                walk(child, path[1:], [*parts, str(key)])
        elif path[0] in node:
            walk(node[path[0]], path[1:], [*parts, path[0]])

    for path in CONFIG_PERMISSION_PATHS:
        walk(document, path, [])
    return found


def memory_summary(text: str) -> tuple[dict[str, str], str]:
    """Return the header lines and the summary of a `memories/rollout_summaries/<stem>.md` file.

    The header (`thread_id:`, `updated_at:`, `rollout_path:`, `cwd:`, `git_branch:`) runs up to
    the first blank line.
    """
    header: dict[str, str] = {}
    lines = split_lines(text)
    for i, line in enumerate(lines):
        if not line.strip(ASCII_SPACE):
            return header, "\n".join(lines[i + 1 :])
        key, separator, value = line.partition(":")
        key = key.strip(ASCII_SPACE)
        if not separator or key not in MEMORY_HEADER_KEYS:
            return header, "\n".join(lines[i:])
        header[key] = value.strip(ASCII_SPACE)
    return header, ""


MEMORY_HEADER_KEYS = ("thread_id", "updated_at", "rollout_path", "cwd", "git_branch")


class RolloutParser(Parser):
    """Parse Codex rollouts and prompt history under `home_prefix`.

    Codex forks that keep the format subclass this with their own agent and home.
    """

    has_secondary = True
    home_prefix = ".codex"
    ledger_name = ""  # import ledger file under home_prefix; empty when the agent has none
    automations = False  # read `automations/<slug>/automation.toml` (the ChatGPT desktop app)
    global_state = False  # read `.codex-global-state.json` (the ChatGPT desktop app)
    # A `/feedback` report in `logs_<N>.sqlite` marks a thread sent to Sentry.
    records_sync = True

    def __init__(self) -> None:
        """Compile the path patterns for `home_prefix`."""
        self.rollout_rx = re.compile(ROLLOUT_TMPL % re.escape(self.home_prefix))
        self.automation_rx = re.compile(AUTOMATION_TMPL % re.escape(self.home_prefix))
        self.logs_rx = re.compile(LOGS_TMPL % re.escape(self.home_prefix))
        self.rules_rx = re.compile(RULES_TMPL % re.escape(self.home_prefix))
        self.memory_rx = re.compile(MEMORY_SUMMARY_TMPL % re.escape(self.home_prefix))
        self.backup_state_rx = re.compile(BACKUP_STATE_TMPL % re.escape(self.home_prefix))
        self.config_rel = self.home_prefix + "/config.toml"
        self.history_rel = self.home_prefix + "/history.jsonl"
        self.state_rel = self.home_prefix + "/state_5.sqlite"
        self.thread_history_rel = self.home_prefix + "/thread_history_1.sqlite"
        self.ledger_rel = self.home_prefix + "/" + self.ledger_name if self.ledger_name else ""
        self.global_state_rel = (
            self.home_prefix + "/" + GLOBAL_STATE_NAME if self.global_state else ""
        )

    def wants(self, artifact: Artifact) -> bool:
        """Return whether the artifact is one of the files this parser reads."""
        rel = artifact.rel
        return (
            rel in (self.history_rel, self.state_rel, self.thread_history_rel)
            or bool(self.rollout_rx.match(rel))
            or bool(self.ledger_rel and rel == self.ledger_rel)
            or bool(self.automations and self.automation_rx.match(rel))
            or bool(self.logs_rx.match(rel))
            or bool(self.global_state_rel and rel == self.global_state_rel)
            or rel == self.config_rel
            or bool(self.rules_rx.match(rel))
            or bool(self.memory_rx.match(rel))
            or bool(self.backup_state_rx.match(rel))
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return whether the artifact repeats events the rollouts record."""
        rel = artifact.rel
        return (
            rel in (self.history_rel, self.state_rel, self.thread_history_rel)
            or bool(self.global_state_rel and rel == self.global_state_rel)
            or bool(self.backup_state_rx.match(rel))
        )

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        # codex_state imports item_rows from this module, so it is imported here.
        """Yield the rows of one artifact, by the file it is."""
        from .codex_state import parse_global_state, parse_logs, parse_state, parse_thread_history

        if artifact.rel == self.history_rel:
            yield from self._parse_history(artifact, opts)
        elif artifact.rel == self.state_rel or self.backup_state_rx.match(artifact.rel):
            yield from parse_state(self, artifact, opts)
        elif artifact.rel == self.thread_history_rel:
            yield from parse_thread_history(self, artifact, opts)
        elif self.automations and self.automation_rx.match(artifact.rel):
            yield from self._parse_automation(artifact)
        elif self.logs_rx.match(artifact.rel):
            yield from parse_logs(self, artifact)
        elif self.ledger_rel and artifact.rel == self.ledger_rel:
            yield from self._parse_ledger(artifact, opts)
        elif self.global_state_rel and artifact.rel == self.global_state_rel:
            yield from parse_global_state(self, artifact)
        elif artifact.rel == self.config_rel:
            yield from self._parse_config(artifact)
        elif self.rules_rx.match(artifact.rel):
            yield from self._parse_rules(artifact)
        elif self.memory_rx.match(artifact.rel):
            yield from self._parse_memory_summary(artifact)
        else:
            yield from self._parse_rollout(artifact, opts)

    def _system(self, artifact: Artifact, text: str, session_id: str = "", line: int = 0) -> Row:
        row = self.base_row(artifact)
        row.turn_type = "system"
        row.session_id = session_id
        row.source_line = line
        row.text = text
        return row

    def _parse_ledger(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors)
            return
        try:
            data = json.loads(content.decode("utf-8", errors="replace"))
        except ValueError as error:
            yield self._system(
                artifact, "parser: unreadable import ledger: %s" % error_reason(error)
            )
            return
        records = data.get("records") if isinstance(data, dict) else None
        if not isinstance(records, list):
            yield self._system(artifact, "parser: import ledger has no records list")
            return
        for i, record in enumerate(records, 1):
            if not isinstance(record, dict):
                continue
            # source_line is the record's position in `records`, not a file line.
            row = self._system(artifact, "", str(record.get("imported_thread_id") or ""), i)
            row.timestamp_utc = to_utc(record.get("imported_at"))
            text = "imported session from %s sha256=%s" % (
                record.get("source_path") or "",
                record.get("content_sha256") or "",
            )
            modified = to_utc(record.get("source_modified_at"))
            if modified:
                text += " source_modified=%s" % modified
            row.text = compact(text)
            yield row

    def _parse_automation(self, artifact: Artifact) -> Iterator[Row]:
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors)
            return
        try:
            document = load_toml(content.decode("utf-8", errors="replace"))
        except ValueError as error:
            yield self._system(
                artifact, "parser: unreadable automation.toml: %s" % error_reason(error)
            )
            return

        def field(key: str) -> str:
            value = document.get(key)
            return "" if value is None else str(value)

        row = self._system(artifact, "", field("target_thread_id"), 1)
        row.timestamp_utc = file_mtime(artifact)
        row.text = (
            "scheduled task: %s name=%s kind=%s status=%s rrule=%s target_thread=%s "
            "created=%s updated=%s (from %s, file mtime) prompt: %s"
            % (
                field("id"),
                field("name"),
                field("kind"),
                field("status"),
                field("rrule"),
                field("target_thread_id"),
                to_utc(document.get("created_at")),
                to_utc(document.get("updated_at")),
                artifact.rel,
                field("prompt"),
            )
        )
        yield row

    def _setting(self, artifact: Artifact, text: str, line: int, project: str) -> Row:
        """A standing `permission setting:` row at the file's modification time."""
        row = self._system(artifact, "", "", line)
        row.timestamp_utc = file_mtime(artifact)
        row.project_path = project
        row.text = compact("permission setting: %s (from %s, file mtime)" % (text, artifact.rel))
        return row

    def _parse_config(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `permission setting:` row per standing permission key of `config.toml`.

        A project's trust level carries the project as `project_path`, and a project's own
        `.codex/config.toml` the project.
        """
        base = artifact.home.original if artifact.home.project else ""
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors, "", base)
            return
        try:
            document = load_config_toml(content.decode("utf-8", errors="replace"))
        except ValueError as error:
            yield self._system(artifact, "parser: unreadable config.toml: %s" % error_reason(error))
            return
        for parts, value in config_settings(document):
            project = parts[1] if parts[0] == "projects" else base
            text = "%s=%s" % (".".join(parts), setting_value(value))
            yield self._setting(artifact, text, 0, project)

    def _parse_rules(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `permission setting:` row per statement of a `rules/` exec policy file."""
        base = artifact.home.original if artifact.home.project else ""
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors, "", base)
            return
        text = content.decode("utf-8", errors="replace")
        for line, statement in rule_statements(text, errors):
            yield self._setting(artifact, statement, line, base)
        yield from error_rows(self, artifact, errors, "", base)

    def _parse_memory_summary(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `memory summary:` row for a thread's summary file, at its `updated_at`."""
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors)
            return
        text = content.decode("utf-8", errors="replace")
        header, body = memory_summary(text)
        if not header.get("thread_id"):
            yield self._system(artifact, "parser: memory summary has no thread_id header")
            return
        row = self._system(artifact, "", header["thread_id"], 1)
        row.timestamp_utc = to_utc(header.get("updated_at"))
        row.project_path = header.get("cwd", "")
        row.git_branch = header.get("git_branch", "")
        row.text = compact(
            "memory summary: thread %s rollout=%s (from %s, updated_at): %s"
            % (header["thread_id"], header.get("rollout_path") or "-", artifact.rel, body.strip())
        )
        yield row

    def _parse_history(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            row = self.base_row(artifact)
            row.source_line = line_number
            row.timestamp_utc = to_utc(record.get("ts"))
            row.session_id = str(record.get("session_id") or "")
            row.turn_type = "user"
            row.text = compact(record.get("text"))
            yield row
        yield from error_rows(self, artifact, errors)

    def _parse_rollout(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        match = UUID_TAIL_PATTERN.search(artifact.rel)
        session_id = match.group(1) if match else ""  # until session_meta says otherwise
        if artifact.rel.endswith(".zst"):
            records = list(iter_zst_jsonl(artifact.disk_path, errors))
        else:
            records = list(iter_jsonl(artifact.disk_path, errors))
        carried, exit_codes = _prescan(records)
        cwd = ""
        model = ""
        branch = ""
        meta_seen = False
        seen_model: dict = {}  # session id -> last model, for the model rows
        seen_mode: dict = {}  # session id -> last composite permission mode
        policy: dict = {}  # session id -> the policy fields last recorded
        task_pending = False  # a subagent file whose first prompt is still to come
        guardian_parent = ""  # the parent thread of a guardian review thread
        started = ""  # the guardian thread's session_meta time
        for line_number, record in records:
            record_type = record.get("type")
            payload = record.get("payload")
            if not isinstance(payload, dict):
                payload = {}

            def base(turn_type: str = "system", timestamp=None) -> Row:
                row = self.base_row(artifact)
                row.source_line = line_number
                row.timestamp_utc = to_utc(timestamp) or to_utc(record.get("timestamp"))
                row.session_id = session_id
                row.project_path = cwd
                row.git_branch = branch
                row.model = model
                row.turn_type = turn_type
                return row

            def prompt(text: str) -> Row:
                """A prompt that is not context: the person's, or a subagent's task."""
                nonlocal task_pending
                if task_pending:
                    task_pending = False
                    return self._fill(base(), "subagent task: " + text, opts)
                return self._fill(base("user"), text, opts)

            if record_type == "session_meta":
                thread_id = str(payload.get("id") or payload.get("session_id") or "")
                if meta_seen:
                    if thread_id and thread_id != session_id:
                        yield self._fill(
                            base(timestamp=payload.get("timestamp")),
                            "copied from ancestor: session start %s %s id=%s cwd=%s"
                            % (
                                payload.get("originator") or "codex",
                                payload.get("cli_version") or "",
                                thread_id,
                                payload.get("cwd") or "",
                            ),
                            opts,
                        )
                    continue
                meta_seen = True
                session_id = thread_id or session_id
                cwd = str(payload.get("cwd") or cwd)
                git = payload.get("git") or {}
                if not isinstance(git, dict):
                    git = {}
                branch = str(git.get("branch") or branch)
                start = "session start: %s %s provider=%s cwd=%s" % (
                    payload.get("originator") or "codex",
                    payload.get("cli_version") or "",
                    payload.get("model_provider") or "",
                    cwd,
                )
                for key, label in (("creator_account_id", "account"), ("creator_user_id", "user")):
                    if payload.get(key):
                        start += " %s=%s" % (label, payload.get(key))
                yield self._fill(base(timestamp=payload.get("timestamp")), start, opts)
                if isinstance(git.get("repository_url"), str) and git["repository_url"].strip():
                    yield self._fill(
                        base(timestamp=payload.get("timestamp")),
                        GIT_REMOTE_PREFIX + git["repository_url"],
                        opts,
                    )
                parent = _parent_of(payload)
                if parent:
                    extra = " ".join(
                        "%s=%s" % (field, payload.get(field))
                        for field in ("agent_path", "agent_role", "agent_nickname")
                        if payload.get(field)
                    )
                    yield self._fill(
                        base(timestamp=payload.get("timestamp")),
                        ("subagent %s of %s %s" % (session_id, parent, extra)).strip(),
                        opts,
                    )
                if payload.get("forked_from_id"):
                    yield self._fill(
                        base(timestamp=payload.get("timestamp")),
                        "forked from %s" % payload.get("forked_from_id"),
                        opts,
                    )
                task_pending = bool(parent) or _is_subagent(payload)
                if _is_guardian(payload) and parent:
                    guardian_parent = parent
                    started = str(payload.get("timestamp") or record.get("timestamp") or "")
            elif record_type == "turn_context":
                cwd = str(payload.get("cwd") or cwd)
                model = str(payload.get("model") or model)
                policy[session_id] = {key: payload.get(key) for key in POLICY_KEYS}
                for text in (
                    track_change("model", seen_model, session_id, str(payload.get("model") or "")),
                    track_change(
                        "permission mode", seen_mode, session_id, policy_text(policy[session_id])
                    ),
                ):
                    if text:
                        yield self._fill(base(), text, opts)
            elif record_type == "response_item":
                metadata = record.get("metadata")
                if isinstance(metadata, dict) and metadata.get("inherited_user_message"):
                    continue  # copied from the parent thread, which records it itself
                payload_type = payload.get("type")
                if payload_type == "message":
                    role = payload.get("role")
                    if role == "user":
                        for kind, text in user_parts(payload):
                            if not kind:
                                yield prompt(text)
                            else:
                                yield self._fill(base(), "context: %s: %s" % (kind, text), opts)
                        continue
                    text = text_of(payload.get("content"))
                    if not text:
                        continue
                    if role == "assistant":
                        yield self._fill(base("assistant"), text, opts)
                    else:
                        yield self._fill(base(), "%s: %s" % (role, text), opts)
                elif payload_type in CALL_TYPES:
                    row = base("tool_use")
                    row.tool_name = call_name(payload)
                    row.tool_use_id = str(payload.get("call_id") or payload.get("id") or "")
                    yield self._fill(row, call_summary(payload), opts)
                elif payload_type in OUTPUT_TYPES:
                    row = base("tool_result")
                    row.tool_use_id = str(payload.get("call_id") or "")
                    text = text_of(payload.get("output"))
                    yield self._fill(row, _with_exit(text, exit_codes.get(row.tool_use_id)), opts)
                elif payload_type == "tool_search_call":
                    row = base("tool_use")
                    row.tool_name = "tool_search"
                    row.tool_use_id = str(payload.get("call_id") or payload.get("id") or "")
                    yield self._fill(row, compact_json(payload.get("arguments")), opts)
                elif payload_type == "tool_search_output":
                    row = base("tool_result")
                    row.tool_use_id = str(payload.get("call_id") or "")
                    yield self._fill(row, _tool_names(payload.get("tools")), opts)
                elif payload_type == "agent_message":
                    text = _agent_text(payload)
                    if task_pending:
                        yield prompt(text)
                    else:
                        yield self._fill(base(), _agent_label(payload) + text, opts)
                elif payload_type == "image_generation_call":
                    yield self._fill(
                        base(),
                        "image generation: %s status=%s"
                        % (payload.get("revised_prompt") or "", payload.get("status") or ""),
                        opts,
                    )
                elif payload_type == "reasoning" and opts.include_thinking:
                    text = text_of(payload.get("summary")) or text_of(payload.get("content"))
                    if text:
                        yield self._fill(base("thinking"), text, opts)
            elif record_type == "event_msg":
                payload_type = payload.get("type")
                if payload_type == "task_started":
                    yield self._fill(
                        base(), "task_started turn=%s" % (payload.get("turn_id") or ""), opts
                    )
                    attribution = payload.get("turn_attribution")
                    if isinstance(attribution, dict) and attribution.get("turn_trigger"):
                        turn = payload.get("turn_id") or attribution.get("turn_id") or ""
                        text = "turn trigger: %s turn=%s" % (attribution["turn_trigger"], turn)
                        yield self._fill(base(), text, opts)
                elif payload_type == "task_complete":
                    yield self._fill(
                        base(),
                        "task_complete turn=%s duration_ms=%s"
                        % (payload.get("turn_id") or "", payload.get("duration_ms") or ""),
                        opts,
                    )
                    verdict = guardian_verdict(payload.get("last_agent_message"))
                    if guardian_parent and verdict:
                        row = base()
                        row.session_id = guardian_parent
                        row.model = ""  # the reviewer's model is not the parent thread's
                        call = self._guardian_call(artifact, guardian_parent, started, row)
                        row.tool_use_id = call
                        yield self._fill(row, guardian_text(verdict, call or "-", session_id), opts)
                elif payload_type == "thread_settings_applied":
                    settings = payload.get("thread_settings")
                    if not isinstance(settings, dict):
                        continue
                    new_model = settings_model(settings)
                    model = new_model or model
                    texts = [track_change("model", seen_model, session_id, new_model)]
                    previous = policy.get(session_id)
                    if previous is not None:
                        # Before the thread's first turn_context the event's policy is
                        # folded into that turn's row. The event is the thread's
                        # settings, not a turn's: it carries no sandbox_policy, and
                        # its permission_profile lacks the per-turn roots the
                        # turn_context adds, so only the three thread-level fields
                        # come from it; the two turn-level fields are carried over,
                        # or unknown (`-`) once the access level changed.
                        state = dict(previous)
                        for key in SETTINGS_POLICY_KEYS:
                            if key in settings:
                                state[key] = settings[key]
                        old_id = _policy_value(
                            "active_permission_profile", previous.get("active_permission_profile")
                        )
                        new_id = _policy_value(
                            "active_permission_profile", state.get("active_permission_profile")
                        )
                        if new_id != old_id:
                            state["sandbox_policy"] = None
                            state["permission_profile"] = None
                        policy[session_id] = state
                        texts.append(
                            track_change(
                                "permission mode", seen_mode, session_id, policy_text(state)
                            )
                        )
                    for text in texts:
                        if text:
                            yield self._fill(base(), text, opts)
                elif payload_type == "thread_goal_updated":
                    yield self._fill(base(), goal_text(payload), opts)
                elif payload_type == "turn_aborted":
                    error = payload.get("error")
                    detail = error.get("message") if isinstance(error, dict) else ""
                    text = "turn aborted: %s" % (payload.get("reason") or "")
                    yield self._fill(base(), ("%s %s" % (text, detail or "")).strip(), opts)
                elif payload_type == "thread_rolled_back":
                    yield self._fill(
                        base(), "rolled back %s turns" % payload.get("num_turns", ""), opts
                    )
                elif payload_type == "item_completed":
                    item = payload.get("item")
                    if not isinstance(item, dict):
                        continue
                    if item.get("type") in RESPONSE_CARRIED_ITEMS:
                        continue
                    if str(item.get("id") or "") in carried:
                        continue
                    for turn_type, timestamp, fields, text in item_rows(item, payload):
                        row = base(turn_type, timestamp)
                        row.tool_name = fields.get("tool_name", "")
                        row.tool_use_id = fields.get("tool_use_id", "")
                        yield self._fill(row, text, opts)
            elif record_type == "compacted":
                message = payload.get("message")
                if isinstance(message, str) and message:
                    yield self._fill(base(), "compaction summary: " + message, opts)
                else:
                    history = payload.get("replacement_history")
                    item_count = len(history) if isinstance(history, list) else 0
                    yield self._fill(
                        base(), "compaction: replacement history of %d items" % item_count, opts
                    )
            elif record_type == "inter_agent_communication":
                if str(payload.get("id") or "") in carried:
                    continue  # the agent_message response_item with this id wins
                text = payload.get("content")
                if not isinstance(text, str) or not text:
                    text = "[encrypted]" if payload.get("encrypted_content") else ""
                if task_pending:
                    yield prompt(text)
                else:
                    yield self._fill(base(), _agent_label(payload) + text, opts)
            elif record_type == "realtime_item":
                kind = payload.get("type")
                if kind == "transcript_segment" and payload.get("role") in ("user", "assistant"):
                    yield self._fill(
                        base(str(payload["role"])), str(payload.get("text") or ""), opts
                    )
                else:
                    rest = {
                        field: value
                        for field, value in payload.items()
                        if field not in ("type", "id", "text")
                    }
                    text = "realtime: %s %s" % (kind or "", compact_json(rest))
                    if payload.get("text"):
                        text += " " + str(payload["text"])
                    yield self._fill(base(), text, opts)
        yield from error_rows(self, artifact, errors, session_id)

    def _guardian_call(self, artifact: Artifact, parent: str, started: str, row: Row) -> str:
        """Return the item id of the call a guardian verdict at `row`'s time governs, or "".

        The id comes from the `guardian_review{...}` spans of the `logs_<N>.sqlite` files in the
        Codex home, and only when exactly one call of `parent` was reviewed between the review
        thread's start and its verdict.
        """
        from .codex_state import guardian_calls

        home = artifact.disk_path
        for _ in range(len(artifact.rel.split("/")) - len(self.home_prefix.split("/"))):
            home = home.parent
        calls = guardian_calls(home, parent, started or row.timestamp_utc, row.timestamp_utc)
        return calls[0] if len(calls) == 1 else ""

    @staticmethod
    def _fill(row: Row, text: str, opts: Options) -> Row:
        row.text = compact(text)
        return row


class CodexParser(RolloutParser):
    """Parse the Codex CLI and the Codex Desktop app under `~/.codex`."""

    agent = "codex-cli"
    name = "codex-cli"
    home_prefix = ".codex"
    automations = True
    global_state = True
