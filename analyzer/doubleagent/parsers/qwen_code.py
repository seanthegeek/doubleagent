"""Parse Qwen Code sessions, prompt logs, settings, channel files and past-use stores.

Follows `research/qwen-code.md`. Validated against source, QwenLM/qwen-code at
835b080 (0.26.4, beside release v0.26.5), with the shapes of 2c591ec (0.24.7)
and 5dcf7c7 (0.25.3) still read, and a synthetic fixture; no real install. The
runtime-base stores are read under `.qwen/` and under the managed local mode's
`.qwen/managed-local/harness/` and `.qwen/managed-local/worker-home/`. Files read:

- `projects/<cwd>/chats/[archive/]<sessionId>.jsonl`, one `ChatRecord` per line:
  `sessionId`, `timestamp`, `type`, `subtype`, `cwd`, `gitBranch`, `provenance`,
  `agentId`, `agentName`, `model`, `executionContext`, `systemPayload`,
  `message.parts[]` and `toolCallResult`; a channel prompt's quoted group
  messages give `group message:` rows. Sidecars `<sessionId>.worktree.json`
  (`worktreePath`) and `<sessionId>.pr.json` (`prs[]`); `.ledger.jsonl` and
  `.runtime.json` are not read.
- `tmp/<hash>/logs.json` and `.qwen/usage_record.jsonl`, secondary sources.
- `usage/token-usage-<YYYY-MM>.jsonl`, `todos/<sessionId>.json`,
  `projects/<cwd>/session-organization.v1.json`, `.qwen/PROJECT_SUMMARY.md`.
- `.qwen/trustedFolders.json`; `.qwen/settings.json` (only `permissions` and
  `tools.approvalMode`, since the file holds provider keys and channel tokens).
- `.qwen/channels/sessions.json`, `.qwen/channels/daemon/<hash>/routes.json`,
  `<channel>-group-history.jsonl` (secondary) and the GitHub channel's
  `<scope>/<channel>-<hash>-github-audit.jsonl`.

The managed local mode's H2 database `managed.mv.db` is not read: no standard
library module decodes its MVStore format. Model and approval mode are tracked
per `sessionId`.
"""

from __future__ import annotations

import json
import os
import re
import stat
from collections.abc import Iterator
from urllib.parse import urlsplit

from ..dedup import MATCH_TOLERANCES
from ..inputs import Artifact
from ..model import Row, compact
from ..timeutil import to_utc
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_bytes,
    file_mtime,
    iter_jsonl,
    setting_value,
    tail_text,
    track_change,
)
from .gemini_cli import parse_jsonc, trusted_folder_rows

# Runtime-base stores are matched on the path below their root, with the
# managed local mode's harness and worker roots read like `.qwen/`.
RUNTIME_ROOTS = (".qwen/managed-local/harness/", ".qwen/managed-local/worker-home/")
# The session list's own file name test, which leaves out the sidecars.
SESSION_PATTERN = re.compile(
    r"^\.qwen/projects/[^/]+/chats/(?:archive/)?[0-9a-fA-F-]{32,36}\.jsonl$"
)
LOGS_PATTERN = re.compile(r"^\.qwen/tmp/[^/]+/logs\.json$")
TOKEN_USAGE_PATTERN = re.compile(r"^\.qwen/usage/token-usage-[0-9]{4}-[0-9]{2}\.jsonl$")
TODO_PATTERN = re.compile(r"^\.qwen/todos/[^/]+\.json$")
ORGANIZATION_PATTERN = re.compile(r"^\.qwen/projects/[^/]+/session-organization\.v1\.json$")
PULL_REQUESTS_PATTERN = re.compile(
    r"^\.qwen/projects/[^/]+/chats/(?:archive/)?[0-9a-fA-F-]{32,36}\.pr\.json$"
)
RUNTIME_PATTERNS = (
    SESSION_PATTERN,
    LOGS_PATTERN,
    TOKEN_USAGE_PATTERN,
    TODO_PATTERN,
    ORGANIZATION_PATTERN,
    PULL_REQUESTS_PATTERN,
)
TRUSTED_PATTERN = re.compile(r"^\.qwen/trustedFolders\.json$")
CHANNELS_PATTERN = re.compile(r"^\.qwen/channels/sessions\.json$")
DAEMON_ROUTES_PATTERN = re.compile(r"^\.qwen/channels/daemon/[^/]+/routes\.json$")
GROUP_HISTORY_PATTERN = re.compile(r"^\.qwen/channels/[^/]+-group-history\.jsonl$")
AUDIT_PATTERN = re.compile(r"^\.qwen/channels/[^/]+/[^/]+-[0-9a-f]{16}-github-audit\.jsonl$")
# `~/.qwen/settings.json`, or a project's `.qwen/settings.json` (a project
# catalog file), for the standing permission rules.
SETTINGS_PATTERN = re.compile(r"^\.qwen/settings\.json$")
USAGE_PATTERN = re.compile(r"^\.qwen/usage_record\.jsonl$")
SUMMARY_PATTERN = re.compile(r"^\.qwen/PROJECT_SUMMARY\.md$")
PATTERNS = (
    TRUSTED_PATTERN,
    CHANNELS_PATTERN,
    DAEMON_ROUTES_PATTERN,
    GROUP_HISTORY_PATTERN,
    AUDIT_PATTERN,
    SETTINGS_PATTERN,
    USAGE_PATTERN,
    SUMMARY_PATTERN,
)
# The markers `prependGroupHistoryContext` wraps a channel prompt's quoted
# group messages in, one `- [<sender>] <text>` line each.
GROUP_HISTORY_MARKER = "[Chat messages since your last reply - for context]"
CURRENT_MESSAGE_MARKER = "[Current message - respond to this]"
QUOTED_LINE = re.compile(r"^- \[([^\]\r\n]*)\] (.*)$")
# A group message waits in the history file, with no age limit, until the
# bot is next addressed in that chat and the prompt quotes it, so its quote
# is matched forward this far, bounded by the next line of the same text.
GROUP_MESSAGE_WINDOW_SECONDS = 366 * 24 * 3600.0
PERMISSION_LISTS = ("allow", "ask", "deny")
# `ToolCallEvent` as `logToolCall` records it in a `ui_telemetry` record,
# and the class's own default name, which an older build may have written.
TOOL_CALL_EVENTS = ("qwen-code.tool_call", "tool_call")
DECISION_STATUSES = ("cancelled", "awaiting_approval")
# `type: user` subtypes that never hold a prompt the user typed.
INJECTED_SUBTYPES = ("notification", "cron", "agent_message")

