"""Tests of the Claude Code parser."""

import json
import os
import unittest
from typing import ClassVar

from test_errors import CAN_DENY, DENY_REASON

from doubleagent import cli
from doubleagent.dedup import deduplicate
from doubleagent.inputs import Artifact, Home, open_input
from doubleagent.model import Row, summarise
from doubleagent.parsers import Options, by_agent
from doubleagent.parsers.claude_code import (
    PASTE_PATTERN,
    PASTE_WRAPPER_PATTERN,
    ClaudeCodeParser,
    match_form,
    match_tolerance,
    tool_summary,
)
from doubleagent.parsers.claude_desktop import ClaudeDesktopParser as _ClaudeDesktopParser
from fixtures import (
    CLAUDE_295_SESSION,
    CLAUDE_296_SESSION,
    CLAUDE_309_BRIDGE,
    CLAUDE_309_BRIDGE_URL,
    CLAUDE_309_PERSISTED,
    CLAUDE_309_SCRATCH,
    CLAUDE_309_SESSION,
    CLAUDE_309_TOOL_RESULT,
    CLAUDE_309_WORKER,
    CLAUDE_310_SESSION,
    CLAUDE_310_VSCODE_SESSION,
    CLAUDE_BRIDGE_SESSION,
    CLAUDE_MAC_CLEARED_SESSION,
    CLAUDE_MAC_CLOUD_SCRATCH,
    CLAUDE_MAC_LOCAL_SCRATCH,
    CLAUDE_MAC_LONG_PASTE,
    CLAUDE_MAC_PASTE_HASH,
    CLAUDE_MAC_SESSION,
    CLAUDE_MAC_SPAWN_SESSION,
    CLAUDE_MAC_TELEPORT_SESSION,
    CLAUDE_REGISTRY_KEY,
    CLAUDE_SESSION,
    CLAUDE_SETTINGS_MTIME,
    CLAUDE_WORK_SESSION,
    bad_line_text,
    claude_309_history_records,
    claude_310_global_config,
    claude_310_records,
    claude_310_user_settings,
    claude_310_vscode_records,
    claude_mac_cleared_records,
    claude_mac_global_config,
    claude_mac_history_records,
    claude_mac_records,
    claude_mac_spawn_records,
    claude_mac_teleport_records,
    claude_project_local_settings,
    write_bad_line,
)

from .base import ParserBase


class ClaudeCodeTests(ParserBase):
    """Test the Claude Code parser on the fixture transcript and history."""

    def test_paste_markup_takes_ascii_digits_and_whitespace(self):
        self.assertTrue(PASTE_PATTERN.fullmatch("[Pasted text #3 +12 lines]"))
        self.assertIsNone(PASTE_PATTERN.fullmatch("[Pasted text #%s]" % chr(0x0663)))
        self.assertIsNone(PASTE_PATTERN.fullmatch("[Pasted text #3 +1%s lines]" % chr(0x0663)))
        self.assertTrue(PASTE_WRAPPER_PATTERN.fullmatch('<pasted_content\tid="ab12">'))
        for space in map(chr, (0x00A0, 0x2028, 0x0085)):
            tag = '<pasted_content%sid="ab12">' % space
            self.assertIsNone(PASTE_WRAPPER_PATTERN.fullmatch(tag), repr(tag))

    REL = ".claude/projects/-srv-proj/%s.jsonl" % CLAUDE_SESSION
    SUB = ".claude/projects/-srv-proj/%s/subagents/agent-abc.jsonl" % CLAUDE_SESSION
    FORK = ".claude/projects/-srv-proj/%s/subagents/agent-fork1.jsonl" % CLAUDE_SESSION
    # A user line written after the bad line.
    AFTER_CUT: ClassVar[dict] = {
        "type": "user",
        "sessionId": CLAUDE_SESSION,
        "timestamp": "2026-10-01T10:30:00.000Z",
        "message": {"role": "user", "content": "after the cut"},
    }

    # (source_line, turn_type, text) for every row of the fixture session;
    # an empty text is checked in its own test.
    EXPECTED: ClassVar[list[tuple[int, str, str]]] = [
        (2, "system", "session surface: cli"),
        (2, "user", "delete the logs in /var/log please"),
        (3, "system", "model: claude-fable-5-1"),
        (5, "tool_use", "cd /srv/proj && rm -rf /var/log/*.log"),
        (6, "tool_result", "removed 3 files"),
        (7, "system", "prompt queued: delivered at line 8"),
        (8, "user", "also check /tmp"),
        (9, "system", "edited file: /srv/proj/notes.md"),
        (10, "system", "hook: PostToolUse:Bash: lint passed"),
        (12, "assistant", "Done. Three log files were removed."),
        (13, "system", "turn_duration: 4000 ms, 5 messages"),
        (
            14,
            "system",
            "slash command: <command-name>/model</command-name>\n"
            "<command-message>model</command-message>\n<command-args></command-args>",
        ),
        (
            15,
            "system",
            "command output: <local-command-stdout>Set model to Fable</local-command-stdout>",
        ),
        (
            16,
            "system",
            "task notification: <task-notification>\n<task-id>b1</task-id>\n"
            "<status>completed</status>\n</task-notification>",
        ),
        (16, "system", "prompt trigger: promptSource=system turnOrigin=-"),
        (17, "tool_use", "select:WebFetch"),
        (18, "tool_result", "WebFetch"),
        (19, "tool_use", "/srv/proj/big.log"),
        (20, "tool_result", "line 1 of the full output\nline 2 of the full output"),
        (21, "tool_use", "/srv/proj/gone.log"),
        (22, "tool_result", ""),
        (23, "system", "api error: 429 rate_limit: API Error: rate limited"),
        (
            24,
            "system",
            'api_error: {"level":"error","maxRetries":10,"retryAttempt":1,"retryInMs":500}',
        ),
        (25, "system", "interrupted: [Request interrupted by user]"),
        (
            26,
            "system",
            "compaction summary: This session is being continued from a previous "
            "conversation. Summary: logs removed.",
        ),
        (27, "system", "session title: Remove old logs"),
        (29, "system", "session title: log cleanup"),
        (30, "system", "pr-link: https://github.com/x/y/pull/7"),
        (32, "system", "cwd changed: /srv/proj2"),
        (33, "user", "thanks"),
    ]

    def test_secondary(self):
        # The history file is a secondary source, the transcript is not.
        parser = ClaudeCodeParser()
        self.assertTrue(parser.secondary(self.artifact(".claude/history.jsonl")))
        self.assertFalse(parser.secondary(self.artifact(self.REL)))

    def test_rows(self):
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        self.assertEqual(
            [(row.source_line, row.turn_type) for row in rows],
            [(line, turn_type) for line, turn_type, _ in self.EXPECTED],
        )
        for row, (_, _, text) in zip(rows, self.EXPECTED, strict=True):
            if text:
                self.assertEqual(row.text, text)
        for row in rows:
            self.assertEqual((row.host, row.user, row.agent), ("h1", "alice", "claude-code"))
            self.assertEqual(row.session_id, CLAUDE_SESSION)
            self.assertEqual(row.source_file, "/alice/" + self.REL)  # relative to the input root
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
            self.assertEqual(row.git_branch, "main", row)
        by_line = {row.source_line: row for row in rows}
        use = by_line[5]
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.model), ("Bash", "toolu_01", "claude-fable-5-1")
        )
        self.assertEqual(by_line[6].tool_use_id, "toolu_01")
        self.assertEqual(
            (by_line[18].tool_use_id, by_line[20].tool_use_id), ("toolu_02", "toolu_03")
        )
        self.assertEqual(by_line[23].model, "")

    def test_state_rows_take_session_project_and_time(self):
        rows = {row.source_line: row for row in self.rows_for(ClaudeCodeParser(), self.REL)}
        for line in (27, 29, 30):
            self.assertEqual(rows[line].project_path, "/srv/proj")
        # Title records have no timestamp: the latest one seen is used.
        self.assertEqual(rows[27].timestamp_utc, "2026-10-01T10:00:17.000Z")
        # After `relocated`, rows take the new cwd.
        self.assertEqual(rows[32].project_path, "/srv/proj2")
        self.assertEqual(rows[33].project_path, "/srv/proj2")
        self.assertEqual(rows[2].project_path, "/srv/proj")

    def test_duplicates_dropped(self):
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        texts = [row.text for row in rows]
        self.assertEqual(texts.count("pr-link: https://github.com/x/y/pull/7"), 1)
        self.assertEqual(texts.count("session title: Remove old logs"), 1)
        # The enqueue on line 7 is delivered by the attachment on line 8: the
        # prompt text appears once, and the enqueue keeps its own time.
        self.assertEqual(sum("also check /tmp" in text for text in texts), 1)
        queued = next(row for row in rows if row.source_line == 7)
        self.assertEqual(queued.timestamp_utc, "2026-10-01T10:00:03.500Z")
        self.assertEqual(queued.project_path, "/srv/proj")

    def test_undelivered_enqueue_keeps_its_text(self):
        path = self.home / self.REL
        with open(path, "a", encoding="utf-8") as handle:
            handle.write(
                '{"type":"queue-operation","operation":"enqueue",'
                '"timestamp":"2026-10-01T10:00:30.000Z","sessionId":"%s",'
                '"content":"never delivered"}\n' % CLAUDE_SESSION
            )
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        self.assertEqual(rows[-1].text, "queue enqueue: never delivered")
        self.assertEqual(rows[-1].source_line, 35)

    def test_persisted_output(self):
        rows = {row.source_line: row for row in self.rows_for(ClaudeCodeParser(), self.REL)}
        self.assertEqual(rows[20].text, "line 1 of the full output\nline 2 of the full output")
        missing = rows[22].text
        self.assertTrue(missing.startswith("<persisted-output>"), missing)
        self.assertTrue(
            missing.endswith("[persisted output not found: tool-results/f0e1d2c3b.txt]"), missing
        )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_persisted_output_is_noted_and_reported(self):
        results_dir = self.home / ".claude/projects/-srv-proj" / CLAUDE_SESSION / "tool-results"
        stored = results_dir / "b1c2d3e4f.txt"
        stored.chmod(0)
        try:
            parser = ClaudeCodeParser()
            rows = self.rows_for(parser, self.REL)
            history = self.rows_for(parser, ".claude/history.jsonl")
        finally:
            stored.chmod(0o600)
        by_line = {row.source_line: row for row in rows}
        self.assertTrue(by_line[20].text.startswith("<persisted-output>"), by_line[20].text)
        self.assertTrue(
            by_line[20].text.endswith("\n[persisted output could not be read (Permission denied)]"),
            by_line[20].text,
        )
        self.assertEqual(
            [
                (row.turn_type, row.source_line, row.timestamp_utc, row.text, row.session_id)
                for row in rows
                if row.text.startswith("parser:")
            ],
            [
                (
                    "system",
                    0,
                    "",
                    "parser: b1c2d3e4f.txt could not be read (Permission denied)",
                    CLAUDE_SESSION,
                )
            ],
        )
        self.assertEqual(len(history), 2)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_json_file_is_one_row(self):
        for rel in (".claude/settings.json", ".claude/sessions/4242.json", ".claude.json"):
            path = self.home / rel
            path.chmod(0)
            try:
                parser = ClaudeCodeParser()
                rows = self.rows_for(parser, rel)
                session_rows = self.rows_for(parser, self.REL)
            finally:
                path.chmod(0o600)
            name = rel.rsplit("/", 1)[-1]
            self.assertEqual(
                [(row.turn_type, row.source_line, row.timestamp_utc, row.text) for row in rows],
                [("system", 0, "", "parser: %s could not be read (Permission denied)" % name)],
                rel,
            )
            self.assertEqual(
                [(row.source_line, row.turn_type, row.text) for row in session_rows][:3],
                self.EXPECTED[:3],
                rel,
            )

    def test_persisted_output_not_through_symlink(self):
        results_dir = self.home / ".claude/projects/-srv-proj" / CLAUDE_SESSION / "tool-results"
        stored = results_dir / "b1c2d3e4f.txt"
        target = self.tmp / "outside.txt"
        target.write_text("outside", encoding="utf-8")
        stored.unlink()
        try:
            stored.symlink_to(target)
        except OSError:
            self.skipTest("symlinks not available")
        rows = {row.source_line: row for row in self.rows_for(ClaudeCodeParser(), self.REL)}
        self.assertIn("[persisted output not found", rows[20].text)
        self.assertNotIn("outside", rows[20].text)

    def test_thinking_opt_in(self):
        rows = self.rows_for(ClaudeCodeParser(), self.REL, include_thinking=True)
        thinking = [row for row in rows if row.turn_type == "thinking"]
        # The empty block on line 4 (signature only) is skipped.
        self.assertEqual(
            [(row.source_line, row.text) for row in thinking], [(3, "private reasoning")]
        )

    def test_history(self):
        rows = self.rows_for(ClaudeCodeParser(), ".claude/history.jsonl")
        self.assertEqual(len(rows), 2)
        self.assertEqual(rows[0].timestamp_utc, "2026-10-01T10:00:00.000Z")
        self.assertEqual(rows[0].project_path, "/srv/proj")
        self.assertEqual(rows[1].project_path, "/srv/old")
        self.assertTrue(all(row.turn_type == "user" for row in rows))

    def test_history_reports_truncated_line_and_keeps_valid_rows(self):
        # A bad history line is a system row, not silence.
        (self.home / ".claude/history.jsonl").write_text(
            '{"display":"keep before","sessionId":"s1"}\n'
            '{"display":\n'
            '{"display":"keep after","sessionId":"s2"}\n',
            encoding="utf-8",
        )
        rows = self.rows_for(ClaudeCodeParser(), ".claude/history.jsonl")
        self.assertEqual(
            [(row.turn_type, row.source_line, row.text) for row in rows],
            [
                ("user", 1, "keep before"),
                ("user", 3, "keep after"),
                ("system", 2, bad_line_text('{"display":')),
            ],
        )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_history_is_one_row_and_sessions_still_parse(self):
        history = self.home / ".claude/history.jsonl"
        history.chmod(0)
        try:
            parser = ClaudeCodeParser()
            rows = self.rows_for(parser, ".claude/history.jsonl")
            session_rows = self.rows_for(parser, self.REL)
        finally:
            history.chmod(0o600)
        self.assertEqual(
            [(row.turn_type, row.source_line, row.timestamp_utc, row.text) for row in rows],
            [("system", 0, "", "parser: history.jsonl could not be read (Permission denied)")],
        )
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in session_rows][:3],
            self.EXPECTED[:3],
        )

    def test_subagent_file(self):
        rows = self.rows_for(ClaudeCodeParser(), self.SUB)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                (1, "system", "subagent abc of %s" % CLAUDE_SESSION),
                (1, "system", "session surface: cli"),
                (1, "system", "subagent task: Find where the logs are rotated."),
                (2, "system", "model: claude-fable-5-1"),
                (2, "assistant", "By logrotate."),
            ],
        )
        self.assertTrue(all(row.session_id == CLAUDE_SESSION for row in rows))

    def test_fork_subagent_file(self):
        rows = self.rows_for(ClaudeCodeParser(), self.FORK)
        self.assertEqual(
            [(row.source_line, row.turn_type) for row in rows],
            [
                (1, "system"),
                (1, "system"),
                (1, "user"),
                (2, "system"),
                (2, "tool_use"),
                (3, "tool_result"),
                (3, "system"),
            ],
        )
        self.assertEqual(rows[0].text, "subagent fork1 of %s" % CLAUDE_SESSION)
        self.assertEqual(rows[1].text, "session surface: cli")
        # The copied context keeps its rows; the boilerplate block is the task.
        self.assertEqual(rows[2].text, "delete the logs in /var/log please")
        # The fork tracks its own model.
        self.assertEqual(rows[3].text, "model: claude-fable-5-1")
        self.assertTrue(rows[6].text.startswith("subagent task: <fork-boilerplate>"), rows[6].text)

    def test_sidecars_not_wanted(self):
        base = ".claude/projects/-srv-proj/%s/" % CLAUDE_SESSION
        for rel in (
            base + "subagents/agent-abc.meta.json",
            base + "tool-results/b1c2d3e4f.txt",
        ):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertFalse(ClaudeCodeParser().wants(art), rel)

    def test_relocated_config_home(self):
        # CLAUDE_CONFIG_DIR=~/.claude-work: the session and history are parsed.
        rel = ".claude-work/projects/-srv-proj/%s.jsonl" % CLAUDE_WORK_SESSION
        art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
        self.assertEqual(art.agent, "claude-code")
        rows = self.rows_for(ClaudeCodeParser(), rel)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                (1, "system", "session surface: cli"),
                (1, "user", "rotate the work logs"),
                (2, "system", "model: claude-fable-5-1"),
                (2, "assistant", "Rotated."),
            ],
        )
        self.assertTrue(all(row.session_id == CLAUDE_WORK_SESSION for row in rows))
        self.assertTrue(all(row.project_path == "/srv/proj" for row in rows))
        rows = self.rows_for(ClaudeCodeParser(), ".claude-work/history.jsonl")
        self.assertEqual(
            [(row.turn_type, row.session_id) for row in rows], [("user", CLAUDE_WORK_SESSION)]
        )

    def test_claude_code_router_not_wanted(self):
        # .claude-code-router shares the prefix but is not a Claude Code home:
        # the catalog does not claim it and the parser would not want it.
        self.assertFalse(
            any(".claude-code-router" in artifact.rel for artifact in self.col.artifacts)
        )
        home = self.col.artifacts[0].home
        for rel in (".claude-code-router/config.json", ".claude-code-router/history.json"):
            art = Artifact(self.home / rel, rel, rel, "claude-code", home)
            self.assertFalse(ClaudeCodeParser().wants(art), rel)

    def test_tool_summary_shapes(self):
        self.assertEqual(
            tool_summary("Edit", {"file_path": "/a", "old_string": "x", "new_string": "y"}), "/a"
        )
        self.assertEqual(tool_summary("Grep", {"pattern": "foo", "path": "/src"}), "foo | /src")
        self.assertEqual(tool_summary("Unknown", {"z": 1, "a": 2}), '{"a":2,"z":1}')
        self.assertEqual(tool_summary("Unknown", "raw"), '"raw"')

    def test_truncated_last_line_is_reported_not_fatal(self):
        # The bad line is its own row, in full; the rows before
        # it and a line written after it survive.
        write_bad_line(self.home / self.REL)
        with open(self.home / self.REL, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(self.AFTER_CUT) + "\n")
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        self.assertEqual(len(rows), len(self.EXPECTED) + 2)
        self.assertEqual(
            [(row.source_line, row.turn_type) for row in rows[: len(self.EXPECTED)]],
            [(line, turn_type) for line, turn_type, _ in self.EXPECTED],
        )
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[len(self.EXPECTED) :]],
            [(36, "user", "after the cut"), (35, "system", bad_line_text())],
        )
        self.assertEqual(rows[-1].session_id, CLAUDE_SESSION)


