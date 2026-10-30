"""Synthetic Cline state for the analyzer tests."""

from __future__ import annotations

import json
import os
import sqlite3
from pathlib import Path

from .common import _json

# ---- Cline and Roo Code (research/cline.md, research/roo-code.md) ----------------

CLINE_TASK = "1791021600000"  # 3 October 10:00:00 UTC, a Date.now() id
CLINE_CLI_TASK = "1791021300000"  # API history only, no ui_messages.json
CLINE_SESSION = "1791021700000_k3x9q"
CLINE_OLD_SESSION = "1791000000000_old01"


CLINE_T0 = 1791021600000


def cline_ui_records(start=CLINE_T0):
    model = {"modelId": "claude-sonnet-4-5", "providerId": "anthropic", "mode": "act"}
    return [
        {
            "ts": start,
            "type": "say",
            "say": "task",
            "text": "fix tests",
            "images": ["data:image/png;base64,AAAA"],
        },
        {
            "ts": start + 1000,
            "type": "say",
            "say": "api_req_started",
            "text": json.dumps(
                {
                    "request": "<task>fix tests</task>",
                    "tokensIn": 1200,
                    "tokensOut": 80,
                    "cacheWrites": 0,
                    "cacheReads": 900,
                    "cost": 0.0041,
                }
            ),
        },
        {
            "ts": start + 1500,
            "type": "say",
            "say": "reasoning",
            "text": "read the test first",
            "partial": False,
        },
        {
            "ts": start + 2000,
            "type": "say",
            "say": "tool",
            "modelInfo": model,
            "text": json.dumps(
                {"tool": "readFile", "path": "src/app.py", "content": "/srv/proj/src/app.py"}
            ),
        },
        {
            "ts": start + 3000,
            "type": "say",
            "say": "text",
            "text": "Running the suite.",
            "modelInfo": model,
        },
        {
            "ts": start + 4000,
            "type": "ask",
            "ask": "command",
            "text": "pytest -q",
            "modelInfo": model,
        },
        {"ts": start + 5000, "type": "say", "say": "command_output", "text": "3 passed"},
        {
            "ts": start + 6000,
            "type": "say",
            "say": "checkpoint_created",
            "lastCheckpointHash": "abc123",
        },
        {
            "ts": start + 7000,
            "type": "say",
            "say": "completion_result",
            "text": "All tests pass.",
            "modelInfo": model,
        },
        {"ts": start + 7001, "type": "ask", "ask": "completion_result", "text": ""},
    ]


def cline_api_records():
    return [
        {"role": "user", "content": [{"type": "text", "text": "<task>fix tests</task>"}]},
        {
            "role": "assistant",
            "content": [
                {"type": "thinking", "thinking": "run the suite"},
                {
                    "type": "tool_use",
                    "id": "toolu_01A",
                    "name": "execute_command",
                    "input": {"command": "pytest -q"},
                },
            ],
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
        },
    ]


def cline_task_metadata(start=CLINE_T0):
    return {
        "files_in_context": [
            {
                "path": "src/app.py",
                "record_state": "active",
                "record_source": "read_tool",
                "cline_read_date": start + 2000,
                "cline_edit_date": None,
            },
        ],
        "model_usage": [
            {
                "ts": start + 300,
                "model_id": "claude-sonnet-4-5",
                "model_provider_id": "anthropic",
                "mode": "plan",
            },
            {
                "ts": start + 500,
                "model_id": "claude-sonnet-4-5",
                "model_provider_id": "anthropic",
                "mode": "act",
            },
            {
                "ts": start + 8000,
                "model_id": "claude-opus-4-5",
                "model_provider_id": "anthropic",
                "mode": "act",
            },
        ],
        "environment_history": [
            {
                "ts": start + 100,
                "os_name": "linux",
                "os_version": "6.6",
                "os_arch": "x64",
                "host_name": "Visual Studio Code",
                "host_version": "1.105.0",
                "cline_version": "3.40.0",
            }
        ],
    }


def cline_history_items():
    return [
        {
            "id": CLINE_TASK,
            "ulid": "01JAX3Q8M4",
            "ts": CLINE_T0 + 7001,
            "task": "fix tests",
            "tokensIn": 1200,
            "tokensOut": 80,
            "totalCost": 0.0041,
            "cwdOnTaskInitialization": "/srv/proj",
            "modelId": "claude-sonnet-4-5",
            "apiProvider": "anthropic",
        },
    ]


