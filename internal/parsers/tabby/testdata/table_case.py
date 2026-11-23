"""Build the tabby package's database whose tables are spelled in two cases.

Run it from any directory:

    python3 -I internal/parsers/tabby/testdata/table_case.py

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
CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, name TEXT);
INSERT INTO users VALUES (1, 'ana@example.com', 'Ana');
CREATE TABLE threads (id INTEGER PRIMARY KEY, user_id INTEGER, created_at TIMESTAMP);
INSERT INTO threads VALUES (7, 1, '2026-10-01 09:00:00');
CREATE TABLE thread_messages (id INTEGER PRIMARY KEY, thread_id INTEGER, role TEXT, content TEXT,
    created_at TIMESTAMP);
INSERT INTO thread_messages VALUES (1, 7, 'user', 'hello', '2026-10-01 09:00:01');
CREATE TABLE pages (id INTEGER PRIMARY KEY, title TEXT, created_at TIMESTAMP);
INSERT INTO pages VALUES (3, 'Notes', '2026-10-01 09:00:00');
CREATE TABLE page_sections (id INTEGER PRIMARY KEY, page_id INTEGER, title TEXT, content TEXT,
    position INTEGER, created_at TIMESTAMP);
INSERT INTO page_sections VALUES (5, 3, 'One', 'body', 0, '2026-10-01 09:00:03');
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
