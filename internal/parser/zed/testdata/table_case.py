"""Build the zed package's database whose tables are spelled in two cases.

Run it from any directory:

    python3 -I internal/parser/zed/testdata/table_case.py

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
CREATE TABLE sidebar_threads(thread_id BLOB PRIMARY KEY, session_id TEXT, agent_id TEXT, title TEXT NOT NULL,
  updated_at TEXT NOT NULL, created_at TEXT, folder_paths TEXT, folder_paths_order TEXT, archived INTEGER DEFAULT 0,
  main_worktree_paths TEXT, main_worktree_paths_order TEXT, remote_connection TEXT);
CREATE TABLE trusted_worktrees (trust_id INTEGER PRIMARY KEY AUTOINCREMENT,
  absolute_path TEXT, user_name TEXT, host_name TEXT) STRICT;
INSERT INTO sidebar_threads (thread_id, session_id, title, updated_at, created_at, folder_paths)
  VALUES (X'01', 's1', 'Fix the tests', '2026-01-02T03:04:05Z', '2026-01-02T03:04:00Z', '/w/proj');
INSERT INTO trusted_worktrees (absolute_path) VALUES ('/w/proj');
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
