"""Synthetic little-coder state for the analyzer tests."""

from __future__ import annotations

import os
from pathlib import Path

from .common import _json

LC_SESSION = "0b9e2f4c-1111-4000-8000-000000000001"
LC_FILE = "2026-10-01T11-00-00-000Z_%s.jsonl" % LC_SESSION


def little_coder_session_records(cwd="/srv/proj"):
    return [
        {
            "type": "session",
            "version": 3,
            "id": LC_SESSION,
            "timestamp": "2026-10-01T11:00:00.000Z",
            "cwd": cwd,
        },
        {
            "type": "custom_message",
            "id": "a1",
            "parentId": None,
            "timestamp": "2026-10-01T11:00:01.000Z",
            "customType": "lc-skills",
            "content": "## Skill: edit\nUse the edit tool.",
            "display": False,
        },
        {
            "type": "message",
            "id": "a2",
            "parentId": "a1",
            "timestamp": "2026-10-01T11:00:02.000Z",
            "message": {
                "role": "user",
                "content": "fix the failing test",
                "timestamp": 1790852402000,
            },
        },
        {
            "type": "message",
            "id": "a3",
            "parentId": "a2",
            "timestamp": "2026-10-01T11:00:05.000Z",
            "message": {
                "role": "assistant",
                "content": [
                    {"type": "thinking", "thinking": "run tests first"},
                    {
                        "type": "toolCall",
                        "id": "call_1",
                        "name": "bash",
                        "arguments": {"command": "npm test"},
                    },
                ],
                "api": "openai-completions",
                "provider": "llamacpp",
                "model": "qwen3.6-35b-a3b",
                "usage": {"input": 900, "output": 40, "cacheRead": 0, "cacheWrite": 0},
                "stopReason": "toolUse",
                "timestamp": 1790852405000,
            },
        },
        {
            "type": "message",
            "id": "a4",
            "parentId": "a3",
            "timestamp": "2026-10-01T11:00:09.000Z",
            "message": {
                "role": "toolResult",
                "toolCallId": "call_1",
                "toolName": "bash",
                "content": [{"type": "text", "text": "1 failing"}],
                "isError": False,
                "timestamp": 1790852409000,
            },
        },
        {
            "type": "message",
            "id": "a5",
            "parentId": "a4",
            "timestamp": "2026-10-01T11:00:12.000Z",
            "message": {
                "role": "assistant",
                "content": [{"type": "text", "text": "One test fails."}],
                "api": "openai-completions",
                "provider": "llamacpp",
                "model": "qwen3.6-27b",
                "stopReason": "stop",
                "timestamp": 1790852412000,
            },
        },
        # The permission gate refuses an edit in manual mode.
        {
            "type": "message",
            "id": "a6",
            "parentId": "a5",
            "timestamp": "2026-10-01T11:00:14.000Z",
            "message": {
                "role": "assistant",
                "content": [
                    {
                        "type": "toolCall",
                        "id": "call_2",
                        "name": "edit",
                        "arguments": {"path": "src/a.ts", "edits": []},
                    }
                ],
                "api": "openai-completions",
                "provider": "llamacpp",
                "model": "qwen3.6-27b",
                "stopReason": "toolUse",
                "timestamp": 1790852414000,
            },
        },
        {
            "type": "message",
            "id": "a7",
            "parentId": "a6",
            "timestamp": "2026-10-01T11:00:15.000Z",
            "message": {
                "role": "toolResult",
                "toolCallId": "call_2",
                "toolName": "edit",
                "content": [{"type": "text", "text": "edit cancelled by user"}],
                "isError": True,
                "timestamp": 1790852415000,
            },
        },
    ]


def build_little_coder(home: Path) -> None:
    _json(
        home / ".pi/agent/little-coder-prompt-history.json",
        ["fix the failing test", "now run lint"],
    )
    checkpoints = home / ".little-coder/checkpoints" / LC_FILE
    checkpoints.mkdir(parents=True, exist_ok=True)
    (checkpoints / "_srv_proj_src_a.ts").write_text("old contents\n", encoding="utf-8")
    (checkpoints / "_srv_proj_src_new.ts.absent").write_text("", encoding="utf-8")
    os.utime(checkpoints / "_srv_proj_src_a.ts", (1790852406, 1790852406))
    os.utime(checkpoints / "_srv_proj_src_new.ts.absent", (1790852407, 1790852407))
    (home / ".config/little-coder").mkdir(parents=True, exist_ok=True)
    (home / ".config/little-coder/settings.json").write_text("{}", encoding="utf-8")
    build_little_coder_legacy(home)


