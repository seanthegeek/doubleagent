"""Synthetic nanobot state for the analyzer tests."""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path

from .common import _jsonl

# -- nanobot (HKUDS/nanobot at acdae3d) ------------------------------------------
NANOBOT_KEY = "telegram:123456789"
NANOBOT_WORKSPACE = "0123456789abcdef0123456789abcdef"
NANOBOT_REL = ".nanobot/sessions/%s/dGVsZWdyYW06MTIzNDU2Nzg5.jsonl" % NANOBOT_WORKSPACE
NANOBOT_LEGACY_REL = ".nanobot/sessions/cli_direct.jsonl"
NANOBOT_HISTORY_REL = ".nanobot/workspace/memory/history.jsonl"


def nanobot_records():
    """The nanobot research sample file and two more lines.

    The research document's sample file (Parser plan), plus a hidden-history message and a
    provider_state line.
    """
    return [
        {
            "_type": "metadata",
            "key": NANOBOT_KEY,
            "created_at": "2026-10-01T10:15:00.000001",
            "updated_at": "2026-10-01T10:15:09.500000",
            "metadata": {"last_channel": NANOBOT_KEY},
            "last_archived": 0,
            "last_consolidated": 0,
        },
        {"_type": "provider_state", "state": {"opaque": "x"}},
        {
            "role": "user",
            "content": "[earlier conversation summarised]",
            "_hidden_history": True,
            "timestamp": "2026-10-01T10:15:00.500000",
        },
        {
            "role": "user",
            "content": "what is using port 8080?",
            "timestamp": "2026-10-01T10:15:01.200000",
        },
        {
            "role": "assistant",
            "content": "",
            "tool_calls": [
                {
                    "id": "call_abc123",
                    "type": "function",
                    "function": {
                        "name": "exec",
                        "arguments": '{"command": "ss -ltnp | grep 8080"}',
                    },
                }
            ],
            "reasoning_content": "Check listening sockets.",
            "timestamp": "2026-10-01T10:15:04.000000",
        },
        {
            "role": "tool",
            "tool_call_id": "call_abc123",
            "name": "exec",
            "content": 'LISTEN 0 128 *:8080 users:(("python3",pid=4242))',
            "timestamp": "2026-10-01T10:15:05.000000",
        },
        {
            "role": "assistant",
            "content": "python3 (pid 4242) is listening on 8080.",
            "timestamp": "2026-10-01T10:15:09.400000",
        },
    ]


def build_nanobot(home: Path) -> None:
    _jsonl(home / NANOBOT_REL, nanobot_records())
    (home / NANOBOT_REL).parent.joinpath(".workspace").write_text("/srv/proj\n", encoding="utf-8")
    (home / NANOBOT_REL).parent.joinpath("dGVsZWdyYW06MTIzNDU2Nzg5.checkpoint.json").write_text(
        "{}", encoding="utf-8"
    )
    _jsonl(
        home / NANOBOT_LEGACY_REL,
        [
            {
                "_type": "metadata",
                "key": "cli:direct",
                "created_at": "2026-09-01T08:00:00",
                "updated_at": "2026-09-01T08:00:05",
                "metadata": {"_nanobot_model_preset": "fast"},
            },
            {
                "role": "user",
                "content": [{"type": "text", "text": "hello"}],
                "timestamp": "2026-09-01T08:00:01",
            },
            {"role": "assistant", "content": "hi"},
        ],
    )
    _jsonl(
        home / NANOBOT_HISTORY_REL,
        [
            {
                "cursor": 1,
                "timestamp": "2026-09-30 22:10",
                "content": "User asked about disk usage.",
                "session_key": NANOBOT_KEY,
            },
            {
                "cursor": 2,
                "timestamp": "2026-10-01 10:20",
                "content": "Found python3 on port 8080.",
            },
        ],
    )
    (home / ".nanobot/config.json").write_text(json.dumps(nanobot_config()), encoding="utf-8")
    build_nanobot_sqlite(home)
    build_nanobot_sweep(home)


