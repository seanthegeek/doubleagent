"""Synthetic Antigravity state for the analyzer tests."""

from __future__ import annotations

import json
import os
import sqlite3
from pathlib import Path

from doubleagent.parsers.antigravity import SCHEMA as AGY
from doubleagent.protobuf import encode

from .common import _jsonl, _utc_text

AGY_CONVERSATION = "799062d5-0000-4000-8000-000000000099"
AGY_T0 = 1790848800  # 1 October 10:00:00 UTC
AGY_APP_CONVERSATION = "a1b2c3d4-0000-4000-8000-0000000000a2"
AGY_ORPHAN = "0d1e2f3a-0000-4000-8000-0000000000b3"  # brain mirror only
AGY_APP_PROJECT = "5e1d7c2a-0000-4000-8000-0000000000a1"
AGY_APP_T0 = AGY_T0 + 3600  # 1 October 11:00:00 UTC
AGY_CONFIG_MTIME = AGY_T0 + 7200  # 1 October 12:00:00 UTC, the standing files' mtime
AGY_SIDECAR = "weekly-check"


def _step(
    kind: str,
    payload: dict,
    created: float,
    completed: float | None = None,
    tool_call: dict | None = None,
    source: int = 2,
    status: int = 3,
) -> dict:
    metadata = {"created_at": created, "source": source}
    if completed is not None:
        metadata["completed_at"] = completed
        metadata["finished_generating_at"] = completed
    if tool_call:
        metadata["tool_call"] = tool_call
    return {"metadata": metadata, kind: payload, "status": status}


def antigravity_steps() -> list[tuple[int, int, dict]]:
    """(step_type, status, Step dict) in the shapes seen on a real install.

    User_input, planner_response with tool calls, generic and typed tool steps.
    """
    tc1 = {
        "id": "call_1",
        "name": "view_file",
        "arguments_json": json.dumps({"AbsolutePath": "/home/u/proj/README.md"}),
    }
    tc2 = {
        "id": "call_2",
        "name": "run_command",
        "arguments_json": json.dumps(
            {"CommandLine": "rm -rf /var/log/*.log", "Cwd": "/home/u/proj"}
        ),
    }
    tc3 = {
        "id": "call_3",
        "name": "write_to_file",
        "arguments_json": json.dumps({"TargetFile": "/home/u/proj/notes.md"}),
    }
    tc4 = {
        "id": "call_4",
        "name": "run_command",
        "arguments_json": json.dumps({"CommandLine": "make test", "Cwd": "/home/u/proj"}),
    }
    tc5 = {
        "id": "call_5",
        "name": "create_issue",
        "arguments_json": json.dumps({"Name": "flaky test"}),
    }
    tc6 = {
        "id": "call_6",
        "name": "run_command",
        "arguments_json": json.dumps({"CommandLine": "ls", "Cwd": "/home/u/proj"}),
    }
    start = AGY_T0
    return [
        (14, 3, dict(_step("user_input", {"query": "clean the logs"}, start, source=4), type=14)),
        (
            15,
            3,
            dict(
                _step(
                    "planner_response",
                    {
                        "response": "I will read the README first.",
                        "thinking": "look before leaping",
                        "tool_calls": [tc1],
                    },
                    start + 1,
                    start + 3,
                ),
                type=15,
            ),
        ),
        (
            132,
            3,
            dict(
                _step(
                    "generic",
                    {
                        "args": [{"key": "AbsolutePath", "value": "/home/u/proj/README.md"}],
                        "result": {"result": "File Path: README.md\n# proj"},
                    },
                    start + 3,
                    start + 4,
                    tool_call=tc1,
                ),
                type=132,
            ),
        ),
        (
            15,
            3,
            dict(
                _step(
                    "planner_response", {"response": "", "tool_calls": [tc2]}, start + 4, start + 6
                ),
                type=15,
            ),
        ),
        (
            28,
            3,
            dict(
                _step(
                    "run_command",
                    {
                        "command_line": "rm -rf /var/log/*.log",
                        "cwd": "/home/u/proj",
                        "exit_code": 0,
                        "combined_output": {"full": "removed 3 files"},
                    },
                    start + 6,
                    start + 7,
                    tool_call=tc2,
                ),
                type=28,
            ),
        ),
        (
            15,
            3,
            dict(
                _step(
                    "planner_response", {"response": "", "tool_calls": [tc3]}, start + 7, start + 8
                ),
                type=15,
            ),
        ),
        (
            23,
            7,
            dict(
                _step(
                    "write_to_file",
                    {"target_file_uri": "file:///home/u/proj/notes.md", "file_created": True},
                    start + 8,
                    start + 9,
                    tool_call=tc3,
                    status=7,
                ),
                type=23,
            ),
        ),
        (
            14,
            3,
            dict(
                _step("user_input", {"query": "<injected reminder>"}, start + 9, source=3), type=14
            ),
        ),
        (
            15,
            3,
            dict(
                _step(
                    "planner_response",
                    {"response": "Done. Three log files were removed."},
                    start + 10,
                    start + 12,
                ),
                type=15,
            ),
        ),
        (
            23,
            3,
            dict(_step("checkpoint", {"conversation_title": "Clean logs"}, start + 13), type=23),
        ),
        # A later planner step on another model, a run_command and an
        # mcp_tool call the user rejected, and a sandboxed run_command.
        (
            15,
            3,
            dict(
                _step(
                    "planner_response",
                    {"response": "", "tool_calls": [tc4]},
                    start + 14,
                    start + 15,
                ),
                type=15,
            ),
        ),
        (
            21,
            3,
            dict(
                _step(
                    "run_command",
                    {"command_line": "make test", "user_rejected": True},
                    start + 15,
                    start + 16,
                    tool_call=tc4,
                ),
                type=21,
            ),
        ),
        (
            15,
            3,
            dict(
                _step(
                    "planner_response",
                    {"response": "", "tool_calls": [tc5]},
                    start + 17,
                    start + 18,
                ),
                type=15,
            ),
        ),
        (
            38,
            3,
            dict(
                _step(
                    "mcp_tool",
                    {"server_name": "github", "tool_call": tc5, "user_rejected": True},
                    start + 18,
                    start + 19,
                    tool_call=tc5,
                ),
                type=38,
            ),
        ),
        (
            15,
            3,
            dict(
                _step(
                    "planner_response",
                    {"response": "", "tool_calls": [tc6]},
                    start + 20,
                    start + 21,
                ),
                type=15,
            ),
        ),
        (
            21,
            3,
            dict(
                _step(
                    "run_command",
                    {
                        "command_line": "ls",
                        "exit_code": 0,
                        "combined_output": {"full": "README.md"},
                        "ran_in_sandbox": True,
                    },
                    start + 21,
                    start + 22,
                    tool_call=tc6,
                ),
                type=21,
            ),
        ),
    ]


