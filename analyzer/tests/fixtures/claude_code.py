"""Synthetic Claude Code state for the analyzer tests.

Paths: `/tmp/claude-0/-home-user-repo/22222222-3333-4444-8555-666666666666/scratchpad`
       `/private/tmp/claude-501/-srv-proj/11111111-2222-4333-8444-000000000002/scratchpad`
       (the teleported session's cloud and local scratchpads).
"""

from __future__ import annotations

import json
import os
from pathlib import Path

from .common import _jsonl

CLAUDE_SESSION = "11111111-2222-4333-8444-555555555555"
CLAUDE_WORK_SESSION = "11111111-2222-4333-8444-666666666666"
# A 2.1.295 session in the same project, and the session a Remote Control
# process in the registry fixture is bound to (no transcript).
CLAUDE_295_SESSION = "11111111-2222-4333-8444-777777777777"
CLAUDE_BRIDGE_SESSION = "11111111-2222-4333-8444-888888888888"
# A 2.1.296 session with permission mode and model changes.
CLAUDE_296_SESSION = "11111111-2222-4333-8444-999999999999"
# A 2.1.309 session in the key set of a Windows install, the session-daemon
# worker it was handed to, its scratchpad, its Remote Control bridge and its
# persisted tool output.
CLAUDE_309_SESSION = "11111111-2222-4333-8444-aaaaaaaaaaaa"
CLAUDE_309_WORKER = "11111111-2222-4333-8444-bbbbbbbbbbbb"
CLAUDE_309_SCRATCH = "/tmp/claude-1000/-srv-proj/%s/scratchpad" % CLAUDE_309_SESSION
CLAUDE_309_BRIDGE = "cse_0a1b2c3d4e5f6a7b"
CLAUDE_309_BRIDGE_URL = "https://claude.ai/code/session_0a1b2c3d4e5f6a7b"
CLAUDE_309_TOOL_RESULT = "k3j9x2m1q.txt"
CLAUDE_309_PERSISTED = "first line of the listing\nsecond line of the listing"
# A 2.1.310 session in the key set of a Linux install and a VS Code extension
# session of the same build; neither is written by build_home.
CLAUDE_310_SESSION = "11111111-2222-4333-8444-dddddddddddd"
CLAUDE_310_VSCODE_SESSION = "11111111-2222-4333-8444-eeeeeeeeeeee"
# 2.1.310 sessions in the key set of a macOS install: a terminal session, a
# Remote Control spawn and a teleported cloud session; none is written by
# build_home.
CLAUDE_MAC_SESSION = "11111111-2222-4333-8444-ffffffffffff"
CLAUDE_MAC_SPAWN_SESSION = "11111111-2222-5333-8444-000000000001"
CLAUDE_MAC_TELEPORT_SESSION = "11111111-2222-4333-8444-000000000002"
CLAUDE_MAC_CLEARED_SESSION = "11111111-2222-4333-8444-000000000003"
CLAUDE_MAC_CLOUD_SCRATCH = (
    "/tmp/claude-0/-home-user-repo/22222222-3333-4444-8555-666666666666/scratchpad"
)
CLAUDE_MAC_LOCAL_SCRATCH = "/private/tmp/claude-501/-srv-proj/%s/scratchpad" % (
    CLAUDE_MAC_TELEPORT_SESSION
)
CLAUDE_MAC_PASTE_HASH = "0f1e2d3c4b5a6978"
CLAUDE_MAC_LONG_PASTE = "alpha\nbeta\ngamma\ndelta\nepsilon"
# Modification time given to the settings fixtures: 1 October 09:00:00 UTC.
CLAUDE_SETTINGS_MTIME = 1790845200
CLAUDE_REGISTRY_KEY = "4242." + "ab" * 32 + ".key"


CLAUDE_TOOL_RESULT = "b1c2d3e4f.txt"
CLAUDE_PERSISTED = "line 1 of the full output\nline 2 of the full output"


def _claude_common(cwd, branch, **extra):
    return dict(
        sessionId=CLAUDE_SESSION,
        cwd=cwd,
        gitBranch=branch,
        version="2.1.289",
        userType="external",
        isSidechain=False,
        entrypoint="cli",
        **extra,
    )


def _claude_assistant(common, uuid, second, blocks, model="claude-fable-5-1", **extra):
    return dict(
        common,
        type="assistant",
        uuid=uuid,
        timestamp="2026-10-01T10:00:%s.000Z" % second,
        requestId="req_" + uuid,
        message={
            "model": model,
            "id": "msg_" + uuid,
            "type": "message",
            "role": "assistant",
            "content": blocks,
        },
        **extra,
    )


def _claude_user(common, uuid, second, content, **extra):
    return dict(
        common,
        type="user",
        uuid=uuid,
        timestamp="2026-10-01T10:00:%s.000Z" % second,
        message={"role": "user", "content": content},
        **extra,
    )


def _claude_attachment(common, uuid, second, attachment):
    return dict(
        common,
        type="attachment",
        uuid=uuid,
        timestamp="2026-10-01T10:00:%s.000Z" % second,
        attachment=attachment,
    )


def _claude_result(tool_use_id, content):
    return [{"tool_use_id": tool_use_id, "type": "tool_result", "content": content}]


def _claude_stub(name):
    return (
        "<persisted-output>\nOutput too large (32.8KB). Full output saved to: "
        "/home/alice/.claude/projects/-srv-proj/%s/tool-results/%s\n\n"
        "Preview (first 2KB):\nline 1 of the full output\n</persisted-output>"
    ) % (CLAUDE_SESSION, name)