ARG_KEYS = (
    "command",
    "absolute_path",
    "file_path",
    "path",
    "pattern",
    "url",
    "query",
    "prompt",
    "description",
)


def args_summary(args) -> str:
    """Return the most identifying argument of a function call, else the arguments as JSON."""
    if isinstance(args, dict):
        for key in ARG_KEYS:
            value = args.get(key)
            if value not in (None, ""):
                return value if isinstance(value, str) else compact_json(value)
        return compact_json(args)
    if args in (None, ""):
        return ""
    return args if isinstance(args, str) else compact_json(args)


def _parts(record: dict) -> list:
    message = record.get("message")
    if not isinstance(message, dict):
        return []
    parts = message.get("parts")
    return [part for part in parts if isinstance(part, dict)] if isinstance(parts, list) else []


def _response_text(response) -> str:
    if isinstance(response, dict):
        for key in ("output", "error", "llmContent"):
            value = response.get(key)
            if isinstance(value, str) and value:
                return value
        return compact_json(response)
    if response in (None, ""):
        return ""
    return response if isinstance(response, str) else compact_json(response)


def system_text(subtype: str, payload) -> str:
    """Return the row text of a `system` record: its subtype and a summary of its payload."""
    fields = payload if isinstance(payload, dict) else {}
    if subtype == "slash_command":
        detail = "%s %s" % (fields.get("phase") or "", fields.get("rawCommand") or "")
    elif subtype == "session_model":
        detail = "%s auth=%s" % (fields.get("modelId") or "", fields.get("authType") or "")
    elif subtype == "custom_title":
        detail = fields.get("customTitle") or ""
    elif subtype == "turn_result":
        detail = "%s %s" % (fields.get("state") or "", fields.get("resultText") or "")
    elif subtype == "rewind":
        detail = "truncated %s record(s)" % fields.get("truncatedCount")
    elif subtype == "chat_compression":
        detail = compact_json(fields.get("info")) if fields.get("info") is not None else ""
    else:
        detail = compact_json(payload) if payload else ""
    detail = str(detail).strip()
    return "%s: %s" % (subtype, detail) if detail else subtype


def injected_origin(record: dict) -> str:
    """Return the row prefix of a `type: user` record the user did not type, else "".

    The prefix is `<subtype> (provenance=<provenance> agent=<agentName>
    agentId=<agentId>)`, with the parts not recorded left out. A record is
    the user's when its `provenance` is absent or `real_user`, unless its
    subtype is one only the system or another agent writes.
    """
    provenance = str(record.get("provenance") or "")
    subtype = str(record.get("subtype") or "")
    if provenance in ("", "real_user") and subtype not in INJECTED_SUBTYPES:
        return ""
    payload = record.get("systemPayload")
    author = payload.get("author") if isinstance(payload, dict) else None
    author = author if isinstance(author, dict) else {}
    name = record.get("agentName") or author.get("name")
    agent_id = record.get("agentId") or author.get("agentId")
    keys = [
        "%s=%s" % (key, value)
        for key, value in (("provenance", provenance), ("agent", name), ("agentId", agent_id))
        if value not in (None, "")
    ]
    label = subtype or "user"
    return "%s (%s)" % (label, " ".join(keys)) if keys else label


