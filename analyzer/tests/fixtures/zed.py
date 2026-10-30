"""Synthetic Zed state for the analyzer tests."""

from __future__ import annotations

import json
import os
import sqlite3
import uuid
from pathlib import Path

import zstandard

ZED_THREAD = "6f1c0b2e-1111-4bbb-8ccc-000000000001"
ZED_EXTERNAL = "ext-claude-code-sess-1"
ZED_WORKTREE_THREAD = "6f1c0b2e-3333-4bbb-8ccc-000000000003"
ZED_TEMP_DIR = "/private/var/folders/xy/abc/T/zed-agent-terminal-Ab12Cd"
ZED_WORKTREE = "/srv/worktrees/fix-tests/proj"
ZED_DRAFT_KEY = "0b5e2d7a-4444-4ddd-9eee-000000000004"
ZED_SETTINGS_JSONC = """// Zed settings
{
  "agent": {
    "default_model": {},
    "tool_permissions": {
      "default": "confirm",
      "tools": {
        "terminal": {
          "always_allow": [{"pattern": "^cargo test"}],
          "always_deny": [{"pattern": "^rm -rf", "case_sensitive": false}],
        },
      },
    },
    "sandbox_permissions": {"network_hosts": ["*.npmjs.org"], "allow_unsandboxed": false},
  },
  "agent_servers": {"claude-acp": {"type": "registry", "default_mode": "plan"}},
  "session": {"trust_all_worktrees": false},
  "stable": {"agent": {"tool_permissions": {"default": "allow"}}},
}
"""


def zed_thread_record(cwd="/srv/proj"):
    """A `threads.data` blob, version 0.3.0, as in research/zed.md."""
    return {
        "version": "0.3.0",
        "title": "Fix flaky test",
        "updated_at": "2026-10-03T09:01:05Z",
        "initial_project_snapshot": {
            "worktree_snapshots": [
                {
                    "worktree_path": cwd,
                    "git_state": {
                        "remote_url": "git@github.com:alice/proj.git",
                        "head_sha": "abc123",
                        "current_branch": "main",
                        "diff": None,
                    },
                }
            ],
            "timestamp": "2026-10-03T09:00:00Z",
        },
        "model": {"provider": "anthropic", "model": "claude-sonnet-4-5"},
        "profile": "write",
        "sandbox_grants": {
            "write_paths": [
                cwd + "/build",
                {"requested": "/tmp/out", "resolved": "/private/tmp/out"},
            ],
            "network_hosts": ["*.npmjs.org"],
            "network_any_host": False,
            "allow_fs_write_all": False,
            "unsandboxed": True,
            "sandbox_fallback": False,
        },
        "messages": [
            {
                "User": {
                    "id": "9a7d6c5b-2222-4ddd-9eee-000000000002",
                    "content": [
                        {"Text": "run the tests"},
                        {"Mention": {"uri": "file://%s/README.md" % cwd, "content": ""}},
                    ],
                }
            },
            {
                "Agent": {
                    "content": [
                        {"Thinking": {"text": "use pytest", "signature": None}},
                        {
                            "ToolUse": {
                                "id": "toolu_01",
                                "name": "terminal",
                                "raw_input": '{"command":"pytest -q"}',
                                "input": {"type": "json", "value": {"command": "pytest -q"}},
                                "is_input_complete": True,
                                "thought_signature": None,
                            }
                        },
                        # A call the user denied: the error text of
                        # crates/agent/src/thread.rs:6905 at 9cd14da.
                        {
                            "ToolUse": {
                                "id": "toolu_02",
                                "name": "terminal",
                                "raw_input": '{"command":"rm -rf build"}',
                                "input": {"type": "json", "value": {"command": "rm -rf build"}},
                                "is_input_complete": True,
                                "thought_signature": None,
                            }
                        },
                        {"Text": "All 3 tests pass."},
                    ],
                    "tool_results": {
                        "toolu_01": {
                            "tool_use_id": "toolu_01",
                            "tool_name": "terminal",
                            "is_error": False,
                            "content": [{"Text": "3 passed"}],
                            "output": None,
                        },
                        "toolu_02": {
                            "tool_use_id": "toolu_02",
                            "tool_name": "terminal",
                            "is_error": True,
                            "content": [{"Text": "Permission to run tool denied by user"}],
                            "output": [{"Text": "Permission to run tool denied by user"}],
                        },
                    },
                    "reasoning_details": None,
                }
            },
            "Resume",
            {"Compaction": {"Summary": "ran the tests"}},
        ],
        # DbThread fields at 9cd14da (crates/agent/src/db.rs:79-83).
        "sandboxed_terminal_temp_dir": ZED_TEMP_DIR,
        "draft_prompt": [{"type": "text", "text": "and then lint"}],
    }


def zed_blob(record):
    """(data_type, data): zstd-compressed JSON, as Zed writes it."""
    raw = json.dumps(record).encode("utf-8")
    return "zstd", zstandard.ZstdCompressor(level=3).compress(raw)


