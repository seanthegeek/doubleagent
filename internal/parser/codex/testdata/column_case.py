"""Build the codex package's database whose columns are spelled in two cases.

Run it from any directory:

    python3 -I internal/parser/codex/testdata/column_case.py

column_case.db is written by SQLite from the SQL below: one thread of a state database and one feedback row of a logs database.
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
create table threads (id text primary key, rollout_path text not null,
  created_at integer not null, updated_at integer not null, source text not null,
  model_provider text not null, cwd text not null, title text not null,
  sandbox_policy text not null, approval_mode text not null, git_branch text,
  git_origin_url text, first_user_message text not null default '', model text);
insert into threads values ('t1', '/home/u/.codex/sessions/rollout-t1.jsonl', 1767323045, 1767323050,
  'cli', 'openai', '/w/proj', 'Fix the tests', '{}', 'on-request', 'main',
  'https://github.com/o/r.git', 'fix the tests', 'gpt-5');
create table logs (id integer primary key autoincrement, ts integer not null, ts_nanos integer not null,
  level text not null, target text not null, feedback_log_body text, thread_id text);
insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id) values (1767323046, 5,
  'INFO', 'codex_feedback', 'feedback event uploaded to Sentry classification=bug', 't1');
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
