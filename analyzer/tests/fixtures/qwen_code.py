"""Synthetic Qwen Code state for the analyzer tests."""

from __future__ import annotations

import json
import os
from pathlib import Path

from .common import _json_file, _jsonl

QWEN_SESSION = "e5f6a7b8-4444-4000-8000-000000000011"
QWEN_ARCHIVED = "0a0b0c0d-4444-4000-8000-000000000022"
QWEN_TEMPORARY = "3f0a9c" + "0" * 58  # sha256(cwd) directory name
QWEN_TRUST_MTIME = 1790856000  # 1 October 12:00:00 UTC, trustedFolders.json's mtime


def qwen_session_records(cwd="/srv/proj", branch="main"):
    """Qwen Code ChatRecords, shapes from research/qwen-code.md, Parser plan."""
    common = dict(sessionId=QWEN_SESSION, cwd=cwd, version="0.24.7", gitBranch=branch)
    return [
        dict(
            common,
            uuid="q0",
            parentUuid=None,
            timestamp="2026-10-01T09:59:59.000Z",
            type="system",
            subtype="session_model",
            provenance="system",
            systemPayload={"modelId": "qwen3-coder-plus", "authType": "qwen-oauth"},
        ),
        dict(
            common,
            uuid="q1",
            parentUuid="q0",
            timestamp="2026-10-01T10:00:00.000Z",
            type="user",
            provenance="real_user",
            promptId=QWEN_SESSION + "########1",
            message={"role": "user", "parts": [{"text": "run the tests"}]},
            systemPayload={"displayText": "run the tests", "hookContext": ""},
        ),
        dict(
            common,
            uuid="q2",
            parentUuid="q1",
            timestamp="2026-10-01T10:00:03.000Z",
            type="assistant",
            provenance="assistant_output",
            model="qwen3-coder-plus",
            message={
                "role": "model",
                "parts": [
                    {"text": "Plan: run npm test.", "thought": True},
                    {"text": "Running tests."},
                    {
                        "functionCall": {
                            "id": "call_abc123",
                            "name": "run_shell_command",
                            "args": {"command": "npm test"},
                        }
                    },
                    {
                        "functionCall": {
                            "id": "call_def456",
                            "name": "read_file",
                            "args": {"absolute_path": "/srv/proj/.env"},
                        }
                    },
                ],
            },
            usageMetadata={
                "promptTokenCount": 200,
                "candidatesTokenCount": 30,
                "totalTokenCount": 230,
            },
            contextWindowSize=131072,
        ),
        dict(
            common,
            uuid="q3",
            parentUuid="q2",
            timestamp="2026-10-01T10:00:09.000Z",
            type="tool_result",
            provenance="tool_result",
            message={
                "role": "user",
                "parts": [
                    {
                        "functionResponse": {
                            "id": "call_abc123",
                            "name": "run_shell_command",
                            "response": {"output": "12 passing"},
                        }
                    }
                ],
            },
            toolCallResult={
                "callId": "call_abc123",
                "status": "success",
                "resultDisplay": "12 passing",
                "errorType": None,
            },
        ),
        dict(
            common,
            uuid="q4",
            parentUuid="q3",
            timestamp="2026-10-01T10:00:10.000Z",
            type="tool_result",
            provenance="tool_result",
            message={
                "role": "user",
                "parts": [
                    {
                        "functionResponse": {
                            "id": "call_def456",
                            "name": "read_file",
                            "response": {"error": "permission denied"},
                        }
                    }
                ],
            },
            toolCallResult={
                "callId": "call_def456",
                "status": "error",
                "errorType": "permission_denied",
            },
        ),
        dict(
            common,
            uuid="q5",
            parentUuid="q4",
            timestamp="2026-10-01T10:00:11.000Z",
            type="system",
            subtype="slash_command",
            provenance="system",
            systemPayload={"phase": "invocation", "rawCommand": "/compress"},
        ),
        dict(
            common,
            uuid="q6",
            parentUuid="q5",
            timestamp="2026-10-01T10:00:12.000Z",
            type="system",
            subtype="custom_title",
            provenance="system",
            systemPayload={"customTitle": "Run tests", "titleSource": "auto"},
        ),
        # A later turn on another model whose tool call the user cancelled.
        dict(
            common,
            uuid="q7",
            parentUuid="q6",
            timestamp="2026-10-01T10:01:00.000Z",
            type="assistant",
            provenance="assistant_output",
            model="qwen3-vl-plus",
            message={
                "role": "model",
                "parts": [
                    {
                        "functionCall": {
                            "id": "call_ghi789",
                            "name": "run_shell_command",
                            "args": {"command": "rm -rf build"},
                        }
                    }
                ],
            },
        ),
        dict(
            common,
            uuid="q8",
            parentUuid="q7",
            timestamp="2026-10-01T10:01:05.000Z",
            type="tool_result",
            provenance="tool_result",
            message={
                "role": "user",
                "parts": [
                    {
                        "functionResponse": {
                            "id": "call_ghi789",
                            "name": "run_shell_command",
                            "response": {"error": "cancelled by user"},
                        }
                    }
                ],
            },
            toolCallResult={"callId": "call_ghi789", "status": "cancelled"},
        ),
    ]