# The retired v0.0.x Python layout under ~/.little-coder (research/little-coder.md,
# The v0.0.x layout).
LC_LEGACY = ".little-coder/"
LC_LEGACY_SESSION = "a1b2c3d4"
LC_LEGACY_OLD_SESSION = "e1f2a3b4"
LC_LEGACY_DAILY = LC_LEGACY + "sessions/daily/2026-09-20/session_101500_%s.json" % LC_LEGACY_SESSION
LC_LEGACY_LATEST = LC_LEGACY + "sessions/mr_sessions/session_latest.json"
LC_LEGACY_HISTORY = LC_LEGACY + "sessions/history.json"
LC_LEGACY_INPUT = LC_LEGACY + "input_history.txt"
LC_LEGACY_CONFIG = LC_LEGACY + "config.json"
LC_LEGACY_CKPT = "c0ffee12"
LC_LEGACY_SNAPSHOTS = LC_LEGACY + "checkpoints/%s/snapshots.json" % LC_LEGACY_CKPT
LC_LEGACY_GIST = "aa11bb22cc33dd44ee55ff6677889900"
LC_LEGACY_CONFIG_MTIME = 1790503200  # 27 September 10:00:00 UTC


def little_coder_legacy_session(session_id=LC_LEGACY_SESSION, saved_at="2026-09-20 10:15:00"):
    return {
        "session_id": session_id,
        "saved_at": saved_at,
        "messages": [
            {"role": "user", "content": "list the files"},
            {
                "role": "assistant",
                "content": "",
                "tool_calls": [{"id": "toolu_l1", "name": "Bash", "input": {"command": "ls"}}],
            },
            {"role": "tool", "tool_call_id": "toolu_l1", "name": "Bash", "content": "README.md"},
            {
                "role": "assistant",
                "content": "One file.",
                "tool_calls": [
                    {
                        "id": "toolu_l2",
                        "name": "Write",
                        "input": {"file_path": "/srv/proj/b.py", "content": "x"},
                    }
                ],
            },
            {
                "role": "tool",
                "tool_call_id": "toolu_l2",
                "name": "Write",
                "content": "Denied: user rejected this operation",
            },
        ],
        "turn_count": 1,
        "total_input_tokens": 1200,
        "total_output_tokens": 60,
    }


def build_little_coder_legacy(home: Path) -> None:
    session = little_coder_legacy_session()
    old = little_coder_legacy_session(LC_LEGACY_OLD_SESSION, "2026-09-19 08:00:00")
    old["messages"] = [{"role": "user", "content": "an old prompt"}]
    old["turn_count"] = 1
    _json(home / LC_LEGACY_DAILY, session)
    _json(home / LC_LEGACY_LATEST, session)
    _json(home / LC_LEGACY_HISTORY, {"total_turns": 2, "sessions": [old, session]})
    (home / LC_LEGACY_INPUT).write_text("list the files\nan older prompt\n", encoding="utf-8")
    _json(
        home / LC_LEGACY_CONFIG,
        {
            "model": "ollama/qwen3.5",
            "permission_mode": "manual",
            "gist_token": "ghp_not_a_real_token",
            "cloudsave_last_gist_id": LC_LEGACY_GIST,
            "telegram_token": "123:not-a-real-token",
            "telegram_chat_id": 424242,
        },
    )
    os.utime(home / LC_LEGACY_CONFIG, (LC_LEGACY_CONFIG_MTIME, LC_LEGACY_CONFIG_MTIME))
    _json(
        home / LC_LEGACY_SNAPSHOTS,
        [
            {
                "id": 1,
                "session_id": LC_LEGACY_CKPT,
                "created_at": "2026-09-20T10:14:00.000000",
                "turn_count": 0,
                "message_index": 0,
                "user_prompt_preview": "(initial state)",
                "token_snapshot": {"input": 0, "output": 0},
                "file_backups": {},
            },
            {
                "id": 2,
                "session_id": LC_LEGACY_CKPT,
                "created_at": "2026-09-20T10:14:30.500000",
                "turn_count": 1,
                "message_index": 5,
                "user_prompt_preview": "list the files",
                "token_snapshot": {"input": 1200, "output": 60},
                "file_backups": {
                    "/srv/proj/a.py": {
                        "backup_filename": "0123456789abcdef@v1",
                        "version": 1,
                        "backup_time": "2026-09-20T10:14:30.500000",
                    },
                    "/srv/proj/b.py": {
                        "backup_filename": None,
                        "version": 0,
                        "backup_time": "2026-09-20T10:14:30.500000",
                    },
                },
            },
        ],
    )
    backup = home / LC_LEGACY / "checkpoints" / LC_LEGACY_CKPT / "backups" / "0123456789abcdef@v1"
    backup.parent.mkdir(parents=True, exist_ok=True)
    backup.write_text("print(1)\n", encoding="utf-8")
    _json(home / LC_LEGACY / "mcp.json", {"mcpServers": {}})
