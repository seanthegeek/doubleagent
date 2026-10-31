"""Parse the agent conversations and standing settings of the Cursor IDE.

Cursor's names: a composer is one conversation (`composerId`), a bubble one message, reasoning
block or tool call in it (`bubbleId`), `toolFormerData` a tool call's record, `yolo` auto-run
and a cloud agent (`bc-<uuid>`) a conversation run on Cursor's servers. `<ide>` is
`Library/Application Support/Cursor`, `.config/Cursor` or `AppData/Roaming/Cursor`. Reads:

- `<ide>/User/globalStorage/state.vscdb`, the primary store: `cursorDiskKV`
  `composerData:<id>` (`status`, `name`, `createdAt`, `isArchived`, `workspaceIdentifier`,
  `trackedGitRepos`, `fullConversationHeadersOnly`), `composerConvHeaders:<id>:<n>` chunks,
  `bubbleId:<id>:<bubbleId>` (`type`, `text`, `createdAt`, `startedAtMs`, `completedAtMs`,
  `modelInfo.modelName`, `thinking.text`, `toolFormerData`), `composerDraft:<id>` and, for
  deleted composers only, JSON `agentKv:blob:` rows; the `composerHeaders` table (`value`
  `name`, `isArchived`, `workspaceIdentifier`, `trackedGitRepos`, `agentLocation`,
  `agentLocationHistory`); and `ItemTable` keys: the settings blob (`composerState`,
  `aiSettings.modelLastUsedAt`), the sandbox switch, `github-<login>` and its `-usages`,
  `remoteAgentHost.`, `cloudAgentRepository.agents.`, `cloudAgentRepository.mcpAuthResolution`,
  `cloudEnvironmentTemplate.githubRepos`, the key names of `mcpOAuth.` and
  `cursor.plugins.installedIds.`. No `secret://` value is read.
- `.cursor/projects/<cwd>/agent-transcripts/<id>/<id>.jsonl`, a secondary source (`role`,
  `message.content[]` `type`, `text`, `name`, `input`); a user block's `<timestamp>` tag has
  minute precision.
- `.cursor/ai-tracking/ai-code-tracking.db` (`TRACKING_TABLES`), `conversation-search.db` and
  `mcp-oauth-attempts/*.json` under `<ide>/User/globalStorage/` (`SEARCH_COLUMNS` and the FTS5
  shadow table, read as a plain table), `.cursor/cli-config.json` and `.cursor/permissions.json`.

The project, session and model of a side-database or transcript row come from the home's
`state.vscdb`. The standalone Cursor Agent CLI's `~/.cursor/chats` and `acp-sessions` are not
read. Validated against real installs of the IDE on macOS, Linux and Windows (3.24.18 to
3.26.13) and the 3.25.9 and 3.26.13 bundles; see `research/cursor.md`, Parsing.
"""

from __future__ import annotations

import base64
import json
import re
import sqlite3
from collections.abc import Iterator
from datetime import datetime, timedelta, timezone
from pathlib import Path
from urllib.parse import unquote, urlparse

from ..inputs import Artifact, error_reason
from ..model import Row, compact
from ..sqlite_util import is_sqlite, open_copy, table_names
from ..timeutil import format_utc, to_utc
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_mtime,
    iter_jsonl,
    setting_value,
    text_of,
    track_change,
)

IDE = r"(?:Library/Application Support|\.config|AppData/Roaming)/Cursor"
IDE_PARENTS = ("Library/Application Support/Cursor", ".config/Cursor", "AppData/Roaming/Cursor")
STATE_PATTERN = re.compile(r"^%s/User/globalStorage/state\.vscdb$" % IDE)
SEARCH_PATTERN = re.compile(r"^%s/User/globalStorage/conversation-search\.db$" % IDE)
TRANSCRIPT_PATTERN = re.compile(r"^\.cursor/projects/[^/]+/agent-transcripts/[^/]+/([^/]+)\.jsonl$")
TRACKING_PATTERN = re.compile(r"^\.cursor/ai-tracking/ai-code-tracking\.db$")
CLI_CONFIG_PATTERN = re.compile(r"^\.cursor/cli-config\.json$")
PERMISSIONS_PATTERN = re.compile(r"^\.cursor/permissions\.json$")
ATTEMPT_PATTERN = re.compile(r"^%s/User/globalStorage/mcp-oauth-attempts/([^/]+\.json)$" % IDE)

SETTINGS_KEY = (
    "src.vs.platform.reactivestorage.browser.reactiveStorageServiceImpl"
    ".persistentStorage.applicationUser"
)
SANDBOX_KEY = "cursor/sandboxAutoRunSettingEnabled"
# The editor's enrolment of this machine as a remote agent host for an
# account; `secret://remoteAgentHost.computerKey.<account>` beside them is
# its credential and is never read.
ENROLLED_PREFIX = "remoteAgentHost.enrolled."
DISPLAY_PREFIX = "remoteAgentHost.displayName."
STANDING_LISTS = (
    "yoloCommandAllowlist",
    "yoloCommandDenylist",
    "smartAllowlistDenylist",
    "webFetchDomainAllowlist",
    "autoApprovedModeTransitions",
)
SWITCHES = (
    "yoloEnableRunEverything",
    "yoloDeleteFileDisabled",
    "yoloMcpToolsDisabled",
    "yoloOutsideWorkspaceDisabled",
    "autoApproveModeTransitions",
    "autoAcceptWebSearchTool",
    "autoApplyFilesOutsideContext",
    "doNotShowYoloModeWarningAgain",
    "doNotShowFullYoloModeWarningAgain",
    "useLegacyTerminalTool",
    "playwrightProtection",
    "mcpAuthBlocking",
)
# The settings blob is flushed late by the app, so its rows say so.
COMPOSER_SOURCE = "(from state.vscdb composerState, file mtime, written late by the app)"
MODEL_USED_SOURCE = "(from state.vscdb aiSettings.modelLastUsedAt, written late by the app)"
STOPPED_TEXT = (
    "permission decision: stopped by the user (reviewData.status None,"
    " additionalData.status rejected)"
)
DELETED_TEXT = "conversation deleted (header kept; time is the header's lastUpdatedAt)"
# The delete sets the header's `isArchived`, so a deleted header reads
# archived although the conversation never was.
ARCHIVED_BY_DELETE = " (isArchived is set by the delete)"
# The cloud agent list, one `ItemTable` key per Cursor account (the suffix
# is the account id, `auth0|user_<id>` or URL-encoded `auth0%7Cuser_<id>`),
# and the repositories a cloud agent may use.
CLOUD_AGENTS_PREFIX = "cloudAgentRepository.agents."
GITHUB_REPOS_KEY = "cloudEnvironmentTemplate.githubRepos"
CLOUD_ORIGIN = "session origin: remote cursor cloud agent"
CLOUD_SOURCE = "(from state.vscdb cloudAgentRepository.agents, rowUpdatedAtMs)"
CLOUD_SYNCED = (
    "session synced: cursor cloud agent %s (vendor copy; from state.vscdb"
    " cloudAgentRepository.agents)"
)
REPOS_SOURCE = "(from state.vscdb cloudEnvironmentTemplate.githubRepos, file mtime)"
CLOUD_CACHE = "cloud-cache"
# The cloud agent's request to authorise an MCP server, answered by the
# user.
MCP_AUTH_KEY = "cloudAgentRepository.mcpAuthResolution"
MCP_AUTH_SOURCE = "(from state.vscdb cloudAgentRepository.mcpAuthResolution, file mtime)"
# MCP OAuth state, one `ItemTable` key per scope and field: the suffix is
# URL-safe base64 of `[<scope>] <field>`. The `mcpOAuth.secret.` values are
# safeStorage ciphertext and are never read: only their key names are
# selected.
MCP_GLOBAL_PREFIX = "mcpOAuth.global."
MCP_SECRET_PREFIX = "mcpOAuth.secret."
MCP_KEY_PATTERN = re.compile(r"^\[(.+)\] (\S+)$")
MCP_TOKENS = " tokens=stored (ciphertext, not read)"
PLUGINS_PREFIX = "cursor.plugins.installedIds."
PLUGINS_SOURCE = "(from state.vscdb cursor.plugins.installedIds, file mtime)"