class ClaudeCode295Tests(ParserBase):
    """Test the records Claude Code 2.1.295 writes.

    Permission decisions, the state lines that version writes and the per-process
    registry `sessions/<pid>.json`.
    """

    REL = ".claude/projects/-srv-proj/%s.jsonl" % CLAUDE_295_SESSION
    REG = ".claude/sessions/4242.json"
    BRIDGE = ".claude/sessions/4343.json"

    EXPECTED: ClassVar[list[tuple[int, str, str]]] = [
        (1, "system", "prompt queued: delivered at line 4"),
        (2, "system", "queue dequeue:"),
        (4, "system", "session surface: claude-desktop"),
        (4, "system", "permission mode: auto"),
        (4, "user", "list the cron jobs"),
        (4, "system", "prompt trigger: promptSource=sdk turnOrigin=human"),
        (5, "system", "model: claude-fable-5-1"),
        (5, "tool_use", "cd /srv/proj && crontab -l"),
        (6, "tool_result", "0 3 * * * x"),
        (6, "system", "permission decision: allow source=auto_mode reason=classifier"),
        (
            8,
            "system",
            'stop_hook_summary: {"hasOutput":false,"hookAdditionalContext":[],"hookCount":1,'
            '"hookErrors":[],"hookInfos":[{"command":"true","durationMs":5}],'
            '"level":"suggestion","preventedContinuation":false,"stopReason":"",'
            '"toolUseID":"toolu_95"}',
        ),
    ]

    def test_rows(self):
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows], self.EXPECTED
        )
        for row in rows:
            self.assertEqual(row.session_id, CLAUDE_295_SESSION)
            self.assertEqual(row.project_path, "/srv/proj")
            self.assertEqual(row.git_branch, "main")
        # The state lines (snapshot, delta, last-prompt, atis-latch) yield
        # nothing, and no line is reported unreadable.
        self.assertFalse(any(row.text.startswith("parser:") for row in rows))

    def test_permission_decision_row(self):
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        i = next(i for i, row in enumerate(rows) if row.text.startswith("permission decision:"))
        result, perm = rows[i - 1], rows[i]
        self.assertEqual(result.turn_type, "tool_result")
        self.assertEqual(
            (perm.turn_type, perm.tool_use_id, perm.tool_name, perm.timestamp_utc, perm.model),
            ("system", "toolu_95", "", "2026-10-01T11:00:03.000Z", ""),
        )

    def test_permission_decision_extra_and_missing_keys(self):
        path = self.home / self.REL
        with open(path, "a", encoding="utf-8") as handle:
            handle.write(
                json.dumps(
                    {
                        "type": "user",
                        "sessionId": CLAUDE_295_SESSION,
                        "timestamp": "2026-10-01T11:00:09.000Z",
                        "message": {
                            "role": "user",
                            "content": [
                                {"type": "tool_result", "tool_use_id": "toolu_96", "content": "x"},
                                {"type": "tool_result", "tool_use_id": "toolu_97", "content": "y"},
                            ],
                        },
                        "permissionDecision": {"decision": "deny", "ruleId": "r1"},
                    }
                )
                + "\n"
            )
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        self.assertEqual(
            [(row.turn_type, row.tool_use_id) for row in rows[-3:]],
            [("tool_result", "toolu_96"), ("tool_result", "toolu_97"), ("system", "toolu_96")],
        )
        self.assertEqual(
            rows[-1].text,
            'permission decision: deny source=- reason=- {"decision":"deny","ruleId":"r1"}',
        )

    def _append_result(self, tool_use_id: str, **keys):
        record = {
            "type": "user",
            "sessionId": CLAUDE_295_SESSION,
            "timestamp": "2026-10-01T11:00:10.000Z",
            "message": {
                "role": "user",
                "content": [{"type": "tool_result", "tool_use_id": tool_use_id, "content": "no"}],
            },
        }
        record.update(keys)
        with open(self.home / self.REL, "a", encoding="utf-8") as handle:
            handle.write(json.dumps(record) + "\n")

    def test_denial_kind_and_feedback(self):
        # `toolDenialKind` and `userFeedback` on the same line are
        # appended to the decision row, printed as recorded (no value set is
        # in the research; these values are fixture only).
        self._append_result(
            "toolu_95",
            permissionDecision={"decision": "deny", "source": "user", "reasonType": "rule"},
            toolDenialKind="user_rejected",
            userFeedback="use the read-only listing instead",
        )
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        # Every earlier row is unchanged.
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[: len(self.EXPECTED)]],
            self.EXPECTED,
        )
        perm = rows[-1]
        self.assertEqual(
            (perm.turn_type, perm.tool_use_id, perm.timestamp_utc, perm.text),
            (
                "system",
                "toolu_95",
                "2026-10-01T11:00:10.000Z",
                "permission decision: deny source=user reason=rule "
                "toolDenialKind=user_rejected userFeedback=use the read-only listing instead",
            ),
        )

    def test_denial_keys_without_decision(self):
        self._append_result("toolu_95", toolDenialKind="user_rejected")
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        self.assertEqual(
            [(row.turn_type, row.tool_use_id, row.text) for row in rows[-2:]],
            [
                ("tool_result", "toolu_95", "no"),
                (
                    "system",
                    "toolu_95",
                    "permission decision: reject (interrupted) toolDenialKind=user_rejected",
                ),
            ],
        )

    def test_thinking_opt_in(self):
        rows = self.rows_for(ClaudeCodeParser(), self.REL, include_thinking=True)
        self.assertEqual([row for row in rows if row.turn_type == "thinking"], [])
        self.assertEqual(len(rows), len(self.EXPECTED))

    def test_registry_rows(self):
        rows = self.rows_for(ClaudeCodeParser(), self.REG)
        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual(
            (
                row.turn_type,
                row.text,
                row.timestamp_utc,
                row.session_id,
                row.project_path,
                row.model,
                row.source_line,
            ),
            (
                "system",
                "claude-code process 4242 interactive status=idle entrypoint=claude-desktop "
                "bridge=none host_session=local_f4f4f4f4-9999-4aaa-8bbb-cccccccccccc",
                "2026-10-01T11:00:00.000Z",
                CLAUDE_295_SESSION,
                "/srv/proj",
                "",
                1,
            ),
        )
        self.assertEqual(row.source_file, "/alice/" + self.REG)
        # The registry row joins the session's transcript rows.
        sessions = {row.session_id for row in self.rows_for(ClaudeCodeParser(), self.REL)}
        self.assertEqual(sessions, {row.session_id})
        rows = self.rows_for(ClaudeCodeParser(), self.BRIDGE)
        self.assertEqual(
            [(row.text, row.session_id, row.project_path, row.turn_type) for row in rows],
            [
                (
                    "claude-code process 4343 interactive status=busy entrypoint=cli "
                    "bridge=cse_0123456789abcdef host_session=none",
                    CLAUDE_BRIDGE_SESSION,
                    "/srv/proj",
                    "system",
                ),
                (
                    "session synced: remote control bridge cse_0123456789abcdef url - "
                    "(from %s, file mtime)" % self.BRIDGE,
                    CLAUDE_BRIDGE_SESSION,
                    "/srv/proj",
                    "system",
                ),
            ],
        )
        self.assertEqual(rows[0].timestamp_utc, "2026-10-01T12:00:00.000Z")
        # The sync marker is timestamped by the registry file's mtime, never left empty.
        self.assertRegex(rows[1].timestamp_utc, r"^\d{4}-\d{2}-\d{2}T.*Z$")
        # A registry file without a bridge id gives no sync marker.
        self.assertEqual(len(self.rows_for(ClaudeCodeParser(), self.REG)), 1)

    def test_registry_missing_keys(self):
        (self.home / self.REG).write_text('{"pid": 4242}', encoding="utf-8")
        rows = self.rows_for(ClaudeCodeParser(), self.REG)
        self.assertEqual(
            [(row.text, row.timestamp_utc, row.session_id) for row in rows],
            [
                (
                    "claude-code process 4242 - status=- entrypoint=- bridge=none "
                    "host_session=none",
                    "",
                    "",
                )
            ],
        )

    def test_not_wanted(self):
        rel = ".claude/sessions/" + CLAUDE_REGISTRY_KEY
        art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
        self.assertEqual(art.agent, "claude-code")
        self.assertFalse(ClaudeCodeParser().wants(art))

    def test_registry_not_json_object(self):
        for body in ('{"pid": 42', "[1, 2]"):
            (self.home / self.REG).write_text(body, encoding="utf-8")
            rows = self.rows_for(ClaudeCodeParser(), self.REG)
            self.assertEqual(
                [(row.turn_type, row.source_line, row.text) for row in rows],
                [("system", 1, "parser: session registry file is not a JSON object")],
            )

    def test_truncated_line_is_reported_not_fatal(self):
        # The bad line is its own row, in full; the rows before
        # it and a line written after it survive.
        write_bad_line(self.home / self.REL)
        after = dict(ClaudeCodeTests.AFTER_CUT, sessionId=CLAUDE_295_SESSION)
        with open(self.home / self.REL, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(after) + "\n")
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[: len(self.EXPECTED)]],
            self.EXPECTED,
        )
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[len(self.EXPECTED) :]],
            [(12, "user", "after the cut"), (11, "system", bad_line_text())],
        )


