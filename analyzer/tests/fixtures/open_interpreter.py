"""Synthetic Open Interpreter state for the analyzer tests."""

from __future__ import annotations

import json
from pathlib import Path

from .codex_cli import codex_log_rows, write_codex_logs, write_codex_state, write_thread_history
from .common import _jsonl

OI_SESSION = "0199a1b2-0000-7000-8000-000000000001"
OI_IMPORTED = "0199a1b2-0000-7000-8000-000000000002"
OI_ROLLOUT = (
    ".openinterpreter/sessions/2026/10/01/rollout-2026-10-01T12-00-00-%s.jsonl" % OI_SESSION
)
OI_ARCHIVED = (
    ".openinterpreter/archived_sessions/2026/09/30/rollout-2026-09-30T08-00-00-%s.jsonl"
    % OI_IMPORTED
)


def open_interpreter_rollout_records(cwd="/srv/proj"):
    """The Open Interpreter records of its research document.

    research/open-interpreter.md, Parser plan, with a reasoning item and a task_complete event
    added.
    """
    return [
        {
            "timestamp": "2026-10-01T10:00:00.000Z",
            "type": "session_meta",
            "payload": {
                "session_id": OI_SESSION,
                "id": OI_SESSION,
                "timestamp": "2026-10-01T10:00:00.000Z",
                "cwd": cwd,
                "originator": "codex_cli_rs",
                "cli_version": "0.9.0",
                "source": "cli",
                "model_provider": "kimi-for-coding",
                "git": {"branch": "main"},
            },
        },
        {
            "timestamp": "2026-10-01T10:00:01.000Z",
            "type": "turn_context",
            "payload": {
                "turn_id": "t1",
                "cwd": cwd,
                "approval_policy": "on-request",
                "sandbox_policy": {"type": "workspace-write"},
                "model": "kimi-k3",
            },
        },
        {
            "timestamp": "2026-10-01T10:00:01.100Z",
            "type": "response_item",
            "payload": {
                "type": "message",
                "role": "user",
                "content": [{"type": "input_text", "text": "list the files"}],
            },
        },
        {
            "timestamp": "2026-10-01T10:00:02.000Z",
            "type": "response_item",
            "payload": {
                "type": "reasoning",
                "summary": [{"type": "summary_text", "text": "run ls"}],
                "encrypted_content": "gAAA",
            },
        },
        {
            "timestamp": "2026-10-01T10:00:03.000Z",
            "type": "response_item",
            "payload": {
                "type": "function_call",
                "name": "Bash",
                "arguments": '{"command":"ls"}',
                "call_id": "call_1",
            },
        },
        {
            "timestamp": "2026-10-01T10:00:04.000Z",
            "type": "response_item",
            "payload": {"type": "function_call_output", "call_id": "call_1", "output": "README.md"},
        },
        {
            "timestamp": "2026-10-01T10:00:05.000Z",
            "type": "response_item",
            "payload": {
                "type": "message",
                "role": "assistant",
                "content": [{"type": "output_text", "text": "One file: README.md"}],
            },
        },
        {
            "timestamp": "2026-10-01T10:00:05.000Z",
            "type": "event_msg",
            "payload": {"type": "task_complete", "turn_id": "t1", "duration_ms": 5000},
        },
        {
            # The next turn switches model and approval policy.
            "timestamp": "2026-10-01T10:00:06.000Z",
            "type": "turn_context",
            "payload": {
                "turn_id": "t2",
                "cwd": cwd,
                "approval_policy": "never",
                "sandbox_policy": {"type": "workspace-write"},
                "model": "kimi-k3-turbo",
            },
        },
    ]


