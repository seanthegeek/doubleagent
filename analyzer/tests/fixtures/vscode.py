"""Synthetic VS Code state for the analyzer tests."""

from __future__ import annotations

import json
import os
from pathlib import Path

from .common import _json, _jsonl, _vscdb

VSCODE_SESSION = "9ab0c1d2-0000-4000-8000-00000000c0de"
VSCODE_LEGACY_SESSION = "9ab0c1d2-0000-4000-8000-0000000001d0"
VSCODE_T0 = 1791025200000  # 3 October 11:00:00 UTC, ms
VSCODE_WORKSPACE = ".config/Code/User/workspaceStorage/abc123"


def vscode_log_records(cwd="/srv/proj"):
    """A `chatSessions/<id>.jsonl` mutation log, as in research/vscode.md.

    The modelId, agent id and responder strings come from the closed Copilot extension and are
    invented.
    """
    start = VSCODE_T0
    return [
        {
            "kind": 0,
            "v": {
                "version": 3,
                "creationDate": start,
                "customTitle": None,
                "initialLocation": "panel",
                "responderUsername": "GitHub Copilot",
                "sessionId": VSCODE_SESSION,
                "requests": [
                    {
                        "requestId": "request_1",
                        "timestamp": start + 1000,
                        "message": {"text": "run tests", "parts": []},
                        "agent": {
                            "id": "github.copilot.default",
                            "name": "GitHub Copilot",
                            "extensionId": {"value": "GitHub.copilot-chat"},
                        },
                        "modelId": "copilot/gpt-4.1",
                        "modeInfo": {
                            "kind": "agent",
                            "telemetryModeId": "agent",
                            "isBuiltin": True,
                            # ChatPermissionLevel (constants.ts:204-213 at 4d22f4f)
                            "permissionLevel": "default",
                        },
                        "variableData": {
                            "variables": [
                                {
                                    "id": "file://%s/test_a.py" % cwd,
                                    "name": "test_a.py",
                                    "value": "...",
                                }
                            ]
                        },
                        "response": [
                            {"kind": "thinking", "value": "Need pytest", "id": "t1"},
                            {
                                "kind": "toolInvocationSerialized",
                                "toolCallId": "call_a",
                                "toolId": "run_in_terminal",
                                "invocationMessage": "Running command",
                                "isComplete": True,
                                # ConfirmedReason, ToolConfirmKind.UserAction = 4 (1.104+)
                                "isConfirmed": {"type": 4, "selectedButton": "Allow"},
                                "source": {"type": "internal", "label": "Built-In"},
                                "toolSpecificData": {
                                    "kind": "terminal",
                                    "commandLine": {"original": "pytest"},
                                },
                                "resultDetails": {
                                    "input": "pytest",
                                    "output": [{"type": "embed", "value": "3 passed"}],
                                },
                            },
                            {"value": "All tests pass in "},
                            {
                                "kind": "inlineReference",
                                "inlineReference": {"scheme": "file", "path": cwd + "/test_a.py"},
                                "name": "test_a.py",
                            },
                            {"value": "."},
                        ],
                        "responseId": "response_1",
                        "responseTimestamp": start + 2000,
                        "modelState": {"value": "complete"},
                    }
                ],
                "workingDirectory": "file://%s" % cwd,
            },
        },
        {
            "kind": 2,
            "k": ["requests"],
            "v": [
                {
                    "requestId": "request_2",
                    "timestamp": start + 10000,
                    "message": {"text": "thanks", "parts": []},
                    "variableData": {"variables": []},
                    "modelId": "copilot/gpt-4.1",
                    "response": [],
                }
            ],
        },
        {
            "kind": 2,
            "k": ["requests", 1, "response"],
            "v": [
                {"value": "You're welcome."},
                {
                    "kind": "confirmation",
                    "title": "Run npm install?",
                    "message": {"value": "This installs the packages."},
                    "data": {},
                    "buttons": ["Allow", "Dismiss"],
                    "isUsed": True,
                },
            ],
        },
        {"kind": 1, "k": ["requests", 1, "responseTimestamp"], "v": start + 11000},
        {"kind": 1, "k": ["customTitle"], "v": "Run tests"},
        {
            # The button click sends a request whose `confirmation` is its label;
            # the user also switched model and mode.
            "kind": 2,
            "k": ["requests"],
            "v": [
                {
                    "requestId": "request_3",
                    "timestamp": start + 20000,
                    "message": {"text": 'Allow: "Run npm install?"', "parts": []},
                    "variableData": {"variables": []},
                    "confirmation": "Allow",
                    "modelId": "copilot/claude-sonnet-4",
                    # The model's options (chatModel.ts:2157-2158 at 4d22f4f);
                    # the key name is invented.
                    "modelConfiguration": {"reasoningEffort": "high"},
                    "modeInfo": {
                        "kind": "ask",
                        "telemetryModeId": "ask",
                        "isBuiltin": True,
                        "permissionLevel": "autoApprove",
                    },
                    "response": [{"value": "Installed."}],
                    "responseTimestamp": start + 21000,
                }
            ],
        },
    ]


