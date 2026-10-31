"""Agent Zero chats, scheduler tasks, plugin settings and connector state.

Reads `<a0>/usr/chats/<ctxid>/chat.json` with its `messages/<n>.txt` tool outputs (and the
older `<a0>/usr/chats/<ctxid>.json` and `<a0>/tmp/chats/` copies), where `<a0>` is
`[Desktop/]agent-zero[/<instance>]`, or the same files under `chats/` at the base of a
collected `/a0/usr` Docker or Podman volume. Beside them it reads `usr/scheduler/tasks.json`
and the Telegram plugin's `state.json` (both secondary sources), the `_tool_access` and
`_orchestrator` plugin `config.json` and `.toggle-0`/`.toggle-1` files, and the A0 CLI
connector's `~/.agent-zero/.env`. It yields the chat log as user, assistant, thinking,
tool_use, tool_result and system rows, and model, permission, sync, relay, origin and
scheduled-task rows.

Fields read: `id`, `name`, `created_at`, `type`, `agent_profile`; `data.project`,
`acp_cwd`, `acp_mode`, `acp_model_id`, `chat_model_override` (provider and name only),
`workdir_path` and the Telegram, WhatsApp and email keys; `log.logs[]` `no`, `id`, `type`,
`heading`, `content`, `kvps`, `timestamp`, `agentno`; `agents[].number`, `agents[].history`
(`Message` `id`, `ai`, `content`, `metadata.responses.provider_model_key`) and
`agents[].data.responses_state`; `tasks[]` `uuid`, `context_id`, `name`, `type`, `state`,
`prompt`, `project_name`, `created_at`, `last_run`, `last_result`; the Telegram `chats` map;
and only `AGENT_ZERO_LAST_CONTEXT_HOST` and `AGENT_ZERO_LAST_CONTEXT_ID` of the `.env`.
`data.telegram_bot_cfg`, a task's `token` and `system_prompt` are never read.

Validated against source (agent0ai/agent-zero at e3051fb and ff2a5b9, agent0ai/a0-connector
at 3741c88, agent0ai/a0-launcher at b0333d8) with a synthetic fixture; no real install.
Drift: log item ids equal history message ids (`helpers/tool.py:56-61`,
`agent.py:529-534`), which the research document's fixture gives as different ids. See
`research/agent-zero.md`.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterator

from ..inputs import Artifact
from ..model import Row, compact
from ..timeutil import to_utc
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_mtime,
    setting_value,
    text_of,
    track_change,
)

# A home install keeps the chats under `<a0>/usr/chats` (or the legacy
# `<a0>/tmp/chats`); a volume mounted at `/a0/usr` is collected with its
# `_data` directory as the home, so its chats start at `chats/`.
CHATS_ROOT = r"(?:(?:Desktop/)?agent-zero/(?:[^/]+/)?(?:usr|tmp)/)?chats"
CHAT_PATTERN = re.compile(r"^%s/[^/]+/chat\.json$" % CHATS_ROOT)
LEGACY_PATTERN = re.compile(r"^%s/[^/]+\.json$" % CHATS_ROOT)
# The other `usr/` files read beside the chats, in the same two forms (a home
# install's `<a0>/usr/` prefix, or none at a volume's base).
USR = r"^(?:(?:Desktop/)?agent-zero/(?:[^/]+/)?usr/)?"
TASKS_PATTERN = re.compile(
    r"^(?:(?:Desktop/)?agent-zero/(?:[^/]+/)?(?:usr|tmp)/)?scheduler/tasks\.json$"
)
TELEGRAM_STATE_PATTERN = re.compile(USR + r"plugins/_telegram_integration/state\.json$")
# A plugin's scoped directory: `usr/plugins/<plugin>`, a profile's
# `usr/agents/<profile>/plugins/<plugin>`, a project's
# `usr/projects/<name>/.a0proj/plugins/<plugin>` and a project profile's
# `.a0proj/agents/<profile>/plugins/<plugin>` (helpers/plugins.py:850-863).
SCOPE = r"(?:projects/(?P<project>[^/]+)/\.a0proj/)?(?:agents/(?P<profile>[^/]+)/)?plugins/"
POLICY_PATTERN = re.compile(USR + SCOPE + r"(?P<plugin>_tool_access|_orchestrator)/config\.json$")
TOGGLE_PATTERN = re.compile(
    USR + SCOPE + r"(?P<plugin>_infection_check|_orchestrator)/\.toggle-(?P<state>[01])$"
)
CONNECTOR_ENV_PATTERN = re.compile(r"^\.agent-zero/\.env$")

LOG_SIZE = 1000
TOOL_TYPES = ("tool", "mcp", "code_exe", "browser", "subagent")
ARG_KEYS = ("code", "command", "url", "query", "path")
ICON_PATTERN = re.compile(r"icon://\S+\s*")
LOGS_PATTERN = re.compile(r'"logs"[ \t\n\r]*:[ \t\n\r]*\[')
# The `data` keys of a Telegram chat, in row order (`telegram_bot_cfg` is never read).
TELEGRAM_KEYS = ("telegram_bot", "telegram_chat_id", "telegram_user_id", "telegram_username")
# The policy keys of `_tool_access` (plugins/_tool_access/default_config.yaml:1-5).
POLICY_SCALARS = ("mode", "default", "mcp_default")
POLICY_LISTS = ("allowed", "blocked")
# The approval bypass of each coding CLI the Orchestrator drives
# (plugins/_orchestrator/default_config.yaml:7-37); nothing else of the file
# is read.
ORCHESTRATOR_KEYS = (
    ("codex", "bypass_sandbox"),
    ("claude", "permission_mode"),
    ("claude", "allowed_tools"),
    ("cursor", "force"),
    ("grok", "always_approve"),
    ("hermes", "yolo"),
    ("opencode", "auto"),
)
# The message `ensure_tool_allowed` raises for a blocked tool, inside the
# formatted error of the `warning` log item (helpers/tool_policy.py:197-208).
BLOCKED_PATTERN = re.compile(
    r'Tool "(?P<tool>[^"]*)" is blocked for agent profile "(?P<profile>[^"]*)"\.'
)
MCP_APPROVAL_HEADING = "Responses MCP approval required"
# Infection Check warning headings (plugins/_infection_check/helpers/checker.py).
INFECTION_HEADINGS = (
    ("Infection check: requesting clarification", "clarify"),
    ("Infection check: clarification passed", "passed"),
    ("Infection check: TERMINATED", "terminated"),
)
# The connector `.env` keys read, and only these (a0-connector config.py:13-14).
LAST_CONTEXT_KEYS = ("AGENT_ZERO_LAST_CONTEXT_HOST", "AGENT_ZERO_LAST_CONTEXT_ID")
# A scheduler task's `last_run` is stamped when the run finishes, after the
# chat's `response` log item, so its result is matched in a wider window.
TASKS_TOLERANCE = 60.0
OVERRIDE_SLOTS = ("chat", "utility", "embedding")


def walk_history(node, summaries: bool) -> Iterator[tuple[str, dict]]:
    """Yield the history's messages and, with `summaries`, its summaries in order.

    Each item is ("message", Message) or ("summary", {"summary": text}): bulks, then
    topics, then the current topic.
    """
    if isinstance(node, list):
        for child in node:
            yield from walk_history(child, summaries)
        return
    if not isinstance(node, dict):
        return
    cls = node.get("_cls")
    if cls == "Message":
        yield "message", node
    elif cls == "History":
        for key in ("bulks", "topics", "current"):
            yield from walk_history(node.get(key), summaries)
    elif cls in ("Topic", "Bulk") or "messages" in node or "records" in node:
        if summaries and node.get("summary"):
            yield "summary", {"summary": node["summary"], "_cls": cls}
        yield from walk_history(node.get("messages") or node.get("records") or [], summaries)


def parse_ai(content) -> dict | None:
    """Return the assistant's raw output parsed as the agent's JSON response, or None."""
    if isinstance(content, dict):
        return content
    if isinstance(content, str):
        text = content.strip()
        if text.startswith("{"):
            try:
                value = json.loads(text)
            except ValueError:
                return None
            return value if isinstance(value, dict) else None
    return None


def headline_text(response: dict) -> str:
    """Return a response's headline and thoughts, one per line."""
    parts = []
    if response.get("headline"):
        parts.append(str(response["headline"]))
    thoughts = response.get("thoughts")
    if isinstance(thoughts, list):
        parts.extend(str(thought) for thought in thoughts if thought)
    elif thoughts:
        parts.append(str(thoughts))
    return "\n".join(parts)


