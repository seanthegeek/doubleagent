"""Tests of the pi parser."""

import io
import json

from doubleagent import cli
from doubleagent.dedup import deduplicate
from doubleagent.parsers.pi import PiParser
from doubleagent.parsers.pi import args_summary as pi_args_summary
from fixtures import (
    BAD_LINE,
    LC_FILE,
    LC_SESSION,
    PI_BUG_FILE,
    PI_BUG_REPORT,
    PI_BUG_SESSION,
    PI_BUG_ZIP,
    PI_CRASHES,
    PI_DEBUG_LOG,
    PI_DELETED_PROMPT,
    PI_DIR,
    PI_EXPERIMENTAL,
    PI_FILE,
    PI_FORK,
    PI_FORK_FILE,
    PI_SESSION,
    PI_SETTINGS,
    PI_TRUST,
    _json,
    _jsonl,
    bad_line_text,
    pi_session_records,
    write_bad_line,
)

from . import test_codex
from .base import ParserBase, read_jsonl, rel_only


class PiTests(ParserBase):
    """Test the Pi parser."""

    REL = PI_DIR + PI_FILE
    FORK = PI_DIR + PI_FORK_FILE
    LITTLE_CODER = PI_DIR + LC_FILE

    def test_rows(self):
        rows = self.rows_for(PiParser(), self.REL)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "system",
                "user",
                "tool_use",
                "tool_result",
                "tool_use",
                "tool_result",
                "system",
                "system",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id), ("h1", "alice", "pi", PI_SESSION)
            )
            self.assertEqual((row.project_path, row.git_branch), ("/srv/proj", ""))
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        start, model_change, user, use, result, bash, bash_out, info, change = rows
        self.assertEqual(
            (start.text, start.timestamp_utc),
            ("session start: version=3 cwd=/srv/proj", "2026-10-01T09:00:00.000Z"),
        )
        # The model_change entry starts the model; the assistant message on
        # line 4 names the same model and adds no row.
        self.assertEqual(
            (model_change.model, model_change.text, model_change.timestamp_utc),
            (
                "claude-sonnet-4-5",
                "model: claude-sonnet-4-5 provider=anthropic",
                "2026-10-01T09:00:00.100Z",
            ),
        )
        self.assertEqual(
            (change.model, change.text, change.timestamp_utc),
            (
                "gpt-5",
                "model: claude-sonnet-4-5 -> gpt-5 provider=openai",
                "2026-10-01T09:00:11.000Z",
            ),
        )
        self.assertEqual(
            (user.text, user.timestamp_utc), ("fix the failing test", "2026-10-01T09:00:01.000Z")
        )
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.model, use.text),
            ("bash", "toolu_01", "claude-sonnet-4-5", "npm test"),
        )
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.text), ("bash", "toolu_01", "1 failing")
        )
        self.assertEqual(
            (bash.tool_name, bash.tool_use_id, bash.text), ("bash", "5e6f7081", "git status")
        )
        self.assertEqual((bash_out.tool_use_id, bash_out.text), ("5e6f7081", "M src/a.ts"))
        self.assertEqual(info.text, "session_info: name=test fix")
        self.assertEqual(
            [row.source_line for row in rows], [1, 2, 3, 4, 5, 6, 6, 8, 9]
        )  # the label on line 7 is skipped
        self.assertEqual(
            pi_args_summary({"edits": [{"oldText": "a", "newText": "b"}], "path": "src/a.ts"}),
            "src/a.ts",
        )

    def test_thinking_opt_in(self):
        rows = self.rows_for(PiParser(), self.REL, include_thinking=True)
        thinking = [row for row in rows if row.turn_type == "thinking"]
        self.assertEqual(
            [(row.text, row.model) for row in thinking],
            [("run the tests first", "claude-sonnet-4-5")],
        )

    def test_fork_drops_entries_copied_from_parent(self):
        rows = self.rows_for(PiParser(), self.FORK)
        self.assertEqual(
            [row.turn_type for row in rows], ["system", "system", "system", "assistant"]
        )
        self.assertIn("parentSession=/home/alice/" + self.REL, rows[0].text)
        self.assertTrue(
            rows[1].text.startswith("fork: 3 entries copied from the parent session"), rows[1].text
        )
        # The copied model_change is dropped, so the fork's own first
        # assistant message starts its model.
        self.assertEqual(rows[2].text, "model: claude-sonnet-4-5 provider=anthropic")
        self.assertEqual((rows[3].session_id, rows[3].text), (PI_FORK, "Trying another way."))

    def test_fork_kept_whole_without_parent(self):
        (self.home / self.REL).unlink()
        rows = self.rows_for(PiParser(), self.FORK)
        self.assertEqual(
            [row.turn_type for row in rows], ["system", "system", "user", "tool_use", "assistant"]
        )
        self.assertEqual({row.session_id for row in rows}, {PI_FORK})

    def test_little_coder_session_is_flagged(self):
        rows = self.rows_for(PiParser(), self.LITTLE_CODER)
        self.assertEqual({row.agent for row in rows}, {"pi"})
        last = rows[-1]
        self.assertEqual((last.turn_type, last.session_id), ("system", LC_SESSION))
        self.assertTrue(last.text.startswith("session driven by little-coder"), last.text)
        for why in ("lc-skills", "llamacpp", "checkpoints"):
            self.assertIn(why, last.text)
        self.assertIn(
            "custom_message lc-skills: ## Skill: edit\nUse the edit tool.",
            [row.text for row in rows],
        )
        plain = self.rows_for(PiParser(), self.REL)
        self.assertFalse(any("little-coder" in row.text for row in plain))

    def test_experimental_meta(self):
        rows = self.rows_for(
            PiParser(), ".pi/agent/experimental/sessions/%s/meta.json" % PI_EXPERIMENTAL
        )
        self.assertEqual(len(rows), 1)
        self.assertEqual(
            (rows[0].turn_type, rows[0].session_id, rows[0].project_path, rows[0].timestamp_utc),
            ("system", PI_EXPERIMENTAL, "/srv/proj", "2026-10-01T12:00:00.000Z"),
        )

    def test_response_model_and_requested(self):
        path = self.home / self.REL
        recs = pi_session_records()[:4]
        asst = dict(recs[3]["message"], responseModel="claude-sonnet-4-5-20250929")
        recs[3] = dict(recs[3], message=asst)
        _jsonl(path, recs)
        rows = [row for row in self.rows_for(PiParser(), self.REL) if row.text.startswith("model:")]
        self.assertEqual(
            [(row.text, row.model, row.source_line) for row in rows],
            [
                ("model: claude-sonnet-4-5 provider=anthropic", "claude-sonnet-4-5", 2),
                (
                    "model: claude-sonnet-4-5 -> claude-sonnet-4-5-20250929 provider=anthropic "
                    "requested=claude-sonnet-4-5",
                    "claude-sonnet-4-5-20250929",
                    4,
                ),
            ],
        )

    def test_repeated_model_change_keeps_its_row(self):
        # A model_change is the user's switch: one naming the model in
        # force still yields the plain row; a repeating assistant does not.
        path = self.home / self.REL
        recs = pi_session_records()[:4]
        recs.append(dict(recs[1], id="9a0b1c2d", timestamp="2026-10-01T09:00:05.000Z"))
        _jsonl(path, recs)
        rows = [row for row in self.rows_for(PiParser(), self.REL) if row.text.startswith("model:")]
        self.assertEqual(
            [(row.text, row.timestamp_utc, row.source_line) for row in rows],
            [
                ("model: claude-sonnet-4-5 provider=anthropic", "2026-10-01T09:00:00.100Z", 2),
                ("model: claude-sonnet-4-5 provider=anthropic", "2026-10-01T09:00:05.000Z", 5),
            ],
        )

    def test_trust_standing_rows(self):
        self.assertTrue(
            PiParser().wants(
                next(artifact for artifact in self.col.artifacts if artifact.rel == PI_TRUST)
            )
        )
        rows = self.rows_for(PiParser(), PI_TRUST)
        self.assertEqual(
            [
                (
                    row.turn_type,
                    row.text,
                    row.timestamp_utc,
                    row.session_id,
                    row.project_path,
                    row.source_line,
                )
                for row in rows
            ],
            [
                (
                    "system",
                    "permission setting: /srv/proj=true (from .pi/agent/trust.json, file mtime)",
                    "2026-10-01T10:53:20.000Z",
                    "",
                    "/srv/proj",
                    1,
                ),
                (
                    "system",
                    "permission setting: /srv/untrusted=false "
                    "(from .pi/agent/trust.json, file mtime)",
                    "2026-10-01T10:53:20.000Z",
                    "",
                    "/srv/untrusted",
                    1,
                ),
            ],
        )
        self.assertEqual({row.agent for row in rows}, {"pi"})

    def test_trust_not_an_object(self):
        (self.home / PI_TRUST).write_text('["/srv/proj"]', encoding="utf-8")
        rows = self.rows_for(PiParser(), PI_TRUST)
        self.assertEqual(
            [(row.turn_type, row.text, row.session_id) for row in rows],
            [("system", "parser: .pi/agent/trust.json is not a JSON object", "")],
        )
        (self.home / PI_TRUST).write_text('{"/srv/proj": tr', encoding="utf-8")
        rows = self.rows_for(PiParser(), PI_TRUST)
        self.assertEqual(len(rows), 1)
        self.assertIn("is not a JSON object", rows[0].text)

    def test_default_project_trust_row(self):
        # The one key of settings.json the parser reads.
        rows = self.rows_for(PiParser(), PI_SETTINGS)
        self.assertEqual(
            [
                (row.turn_type, row.text, row.timestamp_utc, row.session_id, row.source_line)
                for row in rows
            ],
            [
                (
                    "system",
                    "permission setting: defaultProjectTrust=always "
                    "(from .pi/agent/settings.json, file mtime)",
                    "2026-10-01T10:53:20.000Z",
                    "",
                    1,
                )
            ],
        )
        _json(self.home / PI_SETTINGS, {"defaultProvider": "anthropic"})
        self.assertEqual(self.rows_for(PiParser(), PI_SETTINGS), [])
        (self.home / PI_SETTINGS).write_text("{not json", encoding="utf-8")
        self.assertEqual(self.rows_for(PiParser(), PI_SETTINGS), [])

    def test_permission_gate_refusal(self):
        # little-coder's gate blocks the edit; pi records an error toolResult
        # with the gate's reason, followed by the decision row.
        rows = self.rows_for(PiParser(), self.LITTLE_CODER)
        i = next(
            index for index, row in enumerate(rows) if row.text.startswith("permission decision:")
        )
        result, decision = rows[i - 1], rows[i]
        self.assertEqual(
            (result.turn_type, result.text, result.tool_use_id),
            ("tool_result", "[error] edit cancelled by user", "call_2"),
        )
        self.assertEqual(
            (decision.turn_type, decision.text, decision.tool_name, decision.tool_use_id),
            ("system", "permission decision: denied (edit cancelled by user)", "edit", "call_2"),
        )
        self.assertEqual(
            (decision.timestamp_utc, decision.session_id), ("2026-10-01T11:00:15.000Z", LC_SESSION)
        )
        # A whitelist refusal is one too; an ordinary error result is not.
        path = self.home / self.LITTLE_CODER
        recs = [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines()]
        recs[-1]["message"]["content"][0]["text"] = 'shell whitelist: "rm" is not in SAFE_PREFIXES'
        recs[4]["message"].update(isError=True)  # "1 failing", not a refusal
        _jsonl(path, recs)
        texts = [row.text for row in self.rows_for(PiParser(), self.LITTLE_CODER)]
        self.assertEqual(
            [text for text in texts if text.startswith("permission decision:")],
            ['permission decision: denied (shell whitelist: "rm" is not in SAFE_PREFIXES)'],
        )
        plain = self.rows_for(PiParser(), self.REL)
        self.assertFalse(any(row.text.startswith("permission decision:") for row in plain))

    def test_debug_log_secondary(self):
        art = self.artifact(PI_DEBUG_LOG)
        self.assertTrue(PiParser().secondary(art))
        self.assertEqual(PiParser().secondary_tolerance(art), 3600.0)
        self.assertTrue(PiParser.has_secondary)
        self.assertFalse(PiParser().secondary(self.artifact(self.REL)))
        rows = self.rows_for(PiParser(), PI_DEBUG_LOG)
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc, row.source_line) for row in rows],
            [
                (
                    "system",
                    "debug snapshot: /debug wrote 4 message(s) of the open session, which "
                    "the log does not name (from .pi/agent/pi-debug.log, Debug output at line)",
                    "2026-10-01T09:02:00.000Z",
                    1,
                ),
                ("user", "fix the failing test", "2026-10-01T09:00:01.000Z", 10),
                (
                    "system",
                    "model: claude-sonnet-4-5 provider=anthropic",
                    "2026-10-01T09:00:02.000Z",
                    11,
                ),
                ("tool_use", "npm test", "2026-10-01T09:00:02.000Z", 11),
                ("tool_result", "1 failing", "2026-10-01T09:00:07.000Z", 12),
                ("user", PI_DELETED_PROMPT, "2026-10-01T09:01:40.000Z", 13),
            ],
        )
        for row in rows:
            self.assertEqual((row.agent, row.session_id, row.project_path), ("pi", "", ""))
        self.assertEqual(rows[3].tool_use_id, "toolu_01")

    def test_debug_log_dedup_against_the_session(self):
        # The four rows the session file also holds are dropped, within the
        # one-hour window; the prompt no session holds is recovered.
        primary = self.rows_for(PiParser(), self.REL)
        tolerance = PiParser().secondary_tolerance(self.artifact(PI_DEBUG_LOG))
        secondary = [(row, tolerance) for row in self.rows_for(PiParser(), PI_DEBUG_LOG)]
        kept, dropped = deduplicate(primary, secondary)
        self.assertEqual(dropped, {"pi": 4})
        survivors = kept[len(primary) :]
        self.assertEqual(
            [(row.turn_type, row.recovered) for row in survivors], [("system", ""), ("user", "yes")]
        )
        self.assertEqual(survivors[1].text, PI_DELETED_PROMPT)

    def test_debug_log_bad_line_keeps_the_rows_around_it(self):
        row = test_codex.CodexTests.check_bad_line(self, PiParser, PI_DEBUG_LOG, 10)
        self.assertEqual((row.session_id, row.project_path), ("", ""))

    def test_crash_rows(self):
        rows = self.rows_for(PiParser(), PI_CRASHES)
        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual(
            (row.turn_type, row.timestamp_utc, row.session_id, row.project_path, row.source_line),
            ("system", "2026-10-01T09:05:00.000Z", PI_SESSION, "/srv/proj", 1),
        )
        self.assertEqual(
            row.text,
            "crash: uncaught_exception Cannot read properties of undefined version=1.3.0 "
            "sessionFile=/home/alice/%s%s (from .pi/agent/crashes.json, record timestamp)\n"
            "TypeError: Cannot read properties of undefined\n    at run (ext.ts:3:1)"
            % (PI_DIR, PI_FILE),
        )
        self.assertFalse(PiParser().secondary(self.artifact(PI_CRASHES)))
        # An older record shape (no kind, version or session file).
        _json(
            self.home / PI_CRASHES,
            [{"timestamp": "2026-10-01T09:06:00.000Z", "message": "boom", "cwd": "/srv/proj"}],
        )
        rows = self.rows_for(PiParser(), PI_CRASHES)
        self.assertEqual(
            [(row.text, row.session_id) for row in rows],
            [
                (
                    "crash: - boom version=- sessionFile=- "
                    "(from .pi/agent/crashes.json, record timestamp)",
                    "",
                )
            ],
        )

    def test_truncated_crash_log_keeps_earlier_records(self):
        text = (self.home / PI_CRASHES).read_text(encoding="utf-8")
        (self.home / PI_CRASHES).write_text(
            text.rstrip().rstrip("]") + ', {"timestamp": "2026-10-01T09:0', encoding="utf-8"
        )
        rows = self.rows_for(PiParser(), PI_CRASHES)
        self.assertEqual([row.turn_type for row in rows], ["system", "system"])
        self.assertTrue(rows[0].text.startswith("crash: uncaught_exception"), rows[0].text)
        self.assertEqual(rows[1].source_line, 2)
        self.assertTrue(rows[1].text.startswith("parser: unparseable record 2 ("), rows[1].text)

    def test_config_not_wanted(self):
        for rel in (".pi/agent/auth.json",):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertFalse(PiParser().wants(art), rel)
        for rel in (
            ".pi/agent/auth.json",
            ".pi/agent/mcp-auth.json",
            ".pi/agent/models.json",
            ".pi/agent/oauth.json",
            ".pi/agent/mcp.json",
        ):
            self.assertFalse(PiParser().wants(rel_only(rel)), rel)

    def test_truncated_line_is_reported_not_fatal(self):
        write_bad_line(self.home / self.REL)
        rows = self.rows_for(PiParser(), self.REL)
        self.assertEqual(len(rows), 10)
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].source_line, rows[-1].text),
            ("system", 10, bad_line_text()),
        )
        self.assertIn(
            "model: claude-sonnet-4-5 -> gpt-5 provider=openai", [row.text for row in rows]
        )

    def test_bad_line_mid_file_keeps_the_rows_around_it(self):
        row = test_codex.CodexTests.check_bad_line(self, PiParser, self.REL, 2)
        self.assertEqual(row.project_path, "/srv/proj")
        self.assertTrue(row.session_id)

    def test_headerless_file_gives_one_row_per_bad_line(self):
        (self.home / self.REL).write_text(BAD_LINE + "\n" + BAD_LINE + "\n", encoding="utf-8")
        rows = self.rows_for(PiParser(), self.REL)
        self.assertEqual(
            [(row.turn_type, row.source_line, row.text, row.session_id) for row in rows],
            [("system", line, bad_line_text(), "") for line in (1, 2)],
        )


