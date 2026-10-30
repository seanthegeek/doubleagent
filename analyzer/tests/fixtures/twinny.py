"""Synthetic Twinny state for the analyzer tests."""

from __future__ import annotations

import json
import os
from pathlib import Path

from .common import VSCDB_SCHEMA, _vscdb, _wal_database

TWINNY_CONVERSATION = "7d1c2b3a-1111-4000-8000-000000000001"
TWINNY_ACTIVE = "7d1c2b3a-1111-4000-8000-000000000002"
TWINNY_API_KEY = "sk-twinny-fixture-not-real"
TWINNY_VSCDB_REL = ".vscode-server/data/User/globalStorage/state.vscdb"


def twinny_records():
    """The `rjmacarthy.twinny` global state value.

    The research fixture plus an unsaved active conversation, a failed tool step and provider keys.
    """
    return {
        "twinny.conversations": {
            TWINNY_CONVERSATION: {
                "id": TWINNY_CONVERSATION,
                "title": "List the tests",
                "updatedAt": 1791028805000,
                "messages": [
                    {"role": "user", "content": "which tests fail?"},
                    {
                        "role": "assistant",
                        "content": "<think>run the suite</think>Two tests fail.",
                        "meta": {
                            "model": "qwen2.5-coder:7b",
                            "provider": "Ollama",
                            "durationMs": 5200,
                            "withheld": [{"kind": "aws-key", "count": 1}],
                        },
                        "toolSteps": [
                            {
                                "id": "step-1",
                                "name": "run_command",
                                "summary": "ran `npm test`",
                                "args": {"command": "npm test"},
                                "output": "2 failing",
                                "status": "done",
                            },
                            {
                                "id": "step-2",
                                "name": "read_file",
                                "args": {"path": "src/a.ts"},
                                "output": "permission denied",
                                "status": "failed",
                            },
                        ],
                    },
                    {
                        # A switched model, a step saved while it waited for Run or
                        # Skip, and a skipped one (tool-steps.ts at 9339bd10).
                        "role": "assistant",
                        "content": "Cleaning the build.",
                        "meta": {"model": "qwen2.5-coder:14b", "provider": "Ollama"},
                        "toolSteps": [
                            {
                                "id": "step-3",
                                "name": "run_command",
                                "summary": "wants to run `rm -rf build`",
                                "args": {"command": "rm -rf build"},
                                "command": "rm -rf build",
                                "approval": "command",
                                "status": "waiting",
                            },
                            {
                                "id": "step-4",
                                "name": "delete_file",
                                "summary": "skipped",
                                "args": {"path": "dist"},
                                "output": "skipped by the user",
                                "status": "skipped",
                            },
                        ],
                    },
                ],
            }
        },
        "twinny.active-conversation": {
            "id": TWINNY_ACTIVE,
            "title": "draft",
            "updatedAt": 1791028900000,
            "messages": [{"role": "user", "content": "unsaved question"}],
        },
        "twinny.inference-providers": {
            "p1": {
                "id": "p1",
                "label": "Ollama",
                "provider": "ollama",
                "type": "chat",
                "modelName": "qwen2.5-coder:7b",
                "apiHostname": "localhost",
                "apiPort": 11434,
                "apiKey": TWINNY_API_KEY,
            }
        },
        "twinny.active-chat-provider": {"id": "p1", "apiKey": TWINNY_API_KEY},
    }


def build_twinny(home: Path) -> None:
    """Rows only in the -wal sidecar, as with the editor still running."""

    def lit(text):
        return "'" + text.replace("'", "''") + "'"

    _wal_database(
        home / TWINNY_VSCDB_REL,
        VSCDB_SCHEMA,
        [
            "INSERT INTO ItemTable VALUES ('rjmacarthy.twinny', CAST(%s AS BLOB))"
            % lit(json.dumps(twinny_records())),
            "INSERT INTO ItemTable VALUES (%s, %s)"
            % (
                lit('secret://{"extensionId":"rjmacarthy.twinny","key":"gateway"}'),
                lit(TWINNY_API_KEY),
            ),
        ],
    )
    twinny = home / ".config/Code/User/globalStorage/rjmacarthy.twinny"
    twinny.mkdir(parents=True, exist_ok=True)
    (twinny / "twinny-providers.json").write_text(
        json.dumps({"apiKey": TWINNY_API_KEY}), encoding="utf-8"
    )


TWINNY_SAID_CONVERSATION = "7d1c2b3a-1111-4000-8000-000000000003"
TWINNY_SAID_REL = ".config/Code/User/globalStorage/state.vscdb"


