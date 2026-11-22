"""Build the cursor package's database whose join keys are NULL on both sides.

Run it from any directory:

    python3 -I internal/parser/cursor/testdata/null_join.py

null_join.db is written by SQLite from the SQL below: a conversation whose fts_rowid is NULL
and a full-text content row whose id is NULL. SQL's = never finds NULL equal to NULL, so the
join of the two tables has no row. Pages are 512 bytes to keep the file small. No test runs
this script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

NULL_JOIN = """
PRAGMA page_size = 512;
CREATE TABLE conversations (fts_rowid, source TEXT, scope TEXT, id TEXT, title TEXT, branches TEXT,
  updated_at INTEGER, is_archived INTEGER);
CREATE TABLE conversation_fts_content (id, c0, c1, c2);
INSERT INTO conversations VALUES (NULL, 'local', NULL, 'c1', 'Fix the tests', NULL, 1767323045000, 0);
INSERT INTO conversation_fts_content VALUES (NULL, 'other', 'the body of another row', '');
"""

JOIN = (
    "SELECT count(*) FROM conversations"
    " JOIN conversation_fts_content AS content ON fts_rowid = content.id"
)


def main() -> None:
    """Rewrite the database in this directory and check that the join is empty."""
    path = HERE / "null_join.db"
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(NULL_JOIN)
        connection.commit()
        if connection.execute(JOIN).fetchone() != (0,):
            raise SystemExit("the join of the NULL keys gave a row")


if __name__ == "__main__":
    main()
