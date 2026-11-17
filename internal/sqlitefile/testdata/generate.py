"""Build the sqlitefile package's test databases and the rows SQLite reads from them.

Run it with the analyzer's interpreter, from any directory:

    analyzer/.venv/bin/python -I internal/sqlitefile/testdata/generate.py

It rewrites every database, write-ahead log (`-wal`) and rollback journal
(`-journal`) in this directory, then opens a scratch copy of each database
with its sidecars, as the analyzer's `open_copy` does, and writes what the
stdlib `sqlite3` module reads to `expected.txt`. The Go tests compare the
reader with that file, so the expected values come from SQLite itself.

Logs and journals carry random salts and nonces, so a rerun changes their
bytes but not the expected rows. Every table is declared without column
types and without an INTEGER PRIMARY KEY, so the values SQL returns are the
values stored in each record.

`expected.txt` has one `db <file>` line per database, then `schema` lines
(type, name, table name, root page, SQL as hex), `tree <table|index> <name>`
lines each followed by its `row <rowid or -> <values>` lines in b-tree order,
and an `error <text>` line where SQLite stopped: after the `db` line when it
cannot read the schema, after a `tree` line when it cannot read that tree. A value is
`null`, `int:<decimal>`, `float:<hex of the 8 IEEE bytes>`, `text:<hex>` or
`blob:<hex>`; a text or blob over 64 bytes is `text:<length>:<SHA-256 in hex>`.
"""

from __future__ import annotations

import hashlib
import shutil
import sqlite3
import struct
import tempfile
from collections.abc import Callable
from pathlib import Path

HERE = Path(__file__).resolve().parent
JOURNAL_MAGIC = bytes.fromhex("d9d505f920a163d7")
LOG_HEADER_SIZE = 32
LOG_FRAME_HEADER_SIZE = 24

# Per database, the trees to dump: (kind, name, query, record length or None).
# A table query returns the rowid then the record's values; an index query
# returns the key's values. The record length, when given, maps a rowid to
# how many values its record holds (rows written before an ALTER TABLE).
Length = Callable[[int], int] | None
Tree = tuple[str, str, str, Length]
TREES: dict[str, list[Tree]] = {}


def connect(path: Path, page_size: int, **pragmas: str) -> sqlite3.Connection:
    """Create a database at `path` with the page size and the pragmas given."""
    path.unlink(missing_ok=True)
    connection = sqlite3.connect(path, isolation_level=None)
    connection.execute("PRAGMA page_size=%d" % page_size)
    for name, value in pragmas.items():
        connection.execute("PRAGMA %s=%s" % (name, value))
    return connection


def every_serial_type() -> list[tuple]:
    """Rows of (a, b, c) holding a value of every record serial type."""
    integers = [0, 1, -1, 127, -128, 32767, -32768, 8388607, -8388608, 2**31 - 1, -(2**31)]
    integers += [2**47 - 1, -(2**47), 2**63 - 1, -(2**63)]
    rows: list[tuple] = [(value, None, "integer") for value in integers]
    rows += [(1.5, -0.0, 1e300), (float("inf"), float("-inf"), 2.0)]
    rows += [("", b"", "empty text and blob"), ("café \U0001f600", b"\x00\xff", None)]
    return rows


def build_page_size(page_size: int) -> str:
    """A database of `page_size` holding every serial type and, below 65536, overflow."""
    name = "page_%d.db" % page_size
    connection = connect(HERE / name, page_size)
    connection.execute("CREATE TABLE t(a, b, c)")
    connection.executemany("INSERT INTO t VALUES (?, ?, ?)", every_serial_type())
    if page_size < 65536:
        for size in (page_size - 36, page_size, page_size * 3 + 7):
            connection.execute("INSERT INTO t VALUES (?, ?, ?)", ("x" * size, b"\x01" * size, size))
    connection.close()
    TREES[name] = [("table", "t", "SELECT rowid, a, b, c FROM t", None)]
    return name