def antigravity_app_steps() -> list[tuple[int, int, dict]]:
    """Steps of a desktop-app conversation.

    The kinds the macOS real install of app 2.23.0 showed (user_input, planner_response, generic)
    plus a system_message (101) and an error_message (17) step, both seen there.
    """
    tool_call = {
        "id": "call_7",
        "name": "run_command",
        "arguments_json": json.dumps({"CommandLine": "brew outdated", "Cwd": "/home/u/proj"}),
    }
    tc_del = {
        "id": "call_8",
        "name": "delete_file",
        "arguments_json": json.dumps({"TargetFile": "/home/u/proj/old.log"}),
    }
    start = AGY_APP_T0
    return [
        (
            14,
            3,
            dict(
                _step("user_input", {"query": "which packages are outdated?"}, start, source=4),
                type=14,
            ),
        ),
        (
            15,
            3,
            dict(
                _step(
                    "planner_response",
                    {
                        "response": "I will ask Homebrew.",
                        "thinking": "brew knows",
                        "tool_calls": [tool_call],
                    },
                    start + 1,
                    start + 2,
                ),
                type=15,
            ),
        ),
        (
            132,
            3,
            dict(
                _step(
                    "generic",
                    {
                        "args": [{"key": "CommandLine", "value": "brew outdated"}],
                        "result": {"result": "node (24.1.0) < 24.2.0"},
                    },
                    start + 2,
                    start + 3,
                    tool_call=tool_call,
                ),
                type=132,
                # steps.permissions: one grant and one file access entry.
                permissions={
                    "permission_grants": [
                        {
                            "resource": {"action": "run_command", "target": "brew outdated"},
                            "allow": True,
                            "scope": 2,
                            "action": "run_command",
                            "target": "brew outdated",
                        }
                    ],
                    "file_access_permissions": [
                        {"path": "/home/u/proj", "is_directory": True, "allow": True, "scope": 3}
                    ],
                },
            ),
        ),
        (
            101,
            3,
            dict(
                _step(
                    "system_message", {"message": "background task finished"}, start + 4, source=5
                ),
                type=101,
            ),
        ),
        (
            17,
            3,
            dict(
                _step(
                    "error_message",
                    {"error": {"short_error": "model overloaded", "full_error": "HTTP 529"}},
                    start + 5,
                    source=5,
                ),
                type=17,
            ),
        ),
        (
            15,
            3,
            dict(
                _step(
                    "planner_response",
                    {"response": "One package is outdated: node."},
                    start + 6,
                    start + 7,
                ),
                type=15,
            ),
        ),
        # A code_action that deletes a file: ActionSpecDeleteFile keeps its
        # path in field 1.
        (
            15,
            3,
            dict(
                _step(
                    "planner_response",
                    {"response": "", "tool_calls": [tc_del]},
                    start + 8,
                    start + 9,
                ),
                type=15,
            ),
        ),
        (
            5,
            3,
            dict(
                _step(
                    "code_action",
                    {
                        "action_spec": {
                            "delete_file": {"path": {"absolute_uri": "file:///home/u/proj/old.log"}}
                        },
                        "description": "remove the stale log",
                    },
                    start + 9,
                    start + 10,
                    tool_call=tc_del,
                ),
                type=5,
            ),
        ),
    ]