def claude_session_records(cwd="/srv/proj", branch="main"):
    """Line numbers are what ClaudeCodeTests.test_rows asserts."""
    common = _claude_common(cwd, branch)
    session_id = CLAUDE_SESSION
    return [
        dict(type="mode", mode="normal", sessionId=session_id),  # 1
        _claude_user(
            common,
            "u1",
            "00",
            "delete the logs in /var/log please",
            parentUuid=None,
            origin={"kind": "human"},
            promptSource="typed",
        ),  # 2
        _claude_assistant(
            common,
            "a1",
            "01",
            [{"type": "thinking", "thinking": "private reasoning", "signature": "x"}],
        ),  # 3
        _claude_assistant(
            common, "a1b", "01", [{"type": "thinking", "thinking": "", "signature": "c2ln"}]
        ),  # 4: empty thinking, skipped
        _claude_assistant(
            common,
            "a2",
            "02",
            [
                {
                    "type": "tool_use",
                    "id": "toolu_01",
                    "name": "Bash",
                    "input": {"command": "rm -rf /var/log/*.log", "description": "Remove logs"},
                    "caller": {"type": "direct"},
                }
            ],
            wireToolInputs={
                "toolu_01": {
                    "command": "cd /srv/proj && rm -rf /var/log/*.log",
                    "description": "Remove logs",
                }
            },
            wireIngestContext={"toolu_01": {"cwd": "/srv/proj"}},
        ),  # 5
        _claude_user(
            common,
            "u2",
            "03",
            [
                {
                    "tool_use_id": "toolu_01",
                    "type": "tool_result",
                    "content": "removed 3 files",
                    "is_error": False,
                }
            ],
            toolUseResult={"stdout": "removed 3 files", "stderr": "", "interrupted": False},
            sourceToolAssistantUUID="a2",
        ),  # 6
        dict(
            type="queue-operation",
            operation="enqueue",
            timestamp="2026-10-01T10:00:03.500Z",
            sessionId=session_id,
            content="also check /tmp",
        ),  # 7: delivered by the attachment on line 8
        _claude_attachment(
            common,
            "t1",
            "04",
            {
                "type": "queued_command",
                "prompt": "also check /tmp",
                "commandMode": "prompt",
                "origin": {"kind": "human"},
                "humanTurn": True,
                "source_uuid": "q1",
                "delivery_id": "d1",
                "timestamp": "2026-10-01T10:00:03.500Z",
            },
        ),  # 8
        _claude_attachment(
            common,
            "t2",
            "04",
            {
                "type": "edited_text_file",
                "filename": "/srv/proj/notes.md",
                "snippet": "1\tfile body",
            },
        ),  # 9
        _claude_attachment(
            common,
            "t3",
            "04",
            {
                "type": "hook_system_message",
                "hookName": "PostToolUse:Bash",
                "hookEvent": "PostToolUse",
                "toolUseID": "toolu_01",
                "content": "lint passed",
            },
        ),  # 10
        _claude_attachment(
            common, "t4", "04", {"type": "total_tokens_reminder", "text": "tokens used"}
        ),  # 11: skipped
        _claude_assistant(
            common, "a3", "04", [{"type": "text", "text": "Done. Three log files were removed."}]
        ),  # 12
        dict(
            common,
            type="system",
            uuid="s1",
            parentUuid="a3",
            timestamp="2026-10-01T10:00:05.000Z",
            subtype="turn_duration",
            durationMs=4000,
            messageCount=5,
            isMeta=False,
        ),  # 13
        _claude_user(
            common,
            "u3",
            "06",
            "<command-name>/model</command-name>\n<command-message>model</command-message>\n"
            "<command-args></command-args>",
        ),  # 14
        _claude_user(
            common, "u4", "06", "<local-command-stdout>Set model to Fable</local-command-stdout>"
        ),  # 15
        _claude_user(
            common,
            "u5",
            "07",
            "<task-notification>\n<task-id>b1</task-id>\n<status>completed</status>\n"
            "</task-notification>",
            origin={"kind": "task-notification"},
            promptSource="system",
        ),  # 16
        _claude_assistant(
            common,
            "a4",
            "08",
            [
                {
                    "type": "tool_use",
                    "id": "toolu_02",
                    "name": "ToolSearch",
                    "input": {"query": "select:WebFetch"},
                }
            ],
        ),  # 17
        _claude_user(
            common,
            "u6",
            "09",
            _claude_result("toolu_02", [{"type": "tool_reference", "tool_name": "WebFetch"}]),
        ),  # 18
        _claude_assistant(
            common,
            "a5",
            "10",
            [
                {
                    "type": "tool_use",
                    "id": "toolu_03",
                    "name": "Read",
                    "input": {"file_path": "/srv/proj/big.log"},
                }
            ],
        ),  # 19
        _claude_user(
            common, "u7", "11", _claude_result("toolu_03", _claude_stub(CLAUDE_TOOL_RESULT))
        ),  # 20: the stub's file exists
        _claude_assistant(
            common,
            "a6",
            "12",
            [
                {
                    "type": "tool_use",
                    "id": "toolu_04",
                    "name": "Read",
                    "input": {"file_path": "/srv/proj/gone.log"},
                }
            ],
        ),  # 21
        _claude_user(
            common, "u8", "13", _claude_result("toolu_04", _claude_stub("f0e1d2c3b.txt"))
        ),  # 22: the stub's file is missing
        _claude_assistant(
            common,
            "a7",
            "14",
            [{"type": "text", "text": "API Error: rate limited"}],
            model="<synthetic>",
            isApiErrorMessage=True,
            apiErrorStatus=429,
            error="rate_limit",
        ),  # 23
        dict(
            common,
            type="system",
            uuid="s2",
            timestamp="2026-10-01T10:00:15.000Z",
            subtype="api_error",
            level="error",
            retryInMs=500,
            retryAttempt=1,
            maxRetries=10,
            isMeta=False,
        ),  # 24
        _claude_user(
            common,
            "u9",
            "16",
            [{"type": "text", "text": "[Request interrupted by user]"}],
            interruptedMessageId="msg_a7",
        ),  # 25
        _claude_user(
            common,
            "u10",
            "17",
            "This session is being continued from a previous conversation. Summary: logs removed.",
            isCompactSummary=True,
        ),  # 26
        dict(type="ai-title", aiTitle="Remove old logs", sessionId=session_id),  # 27
        dict(
            type="ai-title", aiTitle="Remove old logs", sessionId=session_id
        ),  # 28: repeat, skipped
        dict(type="custom-title", customTitle="log cleanup", sessionId=session_id),  # 29
        dict(
            type="pr-link",
            sessionId=session_id,
            prNumber=7,
            prUrl="https://github.com/x/y/pull/7",
            prRepository="x/y",
            timestamp="2026-10-01T10:00:18.000Z",
        ),  # 30
        dict(
            type="pr-link",
            sessionId=session_id,
            prNumber=7,
            prUrl="https://github.com/x/y/pull/7",
            prRepository="x/y",
            timestamp="2026-10-01T10:00:19.000Z",
        ),  # 31: re-appended, skipped
        dict(type="relocated", relocatedCwd="/srv/proj2", sessionId=session_id),  # 32
        _claude_user(
            common, "u11", "20", "thanks", origin={"kind": "human"}, promptSource="typed"
        ),  # 33
        dict(
            type="file-history-snapshot",
            messageId="u1",
            snapshot={
                "messageId": "u1",
                "trackedFileBackups": {},
                "timestamp": "2026-10-01T10:00:00.000Z",
            },
            isSnapshotUpdate=False,
        ),  # 34
    ]


def claude_subagent_records():
    common = _claude_common("/srv/proj", "main", agentId="abc")
    common["isSidechain"] = True
    return [
        _claude_user(common, "s1", "01", "Find where the logs are rotated.", parentUuid=None),
        _claude_assistant(common, "s2", "02", [{"type": "text", "text": "By logrotate."}]),
    ]


def claude_fork_records():
    """A forked subagent.

    A copy of the forking agent's context, then the task in a `<fork-boilerplate>` block beside the
    fork call's result.
    """
    common = _claude_common("/srv/proj", "main", agentId="fork1")
    common["isSidechain"] = True
    return [
        _claude_user(common, "f1", "02", "delete the logs in /var/log please", parentUuid=None),
        _claude_assistant(
            common,
            "f2",
            "02",
            [
                {
                    "type": "tool_use",
                    "id": "toolu_f1",
                    "name": "Agent",
                    "input": {"description": "Check rotation", "prompt": "Check rotation"},
                }
            ],
        ),
        _claude_user(
            common,
            "f3",
            "02",
            [
                {"tool_use_id": "toolu_f1", "type": "tool_result", "content": "forked"},
                {
                    "type": "text",
                    "text": "<fork-boilerplate>\nYou are a fork.\n</fork-boilerplate>\n"
                    "Check the rotation config.",
                },
            ],
        ),
    ]


def claude_work_records():
    """A session in a config home moved with CLAUDE_CONFIG_DIR (~/.claude-work)."""
    common = _claude_common("/srv/proj", "main")
    common["sessionId"] = CLAUDE_WORK_SESSION
    return [
        _claude_user(common, "w1", "30", "rotate the work logs", parentUuid=None),
        _claude_assistant(common, "w2", "31", [{"type": "text", "text": "Rotated."}]),
    ]


def claude_work_history_records():
    return [
        {
            "display": "rotate the work logs",
            "pastedContents": {},
            "timestamp": 1790848830000,
            "project": "/srv/proj",
            "sessionId": CLAUDE_WORK_SESSION,
        },
    ]