def build_shapes() -> str:
    """Rowid extremes, an ALTERed table, WITHOUT ROWID, and indexes with overflow keys."""
    name = "shapes.db"
    connection = connect(HERE / name, 1024)
    connection.execute("CREATE TABLE ids(a)")
    for rowid in (-(2**63), -1, 0, 1, 2**31, 2**63 - 1):
        connection.execute("INSERT INTO ids(rowid, a) VALUES (?, ?)", (rowid, rowid))
    connection.execute("CREATE TABLE altered(a, b)")
    connection.executemany("INSERT INTO altered VALUES (?, ?)", [(1, "one"), (2, "two")])
    connection.execute("ALTER TABLE altered ADD COLUMN c DEFAULT 'later'")
    connection.execute("INSERT INTO altered VALUES (3, 'three', 'set')")
    connection.execute("CREATE TABLE w(k PRIMARY KEY, v) WITHOUT ROWID")
    keys = ["key %03d " % number + "k" * (number * 37 % 900) for number in range(25)]
    connection.executemany("INSERT INTO w VALUES (?, ?)", [(key, len(key)) for key in keys])
    connection.execute("CREATE TABLE notes(a, b)")
    values = [(number % 7 - 3, "n" * (number * 53 % 1200)) for number in range(30)]
    connection.executemany("INSERT INTO notes VALUES (?, ?)", values)
    connection.execute("CREATE INDEX notes_b ON notes(b)")
    connection.execute("CREATE INDEX notes_a ON notes(a)")
    connection.execute("CREATE VIEW notes_view AS SELECT a FROM notes")
    connection.close()
    TREES[name] = [
        ("table", "ids", "SELECT rowid, a FROM ids", None),
        (
            "table",
            "altered",
            "SELECT rowid, a, b, c FROM altered",
            lambda rowid: 2 if rowid < 3 else 3,
        ),
        ("index", "w", "SELECT k, v FROM w ORDER BY k", None),
        ("table", "notes", "SELECT rowid, a, b FROM notes", None),
        ("index", "notes_b", "SELECT b, rowid FROM notes ORDER BY b, rowid", None),
        ("index", "notes_a", "SELECT a, rowid FROM notes ORDER BY a, rowid", None),
    ]
    return name


def build_deep() -> str:
    """A 512-byte-page table and index deep enough for interior pages over interior pages."""
    name = "deep.db"
    connection = connect(HERE / name, 512)
    connection.execute("CREATE TABLE t(a, b)")
    rows = [
        ((number * 7919) % 3001 - 1500, "row %05d " % number + "d" * 20) for number in range(1000)
    ]
    connection.execute("BEGIN")
    connection.executemany("INSERT INTO t VALUES (?, ?)", rows)
    connection.execute("CREATE INDEX t_b ON t(b)")
    connection.execute("COMMIT")
    connection.close()
    TREES[name] = [
        ("table", "t", "SELECT rowid, a, b FROM t", None),
        ("index", "t_b", "SELECT b, rowid FROM t ORDER BY b, rowid", None),
    ]
    return name


def build_vacuum_modes() -> list[str]:
    """Databases with pointer-map pages (both auto-vacuum modes) and one with a freelist."""
    names = []
    for name, mode in (
        ("autovacuum.db", "FULL"),
        ("incremental.db", "INCREMENTAL"),
        ("freelist.db", "NONE"),
    ):
        connection = connect(HERE / name, 1024, auto_vacuum=mode)
        connection.execute("CREATE TABLE t(a, b)")
        connection.executemany(
            "INSERT INTO t VALUES (?, ?)", [(number, "v" * 900) for number in range(30)]
        )
        connection.execute("CREATE TABLE u(a)")
        connection.executemany("INSERT INTO u VALUES (?)", [("u" * 3000,), ("short",)])
        connection.execute("DELETE FROM t WHERE a % 3 = 0")
        connection.close()
        TREES[name] = [
            ("table", "t", "SELECT rowid, a, b FROM t", None),
            ("table", "u", "SELECT rowid, a FROM u", None),
        ]
        names.append(name)
    return names


def build_utf16() -> str:
    """A UTF-16le database, which the reader refuses."""
    name = "utf16le.db"
    connection = connect(HERE / name, 512, encoding="'UTF-16le'")
    connection.execute("CREATE TABLE t(a)")
    connection.execute("INSERT INTO t VALUES ('text')")
    connection.close()
    TREES[name] = [("table", "t", "SELECT rowid, a FROM t", None)]
    return name


def copy_open(source: Path, target: Path, suffixes: tuple[str, ...]) -> None:
    """Copy `source` and its sidecars to `target` while a connection still holds them."""
    for suffix in ("", *suffixes):
        if Path(str(source) + suffix).exists():
            shutil.copyfile(str(source) + suffix, str(target) + suffix)


