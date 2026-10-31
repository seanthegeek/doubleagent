"""Synthetic OpenCode state for the analyzer tests.

The shapes follow `research/opencode.md` (anomalyco/opencode at bf544cd, 2.2.5; cb07c55, 2.0.32;
sst/opencode at 907b3bc, 1.x): a 1.x database with a `session_message` projection, a channel
database with the 1.x share records, a 1.x database upgraded by 2.x, the pre-release 2.0
database, the 1.x JSON tree, and the standing state and terminal UI files.

Paths: `/srv/proj/web`
       `/home/alice/.local/share/opencode/worktree/proj_a/brave-otter`
"""

from __future__ import annotations

import json
import os
import sqlite3
from pathlib import Path

from .common import _json, _json_file, _jsonl

# ---- OpenCode and Kilo Code ---------------------------------------------------------
# Shapes from research/opencode.md and kilo-code.md (drizzle tables in
# packages/core/src/session/sql.ts, JSON in packages/schema/src/v1/session.ts).

OPENCODE_SESSION = "ses_01OC"
OPENCODE_V2_SESSION = "ses_02OCV2"
OPENCODE_LEGACY_SESSION = "ses_00LEGACY"
OPENCODE_T0 = 1791018000000  # 3 October 09:00:00 UTC, ms


OPENCODE_SCHEMA = """
CREATE TABLE project (id text PRIMARY KEY, worktree text NOT NULL, vcs text, name text, icon_url text, icon_color text,
  time_created integer NOT NULL, time_updated integer NOT NULL, time_initialized integer, sandboxes text NOT NULL);
CREATE TABLE session (id text PRIMARY KEY, project_id text NOT NULL, workspace_id text, parent_id text, slug text NOT NULL,
  directory text NOT NULL, path text, title text NOT NULL, version text NOT NULL, share_url text, summary_additions integer,
  summary_deletions integer, summary_files integer, summary_diffs text, metadata text, cost real NOT NULL DEFAULT 0,
  tokens_input integer NOT NULL DEFAULT 0, tokens_output integer NOT NULL DEFAULT 0,
  tokens_reasoning integer NOT NULL DEFAULT 0, tokens_cache_read integer NOT NULL DEFAULT 0,
  tokens_cache_write integer NOT NULL DEFAULT 0, revert text,
  permission text, agent text, model text, time_created integer NOT NULL, time_updated integer NOT NULL,
  time_compacting integer, time_archived integer);
CREATE TABLE message (id text PRIMARY KEY, session_id text NOT NULL, time_created integer NOT NULL,
  time_updated integer NOT NULL, data text NOT NULL);
CREATE TABLE part (id text PRIMARY KEY, message_id text NOT NULL, session_id text NOT NULL, time_created integer NOT NULL,
  time_updated integer NOT NULL, data text NOT NULL);
CREATE INDEX part_message_id_id_idx ON part (message_id, id);
CREATE TABLE session_message (id text PRIMARY KEY, session_id text NOT NULL, type text NOT NULL, seq integer NOT NULL,
  time_created integer NOT NULL, time_updated integer NOT NULL, data text NOT NULL);
CREATE TABLE credential (id text PRIMARY KEY, integration_id text, label text NOT NULL, value text NOT NULL,
  connector_id text, method_id text, active integer, time_created integer NOT NULL, time_updated integer NOT NULL);
"""


def _oc_session(
    connection, session_id, project_id, directory, title, created, parent=None, model=None
):
    connection.execute(
        "INSERT INTO session (id, project_id, parent_id, slug, directory, title, version, agent, model, "
        "time_created, time_updated) VALUES (?,?,?,?,?,?,?,?,?,?,?)",
        (
            session_id,
            project_id,
            parent,
            title.lower().replace(" ", "-"),
            directory,
            title,
            "1.2.0",
            "build",
            json.dumps(model) if model else None,
            created,
            created + 5000,
        ),
    )


def _oc_message(connection, mid, session_id, created, data):
    connection.execute(
        "INSERT INTO message VALUES (?,?,?,?,?)",
        (mid, session_id, created, created, json.dumps(data)),
    )


def _oc_part(connection, pid, mid, session_id, created, data):
    connection.execute(
        "INSERT INTO part VALUES (?,?,?,?,?,?)",
        (pid, mid, session_id, created, created, json.dumps(data)),
    )


