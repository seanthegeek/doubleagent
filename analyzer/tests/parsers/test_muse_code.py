"""Tests of the Muse Code parser."""

import json

from doubleagent.inputs import open_input
from doubleagent.parsers import Options
from doubleagent.parsers.antigravity import file_mtime
from doubleagent.parsers.muse_code import MuseCodeParser, micros_to_utc, result_text
from doubleagent.parsers.muse_code import args_summary as muse_args_summary
from fixtures import (
    BAD_LINE,
    MUSE_CHILD,
    MUSE_CHILD_REL,
    MUSE_DIR,
    MUSE_HISTORY_REL,
    MUSE_MODEL,
    MUSE_MODEL_2,
    MUSE_REL,
    MUSE_RUN,
    MUSE_SESSION,
    MUSE_T0,
    MUSE_TRUST_REL,
    _jsonl,
    _muse_env,
    _muse_run,
    bad_line_text,
    write_bad_line,
)

from . import test_codex
from .base import ParserBase


class MuseCodeTests(ParserBase):
    """Test the Muse Code parser."""

    def test_secondary(self):
        # The history file is a secondary source, the transcript is not.
        parser = MuseCodeParser()
        self.assertTrue(parser.secondary(self.artifact(MUSE_HISTORY_REL)))
        self.assertFalse(parser.secondary(self.artifact(MUSE_REL)))

    def append(self, rel, *records):
        with open(self.home / rel, "a", encoding="utf-8") as handle:
            for record in records:
                handle.write(json.dumps(record) + "\n")

    def test_rows(self):
        rows = self.rows_for(MuseCodeParser(), MUSE_REL)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "system",
                "system",
                "system",
                "user",
                "tool_use",
                "tool_result",
                "system",
                "system",
                "system",
                "assistant",
                "system",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "muse-code", MUSE_SESSION),
            )
            self.assertEqual((row.project_path, row.git_branch), ("/srv/proj", "main"))
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        mode, model, security, name, user, use, result, request, review, dec, asst, change = rows
        self.assertEqual({row.model for row in rows[:-1]}, {MUSE_MODEL})
        self.assertEqual(
            (mode.text, mode.timestamp_utc),
            (
                "permission mode: approval=on_request reviewer=auto_review profile=:auto-review"
                " cause=new_session_default format=profile_v1",
                "2026-10-01T09:00:00.000Z",
            ),
        )
        self.assertEqual(
            (model.text, model.timestamp_utc),
            ("model: muse-spark-1.3-contributor provider=meta", "2026-10-01T09:00:00.100Z"),
        )
        self.assertEqual(
            (security.text, security.timestamp_utc),
            ("permission mode: security_mode=example_mode", "2026-10-01T09:00:00.250Z"),
        )
        self.assertEqual(
            (review.tool_name, review.tool_use_id, review.text, review.timestamp_utc),
            (
                "network",
                "call_02",
                "permission decision: allow kind=automated_review_completed risk_level=low"
                " user_authorization=not_required status=completed"
                " Fetching a package index is low risk.",
                "2026-10-01T09:00:05.900Z",
            ),
        )
        self.assertEqual(
            (change.text, change.model, change.timestamp_utc),
            (
                "model: muse-spark-1.3-contributor -> muse-spark-1.3-fast",
                MUSE_MODEL_2,
                "2026-10-01T09:00:15.000Z",
            ),
        )
        self.assertEqual(
            (name.text, name.timestamp_utc),
            ("session name: quiet-lyra", "2026-10-01T09:00:00.300Z"),
        )
        self.assertEqual(
            (user.text, user.timestamp_utc), ("fix the failing test", "2026-10-01T09:00:01.000Z")
        )
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text), ("bash", "call_01", "npm test")
        )
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.text),
            ("bash", "call_01", "[exit 1] 1 failing"),
        )
        self.assertEqual(
            (request.tool_name, request.tool_use_id, request.text),
            ("network", "call_02", "approval requested: https registry.example.org:443"),
        )
        self.assertEqual(
            (dec.tool_name, dec.tool_use_id, dec.text),
            (
                "network",
                "call_02",
                "permission decision: approved source=llm_judge policy_result=allow amendment=-",
            ),
        )
        self.assertEqual(asst.text, "The test fails because the fixture date is stale.")
        self.assertEqual(
            [row.source_line for row in rows], [1, 2, 4, 5, 8, 13, 17, 18, 19, 20, 22, 24]
        )

    def test_thinking_opt_in(self):
        rows = self.rows_for(MuseCodeParser(), MUSE_REL, include_thinking=True)
        thinking = [row for row in rows if row.turn_type == "thinking"]
        # The delta repeats the committed summary; the encrypted reasoning
        # record has empty text. One row.
        self.assertEqual(
            [(row.text, row.source_line, row.model) for row in thinking],
            [("Running the tests first.", 10, MUSE_MODEL)],
        )

    def test_subagent_takes_parent_project(self):
        rows = self.rows_for(MuseCodeParser(), MUSE_CHILD_REL)
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "system", "system", "tool_use", "tool_result"],
        )
        self.assertEqual({row.session_id for row in rows}, {MUSE_CHILD})
        self.assertEqual(
            {(row.project_path, row.git_branch) for row in rows}, {("/srv/proj", "main")}
        )
        # Each subagent log tracks its own permission mode and model.
        self.assertTrue(rows[0].text.startswith("permission mode: approval=on_request"))
        self.assertEqual(rows[1].text, "model: muse-spark-1.3-contributor provider=meta")
        rows = rows[2:]
        self.assertEqual(
            rows[0].text, "subagent task: You are a reminder observer for the main agent."
        )
        self.assertEqual(
            (rows[1].tool_name, rows[1].tool_use_id), ("submit_reminder_decision", "call_10")
        )
        self.assertEqual(
            (rows[2].tool_name, rows[2].tool_use_id, rows[2].text),
            ("submit_reminder_decision", "call_10", "reminder decision recorded"),
        )

    def test_subagent_without_parent(self):
        (self.home / MUSE_REL).unlink()
        rows = self.rows_for(MuseCodeParser(), MUSE_CHILD_REL)
        self.assertEqual(len(rows), 5)
        self.assertEqual({(row.project_path, row.git_branch) for row in rows}, {("", "")})

    def test_history(self):
        rows = self.rows_for(MuseCodeParser(), MUSE_HISTORY_REL)
        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual(
            (
                row.turn_type,
                row.text,
                row.session_id,
                row.project_path,
                row.timestamp_utc,
                row.source_line,
            ),
            ("user", "fix the failing test", MUSE_SESSION, "/srv/proj", "", 1),
        )

    def test_cut_log_and_other_run_events(self):
        session, run, started = MUSE_SESSION, MUSE_RUN, MUSE_T0
        self.append(
            MUSE_REL,
            _muse_run(
                session,
                24,
                started + 30000000,
                run,
                {
                    "kind": "inbox_item_queued",
                    "source": {"source": "user_steer"},
                    "payload": {"prompt": "also check the linter"},
                },
                40,
            ),
            _muse_run(
                session,
                25,
                started + 31000000,
                run,
                {"kind": "reasoning_summary_delta", "message_id": "m9", "text": "First."},
                41,
            ),
            _muse_run(
                session,
                26,
                started + 31100000,
                run,
                {"kind": "reasoning_summary_delta", "message_id": "m9", "text": "Second."},
                42,
            ),
            _muse_run(
                session,
                27,
                started + 32000000,
                run,
                {"kind": "terminal", "terminal": "failed", "reason": "402 Payment Required"},
                43,
            ),
            _muse_env(
                session,
                28,
                started + 33000000,
                "user_shell.command",
                {"record": {"command_id": "sh1", "command_text": "ls"}},
            ),
            _muse_env(
                session,
                29,
                started + 33100000,
                "user_shell.result",
                {"record": {"exit_code": 2, "visible_output": "no such file"}},
            ),
            _muse_env(session, 30, started + 34000000, "some.future.type", {"kind": "x"}),
        )
        rows = self.rows_for(MuseCodeParser(), MUSE_REL, include_thinking=True)[-5:]
        self.assertEqual(
            [(row.turn_type, row.text, row.source_line) for row in rows],
            [
                ("user", "also check the linter", 26),
                ("thinking", "First.\n\nSecond.", 28),
                ("system", "run failed: 402 Payment Required", 29),
                ("tool_use", "ls", 30),
                ("tool_result", "[exit 2] no such file", 31),
            ],
        )
        self.assertEqual(
            {(row.tool_name, row.tool_use_id) for row in rows[3:]}, {("user_shell", "sh1")}
        )

    def test_helpers(self):
        self.assertEqual(micros_to_utc(MUSE_T0), "2026-10-01T09:00:00.000Z")
        self.assertEqual(micros_to_utc(None), "")
        self.assertEqual(muse_args_summary('{"url": "https://example.org"}'), "https://example.org")
        self.assertEqual(muse_args_summary({"cmd": "make"}), "make")
        self.assertEqual(muse_args_summary("not json"), "not json")
        self.assertEqual(result_text('{"output": "ok", "exit_code": 0}'), "ok")
        self.assertEqual(result_text("tool failed: 403"), "tool failed: 403")

    def test_not_wanted(self):
        for rel in (
            ".config/muse/settings.json",
            MUSE_DIR + "approval-review/7a7a7a7a-0000-4000-8000-000000000001.jsonl",
        ):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertEqual(art.agent, "muse-code")
            self.assertFalse(MuseCodeParser().wants(art), rel)

    def test_not_muse_file_gives_no_rows(self):
        with open(self.home / MUSE_REL, "w", encoding="utf-8") as handle:
            handle.write('{"type": "user", "content": "x"}\n')
        self.assertEqual(self.rows_for(MuseCodeParser(), MUSE_REL), [])

    def test_truncated_line_is_reported_not_fatal(self):
        write_bad_line(self.home / MUSE_REL)
        rows = self.rows_for(MuseCodeParser(), MUSE_REL)
        self.assertEqual(len(rows), 13)
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].text, rows[-1].source_line, rows[-1].session_id),
            ("system", bad_line_text(), 26, MUSE_SESSION),
        )

    def test_bad_line_mid_file_keeps_the_rows_around_it(self):
        row = test_codex.CodexTests.check_bad_line(self, MuseCodeParser, MUSE_REL, 2)
        self.assertEqual(row.session_id, MUSE_SESSION)

    def test_truncated_history_line(self):
        line_count = len((self.home / MUSE_HISTORY_REL).read_text(encoding="utf-8").splitlines())
        with open(self.home / MUSE_HISTORY_REL, "a", encoding="utf-8") as handle:
            handle.write(BAD_LINE + "\n" + json.dumps("next") + "\n")
        rows = self.rows_for(MuseCodeParser(), MUSE_HISTORY_REL)
        self.assertEqual(
            [(row.turn_type, row.source_line, row.text) for row in rows],
            [
                ("user", 1, "fix the failing test"),
                ("user", line_count + 2, "next"),
                ("system", line_count + 1, bad_line_text()),
            ],
        )

    def test_bad_retained_frame_child_carries_the_child_text(self):
        child = '{"cut": tr'
        try:
            json.loads(child)
        except ValueError as error:
            reason = str(error)
        else:
            raise AssertionError("the child parsed")
        next_line = len((self.home / MUSE_REL).read_text(encoding="utf-8").splitlines()) + 1
        self.append(
            MUSE_REL,
            {
                "retained_frame": "session_permission_transaction",
                "children": [{"child_index": 0, "record_json": child}],
            },
        )
        rows = self.rows_for(MuseCodeParser(), MUSE_REL)
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].source_line, rows[-1].text),
            (
                "system",
                next_line,
                "parser: unparseable line (retained_frame child: %s): %s" % (reason, child),
            ),
        )

    def test_reconfigure_without_earlier_model_uses_previous(self):
        rel = (
            MUSE_DIR.replace(MUSE_SESSION, "01a0f000-0000-7000-8000-0000000000ff") + "session.jsonl"
        )
        _jsonl(
            self.home / rel,
            [
                _muse_env(
                    "s2",
                    1,
                    MUSE_T0,
                    "runtime.model_reconfigure.completed",
                    {
                        "record": {
                            "effective": {"model_id": MUSE_MODEL_2},
                            "previous": {"model_id": MUSE_MODEL},
                            "apply_outcome": "applied",
                        }
                    },
                ),
                _muse_env(
                    "s2",
                    2,
                    MUSE_T0 + 1000000,
                    "runtime.model_reconfigure.completed",
                    {"record": {"effective": {"model_id": MUSE_MODEL_2}}},
                ),
            ],
        )
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(MuseCodeParser(), rel)
        self.assertEqual(
            [(row.text, row.model, row.source_line) for row in rows],
            [
                ("model: %s" % MUSE_MODEL, MUSE_MODEL, 1),
                (
                    "model: %s -> %s apply_outcome=applied" % (MUSE_MODEL, MUSE_MODEL_2),
                    MUSE_MODEL_2,
                    1,
                ),
                # A `/model` to the same model keeps its row.
                ("model: %s -> %s" % (MUSE_MODEL_2, MUSE_MODEL_2), MUSE_MODEL_2, 2),
            ],
        )

    def test_trusted_projects(self):
        art = next(artifact for artifact in self.col.artifacts if artifact.rel == MUSE_TRUST_REL)
        rows = list(MuseCodeParser().parse(art, Options()))
        mtime = file_mtime(art)
        self.assertTrue(mtime)
        self.assertEqual(
            [(row.turn_type, row.text, row.project_path) for row in rows],
            [
                (
                    "system",
                    "permission setting: projects./srv/proj.decision=trusted"
                    " (from .config/muse/trust.json, file mtime)",
                    "/srv/proj",
                ),
                (
                    "system",
                    "permission setting: projects./srv/other.decision=trusted"
                    " (from .config/muse/trust.json, file mtime)",
                    "/srv/other",
                ),
            ],
        )
        for row in rows:
            self.assertEqual((row.timestamp_utc, row.session_id, row.source_line), (mtime, "", 1))

    def test_unreadable_trust_file(self):
        (self.home / MUSE_TRUST_REL).write_text("[1, 2", encoding="utf-8")
        rows = self.rows_for(MuseCodeParser(), MUSE_TRUST_REL)
        self.assertEqual(
            [(row.turn_type, row.text, row.session_id) for row in rows],
            [("system", "parser: trust.json is not a JSON object", "")],
        )
        self.assertTrue(rows[0].timestamp_utc)
        # The session log still yields every row.
        self.assertEqual(len(self.rows_for(MuseCodeParser(), MUSE_REL)), 12)
