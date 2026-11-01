"""Synthetic Gemini CLI state for the analyzer tests.

The records follow the Record schema and Other stores of `research/gemini-cli.md`, at
google-gemini/gemini-cli 6814e00.

Paths: `/srv/other` (a trusted parent folder).
"""

from __future__ import annotations

import json
import os
from pathlib import Path

from .common import _jsonl

GEMINI_SESSION = "a1b2c3d4-0000-4000-8000-000000000001"
GEMINI_RESUMED = "a1b2c3d4-0000-4000-8000-0000000000ff"
GEMINI_LEGACY = "b2c3d4e5-0000-4000-8000-000000000002"
GEMINI_SUBAGENT = "c3d4e5f6-0000-4000-8000-000000000003"
GEMINI_HASH = (
    "6b9af143446a411aefa3edb76e4693799bd7513e7ed8d919962d6713d3d3b972"  # sha256("/srv/proj")
)
GEMINI_REL = ".gemini/tmp/proj/chats/session-2026-10-01T09-00-a1b2c3d4.jsonl"


def gemini_session_records():
    """Session JSONL in the chatRecordingTypes.ts shapes.

    A re-appended message, $set, $rewindTo, $patch and a $set.sessionId from a resume.

    Source: research/gemini-cli.md#session-jsonl; source 6814e00
    """
    timestamp_format = "2026-10-01T09:00:%s.000Z"
    ls_call = {
        "id": "list_directory__list_directory_1759309207000_0",
        "name": "list_directory",
        "args": {"dir_path": "/srv/proj/src"},
        "status": "executing",
        "timestamp": "2026-10-01T09:00:07.100Z",
        "displayName": "ReadFolder",
        "description": "Lists files",
    }
    ls_done = dict(
        ls_call,
        status="success",
        result=[
            {
                "functionResponse": {
                    "id": "list_directory__list_directory_1759309207000_0",
                    "name": "list_directory",
                    "response": {"output": "main.ts\nutil.ts"},
                }
            }
        ],
    )
    gemini_reply = {
        "id": "msg-g1",
        "timestamp": timestamp_format % "07",
        "type": "gemini",
        "content": "Listing now.",
        "thoughts": [
            {
                "subject": "Plan",
                "description": "Use the ls tool.",
                "timestamp": "2026-10-01T09:00:06.500Z",
            }
        ],
        "tokens": {"input": 120, "output": 12, "cached": 0, "thoughts": 8, "tool": 0, "total": 140},
        "model": "gemini-2.5-pro",
        "toolCalls": [ls_call],
    }
    read_file_call = {
        "id": "read_file__read_file_1759309222000_0",
        "name": "read_file",
        "args": {"file_path": "/srv/proj/src/util.ts"},
        "status": "success",
        "timestamp": "2026-10-01T09:00:22.100Z",
        "displayName": "ReadFile",
        "result": [
            {
                "functionResponse": {
                    "id": "read_file__read_file_1759309222000_0",
                    "name": "read_file",
                    "response": {"output": "draft"},
                }
            }
        ],
    }
    return [
        {
            "sessionId": GEMINI_SESSION,
            "projectHash": GEMINI_HASH,
            "startTime": timestamp_format % "00",
            "lastUpdated": timestamp_format % "00",
            "kind": "main",
        },
        {
            "id": "msg-u1",
            "timestamp": timestamp_format % "05",
            "type": "user",
            "content": [{"text": "list files in src"}],
        },
        gemini_reply,
        dict(gemini_reply, toolCalls=[ls_done]),
        {"$set": {"lastUpdated": timestamp_format % "08", "summary": "List src files"}},
        {
            "id": "msg-u2",
            "timestamp": timestamp_format % "10",
            "type": "user",
            "content": [{"text": "delete everything in /srv/proj"}],
        },
        {
            "id": "msg-g2",
            "timestamp": timestamp_format % "11",
            "type": "gemini",
            "content": "",
            "model": "gemini-2.5-pro",
            "toolCalls": [
                {
                    "id": "run_shell_command__run_shell_command_1759309211000_0",
                    "name": "run_shell_command",
                    "args": {"command": "rm -rf /srv/proj/*"},
                    "status": "awaiting_approval",
                    "timestamp": timestamp_format % "11",
                    "displayName": "Shell",
                }
            ],
        },
        {"$rewindTo": "msg-u2"},
        {
            "id": "msg-u3",
            "timestamp": timestamp_format % "20",
            "type": "user",
            "content": "show util.ts",
        },
        {
            "id": "msg-g3",
            "timestamp": timestamp_format % "22",
            "type": "gemini",
            "content": "draft answer",
            "model": "gemini-2.5-pro",
            "toolCalls": [read_file_call],
        },
        {
            "$patch": {
                "id": "msg-g3",
                "content": [{"text": "Here is util.ts."}],
                "toolCalls": [
                    {
                        "id": "read_file__read_file_1759309222000_0",
                        "result": [
                            {
                                "functionResponse": {
                                    "id": "read_file__read_file_1759309222000_0",
                                    "name": "read_file",
                                    "response": {"output": "export const x = 1"},
                                }
                            }
                        ],
                    }
                ],
            }
        },
        {
            "id": "msg-i1",
            "timestamp": timestamp_format % "23",
            "type": "info",
            "content": "Request cancelled.",
        },
        {
            "id": "msg-g4",
            "timestamp": timestamp_format % "24",
            "type": "gemini",
            "content": "",
            "model": "gemini-2.5-flash",
            "toolCalls": [
                {
                    "id": "run_shell_command__run_shell_command_1759309224000_0",
                    "name": "run_shell_command",
                    "args": {"command": "curl http://x"},
                    "status": "cancelled",
                    "timestamp": timestamp_format % "24",
                    "displayName": "Shell",
                }
            ],
        },
        {"$set": {"sessionId": GEMINI_RESUMED}},
        {
            "id": "msg-u4",
            "timestamp": "2026-10-01T09:01:00.000Z",
            "type": "user",
            "content": "resume work",
        },
    ]


