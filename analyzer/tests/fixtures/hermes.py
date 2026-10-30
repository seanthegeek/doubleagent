"""Synthetic Hermes state for the analyzer tests."""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path

from .common import _json, _jsonl, _wal_database

HERMES_SESSION = "20261001_120000_a1b2c3d4"
HERMES_CHILD = "20261001_121500_e5f60708"
HERMES_DIVERTED = "20261001_130000_0badf00d"

# hermes_state_common.py:372-480 at hermes-agent 8b66a51, the columns the
# Parser plan of research/hermes.md lists (schema version 31).
HERMES_SCHEMA = """
CREATE TABLE schema_version (version INTEGER NOT NULL);
CREATE TABLE sessions (id TEXT PRIMARY KEY, source TEXT NOT NULL, model TEXT,
  parent_session_id TEXT, started_at REAL NOT NULL, ended_at REAL, end_reason TEXT,
  cwd TEXT, git_branch TEXT, git_repo_root TEXT, title TEXT);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL,
  role TEXT NOT NULL, content TEXT, tool_call_id TEXT, tool_calls TEXT, tool_name TEXT,
  timestamp REAL NOT NULL, finish_reason TEXT, reasoning TEXT, reasoning_content TEXT,
  active INTEGER NOT NULL DEFAULT 1, compacted INTEGER NOT NULL DEFAULT 0);
CREATE TABLE session_model_usage (session_id TEXT NOT NULL, model TEXT NOT NULL,
  billing_provider TEXT NOT NULL DEFAULT '', billing_base_url TEXT NOT NULL DEFAULT '',
  billing_mode TEXT NOT NULL DEFAULT '', task TEXT NOT NULL DEFAULT '',
  api_call_count INTEGER NOT NULL DEFAULT 0, first_seen REAL, last_seen REAL,
  PRIMARY KEY (session_id, model, billing_provider, billing_base_url, billing_mode, task));
"""


def hermes_records(cwd="/srv/proj"):
    """The INSERT statements of research/hermes.md, Parser plan, and three more.

    The INSERT statements of research/hermes.md, Parser plan, with the shared project path, plus a
    rewound turn kept as active=0, a multimodal user message behind the NUL json prefix, and a
    subagent child session.
    """
    return [
        "INSERT INTO schema_version VALUES (31)",
        "INSERT INTO sessions VALUES ('%s','cli','anthropic/claude-sonnet-4',NULL,1790856000.0,NULL,NULL,"
        "'%s','main','%s','List files')" % (HERMES_SESSION, cwd, cwd),
        "INSERT INTO sessions VALUES ('%s','subagent','openai/gpt-5',"
        "'%s',1790856900.0,1790856910.0,'completed',NULL,NULL,'%s','Check disk')"
        % (HERMES_CHILD, HERMES_SESSION, cwd),
        "INSERT INTO messages (session_id,role,content,tool_call_id,tool_calls,tool_name,timestamp,finish_reason,reasoning) VALUES "
        "('%s','user','list files',NULL,NULL,NULL,1790856001.25,NULL,NULL),"
        % HERMES_SESSION
        + """('%s','assistant','',NULL,""" % HERMES_SESSION
        + """'[{"id":"call_1","call_id":"call_1","response_item_id":null,"type":"function","function":{"name":"terminal","arguments":"{\\"command\\":\\"ls\\"}"}}]',"""
        "NULL,1790856002.5,'tool_calls','User wants a listing.'),"
        "('%s','tool','a.txt' || char(10) || 'b.txt','call_1',NULL,'terminal',1790856003.0,NULL,NULL),"
        % HERMES_SESSION
        + "('%s','assistant','Two files: a.txt, b.txt.',NULL,NULL,NULL,1790856004.0,'stop',NULL)"
        % HERMES_SESSION,
        "INSERT INTO messages (session_id,role,content,timestamp,active,compacted) VALUES "
        "('%s','user','delete them instead',1790856005.0,0,1),"
        % HERMES_SESSION
        + """('%s','user',char(0) || 'json:[{"type":"text","text":"what is in this screenshot?"},"""
        """{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]',1790856006.0,1,0)"""
        % HERMES_SESSION,
        "INSERT INTO messages (session_id,role,content,timestamp) VALUES ('%s','user','df -h',1790856901.0)"
        % HERMES_CHILD,
        "INSERT INTO session_model_usage (session_id,model,billing_provider,task,api_call_count,"
        "first_seen,last_seen) VALUES "
        "('%s','anthropic/claude-sonnet-4','openrouter','',2,1790856002.5,1790856003.5),"
        "('%s','anthropic/claude-opus-4','openrouter','',1,1790856004.0,1790856004.0),"
        "('%s','google/gemini-flash','openrouter','title_generation',1,1790856001.5,1790856001.5)"
        % (HERMES_SESSION, HERMES_SESSION, HERMES_SESSION),
    ]