def qwen_logs_entries():
    """Gemini-legacy LogEntry array; model_switch carries a JSON string."""
    return [
        {
            "sessionId": QWEN_SESSION,
            "messageId": 0,
            "timestamp": "2026-10-01T10:00:00.000Z",
            "type": "user",
            "message": "run the tests",
        },
        {
            "sessionId": QWEN_SESSION,
            "messageId": 1,
            "timestamp": "2026-10-01T10:00:05.000Z",
            "type": "model_switch",
            "message": json.dumps(
                {
                    "fromModel": "qwen3-coder-plus",
                    "toModel": "qwen3-vl-plus",
                    "reason": "vision_auto_switch",
                }
            ),
        },
    ]


def build_qwen(home: Path) -> None:
    chats = home / ".qwen/projects/-srv-proj/chats"
    _jsonl(chats / (QWEN_SESSION + ".jsonl"), qwen_session_records())
    _jsonl(
        chats / "archive" / (QWEN_ARCHIVED + ".jsonl"),
        [dict(record, sessionId=QWEN_ARCHIVED) for record in qwen_session_records()[1:2]],
    )
    _jsonl(chats / (QWEN_SESSION + ".ledger.jsonl"), [{"promptId": "p1", "text": "run the tests"}])
    (chats / (QWEN_SESSION + ".runtime.json")).write_text(
        '{"session_id": "x", "started_at": 1790848800}', encoding="utf-8"
    )
    temporary = home / ".qwen/tmp" / QWEN_TEMPORARY
    temporary.mkdir(parents=True, exist_ok=True)
    (temporary / "logs.json").write_text(
        json.dumps(qwen_logs_entries(), indent=2), encoding="utf-8"
    )
    (home / ".qwen/settings.json").write_text("{}", encoding="utf-8")
    (home / ".qwen/oauth_creds.json").write_text('{"access_token": "secret"}', encoding="utf-8")
    # JSONC, as Qwen Code reads it: keys are folders, values TrustLevel strings.
    trust = home / ".qwen/trustedFolders.json"
    trust.write_text(
        '{\n  // trust dialog\n  "/srv/proj": "TRUST_FOLDER",\n  "/srv": "DO_NOT_TRUST"\n}\n',
        encoding="utf-8",
    )
    os.utime(trust, (QWEN_TRUST_MTIME, QWEN_TRUST_MTIME))


# Channel routes, the `PersistedEntry` shape of
# packages/channels/base/src/SessionRouter.ts:25-36 at 2c591ecc:
# QWEN_SESSION was driven from a Telegram chat; QWEN_DAEMON_SESSION by the
# daemon from a GitHub thread, with no `startedAt`.
QWEN_DAEMON_SESSION = "e5f6a7b8-4444-4000-8000-000000000033"
QWEN_DAEMON_HASH = "9c1d" + "0" * 12
QWEN_ROUTES_MTIME = 1791367200  # 7 October 10:00:00 UTC


