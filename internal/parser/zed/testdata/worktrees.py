"""Build the zed package's db.sqlite databases of Zed-created worktrees.

Run it from any directory:

    python3 -I internal/parser/zed/testdata/worktrees.py

worktrees_<n>.db holds n sidebar threads, n archived worktrees and n created worktrees: thread i
and archived worktree i name the folder /w/tree<i>, and created worktree i is at that folder.
Pages are 512 bytes to keep the files small. No test runs this script; rerun it after changing
the rows.
"""

from __future__ import annotations

import contextlib
import json
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

SCHEMA = """
PRAGMA page_size = 512;
CREATE TABLE sidebar_threads(thread_id BLOB PRIMARY KEY, session_id TEXT, agent_id TEXT,
  title TEXT NOT NULL, updated_at TEXT NOT NULL, created_at TEXT, folder_paths TEXT,
  folder_paths_order TEXT, archived INTEGER DEFAULT 0, main_worktree_paths TEXT,
  main_worktree_paths_order TEXT, remote_connection TEXT);
CREATE TABLE scoped_kv_store(namespace TEXT NOT NULL, key TEXT NOT NULL,
  value TEXT NOT NULL, PRIMARY KEY(namespace, key)) STRICT;
CREATE TABLE archived_git_worktrees(id INTEGER PRIMARY KEY, worktree_path TEXT NOT NULL,
  main_repo_path TEXT NOT NULL, branch_name TEXT, staged_commit_hash TEXT,
  unstaged_commit_hash TEXT, original_commit_hash TEXT) STRICT;
"""

SIZES = (8, 64)


def write(count: int) -> None:
    """Write worktrees_<count>.db afresh."""
    path = HERE / f"worktrees_{count}.db"
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(SCHEMA)
        for i in range(count):
            folder = f"/w/tree{i}"
            connection.execute(
                "INSERT INTO sidebar_threads (thread_id, session_id, title, updated_at,"
                " folder_paths, main_worktree_paths) VALUES (?, ?, ?, ?, ?, ?)",
                (
                    bytes([i]),
                    f"s{i}",
                    f"thread {i}",
                    f"2026-01-02T03:{i % 60:02d}:00Z",
                    folder,
                    "/w/main",
                ),
            )
            connection.execute(
                "INSERT INTO archived_git_worktrees (worktree_path, main_repo_path) VALUES (?, ?)",
                (folder, "/w/archived-main"),
            )
            connection.execute(
                "INSERT INTO scoped_kv_store VALUES ('created_git_worktrees', ?, ?)",
                (f"local\n{folder}", json.dumps({"created_at_seconds": 1767323045 + i})),
            )
        connection.commit()


def main() -> None:
    """Rewrite every database of SIZES in this directory."""
    for count in SIZES:
        write(count)


if __name__ == "__main__":
    main()
