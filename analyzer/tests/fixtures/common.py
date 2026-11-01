"""Helpers and project values the fixture modules share."""

from __future__ import annotations

import datetime
import json
import sqlite3
from pathlib import Path

# The `/srv/proj` repository's remotes: `origin`, which the
# Codex rollout, the Codex state database, the Zed thread blob and the
# OpenClaw projects table record, with a push URL that embeds a token, and
# `upstream`. `origin` has a second `url`, a mirror every push also goes
# to. HEAD names the branch `main`.
PROJECT_REMOTE = "git@github.com:alice/proj.git"
PROJECT_MIRROR = "https://mirror.example.com/alice/proj.git"
PROJECT_PUSH_URL = "https://alice:FIXTURE-TOKEN@github.com/alice/proj.git"
PROJECT_UPSTREAM = "https://github.com/upstream/proj.git"
PROJECT_GIT_CONFIG = (
    "[core]\n"
    "\trepositoryformatversion = 0\n"
    "\tbare = false\n"
    '[remote "origin"]\n'
    "\turl = %s\n"
    "\turl = %s\n"
    "\tpushurl = %s ; the push goes through HTTPS\n"
    "\tfetch = +refs/heads/*:refs/remotes/origin/*\n"
    '[remote "upstream"]\n'
    '\turl = "%s"\n'
    "\tfetch = +refs/heads/*:refs/remotes/upstream/*\n"
    '[branch "main"]\n'
    "\tremote = origin\n"
    "\tmerge = refs/heads/main\n"
) % (PROJECT_REMOTE, PROJECT_MIRROR, PROJECT_PUSH_URL, PROJECT_UPSTREAM)
PROJECT_GIT_HEAD = "ref: refs/heads/main\n"
PROJECT_GIT_REMOTES = [
    {"name": "origin", "urls": [PROJECT_REMOTE, PROJECT_MIRROR], "pushurls": [PROJECT_PUSH_URL]},
    {"name": "upstream", "urls": [PROJECT_UPSTREAM], "pushurls": []},
]


def _jsonl(path: Path, records) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w", encoding="utf-8") as handle:
        for record in records:
            handle.write(json.dumps(record, ensure_ascii=False) + "\n")


# A JSON object cut off mid-write, as a running agent leaves a transcript's
# last line.
BAD_LINE = '{"type": "user", "truncated": tr'


def write_bad_line(path: Path) -> None:
    """Append `BAD_LINE` to `path`, with no line terminator before or after it.

    After a file that ends in a newline it is a last line cut off mid-write, after one that does not
    it joins the last line. The row `error_rows` yields for it is `bad_line_text()`.
    """
    with open(path, "a", encoding="utf-8") as handle:
        handle.write(BAD_LINE)


def bad_line_text(line: str = BAD_LINE) -> str:
    """The `text` of the `system` row `error_rows` yields for the JSONL line `line`.

    `parser: unparseable line (<reason>): <line>`, with the reason the running Python's json module
    gives, since its wording differs between versions.
    """
    try:
        json.loads(line)
    except ValueError as error:
        return "parser: unparseable line (%s): %s" % (error, line)
    raise AssertionError("line parses: %r" % line)


def _utc_text(epoch: int) -> str:
    """`YYYY-MM-DD HH:MM:SS` in UTC, the summaries database's time text."""
    return datetime.datetime.fromtimestamp(epoch, datetime.UTC).strftime("%Y-%m-%d %H:%M:%S")


def _wal_database(path: Path, script: str, inserts) -> None:
    """Build a WAL-mode SQLite file whose rows live only in the -wal sidecar.

    Build a WAL-mode SQLite file whose schema is checkpointed into the main file and whose rows live
    only in the -wal sidecar, as on a host where the agent is still running. The files are copied
    out while the writer is still open, so closing it cannot checkpoint the copy.
    """
    import shutil
    import tempfile

    temporary = Path(tempfile.mkdtemp(prefix="cac-fixture-"))
    try:
        source = temporary / path.name
        connection = sqlite3.connect(str(source))
        connection.execute("PRAGMA journal_mode=WAL")
        connection.execute("PRAGMA wal_autocheckpoint=0")
        connection.executescript(script)
        connection.commit()
        connection.execute("PRAGMA wal_checkpoint(TRUNCATE)")
        for sql in inserts:
            connection.execute(sql)
        connection.commit()
        path.parent.mkdir(parents=True, exist_ok=True)
        for suffix in ("", "-wal", "-shm"):
            if Path(str(source) + suffix).exists():
                shutil.copyfile(str(source) + suffix, str(path) + suffix)
        connection.close()
    finally:
        shutil.rmtree(temporary, ignore_errors=True)


def _json_file(path: Path, record) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(record, indent=2), encoding="utf-8")


def _json(path: Path, value) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False), encoding="utf-8")


VSCDB_SCHEMA = "CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB);"


def _vscdb(path: Path, items: dict) -> None:
    """Add `ItemTable` rows to a plain (non-WAL) state.vscdb."""
    path.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(path))
    connection.executescript(VSCDB_SCHEMA.replace("CREATE TABLE", "CREATE TABLE IF NOT EXISTS"))
    for k, value in items.items():
        connection.execute(
            "INSERT INTO ItemTable (key, value) VALUES (?, ?)",
            (k, (value if isinstance(value, str) else json.dumps(value)).encode("utf-8")),
        )
    connection.commit()
    connection.close()
