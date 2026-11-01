"""Parse OpenClaw (formerly Clawdbot and Moltbot) agent transcripts and Gateway state.

Follows `research/openclaw.md`. Validated against source, openclaw/openclaw at
fc3490a (state schema 36, agent schema 38), with the older shapes of 3b16db7 and
146284a, and a synthetic fixture; no real install. The state directory is
`.openclaw`, `.openclaw-<profile>`, `.clawdbot` or `.moltbot` (`STATE`). Files read:

- `agents/<id>/agent/openclaw-agent.sqlite`, through `sqlite_util`: the
  entries of `transcript_events` (`event_json`, or a zstd frame in
  `event_zstd`), the generations of `session_windows`, the reset and deleted
  generations of `session_transcript_archives` and the `sqlite`-stored
  `session_transcript_cold_archives` (or their
  `session_transcript_cold_archive_chunks`), and `session_nodes.entry_json`
  (`publicShare`, `delivery`, `modelOverride`, `SESSION_PERMISSION_KEYS`,
  `archivedAt`). Also `session_conversations` with `conversations`,
  `session_pending_inputs` and the `session_transcript_fts_content` index
  (joined on the entry id, keeping only what the transcript lost),
  `conversation_deliveries` and `memory_session_tombstones`.
- `state/openclaw.sqlite`, the Gateway's state: the `STATE_QUERIES` tables.
- `[agents/<id>/]sessions/<id>.jsonl` legacy transcripts, with the session
  key from the sibling `sessions.json`, and `sessions/cold/<sha256>.jsonl.zst`.
- Secondary sources (files repeating the store's entries): the published
  `.jsonl.reset.*` and `.jsonl.deleted.*` archives, `.checkpoint.<uuid>.jsonl`
  snapshots and `.pre-doctor-*-repair-*.bak` copies. Not read:
  `*.trajectory.jsonl`, `*.migrated` and `.jsonl.bak.<ts>`.

Entries share pi's format and model rows (`pi.model_text`, `pi.switch_text`).
A PostgreSQL anchor database is one `parser:` row; credential tables are never
read.
"""

from __future__ import annotations

import json
import re
import sqlite3
from collections.abc import Iterator

from ..inputs import Artifact
from ..model import GIT_REMOTE_PREFIX, Row, compact
from ..sqlite_util import NOT_SQLITE, open_copy, sqlite_problem, table_names
from ..timeutil import to_utc
from .base import Options, Parser, compact_json, error_rows, file_bytes, line_text, text_of
from .pi import model_text, switch_text
from .zed import decompress

STATE = r"(?:\.openclaw(?:-[^/]+)?|\.clawdbot|\.moltbot)"
AGENT_DATABASE_PATTERN = re.compile(r"^" + STATE + r"/agents/[^/]+/agent/openclaw-agent\.sqlite$")
STATE_DATABASE_PATTERN = re.compile(r"^" + STATE + r"/state/openclaw\.sqlite$")
SESSIONS = r"^" + STATE + r"/(?:agents/[^/]+/)?sessions/"
ARCHIVE_SUFFIX = r"\.jsonl\.(reset|deleted)\.\d{4}-\d\d-\d\dT\d\d-\d\d-\d\d(?:\.\d{3})?Z(?:\.[0-9a-f]{32})?(?:\.zst)?"
# A pre-compaction checkpoint and a pre-doctor repair backup.
COPY_SUFFIX = (
    r"\.checkpoint\.[0-9a-fA-F-]{36}\.jsonl"
    r"|\.jsonl\.pre-doctor-(?:branch|openai-codex)-repair-"
    r"\d{4}-\d\d-\d\dT\d\d-\d\d-\d\d-\d{3}Z\.bak"
)
JSONL_PATTERN = re.compile(
    SESSIONS
    + r"(?P<name>[^/]+?)(?:(?P<archive>"
    + ARCHIVE_SUFFIX
    + r")|(?P<copy>"
    + COPY_SUFFIX
    + r")|\.jsonl)$"
)
COLD_PATTERN = re.compile(SESSIONS + r"cold/[0-9a-f]{64}\.jsonl\.zst$")
SKIP_NAME_PATTERN = re.compile(r"\.trajectory$")
ZSTD_MAGIC = b"\x28\xb5\x2f\xfd"
# Channels that are not a messaging platform: the Control UI and the
# internal wake sources.
NOT_PLATFORMS = frozenset(("webchat", "heartbeat", "cron", "webhook", "voice", "sessions_send"))
# Per-session permission state in `session_nodes.entry_json`.
SESSION_PERMISSION_KEYS = (
    "elevatedLevel",
    "execHost",
    "execNode",
    "execCwd",
    "toolOverrides",
    "sendPolicy",
)
# The GitHub publication requests of `state/openclaw.sqlite`, each naming the
# GitHub account that pushed or opened the pull request
# (`src/state/openclaw-state-schema.sql`; the lines are cited in research/openclaw.md).
GITHUB_PUBLICATION_TABLES = (
    "github_publication_requests",
    "github_personal_publication_requests",
    "github_repository_publication_requests",
)
# The `state/openclaw.sqlite` tables read, in the order their rows are yielded.
STATE_QUERIES = (
    (
        "backup_runs",
        "select rowid as _rowid, id, created_at, archive_path, status, manifest_json"
        " from backup_runs order by created_at, id",
    ),
    (
        "operator_approvals",
        "select rowid as _rowid, approval_id, kind, status, decision, terminal_reason,"
        " resolver_kind, source_session_id, source_tool_call_id, source_tool_name,"
        " created_at_ms, resolved_at_ms from operator_approvals order by created_at_ms, rowid",
    ),
    (
        "exec_approvals_config",
        "select rowid as _rowid, raw_json, updated_at_ms from exec_approvals_config order by rowid",
    ),
    (
        "operator_approval_standing_grants",
        "select rowid as _rowid, grant_id, minted_by_approval_id, agent_id, cron_job_id,"
        " operation_binding, created_at_ms, expires_at_ms, revoked_at_ms, use_count"
        " from operator_approval_standing_grants order by created_at_ms, rowid",
    ),
    (
        "channel_pairing_allow_entries",
        "select rowid as _rowid, channel_key, account_id, entry, updated_at"
        " from channel_pairing_allow_entries order by channel_key, account_id, sort_order",
    ),
    (
        "channel_ingress_events",
        "select rowid as _rowid, channel_id, account_id, status, payload_json, received_at"
        " from channel_ingress_events order by received_at, rowid",
    ),
    *(
        (
            table,
            "select rowid as _rowid, session_id, identity_source, identity_account_id,"
            " identity_login, status, repository, branch, pull_request_url, created_at_ms"
            " from %s order by created_at_ms, rowid" % table,
        )
        for table in GITHUB_PUBLICATION_TABLES
    ),
    (
        "projects",
        "select rowid as _rowid, repo_root, origin_url, created_at_ms"
        " from projects order by created_at_ms, rowid",
    ),
)


def tool_args_text(args) -> str:
    """Return a tool call's `command`, `path` or `url` argument, else its arguments as JSON."""
    if isinstance(args, str):
        try:
            args = json.loads(args)
        except ValueError:
            return args
    if isinstance(args, dict):
        for key in ("command", "path", "url"):
            if isinstance(args.get(key), str) and args[key]:
                return args[key]
    if args in (None, "", {}):
        return ""
    return compact_json(args)


