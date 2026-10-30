"""Synthetic Kilo Code state for the analyzer tests.

The shapes follow `research/kilo-code.md` (Kilo-Org/kilocode at 0a9faef, 7.8.25): the `kilo.db`
tables Kilo's parser reads, the pre-fork extension's task files, the cloud session marker and
the standing state, past use and remote control files.
"""

from __future__ import annotations

import json
import os
import sqlite3
from pathlib import Path

from .common import _json, _json_file, _jsonl

KILO_SESSION = "ses_01JAX"
KILO_TASK = "8f1c2b7e-1"
KILO_T0 = 1791021600000  # 3 October 10:00:00 UTC, ms

# The `kilo.db` tables the parser reads, with Kilo's own columns (`kilo.db` and `project` and
# `project_directory` in research/kilo-code.md).
KILO_SCHEMA = """
CREATE TABLE project (id text PRIMARY KEY, worktree text NOT NULL, vcs text, name text, icon_url text,
  icon_url_override text, icon_color text, time_created integer NOT NULL, time_updated integer NOT NULL,
  time_initialized integer, sandboxes text NOT NULL, commands text);
CREATE TABLE project_directory (project_id text NOT NULL, directory text NOT NULL, type text, strategy text,
  time_created integer NOT NULL, PRIMARY KEY (project_id, directory));
CREATE TABLE session (id text PRIMARY KEY, project_id text NOT NULL, workspace_id text, parent_id text,
  slug text NOT NULL, directory text NOT NULL, path text, title text NOT NULL, version text NOT NULL,
  share_url text, summary_additions integer, summary_deletions integer, summary_files integer,
  summary_diffs text, metadata text, cost real NOT NULL DEFAULT 0, tokens_input integer NOT NULL DEFAULT 0,
  tokens_output integer NOT NULL DEFAULT 0, tokens_reasoning integer NOT NULL DEFAULT 0,
  tokens_cache_read integer NOT NULL DEFAULT 0, tokens_cache_write integer NOT NULL DEFAULT 0,
  revert text, permission text, agent text, model text, time_created integer NOT NULL,
  time_updated integer NOT NULL, time_compacting integer, time_archived integer);
CREATE TABLE message (id text PRIMARY KEY, session_id text NOT NULL, time_created integer NOT NULL,
  time_updated integer NOT NULL, data text NOT NULL);
CREATE TABLE part (id text PRIMARY KEY, message_id text NOT NULL, session_id text NOT NULL,
  time_created integer NOT NULL, time_updated integer NOT NULL, data text NOT NULL);
CREATE TABLE session_message (id text PRIMARY KEY, session_id text NOT NULL, type text NOT NULL, seq integer,
  time_created integer NOT NULL, time_updated integer NOT NULL, data text NOT NULL);
"""


def _message(connection, message_id, created, data):
    connection.execute(
        "INSERT INTO message VALUES (?,?,?,?,?)",
        (message_id, KILO_SESSION, created, created, json.dumps(data)),
    )


def _part(connection, part_id, message_id, created, data):
    connection.execute(
        "INSERT INTO part VALUES (?,?,?,?,?,?)",
        (part_id, message_id, KILO_SESSION, created, created, json.dumps(data)),
    )


def _assistant(created, model):
    """Return a V1 assistant message of `model`, `parentID` the first user message."""
    return {
        "role": "assistant",
        "time": {"created": created, "completed": created + 1000},
        "parentID": "msg_01JAXU",
        "modelID": model,
        "providerID": "anthropic",
        "mode": "build",
        "agent": "build",
        "path": {"cwd": "/srv/proj", "root": "/srv/proj"},
        "cost": 0.004,
        "tokens": {"input": 1200, "output": 80, "reasoning": 0, "cache": {"read": 0, "write": 0}},
    }


def kilo_task_records(start: int = KILO_T0 - 86400000):
    """Return a pre-fork extension task's `api_conversation_history.json` records.

    Source: research/kilo-code.md#legacy-extension-tasks; source 0a9faef
    """
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
        {"type": "reasoning", "reasoning_content": "tests pass", "ts": start + 3500},
        {"role": "assistant", "content": "All three tests pass.", "ts": start + 4000},
    ]