def vscode_legacy_session(cwd="/srv/proj"):
    """A pre-1.109 flat `.json` session from an empty window on a remote host."""
    start = VSCODE_T0 - 3600000
    return {
        "version": 3,
        "sessionId": VSCODE_LEGACY_SESSION,
        "creationDate": start,
        "initialLocation": "panel",
        "responderUsername": "GitHub Copilot",
        "workingDirectory": "file://%s" % cwd,
        "requests": [
            {
                "requestId": "request_1",
                "timestamp": start + 1000,
                "message": "where is the config?",
                "modelId": "copilot/gpt-4o",
                "response": ["It is in ", "config.toml."],
                "responseTimestamp": start + 2000,
                "result": {"errorDetails": {"message": "Rate limited"}},
            }
        ],
    }


def build_vscode(home: Path) -> None:
    workspace = home / VSCODE_WORKSPACE
    _jsonl(workspace / "chatSessions" / (VSCODE_SESSION + ".jsonl"), vscode_log_records())
    (workspace / "workspace.json").write_text(
        json.dumps({"folder": "file:///srv/proj"}), encoding="utf-8"
    )
    (home / ".config/Code/User/settings.json").write_text(
        '{"chat.useLogSessionStorage": true}', encoding="utf-8"
    )
    legacy = home / ".vscode-server/data/User/globalStorage/emptyWindowChatSessions"
    legacy.mkdir(parents=True, exist_ok=True)
    (legacy / (VSCODE_LEGACY_SESSION + ".json")).write_text(
        json.dumps(vscode_legacy_session(), indent=2), encoding="utf-8"
    )
    build_vscode_sync(home)
    build_vscode_grants(home)


# Standing grants, settings, the GitHub account and the chat editing state,
# source-derived from microsoft/vscode at 4d22f4f
# (research/vscode.md, Standing permission grants and Chat editing
# sessions). The ids, labels and the login are invented.
VSCODE_WORKSPACE_STATE_REL = VSCODE_WORKSPACE + "/state.vscdb"
VSCODE_PROFILE_SETTINGS_REL = ".config/Code/User/profiles/-5f3a1b2c/settings.json"
VSCODE_EDITING_REL = VSCODE_WORKSPACE + "/chatEditingSessions/%s/state.json" % VSCODE_SESSION
VSCODE_GRANTS_MTIME = 1791025300  # 3 October 11:01:40 UTC


def vscode_editing_state(cwd="/srv/proj"):
    """A chat editing `state.json` (chatEditingSessionStorage.ts:242-288)."""

    def entry(name, state, request, **more):
        return {
            "resource": "file://%s/%s" % (cwd, name),
            "languageId": "python",
            "originalHash": "aaaaaaa",
            "currentHash": "bbbbbbb",
            "state": state,
            "snapshotUri": "chat-editing-snapshot-text-model:/%s/%s" % (cwd, name),
            "telemetryInfo": {"requestId": request, "modelId": "copilot/gpt-4.1", **more},
        }

    return {
        "version": 2,
        "initialFileContents": [["file://%s/test_a.py" % cwd, "aaaaaaa"]],
        "timeline": None,
        "recentSnapshot": {
            "stopId": None,
            "entries": [
                entry("test_a.py", 1, "request_1", agentId="github.copilot.editsAgent"),
                {**entry("old.py", 2, "request_1"), "isDeleted": True},
            ],
        },
    }


def build_vscode_grants(home: Path) -> None:
    _vscdb(
        home / VSCODE_WORKSPACE_STATE_REL,
        {
            "chat/autoconfirm": {
                "run_in_terminal": True,
                "mcp_github_create_issue": {"label": "Create issue", "arguments": '{"repo":"x"}'},
            },
            "chat/servers/autoconfirm": ["github"],
            "chat.externalPath.workspaceAllowlist": ["file:///srv/shared"],
            "workbench.panel.chat": "not read",
        },
    )
    _vscdb(
        home / VSCODE_STATE_REL,
        {
            "chat/autoconfirm-post": {"read_file": "Read File"},
            "chat.tools.global.autoApprove.optIn": "true",
            # authenticationUsageService.ts:103, authenticationMcpUsageService.ts:101
            "github-octocat-usages": [
                {
                    "extensionId": "GitHub.copilot-chat",
                    "extensionName": "GitHub Copilot Chat",
                    "scopes": ["read:user", "user:email"],
                    "lastUsed": 1791025100000,
                }
            ],
            "github-octocat-mcpserver-usages": [
                {
                    "mcpServerId": "github",
                    "mcpServerName": "GitHub",
                    "scopes": ["repo"],
                    "lastUsed": 1791025150000,
                }
            ],
        },
    )
    settings = home / VSCODE_PROFILE_SETTINGS_REL
    settings.parent.mkdir(parents=True, exist_ok=True)
    settings.write_text(
        "{\n"
        "  // a profile's settings, JSONC\n"
        '  "chat.tools.terminal.autoApprove": {"npm test": true, "rm": false},\n'
        '  "chat.tools.edits.autoApprove": {},\n'
        '  "chat.permissions.default": "autoApprove",\n'
        '  "chat.defaultModel": "copilot/gpt-5",\n'
        '  "editor.fontSize": 14,\n'
        "}\n",
        encoding="utf-8",
    )
    _json(home / VSCODE_EDITING_REL, vscode_editing_state())
    contents = home / VSCODE_WORKSPACE / "chatEditingSessions" / VSCODE_SESSION / "contents"
    contents.mkdir(parents=True, exist_ok=True)
    (contents / "aaaaaaa").write_text("print('a')\n", encoding="utf-8")
    for rel in (
        VSCODE_WORKSPACE_STATE_REL,
        VSCODE_STATE_REL,
        VSCODE_PROFILE_SETTINGS_REL,
        VSCODE_EDITING_REL,
    ):
        os.utime(home / rel, (VSCODE_GRANTS_MTIME, VSCODE_GRANTS_MTIME))