def args_summary(args) -> str:
    """Return the argument that names a tool call's action, else the arguments as JSON."""
    if isinstance(args, dict):
        visible = {k: value for k, value in args.items() if not str(k).startswith("_")}
        for k in ARG_KEYS:
            if visible.get(k) not in (None, ""):
                return str(visible[k])
        return compact_json(visible)
    return args if isinstance(args, str) else compact_json(args)


def model_of(message: dict | None) -> str:
    """Return a history message's `provider_model_key`, or ""."""
    if not message:
        return ""
    meta = message.get("metadata")
    resp = meta.get("responses") if isinstance(meta, dict) else None
    return str(resp.get("provider_model_key") or "") if isinstance(resp, dict) else ""


def _content_text(value) -> str:
    if isinstance(value, (dict, list)) and not (isinstance(value, dict) and "text" in value):
        return text_of(value) if isinstance(value, list) else compact_json(value)
    return text_of(value)


def recover_logs(raw: str) -> list[dict]:
    """Return the complete log items before the point where a damaged file breaks off."""
    match = LOGS_PATTERN.search(raw)
    if not match:
        return []
    decoder = json.JSONDecoder()
    pos, out = match.end(), []
    while True:
        while pos < len(raw) and raw[pos] in " \t\r\n,":
            pos += 1
        try:
            item, pos = decoder.raw_decode(raw, pos)
        except ValueError:
            return out
        if not isinstance(item, dict):
            return out
        out.append(item)


