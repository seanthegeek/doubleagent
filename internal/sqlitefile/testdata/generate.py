"""Build the sqlitefile package's test databases and the rows SQLite reads from them.

Run it with the analyzer's interpreter, from any directory:

    analyzer/.venv/bin/python -I internal/sqlitefile/testdata/generate.py

It rewrites every database, write-ahead log (`-wal`) and rollback journal
(`-journal`) in this directory, then opens a scratch copy of each database
with its sidecars, as the analyzer's `open_copy` does, and writes what the
stdlib `sqlite3` module reads to `expected.txt`. The Go tests compare the
reader with that file, so the expected values come from SQLite itself.

Logs and journals carry random salts and nonces, so a rerun changes their
bytes but not the expected rows. Every table with trees in expected.txt is
declared without column types and without an INTEGER PRIMARY KEY, so the
values SQL returns are the values stored in each record. schema.db, sites.db
and sites_hermes.db hold typed tables and the tables of the parsers' queries;
what SQL returns from them (`dump_queries`, also used by fixture_tables.py)
goes to `tables.txt`.

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


# Rowid alias declarations; each table holds one row inserted with rowid 7
# and id 3, so the dump shows which declarations make id the rowid.
ALIAS_FORMS = [
    "id INTEGER PRIMARY KEY, v",
    "id integer primary key, v",
    "id INTEGER PRIMARY KEY ASC, v",
    "id INTEGER PRIMARY KEY DESC, v",
    "id INTEGER, v, PRIMARY KEY(id)",
    "id INTEGER, v, PRIMARY KEY(id DESC)",
    "id INT PRIMARY KEY, v",
    "id BIGINT PRIMARY KEY, v",
    "id INTEGER NOT NULL PRIMARY KEY, v",
    "id INTEGER PRIMARY KEY AUTOINCREMENT, v",
    '"id" "INTEGER" PRIMARY KEY, v',
    "id [INTEGER] PRIMARY KEY, v",
    "id INTEGER(10) PRIMARY KEY, v",
    "id UNSIGNED INTEGER PRIMARY KEY, v",
    "id, v, PRIMARY KEY(id)",
    "id INTEGER, v INTEGER, PRIMARY KEY(id, v)",
    "id INTEGER CONSTRAINT pk PRIMARY KEY ON CONFLICT REPLACE, v",
    "id 'INTEGER' PRIMARY KEY, v",
    "id INTEGER, v, PRIMARY KEY(ID COLLATE nocase)",
    "id INTEGER COLLATE binary PRIMARY KEY, v",
]

# DEFAULT clauses for ALTER TABLE ADD COLUMN, each added under every type, so
# the rows written before read the default with each affinity applied.
ADDED_DEFAULTS = [
    "5",
    "-5",
    "+5",
    "1.5",
    "-1.50",
    "1e3",
    "-1e3",
    "0x10",
    "0xffffffffffffffff",
    "'x'",
    "X'00ff'",
    "NULL",
    "TRUE",
    "false",
    "(1)",
    "'5'",
    "'1e3'",
    '"q"',
    "-'5'",
    "+'5'",
    "- 3",
    "9223372036854775808",
    "-9223372036854775808",
    "1.",
    ".5",
    "-0.0",
    "1_000",
    "007",
    "(-(5))",
    "(CAST(1 AS TEXT))",
    "abc",
]
ADDED_TYPES = ["", "INTEGER", "TEXT", "REAL", "NUMERIC", "BLOB"]

# An original column list whose defaults only a short record reads: SQLite
# writes no such record for these columns, so the schema text is rewritten
# after the row is written.
WIDENED_SQL = (
    "CREATE TABLE widened(a, b DEFAULT CURRENT_TIMESTAMP, c DEFAULT (1+2), d DEFAULT (-(5)),"
    " f DEFAULT abc, g REAL DEFAULT 3, i DEFAULT X'41', j TEXT DEFAULT (X'41'), k DEFAULT (+5),"
    " l DEFAULT (CAST(1 AS TEXT)), m DEFAULT ('a'||'b'), n INTEGER DEFAULT ('7'),"
    " o DEFAULT (NULL), p DEFAULT (TRUE), q DEFAULT (strftime('%s','now')))"
)

# Values the function queries read: text with a NUL, multi-byte text, blobs,
# numbers, NULL, and (through a cast) text that is not valid UTF-8.
SAMPLES: list[object] = [
    "abc",
    "ABC",
    "a\x00bc",
    "é",
    "É",
    "日本語",
    "\U0001f600x",
    b"abc",
    b"a\x00bc",
    b"\xff\xfeA",
    b"\xc3",
    b"\xe2\x82",
    b"\x80\x80A",
    12,
    -3,
    1.5,
    1e20,
    0.1,
    100.0,
    -0.0,
    1e-7,
    123456789012345678,
    2.5e15,
    None,
    "",
    b"",
    "a%c",
    "a_c",
    " 5 ",
    "3.0e+5",
    "0x10",
]
INVALID_TEXT = [b"\xff\xfeA", b"\xc3", b"\xe2\x82", b"\x80\x80A"]


def add_alias_tables(connection: sqlite3.Connection) -> None:
    """One table per rowid alias declaration, holding rowid 7 with id 3."""
    for number, form in enumerate(ALIAS_FORMS):
        table = "alias_%d" % number
        connection.execute("CREATE TABLE %s(%s)" % (table, form))
        try:
            connection.execute("INSERT INTO %s(rowid, id, v) VALUES (7, 3, 'x')" % table)
        except sqlite3.IntegrityError:
            connection.execute("INSERT INTO %s(id, v) VALUES (3, 'x')" % table)


def add_default_tables(connection: sqlite3.Connection) -> None:
    """Tables whose older rows are shorter than the table: ALTERed and rewritten ones."""
    connection.execute("CREATE TABLE added(a)")
    connection.executemany("INSERT INTO added VALUES (?)", [(1,), (2,)])
    number = 0
    for default in ADDED_DEFAULTS:
        for kind in ADDED_TYPES:
            statement = "ALTER TABLE added ADD COLUMN c%d %s DEFAULT %s" % (number, kind, default)
            connection.execute(statement)
            number += 1
    connection.execute("INSERT INTO added(a) VALUES (3)")
    connection.execute("CREATE TABLE widened(a)")
    connection.execute("INSERT INTO widened VALUES (1)")
    connection.execute("PRAGMA writable_schema=ON")
    connection.execute("UPDATE sqlite_master SET sql=? WHERE name='widened'", (WIDENED_SQL,))
    connection.execute("PRAGMA writable_schema=OFF")


def add_shape_tables(connection: sqlite3.Connection) -> None:
    """REAL and STRICT columns, WITHOUT ROWID key orders, generated columns and quoting."""
    connection.execute("CREATE TABLE reals(r REAL, f FLOAT, d DOUBLE PRECISION, n NUMERIC, u)")
    values = (2, 2.5, "3", None, 2**53 + 1, -0.0, "x", 1e300)
    connection.executemany(
        "INSERT INTO reals VALUES (?, ?, ?, ?, ?)", [(value,) * 5 for value in values]
    )
    connection.execute("CREATE TABLE strict_t(i INTEGER, r REAL, t TEXT, b BLOB, a ANY) STRICT")
    connection.execute("INSERT INTO strict_t VALUES (1, 2, 't', x'00', '000123')")
    connection.execute("INSERT INTO strict_t VALUES (NULL, 2.5, NULL, NULL, 4)")
    connection.execute(
        "CREATE TABLE keyed(a TEXT, b INTEGER, c TEXT, d, PRIMARY KEY(c, b)) WITHOUT ROWID"
    )
    keyed = [("a%d" % row, 7 - row, "c%d" % (row % 2), row * 1.5) for row in range(8)]
    connection.executemany("INSERT INTO keyed VALUES (?, ?, ?, ?)", keyed)
    connection.execute("ALTER TABLE keyed ADD COLUMN e DEFAULT 'later'")
    connection.execute("INSERT INTO keyed VALUES ('z', 9, 'c0', NULL, 'set')")
    connection.execute("CREATE TABLE keyed_desc(k INTEGER, v, PRIMARY KEY(k DESC)) WITHOUT ROWID")
    descending = [(row, "v%d" % row) for row in range(5)]
    connection.executemany("INSERT INTO keyed_desc VALUES (?, ?)", descending)
    connection.execute(
        "CREATE TABLE stored_gen(a INTEGER, b TEXT GENERATED ALWAYS AS (a || 'x') STORED, c)"
    )
    connection.executemany("INSERT INTO stored_gen(a, c) VALUES (?, ?)", [(1, "one"), (2, None)])
    connection.execute("CREATE TABLE computed(a INTEGER, b AS (a * 2), c)")
    connection.execute("INSERT INTO computed(a, c) VALUES (4, 'four')")
    connection.execute(
        'CREATE TABLE "odd ""name"" here" ([a b] TEXT, `c d` INTEGER, \'e f\', "g""h" -- a note\n'
        " , /* a block comment */ i NUMERIC DEFAULT 'n/a')"
    )
    connection.execute("INSERT INTO \"odd \"\"name\"\" here\" VALUES ('x', '7', 1.0, 2, '3.5')")


def add_query_tables(connection: sqlite3.Connection) -> None:
    """Tables the queries read: fts5, collations, mixed values, invalid text, samples."""
    connection.execute(
        "CREATE VIRTUAL TABLE search USING fts5(title, body UNINDEXED,"
        " tokenize='unicode61 remove_diacritics 2')"
    )
    search = [("first", "one"), ("second", None), ("third", 3), ("fourth", b"\x04")]
    connection.executemany("INSERT INTO search(title, body) VALUES (?, ?)", search)
    connection.execute("DELETE FROM search WHERE title = 'second'")
    connection.execute(
        "CREATE TABLE people(name TEXT COLLATE NOCASE, n INTEGER, t TEXT COLLATE RTRIM)"
    )
    people = [("bob", 2, "b  "), ("Alice", 1, "b"), ("alice", 1, "a "), ("BOB", 3, "a")]
    connection.executemany("INSERT INTO people VALUES (?, ?, ?)", [*people, (None, 0, "")])
    connection.execute("CREATE TABLE mixed(x, y TEXT, z INTEGER, w REAL)")
    connection.executemany("INSERT INTO mixed VALUES (?, ?, ?, ?)", MIXED)
    connection.execute("CREATE TABLE broken(a TEXT, b)")
    connection.execute("INSERT INTO broken VALUES ('ok', 1)")
    connection.execute("INSERT INTO broken VALUES (CAST(x'ff41' AS TEXT), 2)")
    connection.execute("INSERT INTO broken VALUES ('after', 3)")
    connection.execute("CREATE TABLE samples(x)")
    for value in SAMPLES:
        connection.execute("INSERT INTO samples VALUES (?)", (value,))
    for value in INVALID_TEXT:
        connection.execute("INSERT INTO samples VALUES (CAST(? AS TEXT))", (value,))


MIXED = [
    (5, "5", 5, 5),
    ("5", "b", "5", 5.5),
    (5.0, "a", 4, None),
    (None, None, None, 1),
    (b"5", "B", 3, 2),
    ("x", "5.0", 5, 5),
    (2, " 5", "abc", -1),
    (5, "c", 5, 5),
]


def build_schema() -> str:
    """A database of the table shapes the schema layer reads and the tables queries use."""
    name = "schema.db"
    connection = connect(HERE / name, 1024)
    add_alias_tables(connection)
    add_default_tables(connection)
    add_shape_tables(connection)
    add_query_tables(connection)
    connection.close()
    TREES[name] = []
    return name


def build_sites() -> list[str]:
    """Databases holding the tables the parsers' queries read, with ties, NULLs and odd keys."""
    name = "sites.db"
    connection = connect(HERE / name, 1024)
    for statement in SITE_SCHEMA:
        connection.execute(statement)
    for table, rows in SITE_ROWS.items():
        marks = ", ".join("?" * len(rows[0]))
        connection.executemany("INSERT INTO %s VALUES (%s)" % (table, marks), rows)
    connection.execute("DELETE FROM session_search WHERE rowid = 2")
    connection.close()
    hermes = "sites_hermes.db"
    connection = connect(HERE / hermes, 1024)
    connection.execute("CREATE TABLE conversations (name TEXT, response_id TEXT NOT NULL)")
    names = [("math", "resp_2"), ("chat", "resp_1"), ("math again", "resp_2"), (None, "resp_3")]
    connection.executemany("INSERT INTO conversations VALUES (?, ?)", names)
    connection.close()
    TREES[name] = []
    TREES[hermes] = []
    return [name, hermes]