def cline_sdk_manifest():
    return {
        "version": 1,
        "session_id": CLINE_SESSION,
        "source": "cli",
        "pid": 4242,
        "started_at": "2026-10-03T10:01:40.000Z",
        "ended_at": "2026-10-03T10:01:50.000Z",
        "exit_code": 0,
        "status": "completed",
        "interactive": True,
        "provider": "anthropic",
        "model": "claude-sonnet-4-5",
        "cwd": "/srv/proj",
        "workspace_root": "/srv/proj",
        "enable_tools": True,
        "enable_spawn": False,
        "enable_teams": False,
        "prompt": "fix tests",
        "metadata": {"title": "fix tests"},
        "messages_path": "/home/u/.cline/data/sessions/%s/%s.messages.json"
        % (CLINE_SESSION, CLINE_SESSION),
    }


def cline_sdk_messages(start=CLINE_T0 + 100000):
    return {
        "version": 1,
        "updated_at": "2026-10-03T10:01:50.000Z",
        "agent": "lead",
        "sessionId": CLINE_SESSION,
        "origin": {"source": "cli", "mode": "user", "sessionId": CLINE_SESSION},
        "system_prompt": "You are Cline.",
        "messages": [
            {"id": "m1", "role": "user", "content": "fix tests", "ts": start},
            {
                "id": "m2",
                "role": "assistant",
                "content": [
                    {"type": "thinking", "thinking": "run the suite"},
                    {
                        "type": "tool_use",
                        "id": "call_1",
                        "name": "bash",
                        "input": {"command": "pytest -q"},
                    },
                ],
                "modelInfo": {"id": "claude-sonnet-4-5", "provider": "anthropic"},
                "metrics": {"inputTokens": 1200, "outputTokens": 80, "cost": 0.0041},
                "ts": start + 2000,
            },
            {
                "id": "m3",
                "role": "user",
                "content": [
                    {
                        "type": "tool_result",
                        "tool_use_id": "call_1",
                        "name": "bash",
                        "content": "3 passed",
                        "is_error": False,
                    }
                ],
                "ts": start + 3000,
            },
            {
                "id": "m4",
                "role": "assistant",
                "content": [{"type": "text", "text": "Fixed."}],
                "modelInfo": {"id": "claude-opus-4-5", "provider": "anthropic"},
            },
        ],
    }


def build_cline(home: Path) -> None:
    global_storage = home / ".config/Code/User/globalStorage/saoudrizwan.claude-dev"
    task = global_storage / "tasks" / CLINE_TASK
    _json(task / "ui_messages.json", cline_ui_records())
    _json(task / "api_conversation_history.json", cline_api_records())
    _json(task / "task_metadata.json", cline_task_metadata())
    _json(global_storage / "state/taskHistory.json", cline_history_items())
    _json(global_storage / "settings/cline_mcp_settings.json", {"mcpServers": {}})
    data = home / ".cline/data"
    _json(data / "tasks" / CLINE_CLI_TASK / "api_conversation_history.json", cline_api_records())
    _json(
        data / "state/taskHistory.json",
        [dict(cline_history_items()[0], id=CLINE_CLI_TASK, ts=1791021305000)],
    )
    _json(data / "secrets.json", {"apiKey": "sk-not-real"})
    session_dir = data / "sessions" / CLINE_SESSION
    _json(session_dir / (CLINE_SESSION + ".json"), cline_sdk_manifest())
    _json(session_dir / (CLINE_SESSION + ".messages.json"), cline_sdk_messages())
    _json(session_dir / (CLINE_SESSION + ".compaction.json"), {"version": 1})
    (data / "db").mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(data / "db/sessions.db"))
    connection.execute(
        "CREATE TABLE sessions (session_id TEXT PRIMARY KEY, source TEXT, pid INTEGER, started_at TEXT, "
        "ended_at TEXT, exit_code INTEGER, status TEXT, interactive INTEGER, provider TEXT, model TEXT, "
        "cwd TEXT, workspace_root TEXT, prompt TEXT, metadata_json TEXT, is_subagent INTEGER, "
        "parent_session_id TEXT, updated_at TEXT)"
    )
    for session_id, started, prompt in (
        (CLINE_SESSION, "2026-10-03T10:01:40.000Z", "fix tests"),
        (CLINE_OLD_SESSION, "2026-10-02T23:00:00.000Z", "an older deleted session"),
    ):
        connection.execute(
            "INSERT INTO sessions VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
            (
                session_id,
                "cli",
                4242,
                started,
                None,
                None,
                "completed",
                1,
                "anthropic",
                "claude-sonnet-4-5",
                "/srv/proj",
                "/srv/proj",
                prompt,
                json.dumps({"title": prompt}),
                0,
                None,
                started,
            ),
        )
    connection.commit()
    connection.close()