def twinny_said_records():
    """A 4.3.15 conversation with a message the user sent mid-reply.

    Since 4.3.5 such a message is saved among the reply's `toolSteps[]` as a step `{id: "said.<id>",
    name: "said", summary: <text>, status: "done", said: true}`
    (src/extension/chat/tool-steps.ts:101-104 and src/common/types.ts:129-133 at twinnydotdev/twinny
    0337b265). The second `said` step has no summary, the third a non-string one.
    """
    return {
        "twinny.conversations": {
            TWINNY_SAID_CONVERSATION: {
                "id": TWINNY_SAID_CONVERSATION,
                "title": "Install deps",
                "updatedAt": 1791029000000,
                "messages": [
                    {"role": "user", "content": "install the dependencies"},
                    {
                        "role": "assistant",
                        "content": "Installed with pnpm.",
                        "meta": {"model": "qwen2.5-coder:7b", "provider": "Ollama"},
                        "toolSteps": [
                            {
                                "id": "step-1",
                                "name": "run_command",
                                "summary": "ran `npm install`",
                                "args": {"command": "npm install"},
                                "output": "added 3 packages",
                                "status": "done",
                                "at": 0,
                            },
                            {
                                "id": "said.m-1",
                                "name": "said",
                                "summary": "use pnpm instead",
                                "status": "done",
                                "said": True,
                            },
                            {"id": "said.m-2", "name": "said", "status": "done", "said": True},
                            {
                                "id": "said.m-3",
                                "name": "said",
                                "summary": ["and", "add lodash"],
                                "status": "done",
                                "said": True,
                            },
                            {
                                "id": "step-2",
                                "name": "run_command",
                                "summary": "ran `pnpm install`",
                                "args": {"command": "pnpm install"},
                                "output": "done",
                                "status": "done",
                                "at": 0,
                            },
                        ],
                    },
                ],
            }
        }
    }


def build_twinny_said(home: Path) -> None:
    """Write `twinny_said_records()` to a state.vscdb in a home of its own.

    Write `twinny_said_records()` to a plain state.vscdb in a home of its own (not `build_home`'s),
    so the main Twinny fixture's rows and the shared homes' session counts are unchanged.
    """
    _vscdb(home / TWINNY_SAID_REL, {"rjmacarthy.twinny": twinny_said_records()})


# Twinny 4.3.15 records beside the conversations, from
# research/twinny.md (twinnydotdev/twinny at 0337b265): the standing grants
# and team policy in the global state value, the per-reply agent-changes
# record, the Twinny log channel and the review tab's repository in the
# workspace state value. A home of its own, like `build_twinny_said`.
TWINNY_415_CONVERSATION = "7d1c2b3a-1111-4000-8000-000000000004"
TWINNY_415_CHANGE = "5f0e9d8c-2222-4000-8000-000000000001"
TWINNY_415_USER = ".config/Code/User"
TWINNY_415_REL = TWINNY_415_USER + "/globalStorage/state.vscdb"
TWINNY_415_WORKSPACE = TWINNY_415_USER + "/workspaceStorage/ws1"
TWINNY_415_WORKSPACE_DATABASE_REL = TWINNY_415_WORKSPACE + "/state.vscdb"
TWINNY_415_CHANGES_REL = (
    TWINNY_415_WORKSPACE + "/rjmacarthy.twinny/agent-changes/%s.json" % TWINNY_415_CHANGE
)
TWINNY_415_LOG_DIR = ".config/Code/logs/20261003T120000/window1/exthost/rjmacarthy.twinny"
TWINNY_415_LOG_REL = TWINNY_415_LOG_DIR + "/Twinny.log"
TWINNY_415_LOG_OLD_REL = TWINNY_415_LOG_DIR + "/Twinny.log_1"
TWINNY_415_MTIME = 1791029400  # 3 October 12:10:00 UTC, the state files' mtime
TWINNY_415_EMAIL = "dev@example.invalid"
# Two entries and a Debug block, in VS Code's file log format
# (src/vs/platform/log/common/fileLog.ts:62-101 at microsoft/vscode 4d22f4f9)
# with Twinny's messages (generation.ts:150-153, 188-207; logger.ts:77-87).
TWINNY_415_LOG = (
    "2026-10-03 14:00:01.250 [info] Chat → qwen2.5-coder:7b (Ollama) · "
    "2 messages, 41 chars\n"
    "2026-10-03 14:00:01.251 [debug] Chat messages\n"
    "    [user]\n"
    "    which tests fail?\n"
    "    \n"
    "    [assistant]\n"
    "    Two tests fail.\n"
    "2026-10-03 14:00:06.400 [info] Chat ← 5.1s · 15 chars\n"
)


