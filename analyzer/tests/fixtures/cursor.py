"""Synthetic Cursor state for the analyzer tests."""

from __future__ import annotations

import base64
import datetime
import json
import os
import sqlite3
from pathlib import Path

from .common import _json_file, _jsonl, write_bad_line

# Cursor IDE (research/cursor.md, Fixtures). Composer ids are
# invented UUIDs: a live conversation with the 3.25.9 inline header list,
# an archived one with 3.26.13 header chunks, a deleted one that only its
# header row, an agentKv blob, its transcript and ai-tracking row still
# name, a draft with an unsent message and a header-only draft. Two cloud
# agents (`bc-<uuid>`, Windows 3.26.13): one with a cached
# conversation in conversation-search.db, one in the agent list only.
CURSOR_LIVE = "c0000001-0000-4000-8000-000000000001"
CURSOR_CLOUD = "bc-e0000001-0000-4000-8000-000000000001"
CURSOR_CLOUD_LISTED = "bc-e0000002-0000-4000-8000-000000000002"
CURSOR_CLOUD_REPO = "github.com/alice-gh/demo"
CURSOR_CLOUD_BODY = (
    "Describe this repository, change nothing, open no pull request.\n"
    "The repository holds calc.py, a small calculator module.\n"
    "  It has no tests."
)
CURSOR_ARCHIVED = "c0000002-0000-4000-8000-000000000002"
CURSOR_DELETED = "c0000003-0000-4000-8000-000000000003"
CURSOR_DRAFT = "d0000001-0000-4000-8000-000000000001"
CURSOR_EMPTY_HEAD = "d0000002-0000-4000-8000-000000000002"
CURSOR_WORKSPACE = "0123456789abcdef0123456789abcdef"
CURSOR_T0 = 1790848800000  # 1 October 10:00:00 UTC, epoch ms
CURSOR_MTIME = 1790863200  # 1 October 14:00:00 UTC, the settings files' mtime
CURSOR_STATE_REL = "Library/Application Support/Cursor/User/globalStorage/state.vscdb"
CURSOR_SEARCH_REL = "Library/Application Support/Cursor/User/globalStorage/conversation-search.db"
CURSOR_TRANSCRIPT_REL = ".cursor/projects/srv-proj/agent-transcripts/%s/%s.jsonl" % (
    CURSOR_LIVE,
    CURSOR_LIVE,
)
CURSOR_DELETED_TRANSCRIPT_REL = ".cursor/projects/srv-proj/agent-transcripts/%s/%s.jsonl" % (
    CURSOR_DELETED,
    CURSOR_DELETED,
)
CURSOR_TRACKING_REL = ".cursor/ai-tracking/ai-code-tracking.db"
CURSOR_SETTINGS_KEY = (
    "src.vs.platform.reactivestorage.browser.reactiveStorageServiceImpl"
    ".persistentStorage.applicationUser"
)
CURSOR_STATE_SCHEMA = """
CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB);
CREATE TABLE cursorDiskKV (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB);
CREATE TABLE composerHeaders (composerId TEXT PRIMARY KEY, workspaceId TEXT, createdAt INTEGER,
  lastUpdatedAt INTEGER, isArchived INTEGER, isSubagent INTEGER, recency INTEGER,
  checkpointAt INTEGER, subagentTypeName TEXT, value TEXT);
"""


