"""Tests of the Tabby parser."""

import io
import sys
import unittest

from test_errors import CAN_DENY, DENY_REASON

from doubleagent import cli
from doubleagent.inputs import open_input
from doubleagent.parsers import Options
from doubleagent.parsers.tabby import TabbyParser
from doubleagent.timeutil import to_utc
from fixtures import (
    TABBY_BACKUP2_REL,
    TABBY_BACKUP_REL,
    TABBY_CLIENT_TOKEN,
    TABBY_CONFIG_REL,
    TABBY_ENDPOINT_TOKEN,
    TABBY_EVENTS_REL,
    TABBY_INTELLIJ_REL,
    TABBY_REL,
    TABBY_SECRETS,
    TABBY_VSCDB_REL,
    bad_line_text,
    write_bad_line,
)

from .base import ParserBase, rel_only

TABBY_SOURCE_HOST = (
    "source host: github.com integration acme kind=github api_base=- (account not recorded; "
    "from integrations table, created_at)"
)
TABBY_SOURCE_REPO = (
    "source host repository: https://github.com/acme/app type=github name=acme/app active=yes "
    "(integration acme; from provided_repositories table, updated_at)"
)


class TabbyTests(ParserBase):
    """Test the Tabby parser."""

    def test_secondary(self):
        # The backups and the event log are secondary sources, the live
        # database is not.
        parser = TabbyParser()
        for rel in (
            TABBY_BACKUP_REL,
            TABBY_BACKUP2_REL,
            TABBY_EVENTS_REL,
            TABBY_INTELLIJ_REL,
            TABBY_VSCDB_REL,
        ):
            self.assertTrue(parser.secondary(self.artifact(rel)), rel)
        self.assertFalse(parser.secondary(self.artifact(TABBY_REL)))
        self.assertFalse(parser.secondary(self.artifact(TABBY_CONFIG_REL)))

    def test_rows(self):
        rows = self.rows_for(TabbyParser(), TABBY_REL)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                *("system", "user", "assistant"),  # thread 7
                *("system", "assistant", "assistant", "assistant"),  # page 4
                *("system", "user", "assistant", "system"),  # user_events
                *("system", "system"),  # integration, cached repository
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.git_branch, row.model),
                ("h1", "alice", "tabby", "", ""),
            )
            self.assertTrue(row.source_file.endswith(TABBY_REL), row.source_file)
        start, user, asst = rows[:3]
        select = rows[-3]
        # The user_events copy of the event log's completion line.
        app = "https://github.com/acme/app"
        self.assertEqual(
            [(row.session_id, row.project_path, row.timestamp_utc, row.text) for row in rows[7:10]],
            [
                (
                    "cmpl-7f3a",
                    app,
                    "2025-10-01T10:00:00.000Z",
                    "completion | user=alice@example.com (Alice) | language=python"
                    " | file=app/math.py | client=tabby-agent/1.9",
                ),
                ("cmpl-7f3a", app, "2025-10-01T10:00:00.000Z", "def add(a, b):"),
                ("cmpl-7f3a", app, "2025-10-01T10:00:00.000Z", "return a + b"),
            ],
        )
        for row in (start, user, asst):
            self.assertEqual(
                (row.session_id, row.project_path), ("7", "https://github.com/acme/app")
            )
        self.assertEqual(
            (start.timestamp_utc, start.text),
            (
                "2026-10-01T10:00:00.000Z",
                "thread start | owner alice@example.com (Alice) | relevant questions: How is the cache invalidated?",
            ),
        )
        self.assertEqual(
            (user.text, user.source_line),
            ("Where is the cache cleared? [attachments: client_code src/cache.rs:10]", 1),
        )
        self.assertEqual(
            (asst.timestamp_utc, asst.text),
            (
                "2026-10-01T10:00:02.000Z",
                "In `clear()` in src/cache.rs. [attachments: code https://github.com/acme/app@abc123 src/cache.rs:10]",
            ),
        )
        self.assertEqual(
            (select.session_id, select.timestamp_utc, select.text),
            (
                "cmpl-7f3a",
                "2026-10-01T10:00:01.000Z",
                "select choice 0 | user=alice@example.com (Alice) | view=view-1 | elapsed=1377ms",
            ),
        )

    def test_thinking_opt_in(self):
        # Tabby stores no reasoning; the flag must not invent rows.
        self.assertEqual(
            len(self.rows_for(TabbyParser(), TABBY_REL, include_thinking=True)),
            len(self.rows_for(TabbyParser(), TABBY_REL)),
        )

    def test_backup_and_event_log(self):
        rows = self.rows_for(TabbyParser(), TABBY_BACKUP_REL)
        self.assertEqual(
            [row.turn_type for row in rows], ["system", "system", "user", "assistant", "system"]
        )
        self.assertEqual(rows.pop().text, TABBY_SOURCE_HOST)
        self.assertEqual(
            rows[0].text,
            "backup database db.backup-20260915.sqlite copied before a schema migration; it can "
            "hold threads and events since deleted from the live database",
        )
        self.assertTrue(all(row.source_file.endswith(TABBY_BACKUP_REL) for row in rows))
        self.assertEqual(
            {(row.session_id, row.project_path) for row in rows[1:]},
            {("3", "https://github.com/acme/ops")},
        )
        self.assertIn("ephemeral", rows[1].text)
        self.assertEqual(
            rows[3].text,
            "Run `select * from users`. [attachments: code https://github.com/acme/ops db.sql]",
        )
        rows = self.rows_for(TabbyParser(), TABBY_EVENTS_REL)
        self.assertEqual(
            [(row.turn_type, row.session_id) for row in rows],
            [
                ("system", "cmpl-7f3a"),
                ("user", "cmpl-7f3a"),
                ("assistant", "cmpl-7f3a"),
                ("system", "cmpl-7f3a"),
                ("system", ""),
            ],
        )
        self.assertEqual(
            rows[0].text,
            "completion | user=1 | language=python | file=app/math.py | client=tabby-agent/1.9",
        )
        self.assertEqual(
            (rows[1].text, rows[1].timestamp_utc, rows[1].project_path),
            ("def add(a, b):", "2025-10-01T10:00:00.123Z", "https://github.com/acme/app"),
        )
        self.assertEqual(rows[2].text, "return a + b")
        self.assertEqual(
            (rows[3].text, rows[3].timestamp_utc),
            ("select choice 0 | user=1 | view=view-1 | elapsed=1377ms", "2025-10-01T10:00:01.500Z"),
        )
        self.assertEqual(rows[4].text, "chat_completion")

    def test_config_and_sidecars_not_wanted(self):
        parser = TabbyParser()
        seen = set()
        for artifact in self.col.artifacts:
            if artifact.agent == "tabby":
                seen.add(artifact.rel)
                self.assertEqual(
                    parser.wants(artifact),
                    artifact.rel
                    in (
                        TABBY_REL,
                        TABBY_BACKUP_REL,
                        TABBY_BACKUP2_REL,
                        TABBY_EVENTS_REL,
                        TABBY_CONFIG_REL,
                        TABBY_INTELLIJ_REL,
                    ),
                    artifact.rel,
                )
        self.assertIn(".tabby-client/agent/config.toml", seen)
        self.assertIn(".config/JetBrains/IntelliJIdea2025.2/options/intellij-tabby.xml", seen)
        self.assertIn(".tabby/ee/db.sqlite-wal", seen)

    def test_truncated_line_is_reported_not_fatal(self):
        write_bad_line(self.home / TABBY_EVENTS_REL)
        with open(self.home / TABBY_EVENTS_REL, "a", encoding="utf-8") as handle:
            handle.write(
                "\n"
                + '{"user": null, "ts": 1759312803000, "event": {"chat_completion": {}}}'
                + "\n"
            )
        rows = self.rows_for(TabbyParser(), TABBY_EVENTS_REL)
        self.assertEqual(len(rows), 7)
        self.assertEqual([row.source_line for row in rows[:-1]], [1, 1, 1, 2, 3, 5])
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].source_line, rows[-1].text),
            ("system", 4, bad_line_text()),
        )

    def test_bad_payload_is_reported_not_fatal(self):
        import sqlite3

        connection = sqlite3.connect(str(self.home / TABBY_REL))
        connection.execute(
            "INSERT INTO user_events(user_id,kind,created_at,payload) VALUES(1,'view','2026-10-01 10:00:05','{\"view\":')"
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(TabbyParser(), TABBY_REL)
        self.assertEqual(len(rows), 14)
        self.assertIn("payload did not parse", rows[-3].text)
        self.assertEqual([row.text for row in rows[-2:]], [TABBY_SOURCE_HOST, TABBY_SOURCE_REPO])

    def test_corrupt_backup_is_reported_not_fatal(self):
        (self.home / TABBY_BACKUP2_REL).write_bytes(b"SQLite format 3\x00" + b"\x00" * 84)
        rows = self.rows_for(TabbyParser(), TABBY_BACKUP2_REL)
        self.assertEqual(rows[-1].turn_type, "system")
        self.assertTrue(rows[-1].text.startswith("parser:"), rows[-1].text)

    def test_second_backup_rows(self):
        rows = self.rows_for(TabbyParser(), TABBY_BACKUP2_REL)
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc) for row in rows],
            [
                ("system", "", ""),
                ("system", "3", "2026-09-10T08:00:00.000Z"),
                ("user", "3", "2026-09-10T08:00:00.000Z"),
                ("assistant", "3", "2026-09-10T08:00:04.000Z"),
                ("system", "7", "2026-10-01T10:00:00.000Z"),
                ("user", "7", "2026-10-01T10:00:00.000Z"),
                ("assistant", "7", "2026-10-01T10:00:02.000Z"),
                ("system", "", "2026-09-01T09:00:00.000Z"),
            ],
        )
        self.assertEqual(rows.pop().text, TABBY_SOURCE_HOST)
        self.assertIn("db.backup-20261005.sqlite", rows[0].text)
        # The same texts and projects as the first backup and the live database.
        first = self.rows_for(TabbyParser(), TABBY_BACKUP_REL)[:-1]
        live = self.rows_for(TabbyParser(), TABBY_REL)
        self.assertEqual(
            [(row.text, row.project_path) for row in rows[1:4]],
            [(row.text, row.project_path) for row in first[1:]],
        )
        self.assertEqual(
            [(row.text, row.project_path) for row in rows[4:]],
            [(row.text, row.project_path) for row in live[:3]],
        )

    def test_secondary_copies_dropped(self):
        # Every source is parsed and the copies are dropped: the event
        # log's prompt and choice (user_events has them, within a second), the
        # later backup's thread 7 (the live database has it) and one of the
        # two backup copies of ephemeral thread 3. The event log's system rows
        # name the owner by id, the database's by email, so both are kept.
        rows, _, _, dropped = cli.collect_rows(self.col, Options(), ["tabby"])
        self.assertEqual(dropped, {"tabby": 10})
        self.assertEqual(len([row for row in rows if row.text == TABBY_SOURCE_HOST]), 1)
        texts = [(row.source_file.rsplit("/", 1)[-1], row.turn_type, row.text) for row in rows]
        self.assertEqual(
            [text for text in texts if text[2] in ("def add(a, b):", "return a + b")],
            [("db.sqlite", "user", "def add(a, b):"), ("db.sqlite", "assistant", "return a + b")],
        )
        self.assertEqual(len([text for text in texts if text[2] == "dump the users table"]), 1)
        self.assertEqual(
            len([text for text in texts if text[2].startswith("Where is the cache")]), 1
        )
        self.assertIn(("2025-10-01.json", "system", "chat_completion"), texts)
        self.assertEqual(len([text for text in texts if text[2].startswith("backup database")]), 2)

    def test_secret_columns_never_reach_output(self):
        for rel in (TABBY_REL, TABBY_BACKUP_REL):
            for row in self.rows_for(TabbyParser(), rel):
                for value in row.as_dict().values():
                    for secret in TABBY_SECRETS:
                        self.assertNotIn(secret, str(value))
        out = self.tmp / "out"
        buffer = io.StringIO()
        import contextlib

        with contextlib.redirect_stdout(buffer):
            exit_code = cli.main(
                [
                    "timeline",
                    str(self.tmp / "home"),
                    "-o",
                    str(out),
                    "--host",
                    "h1",
                    "--agent",
                    "tabby",
                ]
            )
        self.assertEqual(exit_code, 0, buffer.getvalue())
        produced = [path for path in out.rglob("*") if path.is_file()]
        self.assertTrue(produced)
        for path in produced:
            data = path.read_bytes()
            for secret in TABBY_SECRETS:
                self.assertNotIn(secret.encode(), data, path)
        for secret in TABBY_SECRETS:
            self.assertNotIn(secret, buffer.getvalue())

    def test_no_new_secret_reaches_rows(self):
        # The model api_key, the client token and the VS Code endpoint token
        # sit beside what the parser reads.
        rows, _, _, _ = cli.collect_rows(self.col, Options(), ["tabby"])
        for row in rows:
            for value in row.as_dict().values():
                for secret in ("sk-tabby-test", TABBY_CLIENT_TOKEN, TABBY_ENDPOINT_TOKEN):
                    self.assertNotIn(secret, str(value))

    def test_pages(self):
        # AI-written pages outlive their thread (page.rs:92-150).
        rows = [
            row for row in self.rows_for(TabbyParser(), TABBY_REL) if row.session_id == "page:4"
        ]
        app = "https://github.com/acme/app"
        self.assertEqual(
            [
                (row.turn_type, row.source_line, row.timestamp_utc, row.project_path, row.text)
                for row in rows
            ],
            [
                (
                    "system",
                    4,
                    "2026-10-01T11:00:00.000Z",
                    app,
                    "page start | title Cache invalidation | author alice@example.com (Alice)"
                    " | code_source acme-app",
                ),
                ("assistant", 4, "2026-10-01T11:00:00.000Z", app, "How the cache is cleared."),
                (
                    "assistant",
                    9,
                    "2026-10-01T11:00:10.000Z",
                    app,
                    "Clearing\n\nCall `clear()`. [attachments: code https://github.com/acme/app"
                    " src/cache.rs:10]",
                ),
                (
                    "assistant",
                    10,
                    "2026-10-01T11:00:20.000Z",
                    app,
                    "Expiry\n\nEntries expire after an hour.",
                ),
            ],
        )

    def test_source_host_rows(self):
        # The integration and its cached repository list (schema.sql:142-165);
        # the access token is never selected.
        rows = self.rows_for(TabbyParser(), TABBY_REL)
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows[-2:]],
            [
                ("system", "", "2026-09-01T09:00:00.000Z", TABBY_SOURCE_HOST),
                ("system", "", "2026-09-02T09:05:00.000Z", TABBY_SOURCE_REPO),
            ],
        )

    def test_config_model_rows(self):
        # config.rs:259-342: one row per configured model, at the file mtime.
        rows = self.rows_for(TabbyParser(), TABBY_CONFIG_REL)
        mtime = to_utc((self.home / TABBY_CONFIG_REL).stat().st_mtime)
        self.assertTrue(all(row.timestamp_utc == mtime for row in rows))
        self.assertTrue(all((row.turn_type, row.session_id) == ("system", "") for row in rows))
        if sys.version_info < (3, 11):
            self.assertEqual(
                [row.text for row in rows],
                ["parser: config.toml model tables not read: Python 3.11 or later needed"],
            )
            return
        self.assertEqual(
            [row.text for row in rows],
            [
                "model configured: completion StarCoder-1B via local"
                " (from config.toml [model.completion.local], file mtime)",
                "model configured: chat gpt-4o via http kind=openai/chat"
                " endpoint=https://api.openai.com/v1 supported_models=gpt-4o,gpt-4o-mini"
                " (from config.toml [model.chat.http], file mtime)",
            ],
        )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_config_unreadable_is_reported_on_every_python(self):
        path = self.home / TABBY_CONFIG_REL
        path.chmod(0)
        try:
            rows = self.rows_for(TabbyParser(), TABBY_CONFIG_REL)
        finally:
            path.chmod(0o600)
        self.assertEqual(
            [(row.turn_type, row.timestamp_utc, row.text, row.source_line) for row in rows],
            [("system", "", "parser: config.toml could not be read (Permission denied)", 0)],
        )

    def test_bad_config_is_reported_not_fatal(self):
        (self.home / TABBY_CONFIG_REL).write_text(
            '[model.chat.http]\napi_key = "sk-tabby-test\n', encoding="utf-8"
        )
        rows = self.rows_for(TabbyParser(), TABBY_CONFIG_REL)
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("parser: config.toml"), rows[0].text)
        self.assertNotIn("sk-tabby-test", rows[0].text)

    def test_intellij_command_history(self):
        # CommandHistory.kt:5-31: newest first, no times.
        rows = self.rows_for(TabbyParser(), TABBY_INTELLIJ_REL)
        self.assertEqual(
            [
                (row.turn_type, row.source_line, row.timestamp_utc, row.session_id, row.text)
                for row in rows
            ],
            [
                (
                    "system",
                    0,
                    "",
                    "",
                    "inline edit command history: 2 commands, newest first"
                    " (from intellij-tabby-command-history.xml, no times)",
                ),
                ("user", 1, "", "", "add docstrings"),
                ("user", 2, "", "", "convert to async"),
            ],
        )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_intellij_history_is_one_row(self):
        path = self.home / TABBY_INTELLIJ_REL
        path.chmod(0)
        try:
            rows = self.rows_for(TabbyParser(), TABBY_INTELLIJ_REL)
            config = self.rows_for(TabbyParser(), TABBY_CONFIG_REL)
        finally:
            path.chmod(0o600)
        self.assertEqual(
            [
                (row.turn_type, row.timestamp_utc, row.text, row.source_line, row.session_id)
                for row in rows
            ],
            [("system", "", "parser: %s could not be read (Permission denied)" % path.name, 0, "")],
        )
        self.assertTrue(any(not row.text.startswith("parser:") for row in config))
        readable = self.rows_for(TabbyParser(), TABBY_INTELLIJ_REL)
        self.assertEqual([row.turn_type for row in readable], ["system", "user", "user"])

    def test_truncated_intellij_history_is_reported_not_fatal(self):
        path = self.home / TABBY_INTELLIJ_REL
        path.write_text(path.read_text(encoding="utf-8")[:120], encoding="utf-8")
        rows = self.rows_for(TabbyParser(), TABBY_INTELLIJ_REL)
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("parser: not a readable XML document"))

    def test_vscode_command_history(self):
        # Config.ts:139-145 and quickPick.ts:322-351, read from the editor's
        # state.vscdb through reads_agents.
        parser = TabbyParser()
        self.assertIn("vscode", parser.reads_agents)
        art = self.artifact(TABBY_VSCDB_REL)
        self.assertEqual(art.agent, "vscode")
        self.assertTrue(parser.wants(art))
        rows = list(parser.parse(art, Options()))
        self.assertEqual(
            [
                (row.agent, row.turn_type, row.source_line, row.timestamp_utc, row.text)
                for row in rows
            ],
            [
                (
                    "tabby",
                    "system",
                    0,
                    "",
                    "inline edit command history: 3 commands, newest first (from state.vscdb"
                    " TabbyML.vscode-tabby edit.recentlyCommand, no times)",
                ),
                (
                    "tabby",
                    "user",
                    1,
                    "",
                    "rename @file to snake case [context: file:///srv/proj/app/math.py]",
                ),
                ("tabby", "user", 2, "", "add type hints"),
                ("tabby", "user", 3, "", "fix the typo"),
            ],
        )
        # Another editor's state.vscdb without Tabby's row yields nothing.
        other = self.artifact(".config/Code - Insiders/User/globalStorage/state.vscdb")
        self.assertEqual(list(parser.parse(other, Options())), [])
        # A Tabby-attributed path is never routed as an editor database.
        self.assertFalse(parser.wants(self.artifact(".tabby-client/agent/config.toml")))

    def test_volume_root(self):
        """Read a Tabby server root collected from a Docker named volume.

        Its files sit at the base's root; the parser reads them as it reads a home's
        `.tabby`.
        """
        from fixtures import TABBY_VOLUME_HOME, build_tabby_volume

        parser = TabbyParser()
        for rel in (
            "ee/db.sqlite",
            "ee/dev-db.sqlite",
            "ee/db.backup-20260915.sqlite",
            "events/2025-10-01.json",
            "config.toml",
        ):
            self.assertTrue(parser.wants(rel_only(rel)), rel)
        for rel in (
            "ee/db.sqlite-wal",
            "usage_anonymous_id",
            "x/ee/db.sqlite",
            "events/today.json",
            "agent/config.toml",
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)
        self.assertTrue(parser.secondary(rel_only("events/2025-10-01.json")))
        self.assertFalse(parser.secondary(rel_only("ee/db.sqlite")))

        root = self.tmp / "volume"
        build_tabby_volume(root)
        col = open_input(root, self.cat)
        self.assertEqual(col.kind, "collected")
        wanted = sorted(
            artifact.rel
            for artifact in col.artifacts
            if artifact.agent == "tabby" and parser.wants(artifact)
        )
        self.assertEqual(
            wanted,
            sorted(
                [
                    "ee/db.sqlite",
                    "ee/db.backup-20260915.sqlite",
                    "events/2025-10-01.json",
                    "config.toml",
                ]
            ),
        )
        rows, _, problems, _ = cli.collect_rows(col, Options(), [])
        self.assertEqual(problems, [])
        self.assertEqual(
            {(row.host, row.user, row.agent) for row in rows}, {("h2", "docker", "tabby")}
        )
        self.assertIn(
            "Where is the cache cleared? [attachments: client_code src/cache.rs:10]",
            [row.text for row in rows if row.session_id == "7"],
        )
        self.assertIn("dump the users table", [row.text for row in rows if row.session_id == "3"])
        self.assertIn("chat_completion", [row.text for row in rows])
        self.assertTrue(
            all(artifact.home.original == TABBY_VOLUME_HOME for artifact in col.artifacts)
        )
