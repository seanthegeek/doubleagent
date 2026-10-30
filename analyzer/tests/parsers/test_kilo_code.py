"""Tests of the Kilo Code parser."""

import json

from doubleagent.inputs import open_input
from doubleagent.parsers.kilo_code import KiloCodeParser, is_task_file
from doubleagent.parsers.kilo_code import iter_json_array as kilo_iter_json_array
from doubleagent.parsers.opencode import OpenCodeParser
from doubleagent.timeutil import to_utc
from fixtures import (
    KILO_DIGEST_REL,
    KILO_LOG_NAME,
    KILO_SANDBOX_DIR_HASH,
    KILO_SANDBOX_SESSION_HASH,
    KILO_SESSION,
    KILO_SHARE_MTIME,
    KILO_SHARE_REL,
    KILO_TASK,
    bad_line_text,
    build_kilo_state,
    write_bad_line,
)

from .base import ParserBase


class KiloCodeTests(ParserBase):
    """Test the Kilo Code parser."""

    DATABASE = ".local/share/kilo/kilo.db"
    TASK = (
        ".config/Code/User/globalStorage/kilocode.kilo-code/tasks/%s/api_conversation_history.json"
        % KILO_TASK
    )

    def test_rows(self):
        rows = self.rows_for(KiloCodeParser(), self.DATABASE)
        # The V2 session_message copy of the tool call is not repeated.
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
                "system",
                "user",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id, row.project_path),
                ("h1", "alice", "kilo-code", KILO_SESSION, "/srv/proj"),
            )
        start, model, mode, user, use, result, switch, mode_switch, user2 = rows
        self.assertEqual(
            [(row.text, row.timestamp_utc) for row in (model, mode)],
            [
                ("model: claude-sonnet-4-5 provider=anthropic", "2026-10-03T10:00:01.000Z"),
                ("permission mode: agent=build", "2026-10-03T10:00:01.000Z"),
            ],
        )
        # The later user message changes model and agent (from the V1 message,
        # since V2 switch records are read only for sessions with no V1 rows).
        self.assertEqual(
            [(row.text, row.timestamp_utc, row.model) for row in (switch, mode_switch)],
            [
                (
                    "model: claude-sonnet-4-5 -> gpt-5 provider=openai",
                    "2026-10-03T10:00:04.000Z",
                    "openai/gpt-5",
                ),
                (
                    "permission mode: agent=build -> agent=plan",
                    "2026-10-03T10:00:04.000Z",
                    "openai/gpt-5",
                ),
            ],
        )
        self.assertEqual(user2.text, "plan the next fix")
        self.assertIn("session start: fix tests", start.text)
        self.assertEqual((user.text, user.model), ("fix the tests", "anthropic/claude-sonnet-4-5"))
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text), ("bash", "call_1", "pytest -q")
        )
        self.assertEqual(
            (result.tool_use_id, result.text, result.timestamp_utc),
            ("call_1", "3 passed", "2026-10-03T10:00:03.000Z"),
        )

    def test_legacy_task(self):
        rows = self.rows_for(KiloCodeParser(), self.TASK)
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "user", "system", "assistant", "tool_use", "tool_result", "assistant"],
        )
        for row in rows:
            self.assertEqual(
                (row.agent, row.session_id, row.project_path, row.model),
                ("kilo-code", KILO_TASK, "/srv/proj", ""),
            )
        start, user, env, asst, use, result, final = rows
        self.assertEqual(start.text, "task: fix tests mode=code status=completed")
        self.assertEqual(
            (user.text, user.timestamp_utc, user.source_line),
            ("<task>fix tests</task>", "2026-10-02T10:00:01.000Z", 1),
        )
        self.assertTrue(env.text.startswith("<environment_details>"))
        self.assertEqual(asst.text, "Running the suite.")
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text),
            ("execute_command", "toolu_01A", "pytest -q"),
        )
        self.assertEqual((result.tool_use_id, result.text), ("toolu_01A", "3 passed"))
        self.assertEqual((final.text, final.source_line), ("All three tests pass.", 5))

    def test_task_file_under_any_user_directory(self):
        rel = "User/globalStorage/kilocode.kilo-code/tasks/t1/api_conversation_history.json"
        self.assertTrue(is_task_file(rel))
        self.assertTrue(is_task_file(".config/VSCodium/" + rel))
        self.assertFalse(is_task_file(".config/Code/X" + rel))
        self.assertFalse(is_task_file(rel.replace("api_conversation_history", "ui_messages")))

    def test_thinking_opt_in(self):
        rows = self.rows_for(KiloCodeParser(), self.TASK, include_thinking=True)
        self.assertEqual([row.text for row in rows if row.turn_type == "thinking"], ["tests pass"])

    def test_history_item_preferred_over_index(self):
        task_dir = (self.home / self.TASK).parent
        (task_dir / "history_item.json").write_text(
            '{"id":"%s","task":"from item","workspace":"/srv/other","ts":1}' % KILO_TASK
        )
        rows = self.rows_for(KiloCodeParser(), self.TASK)
        self.assertEqual(rows[0].text, "task: from item")
        self.assertEqual({row.project_path for row in rows}, {"/srv/other"})

    def test_credentials_and_ui_messages_not_wanted(self):
        parser = KiloCodeParser()
        wanted = sorted(
            artifact.rel
            for artifact in self.col.artifacts
            if artifact.agent == "kilo-code" and parser.wants(artifact)
        )
        self.assertEqual(
            wanted,
            [self.TASK, self.DATABASE, ".local/share/kilo/storage/session_share/ses_01JAX.json"],
        )

    def test_truncated_task_file_keeps_earlier_records(self):
        path = self.home / self.TASK
        raw = path.read_bytes()
        offset = raw.index(b'{"type": "reasoning"')
        cut = raw[: offset + 10].decode("utf-8")
        path.write_text(cut, encoding="utf-8")
        start = len(raw[:offset].decode("utf-8"))
        try:
            json.JSONDecoder().raw_decode(cut, start)
        except ValueError as error:
            reason = str(error)
        else:
            raise AssertionError("cut record parses")
        rows = self.rows_for(KiloCodeParser(), self.TASK)
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "user", "system", "assistant", "tool_use", "tool_result", "system"],
        )
        # The cut record is carried from its first character to the end.
        self.assertEqual(
            (rows[-1].source_line, rows[-1].session_id, rows[-1].text),
            (4, KILO_TASK, "parser: unparseable record 4 (%s): %s" % (reason, cut[start:])),
        )

    def test_bad_line_appended(self):
        write_bad_line(self.home / self.TASK)
        rows = self.rows_for(KiloCodeParser(), self.TASK)
        self.assertEqual(len(rows), 8)
        self.assertIn("unexpected data after the array", rows[-1].text)

    def test_iter_json_array(self):
        def run(raw: str) -> tuple[list, list]:
            errors: list = []
            return kilo_iter_json_array(raw, errors), errors

        self.assertEqual(run('[1, {"a": 2} ,3]'), ([1, {"a": 2}, 3], []))
        self.assertEqual(run("[]"), ([], []))
        try:
            json.JSONDecoder().raw_decode('[1, {"a"', 4)
        except ValueError as error:
            reason = str(error)
        else:
            raise AssertionError("cut record parses")
        self.assertEqual(
            run('[1, {"a"'), ([1], [(2, "unparseable record 2 (%s)" % reason, '{"a"')])
        )
        self.assertEqual(run("[1, 2"), ([1, 2], [(3, "array cut off after record 2", "")]))
        self.assertEqual(run('{"a":1}'), ([], [(1, "not a JSON array", "")]))
        self.assertEqual(run("[1] x"), ([1], [(2, "unexpected data after the array", "")]))

    # -- standing state, past use, remote control, retention ------------------------
    LOG = ".local/share/kilo/log/" + KILO_LOG_NAME
    CONFIG = ".config/kilo/kilo.jsonc"
    POLICY = ".local/state/kilo-sandbox-policy/%s/%s.json" % (
        KILO_SANDBOX_SESSION_HASH,
        KILO_SANDBOX_DIR_HASH,
    )
    PREFERENCE = ".local/state/kilo-sandbox-preference/%s.json" % KILO_SANDBOX_DIR_HASH
    RETENTION = ".local/share/kilo/retention/state.json"
    HISTORY = ".local/state/kilo/prompt-history.jsonl"

    def with_state(self):
        build_kilo_state(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")

    def test_config_permission_rows(self):
        self.with_state()
        rows = self.rows_for(KiloCodeParser(), self.CONFIG)
        source = "(from %s, file mtime)" % self.CONFIG
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    "",
                    to_utc(KILO_SHARE_MTIME),
                    "permission setting: %s %s" % (text, source),
                )
                for text in (
                    "permission.bash.git status=allow",
                    "permission.bash.rm *=deny",
                    "permission.webfetch=ask",
                )
            ],
        )
        (self.home / self.CONFIG).write_text('{"permission": "allow"}', encoding="utf-8")
        rows = self.rows_for(KiloCodeParser(), self.CONFIG)
        self.assertEqual(
            [row.text for row in rows], ["permission setting: permission=allow " + source]
        )
        (self.home / self.CONFIG).write_text('{"model": "x"}', encoding="utf-8")
        self.assertEqual(self.rows_for(KiloCodeParser(), self.CONFIG), [])
        (self.home / self.CONFIG).write_text('{"permission": {', encoding="utf-8")
        rows = self.rows_for(KiloCodeParser(), self.CONFIG)
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("parser: %s could not be decoded" % self.CONFIG))

    def test_sandbox_rows(self):
        self.with_state()
        rows = self.rows_for(KiloCodeParser(), self.POLICY)
        source = "(from %s, file mtime)" % self.POLICY
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: sandbox.%s %s" % (text, source)
                for text in (
                    "enabled=true",
                    "mode=proxy",
                    'allowedHosts=["registry.npmjs.org"]',
                    'writablePaths=["/srv/proj"]',
                    "version=3",
                )
            ],
        )
        self.assertEqual(
            {(row.session_id, row.timestamp_utc) for row in rows}, {("", to_utc(KILO_SHARE_MTIME))}
        )
        rows = self.rows_for(KiloCodeParser(), self.PREFERENCE)
        self.assertEqual(
            [row.text for row in rows],
            ["permission setting: sandbox.enabled=false (from %s, file mtime)" % self.PREFERENCE],
        )
        (self.home / self.PREFERENCE).write_text("{", encoding="utf-8")
        rows = self.rows_for(KiloCodeParser(), self.PREFERENCE)
        self.assertEqual(
            [row.text for row in rows], ["parser: %s is not true or false" % self.PREFERENCE]
        )
        (self.home / self.POLICY).write_text("[1]", encoding="utf-8")
        rows = self.rows_for(KiloCodeParser(), self.POLICY)
        self.assertEqual(
            [row.text for row in rows], ["parser: %s is not a JSON object" % self.POLICY]
        )

    def test_retention_row(self):
        self.with_state()
        rows = self.rows_for(KiloCodeParser(), self.RETENTION)
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    "",
                    "2026-10-03T12:00:00.000Z",
                    "sessions deleted by retention: deleted=12 scanned=40 skippedActive=1 "
                    "failed=0 durationMs=830 reclaimedBytes=4096 (from retention/state.json, at)",
                )
            ],
        )
        (self.home / self.RETENTION).write_text("[]", encoding="utf-8")
        rows = self.rows_for(KiloCodeParser(), self.RETENTION)
        self.assertEqual(
            [row.text for row in rows], ["parser: retention/state.json is not a JSON object"]
        )

    def test_remote_control_rows(self):
        self.with_state()
        rows = self.rows_for(KiloCodeParser(), self.LOG)
        source = "(from log/%s, " % KILO_LOG_NAME
        self.assertEqual(
            [
                (row.source_line, row.session_id, row.project_path, row.timestamp_utc, row.text)
                for row in rows
            ],
            [
                (
                    2,
                    "",
                    "",
                    "2026-10-03T10:00:05.000Z",
                    "remote control: connection enabled connected=true " + source + "line time)",
                ),
                (
                    3,
                    KILO_SESSION,
                    "/srv/proj",
                    "2026-10-03T10:00:09.250Z",
                    "remote control: session started model=anthropic/claude-sonnet-4-5 "
                    "directory=/srv/proj " + source + "startedAt)",
                ),
                (
                    4,
                    KILO_SESSION,
                    "",
                    "2026-10-03T10:05:00.000Z",
                    "remote control: session ended reason=disabled durationMs=290750 "
                    "startedAt=2026-10-03T10:00:09.250Z " + source + "line time)",
                ),
                (
                    5,
                    "",
                    "",
                    "2026-10-03T10:05:00.000Z",
                    "remote control: connection disabled " + source + "line time)",
                ),
            ],
        )
        self.assertFalse(KiloCodeParser.records_relay or KiloCodeParser.records_origin)

    def test_remote_control_line_without_fields(self):
        self.with_state()
        bad = "INFO  2026-10-03T10:06:00 +1ms service=kilo-sessions remote session started"
        with open(self.home / self.LOG, "a", encoding="utf-8") as handle:
            handle.write(bad + "\n")
            handle.write(
                "INFO  2026-10-03T10:07:00 +1ms service=kilo-sessions connected=false "
                "remote connection enabled\n"
            )
        rows = self.rows_for(KiloCodeParser(), self.LOG)
        self.assertEqual(len(rows), 6)
        self.assertTrue(rows[4].text.startswith("remote control: connection enabled connected=f"))
        self.assertEqual(
            (rows[5].text, rows[5].source_line, rows[5].timestamp_utc),
            (
                "parser: unparseable line (remote session started without its fields): " + bad,
                6,
                "",
            ),
        )

    def test_prompt_history_is_secondary(self):
        self.with_state()
        parser = KiloCodeParser()
        self.assertTrue(KiloCodeParser.has_secondary)
        self.assertTrue(parser.secondary(self.artifact(self.HISTORY)))
        self.assertFalse(parser.secondary(self.artifact(self.DATABASE)))
        rows = self.rows_for(parser, self.HISTORY)
        self.assertEqual(
            [(row.turn_type, row.tool_name, row.text, row.timestamp_utc) for row in rows],
            [("user", "", "fix the tests", ""), ("tool_use", "shell", "ls -la", "")],
        )
        path = self.home / self.HISTORY
        write_bad_line(path)
        with open(path, "a", encoding="utf-8") as handle:
            handle.write('\n{"input": "after the cut", "parts": []}\n')
        rows = self.rows_for(parser, self.HISTORY)
        self.assertEqual(
            [row.text for row in rows],
            ["fix the tests", "ls -la", "after the cut", bad_line_text()],
        )
        rows = self.rows_for(parser, ".local/state/kilo/prompt-stash.jsonl")
        self.assertEqual(
            [row.text for row in rows],
            [
                "prompt stashed (unsent): stashed idea "
                "(from .local/state/kilo/prompt-stash.jsonl, timestamp)"
            ],
        )

    def test_v1_refusal_in_kilo_database(self):
        # packages/core/src/v1/permission.ts:7-27 at 87cc35c: the error's
        # message is the part's `state.error` (session/processor.ts:299-312).
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.DATABASE))
        data = {
            "type": "tool",
            "callID": "call_k9",
            "tool": "bash",
            "state": {
                "status": "error",
                "input": {"command": "rm -rf /srv/proj/build"},
                "error": "The user rejected permission to use this specific tool call with the "
                "following feedback: not that directory",
                "time": {"start": 1791021603500, "end": 1791021603600},
            },
        }
        connection.execute(
            "INSERT INTO part VALUES ('prt_01JAXZ','msg_01JAXA',?,1791021603500,1791021603500,?)",
            (KILO_SESSION, json.dumps(data)),
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(KiloCodeParser(), self.DATABASE)
        dec = [row for row in rows if row.text.startswith("permission decision:")]
        self.assertEqual(
            [(row.text, row.tool_use_id, row.timestamp_utc) for row in dec],
            [
                (
                    "permission decision: rejected with feedback",
                    "call_k9",
                    "2026-10-03T10:00:03.600Z",
                )
            ],
        )

    def test_memory_digest_row(self):
        self.with_state()
        rows = self.rows_for(KiloCodeParser(), KILO_DIGEST_REL)
        self.assertEqual(
            [
                (
                    row.turn_type,
                    row.session_id,
                    row.project_path,
                    row.timestamp_utc,
                    row.source_line,
                    row.text,
                )
                for row in rows
            ],
            [
                (
                    "system",
                    KILO_SESSION,
                    "/srv/proj",
                    "2026-10-03T10:30:00.000Z",
                    8,
                    "memory digest: test fixes: Ran the suite; three tests pass. "
                    "(from %s, Updated)" % KILO_DIGEST_REL,
                )
            ],
        )
        self.assertFalse(KiloCodeParser().secondary(self.artifact(KILO_DIGEST_REL)))

    def test_memory_digest_keeps_every_summary_line(self):
        self.with_state()
        path = self.home / KILO_DIGEST_REL
        path.write_bytes(
            b"# Session s2\r\nUpdated: 2026-10-03T11:00:00.000Z\r\n\r\n## Summary\r\n"
            b"first line\r\nsecond line\r\n"
        )
        rows = self.rows_for(KiloCodeParser(), KILO_DIGEST_REL)
        self.assertEqual(
            [(row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "s2",
                    "2026-10-03T11:00:00.000Z",
                    "memory digest: -: first line\nsecond line (from %s, Updated)"
                    % KILO_DIGEST_REL,
                )
            ],
        )

    def test_memory_digest_problems(self):
        self.with_state()
        path = self.home / KILO_DIGEST_REL
        manifest = path.parent.parent / "manifest.json"
        path.write_text("# Session s3\nUpdated: 2026-10-03T11:00:00.000Z\n", encoding="utf-8")
        rows = self.rows_for(KiloCodeParser(), KILO_DIGEST_REL)
        self.assertEqual(
            [(row.session_id, row.project_path, row.text) for row in rows],
            [
                (
                    "s3",
                    "/srv/proj",
                    "parser: %s is not a memory digest (no ## Summary line)" % KILO_DIGEST_REL,
                )
            ],
        )
        path.write_text("# Session s3\n## Summary\ndone\n", encoding="utf-8")
        manifest.write_text("[1]", encoding="utf-8")
        rows = self.rows_for(KiloCodeParser(), KILO_DIGEST_REL)
        self.assertEqual(
            [(row.project_path, row.text) for row in rows],
            [
                ("", "memory digest: -: done (from %s, Updated)" % KILO_DIGEST_REL),
                ("", "parser: memory manifest.json is not a JSON object"),
            ],
        )
        # A manifest written without the project's identity has no `canonical`, and says nothing.
        manifest.write_text('{"kind": "kilo-memory"}', encoding="utf-8")
        rows = self.rows_for(KiloCodeParser(), KILO_DIGEST_REL)
        self.assertEqual([(row.project_path, row.turn_type) for row in rows], [("", "system")])
        manifest.unlink()
        rows = self.rows_for(KiloCodeParser(), KILO_DIGEST_REL)
        self.assertEqual(len(rows), 1)

    def test_memory_notes_not_wanted(self):
        self.with_state()
        folder = (self.home / KILO_DIGEST_REL).parent.parent
        for name in ("project.md", "manifest.json", "state.json"):
            (folder / name).write_text("{}", encoding="utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        for name in ("project.md", "manifest.json", "state.json"):
            rel = ".local/share/kilo/memory/%s/%s" % (folder.name, name)
            self.assertFalse(KiloCodeParser().wants(self.artifact(rel)), rel)

    def test_state_files_wanted_by_kilo_only(self):
        self.with_state()
        for rel in (
            self.LOG,
            self.CONFIG,
            self.POLICY,
            self.PREFERENCE,
            self.RETENTION,
            KILO_DIGEST_REL,
        ):
            artifact = self.artifact(rel)
            self.assertEqual(artifact.agent, "kilo-code", rel)
            self.assertTrue(KiloCodeParser().wants(artifact), rel)
            self.assertFalse(OpenCodeParser().wants(artifact), rel)
        # Kilo's own auth file stays unwanted.
        self.assertFalse(KiloCodeParser().wants(self.artifact(".local/share/kilo/auth.json")))


class KiloCodeSyncTests(ParserBase):
    """Test Kilo's cloud session marker."""

    def test_rows(self):
        self.assertTrue(KiloCodeParser.records_sync)
        rows = self.rows_for(KiloCodeParser(), KILO_SHARE_REL)
        self.assertEqual(
            [
                (row.session_id, row.turn_type, row.timestamp_utc, row.source_line, row.text)
                for row in rows
            ],
            [
                (
                    KILO_SESSION,
                    "system",
                    to_utc(KILO_SHARE_MTIME),
                    1,
                    "session synced: kilo session ks_01JAX ingest /api/session/ks_01JAX/ingest "
                    "share https://app.kilo.ai/s/share01 (from storage/session_share/%s.json, "
                    "file mtime)" % KILO_SESSION,
                )
            ],
        )

    def test_unshared_marker_prints_dash(self):
        path = self.home / KILO_SHARE_REL
        path.write_text('{"id": "ks_01JAX", "ingestPath": "/api/x"}', encoding="utf-8")
        rows = self.rows_for(KiloCodeParser(), KILO_SHARE_REL)
        self.assertIn(" share - (from", rows[0].text)

    def test_session_without_marker_has_no_sync_row(self):
        rows = self.rows_for(KiloCodeParser(), ".local/share/kilo/kilo.db")
        self.assertEqual([row for row in rows if row.text.startswith("session synced:")], [])

    def test_not_a_json_object(self):
        for raw in ("[1, 2]", '{"id": "ks_01'):
            (self.home / KILO_SHARE_REL).write_text(raw, encoding="utf-8")
            rows = self.rows_for(KiloCodeParser(), KILO_SHARE_REL)
            self.assertEqual(
                [(row.session_id, row.turn_type, row.text) for row in rows],
                [
                    (
                        KILO_SESSION,
                        "system",
                        "parser: storage/session_share/%s.json is not a JSON object" % KILO_SESSION,
                    )
                ],
            )

    def test_opencode_does_not_want_the_marker(self):
        self.assertFalse(OpenCodeParser().wants(self.artifact(KILO_SHARE_REL)))
        self.assertTrue(KiloCodeParser().wants(self.artifact(KILO_SHARE_REL)))
