"""Write the copilotdesktop package's databases of one project with a long path.

Run from this directory with any Python 3: `python3 -I long_path.py`.

- long_path_<n>.db: one project whose main_repo_path is 20,000 bytes, one
  workspace of it, n sessions aliased to the workspace and n worktrees of
  the project, so every session and worktree row is joined to the path.

Pages are 512 bytes to keep the files small. No test runs this script;
rerun it after changing the SQL.
"""

import contextlib
import os
import sqlite3

SCHEMA = """
PRAGMA page_size = 512;
CREATE TABLE projects (id TEXT, name TEXT, container_kind TEXT, main_repo_path TEXT,
  github_owner TEXT, github_repo TEXT, default_branch TEXT, created_at TEXT);
CREATE TABLE workspaces (id TEXT, project_id TEXT, name TEXT, workspace_type TEXT, branch TEXT,
  created_at TEXT);
CREATE TABLE workspace_session_aliases (session_id TEXT, workspace_id TEXT, created_at TEXT);
CREATE TABLE sessions (id TEXT, title TEXT, session_type TEXT, model TEXT, auto_approve INTEGER,
  permission_mode TEXT, remote_control_enabled INTEGER, archived_at TEXT,
  forked_from_session_id TEXT, created_at TEXT);
CREATE TABLE worktrees (id TEXT, project_id TEXT, path TEXT, branch TEXT, base_branch TEXT,
  created_at TEXT);
INSERT INTO workspaces VALUES ('w', 'p', 'main', 'local', 'main', '2026-01-01T00:00:00Z');
"""

PATH_LENGTH = 20_000


def build(count: int) -> None:
    """Write long_path_<count>.db with count sessions and count worktrees."""
    name = f"long_path_{count}.db"
    if os.path.exists(name):
        os.remove(name)
    with contextlib.closing(sqlite3.connect(name)) as connection:
        connection.executescript(SCHEMA)
        connection.execute(
            "INSERT INTO projects VALUES ('p', 'app', 'repo', ?, NULL, NULL, 'main', '2026-01-01T00:00:00Z')",
            ("/" + "p" * (PATH_LENGTH - 1),),
        )
        for i in range(count):
            connection.execute(
                "INSERT INTO workspace_session_aliases VALUES (?, 'w', '2026-01-01T00:00:00Z')",
                (f"s{i}",),
            )
            connection.execute(
                "INSERT INTO sessions VALUES (?, 't', 'chat', 'm', 0, NULL, 0, NULL, NULL, '2026-01-01T00:00:00Z')",
                (f"s{i}",),
            )
            connection.execute(
                "INSERT INTO worktrees VALUES (?, 'p', '/w', 'b', NULL, '2026-01-01T00:00:00Z')",
                (f"t{i}",),
            )
        connection.commit()


if __name__ == "__main__":
    for size in (20, 40):
        build(size)