SITE_SCHEMA = [
    "CREATE TABLE local_thread_catalog (host_id TEXT NOT NULL, thread_id TEXT NOT NULL,"
    " display_title TEXT, source_created_at REAL, source_kind TEXT,"
    " PRIMARY KEY (host_id, thread_id))",
    "CREATE TABLE cursorDiskKV (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)",
    "CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)",
    "CREATE TABLE archived_git_worktrees(id INTEGER PRIMARY KEY, worktree_path TEXT NOT NULL)"
    " STRICT",
    "CREATE TABLE thread_archived_worktrees(thread_id BLOB NOT NULL, archived_worktree_id INTEGER"
    " NOT NULL, PRIMARY KEY (thread_id, archived_worktree_id)) STRICT",
    "CREATE TABLE conversations (conversation_id TEXT NOT NULL PRIMARY KEY, channel TEXT NOT NULL,"
    " account_id TEXT, kind TEXT, peer_id TEXT, thread_id TEXT)",
    "CREATE TABLE session_conversations (session_id TEXT, conversation_id TEXT, role TEXT,"
    " first_seen_at INTEGER)",
    "CREATE TABLE session_transcript_cold_archives (session_id TEXT, archive_name TEXT,"
    " archived_at INTEGER, storage TEXT, archive_blob BLOB)",
    "CREATE TABLE session_transcript_cold_archive_chunks (session_id TEXT NOT NULL, chunk_index"
    " INTEGER NOT NULL, archive_name TEXT NOT NULL UNIQUE, archive_blob BLOB,"
    " PRIMARY KEY (session_id, chunk_index)) WITHOUT ROWID",
    "CREATE TABLE logs (id INTEGER PRIMARY KEY AUTOINCREMENT, ts INTEGER NOT NULL, ts_nanos INTEGER"
    " NOT NULL, target TEXT NOT NULL, feedback_log_body TEXT, thread_id TEXT)",
    "CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL,"
    " content_json TEXT NOT NULL)",
    "CREATE VIRTUAL TABLE session_search USING fts5(session_id UNINDEXED, document_id UNINDEXED,"
    " ordinal UNINDEXED, role UNINDEXED, started_at UNINDEXED, workspace_root, title, content,"
    " tokenize='unicode61 remove_diacritics 2 tokenchars ''_-@''')",
    "CREATE TABLE session_message (id text PRIMARY KEY, session_id text NOT NULL, type text NOT"
    " NULL, seq integer, time_created integer NOT NULL, data text NOT NULL)",
    "CREATE TABLE files (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, path TEXT NOT NULL,"
    " content TEXT, version INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL)",
]
SITE_ROWS: dict[str, list[tuple]] = {
    "local_thread_catalog": [("h2", "t1", "late", 2.0, "chatgpt"), ("h1", "t2", None, 1, "codex")],
    "cursorDiskKV": [
        ("composerData:b", b"{}"),
        ("composerData:a", "x"),
        ("composerData;", b"edge"),
        ("composerDat", b"short"),
        ("bubbleId:1:2", b'{"t":1}'),
        ("composerData:é", None),
    ],
    "ItemTable": [
        ("cloudAgentRepository.agents.u2", b"[]"),
        ("cloudAgentRepository.agents.u1", "[1]"),
        ("cloudAgentRepository.agentsX", b"no"),
        ("secret://x", b"s"),
        ("CLOUDAGENTREPOSITORY.agents.u3", b"[]"),
    ],
    "archived_git_worktrees": [(3, "/w/three"), (1, "/w/one"), (2, "/w/two")],
    "thread_archived_worktrees": [(b"\x02", 1), (b"\x01", 1), (b"\x05", 3), (b"\x01\x00", 1)],
    "conversations": [
        ("c1", "telegram", "a1", "dm", "p1", None),
        ("c2", "slack", None, None, "p2", "t9"),
    ],
    "session_conversations": [
        ("s2", "c2", "user", 20),
        ("s1", "c1", "owner", 10),
        ("s3", "c9", "user", 10),
        ("s4", "c1", None, 20),
        ("s5", None, "x", 5),
    ],
    "session_transcript_cold_archives": [
        ("s1", "one.jsonl.zst", 30, "sqlite", b"one"),
        ("s2", "two.jsonl.zst", 10, "sqlite", None),
        ("s3", "three", 10, "file", None),
        ("s1", "again", 30, "sqlite", b"again"),
    ],
    "session_transcript_cold_archive_chunks": [
        ("s1", 2, "one.2", b"b"),
        ("s1", 1, "one.1", b"a"),
        ("s2", 0, "two.0", None),
    ],
    "logs": [
        (None, 100, 1, "codex_feedback", "x feedback event uploaded to Sentry y", "th1"),
        (None, 101, 2, "codex_core::session::handlers", "op=ExecApproval { a }", None),
        (None, 102, 3, "codex_core::session::handlers", "op=PatchApproval { b }", "th2"),
        (None, 103, 4, "codex_app_server::outgoing_message", "no mark", "th3"),
        (None, 104, 5, "other", "op=ExecApproval {", "th4"),
        (
            None,
            105,
            6,
            "codex_app_server::outgoing_message",
            "CommandExecutionRequestApproval {",
            None,
        ),
    ],
    "messages": [
        (None, "s1", '{"type":"toolConfirmationResponse"}'),
        (None, "s2", '{"type":"text"}'),
        (None, "s3", "TOOLCONFIRMATIONRESPONSE"),
        (None, "s4", "toolConfirmationRespons"),
    ],
    "session_search": [
        ("s1", "d1", 1, "user", "2026", "/w", "title one", "content one"),
        ("s2", "d2", 2, "assistant", None, "/w", "gone", "deleted later"),
        ("s3", "d3", 3, "user", "2027", None, "title three", None),
    ],
    "session_message": [
        ("m3", "s1", "user", 2, 10, "{}"),
        ("m1", "s1", "user", 1, 10, "{}"),
        ("m2", "s1", "assistant", 1, 10, "[]"),
        ("m4", "s0", "user", None, 5, "{}"),
        ("m5", "s1", "user", 1, 9, "{}"),
        ("m6", "s0", "user", None, 5, "x"),
    ],
    "files": [
        ("f1", "s1", "/a", "héllo", 1, 20),
        ("f2", "s1", "/b", None, 0, 10),
        ("f3", "s2", "/c", "a\x00b", 2, 10),
        ("f4", "s2", "/d", "", 3, 20),
    ],
}


