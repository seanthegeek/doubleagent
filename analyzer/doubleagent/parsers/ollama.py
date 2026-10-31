"""Ollama desktop chats, REPL prompt history and launcher auto-mode settings.

Reads, relative to the home:

* `Library/Application Support/Ollama/db.sqlite` and `AppData/Local/Ollama/db.sqlite`, the
  desktop app's database (schema versions 16 to 19): `messages` (`id`, `chat_id`, `role`,
  `content`, `thinking`, `model_name`, `created_at`, `tool_result`), `tool_calls` (`id`,
  `message_id`, `function_name`, `function_arguments`) and `attachments` (`message_id`,
  `filename`, and the byte length of `data`, computed in SQL so the contents are never read).
  `created_at` is go-sqlite3's text form of a Go `time.Time`, local time with an offset.
  The `users` and `settings` tables are never read.
* `.ollama/history`, the `ollama run` REPL's readline history, one entry per line with no
  time, session or model; a line that is not UTF-8 is a bad line.
* `.ollama/config.json` (`integrations.<name>.automode`) and Claude Desktop's third-party
  profile `Claude-3p/configLibrary/00000000-0000-4000-8000-000000000114.json` that Ollama's
  launcher writes (`autoModeEnabled`), as `permission setting:` rows at the file's mtime.

There is no call id: the n-th `tool` message after an assistant message pairs with that
message's n-th call, and `tool_use_id` is `<chat_id>:<tool_calls.id>`; an extra `tool`
message takes the last call's name and no id, as the app's reload does. `session_id` is the
chat id; `model_name` gives the model rows. The app asks before no tool call, so there are no
per-call permission rows.

Validated against source, ollama/ollama at v0.35.1 (b0c1ca4) and, for the settings, at
678722224b, with a synthetic fixture; no real `db.sqlite`. See `research/ollama.md`.
"""

from __future__ import annotations

import json
import re
import sqlite3
from collections.abc import Iterator

from ..inputs import Artifact
from ..model import Row, compact
from ..sqlite_util import is_sqlite, open_copy, table_names
from ..textlines import ASCII_WHITESPACE
from ..timeutil import to_utc
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_mtime,
    line_text,
    setting_value,
    track_change,
)

DATABASE_PATTERN = re.compile(
    r"^(?:Library/Application Support/Ollama|AppData/Local/Ollama)/db\.sqlite$"
)
HISTORY_PATTERN = re.compile(r"^\.ollama/history$")
CONFIG_PATTERN = re.compile(r"^\.ollama/config\.json$")
# The profile id is Ollama's own (claudeDesktopProfileID).
PROFILE_PATTERN = re.compile(
    r"^(?:Library/Application Support|AppData/Local)/Claude-3p/configLibrary/"
    r"00000000-0000-4000-8000-000000000114\.json$"
)


def call_text(arguments) -> str:
    """Return the `url` or `query` argument of a desktop tool call, else the arguments as stored."""
    try:
        args = json.loads(arguments) if isinstance(arguments, str) else arguments
    except ValueError:
        return str(arguments or "")
    if isinstance(args, dict):
        for key in ("url", "query"):
            value = args.get(key)
            if isinstance(value, str) and value:
                return value
        return compact_json(args)
    return str(arguments or "") if isinstance(arguments, str) else compact_json(args)


def _result_fallback(value) -> str:
    """Return a `tool` message's `tool_result` JSON as compact text, `""` when empty."""
    if value is None or value == "" or value == "null":
        return ""
    try:
        return compact_json(json.loads(value))
    except (TypeError, ValueError):
        return str(value)


