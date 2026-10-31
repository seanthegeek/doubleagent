"""Tests of the nanobot parser."""

import io
import json
import sqlite3
from types import SimpleNamespace
from typing import ClassVar, cast

from doubleagent import cli
from doubleagent.inputs import Artifact, open_input
from doubleagent.parsers.nanobot import NanobotParser, decode_stem
from doubleagent.timeutil import to_utc
from fixtures import (
    NANOBOT_CLI_HISTORY_REL,
    NANOBOT_DATABASE_BACKUP_REL,
    NANOBOT_DATABASE_FROZEN_REL,
    NANOBOT_DATABASE_KEY,
    NANOBOT_DATABASE_REL,
    NANOBOT_DATABASE_WORKSPACE,
    NANOBOT_HISTORY_REL,
    NANOBOT_KEY,
    NANOBOT_LEGACY_REL,
    NANOBOT_REL,
    NANOBOT_SWEEP_RELS,
    NANOBOT_TOKEN,
    NANOBOT_WEBUI_KEY,
    NANOBOT_WEBUI_REL,
    NANOBOT_WEBUI_SEGMENT_REL,
    NANOBOT_WEBUI_SESSION_REL,
    bad_line_text,
    nanobot_database_insert_message,
    write_bad_line,
)

from .base import ParserBase, read_jsonl, rel_only