# nanobot permission, relay, model and run-record sources, from
# HKUDS/nanobot at 909b277: config/schema.py:428-458,
# agent/tools/shell_config.py:11-28, channels/base.py:244-257,
# pairing/store.py:73-85 and 130-160, command/builtin.py:405-448,
# security/workspace_access.py:80-84 and 145-154, utils/run_records.py:19-29,
# cron/bound_runner.py:63-146, triggers/local_store.py:292-310 and 439-464,
# triggers/local_runner.py:212-214, webui/session_list_index.py:198-205 and
# 416-497. Source-derived; no real install.
NANOBOT_TOKEN = "sk-not-real"
NANOBOT_UNIFIED_REL = ".nanobot/sessions/%s/dW5pZmllZDpkZWZhdWx0.jsonl" % NANOBOT_WORKSPACE
NANOBOT_PAIRING_REL = ".nanobot/pairing.json"
NANOBOT_CONFIG_REL = ".nanobot/config.json"
NANOBOT_SIDEBAR_REL = ".nanobot/sessions/%s/.webui_session_index.json" % NANOBOT_WORKSPACE
NANOBOT_CRON_RUN_REL = ".nanobot/workspace/cron/runs/job1_1791187200000_ab12cd34.json"
NANOBOT_TRIGGER_RUN_REL = ".nanobot/workspace/triggers/runs/d-1.json"
NANOBOT_RUN_KEY = "slack:C42"
NANOBOT_SWEEP_RELS = {
    "unified": NANOBOT_UNIFIED_REL,
    "pairing": NANOBOT_PAIRING_REL,
    "config": NANOBOT_CONFIG_REL,
    "sidebar": NANOBOT_SIDEBAR_REL,
    "cron": NANOBOT_CRON_RUN_REL,
    "trigger": NANOBOT_TRIGGER_RUN_REL,
    "run_key": NANOBOT_RUN_KEY,
}


def nanobot_config():
    """The nanobot `config.json` fixture.

    `config.json` with the permission settings the parser reads, a channel without `allowFrom`, and
    a provider key it must never show.
    """
    return {
        "providers": {"openai": {"apiKey": NANOBOT_TOKEN}},
        "tools": {
            "restrictToWorkspace": True,
            "ssrfWhitelist": ["100.64.0.0/10"],
            "exec": {
                "enable": True,
                "sandbox": "bwrap",
                "denyPatterns": ["rm -rf /", "curl *"],
                "timeout": 60,
            },
        },
        "channels": {
            "sendProgress": True,
            "telegram": {"enabled": True, "token": "123:abc", "allowFrom": ["123456789"]},
            "slack": {"enabled": False},
        },
    }


