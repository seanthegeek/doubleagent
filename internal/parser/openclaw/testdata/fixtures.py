"""Build the openclaw package's test databases.

Run it with the analyzer's interpreter, which has zstandard, from any directory:

    analyzer/.venv/bin/python -I internal/parser/openclaw/testdata/fixtures.py

Each database below is an openclaw-agent.sqlite written by SQLite itself, so the Go tests read
files SQLite wrote. The cold archive tables have the agent's own schema. Pages are 512 bytes to
keep the files small. No test runs this script; rerun it after changing the rows.
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
CREATE TABLE transcript_events (session_id TEXT NOT NULL, seq INTEGER NOT NULL, event_json TEXT,
  created_at INTEGER NOT NULL, event_zstd BLOB, event_utf8_bytes INTEGER, navigation_json TEXT,
  PRIMARY KEY (session_id, seq));
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


def archive(
    session: str, name: str, archived_at: int, blob: bytes | None, generation: str = "g1"
) -> tuple:
    """Return a sqlite-stored cold archive row."""
    return (session, generation, name, "0" * 64, 1, 0, 0, 1, archived_at, "sqlite", blob)


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


def repeated(count: int) -> tuple[list[tuple], list[tuple]]:
    """Return count archives of one session, which the agent's schema does not allow, and count chunks."""
    archives = [archive("s", f"a{number}", number, None) for number in range(count)]
    chunks = [
        chunk("s", number, f"c{number}", cold_jsonl("s", f"event {number}"))
        for number in range(count)
    ]
    return archives, chunks


def generations() -> tuple[list[tuple], list[tuple]]:
    """Return two archives of one session of two generations, the second with its own bytes, and two chunks."""
    archives = [
        archive("s", "archive-g1", 1, None, "g1"),
        archive("s", "archive-g2", 2, cold_jsonl("s", "g2 own"), "g2"),
    ]
    chunks = [
        chunk("s", 0, "c0", cold_jsonl("s", "first")),
        chunk("s", 1, "c1", cold_jsonl("s", "second")),
    ]
    return archives, chunks


def conversations(count: int) -> str:
    """Return the SQL of count conversations rows of one conversation_id, and as many links and deliveries.

    conversation_id is the key of the agent's conversations table; a table without that key is the only
    way one id can name several rows.
    """
    rows = [
        "PRAGMA page_size = 512;",
        "CREATE TABLE conversations (conversation_id TEXT, channel TEXT, account_id TEXT, kind TEXT,"
        " peer_id TEXT, thread_id TEXT);",
        "CREATE TABLE session_conversations (session_id TEXT, role TEXT, first_seen_at INTEGER,"
        " conversation_id TEXT);",
        "CREATE TABLE conversation_deliveries (conversation_id TEXT, created_at INTEGER,"
        " source_session_key TEXT, operation_kind TEXT, status TEXT, platform_message_id TEXT);",
    ]
    for number in range(count):
        rows.append(
            f"INSERT INTO conversations VALUES ('c', 'telegram', 'acct', 'dm', 'p{number}', NULL);"
        )
        rows.append(f"INSERT INTO session_conversations VALUES ('s', 'primary', {number}, 'c');")
        rows.append(
            f"INSERT INTO conversation_deliveries VALUES ('c', {number}, 'k', 'send', 'sent', 'm{number}');"
        )
    return "\n".join(rows)


def large_event(seq: int) -> tuple:
    """Return a transcript_events row whose event_zstd decompresses to exactly 150000 bytes."""
    text = json.dumps({"type": "label", "label": f"event {seq}", "pad": ""})
    padded = text[:-2] + "x" * (150000 - len(text)) + '"}'
    blob = zstandard.ZstdCompressor(level=3).compress(padded.encode())
    return ("s1", seq, None, 1790845200000 + seq, blob, len(padded), None)


DATABASES = {
    "cold_archives_2.db": many(2),
    "cold_archives_8.db": many(8),
    "cold_chunks.db": grouped(),
    "zstd_events.db": ([], []),
}

# A cold archives table without the key on session_id, so one session holds several archives.
DUPLICATE_SCHEMA = SCHEMA.replace(
    "session_transcript_cold_archives (session_id TEXT NOT NULL PRIMARY KEY,",
    "session_transcript_cold_archives (session_id TEXT NOT NULL,",
)

DUPLICATE_DATABASES = {
    "cold_repeated_10.db": repeated(10),
    "cold_repeated_40.db": repeated(40),
    "cold_generations.db": generations(),
}


# A state database whose exec approvals config does not decode; it holds the socket token.
BAD_EXEC_CONFIG = """
PRAGMA page_size = 512;
CREATE TABLE exec_approvals_config (config_key TEXT NOT NULL PRIMARY KEY, raw_json TEXT NOT NULL,
  has_socket_token INTEGER NOT NULL, updated_at_ms INTEGER NOT NULL);
INSERT INTO exec_approvals_config VALUES ('default', '{"socket": {"token": sk-SECRET}}', 1,
  1790845100000);
"""