def gemini_legacy_record():
    """A legacy single-object `session-*.json` with its `messages` list.

    Source: research/gemini-cli.md#transcript-stores; source 6814e00
    """
    return {
        "sessionId": GEMINI_LEGACY,
        "projectHash": GEMINI_HASH,
        "startTime": "2026-09-30T08:00:00.000Z",
        "lastUpdated": "2026-09-30T08:00:03.000Z",
        "messages": [
            {
                "id": "l-u1",
                "timestamp": "2026-09-30T08:00:01.000Z",
                "type": "user",
                "content": "hello",
            },
            {
                "id": "l-g1",
                "timestamp": "2026-09-30T08:00:03.000Z",
                "type": "gemini",
                "content": [{"text": "weighing it", "thought": True}, {"text": "Hi."}],
                "model": "gemini-2.0-flash",
            },
        ],
    }


def gemini_logs_records():
    """The `logs.json` prompt history of the session.

    Source: research/gemini-cli.md#logsjson-records; source 6814e00
    """
    return [
        {
            "sessionId": GEMINI_SESSION,
            "messageId": 0,
            "timestamp": "2026-10-01T09:00:05.000Z",
            "type": "user",
            "message": "list files in src",
        },
        {
            "sessionId": GEMINI_SESSION,
            "messageId": 1,
            "timestamp": "2026-10-01T09:00:10.000Z",
            "type": "user",
            "message": "delete everything in /srv/proj",
        },
    ]


GEMINI_TRUST_MTIME = 1790856000  # 1 October 12:00:00 UTC, trustedFolders.json's mtime