def build_nanobot_sweep(home: Path) -> None:
    """Write a nanobot Telegram session with model switches and its stores.

    A `unified:default` session answering through Telegram with two `/model` switches,
    `pairing.json`, a cron and a trigger run record of a Slack session that is gone, the noise
    beside them, and the WebUI sidebar cache.
    """
    switch = "Switched model preset to `%s`.\n- Scope: current session\n- Model: `%s`\n"
    _jsonl(
        home / NANOBOT_UNIFIED_REL,
        [
            {
                "_type": "metadata",
                "key": "unified:default",
                "created_at": "2026-10-04T08:00:00",
                "updated_at": "2026-10-04T08:05:00",
                "metadata": {
                    "last_channel": "telegram:777",
                    "_nanobot_model_preset": "smart",
                },
            },
            {"role": "user", "content": "good morning", "timestamp": "2026-10-04T08:00:01"},
            {"role": "assistant", "content": "morning!", "timestamp": "2026-10-04T08:00:02"},
            {
                "role": "user",
                "content": "/model fast",
                "_command": True,
                "timestamp": "2026-10-04T08:01:00",
            },
            {
                "role": "assistant",
                "content": switch % ("fast", "gpt-5.4-mini") + "- Context window: 128000",
                "_command": True,
                "timestamp": "2026-10-04T08:01:00",
            },
            {"role": "user", "content": "/model smart", "_command": True},
            {
                "role": "assistant",
                "content": switch % ("smart", "claude-opus-5-5"),
                "_command": True,
                "timestamp": "2026-10-04T08:02:00",
            },
            {"role": "user", "content": "/model nope", "_command": True},
            {
                "role": "assistant",
                "content": "Could not switch model preset: nope\n\nAvailable presets: fast, smart",
                "_command": True,
                "timestamp": "2026-10-04T08:03:00",
            },
        ],
    )
    (home / NANOBOT_PAIRING_REL).write_text(
        json.dumps(
            {
                "approved": {"telegram": ["123456789", "555"]},
                "pending": {
                    "ABCD-EFGH": {
                        "channel": "discord",
                        "sender_id": "999",
                        "created_at": 1791187200.5,
                        "expires_at": 1791187800.5,
                        "delivery_status": "sent",
                    }
                },
            }
        ),
        encoding="utf-8",
    )
    cron = home / NANOBOT_CRON_RUN_REL
    cron.parent.mkdir(parents=True, exist_ok=True)
    cron.write_text(
        json.dumps(
            {
                "job_id": "job1",
                "job_name": "morning digest",
                "session_key": NANOBOT_RUN_KEY,
                "prompt_ref": "x",
                "prompt_vars": {"message": "summarise the inbox"},
                "rendered_prompt": "Reminder: summarise the inbox",
                "status": "ok",
                "response": "Three new mails.",
                "run_id": "job1:1791187200000:ab12cd34",
                "updated_at_ms": 1791187205000,
            }
        ),
        encoding="utf-8",
    )
    (cron.parent.parent / "jobs.json").write_text('{"jobs": []}', encoding="utf-8")
    (cron.parent.parent / "action.jsonl").write_text("", encoding="utf-8")
    trig = home / NANOBOT_TRIGGER_RUN_REL
    trig.parent.mkdir(parents=True, exist_ok=True)
    trig.write_text(
        json.dumps(
            {
                "kind": "local_trigger",
                "trigger_id": "t1",
                "delivery_id": "d-1",
                "content": "build failed on main",
                "created_at_ms": 1791190800000,
                "attempts": 1,
                "trigger_name": "ci",
                "session_key": NANOBOT_RUN_KEY,
                "channel": "slack",
                "chat_id": "C42",
                "sender_id": "trigger",
                "origin_metadata": {},
                "status": "error",
                "error": "target channel is not enabled: slack",
                "run_id": "d-1",
                "updated_at_ms": 1791190801000,
            }
        ),
        encoding="utf-8",
    )
    (trig.parent.parent / "triggers.json").write_text('{"triggers": []}', encoding="utf-8")
    (trig.parent.parent / "inbox").mkdir()
    (trig.parent.parent / "inbox/d-2.json").write_text(
        '{"id": "d-2", "triggerId": "t1", "content": "queued"}', encoding="utf-8"
    )
    (home / NANOBOT_SIDEBAR_REL).write_text(
        json.dumps(
            {
                "version": 8,
                "sessions": [
                    {
                        "key": "websocket:chat-9",
                        "created_at": "2026-10-04T09:00:00",
                        "updated_at": "2026-10-04T09:10:00",
                        "title": "",
                        "preview": "draft the release notes",
                        "model_preset": None,
                        "recovery_state": None,
                        "_workspace_scope_present": False,
                        "_workspace_scope_value": None,
                        "_source": "webui_transcript",
                        "file": "websocket_chat-9",
                    }
                ],
            }
        )
        + "\n",
        encoding="utf-8",
    )


# nanobot WebUI mirror and CLI history. The host is taken to be
# at UTC+2: the session file's local times read two hours later than the
# WebUI's true-UTC created_at_ms.
NANOBOT_WEBUI_KEY = "websocket:chat-1"
NANOBOT_WEBUI_SESSION_REL = ".nanobot/sessions/%s/d2Vic29ja2V0OmNoYXQtMQ.jsonl" % NANOBOT_WORKSPACE
NANOBOT_WEBUI_REL = ".nanobot/webui/websocket_chat-1.jsonl"
NANOBOT_WEBUI_SEGMENT_REL = ".nanobot/webui/websocket_chat-1.segments/000001.jsonl"
NANOBOT_CLI_HISTORY_REL = ".nanobot/history/cli_history"


