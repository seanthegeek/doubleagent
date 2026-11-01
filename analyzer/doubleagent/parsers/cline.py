"""Cline transcripts, legacy task layout and SDK sessions.

Reads the legacy task layout under the VS Code extension's
`<editor>/User/globalStorage/saoudrizwan.claude-dev/` or `~/.cline/data/` through the shared
mapping in `cline_legacy.py`, with `state/taskHistory.json` (`id`, `ts`, `task`,
`cwdOnTaskInitialization`, `modelId`, `apiProvider` and the token counts) and the
extension's `cache/sync-queue.json` (`items.<key>` `taskId`, `key`, `timestamp`,
`status`, `retryCount`; `data` and `lastError` are never read). Under `~/.cline/data/` it
reads the SDK sessions: `sessions/<id>/<id>.json` manifests (`session_id`, `source`,
`pid`, `started_at`, `ended_at`, `exit_code`, `status`, `interactive`, `provider`,
`model`, `cwd`, `workspace_root`, `prompt`, `metadata.title`,
`metadata.enterprise.blobUpload`, `metadata.importedFrom`) and `*.messages.json`
(`messages[]` `role`, `content` blocks, `modelInfo.id` and `.provider`, `ts`;
`updated_at`); `db/sessions.db` table `sessions` for sessions whose manifest is gone; and
the secondary sources `db/session-search.db` (`indexed_sessions`, the FTS5 table
`session_search`), `cache/user_input_history.jsonl` (`prompt`, `ts`) and
`logs/hooks.jsonl` (`hookName`, `ts`, `taskId`, `sessionContext.rootSessionId`,
`workspaceRoots`, `userPromptSubmit`, `tool_call`, `tool_result`, `postToolUse`, `turn`
and the `HOOK_KEYS`). It also reads the standing settings of `globalState.json`
(`autoApprovalSettings`, `mode`, `cloudTaskTarget`, `cloudSessionStatuses`),
`settings/global-settings.json`, `settings/cline_mcp_settings.json` (only `autoApprove`
and `disabled` per server) and `cloud-pending-starts/*.json`.

It yields user, assistant, thinking, tool_use, tool_result and system rows, with model,
permission decision and setting, sync, origin and session index rows. The API history is
a fallback, not a secondary source: it words a turn differently from `ui_messages.json`.

Validated against source (cline/cline at 39ff2359 and 41e75efc) with a synthetic fixture;
no real install. See `research/cline.md`.
"""

from __future__ import annotations

import json
import re
import sqlite3
from collections.abc import Iterator
from dataclasses import replace
from pathlib import Path

from ..inputs import Artifact
from ..model import Row, compact
from ..sqlite_util import NOT_SQLITE, open_copy, sqlite_problem, table_names
from ..timeutil import to_utc
from .base import (
    Options,
    compact_json,
    error_rows,
    file_mtime,
    iter_jsonl,
    setting_value,
    text_of,
    track_change,
)
from .claude_code import tool_summary
from .cline_legacy import (
    LegacyTaskParser,
    content_rows,
    iter_array_at,
    iter_json_array,
    load_json,
    read_text,
)