def hermes_diverted_records():
    """sessions/<id>.jsonl.

    Message dicts appended while state.db was replaced under a running process
    (hermes_state.py:429-444).
    """
    return [
        {"role": "user", "content": "show the env", "timestamp": 1790859600.0},
        {
            "role": "assistant",
            "content": "",
            "tool_calls": [
                {
                    "id": "call_9",
                    "type": "function",
                    "function": {"name": "read_file", "arguments": '{"path":"/srv/proj/.env"}'},
                }
            ],
            "timestamp": 1790859601.0,
        },
        {
            "role": "tool",
            "content": "TOKEN=x",
            "tool_call_id": "call_9",
            "tool_name": "read_file",
            "timestamp": 1790859602.0,
        },
    ]


HERMES_DIVERTED_REL = ".hermes/sessions/%s.jsonl" % HERMES_DIVERTED


HERMES_PRUNED = "20260920_090000_deadbeef"
HERMES_SNAPSHOT_OLD = "20261001-110000"
HERMES_SNAPSHOT = "20261001-120004-pre-update"
HERMES_SNAPSHOT_REL = ".hermes/state-snapshots/%s/state.db" % HERMES_SNAPSHOT
HERMES_SNAPSHOT_OLD_REL = ".hermes/state-snapshots/%s/state.db" % HERMES_SNAPSHOT_OLD
HERMES_SESSION_JSONL_REL = ".hermes/sessions/%s.jsonl" % HERMES_SESSION


def hermes_pruned_records(cwd="/srv/proj"):
    """A Hermes session the live state.db no longer holds.

    A session the live state.db no longer holds (pruned or deleted), as the quick snapshots keep it.
    """
    return [
        "INSERT INTO sessions VALUES ('%s','cli','anthropic/claude-sonnet-4',NULL,1789894800.0,"
        "1789894860.0,'user_exit','%s','main','%s','Audit logs')" % (HERMES_PRUNED, cwd, cwd),
        "INSERT INTO messages (session_id,role,content,timestamp) VALUES "
        "('%s','user','summarise the audit log',1789894801.0),"
        "('%s','assistant','The log shows three failed logins.',1789894802.0)"
        % (HERMES_PRUNED, HERMES_PRUNED),
    ]


def hermes_snapshot_records(cwd="/srv/proj"):
    """state-snapshots/<id>/state.db (hermes_cli/backup.py:1214-1420).

    The live database as it was at 12:00:04, the main session's first four messages and its first
    model, plus the pruned session.
    """
    live = hermes_records(cwd)
    return [
        live[0],
        live[1],
        live[3],
        "INSERT INTO session_model_usage (session_id,model,billing_provider,task,api_call_count,"
        "first_seen,last_seen) VALUES "
        "('%s','anthropic/claude-sonnet-4','openrouter','',2,1790856002.5,1790856003.5)"
        % HERMES_SESSION,
        *hermes_pruned_records(cwd),
    ]


def hermes_session_jsonl_records():
    """sessions/<main session>.jsonl.

    A divert of the main session, one message the live state.db also holds (list content, not the
    NUL json form) and one it lacks.
    """
    return [
        {
            "role": "user",
            "content": [
                {"type": "text", "text": "what is in this screenshot?"},
                {"type": "image_url", "image_url": {"url": "data:image/png;base64,AAAA"}},
            ],
            "timestamp": 1790856006.0,
        },
        {"role": "user", "content": "and the hidden files?", "timestamp": 1790856007.0},
    ]


