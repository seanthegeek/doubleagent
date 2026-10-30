"""Tests of the Antigravity parser."""

import json
from typing import cast

from doubleagent import protobuf
from doubleagent.dedup import deduplicate
from doubleagent.inputs import Artifact
from doubleagent.parsers.antigravity import (
    AntigravityParser,
    file_mtime,
    permission_texts,
    tool_args_summary,
    uri_to_path,
)
from doubleagent.timeutil import epoch_ms, to_utc
from fixtures import (
    AGY,
    AGY_APP_CONVERSATION,
    AGY_APP_PROJECT,
    AGY_CONFIG_MTIME,
    AGY_CONVERSATION,
    AGY_ORPHAN,
    AGY_SIDECAR,
    bad_line_text,
    write_bad_line,
)

from .base import ParserBase


class AntigravityTests(ParserBase):
    """Test the Antigravity CLI parser."""

    REL = ".gemini/antigravity-cli/conversations/%s.db" % AGY_CONVERSATION

    def test_secondary(self):
        # The history file is a secondary source, the transcript is not.
        parser = AntigravityParser()
        self.assertTrue(parser.secondary(self.artifact(".gemini/antigravity-cli/history.jsonl")))
        self.assertFalse(parser.secondary(self.artifact(self.REL)))

    def test_rows(self):
        rows = self.rows_for(AntigravityParser(), self.REL)
        types = [row.turn_type for row in rows]
        self.assertEqual(
            types,
            [
                "system",
                "user",
                "system",
                "assistant",
                "tool_use",
                "tool_result",
                "tool_use",
                "tool_result",
                "tool_use",
                "tool_result",
                "system",
                "assistant",
                "system",
                # The model and decision steps, checked in their own tests.
                "system",
                "tool_use",
                "tool_result",
                "system",
                "tool_use",
                "tool_result",
                "system",
                "tool_use",
                "tool_result",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "antigravity", AGY_CONVERSATION),
            )
            self.assertEqual((row.project_path, row.git_branch), ("/home/u/proj", "main"))
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        start, user, _model, asst, use1, res1, use2, res2, use3, res3, injected, final, ckpt = rows[
            :13
        ]
        self.assertIn("session start", start.text)
        self.assertIn("project_id=default-cli-project", start.text)
        self.assertEqual(user.text, "clean the logs")
        self.assertEqual(user.timestamp_utc, "2026-10-01T10:00:00.000Z")
        self.assertEqual(
            (asst.text, asst.model), ("I will read the README first.", "gemini-3.8-flash")
        )
        self.assertEqual(
            (use1.tool_name, use1.tool_use_id, use1.text),
            ("view_file", "call_1", "/home/u/proj/README.md"),
        )
        self.assertEqual((res1.tool_name, res1.tool_use_id), ("view_file", "call_1"))
        self.assertTrue(res1.text.startswith("File Path: README.md"))
        self.assertEqual(
            res1.timestamp_utc, "2026-10-01T10:00:04.000Z"
        )  # completed_at, not created_at
        self.assertEqual((use2.tool_name, use2.text), ("run_command", "rm -rf /var/log/*.log"))
        self.assertEqual(res2.text, "rm -rf /var/log/*.log | exit=0 | removed 3 files")
        self.assertEqual(use3.text, "/home/u/proj/notes.md")
        self.assertEqual(res3.text, "[error] /home/u/proj/notes.md [created]")
        self.assertEqual(injected.turn_type, "system")  # USER_IMPLICIT source
        self.assertEqual(final.text, "Done. Three log files were removed.")
        self.assertEqual(ckpt.text, "checkpoint: Clean logs")
        self.assertEqual(
            [row.source_line for row in rows],
            [0, 0, 1, 1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 10, 11, 11, 12, 13, 13, 14, 15],
        )

    def test_model_rows(self):
        # gen_metadata names gemini-3.8-flash for steps 1-8 and
        # gemini-3.8-pro from step 10: a start row before the first
        # planner step's rows and a change row before step 10's, each with
        # the step's created_at and the new id in the model column.
        rows = self.rows_for(AntigravityParser(), self.REL)
        models = [row for row in rows if row.text.startswith("model:")]
        self.assertEqual(
            [
                (row.turn_type, row.text, row.timestamp_utc, row.source_line, row.model)
                for row in models
            ],
            [
                (
                    "system",
                    "model: gemini-3.8-flash",
                    "2026-10-01T10:00:01.000Z",
                    1,
                    "gemini-3.8-flash",
                ),
                (
                    "system",
                    "model: gemini-3.8-flash -> gemini-3.8-pro",
                    "2026-10-01T10:00:14.000Z",
                    10,
                    "gemini-3.8-pro",
                ),
            ],
        )
        for model_row in models:
            i = rows.index(model_row)
            self.assertEqual(rows[i + 1].source_line, model_row.source_line)
            self.assertEqual(rows[i + 1].model, model_row.model)
        self.assertEqual(rows[3].text, "I will read the README first.")

    def test_rejection_decision_rows(self):
        # run_command.user_rejected and mcp_tool.user_rejected: a
        # `permission decision:` row right after the tool_result, which
        # keeps its [rejected by user] marker.
        rows = self.rows_for(AntigravityParser(), self.REL)
        decisions = [i for i, row in enumerate(rows) if row.text.startswith("permission decision:")]
        self.assertEqual(len(decisions), 2)
        expected = [
            ("run_command", "call_4", "make test | [rejected by user]", "2026-10-01T10:00:16.000Z"),
            (
                "create_issue",
                "call_5",
                "github/create_issue | [rejected by user]",
                "2026-10-01T10:00:19.000Z",
            ),
        ]
        for i, (tool, call, tool_result_text, timestamp) in zip(decisions, expected, strict=True):
            result, dec = rows[i - 1], rows[i]
            self.assertEqual(
                (result.turn_type, result.tool_name, result.tool_use_id, result.text),
                ("tool_result", tool, call, tool_result_text),
            )
            self.assertEqual(
                (dec.turn_type, dec.text, dec.tool_name, dec.tool_use_id, dec.timestamp_utc),
                ("system", "permission decision: user_rejected", tool, call, timestamp),
            )
            self.assertEqual(dec.source_line, result.source_line)

    def test_sandbox_flag_on_result(self):
        # run_command.ran_in_sandbox is appended to the result text;
        # it is not a decision, so no row of its own.
        rows = self.rows_for(AntigravityParser(), self.REL)
        result = rows[-1]
        self.assertEqual(
            (result.turn_type, result.tool_use_id, result.text),
            ("tool_result", "call_6", "ls | exit=0 | README.md sandbox=true"),
        )
        self.assertNotIn("sandbox=true", " ".join(row.text for row in rows[:-1]))

    def test_thinking_opt_in(self):
        rows = self.rows_for(AntigravityParser(), self.REL, include_thinking=True)
        self.assertEqual(
            [row.text for row in rows if row.turn_type == "thinking"], ["look before leaping"]
        )
        # With thinking on, the model row still precedes the step's rows.
        i = next(i for i, row in enumerate(rows) if row.turn_type == "thinking")
        self.assertEqual(rows[i - 1].text, "model: gemini-3.8-flash")

    def test_summaries_and_history(self):
        rows = self.rows_for(
            AntigravityParser(), ".gemini/antigravity-cli/conversation_summaries.db"
        )
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0].session_id, AGY_CONVERSATION)
        self.assertEqual(rows[0].timestamp_utc, "2026-10-01T10:00:13.002Z")
        self.assertIn("CASCADE_RUN_STATUS_IDLE", rows[0].text)
        self.assertEqual(rows[0].project_path, "/home/u/proj")
        rows = self.rows_for(AntigravityParser(), ".gemini/antigravity-cli/history.jsonl")
        self.assertEqual(
            (rows[0].turn_type, rows[0].text, rows[0].project_path),
            ("user", "clean the logs", "/home/u/proj"),
        )

    def test_history_conversation_id(self):
        # CLI 1.3.x writes conversationId on some history lines.
        rows = self.rows_for(AntigravityParser(), ".gemini/antigravity-cli/history.jsonl")
        self.assertEqual([row.session_id for row in rows], ["", AGY_CONVERSATION])
        self.assertEqual(rows[1].text, "now the tests")

    def test_history_reports_truncated_line_and_keeps_valid_rows(self):
        # A bad history line is a system row, not silence.
        rel = ".gemini/antigravity-cli/history.jsonl"
        (self.home / rel).write_text(
            '{"display":"keep before","conversationId":"c1"}\n'
            '{"display":\n'
            '{"display":"keep after","conversationId":"c2"}\n',
            encoding="utf-8",
        )
        rows = self.rows_for(AntigravityParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.source_line, row.text) for row in rows],
            [
                ("user", 1, "keep before"),
                ("user", 3, "keep after"),
                ("system", 2, bad_line_text('{"display":')),
            ],
        )

    def test_sidecars_and_token_not_wanted(self):
        parser = AntigravityParser()
        for artifact in self.col.artifacts:
            if artifact.rel.endswith((".db-wal", ".db-shm", "antigravity-oauth-token")):
                self.assertFalse(parser.wants(artifact), artifact.rel)

    def test_session_start_names_cli_source(self):
        start = self.rows_for(AntigravityParser(), self.REL)[0]
        self.assertEqual(
            start.text,
            "session start | source=cli | data_dir=antigravity-cli | project_id=default-cli-project",
        )

    def test_helpers(self):
        self.assertEqual(uri_to_path("file:///home/u/proj"), "/home/u/proj")
        self.assertEqual(uri_to_path("file:///c%3A/Users/u/proj"), "c:/Users/u/proj")
        self.assertEqual(uri_to_path("/plain/path"), "/plain/path")
        self.assertEqual(tool_args_summary('{"CommandLine":"ls","Cwd":"/x"}'), "ls")
        self.assertEqual(tool_args_summary('{"Other":1}'), '{"Other":1}')
        self.assertEqual(tool_args_summary("not json"), "not json")