def _value(record: dict, key: str) -> str:
    """Return a record's value as row text, `-` when absent or empty."""
    field = record.get(key)
    return str(field) if field not in (None, "") else "-"


def runtime_rel(rel: str) -> str:
    """Return `rel` with a managed local root replaced by `.qwen/`, the runtime base it mirrors."""
    for root in RUNTIME_ROOTS:
        if rel.startswith(root):
            return ".qwen/" + rel[len(root) :]
    return rel


def quoted_group_messages(text: str) -> list[str]:
    """Return the `group message: [<sender>] <text>` texts a channel prompt quotes, else [].

    The quote is the block between `GROUP_HISTORY_MARKER` at the start of a
    line and a blank line before `CURRENT_MESSAGE_MARKER`; a block with a line
    of another form is not the channel's, and gives none.
    """
    start = text.find(GROUP_HISTORY_MARKER + "\n")
    while start > 0 and text[start - 1] != "\n":
        start = text.find(GROUP_HISTORY_MARKER + "\n", start + 1)
    if start < 0:
        return []
    body = start + len(GROUP_HISTORY_MARKER) + 1
    end = text.find("\n\n" + CURRENT_MESSAGE_MARKER, body)
    if end < 0:
        return []
    out = []
    for line in text[body:end].split("\n"):
        match = QUOTED_LINE.match(line)
        if not match:
            return []
        out.append("group message: [%s] %s" % (match.group(1), match.group(2)))
    return out


def match_tolerance(row: Row) -> float | None:
    """Return the forward window of a group history row (`dedup.MATCH_TOLERANCES`), else None.

    The prompt quoting a group message is written when the bot is next
    addressed in that chat, any time after the message arrived.
    """
    if row.turn_type == "system" and row.text.startswith("group message: "):
        return GROUP_MESSAGE_WINDOW_SECONDS
    return None


