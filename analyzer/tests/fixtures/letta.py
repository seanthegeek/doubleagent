"""Synthetic Letta state for the analyzer tests."""

from __future__ import annotations

import os
from pathlib import Path

from .common import _json, _jsonl

LETTA_LOCAL_CONV = "local-conv-1"
LETTA_LOCAL_DIR = "Y29udmVyc2F0aW9uOmxvY2FsLWNvbnYtMQ"  # base64url("conversation:local-conv-1")
LETTA_AGENT = "agent-1a2b"
LETTA_CONV = "conv-9f"


def letta_transcript_records():
    return [
        {
            "kind": "user",
            "text": "list the repo",
            "captured_at": "2026-10-01T12:00:05.120Z",
            "source_line_id": "ui-1",
        },
        {"kind": "reasoning", "text": "use ls", "captured_at": "2026-10-01T12:00:05.120Z"},
        {
            "kind": "tool_call",
            "name": "Bash",
            "argsText": '{"command":"ls"}',
            "resultText": "README.md",
            "resultOk": True,
            "captured_at": "2026-10-01T12:00:05.120Z",
            "source_line_id": "ui-2",
        },
        {
            "kind": "assistant",
            "text": "One file: README.md",
            "captured_at": "2026-10-01T12:00:05.120Z",
            "source_line_id": "ui-3",
            "source_message_id": "message-77",
        },
    ]


def letta_twin_transcript_records():
    """The local-backend conversation's transcript.

    The turn its messages.jsonl also holds, captured at the turn's end, then a turn messages.jsonl
    no longer has.
    """
    return [
        *letta_transcript_records(),
        {
            "kind": "user",
            "text": "now delete the build directory",
            "captured_at": "2026-10-01T12:00:40.500Z",
            "source_line_id": "ui-4",
        },
        {
            "kind": "assistant",
            "text": "Deleted build/",
            "captured_at": "2026-10-01T12:00:40.500Z",
            "source_line_id": "ui-5",
        },
    ]


