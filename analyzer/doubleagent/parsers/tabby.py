"""Parse Tabby server chat threads, pages, completion events, model config and edit histories.

Follows `research/tabby.md`. Validated against source, TabbyML/tabby at 21b2904,
with a synthetic fixture; no real install. Tabby is a self-hosted server, so
its stores sit under the server account's home: the `user` column is that
account, and the person who chatted is named in each thread's `system` row.
Paths start at `.tabby/`, or at the server root itself when the collection
base is a Docker or Podman volume's `_data` (`ROOT`). Files read:

- `ee/db.sqlite` or `ee/dev-db.sqlite`, through `sqlite_util.open_copy`, and
  the `ee/*db.backup-<date>.sqlite` copies made before each migration, a
  secondary source (a file repeating the live database's rows) that can hold
  ephemeral threads since deleted. Only the `COLUMNS` are selected, each when
  the table has it; the secret-bearing columns are never read. Pre-0.25
  `thread_messages` hold separate `*_attachments` columns.
- `events/<date>.json`, one `LogEntry` per line (`user`, `ts`, `event`), a
  secondary source: the server writes each event both here and to
  `user_events`.
- `config.toml`: one `model configured:` row per `[model.<group>.<http|local>]`
  table at the file's time; `api_key` is never read. Needs `tomllib` (3.11).
- The IntelliJ plugin's `options/intellij-tabby-command-history.xml` and the
  VS Code extension's `edit.recentlyCommand` in the editor's `state.vscdb`
  (`reads_agents`), secondary sources with no times.

`session_id` is the thread id, `page:<id>` or the `completion_id`;
`project_path` is the first attached `code[].git_url` (Tabby records no local
directory) or the event's `segments.git_url`. Integrations and their cached
repositories give `source host:` rows. No per-message model, tool call,
thinking or git branch is recorded.
"""

from __future__ import annotations

import json
import re
import sqlite3
import xml.etree.ElementTree as ET
from collections.abc import Iterator
from urllib.parse import urlparse

from ..inputs import Artifact
from ..model import Row, compact
from ..sqlite_util import is_sqlite, open_copy, table_names
from ..timeutil import to_utc
from ..vscode_state import EDITOR_AGENTS, STATE_DATABASE_PATTERN, read_item
from .base import Options, Parser, compact_json, error_rows, file_mtime, iter_jsonl
from .codex import flat_toml, load_toml

# The server root a path starts with: `.tabby/` under a user's home, or
# nothing when the collection base is a Docker or Podman named volume's
# `_data` (the `tabby|*tabby*` volume line), which holds the server root
# itself, since the images set `TABBY_ROOT=/data`.
ROOT = r"(?:\.tabby/)?"
DATABASE_PATTERN = re.compile(r"^%see/(?:dev-)?db\.sqlite$" % ROOT)
BACKUP_PATTERN = re.compile(r"^%see/(?:dev-)?db\.backup-[^/]+\.sqlite$" % ROOT)
EVENTS_PATTERN = re.compile(r"^%sevents/\d{4}-\d{2}-\d{2}\.json$" % ROOT)
CONFIG_PATTERN = re.compile(r"^%sconfig\.toml$" % ROOT)
# The IntelliJ plugin's inline-edit command history, in the IDE's `options/`
# directory where the catalog's `intellij-tabby*.xml` lines collect it.
INTELLIJ_HISTORY_PATTERN = re.compile(
    r"(?:^|/)JetBrains/[^/]+/options/intellij-tabby-command-history\.xml$"
)
INTELLIJ_HISTORY_COMPONENT = "com.tabbyml.intellijtabby.inlineChat.CommandHistory"
# The VS Code extension's global state row in the editor's state.vscdb.
VSCODE_ITEM_KEY = "TabbyML.vscode-tabby"
VSCODE_HISTORY_KEY = "edit.recentlyCommand"
MODEL_GROUPS = ("completion", "chat", "embedding")