def claude_295_records():
    """A Claude Code 2.1.295 session.

    The state lines and keys that version writes (research/claude-code.md, Record schema and Other
    stores). Line numbers are what ClaudeCode295Tests asserts.
    """
    common = _claude_common("/srv/proj", "main")
    common.update(sessionId=CLAUDE_295_SESSION, version="2.1.295", entrypoint="claude-desktop")
    session_id = CLAUDE_295_SESSION
    prompt = "list the cron jobs"
    return [
        dict(
            type="queue-operation",
            operation="enqueue",
            content=prompt,
            sessionId=session_id,
            timestamp="2026-10-01T11:00:00.000Z",
        ),  # 1
        dict(
            type="queue-operation",
            operation="dequeue",
            sessionId=session_id,
            timestamp="2026-10-01T11:00:00.100Z",
        ),  # 2
        dict(
            type="file-history-snapshot",
            messageId="p1",
            snapshot={
                "messageId": "p1",
                "trackedFileBackups": {},
                "timestamp": "2026-10-01T11:00:00.200Z",
            },
            isSnapshotUpdate=False,
        ),  # 3
        dict(
            common,
            type="user",
            uuid="p1",
            parentUuid=None,
            timestamp="2026-10-01T11:00:01.000Z",
            message={"role": "user", "content": prompt},
            turnOrigin="human",
            promptSource="sdk",
            permissionMode="auto",
            origin={"kind": "human"},
            turnPosition=0,
            promptId="prompt-0001",
        ),  # 4
        dict(
            common,
            type="assistant",
            uuid="p2",
            parentUuid="p1",
            timestamp="2026-10-01T11:00:02.000Z",
            requestId="req_p2",
            message={
                "model": "claude-fable-5-1",
                "id": "msg_p2",
                "type": "message",
                "role": "assistant",
                "content": [
                    {
                        "type": "tool_use",
                        "id": "toolu_95",
                        "name": "Bash",
                        "input": {"command": "crontab -l", "description": "List cron jobs"},
                    }
                ],
            },
            advisorModel="claude-fable-5-1",
            apiBlockIndex=0,
            effort="high",
            perTurnEffort="high",
            requestedModel="claude-fable-5-1",
            serverClassifierRequest=True,
            thinkingDurationMs=120,
            wireToolInputs={"toolu_95": {"command": "cd /srv/proj && crontab -l"}},
        ),  # 5
        dict(
            common,
            type="user",
            uuid="p3",
            parentUuid="p2",
            timestamp="2026-10-01T11:00:03.000Z",
            message={
                "role": "user",
                "content": [
                    {"tool_use_id": "toolu_95", "type": "tool_result", "content": "0 3 * * * x"}
                ],
            },
            toolUseResult={"stdout": "0 3 * * * x", "stderr": "", "interrupted": False},
            permissionDecision={
                "decision": "allow",
                "source": "auto_mode",
                "reasonType": "classifier",
            },
            sourceToolAssistantUUID="p2",
            serverClassifierContext={"verdict": "allow"},
        ),  # 6
        dict(
            type="file-history-delta",
            backup={"backupFileName": None, "backupTime": "2026-10-01T11:00:03.100Z"},
            messageId="p3",
            snapshotMessageId="p1",
            timestamp="2026-10-01T11:00:03.100Z",
            trackingPath="/srv/proj/notes.md",
        ),  # 7
        dict(
            common,
            type="system",
            uuid="p4",
            parentUuid="p3",
            timestamp="2026-10-01T11:00:04.000Z",
            subtype="stop_hook_summary",
            hasOutput=False,
            hookAdditionalContext=[],
            hookCount=1,
            hookErrors=[],
            hookInfos=[{"command": "true", "durationMs": 5}],
            level="suggestion",
            preventedContinuation=False,
            stopReason="",
            toolUseID="toolu_95",
        ),  # 8
        dict(type="last-prompt", lastPrompt=prompt, leafUuid="p4", sessionId=session_id),  # 9
        dict(type="atis-latch", atis=True, sessionId=session_id),  # 10
    ]


def claude_296_records():
    """A Claude Code 2.1.296 session.

    The `permission-mode` record and the prompt key `permissionMode` changing, and a `/model` switch
    with the command output text of that version (research/claude-code.md, section 8). Line numbers
    are what ClaudeCode296Tests asserts.
    """
    common = _claude_common("/srv/proj", "main")
    common.update(sessionId=CLAUDE_296_SESSION, version="2.1.296")
    session_id = CLAUDE_296_SESSION

    def user(uuid, second, content, mode):
        return dict(
            common,
            type="user",
            uuid=uuid,
            timestamp="2026-10-01T12:00:%s.000Z" % second,
            message={"role": "user", "content": content},
            permissionMode=mode,
            promptSource="typed",
        )

    def assistant(uuid, second, text, model, requested):
        return dict(
            common,
            type="assistant",
            uuid=uuid,
            timestamp="2026-10-01T12:00:%s.000Z" % second,
            requestId="req_" + uuid,
            message={
                "model": model,
                "id": "msg_" + uuid,
                "type": "message",
                "role": "assistant",
                "content": [{"type": "text", "text": text}],
            },
            requestedModel=requested,
        )

    return [
        user("m1", "01", "plan the log cleanup", "auto"),  # 1
        assistant("m2", "02", "Here is a plan.", "claude-fable-5-1", "claude-fable-5-1"),  # 2
        dict(type="permission-mode", permissionMode="auto", sessionId=session_id),  # 3: unchanged
        dict(type="permission-mode", permissionMode="plan", sessionId=session_id),  # 4
        user("m3", "05", "go on", "plan"),  # 5: unchanged
        user(
            "m4",
            "06",
            "<command-name>/model</command-name>\n<command-message>model</command-message>\n"
            "<command-args>opus</command-args>",
            "plan",
        ),  # 6
        user(
            "m5",
            "07",
            "<local-command-stdout>Set model to `Opus 5.5` and saved as your default "
            "for new sessions</local-command-stdout>",
            "plan",
        ),  # 7
        assistant("m6", "08", "Switched.", "claude-opus-5-5", "opus"),  # 8
        user("m7", "09", "apply it", "acceptEdits"),  # 9
        assistant("m8", "10", "Applied.", "claude-opus-5-5", "opus"),  # 10: unchanged
    ]