class QwenCodeParser(Parser):
    """Parse Qwen Code's chat files, prompt logs, settings, routes and usage records."""

    agent = "qwen-code"
    name = "qwen-code"
    has_secondary = True
    records_relay = True

    def wants(self, artifact: Artifact) -> bool:
        """Return True for a chat file (not a ledger or runtime sidecar) or another file it reads."""
        rel = runtime_rel(artifact.rel)
        return any(pattern.match(rel) for pattern in RUNTIME_PATTERNS) or any(
            pattern.match(artifact.rel) for pattern in PATTERNS
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for a prompt log, the usage records or a channel's group history."""
        return bool(
            LOGS_PATTERN.match(runtime_rel(artifact.rel))
            or USAGE_PATTERN.match(artifact.rel)
            or GROUP_HISTORY_PATTERN.match(artifact.rel)
        )

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Qwen Code file."""
        rel = runtime_rel(artifact.rel)
        if TRUSTED_PATTERN.match(artifact.rel):
            yield from trusted_folder_rows(self, artifact)
        elif SETTINGS_PATTERN.match(artifact.rel):
            yield from self._parse_settings(artifact)
        elif USAGE_PATTERN.match(artifact.rel):
            yield from self._parse_usage(artifact)
        elif CHANNELS_PATTERN.match(artifact.rel) or DAEMON_ROUTES_PATTERN.match(artifact.rel):
            # Named without the leading `.qwen/`, as the row text names it.
            yield from self._parse_channel_routes(artifact, artifact.rel.split("/", 1)[1])
        elif GROUP_HISTORY_PATTERN.match(artifact.rel):
            yield from self._parse_group_history(artifact)
        elif AUDIT_PATTERN.match(artifact.rel):
            yield from self._parse_audit(artifact)
        elif SUMMARY_PATTERN.match(artifact.rel):
            yield from self._parse_summary(artifact)
        elif LOGS_PATTERN.match(rel):
            yield from self._parse_logs(artifact, opts)
        elif TOKEN_USAGE_PATTERN.match(rel):
            yield from self._parse_token_usage(artifact)
        elif TODO_PATTERN.match(rel):
            yield from self._parse_todos(artifact)
        elif ORGANIZATION_PATTERN.match(rel):
            yield from self._parse_organization(artifact)
        elif PULL_REQUESTS_PATTERN.match(rel):
            yield from self._parse_pull_requests(artifact)
        else:
            yield from self._parse_session(artifact, opts)

    def _system_row(self, artifact: Artifact, line: int, timestamp: str, text: str) -> Row:
        row = self.base_row(artifact)
        row.source_line = line
        row.turn_type = "system"
        row.timestamp_utc = timestamp
        row.text = compact(text)
        return row

    @staticmethod
    def _read_object(artifact: Artifact, errors: list) -> dict | None:
        """Return a JSON file's object, or None after appending its failure to `errors`.

        The failure is the unreadable entry, or `<file> is not a JSON object`.
        """
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            return None
        try:
            doc = json.loads(content.decode("utf-8", errors="replace"))
        except ValueError:
            doc = None
        if not isinstance(doc, dict):
            errors.append((0, "%s is not a JSON object" % artifact.disk_path.name, ""))
            return None
        return doc

    def _parse_token_usage(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `token usage:` row per record of a monthly token usage log, at its time.

        No delete touches the log, so it keeps every response of a deleted session.
        """
        errors: list = []
        name = artifact.disk_path.name
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            counts = " ".join(
                "%s %s" % (label, _value(record, key))
                for label, key in (
                    ("input", "inputTokens"),
                    ("output", "outputTokens"),
                    ("cached", "cachedTokens"),
                    ("thoughts", "thoughtsTokens"),
                    ("total", "totalTokens"),
                )
            )
            duration = record.get("apiDurationMs")
            text = "token usage: model %s auth %s source %s tokens %s api %s (from %s)" % (
                _value(record, "model"),
                _value(record, "authType"),
                _value(record, "source"),
                counts,
                "%sms" % duration if duration not in (None, "") else "-",
                name,
            )
            row = self._system_row(artifact, line_number, to_utc(record.get("timestamp")), text)
            row.session_id = str(record.get("sessionId") or "")
            row.model = str(record.get("model") or "")
            yield row
        yield from error_rows(self, artifact, errors)

    def _parse_group_history(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the rows of a channel's `<channel>-group-history.jsonl`, a secondary source.

        A `message` record is `group message: [<sender>] <text>` at the
        message's time, the form a prompt quoting it gives; a `clear` record,
        written when a prompt drains the chat's messages or the chat is
        cleared, is a `group history cleared:` row.
        """
        errors: list = []
        name = artifact.disk_path.name
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            record_type = record.get("type")
            entry = record.get("entry")
            if record_type == "message" and isinstance(entry, dict):
                sender = entry.get("senderName") or entry.get("senderId") or ""
                text = "group message: [%s] %s" % (sender, entry.get("text") or "")
                timestamp = to_utc(entry.get("timestamp"))
            elif record_type == "clear":
                text = "group history cleared: %s (from %s, recordedAt)" % (
                    record.get("key") or "-",
                    name,
                )
                timestamp = to_utc(record.get("recordedAt"))
            else:
                message = "unparseable line (not a group history record)"
                errors.append((line_number, message, compact_json(record)))
                continue
            yield self._system_row(artifact, line_number, timestamp, text)
        yield from error_rows(self, artifact, errors)

    def _parse_summary(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the one `project summary` row of a `.qwen/PROJECT_SUMMARY.md`.

        The row holds the whole file at its modification time, in the project
        whose `.qwen/` holds it.
        """
        errors: list = []
        project = artifact.home.original
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors, "", project)
            return
        text = "project summary (from .qwen/PROJECT_SUMMARY.md, file mtime): " + content.decode(
            "utf-8", errors="replace"
        )
        row = self._system_row(artifact, 1, file_mtime(artifact), text)
        row.project_path = project
        yield row

    def _parse_todos(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `todo:` row per item of a session's todo file, at the file's time.

        The session is the file's `sessionId`, else its name; `default`, the
        file a process without a session id writes, names none.
        """
        errors: list = []
        doc = self._read_object(artifact, errors)
        stem = artifact.disk_path.stem
        session_id = "" if stem == "default" else stem
        if doc is None:
            yield from error_rows(self, artifact, errors, session_id)
            return
        recorded = str(doc.get("sessionId") or stem)
        session_id = "" if recorded == "default" else recorded
        todos = doc.get("todos")
        mtime = file_mtime(artifact)
        name = "todos/" + artifact.disk_path.name
        for index, item in enumerate(todos if isinstance(todos, list) else [], 1):
            if not isinstance(item, dict):
                errors.append((index, "todo %d is not a JSON object" % index, ""))
                continue
            blocked = item.get("blockedBy")
            blocked_by = (
                ",".join(str(each) for each in blocked) if isinstance(blocked, list) else ""
            )
            text = "todo: [%s] %s (id %s, blocked by %s, from %s, file mtime)" % (
                _value(item, "status"),
                item.get("content") or "",
                _value(item, "id"),
                blocked_by or "-",
                name,
            )
            row = self._system_row(artifact, index, mtime, text)
            row.session_id = session_id
            yield row
        yield from error_rows(self, artifact, errors, session_id)

    def _parse_organization(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `session organization:` row per session of the Web Shell's grouping file.

        Each row names the session's group (by name when the file lists it),
        colour and pin time, at the entry's `updatedAt`.
        """
        errors: list = []
        doc = self._read_object(artifact, errors)
        if doc is None:
            yield from error_rows(self, artifact, errors)
            return
        groups = doc.get("groups")
        names = {
            str(group["id"]): str(group.get("name") or group["id"])
            for group in (groups if isinstance(groups, list) else [])
            if isinstance(group, dict) and group.get("id")
        }
        sessions = doc.get("sessions")
        sessions = sessions if isinstance(sessions, dict) else {}
        for index, (session_id, entry) in enumerate(sessions.items(), 1):
            if not isinstance(entry, dict):
                errors.append((index, "session %s is not a JSON object" % session_id, ""))
                continue
            group_id = entry.get("groupId")
            text = (
                "session organization: group %s color %s pinned %s"
                " (from session-organization.v1.json, updatedAt)"
                % (
                    names.get(str(group_id), str(group_id)) if group_id else "-",
                    _value(entry, "color"),
                    entry.get("pinnedAt") or "no",
                )
            )
            row = self._system_row(artifact, index, to_utc(entry.get("updatedAt")), text)
            row.session_id = str(session_id)
            yield row
        yield from error_rows(self, artifact, errors)

    def _parse_pull_requests(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `source host:` row per pull request a session's `.pr.json` binds.

        The host and repository come from the pull request's `url`; the row is
        at its `createdAt`.
        """
        errors: list = []
        name = artifact.disk_path.name
        session_id = name[: -len(".pr.json")]
        doc = self._read_object(artifact, errors)
        prs = doc.get("prs") if doc is not None else None
        for index, pull_request in enumerate(prs if isinstance(prs, list) else [], 1):
            if not isinstance(pull_request, dict):
                errors.append((index, "pull request %d is not a JSON object" % index, ""))
                continue
            url = str(pull_request.get("url") or "")
            address = urlsplit(url)
            segments = [segment for segment in address.path.split("/") if segment]
            issues = pull_request.get("issues")
            numbers = [
                str(issue["number"])
                for issue in (issues if isinstance(issues, list) else [])
                if isinstance(issue, dict) and issue.get("number") is not None
            ]
            text = (
                "source host: %s repository %s pull request %s state %s source %s issues %s"
                " (from %s, createdAt)"
                % (
                    address.hostname or "-",
                    "/".join(segments[:2]) if len(segments) >= 2 else "-",
                    url or "-",
                    _value(pull_request, "state"),
                    _value(pull_request, "source"),
                    ",".join(numbers) or "-",
                    name,
                )
            )
            row = self._system_row(artifact, index, to_utc(pull_request.get("createdAt")), text)
            row.session_id = session_id
            yield row
        yield from error_rows(self, artifact, errors, session_id)

    def _parse_audit(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `source host: github` row per publication record of the GitHub channel.

        The actor is the GitHub login the reply answered, where recorded; the
        reply's body is recorded only as its hash and length, left out.
        """
        errors: list = []
        name = artifact.disk_path.name
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            failure = ""
            if record.get("failurePhase") or record.get("failureError"):
                failure = " failure %s: %s" % (
                    _value(record, "failurePhase"),
                    _value(record, "failureError"),
                )
            text = (
                "source host: github repository %s actor %s publication %s trigger %s number %s"
                " comment %s%s (from %s, at)"
                % (
                    _value(record, "repository"),
                    _value(record, "actor"),
                    _value(record, "outcome"),
                    _value(record, "triggerKind"),
                    _value(record, "number"),
                    _value(record, "commentUrl"),
                    failure,
                    name,
                )
            )
            row = self._system_row(artifact, line_number, to_utc(record.get("at")), text)
            row.session_id = str(record.get("sessionId") or "")
            yield row
        yield from error_rows(self, artifact, errors)

    def _parse_channel_routes(self, artifact: Artifact, name: str) -> Iterator[Row]:
        """One `session relayed:` row per entry of a channel routes file."""
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors)
            return
        try:
            doc = json.loads(content.decode("utf-8", errors="replace"))
        except ValueError:
            doc = None
        if not isinstance(doc, dict):
            yield from error_rows(self, artifact, [(0, "%s is not a JSON object" % name, "")])
            return
        mtime = ""
        for index, (key, entry) in enumerate(doc.items(), 1):
            if not isinstance(entry, dict):
                errors.append((index, "entry %s is not a JSON object" % key, ""))
                continue
            target = entry.get("target")
            target = target if isinstance(target, dict) else {}

            def value(key: str, target: dict = target) -> str:
                field = target.get(key)
                return str(field) if field not in (None, "") else "-"

            row = self.base_row(artifact)
            row.source_line = index
            row.turn_type = "system"
            row.session_id = str(entry.get("sessionId") or "")
            row.project_path = str(entry.get("cwd") or "")
            row.timestamp_utc = to_utc(entry.get("startedAt"))
            when = "startedAt"
            if not row.timestamp_utc:
                mtime = mtime or file_mtime(artifact)
                row.timestamp_utc, when = mtime, "file mtime"
            row.text = "session relayed: channel %s chat %s sender %s thread %s (from %s, %s)" % (
                value("channelName"),
                value("chatId"),
                value("senderId"),
                value("threadId"),
                name,
                when,
            )
            yield row
        yield from error_rows(self, artifact, errors)

    def _settings_row(self, artifact: Artifact, timestamp: str, project: str, text: str) -> Row:
        row = self.base_row(artifact)
        row.source_line = 1
        row.turn_type = "system"
        row.timestamp_utc = timestamp
        row.project_path = project
        row.text = compact(text)
        return row

    def _parse_settings(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the permission rules and approval mode of a `settings.json`.

        One `permission setting:` row per `permissions.allow`, `ask` and
        `deny` rule and one for `tools.approvalMode`, read as JSONC as Qwen
        Code reads it; no other key is read, since the file can hold provider
        keys and channel tokens.
        """
        errors: list = []
        raw = file_bytes(artifact.disk_path, errors)
        if raw is None:
            yield from error_rows(self, artifact, errors)
            return
        timestamp = file_mtime(artifact)
        project = artifact.home.original if artifact.home.project else ""
        try:
            doc = parse_jsonc(raw)
        except ValueError:
            doc = None
        if not isinstance(doc, dict):
            message = "%s is not a JSON object" % artifact.rel
            yield from error_rows(self, artifact, [(0, message, "")], "", project)
            return
        settings: list = []
        permissions = doc.get("permissions")
        permissions = permissions if isinstance(permissions, dict) else {}
        for key in PERMISSION_LISTS:
            entries = permissions.get(key)
            if isinstance(entries, list):
                settings.extend(("permissions." + key, entry) for entry in entries)
        tools = doc.get("tools")
        if isinstance(tools, dict) and tools.get("approvalMode") is not None:
            settings.append(("tools.approvalMode", tools["approvalMode"]))
        for key, value in settings:
            text = "permission setting: %s=%s (from %s, file mtime)" % (
                key,
                setting_value(value),
                artifact.rel,
            )
            yield self._settings_row(artifact, timestamp, project, text)

    def _parse_usage(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `session usage:` row per `usage_record.jsonl` line, at its `startTime`.

        The file survives the session's delete.
        """
        errors: list = []
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            models = record.get("models")
            tools = record.get("tools")
            by_name = tools.get("byName") if isinstance(tools, dict) else None
            counts = [
                "%s=%s" % (name, stats.get("count") if isinstance(stats, dict) else stats)
                for name, stats in (by_name.items() if isinstance(by_name, dict) else ())
            ]
            row = self.base_row(artifact)
            row.source_line = line_number
            row.turn_type = "system"
            row.timestamp_utc = to_utc(record.get("startTime"))
            row.session_id = str(record.get("sessionId") or "")
            row.project_path = str(record.get("project") or "")
            duration = record.get("durationMs")
            row.text = compact(
                "session usage: project %s duration %s models %s tools %s"
                " (from usage_record.jsonl, startTime)"
                % (
                    row.project_path or "-",
                    "%sms" % duration if duration not in (None, "") else "-",
                    ",".join(models) if isinstance(models, dict) and models else "-",
                    ",".join(counts) or "-",
                )
            )
            yield row
        yield from error_rows(self, artifact, errors)

    @staticmethod
    def _context_changes(record: dict, session_id: str, models: dict, modes: dict) -> list:
        """Return the `model:` and `permission mode:` changes a record's `executionContext` makes.

        Each item is (row text, model for the `model` column), for a model or
        mode that differs from the one in force for the session.
        """
        execution_context = record.get("executionContext")
        if not isinstance(execution_context, dict):
            return []
        out = []
        model = str(execution_context.get("modelId") or "")
        change = track_change("model", models, session_id, model)
        if change:
            auth_type = execution_context.get("authType")
            out.append((change + (" auth=%s" % auth_type if auth_type else ""), model))
        mode = track_change(
            "permission mode", modes, session_id, str(execution_context.get("approvalMode") or "")
        )
        if mode:
            out.append((mode, ""))
        return out

    def _parse_session(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        session_id = project_path = ""
        models: dict = {}  # session id -> model in force, for the model rows
        modes: dict = {}  # session id -> approval mode in force
        first: tuple = ()  # (line, timestamp, session id, cwd) of the first record
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            record_type = record.get("type")
            if not first:
                first = (
                    line_number,
                    to_utc(record.get("timestamp")),
                    str(record.get("sessionId") or ""),
                    str(record.get("cwd") or ""),
                )
            if record.get("sessionId"):
                session_id = str(record["sessionId"])
            if record.get("cwd"):
                project_path = str(record["cwd"])

            def base(turn_type: str, text: str) -> Row:
                row = self.base_row(artifact)
                row.source_line = line_number
                row.timestamp_utc = to_utc(record.get("timestamp"))
                row.session_id = session_id
                row.project_path = str(record.get("cwd") or "")
                row.git_branch = str(record.get("gitBranch") or "")
                row.turn_type = turn_type
                row.text = compact(text)
                return row

            for text, model in self._context_changes(record, session_id, models, modes):
                row = base("system", text)
                row.model = model
                yield row
            parts = _parts(record)
            if record_type == "user":
                text = "\n".join(
                    part["text"]
                    for part in parts
                    if isinstance(part.get("text"), str) and not part.get("thought")
                )
                if not text:
                    payload = record.get("systemPayload")
                    if isinstance(payload, dict):
                        text = str(payload.get("displayText") or "")
                origin = injected_origin(record)
                if origin:
                    yield base("system", "%s: %s" % (origin, text) if text else origin)
                else:
                    yield base("user", text)
                for quoted in quoted_group_messages(text):
                    yield base("system", quoted)
            elif record_type == "assistant":
                model = str(record.get("model") or "")
                change = track_change("model", models, session_id, model)
                if change:
                    row = base("system", change)
                    row.model = model
                    yield row
                for part in parts:
                    function_call = part.get("functionCall")
                    if isinstance(function_call, dict):
                        row = base("tool_use", args_summary(function_call.get("args")))
                        row.tool_name = str(function_call.get("name") or "")
                        row.tool_use_id = str(function_call.get("id") or "")
                    elif isinstance(part.get("text"), str) and part.get("thought"):
                        if not opts.include_thinking:
                            continue
                        row = base("thinking", part["text"])
                    elif isinstance(part.get("text"), str) and part["text"]:
                        row = base("assistant", part["text"])
                    else:
                        continue
                    row.model = model
                    yield row
            elif record_type == "tool_result":
                tool_call_result = record.get("toolCallResult")
                tool_call_result = tool_call_result if isinstance(tool_call_result, dict) else {}
                responses = [
                    part["functionResponse"]
                    for part in parts
                    if isinstance(part.get("functionResponse"), dict)
                ]
                first_response = responses[0] if responses else {}
                display = tool_call_result.get("resultDisplay")
                if isinstance(display, str) and display:
                    text = display
                elif first_response:
                    text = _response_text(first_response.get("response"))
                else:
                    text = compact_json(display) if display else ""
                status = str(tool_call_result.get("status") or "")
                if status and status != "success":
                    tag = (
                        status
                        if not tool_call_result.get("errorType")
                        else "%s %s" % (status, tool_call_result.get("errorType"))
                    )
                    text = "[%s] %s" % (tag, text)
                row = base("tool_result", text)
                row.tool_name = str(first_response.get("name") or "")
                row.tool_use_id = str(
                    tool_call_result.get("callId") or first_response.get("id") or ""
                )
                yield row
                if status in DECISION_STATUSES:
                    decision_row = base("system", "permission decision: %s" % status)
                    decision_row.tool_name, decision_row.tool_use_id = (
                        row.tool_name,
                        row.tool_use_id,
                    )
                    yield decision_row
            elif record_type == "system":
                subtype = str(record.get("subtype") or "system")
                payload = record.get("systemPayload")
                text = system_text(subtype, payload)
                model = ""
                if subtype == "session_model" and isinstance(payload, dict):
                    model = str(payload.get("modelId") or "")
                    if model:
                        change = track_change("model", models, session_id, model)
                        text = "%s auth=%s" % (
                            change or "model: %s" % model,
                            payload.get("authType") or "",
                        )
                elif subtype == "session_approval_mode" and isinstance(payload, dict):
                    mode = str(payload.get("mode") or "")
                    if mode:
                        change = track_change("permission mode", modes, session_id, mode)
                        extras = [
                            " %s=%s" % (key, payload[key])
                            for key in ("prePlanMode", "planExecutionMode")
                            if payload.get(key)
                        ]
                        text = (change or "permission mode: %s" % mode) + "".join(extras)
                row = base("system", text)
                row.model = model
                yield row
                event = payload.get("uiEvent") if isinstance(payload, dict) else None
                if (
                    subtype == "ui_telemetry"
                    and isinstance(event, dict)
                    and event.get("event.name") in TOOL_CALL_EVENTS
                    and event.get("decision")
                ):
                    decision_row = base(
                        "system",
                        "permission decision: %s (tool_call ui_telemetry)" % event["decision"],
                    )
                    decision_row.tool_name = str(event.get("function_name") or "")
                    decision_row.tool_use_id = str(event.get("call_id") or "")
                    yield decision_row
        if first:
            yield from self._worktree_rows(artifact, first, errors)
        yield from error_rows(self, artifact, errors, session_id, project_path)

    def _worktree_rows(self, artifact: Artifact, first: tuple, errors: list) -> Iterator[Row]:
        """Yield the `scratch directory:` row of a session's `.worktree.json` sidecar.

        The sidecar marks a session working in an `enter_worktree` checkout. An
        absent sidecar is silent; one that cannot be read or is not a JSON object
        is appended to `errors`. The row takes the session's first record's
        line, time and session id; its `project_path` is the worktree, which the
        analyzer then lists as a scratch directory.
        """
        name = artifact.disk_path.name[: -len(".jsonl")] + ".worktree.json"
        path = artifact.disk_path.with_name(name)
        try:
            present = stat.S_ISREG(os.lstat(path).st_mode)
        except OSError:
            present = False
        if not present:
            return
        content = file_bytes(path, errors)
        if content is None:
            return
        try:
            doc = json.loads(content.decode("utf-8", errors="replace"))
        except ValueError:
            doc = None
        if not isinstance(doc, dict):
            errors.append((0, "%s is not a JSON object" % name, ""))
            return
        worktree_path = str(doc.get("worktreePath") or "")
        if not worktree_path:
            return
        slug = re.split(r"[/\\]", worktree_path.rstrip("/\\"))[-1]
        line, timestamp, session_id, _ = first
        row = self.base_row(artifact)
        row.source_line = line
        row.turn_type = "system"
        row.timestamp_utc = timestamp
        row.session_id = session_id
        row.project_path = worktree_path
        row.text = "scratch directory: %s (enter_worktree %s)" % (worktree_path, slug)
        yield row

    def _parse_logs(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        for index, entry in self._iter_array(artifact.disk_path, errors):
            row = self.base_row(artifact)
            row.source_line = index
            row.timestamp_utc = to_utc(entry.get("timestamp"))
            row.session_id = str(entry.get("sessionId") or "")
            entry_type = entry.get("type")
            message = entry.get("message")
            if entry_type == "user":
                row.turn_type = "user"
                text = message if isinstance(message, str) else compact_json(message)
            else:
                row.turn_type = "system"
                event = message
                if isinstance(message, str):
                    try:
                        event = json.loads(message)
                    except ValueError:
                        event = message
                if entry_type == "model_switch" and isinstance(event, dict):
                    text = "model: %s -> %s reason=%s" % (
                        event.get("fromModel"),
                        event.get("toModel"),
                        event.get("reason"),
                    )
                    row.model = str(event.get("toModel") or "")
                else:
                    text = "%s: %s" % (
                        entry_type,
                        event if isinstance(event, str) else compact_json(event),
                    )
            row.text = compact(text)
            yield row
        yield from error_rows(self, artifact, errors)

    @staticmethod
    def _iter_array(path, errors: list):
        """Yield (1-based index, entry) from a JSON array, decoding element by element.

        A file cut mid-write still yields the entries before the cut. A parse
        failure is recorded in `errors` as (index, message, raw) and ends the
        walk; raw is the text from the element the failed decode started at
        to the end of the file, less a trailing line terminator, since a
        broken element has no boundary of its own. A whole-file error has
        raw `""`; a file that cannot be read is one entry and yields nothing.
        """
        content = file_bytes(path, errors)
        if content is None:
            return
        data = content.decode("utf-8", errors="replace")
        decoder = json.JSONDecoder()
        offset = 0
        length = len(data)

        def skip(position: int) -> int:
            while position < length and data[position] in " \t\r\n":
                position += 1
            return position

        offset = skip(offset)
        if offset >= length:
            return
        if data[offset] != "[":
            errors.append((0, "not a JSON array", ""))
            return
        offset = skip(offset + 1)
        index = 0
        while offset < length and data[offset] != "]":
            index += 1
            try:
                value, offset = decoder.raw_decode(data, offset)
            except ValueError as error:
                errors.append(
                    (index, "unparseable record %d (%s)" % (index, error), tail_text(data, offset))
                )
                return
            if isinstance(value, dict):
                yield index, value
            offset = skip(offset)
            if offset < length and data[offset] == ",":
                offset = skip(offset + 1)
        if offset >= length:
            errors.append((index + 1, "unterminated JSON array", ""))


MATCH_TOLERANCES[QwenCodeParser.agent] = match_tolerance
