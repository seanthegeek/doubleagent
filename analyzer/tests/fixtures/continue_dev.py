"""Synthetic Continue state for the analyzer tests."""

from __future__ import annotations

import json
from pathlib import Path

from .common import _jsonl

CONTINUE_SESSION = "0f1e"
CONTINUE_CREATED = (
    "1790935200000"  # 2 October 10:00:00 UTC, a millisecond epoch string as sessions.json stores it
)


def continue_session():
    """A Continue session file in the shape of core/index.d.ts `Session`."""
    edit = {
        "id": "call_1",
        "type": "function",
        "function": {"name": "edit_existing_file", "arguments": '{"filepath":"a.py"}'},
    }
    term = {
        "id": "call_2",
        "type": "function",
        "function": {"name": "run_terminal_command", "arguments": '{"command":"rm -rf build"}'},
    }
    return {
        "sessionId": CONTINUE_SESSION,
        "title": "Fix bug",
        "workspaceDirectory": "/srv/proj",
        "history": [
            {
                "message": {
                    "role": "user",
                    "content": [
                        {"type": "text", "text": "rename foo"},
                        {"type": "imageUrl", "imageUrl": {"url": "data:image/png;base64,AA"}},
                    ],
                },
                "contextItems": [],
            },
            {
                "message": {"role": "thinking", "content": "consider the callers"},
                "contextItems": [],
            },
            {
                "message": {"role": "assistant", "content": "", "toolCalls": [edit]},
                "contextItems": [],
                "reasoning": {
                    "active": False,
                    "text": "find foo first",
                    "startAt": 1790935201000,
                    "endAt": 1790935202000,
                },
                "promptLogs": [
                    {
                        "modelTitle": "GPT-4o",
                        "modelProvider": "openai",
                        "prompt": "<long prompt>",
                        "completion": "",
                    }
                ],
                "toolCallStates": [
                    {
                        "toolCallId": "call_1",
                        "toolCall": edit,
                        "status": "done",
                        "parsedArgs": {"filepath": "a.py"},
                        "output": [{"name": "Edit", "description": "", "content": "ok"}],
                    }
                ],
            },
            {
                "message": {"role": "tool", "content": "ok", "toolCallId": "call_1"},
                "contextItems": [],
            },
            {
                "message": {
                    "role": "assistant",
                    "content": "Renamed. Cleaning the build too.",
                    "toolCalls": [term],
                },
                "contextItems": [],
                "promptLogs": [
                    {
                        "modelTitle": "Claude Sonnet 4.5",
                        "modelProvider": "anthropic",
                        "prompt": "<long prompt>",
                        "completion": "Renamed. Cleaning the build too.",
                    }
                ],
                "toolCallStates": [
                    {
                        "toolCallId": "call_2",
                        "toolCall": term,
                        "status": "canceled",
                        "parsedArgs": {"command": "rm -rf build"},
                        # the CLI's refusal at the prompt
                        "output": [
                            {
                                "content": "Permission denied by user",
                                "name": "Tool Result",
                                "description": "Tool execution result",
                            }
                        ],
                    }
                ],
            },
        ],
        "mode": "agent",
        "chatModelTitle": "GPT-4o",
        "usage": {"promptTokens": 120, "completionTokens": 30, "totalCost": 0.01},
    }


def continue_index():
    return [
        {
            "sessionId": CONTINUE_SESSION,
            "title": "Fix bug",
            "dateCreated": CONTINUE_CREATED,
            "workspaceDirectory": "/srv/proj",
            "messageCount": 2,
        }
    ]


def continue_dev_data():
    base = {
        "schema": "0.2.0",
        "userId": "",
        "userAgent": "vscode/1.1 (Continue/1.0)",
        "selectedProfileId": "local",
    }
    chat = dict(
        base,
        eventName="chatInteraction",
        timestamp="2026-10-02T10:00:05.000Z",
        prompt="rename foo",
        completion="done",
        modelName="gpt-4o",
        modelTitle="GPT-4o",
        modelProvider="openai",
        sessionId=CONTINUE_SESSION,
    )
    tool = dict(
        base,
        eventName="toolUsage",
        timestamp="2026-10-02T10:00:03.000Z",
        toolCallId="call_1",
        functionName="edit_existing_file",
        functionParams={"filepath": "a.py"},
        toolCallArgs='{"filepath":"a.py"}',
        accepted=True,
        succeeded=True,
        output=[{"name": "Edit", "description": "", "content": "ok"}],
    )
    rejected = dict(
        base,
        eventName="toolUsage",
        timestamp="2026-10-02T10:00:06.000Z",
        toolCallId="call_2",
        functionName="run_terminal_command",
        functionParams={"command": "rm -rf build"},
        toolCallArgs='{"command":"rm -rf build"}',
        accepted=False,
        succeeded=False,
        output=[],
    )
    return [chat], [tool, rejected]