def claude_309_records(entrypoint="cli", session=CLAUDE_309_SESSION):
    """A Claude Code 2.1.309 session in the key set of a Windows install.

    The line types and keys
    (research/claude-code.md, Record schema): the environment attachment's scratchpad, the
    request-capture lines, a one-time approval with `grantScope`/`reasonCode` and a persisted output
    named by `toolUseResult.persistedOutputPath`, a denial, a Remote Control bridge, prompts that
    were not typed, and the daemon handover. Values are invented. Line numbers are what
    ClaudeCode309Tests asserts.
    """
    common = _claude_common("/srv/proj", "main")
    common.update(sessionId=session, version="2.1.309", entrypoint=entrypoint)
    session_id = session
    side = dict(common, sessionKind="bg", slug="lab-round")

    def stamp(second):
        return "2026-10-01T13:00:%s.000Z" % second

    def prompt(uuid, second, text, **keys):
        return dict(
            common,
            type="user",
            uuid=uuid,
            timestamp=stamp(second),
            message={"role": "user", "content": text},
            permissionMode="default",
            sandboxed=False,
            turnPosition=0,
            session_id=session_id,
            **keys,
        )

    def call(uuid, second, tool_id, command):
        return dict(
            side,
            type="assistant",
            uuid=uuid,
            timestamp=stamp(second),
            requestId="req_" + uuid,
            requestRef="ref_" + uuid,
            requestedModel="claude-sonnet-5-5",
            thinkingDisplay="summarized",
            thinkingDurationMs=900,
            message={
                "model": "claude-sonnet-5-5",
                "id": "msg_" + uuid,
                "type": "message",
                "role": "assistant",
                "content": [
                    {
                        "type": "tool_use",
                        "id": tool_id,
                        "name": "PowerShell",
                        "input": {"command": command, "description": "Run the script"},
                    }
                ],
            },
        )

    def result(uuid, second, tool_id, content, **keys):
        return dict(
            common,
            type="user",
            uuid=uuid,
            timestamp=stamp(second),
            message={
                "role": "user",
                "content": [{"tool_use_id": tool_id, "type": "tool_result", "content": content}],
            },
            sourceToolAssistantUUID="a" + uuid,
            **keys,
        )

    def bridge(bridge_id, seq):
        return dict(
            type="bridge-session",
            bridgeSessionId=bridge_id,
            lastSequenceNum=seq,
            ownerAccountUuid="00000000-0000-4000-8000-00000000000a",
            ownerOrganizationUuid="00000000-0000-4000-8000-00000000000b",
            sessionId=session_id,
        )

    saved = "C:\\Users\\alice\\.claude\\projects\\-srv-proj\\%s\\tool-results\\%s" % (
        session_id,
        CLAUDE_309_TOOL_RESULT,
    )
    shell = {"interrupted": False, "isImage": False, "noOutputExpected": False, "stderr": ""}
    return [
        dict(type="mode", mode="normal", sessionId=session_id),  # 1
        dict(type="permission-mode", permissionMode="default", sessionId=session_id),  # 2
        prompt(
            "r1",
            "01",
            "Run python calc.py 2 3",
            origin={"kind": "human"},
            promptSource="typed",
            turnOrigin="human",
        ),  # 3
        dict(
            side,
            type="attachment",
            uuid="r2",
            timestamp=stamp("01"),
            attachment={
                "type": "environment",
                "snapshot": {
                    "additionalWorkingDirectories": [],
                    "isGitRepo": True,
                    "isWorktree": False,
                    "osVersion": "Linux 6.17",
                    "platform": "linux",
                    "scratchpadDirectory": CLAUDE_309_SCRATCH.replace(
                        CLAUDE_309_SESSION, session_id
                    ),
                    "shell": "bash",
                    "workingDirectory": "/srv/proj",
                },
            },
        ),  # 4
        dict(
            side,
            type="attachment",
            uuid="r3",
            timestamp=stamp("01"),
            attachment={
                "type": "auto_mode",
                "autoModeConsentFlow": "none",
                "bashFirst": False,
                "bashFirstSteer": False,
                "bypass": False,
                "finishExposedWork": False,
                "steerOnly": False,
            },
        ),  # 5: the turn's auto-mode configuration, skipped
        dict(
            type="api-request-shape",
            shape={"tools": []},
            shapeHash="0f0f",
            timestamp=stamp("02"),
            version="2.1.309",
            sessionId=session_id,
        ),  # 6
        dict(
            type="api-request",
            base="0f0f",
            cacheControl={},
            id="areq1",
            keep=[],
            params={"model": "claude-sonnet-5-5", "max_tokens": 32000},
            querySource="repl_main_thread",
            requestDigest="d1",
            shapeHash="0f0f",
            tail=[],
            timestamp=stamp("02"),
            version="2.1.309",
        ),  # 7
        dict(
            type="api-request-blob",
            hash="b1",
            message={"role": "user", "content": "Run python calc.py 2 3"},
        ),  # 8: the prompt again, inside the captured request; skipped
        call("ar4", "03", "toolu_309a", "python calc.py 2 3"),  # 9
        result(
            "r4",
            "04",
            "toolu_309a",
            "<persisted-output>\nOutput too large (1.2MB).\n\nPreview (first 2KB):\n"
            "first line of the listing\n</persisted-output>",
            toolUseResult=dict(
                shell,
                stdout="first line of the listing",
                persistedOutputPath=saved,
                persistedOutputSize=1200000,
            ),
            permissionDecision={
                "decision": "accept",
                "source": "user_temporary",
                "grantScope": "once",
                "reasonCode": "other",
            },
        ),  # 10
        call("ar5", "05", "toolu_309b", "python calc.py 5 7"),  # 11
        result(
            "r5",
            "06",
            "toolu_309b",
            "The user doesn't want to proceed with this tool use.",
            toolUseResult="User rejected tool use",
            permissionDecision={
                "decision": "reject",
                "source": "user_reject",
                "reasonCode": "other",
            },
            toolDenialKind="user-rejected",
            userFeedback="use the test runner instead",
        ),  # 12
        dict(
            side,
            type="system",
            uuid="r6",
            timestamp=stamp("07"),
            subtype="bridge_status",
            content="/remote-control is active",
            url=CLAUDE_309_BRIDGE_URL,
            isMeta=False,
            level="info",
        ),  # 13
        bridge(CLAUDE_309_BRIDGE, 1),  # 14
        prompt(
            "r7",
            "08",
            "Run python calc.py 3 4",
            origin={"kind": "human"},
            promptSource="queued",
            turnOrigin="human",
            queueSkipSlashCommands=True,
        ),  # 15: sent from the phone
        bridge(CLAUDE_309_BRIDGE, 1),  # 16: re-appended, skipped
        bridge(None, 4),  # 17: detached
        prompt(
            "r8",
            "09",
            "Say tick in one word",
            turnOrigin="scheduled",
            scheduledFireId="fire1",
            scheduledTaskId="task1",
            wakeupSource="cron",
            queuePriority="later",
        ),  # 18: a /loop firing
        prompt(
            "r9",
            "10",
            "Add a subtract function to calc.py",
            origin={"kind": "human"},
            promptSource="suggestion_accepted",
            turnOrigin="human",
        ),  # 19
        dict(
            type="continued-in",
            continuedInSessionId=CLAUDE_309_WORKER,
            sessionId=session_id,
            timestamp=stamp("11"),
        ),  # 20
        dict(
            type="cost-state",
            hasUnknownModelCost=False,
            modelUsage={},
            sessionId=session_id,
            startTime=1790859600000,
            totalAPIDuration=4000,
            totalAPIDurationWithoutRetries=4000,
            totalCostUSD=0.01,
            totalDuration=11000,
            totalLinesAdded=0,
            totalLinesRemoved=0,
            totalToolDuration=900,
        ),  # 21
    ]


def claude_309_history_records():
    """`history.jsonl` lines of 2.1.309.

    A long paste is named by its hash in `pastedContents`, its text kept in
    `paste-cache/<contentHash>.txt`.
    """
    return [
        {
            "display": "[Pasted text #1 +40 lines] Summarise the text above in one sentence",
            "pastedContents": {"1": {"contentHash": "0123456789abcdef", "id": 1, "type": "text"}},
            "timestamp": 1790859600000,
            "project": "/srv/proj",
            "sessionId": CLAUDE_309_SESSION,
        },
    ]


