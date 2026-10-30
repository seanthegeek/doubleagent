"""Tests of the Cline parser."""

import json
import sqlite3

from doubleagent.dedup import deduplicate
from doubleagent.inputs import open_input
from doubleagent.model import summarise
from doubleagent.parsers import Options
from doubleagent.parsers.cline import ClineParser
from doubleagent.parsers.cline_legacy import iter_json_array
from doubleagent.parsers.roo_code import RooCodeParser
from fixtures import (
    BAD_LINE,
    CLINE_BLOB_DELETED,
    CLINE_BLOB_SESSION,
    CLINE_CLI_TASK,
    CLINE_CLOUD_SESSION,
    CLINE_DENIED_SESSION,
    CLINE_GLOBAL_SETTINGS,
    CLINE_GLOBAL_STATE,
    CLINE_HOOKS_LOG,
    CLINE_IMPORTED_SESSION,
    CLINE_MCP_SETTINGS,
    CLINE_OLD_SESSION,
    CLINE_PENDING_SESSION,
    CLINE_PENDING_START,
    CLINE_QUEUE_FAILED,
    CLINE_QUEUE_PENDING,
    CLINE_SESSION,
    CLINE_SYNC_QUEUE,
    CLINE_SYNC_SECRET,
    CLINE_TASK,
    ROO_CLI_TASK,
    ROO_GONE_TASK,
    ROO_TASK,
    bad_line_text,
    build_cline_blob_upload,
    build_cline_denied_and_imported,
    build_cline_hooks_log,
    build_cline_settings_and_cloud,
    build_cline_sync_queue,
    write_bad_line,
)

from .base import ParserBase

CLINE_GS = ".config/Code/User/globalStorage/saoudrizwan.claude-dev/"


