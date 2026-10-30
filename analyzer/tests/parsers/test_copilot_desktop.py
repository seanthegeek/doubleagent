"""Tests of the GitHub Copilot Desktop parser."""

import sqlite3
from pathlib import Path
from typing import ClassVar

from doubleagent import cli
from doubleagent.inputs import open_input
from doubleagent.parsers import Options
from doubleagent.parsers.copilot_cli import CopilotCliParser
from doubleagent.parsers.copilot_desktop import CopilotDesktopParser
from fixtures import (
    COPILOT_DESKTOP_BAD_REF_REL,
    COPILOT_DESKTOP_CHAT,
    COPILOT_DESKTOP_DATABASE_REL,
    COPILOT_DESKTOP_HEAD_REF_REL,
    COPILOT_DESKTOP_REF_REL,
    COPILOT_DESKTOP_WORKTREE,
    COPILOT_SESSION,
)

from .base import ParserBase, rel_only


class CopilotDesktopTests(ParserBase):
    """Test the Copilot Desktop parser."""

    # (timestamp_utc, session_id, project_path, text) for every row of the
    # fixture data.db (research/copilot-desktop.md, Parser plan).
    EXPECTED: ClassVar[list[tuple[str, str, str, str]]] = [
        (
            "2026-10-01T08:50:00.000Z",
            "",
            "",
            "account: octo-user host=github.com kind=github default=1 token_present=false",
        ),
        (
            "2026-10-01T10:30:00.000Z",
            "",
            "",
            "desktop settings: storage_location=/home/alice/.copilot/repos instructions=-"
            " github_token_present=false github_token_type=- (from .copilot/data.db, file mtime)",
        ),
        ("2026-10-01T08:50:01.000Z", "", "", "model provider: GitHub Copilot type=github_copilot"),
        (
            "2026-10-01T08:55:00.000Z",
            "",
            "/srv/proj",
            "project added: proj kind=repository path=/srv/proj github=octo-user/proj"
            " default_branch=main",
        ),
        (
            "2026-10-01T08:56:00.000Z",
            "",
            "/srv/proj",
            "worktree: %s branch=octo-user-happy-otter base=main" % COPILOT_DESKTOP_WORKTREE,
        ),
        (
            "2026-10-01T08:56:01.000Z",
            "",
            "/srv/proj",
            "workspace: happy-otter type=worktree branch=octo-user-happy-otter"
            " source_pr=octo-user/proj#12",
        ),
        (
            "2026-10-01T08:51:00.000Z",
            COPILOT_DESKTOP_CHAT,
            "",
            "desktop session: Say hello type=general_chat model=claude-sonnet-5.5 auto_approve=1"
            " permission_mode=- remote_control=0 archived=true",
        ),
        (
            "2026-10-01T09:00:00.000Z",
            COPILOT_SESSION,
            "/srv/proj",
            "desktop session: Fix failing test type=project model=gpt-6.1-sol auto_approve=1"
            " permission_mode=on remote_control=0 archived=false",
        ),
        (
            "2026-10-01T09:00:00.000Z",
            COPILOT_SESSION,
            "/srv/proj",
            "forked from session 9f9f9f9f-0000-4000-8000-000000000001",
        ),
        (
            "2026-10-01T08:51:05.000Z",
            COPILOT_DESKTOP_CHAT,
            "",
            "activity agent_idle: Hello! How can I help?",
        ),
        (
            "2026-10-01T08:56:02.000Z",
            "",
            "",
            "activity workspace_ready: Workspace happy-otter is ready",
        ),
        (
            "2026-10-01T09:00:15.500Z",
            COPILOT_SESSION,
            "",
            "activity agent_idle: The fixture year was stale",
        ),
        (
            "2026-10-01T09:00:01.200Z",
            COPILOT_SESSION,
            "",
            "checkpoint commit 0123456789abcdef0123456789abcdef01234567",
        ),
        (
            "2026-10-01T09:10:00.000Z",
            "",
            "",
            "scheduled task: Nightly lint enabled=1 mode=background agent=general-purpose"
            " prompt: run the linter",
        ),
        (
            "2026-10-01T09:10:01.000Z",
            "",
            "",
            "scheduled task trigger: task=t1 kind=cron enabled=1"
            ' next_run_at=2026-10-02T02:00:00.000Z config: {"cron":"0 2 * * *"}',
        ),
        (
            "2026-10-02T02:00:01.000Z",
            COPILOT_DESKTOP_CHAT,
            "",
            "scheduled task run: Nightly lint status=completed source=schedule"
            " scheduled_for=2026-10-02T02:00:00.000Z prompt: run the linter",
        ),
        (
            "2026-10-01T09:20:00.000Z",
            COPILOT_SESSION,
            "",
            "session automation: CI check interval=daily every_minutes=-"
            " next_run_at=2026-10-02T09:00:00.000Z prompt: check CI",
        ),
        ("2026-10-01T10:05:00.000Z", COPILOT_DESKTOP_CHAT, "", "session deletion requested"),
    ]

    def test_rows(self):
        # The fixture's rows live only in the -wal sidecar.
        rows = self.rows_for(CopilotDesktopParser(), COPILOT_DESKTOP_DATABASE_REL)
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.project_path, row.text) for row in rows],
            self.EXPECTED,
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.turn_type, row.source_line, row.tool_use_id),
                ("h1", "alice", "copilot-desktop", "system", 0, ""),
            )
            self.assertEqual(row.source_file, "/alice/" + COPILOT_DESKTOP_DATABASE_REL)
        self.assertEqual(
            [row.model for row in rows if row.text.startswith(("desktop session", "forked"))],
            ["claude-sonnet-5.5", "gpt-6.1-sol", "gpt-6.1-sol"],
        )
        self.assertEqual({row.model for row in rows[:6] + rows[9:]}, {""})

    def test_thinking_opt_in(self):
        # The store holds no reasoning: the option changes nothing.
        rows = self.rows_for(
            CopilotDesktopParser(), COPILOT_DESKTOP_DATABASE_REL, include_thinking=True
        )
        self.assertEqual(len(rows), len(self.EXPECTED))
        self.assertFalse(any(row.turn_type == "thinking" for row in rows))

    def test_nulls_and_missing_tables(self):
        # Every column may be NULL and a missing table gives no rows.
        database = self.home / COPILOT_DESKTOP_DATABASE_REL
        for side in ("-wal", "-shm"):
            Path(str(database) + side).unlink(missing_ok=True)
        database.unlink()
        connection = sqlite3.connect(str(database))
        connection.executescript(
            "CREATE TABLE accounts (login TEXT, host TEXT, kind TEXT, is_default INTEGER,"
            " access_token TEXT, created_at TEXT);"
            "INSERT INTO accounts VALUES (NULL, NULL, NULL, NULL, 'gho_x', NULL);"
            "CREATE TABLE projects (id TEXT, name TEXT, container_kind TEXT,"
            " main_repo_path TEXT, default_branch TEXT, github_owner TEXT, github_repo TEXT,"
            " created_at TEXT);"
            "INSERT INTO projects VALUES ('p', 'local', 'folder', '/srv/local', 'main', NULL,"
            " NULL, '2026-10-01T08:00:00.000Z');"
            "CREATE TABLE sessions (id TEXT, created_at TEXT);"
            "INSERT INTO sessions VALUES ('s1', NULL);"
        )
        connection.commit()
        connection.close()
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(CopilotDesktopParser(), COPILOT_DESKTOP_DATABASE_REL)
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.text) for row in rows],
            [
                ("", "", "account:  host= kind= default= token_present=true"),
                (
                    "2026-10-01T08:00:00.000Z",
                    "",
                    "project added: local kind=folder path=/srv/local default_branch=main",
                ),
                (
                    "",
                    "s1",
                    "desktop session: - type= model=- auto_approve= permission_mode=-"
                    " remote_control= archived=false",
                ),
            ],
        )

    def test_secondary_none(self):
        art = self.artifact(COPILOT_DESKTOP_DATABASE_REL)
        self.assertFalse(CopilotDesktopParser().secondary(art))
        rows, _, problems, dropped = cli.collect_rows(self.col, Options(), ["copilot-desktop"])
        self.assertEqual(
            len([row for row in rows if row.agent == "copilot-desktop"]), len(self.EXPECTED) + 2
        )
        for rel in (COPILOT_DESKTOP_REF_REL, COPILOT_DESKTOP_BAD_REF_REL):
            self.assertFalse(CopilotDesktopParser().secondary(self.artifact(rel)), rel)
        self.assertEqual(dropped["copilot-desktop"], 0)
        self.assertEqual(problems, [])

    def test_not_wanted(self):
        parser = CopilotDesktopParser()
        for rel in (
            ".copilot/data.db-wal",
            ".copilot/data.db-shm",
            ".copilot/run/ws.token",
            ".local/share/com.github.githubapp/localstorage/tauri_localhost_0.localstorage",
        ):
            art = self.artifact(rel)
            self.assertEqual(art.agent, "copilot-desktop", rel)
            self.assertFalse(parser.wants(art), rel)
        for rel in (
            ".copilot/repo-metadata-cache.db",
            ".copilot/logs/github-app.4242.log",
            "proj/.copilot/data.db",
            ".copilot/session-store.db",
            ".copilot/session-state/%s/session.db" % COPILOT_SESSION,
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)
        self.assertTrue(parser.wants(rel_only(COPILOT_DESKTOP_DATABASE_REL)))
        # The CLI parser does not read the app's store either.
        self.assertFalse(CopilotCliParser().wants(self.artifact(COPILOT_DESKTOP_DATABASE_REL)))

    def test_checkpoint_refs(self):
        # One row per loose checkpoint ref, at the file's mtime.
        parser = CopilotDesktopParser()
        rows = self.rows_for(parser, COPILOT_DESKTOP_REF_REL) + self.rows_for(
            parser, COPILOT_DESKTOP_BAD_REF_REL
        )
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.project_path, row.text) for row in rows],
            [
                (
                    "2026-10-01T09:00:01.000Z",
                    COPILOT_SESSION,
                    "",
                    "checkpoint ref 1: commit 0123456789abcdef0123456789abcdef01234567"
                    " event e0000000-0000-4000-8000-000000000004 (file mtime)",
                ),
                # A malformed ref keeps its content as read, nothing repaired.
                (
                    "2026-10-01T09:00:30.000Z",
                    COPILOT_SESSION,
                    "",
                    "checkpoint ref 2: commit not-a-hash"
                    " event e0000000-0000-4000-8000-000000000020 (file mtime)",
                ),
            ],
        )
        for row, rel in zip(
            rows, (COPILOT_DESKTOP_REF_REL, COPILOT_DESKTOP_BAD_REF_REL), strict=True
        ):
            self.assertEqual(
                (
                    row.host,
                    row.user,
                    row.agent,
                    row.turn_type,
                    row.source_line,
                    row.model,
                    row.tool_use_id,
                ),
                ("h1", "alice", "copilot-desktop", "system", 0, "", ""),
            )
            self.assertEqual(row.source_file, "/alice/" + rel)
        # An empty ref file still gives its row.
        (self.home / COPILOT_DESKTOP_BAD_REF_REL).write_text("", encoding="utf-8")
        rows = self.rows_for(parser, COPILOT_DESKTOP_BAD_REF_REL)
        self.assertEqual(
            [row.text for row in rows],
            ["checkpoint ref 2: commit  event e0000000-0000-4000-8000-000000000020 (file mtime)"],
        )

    def test_checkpoint_refs_not_wanted(self):
        parser = CopilotDesktopParser()
        art = self.artifact(COPILOT_DESKTOP_HEAD_REF_REL)
        self.assertEqual(art.agent, "copilot-desktop")
        self.assertFalse(parser.wants(art))
        ref = COPILOT_DESKTOP_REF_REL
        for rel in (
            ref.replace("/refs/copilot/checkpoints/", "/refs/heads/"),
            ref.replace(".copilot/repos/proj/", ".copilot/repos/"),
            ref.replace("/00000000000000000001/", "/1a/"),
            ref + ".lock",
            ref.rsplit("/", 1)[0],
            "proj/" + ref,
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)
        self.assertTrue(parser.wants(rel_only(ref)))
        self.assertTrue(parser.wants(self.artifact(ref)))

    def test_unreadable_database(self):
        database = self.home / COPILOT_DESKTOP_DATABASE_REL
        for side in ("-wal", "-shm"):
            Path(str(database) + side).unlink(missing_ok=True)
        database.write_text("not sqlite", encoding="utf-8")
        rows = self.rows_for(CopilotDesktopParser(), COPILOT_DESKTOP_DATABASE_REL)
        self.assertEqual(
            [(row.turn_type, row.text, row.agent) for row in rows],
            [("system", "parser: not a SQLite database", "copilot-desktop")],
        )
        # A SQLite header over a corrupt body is one SQLite error row.
        database.write_bytes(b"SQLite format 3\x00" + b"\xff" * 200)
        rows = self.rows_for(CopilotDesktopParser(), COPILOT_DESKTOP_DATABASE_REL)
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("parser: SQLite error: "), rows[0].text)
