"""Build the copilotcli package's databases of one session with a long cwd and branch.

Run it from any directory:

    python3 -I internal/parsers/copilotcli/testdata/session_columns.py

session_columns_<n>.db holds one sessions row whose cwd and branch are 50,000 bytes each and n
turns of that session, each with a user message and a response, for n of 20 and 40. No test
runs this script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

LENGTH = 50_000

SCHEMA = """
CREATE TABLE sessions (id TEXT PRIMARY KEY, cwd TEXT, repository TEXT, host_type TEXT,
  branch TEXT, summary TEXT, created_at TEXT, updated_at TEXT);
CREATE TABLE turns (session_id TEXT, turn_index INTEGER, user_message TEXT,
  assistant_response TEXT, timestamp TEXT, UNIQUE (session_id, turn_index));
"""

TIME = "2026-01-02T03:04:05.000Z"


def build(path: Path, turns: int) -> None:
    """Write the database of one session and its turns at path."""
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(SCHEMA)
        connection.execute(
            "INSERT INTO sessions VALUES (?, ?, NULL, NULL, ?, NULL, ?, ?)",
            ("s1", "/" + "c" * (LENGTH - 1), "b" * LENGTH, TIME, TIME),
        )
        connection.executemany(
            "INSERT INTO turns VALUES ('s1', ?, ?, ?, ?)",
            [(i, f"u{i}", f"a{i}", TIME) for i in range(turns)],
        )
        connection.commit()


def main() -> None:
    """Rewrite both databases in this directory."""
    for turns in (20, 40):
        build(HERE / f"session_columns_{turns}.db", turns)


if __name__ == "__main__":
    main()