def build_zed(home: Path) -> None:
    data = home / ".local/share/zed"
    (data / "threads").mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(data / "threads/threads.db"))
    connection.executescript("""
    CREATE TABLE threads (id TEXT PRIMARY KEY, summary TEXT NOT NULL, updated_at TEXT NOT NULL,
      data_type TEXT NOT NULL, data BLOB NOT NULL);
    ALTER TABLE threads ADD COLUMN parent_id TEXT;
    ALTER TABLE threads ADD COLUMN folder_paths TEXT;
    ALTER TABLE threads ADD COLUMN folder_paths_order TEXT;
    ALTER TABLE threads ADD COLUMN created_at TEXT;
    """)
    dtype, blob = zed_blob(zed_thread_record())
    connection.execute(
        "INSERT INTO threads(id,parent_id,folder_paths,folder_paths_order,summary,updated_at,data_type,data,created_at) "
        "VALUES (?,?,?,?,?,?,?,?,?)",
        (
            ZED_THREAD,
            None,
            "/srv/proj",
            "0",
            "Fix flaky test",
            "2026-10-03T09:01:05.000000000+00:00",
            dtype,
            blob,
            "2026-10-03T09:00:00.000000000+00:00",
        ),
    )
    connection.commit()
    connection.close()
    (data / "db/0-stable").mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(str(data / "db/0-stable/db.sqlite"))
    connection.executescript("""
    CREATE TABLE sidebar_threads(thread_id BLOB PRIMARY KEY, session_id TEXT, agent_id TEXT, title TEXT NOT NULL,
      updated_at TEXT NOT NULL, created_at TEXT, folder_paths TEXT, folder_paths_order TEXT, archived INTEGER DEFAULT 0,
      main_worktree_paths TEXT, main_worktree_paths_order TEXT, remote_connection TEXT);
    ALTER TABLE sidebar_threads ADD COLUMN interacted_at TEXT;
    ALTER TABLE sidebar_threads ADD COLUMN title_override TEXT;
    """)
    connection.execute(
        "INSERT INTO sidebar_threads(thread_id,session_id,agent_id,title,updated_at,created_at,folder_paths,"
        "folder_paths_order,archived) VALUES (?,?,?,?,?,?,?,?,?)",
        (
            os.urandom(16),
            ZED_THREAD,
            None,
            "Fix flaky test",
            "2026-10-03T09:01:05+00:00",
            "2026-10-03T09:00:00+00:00",
            "/srv/proj",
            "0",
            0,
        ),
    )
    connection.execute(
        "INSERT INTO sidebar_threads(thread_id,session_id,agent_id,title,updated_at,folder_paths,folder_paths_order) "
        "VALUES (?,?,?,?,?,?,?)",
        (
            os.urandom(16),
            ZED_EXTERNAL,
            "claude-code",
            "External thread",
            "2026-10-03T10:00:00+00:00",
            "/srv/proj",
            "0",
        ),
    )
    build_zed_database_stores(connection)
    connection.commit()
    connection.close()
    (home / ".config/zed").mkdir(parents=True, exist_ok=True)
    (home / ".config/zed/settings.json").write_text(ZED_SETTINGS_JSONC, encoding="utf-8")
    # Noise the parser must not want.
    (home / ".config/zed/keymap.json").write_text("[]", encoding="utf-8")