class OllamaParser(Parser):
    """Parse the Ollama desktop database, the REPL history and the launcher settings."""

    agent = "ollama"
    name = "ollama"
    # The Claude Desktop profile Ollama writes is under a claude-desktop base.
    reads_agents = ("claude-desktop",)

    def wants(self, artifact: Artifact) -> bool:
        """Return True for the Ollama files this parser reads."""
        rel = artifact.rel
        return any(
            pattern.match(rel)
            for pattern in (DATABASE_PATTERN, HISTORY_PATTERN, CONFIG_PATTERN, PROFILE_PATTERN)
        )

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Ollama file."""
        if DATABASE_PATTERN.match(artifact.rel):
            yield from self._parse_database(artifact, opts)
        elif CONFIG_PATTERN.match(artifact.rel):
            yield from self._parse_config(artifact)
        elif PROFILE_PATTERN.match(artifact.rel):
            yield from self._parse_profile(artifact)
        else:
            yield from self._parse_history(artifact)

    def _row(self, artifact: Artifact, line: int, session_id: str = "", timestamp: str = "") -> Row:
        row = self.base_row(artifact)
        row.source_line = line
        row.session_id = session_id
        row.timestamp_utc = timestamp
        return row

    def _system(self, artifact: Artifact, text: str) -> Row:
        row = self._row(artifact, 0)
        row.turn_type = "system"
        row.text = text
        return row

    # -- db.sqlite ------------------------------------------------------------------------
    def _parse_database(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        if not is_sqlite(artifact.disk_path):
            yield self._system(artifact, "parser: not a SQLite database")
            return
        try:
            with open_copy(artifact.disk_path) as connection:
                rows = list(self._database_rows(connection, artifact, opts))
        except sqlite3.DatabaseError as error:
            yield self._system(artifact, "parser: SQLite error: %s" % error)
            return
        yield from rows

    def _database_rows(
        self, connection: sqlite3.Connection, artifact: Artifact, opts: Options
    ) -> Iterator[Row]:
        """Yield the rows of `db.sqlite`, ordered by `chat_id` then `messages.id`."""
        tables = table_names(connection)
        if "messages" not in tables:
            return
        calls: dict[int, list[dict]] = {}
        if "tool_calls" in tables:
            for call in connection.execute(
                "select id, message_id, function_name, function_arguments"
                " from tool_calls order by message_id, id"
            ):
                calls.setdefault(call["message_id"], []).append(dict(call))
        files: dict[int, list[tuple[str, int]]] = {}
        if "attachments" in tables:
            for attachment in connection.execute(
                "select message_id, filename, length(data) as n from attachments"
                " order by message_id, id"
            ):
                files.setdefault(attachment["message_id"], []).append(
                    (attachment["filename"] or "", attachment["n"] or 0)
                )

        chat = None
        seen_model: dict[str, str] = {}
        pending: list[dict] = []  # calls of the last assistant message with calls
        used = 0
        for message in connection.execute("select * from messages order by chat_id, id"):
            message = dict(message)
            message_id = message.get("id") or 0
            session_id = str(message.get("chat_id") or "")
            if session_id != chat:
                chat, pending, used = session_id, [], 0
            role = str(message.get("role") or "")
            timestamp = to_utc(message.get("created_at"))
            content = message.get("content") or ""
            model = message.get("model_name") or ""

            if role == "user":
                row = self._row(artifact, message_id, session_id, timestamp)
                row.turn_type = "user"
                parts = [content] + [
                    "[attachment: %s, %d bytes]" % (filename, size)
                    for filename, size in files.get(message_id, [])
                ]
                row.text = compact("\n".join(part for part in parts if part))
                yield row
            elif role == "assistant":
                thinking = message.get("thinking") or ""
                if content or thinking or calls.get(message_id):
                    text = track_change("model", seen_model, session_id, model)
                    if text:
                        row = self._row(artifact, message_id, session_id, timestamp)
                        row.turn_type = "system"
                        row.model = model
                        row.text = text
                        yield row
                if thinking and opts.include_thinking:
                    row = self._row(artifact, message_id, session_id, timestamp)
                    row.turn_type = "thinking"
                    row.model = model
                    row.text = compact(thinking)
                    yield row
                if content:
                    row = self._row(artifact, message_id, session_id, timestamp)
                    row.turn_type = "assistant"
                    row.model = model
                    row.text = compact(content)
                    yield row
                message_calls = calls.get(message_id, [])
                if message_calls:
                    pending, used = message_calls, 0
                for call in message_calls:
                    row = self._row(artifact, call["id"], session_id, timestamp)
                    row.turn_type = "tool_use"
                    row.model = model
                    row.tool_name = call["function_name"] or ""
                    row.tool_use_id = "%s:%s" % (session_id, call["id"])
                    row.text = compact(call_text(call["function_arguments"]))
                    yield row
            elif role == "tool":
                row = self._row(artifact, message_id, session_id, timestamp)
                row.turn_type = "tool_result"
                if used < len(pending):
                    call = pending[used]
                    row.tool_name = call["function_name"] or ""
                    row.tool_use_id = "%s:%s" % (session_id, call["id"])
                elif pending:
                    row.tool_name = pending[-1]["function_name"] or ""
                used += 1
                row.text = compact(content or _result_fallback(message.get("tool_result")))
                yield row
            else:
                row = self._row(artifact, message_id, session_id, timestamp)
                row.turn_type = "system"
                row.text = compact(content if role == "system" else "%s: %s" % (role, content))
                yield row

    # -- .ollama/history ------------------------------------------------------------------
    def _parse_history(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `user` or `slash command:` row per line of `.ollama/history`."""
        errors: list = []
        with open(artifact.disk_path, "rb") as handle:
            for line_number, raw in enumerate(handle, 1):
                if not raw.strip():
                    continue
                try:
                    line = raw.decode("utf-8").strip(ASCII_WHITESPACE)
                except UnicodeDecodeError as error:
                    errors.append((line_number, "unparseable line (%s)" % error, line_text(raw)))
                    continue
                if not line:
                    continue
                row = self._row(artifact, line_number)
                if line.startswith("/"):
                    row.turn_type = "system"
                    row.text = "slash command: " + line
                else:
                    row.turn_type = "user"
                    row.text = line
                yield row
        yield from error_rows(self, artifact, errors)

    # -- standing permissions -------------------------------------------------------------
    def _read_object(self, artifact: Artifact) -> tuple[dict | None, str]:
        """Return (JSON object, problem) of a settings file, the problem a `parser:` text."""
        try:
            document = json.loads(artifact.disk_path.read_text(encoding="utf-8"))
        except (OSError, UnicodeDecodeError, ValueError) as error:
            return None, "parser: %s is not readable JSON (%s)" % (artifact.rel, error)
        if not isinstance(document, dict):
            return None, "parser: %s is not a JSON object" % artifact.rel
        return document, ""

    def _setting(self, artifact: Artifact, timestamp: str, text: str) -> Row:
        row = self._row(artifact, 0, "", timestamp)
        row.turn_type = "system"
        row.text = text
        return row

    def _parse_config(self, artifact: Artifact) -> Iterator[Row]:
        document, problem = self._read_object(artifact)
        if document is None:
            yield self._system(artifact, problem)
            return
        timestamp = file_mtime(artifact)
        integrations = document.get("integrations")
        if not isinstance(integrations, dict):
            return
        for name, entry in integrations.items():
            if isinstance(entry, dict) and "automode" in entry:
                yield self._setting(
                    artifact,
                    timestamp,
                    "permission setting: integrations.%s.automode=%s (from %s, file mtime)"
                    % (name, setting_value(entry["automode"]), artifact.rel),
                )

    def _parse_profile(self, artifact: Artifact) -> Iterator[Row]:
        document, problem = self._read_object(artifact)
        if document is None:
            yield self._system(artifact, problem)
            return
        if "autoModeEnabled" in document:
            yield self._setting(
                artifact,
                file_mtime(artifact),
                "permission setting: autoModeEnabled=%s (set by Ollama's Claude Desktop"
                " launcher, from %s, file mtime)"
                % (setting_value(document["autoModeEnabled"]), artifact.rel),
            )