def nanobot_webui_records():
    """The active WebUI file: one turn the session file also holds."""
    turn = {"chat_id": "chat-1", "turn_id": "t-2"}
    return [
        {"event": "user", **turn, "text": "show disk usage", "created_at_ms": 1790924400900},
        {
            "event": "reasoning_end",
            **turn,
            "text": "Run df.",
            "created_at_ms": 1790924401500,
        },
        {
            "event": "message",
            **turn,
            "kind": "tool_hint",
            "text": 'exec("df -h")',
            "tool_events": [
                {
                    "version": 1,
                    "phase": "start",
                    "call_id": "call_df1",
                    "name": "exec",
                    "arguments": {"command": "df -h"},
                    "result": None,
                }
            ],
            "created_at_ms": 1790924402000,
        },
        {
            "event": "message",
            **turn,
            "kind": "progress",
            "text": "",
            "tool_events": [
                {
                    "version": 1,
                    "phase": "end",
                    "call_id": "call_df1",
                    "name": "exec",
                    "arguments": {"command": "df -h"},
                    "result": "/dev/sda1 42%",
                }
            ],
            "created_at_ms": 1790924404000,
        },
        {
            "event": "file_edit",
            **turn,
            "edits": [
                {
                    "call_id": "call_w1",
                    "tool": "write_file",
                    "path": "notes.txt",
                    "absolute_path": "/srv/proj/notes.txt",
                    "phase": "end",
                    "status": "done",
                    "added": 1,
                    "deleted": 0,
                }
            ],
            "created_at_ms": 1790924404500,
        },
        {"event": "delta", **turn, "text": "The disk", "created_at_ms": 1790924405000},
        {
            "event": "stream_end",
            **turn,
            "text": "The disk is",
            "merge_next": True,
            "created_at_ms": 1790924405200,
        },
        {
            "event": "stream_end",
            **turn,
            "text": "The disk is 42% full.",
            "created_at_ms": 1790924405500,
        },
        {"event": "turn_end", **turn, "latency_ms": 5100, "created_at_ms": 1790924406000},
    ]


def build_nanobot_webui(home: Path) -> None:
    """Write a nanobot WebUI chat, its mirror and the CLI history.

    A WebUI chat whose session file holds only the second turn (a `/new` emptied the first), its
    WebUI mirror (the first turn rotated into a segment), and the CLI history with one prompt of the
    legacy cli:direct session and one whose session is gone.
    """
    _jsonl(
        home / NANOBOT_WEBUI_SESSION_REL,
        [
            {
                "_type": "metadata",
                "key": NANOBOT_WEBUI_KEY,
                "created_at": "2026-10-02T09:00:00",
                "updated_at": "2026-10-02T09:00:06",
                "metadata": {
                    "last_channel": NANOBOT_WEBUI_KEY,
                    "workspace_scope": {"project_path": "/srv/proj", "access_mode": "restricted"},
                },
            },
            {"role": "user", "content": "show disk usage", "timestamp": "2026-10-02T09:00:01"},
            {
                "role": "assistant",
                "content": "",
                "tool_calls": [
                    {
                        "id": "call_df1",
                        "type": "function",
                        "function": {"name": "exec", "arguments": '{"command": "df -h"}'},
                    }
                ],
                "timestamp": "2026-10-02T09:00:06",
            },
            {
                "role": "tool",
                "tool_call_id": "call_df1",
                "name": "exec",
                "content": "/dev/sda1 42%",
                "timestamp": "2026-10-02T09:00:06",
            },
            {
                "role": "assistant",
                "content": "The disk is 42% full.",
                "timestamp": "2026-10-02T09:00:06",
            },
        ],
    )
    _jsonl(home / NANOBOT_WEBUI_REL, nanobot_webui_records())
    old = {"chat_id": "chat-1", "turn_id": "t-1"}
    _jsonl(
        home / NANOBOT_WEBUI_SEGMENT_REL,
        [
            {"event": "user", **old, "text": "list the old logs", "created_at_ms": 1789905600000},
            {
                "event": "message",
                **old,
                "text": "There are 3 old logs.",
                "created_at_ms": 1789905603000,
            },
            {"event": "fork_marker", "chat_id": "chat-1", "created_at_ms": 1789905604000},
        ],
    )
    segments = (home / NANOBOT_WEBUI_SEGMENT_REL).parent
    (segments / "manifest.json").write_text(
        '{"version": 2, "segments": [{"id": "000001", "turn_count": 1}]}', encoding="utf-8"
    )
    (home / ".nanobot/webui/sidebar-state.json").write_text("{}", encoding="utf-8")
    (home / NANOBOT_CLI_HISTORY_REL).parent.mkdir(parents=True, exist_ok=True)
    (home / NANOBOT_CLI_HISTORY_REL).write_text(
        "\n# 2026-09-01 08:00:01.200000\n+hello\n"
        "\n# 2026-09-01 07:59:00.000000\n+first line\n+second line\n",
        encoding="utf-8",
    )