def _hermes_snapshot(path: Path, inserts) -> None:
    """A snapshot copy: SQLite's backup API leaves a plain database, no WAL."""
    path.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(path))
    connection.executescript(HERMES_SCHEMA)
    for stmt in inserts:
        connection.execute(stmt)
    connection.commit()
    connection.close()


def build_hermes(home: Path) -> None:
    hermes = home / ".hermes"
    _wal_database(hermes / "state.db", HERMES_SCHEMA, hermes_records())
    _jsonl(home / HERMES_DIVERTED_REL, hermes_diverted_records())
    _hermes_snapshot(
        home / HERMES_SNAPSHOT_OLD_REL,
        ["INSERT INTO schema_version VALUES (31)", *hermes_pruned_records()],
    )
    _hermes_snapshot(home / HERMES_SNAPSHOT_REL, hermes_snapshot_records())
    _json(
        hermes / "state-snapshots" / HERMES_SNAPSHOT / "manifest.json",
        {"id": HERMES_SNAPSHOT, "timestamp": "20261001-120004", "label": "pre-update"},
    )
    _jsonl(home / HERMES_SESSION_JSONL_REL, hermes_session_jsonl_records())
    (hermes / "config.yaml").write_text("model: anthropic/claude-sonnet-4\n", encoding="utf-8")
    (hermes / "auth.json").write_text(
        '{"providers": {"nous": {"access_token": "not-real"}}}', encoding="utf-8"
    )
    (hermes / "sessions/sessions.json").write_text("{}", encoding="utf-8")


# Session sync: pastes/pending.json, the paste.rs links of
# `hermes debug share` uploads (hermes_cli/debug.py:38-39 at hermes-agent
# 8b66a51, research/hermes.md, Other stores): expire_at is upload + 21600 s.
HERMES_PASTES_REL = ".hermes/pastes/pending.json"


def hermes_pastes_records():
    return [
        {"url": "https://paste.rs/AbCdE", "expire_at": 1790875200.5},
        {"url": "https://paste.rs/FgHiJ", "expire_at": 1790875260.0},
    ]


def build_hermes_pastes(home: Path) -> None:
    path = home / HERMES_PASTES_REL
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(hermes_pastes_records()), encoding="utf-8")


# Model-switch markers and malformed tool_calls, in a
# database of their own so the main fixture's rows stay as they are. Shapes
# from hermes-agent 86fa9e6ca1de7ee71132d2be0103f3e37d1fa872: the TUI
# gateway appends a `role = 'user'` row with `display_kind = 'model_switch'`
# and the marker text, after setting `active = 0` on the session's earlier
# markers (tui_gateway/server.py:1274-1331, hermes_state_messages.py:1064-1070);
# `display_kind` and `display_metadata` are messages columns
# (hermes_state_common.py:538-539). No real install has shown either shape.
HERMES_SWITCH = "20261001_170000_5ca1ab1e"
HERMES_SWITCH_SCHEMA = (
    HERMES_SCHEMA
    + "ALTER TABLE messages ADD COLUMN display_kind TEXT;\n"
    + "ALTER TABLE messages ADD COLUMN display_metadata TEXT;\n"
)
HERMES_SWITCH_MARKER = (
    "[System: The active model for this chat has changed to %s%s. From this point forward, "
    "use this runtime metadata when answering questions about what model/provider is active.]"
)
# tool_calls values that do not decode to a list of call objects.
HERMES_BAD_CALLS_JSON = '[{"id":"call_7","function":{"name":"terminal","argu'
HERMES_BAD_CALLS_SCALAR = "42"
HERMES_BAD_CALLS_ENTRY = (
    '["oops",{"id":"call_8","type":"function",'
    '"function":{"name":"terminal","arguments":"{\\"command\\":\\"pwd\\"}"}}]'
)


