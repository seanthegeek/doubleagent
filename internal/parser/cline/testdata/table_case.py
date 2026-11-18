"""Build the cline package's database whose tables are spelled in two cases.

Run it from any directory:

    python3 -I internal/parser/cline/testdata/table_case.py

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
CREATE TABLE sessions (session_id TEXT PRIMARY KEY, source TEXT, pid INTEGER, started_at TEXT,
  ended_at TEXT, exit_code INTEGER, status TEXT, interactive INTEGER, provider TEXT, model TEXT,
  cwd TEXT, workspace_root TEXT, prompt TEXT, metadata_json TEXT, is_subagent INTEGER,
  parent_session_id TEXT, updated_at TEXT);
INSERT INTO sessions VALUES ('s1', 'cli', 4242, '2026-10-03T10:01:40.000Z', NULL, NULL, 'running',
  1, 'anthropic', 'model-a', '/w/proj', '/w/proj', 'fix tests', NULL, 0, NULL,
  '2026-10-03T10:02:00.000Z');
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
