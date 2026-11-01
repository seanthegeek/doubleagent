"""Synthetic Qwen Code state for the analyzer tests.

The records follow the Record schema and Other stores of research/qwen-code.md, from
QwenLM/qwen-code at 835b080 (0.26.4); the session and permission records keep the `version` of
the 0.24.7 and 0.25.3 builds whose shapes are unchanged (Format versions).

Paths: `/srv/proj/.qwen/worktrees/fix-tests`, `/srv`, `/srv/a,]`, `/srv/b`
"""

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
    """Return a session's ChatRecords: a model binding, a turn with two tool calls and a cancel.

    Source: research/qwen-code.md#chatrecord; source 835b080
    """
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
                            "args": {"file_path": "/srv/proj/.env"},
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
    """Return `logs.json` entries: a prompt and a `model_switch` whose message is a JSON string.

    Source: research/qwen-code.md#logsjson; source 835b080
    """
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


QWEN_WORKTREE = "/srv/proj/.qwen/worktrees/fix-tests"


def qwen_worktree_sidecar():
    """Return the `.worktree.json` sidecar of a session in an `enter_worktree` checkout.

    Source: research/qwen-code.md#worktree-sidecar; source 835b080
    """
    return {
        "slug": "fix-tests",
        "worktreePath": QWEN_WORKTREE,
        "worktreeBranch": "worktree-fix-tests",
        "originalCwd": "/srv/proj",
        "originalBranch": "main",
        "originalHeadCommit": "",
    }


def build_qwen(home: Path) -> None:
    """Write a session, an archived session, their sidecars, the prompt log and settings.

    Source: research/qwen-code.md#transcript-stores; source 835b080
    """
    chats = home / ".qwen/projects/-srv-proj/chats"
    _jsonl(chats / (QWEN_SESSION + ".jsonl"), qwen_session_records())
    _jsonl(
        chats / "archive" / (QWEN_ARCHIVED + ".jsonl"),
        [dict(record, sessionId=QWEN_ARCHIVED) for record in qwen_session_records()[1:2]],
    )
    _jsonl(chats / (QWEN_SESSION + ".ledger.jsonl"), [{"promptId": "p1", "text": "run the tests"}])
    # The session works in an enter_worktree checkout; the archived session has no sidecar.
    _json_file(chats / (QWEN_SESSION + ".worktree.json"), qwen_worktree_sidecar())
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


# QWEN_SESSION was driven from a Telegram chat; QWEN_DAEMON_SESSION by the
# daemon from a GitHub thread, with no `startedAt`.
QWEN_DAEMON_SESSION = "e5f6a7b8-4444-4000-8000-000000000033"
QWEN_DAEMON_HASH = "9c1d" + "0" * 12
QWEN_ROUTES_MTIME = 1791367200  # 7 October 10:00:00 UTC


def build_qwen_channels(home: Path) -> None:
    """Write the foreground service's and the daemon's channel route files.

    Source: research/qwen-code.md#channel-routes; source 835b080
    """
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


# Injected `type: "user"` records. Not wired into build_home, so the shared
# fixture's session counts stay as they are; the tests that need it call
# build_qwen_injected.
QWEN_INJECTED_SESSION = "e5f6a7b8-4444-4000-8000-000000000044"
QWEN_INJECTED_REL = ".qwen/projects/-srv-proj/chats/%s.jsonl" % QWEN_INJECTED_SESSION


def qwen_injected_records():
    """Return cron, notification, a user's and an agent's mention, and an agent's reply.

    Source: research/qwen-code.md#system-payloads; source 835b080
    """
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
    """Write the injected-records session.

    Source: research/qwen-code.md#system-payloads; source 835b080
    """
    _jsonl(home / QWEN_INJECTED_REL, qwen_injected_records())


# Permission, approval-mode and usage records, built only by the tests that
# need them, as build_qwen_injected is.
QWEN_PERM_SESSION = "e5f6a7b8-4444-4000-8000-000000000044"
QWEN_DELETED_SESSION = "e5f6a7b8-4444-4000-8000-000000000055"
QWEN_PERM_REL = ".qwen/projects/-srv-proj/chats/%s.jsonl" % QWEN_PERM_SESSION
QWEN_USAGE_REL = ".qwen/usage_record.jsonl"
# A session whose ui_telemetry tool-call event carries the ToolCallEvent
# class's bare default name `tool_call` instead of `qwen-code.tool_call`.
QWEN_BARE_SESSION = "e5f6a7b8-4444-4000-8000-000000000066"
QWEN_BARE_REL = ".qwen/projects/-srv-proj/chats/%s.jsonl" % QWEN_BARE_SESSION
QWEN_SETTINGS_MTIME = 1790863200  # 1 October 14:00:00 UTC


