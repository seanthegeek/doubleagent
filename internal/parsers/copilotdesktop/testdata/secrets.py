"""Build the copilotdesktop package's database whose token columns hold fake tokens.

Run it from any directory:

    python3 -I internal/parsers/copilotdesktop/testdata/secrets.py

secrets.db is written by SQLite from the SQL below: one account with a token, one without, and
the settings row with a GitHub token. Pages are 512 bytes to keep the file small. No test runs
this script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

SECRETS = """
PRAGMA page_size = 512;
CREATE TABLE accounts (login TEXT, host TEXT, kind TEXT, is_default INTEGER, access_token TEXT,
  created_at TEXT);
INSERT INTO accounts VALUES ('ana', 'github.com', 'github', 1, 'FIXTURE-TOKEN',
  '2026-01-01T00:00:00Z');
INSERT INTO accounts VALUES ('bo', 'github.com', 'github', 0, NULL, '2026-01-01T00:00:01Z');
CREATE TABLE settings (id INTEGER PRIMARY KEY CHECK (id = 1), storage_location TEXT,
  instructions TEXT, github_access_token TEXT, github_token_type TEXT);
INSERT INTO settings VALUES (1, '/w/store', '', 'FIXTURE-GITHUB-TOKEN', 'oauth');
"""


def main() -> None:
    """Rewrite secrets.db in this directory."""
    path = HERE / "secrets.db"
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(SECRETS)
        connection.commit()


if __name__ == "__main__":
    main()
