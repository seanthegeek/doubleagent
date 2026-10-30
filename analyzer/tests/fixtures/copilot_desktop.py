"""Synthetic GitHub Copilot Desktop state for the analyzer tests."""

from __future__ import annotations

import os
from pathlib import Path

from .common import _wal_database
from .copilot_cli import COPILOT_SESSION

# GitHub Copilot Desktop: the app's own store `.copilot/data.db`,
# with the table and column names of research/copilot-desktop.md, Record schema
# (a subset of each table's columns, names exact). The project session's id
# is the Copilot CLI fixture's session, since the app's sessions are the
# bundled CLI's session-state directories. The rows live only in the -wal
# sidecar, as on a host where the app is still running. The four scheduling
# tables and the deletion intent were empty on the real install: their rows
# are fixture only.
COPILOT_DESKTOP_DATABASE_REL = ".copilot/data.db"
COPILOT_DESKTOP_CHAT = "5d404e14-0000-4000-8000-000000000001"
COPILOT_DESKTOP_FORK = "9f9f9f9f-0000-4000-8000-000000000001"
COPILOT_DESKTOP_WORKTREE = "/home/alice/.copilot/repos/copilot-worktrees/proj/octo-user-happy-otter"
COPILOT_DESKTOP_MTIME = 1790850600  # 1 October 10:30:00 UTC
# Checkpoint refs in the app's clone of the project: one well
# formed, at the project session's checkpoint, and one malformed.
COPILOT_DESKTOP_REF_DIR = ".copilot/repos/proj/.git/refs/copilot/checkpoints/" + COPILOT_SESSION
COPILOT_DESKTOP_REF_REL = (
    COPILOT_DESKTOP_REF_DIR + "/00000000000000000001/e0000000-0000-4000-8000-000000000004"
)
COPILOT_DESKTOP_REF_MTIME = 1790845201  # 1 October 09:00:01 UTC
COPILOT_DESKTOP_BAD_REF_REL = (
    COPILOT_DESKTOP_REF_DIR + "/00000000000000000002/e0000000-0000-4000-8000-000000000020"
)
COPILOT_DESKTOP_BAD_REF_MTIME = 1790845230  # 1 October 09:00:30 UTC
COPILOT_DESKTOP_HEAD_REF_REL = ".copilot/repos/proj/.git/refs/heads/main"

