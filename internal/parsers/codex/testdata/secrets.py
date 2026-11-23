"""Build the codex package's state database whose thread names a remote URL with credentials.

Run it from any directory:

    python3 -I internal/parsers/codex/testdata/secrets.py

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
create table threads (id text primary key, rollout_path text not null,
  created_at integer not null, updated_at integer not null, source text not null,
  model_provider text not null, cwd text not null, title text not null,
  sandbox_policy text not null, approval_mode text not null, git_origin_url text, model text);
insert into threads values ('t1', '/home/u/.codex/sessions/rollout-t1.jsonl', 1767323045, 1767323050,
  'cli', 'openai', '/w/proj', 'Fix the tests', '{}', 'on-request',
  'https://ci:s3cret@github.com/o/r.git', 'gpt-5');
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
