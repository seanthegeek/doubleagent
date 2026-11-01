"""Tests of the Ollama parser."""

import io
import unittest
from pathlib import Path

from test_errors import CAN_DENY, DENY_REASON

from doubleagent import cli
from doubleagent.parsers import Options
from doubleagent.parsers.antigravity import file_mtime
from doubleagent.parsers.claude_desktop import ClaudeDesktopParser
from doubleagent.parsers.ollama import OllamaParser, call_text
from fixtures import (
    OLLAMA_ATTACHMENT,
    OLLAMA_CD_PROFILE_REL,
    OLLAMA_CHAT,
    OLLAMA_DATABASE_REL,
    OLLAMA_MANIFEST_REL,
    OLLAMA_USER_EMAIL,
    OLLAMA_USER_NAME,
    OLLAMA_WINDOWS_DATABASE_REL,
)

from .base import ParserBase, model_or_permission_row, read_jsonl, rel_only


class OllamaTests(ParserBase):
    """Test the Ollama parser."""

    def test_history_trims_ascii_whitespace_only(self):
        path = self.home / self.HISTORY_REL
        space = chr(0x00A0)
        path.write_bytes(("why%s \t\r\n%s\n \n" % (space, chr(0x2028))).encode("utf-8"))
        rows = self.rows_for(OllamaParser(), self.HISTORY_REL)
        self.assertEqual(
            [(row.turn_type, row.text, row.source_line) for row in rows],
            [("user", "why" + space, 1), ("user", chr(0x2028), 2)],
        )

    HISTORY_REL = ".ollama/history"
    CONFIG_REL = ".ollama/config.json"
    CALL_ID = OLLAMA_CHAT + ":1"

    def run_timeline(self, name, *extra):
        import contextlib

        out = self.tmp / name
        with contextlib.redirect_stdout(io.StringIO()):
            exit_code = cli.main(
                ["timeline", str(self.tmp / "home"), "-o", str(out), "--agent", "ollama", *extra]
            )
        self.assertEqual(exit_code, 0)
        return out

    def test_rows(self):
        rows = [
            row
            for row in self.rows_for(OllamaParser(), OLLAMA_DATABASE_REL)
            if not model_or_permission_row(row)
        ]
        self.assertEqual(
            [row.turn_type for row in rows], ["user", "tool_use", "tool_result", "assistant"]
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "ollama", OLLAMA_CHAT),
            )
            self.assertEqual((row.project_path, row.git_branch), ("", ""))
        user, use, result, asst = rows
        self.assertEqual(
            (user.text, user.timestamp_utc, user.model, user.source_line),
            (
                "summarise https://example.invalid/notes\n[attachment: notes.txt, 5 bytes]",
                "2026-03-01T17:00:00.500Z",
                "",
                1,
            ),
        )
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text, use.model, use.timestamp_utc),
            (
                "web_fetch",
                self.CALL_ID,
                "https://example.invalid/notes",
                "gemma4:e4b",
                "2026-03-01T17:00:01.250Z",
            ),
        )
        self.assertEqual(use.source_line, 1)  # tool_calls.id
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.text, result.timestamp_utc),
            ("web_fetch", self.CALL_ID, "Release 2.0 adds X.", "2026-03-01T17:00:03.000Z"),
        )
        self.assertEqual(
            (asst.text, asst.model, asst.timestamp_utc, asst.source_line),
            ("Release 2.0 adds X.", "qwen3:8b", "2026-03-01T17:00:04.125Z", 4),
        )

    def test_model_rows(self):
        for thinking in (False, True):
            rows = self.rows_for(OllamaParser(), OLLAMA_DATABASE_REL, include_thinking=thinking)
            state = [row for row in rows if model_or_permission_row(row)]
            self.assertEqual(
                [
                    (row.text, row.timestamp_utc, row.model, row.source_line, row.session_id)
                    for row in state
                ],
                [
                    ("model: gemma4:e4b", "2026-03-01T17:00:01.250Z", "gemma4:e4b", 2, OLLAMA_CHAT),
                    (
                        "model: gemma4:e4b -> qwen3:8b",
                        "2026-03-01T17:00:04.125Z",
                        "qwen3:8b",
                        4,
                        OLLAMA_CHAT,
                    ),
                ],
            )
            # each sits immediately before its message's first row
            self.assertEqual(rows[rows.index(state[0]) + 1].source_line, 2 if thinking else 1)
            self.assertEqual(rows[rows.index(state[1]) + 1].text, "Release 2.0 adds X.")

    def test_thinking_opt_in(self):
        rows = self.rows_for(OllamaParser(), OLLAMA_DATABASE_REL, include_thinking=True)
        rows = [row for row in rows if not model_or_permission_row(row)]
        self.assertEqual(
            [row.turn_type for row in rows],
            ["user", "thinking", "tool_use", "tool_result", "assistant"],
        )
        thinking = rows[1]
        self.assertEqual(
            (thinking.text, thinking.model, thinking.timestamp_utc, thinking.source_line),
            ("need to fetch the page", "gemma4:e4b", "2026-03-01T17:00:01.250Z", 2),
        )

    def test_schema_version_16_gives_the_same_rows(self):
        def strip(rows):
            return [
                {k: value for k, value in row.as_dict().items() if k != "source_file"}
                for row in rows
            ]

        for thinking in (False, True):
            mac = self.rows_for(OllamaParser(), OLLAMA_DATABASE_REL, include_thinking=thinking)
            win = self.rows_for(
                OllamaParser(), OLLAMA_WINDOWS_DATABASE_REL, include_thinking=thinking
            )
            self.assertTrue(mac)
            self.assertEqual(strip(win), strip(mac))
            self.assertTrue(
                all(row.source_file.endswith(OLLAMA_WINDOWS_DATABASE_REL) for row in win)
            )

    def test_account_and_attachment_bytes_never_emitted(self):
        out = self.run_timeline("out", "--include-thinking")
        self.assertTrue(read_jsonl(out / "timeline.jsonl"))
        forbidden = (
            OLLAMA_USER_NAME,
            OLLAMA_USER_EMAIL,
            '"free"',
            "0190a000-0000-7000-8000-00000000d001",  # settings.device_id
            OLLAMA_ATTACHMENT.decode(),
            OLLAMA_ATTACHMENT.hex(),
        )
        for name in ("timeline.jsonl", "sessions.jsonl"):
            text = (out / name).read_text(encoding="utf-8")
            for secret in forbidden:
                self.assertNotIn(secret, text, (name, secret))

    def test_history_and_slash_commands(self):
        rows = self.rows_for(OllamaParser(), self.HISTORY_REL)
        self.assertEqual(
            [(row.turn_type, row.text, row.source_line) for row in rows],
            [
                ("user", "why is the sky blue", 1),
                ("system", "slash command: /set nohistory", 2),
                ("system", "slash command: /bye", 3),
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.timestamp_utc, row.session_id, row.model, row.agent), ("", "", "", "ollama")
            )

    def test_undated_history_in_timeline_and_sessions(self):
        # cli.collect_rows sorts on (timestamp_utc == "", ...), so undated
        # rows come last; filters.RowFilter.in_window drops them under
        # --since unless --keep-undated; model.summarise skips rows with no
        # session_id, so the history adds no session.
        out = self.run_timeline("all")
        rows = read_jsonl(out / "timeline.jsonl")
        sessions = read_jsonl(out / "sessions.jsonl")
        history = [row for row in rows if row["source_file"].endswith("/.ollama/history")]
        self.assertEqual(len(history), 3)
        self.assertEqual(rows[-3:], history)
        self.assertTrue(all(row["timestamp_utc"] for row in rows[:-3]))
        self.assertEqual({session["session_id"] for session in sessions}, {OLLAMA_CHAT})
        for session in sessions:
            self.assertEqual(
                (session["first_timestamp_utc"], session["last_timestamp_utc"], session["models"]),
                (
                    "2026-03-01T17:00:00.500Z",
                    "2026-03-01T17:00:04.125Z",
                    ["gemma4:e4b", "qwen3:8b"],
                ),
            )
        rows = read_jsonl(self.run_timeline("since", "--since", "2026-03-01") / "timeline.jsonl")
        self.assertTrue(rows)
        self.assertFalse([row for row in rows if not row["timestamp_utc"]])
        out = self.run_timeline("undated", "--since", "2026-03-01", "--keep-undated")
        rows = read_jsonl(out / "timeline.jsonl")
        self.assertEqual(len([row for row in rows if not row["timestamp_utc"]]), 3)

    def test_tool_pairing_and_placeholder(self):
        import sqlite3

        chat = OLLAMA_CHAT
        connection = sqlite3.connect(str(self.home / OLLAMA_DATABASE_REL))
        connection.executescript(
            "INSERT INTO messages (id, chat_id, role, content, model_name, created_at) VALUES"
            " (5, '%(c)s', 'user', 'search twice', NULL, '2026-03-01 09:01:00-08:00'),"
            " (6, '%(c)s', 'assistant', 'Searching.', 'gemma4:e4b', '2026-03-01 09:01:01-08:00'),"
            " (7, '%(c)s', 'tool', '', NULL, '2026-03-01 09:01:02-08:00'),"
            " (8, '%(c)s', 'tool', 'second', NULL, '2026-03-01 09:01:03-08:00'),"
            " (9, '%(c)s', 'tool', 'extra', NULL, '2026-03-01 09:01:04-08:00'),"
            " (10, '%(c)s', 'assistant', '', 'gemma4:e4b', '2026-03-01 09:01:05-08:00');"
            "UPDATE messages SET tool_result = '{\"results\":[1]}' WHERE id = 7;"
            "INSERT INTO tool_calls VALUES (2, 6, 'function', 'web_search',"
            ' \'{"query":"ollama","max_results":3}\', NULL),'
            " (3, 6, 'function', 'browser.find', '{\"pattern\":\"x\"}', NULL);" % {"c": chat}
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(OllamaParser(), OLLAMA_DATABASE_REL)
        # message 6 switches back to gemma4; the placeholder 10 yields nothing
        self.assertEqual(
            [row.text for row in rows if model_or_permission_row(row)][2:],
            ["model: qwen3:8b -> gemma4:e4b"],
        )
        rows = [row for row in rows if not model_or_permission_row(row)][4:]
        self.assertEqual(
            [
                (row.turn_type, row.tool_name, row.tool_use_id, row.text, row.source_line)
                for row in rows
            ],
            [
                ("user", "", "", "search twice", 5),
                ("assistant", "", "", "Searching.", 6),
                ("tool_use", "web_search", chat + ":2", "ollama", 2),
                ("tool_use", "browser.find", chat + ":3", '{"pattern":"x"}', 3),
                ("tool_result", "web_search", chat + ":2", '{"results":[1]}', 7),
                ("tool_result", "browser.find", chat + ":3", "second", 8),
                ("tool_result", "browser.find", "", "extra", 9),
            ],
        )
        self.assertEqual(call_text("not json"), "not json")
        self.assertEqual(call_text('{"url":""}'), '{"url":""}')

    def test_other_files_not_wanted(self):
        parser = OllamaParser()
        wanted = {
            OLLAMA_DATABASE_REL,
            OLLAMA_WINDOWS_DATABASE_REL,
            self.HISTORY_REL,
            self.CONFIG_REL,
        }
        rels = {artifact.rel for artifact in self.col.artifacts if artifact.agent == "ollama"}
        for rel in (
            OLLAMA_MANIFEST_REL,
            OLLAMA_DATABASE_REL + "-wal",
            OLLAMA_WINDOWS_DATABASE_REL + "-shm",
            ".ollama/backup/config.json.1772355600",
            ".ollama/onboarding-v1.completed",
            ".ollama/id_ed25519",
            ".ollama/logs/server.log",
        ):
            self.assertIn(rel, rels)
        for artifact in self.col.artifacts:
            if artifact.agent == "ollama":
                self.assertEqual(parser.wants(artifact), artifact.rel in wanted, artifact.rel)
        # Of the claude-desktop files, only the profile Ollama writes.
        profile = self.artifact(OLLAMA_CD_PROFILE_REL)
        self.assertEqual(profile.agent, "claude-desktop")
        meta = self.artifact(str(Path(OLLAMA_CD_PROFILE_REL).parent / "_meta.json"))
        self.assertFalse(parser.wants(meta))
        self.assertFalse(ClaudeDesktopParser().wants(profile))
        self.assertEqual(
            wanted | {OLLAMA_CD_PROFILE_REL},
            {artifact.rel for artifact in self.col.artifacts if parser.wants(artifact)},
        )
        self.assertIn("claude-desktop", parser.reads_agents)
        for root in ("AppData/Local/Claude-3p/", "AppData/Roaming/Claude-3p/"):
            rel = root + "configLibrary/00000000-0000-4000-8000-000000000114.json"
            self.assertEqual(parser.wants(rel_only(rel)), root.startswith("AppData/Local"), rel)
        self.assertFalse(
            parser.wants(rel_only("Library/Application Support/Claude-3p/configLibrary/other.json"))
        )

    def test_not_a_database_is_one_system_row(self):
        database = self.home / OLLAMA_DATABASE_REL
        for suffix in ("-wal", "-shm"):
            Path(str(database) + suffix).unlink()
        database.write_text("not a database\n", encoding="utf-8")
        rows = self.rows_for(OllamaParser(), OLLAMA_DATABASE_REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("system", "parser: not a SQLite database")],
        )

    def test_truncated_database_is_reported_not_fatal(self):
        database = self.home / OLLAMA_DATABASE_REL
        for suffix in ("-wal", "-shm"):
            Path(str(database) + suffix).unlink()
        database.write_bytes(database.read_bytes()[:200])
        rows = self.rows_for(OllamaParser(), OLLAMA_DATABASE_REL)
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0].turn_type, "system")
        self.assertIn("parser: SQLite error", rows[0].text)

    def test_truncated_history_line_still_yields_earlier_lines(self):
        # A last line cut inside a UTF-8 sequence is reported as read, not
        # yielded as a user row with a replacement character.
        with open(self.home / self.HISTORY_REL, "ab") as handle:
            handle.write(b"half a pro\xe2")
        rows = self.rows_for(OllamaParser(), self.HISTORY_REL)
        self.assertEqual(
            [(row.turn_type, row.text, row.source_line) for row in rows],
            [
                ("user", "why is the sky blue", 1),
                ("system", "slash command: /set nohistory", 2),
                ("system", "slash command: /bye", 3),
                (
                    "system",
                    "parser: unparseable line ('utf-8' codec can't decode byte 0xe2"
                    " in position 10: unexpected end of data): half a pro\ufffd",
                    4,
                ),
            ],
        )

    def test_undecodable_history_line_is_its_own_parser_row(self):
        # Each line that is not valid UTF-8 becomes one system row carrying
        # the line in full; the lines before and after it are still
        # yielded, and so is a second bad line.
        (self.home / self.HISTORY_REL).write_bytes(
            b"before\n  caf\xe9 \xff au lait \r\n\n/bye\n\xc3after\nlast\n"
        )
        rows = self.rows_for(OllamaParser(), self.HISTORY_REL)
        self.assertEqual(
            [(row.turn_type, row.text, row.source_line) for row in rows],
            [
                ("user", "before", 1),
                ("system", "slash command: /bye", 4),
                ("user", "last", 6),
                (
                    "system",
                    "parser: unparseable line ('utf-8' codec can't decode byte 0xe9"
                    " in position 5: invalid continuation byte):   caf\ufffd \ufffd au lait ",
                    2,
                ),
                (
                    "system",
                    "parser: unparseable line ('utf-8' codec can't decode byte 0xc3"
                    " in position 0: invalid continuation byte): \ufffdafter",
                    5,
                ),
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.timestamp_utc, row.session_id, row.project_path, row.agent),
                ("", "", "", "ollama"),
            )
        self.assertFalse([row for row in rows if row.turn_type == "user" and "\ufffd" in row.text])

    def test_auto_mode_permission_settings(self):
        # The launcher's automode key and the autoModeEnabled key it
        # writes into Claude Desktop's profile, at each file's mtime.
        rows = self.rows_for(OllamaParser(), self.CONFIG_REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [
                (
                    "system",
                    "permission setting: integrations.claude-desktop.automode=true"
                    " (from .ollama/config.json, file mtime)",
                )
            ],
        )
        self.assertEqual(rows[0].timestamp_utc, file_mtime(self.artifact(self.CONFIG_REL)))
        rows = self.rows_for(OllamaParser(), OLLAMA_CD_PROFILE_REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [
                (
                    "system",
                    "permission setting: autoModeEnabled=true (set by Ollama's Claude Desktop"
                    " launcher, from %s, file mtime)" % OLLAMA_CD_PROFILE_REL,
                )
            ],
        )
        self.assertEqual(rows[0].timestamp_utc, file_mtime(self.artifact(OLLAMA_CD_PROFILE_REL)))
        for row in rows:
            self.assertEqual(
                (row.agent, row.session_id, row.project_path, row.model), ("ollama", "", "", "")
            )
            self.assertTrue(row.source_file.endswith(OLLAMA_CD_PROFILE_REL))
        # The profile, catalogued under claude-desktop, reaches the parser.
        out = self.run_timeline("perm")
        texts = [row["text"] for row in read_jsonl(out / "timeline.jsonl")]
        self.assertIn(
            "permission setting: autoModeEnabled=true (set by Ollama's Claude Desktop"
            " launcher, from %s, file mtime)" % OLLAMA_CD_PROFILE_REL,
            texts,
        )
        # The inference gateway's literal key is not a row.
        self.assertFalse([text for text in texts if "inferenceGatewayApiKey" in text])

    def test_auto_mode_absent_gives_no_row(self):
        # An unset automode key (the launcher's default, true) and a profile
        # that --restore stripped of autoModeEnabled give no row.
        (self.home / self.CONFIG_REL).write_text(
            '{"integrations":{"claude-desktop":{"models":["gemma4:e4b"]}}}', encoding="utf-8"
        )
        (self.home / OLLAMA_CD_PROFILE_REL).write_text(
            '{"disableDeploymentModeChooser":false}', encoding="utf-8"
        )
        self.assertEqual(self.rows_for(OllamaParser(), self.CONFIG_REL), [])
        self.assertEqual(self.rows_for(OllamaParser(), OLLAMA_CD_PROFILE_REL), [])

    def test_config_and_profile_not_json_are_one_parser_row(self):
        for rel, body in (
            (self.CONFIG_REL, '{"integrations":{"claude-desktop":{"automode":tr'),
            (OLLAMA_CD_PROFILE_REL, "[true]"),
        ):
            (self.home / rel).write_text(body, encoding="utf-8")
            rows = self.rows_for(OllamaParser(), rel)
            self.assertEqual(len(rows), 1, rel)
            self.assertEqual(rows[0].turn_type, "system")
            self.assertTrue(rows[0].text.startswith("parser: %s is not " % rel), rows[0].text)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_settings_file_is_one_row_with_no_line_or_time(self):
        parser = OllamaParser()
        for target in (self.CONFIG_REL, OLLAMA_CD_PROFILE_REL):
            locked = self.home / target
            locked.chmod(0)
            try:
                rows = self.rows_for(parser, target)
            finally:
                locked.chmod(0o600)
            self.assertEqual(
                [(row.turn_type, row.text, row.source_line, row.timestamp_utc) for row in rows],
                [
                    (
                        "system",
                        "parser: %s could not be read (Permission denied)" % locked.name,
                        0,
                        "",
                    )
                ],
                target,
            )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_history_is_one_row(self):
        parser = OllamaParser()
        target = self.HISTORY_REL
        others = [
            artifact
            for artifact in self.col.artifacts
            if parser.wants(artifact) and artifact.rel != target
        ]
        locked = self.home / target
        locked.chmod(0)
        try:
            rows = self.rows_for(parser, target)
            later = [row for artifact in others for row in parser.parse(artifact, Options())]
        finally:
            locked.chmod(0o600)
        reported = [row for row in rows if row.text.startswith("parser:")]
        self.assertEqual(
            [
                (row.turn_type, row.text, row.source_line, row.timestamp_utc, row.session_id)
                for row in reported
            ],
            [
                (
                    "system",
                    "parser: %s could not be read (Permission denied)" % locked.name,
                    0,
                    "",
                    "",
                )
            ],
        )
        self.assertEqual(reported[0].project_path, "")
        self.assertEqual(len(rows), 1)
        self.assertTrue(any(not row.text.startswith("parser:") for row in later))
