"""Tests of the Goose parser."""

import json

from doubleagent.dedup import deduplicate
from doubleagent.inputs import open_input
from doubleagent.parsers.goose import GooseParser, unescape_history
from doubleagent.timeutil import to_utc
from fixtures import (
    GOOSE_GATEWAY_SESSION,
    GOOSE_LEGACY_REL,
    GOOSE_PERMISSION_SESSION,
    GOOSE_PERMISSION_YAML,
    GOOSE_SCHEDULE,
    GOOSE_SESSION,
    GOOSE_STANDING_MTIME,
    bad_line_text,
    build_goose_standing,
    goose_gateway_records,
    goose_permission_records,
    goose_streamed_llm_records,
    write_bad_line,
)

from .base import ParserBase, model_or_permission_row


class GooseTests(ParserBase):
    """Test the Goose parser."""

    REL = ".local/share/goose/sessions/sessions.db"
    HISTORY = ".local/state/goose/history.txt"

    def test_secondary(self):
        # The CLI history is a secondary source, the database is not.
        parser = GooseParser()
        self.assertTrue(parser.secondary(self.artifact(self.HISTORY)))
        self.assertFalse(parser.secondary(self.artifact(self.REL)))
        self.assertFalse(parser.secondary(self.artifact(GOOSE_LEGACY_REL)))
        # The request logs repeat the database's messages, with a
        # wider window; the history keeps the default.
        log = self.artifact(".local/state/goose/logs/llm_request.0.jsonl")
        self.assertTrue(parser.secondary(log))
        self.assertEqual(parser.secondary_tolerance(log), 600.0)
        self.assertEqual(parser.secondary_tolerance(self.artifact(self.HISTORY)), 2.0)

    def test_history_rows_have_no_time_session_or_project(self):
        # "run the tests" repeats the database's first prompt; the second
        # entry is only in the history. Neither carries time or scope, so
        # dedup matches them on text alone.
        rows = self.rows_for(GooseParser(), self.HISTORY)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [(2, "user", "run the tests"), (3, "user", "line one\nline two \\ done")],
        )
        self.assertEqual(
            {(row.timestamp_utc, row.session_id, row.project_path) for row in rows}, {("", "", "")}
        )
        from_database = [
            row.text for row in self.rows_for(GooseParser(), self.REL) if row.turn_type == "user"
        ]
        self.assertIn("run the tests", from_database)
        self.assertNotIn("line one\nline two \\ done", from_database)

    def test_old_history_file_is_read_and_secondary(self):
        old = self.home / ".config/goose/history.txt"
        old.write_text("#V2\nan older prompt\n", encoding="utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        parser = GooseParser()
        self.assertTrue(parser.wants(self.artifact(".config/goose/history.txt")))
        self.assertTrue(parser.secondary(self.artifact(".config/goose/history.txt")))
        rows = self.rows_for(parser, ".config/goose/history.txt")
        self.assertEqual([(row.turn_type, row.text) for row in rows], [("user", "an older prompt")])
        # Both history files are read when both exist.
        self.assertEqual(len(self.rows_for(parser, self.HISTORY)), 2)

    def test_malformed_history_is_not_fatal(self):
        # Bytes that are not UTF-8 and a last line cut without its newline.
        (self.home / self.HISTORY).write_bytes(b"#V2\nfirst\n\xff\xfebad\ncut mid-wri")
        rows = self.rows_for(GooseParser(), self.HISTORY)
        self.assertEqual([row.text for row in rows], ["first", "��bad", "cut mid-wri"])

    def test_rows(self):
        rows = [
            row
            for row in self.rows_for(GooseParser(), self.REL)
            if not model_or_permission_row(row)
        ]
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "user", "tool_use", "tool_result", "assistant"],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "goose", GOOSE_SESSION),
            )
            self.assertEqual((row.project_path, row.git_branch), ("/srv/proj", ""))
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        # the last assistant message names its own provider and model
        self.assertEqual(
            [row.model for row in rows],
            ["anthropic/claude-sonnet-4-5"] * 4 + ["openai/gpt-4.1-2025-04-14"],
        )
        start, user, use, result, asst = rows
        self.assertEqual(
            (start.text, start.timestamp_utc),
            ("session start | Fix flaky test | type=user", "2026-03-01T09:00:00.000Z"),
        )
        self.assertEqual(
            (user.text, user.timestamp_utc), ("run the tests", "2026-03-01T09:00:00.000Z")
        )
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text),
            ("developer__shell", "call_1", '{"command":"pytest -q"}'),
        )
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.text),
            ("developer__shell", "call_1", "3 passed"),
        )
        self.assertEqual(
            (asst.text, asst.timestamp_utc), ("All 3 tests pass.", "2026-03-01T09:00:05.000Z")
        )
        self.assertEqual([row.source_line for row in rows], [1, 1, 2, 3, 4])

    def test_thinking_opt_in(self):
        rows = self.rows_for(GooseParser(), self.REL, include_thinking=True)
        self.assertEqual([row.text for row in rows if row.turn_type == "thinking"], ["all green"])

    def test_legacy_request_log_and_history(self):
        rows = self.rows_for(GooseParser(), GOOSE_LEGACY_REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("system", "session start (legacy) | old chat | 1 messages"), ("user", "hello")],
        )
        self.assertEqual(
            {(row.session_id, row.project_path) for row in rows}, {("20260301_090000", "/srv/old")}
        )
        rows = self.rows_for(GooseParser(), ".local/state/goose/logs/llm_request.0.jsonl")
        self.assertEqual(
            [(row.turn_type, row.model, row.text) for row in rows],
            [
                ("system", "gpt-4.1", "llm request | 1 messages | last user: hello"),
                ("assistant", "gpt-4.1", "hi"),
            ],
        )
        self.assertEqual({row.timestamp_utc for row in rows}, {"2026-03-01T09:00:01.000Z"})
        rows = self.rows_for(GooseParser(), ".local/state/goose/history.txt")
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("user", "run the tests"), ("user", "line one\nline two \\ done")],
        )
        self.assertEqual(unescape_history("a\\nb\\\\c"), "a\nb\\c")

    def test_millisecond_timestamps(self):
        from doubleagent.parsers.goose import epoch

        self.assertEqual(epoch(1772355600000), "2026-03-01T09:00:00.000Z")
        self.assertEqual(
            epoch(17723556000), to_utc(17723556)
        )  # Goose divides above 1e10, to_utc only above 1e11

    def test_config_secrets_and_sidecars_not_wanted(self):
        parser = GooseParser()
        build_goose_standing(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        for artifact in self.col.artifacts:
            if artifact.agent != "goose" or artifact.rel.endswith("/permission.yaml"):
                continue
            if artifact.rel.endswith((".yaml", "-wal", "-shm", ".lock", "/roaming_peers.json")):
                self.assertFalse(parser.wants(artifact), artifact.rel)
        for rel in (
            ".config/goose/permission.yaml",
            ".config/goose/roaming_trust.json",
            ".local/share/goose/schedule.json",
        ):
            self.assertTrue(parser.wants(self.artifact(rel)), rel)

    def test_truncated_line_is_reported_not_fatal(self):
        write_bad_line(self.home / GOOSE_LEGACY_REL)
        with open(self.home / GOOSE_LEGACY_REL, "a", encoding="utf-8") as handle:
            handle.write(
                "\n" + '{"id": "m9", "role": "user", "created": 1772355700, '
                '"content": [{"type": "text", "text": "after the cut"}]}' + "\n"
            )
        rows = self.rows_for(GooseParser(), GOOSE_LEGACY_REL)
        self.assertEqual(
            [(row.source_line, row.turn_type) for row in rows],
            [(1, "system"), (2, "user"), (4, "user"), (3, "system")],
        )
        self.assertEqual((rows[1].text, rows[2].text), ("hello", "after the cut"))
        self.assertEqual(rows[-1].text, bad_line_text())
        self.assertEqual(rows[-1].session_id, "20260301_090000")

    def test_bad_content_json_is_reported_not_fatal(self):
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.REL))
        connection.execute(
            "INSERT INTO messages(session_id,role,content_json,created_timestamp) "
            "VALUES ('20260301_1','assistant','[{\"type\":\"text\",\"te',1772355606)"
        )
        connection.commit()
        connection.close()
        rows = [
            row
            for row in self.rows_for(GooseParser(), self.REL)
            if not model_or_permission_row(row)
        ]
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "user", "tool_use", "tool_result", "assistant", "system"],
        )
        # The stored value in full, not a summary of it.
        self.assertEqual(
            rows[-1].text,
            "parser: unparseable record 5 (messages.content_json: Unterminated string"
            ' starting at: line 1 column 17 (char 16)): [{"type":"text","te',
        )

    def test_model_and_mode_rows(self):
        rows = self.rows_for(GooseParser(), self.REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows[:4]],
            [
                ("system", "session start | Fix flaky test | type=user"),
                ("system", "model: claude-sonnet-4-5 provider=anthropic"),
                ("system", "permission mode: auto"),
                ("user", "run the tests"),
            ],
        )
        for row in rows[1:3]:
            self.assertEqual(
                (row.timestamp_utc, row.session_id, row.project_path, row.model, row.source_line),
                (
                    "2026-03-01T09:00:00.000Z",
                    GOOSE_SESSION,
                    "/srv/proj",
                    "anthropic/claude-sonnet-4-5",
                    1,
                ),
            )
        # message 2 requests the session's model: no row; message 4 switches
        change = [row for row in rows if row.text.startswith("model: ") and "->" in row.text]
        self.assertEqual(len(change), 1)
        first_change = change[0]
        self.assertEqual(
            first_change.text,
            "model: claude-sonnet-4-5 -> gpt-4.1-2025-04-14 provider=openai requested=gpt-4.1",
        )
        self.assertEqual(
            (
                first_change.turn_type,
                first_change.timestamp_utc,
                first_change.session_id,
                first_change.model,
                first_change.source_line,
            ),
            ("system", "2026-03-01T09:00:05.000Z", GOOSE_SESSION, "openai/gpt-4.1-2025-04-14", 4),
        )
        self.assertEqual(rows[rows.index(first_change) + 1].text, "All 3 tests pass.")

    def test_legacy_and_request_log_have_no_model_rows(self):
        for rel in (GOOSE_LEGACY_REL, ".local/state/goose/logs/llm_request.0.jsonl"):
            self.assertFalse(
                [row for row in self.rows_for(GooseParser(), rel) if model_or_permission_row(row)]
            )

    # -- relayed gateway sessions -----------------------------------------------

    def _gateway_rows(self, display="alice"):
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.REL))
        for sql in goose_gateway_records(display):
            connection.execute(sql)
        connection.commit()
        connection.close()
        return [
            row
            for row in self.rows_for(GooseParser(), self.REL)
            if row.session_id == GOOSE_GATEWAY_SESSION
        ]

    def test_gateway_session_row(self):
        self.assertFalse(GooseParser.records_sync)
        rows = self._gateway_rows()
        self.assertEqual(rows[0].text, "session start | telegram/alice | type=gateway")
        relayed = rows[1]
        self.assertEqual(
            (relayed.turn_type, relayed.timestamp_utc, relayed.project_path, relayed.text),
            (
                "system",
                "2026-10-17T08:00:00.000Z",
                "/home/alice/.config/goose/gateway/telegram/123456789",
                "session relayed: gateway telegram user 123456789 display alice"
                " (from sessions.db session_type gateway, created_at)",
            ),
        )
        self.assertEqual(rows[-1].text, "disk usage?")

    def test_gateway_session_without_display_name(self):
        rows = self._gateway_rows(display="123456789")
        self.assertEqual(
            rows[1].text,
            "session relayed: gateway telegram user 123456789 display -"
            " (from sessions.db session_type gateway, created_at)",
        )

    def test_user_session_has_no_relayed_row(self):
        rows = self.rows_for(GooseParser(), self.REL)
        self.assertFalse([row for row in rows if row.text.startswith("session relayed:")])

    # -- bad stored JSON in sessions.db -----------------------------------------

    def _bad_json_rows(self):
        import sqlite3

        from fixtures import GOOSE_BAD_SESSION, goose_bad_json_records

        connection = sqlite3.connect(str(self.home / self.REL))
        for sql in goose_bad_json_records():
            connection.execute(sql)
        connection.commit()
        connection.close()
        return [
            row
            for row in self.rows_for(GooseParser(), self.REL)
            if row.session_id == GOOSE_BAD_SESSION
        ]

    def test_bad_model_config_json_is_its_own_row(self):
        from fixtures import GOOSE_BAD_MODEL_CONFIG

        rows = self._bad_json_rows()
        self.assertEqual(rows[0].text, "session start | bad json | type=user")
        bad = rows[1]
        self.assertEqual(
            bad.text,
            "parser: unparseable record 2 (sessions.model_config_json: Expecting property"
            " name enclosed in double quotes: line 1 column 25 (char 24)): "
            + GOOSE_BAD_MODEL_CONFIG,
        )
        self.assertEqual(
            (bad.turn_type, bad.timestamp_utc, bad.project_path, bad.source_line),
            ("system", "", "/srv/bad", 2),
        )
        # The session has no model, so no `model:` row; the mode row follows.
        self.assertEqual(rows[2].text, "permission mode: auto")
        self.assertFalse([row for row in rows if row.text.startswith("model:")])

    def test_bad_metadata_json_is_its_own_row(self):
        from fixtures import GOOSE_BAD_METADATA

        rows = self._bad_json_rows()
        texts = [row.text for row in rows]
        i = texts.index("kept despite bad metadata")
        bad = rows[i - 1]
        self.assertEqual(
            bad.text,
            "parser: unparseable record 102 (messages.metadata_json: Expecting value:"
            " line 1 column 16 (char 15)): " + GOOSE_BAD_METADATA,
        )
        self.assertEqual(
            (bad.turn_type, bad.timestamp_utc, bad.project_path, bad.source_line),
            ("system", "", "/srv/bad", 102),
        )
        # The content is kept, as visible text at the message's time.
        self.assertEqual(
            (rows[i].turn_type, rows[i].timestamp_utc), ("assistant", "2026-10-19T08:00:01.000Z")
        )

    def test_bad_content_json_carries_the_stored_value(self):
        from fixtures import GOOSE_BAD_CONTENT

        rows = self._bad_json_rows()
        self.assertEqual(
            [(row.source_line, row.turn_type) for row in rows if row.source_line > 100],
            [(101, "user"), (102, "system"), (102, "assistant"), (103, "system"), (104, "user")],
        )
        bad = next(row for row in rows if row.source_line == 103)
        self.assertEqual(
            bad.text,
            "parser: unparseable record 103 (messages.content_json: Unterminated string"
            " starting at: line 1 column 17 (char 16)): " + GOOSE_BAD_CONTENT,
        )
        self.assertEqual((bad.timestamp_utc, bad.project_path), ("", "/srv/bad"))
        self.assertEqual(
            [row.text for row in rows if row.turn_type == "user"],
            ["before the bad rows", "after the bad rows"],
        )

    def test_content_json_of_the_wrong_shape_or_empty_is_reported(self):
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.REL))
        connection.execute(
            "INSERT INTO messages(id,session_id,role,content_json,created_timestamp) VALUES "
            "(201,'20260301_1','assistant','{\"type\":\"text\"}',1772355606),"
            "(202,'20260301_1','assistant','',1772355607)"
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(GooseParser(), self.REL)
        self.assertEqual(
            [row.text for row in rows if row.source_line in (201, 202)],
            [
                "parser: unparseable record 201 (messages.content_json: not a JSON array):"
                ' {"type":"text"}',
                "parser: unparseable record 202 (messages.content_json: empty)",
            ],
        )
        self.assertEqual(rows[-1].session_id, GOOSE_SESSION)

    # -- tool confirmations and standing permissions ---------------------------

    def _permission_rows(self):
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.REL))
        for sql in goose_permission_records():
            connection.execute(sql)
        connection.commit()
        connection.close()
        return [
            row
            for row in self.rows_for(GooseParser(), self.REL)
            if row.session_id == GOOSE_PERMISSION_SESSION
        ]

    def test_tool_confirmation_rows(self):
        rows = self._permission_rows()
        self.assertEqual(
            [
                (row.source_line, row.turn_type, row.tool_name, row.tool_use_id, row.text)
                for row in rows[2:]
            ],
            [
                (301, "tool_use", "developer__shell", "call_p1", '{"command":"rm -rf build"}'),
                (
                    302,
                    "system",
                    "developer__shell",
                    "call_p1",
                    'permission request: developer__shell | {"command":"rm -rf build"}',
                ),
                (
                    303,
                    "system",
                    "developer__shell",
                    "call_p1",
                    "permission decision: allow_once (toolConfirmationResponse)",
                ),
                (304, "tool_result", "developer__shell", "call_p1", "removed"),
                (305, "tool_use", "developer__text_editor", "call_p2", '{"path":"/etc/hosts"}'),
                (
                    305,
                    "system",
                    "developer__text_editor",
                    "call_p2",
                    "permission decision: not executable (goose.executable=false)",
                ),
                (
                    306,
                    "tool_result",
                    "developer__text_editor",
                    "call_p2",
                    "The user has declined to run this tool. DO NOT attempt to call this tool"
                    " again.",
                ),
                (
                    307,
                    "system",
                    "",
                    "",
                    'action required: {"actionType":"elicitation","id":"el_1",'
                    '"message":"Pick one","requestedSchema":{}}',
                ),
            ],
        )
        self.assertEqual(rows[1].text, "permission mode: smart_approve")
        decision = rows[4]
        self.assertEqual(
            (decision.timestamp_utc, decision.project_path),
            ("2026-10-21T08:00:05.000Z", "/srv/proj"),
        )

    def test_answered_executable_false_has_no_verdict_row(self):
        # A request the user answered keeps only the answer's decision row,
        # even while its `_meta` still says false.
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.REL))
        for sql in goose_permission_records():
            connection.execute(sql.replace('"goose.executable":true', '"goose.executable":false'))
        connection.commit()
        connection.close()
        rows = self.rows_for(GooseParser(), self.REL)
        verdicts = [row.tool_use_id for row in rows if "goose.executable" in row.text]
        self.assertEqual(verdicts, ["call_p2"])

    def test_legacy_and_log_have_no_verdict_rows(self):
        for rel in (GOOSE_LEGACY_REL, ".local/state/goose/logs/llm_request.0.jsonl"):
            rows = self.rows_for(GooseParser(), rel)
            self.assertFalse([row for row in rows if row.text.startswith("permission ")])

    def _standing(self, rel):
        build_goose_standing(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        return self.rows_for(GooseParser(), rel)

    def test_permission_yaml_rows(self):
        rel = ".config/goose/permission.yaml"
        rows = self._standing(rel)
        self.assertEqual(
            [(row.source_line, row.text) for row in rows],
            [
                (
                    3,
                    "permission setting: user.always_allow=developer__shell (from %s, file mtime)"
                    % rel,
                ),
                (
                    6,
                    "permission setting: user.never_allow=developer__text_editor"
                    " (from %s, file mtime)" % rel,
                ),
                (
                    10,
                    "permission setting: smart_approve.ask_before="
                    "computercontroller__automation_script (from %s, file mtime)" % rel,
                ),
            ],
        )
        self.assertEqual(
            {(row.turn_type, row.timestamp_utc, row.session_id, row.project_path) for row in rows},
            {("system", to_utc(GOOSE_STANDING_MTIME), "", "")},
        )

    def test_permission_yaml_bad_line_is_reported_not_fatal(self):
        rel = ".config/goose/permission.yaml"
        build_goose_standing(self.home)
        path = self.home / rel
        lines = GOOSE_PERMISSION_YAML.splitlines(keepends=True)
        lines.insert(3, "  ask_bef\n")  # cut mid-write
        path.write_text("".join(lines), encoding="utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(GooseParser(), rel)
        self.assertEqual([row.source_line for row in rows], [3, 7, 11, 4])
        self.assertEqual(
            rows[-1].text,
            "parser: unparseable line (not a category, level or list item):   ask_bef",
        )
        self.assertEqual((rows[-1].turn_type, rows[-1].timestamp_utc), ("system", ""))

    def test_permission_yaml_line_ends_and_trims(self):
        rel = ".config/goose/permission.yaml"
        build_goose_standing(self.home)
        path = self.home / rel

        def tools(text):
            path.write_text(text, encoding="utf-8", newline="")
            self.col = open_input(self.tmp / "home", self.cat, host="h1")
            return [
                (row.source_line, row.text.split(" (from")[0])
                for row in self.rows_for(GooseParser(), rel)
            ]

        prefix = "permission setting: user.always_allow="
        self.assertEqual(
            tools("user:\n  always_allow:\n  - developer__shell\u00a0\n"),
            [(3, prefix + "developer__shell\u00a0")],
        )
        self.assertEqual(
            tools("user:\n  always_allow:\n  - a\u2028b\n"), [(3, prefix + "a\u2028b")]
        )
        self.assertEqual(
            tools(GOOSE_PERMISSION_YAML.replace("\n", "\r\n")), tools(GOOSE_PERMISSION_YAML)
        )
        # A U+00A0 around a category, a level or an empty value is part of the text.
        self.assertEqual(
            tools("user\u00a0:\n  always_allow:\n  - x\n"),
            [(3, "permission setting: user\u00a0.always_allow=x")],
        )
        self.assertEqual(
            tools("user:\n  always_allow\u00a0:\n  - x\n"),
            [(3, "permission setting: user.always_allow\u00a0=x")],
        )
        self.assertNotIn((3, prefix + "x"), tools("user:\n  always_allow: \u00a0\n  - x\n"))
        # Only one LF or CRLF ends a line; a CR before it stays in the text.
        self.assertEqual(tools("user:\n  always_allow:\n  - x\r\r\n"), [(3, prefix + "x\r")])

    def test_roaming_trust_rows(self):
        rel = ".config/goose/roaming_trust.json"
        rows = self._standing(rel)
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: roaming_trust.allowed=peerkeyaaaa (from %s, file mtime)" % rel,
                "permission setting: roaming_trust.revoked_keys=peerkeybbbb"
                " (from %s, file mtime)" % rel,
            ],
        )
        self.assertEqual({row.timestamp_utc for row in rows}, {to_utc(GOOSE_STANDING_MTIME)})

    def test_roaming_trust_not_json_is_one_row(self):
        rel = ".config/goose/roaming_trust.json"
        build_goose_standing(self.home)
        (self.home / rel).write_text('{"allowed": ["peer', encoding="utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(GooseParser(), rel)
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("parser: %s is not JSON (" % rel), rows[0].text)

    def test_schedule_rows(self):
        rel = ".local/share/goose/schedule.json"
        rows = self._standing(rel)
        self.assertEqual(
            [row.text for row in rows],
            [
                "scheduled task: nightly-report 0 0 2 * * * paused=false running=false"
                " source=/home/alice/.local/share/goose/scheduled_recipes/nightly-report.yaml"
                " last_run=2026-10-21T02:00:00.123Z current_session=-"
                " recipe_base_dir=/srv/proj/recipes (from %s, file mtime)" % rel,
                "scheduled task: weekly 0 0 9 * * 1 paused=true running=false"
                " source=/home/alice/.local/share/goose/scheduled_recipes/weekly.yaml"
                ' last_run=- current_session=- recipe_base_dir=- parameters=[["env","prod"]]'
                " (from %s, file mtime)" % rel,
            ],
        )
        self.assertEqual(
            {(row.turn_type, row.timestamp_utc, row.session_id) for row in rows},
            {("system", to_utc(GOOSE_STANDING_MTIME), "")},
        )

    def test_schedule_bad_record_is_reported_not_fatal(self):
        rel = ".local/share/goose/schedule.json"
        build_goose_standing(self.home)
        (self.home / rel).write_text(
            json.dumps([GOOSE_SCHEDULE[0], "oops", GOOSE_SCHEDULE[1]]), encoding="utf-8"
        )
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(GooseParser(), rel)
        self.assertEqual(len(rows), 3)
        self.assertTrue(rows[1].text.startswith("scheduled task: weekly "))
        self.assertEqual(rows[2].text, 'parser: unparseable record 2 (not a JSON object): "oops"')

    # -- request logs as a secondary source -------------------------------------

    LOG = ".local/state/goose/logs/llm_request.0.jsonl"

    def _streamed(self):
        path = self.home / self.LOG
        path.write_text(
            "".join(json.dumps(record) + "\n" for record in goose_streamed_llm_records()),
            encoding="utf-8",
        )
        return self.rows_for(GooseParser(), self.LOG)

    def test_streamed_chunks_are_assembled(self):
        rows = self._streamed()
        self.assertEqual(
            [(row.source_line, row.turn_type, row.tool_use_id, row.text) for row in rows],
            [
                (1, "system", "", "llm request | 1 messages | last user: run the tests"),
                (2, "assistant", "", "All 3 tests pass."),
                (2, "tool_use", "call_1", '{"command":"pytest -q"}'),
            ],
        )
        self.assertEqual({row.timestamp_utc for row in rows}, {"2026-03-01T09:00:05.000Z"})

    def test_log_rows_repeating_the_database_are_dropped(self):
        parser = GooseParser()
        log = self._streamed()
        primary = self.rows_for(parser, self.REL)
        tolerance = parser.secondary_tolerance(self.artifact(self.LOG))
        kept, dropped = deduplicate(primary, [(row, tolerance) for row in log])
        self.assertEqual(dropped["goose"], 2)
        recovered = [row for row in kept if row.source_file.endswith("llm_request.0.jsonl")]
        # Only the request row, which no database row repeats, is kept.
        self.assertEqual([row.text for row in recovered], [log[0].text])
        # Within the default window the tool call, four seconds off, stays.
        _, dropped = deduplicate(primary, [(row, 2.0) for row in log])
        self.assertEqual(dropped["goose"], 1)

    def test_request_log_truncated_line_is_reported_not_fatal(self):
        path = self.home / self.LOG
        write_bad_line(path)
        with open(path, "a", encoding="utf-8") as handle:
            handle.write(
                "\n"
                + json.dumps(
                    {
                        "data": {
                            "role": "assistant",
                            "created": 1772355609,
                            "content": [{"type": "text", "text": "after the cut"}],
                        }
                    }
                )
                + "\n"
            )
        rows = self.rows_for(GooseParser(), self.LOG)
        self.assertEqual(
            [(row.source_line, row.turn_type) for row in rows],
            [(1, "system"), (2, "assistant"), (4, "assistant"), (3, "system")],
        )
        self.assertEqual((rows[1].text, rows[2].text), ("hi", "after the cut"))
        self.assertEqual(rows[-1].text, bad_line_text())