def site_rows(connection: sqlite3.Connection, site: str) -> list[tuple] | None:
    """The rows one parser's query returns from a database, or None when it lacks the tables.

    Each query is the parser's own text with its parameters; the cursor loop
    the parser runs around a query is run here too.
    """
    tables = {
        row[0] for row in connection.execute("SELECT name FROM sqlite_master WHERE type='table'")
    }

    def has(*names: str) -> bool:
        return all(name in tables for name in names)

    def columns(table: str) -> set:
        return {info[1] for info in connection.execute("PRAGMA table_info(%s)" % table)}

    def run(sql: str, parameters: tuple = ()) -> list[tuple]:
        return [tuple(row) for row in connection.execute(sql, parameters).fetchall()]

    if site in SITE_QUERIES:
        needed, sql, parameters = SITE_QUERIES[site]
        return run(sql, parameters) if has(*needed) else None
    if site == "zed_join" and has("archived_git_worktrees", "thread_archived_worktrees"):
        rows = run(ZED_JOIN)
        # Difference 55: rows of one archived worktree are ordered by thread id as a blob.
        return sorted(rows, key=lambda row: (row[1], row[-1] is not None, row[-1] or b""))
    if site == "openclaw_cold" and has("session_transcript_cold_archives"):
        found = []
        chunked = has("session_transcript_cold_archive_chunks")
        for row in run(COLD_ARCHIVES):
            found.append(row)
            if chunked:
                found += run(COLD_CHUNKS, (row[1],))
        return found
    if site == "codex_logs" and has("logs"):
        have = columns("logs")
        body = "feedback_log_body" if "feedback_log_body" in have else "message"
        if body not in have or "target" not in have:
            return None
        names = ", ".join(
            "%s as %s" % (column if column in have else "null", column)
            for column in ("ts", "ts_nanos", "thread_id")
        )
        where = " or ".join("(target = ? and instr(%s, ?) > 0)" % body for _kind in LOG_KINDS)
        sql = "select rowid as _rowid, %s, target, %s as body from logs where %s order by rowid"
        parameters = tuple(value for kind in LOG_KINDS for value in kind)
        return run(sql % (names, body, where), parameters)
    if (
        site == "hermes_names"
        and has("conversations")
        and {"name", "response_id"} <= columns("conversations")
    ):
        return run("select name, response_id from conversations")
    return None