def build_qwen_channels(home: Path) -> None:
    channels = home / ".qwen/channels"
    _json_file(
        channels / "sessions.json",
        {
            "telegram:-1001234": {
                "sessionId": QWEN_SESSION,
                "target": {"channelName": "telegram", "senderId": "42", "chatId": "-1001234"},
                "cwd": "/srv/proj",
                "turns": 3,
                "startedAt": 1790848800000,
            }
        },
    )
    routes = channels / "daemon" / QWEN_DAEMON_HASH / "routes.json"
    _json_file(
        routes,
        {
            "github:octo/proj#7": {
                "sessionId": QWEN_DAEMON_SESSION,
                "target": {
                    "channelName": "github",
                    "messageRoute": "issue_comment",
                    "senderId": "octocat",
                    "chatId": "octo/proj#7",
                    "threadId": "c-991",
                    "isGroup": True,
                },
                "cwd": "/srv/proj",
            }
        },
    )
    os.utime(routes, (QWEN_ROUTES_MTIME, QWEN_ROUTES_MTIME))
    # A half-written temporary file of the router, which the parser must not want.
    (channels / "1791367200000-4242-ab12.tmp").write_text("{", encoding="utf-8")


# Injected `type: "user"` records, shapes from QwenLM/qwen-code at
# 5dcf7c7cb162e825d95d531081f67ee60da3c0e1 (0.25.3): createNotificationRecord
# (cron, notification; provenance "system") and recordExternalAgentRecordStrict
# (agent_mention, agent_message; provenance "external_agent", or "real_user"
# for the user's own mention) in
# packages/core/src/services/chatRecordingService.ts:2541-2585 and 2655-2690,
# payloads in packages/core/src/agents/session-agents/contract.ts. Not wired
# into build_home, so the shared fixture's session counts stay as they are;
# the tests that need it call build_qwen_injected.
QWEN_INJECTED_SESSION = "e5f6a7b8-4444-4000-8000-000000000044"
QWEN_INJECTED_REL = ".qwen/projects/-srv-proj/chats/%s.jsonl" % QWEN_INJECTED_SESSION


def qwen_injected_records():
    """Cron, notification, a user's and an agent's mention, an agent's reply."""
    common = dict(
        sessionId=QWEN_INJECTED_SESSION,
        cwd="/srv/proj",
        version="0.25.3",
        gitBranch="main",
        type="user",
    )
    reviewer = {"agentId": "agt-reviewer", "name": "Reviewer", "program": "claude"}
    return [
        dict(
            common,
            uuid="i1",
            parentUuid=None,
            timestamp="2026-10-02T09:00:00.000Z",
            subtype="cron",
            provenance="system",
            message={"role": "user", "parts": [{"text": "check the nightly build"}]},
        ),
        dict(
            common,
            uuid="i2",
            parentUuid="i1",
            timestamp="2026-10-02T09:01:00.000Z",
            subtype="notification",
            provenance="system",
            message={"role": "user", "parts": []},
            systemPayload={
                "displayText": "Background shell finished",
                "backgroundTask": {"taskId": "bg-1", "status": "completed", "kind": "shell"},
            },
        ),
        dict(
            common,
            uuid="i3",
            parentUuid="i2",
            timestamp="2026-10-02T09:02:00.000Z",
            subtype="agent_mention",
            provenance="real_user",
            message={"role": "user", "parts": [{"text": "@Reviewer look at the diff"}]},
            systemPayload={
                "displayText": "@Reviewer look at the diff",
                "mentionedAgentIds": ["agt-reviewer"],
            },
            externalRecordKey="k1",
        ),
        dict(
            common,
            uuid="i4",
            parentUuid="i3",
            timestamp="2026-10-02T09:03:00.000Z",
            subtype="agent_message",
            provenance="external_agent",
            message={"role": "user", "parts": [{"text": "The diff deletes the tests."}]},
            systemPayload={
                "displayText": "The diff deletes the tests.",
                "author": reviewer,
                "runId": "run-1",
                "status": "completed",
            },
            externalRecordKey="k2",
            agentId="agt-reviewer",
            agentName="Reviewer",
        ),
        dict(
            common,
            uuid="i5",
            parentUuid="i4",
            timestamp="2026-10-02T09:04:00.000Z",
            subtype="agent_mention",
            provenance="external_agent",
            message={"role": "user", "parts": [{"text": "@Fixer restore the tests"}]},
            systemPayload={
                "displayText": "@Fixer restore the tests",
                "mentionedAgentIds": ["agt-fixer"],
                "author": reviewer,
            },
            externalRecordKey="k3",
        ),
    ]