def build_continue(home: Path) -> None:
    sessions = home / ".continue/sessions"
    sessions.mkdir(parents=True, exist_ok=True)
    (sessions / (CONTINUE_SESSION + ".json")).write_text(
        json.dumps(continue_session(), indent=2), encoding="utf-8"
    )
    (sessions / "sessions.json").write_text(json.dumps(continue_index()), encoding="utf-8")
    chat, tool = continue_dev_data()
    _jsonl(home / ".continue/dev_data/0.2.0/chatInteraction.jsonl", chat)
    _jsonl(home / ".continue/dev_data/0.2.0/toolUsage.jsonl", tool)
    for event, records in continue_dev_data_more().items():
        _jsonl(home / (".continue/dev_data/0.2.0/%s.jsonl" % event), records)
    (home / ".continue/config.yaml").write_text("name: Local\nmodels: []\n", encoding="utf-8")
    (home / ".continue/permissions.yaml").write_text(CONTINUE_PERMISSIONS, encoding="utf-8")
    (home / ".continue/input_history.json").write_text(
        json.dumps(CONTINUE_INPUT_HISTORY, indent=2), encoding="utf-8"
    )


# The CLI's standing tool grants as policyWriter.ts:57-66 writes them at
# 5522c6f (a header comment, then YAML.stringify of the three lists), with
# a quoted rule and a trailing comment a hand edit can add.
CONTINUE_PERMISSIONS = (
    "# Continue CLI Permissions Configuration\n"
    "# This file is managed by the Continue CLI and should not be edited manually.\n"
    "# Use the TUI to modify permissions interactively.\n"
    "\n"
    "allow:\n"
    "  - Read\n"
    "  - Bash(npm*)  # added by hand\n"
    "ask: []\n"
    "exclude:\n"
    '  - "Bash(rm*)"\n'
)

# extensions/cli/src/util/inputHistory.ts:53-72 at 5522c6f: newest first,
# `timestamp` in epoch milliseconds, written with JSON.stringify(_, null, 2).
CONTINUE_INPUT_HISTORY = [
    {"text": "/model", "timestamp": 1790935260000},  # 2 October 10:01:00 UTC
    {"text": "rename foo", "timestamp": 1790935201000},  # 2 October 10:00:01 UTC
]


def continue_dev_data_more() -> dict[str, list[dict]]:
    """The `chatFeedback`, `editInteraction` and `editOutcome` dev_data lines.

    The `chatFeedback`, `editInteraction` and `editOutcome` lines of dev_data 0.2.0
    (packages/config-yaml/src/schemas/data/ at 5522c6f): the rating of the fixture's chatInteraction
    reply, and one edit request the user rejected.
    """
    base = {
        "schema": "0.2.0",
        "userId": "",
        "userAgent": "vscode/1.1 (Continue/1.0)",
        "selectedProfileId": "local",
        "modelProvider": "openai",
        "modelName": "GPT-4o",
        "modelTitle": "GPT-4o",
    }
    feedback = dict(
        base,
        eventName="chatFeedback",
        timestamp="2026-10-02T10:01:30.000Z",
        completionOptions={},
        prompt="rename foo",
        completion="done",
        feedback=True,
        sessionId=CONTINUE_SESSION,
    )
    edit = dict(
        base,
        eventName="editInteraction",
        timestamp="2026-10-02T10:02:00.000Z",
        prompt="add a docstring",
        completion='def foo():\n    """Doc."""\n',
        filepath="/srv/proj/a.py",
    )
    outcome = dict(
        base,
        eventName="editOutcome",
        timestamp="2026-10-02T10:02:30.000Z",
        prompt="add a docstring",
        completion='def foo():\n    """Doc."""\n',
        previousCode="def foo(): pass\n",
        newCode='def foo():\n    """Doc."""\n',
        previousCodeLines=1,
        newCodeLines=2,
        lineChange=1,
        accepted=False,
        filepath="/srv/proj/a.py",
    )
    return {"chatFeedback": [feedback], "editInteraction": [edit], "editOutcome": [outcome]}


