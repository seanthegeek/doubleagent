"""Tests of the OpenCode parser."""

import io
import json
import os
import sqlite3
import unittest

from test_errors import CAN_DENY, DENY_REASON

from doubleagent import cli
from doubleagent.inputs import open_input
from doubleagent.parsers.kilo_code import KiloCodeParser
from doubleagent.parsers.opencode import OpenCodeParser
from doubleagent.parsers.opencode import tool_summary as opencode_tool_summary
from doubleagent.timeutil import to_utc
from fixtures import (
    KILO_SESSION,
    KILO_TASK,
    OPENCODE_ARCHIVED,
    OPENCODE_DELETED,
    OPENCODE_LEGACY_SESSION,
    OPENCODE_MIGRATED,
    OPENCODE_NEXT_DATABASE_NAME,
    OPENCODE_NEXT_ONLY,
    OPENCODE_PINNED_GONE,
    OPENCODE_PRIVATE,
    OPENCODE_SESSION,
    OPENCODE_SHARE_DATABASE_NAME,
    OPENCODE_SHARE_SECRET,
    OPENCODE_SHARED,
    OPENCODE_STATE_DATABASE_NAME,
    OPENCODE_STATE_MTIME,
    OPENCODE_UNSHARED,
    OPENCODE_V2_DATABASE_NAME,
    OPENCODE_V2_ONLY,
    OPENCODE_V2_SESSION,
    OPENCODE_WORKTREE,
    bad_line_text,
    build_opencode_state,
    write_bad_line,
)

from .base import ParserBase, read_jsonl


def json_reason(raw: str) -> str:
    """Return the decoder's message for `raw`, which must not decode."""
    try:
        json.loads(raw)
    except ValueError as error:
        return str(error)
    raise AssertionError("decodes: %r" % raw)