def opencode_assistant(created, cwd="/srv/proj", model="claude-sonnet-4", **extra):
    message = {
        "role": "assistant",
        "time": {"created": created, "completed": created + 3000},
        "parentID": "msg_01",
        "modelID": model,
        "providerID": "anthropic",
        "mode": "build",
        "agent": "build",
        "path": {"cwd": cwd, "root": cwd},
        "cost": 0.01,
        "tokens": {"input": 10, "output": 5, "reasoning": 0, "cache": {"read": 0, "write": 0}},
    }
    message.update(extra)
    return message


def build_opencode_database(database: Path, start: int = OPENCODE_T0) -> None:
    database.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(database))
    connection.executescript(OPENCODE_SCHEMA)
    connection.execute(
        "INSERT INTO project (id, worktree, vcs, time_created, time_updated, sandboxes) VALUES (?,?,?,?,?,?)",
        ("proj_a1", "/srv/proj", "git", start, start, "[]"),
    )
    _oc_session(connection, OPENCODE_SESSION, "proj_a1", "/srv/proj", "Fix bug", start)
    _oc_message(
        connection,
        "msg_01",
        OPENCODE_SESSION,
        start + 1000,
        {
            "role": "user",
            "time": {"created": start + 1000},
            "agent": "build",
            "model": {"providerID": "anthropic", "modelID": "claude-sonnet-4"},
        },
    )
    _oc_part(
        connection,
        "prt_01",
        "msg_01",
        OPENCODE_SESSION,
        start + 1000,
        {"type": "text", "text": "list files"},
    )
    _oc_part(
        connection,
        "prt_01b",
        "msg_01",
        OPENCODE_SESSION,
        start + 1000,
        {"type": "text", "text": "<system-reminder>plan mode</system-reminder>", "synthetic": True},
    )
    _oc_message(
        connection, "msg_02", OPENCODE_SESSION, start + 2000, opencode_assistant(start + 2000)
    )
    _oc_part(
        connection,
        "prt_02",
        "msg_02",
        OPENCODE_SESSION,
        start + 2000,
        {"type": "step-start", "snapshot": "abc123"},
    )
    _oc_part(
        connection,
        "prt_03",
        "msg_02",
        OPENCODE_SESSION,
        start + 2100,
        {
            "type": "reasoning",
            "text": "user wants a listing",
            "time": {"start": start + 2100, "end": start + 2200},
        },
    )
    _oc_part(
        connection,
        "prt_04",
        "msg_02",
        OPENCODE_SESSION,
        start + 2500,
        {
            "type": "tool",
            "callID": "call_x1",
            "tool": "bash",
            "state": {
                "status": "completed",
                "input": {"command": "ls"},
                "output": "a.txt",
                "title": "ls",
                "metadata": {},
                "time": {"start": start + 2500, "end": start + 3000},
            },
        },
    )
    _oc_part(
        connection,
        "prt_05",
        "msg_02",
        OPENCODE_SESSION,
        start + 3500,
        {
            "type": "text",
            "text": "There is one file, a.txt.",
            "time": {"start": start + 3500, "end": start + 3600},
        },
    )
    _oc_part(
        connection,
        "prt_06",
        "msg_02",
        OPENCODE_SESSION,
        start + 4000,
        {
            "type": "step-finish",
            "reason": "stop",
            "cost": 0.01,
            "tokens": {"input": 10, "output": 5, "reasoning": 0, "cache": {"read": 0, "write": 0}},
        },
    )
    # A session present only in the V2 session_message projection.
    _oc_session(connection, OPENCODE_V2_SESSION, "proj_a1", "/srv/proj", "Run tests", start + 10000)
    connection.execute(
        "INSERT INTO session_message VALUES (?,?,?,?,?,?,?)",
        (
            "msg_v2u",
            OPENCODE_V2_SESSION,
            "user",
            1,
            start + 11000,
            start + 11000,
            json.dumps(
                {"id": "msg_v2u", "time": {"created": start + 11000}, "text": "run the tests"}
            ),
        ),
    )
    connection.execute(
        "INSERT INTO session_message VALUES (?,?,?,?,?,?,?)",
        (
            "msg_v2a",
            OPENCODE_V2_SESSION,
            "assistant",
            2,
            start + 12000,
            start + 14000,
            json.dumps(
                {
                    "id": "msg_v2a",
                    "agent": "build",
                    "model": {"id": "gpt-5", "providerID": "openai"},
                    "content": [
                        {"type": "text", "id": "c1", "text": "Running them."},
                        {
                            "type": "tool",
                            "id": "call_v2",
                            "name": "bash",
                            "state": {
                                "status": "completed",
                                "input": {"command": "pytest -q"},
                                "content": [{"type": "text", "text": "3 passed"}],
                                "structured": {},
                            },
                            "time": {"created": start + 12500, "completed": start + 13000},
                        },
                    ],
                    "time": {"created": start + 12000, "completed": start + 14000},
                }
            ),
        ),
    )
    # Switch records (packages/schema/src/session-message.ts AgentSwitched and
    # ModelSwitched at 907b3bc): `id` and `type` are columns, `data` keeps the rest.
    connection.execute(
        "INSERT INTO session_message VALUES (?,?,?,?,?,?,?)",
        (
            "msg_v2s1",
            OPENCODE_V2_SESSION,
            "agent-switched",
            3,
            start + 15000,
            start + 15000,
            json.dumps({"time": {"created": start + 15000}, "agent": "plan"}),
        ),
    )
    connection.execute(
        "INSERT INTO session_message VALUES (?,?,?,?,?,?,?)",
        (
            "msg_v2s2",
            OPENCODE_V2_SESSION,
            "model-switched",
            4,
            start + 16000,
            start + 16000,
            json.dumps(
                {
                    "time": {"created": start + 16000},
                    "model": {"id": "gpt-5-mini", "providerID": "openai"},
                }
            ),
        ),
    )
    connection.execute(
        "INSERT INTO credential (id, integration_id, label, value, active, time_created, time_updated) "
        "VALUES (?,?,?,?,?,?,?)",
        (
            "cred_01",
            "anthropic",
            "API key",
            '{"type":"key","key":"sk-ant-REDACT"}',
            1,
            start,
            start,
        ),
    )
    connection.commit()
    connection.close()