# `<timestamp>Thursday, Oct 1, <year>, 6:00 AM (UTC-4)</timestamp>` LF
# `<user_query>Q</user_query>`, the worker's wrapper of a user text block.
WRAPPER_PATTERN = re.compile(
    r"^<timestamp>([^<]*)</timestamp>\s*<user_query>(.*)</user_query>\s*$", re.DOTALL
)
TAG_PATTERN = re.compile(
    r"^\s*[A-Za-z]+,\s+([A-Za-z]{3})[A-Za-z]*\.?\s+(\d{1,2}),\s+(\d{4}),\s+(\d{1,2}):(\d{2})"
    r"\s*([AaPp][Mm])\s*\(UTC(?:([+-])(\d{1,2})(?::?(\d{2}))?)?\)\s*$"
)
MONTHS = ("jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec")

TRACKING_TABLES = {
    "ai_code_hashes": (
        "fileName",
        "source",
        "model",
        "requestId",
        "conversationId",
        "timestamp",
    ),
    "conversation_summaries": ("conversationId", "title", "tldr", "model", "updatedAt"),
    "ai_deleted_files": ("gitPath", "composerId", "conversationId", "model", "deletedAt"),
    "scored_commits": ("commitHash", "branchName", "linesAdded", "linesDeleted", "scoredAt"),
}
SEARCH_COLUMNS = ("fts_rowid", "id", "title", "source", "is_archived", "updated_at")


def run_mode(mode: dict, sandbox: bool) -> str:
    """Return the UI label of a `modes4` entry's run mode.

    The label is derived from the `autoRun`, `smartModeAutoRun` and `fullAutoRun` flags and the
    sandbox key (research/cursor.md, Run mode and standing lists); no run-mode string is stored.
    """
    if mode.get("fullAutoRun") is True:
        return "Run Everything"
    if mode.get("autoRun") is not True:
        return "Ask Every Time"
    label = "Auto-review" if mode.get("smartModeAutoRun") is True else "Allowlist"
    return label + " (with Sandbox)" if sandbox else label


def tag_time(tag: str) -> str:
    """Return the transcript's `<timestamp>` tag as a UTC timestamp, or "" when it does not parse.

    The tag is local time to the minute with its UTC offset in parentheses.
    """
    match = TAG_PATTERN.match(tag)
    if not match:
        return ""
    month, day, year, hour, minute, meridiem, sign, offset_hours, offset_minutes = match.groups()
    if month.lower() not in MONTHS:
        return ""
    hour_24 = int(hour) % 12 + (12 if meridiem.lower() == "pm" else 0)
    try:
        local = datetime(int(year), MONTHS.index(month.lower()) + 1, int(day), hour_24, int(minute))
    except ValueError:
        return ""
    offset = timedelta(hours=int(offset_hours or 0), minutes=int(offset_minutes or 0))
    if sign == "-":
        offset = -offset
    return format_utc((local - offset).replace(tzinfo=timezone.utc))


def unwrap_query(text: str) -> tuple[str, str]:
    """Return (query, timestamp) of a transcript user text block.

    The query is the text inside `<user_query>` and the time the tag's, or the whole text and ""
    when the wrapper or its tag does not parse.
    """
    match = WRAPPER_PATTERN.match(text)
    if match:
        when = tag_time(match.group(1))
        if when:
            return match.group(2), when
    return text, ""


def _as_text(value) -> str:
    """A database value as text: bytes decoded as UTF-8 with replacement."""
    if isinstance(value, (bytes, bytearray, memoryview)):
        return bytes(value).decode("utf-8", errors="replace")
    return "" if value is None else str(value)


def _object(value) -> dict:
    return value if isinstance(value, dict) else {}


def _decode(rowid: int, key: str, value, errors: list):
    """Return the JSON value of a row, or None, appending an error entry when it does not decode."""
    text = _as_text(value)
    try:
        return json.loads(text)
    except ValueError as error:
        errors.append((rowid, "unparseable record %d (%s: %s)" % (rowid, key, error), text))
        return None


def _prefixed(connection: sqlite3.Connection, prefix: str) -> list[tuple[int, str, object]]:
    """Return the `cursorDiskKV` rows whose key starts with `prefix`, in key order.

    A range over the key selects them, so the table's index is used.
    """
    end = prefix[:-1] + chr(ord(prefix[-1]) + 1)
    sql = "select rowid, key, value from cursorDiskKV where key >= ? and key < ? order by key"
    return [
        (int(row[0]), _as_text(row[1]), row[2]) for row in connection.execute(sql, (prefix, end))
    ]


def _one(connection: sqlite3.Connection, key: str) -> tuple[int, object] | None:
    row = connection.execute(
        "select rowid, value from cursorDiskKV where key = ?", (key,)
    ).fetchone()
    return (int(row[0]), row[1]) if row else None


def _select(connection: sqlite3.Connection, table: str, wanted: tuple[str, ...]) -> list[dict]:
    """Return every row of `table` as a dict of the `wanted` columns it has, plus `_rowid`."""
    have = {info[1] for info in connection.execute("pragma table_info(%s)" % table)}
    columns = [column for column in wanted if column in have]
    if not columns:
        return []
    sql = 'select rowid as _rowid, %s from "%s" order by rowid' % (
        ", ".join('"%s"' % column for column in columns),
        table,
    )
    return [dict(row) for row in connection.execute(sql)]


def _project(*documents: dict) -> tuple[str, str]:
    """Return (project_path, git_branch) of a composer from its header value and `composerData`.

    The documents are tried in the order given.
    """
    path = ""
    for document in documents:
        path = str(
            _object(_object(document.get("workspaceIdentifier")).get("uri")).get("fsPath") or ""
        )
        if path:
            break
    repos = [
        tracked
        for document in documents
        for tracked in document.get("trackedGitRepos") or []
        if isinstance(tracked, dict)
    ]
    if not path and repos:
        path = str(repos[0].get("repoPath") or "")
    for repo in repos:
        if str(repo.get("repoPath") or "") == path:
            branches = repo.get("branches") or []
            if branches and isinstance(branches[0], str):
                return path, branches[0]
            break
    return path, ""


class _Composer:
    """Hold what the store has about one composer: data, header row and header value."""

    def __init__(self, composer_id: str) -> None:
        self.composer_id = composer_id
        self.data: dict | None = None
        self.data_rowid = 0
        self.header: dict | None = None
        self.head: dict = {}
        self.errors: list = []

    @property
    def project(self) -> tuple[str, str]:
        return _project(self.head, self.data or {})

    def created(self):
        for document in (self.data or {}, self.head, self.header or {}):
            if document.get("createdAt") not in (None, ""):
                return document.get("createdAt")
        return None

    def header_updated(self):
        header = self.header or {}
        value = header.get("lastUpdatedAt")
        return value if value not in (None, "") else self.head.get("lastUpdatedAt")

    def archived(self) -> bool:
        header = self.header or {}
        return (
            bool(header.get("isArchived"))
            or self.head.get("isArchived") is True
            or ((self.data or {}).get("isArchived") is True)
        )