def build_qwen_injected(home: Path) -> None:
    _jsonl(home / QWEN_INJECTED_REL, qwen_injected_records())


# Permission, approval-mode and usage records, shapes from
# QwenLM/qwen-code at 5dcf7c7cb162e825d95d531081f67ee60da3c0e1 (0.25.3):
# ChatRecord.executionContext (chatRecordingService.ts:475-479),
# SessionApprovalModeRecordPayload (749-754), the `ui_telemetry` record of
# logToolCall (telemetry/loggers.ts:317-331, event name
# telemetry/constants.ts:11), settings.json `permissions` and
# `tools.approvalMode` (cli/src/config/settingsSchema.ts) and
# UsageSummaryRecord (services/usageHistoryService.ts:37-77). Built only by
# the tests that need it, as build_qwen_injected is.
QWEN_PERM_SESSION = "e5f6a7b8-4444-4000-8000-000000000044"
QWEN_DELETED_SESSION = "e5f6a7b8-4444-4000-8000-000000000055"
QWEN_PERM_REL = ".qwen/projects/-srv-proj/chats/%s.jsonl" % QWEN_PERM_SESSION
QWEN_USAGE_REL = ".qwen/usage_record.jsonl"
# A session whose ui_telemetry tool-call event carries the ToolCallEvent
# class's bare default name `tool_call` (telemetry/types.ts:214) instead of
# the `qwen-code.tool_call` that logToolCall records at 5dcf7c7.
QWEN_BARE_SESSION = "e5f6a7b8-4444-4000-8000-000000000066"
QWEN_BARE_REL = ".qwen/projects/-srv-proj/chats/%s.jsonl" % QWEN_BARE_SESSION
QWEN_SETTINGS_MTIME = 1790863200  # 1 October 14:00:00 UTC


def qwen_permission_records():
    common = dict(sessionId=QWEN_PERM_SESSION, cwd="/srv/proj", version="0.25.3", gitBranch="main")
    context = {"modelId": "qwen3-coder-plus", "authType": "qwen-oauth", "approvalMode": "default"}
    return [
        dict(
            common,
            uuid="p1",
            parentUuid=None,
            timestamp="2026-10-03T09:00:00.000Z",
            type="user",
            provenance="real_user",
            executionContext=context,
            message={"role": "user", "parts": [{"text": "clean the build"}]},
        ),
        dict(
            common,
            uuid="p2",
            parentUuid="p1",
            timestamp="2026-10-03T09:00:02.000Z",
            type="assistant",
            model="qwen3-coder-plus",
            message={
                "role": "model",
                "parts": [
                    {
                        "functionCall": {
                            "id": "call_p1",
                            "name": "run_shell_command",
                            "args": {"command": "rm -rf build"},
                        }
                    }
                ],
            },
        ),
        dict(
            common,
            uuid="p3",
            parentUuid="p2",
            timestamp="2026-10-03T09:00:09.000Z",
            type="system",
            subtype="ui_telemetry",
            systemPayload={
                "uiEvent": {
                    "event.name": "qwen-code.tool_call",
                    "event.timestamp": "2026-10-03T09:00:09.000Z",
                    "call_id": "call_p1",
                    "function_name": "run_shell_command",
                    "status": "success",
                    "decision": "auto_accept",
                    "prompt_id": "p#1",
                }
            },
        ),
        # The same mode again: no row; then a daemon mode change.
        dict(
            common,
            uuid="p4",
            parentUuid="p3",
            timestamp="2026-10-03T09:01:00.000Z",
            type="system",
            subtype="session_approval_mode",
            systemPayload={"mode": "plan", "prePlanMode": "default"},
        ),
        dict(
            common,
            uuid="p5",
            parentUuid="p4",
            timestamp="2026-10-03T09:02:00.000Z",
            type="user",
            provenance="real_user",
            executionContext=dict(context, modelId="qwen3-max", approvalMode="yolo"),
            message={"role": "user", "parts": [{"text": "now ship it"}]},
        ),
    ]


