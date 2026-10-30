"""Tests of the OpenHands parser."""

import json

from doubleagent.parsers import Options
from doubleagent.parsers.openhands import OpenHandsParser
from fixtures import (
    OPENHANDS_CLI_CONV,
    OPENHANDS_CONV,
    OPENHANDS_LEGACY,
    OPENHANDS_PROJECT_HASH,
    OPENHANDS_WT_CONV,
    OPENHANDS_WT_DIR,
    write_bad_line,
)

from .base import ParserBase


class OpenHandsTests(ParserBase):
    """Test the OpenHands parser."""

    CANVAS = ".openhands/agent-canvas/dev_conversations/%s/events/" % OPENHANDS_CONV
    CLI = ".openhands/conversations/%s/events/" % OPENHANDS_CLI_CONV

    def event(self, base, number):
        return next(
            artifact.rel
            for artifact in self.col.artifacts
            if artifact.rel.startswith(base + "event-%05d-" % number)
        )

    def conversation(self, base, **options):
        rows = []
        for artifact in sorted(self.col.artifacts, key=lambda artifact: artifact.rel):
            if artifact.rel.startswith(base) and OpenHandsParser().wants(artifact):
                rows.extend(OpenHandsParser().parse(artifact, Options(**options)))
        return rows

    def test_rows(self):
        rows = self.conversation(self.CANVAS)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "system",
                "system",
                "user",
                "tool_use",
                "tool_result",
                "tool_use",
                "tool_result",
                "system",
                "assistant",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "openhands", OPENHANDS_CONV),
            )
            self.assertEqual(
                (row.project_path, row.git_branch, row.model),
                ("/srv/proj", "", "litellm_proxy/claude-sonnet-4-5"),
            )
            self.assertEqual(row.source_line, 1)
        header, model, prompt, user, listing, out, view, reject, decision, asst = rows
        self.assertTrue(header.text.startswith("conversation start | List files"), header.text)
        self.assertEqual(
            (model.text, model.timestamp_utc),
            ("model: litellm_proxy/claude-sonnet-4-5", "2026-10-01T10:00:00.000Z"),
        )
        self.assertIn("corrected by UTC+02:00", header.text)
        self.assertEqual(header.timestamp_utc, "2026-10-01T10:00:00.000Z")
        self.assertEqual(prompt.text, "system prompt, 2 tools: You are OpenHands agent.")
        # Naive local 12:00:01 anchored by meta.json created_at 10:00:00Z.
        self.assertEqual(
            (user.text, user.timestamp_utc), ("list the files", "2026-10-01T10:00:01.000Z")
        )
        self.assertEqual(
            (listing.tool_name, listing.tool_use_id, listing.text),
            ("terminal", "call_1", "ls security_risk=LOW"),
        )
        self.assertEqual(
            (out.tool_use_id, out.text, out.timestamp_utc),
            ("call_1", "README.md", "2026-10-01T10:00:04.000Z"),
        )
        self.assertEqual((view.tool_name, view.text), ("file_editor", "view /srv/proj/README.md"))
        self.assertEqual(
            (reject.tool_use_id, reject.text), ("call_2", "[rejected by user] not that file")
        )
        self.assertEqual(
            (decision.tool_name, decision.tool_use_id, decision.text, decision.timestamp_utc),
            (
                "file_editor",
                "call_2",
                "permission decision: rejected rejection_source=user not that file",
                reject.timestamp_utc,
            ),
        )
        self.assertEqual(rows.index(decision), rows.index(reject) + 1)
        self.assertEqual(asst.text, "The project has a README.")
        self.assertTrue(asst.source_file.endswith(".json"))

    def test_no_meta_emits_naive_time_as_utc(self):
        rows = self.conversation(self.CLI)
        self.assertEqual(rows[0].turn_type, "system")
        self.assertIn("emitted as if UTC", rows[0].text)
        self.assertEqual(rows[1].text, "model: litellm_proxy/claude-sonnet-4-5")
        self.assertEqual(rows[3].timestamp_utc, "2026-10-02T08:30:01.000Z")
        self.assertEqual({row.session_id for row in rows}, {OPENHANDS_CLI_CONV})

    def test_thinking_opt_in(self):
        self.assertNotIn("thinking", [row.turn_type for row in self.conversation(self.CANVAS)])
        rows = self.conversation(self.CANVAS, include_thinking=True)
        self.assertEqual(
            [row.text for row in rows if row.turn_type == "thinking"],
            ["I will run ls.", "check the readme"],
        )

    def test_legacy_session_reported_once(self):
        rows = [
            row
            for artifact in self.col.artifacts
            if OpenHandsParser().wants(artifact) and "/sessions/" in artifact.rel
            for row in OpenHandsParser().parse(artifact, Options())
        ]
        self.assertEqual(len(rows), 1)
        self.assertEqual((rows[0].session_id, rows[0].turn_type), (OPENHANDS_LEGACY, "system"))
        self.assertIn("3 event files, not parsed", rows[0].text)

    def test_secrets_not_wanted(self):
        for rel in (
            ".openhands/settings.json",
            ".openhands/secrets.json",
            ".openhands/agent-canvas/dev_conversations/%s/meta.json" % OPENHANDS_CONV,
        ):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertFalse(OpenHandsParser().wants(art), rel)

    def test_truncated_line_is_reported_not_fatal(self):
        rel = self.event(self.CANVAS, 6)
        write_bad_line(self.home / rel)
        rows = self.rows_for(OpenHandsParser(), rel)
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0].turn_type, "system")
        self.assertTrue(
            rows[0].text.startswith("parser: event file could not be decoded ("), rows[0].text
        )
        self.assertEqual(len(self.conversation(self.CANVAS)), 10)  # the other events still parse

    def _cut(self, rel):
        path = self.home / rel
        raw = path.read_text(encoding="utf-8")
        path.write_text(raw[: len(raw) // 2], encoding="utf-8")
        try:
            json.loads(raw[: len(raw) // 2])
        except ValueError as error:
            return str(error)
        self.fail("cut file still decodes")

    def test_undecodable_meta_json_is_reported(self):
        reason = self._cut(
            ".openhands/agent-canvas/dev_conversations/%s/meta.json" % OPENHANDS_CONV
        )
        rows = self.conversation(self.CANVAS)
        header, model, problem = rows[:3]
        self.assertTrue(header.text.startswith("conversation start | model="), header.text)
        self.assertEqual(model.text, "model: litellm_proxy/claude-sonnet-4-5")
        self.assertEqual(
            (problem.turn_type, problem.text, problem.timestamp_utc, problem.source_line),
            ("system", "parser: meta.json could not be decoded (%s)" % reason, "", 1),
        )
        # The session id and the project from base_state.json still apply.
        self.assertEqual((problem.session_id, problem.project_path), (OPENHANDS_CONV, "/srv/proj"))
        self.assertEqual(len(rows), 11)  # every event still yields its rows
        self.assertEqual(rows[3].text, "system prompt, 2 tools: You are OpenHands agent.")
        self.assertEqual(rows[-1].text, "The project has a README.")
        self.assertEqual([row for row in rows if row.text.startswith("parser:")], [problem])

    def test_undecodable_base_state_json_is_reported(self):
        reason = self._cut(".openhands/conversations/%s/base_state.json" % OPENHANDS_CLI_CONV)
        rows = self.conversation(self.CLI)
        header, problem = rows[:2]
        self.assertIn("emitted as if UTC", header.text)
        self.assertEqual(
            (problem.turn_type, problem.text, problem.timestamp_utc),
            ("system", "parser: base_state.json could not be decoded (%s)" % reason, ""),
        )
        # No meta.json either: nothing records the project or the model.
        self.assertEqual(
            (problem.session_id, problem.project_path, problem.model),
            (OPENHANDS_CLI_CONV, "", ""),
        )
        self.assertFalse([row for row in rows if row.text.startswith("model: ")])
        self.assertEqual(rows[2].text, "system prompt, 2 tools: You are OpenHands agent.")
        self.assertEqual(len(rows), 6)

    def test_sibling_file_not_an_object_is_reported(self):
        path = self.home / (
            ".openhands/agent-canvas/dev_conversations/%s/base_state.json" % OPENHANDS_CONV
        )
        path.write_text("[]", encoding="utf-8")
        rows = self.conversation(self.CANVAS)
        self.assertEqual(rows[1].text, "parser: base_state.json is not a JSON object")
        self.assertEqual(
            (rows[1].session_id, rows[1].project_path, rows[1].timestamp_utc),
            (OPENHANDS_CONV, "/srv/proj", ""),
        )
        self.assertEqual(rows[-1].text, "The project has a README.")

    def test_null_meta_json_is_reported(self):
        path = self.home / (
            ".openhands/agent-canvas/dev_conversations/%s/meta.json" % OPENHANDS_CONV
        )
        path.write_text("null", encoding="utf-8")
        rows = self.conversation(self.CANVAS)
        self.assertEqual(rows[2].text, "parser: meta.json is not a JSON object")
        self.assertEqual(len(rows), 11)

    WORKTREE_CONVERSATION = ".openhands/agent-canvas/dev_conversations/%s/" % OPENHANDS_WT_CONV
    HISTORY = ".openhands/projects/%s/prompt_history.json" % OPENHANDS_PROJECT_HASH
    SONNET, MINI, HAIKU = (
        "litellm_proxy/claude-sonnet-4-5",
        "openai/gpt-5-mini",
        "litellm_proxy/claude-haiku-4-5",
    )

    def test_worktree_conversation_rows(self):
        rows = self.conversation(self.WORKTREE_CONVERSATION + "events/")
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [
                ("system", rows[0].text),
                ("system", "model: " + self.SONNET),
                (
                    "system",
                    "scratch directory: %s (conversation worktree under the Agent Server's "
                    "default conversation_worktree_root /tmp/conversation-worktrees)"
                    % OPENHANDS_WT_DIR,
                ),
                ("user", "fix the tests"),
                ("tool_use", "pytest"),
                (
                    "system",
                    "permission decision: analyzed risk=LOW analyzer=LLMSecurityAnalyzer "
                    'policy=ConfirmRisky details={"rationale":"runs the test suite"}',
                ),
                ("tool_result", "2 failed"),
                ("tool_use", "{}"),
                ("tool_result", "Switched LLM profile to 'fast'"),
                ("system", "model: %s -> %s profile=fast" % (self.SONNET, self.MINI)),
                ("tool_use", "rm -rf build"),
                (
                    "system",
                    "permission decision: analyzed risk=HIGH analyzer=LLMSecurityAnalyzer "
                    "policy=ConfirmRisky",
                ),
                ("tool_result", "[rejected by hook] rm is blocked by hook"),
                (
                    "system",
                    "permission decision: rejected rejection_source=hook rm is blocked by hook",
                ),
                ("system", "model: %s -> %s" % (self.MINI, self.HAIKU)),
                ("assistant", "Done."),
            ],
        )
        self.assertTrue(rows[0].text.startswith("conversation start | Fix the tests"))
        self.assertIn("model=" + self.SONNET, rows[0].text)
        self.assertEqual({row.project_path for row in rows}, {OPENHANDS_WT_DIR})
        self.assertEqual({row.session_id for row in rows}, {OPENHANDS_WT_CONV})
        # Verdicts join to the analysed action's tool call.
        low, high = (row for row in rows if row.text.startswith("permission decision: analyzed"))
        self.assertEqual((low.tool_name, low.tool_use_id), ("terminal", "call_w1"))
        self.assertEqual((high.tool_name, high.tool_use_id), ("terminal", "call_w3"))
        self.assertEqual(low.timestamp_utc, "2026-10-03T09:00:02.500Z")
        # The model column follows the switches.
        self.assertEqual(rows[4].model, self.SONNET)
        self.assertEqual((rows[8].model, rows[9].model), (self.SONNET, self.MINI))
        self.assertEqual(rows[10].model, self.MINI)
        self.assertEqual((rows[14].model, rows[15].model), (self.HAIKU, self.HAIKU))

    def test_base_state_permission_and_model_rows(self):
        rel = self.WORKTREE_CONVERSATION + "base_state.json"
        rows = self.rows_for(OpenHandsParser(), rel)
        tail = " (from base_state.json, file mtime)"
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: confirmation_policy=ConfirmRisky threshold=HIGH "
                "confirm_unknown=true" + tail,
                "permission setting: security_analyzer=LLMSecurityAnalyzer" + tail,
                "permission setting: blocked_actions.8a2f0c1d-0000-4000-8000-000000000006="
                "rm is blocked by hook" + tail,
                "model: %s -> litellm_proxy/claude-opus-4-1 (from base_state.json agent.llm, "
                "file mtime)" % self.HAIKU,
            ],
        )
        self.assertEqual({row.turn_type for row in rows}, {"system"})
        self.assertEqual((rows[2].tool_name, rows[2].tool_use_id), ("terminal", "call_w3"))
        self.assertEqual({row.session_id for row in rows}, {OPENHANDS_WT_CONV})
        self.assertTrue(all(row.timestamp_utc for row in rows))
        # A conversation with no analyzer says so; no switch, no model row.
        rows = self.rows_for(
            OpenHandsParser(), ".openhands/conversations/%s/base_state.json" % OPENHANDS_CLI_CONV
        )
        self.assertEqual(
            [row.text for row in rows], ["permission setting: security_analyzer=none" + tail]
        )

    def test_prompt_history_is_secondary(self):
        parser = OpenHandsParser()
        art = next(artifact for artifact in self.col.artifacts if artifact.rel == self.HISTORY)
        self.assertTrue(parser.wants(art))
        self.assertTrue(parser.secondary(art))
        self.assertTrue(parser.has_secondary)
        event = next(
            artifact for artifact in self.col.artifacts if artifact.rel == self.event(self.CLI, 1)
        )
        self.assertFalse(parser.secondary(event))
        rows = self.rows_for(parser, self.HISTORY)
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc, row.source_line) for row in rows],
            [
                ("user", "list the files", "2026-10-02T08:30:01.000Z", 1),
                ("user", "and the hidden files?", "2026-10-02T08:45:00.000Z", 2),
            ],
        )
        # The directory name is the SHA-256 of a conversation's working_dir.
        self.assertEqual({(row.session_id, row.project_path) for row in rows}, {("", "/srv/proj")})

    def test_prompt_history_cut_is_reported(self):
        path = self.home / self.HISTORY
        raw = path.read_text(encoding="utf-8")
        cut = raw.index('"and the hidden')
        path.write_text(raw[:cut], encoding="utf-8")
        rows = self.rows_for(OpenHandsParser(), self.HISTORY)
        self.assertEqual(rows[0].text, "list the files")
        self.assertEqual(rows[1].turn_type, "system")
        self.assertTrue(rows[1].text.startswith("parser: unparseable record 2 ("), rows[1].text)
        self.assertEqual(rows[1].project_path, "/srv/proj")
        self.assertEqual(len(rows), 2)

    def test_prompt_history_bad_record_is_reported(self):
        path = self.home / self.HISTORY
        entries = json.loads(path.read_text(encoding="utf-8"))
        path.write_text(json.dumps(["oops", *entries]), encoding="utf-8")
        rows = self.rows_for(OpenHandsParser(), self.HISTORY)
        self.assertEqual(
            [row.text for row in rows],
            [
                "list the files",
                "and the hidden files?",
                'parser: unparseable record 1 (not a JSON object): "oops"',
            ],
        )