def _read_headers(connection: sqlite3.Connection, composers: dict) -> None:
    """Attach each `composerHeaders` row and its decoded `value` to its composer.

    A value that does not decode is an error of that composer.
    """
    if "composerHeaders" not in table_names(connection):
        return
    wanted = ("composerId", "createdAt", "lastUpdatedAt", "isArchived", "value")
    for record in _select(connection, "composerHeaders", wanted):
        composer_id = str(record.get("composerId") or "")
        if not composer_id:
            continue
        composer = composers.setdefault(composer_id, _Composer(composer_id))
        composer.header = record
        if record.get("value") is not None:
            label = "composerHeaders " + composer_id
            composer.head = _object(
                _decode(int(record["_rowid"]), label, record["value"], composer.errors)
            )


def read_composers(connection: sqlite3.Connection) -> dict[str, _Composer]:
    """Return every composer of the store by id.

    `composerData` rows come in rowid order first, then the header-only composers.
    """
    composers: dict[str, _Composer] = {}
    if "cursorDiskKV" in table_names(connection):
        rows = sorted(_prefixed(connection, "composerData:"))
        for rowid, key, value in rows:
            composer_id = key[len("composerData:") :]
            composer = composers.setdefault(composer_id, _Composer(composer_id))
            composer.data = _object(_decode(rowid, key, value, composer.errors))
            composer.data_rowid = rowid
    _read_headers(connection, composers)
    return composers


def read_cloud_agents(connection: sqlite3.Connection, errors: list) -> list[tuple[int, dict]]:
    """Return (rowid, agent) of every element with a `bcId` of the cloud agent lists, in key order.

    The `cloudAgentRepository.agents.<account>` keys are selected by the prefix alone, so either
    spelling of the account id matches; a value that is not a JSON list is skipped, and one that
    starts as a list and does not decode is an error entry.
    """
    if "ItemTable" not in table_names(connection):
        return []
    sql = "select rowid, key, value from ItemTable where substr(key, 1, ?) = ? order by key"
    found: list[tuple[int, dict]] = []
    for rowid, key, value in connection.execute(
        sql, (len(CLOUD_AGENTS_PREFIX), CLOUD_AGENTS_PREFIX)
    ):
        agents = None
        if _as_text(value).lstrip().startswith("["):
            agents = _decode(int(rowid), _as_text(key), value, errors)
        for agent in agents if isinstance(agents, list) else []:
            if isinstance(agent, dict) and agent.get("bcId"):
                found.append((int(rowid), agent))
    return found


def cloud_model(agent: dict) -> tuple[str, str]:
    """Return (model, requested) of a cloud agent.

    The model is `modelDetails.modelName`, else `requestedModel.modelId`; requested is that
    `modelId`.
    """
    requested = _as_text(_object(agent.get("requestedModel")).get("modelId"))
    model = _as_text(_object(agent.get("modelDetails")).get("modelName")) or requested
    return model, requested


def project_map(path: Path) -> dict[str, tuple[str, str]]:
    """Return composerId or cloud agent `bcId` -> (project_path, git_branch) of a `state.vscdb`.

    The result is {} when the file is missing or unreadable.
    """
    return _state_index(path)[0]


def model_map(path: Path) -> dict[str, str]:
    """Return cloud agent `bcId` -> model of a `state.vscdb`, {} when it is unreadable."""
    return _state_index(path)[1]


def _state_index(path: Path) -> tuple[dict[str, tuple[str, str]], dict[str, str]]:
    """Return (project_map, model_map) of the `state.vscdb` at `path`.

    A cloud agent's project is its `repoUrl`. The result is cached by the file's and its sidecar's
    size and mtime, since every transcript and side database of a home asks for the same file.
    """
    key = [str(path)]
    for file_path in (path, Path(str(path) + "-wal")):
        try:
            file_stat = file_path.stat()
            key += [file_stat.st_size, file_stat.st_mtime_ns]
        except OSError:
            key += [None, None]
    cache_key = tuple(key)
    if cache_key in _PROJECT_CACHE:
        return _PROJECT_CACHE[cache_key]
    found: dict[str, tuple[str, str]] = {}
    models: dict[str, str] = {}
    if is_sqlite(path):
        try:
            with open_copy(path) as connection:
                # A pre-scan: the file's own parse reports its errors.
                found = {
                    composer_id: composer.project
                    for composer_id, composer in read_composers(connection).items()
                }
                for _rowid, agent in read_cloud_agents(connection, []):
                    bc_id = str(agent["bcId"])
                    found.setdefault(bc_id, (_as_text(agent.get("repoUrl")), ""))
                    models.setdefault(bc_id, cloud_model(agent)[0])
        except sqlite3.DatabaseError:
            found, models = {}, {}
    _PROJECT_CACHE.clear()
    _PROJECT_CACHE[cache_key] = (found, models)
    return found, models


_PROJECT_CACHE: dict[tuple, tuple[dict[str, tuple[str, str]], dict[str, str]]] = {}


def _home_state(artifact: Artifact) -> Path | None:
    """Return the IDE `state.vscdb` in the artifact's home, or None."""
    for parent in IDE_PARENTS:
        path = artifact.home.disk_path / parent / "User/globalStorage/state.vscdb"
        if path.is_file():
            return path
    return None


def home_projects(artifact: Artifact) -> dict[str, tuple[str, str]]:
    """Return the composer and cloud agent projects of the home's IDE `state.vscdb`."""
    path = _home_state(artifact)
    return project_map(path) if path else {}


def home_models(artifact: Artifact) -> dict[str, str]:
    """The cloud agent models of the home's IDE `state.vscdb`."""
    path = _home_state(artifact)
    return model_map(path) if path else {}


def cloud_cached_ids(state: Path) -> set[str]:
    """Return the ids of the `cloud-cache` rows of the `conversation-search.db` beside `state`.

    The parse of that database yields their origin rows. The set is empty when it is missing or
    unreadable.
    """
    path = state.parent / "conversation-search.db"
    if not is_sqlite(path):
        return set()
    try:
        with open_copy(path) as connection:
            if "conversations" not in table_names(connection):
                return set()
            sql = "select id from conversations where source = ?"
            return {_as_text(row[0]) for row in connection.execute(sql, (CLOUD_CACHE,))}
    except sqlite3.DatabaseError:
        return set()