def cline_search_text(message: dict) -> str:
    """A message as session-search.db indexes it.

    Every string value under `content`, skipping the `data`, `image`, `images`, `reasoning`,
    `thinking` and `metadata` keys, joined with line breaks (session-history-search.ts:54-95 at
    39ff2359).
    """
    parts: list[str] = []

    def walk(value) -> None:
        if isinstance(value, str):
            if value:
                parts.append(value)
        elif isinstance(value, list):
            for item in value:
                walk(item)
        elif isinstance(value, dict):
            for key, item in value.items():
                if key not in ("data", "image", "images", "reasoning", "thinking", "metadata"):
                    walk(item)

    walk(message.get("content", message))
    return "\n".join(parts).strip()


def cline_session_search_sessions():
    """(session id, started_at, workspace_root, title, prompt, messages) per indexed session.

    The fixture's SDK session, whose transcript is on disk, and the older session whose manifest and
    transcript are gone.
    """
    return [
        (
            CLINE_SESSION,
            "2026-10-03T10:01:40.000Z",
            "/srv/proj",
            "fix tests",
            "fix tests",
            cline_sdk_messages()["messages"],
        ),
        (
            CLINE_OLD_SESSION,
            "2026-10-02T23:00:00.000Z",
            "/srv/proj",
            "an older deleted session",
            "an older deleted session",
            [
                {"role": "user", "content": "prune the stale branches"},
                {
                    "role": "assistant",
                    "content": [{"type": "text", "text": "Deleted two branches."}],
                },
            ],
        ),
    ]


CLINE_SEARCH_FTS = (
    "CREATE VIRTUAL TABLE session_search USING fts5(session_id UNINDEXED, document_id "
    "UNINDEXED, ordinal UNINDEXED, role UNINDEXED, started_at UNINDEXED, workspace_root, "
    "title, content, tokenize='unicode61 remove_diacritics 2 tokenchars ''_-@''')"
)


def build_cline_session_search(home: Path) -> None:
    """Write `~/.cline/data/db/session-search.db` and `cron.db` beside it.

    `~/.cline/data/db/session-search.db` as the SDK hub writes it (session-history-search.ts:120-154
    and 402-465 at 39ff2359), and `cron.db` beside it, which the parser must not want.
    """
    database_dir = home / ".cline/data/db"
    database_dir.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(database_dir / "session-search.db"))
    connection.execute(
        "CREATE TABLE indexed_sessions (session_id TEXT PRIMARY KEY, source_revision TEXT NOT "
        "NULL, indexed_at TEXT NOT NULL, document_count INTEGER NOT NULL, index_version INTEGER "
        "NOT NULL, title TEXT NOT NULL DEFAULT '')"
    )
    connection.execute(CLINE_SEARCH_FTS)
    insert = "INSERT INTO session_search VALUES (?,?,?,?,?,?,?,?)"
    for session_id, started, workspace, title, prompt, messages in cline_session_search_sessions():
        connection.execute(
            insert,
            (
                session_id,
                session_id + ":metadata",
                -1,
                "session",
                started,
                workspace,
                title,
                prompt,
            ),
        )
        count = 1
        for ordinal, message in enumerate(messages):
            content = cline_search_text(message)
            if not content:
                continue
            connection.execute(
                insert,
                (
                    session_id,
                    "%s:%d" % (session_id, ordinal),
                    ordinal,
                    message.get("role") or "unknown",
                    started,
                    workspace,
                    "",
                    content,
                ),
            )
            count += 1
        connection.execute(
            "INSERT INTO indexed_sessions VALUES (?,?,?,?,?,?)",
            (
                session_id,
                started + ":1024:1791021710000",
                "2026-10-03T10:02:00.000Z",
                count,
                2,
                title,
            ),
        )
    connection.commit()
    connection.close()
    connection = sqlite3.connect(str(database_dir / "cron.db"))
    connection.execute("CREATE TABLE cron_jobs (id TEXT PRIMARY KEY, prompt TEXT)")
    connection.commit()
    connection.close()


