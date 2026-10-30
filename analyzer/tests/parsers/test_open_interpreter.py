"""Tests of the Open Interpreter parser."""

import json

import zstandard

from doubleagent.inputs import open_input
from doubleagent.parsers import Options
from doubleagent.parsers.codex import CodexParser
from doubleagent.parsers.open_interpreter import OpenInterpreterParser
from fixtures import (
    BAD_LINE,
    OI_ARCHIVED,
    OI_IMPORTED,
    OI_ORPHAN,
    OI_ROLLOUT,
    OI_SESSION,
    bad_line_text,
    open_interpreter_rollout_records,
    write_bad_line,
)

from . import test_codex
from .base import ParserBase, _rollout_bytes, rel_only


class OpenInterpreterTests(ParserBase):
    """Test the Open Interpreter parser."""

    LEDGER = ".openinterpreter/external_agent_session_imports.json"

    def test_secondary(self):
        # The history file is a secondary source, the transcript is not.
        parser = OpenInterpreterParser()
        self.assertTrue(parser.secondary(self.artifact(".openinterpreter/history.jsonl")))
        self.assertFalse(parser.secondary(self.artifact(OI_ROLLOUT)))
        for rel in (".openinterpreter/state_5.sqlite", ".openinterpreter/thread_history_1.sqlite"):
            self.assertTrue(parser.wants(self.artifact(rel)), rel)
            self.assertTrue(parser.secondary(self.artifact(rel)), rel)
            self.assertFalse(CodexParser().wants(self.artifact(rel)), rel)

    def test_sqlite_stores(self):
        parser = OpenInterpreterParser()
        state = self.rows_for(parser, ".openinterpreter/state_5.sqlite")
        self.assertEqual(
            [(row.turn_type, row.timestamp_utc, row.session_id, row.text[:40]) for row in state],
            [
                (
                    "system",
                    "2026-09-29T07:00:00.000Z",
                    OI_ORPHAN,
                    "thread record: " + OI_ORPHAN[:25],
                ),
                ("user", "", OI_ORPHAN, "summarise the README"),
                (
                    "system",
                    "2026-10-01T10:00:00.000Z",
                    OI_SESSION,
                    "thread record: " + OI_SESSION[:25],
                ),
                ("user", "", OI_SESSION, "list the files"),
            ],
        )
        self.assertEqual(
            {(row.agent, row.project_path) for row in state}, {("open-interpreter", "/srv/proj")}
        )
        # The fork's schema has no item lifecycle times: created_at_ms is used.
        history = self.rows_for(parser, ".openinterpreter/thread_history_1.sqlite")
        self.assertEqual(
            [
                (row.turn_type, row.timestamp_utc, row.session_id, row.project_path, row.text)
                for row in history
            ],
            [
                (
                    "system",
                    "2026-09-29T07:00:01.000Z",
                    OI_ORPHAN,
                    "/srv/proj",
                    "task_started turn=t1",
                ),
                (
                    "user",
                    "2026-09-29T07:00:02.000Z",
                    OI_ORPHAN,
                    "/srv/proj",
                    "summarise the README",
                ),
                (
                    "assistant",
                    "2026-09-29T07:00:04.000Z",
                    OI_ORPHAN,
                    "/srv/proj",
                    "It describes the project.",
                ),
                (
                    "system",
                    "2026-09-29T07:00:06.000Z",
                    OI_ORPHAN,
                    "/srv/proj",
                    "task_complete turn=t1 duration_ms=5000",
                ),
            ],
        )

    def test_rows(self):
        rows = self.rows_for(OpenInterpreterParser(), OI_ROLLOUT)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",  # session start
                "system",  # model:
                "system",  # permission mode:
                "user",
                "tool_use",
                "tool_result",
                "assistant",
                "system",  # task_complete
                "system",  # model: old -> new
                "system",  # permission mode: old -> new
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "open-interpreter", OI_SESSION),
            )
            self.assertEqual((row.project_path, row.git_branch), ("/srv/proj", "main"))
            self.assertTrue(row.source_file.endswith(OI_ROLLOUT), row.source_file)
        start, model1, mode1, user, use, result, asst, done, model2, mode2 = rows
        self.assertIn("session start: codex_cli_rs 0.9.0 provider=kimi-for-coding", start.text)
        sandbox = 'sandbox={"type":"workspace-write"} profile=- network=- reviewer=- profile_id=-'
        self.assertEqual(
            [(row.timestamp_utc, row.model, row.text) for row in (model1, mode1, model2, mode2)],
            [
                ("2026-10-01T10:00:01.000Z", "kimi-k3", "model: kimi-k3"),
                (
                    "2026-10-01T10:00:01.000Z",
                    "kimi-k3",
                    "permission mode: approval=on-request " + sandbox,
                ),
                ("2026-10-01T10:00:06.000Z", "kimi-k3-turbo", "model: kimi-k3 -> kimi-k3-turbo"),
                (
                    "2026-10-01T10:00:06.000Z",
                    "kimi-k3-turbo",
                    "permission mode: approval=on-request %s -> approval=never %s"
                    % (sandbox, sandbox),
                ),
            ],
        )
        self.assertEqual((start.timestamp_utc, start.model), ("2026-10-01T10:00:00.000Z", ""))
        self.assertEqual(
            (user.text, user.model, user.timestamp_utc),
            ("list the files", "kimi-k3", "2026-10-01T10:00:01.100Z"),
        )
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text), ("Bash", "call_1", '{"command":"ls"}')
        )
        self.assertEqual((result.tool_use_id, result.text), ("call_1", "README.md"))
        self.assertEqual(asst.text, "One file: README.md")
        self.assertEqual(done.text, "task_complete turn=t1 duration_ms=5000")

    def test_thinking_opt_in(self):
        rows = self.rows_for(OpenInterpreterParser(), OI_ROLLOUT, include_thinking=True)
        self.assertEqual([row.text for row in rows if row.turn_type == "thinking"], ["run ls"])

    def test_history(self):
        rows = self.rows_for(OpenInterpreterParser(), ".openinterpreter/history.jsonl")
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows],
            [("user", OI_SESSION, "2026-10-01T10:00:01.000Z", "list the files")],
        )

    def test_archived_and_import_ledger(self):
        rows = self.rows_for(OpenInterpreterParser(), OI_ARCHIVED)
        self.assertEqual(
            [(row.turn_type, row.session_id) for row in rows],
            [("system", OI_IMPORTED), ("user", OI_IMPORTED)],
        )
        rows = self.rows_for(OpenInterpreterParser(), self.LEDGER)
        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual(
            (row.turn_type, row.session_id, row.timestamp_utc, row.source_line),
            ("system", OI_IMPORTED, "2026-09-30T10:00:00.000Z", 1),
        )
        self.assertEqual(
            row.text,
            "imported session from /home/alice/.claude/projects/-home-alice-proj/5b1c.jsonl "
            "sha256=ab12cd34 source_modified=2026-09-30T09:00:00.000Z",
        )

    def test_bad_ledger_is_one_row(self):
        (self.home / self.LEDGER).write_text('{"records": [', encoding="utf-8")
        rows = self.rows_for(OpenInterpreterParser(), self.LEDGER)
        self.assertEqual([row.turn_type for row in rows], ["system"])
        self.assertIn("unreadable import ledger", rows[0].text)

    def test_not_wanted(self):
        parser = OpenInterpreterParser()
        arts = {artifact.rel: artifact for artifact in self.col.artifacts}
        for rel in (
            ".openinterpreter/auth.json",
            ".openinterpreter/session_index.jsonl",
        ):
            self.assertEqual(arts[rel].agent, "open-interpreter")
            self.assertFalse(parser.wants(arts[rel]), rel)
        # config.toml is read for its standing permission keys; the
        # fixture's names none, so it gives no rows.
        config = arts[".openinterpreter/config.toml"]
        self.assertTrue(parser.wants(config))
        self.assertEqual(list(parser.parse(config, Options())), [])
        self.assertTrue(parser.wants(rel_only(".openinterpreter/rules/default.rules")))
        self.assertFalse(CodexParser().wants(rel_only(".openinterpreter/rules/default.rules")))
        self.assertFalse(parser.wants(arts[test_codex.CodexTests.REL]))
        self.assertFalse(CodexParser().wants(arts[OI_ROLLOUT]))
        self.assertFalse(CodexParser().wants(arts[self.LEDGER]))
        self.assertTrue(parser.wants(rel_only(OI_ARCHIVED + ".zst")))

    def test_truncated_line_is_reported_not_fatal(self):
        write_bad_line(self.home / OI_ROLLOUT)
        rows = self.rows_for(OpenInterpreterParser(), OI_ROLLOUT)
        self.assertEqual(len(rows), 11)
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].session_id, rows[-1].source_line, rows[-1].text),
            ("system", OI_SESSION, 10, bad_line_text()),
        )

    def test_bad_line_mid_file_keeps_the_rows_around_it(self):
        row = test_codex.CodexTests.check_bad_line(self, OpenInterpreterParser, OI_ROLLOUT, 3)
        self.assertEqual(row.session_id, OI_SESSION)

    def test_truncated_history_line(self):
        rel = ".openinterpreter/history.jsonl"
        with open(self.home / rel, "a", encoding="utf-8") as handle:
            handle.write(BAD_LINE + "\n")
            handle.write(json.dumps({"session_id": OI_SESSION, "ts": 1790848900, "text": "next"}))
            handle.write("\n")
        rows = self.rows_for(OpenInterpreterParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.source_line, row.text) for row in rows],
            [
                ("user", 1, "list the files"),
                ("user", 3, "next"),
                ("system", 2, bad_line_text()),
            ],
        )

    def test_zst_rollout(self):
        rel = OI_ROLLOUT + ".zst"
        path = self.home / rel
        path.write_bytes(
            zstandard.ZstdCompressor().compress(_rollout_bytes(open_interpreter_rollout_records()))
        )
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(OpenInterpreterParser(), rel)
        self.assertEqual(len(rows), 10)
        self.assertEqual({row.agent for row in rows}, {"open-interpreter"})