class ClineTests(ParserBase):
    """Test the Cline parser."""

    UI_MESSAGES = CLINE_GS + "tasks/%s/ui_messages.json" % CLINE_TASK
    SDK = ".cline/data/sessions/%s/%s" % (CLINE_SESSION, CLINE_SESSION)

    def test_rows(self):
        rows = self.rows_for(ClineParser(), self.UI_MESSAGES)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "user",
                "system",
                "tool_use",
                "assistant",
                "tool_use",
                "tool_result",
                "system",
                "assistant",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "cline", CLINE_TASK),
            )
            self.assertEqual(
                (row.project_path, row.git_branch, row.model),
                ("/srv/proj", "", "claude-sonnet-4-5"),
            )
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        user, api, read, _text, command, out, ckpt, done = rows
        self.assertEqual(
            (user.text, user.timestamp_utc),
            ("fix tests | [1 image(s)]", "2026-10-03T10:00:00.000Z"),
        )
        self.assertEqual(
            api.text,
            "api_req_started: tokensIn=1200 tokensOut=80 cacheWrites=0 cacheReads=900 cost=0.0041",
        )
        self.assertEqual(
            (read.tool_name, read.tool_use_id, read.text),
            ("readFile", "1791021602000", "src/app.py | /srv/proj/src/app.py"),
        )
        self.assertEqual(
            (command.tool_name, command.tool_use_id, command.text),
            ("execute_command", "1791021604000", "pytest -q"),
        )
        self.assertEqual((out.tool_use_id, out.text), ("1791021604000", "3 passed"))
        self.assertEqual(ckpt.text, "checkpoint_created: abc123")
        self.assertEqual(done.text, "All tests pass.")
        self.assertEqual(
            [row.source_line for row in rows], [1, 2, 4, 5, 6, 7, 8, 9]
        )  # empty ask at 10 skipped

    def test_thinking_opt_in(self):
        rows = self.rows_for(ClineParser(), self.UI_MESSAGES, include_thinking=True)
        self.assertEqual(
            [row.text for row in rows if row.turn_type == "thinking"], ["read the test first"]
        )
        rows = self.rows_for(ClineParser(), self.SDK + ".messages.json", include_thinking=True)
        self.assertEqual(
            [row.text for row in rows if row.turn_type == "thinking"], ["run the suite"]
        )

    def test_api_history_only_without_ui_messages(self):
        self.assertEqual(
            self.rows_for(
                ClineParser(), CLINE_GS + "tasks/%s/api_conversation_history.json" % CLINE_TASK
            ),
            [],
        )
        rows = self.rows_for(
            ClineParser(), ".cline/data/tasks/%s/api_conversation_history.json" % CLINE_CLI_TASK
        )
        self.assertEqual([row.turn_type for row in rows], ["user", "tool_use", "tool_result"])
        # No per-message timestamps: every row inherits the task id's time.
        self.assertEqual({row.timestamp_utc for row in rows}, {"2026-10-03T09:55:00.000Z"})
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})
        use, result = rows[1], rows[2]
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text),
            ("execute_command", "toolu_01A", "pytest -q"),
        )
        self.assertEqual((result.tool_use_id, result.text), ("toolu_01A", "3 passed"))

    def test_history_and_metadata(self):
        rows = self.rows_for(ClineParser(), CLINE_GS + "state/taskHistory.json")
        self.assertEqual(len(rows), 1)
        self.assertEqual(
            (rows[0].turn_type, rows[0].session_id, rows[0].project_path),
            ("system", CLINE_TASK, "/srv/proj"),
        )
        self.assertTrue(rows[0].text.startswith("task: fix tests | tokensIn=1200"))
        rows = self.rows_for(ClineParser(), CLINE_GS + "tasks/%s/task_metadata.json" % CLINE_TASK)
        self.assertEqual(
            [row.text.split(":")[0] for row in rows],
            [
                "environment",
                "model",
                "permission mode",
                "permission mode",
                "file cline_read",
                "model",
            ],
        )
        self.assertEqual(rows[4].timestamp_utc, "2026-10-03T10:00:02.000Z")
        self.assertIn("src/app.py", rows[4].text)

    def test_model_and_mode_rows_from_model_usage(self):
        rows = self.rows_for(ClineParser(), CLINE_GS + "tasks/%s/task_metadata.json" % CLINE_TASK)
        got = [
            (row.turn_type, row.text, row.timestamp_utc, row.model, row.session_id, row.source_line)
            for row in rows
            if row.text.startswith(("model", "permission"))
        ]
        self.assertEqual(
            got,
            [
                (
                    "system",
                    "model: claude-sonnet-4-5 model_provider_id=anthropic mode=plan",
                    "2026-10-03T10:00:00.300Z",
                    "claude-sonnet-4-5",
                    CLINE_TASK,
                    1,
                ),
                (
                    "system",
                    "permission mode: plan",
                    "2026-10-03T10:00:00.300Z",
                    "claude-sonnet-4-5",
                    CLINE_TASK,
                    1,
                ),
                # the second entry changes only the mode
                (
                    "system",
                    "permission mode: plan -> act",
                    "2026-10-03T10:00:00.500Z",
                    "claude-sonnet-4-5",
                    CLINE_TASK,
                    2,
                ),
                (
                    "system",
                    "model: claude-sonnet-4-5 -> claude-opus-4-5 model_provider_id=anthropic "
                    "mode=act",
                    "2026-10-03T10:00:08.000Z",
                    "claude-opus-4-5",
                    CLINE_TASK,
                    3,
                ),
            ],
        )
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})

    def test_sdk_session(self):
        rows = self.rows_for(ClineParser(), self.SDK + ".json")
        self.assertEqual(
            [row.text.split(":")[0] for row in rows], ["session start", "model", "session end"]
        )
        self.assertEqual(
            (rows[0].session_id, rows[0].project_path, rows[0].timestamp_utc),
            (CLINE_SESSION, "/srv/proj", "2026-10-03T10:01:40.000Z"),
        )
        self.assertIn("title=fix tests", rows[0].text)
        self.assertNotIn("model=", rows[0].text)  # the model has its own row
        self.assertEqual(
            (rows[1].turn_type, rows[1].text, rows[1].model, rows[1].timestamp_utc),
            (
                "system",
                "model: claude-sonnet-4-5 provider=anthropic",
                "claude-sonnet-4-5",
                "2026-10-03T10:01:40.000Z",
            ),
        )
        self.assertEqual((rows[1].session_id, rows[1].project_path), (CLINE_SESSION, "/srv/proj"))
        rows = self.rows_for(ClineParser(), self.SDK + ".messages.json")
        self.assertEqual(
            [row.turn_type for row in rows],
            ["user", "tool_use", "tool_result", "system", "assistant"],
        )
        _user, use, result, switch, asst = rows
        # m2 names the manifest's model, so no row; m4 switches
        self.assertEqual(
            (switch.text, switch.model, switch.timestamp_utc, switch.source_line),
            (
                "model: claude-sonnet-4-5 -> claude-opus-4-5 provider=anthropic",
                "claude-opus-4-5",
                result.timestamp_utc,
                4,
            ),
        )
        self.assertEqual(asst.model, "claude-opus-4-5")
        self.assertEqual({row.session_id for row in rows}, {CLINE_SESSION})
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text), ("bash", "call_1", "pytest -q")
        )
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.text), ("bash", "call_1", "3 passed")
        )
        self.assertEqual(asst.timestamp_utc, result.timestamp_utc)  # no ts: previous message's time
        self.assertEqual([row.source_line for row in rows], [1, 2, 3, 4, 4])

    def test_sdk_messages_without_manifest_start_from_first_model(self):
        (self.home / (self.SDK + ".json")).unlink()
        rows = self.rows_for(ClineParser(), self.SDK + ".messages.json")
        models = [(row.text, row.source_line) for row in rows if row.text.startswith("model:")]
        self.assertEqual(
            models,
            [
                ("model: claude-sonnet-4-5 provider=anthropic", 2),
                ("model: claude-sonnet-4-5 -> claude-opus-4-5 provider=anthropic", 4),
            ],
        )

    def test_sessions_database_only_lists_sessions_without_manifest(self):
        rows = self.rows_for(ClineParser(), ".cline/data/db/sessions.db")
        self.assertEqual([row.session_id for row in rows], [CLINE_OLD_SESSION] * 2)
        self.assertEqual(rows[0].timestamp_utc, "2026-10-02T23:00:00.000Z")
        self.assertIn("an older deleted session", rows[0].text)
        self.assertEqual(
            (rows[1].text, rows[1].timestamp_utc),
            ("model: claude-sonnet-4-5 provider=anthropic", "2026-10-02T23:00:00.000Z"),
        )

    SEARCH = ".cline/data/db/session-search.db"
    INPUT_HISTORY = ".cline/data/cache/user_input_history.jsonl"

    def test_input_history_rows(self):
        rows = self.rows_for(ClineParser(), self.INPUT_HISTORY)
        self.assertEqual(
            [
                (
                    row.source_line,
                    row.turn_type,
                    row.timestamp_utc,
                    row.session_id,
                    row.project_path,
                    row.text,
                )
                for row in rows
            ],
            [
                # Stamped with the last rewrite's time, not its own: undated.
                (1, "user", "", "", "", "fix tests"),
                # The newest prompt, written by that rewrite.
                (2, "user", "2026-10-03T10:05:00.000Z", "", "", "list the open ports"),
            ],
        )

    def test_input_history_truncated_line(self):
        path = self.home / self.INPUT_HISTORY
        first, rest = path.read_bytes().split(b"\n", 1)
        path.write_bytes(first + b"\n" + BAD_LINE.encode() + b"\r\n" + rest)
        write_bad_line(path)
        rows = self.rows_for(ClineParser(), self.INPUT_HISTORY)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.timestamp_utc, row.text) for row in rows],
            [
                (1, "user", "", "fix tests"),
                (3, "user", "", "list the open ports"),
                (2, "system", "", bad_line_text()),
                (4, "system", "", bad_line_text()),
            ],
        )

    def test_secondary(self):
        parser = ClineParser()
        self.assertTrue(parser.secondary(self.artifact(self.SEARCH)))
        self.assertTrue(parser.secondary(self.artifact(self.INPUT_HISTORY)))
        for rel in (self.UI_MESSAGES, self.SDK + ".messages.json", ".cline/data/db/sessions.db"):
            self.assertFalse(parser.secondary(self.artifact(rel)), rel)

    def test_session_search_rows(self):
        rows = self.rows_for(ClineParser(), self.SEARCH)
        self.assertEqual(
            [
                (row.source_line, row.turn_type, row.session_id, row.timestamp_utc, row.text)
                for row in rows
            ],
            [
                (
                    0,
                    "system",
                    CLINE_SESSION,
                    "2026-10-03T10:01:40.000Z",
                    "session search index: title=fix tests workspace_root=/srv/proj "
                    "prompt=fix tests document_count=5 indexed_at=2026-10-03T10:02:00.000Z",
                ),
                # A plain-string prompt: the same text as the transcript's row.
                (1, "user", CLINE_SESSION, "", "fix tests"),
                # Block arrays keep their type names and tool ids.
                (2, "assistant", CLINE_SESSION, "", "thinking\ntool_use\ncall_1\nbash\npytest -q"),
                (3, "user", CLINE_SESSION, "", "tool_result\ncall_1\nbash\n3 passed"),
                (4, "assistant", CLINE_SESSION, "", "text\nFixed."),
                (
                    0,
                    "system",
                    CLINE_OLD_SESSION,
                    "2026-10-02T23:00:00.000Z",
                    "session search index: title=an older deleted session "
                    "workspace_root=/srv/proj prompt=an older deleted session "
                    "document_count=3 indexed_at=2026-10-03T10:02:00.000Z",
                ),
                # The transcript is gone; the index still holds its turns.
                (1, "user", CLINE_OLD_SESSION, "", "prune the stale branches"),
                (2, "assistant", CLINE_OLD_SESSION, "", "text\nDeleted two branches."),
            ],
        )
        self.assertEqual(
            {(row.project_path, row.model, row.agent) for row in rows}, {("/srv/proj", "", "cline")}
        )
        # The session id and project the transcript reader gives the same session.
        sdk = self.rows_for(ClineParser(), self.SDK + ".messages.json")
        self.assertEqual(
            {(row.session_id, row.project_path) for row in sdk},
            {(row.session_id, row.project_path) for row in rows if row.session_id == CLINE_SESSION},
        )
        self.assertIn(("user", "fix tests"), [(row.turn_type, row.text) for row in sdk])

    def test_session_search_project_from_manifest(self):
        manifest = self.home / (self.SDK + ".json")
        data = json.loads(manifest.read_text(encoding="utf-8"))
        data["cwd"] = "/srv/proj/sub"
        manifest.write_text(json.dumps(data), encoding="utf-8")
        rows = self.rows_for(ClineParser(), self.SEARCH)
        got = {(row.session_id, row.project_path) for row in rows}
        self.assertEqual(got, {(CLINE_SESSION, "/srv/proj/sub"), (CLINE_OLD_SESSION, "/srv/proj")})

    def test_session_search_without_fts5(self):
        # A SQLite build without the module: the virtual table cannot be read.
        path = self.home / self.SEARCH
        connection = sqlite3.connect(str(path))
        connection.execute("PRAGMA writable_schema = ON")
        connection.execute(
            "UPDATE sqlite_master SET sql = replace(sql, 'fts5(', 'nofts(') "
            "WHERE name = 'session_search'"
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(ClineParser(), self.SEARCH)
        self.assertEqual(rows[0].turn_type, "system")
        self.assertIn("parser: session_search unreadable (no such module: nofts)", rows[0].text)
        self.assertEqual(
            [(row.session_id, row.timestamp_utc, row.text) for row in rows[1:]],
            [
                (
                    CLINE_SESSION,
                    "",
                    "session search index: title=fix tests document_count=5 "
                    "indexed_at=2026-10-03T10:02:00.000Z",
                ),
                (
                    CLINE_OLD_SESSION,
                    "",
                    "session search index: title=an older deleted session document_count=3 "
                    "indexed_at=2026-10-03T10:02:00.000Z",
                ),
            ],
        )

    def test_session_search_damaged(self):
        path = self.home / self.SEARCH
        path.write_bytes(path.read_bytes()[:16] + b"\0" * 84 + b"cut mid-write")
        rows = self.rows_for(ClineParser(), self.SEARCH)
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0].turn_type, "system")
        self.assertIn("parser: session-search.db unreadable", rows[0].text)
        path.write_text("not a database", encoding="utf-8")
        rows = self.rows_for(ClineParser(), self.SEARCH)
        self.assertEqual(
            [row.text for row in rows], ["parser: session-search.db is not a SQLite database"]
        )

    def test_config_and_secrets_not_wanted(self):
        parser = ClineParser()
        for rel in (
            ".cline/data/secrets.json",
            self.SDK + ".compaction.json",
            ".cline/data/db/cron.db",
            ".cline/data/cache/openrouter_models.json",
        ):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertFalse(parser.wants(art), rel)

    def test_wants_any_editor_prefix(self):
        parser = ClineParser()
        art = next(artifact for artifact in self.col.artifacts if artifact.rel == self.UI_MESSAGES)
        for prefix in (
            "Library/Application Support/Cursor/User/globalStorage/",
            "AppData/Roaming/Code - Insiders/User/globalStorage/",
            ".vscode-server/data/User/globalStorage/",
        ):
            art.rel = prefix + "saoudrizwan.claude-dev/tasks/1/ui_messages.json"
            self.assertTrue(parser.wants(art), art.rel)
        art.rel = ".config/Code/User/globalStorage/other.ext/tasks/1/ui_messages.json"
        self.assertFalse(parser.wants(art))

    def test_truncated_line_is_reported_not_fatal(self):
        write_bad_line(self.home / self.UI_MESSAGES)
        rows = self.rows_for(ClineParser(), self.UI_MESSAGES)
        self.assertEqual(len(rows), 9)
        self.assertEqual(rows[-1].turn_type, "system")
        self.assertIn("after the JSON array", rows[-1].text)

    def test_array_cut_mid_write_keeps_earlier_records(self):
        path = self.home / self.UI_MESSAGES
        data = path.read_text(encoding="utf-8")
        cut = data[: data.index('"say": "command_output"') - 30]
        path.write_text(cut, encoding="utf-8")
        rows = self.rows_for(ClineParser(), self.UI_MESSAGES)
        self.assertEqual(
            [row.turn_type for row in rows],
            ["user", "system", "tool_use", "assistant", "tool_use", "system"],
        )
        self.assertEqual(rows[-1].source_line, 7)
        # The cut record is carried from its first character to the end.
        start = cut.rindex('{"ts"')
        try:
            json.JSONDecoder().raw_decode(cut, start)
        except ValueError as error:
            reason = str(error)
        else:
            raise AssertionError("cut record parses")
        self.assertEqual(
            rows[-1].text, "parser: unparseable record 7 (%s): %s" % (reason, cut[start:])
        )
        sdk = self.home / (self.SDK + ".messages.json")
        data = sdk.read_text(encoding="utf-8")
        sdk.write_text(data[: data.index('"id": "m3"') - 2], encoding="utf-8")
        rows = self.rows_for(ClineParser(), self.SDK + ".messages.json")
        self.assertEqual([row.turn_type for row in rows], ["user", "tool_use", "system"])
        self.assertEqual(rows[0].timestamp_utc, "2026-10-03T10:01:40.000Z")

    def test_sessions_summary(self):
        rows = []
        for parser in (ClineParser(), RooCodeParser()):
            for artifact in self.col.artifacts_for(parser.agent):
                if parser.wants(artifact):
                    rows.extend(parser.parse(artifact, Options()))
        sessions = {session.session_id: session for session in summarise(rows)}
        self.assertEqual(
            set(sessions),
            {
                CLINE_TASK,
                CLINE_CLI_TASK,
                CLINE_SESSION,
                CLINE_OLD_SESSION,
                ROO_TASK,
                ROO_CLI_TASK,
                ROO_GONE_TASK,
            },
        )
        session = sessions[CLINE_TASK]
        # task_metadata.json's model_usage records the later switch to Opus
        self.assertEqual(
            (session.models, session.user_turns, session.assistant_turns, session.tool_calls),
            ({"claude-sonnet-4-5": None, "claude-opus-4-5": None}, 1, 2, 2),
        )
        self.assertTrue(session.source_file.endswith("ui_messages.json"), session.source_file)
        self.assertEqual(sessions[CLINE_SESSION].first_timestamp_utc, "2026-10-03T10:01:40.000Z")
        self.assertEqual(sessions[ROO_TASK].project_path, "/srv/proj")

    def test_json_array_reader(self):
        path = self.tmp / "a.json"
        for text, items, nerr in (
            ("[]", [], 0),
            ('[{"a":1}, {"b":2}]', [{"a": 1}, {"b": 2}], 0),
            ('[{"a":1}, {"b":', [{"a": 1}], 1),
            ('{"a":1}', [], 1),
        ):
            path.write_text(text, encoding="utf-8")
            errors = []
            self.assertEqual([value for _, value in iter_json_array(path, errors)], items, text)
            self.assertEqual(len(errors), nerr, text)

    BLOB_REL = ".cline/data/sessions/%s/%s.json" % (CLINE_BLOB_SESSION, CLINE_BLOB_SESSION)

    def _with_blob_upload(self) -> None:
        build_cline_blob_upload(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")

    def test_enterprise_blob_upload_from_manifest(self):
        self._with_blob_upload()
        # `enterprise.blobUpload` marks a transcript uploaded to the
        # organisation's bucket: one row after the model row.
        rows = self.rows_for(ClineParser(), self.BLOB_REL)
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.project_path, row.text) for row in rows[2:]],
            [
                (
                    "2026-10-03T10:06:40.000Z",
                    CLINE_BLOB_SESSION,
                    "/srv/proj",
                    "session synced: enterprise prompt upload to s3 bucket acme-cline-prompts"
                    " object sessions/usr_0f9e8d7c/%s/messages.json endpoint=- region=us-east-1"
                    " account=- (from %s.json metadata enterprise.blobUpload, started_at)"
                    % (CLINE_BLOB_SESSION, CLINE_BLOB_SESSION),
                )
            ],
        )
        self.assertEqual(rows[1].text, "model: claude-sonnet-4-5 provider=anthropic")
        self.assertTrue(ClineParser().records_sync)

    def test_enterprise_blob_upload_from_sessions_database(self):
        self._with_blob_upload()
        # A session whose manifest is gone keeps the key in `metadata_json`;
        # the CLI's form has no `userDistinctId`, so the object is `unknown`.
        rows = self.rows_for(ClineParser(), ".cline/data/db/sessions.db")
        synced = [row for row in rows if row.text.startswith("session synced:")]
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.source_line, row.text) for row in synced],
            [
                (
                    "2026-10-03T10:05:00.000Z",
                    CLINE_BLOB_DELETED,
                    3,
                    "session synced: enterprise prompt upload to r2 bucket acme-r2 object"
                    " sessions/unknown/%s/messages.json"
                    " endpoint=https://acct.r2.cloudflarestorage.com region=- account=acct"
                    " (from sessions.db sessions.metadata_json enterprise.blobUpload, started_at)"
                    % CLINE_BLOB_DELETED,
                )
            ],
        )
        # The sessions.db row of a session whose manifest is on disk adds
        # nothing, whatever its metadata holds.
        self.assertNotIn(CLINE_BLOB_SESSION, {row.session_id for row in rows})

    def test_no_blob_upload_no_row(self):
        self._with_blob_upload()
        rel = ".cline/data/sessions/%s/%s.json" % (CLINE_SESSION, CLINE_SESSION)
        rows = self.rows_for(ClineParser(), rel)
        self.assertFalse([row for row in rows if row.text.startswith("session synced:")])
        sessions = summarise(rows, frozenset({"cline"}))
        self.assertEqual(
            [(session.session_id, session.synced) for session in sessions], [(CLINE_SESSION, "no")]
        )
        # A key that is not an object yields no row either.
        path = self.home / self.BLOB_REL
        doc = json.loads(path.read_text(encoding="utf-8"))
        doc["metadata"]["enterprise.blobUpload"] = "s3"
        path.write_text(json.dumps(doc), encoding="utf-8")
        rows = self.rows_for(ClineParser(), self.BLOB_REL)
        self.assertFalse([row for row in rows if row.text.startswith("session synced:")])

    QUEUED = (
        "enterprise prompt upload %s key %s retries %s"
        " (from saoudrizwan.claude-dev/cache/sync-queue.json, item timestamp)"
    )

    def test_sync_queue(self):
        # The extension's upload queue names each task file it uploaded or
        # failed to; the file content in `data` is never printed.
        self.assertFalse(
            [artifact for artifact in self.col.artifacts if artifact.rel == CLINE_SYNC_QUEUE]
        )
        build_cline_sync_queue(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        parser = ClineParser()
        self.assertTrue(parser.wants(self.artifact(CLINE_SYNC_QUEUE)))
        self.assertFalse(parser.secondary(self.artifact(CLINE_SYNC_QUEUE)))
        rows = self.rows_for(parser, CLINE_SYNC_QUEUE)
        self.assertEqual(
            [
                (row.timestamp_utc, row.session_id, row.project_path, row.source_line, row.text)
                for row in rows
            ],
            [
                (
                    "2026-10-03T10:00:05.000Z",
                    CLINE_TASK,
                    "/srv/proj",
                    1,
                    "session synced: "
                    + self.QUEUED % ("synced", "api_conversation_history.json", "0"),
                ),
                # Not uploaded yet: no `session synced:` prefix, no project
                # (the task is known only from the queue).
                (
                    "2026-10-03T10:00:05.500Z",
                    CLINE_QUEUE_PENDING,
                    "",
                    2,
                    self.QUEUED % ("pending", "ui_messages.json", "0"),
                ),
                (
                    "2026-10-03T10:00:06.000Z",
                    CLINE_QUEUE_FAILED,
                    "",
                    3,
                    self.QUEUED % ("failed", "ui_messages.json", "3"),
                ),
            ],
        )
        self.assertFalse(
            [row for row in rows if CLINE_SYNC_SECRET in row.text or "403" in row.text]
        )
        synced = {
            session.session_id: session.synced for session in summarise(rows, frozenset({"cline"}))
        }
        self.assertEqual(
            synced, {CLINE_TASK: "yes", CLINE_QUEUE_PENDING: "no", CLINE_QUEUE_FAILED: "no"}
        )

    def test_sync_queue_not_an_object(self):
        build_cline_sync_queue(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        path = self.home / CLINE_SYNC_QUEUE
        for text in ('{"items": {"a": ', "[]", '{"items": []}'):
            path.write_text(text, encoding="utf-8")
            self.assertEqual(
                [
                    (row.source_line, row.text)
                    for row in self.rows_for(ClineParser(), CLINE_SYNC_QUEUE)
                ],
                [(0, "parser: sync-queue.json is not a JSON object with an items object")],
                text,
            )
        path.write_text(json.dumps({"items": {"x": "pending"}}), encoding="utf-8")
        self.assertEqual(
            [row.text for row in self.rows_for(ClineParser(), CLINE_SYNC_QUEUE)],
            ["parser: sync-queue item x is not a JSON object"],
        )

    # -- source-derived rows

    def _reopen(self, *builders) -> None:
        for build in builders:
            build(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")

    def test_hooks_log_rows(self):
        self._reopen(build_cline_hooks_log)
        parser = ClineParser()
        self.assertTrue(parser.wants(self.artifact(CLINE_HOOKS_LOG)))
        self.assertTrue(parser.secondary(self.artifact(CLINE_HOOKS_LOG)))
        self.assertEqual(parser.secondary_tolerance(self.artifact(CLINE_HOOKS_LOG)), 3600.0)
        self.assertEqual(parser.secondary_tolerance(self.artifact(self.INPUT_HISTORY)), 2.0)
        rows = self.rows_for(parser, CLINE_HOOKS_LOG)
        session, old = CLINE_SESSION, CLINE_OLD_SESSION
        self.assertEqual(
            [
                (
                    row.source_line,
                    row.turn_type,
                    row.session_id,
                    row.tool_name,
                    row.tool_use_id,
                    row.text,
                )
                for row in rows
            ],
            [
                (
                    1,
                    "system",
                    session,
                    "",
                    "",
                    "hook audit: agent_start cline_version=4.1.25 user=alice agent=agent_lead",
                ),
                (2, "user", session, "", "", "fix tests"),
                (3, "tool_use", session, "bash", "call_1", "pytest -q"),
                (4, "tool_result", session, "bash", "call_1", "3 passed"),
                (5, "assistant", session, "", "", "Fixed."),
                (6, "tool_use", old, "bash", "call_9", "git branch -D stale"),
                (7, "tool_result", old, "bash", "call_9", "[error] exit code 1"),
                (
                    8,
                    "system",
                    old,
                    "",
                    "",
                    "hook audit: agent_abort cline_version=4.1.25 user=alice agent=agent_lead"
                    " reason=aborted by user",
                ),
                (
                    9,
                    "system",
                    old,
                    "",
                    "",
                    "hook audit: session_shutdown reason=stale_process pid=4100 source=cli",
                ),
            ],
        )
        self.assertEqual(rows[2].timestamp_utc, "2026-10-03T10:01:42.500Z")
        self.assertEqual({row.project_path for row in rows[:8]}, {"/srv/proj"})
        self.assertEqual(rows[8].project_path, "")

    def test_hooks_log_dedup_keeps_what_the_transcripts_lost(self):
        self._reopen(build_cline_hooks_log)
        parser = ClineParser()
        primary = self.rows_for(parser, self.SDK + ".messages.json")
        audit = self.rows_for(parser, CLINE_HOOKS_LOG)
        tolerance = parser.secondary_tolerance(self.artifact(CLINE_HOOKS_LOG))
        kept, dropped = deduplicate(primary, [(row, tolerance) for row in audit])
        self.assertEqual(dropped["cline"], 4)
        survivors = kept[len(primary) :]
        self.assertEqual(
            [(row.session_id, row.turn_type, row.recovered) for row in survivors],
            [
                (CLINE_SESSION, "system", ""),
                (CLINE_OLD_SESSION, "tool_use", "yes"),
                (CLINE_OLD_SESSION, "tool_result", "yes"),
                (CLINE_OLD_SESSION, "system", ""),
                (CLINE_OLD_SESSION, "system", ""),
            ],
        )

    def test_hooks_log_truncated_line(self):
        self._reopen(build_cline_hooks_log)
        path = self.home / CLINE_HOOKS_LOG
        first, rest = path.read_bytes().split(b"\n", 1)
        path.write_bytes(first + b"\n" + BAD_LINE.encode() + b"\n" + rest)
        write_bad_line(path)
        rows = self.rows_for(ClineParser(), CLINE_HOOKS_LOG)
        self.assertEqual(len(rows), 11)
        self.assertEqual((rows[0].source_line, rows[1].source_line), (1, 3))
        self.assertEqual(rows[1].text, "fix tests")
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[-2:]],
            [(2, "system", bad_line_text()), (11, "system", bad_line_text())],
        )

    def test_standing_permission_settings(self):
        self._reopen(build_cline_settings_and_cloud)
        parser = ClineParser()
        mtime = "2026-10-03T10:10:00.000Z"

        def settings(rel):
            rows = self.rows_for(parser, rel)
            got = [row for row in rows if row.text.startswith("permission setting:")]
            for row in got:
                self.assertEqual(
                    (row.turn_type, row.timestamp_utc, row.session_id, row.project_path),
                    ("system", mtime, "", ""),
                )
                self.assertTrue(row.text.endswith("(from %s, file mtime)" % rel), row.text)
            return [row.text.split(" (from ")[0] for row in got]

        self.assertEqual(
            settings(CLINE_GLOBAL_STATE),
            [
                "permission setting: autoApprovalSettings.enabled=true",
                "permission setting: autoApprovalSettings.maxRequests=20",
                "permission setting: autoApprovalSettings.actions.readFiles=true",
                "permission setting: autoApprovalSettings.actions.editFiles=true",
                "permission setting: autoApprovalSettings.actions.executeSafeCommands=false",
                "permission setting: autoApprovalSettings.actions.executeAllCommands=true",
                "permission setting: autoApprovalSettings.actions.useBrowser=false",
                "permission setting: autoApprovalSettings.actions.useMcp=true",
                "permission setting: mode=act",
            ],
        )
        self.assertEqual(
            settings(CLINE_GLOBAL_SETTINGS),
            [
                "permission setting: toolAutoApprove=false",
                "permission setting: planActMode=plan",
                "permission setting: disabledTools=browser_action",
                "permission setting: disabledPlugins=acme-plugin",
                "permission setting: tools.web_fetch.enabled=false",
            ],
        )
        self.assertEqual(
            settings(CLINE_MCP_SETTINGS),
            [
                "permission setting: mcpServers.github.autoApprove=list_issues",
                "permission setting: mcpServers.github.autoApprove=get_issue",
                "permission setting: mcpServers.github.disabled=false",
            ],
        )
        everything = " ".join(
            row.text
            for rel in (CLINE_GLOBAL_STATE, CLINE_GLOBAL_SETTINGS, CLINE_MCP_SETTINGS)
            for row in self.rows_for(parser, rel)
        )
        for secret in ("ghp_not_real", "npx", "apiProvider", "telemetryOptOut"):
            self.assertNotIn(secret, everything)
        # The extension's older copy under globalStorage is read the same way.
        self.assertTrue(parser.wants(self.artifact(CLINE_GS + "settings/cline_mcp_settings.json")))
        self.assertEqual(self.rows_for(parser, CLINE_GS + "settings/cline_mcp_settings.json"), [])

    def test_cloud_origin_and_sync_rows(self):
        self._reopen(build_cline_settings_and_cloud)
        parser = ClineParser()
        self.assertTrue(parser.records_origin)
        state = [
            row
            for row in self.rows_for(parser, CLINE_GLOBAL_STATE)
            if not row.text.startswith("permission")
        ]
        source = "(from %s cloudSessionStatuses, observedAt)" % CLINE_GLOBAL_STATE
        self.assertEqual(
            [(row.turn_type, row.timestamp_utc, row.session_id, row.text) for row in state],
            [
                (
                    "system",
                    "2026-10-03T10:10:00.000Z",
                    "",
                    "cloud task target: target=cloud repository_id=812345678 branch=main"
                    " (from %s, file mtime)" % CLINE_GLOBAL_STATE,
                ),
                (
                    "system",
                    "2026-10-03T10:03:20.000Z",
                    CLINE_CLOUD_SESSION,
                    "session origin: remote cline-cloud session %s status completed %s"
                    % (CLINE_CLOUD_SESSION, source),
                ),
                (
                    "system",
                    "2026-10-03T10:03:20.000Z",
                    CLINE_CLOUD_SESSION,
                    "session synced: cline-cloud holds session %s, no local transcript %s"
                    % (CLINE_CLOUD_SESSION, source),
                ),
            ],
        )
        self.assertTrue(parser.wants(self.artifact(CLINE_PENDING_START)))
        pending = self.rows_for(parser, CLINE_PENDING_START)
        source = "(from %s, file mtime)" % CLINE_PENDING_START
        self.assertEqual(
            [(row.source_line, row.timestamp_utc, row.session_id, row.text) for row in pending],
            [
                (
                    1,
                    "2026-10-03T10:10:00.000Z",
                    CLINE_PENDING_SESSION,
                    "session origin: remote cline-cloud session %s sandbox start pending"
                    " account=user:usr_0f9e8d7c endpoint=https://api.cline.bot %s"
                    % (CLINE_PENDING_SESSION, source),
                ),
                (
                    1,
                    "2026-10-03T10:10:00.000Z",
                    CLINE_PENDING_SESSION,
                    "session synced: cline-cloud holds session %s, no local transcript %s"
                    % (CLINE_PENDING_SESSION, source),
                ),
            ],
        )
        local = self.rows_for(parser, self.SDK + ".json")
        sessions = {
            summary.session_id: (summary.origin, summary.synced)
            for summary in summarise(
                state + pending + local, frozenset({"cline"}), origin_agents=frozenset({"cline"})
            )
        }
        self.assertEqual(
            sessions,
            {
                CLINE_CLOUD_SESSION: ("remote", "yes"),
                CLINE_PENDING_SESSION: ("remote", "yes"),
                CLINE_SESSION: ("local", "no"),
            },
        )

    def test_settings_files_that_are_not_objects(self):
        self._reopen(build_cline_settings_and_cloud)
        parser = ClineParser()
        for rel, what in (
            (CLINE_GLOBAL_STATE, "globalState.json"),
            (CLINE_GLOBAL_SETTINGS, "global-settings.json"),
            (CLINE_MCP_SETTINGS, "cline_mcp_settings.json"),
        ):
            (self.home / rel).write_text("[1, 2]", encoding="utf-8")
            self.assertEqual(
                [row.text for row in self.rows_for(parser, rel)],
                ["parser: %s is not a JSON object" % what],
            )
        path = self.home / CLINE_PENDING_START
        path.write_text('[{"account": "x"}, {"sessionId": "ses-1"}]', encoding="utf-8")
        rows = self.rows_for(parser, CLINE_PENDING_START)
        self.assertEqual(
            [(row.source_line, row.session_id, row.text.split(" ")[0]) for row in rows],
            [
                (2, "ses-1", "session"),
                (2, "ses-1", "session"),
                (1, "", "parser:"),
            ],
        )
        self.assertEqual(rows[-1].text, "parser: pending-start record 1 has no sessionId")
        path.write_text("{}", encoding="utf-8")
        self.assertEqual(
            [row.text for row in self.rows_for(parser, CLINE_PENDING_START)],
            ["parser: cloud pending-start file is not a JSON array"],
        )

    def test_rejected_and_blocked_calls(self):
        self._reopen(build_cline_denied_and_imported)
        rel = ".cline/data/sessions/%s/%s.messages.json" % (
            CLINE_DENIED_SESSION,
            CLINE_DENIED_SESSION,
        )
        rows = self.rows_for(ClineParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.tool_name, row.tool_use_id) for row in rows],
            [
                ("user", "", ""),
                ("tool_use", "bash", "call_r"),
                ("tool_use", "web_fetch", "call_b"),
                ("tool_result", "bash", "call_r"),
                ("system", "bash", "call_r"),
                ("tool_result", "web_fetch", "call_b"),
                ("system", "web_fetch", "call_b"),
            ],
        )
        self.assertEqual(
            rows[4].text,
            "permission decision: rejected reason=This tool call was rejected by the user and"
            " not executed. The user provided the following feedback:\n<feedback>\nnot that"
            " one\n</feedback>",
        )
        self.assertEqual(
            rows[6].text,
            'permission decision: blocked reason=Tool "web_fetch" is disabled by policy',
        )
        self.assertEqual(
            {(row.timestamp_utc, row.session_id, row.model) for row in rows[3:]},
            {("2026-10-03T10:08:22.000Z", CLINE_DENIED_SESSION, "claude-sonnet-4-5")},
        )
        # A successful or merely failed call has no decision row.
        plain = self.rows_for(ClineParser(), self.SDK + ".messages.json")
        self.assertFalse([row for row in plain if row.text.startswith("permission decision:")])

    def test_imported_session_names_its_source(self):
        self._reopen(build_cline_denied_and_imported)
        rel = ".cline/data/sessions/%s/%s.json" % (CLINE_IMPORTED_SESSION, CLINE_IMPORTED_SESSION)
        start = self.rows_for(ClineParser(), rel)[0]
        self.assertEqual(start.session_id, CLINE_IMPORTED_SESSION)
        self.assertTrue(start.text.startswith("session start: source=desktop"), start.text)
        self.assertIn(
            " prompt=port the parser imported_from=claude-code"
            " imported_session=5f0c1d2e-0000-4000-8000-00000000c1a0"
            " imported_path=/home/u/.claude/projects/-srv-proj/5f0c1d2e.jsonl"
            " imported_at=2026-10-03T10:11:45.000Z imported_provider=anthropic"
            " imported_model=claude-opus-4-5",
            start.text,
        )
        native = self.rows_for(ClineParser(), self.SDK + ".json")[0]
        self.assertNotIn("imported_", native.text)