def build_antigravity_app(base: Path) -> None:
    """The desktop app's data directory, ~/.gemini/antigravity.

    The same stores as the CLI's with source 1 and app_data_dir `antigravity`, no history.jsonl,
    noise the parser must not want (the gdocs plugin's OAuth token), and the brain mirror's
    shortened transcript.jsonl alone.
    """
    build_antigravity(
        base,
        conversation=AGY_APP_CONVERSATION,
        source=1,
        steps=antigravity_app_steps(),
        start=AGY_APP_T0,
        project_id=AGY_APP_PROJECT,
        title="Outdated packages",
        app_data_dir="antigravity",
        models={"gemini-3.8-pro": [1, 5]},
        cli=False,
    )
    token = base / "plugin_data/gdocs/token.json"
    token.parent.mkdir(parents=True)
    token.write_text(
        json.dumps({"client_id": "x", "token": {"access_token": "y"}}), encoding="utf-8"
    )
    # The brain mirror's shortened twin alone: read because no
    # transcript_full.jsonl or chunk sits beside it.
    _jsonl(
        base / "brain" / AGY_APP_CONVERSATION / ".system_generated/logs/transcript.jsonl",
        antigravity_app_brain_records(),
    )


def _brain_line(index: int, kind: str, timestamp: str, content: str = "", **extra) -> dict:
    """One brain transcript line in the key set the real installs showed."""
    source = {"USER_INPUT": "USER_EXPLICIT", "PLANNER_RESPONSE": "MODEL"}.get(kind, "SYSTEM")
    return {
        "step_index": index,
        "source": source,
        "type": kind,
        "status": "DONE",
        "created_at": timestamp,
        "content": content,
        **extra,
    }


def antigravity_brain_records() -> list[dict]:
    """The CLI conversation's brain transcript_full.jsonl.

    The lines of database steps 0-3, 7 and 8, each at the step's created_at, so every row pairs with
    the database's (the assistant, tool_use and tool_result rows there carry completed_at, one to
    two seconds later).
    """
    return [
        _brain_line(0, "USER_INPUT", "2026-10-01T10:00:00Z", "clean the logs"),
        _brain_line(
            1,
            "PLANNER_RESPONSE",
            "2026-10-01T10:00:01Z",
            "I will read the README first.",
            thinking="look before leaping",
            tool_calls=[
                {
                    "id": "call_1",
                    "name": "view_file",
                    "args": {"AbsolutePath": "/home/u/proj/README.md"},
                }
            ],
            input_tokens=100,
            output_tokens=20,
            cache_read_tokens=0,
        ),
        _brain_line(2, "GENERIC", "2026-10-01T10:00:03Z", "File Path: README.md\n# proj"),
        _brain_line(
            3,
            "PLANNER_RESPONSE",
            "2026-10-01T10:00:04Z",
            tool_calls=[
                {
                    "id": "call_2",
                    "name": "run_command",
                    "args": {"CommandLine": "rm -rf /var/log/*.log", "Cwd": "/home/u/proj"},
                }
            ],
        ),
        dict(
            _brain_line(7, "USER_INPUT", "2026-10-01T10:00:09Z", "<injected reminder>"),
            source="USER_IMPLICIT",
        ),
        _brain_line(
            8, "PLANNER_RESPONSE", "2026-10-01T10:00:10Z", "Done. Three log files were removed."
        ),
    ]


