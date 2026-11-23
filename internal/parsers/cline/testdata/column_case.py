"""Build the cline package's database whose columns are spelled in two cases.

Run it from any directory:

    python3 -I internal/parsers/cline/testdata/column_case.py

column_case.db is written by SQLite from the SQL below: one session row of sessions.db.
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
CREATE TABLE sessions (session_id TEXT PRIMARY KEY, source TEXT, pid INTEGER, started_at TEXT,
  ended_at TEXT, exit_code INTEGER, status TEXT, provider TEXT, model TEXT, cwd TEXT,
  workspace_root TEXT, prompt TEXT, metadata_json TEXT);
INSERT INTO sessions VALUES ('s1', 'cli', 7, '2026-01-01T00:00:00.000Z', '2026-01-01T00:05:00.000Z', 0,
  'completed', 'anthropic', 'claude-x', '/w/proj', '/w/proj', 'fix it', '{"title": "Fix"}');
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
