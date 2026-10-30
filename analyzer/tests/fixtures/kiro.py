"""Synthetic Kiro state for the analyzer tests."""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path

KIRO_SESSION = "3f2b8c1e-9d7a-4e6b-b1c2-0a9f8e7d6c5b"
KIRO_EXPORT_SESSION = "7c1d2e3f-4a5b-4c6d-8e7f-901234567890"
KIRO_SHELL_SESSION = "fig-shell-1"


def _kiro_user(content, timestamp=None, cwd="/srv/proj"):
    return {
        "additional_context": "",
        "env_context": {
            "env_state": {
                "operating_system": "linux",
                "current_working_directory": cwd,
                "environment_variables": [],
            }
        },
        "content": content,
        "timestamp": timestamp,
        "images": None,
    }


def _kiro_meta(number, start_ms, end_ms, kind="NotToolUse", tools=(), tags=()):
    return {
        "request_id": "req-%03d" % number,
        "message_id": "msg-%03d" % number,
        "request_start_timestamp_ms": start_ms,
        "stream_end_timestamp_ms": end_ms,
        "time_to_first_chunk": None,
        "time_between_chunks": [],
        "user_prompt_length": 0,
        "response_size": 0,
        "chat_conversation_type": kind,
        "tool_use_ids_and_names": [list(tool) for tool in tools],
        "model_id": "claude-sonnet-4",
        "message_meta_tags": list(tags),
    }


def kiro_conversation_records():
    """The ConversationState of research/kiro.md, Parser plan.

    The ConversationState from research/kiro.md, Parser plan, with the project moved to /srv/proj.
    The research sample's user timestamp was six hours off its own request metadata; it is corrected
    here so the turns are in order.
    """
    return {
        "conversation_id": KIRO_SESSION,
        "next_message": None,
        "history": [
            {
                "user": _kiro_user(
                    {"Prompt": {"prompt": "list the files here"}},
                    "2026-04-23T20:17:15.123456789-07:00",
                ),
                "assistant": {
                    "ToolUse": {
                        "message_id": "msg-001",
                        "content": "I will list the directory.",
                        "tool_uses": [
                            {
                                "id": "tooluse_abc123",
                                "name": "execute_bash",
                                "orig_name": "execute_bash",
                                "args": {"command": "ls -la"},
                                "orig_args": {"command": "ls -la"},
                            }
                        ],
                    }
                },
                "request_metadata": _kiro_meta(
                    1, 1777000635200, 1777000636900, "ToolUse", [("tooluse_abc123", "execute_bash")]
                ),
            },
            {
                "user": _kiro_user(
                    {
                        "ToolUseResults": {
                            "tool_use_results": [
                                {
                                    "tool_use_id": "tooluse_abc123",
                                    "content": [{"Text": "total 8\nREADME.md"}],
                                    "status": "Success",
                                }
                            ]
                        }
                    }
                ),
                "assistant": {
                    "Response": {
                        "message_id": "msg-002",
                        "content": "The directory contains README.md.",
                    }
                },
                "request_metadata": dict(
                    _kiro_meta(2, 1777000637000, 1777000638100), model_id="claude-opus-4"
                ),
            },
        ],
        "valid_history_range": [0, 2],
        "transcript": ["> list the files here", "The directory contains README.md."],
        "tools": {"native___": []},
        "context_manager": None,
        "context_message_length": None,
        "latest_summary": None,
        "model_info": {"model_id": "claude-sonnet-4", "model_name": "Claude Sonnet 4"},
        "file_line_tracker": {},
        "checkpoint_manager": None,
        "mcp_enabled": True,
    }