def build_gemini_cli(base: Path) -> None:
    """Write a `~/.gemini` with slug and hash directories, a subagent and noise.

    ~/.gemini with a slug directory (marker file), a legacy hash directory resolved through
    projects.json, a subagent session and noise.

    Source: research/gemini-cli.md#per-user-storage; source 6814e00
    """
    slug = base / "tmp/proj"
    _jsonl(base / GEMINI_REL[len(".gemini/") :], gemini_session_records())
    _jsonl(
        slug / "chats" / GEMINI_SESSION / (GEMINI_SUBAGENT + ".jsonl"),
        [
            {
                "sessionId": GEMINI_SUBAGENT,
                "projectHash": GEMINI_HASH,
                "startTime": "2026-10-01T09:00:30.000Z",
                "kind": "subagent",
            },
            {
                "id": "s-u1",
                "timestamp": "2026-10-01T09:00:30.000Z",
                "type": "user",
                "content": "investigate",
            },
        ],
    )
    (slug / ".project_root").write_text("/srv/proj", encoding="utf-8")
    (slug / "logs.json").write_text(json.dumps(gemini_logs_records()), encoding="utf-8")
    (slug / "shell_history").write_text("ls\n", encoding="utf-8")
    legacy = base / "tmp" / GEMINI_HASH / "chats"
    legacy.mkdir(parents=True, exist_ok=True)
    (legacy / "session-2026-09-30T08-00-b2c3d4e5.json").write_text(
        json.dumps(gemini_legacy_record()), encoding="utf-8"
    )
    (base / "projects.json").write_text(
        json.dumps({"projects": {"/srv/proj": "proj"}}), encoding="utf-8"
    )
    (base / "oauth_creds.json").write_text(
        json.dumps({"access_token": "secret", "token_type": "Bearer"}), encoding="utf-8"
    )
    trust = base / "trustedFolders.json"
    trust.write_text(
        json.dumps({"/srv/proj": "TRUST_FOLDER", "/srv/other": "TRUST_PARENT"}, indent=2),
        encoding="utf-8",
    )
    os.utime(trust, (GEMINI_TRUST_MTIME, GEMINI_TRUST_MTIME))


# Standing permission files and past-use copies of Gemini CLI, in the shapes
# of google-gemini/gemini-cli at 6814e00: settingsSchema.ts,
# policy/config.ts (createTomlRule), policy/sandboxPolicyManager.ts, hooks/trustedHooks.ts,
# agents/acknowledgedAgents.ts, useShellHistory.ts, core/logger.ts
# (saveCheckpoint) and bugCommand.ts with historyExportUtils.ts. Not wired
# into build_home, so the shared timeline counts stay as they are.
GEMINI_SOURCES_MTIME = 1790859600  # 1 October 13:00:00 UTC
GEMINI_BUG_MS = 1790856000000  # 1 October 12:00:00 UTC, in the file name
GEMINI_CHECKPOINT_REL = ".gemini/tmp/proj/checkpoint-my%20tag.json"
GEMINI_BUG_REL = ".gemini/tmp/proj/bug-report-history-%d.json" % GEMINI_BUG_MS


def gemini_settings():
    """`settings.json` with the permission, sandbox and retention keys.

    `settings.json` with the permission, sandbox and retention keys, and an MCP server environment
    the parser must not read.

    Source: research/gemini-cli.md#standing-permission-files; source 6814e00
    """
    return {
        "general": {
            "defaultApprovalMode": "auto_edit",
            "sessionRetention": {"enabled": True, "maxAge": "7d"},
        },
        "tools": {
            "sandbox": "docker",
            "allowed": ["run_shell_command(git)", "run_shell_command(npm test)"],
            "exclude": ["web_fetch"],
        },
        "security": {
            "disableYoloMode": True,
            "folderTrust": {"enabled": True},
            "environmentVariableRedaction": {"enabled": True},
        },
        "mcpServers": {"x": {"command": "x", "env": {"API_KEY": "secret"}}},
        "model": {"name": "gemini-2.5-pro"},
    }


# Two "Allow for all future sessions" grants as @iarna/toml writes them.
GEMINI_POLICY_TOML = """[[rule]]
decision = "allow"
priority = 950
toolName = "run_shell_command"
commandPrefix = [ "git", "status" ]

[[rule]]
decision = "allow"
priority = 950
toolName = "search"
mcpName = "docs"
modes = [ "autoEdit" ]
"""

# The sandbox's standing grants after one "allow and save" answer, as
# SandboxPolicyManager.saveConfig writes the whole config with @iarna/toml.
GEMINI_SANDBOX_TOML = """[modes.plan]
network = false
readonly = true
approvedTools = [ ]
allowOverrides = true

[modes.default]
network = false
readonly = false
approvedTools = [ ]
allowOverrides = true

[modes.accepting_edits]
network = false
readonly = false
approvedTools = [ "sed", "grep" ]
allowOverrides = true

[commands.cargo]
allowed_paths = [ "/srv/proj/target", "/srv/proj/.cargo" ]
allow_network = false
"""