def log_checksum(data: bytes, big_endian: bool, first: int, second: int) -> tuple[int, int]:
    """The write-ahead log's running checksum over `data`, continued from (first, second)."""
    words = struct.unpack(("<" if not big_endian else ">") + "%dI" % (len(data) // 4), data)
    for position in range(0, len(words), 2):
        first = (first + words[position] + second) & 0xFFFFFFFF
        second = (second + words[position + 1] + first) & 0xFFFFFFFF
    return first, second


def reseal_log(log: bytearray) -> bytearray:
    """Recompute the header checksum and every frame's running checksum of `log`."""
    big_endian = struct.unpack(">I", log[:4])[0] == 0x377F0683
    page_size = struct.unpack(">I", log[8:12])[0]
    first, second = log_checksum(bytes(log[:24]), big_endian, 0, 0)
    struct.pack_into(">II", log, 24, first, second)
    offset = LOG_HEADER_SIZE
    while offset + LOG_FRAME_HEADER_SIZE + page_size <= len(log):
        first, second = log_checksum(bytes(log[offset : offset + 8]), big_endian, first, second)
        data = log[offset + LOG_FRAME_HEADER_SIZE : offset + LOG_FRAME_HEADER_SIZE + page_size]
        first, second = log_checksum(bytes(data), big_endian, first, second)
        struct.pack_into(">II", log, offset + 16, first, second)
        offset += LOG_FRAME_HEADER_SIZE + page_size
    return log


def build_log_base(scratch: Path) -> tuple[bytes, bytes]:
    """A database whose schema is in the main file and whose rows are only in its log."""
    source = scratch / "log.db"
    connection = connect(source, 512, journal_mode="WAL", wal_autocheckpoint="0")
    connection.execute("CREATE TABLE t(a)")
    connection.execute("PRAGMA wal_checkpoint(TRUNCATE)")
    for batch in ([1, 2], [3], [4, 5]):
        connection.execute("BEGIN")
        connection.executemany("INSERT INTO t VALUES (?)", [(value,) for value in batch])
        connection.execute("COMMIT")
    main = source.read_bytes()
    log = Path(str(source) + "-wal").read_bytes()
    connection.close()
    return main, log


def write_pair(
    name: str, main: bytes, log: bytes | None = None, journal: bytes | None = None
) -> str:
    """Write a database and its sidecars under `name`, with a dump of table t."""
    (HERE / name).write_bytes(main)
    if log is not None:
        Path(str(HERE / name) + "-wal").write_bytes(log)
    if journal is not None:
        Path(str(HERE / name) + "-journal").write_bytes(journal)
    TREES[name] = [("table", "t", "SELECT rowid, * FROM t", None)]
    return name


def build_log_variants(scratch: Path) -> list[str]:
    """Logs SQLite honours in part, in whole or not at all."""
    main, log = build_log_base(scratch)
    page_size = struct.unpack(">I", log[8:12])[0]
    frame_size = LOG_FRAME_HEADER_SIZE + page_size
    frames = (len(log) - LOG_HEADER_SIZE) // frame_size
    last = LOG_HEADER_SIZE + (frames - 1) * frame_size

    def changed(edit: Callable[[bytearray], None], reseal: bool = True) -> bytes:
        copy = bytearray(log)
        edit(copy)
        return bytes(reseal_log(copy) if reseal else copy)

    def flip(offset: int) -> Callable[[bytearray], None]:
        return lambda data: data.__setitem__(offset, data[offset] ^ 1)

    legacy = bytearray(main)
    legacy[18:20] = b"\x01\x01"
    return [
        write_pair("log_rows.db", main, log),
        write_pair("log_torn_frame.db", main, log[: last + 100]),
        write_pair(
            "log_uncommitted.db",
            main,
            changed(lambda data: struct.pack_into(">I", data, last + 4, 0)),
        ),
        write_pair(
            "log_bad_frame.db", main, changed(flip(LOG_HEADER_SIZE + 24 + 50), reseal=False)
        ),
        write_pair("log_bad_salt.db", main, changed(flip(LOG_HEADER_SIZE + 8), reseal=False)),
        write_pair("log_bad_header.db", main, changed(flip(24), reseal=False)),
        write_pair(
            "log_frame_page_zero.db",
            main,
            changed(lambda data: struct.pack_into(">I", data, last, 0)),
        ),
        write_pair(
            "log_big_endian.db",
            main,
            changed(lambda data: struct.pack_into(">I", data, 0, 0x377F0683)),
        ),
        write_pair(
            "log_bad_page_size.db",
            main,
            changed(lambda data: struct.pack_into(">I", data, 8, 1000)),
        ),
        write_pair(
            "log_other_page_size.db",
            main,
            changed(lambda data: struct.pack_into(">I", data, 8, 1024)),
        ),
        write_pair(
            "log_version.db", main, changed(lambda data: struct.pack_into(">I", data, 4, 3007001))
        ),
        write_pair(
            "log_shrinks.db", main, changed(lambda data: struct.pack_into(">I", data, last + 4, 1))
        ),
        write_pair("log_legacy_header.db", bytes(legacy), log),
        write_pair("log_empty_main.db", b"", log),
    ]


def build_log_fresh_and_reset(scratch: Path) -> list[str]:
    """A log holding page 1 itself, and a reset log with stale frames after the live ones."""
    source = scratch / "fresh.db"
    connection = connect(source, 512, journal_mode="WAL", wal_autocheckpoint="0")
    connection.execute("CREATE TABLE t(a)")
    connection.execute("INSERT INTO t VALUES ('fresh')")
    copy_open(source, HERE / "log_fresh.db", ("-wal",))
    connection.close()
    source = scratch / "reset.db"
    connection = connect(source, 512, journal_mode="WAL", wal_autocheckpoint="0")
    connection.execute("CREATE TABLE t(a)")
    connection.executemany(
        "INSERT INTO t VALUES (?)", [("old %d" % number,) for number in range(20)]
    )
    connection.execute("PRAGMA wal_checkpoint(PASSIVE)")
    connection.execute("INSERT INTO t VALUES ('after reset')")
    copy_open(source, HERE / "log_reset.db", ("-wal",))
    connection.close()
    for name in ("log_fresh.db", "log_reset.db"):
        TREES[name] = [("table", "t", "SELECT rowid, * FROM t", None)]
    return ["log_fresh.db", "log_reset.db"]


def build_hot_journal(scratch: Path) -> tuple[bytes, bytes]:
    """A main file torn mid-transaction and its rollback journal."""
    source = scratch / "hot.db"
    connection = connect(source, 512)
    connection.execute("CREATE TABLE t(a, b)")
    connection.execute("BEGIN")
    connection.executemany(
        "INSERT INTO t VALUES (?, ?)", [(number, "h" * 60) for number in range(12)]
    )
    connection.execute("COMMIT")
    connection.execute("PRAGMA cache_size=2")
    connection.execute("BEGIN")
    connection.executemany(
        "INSERT INTO t VALUES (?, ?)", [(number, "y" * 150) for number in range(100, 130)]
    )
    connection.execute("UPDATE t SET b = 'changed' WHERE a < 5")
    main = source.read_bytes()
    journal = Path(str(source) + "-journal").read_bytes()
    connection.execute("ROLLBACK")
    connection.close()
    return main, journal


def super_journal_block(name: bytes, page_size: int, good: bool) -> bytes:
    """The record naming a super-journal that ends a journal, with a right or wrong checksum."""
    lock_page = 1073741824 // page_size + 1
    checksum = (sum(name) & 0xFFFFFFFF) ^ (0 if good else 1)
    return (
        struct.pack(">I", lock_page)
        + name
        + struct.pack(">II", len(name), checksum)
        + JOURNAL_MAGIC
    )


def build_journal_variants(scratch: Path) -> list[str]:
    """Hot journals SQLite plays back in whole, in part or not at all."""
    main, journal = build_hot_journal(scratch)
    sector = struct.unpack(">I", journal[20:24])[0]
    page_size = struct.unpack(">I", journal[24:28])[0]

    def changed(offset: int, value: int) -> bytes:
        copy = bytearray(journal)
        struct.pack_into(">I", copy, offset, value)
        return bytes(copy)

    def flipped(offset: int) -> bytes:
        copy = bytearray(journal)
        copy[offset] ^= 1
        return bytes(copy)

    missing = b"/nonexistent/sqlitefile/hot.db-mj0A1B2C3D"
    return [
        write_pair("journal_hot.db", main, journal=journal),
        write_pair("journal_torn_alone.db", main),
        write_pair("journal_first_byte_zero.db", main, journal=b"\x00" + journal[1:]),
        write_pair("journal_bad_magic.db", main, journal=flipped(1)),
        write_pair("journal_count_zero.db", main, journal=changed(8, 0)),
        write_pair("journal_count_all.db", main, journal=changed(8, 0xFFFFFFFF)),
        write_pair("journal_bad_checksum.db", main, journal=flipped(sector + 4 + page_size)),
        write_pair("journal_one_record.db", main, journal=journal[: sector + page_size + 8]),
        write_pair("journal_short.db", main, journal=changed(16, 1)[:100]),
        write_pair("journal_shrinks.db", main, journal=changed(16, 1)),
        write_pair("journal_grows.db", main, journal=changed(16, 400)),
        write_pair("journal_page_size_zero.db", main, journal=changed(24, 0)),
        write_pair("journal_bad_sector.db", main, journal=changed(20, 131072)),
        write_pair(
            "journal_super.db",
            main,
            journal=journal + super_journal_block(missing, page_size, True),
        ),
        write_pair(
            "journal_super_bad_checksum.db",
            main,
            journal=journal + super_journal_block(missing, page_size, False),
        ),
        write_pair("journal_empty_main.db", b"", journal=journal),
    ]


def content(kind: str, data: bytes) -> str:
    """A text or blob value: its bytes in hex, or past 64 bytes its length and SHA-256."""
    if len(data) <= 64:
        return "%s:%s" % (kind, data.hex())
    return "%s:%d:%s" % (kind, len(data), hashlib.sha256(data).hexdigest())


def describe(value: object) -> str:
    """One value in the `expected.txt` form."""
    if value is None:
        return "null"
    if isinstance(value, int):
        return "int:%d" % value
    if isinstance(value, float):
        return "float:" + struct.pack(">d", value).hex()
    if isinstance(value, str):
        return content("text", value.encode("utf-8"))
    if isinstance(value, bytes):
        return content("blob", value)
    raise TypeError(type(value))


def dump_tree(connection: sqlite3.Connection, kind: str, query: str, length: Length) -> list[str]:
    """The `row` lines of one tree, in the order the query returns them."""
    lines = []
    for row in connection.execute(query).fetchall():
        values = list(row)
        rowid = "-"
        if kind == "table":
            rowid = str(values.pop(0))
            if length is not None:
                values = values[: length(int(rowid))]
        lines.append(" ".join(["row", rowid, *map(describe, values)]))
    return lines


def dump(name: str, scratch: Path) -> list[str]:
    """What SQLite reads from a scratch copy of database `name` and its sidecars."""
    target = scratch / ("read-" + name)
    copy_open(HERE / name, target, ("-wal", "-journal"))
    lines = ["db " + name]
    connection = sqlite3.connect(target)
    try:
        schema = connection.execute("SELECT type, name, tbl_name, rootpage, sql FROM sqlite_master")
        for kind, object_name, table, root, sql in schema.fetchall():
            text = (sql or "").encode("utf-8").hex() or "-"
            lines.append("schema %s %s %s %d %s" % (kind, object_name, table, root or 0, text))
        for kind, tree, query, length in TREES[name]:
            lines.append("tree %s %s" % (kind, tree))
            lines += dump_tree(connection, kind, query, length)
    except sqlite3.Error as error:
        lines.append("error " + str(error))
    finally:
        connection.close()
    return lines


def main() -> None:
    """Rebuild every test database and `expected.txt`."""
    for stale in HERE.iterdir():
        if ".db" in stale.name:
            stale.unlink()
    names = [build_page_size(size) for size in (512, 1024, 4096, 65536)]
    names += [build_shapes(), build_deep(), *build_vacuum_modes(), build_utf16()]
    with tempfile.TemporaryDirectory() as directory:
        scratch = Path(directory)
        names += build_log_variants(scratch)
        names += build_log_fresh_and_reset(scratch)
        names += build_journal_variants(scratch)
        lines = ["# Written by generate.py from SQLite %s; do not edit." % sqlite3.sqlite_version]
        for name in names:
            lines += dump(name, scratch)
    (HERE / "expected.txt").write_text("\n".join(lines) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
