"""Parse Twinny conversations, standing grants, file changes and its log channel.

Follows `research/twinny.md`. Validated against source, twinnydotdev/twinny at
9339bd10 (conversations) and 0337b265 (4.3.15: grants, team policy, file
changes, messages sent mid-reply, the log channel and the review repository),
with a synthetic fixture; no real install.

Files read:

- `<User dir>/globalStorage/state.vscdb`, `ItemTable` row `rjmacarthy.twinny`:
  `twinny.conversations` (keyed by id) and `twinny.active-conversation` (used
  only when its id is missing from the first); conversation `id`, `title`,
  `updatedAt`, `messages[]`; message `role`, `content` (reasoning inline as
  `<think>` or `<thinking>`), `images`, `meta.model`, `meta.withheld`,
  `changes` and `toolSteps[]` (`id`, `name`, `summary`, `args`, `command`,
  `output`, `status`, `approval`, `said`); the `STANDING_KEYS` under
  `GLOBAL_PREFIX` and `twinny.teamPolicy`.
- `<User dir>/workspaceStorage/<id>/state.vscdb`: the review tab's
  `reviewOwner` and `reviewRepo`, one `source host repository:` row.
- `agent-changes/<uuid>.json`, a secondary source (a file repeating the
  conversation's events): `{id, root, createdAt, files[]}`.
- `Twinny.log` and its rotated copies, secondary: one `system` row per entry,
  whose host-local time is read as UTC.

The catalog attributes the state database to the editor whose `User/`
directory holds it, so `reads_agents` lists every editor agent. Messages
carry no timestamp: every row of a conversation takes its `updatedAt`, and
`source_line` is 0. Provider `apiKey`s and the ChatGPT plan profile share the
value and are never read.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterator

from ..inputs import Artifact, error_reason
from ..model import Row, compact
from ..timeutil import to_utc
from ..vscode_state import EDITOR_AGENTS, STATE_DATABASE_PATTERN, read_item, shown_problem
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_mtime,
    line_text,
    setting_value,
    text_of,
    track_change,
)

ITEM_KEY = "rjmacarthy.twinny"
CONVERSATIONS = "twinny.conversations"
ACTIVE = "twinny.active-conversation"
THINK_PATTERN = re.compile(r"<(think|thinking)>(.*?)(?:</\1>|$)", re.S)

# The webview's global values, stored as `twinny-global-context-<key>`
# (webview/base.ts:44, 348-360 at 0337b265), in the order they are written.
GLOBAL_PREFIX = "twinny-global-context-"
STANDING_KEYS = ("agentMode", "autoRunCommands", "alwaysRunCommands", "alwaysRunMcpTools")
TEAM_POLICY = "twinny.teamPolicy"
STANDING_SOURCE = "(from state.vscdb rjmacarthy.twinny, file mtime)"

WORKSPACE_DATABASE_PATTERN = re.compile(r"(?:^|/)User/workspaceStorage/[^/]+/state\.vscdb$")
WORKSPACE_PREFIX = "twinny-workspace-context-"
CHANGES_PATTERN = re.compile(
    r"(?:^|/)User/(?:workspaceStorage/[^/]+|globalStorage)/rjmacarthy\.twinny/"
    r"agent-changes/[^/]+\.json$"
)
LOG_PATTERN = re.compile(r"(?:^|/)logs/(?:[^/]+/)+rjmacarthy\.twinny/Twinny\.log(?:_[0-9]+)?$")
LOG_HEADER_PATTERN = re.compile(
    r"^(\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{3}) \[(trace|debug|info|warning|error)\] (.*)$",
    re.ASCII,
)


def split_think(content: str):
    """Split the reasoning blocks out of a message: return (text, [reasoning texts])."""
    thoughts: list[str] = []

    def keep(match):
        if match.group(2).strip():
            thoughts.append(match.group(2).strip())
        return ""

    return THINK_PATTERN.sub(keep, content).strip(), thoughts


def file_change_text(change_id, entry: dict) -> str:
    """Return the `file changed:` text of one changed file of a reply.

    The text is `file changed: <status> <path> +<added> -<removed> reply=<id>`,
    with ` undone=true` and ` noUndo=true` when the entry sets them.
    """
    text = "file changed: %s %s +%s -%s reply=%s" % (
        setting_value(entry.get("status", "")),
        setting_value(entry.get("path", "")),
        setting_value(entry.get("added", "")),
        setting_value(entry.get("removed", "")),
        setting_value(change_id if change_id is not None else ""),
    )
    for flag in ("undone", "noUndo"):
        if entry.get(flag) is True:
            text += " %s=true" % flag
    return text


def flatten(prefix: str, value) -> Iterator[tuple[str, str]]:
    """Yield (dotted key, value text) per leaf of `value`, one pair per list entry."""
    if isinstance(value, dict):
        for member, item in value.items():
            yield from flatten("%s.%s" % (prefix, member), item)
    elif isinstance(value, list):
        for item in value:
            if isinstance(item, (dict, list)):
                yield prefix, compact_json(item)
            else:
                yield prefix, setting_value(item)
    elif value is not None:
        yield prefix, setting_value(value)


class TwinnyParser(Parser):
    """Parse Twinny's editor state, agent-changes records and log channel."""

    agent = "twinny"
    name = "twinny"
    reads_agents = EDITOR_AGENTS
    has_secondary = True

    def wants(self, artifact: Artifact) -> bool:
        """Return True for a state database, agent-changes record or Twinny log."""
        rel = artifact.rel
        return bool(
            STATE_DATABASE_PATTERN.search(rel)
            or WORKSPACE_DATABASE_PATTERN.search(rel)
            or CHANGES_PATTERN.search(rel)
            or LOG_PATTERN.search(rel)
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for an agent-changes record or a log file."""
        return bool(CHANGES_PATTERN.search(artifact.rel) or LOG_PATTERN.search(artifact.rel))

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Twinny file."""
        rel = artifact.rel
        if CHANGES_PATTERN.search(rel):
            yield from self._parse_changes(artifact)
            return
        if LOG_PATTERN.search(rel):
            yield from self._parse_log(artifact)
            return
        state, problems = read_item(artifact.disk_path, ITEM_KEY)
        workspace = bool(WORKSPACE_DATABASE_PATTERN.search(rel))
        if isinstance(state, dict):
            if workspace:
                yield from self._workspace_rows(artifact, state)
            else:
                yield from self._global_rows(artifact, state, opts)
        elif state is not None:
            problems = problems or ["%s value is not a JSON object" % ITEM_KEY]
        for problem in problems:
            row = self.base_row(artifact)
            row.turn_type = "system"
            # never echo the value itself: it holds provider apiKeys
            row.text = "parser: " + shown_problem(problem)
            yield row

    def _global_rows(self, artifact: Artifact, state: dict, opts: Options) -> Iterator[Row]:
        conversations = state.get(CONVERSATIONS)
        conversations = (
            list(conversations.values())
            if isinstance(conversations, dict)
            else conversations
            if isinstance(conversations, list)
            else []
        )
        conversations = [
            conversation for conversation in conversations if isinstance(conversation, dict)
        ]
        active = state.get(ACTIVE)
        ids = {str(conversation.get("id")) for conversation in conversations}
        if isinstance(active, dict) and active.get("messages") and str(active.get("id")) not in ids:
            conversations.append(active)
        for record in conversations:
            yield from self._conversation(artifact, record, opts)
        yield from self._standing_rows(artifact, state)

    def _system_row(self, artifact: Artifact, timestamp: str, text: str) -> Row:
        row = self.base_row(artifact)
        row.timestamp_utc = timestamp
        row.turn_type = "system"
        row.text = text
        return row

    def _standing_rows(self, artifact: Artifact, state: dict) -> Iterator[Row]:
        """Standing grants and the team policy, at the file's mtime."""
        timestamp = ""
        for key in STANDING_KEYS:
            value = state.get(GLOBAL_PREFIX + key)
            if value is None:
                continue
            timestamp = timestamp or file_mtime(artifact)
            label = "permission mode" if key == "agentMode" else "permission setting"
            values = value if isinstance(value, list) else [value]
            for item in values:
                text = "%s: %s=%s %s" % (label, key, setting_value(item), STANDING_SOURCE)
                yield self._system_row(artifact, timestamp, text)
        policy = state.get(TEAM_POLICY)
        if isinstance(policy, dict):
            timestamp = timestamp or file_mtime(artifact)
            for key, value in flatten(TEAM_POLICY, policy):
                text = "permission setting: %s=%s %s" % (key, value, STANDING_SOURCE)
                yield self._system_row(artifact, timestamp, text)

    def _workspace_rows(self, artifact: Artifact, state: dict) -> Iterator[Row]:
        """The repository the pull request review tab last listed."""
        owner = state.get(WORKSPACE_PREFIX + "reviewOwner")
        repository = state.get(WORKSPACE_PREFIX + "reviewRepo")
        if isinstance(owner, str) and owner and isinstance(repository, str) and repository:
            text = (
                "source host repository: https://github.com/%s/%s type=review "
                "(from workspaceStorage state.vscdb rjmacarthy.twinny, file mtime)"
                % (owner, repository)
            )
            yield self._system_row(artifact, file_mtime(artifact), text)

    def _parse_changes(self, artifact: Artifact) -> Iterator[Row]:
        """`file changed:` rows of one agent-changes record."""
        try:
            raw = artifact.disk_path.read_bytes()
        except OSError as error:
            yield from error_rows(
                self, artifact, [(0, "unreadable (%s)" % error_reason(error), "")]
            )
            return
        try:
            record = json.loads(raw.decode("utf-8", errors="replace"))
        except ValueError as error:
            message = "agent-changes record is not valid JSON (%s)" % error
            yield from error_rows(self, artifact, [(0, message, "")])
            return
        if not isinstance(record, dict):
            message = "agent-changes record is not a JSON object"
            yield from error_rows(self, artifact, [(0, message, "")])
            return
        root = record.get("root")
        root = root if isinstance(root, str) else ""
        timestamp = to_utc(record.get("createdAt"))
        files = record.get("files")
        for entry in files if isinstance(files, list) else []:
            if not isinstance(entry, dict):
                continue
            row = self._system_row(artifact, timestamp, file_change_text(record.get("id"), entry))
            row.project_path = root
            yield row

    def _parse_log(self, artifact: Artifact) -> Iterator[Row]:
        """One `system` row per entry of a Twinny log channel file."""
        name = artifact.disk_path.name
        errors: list = []
        entries: list[tuple[int, str, str, str, list[str]]] = []
        more: list[str] | None = None  # the open entry's continuation lines
        try:
            with open(artifact.disk_path, "rb") as handle:
                for line_number, raw in enumerate(handle, 1):
                    line = line_text(raw)
                    match = LOG_HEADER_PATTERN.match(line)
                    if match:
                        more = []
                        entries.append(
                            (line_number, match.group(1), match.group(2), match.group(3), more)
                        )
                    elif more is not None and (not line or line[0] in " \t"):
                        more.append(line[4:] if line.startswith("    ") else line)
                    elif line:
                        errors.append((line_number, "unparseable line (no log line header)", line))
                        more = None  # a continuation after a bad line has no entry
        except OSError as error:
            yield from error_rows(
                self, artifact, [(0, "unreadable (%s)" % error_reason(error), "")]
            )
            return
        for line_number, when, level, message, lines in entries:
            head = "log %s: %s (from %s, log line time, host-local)" % (level, message, name)
            row = self._system_row(artifact, to_utc(when), "\n".join([head, *lines]))
            row.source_line = line_number
            yield row
        yield from error_rows(self, artifact, errors)

    def _conversation(self, artifact: Artifact, conversation: dict, opts: Options) -> Iterator[Row]:
        timestamp = to_utc(conversation.get("updatedAt"))
        session_id = str(conversation.get("id") or "")
        messages = conversation.get("messages")
        messages = (
            [message for message in messages if isinstance(message, dict)]
            if isinstance(messages, list)
            else []
        )

        def row(turn: str, text, model: str = "", name: str = "", tool_use_id: str = "") -> Row:
            result = self.base_row(artifact)
            result.timestamp_utc = timestamp
            result.session_id = session_id
            result.turn_type = turn
            result.model = model
            result.tool_name = name
            result.tool_use_id = tool_use_id
            result.text = compact(text)
            return result

        yield row(
            "system",
            "conversation: %s | messages=%d%s"
            % (
                conversation.get("title") or "",
                len(messages),
                "" if timestamp else " | updatedAt unknown",
            ),
        )
        seen: dict = {}  # per conversation, so the tracked key needs no session id
        for chat_message in messages:
            role = chat_message.get("role")
            meta = chat_message.get("meta")
            if not isinstance(meta, dict):
                meta = {}
            model = str(meta.get("model") or "")
            if role == "assistant":
                change = track_change("model", seen, "model", model)
                if change:
                    yield row("system", change, model)
            content = chat_message.get("content")
            text = content if isinstance(content, str) else text_of(content)
            text, thoughts = split_think(text)
            if opts.include_thinking:
                for thought in thoughts:
                    yield row("thinking", thought, model)
            images = chat_message.get("images")
            if isinstance(images, list) and images:
                text = ("%s\n[%d image(s)]" % (text, len(images))).strip()
            if text:
                turn = (
                    "user" if role == "user" else "assistant" if role == "assistant" else "system"
                )
                yield row(
                    turn,
                    text if turn != "system" else "%s: %s" % (role or "unknown", text),
                    model if turn != "user" else "",
                )
            for step in chat_message.get("toolSteps") or []:
                if not isinstance(step, dict):
                    continue
                if step.get("said") is True:
                    # Not a tool: a message the user sent while the reply ran.
                    said = step.get("summary")
                    # A summary that is not a string is kept whole, as JSON.
                    said = (
                        said
                        if isinstance(said, str)
                        else ""
                        if said is None
                        else compact_json(said)
                    )
                    yield row("user", said)
                    continue
                name, tool_use_id = str(step.get("name") or ""), str(step.get("id") or "")
                args = step.get("args")
                body = (
                    step.get("command")
                    or (compact_json(args) if args else "")
                    or step.get("summary")
                    or ""
                )
                yield row("tool_use", body, model, name, tool_use_id)
                out = step.get("output")
                out = out if isinstance(out, str) else compact_json(out) if out is not None else ""
                status = str(step.get("status") or "")
                if status and status != "done":
                    out = "[%s] %s" % (status, out)
                yield row("tool_result", out, model, name, tool_use_id)
                if status in ("waiting", "skipped"):
                    decision = "permission decision: " + status
                    if step.get("approval"):
                        decision += " approval=" + setting_value(step["approval"])
                    yield row("system", decision, model, name, tool_use_id)
            changes = chat_message.get("changes")
            if isinstance(changes, dict) and isinstance(changes.get("files"), list):
                for entry in changes["files"]:
                    if isinstance(entry, dict):
                        yield row("system", file_change_text(changes.get("id"), entry), model)
            withheld = meta.get("withheld")
            if withheld:
                yield row("system", "secret shield withheld: %s" % compact_json(withheld), model)


__all__ = ["TwinnyParser", "split_think"]
