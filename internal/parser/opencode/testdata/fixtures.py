"""Build the opencode package's test databases.

Run it from any directory:

    python3 -I internal/parser/opencode/testdata/fixtures.py

Each database below is rewritten from its SQL by SQLite itself, so the Go tests read files
SQLite wrote. Pages are 512 bytes to keep the files small. No test runs this script; rerun it
after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

# A 1.x database: one session with one user message.
ONE_SESSION = """
CREATE TABLE project (id text PRIMARY KEY, worktree text NOT NULL, time_created integer,
    time_updated integer);
INSERT INTO project VALUES ('p1', '/work/proj', 1700000000000, 1700000000000);
CREATE TABLE session (id text PRIMARY KEY, project_id text, parent_id text, slug text,
    directory text, title text, version text, time_created integer, time_updated integer);
INSERT INTO session VALUES ('ses_1', 'p1', NULL, 'slug', '/work/proj', 'Fix bug', '1.0.0',
    1700000000000, 1700000001000);
CREATE TABLE message (id text PRIMARY KEY, session_id text, time_created integer,
    time_updated integer, data text);
INSERT INTO message VALUES ('msg_1', 'ses_1', 1700000000500, 1700000000500,
    '{"role":"user","time":{"created":1700000000500}}');
CREATE TABLE part (id text PRIMARY KEY, message_id text, session_id text, time_created integer,
    time_updated integer, data text);
INSERT INTO part VALUES ('prt_1', 'msg_1', 'ses_1', 1700000000500, 1700000000500,
    '{"type":"text","text":"hello there"}');
"""

DATABASES = {
    "no_permission_table.db": ONE_SESSION,
    # The first layout of the permission table, which every early 1.x database had.
    "first_layout_permission.db": ONE_SESSION
    + "CREATE TABLE permission (project_id text PRIMARY KEY, data text NOT NULL);",
    "first_layout_permission_with_times.db": ONE_SESSION
    + """CREATE TABLE permission (project_id text PRIMARY KEY, data text NOT NULL,
        time_created integer, time_updated integer);
    INSERT INTO permission VALUES ('p1', '{}', 1700000000000, 1700000000000);""",
}


def main() -> None:
    """Rewrite every database of DATABASES in this directory."""
    for name, script in DATABASES.items():
        path = HERE / name
        path.unlink(missing_ok=True)
        with contextlib.closing(sqlite3.connect(path)) as connection:
            connection.executescript("PRAGMA page_size = 512;" + script)
            connection.commit()


if __name__ == "__main__":
    main()