class NanobotTests(ParserBase):
    """Test the nanobot parser."""

    def test_cron_run_id_start_takes_ascii_digits_only(self):
        path = self.home / NANOBOT_SWEEP_RELS["cron"]
        record = json.loads(path.read_text(encoding="utf-8"))
        digits = "".join(chr(0x0660 + int(digit)) for digit in "1791187200000")
        record["run_id"] = "job1:%s:ab12cd34" % digits
        path.write_text(json.dumps(record), encoding="utf-8")
        rows = self.rows_for(NanobotParser(), NANOBOT_SWEEP_RELS["cron"])
        # Not an ASCII epoch: the prompt takes the run's end, not a start read from the id.
        self.assertEqual(rows[0].timestamp_utc, "2026-10-05T08:00:05.000Z")

    def test_rows(self):
        rows = self.rows_for(NanobotParser(), NANOBOT_REL)
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "system", "system", "user", "tool_use", "tool_result", "assistant"],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "nanobot", NANOBOT_KEY),
            )
            self.assertEqual((row.project_path, row.git_branch, row.model), ("/srv/proj", "", ""))
        start, relayed, hidden, user, use, result, asst = rows
        self.assertIn("last_channel=telegram:123456789", start.text)
        self.assertEqual(start.timestamp_utc, "2026-10-01T10:15:00.000Z")
        # The session key names a messaging platform; the metadata's
        # last_channel is the same route, so one row.
        self.assertEqual(
            (relayed.text, relayed.timestamp_utc, relayed.source_line),
            ("session relayed: telegram chat 123456789 (from session key)", start.timestamp_utc, 1),
        )
        self.assertTrue(hidden.text.startswith("summarised history:"))
        self.assertEqual(
            (user.text, user.timestamp_utc),
            ("what is using port 8080?", "2026-10-01T10:15:01.200Z"),
        )
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text),
            ("exec", "call_abc123", "ss -ltnp | grep 8080"),
        )
        self.assertEqual(
            (result.tool_use_id, result.timestamp_utc), ("call_abc123", "2026-10-01T10:15:05.000Z")
        )
        self.assertEqual(asst.text, "python3 (pid 4242) is listening on 8080.")
        self.assertEqual([row.source_line for row in rows], [1, 1, 3, 4, 5, 6, 7])

    def test_thinking_opt_in(self):
        rows = self.rows_for(NanobotParser(), NANOBOT_REL, include_thinking=True)
        self.assertEqual(
            [row.text for row in rows if row.turn_type == "thinking"], ["Check listening sockets."]
        )

    def test_memory_history(self):
        rows = self.rows_for(NanobotParser(), NANOBOT_HISTORY_REL)
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc) for row in rows],
            [
                ("system", NANOBOT_KEY, "2026-09-30T22:10:00.000Z"),
                ("system", "", "2026-10-01T10:20:00.000Z"),
            ],
        )
        self.assertEqual(rows[0].text, "memory history #1: User asked about disk usage.")
        self.assertEqual(
            rows[0].project_path, "/alice/.nanobot/workspace"
        )  # loose input: original path

    def test_legacy_file(self):
        rows = self.rows_for(NanobotParser(), NANOBOT_LEGACY_REL)
        self.assertEqual(
            [(row.turn_type, row.session_id, row.model) for row in rows],
            [
                ("system", "cli:direct", "fast"),
                ("system", "cli:direct", "fast"),
                ("user", "cli:direct", "fast"),
                ("assistant", "cli:direct", "fast"),
            ],
        )
        self.assertEqual(
            (rows[1].text, rows[1].timestamp_utc, rows[1].source_line),
            ("model: fast (preset)", rows[0].timestamp_utc, 1),
        )
        self.assertEqual(
            rows[3].timestamp_utc, "2026-09-01T08:00:01.000Z"
        )  # inherits the previous message
        self.assertEqual(rows[0].project_path, "")

    def test_decode_stem(self):
        self.assertEqual(decode_stem("dGVsZWdyYW06MTIzNDU2Nzg5"), NANOBOT_KEY)
        self.assertEqual(decode_stem("Y2xpOmRpcmVjdA"), "cli:direct")
        self.assertEqual(decode_stem("cli_direct"), "cli_direct")

    def test_not_wanted(self):
        parser = NanobotParser()
        for rel in (
            ".nanobot/workspace/cron/jobs.json",
            ".nanobot/workspace/cron/action.jsonl",
            ".nanobot/workspace/triggers/triggers.json",
            ".nanobot/workspace/triggers/inbox/d-2.json",
            ".nanobot/sessions/%s/.workspace" % "0123456789abcdef0123456789abcdef",
            ".nanobot/sessions/0123456789abcdef0123456789abcdef/dGVsZWdyYW06MTIzNDU2Nzg5.checkpoint.json",
        ):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertFalse(parser.wants(art), rel)
        for rel in (
            ".nanobot-work/sessions/0123456789abcdef0123456789abcdef/Y2xpOmRpcmVjdA.jsonl",
            ".nanobot/sessions/0123456789abcdef0123456789abcdef/.migration-conflicts/Y2xpOmRpcmVjdA.jsonl",
        ):
            self.assertTrue(parser.wants(rel_only(rel)), rel)
        for rel in (
            ".nanobot/webui/websocket_chat-1.segments/manifest.json",
            ".nanobot/webui/sidebar-state.json",
        ):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertFalse(parser.wants(art), rel)
        for rel in (
            ".nanobot/webui/websocket_chat-1.json",  # legacy thread, shape not recorded
            ".nanobot/history/cli_history.bak",
            ".nanobot/workspace/config.json",
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)

    def test_truncated_line_is_reported_not_fatal(self):
        write_bad_line(self.home / NANOBOT_REL)
        with open(self.home / NANOBOT_REL, "a", encoding="utf-8") as handle:
            handle.write(
                "\n" + '{"role": "user", "content": "after the cut", '
                '"timestamp": "2026-10-01T10:16:00.000000"}' + "\n"
            )
        rows = self.rows_for(NanobotParser(), NANOBOT_REL)
        self.assertEqual(len(rows), 9)
        self.assertEqual(
            (rows[-2].source_line, rows[-2].turn_type, rows[-2].text), (9, "user", "after the cut")
        )
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].session_id, rows[-1].source_line),
            ("system", NANOBOT_KEY, 8),
        )
        self.assertEqual(rows[-1].project_path, rows[0].project_path)
        self.assertEqual(rows[-1].text, bad_line_text())

    def test_secondary(self):
        parser = NanobotParser()
        for rel in (NANOBOT_WEBUI_REL, NANOBOT_WEBUI_SEGMENT_REL, NANOBOT_CLI_HISTORY_REL):
            self.assertTrue(parser.secondary(self.artifact(rel)), rel)
        for rel in (
            NANOBOT_REL,
            NANOBOT_WEBUI_SESSION_REL,
            NANOBOT_LEGACY_REL,
            NANOBOT_HISTORY_REL,
        ):
            self.assertFalse(parser.secondary(self.artifact(rel)), rel)
        self.assertEqual(parser.secondary_tolerance(self.artifact(NANOBOT_WEBUI_REL)), 15 * 3600.0)
        self.assertEqual(parser.secondary_tolerance(self.artifact(NANOBOT_CLI_HISTORY_REL)), 2.0)

    def test_webui_rows(self):
        rows = self.rows_for(NanobotParser(), NANOBOT_WEBUI_REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text, row.timestamp_utc) for row in rows],
            [
                (1, "user", "show disk usage", "2026-10-02T07:00:00.900Z"),
                (3, "tool_use", "df -h", "2026-10-02T07:00:02.000Z"),
                (4, "tool_result", "/dev/sda1 42%", "2026-10-02T07:00:04.000Z"),
                (
                    5,
                    "system",
                    "file edit: done /srv/proj/notes.txt +1 -0",
                    "2026-10-02T07:00:04.500Z",
                ),
                (8, "assistant", "The disk is 42% full.", "2026-10-02T07:00:05.500Z"),
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.agent, row.session_id, row.project_path),
                ("nanobot", NANOBOT_WEBUI_KEY, "/srv/proj"),
            )
        self.assertEqual(
            [(row.tool_name, row.tool_use_id) for row in rows[1:4]],
            [("exec", "call_df1"), ("exec", "call_df1"), ("write_file", "call_w1")],
        )
        thinking = self.rows_for(NanobotParser(), NANOBOT_WEBUI_REL, include_thinking=True)
        self.assertEqual([row.text for row in thinking if row.turn_type == "thinking"], ["Run df."])

    def test_webui_segment(self):
        rows = self.rows_for(NanobotParser(), NANOBOT_WEBUI_SEGMENT_REL)
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc, row.session_id) for row in rows],
            [
                ("user", "list the old logs", "2026-09-20T12:00:00.000Z", NANOBOT_WEBUI_KEY),
                (
                    "assistant",
                    "There are 3 old logs.",
                    "2026-09-20T12:00:03.000Z",
                    NANOBOT_WEBUI_KEY,
                ),
                (
                    "system",
                    "webui fork marker: the fork starts here",
                    "2026-09-20T12:00:04.000Z",
                    NANOBOT_WEBUI_KEY,
                ),
            ],
        )
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})

    def test_webui_session_from_file_name(self):
        path = self.home / NANOBOT_WEBUI_REL
        path.write_text('{"event": "user", "text": "no chat id"}\n', encoding="utf-8")
        rows = self.rows_for(NanobotParser(), NANOBOT_WEBUI_REL)
        self.assertEqual(
            [(row.session_id, row.project_path, row.timestamp_utc) for row in rows],
            [(NANOBOT_WEBUI_KEY, "/srv/proj", "")],
        )

    def test_webui_truncated_line(self):
        write_bad_line(self.home / NANOBOT_WEBUI_REL)
        with open(self.home / NANOBOT_WEBUI_REL, "a", encoding="utf-8") as handle:
            handle.write(
                "\n" + '{"event": "user", "chat_id": "chat-1", "text": "after the cut", '
                '"created_at_ms": 1790924407000}' + "\n"
            )
        rows = self.rows_for(NanobotParser(), NANOBOT_WEBUI_REL)
        self.assertEqual(len(rows), 7)
        self.assertEqual(
            (rows[-2].source_line, rows[-2].turn_type, rows[-2].text), (11, "user", "after the cut")
        )
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].session_id, rows[-1].source_line),
            ("system", NANOBOT_WEBUI_KEY, 10),
        )
        self.assertEqual(rows[-1].project_path, rows[-2].project_path)
        self.assertEqual(rows[-1].text, bad_line_text())

    def test_cli_history(self):
        rows = self.rows_for(NanobotParser(), NANOBOT_CLI_HISTORY_REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text, row.timestamp_utc) for row in rows],
            [
                (2, "user", "hello", "2026-09-01T08:00:01.200Z"),
                (5, "user", "first line\nsecond line", "2026-09-01T07:59:00.000Z"),
            ],
        )
        self.assertEqual({(row.session_id, row.project_path) for row in rows}, {("", "")})

    def test_cli_history_bad_line(self):
        with open(self.home / NANOBOT_CLI_HISTORY_REL, "a", encoding="utf-8") as handle:
            handle.write("not a history line\n\n# 2026-09-01 08:10:00.000000\n+after the cut\n")
        rows = self.rows_for(NanobotParser(), NANOBOT_CLI_HISTORY_REL)
        self.assertEqual([row.turn_type for row in rows], ["user", "user", "user", "system"])
        self.assertEqual((rows[1].text, rows[2].text), ("first line\nsecond line", "after the cut"))
        self.assertEqual(rows[-1].source_line, 8)
        self.assertEqual(
            rows[-1].text, "parser: unparseable line (not a FileHistory line): not a history line"
        )

    def test_timeline_pairs_secondary_rows(self):
        import contextlib

        out = self.tmp / "out"
        with contextlib.redirect_stdout(io.StringIO()):
            cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), "--agent", "nanobot"])
        rows = read_jsonl(out / "timeline.jsonl")
        by_text: dict = {}
        for row in rows:
            by_text.setdefault(row["text"], []).append(row["source_file"].rsplit("/", 1)[-1])
        # The session file's copy survives; the WebUI and CLI history copies go.
        self.assertEqual(by_text["show disk usage"], ["d2Vic29ja2V0OmNoYXQtMQ.jsonl"])
        self.assertEqual(by_text["df -h"], ["d2Vic29ja2V0OmNoYXQtMQ.jsonl"])
        self.assertEqual(by_text["The disk is 42% full."], ["d2Vic29ja2V0OmNoYXQtMQ.jsonl"])
        self.assertEqual(by_text["hello"], ["cli_direct.jsonl"])
        # What only the secondary sources still hold is kept.
        self.assertEqual(by_text["list the old logs"], ["000001.jsonl"])
        self.assertEqual(by_text["first line\nsecond line"], ["cli_history"])
        inv = json.loads((out / "inventory.json").read_text(encoding="utf-8"))
        # Six more: the frozen discord:555 JSONL's relay row, model row and
        # four message rows, which sessions.sqlite3 also holds.
        self.assertEqual(inv["deduplicated"], {"nanobot": 11})

    # -- sessions.sqlite3 --------------------------------------------------------
    DATABASE_ROWS: ClassVar[list[tuple[int, str, str, str]]] = [
        (
            2,
            "system",
            "heartbeat",
            "session start: key=heartbeat updated_at=2026-10-03T06:00:05 workspace=/srv/proj",
        ),
        (7, "user", "heartbeat", "check the inbox"),
        (8, "assistant", "heartbeat", "Inbox is empty."),
        (
            1,
            "system",
            NANOBOT_DATABASE_KEY,
            "session start: key=discord:555 last_channel=discord:555"
            " updated_at=2026-10-03T11:00:31 workspace=/srv/proj",
        ),
        (1, "system", NANOBOT_DATABASE_KEY, "session relayed: discord chat 555 (from session key)"),
        (1, "system", NANOBOT_DATABASE_KEY, "model: smart (preset)"),
        (1, "user", NANOBOT_DATABASE_KEY, "rotate the api key"),
        (2, "tool_use", NANOBOT_DATABASE_KEY, "./rotate.sh"),
        (3, "tool_result", NANOBOT_DATABASE_KEY, "rotated"),
        (4, "assistant", NANOBOT_DATABASE_KEY, "The key is rotated."),
        (5, "user", NANOBOT_DATABASE_KEY, "thanks"),
        (6, "assistant", NANOBOT_DATABASE_KEY, "You're welcome."),
    ]

    def test_sqlite_store_rows(self):
        parser = NanobotParser()
        self.assertTrue(parser.wants(self.artifact(NANOBOT_DATABASE_REL)))
        self.assertFalse(parser.secondary(self.artifact(NANOBOT_DATABASE_REL)))
        for rel in (NANOBOT_DATABASE_REL + "-wal", NANOBOT_DATABASE_REL + "-shm"):
            self.assertFalse(parser.wants(rel_only(rel)), rel)
        rows = self.rows_for(parser, NANOBOT_DATABASE_REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.session_id, row.text) for row in rows],
            self.DATABASE_ROWS,
        )
        self.assertEqual(
            {(row.agent, row.project_path) for row in rows}, {("nanobot", "/srv/proj")}
        )
        by_text = {row.text: row for row in rows}
        self.assertEqual(by_text["thanks"].timestamp_utc, "2026-10-03T11:00:30.000Z")
        self.assertEqual(
            (by_text["./rotate.sh"].tool_name, by_text["./rotate.sh"].tool_use_id),
            ("exec", "call_rot1"),
        )
        self.assertEqual(by_text["rotated"].tool_use_id, "call_rot1")
        self.assertEqual(
            {row.model for row in rows if row.session_id == NANOBOT_DATABASE_KEY}, {"smart"}
        )

    def test_sqlite_frozen_jsonl_is_secondary(self):
        parser = NanobotParser()
        for rel in (NANOBOT_DATABASE_FROZEN_REL, NANOBOT_DATABASE_BACKUP_REL):
            self.assertTrue(parser.wants(self.artifact(rel)), rel)
            self.assertTrue(parser.secondary(self.artifact(rel)), rel)
        # No database beside it: the session file is still the store of record.
        self.assertFalse(parser.secondary(self.artifact(NANOBOT_REL)))
        conflict = (
            ".nanobot/sessions/%s/.migration-conflicts/Y2xpOmRpcmVjdA.jsonl"
            % NANOBOT_DATABASE_WORKSPACE
        )
        self.assertTrue(parser.wants(rel_only(conflict)))
        backup = self.rows_for(parser, NANOBOT_DATABASE_BACKUP_REL)
        self.assertEqual(
            [(row.turn_type, row.session_id, row.project_path, row.text) for row in backup[1:]],
            [
                ("user", "heartbeat", "/srv/proj", "old heartbeat prompt"),
                ("assistant", "heartbeat", "/srv/proj", "Nothing to do."),
            ],
        )
        self.assertFalse(
            parser.wants(
                rel_only(NANOBOT_DATABASE_BACKUP_REL.replace(".jsonl", ".checkpoint.json"))
            )
        )

    def test_sqlite_bad_record(self):
        database = self.home / NANOBOT_DATABASE_REL
        cut = '{"role": "user", "content": "cut'
        nanobot_database_insert_message(database, NANOBOT_DATABASE_KEY, 6, cut)
        nanobot_database_insert_message(database, NANOBOT_DATABASE_KEY, 7, "[1, 2]")
        nanobot_database_insert_message(
            database,
            NANOBOT_DATABASE_KEY,
            8,
            '{"role": "user", "content": "after the cut", "timestamp": "2026-10-03T11:01:00"}',
        )
        rows = self.rows_for(NanobotParser(), NANOBOT_DATABASE_REL)
        reason = ""
        try:
            json.loads(cut)
        except ValueError as error:
            reason = str(error)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[-5:]],
            [
                (5, "user", "thanks"),
                (6, "assistant", "You're welcome."),
                (11, "user", "after the cut"),
                (9, "system", "parser: unparseable record 6 (%s): %s" % (reason, cut)),
                (10, "system", "parser: unparseable record 7 (not a JSON object): [1, 2]"),
            ],
        )
        for row in rows[-2:]:
            self.assertEqual(
                (row.session_id, row.project_path, row.timestamp_utc),
                (NANOBOT_DATABASE_KEY, "/srv/proj", ""),
            )

    def test_sqlite_bad_metadata(self):
        connection = sqlite3.connect(str(self.home / NANOBOT_DATABASE_REL))
        connection.execute("UPDATE sessions SET metadata = 'not json' WHERE key = 'heartbeat'")
        connection.commit()
        connection.close()
        rows = [
            row
            for row in self.rows_for(NanobotParser(), NANOBOT_DATABASE_REL)
            if row.session_id == "heartbeat"
        ]
        reason = ""
        try:
            json.loads("not json")
        except ValueError as error:
            reason = str(error)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                *[(line, turn_type, text) for line, turn_type, _, text in self.DATABASE_ROWS[:3]],
                (2, "system", "parser: unparseable session metadata (%s): not json" % reason),
            ],
        )

    def test_sqlite_not_a_database(self):
        (self.home / NANOBOT_DATABASE_REL).write_bytes(b"not a database")
        rows = self.rows_for(NanobotParser(), NANOBOT_DATABASE_REL)
        self.assertEqual(
            [(row.turn_type, row.project_path, row.text) for row in rows],
            [("system", "/srv/proj", "parser: not a SQLite database")],
        )

    def test_sqlite_timeline_dedup(self):
        import contextlib

        out = self.tmp / "out"
        with contextlib.redirect_stdout(io.StringIO()):
            cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), "--agent", "nanobot"])
        rows = read_jsonl(out / "timeline.jsonl")
        by_text: dict = {}
        for row in rows:
            by_text.setdefault(row["text"], []).append(
                (row["source_file"].rsplit("/", 1)[-1], row["recovered"])
            )
        database = ("sessions.sqlite3", "")
        for text in ("rotate the api key", "./rotate.sh", "rotated", "The key is rotated."):
            self.assertEqual(by_text[text], [database], text)
        self.assertEqual(by_text["model: smart (preset)"], [database])
        self.assertEqual(by_text["thanks"], [database])
        # What only the jsonl-backup/ copy still holds is recovered from it.
        self.assertEqual(by_text["old heartbeat prompt"], [("heartbeat.jsonl", "yes")])

    # -- CLI history under an instance home -------------------------------------
    def test_cli_history_under_instance_home(self):
        rel = ".nanobot-work/history/cli_history"
        path = self.home / rel
        path.parent.mkdir(parents=True)
        path.write_text("\n# 2026-10-04 09:00:00.000000\n+deploy the bot\n", encoding="utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        parser = NanobotParser()
        self.assertTrue(parser.wants(self.artifact(rel)))
        self.assertTrue(parser.secondary(self.artifact(rel)))
        rows = self.rows_for(parser, rel)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text, row.timestamp_utc) for row in rows],
            [(2, "user", "deploy the bot", "2026-10-04T09:00:00.000Z")],
        )
        self.assertFalse(parser.wants(rel_only(".nanobot-work/history/cli_history.bak")))
        self.assertFalse(parser.wants(rel_only("work/.nanobot/history/cli_history")))

    # -- relay, model switches, permission state, run records, sidebar ---------
    def test_relay_from_last_channel(self):
        rows = self.rows_for(NanobotParser(), NANOBOT_SWEEP_RELS["unified"])
        self.assertEqual(
            [row.text for row in rows[:2]],
            [
                "session start: key=unified:default last_channel=telegram:777"
                " updated_at=2026-10-04T08:05:00 workspace=/srv/proj",
                "session relayed: telegram chat 777 (from metadata.last_channel)",
            ],
        )
        self.assertTrue(NanobotParser.records_relay)
        # The WebUI, CLI, heartbeat and legacy sessions are local.
        for rel in (NANOBOT_WEBUI_SESSION_REL, NANOBOT_LEGACY_REL):
            rows = self.rows_for(NanobotParser(), rel)
            self.assertFalse([row for row in rows if row.text.startswith("session relayed:")], rel)

    def test_model_switch_rows(self):
        rows = self.rows_for(NanobotParser(), NANOBOT_SWEEP_RELS["unified"])
        models = [
            (row.source_line, row.text, row.model) for row in rows if row.text.startswith("model:")
        ]
        # The metadata's preset (`smart`) is the last switch's, so no start
        # row names it; the first switch has no recorded previous preset,
        # and the failed `/model nope` switches nothing.
        self.assertEqual(
            models,
            [
                (5, "model: fast (preset) requested=fast resolved=gpt-5.4-mini", "fast"),
                (
                    7,
                    "model: fast -> smart (preset) requested=smart resolved=claude-opus-5-5",
                    "smart",
                ),
            ],
        )
        by_text = {row.text: row for row in rows}
        self.assertEqual(
            (by_text["good morning"].model, by_text["/model nope"].model), ("", "smart")
        )
        self.assertEqual(by_text["/model fast"].turn_type, "user")

    def test_permission_mode_from_workspace_scope(self):
        rows = self.rows_for(NanobotParser(), NANOBOT_WEBUI_SESSION_REL)
        self.assertEqual(
            (rows[1].turn_type, rows[1].text, rows[1].timestamp_utc, rows[1].source_line),
            (
                "system",
                "permission mode: access_mode=restricted project=/srv/proj"
                " (from session metadata workspace_scope)",
                rows[0].timestamp_utc,
                1,
            ),
        )

    def test_config_permission_settings(self):
        parser = NanobotParser()
        art = self.artifact(NANOBOT_SWEEP_RELS["config"])
        self.assertTrue(parser.wants(art))
        self.assertFalse(parser.secondary(art))
        rows = self.rows_for(parser, NANOBOT_SWEEP_RELS["config"])
        tail = " (from .nanobot/config.json, file mtime)"
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: " + text + tail
                for text in (
                    "tools.restrictToWorkspace=true",
                    "tools.ssrfWhitelist=100.64.0.0/10",
                    "tools.exec.enable=true",
                    "tools.exec.sandbox=bwrap",
                    "tools.exec.denyPatterns=rm -rf /",
                    "tools.exec.denyPatterns=curl *",
                    "channels.telegram.allowFrom=123456789",
                )
            ],
        )
        mtime = to_utc((self.home / NANOBOT_SWEEP_RELS["config"]).stat().st_mtime)
        for row in rows:
            self.assertEqual(
                (row.turn_type, row.session_id, row.project_path, row.timestamp_utc),
                ("system", "", "", mtime),
            )
            self.assertNotIn(NANOBOT_TOKEN, row.text)
            self.assertNotIn("123:abc", row.text)

    def test_config_snake_case_and_bad_file(self):
        path = self.home / NANOBOT_SWEEP_RELS["config"]
        path.write_text('{"tools": {"restrict_to_workspace": false}}', encoding="utf-8")
        rows = self.rows_for(NanobotParser(), NANOBOT_SWEEP_RELS["config"])
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: tools.restrict_to_workspace=false"
                " (from .nanobot/config.json, file mtime)"
            ],
        )
        path.write_text('{"tools": ', encoding="utf-8")
        rows = self.rows_for(NanobotParser(), NANOBOT_SWEEP_RELS["config"])
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("parser: config.json did not parse: "))

    def test_pairing_rows(self):
        rows = self.rows_for(NanobotParser(), NANOBOT_SWEEP_RELS["pairing"])
        mtime = to_utc((self.home / NANOBOT_SWEEP_RELS["pairing"]).stat().st_mtime)
        self.assertEqual(
            [(row.timestamp_utc, row.text) for row in rows],
            [
                (
                    mtime,
                    "permission setting: pairing.approved.telegram=123456789"
                    " (from .nanobot/pairing.json, file mtime)",
                ),
                (
                    mtime,
                    "permission setting: pairing.approved.telegram=555"
                    " (from .nanobot/pairing.json, file mtime)",
                ),
                (
                    "2026-10-05T08:00:00.500Z",
                    "permission setting: pairing.pending.discord=999"
                    " expires_at=2026-10-05T08:10:00.500Z delivery_status=sent"
                    " (from .nanobot/pairing.json, created_at)",
                ),
            ],
        )
        self.assertFalse([row for row in rows if "ABCD-EFGH" in row.text])

    def test_cron_run_record(self):
        parser = NanobotParser()
        rel = NANOBOT_SWEEP_RELS["cron"]
        self.assertTrue(parser.secondary(self.artifact(rel)))
        self.assertEqual(parser.secondary_tolerance(self.artifact(rel)), 15 * 3600.0)
        rows = self.rows_for(parser, rel)
        self.assertEqual(
            [(row.timestamp_utc, row.turn_type, row.text) for row in rows],
            [
                (
                    "2026-10-05T08:00:00.000Z",
                    "user",
                    "Scheduled cron job triggered: morning digest\n\nsummarise the inbox",
                ),
                ("2026-10-05T08:00:05.000Z", "assistant", "Three new mails."),
                (
                    "2026-10-05T08:00:05.000Z",
                    "system",
                    "cron run: job morning digest (job1) run job1:1791187200000:ab12cd34 status=ok",
                ),
            ],
        )
        self.assertEqual(
            {(row.session_id, row.project_path) for row in rows},
            {(NANOBOT_SWEEP_RELS["run_key"], "/alice/.nanobot/workspace")},
        )

    def test_trigger_run_record(self):
        rel = NANOBOT_SWEEP_RELS["trigger"]
        self.assertTrue(NanobotParser().secondary(self.artifact(rel)))
        rows = self.rows_for(NanobotParser(), rel)
        self.assertEqual(
            [(row.timestamp_utc, row.turn_type, row.text) for row in rows],
            [
                (
                    "2026-10-05T09:00:00.000Z",
                    "user",
                    "Local trigger received: ci\n\nbuild failed on main",
                ),
                (
                    "2026-10-05T09:00:01.000Z",
                    "system",
                    "trigger run: trigger ci (t1) delivery d-1 status=error attempts=1"
                    " error=target channel is not enabled: slack",
                ),
            ],
        )

    def test_run_record_in_project_and_bad_file(self):
        parser = NanobotParser()
        project = cast(
            Artifact, SimpleNamespace(rel="cron/runs/x.json", home=SimpleNamespace(project=True))
        )
        home = cast(
            Artifact, SimpleNamespace(rel="cron/runs/x.json", home=SimpleNamespace(project=False))
        )
        self.assertTrue(parser.wants(project))
        self.assertFalse(parser.wants(home))
        self.assertTrue(parser.wants(rel_only(".nanobot-work/triggers/runs/d.json")))
        (self.home / NANOBOT_SWEEP_RELS["cron"]).write_text("[]", encoding="utf-8")
        rows = self.rows_for(parser, NANOBOT_SWEEP_RELS["cron"])
        self.assertEqual(
            [(row.text, row.project_path) for row in rows],
            [("parser: cron run record is not a JSON object", "/alice/.nanobot/workspace")],
        )

    def test_sidebar_cache(self):
        parser = NanobotParser()
        rel = NANOBOT_SWEEP_RELS["sidebar"]
        self.assertTrue(parser.secondary(self.artifact(rel)))
        rows = self.rows_for(parser, rel)
        self.assertEqual(
            [(row.timestamp_utc, row.turn_type, row.session_id, row.text) for row in rows],
            [
                (
                    "2026-10-04T09:00:00.000Z",
                    "system",
                    "websocket:chat-9",
                    "webui sidebar entry: key=websocket:chat-9 preview=draft the release notes"
                    " updated_at=2026-10-04T09:10:00 (from .webui_session_index.json version 8)",
                )
            ],
        )

    def test_sidebar_bad_entry(self):
        path = self.home / NANOBOT_SWEEP_RELS["sidebar"]
        data = json.loads(path.read_text(encoding="utf-8"))
        data["sessions"] = ["cut", *data["sessions"]]
        path.write_text(json.dumps(data), encoding="utf-8")
        rows = self.rows_for(NanobotParser(), NANOBOT_SWEEP_RELS["sidebar"])
        self.assertEqual([row.session_id for row in rows], ["websocket:chat-9", ""])
        self.assertEqual(rows[-1].text, 'parser: unparseable record 1 (not a JSON object): "cut"')

    def test_sweep_timeline(self):
        import contextlib

        out = self.tmp / "out"
        with contextlib.redirect_stdout(io.StringIO()):
            cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), "--agent", "nanobot"])
        sessions = {
            session["session_id"]: session
            for session in read_jsonl(out / "sessions.jsonl")
            if session["agent"] == "nanobot"
        }
        self.assertEqual(sessions["telegram:123456789"]["relayed"], "yes")
        self.assertEqual(sessions["unified:default"]["relayed"], "yes")
        self.assertEqual(sessions["discord:555"]["relayed"], "yes")
        self.assertEqual(sessions["websocket:chat-1"]["relayed"], "no")
        # The run records' session is gone: its turns are recovered from them.
        self.assertEqual(sessions[NANOBOT_SWEEP_RELS["run_key"]]["recovered"], "yes")