class PiSyncTests(ParserBase):
    """Test the `pi.bug-report` entry that `/bug` appends."""

    REL = PI_DIR + PI_BUG_FILE
    SOURCE = "(from session entry pi.bug-report, entry timestamp)"

    def test_rows(self):
        self.assertTrue(PiParser.records_sync)
        rows = self.rows_for(PiParser(), self.REL)
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc, row.source_line) for row in rows],
            [
                (
                    "system",
                    "session start: version=3 cwd=/srv/proj",
                    "2026-10-01T13:00:00.000Z",
                    1,
                ),
                ("user", "the build hangs", "2026-10-01T13:00:01.000Z", 2),
                (
                    "system",
                    "session synced: bug report %s uploaded to radius.pi.dev "
                    "transcript=true summary=false %s" % (PI_BUG_REPORT, self.SOURCE),
                    "2026-10-01T13:00:30.000Z",
                    3,
                ),
                (
                    "system",
                    "bug report exported: %s path=/srv/proj/pi-bug-report-%s.zip %s"
                    % (PI_BUG_ZIP, PI_BUG_ZIP, self.SOURCE),
                    "2026-10-01T13:01:00.000Z",
                    4,
                ),
            ],
        )
        for row in rows:
            self.assertEqual((row.session_id, row.project_path), (PI_BUG_SESSION, "/srv/proj"))
        # The user's description is not printed.
        self.assertEqual([row for row in rows if "step 3" in row.text], [])

    def test_session_without_report_has_no_sync_row(self):
        rows = self.rows_for(PiParser(), PI_DIR + PI_FILE)
        self.assertEqual([row for row in rows if "bug report" in row.text], [])

    def test_missing_values_print_dash(self):
        path = self.home / self.REL
        records = [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines()]
        records[2]["data"] = {"delivery": "upload"}
        records[3]["data"] = {"delivery": "zip"}
        _jsonl(path, records)
        texts = [row.text for row in self.rows_for(PiParser(), self.REL)]
        self.assertIn(
            "session synced: bug report - uploaded to radius.pi.dev transcript=- summary=- "
            + self.SOURCE,
            texts,
        )
        self.assertIn("bug report exported: - path=- " + self.SOURCE, texts)

    def test_truncated_line_is_reported_not_fatal(self):
        row = test_codex.CodexTests.check_bad_line(self, PiParser, self.REL, 3)
        self.assertEqual((row.session_id, row.project_path), (PI_BUG_SESSION, "/srv/proj"))

    def test_sessions_synced(self):
        import contextlib

        out = self.tmp / "out"
        with contextlib.redirect_stdout(io.StringIO()):
            cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), "--agent", "pi"])
        synced = {row["session_id"]: row["synced"] for row in read_jsonl(out / "sessions.jsonl")}
        self.assertEqual((synced[PI_BUG_SESSION], synced[PI_SESSION]), ("yes", "no"))
