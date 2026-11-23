"""Build the hermes package's test databases.

Run it from any directory:

    python3 -I internal/parsers/hermes/testdata/fixtures.py

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

# Two sessions rows of one id, an old one and a new one five months later, each with its own
# project, branch and model; a usage row first seen a day after the new row and one with no
# first_seen; a message between the rows and one after them; and a deletion fence after those.
REPEATED_SESSION_ROWS = """
PRAGMA page_size = 512;
CREATE TABLE sessions (id TEXT, source TEXT NOT NULL, model TEXT, started_at REAL NOT NULL,
  title TEXT, cwd TEXT, git_branch TEXT);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL,
  role TEXT NOT NULL, content TEXT, timestamp REAL NOT NULL);
CREATE TABLE session_model_usage (session_id TEXT NOT NULL, model TEXT NOT NULL,
  billing_provider TEXT, task TEXT, first_seen REAL);
CREATE TABLE state_meta (key TEXT PRIMARY KEY, value TEXT);
INSERT INTO sessions VALUES ('s1', 'cli', 'model-old', 1767225600.0, 'old', '/proj/old', 'branch-old');
INSERT INTO sessions VALUES ('s1', 'cli', 'model-new', 1780272000.0, 'new', '/proj/new', 'branch-new');
INSERT INTO session_model_usage VALUES ('s1', 'usage-dated', NULL, NULL, 1780358400.0);
INSERT INTO session_model_usage VALUES ('s1', 'usage-undated', NULL, NULL, NULL);
INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s1', 'user', 'early message', 1769904000.0);
INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s1', 'assistant', 'late message', 1780444800.0);
INSERT INTO state_meta VALUES ('gateway.retired_session.v1.s1', '{"retired_at": 1780531200.0}');
"""


def repeated_session(count: int) -> str:
    """Return a state.db with count sessions rows of one id and count usage rows of it."""
    rows = [
        """PRAGMA page_size = 512;
CREATE TABLE sessions (id TEXT, source TEXT NOT NULL, model TEXT, started_at REAL NOT NULL,
  title TEXT);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL,
  role TEXT NOT NULL, content TEXT, timestamp REAL NOT NULL);
CREATE TABLE session_model_usage (session_id TEXT NOT NULL, model TEXT NOT NULL,
  billing_provider TEXT, task TEXT, first_seen REAL NOT NULL);"""
    ]
    for i in range(count):
        rows.append(
            "INSERT INTO sessions VALUES ('s1', 'cli', 'model-b', %d.0, 'copy %d');"
            % (1767323045 + i, i)
        )
        rows.append(
            "INSERT INTO session_model_usage VALUES ('s1', 'model-%s', NULL, NULL, %d.5);"
            % ("ab"[i % 2], 1767323045 + i)
        )
    return "\n".join(rows)


def repeated_response(count: int) -> str:
    """Return a response_store.db with count responses rows of one id and a 5,000-byte name."""
    rows = [
        """PRAGMA page_size = 512;
CREATE TABLE responses (response_id TEXT, data TEXT, accessed_at REAL);
CREATE TABLE conversations (name TEXT, response_id TEXT);""",
        "INSERT INTO conversations VALUES ('%s', 'r1');" % ("n" * 5000),
    ]
    for i in range(count):
        rows.append(
            """INSERT INTO responses VALUES ('r1', '{"session_id": "s1", "response": """
            """{"status": "completed", "model": "m", "created_at": %d}}', 0);""" % (1767323045 + i)
        )
    return "\n".join(rows)


DATABASES = {
    "generated_column.db": GENERATED_COLUMN,
    "room_events_without_rowid.db": ROOM_EVENTS_WITHOUT_ROWID,
    "repeated_session_id_10.db": repeated_session(10),
    "repeated_session_id_40.db": repeated_session(40),
    "repeated_response_id_10.db": repeated_response(10),
    "repeated_response_id_40.db": repeated_response(40),
    "repeated_session_rows.db": REPEATED_SESSION_ROWS,
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