# The only columns ever read. No secret-bearing column is listed: of the
# tables holding one, `users` and `integrations` are read without it.
COLUMNS = {
    "users": ("id", "email", "name"),
    "threads": ("id", "is_ephemeral", "user_id", "created_at", "updated_at", "relevant_questions"),
    "thread_messages": (
        "id",
        "thread_id",
        "role",
        "content",
        "code_source_id",
        "attachment",
        "code_attachments",
        "client_code_attachments",
        "doc_attachments",
        "created_at",
    ),
    "user_events": ("id", "user_id", "kind", "created_at", "payload"),
    "pages": ("id", "author_id", "title", "content", "code_source_id", "created_at"),
    "page_sections": ("id", "page_id", "title", "content", "position", "attachment", "created_at"),
    # `integrations.access_token` is never selected.
    "integrations": ("id", "kind", "display_name", "api_base", "created_at"),
    "provided_repositories": ("id", "integration_id", "name", "git_url", "active", "updated_at"),
}
# The host a source-host integration of each kind talks to; self-hosted kinds
# take the host of their `api_base` (integration.rs:79-93).
INTEGRATION_HOSTS = {"github": "github.com", "gitlab": "gitlab.com"}


def _json(value, default):
    if value is None or value == "":
        return default
    if isinstance(value, (bytes, bytearray)):
        value = bytes(value).decode("utf-8", errors="replace")
    if not isinstance(value, str):
        return value
    try:
        return json.loads(value)
    except ValueError:
        return default


def _join(*parts: str) -> str:
    return " | ".join(part for part in parts if part)


def _list(value) -> list:
    return value if isinstance(value, list) else []


def _select(connection: sqlite3.Connection, table: str, order: str) -> Iterator[dict]:
    """Yield the rows of `table`, restricted to the whitelisted columns it has."""
    have = {row[1] for row in connection.execute("pragma table_info(%s)" % table)}
    columns = [column for column in COLUMNS[table] if column in have]
    if not columns:
        return
    order_columns = [column for column in order.split(",") if column.strip() in have]
    sql = "select %s from %s" % (", ".join(columns), table)
    if order_columns:
        sql += " order by " + ", ".join(order_columns)
    for row in connection.execute(sql):
        yield dict(zip(columns, row, strict=False))


def _location(filepath, start_line) -> str:
    path_text = str(filepath or "")
    return (
        "%s:%s" % (path_text, start_line)
        if path_text and start_line not in (None, "")
        else path_text
    )


def attachment_parts(attachment: dict) -> list[str]:
    """Return short descriptions of what was attached to a message."""
    parts: list[str] = []
    for code in _list(attachment.get("code")):
        if isinstance(code, dict):
            repository = str(code.get("git_url") or "")
            if code.get("commit"):
                repository += "@" + str(code["commit"])
            parts.append(
                _join(
                    "code", repository, _location(code.get("filepath"), code.get("start_line"))
                ).replace(" | ", " ")
            )
    for code in _list(attachment.get("client_code")):
        if isinstance(code, dict):
            parts.append(
                ("client_code " + _location(code.get("filepath"), code.get("start_line"))).strip()
            )
    for doc in _list(attachment.get("doc")):
        if isinstance(doc, dict):
            label = doc.get("title") or doc.get("sha") or ""
            link = doc.get("link") or doc.get("page_link") or ""
            parts.append(_join("doc", str(label), str(link)).replace(" | ", " "))
    file_list = attachment.get("code_file_list")
    if isinstance(file_list, dict) and isinstance(file_list.get("file_list"), list):
        parts.append("code_file_list %d files" % len(file_list["file_list"]))
    return parts


def message_attachment(message: dict) -> dict:
    """Return a thread message's attachment, from the pre-0.25 columns when it lacks one."""
    attachment = _json(message.get("attachment"), None)
    if isinstance(attachment, dict):
        return attachment
    # Pre-0.25 databases (threads.rs:34-38): separate columns, no `attachment`.
    return {
        "code": _json(message.get("code_attachments"), None),
        "client_code": _json(message.get("client_code_attachments"), None),
        "doc": _json(message.get("doc_attachments"), None),
    }


