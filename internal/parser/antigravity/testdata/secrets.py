"""Build the antigravity package's summaries database whose workspace URI carries credentials.

Run it from any directory:

    python3 -I internal/parser/antigravity/testdata/secrets.py

secrets.db is written by SQLite from the SQL below. Pages are 512 bytes to keep the file small.
No test runs this script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

SECRETS = """
PRAGMA page_size = 512;
CREATE TABLE conversation_summaries (conversation_id text, title text NOT NULL DEFAULT "",
  last_modified_time datetime NOT NULL, workspace_uris text NOT NULL, PRIMARY KEY (conversation_id));
INSERT INTO conversation_summaries VALUES ('c1', 'Fix the tests', '2026-01-02T03:04:05Z',
  '["vscode-remote://ci:s3cret@devbox.example/w"]');
"""


def main() -> None:
    """Rewrite secrets.db in this directory."""
    path = HERE / "secrets.db"
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(SECRETS)
        connection.commit()


if __name__ == "__main__":
    main()
