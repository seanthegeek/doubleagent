"""Tests of the ChatGPT desktop app parser."""

import sqlite3
from pathlib import Path
from typing import ClassVar

from doubleagent import cli
from doubleagent.model import summarise
from doubleagent.parsers import Options, by_agent
from doubleagent.parsers.chatgpt_desktop import ChatGptDesktopParser
from doubleagent.parsers.codex import CodexParser
from fixtures import (
    CHATGPT_CONVERSATION,
    CHATGPT_DESKTOP_DATABASE_REL,
    CHATGPT_HOST,
    CHATGPT_TPP_CONVERSATION,
    CODEX_SESSION,
)

from .base import ParserBase, rel_only


class ChatGptDesktopTests(ParserBase):
    """Test the conversation list in `.codex/sqlite/codex-dev.db` `local_thread_catalog`.

    The shapes follow the Parser plan of research/chatgpt-desktop.md.
    """

    SOURCE = "(from .codex/sqlite/codex-dev.db local_thread_catalog)"
    # (timestamp_utc, session_id, text) for every row of the fixture; the
    # `vscode` row (a Codex thread) gives none.
    EXPECTED: ClassVar[list[tuple[str, str, str]]] = [
        (
            "2026-10-01T11:00:00.500Z",
            CHATGPT_CONVERSATION,
            "account conversation: Plan a trip origin=- created=2026-10-01T10:00:00.250Z"
            " updated=2026-10-01T11:00:00.500Z host=%s %s" % (CHATGPT_HOST, SOURCE),
        ),
        (
            "2026-10-01T11:00:00.500Z",
            CHATGPT_CONVERSATION,
            "session origin: remote chatgpt.com conversation %s"
            " (listed at sign-in; its transcript is server-side)" % CHATGPT_CONVERSATION,
        ),
        (
            "2026-10-01T11:00:00.500Z",
            CHATGPT_CONVERSATION,
            "session synced: chatgpt.com holds conversation %s, no local transcript"
            % CHATGPT_CONVERSATION,
        ),
        (
            "2026-10-01T09:30:00.000Z",
            CHATGPT_TPP_CONVERSATION,
            "account conversation: - origin=tpp created=2026-10-01T09:00:00.000Z"
            " updated=2026-10-01T09:30:00.000Z host=%s %s" % (CHATGPT_HOST, SOURCE),
        ),
        (
            "2026-10-01T09:30:00.000Z",
            CHATGPT_TPP_CONVERSATION,
            "session origin: remote chatgpt.com conversation %s"
            " (listed at sign-in; its transcript is server-side)" % CHATGPT_TPP_CONVERSATION,
        ),
        (
            "2026-10-01T09:30:00.000Z",
            CHATGPT_TPP_CONVERSATION,
            "session synced: chatgpt.com holds conversation %s, no local transcript"
            % CHATGPT_TPP_CONVERSATION,
        ),
    ]

    def test_rows(self):
        # The fixture's rows live only in the -wal sidecar.
        rows = self.rows_for(ChatGptDesktopParser(), CHATGPT_DESKTOP_DATABASE_REL)
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.text) for row in rows], self.EXPECTED
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.turn_type, row.source_line),
                ("h1", "alice", "chatgpt-desktop", "system", 0),
            )
            self.assertEqual(
                (row.project_path, row.model, row.git_branch, row.tool_use_id, row.tool_name),
                ("", "", "", "", ""),
            )
            self.assertEqual(row.source_file, "/alice/" + CHATGPT_DESKTOP_DATABASE_REL)

    def test_vscode_rows_skipped(self):
        # The `vscode` row is the app's Codex thread, which its rollout gives.
        rows = self.rows_for(ChatGptDesktopParser(), CHATGPT_DESKTOP_DATABASE_REL)
        self.assertNotIn(CODEX_SESSION, {row.session_id for row in rows})
        self.assertFalse(any("Fix the tests" in row.text for row in rows))

    def test_thinking_opt_in(self):
        # The table holds no reasoning: the option changes nothing.
        rows = self.rows_for(
            ChatGptDesktopParser(), CHATGPT_DESKTOP_DATABASE_REL, include_thinking=True
        )
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.text) for row in rows], self.EXPECTED
        )

    def test_sessions(self):
        # Each conversation is a session of remote origin, synced, not
        # relayed, with no turns: its text is on the server only.
        rows, _, problems, dropped = cli.collect_rows(self.col, Options(), ["chatgpt-desktop"])
        self.assertEqual(problems, [])
        self.assertEqual(dropped["chatgpt-desktop"], 0)
        sessions = summarise(
            rows, cli.SYNC_AGENTS, cli.RELAY_AGENTS, cli.ORIGIN_AGENTS, cli.SECONDARY_AGENTS
        )
        got = {
            session.session_id: (
                session.synced,
                session.relayed,
                session.origin,
                session.recovered,
                session.user_turns,
                session.assistant_turns,
                session.tool_calls,
                session.first_timestamp_utc,
            )
            for session in sessions
            if session.agent == "chatgpt-desktop"
        }
        self.assertEqual(
            got,
            {
                CHATGPT_CONVERSATION: (
                    "yes",
                    "",
                    "remote",
                    "",
                    0,
                    0,
                    0,
                    "2026-10-01T11:00:00.500Z",
                ),
                CHATGPT_TPP_CONVERSATION: (
                    "yes",
                    "",
                    "remote",
                    "",
                    0,
                    0,
                    0,
                    "2026-10-01T09:30:00.000Z",
                ),
            },
        )

    def test_routing(self):
        # The catalog gives the database to codex-cli; `reads_agents` offers
        # it to this parser too, and the codex-cli parser does not want it.
        art = self.artifact(CHATGPT_DESKTOP_DATABASE_REL)
        self.assertEqual(art.agent, "codex-cli")
        offered = cli.parsers_for(art, by_agent())
        wanting = [type(parser).__name__ for parser in offered if parser.wants(art)]
        self.assertEqual(wanting, ["ChatGptDesktopParser"])
        self.assertIn("CodexParser", [type(parser).__name__ for parser in offered])
        self.assertFalse(CodexParser().wants(art))
        self.assertFalse(ChatGptDesktopParser().secondary(art))

    def test_not_wanted(self):
        parser = ChatGptDesktopParser()
        for rel in (
            CHATGPT_DESKTOP_DATABASE_REL + "-wal",
            CHATGPT_DESKTOP_DATABASE_REL + "-shm",
            ".codex/config.toml",
            ".codex/state_5.sqlite",
            "Library/Preferences/com.openai.codex.plist",
        ):
            self.assertFalse(parser.wants(self.artifact(rel)), rel)
        self.assertEqual(
            self.artifact("Library/Preferences/com.openai.codex.plist").agent, "chatgpt-desktop"
        )
        for rel in ("proj/.codex/sqlite/codex-dev.db", ".openinterpreter/sqlite/codex-dev.db"):
            self.assertFalse(parser.wants(rel_only(rel)), rel)
        self.assertTrue(parser.wants(rel_only(CHATGPT_DESKTOP_DATABASE_REL)))

    def test_unreadable_database(self):
        database = self.home / CHATGPT_DESKTOP_DATABASE_REL
        for side in ("-wal", "-shm"):
            Path(str(database) + side).unlink(missing_ok=True)
        database.write_text("not sqlite", encoding="utf-8")
        rows = self.rows_for(ChatGptDesktopParser(), CHATGPT_DESKTOP_DATABASE_REL)
        self.assertEqual(
            [(row.turn_type, row.text, row.agent, row.session_id) for row in rows],
            [("system", "parser: not a SQLite database", "chatgpt-desktop", "")],
        )
        # A SQLite header over a corrupt body is one SQLite error row.
        database.write_bytes(b"SQLite format 3\x00" + b"\xff" * 200)
        rows = self.rows_for(ChatGptDesktopParser(), CHATGPT_DESKTOP_DATABASE_REL)
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("parser: SQLite error: "), rows[0].text)

    def test_missing_table_and_nulls(self):
        database = self.home / CHATGPT_DESKTOP_DATABASE_REL
        for side in ("-wal", "-shm"):
            Path(str(database) + side).unlink(missing_ok=True)
        database.unlink()
        connection = sqlite3.connect(str(database))
        connection.execute("CREATE TABLE automations (id TEXT)")
        connection.commit()
        connection.close()
        rows = self.rows_for(ChatGptDesktopParser(), CHATGPT_DESKTOP_DATABASE_REL)
        self.assertEqual([row.text for row in rows], ["parser: no local_thread_catalog table"])
        # Every column may be NULL but source_kind; a NULL time prints `-`
        # and the row falls back to source_created_at, then to no time.
        connection = sqlite3.connect(str(database))
        connection.executescript(
            "CREATE TABLE local_thread_catalog (host_id TEXT, thread_id TEXT,"
            " display_title TEXT, source_created_at REAL, source_updated_at REAL,"
            " source_kind TEXT, conversation_origin TEXT);"
            "INSERT INTO local_thread_catalog VALUES"
            " ('h', 't1', 'Only created', 1790848800.0, NULL, 'chatgpt', NULL);"
            "INSERT INTO local_thread_catalog VALUES"
            " (NULL, NULL, NULL, NULL, NULL, 'chatgpt', NULL);"
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(ChatGptDesktopParser(), CHATGPT_DESKTOP_DATABASE_REL)
        self.assertEqual(
            [
                (row.timestamp_utc, row.session_id, row.text)
                for row in rows
                if row.text.startswith("acc")
            ],
            [
                (
                    "2026-10-01T10:00:00.000Z",
                    "t1",
                    "account conversation: Only created origin=-"
                    " created=2026-10-01T10:00:00.000Z updated=- host=h " + self.SOURCE,
                ),
                (
                    "",
                    "",
                    "account conversation: - origin=- created=- updated=- host=- " + self.SOURCE,
                ),
            ],
        )
        self.assertEqual(len(rows), 6)
        self.assertEqual(
            rows[-1].text, "session synced: chatgpt.com holds conversation -, no local transcript"
        )
