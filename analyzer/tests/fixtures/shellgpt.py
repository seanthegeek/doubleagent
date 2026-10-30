"""Synthetic ShellGPT state for the analyzer tests."""

from __future__ import annotations

import json
from pathlib import Path

from .common import _json_file

SHELLGPT_CHAT = "deploy-check"
SHELLGPT_LEGACY_CHAT = "old-functions"
SHELLGPT_CACHE_COPY = "900150983cd24fb0d6963f7d28e17f72"
# `.sgptrc` as ShellGPT 1.5.1 writes it (config.py DEFAULT_CONFIG order,
# shortened), with the API key the parser must never put in a row.
SHELLGPT_SGPTRC = (
    "CHAT_CACHE_PATH=/tmp/chat_cache\n"
    "CACHE_PATH=/tmp/cache\n"
    "DEFAULT_MODEL=gpt-5.4-mini\n"
    "DEFAULT_EXECUTE_SHELL_CMD=true\n"
    "OPENAI_USE_FUNCTIONS=true\n"
    "API_BASE_URL=default\n"
    "SHELL_INTERACTION=false\n"
    "OPENAI_API_KEY=sk-not-real\n"
)


def shellgpt_messages():
    return [
        {
            "role": "system",
            "content": "You are ShellGPT\nYou are programming and system administration assistant.",
        },
        {"role": "user", "content": "what is listening on port 8080"},
        {
            "role": "assistant",
            "content": None,
            "tool_calls": [
                {
                    "id": "call_Q1w2e3r4",
                    "type": "function",
                    "function": {
                        "name": "execute_shell_command",
                        "arguments": '{"shell_command": "ss -ltnp | grep 8080"}',
                    },
                }
            ],
        },
        {
            "role": "tool",
            "content": 'Exit code: 0, Output:\nLISTEN 0 4096 0.0.0.0:8080 users:(("python3",pid=4242))\n',
            "tool_call_id": "call_Q1w2e3r4",
        },
        {
            "role": "assistant",
            "content": '\n> @FunctionCall `execute_shell_command(shell_command="ss -ltnp | grep 8080")` '
            "\n\nA python3 process (PID 4242) is listening on 8080.",
        },
    ]


def shellgpt_legacy_messages():
    return [
        {"role": "system", "content": "You are ShellGPT"},
        {"role": "user", "content": "how much disk is free"},
        {
            "role": "assistant",
            "content": "",
            "function_call": {
                "name": "execute_shell_command",
                "arguments": '{"shell_command": "df -h /"}',
            },
        },
        {
            "role": "function",
            "content": "Exit code: 0, Output:\n/dev/sda1 50G 20G 30G 40% /\n",
            "name": "execute_shell_command",
        },
        {"role": "assistant", "content": "30G is free on /."},
    ]


def build_shellgpt(home: Path) -> None:
    temp = home / "AppData/Local/Temp"
    (temp / "chat_cache").mkdir(parents=True, exist_ok=True)
    (temp / "chat_cache" / SHELLGPT_CHAT).write_text(
        json.dumps(shellgpt_messages()), encoding="utf-8"
    )
    (temp / "shell_gpt/chat_cache").mkdir(parents=True, exist_ok=True)
    (temp / "shell_gpt/chat_cache" / SHELLGPT_LEGACY_CHAT).write_text(
        json.dumps(shellgpt_legacy_messages()), encoding="utf-8"
    )
    (temp / "shell_gpt/cache").mkdir(parents=True, exist_ok=True)
    (temp / "shell_gpt/cache/0cc175b9c0f1b6a831c399e269772661").write_text(
        "a cached response", encoding="utf-8"
    )
    # The same answer as the legacy chat's last message: a copy the primary holds.
    (temp / "shell_gpt/cache" / SHELLGPT_CACHE_COPY).write_text(
        "30G is free on /.", encoding="utf-8"
    )
    # An empty response is written too; it yields no row.
    (temp / "shell_gpt/cache/d41d8cd98f00b204e9800998ecf8427e").write_text("", encoding="utf-8")
    config = home / ".config/shell_gpt"
    config.mkdir(parents=True, exist_ok=True)
    (config / ".sgptrc").write_text(SHELLGPT_SGPTRC, encoding="utf-8")
    (config / "functions").mkdir(parents=True, exist_ok=True)
    (config / "functions/execute_shell.py").write_text("class Function: pass\n", encoding="utf-8")
    (config / "roles").mkdir(parents=True, exist_ok=True)
    _json_file(config / "roles/ShellGPT.json", {"name": "ShellGPT", "role": "assistant"})