class OpenCodeTests(ParserBase):
    """Test the OpenCode parser."""

    DATABASE = ".local/share/opencode/opencode.db"
    STORAGE = ".local/share/opencode/storage/"

    def test_rows(self):
        rows = self.rows_for(OpenCodeParser(), self.DATABASE)
        self.assertEqual(
            [(row.session_id, row.turn_type) for row in rows],
            [
                (OPENCODE_SESSION, "system"),
                (OPENCODE_V2_SESSION, "system"),
                (OPENCODE_SESSION, "system"),
                (OPENCODE_SESSION, "system"),
                (OPENCODE_SESSION, "user"),
                (OPENCODE_SESSION, "system"),
                (OPENCODE_SESSION, "tool_use"),
                (OPENCODE_SESSION, "tool_result"),
                (OPENCODE_SESSION, "assistant"),
                (OPENCODE_SESSION, "system"),
                (OPENCODE_V2_SESSION, "user"),
                (OPENCODE_V2_SESSION, "system"),
                (OPENCODE_V2_SESSION, "system"),
                (OPENCODE_V2_SESSION, "assistant"),
                (OPENCODE_V2_SESSION, "tool_use"),
                (OPENCODE_V2_SESSION, "tool_result"),
                (OPENCODE_V2_SESSION, "system"),
                (OPENCODE_V2_SESSION, "system"),
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.project_path, row.git_branch),
                ("h1", "alice", "opencode", "/srv/proj", ""),
            )
            self.assertEqual(row.source_file, "/alice/" + self.DATABASE)
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        (
            start,
            _,
            model,
            mode,
            user,
            synth,
            use,
            result,
            asst,
            finish,
            v2user,
            v2model,
            v2mode,
            v2asst,
            v2use,
            v2res,
            v2agent_switch,
            v2model_switch,
        ) = rows
        self.assertIn("session start: Fix bug", start.text)
        # The assistant message names the same model and agent, so no change row.
        self.assertEqual(
            [(row.text, row.timestamp_utc, row.source_line, row.model) for row in (model, mode)],
            [
                (
                    "model: claude-sonnet-4 provider=anthropic",
                    "2026-10-03T09:00:01.000Z",
                    1,
                    "anthropic/claude-sonnet-4",
                ),
                (
                    "permission mode: agent=build",
                    "2026-10-03T09:00:01.000Z",
                    1,
                    "anthropic/claude-sonnet-4",
                ),
            ],
        )
        self.assertEqual(
            [(row.text, row.timestamp_utc) for row in (v2model, v2mode)],
            [
                ("model: gpt-5 provider=openai", "2026-10-03T09:00:12.000Z"),
                ("permission mode: agent=build", "2026-10-03T09:00:12.000Z"),
            ],
        )
        self.assertEqual(
            (v2agent_switch.text, v2agent_switch.timestamp_utc, v2agent_switch.source_line),
            ("permission mode: agent=build -> agent=plan", "2026-10-03T09:00:15.000Z", 3),
        )
        self.assertEqual(
            (
                v2model_switch.text,
                v2model_switch.timestamp_utc,
                v2model_switch.source_line,
                v2model_switch.model,
            ),
            (
                "model: gpt-5 -> gpt-5-mini provider=openai",
                "2026-10-03T09:00:16.000Z",
                4,
                "openai/gpt-5-mini",
            ),
        )
        self.assertEqual(
            (user.text, user.timestamp_utc, user.model),
            ("list files", "2026-10-03T09:00:01.000Z", "anthropic/claude-sonnet-4"),
        )
        self.assertTrue(synth.text.startswith("synthetic: <system-reminder>"))
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text, use.timestamp_utc),
            ("bash", "call_x1", "ls", "2026-10-03T09:00:02.500Z"),
        )
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.text, result.timestamp_utc),
            ("bash", "call_x1", "a.txt", "2026-10-03T09:00:03.000Z"),
        )
        self.assertEqual(asst.text, "There is one file, a.txt.")
        self.assertEqual(finish.text, "step-finish reason=stop cost=0.01 tokens_in=10 tokens_out=5")
        self.assertEqual(v2user.text, "run the tests")
        self.assertEqual((v2asst.text, v2asst.model), ("Running them.", "openai/gpt-5"))
        self.assertEqual(
            (v2use.tool_use_id, v2use.text, v2res.text), ("call_v2", "pytest -q", "3 passed")
        )

    def test_thinking_opt_in(self):
        rows = self.rows_for(OpenCodeParser(), self.DATABASE, include_thinking=True)
        self.assertEqual(
            [row.text for row in rows if row.turn_type == "thinking"], ["user wants a listing"]
        )

    def test_legacy_json_tree(self):
        parser = OpenCodeParser()
        rows = self.rows_for(
            parser, self.STORAGE + "session/proj_a1/%s.json" % OPENCODE_LEGACY_SESSION
        )
        self.assertEqual(
            [(row.turn_type, row.session_id, row.project_path) for row in rows],
            [("system", OPENCODE_LEGACY_SESSION, "/srv/proj")],
        )
        self.assertIn("Old session", rows[0].text)
        rows = self.rows_for(
            parser, self.STORAGE + "message/%s/msg_a.json" % OPENCODE_LEGACY_SESSION
        )
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [
                ("system", "model: claude-sonnet-4 provider=anthropic"),
                ("system", "permission mode: agent=build"),
                ("user", "cat the secrets file"),
            ],
        )
        self.assertEqual(
            rows[0].source_file,
            "/alice/" + self.STORAGE + "message/%s/msg_a.json" % OPENCODE_LEGACY_SESSION,
        )
        self.assertEqual(rows[2].source_file, "/alice/" + self.STORAGE + "part/msg_a/prt_a1.json")
        self.assertEqual({row.timestamp_utc for row in rows}, {"2026-10-02T09:00:01.000Z"})
        # msg_b names the model and agent msg_a did, the message before it in
        # the session directory, so its file yields no model or mode row.
        rows = self.rows_for(
            parser, self.STORAGE + "message/%s/msg_b.json" % OPENCODE_LEGACY_SESSION
        )
        self.assertEqual(
            [row.turn_type for row in rows], ["tool_use", "tool_result", "assistant", "system"]
        )
        use, result, _asst, error = rows
        self.assertEqual((use.tool_name, use.text), ("read", "/srv/proj/.env"))
        self.assertEqual(result.text, "[error] permission denied")
        self.assertEqual(error.text, "error: APIError overloaded")
        self.assertEqual(
            error.source_file,
            "/alice/" + self.STORAGE + "message/%s/msg_b.json" % OPENCODE_LEGACY_SESSION,
        )
        self.assertTrue(all(row.session_id == OPENCODE_LEGACY_SESSION for row in rows))

    def test_old_per_project_layout(self):
        base = self.home / ".local/share/opencode/project/srv-proj/storage/session"
        (base / "info").mkdir(parents=True)
        (base / "info/ses_old.json").write_text(
            '{"id":"ses_old","title":"Older","time":{"created":1790000000000}}'
        )
        (base / "message/ses_old").mkdir(parents=True)
        (base / "message/ses_old/msg_o.json").write_text(
            '{"id":"msg_o","sessionID":"ses_old","role":"user","time":{"created":1790000001000},'
            '"path":{"cwd":"/srv/proj","root":"/srv/proj"}}'
        )
        (base / "part/ses_old/msg_o").mkdir(parents=True)
        (base / "part/ses_old/msg_o/prt_o.json").write_text(
            '{"id":"prt_o","type":"text","text":"hello"}'
        )
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(
            OpenCodeParser(),
            ".local/share/opencode/project/srv-proj/storage/session/message/ses_old/msg_o.json",
        )
        self.assertEqual(
            [(row.turn_type, row.text, row.session_id, row.project_path) for row in rows],
            [("user", "hello", "ses_old", "/srv/proj")],
        )
        rows = self.rows_for(
            OpenCodeParser(),
            ".local/share/opencode/project/srv-proj/storage/session/info/ses_old.json",
        )
        self.assertEqual([row.turn_type for row in rows], ["system"])

    def test_config_parts_and_credentials_not_wanted(self):
        parser = OpenCodeParser()
        wanted = sorted(
            artifact.rel
            for artifact in self.col.artifacts
            if artifact.agent == "opencode" and parser.wants(artifact)
        )
        self.assertEqual(
            wanted,
            [
                ".local/share/opencode/opencode-beta.db",
                ".local/share/opencode/" + OPENCODE_V2_DATABASE_NAME,
                ".local/share/opencode/" + OPENCODE_NEXT_DATABASE_NAME,
                self.DATABASE,
                self.STORAGE + "message/%s/msg_a.json" % OPENCODE_LEGACY_SESSION,
                self.STORAGE + "message/%s/msg_b.json" % OPENCODE_LEGACY_SESSION,
                self.STORAGE + "session/proj_a1/%s.json" % OPENCODE_LEGACY_SESSION,
            ],
        )

    def test_truncated_part_is_reported_not_fatal(self):
        # The part file is carried in full, not counted.
        part = self.home / self.STORAGE / "part/msg_b/prt_b2.json"
        write_bad_line(part)
        raw = part.read_text(encoding="utf-8")
        rows = self.rows_for(
            OpenCodeParser(), self.STORAGE + "message/%s/msg_b.json" % OPENCODE_LEGACY_SESSION
        )
        self.assertEqual(
            [row.turn_type for row in rows], ["tool_use", "tool_result", "system", "system"]
        )
        self.assertEqual(rows[0].tool_use_id, "call_r1")
        self.assertEqual(rows[2].text, "error: APIError overloaded")
        bad = rows[3]
        self.assertEqual(
            bad.text, "parser: unparseable part prt_b2 (%s): %s" % (json_reason(raw), raw)
        )
        self.assertEqual(
            (bad.session_id, bad.project_path, bad.timestamp_utc, bad.source_line),
            (OPENCODE_LEGACY_SESSION, "/srv/proj", "", 1),
        )
        self.assertEqual(bad.source_file, "/alice/" + self.STORAGE + "part/msg_b/prt_b2.json")

    def test_truncated_message_file(self):
        write_bad_line(
            self.home / self.STORAGE / "message" / OPENCODE_LEGACY_SESSION / "msg_a.json"
        )
        rows = self.rows_for(
            OpenCodeParser(), self.STORAGE + "message/%s/msg_a.json" % OPENCODE_LEGACY_SESSION
        )
        self.assertEqual(
            [(row.turn_type, row.session_id) for row in rows], [("system", OPENCODE_LEGACY_SESSION)]
        )
        self.assertIn("could not be decoded", rows[0].text)

    def test_bad_data_column_is_reported_not_fatal(self):
        # One row per record, carrying the stored value in full.
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.DATABASE))
        part_rowid = connection.execute(
            "INSERT INTO part VALUES ('prt_09','msg_02',?,1791018005000,1791018005000,'{\"type\":\"te')",
            (OPENCODE_SESSION,),
        ).lastrowid
        message_rowid = connection.execute(
            "INSERT INTO message VALUES ('msg_09',?,1791018006000,1791018006000,'{bad')",
            (OPENCODE_SESSION,),
        ).lastrowid
        connection.commit()
        connection.close()
        rows = self.rows_for(OpenCodeParser(), self.DATABASE)
        self.assertEqual(len([row for row in rows if row.turn_type == "tool_use"]), 2)
        bad = [row for row in rows if row.text.startswith("parser:")]
        self.assertEqual(
            [
                (row.text, row.session_id, row.project_path, row.timestamp_utc, row.source_line)
                for row in bad
            ],
            [
                (
                    'parser: unparseable record %d (part.data: %s): {"type":"te'
                    % (part_rowid, json_reason('{"type":"te')),
                    OPENCODE_SESSION,
                    "/srv/proj",
                    "",
                    part_rowid,
                ),
                (
                    "parser: unparseable record %d (message.data: %s): {bad"
                    % (message_rowid, json_reason("{bad")),
                    OPENCODE_SESSION,
                    "/srv/proj",
                    "",
                    message_rowid,
                ),
            ],
        )
        # The part rows before and after the bad one are still read: the
        # bad part sorts last in msg_02, so its row follows the message's.
        texts = [row.text for row in rows if row.session_id == OPENCODE_SESSION]
        self.assertLess(texts.index("There is one file, a.txt."), texts.index(bad[0].text))
        self.assertIn("step-finish reason=stop cost=0.01 tokens_in=10 tokens_out=5", texts)

    def test_tool_summary(self):
        self.assertEqual(opencode_tool_summary({"command": "ls", "description": "list"}), "ls")
        self.assertEqual(opencode_tool_summary({"filePath": "/a"}), "/a")
        self.assertEqual(opencode_tool_summary({"pattern": "foo", "path": "/src"}), "foo | /src")
        self.assertEqual(opencode_tool_summary({"z": 1, "a": 2}), '{"a":2,"z":1}')
        self.assertEqual(opencode_tool_summary(None), "")

    def test_timeline_includes_opencode_and_kilo(self):
        out = self.tmp / "out"
        import contextlib

        with contextlib.redirect_stdout(io.StringIO()):
            cli.main(
                [
                    "timeline",
                    str(self.tmp / "home"),
                    "-o",
                    str(out),
                    "--agent",
                    "opencode",
                    "--agent",
                    "kilo-code",
                ]
            )
        sessions = {row["session_id"]: row for row in read_jsonl(out / "sessions.jsonl")}
        self.assertEqual(
            set(sessions),
            {
                OPENCODE_SESSION,
                OPENCODE_V2_SESSION,
                OPENCODE_LEGACY_SESSION,
                OPENCODE_MIGRATED,
                OPENCODE_DELETED,
                OPENCODE_V2_ONLY,
                OPENCODE_NEXT_ONLY,
                "ses_05SHARE",
                "ses_06UNSHARE",
                "ses_07PRIVATE",
                KILO_SESSION,
                KILO_TASK,
            },
        )
        self.assertEqual(sessions[OPENCODE_SESSION]["models"], ["anthropic/claude-sonnet-4"])
        self.assertEqual(sessions[OPENCODE_SESSION]["tool_calls"], 1)
        self.assertEqual(sessions[KILO_SESSION]["agent"], "kilo-code")
        self.assertEqual({session["project_path"] for session in sessions.values()}, {"/srv/proj"})

    V2_DATABASE = ".local/share/opencode/" + OPENCODE_V2_DATABASE_NAME
    NEXT_DATABASE = ".local/share/opencode/" + OPENCODE_NEXT_DATABASE_NAME

    def test_v2_database(self):
        # A database upgraded to 2.0 is read from `session_v2` and
        # `session_message`; the 1.x rows of a migrated session are not
        # repeated, and a session deleted in 2.0 is recovered from them.
        parser = OpenCodeParser()
        rows = self.rows_for(parser, self.V2_DATABASE)
        self.assertFalse(parser.secondary(self.artifact(self.V2_DATABASE)))
        got = [(row.session_id, row.turn_type, row.text, row.recovered) for row in rows]
        self.assertEqual(
            got,
            [
                (
                    OPENCODE_MIGRATED,
                    "system",
                    "session start: Migrated slug=migrated version=2.0.32 agent=build "
                    "directory=/srv/proj",
                    "",
                ),
                (
                    OPENCODE_V2_ONLY,
                    "system",
                    "session start: Start server slug=start-server version=2.0.32 agent=build "
                    "directory=/srv/proj",
                    "",
                ),
                (
                    OPENCODE_DELETED,
                    "system",
                    "session start: Deleted slug=deleted version=1.2.0 agent=build "
                    "directory=/srv/proj (only in the 1.x session table)",
                    "",
                ),
                (OPENCODE_MIGRATED, "user", "migrate me", ""),
                (OPENCODE_MIGRATED, "system", "model: claude-sonnet-4 provider=anthropic", ""),
                (OPENCODE_MIGRATED, "system", "permission mode: agent=build", ""),
                (OPENCODE_MIGRATED, "assistant", "Migrated reply.", ""),
                (OPENCODE_V2_ONLY, "user", "start the server", ""),
                (OPENCODE_V2_ONLY, "system", "model: claude-sonnet-4-5 provider=anthropic", ""),
                (OPENCODE_V2_ONLY, "system", "permission mode: agent=build", ""),
                (OPENCODE_V2_ONLY, "assistant", "Starting it.", ""),
                (OPENCODE_V2_ONLY, "tool_use", "npm start", ""),
                (OPENCODE_V2_ONLY, "tool_result", "listening on 3000", ""),
                (OPENCODE_V2_ONLY, "tool_use", "/srv/proj/.env", ""),
                (
                    OPENCODE_V2_ONLY,
                    "tool_result",
                    "[error] aborted The user declined this tool call",
                    "",
                ),
                (OPENCODE_V2_ONLY, "system", "permission decision: rejected", ""),
                (OPENCODE_V2_ONLY, "tool_use", "git status", ""),
                (OPENCODE_V2_ONLY, "tool_result", "[exit 128] fatal: not a git repository", ""),
                (OPENCODE_V2_ONLY, "system", "location switched: /srv/proj -> /srv/proj/web", ""),
                (OPENCODE_V2_ONLY, "system", "skill deploy: Run the deploy script.", ""),
                (OPENCODE_V2_ONLY, "system", "compaction auto completed Server started.", ""),
                (
                    OPENCODE_V2_ONLY,
                    "system",
                    "model: claude-sonnet-4-5 -> gpt-5 provider=openai",
                    "",
                ),
                (OPENCODE_V2_ONLY, "system", "idle: succeeded", ""),
                (OPENCODE_DELETED, "system", "permission mode: agent=build", ""),
                (OPENCODE_DELETED, "user", "what was deleted", "yes"),
                (OPENCODE_DELETED, "system", "model: claude-sonnet-4 provider=anthropic", ""),
                (OPENCODE_DELETED, "assistant", "A deleted reply.", "yes"),
            ],
        )
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})
        decision = next(row for row in rows if row.text == "permission decision: rejected")
        self.assertEqual(
            (decision.tool_name, decision.tool_use_id, decision.timestamp_utc),
            ("edit", "call_n2", "2026-10-03T11:20:02.950Z"),
        )
        shell = [row for row in rows if row.tool_use_id == "sh_01N"]
        self.assertEqual(
            [(row.tool_name, row.timestamp_utc) for row in shell],
            [("shell", "2026-10-03T11:20:04.000Z"), ("shell", "2026-10-03T11:20:04.500Z")],
        )

    def test_next_database_is_secondary(self):
        # `opencode-next.db` repeats what 2.0 imported from it, so only the
        # session it alone still holds is kept, as recovered.
        import contextlib

        parser = OpenCodeParser()
        self.assertTrue(OpenCodeParser.has_secondary)
        self.assertTrue(parser.secondary(self.artifact(self.NEXT_DATABASE)))
        self.assertFalse(parser.secondary(self.artifact(self.DATABASE)))
        out = self.tmp / "out"
        with contextlib.redirect_stdout(io.StringIO()):
            cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), "--agent", "opencode"])
        timeline = read_jsonl(out / "timeline.jsonl")
        self.assertEqual(
            [
                (row["source_file"].rsplit("/", 1)[-1], row["recovered"])
                for row in timeline
                if row["text"] == "start the server"
            ],
            [(OPENCODE_V2_DATABASE_NAME, "")],
        )
        self.assertEqual(
            [
                (row["source_file"].rsplit("/", 1)[-1], row["recovered"])
                for row in timeline
                if row["text"] == "only in next"
            ],
            [(OPENCODE_NEXT_DATABASE_NAME, "yes")],
        )
        recovered = {
            row["session_id"]: row["recovered"] for row in read_jsonl(out / "sessions.jsonl")
        }
        self.assertEqual(
            [
                recovered[session]
                for session in (
                    OPENCODE_DELETED,
                    OPENCODE_NEXT_ONLY,
                    OPENCODE_V2_ONLY,
                    OPENCODE_MIGRATED,
                )
            ],
            ["yes", "yes", "no", "no"],
        )

    def test_v2_bad_session_message_record(self):
        # For the 2.0 transcript: the record is carried in full and the
        # records before and after it are still read.
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.V2_DATABASE))
        rowid = connection.execute(
            "INSERT INTO session_message VALUES ('msg_nbad',?,'assistant',20,1791025210000,"
            '1791025210000,\'{"agent":"build","content":[\')',
            (OPENCODE_V2_ONLY,),
        ).lastrowid
        connection.execute(
            "INSERT INTO session_message VALUES ('msg_nafter',?,'user',21,1791025211000,"
            '1791025211000,\'{"text":"after the cut"}\')',
            (OPENCODE_V2_ONLY,),
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(OpenCodeParser(), self.V2_DATABASE)
        texts = [row.text for row in rows if row.session_id == OPENCODE_V2_ONLY]
        self.assertIn("idle: succeeded", texts)
        self.assertIn("after the cut", texts)
        raw = '{"agent":"build","content":['
        bad = [row for row in rows if row.text.startswith("parser:")]
        self.assertEqual(
            [
                (row.text, row.session_id, row.project_path, row.timestamp_utc, row.source_line)
                for row in bad
            ],
            [
                (
                    "parser: unparseable record %d (session_message.data: %s): %s"
                    % (rowid, json_reason(raw), raw),
                    OPENCODE_V2_ONLY,
                    "/srv/proj",
                    "",
                    rowid,
                )
            ],
        )

    def test_v2_bad_legacy_record_of_deleted_session(self):
        # The recovered 1.x rows go through the same reader: a bad part row
        # is its own row and the parts around it are kept.
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.V2_DATABASE))
        rowid = connection.execute(
            "INSERT INTO part VALUES ('prt_d1b','msg_d1',?,1791025801000,1791025801000,'[1,')",
            (OPENCODE_DELETED,),
        ).lastrowid
        connection.commit()
        connection.close()
        rows = self.rows_for(OpenCodeParser(), self.V2_DATABASE)
        deleted = [(row.turn_type, row.text) for row in rows if row.session_id == OPENCODE_DELETED]
        self.assertIn(("user", "what was deleted"), deleted)
        self.assertIn(("assistant", "A deleted reply."), deleted)
        self.assertIn(
            (
                "system",
                "parser: unparseable record %d (part.data: %s): [1," % (rowid, json_reason("[1,")),
            ),
            deleted,
        )

    def test_not_an_object_record(self):
        # A value that decodes but is not a JSON object is reported too.
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.DATABASE))
        rowid = connection.execute(
            "INSERT INTO message VALUES ('msg_10',?,1791018007000,1791018007000,'[1]')",
            (OPENCODE_SESSION,),
        ).lastrowid
        connection.commit()
        connection.close()
        rows = self.rows_for(OpenCodeParser(), self.DATABASE)
        self.assertEqual(
            [row.text for row in rows if row.text.startswith("parser:")],
            ["parser: unparseable record %d (message.data: not a JSON object): [1]" % rowid],
        )

    def add_session_parts(self, content: list) -> None:
        """Give the 2.x-only session an assistant message whose items are `session_part` rows.

        Two messages, each with its own parts, interleaved by `ordinal`; the second keeps an
        inline `content` that the part rows replace, as OpenCode's reader does.
        """
        connection = sqlite3.connect(str(self.home / self.V2_DATABASE))
        connection.execute(
            "CREATE TABLE session_part (message_id text NOT NULL, session_id text NOT NULL,"
            " ordinal integer NOT NULL, type text NOT NULL, call_id text,"
            " time_created integer NOT NULL, time_updated integer NOT NULL, data text NOT NULL,"
            " PRIMARY KEY (message_id, ordinal))"
        )
        messages = [("msg_p1", 30, 1791025300000, content), ("msg_p2", 31, 1791025310000, [])]
        for message_id, seq, created, inline in messages:
            data = {
                "time": {"created": created},
                "agent": "build",
                "model": {"id": "gpt-5", "providerID": "openai"},
                "content": inline,
            }
            connection.execute(
                "INSERT INTO session_message VALUES (?,?,'assistant',?,?,?,?)",
                (message_id, OPENCODE_V2_ONLY, seq, created, created, json.dumps(data)),
            )
        parts = [
            ("msg_p2", 0, "text", None, {"type": "reasoning", "text": "second one"}),
            (
                "msg_p1",
                1,
                "tool",
                "call_p1",
                {
                    "id": "call_p1",
                    "name": "bash",
                    "state": {
                        "status": "completed",
                        "input": {"command": "make"},
                        "content": [{"type": "text", "text": "built"}],
                    },
                    "time": {"created": 1791025301000, "completed": 1791025302000},
                },
            ),
            ("msg_p1", 0, "text", None, {"type": "text", "text": "from the part rows"}),
        ]
        for message_id, ordinal, part_type, call_id, data in parts:
            connection.execute(
                "INSERT INTO session_part VALUES (?,?,?,?,?,1791025300000,1791025300000,?)",
                (message_id, OPENCODE_V2_ONLY, ordinal, part_type, call_id, json.dumps(data)),
            )
        connection.commit()
        connection.close()

    def test_session_part_items_replace_inline_content(self):
        # Each message takes its own part rows, in ordinal order, never the
        # inline items or another message's parts. The row's `type` column
        # gives the tool item its type and overrides the second item's own.
        self.add_session_parts([{"type": "text", "text": "inline, superseded"}])
        rows = self.rows_for(OpenCodeParser(), self.V2_DATABASE)
        texts = [row.text for row in rows]
        self.assertNotIn("inline, superseded", texts)
        by_line: dict[int, list[tuple[str, str, str]]] = {}
        for row in rows:
            if row.session_id == OPENCODE_V2_ONLY and row.turn_type != "system":
                got = (row.turn_type, row.text, row.tool_use_id)
                by_line.setdefault(row.source_line, []).append(got)
        self.assertIn(
            [
                ("assistant", "from the part rows", ""),
                ("tool_use", "make", "call_p1"),
                ("tool_result", "built", "call_p1"),
            ],
            by_line.values(),
        )
        self.assertIn([("assistant", "second one", "")], by_line.values())

    def test_inline_content_without_part_rows(self):
        # A message no part row names keeps its inline items.
        self.add_session_parts([{"type": "text", "text": "inline, superseded"}])
        rows = self.rows_for(OpenCodeParser(), self.V2_DATABASE)
        self.assertIn("Starting it.", [row.text for row in rows])

    def test_bad_session_part_record(self):
        # The record is carried in full under its session, and the items
        # around it are still read.
        self.add_session_parts([])
        connection = sqlite3.connect(str(self.home / self.V2_DATABASE))
        rowid = connection.execute(
            "INSERT INTO session_part VALUES ('msg_p1',?,2,'text',NULL,1,1,'{\"type\":')",
            (OPENCODE_V2_ONLY,),
        ).lastrowid
        connection.commit()
        connection.close()
        rows = self.rows_for(OpenCodeParser(), self.V2_DATABASE)
        texts = [row.text for row in rows]
        self.assertIn("from the part rows", texts)
        self.assertIn("second one", texts)
        bad = [row for row in rows if row.text.startswith("parser:")]
        self.assertEqual(
            [(row.text, row.session_id, row.timestamp_utc, row.source_line) for row in bad],
            [
                (
                    'parser: unparseable record %d (session_part.data: %s): {"type":'
                    % (rowid, json_reason('{"type":')),
                    OPENCODE_V2_ONLY,
                    "",
                    rowid,
                )
            ],
        )

    def test_bad_project_file_is_reported(self):
        # A session file without a directory takes its project's worktree; a
        # project file that does not decode is a row, an absent one is silent.
        session_rel = self.STORAGE + "session/proj_a1/%s.json" % OPENCODE_LEGACY_SESSION
        path = self.home / session_rel
        session = json.loads(path.read_text(encoding="utf-8"))
        del session["directory"]
        path.write_text(json.dumps(session), encoding="utf-8")
        rows = self.rows_for(OpenCodeParser(), session_rel)
        self.assertEqual(
            [(row.turn_type, row.project_path) for row in rows], [("system", "/srv/proj")]
        )
        project = self.home / self.STORAGE / "project/proj_a1.json"
        write_bad_line(project)
        rows = self.rows_for(OpenCodeParser(), session_rel)
        self.assertEqual(len(rows), 2)
        self.assertTrue(rows[0].text.startswith("session start: Old session"))
        self.assertTrue(
            rows[1].text.startswith("parser: project file proj_a1.json could not be read ("),
            rows[1].text,
        )
        self.assertEqual((rows[1].session_id, rows[1].timestamp_utc), (OPENCODE_LEGACY_SESSION, ""))
        project.unlink()
        rows = self.rows_for(OpenCodeParser(), session_rel)
        self.assertEqual([row.project_path for row in rows], [""])

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_part_file(self):
        part = self.home / self.STORAGE / "part/msg_b/prt_b2.json"
        os.chmod(part, 0)
        try:
            rows = self.rows_for(
                OpenCodeParser(), self.STORAGE + "message/%s/msg_b.json" % OPENCODE_LEGACY_SESSION
            )
        finally:
            os.chmod(part, 0o644)
        self.assertEqual(rows[0].tool_use_id, "call_r1")
        bad = rows[-1]
        self.assertTrue(
            bad.text.startswith("parser: unparseable part prt_b2 (unreadable: "), bad.text
        )
        self.assertEqual(bad.source_file, "/alice/" + self.STORAGE + "part/msg_b/prt_b2.json")

    # -- standing state, past use, scratch directories --------------------------------
    STATE = ".local/state/opencode/"
    STATE_DATABASE = ".local/share/opencode/" + OPENCODE_STATE_DATABASE_NAME

    def with_state(self):
        build_opencode_state(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")

    def test_state_database_rows(self):
        self.with_state()
        rows = self.rows_for(OpenCodeParser(), self.STATE_DATABASE)
        database_name = OPENCODE_STATE_DATABASE_NAME
        self.assertEqual(
            [(row.session_id, row.project_path, row.timestamp_utc, row.text) for row in rows[1:6]],
            [
                (
                    OPENCODE_ARCHIVED,
                    "/srv/proj",
                    "2026-10-03T12:01:30.000Z",
                    "session archived (from %s session_v2, time_archived)" % database_name,
                ),
                (
                    OPENCODE_ARCHIVED,
                    "/srv/proj",
                    "2026-10-03T12:01:00.000Z",
                    "permission setting: session bash git *=allow "
                    "(from %s session_v2.permission, time_updated)" % database_name,
                ),
                (
                    OPENCODE_ARCHIVED,
                    "/srv/proj",
                    "2026-10-03T12:01:00.000Z",
                    "permission setting: session edit *.env=deny "
                    "(from %s session_v2.permission, time_updated)" % database_name,
                ),
                (
                    "",
                    OPENCODE_WORKTREE,
                    "2026-10-03T12:00:04.000Z",
                    "scratch directory: %s (git worktree of /srv/proj strategy=git) "
                    "(from %s worktree, time_created)" % (OPENCODE_WORKTREE, database_name),
                ),
                (
                    "",
                    "/srv/proj",
                    "2026-10-03T12:00:03.000Z",
                    "permission setting: bash npm test=allow (from %s permission, time_created)"
                    % database_name,
                ),
            ],
        )
        self.assertTrue(all(row.turn_type == "system" for row in rows[:6]))
        # A deny rule in 2.0: the result, then the decision on the call's id.
        result, decision = rows[-2:]
        self.assertEqual(result.text, "[error] permission.rejected Permission denied: edit")
        self.assertEqual(
            (decision.turn_type, decision.text, decision.tool_use_id, decision.timestamp_utc),
            (
                "system",
                "permission decision: rejected (permission.rejected)",
                "call_s1",
                result.timestamp_utc,
            ),
        )

    def test_bad_session_permission_is_reported(self):
        import sqlite3

        self.with_state()
        connection = sqlite3.connect(str(self.home / self.STATE_DATABASE))
        connection.execute(
            "UPDATE session_v2 SET permission = '{\"x\":' WHERE id = ?", (OPENCODE_ARCHIVED,)
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(OpenCodeParser(), self.STATE_DATABASE)
        bad = [row for row in rows if row.text.startswith("parser:")]
        self.assertEqual(
            [(row.text, row.session_id) for row in bad],
            [
                (
                    'parser: unparseable record 1 (session_v2.permission: not a JSON array): {"x":',
                    OPENCODE_ARCHIVED,
                )
            ],
        )
        self.assertIn(
            "permission decision: rejected (permission.rejected)", [row.text for row in rows]
        )

    def test_v1_permission_decisions(self):
        # 1.x refusal texts in a tool part's `state.error` (packages/core/src/
        # v1/permission.ts:7-25 at 907b3bc), each followed by its decision.
        import sqlite3

        texts = {
            "call_r1": "The user rejected permission to use this specific tool call.",
            "call_r2": "The user rejected permission to use this specific tool call with the "
            "following feedback: use the staging host",
            "call_r3": "The user has specified a rule which prevents you from using this specific "
            'tool call. Here are some of the relevant rules [{"permission":"bash"}]',
            "call_r4": "Tool execution aborted",
        }
        connection = sqlite3.connect(str(self.home / self.DATABASE))
        for index, (call, error) in enumerate(texts.items()):
            data = {
                "type": "tool",
                "callID": call,
                "tool": "bash",
                "state": {
                    "status": "error",
                    "input": {"command": "rm -rf build"},
                    "error": error,
                    "time": {"start": 1791018004000 + index, "end": 1791018004500 + index},
                },
            }
            connection.execute(
                "INSERT INTO part VALUES (?, 'msg_02', ?, 1791018004000, 1791018004000, ?)",
                ("prt_07%d" % index, OPENCODE_SESSION, json.dumps(data)),
            )
        connection.commit()
        connection.close()
        rows = self.rows_for(OpenCodeParser(), self.DATABASE)
        got = [
            (row.tool_use_id, row.text)
            for row in rows
            if row.text.startswith("permission decision:") and row.tool_use_id
        ]
        self.assertEqual(
            got,
            [
                ("call_r1", "permission decision: rejected"),
                ("call_r2", "permission decision: rejected with feedback"),
                ("call_r3", "permission decision: denied by rule"),
            ],
        )
        for call, _ in got:
            result = next(
                row for row in rows if row.tool_use_id == call and row.turn_type == "tool_result"
            )
            dec = next(row for row in rows if row.tool_use_id == call and row.turn_type == "system")
            self.assertEqual(rows.index(dec), rows.index(result) + 1)
            self.assertEqual(dec.timestamp_utc, result.timestamp_utc)

    def test_prompt_history_is_secondary(self):
        self.with_state()
        parser = OpenCodeParser()
        rel = self.STATE + "prompt-history.jsonl"
        self.assertTrue(parser.secondary(self.artifact(rel)))
        rows = self.rows_for(parser, rel)
        self.assertEqual(
            [
                (
                    row.turn_type,
                    row.tool_name,
                    row.text,
                    row.timestamp_utc,
                    row.session_id,
                    row.source_line,
                )
                for row in rows
            ],
            [
                ("user", "", "list files", "", "", 1),
                ("tool_use", "shell", "git status", "", "", 2),
                ("user", "", "only the history kept this", "", "", 3),
            ],
        )

    def test_prompt_history_truncated_line(self):
        self.with_state()
        path = self.home / self.STATE / "prompt-history.jsonl"
        write_bad_line(path)
        with open(path, "a", encoding="utf-8") as handle:
            handle.write('\n{"text": "after the cut", "pasted": []}\n')
        rows = self.rows_for(OpenCodeParser(), self.STATE + "prompt-history.jsonl")
        self.assertEqual(
            [row.text for row in rows],
            [
                "list files",
                "git status",
                "only the history kept this",
                "after the cut",
                bad_line_text(),
            ],
        )
        self.assertEqual((rows[-1].turn_type, rows[-1].source_line), ("system", 4))

    def test_prompt_stash(self):
        self.with_state()
        parser = OpenCodeParser()
        rel = self.STATE + "prompt-stash.jsonl"
        self.assertFalse(parser.secondary(self.artifact(rel)))
        rows = self.rows_for(parser, rel)
        self.assertEqual(
            [(row.turn_type, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    "2026-10-03T12:00:05.000Z",
                    "prompt stashed (unsent): draft: rotate the keys (from %s, timestamp)" % rel,
                ),
                (
                    "system",
                    "2026-10-03T12:00:06.000Z",
                    "prompt stashed (unsent): older stash (from %s, timestamp)" % rel,
                ),
            ],
        )
        path = self.home / rel
        write_bad_line(path)
        with open(path, "a", encoding="utf-8") as handle:
            handle.write('\n{"input": "after the cut", "timestamp": 1791028810000}\n')
        rows = self.rows_for(parser, rel)
        self.assertEqual(len(rows), 4)
        self.assertIn("after the cut", rows[2].text)
        self.assertEqual(rows[3].text, bad_line_text())

    def test_pinned_sessions(self):
        self.with_state()
        parser = OpenCodeParser()
        rel = self.STATE + "session.json"
        self.assertTrue(parser.secondary(self.artifact(rel)))
        rows = self.rows_for(parser, rel)
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    session_id,
                    to_utc(OPENCODE_STATE_MTIME),
                    "session pinned: %s (from %s pinned, file mtime)" % (session_id, rel),
                )
                for session_id in (OPENCODE_SESSION, OPENCODE_PINNED_GONE)
            ],
        )
        (self.home / rel).write_text('{"pinned": [', encoding="utf-8")
        rows = self.rows_for(parser, rel)
        self.assertEqual([row.text for row in rows], ["parser: %s is not a JSON object" % rel])

    def test_state_noise_not_wanted(self):
        self.with_state()
        parser = OpenCodeParser()
        for name in ("kv.json", "service.json"):
            self.assertFalse(parser.wants(self.artifact(self.STATE + name)), name)
        # Kilo Code's TUI keeps OpenCode's pinned-session file under its own state directory.
        self.assertTrue(
            KiloCodeParser().pinned_pattern.match(".local/state/kilo/session.json"), "kilo pinned"
        )

    def test_history_pairs_with_the_transcript(self):
        import contextlib

        self.with_state()
        out = self.tmp / "out"
        with contextlib.redirect_stdout(io.StringIO()):
            cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), "--agent", "opencode"])
        timeline = read_jsonl(out / "timeline.jsonl")
        history = [row for row in timeline if row["source_file"].endswith("prompt-history.jsonl")]
        # "list files" pairs with the 1.x transcript's prompt and the shell
        # line "git status" with the 2.0 `shell` record's call, so both are
        # dropped; the prompt no transcript holds is kept, as recovered.
        self.assertEqual(
            [(row["text"], row["recovered"]) for row in history],
            [("only the history kept this", "yes")],
        )
        sessions = {row["session_id"] for row in read_jsonl(out / "sessions.jsonl")}
        self.assertIn(OPENCODE_PINNED_GONE, sessions)


