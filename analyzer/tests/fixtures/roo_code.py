"""Synthetic Roo Code state for the analyzer tests."""

from __future__ import annotations

import json
from pathlib import Path

from .cline import CLINE_T0
from .common import _json, _vscdb

ROO_TASK = "8f1c2b7e-0000-4000-8000-000000000001"
ROO_CLI_TASK = "8f1c2b7e-0000-4000-8000-000000000002"
ROO_GONE_TASK = "8f1c2b7e-0000-4000-8000-000000000003"


def roo_ui_records(start=CLINE_T0 + 200000):
    return [
        {"ts": start, "type": "say", "say": "text", "text": "rename the helper"},
        {
            "ts": start + 1000,
            "type": "say",
            "say": "api_req_started",
            "text": json.dumps(
                {
                    "request": "rename the helper",
                    "tokensIn": 900,
                    "tokensOut": 40,
                    "cost": 0.002,
                    "apiProtocol": "anthropic",
                }
            ),
        },
        {
            "ts": start + 2000,
            "type": "ask",
            "ask": "tool",
            "text": json.dumps(
                {"tool": "appliedDiff", "path": "src/util.py", "diff": "-old\n+new"}
            ),
            # approved, by a click or automatically (Task.ts:1489-1502)
            "isAnswered": True,
        },
        {"ts": start + 3000, "type": "ask", "ask": "command", "text": "pytest -q"},
        {"ts": start + 4000, "type": "ask", "ask": "command_output", "text": "1 passed"},
        {
            "ts": start + 5000,
            "type": "say",
            "say": "condense_context",
            "partial": False,
            "contextCondense": {
                "cost": 0.001,
                "prevContextTokens": 9000,
                "newContextTokens": 1200,
                "summary": "Renamed helper; tests pass.",
                "condenseId": "c1",
            },
        },
        {
            "ts": start + 6000,
            "type": "ask",
            "ask": "followup",
            "text": json.dumps(
                {"question": "Commit now?", "suggest": [{"answer": "yes"}, {"answer": "no"}]}
            ),
        },
        {"ts": start + 7000, "type": "say", "say": "user_feedback", "text": "yes"},
        {
            "ts": start + 8000,
            "type": "say",
            "say": "completion_result",
            "text": "Renamed and committed.",
        },
    ]


def roo_api_records(start=CLINE_T0 + 300000):
    return [
        {
            "role": "user",
            "content": [{"type": "text", "text": "<task>fix tests</task>"}],
            "ts": start,
        },
        {
            "type": "reasoning",
            "summary": [{"type": "summary_text", "text": "check the runner"}],
            "encrypted_content": "gAAAA",
            "ts": start + 1000,
        },
        {
            "role": "assistant",
            "content": [
                {
                    "type": "tool_use",
                    "id": "toolu_01A",
                    "name": "execute_command",
                    "input": {"command": "pytest -q"},
                }
            ],
            "ts": start + 2000,
        },
        {
            "role": "user",
            "content": [
                {
                    "type": "tool_result",
                    "tool_use_id": "toolu_01A",
                    "content": "3 passed",
                    "is_error": False,
                }
            ],
            "ts": start + 3000,
        },
        {
            "role": "assistant",
            "content": [
                {
                    "type": "tool_use",
                    "id": "toolu_01B",
                    "name": "write_to_file",
                    "input": {"path": "conftest.py", "content": "x"},
                },
                {
                    "type": "tool_use",
                    "id": "toolu_01C",
                    "name": "read_file",
                    "input": {"path": "pytest.ini"},
                },
            ],
            "ts": start + 4000,
        },
        {
            "role": "user",
            "content": [
                # formatResponse.toolDeniedWithFeedback and
                # toolApprovedWithFeedback (responses.ts:8-24)
                {
                    "type": "tool_result",
                    "tool_use_id": "toolu_01B",
                    "content": '{"status":"denied","feedback":"leave conftest alone"}',
                },
                {
                    "type": "tool_result",
                    "tool_use_id": "toolu_01C",
                    "content": '{"status":"approved","feedback":"ok"}\n\n[pytest]',
                },
            ],
            "ts": start + 5000,
        },
    ]


def roo_history_item(task_id, timestamp, task, workspace="/srv/proj"):
    return {
        "id": task_id,
        "number": 1,
        "ts": timestamp,
        "task": task,
        "tokensIn": 1200,
        "tokensOut": 80,
        "totalCost": 0.0041,
        "workspace": workspace,
        "mode": "code",
        "status": "completed",
    }


# The Roo CLI's input history (`.roo/cli-history.json`): text
# only, so "rename the helper" and the follow-up "yes" pair with the task's
# own user rows on text alone (any task: the file names none); "/new" (a
# CLI command that never reaches a task) and "summarise the open issues"
# (its task directory is gone) are kept. The number and the blank entry are
# skipped, as the CLI's loadHistory skips them.
ROO_CLI_HISTORY_ENTRIES = [
    "rename the helper",
    "yes",
    "/new",
    42,
    "  ",
    "summarise the open issues",
]


def build_roo_cli_history(home: Path) -> None:
    roo = home / ".roo"
    _json(roo / "cli-history.json", {"version": 1, "entries": ROO_CLI_HISTORY_ENTRIES})
    _json(
        roo / "cli-settings.json",
        {"mode": "code", "provider": "anthropic", "requireApproval": True},
    )


