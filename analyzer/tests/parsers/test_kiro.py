"""Tests of the Kiro parser."""

import json
import unittest

from test_errors import CAN_DENY, DENY_REASON

from doubleagent.dedup import deduplicate
from doubleagent.inputs import Artifact, Home, open_input
from doubleagent.parsers import Options
from doubleagent.parsers.kiro import KiroParser, salvage
from doubleagent.parsers.kiro import unescape_history as kiro_unescape
from doubleagent.timeutil import to_utc
from fixtures import (
    KIRO_DENIAL_SESSION,
    KIRO_EXPORT_SESSION,
    KIRO_SESSION,
    KIRO_SHELL_SESSION,
    kiro_delegate_records,
)

from .base import ParserBase, kept_rows, model_or_permission_row, parse_with, unreadable_rows


class KiroTests(ParserBase):
    """Test the Kiro parser."""

    def test_salvage_takes_json_whitespace_only(self):
        template = '{"conversation_id"%s:%s"c1", "history"%s:%s[{"a": 1}, {"b'
        state, _ = salvage(template % (" ", "\n", "\r\n", "\t"))
        self.assertEqual(state and state.get("conversation_id"), "c1")
        for space in map(chr, (0x00A0, 0x2028, 0x0085)):
            state, _ = salvage(template % (space, "", space, ""))
            self.assertIsNone(state and state.get("conversation_id"), repr(space))

    DATABASE = ".local/share/amazon-q/data.sqlite3"
    EXPORT = ".aws/amazonq/exports/issue-chat.json"

    def test_rows(self):
        rows = self.rows_for(KiroParser(), self.DATABASE)
        conv = [
            row
            for row in rows
            if row.session_id == KIRO_SESSION and not model_or_permission_row(row)
        ]
        self.assertEqual(
            [row.turn_type for row in conv],
            ["user", "assistant", "tool_use", "tool_result", "assistant"],
        )
        for row in conv:
            self.assertEqual((row.host, row.user, row.agent), ("h1", "alice", "kiro"))
            self.assertEqual((row.project_path, row.git_branch), ("/srv/proj", ""))
            self.assertEqual(row.source_line, 1)
        # the second request was answered by another model
        self.assertEqual(
            [row.model for row in conv], ["claude-sonnet-4"] * 3 + ["claude-opus-4"] * 2
        )
        user, asst, use, result, final = conv
        self.assertEqual(
            (user.text, user.timestamp_utc), ("list the files here", "2026-04-24T03:17:15.123Z")
        )
        self.assertEqual(
            (asst.text, asst.timestamp_utc),
            ("I will list the directory.", "2026-04-24T03:17:16.900Z"),
        )
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text),
            ("execute_bash", "tooluse_abc123", '{"command":"ls -la"}'),
        )
        # null user.timestamp falls back to the entry's request start
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.timestamp_utc),
            ("execute_bash", "tooluse_abc123", "2026-04-24T03:17:17.000Z"),
        )
        self.assertEqual(result.text, "total 8\nREADME.md [Success]")
        self.assertEqual(final.timestamp_utc, "2026-04-24T03:17:18.100Z")

    def test_export_variants(self):
        rows = [
            row
            for row in self.rows_for(KiroParser(), self.EXPORT)
            if not model_or_permission_row(row)
        ]
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "user",
                "tool_use",
                "tool_result",
                "assistant",
                "tool_use",
                "user",
                "tool_result",
                "assistant",
                "system",
                "system",
                "system",
            ],
        )
        self.assertTrue(
            all(
                row.session_id == KIRO_EXPORT_SESSION and row.project_path == "/srv/proj"
                for row in rows
            )
        )
        user, mcp, mcpres, _, _, stop, cancelled, _, compact_, summary, pending = rows
        self.assertEqual(
            (user.timestamp_utc, user.model), ("2026-04-24T14:00:00.000Z", "claude-3.7-sonnet")
        )  # legacy model
        self.assertEqual(
            (mcp.tool_name, mcp.text),
            ("github___create_issue", 'orig_name=create_issue {"title":"bug"}'),
        )
        self.assertEqual(
            (mcpres.tool_name, mcpres.text), ("github___create_issue", '{"number":7} [Error]')
        )
        self.assertEqual(stop.text, "stop, do not delete")
        self.assertEqual(
            (cancelled.tool_use_id, cancelled.text),
            ("tooluse_bash2", "cancelled Tool use was cancelled by the user [Error]"),
        )
        self.assertTrue(compact_.text.startswith("compact:"))
        self.assertEqual(
            summary.text, "summary: User asked to open an issue and cancelled a delete."
        )
        self.assertEqual(pending.text, "pending message (not sent): now push it")

    def test_model_rows(self):
        rows = [
            row
            for row in self.rows_for(KiroParser(), self.DATABASE)
            if row.session_id == KIRO_SESSION
        ]
        state = [(i, row) for i, row in enumerate(rows) if model_or_permission_row(row)]
        self.assertEqual(
            [(i, row.text, row.timestamp_utc, row.model) for i, row in state],
            [
                (0, "model: claude-sonnet-4", "2026-04-24T03:17:15.123Z", "claude-sonnet-4"),
                # before the second entry's rows; its user timestamp is null,
                # so the request start
                (
                    4,
                    "model: claude-sonnet-4 -> claude-opus-4",
                    "2026-04-24T03:17:17.000Z",
                    "claude-opus-4",
                ),
            ],
        )
        for _, row in state:
            self.assertEqual(
                (row.turn_type, row.project_path, row.source_line), ("system", "/srv/proj", 1)
            )
        # an entry without model_id falls back to the conversation's model and says so
        rows = self.rows_for(KiroParser(), self.EXPORT)
        self.assertEqual(
            [row.text for row in rows if row.text.startswith("model: ")],
            [
                "model: claude-3.7-sonnet source=model",
                "model: claude-3.7-sonnet -> claude-sonnet-4",
            ],
        )
        self.assertEqual(rows[0].timestamp_utc, "2026-04-24T14:00:00.000Z")

    def test_thinking_opt_in(self):
        # the format records no reasoning, so the option changes nothing
        for rel in (self.DATABASE, self.EXPORT):
            with_thinking = self.rows_for(KiroParser(), rel, include_thinking=True)
            self.assertEqual(len(self.rows_for(KiroParser(), rel)), len(with_thinking))
            self.assertFalse([row for row in with_thinking if row.turn_type == "thinking"])

    def test_shell_history_and_bad_blob(self):
        rows = self.rows_for(KiroParser(), self.DATABASE)
        shell = [row for row in rows if row.session_id == KIRO_SHELL_SESSION]
        self.assertEqual(len(shell), 1)
        self.assertEqual(
            (shell[0].turn_type, shell[0].timestamp_utc, shell[0].project_path),
            ("system", "2026-04-24T03:00:00.000Z", "/srv/proj"),
        )
        self.assertTrue(shell[0].text.startswith("shell history: git status"))
        bad = [row for row in rows if row.project_path == "/srv/broken"]
        self.assertEqual(len(bad), 1)
        self.assertEqual((bad[0].turn_type, bad[0].source_line), ("system", 2))
        self.assertIn("unparseable", bad[0].text)
        self.assertNotIn("REDACT-ME", " ".join(row.text for row in rows))

    def test_not_wanted(self):
        parser = KiroParser()
        for rel in (".kiro/settings/cli.json", ".kiro/sessions/s1.json"):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertEqual(art.agent, "kiro")
            self.assertFalse(parser.wants(art), rel)

    def test_truncated_export_is_reported_not_fatal(self):
        rows = self.rows_for(KiroParser(), ".aws/amazonq/exports/issue-chat-cut.json")
        self.assertEqual(
            [row.turn_type for row in rows if not model_or_permission_row(row)],
            ["user", "tool_use", "tool_result", "assistant", "tool_use", "system"],
        )
        self.assertEqual(rows[0].session_id, KIRO_EXPORT_SESSION)
        self.assertIn("2 history entries recovered", rows[-1].text)

    def test_salvage(self):
        self.assertEqual(salvage('{"a": 1}'), ({"a": 1}, ""))
        state, error = salvage('{"conversation_id": "c\\"1", "history": [{"x": 1}, {"y": ')
        self.assertEqual(state, {"conversation_id": 'c"1', "history": [{"x": 1}]})
        self.assertTrue(error)
        self.assertEqual(salvage("garbage")[0], None)

    # -- source-derived rows ---------------------------------------------------
    DENIALS = ".aws/amazonq/exports/denials.json"
    HISTORY = ".aws/amazonq/.cli_bash_history"
    AGENT = ".aws/amazonq/cli-agents/dev.json"

    def test_permission_decisions(self):
        rows = self.rows_for(KiroParser(), self.DENIALS)
        main = [
            row for row in rows if row.recovered != "yes" and row.text.startswith("permission ")
        ]
        self.assertEqual(
            [(row.text, row.tool_use_id, row.tool_name, row.timestamp_utc) for row in main],
            [
                (
                    "permission decision: denied by=n",
                    "tooluse_d1",
                    "execute_bash",
                    "2026-04-25T16:00:05.000Z",
                ),
                (
                    "permission decision: denied by=deny-rule tool=execute_bash",
                    "tooluse_d2",
                    "execute_bash",
                    "2026-04-25T16:00:10.000Z",
                ),
                (
                    "permission decision: cancelled by=interrupt",
                    "tooluse_d3",
                    "execute_bash",
                    "2026-04-25T16:00:15.000Z",
                ),
                # the deny-rule text as a plain prompt names the tool, not a call
                (
                    "permission decision: denied by=deny-rule tool=execute_bash",
                    "",
                    "execute_bash",
                    "2026-04-25T16:00:20.000Z",
                ),
            ],
        )
        for row in main:
            self.assertEqual(
                (row.turn_type, row.session_id, row.project_path),
                ("system", KIRO_DENIAL_SESSION, "/srv/proj"),
            )
        # each decision follows the cancelled result it governs
        for i, row in enumerate(rows):
            if row.tool_use_id and row.text.startswith("permission decision:"):
                self.assertEqual(
                    (rows[i - 1].turn_type, rows[i - 1].tool_use_id),
                    ("tool_result", row.tool_use_id),
                )
        # the user's own words at the approval prompt
        rows = self.rows_for(KiroParser(), self.EXPORT)
        self.assertEqual(
            [(row.text, row.tool_use_id) for row in rows if row.text.startswith("permission ")],
            [("permission decision: denied by=user-text", "tooluse_bash2")],
        )

    def test_history_copies_recovered(self):
        rows = self.rows_for(KiroParser(), self.DENIALS)
        markers = [
            row for row in rows if row.text.startswith(("checkpoint snapshot:", "tangent mode:"))
        ]
        self.assertEqual(
            [(row.text, row.timestamp_utc) for row in markers],
            [
                (
                    "checkpoint snapshot: tag=1 recovered=1 entries: the turn a restore discarded",
                    "2026-04-25T15:59:30.000Z",
                ),
                # inherits the previous row's time: tangent_start_time is not read
                (
                    "tangent mode: main conversation held aside, recovered=1 entries",
                    "2026-04-25T15:59:00.500Z",
                ),
            ],
        )
        recovered = [row for row in rows if row.recovered == "yes"]
        self.assertEqual(
            [(row.turn_type, row.text) for row in recovered],
            [
                ("user", "the turn a restore discarded"),
                ("assistant", "Discarded reply."),
                ("user", "the main conversation"),
                ("assistant", "Main reply."),
            ],
        )
        self.assertTrue(all(row.session_id == KIRO_DENIAL_SESSION for row in recovered))
        # the snapshot entry the history still holds is not repeated
        self.assertEqual(len([row for row in rows if row.text == "clean the build"]), 1)
        # recovered entries keep their own model sequence; system rows are not marked
        self.assertEqual(
            [row.text for row in rows if row.text.startswith("model: ")],
            ["model: claude-sonnet-4", "model: m-old", "model: m-old -> claude-sonnet-4"],
        )
        self.assertFalse([row for row in rows if row.turn_type == "system" and row.recovered])

    def test_prompt_history(self):
        parser = KiroParser()
        art = self.artifact(self.HISTORY)
        self.assertTrue(parser.wants(art))
        self.assertTrue(parser.secondary(art))
        self.assertTrue(parser.has_secondary)
        self.assertFalse(parser.secondary(self.artifact(self.DATABASE)))
        rows = list(parser.parse(art, Options()))
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                (2, "user", "list the files here"),
                (3, "user", "/clear"),
                (4, "user", "fix the\nbuild"),
                (5, "user", "path C:\\tmp"),
                (7, "user", "after the cut"),
                # the line that is not UTF-8, in full, after the lines around it
                (6, "system", "parser: unparseable line (not valid UTF-8): \ufffd\ufffd not utf-8"),
            ],
        )
        for row in rows:
            self.assertEqual((row.timestamp_utc, row.session_id, row.project_path), ("", "", ""))
        # a history line the database also holds is dropped as its copy
        primary = self.rows_for(KiroParser(), self.DATABASE)
        kept, dropped = deduplicate(primary, [(row, 2.0) for row in rows])
        self.assertEqual(dropped["kiro"], 1)
        self.assertEqual(
            [row.text for row in kept if row.recovered == "yes"],
            ["/clear", "fix the\nbuild", "path C:\\tmp", "after the cut"],
        )

    def test_unescape_history(self):
        self.assertEqual(kiro_unescape("a\\nb\\\\c"), "a\nb\\c")
        # an escape rustyline never writes keeps the line as written
        self.assertEqual(kiro_unescape("C:\\tmp\\nx"), "C:\\tmp\\nx")
        self.assertEqual(kiro_unescape("trailing\\"), "trailing\\")

    def test_agent_config_settings(self):
        parser = KiroParser()
        self.assertTrue(parser.wants(self.artifact(self.AGENT)))
        rows = self.rows_for(parser, self.AGENT)
        source = "(from .aws/amazonq/cli-agents/dev.json, file mtime)"
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: allowedTools=fs_read " + source,
                "permission setting: allowedTools=@git " + source,
                'permission setting: toolsSettings.execute_bash.allowedCommands=["git status"] '
                + source,
                'permission setting: toolsSettings.execute_bash.deniedCommands=["rm -rf .*"] '
                + source,
                "permission setting: toolsSettings.execute_bash.autoAllowReadonly=true " + source,
                'permission setting: toolsSettings.fs_write.deniedPaths=["/etc"] ' + source,
            ],
        )
        mtime = to_utc((self.home / self.AGENT).stat().st_mtime)
        for row in rows:
            self.assertEqual(
                (row.turn_type, row.timestamp_utc, row.session_id, row.project_path),
                ("system", mtime, "", ""),
            )
        bad = self.rows_for(parser, ".aws/amazonq/cli-agents/broken.json")
        self.assertEqual(
            [(row.turn_type, row.text) for row in bad],
            [("system", "parser: agent configuration is not a JSON object")],
        )

    def _project_artifact(self, rel, record):
        project = self.tmp / "srv" / "proj"
        path = project / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(record), encoding="utf-8")
        home = Home(disk_path=project, original="/srv/proj", user="alice", host="h1", project=True)
        return Artifact(path, "/srv/proj/" + rel, rel, "kiro", home)

    def test_project_agent_config(self):
        rel = ".amazonq/cli-agents/proj.json"
        art = self._project_artifact(rel, {"name": "proj", "allowedTools": ["execute_bash"]})
        self.assertTrue(KiroParser().wants(art))
        rows = list(KiroParser().parse(art, Options()))
        self.assertEqual(
            [(row.text, row.project_path) for row in rows],
            [
                (
                    "permission setting: allowedTools=execute_bash (from %s, file mtime)" % rel,
                    "/srv/proj",
                )
            ],
        )

    def test_delegate_records(self):
        records = kiro_delegate_records()
        done = self._project_artifact(
            ".amazonq/.subagents/q_cli_default.json", records["q_cli_default.json"]
        )
        running = self._project_artifact(
            ".amazonq/.subagents/reviewer.json", records["reviewer.json"]
        )
        parser = KiroParser()
        self.assertTrue(parser.wants(done) and parser.wants(running))
        self.assertFalse(parser.secondary(done))
        self.assertEqual(
            [
                (row.turn_type, row.timestamp_utc, row.project_path, row.text)
                for row in parser.parse(done, Options())
            ],
            [
                (
                    "system",
                    "2026-04-25T16:01:40.000Z",
                    "/srv/proj",
                    "delegate task launched: agent=q_cli_default pid=4711 trust_all_tools=true: "
                    "run the tests",
                ),
                (
                    "system",
                    "2026-04-25T16:02:40.000Z",
                    "/srv/proj",
                    "delegate task completed: agent=q_cli_default exit_code=0\n"
                    "summary: All tests pass.\noutput: 12 passed",
                ),
            ],
        )
        # still running: the launch only; the `Unknown` cwd falls back to the project
        self.assertEqual(
            [(row.text, row.project_path) for row in parser.parse(running, Options())],
            [("delegate task launched: agent=reviewer pid=4712: review the diff", "/srv/proj")],
        )

    def test_new_files_not_wanted(self):
        parser = KiroParser()
        for rel in (".aws/amazonq/cli-agents/sub/x.json", ".amazonq/.subagents/notes.txt"):
            art = self._project_artifact(rel, {"name": "x"})
            self.assertFalse(parser.wants(art), rel)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_file_is_one_row(self):
        unlocked = parse_with(self, KiroParser)
        for rel in (
            ".aws/amazonq/.cli_bash_history",
            ".aws/amazonq/cli-agents/dev.json",
            self.EXPORT,
        ):
            rows = parse_with(self, KiroParser, rel)
            self.assertEqual(
                [
                    (row.source_file.endswith(rel), row.source_line, row.timestamp_utc, row.text)
                    for row in unreadable_rows(rows)
                ],
                [
                    (
                        True,
                        0,
                        "",
                        "parser: %s could not be read (Permission denied)" % rel.split("/")[-1],
                    )
                ],
                rel,
            )
            # Every other file of the parser still gives its rows.
            self.assertEqual(kept_rows(rows), kept_rows(unlocked, rel), rel)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_export_unreadable_before_selection_is_one_row(self):
        """Offer an export candidate the sniff cannot read, so it gives exactly one row."""
        unlocked = parse_with(self, KiroParser)
        path = self.home / self.EXPORT
        path.chmod(0)
        try:
            rows = parse_with(self, KiroParser)
        finally:
            path.chmod(0o600)
        own = [row for row in rows if row.source_file.endswith(self.EXPORT)]
        self.assertEqual(
            [(row.turn_type, row.source_line, row.timestamp_utc, row.text) for row in own],
            [("system", 0, "", "parser: %s could not be read (Permission denied)" % path.name)],
        )
        self.assertEqual(kept_rows(rows, self.EXPORT), kept_rows(unlocked, self.EXPORT))

    def test_readable_non_export_json_is_not_wanted(self):
        rel = ".aws/amazonq/exports/notes.json"
        (self.home / rel).write_text(json.dumps({"name": "x"}), encoding="utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        self.assertFalse(KiroParser().wants(self.artifact(rel)))
        self.assertTrue(KiroParser().wants(self.artifact(self.EXPORT)))

    def test_damaged_database_is_one_row(self):
        database = self.home / self.DATABASE
        for suffix in ("-wal", "-shm"):
            database.with_name(database.name + suffix).unlink(missing_ok=True)
        unlocked = parse_with(self, KiroParser)
        database.write_bytes(b"SQLite format 3\x00" + b"\xff" * 200)
        rows = parse_with(self, KiroParser)
        own = [row for row in rows if row.source_file.endswith(self.DATABASE)]
        self.assertEqual([(row.turn_type, row.source_line) for row in own], [("system", 0)])
        self.assertTrue(own[0].text.startswith("parser: SQLite error: "), own[0].text)
        self.assertEqual(kept_rows(rows, self.DATABASE), kept_rows(unlocked, self.DATABASE))