COPILOT_DESKTOP_SCHEMA = """
CREATE TABLE "accounts" (id TEXT PRIMARY KEY NOT NULL, login TEXT NOT NULL, name TEXT,
  is_default INTEGER NOT NULL DEFAULT 0, host TEXT NOT NULL DEFAULT 'github.com',
  token_updated_at TEXT, created_at TEXT, updated_at TEXT, access_token TEXT,
  kind TEXT NOT NULL DEFAULT 'github', settings_json TEXT NOT NULL DEFAULT '{}',
  parent_account_id TEXT REFERENCES accounts(id) ON DELETE CASCADE);
CREATE TABLE settings (id INTEGER PRIMARY KEY CHECK (id = 1), last_active_workspace_id TEXT,
  storage_location TEXT NOT NULL DEFAULT '', instructions TEXT NOT NULL DEFAULT '',
  github_access_token TEXT, github_token_type TEXT, github_scope TEXT,
  github_token_updated_at TEXT);
CREATE TABLE sessions (id TEXT PRIMARY KEY NOT NULL, title TEXT,
  session_type TEXT NOT NULL DEFAULT 'workspace', mode TEXT, model TEXT, reasoning_effort TEXT,
  is_running INTEGER NOT NULL DEFAULT 0, was_interrupted INTEGER NOT NULL DEFAULT 0,
  interruption_reason TEXT, created_at TEXT, updated_at TEXT,
  remote_control_enabled INTEGER NOT NULL DEFAULT 0, auto_approve INTEGER NOT NULL DEFAULT 1,
  enabled_experiments_json TEXT, forked_from_session_id TEXT, archived_at TEXT,
  permission_mode TEXT);
CREATE TABLE projects (id TEXT PRIMARY KEY NOT NULL, name TEXT NOT NULL,
  container_kind TEXT NOT NULL DEFAULT 'repository'
    CHECK (container_kind IN ('folder', 'repository', 'collection')),
  main_repo_path TEXT NOT NULL UNIQUE, default_branch TEXT NOT NULL DEFAULT 'main',
  github_owner TEXT, github_repo TEXT, github_account_id TEXT, tab_order INTEGER,
  created_at TEXT, last_opened_at TEXT);
CREATE TABLE worktrees (id TEXT PRIMARY KEY NOT NULL,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE, path TEXT NOT NULL,
  branch TEXT NOT NULL, base_branch TEXT, created_at TEXT);
CREATE TABLE workspaces (id TEXT PRIMARY KEY NOT NULL,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  worktree_id TEXT REFERENCES worktrees(id) ON DELETE SET NULL,
  workspace_type TEXT NOT NULL CHECK (workspace_type IN ('worktree', 'branch', 'folder', 'cloud')),
  branch TEXT NOT NULL, name TEXT NOT NULL, tab_order INTEGER NOT NULL DEFAULT 0,
  port INTEGER NOT NULL DEFAULT 0, created_at TEXT, updated_at TEXT,
  source_pr_repo_full_name TEXT, source_pr_number INTEGER, created_pr_repo_full_name TEXT,
  created_pr_number INTEGER);
CREATE TABLE workspace_session_aliases (session_id TEXT PRIMARY KEY NOT NULL,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, created_at TEXT);
CREATE TABLE activity_items (id TEXT PRIMARY KEY NOT NULL,
  workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE, session_id TEXT,
  activity_type TEXT NOT NULL, preview TEXT NOT NULL DEFAULT '',
  is_read INTEGER NOT NULL DEFAULT 0, created_at TEXT, updated_at TEXT,
  metadata_json TEXT NOT NULL DEFAULT '{}');
CREATE TABLE session_checkpoints (session_id TEXT NOT NULL, event_id TEXT NOT NULL,
  commit_sha TEXT NOT NULL, created_at TEXT, PRIMARY KEY (session_id, event_id));
CREATE TABLE "model_providers" (id TEXT PRIMARY KEY NOT NULL, name TEXT NOT NULL,
  created_at TEXT, updated_at TEXT, type TEXT NOT NULL DEFAULT 'openai',
  settings_json TEXT NOT NULL DEFAULT '{}', account_id TEXT);
CREATE TABLE app_state (key TEXT PRIMARY KEY NOT NULL, value TEXT NOT NULL, updated_at TEXT);
CREATE TABLE my_copilot_scheduled_tasks (id TEXT PRIMARY KEY NOT NULL,
  revision INTEGER NOT NULL DEFAULT 1, title TEXT NOT NULL, prompt TEXT NOT NULL,
  enabled INTEGER NOT NULL, execution_mode TEXT NOT NULL,
  background_agent_type TEXT NOT NULL DEFAULT 'general-purpose', created_at TEXT,
  updated_at TEXT, archived_at TEXT);
CREATE TABLE my_copilot_scheduled_task_triggers (id TEXT PRIMARY KEY NOT NULL,
  task_id TEXT NOT NULL, kind TEXT NOT NULL, enabled INTEGER NOT NULL,
  config_version INTEGER NOT NULL DEFAULT 1, config_json TEXT NOT NULL,
  state_version INTEGER NOT NULL DEFAULT 1, state_json TEXT NOT NULL DEFAULT '{}',
  next_run_at TEXT, last_fired_at TEXT, created_at TEXT, updated_at TEXT, archived_at TEXT);
CREATE TABLE my_copilot_scheduled_task_invocations (id TEXT PRIMARY KEY NOT NULL,
  task_id TEXT NOT NULL, trigger_id TEXT, occurrence_key TEXT NOT NULL UNIQUE,
  source TEXT NOT NULL, scheduled_for TEXT, task_revision INTEGER NOT NULL,
  task_title TEXT NOT NULL, prompt_snapshot TEXT NOT NULL, trigger_snapshot_json TEXT,
  execution_mode TEXT NOT NULL, parent_session_id TEXT, runtime_agent_id TEXT,
  status TEXT NOT NULL, claimed_at TEXT NOT NULL, dispatched_at TEXT);
CREATE TABLE "session_automations" (session_id TEXT PRIMARY KEY NOT NULL, prompt TEXT NOT NULL,
  interval TEXT NOT NULL CHECK (interval IN ('once', 'minutes', 'daily', 'weekly')),
  every_minutes INTEGER, schedule_hour INTEGER NOT NULL DEFAULT 9,
  schedule_day INTEGER NOT NULL DEFAULT 1, last_run_at TEXT, next_run_at TEXT, updated_at TEXT,
  name TEXT, owner_session_id TEXT, paused_at TEXT);
CREATE TABLE session_deletion_intents (session_id TEXT PRIMARY KEY NOT NULL, created_at TEXT);
"""