# nanobot's SQLite session store, from the schema and import of
# HKUDS/nanobot at 909b277 (nanobot/session/sqlite_store.py:44-73, 185-200;
# nanobot/session/jsonl_migration.py:155-197, 221-232); source-derived, no
# real install. A second namespace holds the database, the frozen JSONL of
# the session it imported (discord:555, two turns later only in the
# database) and a jsonl-backup/ copy of the older workspace file of the
# heartbeat session, whose first turn a /new later removed from the
# database.
NANOBOT_DATABASE_WORKSPACE = "fedcba9876543210fedcba9876543210"
NANOBOT_DATABASE_REL = ".nanobot/sessions/%s/sessions.sqlite3" % NANOBOT_DATABASE_WORKSPACE
NANOBOT_DATABASE_KEY = "discord:555"
NANOBOT_DATABASE_FROZEN_REL = (
    ".nanobot/sessions/%s/ZGlzY29yZDo1NTU.jsonl" % NANOBOT_DATABASE_WORKSPACE
)
NANOBOT_DATABASE_BACKUP_REL = (
    ".nanobot/sessions/%s/jsonl-backup/heartbeat.jsonl" % NANOBOT_DATABASE_WORKSPACE
)
NANOBOT_DATABASE_SCHEMA = """
CREATE TABLE storage_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE sessions (
    key TEXT PRIMARY KEY, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
    metadata TEXT NOT NULL, provider_state TEXT, last_archived INTEGER NOT NULL,
    revision INTEGER NOT NULL, generation TEXT NOT NULL,
    title TEXT NOT NULL, preview TEXT NOT NULL, visible_updated_at TEXT
);
CREATE TABLE messages (
    session_key TEXT NOT NULL REFERENCES sessions(key) ON DELETE CASCADE,
    position INTEGER NOT NULL, record TEXT NOT NULL,
    PRIMARY KEY (session_key, position)
);
CREATE TABLE checkpoints (
    session_key TEXT PRIMARY KEY REFERENCES sessions(key) ON DELETE CASCADE,
    record TEXT NOT NULL
);
CREATE TABLE pending_inputs (
    session_key TEXT NOT NULL REFERENCES sessions(key) ON DELETE CASCADE,
    kind TEXT NOT NULL, position INTEGER NOT NULL, record TEXT NOT NULL,
    PRIMARY KEY (session_key, kind, position)
);
CREATE INDEX sessions_updated ON sessions(updated_at DESC);
"""