class ClaudeCode296Tests(ParserBase):
    """Test the permission mode and model events and the standing permission settings.

    The settings come from settings.json, settings.local.json and .claude.json.
    """

    REL = ".claude/projects/-srv-proj/%s.jsonl" % CLAUDE_296_SESSION
    SETTINGS_TIMESTAMP = "2026-10-01T09:00:00.000Z"

    EXPECTED: ClassVar[list[tuple[int, str, str]]] = [
        (1, "system", "session surface: cli"),
        (1, "system", "permission mode: auto"),
        (1, "user", "plan the log cleanup"),
        (2, "system", "model: claude-fable-5-1"),
        (2, "assistant", "Here is a plan."),
        (4, "system", "permission mode: auto -> plan"),
        (5, "user", "go on"),
        (
            6,
            "system",
            "slash command: <command-name>/model</command-name>\n"
            "<command-message>model</command-message>\n<command-args>opus</command-args>",
        ),
        (
            7,
            "system",
            "command output: <local-command-stdout>Set model to `Opus 5.5` and saved as "
            "your default for new sessions</local-command-stdout>",
        ),
        (8, "system", "model: claude-fable-5-1 -> claude-opus-5-5 requested=opus"),
        (8, "assistant", "Switched."),
        (9, "system", "permission mode: plan -> acceptEdits"),
        (9, "user", "apply it"),
        (10, "assistant", "Applied."),
    ]

    def test_rows(self):
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows], self.EXPECTED
        )
        for row in rows:
            self.assertEqual((row.session_id, row.project_path), (CLAUDE_296_SESSION, "/srv/proj"))

    def test_mode_and_model_columns_and_times(self):
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        by_key = {
            (row.source_line, row.text.split(":")[0]): row
            for row in rows
            if row.turn_type == "system"
        }
        # A user line's mode row takes the line's time; a permission-mode
        # record has none and takes the latest seen.
        self.assertEqual(by_key[1, "permission mode"].timestamp_utc, "2026-10-01T12:00:01.000Z")
        self.assertEqual(by_key[4, "permission mode"].timestamp_utc, "2026-10-01T12:00:02.000Z")
        self.assertEqual(by_key[1, "permission mode"].model, "")
        self.assertEqual(by_key[2, "model"].model, "claude-fable-5-1")
        self.assertEqual(by_key[8, "model"].model, "claude-opus-5-5")
        self.assertEqual(by_key[8, "model"].timestamp_utc, "2026-10-01T12:00:08.000Z")

    def test_subagent_tracks_its_own_mode(self):
        # A subagent file of the session starts its own tracker.
        rel = ".claude/projects/-srv-proj/%s/subagents/agent-m1.jsonl" % CLAUDE_296_SESSION
        path = self.home / rel
        path.parent.mkdir(parents=True)
        lines = [
            {
                "type": "user",
                "sessionId": CLAUDE_296_SESSION,
                "agentId": "m1",
                "timestamp": "2026-10-01T12:00:20.000Z",
                "message": {"role": "user", "content": "check it"},
                "permissionMode": "acceptEdits",
            },
            {
                "type": "assistant",
                "sessionId": CLAUDE_296_SESSION,
                "agentId": "m1",
                "timestamp": "2026-10-01T12:00:21.000Z",
                "message": {
                    "model": "claude-opus-5-5",
                    "content": [{"type": "text", "text": "ok"}],
                },
            },
        ]
        path.write_text("".join(json.dumps(line) + "\n" for line in lines), encoding="utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(ClaudeCodeParser(), rel)
        self.assertEqual(
            [row.text for row in rows],
            [
                "subagent m1 of %s" % CLAUDE_296_SESSION,
                "permission mode: acceptEdits",
                "subagent task: check it",
                "model: claude-opus-5-5",
                "ok",
            ],
        )

    def test_thinking_opt_in(self):
        rows = self.rows_for(ClaudeCodeParser(), self.REL, include_thinking=True)
        self.assertEqual(len(rows), len(self.EXPECTED))

    def test_truncated_line_is_reported_not_fatal(self):
        # The bad line is its own row, in full; the rows before
        # it and a line written after it survive.
        write_bad_line(self.home / self.REL)
        after = dict(ClaudeCodeTests.AFTER_CUT, sessionId=CLAUDE_296_SESSION)
        with open(self.home / self.REL, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(after) + "\n")
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[: len(self.EXPECTED)]],
            self.EXPECTED,
        )
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[len(self.EXPECTED) :]],
            [(12, "user", "after the cut"), (11, "system", bad_line_text())],
        )

    def test_user_settings_rows(self):
        rows = self.rows_for(ClaudeCodeParser(), ".claude/settings.json")
        source = "(from .claude/settings.json, file mtime)"
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: permissions.allow=Bash(npm test:*) " + source,
                "permission setting: permissions.allow=Read(~/notes/**) " + source,
                "permission setting: permissions.deny=Bash(rm -rf:*) " + source,
                "permission setting: permissions.ask=WebFetch " + source,
                "permission setting: permissions.additionalDirectories=/srv/shared " + source,
                "permission setting: permissions.defaultMode=acceptEdits " + source,
                "permission setting: permissions.disableBypassPermissionsMode=disable " + source,
                "model setting: model=opus " + source,
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.turn_type, row.timestamp_utc, row.session_id, row.project_path, row.agent),
                ("system", self.SETTINGS_TIMESTAMP, "", "", "claude-code"),
            )
            # env and apiKeyHelper are never read.
            self.assertNotIn("not-a-real-key", row.text)

    def test_global_config_rows(self):
        rows = self.rows_for(ClaudeCodeParser(), ".claude.json")
        source = "(from .claude.json, file mtime)"
        self.assertEqual(
            [(row.text, row.project_path, row.timestamp_utc) for row in rows],
            [
                (
                    "permission setting: allowedTools=Bash(git status) " + source,
                    "/srv/proj",
                    self.SETTINGS_TIMESTAMP,
                ),
                (
                    "permission setting: hasTrustDialogAccepted=true " + source,
                    "/srv/proj",
                    self.SETTINGS_TIMESTAMP,
                ),
                (
                    "permission setting: hasClaudeMdExternalIncludesApproved=false " + source,
                    "/srv/proj",
                    self.SETTINGS_TIMESTAMP,
                ),
            ],
        )
        for row in rows:
            self.assertNotIn("example.invalid", row.text)
            self.assertNotIn("Authorization", row.text)
        rows = self.rows_for(ClaudeCodeParser(), ".claude-work/.claude.json")
        self.assertEqual(
            [(row.text, row.project_path) for row in rows],
            [
                (
                    "permission setting: allowedTools=Edit "
                    "(from .claude-work/.claude.json, file mtime)",
                    "/srv/work",
                )
            ],
        )

    def test_manifest_mtime_wins(self):
        art = next(
            artifact for artifact in self.col.artifacts if artifact.rel == ".claude/settings.json"
        )
        art.manifest = {"mtime": CLAUDE_SETTINGS_MTIME + 3600}
        rows = list(ClaudeCodeParser().parse(art, Options()))
        self.assertEqual({row.timestamp_utc for row in rows}, {"2026-10-01T10:00:00.000Z"})

    def test_project_local_settings(self):
        # A file the collector found in a discovered project: agent
        # `project`, home the project directory.
        project = self.tmp / "srv" / "proj"
        path = project / ".claude/settings.local.json"
        path.parent.mkdir(parents=True)
        path.write_text(json.dumps(claude_project_local_settings()), encoding="utf-8")
        rel = ".claude/settings.local.json"
        home = Home(disk_path=project, original="/srv/proj", user="alice", host="h1", project=True)
        art = Artifact(path, "/srv/proj/" + rel, rel, "project", home)
        self.assertIn(ClaudeCodeParser, [type(parser) for parser in cli.parsers_for(art, {})])
        self.assertTrue(ClaudeCodeParser().wants(art))
        rows = list(ClaudeCodeParser().parse(art, Options()))
        # A collector that names the agent gives the same rows.
        own = Artifact(path, "/srv/proj/" + rel, rel, "claude-code", home)
        self.assertIn(
            ClaudeCodeParser, [type(parser) for parser in cli.parsers_for(own, by_agent())]
        )
        self.assertEqual(
            [row.as_dict() for row in ClaudeCodeParser().parse(own, Options())],
            [row.as_dict() for row in rows],
        )
        source = "(from .claude/settings.local.json, file mtime)"
        self.assertEqual(
            [(row.text, row.project_path, row.source_file) for row in rows],
            [
                (
                    "permission setting: permissions.allow=Bash(make lint) " + source,
                    "/srv/proj",
                    "/srv/proj/" + rel,
                ),
                (
                    "permission setting: permissions.defaultMode=plan " + source,
                    "/srv/proj",
                    "/srv/proj/" + rel,
                ),
            ],
        )

    def test_settings_without_permissions_and_unreadable(self):
        path = self.home / ".claude/settings.json"
        path.write_text('{"model": "opus", "env": {"X": "y"}}', encoding="utf-8")
        # Only the model alias is read; `env` is never printed.
        self.assertEqual(
            [row.text for row in self.rows_for(ClaudeCodeParser(), ".claude/settings.json")],
            ["model setting: model=opus (from .claude/settings.json, file mtime)"],
        )
        path.write_text('{"env": {"X": "y"}}', encoding="utf-8")
        self.assertEqual(self.rows_for(ClaudeCodeParser(), ".claude/settings.json"), [])
        for body in ("[1]", "{not json"):
            path.write_text(body, encoding="utf-8")
            rows = self.rows_for(ClaudeCodeParser(), ".claude/settings.json")
            self.assertEqual(
                [(row.turn_type, row.text) for row in rows],
                [("system", "parser: settings file is not a JSON object")],
            )

    def test_backups_and_other_files_not_wanted(self):
        for rel in (".claude.json.backup.1790845200000",):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertFalse(ClaudeCodeParser().wants(art), rel)
        home = self.col.artifacts[0].home
        for rel in (
            ".claude/settings.json.bak",
            ".claude/cowork_settings.json",
            ".claude/.credentials.json",
            "proj/.claude/settings.json",
        ):
            art = Artifact(self.home / rel, rel, rel, "claude-code", home)
            self.assertFalse(ClaudeCodeParser().wants(art), rel)