def claude_310_records():
    """A Claude Code 2.1.310 session in the key set of a Linux install.

    The line types and keys
    (research/claude-code.md, Record schema): the rule-miss `reasonCode` on a one-time approval, a
    denial and an allow-always answer, a compaction that re-emits the session-state records, the
    attachments a compaction re-injects, a prompt on a detached head, and the 2.1.310 keys the
    parser does not read. Values are invented; the session-state records carry only the keys the
    parser reads. Line numbers are what ClaudeCode310Tests asserts.
    """
    common = _claude_common("/srv/proj", "main")
    common.update(
        sessionId=CLAUDE_310_SESSION, version="2.1.310", entrypoint="cli", slug="lab-round"
    )
    session_id = CLAUDE_310_SESSION

    def stamp(second):
        return "2026-10-01T10:00:%s.000Z" % second

    def prompt(uuid, second, text, **keys):
        return dict(
            common,
            type="user",
            uuid=uuid,
            timestamp=stamp(second),
            message={"role": "user", "content": text},
            origin={"kind": "human"},
            promptSource="typed",
            turnOrigin="human",
            turnPosition=0,
            session_id=session_id,
            **keys,
        )

    def call(uuid, second, tool_id, command):
        return _claude_assistant(
            common,
            uuid,
            second,
            [{"type": "tool_use", "id": tool_id, "name": "Bash", "input": {"command": command}}],
            model="claude-sonnet-5-5",
            requestedModel="claude-sonnet-5-5",
        )

    def result(uuid, second, tool_id, content, decision, **keys):
        return dict(
            common,
            type="user",
            uuid=uuid,
            timestamp=stamp(second),
            message={
                "role": "user",
                "content": [{"tool_use_id": tool_id, "type": "tool_result", "content": content}],
            },
            sourceToolAssistantUUID="a" + uuid,
            permissionDecision=decision,
            **keys,
        )

    shell = {
        "interrupted": False,
        "isImage": False,
        "noOutputExpected": False,
        "stderr": "",
        "type": "text",
    }
    miss = "bashMiss:no-rule-match"
    snapshot = {
        "type": "file-history-snapshot",
        "isSnapshotUpdate": False,
        "messageId": "u1",
        "snapshot": {"messageId": "u1", "timestamp": stamp("01"), "trackedFileBackups": {}},
    }
    return [
        dict(type="mode", mode="normal", sessionId=session_id),  # 1
        dict(type="permission-mode", permissionMode="auto", sessionId=session_id),  # 2
        prompt("u1", "01", "Run python calc.py 2 3", permissionMode="default"),  # 3
        call("a2", "02", "toolu_310a", "python3 calc.py 2 3"),  # 4
        result(
            "u2",
            "03",
            "toolu_310a",
            "5",
            {
                "decision": "accept",
                "source": "user_temporary",
                "grantScope": "once",
                "reasonCode": miss,
            },
            sandboxed=False,
            toolUseResult=dict(shell, stdout="5"),
        ),  # 5
        call("a3", "04", "toolu_310b", "python3 calc.py 5 7"),  # 6
        result(
            "u3",
            "05",
            "toolu_310b",
            "The user doesn't want to proceed with this tool use.",
            {"decision": "reject", "source": "user_reject", "reasonCode": miss},
            toolUseResult="User rejected tool use",
            toolDenialKind="user-rejected",
            userFeedback="I changed my mind",
        ),  # 7
        call("a4", "06", "toolu_310c", "python3 calc.py 1 1"),  # 8
        result(
            "u4",
            "07",
            "toolu_310c",
            "2",
            {"decision": "accept", "source": "user_permanent", "reasonCode": miss},
            sandboxed=False,
            toolUseResult=dict(shell, stdout="2"),
        ),  # 9
        dict(type="ai-title", aiTitle="Run calc.py", sessionId=session_id),  # 10
        dict(
            common,
            type="system",
            uuid="s5",
            timestamp=stamp("08"),
            subtype="compact_boundary",
            content="Conversation compacted",
            logicalParentUuid="u4",
            level="info",
            compactMetadata={
                "cumulativeDroppedTokens": 0,
                "durationMs": 9000,
                "postTokens": 4000,
                "preCompactDiscoveredTools": [],
                "preTokens": 40000,
                # The 2.1.310 macOS shape (research/claude-code.md, Record
                # schema): the segment holds the three uuids, the messages
                # object the two lists.
                "preservedMessages": {"allUuids": [], "uuids": []},
                "preservedSegment": {"anchorUuid": "u4", "headUuid": "u1", "tailUuid": "u4"},
                "trigger": "manual",
            },
        ),  # 11
        prompt(
            "u6",
            "09",
            "We ran calc.py three times.",
            isCompactSummary=True,
            isVisibleInTranscriptOnly=True,
        ),  # 12
        _claude_attachment(
            common,
            "t7",
            "09",
            {
                "type": "compact_file_reference",
                "displayPath": "calc.py",
                "filename": "/srv/proj/calc.py",
            },
        ),  # 13
        _claude_attachment(
            common,
            "t8",
            "09",
            {
                "type": "plan_file_reference",
                "planContent": "1. add subtract",
                "planFilePath": "/home/alice/.claude/plans/functional-painting-curry.md",
            },
        ),  # 14
        _claude_attachment(
            common,
            "t9",
            "09",
            {
                "type": "file",
                "content": {"file": {}, "type": "text"},
                "displayPath": "calc.py",
                "filename": "/srv/proj/calc.py",
            },
        ),  # 15
        # The compaction re-emits the session-state records with unchanged values.
        dict(type="mode", mode="normal", sessionId=session_id),  # 16
        dict(type="permission-mode", permissionMode="default", sessionId=session_id),  # 17
        dict(type="ai-title", aiTitle="Run calc.py", sessionId=session_id),  # 18
        dict(type="last-prompt", sessionId=session_id),  # 19
        dict(type="atis-latch", sessionId=session_id),  # 20
        snapshot,  # 21
        dict(type="permission-mode", permissionMode="default", sessionId=session_id),  # 22
        dict(type="ai-title", aiTitle="Run calc.py", sessionId=session_id),  # 23
        prompt(
            "u10",
            "10",
            "What git branch is checked out?",
            permissionMode="default",
            gitBranch="HEAD",
            queueSkipAttachments=True,
        ),  # 24: a detached head, a prompt typed while another turn ran
        _claude_attachment(
            dict(common, gitBranch="HEAD"),
            "t11",
            "10",
            {"type": "plan_mode_reentry", "planFilePath": "/home/alice/.claude/plans/x.md"},
        ),  # 25
        _claude_attachment(
            dict(common, gitBranch="HEAD"),
            "t12",
            "10",
            {"type": "deferred_tools_delta", "needsAuthMcpServers": []},
        ),  # 26
        _claude_assistant(
            dict(common, gitBranch="HEAD"),
            "a13",
            "11",
            [{"type": "text", "text": "HEAD is detached."}],
            model="claude-sonnet-5-5",
            requestedModel="claude-sonnet-5-5",
            wireIngestContext={},
        ),  # 27
        dict(
            common,
            type="system",
            uuid="s14",
            timestamp=stamp("12"),
            gitBranch="HEAD",
            subtype="turn_duration",
            durationMs=4000,
            messageCount=3,
            pendingBackgroundAgentCount=0,
            isMeta=False,
        ),  # 28
    ]


def claude_310_vscode_records():
    """A 2.1.310 VS Code extension session file.

    `entrypoint` claude-vscode, no `mode` or `permission-mode` lines, `queue-operation` lines
    without `content` and a `cost-state` line (research/claude-code.md, Record schema). Line numbers
    are what ClaudeCode310Tests asserts.
    """
    common = _claude_common("/srv/proj", "main")
    common.update(
        sessionId=CLAUDE_310_VSCODE_SESSION, version="2.1.310", entrypoint="claude-vscode"
    )
    session_id = CLAUDE_310_VSCODE_SESSION
    return [
        dict(
            type="queue-operation",
            operation="enqueue",
            sessionId=session_id,
            timestamp="2026-10-01T10:00:20.000Z",
        ),  # 1
        dict(
            type="queue-operation",
            operation="dequeue",
            sessionId=session_id,
            timestamp="2026-10-01T10:00:20.000Z",
        ),  # 2
        _claude_user(
            common,
            "v3",
            "21",
            "Run python3 calc.py 6 1",
            origin={"kind": "human"},
            promptSource="sdk",
            turnOrigin="human",
            permissionMode="auto",
        ),  # 3
        dict(
            type="cost-state",
            hasUnknownModelCost=False,
            modelUsage={},
            sessionId=session_id,
            startTime=1790935200000,
            totalAPIDuration=3000,
            totalAPIDurationWithoutRetries=3000,
            totalCostUSD=0.01,
            totalDuration=5000,
            totalLinesAdded=0,
            totalLinesRemoved=0,
            totalToolDuration=800,
        ),  # 4
    ]


def claude_310_user_settings():
    """`~/.claude/settings.json` as 2.1.310 left it on Linux after the retention steps.

    No `permissions`, the model alias, the phone's push opt-in and a one-day retention period.
    """
    return {"theme": "dark", "model": "opus", "agentPushNotifEnabled": True, "cleanupPeriodDays": 1}