def kiro_export_records():
    """A /save export exercising the less common variants.

    An MCP tool whose name differs from orig_name, a Json result, a cancelled tool use, a Compact
    tag, latest_summary, a pending next_message and the legacy `model` field instead of model_info.
    """
    return {
        "conversation_id": KIRO_EXPORT_SESSION,
        "next_message": _kiro_user(
            {"Prompt": {"prompt": "now push it"}}, "2026-04-24T08:00:00-07:00"
        ),
        "history": [
            {
                "user": _kiro_user(
                    {"Prompt": {"prompt": "open an issue"}}, "2026-04-24T07:00:00.000-07:00"
                ),
                "assistant": {
                    "ToolUse": {
                        "message_id": "msg-010",
                        "content": "",
                        "tool_uses": [
                            {
                                "id": "tooluse_mcp1",
                                "name": "github___create_issue",
                                "orig_name": "create_issue",
                                "args": {"title": "bug"},
                                "orig_args": {"title": "bug"},
                            }
                        ],
                    }
                },
                "request_metadata": dict(
                    _kiro_meta(10, 1777039201000, 1777039202000, "ToolUse"), model_id=None
                ),
            },
            {
                "user": _kiro_user(
                    {
                        "ToolUseResults": {
                            "tool_use_results": [
                                {
                                    "tool_use_id": "tooluse_mcp1",
                                    "content": [{"Json": {"number": 7}}],
                                    "status": "Error",
                                }
                            ]
                        }
                    }
                ),
                "assistant": {
                    "ToolUse": {
                        "message_id": "msg-011",
                        "content": "Retrying.",
                        "tool_uses": [
                            {
                                "id": "tooluse_bash2",
                                "name": "execute_bash",
                                "orig_name": "execute_bash",
                                "args": {"command": "rm -rf build"},
                                "orig_args": {"command": "rm -rf build"},
                            }
                        ],
                    }
                },
                "request_metadata": _kiro_meta(11, 1777039203000, 1777039204000, "ToolUse"),
            },
            {
                "user": _kiro_user(
                    {
                        "CancelledToolUses": {
                            "prompt": "stop, do not delete",
                            "tool_use_results": [
                                {
                                    "tool_use_id": "tooluse_bash2",
                                    "content": [{"Text": "Tool use was cancelled by the user"}],
                                    "status": "Error",
                                }
                            ],
                        }
                    },
                    "2026-04-24T07:00:10.000-07:00",
                ),
                "assistant": {"Response": {"message_id": "msg-012", "content": "Stopped."}},
                "request_metadata": _kiro_meta(12, 1777039211000, 1777039212000, tags=["Compact"]),
            },
        ],
        "valid_history_range": [0, 3],
        "transcript": [],
        "tools": {"native___": []},
        "latest_summary": [
            "User asked to open an issue and cancelled a delete.",
            _kiro_meta(13, 1777039213000, 1777039214000),
        ],
        "model": "claude-3.7-sonnet",
    }


KIRO_DENIAL_SESSION = "9e8d7c6b-5a49-4382-9170-6f5e4d3c2b1a"
KIRO_DENY_TEXT = "I deny this tool request. Ask a follow up question clarifying the expected action"
KIRO_RULE_TEXT = (
    "Tool use with execute_bash was rejected because the arguments supplied were forbidden"
)


def _kiro_tool_entry(number, prompt_content, timestamp, tool_id, command, start_ms):
    """A history entry whose assistant message asks for one execute_bash call."""
    return {
        "user": _kiro_user(prompt_content, timestamp),
        "assistant": {
            "ToolUse": {
                "message_id": "msg-%03d" % number,
                "content": "",
                "tool_uses": [
                    {
                        "id": tool_id,
                        "name": "execute_bash",
                        "orig_name": "execute_bash",
                        "args": {"command": command},
                        "orig_args": {"command": command},
                    }
                ],
            }
        },
        "request_metadata": _kiro_meta(number, start_ms, start_ms + 500, "ToolUse"),
    }


def _kiro_cancelled(prompt, tool_id):
    return {
        "CancelledToolUses": {
            "prompt": prompt,
            "tool_use_results": [
                {
                    "tool_use_id": tool_id,
                    "content": [{"Text": "Tool use was cancelled by the user"}],
                    "status": "Error",
                }
            ],
        }
    }


