"""Build the goose package's test databases.

Run it from any directory:

    python3 -I internal/parser/goose/testdata/fixtures.py

Each database below is a sessions.db written by SQLite itself, so the Go tests read files SQLite
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
CREATE TABLE sessions (id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '',
  working_dir TEXT NOT NULL, created_at TIMESTAMP, model_config_json TEXT,
  shown_name TEXT GENERATED ALWAYS AS (upper(name)) VIRTUAL);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, message_id TEXT,
  session_id TEXT NOT NULL, role TEXT NOT NULL, content_json TEXT NOT NULL,
  created_timestamp INTEGER NOT NULL, timestamp TIMESTAMP, tokens INTEGER, metadata_json TEXT);
INSERT INTO sessions (id, name, working_dir, created_at)
  VALUES ('s1', 'first', '/w/proj', '2026-01-02 03:04:05');
INSERT INTO messages (message_id, session_id, role, content_json, created_timestamp)
  VALUES ('m1', 's1', 'user', '[{"type":"text","text":"hello"}]', 1767323045);
"""

DATABASES = {"generated_column.db": GENERATED_COLUMN}


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
