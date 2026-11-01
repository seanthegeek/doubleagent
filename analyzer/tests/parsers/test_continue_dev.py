"""Tests of the Continue parser."""

import json
import unittest

from test_errors import CAN_DENY, DENY_REASON

from doubleagent.dedup import deduplicate
from doubleagent.inputs import open_input
from doubleagent.parsers.continue_dev import (
    LOG_LINE_PATTERN,
    ContinueParser,
    load_session,
    yaml_scalar,
)
from doubleagent.timeutil import to_utc
from fixtures import (
    CONTINUE_SESSION,
    bad_line_text,
    continue_index,
    continue_index_cut,
    write_bad_line,
)

from .base import ParserBase


class ContinueTests(ParserBase):
    """Test the Continue parser."""

    def test_session_recovery_takes_json_whitespace_only(self):
        template = '{"sessionId"%s:%s"s1", "history"%s:%s[{"a": 1}, {"b'
        session, _ = load_session(template % (" ", "\n", "\r\n", "\t"))
        self.assertEqual((session.get("sessionId"), session["history"]), ("s1", [{"a": 1}]))
        for space in map(chr, (0x00A0, 0x2028, 0x0085)):
            session, _ = load_session(template % (space, "", space, ""))
            self.assertEqual(session, {"history": []}, repr(space))

    def test_yaml_mapping_value_after_a_tab_is_no_scalar(self):
        self.assertIsNone(yaml_scalar("Bash:\tls"))
        self.assertEqual(yaml_scalar("Bash(ls:%s)" % chr(0x00A0)), "Bash(ls:%s)" % chr(0x00A0))

    def test_log_line_takes_ascii_digits_only(self):
        line = "2026-10-01 08:00:00.123 [0123abcd] [info]: x"
        self.assertTrue(LOG_LINE_PATTERN.match(line))
        self.assertIsNone(LOG_LINE_PATTERN.match(line.replace("3", chr(0x0663), 1)))

    REL = ".continue/sessions/%s.json" % CONTINUE_SESSION

    def test_rows(self):
        rows = self.rows_for(ContinueParser(), self.REL)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "system",
                "system",
                "user",
                "tool_use",
                "tool_result",
                "system",
                "assistant",
                "tool_use",
                "tool_result",
                "system",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "continue", CONTINUE_SESSION),
            )
            self.assertEqual((row.project_path, row.git_branch), ("/srv/proj", ""))
            # no per-message time: every row inherits sessions.json dateCreated
            self.assertEqual(row.timestamp_utc, "2026-10-02T10:00:00.000Z")
        start, model, mode, user, use1, res1, switch, asst, _use2, res2, canceled = rows
        self.assertIn("session start: Fix bug mode=agent items=5", start.text)
        # chatModelTitle starts the session; item 5's promptLogs switch it
        self.assertEqual(
            (model.text, model.model, model.source_line), ("model: GPT-4o", "GPT-4o", 0)
        )
        # the session's mode is a permission mode
        self.assertEqual((mode.text, mode.source_line), ("permission mode: agent", 0))
        self.assertEqual(
            (switch.text, switch.model, switch.source_line),
            ("model: GPT-4o -> Claude Sonnet 4.5", "Claude Sonnet 4.5", 5),
        )
        self.assertEqual(user.text, "rename foo\n[image]")
        self.assertEqual(
            (use1.tool_name, use1.tool_use_id, use1.text, use1.model),
            ("edit_existing_file", "call_1", '{"filepath":"a.py"}', "GPT-4o"),
        )
        self.assertEqual(
            (res1.tool_name, res1.tool_use_id, res1.text), ("edit_existing_file", "call_1", "ok")
        )
        self.assertEqual(asst.text, "Renamed. Cleaning the build too.")
        # a call with no tool message keeps its outcome in toolCallStates
        self.assertEqual(
            (res2.tool_use_id, res2.text), ("call_2", "[canceled] Permission denied by user")
        )
        # the canceled state is a per-call decision, after its result row,
        # with the CLI's denial reason
        self.assertEqual(
            (canceled.text, canceled.tool_use_id, canceled.tool_name, canceled.timestamp_utc),
            (
                "permission decision: canceled reason=user",
                "call_2",
                "run_terminal_command",
                "2026-10-02T10:00:00.000Z",
            ),
        )
        self.assertEqual([row.source_line for row in rows], [0, 0, 0, 1, 3, 4, 5, 5, 5, 5, 5])

    def _write_session(self, session_id: str, workspace: str, states: list, mode: str = "") -> str:
        """Write a one-item session file `sid` in `workspace`.

        Its assistant item has `states`; the input is reopened so the parser is offered
        it.
        """
        calls = [state["toolCall"] for state in states]
        session = {
            "sessionId": session_id,
            "title": "t",
            "workspaceDirectory": workspace,
            "history": [
                {
                    "message": {"role": "assistant", "content": "", "toolCalls": calls},
                    "contextItems": [],
                    "toolCallStates": states,
                }
            ],
        }
        if mode:
            session["mode"] = mode
        rel = ".continue/sessions/%s.json" % session_id
        (self.home / rel).write_text(json.dumps(session), encoding="utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        return rel

    @staticmethod
    def _state(call_id: str, status: str, output: list) -> dict:
        call = {"id": call_id, "type": "function", "function": {"name": "t", "arguments": "{}"}}
        return {"toolCallId": call_id, "toolCall": call, "status": status, "output": output}

    def test_policy_decisions_in_states(self):
        # The CLI's policy refusal and the extension's Security Policy
        # Violation; an errored call without that item is no decision.
        violation = {"name": "Security Policy Violation", "content": "disabled"}
        rel = self._write_session(
            "5e5e",
            "/srv/proj",
            [
                self._state("c1", "canceled", [{"content": "Command blocked by security policy"}]),
                self._state("c2", "errored", [violation]),
                self._state("c3", "errored", [{"name": "Tool Result", "content": "boom"}]),
                self._state("c4", "canceled", []),
            ],
            mode="plan",
        )
        rows = self.rows_for(ContinueParser(), rel)
        decisions = [
            (row.tool_use_id, row.text)
            for row in rows
            if row.text.startswith("permission decision:")
        ]
        self.assertEqual(
            decisions,
            [
                ("c1", "permission decision: canceled reason=policy"),
                ("c2", "permission decision: errored reason=policy"),
                ("c4", "permission decision: canceled"),
            ],
        )
        self.assertIn("permission mode: plan", [row.text for row in rows])
        self.assertFalse([row for row in rows if row.text.startswith("scratch directory:")])

    def test_cn_review_worktree_is_scratch(self):
        # `cn review` runs in <tmp>/cn-review-<ms>-<n>; a CLI session
        # has no mode, so no permission mode row.
        path = "/tmp/cn-review-1790935200000-0"
        rel = self._write_session("6f6f", path, [])
        rows = self.rows_for(ContinueParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.project_path, row.source_line, row.text) for row in rows[1:]],
            [("system", path, 0, "scratch directory: %s (cn review worktree)" % path)],
        )
        rel = self._write_session("7a7a", "C:\\Users\\a\\AppData\\Local\\Temp\\cn-review-17-1", [])
        texts = [row.text for row in self.rows_for(ContinueParser(), rel)]
        self.assertTrue(texts[-1].startswith("scratch directory: C:\\Users"), texts)
        rel = self._write_session("8b8b", "/srv/cn-review-notes", [])
        texts = [row.text for row in self.rows_for(ContinueParser(), rel)]
        self.assertFalse([text for text in texts if text.startswith("scratch directory:")])

    PERMS = ".continue/permissions.yaml"

    def test_permissions_yaml(self):
        # One standing-grant row per rule, at the file's mtime.
        rows = self.rows_for(ContinueParser(), self.PERMS)
        mtime = to_utc((self.home / self.PERMS).stat().st_mtime)
        self.assertEqual(
            [
                (row.turn_type, row.source_line, row.timestamp_utc, row.session_id, row.text)
                for row in rows
            ],
            [
                (
                    "system",
                    line,
                    mtime,
                    "",
                    "permission setting: %s (from .continue/permissions.yaml, file mtime)" % rule,
                )
                for line, rule in (
                    (6, "allow=Read"),
                    (7, "allow=Bash(npm*)"),
                    (10, "exclude=Bash(rm*)"),
                )
            ],
        )
        self.assertFalse(ContinueParser().secondary(self.artifact(self.PERMS)))

    def test_permissions_yaml_other_constructs(self):
        path = self.home / self.PERMS
        for text, line in (
            ("allow:\n  - Read\nask: [Write]\n", 3),
            ("allow:\n  - Read\nmodels:\n  - x\n", 3),
            ("allow:\n  - {tool: Read}\n", 2),
            ("- Read\n", 1),
            ("allow: null\n  - Read\n", 2),
        ):
            path.write_text(text, encoding="utf-8")
            rows = self.rows_for(ContinueParser(), self.PERMS)
            self.assertEqual(
                [(row.source_line, row.text) for row in rows],
                [
                    (
                        0,
                        "parser: unsupported YAML at line %d (only allow, ask and exclude lists"
                        " of strings are read)" % line,
                    )
                ],
                text,
            )
        path.write_text("# cn tool permissions\n\nallow: []\nask: []\nexclude: []\n")
        self.assertEqual(self.rows_for(ContinueParser(), self.PERMS), [])
        path.write_text("allow:\n- 'it''s'\n- \"a\\\"b\"\n", encoding="utf-8")
        self.assertEqual(
            [row.text.split(" (from")[0] for row in self.rows_for(ContinueParser(), self.PERMS)],
            ["permission setting: allow=it's", 'permission setting: allow=a"b'],
        )

    def test_permissions_yaml_line_ends_and_trims(self):
        path = self.home / self.PERMS

        def rules(text):
            path.write_text(text, encoding="utf-8", newline="")
            return [
                (row.source_line, row.text.split(" (from")[0])
                for row in self.rows_for(ContinueParser(), self.PERMS)
            ]

        self.assertEqual(
            rules('allow:\n  - "a\u2028b"\n'), [(2, "permission setting: allow=a\u2028b")]
        )
        with_lf = "# rules\nallow:\n  - Read\nexclude:\n  - Bash(rm*)\n"
        self.assertEqual(rules(with_lf.replace("\n", "\r\n")), rules(with_lf))
        self.assertEqual(len(rules(with_lf)), 2)
        # A line of U+00A0 is not YAML whitespace, so not a blank line.
        self.assertEqual(
            rules("allow:\n  - Read\n\u00a0\n"),
            [
                (
                    0,
                    "parser: unsupported YAML at line 3 (only allow, ask and exclude lists"
                    " of strings are read)",
                )
            ],
        )

    HISTORY = ".continue/input_history.json"

    def test_input_history(self):
        # The CLI's prompt history is a secondary source, newest first.
        parser = ContinueParser()
        rows = self.rows_for(parser, self.HISTORY)
        self.assertEqual(
            [
                (row.turn_type, row.source_line, row.timestamp_utc, row.session_id, row.text)
                for row in rows
            ],
            [
                ("user", 1, "2026-10-02T10:01:00.000Z", "", "/model"),
                ("user", 2, "2026-10-02T10:00:01.000Z", "", "rename foo"),
            ],
        )
        art = self.artifact(self.HISTORY)
        self.assertTrue(parser.secondary(art))
        self.assertTrue(parser.has_secondary)
        # The session rows carry the session's creation time, so a history
        # entry pairs on text whatever the time; the slash command is kept.
        primary = self.rows_for(parser, ".continue/dev_data/0.2.0/chatInteraction.jsonl")
        kept, dropped = deduplicate(
            primary, [(row, parser.secondary_tolerance(art)) for row in rows]
        )
        self.assertEqual(dropped["continue"], 1)
        self.assertEqual([row.text for row in kept[len(primary) :]], ["/model"])
        _, dropped = deduplicate(primary, [(row, 2.0) for row in rows])
        self.assertEqual(dropped["continue"], 0)

    def test_input_history_cut_keeps_earlier_entries(self):
        path = self.home / self.HISTORY
        raw = path.read_text(encoding="utf-8")
        cut = raw[: raw.index("rename") + 3]
        path.write_text(cut, encoding="utf-8")
        rows = self.rows_for(ContinueParser(), self.HISTORY)
        self.assertEqual(
            [(row.turn_type, row.source_line) for row in rows], [("user", 1), ("system", 2)]
        )
        self.assertEqual(rows[0].text, "/model")
        self.assertTrue(rows[1].text.startswith("parser: unparseable record 2 ("), rows[1].text)
        self.assertTrue(rows[1].text.endswith(cut[cut.rindex("{") :]), rows[1].text)
        # an element that is not an entry is reported and the walk goes on
        path.write_text(json.dumps([7, {"text": "after", "timestamp": 1}]), encoding="utf-8")
        self.assertEqual(
            [(row.source_line, row.text) for row in self.rows_for(ContinueParser(), self.HISTORY)],
            [
                (2, "after"),
                (1, "parser: unparseable record 1 (not an object with a text): 7"),
            ],
        )

    DEV = ".continue/dev_data/0.2.0/%s.jsonl"

    def test_chat_feedback_is_secondary(self):
        parser = ContinueParser()
        rel = self.DEV % "chatFeedback"
        rows = self.rows_for(parser, rel)
        self.assertEqual(
            [
                (row.turn_type, row.session_id, row.project_path, row.model, row.text)
                for row in rows
            ],
            [
                ("user", CONTINUE_SESSION, "/srv/proj", "", "rename foo"),
                ("assistant", CONTINUE_SESSION, "/srv/proj", "GPT-4o", "done"),
                (
                    "system",
                    CONTINUE_SESSION,
                    "/srv/proj",
                    "GPT-4o",
                    "chat feedback: feedback=true model=GPT-4o",
                ),
            ],
        )
        self.assertEqual({row.timestamp_utc for row in rows}, {"2026-10-02T10:01:30.000Z"})
        art = self.artifact(rel)
        self.assertTrue(parser.secondary(art))
        # the rated reply's chatInteraction line holds the prompt and reply
        primary = self.rows_for(parser, self.DEV % "chatInteraction")
        kept, dropped = deduplicate(
            primary, [(row, parser.secondary_tolerance(art)) for row in rows]
        )
        self.assertEqual(dropped["continue"], 2)
        self.assertEqual([row.turn_type for row in kept[len(primary) :]], ["system"])

    def test_edit_interaction_and_outcome(self):
        parser = ContinueParser()
        rows = self.rows_for(parser, self.DEV % "editInteraction")
        self.assertEqual(
            [(row.turn_type, row.model, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    "GPT-4o",
                    "2026-10-02T10:02:00.000Z",
                    "edit interaction: file=/srv/proj/a.py",
                ),
                ("user", "", "2026-10-02T10:02:00.000Z", "add a docstring"),
                ("assistant", "GPT-4o", "2026-10-02T10:02:00.000Z", 'def foo():\n    """Doc."""'),
            ],
        )
        rows = self.rows_for(parser, self.DEV % "editOutcome")
        self.assertEqual(
            [(row.turn_type, row.model, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    "GPT-4o",
                    "2026-10-02T10:02:30.000Z",
                    "permission decision: edit accepted=false file=/srv/proj/a.py lines 1 -> 2",
                )
            ],
        )
        for event in ("editInteraction", "editOutcome"):
            self.assertFalse(parser.secondary(self.artifact(self.DEV % event)))

    def test_truncated_new_dev_data_lines(self):
        # Each new line-oriented file keeps the lines before and after a cut.
        after = {
            "chatFeedback": '{"sessionId": "0f1e", "prompt": "later", "feedback": false}',
            "editInteraction": '{"prompt": "later", "filepath": "b.py"}',
            "editOutcome": '{"accepted": true, "filepath": "b.py"}',
        }
        want = {
            "chatFeedback": [
                (1, "user"),
                (1, "assistant"),
                (1, "system"),
                (3, "user"),
                (3, "system"),
            ],
            "editInteraction": [
                (1, "system"),
                (1, "user"),
                (1, "assistant"),
                (3, "system"),
                (3, "user"),
            ],
            "editOutcome": [(1, "system"), (3, "system")],
        }
        for event, line in after.items():
            rel = self.DEV % event
            write_bad_line(self.home / rel)
            with open(self.home / rel, "a", encoding="utf-8") as handle:
                handle.write("\n" + line + "\n")
            rows = self.rows_for(ContinueParser(), rel)
            self.assertEqual(
                [(row.source_line, row.turn_type) for row in rows[:-1]], want[event], event
            )
            self.assertEqual((rows[-1].source_line, rows[-1].text), (2, bad_line_text()), event)

    def test_cn_log_policy_rule(self):
        # A "don't ask again" answer logs the rule it added.
        path = self.home / ".continue/logs/cn.log"
        path.write_text(
            "2026-10-07 11:00:00.000 [a1b2c3d4] [info]: Added policy rule to permissions.yaml:"
            " Bash(npm*)\n",
            encoding="utf-8",
        )
        rows = self.rows_for(ContinueParser(), ".continue/logs/cn.log")
        self.assertEqual(
            [(row.timestamp_utc, row.source_line, row.session_id, row.text) for row in rows],
            [
                (
                    "2026-10-07T11:00:00.000Z",
                    1,
                    "",
                    "permission decision: allow always rule=Bash(npm*) (from logs/cn.log, log"
                    " line time, host-local)",
                )
            ],
        )

    def test_thinking_opt_in(self):
        rows = self.rows_for(ContinueParser(), self.REL, include_thinking=True)
        thinking = [row for row in rows if row.turn_type == "thinking"]
        self.assertEqual([row.text for row in thinking], ["consider the callers", "find foo first"])
        self.assertEqual(thinking[1].timestamp_utc, "2026-10-02T10:00:01.000Z")  # reasoning.startAt

    def test_dev_data(self):
        rows = self.rows_for(ContinueParser(), ".continue/dev_data/0.2.0/chatInteraction.jsonl")
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("user", "rename foo"), ("assistant", "done")],
        )
        self.assertTrue(all(row.timestamp_utc == "2026-10-02T10:00:05.000Z" for row in rows))
        self.assertEqual(
            (rows[1].session_id, rows[1].project_path, rows[1].model),
            (CONTINUE_SESSION, "/srv/proj", "GPT-4o"),
        )
        rows = self.rows_for(ContinueParser(), ".continue/dev_data/0.2.0/toolUsage.jsonl")
        self.assertEqual(
            [(row.turn_type, row.tool_name, row.tool_use_id) for row in rows],
            [
                ("tool_use", "edit_existing_file", "call_1"),
                ("tool_result", "edit_existing_file", "call_1"),
                ("system", "edit_existing_file", "call_1"),
                ("tool_use", "run_terminal_command", "call_2"),
                ("tool_result", "run_terminal_command", "call_2"),
                ("system", "run_terminal_command", "call_2"),
            ],
        )
        self.assertEqual(rows[1].text, "accepted=true succeeded=true ok")
        self.assertEqual(
            (rows[2].text, rows[2].timestamp_utc, rows[2].session_id),
            ("permission decision: accepted=true", "2026-10-02T10:00:03.000Z", ""),
        )
        self.assertEqual(rows[4].text, "accepted=false succeeded=false")
        self.assertEqual(
            (rows[5].text, rows[5].timestamp_utc, rows[5].source_line),
            ("permission decision: accepted=false", "2026-10-02T10:00:06.000Z", 2),
        )

    def test_index_and_config_not_wanted(self):
        parser = ContinueParser()
        art = self.artifact(".continue/config.yaml")
        self.assertFalse(parser.wants(art))
        # the index is wanted only for the rows that say what it lost
        self.assertEqual(self.rows_for(parser, ".continue/sessions/sessions.json"), [])

    def test_truncated_session_keeps_earlier_turns(self):
        path = self.home / self.REL
        raw = path.read_text(encoding="utf-8")
        path.write_text(raw[: raw.index('"toolCallId": "call_2"')], encoding="utf-8")
        rows = self.rows_for(ContinueParser(), self.REL)
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "user", "system", "tool_use", "tool_result", "system"],
        )
        # chatModelTitle sits after the cut, so the first promptLogs title starts it
        self.assertEqual((rows[2].text, rows[2].source_line), ("model: GPT-4o", 3))
        self.assertIn("recovered 4 history item(s)", rows[-1].text)
        self.assertEqual(rows[1].session_id, CONTINUE_SESSION)

    def test_truncated_dev_data_line(self):
        rel = ".continue/dev_data/0.2.0/chatInteraction.jsonl"
        write_bad_line(self.home / rel)
        with open(self.home / rel, "a", encoding="utf-8") as handle:
            handle.write(
                "\n" + '{"sessionId": "0f1e", "timestamp": "2026-10-02T10:00:09.000Z", '
                '"prompt": "after the cut"}' + "\n"
            )
        rows = self.rows_for(ContinueParser(), rel)
        self.assertEqual(
            [(row.source_line, row.turn_type) for row in rows],
            [(1, "user"), (1, "assistant"), (3, "user"), (2, "system")],
        )
        self.assertEqual((rows[0].text, rows[2].text), ("rename foo", "after the cut"))
        self.assertEqual(rows[-1].text, bad_line_text())

    def test_load_session_trailing_data(self):
        session, problem = load_session('{"sessionId": "x", "history": []}\n{"junk"')
        self.assertEqual(session["sessionId"], "x")
        self.assertIn("trailing data", problem)

    POSTED = (
        "agent metadata posted: agent agt_7f3e keys %s %s (from logs/%s, log line time, host-local)"
    )

    def test_cn_log_rows(self):
        # `cn serve --id` posts and the skipped storage upload; a
        # result line settles the oldest post of its process and URL, and the
        # request body and diagnostics are not events.
        rows = self.rows_for(ContinueParser(), ".continue/logs/cn.log")
        self.assertEqual(
            [
                (row.turn_type, row.timestamp_utc, row.session_id, row.source_line, row.text)
                for row in rows
            ],
            [
                (
                    "system",
                    "2026-10-07T10:00:00.010Z",
                    "",
                    2,
                    "storage upload requested, skipped (from logs/cn.log, log line time,"
                    " host-local)",
                ),
                (
                    "system",
                    "2026-10-07T10:00:05.100Z",
                    "",
                    3,
                    self.POSTED % ("summary,isComplete", "Success", "cn.log"),
                ),
                (
                    "system",
                    "2026-10-07T10:00:05.200Z",
                    "",
                    5,
                    self.POSTED % ("additions", "Failed", "cn.log"),
                ),
                (
                    "system",
                    "2026-10-07T10:00:06.000Z",
                    "",
                    7,
                    self.POSTED % ("isComplete", "-", "cn.log"),
                ),
            ],
        )
        self.assertFalse([row for row in rows if "Renamed foo" in row.text])
        self.assertFalse(ContinueParser().records_sync)

    def test_rotated_cn_log(self):
        rows = self.rows_for(ContinueParser(), ".continue/logs/cn1.log")
        self.assertEqual(
            [(row.timestamp_utc, row.text) for row in rows],
            [("2026-10-06T09:00:00.000Z", self.POSTED % ("summary", "Auth required", "cn1.log"))],
        )
        parser = ContinueParser()
        self.assertFalse(parser.wants(self.artifact(".continue/logs/prompt.log")))
        self.assertFalse(parser.secondary(self.artifact(".continue/logs/cn.log")))

    def test_cn_log_without_posts_has_no_rows(self):
        # A log of an ordinary `cn` session, cut mid-line: no event, no error.
        path = self.home / ".continue/logs/cn.log"
        path.write_text(
            "2026-10-07 10:00:00.001 [a1b2c3d4] [info]: Starting server on port 8000\n"
            "2026-10-07 10:00:00.002 [a1b2c3d4] [info]: [metadata] Succ",
            encoding="utf-8",
        )
        self.assertEqual(self.rows_for(ContinueParser(), ".continue/logs/cn.log"), [])

    INDEX = ".continue/sessions/sessions.json"

    def _session_times(self, parser) -> set:
        return {row.timestamp_utc for row in self.rows_for(parser, self.REL)}

    def test_index_cut_mid_write_is_reported(self):
        # The entries before the cut still time their sessions, and
        # the broken entry is one row carrying it to the end of the file.
        text, tail = continue_index_cut(continue_index())
        (self.home / self.INDEX).write_text(text, encoding="utf-8")
        parser = ContinueParser()
        self.assertEqual(self._session_times(parser), {"2026-10-02T10:00:00.000Z"})
        rows = self.rows_for(parser, self.INDEX)
        self.assertEqual(
            [(row.turn_type, row.source_line, row.session_id, row.timestamp_utc) for row in rows],
            [("system", 2, "", "")],
        )
        self.assertTrue(
            rows[0].text.startswith("parser: unparseable record 2 (Unterminated string")
        )
        self.assertTrue(rows[0].text.endswith("): " + tail), rows[0].text)
        self.assertEqual(rows[0].source_file, "/alice/" + self.INDEX)

    def test_index_bad_entry_keeps_the_entries_after_it(self):
        entry = continue_index()[0]
        (self.home / self.INDEX).write_text(
            json.dumps(["not an entry", {"session_id": "pre-1.0"}, entry]), encoding="utf-8"
        )
        parser = ContinueParser()
        self.assertEqual(self._session_times(parser), {"2026-10-02T10:00:00.000Z"})
        # the pre-1.0 `session_id` entry is filtered as the extension does
        self.assertEqual(
            [(row.source_line, row.text) for row in self.rows_for(parser, self.INDEX)],
            [(1, 'parser: unparseable record 1 (not an object with a sessionId): "not an entry"')],
        )

    def test_index_not_an_array_is_reported(self):
        (self.home / self.INDEX).write_text("{}", encoding="utf-8")
        parser = ContinueParser()
        # the session keeps its rows, without the index's time
        self.assertEqual(self._session_times(parser), {""})
        self.assertEqual(
            [(row.source_line, row.text) for row in self.rows_for(parser, self.INDEX)],
            [(1, "parser: not a JSON array")],
        )
        # the index is shared by the dev_data reader; it reports nothing itself
        self.assertEqual(
            [
                row.turn_type
                for row in self.rows_for(parser, ".continue/dev_data/0.2.0/chatInteraction.jsonl")
            ],
            ["user", "assistant"],
        )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_file_is_one_row_and_others_still_parse(self):
        cases = (
            (self.REL, "%s.json" % CONTINUE_SESSION, CONTINUE_SESSION),
            (self.HISTORY, "input_history.json", ""),
            (self.PERMS, "permissions.yaml", ""),
            (".continue/logs/cn.log", "cn.log", ""),
        )
        parser = ContinueParser()
        for rel, name, session_id in cases:
            path = self.home / rel
            other_rel = ".continue/dev_data/0.2.0/chatInteraction.jsonl"
            path.chmod(0)
            try:
                rows = self.rows_for(parser, rel)
                other = self.rows_for(parser, other_rel)
            finally:
                path.chmod(0o600)
            self.assertEqual(
                [
                    (row.turn_type, row.text, row.source_line, row.timestamp_utc, row.session_id)
                    for row in rows
                ],
                [
                    (
                        "system",
                        "parser: %s could not be read (Permission denied)" % name,
                        0,
                        "",
                        session_id,
                    )
                ],
                rel,
            )
            self.assertEqual([row.turn_type for row in other], ["user", "assistant"], rel)

    def test_index_unreadable_is_reported(self):
        path = self.home / self.INDEX
        path.chmod(0)
        try:
            try:
                path.read_bytes()
                self.skipTest("a file with no permissions is still readable here (root)")
            except PermissionError:
                pass
            parser = ContinueParser()
            self.assertEqual(self._session_times(parser), {""})
            self.assertEqual(
                [(row.source_line, row.text) for row in self.rows_for(parser, self.INDEX)],
                [(0, "parser: sessions.json could not be read (Permission denied)")],
            )
        finally:
            path.chmod(0o644)
