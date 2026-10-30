"""Synthetic Goose state for the analyzer tests."""

from __future__ import annotations

import json
import os
from pathlib import Path

from .common import _jsonl, _wal_database

GOOSE_SESSION = "20260301_1"

# session_manager.rs at goose 591edd4, schema version 16 (research/goose.md, Record schema).
GOOSE_SCHEMA = """
CREATE TABLE schema_version (version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE sessions (
  id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '',
  user_set_name BOOLEAN DEFAULT FALSE, session_type TEXT NOT NULL DEFAULT 'user',
  working_dir TEXT NOT NULL, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, extension_data TEXT DEFAULT '{}',
  total_tokens INTEGER, input_tokens INTEGER, output_tokens INTEGER, cache_read_tokens INTEGER,
  cache_write_tokens INTEGER, accumulated_total_tokens INTEGER, accumulated_input_tokens INTEGER,
  accumulated_output_tokens INTEGER, accumulated_cache_read_tokens INTEGER,
  accumulated_cache_write_tokens INTEGER, accumulated_cost REAL, schedule_id TEXT, recipe_json TEXT,
  user_recipe_values_json TEXT, provider_name TEXT, model_config_json TEXT,
  goose_mode TEXT NOT NULL DEFAULT 'auto', archived_at TIMESTAMP, project_id TEXT, parent_session_id TEXT);
CREATE TABLE messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT, message_id TEXT, session_id TEXT NOT NULL REFERENCES sessions(id),
  role TEXT NOT NULL, content_json TEXT NOT NULL, created_timestamp INTEGER NOT NULL,
  timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP, tokens INTEGER, metadata_json TEXT);
CREATE TABLE usage_ledger (
  id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  created_timestamp INTEGER NOT NULL, model TEXT, input_tokens INTEGER, output_tokens INTEGER,
  total_tokens INTEGER, cache_read_tokens INTEGER, cache_write_tokens INTEGER, cost REAL,
  cost_source TEXT, is_compaction INTEGER DEFAULT 0);
"""


def goose_records(cwd="/srv/proj"):
    """The INSERT statements of research/goose.md, Parser plan.

    The INSERT statements of research/goose.md, Parser plan, with the working directory moved to the
    shared project path.
    """
    return [
        "INSERT INTO schema_version(version) VALUES (16)",
        "INSERT INTO sessions(id,name,session_type,working_dir,created_at,updated_at,extension_data,provider_name,model_config_json,goose_mode) "
        "VALUES ('20260301_1','Fix flaky test','user','%s','2026-03-01 09:00:00','2026-03-01 09:01:05','{}','anthropic',"
        """'{"model_name":"claude-sonnet-4-5","temperature":null,"max_tokens":null,"toolshim":false,"toolshim_model":null}','auto')"""
        % cwd,
        "INSERT INTO messages(message_id,session_id,role,content_json,created_timestamp,metadata_json) VALUES "
        """('msg_20260301_1_a1','20260301_1','user','[{"type":"text","text":"run the tests"}]',1772355600,'{"userVisible":true,"agentVisible":true}'),"""
        """('msg_20260301_1_a2','20260301_1','assistant','[{"type":"toolRequest","id":"call_1","toolCall":{"status":"success","value":{"name":"developer__shell","arguments":{"command":"pytest -q"}}}}]',1772355601,"""
        """ '{"userVisible":true,"agentVisible":true,"inference":{"provider":"anthropic","requestedModel":"claude-sonnet-4-5"}}'),"""
        """('msg_20260301_1_a3','20260301_1','user','[{"type":"toolResponse","id":"call_1","toolResult":{"status":"success","value":{"content":[{"type":"text","text":"3 passed"}]}}}]',1772355603,'{"userVisible":true,"agentVisible":true}'),"""
        """('msg_20260301_1_a4','20260301_1','assistant','[{"type":"thinking","thinking":"all green","signature":"sig"},{"type":"text","text":"All 3 tests pass."}]',1772355605, '{"userVisible":true,"agentVisible":true,"inference":{"provider":"openai","requestedModel":"gpt-4.1","resolvedModel":"gpt-4.1-2025-04-14"}}')""",
        "INSERT INTO usage_ledger(session_id,created_timestamp,model,input_tokens,output_tokens,total_tokens) "
        "VALUES ('20260301_1',1772355605,'claude-sonnet-4-5',120,30,150)",
    ]


