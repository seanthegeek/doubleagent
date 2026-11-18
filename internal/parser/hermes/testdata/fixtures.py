"""Build the hermes package's test databases.

Run it from any directory:

    python3 -I internal/parser/hermes/testdata/fixtures.py

Each database below is a state.db written by SQLite itself, so the Go tests read files SQLite
wrote. Pages are 512 bytes to keep the files small. No test runs this script; rerun it after
changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

# A sessions table with a virtual generated column, which is computed when read and not stored.
GENERATED_COLUMN = """
PRAGMA page_size = 512;
CREATE TABLE sessions (id TEXT PRIMARY KEY, source TEXT NOT NULL, model TEXT,
  parent_session_id TEXT, started_at REAL NOT NULL, ended_at REAL, end_reason TEXT,
  cwd TEXT, git_branch TEXT, git_repo_root TEXT, title TEXT,
  shown_title TEXT GENERATED ALWAYS AS (upper(title)) VIRTUAL);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL,
  role TEXT NOT NULL, content TEXT, tool_call_id TEXT, tool_calls TEXT, tool_name TEXT,
  timestamp REAL NOT NULL, active INTEGER NOT NULL DEFAULT 1);
INSERT INTO sessions (id, source, model, started_at, cwd, title)
  VALUES ('s1', 'cli', 'model-a', 1767323045.0, '/w/proj', 'first');
INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s1', 'user', 'hello', 1767323046.0);
"""

# Room events in a table without rowids, which a query naming the rowid cannot read.
ROOM_EVENTS_WITHOUT_ROWID = """
PRAGMA page_size = 512;
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL,
  role TEXT NOT NULL, content TEXT, timestamp REAL NOT NULL);
CREATE TABLE hosted_room_events (room_id TEXT NOT NULL, seq INTEGER NOT NULL,
  event_id TEXT NOT NULL, kind TEXT NOT NULL, actor_json TEXT NOT NULL,
  payload_json TEXT NOT NULL, created_at REAL NOT NULL, PRIMARY KEY (room_id, seq)) WITHOUT ROWID;
INSERT INTO hosted_room_events VALUES ('r1', 1, 'e1', 'message', '{}', '{}', 1767323045.0);
"""

DATABASES = {
    "generated_column.db": GENERATED_COLUMN,
    "room_events_without_rowid.db": ROOM_EVENTS_WITHOUT_ROWID,
}


def main() -> None:
    """Rewrite every database of DATABASES in this directory."""
    for name, script in DATABASES.items():
        path = HERE / name
        path.unlink(missing_ok=True)
        with contextlib.closing(sqlite3.connect(path)) as connection:
            connection.executescript(script)
            connection.commit()


if __name__ == "__main__":
    main()