def build_roo_code(home: Path) -> None:
    global_storage = home / ".config/Code/User/globalStorage/rooveterinaryinc.roo-cline"
    task = global_storage / "tasks" / ROO_TASK
    _json(task / "ui_messages.json", roo_ui_records())
    _json(task / "api_conversation_history.json", roo_api_records())
    _json(
        task / "history_item.json",
        roo_history_item(ROO_TASK, CLINE_T0 + 208000, "rename the helper"),
    )
    _json(
        task / "task_metadata.json",
        {
            "files_in_context": [
                {
                    "path": "src/util.py",
                    "record_state": "active",
                    "record_source": "roo_edited",
                    "roo_read_date": CLINE_T0 + 202000,
                    "roo_edit_date": CLINE_T0 + 202500,
                    "user_edit_date": None,
                }
            ]
        },
    )
    _json(
        global_storage / "tasks/_index.json",
        {
            "version": 1,
            "updatedAt": CLINE_T0 + 208000,
            "entries": [
                roo_history_item(ROO_TASK, CLINE_T0 + 208000, "rename the helper"),
                roo_history_item(ROO_GONE_TASK, CLINE_T0 - 3600000, "a task deleted from disk"),
            ],
        },
    )
    mock = home / ".vscode-mock/global-storage"
    _json(mock / "tasks" / ROO_CLI_TASK / "api_conversation_history.json", roo_api_records())
    _json(
        mock / "tasks" / ROO_CLI_TASK / "history_item.json",
        roo_history_item(ROO_CLI_TASK, CLINE_T0 + 303000, "fix tests"),
    )
    _json(mock / "secrets.json", {"roo_cline_config_api_config": '{"apiKey":"sk-not-real"}'})
    _json(mock / "global-state.json", roo_global_state())
    _json(
        home / ROO_WORKSPACE_STATE_REL,
        {"someWorkspaceKey": 1, ROO_RETRY_QUEUE: roo_retry_queue()},
    )
    _json(global_storage / "settings/mcp_settings.json", roo_mcp_settings())
    # A project's MCP file; in a project home its rows carry the project.
    _json(home / ".roo/mcp.json", roo_mcp_settings())
    _vscdb(home / ROO_VSCDB_REL, {ROO_EXTENSION_KEY: roo_global_state()})
    _vscdb(
        home / ROO_WORKSPACE_VSCDB_REL, {ROO_EXTENSION_KEY: {ROO_RETRY_QUEUE: roo_retry_queue()}}
    )


# Roo Code's global state (research/roo-code.md, Permission rows), as the
# extension keeps it in its state.vscdb row and the CLI's shim in
# global-state.json, with the Roo Code Cloud settings cache of releases up
# to 3.53.0 (Sync, relay and origin rows), and the cloud client's retry
# queue in the workspace state.
ROO_EXTENSION_KEY = "RooVeterinaryInc.roo-cline"
ROO_VSCDB_REL = ".config/Code - OSS/User/globalStorage/state.vscdb"
ROO_WORKSPACE_VSCDB_REL = ".config/Code - OSS/User/workspaceStorage/r00c0de/state.vscdb"
ROO_WORKSPACE_STATE_REL = ".vscode-mock/workspace-storage/5f3a2c1d/workspace-state.json"
ROO_RETRY_QUEUE = "roo.retryQueue"
ROO_CLOUD_TOKEN = "eyJ-not-a-real-session-token"


def roo_global_state():
    return {
        "autoApprovalEnabled": True,
        "alwaysAllowReadOnly": True,
        "alwaysAllowExecute": False,
        "allowedCommands": ["git status", "npm test"],
        "deniedCommands": ["rm -rf"],
        "mode": "architect",
        "roo-cline.allowedCommands": ["git status"],
        "customInstructions": "not a permission",
        "user-settings": {"features": {}, "settings": {"taskSyncEnabled": True}, "version": 3},
        "organization-settings": {
            "version": 1,
            "cloudSettings": {"recordTaskMessages": True, "enableTaskSharing": False},
            "defaultSettings": {},
            "allowList": {"allowAll": True, "providers": {}},
        },
    }


def roo_retry_queue():
    """One queued Roo Code Task Message upload.

    One queued Task Message upload (RetryQueue.ts:73-95 and TelemetryClient.ts:102-153 at Roo-Code
    v3.53.0); its headers and body must never reach a row.
    """
    message = {"ts": CLINE_T0 + 200000, "type": "say", "say": "text", "text": "rename the helper"}
    return [
        {
            "id": "1791021800000-abc123def",
            "url": "https://app.roocode.com/api/events",
            "options": {
                "method": "POST",
                "body": json.dumps(
                    {
                        "type": "Task Message",
                        "properties": {"taskId": ROO_TASK, "message": message},
                    }
                ),
                "headers": {
                    "Authorization": "Bearer " + ROO_CLOUD_TOKEN,
                    "Content-Type": "application/json",
                },
            },
            "timestamp": CLINE_T0 + 200500,
            "retryCount": 2,
            "type": "telemetry",
            "lastError": "fetch failed",
        }
    ]


def roo_mcp_settings():
    return {
        "mcpServers": {
            "github": {
                "command": "npx",
                "args": ["-y", "github-mcp"],
                "env": {"GITHUB_TOKEN": "ghp-not-a-real-token"},
                "alwaysAllow": ["list_issues", "get_issue"],
            },
            "fs": {"command": "fs-mcp", "alwaysAllow": []},
        }
    }
