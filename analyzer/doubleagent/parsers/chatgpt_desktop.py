"""ChatGPT desktop app: the account conversation list.

Reads `.codex/sqlite/codex-dev.db` (SQLite in WAL mode, opened through `sqlite_util`),
table `local_thread_catalog`, the rows whose `source_kind` is `chatgpt`: a ChatGPT-mode
conversation leaves no transcript on disk, only the row the app writes at sign-in. The
`vscode` rows are the app's Codex threads, whose rollouts the `codex-cli` parser reads, and
are skipped. The catalog attributes `.codex` to `codex-cli`, so this parser lists that
agent in `reads_agents`.

Columns read, each possibly NULL: `thread_id`, `display_title`, `source_created_at`,
`source_updated_at` (REAL epoch seconds), `conversation_origin`, `host_id` and
`source_kind`. Each conversation yields three `system` rows at `source_updated_at` (else
`source_created_at`): the `account conversation:` row, a `session origin: remote` row and a
`session synced:` row. The conversation's text is not on disk, so these sessions have no
user or assistant turns. A file that is not SQLite, a SQLite error, or a database without
the table is one `parser:` row.

Closed source. Validated against real installs of the desktop app (build 26.1013.40934 on
macOS, build 26.1007.9446.0 on Windows 11) with a synthetic fixture. See
`research/chatgpt-desktop.md`.
"""

from __future__ import annotations

import sqlite3
from collections.abc import Iterator

from ..inputs import Artifact
from ..model import Row
from ..sqlite_util import open_copy, sqlite_problem, table_names
from ..timeutil import to_utc
from .base import Options, Parser

DATABASE_REL = ".codex/sqlite/codex-dev.db"
TABLE = "local_thread_catalog"


def _value(value) -> str:
    """Return a column as row text, `-` for NULL or empty."""
    if value is None or value == "":
        return "-"
    return str(value)


class ChatGptDesktopParser(Parser):
    """Parse the ChatGPT desktop app's list of account conversations."""

    agent = "chatgpt-desktop"
    name = "chatgpt-desktop"
    # The database sits under `.codex`, which the catalog gives to codex-cli.
    reads_agents = ("codex-cli",)
    records_sync = True
    records_origin = True
    has_secondary = False

    def wants(self, artifact: Artifact) -> bool:
        """Return True for `.codex/sqlite/codex-dev.db`."""
        return artifact.rel == DATABASE_REL

    def _system(self, artifact: Artifact, session_id: str, timestamp: str, text: str) -> Row:
        row = self.base_row(artifact)
        row.turn_type = "system"
        row.session_id = session_id
        row.timestamp_utc = timestamp
        row.text = text
        return row

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield three `system` rows per ChatGPT conversation in the catalog table."""
        problem = sqlite_problem(artifact.disk_path)
        if problem:
            yield self._system(artifact, "", "", "parser: " + problem)
            return
        try:
            with open_copy(artifact.disk_path) as connection:
                if TABLE not in table_names(connection):
                    yield self._system(artifact, "", "", "parser: no %s table" % TABLE)
                    return
                records = [
                    dict(row)
                    for row in connection.execute(
                        "select rowid as _rowid, * from %s order by rowid" % TABLE
                    )
                ]
        except sqlite3.DatabaseError as error:
            yield self._system(artifact, "", "", "parser: SQLite error: %s" % error)
            return
        for record in records:
            if record.get("source_kind") != "chatgpt":
                continue
            yield from self._conversation_rows(artifact, record)

    def _conversation_rows(self, artifact: Artifact, record: dict) -> Iterator[Row]:
        thread_id = str(record.get("thread_id") or "")
        created = to_utc(record.get("source_created_at"))
        updated = to_utc(record.get("source_updated_at"))
        timestamp = updated or created
        yield self._system(
            artifact,
            thread_id,
            timestamp,
            "account conversation: %s origin=%s created=%s updated=%s host=%s (from %s %s)"
            % (
                _value(record.get("display_title")),
                _value(record.get("conversation_origin")),
                created or "-",
                updated or "-",
                _value(record.get("host_id")),
                DATABASE_REL,
                TABLE,
            ),
        )
        yield self._system(
            artifact,
            thread_id,
            timestamp,
            "session origin: remote chatgpt.com conversation %s"
            " (listed at sign-in; its transcript is server-side)" % _value(thread_id),
        )
        yield self._system(
            artifact,
            thread_id,
            timestamp,
            "session synced: chatgpt.com holds conversation %s, no local transcript"
            % _value(thread_id),
        )