def build_opencode_storage(storage: Path, start: int = OPENCODE_T0 - 86400000) -> None:
    """The legacy JSON tree: ids inline, one file per record."""
    session_id = OPENCODE_LEGACY_SESSION
    _json_file(
        storage / "project/proj_a1.json",
        {"id": "proj_a1", "vcs": "git", "worktree": "/srv/proj", "time": {"created": start}},
    )
    _json_file(
        storage / "session/proj_a1" / (session_id + ".json"),
        {
            "id": session_id,
            "projectID": "proj_a1",
            "directory": "/srv/proj",
            "title": "Old session",
            "version": "0.9.0",
            "time": {"created": start, "updated": start + 5000},
        },
    )
    _json_file(
        storage / "message" / session_id / "msg_a.json",
        {
            "id": "msg_a",
            "sessionID": session_id,
            "role": "user",
            "time": {"created": start + 1000},
            "agent": "build",
            "model": {"providerID": "anthropic", "modelID": "claude-sonnet-4"},
        },
    )
    _json_file(
        storage / "part/msg_a/prt_a1.json",
        {
            "id": "prt_a1",
            "sessionID": session_id,
            "messageID": "msg_a",
            "type": "text",
            "text": "cat the secrets file",
        },
    )
    _json_file(
        storage / "message" / session_id / "msg_b.json",
        dict(
            opencode_assistant(start + 2000),
            id="msg_b",
            sessionID=session_id,
            error={"name": "APIError", "data": {"message": "overloaded"}},
        ),
    )
    _json_file(
        storage / "part/msg_b/prt_b1.json",
        {
            "id": "prt_b1",
            "sessionID": session_id,
            "messageID": "msg_b",
            "type": "tool",
            "callID": "call_r1",
            "tool": "read",
            "state": {
                "status": "error",
                "input": {"filePath": "/srv/proj/.env"},
                "error": "permission denied",
                "time": {"start": start + 2500, "end": start + 2600},
            },
        },
    )
    _json_file(
        storage / "part/msg_b/prt_b2.json",
        {
            "id": "prt_b2",
            "sessionID": session_id,
            "messageID": "msg_b",
            "type": "text",
            "text": "I could not read it.",
            "time": {"start": start + 2700},
        },
    )
    (storage / "migration").write_text("2", encoding="utf-8")


def build_opencode(home: Path) -> None:
    data = home / ".local/share/opencode"
    build_opencode_database(data / "opencode.db")
    build_opencode_storage(data / "storage")
    (data / "auth.json").write_text(
        '{"github-copilot":{"type":"oauth","refresh":"REDACT","access":"REDACT","expires":0}}',
        encoding="utf-8",
    )
    _json_file(
        home / ".config/opencode/opencode.json", {"$schema": "https://opencode.ai/config.json"}
    )
    build_opencode_share_database(data / OPENCODE_SHARE_DATABASE_NAME)
    build_opencode_v2_database(data / OPENCODE_V2_DATABASE_NAME)
    build_opencode_next_database(data / OPENCODE_NEXT_DATABASE_NAME)