def claude_310_global_config():
    """`~/.claude.json` of 2.1.310 after `/install-github-app`.

    The GitHub connection cache, whose UUIDs must never be printed, beside the project entry and
    `githubRepoPaths`, which is not read.
    """
    return {
        "oauthAccount": {"emailAddress": "user@example.invalid"},
        "projects": {"/srv/proj": {"allowedTools": [], "hasTrustDialogAccepted": True}},
        "githubRepoPaths": {"example/logs": ["/srv/proj"]},
        "githubWebConnectionStatusCache": {
            "accountUuid": "00000000-0000-4000-8000-0000000000aa",
            "orgUuid": "00000000-0000-4000-8000-0000000000bb",
            "status": "connected",
        },
    }


def _claude_command(name, args=""):
    """A slash command's `user` text in the transcript's tag form."""
    return (
        "<command-name>/%s</command-name>\n            <command-message>%s</command-message>\n"
        "            <command-args>%s</command-args>" % (name, name, args)
    )


def claude_mac_records():
    """A 2.1.310 terminal session in the key set of a macOS install.

    The line types follow research/claude-code.md, Record schema: a `No` at the permission dialog
    and a declined plan, each a result line with `toolDenialKind`; `/model` twice and `/rename` in
    tag form; a short and a long paste; a `/loop` firing; the `auto_mode`, `auto_mode_exit` and
    `command_permissions` attachments; and `cost-state`. Values are invented. Line numbers are what
    ClaudeCodeMacTests asserts; `claude_mac_history_records` gives the same events' history lines.
    """
    common = _claude_common("/srv/proj", "main")
    common.update(
        sessionId=CLAUDE_MAC_SESSION, version="2.1.310", entrypoint="cli", slug="lab-round"
    )
    session_id = CLAUDE_MAC_SESSION

    def stamp(second):
        return "2026-10-01T11:00:%s.000Z" % second

    def prompt(uuid, second, text, **keys):
        record = dict(
            common,
            type="user",
            uuid=uuid,
            timestamp=stamp(second),
            message={"role": "user", "content": text},
            origin={"kind": "human"},
            promptSource="typed",
            turnOrigin="human",
            permissionMode="default",
        )
        record.update(keys)
        return record

    def call(uuid, second, tool_id, name, inp):
        return dict(
            _claude_assistant(
                common,
                uuid,
                second,
                [{"type": "tool_use", "id": tool_id, "name": name, "input": inp}],
            ),
            timestamp=stamp(second),
        )

    def denied(uuid, second, tool_id):
        # 2.1.310 macOS: the denial kind only, no decision record and no
        # `userFeedback`.
        return dict(
            common,
            type="user",
            uuid=uuid,
            timestamp=stamp(second),
            message={
                "role": "user",
                "content": _claude_result(
                    tool_id, "The user doesn't want to proceed with this tool use."
                ),
            },
            toolUseResult="User rejected tool use",
            toolDenialKind="user-rejected",
            sourceToolAssistantUUID="a" + uuid,
        )

    def att(uuid, second, attachment):
        return dict(_claude_attachment(common, uuid, second, attachment), timestamp=stamp(second))

    return [
        dict(type="permission-mode", permissionMode="default", sessionId=session_id),  # 1
        prompt("u2", "01", "run python3 calc.py 2 3"),  # 2
        call("a3", "02", "toolu_m1", "Bash", {"command": "python3 calc.py 2 3"}),  # 3
        denied("u4", "03", "toolu_m1"),  # 4
        prompt("u5", "04", "Do not run it again."),  # 5
        call("a6", "05", "toolu_m2", "ExitPlanMode", {"plan": "1. add subtract"}),  # 6
        denied("u7", "06", "toolu_m2"),  # 7
        dict(prompt("u8", "07", _claude_command("model")), isMeta=False),  # 8
        dict(
            common,
            type="user",
            uuid="u9",
            timestamp=stamp("07"),
            message={
                "role": "user",
                "content": "<local-command-stdout>Set model to Fable 5.1 and saved as your "
                "default for new sessions</local-command-stdout>",
            },
        ),  # 9
        prompt(
            "u10",
            "10",
            '<pasted_content id="0bb3">\none\ntwo\nthree\nfour\n</pasted_content>'
            " Summarise the pasted text in one sentence.",
        ),  # 10: a short paste, wrapped in the prompt as 2.1.310 writes it
        dict(
            common,
            type="system",
            uuid="s11",
            timestamp=stamp("20"),
            subtype="scheduled_task_fire",
            content="Running scheduled task (Oct 1 11:00am)",
            cron="*/1 * * * *",
            prompt="echo hi",
            taskId="a1b2c3d4",
            level="info",
        ),  # 11
        prompt(
            "u12",
            "20",
            "echo hi",
            promptSource="system",
            turnOrigin="scheduled",
            scheduledFireId="f1",
            scheduledTaskId="a1b2c3d4",
            wakeupSource="cron",
            queuePriority="next",
            queueSkipSlashCommands=True,
            turnCompanion=False,
        ),  # 12
        att(
            "t13",
            "21",
            {
                "type": "auto_mode",
                "autoModeConsentFlow": "default",
                "bashFirst": False,
                "bashFirstSteer": False,
                "bypass": False,
                "finishExposedWork": False,
                "steerOnly": False,
            },
        ),  # 13
        att(
            "t14",
            "21",
            {
                "type": "auto_mode_exit",
                "bashFirst": False,
                "finishExposedWork": False,
                "steerOnly": False,
            },
        ),  # 14
        att("t15", "21", {"type": "command_permissions", "allowedTools": ["Bash(echo:*)"]}),
        # 15
        prompt("u16", "30", _claude_command("rename", "lab-round")),  # 16
        prompt(
            "u17",
            "40",
            '<pasted_content id="a91f">\n'
            + CLAUDE_MAC_LONG_PASTE
            + "\n</pasted_content> Summarise again.",
        ),  # 17: a long paste
        # 18: a second /model, recorded 27 s after its history line, when
        # the picker closed
        prompt("u18", "50", _claude_command("model")),
        dict(
            type="cost-state",
            hasUnknownModelCost=False,
            modelUsage={},
            sessionId=session_id,
            startTime=1790852400000,
            totalCostUSD=0.01,
        ),  # 19
        dict(
            common,
            type="system",
            uuid="s20",
            timestamp=stamp("55"),
            subtype="local_command",
            commandRun="help",
            content=_claude_command("help"),
            level="info",
        ),  # 20: `/help`, a `local_command:` row with empty args
    ]


def claude_mac_cleared_records():
    """The file a `/clear` in `claude_mac_records`'s session opens (2.1.310 macOS, row 24a).

    Its first lines are the `/clear` command in tag form and its output, under the new session's id.
    """
    common = _claude_common("/srv/proj", "main")
    common.update(sessionId=CLAUDE_MAC_CLEARED_SESSION, version="2.1.310", entrypoint="cli")
    return [
        dict(
            _claude_user(common, "k1", "57", _claude_command("clear")),
            timestamp="2026-10-01T11:00:57.000Z",
        ),
        dict(
            _claude_user(common, "k2", "57", "<local-command-stdout></local-command-stdout>"),
            timestamp="2026-10-01T11:00:57.000Z",
        ),
    ]