class ClaudeCode309Tests(ParserBase):
    """Test the records Claude Code 2.1.309 writes on Windows.

    The surface, the prompt trigger, the scratchpad, the daemon handover, Remote
    Control as sync, `grantScope`/`reasonCode` and the persisted output's path key.
    """

    REL = ".claude/projects/-srv-proj/%s.jsonl" % CLAUDE_309_SESSION
    SCRATCH_TEXT = (
        "scratch directory: %s (session scratchpad, from the environment attachment)"
        % CLAUDE_309_SCRATCH
    )

    EXPECTED: ClassVar[list[tuple[int, str, str]]] = [
        (2, "system", "permission mode: default"),
        (3, "system", "session surface: cli"),
        (3, "user", "Run python calc.py 2 3"),
        (4, "system", SCRATCH_TEXT),
        (9, "system", "model: claude-sonnet-5-5"),
        (9, "tool_use", "python calc.py 2 3"),
        (10, "tool_result", CLAUDE_309_PERSISTED),
        (
            10,
            "system",
            "permission decision: accept source=user_temporary reason=- "
            '{"decision":"accept","grantScope":"once","reasonCode":"other",'
            '"source":"user_temporary"}',
        ),
        (11, "tool_use", "python calc.py 5 7"),
        (12, "tool_result", "The user doesn't want to proceed with this tool use."),
        (
            12,
            "system",
            "permission decision: reject source=user_reject reason=- "
            '{"decision":"reject","reasonCode":"other","source":"user_reject"} '
            "toolDenialKind=user-rejected userFeedback=use the test runner instead",
        ),
        (13, "system", "bridge_status: /remote-control is active"),
        (14, "system", "remote control: bridge session %s lastSequenceNum=1" % CLAUDE_309_BRIDGE),
        (
            14,
            "system",
            "session synced: remote control bridge %s url %s"
            % (CLAUDE_309_BRIDGE, CLAUDE_309_BRIDGE_URL),
        ),
        (15, "user", "Run python calc.py 3 4"),
        (15, "system", "prompt trigger: promptSource=queued turnOrigin=human"),
        (17, "system", "remote control: bridge session none lastSequenceNum=4"),
        (18, "user", "Say tick in one word"),
        (18, "system", "prompt trigger: promptSource=- turnOrigin=scheduled"),
        (19, "user", "Add a subtract function to calc.py"),
        (19, "system", "prompt trigger: promptSource=suggestion_accepted turnOrigin=human"),
        (20, "system", "session continued in: " + CLAUDE_309_WORKER),
    ]

    def _append(self, *records):
        with open(self.home / self.REL, "a", encoding="utf-8") as handle:
            for record in records:
                handle.write(json.dumps(record) + "\n")

    def test_rows(self):
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows], self.EXPECTED
        )
        for row in rows:
            self.assertEqual(row.session_id, CLAUDE_309_SESSION)
            self.assertEqual(row.git_branch, "main")
        # The request-capture lines, the auto_mode attachment, the repeated
        # bridge record and cost-state yield nothing.
        self.assertFalse({6, 7, 8, 5, 16, 21} & {row.source_line for row in rows})

    def test_rows_take_line_time(self):
        rows = {
            (row.source_line, row.text.split(":")[0]): row
            for row in self.rows_for(ClaudeCodeParser(), self.REL)
        }
        self.assertEqual(rows[3, "session surface"].timestamp_utc, "2026-10-01T13:00:01.000Z")
        self.assertEqual(rows[15, "prompt trigger"].timestamp_utc, "2026-10-01T13:00:08.000Z")
        self.assertEqual(rows[20, "session continued in"].timestamp_utc, "2026-10-01T13:00:11.000Z")
        # bridge-session records have no timestamp: the latest one seen.
        self.assertEqual(rows[14, "session synced"].timestamp_utc, "2026-10-01T13:00:07.000Z")
        self.assertEqual(rows[17, "remote control"].timestamp_utc, "2026-10-01T13:00:08.000Z")

    def test_scratch_directory_row(self):
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        scratch = [row for row in rows if row.text.startswith("scratch directory:")]
        self.assertEqual(len(scratch), 1)
        self.assertEqual(
            (scratch[0].project_path, scratch[0].timestamp_utc),
            (CLAUDE_309_SCRATCH, "2026-10-01T13:00:01.000Z"),
        )
        # Every other row keeps the session's cwd, and so does the session.
        self.assertEqual({row.project_path for row in rows if row not in scratch}, {"/srv/proj"})
        (session,) = summarise(rows)
        self.assertEqual(session.project_path, "/srv/proj")
        # A session whose only path is the scratch row falls back to it.
        (only,) = summarise(scratch)
        self.assertEqual(only.project_path, CLAUDE_309_SCRATCH)

    def test_scratch_directory_in_inventory(self):
        # inventory.json lists the scratchpad under scratch_directories and
        # keeps the session's cwd a project.
        keys = cli.scratch_keys(self.col, self.rows_for(ClaudeCodeParser(), self.REL), self.cat)
        self.assertIn(CLAUDE_309_SCRATCH, keys)
        self.assertNotIn("/srv/proj", keys)

    def test_persisted_output_from_key(self):
        rows = {
            (row.source_line, row.turn_type): row
            for row in self.rows_for(ClaudeCodeParser(), self.REL)
        }
        self.assertEqual(rows[10, "tool_result"].text, CLAUDE_309_PERSISTED)
        (self.home / self.REL).with_suffix("").joinpath(
            "tool-results", CLAUDE_309_TOOL_RESULT
        ).unlink()
        rows = {
            (row.source_line, row.turn_type): row
            for row in self.rows_for(ClaudeCodeParser(), self.REL)
        }
        text = rows[10, "tool_result"].text
        self.assertTrue(text.startswith("<persisted-output>"), text)
        self.assertTrue(
            text.endswith("[persisted output not found: tool-results/%s]" % CLAUDE_309_TOOL_RESULT),
            text,
        )

    def test_persisted_output_key_outside_session_dir(self):
        # A path whose directories are not this session's tool-results is
        # never read.
        self._append(
            {
                "type": "user",
                "sessionId": CLAUDE_309_SESSION,
                "timestamp": "2026-10-01T13:00:30.000Z",
                "message": {
                    "role": "user",
                    "content": [{"type": "tool_result", "tool_use_id": "t9", "content": "stub"}],
                },
                "toolUseResult": {
                    "persistedOutputPath": "C:\\Users\\alice\\other\\tool-results\\"
                    + CLAUDE_309_TOOL_RESULT
                },
            }
        )
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        self.assertEqual(
            rows[-1].text,
            "stub\n[persisted output not found: tool-results/%s]" % CLAUDE_309_TOOL_RESULT,
        )

    def test_bridge_rows_once_per_change(self):
        self._append(
            {
                "type": "bridge-session",
                "bridgeSessionId": CLAUDE_309_BRIDGE,
                "lastSequenceNum": 6,
                "sessionId": CLAUDE_309_SESSION,
            },
        )
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        texts = [row.text for row in rows]
        self.assertEqual(sum(text.startswith("session synced:") for text in texts), 1)
        self.assertEqual(
            texts[-1], "remote control: bridge session %s lastSequenceNum=6" % CLAUDE_309_BRIDGE
        )
        # No owner UUID is printed.
        self.assertFalse(any("00000000-0000-4000-8000" in text for text in texts))

    def test_second_bridge_takes_its_own_status_url(self):
        # A detach, a second bridge and its own status record: the second sync
        # row names the second URL, never the first bridge's.
        second = "cse_9f8e7d6c5b4a3928"
        second_url = "https://claude.ai/code/session_9f8e7d6c5b4a3928"
        self._append(
            {"type": "bridge-session", "bridgeSessionId": None, "sessionId": CLAUDE_309_SESSION},
            {
                "type": "bridge-session",
                "bridgeSessionId": second,
                "lastSequenceNum": 1,
                "sessionId": CLAUDE_309_SESSION,
            },
            {
                "type": "system",
                "subtype": "bridge_status",
                "content": "/remote-control is active",
                "url": second_url,
                "sessionId": CLAUDE_309_SESSION,
                "timestamp": "2026-10-01T13:05:00.000Z",
                "uuid": "r9",
            },
        )
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        synced = [row.text for row in rows if row.text.startswith("session synced:")]
        self.assertEqual(
            synced,
            [
                "session synced: remote control bridge %s url %s"
                % (CLAUDE_309_BRIDGE, CLAUDE_309_BRIDGE_URL),
                "session synced: remote control bridge %s url %s" % (second, second_url),
            ],
        )

    def test_second_bridge_without_status_has_no_url(self):
        # The first bridge's status record is not reused for a second bridge
        # that has none of its own.
        second = "cse_9f8e7d6c5b4a3928"
        self._append(
            {"type": "bridge-session", "bridgeSessionId": None, "sessionId": CLAUDE_309_SESSION},
            {
                "type": "bridge-session",
                "bridgeSessionId": second,
                "lastSequenceNum": 1,
                "sessionId": CLAUDE_309_SESSION,
            },
        )
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        synced = [row.text for row in rows if row.text.startswith("session synced:")]
        self.assertEqual(synced[-1], "session synced: remote control bridge %s url -" % second)
        self.assertEqual(len(synced), 2)

    def test_sync_flag_and_sessions(self):
        self.assertTrue(ClaudeCodeParser.records_sync)
        self.assertFalse(_ClaudeDesktopParser.records_sync)
        parser = ClaudeCodeParser()
        rows = self.rows_for(parser, self.REL) + self.rows_for(
            parser, ".claude/projects/-srv-proj/%s.jsonl" % CLAUDE_296_SESSION
        )
        synced = {
            session.session_id: session.synced
            for session in summarise(rows, frozenset({"claude-code"}))
        }
        self.assertEqual(synced, {CLAUDE_309_SESSION: "yes", CLAUDE_296_SESSION: "no"})
        # A session known only from its registry file, whose bridge id is the one
        # trace of Remote Control left, is synced too; one without a bridge id is not.
        rows = self.rows_for(parser, ClaudeCode295Tests.BRIDGE) + self.rows_for(
            parser, ClaudeCode295Tests.REG
        )
        synced = {
            session.session_id: session.synced
            for session in summarise(rows, frozenset({"claude-code"}))
        }
        self.assertEqual(synced, {CLAUDE_BRIDGE_SESSION: "yes", CLAUDE_295_SESSION: "no"})

    def test_surface_change(self):
        self._append(
            {
                "type": "user",
                "sessionId": CLAUDE_309_SESSION,
                "entrypoint": "claude-vscode",
                "timestamp": "2026-10-01T13:00:30.000Z",
                "message": {"role": "user", "content": "from the panel"},
                "promptSource": "sdk",
                "turnOrigin": "human",
            }
        )
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        self.assertEqual(
            [(row.source_line, row.text) for row in rows[-4:]],
            [
                (22, "session surface: cli -> claude-vscode"),
                (22, "session origin: local (VS Code extension)"),
                (22, "from the panel"),
                (22, "prompt trigger: promptSource=sdk turnOrigin=human"),
            ],
        )

    def test_vscode_extension_session(self):
        # The extension writes the same tree with entrypoint claude-vscode,
        # promptSource sdk and an environment attachment with `changes`.
        session_id = "11111111-2222-4333-8444-cccccccccccc"
        rel = ".claude/projects/-srv-proj/%s.jsonl" % session_id
        scratch = "/tmp/claude-1000/-srv-proj/%s/scratchpad" % session_id
        common = {
            "sessionId": session_id,
            "cwd": "/srv/proj",
            "gitBranch": "main",
            "version": "2.1.309",
            "entrypoint": "claude-vscode",
            "isSidechain": False,
            "userType": "external",
        }
        lines = [
            dict(
                common,
                type="user",
                uuid="v1",
                timestamp="2026-10-01T14:00:00.000Z",
                message={"role": "user", "content": "Run python calc.py 6 1"},
                origin={"kind": "human"},
                promptSource="sdk",
                turnOrigin="human",
                permissionMode="auto",
            ),
            dict(
                common,
                type="attachment",
                uuid="v2",
                timestamp="2026-10-01T14:00:00.000Z",
                attachment={
                    "type": "environment",
                    "changes": {},
                    "snapshot": {"scratchpadDirectory": scratch, "platform": "linux"},
                },
            ),
        ]
        path = self.home / rel
        path.write_text("".join(json.dumps(line) + "\n" for line in lines), encoding="utf-8")
        art = Artifact(path, rel, rel, "claude-code", self.artifact(self.REL).home)
        rows = list(ClaudeCodeParser().parse(art, Options()))
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                (1, "system", "session surface: claude-vscode"),
                (1, "system", "session origin: local (VS Code extension)"),
                (1, "system", "permission mode: auto"),
                (1, "user", "Run python calc.py 6 1"),
                (1, "system", "prompt trigger: promptSource=sdk turnOrigin=human"),
                (
                    2,
                    "system",
                    "scratch directory: %s (session scratchpad, from the environment"
                    " attachment)" % scratch,
                ),
            ],
        )
        self.assertEqual(rows[-1].project_path, scratch)

    def test_history_pasted_contents(self):
        # The paste is named by its hash and no `paste-cache/<hash>.txt` is
        # on disk, so the row keeps the placeholder as written.
        path = self.home / ".claude/history.jsonl"
        path.write_text(
            "".join(json.dumps(record) + "\n" for record in claude_309_history_records()),
            encoding="utf-8",
        )
        rows = self.rows_for(ClaudeCodeParser(), ".claude/history.jsonl")
        self.assertEqual(
            [(row.turn_type, row.session_id, row.text) for row in rows],
            [
                (
                    "user",
                    CLAUDE_309_SESSION,
                    "[Pasted text #1 +40 lines] Summarise the text above in one sentence",
                )
            ],
        )

    def test_tool_results_not_wanted(self):
        rel = ".claude/projects/-srv-proj/%s/tool-results/%s" % (
            CLAUDE_309_SESSION,
            CLAUDE_309_TOOL_RESULT,
        )
        self.assertFalse(ClaudeCodeParser().wants(self.artifact(rel)))

    def test_thinking_opt_in(self):
        rows = self.rows_for(ClaudeCodeParser(), self.REL, include_thinking=True)
        self.assertEqual([row for row in rows if row.turn_type == "thinking"], [])
        self.assertEqual(len(rows), len(self.EXPECTED))

    def test_truncated_line_is_reported_not_fatal(self):
        write_bad_line(self.home / self.REL)
        after = dict(ClaudeCodeTests.AFTER_CUT, sessionId=CLAUDE_309_SESSION)
        with open(self.home / self.REL, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(after) + "\n")
        rows = self.rows_for(ClaudeCodeParser(), self.REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[: len(self.EXPECTED)]],
            self.EXPECTED,
        )
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[len(self.EXPECTED) :]],
            [(23, "user", "after the cut"), (22, "system", bad_line_text())],
        )
        self.assertEqual(rows[-1].session_id, CLAUDE_309_SESSION)


