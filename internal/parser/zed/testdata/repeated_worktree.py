"""Build the zed package's db.sqlite databases whose archived worktrees repeat one id.

Run it from any directory:

    python3 -I internal/parser/zed/testdata/repeated_worktree.py

repeated_worktree_<n>.db holds n archived_git_worktrees rows of the id 1, as only damage writes
(the id is the table's primary key in Zed's schema, left out here), the row of copy i at
/w/copy<i>, n thread links to the id, and a sidebar thread of session s<i> for each linked
thread t<i>. Pages are 512 bytes to keep the files small. No test runs this script; rerun it
after changing the rows.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

SCHEMA = """
PRAGMA page_size = 512;
CREATE TABLE sidebar_threads(thread_id BLOB PRIMARY KEY, session_id TEXT, agent_id TEXT,
  title TEXT NOT NULL, updated_at TEXT NOT NULL, created_at TEXT, folder_paths TEXT,
  folder_paths_order TEXT, archived INTEGER DEFAULT 0, main_worktree_paths TEXT,
  main_worktree_paths_order TEXT, remote_connection TEXT);
CREATE TABLE archived_git_worktrees(id INTEGER, worktree_path TEXT NOT NULL,
  main_repo_path TEXT NOT NULL, branch_name TEXT, staged_commit_hash TEXT,
  unstaged_commit_hash TEXT, original_commit_hash TEXT) STRICT;
CREATE TABLE thread_archived_worktrees(thread_id TEXT NOT NULL, archived_worktree_id INTEGER NOT NULL);
"""

COUNTS = (2, 10, 40)


def build(path: Path, count: int) -> None:
    """Write the database at path with count rows of the id 1 and count links to it."""
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(SCHEMA)
        connection.executemany(
            "INSERT INTO archived_git_worktrees VALUES (1, ?, '/w/main', NULL, NULL, NULL, NULL)",
            [(f"/w/copy{i}",) for i in range(count)],
        )
        connection.executemany(
            "INSERT INTO thread_archived_worktrees VALUES (?, 1)",
            [(f"t{i:03d}",) for i in range(count)],
        )
        connection.executemany(
            "INSERT INTO sidebar_threads (thread_id, session_id, title, updated_at)"
            " VALUES (?, ?, 'thread', '2026-03-01T09:00:00Z')",
            [(f"t{i:03d}", f"s{i:03d}") for i in range(count)],
        )
        connection.commit()


def main() -> None:
    """Rewrite every database this script builds."""
    for count in COUNTS:
        build(HERE / f"repeated_worktree_{count}.db", count)


if __name__ == "__main__":
    main()