# Session sync: Copilot Chat's cloudSessions.json, naming the
# fixture session and one uploaded from another machine, and the
# extension's global state with its cloud agent tasks, in an Insiders data
# directory (research/vscode.md, Other stores).
VSCODE_CLOUD_SESSIONS_REL = ".config/Code/User/globalStorage/github.copilot-chat/cloudSessions.json"
VSCODE_CLOUD_SESSIONS_MTIME = 1759400100  # a year before VSCODE_T0, 2 October 10:15:00 UTC
VSCODE_OTHER_MACHINE_SESSION = "9ab0c1d2-0000-4000-8000-00000000a1b2"
VSCODE_STATE_REL = ".config/Code - Insiders/User/globalStorage/state.vscdb"
VSCODE_CLOUD_TASK = "1234567"
VSCODE_OLD_TASK = "1234000"


def build_vscode_sync(home: Path) -> None:
    _json(
        home / VSCODE_CLOUD_SESSIONS_REL,
        {
            VSCODE_SESSION: {"cloudSessionId": "cs-0001", "cloudTaskId": "ct-0001"},
            VSCODE_OTHER_MACHINE_SESSION: {"cloudSessionId": "cs-0002"},
        },
    )
    os.utime(
        home / VSCODE_CLOUD_SESSIONS_REL,
        (VSCODE_CLOUD_SESSIONS_MTIME, VSCODE_CLOUD_SESSIONS_MTIME),
    )
    _vscdb(
        home / VSCODE_STATE_REL,
        {
            "GitHub.copilot-chat": {
                "github.copilot.cloudAgent.ownedTasks": {
                    VSCODE_CLOUD_TASK: {"recordedAt": 1759400200000, "application": "vscode"},
                    VSCODE_OLD_TASK: 1759300000000,
                },
                "github.copilot.someOtherKey": "not read",
                # copilotCloudSessionsProvider.ts:416-419 at 4d22f4f
                "userSelectedRepositories": [
                    {"name": "octocat/hello-world", "timestamp": 1791025000000}
                ],
            },
            'secret://{"extensionId":"github.copilot-chat","key":"token"}': "gho_not_real",
        },
    )


# Agents window and multi-root workspaces. The Agents window
# opens the workspace file `User/agent-sessions.code-workspace` in the
# editor's own data directory (microsoft/vscode at 4d22f4f,
# src/vs/platform/environment/common/environmentService.ts:280-281), so
# its chats would be stored under that workspace's storage hash with
# `workspace.json` `{"workspace": <that file's URI>}`; the hash is inferred
# from the store's rule, not seen on an install. The chat's real project
# is its `workingDirectory`. The second workspace is a user's multi-root
# workspace file and a session that records no `workingDirectory`.
VSCODE_AGENTS_WORKSPACE = ".config/Code/User/workspaceStorage/a9e47500"
VSCODE_AGENTS_CWD = "/srv/agentproj"
VSCODE_MULTIROOT_WORKSPACE = ".config/Code/User/workspaceStorage/b0071000"
VSCODE_MULTIROOT_FILE = "/srv/team/team.code-workspace"


def build_vscode_agents_window(home: Path) -> None:
    workspace = home / VSCODE_AGENTS_WORKSPACE
    _jsonl(
        workspace / "chatSessions" / (VSCODE_SESSION + ".jsonl"),
        vscode_log_records(cwd=VSCODE_AGENTS_CWD),
    )
    agents_file = home / ".config/Code/User/agent-sessions.code-workspace"
    _json(agents_file, {"folders": []})
    _json(workspace / "workspace.json", {"workspace": agents_file.as_uri()})
    multi = home / VSCODE_MULTIROOT_WORKSPACE
    session = vscode_legacy_session()
    del session["workingDirectory"]
    _json(multi / "chatSessions" / (VSCODE_LEGACY_SESSION + ".json"), session)
    _json(multi / "workspace.json", {"workspace": "file://" + VSCODE_MULTIROOT_FILE})