ROOT = r"(?:(?:.*/)?User/globalStorage/saoudrizwan\.claude-dev|\.cline/data)"
STATE_PATTERN = re.compile(r"^%s/state/taskHistory\.json$" % ROOT, re.I)
MANIFEST_PATTERN = re.compile(r"^\.cline/data/sessions/([^/]+)/\1\.json$")
MESSAGES_PATTERN = re.compile(r"^\.cline/data/sessions/[^/]+/[^/]+\.messages\.json$")
SESSIONS_DATABASE_REL = ".cline/data/db/sessions.db"
SYNC_QUEUE_PATTERN = re.compile(
    r"^(?:.*/)?User/globalStorage/saoudrizwan\.claude-dev/cache/sync-queue\.json$", re.I
)
SEARCH_DATABASE_REL = ".cline/data/db/session-search.db"
INPUT_HISTORY_REL = ".cline/data/cache/user_input_history.jsonl"
SEARCH_COLUMNS = (
    "session_id, document_id, ordinal, role, started_at, workspace_root, title, content"
)
BLOB_UPLOAD_KEY = "enterprise.blobUpload"
SEARCH_KEYS = ("title", "workspace_root", "prompt", "document_count", "indexed_at")
MESSAGES_KEY_PATTERN = re.compile(r'"messages"[ \t\n\r]*:[ \t\n\r]*(?=\[)')
HOOKS_REL = ".cline/data/logs/hooks.jsonl"
# A tool_call or tool_result line is written when the call starts or ends,
# the transcript's message when the whole turn is persisted, which can be a
# long command later; the call id pairs them, so the wide window is safe.
HOOKS_TOLERANCE_SECONDS = 3600.0
GLOBAL_STATE_REL = ".cline/data/globalState.json"
GLOBAL_SETTINGS_REL = ".cline/data/settings/global-settings.json"
MCP_SETTINGS_PATTERN = re.compile(r"^%s/settings/cline_mcp_settings\.json$" % ROOT, re.I)
PENDING_START_PATTERN = re.compile(r"^\.cline/data/cloud-pending-starts/\d+-[0-9a-f-]+\.json$")
# The runtime appends this to every rejected approval's reason
# (sdk/packages/shared/src/llms/tools.ts:104-105, agent-runtime.ts:2490).
REJECTION_SUFFIX = " -- NOT a tool or system failure. Clarify with user before proceeding."
# The two fixed reasons of a call the runtime skips without asking
# (sdk/packages/agents/src/agent-runtime.ts:2469, 2481).
BLOCKED_PATTERN = re.compile(
    r'^(?:Tool "[^"\n]*" is disabled by policy|Tool [^\n]* was blocked by a runtime hook)$'
)
# Hook audit keys named on a `hook audit:` row, with the name the row uses.
HOOK_KEYS = (
    ("clineVersion", "cline_version"),
    ("userId", "user"),
    ("agent_id", "agent"),
    ("parent_agent_id", "parent_agent"),
    ("reason", "reason"),
    ("pid", "pid"),
    ("source", "source"),
)
# metadata.importedFrom keys named on the session start row.
IMPORTED_KEYS = (
    ("tool", "imported_from"),
    ("sourceSessionId", "imported_session"),
    ("sourcePath", "imported_path"),
    ("importedAt", "imported_at"),
    ("sourceProvider", "imported_provider"),
    ("sourceModel", "imported_model"),
)