def claude_mac_history_records():
    """`history.jsonl` lines of the events of `claude_mac_records`.

    The commands as typed, the short paste inline and the long one by hash
    (`paste-cache/<CLAUDE_MAC_PASTE_HASH>.txt`, written by the test), and the three lines no
    transcript record matches: `/clear` under the ended session's id (its record is in
    `claude_mac_cleared_records`' file), `/logout` (the process ends before any line) and `/exit`
    typed while a turn ran (no line written).
    """
    session_id = CLAUDE_MAC_SESSION

    def line(second, display, **pasted):
        return {
            "display": display,
            "pastedContents": pasted,
            # 1 October 11:00:<t> UTC in epoch milliseconds.
            "timestamp": 1790852400000 + int(second) * 1000,
            "project": "/srv/proj",
            "sessionId": session_id,
        }

    return [
        line("01", "run python3 calc.py 2 3"),
        line("07", "/model"),
        line(
            "10",
            "[Pasted text #1 +3 lines] Summarise the pasted text in one sentence.",
            **{"1": {"content": "one\ntwo\nthree\nfour", "id": 1, "type": "text"}},
        ),
        line("23", "/model"),
        line("30", "/rename lab-round"),
        line(
            "40",
            "[Pasted text #2 +4 lines] Summarise again.",
            **{"2": {"contentHash": CLAUDE_MAC_PASTE_HASH, "id": 2, "type": "text"}},
        ),
        line("55", "/help"),
        line("57", "/clear"),
        line("58", "/logout"),
        line("59", "/exit"),
    ]


def claude_mac_spawn_records():
    """A Remote Control spawn's transcript (2.1.310 macOS).

    The prompt from claude.ai arrives through the queue, every line has `entrypoint` sdk-cli, the
    prompt `promptSource` sdk and `permissionMode` default, and the file ends with `cost-state` when
    the bridge stops. Line numbers are what ClaudeCodeMacTests asserts.
    """
    common = _claude_common("/srv/proj", "main")
    common.update(sessionId=CLAUDE_MAC_SPAWN_SESSION, version="2.1.310", entrypoint="sdk-cli")
    session_id = CLAUDE_MAC_SPAWN_SESSION
    text = "Run ls -la in this folder and tell me what files are here."
    start_time = "2026-10-01T12:00:00.000Z"
    return [
        dict(
            type="queue-operation",
            operation="enqueue",
            content=text,
            sessionId=session_id,
            timestamp=start_time,
        ),
        dict(
            type="queue-operation", operation="dequeue", sessionId=session_id, timestamp=start_time
        ),
        dict(
            _claude_user(common, "r3", "01", text),
            timestamp="2026-10-01T12:00:01.000Z",
            origin={"kind": "human"},
            promptSource="sdk",
            turnOrigin="human",
            permissionMode="default",
        ),  # 3
        dict(
            _claude_assistant(
                common,
                "r4",
                "02",
                [
                    {
                        "type": "tool_use",
                        "id": "toolu_r1",
                        "name": "Bash",
                        "input": {"command": "ls -la"},
                    }
                ],
            ),
            timestamp="2026-10-01T12:00:02.000Z",
        ),  # 4
        dict(
            _claude_user(common, "r5", "03", _claude_result("toolu_r1", "calc.py\nREADME.md")),
            timestamp="2026-10-01T12:00:03.000Z",
            permissionDecision={
                "decision": "accept",
                "source": "config",
                "reasonType": "subcommandResults",
            },
        ),  # 5
        dict(type="cost-state", sessionId=session_id, totalCostUSD=0.01),  # 6
    ]


def claude_mac_teleport_records():
    """A teleported cloud session's local file (2.1.310 macOS).

    The cloud turn replayed with `remoteSourced` on its `user` and `assistant` lines, the cloud
    machine's `environment` attachment (its scratchpad under `/tmp/claude-0/-home-user-repo/`), a
    cloud decision by rule (`ruleSource` cliArg), the `stop_hook_summary` and `informational` system
    lines, then the local environment and one local prompt. `entrypoint` is cli throughout. Line
    numbers are what ClaudeCodeMacTests asserts.
    """
    common = _claude_common("/srv/proj", "main")
    common.update(sessionId=CLAUDE_MAC_TELEPORT_SESSION, version="2.1.310", entrypoint="cli")

    def stamp(second):
        return "2026-10-01T13:00:%s.000Z" % second

    def att(uuid, second, scratch):
        return dict(
            _claude_attachment(
                common,
                uuid,
                second,
                {"type": "environment", "snapshot": {"scratchpadDirectory": scratch}},
            ),
            timestamp=stamp(second),
        )

    remote = {"remoteSourced": True}
    return [
        att("e1", "00", CLAUDE_MAC_CLOUD_SCRATCH),  # 1
        dict(
            _claude_user(common, "c2", "01", "Describe this repository in three sentences."),
            timestamp=stamp("01"),
            origin={"kind": "human"},
            promptSource="sdk",
            turnOrigin="human",
            permissionMode="default",
            **remote,
        ),  # 2
        dict(
            _claude_assistant(
                common,
                "c3",
                "02",
                [
                    {
                        "type": "tool_use",
                        "id": "toolu_t1",
                        "name": "Read",
                        "input": {"file_path": "/home/user/repo/README.md"},
                    }
                ],
                model="claude-opus-5-5",
            ),
            timestamp=stamp("02"),
            **remote,
        ),  # 3
        dict(
            _claude_user(common, "c4", "03", _claude_result("toolu_t1", "# logsleuth")),
            timestamp=stamp("03"),
            permissionDecision={
                "decision": "accept",
                "source": "config",
                "reasonType": "rule",
                "ruleSource": "cliArg",
            },
            **remote,
        ),  # 4
        dict(
            common,
            type="system",
            uuid="c5",
            timestamp=stamp("04"),
            subtype="stop_hook_summary",
            hookCount=1,
            hookErrors=[],
            hookInfos=[],
            hookAdditionalContext=[],
            hasOutput=False,
            preventedContinuation=False,
            stopReason="",
            toolUseID="",
            level="info",
        ),  # 5
        dict(
            common,
            type="system",
            uuid="c6",
            timestamp=stamp("05"),
            subtype="informational",
            content="Session resumed",
            level="info",
        ),  # 6
        att("e7", "10", CLAUDE_MAC_LOCAL_SCRATCH),  # 7
        dict(
            _claude_user(common, "l8", "11", "In one sentence, what did you do so far?"),
            timestamp=stamp("11"),
            origin={"kind": "human"},
            promptSource="typed",
            turnOrigin="human",
            permissionMode="auto",
        ),  # 8
    ]


def claude_mac_global_config():
    """`~/.claude.json` of 2.1.310 after Remote Control and a cloud session (macOS).

    The machine id (never printed), the project's spawn mode and `hasRemoteEnvironment`.
    """
    return {
        "remoteControlMachineId": "00000000-0000-4000-8000-0000000000cc",
        "hasRemoteEnvironment": True,
        "projects": {
            "/srv/proj": {
                "hasTrustDialogAccepted": True,
                "remoteControlSpawnMode": "same-dir",
            }
        },
    }


def claude_user_settings():
    """`~/.claude/settings.json` with every `permissions` key the parser reads.

    `~/.claude/settings.json` with every `permissions` key the parser reads, and keys it must never
    print (`env`, `apiKeyHelper`).
    """
    return {
        "permissions": {
            "allow": ["Bash(npm test:*)", "Read(~/notes/**)"],
            "deny": ["Bash(rm -rf:*)"],
            "ask": ["WebFetch"],
            "defaultMode": "acceptEdits",
            "disableBypassPermissionsMode": "disable",
            "additionalDirectories": ["/srv/shared"],
        },
        "env": {"ANTHROPIC_API_KEY": "sk-not-a-real-key"},
        "apiKeyHelper": "/usr/local/bin/print-not-a-real-key",
        "model": "opus",
    }


def claude_project_local_settings():
    """A project's `.claude/settings.local.json`."""
    return {"permissions": {"allow": ["Bash(make lint)"], "defaultMode": "plan"}}