def open_interpreter_imported_records(cwd="/srv/proj"):
    """A thread written by /import from a Claude Code transcript."""
    return [
        {
            "timestamp": "2026-09-30T08:00:00.000Z",
            "type": "session_meta",
            "payload": {
                "session_id": OI_IMPORTED,
                "id": OI_IMPORTED,
                "timestamp": "2026-09-30T08:00:00.000Z",
                "cwd": cwd,
                "originator": "codex_cli_rs",
                "cli_version": "0.9.0",
                "source": "cli",
                "model_provider": "openai",
            },
        },
        {
            "timestamp": "2026-09-30T08:00:00.000Z",
            "type": "response_item",
            "payload": {
                "type": "message",
                "role": "user",
                "content": [{"type": "input_text", "text": "imported prompt"}],
            },
        },
    ]


def open_interpreter_ledger():
    return {
        "records": [
            {
                "source_path": "/home/alice/.claude/projects/-home-alice-proj/5b1c.jsonl",
                "content_sha256": "ab12cd34",
                "imported_thread_id": OI_IMPORTED,
                "imported_at": 1790762400,
                "source_modified_at": 1790758800,
            },
        ]
    }


def build_open_interpreter(home: Path) -> None:
    interpreter = home / ".openinterpreter"
    _jsonl(home / OI_ROLLOUT, open_interpreter_rollout_records())
    _jsonl(home / OI_ARCHIVED, open_interpreter_imported_records())
    _jsonl(
        interpreter / "history.jsonl",
        [{"session_id": OI_SESSION, "ts": 1790848801, "text": "list the files"}],
    )
    _jsonl(
        interpreter / "session_index.jsonl",
        [{"id": OI_SESSION, "thread_name": "ls", "updated_at": "2026-10-01T10:00:05Z"}],
    )
    (interpreter / "external_agent_session_imports.json").write_text(
        json.dumps(open_interpreter_ledger()), encoding="utf-8"
    )
    (interpreter / "config.toml").write_text('harness = "claude-code"\n', encoding="utf-8")
    (interpreter / "auth.json").write_text('{"OPENAI_API_KEY": "sk-not-real"}', encoding="utf-8")
    build_open_interpreter_stores(home)


# The fork's SQLite set: OI_SESSION's thread row repeats its first
# prompt; OI_ORPHAN's rollout is gone and only the projection, in the
# fork's schema without item lifecycle times, holds its turn.
OI_ORPHAN = "0199a1b2-0000-7000-8000-000000000003"


def build_open_interpreter_stores(home: Path) -> None:
    interpreter = home / ".openinterpreter"
    sessions = "/home/alice/.openinterpreter/sessions"
    write_codex_state(
        interpreter / "state_5.sqlite",
        [
            (
                OI_SESSION,
                sessions + "/2026/10/01/rollout-2026-10-01T12-00-00-%s.jsonl" % OI_SESSION,
                1790848800,
                "list the files",
                "list the files",
                None,
                0,
                "user",
                "main",
                "kimi-k3-turbo",
            ),
            (
                OI_ORPHAN,
                sessions + "/2026/09/29/rollout-2026-09-29T07-00-00-%s.jsonl" % OI_ORPHAN,
                1790665200,
                "summarise the README",
                "summarise the README",
                None,
                0,
                "user",
                None,
                "kimi-k3",
            ),
        ],
    )
    write_thread_history(
        interpreter / "thread_history_1.sqlite",
        [(OI_ORPHAN, "t1", 1, 6, "completed", 1790665201, 1790665206, 5000)],
        [
            (
                OI_ORPHAN,
                "t1",
                3,
                1790665202000,
                None,
                None,
                {
                    "type": "userMessage",
                    "id": "u1",
                    "content": [{"type": "text", "text": "summarise the README"}],
                },
            ),
            (
                OI_ORPHAN,
                "t1",
                5,
                1790665204000,
                None,
                None,
                {"type": "agentMessage", "id": "a1", "text": "It describes the project."},
            ),
        ],
        lifecycle=False,
    )
    # The fork's tracing log with a sent `/feedback` report for OI_SESSION.
    write_codex_logs(interpreter / "logs_2.sqlite", codex_log_rows(OI_SESSION))