def kiro_denial_records():
    """A /save export for the denial and checkpoint rows, from the Amazon Q source at 15cc8f3.

    A bare `n` denial, a deny-rule rejection as a CancelledToolUses prompt and as a plain Prompt, an
    interrupt, a checkpoint whose snapshot keeps a turn a restore discarded, and a tangent
    held-aside main history.
    """
    first = _kiro_tool_entry(
        20,
        {"Prompt": {"prompt": "clean the build"}},
        "2026-04-25T09:00:00.000-07:00",
        "tooluse_d1",
        "rm -rf build",
        1777132800000,
    )
    second = _kiro_tool_entry(
        21,
        _kiro_cancelled(KIRO_DENY_TEXT, "tooluse_d1"),
        "2026-04-25T09:00:05.000-07:00",
        "tooluse_d2",
        "rm -rf /",
        1777132805000,
    )
    third = _kiro_tool_entry(
        22,
        _kiro_cancelled(KIRO_RULE_TEXT, "tooluse_d2"),
        "2026-04-25T09:00:10.000-07:00",
        "tooluse_d3",
        "sleep 100",
        1777132810000,
    )
    fourth = {
        "user": _kiro_user(
            _kiro_cancelled("The user interrupted the tool execution.", "tooluse_d3"),
            "2026-04-25T09:00:15.000-07:00",
        ),
        "assistant": {"Response": {"message_id": "msg-023", "content": "Interrupted."}},
        "request_metadata": _kiro_meta(23, 1777132815000, 1777132815500),
    }
    fifth = {
        "user": _kiro_user({"Prompt": {"prompt": KIRO_RULE_TEXT}}, "2026-04-25T09:00:20.000-07:00"),
        "assistant": {"Response": {"message_id": "msg-024", "content": "Understood."}},
        "request_metadata": _kiro_meta(24, 1777132820000, 1777132820500),
    }
    discarded = {
        "user": _kiro_user(
            {"Prompt": {"prompt": "the turn a restore discarded"}},
            "2026-04-25T08:59:00.000-07:00",
        ),
        "assistant": {"Response": {"message_id": "msg-019", "content": "Discarded reply."}},
        "request_metadata": dict(_kiro_meta(19, 1777132740000, 1777132740500), model_id="m-old"),
    }
    aside = {
        "user": _kiro_user(
            {"Prompt": {"prompt": "the main conversation"}}, "2026-04-25T08:58:00.000-07:00"
        ),
        "assistant": {"Response": {"message_id": "msg-018", "content": "Main reply."}},
        "request_metadata": _kiro_meta(18, 1777132680000, 1777132680500),
    }
    return {
        "conversation_id": KIRO_DENIAL_SESSION,
        "next_message": None,
        "history": [first, second, third, fourth, fifth],
        "valid_history_range": [0, 5],
        "transcript": [],
        "tools": {"native___": []},
        "latest_summary": None,
        "model_info": {"model_id": "claude-sonnet-4"},
        "checkpoint_manager": {
            "shadow_repo_path": "/home/alice/.aws/amazonq/cli-checkouts/" + KIRO_DENIAL_SESSION,
            "work_tree_path": "/srv/proj",
            "checkpoints": [
                {
                    "tag": "1",
                    "timestamp": "2026-04-25T08:59:30.000-07:00",
                    "description": "the turn a restore discarded",
                    "history_snapshot": [discarded, first],
                    "is_turn": True,
                    "tool_name": None,
                },
                {
                    "tag": "1.1",
                    "timestamp": "2026-04-25T09:00:01.000-07:00",
                    "description": "rm -rf build",
                    "history_snapshot": [first],
                    "is_turn": False,
                    "tool_name": "execute_bash",
                },
            ],
            "tag_index": {"1": 0, "1.1": 1},
            "current_turn": 2,
            "tools_in_turn": 0,
            "pending_user_message": None,
            "message_locked": False,
        },
        "tangent_state": {
            "main_history": [aside],
            "main_next_message": None,
            "main_transcript": [],
            "main_latest_summary": None,
        },
    }


def kiro_delegate_records():
    """The delegate tool's `.amazonq/.subagents/<agent>.json` records.

    The delegate tool's `.amazonq/.subagents/<agent>.json` records, which the CLI writes in the
    working directory, so a test places them in a project: one finished task of the default agent
    and one still running with the `Unknown` cwd default.
    """
    return {
        "q_cli_default.json": {
            "agent": "q_cli_default",
            "task": "run the tests",
            "status": "completed",
            "launched_at": 1777132900,
            "completed_at": 1777132960,
            "pid": 4711,
            "exit_code": 0,
            "output": "12 passed",
            "user_notified": True,
            "summary": "All tests pass.",
            "cwd": "/srv/proj",
        },
        "reviewer.json": {
            "agent": "reviewer",
            "task": "review the diff",
            "status": "running",
            "launched_at": 1777132970,
            "completed_at": None,
            "pid": 4712,
            "exit_code": None,
            "output": "",
            "user_notified": False,
            "summary": None,
            "cwd": "Unknown",
        },
    }


