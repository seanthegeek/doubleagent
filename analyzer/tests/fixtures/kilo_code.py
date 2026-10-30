"""Synthetic Kilo Code state for the analyzer tests."""

from __future__ import annotations

import json
import os
import sqlite3
from pathlib import Path

from .common import _json, _json_file, _jsonl
from .opencode import OPENCODE_SCHEMA, _oc_message, _oc_part, _oc_session, opencode_assistant

KILO_SESSION = "ses_01JAX"
KILO_TASK = "8f1c2b7e-1"
KILO_T0 = 1791021600000  # 3 October 10:00:00 UTC, ms


def kilo_task_records(start: int = KILO_T0 - 86400000):
    return [
        {
            "role": "user",
            "content": [
                {"type": "text", "text": "<task>fix tests</task>"},
                {
                    "type": "text",
                    "text": "<environment_details>cwd /srv/proj</environment_details>",
                },
            ],
            "ts": start + 1000,
        },
        {
            "role": "assistant",
            "content": [
                {"type": "text", "text": "Running the suite."},
                {
                    "type": "tool_use",
                    "id": "toolu_01A",
                    "name": "execute_command",
                    "input": {"command": "pytest -q"},
                },
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
            "type": "reasoning",
            "summary": [{"type": "summary_text", "text": "tests pass"}],
            "ts": start + 3500,
        },
        {"role": "assistant", "content": "All three tests pass.", "ts": start + 4000},
    ]


def build_kilo(home: Path) -> None:
    data = home / ".local/share/kilo"
    database = data / "kilo.db"
    database.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(database))
    connection.executescript(OPENCODE_SCHEMA)
    start = KILO_T0
    connection.execute(
        "INSERT INTO project (id, worktree, vcs, time_created, time_updated, sandboxes) VALUES (?,?,?,?,?,?)",
        ("prj_01JAX", "/srv/proj", "git", start, start, "[]"),
    )
    _oc_session(
        connection,
        KILO_SESSION,
        "prj_01JAX",
        "/srv/proj",
        "fix tests",
        start,
        model={"id": "claude-sonnet-4-5", "providerID": "anthropic"},
    )
    _oc_message(
        connection,
        "msg_01JAXU",
        KILO_SESSION,
        start + 1000,
        {
            "role": "user",
            "time": {"created": start + 1000},
            "agent": "build",
            "model": {"providerID": "anthropic", "modelID": "claude-sonnet-4-5"},
        },
    )
    _oc_part(
        connection,
        "prt_01JAXP",
        "msg_01JAXU",
        KILO_SESSION,
        start + 1000,
        {"type": "text", "text": "fix the tests"},
    )
    _oc_message(
        connection,
        "msg_01JAXA",
        KILO_SESSION,
        start + 2000,
        dict(opencode_assistant(start + 2000, model="claude-sonnet-4-5"), parentID="msg_01JAXU"),
    )
    _oc_part(
        connection,
        "prt_01JAXT",
        "msg_01JAXA",
        KILO_SESSION,
        start + 2500,
        {
            "type": "tool",
            "callID": "call_1",
            "tool": "bash",
            "state": {
                "status": "completed",
                "input": {"command": "pytest -q"},
                "output": "3 passed",
                "title": "pytest -q",
                "metadata": {},
                "time": {"start": start + 2500, "end": start + 3000},
            },
        },
    )
    # A later turn the user sent with another model and the plan agent.
    _oc_message(
        connection,
        "msg_01JAXV",
        KILO_SESSION,
        start + 4000,
        {
            "role": "user",
            "time": {"created": start + 4000},
            "agent": "plan",
            "model": {"providerID": "openai", "modelID": "gpt-5"},
        },
    )
    _oc_part(
        connection,
        "prt_01JAXQ",
        "msg_01JAXV",
        KILO_SESSION,
        start + 4000,
        {"type": "text", "text": "plan the next fix"},
    )
    # Kilo writes the V2 projection alongside V1; it must not duplicate the rows.
    connection.execute(
        "INSERT INTO session_message VALUES (?,?,?,?,?,?,?)",
        (
            "msg_01JAXB",
            KILO_SESSION,
            "assistant",
            4,
            start + 2000,
            start + 3000,
            json.dumps(
                {
                    "agent": "build",
                    "model": {"providerID": "anthropic", "modelID": "claude-sonnet-4-5"},
                    "content": [
                        {
                            "type": "tool",
                            "id": "call_1",
                            "name": "bash",
                            "state": {
                                "status": "completed",
                                "input": {"command": "pytest -q"},
                                "content": [{"type": "text", "text": "3 passed"}],
                                "structured": {},
                            },
                            "time": {"created": start + 2500, "completed": start + 3000},
                        }
                    ],
                    "time": {"created": start + 2000, "completed": start + 3000},
                }
            ),
        ),
    )
    connection.commit()
    connection.close()
    (data / "auth.json").write_text('{"kilo":{"type":"oauth","access":"REDACT"}}', encoding="utf-8")
    tasks = home / ".config/Code/User/globalStorage/kilocode.kilo-code/tasks"
    task = tasks / KILO_TASK
    task.mkdir(parents=True, exist_ok=True)
    (task / "api_conversation_history.json").write_text(
        json.dumps(kilo_task_records()), encoding="utf-8"
    )
    (task / "ui_messages.json").write_text("[]", encoding="utf-8")
    _json_file(
        tasks / "_index.json",
        {
            "version": 1,
            "updatedAt": KILO_T0 - 86395000,
            "entries": [
                {
                    "id": KILO_TASK,
                    "number": 1,
                    "ts": KILO_T0 - 86400000,
                    "task": "fix tests",
                    "tokensIn": 1200,
                    "tokensOut": 80,
                    "totalCost": 0.0041,
                    "workspace": "/srv/proj",
                    "mode": "code",
                    "status": "completed",
                }
            ],
        },
    )
    build_kilo_share(home)


