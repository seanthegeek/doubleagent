"""Build the cursor package's database whose tables are spelled in two cases.

Run it from any directory:

    python3 -I internal/parsers/cursor/testdata/table_case.py

table_case.db is written by SQLite from the SQL below. table_case_respelled.db is the same
database with every table renamed to its name with the case of each letter swapped, which SQL
matches as the same table. Pages are 512 bytes to keep the files small. No test runs this
script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

TABLE_CASE = """
PRAGMA page_size = 512;
CREATE TABLE conversations (fts_rowid INTEGER PRIMARY KEY, source TEXT NOT NULL
  CHECK (source IN ('local', 'cloud-cache')), scope TEXT, id TEXT, title TEXT, branches TEXT,
  updated_at INTEGER, is_archived INTEGER, root_fingerprint TEXT, cache_fingerprint TEXT);
CREATE TABLE conversation_fts_content (id INTEGER PRIMARY KEY, c0, c1, c2);
INSERT INTO conversations VALUES (1, 'local', NULL, 'c1', 'Fix the tests', NULL, 1767323045000, 0, NULL, NULL);
INSERT INTO conversation_fts_content VALUES (1, 'Fix the tests', 'the body of the chat', '');
"""


def build(path: Path, respell: bool) -> None:
    """Write the database at path, its tables respelled when respell is set."""
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(TABLE_CASE)
        if respell:
            names = [
                row[0]
                for row in connection.execute(
                    "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite%'"
                )
            ]
            for name in names:
                connection.execute(f'ALTER TABLE "{name}" RENAME TO "{name}_respelling"')
                connection.execute(f'ALTER TABLE "{name}_respelling" RENAME TO "{name.swapcase()}"')
        connection.commit()


def main() -> None:
    """Rewrite both databases in this directory."""
    build(HERE / "table_case.db", respell=False)
    build(HERE / "table_case_respelled.db", respell=True)


if __name__ == "__main__":
    main()