CLOUD_PREFIX = "cloudAgentRepository.agents."
# The substr calls of the functions query: as text they may hold invalid
# UTF-8, which Python cannot return, so the query reads each one's type and hex.
SUBSTR_CALLS = [
    "substr(x, 1, 1)",
    "substr(x, 2, 1000000000)",
    "substr(x, -1, 1)",
    "substr(x, 0, 2)",
    "substr(x, 2, -1)",
    "substr(x, -3, 2)",
]
LOG_KINDS = [
    ("codex_feedback", "feedback event uploaded to Sentry"),
    ("codex_core::session::handlers", "op=ExecApproval {"),
    ("codex_core::session::handlers", "op=PatchApproval {"),
    ("codex_app_server::outgoing_message", "CommandExecutionRequestApproval {"),
]
ZED_JOIN = (
    "select a.rowid as _rowid, a.*, t.thread_id as _thread from"
    " archived_git_worktrees a left join thread_archived_worktrees t"
    " on t.archived_worktree_id = a.id order by a.id"
)
COLD_ARCHIVES = (
    "select rowid as _rowid, session_id, archive_name, archived_at, archive_blob "
    "from session_transcript_cold_archives where storage = 'sqlite' order by archived_at"
)
COLD_CHUNKS = (
    "select archive_name, archive_blob from"
    " session_transcript_cold_archive_chunks where session_id = ?"
    " order by chunk_index"
)
SITE_QUERIES: dict[str, tuple[tuple[str, ...], str, tuple]] = {
    "chatgpt_desktop": (
        ("local_thread_catalog",),
        "select rowid as _rowid, * from local_thread_catalog order by rowid",
        (),
    ),
    "cursor_prefixed": (
        ("cursorDiskKV",),
        "select rowid, key, value from cursorDiskKV where key >= ? and key < ? order by key",
        ("composerData:", "composerData;"),
    ),
    "cursor_one": (
        ("cursorDiskKV",),
        "select rowid, value from cursorDiskKV where key = ?",
        ("bubbleId:1:2",),
    ),
    "cursor_substr": (
        ("ItemTable",),
        "select rowid, key, value from ItemTable where substr(key, 1, ?) = ? order by key",
        (len(CLOUD_PREFIX), CLOUD_PREFIX),
    ),
    "openclaw_join": (
        ("session_conversations", "conversations"),
        "select l.rowid as _rowid, l.session_id, l.role, l.first_seen_at, c.channel,"
        " c.account_id, c.kind, c.peer_id, c.thread_id from session_conversations l"
        " join conversations c using (conversation_id)"
        " order by l.first_seen_at, l.rowid",
        (),
    ),
    "goose_like": (
        ("messages",),
        "select session_id, content_json from messages"
        " where content_json like '%toolConfirmationResponse%'",
        (),
    ),
    "cline_fts": (
        ("session_search",),
        "select session_id, document_id, ordinal, role, started_at, workspace_root, title, content"
        " from session_search order by rowid",
        (),
    ),
    "opencode_messages": (
        ("session_message",),
        "select rowid as _rowid, id, session_id, type, seq, time_created, data "
        "from session_message order by session_id, seq, time_created",
        (),
    ),
    "crush_files": (
        ("files",),
        "select rowid as _rowid, session_id, path, version, length(content) as size,"
        " created_at from files order by created_at, rowid",
        (),
    ),
    "functions": (
        ("samples",),
        "select rowid, length(x), %s, instr(x, 'b'), instr(x, x'62'),"
        " instr(x, 'é'), instr(x, ''), x like 'A%%', x like '%%c', x like '_b%%', x like '%%',"
        " x like '_', x like 'é', x like 'É', x like '_A', x like '__', x like '%%B_',"
        " x like 'a\\%%c', x like '%%5%%' from samples order by rowid"
        % ", ".join("typeof(%s), hex(%s)" % (call, call) for call in SUBSTR_CALLS),
        (),
    ),
    "comparisons": (
        ("mixed",),
        "select rowid, x = 5, x = '5', y = 5, y = '5', z = '5', z = 5.0, w = '5', w = 5,"
        " y < 'b', z > '3', x >= 2 from mixed order by rowid",
        (),
    ),
    "order_mixed": (("mixed",), "select rowid from mixed order by x, y desc", ()),
    "order_people": (("people",), "select rowid, name from people order by name, t desc", ()),
}
SITES = [*SITE_QUERIES, "zed_join", "openclaw_cold", "codex_logs", "hermes_names"]