# Session sync: a gateway session, created when a Telegram user
# pairs (crates/goose/src/gateway/handler.rs:304-326 at aaif-goose/goose
# bfcc6ee): name `<platform>/<display name or user id>`, working_dir
# `<config>/gateway/<platform>/<user_id>`. Kept out of build_home so the
# shared session's row order stays as the other tests expect.
GOOSE_GATEWAY_SESSION = "20261017_4"


def goose_gateway_records(display="alice"):
    return [
        "INSERT INTO sessions(id,name,session_type,working_dir,created_at,updated_at) VALUES "
        "('%s','telegram/%s','gateway','/home/alice/.config/goose/gateway/telegram/123456789',"
        "'2026-10-17 08:00:00','2026-10-17 08:05:00')" % (GOOSE_GATEWAY_SESSION, display),
        "INSERT INTO messages(message_id,session_id,role,content_json,created_timestamp,metadata_json)"
        " VALUES ('msg_gw_1','%s','user','[{\"type\":\"text\",\"text\":\"disk usage?\"}]',"
        "1792224060,'{\"userVisible\":true}')" % GOOSE_GATEWAY_SESSION,
    ]


def goose_legacy_records():
    return [
        {
            "description": "old chat",
            "working_dir": "/srv/old",
            "created_at": "2026-03-01T09:00:00Z",
            "updated_at": "2026-03-01T09:00:10Z",
            "extension_data": {},
            "message_count": 1,
        },
        {
            "id": "m1",
            "role": "user",
            "created": 1772355600,
            "content": [{"type": "text", "text": "hello"}],
        },
    ]


def goose_llm_request_records():
    return [
        {
            "model_config": {"model_name": "gpt-4.1"},
            "input": {"model": "gpt-4.1", "messages": [{"role": "user", "content": "hello"}]},
        },
        {
            "data": {
                "role": "assistant",
                "created": 1772355601,
                "content": [{"type": "text", "text": "hi"}],
            },
            "usage": {"input_tokens": 5, "output_tokens": 1},
        },
    ]


GOOSE_LEGACY_REL = ".local/share/goose/sessions/20260301_090000.jsonl"


def build_goose(home: Path) -> None:
    data = home / ".local/share/goose/sessions"
    _wal_database(data / "sessions.db", GOOSE_SCHEMA, goose_records())
    _jsonl(home / GOOSE_LEGACY_REL, goose_legacy_records())
    state = home / ".local/state/goose"
    _jsonl(state / "logs/llm_request.0.jsonl", goose_llm_request_records())
    (state / "history.txt").write_text(
        "#V2\nrun the tests\nline one\\nline two \\\\ done\n", encoding="utf-8"
    )
    (home / ".config/goose").mkdir(parents=True, exist_ok=True)
    (home / ".config/goose/config.yaml").write_text("GOOSE_PROVIDER: anthropic\n", encoding="utf-8")
    (home / ".config/goose/secrets.yaml").write_text(
        "ANTHROPIC_API_KEY: sk-test\n", encoding="utf-8"
    )


# Bad stored JSON in sessions.db. Column shapes from
# session_manager.rs at block/goose 591edd4 (research/goose.md, Record schema):
# content_json is a JSON array, metadata_json and model_config_json JSON
# objects. Kept out of build_home, as the gateway session is, so the shared
# session's rows stay as the other tests expect.
GOOSE_BAD_SESSION = "20261019_5"
GOOSE_BAD_MODEL_CONFIG = '{"model_name":"gpt-4.1",'
GOOSE_BAD_METADATA = '{"userVisible":tru'
GOOSE_BAD_CONTENT = '[{"type":"text","te'