def first_git_url(attachment: dict) -> str:
    """Return the first `code[].git_url` of an attachment, else ""."""
    for code in _list(attachment.get("code")):
        if isinstance(code, dict) and code.get("git_url"):
            return str(code["git_url"])
    return ""


def _model_text(group: str, kind: str, body: dict) -> str:
    """Return the `model configured:` text of one `[model.<group>.<kind>]` table.

    The `http` form gives `model_name`, `kind`, `api_endpoint` and
    `supported_models`, the `local` form `model_id`. `api_key` is never read.
    """
    if kind == "http":
        supported = body.get("supported_models")
        detail = " ".join(
            part
            for part in (
                "kind=%s" % body["kind"] if body.get("kind") else "",
                "endpoint=%s" % body["api_endpoint"] if body.get("api_endpoint") else "",
                "supported_models=%s"
                % ",".join(str(supported_model) for supported_model in supported)
                if isinstance(supported, list) and supported
                else "",
            )
            if part
        )
        model = body.get("model_name")
    else:
        detail = ""
        model = body.get("model_id")
    return "model configured: %s %s via %s%s (from config.toml [model.%s.%s], file mtime)" % (
        group,
        model or "-",
        kind,
        " " + detail if detail else "",
        group,
        kind,
    )


def _command_text(item) -> str:
    """Return one `edit.recentlyCommand` entry as row text, its context files' URIs appended.

    An entry is `JSON.stringify({command, context})` or a bare command string
    (quickPick.ts:322-351).
    """
    command = _json(item, None) if isinstance(item, str) else item
    if isinstance(command, dict) and command.get("command"):
        text = str(command["command"])
        files = [
            str(context.get("uri"))
            for context in _list(command.get("context"))
            if isinstance(context, dict) and context.get("uri")
        ]
        return "%s [context: %s]" % (text, "; ".join(files)) if files else text
    if item in (None, ""):
        return ""
    return item if isinstance(item, str) else compact_json(item)