class CursorParser(Parser):
    """Parse the Cursor IDE's stores, transcripts and settings files."""

    agent = "cursor"
    name = "cursor"
    records_origin = True
    records_sync = True
    has_secondary = True

    def wants(self, artifact: Artifact) -> bool:
        """Return whether the artifact is one of the files this parser reads."""
        rel = artifact.rel
        return any(
            pattern.match(rel)
            for pattern in (
                STATE_PATTERN,
                SEARCH_PATTERN,
                TRANSCRIPT_PATTERN,
                TRACKING_PATTERN,
                CLI_CONFIG_PATTERN,
                PERMISSIONS_PATTERN,
                ATTEMPT_PATTERN,
            )
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return whether the artifact is an agent transcript, which repeats the store's events."""
        return bool(TRANSCRIPT_PATTERN.match(artifact.rel))

    def secondary_tolerance(self, artifact: Artifact) -> float:
        """Return 60 seconds: the transcript's tag has minute precision."""
        return 60.0

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one artifact, by the file it is."""
        rel = artifact.rel
        if STATE_PATTERN.match(rel):
            yield from self._database(artifact, self._state, opts)
        elif SEARCH_PATTERN.match(rel):
            yield from self._database(artifact, self._search, opts)
        elif TRACKING_PATTERN.match(rel):
            yield from self._database(artifact, self._tracking, opts)
        elif TRANSCRIPT_PATTERN.match(rel):
            yield from self._transcript(artifact)
        elif ATTEMPT_PATTERN.match(rel):
            yield from self._attempt(artifact)
        else:
            yield from self._settings_file(artifact)

    # -- shared -----------------------------------------------------------

    def _row(
        self, artifact: Artifact, turn_type: str, text: str, when: object = "", line: int = 0
    ) -> Row:
        row = self.base_row(artifact)
        row.turn_type = turn_type
        row.text = text
        row.timestamp_utc = to_utc(when) if when not in (None, "") else ""
        row.source_line = line
        return row

    def _database(self, artifact: Artifact, reader, opts: Options) -> Iterator[Row]:
        if not is_sqlite(artifact.disk_path):
            yield self._row(artifact, "system", "parser: not a SQLite database")
            return
        try:
            with open_copy(artifact.disk_path) as connection:
                rows = list(reader(artifact, connection, opts))
        except sqlite3.DatabaseError as error:
            yield self._row(artifact, "system", "parser: SQLite error: %s" % error)
            return
        yield from rows

    # -- state.vscdb ------------------------------------------------------

    def _state(
        self, artifact: Artifact, connection: sqlite3.Connection, opts: Options
    ) -> Iterator[Row]:
        errors: list = []
        composers = read_composers(connection)
        has_key_values = "cursorDiskKV" in table_names(connection)
        drafts = {
            key[len("composerDraft:") :]: (rowid, key, value)
            for rowid, key, value in (
                _prefixed(connection, "composerDraft:") if has_key_values else []
            )
        }
        for composer_id in drafts:
            composers.setdefault(composer_id, _Composer(composer_id))
        deleted: list[_Composer] = []
        for composer in composers.values():
            stamp = self._stamper(artifact, composer)
            if composer.data is not None:
                entries = self._entries(connection, composer, has_key_values)
                # A draft: never sent, no header list in either layout.
                if entries or composer.data.get("status") != "none":
                    yield from self._conversation(connection, composer, entries, stamp, opts)
            elif self._was_conversation(composer):
                deleted.append(composer)
                yield from self._deleted(composer, stamp)
            if composer.composer_id in drafts:
                yield from self._draft(composer, drafts[composer.composer_id], stamp)
            yield from error_rows(
                self, artifact, composer.errors, composer.composer_id, composer.project[0]
            )
        if deleted and has_key_values:
            yield from self._blobs(artifact, connection, deleted)
        yield from self._standing(artifact, connection, errors)
        yield from self._integrations(artifact, connection, errors)
        yield from self._cloud(artifact, connection, errors)
        yield from self._mcp(artifact, connection)
        yield from self._plugins(artifact, connection, errors)
        yield from error_rows(self, artifact, errors)

    def _stamper(self, artifact: Artifact, composer: _Composer):
        path, branch = composer.project

        def stamp(turn_type: str, text: str, when="", line=0, model="") -> Row:
            row = self._row(artifact, turn_type, text, when, line)
            row.session_id = composer.composer_id
            row.project_path = path
            row.git_branch = branch
            row.model = model
            return row

        return stamp

    @staticmethod
    def _was_conversation(composer: _Composer) -> bool:
        """Return whether a header with no `composerData` is a deleted conversation.

        It is one when it is not a draft: it has an `agentLocationHistory` entry or is archived.
        """
        return bool(composer.head.get("agentLocationHistory")) or composer.archived()

    def _entries(
        self, connection: sqlite3.Connection, composer: _Composer, has_key_values: bool
    ) -> list[dict]:
        inline = (composer.data or {}).get("fullConversationHeadersOnly")
        if isinstance(inline, list) and inline:
            return [entry for entry in inline if isinstance(entry, dict)]
        if not has_key_values:
            return []
        entries: list[dict] = []
        for rowid, key, value in _prefixed(
            connection, "composerConvHeaders:%s:" % composer.composer_id
        ):
            chunk = _decode(rowid, key, value, composer.errors)
            if isinstance(chunk, list):
                entries += [entry for entry in chunk if isinstance(entry, dict)]
        return entries

    def _conversation(
        self,
        connection: sqlite3.Connection,
        composer: _Composer,
        entries: list[dict],
        stamp,
        opts: Options,
    ) -> Iterator[Row]:
        data = composer.data or {}
        name = compact(data.get("name") or composer.head.get("name"))
        if name:
            yield stamp("system", "session name: " + name, composer.created(), composer.data_rowid)
        yield from self._origin(composer, stamp)
        seen: dict = {}
        model = ""
        for entry in entries:
            bubble_id = str(entry.get("bubbleId") or "")
            key = "bubbleId:%s:%s" % (composer.composer_id, bubble_id)
            found = _one(connection, key) if bubble_id else None
            if found is None:
                yield stamp("system", "parser: missing bubble " + bubble_id, entry.get("createdAt"))
                continue
            rowid, value = found
            bubble = _object(_decode(rowid, key, value, composer.errors))
            if not bubble:
                continue
            if (bubble.get("type") or entry.get("type")) == 1:
                change = track_change(
                    "model",
                    seen,
                    composer.composer_id,
                    str(_object(bubble.get("modelInfo")).get("modelName") or ""),
                )
                model = seen.get(composer.composer_id, "")
                when = bubble.get("createdAt") or entry.get("createdAt")
                if change:
                    yield stamp("system", change, when, rowid, model)
                text = compact(bubble.get("text"))
                if text:
                    yield stamp("user", text, when, rowid, model)
            elif isinstance(bubble.get("toolFormerData"), dict):
                yield from self._tool(bubble, entry, rowid, stamp, model)
            else:
                when = (
                    bubble.get("createdAt") or bubble.get("startedAtMs") or entry.get("createdAt")
                )
                thinking = compact(_object(bubble.get("thinking")).get("text"))
                if thinking and opts.include_thinking:
                    yield stamp("thinking", thinking, when, rowid, model)
                text = compact(bubble.get("text"))
                if text:
                    yield stamp("assistant", text, when, rowid, model)
        if composer.archived():
            yield stamp(
                "system",
                "conversation archived",
                composer.header_updated(),
                composer.data_rowid,
                model,
            )

    def _origin(self, composer: _Composer, stamp) -> Iterator[Row]:
        kind = str(_object(composer.head.get("agentLocation")).get("type") or "")
        if kind and kind != "local":
            yield stamp("system", "session origin: remote " + kind, composer.created())

    def _tool(self, bubble: dict, entry: dict, rowid: int, stamp, model: str) -> Iterator[Row]:
        tool = bubble["toolFormerData"]
        extra = _object(tool.get("additionalData"))
        name = str(tool.get("name") or "")
        call_id = str(tool.get("toolCallId") or "")
        started = (
            bubble.get("startedAtMs")
            or extra.get("startedAtMs")
            or bubble.get("createdAt")
            or entry.get("createdAt")
        )
        arguments = tool.get("params") or tool.get("rawArgs")
        args_text = (
            arguments
            if isinstance(arguments, str)
            else compact_json(arguments)
            if arguments
            else ""
        )

        def row(turn_type: str, text: str, when) -> Row:
            new_row = stamp(turn_type, text, when, rowid, model)
            new_row.tool_name, new_row.tool_use_id = name, call_id
            return new_row

        yield row("tool_use", compact(args_text), started)
        review = extra.get("reviewData")
        pending = False
        if isinstance(review, dict):
            if review.get("status") == "None" and extra.get("status") == "rejected":
                # A turn the user stopped: `selectedOption` is a preset.
                text = STOPPED_TEXT
            else:
                pending = review.get("status") != "Done" or tool.get("status") == "loading"
                text = (
                    pending_text(name, extra, review) if pending else permission_text(extra, review)
                )
            yield row("system", text, started)
        result = tool.get("result")
        result_text = ""
        if result not in (None, ""):
            result_text = compact(result if isinstance(result, str) else compact_json(result))
        rejected = extra.get("status") == "rejected" and not pending
        if result_text or rejected:
            text = ("[rejected] " if rejected else "") + result_text
            yield row("tool_result", compact(text), bubble.get("completedAtMs") or started)

    def _deleted(self, composer: _Composer, stamp) -> Iterator[Row]:
        line = int((composer.header or {}).get("_rowid") or 0)
        name = compact(composer.head.get("name"))
        if name:
            yield stamp("system", "session name: " + name, composer.created(), line)
        yield from self._origin(composer, stamp)
        # The delete sets `isArchived`: no `conversation archived` row.
        text = DELETED_TEXT + (ARCHIVED_BY_DELETE if composer.archived() else "")
        yield stamp("system", text, composer.header_updated(), line)

    def _draft(self, composer: _Composer, found, stamp) -> Iterator[Row]:
        rowid, key, value = found
        draft = _object(_decode(rowid, key, value, composer.errors))
        text = compact(draft.get("text"))
        if text:
            yield stamp("system", "unsent draft: " + text, composer.created(), rowid)

    def _blobs(
        self, artifact: Artifact, connection: sqlite3.Connection, deleted: list[_Composer]
    ) -> Iterator[Row]:
        """Yield user and assistant rows from the JSON `agentKv` blobs that name a deleted composer.

        A blob that is not JSON is protobuf, not an error.
        """
        for rowid, _key, value in _prefixed(connection, "agentKv:blob:"):
            try:
                blob = json.loads(_as_text(value))
            except ValueError:
                continue
            if not isinstance(blob, dict) or "content" not in blob:
                continue
            content = blob.get("content")
            haystack = content if isinstance(content, str) else compact_json(content)
            for composer in deleted:
                if composer.composer_id not in haystack:
                    continue
                stamp = self._stamper(artifact, composer)
                role = str(blob.get("role") or "")
                text = compact(text_of(content))
                if role in ("user", "assistant"):
                    row = stamp(role, text, "", rowid)
                    row.recovered = "yes"
                else:
                    row = stamp("system", "%s: %s" % (_role_label(role), text), "", rowid)
                yield row

    def _standing(
        self, artifact: Artifact, connection: sqlite3.Connection, errors: list
    ) -> Iterator[Row]:
        if "ItemTable" not in table_names(connection):
            return
        items = {}
        sql = "select rowid, key, value from ItemTable where key in (?, ?)"
        for rowid, key, value in connection.execute(sql, (SETTINGS_KEY, SANDBOX_KEY)):
            items[_as_text(key)] = (int(rowid), value)
        if SETTINGS_KEY not in items:
            return
        rowid, value = items[SETTINGS_KEY]
        settings = _object(_decode(rowid, SETTINGS_KEY, value, errors))
        sandbox = False
        if SANDBOX_KEY in items:
            sandbox = _decode(items[SANDBOX_KEY][0], SANDBOX_KEY, items[SANDBOX_KEY][1], []) is True
        mtime = file_mtime(artifact)
        for text in standing_texts(_object(settings.get("composerState")), sandbox):
            yield self._row(artifact, "system", text, mtime, rowid)
        used = _object(_object(settings.get("aiSettings")).get("modelLastUsedAt"))
        for model_id, when in used.items():
            text = "model last used: %s %s" % (model_id, MODEL_USED_SOURCE)
            yield self._row(artifact, "system", text, when, rowid)

    def _integrations(
        self, artifact: Artifact, connection: sqlite3.Connection, errors: list
    ) -> Iterator[Row]:
        """Yield the `source host:` and `remote agent host enrolled:` rows of `ItemTable`.

        Only the `github-*` and `remoteAgentHost.*` keys are selected, so no `secret://` value is
        ever read.
        """
        if "ItemTable" not in table_names(connection):
            return
        sql = (
            "select rowid, key, value from ItemTable"
            " where substr(key, 1, 7) = 'github-' or substr(key, 1, 16) = 'remoteAgentHost.'"
        )
        items = {
            _as_text(key): (int(rowid), value) for rowid, key, value in connection.execute(sql)
        }
        mtime = file_mtime(artifact)
        for key in sorted(items):
            rowid, value = items[key]
            if key.startswith("github-") and not key.endswith("-usages"):
                login = key[len("github-") :]
                # Another extension's `github-` key that is not the account
                # list is skipped; a cut account list is a bad record.
                if not login or not isinstance(_plain(value), list):
                    if _as_text(value).startswith("["):
                        _decode(rowid, key, value, errors)
                    continue
                usages = []
                if key + "-usages" in items:
                    usages_rowid, usages_value = items[key + "-usages"]
                    found = _decode(usages_rowid, key + "-usages", usages_value, errors)
                    usages = (
                        [usage for usage in found if isinstance(usage, dict)]
                        if isinstance(found, list)
                        else []
                    )
                yield self._row(
                    artifact,
                    "system",
                    github_text(login, key, usages),
                    last_used(usages) or mtime,
                    rowid,
                )
            elif key.startswith(ENROLLED_PREFIX) and _plain(value) is True:
                account = key[len(ENROLLED_PREFIX) :]
                raw = items.get(DISPLAY_PREFIX + account, (0, None))[1]
                name = _plain(raw)
                name = compact(name if isinstance(name, str) else _as_text(raw)) or "-"
                text = (
                    "remote agent host enrolled: %s (from state.vscdb %s, file mtime;"
                    " on by default, a paired device leaves no local record)" % (name, key)
                )
                yield self._row(artifact, "system", text, mtime, rowid)

    def _cloud(
        self, artifact: Artifact, connection: sqlite3.Connection, errors: list
    ) -> Iterator[Row]:
        """Yield the rows of the cloud agent lists and of the repository list.

        The origin row of an agent is left to `conversation-search.db` when it has a `cloud-cache`
        row for the agent. `bcCachedDetails:<bcId>` is protobuf with no schema and is not read.
        """
        if "ItemTable" not in table_names(connection):
            return
        cached = cloud_cached_ids(artifact.disk_path)
        agents = read_cloud_agents(connection, errors)
        for rowid, agent in agents:
            yield from self._cloud_agent(artifact, rowid, agent, cached)
        yield from self._mcp_auth(artifact, connection, agents, errors)
        found = connection.execute(
            "select rowid, value from ItemTable where key = ?", (GITHUB_REPOS_KEY,)
        ).fetchone()
        if found is None:
            return
        repos = _decode(int(found[0]), GITHUB_REPOS_KEY, found[1], errors)
        mtime = file_mtime(artifact)
        for repo in repos if isinstance(repos, list) else []:
            if isinstance(repo, dict):
                yield self._row(artifact, "system", repository_text(repo), mtime, int(found[0]))

    def _cloud_agent(
        self, artifact: Artifact, rowid: int, agent: dict, cached: set[str]
    ) -> Iterator[Row]:
        """Yield the origin, sync, model, source host and `cloud agent:` rows of one cloud agent."""
        bc_id = str(agent["bcId"])
        repo = _as_text(agent.get("repoUrl"))
        model, requested = cloud_model(agent)
        created = agent.get("createdAt") or agent.get("rowUpdatedAtMs")

        def row(text: str, when) -> Row:
            new_row = self._row(artifact, "system", text, when, rowid)
            new_row.session_id, new_row.project_path, new_row.model = bc_id, repo, model
            return new_row

        if bc_id not in cached:
            yield row(CLOUD_ORIGIN, created)
        yield row(CLOUD_SYNCED % bc_id, created)
        if model:
            extra = " requested=" + requested if requested and requested != model else ""
            yield row("model: " + model + extra, created)
        github_repo = github_repository(repo)
        if github_repo:
            yield row("source host: github.com %s (cloud agent)" % github_repo, created)
        yield row(cloud_text(agent), agent.get("rowUpdatedAtMs") or created)

    def _mcp_auth(
        self, artifact: Artifact, connection: sqlite3.Connection, agents: list, errors: list
    ) -> Iterator[Row]:
        """Yield a `permission decision:` row per cloud agent MCP authorisation answer.

        The row is in the agent's session, joined on its `toolCallId`, at the file's modification
        time, with the project and model from the agent list.
        """
        found = connection.execute(
            "select rowid, value from ItemTable where key = ?", (MCP_AUTH_KEY,)
        ).fetchone()
        if found is None:
            return
        document = _decode(int(found[0]), MCP_AUTH_KEY, found[1], errors)
        known = {str(agent["bcId"]): agent for _rowid, agent in agents}
        mtime = file_mtime(artifact)
        for resolution in document if isinstance(document, list) else [document]:
            if not isinstance(resolution, dict):
                continue
            agent_id, call_id = (
                _as_text(resolution.get("agentId")),
                _as_text(resolution.get("toolCallId")),
            )
            text = "permission decision: %s mcp auth for cloud agent %s tool_call=%s %s" % (
                _field(resolution.get("userDecision")),
                _field(agent_id),
                _field(call_id),
                MCP_AUTH_SOURCE,
            )
            row = self._row(artifact, "system", text, mtime, int(found[0]))
            row.session_id, row.tool_use_id = agent_id, call_id
            if agent_id in known:
                row.project_path = _as_text(known[agent_id].get("repoUrl"))
                row.model = cloud_model(known[agent_id])[0]
            yield row

    def _mcp(self, artifact: Artifact, connection: sqlite3.Connection) -> Iterator[Row]:
        """Yield an `mcp server:` row per MCP OAuth scope with an `mcp_server_url`.

        The row is at the scope's `mcp_oauth_updated_at_ms`, else the file's mtime, and ends `
        tokens=stored (ciphertext, not read)` when an `mcpOAuth.secret.` key names `mcp_tokens` for
        the scope. Only the key names of the secret rows are selected. The `oauth-attempt:` scopes
        and keys whose name does not decode to `[<scope>] <field>` are skipped.
        """
        if "ItemTable" not in table_names(connection):
            return
        sql = "select key from ItemTable where substr(key, 1, ?) = ?"
        tokens = set()
        for (key,) in connection.execute(sql, (len(MCP_SECRET_PREFIX), MCP_SECRET_PREFIX)):
            name = mcp_key_name(_as_text(key), MCP_SECRET_PREFIX)
            if name and name[1] == "mcp_tokens":
                tokens.add(name[0])
        sql = "select rowid, key, value from ItemTable where substr(key, 1, ?) = ? order by key"
        scopes: dict[str, dict[str, tuple[int, object]]] = {}
        for rowid, key, value in connection.execute(
            sql, (len(MCP_GLOBAL_PREFIX), MCP_GLOBAL_PREFIX)
        ):
            name = mcp_key_name(_as_text(key), MCP_GLOBAL_PREFIX)
            if name and not name[0].startswith("oauth-attempt:"):
                scopes.setdefault(name[0], {})[name[1]] = (int(rowid), value)
        mtime = file_mtime(artifact)
        for scope in sorted(scopes):
            fields = scopes[scope]
            if "mcp_server_url" not in fields:
                continue
            rowid, raw = fields["mcp_server_url"]
            url = _plain(raw)
            url = url if isinstance(url, str) else _as_text(raw)
            updated = _plain(fields.get("mcp_oauth_updated_at_ms", (0, None))[1])
            has_time = isinstance(updated, (int, float)) and not isinstance(updated, bool)
            text = "mcp server: %s url=%s (from state.vscdb mcpOAuth.global, %s)" % (
                scope.split("::", 1)[0],
                _field(compact(url)),
                "mcp_oauth_updated_at_ms" if has_time else "file mtime",
            )
            if scope in tokens:
                text += MCP_TOKENS
            yield self._row(artifact, "system", text, updated if has_time else mtime, rowid)

    def _plugins(
        self, artifact: Artifact, connection: sqlite3.Connection, errors: list
    ) -> Iterator[Row]:
        """Yield a `plugin installed:` row per entry of each `cursor.plugins.installedIds.` list.

        The rows are at the file's modification time; a `file://` scope is the row's `project_path`.
        A value that is not a JSON list is skipped, one that starts as a list and does not decode is
        an error entry.
        """
        if "ItemTable" not in table_names(connection):
            return
        sql = "select rowid, key, value from ItemTable where substr(key, 1, ?) = ? order by key"
        mtime = file_mtime(artifact)
        for rowid, key, value in connection.execute(sql, (len(PLUGINS_PREFIX), PLUGINS_PREFIX)):
            key = _as_text(key)
            scope = key[len(PLUGINS_PREFIX) :].rsplit("|", 1)[-1]
            entries = None
            if _as_text(value).lstrip().startswith("["):
                entries = _decode(int(rowid), key, value, errors)
            for entry in entries if isinstance(entries, list) else []:
                if not isinstance(entry, dict):
                    continue
                sources = entry.get("sources")
                names = (
                    ",".join(str(source) for source in sources) if isinstance(sources, list) else ""
                )
                text = "plugin installed: id=%s sources=%s scope=%s %s" % (
                    _value(entry.get("id")),
                    names or "-",
                    _field(scope),
                    PLUGINS_SOURCE,
                )
                row = self._row(artifact, "system", text, mtime, int(rowid))
                row.project_path = uri_path(scope) if scope.startswith("file:") else ""
                yield row

    # -- side databases --------------------------------------------------

    def _search(
        self, artifact: Artifact, connection: sqlite3.Connection, opts: Options
    ) -> Iterator[Row]:
        if "conversations" not in table_names(connection):
            return
        projects, models = home_projects(artifact), home_models(artifact)
        bodies = fts_bodies(connection)
        for record in _select(connection, "conversations", SEARCH_COLUMNS):
            session_id = _as_text(record.get("id"))

            def row(text: str, record=record, session_id=session_id) -> Row:
                new_row = self._row(
                    artifact, "system", text, record.get("updated_at"), int(record["_rowid"])
                )
                self._join(new_row, session_id, projects)
                new_row.model = models.get(session_id, "")
                return new_row

            source = _as_text(record.get("source"))
            if source == CLOUD_CACHE:
                yield row(CLOUD_ORIGIN)
            title = compact(_as_text(record.get("title")))
            if title:
                yield row(
                    "conversation index: %s source=%s archived=%s"
                    % (title, _field(source), _field(record.get("is_archived")))
                )
            body = compact(_as_text(bodies.get(record.get("fts_rowid"))))
            if body:
                yield row(
                    "cached conversation text (conversation-search.db, source=%s): %s"
                    % (_field(source), body)
                )

    def _tracking(
        self, artifact: Artifact, connection: sqlite3.Connection, opts: Options
    ) -> Iterator[Row]:
        tables = table_names(connection)
        projects = home_projects(artifact)
        for table, columns in TRACKING_TABLES.items():
            if table not in tables:
                continue
            for record in _select(connection, table, columns):
                text, when, session_id = tracking_text(table, record)
                row = self._row(artifact, "system", text, when, int(record["_rowid"]))
                self._join(row, session_id, projects)
                row.model = _as_text(record.get("model"))
                if table == "scored_commits":
                    row.git_branch = _as_text(record.get("branchName"))
                yield row

    @staticmethod
    def _join(row: Row, session_id: str, projects: dict) -> None:
        row.session_id = session_id
        if session_id in projects:
            row.project_path, row.git_branch = projects[session_id]

    # -- transcript JSONL -------------------------------------------------

    def _transcript(self, artifact: Artifact) -> Iterator[Row]:
        match = TRANSCRIPT_PATTERN.match(artifact.rel)
        session_id = match.group(1) if match else ""
        path, branch = home_projects(artifact).get(session_id, ("", ""))
        errors: list = []

        def row(turn_type: str, text: str, when: str, line: int) -> Row:
            new_row = self._row(artifact, turn_type, text, "", line)
            new_row.timestamp_utc = when
            new_row.session_id, new_row.project_path, new_row.git_branch = session_id, path, branch
            return new_row

        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            role = record.get("role")
            if role not in ("user", "assistant"):
                continue
            content = _object(record.get("message")).get("content")
            if isinstance(content, str):
                content = [{"type": "text", "text": content}]
            for block in content if isinstance(content, list) else []:
                if not isinstance(block, dict):
                    continue
                if block.get("type") == "text" and isinstance(block.get("text"), str):
                    if role == "user":
                        query, when = unwrap_query(block["text"])
                        text = compact(query)
                        if text:
                            yield row("user", text, when, line_number)
                    elif compact(block["text"]):
                        yield row("assistant", compact(block["text"]), "", line_number)
                elif block.get("type") == "tool_use":
                    tool_row = row("tool_use", compact_json(block.get("input")), "", line_number)
                    tool_row.tool_name = str(block.get("name") or "")
                    yield tool_row
        yield from error_rows(self, artifact, errors, session_id, path)

    # -- MCP OAuth attempt files ------------------------------------------

    def _attempt(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the `mcp oauth attempt:` row of an MCP OAuth attempt file.

        The row is at `createdAtMs`, else the file's modification time, and the text says which. The
        client id is an identifier, not a secret.
        """
        rel = artifact.rel
        try:
            with open(artifact.disk_path, "rb") as handle:
                document = json.loads(handle.read().decode("utf-8", errors="replace"))
        except (OSError, ValueError) as error:
            yield self._row(
                artifact, "system", "parser: unreadable %s: %s" % (rel, error_reason(error))
            )
            return
        if not isinstance(document, dict):
            yield self._row(artifact, "system", "parser: %s is not a JSON object" % rel)
            return
        match = ATTEMPT_PATTERN.match(rel)
        source = "mcp-oauth-attempts/" + (match.group(1) if match else "")
        created = document.get("createdAtMs")
        if created in (None, ""):
            created, source = file_mtime(artifact), source + ", file mtime"
        text = "mcp oauth attempt: server=%s client_id=%s workspace=%s (from %s)" % (
            _field(document.get("serverUrl")),
            _field(_object(document.get("clientInformation")).get("client_id")),
            _field(_object(document.get("owner")).get("workspaceId")),
            source,
        )
        yield self._row(artifact, "system", text, created, 1)

    # -- settings files ---------------------------------------------------

    def _settings_file(self, artifact: Artifact) -> Iterator[Row]:
        rel = artifact.rel
        try:
            with open(artifact.disk_path, "rb") as handle:
                document = json.loads(handle.read().decode("utf-8", errors="replace"))
        except (OSError, ValueError) as error:
            yield self._row(
                artifact, "system", "parser: unreadable %s: %s" % (rel, error_reason(error))
            )
            return
        if not isinstance(document, dict):
            yield self._row(artifact, "system", "parser: %s is not a JSON object" % rel)
            return
        if CLI_CONFIG_PATTERN.match(rel):
            settings = cli_config_settings(document)
        else:
            settings = permissions_settings(document)
        mtime = file_mtime(artifact)
        project = artifact.home.original if artifact.home.project else ""
        for setting in settings:
            row = self._row(
                artifact,
                "system",
                compact("permission setting: %s (from %s, file mtime)" % (setting, rel)),
                mtime,
                1,
            )
            row.project_path = project
            yield row


def _field(value) -> str:
    text = _as_text(value)
    return text if text != "" else "-"


def _role_label(role: str) -> str:
    return "system prompt" if role == "system" else "%s message" % (role or "-")


def _value(value) -> str:
    """Return a cloud agent field as row text: `-` when absent or empty, else `setting_value`."""
    return "-" if value in (None, "") else setting_value(value)


def mcp_key_name(key: str, prefix: str) -> tuple[str, str] | None:
    """Return (scope, field) of an `mcpOAuth.global.` or `mcpOAuth.secret.` key, or None.

    The key's suffix is URL-safe base64 (unpadded) of `[<scope>] <field>`; only the key name is
    read.
    """
    suffix = key[len(prefix) :]
    try:
        name = base64.urlsafe_b64decode(suffix + "=" * (-len(suffix) % 4)).decode("utf-8")
    except ValueError:
        return None
    match = MCP_KEY_PATTERN.match(name)
    return (match.group(1), match.group(2)) if match else None


def uri_path(uri: str) -> str:
    r"""Return the percent-decoded path of a `file://` URI.

    A Windows drive path (`file:///c%3A/Users/...`) is returned in the `c:\Users\...` form
    `fsPath` writes.
    """
    path = unquote(urlparse(uri).path)
    if re.match(r"^/[A-Za-z]:", path):
        return path[1:].replace("/", "\\")
    return path


def fts_bodies(connection: sqlite3.Connection) -> dict:
    """Return `conversation_fts_content` `id` -> `c1` (the body; `c0` is the title).

    The FTS5 shadow table is read as a plain table, so the fts5 module is never needed; the result
    is {} when the table or a column is missing.
    """
    if "conversation_fts_content" not in table_names(connection):
        return {}
    have = {info[1] for info in connection.execute("pragma table_info(conversation_fts_content)")}
    if not {"id", "c1"} <= have:
        return {}
    return {
        row[0]: row[1] for row in connection.execute("select id, c1 from conversation_fts_content")
    }


def github_repository(url: str) -> str:
    """Return `<owner>/<repo>`, less a `.git` suffix, when the host of `url` is `github.com`, else "".

    Cursor writes `repoUrl` without a scheme, so a scheme-less value is parsed as a network path;
    the host is compared after parsing, so a path or a longer host that merely contains
    `github.com` does not match.
    """
    parsed = urlparse(url if "://" in url else "//" + url)
    if parsed.hostname != "github.com":
        return ""
    path = parsed.path.strip("/")
    return path[: -len(".git")] if path.endswith(".git") else path


def cloud_text(agent: dict) -> str:
    """Return the `cloud agent:` text of a cloud agent, `-` for an absent value."""
    return (
        "cloud agent: %s repo=%s status=%s workflowStatus=%s archived=%s killed=%s vmStarted=%s %s"
        % (
            compact(agent.get("name")) or "-",
            _value(agent.get("repoUrl")),
            _value(agent.get("status")),
            _value(agent.get("workflowStatus")),
            _value(agent.get("isArchived")),
            _value(agent.get("isKilled")),
            _value(agent.get("hasStartedVm")),
            CLOUD_SOURCE,
        )
    )


def repository_text(repo: dict) -> str:
    """Return the `source host repository:` text of one `githubRepos` element."""
    target = _as_text(repo.get("url")) or _as_text(repo.get("name")) or "-"
    return "source host repository: %s type=%s %s" % (
        target,
        _value(repo.get("type")),
        REPOS_SOURCE,
    )


def pending_text(name: str, extra: dict, review: dict) -> str:
    """Return the `permission prompt pending:` text of a prompt still open at the copy.

    Its `selectedOption` is a preset, not a decision.
    """
    ids = [
        _as_text(option.get("id"))
        for option in review.get("allowlistOptions") or []
        if isinstance(option, dict) and option.get("id") is not None
    ]
    return "permission prompt pending: %s reason=%s options=%s" % (
        _field(name),
        _field(extra.get("blockReason")),
        ",".join(ids) or "-",
    )


def permission_text(extra: dict, review: dict) -> str:
    """Return the `permission decision:` text of an answered prompt.

    ` allowlisted=` with the applied allowlist entries follows when there are any, which marks an
    allow-always decision whatever `selectedOption` says.
    """
    text = "permission decision: %s outcome=%s approval=%s reason=%s" % (
        _field(review.get("selectedOption")),
        _field(extra.get("status")),
        _field(review.get("approvalType")),
        _field(extra.get("blockReason")),
    )
    applied = review.get("appliedAllowlistOptionIds")
    if isinstance(applied, list) and applied:
        names = {}
        for option in review.get("allowlistOptions") or []:
            if isinstance(option, dict) and option.get("id") is not None:
                names[str(option["id"])] = str(option.get("commandName") or option["id"])
        text += " allowlisted=" + ",".join(names.get(str(i), str(i)) for i in applied)
    return text


def standing_texts(state: dict, sandbox: bool) -> Iterator[str]:
    """Yield the `permission setting:` texts of the settings blob's `composerState`.

    There is one per standing-list entry, one per `modes4` entry and one per boolean switch.
    """
    for key in STANDING_LISTS:
        for entry in state.get(key) or [] if isinstance(state.get(key), list) else []:
            yield "permission setting: %s=%s %s" % (key, setting_value(entry), COMPOSER_SOURCE)
    for mode in state.get("modes4") or [] if isinstance(state.get("modes4"), list) else []:
        if not isinstance(mode, dict):
            continue
        flags = " ".join(
            "%s=%s" % (flag, setting_value(mode[flag]) if flag in mode else "-")
            for flag in ("autoRun", "smartModeAutoRun", "fullAutoRun")
        )
        yield "permission setting: run mode %s=%s %s sandbox=%s %s" % (
            _field(mode.get("id")),
            run_mode(mode, sandbox),
            flags,
            setting_value(sandbox),
            COMPOSER_SOURCE,
        )
    for key in SWITCHES:
        if key in state:
            yield "permission setting: %s=%s %s" % (key, setting_value(state[key]), COMPOSER_SOURCE)


def _plain(value):
    """Return a short ItemTable value's JSON value when it decodes, else its text."""
    text = _as_text(value)
    try:
        return json.loads(text)
    except ValueError:
        return text


def last_used(usages: list[dict]):
    """Return the latest `lastUsed` (epoch ms) of a `github-<login>-usages` list, or None."""
    times = [
        usage["lastUsed"] for usage in usages if isinstance(usage.get("lastUsed"), (int, float))
    ]
    return max(times) if times else None


def github_text(login: str, key: str, usages: list[dict]) -> str:
    """Return the `source host: github account` text, the scopes deduplicated from the usages."""
    scopes: list[str] = []
    for usage in usages:
        for scope in usage.get("scopes") or [] if isinstance(usage.get("scopes"), list) else []:
            if str(scope) not in scopes:
                scopes.append(str(scope))
    return "source host: github account %s scopes=%s (from state.vscdb key %s)" % (
        login,
        ",".join(scopes) or "-",
        key,
    )


def _entries(value) -> list:
    return value if isinstance(value, list) else []


def cli_config_settings(document: dict) -> Iterator[str]:
    """Yield `<key>=<value>` per setting of `.cursor/cli-config.json` that governs permissions."""
    if "approvalMode" in document:
        yield "approvalMode=%s" % setting_value(document["approvalMode"])
    permissions = _object(document.get("permissions"))
    for key in ("allow", "deny"):
        for entry in _entries(permissions.get(key)):
            yield "permissions.%s=%s" % (key, setting_value(entry))
    sandbox = _object(document.get("sandbox"))
    if "mode" in sandbox:
        yield "sandbox.mode=%s" % setting_value(sandbox["mode"])


def permissions_settings(document: dict) -> Iterator[str]:
    """`<key>=<value>` per entry of a `permissions.json`."""
    allowlists = _object(document.get("allowlists"))
    for key in ("mcp", "terminal"):
        for entry in _entries(allowlists.get(key)):
            yield "allowlists.%s=%s" % (key, setting_value(entry))
    if "approvalMode" in document:
        yield "approvalMode=%s" % setting_value(document["approvalMode"])
    for key in ("allowInstructions", "blockInstructions"):
        for entry in _entries(document.get(key)):
            yield "%s=%s" % (key, setting_value(entry))


def tracking_text(table: str, record: dict) -> tuple[str, object, str]:
    """(text, time, session id) of an `ai-code-tracking.db` row."""
    fields = {key: _field(value) for key, value in record.items()}
    session_id = _as_text(record.get("conversationId") or record.get("composerId"))
    if table == "ai_code_hashes":
        text = "ai code: %s source=%s model=%s request=%s" % (
            fields.get("fileName", "-"),
            fields.get("source", "-"),
            fields.get("model", "-"),
            fields.get("requestId", "-"),
        )
        return text, record.get("timestamp"), session_id
    if table == "conversation_summaries":
        text = "conversation summary: %s: %s" % (fields.get("title", "-"), fields.get("tldr", "-"))
        return text, record.get("updatedAt"), session_id
    if table == "ai_deleted_files":
        text = "file deleted by agent: %s model=%s" % (
            fields.get("gitPath", "-"),
            fields.get("model", "-"),
        )
        return text, record.get("deletedAt"), session_id
    text = "scored commit: %s branch=%s +%s -%s" % (
        fields.get("commitHash", "-"),
        fields.get("branchName", "-"),
        fields.get("linesAdded", "-"),
        fields.get("linesDeleted", "-"),
    )
    return text, record.get("scoredAt"), ""
