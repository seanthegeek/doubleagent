"""Build the openclaw package's test databases.

Run it from any directory:

    python3 -I internal/parser/openclaw/testdata/fixtures.py

Each database below is an openclaw-agent.sqlite written by SQLite itself, so the Go tests read
files SQLite wrote. The cold archive tables have the agent's own schema. Pages are 512 bytes to
keep the files small. No test runs this script; rerun it after changing the rows.
"""

from __future__ import annotations

import contextlib
import json
import sqlite3
from pathlib import Path

HERE = Path(__file__).resolve().parent

SCHEMA = """
PRAGMA page_size = 512;
CREATE TABLE session_transcript_cold_archives (session_id TEXT NOT NULL PRIMARY KEY,
  generation TEXT NOT NULL, archive_name TEXT NOT NULL UNIQUE, archive_sha256 TEXT NOT NULL,
  event_count INTEGER NOT NULL, raw_bytes INTEGER NOT NULL, archive_bytes INTEGER NOT NULL,
  last_seq INTEGER NOT NULL, archived_at INTEGER NOT NULL, storage TEXT NOT NULL,
  archive_blob BLOB);
CREATE TABLE session_transcript_cold_archive_chunks (session_id TEXT NOT NULL,
  chunk_index INTEGER NOT NULL, archive_name TEXT NOT NULL UNIQUE, archive_sha256 TEXT NOT NULL,
  first_seq INTEGER NOT NULL, last_seq INTEGER NOT NULL, event_count INTEGER NOT NULL,
  raw_bytes INTEGER NOT NULL, archive_bytes INTEGER NOT NULL, archive_blob BLOB,
  PRIMARY KEY (session_id, chunk_index)) WITHOUT ROWID;
"""


def archive(session: str, name: str, archived_at: int, blob: bytes | None) -> tuple:
    """Return a sqlite-stored cold archive row."""
    return (session, "g1", name, "0" * 64, 1, 0, 0, 1, archived_at, "sqlite", blob)


def chunk(session: str, index: int, name: str, blob: bytes | None) -> tuple:
    """Return a chunk row of a session's cold archive."""
    return (session, index, name, "0" * 64, index, index, 1, 0, 0, blob)


def cold_jsonl(session: str, text: str) -> bytes:
    """Return a cold archive's JSONL: its header and one user message event."""
    entry = {
        "type": "message",
        "id": text,
        "timestamp": "2026-10-01T09:00:01.000Z",
        "message": {"role": "user", "content": [{"type": "text", "text": text}]},
    }
    records = [
        {"kind": "header", "version": 1, "sessionId": session},
        {"kind": "event", "row": {"seq": 0, "event_json": json.dumps(entry), "created_at": 1}},
    ]
    return "".join(json.dumps(record) + "\n" for record in records).encode()


def many(count: int) -> tuple[list[tuple], list[tuple]]:
    """Return count archives of distinct sessions, each with one chunk of no bytes."""
    archives = [archive(f"s{number}", f"a{number}", number, None) for number in range(count)]
    chunks = [chunk(f"s{number}", 0, f"c{number}", b"") for number in range(count)]
    return archives, chunks


def grouped() -> tuple[list[tuple], list[tuple]]:
    """Return two chunked archives, chunks stored out of order, and one with its own bytes."""
    archives = [
        archive("sa", "archive-a", 1, None),
        archive("sb", "archive-b", 2, None),
        archive("sc", "archive-c", 3, cold_jsonl("sc", "c whole")),
    ]
    chunks = [
        chunk("sb", 1, "b1", cold_jsonl("sb", "b second")),
        chunk("sa", 1, "a1", cold_jsonl("sa", "a second")),
        chunk("sb", 0, "b0", cold_jsonl("sb", "b first")),
        chunk("sa", 0, "a0", cold_jsonl("sa", "a first")),
        chunk("sz", 0, "z0", cold_jsonl("sz", "no archive")),
    ]
    return archives, chunks


DATABASES = {
    "cold_archives_2.db": many(2),
    "cold_archives_8.db": many(8),
    "cold_chunks.db": grouped(),
}


def main() -> None:
    """Rewrite every database of DATABASES in this directory."""
    for name, (archives, chunks) in DATABASES.items():
        path = HERE / name
        path.unlink(missing_ok=True)
        with contextlib.closing(sqlite3.connect(path)) as connection:
            connection.executescript(SCHEMA)
            connection.executemany(
                "INSERT INTO session_transcript_cold_archives VALUES (?,?,?,?,?,?,?,?,?,?,?)",
                archives,
            )
            connection.executemany(
                "INSERT INTO session_transcript_cold_archive_chunks VALUES (?,?,?,?,?,?,?,?,?,?)",
                chunks,
            )
            connection.commit()


if __name__ == "__main__":
    main()