def copilot_desktop_records() -> list[str]:
    """The INSERTs of the Copilot Desktop fixture `data.db`.

    The INSERTs of the fixture `data.db`, in the order the parser reads the tables;
    `build_copilot_desktop` writes them to the -wal sidecar.
    """
    session, chat, fork = COPILOT_SESSION, COPILOT_DESKTOP_CHAT, COPILOT_DESKTOP_FORK
    return [
        "INSERT INTO accounts VALUES ('acc1', 'octo-user', 'Octo User', 1, 'github.com',"
        " '2026-10-01T08:50:00.000Z', '2026-10-01T08:50:00.000Z', '2026-10-01T08:50:00.000Z',"
        " NULL, 'github', '{}', NULL)",
        "INSERT INTO settings VALUES (1, 'ws1', '/home/alice/.copilot/repos', '', NULL, NULL,"
        " NULL, NULL)",
        "INSERT INTO model_providers VALUES ('mp1', 'GitHub Copilot', '2026-10-01T08:50:01.000Z',"
        " '2026-10-01T08:50:01.000Z', 'github_copilot', '{}', 'acc1')",
        "INSERT INTO projects VALUES ('p1', 'proj', 'repository', '/srv/proj', 'main',"
        " 'octo-user', 'proj', 'acc1', 0, '2026-10-01T08:55:00.000Z', '2026-10-01T09:00:00.000Z')",
        "INSERT INTO worktrees VALUES ('wt1', 'p1', '%s', 'octo-user-happy-otter', 'main',"
        " '2026-10-01T08:56:00.000Z')" % COPILOT_DESKTOP_WORKTREE,
        "INSERT INTO workspaces VALUES ('ws1', 'p1', 'wt1', 'worktree', 'octo-user-happy-otter',"
        " 'happy-otter', 0, 3000, '2026-10-01T08:56:01.000Z', '2026-10-01T08:56:01.000Z',"
        " 'octo-user/proj', 12, NULL, NULL)",
        "INSERT INTO workspace_session_aliases VALUES ('%s', 'ws1', '2026-10-01T09:00:00.000Z')"
        % session,
        # The project session first by rowid, to show the rows follow created_at.
        "INSERT INTO sessions (id, title, session_type, model, created_at, updated_at,"
        " remote_control_enabled, auto_approve, forked_from_session_id, archived_at,"
        " permission_mode) VALUES ('%s', 'Fix failing test', 'project', 'gpt-6.1-sol',"
        " '2026-10-01T09:00:00.000Z', '2026-10-01T09:00:30.000Z', 0, 1, '%s', NULL, 'on')"
        % (session, fork),
        "INSERT INTO sessions (id, title, session_type, model, created_at, updated_at,"
        " remote_control_enabled, auto_approve, forked_from_session_id, archived_at,"
        " permission_mode) VALUES ('%s', 'Say hello', 'general_chat', 'claude-sonnet-5.5',"
        " '2026-10-01T08:51:00.000Z', '2026-10-01T08:51:05.000Z', 0, 1, NULL,"
        " '2026-10-01T10:00:00.000Z', NULL)" % chat,
        "INSERT INTO activity_items VALUES ('a1', 'ws1', '%s', 'agent_idle',"
        " 'The fixture year was stale', 0, '2026-10-01T09:00:15.500Z',"
        " '2026-10-01T09:00:15.500Z', '{}')" % session,
        "INSERT INTO activity_items VALUES ('a2', NULL, '%s', 'agent_idle',"
        " 'Hello! How can I help?', 1, '2026-10-01T08:51:05.000Z', '2026-10-01T08:51:05.000Z',"
        " '{}')" % chat,
        "INSERT INTO activity_items VALUES ('a3', 'ws1', NULL, 'workspace_ready',"
        " 'Workspace happy-otter is ready', 1, '2026-10-01T08:56:02.000Z',"
        " '2026-10-01T08:56:02.000Z', '{}')",
        "INSERT INTO session_checkpoints VALUES ('%s', 'e0000000-0000-4000-8000-000000000004',"
        " '0123456789abcdef0123456789abcdef01234567', '2026-10-01T09:00:01.200Z')" % session,
        "INSERT INTO app_state VALUES ('permission-mode-default-on', 'true',"
        " '2026-10-01T08:50:00.000Z')",
        "INSERT INTO my_copilot_scheduled_tasks VALUES ('t1', 1, 'Nightly lint',"
        " 'run the linter', 1, 'background', 'general-purpose', '2026-10-01T09:10:00.000Z',"
        " '2026-10-01T09:10:00.000Z', NULL)",
        "INSERT INTO my_copilot_scheduled_task_triggers VALUES ('tr1', 't1', 'cron', 1, 1,"
        " '{\"cron\":\"0 2 * * *\"}', 1, '{}', '2026-10-02T02:00:00.000Z', NULL,"
        " '2026-10-01T09:10:01.000Z', '2026-10-01T09:10:01.000Z', NULL)",
        "INSERT INTO my_copilot_scheduled_task_invocations VALUES ('inv1', 't1', 'tr1',"
        " 't1:2026-10-02T02:00', 'schedule', '2026-10-02T02:00:00.000Z', 1, 'Nightly lint',"
        " 'run the linter', NULL, 'background', '%s', NULL, 'completed',"
        " '2026-10-02T02:00:01.000Z', '2026-10-02T02:00:02.000Z')" % chat,
        "INSERT INTO session_automations VALUES ('%s', 'check CI', 'daily', NULL, 9, 1, NULL,"
        " '2026-10-02T09:00:00.000Z', '2026-10-01T09:20:00.000Z', 'CI check', NULL, NULL)"
        % session,
        "INSERT INTO session_deletion_intents VALUES ('%s', '2026-10-01T10:05:00.000Z')" % chat,
    ]