def qwen_permission_records():
    """Return prompts with `executionContext`, a tool-call decision and an approval-mode change.

    Source: research/qwen-code.md#permission-records; source 835b080
    """
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
    """Return one `usage_record.jsonl` summary, of a session no transcript holds.

    Source: research/qwen-code.md#usage_recordjsonl; source 835b080
    """
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
    """Return a tool call whose telemetry event has the bare `tool_call` name.

    Source: research/qwen-code.md#format-versions; source 835b080
    """
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
    """Write the permission, bare-event and usage records and a settings.json with rules.

    Source: research/qwen-code.md#permission-lists-and-trusted-folders; source 835b080
    """
    _jsonl(home / QWEN_PERM_REL, qwen_permission_records())
    _jsonl(home / QWEN_BARE_REL, qwen_bare_event_records())
    _jsonl(home / QWEN_USAGE_REL, qwen_usage_records())
    settings = home / ".qwen/settings.json"
    settings.write_text(
        "\ufeff{\n  // written by an allow-always answer\n"
        '  "permissions": {"allow": ["Bash(git *)", "ReadFileTool"], "deny": ["Bash(rm *)"],\n'
        '                  "autoMode": {"hints": []}},\n'
        '  "tools": {"approvalMode": "auto-edit"},\n'
        '  "security": {"auth": {"apiKey": "secret"}}\n}\n',
        encoding="utf-8",
    )
    os.utime(settings, (QWEN_SETTINGS_MTIME, QWEN_SETTINGS_MTIME))


# A trustedFolders.json that Qwen Code's `parseJsoncObject` accepts and
# `json.loads` does not: a byte-order mark, a comment and trailing
# commas. The comma inside the string value must survive.
TRUST_JSONC = (
    b"\xef\xbb\xbf{\n  // trust dialog\n"
    b'  "/srv/a,]": "TRUST_FOLDER",\n'
    b'  "/srv/b": "DO_NOT_TRUST", /* last */\n'
    b"}\n"
)


# The past-use stores beside the transcripts. QWEN_GROUP_SESSION is a
# Telegram group's session whose second prompt quotes the group messages the
# channel kept; the managed local mode's harness and worker each hold a
# transcript of their own.
QWEN_GROUP_SESSION = "e5f6a7b8-4444-4000-8000-000000000077"
QWEN_HARNESS_SESSION = "e5f6a7b8-4444-4000-8000-000000000088"
QWEN_WORKER_SESSION = "e5f6a7b8-4444-4000-8000-000000000099"
QWEN_TOKEN_USAGE_REL = ".qwen/usage/token-usage-2026-10.jsonl"
QWEN_GROUP_HISTORY_REL = ".qwen/channels/telegram-group-history.jsonl"
QWEN_GROUP_REL = ".qwen/projects/-srv-proj/chats/%s.jsonl" % QWEN_GROUP_SESSION
QWEN_TODO_REL = ".qwen/todos/%s.json" % QWEN_SESSION
QWEN_TODO_DEFAULT_REL = ".qwen/todos/default.json"
QWEN_ORGANIZATION_REL = ".qwen/projects/-srv-proj/session-organization.v1.json"
QWEN_PR_REL = ".qwen/projects/-srv-proj/chats/%s.pr.json" % QWEN_SESSION
QWEN_AUDIT_REL = ".qwen/channels/proj-0123456789ab/github-0123456789abcdef-github-audit.jsonl"
QWEN_SUMMARY_REL = ".qwen/PROJECT_SUMMARY.md"
QWEN_HARNESS_REL = ".qwen/managed-local/harness/projects/-srv-proj/chats/%s.jsonl" % (
    QWEN_HARNESS_SESSION
)
QWEN_WORKER_REL = ".qwen/managed-local/worker-home/projects/-srv-proj/chats/%s.jsonl" % (
    QWEN_WORKER_SESSION
)
QWEN_PAST_USE_MTIME = 1791453600  # 8 October 10:00:00 UTC
# 3 October 08:00:00, 08:05:00 and 08:20:00 UTC in epoch milliseconds.
QWEN_GROUP_TIMES = (1791014400000, 1791014700000, 1791015600000)