class ClaudeCode310Tests(ParserBase):
    """Test the records Claude Code 2.1.310 writes on Linux.

    The rule-miss `reasonCode`, a compaction's re-emitted state records, a detached
    head, the extension's `queue-operation` without `content`, the retention setting
    and the GitHub connection cache in `.claude.json`.
    """

    REL = ".claude/projects/-srv-proj/%s.jsonl" % CLAUDE_310_SESSION
    VSCODE_REL = ".claude/projects/-srv-proj/%s.jsonl" % CLAUDE_310_VSCODE_SESSION
    SETTINGS_TIMESTAMP = "2026-10-01T09:00:00.000Z"
    MISS = '"reasonCode":"bashMiss:no-rule-match"'

    EXPECTED: ClassVar[list[tuple[int, str, str]]] = [
        (2, "system", "permission mode: auto"),
        (3, "system", "session surface: cli"),
        (3, "system", "permission mode: auto -> default"),
        (3, "user", "Run python calc.py 2 3"),
        (4, "system", "model: claude-sonnet-5-5"),
        (4, "tool_use", "python3 calc.py 2 3"),
        (5, "tool_result", "5"),
        (
            5,
            "system",
            "permission decision: accept source=user_temporary reason=- "
            '{"decision":"accept","grantScope":"once",' + MISS + ',"source":"user_temporary"}',
        ),
        (6, "tool_use", "python3 calc.py 5 7"),
        (7, "tool_result", "The user doesn't want to proceed with this tool use."),
        (
            7,
            "system",
            "permission decision: reject source=user_reject reason=- "
            '{"decision":"reject",' + MISS + ',"source":"user_reject"} '
            "toolDenialKind=user-rejected userFeedback=I changed my mind",
        ),
        (8, "tool_use", "python3 calc.py 1 1"),
        (9, "tool_result", "2"),
        (
            9,
            "system",
            "permission decision: accept source=user_permanent reason=- "
            '{"decision":"accept",' + MISS + ',"source":"user_permanent"}',
        ),
        (10, "system", "session title: Run calc.py"),
        (11, "system", "compact_boundary: Conversation compacted"),
        (12, "system", "compaction summary: We ran calc.py three times."),
        (24, "user", "What git branch is checked out?"),
        (27, "assistant", "HEAD is detached."),
        (28, "system", "turn_duration: 4000 ms, 3 messages"),
    ]

    def _write(self, rel, records):
        path = self.home / rel
        path.write_text("".join(json.dumps(record) + "\n" for record in records), encoding="utf-8")
        home = self.artifact(".claude/settings.json").home
        return Artifact(path, rel, rel, "claude-code", home)

    def _rows(self, rel, records, **options):
        return list(ClaudeCodeParser().parse(self._write(rel, records), Options(**options)))

    def test_rows(self):
        rows = self._rows(self.REL, claude_310_records())
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows], self.EXPECTED
        )
        for row in rows:
            self.assertEqual(row.session_id, CLAUDE_310_SESSION)

    def test_compaction_copies_yield_nothing(self):
        # The attachments a compaction re-injects and the session-state records
        # it re-emits with unchanged values (lines 13 to 23), the plan
        # re-entry and the deferred-tools delta yield no row: one title row,
        # one permission mode row per real change.
        rows = self._rows(self.REL, claude_310_records())
        self.assertFalse(set(range(13, 24)) & {row.source_line for row in rows})
        self.assertFalse({25, 26} & {row.source_line for row in rows})
        texts = [row.text for row in rows]
        self.assertEqual(texts.count("session title: Run calc.py"), 1)
        self.assertEqual(
            [text for text in texts if text.startswith("permission mode:")],
            ["permission mode: auto", "permission mode: auto -> default"],
        )

    def test_mode_change_after_compaction_still_tracked(self):
        # A real change after the re-emitted copies is still a row.
        records = claude_310_records()
        records.append(
            {"type": "permission-mode", "permissionMode": "plan", "sessionId": CLAUDE_310_SESSION}
        )
        rows = self._rows(self.REL, records)
        self.assertEqual(
            (rows[-1].source_line, rows[-1].text), (29, "permission mode: default -> plan")
        )

    def test_detached_head_branch(self):
        # gitBranch `HEAD` on a detached head is passed through as the branch.
        rows = self._rows(self.REL, claude_310_records())
        self.assertEqual(
            {row.git_branch for row in rows if row.source_line >= 24},
            {"HEAD"},
        )
        self.assertEqual({row.git_branch for row in rows if row.source_line < 24}, {"main"})

    def test_vscode_session(self):
        # The extension's queue-operation lines have no `content`, the file
        # has no mode lines, and cost-state yields nothing.
        rows = self._rows(self.VSCODE_REL, claude_310_vscode_records())
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                (1, "system", "queue enqueue:"),
                (2, "system", "queue dequeue:"),
                (3, "system", "session surface: claude-vscode"),
                (3, "system", "session origin: local (VS Code extension)"),
                (3, "system", "permission mode: auto"),
                (3, "user", "Run python3 calc.py 6 1"),
                (3, "system", "prompt trigger: promptSource=sdk turnOrigin=human"),
            ],
        )

    def test_retention_setting(self):
        path = self.home / ".claude/settings.json"
        path.write_text(json.dumps(claude_310_user_settings()), encoding="utf-8")
        os.utime(path, (CLAUDE_SETTINGS_MTIME, CLAUDE_SETTINGS_MTIME))
        rows = self.rows_for(ClaudeCodeParser(), ".claude/settings.json")
        self.assertEqual(
            [
                (row.turn_type, row.timestamp_utc, row.session_id, row.project_path, row.text)
                for row in rows
            ],
            [
                (
                    "system",
                    self.SETTINGS_TIMESTAMP,
                    "",
                    "",
                    "retention setting: cleanupPeriodDays=1 "
                    "(from .claude/settings.json, file mtime)",
                ),
                (
                    "system",
                    self.SETTINGS_TIMESTAMP,
                    "",
                    "",
                    "model setting: model=opus (from .claude/settings.json, file mtime)",
                ),
            ],
        )

    def test_retention_setting_after_permission_rows(self):
        path = self.home / ".claude/settings.json"
        doc = dict(claude_310_user_settings(), permissions={"allow": ["Bash(python3 *)"]})
        path.write_text(json.dumps(doc), encoding="utf-8")
        rows = self.rows_for(ClaudeCodeParser(), ".claude/settings.json")
        source = "(from .claude/settings.json, file mtime)"
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: permissions.allow=Bash(python3 *) " + source,
                "retention setting: cleanupPeriodDays=1 " + source,
                "model setting: model=opus " + source,
            ],
        )

    def test_source_host_row(self):
        path = self.home / ".claude.json"
        path.write_text(json.dumps(claude_310_global_config()), encoding="utf-8")
        os.utime(path, (CLAUDE_SETTINGS_MTIME, CLAUDE_SETTINGS_MTIME))
        rows = self.rows_for(ClaudeCodeParser(), ".claude.json")
        self.assertEqual(
            [(row.text, row.project_path, row.session_id, row.timestamp_utc) for row in rows],
            [
                (
                    "permission setting: hasTrustDialogAccepted=true "
                    "(from .claude.json, file mtime)",
                    "/srv/proj",
                    "",
                    self.SETTINGS_TIMESTAMP,
                ),
                (
                    "source host: github status=connected (from .claude.json, file mtime)",
                    "",
                    "",
                    self.SETTINGS_TIMESTAMP,
                ),
            ],
        )
        for row in rows:
            # The account and organisation UUIDs, the OAuth account and
            # githubRepoPaths are never printed.
            self.assertNotIn("00000000-0000-4000-8000", row.text)
            self.assertNotIn("example.invalid", row.text)
            self.assertNotIn("example/logs", row.text)

    def test_source_host_without_projects_or_status(self):
        path = self.home / ".claude.json"
        cache = {"accountUuid": "00000000-0000-4000-8000-0000000000aa"}
        path.write_text(json.dumps({"githubWebConnectionStatusCache": cache}), encoding="utf-8")
        rows = self.rows_for(ClaudeCodeParser(), ".claude.json")
        self.assertEqual(
            [row.text for row in rows],
            ["source host: github status=- (from .claude.json, file mtime)"],
        )
        path.write_text(json.dumps({"githubWebConnectionStatusCache": "connected"}), "utf-8")
        self.assertEqual(self.rows_for(ClaudeCodeParser(), ".claude.json"), [])

    def test_history_inline_paste(self):
        # A short paste is kept inline (`content`) in pastedContents; the row
        # is the display text with the placeholder replaced by the paste, as
        # the transcript's prompt holds it.
        line = {
            "display": "[Pasted text #1 +3 lines] Summarise the text above in one sentence",
            "pastedContents": {"1": {"content": "one\ntwo\nthree", "id": 1, "type": "text"}},
            "timestamp": 1790935200000,
            "project": "/srv/proj",
            "sessionId": CLAUDE_310_SESSION,
        }
        (self.home / ".claude/history.jsonl").write_text(json.dumps(line) + "\n", "utf-8")
        rows = self.rows_for(ClaudeCodeParser(), ".claude/history.jsonl")
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("user", "one\ntwo\nthree Summarise the text above in one sentence")],
        )

    def test_debug_log_and_key_not_wanted(self):
        home = self.artifact(".claude/settings.json").home
        for rel in (
            ".claude/debug/%s.txt" % CLAUDE_310_SESSION,
            ".claude/debug/latest",
            ".claude/sessions/4242." + "ab" * 32 + ".key",
            ".claude/projects/-srv-proj/%s/subagents/agent-a1.meta.json" % CLAUDE_310_SESSION,
        ):
            art = Artifact(self.home / rel, rel, rel, "claude-code", home)
            self.assertFalse(ClaudeCodeParser().wants(art), rel)

    def test_thinking_opt_in(self):
        rows = self._rows(self.REL, claude_310_records(), include_thinking=True)
        self.assertEqual([row for row in rows if row.turn_type == "thinking"], [])
        self.assertEqual(len(rows), len(self.EXPECTED))

    def test_truncated_line_is_reported_not_fatal(self):
        art = self._write(self.REL, claude_310_records())
        write_bad_line(art.disk_path)
        after = dict(ClaudeCodeTests.AFTER_CUT, sessionId=CLAUDE_310_SESSION)
        with open(art.disk_path, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(after) + "\n")
        rows = list(ClaudeCodeParser().parse(art, Options()))
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[: len(self.EXPECTED)]],
            self.EXPECTED,
        )
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[len(self.EXPECTED) :]],
            [(30, "user", "after the cut"), (29, "system", bad_line_text())],
        )
        self.assertEqual(rows[-1].session_id, CLAUDE_310_SESSION)


