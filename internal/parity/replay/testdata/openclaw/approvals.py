"""Build the replay package's OpenClaw state database of exec approvals.

Run it from any directory:

    python3 -I internal/parity/replay/testdata/openclaw/approvals.py

The database is a state/openclaw.sqlite written by SQLite itself, holding three exec approvals rows:
settings with a socket token, a token alone, and settings without one, so a test can tell which rows
a token row belongs to. Pages are 512 bytes to keep the file small. No test runs this script; rerun
it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

APPROVALS = """
PRAGMA page_size = 512;
CREATE TABLE exec_approvals_config (config_key TEXT NOT NULL PRIMARY KEY, raw_json TEXT NOT NULL,
  has_socket_token INTEGER NOT NULL, updated_at_ms INTEGER NOT NULL);
INSERT INTO exec_approvals_config VALUES ('default', '{"socket": {"token": "FIXTURE-SOCKET-TOKEN"},
  "defaults": {"security": "allowlist", "ask": "on-miss"}}', 1, 1790845100000);
INSERT INTO exec_approvals_config VALUES ('bare', '{"socket": {"token": "FIXTURE-SOCKET-TOKEN"}}', 1,
  1790845200000);
INSERT INTO exec_approvals_config VALUES ('plain', '{"defaults": {"security": "deny"}}', 0, 1790845300000);
"""


def main() -> None:
    """Rewrite approvals.db in this directory."""
    path = HERE / "approvals.db"
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(APPROVALS)
        connection.commit()


if __name__ == "__main__":
    main()