class AntigravityAppTests(ParserBase):
    """Test the desktop app's conversations under ~/.gemini/antigravity."""

    BASE = ".gemini/antigravity/"
    REL = BASE + "conversations/%s.db" % AGY_APP_CONVERSATION

    def test_rows(self):
        rows = self.rows_for(AntigravityParser(), self.REL)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "user",
                "system",
                "assistant",
                "tool_use",
                "tool_result",
                "system",
                "system",
                "system",
                "system",
                "assistant",
                "tool_use",
                "tool_result",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.agent, row.session_id, row.project_path),
                ("antigravity", AGY_APP_CONVERSATION, "/home/u/proj"),
            )
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        (
            start,
            user,
            model,
            asst,
            use,
            result,
            _grant,
            _files,
            sysmsg,
            error,
            final,
            del_use,
            delete_result,
        ) = rows
        # One model for the whole conversation: a start row only.
        self.assertEqual(
            (model.text, model.timestamp_utc, model.model),
            ("model: gemini-3.8-pro", "2026-10-01T11:00:01.000Z", "gemini-3.8-pro"),
        )
        self.assertEqual(
            start.text,
            "session start | source=cascade_client | data_dir=antigravity"
            " | project_id=%s" % AGY_APP_PROJECT,
        )
        self.assertEqual(start.timestamp_utc, "2026-10-01T10:59:59.000Z")
        self.assertEqual(
            (user.text, user.timestamp_utc),
            ("which packages are outdated?", "2026-10-01T11:00:00.000Z"),
        )
        self.assertEqual((asst.text, asst.model), ("I will ask Homebrew.", "gemini-3.8-pro"))
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text), ("run_command", "call_7", "brew outdated")
        )
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.text),
            ("run_command", "call_7", "node (24.1.0) < 24.2.0"),
        )
        self.assertEqual(sysmsg.text, "system_message: background task finished")
        self.assertEqual(error.text, "error: model overloaded")
        self.assertEqual(
            (final.text, final.model), ("One package is outdated: node.", "gemini-3.8-pro")
        )
        # A delete_file code action: the path is in ActionSpecDeleteFile
        # field 1.
        self.assertEqual(
            (del_use.tool_name, del_use.tool_use_id, del_use.text),
            ("delete_file", "call_8", "/home/u/proj/old.log"),
        )
        self.assertEqual(
            (delete_result.tool_name, delete_result.tool_use_id, delete_result.text),
            ("delete_file", "call_8", "delete_file | /home/u/proj/old.log | remove the stale log"),
        )
        self.assertEqual([row.source_line for row in rows], [0, 0, 1, 1, 1, 2, 2, 2, 3, 4, 5, 6, 7])

    def test_permission_grant_rows(self):
        # steps.permissions on the run_command step: one row per
        # permission_grants entry and per file_access_permissions entry,
        # after the step's tool_result, keyed to its call.
        rows = self.rows_for(AntigravityParser(), self.REL)
        result, grant, files = rows[5:8]
        self.assertEqual(result.turn_type, "tool_result")
        for row in (grant, files):
            self.assertEqual(
                (row.turn_type, row.tool_name, row.tool_use_id, row.timestamp_utc, row.source_line),
                ("system", "run_command", "call_7", result.timestamp_utc, 2),
            )
        self.assertEqual(
            grant.text,
            "permission grant: action=run_command target=brew outdated allow=true"
            " scope=conversation",
        )
        self.assertEqual(
            files.text,
            "permission grant: path=/home/u/proj is_directory=true allow=true scope=workspace",
        )

    def test_permissions_from_payload_when_column_empty(self):
        # Step field 133 is read when the steps.permissions column is empty.
        import sqlite3

        step = {
            "metadata": {
                "created_at": 1790860000,
                "completed_at": 1790860001,
                "tool_call": {"id": "call_9", "name": "run_command"},
            },
            "run_command": {"command_line": "ls"},
            "permissions": {
                "permission_grants": [
                    {
                        "resource": {"action": "run_command", "target": "ls"},
                        "user_deny_instruction": "not in my home",
                    }
                ]
            },
        }
        connection = sqlite3.connect(str(self.home / self.REL))
        connection.execute(
            "INSERT INTO steps (idx, step_type, status, step_payload) VALUES (50, 21, 3, ?)",
            (protobuf.encode(step, "Step", AGY),),
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(AntigravityParser(), self.REL)
        self.assertEqual(
            [(row.turn_type, row.tool_use_id) for row in rows[-2:]],
            [("tool_result", "call_9"), ("system", "call_9")],
        )
        self.assertEqual(
            rows[-1].text,
            "permission grant: action=run_command target=ls allow=false scope=unspecified"
            " deny_instruction=not in my home",
        )
        self.assertEqual(rows[-1].timestamp_utc, "2026-10-01T13:06:41.000Z")

    def test_bad_permissions_blob_is_reported(self):
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.REL))
        connection.execute("UPDATE steps SET permissions = ? WHERE idx = 2", (b"\x12\x05ab",))
        connection.commit()
        connection.close()
        rows = self.rows_for(AntigravityParser(), self.REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows if row.text.startswith("parser:")],
            [("system", "parser: the permissions of step 2 could not be decoded")],
        )
        self.assertNotIn("permission grant", " ".join(row.text for row in rows))

    def test_permission_texts(self):
        self.assertEqual(
            permission_texts(
                {
                    "permission_grants": [
                        {"allow": True, "scope": 9, "action": "a", "target": "t"}
                    ],
                    "file_access_permissions": [
                        {"path": "file:///home/u/x", "scope": 1, "from_current_step": True}
                    ],
                }
            ),
            [
                "permission grant: action=a target=t allow=true scope=9",
                "permission grant: path=/home/u/x is_directory=false allow=false scope=once"
                " from_current_step=true",
            ],
        )

    def test_delete_file_path_is_field_1(self):
        # The fixture is encoded with the parser's own SCHEMA, so check the
        # wire layout from the descriptors by hand: ActionSpec field 4 is
        # ActionSpecDeleteFile, whose field 1 is the PathScopeItem.
        uri = b"file:///home/u/proj/old.log"
        scope = b"\x2a" + bytes([len(uri)]) + uri  # PathScopeItem 5 absolute_uri
        delete = b"\x0a" + bytes([len(scope)]) + scope  # ActionSpecDeleteFile 1 path
        spec = b"\x22" + bytes([len(delete)]) + delete  # ActionSpec 4 delete_file
        self.assertEqual(
            protobuf.decode(spec, "ActionSpec", AGY),
            {"delete_file": {"path": {"absolute_uri": uri.decode()}}},
        )

    def test_thinking_opt_in(self):
        rows = self.rows_for(AntigravityParser(), self.REL)
        self.assertNotIn("thinking", [row.turn_type for row in rows])
        rows = self.rows_for(AntigravityParser(), self.REL, include_thinking=True)
        self.assertEqual([row.text for row in rows if row.turn_type == "thinking"], ["brew knows"])

    def test_summaries(self):
        rows = self.rows_for(AntigravityParser(), self.BASE + "conversation_summaries.db")
        self.assertEqual(len(rows), 1)
        self.assertEqual(
            (rows[0].turn_type, rows[0].session_id, rows[0].timestamp_utc),
            ("system", AGY_APP_CONVERSATION, "2026-10-01T11:00:13.002Z"),
        )
        self.assertEqual(
            rows[0].text,
            "conversation summary | Outdated packages | CASCADE_RUN_STATUS_IDLE | 8 steps",
        )

    def test_other_data_directories_wanted(self):
        parser = AntigravityParser()

        def art(rel):
            return cast(Artifact, type("A", (), {"rel": rel})())

        for directory in (
            "antigravity-cli",
            "antigravity",
            "antigravity-ide",
            "antigravity-backup",
        ):
            self.assertTrue(
                parser.wants(art(".gemini/%s/conversations/x.db" % directory)), directory
            )
            self.assertTrue(
                parser.wants(art(".gemini/%s/conversation_summaries.db" % directory)), directory
            )
        for rel in (
            ".gemini/antigravity/history.jsonl",  # the app writes none; CLI only
            ".gemini/antigravity-other/conversations/x.db",
            ".gemini/antigravity/conversations/x.db-wal",
            # The CLI's settings only; the app's directory has none.
            ".gemini/antigravity/settings.json",
        ):
            self.assertFalse(parser.wants(art(rel)), rel)

    def test_token_not_wanted(self):
        # The plugin's OAuth token is never read; the brain transcript beside
        # it is (AntigravityBrainTests).
        parser = AntigravityParser()
        rels = {artifact.rel: artifact for artifact in self.col.artifacts}
        token = self.BASE + "plugin_data/gdocs/token.json"
        brain = (
            self.BASE + "brain/%s/.system_generated/logs/transcript.jsonl" % AGY_APP_CONVERSATION
        )
        for rel in (token, brain):
            self.assertEqual(rels[rel].agent, "antigravity", rel)
        self.assertFalse(parser.wants(rels[token]))
        self.assertTrue(parser.wants(rels[brain]))

    def test_bad_step_is_reported_not_fatal(self):
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.REL))
        connection.execute(
            "INSERT INTO steps (idx, step_type, status, step_payload) VALUES (99, 15, 3, ?)",
            (b"\xa2\x01\x05ab",),
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(AntigravityParser(), self.REL)
        self.assertEqual(len(rows), 14)
        self.assertEqual(rows[10].text, "One package is outdated: node.")
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].text),
            ("system", "parser: 1 step(s) could not be decoded"),
        )


