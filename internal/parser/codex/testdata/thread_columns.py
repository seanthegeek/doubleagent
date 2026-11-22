"""Build the codex package's state databases of one thread with a long cwd and branch.

Run it from any directory:

    python3 -I internal/parser/codex/testdata/thread_columns.py

thread_columns_<n>.db is a state_5.sqlite whose threads table holds one thread, t1, whose cwd and
git_branch are 50,000 bytes each, and whose guardian_review_feedback table holds n records
reviewing t1, for n of 20 and 40. Pages are 512 bytes to keep the files small. No test runs this
script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

LENGTH = 50_000

SCHEMA = """
PRAGMA page_size = 512;
CREATE TABLE threads (id TEXT PRIMARY KEY, rollout_path TEXT, created_at INTEGER, cwd TEXT,
  title TEXT, git_branch TEXT);
CREATE TABLE guardian_review_feedback (id TEXT, thread_id TEXT, record TEXT);
"""


def build(path: Path, records: int) -> None:
    """Write the database of one thread and records feedback records at path."""
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(SCHEMA)
        connection.execute(
            "INSERT INTO threads VALUES ('t1', '/r.jsonl', 1767323045, ?, 'title', ?)",
            ("/" + "c" * (LENGTH - 1), "b" * LENGTH),
        )
        connection.executemany(
            "INSERT INTO guardian_review_feedback VALUES (?, 't1', ?)",
            [
                (f"0198f0a1-0000-7000-8000-{number:012d}", '{"reviewed_thread_id": "t1"}')
                for number in range(records)
            ],
        )
        connection.commit()


def main() -> None:
    """Rewrite both databases in this directory."""
    for records in (20, 40):
        build(HERE / f"thread_columns_{records}.db", records)


if __name__ == "__main__":
    main()
