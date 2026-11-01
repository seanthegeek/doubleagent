"""Tests of the little-coder parser."""

import unittest
from typing import ClassVar

from test_errors import CAN_DENY, DENY_REASON

from doubleagent.parsers.little_coder import (
    MESSAGES_KEY_PATTERN,
    SAVED_AT_PATTERN,
    SESSION_ID_PATTERN,
    SESSIONS_KEY_PATTERN,
    LittleCoderParser,
)
from doubleagent.parsers.pi import PiParser
from fixtures import (
    LC_FILE,
    LC_LEGACY_CKPT,
    LC_LEGACY_CONFIG,
    LC_LEGACY_DAILY,
    LC_LEGACY_GIST,
    LC_LEGACY_HISTORY,
    LC_LEGACY_INPUT,
    LC_LEGACY_LATEST,
    LC_LEGACY_OLD_SESSION,
    LC_LEGACY_SESSION,
    LC_LEGACY_SNAPSHOTS,
    LC_SESSION,
    PI_DIR,
)

from .base import ParserBase


class LittleCoderTests(ParserBase):
    """Test the little-coder parser."""

    def test_recovered_keys_take_json_whitespace_only(self):
        patterns = (
            (SESSIONS_KEY_PATTERN, '"sessions"%s:%s['),
            (MESSAGES_KEY_PATTERN, '"messages"%s:%s['),
            (SESSION_ID_PATTERN, '"session_id"%s:%s"s1"'),
            (SAVED_AT_PATTERN, '"saved_at"%s:%s"t"'),
        )
        for pattern, template in patterns:
            self.assertTrue(pattern.search(template % (" \r\n", "\t")), template)
            for space in map(chr, (0x00A0, 0x2028, 0x0085)):
                text = template % (space, "")
                self.assertIsNone(pattern.search(text), repr(text))

    HISTORY = ".pi/agent/little-coder-prompt-history.json"
    CHECKPOINTS = ".little-coder/checkpoints/%s/" % LC_FILE

    def test_prompt_history(self):
        rows = self.rows_for(LittleCoderParser(), self.HISTORY)
        self.assertEqual(
            [
                (row.turn_type, row.text, row.timestamp_utc, row.session_id, row.source_line)
                for row in rows
            ],
            [("user", "fix the failing test", "", "", 1), ("user", "now run lint", "", "", 2)],
        )
        self.assertEqual({row.agent for row in rows}, {"little-coder"})

    def test_checkpoints_one_row_per_set(self):
        self.assertEqual(
            self.rows_for(LittleCoderParser(), self.CHECKPOINTS + "_srv_proj_src_new.ts.absent"), []
        )
        rows = self.rows_for(LittleCoderParser(), self.CHECKPOINTS + "_srv_proj_src_a.ts")
        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual(
            (row.turn_type, row.session_id, row.project_path, row.timestamp_utc),
            ("system", LC_SESSION, "/srv/proj", "2026-10-01T11:00:06.000Z"),
        )
        self.assertIn("2 pre-edit file(s)", row.text)
        self.assertIn("_srv_proj_src_new.ts (did not exist)", row.text)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_session_file_is_one_row_after_the_checkpoint_row(self):
        session_file = self.home / PI_DIR / LC_FILE
        session_file.chmod(0)
        try:
            rows = self.rows_for(LittleCoderParser(), self.CHECKPOINTS + "_srv_proj_src_a.ts")
        finally:
            session_file.chmod(0o600)
        self.assertEqual(len(rows), 2)
        self.assertIn("pi session %s" % LC_FILE, rows[0].text)
        self.assertNotIn("session file not collected", rows[0].text)
        self.assertEqual(
            (
                rows[1].turn_type,
                rows[1].text,
                rows[1].source_line,
                rows[1].timestamp_utc,
                rows[1].session_id,
            ),
            (
                "system",
                "parser: %s could not be read (Permission denied)" % LC_FILE,
                0,
                "",
                LC_SESSION,
            ),
        )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unlistable_sessions_directory_keeps_the_checkpoint_row(self):
        sessions = self.home / ".pi/agent/sessions"
        sessions.chmod(0)
        try:
            rows = self.rows_for(LittleCoderParser(), self.CHECKPOINTS + "_srv_proj_src_a.ts")
        finally:
            sessions.chmod(0o700)
        self.assert_unreadable_directory_rows(rows, "sessions")

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unsearchable_project_directory_keeps_the_checkpoint_row(self):
        project = self.home / PI_DIR
        project.chmod(0)
        try:
            rows = self.rows_for(LittleCoderParser(), self.CHECKPOINTS + "_srv_proj_src_a.ts")
        finally:
            project.chmod(0o700)
        self.assert_unreadable_directory_rows(rows, project.name)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unsearchable_sessions_directory_keeps_the_checkpoint_row(self):
        """A sessions directory that lists but cannot be searched names itself in the row."""
        sessions = self.home / ".pi/agent/sessions"
        sessions.chmod(0o400)
        try:
            rows = self.rows_for(LittleCoderParser(), self.CHECKPOINTS + "_srv_proj_src_a.ts")
        finally:
            sessions.chmod(0o700)
        self.assert_unreadable_directory_rows(rows, "sessions")

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unsearchable_agent_directory_keeps_the_checkpoint_row(self):
        """An unsearchable `.pi/agent` above the sessions directory gives the sessions row."""
        agent_dir = self.home / ".pi/agent"
        agent_dir.chmod(0)
        try:
            rows = self.rows_for(LittleCoderParser(), self.CHECKPOINTS + "_srv_proj_src_a.ts")
        finally:
            agent_dir.chmod(0o700)
        self.assert_unreadable_directory_rows(rows, "sessions")

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unsearchable_sibling_project_directory_does_not_hide_the_session(self):
        """A denied project directory sorting first is passed over; a later one yields the header."""
        sibling = self.home / ".pi/agent/sessions/--aaa--"
        sibling.mkdir()
        sibling.chmod(0)
        try:
            rows = self.rows_for(LittleCoderParser(), self.CHECKPOINTS + "_srv_proj_src_a.ts")
        finally:
            sibling.chmod(0o700)
        self.assertEqual(len(rows), 1)
        self.assertEqual((rows[0].session_id, rows[0].project_path), (LC_SESSION, "/srv/proj"))
        self.assertNotIn("session file not collected", rows[0].text)

    def assert_unreadable_directory_rows(self, rows, directory):
        """Check the checkpoint row without the not-collected clause, then the directory row."""
        self.assertEqual(len(rows), 2)
        self.assertIn("little-coder checkpoints for", rows[0].text)
        self.assertNotIn("session file not collected", rows[0].text)
        self.assertEqual(
            (
                rows[1].turn_type,
                rows[1].text,
                rows[1].source_line,
                rows[1].timestamp_utc,
                rows[1].session_id,
            ),
            (
                "system",
                "parser: %s could not be read (Permission denied)" % directory,
                0,
                "",
                LC_SESSION,
            ),
        )

    def test_bad_session_line_is_left_to_the_pi_parser(self):
        session_file = self.home / PI_DIR / LC_FILE
        original = session_file.read_bytes()
        session_file.write_bytes(original + b"{not json\n")
        try:
            rows = self.rows_for(LittleCoderParser(), self.CHECKPOINTS + "_srv_proj_src_a.ts")
        finally:
            session_file.write_bytes(original)
        self.assertEqual([row.text[:30] for row in rows], ["little-coder checkpoints for p"])
        self.assertNotIn("session file not collected", rows[0].text)

    def test_thinking_opt_in(self):
        # little-coder's own files hold no reasoning; its transcripts are pi's.
        rows = self.rows_for(LittleCoderParser(), self.HISTORY, include_thinking=True)
        self.assertNotIn("thinking", [row.turn_type for row in rows])

    def test_session_model_rows_through_pi_reader(self):
        # little-coder sessions are pi files: the pi parser yields their
        # model rows, from the assistant `model` (no model_change entry).
        rows = self.rows_for(PiParser(), PI_DIR + LC_FILE)
        models = [row for row in rows if row.text.startswith("model:")]
        self.assertEqual(
            [
                (row.text, row.model, row.timestamp_utc, row.session_id, row.source_line)
                for row in models
            ],
            [
                (
                    "model: qwen3.6-35b-a3b provider=llamacpp",
                    "qwen3.6-35b-a3b",
                    "2026-10-01T11:00:05.000Z",
                    LC_SESSION,
                    4,
                ),
                (
                    "model: qwen3.6-35b-a3b -> qwen3.6-27b provider=llamacpp",
                    "qwen3.6-27b",
                    "2026-10-01T11:00:12.000Z",
                    LC_SESSION,
                    6,
                ),
            ],
        )
        i = rows.index(models[0])
        self.assertEqual(rows[i + 1].turn_type, "tool_use")  # before the record's own rows

    def test_settings_and_sessions_not_wanted(self):
        art = next(
            artifact
            for artifact in self.col.artifacts
            if artifact.rel == ".config/little-coder/settings.json"
        )
        self.assertFalse(LittleCoderParser().wants(art))
        art = next(artifact for artifact in self.col.artifacts if artifact.rel == PI_DIR + LC_FILE)
        self.assertEqual(art.agent, "pi")
        self.assertFalse(LittleCoderParser().wants(art))

    def test_truncated_history_keeps_earlier_prompts(self):
        (self.home / self.HISTORY).write_text(
            '["fix the failing test", "now run li', encoding="utf-8"
        )
        rows = self.rows_for(LittleCoderParser(), self.HISTORY)
        self.assertEqual([row.turn_type for row in rows], ["user", "system"])
        # The cut record is carried from its first character to the end.
        cut = '"now run li'
        self.assertEqual(rows[-1].source_line, 2)
        self.assertTrue(rows[-1].text.startswith("parser: unparseable record 2 ("), rows[-1].text)
        self.assertTrue(rows[-1].text.endswith("): " + cut), rows[-1].text)

    # ---- the v0.0.x layout under ~/.little-coder ------------------------------

    LEGACY_TYPES: ClassVar[list] = [
        "system",
        "user",
        "tool_use",
        "tool_result",
        "assistant",
        "tool_use",
        "tool_result",
        "system",
    ]

    def test_legacy_session_rows(self):
        rows = self.rows_for(LittleCoderParser(), LC_LEGACY_DAILY)
        self.assertEqual([row.turn_type for row in rows], self.LEGACY_TYPES)
        for row in rows:
            self.assertEqual(
                (row.agent, row.session_id, row.project_path, row.timestamp_utc),
                ("little-coder", LC_LEGACY_SESSION, "", "2026-09-20T10:15:00.000Z"),
            )
        saved, user, use, result, reply, write, denied, decision = rows
        self.assertEqual(
            (denied.turn_type, denied.text), ("tool_result", "Denied: user rejected this operation")
        )
        self.assertEqual(
            saved.text,
            "session saved: little-coder v0.0.x session %s turn_count=1 input_tokens=1200 "
            "output_tokens=60 (saved_at 2026-09-20 10:15:00, host local time)" % LC_LEGACY_SESSION,
        )
        self.assertEqual(user.text, "list the files")
        self.assertEqual((use.tool_name, use.tool_use_id, use.text), ("Bash", "toolu_l1", "ls"))
        self.assertEqual((result.tool_use_id, result.text), ("toolu_l1", "README.md"))
        self.assertEqual(reply.text, "One file.")
        self.assertEqual((write.tool_name, write.text), ("Write", "/srv/proj/b.py"))
        self.assertEqual(
            (decision.text, decision.tool_name, decision.tool_use_id),
            (
                "permission decision: denied (Denied: user rejected this operation)",
                "Write",
                "toolu_l2",
            ),
        )
        self.assertEqual([row.source_line for row in rows], [0, 1, 2, 3, 4, 4, 5, 5])
        self.assertFalse(LittleCoderParser().secondary(self.artifact(LC_LEGACY_DAILY)))

    def test_legacy_copies_are_secondary(self):
        self.assertTrue(LittleCoderParser.has_secondary)
        for rel in (LC_LEGACY_LATEST, LC_LEGACY_HISTORY, LC_LEGACY_INPUT):
            self.assertTrue(LittleCoderParser().secondary(self.artifact(rel)), rel)
        for rel in (self.HISTORY, LC_LEGACY_CONFIG, LC_LEGACY_SNAPSHOTS):
            self.assertFalse(LittleCoderParser().secondary(self.artifact(rel)), rel)
        daily = self.rows_for(LittleCoderParser(), LC_LEGACY_DAILY)
        latest = self.rows_for(LittleCoderParser(), LC_LEGACY_LATEST)
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc) for row in latest],
            [(row.turn_type, row.text, row.timestamp_utc) for row in daily],
        )

    def test_legacy_history_json(self):
        rows = self.rows_for(LittleCoderParser(), LC_LEGACY_HISTORY)
        old = [row for row in rows if row.session_id == LC_LEGACY_OLD_SESSION]
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc, row.source_line) for row in old[1:]],
            [("user", "an old prompt", "2026-09-19T08:00:00.000Z", 1)],
        )
        new = [row for row in rows if row.session_id == LC_LEGACY_SESSION]
        self.assertEqual([row.turn_type for row in new], self.LEGACY_TYPES)
        self.assertEqual({row.source_line for row in new}, {2})

    def test_legacy_history_cut_keeps_earlier_sessions(self):
        path = self.home / LC_LEGACY_HISTORY
        text = path.read_text(encoding="utf-8")
        cut = text[: text.index('"list the files"') + 8]
        path.write_text(cut, encoding="utf-8")
        rows = self.rows_for(LittleCoderParser(), LC_LEGACY_HISTORY)
        self.assertEqual(
            [(row.turn_type, row.session_id) for row in rows[:2]],
            [("system", LC_LEGACY_OLD_SESSION), ("user", LC_LEGACY_OLD_SESSION)],
        )
        self.assertEqual(len(rows), 3)
        self.assertEqual(rows[-1].source_line, 2)
        self.assertTrue(rows[-1].text.startswith("parser: unparseable record 2 ("), rows[-1].text)

    def test_legacy_session_cut_keeps_earlier_messages(self):
        path = self.home / LC_LEGACY_DAILY
        text = path.read_text(encoding="utf-8")
        path.write_text(text[: text.index('"One file."')], encoding="utf-8")
        rows = self.rows_for(LittleCoderParser(), LC_LEGACY_DAILY)
        self.assertEqual(
            [row.turn_type for row in rows], ["system", "user", "tool_use", "tool_result", "system"]
        )
        self.assertEqual(rows[1].session_id, LC_LEGACY_SESSION)
        self.assertEqual(rows[1].timestamp_utc, "2026-09-20T10:15:00.000Z")
        self.assertEqual(rows[-1].source_line, 4)
        self.assertTrue(rows[-1].text.startswith("parser: unparseable record 4 ("), rows[-1].text)
        path.write_text("not json", encoding="utf-8")
        rows = self.rows_for(LittleCoderParser(), LC_LEGACY_DAILY)
        self.assertEqual([row.text for row in rows], ["parser: session file is not a JSON object"])

    def test_legacy_input_history(self):
        rows = self.rows_for(LittleCoderParser(), LC_LEGACY_INPUT)
        self.assertEqual(
            [
                (row.turn_type, row.text, row.timestamp_utc, row.session_id, row.source_line)
                for row in rows
            ],
            [("user", "list the files", "", "", 1), ("user", "an older prompt", "", "", 2)],
        )

    def test_legacy_config_rows(self):
        rows = self.rows_for(LittleCoderParser(), LC_LEGACY_CONFIG)
        source = "(from .little-coder/config.json, file mtime)"
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc, row.session_id) for row in rows],
            [
                (
                    "system",
                    "session synced: gist %s %s" % (LC_LEGACY_GIST, source),
                    "2026-09-27T10:00:00.000Z",
                    "",
                ),
                (
                    "system",
                    "session relayed: telegram chat 424242 %s" % source,
                    "2026-09-27T10:00:00.000Z",
                    "",
                ),
                (
                    "system",
                    "permission setting: permission_mode=manual %s" % source,
                    "2026-09-27T10:00:00.000Z",
                    "",
                ),
            ],
        )
        # The tokens in the same file are never printed.
        for row in rows:
            self.assertNotIn("not-a-real", row.text)
            self.assertNotIn("ghp_", row.text)
        self.assertTrue(LittleCoderParser.records_sync and LittleCoderParser.records_relay)
        (self.home / LC_LEGACY_CONFIG).write_text("[1]", encoding="utf-8")
        rows = self.rows_for(LittleCoderParser(), LC_LEGACY_CONFIG)
        self.assertEqual(
            [row.text for row in rows], ["parser: .little-coder/config.json is not a JSON object"]
        )

    def test_legacy_snapshots(self):
        rows = self.rows_for(LittleCoderParser(), LC_LEGACY_SNAPSHOTS)
        self.assertEqual(
            [(row.turn_type, row.timestamp_utc, row.session_id, row.source_line) for row in rows],
            [
                ("system", "2026-09-20T10:14:00.000Z", LC_LEGACY_CKPT, 1),
                ("system", "2026-09-20T10:14:30.500Z", LC_LEGACY_CKPT, 2),
            ],
        )
        self.assertEqual(
            rows[1].text,
            "little-coder v0.0.x checkpoint 2: turn_count=1 prompt=list the files "
            "files=/srv/proj/a.py (backup 0123456789abcdef@v1); /srv/proj/b.py (did not exist) "
            "(created_at host local time)",
        )

    def test_legacy_noise_not_wanted(self):
        for rel in (
            ".little-coder/mcp.json",
            ".little-coder/checkpoints/%s/backups/0123456789abcdef@v1" % LC_LEGACY_CKPT,
        ):
            self.assertFalse(LittleCoderParser().wants(self.artifact(rel)), rel)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_legacy_files_are_one_row_each(self):
        prompts = ".pi/agent/little-coder-prompt-history.json"
        for rel in (".little-coder/input_history.txt", ".little-coder/config.json"):
            path = self.home / rel
            path.chmod(0)
            try:
                rows = self.rows_for(LittleCoderParser(), rel)
                other = self.rows_for(LittleCoderParser(), prompts)
            finally:
                path.chmod(0o600)
            self.assertEqual(
                [(row.turn_type, row.text, row.source_line, row.timestamp_utc) for row in rows],
                [("system", "parser: %s could not be read (Permission denied)" % path.name, 0, "")],
                rel,
            )
            self.assertTrue(other, rel)
            self.assertFalse([row for row in other if row.text.startswith("parser:")], rel)