def nanobot_database_messages():
    """The discord:555 messages in `position` order; the first four are the frozen JSONL's."""
    return [
        {"role": "user", "content": "rotate the api key", "timestamp": "2026-10-03T11:00:01"},
        {
            "role": "assistant",
            "content": "",
            "tool_calls": [
                {
                    "id": "call_rot1",
                    "type": "function",
                    "function": {"name": "exec", "arguments": '{"command": "./rotate.sh"}'},
                }
            ],
            "timestamp": "2026-10-03T11:00:03",
        },
        {
            "role": "tool",
            "tool_call_id": "call_rot1",
            "name": "exec",
            "content": "rotated",
            "timestamp": "2026-10-03T11:00:04",
        },
        {"role": "assistant", "content": "The key is rotated.", "timestamp": "2026-10-03T11:00:05"},
        {"role": "user", "content": "thanks", "timestamp": "2026-10-03T11:00:30"},
        {"role": "assistant", "content": "You're welcome.", "timestamp": "2026-10-03T11:00:31"},
    ]


def nanobot_database_insert_message(path: Path, key: str, position: int, record: str) -> None:
    """Store `record` verbatim as `messages` row (`key`, `position`)."""
    connection = sqlite3.connect(str(path))
    connection.execute(
        "INSERT INTO messages (session_key, position, record) VALUES (?, ?, ?)",
        (key, position, record),
    )
    connection.commit()
    connection.close()


def build_nanobot_sqlite(home: Path) -> None:
    database = home / NANOBOT_DATABASE_REL
    database.parent.mkdir(parents=True, exist_ok=True)
    (database.parent / ".workspace").write_text("/srv/proj\n", encoding="utf-8")
    meta = {"last_channel": NANOBOT_DATABASE_KEY, "_nanobot_model_preset": "smart"}
    connection = sqlite3.connect(str(database))
    connection.execute("PRAGMA journal_mode = WAL")
    connection.executescript(NANOBOT_DATABASE_SCHEMA)
    connection.execute("PRAGMA user_version = 1")
    connection.execute("INSERT INTO storage_meta VALUES ('jsonl_import', 'complete')")
    sessions = [
        (NANOBOT_DATABASE_KEY, "2026-10-03T11:00:00", "2026-10-03T11:00:31", meta),
        ("heartbeat", "2026-09-28T06:00:00", "2026-10-03T06:00:05", {}),
    ]
    for key, created, updated, metadata in sessions:
        connection.execute(
            "INSERT INTO sessions VALUES (?, ?, ?, ?, NULL, 0, 3, 'g1', '', '', ?)",
            (key, created, updated, json.dumps(metadata), updated),
        )
    heartbeat = [
        {"role": "user", "content": "check the inbox", "timestamp": "2026-10-03T06:00:01"},
        {"role": "assistant", "content": "Inbox is empty.", "timestamp": "2026-10-03T06:00:05"},
    ]
    for key, records in (
        (NANOBOT_DATABASE_KEY, nanobot_database_messages()),
        ("heartbeat", heartbeat),
    ):
        for position, record in enumerate(records):
            connection.execute(
                "INSERT INTO messages VALUES (?, ?, ?)",
                (key, position, json.dumps(record, separators=(",", ":"))),
            )
    connection.commit()
    connection.close()
    frozen_meta = {
        "_type": "metadata",
        "key": NANOBOT_DATABASE_KEY,
        "created_at": "2026-10-03T11:00:00",
        "updated_at": "2026-10-03T11:00:05",
        "metadata": meta,
    }
    _jsonl(home / NANOBOT_DATABASE_FROZEN_REL, [frozen_meta, *nanobot_database_messages()[:4]])
    _jsonl(
        home / NANOBOT_DATABASE_BACKUP_REL,
        [
            {
                "_type": "metadata",
                "key": "heartbeat",
                "created_at": "2026-09-28T06:00:00",
                "updated_at": "2026-09-28T06:00:04",
                "metadata": {},
            },
            {"role": "user", "content": "old heartbeat prompt", "timestamp": "2026-09-28T06:00:01"},
            {"role": "assistant", "content": "Nothing to do.", "timestamp": "2026-09-28T06:00:04"},
        ],
    )
    (home / NANOBOT_DATABASE_BACKUP_REL).parent.joinpath("heartbeat.checkpoint.json").write_text(
        "{}", encoding="utf-8"
    )