def qwen_usage_records():
    return [
        {
            "version": 1,
            "sessionId": QWEN_DELETED_SESSION,
            "timestamp": 1790935200000,
            "startTime": 1790931600000,
            "project": "/srv/proj",
            "durationMs": 3600000,
            "models": {
                "qwen3-coder-plus": {
                    "requests": 4,
                    "inputTokens": 100,
                    "outputTokens": 20,
                    "cachedTokens": 0,
                    "thoughtsTokens": 0,
                    "totalTokens": 120,
                }
            },
            "tools": {
                "totalCalls": 3,
                "totalSuccess": 3,
                "totalFail": 0,
                "byName": {
                    "run_shell_command": {"count": 2, "success": 2, "fail": 0},
                    "read_file": {"count": 1, "success": 1, "fail": 0},
                },
            },
            "files": {"linesAdded": 4, "linesRemoved": 1},
        }
    ]


def qwen_bare_event_records():
    common = dict(sessionId=QWEN_BARE_SESSION, cwd="/srv/proj", version="0.25.3")
    return [
        dict(
            common,
            uuid="b1",
            parentUuid=None,
            timestamp="2026-10-03T10:00:00.000Z",
            type="assistant",
            model="qwen3-coder-plus",
            message={
                "role": "model",
                "parts": [
                    {
                        "functionCall": {
                            "id": "call_b1",
                            "name": "write_file",
                            "args": {"file_path": "/srv/proj/a.txt"},
                        }
                    }
                ],
            },
        ),
        dict(
            common,
            uuid="b2",
            parentUuid="b1",
            timestamp="2026-10-03T10:00:05.000Z",
            type="system",
            subtype="ui_telemetry",
            systemPayload={
                "uiEvent": {
                    "event.name": "tool_call",
                    "event.timestamp": "2026-10-03T10:00:05.000Z",
                    "call_id": "call_b1",
                    "function_name": "write_file",
                    "status": "cancelled",
                    "decision": "reject",
                    "prompt_id": "b#1",
                }
            },
        ),
    ]


def build_qwen_permissions(home: Path) -> None:
    _jsonl(home / QWEN_PERM_REL, qwen_permission_records())
    _jsonl(home / QWEN_BARE_REL, qwen_bare_event_records())
    _jsonl(home / QWEN_USAGE_REL, qwen_usage_records())
    settings = home / ".qwen/settings.json"
    settings.write_text(
        "\ufeff{\n  // written by an allow-always answer\n"
        '  "permissions": {"allow": ["Bash(git *)", "ReadFileTool"], "deny": ["Bash(rm *)"],\n'
        '                  "autoMode": {"hints": []}},\n'
        '  "tools": {"approvalMode": "auto-edit"},\n'
        '  "modelProviders": [{"apiKey": "secret"}]\n}\n',
        encoding="utf-8",
    )
    os.utime(settings, (QWEN_SETTINGS_MTIME, QWEN_SETTINGS_MTIME))


# A trustedFolders.json that Qwen Code's `parseJsoncObject` accepts and
# `json.loads` does not: a byte-order mark, a comment and trailing
# commas (QwenLM/qwen-code at 5dcf7c7cb162e825d95d531081f67ee60da3c0e1,
# packages/cli/src/utils/jsonc-editor.ts lines 24 and 92-107). The comma
# inside the string value must survive.
TRUST_JSONC = (
    b"\xef\xbb\xbf{\n  // trust dialog\n"
    b'  "/srv/a,]": "TRUST_FOLDER",\n'
    b'  "/srv/b": "DO_NOT_TRUST", /* last */\n'
    b"}\n"
)