def build_cline_input_history(home: Path) -> None:
    """Write the Cline CLI's `user_input_history.jsonl` and a model cache.

    The CLI's `~/.cline/data/cache/user_input_history.jsonl`
    (apps/cli/src/utils/input-history.ts:54-72 at 39ff2359): every line stamped with the last
    rewrite's time. "fix tests" repeats the SDK session's prompt; "list the open ports" has no
    transcript. A model cache beside it is noise the parser must not want.
    """
    cache = home / ".cline/data/cache"
    cache.mkdir(parents=True, exist_ok=True)
    rewrite = CLINE_T0 + 300000  # 3 October 10:05:00 UTC
    lines = [{"prompt": prompt, "ts": rewrite} for prompt in ("fix tests", "list the open ports")]
    (cache / "user_input_history.jsonl").write_text(
        "".join(json.dumps(line) + "\n" for line in lines), encoding="utf-8"
    )
    _json(cache / "openrouter_models.json", {"anthropic/claude-sonnet-4-5": {}})


# Enterprise prompt upload: `metadata["enterprise.blobUpload"]`
# (sdk/packages/shared/src/remote-config/blob-storage.ts:56-57, 129-160 at
# 39ff2359). CLINE_BLOB_SESSION's manifest has the VS Code host's form, with
# `userDistinctId`; CLINE_BLOB_DELETED survives only as a sessions.db row
# with the CLI's form, without it. Not part of `build_home`: the tests that
# need them add them, so the session counts of the other Cline tests stay.
CLINE_BLOB_SESSION = "1791022000000_b10b5"
CLINE_BLOB_DELETED = "1791021900000_d31e7"


def cline_blob_upload(user: str = "usr_0f9e8d7c"):
    blob: dict = {
        "version": 1,
        "storage": {"adapterType": "s3", "bucket": "acme-cline-prompts", "region": "us-east-1"},
    }
    if user:
        blob["userDistinctId"] = user
    return blob


def build_cline_blob_upload(home: Path) -> None:
    data = home / ".cline/data"
    manifest = dict(
        cline_sdk_manifest(),
        session_id=CLINE_BLOB_SESSION,
        started_at="2026-10-03T10:06:40.000Z",
        ended_at=None,
        prompt="summarise the diff",
        metadata={"title": "summarise the diff", "enterprise.blobUpload": cline_blob_upload()},
    )
    _json(data / "sessions" / CLINE_BLOB_SESSION / (CLINE_BLOB_SESSION + ".json"), manifest)
    blob = dict(
        cline_blob_upload(""),
        storage={
            "adapterType": "r2",
            "bucket": "acme-r2",
            "endpoint": "https://acct.r2.cloudflarestorage.com",
            "accountId": "acct",
        },
    )
    connection = sqlite3.connect(str(data / "db/sessions.db"))
    connection.execute(
        "INSERT INTO sessions (session_id, source, started_at, status, provider, model, cwd,"
        " workspace_root, prompt, metadata_json) VALUES (?,?,?,?,?,?,?,?,?,?)",
        (
            CLINE_BLOB_DELETED,
            "cli",
            "2026-10-03T10:05:00.000Z",
            "completed",
            "anthropic",
            "claude-sonnet-4-5",
            "/srv/proj",
            "/srv/proj",
            "a deleted uploaded session",
            json.dumps({"title": "uploaded", "enterprise.blobUpload": blob}),
        ),
    )
    connection.commit()
    connection.close()