def claude_global_config():
    """`~/.claude.json` with one project entry.

    `~/.claude.json` with one project entry; `oauthAccount` and the MCP server headers must never be
    printed.
    """
    return {
        "numStartups": 3,
        "oauthAccount": {"emailAddress": "user@example.invalid"},
        "projects": {
            "/srv/proj": {
                "allowedTools": ["Bash(git status)"],
                "mcpContextUris": [],
                "mcpServers": {
                    "remote": {"url": "https://mcp.invalid", "headers": {"Authorization": "x"}}
                },
                "enabledMcpjsonServers": [],
                "disabledMcpjsonServers": [],
                "hasTrustDialogAccepted": True,
                "hasClaudeMdExternalIncludesApproved": False,
            }
        },
    }


def claude_registry_records():
    """`.claude/sessions/<pid>.json` (2.1.295 key set, the research document's Other stores).

    A Code-tab process, and a process under Remote Control.
    """
    base = {
        "procStart": "Thu Oct  1 11:00:00 2026",
        "version": "2.1.295",
        "peerProtocol": 1,
        "peerFeatures": ["messaging"],
        "kind": "interactive",
        "entrypoint": "claude-desktop",
        "pidDomain": "linux",
        "name": "cron jobs",
        "nameSource": "user",
        "nameSince": 1790852400000,
        "status": "idle",
        "updatedAt": 1790852404000,
        "statusUpdatedAt": 1790852404000,
    }
    first = dict(
        base,
        pid=4242,
        sessionId=CLAUDE_295_SESSION,
        cwd="/srv/proj",
        startedAt=1790852400000,
        hostSessionId="local_f4f4f4f4-9999-4aaa-8bbb-cccccccccccc",
        messagingSocketPath="/tmp/claude-4242.sock",
    )
    second = dict(
        base,
        pid=4343,
        sessionId=CLAUDE_BRIDGE_SESSION,
        cwd="/srv/proj",
        startedAt=1790856000000,
        entrypoint="cli",
        status="busy",
        messagingSocketPath="/tmp/claude-4343.sock",
        bridgeSessionId="cse_0123456789abcdef",
    )
    return {"4242.json": first, "4343.json": second}


def claude_history_records():
    return [
        {
            "display": "delete the logs in /var/log please",
            "pastedContents": {},
            "timestamp": 1790848800000,
            "project": "/srv/proj",
            "sessionId": CLAUDE_SESSION,
        },
        {
            "display": "an older prompt whose session file is gone",
            "pastedContents": {},
            "timestamp": 1790762400000,
            "project": "/srv/old",
            "sessionId": "99999999-0000-4000-8000-000000000000",
        },
    ]


def build_claude_code(home: Path) -> None:
    """The Claude Code state of a fixture home, and a moved config home beside it."""
    _jsonl(
        home / ".claude/projects/-srv-proj" / (CLAUDE_SESSION + ".jsonl"), claude_session_records()
    )
    _jsonl(
        home / ".claude/projects/-srv-proj" / CLAUDE_SESSION / "subagents" / "agent-abc.jsonl",
        claude_subagent_records(),
    )
    _jsonl(
        home / ".claude/projects/-srv-proj" / CLAUDE_SESSION / "subagents" / "agent-fork1.jsonl",
        claude_fork_records(),
    )
    claude_sidecars = home / ".claude/projects/-srv-proj" / CLAUDE_SESSION
    (claude_sidecars / "subagents/agent-abc.meta.json").write_text(
        json.dumps({"agentType": "general-purpose", "toolUseId": "toolu_x", "spawnDepth": 1}),
        encoding="utf-8",
    )
    (claude_sidecars / "tool-results").mkdir()
    (claude_sidecars / "tool-results" / CLAUDE_TOOL_RESULT).write_text(
        CLAUDE_PERSISTED, encoding="utf-8"
    )
    _jsonl(
        home / ".claude/projects/-srv-proj" / (CLAUDE_295_SESSION + ".jsonl"), claude_295_records()
    )
    registry = home / ".claude/sessions"
    registry.mkdir(parents=True)
    for name, record in claude_registry_records().items():
        (registry / name).write_text(json.dumps(record), encoding="utf-8")
    (registry / CLAUDE_REGISTRY_KEY).write_text("not a real key", encoding="utf-8")
    _jsonl(
        home / ".claude/projects/-srv-proj" / (CLAUDE_296_SESSION + ".jsonl"), claude_296_records()
    )
    _jsonl(
        home / ".claude/projects/-srv-proj" / (CLAUDE_309_SESSION + ".jsonl"), claude_309_records()
    )
    claude_309_results = home / ".claude/projects/-srv-proj" / CLAUDE_309_SESSION / "tool-results"
    claude_309_results.mkdir(parents=True)
    (claude_309_results / CLAUDE_309_TOOL_RESULT).write_text(CLAUDE_309_PERSISTED, encoding="utf-8")
    _jsonl(home / ".claude/history.jsonl", claude_history_records())
    for rel, doc in (
        (".claude/settings.json", claude_user_settings()),
        (".claude.json", claude_global_config()),
        (".claude-work/.claude.json", {"projects": {"/srv/work": {"allowedTools": ["Edit"]}}}),
    ):
        (home / rel).parent.mkdir(parents=True, exist_ok=True)
        (home / rel).write_text(json.dumps(doc), encoding="utf-8")
        os.utime(home / rel, (CLAUDE_SETTINGS_MTIME, CLAUDE_SETTINGS_MTIME))
    # A backup of the global config, which the parser must not want.
    (home / ".claude.json.backup.1790845200000").write_text("{}", encoding="utf-8")
    # A config home moved with CLAUDE_CONFIG_DIR, and claude-code-router's
    # directory, which shares the .claude- prefix and must not be parsed.
    _jsonl(
        home / ".claude-work/projects/-srv-proj" / (CLAUDE_WORK_SESSION + ".jsonl"),
        claude_work_records(),
    )
    _jsonl(home / ".claude-work/history.jsonl", claude_work_history_records())
    (home / ".claude-code-router").mkdir()
    (home / ".claude-code-router/config.json").write_text('{"PORT": 3456}', encoding="utf-8")


def claude_ccr_tip():
    """`ccr-tip.json` of the Remote Control spawn: the last claude.ai event it processed.

    Source: research/claude-code.md#remote-control; build 2.1.310
    """
    return {"eventId": "33333333-4444-4555-8666-777777777777", "updatedAt": 1790856300000}


def claude_bridge_pointer():
    """The project's `bridge-pointer.json`, left by a Remote Control server.

    Source: research/claude-code.md#remote-control; build 2.1.310
    """
    return {
        "environmentId": "env_0123456789abcdefghijkl",
        "pid": 15081,
        "procStart": "Mon Oct  1 12:00:00 2026",
        "sessionId": "0123456789abcdef0123456789abcdef",
        "source": "standalone",
    }


def build_claude_code_remote(home: Path) -> None:
    """The Remote Control spawn and the teleported cloud session, with their sidecars.

    `build_home` leaves them out; the output examples' collection writes them beside the files
    of `build_claude_code`.

    Source: research/claude-code.md#remote-control; build 2.1.310
    """
    project = home / ".claude/projects/-srv-proj"
    _jsonl(project / (CLAUDE_MAC_SPAWN_SESSION + ".jsonl"), claude_mac_spawn_records())
    _jsonl(project / (CLAUDE_MAC_TELEPORT_SESSION + ".jsonl"), claude_mac_teleport_records())
    (project / CLAUDE_MAC_SPAWN_SESSION).mkdir(parents=True, exist_ok=True)
    (project / CLAUDE_MAC_SPAWN_SESSION / "ccr-tip.json").write_text(
        json.dumps(claude_ccr_tip()), encoding="utf-8"
    )
    (project / "bridge-pointer.json").write_text(
        json.dumps(claude_bridge_pointer()), encoding="utf-8"
    )
