"""Build the tabby package's test databases.

Run it from any directory:

    python3 -I internal/parser/tabby/testdata/fixtures.py

Each database below is an ee/db.sqlite written by SQLite itself, so the Go tests read files
SQLite wrote. Pages are 512 bytes to keep the files small. No test runs this script; rerun it
after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

# A self-hosted integration whose api_base carries a credential.
INTEGRATIONS = """
PRAGMA page_size = 512;
CREATE TABLE integrations (id INTEGER PRIMARY KEY, kind TEXT, display_name TEXT,
    access_token TEXT, api_base TEXT, created_at TIMESTAMP);
INSERT INTO integrations VALUES (1, 'gitlab_self_hosted', 'work', 'glpat-TOKEN',
    'https://ci:s3cret@gitlab.example/api', '2026-10-01 09:00:00');
"""

DATABASES = {"integrations.db": INTEGRATIONS}


def main() -> None:
    """Rewrite every database of DATABASES in this directory."""
    for name, script in DATABASES.items():
        path = HERE / name
        path.unlink(missing_ok=True)
        with contextlib.closing(sqlite3.connect(path)) as connection:
            connection.executescript(script)
            connection.commit()


if __name__ == "__main__":
    main()