def _maybe_zstd(blob: bytes) -> tuple[bytes | None, str]:
    """Return (data, error): a zstd frame decompressed, anything else as it is."""
    blob = bytes(blob)
    if not blob.startswith(ZSTD_MAGIC):
        return blob, ""
    try:
        return decompress(blob), ""
    except Exception as error:  # noqa: BLE001 - zstandard.ZstdError or a stream read error
        return None, "%s: %s" % (type(error).__name__, error)


def _iter_lines(data: bytes, errors: list) -> Iterator[tuple[int, dict]]:
    """Yield (line number, record) for each JSON object line of `data`.

    A line that does not decode, or decodes to a value other than an object
    (an array, a string, a number, `null`), is appended to `errors` as (line,
    message, line text), the message `unparseable line (not a JSON object)`
    for the second kind, so `error_rows` reports it in full.
    """
    for line_number, raw in enumerate(data.split(b"\n"), 1):
        stripped = raw.strip()
        if not stripped:
            continue
        try:
            record = json.loads(stripped.decode("utf-8", errors="replace"))
        except ValueError as error:
            errors.append((line_number, "unparseable line (%s)" % error, line_text(raw)))
            continue
        if not isinstance(record, dict):
            errors.append((line_number, "unparseable line (not a JSON object)", line_text(raw)))
            continue
        yield line_number, record


def _platform(channel) -> bool:
    """Return True for a channel id that names a messaging platform."""
    return isinstance(channel, str) and bool(channel) and channel not in NOT_PLATFORMS


def _delivery_relay(delivery) -> str:
    """Return the `session relayed:` text of an external `entry_json.delivery`, else "".

    The delivery's `kind` is `external` and its `route.channel` a platform
    (`src/config/sessions/types.ts:60-83`; `route` is a `ChannelRouteRef`,
    `src/plugin-sdk/channel-route.ts:18-25`).
    """
    if not isinstance(delivery, dict) or delivery.get("kind") != "external":
        return ""
    route = delivery.get("route")
    route = route if isinstance(route, dict) else {}
    origin = delivery.get("origin")
    origin = origin if isinstance(origin, dict) else {}
    channel = route.get("channel")
    if not _platform(channel):
        return ""
    return (
        "session relayed: %s account %s %s from %s to %s (from openclaw-agent.sqlite"
        " session_nodes.entry_json delivery, updated_at)"
        % (
            channel,
            route.get("accountId") or origin.get("accountId") or "-",
            origin.get("chatType") or "-",
            origin.get("from") or "-",
            origin.get("to") or "-",
        )
    )


def _override_text(entry: dict) -> str:
    """Return the `model:` text of a session's `modelOverride` (`types.ts:502-516`)."""
    origin = "/".join(
        str(entry[key])
        for key in ("modelOverrideFallbackOriginProvider", "modelOverrideFallbackOriginModel")
        if entry.get(key)
    )
    return (
        "model: %s provider=%s source=%s%s (from openclaw-agent.sqlite"
        " session_nodes.entry_json modelOverride, updated_at)"
        % (
            entry.get("modelOverride"),
            entry.get("providerOverride") or "-",
            entry.get("modelOverrideSource") or "-",
            " fallback_from=%s" % origin if origin else "",
        )
    )


def _policy_settings(policy) -> list[str]:
    """Return `<key>=<value>` per policy field, allowlist entry and MCP tool grant.

    The policy is one `defaults` or `agents.<id>` object of an exec approvals
    file (`packages/gateway-protocol/src/schema/exec-approvals.ts:15-75`).
    """
    if not isinstance(policy, dict):
        return []
    out = []
    for key in ("security", "ask", "askFallback", "autoAllowSkills"):
        value = policy.get(key)
        if value is not None:
            out.append("%s=%s" % (key, json.dumps(value) if isinstance(value, bool) else value))
    allowlist = policy.get("allowlist")
    for entry in allowlist if isinstance(allowlist, list) else []:
        if isinstance(entry, dict) and entry.get("pattern") is not None:
            arg = " argPattern=%s" % entry["argPattern"] if entry.get("argPattern") else ""
            out.append(
                "allowlist=%s%s source=%s" % (entry["pattern"], arg, entry.get("source") or "-")
            )
    grants = policy.get("mcpTools")
    for grant in grants if isinstance(grants, list) else []:
        if isinstance(grant, dict):
            out.append(
                "mcpTools=%s/%s source=%s"
                % (grant.get("server") or "-", grant.get("tool") or "-", grant.get("source") or "-")
            )
    return out


def _grant_text(row: dict) -> str:
    """Return the `permission setting:` text of an `operator_approval_standing_grants` row.

    That is the grant an `allow-always` decision on a cron job mints.
    """
    return (
        "permission setting: standing grant %s allow-always cron job %s agent %s binding %s"
        " approval %s expires=%s revoked=%s uses=%s (from state/openclaw.sqlite"
        " operator_approval_standing_grants, created_at_ms)"
        % (
            row.get("grant_id") or "-",
            row.get("cron_job_id") or "-",
            row.get("agent_id") or "-",
            row.get("operation_binding") or "-",
            row.get("minted_by_approval_id") or "-",
            to_utc(row.get("expires_at_ms")) or "-",
            to_utc(row.get("revoked_at_ms")) or "-",
            row.get("use_count") if row.get("use_count") is not None else "-",
        )
    )


def _publication_text(table: str, row: dict) -> str:
    """Return the `source host:` text of a GitHub publication request.

    The text names the account the Gateway pushed or opened the pull request
    as, and what it published.
    """
    return (
        "source host: github account %s id %s identity %s; publication %s repository %s"
        " branch %s pull request %s (from state/openclaw.sqlite %s, created_at_ms)"
        % (
            row.get("identity_login") or "-",
            row.get("identity_account_id") if row.get("identity_account_id") is not None else "-",
            row.get("identity_source") or "-",
            row.get("status") or "-",
            row.get("repository") or "-",
            row.get("branch") or "-",
            row.get("pull_request_url") or "-",
            table,
        )
    )


class _DatabaseState:
    """Hold what one agent database's passes share.

    That is the rows to attach after a generation's session-start row, the
    transcript entry ids already yielded (for the exact joins of the
    full-text index and the pending inputs) and each generation's `cwd`.
    """

    def __init__(self) -> None:
        # {generation: [(source_line, timestamp, text, model)]}
        self.attached: dict[str, list[tuple[int, str, str, str]]] = {}
        self.ids: set[tuple[str, str]] = set()
        self.cwds: dict[str, str] = {}

    def attach(
        self, session_id: str, line: int, timestamp: str, text: str, model: str = ""
    ) -> None:
        self.attached.setdefault(session_id, []).append((line, timestamp, text, model))


class _Session:
    """State carried across one transcript's entries."""

    def __init__(
        self,
        session_id: str = "",
        key: str = "",
        extra: str = "",
        model: str = "",
        provider: str = "",
    ):
        self.session_id = session_id
        self.key = key
        self.extra = extra
        self.model = model
        self.provider = provider
        self.cwd = ""
        self.header_id = False  # True: the header `id` replaces session_id
        self.started = False
        self.seen: dict = {}  # the model in use, for the `model:` rows
        # The agent database's shared state; the session-start row pops the
        # rows attached to its generation and yields them.
        self.database_state: _DatabaseState | None = None