def hermes_switch_records():
    """A Hermes TUI session with two model switches and undecodable calls.

    A TUI session (sessions.model rewritten to the last switch's model) whose first API calls used
    claude-sonnet-4, then two switches: the first marker, superseded, has active=0; the second names
    no provider, and a third marker's text has an unknown form. Between them three assistant
    messages whose tool_calls do not decode, with plain user turns before and after.
    """
    session_id = HERMES_SWITCH
    marker = HERMES_SWITCH_MARKER

    def message_row(role, content, timestamp, active=1, kind=None, calls=None):
        return (
            "INSERT INTO messages (session_id,role,content,tool_calls,timestamp,active,"
            "display_kind) VALUES (%s,%s,%s,%s,%s,%d,%s)"
            % (
                _hermes_sql(session_id),
                _hermes_sql(role),
                _hermes_sql(content),
                _hermes_sql(calls),
                timestamp,
                active,
                _hermes_sql(kind),
            )
        )

    return [
        "INSERT INTO schema_version VALUES (31)",
        "INSERT INTO sessions VALUES ('%s','tui','anthropic/claude-opus-4',NULL,1790874000.0,"
        "NULL,NULL,'/srv/proj','main','/srv/proj','Switch models')" % session_id,
        message_row("user", "hello", 1790874001.0),
        message_row("assistant", "Hi.", 1790874002.0),
        message_row(
            "user",
            marker % ("openai/gpt-5", " via provider openrouter"),
            1790874010.0,
            active=0,
            kind="model_switch",
        ),
        message_row("user", "list files", 1790874011.0),
        message_row("assistant", "bad json", 1790874012.0, calls=HERMES_BAD_CALLS_JSON),
        message_row("assistant", "", 1790874013.0, calls=HERMES_BAD_CALLS_SCALAR),
        message_row("assistant", "", 1790874014.0, calls=HERMES_BAD_CALLS_ENTRY),
        message_row(
            "user", marker % ("anthropic/claude-opus-4", ""), 1790874020.0, kind="model_switch"
        ),
        message_row("user", "[System: model changed]", 1790874021.0, kind="model_switch"),
        message_row("user", "after the switches", 1790874030.0),
        "INSERT INTO session_model_usage (session_id,model,billing_provider,task,api_call_count,"
        "first_seen,last_seen) VALUES "
        "('%s','anthropic/claude-sonnet-4','openrouter','',1,1790874002.0,1790874002.0)"
        % session_id,
    ]


def _hermes_sql(value) -> str:
    """`value` as an SQL literal: NULL, or a single-quoted string."""
    if value is None:
        return "NULL"
    return "'%s'" % str(value).replace("'", "''")


def build_hermes_switch(path: Path) -> None:
    """state.db at `path` holding hermes_switch_records()."""
    _wal_database(path, HERMES_SWITCH_SCHEMA, hermes_switch_records())