def antigravity_app_brain_records() -> list[dict]:
    """The desktop-app conversation's shortened transcript.jsonl.

    The lines of database steps 0-4, the tool output shortened (`truncated_fields`).
    """
    return [
        _brain_line(0, "USER_INPUT", "2026-10-01T11:00:00Z", "which packages are outdated?"),
        _brain_line(
            1,
            "PLANNER_RESPONSE",
            "2026-10-01T11:00:01Z",
            "I will ask Homebrew.",
            thinking="brew knows",
            tool_calls=[
                {
                    "id": "call_7",
                    "name": "run_command",
                    "args": {"CommandLine": "brew outdated", "Cwd": "/home/u/proj"},
                }
            ],
        ),
        _brain_line(
            2,
            "GENERIC",
            "2026-10-01T11:00:02Z",
            "node (24.1.0) <",
            error="",
            truncated_fields=["content"],
        ),
        _brain_line(3, "SYSTEM_MESSAGE", "2026-10-01T11:00:04Z", "background task finished"),
        _brain_line(
            4,
            "ERROR_MESSAGE",
            "2026-10-01T11:00:05Z",
            "",
            error={"short_error": "model overloaded", "full_error": "HTTP 529"},
        ),
    ]


def antigravity_orphan_brain_records() -> list[dict]:
    """A conversation whose database is gone: only its brain mirror is left."""
    return [
        _brain_line(0, "USER_INPUT", "2026-10-01T10:30:00Z", "summarise the deleted project"),
        _brain_line(
            1,
            "PLANNER_RESPONSE",
            "2026-10-01T10:30:01.250Z",
            "I will list it first.",
            thinking="it may be gone",
            tool_calls=[
                {
                    "id": "call_o1",
                    "name": "list_directory",
                    "arguments_json": json.dumps({"DirectoryPath": "/home/u/gone"}),
                }
            ],
        ),
        dict(
            _brain_line(2, "GENERIC", "2026-10-01T10:30:02Z", "directory not found"),
            status="ERROR",
        ),
        _brain_line(3, "SYSTEM_MESSAGE", "2026-10-01T10:30:03Z", "background task finished"),
    ]


def build_antigravity_brain(gemini: Path) -> None:
    """The CLI's brain mirror.

    For the fixture conversation: transcript_full.jsonl (the copy read), a shortened
    transcript.jsonl and a transcript_full chunk with different text (copies not read), and a tool
    output file the parser must not want. For AGY_ORPHAN, whose conversation database is gone: two
    transcript_full chunks, the second line cut across the boundary, and a shortened
    transcript.jsonl that is not read.
    """
    logs = gemini / "antigravity-cli/brain" / AGY_CONVERSATION / ".system_generated/logs"
    _jsonl(logs / "transcript_full.jsonl", antigravity_brain_records())
    short = [dict(record, content="(not read)") for record in antigravity_brain_records()]
    _jsonl(logs / "transcript.jsonl", short)
    _jsonl(logs / "chunks/transcript_full/00000000.jsonl", short)
    out = logs.parent / "steps/2/output.txt"
    out.parent.mkdir(parents=True)
    out.write_text("File Path: README.md\n# proj\n", encoding="utf-8")
    orphan = gemini / "antigravity-cli/brain" / AGY_ORPHAN / ".system_generated/logs"
    blob = "".join(json.dumps(record) + "\n" for record in antigravity_orphan_brain_records())
    cut = blob.index("\n") + 40  # inside the second line
    chunks = orphan / "chunks/transcript_full"
    chunks.mkdir(parents=True)
    (chunks / "00000000.jsonl").write_text(blob[:cut], encoding="utf-8")
    (chunks / "00000001.jsonl").write_text(blob[cut:], encoding="utf-8")
    _jsonl(
        orphan / "transcript.jsonl",
        [dict(record, content="(not read)") for record in antigravity_orphan_brain_records()],
    )


