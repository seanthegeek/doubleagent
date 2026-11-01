"""Tests of the pi parser."""

import base64
import io
import json
import unittest

from test_errors import CAN_DENY, DENY_REASON

from doubleagent import cli
from doubleagent.dedup import deduplicate
from doubleagent.inputs import Artifact, Home
from doubleagent.parsers import Options
from doubleagent.parsers.pi import PiParser
from doubleagent.parsers.pi import args_summary as pi_args_summary
from doubleagent.timeutil import to_utc
from fixtures import (
    BAD_LINE,
    LC_FILE,
    LC_SESSION,
    PI_BUG_FILE,
    PI_BUG_REPORT,
    PI_BUG_SESSION,
    PI_BUG_SUMMARY,
    PI_BUG_SUMMARY_FILE,
    PI_BUG_ZIP,
    PI_BUG_ZIP_FILE,
    PI_CRASHES,
    PI_DEBUG_LOG,
    PI_DELETED_PROMPT,
    PI_DIR,
    PI_EXPERIMENTAL,
    PI_FILE,
    PI_FORK,
    PI_FORK_FILE,
    PI_FORK_HTML,
    PI_FORK_JSONL,
    PI_GONE_HTML,
    PI_GONE_JSONL,
    PI_GONE_PROMPT,
    PI_GONE_SESSION,
    PI_HTML,
    PI_KINDS_FILE,
    PI_KINDS_SESSION,
    PI_MCP_LOG,
    PI_SESSION,
    PI_SETTINGS,
    PI_TRUST,
    PI_TUI_CRASH,
    PI_V1_FILE,
    PI_V1_SESSION,
    _json,
    _jsonl,
    bad_line_text,
    build_pi_exports,
    pi_bug_zip_members,
    pi_crash_records,
    pi_debug_log_text,
    pi_gone_records,
    pi_html_export_text,
    pi_session_records,
    pi_tui_crash_text,
    pi_zip_bytes,
    write_bad_line,
)

from . import test_codex
from .base import ParserBase, kept_rows, parse_with, read_jsonl, rel_only, unreadable_rows


