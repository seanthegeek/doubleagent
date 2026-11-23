"""Build the hermes package's database whose tables are spelled in two cases.

Run it from any directory:

    python3 -I internal/parsers/hermes/testdata/table_case.py

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
CREATE TABLE sessions (id TEXT PRIMARY KEY, source TEXT NOT NULL, model TEXT,
  parent_session_id TEXT, started_at REAL NOT NULL, ended_at REAL, end_reason TEXT,
  cwd TEXT, git_branch TEXT, git_repo_root TEXT, title TEXT);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL,
  role TEXT NOT NULL, content TEXT, tool_call_id TEXT, tool_calls TEXT, tool_name TEXT,
  timestamp REAL NOT NULL, active INTEGER NOT NULL DEFAULT 1);
CREATE TABLE session_model_usage (session_id TEXT NOT NULL, model TEXT NOT NULL,
  billing_provider TEXT NOT NULL DEFAULT '', billing_base_url TEXT NOT NULL DEFAULT '',
  billing_mode TEXT NOT NULL DEFAULT '', task TEXT NOT NULL DEFAULT '',
  api_call_count INTEGER NOT NULL DEFAULT 0, first_seen REAL, last_seen REAL,
  PRIMARY KEY (session_id, model, billing_provider, billing_base_url, billing_mode, task));
CREATE TABLE conversation_generations (source TEXT NOT NULL, session_key TEXT NOT NULL,
  generation INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (source, session_key));
INSERT INTO sessions (id, source, model, started_at, cwd, title)
  VALUES ('s1', 'cli', 'model-a', 1767323045.0, '/w/proj', 'first');
INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s1', 'user', 'hello', 1767323046.0);
INSERT INTO session_model_usage (session_id, model, api_call_count, first_seen, last_seen)
  VALUES ('s1', 'model-b', 2, 1767323047.0, 1767323048.0);
INSERT INTO conversation_generations VALUES ('telegram', 'chat-1', 2);
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
