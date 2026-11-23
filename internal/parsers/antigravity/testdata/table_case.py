"""Build the antigravity package's database whose tables are spelled in two cases.

Run it from any directory:

    python3 -I internal/parsers/antigravity/testdata/table_case.py

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
CREATE TABLE conversation_summaries (conversation_id text, title text NOT NULL DEFAULT "",
  preview text NOT NULL DEFAULT "", step_count integer NOT NULL DEFAULT 0,
  last_modified_time datetime NOT NULL, workspace_uris text NOT NULL,
  status text NOT NULL DEFAULT "", agent_name text NOT NULL DEFAULT "",
  parent_conversation_id text NOT NULL DEFAULT "", PRIMARY KEY (conversation_id));
INSERT INTO conversation_summaries (conversation_id, title, step_count, last_modified_time,
  workspace_uris, status) VALUES ('c1', 'Fix the tests', 3, '2026-01-02T03:04:05Z',
  '["file:///w/proj"]', 'idle');
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