# Source-derived from hermes-agent 86fa9e6ca1de7ee71132d2be0103f3e37d1fa872
# (research/hermes.md, Parser plan); no real install has shown these shapes.
# A gateway state.db of its own: a Telegram session (sessions columns of gateway/session.py and
# hermes_state_gateway.py:236-292, its /yolo flag in model_config) with the
# three terminal approval outcomes (tools/terminal_tool.py:1062-1087), a
# session imported from Claude Code (hermes_cli/foreign_sessions.py:254-260),
# a `hermes -w` worktree session (hermes_cli/worktree_ops.py:435-441), a
# session whose origin_json does not decode, the deletion fences of
# hermes_state_mutation_retirement.py, a conversation_generations row and a
# Bot Mode room log (gateway/hosted_rooms.py, gateway/hosted_room_discussion.py).
HERMES_TG = "20261001_180000_7e1e9a40"
HERMES_IMPORTED = "20261001_181000_1a2b3c4d"
HERMES_WORKTREE = "20261001_182000_5e6f7a8b"
HERMES_BADORIGIN = "20261001_183000_9c0d1e2f"
HERMES_DELETED = "20260915_090000_0ddba11"
HERMES_SWEPT = "20260601_090000_5ca1ed"
HERMES_GATEWAY_SCHEMA = """
CREATE TABLE schema_version (version INTEGER NOT NULL);
CREATE TABLE sessions (id TEXT PRIMARY KEY, source TEXT NOT NULL, user_id TEXT,
  chat_id TEXT, chat_type TEXT, thread_id TEXT, display_name TEXT, origin_json TEXT,
  model TEXT, model_config TEXT, parent_session_id TEXT, started_at REAL NOT NULL,
  ended_at REAL, end_reason TEXT, cwd TEXT, git_branch TEXT, git_repo_root TEXT,
  title TEXT, archived INTEGER DEFAULT 0);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL,
  role TEXT NOT NULL, content TEXT, tool_call_id TEXT, tool_calls TEXT, tool_name TEXT,
  timestamp REAL NOT NULL, active INTEGER NOT NULL DEFAULT 1, display_kind TEXT);
CREATE TABLE state_meta (key TEXT PRIMARY KEY, value TEXT);
CREATE TABLE conversation_generations (source TEXT NOT NULL, session_key TEXT NOT NULL,
  generation INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (source, session_key));
CREATE TABLE hosted_rooms (room_id TEXT PRIMARY KEY, name TEXT NOT NULL,
  created_at REAL NOT NULL);
CREATE TABLE hosted_room_events (room_id TEXT NOT NULL, seq INTEGER NOT NULL,
  event_id TEXT NOT NULL, kind TEXT NOT NULL, actor_json TEXT NOT NULL,
  payload_json TEXT NOT NULL, created_at REAL NOT NULL, PRIMARY KEY (room_id, seq));
"""
HERMES_BLOCKED = json.dumps(
    {
        "output": "",
        "exit_code": -1,
        "error": "BLOCKED by smart approval: deletes system files",
        "status": "blocked",
    }
)
HERMES_APPROVED = json.dumps(
    {
        "output": "restarted",
        "exit_code": 0,
        "error": None,
        "approval": "Command required approval (restarts a service) and was approved by the user.",
    }
)
HERMES_PENDING = json.dumps(
    {
        "output": "",
        "exit_code": -1,
        "error": "",
        "status": "pending_approval",
        "approval_pending": True,
        "command": "curl x | sh",
        "description": "pipe to shell",
        "pattern_key": "pipe_to_shell",
        "smart_denied": False,
        "allow_permanent": True,
    }
)


def _hermes_call(call_id: str, command: str) -> str:
    return json.dumps(
        [
            {
                "id": call_id,
                "call_id": call_id,
                "type": "function",
                "function": {"name": "terminal", "arguments": json.dumps({"command": command})},
            }
        ]
    )