class OpenCodeSyncTests(ParserBase):
    """Test the share records of an OpenCode database."""

    DATABASE = ".local/share/opencode/" + OPENCODE_SHARE_DATABASE_NAME

    @staticmethod
    def sync_rows(rows):
        return [
            (row.session_id, row.text, row.timestamp_utc, row.source_line, row.project_path)
            for row in rows
            if row.text.startswith(("session synced:", "session unshared"))
        ]

    def test_rows(self):
        self.assertTrue(OpenCodeParser.records_sync)
        rows = self.rows_for(OpenCodeParser(), self.DATABASE)
        source = (
            "(from %s event session.updated.1, info.time.updated)" % OPENCODE_SHARE_DATABASE_NAME
        )
        self.assertEqual(
            self.sync_rows(rows),
            [
                (
                    OPENCODE_SHARED,
                    "session synced: share SHARE05 url https://opncd.ai/share/SHARE05 "
                    "(from %s session_share, time_created)" % OPENCODE_SHARE_DATABASE_NAME,
                    "2026-10-03T10:00:02.000Z",
                    1,
                    "/srv/proj",
                ),
                (
                    OPENCODE_UNSHARED,
                    "session synced: share url https://opncd.ai/share/UNSHAR06 " + source,
                    "2026-10-03T10:00:12.000Z",
                    3,
                    "/srv/proj",
                ),
                (
                    OPENCODE_UNSHARED,
                    "session unshared " + source,
                    "2026-10-03T10:00:14.000Z",
                    5,
                    "/srv/proj",
                ),
            ],
        )
        # The sync rows follow the session-start rows and are `system`.
        self.assertEqual([row.turn_type for row in rows], ["system"] * 6)
        self.assertTrue(all(row.text.startswith("session start:") for row in rows[:3]))

    def test_session_without_share_has_no_sync_row(self):
        rows = self.rows_for(OpenCodeParser(), self.DATABASE)
        self.assertEqual(
            [row.text[:14] for row in rows if row.session_id == OPENCODE_PRIVATE],
            ["session start:"],
        )
        # The database the other OpenCode tests read has no share tables.
        rows = self.rows_for(OpenCodeParser(), ".local/share/opencode/opencode.db")
        self.assertEqual(self.sync_rows(rows), [])

    def test_secret_never_printed(self):
        rows = self.rows_for(OpenCodeParser(), self.DATABASE)
        self.assertEqual([row for row in rows if OPENCODE_SHARE_SECRET in row.text], [])

    def test_bad_event_data_is_reported_not_fatal(self):
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.DATABASE))
        connection.execute(
            "INSERT INTO event VALUES ('evt_bad', ?, 9, 'session.updated.1', '{\"info\":')",
            (OPENCODE_PRIVATE,),
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(OpenCodeParser(), self.DATABASE)
        self.assertEqual(len(self.sync_rows(rows)), 3)
        # The event is carried in full, under its session.
        last = rows[-1]
        self.assertEqual(
            last.text,
            'parser: unparseable record 8 (event.data: %s): {"info":' % json_reason('{"info":'),
        )
        self.assertEqual(
            (last.session_id, last.project_path, last.timestamp_utc, last.source_line),
            (OPENCODE_PRIVATE, "/srv/proj", "", 8),
        )

    def test_unreadable_database_is_one_row(self):
        path = self.home / self.DATABASE
        path.write_bytes(b"SQLite format 3\x00" + b"\x00" * 200)
        rows = self.rows_for(OpenCodeParser(), self.DATABASE)
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("parser: database could not be read"), rows[0])

    def test_sessions_synced(self):
        import contextlib

        out = self.tmp / "out"
        with contextlib.redirect_stdout(io.StringIO()):
            cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), "--agent", "opencode"])
        synced = {row["session_id"]: row["synced"] for row in read_jsonl(out / "sessions.jsonl")}
        self.assertEqual(synced[OPENCODE_SHARED], "yes")
        self.assertEqual(synced[OPENCODE_UNSHARED], "yes")
        self.assertEqual(synced[OPENCODE_PRIVATE], "no")
        self.assertEqual(synced[OPENCODE_SESSION], "no")