def qwen_token_usage_records():
    """Return two token usage records of one session, one with a model and one without.

    Source: research/qwen-code.md#token-usage-log; source 835b080
    """
    common = dict(schemaVersion=1, localDate="2026-10-01", localMonth="2026-10")
    return [
        dict(
            common,
            id="u-1",
            timestamp="2026-10-01T10:00:03.000Z",
            sessionId=QWEN_SESSION,
            model="qwen3-coder-plus",
            authType="qwen-oauth",
            source="main",
            inputTokens=200,
            outputTokens=30,
            cachedTokens=0,
            thoughtsTokens=12,
            totalTokens=242,
            apiDurationMs=2900,
        ),
        dict(
            common,
            id="u-2",
            timestamp="2026-10-01T10:01:00.000Z",
            sessionId=QWEN_SESSION,
            model="unknown",
            authType="unknown",
            source="main",
            inputTokens=0,
            outputTokens=0,
            cachedTokens=0,
            thoughtsTokens=0,
            totalTokens=0,
            apiDurationMs=0,
        ),
    ]


def qwen_group_history_records():
    """Return a group history: two messages, the clear a prompt's drain writes, and a third.

    The first message has a sender name and a message id, the second neither;
    the third arrived after the drain and is still waiting.

    Source: research/qwen-code.md#channel-state-files; source 835b080
    """
    key = json.dumps(["telegram", "-1001234", None])
    first, second, third = QWEN_GROUP_TIMES
    entry = {"senderId": "42", "senderName": "Bob"}
    return [
        {
            "type": "message",
            "key": key,
            "limit": 20,
            "entry": dict(entry, text="the deploy is broken", messageId="m-1", timestamp=first),
            "recordedAt": first + 5,
        },
        {
            "type": "message",
            "key": key,
            "limit": 20,
            "entry": {"senderId": "43", "senderName": "", "text": "rollback?", "timestamp": second},
            "recordedAt": second + 5,
        },
        {"type": "clear", "key": key, "recordedAt": second + 60000},
        {
            "type": "message",
            "key": key,
            "limit": 20,
            "entry": dict(entry, text="fixed now", messageId="m-3", timestamp=third),
            "recordedAt": third + 5,
        },
    ]


def qwen_group_session_records():
    """Return a channel session: a prompt with no quote, then one quoting two group messages.

    Source: research/qwen-code.md#channel-state-files; source 835b080
    """
    common = dict(
        sessionId=QWEN_GROUP_SESSION,
        cwd="/srv/proj",
        version="0.26.4",
        gitBranch="main",
        type="user",
        provenance="real_user",
    )
    quoted = (
        "[Chat messages since your last reply - for context]\n"
        "- [Bob] the deploy is broken\n"
        "- [43] rollback?\n"
        "\n"
        "[Current message - respond to this]\n"
        "@bot what happened?"
    )
    return [
        dict(
            common,
            uuid="g1",
            parentUuid=None,
            timestamp="2026-10-03T07:59:00.000Z",
            message={"role": "user", "parts": [{"text": "hello"}]},
        ),
        dict(
            common,
            uuid="g2",
            parentUuid="g1",
            timestamp="2026-10-03T08:06:00.500Z",
            message={"role": "user", "parts": [{"text": quoted}]},
        ),
    ]


def qwen_todo_file():
    """Return a session's todo file with a plan id and one item blocked by another.

    Source: research/qwen-code.md#todo-files; source 835b080
    """
    return {
        "planId": "plan-1",
        "todos": [
            {"id": "t1", "content": "Run the test suite", "status": "completed"},
            {"id": "t2", "content": "Fix the failing test", "status": "in_progress"},
            {
                "id": "t3",
                "content": "Open a pull request",
                "status": "pending",
                "blockedBy": ["t2"],
            },
        ],
        "sessionId": QWEN_SESSION,
    }


def qwen_session_organization():
    """Return the Web Shell's grouping file: one group, a pinned session and an ungrouped one.

    Source: research/qwen-code.md#session-organization; source 835b080
    """
    return {
        "schemaVersion": 1,
        "groups": [
            {
                "id": "grp-1",
                "name": "Release",
                "color": "blue",
                "order": 0,
                "createdAt": "2026-10-01T11:00:00.000Z",
                "updatedAt": "2026-10-01T11:00:00.000Z",
            }
        ],
        "sessions": {
            QWEN_SESSION: {
                "groupId": "grp-1",
                "color": "green",
                "pinnedAt": "2026-10-01T11:05:00.000Z",
                "updatedAt": "2026-10-01T11:05:00.000Z",
            },
            QWEN_ARCHIVED: {"groupId": None, "updatedAt": "2026-10-01T11:06:00.000Z"},
        },
    }