# The chat prompt's rustyline history: the #V2 header, a prompt the
# conversation also holds, a slash command, an entry with an escaped line
# feed and one with an escaped backslash, then a line that is not UTF-8 and
# one after it.
KIRO_PROMPT_HISTORY = (
    b"#V2\nlist the files here\n/clear\nfix the\\nbuild\npath C:\\\\tmp\n"
    b"\xff\xfe not utf-8\nafter the cut\n"
)


def build_kiro(home: Path) -> None:
    base = home / ".local/share/amazon-q"
    base.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(base / "data.sqlite3"))
    connection.executescript("""
    CREATE TABLE migrations (id INTEGER PRIMARY KEY, version INTEGER NOT NULL, migration_time INTEGER NOT NULL);
    CREATE TABLE history (id INTEGER PRIMARY KEY, command TEXT, shell TEXT, pid INTEGER, session_id TEXT, cwd TEXT,
      start_time INTEGER, hostname TEXT, exit_code INTEGER, end_time INTEGER, duration INTEGER);
    CREATE TABLE state (key TEXT PRIMARY KEY, value BLOB);
    CREATE TABLE auth_kv (key TEXT PRIMARY KEY, value TEXT);
    CREATE TABLE conversations (key TEXT PRIMARY KEY, value TEXT);
    """)
    connection.execute(
        "INSERT INTO conversations (key, value) VALUES (?, ?)",
        ("/srv/proj", json.dumps(kiro_conversation_records())),
    )
    connection.execute(
        "INSERT INTO conversations (key, value) VALUES (?, ?)", ("/srv/broken", "not json")
    )
    connection.execute(
        "INSERT INTO history (command, shell, pid, session_id, cwd, start_time, hostname, exit_code, end_time, "
        "duration) VALUES (?,?,?,?,?,?,?,?,?,?)",
        (
            "git status",
            "zsh",
            4242,
            KIRO_SHELL_SESSION,
            "/srv/proj",
            1776999600,
            "ws1",
            0,
            1776999601,
            1000,
        ),
    )
    connection.execute(
        "INSERT INTO auth_kv VALUES ('codewhisperer:odic:token', '{\"access_token\":\"REDACT-ME\"}')"
    )
    connection.commit()
    connection.close()
    exports = home / ".aws/amazonq/exports"
    exports.mkdir(parents=True, exist_ok=True)
    text = json.dumps(kiro_export_records(), indent=2)
    (exports / "issue-chat.json").write_text(text, encoding="utf-8")
    # an export cut mid-write, inside the third history entry
    (exports / "issue-chat-cut.json").write_text(
        text[: text.index("stop, do not delete")], encoding="utf-8"
    )
    (exports / "denials.json").write_text(
        json.dumps(kiro_denial_records(), indent=2), encoding="utf-8"
    )
    (home / ".aws/amazonq/.cli_bash_history").write_bytes(KIRO_PROMPT_HISTORY)
    agents = home / ".aws/amazonq/cli-agents"
    agents.mkdir(parents=True, exist_ok=True)
    (agents / "dev.json").write_text(
        json.dumps(
            {
                "name": "dev",
                "tools": ["*"],
                "allowedTools": ["fs_read", "@git"],
                "toolsSettings": {
                    "execute_bash": {
                        "allowedCommands": ["git status"],
                        "deniedCommands": ["rm -rf .*"],
                        "autoAllowReadonly": True,
                    },
                    "fs_write": {"deniedPaths": ["/etc"]},
                },
                "model": "claude-sonnet-4",
            }
        ),
        encoding="utf-8",
    )
    (agents / "broken.json").write_text("{not json", encoding="utf-8")
    # noise the parser must not want: CLI settings, and a Kiro CLI session
    # file whose format is unverified even though it looks like an export
    (home / ".kiro/settings").mkdir(parents=True, exist_ok=True)
    (home / ".kiro/settings/cli.json").write_text('{"chat.defaultModel": "auto"}', encoding="utf-8")
    (home / ".kiro/sessions").mkdir(parents=True, exist_ok=True)
    (home / ".kiro/sessions/s1.json").write_text(
        json.dumps(
            {
                "conversation_id": "x",
                "history": [],
                "messages": [{"role": "user", "content": [{"text": "hi"}]}],
            }
        ),
        encoding="utf-8",
    )