def build_copilot_desktop(home: Path) -> None:
    _wal_database(
        home / COPILOT_DESKTOP_DATABASE_REL, COPILOT_DESKTOP_SCHEMA, copilot_desktop_records()
    )
    database = home / COPILOT_DESKTOP_DATABASE_REL
    os.utime(database, (COPILOT_DESKTOP_MTIME, COPILOT_DESKTOP_MTIME))
    # Noise the parser must not want: the WebSocket token and the web view's
    # local storage (research/copilot-desktop.md, Credentials and Other stores).
    run = home / ".copilot/run"
    run.mkdir(parents=True, exist_ok=True)
    (run / "ws.token").write_text("A" * 49, encoding="utf-8")
    store = home / ".local/share/com.github.githubapp/localstorage/tauri_localhost_0.localstorage"
    store.parent.mkdir(parents=True, exist_ok=True)
    store.write_bytes(b"SQLite format 3\x00not a real store")
    # The checkpoint refs the parser reads, and a branch ref it must not want.
    for rel, content, mtime in (
        (
            COPILOT_DESKTOP_REF_REL,
            "0123456789abcdef0123456789abcdef01234567\n",
            COPILOT_DESKTOP_REF_MTIME,
        ),
        (COPILOT_DESKTOP_BAD_REF_REL, "not-a-hash\n", COPILOT_DESKTOP_BAD_REF_MTIME),
        (COPILOT_DESKTOP_HEAD_REF_REL, "fedcba9876543210fedcba9876543210fedcba98\n", None),
    ):
        ref = home / rel
        ref.parent.mkdir(parents=True, exist_ok=True)
        ref.write_text(content, encoding="utf-8")
        if mtime:
            os.utime(ref, (mtime, mtime))