def twinny_415_records():
    """The global state value.

    One conversation whose reply changed two files, the standing grants, a team policy and the
    ChatGPT plan profile (an identity the parser must never emit).
    """
    return {
        "twinny.conversations": {
            TWINNY_415_CONVERSATION: {
                "id": TWINNY_415_CONVERSATION,
                "title": "Fix the tests",
                "updatedAt": 1791029100000,
                "messages": [
                    {"role": "user", "content": "fix the failing tests"},
                    {
                        "role": "assistant",
                        "content": "Fixed both.",
                        "meta": {"model": "qwen2.5-coder:7b", "provider": "Ollama"},
                        "changes": {
                            "id": TWINNY_415_CHANGE,
                            "files": [
                                {
                                    "path": "src/a.ts",
                                    "status": "modified",
                                    "added": 3,
                                    "removed": 1,
                                },
                                {
                                    "path": "logo.png",
                                    "status": "created",
                                    "added": 0,
                                    "removed": 0,
                                    "noUndo": True,
                                },
                            ],
                        },
                    },
                ],
            }
        },
        "twinny-global-context-agentMode": True,
        "twinny-global-context-autoRunCommands": False,
        "twinny-global-context-alwaysRunCommands": ["npm test", "git status"],
        "twinny-global-context-alwaysRunMcpTools": ["github/list_issues"],
        "twinny-global-context-twinny.selectedModel": "qwen2.5-coder:7b",
        "twinny.teamPolicy": {
            "url": "https://gateway.example.invalid",
            "policy": {
                "teamOnly": True,
                "agent": {"commands": "ask", "blockedPaths": ["secrets/", ".env"]},
            },
            "providerIds": ["team-0a1b-chat"],
            "fetchedAt": "2026-10-03T09:00:00.000Z",
        },
        "twinny.chatgpt-plan.profile": {
            "clientId": "app_x",
            "subject": "user-subject",
            "email": TWINNY_415_EMAIL,
            "name": "Dev",
        },
    }


def twinny_415_change_record():
    """The agent-changes record of the reply above (reply-changes.ts:35-47).

    `src/a.ts` since put back, so `undone`.
    """
    return {
        "id": TWINNY_415_CHANGE,
        "root": "/srv/proj",
        "createdAt": 1791029098000,
        "files": [
            {
                "path": "src/a.ts",
                "status": "modified",
                "added": 3,
                "removed": 1,
                "undone": True,
                "before": "export const a = 1\n",
                "afterHash": "ab" * 32,
            },
            {
                "path": "logo.png",
                "status": "created",
                "added": 0,
                "removed": 0,
                "noUndo": True,
                "before": None,
                "afterHash": None,
            },
        ],
    }


def build_twinny_415(home: Path) -> None:
    """Write the Twinny 4.3.15 stores to a home of their own.

    Write the 4.3.15 stores to a home of their own, plus a noise file the parser must not want (the
    workspace's workspace.json and the extension host's own log).
    """
    _vscdb(home / TWINNY_415_REL, {"rjmacarthy.twinny": twinny_415_records()})
    _vscdb(
        home / TWINNY_415_WORKSPACE_DATABASE_REL,
        {
            "rjmacarthy.twinny": {
                "twinny-workspace-context-reviewOwner": "acme",
                "twinny-workspace-context-reviewRepo": "widgets",
                "twinny-workspace-context-chatMessage": "draft",
            }
        },
    )
    for rel in (TWINNY_415_REL, TWINNY_415_WORKSPACE_DATABASE_REL):
        os.utime(home / rel, (TWINNY_415_MTIME, TWINNY_415_MTIME))
    changes = home / TWINNY_415_CHANGES_REL
    changes.parent.mkdir(parents=True, exist_ok=True)
    changes.write_text(json.dumps(twinny_415_change_record()), encoding="utf-8")
    (home / TWINNY_415_WORKSPACE / "workspace.json").write_text(
        '{"folder":"file:///srv/proj"}\n', encoding="utf-8"
    )
    log = home / TWINNY_415_LOG_REL
    log.parent.mkdir(parents=True, exist_ok=True)
    log.write_text(TWINNY_415_LOG, encoding="utf-8")
    (home / TWINNY_415_LOG_OLD_REL).write_text(
        "2026-10-03 13:00:00.000 [warning] Chat failed: connection refused\n", encoding="utf-8"
    )
    (log.parent.parent / "exthost.log").write_text(
        "2026-10-03 14:00:00.000 [info] Extension host started\n", encoding="utf-8"
    )