# The `cn` CLI's winston log in `cn serve --id` mode, in the line
# format of extensions/cli/src/util/logger.ts:74-95 and the `[metadata]`
# lines of extensions/cli/src/util/metadata.ts:125-181 at 5522c6f. Two
# processes (a1b2c3d4, 0e0f1a2b) post for the same agent; the second's
# result comes after the first's second post, which never gets one.
CONTINUE_AGENT = "agt_7f3e"
CONTINUE_METADATA_URL = "https://api.continue.dev/agents/%s/metadata" % CONTINUE_AGENT
CONTINUE_CN_LOG = (
    "\n".join(
        [
            "2026-10-07 10:00:00.001 [a1b2c3d4] [info]: Starting server on port 8000",
            "2026-10-07 10:00:00.010 [a1b2c3d4] [warn]: Storage sync requested but no Continue API"
            " key is available; skipping uploads.",
            "2026-10-07 10:00:05.100 [a1b2c3d4] [info]: [metadata] POST %s"
            ' {"agentId":"%s","metadataKeys":["summary","isComplete"]}'
            % (CONTINUE_METADATA_URL, CONTINUE_AGENT),
            '2026-10-07 10:00:05.101 [a1b2c3d4] [info]: [metadata] Request body {"metadata":'
            '{"summary":"Renamed foo to bar","isComplete":false}}',
            "2026-10-07 10:00:05.200 [0e0f1a2b] [info]: [metadata] POST %s"
            ' {"agentId":"%s","metadataKeys":["additions"]}'
            % (CONTINUE_METADATA_URL, CONTINUE_AGENT),
            "2026-10-07 10:00:05.350 [a1b2c3d4] [info]: [metadata] Success: 200 (250ms) %s"
            % CONTINUE_METADATA_URL,
            "2026-10-07 10:00:06.000 [a1b2c3d4] [info]: [metadata] POST %s"
            ' {"agentId":"%s","metadataKeys":["isComplete"]}'
            % (CONTINUE_METADATA_URL, CONTINUE_AGENT),
            "2026-10-07 10:00:06.400 [0e0f1a2b] [warn]: [metadata] Failed: 502 Bad Gateway (1200ms) %s"
            ' {"response":"upstream"}' % CONTINUE_METADATA_URL,
            "2026-10-07 10:00:07.000 [a1b2c3d4] [error]: Unhandled error",
            "Error: boom",
            "    at main (index.js:1:1)",
        ]
    )
    + "\n"
)
CONTINUE_CN1_LOG = (
    "2026-10-06 09:00:00.000 [99aa88bb] [info]: [metadata] POST %s"
    ' {"agentId":"%s","metadataKeys":["summary"]}\n'
    "2026-10-06 09:00:00.500 [99aa88bb] [info]: [metadata] Auth required (skipping) (5ms) %s\n"
    % (CONTINUE_METADATA_URL, CONTINUE_AGENT, CONTINUE_METADATA_URL)
)


def build_continue_log(home: Path) -> None:
    logs = home / ".continue/logs"
    logs.mkdir(parents=True, exist_ok=True)
    (logs / "cn.log").write_text(CONTINUE_CN_LOG, encoding="utf-8")
    (logs / "cn1.log").write_text(CONTINUE_CN1_LOG, encoding="utf-8")
    # The extension's prompt log, which the parser does not read.
    (logs / "prompt.log").write_text("==========================\nprompt\n", encoding="utf-8")


# A sessions.json cut mid-write. The index is one JSON array that
# core/util/history.ts at 5522c6f rewrites whole on every save, so a crash
# or a partial copy leaves the entries before the cut and a broken last
# one. The added entry names a session with no file of its own.
CONTINUE_LOST_SESSION = "1a2b"


def continue_index_cut(entries: list) -> tuple[str, str]:
    """Return a `sessions.json` cut inside an added entry's title.

    `entries` plus one more, serialised as `sessions.json` is and cut inside the added entry's
    title. Returns the file text and the text from the broken entry's first character to the end,
    which the parser's row must carry in full. The PearAI fork's index has the same shape.
    """
    lost = {
        "sessionId": CONTINUE_LOST_SESSION,
        "title": "Lost entry",
        "dateCreated": "1790935300000",
        "workspaceDirectory": "/srv/proj",
    }
    text = json.dumps([*entries, lost])
    text = text[: text.index("Lost entry") + 4]
    return text, text[text.index('{"sessionId": "%s"' % CONTINUE_LOST_SESSION) :]
