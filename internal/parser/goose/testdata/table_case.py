"""Build the goose package's database whose tables are spelled in two cases.

Run it from any directory:

    python3 -I internal/parser/goose/testdata/table_case.py

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
CREATE TABLE sessions (id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '',
  working_dir TEXT NOT NULL, created_at TIMESTAMP, model_config_json TEXT);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, message_id TEXT,
  session_id TEXT NOT NULL, role TEXT NOT NULL, content_json TEXT NOT NULL,
  created_timestamp INTEGER NOT NULL, timestamp TIMESTAMP, tokens INTEGER, metadata_json TEXT);
INSERT INTO sessions (id, name, working_dir, created_at)
  VALUES ('s1', 'first', '/w/proj', '2026-01-02 03:04:05');
INSERT INTO messages (message_id, session_id, role, content_json, created_timestamp)
  VALUES ('m1', 's1', 'user', '[{"type":"text","text":"hello"}]', 1767323045);
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