# The VS Code extension's enterprise prompt upload queue, the
# `SyncQueueItem` shape of apps/vscode/src/shared/services/worker/queue.ts:9-31
# at 39ff2359, one item of each status: CLINE_TASK's transcript uploaded,
# CLINE_QUEUE_PENDING's waiting and CLINE_QUEUE_FAILED's failing. The two
# queued-only tasks have no task directory.
CLINE_QUEUE_PENDING = "1791021700001"
CLINE_QUEUE_FAILED = "1791021700002"
CLINE_SYNC_QUEUE = ".config/Code/User/globalStorage/saoudrizwan.claude-dev/cache/sync-queue.json"
CLINE_SYNC_SECRET = "FULL TASK FILE CONTENT"


def build_cline_sync_queue(home: Path) -> None:
    items = {}
    for task, key, status, retries, timestamp, error in (
        (CLINE_TASK, "api_conversation_history.json", "synced", 0, CLINE_T0 + 5000, None),
        (CLINE_QUEUE_PENDING, "ui_messages.json", "pending", 0, CLINE_T0 + 5500, None),
        (CLINE_QUEUE_FAILED, "ui_messages.json", "failed", 3, CLINE_T0 + 6000, "403 Forbidden"),
    ):
        items[task + "/" + key] = {
            "id": task + "/" + key,
            "taskId": task,
            "key": key,
            "data": CLINE_SYNC_SECRET,
            "timestamp": timestamp,
            "status": status,
            "retryCount": retries,
            "lastError": error,
        }
    _json(home / CLINE_SYNC_QUEUE, {"items": items})


# From cline/cline at 41e75efc: the hook audit log
# (sdk/packages/core/src/hooks/hook-file-hooks.ts:234-254, 629-756), the
# standing permission settings (apps/vscode/src/shared/AutoApprovalSettings.ts:1-44,
# sdk/packages/core/src/services/global-settings.ts:61-74,
# apps/vscode/src/services/mcp/schemas.ts:10-28), the Cline Cloud state
# (apps/vscode/src/shared/cloud/cloud-sessions.ts:15-52, cloud-pending-starts.ts:5-12),
# a rejected and a blocked SDK call (sdk/packages/agents/src/agent-runtime.ts:2469-2512)
# and an imported session (sdk/packages/core/src/services/session-import/service.ts:20-28).
# Not part of `build_home`, so the session counts of the other Cline tests stay.
CLINE_HOOKS_LOG = ".cline/data/logs/hooks.jsonl"
CLINE_GLOBAL_STATE = ".cline/data/globalState.json"
CLINE_GLOBAL_SETTINGS = ".cline/data/settings/global-settings.json"
CLINE_MCP_SETTINGS = ".cline/data/settings/cline_mcp_settings.json"
CLINE_CLOUD_SESSION = "ses-01JCLOUDDONE"
CLINE_PENDING_SESSION = "ses-01JCLOUDWAIT"
CLINE_PENDING_START = (
    ".cline/data/cloud-pending-starts/4242-0b6f2c1e-6a1d-4c51-9a7e-2f3d1c0e9b8a.json"
)
CLINE_SETTINGS_MTIME = 1791022200  # 3 October 10:10:00 UTC
CLINE_DENIED_SESSION = "1791022100000_den1a"
CLINE_IMPORTED_SESSION = "1791022300000_imp0r"
CLINE_REJECTION_SUFFIX = " -- NOT a tool or system failure. Clarify with user before proceeding."


def _hook_line(hook: str, timestamp: str, session: str, **extra) -> dict:
    line = {
        "ts": timestamp,
        "clineVersion": "4.1.25",
        "timestamp": timestamp,
        "taskId": session,
        "sessionContext": {"rootSessionId": session},
        "workspaceRoots": ["/srv/proj"],
        "userId": "alice",
        "agent_id": "agent_lead",
        "hookName": hook,
    }
    line.update(extra)
    return line


