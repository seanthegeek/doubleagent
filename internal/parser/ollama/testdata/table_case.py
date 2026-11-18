"""Build the ollama package's database whose tables are spelled in two cases.

Run it from any directory:

    python3 -I internal/parser/ollama/testdata/table_case.py

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
CREATE TABLE chats (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, browser_state TEXT);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, chat_id TEXT NOT NULL, role TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '', thinking TEXT NOT NULL DEFAULT '', stream BOOLEAN NOT NULL DEFAULT 0,
  model_name TEXT, model_cloud BOOLEAN, model_ollama_host BOOLEAN,
  created_at TIMESTAMP NOT NULL DEFAULT '2026-01-02 03:04:05', updated_at TIMESTAMP NOT NULL DEFAULT '2026-01-02 03:04:05',
  thinking_time_start TIMESTAMP, thinking_time_end TIMESTAMP, tool_result TEXT);
CREATE TABLE tool_calls (id INTEGER PRIMARY KEY AUTOINCREMENT, message_id INTEGER NOT NULL, type TEXT NOT NULL,
  function_name TEXT NOT NULL, function_arguments TEXT NOT NULL, function_result TEXT);
CREATE TABLE attachments (id INTEGER PRIMARY KEY AUTOINCREMENT, message_id INTEGER NOT NULL,
  filename TEXT NOT NULL, data BLOB NOT NULL);
INSERT INTO chats (id, title, created_at) VALUES ('c1', 'first', '2026-01-02 03:04:05');
INSERT INTO messages (chat_id, role, content, model_name) VALUES ('c1', 'user', 'hello', 'llama3');
INSERT INTO messages (chat_id, role, content, model_name) VALUES ('c1', 'assistant', 'hi', 'llama3');
INSERT INTO tool_calls (message_id, type, function_name, function_arguments) VALUES (2, 'function', 'search', '{}');
INSERT INTO attachments (message_id, filename, data) VALUES (1, 'a.txt', X'616263');
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
