"""Tests of the PearAI parser."""

from doubleagent.dedup import deduplicate
from doubleagent.parsers.pearai import PearAiParser
from fixtures import (
    BAD_LINE,
    PEARAI_GS,
    PEARAI_LOG_REL,
    PEARAI_ROO,
    PEARAI_SEARCH_SESSION,
    PEARAI_SESSION,
    PEARAI_TASK,
    PEARAI_UI_TASK,
    _vscdb,
    bad_line_text,
    pearai_log_completion,
    pearai_log_settings,
    write_bad_line,
)

from .base import ParserBase


class PearAiTests(ParserBase):
    """Test the PearAI parser."""

    def test_settings_header_splits_at_lf_only(self):
        path = self.home / PEARAI_LOG_REL
        model = "m1%sx" % chr(0x2028)
        path.write_text(pearai_log_settings(model, "<user>\nfirst\n\n"), encoding="utf-8")
        rows = self.rows_for(PearAiParser(), PEARAI_LOG_REL)
        self.assertEqual(rows[0].text, "model: " + model)
        self.assertEqual(rows[0].model, model)

    SESSION = ".pearai/sessions/%s.json" % PEARAI_SESSION
    API = PEARAI_ROO + "tasks/%s/api_conversation_history.json" % PEARAI_TASK
    UI_API = PEARAI_ROO + "tasks/%s/api_conversation_history.json" % PEARAI_UI_TASK
    STATE = PEARAI_GS + "state.vscdb"
    DEV_CHAT = ".pearai/dev_data/chat.jsonl"
    TOKENS = ".pearai/dev_data/tokens_generated.jsonl"

    def test_rows(self):
        rows = self.rows_for(PearAiParser(), self.SESSION)
        self.assertEqual(
            [(row.turn_type, row.source_line) for row in rows],
            [
                ("system", 0),
                ("user", 1),
                ("system", 2),
                ("assistant", 2),
                ("user", 3),
                ("system", 4),
                ("assistant", 4),
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.agent, row.session_id, row.project_path, row.timestamp_utc),
                ("pearai", PEARAI_SESSION, "/srv/proj", "2026-10-03T13:00:00.000Z"),
            )
        self.assertEqual(
            rows[0].text,
            "session start: why does build.sh fail | integration=continue | "
            "history=4 perplexityHistory=0",
        )
        self.assertEqual(rows[1].text, "why does build.sh fail\n[context: /srv/proj/build.sh]")
        self.assertEqual(
            (rows[3].text, rows[3].model), ("The make target is missing.", "pearai_model")
        )
        # promptLogs[].completionOptions.model: the first one starts, a new one switches
        self.assertEqual(
            [(row.text, row.model) for row in (rows[2], rows[5])],
            [
                ("model: pearai_model", "pearai_model"),
                ("model: pearai_model -> pearai_model_2", "pearai_model_2"),
            ],
        )
        rows = self.rows_for(PearAiParser(), ".pearai/sessions/%s.json" % PEARAI_SEARCH_SESSION)
        self.assertEqual(
            [(row.turn_type, row.source_line, row.text) for row in rows][1:],
            [
                ("user", 1, "latest make release"),
                (
                    "assistant",
                    2,
                    "GNU make 4.4.1.\n[citations: https://www.gnu.org/software/make/]",
                ),
            ],
        )
        self.assertIn("integration=perplexity", rows[0].text)

    def test_task_xml_tool_calls(self):
        rows = self.rows_for(PearAiParser(), self.API)[:6]
        self.assertEqual(
            [row.turn_type for row in rows],
            ["user", "system", "assistant", "tool_use", "tool_result", "tool_use"],
        )
        self.assertEqual(
            {(row.agent, row.session_id, row.project_path, row.model) for row in rows},
            {("pearai", PEARAI_TASK, "/srv/proj", "")},
        )  # project from state.vscdb taskHistory
        user, env, asst, use, result, done = rows
        self.assertEqual(
            (user.text, user.timestamp_utc), ("run the tests", "2026-10-03T14:00:00.000Z")
        )
        self.assertTrue(env.text.startswith("<environment_details>"))
        self.assertEqual(asst.text, "I will run them.")
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text, use.source_line),
            ("execute_command", "2", "npm test", 2),
        )
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.text, result.timestamp_utc),
            ("execute_command", "2", "12 passing", "2026-10-03T14:00:09.000Z"),
        )
        self.assertEqual(
            (done.tool_name, done.tool_use_id, done.text),
            ("attempt_completion", "4", "All 12 tests pass."),
        )

    def test_permission_decisions(self):
        # Roo 3.15.3 result texts: each decision follows its tool_result on
        # the call's tool_use_id (the assistant message's position).
        rows = self.rows_for(PearAiParser(), self.API)[6:]
        self.assertEqual(
            [
                (row.turn_type, row.tool_name, row.tool_use_id, row.text.split("\n")[0])
                for row in rows
            ],
            [
                ("tool_use", "write_to_file", "5", "build/out.txt | x"),
                (
                    "tool_result",
                    "write_to_file",
                    "5",
                    "The user denied this operation and provided the following feedback:",
                ),
                ("system", "write_to_file", "5", "permission decision: denied with feedback"),
                ("tool_use", "execute_command", "7", "rm -rf build"),
                ("tool_result", "execute_command", "7", "The user denied this operation."),
                ("system", "execute_command", "7", "permission decision: denied"),
                ("tool_use", "execute_command", "9", "make all"),
                (
                    "tool_result",
                    "execute_command",
                    "9",
                    "The user approved this operation and provided the following context:",
                ),
                ("system", "execute_command", "9", "permission decision: approved with feedback"),
                ("tool_result", "execute_command", "9", "built"),
            ],
        )
        self.assertEqual(rows[2].timestamp_utc, "2026-10-03T14:00:22.000Z")
        self.assertEqual({row.session_id for row in rows}, {PEARAI_TASK})

    def test_permission_decisions_beside_ui_messages(self):
        # ui_messages.json holds the transcript; the API history still gives
        # the decisions, joined to the ts of the ui ask before each.
        rows = self.rows_for(PearAiParser(), self.UI_API)
        base_time = int(PEARAI_UI_TASK) + 20000
        self.assertEqual(
            [
                (row.turn_type, row.tool_name, row.tool_use_id, row.text, row.source_line)
                for row in rows
            ],
            [
                (
                    "system",
                    "write_to_file",
                    str(base_time + 1000),
                    "permission decision: denied with feedback",
                    6,
                ),
                (
                    "system",
                    "execute_command",
                    str(base_time + 4000),
                    "permission decision: denied",
                    8,
                ),
                (
                    "system",
                    "execute_command",
                    str(base_time + 7000),
                    "permission decision: approved with feedback",
                    10,
                ),
            ],
        )
        self.assertEqual(rows[0].timestamp_utc, "2026-10-03T15:00:22.000Z")
        ui_rows = self.rows_for(
            PearAiParser(), PEARAI_ROO + "tasks/%s/ui_messages.json" % PEARAI_UI_TASK
        )
        self.assertIn(
            str(base_time + 4000),
            {row.tool_use_id for row in ui_rows if row.turn_type == "tool_use"},
        )

    def test_state_task_history_and_settings(self):
        rows = self.rows_for(PearAiParser(), self.STATE)
        tasks = [row for row in rows if row.text.startswith("task: ")]
        self.assertEqual(
            [
                (row.session_id, row.project_path, row.timestamp_utc, row.source_line)
                for row in tasks
            ],
            [
                (PEARAI_TASK, "/srv/proj", "2026-10-03T14:00:10.000Z", 1),
                ("1791037000000", "/srv/proj", "2026-10-03T14:16:40.000Z", 2),
            ],
        )
        self.assertEqual(
            tasks[1].text,
            "task: make a todo app | tokensIn=300 tokensOut=20 totalCost=0.001 number=2"
            " | creator mode: newProjectType=webapp newProjectPath=/srv/proj",
        )
        settings = [row for row in rows if row.text.startswith("permission setting:")]
        self.assertEqual(
            [row.text for row in settings],
            [
                "permission setting: autoApprovalEnabled=true "
                "(from state.vscdb pearai.pearai-roo-cline, file mtime)",
                "permission setting: alwaysAllowReadOnly=true "
                "(from state.vscdb pearai.pearai-roo-cline, file mtime)",
                "permission setting: alwaysAllowExecute=false "
                "(from state.vscdb pearai.pearai-roo-cline, file mtime)",
                'permission setting: allowedCommands=["npm test","npm run lint"] '
                "(from state.vscdb pearai.pearai-roo-cline, file mtime)",
            ],
        )
        self.assertEqual({(row.session_id, row.turn_type) for row in settings}, {("", "system")})
        self.assertTrue(all(row.timestamp_utc for row in settings))
        self.assertEqual(len(rows), len(tasks) + len(settings))

    def test_state_without_the_extension_row(self):
        other = ".config/PearAI/User/globalStorage/state.vscdb"
        (self.home / other).unlink()
        _vscdb(self.home / other, {"some.other-extension": {"x": 1}})
        self.assertEqual(self.rows_for(PearAiParser(), other), [])

    def test_output_log_secondary(self):
        parser = PearAiParser()
        art = self.artifact(PEARAI_LOG_REL)
        self.assertTrue(parser.wants(art))
        self.assertTrue(parser.secondary(art))
        self.assertTrue(parser.has_secondary)
        rows = self.rows_for(parser, PEARAI_LOG_REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text, row.model) for row in rows],
            [
                (3, "system", "model: pearai_model_2", "pearai_model_2"),
                (3, "user", "add the target", "pearai_model_2"),
                (24, "assistant", "Added `all:` to the Makefile.", "pearai_model_2"),
                (30, "user", "delete the old logs", "pearai_model_2"),
                (42, "assistant", "Removed logs/.", "pearai_model_2"),
                (48, "system", "model: pearai_model_2 -> starcoder2:3b", "starcoder2:3b"),
                (48, "system", "model request: def add(a, b):\n<FIM>", "starcoder2:3b"),
                (59, "system", "model completion: return a + b", "starcoder2:3b"),
            ],
        )
        self.assertEqual(
            {(row.timestamp_utc, row.session_id, row.project_path) for row in rows}, {("", "", "")}
        )

    def test_output_log_pairs_with_the_session(self):
        # The logged turn of the fixture session is dropped against the
        # session's rows; the turn whose session is gone is kept, recovered.
        parser = PearAiParser()
        primary = self.rows_for(parser, self.SESSION)
        secondary = [(row, 2.0) for row in self.rows_for(parser, PEARAI_LOG_REL)]
        kept, dropped = deduplicate(primary, secondary)
        texts = [
            (row.turn_type, row.text, row.recovered) for row in kept if row.turn_type != "system"
        ]
        self.assertIn(("user", "delete the old logs", "yes"), texts)
        self.assertIn(("assistant", "Removed logs/.", "yes"), texts)
        self.assertEqual(
            [text for text in texts if text[1] == "add the target"],
            [("user", "add the target", "")],
        )
        self.assertEqual(dropped["pearai"], 2)

    def test_dev_data_chat_secondary(self):
        parser = PearAiParser()
        self.assertTrue(parser.secondary(self.artifact(self.DEV_CHAT)))
        rows = self.rows_for(parser, self.DEV_CHAT)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text, row.model, row.session_id) for row in rows],
            [
                (1, "user", "add the target", "pearai_model_2", PEARAI_SESSION),
                (1, "assistant", "Added `all:` to the Makefile.", "pearai_model_2", PEARAI_SESSION),
                (
                    1,
                    "system",
                    "user feedback: true (from dev_data/chat.jsonl)",
                    "pearai_model_2",
                    PEARAI_SESSION,
                ),
                (
                    2,
                    "user",
                    "what broke the deploy",
                    "pearai_model",
                    "9b1c2d3e-0000-4000-8000-0000000000ff",
                ),
                (
                    2,
                    "assistant",
                    "The DNS record.",
                    "pearai_model",
                    "9b1c2d3e-0000-4000-8000-0000000000ff",
                ),
                (
                    2,
                    "system",
                    "user feedback: false (from dev_data/chat.jsonl)",
                    "pearai_model",
                    "9b1c2d3e-0000-4000-8000-0000000000ff",
                ),
            ],
        )
        # the session's project from sessions.json; no time of its own
        self.assertEqual([row.project_path for row in rows], ["/srv/proj"] * 3 + [""] * 3)
        self.assertEqual({row.timestamp_utc for row in rows}, {""})

    def test_tokens_generated(self):
        parser = PearAiParser()
        self.assertFalse(parser.secondary(self.artifact(self.TOKENS)))
        rows = self.rows_for(parser, self.TOKENS)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text, row.model) for row in rows],
            [
                (
                    1,
                    "system",
                    "tokens generated: model=pearai_model provider=pearai_server "
                    "promptTokens=120 generatedTokens=9",
                    "pearai_model",
                ),
                (
                    2,
                    "system",
                    "tokens generated: model=pearai_model_2 provider=pearai_server "
                    "promptTokens=180 generatedTokens=11",
                    "pearai_model_2",
                ),
            ],
        )
        self.assertEqual({(row.timestamp_utc, row.session_id) for row in rows}, {("", "")})

    def test_thinking_opt_in(self):
        self.assertNotIn(
            "thinking", [row.turn_type for row in self.rows_for(PearAiParser(), self.API)]
        )
        rows = self.rows_for(PearAiParser(), self.API, include_thinking=True)
        self.assertEqual([row.text for row in rows if row.turn_type == "thinking"], ["use npm"])

    def test_ui_messages_preferred(self):
        task = PEARAI_ROO + "tasks/%s/" % PEARAI_UI_TASK
        # only the permission decisions come from the API history then
        self.assertEqual(
            {
                row.text.split(":")[0]
                for row in self.rows_for(PearAiParser(), task + "api_conversation_history.json")
            },
            {"permission decision"},
        )
        rows = self.rows_for(PearAiParser(), task + "ui_messages.json")
        self.assertEqual(rows[0].turn_type, "user")
        self.assertEqual({row.session_id for row in rows}, {PEARAI_UI_TASK})
        self.assertIn("tool_result", [row.turn_type for row in rows])

    def test_not_wanted(self):
        for rel in (
            ".pearai/config.json",
            ".pearai/dev_data/autocomplete.jsonl",
            PEARAI_LOG_REL.replace("1-PearAI.log", "2-Git.log"),
        ):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertEqual(art.agent, "pearai")
            self.assertFalse(PearAiParser().wants(art), rel)
        # the index is wanted only for the rows that say what it lost
        self.assertEqual(self.rows_for(PearAiParser(), ".pearai/sessions/sessions.json"), [])

    def test_truncated_line_is_reported_not_fatal(self):
        write_bad_line(self.home / self.SESSION)
        rows = self.rows_for(PearAiParser(), self.SESSION)
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "user", "system", "assistant", "user", "system", "assistant", "system"],
        )
        self.assertIn("parser:", rows[-1].text)
        write_bad_line(self.home / self.API)
        rows = self.rows_for(PearAiParser(), self.API)
        self.assertEqual(len(rows), 17)
        self.assertEqual(rows[-1].turn_type, "system")
        self.assertIn("parser:", rows[-1].text)
        # beside ui_messages.json the API history is read for decisions only,
        # and still reports what it lost
        write_bad_line(self.home / self.UI_API)
        rows = self.rows_for(PearAiParser(), self.UI_API)
        self.assertEqual(len(rows), 4)
        self.assertTrue(rows[-1].text.startswith("parser: unparseable data after"), rows[-1].text)

    def _jsonl_with_bad_line(self, rel):
        path = self.home / rel
        lines = path.read_text(encoding="utf-8").splitlines()
        path.write_text("%s\n%s\n%s\n" % (lines[0], BAD_LINE, lines[1]), encoding="utf-8")

    def test_truncated_dev_data_lines(self):
        for rel, per_line in ((self.DEV_CHAT, 3), (self.TOKENS, 1)):
            self._jsonl_with_bad_line(rel)
            rows = self.rows_for(PearAiParser(), rel)
            self.assertEqual(len(rows), 2 * per_line + 1, rel)
            bad = rows[-1]
            self.assertEqual((bad.turn_type, bad.text), ("system", bad_line_text()))
            self.assertTrue(any(row.source_line == 3 for row in rows[:-1]), rel)

    def test_damaged_output_log_record(self):
        # a write that is neither header, between two good ones, and a
        # Settings write cut off before its rule at the end of the file
        path = self.home / PEARAI_LOG_REL
        path.write_text(
            pearai_log_settings("m1", "<user>\nfirst\n\n")
            + ("=" * 74 + "\n") * 2
            + "garbage line\n"
            + pearai_log_completion("second")
            + ("=" * 74 + "\n") * 2
            + "Settings:\ncontextLength: 8192\nmod",
            encoding="utf-8",
        )
        rows = self.rows_for(PearAiParser(), PEARAI_LOG_REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                (3, "system", "model: m1"),
                (3, "user", "first"),
                (18, "assistant", "second"),
                (
                    15,
                    "system",
                    "parser: unparseable record (not a Settings: or Completion: write): "
                    "garbage line",
                ),
                (
                    24,
                    "system",
                    "parser: unparseable record (no prompt rule): Settings:\ncontextLength: 8192\nmod",
                ),
            ],
        )

    def test_index_cut_mid_write_is_reported(self):
        # Through the Continue parser's index reader: the entry before the
        # cut still times its session, the cut one is lost, and the index
        # yields one pearai row carrying the broken entry to the end.
        from fixtures import continue_index_cut, pearai_index

        index = ".pearai/sessions/sessions.json"
        text, tail = continue_index_cut(pearai_index()[:1])
        (self.home / index).write_text(text, encoding="utf-8")
        parser = PearAiParser()
        search = ".pearai/sessions/%s.json" % PEARAI_SEARCH_SESSION
        self.assertEqual(
            {row.timestamp_utc for row in self.rows_for(parser, self.SESSION)},
            {"2026-10-03T13:00:00.000Z"},
        )
        self.assertEqual({row.timestamp_utc for row in self.rows_for(parser, search)}, {""})
        rows = self.rows_for(parser, index)
        self.assertEqual(
            [
                (row.agent, row.turn_type, row.source_line, row.session_id, row.timestamp_utc)
                for row in rows
            ],
            [("pearai", "system", 2, "", "")],
        )
        self.assertTrue(
            rows[0].text.startswith("parser: unparseable record 2 (Unterminated string")
        )
        self.assertTrue(rows[0].text.endswith("): " + tail), rows[0].text)

    def test_index_not_an_array_is_reported(self):
        index = ".pearai/sessions/sessions.json"
        (self.home / index).write_text('{"sessions": []}', encoding="utf-8")
        parser = PearAiParser()
        self.assertEqual({row.timestamp_utc for row in self.rows_for(parser, self.SESSION)}, {""})
        self.assertEqual(
            [(row.agent, row.source_line, row.text) for row in self.rows_for(parser, index)],
            [("pearai", 1, "parser: not a JSON array")],
        )
