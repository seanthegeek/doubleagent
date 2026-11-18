"""Build the opencode package's database whose tables are spelled in two cases.

Run it from any directory:

    python3 -I internal/parser/opencode/testdata/table_case.py

table_case.db is written by SQLite from the SQL below. table_case_respelled.db is the same
database with every table renamed to its name with the case of each letter swapped, which SQL
matches as the same table. Pages are 512 bytes to keep the files small. No test runs this
script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

TABLE_CASE = """
PRAGMA page_size = 512;
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


def build(path: Path, respell: bool) -> None:
    """Write the database at path, its tables respelled when respell is set."""
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(TABLE_CASE)
        if respell:
            names = [
                row[0]
                for row in connection.execute(
                    "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite%'"
                )
            ]
            for name in names:
                connection.execute(f'ALTER TABLE "{name}" RENAME TO "{name}_respelling"')
                connection.execute(f'ALTER TABLE "{name}_respelling" RENAME TO "{name.swapcase()}"')
        connection.commit()


def main() -> None:
    """Rewrite both databases in this directory."""
    build(HERE / "table_case.db", respell=False)
    build(HERE / "table_case_respelled.db", respell=True)


if __name__ == "__main__":
    main()
