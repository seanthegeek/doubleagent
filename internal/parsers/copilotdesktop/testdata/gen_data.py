"""Write the app database the copilotdesktop package's fuzz target starts from.

Run from this directory with any Python 3: `python3 -I gen_data.py`.

- data.db: one row in each of the tables the parser reads most, the
  project joined to the session through a workspace and its alias, and a
  NULL in each optional column of the session.
"""

import os
import sqlite3

NAME = "data.db"
TABLES = {
    "accounts": (
        "login TEXT, host TEXT, kind TEXT, is_default INTEGER, access_token TEXT, created_at TEXT",
        ("ana", "github.com", "user", 1, "x", "2026-01-01T00:00:00Z"),
    ),
    "projects": (
        "id TEXT, name TEXT, container_kind TEXT, main_repo_path TEXT, github_owner TEXT,"
        " github_repo TEXT, default_branch TEXT, created_at TEXT",
        ("p1", "app", "repo", "/work/app", "ana", "app", "main", "2026-01-01T00:00:01Z"),
    ),
    "workspaces": (
        "id TEXT, project_id TEXT, name TEXT, workspace_type TEXT, branch TEXT, created_at TEXT",
        ("w1", "p1", "main", "local", "main", "2026-01-01T00:00:02Z"),
    ),
    "workspace_session_aliases": (
        "session_id TEXT, workspace_id TEXT, created_at TEXT",
        ("s1", "w1", "2026-01-01T00:00:03Z"),
    ),
    "sessions": (
        "id TEXT, title TEXT, session_type TEXT, model TEXT, auto_approve INTEGER,"
        " permission_mode TEXT, remote_control_enabled INTEGER, archived_at TEXT,"
        " forked_from_session_id TEXT, created_at TEXT",
        ("s1", "fix it", "chat", "gpt-5", 0, None, 0, None, None, "2026-01-01T00:00:04Z"),
    ),
    "activity_items": (
        "session_id TEXT, activity_type TEXT, preview TEXT, created_at TEXT",
        ("s1", "message", "hello", "2026-01-01T00:00:05Z"),
    ),
}


def main() -> None:
    """Write data.db afresh."""
    if os.path.exists(NAME):
        os.remove(NAME)
    connection = sqlite3.connect(NAME)
    for table, (columns, row) in TABLES.items():
        connection.execute("CREATE TABLE %s (%s)" % (table, columns))
        marks = ", ".join("?" for _ in row)
        connection.execute("INSERT INTO %s VALUES (%s)" % (table, marks), row)
    connection.commit()
    connection.execute("VACUUM")
    connection.close()


if __name__ == "__main__":
    main()
