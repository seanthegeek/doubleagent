"""Build the crush package's test databases.

Run it from any directory:

    python3 -I internal/parser/crush/testdata/fixtures.py

Each database below is a crush.db written by SQLite itself, so the Go tests read files SQLite
wrote. Pages are 512 bytes to keep the files small. No test runs this script; rerun it after
changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

# File snapshots whose content is ASCII, non-ASCII text, a blob, a number and NULL.
FILE_VERSIONS = """
PRAGMA page_size = 512;
CREATE TABLE sessions (id TEXT PRIMARY KEY, title TEXT, created_at INTEGER);
CREATE TABLE messages (id TEXT, session_id TEXT, role TEXT, parts TEXT, created_at INTEGER);
CREATE TABLE files (id TEXT, session_id TEXT, path TEXT, content TEXT, version INTEGER,
    created_at INTEGER);
INSERT INTO files VALUES ('f1', 's1', '/w/ascii.txt', 'hello world', 1, 1760000001);
INSERT INTO files VALUES ('f2', 's1', '/w/accents.txt', 'héllo wörld ✓', 1, 1760000002);
INSERT INTO files VALUES ('f3', 's1', '/w/blob.bin', X'00ff00ff', 1, 1760000003);
INSERT INTO files VALUES ('f4', 's1', '/w/number.txt', 12345, 1, 1760000004);
INSERT INTO files VALUES ('f5', 's1', '/w/null.txt', NULL, 1, 1760000005);
"""

# A messages table with a virtual generated column, which is computed when read and not stored.
GENERATED_COLUMN = """
PRAGMA page_size = 512;
CREATE TABLE sessions (id TEXT PRIMARY KEY, title TEXT, created_at INTEGER);
CREATE TABLE messages (id TEXT, session_id TEXT, role TEXT, parts TEXT, created_at INTEGER,
    shown_role TEXT GENERATED ALWAYS AS (upper(role)) VIRTUAL);
INSERT INTO sessions VALUES ('s1', 'first', 1760000000);
INSERT INTO messages (id, session_id, role, parts, created_at)
    VALUES ('m1', 's1', 'user', '[{"type":"text","data":{"text":"hello"}}]', 1760000001);
"""

DATABASES = {"file_versions.db": FILE_VERSIONS, "generated_column.db": GENERATED_COLUMN}


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