def _key(record: dict, key: str) -> str:
    """Return `record[key]` as row text, `-` when absent or empty."""
    value = record.get(key)
    return "-" if value in (None, "") else str(value)


def remote_copy_texts(chat: dict, data: dict) -> list[str]:
    """Return the `session synced:` and `session relayed:` texts of a chat.

    A synced text names a provider response chain, a relayed text a Telegram, WhatsApp
    or email chat. `telegram_bot_cfg`, which holds the bot token, is never read.
    """
    out = []
    for i, agent in enumerate(chat.get("agents") or []):
        agent_data = agent.get("data") if isinstance(agent, dict) else None
        state = agent_data.get("responses_state") if isinstance(agent_data, dict) else None
        if not isinstance(state, dict) or state.get("previous_response_id") in (None, ""):
            continue
        ids = state.get("response_ids")
        out.append(
            "session synced: provider response %s previous %s model %s (%d response ids)"
            " agent=%s (from chat.json agents[].data.responses_state, file mtime)"
            % (
                _key(state, "response_id"),
                _key(state, "previous_response_id"),
                _key(state, "provider_model_key"),
                len(ids) if isinstance(ids, list) else 0,
                agent.get("number", i),
            )
        )
    if any(data.get(k) not in (None, "") for k in TELEGRAM_KEYS):
        out.append(
            "session relayed: telegram bot %s chat %s user %s @%s (from chat.json data, file mtime)"
            % tuple(_key(data, k) for k in TELEGRAM_KEYS)
        )
    if data.get("wa_chat_id") not in (None, ""):
        out.append(
            "session relayed: whatsapp chat %s number %s (from chat.json data, file mtime)"
            % (_key(data, "wa_chat_id"), _key(data, "wa_sender_number"))
        )
    if any(data.get(k) not in (None, "") for k in ("email_thread_id", "email_sender")):
        out.append(
            "session relayed: email thread %s from %s (from chat.json data, file mtime)"
            % (_key(data, "email_thread_id"), _key(data, "email_sender"))
        )
    return out


def _model_name(config: dict) -> str:
    """Return `<provider>/<name>` of a model configuration, or "" when it names neither.

    The `api_key` and `api_base` are never read.
    """
    parts = [str(config.get(k) or "").strip() for k in ("provider", "name")]
    return "/".join(part for part in parts if part)


def override_text(override) -> str:
    """Return the model `data.chat_model_override` requests, as row text.

    A preset is named by its name, a custom configuration by the provider and name of
    each slot. A custom configuration from the connector can carry an `api_key`
    (plugins/_a0_connector/api/v1/model_switcher.py:27-46), so the object itself is
    never printed.
    """
    if not isinstance(override, dict) or not override:
        return ""
    if override.get("preset_name"):
        return "preset %s" % override["preset_name"]
    slots = [
        "%s=%s" % (slot, _model_name(override[slot]))
        for slot in OVERRIDE_SLOTS
        if isinstance(override.get(slot), dict) and _model_name(override[slot])
    ]
    if slots:
        return " ".join(slots)
    return _model_name(override) or "custom configuration"