# Session sync: Kilo's cloud session marker,
# storage/session_share/<sessionID>.json (research/kilo-code.md, Other stores).
KILO_SHARE_REL = ".local/share/kilo/storage/session_share/%s.json" % KILO_SESSION
KILO_SHARE_MTIME = 1791022000  # 3 October 10:06:40 UTC


def build_kilo_share(home: Path) -> None:
    _json_file(
        home / KILO_SHARE_REL,
        {
            "id": "ks_01JAX",
            "ingestPath": "/api/session/ks_01JAX/ingest",
            "url": "https://app.kilo.ai/s/share01",
        },
    )
    os.utime(home / KILO_SHARE_REL, (KILO_SHARE_MTIME, KILO_SHARE_MTIME))


KILO_LOG_NAME = "2026-10-03T100000.log"
KILO_SANDBOX_SESSION_HASH = "a" * 64
KILO_SANDBOX_DIR_HASH = "b" * 64


def build_kilo_state(home: Path) -> None:
    """Kilo Code's standing state, past use and remote control at 87cc35c.

    The TUI history and stash in the 1.x shapes (packages/tui/src/prompt/ history.tsx:9-25,
    stash.tsx:9-13), the global config's `permission` key
    (packages/core/src/v1/config/permission.ts:5-56), a sandbox policy and preference
    (kilocode/sandbox/store.ts:11-17, preference.ts:10-29), the retention state
    (kilocode/session/retention.ts:45-54) and a log with the remote control lines
    (kilo-sessions/remote-session-log.ts:37-61, kilo-sessions.ts:1220, :1259;
    core/src/util/log.ts:162-189).
    """
    state = home / ".local/state/kilo"
    _jsonl(
        state / "prompt-history.jsonl",
        [
            {"input": "fix the tests", "parts": []},
            {"input": "ls -la", "mode": "shell", "parts": []},
        ],
    )
    _jsonl(
        state / "prompt-stash.jsonl",
        [{"input": "stashed idea", "parts": [], "timestamp": KILO_T0 + 9000}],
    )
    config = home / ".config/kilo/kilo.jsonc"
    config.parent.mkdir(parents=True, exist_ok=True)
    config.write_text(
        '{\n  // written by an "always" approval\n'
        '  "permission": {"bash": {"git status": "allow", "rm *": "deny"}, "webfetch": "ask"},\n'
        '  "model": "anthropic/claude-sonnet-4-5",\n}\n',
        encoding="utf-8",
    )
    os.utime(config, (KILO_SHARE_MTIME, KILO_SHARE_MTIME))
    policy = (
        home
        / ".local/state/kilo-sandbox-policy"
        / KILO_SANDBOX_SESSION_HASH
        / (KILO_SANDBOX_DIR_HASH + ".json")
    )
    _json(
        policy,
        {
            "enabled": True,
            "mode": "proxy",
            "allowedHosts": ["registry.npmjs.org"],
            "writablePaths": ["/srv/proj"],
            "version": 3,
        },
    )
    os.utime(policy, (KILO_SHARE_MTIME, KILO_SHARE_MTIME))
    pref = home / ".local/state/kilo-sandbox-preference" / (KILO_SANDBOX_DIR_HASH + ".json")
    pref.parent.mkdir(parents=True, exist_ok=True)
    pref.write_text("false", encoding="utf-8")
    os.utime(pref, (KILO_SHARE_MTIME, KILO_SHARE_MTIME))
    _json(
        home / ".local/share/kilo/retention/state.json",
        {
            "at": KILO_T0 + 7200000,
            "scanned": 40,
            "deleted": 12,
            "skippedActive": 1,
            "failed": 0,
            "durationMs": 830,
            "reclaimedBytes": 4096,
        },
    )
    log = home / ".local/share/kilo/log" / KILO_LOG_NAME
    log.parent.mkdir(parents=True, exist_ok=True)
    log.write_text(
        'INFO  2026-10-03T10:00:00 +0ms service=default version=7.0.0 args=["remote"] kilo\n'
        "INFO  2026-10-03T10:00:05 +12ms service=kilo-sessions connected=true "
        "remote connection enabled\n"
        "INFO  2026-10-03T10:00:09 +40ms service=kilo-sessions sessionID=%s "
        "startedAt=2026-10-03T10:00:09.250Z model=anthropic/claude-sonnet-4-5 "
        "directory=/srv/proj remote session started\n"
        "INFO  2026-10-03T10:05:00 +3ms service=kilo-sessions sessionID=%s "
        "startedAt=2026-10-03T10:00:09.250Z durationMs=290750 reason=disabled "
        "remote session ended\n"
        "INFO  2026-10-03T10:05:00 +1ms service=kilo-sessions remote connection disabled\n"
        % (KILO_SESSION, KILO_SESSION),
        encoding="utf-8",
    )
