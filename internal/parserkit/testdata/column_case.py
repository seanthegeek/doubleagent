"""Build the parser kit's database whose columns are spelled in mixed case.

Run it from any directory:

    python3 -I internal/parserkit/testdata/column_case.py

column_case.db is written by SQLite from the SQL below: one table whose columns are spelled
Name, VALUE and kind, and one row. Pages are 512 bytes to keep the file small. No test runs this
script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

COLUMN_CASE = """
PRAGMA page_size = 512;
CREATE TABLE things (Name TEXT, VALUE INTEGER, kind TEXT);
INSERT INTO things VALUES ('a', 7, 'k');
"""


def main() -> None:
    """Rewrite column_case.db in this directory."""
    path = HERE / "column_case.db"
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(COLUMN_CASE)
        connection.commit()


if __name__ == "__main__":
    main()
