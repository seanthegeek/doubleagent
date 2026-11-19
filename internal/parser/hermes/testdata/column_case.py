"""Build the hermes package's database whose columns are spelled in two cases.

Run it from any directory:

    python3 -I internal/parser/hermes/testdata/column_case.py

column_case.db is written by SQLite from the SQL below: one session of state.db with two messages.
column_case_respelled.db is the same database with every column of every table renamed to its
name with the case of each letter swapped, which SQL matches as the same column. Pages are 512
bytes to keep the files small. No test runs this script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

COLUMN_CASE = """
PRAGMA page_size = 512;
CREATE TABLE sessions (id TEXT PRIMARY KEY, source TEXT NOT NULL, model TEXT,
  parent_session_id TEXT, started_at REAL NOT NULL, ended_at REAL, end_reason TEXT,
  cwd TEXT, git_branch TEXT, git_repo_root TEXT, title TEXT, archived INTEGER NOT NULL DEFAULT 0);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL,
  role TEXT NOT NULL, content TEXT, tool_call_id TEXT, tool_calls TEXT, tool_name TEXT,
  timestamp REAL NOT NULL, finish_reason TEXT);
INSERT INTO sessions VALUES ('20261029_093015_5c7e19a2', 'cli', 'openai/gpt-5.1', NULL,
  1793266215.0, NULL, NULL, '/home/u/work/site', 'fix-nav', '/home/u/work/site', 'Count lines', 0);
INSERT INTO messages (session_id, role, content, timestamp) VALUES
  ('20261029_093015_5c7e19a2', 'user', 'how many lines?', 1793266216.4),
  ('20261029_093015_5c7e19a2', 'assistant', 'Twelve.', 1793266217.9);
"""


def build(path: Path, respell: bool) -> None:
    """Write the database at path, its columns respelled when respell is set."""
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(COLUMN_CASE)
        if respell:
            tables = [
                row[0]
                for row in connection.execute(
                    "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite%'"
                )
            ]
            for table in tables:
                columns = [row[1] for row in connection.execute(f'PRAGMA table_info("{table}")')]
                for column in columns:
                    connection.execute(
                        f'ALTER TABLE "{table}" RENAME COLUMN "{column}" TO "{column}_respelling"'
                    )
                    connection.execute(
                        f'ALTER TABLE "{table}" RENAME COLUMN "{column}_respelling" TO "{column.swapcase()}"'
                    )
        connection.commit()
        connection.execute("VACUUM")


def main() -> None:
    """Rewrite both databases in this directory."""
    build(HERE / "column_case.db", respell=False)
    build(HERE / "column_case_respelled.db", respell=True)


if __name__ == "__main__":
    main()