def build_antigravity(
    base: Path,
    conversation: str = AGY_CONVERSATION,
    source: int = 17,
    steps: list[tuple[int, int, dict]] | None = None,
    start: int = AGY_T0,
    project_id: str = "default-cli-project",
    title: str = "",
    app_data_dir: str = "antigravity-cli",
    models: dict[str, list[int]] | None = None,
    cli: bool = True,
) -> None:
    """One Antigravity data directory.

    A conversation database, the summaries database and, for the CLI (`cli`), history.jsonl and the
    OAuth token file.
    """
    steps = antigravity_steps() if steps is None else steps
    if models is None:
        models = {"gemini-3.8-flash": [1, 3, 5, 8], "gemini-3.8-pro": [10, 12, 14]}
    conv = base / "conversations"
    conv.mkdir(parents=True, exist_ok=True)
    database = conv / (conversation + ".db")
    connection = sqlite3.connect(str(database))
    connection.executescript("""
    CREATE TABLE trajectory_meta (trajectory_id text, cascade_id text, trajectory_type integer, source integer, PRIMARY KEY (trajectory_id));
    CREATE TABLE steps (idx integer, step_type integer NOT NULL DEFAULT 0, status integer NOT NULL DEFAULT 0,
      has_subtrajectory numeric NOT NULL DEFAULT false, metadata blob, error_details blob, permissions blob, task_details blob,
      render_info blob, step_payload blob, step_format integer NOT NULL DEFAULT 0, PRIMARY KEY (idx));
    CREATE TABLE gen_metadata (idx integer, data blob, size integer NOT NULL DEFAULT 0, PRIMARY KEY (idx));
    CREATE TABLE trajectory_metadata_blob (id text DEFAULT "main", data blob, PRIMARY KEY (id));
    """)
    connection.execute(
        "INSERT INTO trajectory_meta VALUES (?,?,?,?)", ("traj-1", conversation, 4, source)
    )
    meta = encode(
        {
            "workspaces": [
                {"workspace_folder_absolute_uri": "file:///home/u/proj", "branch_name": "main"}
            ],
            "created_at": start - 1,
            "workspace_uris": ["file:///home/u/proj"],
            "project_id": project_id,
        },
        "CortexTrajectoryMetadata",
        AGY,
    )
    connection.execute("INSERT INTO trajectory_metadata_blob VALUES ('main', ?)", (meta,))
    for step_index, (step_type, status, step) in enumerate(steps):
        # A step's `permissions` go in the steps.permissions column, as on
        # the real installs; the payload is encoded without them.
        perms = step.get("permissions")
        payload = encode({k: value for k, value in step.items() if k != "permissions"}, "Step", AGY)
        metadata = encode(step["metadata"], "CortexStepMetadata", AGY)
        col = encode(perms, "TrajectoryPermissions", AGY) if perms else None
        connection.execute(
            "INSERT INTO steps (idx, step_type, status, metadata, step_payload, permissions)"
            " VALUES (?,?,?,?,?,?)",
            (step_index, step_type, status, metadata, payload, col),
        )
    for generator_index, (model, indices) in enumerate(models.items()):
        gen = encode(
            {"chat_model": {"response_model": model}, "step_indices": indices},
            "CortexStepGeneratorMetadata",
            AGY,
        )
        connection.execute(
            "INSERT INTO gen_metadata VALUES (?, ?, ?)", (generator_index, gen, len(gen))
        )
    connection.commit()
    connection.close()
    summ = sqlite3.connect(str(base / "conversation_summaries.db"))
    summ.executescript("""
    CREATE TABLE conversation_summaries (conversation_id text, title text NOT NULL DEFAULT "", preview text NOT NULL DEFAULT "",
      step_count integer NOT NULL DEFAULT 0, last_modified_time datetime NOT NULL, workspace_uris text NOT NULL,
      status text NOT NULL DEFAULT "", source text NOT NULL DEFAULT "", project_id text NOT NULL DEFAULT "",
      agent_name text NOT NULL DEFAULT "", parent_conversation_id text NOT NULL DEFAULT "", nesting_depth integer NOT NULL DEFAULT 0,
      battle_id text NOT NULL DEFAULT "", winning_conversation_id text NOT NULL DEFAULT "", not_fully_idle numeric NOT NULL DEFAULT false,
      killed numeric NOT NULL DEFAULT false, last_user_input_time datetime NOT NULL, last_user_input_step_index integer NOT NULL DEFAULT -1,
      app_data_dir text NOT NULL DEFAULT "", raw_summary blob, group_id text NOT NULL DEFAULT "", PRIMARY KEY (conversation_id));
    """)
    summ.execute(
        "INSERT INTO conversation_summaries (conversation_id, title, preview, step_count, last_modified_time, workspace_uris, status, "
        "project_id, last_user_input_time, app_data_dir) VALUES (?,?,?,?,?,?,?,?,?,?)",
        (
            conversation,
            title,
            "clean the logs" if cli else "",
            len(steps),
            "%s.002530482+00:00" % _utc_text(start + 13),
            json.dumps(["file:///home/u/proj"]),
            "CASCADE_RUN_STATUS_IDLE",
            project_id,
            "%s.000000000+00:00" % _utc_text(start),
            app_data_dir,
        ),
    )
    summ.commit()
    summ.close()
    if not cli:
        return
    _jsonl(
        base / "history.jsonl",
        [
            {"display": "clean the logs", "timestamp": start * 1000, "workspace": "/home/u/proj"},
            # CLI 1.3.x adds conversationId on some lines.
            {
                "conversationId": conversation,
                "display": "now the tests",
                "timestamp": (start + 20) * 1000,
                "workspace": "/home/u/proj",
            },
        ],
    )
    (base / "antigravity-oauth-token").write_text("secret", encoding="utf-8")


