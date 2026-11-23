"""Build the cline package's search indexes whose session ids are case spellings of one id.

Run it from any directory:

    python3 -I internal/parsers/cline/testdata/search_spellings.py

search_spellings_<n>.db is a db/session-search.db whose indexed_sessions table holds n rows, for
n of 20 and 40, each a different spelling in ASCII case of the session id "abcdefgh". Pages are
512 bytes to keep the files small. No test runs this script; rerun it after changing the SQL.
"""

from __future__ import annotations

import contextlib
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

SESSION_ID = "abcdefgh"


def spelling(number: int) -> str:
    """Return the session id with the letters whose bit is set in number in capitals."""
    return "".join(
        letter.upper() if number >> place & 1 else letter for place, letter in enumerate(SESSION_ID)
    )


def build(path: Path, rows: int) -> None:
    """Write the search index of rows spellings at path."""
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(
            "PRAGMA page_size = 512;"
            "CREATE TABLE indexed_sessions (session_id TEXT PRIMARY KEY, title TEXT,"
            " document_count INTEGER, indexed_at INTEGER);"
        )
        connection.executemany(
            "INSERT INTO indexed_sessions VALUES (?, 't', 1, 1)",
            [(spelling(number),) for number in range(rows)],
        )
        connection.commit()


def main() -> None:
    """Rewrite both databases in this directory."""
    for rows in (20, 40):
        build(HERE / f"search_spellings_{rows}.db", rows)


if __name__ == "__main__":
    main()