# Session sync: a channel database with the share records of
# research/opencode.md, Other stores: the `session_share` table and the
# `session.updated.1` events. One session is shared now, one was shared
# and then unshared (its share row is gone, its events stay), one never.
OPENCODE_SHARE_DATABASE_NAME = "opencode-beta.db"
OPENCODE_SHARED = "ses_05SHARE"
OPENCODE_UNSHARED = "ses_06UNSHARE"
OPENCODE_PRIVATE = "ses_07PRIVATE"
OPENCODE_SHARE_SECRET = "fixture-share-secret-not-real"
OPENCODE_SHARE_T0 = OPENCODE_T0 + 3600000  # 3 October 10:00:00 UTC
OPENCODE_SHARE_SCHEMA = """
CREATE TABLE session_share (session_id text PRIMARY KEY, id text NOT NULL, secret text NOT NULL,
  url text NOT NULL, time_created integer NOT NULL, time_updated integer NOT NULL);
CREATE TABLE event_sequence (aggregate_id text PRIMARY KEY, seq integer NOT NULL, owner_id text);
CREATE TABLE event (id text PRIMARY KEY, aggregate_id text NOT NULL, seq integer NOT NULL,
  type text NOT NULL, data text NOT NULL);
"""


def opencode_session_event(session_id: str, seq: int, updated: int, share_url: str = "") -> tuple:
    """An `event` row of type `session.updated.1`.

    `data` is `{sessionID, info}`, the session's info with `share.url` while it is shared.
    """
    info: dict = {
        "id": session_id,
        "slug": "s",
        "projectID": "proj_a1",
        "directory": "/srv/proj",
        "title": "Share test",
        "version": "1.2.0",
        "time": {"created": OPENCODE_SHARE_T0, "updated": updated},
    }
    if share_url:
        info["share"] = {"url": share_url}
    data = json.dumps({"sessionID": session_id, "info": info})
    return ("evt_%s_%d" % (session_id, seq), session_id, seq, "session.updated.1", data)


def build_opencode_share_database(database: Path, start: int = OPENCODE_SHARE_T0) -> None:
    database.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(database))
    connection.executescript(OPENCODE_SCHEMA + OPENCODE_SHARE_SCHEMA)
    connection.execute(
        "INSERT INTO project (id, worktree, vcs, time_created, time_updated, sandboxes) VALUES (?,?,?,?,?,?)",
        ("proj_a1", "/srv/proj", "git", start, start, "[]"),
    )
    _oc_session(connection, OPENCODE_SHARED, "proj_a1", "/srv/proj", "Shared", start)
    _oc_session(connection, OPENCODE_UNSHARED, "proj_a1", "/srv/proj", "Unshared", start + 10000)
    _oc_session(connection, OPENCODE_PRIVATE, "proj_a1", "/srv/proj", "Private", start + 20000)
    connection.execute(
        "INSERT INTO session_share VALUES (?,?,?,?,?,?)",
        (
            OPENCODE_SHARED,
            "SHARE05",
            OPENCODE_SHARE_SECRET,
            "https://opncd.ai/share/SHARE05",
            start + 2000,
            start + 2000,
        ),
    )
    events = [
        # The shared session's own event: its share row already says so.
        opencode_session_event(OPENCODE_SHARED, 1, start + 2000, "https://opncd.ai/share/SHARE05"),
        opencode_session_event(OPENCODE_UNSHARED, 1, start + 11000),
        opencode_session_event(
            OPENCODE_UNSHARED, 2, start + 12000, "https://opncd.ai/share/UNSHAR06"
        ),
        opencode_session_event(
            OPENCODE_UNSHARED, 3, start + 13000, "https://opncd.ai/share/UNSHAR06"
        ),
        opencode_session_event(OPENCODE_UNSHARED, 4, start + 14000),
        opencode_session_event(OPENCODE_UNSHARED, 5, start + 15000),
        opencode_session_event(OPENCODE_PRIVATE, 1, start + 21000),
    ]
    connection.executemany("INSERT INTO event VALUES (?,?,?,?,?)", events)
    connection.executemany(
        "INSERT INTO event_sequence VALUES (?,?,NULL)",
        [(OPENCODE_SHARED, 1), (OPENCODE_UNSHARED, 5), (OPENCODE_PRIVATE, 1)],
    )
    connection.commit()
    connection.close()