def build_antigravity_config(gemini: Path) -> None:
    """Write the standing permission files Antigravity keeps.

    The standing permission files Antigravity keeps, in the key shapes of the macOS and Ubuntu real
    installs: `.gemini/config/config.json`, a project file, a sidecar, the CLI's `settings.json`;
    plus files the parser must not want (`mcp_config.json`, a plugin manifest). Every file's mtime
    is AGY_CONFIG_MTIME.
    """
    files = {
        "config/config.json": {
            "plugins": {
                "gdocs": {
                    "installedFrom": {"id": "gdocs", "marketplace": "official", "version": "0.0.6"}
                }
            },
            "userSettings": {
                "globalPermissionGrants": {"allow": ["run_command(brew outdated)"]},
                "permissionGrantsV2Migrated": True,
                "remoteControlHostname": "do-not-print-host",
                "sandboxEnabledAtV2Migration": False,
            },
            "sidecars": {
                AGY_SIDECAR: {
                    "enabled": True,
                    "resolvedPermissions": {"accessGrants": ["run_command(brew outdated)"]},
                }
            },
        },
        "config/projects/%s.json" % AGY_APP_PROJECT: {
            "id": AGY_APP_PROJECT,
            "isWorkspaceOnly": False,
            "name": "do-not-print-project",
            "permissionGrants": {
                "permissionGrants": {"allow": ["write_to_file(/home/u/proj/notes.md)"]},
                "v2Migrated": True,
            },
            "projectResources": {"resources": [{"folderUri": "file:///home/u/proj"}]},
            "settings": {},
        },
        "config/projects/default-cli-project.json": {
            "id": "default-cli-project",
            "name": "Default",
            "projectResources": {},
        },
        "config/sidecars/%s/sidecar.json" % AGY_SIDECAR: {
            "builtin": "schedule",
            "display_name": "Weekly check",
            "description": "check for outdated packages",
            "args": ["--schedule", "0 9 * * 1", "--prompt", "do-not-print-prompt"],
            "restart_policy": "always",
            "agent_permissions": {"access_grants": ["run_command(brew outdated)"]},
        },
        "config/plugins/gdocs/plugin.json": {"name": "gdocs", "version": "0.0.6"},
        "antigravity-cli/settings.json": {
            "permissions": {"allow": ["run_command(git status)"]},
            "trustedWorkspaces": ["/home/u/proj"],
        },
    }
    for rel, doc in files.items():
        path = gemini / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(doc), encoding="utf-8")
    (gemini / "config/mcp_config.json").write_text("", encoding="utf-8")
    for rel in [*files, "config/mcp_config.json"]:
        os.utime(gemini / rel, (AGY_CONFIG_MTIME, AGY_CONFIG_MTIME))