def letta_local_records(cwd="/srv/proj"):
    asst = {
        "id": "letta-msg-2",
        "role": "assistant",
        "content": [
            {"type": "thinking", "thinking": "use ls"},
            {"type": "toolCall", "id": "call_1", "name": "Bash", "arguments": {"command": "ls"}},
        ],
        "api": "anthropic-messages",
        "provider": "anthropic",
        "model": "claude-sonnet-4-5",
        "usage": {
            "input": 10,
            "output": 5,
            "cacheRead": 0,
            "cacheWrite": 0,
            "totalTokens": 15,
            "cost": {"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0, "total": 0},
        },
        "stopReason": "toolUse",
        "timestamp": 1790856002000,
        "metadata": {"agent_id": LETTA_AGENT, "conversation_id": LETTA_LOCAL_CONV},
    }
    partial = dict(asst, content=[{"type": "thinking", "thinking": "use"}], stopReason="pending")
    return [
        {
            "type": "session",
            "version": 3,
            "id": LETTA_LOCAL_CONV,
            "timestamp": "2026-10-01T12:00:00.000Z",
            "cwd": cwd,
        },
        {
            "type": "message",
            "id": "a1b2c3d4",
            "parentId": None,
            "timestamp": "2026-10-01T12:00:01.000Z",
            "message": {
                "id": "letta-msg-1",
                "role": "user",
                "content": "list the repo",
                "timestamp": 1790856001000,
            },
        },
        {
            "type": "message",
            "id": "b2c3d4e4",
            "parentId": "a1b2c3d4",
            "timestamp": "2026-10-01T12:00:02.000Z",
            "message": partial,
        },
        {
            "type": "message",
            "id": "b2c3d4e5",
            "parentId": "a1b2c3d4",
            "timestamp": "2026-10-01T12:00:02.000Z",
            "message": asst,
        },
        {
            "type": "message",
            "id": "c3d4e5f6",
            "parentId": "b2c3d4e5",
            "timestamp": "2026-10-01T12:00:03.000Z",
            "message": {
                "id": "letta-msg-3",
                "role": "toolResult",
                "toolCallId": "call_1",
                "toolName": "Bash",
                "content": [{"type": "text", "text": "README.md"}],
                "isError": False,
                "timestamp": 1790856003000,
            },
        },
        {
            "type": "message",
            "id": "d4e5f6a7",
            "parentId": "c3d4e5f6",
            "timestamp": "2026-10-01T12:00:04.000Z",
            "message": {
                "id": "letta-msg-4",
                "role": "assistant",
                "content": [{"type": "text", "text": "One file: README.md"}],
                "provider": "anthropic",
                "model": "claude-haiku-4-5",
                "stopReason": "stop",
                "timestamp": 1790856004000,
            },
        },
    ]


def letta_sessions_records(project="/srv/proj"):
    usage = {
        "prompt_tokens": 0,
        "completion_tokens": 0,
        "total_tokens": 0,
        "cached_input_tokens": 0,
        "cache_write_tokens": 0,
        "reasoning_tokens": 0,
        "steps": 0,
    }
    # Only recordSessionEnd writes the file: one line per CLI
    # session that ended with /exit or /logout, at the time of exit.
    end = {
        "agent_id": LETTA_AGENT,
        "session_id": "s-1",
        "timestamp": 1790856060000,
        "project": project,
        "model": "claude-sonnet-4-5",
        "provider": "anthropic",
        "usage": usage,
        "duration": {"api_ms": 900, "wall_ms": 60000},
        "cost": {"type": "hosted"},
        "message_count": 4,
        "tool_call_count": 1,
        "exit_reason": "exit_command",
    }
    older = dict(
        end, session_id="s-0", timestamp=1790766000000, project="/srv/old", exit_reason="logout"
    )
    return [older, end]


def build_letta(home: Path) -> None:
    letta = home / ".letta"
    _jsonl(
        letta / "transcripts" / LETTA_AGENT / LETTA_CONV / "transcript.jsonl",
        letta_transcript_records(),
    )
    _jsonl(
        letta / "transcripts" / LETTA_AGENT / LETTA_LOCAL_CONV / "transcript.jsonl",
        letta_twin_transcript_records(),
    )
    _json(
        letta / "transcripts" / LETTA_AGENT / LETTA_CONV / "state.json",
        {"schema_version": "v3_assistant_steps"},
    )
    conv = letta / "lc-local-backend/conversations" / LETTA_LOCAL_DIR
    _jsonl(conv / "messages.jsonl", letta_local_records())
    _json(conv / "manifest.json", {"message_format": "pi-session-entry-jsonl", "schema_version": 2})
    _jsonl(letta / "sessions.jsonl", letta_sessions_records())
    _json(letta / "settings.json", {"env": {"LETTA_API_KEY": "sk-not-real"}})
    _json(letta / "transcripts" / LETTA_AGENT / LETTA_LOCAL_CONV / "payload-auto-a1b2c3.json", [])
    build_letta_sync(home)
    build_letta_stores(home)


# The stores of the Parser plan of research/letta.md at letta-code 2a46615d
# (permissions, channel routing, scheduled tasks and their runs, an
# interrupted listener turn), all on the server conversation LETTA_CONV so
# they add no session.
LETTA_LEGACY_SETTINGS_MTIME = 1790856060  # 1 October 12:01:00 UTC
LETTA_LISTENER_MTIME = 1790856240  # 1 October 12:04:00 UTC
LETTA_TASK = "a1b2c3d4"
LETTA_LISTENER_HASH = "0123456789abcdef01234567"
LETTA_LISTENER_FILE = "%s_%s.json" % (LETTA_AGENT, LETTA_CONV)


def letta_cron_task():
    return {
        "id": LETTA_TASK,
        "agent_id": LETTA_AGENT,
        "conversation_id": LETTA_CONV,
        "name": "nightly tests",
        "description": "run the suite",
        "cron": "0 2 * * *",
        "timezone": "UTC",
        "recurring": True,
        "prompt": "run the test suite and report failures",
        "status": "active",
        "created_at": "2026-10-01T12:10:00.000Z",
        "expires_at": None,
        "last_fired_at": "2026-10-02T02:00:00.000Z",
        "fire_count": 1,
        "cancel_reason": None,
        "jitter_offset_ms": 0,
        "last_run_at": "2026-10-02T02:00:00.000Z",
        "last_run_outcome": "queued",
        "last_run_reason": "scheduled_time_matched",
        "last_run_error": None,
        "last_missed_at": None,
        "missed_count": 0,
        "failed_count": 0,
        "scheduled_for": None,
        "fired_at": None,
        "missed_at": None,
    }


def letta_run_records():
    base = {"jobId": LETTA_TASK, "action": "finished", "agentId": LETTA_AGENT}
    return [
        dict(
            base,
            ts=1790906400000,  # 2 October 02:00:00 UTC
            status="ok",
            outcome="queued",
            reason="scheduled_time_matched",
            summary="nightly tests",
            conversationId=LETTA_CONV,
            runId="run-1",
            firedAt="2026-10-02T02:00:00.000Z",
        ),
        dict(
            base,
            ts=1790992800000,  # 3 October 02:00:00 UTC
            status="skipped",
            outcome="missed",
            reason="scheduler_inactive",
            summary="missed",
            conversationId=LETTA_CONV,
            missedCount=1,
        ),
    ]


def letta_interrupted_record():
    return {
        "agentId": LETTA_AGENT,
        "conversationId": LETTA_CONV,
        "runId": "run-2",
        "toolCallIds": ["call_7", "call_8"],
        "unstartedToolCallIds": ["call_8"],
        "results": [
            {
                "type": "tool",
                "status": "error",
                "tool_call_id": "call_7",
                "tool_return": "permission denied",
                "stdout": [],
                "stderr": [],
            },
            {
                "type": "approval",
                "approve": False,
                "tool_call_id": "call_8",
                "reason": "not on main",
            },
        ],
        "requestOtid": "otid-1",
        "workingDirectory": "/srv/proj",
        "actingUserId": "user-1",
    }


def build_letta_stores(home: Path) -> None:
    letta = home / ".letta"
    legacy = home / ".config/letta/settings.json"
    _json(
        legacy,
        {
            "permissions": {
                "mode": "acceptEdits",
                "allow": ["Bash(npm test)"],
                "deny": ["Bash(rm -rf:*)"],
                "additionalDirectories": ["/srv/shared"],
            },
            # Not read from the legacy file: Letta reads only permissions.
            "sessionsByServer": {"api.letta.com": {"agentId": "a", "conversationId": "c"}},
        },
    )
    os.utime(legacy, (LETTA_LEGACY_SETTINGS_MTIME, LETTA_LEGACY_SETTINGS_MTIME))
    channel = letta / "channels" / "telegram"
    _json(
        channel / "routing.json",
        {
            "routes": [
                {
                    "accountId": "bot-1",
                    "chatId": "123456789",
                    "chatType": "direct",
                    "threadId": None,
                    "agentId": LETTA_AGENT,
                    "conversationId": LETTA_CONV,
                    "enabled": True,
                    "outboundEnabled": True,
                    "createdAt": "2026-10-01T12:20:00.000Z",
                    "updatedAt": "2026-10-01T12:21:00.000Z",
                }
            ]
        },
    )
    # A credential file the parser must not want.
    _json(channel / "accounts.json", {"accounts": [{"id": "bot-1", "token": "not-real"}]})
    _json(
        letta / "crons.json",
        {
            "version": 1,
            "scheduler_owner": None,
            "scheduler_owners": {},
            "tasks": [letta_cron_task()],
        },
    )
    _jsonl(letta / "runs" / (LETTA_TASK + ".jsonl"), letta_run_records())
    state = letta / "listener-state" / LETTA_LISTENER_HASH
    _json(state / LETTA_LISTENER_FILE, letta_interrupted_record())
    os.utime(state / LETTA_LISTENER_FILE, (LETTA_LISTENER_MTIME, LETTA_LISTENER_MTIME))
    # A recovery sidecar beside the record, not wanted.
    _json(state / (LETTA_LISTENER_FILE + ".recovery-" + "ab" * 12), {})


# Session sync: the server-side session markers of
# research/letta.md, Transcript stores. settings.json is rewritten with
# `sessionsByServer` and `agents[]` beside the credential the parser must
# never print, and a local-backend agent's transcript, which is not
# server state. A project's settings.local.json is built by its test.
LETTA_SETTINGS_MTIME = 1790856120  # 1 October 12:02:00 UTC
LETTA_LOCAL_SETTINGS_MTIME = 1790856180  # 1 October 12:03:00 UTC
LETTA_LOCAL_AGENT = "agent-local-77"
LETTA_LOCAL_AGENT_CONV = "local-conv-2"


def build_letta_sync(home: Path) -> None:
    letta = home / ".letta"
    _json(
        letta / "settings.json",
        {
            "env": {"LETTA_API_KEY": "sk-not-real"},
            "agents": [
                {"agentId": LETTA_AGENT},
                {"agentId": "agent-selfhost", "baseUrl": "http://localhost:8283"},
            ],
            "sessionsByServer": {
                "api.letta.com": {"agentId": LETTA_AGENT, "conversationId": LETTA_CONV},
                "local:/home/alice/.letta/lc-local-backend": {
                    "agentId": LETTA_LOCAL_AGENT,
                    "conversationId": LETTA_LOCAL_AGENT_CONV,
                },
            },
        },
    )
    os.utime(letta / "settings.json", (LETTA_SETTINGS_MTIME, LETTA_SETTINGS_MTIME))
    _jsonl(
        letta / "transcripts" / LETTA_LOCAL_AGENT / LETTA_LOCAL_AGENT_CONV / "transcript.jsonl",
        [
            {"kind": "user", "text": "hello", "captured_at": "2026-10-01T12:05:00.000Z"},
            {"kind": "assistant", "text": "hi", "captured_at": "2026-10-01T12:05:00.000Z"},
        ],
    )


def letta_project_settings():
    """A project's `.letta/settings.local.json`.

    A project's `.letta/settings.local.json`, which a test places in a discovered project directory.
    """
    return {
        "permissions": {"allow": ["Bash(ls)"]},
        "sessionsByServer": {
            "localhost:8283": {"agentId": "agent-selfhost", "conversationId": "conv-sh1"}
        },
    }
