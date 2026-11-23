"""Build the tabby package's database whose columns are spelled in two cases.

Run it from any directory:

    python3 -I internal/parsers/tabby/testdata/column_case.py

column_case.db is written by SQLite from the SQL below: a user, an integration and a repository it reaches.
column_case_respelled.db is the same database with every column of every table renamed to its
name with the case of each letter swapped, which SQL matches as the same column. Pages are 512
bytes to keep the files small. No test runs this script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

COLUMN_CASE = """
PRAGMA page_size = 512;
CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, name TEXT, auth_token TEXT,
  created_at TIMESTAMP);
INSERT INTO users VALUES (1, 'ana@example.com', 'Ana', 'auth_FIXTURE', '2026-10-01 09:00:00');
CREATE TABLE integrations (id INTEGER PRIMARY KEY, kind TEXT, display_name TEXT,
  access_token TEXT, api_base TEXT, created_at TIMESTAMP);
INSERT INTO integrations VALUES (1, 'github', 'work', 'FIXTURE-TOKEN', NULL, '2026-10-01 09:00:00');
CREATE TABLE provided_repositories (id INTEGER PRIMARY KEY, integration_id INTEGER, name TEXT,
  git_url TEXT, active BOOLEAN, updated_at TIMESTAMP);
INSERT INTO provided_repositories VALUES (3, 1, 'o/r', 'https://github.com/o/r', 1,
  '2026-10-02 09:00:00');
"""


def build(path: Path, respell: bool) -> None:
    """Write the database at path, its columns respelled when respell is set."""
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(COLUMN_CASE)
        if respell:
            tables = [
                row[0]
                for row in connection.execute(
                    "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite%'"
                )
            ]
            for table in tables:
                columns = [row[1] for row in connection.execute(f'PRAGMA table_info("{table}")')]
                for column in columns:
                    connection.execute(
                        f'ALTER TABLE "{table}" RENAME COLUMN "{column}" TO "{column}_respelling"'
                    )
                    connection.execute(
                        f'ALTER TABLE "{table}" RENAME COLUMN "{column}_respelling" TO "{column.swapcase()}"'
                    )
        connection.commit()
        connection.execute("VACUUM")


def main() -> None:
    """Rewrite both databases in this directory."""
    build(HERE / "column_case.db", respell=False)
    build(HERE / "column_case_respelled.db", respell=True)


if __name__ == "__main__":
    main()