def _cursor_iso(offset_ms: int) -> str:
    """Return the ISO 8601 time `offset_ms` after `CURSOR_T0`.

    ISO 8601 with milliseconds and a Z, as bubbles and header entries write `createdAt`, `offset_ms`
    after `CURSOR_T0`.
    """
    moment = datetime.datetime.fromtimestamp((CURSOR_T0 + offset_ms) / 1000, datetime.timezone.utc)
    return moment.strftime("%Y-%m-%dT%H:%M:%S.") + "%03dZ" % (moment.microsecond // 1000)


def _cursor_head(bubble_id: str, kind: int, offset_ms: int) -> dict:
    return {"bubbleId": bubble_id, "type": kind, "createdAt": _cursor_iso(offset_ms)}


def _cursor_tool(call: str, number: int, command: str, extra: dict | None, result: str) -> dict:
    tool = {
        "name": "run_terminal_command_v2",
        "params": json.dumps({"command": command}),
        "rawArgs": "",
        "result": result,
        "status": "completed",
        "tool": 15,
        "toolCallId": "call-%s-%d\nfc_%s_%d" % (call, number, call, number),
        "modelCallId": "",
        "toolIndex": number,
    }
    if extra is not None:
        tool["additionalData"] = extra
    return {"type": 2, "capabilityType": 15, "toolFormerData": tool}


def cursor_kv_records() -> list[tuple[str, object]]:
    """(key, value) rows of the fixture's `cursorDiskKV`, in insert order.

    (key, value) rows of the fixture's `cursorDiskKV`, in insert order; a str value is stored as it
    is, anything else as JSON text.
    """
    live, arch, gone = CURSOR_LIVE, CURSOR_ARCHIVED, CURSOR_DELETED
    curl = "curl -s example.test | head"
    options = [{"id": "curl", "commandName": "curl"}, {"id": "head", "commandName": "head"}]
    reason = "Not in allowlist: curl, head"
    tool_call_1, tool_call_2, tool_call_3, tool_call_4 = (
        "a1/t:call-t1-1\nfc_t1_1",
        "a2/t:call-t2-0\nfc_t2_0",
        "a2/t:call-t3-1\nfc_t3_1",
        "a2/t:call-t4-2\nfc_t4_2",
    )
    headers = [
        _cursor_head("u1", 1, 0),
        _cursor_head("a1/s:0", 2, 1000),
        _cursor_head(tool_call_1, 2, 2000),
        _cursor_head("a1/s:2", 2, 5000),
        _cursor_head("u2", 1, 60000),
        _cursor_head(tool_call_2, 2, 61000),
        _cursor_head(tool_call_3, 2, 70000),
        _cursor_head(tool_call_4, 2, 80000),
    ]
    read = {
        "type": 2,
        "capabilityType": 15,
        "startedAtMs": CURSOR_T0 + 2000,
        "completedAtMs": CURSOR_T0 + 2500,
        "toolFormerData": {
            "name": "read_file_v2",
            "params": json.dumps({"path": "/srv/proj/calc.py"}),
            "rawArgs": json.dumps({"path": "calc.py"}),
            "result": json.dumps({"contents": "def add(a, b): ..."}),
            "status": "completed",
            "tool": 40,
            "toolCallId": "tc1",
            "modelCallId": "mc1",
            "toolIndex": 0,
        },
    }
    ran = _cursor_tool(
        "t2",
        0,
        curl,
        {
            "status": "success",
            "startedAtMs": CURSOR_T0 + 61000,
            "blockReason": reason,
            "reviewData": {
                "selectedOption": "run",
                "approvalType": "user",
                "status": "Done",
                "allowlistOptions": options,
            },
        },
        json.dumps({"output": "ok"}),
    )
    skipped = _cursor_tool(
        "t3",
        1,
        curl,
        {
            "status": "rejected",
            "blockReason": reason,
            "reviewData": {"selectedOption": "skip", "approvalType": "user", "status": "Done"},
        },
        json.dumps({"error": "skipped"}),
    )
    always = _cursor_tool(
        "t4",
        2,
        curl,
        {
            "status": "success",
            "startedAtMs": CURSOR_T0 + 80000,
            "blockReason": reason,
            "reviewData": {
                "selectedOption": "allowlistCommands",
                "approvalType": "user",
                "status": "Done",
                "allowlistOptions": options,
                "appliedAllowlistOptionIds": ["curl", "head"],
            },
        },
        json.dumps({"output": "ok"}),
    )
    workspace = {"id": CURSOR_WORKSPACE, "uri": {"fsPath": "/srv/proj"}}
    return [
        (
            "composerData:" + live,
            {
                "_v": 18,
                "composerId": live,
                "status": "completed",
                "name": "Add subtract",
                "createdAt": CURSOR_T0 - 60000,
                "lastUpdatedAt": CURSOR_T0 + 300000,
                "isArchived": None,
                "modelConfig": {
                    "modelName": "grok-4.7",
                    "maxMode": False,
                    "selectedModels": [
                        {"modelId": "grok-4.7", "parameters": [{"id": "context", "value": "256k"}]}
                    ],
                },
                "workspaceIdentifier": workspace,
                "trackedGitRepos": [{"repoPath": "/srv/proj", "branches": []}],
                "fullConversationHeadersOnly": headers,
            },
        ),
        (
            "bubbleId:%s:u1" % live,
            {
                "type": 1,
                "text": "Add a subtract function",
                "createdAt": _cursor_iso(0),
                "startedAtMs": CURSOR_T0,
                "completedAtMs": CURSOR_T0 + 5000,
                "requestId": "r1",
                "checkpointId": "k1",
                "modelInfo": {"modelName": "grok-4.6"},
            },
        ),
        (
            "bubbleId:%s:a1/s:0" % live,
            {
                "type": 2,
                "bubbleId": "a1/s:0",
                "capabilityType": 30,
                "text": "",
                "createdAt": _cursor_iso(1000),
                "thinking": {"text": "Read calc.py first.", "signature": "sig"},
            },
        ),
        ("bubbleId:%s:%s" % (live, tool_call_1), read),
        (
            "bubbleId:%s:a1/s:2" % live,
            {
                "type": 2,
                "capabilityType": None,
                "text": "Added subtract().",
                "startedAtMs": CURSOR_T0 + 5000,
                "turnDurationMs": 5000,
            },
        ),
        (
            "bubbleId:%s:u2" % live,
            {
                "type": 1,
                "text": "Fetch the page with curl",
                "createdAt": _cursor_iso(60000),
                "modelInfo": {"modelName": "grok-4.7"},
            },
        ),
        ("bubbleId:%s:%s" % (live, tool_call_2), ran),
        ("bubbleId:%s:%s" % (live, tool_call_3), skipped),
        ("bubbleId:%s:%s" % (live, tool_call_4), always),
        (
            "composerData:" + arch,
            {
                "_v": 18,
                "composerId": arch,
                "status": "completed",
                "name": "Read the README",
                "createdAt": CURSOR_T0 + 1200000,
                "lastUpdatedAt": CURSOR_T0 + 1300000,
                "isArchived": None,
                "workspaceIdentifier": workspace,
                "fullConversationHeadersOnly": [],
            },
        ),
        # The second chunk first, so the suffix, not the rowid, orders them.
        (
            "composerConvHeaders:%s:000001" % arch,
            [_cursor_head("u4", 1, 1260000), _cursor_head("a4/s:0", 2, 1262000)],
        ),
        (
            "composerConvHeaders:%s:000000" % arch,
            [_cursor_head("u3", 1, 1200000), _cursor_head("a3/s:0", 2, 1202000)],
        ),
        (
            "bubbleId:%s:u3" % arch,
            {
                "type": 1,
                "text": "What does the README say?",
                "modelInfo": {"modelName": "grok-4.7"},
            },
        ),
        ("bubbleId:%s:a3/s:0" % arch, {"type": 2, "text": "It describes calc.py."}),
        (
            "bubbleId:%s:u4" % arch,
            {"type": 1, "text": "And the LICENSE?", "modelInfo": {"modelName": "grok-4.7"}},
        ),
        ("bubbleId:%s:a4/s:0" % arch, {"type": 2, "text": "There is none."}),
        (
            "composerData:" + CURSOR_DRAFT,
            {
                "_v": 18,
                "composerId": CURSOR_DRAFT,
                "status": "none",
                "createdAt": CURSOR_T0 + 9000000,
                "fullConversationHeadersOnly": [],
            },
        ),
        ("composerDraft:" + CURSOR_DRAFT, {"richText": "{}", "text": "draft not sent"}),
        (
            "agentKv:blob:" + "00ff" * 16,
            {
                "role": "user",
                "providerOptions": {},
                "content": "<context>conversation %s</context>\n"
                "<user_query>c3 deleted question</user_query>" % gone,
            },
        ),
        (
            "agentKv:blob:" + "11ee" * 16,
            {
                "role": "user",
                "providerOptions": {},
                "content": [{"type": "text", "text": "<context>%s</context> live" % live}],
            },
        ),
        ("agentKv:blob:" + "22dd" * 16, {"role": "system", "content": "You are an agent."}),
        # Protobuf: not JSON, though its bytes name the deleted composer.
        ("agentKv:blob:" + "0a0b" * 16, b"\x0a\x24" + gone.encode("ascii") + b"\x1a\x00"),
        ("composerTranscriptHeightTail:" + gone, {"height": 120}),
        # A reopened cloud conversation's cached details: protobuf, not read.
        (
            "bcCachedDetails:" + CURSOR_CLOUD,
            b"\x1a\x2b\x0a\x27" + CURSOR_CLOUD.encode("ascii") + b"\x12\x00",
        ),
    ]


def cursor_header_rows() -> list[tuple]:
    """The fixture's `composerHeaders` rows.

    (composerId, workspaceId, createdAt, lastUpdatedAt, isArchived, value).
    """
    workspace = {"id": CURSOR_WORKSPACE, "uri": {"fsPath": "/srv/proj"}}
    location = {"type": "local", "status": "active"}

    def head(cid, created, updated, archived, **extra):
        value = {
            "type": "head",
            "composerId": cid,
            "createdAt": created,
            "lastUpdatedAt": updated,
            "isArchived": bool(archived),
            **extra,
        }
        return (cid, CURSOR_WORKSPACE, created, updated, archived, json.dumps(value))

    history = [{"id": "l1", "reason": "created", "timestamp": CURSOR_T0}]
    return [
        head(
            CURSOR_LIVE,
            CURSOR_T0 - 60000,
            CURSOR_T0 + 300000,
            0,
            name="Add subtract",
            workspaceIdentifier=workspace,
            agentLocation=location,
            agentLocationHistory=history,
        ),
        head(
            CURSOR_ARCHIVED,
            CURSOR_T0 + 1200000,
            CURSOR_T0 + 7200000,
            1,
            name="Read the README",
            workspaceIdentifier=workspace,
            agentLocation=location,
            agentLocationHistory=history,
        ),
        # Deleted: the header keeps its times, location and workspace, and
        # loses `name` and `subtitle`.
        head(
            CURSOR_DELETED,
            CURSOR_T0 - 30600000,
            CURSOR_T0 + 10800000,
            1,
            workspaceIdentifier=workspace,
            agentLocation=location,
            agentLocationHistory=history,
            totalLinesAdded=0,
            totalLinesRemoved=0,
        ),
        head(CURSOR_DRAFT, CURSOR_T0 + 9000000, CURSOR_T0 + 9000000, 0, isDraft=True),
        # A pre-login draft whose composerData is gone: no location
        # history, not archived, so not a deleted conversation.
        head(CURSOR_EMPTY_HEAD, CURSOR_T0 - 90000000, CURSOR_T0 - 90000000, 0, isDraft=True),
    ]


def cursor_review_bubbles() -> list[tuple[dict, dict]]:
    """(header entry, bubble) pairs a test appends to the live conversation.

    (header entry, bubble) pairs that a test appends to the live conversation, from a Linux install
    (3.25.9 editor window): a command whose prompt was still open when the store was copied
    (`toolFormerData.status` `loading`, `reviewData.status` `Requested` and the preset
    `selectedOption`), and the editor window's "add python3 to the allow list", stored as
    `selectedOption` `run` with `appliedAllowlistOptionIds`.
    """
    command = "python3 -c 'print(1 + 1)'"
    options = [{"id": "python3", "commandName": "python3"}]
    reason = "Not in allowlist: python3"
    pending = _cursor_tool(
        "t5",
        3,
        command,
        {
            "blockReason": reason,
            "reviewData": {
                "status": "Requested",
                "selectedOption": "rejectAndTellWhatToDoDifferently",
                "isShowingInput": False,
                "allowlistOptions": options,
            },
        },
        "",
    )
    pending["toolFormerData"]["status"] = "loading"
    added = _cursor_tool(
        "t6",
        4,
        command,
        {
            "status": "success",
            "startedAtMs": CURSOR_T0 + 100000,
            "blockReason": reason,
            "reviewData": {
                "status": "Done",
                "selectedOption": "run",
                "allowlistOptions": options,
                "candidatesForAllowlist": ["python3"],
                "appliedAllowlistOptionIds": ["python3"],
            },
        },
        json.dumps({"output": "2"}),
    )
    return [
        (_cursor_head("a2/t:call-t5-3\nfc_t5_3", 2, 90000), pending),
        (_cursor_head("a2/t:call-t6-4\nfc_t6_4", 2, 100000), added),
    ]


def cursor_settings() -> dict:
    """The `ItemTable` settings blob (`composerState`, `aiSettings`)."""
    return {
        "composerState": {
            "modes4": [
                {"id": "agent", "autoRun": True, "smartModeAutoRun": False, "fullAutoRun": False},
                {"id": "plan", "autoRun": False, "smartModeAutoRun": False, "fullAutoRun": False},
            ],
            "yoloCommandAllowlist": ["curl", "head"],
            "yoloCommandDenylist": [],
            "smartAllowlistDenylist": [],
            "webFetchDomainAllowlist": [],
            "autoApprovedModeTransitions": [],
            "yoloOutsideWorkspaceDisabled": True,
        },
        "aiSettings": {
            "modelConfig": {"composer": {"modelName": "grok-4.7", "maxMode": False}},
            "modelLastUsedAt": {"grok-4.7": "2026-10-01T10:00:30.000Z"},
        },
        "membershipType": "premium",
    }


CURSOR_GITHUB_LOGIN = "alice-gh"
CURSOR_ACCOUNT = "auth0|user_01"
CURSOR_SECRET = "CURSOR-SECRET-SENTINEL"


def cursor_integration_items() -> list[tuple[str, str]]:
    """ItemTable rows of Cursor's GitHub sign-in and agent host enrolment.

    ItemTable rows of the editor's own GitHub sign-in and the remote agent host enrolment (3.25.9
    Linux), with two `secret://` rows whose values the parser must never read.
    """
    login, account = CURSOR_GITHUB_LOGIN, CURSOR_ACCOUNT
    scopes = ["repo", "workflow", "user:email", "read:user"]
    usages = [
        {
            "extensionId": "vscode.github",
            "extensionName": "GitHub",
            "lastUsed": CURSOR_T0 + 400000,
            "scopes": scopes,
        },
        {
            "extensionId": "vscode.git",
            "extensionName": "Git",
            "lastUsed": CURSOR_T0 + 300000,
            "scopes": ["repo", "read:user"],
        },
    ]
    buffer = {"type": "Buffer", "data": [ord(char) for char in CURSOR_SECRET]}
    return [
        (
            "github-" + login,
            json.dumps([{"id": "vscode.github", "name": "GitHub", "allowed": True}]),
        ),
        ("github-%s-usages" % login, json.dumps(usages)),
        ("vscode.github-github-" + " ".join(scopes), "0123456789abcdef"),
        ("remoteAgentHost.enrolled." + account, "true"),
        ("remoteAgentHost.displayName." + account, "lab-vm"),
        ("remoteAgentHost.targetId", "e0000001-0000-4000-8000-000000000001"),
        ("secret://remoteAgentHost.computerKey." + account, CURSOR_SECRET),
        (
            'secret://{"extensionId":"vscode.github-authentication","key":"github.auth"}',
            json.dumps(buffer),
        ),
    ]


def cursor_cloud_agent(bc_id: str, offset_ms: int, **extra) -> dict:
    """One element of a `cloudAgentRepository.agents.<account>` list.

    One element of a `cloudAgentRepository.agents.<account>` list, with the key set of a Windows
    3.26.13 install; times are epoch ms `offset_ms` after `CURSOR_T0`.
    """
    agent = {
        "bcId": bc_id,
        "cloudTargetKey": "github:alice-gh/demo:main:0123456789abcdef0123",
        "createdAt": CURSOR_T0 + offset_ms,
        "environmentPublicId": "e1000001-0000-4000-8000-000000000001",
        "hasPendingInteraction": False,
        "hasStartedVm": True,
        "isArchived": False,
        "isKilled": False,
        "isOwnedByDifferentTeamMember": False,
        "isUnread": False,
        "lastMessageActivityAtMs": CURSOR_T0 + offset_ms + 60000,
        "lastReadAtMs": CURSOR_T0 + offset_ms + 90000,
        "modelDetails": {"maxMode": False, "modelName": "grok-4.7"},
        "name": "Describe the demo repository",
        "repoUrl": CURSOR_CLOUD_REPO,
        "repoUrls": [CURSOR_CLOUD_REPO],
        "requestedModel": {
            "builtInModel": "",
            "maxMode": False,
            "modelId": "grok-4.7",
            "parameters": [{"id": "context", "value": "256k"}],
        },
        "rowUpdatedAtMs": CURSOR_T0 + offset_ms + 120000,
        "source": 16,
        "status": 2,
        "transportRepoUrls": ["https://" + CURSOR_CLOUD_REPO],
        "updatedAt": CURSOR_T0 + offset_ms + 60000,
        "usePrivateWorker": False,
        "workflowStatus": 2,
        "workspaceRootPath": "/home/ubuntu/demo",
    }
    agent.update(extra)
    return agent


def cursor_cloud_items() -> list[tuple[str, str]]:
    """ItemTable rows of the cloud agents (Windows 3.26.13).

    The agent list under both spellings of the account id, the server pins, the repository list, and
    the connection token, a `secret://` row the parser must never read.
    """
    archived = cursor_cloud_agent(CURSOR_CLOUD, 10000000, isArchived=True)
    listed = cursor_cloud_agent(
        CURSOR_CLOUD_LISTED,
        11000000,
        name="Summarise the web repository",
        repoUrl="github.com/alice-gh/web",
        requestedModel={"builtInModel": "", "maxMode": False, "modelId": "default"},
        hasStartedVm=False,
        status=1,
        workflowStatus=1,
    )
    repos = [
        {
            "name": "alice-gh/demo",
            "scmRepoNodeId": "R_kgDOAAAAAA",
            "type": "github",
            "url": "https://github.com/alice-gh/demo",
        },
        {"name": "alice-gh/web", "scmRepoNodeId": "R_kgDOAAAAAB", "type": "github"},
    ]
    token = 'secret://{"extensionId":"anysphere.cursor-resolver-helper","key":"connectionToken:%s"}'
    return [
        ("cloudAgentRepository.agents." + CURSOR_ACCOUNT, json.dumps([archived])),
        ("cloudAgentRepository.agents.auth0%7Cuser_02", json.dumps([listed])),
        ("cloudAgentRepository.serverPinnedBcIds." + CURSOR_ACCOUNT, "[]"),
        ("cloudEnvironmentTemplate.githubRepos", json.dumps(repos)),
        ("glass.cloudAgentProjectMembership.v1", json.dumps({CURSOR_CLOUD: "p1"})),
        (token % CURSOR_CLOUD, CURSOR_SECRET),
    ]


CURSOR_MCP_SCOPE = "plugin-calendar-calendar::mcpScope:profile:ZGVmYXVsdA"
CURSOR_MCP_ATTEMPT = "oauth-attempt:a0000001-0000-4000-8000-000000000001"
CURSOR_MCP_CALL = "toolu_mcp_auth_01\nfc_mcp_auth_01"
CURSOR_ATTEMPT_REL = (
    "Library/Application Support/Cursor/User/globalStorage/mcp-oauth-attempts/0000.json"
)


def cursor_mcp_key(prefix: str, name: str, padded: bool = False) -> str:
    """An `mcpOAuth.global.` or `mcpOAuth.secret.` key.

    The prefix and the URL-safe base64 of `name` (`[<scope>] <field>`), unpadded as on the real
    install unless `padded`.
    """
    encoded = base64.urlsafe_b64encode(name.encode("utf-8")).decode("ascii")
    return prefix + (encoded if padded else encoded.rstrip("="))


def cursor_mcp_items() -> list[tuple[str, str]]:
    """ItemTable rows of the plugin and MCP steps (Windows 3.26.13).

    The MCP OAuth keys of a plugin server and of a second server with no update time or token, an
    attempt scope, the migration flag, the secret keys whose ciphertext values hold the sentinel the
    parser must never read, the installed plugin lists and the cloud agent's MCP auth resolution.
    """
    glob, secret = "mcpOAuth.global.", "mcpOAuth.secret."
    other = "plugin-acme-tracker-tracker::mcpScope:profile:ZGVmYXVsdA"
    buffer = json.dumps({"type": "Buffer", "data": [ord(char) for char in CURSOR_SECRET]})
    plugins = json.dumps([{"id": "12345678", "sources": ["user"]}])
    resolution = {
        "agentId": CURSOR_CLOUD,
        "authId": "f0000001-0000-4000-8000-000000000001",
        "nonce": "f0000002-0000-4000-8000-000000000002",
        "queryId": "f0000003-0000-4000-8000-000000000003",
        "toolCallId": CURSOR_MCP_CALL,
        "userDecision": "accepted",
        "version": 1,
    }
    return [
        (
            cursor_mcp_key(glob, "[%s] mcp_server_url" % CURSOR_MCP_SCOPE),
            json.dumps("https://calendar.mcp.example.test/mcp"),
        ),
        (
            cursor_mcp_key(glob, "[%s] mcp_oauth_updated_at_ms" % CURSOR_MCP_SCOPE),
            str(CURSOR_T0 + 500000),
        ),
        (
            cursor_mcp_key(glob, "[%s] mcp_tokens_canonical_migration_complete" % CURSOR_MCP_SCOPE),
            "true",
        ),
        (
            cursor_mcp_key(glob, "[%s] mcp_server_url" % other, padded=True),
            "https://tracker.example.test/mcp",
        ),
        (
            cursor_mcp_key(glob, "[%s] mcp_server_url" % CURSOR_MCP_ATTEMPT),
            json.dumps("https://calendar.mcp.example.test/mcp"),
        ),
        (cursor_mcp_key(glob, "mcpOAuth.extensionMigrationComplete"), "true"),
        (cursor_mcp_key(secret, "[%s] mcp_tokens" % CURSOR_MCP_SCOPE), buffer),
        (cursor_mcp_key(secret, "[%s] mcp_client_information" % CURSOR_MCP_SCOPE), buffer),
        (cursor_mcp_key(secret, "[%s] mcp_code_verifier" % CURSOR_MCP_ATTEMPT), buffer),
        ("cursor.plugins.installedIds.no-team|no-workspace", plugins),
        ("cursor.plugins.installedIds.no-team|file:///srv/proj", plugins),
        ("cloudAgentRepository.mcpAuthResolution", json.dumps(resolution)),
    ]


def cursor_attempt_record() -> dict:
    """An `mcp-oauth-attempts/<uuid>.json` file.

    An `mcp-oauth-attempts/<uuid>.json` file (Windows 3.26.13, the shape after authentication, with
    no `authorizationServerUrl`).
    """
    return {
        "activeIdentifier": "plugin-calendar-calendar::mcpScope:profile:default",
        "attemptId": "a0000002-0000-4000-8000-000000000002",
        "clientInformation": {
            "client_id": "client-0123456789abcdef",
            "grant_types": ["authorization_code", "refresh_token"],
            "redirect_uris": ["cursor://anysphere.cursor-mcp/oauth/callback"],
            "response_types": ["code"],
        },
        "createdAtMs": CURSOR_T0 + 480000,
        "identifier": "plugin-calendar-calendar",
        "owner": {"workspaceId": "empty-window"},
        "serverUrl": "https://calendar.mcp.example.test/mcp",
        "transportServerUrl": "https://calendar.mcp.example.test/mcp",
    }


def cursor_mcp_bubble() -> tuple[dict, dict]:
    """(header entry, bubble) of an MCP tool call approved under Allowlist (Windows 3.26.13).

    No `reviewData`, `additionalData` holds only `status`, and `toolCallBinary` a serialized copy of
    the call.
    """
    call = "call-m1-5\nfc_m1_5"
    tool = {
        "name": "mcp-google-calendar-list_events",
        "tool": 19,
        "status": "completed",
        "toolCallId": call,
        "modelCallId": "model-call-m1",
        "toolIndex": 5,
        "rawArgs": json.dumps({"calendarId": "primary", "timeMin": "2026-10-01T00:00:00Z"}),
        "params": json.dumps({"calendarId": "primary"}),
        "result": json.dumps({"events": [{"summary": "Standup"}]}),
        "additionalData": {"status": "success"},
        "toolCallBinary": "Q0FMTEJJTkFSWQ==",
    }
    bubble = {
        "type": 2,
        "capabilityType": 15,
        "startedAtMs": CURSOR_T0 + 110000,
        "completedAtMs": CURSOR_T0 + 111000,
        "toolFormerData": tool,
    }
    return _cursor_head("a2/t:%s" % call, 2, 110000), bubble


def cursor_stopped_bubbles() -> list[tuple[dict, dict]]:
    """(header entry, bubble) of a turn the user stopped (Windows 3.26.13).

    The call in flight (`tc9`), rejected with `reviewData.status` the string `None` and the preset
    option, and a queued call (`tc10`), cancelled with an empty `additionalData`.
    """
    stopped = _cursor_tool(
        "t9",
        5,
        "dir",
        {
            "status": "rejected",
            "reviewData": {
                "status": "None",
                "selectedOption": "rejectAndTellWhatToDoDifferently",
                "isShowingInput": False,
            },
        },
        "",
    )
    stopped["toolFormerData"]["toolCallId"] = "tc9"
    stopped["startedAtMs"] = CURSOR_T0 + 120000
    cancelled = _cursor_tool("t10", 6, "type README.md", {}, "")
    cancelled["toolFormerData"].update(toolCallId="tc10", status="cancelled")
    cancelled["startedAtMs"] = CURSOR_T0 + 121000
    return [
        (_cursor_head("a2/t:tc9", 2, 120000), stopped),
        (_cursor_head("a2/t:tc10", 2, 121000), cancelled),
    ]


def write_cursor_state(path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(path))
    connection.executescript(CURSOR_STATE_SCHEMA)
    for key, value in cursor_kv_records():
        stored = value if isinstance(value, (str, bytes)) else json.dumps(value)
        connection.execute("INSERT INTO cursorDiskKV (key, value) VALUES (?, ?)", (key, stored))
    for row in cursor_header_rows():
        connection.execute(
            "INSERT INTO composerHeaders (composerId, workspaceId, createdAt, lastUpdatedAt,"
            " isArchived, isSubagent, value) VALUES (?, ?, ?, ?, ?, 0, ?)",
            row,
        )
    for key, value in (
        (CURSOR_SETTINGS_KEY, json.dumps(cursor_settings())),
        ("cursor/sandboxAutoRunSettingEnabled", "true"),
        ("cursor/sandboxSupported", "true"),
        *cursor_integration_items(),
        *cursor_cloud_items(),
        *cursor_mcp_items(),
    ):
        connection.execute(
            "INSERT INTO ItemTable (key, value) VALUES (?, ?)", (key, value.encode("utf-8"))
        )
    connection.commit()
    connection.close()
    os.utime(path, (CURSOR_MTIME, CURSOR_MTIME))


def cursor_transcript_records() -> list[dict]:
    """The live conversation's transcript, written at UTC-4.

    Its user lines are the bubbles u1 and u2 (10:00Z and 10:01Z).
    """

    def user(tag: str, query: str) -> dict:
        text = "<timestamp>%s</timestamp>\n<user_query>%s</user_query>" % (tag, query)
        return {"role": "user", "message": {"content": [{"type": "text", "text": text}]}}

    shell = {
        "command": "curl -s example.test | head",
        "description": "Fetch the page",
        "required_permissions": ["full_network"],
    }
    return [
        user("Thursday, Oct 1, 2026, 6:00 AM (UTC-4)", "Add a subtract function"),
        {
            "role": "assistant",
            "message": {"content": [{"type": "text", "text": "Added subtract()."}]},
        },
        {"type": "turn_ended", "status": "success"},
        user("Thursday, Oct 1, 2026, 6:01 AM (UTC-4)", "Fetch the page with curl"),
        {
            "role": "assistant",
            "message": {"content": [{"type": "tool_use", "name": "Shell", "input": shell}]},
        },
        {"type": "turn_ended", "status": "success"},
    ]


def cursor_deleted_transcript_records() -> list[dict]:
    """The deleted conversation's transcript, which the delete left."""
    text = (
        "<timestamp>Wednesday, Sep 30, 2026, 9:30 PM (UTC-4)</timestamp>\n"
        "<user_query>c3 deleted question</user_query>"
    )
    return [
        {"role": "user", "message": {"content": [{"type": "text", "text": text}]}},
        {
            "role": "assistant",
            "message": {"content": [{"type": "text", "text": "Answer kept only here."}]},
        },
        {"type": "turn_ended", "status": "success"},
    ]


CURSOR_TRACKING_SCHEMA = """
CREATE TABLE ai_code_hashes (hash TEXT PRIMARY KEY, source TEXT, fileExtension TEXT,
  fileName TEXT, requestId TEXT, conversationId TEXT, timestamp INTEGER, model TEXT,
  createdAt INTEGER);
CREATE TABLE scored_commits (commitHash TEXT, branchName TEXT, scoredAt INTEGER,
  linesAdded INTEGER, linesDeleted INTEGER, tabLinesAdded INTEGER, tabLinesDeleted INTEGER,
  composerLinesAdded INTEGER, composerLinesDeleted INTEGER, humanLinesAdded INTEGER,
  humanLinesDeleted INTEGER, blankLinesAdded INTEGER, blankLinesDeleted INTEGER,
  commitMessage TEXT, commitDate TEXT, v1AiPercentage TEXT, v2AiPercentage TEXT);
CREATE TABLE tracking_state (key TEXT PRIMARY KEY, value TEXT);
CREATE TABLE conversation_summaries (conversationId TEXT, title TEXT, tldr TEXT, overview TEXT,
  summaryBullets TEXT, model TEXT, mode TEXT, updatedAt INTEGER);
CREATE TABLE tracked_file_content (gitPath TEXT, content TEXT, conversationId TEXT, model TEXT,
  fileExtension TEXT, createdAt INTEGER);
CREATE TABLE ai_deleted_files (gitPath TEXT, composerId TEXT, conversationId TEXT, model TEXT,
  deletedAt INTEGER);
"""


def write_cursor_tracking(path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(path))
    connection.executescript(CURSOR_TRACKING_SCHEMA)
    for row in (
        ("h1", "composer", "py", "/srv/proj/calc.py", "r1", CURSOR_LIVE, CURSOR_T0 + 3000),
        ("h2", "composer", "py", "/srv/proj/calc.py", "r9", CURSOR_DELETED, CURSOR_T0 - 30540000),
    ):
        connection.execute(
            "INSERT INTO ai_code_hashes VALUES (?, ?, ?, ?, ?, ?, ?, 'grok-4.6', ?)", (*row, 0)
        )
    connection.execute("INSERT INTO tracking_state VALUES ('trackingStartTime', '1790800000000')")
    # Unpopulated on the real install; shapes from the table definitions.
    connection.execute(
        "INSERT INTO conversation_summaries VALUES (?, 'Add subtract', 'Added subtract().',"
        " '', '[]', 'grok-4.6', 'agent', ?)",
        (CURSOR_LIVE, CURSOR_T0 + 6000),
    )
    connection.execute(
        "INSERT INTO ai_deleted_files VALUES ('old.py', ?, ?, 'grok-4.7', ?)",
        (CURSOR_LIVE, CURSOR_LIVE, CURSOR_T0 + 90000),
    )
    connection.execute(
        "INSERT INTO scored_commits (commitHash, branchName, scoredAt, linesAdded, linesDeleted)"
        " VALUES ('abc1234', 'main', ?, 4, 0)",
        (CURSOR_T0 + 400000,),
    )
    connection.commit()
    connection.close()


CURSOR_SEARCH_SCHEMA = """
CREATE TABLE conversations (fts_rowid INTEGER PRIMARY KEY, source TEXT NOT NULL
  CHECK (source IN ('local', 'cloud-cache')), scope TEXT, id TEXT, title TEXT, branches TEXT,
  updated_at INTEGER, is_archived INTEGER, root_fingerprint TEXT, cache_fingerprint TEXT);
CREATE TABLE conversation_search_settings (effective_conversation_cap INTEGER);
CREATE TABLE conversation_fts_content (id INTEGER PRIMARY KEY, c0, c1, c2);
"""


def write_cursor_search(path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(path))
    connection.executescript(CURSOR_SEARCH_SCHEMA)
    for row in (
        ("local", CURSOR_LIVE, "Add subtract", CURSOR_T0 + 300000, 0),
        ("local", CURSOR_ARCHIVED, "Read the README", CURSOR_T0 + 7200000, 1),
        ("local", CURSOR_DRAFT, "", CURSOR_T0 + 9000000, 0),
    ):
        connection.execute(
            "INSERT INTO conversations (source, scope, id, title, branches, updated_at,"
            " is_archived) VALUES (?, 'workspace', ?, ?, '[]', ?, ?)",
            row,
        )
    # A cloud agent's cached conversation (Windows 3.26.13): the
    # FTS5 content holds its title and body; a local row's may be empty
    # (Windows) or hold text (macOS). The
    # fixture writes the shadow table as a plain table, as the parser reads it.
    connection.execute(
        "INSERT INTO conversations (fts_rowid, source, scope, id, title, branches, updated_at,"
        " is_archived, cache_fingerprint) VALUES (10, 'cloud-cache', ?, ?, ?, '', ?, 1, ?)",
        (
            "ab" * 32,
            CURSOR_CLOUD,
            "Describe the demo repository",
            CURSOR_T0 + 10060000,
            "cd" * 32,
        ),
    )
    for rowid, title, body in (
        (1, "Add subtract", "Add a subtract function"),
        (2, "", ""),
        (3, "", ""),
        (10, "Describe the demo repository", CURSOR_CLOUD_BODY),
    ):
        connection.execute(
            "INSERT INTO conversation_fts_content VALUES (?, ?, ?, '')", (rowid, title, body)
        )
    connection.execute("INSERT INTO conversation_search_settings VALUES (10000)")
    connection.commit()
    connection.close()


def build_cursor(home: Path) -> None:
    write_cursor_state(home / CURSOR_STATE_REL)
    write_cursor_search(home / CURSOR_SEARCH_REL)
    write_cursor_tracking(home / CURSOR_TRACKING_REL)
    _jsonl(home / CURSOR_TRANSCRIPT_REL, cursor_transcript_records())
    write_bad_line(home / CURSOR_TRANSCRIPT_REL)
    _jsonl(home / CURSOR_DELETED_TRANSCRIPT_REL, cursor_deleted_transcript_records())
    _json_file(home / CURSOR_ATTEMPT_REL, cursor_attempt_record())
    for rel, doc in (
        (
            ".cursor/cli-config.json",
            {
                "version": 1,
                "approvalMode": "allowlist",
                "permissions": {"allow": ["Shell(ls)"], "deny": []},
                "sandbox": {"mode": "disabled", "networkAccess": "allowlist"},
                "display": {"mode": "zen"},
            },
        ),
        (
            ".cursor/permissions.json",
            {
                "approvalMode": "allowlist",
                "allowlists": {"mcp": ["github:list_issues"], "terminal": ["git status"]},
                "allowInstructions": ["read-only git commands"],
                "blockInstructions": ["anything that pushes"],
            },
        ),
    ):
        _json_file(home / rel, doc)
        os.utime(home / rel, (CURSOR_MTIME, CURSOR_MTIME))
    # Noise the parser must not want.
    _json_file(home / ".cursor/statsig-cache.json", {"flags": {}})
    _json_file(
        home / ".cursor/projects/srv-proj/mcps/cursor-ide-browser/SERVER_METADATA.json",
        {"name": "cursor-ide-browser"},
    )
    _json_file(home / ".cursor/mcp.json", {"mcpServers": {}})