def goose_bad_json_records():
    """A session whose model_config_json is cut off, then four messages.

    A good one, one with a cut-off metadata_json, one with a cut-off content_json, and a good one
    after it.
    """

    def quote(text: str) -> str:
        return text.replace("'", "''")

    message = (
        "INSERT INTO messages(id,message_id,session_id,role,content_json,created_timestamp,"
        "metadata_json) VALUES (%d,'%s','%s','%s','%s',%d,'%s')"
    )
    good = '{"userVisible":true}'
    return [
        "INSERT INTO sessions(id,name,session_type,working_dir,created_at,updated_at,"
        "provider_name,model_config_json) VALUES ('%s','bad json','user','/srv/bad',"
        "'2026-10-19 08:00:00','2026-10-19 08:05:00','openai','%s')"
        % (GOOSE_BAD_SESSION, quote(GOOSE_BAD_MODEL_CONFIG)),
        message
        % (
            101,
            "msg_bad_1",
            GOOSE_BAD_SESSION,
            "user",
            '[{"type":"text","text":"before the bad rows"}]',
            1792396800,
            good,
        ),
        message
        % (
            102,
            "msg_bad_2",
            GOOSE_BAD_SESSION,
            "assistant",
            '[{"type":"text","text":"kept despite bad metadata"}]',
            1792396801,
            quote(GOOSE_BAD_METADATA),
        ),
        message
        % (
            103,
            "msg_bad_3",
            GOOSE_BAD_SESSION,
            "assistant",
            quote(GOOSE_BAD_CONTENT),
            1792396802,
            good,
        ),
        message
        % (
            104,
            "msg_bad_4",
            GOOSE_BAD_SESSION,
            "user",
            '[{"type":"text","text":"after the bad rows"}]',
            1792396803,
            good,
        ),
    ]


# Tool confirmations, shapes from aaif-goose/goose fb9e3c8
# (research/goose.md, Tool confirmations and permission mode): the question
# is an `assistant` message holding an `actionRequired` block of
# `actionType` `toolConfirmation`, the answer a `user` message holding one
# of `actionType` `toolConfirmationResponse`, and the approval step patches
# `goose.executable` into the `toolRequest`'s `_meta`. call_p1 was asked
# about and allowed once; call_p2 was refused by the permission check (a
# `never_allow` tool) with no question; the last message is an MCP
# elicitation, which keeps the generic row. Kept out of build_home, as the
# gateway session is.
GOOSE_PERMISSION_SESSION = "20261021_6"


def goose_permission_records():
    def message_row(
        number, role, content, timestamp, meta='{"userVisible":true,"agentVisible":true}'
    ):
        return "(%d,'msg_perm_%d','%s','%s','%s',%d,'%s')" % (
            number,
            number,
            GOOSE_PERMISSION_SESSION,
            role,
            json.dumps(content, separators=(",", ":")).replace("'", "''"),
            timestamp,
            meta,
        )

    def call(cid, name, arguments, executable):
        return {
            "type": "toolRequest",
            "id": cid,
            "toolCall": {"status": "success", "value": {"name": name, "arguments": arguments}},
            "_meta": {"goose.executable": executable},
        }

    def response(cid, text):
        return {
            "type": "toolResponse",
            "id": cid,
            "toolResult": {
                "status": "success",
                "value": {"content": [{"type": "text", "text": text}]},
            },
        }

    def action(data):
        return {"type": "actionRequired", "data": data}

    rows = [
        message_row(
            301,
            "assistant",
            [call("call_p1", "developer__shell", {"command": "rm -rf build"}, True)],
            1792569600,
        ),
        message_row(
            302,
            "assistant",
            [
                action(
                    {
                        "actionType": "toolConfirmation",
                        "id": "call_p1",
                        "toolName": "developer__shell",
                        "arguments": {"command": "rm -rf build"},
                        "prompt": None,
                    }
                )
            ],
            1792569600,
            '{"userVisible":true,"agentVisible":false}',
        ),
        message_row(
            303,
            "user",
            [
                action(
                    {
                        "actionType": "toolConfirmationResponse",
                        "id": "call_p1",
                        "permission": "allow_once",
                    }
                )
            ],
            1792569605,
            '{"userVisible":false,"agentVisible":false}',
        ),
        message_row(304, "user", [response("call_p1", "removed")], 1792569606),
        message_row(
            305,
            "assistant",
            [call("call_p2", "developer__text_editor", {"path": "/etc/hosts"}, False)],
            1792569610,
        ),
        message_row(
            306,
            "user",
            [
                response(
                    "call_p2",
                    "The user has declined to run this tool. DO NOT attempt to call this tool again.",
                )
            ],
            1792569610,
        ),
        message_row(
            307,
            "assistant",
            [
                action(
                    {
                        "actionType": "elicitation",
                        "id": "el_1",
                        "message": "Pick one",
                        "requestedSchema": {},
                    }
                )
            ],
            1792569611,
        ),
    ]
    return [
        "INSERT INTO sessions(id,name,session_type,working_dir,created_at,updated_at,goose_mode)"
        " VALUES ('%s','permissions','user','/srv/proj','2026-10-21 08:00:00',"
        "'2026-10-21 08:01:00','smart_approve')" % GOOSE_PERMISSION_SESSION,
        "INSERT INTO messages(id,message_id,session_id,role,content_json,created_timestamp,"
        "metadata_json) VALUES " + ",".join(rows),
    ]