def build_zed_database_stores(connection: sqlite3.Connection) -> None:
    """Write the other db.sqlite stores of research/zed.md.

    The other db.sqlite stores of research/zed.md (Other db.sqlite tables, Permission settings and
    project trust), as zed-industries/zed at 9cd14da creates them: a draft thread and its prompt, an
    orphan draft, a thread run in a Zed-created worktree that was archived, the worktree registry,
    project trust and a sidebar terminal.
    """
    connection.executescript("""
    CREATE TABLE trusted_worktrees (trust_id INTEGER PRIMARY KEY AUTOINCREMENT,
      absolute_path TEXT, user_name TEXT, host_name TEXT) STRICT;
    CREATE TABLE scoped_kv_store(namespace TEXT NOT NULL, key TEXT NOT NULL,
      value TEXT NOT NULL, PRIMARY KEY(namespace, key)) STRICT;
    CREATE TABLE archived_git_worktrees(id INTEGER PRIMARY KEY, worktree_path TEXT NOT NULL,
      main_repo_path TEXT NOT NULL, branch_name TEXT, staged_commit_hash TEXT,
      unstaged_commit_hash TEXT, original_commit_hash TEXT) STRICT;
    CREATE TABLE thread_archived_worktrees(thread_id BLOB NOT NULL,
      archived_worktree_id INTEGER NOT NULL REFERENCES archived_git_worktrees(id),
      PRIMARY KEY (thread_id, archived_worktree_id)) STRICT;
    CREATE TABLE sidebar_terminal_threads(terminal_id TEXT PRIMARY KEY, title TEXT NOT NULL,
      custom_title TEXT, created_at TEXT NOT NULL, working_directory TEXT, folder_paths TEXT,
      folder_paths_order TEXT, main_worktree_paths TEXT, main_worktree_paths_order TEXT,
      remote_connection TEXT) STRICT;
    """)
    draft_id = uuid.UUID(ZED_DRAFT_KEY).bytes
    worktree_thread_id = os.urandom(16)
    sidebar = (
        "INSERT INTO sidebar_threads(thread_id,session_id,agent_id,title,updated_at,folder_paths,"
        "folder_paths_order,archived,main_worktree_paths,interacted_at) VALUES (?,?,?,?,?,?,?,?,?,?)"
    )
    connection.execute(
        sidebar,
        (
            draft_id,
            None,
            None,
            "New Thread",
            "2026-10-03T11:00:00+00:00",
            "/srv/proj",
            "0",
            0,
            None,
            None,
        ),
    )
    connection.execute(
        sidebar,
        (
            worktree_thread_id,
            ZED_WORKTREE_THREAD,
            None,
            "Fix tests in a worktree",
            "2026-10-03T12:00:00+00:00",
            ZED_WORKTREE,
            "0",
            1,
            "/srv/proj",
            "2026-10-03T11:59:00+00:00",
        ),
    )
    insert_scoped = "INSERT INTO scoped_kv_store(namespace,key,value) VALUES (?,?,?)"
    connection.execute(
        insert_scoped,
        (
            "agent_draft_prompts",
            ZED_DRAFT_KEY,
            '[{"type":"text","text":"refactor the parser"},'
            '{"type":"resource_link","name":"main.py","uri":"file:///srv/proj/main.py"}]',
        ),
    )
    connection.execute(
        insert_scoped,
        (
            "agent_draft_prompts",
            "0b5e2d7a-5555-4ddd-9eee-000000000005",
            '[{"type":"text","text":"an orphan draft"}]',
        ),
    )
    connection.execute(
        insert_scoped,
        (
            "created_git_worktrees",
            "local\n" + ZED_WORKTREE,
            '{"created_at_seconds":1791025200,"created_at_subsec_nanos":500000000}',
        ),
    )
    # Another namespace the parser does not read.
    connection.execute(insert_scoped, ("agent_panel", "width", "640"))
    connection.execute(
        "INSERT INTO trusted_worktrees(absolute_path,user_name,host_name) VALUES (?,?,?)",
        ("/srv/proj", None, None),
    )
    connection.execute(
        "INSERT INTO trusted_worktrees(absolute_path,user_name,host_name) VALUES (?,?,?)",
        ("/home/alice/remote", "alice", "build.example"),
    )
    connection.execute(
        "INSERT INTO archived_git_worktrees VALUES (?,?,?,?,?,?,?)",
        (1, ZED_WORKTREE, "/srv/proj", "fix-tests", "a1b2c3", "d4e5f6", "0a0b0c"),
    )
    connection.execute(
        "INSERT INTO thread_archived_worktrees VALUES (?,?)", (worktree_thread_id, 1)
    )
    connection.execute(
        "INSERT INTO sidebar_terminal_threads(terminal_id,title,custom_title,created_at,"
        "working_directory,folder_paths,folder_paths_order) VALUES (?,?,?,?,?,?,?)",
        (
            "7c1d0e2f-6666-4fff-8aaa-000000000006",
            "zsh",
            None,
            "2026-10-03T10:30:00+00:00",
            "/srv/proj",
            "/srv/proj",
            "0",
        ),
    )


def zed_unknown_variants_record():
    """A version 0.3.0 Zed thread blob with unknown items mixed in.

    A version 0.3.0 thread blob whose known messages follow the `UserMessage`, `AgentMessage` and
    `Message` enums of zed-industries/zed at a84689073d29 (crates/agent/src/thread.rs:202-208,
    287-301, 743-759; research/zed.md, Record schema), with items no release writes today mixed in:
    a message variant, a `Compaction` variant, an `Agent` content variant, a content item and a
    `tool_results` entry that are not objects, and a legacy segment type. Each must become its own
    `parser:` row while the messages around them still yield theirs.
    """
    return {
        "version": "0.3.0",
        "title": "Unknown variants",
        "messages": [
            {"User": {"id": "u1", "content": [{"Text": "before the unknown"}]}},
            {"Checkpoint": {"id": "c1", "label": "naïve"}},
            {
                "Agent": {
                    "content": [
                        {"Text": "agent text"},
                        {"Citation": {"url": "https://example.com"}},
                        42,
                    ],
                    "tool_results": {"toolu_x": "not an object"},
                }
            },
            7,
            {"Compaction": {"Remote": {"id": "r1"}}},
            {"role": "assistant", "segments": [{"type": "audio", "data": "AAA="}]},
            {"User": {"id": "u2", "content": [{"Text": "after the unknown"}]}},
        ],
    }
