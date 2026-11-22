"""Build the nanobot package's database whose tables are spelled in two cases.

Run it from any directory:

    python3 -I internal/parser/nanobot/testdata/table_case.py

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
CREATE TABLE storage_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE sessions (
    key TEXT PRIMARY KEY, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
    metadata TEXT NOT NULL, provider_state TEXT, last_archived INTEGER NOT NULL,
    revision INTEGER NOT NULL, generation TEXT NOT NULL,
    title TEXT NOT NULL, preview TEXT NOT NULL, visible_updated_at TEXT
);
CREATE TABLE messages (
    session_key TEXT NOT NULL REFERENCES sessions(key) ON DELETE CASCADE,
    position INTEGER NOT NULL, record TEXT NOT NULL,
    PRIMARY KEY (session_key, position)
);
INSERT INTO sessions VALUES ('cli:direct', '2026-01-02T03:04:05', '2026-01-02T03:05:00', '{}', NULL, 0, 3, 'g1', '', '', NULL);
INSERT INTO messages VALUES ('cli:direct', 0, '{"role":"user","content":"hello","timestamp":"2026-01-02T03:04:06"}');
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