class PiTests(ParserBase):
    """Test the Pi parser."""

    def test_debug_log_line_of_unicode_space_is_a_bad_line(self):
        path = self.home / PI_DEBUG_LOG
        clean = self.rows_for(PiParser(), PI_DEBUG_LOG)
        for space in map(chr, (0x00A0, 0x2028, 0x0085)):
            path.write_text(pi_debug_log_text() + " \t\n" + space + "\n", encoding="utf-8")
            rows = self.rows_for(PiParser(), PI_DEBUG_LOG)
            self.assertEqual(len(rows), len(clean) + 1, repr(space))
            self.assertTrue(rows[-1].text.startswith("parser: unparseable line ("), rows[-1].text)
            self.assertEqual(rows[-1].source_line, len(pi_debug_log_text().split("\n")) + 1)

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
            [(row.turn_type, row.text, row.session_id, row.timestamp_utc) for row in rows],
            [("system", "parser: .pi/agent/trust.json is not a JSON object", "", "")],
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

    def test_settings_not_an_object_is_one_row(self):
        for text in ("{not json", "[]"):
            (self.home / PI_SETTINGS).write_text(text, encoding="utf-8")
            rows = self.rows_for(PiParser(), PI_SETTINGS)
            self.assertEqual(
                [(row.turn_type, row.text, row.timestamp_utc, row.source_line) for row in rows],
                [("system", "parser: .pi/agent/settings.json is not a JSON object", "", 0)],
                text,
            )

    def test_empty_settings_gives_no_row_but_empty_trust_does(self):
        # pi reads an empty settings.json as no settings and rejects an empty
        # trust.json.
        (self.home / PI_SETTINGS).write_bytes(b"")
        (self.home / PI_TRUST).write_bytes(b"")
        self.assertEqual(self.rows_for(PiParser(), PI_SETTINGS), [])
        self.assertEqual(
            [row.text for row in self.rows_for(PiParser(), PI_TRUST)],
            ["parser: .pi/agent/trust.json is not a JSON object"],
        )

    def test_byte_order_mark_is_dropped(self):
        # pi drops a leading BOM from trust.json and settings.json.
        (self.home / PI_TRUST).write_text('\ufeff{"/srv/proj": true}', encoding="utf-8")
        (self.home / PI_SETTINGS).write_text(
            '\ufeff{"defaultProjectTrust": "never"}', encoding="utf-8"
        )
        trust = self.rows_for(PiParser(), PI_TRUST)
        settings = self.rows_for(PiParser(), PI_SETTINGS)
        self.assertEqual(
            [row.text for row in trust + settings],
            [
                "permission setting: /srv/proj=true (from .pi/agent/trust.json, file mtime)",
                "permission setting: defaultProjectTrust=never "
                "(from .pi/agent/settings.json, file mtime)",
            ],
        )

    def test_meta_not_an_object_is_one_row(self):
        rel = ".pi/agent/experimental/sessions/%s/meta.json" % PI_EXPERIMENTAL
        (self.home / rel).write_text('["/srv/proj"]', encoding="utf-8")
        rows = self.rows_for(PiParser(), rel)
        self.assertEqual(
            [(row.text, row.session_id, row.source_line) for row in rows],
            [("parser: %s is not a JSON object" % rel, PI_EXPERIMENTAL, 0)],
        )

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
        artifact = self.artifact(PI_DEBUG_LOG)
        self.assertTrue(PiParser().secondary(artifact))
        self.assertEqual(PiParser().secondary_tolerance(artifact), 3600.0)
        self.assertTrue(PiParser.has_secondary)
        self.assertFalse(PiParser().secondary(self.artifact(self.REL)))
        rows = self.rows_for(PiParser(), PI_DEBUG_LOG)
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc, row.source_line) for row in rows],
            [
                (
                    "system",
                    "debug snapshot: /debug wrote 6 message(s) of the open session, which "
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
                ("system", "compactionSummary: the turns before", "2026-10-01T09:01:50.000Z", 14),
                ("system", "branchSummary: the other branch", "2026-10-01T09:02:00.000Z", 15),
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
            [(row.turn_type, row.recovered) for row in survivors],
            [("system", ""), ("user", "yes"), ("system", ""), ("system", "")],
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

    def test_crash_element_not_an_object_is_a_bad_record(self):
        records = [*pi_crash_records(), "boom", *pi_crash_records()]
        _json(self.home / PI_CRASHES, records)
        rows = self.rows_for(PiParser(), PI_CRASHES)
        self.assertEqual(
            [(row.source_line, row.text[:24]) for row in rows],
            [
                (1, "crash: uncaught_exceptio"),
                (3, "crash: uncaught_exceptio"),
                (2, "parser: unparseable reco"),
            ],
        )
        self.assertEqual(rows[-1].text, 'parser: unparseable record 2 (not a JSON object): "boom"')

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
        for rel in (PI_MCP_LOG,):
            artifact = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertFalse(PiParser().wants(artifact), rel)
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

    def test_headerless_file_with_entries_reports_its_bad_lines(self):
        # pi skips a file whose first record is not a header; its bad lines
        # are still rows.
        entries = pi_session_records()[1:3]
        lines = [BAD_LINE, *(json.dumps(record) for record in entries)]
        (self.home / self.REL).write_text("\n".join(lines) + "\n", encoding="utf-8")
        rows = self.rows_for(PiParser(), self.REL)
        self.assertEqual(
            [(row.turn_type, row.source_line, row.text, row.session_id) for row in rows],
            [("system", 1, bad_line_text(), "")],
        )

    def test_entry_kinds(self):
        rows = self.rows_for(PiParser(), PI_DIR + PI_KINDS_FILE)
        self.assertEqual(
            [(row.turn_type, row.text, row.source_line) for row in rows],
            [
                ("system", "session start: version=3 cwd=/srv/proj", 1),
                ("system", "system prompt: You are pi.", 2),
                ("system", "thinking_level_change: high", 3),
                ("system", "model: gpt-5.1-codex provider=openai", 4),
                ("tool_use", "Get-ChildItem", 4),
                ("tool_result", "[error] Tool execution was blocked", 5),
                ("system", "assistant stopReason=error: 429 rate limited", 6),
                ("system", "compaction: tokensBefore=51234: earlier turns", 7),
                ("system", "branch_summary from c1000003: the abandoned branch", 8),
                ("system", "custom_message notes: remember the deadline", 9),
                ("system", "context_edit target=c1000004: blocked", 10),
                ("system", "context_edit target=c1000005: omitted from context", 11),
                ("system", "custom hook: hook text", 12),
                ("system", "assistant stopReason=aborted", 14),
                ("tool_use", "make test", 15),
                ("tool_result", "partial\n[cancelled, truncated]", 15),
            ],
        )
        self.assertEqual((rows[-1].tool_name, rows[-1].tool_use_id), ("bash", "c100000e"))
        self.assertEqual(rows[4].tool_name, "powershell")
        self.assertEqual({row.session_id for row in rows}, {PI_KINDS_SESSION})
        # Not a gate refusal: no permission decision row.
        self.assertFalse(any(row.text.startswith("permission decision:") for row in rows))
        thinking = [
            (row.text, row.source_line)
            for row in self.rows_for(PiParser(), PI_DIR + PI_KINDS_FILE, include_thinking=True)
            if row.turn_type == "thinking"
        ]
        self.assertEqual(thinking, [("[redacted]", 14)])

    def test_version_1_header(self):
        rows = self.rows_for(PiParser(), PI_V1_FILE)
        self.assertEqual(
            [(row.turn_type, row.text, row.session_id, row.timestamp_utc) for row in rows],
            [
                (
                    "system",
                    "session start: version=1 cwd=/srv/proj",
                    PI_V1_SESSION,
                    "2026-09-01T08:00:00.000Z",
                ),
                ("user", "an old prompt", PI_V1_SESSION, "2026-09-01T08:00:05.000Z"),
            ],
        )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_file_is_one_row(self):
        unlocked = parse_with(self, PiParser)
        # (file made unreadable, the session id of its row)
        for rel, session_id in (
            (".pi/agent/trust.json", ""),
            (".pi/agent/settings.json", ""),
            (".pi/agent/pi-debug.log", ""),
            (".pi/agent/experimental/sessions/exp-session-1/meta.json", "exp-session-1"),
            (PI_CRASHES, ""),
            (PI_TUI_CRASH, ""),
            (PI_DIR + PI_KINDS_FILE, ""),
        ):
            rows = parse_with(self, PiParser, rel)
            text = "parser: %s could not be read (Permission denied)" % rel.split("/")[-1]
            self.assertEqual(
                [
                    (
                        row.source_file.endswith(rel),
                        row.source_line,
                        row.timestamp_utc,
                        row.session_id,
                        row.text,
                    )
                    for row in unreadable_rows(rows)
                ],
                [(True, 0, "", session_id, text)],
                rel,
            )
            # Every other file of the parser still gives its rows.
            self.assertEqual(kept_rows(rows, rel), kept_rows(unlocked, rel), rel)

    # ---- project exports, the bug report zip and the terminal UI crash log ----

    def project(self):
        """Return the fixture's project directory, built once."""
        project = self.tmp / "srv/proj"
        if not project.exists():
            build_pi_exports(project)
        return project

    def export_artifact(self, rel, original="/srv/proj"):
        """Return a project-local pi file of the fixture as the collector attributes it."""
        project = self.project()
        home = Home(disk_path=project, original=original, user="alice", host="h1", project=True)
        return Artifact(project / rel, original + "/" + rel, rel, "pi", home)

    def export_rows(self, rel, original="/srv/proj", **options):
        return list(PiParser().parse(self.export_artifact(rel, original), Options(**options)))

    def home_rows(self, rel):
        """Return the rows of a file written into the fixture home after the input was opened."""
        home = self.artifact(self.REL).home
        artifact = Artifact(self.home / rel, "/home/alice/" + rel, rel, "pi", home)
        return list(PiParser().parse(artifact, Options()))

    def test_exports_wanted_and_secondary(self):
        parser = PiParser()
        for rel in (
            PI_HTML,
            PI_GONE_HTML,
            PI_GONE_JSONL,
            PI_BUG_ZIP_FILE,
            PI_BUG_SUMMARY_FILE,
            PI_FORK_HTML,
            PI_FORK_JSONL,
        ):
            artifact = self.export_artifact(rel)
            self.assertTrue(parser.wants(artifact), rel)
            self.assertTrue(parser.secondary(artifact), rel)
            self.assertEqual(parser.secondary_tolerance(artifact), 2.0, rel)
        for rel in (PI_TUI_CRASH, ".pi/agent/pi-crash.log"):
            self.assertTrue(parser.wants(rel_only(rel)), rel)
            self.assertFalse(parser.secondary(rel_only(rel)), rel)
        for rel in (
            "docs/pi-session-x.html",
            "pi-session-x.htm",
            "session-latest.jsonl",
            "session-2026-10-02.json",
            "pi-bug-report-x.tar",
            ".pi/agent/pi-tui-debug.log",
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)

    def test_html_export_rows(self):
        rows = self.export_rows(PI_GONE_HTML)
        mtime = to_utc((self.project() / PI_GONE_HTML).stat().st_mtime)
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc, row.source_line) for row in rows],
            [
                (
                    "system",
                    "session exported: HTML /export of session %s, 3 entries of every branch, "
                    "leaf e1000003 (from %s, file mtime)" % (PI_GONE_SESSION, PI_GONE_HTML),
                    mtime,
                    0,
                ),
                (
                    "system",
                    "session start: version=3 cwd=/srv/proj",
                    "2026-10-02T08:00:00.000Z",
                    0,
                ),
                ("user", PI_GONE_PROMPT, "2026-10-02T08:00:02.000Z", 1),
                ("assistant", "the first answer", "2026-10-02T08:00:05.000Z", 2),
                ("assistant", "the second answer", "2026-10-02T08:01:00.000Z", 3),
            ],
        )
        # No `model:` row, but every assistant row keeps its model.
        self.assertEqual([row.model for row in rows[3:]], ["gpt-5", "gpt-5"])
        for row in rows:
            self.assertEqual(
                (row.agent, row.session_id, row.project_path),
                ("pi", PI_GONE_SESSION, "/srv/proj"),
            )
            self.assertTrue(row.source_file.endswith("/srv/proj/" + PI_GONE_HTML), row)

    def test_html_export_dedup_against_the_session(self):
        primary = self.rows_for(PiParser(), self.REL)
        copies = self.export_rows(PI_HTML)
        self.assertFalse([row for row in copies if row.text.startswith("model: ")])
        kept, dropped = deduplicate(primary, [(row, 2.0) for row in copies])
        # Every row but the export's own is a copy of a session file row.
        model_rows = [row for row in primary if row.text.startswith("model: ")]
        self.assertEqual(len(model_rows), 2)
        self.assertEqual(dropped, {"pi": len(primary) - len(model_rows)})
        self.assertEqual(
            [row.text.split(":")[0] for row in kept[len(primary) :]], ["session exported"]
        )

    def test_export_of_a_relocated_project_still_joins_on_the_session(self):
        primary = self.rows_for(PiParser(), self.REL)
        rows = self.export_rows(PI_HTML, original="/srv/moved")
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})
        self.assertTrue(rows[0].source_file.startswith("/srv/moved/"))
        _, dropped = deduplicate(primary, [(row, 2.0) for row in rows])
        self.assertEqual(dropped, {"pi": len(rows) - 1})

    def test_export_only_and_two_exports_of_one_session(self):
        # No session file holds PI_GONE_SESSION: the HTML export's rows are
        # kept, and the JSONL export's copies of the same entries go.
        html = self.export_rows(PI_GONE_HTML)
        jsonl = self.export_rows(PI_GONE_JSONL)
        kept, dropped = deduplicate([], [(row, 2.0) for row in html + jsonl])
        self.assertEqual(dropped, {"pi": 2})
        texts = [row.text for row in kept]
        for text in (PI_GONE_PROMPT, "the first answer", "the second answer"):
            self.assertEqual(texts.count(text), 1, text)
        self.assertFalse([text for text in texts if text.startswith("model: ")])
        self.assertEqual(
            sorted(row.recovered for row in kept if row.turn_type != "system"), ["yes"] * 3
        )
        # The negative half: a different session's export is never dropped.
        _, dropped = deduplicate(self.rows_for(PiParser(), self.REL), [(row, 2.0) for row in html])
        self.assertEqual(dropped, {})

    def repeated_prompt_records(self, *seconds):
        """Return a session holding the prompt `again` at each given second after 10:00."""
        records = [
            {
                "type": "session",
                "version": 3,
                "id": PI_GONE_SESSION,
                "timestamp": "2026-10-03T10:00:00.000Z",
                "cwd": "/srv/proj",
            }
        ]
        for index, second in enumerate(seconds):
            stamp = "2026-10-03T10:%02d:%02d.000Z" % divmod(second, 60)
            records.append(
                {
                    "type": "message",
                    "id": "f%07d" % index,
                    "parentId": None,
                    "timestamp": stamp,
                    "message": {"role": "user", "content": "again", "timestamp": 0},
                }
            )
        return records

    def join_repeated_prompt(self, primary_seconds, export_seconds):
        """Return the export's `again` rows kept beside a session file holding some of them."""
        _jsonl(self.home / PI_DIR / "repeat.jsonl", self.repeated_prompt_records(*primary_seconds))
        primary = self.home_rows(PI_DIR + "repeat.jsonl")
        page = pi_html_export_text(self.repeated_prompt_records(*export_seconds), None)
        (self.project() / PI_GONE_HTML).write_text(page, encoding="utf-8")
        copies = self.export_rows(PI_GONE_HTML)
        kept, _ = deduplicate(primary, [(row, 2.0) for row in copies])
        return [row.timestamp_utc for row in kept[len(primary) :] if row.text == "again"]

    def test_export_join_is_one_to_one_within_the_window(self):
        early, late = "2026-10-03T10:00:10.000Z", "2026-10-03T10:05:00.000Z"
        # (session file's prompt seconds, export's prompt seconds, export rows kept)
        cases = (
            ((10, 300), (10, 300), []),
            ((300,), (10, 300), [early]),  # the earlier claimant missing
            ((10,), (10, 300), [late]),  # the later claimant missing
            ((10,), (20,), ["2026-10-03T10:00:20.000Z"]),  # outside the two-second window
            ((10,), (11,), []),  # inside it
            ((10,), (9,), []),  # inside it, the export earlier
            ((10,), (7,), ["2026-10-03T10:00:07.000Z"]),  # outside it, earlier
        )
        for primary, export, expected in cases:
            self.assertEqual(
                self.join_repeated_prompt(primary, export), expected, (primary, export)
            )

    def test_fork_export_keeps_the_entries_copied_from_its_parent(self):
        # The parent's rows hold those entries under the parent's id, so the
        # copies join nothing and stay; the fork's own entry is dropped.
        parent = self.rows_for(PiParser(), self.REL)
        fork = self.rows_for(PiParser(), self.FORK)
        for rel in (PI_FORK_HTML, PI_FORK_JSONL):
            copies = self.export_rows(rel)
            self.assertFalse([row for row in copies if row.text.startswith("fork: ")], rel)
            self.assertIn("fix the failing test", [row.text for row in copies], rel)
            kept, _ = deduplicate(parent + fork, [(row, 2.0) for row in copies])
            texts = [row.text for row in kept[len(parent + fork) :]]
            self.assertIn("fix the failing test", texts, rel)
            self.assertNotIn("Trying another way.", texts, rel)

    def test_jsonl_export_rows_and_truncated_line(self):
        rows = self.export_rows(PI_GONE_JSONL)
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc, row.source_line) for row in rows],
            [
                (
                    "system",
                    "session exported: JSONL /export of the current branch of session %s, "
                    "2 entries (from %s, header timestamp)" % (PI_GONE_SESSION, PI_GONE_JSONL),
                    "2026-10-02T08:10:00.000Z",
                    1,
                ),
                ("user", PI_GONE_PROMPT, "2026-10-02T08:00:02.000Z", 2),
                ("assistant", "the second answer", "2026-10-02T08:01:00.000Z", 3),
                ("system", bad_line_text(), "", 4),
            ],
        )
        for row in rows:
            self.assertEqual((row.session_id, row.project_path), (PI_GONE_SESSION, "/srv/proj"))

    def test_jsonl_export_bad_line_mid_file_keeps_the_rows_around_it(self):
        path = self.project() / PI_GONE_JSONL
        lines = path.read_text(encoding="utf-8").split("\n")
        path.write_text("\n".join([lines[0], lines[1], BAD_LINE, lines[2]]), encoding="utf-8")
        rows = self.export_rows(PI_GONE_JSONL)
        self.assertEqual(
            [(row.turn_type, row.source_line) for row in rows],
            [("system", 1), ("user", 2), ("assistant", 4), ("system", 3)],
        )
        self.assertEqual(rows[-1].text, bad_line_text())
        self.assertEqual(rows[-1].session_id, PI_GONE_SESSION)

    def test_jsonl_export_without_header_gives_its_bad_lines_alone(self):
        path = self.project() / PI_GONE_JSONL
        path.write_text(BAD_LINE + "\n", encoding="utf-8")
        rows = self.export_rows(PI_GONE_JSONL)
        self.assertEqual([(row.text, row.session_id) for row in rows], [(bad_line_text(), "")])

    def test_html_export_failures_are_one_row(self):
        path = self.project() / PI_GONE_HTML
        page = pi_html_export_text(pi_gone_records(), "e1000003")
        marker = '<script id="session-data" type="application/json">'
        start = page.index(marker) + len(marker)
        end = page.index("</script>", start)

        def embedded(payload):
            return page[:start] + payload + page[end:]

        def encoded(value):
            return base64.b64encode(json.dumps(value).encode("utf-8")).decode("ascii")

        cases = (
            ("<html></html>", "holds no session data"),
            (page[:start] + "e30=", "session data has no closing tag"),
            (embedded("not base64!"), "session data is not base64 ("),
            (embedded(base64.b64encode(b"{oops").decode("ascii")), "session data is not JSON ("),
            (embedded(encoded([1])), "session data is not a JSON object"),
            (embedded(encoded({"entries": []})), "session data has no session header"),
        )
        for text, message in cases:
            path.write_text(text, encoding="utf-8")
            rows = self.export_rows(PI_GONE_HTML)
            self.assertEqual(len(rows), 1, message)
            self.assertTrue(
                rows[0].text.startswith("parser: %s %s" % (PI_GONE_HTML, message)), rows[0].text
            )
            self.assertEqual((rows[0].source_line, rows[0].timestamp_utc), (0, ""), message)
        records = pi_gone_records()
        broken = {"header": records[0], "entries": [records[1], "boom"], "leafId": None}
        path.write_text(embedded(encoded(broken)), encoding="utf-8")
        rows = self.export_rows(PI_GONE_HTML)
        self.assertEqual(
            [(row.turn_type, row.source_line) for row in rows],
            [("system", 0), ("system", 0), ("user", 1), ("system", 2)],
        )
        self.assertEqual(rows[0].text.split(", leaf ")[1].split(" ")[0], "-")
        self.assertEqual(
            (rows[-1].text, rows[-1].session_id),
            ('parser: unparseable record 2 (not a JSON object): "boom"', PI_GONE_SESSION),
        )
        path.write_text(embedded(encoded({"header": records[0], "entries": {}})), encoding="utf-8")
        rows = self.export_rows(PI_GONE_HTML)
        self.assertEqual(
            rows[-1].text, "parser: %s session data has no entries array" % PI_GONE_HTML
        )

    def test_bug_zip_with_transcript_rows(self):
        rows = self.export_rows(PI_BUG_ZIP_FILE)
        report, crash, user, bad = rows
        self.assertEqual(
            report.text,
            "bug report: %s session=%s transcript=true summary=false messages=1 entries=3 "
            "version=1.4.0 platform=linux/x64 model=openai/gpt-5 "
            "(from %s report.json, createdAt)\n"
            "failed assistant turn b10000aa at 2026-10-01T13:00:20.000Z: stopReason=error "
            "socket hang up" % (PI_BUG_ZIP, PI_BUG_SESSION, PI_BUG_ZIP_FILE),
        )
        self.assertEqual(
            crash.text,
            "crash: fatal_error EPIPE: broken pipe, write version=1.4.0 sessionFile=/home/alice/"
            "%s%s (from %s diagnostics.json, record timestamp)"
            % (PI_DIR, PI_BUG_FILE, PI_BUG_ZIP_FILE),
        )
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.project_path) for row in rows],
            [
                ("2026-10-01T13:00:59.000Z", PI_BUG_SESSION, "/srv/proj"),
                ("2026-10-01T12:58:00.000Z", PI_BUG_SESSION, "/srv/proj"),
                ("2026-10-01T13:00:01.000Z", PI_BUG_SESSION, "/srv/proj"),
                ("", PI_BUG_SESSION, "/srv/proj"),
            ],
        )
        self.assertEqual(
            (user.turn_type, user.text, user.source_line), ("user", "the build hangs", 2)
        )
        self.assertEqual((bad.text, bad.source_line), (bad_line_text(), 4))
        # The hint and the redacted settings are never printed.
        self.assertNotIn("<redacted>", "".join(row.text for row in rows))
        # The transcript copy joins the session file; what only the zip holds stays.
        primary = self.rows_for(PiParser(), PI_DIR + PI_BUG_FILE)
        kept, dropped = deduplicate(primary, [(row, 2.0) for row in rows])
        self.assertEqual(dropped, {"pi": 1})
        self.assertEqual(len(kept), len(primary) + 3)

    def test_bug_zip_with_summary_rows(self):
        rows = self.export_rows(PI_BUG_SUMMARY_FILE)
        self.assertEqual(
            [(row.text.split(" (from ")[0], row.project_path) for row in rows],
            [
                (
                    "bug report: %s session=%s transcript=false summary=true messages=1 "
                    "entries=3 version=1.4.0 platform=linux/x64 model=openai/gpt-5"
                    % (PI_BUG_SUMMARY, PI_BUG_SESSION),
                    "",
                ),
                (
                    "crash: fatal_error EPIPE: broken pipe, write version=1.4.0 sessionFile="
                    "/home/alice/%s%s" % (PI_DIR, PI_BUG_FILE),
                    "/srv/proj",
                ),
                ("bug report summary: %s" % PI_BUG_SUMMARY, ""),
            ],
        )
        self.assertEqual(
            rows[-1].text,
            "bug report summary: %s (from %s summary.md, report.json createdAt)\n"
            "## What the user was doing\nRunning the build."
            % (PI_BUG_SUMMARY, PI_BUG_SUMMARY_FILE),
        )
        self.assertEqual(
            {(row.session_id, row.timestamp_utc) for row in (rows[0], rows[2])},
            {(PI_BUG_SESSION, "2026-10-01T13:02:10.000Z")},
        )

    def test_bug_zip_without_transcript_or_summary(self):
        members = pi_bug_zip_members()
        report = json.loads(members["report.json"])
        report["session"] = {"id": PI_BUG_SESSION, "included": False, "summaryIncluded": False}
        members = {"report.json": json.dumps(report), "diagnostics.json": "{}"}
        (self.project() / PI_BUG_ZIP_FILE).write_bytes(pi_zip_bytes(members))
        rows = self.export_rows(PI_BUG_ZIP_FILE)
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("bug report: %s session=" % PI_BUG_ZIP))
        self.assertIn(" transcript=false summary=false messages=- entries=- ", rows[0].text)
        self.assertEqual((rows[0].session_id, rows[0].project_path), (PI_BUG_SESSION, ""))

    def test_bug_zip_non_object_diagnostics_items_are_skipped(self):
        members = pi_bug_zip_members()
        diagnostics = json.loads(members["diagnostics.json"])
        diagnostics["crashes"].insert(0, "stray")
        diagnostics["assistant"].insert(0, 7)
        members["diagnostics.json"] = json.dumps(diagnostics)
        (self.project() / PI_BUG_ZIP_FILE).write_bytes(pi_zip_bytes(members))
        rows = self.export_rows(PI_BUG_ZIP_FILE)
        self.assertEqual(len([row for row in rows if row.text.startswith("crash: ")]), 1)
        report = rows[0].text
        self.assertEqual(report.count("\nfailed assistant turn "), 1, report)

    def test_bug_zip_session_without_a_header_gives_no_copy_rows(self):
        members = pi_bug_zip_members()
        members["session.jsonl"] = BAD_LINE
        (self.project() / PI_BUG_ZIP_FILE).write_bytes(pi_zip_bytes(members))
        rows = self.export_rows(PI_BUG_ZIP_FILE)
        self.assertNotIn("the build hangs", [row.text for row in rows])
        self.assertEqual(
            [row.turn_type for row in rows if not row.text.startswith(("bug ", "crash: "))],
            ["system"],
        )
        self.assertEqual(len([row for row in rows if row.text.startswith("parser: ")]), 1)

    def test_bug_zip_failures_name_the_file_and_member(self):
        path = self.project() / PI_BUG_ZIP_FILE
        path.write_bytes(b"not a zip")
        rows = self.export_rows(PI_BUG_ZIP_FILE)
        self.assertEqual(len(rows), 1)
        self.assertTrue(
            rows[0].text.startswith("parser: %s is not a zip archive (" % PI_BUG_ZIP_FILE),
            rows[0].text,
        )
        members = pi_bug_zip_members()
        members["report.json"] = "[1]"
        del members["diagnostics.json"]
        path.write_bytes(pi_zip_bytes(members))
        rows = self.export_rows(PI_BUG_ZIP_FILE)
        self.assertEqual(
            [row.text for row in rows if row.text.startswith("parser: ")],
            [
                "parser: %s has no member diagnostics.json" % PI_BUG_ZIP_FILE,
                "parser: %s member report.json is not a JSON object" % PI_BUG_ZIP_FILE,
                bad_line_text(),
            ],
        )
        # The transcript copy still gives its rows.
        self.assertIn("the build hangs", [row.text for row in rows])
        member_failure = "parser: %s member report.json could not be read (" % PI_BUG_ZIP_FILE
        # A member whose stored data is damaged.
        data = bytearray(pi_zip_bytes({"report.json": "{}" * 64, "diagnostics.json": "{}"}))
        offset = data.index(b"report.json") + len(b"report.json")
        data[offset : offset + 8] = b"\xff" * 8
        path.write_bytes(bytes(data))
        rows = self.export_rows(PI_BUG_ZIP_FILE)
        self.assertTrue(
            any(row.text.startswith(member_failure) for row in rows), [row.text for row in rows]
        )
        # An end record whose central directory offset is past its real place.
        data = bytearray(pi_zip_bytes(pi_bug_zip_members()))
        end = data.rindex(b"PK\x05\x06")
        moved = int.from_bytes(data[end + 16 : end + 20], "little") + 1000
        data[end + 16 : end + 20] = moved.to_bytes(4, "little")
        path.write_bytes(bytes(data))
        rows = self.export_rows(PI_BUG_ZIP_FILE)
        self.assertTrue(rows, "rows")
        self.assertTrue(all(row.text.startswith("parser: ") for row in rows), rows)
        self.assertTrue(
            any(
                row.text.startswith(member_failure) or " is not a zip archive (" in row.text
                for row in rows
            ),
            [row.text for row in rows],
        )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_export_is_one_row(self):
        for rel in (PI_HTML, PI_GONE_JSONL, PI_BUG_ZIP_FILE):
            path = self.project() / rel
            path.chmod(0)
            try:
                rows = self.export_rows(rel)
            finally:
                path.chmod(0o644)
            self.assertEqual(
                [(row.text, row.source_line, row.timestamp_utc) for row in rows],
                [("parser: %s could not be read (Permission denied)" % rel, 0, "")],
                rel,
            )

    def test_tui_crash_rows(self):
        rows = self.rows_for(PiParser(), PI_TUI_CRASH)
        self.assertEqual(
            [(row.turn_type, row.timestamp_utc, row.source_line, row.text) for row in rows],
            [
                (
                    "system",
                    "2026-10-01T09:06:30.000Z",
                    1,
                    "tui crash: Terminal width: 40; Line 1 visible width: 52 "
                    "(from .pi/agent/pi-tui-crash.log, Crash at line)\n"
                    "[0] (w=20) fix the failing test\n"
                    "[1] (w=52) a status line from an extension that ran past the edge",
                )
            ],
        )
        self.assertEqual((rows[0].session_id, rows[0].project_path), ("", ""))

    def test_tui_crash_blank_edges_are_not_bad_lines(self):
        path = self.home / PI_TUI_CRASH
        path.write_text("\n  \n" + pi_tui_crash_text() + "\n  \n\n", encoding="utf-8")
        rows = self.rows_for(PiParser(), PI_TUI_CRASH)
        self.assertEqual(len(rows), 1, [row.text for row in rows])
        self.assertTrue(rows[0].text.startswith("tui crash: "), rows[0].text)
        self.assertTrue(
            rows[0].text.endswith("a status line from an extension that ran past the edge"),
            rows[0].text,
        )

    def test_tui_crash_bad_lines_keep_the_record(self):
        path = self.home / PI_TUI_CRASH
        lines = pi_tui_crash_text().split("\n")
        lines.insert(3, "garbage between the header lines")
        path.write_text("stray text\n" + "\n".join(lines), encoding="utf-8")
        rows = self.rows_for(PiParser(), PI_TUI_CRASH)
        self.assertEqual(
            [(row.source_line, row.text.split("\n")[0]) for row in rows],
            [
                (
                    2,
                    "tui crash: Terminal width: 40; Line 1 visible width: 52 "
                    "(from .pi/agent/pi-tui-crash.log, Crash at line)",
                ),
                (1, "parser: unparseable line (before the first Crash at line): stray text"),
                (
                    5,
                    "parser: unparseable line (not a crash header line): "
                    "garbage between the header lines",
                ),
            ],
        )
        # A file cut before its screen lines still gives the record.
        path.write_text("\n".join(lines[:2]), encoding="utf-8")
        rows = self.rows_for(PiParser(), PI_TUI_CRASH)
        self.assertEqual(
            [row.text for row in rows],
            ["tui crash: Terminal width: 40 (from .pi/agent/pi-tui-crash.log, Crash at line)"],
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