class ClineParser(LegacyTaskParser):
    """Parse Cline's legacy tasks, SDK sessions and the stores beside them."""

    agent = "cline"
    # `enterprise.blobUpload` in a session's metadata marks an uploaded transcript.
    records_sync = True
    # Cline Cloud sessions listed in globalState.json or a pending-start file.
    records_origin = True
    name = "cline"
    has_secondary = True
    ROOT = ROOT
    PROJECT_KEY = "cwdOnTaskInitialization"

    def wants(self, artifact: Artifact) -> bool:
        """Return True for a Cline task, session, database, history, log or settings file."""
        rel = artifact.rel
        return bool(
            self.task_match(artifact)
            or STATE_PATTERN.match(rel)
            or MANIFEST_PATTERN.match(rel)
            or MESSAGES_PATTERN.match(rel)
            or SYNC_QUEUE_PATTERN.match(rel)
            or MCP_SETTINGS_PATTERN.match(rel)
            or PENDING_START_PATTERN.match(rel)
        ) or rel in (
            SESSIONS_DATABASE_REL,
            SEARCH_DATABASE_REL,
            INPUT_HISTORY_REL,
            HOOKS_REL,
            GLOBAL_STATE_REL,
            GLOBAL_SETTINGS_REL,
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for the search index, the input history and the hook log."""
        return artifact.rel in (SEARCH_DATABASE_REL, INPUT_HISTORY_REL, HOOKS_REL)

    def secondary_tolerance(self, artifact: Artifact) -> float:
        """Return an hour for `logs/hooks.jsonl`, else the default window.

        A hook line is stamped when a call starts or ends, the transcript's message when
        the turn is persisted (`HOOKS_TOLERANCE_SECONDS`).
        """
        if artifact.rel == HOOKS_REL:
            return HOOKS_TOLERANCE_SECONDS
        return super().secondary_tolerance(artifact)

    def history_item(self, task_dir: Path, root: Path, task_id: str) -> dict | None:
        """Return the task's entry in `state/taskHistory.json`, or None."""
        # A pre-scan: the same parser reports taskHistory.json when it parses the file.
        items = self.cached_json(root / "state" / "taskHistory.json", [])
        if isinstance(items, list):
            for item in items:
                if isinstance(item, dict) and str(item.get("id")) == task_id:
                    return item
        return None

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Cline file."""
        rel = artifact.rel
        if self.task_match(artifact):
            yield from self.parse_task_file(artifact, opts)
        elif STATE_PATTERN.match(rel):
            yield from self._parse_task_history(artifact, opts)
        elif MANIFEST_PATTERN.match(rel):
            yield from self._parse_manifest(artifact, opts)
        elif MESSAGES_PATTERN.match(rel):
            yield from self._parse_messages(artifact, opts)
        elif rel == SESSIONS_DATABASE_REL:
            yield from self._parse_sessions_database(artifact, opts)
        elif rel == SEARCH_DATABASE_REL:
            yield from self._parse_search_database(artifact)
        elif rel == INPUT_HISTORY_REL:
            yield from self._parse_input_history(artifact)
        elif SYNC_QUEUE_PATTERN.match(rel):
            yield from self._parse_sync_queue(artifact)
        elif rel == HOOKS_REL:
            yield from self._parse_hooks_log(artifact)
        elif rel == GLOBAL_STATE_REL:
            yield from self._parse_global_state(artifact)
        elif rel == GLOBAL_SETTINGS_REL:
            yield from self._parse_global_settings(artifact)
        elif MCP_SETTINGS_PATTERN.match(rel):
            yield from self._parse_mcp_settings(artifact)
        elif PENDING_START_PATTERN.match(rel):
            yield from self._parse_pending_starts(artifact)

    # -- cache/sync-queue.json (VS Code extension)

    def _parse_sync_queue(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `enterprise prompt upload <status>` row per queued task file.

        The row is prefixed `session synced: ` only when the item's status is `synced`;
        `data` and `lastError` are never read into a row.
        """
        errors: list = []
        doc = load_json(artifact.disk_path, errors)
        items = doc.get("items") if isinstance(doc, dict) else None
        if not isinstance(items, dict):
            message = "sync-queue.json is not a JSON object with an items object"
            yield from error_rows(self, artifact, errors or [(0, message, "")])
            return
        root = artifact.disk_path.parent.parent
        for position, (key, item) in enumerate(items.items(), 1):
            if not isinstance(item, dict):
                errors.append((position, "sync-queue item %s is not a JSON object" % key, ""))
                continue

            def value(k: str, item: dict = item) -> str:
                found = item.get(k)
                return str(found) if found not in (None, "") else "-"

            task_id = str(item.get("taskId") or "")
            history = self.history_item(root, root, task_id) if task_id else None
            row = self.base_row(artifact)
            row.source_line = position
            row.turn_type = "system"
            row.session_id = task_id
            row.project_path = str((history or {}).get(self.PROJECT_KEY) or "")
            row.timestamp_utc = to_utc(item.get("timestamp"))
            row.text = (
                "enterprise prompt upload %s key %s retries %s"
                " (from saoudrizwan.claude-dev/cache/sync-queue.json, item timestamp)"
                % (value("status"), value("key"), value("retryCount"))
            )
            if item.get("status") == "synced":
                row.text = "session synced: " + row.text
            yield row
        yield from error_rows(self, artifact, errors)

    def _parse_task_history(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        for position, item in iter_json_array(artifact.disk_path, errors):
            if isinstance(item, dict):
                yield self.history_row(artifact, item, position, opts)
        yield from error_rows(self, artifact, errors)

    # -- SDK sessions

    def _session_start(
        self,
        session: dict,
        artifact: Artifact,
        position: int,
        opts: Options,
        time_key: str,
        label: str,
        keys,
    ) -> Row:
        row = self.base_row(artifact)
        row.source_line = position
        row.timestamp_utc = to_utc(session.get(time_key))
        row.session_id = str(session.get("session_id") or "")
        row.project_path = str(session.get("cwd") or session.get("workspace_root") or "")
        row.model = str(session.get("model") or "")
        row.turn_type = "system"
        detail = " ".join(
            "%s=%s" % (k, session[k]) for k in keys if session.get(k) not in (None, "")
        )
        row.text = compact("%s: %s" % (label, detail))
        return row

    def _model_start(self, start: Row, session: dict) -> Iterator[Row]:
        """Yield `model: <model> provider=<provider>` with a start row's time and session."""
        model = str(session.get("model") or "")
        if not model:
            return
        provider = str(session.get("provider") or "")
        yield replace(
            start,
            model=model,
            text=compact("model: %s%s" % (model, " provider=%s" % provider if provider else "")),
        )

    def _blob_upload(self, start: Row, meta, source: str) -> Iterator[Row]:
        """Yield the `session synced:` row of an `enterprise.blobUpload` metadata key.

        The row has the start row's time, session and project; nothing is yielded when
        `meta` has no such object.
        """
        blob = meta.get(BLOB_UPLOAD_KEY) if isinstance(meta, dict) else None
        if not isinstance(blob, dict):
            return
        storage = blob.get("storage")
        storage = storage if isinstance(storage, dict) else {}

        def value(key: str, default: str = "-") -> str:
            found = storage.get(key)
            return str(found) if found not in (None, "") else default

        user = blob.get("userDistinctId")
        text = (
            "session synced: enterprise prompt upload to %s bucket %s object"
            " sessions/%s/%s/messages.json endpoint=%s region=%s account=%s"
            " (from %s %s, started_at)"
            % (
                value("adapterType"),
                value("bucket"),
                user if user not in (None, "") else "unknown",
                start.session_id,
                value("endpoint"),
                value("region"),
                value("accountId"),
                source,
                BLOB_UPLOAD_KEY,
            )
        )
        yield replace(start, text=text)

    START_KEYS = (
        "source",
        "provider",
        "status",
        "interactive",
        "pid",
        "parent_session_id",
        "is_subagent",
        "title",
        "prompt",
        *(name for _, name in IMPORTED_KEYS),
    )
    END_KEYS = ("status", "exit_code")

    def _parse_manifest(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        manifest = load_json(artifact.disk_path, errors)
        if not isinstance(manifest, dict):
            yield from error_rows(
                self,
                artifact,
                errors or [(0, "session manifest is not a JSON object", "")],
                artifact.disk_path.parent.name,
            )
            return
        manifest = dict(manifest)
        manifest.setdefault("session_id", artifact.disk_path.parent.name)
        meta = manifest.get("metadata")
        if isinstance(meta, dict) and meta.get("title"):
            manifest["title"] = meta["title"]
        _imported(manifest, meta)
        start = self._session_start(
            manifest, artifact, 0, opts, "started_at", "session start", self.START_KEYS
        )
        yield start
        yield from self._model_start(start, manifest)
        yield from self._blob_upload(start, meta, artifact.disk_path.name + " metadata")
        if manifest.get("ended_at"):
            yield self._session_start(
                manifest, artifact, 0, opts, "ended_at", "session end", self.END_KEYS
            )

    def _parse_messages(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        session_id = artifact.disk_path.parent.name
        # A pre-scan: the same parser reports the manifest when it parses the file.
        manifest = load_json(artifact.disk_path.parent / (session_id + ".json"), [])
        manifest = manifest if isinstance(manifest, dict) else {}
        project = str(manifest.get("cwd") or manifest.get("workspace_root") or "")
        errors: list = []
        raw_text = read_text(artifact.disk_path, errors)
        payload = None
        if raw_text is not None:
            try:
                payload = json.loads(raw_text)
            except ValueError:
                payload = None
        if isinstance(payload, dict):
            messages = enumerate(payload.get("messages") or [], 1)
            fallback = to_utc(manifest.get("started_at")) or to_utc(payload.get("updated_at"))
        elif raw_text is not None:
            # Cut mid-write: walk the messages array as far as it parses.
            match = MESSAGES_KEY_PATTERN.search(raw_text)
            if match:
                messages = iter_array_at(raw_text, match.end(), errors)
            else:
                errors.append((0, "messages file is not a JSON object", ""))
                messages = iter(())
            fallback = to_utc(manifest.get("started_at"))
        else:
            messages = iter(())
            fallback = ""
        # The lead transcript continues from the manifest's model, whose start
        # row the manifest yields; a subagent transcript tracks its own.
        seen: dict = {}
        if artifact.disk_path.name == session_id + ".messages.json" and manifest.get("model"):
            seen[session_id] = str(manifest["model"])
        for position, message in messages:
            if not isinstance(message, dict):
                continue
            timestamp = to_utc(message.get("ts")) or fallback
            fallback = timestamp
            info = message.get("modelInfo")
            info = info if isinstance(info, dict) else {}
            model = str(info.get("id") or manifest.get("model") or "")
            change = track_change("model", seen, session_id, str(info.get("id") or ""))
            if change:
                row = self.base_row(artifact)
                row.source_line = position
                row.timestamp_utc = timestamp
                row.session_id = session_id
                row.project_path = project
                row.model = model
                row.turn_type = "system"
                provider = str(info.get("provider") or "")
                row.text = compact(change + (" provider=%s" % provider if provider else ""))
                yield row
            role = message.get("role")
            base_turn = (
                "user" if role == "user" else "assistant" if role == "assistant" else "system"
            )
            decisions = _decisions(message.get("content"))
            for turn, text, name, tool_id in content_rows(
                message.get("content"), base_turn, opts.include_thinking
            ):
                row = self.base_row(artifact)
                row.source_line = position
                row.timestamp_utc = timestamp
                row.session_id = session_id
                row.project_path = project
                row.model = model
                row.turn_type = turn
                row.tool_name = name
                row.tool_use_id = tool_id
                row.text = compact(text)
                yield row
                if turn == "tool_result" and tool_id in decisions:
                    yield replace(row, turn_type="system", text=decisions.pop(tool_id))
        yield from error_rows(self, artifact, errors, session_id, project)

    def _parse_sessions_database(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        problem = sqlite_problem(artifact.disk_path)
        if problem:
            if problem == NOT_SQLITE:
                problem = "sessions.db is not a SQLite database"
            yield from error_rows(self, artifact, [(0, problem, "")])
            return
        sessions_dir = artifact.disk_path.parent.parent / "sessions"
        try:
            with open_copy(artifact.disk_path) as connection:
                if "sessions" not in table_names(connection):
                    return
                rows = [
                    dict(database_row)
                    for database_row in connection.execute(
                        "select rowid as _rowid, * from sessions"
                    )
                ]
        except Exception as error:  # noqa: BLE001 - a damaged database must not stop the timeline
            yield from error_rows(self, artifact, [(0, "sessions.db unreadable (%s)" % error, "")])
            return
        for session in rows:
            session_id = str(session.get("session_id") or "")
            if session_id and (sessions_dir / session_id / (session_id + ".json")).is_file():
                continue
            meta = (
                load_json_text(session["metadata_json"]) if session.get("metadata_json") else None
            )
            if isinstance(meta, dict) and meta.get("title"):
                session["title"] = meta["title"]
            _imported(session, meta)
            start = self._session_start(
                session,
                artifact,
                int(session.get("_rowid") or 0),
                opts,
                "started_at",
                "session index",
                (*self.START_KEYS, "ended_at", "exit_code"),
            )
            yield start
            yield from self._model_start(start, session)
            yield from self._blob_upload(start, meta, "sessions.db sessions.metadata_json")

    # -- logs/hooks.jsonl (secondary)

    def _parse_hooks_log(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one row per hook audit line.

        `prompt_submit` is a `user` row, `tool_call` a `tool_use` row, `tool_result` a
        `tool_result` row (both with the call id), `agent_end` an `assistant` row with the
        final reply, and every other line a `hook audit: <hookName> ...` row.
        """
        errors: list = []
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            session_context = _object(record.get("sessionContext"))
            roots = record.get("workspaceRoots")
            row = self.base_row(artifact)
            row.source_line = line_number
            row.timestamp_utc = to_utc(record.get("ts") or record.get("timestamp"))
            row.session_id = str(
                session_context.get("rootSessionId")
                or record.get("taskId")
                or record.get("sessionId")
                or ""
            )
            if isinstance(roots, list) and roots and isinstance(roots[0], str):
                row.project_path = roots[0]
            hook = str(record.get("hookName") or "")
            if not _hook_event(row, hook, record):
                detail = "".join(
                    " %s=%s" % (name, setting_value(record[key]))
                    for key, name in HOOK_KEYS
                    if record.get(key) not in (None, "")
                )
                error = _object(record.get("error"))
                if error:
                    detail += " error=%s: %s" % (
                        _dash(error.get("name")),
                        _dash(error.get("message")),
                    )
                row.turn_type = "system"
                row.text = compact("hook audit: %s%s" % (hook or "-", detail))
            yield row
        yield from error_rows(self, artifact, errors)

    # -- standing settings and Cline Cloud state

    def _settings_row(self, artifact: Artifact, timestamp: str, key: str, value) -> Row:
        row = self.base_row(artifact)
        row.source_line = 1
        row.turn_type = "system"
        row.timestamp_utc = timestamp
        row.text = compact(
            "permission setting: %s=%s (from %s, file mtime)"
            % (key, setting_value(value), artifact.rel)
        )
        return row

    def _json_object(self, artifact: Artifact, what: str) -> tuple[dict | None, list]:
        """Return the file as a JSON object, or None with the whole-file error."""
        errors: list = []
        doc = load_json(artifact.disk_path, errors)
        if isinstance(doc, dict):
            return doc, []
        return None, errors or [(0, "%s is not a JSON object" % what, "")]

    def _parse_global_state(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the standing setting and Cline Cloud rows of `globalState.json`.

        `autoApprovalSettings` and `mode` are `permission setting:` rows, `cloudTaskTarget`
        one `cloud task target:` row, and each `cloudSessionStatuses` entry a `session
        origin:` and a `session synced:` row. No other key is read.
        """
        doc, errors = self._json_object(artifact, "globalState.json")
        if doc is None:
            yield from error_rows(self, artifact, errors)
            return
        timestamp = file_mtime(artifact)
        auto = _object(doc.get("autoApprovalSettings"))
        for key in ("enabled", "maxRequests"):
            if key in auto:
                yield self._settings_row(
                    artifact, timestamp, "autoApprovalSettings." + key, auto[key]
                )
        favorites = auto.get("favorites")
        for entry in favorites if isinstance(favorites, list) else []:
            yield self._settings_row(artifact, timestamp, "autoApprovalSettings.favorites", entry)
        for key, value in _object(auto.get("actions")).items():
            yield self._settings_row(
                artifact, timestamp, "autoApprovalSettings.actions." + key, value
            )
        if doc.get("mode") not in (None, ""):
            yield self._settings_row(artifact, timestamp, "mode", doc["mode"])
        target = doc.get("cloudTaskTarget")
        if isinstance(target, dict):
            row = self.base_row(artifact)
            row.source_line = 1
            row.turn_type = "system"
            row.timestamp_utc = timestamp
            row.text = "cloud task target: target=%s repository_id=%s branch=%s (from %s, %s)" % (
                _dash(target.get("target")),
                _dash(target.get("repositoryId")),
                _dash(target.get("branch")),
                artifact.rel,
                "file mtime",
            )
            yield row
        source = "%s cloudSessionStatuses, observedAt" % artifact.rel
        for session_id, status in _object(doc.get("cloudSessionStatuses")).items():
            status = _object(status)
            yield from self._cloud_rows(
                artifact,
                str(session_id),
                to_utc(status.get("observedAt")),
                "status %s" % _dash(status.get("status")),
                source,
            )

    def _cloud_rows(
        self, artifact: Artifact, session_id: str, timestamp: str, detail: str, source: str
    ) -> Iterator[Row]:
        """Yield the `session origin:` and `session synced:` rows of a Cline Cloud session."""
        row = self.base_row(artifact)
        row.turn_type = "system"
        row.timestamp_utc = timestamp
        row.session_id = session_id
        row.text = "session origin: remote cline-cloud session %s %s (from %s)" % (
            session_id,
            detail,
            source,
        )
        yield row
        yield replace(
            row,
            text="session synced: cline-cloud holds session %s, no local transcript (from %s)"
            % (session_id, source),
        )

    def _parse_pending_starts(self, artifact: Artifact) -> Iterator[Row]:
        """Yield a pair of Cline Cloud rows per record of a pending-start file.

        Each record is `{sessionId, account, endpoint}`; the rows are at the file's
        modification time.
        """
        errors: list = []
        doc = load_json(artifact.disk_path, errors)
        if not isinstance(doc, list):
            message = "cloud pending-start file is not a JSON array"
            yield from error_rows(self, artifact, errors or [(0, message, "")])
            return
        timestamp = file_mtime(artifact)
        for position, record in enumerate(doc, 1):
            if not isinstance(record, dict) or not isinstance(record.get("sessionId"), str):
                errors.append((position, "pending-start record %d has no sessionId" % position, ""))
                continue
            detail = "sandbox start pending account=%s endpoint=%s" % (
                _dash(record.get("account")),
                _dash(record.get("endpoint")),
            )
            source = "%s, file mtime" % artifact.rel
            for row in self._cloud_rows(artifact, record["sessionId"], timestamp, detail, source):
                row.source_line = position
                yield row
        yield from error_rows(self, artifact, errors)

    def _parse_global_settings(self, artifact: Artifact) -> Iterator[Row]:
        """Yield a `permission setting:` row per tool setting of `global-settings.json`."""
        doc, errors = self._json_object(artifact, "global-settings.json")
        if doc is None:
            yield from error_rows(self, artifact, errors)
            return
        timestamp = file_mtime(artifact)
        for key in ("toolAutoApprove", "planActMode"):
            if doc.get(key) not in (None, ""):
                yield self._settings_row(artifact, timestamp, key, doc[key])
        for key in ("disabledTools", "disabledPlugins", "disabledAgentPlugins"):
            entries = doc.get(key)
            for entry in entries if isinstance(entries, list) else []:
                yield self._settings_row(artifact, timestamp, key, entry)
        for name, tool in _object(doc.get("tools")).items():
            tool = _object(tool)
            if "enabled" in tool:
                yield self._settings_row(
                    artifact, timestamp, "tools.%s.enabled" % name, tool["enabled"]
                )

    def _parse_mcp_settings(self, artifact: Artifact) -> Iterator[Row]:
        """Yield each MCP server's `autoApprove` entries and `disabled` flag as settings.

        The server's command, `env`, `headers` and OAuth state are never read.
        """
        doc, errors = self._json_object(artifact, "cline_mcp_settings.json")
        if doc is None:
            yield from error_rows(self, artifact, errors)
            return
        timestamp = file_mtime(artifact)
        for name, server in _object(doc.get("mcpServers")).items():
            server = _object(server)
            entries = server.get("autoApprove")
            for entry in entries if isinstance(entries, list) else []:
                key = "mcpServers.%s.autoApprove" % name
                yield self._settings_row(artifact, timestamp, key, entry)
            if isinstance(server.get("disabled"), bool):
                key = "mcpServers.%s.disabled" % name
                yield self._settings_row(artifact, timestamp, key, server["disabled"])

    # -- cache/user_input_history.jsonl

    def _parse_input_history(self, artifact: Artifact) -> Iterator[Row]:
        errors: list = []
        entries = [
            (entry_line, record["prompt"], record.get("ts"))
            for entry_line, record in iter_jsonl(artifact.disk_path, errors)
            if isinstance(record.get("prompt"), str) and record["prompt"].strip()
        ]
        for k, (line_number, prompt, timestamp) in enumerate(entries):
            row = self.base_row(artifact)
            row.source_line = line_number
            row.turn_type = "user"
            # Every line carries the time of the file's last rewrite, the
            # newest prompt's own time only; a bad line makes that unsure.
            if k == len(entries) - 1 and not errors and isinstance(timestamp, (int, float)):
                row.timestamp_utc = to_utc(timestamp)
            row.text = compact(prompt)
            yield row
        yield from error_rows(self, artifact, errors)

    # -- session-search.db

    def _parse_search_database(self, artifact: Artifact) -> Iterator[Row]:
        problem = sqlite_problem(artifact.disk_path)
        if problem:
            if problem == NOT_SQLITE:
                problem = "session-search.db is not a SQLite database"
            yield from error_rows(self, artifact, [(0, problem, "")])
            return
        try:
            with open_copy(artifact.disk_path) as connection:
                tables = table_names(connection)
                indexed = []
                if "indexed_sessions" in tables:
                    indexed = [
                        dict(database_row)
                        for database_row in connection.execute(
                            "select * from indexed_sessions order by rowid"
                        )
                    ]
                docs = []
                if "session_search" in tables:
                    try:
                        docs = [
                            dict(database_row)
                            for database_row in connection.execute(
                                "select %s from session_search order by rowid" % SEARCH_COLUMNS
                            )
                        ]
                    except sqlite3.OperationalError as error:
                        problem = (
                            "session_search unreadable (%s), indexed_sessions read alone" % error
                        )
        except Exception as error:  # noqa: BLE001 - a damaged database must not stop the timeline
            yield from error_rows(
                self, artifact, [(0, "session-search.db unreadable (%s)" % error, "")]
            )
            return
        if problem:
            yield from error_rows(self, artifact, [(0, problem, "")])
        by_session: dict[str, list[dict]] = {}
        for indexed_session in indexed:
            by_session.setdefault(str(indexed_session.get("session_id") or ""), [])
        for document in docs:
            by_session.setdefault(str(document.get("session_id") or ""), []).append(document)
        meta = {str(entry.get("session_id") or ""): entry for entry in indexed}
        sessions_dir = artifact.disk_path.parent.parent / "sessions"
        for session_id, session_docs in by_session.items():
            yield from self._search_session(
                artifact, session_id, meta.get(session_id, {}), session_docs, sessions_dir
            )

    def _search_session(
        self, artifact: Artifact, session_id: str, indexed: dict, docs: list, sessions_dir: Path
    ) -> Iterator[Row]:
        """Yield the search index's session row, then a row per message document in order."""
        head = next((candidate for candidate in docs if candidate.get("role") == "session"), None)
        docs = sorted(
            (candidate for candidate in docs if candidate.get("role") != "session"),
            key=lambda candidate: _ordinal(candidate.get("ordinal")),
        )
        workspace = str((head or (docs[0] if docs else {})).get("workspace_root") or "")
        # A pre-scan: the same parser reports the manifest when it parses the file.
        manifest = (
            load_json(sessions_dir / session_id / (session_id + ".json"), [])
            if session_id
            else None
        )
        if isinstance(manifest, dict):
            project = str(manifest.get("cwd") or manifest.get("workspace_root") or "")
        else:
            project = workspace
        fields = {
            "title": indexed.get("title") or (head or {}).get("title"),
            "workspace_root": workspace,
            "prompt": (head or {}).get("content"),
            "document_count": indexed.get("document_count"),
            "indexed_at": indexed.get("indexed_at"),
        }
        row = self.base_row(artifact)
        row.timestamp_utc = to_utc((head or (docs[0] if docs else {})).get("started_at"))
        row.session_id = session_id
        row.project_path = project
        row.turn_type = "system"
        detail = " ".join(
            "%s=%s" % (k, fields[k]) for k in SEARCH_KEYS if fields[k] not in (None, "")
        )
        row.text = compact("session search index: %s" % detail)
        yield row
        for document in docs:
            role = document.get("role")
            row = self.base_row(artifact)
            row.source_line = _ordinal(document.get("ordinal")) + 1
            row.session_id = session_id
            row.project_path = project
            row.turn_type = role if role in ("user", "assistant") else "system"
            row.text = compact(document.get("content"))
            yield row


def _ordinal(value) -> int:
    """Return a message document's `ordinal`, -1 when it is not a number."""
    try:
        return int(value)
    except (TypeError, ValueError):
        return -1


def load_json_text(text):
    """Return `text` parsed as JSON, or None."""
    try:
        return json.loads(text)
    except (TypeError, ValueError):
        return None


def _object(value) -> dict:
    """Return `value` when it is a JSON object, else an empty one."""
    return value if isinstance(value, dict) else {}


def _dash(value) -> str:
    """Return a value as row text, `-` when absent or empty."""
    return "-" if value in (None, "") else str(value)


def _imported(session: dict, meta) -> None:
    """Copy `metadata.importedFrom` onto the session under the start row's `imported_*` names."""
    source = _object(_object(meta).get("importedFrom"))
    for key, name in IMPORTED_KEYS:
        if source.get(key) not in (None, ""):
            session[name] = source[key]


def _denial(content) -> str:
    """Return the `permission decision:` text of a skipped call's tool_result, or "".

    The content is `{"error": <reason>}`, stored as a JSON string.
    """
    if isinstance(content, str):
        try:
            content = json.loads(content)
        except ValueError:
            return ""
    reason = _object(content).get("error")
    if not isinstance(reason, str):
        return ""
    if reason.endswith(REJECTION_SUFFIX):
        return "permission decision: rejected reason=%s" % reason[: -len(REJECTION_SUFFIX)]
    if BLOCKED_PATTERN.match(reason):
        return "permission decision: blocked reason=%s" % reason
    return ""


def _decisions(content) -> dict:
    """Return tool_use_id -> `permission decision:` text for a message's skipped calls.

    A skipped call is an errored tool_result block whose reason is a rejection or a block.
    """
    out: dict = {}
    for block in content if isinstance(content, list) else []:
        block = _object(block)
        if block.get("type") == "tool_result" and block.get("is_error"):
            text = _denial(block.get("content"))
            if text:
                out[str(block.get("tool_use_id") or "")] = compact(text)
    return out


def _hook_event(row: Row, hook: str, entry: dict) -> bool:
    """Fill `row` as the transcript event a hook audit line repeats; return whether it is one."""
    if hook == "prompt_submit":
        prompt = _object(entry.get("userPromptSubmit")).get("prompt")
        if not isinstance(prompt, str) or not prompt.strip():
            return False
        row.turn_type, row.text = "user", compact(prompt)
    elif hook == "tool_call":
        call = _object(entry.get("tool_call"))
        row.turn_type = "tool_use"
        row.tool_name = str(call.get("name") or "")
        row.tool_use_id = str(call.get("id") or "")
        row.text = compact(tool_summary(row.tool_name, call.get("input")))
    elif hook == "tool_result":
        record = _object(entry.get("tool_result"))
        post = _object(entry.get("postToolUse"))
        row.turn_type = "tool_result"
        row.tool_name = str(record.get("name") or post.get("toolName") or "")
        row.tool_use_id = str(record.get("id") or "")
        if record.get("error"):
            row.text = compact("[error] %s" % record["error"])
        elif isinstance(post.get("result"), str):
            row.text = compact(post["result"])
        else:
            output = record.get("output")
            row.text = compact(
                text_of(output) if isinstance(output, (str, list)) else compact_json(output)
            )
    elif hook == "agent_end":
        text = _object(entry.get("turn")).get("outputText")
        if not isinstance(text, str) or not text.strip():
            return False
        row.turn_type, row.text = "assistant", compact(text)
    else:
        return False
    return True
