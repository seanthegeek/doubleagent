"""Build the nanobot package's session databases whose sessions table repeats one key.

Run it from any directory:

    python3 -I internal/parser/nanobot/testdata/repeated_key.py

repeated_key_<n>.db holds n sessions rows of the key cli:direct, as only damage writes (the key
is the table's primary key in nanobot's schema, left out here), each created a second after the
last, and n messages of that key; repeated_key_without_messages_<n>.db the same rows and no
messages. Pages are 512 bytes to keep the files small. No test runs this
script; rerun it after changing the rows.
"""

from __future__ import annotations

import contextlib
import json
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

SCHEMA = """
PRAGMA page_size = 512;
CREATE TABLE sessions (key TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  metadata TEXT NOT NULL);
CREATE TABLE messages (session_key TEXT NOT NULL, position INTEGER NOT NULL, record TEXT NOT NULL);
"""

COUNTS = (10, 40)


def build(path: Path, count: int, messages: bool = True) -> None:
    """Write the database at path with count rows of the key and, if messages, count messages."""
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(SCHEMA)
        connection.executemany(
            "INSERT INTO sessions VALUES (?, ?, '', '{}')",
            [("cli:direct", f"2026-01-02T03:{i // 60:02d}:{i % 60:02d}") for i in range(count)],
        )
        record = {"role": "user", "timestamp": "2026-01-02T04:00:00"}
        connection.executemany(
            "INSERT INTO messages VALUES ('cli:direct', ?, ?)",
            [
                (i, json.dumps({**record, "content": f"message {i}"}))
                for i in range(count if messages else 0)
            ],
        )
        connection.commit()


def main() -> None:
    """Rewrite every database this script builds."""
    for count in COUNTS:
        build(HERE / f"repeated_key_{count}.db", count)
        build(HERE / f"repeated_key_without_messages_{count}.db", count, messages=False)


if __name__ == "__main__":
    main()
