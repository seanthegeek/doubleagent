"""Tests of the OpenClaw parser."""

import io
import json
import sqlite3
from pathlib import Path

from doubleagent import cli
from doubleagent.parsers import Options
from doubleagent.parsers.openclaw import OpenClawParser
from fixtures import (
    BAD_LINE,
    OPENCLAW_AGENT,
    OPENCLAW_BACKUP_COMMIT,
    OPENCLAW_BACKUP_KEY,
    OPENCLAW_CHECKPOINT,
    OPENCLAW_CHUNKED_SESSION,
    OPENCLAW_COLD_SESSION,
    OPENCLAW_DELETED,
    OPENCLAW_FTS_ONLY,
    OPENCLAW_KEY,
    OPENCLAW_LEGACY,
    OPENCLAW_REPAIR_BAK,
    OPENCLAW_RESET_SESSION,
    OPENCLAW_SESSION,
    OPENCLAW_SHARE_ID,
    OPENCLAW_SOCKET_TOKEN,
    OPENCLAW_STATE_DATABASE,
    OPENCLAW_TOKEN,
    PROJECT_REMOTE,
    _jsonl,
    bad_line_text,
    openclaw_legacy_records,
    write_bad_line,
)

from .base import ParserBase, rel_only


class OpenClawTests(ParserBase):
    """Test the OpenClaw parser."""

    DATABASE = OPENCLAW_AGENT + "/agent/openclaw-agent.sqlite"
    LEGACY = OPENCLAW_AGENT + "/sessions/%s.jsonl" % OPENCLAW_LEGACY

    def transcript(self, session_id):
        """Return the database rows of generation `sid` that come from its transcript.

        This leaves out the rows of the session metadata and side tables (which name
        their table in the text), the pending prompts and the rows recovered from the
        full-text index.
        """
        return [
            row
            for row in self.rows_for(OpenClawParser(), self.DATABASE)
            if row.session_id == session_id
            and "openclaw-agent.sqlite" not in row.text
            and not row.recovered
            and row.text != "also restart nginx"
        ]

    def test_rows(self):
        # The rows of the session metadata and side tables are checked in
        # the tests further down.
        rows = self.transcript(OPENCLAW_SESSION)
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "system", "user", "assistant", "tool_use", "tool_result", "system"],
        )
        for row in rows:
            self.assertEqual((row.host, row.user, row.agent), ("h1", "alice", "openclaw"))
            self.assertEqual((row.project_path, row.git_branch), ("/srv/proj", ""))
            self.assertEqual(row.source_file, "/alice/" + self.DATABASE)
        start, start_model, user, asst, use, result, change = rows
        # The session window names the starting model; the assistant reply
        # names the same one and adds no row.
        self.assertEqual(
            (start_model.text, start_model.model, start_model.timestamp_utc),
            (
                "model: claude-sonnet-4-5 provider=anthropic",
                "claude-sonnet-4-5",
                "2026-10-01T09:00:00.000Z",
            ),
        )
        self.assertIn("key=" + OPENCLAW_KEY, start.text)
        self.assertIn("channel=telegram", start.text)
        self.assertIn("chat_type=direct", start.text)
        self.assertEqual(start.timestamp_utc, "2026-10-01T09:00:00.000Z")
        self.assertEqual(
            (user.text, user.timestamp_utc),
            ("check disk space on the server", "2026-10-01T09:00:01.000Z"),
        )
        self.assertEqual((asst.text, asst.model), ("Checking.", "claude-sonnet-4-5"))
        self.assertEqual((use.tool_name, use.tool_use_id, use.text), ("exec", "call_01", "df -h"))
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.text),
            ("exec", "call_01", "/dev/sda1  50G  20G  30G  40% /"),
        )
        self.assertEqual(
            (change.text, change.model, change.timestamp_utc),
            (
                "model: claude-sonnet-4-5 -> gpt-5 provider=openai",
                "gpt-5",
                "2026-10-01T09:01:00.000Z",
            ),
        )
        self.assertEqual(
            [row.source_line for row in rows], [1, 1, 2, 3, 3, 4, 5]
        )  # transcript_events rowid

    def test_repeated_model_change_keeps_its_row(self):
        path = self.home / self.LEGACY
        recs = openclaw_legacy_records()
        recs.append(
            {
                "type": "model_change",
                "id": "e5",
                "parentId": "e4",
                "timestamp": "2026-09-20T08:06:00.000Z",
                "provider": "openai",
                "modelId": "gpt-5",
            }
        )
        _jsonl(path, recs)
        rows = [
            row
            for row in self.rows_for(OpenClawParser(), self.LEGACY)
            if row.text.startswith("model:")
        ]
        self.assertEqual(
            [(row.text, row.model, row.timestamp_utc) for row in rows],
            [
                ("model: gpt-5", "gpt-5", "2026-09-20T08:05:10.000Z"),
                ("model: gpt-5 provider=openai", "gpt-5", "2026-09-20T08:06:00.000Z"),
            ],
        )

    def test_thinking_opt_in(self):
        rows = self.rows_for(OpenClawParser(), self.DATABASE, include_thinking=True)
        self.assertEqual(
            [row.text for row in rows if row.turn_type == "thinking"], ["Use exec with df."]
        )

    def test_archives_in_database(self):
        rows = self.rows_for(OpenClawParser(), self.DATABASE)
        reset = self.transcript(OPENCLAW_RESET_SESSION)
        self.assertEqual(
            [(row.turn_type, row.text.split(":")[0]) for row in reset],
            [
                ("system", "session start"),
                ("user", "check disk space on the server"),
                ("system", "reset"),
            ],
        )
        self.assertIn("archive=reset", reset[0].text)
        # A published archive is parsed in full too: the blob is the
        # canonical copy, the .jsonl.deleted.* file the secondary one.
        published = [
            row
            for row in rows
            if row.session_id == OPENCLAW_DELETED
            and not row.recovered
            and not row.text.startswith("session forgotten")
        ]
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc) for row in published],
            [
                (
                    "system",
                    "session start: key=%s archive=deleted "
                    "name=%s.jsonl.deleted.2026-10-02T08-00-00.000Z.zst version=4"
                    % (OPENCLAW_KEY, OPENCLAW_DELETED),
                    "2026-10-01T09:00:00.000Z",
                ),
                ("user", "check disk space on the server", "2026-10-01T09:00:01.000Z"),
                (
                    "system",
                    "session deleted: %s archive %s.jsonl.deleted.2026-10-02T08-00-00.000Z.zst"
                    " (from openclaw-agent.sqlite session_transcript_archives reason deleted,"
                    " created_at)" % (OPENCLAW_DELETED, OPENCLAW_DELETED),
                    "2026-10-02T08:00:00.000Z",
                ),
            ],
        )
        self.assertEqual({row.project_path for row in published}, {"/srv/proj"})
        cold = [row for row in rows if row.session_id == OPENCLAW_COLD_SESSION]
        self.assertEqual([row.turn_type for row in cold], ["system", "user"])
        self.assertEqual(cold[1].project_path, "/srv/proj")

    def test_chunked_cold_archive_read_chunk_by_chunk(self):
        """A chunked cold archive gives both chunks' entries, in chunk order, after one start."""
        rows = self.rows_for(OpenClawParser(), OPENCLAW_AGENT + "/agent/openclaw-agent.sqlite")
        chunked = [row for row in rows if row.session_id == OPENCLAW_CHUNKED_SESSION]
        self.assertEqual(
            [(row.turn_type, row.text) for row in chunked],
            [
                ("system", "session start: cold_archive=%s.jsonl.zst version=4" % ("d" * 64)),
                ("user", "check disk space on the server"),
            ],
        )
        self.assertEqual({row.project_path for row in chunked}, {"/srv/proj"})

    def test_legacy_jsonl(self):
        rows = self.rows_for(OpenClawParser(), self.LEGACY)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "system",
                "tool_use",
                "tool_result",
                "system",
                "system",
                "assistant",
                "system",
            ],
        )
        start, context, bash, out, compaction, model, asst, error = rows
        self.assertEqual(
            (model.text, model.model, model.timestamp_utc),
            ("model: gpt-5", "gpt-5", "2026-09-20T08:05:10.000Z"),
        )
        self.assertEqual({row.session_id for row in rows}, {OPENCLAW_LEGACY})
        self.assertIn("key=" + OPENCLAW_KEY, start.text)  # from the legacy sessions.json
        self.assertTrue(context.text.startswith("runtime context:"))
        self.assertEqual(
            (bash.tool_name, bash.text, out.tool_use_id), ("bash", "uptime", bash.tool_use_id)
        )
        self.assertEqual(out.text, "[exit=0] up 3 days")
        self.assertEqual(compaction.text, "compaction (tokens_before=5000): checked uptime")
        self.assertEqual(asst.timestamp_utc, "2026-09-20T08:05:10.000Z")  # message ms timestamp
        self.assertEqual((error.text, error.model), ("assistant error: rate limited", "gpt-5"))
        self.assertEqual([row.source_line for row in rows], [1, 2, 3, 3, 4, 5, 5, 5])

    def test_deleted_archive_file(self):
        rel = next(
            artifact.rel for artifact in self.col.artifacts if ".jsonl.deleted." in artifact.rel
        )
        rows = self.rows_for(OpenClawParser(), rel)
        self.assertEqual([row.turn_type for row in rows], ["system", "user"])
        self.assertEqual({row.session_id for row in rows}, {OPENCLAW_DELETED})
        self.assertIn("archive=deleted.2026-10-02T08-00-00.000Z", rows[0].text)

    def test_secondary(self):
        parser = OpenClawParser()
        sessions = OPENCLAW_AGENT + "/sessions/"
        for rel in (
            sessions + OPENCLAW_DELETED + ".jsonl.deleted.2026-10-02T08-00-00.000Z.zst",
            sessions + "x.jsonl.reset.2026-10-01T09-30-00Z",
            sessions + OPENCLAW_LEGACY + OPENCLAW_CHECKPOINT,
            sessions + OPENCLAW_LEGACY + OPENCLAW_REPAIR_BAK,
            sessions + "x.jsonl.pre-doctor-openai-codex-repair-2026-09-20T08-10-00-000Z.bak",
        ):
            self.assertTrue(parser.wants(rel_only(rel)), rel)
            self.assertTrue(parser.secondary(rel_only(rel)), rel)
        for rel in (
            self.DATABASE,
            self.LEGACY,
            ".openclaw/sessions/x.jsonl",
            sessions + "cold/" + "a" * 64 + ".jsonl.zst",
        ):
            self.assertFalse(parser.secondary(rel_only(rel)), rel)

    def test_checkpoint_and_repair_backup(self):
        sessions = OPENCLAW_AGENT + "/sessions/"
        legacy = self.rows_for(OpenClawParser(), self.LEGACY)
        ckpt = self.rows_for(OpenClawParser(), sessions + OPENCLAW_LEGACY + OPENCLAW_CHECKPOINT)
        self.assertEqual(
            [row.turn_type for row in ckpt], ["system", "system", "tool_use", "tool_result"]
        )
        self.assertEqual(
            ckpt[0].text,
            "session start: key=%s copy=%s version=3"
            % (OPENCLAW_KEY, OPENCLAW_CHECKPOINT.lstrip(".")),
        )

        # The entries are copies: same session, project, text and time.
        def key(row):
            return (row.session_id, row.project_path, row.turn_type, row.text, row.timestamp_utc)

        self.assertEqual([key(row) for row in ckpt[1:]], [key(row) for row in legacy[1:4]])
        bak = self.rows_for(OpenClawParser(), sessions + OPENCLAW_LEGACY + OPENCLAW_REPAIR_BAK)
        self.assertIn("copy=jsonl.pre-doctor-branch-repair-", bak[0].text)
        self.assertEqual([key(row) for row in bak[1:-1]], [key(row) for row in legacy[1:]])
        self.assertEqual(
            key(bak[-1]),
            (
                OPENCLAW_LEGACY,
                "/srv/proj",
                "user",
                "abandoned branch prompt",
                "2026-09-20T08:00:30.000Z",
            ),
        )

    def test_copies_deduplicated(self):
        rows, _, _, dropped = cli.collect_rows(self.col, Options(), ["openclaw"])
        # The deleted archive file's user row and the backup's seven entry
        # rows pair with the primary rows. The checkpoint's three entry
        # rows lose the one-to-one primary match to the backup's copies of
        # the same entries and collapse onto those copies, so only the
        # checkpoint's session-start row is kept.
        self.assertEqual(dropped["openclaw"], 11)
        ckpt = "/alice/" + OPENCLAW_AGENT + "/sessions/" + OPENCLAW_LEGACY + OPENCLAW_CHECKPOINT
        kept_ckpt = [row for row in rows if row.source_file == ckpt]
        self.assertEqual(len(kept_ckpt), 1)
        self.assertTrue(kept_ckpt[0].text.startswith("session start:"))
        users = [row for row in rows if row.turn_type == "user"]
        # The abandoned branch survives only in the backup and is kept.
        self.assertEqual(
            [row.source_file for row in users if row.text == "abandoned branch prompt"],
            ["/alice/" + OPENCLAW_AGENT + "/sessions/" + OPENCLAW_LEGACY + OPENCLAW_REPAIR_BAK],
        )
        # The deleted session's prompt is kept once, from the database.
        self.assertEqual(
            [
                row.source_file
                for row in users
                if row.session_id == OPENCLAW_DELETED and not row.recovered
            ],
            ["/alice/" + self.DATABASE],
        )
        # Each copy keeps the session-start row that names it.
        starts = [row for row in rows if row.text.startswith("session start:")]
        self.assertEqual(len([row for row in starts if "copy=" in row.text]), 2)

    def test_not_wanted(self):
        parser = OpenClawParser()
        for artifact in self.col.artifacts:
            if (
                artifact.agent == "openclaw"
                and artifact.rel not in (self.DATABASE, self.LEGACY, OPENCLAW_STATE_DATABASE)
                and ".jsonl.deleted." not in artifact.rel
                and not artifact.rel.endswith((OPENCLAW_CHECKPOINT, OPENCLAW_REPAIR_BAK))
            ):
                self.assertFalse(
                    parser.wants(artifact), artifact.rel
                )  # config, credentials, sidecars, trajectory, .migrated
        for rel in (
            ".openclaw-work/agents/ops/agent/openclaw-agent.sqlite",
            ".clawdbot/agents/main/sessions/x.jsonl",
            ".openclaw/sessions/x.jsonl",
            ".openclaw/agents/main/sessions/x.jsonl.reset.2026-10-01T09-30-00Z",
            ".openclaw/agents/main/sessions/cold/" + "a" * 64 + ".jsonl.zst",
            ".openclaw/state/openclaw.sqlite",
            ".openclaw-work/state/openclaw.sqlite",
        ):
            self.assertTrue(parser.wants(rel_only(rel)), rel)
        for rel in (
            ".openclaw/agents/main/sessions/x.jsonl.migrated",
            ".openclaw/agents/main/sessions/x.jsonl.bak",
            ".openclaw/agents/main/sessions/x.jsonl.bak.2026-10-01T09-30-00Z",
            ".openclaw/agents/main/sessions/x.trajectory.jsonl",
            ".openclaw/state/openclaw.sqlite-wal",
            ".openclaw/logs/raw-stream.jsonl",
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)

    def test_credentials_never_emitted(self):
        rows = self.rows_for(OpenClawParser(), self.DATABASE, include_thinking=True)
        rows += self.rows_for(OpenClawParser(), OPENCLAW_STATE_DATABASE)
        self.assertTrue(rows)
        for token in (OPENCLAW_TOKEN, OPENCLAW_SOCKET_TOKEN):
            self.assertFalse(
                [
                    row
                    for row in rows
                    if token in "|".join(str(value) for value in row.as_dict().values())
                ]
            )
        out = self.tmp / "out"
        import contextlib

        with contextlib.redirect_stdout(io.StringIO()):
            cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), "--include-thinking"])
        for path in out.iterdir():
            if path.is_file():
                text = path.read_text(encoding="utf-8", errors="replace")
                self.assertNotIn(OPENCLAW_TOKEN, text, path.name)
                self.assertNotIn(OPENCLAW_SOCKET_TOKEN, text, path.name)

    def test_truncated_line_is_reported_not_fatal(self):
        clean = [(row.turn_type, row.text) for row in self.rows_for(OpenClawParser(), self.LEGACY)]
        path = self.home / self.LEGACY
        lines = path.read_bytes().split(b"\n")
        lines.insert(2, BAD_LINE.encode() + b"\r")
        path.write_bytes(b"\n".join(lines))
        write_bad_line(path)
        rows = self.rows_for(OpenClawParser(), self.LEGACY)
        self.assertEqual([(row.turn_type, row.text) for row in rows[:-2]], clean)
        bad = bad_line_text()
        self.assertEqual(
            [(row.turn_type, row.source_line, row.session_id, row.text) for row in rows[-2:]],
            [
                ("system", 3, OPENCLAW_LEGACY, bad),
                ("system", len(lines), OPENCLAW_LEGACY, bad),
            ],
        )

    def test_bad_line_in_archive_blob(self):
        connection = sqlite3.connect(str(self.home / self.DATABASE))
        rowid, name, blob = connection.execute(
            "SELECT rowid, archive_name, archive_blob FROM session_transcript_archives"
            " WHERE encoding = 'identity'"
        ).fetchone()
        connection.execute(
            "UPDATE session_transcript_archives SET archive_blob = ? WHERE rowid = ?",
            (bytes(blob).rstrip(b"\n") + b"\n" + BAD_LINE.encode(), rowid),
        )
        connection.commit()
        connection.close()
        line = bytes(blob).rstrip(b"\n").count(b"\n") + 2
        rows = self.transcript(OPENCLAW_RESET_SESSION)
        self.assertEqual([row.turn_type for row in rows], ["system", "user", "system", "system"])
        self.assertEqual(rows[1].text, "check disk space on the server")
        try:
            json.loads(BAD_LINE)
        except ValueError as error:
            reason = str(error)
        else:
            raise AssertionError("bad line parses")
        self.assertEqual(
            (rows[-1].source_line, rows[-1].text),
            (
                rowid,
                "parser: archive %s line %d: unparseable line (%s): %s"
                % (name, line, reason, BAD_LINE),
            ),
        )

    def test_truncated_copy_is_reported_not_fatal(self):
        rel = OPENCLAW_AGENT + "/sessions/" + OPENCLAW_LEGACY + OPENCLAW_REPAIR_BAK
        write_bad_line(self.home / rel)
        rows = self.rows_for(OpenClawParser(), rel)
        self.assertEqual(len(rows), 10)
        self.assertEqual(rows[-2].text, "abandoned branch prompt")
        self.assertEqual((rows[-1].turn_type, rows[-1].text), ("system", bad_line_text()))

    def test_bad_event_row_is_reported_not_fatal(self):
        connection = sqlite3.connect(str(self.home / self.DATABASE))
        connection.execute(
            "INSERT INTO transcript_events VALUES (?,?,?,?,NULL,NULL,NULL)",
            (OPENCLAW_SESSION, 9, '{"type":"message","trunc', 1790845300000),
        )
        connection.commit()
        connection.close()
        rows = self.transcript(OPENCLAW_SESSION)
        self.assertEqual(len(rows), 8)
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].timestamp_utc), ("system", "2026-10-01T09:01:40.000Z")
        )
        self.assertIn("event seq 9 unreadable", rows[-1].text)

    def test_cold_archive_without_bytes_is_reported(self):
        connection = sqlite3.connect(str(self.home / self.DATABASE))
        connection.execute(
            "INSERT INTO session_transcript_cold_archives VALUES "
            "(?,'g9',?,?,1,1,1,0,1790852000000,'sqlite',NULL)",
            ("lost-cold", "e" * 64 + ".jsonl.zst", "e" * 64),
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(OpenClawParser(), self.DATABASE)
        lost = [row for row in rows if row.session_id == "lost-cold"]
        self.assertEqual(
            [(row.turn_type, row.text) for row in lost],
            [("system", "parser: cold archive %s.jsonl.zst has no stored bytes" % ("e" * 64))],
        )
        self.assertEqual(lost[0].timestamp_utc, "2026-10-01T10:53:20.000Z")

    def test_null_event_row_is_reported_not_fatal(self):
        # A row whose event_json is valid JSON but not an object (here
        # `null`) is reported as "not a JSON object".
        connection = sqlite3.connect(str(self.home / self.DATABASE))
        connection.execute(
            "INSERT INTO transcript_events VALUES (?,?,?,?,NULL,NULL,NULL)",
            (OPENCLAW_SESSION, 9, "null", 1790845300000),
        )
        connection.commit()
        connection.close()
        rows = self.transcript(OPENCLAW_SESSION)
        self.assertEqual(len(rows), 8)
        self.assertEqual(rows[-1].turn_type, "system")
        self.assertIn("event seq 9 unreadable: not a JSON object", rows[-1].text)

    # -- session sync -----------------------------------------------------------

    SHARE_TEXT = (
        "session synced: public share %s generation %s (from openclaw-agent.sqlite"
        " session_nodes.entry_json, publicShare.createdAt)"
    )

    def test_public_share_row(self):
        self.assertTrue(OpenClawParser.records_sync)
        rows = [
            row
            for row in self.rows_for(OpenClawParser(), self.DATABASE)
            if row.session_id == OPENCLAW_SESSION
        ]
        # After the session-start and starting-model rows.
        share = next(row for row in rows if row.text.startswith("session synced:"))
        self.assertEqual([row.text.split(":")[0] for row in rows[:2]], ["session start", "model"])
        self.assertEqual(
            (share.turn_type, share.text, share.timestamp_utc, share.project_path),
            (
                "system",
                self.SHARE_TEXT % (OPENCLAW_SHARE_ID, OPENCLAW_SESSION),
                "2026-10-01T09:02:00.000Z",
                "/srv/proj",
            ),
        )
        self.assertEqual(share.source_line, 1)  # session_nodes rowid
        self.assertEqual(
            [row.text for row in rows if row.text.startswith("session synced:")], [share.text]
        )

    def test_unpublished_session_has_no_share_row(self):
        rows = self.rows_for(OpenClawParser(), self.DATABASE)
        synced = {row.session_id for row in rows if row.text.startswith("session synced:")}
        self.assertEqual(synced, {OPENCLAW_SESSION})

    def test_share_without_transcript_rows(self):
        connection = sqlite3.connect(str(self.home / self.DATABASE))
        entry = {
            "sessionId": "gone",
            "publicShare": {"id": "f" * 48, "sessionId": "gone", "createdAt": 1790900000000},
        }
        connection.execute(
            "INSERT INTO session_nodes VALUES ('agent:main:x','gone',?,0,NULL,NULL)",
            (json.dumps(entry),),
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(OpenClawParser(), self.DATABASE)
        self.assertEqual(
            (rows[-1].session_id, rows[-1].text, rows[-1].timestamp_utc),
            ("gone", self.SHARE_TEXT % ("f" * 48, "gone"), "2026-10-02T00:13:20.000Z"),
        )

    def test_bad_entry_json_is_reported_not_fatal(self):
        connection = sqlite3.connect(str(self.home / self.DATABASE))
        connection.execute(
            "INSERT INTO session_nodes VALUES ('agent:main:y','y',?,0,NULL,NULL)",
            (BAD_LINE,),
        )
        rowid = connection.execute(
            "SELECT rowid FROM session_nodes WHERE session_key = 'agent:main:y'"
        )
        rowid = rowid.fetchone()[0]
        connection.commit()
        connection.close()
        rows = self.rows_for(OpenClawParser(), self.DATABASE)
        self.assertIn(
            self.SHARE_TEXT % (OPENCLAW_SHARE_ID, OPENCLAW_SESSION), [row.text for row in rows]
        )
        reason = bad_line_text().split("(", 1)[1].split("): ", 1)[0]
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].source_line, rows[-1].text),
            (
                "system",
                rowid,
                "parser: session_nodes entry_json unparseable (%s): %s" % (reason, BAD_LINE),
            ),
        )

    BACKUP_TEXT = (
        "backup uploaded: %s to %s key %s namespace %s"
        " (from state/openclaw.sqlite backup_runs, created_at)"
    )

    def test_backup_rows(self):
        rows = [
            row
            for row in self.rows_for(OpenClawParser(), OPENCLAW_STATE_DATABASE)
            if row.text.startswith("backup uploaded:")
        ]
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    "",
                    "2026-10-01T10:20:00.000Z",
                    self.BACKUP_TEXT % ("archive", "r2-offsite", OPENCLAW_BACKUP_KEY, "main"),
                ),
                (
                    "system",
                    "",
                    "2026-10-01T11:00:00.000Z",
                    self.BACKUP_TEXT
                    % ("git", "/home/alice/openclaw-backup.git", OPENCLAW_BACKUP_COMMIT, "main"),
                ),
            ],
        )  # the failed push, the local archive and the failed run yield none
        self.assertEqual([row.source_line for row in rows], [1, 2])
        self.assertFalse([row for row in rows if row.text.startswith("session synced:")])

    def test_bad_manifest_json_is_reported_not_fatal(self):
        path = self.home / OPENCLAW_STATE_DATABASE
        connection = sqlite3.connect(str(path))
        connection.execute(
            "INSERT INTO backup_runs VALUES ('run-99',1790870000000,'/x','ok',?)", (BAD_LINE,)
        )
        connection.commit()
        connection.close()
        rows = [
            row
            for row in self.rows_for(OpenClawParser(), OPENCLAW_STATE_DATABASE)
            if row.text.startswith(("backup uploaded:", "parser:"))
        ]
        self.assertEqual(len(rows), 3)
        self.assertTrue(rows[1].text.startswith("backup uploaded: git"))
        self.assertTrue(
            rows[-1].text.startswith("parser: backup_runs manifest_json unparseable ("), rows[-1]
        )
        self.assertTrue(rows[-1].text.endswith("): " + BAD_LINE))

    def test_state_database_not_sqlite_is_one_row(self):
        path = self.home / OPENCLAW_STATE_DATABASE
        for side in ("-wal", "-shm"):
            Path(str(path) + side).unlink(missing_ok=True)
        path.write_bytes(b"not a database")
        rows = self.rows_for(OpenClawParser(), OPENCLAW_STATE_DATABASE)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("system", "parser: state/openclaw.sqlite is not a SQLite database")],
        )

    def test_bad_legacy_sessions_json_is_one_row(self):
        # A legacy sessions.json that does not decode or is not an object
        # is a whole-file row; the transcript's rows still come out.
        clean = [(row.turn_type, row.text) for row in self.rows_for(OpenClawParser(), self.LEGACY)]
        store = self.home / OPENCLAW_AGENT / "sessions/sessions.json"
        cut = '{"%s": {"sessionId": "%s"' % (OPENCLAW_KEY, OPENCLAW_LEGACY)
        try:
            json.loads(cut)
        except ValueError as error:
            reason = str(error)
        else:
            raise AssertionError("cut sessions.json parses")
        for content, message in (
            (cut, "parser: sessions.json unparseable (%s)" % reason),
            ("[1, 2]", "parser: sessions.json is not a JSON object"),
        ):
            store.write_text(content, encoding="utf-8")
            rows = self.rows_for(OpenClawParser(), self.LEGACY)
            self.assertEqual(len(rows), len(clean) + 1)
            body = [(row.turn_type, row.text) for row in rows[:-1]]
            self.assertEqual(body[1:], clean[1:])
            self.assertNotIn("key=", body[0][1])  # the key is lost, not guessed
            self.assertEqual(
                (rows[-1].turn_type, rows[-1].source_line, rows[-1].session_id),
                ("system", 0, OPENCLAW_LEGACY),
            )
            self.assertEqual(rows[-1].project_path, rows[0].project_path)
            self.assertEqual((rows[-1].timestamp_utc, rows[-1].text), ("", message))

    def test_non_object_line_is_reported_not_fatal(self):
        # A line that is valid JSON but not an object is its own row
        # carrying the line; the lines before and after still come out.
        clean = [(row.turn_type, row.text) for row in self.rows_for(OpenClawParser(), self.LEGACY)]
        path = self.home / self.LEGACY
        lines = path.read_bytes().split(b"\n")
        lines.insert(2, b'["not", "an", "object"]')
        lines.insert(4, b"null")
        path.write_bytes(b"\n".join(lines))
        rows = self.rows_for(OpenClawParser(), self.LEGACY)
        self.assertEqual([(row.turn_type, row.text) for row in rows[:-2]], clean)
        self.assertEqual(
            [
                (row.turn_type, row.source_line, row.session_id, row.timestamp_utc, row.text)
                for row in rows[-2:]
            ],
            [
                (
                    "system",
                    3,
                    OPENCLAW_LEGACY,
                    "",
                    'parser: unparseable line (not a JSON object): ["not", "an", "object"]',
                ),
                (
                    "system",
                    5,
                    OPENCLAW_LEGACY,
                    "",
                    "parser: unparseable line (not a JSON object): null",
                ),
            ],
        )

    def test_non_object_line_in_archive_blob(self):
        connection = sqlite3.connect(str(self.home / self.DATABASE))
        rowid, name, blob = connection.execute(
            "SELECT rowid, archive_name, archive_blob FROM session_transcript_archives"
            " WHERE encoding = 'identity'"
        ).fetchone()
        head, _, rest = bytes(blob).partition(b"\n")
        connection.execute(
            "UPDATE session_transcript_archives SET archive_blob = ? WHERE rowid = ?",
            (head + b"\n42\n" + rest, rowid),
        )
        connection.commit()
        connection.close()
        rows = self.transcript(OPENCLAW_RESET_SESSION)
        self.assertEqual([row.turn_type for row in rows], ["system", "user", "system", "system"])
        self.assertEqual(rows[1].text, "check disk space on the server")
        self.assertEqual(
            (rows[-1].source_line, rows[-1].text),
            (rowid, "parser: archive %s line 2: unparseable line (not a JSON object): 42" % name),
        )

    # -- relay, permission, model, past-use and deletion rows -------------------

    def connect_database(self):
        return sqlite3.connect(str(self.home / self.DATABASE))

    def texts(self, prefix, rel=None, session_id=None):
        return [
            row
            for row in self.rows_for(OpenClawParser(), rel or self.DATABASE)
            if row.text.startswith(prefix) and (session_id is None or row.session_id == session_id)
        ]

    def test_relay_row_from_conversation(self):
        self.assertTrue(OpenClawParser.records_relay)
        rows = self.texts("session relayed:")
        self.assertEqual(
            [(row.session_id, row.timestamp_utc, row.project_path, row.text) for row in rows],
            [
                (
                    OPENCLAW_SESSION,
                    "2026-10-01T09:00:00.500Z",
                    "/srv/proj",
                    "session relayed: telegram account default direct peer 123456789 thread -"
                    " role primary (from openclaw-agent.sqlite session_conversations and"
                    " conversations, first_seen_at)",
                )
            ],
        )  # the reset session's Control UI conversation and delivery are no relay

    def test_relay_row_fallbacks(self):
        connection = self.connect_database()
        connection.execute("DELETE FROM session_conversations")
        connection.commit()
        connection.close()
        self.assertEqual(
            [row.text for row in self.texts("session relayed:")],
            [
                "session relayed: telegram account default direct (from openclaw-agent.sqlite"
                " session_windows, started_at)"
            ],
        )
        connection = self.connect_database()
        connection.execute("UPDATE session_windows SET channel = 'webchat'")
        connection.commit()
        connection.close()
        rows = self.texts("session relayed:")
        self.assertEqual(
            [(row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    OPENCLAW_SESSION,
                    "2026-10-01T09:02:00.000Z",
                    "session relayed: telegram account default direct from 123456789 to -"
                    " (from openclaw-agent.sqlite session_nodes.entry_json delivery,"
                    " updated_at)",
                )
            ],
        )
        connection = self.connect_database()
        connection.execute("UPDATE session_windows SET hook_external_content_source = 'gmail'")
        connection.commit()
        connection.close()
        self.assertEqual(
            [row.text for row in self.texts("session relayed:")],
            [
                "session relayed: gmail hook (from openclaw-agent.sqlite session_windows"
                " hook_external_content_source, started_at)"
            ],
        )

    def test_model_override_row(self):
        rows = self.texts("model: gpt-5 provider=openai source=")
        self.assertEqual(
            [(row.session_id, row.model, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    OPENCLAW_SESSION,
                    "gpt-5",
                    "2026-10-01T09:02:00.000Z",
                    "model: gpt-5 provider=openai source=user (from openclaw-agent.sqlite"
                    " session_nodes.entry_json modelOverride, updated_at)",
                )
            ],
        )

    def test_session_permission_settings(self):
        rows = self.texts("permission setting:")
        self.assertEqual(
            [row.text.split(" (from")[0] for row in rows],
            [
                "permission setting: elevatedLevel=on",
                "permission setting: execHost=gateway",
                'permission setting: toolOverrides={"mcpServers":{"github":false}}',
                "permission setting: sendPolicy=allow",
            ],
        )
        self.assertEqual({row.session_id for row in rows}, {OPENCLAW_SESSION})
        self.assertEqual({row.timestamp_utc for row in rows}, {"2026-10-01T09:02:00.000Z"})
        self.assertTrue(
            rows[0].text.endswith(
                "(from openclaw-agent.sqlite session_nodes.entry_json, updated_at)"
            )
        )

    def test_archived_row(self):
        rows = self.texts("session archived:")
        self.assertEqual(
            [(row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    OPENCLAW_RESET_SESSION,
                    "2026-10-01T09:46:40.000Z",
                    "session archived: manual by human:operator-1 (from openclaw-agent.sqlite"
                    " session_nodes.entry_json, archivedAt)",
                )
            ],
        )

    def test_forgotten_from_memory_row(self):
        rows = self.texts("session forgotten from memory:")
        self.assertEqual(
            [(row.session_id, row.project_path, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    OPENCLAW_DELETED,
                    "/srv/proj",
                    "2026-10-02T08:33:20.000Z",
                    "session forgotten from memory: %s agent main reason=forgotten (from"
                    " openclaw-agent.sqlite memory_session_tombstones, created_at)"
                    % OPENCLAW_DELETED,
                )
            ],
        )

    def test_pending_inputs(self):
        rows = [
            row
            for row in self.rows_for(OpenClawParser(), self.DATABASE)
            if row.text in ("also restart nginx", "show the biggest directories")
            or row.text.startswith("pending input:")
        ]
        self.assertEqual(
            [
                (row.turn_type, row.session_id, row.timestamp_utc, row.recovered, row.text)
                for row in rows
            ],
            [
                ("user", OPENCLAW_SESSION, "2026-10-01T09:00:50.000Z", "", "also restart nginx"),
                (
                    "system",
                    OPENCLAW_SESSION,
                    "2026-10-01T09:00:50.000Z",
                    "",
                    "pending input: cancelled, not consumed by the transcript (input in-1 run"
                    " run-1; from openclaw-agent.sqlite session_pending_inputs, accepted_at)",
                ),
                (
                    "user",
                    OPENCLAW_SESSION,
                    "2026-10-01T09:00:30.000Z",
                    "yes",
                    "show the biggest directories",
                ),
            ],
        )  # the prompt consumed by a transcript entry the database holds yields nothing
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})

    def test_bad_pending_input_is_reported_not_fatal(self):
        connection = self.connect_database()
        connection.execute(
            "INSERT INTO session_pending_inputs VALUES (9,'in-9',?,?,'run-9',?,'queued',1,NULL)",
            (OPENCLAW_KEY, OPENCLAW_SESSION, BAD_LINE),
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(OpenClawParser(), self.DATABASE)
        self.assertIn("also restart nginx", [row.text for row in rows])
        reason = bad_line_text().split("(", 1)[1].split("): ", 1)[0]
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].source_line, rows[-1].text),
            (
                "system",
                9,
                "parser: session_pending_inputs message_json unparseable (%s): %s"
                % (reason, BAD_LINE),
            ),
        )

    def test_full_text_index_rows(self):
        rows = [row for row in self.rows_for(OpenClawParser(), self.DATABASE) if row.recovered]
        fts = [row for row in rows if row.text == OPENCLAW_FTS_ONLY]
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.source_line) for row in fts],
            [("user", OPENCLAW_DELETED, "2026-10-01T07:33:20.000Z", 2)],
        )  # the index row of an entry the transcript holds is dropped
        self.assertEqual(
            [row.text for row in rows], ["show the biggest directories", OPENCLAW_FTS_ONLY]
        )

    def test_delivery_rows(self):
        rows = self.texts("channel delivery")
        self.assertEqual(
            [(row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    OPENCLAW_SESSION,
                    "2026-10-01T09:00:06.000Z",
                    "channel delivery: turn replied channel telegram conversation conv-tg"
                    " platform message tg-msg-77 (from openclaw-agent.sqlite"
                    " conversation_deliveries, created_at)",
                ),
                (
                    OPENCLAW_SESSION,
                    "2026-10-01T09:01:30.000Z",
                    "channel delivery reply: thanks, looks fine (reply message tg-msg-78; from"
                    " openclaw-agent.sqlite conversation_deliveries, reply_timestamp)",
                ),
            ],
        )

    def test_postgres_anchor_is_one_row(self):
        for rel, label, where in (
            (self.DATABASE, "agent", "transcripts are"),
            (OPENCLAW_STATE_DATABASE, "state", "the Gateway state is"),
        ):
            path = self.home / rel
            for side in ("-wal", "-shm"):
                Path(str(path) + side).unlink(missing_ok=True)
            path.unlink()
            connection = sqlite3.connect(str(path))
            connection.execute(
                "CREATE TABLE openclaw_engine_anchor (store_id TEXT NOT NULL, engine TEXT NOT NULL"
                " CHECK (engine = 'postgres'), schema TEXT NOT NULL)"
            )
            connection.execute(
                "INSERT INTO openclaw_engine_anchor VALUES ('st-1','postgres','oc_%s_0123')" % label
            )
            connection.commit()
            connection.close()
            rows = self.rows_for(OpenClawParser(), rel)
            self.assertEqual(
                [(row.turn_type, row.source_line, row.text) for row in rows],
                [
                    (
                        "system",
                        0,
                        "parser: %s database is a PostgreSQL anchor (store st-1, schema"
                        " oc_%s_0123): %s on the database server" % (label, label, where),
                    )
                ],
            )

    def test_state_permission_rows(self):
        rows = self.texts("permission ", rel=OPENCLAW_STATE_DATABASE)
        self.assertEqual(
            [row.text.split(" (from")[0] for row in rows],
            [
                "permission decision: allow-once status=allowed kind=exec resolver=device"
                " reason=user approval appr-1",
                "permission decision: pending status=pending kind=exec resolver=- reason=-"
                " approval appr-2",
                "permission setting: defaults.security=allowlist",
                "permission setting: defaults.ask=on-miss",
                "permission setting: agents.main.autoAllowSkills=true",
                "permission setting: agents.main.allowlist=df source=allow-always",
                "permission setting: agents.main.mcpTools=github/search source=allow-always",
                "permission setting: standing grant grant-1 allow-always cron job cron-7 agent"
                " main binding exec:df approval appr-1 expires=- revoked=- uses=3",
                "permission setting: channel telegram account default allowFrom=123456789",
            ],
        )
        allowed, pending = rows[:2]
        self.assertEqual(
            (allowed.session_id, allowed.tool_name, allowed.tool_use_id, allowed.timestamp_utc),
            (OPENCLAW_SESSION, "exec", "call_01", "2026-10-01T09:00:04.500Z"),
        )
        self.assertTrue(allowed.text.endswith("operator_approvals, resolved_at_ms)"))
        self.assertTrue(pending.text.endswith("operator_approvals, created_at_ms)"))
        self.assertEqual(rows[2].timestamp_utc, "2026-10-01T08:58:20.000Z")
        self.assertTrue(rows[2].text.endswith("exec_approvals_config, updated_at_ms)"))

    def test_channel_ingress_row(self):
        rows = self.texts("channel ingress:", rel=OPENCLAW_STATE_DATABASE)
        self.assertEqual(
            [(row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "",
                    "2026-10-01T09:00:00.900Z",
                    'channel ingress: telegram account default status completed: {"text":"check'
                    ' disk space on the server"} (from state/openclaw.sqlite'
                    " channel_ingress_events, received_at)",
                )
            ],
        )

    def test_source_host_row(self):
        rows = self.texts("source host:", rel=OPENCLAW_STATE_DATABASE)
        self.assertEqual(
            [(row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    OPENCLAW_SESSION,
                    "2026-10-01T09:03:20.000Z",
                    "source host: github account octo-bot id 4242 identity system-configured;"
                    " publication published repository octo/infra branch openclaw/disk-check"
                    " pull request https://github.com/octo/infra/pull/7 (from"
                    " state/openclaw.sqlite github_publication_requests, created_at_ms)",
                )
            ],
        )

    def test_git_remote_row(self):
        # `projects.origin_url` at `created_at_ms`, the project path its
        # `repo_root`; a project without an origin has no row.
        rows = self.texts("git remote:", rel=OPENCLAW_STATE_DATABASE)
        self.assertEqual(
            [
                (row.turn_type, row.session_id, row.project_path, row.timestamp_utc, row.text)
                for row in rows
            ],
            [
                (
                    "system",
                    "",
                    "/srv/proj",
                    "2026-10-01T08:58:20.500Z",
                    "git remote: " + PROJECT_REMOTE,
                )
            ],
        )

    def test_bad_exec_config_withholds_the_text(self):
        connection = sqlite3.connect(str(self.home / OPENCLAW_STATE_DATABASE))
        connection.execute(
            "UPDATE exec_approvals_config SET raw_json = ?",
            ('{"socket": {"token": "%s"' % OPENCLAW_SOCKET_TOKEN,),
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(OpenClawParser(), OPENCLAW_STATE_DATABASE)
        self.assertTrue(rows[-1].text.startswith("parser: exec_approvals_config raw_json"))
        self.assertTrue(
            rows[-1].text.endswith("text withheld, it holds the approvals socket token)")
        )
        self.assertFalse([row for row in rows if OPENCLAW_SOCKET_TOKEN in row.text])