# Standing permission and scheduler files, shapes from
# aaif-goose/goose fb9e3c8: permission.yaml as serde_yaml writes the map of
# `PermissionConfig` (config/permission.rs:13-44, 260-262), roaming_trust.json
# the `TrustBook` (goose-roaming/src/trust.rs:19-26) and schedule.json a list
# of `ScheduledJob` (scheduler/common.rs:167-187). The peer keys and ids are
# invented. Kept out of build_home so the shared timeline stays as the CLI
# tests expect.
GOOSE_STANDING_MTIME = 1792656000  # 22 October 08:00:00 UTC
GOOSE_PERMISSION_YAML = """user:
  always_allow:
  - developer__shell
  ask_before: []
  never_allow:
  - developer__text_editor
smart_approve:
  always_allow: []
  ask_before:
  - 'computercontroller__automation_script'
  never_allow: []
"""
GOOSE_ROAMING_TRUST = {"allowed": ["peerkeyaaaa"], "revoked_keys": ["peerkeybbbb"]}
GOOSE_SCHEDULE = [
    {
        "id": "nightly-report",
        "source": "/home/alice/.local/share/goose/scheduled_recipes/nightly-report.yaml",
        "cron": "0 0 2 * * *",
        "last_run": "2026-10-21T02:00:00.123456Z",
        "currently_running": False,
        "paused": False,
        "current_session_id": None,
        "process_start_time": None,
        "parameters": [],
        "recipe_base_dir": "/srv/proj/recipes",
    },
    {
        "id": "weekly",
        "source": "/home/alice/.local/share/goose/scheduled_recipes/weekly.yaml",
        "cron": "0 0 9 * * 1",
        "last_run": None,
        "paused": True,
        "parameters": [["env", "prod"]],
    },
]


def build_goose_standing(home: Path) -> None:
    """Write the Goose standing files beside two noise files.

    The standing files beside two noise files the parser must not want (`permission.yaml.lock`,
    `roaming_peers.json`).
    """
    config = home / ".config/goose"
    data = home / ".local/share/goose"
    config.mkdir(parents=True, exist_ok=True)
    data.mkdir(parents=True, exist_ok=True)
    files = {
        config / "permission.yaml": GOOSE_PERMISSION_YAML,
        config / "roaming_trust.json": json.dumps(GOOSE_ROAMING_TRUST, indent=2),
        data / "schedule.json": json.dumps(GOOSE_SCHEDULE, indent=2),
        config / "permission.yaml.lock": "",
        config / "roaming_peers.json": "{}",
    }
    for path, text in files.items():
        path.write_text(text, encoding="utf-8")
        os.utime(path, (GOOSE_STANDING_MTIME, GOOSE_STANDING_MTIME))


# A streamed request log: the response lines are chunks of one
# message (`id` msg_s1) as the providers log them, two text deltas and a
# tool request, which `Conversation::push` assembles into the message
# sessions.db stores (goose-provider-types/src/conversation.rs:75-119 at
# aaif-goose/goose fb9e3c8). The assembled text and the tool call repeat
# the shared session's database rows.
def goose_streamed_llm_records():
    def chunk(content, created):
        return {
            "data": {"id": "msg_s1", "role": "assistant", "created": created, "content": content},
            "usage": None,
        }

    return [
        {
            "model_config": {"model_name": "claude-sonnet-4-5"},
            "input": {"messages": [{"role": "user", "content": "run the tests"}]},
        },
        chunk([{"type": "text", "text": "All 3 "}], 1772355605),
        chunk([{"type": "text", "text": "tests pass."}], 1772355606),
        chunk(
            [
                {
                    "type": "toolRequest",
                    "id": "call_1",
                    "toolCall": {
                        "status": "success",
                        "value": {
                            "name": "developer__shell",
                            "arguments": {"command": "pytest -q"},
                        },
                    },
                }
            ],
            1772355607,
        ),
        {
            "data": {"id": "msg_s1", "role": "assistant", "created": 1772355607, "content": []},
            "usage": {"input_tokens": 9},
        },
    ]
