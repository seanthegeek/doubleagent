"""Tests of the ShellGPT parser."""

import unittest

from test_errors import CAN_DENY, DENY_REASON

from doubleagent import cli
from doubleagent.parsers import Options
from doubleagent.parsers.shellgpt import ShellGptParser, load_messages
from fixtures import SHELLGPT_CACHE_COPY, SHELLGPT_CHAT, SHELLGPT_LEGACY_CHAT

from .base import ParserBase


class ShellGptTests(ParserBase):
    """Test the ShellGPT parser."""

    CHAT = "AppData/Local/Temp/chat_cache/" + SHELLGPT_CHAT
    LEGACY = "AppData/Local/Temp/shell_gpt/chat_cache/" + SHELLGPT_LEGACY_CHAT

    def test_rows(self):
        rows = self.rows_for(ShellGptParser(), self.CHAT)
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "user", "tool_use", "tool_result", "assistant"],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "shellgpt", SHELLGPT_CHAT),
            )
            self.assertEqual(
                (row.timestamp_utc, row.project_path, row.git_branch, row.model, row.source_line),
                ("", "", "", "", 1),
            )
        system, user, use, result, asst = rows
        self.assertTrue(system.text.startswith("You are ShellGPT"))
        self.assertEqual(user.text, "what is listening on port 8080")
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text),
            ("execute_shell_command", "call_Q1w2e3r4", "ss -ltnp | grep 8080"),
        )
        self.assertEqual(
            (result.tool_name, result.tool_use_id), ("execute_shell_command", "call_Q1w2e3r4")
        )
        self.assertTrue(result.text.startswith("Exit code: 0, Output:\nLISTEN"))
        self.assertTrue(asst.text.endswith("is listening on 8080."))

    def test_legacy_function_call(self):
        rows = self.rows_for(ShellGptParser(), self.LEGACY)
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "user", "tool_use", "tool_result", "assistant"],
        )
        use, result = rows[2], rows[3]
        self.assertEqual((use.tool_use_id, use.text), ("function_call_2", "df -h /"))
        self.assertEqual(
            (result.tool_use_id, result.tool_name), ("function_call_2", "execute_shell_command")
        )

    def test_thinking_opt_in(self):
        # ShellGPT stores no reasoning; the option changes nothing.
        self.assertEqual(
            [
                row.turn_type
                for row in self.rows_for(ShellGptParser(), self.CHAT, include_thinking=True)
            ],
            [row.turn_type for row in self.rows_for(ShellGptParser(), self.CHAT)],
        )

    SGPTRC = ".config/shell_gpt/.sgptrc"
    FUNCTION = ".config/shell_gpt/functions/execute_shell.py"
    CACHE = "AppData/Local/Temp/shell_gpt/cache/"

    def test_not_wanted(self):
        art = self.artifact(".config/shell_gpt/roles/ShellGPT.json")
        self.assertFalse(ShellGptParser().wants(art))
        for rel in (self.SGPTRC, self.FUNCTION, self.CACHE + SHELLGPT_CACHE_COPY):
            self.assertTrue(ShellGptParser().wants(self.artifact(rel)), rel)

    def test_settings_rows(self):
        rows = self.rows_for(ShellGptParser(), self.SGPTRC)
        rel = self.SGPTRC
        self.assertEqual(
            [row.text for row in rows],
            [
                "model: gpt-5.4-mini (configured default, from %s, file mtime)" % rel,
                "permission setting: DEFAULT_EXECUTE_SHELL_CMD=true (from %s, file mtime)" % rel,
                "permission setting: OPENAI_USE_FUNCTIONS=true (from %s, file mtime)" % rel,
                "permission setting: SHELL_INTERACTION=false (from %s, file mtime)" % rel,
            ],
        )
        self.assertEqual(rows[0].model, "gpt-5.4-mini")
        for row in rows:
            self.assertEqual((row.turn_type, row.session_id), ("system", ""))
            self.assertTrue(row.timestamp_utc.endswith("Z"))
            self.assertNotIn("sk-not-real", row.text)
            self.assertNotIn("API_BASE_URL", row.text)

    def test_settings_bad_line_never_shows_the_line(self):
        path = self.home / self.SGPTRC
        path.write_text(
            "DEFAULT_MODEL=gpt-4o\nOPENAI_API_KEY sk-not-real\nSHELL_INTERACTION=true\n",
            encoding="utf-8",
        )
        rows = self.rows_for(ShellGptParser(), self.SGPTRC)
        self.assertEqual(
            [row.text.split(" (")[0] for row in rows],
            [
                "model: gpt-4o",
                "permission setting: SHELL_INTERACTION=true",
                "parser: unparseable line 2",
            ],
        )
        self.assertEqual(
            rows[-1].text,
            'parser: unparseable line 2 (no "=" separator; not shown, the file holds the API key)',
        )
        for row in rows:
            self.assertNotIn("sk-not-real", row.text)

    def test_function_rows(self):
        rows = self.rows_for(ShellGptParser(), self.FUNCTION)
        self.assertEqual(len(rows), 1)
        self.assertEqual(
            rows[0].text,
            "permission setting: function execute_shell.py "
            "(from .config/shell_gpt/functions, file mtime)",
        )
        self.assertEqual(rows[0].turn_type, "system")
        self.assertTrue(rows[0].timestamp_utc.endswith("Z"))

    def test_response_cache_is_secondary(self):
        parser = ShellGptParser()
        self.assertTrue(parser.has_secondary)
        cache = self.artifact(self.CACHE + SHELLGPT_CACHE_COPY)
        self.assertTrue(parser.secondary(cache))
        for rel in (self.CHAT, self.LEGACY, self.SGPTRC, self.FUNCTION):
            self.assertFalse(parser.secondary(self.artifact(rel)), rel)
        rows = self.rows_for(parser, self.CACHE + SHELLGPT_CACHE_COPY)
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc, row.session_id) for row in rows],
            [("assistant", "30G is free on /.", "", "")],
        )
        self.assertEqual(self.rows_for(parser, self.CACHE + "d41d8cd98f00b204e9800998ecf8427e"), [])

    def test_response_cache_dedup(self):
        rows, _, _, deduplicated = cli.collect_rows(self.col, Options(), ["shellgpt"])
        self.assertEqual(deduplicated, {"shellgpt": 1})
        cached = [
            row
            for row in rows
            if row.source_file.endswith("/shell_gpt/cache/" + "0cc175b9c0f1b6a831c399e269772661")
        ]
        self.assertEqual(
            [(row.text, row.recovered) for row in cached], [("a cached response", "yes")]
        )
        self.assertFalse([row for row in rows if row.source_file.endswith(SHELLGPT_CACHE_COPY)])

    def test_loose_temp_directory_is_wanted(self):
        art = next(artifact for artifact in self.col.artifacts if artifact.rel == self.CHAT)
        art.rel = "tmp/chat_cache/" + SHELLGPT_CHAT
        self.assertTrue(ShellGptParser().wants(art))

    def test_truncated_line_is_reported_not_fatal(self):
        path = self.home / self.CHAT
        raw = path.read_text(encoding="utf-8")
        path.write_text(raw[: raw.index('{"role": "tool"') + 20], encoding="utf-8")
        rows = self.rows_for(ShellGptParser(), self.CHAT)
        self.assertEqual([row.turn_type for row in rows], ["system", "user", "tool_use", "system"])
        self.assertIn("parser: truncated after 3 message(s)", rows[-1].text)
        msgs, error = load_messages("not json")
        self.assertEqual(msgs, [])
        self.assertTrue(error)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_chat_file_is_one_row(self):
        parser = ShellGptParser()
        target = self.CHAT
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
                    SHELLGPT_CHAT,
                )
            ],
        )
        self.assertEqual(reported[0].project_path, "")
        self.assertEqual(len(rows), 1)
        self.assertTrue(any(not row.text.startswith("parser:") for row in later))
