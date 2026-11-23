"""Build the chatgptdesktop package's database whose tables are spelled in two cases.

Run it from any directory:

    python3 -I internal/parsers/chatgptdesktop/testdata/table_case.py

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
CREATE TABLE local_thread_catalog (host_id TEXT NOT NULL, thread_id TEXT NOT NULL,
  display_title TEXT, source_created_at REAL, source_updated_at REAL, cwd TEXT,
  source_kind TEXT, source_detail TEXT, model_provider TEXT, git_branch TEXT,
  observation_sequence INTEGER, missing_candidate INTEGER, thread_source TEXT,
  source_recency_at REAL, pending_observed_title TEXT, project_id TEXT,
  conversation_origin TEXT, trial_conversation_type TEXT, chatgpt_async_status TEXT,
  PRIMARY KEY (host_id, thread_id));
CREATE TABLE local_thread_catalog_hosts (host_id TEXT PRIMARY KEY, host_kind TEXT);
INSERT INTO local_thread_catalog_hosts VALUES ('chatgpt:h1', 'chatgpt');
INSERT INTO local_thread_catalog VALUES ('chatgpt:h1', 't1', 'Plan a trip',
  1790848800.25, 1790852400.5, NULL, 'chatgpt', NULL, NULL, NULL, 1, 0, NULL,
  1790852400.5, NULL, NULL, '', NULL, NULL);
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