# OpenCode 2.0, source-derived, not observed on any real install:
# anomalyco/opencode at cb07c555d4d1ca34430b7857b52f104c3d95e09e (CLI 2.0.32).
# `session_v2` from packages/core/src/session/sql.ts:22-68; `session_message`
# `data` without `type` and `id` (sql.ts:20, :79-91), shapes from
# packages/schema/src/session-message.ts:44-309 and shell.ts:25, :77-84. The
# upgrade copies each 1.x session into `session_v2` and its messages into
# `session_message` and keeps the 1.x rows (v1-migration.bun.ts:534-700); a
# session deleted in 2.0 is left only in the 1.x tables. `opencode-next.db`
# is a pre-release 2.0 database (`project`, `session`, `session_message`,
# v1-migration.bun.ts:839-846) that 2.0 imports and leaves in place.
OPENCODE_V2_DATABASE_NAME = "opencode-local.db"
OPENCODE_NEXT_DATABASE_NAME = "opencode-next.db"
OPENCODE_MIGRATED = "ses_10MIGRATED"
OPENCODE_DELETED = "ses_11DELETED"
OPENCODE_V2_ONLY = "ses_12NEW"
OPENCODE_NEXT_ONLY = "ses_13NEXTONLY"
OPENCODE_V2_T0 = OPENCODE_T0 + 7200000  # 3 October 11:00:00 UTC
OPENCODE_SESSION_V2_COLUMNS = """(id text PRIMARY KEY, project_id text NOT NULL, workspace_id text,
  parent_id text, fork_session_id text, fork_boundary text, slug text NOT NULL, directory text NOT NULL,
  path text, title text, version text NOT NULL, share_url text, summary_additions integer,
  summary_deletions integer, summary_files integer, summary_diffs text, metadata text,
  cost real NOT NULL DEFAULT 0, tokens_input integer NOT NULL DEFAULT 0,
  tokens_output integer NOT NULL DEFAULT 0, tokens_reasoning integer NOT NULL DEFAULT 0,
  tokens_cache_read integer NOT NULL DEFAULT 0, tokens_cache_write integer NOT NULL DEFAULT 0,
  revert text, permission text, agent text, model text, time_created integer NOT NULL,
  time_updated integer NOT NULL, time_idle integer, time_viewed integer, idle_outcome text,
  time_compacting integer, time_archived integer, time_suspended integer,
  resume_attempts integer NOT NULL DEFAULT 0)"""


def _oc_v2_session(connection, table, session_id, title, created):
    connection.execute(
        "INSERT INTO %s (id, project_id, slug, directory, title, version, agent, model, "
        "time_created, time_updated) VALUES (?,?,?,?,?,?,?,?,?,?)" % table,
        (
            session_id,
            "proj_a1",
            title.lower().replace(" ", "-"),
            "/srv/proj",
            title,
            "2.0.32",
            "build",
            json.dumps({"id": "claude-sonnet-4-5", "providerID": "anthropic"}),
            created,
            created + 60000,
        ),
    )


def _oc_v2_message(connection, mid, session_id, mtype, seq, created, data):
    """A `session_message` row: `id` and `type` are columns, `data` the rest."""
    connection.execute(
        "INSERT INTO session_message VALUES (?,?,?,?,?,?,?)",
        (
            mid,
            session_id,
            mtype,
            seq,
            created,
            created,
            json.dumps(dict(data, time=dict(data.get("time", {}), created=created))),
        ),
    )


def opencode_v2_assistant(created, content, model="claude-sonnet-4-5"):
    return {
        "agent": "build",
        "model": {"id": model, "providerID": "anthropic", "variant": "default"},
        "content": content,
        "finish": "stop",
        "time": {"completed": created + 3000},
    }