class ClaudeCodeMacTests(ParserBase):
    """Test the records Claude Code 2.1.310 writes on macOS.

    Denials recorded as an interrupt with no decision, history lines that match
    their transcript records across the command and paste forms, the origin rows of
    a Remote Control spawn, a teleported cloud session and the VS Code extension,
    `ccr-tip.json` and `bridge-pointer.json`, the `.claude.json` Remote Control and
    cloud keys, and the scheduled-task firing.
    """

    PROJECT = ".claude/projects/-srv-proj/"
    REL = PROJECT + "%s.jsonl" % CLAUDE_MAC_SESSION
    SPAWN_REL = PROJECT + "%s.jsonl" % CLAUDE_MAC_SPAWN_SESSION
    TELEPORT_REL = PROJECT + "%s.jsonl" % CLAUDE_MAC_TELEPORT_SESSION
    CCR_REL = PROJECT + "%s/ccr-tip.json" % CLAUDE_MAC_SPAWN_SESSION
    POINTER_REL = PROJECT + "bridge-pointer.json"
    MTIME_TIMESTAMP = "2026-10-01T09:00:00.000Z"
    DENIED = "permission decision: reject (interrupted) toolDenialKind=user-rejected"
    REFUSED = "The user doesn't want to proceed with this tool use."

    EXPECTED: ClassVar[list[tuple[int, str, str]]] = [
        (1, "system", "permission mode: default"),
        (2, "system", "session surface: cli"),
        (2, "user", "run python3 calc.py 2 3"),
        (3, "system", "model: claude-fable-5-1"),
        (3, "tool_use", "python3 calc.py 2 3"),
        (4, "tool_result", REFUSED),
        (4, "system", DENIED),
        (5, "user", "Do not run it again."),
        (6, "tool_use", '{"plan":"1. add subtract"}'),
        (7, "tool_result", REFUSED),
        (7, "system", DENIED),
        (
            8,
            "system",
            "slash command: <command-name>/model</command-name>\n"
            "            <command-message>model</command-message>\n"
            "            <command-args></command-args>",
        ),
        (
            9,
            "system",
            "command output: <local-command-stdout>Set model to Fable 5.1 and saved as "
            "your default for new sessions</local-command-stdout>",
        ),
        (
            10,
            "user",
            '<pasted_content id="0bb3">\none\ntwo\nthree\nfour\n</pasted_content>'
            " Summarise the pasted text in one sentence.",
        ),
        (11, "system", "scheduled task fired: a1b2c3d4 cron=*/1 * * * *"),
        (12, "user", "echo hi"),
        (12, "system", "prompt trigger: promptSource=system turnOrigin=scheduled"),
        (
            16,
            "system",
            "slash command: <command-name>/rename</command-name>\n"
            "            <command-message>rename</command-message>\n"
            "            <command-args>lab-round</command-args>",
        ),
        (
            17,
            "user",
            '<pasted_content id="a91f">\n'
            + CLAUDE_MAC_LONG_PASTE
            + "\n</pasted_content> Summarise again.",
        ),
        (
            18,
            "system",
            "slash command: <command-name>/model</command-name>\n"
            "            <command-message>model</command-message>\n"
            "            <command-args></command-args>",
        ),
        (
            20,
            "system",
            "local_command: <command-name>/help</command-name>\n"
            "            <command-message>help</command-message>\n"
            "            <command-args></command-args>",
        ),
    ]
    # The history lines no transcript record matches.
    HISTORY_ONLY: ClassVar[list[str]] = ["/clear", "/logout", "/exit"]

    def _write(self, rel, records):
        path = self.home / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        if isinstance(records, str):
            path.write_text(records, encoding="utf-8")
        else:
            path.write_text(
                "".join(json.dumps(record) + "\n" for record in records), encoding="utf-8"
            )
        os.utime(path, (CLAUDE_SETTINGS_MTIME, CLAUDE_SETTINGS_MTIME))
        home = self.artifact(".claude/settings.json").home
        return Artifact(path, rel, rel, "claude-code", home)

    def _rows(self, rel, records, **options):
        return list(ClaudeCodeParser().parse(self._write(rel, records), Options(**options)))

    def _history(self):
        self._write(".claude/paste-cache/%s.txt" % CLAUDE_MAC_PASTE_HASH, CLAUDE_MAC_LONG_PASTE)
        return self._rows(".claude/history.jsonl", claude_mac_history_records())

    def test_rows(self):
        rows = self._rows(self.REL, claude_mac_records())
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows], self.EXPECTED
        )
        for row in rows:
            self.assertEqual(row.session_id, CLAUDE_MAC_SESSION)
        # The denial rows join the denied calls.
        self.assertEqual(
            [row.tool_use_id for row in rows if row.text == self.DENIED], ["toolu_m1", "toolu_m2"]
        )

    def test_denial_with_feedback_keeps_its_form(self):
        # A denial kind beside a decision record (the Linux form) is not an
        # interrupt; only the kind alone is.
        records = claude_mac_records()
        records[3] = dict(
            records[3],
            permissionDecision={"decision": "reject", "source": "user_reject"},
            userFeedback="no",
        )
        rows = self._rows(self.REL, records)
        self.assertEqual(
            [row.text for row in rows if row.source_line == 4][1],
            "permission decision: reject source=user_reject reason=- "
            "toolDenialKind=user-rejected userFeedback=no",
        )

    def test_history_expands_pastes(self):
        rows = self._history()
        self.assertEqual(
            [row.text for row in rows],
            [
                "run python3 calc.py 2 3",
                "/model",
                "one\ntwo\nthree\nfour Summarise the pasted text in one sentence.",
                "/model",
                "/rename lab-round",
                CLAUDE_MAC_LONG_PASTE + " Summarise again.",
                "/help",
                *self.HISTORY_ONLY,
            ],
        )

    def test_history_paste_not_followed_through_symlink(self):
        cache = self.home / ".claude/paste-cache"
        cache.mkdir(parents=True, exist_ok=True)
        target = self.tmp / "elsewhere.txt"
        target.write_text("not the paste", encoding="utf-8")
        try:
            (cache / ("%s.txt" % CLAUDE_MAC_PASTE_HASH)).symlink_to(target)
        except OSError:
            self.skipTest("symlinks not available")
        rows = self._rows(".claude/history.jsonl", claude_mac_history_records())
        self.assertEqual(rows[5].text, "[Pasted text #2 +4 lines] Summarise again.")

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_history_paste_cache_unreadable_is_reported(self):
        # A cache file that is there but cannot be read is evidence, not an
        # absent entry: the history line keeps its placeholder and a
        # `parser:` row names the file.
        cache = self.home / ".claude/paste-cache"
        cache.mkdir(parents=True, exist_ok=True)
        paste_file = cache / ("%s.txt" % CLAUDE_MAC_PASTE_HASH)
        paste_file.write_text(CLAUDE_MAC_LONG_PASTE, encoding="utf-8")
        paste_file.chmod(0)
        try:
            rows = self._rows(".claude/history.jsonl", claude_mac_history_records())
        finally:
            paste_file.chmod(0o600)
        self.assertEqual(rows[5].text, "[Pasted text #2 +4 lines] Summarise again.")
        problems = [
            row for row in rows if row.turn_type == "system" and row.text.startswith("parser:")
        ]
        self.assertEqual(len(problems), 1)
        self.assertTrue(
            problems[0].text.startswith(
                "parser: paste-cache/%s.txt unreadable (" % CLAUDE_MAC_PASTE_HASH
            ),
            problems[0].text,
        )
        self.assertEqual(problems[0].source_line, 6)

    def _primary(self):
        rows = self._rows(self.REL, claude_mac_records())
        return rows + self._rows(
            self.PROJECT + "%s.jsonl" % CLAUDE_MAC_CLEARED_SESSION, claude_mac_cleared_records()
        )

    def test_history_matches_transcript_commands_and_pastes(self):
        # Every history line but the three history-only ones is the same
        # event as a transcript record; the two `/model` lines pair with
        # their own command records, the second recorded 27 s after it was
        # typed.
        primary = self._primary()
        rows, dropped = deduplicate(primary, [(row, 2.0) for row in self._history()])
        kept = rows[len(primary) :]
        self.assertEqual(
            [(row.text, row.recovered) for row in kept],
            [(text, "yes") for text in self.HISTORY_ONLY],
        )
        self.assertEqual(dropped["claude-code"], 7)

    def test_command_without_args_matches_both_labels(self):
        # The real shape: a tag block with an empty `<command-args>`, under
        # the `slash command:` label (a user record) or `local_command:` (a
        # system record, `/help`); the match text has no trailing space.
        block = (
            "<command-name>/%s</command-name>\n            <command-message>%s"
            "</command-message>\n            <command-args></command-args>"
        )
        for label, name in (("slash command: ", "model"), ("local_command: ", "help")):
            form = match_form(Row(turn_type="system", text=label + block % (name, name)))
            self.assertEqual(form, match_form(Row(turn_type="user", text="/" + name)))
            self.assertEqual(form, ("user", "/" + name))
        with_args = block.replace("<command-args></command-args>", "<command-args>x</command-args>")
        self.assertEqual(
            match_form(Row(turn_type="system", text="slash command: " + with_args % ("a", "a"))),
            match_form(Row(turn_type="user", text="/a x")),
        )
        # A command line gets the wide window; a prompt keeps the source's.
        self.assertEqual(match_tolerance(Row(turn_type="user", text="/model")), 600.0)
        self.assertIsNone(match_tolerance(Row(turn_type="user", text="run /bin/ls")))
        self.assertIsNone(match_tolerance(Row(turn_type="system", text="/model")))

    def test_pasted_content_wrapper_matches_history_paste(self):
        transcript = Row(
            turn_type="user",
            text='<pasted_content id="0bb3">\none\ntwo\n</pasted_content> Summarise it.',
        )
        history = Row(turn_type="user", text="one\ntwo Summarise it.")
        self.assertEqual(match_form(transcript), match_form(history))
        # Only the wrapper goes: other text still differs.
        self.assertNotEqual(
            match_form(transcript), match_form(Row(turn_type="user", text="one two Summarise"))
        )

    def test_history_only_lines_stay_recovered(self):
        # `/exit` typed while a turn ran and `/logout` write no transcript
        # line; `/clear` is under the ended session's id while its record
        # is the first line of the new session's file.
        primary = self._primary()
        cleared = [
            row
            for row in primary
            if row.session_id == CLAUDE_MAC_CLEARED_SESSION
            and row.text.startswith("slash command:")
        ]
        self.assertEqual([match_form(row) for row in cleared], [("user", "/clear")])
        rows, _ = deduplicate(primary, [(row, 2.0) for row in self._history()])
        kept = rows[len(primary) :]
        self.assertEqual([row.text for row in kept], self.HISTORY_ONLY)
        self.assertEqual({row.session_id for row in kept}, {CLAUDE_MAC_SESSION})
        self.assertEqual({row.recovered for row in kept}, {"yes"})

    def test_history_command_without_transcript_is_recovered(self):
        # With the second `/model` record gone, its history line stays: the
        # first record is taken by the first line, nearer in time.
        primary = [row for row in self._primary() if row.source_line != 18]
        rows, _ = deduplicate(primary, [(row, 2.0) for row in self._history()])
        self.assertEqual(
            [row.text for row in rows[len(primary) :]],
            ["/model", *self.HISTORY_ONLY],
        )
        self.assertEqual(rows[len(primary)].timestamp_utc, "2026-10-01T11:00:23.000Z")

    def test_history_command_missing_first_record_keeps_its_own_line(self):
        # With the first `/model` record gone, the first history line may not
        # take the second line's record, 43 s later: the second line pairs
        # with it and the first stays, under its own time (the proximity rule
        # bounded by the next line of the same command).
        primary = [row for row in self._primary() if row.source_line != 8]
        rows, _ = deduplicate(primary, [(row, 2.0) for row in self._history()])
        self.assertEqual(
            [row.text for row in rows[len(primary) :]],
            ["/model", *self.HISTORY_ONLY],
        )
        self.assertEqual(rows[len(primary)].timestamp_utc, "2026-10-01T11:00:07.000Z")

    def test_history_bounds_skip_equal_time_copies(self):
        # The same `/model` line in two history files, at the same time, is
        # bounded by the next strictly later line, not by its copy: with the
        # first record gone, the 23 line takes the 50 record, one 07 copy is
        # dropped as a copy of the other, and the surviving 07 line keeps its
        # own time.
        primary = [row for row in self._primary() if row.source_line != 8]
        copy = self._rows(".claude-work/history.jsonl", [claude_mac_history_records()[1]])
        self.assertEqual([row.text for row in copy], ["/model"])
        history = self._history() + copy
        rows, _ = deduplicate(primary, [(row, 2.0) for row in history])
        kept = rows[len(primary) :]
        self.assertEqual([row.text for row in kept], ["/model", *self.HISTORY_ONLY])
        self.assertEqual(kept[0].timestamp_utc, "2026-10-01T11:00:07.000Z")

    def test_history_command_window_is_forward_only(self):
        # A record is written after its line, never before: with the first
        # `/model` history line gone and only the first record left, the
        # second line (16 s after that record) stays recovered instead of
        # taking it.
        primary = [row for row in self._primary() if row.source_line != 18]
        history = [
            row
            for row in self._history()
            if not (row.text == "/model" and row.timestamp_utc.endswith("07.000Z"))
        ]
        rows, _ = deduplicate(primary, [(row, 2.0) for row in history])
        kept = rows[len(primary) :]
        self.assertEqual([row.text for row in kept], ["/model", *self.HISTORY_ONLY])
        self.assertEqual(kept[0].timestamp_utc, "2026-10-01T11:00:23.000Z")

    def test_history_bounds_follow_the_matching_scope(self):
        # Two session-less lines of different projects do not bound each
        # other: project A's line still takes A's delayed record although
        # project B typed the same command in between.
        block = (
            "slash command: <command-name>/model</command-name>\n<command-message>model"
            "</command-message>\n<command-args></command-args>"
        )

        def line(second, project, session="", source="h"):
            return Row(
                host="h",
                user="u",
                agent="claude-code",
                turn_type="user",
                text="/model",
                timestamp_utc="2026-10-01T11:00:%02d.000Z" % second,
                project_path=project,
                session_id=session,
                source_file=source,
            )

        record = Row(
            host="h",
            user="u",
            agent="claude-code",
            turn_type="system",
            text=block,
            timestamp_utc="2026-10-01T11:00:40.000Z",
            project_path="/a",
            session_id="s1",
            source_file="t",
        )
        rows, dropped = deduplicate([record], [(line(0, "/a"), 2.0), (line(30, "/b"), 2.0)])
        self.assertEqual(dropped["claude-code"], 1)
        self.assertEqual([(row.project_path, row.recovered) for row in rows[1:]], [("/b", "yes")])

    def test_history_bounds_cover_narrower_scopes(self):
        # A session-less line of project P is bounded by a later line of a
        # session of P: with only that session's record left, the earlier
        # line may not take it, so it stays under its own time and the
        # session's line pairs with its record.
        block = (
            "slash command: <command-name>/model</command-name>\n<command-message>model"
            "</command-message>\n<command-args></command-args>"
        )

        def row(turn_type, text, second, session, source):
            return Row(
                host="h",
                user="u",
                agent="claude-code",
                turn_type=turn_type,
                text=text,
                timestamp_utc="2026-10-01T11:00:%02d.000Z" % second,
                project_path="/srv/proj",
                session_id=session,
                source_file=source,
            )

        primary = [row("system", block, 50, "s1", "t")]
        no_session = row("user", "/model", 7, "", "h")
        in_session = row("user", "/model", 23, "s1", "h")
        rows, dropped = deduplicate(primary, [(no_session, 2.0), (in_session, 2.0)])
        self.assertEqual(dropped["claude-code"], 1)
        kept = rows[len(primary) :]
        self.assertEqual(
            [(kept_row.timestamp_utc, kept_row.recovered) for kept_row in kept],
            [("2026-10-01T11:00:07.000Z", "yes")],
        )

    def test_history_bounds_cover_broader_scopes(self):
        # The inverse: a session line is bounded by a later session-less
        # line of its project, since both could take the session's record;
        # a line of another session never bounds it.
        block = (
            "slash command: <command-name>/model</command-name>\n<command-message>model"
            "</command-message>\n<command-args></command-args>"
        )

        def row(turn_type, text, second, session, source):
            return Row(
                host="h",
                user="u",
                agent="claude-code",
                turn_type=turn_type,
                text=text,
                timestamp_utc="2026-10-01T11:00:%02d.000Z" % second,
                project_path="/srv/proj",
                session_id=session,
                source_file=source,
            )

        primary = [row("system", block, 50, "s1", "t")]
        in_session = row("user", "/model", 7, "s1", "h")
        no_session = row("user", "/model", 23, "", "h")
        rows, dropped = deduplicate(primary, [(in_session, 2.0), (no_session, 2.0)])
        self.assertEqual(dropped["claude-code"], 1)
        kept = rows[len(primary) :]
        self.assertEqual(
            [(kept_row.timestamp_utc, kept_row.recovered) for kept_row in kept],
            [("2026-10-01T11:00:07.000Z", "yes")],
        )
        # A later line of a different session leaves the session line free
        # to take its own record.
        other = row("user", "/model", 23, "s2", "h")
        rows, dropped = deduplicate(primary, [(in_session, 2.0), (other, 2.0)])
        self.assertEqual(dropped["claude-code"], 1)
        self.assertEqual([kept_row.session_id for kept_row in rows[len(primary) :]], ["s2"])

    def test_history_bounds_follow_the_candidate_session(self):
        # A session-less line is bounded only by lines that could take the
        # record it is matched against: with records in two sessions, the
        # later line of session S2 does not keep the session-less line from
        # S1's record, and each line takes its own.
        block = (
            "slash command: <command-name>/model</command-name>\n<command-message>model"
            "</command-message>\n<command-args></command-args>"
        )

        def row(turn_type, text, second, session, source):
            return Row(
                host="h",
                user="u",
                agent="claude-code",
                turn_type=turn_type,
                text=text,
                timestamp_utc="2026-10-01T11:00:%02d.000Z" % second,
                project_path="/srv/proj",
                session_id=session,
                source_file=source,
            )

        primary = [row("system", block, 50, "s1", "t1"), row("system", block, 30, "s2", "t2")]
        free = row("user", "/model", 7, "", "h")
        in_s2 = row("user", "/model", 23, "s2", "h")
        rows, dropped = deduplicate(primary, [(free, 2.0), (in_s2, 2.0)])
        self.assertEqual(dropped["claude-code"], 2)
        self.assertEqual(rows[len(primary) :], [])

    def test_history_bounds_follow_the_candidate_project(self):
        # A session's line is bounded by a later session-less line of the
        # project the record was written in, not of the line's own project:
        # the session moved from P to Q, the :23 line of Q can take the :50
        # record in S/Q, so the :07 line of S/P stays under its own time.
        block = (
            "slash command: <command-name>/model</command-name>\n<command-message>model"
            "</command-message>\n<command-args></command-args>"
        )

        def row(turn_type, text, second, session, project, source):
            return Row(
                host="h",
                user="u",
                agent="claude-code",
                turn_type=turn_type,
                text=text,
                timestamp_utc="2026-10-01T11:00:%02d.000Z" % second,
                project_path=project,
                session_id=session,
                source_file=source,
            )

        primary = [row("system", block, 50, "s", "/srv/Q", "t")]
        moved = row("user", "/model", 7, "s", "/srv/P", "h")
        in_q = row("user", "/model", 23, "", "/srv/Q", "h")
        rows, dropped = deduplicate(primary, [(moved, 2.0), (in_q, 2.0)])
        self.assertEqual(dropped["claude-code"], 1)
        self.assertEqual(rows[len(primary) :], [moved])

    def test_widened_window_stays_between_history_files(self):
        # Two history files of one session each hold a `/model` a minute
        # apart; only the later one's record survives. The earlier line is
        # bounded by the later line against the transcript, and the 600 s
        # window does not make it a copy of the later line: it stays.
        block = (
            "slash command: <command-name>/model</command-name>\n<command-message>model"
            "</command-message>\n<command-args></command-args>"
        )

        def row(turn_type, text, second, source):
            return Row(
                host="h",
                user="u",
                agent="claude-code",
                turn_type=turn_type,
                text=text,
                timestamp_utc="2026-10-01T11:%02d:%02d.000Z" % divmod(second, 60),
                project_path="/srv/proj",
                session_id="s1",
                source_file=source,
            )

        primary = [row("system", block, 80, ".claude/projects/-srv-proj/s1.jsonl")]
        early = row("user", "/model", 7, ".claude/history.jsonl")
        late = row("user", "/model", 67, ".claude-work/history.jsonl")
        rows, dropped = deduplicate(primary, [(early, 2.0), (late, 2.0)])
        self.assertEqual(dropped["claude-code"], 1)
        kept = rows[len(primary) :]
        self.assertEqual(
            [(kept_row.source_file, kept_row.recovered) for kept_row in kept],
            [(early.source_file, "yes")],
        )

    def test_match_form_keeps_whitespace_distinctions(self):
        # Runs of whitespace collapse, but the whitespace stays: prompts and
        # arguments that differ only by a space are different events.
        self.assertEqual(
            match_form(Row(turn_type="user", text="echo  a\tb")),
            match_form(Row(turn_type="user", text="echo a b")),
        )
        self.assertNotEqual(
            match_form(Row(turn_type="user", text="echo a b")),
            match_form(Row(turn_type="user", text="echo ab")),
        )
        block = (
            "slash command: <command-name>/a</command-name>\n<command-message>a"
            "</command-message>\n<command-args>%s</command-args>"
        )
        self.assertEqual(
            match_form(Row(turn_type="system", text=block % "x  y")),
            match_form(Row(turn_type="user", text="/a x y")),
        )
        self.assertNotEqual(
            match_form(Row(turn_type="system", text=block % "x y")),
            match_form(Row(turn_type="user", text="/a xy")),
        )

    def test_remote_control_spawn(self):
        rows = self._rows(self.SPAWN_REL, claude_mac_spawn_records())
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                (1, "system", "prompt queued: delivered at line 3"),
                (2, "system", "queue dequeue:"),
                (3, "system", "session surface: sdk-cli"),
                (3, "system", "session origin: remote (Remote Control, entrypoint sdk-cli)"),
                (3, "system", "permission mode: default"),
                (3, "user", "Run ls -la in this folder and tell me what files are here."),
                (3, "system", "prompt trigger: promptSource=sdk turnOrigin=human"),
                (4, "system", "model: claude-fable-5-1"),
                (4, "tool_use", "ls -la"),
                (5, "tool_result", "calc.py\nREADME.md"),
                (5, "system", "permission decision: accept source=config reason=subcommandResults"),
            ],
        )

    def test_sdk_cli_without_sdk_prompt_is_not_a_spawn(self):
        records = claude_mac_spawn_records()
        records[2] = dict(records[2], promptSource="typed")
        rows = self._rows(self.SPAWN_REL, records)
        self.assertFalse([row for row in rows if row.text.startswith("session origin:")])

    def test_teleported_session(self):
        rows = self._rows(self.TELEPORT_REL, claude_mac_teleport_records())
        got = [(row.source_line, row.turn_type, row.text) for row in rows]
        self.assertEqual(
            got[:6],
            [
                (1, "system", "session surface: cli"),
                (
                    1,
                    "system",
                    "session origin: remote (teleport, cloud scratchpad %s)"
                    % CLAUDE_MAC_CLOUD_SCRATCH,
                ),
                (2, "system", "session origin: remote (teleport, remoteSourced)"),
                (
                    2,
                    "system",
                    "session synced: cloud session (teleported: remoteSourced records "
                    "replayed from the vendor's copy)",
                ),
                (2, "system", "permission mode: default"),
                (2, "user", "Describe this repository in three sentences."),
            ],
        )
        self.assertIn(
            (
                4,
                "system",
                "permission decision: accept source=config reason=rule "
                '{"decision":"accept","reasonType":"rule","ruleSource":"cliArg","source":"config"}',
            ),
            got,
        )
        self.assertIn((6, "system", "informational: Session resumed"), got)
        self.assertTrue(
            any(line == 5 and text.startswith("stop_hook_summary: ") for line, _, text in got)
        )
        # The local environment's scratchpad is this host's: a scratch row.
        local = [row for row in rows if row.source_line == 7]
        self.assertEqual(
            [(row.text, row.project_path) for row in local],
            [
                (
                    "scratch directory: %s (session scratchpad, from the environment"
                    " attachment)" % CLAUDE_MAC_LOCAL_SCRATCH,
                    CLAUDE_MAC_LOCAL_SCRATCH,
                )
            ],
        )
        self.assertEqual(
            [row.text for row in rows if row.text.startswith("session origin:")][1:],
            ["session origin: remote (teleport, remoteSourced)"],
        )

    def test_relocated_local_scratchpad_stays_scratch(self):
        # After a `relocated` record the local process runs under the new
        # cwd's slug; its scratchpad follows the replayed block, so it is
        # this host's even in a teleported file, not the cloud machine's.
        records = claude_mac_teleport_records()
        k = next(
            i
            for i, record in enumerate(records)
            if record.get("type") == "attachment"
            and record["attachment"]["snapshot"].get("scratchpadDirectory")
            == CLAUDE_MAC_LOCAL_SCRATCH
        )
        moved = CLAUDE_MAC_LOCAL_SCRATCH.replace("/-srv-proj/", "/-srv-new/")
        records[k]["attachment"]["snapshot"]["scratchpadDirectory"] = moved
        records.insert(
            k,
            dict(type="relocated", relocatedCwd="/srv/new", sessionId=CLAUDE_MAC_TELEPORT_SESSION),
        )
        rows = self._rows(self.TELEPORT_REL, records)
        self.assertIn(
            (moved, moved),
            [
                (row.project_path, row.text.split(" ")[2])
                for row in rows
                if row.text.startswith("scratch directory:")
            ],
        )
        self.assertEqual(
            [row.text for row in rows if row.text.startswith("session origin:")],
            [
                "session origin: remote (teleport, cloud scratchpad %s)" % CLAUDE_MAC_CLOUD_SCRATCH,
                "session origin: remote (teleport, remoteSourced)",
            ],
        )
        self.assertIn("cwd changed: /srv/new", [row.text for row in rows])

    def test_foreign_scratchpad_without_remote_sourced_stays_scratch(self):
        records = [
            record for record in claude_mac_teleport_records() if not record.get("remoteSourced")
        ]
        rows = self._rows(self.TELEPORT_REL, records)
        self.assertFalse([row for row in rows if row.text.startswith("session origin:")])
        self.assertIn(
            CLAUDE_MAC_CLOUD_SCRATCH,
            [row.project_path for row in rows if row.text.startswith("scratch directory:")],
        )

    def test_same_slug_cloud_scratchpad_is_replayed(self):
        # The cloud checkout and the local one share a path, so the cloud
        # scratchpad sits under the transcript's own slug: it is still the
        # replayed one, on or before the last `remoteSourced` line, and
        # never a scratch directory of this host.
        records = claude_mac_teleport_records()
        same = "/tmp/claude-0/-srv-proj/22222222-3333-4444-8555-666666666666/scratchpad"
        k = next(
            i
            for i, record in enumerate(records)
            if record.get("type") == "attachment"
            and record["attachment"]["snapshot"].get("scratchpadDirectory")
            == CLAUDE_MAC_CLOUD_SCRATCH
        )
        records[k]["attachment"]["snapshot"]["scratchpadDirectory"] = same
        rows = self._rows(self.TELEPORT_REL, records)
        self.assertEqual(
            [row.text for row in rows if row.text.startswith("session origin:")],
            [
                "session origin: remote (teleport, cloud scratchpad %s)" % same,
                "session origin: remote (teleport, remoteSourced)",
            ],
        )
        self.assertEqual(
            [row.project_path for row in rows if row.text.startswith("scratch directory:")],
            [CLAUDE_MAC_LOCAL_SCRATCH],
        )

    def test_case_variant_local_scratchpad_stays_scratch(self):
        # A local slug spelled in another case than the project directory
        # is still this host's: the attachment follows the replayed block.
        records = claude_mac_teleport_records()
        k = next(
            i
            for i, record in enumerate(records)
            if record.get("type") == "attachment"
            and record["attachment"]["snapshot"].get("scratchpadDirectory")
            == CLAUDE_MAC_LOCAL_SCRATCH
        )
        cased = CLAUDE_MAC_LOCAL_SCRATCH.replace("/-srv-proj/", "/-Srv-Proj/")
        records[k]["attachment"]["snapshot"]["scratchpadDirectory"] = cased
        rows = self._rows(self.TELEPORT_REL, records)
        self.assertEqual(
            [row.project_path for row in rows if row.text.startswith("scratch directory:")], [cased]
        )
        self.assertEqual(
            [row.text for row in rows if row.text.startswith("session origin:")],
            [
                "session origin: remote (teleport, cloud scratchpad %s)" % CLAUDE_MAC_CLOUD_SCRATCH,
                "session origin: remote (teleport, remoteSourced)",
            ],
        )

    def test_ccr_tip(self):
        doc = {"eventId": "33333333-4444-4555-8666-777777777777", "updatedAt": 1790856300000}
        art = self._write(self.CCR_REL, json.dumps(doc))
        self.assertTrue(ClaudeCodeParser().wants(art))
        self.assertFalse(ClaudeCodeParser().secondary(art))
        rows = list(ClaudeCodeParser().parse(art, Options()))
        self.assertEqual(
            [(row.turn_type, row.timestamp_utc, row.session_id, row.text) for row in rows],
            [
                (
                    "system",
                    self.MTIME_TIMESTAMP,
                    CLAUDE_MAC_SPAWN_SESSION,
                    "session synced: remote event 33333333-4444-4555-8666-777777777777 at "
                    "1790856300000 (from %s, file mtime)" % self.CCR_REL,
                )
            ],
        )

    def test_ccr_tip_unreadable(self):
        for body in ('{"eventId": "cut', "[1]"):
            rows = self._rows(self.CCR_REL, body)
            self.assertEqual(
                [(row.turn_type, row.session_id, row.text) for row in rows],
                [("system", CLAUDE_MAC_SPAWN_SESSION, "parser: ccr-tip.json is not a JSON object")],
            )

    def test_bridge_pointer(self):
        doc = {
            "environmentId": "env_0123456789abcdefghijkl",
            "pid": 15081,
            "procStart": "Mon Oct  1 12:00:00 2026",
            "sessionId": "0123456789abcdef0123456789abcdef",
            "source": "standalone",
        }
        rows = self._rows(self.POINTER_REL, json.dumps(doc))
        self.assertEqual(
            [
                (row.turn_type, row.timestamp_utc, row.session_id, row.project_path, row.text)
                for row in rows
            ],
            [
                (
                    "system",
                    self.MTIME_TIMESTAMP,
                    "",
                    "",
                    "remote control: environment env_0123456789abcdefghijkl session "
                    "0123456789abcdef0123456789abcdef source=standalone "
                    "(from %s, file mtime)" % self.POINTER_REL,
                )
            ],
        )
        self.assertNotIn("15081", rows[0].text)

    def test_bridge_pointer_unreadable(self):
        rows = self._rows(self.POINTER_REL, "{cut mid-write")
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("system", "parser: bridge-pointer.json is not a JSON object")],
        )

    def test_global_config_remote_keys(self):
        rows = self._rows(".claude.json", json.dumps(claude_mac_global_config()))
        source = "(from .claude.json, file mtime)"
        self.assertEqual(
            [(row.text, row.project_path, row.timestamp_utc) for row in rows],
            [
                (
                    "permission setting: hasTrustDialogAccepted=true " + source,
                    "/srv/proj",
                    self.MTIME_TIMESTAMP,
                ),
                (
                    "remote control: spawn mode same-dir for /srv/proj " + source,
                    "/srv/proj",
                    self.MTIME_TIMESTAMP,
                ),
                ("remote control: machine id set " + source, "", self.MTIME_TIMESTAMP),
                ("cloud: hasRemoteEnvironment=true " + source, "", self.MTIME_TIMESTAMP),
            ],
        )
        for row in rows:
            self.assertNotIn("0000000000cc", row.text)

    def test_session_summary_origin_and_sync(self):
        rows = self._rows(self.REL, claude_mac_records())
        rows += self._rows(self.SPAWN_REL, claude_mac_spawn_records())
        rows += self._rows(self.TELEPORT_REL, claude_mac_teleport_records())
        rows += self._rows(self.CCR_REL, json.dumps({"eventId": "e1", "updatedAt": "t"}))
        parser = ClaudeCodeParser()
        self.assertTrue(parser.records_origin and parser.records_sync)
        agents = frozenset({parser.agent})
        got = {
            session.session_id: (session.origin, session.synced)
            for session in summarise(rows, agents, frozenset(), agents)
        }
        self.assertEqual(
            got,
            {
                CLAUDE_MAC_SESSION: ("local", "no"),
                CLAUDE_MAC_SPAWN_SESSION: ("remote", "yes"),
                CLAUDE_MAC_TELEPORT_SESSION: ("remote", "yes"),
            },
        )

    def test_desktop_subclass_records_no_origin(self):
        self.assertFalse(_ClaudeDesktopParser().records_origin)

    def test_truncated_line_is_reported_not_fatal(self):
        art = self._write(self.SPAWN_REL, claude_mac_spawn_records())
        write_bad_line(art.disk_path)
        after = dict(ClaudeCodeTests.AFTER_CUT, sessionId=CLAUDE_MAC_SPAWN_SESSION)
        with open(art.disk_path, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(after) + "\n")
        rows = list(ClaudeCodeParser().parse(art, Options()))
        self.assertIn(
            "session origin: remote (Remote Control, entrypoint sdk-cli)",
            [row.text for row in rows],
        )
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[-2:]],
            [(8, "user", "after the cut"), (7, "system", bad_line_text())],
        )

    def test_thinking_opt_in(self):
        rows = self._rows(self.REL, claude_mac_records(), include_thinking=True)
        self.assertEqual(len(rows), len(self.EXPECTED))

    def test_sidecars_not_wanted(self):
        home = self.artifact(".claude/settings.json").home
        for rel in (
            self.PROJECT + "%s/custom-title.json" % CLAUDE_MAC_SESSION,
            self.PROJECT + "%s/subagents/agent-a1.meta.json" % CLAUDE_MAC_SESSION,
            ".claude/paste-cache/%s.txt" % CLAUDE_MAC_PASTE_HASH,
            ".claude/bridge-spawn/cse_01/mcp.json",
            ".claude/ide/50123.lock",
        ):
            art = Artifact(self.home / rel, rel, rel, "claude-code", home)
            self.assertFalse(ClaudeCodeParser().wants(art), rel)