class OpenClawParser(Parser):
    """Parse OpenClaw's agent databases, Gateway state database and JSONL transcripts."""

    agent = "openclaw"
    name = "openclaw"
    has_secondary = True
    records_sync = True
    records_relay = True

    def wants(self, artifact: Artifact) -> bool:
        """Return True for an agent or state database, a cold archive or a transcript file."""
        rel = artifact.rel
        if (
            AGENT_DATABASE_PATTERN.match(rel)
            or COLD_PATTERN.match(rel)
            or STATE_DATABASE_PATTERN.match(rel)
        ):
            return True
        match = JSONL_PATTERN.match(rel)
        return bool(match) and not SKIP_NAME_PATTERN.search(match.group("name"))

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for a published archive, checkpoint or pre-doctor copy.

        The database and the plain legacy `.jsonl` are primary.
        """
        match = JSONL_PATTERN.match(artifact.rel)
        return bool(match) and bool(match.group("archive") or match.group("copy"))

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one OpenClaw file."""
        if AGENT_DATABASE_PATTERN.match(artifact.rel):
            problem = sqlite_problem(artifact.disk_path)
            if problem:
                yield from error_rows(self, artifact, [(0, problem)])
            else:
                yield from self._parse_database(artifact, opts)
        elif STATE_DATABASE_PATTERN.match(artifact.rel):
            yield from self._parse_state_database(artifact)
        else:
            yield from self._parse_file(artifact, opts)

    # -- rows -------------------------------------------------------------------
    def _row(
        self,
        artifact: Artifact,
        session: _Session,
        line: int,
        timestamp: str,
        turn_type: str,
        text: str,
        opts: Options,
        model: str = "",
        tool_name: str = "",
        tool_use_id: str = "",
    ) -> Row:
        row = self.base_row(artifact)
        row.source_line = line
        row.session_id = session.session_id
        row.project_path = session.cwd
        row.timestamp_utc = timestamp
        row.turn_type = turn_type
        row.model = model
        row.tool_name = tool_name
        row.tool_use_id = tool_use_id
        row.text = compact(text)
        return row

    def _start(
        self,
        artifact: Artifact,
        session: _Session,
        line: int,
        timestamp: str,
        header: dict | None,
        opts: Options,
    ) -> Iterator[Row]:
        """Yield the session-start row, then a `model:` row when the window names the model."""
        session.started = True
        header = header or {}
        if header.get("id") and (session.header_id or not session.session_id):
            session.session_id = str(header["id"])
        if isinstance(header.get("cwd"), str):
            session.cwd = header["cwd"]
        parent = header.get("parentSession")
        text = " ".join(
            part
            for part in (
                "session start:",
                "key=%s" % session.key if session.key else "",
                session.extra,
                "version=%s" % header["version"] if header.get("version") is not None else "",
                "parent_session=%s" % parent if parent else "",
            )
            if part
        )
        timestamp = timestamp or to_utc(header.get("timestamp"))
        yield self._row(artifact, session, line, timestamp, "system", text, opts, session.model)
        model_line = model_text(session.seen, session.session_id, session.model, session.provider)
        if model_line:
            yield self._row(
                artifact, session, line, timestamp, "system", model_line, opts, session.model
            )
        if session.database_state is not None:
            session.database_state.cwds.setdefault(session.session_id, session.cwd)
            for (
                attached_line,
                attached_timestamp,
                attached_text,
                attached_model,
            ) in session.database_state.attached.pop(session.session_id, []):
                yield self._row(
                    artifact,
                    session,
                    attached_line,
                    attached_timestamp,
                    "system",
                    attached_text,
                    opts,
                    attached_model,
                )

    def _entry_rows(
        self,
        artifact: Artifact,
        session: _Session,
        line: int,
        record: dict,
        created_at,
        opts: Options,
    ) -> Iterator[Row]:
        entry_type = record.get("type")
        message = record.get("message")
        if not isinstance(message, dict):
            message = {}
        timestamp = (
            to_utc(record.get("timestamp"))
            or to_utc(message.get("timestamp"))
            or to_utc(created_at)
        )
        if entry_type == "session":
            yield from self._start(artifact, session, line, timestamp, record, opts)
            return
        if not session.started:
            yield from self._start(artifact, session, line, timestamp, None, opts)
        if session.database_state is not None and record.get("id") not in (None, ""):
            session.database_state.ids.add((session.session_id, str(record["id"])))
        if entry_type == "message":
            yield from self._message_rows(artifact, session, line, timestamp, record, message, opts)
            return

        def system(text: str, model: str = "") -> Row:
            return self._row(artifact, session, line, timestamp, "system", text, opts, model)

        if entry_type == "model_change":
            session.model = str(record.get("modelId") or "")
            model_line = switch_text(
                session.seen, session.session_id, session.model, str(record.get("provider") or "")
            )
            if model_line:
                yield system(model_line, session.model)
        elif entry_type == "thinking_level_change":
            yield system("thinking level: %s" % (record.get("thinkingLevel") or ""))
        elif entry_type == "compaction":
            tokens = " ".join(
                part
                for part in (
                    "tokens_before=%s" % record["tokensBefore"]
                    if record.get("tokensBefore") is not None
                    else "",
                    "tokens_after=%s" % record["tokensAfter"]
                    if record.get("tokensAfter") is not None
                    else "",
                )
                if part
            )
            yield system(
                "compaction%s: %s"
                % (" (%s)" % tokens if tokens else "", record.get("summary") or "")
            )
        elif entry_type == "reset":
            yield system("reset: %s" % (record.get("reason") or ""))
        elif entry_type == "branch_summary":
            yield system("branch summary: %s" % (record.get("summary") or ""))
        elif entry_type == "custom":
            yield system(
                "custom %s: %s" % (record.get("customType") or "", compact_json(record.get("data")))
            )
        elif entry_type == "custom_message":
            yield system(
                "custom message %s: %s"
                % (record.get("customType") or "", text_of(record.get("content")))
            )
        elif entry_type == "label":
            yield system(
                "label: %s (entry %s)" % (record.get("label") or "", record.get("targetId") or "")
            )
        elif entry_type == "session_info":
            yield system("session name: %s" % (record.get("name") or ""))
        else:
            rest = {
                key: value
                for key, value in record.items()
                if key not in ("type", "id", "parentId", "timestamp")
            }
            yield system("%s: %s" % (entry_type or "entry", compact_json(rest)))

    def _message_rows(
        self,
        artifact: Artifact,
        session: _Session,
        line: int,
        timestamp: str,
        record: dict,
        message: dict,
        opts: Options,
    ) -> Iterator[Row]:
        role = message.get("role")
        if role == "user":
            text = text_of(message.get("content"))
            if message.get("runtimeContext") or message.get("runtimeContextCarrier"):
                yield self._row(
                    artifact, session, line, timestamp, "system", "runtime context: " + text, opts
                )
            else:
                yield self._row(artifact, session, line, timestamp, "user", text, opts)
        elif role == "assistant":
            requested = str(message.get("model") or "")
            model = str(message.get("responseModel") or "") or requested or session.model
            if model:
                session.model = model
            if message.get("responseModel") or requested:
                model_line = model_text(
                    session.seen,
                    session.session_id,
                    model,
                    str(message.get("provider") or ""),
                    requested,
                )
                if model_line:
                    yield self._row(
                        artifact, session, line, timestamp, "system", model_line, opts, model
                    )
            content = message.get("content")
            if isinstance(content, str):
                content = [{"type": "text", "text": content}]
            for block in content if isinstance(content, list) else []:
                if not isinstance(block, dict):
                    continue
                block_type = block.get("type")
                if block_type == "text":
                    if block.get("text"):
                        yield self._row(
                            artifact,
                            session,
                            line,
                            timestamp,
                            "assistant",
                            str(block["text"]),
                            opts,
                            model,
                        )
                elif block_type == "thinking":
                    if opts.include_thinking:
                        thinking = block.get("thinking") or (
                            "[redacted thinking]" if block.get("redacted") else ""
                        )
                        if thinking:
                            yield self._row(
                                artifact,
                                session,
                                line,
                                timestamp,
                                "thinking",
                                str(thinking),
                                opts,
                                model,
                            )
                elif block_type == "toolCall":
                    yield self._row(
                        artifact,
                        session,
                        line,
                        timestamp,
                        "tool_use",
                        tool_args_text(block.get("arguments")),
                        opts,
                        model,
                        str(block.get("name") or ""),
                        str(block.get("id") or ""),
                    )
                elif block_type == "image":
                    yield self._row(
                        artifact, session, line, timestamp, "assistant", "[image]", opts, model
                    )
            stop = message.get("stopReason")
            if stop in ("error", "aborted") or message.get("errorMessage"):
                yield self._row(
                    artifact,
                    session,
                    line,
                    timestamp,
                    "system",
                    "assistant %s: %s" % (stop or "error", message.get("errorMessage") or ""),
                    opts,
                    model,
                )
        elif role == "toolResult":
            text = text_of(message.get("content"))
            if message.get("isError"):
                text = "[error] " + text
            yield self._row(
                artifact,
                session,
                line,
                timestamp,
                "tool_result",
                text,
                opts,
                "",
                str(message.get("toolName") or ""),
                str(message.get("toolCallId") or ""),
            )
        elif role == "bashExecution":
            tool_use_id = str(record.get("id") or "")
            yield self._row(
                artifact,
                session,
                line,
                timestamp,
                "tool_use",
                str(message.get("command") or ""),
                opts,
                "",
                "bash",
                tool_use_id,
            )
            status = "exit=%s" % message.get("exitCode")
            if message.get("cancelled"):
                status += " cancelled"
            if message.get("truncated"):
                status += " truncated"
            yield self._row(
                artifact,
                session,
                line,
                timestamp,
                "tool_result",
                "[%s] %s" % (status, message.get("output") or ""),
                opts,
                "",
                "bash",
                tool_use_id,
            )
        elif role in ("branchSummary", "compactionSummary"):
            label = "branch summary" if role == "branchSummary" else "compaction summary"
            yield self._row(
                artifact,
                session,
                line,
                timestamp,
                "system",
                "%s: %s" % (label, message.get("summary") or ""),
                opts,
            )
        elif role == "custom":
            yield self._row(
                artifact,
                session,
                line,
                timestamp,
                "system",
                "custom %s: %s"
                % (message.get("customType") or "", text_of(message.get("content"))),
                opts,
            )
        else:
            yield self._row(
                artifact,
                session,
                line,
                timestamp,
                "system",
                "%s: %s"
                % (role or "message", text_of(message.get("content")) or compact_json(message)),
                opts,
            )

    def _bad(
        self,
        artifact: Artifact,
        session: _Session,
        line: int,
        text: str,
        opts: Options,
        timestamp: str = "",
    ) -> Row:
        return self._row(artifact, session, line, timestamp, "system", "parser: " + text, opts)

    def _blob_errors(
        self, artifact: Artifact, session: _Session, rowid: int, label: str, errors: list
    ) -> Iterator[Row]:
        """Yield `error_rows` for the bad lines of an archive blob.

        `source_line` is the archive row's rowid and the message is prefixed
        with `label` and the line's number in the blob (`archive <name> line
        3: unparseable line (<reason>)`).
        """
        located = [
            (rowid, "%s line %d: %s" % (label, line_number, message), raw)
            for line_number, message, raw in errors
        ]
        yield from error_rows(self, artifact, located, session.session_id, session.cwd)

    def _records(
        self, artifact: Artifact, session: _Session, records, opts: Options, fixed_line: int = 0
    ) -> Iterator[Row]:
        """Yield the rows of (line, record) pairs of a JSONL transcript or a cold archive.

        `fixed_line` overrides the line for blobs.
        """
        for line_number, record in records:
            line = fixed_line or line_number
            if "kind" in record and "type" not in record:
                kind = record.get("kind")
                if kind == "header" and not session.session_id:
                    session.session_id = str(record.get("sessionId") or "")
                if kind != "event" or not isinstance(record.get("row"), dict):
                    continue
                inner = record["row"]
                try:
                    entry = json.loads(inner.get("event_json") or "")
                except (TypeError, ValueError) as error:
                    yield self._bad(
                        artifact,
                        session,
                        line,
                        "cold archive event seq %s unreadable: %s" % (inner.get("seq"), error),
                        opts,
                        to_utc(inner.get("created_at")),
                    )
                    continue
                if isinstance(entry, dict):
                    yield from self._entry_rows(
                        artifact, session, line, entry, inner.get("created_at"), opts
                    )
                continue
            yield from self._entry_rows(artifact, session, line, record, None, opts)

    # -- JSONL files ----------------------------------------------------------------
    def _parse_file(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        match = JSONL_PATTERN.match(artifact.rel)
        session = _Session(session_id=match.group("name") if match else "")
        session.header_id = True
        if match and match.group("archive"):
            session.extra = "archive=%s" % match.group("archive")[len(".jsonl.") :]
        elif match and match.group("copy"):
            session.extra = "copy=%s" % match.group("copy").lstrip(".")
        key_error = ""
        if match:
            session.key, key_error = self._legacy_key(artifact, session.session_id)
        errors: list = []
        raw = file_bytes(artifact.disk_path, errors)
        if raw is None:
            if key_error:
                errors.append((0, key_error, ""))
            yield from error_rows(self, artifact, errors, session.session_id, session.cwd)
            return
        data, error = _maybe_zstd(raw)
        if data is None:
            yield self._bad(artifact, session, 1, "could not decompress (%s)" % error, opts)
            if key_error:
                yield from error_rows(
                    self, artifact, [(0, key_error, "")], session.session_id, session.cwd
                )
            return
        yield from self._records(artifact, session, _iter_lines(data, errors), opts)
        if key_error:
            errors.append((0, key_error, ""))
        yield from error_rows(self, artifact, errors, session.session_id, session.cwd)

    @staticmethod
    def _legacy_key(artifact: Artifact, session_id: str) -> tuple[str, str]:
        """Return (session key, error) for `session_id` from the sibling legacy `sessions.json`.

        A missing file is no error; a file that cannot be read, does not
        decode or is not a JSON object returns the whole-file message
        (`unreadable_message`'s, `sessions.json unparseable (<reason>)`,
        `sessions.json is not a JSON object`) for a `parser:` row of the
        transcript that looked the key up.
        """
        store = artifact.disk_path.parent / "sessions.json"
        if store.is_symlink() or not store.is_file():
            return "", ""
        read_errors: list = []
        content = file_bytes(store, read_errors)
        if content is None:
            return "", read_errors[0][1]
        try:
            data = json.loads(content.decode("utf-8", errors="replace"))
        except ValueError as error:
            return "", "sessions.json unparseable (%s)" % error
        if not isinstance(data, dict):
            return "", "sessions.json is not a JSON object"
        for key, entry in data.items():
            if isinstance(entry, dict) and entry.get("sessionId") == session_id:
                return str(key), ""
        return "", ""

    # -- SQLite store -------------------------------------------------------------------
    def _parse_database(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        database_state = _DatabaseState()
        with open_copy(artifact.disk_path) as connection:
            tables = table_names(connection)
            if "transcript_events" not in tables and "openclaw_engine_anchor" in tables:
                yield from error_rows(self, artifact, [self._anchor_error(connection, "agent")])
                return
            windows: dict[str, dict] = {}
            if "session_windows" in tables:
                for window in connection.execute("select rowid as _rowid, * from session_windows"):
                    window = dict(window)
                    windows[str(window.get("session_id"))] = window
            relayed = self._database_relays(connection, tables, windows, database_state)
            nodes = self._database_nodes(connection, errors) if "session_nodes" in tables else []
            self._attach_nodes(nodes, relayed, database_state)
            if "transcript_events" in tables:
                yield from self._database_events(
                    artifact, connection, windows, database_state, opts
                )
            if "session_transcript_archives" in tables:
                yield from self._database_archives(
                    artifact, connection, windows, database_state, opts
                )
            if "session_transcript_cold_archives" in tables:
                chunked = "session_transcript_cold_archive_chunks" in tables
                yield from self._database_cold(
                    artifact, connection, windows, database_state, opts, chunked
                )
            if "session_pending_inputs" in tables:
                yield from self._database_pending(artifact, connection, database_state, errors)
            if "session_transcript_fts_content" in tables:
                yield from self._database_fts(artifact, connection, database_state)
            if "conversation_deliveries" in tables and "conversations" in tables:
                current = {
                    str(node["session_key"]): str(node["current_session_id"]) for node in nodes
                }
                yield from self._database_deliveries(artifact, connection, database_state, current)
            if "memory_session_tombstones" in tables:
                yield from self._database_tombstones(artifact, connection, database_state)
        # Rows attached to a generation with no transcript rows in the database.
        for session_id, items in list(database_state.attached.items()):
            session = _Session(session_id)
            session.cwd = database_state.cwds.get(session_id, "")
            for line, timestamp, text, model in items:
                yield self._row(artifact, session, line, timestamp, "system", text, opts, model)
        yield from error_rows(self, artifact, errors)

    @staticmethod
    def _anchor_error(connection, label: str) -> tuple[int, str, str]:
        """Return the whole-file error of a PostgreSQL engine anchor.

        The anchor is the `openclaw_engine_anchor (store_id, engine, schema)`
        table that replaces the store when `database.engine` is `postgres`
        (`src/infra/database-engine/open.ts:149`).
        """
        try:
            row = connection.execute(
                "select store_id, schema from openclaw_engine_anchor"
            ).fetchone()
        except sqlite3.DatabaseError:
            row = None
        store, schema = (row["store_id"], row["schema"]) if row is not None else ("-", "-")
        where = "transcripts are" if label == "agent" else "the Gateway state is"
        return (
            0,
            "%s database is a PostgreSQL anchor (store %s, schema %s): %s on the"
            " database server" % (label, store, schema, where),
            "",
        )

    @staticmethod
    def _database_nodes(connection, errors: list) -> list[dict]:
        """Return the `session_nodes` rows with `entry_json` decoded into `entry`.

        An `entry_json` that does not decode is appended to `errors`.
        """
        nodes: list[dict] = []
        for row in connection.execute(
            "select rowid as _rowid, session_key, current_session_id, entry_json, updated_at"
            " from session_nodes order by rowid"
        ):
            row = dict(row)
            raw = row["entry_json"] if isinstance(row["entry_json"], str) else ""
            try:
                entry = json.loads(raw)
            except ValueError as error:
                errors.append(
                    (row["_rowid"], "session_nodes entry_json unparseable (%s)" % error, raw)
                )
                continue
            row["entry"] = entry if isinstance(entry, dict) else {}
            nodes.append(row)
        return nodes

    @staticmethod
    def _attach_nodes(nodes: list[dict], relayed: set, database_state: _DatabaseState) -> None:
        """Attach the rows of each session's metadata entry to its current generation.

        The rows are the public share (to the generation it publishes), the
        external delivery route when no conversation or window recorded the
        relay, the model override, the per-session permission settings and
        the archive marker.
        """
        for node in nodes:
            entry, line = node["entry"], node["_rowid"]
            session_id = str(node["current_session_id"] or "")
            timestamp = to_utc(node.get("updated_at"))
            grant = entry.get("publicShare")
            if isinstance(grant, dict) and grant.get("id"):
                database_state.attach(
                    str(grant.get("sessionId") or session_id),
                    line,
                    to_utc(grant.get("createdAt")),
                    "session synced: public share %s generation %s (from openclaw-agent.sqlite"
                    " session_nodes.entry_json, publicShare.createdAt)"
                    % (grant.get("id") or "-", grant.get("sessionId") or "-"),
                )
            relay = _delivery_relay(entry.get("delivery"))
            if relay and session_id not in relayed:
                database_state.attach(session_id, line, timestamp, relay)
            override = entry.get("modelOverride")
            if isinstance(override, str) and override:
                database_state.attach(session_id, line, timestamp, _override_text(entry), override)
            for key in SESSION_PERMISSION_KEYS:
                value = entry.get(key)
                if value in (None, "", {}, []):
                    continue
                database_state.attach(
                    session_id,
                    line,
                    timestamp,
                    "permission setting: %s=%s (from openclaw-agent.sqlite"
                    " session_nodes.entry_json, updated_at)"
                    % (key, value if isinstance(value, str) else compact_json(value)),
                )
            if entry.get("archivedAt") is not None:
                archived_by = entry.get("archivedBy")
                archived_by = archived_by if isinstance(archived_by, dict) else {}
                actor = (
                    ":".join(
                        str(archived_by[field])
                        for field in ("type", "id")
                        if archived_by.get(field)
                    )
                    or "-"
                )
                database_state.attach(
                    session_id,
                    line,
                    to_utc(entry.get("archivedAt")),
                    "session archived: %s by %s (from openclaw-agent.sqlite"
                    " session_nodes.entry_json, archivedAt)"
                    % (entry.get("archiveReason") or "-", actor),
                )

    @staticmethod
    def _database_relays(
        connection, tables: set, windows: dict[str, dict], database_state: _DatabaseState
    ) -> set:
        """Attach the `session relayed:` rows and return the generations that got one.

        One per platform conversation linked to a generation
        (`session_conversations` joined to `conversations`), else one per
        generation whose window names a platform channel, and one per
        generation a Gmail hook started. The Control UI and the internal
        wake channels are not platforms (`_platform`).
        """
        relayed: set = set()
        if "session_conversations" in tables and "conversations" in tables:
            for row in connection.execute(
                "select l.rowid as _rowid, l.session_id, l.role, l.first_seen_at, c.channel,"
                " c.account_id, c.kind, c.peer_id, c.thread_id from session_conversations l"
                " join conversations c using (conversation_id)"
                " order by l.first_seen_at, l.rowid"
            ):
                if not _platform(row["channel"]):
                    continue
                session_id = str(row["session_id"] or "")
                database_state.attach(
                    session_id,
                    row["_rowid"],
                    to_utc(row["first_seen_at"]),
                    "session relayed: %s account %s %s peer %s thread %s role %s (from"
                    " openclaw-agent.sqlite session_conversations and conversations,"
                    " first_seen_at)"
                    % (
                        row["channel"],
                        row["account_id"] or "-",
                        row["kind"] or "-",
                        row["peer_id"] or "-",
                        row["thread_id"] or "-",
                        row["role"] or "-",
                    ),
                )
                relayed.add(session_id)
        for session_id, window in windows.items():
            timestamp = to_utc(window.get("started_at")) or to_utc(window.get("created_at"))
            if session_id not in relayed and _platform(window.get("channel")):
                database_state.attach(
                    session_id,
                    window["_rowid"],
                    timestamp,
                    "session relayed: %s account %s %s (from openclaw-agent.sqlite"
                    " session_windows, started_at)"
                    % (
                        window["channel"],
                        window.get("account_id") or "-",
                        window.get("chat_type") or "-",
                    ),
                )
                relayed.add(session_id)
            if window.get("hook_external_content_source") == "gmail":
                database_state.attach(
                    session_id,
                    window["_rowid"],
                    timestamp,
                    "session relayed: gmail hook (from openclaw-agent.sqlite session_windows"
                    " hook_external_content_source, started_at)",
                )
                relayed.add(session_id)
        return relayed

    def _context_row(
        self,
        artifact: Artifact,
        database_state: _DatabaseState,
        session_id: str,
        line: int,
        timestamp: str,
        turn_type: str,
        text: str,
    ) -> Row:
        """Return a row of a table read after the transcripts, with its generation's `cwd`."""
        session = _Session(session_id)
        session.cwd = database_state.cwds.get(session_id, "")
        return self._row(artifact, session, line, timestamp, turn_type, text, Options())

    def _database_pending(
        self, artifact: Artifact, connection, database_state: _DatabaseState, errors: list
    ) -> Iterator[Row]:
        """Yield the prompts of `session_pending_inputs` the transcript lacks.

        One never consumed (`queued`, `interrupted` or `cancelled` with
        `consumed_event_id` NULL) never reached the transcript and yields its
        `user` row and a `pending input:` row; a consumed one yields its
        `user` row, marked recovered, only when the transcript entry that
        consumed it is not in the database. A `message_json` that does not
        decode is appended to `errors`.
        """
        for record in connection.execute(
            "select seq, input_id, session_id, run_id, message_json, state, accepted_at,"
            " consumed_event_id from session_pending_inputs order by seq"
        ):
            session_id = str(record["session_id"] or "")
            consumed = record["consumed_event_id"]
            if consumed and (session_id, str(consumed)) in database_state.ids:
                continue
            raw = record["message_json"] if isinstance(record["message_json"], str) else ""
            try:
                message = json.loads(raw)
            except ValueError as error:
                errors.append(
                    (
                        record["seq"],
                        "session_pending_inputs message_json unparseable (%s)" % error,
                        raw,
                    )
                )
                continue
            message = message if isinstance(message, dict) else {}
            timestamp = to_utc(record["accepted_at"])
            row = self._context_row(
                artifact,
                database_state,
                session_id,
                record["seq"],
                timestamp,
                "user",
                text_of(message.get("content")),
            )
            if consumed:
                row.recovered = "yes"
                yield row
                continue
            yield row
            yield self._context_row(
                artifact,
                database_state,
                session_id,
                record["seq"],
                timestamp,
                "system",
                "pending input: %s, not consumed by the transcript (input %s run %s; from"
                " openclaw-agent.sqlite session_pending_inputs, accepted_at)"
                % (record["state"] or "-", record["input_id"] or "-", record["run_id"] or "-"),
            )

    def _database_fts(
        self, artifact: Artifact, connection, database_state: _DatabaseState
    ) -> Iterator[Row]:
        """Yield the full-text index's entries the transcript no longer holds, marked recovered.

        The index `session_transcript_fts` is read from its FTS5 content table
        (`c0` text, `c1` session_id, `c2` message_id, `c3` role, `c4`
        timestamp) as a plain table, matched on `(session_id, message_id)`.
        """
        have = {
            record[1]
            for record in connection.execute("pragma table_info(session_transcript_fts_content)")
        }
        if not {"id", "c0", "c1", "c2", "c3", "c4"} <= have:
            return
        for record in connection.execute(
            "select id, c0, c1, c2, c3, c4 from session_transcript_fts_content order by id"
        ):
            session_id, message_id = str(record["c1"] or ""), str(record["c2"] or "")
            if message_id and (session_id, message_id) in database_state.ids:
                continue
            role = str(record["c3"] or "")
            text = compact(record["c0"])
            timestamp = to_utc(record["c4"])
            if role in ("user", "assistant"):
                row = self._context_row(
                    artifact, database_state, session_id, record["id"], timestamp, role, text
                )
            else:
                label = "%s: %s" % (role or "message", text)
                row = self._context_row(
                    artifact, database_state, session_id, record["id"], timestamp, "system", label
                )
            row.recovered = "yes"
            yield row

    def _database_deliveries(
        self,
        artifact: Artifact,
        connection,
        database_state: _DatabaseState,
        current: dict[str, str],
    ) -> Iterator[Row]:
        """Yield one `channel delivery:` row per send or turn to a platform conversation.

        A `channel delivery reply:` row follows for the platform message that
        replied to it; the session is the current generation of
        `source_session_key`.
        """
        for row in connection.execute(
            "select d.rowid as _rowid, d.*, c.channel from conversation_deliveries d"
            " join conversations c using (conversation_id) order by d.created_at, d.rowid"
        ):
            session_id = current.get(str(row["source_session_key"] or ""), "")
            yield self._context_row(
                artifact,
                database_state,
                session_id,
                row["_rowid"],
                to_utc(row["created_at"]),
                "system",
                "channel delivery: %s %s channel %s conversation %s platform message %s%s"
                " (from openclaw-agent.sqlite conversation_deliveries, created_at)"
                % (
                    row["operation_kind"] or "-",
                    row["status"] or "-",
                    row["channel"] or "-",
                    row["conversation_id"] or "-",
                    row["platform_message_id"] or "-",
                    " error=%s" % row["rejection_error"] if row["rejection_error"] else "",
                ),
            )
            if row["reply_text"] is not None:
                yield self._context_row(
                    artifact,
                    database_state,
                    session_id,
                    row["_rowid"],
                    to_utc(row["reply_timestamp"]),
                    "system",
                    "channel delivery reply: %s (reply message %s; from openclaw-agent.sqlite"
                    " conversation_deliveries, reply_timestamp)"
                    % (row["reply_text"], row["reply_message_id"] or "-"),
                )

    def _database_tombstones(
        self, artifact: Artifact, connection, database_state: _DatabaseState
    ) -> Iterator[Row]:
        """Yield one row per session `openclaw memory forget` removed from the memory index."""
        for row in connection.execute(
            "select rowid as _rowid, session_id, agent_id, reason, created_at"
            " from memory_session_tombstones order by created_at, rowid"
        ):
            session_id = str(row["session_id"] or "")
            yield self._context_row(
                artifact,
                database_state,
                session_id,
                row["_rowid"],
                to_utc(row["created_at"]),
                "system",
                "session forgotten from memory: %s agent %s reason=%s (from"
                " openclaw-agent.sqlite memory_session_tombstones, created_at)"
                % (session_id or "-", row["agent_id"] or "-", row["reason"] or "-"),
            )

    # -- state/openclaw.sqlite ----------------------------------------------------------
    def _parse_state_database(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the rows of `state/openclaw.sqlite`.

        `backup uploaded:` from `backup_runs`, the permission rows of
        `operator_approvals`, `exec_approvals_config`,
        `operator_approval_standing_grants` and `channel_pairing_allow_entries`,
        `channel ingress:` from `channel_ingress_events`, `source host:` from
        the GitHub publication tables, and `git remote:` from `projects`.
        """
        problem = sqlite_problem(artifact.disk_path)
        if problem:
            if problem == NOT_SQLITE:
                problem = "state/openclaw.sqlite is not a SQLite database"
            yield from error_rows(self, artifact, [(0, problem)])
            return
        data: dict[str, list[dict]] = {}
        try:
            with open_copy(artifact.disk_path) as connection:
                tables = table_names(connection)
                if "backup_runs" not in tables and "openclaw_engine_anchor" in tables:
                    yield from error_rows(self, artifact, [self._anchor_error(connection, "state")])
                    return
                for table, query in STATE_QUERIES:
                    if table in tables:
                        data[table] = [dict(record) for record in connection.execute(query)]
        except sqlite3.DatabaseError as error:
            yield from error_rows(
                self, artifact, [(0, "state/openclaw.sqlite unreadable (%s)" % error, "")]
            )
            return
        errors: list = []
        yield from self._backup_rows(artifact, data.get("backup_runs", []), errors)
        for record in data.get("operator_approvals", []):
            yield self._approval_row(artifact, record)
        for record in data.get("exec_approvals_config", []):
            yield from self._exec_config_rows(artifact, record, errors)
        for record in data.get("operator_approval_standing_grants", []):
            yield self._state_row(
                artifact, record["_rowid"], to_utc(record["created_at_ms"]), _grant_text(record)
            )
        for record in data.get("channel_pairing_allow_entries", []):
            yield self._state_row(
                artifact,
                record["_rowid"],
                to_utc(record["updated_at"]),
                "permission setting: channel %s account %s allowFrom=%s (from"
                " state/openclaw.sqlite channel_pairing_allow_entries, updated_at)"
                % (record["channel_key"] or "-", record["account_id"] or "-", record["entry"]),
            )
        for record in data.get("channel_ingress_events", []):
            yield self._state_row(
                artifact,
                record["_rowid"],
                to_utc(record["received_at"]),
                "channel ingress: %s account %s status %s: %s (from state/openclaw.sqlite"
                " channel_ingress_events, received_at)"
                % (
                    record["channel_id"] or "-",
                    record["account_id"] or "-",
                    record["status"] or "-",
                    record["payload_json"],
                ),
            )
        for table in GITHUB_PUBLICATION_TABLES:
            for record in data.get(table, []):
                yield self._state_row(
                    artifact,
                    record["_rowid"],
                    to_utc(record["created_at_ms"]),
                    _publication_text(table, record),
                    str(record["session_id"] or ""),
                )
        for record in data.get("projects", []):
            origin = compact(record["origin_url"])
            if origin:
                row = self._state_row(
                    artifact,
                    record["_rowid"],
                    to_utc(record["created_at_ms"]),
                    GIT_REMOTE_PREFIX + origin,
                )
                row.project_path = str(record["repo_root"] or "")
                yield row
        yield from error_rows(self, artifact, errors)

    def _state_row(
        self, artifact: Artifact, line: int, timestamp: str, text: str, session_id: str = ""
    ) -> Row:
        row = self.base_row(artifact)
        row.source_line = line
        row.timestamp_utc = timestamp
        row.turn_type = "system"
        row.session_id = session_id
        row.text = compact(text)
        return row

    def _backup_rows(self, artifact: Artifact, runs: list[dict], errors: list) -> Iterator[Row]:
        for run in runs:
            raw = run["manifest_json"] if isinstance(run.get("manifest_json"), str) else ""
            try:
                manifest = json.loads(raw)
            except ValueError as error:
                errors.append(
                    (run["_rowid"], "backup_runs manifest_json unparseable (%s)" % error, raw)
                )
                continue
            text = self._backup_text(run, manifest if isinstance(manifest, dict) else {})
            if text:
                yield self._state_row(artifact, run["_rowid"], to_utc(run.get("created_at")), text)

    def _approval_row(self, artifact: Artifact, record: dict) -> Row:
        """Return the `permission decision:` row of one `operator_approvals` row.

        The row is at `resolved_at_ms` (`created_at_ms` while pending), joined
        to the call it governed on `source_session_id` and
        `source_tool_call_id`.
        """
        field = "resolved_at_ms" if record.get("resolved_at_ms") is not None else "created_at_ms"
        row = self._state_row(
            artifact,
            record["_rowid"],
            to_utc(record.get(field)),
            "permission decision: %s status=%s kind=%s resolver=%s reason=%s approval %s"
            " (from state/openclaw.sqlite operator_approvals, %s)"
            % (
                record.get("decision") or record.get("status") or "-",
                record.get("status") or "-",
                record.get("kind") or "-",
                record.get("resolver_kind") or "-",
                record.get("terminal_reason") or "-",
                record.get("approval_id") or "-",
                field,
            ),
            str(record.get("source_session_id") or ""),
        )
        row.tool_name = str(record.get("source_tool_name") or "")
        row.tool_use_id = str(record.get("source_tool_call_id") or "")
        return row

    def _exec_config_rows(self, artifact: Artifact, row: dict, errors: list) -> Iterator[Row]:
        """Yield one `permission setting:` row per policy setting of the migrated exec approvals.

        `raw_json` is the migrated `exec-approvals.json`; each default and
        per-agent policy field, allowlist entry and MCP tool grant gives a
        row. `socket` holds the approvals socket's token and is never read; a
        `raw_json` that does not decode is reported without its text.
        """
        try:
            doc = json.loads(row["raw_json"] if isinstance(row.get("raw_json"), str) else "")
        except ValueError as error:
            errors.append(
                (
                    row["_rowid"],
                    "exec_approvals_config raw_json unparseable (%s; text withheld,"
                    " it holds the approvals socket token)" % error,
                    "",
                )
            )
            return
        doc = doc if isinstance(doc, dict) else {}
        timestamp = to_utc(row.get("updated_at_ms"))
        scopes: list[tuple[str, object]] = [("defaults", doc.get("defaults"))]
        agents = doc.get("agents")
        if isinstance(agents, dict):
            scopes += [
                ("agents.%s" % agent_id, agent_policy) for agent_id, agent_policy in agents.items()
            ]
        for scope, policy in scopes:
            for setting in _policy_settings(policy):
                yield self._state_row(
                    artifact,
                    row["_rowid"],
                    timestamp,
                    "permission setting: %s.%s (from state/openclaw.sqlite"
                    " exec_approvals_config, updated_at_ms)" % (scope, setting),
                )

    @staticmethod
    def _backup_text(run: dict, manifest: dict) -> str:
        """Return the `backup uploaded:` text of a successful upload or Git backup, else "".

        A failed run, a local archive or a Git push that failed gives "".
        """
        if run.get("status") != "ok":
            return ""
        location = manifest.get("location")
        location = location if isinstance(location, dict) else {}
        target = manifest.get("target")
        pushed = manifest.get("kind") == "git" and target and manifest.get("pushFailed") is not True
        if not location and not pushed:
            return ""
        return (
            "backup uploaded: %s to %s key %s namespace %s"
            " (from state/openclaw.sqlite backup_runs, created_at)"
            % (
                manifest.get("kind") or "-",
                location.get("name") or run.get("archive_path") or "-",
                location.get("key") or target or "-",
                location.get("namespace") or manifest.get("namespace") or "-",
            )
        )

    @staticmethod
    def _window_session(
        session_id: str,
        windows: dict[str, dict],
        database_state: _DatabaseState,
        key: str = "",
        extra: str = "",
    ) -> _Session:
        window = windows.get(session_id) or {}
        parts: list[str] = []
        for column in (
            "channel",
            "account_id",
            "chat_type",
            "reason",
            "previous_session_id",
            "parent_session_key",
            "spawned_by",
            "display_name",
        ):
            if window.get(column) not in (None, ""):
                parts.append("%s=%s" % (column, window[column]))
        if extra:
            parts.append(extra)
        if window.get("model_provider"):
            parts.append("model_provider=%s" % window["model_provider"])
        session = _Session(
            session_id,
            key or str(window.get("session_key") or ""),
            " ".join(parts),
            str(window.get("model") or ""),
            str(window.get("model_provider") or ""),
        )
        session.database_state = database_state
        return session

    def _database_events(
        self,
        artifact: Artifact,
        connection,
        windows: dict[str, dict],
        database_state: _DatabaseState,
        opts: Options,
    ) -> Iterator[Row]:
        session: _Session | None = None
        for row in connection.execute(
            "select rowid as _rowid, session_id, seq, event_json, event_zstd, created_at "
            "from transcript_events order by session_id, seq"
        ):
            row = dict(row)
            session_id = str(row.get("session_id") or "")
            if session is None or session.session_id != session_id:
                session = self._window_session(session_id, windows, database_state)
            line = row["_rowid"]
            raw = row.get("event_json")
            if raw is None and row.get("event_zstd") is not None:
                data, reason = _maybe_zstd(row["event_zstd"])
                if data is None:
                    if not session.started:
                        yield from self._start(
                            artifact, session, line, to_utc(row.get("created_at")), None, opts
                        )
                    yield self._bad(
                        artifact,
                        session,
                        line,
                        "event seq %s could not be decompressed (%s)" % (row.get("seq"), reason),
                        opts,
                        to_utc(row.get("created_at")),
                    )
                    continue
                raw = data.decode("utf-8", errors="replace")
            reason = None  # set only when the JSON does not parse
            try:
                record = json.loads(
                    raw
                    if isinstance(raw, str)
                    else bytes(raw or b"").decode("utf-8", errors="replace")
                )
            except ValueError as error:
                record = None
                reason = str(error)
            if not isinstance(record, dict):
                if not session.started:
                    yield from self._start(
                        artifact, session, line, to_utc(row.get("created_at")), None, opts
                    )
                yield self._bad(
                    artifact,
                    session,
                    line,
                    "event seq %s unreadable: %s"
                    % (row.get("seq"), reason if reason is not None else "not a JSON object"),
                    opts,
                    to_utc(row.get("created_at")),
                )
                continue
            yield from self._entry_rows(
                artifact, session, line, record, row.get("created_at"), opts
            )

    def _database_archives(
        self,
        artifact: Artifact,
        connection,
        windows: dict[str, dict],
        database_state: _DatabaseState,
        opts: Options,
    ) -> Iterator[Row]:
        for row in connection.execute(
            "select rowid as _rowid, session_id, session_key, reason, encoding, archive_name, "
            "created_at, archive_blob from session_transcript_archives "
            "order by created_at"
        ):
            row = dict(row)
            session_id = str(row.get("session_id") or "")
            extra = "archive=%s name=%s" % (row.get("reason") or "", row.get("archive_name") or "")
            session = self._window_session(
                session_id, windows, database_state, str(row.get("session_key") or ""), extra
            )
            line = row["_rowid"]
            data, error = _maybe_zstd(row.get("archive_blob") or b"")
            if data is None:
                yield self._bad(
                    artifact,
                    session,
                    line,
                    "archive %s could not be decompressed (%s)"
                    % (row.get("archive_name") or session_id, error),
                    opts,
                    to_utc(row.get("created_at")),
                )
            else:
                errors: list = []
                records = _iter_lines(data, errors)
                yield from self._records(artifact, session, records, opts, fixed_line=line)
                label = "archive %s" % (row.get("archive_name") or session_id)
                yield from self._blob_errors(artifact, session, line, label, errors)
            if row.get("reason") == "deleted":
                yield self._row(
                    artifact,
                    session,
                    line,
                    to_utc(row.get("created_at")),
                    "system",
                    "session deleted: %s archive %s (from openclaw-agent.sqlite"
                    " session_transcript_archives reason deleted, created_at)"
                    % (session_id or "-", row.get("archive_name") or "-"),
                    opts,
                )

    def _database_cold(
        self,
        artifact: Artifact,
        connection,
        windows: dict[str, dict],
        database_state: _DatabaseState,
        opts: Options,
        chunked: bool = False,
    ) -> Iterator[Row]:
        """Yield the rows of each `sqlite`-stored cold archive, chunk by chunk when chunked.

        A chunked archive keeps its bytes in `session_transcript_cold_archive_chunks`, a table
        without row ids, so its rows carry the parent archive's rowid as `source_line`. A
        `sqlite`-stored archive or chunk whose blob is NULL has lost its bytes, so it is one
        `parser:` row.
        """
        for row in connection.execute(
            "select rowid as _rowid, session_id, archive_name, archived_at, archive_blob "
            "from session_transcript_cold_archives where storage = 'sqlite' order by archived_at"
        ):
            row = dict(row)
            session_id = str(row.get("session_id") or "")
            session = self._window_session(
                session_id,
                windows,
                database_state,
                extra="cold_archive=%s" % (row.get("archive_name") or ""),
            )
            line = row["_rowid"]
            blobs = [(row.get("archive_name") or session_id, row.get("archive_blob"))]
            if chunked:
                chunks = [
                    (chunk["archive_name"] or session_id, chunk["archive_blob"])
                    for chunk in connection.execute(
                        "select archive_name, archive_blob from"
                        " session_transcript_cold_archive_chunks where session_id = ?"
                        " order by chunk_index",
                        (session_id,),
                    )
                ]
                blobs = chunks or blobs
            for name, blob in blobs:
                if blob is None:
                    yield self._bad(
                        artifact,
                        session,
                        line,
                        "cold archive %s has no stored bytes" % name,
                        opts,
                        to_utc(row.get("archived_at")),
                    )
                    continue
                data, error = _maybe_zstd(blob)
                if data is None:
                    yield self._bad(
                        artifact,
                        session,
                        line,
                        "cold archive %s could not be decompressed (%s)" % (name, error),
                        opts,
                        to_utc(row.get("archived_at")),
                    )
                    continue
                errors: list = []
                yield from self._records(
                    artifact, session, _iter_lines(data, errors), opts, fixed_line=line
                )
                yield from self._blob_errors(
                    artifact, session, line, "cold archive " + name, errors
                )


__all__ = ["OpenClawParser", "tool_args_text"]
