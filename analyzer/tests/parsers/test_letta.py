"""Tests of the Letta parser."""

import io
import json
import os

from doubleagent import cli
from doubleagent.dedup import deduplicate
from doubleagent.inputs import Artifact, Home
from doubleagent.parsers import Options
from doubleagent.parsers.letta import LettaParser, decode_key
from doubleagent.timeutil import to_utc
from fixtures import (
    LETTA_AGENT,
    LETTA_CONV,
    LETTA_LEGACY_SETTINGS_MTIME,
    LETTA_LISTENER_FILE,
    LETTA_LISTENER_HASH,
    LETTA_LISTENER_MTIME,
    LETTA_LOCAL_AGENT,
    LETTA_LOCAL_AGENT_CONV,
    LETTA_LOCAL_CONV,
    LETTA_LOCAL_DIR,
    LETTA_LOCAL_SETTINGS_MTIME,
    LETTA_SETTINGS_MTIME,
    LETTA_TASK,
    _jsonl,
    bad_line_text,
    letta_project_settings,
    letta_sessions_records,
    write_bad_line,
)

from . import test_codex
from .base import ParserBase, read_jsonl


class LettaTests(ParserBase):
    """Test the Letta Code parser."""

    TRANSCRIPT = ".letta/transcripts/%s/%s/transcript.jsonl" % (LETTA_AGENT, LETTA_CONV)
    TWIN = ".letta/transcripts/%s/%s/transcript.jsonl" % (LETTA_AGENT, LETTA_LOCAL_CONV)
    LOCAL = ".letta/lc-local-backend/conversations/%s/messages.jsonl" % LETTA_LOCAL_DIR

    def test_local_backend_rows(self):
        rows = self.rows_for(LettaParser(), self.LOCAL)
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "user", "system", "tool_use", "tool_result", "system", "assistant"],
        )
        for row in rows:
            self.assertEqual(
                (row.agent, row.session_id, row.project_path),
                ("letta", LETTA_LOCAL_CONV, "/srv/proj"),
            )
        _, user, start_model, use, result, change, _ = rows
        self.assertEqual(
            (start_model.text, start_model.model, start_model.timestamp_utc),
            (
                "model: claude-sonnet-4-5 provider=anthropic",
                "claude-sonnet-4-5",
                "2026-10-01T12:00:02.000Z",
            ),
        )
        self.assertEqual(
            (change.text, change.model, change.timestamp_utc, change.source_line),
            (
                "model: claude-sonnet-4-5 -> claude-haiku-4-5 provider=anthropic",
                "claude-haiku-4-5",
                "2026-10-01T12:00:04.000Z",
                6,
            ),
        )
        self.assertEqual(
            (user.text, user.timestamp_utc), ("list the repo", "2026-10-01T12:00:01.000Z")
        )
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.model, use.text),
            ("Bash", "call_1", "claude-sonnet-4-5", "ls"),
        )
        self.assertEqual(
            use.source_line, 4
        )  # the replacement snapshot, not the pending copy on line 3
        self.assertEqual((result.tool_use_id, result.text), ("call_1", "README.md"))
        self.assertEqual(decode_key(LETTA_LOCAL_DIR), "conversation:" + LETTA_LOCAL_CONV)

    def test_thinking_opt_in(self):
        rows = self.rows_for(LettaParser(), self.LOCAL, include_thinking=True)
        self.assertEqual([row.text for row in rows if row.turn_type == "thinking"], ["use ls"])
        rows = self.rows_for(LettaParser(), self.TRANSCRIPT)
        self.assertNotIn("thinking", [row.turn_type for row in rows])
        rows = self.rows_for(LettaParser(), self.TRANSCRIPT, include_thinking=True)
        self.assertEqual([row.text for row in rows if row.turn_type == "thinking"], ["use ls"])

    def test_reflection_transcript(self):
        rows = self.rows_for(LettaParser(), self.TRANSCRIPT)
        # The first row is the server-conversation sync row (LettaSyncTests).
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "user", "tool_use", "tool_result", "assistant"],
        )
        rows = rows[1:]
        for row in rows:
            self.assertEqual(
                (row.session_id, row.project_path, row.model, row.timestamp_utc),
                (LETTA_CONV, "/srv/proj", "", "2026-10-01T12:00:05.120Z"),
            )
        # argsText rendered as the messages.jsonl row renders the call.
        self.assertEqual((rows[1].tool_name, rows[1].text), ("Bash", "ls"))
        self.assertEqual(
            (rows[2].tool_name, rows[2].tool_use_id, rows[2].text), ("Bash", "", "README.md")
        )

    def test_secondary(self):
        parser = LettaParser()
        for rel in (self.TRANSCRIPT, self.TWIN):
            self.assertTrue(parser.secondary(self.artifact(rel)), rel)
            self.assertEqual(parser.secondary_tolerance(self.artifact(rel)), 3600.0)
        for rel in (self.LOCAL, ".letta/sessions.jsonl"):
            self.assertFalse(parser.secondary(self.artifact(rel)), rel)

    def test_transcript_read_beside_local_backend(self):
        # Read always, not only when messages.jsonl is missing.
        rows = self.rows_for(LettaParser(), self.TWIN)
        # The fixture's agent id has no `agent-local-` prefix, so the twin
        # also gets the server-conversation sync row (LettaSyncTests).
        self.assertTrue(rows[0].text.startswith("session synced: server conversation"))
        rows = rows[1:]
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc) for row in rows],
            [
                ("user", "list the repo", "2026-10-01T12:00:05.120Z"),
                ("tool_use", "ls", "2026-10-01T12:00:05.120Z"),
                ("tool_result", "README.md", "2026-10-01T12:00:05.120Z"),
                ("assistant", "One file: README.md", "2026-10-01T12:00:05.120Z"),
                ("user", "now delete the build directory", "2026-10-01T12:00:40.500Z"),
                ("assistant", "Deleted build/", "2026-10-01T12:00:40.500Z"),
            ],
        )
        local = self.rows_for(LettaParser(), self.LOCAL)
        # The same conversation id and project path as the messages.jsonl
        # rows, so the scope clause of the rule can pair them.
        self.assertEqual(
            {(row.session_id, row.project_path) for row in rows},
            {(row.session_id, row.project_path) for row in local},
        )
        self.assertEqual({row.session_id for row in rows}, {LETTA_LOCAL_CONV})

    def test_transcript_pairs_with_local_backend(self):
        parser = LettaParser()
        primary = self.rows_for(parser, self.LOCAL)
        twin = self.rows_for(parser, self.TWIN)
        tolerance = parser.secondary_tolerance(self.artifact(self.TWIN))
        kept, dropped = deduplicate(primary, [(row, tolerance) for row in twin])
        # Every line of the first turn pairs, the user prompt 4.12 s after
        # its messages.jsonl twin; the turn messages.jsonl lacks survives.
        self.assertEqual(dropped["letta"], 4)
        # The sync row has no twin and is kept, before the turn.
        texts = [row.text for row in kept[len(primary) :]]
        self.assertTrue(texts[0].startswith("session synced: server conversation"), texts)
        self.assertEqual(texts[1:], ["now delete the build directory", "Deleted build/"])
        # The default window pairs the assistant line only.
        _, dropped = deduplicate(primary, [(row, 2.0) for row in twin])
        self.assertEqual(dropped["letta"], 1)

    def test_sessions_history(self):
        # One exit line per CLI session, each followed by the session's
        # model row.
        rows = self.rows_for(LettaParser(), ".letta/sessions.jsonl")
        self.assertEqual(
            [(row.turn_type, row.session_id, row.project_path) for row in rows],
            [
                ("system", "s-0", "/srv/old"),
                ("system", "s-0", "/srv/old"),
                ("system", "s-1", "/srv/proj"),
                ("system", "s-1", "/srv/proj"),
            ],
        )
        self.assertEqual(
            rows[2].text,
            "letta session end: agent=%s exit_reason=exit_command messages=4 tool_calls=1 "
            "wall_ms=60000" % LETTA_AGENT,
        )
        self.assertIn("exit_reason=logout", rows[0].text)
        self.assertEqual(rows[2].model, "claude-sonnet-4-5")
        self.assertEqual(
            (rows[3].text, rows[3].model, rows[3].timestamp_utc, rows[3].source_line),
            (
                "model: claude-sonnet-4-5 provider=anthropic",
                "claude-sonnet-4-5",
                "2026-10-01T12:01:00.000Z",
                2,
            ),
        )

    def test_sessions_start_line(self):
        # No caller writes recordSessionStart's line; one is still read.
        record = dict(letta_sessions_records()[1], timestamp=1790856000000)
        for key in ("message_count", "tool_call_count", "exit_reason"):
            del record[key]
        _jsonl(self.home / ".letta/sessions.jsonl", [record])
        rows = self.rows_for(LettaParser(), ".letta/sessions.jsonl")
        self.assertEqual(
            rows[0].text,
            "letta session start: agent=%s provider=anthropic model=claude-sonnet-4-5 "
            "project=/srv/proj" % LETTA_AGENT,
        )

    def test_transcript_project_from_exit_line(self):
        # The exit line after the turn names its project; with no later
        # line, the latest earlier one does.
        path = self.home / ".letta/sessions.jsonl"
        older, end = letta_sessions_records()
        _jsonl(path, [older, dict(end, project="/srv/other")])
        rows = self.rows_for(LettaParser(), self.TRANSCRIPT)
        self.assertEqual({row.project_path for row in rows}, {"/srv/other"})
        _jsonl(path, [older])
        rows = self.rows_for(LettaParser(), self.TRANSCRIPT)
        self.assertEqual({row.project_path for row in rows}, {"/srv/old"})

    def test_permission_settings(self):
        rel = ".config/letta/settings.json"
        rows = self.rows_for(LettaParser(), rel)
        source = "(from .config/letta/settings.json, file mtime)"
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                ("system", "", to_utc(LETTA_LEGACY_SETTINGS_MTIME), text)
                for text in (
                    "permission setting: permissions.mode=acceptEdits " + source,
                    "permission setting: permissions.allow=Bash(npm test) " + source,
                    "permission setting: permissions.deny=Bash(rm -rf:*) " + source,
                    "permission setting: permissions.additionalDirectories=/srv/shared " + source,
                )
            ],
        )

    def test_routing_relay_rows(self):
        self.assertTrue(LettaParser.records_relay)
        rel = ".letta/channels/telegram/routing.json"
        rows = self.rows_for(LettaParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    LETTA_CONV,
                    "2026-10-01T12:20:00.000Z",
                    "session relayed: telegram chat 123456789 type direct thread - account "
                    "bot-1 agent %s enabled=true outbound=true detached=- "
                    "updated=2026-10-01T12:21:00.000Z (from %s, createdAt)" % (LETTA_AGENT, rel),
                )
            ],
        )
        (self.home / rel).write_text("[]", encoding="utf-8")
        self.assertEqual(
            [row.text for row in self.rows_for(LettaParser(), rel)],
            ["parser: %s is not a JSON object with a routes list" % rel],
        )

    def test_scheduled_task_rows(self):
        rows = self.rows_for(LettaParser(), ".letta/crons.json")
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    LETTA_CONV,
                    "2026-10-01T12:10:00.000Z",
                    "scheduled task: %s name=nightly tests status=active cron=0 2 * * * "
                    "timezone=UTC recurring=true scheduled_for=- agent=%s conversation=%s "
                    "fire_count=1 last_fired=2026-10-02T02:00:00.000Z "
                    "last_run=2026-10-02T02:00:00.000Z outcome=queued "
                    "reason=scheduled_time_matched error=- cancel_reason=- "
                    "(from .letta/crons.json, created_at) description: run the suite "
                    "prompt: run the test suite and report failures"
                    % (LETTA_TASK, LETTA_AGENT, LETTA_CONV),
                )
            ],
        )
        rel = ".letta/runs/%s.jsonl" % LETTA_TASK
        rows = self.rows_for(LettaParser(), rel)
        self.assertEqual(
            [(row.session_id, row.timestamp_utc, row.source_line) for row in rows],
            [
                (LETTA_CONV, "2026-10-02T02:00:00.000Z", 1),
                (LETTA_CONV, "2026-10-03T02:00:00.000Z", 2),
            ],
        )
        self.assertEqual(
            rows[1].text,
            "scheduled task run: %s action=finished status=skipped outcome=missed "
            "reason=scheduler_inactive agent=%s conversation=%s run=- scheduled_for=- "
            "fired=- missed=1 summary=missed error=- (from %s, ts)"
            % (LETTA_TASK, LETTA_AGENT, LETTA_CONV, rel),
        )
        self.assertFalse(LettaParser().secondary(self.artifact(rel)))
        self.assertFalse(LettaParser().secondary(self.artifact(".letta/crons.json")))

    def test_interrupted_turn_rows(self):
        rel = ".letta/listener-state/%s/%s" % (LETTA_LISTENER_HASH, LETTA_LISTENER_FILE)
        rows = self.rows_for(LettaParser(), rel)
        timestamp = to_utc(LETTA_LISTENER_MTIME)
        self.assertEqual(
            [(row.turn_type, row.tool_use_id, row.text) for row in rows],
            [
                (
                    "system",
                    "",
                    "interrupted turn: agent %s conversation %s run run-2 "
                    "tool_calls=call_7,call_8 unstarted=call_8 results=2 "
                    "working_directory=/srv/proj acting_user=user-1 (from %s, file mtime)"
                    % (LETTA_AGENT, LETTA_CONV, rel),
                ),
                ("tool_result", "call_7", "[error] permission denied"),
                (
                    "system",
                    "call_8",
                    "permission decision: deny reason=not on main "
                    "(from %s, interrupted turn, file mtime)" % rel,
                ),
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.session_id, row.project_path, row.timestamp_utc),
                (LETTA_CONV, "/srv/proj", timestamp),
            )
        self.assertFalse(LettaParser().secondary(self.artifact(rel)))

    def test_schema_1_bare_messages(self):
        path = self.home / self.LOCAL
        _jsonl(
            path,
            [
                {
                    "id": "letta-msg-1",
                    "role": "user",
                    "content": "hi",
                    "timestamp": 1790856001000,
                    "metadata": {"created_at": "2026-10-01T12:00:01.000Z", "agent_id": LETTA_AGENT},
                },
                {
                    "id": "letta-msg-2",
                    "role": "assistant",
                    "content": [{"type": "text", "text": "hello"}],
                    "model": "claude-sonnet-4-5",
                    "timestamp": 1790856002000,
                },
            ],
        )
        rows = self.rows_for(LettaParser(), self.LOCAL)
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc, row.session_id) for row in rows],
            [
                ("user", "hi", "2026-10-01T12:00:01.000Z", LETTA_LOCAL_CONV),
                (
                    "system",
                    "model: claude-sonnet-4-5",
                    "2026-10-01T12:00:02.000Z",
                    LETTA_LOCAL_CONV,
                ),
                ("assistant", "hello", "2026-10-01T12:00:02.000Z", LETTA_LOCAL_CONV),
            ],
        )

    def test_config_not_wanted(self):
        for rel in (
            ".letta/transcripts/%s/%s/state.json" % (LETTA_AGENT, LETTA_CONV),
            ".letta/lc-local-backend/conversations/%s/manifest.json" % LETTA_LOCAL_DIR,
            ".letta/transcripts/%s/%s/payload-auto-a1b2c3.json" % (LETTA_AGENT, LETTA_LOCAL_CONV),
            ".letta/channels/telegram/accounts.json",
            ".letta/listener-state/%s/%s.recovery-%s"
            % (LETTA_LISTENER_HASH, LETTA_LISTENER_FILE, "ab" * 12),
        ):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertFalse(LettaParser().wants(art), rel)

    def test_truncated_line_is_reported_not_fatal(self):
        runs = ".letta/runs/%s.jsonl" % LETTA_TASK
        for rel, line in (
            (self.TRANSCRIPT, 5),
            (self.TWIN, 7),
            (self.LOCAL, 7),
            (".letta/sessions.jsonl", 4),
            (runs, 2),
        ):
            write_bad_line(self.home / rel)
            rows = self.rows_for(LettaParser(), rel)
            self.assertEqual(len(rows), line + 1, rel)
            self.assertEqual((rows[-1].turn_type, rows[-1].text), ("system", bad_line_text()), rel)

    def test_bad_line_mid_file_keeps_the_rows_around_it(self):
        runs = ".letta/runs/%s.jsonl" % LETTA_TASK
        for rel in (self.TRANSCRIPT, self.TWIN, self.LOCAL, ".letta/sessions.jsonl", runs):
            with self.subTest(rel=rel):
                test_codex.CodexTests.check_bad_line(self, LettaParser, rel, 1)
        row = test_codex.CodexTests.check_bad_line(self, LettaParser, self.LOCAL, 3)
        self.assertEqual((row.session_id, row.project_path), (LETTA_LOCAL_CONV, "/srv/proj"))