def gemini_history_contents():
    """`Content[]` as chat.getHistory() returns it.

    A prompt, a reply with a function call, its response, a prompt the session rewound, and a reply
    with a thought part.

    Source: research/gemini-cli.md#checkpoint-records; source 6814e00
    """
    return [
        {"role": "user", "parts": [{"text": "list files in src"}]},
        {
            "role": "model",
            "parts": [
                {"text": "Listing now."},
                {
                    "functionCall": {
                        "id": "list_directory__list_directory_1759309207000_0",
                        "name": "list_directory",
                        "args": {"dir_path": "/srv/proj/src"},
                    }
                },
            ],
        },
        {
            "role": "user",
            "parts": [
                {
                    "functionResponse": {
                        "id": "list_directory__list_directory_1759309207000_0",
                        "name": "list_directory",
                        "response": {"output": "main.ts\nutil.ts"},
                    }
                }
            ],
        },
        {"role": "user", "parts": [{"text": "delete everything in /srv/proj"}]},
        {"role": "model", "parts": [{"text": "weighing it", "thought": True}, {"text": "No."}]},
    ]


def build_gemini_cli_sources(base: Path) -> None:
    """Write the Gemini CLI standing permission files and past-use copies.

    The standing permission files and the past-use copies under the `.gemini` directory `base`, with
    noise the parser must not want.

    Source: research/gemini-cli.md#standing-permission-files; source 6814e00
    """
    (base / "settings.json").write_text(json.dumps(gemini_settings()), encoding="utf-8")
    (base / "policies").mkdir(parents=True, exist_ok=True)
    (base / "policies/auto-saved.toml").write_text(GEMINI_POLICY_TOML, encoding="utf-8")
    (base / "policies/sandbox.toml").write_text(GEMINI_SANDBOX_TOML, encoding="utf-8")
    (base / "trusted_hooks.json").write_text(
        json.dumps({"/srv/proj": ["lint:npm run lint"]}, indent=2), encoding="utf-8"
    )
    (base / "acknowledgments").mkdir(exist_ok=True)
    (base / "acknowledgments/agents.json").write_text(
        json.dumps({"/srv/proj": {"reviewer": "0123abcd"}}, indent=2), encoding="utf-8"
    )
    slug = base / "tmp/proj"
    # Oldest first, joined with "\n" and no final newline; "make\" continues.
    (slug / "shell_history").write_bytes(b"ls -la\ngit status\nmake\\\ntest")
    (slug / "checkpoint-my%20tag.json").write_text(
        json.dumps({"history": gemini_history_contents(), "authType": "oauth-personal"}, indent=2),
        encoding="utf-8",
    )
    (slug / ("bug-report-history-%d.json" % GEMINI_BUG_MS)).write_text(
        json.dumps(gemini_history_contents()[:2], indent=2), encoding="utf-8"
    )
    # Noise: the policy integrity hashes, a tool checkpoint and a heap snapshot.
    (base / "policy_integrity.json").write_text("{}", encoding="utf-8")
    (slug / "checkpoints").mkdir(exist_ok=True)
    (slug / "checkpoints/2026-10-01T09_00_07_100Z-util.ts-write_file.json").write_text(
        "{}", encoding="utf-8"
    )
    (slug / "bug-memory-1790856000000.heapsnapshot").write_text("{}", encoding="utf-8")
    for rel in (
        "settings.json",
        "policies/auto-saved.toml",
        "policies/sandbox.toml",
        "trusted_hooks.json",
        "acknowledgments/agents.json",
        "tmp/proj/checkpoint-my%20tag.json",
    ):
        os.utime(base / rel, (GEMINI_SOURCES_MTIME, GEMINI_SOURCES_MTIME))


def build_gemini_cli_empty_settings(home: Path) -> None:
    """An empty Gemini CLI settings.json.

    Source: research/gemini-cli.md#standing-permission-files; source 6814e00
    """
    (home / ".gemini/settings.json").write_text("{}", encoding="utf-8")