def chat_setting_rows(data: dict) -> list[tuple[str, str]]:
    """Return (text, project path or "") for each editor-session and chat-model row.

    The rows come from a chat's `data`, each value the latest as of the file's modification
    time; ACP is the Agent Client Protocol an editor session uses. Only the scratch
    directory row names its own project path.
    """
    out = []
    if data.get("acp_mode"):
        out.append(("permission mode: %s (acp, as of file mtime)" % data["acp_mode"], ""))
    if data.get("acp_model_id"):
        out.append(("model: requested %s (acp, as of file mtime)" % data["acp_model_id"], ""))
    requested = override_text(data.get("chat_model_override"))
    if requested:
        out.append(("model: requested %s (chat override, as of file mtime)" % requested, ""))
    workdir = data.get("workdir_path")
    if isinstance(workdir, str) and workdir.strip():
        out.append(
            (
                "scratch directory: %s (acp container transport, from chat.json"
                " data.workdir_path, as of file mtime)" % workdir.strip(),
                workdir.strip(),
            )
        )
    return out


def warning_decision(item: dict) -> tuple[str, str]:
    """Return (text, tool name) of a `warning` log item's `permission decision:` row.

    Returns ("", "") when the item records no decision.
    """
    heading = str(item.get("heading") or "")
    content = item.get("content")
    match = BLOCKED_PATTERN.search(content) if isinstance(content, str) else None
    if match:
        return (
            "permission decision: blocked tool=%s profile=%s" % (match["tool"], match["profile"]),
            match["tool"],
        )
    if MCP_APPROVAL_HEADING in heading:
        return (
            "permission decision: denied automatically (provider MCP approval request)",
            "mcp_approval_request",
        )
    for label, verdict in INFECTION_HEADINGS:
        if label in heading:
            return "permission decision: %s (infection check)" % verdict, ""
    return "", ""


def project_of(name) -> str:
    """Return the container path of an Agent Zero project, or "" for none."""
    return "/a0/usr/projects/%s" % name if name else ""


def scope_text(match: re.Match) -> str:
    """Return ` profile=<p> project=<n>` for a scoped plugin file, each part when set."""
    return "".join(" %s=%s" % (k, match[k]) for k in ("profile", "project") if match[k])