class LettaSyncTests(ParserBase):
    """Test the server-side session markers of Letta Code."""

    TRANSCRIPT = ".letta/transcripts/%s/%s/transcript.jsonl" % (LETTA_AGENT, LETTA_CONV)
    LOCAL_TRANSCRIPT = ".letta/transcripts/%s/%s/transcript.jsonl" % (
        LETTA_LOCAL_AGENT,
        LETTA_LOCAL_AGENT_CONV,
    )
    SETTINGS = ".letta/settings.json"

    def test_transcript_row(self):
        self.assertTrue(LettaParser.records_sync)
        rows = self.rows_for(LettaParser(), self.TRANSCRIPT)
        first = rows[0]
        self.assertEqual(
            (first.turn_type, first.text, first.timestamp_utc, first.source_line),
            (
                "system",
                "session synced: server conversation %s agent %s server api.letta.com "
                "(from .letta/transcripts path, first captured_at)" % (LETTA_CONV, LETTA_AGENT),
                "2026-10-01T12:00:05.120Z",
                1,
            ),
        )
        self.assertEqual((first.session_id, first.project_path), (LETTA_CONV, "/srv/proj"))
        self.assertEqual([row for row in rows[1:] if row.text.startswith("session synced:")], [])

    def test_transcript_server_from_settings(self):
        path = self.home / self.SETTINGS
        doc = json.loads(path.read_text(encoding="utf-8"))
        doc["agents"][0]["baseUrl"] = "http://localhost:8283"
        path.write_text(json.dumps(doc), encoding="utf-8")
        self.assertIn(
            " server http://localhost:8283 (from",
            self.rows_for(LettaParser(), self.TRANSCRIPT)[0].text,
        )
        doc["agents"] = []
        path.write_text(json.dumps(doc), encoding="utf-8")
        self.assertIn(" server - (from", self.rows_for(LettaParser(), self.TRANSCRIPT)[0].text)
        path.unlink()
        self.assertIn(" server - (from", self.rows_for(LettaParser(), self.TRANSCRIPT)[0].text)

    def test_local_agent_transcript_has_no_sync_row(self):
        rows = self.rows_for(LettaParser(), self.LOCAL_TRANSCRIPT)
        self.assertEqual([row.turn_type for row in rows], ["user", "assistant"])

    def test_settings_rows(self):
        rows = self.rows_for(LettaParser(), self.SETTINGS)
        self.assertEqual(
            [
                (row.turn_type, row.session_id, row.project_path, row.timestamp_utc, row.text)
                for row in rows
            ],
            [
                (
                    "system",
                    LETTA_CONV,
                    "",
                    to_utc(LETTA_SETTINGS_MTIME),
                    "session synced: last session on api.letta.com agent %s conversation %s "
                    "(from .letta/settings.json, file mtime)" % (LETTA_AGENT, LETTA_CONV),
                )
            ],
        )
        # The credential beside it is never printed.
        self.assertEqual([row for row in rows if "sk-not-real" in row.text], [])

    def test_project_settings_rows(self):
        project = self.tmp / "srv" / "proj"
        rel = ".letta/settings.local.json"
        path = project / rel
        path.parent.mkdir(parents=True)
        path.write_text(json.dumps(letta_project_settings()), encoding="utf-8")
        os.utime(path, (LETTA_LOCAL_SETTINGS_MTIME, LETTA_LOCAL_SETTINGS_MTIME))
        home = Home(disk_path=project, original="/srv/proj", user="alice", host="h1", project=True)
        art = Artifact(path, "/srv/proj/" + rel, rel, "letta", home)
        self.assertTrue(LettaParser().wants(art))
        rows = list(LettaParser().parse(art, Options()))
        self.assertEqual(
            [
                (row.session_id, row.project_path, row.timestamp_utc, row.source_file, row.text)
                for row in rows
            ],
            [
                (
                    "conv-sh1",
                    "/srv/proj",
                    to_utc(LETTA_LOCAL_SETTINGS_MTIME),
                    "/srv/proj/" + rel,
                    "session synced: last session on localhost:8283 agent agent-selfhost "
                    "conversation conv-sh1 (from .letta/settings.local.json, file mtime)",
                ),
                # The project's standing rule.
                (
                    "",
                    "/srv/proj",
                    to_utc(LETTA_LOCAL_SETTINGS_MTIME),
                    "/srv/proj/" + rel,
                    "permission setting: permissions.allow=Bash(ls) "
                    "(from .letta/settings.local.json, file mtime)",
                ),
            ],
        )

    def test_settings_not_a_json_object(self):
        for raw in ("[]", '{"sessionsByServer": '):
            (self.home / self.SETTINGS).write_text(raw, encoding="utf-8")
            rows = self.rows_for(LettaParser(), self.SETTINGS)
            self.assertEqual(
                [(row.turn_type, row.text) for row in rows],
                [("system", "parser: .letta/settings.json is not a JSON object")],
            )

    def test_truncated_transcript_keeps_the_sync_row(self):
        write_bad_line(self.home / self.TRANSCRIPT)
        rows = self.rows_for(LettaParser(), self.TRANSCRIPT)
        self.assertTrue(rows[0].text.startswith("session synced: server conversation"))
        self.assertEqual((rows[-1].turn_type, rows[-1].text), ("system", bad_line_text()))

    def test_sessions_synced(self):
        import contextlib

        out = self.tmp / "out"
        with contextlib.redirect_stdout(io.StringIO()):
            cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), "--agent", "letta"])
        synced = {row["session_id"]: row["synced"] for row in read_jsonl(out / "sessions.jsonl")}
        self.assertEqual(synced[LETTA_CONV], "yes")
        self.assertEqual(synced[LETTA_LOCAL_AGENT_CONV], "no")