def _build_database(database: Path, start: int) -> None:
    database.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(database))
    connection.executescript(KILO_SCHEMA)
    connection.execute(
        "INSERT INTO project (id, worktree, vcs, time_created, time_updated, sandboxes) "
        "VALUES (?,?,?,?,?,?)",
        ("prj_01JAX", "/srv/proj", "git", start, start, "[]"),
    )
    connection.execute(
        "INSERT INTO session (id, project_id, slug, directory, title, version, agent, model, "
        "time_created, time_updated) VALUES (?,?,?,?,?,?,?,?,?,?)",
        (
            KILO_SESSION,
            "prj_01JAX",
            "fix-tests",
            "/srv/proj",
            "fix tests",
            "7.8.25",
            "build",
            json.dumps({"id": "claude-sonnet-4-5", "providerID": "anthropic"}),
            start,
            start + 5000,
        ),
    )
    _message(
        connection,
        "msg_01JAXU",
        start + 1000,
        {
            "role": "user",
            "time": {"created": start + 1000},
            "agent": "build",
            "model": {"providerID": "anthropic", "modelID": "claude-sonnet-4-5"},
        },
    )
    _part(
        connection,
        "prt_01JAXP",
        "msg_01JAXU",
        start + 1000,
        {"type": "text", "text": "fix the tests"},
    )
    _message(connection, "msg_01JAXA", start + 2000, _assistant(start + 2000, "claude-sonnet-4-5"))
    _part(
        connection,
        "prt_01JAXT",
        "msg_01JAXA",
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
    _message(
        connection,
        "msg_01JAXV",
        start + 4000,
        {
            "role": "user",
            "time": {"created": start + 4000},
            "agent": "plan",
            "model": {"providerID": "openai", "modelID": "gpt-5"},
        },
    )
    _part(
        connection,
        "prt_01JAXQ",
        "msg_01JAXV",
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
                    "model": {"id": "claude-sonnet-4-5", "providerID": "anthropic"},
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


def build_kilo(home: Path) -> None:
    data = home / ".local/share/kilo"
    _build_database(data / "kilo.db", KILO_T0)
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
            "entries": [
                {
                    "id": KILO_TASK,
                    "ts": KILO_T0 - 86400000,
                    "task": "fix tests",
                    "workspace": "/srv/proj",
                    "mode": "code",
                    "status": "completed",
                }
            ],
        },
    )
    build_kilo_share(home)


# Session sync: Kilo's cloud session marker, storage/session_share/<sessionID>.json.
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
KILO_MEMORY_FOLDER = "proj-0123456789ab"
KILO_DIGEST_REL = ".local/share/kilo/memory/%s/sessions/2026-10-03T10-30-00.000Z_%s_h1.md" % (
    KILO_MEMORY_FOLDER,
    KILO_SESSION,
)


def build_kilo_state(home: Path) -> None:
    """Kilo Code's standing state, past use, project memory and remote control files.

    The TUI history and stash, the global config's `permission` key, a sandbox policy and
    preference, the retention state, a project memory folder with one session digest and a
    log with the remote control lines, in the shapes of `research/kilo-code.md`.
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
    memory = home / ".local/share/kilo/memory" / KILO_MEMORY_FOLDER
    _json(
        memory / "manifest.json",
        {
            "kind": "kilo-memory",
            "display": "proj",
            "canonical": "/srv/proj",
            "folder": KILO_MEMORY_FOLDER,
            "createdAt": "2026-10-02T09:00:00.000Z",
        },
    )
    digest = home / KILO_DIGEST_REL
    digest.parent.mkdir(parents=True, exist_ok=True)
    digest.write_text(
        "# Session %s\n\nVersion: 1\nUpdated: 2026-10-03T10:30:00.000Z\nTopic: test fixes\n\n"
        "## Summary\nRan the suite; three tests pass.\n" % KILO_SESSION,
        encoding="utf-8",
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