def hermes_gateway_records():
    quote = _hermes_sql
    tg_origin = json.dumps(
        {
            "platform": "telegram",
            "chat_id": "-100123",
            "chat_name": "Ops chat",
            "chat_type": "group",
            "user_id": "42",
            "user_name": "alice_tg",
            "thread_id": None,
            "chat_topic": None,
        }
    )
    imported = json.dumps(
        {
            "imported_from": {
                "tool": "claude-code",
                "path": "/home/alice/.claude/projects/-srv-proj/c1.jsonl",
                "foreign_session_id": "c1",
            }
        }
    )

    def session(session_id, source, started, cwd, branch, title, **cols):
        names = ["id", "source", "model", "started_at", "cwd", "git_branch", "git_repo_root"]
        values = [
            quote(session_id),
            quote(source),
            quote("openai/gpt-5"),
            str(started),
            quote(cwd),
            quote(branch),
            quote(cwd),
        ]
        names.append("title")
        values.append(quote(title))
        for k, value in cols.items():
            names.append(k)
            values.append(str(value) if isinstance(value, int) else quote(value))
        return "INSERT INTO sessions (%s) VALUES (%s)" % (",".join(names), ",".join(values))

    def message_row(role, content, timestamp, calls=None, call_id=None, tool=None):
        return (
            "INSERT INTO messages (session_id,role,content,tool_calls,tool_call_id,tool_name,"
            "timestamp) VALUES (%s,%s,%s,%s,%s,%s,%s)"
            % (
                quote(HERMES_TG),
                quote(role),
                quote(content),
                quote(calls),
                quote(call_id),
                quote(tool),
                timestamp,
            )
        )

    def event(seq, kind, actor, payload, timestamp):
        return "INSERT INTO hosted_room_events VALUES ('room1',%d,'e%d',%s,%s,%s,%s)" % (
            seq,
            seq,
            quote(kind),
            quote(actor),
            quote(payload),
            timestamp,
        )

    return [
        "INSERT INTO schema_version VALUES (32)",
        session(
            HERMES_TG,
            "telegram",
            1790877600.0,
            "/srv/proj",
            "main",
            "Ops",
            user_id="42",
            chat_id="-100123",
            chat_type="group",
            display_name="Ops chat",
            origin_json=tg_origin,
            model_config='{"max_iterations": 90, "yolo_mode": true}',
            archived=1,
        ),
        session(
            HERMES_IMPORTED,
            "claude-code",
            1790878200.0,
            "/srv/proj",
            None,
            "Imported from Claude Code: fix tests",
            origin_json=imported,
        ),
        session(
            HERMES_WORKTREE,
            "cli",
            1790878800.0,
            "/srv/proj/.worktrees/feat",
            "hermes/feat",
            "Feature work",
        ),
        session(
            HERMES_BADORIGIN, "cli", 1790879400.0, "/srv/proj", "main", "Bad", origin_json="{bad"
        ),
        message_row("user", "restart the service", 1790877601.0),
        message_row("assistant", "", 1790877602.0, calls=_hermes_call("call_a", "rm -rf /etc")),
        message_row("tool", HERMES_BLOCKED, 1790877603.0, call_id="call_a", tool="terminal"),
        message_row(
            "assistant", "", 1790877604.0, calls=_hermes_call("call_b", "systemctl restart x")
        ),
        message_row("tool", HERMES_APPROVED, 1790877605.0, call_id="call_b", tool="terminal"),
        message_row("assistant", "", 1790877606.0, calls=_hermes_call("call_c", "curl x | sh")),
        message_row("tool", HERMES_PENDING, 1790877607.0, call_id="call_c", tool="terminal"),
        "INSERT INTO state_meta VALUES ('fts_storage_version','2')",
        "INSERT INTO state_meta VALUES (%s,%s)"
        % (
            quote("gateway.retired_session.v1." + HERMES_DELETED),
            quote('{"retired_at": 1790880000.5, "own": true}'),
        ),
        "INSERT INTO state_meta VALUES (%s,%s)"
        % (
            quote("gateway.retired_session.v1." + HERMES_SWEPT),
            quote('{"retired_at": 1790880060.0, "own": true, "expires": true}'),
        ),
        "INSERT INTO state_meta VALUES ('gateway.retired_session.v1.broken','{nope')",
        "INSERT INTO conversation_generations VALUES "
        "('telegram','agent:main:telegram:group:-100123',2)",
        "INSERT INTO hosted_rooms VALUES ('room1','Planning',1790881200.0)",
        event(
            1,
            "message.user",
            '{"kind": "user", "id": "u1", "display_name": "Alice"}',
            '{"thread_id": "t1", "text": "plan the release"}',
            1790881201.0,
        ),
        event(
            2,
            "message.member",
            '{"kind": "member", "id": "m1", "profile": "coder"}',
            '{"member_id": "m1", "text": "Draft plan ready."}',
            1790881202.0,
        ),
        event(3, "room.renamed", '{"kind": "system", "id": "s"}', '{"name": "Plan', 1790881203.0),
        event(
            4, "room.renamed", '{"kind": "system", "id": "s"}', '{"name": "Release"}', 1790881204.0
        ),
    ]


def build_hermes_gateway(path: Path) -> None:
    """state.db at `path` holding hermes_gateway_records()."""
    _wal_database(path, HERMES_GATEWAY_SCHEMA, hermes_gateway_records())


# .hermes_history: prompt_toolkit's FileHistory form (python-prompt-toolkit
# 3.0.52 history.py:297-306), written by the classic CLI with a host-local
# microsecond time, and by the TUI with a UTC millisecond time
# (ui-tui/src/lib/history.ts:71-78 at hermes-agent 86fa9e6).
HERMES_HISTORY_TEXT = (
    "\n# 2026-10-01 12:00:01.250\n+list files\n"
    "\n# 2026-10-01 14:00:05.123456\n+/model openai/gpt-5\n"
    "\n# 2026-10-01 14:01:00.000001\n+first line\n+second line\n"
)

