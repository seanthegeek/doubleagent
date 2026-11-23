"""Build the ollama package's databases of one chat with a long id.

Run it from any directory:

    python3 -I internal/parsers/ollama/testdata/long_chat.py

long_chat_<n>.db holds one assistant message in a chat whose id is 20,000 bytes, with n tool_calls
rows, so every call's tool_use_id is built from the chat's id. Pages are 512 bytes to keep the
files small. No test runs this script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

SCHEMA = """
PRAGMA page_size = 512;
CREATE TABLE messages (id INTEGER PRIMARY KEY, chat_id TEXT, role TEXT, content TEXT, thinking TEXT,
  model_name TEXT, created_at TEXT, tool_result TEXT);
CREATE TABLE tool_calls (id INTEGER PRIMARY KEY, message_id INTEGER, type TEXT, function_name TEXT,
  function_arguments TEXT);
"""

CHAT_LENGTH = 20_000


def build(path: Path, count: int) -> None:
    """Write the database at path with count tool calls."""
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(SCHEMA)
        connection.execute(
            "INSERT INTO messages VALUES (1, ?, 'assistant', 'a', '', 'llama3', '2026-01-02 03:04:05', NULL)",
            ("c" * CHAT_LENGTH,),
        )
        connection.executemany(
            "INSERT INTO tool_calls VALUES (?, 1, 'function', 'search', '{}')",
            [(i,) for i in range(1, count + 1)],
        )
        connection.commit()


def main() -> None:
    """Rewrite the databases in this directory."""
    for count in (20, 40):
        build(HERE / f"long_chat_{count}.db", count)


if __name__ == "__main__":
    main()