def cline_hook_lines():
    """The Cline hook audit log lines of two sessions and a stale one.

    CLINE_SESSION's turns as the audit log repeats them, then the tool call and abort of
    CLINE_OLD_SESSION, whose transcript is gone, and a stale session's shutdown line.
    """
    session, old = CLINE_SESSION, CLINE_OLD_SESSION
    call = {"id": "call_1", "name": "bash", "input": {"command": "pytest -q"}}
    return [
        _hook_line(
            "agent_start", "2026-10-03T10:01:40.050Z", session, taskStart={"taskMetadata": {}}
        ),
        _hook_line(
            "prompt_submit",
            "2026-10-03T10:01:40.100Z",
            session,
            userPromptSubmit={"prompt": "fix tests", "attachments": []},
        ),
        _hook_line(
            "tool_call",
            "2026-10-03T10:01:42.500Z",
            session,
            iteration=1,
            tool_call=call,
            preToolUse={"toolName": "bash", "parameters": {"command": "pytest -q"}},
        ),
        _hook_line(
            "tool_result",
            "2026-10-03T10:01:42.900Z",
            session,
            iteration=1,
            tool_result=dict(
                call,
                output="3 passed",
                durationMs=400,
                startedAt="2026-10-03T10:01:42.500Z",
                endedAt="2026-10-03T10:01:42.900Z",
            ),
            postToolUse={
                "toolName": "bash",
                "parameters": {"command": "pytest -q"},
                "result": "3 passed",
                "success": True,
                "executionTimeMs": 400,
            },
        ),
        _hook_line(
            "agent_end",
            "2026-10-03T10:01:49.000Z",
            session,
            iteration=2,
            turn={"outputText": "Fixed.", "status": "completed"},
            taskComplete={"taskMetadata": {}},
        ),
        _hook_line(
            "tool_call",
            "2026-10-02T23:00:05.000Z",
            old,
            iteration=1,
            tool_call={"id": "call_9", "name": "bash", "input": {"command": "git branch -D stale"}},
        ),
        _hook_line(
            "tool_result",
            "2026-10-02T23:00:06.000Z",
            old,
            iteration=1,
            tool_result={
                "id": "call_9",
                "name": "bash",
                "input": {"command": "git branch -D stale"},
                "output": "",
                "error": "exit code 1",
                "durationMs": 900,
            },
            postToolUse={"toolName": "bash", "result": "", "success": False},
        ),
        _hook_line(
            "agent_abort",
            "2026-10-02T23:00:07.000Z",
            old,
            reason="aborted by user",
            taskCancel={"taskMetadata": {}},
        ),
        {
            "ts": "2026-10-03T10:09:00.000Z",
            "hookName": "session_shutdown",
            "reason": "stale_process",
            "sessionId": old,
            "pid": 4100,
            "source": "cli",
        },
    ]


def build_cline_hooks_log(home: Path) -> None:
    path = home / CLINE_HOOKS_LOG
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        "".join(json.dumps(line) + "\n" for line in cline_hook_lines()), encoding="utf-8"
    )


def cline_global_state() -> dict:
    return {
        "autoApprovalSettings": {
            "version": 3,
            "enabled": True,
            "favorites": [],
            "maxRequests": 20,
            "actions": {
                "readFiles": True,
                "editFiles": True,
                "executeSafeCommands": False,
                "executeAllCommands": True,
                "useBrowser": False,
                "useMcp": True,
            },
            "enableNotifications": False,
        },
        "mode": "act",
        "apiProvider": "anthropic",  # not a permission: never read
        "cloudTaskTarget": {"target": "cloud", "repositoryId": 812345678, "branch": "main"},
        "cloudSessionStatuses": {
            CLINE_CLOUD_SESSION: {"status": "completed", "observedAt": 1791021800000},
        },
    }


def build_cline_settings_and_cloud(home: Path) -> None:
    """Write the Cline settings files and a pending-start file.

    globalState.json, global-settings.json, the MCP settings file and one pending-start file, each
    with `CLINE_SETTINGS_MTIME` as its mtime.
    """
    _json(home / CLINE_GLOBAL_STATE, cline_global_state())
    _json(
        home / CLINE_GLOBAL_SETTINGS,
        {
            "telemetryOptOut": False,
            "autoUpdateEnabled": True,
            "planActMode": "plan",
            "toolAutoApprove": False,
            "disabledTools": ["browser_action"],
            "tools": {"web_fetch": {"enabled": False}},
            "disabledPlugins": ["acme-plugin"],
        },
    )
    _json(
        home / CLINE_MCP_SETTINGS,
        {
            "mcpServers": {
                "github": {
                    "command": "npx",
                    "env": {"GITHUB_TOKEN": "ghp_not_real"},
                    "autoApprove": ["list_issues", "get_issue"],
                    "disabled": False,
                }
            }
        },
    )
    _json(
        home / CLINE_PENDING_START,
        [
            {
                "sessionId": CLINE_PENDING_SESSION,
                "account": "user:usr_0f9e8d7c",
                "endpoint": "https://api.cline.bot",
            }
        ],
    )
    for rel in (CLINE_GLOBAL_STATE, CLINE_GLOBAL_SETTINGS, CLINE_MCP_SETTINGS, CLINE_PENDING_START):
        os.utime(home / rel, (CLINE_SETTINGS_MTIME, CLINE_SETTINGS_MTIME))