def dump_tables(connection: sqlite3.Connection) -> list[str]:
    """Every table's column names and rows, in rowid order (key order without rowids)."""
    lines = []
    names = connection.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall()
    for (table,) in names:
        lines.append("table " + table.encode("utf-8").hex())
        quoted = '"%s"' % table.replace('"', '""')
        try:
            try:
                cursor = connection.execute("SELECT rowid, * FROM %s ORDER BY rowid" % quoted)
                with_rowid = True
            except sqlite3.OperationalError:
                cursor = connection.execute("SELECT * FROM %s" % quoted)
                with_rowid = False
            columns = [entry[0] for entry in cursor.description][1 if with_rowid else 0 :]
            rows = cursor.fetchall()
        except sqlite3.Error as error:
            lines.append("error " + str(error))
            continue
        lines.append(" ".join(["columns", *(name.encode("utf-8").hex() for name in columns)]))
        for row in rows:
            values = list(row)
            rowid = str(values.pop(0)) if with_rowid else "-"
            lines.append(" ".join(["row", rowid, *map(describe, values)]))
    return lines


def dump_sites(connection: sqlite3.Connection) -> list[str]:
    """The rows each parser query returns from the database, for the queries it can run."""
    lines = []
    for site in SITES:
        try:
            rows = site_rows(connection, site)
        except sqlite3.Error as error:
            lines += ["site " + site, "error " + str(error)]
            continue
        if rows is None:
            continue
        lines.append("site " + site)
        lines += [" ".join(["row", *map(describe, row)]) for row in rows]
    return lines


def dump_queries(path: Path, scratch: Path) -> list[str]:
    """The `db` section of tables.txt for one database: its tables, then its query results."""
    target = scratch / ("query-" + path.name)
    copy_open(path, target, ("-wal", "-journal"))
    connection = sqlite3.connect(target)
    try:
        return ["db " + path.name, *dump_tables(connection), *dump_sites(connection)]
    finally:
        connection.close()


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
    queried = [build_schema(), *build_sites()]
    names += queried
    with tempfile.TemporaryDirectory() as directory:
        scratch = Path(directory)
        names += build_log_variants(scratch)
        names += build_log_fresh_and_reset(scratch)
        names += build_journal_variants(scratch)
        lines = ["# Written by generate.py from SQLite %s; do not edit." % sqlite3.sqlite_version]
        for name in names:
            lines += dump(name, scratch)
        tables = ["# Written by generate.py from SQLite %s; do not edit." % sqlite3.sqlite_version]
        for name in queried:
            tables += dump_queries(HERE / name, scratch)
    (HERE / "expected.txt").write_text("\n".join(lines) + "\n", encoding="utf-8")
    (HERE / "tables.txt").write_text("\n".join(tables) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
