"""Build the zed package's db.sqlite databases whose joins copy one long value onto many rows.

Run it from any directory:

    python3 -I internal/parsers/zed/testdata/carried.py

carried_<n>.db holds one archived worktree whose paths and branch are each 10,000 bytes, linked
to n threads, and one sidebar thread whose main worktree is a 10,000-byte path and whose folders
are n created worktrees. Pages are 512 bytes to keep the files small. No test runs this script;
rerun it after changing the rows.
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
CREATE TABLE thread_archived_worktrees(thread_id TEXT NOT NULL, archived_worktree_id INTEGER NOT NULL);
"""

SIZES = (50, 100)


def long_path(name: str) -> str:
    """Return a path of 10,000 bytes starting /w/<name>/."""
    start = f"/w/{name}/"
    return start + "a" * (10_000 - len(start))


def write(count: int) -> None:
    """Write carried_<count>.db afresh."""
    path = HERE / f"carried_{count}.db"
    path.unlink(missing_ok=True)
    folders = [f"/w/tree{i}" for i in range(count)]
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(SCHEMA)
        connection.execute(
            "INSERT INTO sidebar_threads (thread_id, session_id, title, updated_at, folder_paths,"
            " main_worktree_paths) VALUES (?, ?, ?, ?, ?, ?)",
            (
                bytes(16),
                "s0",
                "thread",
                "2026-01-02T03:04:05Z",
                "\n".join(folders),
                long_path("main"),
            ),
        )
        connection.execute(
            "INSERT INTO archived_git_worktrees (id, worktree_path, main_repo_path, branch_name)"
            " VALUES (1, ?, ?, ?)",
            (long_path("tree"), long_path("repo"), "b" * 10_000),
        )
        for i, folder in enumerate(folders):
            connection.execute("INSERT INTO thread_archived_worktrees VALUES (?, 1)", (f"t{i}",))
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
