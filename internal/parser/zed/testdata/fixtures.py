"""Build the zed package's test databases.

Run it with a python3 that has the zstandard package, from any directory:

    python3 -I internal/parser/zed/testdata/fixtures.py

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

# The thread counts of the databases whose every blob announces a 128 MiB window.
WIDE_WINDOW_COUNTS = (200, 800)


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


def wide_window_thread(number: int) -> tuple[str, str, str, str, bytes]:
    """Return a zstd thread row whose frame announces a 128 MiB window for a small thread.

    The frame is written by hand: the magic number, a descriptor with no content size, the window
    byte 0x88 (2^27 bytes) and one raw block holding the thread's JSON.
    """
    text = json.dumps({"version": "0.3.0", "title": f"wide {number}", "messages": []}).encode()
    block = ((len(text) << 3) | 1).to_bytes(3, "little")
    blob = b"\x28\xb5\x2f\xfd\x00\x88" + block + text
    updated = f"2026-03-01T11:{number // 60 % 60:02d}:{number % 60:02d}.000000000+00:00"
    return f"w{number:04d}", f"wide {number}", updated, "zstd", blob


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
    for count in WIDE_WINDOW_COUNTS:
        write(f"wide_window_{count}.db", [wide_window_thread(number) for number in range(count)])


if __name__ == "__main__":
    main()