def build_cline_denied_and_imported(home: Path) -> None:
    """Write a Cline SDK session with refused calls and an imported one.

    An SDK session with one call the user rejected (with feedback) and one the policy blocked, and a
    session imported from Claude Code.
    """
    sessions = home / ".cline/data/sessions"
    manifest = dict(
        cline_sdk_manifest(),
        session_id=CLINE_DENIED_SESSION,
        started_at="2026-10-03T10:08:20.000Z",
        ended_at=None,
        prompt="clean up",
        metadata={"title": "clean up"},
    )
    _json(sessions / CLINE_DENIED_SESSION / (CLINE_DENIED_SESSION + ".json"), manifest)
    start = 1791022100000
    rejected = (
        "This tool call was rejected by the user and not executed. The user provided the"
        " following feedback:\n<feedback>\nnot that one\n</feedback>"
    )
    messages = {
        "version": 1,
        "updated_at": "2026-10-03T10:08:30.000Z",
        "agent": "lead",
        "sessionId": CLINE_DENIED_SESSION,
        "messages": [
            {"id": "m1", "role": "user", "content": "clean up", "ts": start},
            {
                "id": "m2",
                "role": "assistant",
                "content": [
                    {
                        "type": "tool_use",
                        "id": "call_r",
                        "name": "bash",
                        "input": {"command": "rm -rf build"},
                    },
                    {
                        "type": "tool_use",
                        "id": "call_b",
                        "name": "web_fetch",
                        "input": {"url": "https://example.com"},
                    },
                ],
                "modelInfo": {"id": "claude-sonnet-4-5", "provider": "anthropic"},
                "ts": start + 1000,
            },
            {
                "id": "m3",
                "role": "user",
                "content": [
                    {
                        "type": "tool_result",
                        "tool_use_id": "call_r",
                        "name": "bash",
                        "content": json.dumps({"error": rejected + CLINE_REJECTION_SUFFIX}),
                        "is_error": True,
                    },
                    {
                        "type": "tool_result",
                        "tool_use_id": "call_b",
                        "name": "web_fetch",
                        "content": json.dumps({"error": 'Tool "web_fetch" is disabled by policy'}),
                        "is_error": True,
                    },
                ],
                "ts": start + 2000,
            },
        ],
    }
    _json(sessions / CLINE_DENIED_SESSION / (CLINE_DENIED_SESSION + ".messages.json"), messages)
    imported = dict(
        cline_sdk_manifest(),
        session_id=CLINE_IMPORTED_SESSION,
        source="desktop",
        started_at="2026-10-03T10:11:40.000Z",
        ended_at=None,
        prompt="port the parser",
        metadata={
            "title": "port the parser",
            "importedFrom": {
                "tool": "claude-code",
                "sourceSessionId": "5f0c1d2e-0000-4000-8000-00000000c1a0",
                "sourcePath": "/home/u/.claude/projects/-srv-proj/5f0c1d2e.jsonl",
                "importedAt": "2026-10-03T10:11:45.000Z",
                "sourceProvider": "anthropic",
                "sourceModel": "claude-opus-4-5",
            },
        },
    )
    _json(sessions / CLINE_IMPORTED_SESSION / (CLINE_IMPORTED_SESSION + ".json"), imported)


def build_cline_nested(home: Path) -> None:
    """A Cline extension nested inside VS Code's globalStorage."""
    global_storage = home / ".config/Code/User/globalStorage"
    global_storage.mkdir(parents=True, exist_ok=True)
    (global_storage / "state.vscdb").write_text("sqlite", encoding="utf-8")
    (global_storage / "saoudrizwan.claude-dev/state").mkdir(parents=True, exist_ok=True)
    (global_storage / "saoudrizwan.claude-dev/state/taskHistory.json").write_text(
        "[]", encoding="utf-8"
    )