def _oc_v2_new_session(connection, start):
    """The 2.0-only session: every `session_message` type the parser reads."""
    session_id = OPENCODE_V2_ONLY
    _oc_v2_message(
        connection, "msg_n1", session_id, "user", 1, start + 1000, {"text": "start the server"}
    )
    tool_ok = {
        "type": "tool",
        "id": "call_n1",
        "name": "bash",
        "state": {
            "status": "completed",
            "input": {"command": "npm start"},
            "content": [{"type": "text", "text": "listening on 3000"}],
        },
        "time": {"created": start + 2500, "completed": start + 2800},
    }
    tool_declined = {
        "type": "tool",
        "id": "call_n2",
        "name": "edit",
        "state": {
            "status": "error",
            "input": {"filePath": "/srv/proj/.env"},
            "error": {"type": "aborted", "message": "The user declined this tool call"},
        },
        "time": {"created": start + 2900, "completed": start + 2950},
    }
    content = [{"type": "text", "text": "Starting it."}, tool_ok, tool_declined]
    _oc_v2_message(
        connection,
        "msg_n2",
        session_id,
        "assistant",
        2,
        start + 2000,
        opencode_v2_assistant(start + 2000, content),
    )
    shell = {
        "shellID": "sh_01N",
        "command": "git status",
        "status": "exited",
        "exit": 128,
        "output": {
            "output": "fatal: not a git repository",
            "cursor": 27,
            "size": 27,
            "truncated": False,
        },
        "time": {"completed": start + 4500},
    }
    _oc_v2_message(connection, "msg_n3", session_id, "shell", 3, start + 4000, shell)
    location = {
        "location": {"directory": "/srv/proj/web"},
        "previous": {"location": {"directory": "/srv/proj"}},
    }
    _oc_v2_message(connection, "msg_n4", session_id, "location-switched", 4, start + 5000, location)
    skill = {"skill": "skl_deploy", "name": "deploy", "text": "Run the deploy script."}
    _oc_v2_message(connection, "msg_n5", session_id, "skill", 5, start + 6000, skill)
    compaction = {
        "status": "completed",
        "reason": "auto",
        "summary": "Server started.",
        "recent": "",
    }
    _oc_v2_message(connection, "msg_n6", session_id, "compaction", 6, start + 7000, compaction)
    switch = {
        "model": {"id": "gpt-5", "providerID": "openai"},
        "previous": {"id": "claude-sonnet-4-5", "providerID": "anthropic"},
    }
    _oc_v2_message(connection, "msg_n7", session_id, "model-switched", 7, start + 8000, switch)
    _oc_v2_message(
        connection, "msg_n8", session_id, "idle", 8, start + 9000, {"outcome": "succeeded"}
    )


def build_opencode_v2_database(database: Path, start: int = OPENCODE_V2_T0) -> None:
    """A 1.x database upgraded by 2.0: the 1.x tables beside `session_v2`."""
    database.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(database))
    connection.executescript(
        OPENCODE_SCHEMA + "CREATE TABLE session_v2 %s;" % OPENCODE_SESSION_V2_COLUMNS
    )
    connection.execute(
        "INSERT INTO project (id, worktree, vcs, time_created, time_updated, sandboxes) VALUES (?,?,?,?,?,?)",
        ("proj_a1", "/srv/proj", "git", start, start, "[]"),
    )
    # Migrated: the same turns in the 1.x tables and in session_message.
    session_id, created = OPENCODE_MIGRATED, start
    _oc_session(connection, session_id, "proj_a1", "/srv/proj", "Migrated", created)
    _oc_v2_session(connection, "session_v2", session_id, "Migrated", created)
    user = {"role": "user", "time": {"created": created + 1000}, "agent": "build"}
    _oc_message(connection, "msg_m1", session_id, created + 1000, user)
    _oc_part(
        connection,
        "prt_m1",
        "msg_m1",
        session_id,
        created + 1000,
        {"type": "text", "text": "migrate me"},
    )
    _oc_message(
        connection,
        "msg_m2",
        session_id,
        created + 2000,
        opencode_assistant(created + 2000, parentID="msg_m1"),
    )
    _oc_part(
        connection,
        "prt_m2",
        "msg_m2",
        session_id,
        created + 2100,
        {"type": "text", "text": "Migrated reply."},
    )
    _oc_v2_message(
        connection, "msg_m1", session_id, "user", 1, created + 1000, {"text": "migrate me"}
    )
    content = [{"type": "text", "text": "Migrated reply."}]
    _oc_v2_message(
        connection,
        "msg_m2",
        session_id,
        "assistant",
        2,
        created + 2000,
        opencode_v2_assistant(created + 2000, content, model="claude-sonnet-4"),
    )
    # Deleted in 2.0: the session_v2 row and its messages are gone.
    session_id, created = OPENCODE_DELETED, start + 600000
    _oc_session(connection, session_id, "proj_a1", "/srv/proj", "Deleted", created)
    user = {"role": "user", "time": {"created": created + 1000}, "agent": "build"}
    _oc_message(connection, "msg_d1", session_id, created + 1000, user)
    _oc_part(
        connection,
        "prt_d1",
        "msg_d1",
        session_id,
        created + 1000,
        {"type": "text", "text": "what was deleted"},
    )
    _oc_message(
        connection,
        "msg_d2",
        session_id,
        created + 2000,
        opencode_assistant(created + 2000, parentID="msg_d1"),
    )
    _oc_part(
        connection,
        "prt_d2",
        "msg_d2",
        session_id,
        created + 2100,
        {"type": "text", "text": "A deleted reply."},
    )
    # Written by 2.0 only.
    _oc_v2_session(connection, "session_v2", OPENCODE_V2_ONLY, "Start server", start + 1200000)
    _oc_v2_new_session(connection, start + 1200000)
    connection.commit()
    connection.close()