EXEC_CONFIG_WITH_SOCKET = """
PRAGMA page_size = 512;
CREATE TABLE exec_approvals_config (config_key TEXT NOT NULL PRIMARY KEY, raw_json TEXT NOT NULL,
  has_socket_token INTEGER NOT NULL, updated_at_ms INTEGER NOT NULL);
INSERT INTO exec_approvals_config VALUES ('default', '{"version": 1, "socket": {"path":
  "/home/u/.openclaw/exec.sock", "token": "FIXTURE-SOCKET-TOKEN"}, "defaults": {"security": "deny"}}', 1,
  1790845100000);
INSERT INTO exec_approvals_config VALUES ('bare', '{"version": 1, "socket": {"token":
  "FIXTURE-SOCKET-TOKEN"}}', 1, 1790845100000);
"""

STATE_URLS = """
PRAGMA page_size = 512;
CREATE TABLE backup_runs (id TEXT PRIMARY KEY, created_at INTEGER NOT NULL, archive_path TEXT,
  status TEXT NOT NULL, manifest_json TEXT);
INSERT INTO backup_runs VALUES ('b1', 1790845100000, NULL, 'ok',
  '{"kind": "git", "target": "https://ci:s3cret@git.example/o/backup.git"}');
CREATE TABLE github_publication_requests (session_id TEXT, identity_source TEXT,
  identity_account_id INTEGER, identity_login TEXT, status TEXT, repository TEXT, branch TEXT,
  pull_request_url TEXT, created_at_ms INTEGER);
INSERT INTO github_publication_requests VALUES ('s1', 'app', 7, 'ana', 'opened', 'o/r', 'fix',
  'https://ci:s3cret@github.example/o/r/pull/7', 1790845200000);
CREATE TABLE projects (repo_root TEXT, origin_url TEXT, created_at_ms INTEGER);
INSERT INTO projects VALUES ('/w/r', 'https://ci:s3cret@git.example/o/r.git', 1790845300000);
"""


# One window whose key, display name, model and provider are each 10000 bytes, more than the 4096 a
# value copied onto many rows is written with, and ten archived generations of its session that name
# no key of their own, so each generation's start row copies the window's.
LONG = "replace(hex(zeroblob(5000)), '0', '{letter}')"
WINDOW_ARCHIVES = (
    """
PRAGMA page_size = 512;
CREATE TABLE session_windows (session_id TEXT NOT NULL PRIMARY KEY, session_key TEXT NOT NULL,
  previous_session_id TEXT, reason TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
  started_at INTEGER, ended_at INTEGER, status TEXT, chat_type TEXT, channel TEXT, account_id TEXT,
  model_provider TEXT, model TEXT, parent_session_key TEXT, spawned_by TEXT, display_name TEXT,
  hook_external_content_source TEXT);
CREATE TABLE session_transcript_archives (session_id TEXT NOT NULL, generation TEXT NOT NULL,
  session_key TEXT NOT NULL, reason TEXT NOT NULL, encoding TEXT NOT NULL, archive_blob BLOB NOT NULL,
  archive_sha256 TEXT NOT NULL, archive_name TEXT NOT NULL UNIQUE, created_at INTEGER NOT NULL,
  published_at INTEGER, PRIMARY KEY (session_id, generation));
"""
    + f"""INSERT INTO session_windows (session_id, session_key, created_at, updated_at, channel,
  display_name, model, model_provider) VALUES ('s', {LONG.format(letter="k")}, 1, 1, 'webchat',
  {LONG.format(letter="d")}, {LONG.format(letter="m")}, {LONG.format(letter="p")});
"""
    + "".join(
        f"INSERT INTO session_transcript_archives VALUES ('s', 'g{number}', '', 'reset', 'jsonl',"
        f" CAST('{{\"type\": \"label\", \"label\": \"l{number}\"}}' AS BLOB), '', 'a{number}',"
        f" {number}, NULL);\n"
        for number in range(10)
    )
)

def write_state(name: str, script: str) -> None:
    """Write a state database from its SQL."""
    path = HERE / name
    path.unlink(missing_ok=True)
    with contextlib.closing(sqlite3.connect(path)) as connection:
        connection.executescript(script)
        connection.commit()


def main() -> None:
    """Rewrite every database of DATABASES and DUPLICATE_DATABASES in this directory."""
    schemas = {name: SCHEMA for name in DATABASES} | {
        name: DUPLICATE_SCHEMA for name in DUPLICATE_DATABASES
    }
    for name, (archives, chunks) in (DATABASES | DUPLICATE_DATABASES).items():
        path = HERE / name
        path.unlink(missing_ok=True)
        with contextlib.closing(sqlite3.connect(path)) as connection:
            connection.executescript(schemas[name])
            connection.executemany(
                "INSERT INTO session_transcript_cold_archives VALUES (?,?,?,?,?,?,?,?,?,?,?)",
                archives,
            )
            connection.executemany(
                "INSERT INTO session_transcript_cold_archive_chunks VALUES (?,?,?,?,?,?,?,?,?,?)",
                chunks,
            )
            if name == "zstd_events.db":
                connection.executemany(
                    "INSERT INTO transcript_events VALUES (?,?,?,?,?,?,?)",
                    [large_event(seq) for seq in range(3)],
                )
            connection.commit()

    write_state("bad_exec_config.db", BAD_EXEC_CONFIG)
    write_state("exec_config_socket.db", EXEC_CONFIG_WITH_SOCKET)
    write_state("state_urls.db", STATE_URLS)
    for count in (10, 40):
        write_state(f"conversations_repeated_{count}.db", conversations(count))
    write_state("window_archives.db", WINDOW_ARCHIVES)


if __name__ == "__main__":
    main()