def qwen_session_pull_requests():
    """Return a session's `.pr.json`: a created pull request closing an issue, and a bare one.

    Source: research/qwen-code.md#source-host-rows; source 835b080
    """
    return {
        "prs": [
            {
                "number": 12,
                "url": "https://github.com/alice/proj/pull/12",
                "createdAt": "2026-10-01T10:30:00.000Z",
                "state": "merged",
                "source": "create",
                "issues": [
                    {
                        "number": 7,
                        "url": "https://github.com/alice/proj/issues/7",
                        "state": "completed",
                    }
                ],
            },
            {
                "number": 3,
                "url": "https://github.com/upstream/proj/pull/3",
                "createdAt": "2026-10-01T10:40:00.000Z",
            },
        ]
    }


def qwen_github_audit_records():
    """Return the GitHub channel's publication audit: a posted reply and a failed one.

    Source: research/qwen-code.md#source-host-rows; source 835b080
    """
    common = dict(
        type="github_publication",
        channel="github",
        repository="alice/proj",
        sessionId=QWEN_DAEMON_SESSION,
        bodySha256="0" * 64,
    )
    return [
        dict(
            common,
            at="2026-10-07T10:00:05.000Z",
            outcome="posted",
            triggerKind="issue_comment",
            number=7,
            sourceMessageId="c-991",
            actor="octocat",
            threadId="c-991",
            commentId=5001,
            commentUrl="https://github.com/alice/proj/issues/7#issuecomment-5001",
            bodyChars=120,
        ),
        dict(
            common,
            at="2026-10-07T10:02:00.000Z",
            outcome="failed",
            pendingId="p-2",
            failurePhase="delivery",
            failureError="HTTP 403",
            bodyChars=80,
        ),
    ]


def qwen_managed_records(session_id):
    """Return one prompt of a managed local mode session, the harness's or a worker's.

    Source: research/qwen-code.md#managed-local-mode; source 835b080
    """
    return [
        dict(
            uuid="m1",
            parentUuid=None,
            sessionId=session_id,
            timestamp="2026-10-04T09:00:00.000Z",
            type="user",
            provenance="real_user",
            cwd="/srv/proj",
            version="0.26.4",
            message={"role": "user", "parts": [{"text": "summarise the repository"}]},
        )
    ]


def build_qwen_past_use(home: Path) -> None:
    """Write the token usage log, group history, todos, grouping, pull requests and audit.

    Also the project summary, the managed local transcripts, and files the
    parser must not want: the managed H2 database and a GitHub queue.

    Source: research/qwen-code.md#other-stores; source 835b080
    """
    _jsonl(home / QWEN_TOKEN_USAGE_REL, qwen_token_usage_records())
    _jsonl(home / QWEN_GROUP_HISTORY_REL, qwen_group_history_records())
    _jsonl(home / QWEN_GROUP_REL, qwen_group_session_records())
    _json_file(home / QWEN_TODO_REL, qwen_todo_file())
    _json_file(home / QWEN_TODO_DEFAULT_REL, {"todos": [], "sessionId": "default"})
    _json_file(home / QWEN_ORGANIZATION_REL, qwen_session_organization())
    _json_file(home / QWEN_PR_REL, qwen_session_pull_requests())
    _jsonl(home / QWEN_AUDIT_REL, qwen_github_audit_records())
    queue = (home / QWEN_AUDIT_REL).with_name("github-0123456789abcdef-github-inbound-tasks.json")
    queue.write_text("[]", encoding="utf-8")
    (home / QWEN_SUMMARY_REL).write_text("# Project summary\n\n- tests fixed\n", encoding="utf-8")
    _jsonl(home / QWEN_HARNESS_REL, qwen_managed_records(QWEN_HARNESS_SESSION))
    _jsonl(home / QWEN_WORKER_REL, qwen_managed_records(QWEN_WORKER_SESSION))
    database = home / ".qwen/managed-local/database/managed.mv.db"
    database.parent.mkdir(parents=True, exist_ok=True)
    database.write_bytes(b"H:2,block:2,blockSize:1000\n" + b"\0" * 32)
    for rel in (QWEN_TODO_REL, QWEN_TODO_DEFAULT_REL, QWEN_SUMMARY_REL):
        os.utime(home / rel, (QWEN_PAST_USE_MTIME, QWEN_PAST_USE_MTIME))