def build_opencode_next_database(database: Path, start: int = OPENCODE_V2_T0) -> None:
    """The pre-release 2.0 database.

    The 2.0-only session's first turns, imported into the main database, and one session found
    nowhere else.
    """
    database.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(database))
    connection.executescript(
        "CREATE TABLE project (id text PRIMARY KEY, worktree text NOT NULL, time_created integer,"
        " time_updated integer, sandboxes text);"
        "CREATE TABLE session %s;"
        "CREATE TABLE session_message (id text PRIMARY KEY, session_id text NOT NULL,"
        " type text NOT NULL, seq integer NOT NULL, time_created integer NOT NULL,"
        " time_updated integer NOT NULL, data text NOT NULL);" % OPENCODE_SESSION_V2_COLUMNS
    )
    connection.execute(
        "INSERT INTO project VALUES (?,?,?,?,?)", ("proj_a1", "/srv/proj", start, start, "[]")
    )
    created = start + 1200000
    _oc_v2_session(connection, "session", OPENCODE_V2_ONLY, "Start server", created)
    _oc_v2_message(
        connection,
        "msg_n1",
        OPENCODE_V2_ONLY,
        "user",
        1,
        created + 1000,
        {"text": "start the server"},
    )
    created = start + 1800000
    _oc_v2_session(connection, "session", OPENCODE_NEXT_ONLY, "Next only", created)
    _oc_v2_message(
        connection,
        "msg_x1",
        OPENCODE_NEXT_ONLY,
        "user",
        1,
        created + 1000,
        {"text": "only in next"},
    )
    connection.commit()
    connection.close()


# Standing state, past use, scratch directories and deletion,
# source-derived, not observed on any real install: anomalyco/
# opencode at cb07c555d4d1ca34430b7857b52f104c3d95e09e and Kilo-Org/kilocode
# at 87cc35ce6904465a0eed507a8c75c79185863a22. Built by the tests that read
# them, not by `build_home`, so the other tests' session and dedup counts
# stay as they are.
OPENCODE_STATE_DATABASE_NAME = "opencode-state.db"
OPENCODE_ARCHIVED = "ses_20ARCHIVED"
OPENCODE_PINNED_GONE = "ses_21PINNEDGONE"
OPENCODE_STATE_T0 = OPENCODE_T0 + 10800000  # 3 October 12:00:00 UTC
OPENCODE_STATE_MTIME = 1791043200  # 3 October 16:00:00 UTC
OPENCODE_WORKTREE = "/home/alice/.local/share/opencode/worktree/proj_a/brave-otter"