class AgentZeroParser(Parser):
    """Parse Agent Zero chats and the scheduler, plugin and connector files beside them."""

    agent = "agent-zero"
    name = "agent-zero"
    records_sync = True
    records_relay = True
    records_origin = True
    has_secondary = True

    def wants(self, artifact: Artifact) -> bool:
        """Return True for a chat, scheduler, Telegram, plugin or connector file."""
        rel = artifact.rel
        return any(
            pattern.match(rel)
            for pattern in (
                CHAT_PATTERN,
                LEGACY_PATTERN,
                TASKS_PATTERN,
                TELEGRAM_STATE_PATTERN,
                POLICY_PATTERN,
                TOGGLE_PATTERN,
                CONNECTOR_ENV_PATTERN,
            )
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for the scheduler's tasks and the Telegram chat map."""
        return bool(TASKS_PATTERN.match(artifact.rel) or TELEGRAM_STATE_PATTERN.match(artifact.rel))

    def secondary_tolerance(self, artifact: Artifact) -> float:
        """Return the wider window of the scheduler, whose `last_run` follows the reply."""
        return (
            TASKS_TOLERANCE
            if TASKS_PATTERN.match(artifact.rel)
            else super().secondary_tolerance(artifact)
        )

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Agent Zero file."""
        rel = artifact.rel
        if TASKS_PATTERN.match(rel):
            yield from self._tasks(artifact)
            return
        if TELEGRAM_STATE_PATTERN.match(rel):
            yield from self._telegram_state(artifact)
            return
        match = POLICY_PATTERN.match(rel)
        if match:
            yield from self._policy(artifact, match)
            return
        match = TOGGLE_PATTERN.match(rel)
        if match:
            yield from self._toggle(artifact, match)
            return
        if CONNECTOR_ENV_PATTERN.match(rel):
            yield from self._connector_env(artifact)
            return
        yield from self._chat(artifact, opts)

    def _chat(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        path = artifact.disk_path
        legacy = not CHAT_PATTERN.match(artifact.rel)
        fallback_id = path.stem if legacy else path.parent.name
        base = self.base_row(artifact)
        base.session_id = fallback_id
        raw = path.read_bytes().decode("utf-8", errors="replace")
        try:
            chat = json.loads(raw)
            if not isinstance(chat, dict):
                raise ValueError("top level is not an object")
        except ValueError as error:
            yield self._row(
                base,
                "system",
                "parser: chat.json did not parse (%s); log items recovered without history" % error,
                opts,
            )
            yield from self._log_rows(base, recover_logs(raw), {}, None, opts, {})
            return

        base.session_id = str(chat.get("id") or fallback_id)
        data = chat.get("data")
        if not isinstance(data, dict):
            data = {}
        if data.get("project"):
            base.project_path = project_of(data["project"])
        # An editor (ACP) session's workspace is the user's project on the
        # host that ran the editor (plugins/_a0_acp/api/session.py:131-150).
        if isinstance(data.get("acp_cwd"), str) and data["acp_cwd"].strip():
            base.project_path = data["acp_cwd"].strip()
        start = self._row(
            base,
            "system",
            " | ".join(
                part
                for part in (
                    "chat start",
                    str(chat.get("name") or ""),
                    "type=%s" % chat["type"] if chat.get("type") else "",
                    "profile=%s" % chat["agent_profile"] if chat.get("agent_profile") else "",
                    "project=%s" % data["project"] if data.get("project") else "",
                )
                if part
            ),
            opts,
        )
        start.timestamp_utc = to_utc(chat.get("created_at"))
        yield start
        for text in remote_copy_texts(chat, data):
            yield self._row(base, "system", text, opts, timestamp=file_mtime(artifact))
        for text, project in chat_setting_rows(data):
            row = self._row(base, "system", text, opts, timestamp=file_mtime(artifact))
            if project:
                row.project_path = project
            yield row

        log = chat.get("log")
        if not isinstance(log, dict):
            log = {}
        logs = [item for item in (log.get("logs") or []) if isinstance(item, dict)]
        trimmed = len(logs) >= LOG_SIZE
        models: dict = {}

        history: dict[str, tuple[int, dict]] = {}
        ordered: list[tuple[int, str, dict]] = []
        for i, agent in enumerate(chat.get("agents") or []):
            if not isinstance(agent, dict):
                continue
            agent_number = agent.get("number", i)
            agent_history = agent.get("history")
            if isinstance(agent_history, str):
                try:
                    agent_history = json.loads(agent_history) if agent_history else {}
                except ValueError as error:
                    yield self._row(
                        base,
                        "system",
                        "parser: history of agent %s did not parse (%s)" % (agent_number, error),
                        opts,
                    )
                    continue
            for kind, node in walk_history(agent_history, trimmed):
                ordered.append((agent_number, kind, node))
                if kind == "message" and node.get("id"):
                    history.setdefault(str(node["id"]), (agent_number, node))

        if trimmed:
            yield self._row(
                base,
                "system",
                "log trimmed to its last %d items; earlier turns follow from the agent "
                "history without timestamps" % LOG_SIZE,
                opts,
            )
            log_ids = {str(item.get("id")) for item in logs if item.get("id")}
            for agent_number, kind, node in ordered:
                if kind == "summary":
                    yield self._row(
                        base,
                        "system",
                        self._prefix(agent_number, "[summary] " + _content_text(node["summary"])),
                        opts,
                    )
                elif str(node.get("id") or "") not in log_ids:
                    yield from self._history_rows(base, agent_number, node, opts, models)

        yield from self._log_rows(
            base, logs, history, path.parent / "messages" if not legacy else None, opts, models
        )

    # -- helpers -----------------------------------------------------------------------
    @staticmethod
    def _prefix(agent_number, text: str) -> str:
        return (
            "[agent %s] %s" % (agent_number, text) if agent_number not in (0, None, "0") else text
        )

    def _row(
        self,
        base: Row,
        turn_type: str,
        text: str,
        opts: Options,
        line: int = 0,
        timestamp: str = "",
    ) -> Row:
        row = Row(**base.__dict__)
        row.turn_type = turn_type
        row.text = compact(text)
        row.source_line = line
        row.timestamp_utc = timestamp
        return row

    @staticmethod
    def _message_file(messages_dir, content) -> str:
        if messages_dir is None or not isinstance(content, dict) or not content.get("file"):
            return ""
        name = str(content["file"]).replace("\\", "/").rsplit("/", 1)[-1]
        path = messages_dir / name
        if not name or path.is_symlink() or not path.is_file():
            return ""
        return path.read_bytes().decode("utf-8", errors="replace")

    def _model_row(
        self, base: Row, agent_number, model: str, models: dict, opts: Options, line=0, timestamp=""
    ):
        """Return the `model:` row of a new or changed model, or None.

        The model is tracked per chat and agent number; a subordinate agent's row ends
        ` agent=<n>`.
        """
        text = track_change("model", models, (base.session_id, str(agent_number)), model)
        if not text:
            return None
        if agent_number not in (0, None, "0"):
            text += " agent=%s" % agent_number
        row = self._row(base, "system", text, opts, line, timestamp)
        row.model = model
        return row

    def _history_rows(
        self, base: Row, agent_number, message: dict, opts: Options, models: dict
    ) -> Iterator[Row]:
        content = message.get("content")
        message_id = str(message.get("id") or "")
        if message.get("ai"):
            parsed = parse_ai(content)
            model = model_of(message)
            model_row = self._model_row(base, agent_number, model, models, opts)
            if model_row is not None:
                yield model_row
            text = headline_text(parsed) if parsed else _content_text(content)
            if text:
                row = self._row(base, "assistant", self._prefix(agent_number, text), opts)
                row.model = model
                yield row
            if parsed and parsed.get("tool_name"):
                row = self._row(
                    base,
                    "tool_use",
                    self._prefix(agent_number, args_summary(parsed.get("tool_args"))),
                    opts,
                )
                row.tool_name, row.model = str(parsed["tool_name"]), model
                yield row
        elif isinstance(content, dict) and "tool_result" in content:
            row = self._row(
                base,
                "tool_result",
                self._prefix(agent_number, _content_text(content.get("tool_result"))),
                opts,
            )
            row.tool_name, row.tool_use_id = str(content.get("tool_name") or ""), message_id
            yield row
        elif isinstance(content, dict) and "user_message" in content:
            yield self._row(
                base,
                "user",
                self._prefix(agent_number, _content_text(content.get("user_message"))),
                opts,
            )
        else:
            yield self._row(
                base, "system", self._prefix(agent_number, _content_text(content)), opts
            )

    def _log_rows(
        self,
        base: Row,
        logs: list[dict],
        history: dict[str, tuple[int, dict]],
        messages_dir,
        opts: Options,
        models: dict,
    ) -> Iterator[Row]:
        for item in logs:
            kind = str(item.get("type") or "")
            number = item.get("no")
            line: int = number if isinstance(number, int) else 0
            timestamp = to_utc(item.get("timestamp"))
            agent_number = item.get("agentno", 0)
            kvps = item.get("kvps")
            if not isinstance(kvps, dict):
                kvps = {}
            content = item.get("content") or ""
            item_id = str(item.get("id") or "")
            history_message = history.get(item_id, (None, None))[1] if item_id else None
            history_content = history_message.get("content") if history_message else None

            def make_row(turn_type: str, text: str) -> Row:
                return self._row(
                    base, turn_type, self._prefix(agent_number, text), opts, line, timestamp
                )

            if kind == "user":
                text = (
                    _content_text(history_content.get("user_message"))
                    if isinstance(history_content, dict) and "user_message" in history_content
                    else _content_text(content)
                )
                attachments = kvps.get("attachments")
                if isinstance(attachments, list) and attachments:
                    text += " [attachments: %s]" % ", ".join(
                        str(attachment) for attachment in attachments
                    )
                yield make_row("user", text)
            elif kind == "agent":
                model = model_of(history_message)
                model_row = self._model_row(
                    base, agent_number, model, models, opts, line, timestamp
                )
                if model_row is not None:
                    yield model_row
                if opts.include_thinking and kvps.get("reasoning"):
                    row = make_row("thinking", _content_text(kvps["reasoning"]))
                    row.model = model
                    yield row
                parsed = parse_ai(history_content) if history_message else None
                text = (
                    headline_text(kvps)
                    or (headline_text(parsed) if parsed else "")
                    or (_content_text(history_content) if history_message and not parsed else "")
                    or _content_text(content)
                )
                if text:
                    row = make_row("assistant", text)
                    row.model = model
                    yield row
            elif kind == "response":
                yield make_row("assistant", _content_text(content))
            elif kind in TOOL_TYPES:
                tool_name = str(
                    kvps.get("_tool_name")
                    or (
                        history_content.get("tool_name")
                        if isinstance(history_content, dict)
                        else ""
                    )
                    or kind
                )
                tool_id = item_id or "log:%s" % line
                row = make_row("tool_use", args_summary(kvps))
                row.tool_name, row.tool_use_id = tool_name, tool_id
                yield row
                result = self._message_file(messages_dir, history_content)
                if (
                    not result
                    and isinstance(history_content, dict)
                    and "tool_result" in history_content
                ):
                    result = _content_text(history_content["tool_result"])
                if not result:
                    result = _content_text(content)
                if result or history_message:
                    row = make_row("tool_result", result)
                    row.tool_name, row.tool_use_id = tool_name, tool_id
                    yield row
            else:
                heading = ICON_PATTERN.sub("", str(item.get("heading") or "")).strip()
                text = " | ".join(part for part in (heading, _content_text(content)) if part)
                yield make_row(
                    "system", "%s: %s" % (kind or "log", text) if text else kind or "log"
                )
                if kind == "warning":
                    decision, tool_name = warning_decision(item)
                    if decision:
                        row = make_row("system", decision)
                        row.tool_name = tool_name
                        yield row

    # -- files beside the chats ----------------------------------------------------
    def _doc(self, artifact: Artifact) -> tuple[object, str]:
        """The file as JSON, or (None, the reason it is not)."""
        try:
            raw = artifact.disk_path.read_bytes().decode("utf-8", errors="replace")
        except OSError as error:
            return None, "unreadable (%s)" % error
        try:
            return json.loads(raw), ""
        except ValueError as error:
            return None, "not JSON (%s)" % error

    def _file_row(
        self,
        artifact: Artifact,
        text: str,
        timestamp: str = "",
        session_id: str = "",
        project_path="",
    ):
        row = self.base_row(artifact)
        row.turn_type = "system"
        row.text = compact(text)
        row.timestamp_utc = timestamp
        row.session_id = session_id
        row.project_path = project_path
        return row

    def _tasks(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the rows of `usr/scheduler/tasks.json`.

        Per task (helpers/task_scheduler.py:163-175, 512-519, 584): a `scheduled task:`
        row at `created_at`, the prompt as the chat logged it with no time of its own, and
        the last result at `last_run`.
        """
        doc, why = self._doc(artifact)
        tasks = doc.get("tasks") if isinstance(doc, dict) else None
        if not isinstance(tasks, list):
            yield self._file_row(
                artifact, "parser: scheduler tasks file %s" % (why or "has no tasks list")
            )
            return
        errors = []
        for number, task in enumerate(tasks, 1):
            if not isinstance(task, dict):
                errors.append(
                    (
                        number,
                        "unparseable record %d (not a JSON object)" % number,
                        compact_json(task),
                    )
                )
                continue
            yield from self._task_rows(artifact, number, task)
        yield from error_rows(self, artifact, errors)

    def _task_rows(self, artifact: Artifact, number: int, task: dict) -> Iterator[Row]:
        session_id = str(task.get("context_id") or task.get("uuid") or "")
        project_path = project_of(task.get("project_name"))
        row = self._file_row(
            artifact,
            "scheduled task: %s type=%s state=%s"
            % (_key(task, "name"), _key(task, "type"), _key(task, "state")),
            to_utc(task.get("created_at")),
            session_id,
            project_path,
        )
        row.source_line = number
        yield row
        if task.get("prompt"):
            # The scheduler logs `## Task:\n<prompt>` as the chat's user
            # message when the task has no context (task_scheduler.py:945-949).
            row = self._file_row(
                artifact, "## Task:\n%s" % task["prompt"], "", session_id, project_path
            )
            row.turn_type, row.source_line = "user", number
            yield row
        result = task.get("last_result")
        if isinstance(result, str) and result.strip():
            failed = task.get("state") == "error" and result.startswith("ERROR: ")
            row = self._file_row(
                artifact,
                "scheduled task failed: %s" % result[len("ERROR: ") :] if failed else result,
                to_utc(task.get("last_run")),
                session_id,
                project_path,
            )
            row.turn_type = "system" if failed else "assistant"
            row.source_line = number
            yield row

    def _telegram_state(self, artifact: Artifact) -> Iterator[Row]:
        """Yield a `session deleted:` row per Telegram chat whose chat file is absent."""
        doc, why = self._doc(artifact)
        chats = doc.get("chats") if isinstance(doc, dict) else None
        if not isinstance(chats, dict):
            yield self._file_row(artifact, "parser: state.json %s" % (why or "has no chats map"))
            return
        usr = artifact.disk_path.parents[2]
        for key, context_id in chats.items():
            context_id = str(context_id or "")
            if not context_id or self._chat_present(usr, context_id):
                continue
            parts = str(key).rsplit(":", 2)
            bot, user, chat = parts if len(parts) == 3 else (str(key), "-", "-")
            yield self._file_row(
                artifact,
                "session deleted: telegram bot %s chat %s user %s ctxid %s (from state.json,"
                " no chat.json in the input, file mtime)" % (bot, chat, user, context_id),
                file_mtime(artifact),
                context_id,
            )

    @staticmethod
    def _chat_present(usr, context_id: str) -> bool:
        for path in (
            usr / "chats" / context_id / "chat.json",
            usr / "chats" / ("%s.json" % context_id),
        ):
            if not path.is_symlink() and path.is_file():
                return True
        return False

    def _setting(self, artifact: Artifact, text: str, project: str) -> Row:
        return self._file_row(
            artifact,
            "permission setting: %s (from %s, file mtime)" % (text, artifact.rel),
            file_mtime(artifact),
            "",
            project,
        )

    def _policy(self, artifact: Artifact, match: re.Match) -> Iterator[Row]:
        """Yield a `permission setting:` row per key and list entry of a plugin config.

        The file is a `_tool_access` policy (helpers/tool_policy.py:27-37) or the
        Orchestrator's settings.
        """
        doc, why = self._doc(artifact)
        if not isinstance(doc, dict):
            yield self._file_row(
                artifact, "parser: %s %s" % (artifact.rel, why or "is not a JSON object")
            )
            return
        scope, project = scope_text(match), project_of(match["project"])
        if match["plugin"] == "_tool_access":
            for key in POLICY_SCALARS:
                if doc.get(key) not in (None, ""):
                    value = setting_value(doc[key])
                    yield self._setting(
                        artifact, "tool_access.%s=%s%s" % (key, value, scope), project
                    )
            for key in POLICY_LISTS:
                entries = doc.get(key)
                for entry in entries if isinstance(entries, list) else []:
                    text = "tool_access.%s=%s%s" % (key, setting_value(entry), scope)
                    yield self._setting(artifact, text, project)
            return
        for cli, key in ORCHESTRATOR_KEYS:
            section = doc.get(cli)
            if isinstance(section, dict) and section.get(key) not in (None, ""):
                text = "orchestrator.%s.%s=%s" % (cli, key, setting_value(section[key]))
                yield self._setting(artifact, text, project)

    def _toggle(self, artifact: Artifact, match: re.Match) -> Iterator[Row]:
        """Yield the `permission setting:` row of a plugin's `.toggle-1` or `.toggle-0` file.

        The empty file switches the Infection Check or the Orchestrator on (`1`) or off
        (`0`) (helpers/plugins.py:60-61).
        """
        state = "enabled" if match["state"] == "1" else "disabled"
        text = "%s=%s%s" % (match["plugin"].lstrip("_"), state, scope_text(match))
        yield self._setting(artifact, text, project_of(match["project"]))

    def _connector_env(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the `session origin:` row of the A0 CLI connector's `~/.agent-zero/.env`.

        The file is a secret: only the last chat's host and id are read, as the connector
        reads them (a0-connector config.py:51-66), and no other line reaches a row.
        """
        try:
            text = artifact.disk_path.read_bytes().decode("utf-8", errors="replace")
        except OSError:
            return
        values = {}
        for line in text.splitlines():
            key, sep, value = line.strip().partition("=")
            if sep and key.strip() in LAST_CONTEXT_KEYS:
                values[key.strip()] = value.strip().strip("\"'")
        host, context_id = (values.get(k, "") for k in LAST_CONTEXT_KEYS)
        if host and context_id:
            yield self._file_row(
                artifact,
                "session origin: remote agent-zero instance %s context %s"
                " (from .agent-zero/.env, file mtime)" % (host, context_id),
                file_mtime(artifact),
                context_id,
            )
