"""Build the crush package's database whose tables are spelled in two cases.

Run it from any directory:

    python3 -I internal/parsers/crush/testdata/table_case.py

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
CREATE TABLE sessions (id TEXT PRIMARY KEY, title TEXT, created_at INTEGER);
CREATE TABLE messages (id TEXT, session_id TEXT, role TEXT, parts TEXT, created_at INTEGER);
CREATE TABLE files (id TEXT, session_id TEXT, path TEXT, content TEXT, version INTEGER,
    created_at INTEGER);
CREATE TABLE read_files (session_id TEXT, path TEXT, read_at INTEGER);
INSERT INTO sessions VALUES ('s1', 'first', 1760000000);
INSERT INTO messages VALUES ('m1', 's1', 'user', '[{"type":"text","data":{"text":"hello"}}]', 1760000001);
INSERT INTO files VALUES ('f1', 's1', '/w/a.txt', 'hello world', 1, 1760000002);
INSERT INTO read_files VALUES ('s1', '/w/a.txt', 1760000003);
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
