"""Build the ollama package's databases whose messages table repeats a message id.

Run it from any directory:

    python3 -I internal/parsers/ollama/testdata/repeated_message_id.py

repeated_message_id_<n>.db holds n assistant messages of id 1, each with its own content, and n
tool_calls rows of message 1; then n user messages of id 2 and n attachments of message 2. The
messages table has no key, so the id can repeat. equal_message_id.db holds one assistant message
of id 3 stored as an integer and one of id 3.0 stored as a real number, in a column with no
declared type, which SQL's = finds equal, and one tool call of message 3. Pages are 512 bytes to
keep the files small. No test runs this script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

SCHEMA = """
PRAGMA page_size = 512;
CREATE TABLE chats (id TEXT, title TEXT);
CREATE TABLE messages (id, chat_id TEXT, role TEXT, content TEXT, thinking TEXT, model_name TEXT,
  created_at TEXT, tool_result TEXT);
CREATE TABLE tool_calls (id INTEGER, message_id, type TEXT, function_name TEXT, function_arguments TEXT);
CREATE TABLE attachments (id INTEGER, message_id, filename TEXT, data BLOB);
INSERT INTO chats VALUES ('c', 'repeat');
"""

TIME = "2026-01-02 03:04:05"


def build_repeated(path: Path, count: int) -> None:
    """Write the database at path with count rows of each repeated id."""
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(SCHEMA)
        connection.executemany(
            "INSERT INTO messages VALUES (1, 'c', 'assistant', ?, '', 'llama3', ?, NULL)",
            [(f"answer {i}", TIME) for i in range(count)],
        )
        connection.executemany(
            "INSERT INTO tool_calls VALUES (?, 1, 'function', 'search', ?)",
            [(i, f'{{"query":"q{i}"}}') for i in range(count)],
        )
        connection.executemany(
            "INSERT INTO messages VALUES (2, 'c', 'user', ?, '', 'llama3', ?, NULL)",
            [(f"question {i}", TIME) for i in range(count)],
        )
        connection.executemany(
            "INSERT INTO attachments VALUES (?, 2, ?, X'00')",
            [(i, f"f{i}.txt") for i in range(count)],
        )
        connection.commit()


def build_equal(path: Path) -> None:
    """Write the database at path with one id stored as an integer and as a real number."""
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(SCHEMA)
        connection.execute(
            "INSERT INTO messages VALUES (3, 'c', 'assistant', 'first', '', 'llama3', ?, NULL)",
            (TIME,),
        )
        connection.execute(
            "INSERT INTO messages VALUES (3.0, 'c', 'assistant', 'second', '', 'llama3', ?, NULL)",
            (TIME,),
        )
        connection.execute(
            "INSERT INTO tool_calls VALUES (1, 3, 'function', 'search', '{\"query\":\"q\"}')"
        )
        connection.commit()


def main() -> None:
    """Rewrite the databases in this directory."""
    for count in (10, 40):
        build_repeated(HERE / f"repeated_message_id_{count}.db", count)
    build_equal(HERE / "equal_message_id.db")


if __name__ == "__main__":
    main()
