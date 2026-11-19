"""Build the replay package's Tabby users database.

Run it from any directory:

    python3 -I internal/parity/replay/testdata/tabby/users.py

The database is an ee/db.sqlite written by SQLite itself, holding two users, so a test can tell the
time of the user a row names from another user's. Pages are 512 bytes to keep the file small. No
test runs this script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

USERS = """
PRAGMA page_size = 512;
CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, name TEXT, auth_token TEXT,
    created_at TIMESTAMP);
INSERT INTO users VALUES (1, 'alice@example.com', 'Alice', 'auth_FIXTURE', '2026-09-01 09:00:00');
INSERT INTO users VALUES (2, 'bob@example.com', 'Bob', 'auth_OTHER', '2026-09-01 08:00:00');
"""


def main() -> None:
    """Rewrite users.db in this directory."""
    path = HERE / "users.db"
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(USERS)
        connection.commit()


if __name__ == "__main__":
    main()