# cron/output/<job>/<time>.md (cron/scheduler.py:2238-2246 and 2634-2640 at
# 86fa9e6): a framed run, an unframed run of an older writer and a failed run.
HERMES_CRON_JOB = "a1b2c3d4e5f6"
HERMES_CRON_PROMPT = "Summarise the open issues.\n\n## Response\n\nquoted heading"
HERMES_CRON_REPLY = "Three issues are open."


def hermes_cron_docs() -> dict[str, str]:
    head = (
        "# Cron Job: Daily summary\n\n**Job ID:** %s\n**Run Time:** %s\n"
        "**Schedule:** every day at 09:00\n\n"
    )
    framed = (
        head % (HERMES_CRON_JOB, "2026-10-01 09:00:00")
        + "**Prompt Characters:** %d\n## Prompt\n\n%s\n\n" % (len(HERMES_CRON_PROMPT), HERMES_CRON_PROMPT)
        + "**Response Characters:** %d\n## Response\n\n%s\n" % (len(HERMES_CRON_REPLY), HERMES_CRON_REPLY)
    )  # fmt: skip
    legacy = (
        head % (HERMES_CRON_JOB, "2026-09-30 09:00:00")
        + "## Prompt\n\nSummarise yesterday.\n\n## Response\n\nNothing new.\n"
    )
    failed = "# Cron Job: Daily summary (FAILED)\n\nError: provider unreachable\n"
    return {
        "2026-10-01_09-00-00.md": framed,
        "2026-09-30_09-00-00.md": legacy,
        "2026-09-29_09-00-00.md": failed,
    }


# response_store.db (gateway/platforms/api_server_response_store.py:58-61 and
# api_server_openai_routes.py:347-358 at 86fa9e6): two responses of one API
# server session, the second history extending the first, and a row whose
# data does not decode.
HERMES_API_SESSION = "20261001_190000_a9b8c7d6"