class AntigravityBrainTests(ParserBase):
    """Test the brain mirror, a secondary source of the conversation database.

    The mirror is `brain/<uuid>/.system_generated/logs/`.
    """

    LOGS = ".gemini/antigravity-cli/brain/%s/.system_generated/logs/"
    FULL = LOGS % AGY_CONVERSATION + "transcript_full.jsonl"
    SHORT = LOGS % AGY_CONVERSATION + "transcript.jsonl"
    CHUNK = LOGS % AGY_CONVERSATION + "chunks/transcript_full/00000000.jsonl"
    ORPHAN0 = LOGS % AGY_ORPHAN + "chunks/transcript_full/00000000.jsonl"
    ORPHAN1 = LOGS % AGY_ORPHAN + "chunks/transcript_full/00000001.jsonl"
    ORPHAN_SHORT = LOGS % AGY_ORPHAN + "transcript.jsonl"
    APP = ".gemini/antigravity/brain/%s/.system_generated/logs/transcript.jsonl" % (
        AGY_APP_CONVERSATION
    )
    DATABASE = ".gemini/antigravity-cli/conversations/%s.db" % AGY_CONVERSATION
    APP_DATABASE = ".gemini/antigravity/conversations/%s.db" % AGY_APP_CONVERSATION

    @staticmethod
    def shape(rows):
        return [
            (
                row.source_line,
                row.turn_type,
                row.timestamp_utc,
                row.tool_name,
                row.tool_use_id,
                row.text,
            )
            for row in rows
        ]

    def test_secondary(self):
        parser = AntigravityParser()
        for rel in (self.FULL, self.SHORT, self.CHUNK, self.ORPHAN0, self.APP):
            self.assertTrue(parser.secondary(self.artifact(rel)), rel)
            self.assertEqual(parser.secondary_tolerance(self.artifact(rel)), 600.0)
        self.assertFalse(parser.secondary(self.artifact(self.DATABASE)))
        history = self.artifact(".gemini/antigravity-cli/history.jsonl")
        self.assertEqual(parser.secondary_tolerance(history), 2.0)

    def test_rows(self):
        rows = self.rows_for(AntigravityParser(), self.FULL)
        self.assertEqual(
            self.shape(rows),
            [
                (1, "user", "2026-10-01T10:00:00.000Z", "", "", "clean the logs"),
                (
                    2,
                    "assistant",
                    "2026-10-01T10:00:01.000Z",
                    "",
                    "",
                    "I will read the README first.",
                ),
                (
                    2,
                    "tool_use",
                    "2026-10-01T10:00:01.000Z",
                    "view_file",
                    "call_1",
                    "/home/u/proj/README.md",
                ),
                (
                    3,
                    "tool_result",
                    "2026-10-01T10:00:03.000Z",
                    "view_file",
                    "call_1",
                    "File Path: README.md\n# proj",
                ),
                (
                    4,
                    "tool_use",
                    "2026-10-01T10:00:04.000Z",
                    "run_command",
                    "call_2",
                    "rm -rf /var/log/*.log",
                ),
                (5, "system", "2026-10-01T10:00:09.000Z", "", "", "<injected reminder>"),
                (
                    6,
                    "assistant",
                    "2026-10-01T10:00:10.000Z",
                    "",
                    "",
                    "Done. Three log files were removed.",
                ),
            ],
        )
        for row in rows:
            self.assertEqual(
                (
                    row.host,
                    row.user,
                    row.agent,
                    row.session_id,
                    row.project_path,
                    row.git_branch,
                    row.model,
                ),
                ("h1", "alice", "antigravity", AGY_CONVERSATION, "", "", ""),
            )
            self.assertTrue(row.source_file.endswith(self.FULL), row.source_file)

    def test_thinking_opt_in(self):
        rows = self.rows_for(AntigravityParser(), self.FULL, include_thinking=True)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[1:3]],
            [
                (2, "thinking", "look before leaping"),
                (2, "assistant", "I will read the README first."),
            ],
        )
        self.assertEqual(rows[1].timestamp_utc, "2026-10-01T10:00:01.000Z")

    def test_other_copies_not_read(self):
        # transcript_full.jsonl is present, so its shortened twin and its
        # chunk yield nothing; the orphan's full chunks are read through the
        # first one, so the second chunk and the shortened twin yield nothing.
        for rel in (self.SHORT, self.CHUNK, self.ORPHAN1, self.ORPHAN_SHORT):
            self.assertEqual(self.rows_for(AntigravityParser(), rel), [], rel)

    def test_fallback_order_among_copies(self):
        logs = self.home / (self.LOGS % AGY_CONVERSATION)
        (logs / "transcript_full.jsonl").unlink()
        # The chunk's lines all read "(not read)", the planner line with no
        # content of its own included, so it yields one row more.
        rows = self.rows_for(AntigravityParser(), self.CHUNK)
        self.assertEqual(len(rows), 8)
        self.assertEqual(
            {row.text for row in rows},
            {"(not read)", "/home/u/proj/README.md", "rm -rf /var/log/*.log"},
        )
        self.assertEqual(self.rows_for(AntigravityParser(), self.SHORT), [])
        (logs / "chunks/transcript_full/00000000.jsonl").unlink()
        rows = self.rows_for(AntigravityParser(), self.SHORT)
        self.assertEqual(len(rows), 8)
        self.assertTrue(rows[0].source_file.endswith(self.SHORT))

    def test_chunks_reassembled(self):
        # The orphan conversation has no database: its rows come from the
        # two transcript_full chunks, the line cut at the boundary whole.
        rows = self.rows_for(AntigravityParser(), self.ORPHAN0, include_thinking=True)
        self.assertEqual(
            self.shape(rows),
            [
                (1, "user", "2026-10-01T10:30:00.000Z", "", "", "summarise the deleted project"),
                (2, "thinking", "2026-10-01T10:30:01.250Z", "", "", "it may be gone"),
                (2, "assistant", "2026-10-01T10:30:01.250Z", "", "", "I will list it first."),
                (
                    2,
                    "tool_use",
                    "2026-10-01T10:30:01.250Z",
                    "list_directory",
                    "call_o1",
                    "/home/u/gone",
                ),
                (
                    3,
                    "tool_result",
                    "2026-10-01T10:30:02.000Z",
                    "list_directory",
                    "call_o1",
                    "[error] directory not found",
                ),
                (
                    4,
                    "system",
                    "2026-10-01T10:30:03.000Z",
                    "",
                    "",
                    "system_message: background task finished",
                ),
            ],
        )
        self.assertEqual({row.session_id for row in rows}, {AGY_ORPHAN})
        self.assertTrue(rows[0].source_file.endswith(self.ORPHAN0))
        self.assertFalse((self.home / (self.LOGS % AGY_ORPHAN) / "transcript_full.jsonl").exists())

    def test_shortened_twin_alone(self):
        # The app conversation keeps only transcript.jsonl: it is read, and
        # a shortened field is marked.
        rows = self.rows_for(AntigravityParser(), self.APP)
        self.assertEqual(
            [(row.turn_type, row.tool_use_id, row.text) for row in rows],
            [
                ("user", "", "which packages are outdated?"),
                ("assistant", "", "I will ask Homebrew."),
                ("tool_use", "call_7", "brew outdated"),
                ("tool_result", "call_7", "node (24.1.0) < [truncated: content]"),
                ("system", "", "system_message: background task finished"),
                ("system", "", "error: model overloaded"),
            ],
        )
        self.assertEqual({row.session_id for row in rows}, {AGY_APP_CONVERSATION})

    def test_pairs_with_database(self):
        # Rendered as the database reader renders the same steps, and within
        # the tolerance of its rows' times, so every mirror row of both
        # conversations pairs with a database row: the app's shortened tool
        # output on its tool_use_id, the others on their text too.
        parser = AntigravityParser()
        for database, mirror in ((self.DATABASE, self.FULL), (self.APP_DATABASE, self.APP)):
            for thinking in (False, True):
                primary = self.rows_for(parser, database, include_thinking=thinking)
                secondary = [
                    (row, parser.secondary_tolerance(self.artifact(mirror)))
                    for row in self.rows_for(parser, mirror, include_thinking=thinking)
                ]
                rows, dropped = deduplicate(primary, secondary)
                self.assertEqual(dropped["antigravity"], len(secondary))
                self.assertEqual(rows[len(primary) :], [])
                # The pairs are close in time: the mirror's created_at is at
                # most two seconds before the database's completed_at here.
                for row, _ in secondary:
                    if row.text.endswith("[truncated: content]"):
                        twin = next(
                            primary_row
                            for primary_row in primary
                            if (primary_row.turn_type, primary_row.tool_use_id)
                            == (row.turn_type, row.tool_use_id)
                        )
                    else:
                        twin = next(
                            primary_row
                            for primary_row in primary
                            if (primary_row.turn_type, primary_row.text)
                            == (row.turn_type, row.text)
                        )
                    gap = (epoch_ms(twin.timestamp_utc) or 0) - (epoch_ms(row.timestamp_utc) or 0)
                    self.assertTrue(0 <= gap <= 2000, (row.text, gap))

    def test_truncated_line(self):
        # The bad line is its own row, in full; the rows before
        # it and a line written after it survive.
        before = self.rows_for(AntigravityParser(), self.ORPHAN0)
        write_bad_line(self.home / self.ORPHAN1)
        after = {
            "step_index": 4,
            "source": "SYSTEM",
            "type": "SYSTEM_MESSAGE",
            "status": "DONE",
            "created_at": "2026-10-01T10:30:04Z",
            "content": "after the cut",
        }
        with open(self.home / self.ORPHAN1, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(after) + "\n")
        rows = self.rows_for(AntigravityParser(), self.ORPHAN0)
        self.assertEqual(rows[0].text, "summarise the deleted project")
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[: len(before)]],
            [(row.source_line, row.turn_type, row.text) for row in before],
        )
        self.assertEqual(before[-1].text, "system_message: background task finished")
        self.assertEqual(
            [
                (row.turn_type, row.session_id, row.source_line, row.text)
                for row in rows[len(before) :]
            ],
            [
                ("system", AGY_ORPHAN, 6, "system_message: after the cut"),
                ("system", AGY_ORPHAN, 5, bad_line_text()),
            ],
        )

    def test_not_wanted(self):
        parser = AntigravityParser()
        rels = {artifact.rel: artifact for artifact in self.col.artifacts}
        output = self.LOGS % AGY_CONVERSATION
        output = output.replace("logs/", "steps/2/output.txt")
        self.assertFalse(parser.wants(rels[output]))

        def art(rel):
            return cast(Artifact, type("A", (), {"rel": rel})())

        tail = "brain/x/.system_generated/logs/"
        for directory in (
            "antigravity-cli",
            "antigravity",
            "antigravity-ide",
            "antigravity-backup",
        ):
            for name in (
                "transcript_full.jsonl",
                "transcript.jsonl",
                "chunks/transcript/00000003.jsonl",
            ):
                self.assertTrue(
                    parser.wants(art(".gemini/%s/%s%s" % (directory, tail, name))),
                    (directory, name),
                )
        for rel in (
            ".gemini/antigravity/brain/x/.system_generated/messages/m.json",
            ".gemini/antigravity/brain/x/.system_generated/tasks/task-7.log",
            ".gemini/antigravity/brain/x/.system_generated/logs/chunks/other/00000000.jsonl",
            ".gemini/antigravity/brain/x/.system_generated/logs/transcript_full.jsonl.bak",
            ".gemini/antigravity-other/brain/x/.system_generated/logs/transcript.jsonl",
        ):
            self.assertFalse(parser.wants(art(rel)), rel)


