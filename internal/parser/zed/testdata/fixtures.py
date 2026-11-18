"""Build the zed package's test databases.

Run it with the analyzer's interpreter, which has zstandard, from any directory:

    analyzer/.venv/bin/python -I internal/parser/zed/testdata/fixtures.py

Each database below is a threads.db written by SQLite itself with the agent's own schema, so the
Go tests read files SQLite wrote. Pages are 512 bytes to keep the files small. No test runs this
script; rerun it after changing the rows.
"""

from __future__ import annotations

import contextlib
import json
import sqlite3
from pathlib import Path

import zstandard

HERE = Path(__file__).resolve().parent

SCHEMA = """
PRAGMA page_size = 512;
CREATE TABLE threads (id TEXT PRIMARY KEY, summary TEXT NOT NULL, updated_at TEXT NOT NULL,
  data_type TEXT NOT NULL, data BLOB NOT NULL);
"""

# The threads of the damage fixtures.
THREAD_COUNT = 60


def thread(number: int) -> tuple[str, str, str, str, bytes]:
    """Return a json thread row; updated_at runs against rowid order, so sorting shows."""
    blob = json.dumps({"version": "0.3.0", "title": f"thread {number:03d}", "messages": []})
    updated = f"2026-03-01T09:{59 - number % 60:02d}:00.000000000+00:00"
    return f"t{number:03d}", f"thread {number:03d}", updated, "json", blob.encode()


def large_thread(number: int) -> tuple[str, str, str, str, bytes]:
    """Return a zstd thread row whose blob decompresses to exactly 40000 bytes."""
    text = json.dumps({"version": "0.3.0", "title": f"large {number}", "messages": [], "pad": ""})
    padded = text[:-2] + "x" * (40000 - len(text)) + '"}'
    blob = zstandard.ZstdCompressor(level=3).compress(padded.encode())
    updated = f"2026-03-01T10:0{number}:00.000000000+00:00"
    return f"z{number}", f"large {number}", updated, "zstd", blob


def write(name: str, rows: list[tuple[str, str, str, str, bytes]]) -> Path:
    """Write a threads.db holding rows and return its path."""
    path = HERE / name
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(SCHEMA)
        connection.executemany("INSERT INTO threads VALUES (?, ?, ?, ?, ?)", rows)
        connection.commit()
    return path


def damage(path: Path) -> None:
    """Overwrite the threads table's leaf page two thirds of the way along with bytes 0x8f."""
    with contextlib.closing(sqlite3.connect(path)) as connection:
        query = "SELECT pageno FROM dbstat WHERE name = 'threads' AND pagetype = 'leaf'"
        leaves = sorted(page for (page,) in connection.execute(query))
    page = leaves[len(leaves) * 2 // 3]
    data = bytearray(path.read_bytes())
    data[(page - 1) * 512 : page * 512] = b"\x8f" * 512
    path.write_bytes(bytes(data))


def main() -> None:
    """Rewrite every database in this directory."""
    threads = [thread(number) for number in range(THREAD_COUNT)]
    write("threads.db", threads)
    damage(write("damaged_threads.db", threads))
    write("large_threads.db", [large_thread(number) for number in range(3)])


if __name__ == "__main__":
    main()