def build_hermes_responses(path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    first = [
        {"role": "user", "content": "what is 2+2?"},
        {"role": "assistant", "content": "4"},
    ]
    second = [
        *first,
        {"role": "user", "content": "and 3+3?"},
        {"role": "assistant", "content": "6"},
    ]

    def data(rid, created, history):
        return json.dumps(
            {
                "response": {
                    "id": rid,
                    "object": "response",
                    "status": "completed",
                    "created_at": created,
                    "model": "hermes-agent",
                },
                "conversation_history": history,
                "instructions": None,
                "session_id": HERMES_API_SESSION,
            }
        )

    connection = sqlite3.connect(str(path))
    connection.executescript(
        "CREATE TABLE responses (response_id TEXT PRIMARY KEY, data TEXT NOT NULL,"
        " accessed_at REAL NOT NULL);"
        "CREATE TABLE conversations (name TEXT PRIMARY KEY, response_id TEXT NOT NULL);"
    )
    connection.execute(
        "INSERT INTO responses VALUES (?,?,?)",
        ("resp_2", data("resp_2", 1790884810, second), 1790884811.0),
    )
    connection.execute(
        "INSERT INTO responses VALUES (?,?,?)",
        ("resp_1", data("resp_1", 1790884800, first), 1790884801.0),
    )
    connection.execute("INSERT INTO responses VALUES ('resp_bad','{cut',1790884900.0)")
    connection.execute("INSERT INTO conversations VALUES ('math','resp_2')")
    connection.commit()
    connection.close()


# pairing/<platform>-approved.json (gateway/pairing.py:431-438 at 86fa9e6).
HERMES_PAIRING = {
    "42": {"user_name": "alice_tg", "approved_at": 1790856000.0},
    "43": "not an object",
}

# config.yaml with a command_allowlist an "always" answer wrote, in
# hermes_yaml's block style (hermes_yaml.py:64-72 at 86fa9e6).
HERMES_CONFIG_YAML = (
    "model: anthropic/claude-sonnet-4\n"
    "approvals:\n"
    "  mode: smart\n"
    "command_allowlist:\n"
    "  - rm -rf /tmp/*\n"
    '  - "git push --force*"\n'
    "  - 'echo ''hi'''  # a comment\n"
    "quick_commands: {}\n"
)

# The desktop app's connections.json (apps/desktop/electron/connection-registry.ts:49-106
# at 86fa9e6): the local runtime, a remote gateway, an SSH host and a Hermes
# Cloud instance, and one entry the app quarantined.
HERMES_CONNECTIONS = {
    "version": 2,
    "primary": "local",
    "launchMode": "last-used",
    "lastUsed": "local",
    "connections": [
        {"id": "local", "kind": "local", "label": "This Mac"},
        {
            "id": "c1",
            "kind": "remote",
            "label": "Homelab",
            "url": "https://hermes.example.net",
            "authMode": "token",
            "token": {"encoding": "plain", "value": "not-real"},
        },
        {
            "id": "c2",
            "kind": "ssh",
            "label": "Build box",
            "host": "build",
            "user": "ci",
            "port": 22,
        },
        {
            "id": "c3",
            "kind": "cloud",
            "label": "Cloud",
            "url": "https://x.cloud.example",
            "org": "acme",
            "name": "prod",
        },
        "not an object",
    ],
    "quarantined": [{"reason": "missing label", "entry": {"kind": "remote"}}],
}


# trajectory_samples.jsonl (agent/trajectory.py:37-41 and
# agent/agent_runtime_helpers.py:170-196 at 86fa9e6), in a project directory.
def hermes_trajectory_records():
    return [
        {
            "conversations": [
                {"from": "system", "value": "You are a function calling AI model."},
                {"from": "human", "value": "list files"},
                {"from": "gpt", "value": "<tool_call>ls</tool_call>"},
                {"from": "tool", "value": "<tool_response>a.txt</tool_response>"},
                {"from": "gpt", "value": "One file."},
            ],
            "timestamp": "2026-10-01T12:00:05.000123",
            "model": "anthropic/claude-sonnet-4",
            "completed": True,
        },
        {"conversations": "not a list", "timestamp": "2026-10-01T12:10:00", "model": "x"},
    ]


# A Hermes home in a Docker named volume: the catalog's
# `hermes|*hermes*` DOCKER_VOLUMES line collects the volume whole with its
# `_data` directory as the base (user `docker`). The container's Hermes home
# is /opt/data (docker-compose.yml:36-37 at hermes-agent 8b66a51 mounts
# ~/.hermes there), so a named volume mounted in its place holds the Hermes
# home itself and state.db is at the base's root.
HERMES_VOLUME_HOME = "/var/lib/docker/volumes/hermes_data/_data"
HERMES_VOLUME_FILES = (
    "state.db",
    "state.db-wal",
    "state.db-shm",
    "sessions/%s.jsonl" % HERMES_DIVERTED,
    "pastes/pending.json",
    "config.yaml",
)


def build_hermes_volume(root: Path, cwd="/opt/data/workspace") -> None:
    """Write a collector output directory holding only the Hermes volume.

    A collector output directory (collection.json, manifest.jsonl and fs/) holding only the Hermes
    volume: the main fixture's state.db with `cwd` as the project path, its divert, the debug-share
    list and a config.yaml with no command_allowlist, which gives no rows.
    """
    base = root / ("fs" + HERMES_VOLUME_HOME)
    _wal_database(base / "state.db", HERMES_SCHEMA, hermes_records(cwd))
    _jsonl(base / "sessions" / ("%s.jsonl" % HERMES_DIVERTED), hermes_diverted_records())
    _json(base / "pastes" / "pending.json", hermes_pastes_records())
    (base / "config.yaml").write_text("model: anthropic/claude-sonnet-4\n", encoding="utf-8")
    _json(root / "collection.json", {"hostname": "h2"})
    _jsonl(
        root / "manifest.jsonl",
        [
            {
                "user": "docker",
                "home": HERMES_VOLUME_HOME,
                "agent": "hermes",
                "path": HERMES_VOLUME_HOME + "/" + rel,
                "archive_path": "fs" + HERMES_VOLUME_HOME + "/" + rel,
                "type": "file",
                "status": "collected",
                "secret": False,
            }
            for rel in HERMES_VOLUME_FILES
        ],
    )