def build_opencode_state_database(database: Path, start: int = OPENCODE_STATE_T0) -> None:
    """A 2.0 database with standing state.

    The `permission` table (packages/core/src/permission/sql.ts:7-20), the `worktree` table
    (worktree/sql.ts:6-20) and the deprecated `project_directory` (project/sql.ts:31-47), an
    archived session with a rule list (`session_v2.time_archived`, `permission`), and a tool call
    refused by a deny rule (`error.type` `permission.rejected`, to-session-error.ts:61).
    """
    database.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(database))
    connection.executescript(
        "CREATE TABLE project (id text PRIMARY KEY, worktree text NOT NULL, vcs text, name text,"
        " time_created integer NOT NULL, time_updated integer NOT NULL, sandboxes text NOT NULL);"
        "CREATE TABLE session_v2 %s;"
        "CREATE TABLE session_message (id text PRIMARY KEY, session_id text NOT NULL,"
        " type text NOT NULL, seq integer NOT NULL, time_created integer NOT NULL,"
        " time_updated integer NOT NULL, data text NOT NULL);"
        "CREATE TABLE permission (id text PRIMARY KEY, project_id text NOT NULL,"
        " action text NOT NULL, resource text NOT NULL, time_created integer NOT NULL,"
        " time_updated integer NOT NULL);"
        "CREATE TABLE worktree (project_id text NOT NULL, directory text NOT NULL,"
        " strategy text, time_created integer NOT NULL, PRIMARY KEY (project_id, directory));"
        "CREATE TABLE project_directory (project_id text NOT NULL, directory text NOT NULL,"
        " type text, strategy text, time_created integer NOT NULL,"
        " PRIMARY KEY (project_id, directory));" % OPENCODE_SESSION_V2_COLUMNS
    )
    connection.execute(
        "INSERT INTO project (id, worktree, vcs, time_created, time_updated, sandboxes)"
        " VALUES (?,?,?,?,?,?)",
        ("proj_a", "/srv/proj", "git", start, start, "[]"),
    )
    _oc_v2_session(connection, "session_v2", OPENCODE_ARCHIVED, "Archived", start)
    rules = [
        {"action": "bash", "resource": "git *", "effect": "allow"},
        {"action": "edit", "resource": "*.env", "effect": "deny"},
    ]
    connection.execute(
        "UPDATE session_v2 SET project_id = 'proj_a', time_archived = ?, permission = ?"
        " WHERE id = ?",
        (start + 90000, json.dumps(rules), OPENCODE_ARCHIVED),
    )
    blocked = {
        "type": "tool",
        "id": "call_s1",
        "name": "edit",
        "state": {
            "status": "error",
            "input": {"filePath": "/srv/proj/.env"},
            "error": {"type": "permission.rejected", "message": "Permission denied: edit"},
        },
        "time": {"created": start + 2500, "completed": start + 2600},
    }
    _oc_v2_message(
        connection,
        "msg_s1",
        OPENCODE_ARCHIVED,
        "assistant",
        1,
        start + 2000,
        opencode_v2_assistant(start + 2000, [blocked]),
    )
    connection.execute(
        "INSERT INTO permission VALUES (?,?,?,?,?,?)",
        ("psv_01", "proj_a", "bash", "npm test", start + 3000, start + 3000),
    )
    connection.execute(
        "INSERT INTO worktree VALUES (?,?,?,?)", ("proj_a", OPENCODE_WORKTREE, "git", start + 4000)
    )
    # The same worktree in the deprecated table is not repeated; a `main` row is not one.
    connection.executemany(
        "INSERT INTO project_directory VALUES (?,?,?,?,?)",
        [
            ("proj_a", OPENCODE_WORKTREE, "git_worktree", "git", start + 4000),
            ("proj_a", "/srv/proj", "main", None, start),
        ],
    )
    connection.commit()
    connection.close()


def build_opencode_state(home: Path) -> None:
    """Write OpenCode's terminal UI state files at the 2.0 shapes.

    OpenCode's terminal UI state at the 2.0 shapes (packages/tui/src/prompt/history.tsx:20-23,
    stash.tsx:9-12, context/local.tsx:517-530), a 1.x stash line (`input`), the database above, and
    `kv.json` and `service.json` beside them, which the parser must not want.
    """
    state = home / ".local/state/opencode"
    _jsonl(
        state / "prompt-history.jsonl",
        [
            {"text": "list files", "files": [], "agents": [], "skills": [], "pasted": []},
            {
                "text": "git status",
                "files": [],
                "agents": [],
                "skills": [],
                "pasted": [],
                "mode": "shell",
            },
            {
                "text": "only the history kept this",
                "files": [],
                "agents": [],
                "skills": [],
                "pasted": [],
            },
        ],
    )
    _jsonl(
        state / "prompt-stash.jsonl",
        [
            {
                "prompt": {
                    "text": "draft: rotate the keys",
                    "files": [],
                    "agents": [],
                    "skills": [],
                    "pasted": [],
                },
                "timestamp": OPENCODE_STATE_T0 + 5000,
            },
            {"input": "older stash", "parts": [], "timestamp": OPENCODE_STATE_T0 + 6000},
        ],
    )
    _json(state / "session.json", {"pinned": [OPENCODE_SESSION, OPENCODE_PINNED_GONE]})
    os.utime(state / "session.json", (OPENCODE_STATE_MTIME, OPENCODE_STATE_MTIME))
    _json(state / "kv.json", {"share_consent": True})
    _json(
        state / "service.json", {"id": "svc", "url": "http://127.0.0.1:4096", "password": "REDACT"}
    )
    build_opencode_state_database(home / ".local/share/opencode" / OPENCODE_STATE_DATABASE_NAME)