class AntigravityPermissionSettingsTests(ParserBase):
    """Test the standing permission files under ~/.gemini."""

    MTIME = "2026-10-01T12:00:00.000Z"  # AGY_CONFIG_MTIME
    CONFIG = ".gemini/config/config.json"
    PROJECT = ".gemini/config/projects/%s.json" % AGY_APP_PROJECT
    SIDECAR = ".gemini/config/sidecars/%s/sidecar.json" % AGY_SIDECAR
    CLI = ".gemini/antigravity-cli/settings.json"

    def check_common(self, rows, rel):
        for row in rows:
            self.assertEqual(
                (row.agent, row.turn_type, row.session_id, row.timestamp_utc),
                ("antigravity", "system", "", self.MTIME),
            )
            self.assertTrue(row.text.startswith("permission setting: "), row.text)
            self.assertTrue(row.text.endswith("(from %s, file mtime)" % rel), row.text)
        self.assertEqual([row.source_line for row in rows], list(range(1, len(rows) + 1)))

    def texts(self, rows):
        return [row.text.split(" (from ")[0] for row in rows]

    def test_mtime_constant(self):
        self.assertEqual(to_utc(AGY_CONFIG_MTIME), self.MTIME)

    def test_config_json(self):
        rows = self.rows_for(AntigravityParser(), self.CONFIG)
        self.check_common(rows, self.CONFIG)
        self.assertEqual(
            self.texts(rows),
            [
                "permission setting: userSettings.globalPermissionGrants.allow="
                "run_command(brew outdated)",
                "permission setting: userSettings.permissionGrantsV2Migrated=true",
                "permission setting: userSettings.sandboxEnabledAtV2Migration=false",
                "permission setting: sidecars.%s.enabled=true" % AGY_SIDECAR,
                "permission setting: sidecars.%s.resolvedPermissions.accessGrants="
                "run_command(brew outdated)" % AGY_SIDECAR,
            ],
        )
        self.assertEqual({row.project_path for row in rows}, {""})
        joined = " ".join(row.text for row in rows)
        self.assertNotIn("do-not-print", joined)
        self.assertNotIn("gdocs", joined)

    def test_project_file(self):
        rows = self.rows_for(AntigravityParser(), self.PROJECT)
        self.check_common(rows, self.PROJECT)
        self.assertEqual(
            self.texts(rows),
            [
                "permission setting: permissionGrants.permissionGrants.allow="
                "write_to_file(/home/u/proj/notes.md)",
                "permission setting: permissionGrants.v2Migrated=true",
            ],
        )
        self.assertEqual({row.project_path for row in rows}, {"/home/u/proj"})
        self.assertNotIn("do-not-print", " ".join(row.text for row in rows))
        # The CLI's default project file has no grants and yields nothing.
        self.assertEqual(
            self.rows_for(AntigravityParser(), ".gemini/config/projects/default-cli-project.json"),
            [],
        )

    def test_sidecar_json(self):
        rows = self.rows_for(AntigravityParser(), self.SIDECAR)
        self.check_common(rows, self.SIDECAR)
        self.assertEqual(
            self.texts(rows),
            ["permission setting: agent_permissions.access_grants=run_command(brew outdated)"],
        )
        self.assertNotIn("do-not-print", rows[0].text)

    def test_cli_settings(self):
        rows = self.rows_for(AntigravityParser(), self.CLI)
        self.check_common(rows, self.CLI)
        self.assertEqual(
            self.texts(rows),
            [
                "permission setting: permissions.allow=run_command(git status)",
                "permission setting: trustedWorkspace=/home/u/proj",
            ],
        )
        self.assertEqual([row.project_path for row in rows], ["", "/home/u/proj"])

    def test_manifest_mtime_preferred(self):
        art = next(artifact for artifact in self.col.artifacts if artifact.rel == self.CLI)
        art.manifest = {"mtime": AGY_CONFIG_MTIME + 60}
        self.assertEqual(file_mtime(art), "2026-10-01T12:01:00.000Z")
        art.manifest = {"mtime": 0}
        self.assertEqual(file_mtime(art), self.MTIME)

    def test_unreadable_file_is_reported(self):
        (self.home / self.SIDECAR).write_text('{"agent_permissions": {"acc', encoding="utf-8")
        rows = self.rows_for(AntigravityParser(), self.SIDECAR)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("system", "parser: unreadable %s" % self.SIDECAR)],
        )

    def test_other_config_files_not_wanted(self):
        parser = AntigravityParser()
        rels = {artifact.rel: artifact for artifact in self.col.artifacts}
        for rel in (self.CONFIG, self.PROJECT, self.SIDECAR, self.CLI):
            self.assertEqual(rels[rel].agent, "antigravity", rel)
            self.assertTrue(parser.wants(rels[rel]), rel)
        for rel in (
            ".gemini/config/mcp_config.json",
            ".gemini/config/plugins/gdocs/plugin.json",
            ".gemini/antigravity/plugin_data/gdocs/token.json",
            ".gemini/antigravity-cli/antigravity-oauth-token",
        ):
            self.assertEqual(rels[rel].agent, "antigravity", rel)
            self.assertFalse(parser.wants(rels[rel]), rel)
