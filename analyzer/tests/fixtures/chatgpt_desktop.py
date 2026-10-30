"""Synthetic ChatGPT desktop app state for the analyzer tests."""

from __future__ import annotations

from pathlib import Path

from .codex_cli import CODEX_SESSION
from .common import _wal_database

# The ChatGPT desktop app's `.codex/sqlite/codex-dev.db`: the
# `local_thread_catalog` column names seen on a macOS install
# (`pragma table_info`, research/chatgpt-desktop.md, Record schema; the column
# types other than the REAL times were not recorded) and its
# hosts table. The rows live only in the -wal sidecar, as on a running app.
CHATGPT_DESKTOP_DATABASE_REL = ".codex/sqlite/codex-dev.db"
CHATGPT_CONVERSATION = "6a0f0e2c-0000-4000-8000-000000000001"
CHATGPT_TPP_CONVERSATION = "6a0f0e2c-0000-4000-8000-000000000002"
CHATGPT_HOST = "chatgpt:00000000-0000-4000-8000-0000000000aa:user-FAKEACCOUNT0000"
CHATGPT_DESKTOP_SCHEMA = """
CREATE TABLE local_thread_catalog (host_id TEXT NOT NULL, thread_id TEXT NOT NULL,
  display_title TEXT, source_created_at REAL, source_updated_at REAL, cwd TEXT,
  source_kind TEXT, source_detail TEXT, model_provider TEXT, git_branch TEXT,
  observation_sequence INTEGER, missing_candidate INTEGER, thread_source TEXT,
  source_recency_at REAL, pending_observed_title TEXT, project_id TEXT,
  conversation_origin TEXT, trial_conversation_type TEXT, chatgpt_async_status TEXT,
  PRIMARY KEY (host_id, thread_id));
CREATE TABLE local_thread_catalog_hosts (host_id TEXT PRIMARY KEY, host_kind TEXT);
"""


def chatgpt_desktop_records() -> list[str]:
    """The INSERTs of the fixture `codex-dev.db`.

    The app's Codex thread (a `vscode` row, which the parser skips), then two account conversations,
    one with a title and an empty origin, one with no title and origin `tpp`. Times are REAL epoch
    seconds with a fraction.
    """
    return [
        "INSERT INTO local_thread_catalog_hosts VALUES ('local', 'local')",
        "INSERT INTO local_thread_catalog_hosts VALUES ('%s', 'chatgpt')" % CHATGPT_HOST,
        "INSERT INTO local_thread_catalog VALUES ('local', '%s', 'Fix the tests',"
        " 1791367200.0, 1791367260.0, '/srv/proj', 'vscode', NULL, 'openai', NULL, 5, 0,"
        " 'user', 1791367260.0, NULL, NULL, NULL, NULL, NULL)" % CODEX_SESSION,
        # 1 October 10:00:00.250 UTC and 1 October 11:00:00.500 UTC.
        "INSERT INTO local_thread_catalog VALUES ('%s', '%s', 'Plan a trip',"
        " 1790848800.25, 1790852400.5, NULL, 'chatgpt', NULL, NULL, NULL, 1, 0, NULL,"
        " 1790852400.5, NULL, NULL, '', NULL, NULL)" % (CHATGPT_HOST, CHATGPT_CONVERSATION),
        # 1 October 09:00:00.000 UTC and 1 October 09:30:00.000 UTC.
        "INSERT INTO local_thread_catalog VALUES ('%s', '%s', NULL,"
        " 1790845200.0, 1790847000.0, NULL, 'chatgpt', NULL, NULL, NULL, 1, 0, NULL,"
        " 1790847000.0, NULL, NULL, 'tpp', NULL, NULL)" % (CHATGPT_HOST, CHATGPT_TPP_CONVERSATION),
    ]


def build_chatgpt_desktop(home: Path) -> None:
    _wal_database(
        home / CHATGPT_DESKTOP_DATABASE_REL, CHATGPT_DESKTOP_SCHEMA, chatgpt_desktop_records()
    )
    # Noise the parser must not want: the app's preferences, a file the
    # catalog attributes to `chatgpt-desktop` itself, so the agent is detected.
    prefs = home / "Library/Preferences/com.openai.codex.plist"
    prefs.parent.mkdir(parents=True, exist_ok=True)
    prefs.write_bytes(b"bplist00not a real plist")