class TabbyParser(Parser):
    """Parse Tabby's databases, event logs, config and editor command histories."""

    agent = "tabby"
    name = "tabby"
    has_secondary = True
    # The VS Code extension's command history is a row of the editor's
    # state.vscdb, which the catalog attributes to the editor.
    reads_agents = EDITOR_AGENTS

    def _own(self, artifact: Artifact) -> bool:
        # A stand-in artifact with no agent (a wants() test) counts as own.
        return getattr(artifact, "agent", self.agent) == self.agent

    def wants(self, artifact: Artifact) -> bool:
        """Return True for a Tabby database, event log, config or history, or an editor's state."""
        rel = artifact.rel
        if not self._own(artifact):
            return bool(STATE_DATABASE_PATTERN.search(rel))
        return bool(
            DATABASE_PATTERN.match(rel)
            or BACKUP_PATTERN.match(rel)
            or EVENTS_PATTERN.match(rel)
            or CONFIG_PATTERN.match(rel)
            or INTELLIJ_HISTORY_PATTERN.search(rel)
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for a backup database, an event log or a command history."""
        rel = artifact.rel
        if not self._own(artifact):
            return bool(STATE_DATABASE_PATTERN.search(rel))
        return bool(
            BACKUP_PATTERN.match(rel)
            or EVENTS_PATTERN.match(rel)
            or INTELLIJ_HISTORY_PATTERN.search(rel)
        )

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Tabby file."""
        rel = artifact.rel
        if not self._own(artifact):
            yield from self._parse_vscode_state(artifact, opts)
        elif EVENTS_PATTERN.match(rel):
            yield from self._parse_events(artifact, opts)
        elif CONFIG_PATTERN.match(rel):
            yield from self._parse_config(artifact, opts)
        elif INTELLIJ_HISTORY_PATTERN.search(rel):
            yield from self._parse_intellij_history(artifact, opts)
        else:
            yield from self._parse_database(artifact, opts, bool(BACKUP_PATTERN.match(rel)))

    def _row(
        self,
        artifact: Artifact,
        line: int,
        session_id: str = "",
        project_path: str = "",
        timestamp: str = "",
        turn_type: str = "system",
        text: str = "",
        opts: Options | None = None,
    ) -> Row:
        row = self.base_row(artifact)
        row.source_line = line
        row.session_id = session_id
        row.project_path = project_path
        row.timestamp_utc = timestamp
        row.turn_type = turn_type
        row.text = compact(text)
        return row

    # -- db.sqlite and its backups -------------------------------------------------------
    def _parse_database(self, artifact: Artifact, opts: Options, backup: bool) -> Iterator[Row]:
        if not is_sqlite(artifact.disk_path):
            yield self._row(artifact, 0, text="parser: not a SQLite database")
            return
        if backup:
            yield self._row(
                artifact,
                0,
                opts=opts,
                text=(
                    "backup database %s copied before a schema migration; it can hold threads and "
                    "events since deleted from the live database" % artifact.rel.rsplit("/", 1)[-1]
                ),
            )
        try:
            with open_copy(artifact.disk_path) as connection:
                tables = table_names(connection)
                users: dict[str, str] = {}
                if "users" in tables:
                    for user in _select(connection, "users", "id"):
                        label = str(user.get("email") or "")
                        if user.get("name"):
                            label = (
                                "%s (%s)" % (label, user["name"]) if label else str(user["name"])
                            )
                        users[str(user.get("id"))] = label
                if "threads" in tables:
                    yield from self._threads(artifact, connection, tables, users, opts)
                if "pages" in tables:
                    yield from self._pages(artifact, connection, tables, users, opts)
                if "user_events" in tables:
                    yield from self._user_events(artifact, connection, users, opts)
                if "integrations" in tables:
                    yield from self._integrations(artifact, connection, tables, opts)
        except sqlite3.DatabaseError as error:
            yield self._row(artifact, 0, text="parser: SQLite error: %s" % error)

    def _threads(
        self,
        artifact: Artifact,
        connection: sqlite3.Connection,
        tables: set,
        users: dict[str, str],
        opts: Options,
    ) -> Iterator[Row]:
        messages: dict[str, list[dict]] = {}
        if "thread_messages" in tables:
            for message in _select(connection, "thread_messages", "thread_id,created_at,id"):
                messages.setdefault(str(message.get("thread_id")), []).append(message)
        threads = list(_select(connection, "threads", "created_at,id"))
        known = {str(thread.get("id")) for thread in threads}
        # Messages whose thread row is gone still get parsed, under their thread id.
        threads += [{"id": thread_id} for thread_id in messages if thread_id not in known]
        for thread in threads:
            thread_id = str(thread.get("id"))
            thread_messages = messages.get(thread_id, [])
            attachments = [message_attachment(message) for message in thread_messages]
            project = next(
                (url for url in (first_git_url(attachment) for attachment in attachments) if url),
                "",
            )
            questions = [
                str(question) for question in _list(_json(thread.get("relevant_questions"), None))
            ]
            owner = users.get(str(thread.get("user_id")), "")
            yield self._row(
                artifact,
                int(thread["id"]) if str(thread.get("id")).isdigit() else 0,
                thread_id,
                project,
                to_utc(thread.get("created_at")),
                opts=opts,
                text=_join(
                    "thread start",
                    "owner %s" % owner
                    if owner
                    else (
                        "owner user_id=%s" % thread["user_id"]
                        if thread.get("user_id") is not None
                        else ""
                    ),
                    "ephemeral" if thread.get("is_ephemeral") else "",
                    "thread row missing" if "user_id" not in thread else "",
                    "relevant questions: " + "; ".join(questions) if questions else "",
                ),
            )
            for message, attached in zip(thread_messages, attachments, strict=False):
                role = str(message.get("role") or "")
                text = str(message.get("content") or "")
                parts = attachment_parts(attached)
                if message.get("code_source_id"):
                    parts.insert(0, "code_source %s" % message["code_source_id"])
                if parts:
                    text = "%s [attachments: %s]" % (text, "; ".join(parts))
                if role not in ("user", "assistant"):
                    text = "%s: %s" % (role, text)
                yield self._row(
                    artifact,
                    message.get("id") or 0,
                    thread_id,
                    project,
                    to_utc(message.get("created_at")),
                    role if role in ("user", "assistant") else "system",
                    text,
                    opts,
                )

    def _user_events(
        self,
        artifact: Artifact,
        connection: sqlite3.Connection,
        users: dict[str, str],
        opts: Options,
    ) -> Iterator[Row]:
        for user_event in _select(connection, "user_events", "created_at,id"):
            line = user_event.get("id") or 0
            timestamp = to_utc(user_event.get("created_at"))
            owner = "user=%s" % (
                users.get(str(user_event.get("user_id")), "") or user_event.get("user_id")
            )
            event = _json(user_event.get("payload"), None)
            if not isinstance(event, dict):
                yield self._row(
                    artifact,
                    line,
                    timestamp=timestamp,
                    opts=opts,
                    text="parser: user_event %s payload did not parse" % user_event.get("id"),
                )
                continue
            yield from self._event_rows(
                artifact, line, timestamp, owner, event, opts, str(user_event.get("kind") or "")
            )

    def _pages(
        self,
        artifact: Artifact,
        connection: sqlite3.Connection,
        tables: set,
        users: dict[str, str],
        opts: Options,
    ) -> Iterator[Row]:
        """Yield one `system` row per AI-written page, then its content and sections.

        The content and each section, in position order, are `assistant`
        rows (page.rs:92-150).
        """
        sections: dict[str, list[dict]] = {}
        if "page_sections" in tables:
            for section in _select(connection, "page_sections", "page_id,position,id"):
                sections.setdefault(str(section.get("page_id")), []).append(section)
        pages = list(_select(connection, "pages", "created_at,id"))
        known = {str(page.get("id")) for page in pages}
        # Sections whose page row is gone still get parsed, under their page id.
        pages += [{"id": page_id} for page_id in sections if page_id not in known]
        for page in pages:
            page_id = str(page.get("id"))
            line = int(page_id) if page_id.isdigit() else 0
            session_id = "page:%s" % page_id
            page_sections = sections.get(page_id, [])
            attachments = [_json(section.get("attachment"), {}) for section in page_sections]
            attachments = [
                attachment if isinstance(attachment, dict) else {} for attachment in attachments
            ]
            project = next(
                (url for url in (first_git_url(attachment) for attachment in attachments) if url),
                "",
            )
            timestamp = to_utc(page.get("created_at"))
            owner = users.get(str(page.get("author_id")), "")
            if owner:
                author = "author %s" % owner
            elif page.get("author_id") is not None:
                author = "author user_id=%s" % page["author_id"]
            else:
                author = ""
            yield self._row(
                artifact,
                line,
                session_id,
                project,
                timestamp,
                opts=opts,
                text=_join(
                    "page start",
                    "title %s" % page["title"] if page.get("title") else "",
                    author,
                    "code_source %s" % page["code_source_id"] if page.get("code_source_id") else "",
                    "page row missing" if "author_id" not in page else "",
                ),
            )
            if page.get("content"):
                yield self._row(
                    artifact,
                    line,
                    session_id,
                    project,
                    timestamp,
                    "assistant",
                    str(page["content"]),
                    opts,
                )
            for section, attached in zip(page_sections, attachments, strict=False):
                text = "\n\n".join(
                    str(part)
                    for part in (section.get("title"), section.get("content"))
                    if part not in (None, "")
                )
                parts = attachment_parts(attached)
                if parts:
                    text = "%s [attachments: %s]" % (text, "; ".join(parts))
                yield self._row(
                    artifact,
                    section.get("id") or 0,
                    session_id,
                    project,
                    to_utc(section.get("created_at")) or timestamp,
                    "assistant",
                    text,
                    opts,
                )

    def _integrations(
        self, artifact: Artifact, connection: sqlite3.Connection, tables: set, opts: Options
    ) -> Iterator[Row]:
        """Yield the source-host integrations and the repositories cached from them.

        The tables are those of schema.sql:142-165; the token column is never
        selected.
        """
        names: dict[str, tuple[str, str]] = {}
        for integration in _select(connection, "integrations", "created_at,id"):
            kind = str(integration.get("kind") or "")
            api_base = str(integration.get("api_base") or "")
            host = INTEGRATION_HOSTS.get(kind) or urlparse(api_base).hostname or "-"
            name = str(integration.get("display_name") or "")
            names[str(integration.get("id"))] = (name, kind)
            yield self._row(
                artifact,
                integration.get("id") or 0,
                timestamp=to_utc(integration.get("created_at")),
                opts=opts,
                text="source host: %s integration %s kind=%s api_base=%s (account not recorded; "
                "from integrations table, created_at)"
                % (host, name or "-", kind or "-", api_base or "-"),
            )
        if "provided_repositories" not in tables:
            return
        for repository in _select(connection, "provided_repositories", "integration_id,id"):
            name, kind = names.get(str(repository.get("integration_id")), ("", ""))
            yield self._row(
                artifact,
                repository.get("id") or 0,
                timestamp=to_utc(repository.get("updated_at")),
                opts=opts,
                text="source host repository: %s type=%s name=%s active=%s (integration %s; "
                "from provided_repositories table, updated_at)"
                % (
                    repository.get("git_url") or "-",
                    kind or "-",
                    repository.get("name") or "-",
                    "yes" if repository.get("active") else "no",
                    name or "integration_id=%s" % repository.get("integration_id"),
                ),
            )

    # -- config.toml ---------------------------------------------------------------------
    def _parse_config(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield one `system` row per configured model (config.rs:259-342), at the file's mtime.

        Only the model tables' names, kinds and endpoints are read; `api_key`
        never reaches a row.
        """
        timestamp = file_mtime(artifact)
        # The file is read before the interpreter check, so an unreadable one is reported
        # the same on every Python version.
        try:
            text = artifact.disk_path.read_text(encoding="utf-8", errors="replace")
        except OSError as error:
            yield self._row(
                artifact,
                0,
                timestamp=timestamp,
                opts=opts,
                text="parser: config.toml could not be read: %s" % (error.strerror or error),
            )
            return
        if load_toml is flat_toml:
            # flat_toml reads only the top-level keys; the model tables need tomllib.
            yield self._row(
                artifact,
                0,
                timestamp=timestamp,
                opts=opts,
                text="parser: config.toml model tables not read: Python 3.11 or later needed",
            )
            return
        try:
            data = load_toml(text)
        except ValueError as error:
            # tomllib's message names a line and column, never the value.
            yield self._row(
                artifact,
                0,
                timestamp=timestamp,
                opts=opts,
                text="parser: config.toml is not valid TOML (%s)" % error,
            )
            return
        models = data.get("model") if isinstance(data, dict) else None
        if not isinstance(models, dict):
            return
        for group in MODEL_GROUPS:
            config = models.get(group)
            if not isinstance(config, dict):
                continue
            for kind in ("http", "local"):
                body = config.get(kind)
                if isinstance(body, dict):
                    yield self._row(
                        artifact,
                        0,
                        timestamp=timestamp,
                        opts=opts,
                        text=_model_text(group, kind, body),
                    )

    # -- editor command histories ----------------------------------------------------------
    def _history_rows(
        self, artifact: Artifact, source: str, commands: list[str], opts: Options
    ) -> Iterator[Row]:
        """Yield a leading `system` row, then one `user` row per command, newest first.

        The rows have no timestamp: the histories record none.
        """
        yield self._row(
            artifact,
            0,
            opts=opts,
            text="inline edit command history: %d commands, newest first (from %s, no times)"
            % (len(commands), source),
        )
        for index, command in enumerate(commands, 1):
            yield self._row(artifact, index, turn_type="user", text=command, opts=opts)

    def _parse_intellij_history(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        try:
            root = ET.parse(artifact.disk_path).getroot()
        except (ET.ParseError, OSError) as error:
            yield self._row(
                artifact, 0, opts=opts, text="parser: not a readable XML document (%s)" % error
            )
            return
        commands: list[str] = []
        for component in root.iter("component"):
            if component.get("name") != INTELLIJ_HISTORY_COMPONENT:
                continue
            for option in component.iter("option"):
                if option.get("name") == "history":
                    commands += [
                        str(element.get("value"))
                        for element in option.iter()
                        if element is not option and element.get("value") is not None
                    ]
        if commands:
            yield from self._history_rows(
                artifact, "intellij-tabby-command-history.xml", commands, opts
            )

    def _parse_vscode_state(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        state, problems = read_item(artifact.disk_path, VSCODE_ITEM_KEY)
        if isinstance(state, dict):
            history = state.get(VSCODE_HISTORY_KEY)
            commands = [
                command for command in (_command_text(entry) for entry in _list(history)) if command
            ]
            if commands:
                source = "state.vscdb %s %s" % (VSCODE_ITEM_KEY, VSCODE_HISTORY_KEY)
                yield from self._history_rows(artifact, source, commands, opts)
        elif state is not None:
            problems = problems or ["%s value is not a JSON object" % VSCODE_ITEM_KEY]
        for problem in problems:
            # Never echo the value: the same row holds the endpoint tokens.
            yield self._row(artifact, 0, opts=opts, text="parser: " + problem.split(" (", 1)[0])

    # -- events/YYYY-MM-DD.json ----------------------------------------------------------
    def _parse_events(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            user = record.get("user")
            owner = "user=%s" % user if user not in (None, "") else ""
            event = record.get("event")
            if not isinstance(event, dict):
                yield self._row(
                    artifact,
                    line_number,
                    timestamp=to_utc(record.get("ts")),
                    opts=opts,
                    text="unknown log entry: " + compact_json(record),
                )
                continue
            yield from self._event_rows(
                artifact, line_number, to_utc(record.get("ts")), owner, event, opts
            )
        yield from error_rows(self, artifact, errors)

    # -- one Event, from either store -----------------------------------------------------
    def _event_rows(
        self,
        artifact: Artifact,
        line: int,
        timestamp: str,
        owner: str,
        event: dict,
        opts: Options,
        kind: str = "",
    ) -> Iterator[Row]:
        if len(event) != 1:
            yield self._row(
                artifact,
                line,
                timestamp=timestamp,
                opts=opts,
                text=_join(kind or "event", owner, compact_json(event)),
            )
            return
        tag, body = next(iter(event.items()))
        body = body if isinstance(body, dict) else {}
        completion_id = str(body.get("completion_id") or "")
        if tag == "completion":
            seg = body.get("segments")
            if not isinstance(seg, dict):
                seg = {}
            project = str(seg.get("git_url") or "")
            yield self._row(
                artifact,
                line,
                completion_id,
                project,
                timestamp,
                opts=opts,
                text=_join(
                    "completion",
                    owner,
                    "language=%s" % body["language"] if body.get("language") else "",
                    "file=%s" % seg["filepath"] if seg.get("filepath") else "",
                    "client=%s" % body["user_agent"] if body.get("user_agent") else "",
                ),
            )
            yield self._row(
                artifact,
                line,
                completion_id,
                project,
                timestamp,
                "user",
                str(body.get("prompt") or ""),
                opts,
            )
            for choice in _list(body.get("choices")):
                if isinstance(choice, dict):
                    yield self._row(
                        artifact,
                        line,
                        completion_id,
                        project,
                        timestamp,
                        "assistant",
                        str(choice.get("text") or ""),
                        opts,
                    )
        elif tag in ("view", "select", "dismiss"):
            yield self._row(
                artifact,
                line,
                completion_id,
                timestamp=timestamp,
                opts=opts,
                text=_join(
                    "%s choice %s" % (tag, body.get("choice_index", "")),
                    owner,
                    "kind=%s" % body["kind"] if body.get("kind") else "",
                    "view=%s" % body["view_id"] if body.get("view_id") else "",
                    "elapsed=%sms" % body["elapsed"] if body.get("elapsed") is not None else "",
                ),
            )
        else:
            yield self._row(
                artifact,
                line,
                completion_id,
                timestamp=timestamp,
                opts=opts,
                text=_join(str(tag), owner, compact_json(body) if body else ""),
            )
