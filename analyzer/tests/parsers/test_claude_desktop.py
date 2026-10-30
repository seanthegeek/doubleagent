"""Tests of the Claude Desktop parser."""

import json
import shutil

from doubleagent.dedup import deduplicate
from doubleagent.inputs import open_input
from doubleagent.parsers.claude_code import ClaudeCodeParser
from doubleagent.parsers.claude_desktop import ClaudeDesktopParser
from fixtures import (
    CD_AUDIT_REL,
    CD_CLI_SESSION,
    CD_CODE_CLI,
    CD_CODE_RECORD_REL,
    CD_CODE_REMOTE_CONTROL_CLI,
    CD_CODE_REMOTE_CONTROL_RECORD_REL,
    CD_COWORK_ORG,
    CD_FALLBACK_CLI,
    CD_FALLBACK_DIR,
    CD_FALLBACK_REL,
    CD_GUEST_CWD,
    CD_RECORD_REL,
    CD_REMOTE_SESSION,
    CD_SCHED_REL,
    CD_SESSION,
    CD_SPACES_REL,
    CD_TRANSCRIPT_REL,
    CD_WORKTREES_REL,
    CLAUDE_SESSION,
    bad_line_text,
    claude_subagent_records,
    write_bad_line,
)

from . import test_claude_code
from .base import ParserBase, rel_only


class ClaudeDesktopTests(ParserBase):
    """Test the Claude Desktop parser."""

    def test_transcript_rows(self):
        arts = {artifact.rel: artifact for artifact in self.col.artifacts}
        self.assertEqual(arts[CD_TRANSCRIPT_REL].agent, "claude-desktop")
        rows = self.rows_for(ClaudeDesktopParser(), CD_TRANSCRIPT_REL)
        # The claude-code reader's rows, line for line.
        self.assertEqual(
            [(row.source_line, row.turn_type) for row in rows],
            [(line, turn_type) for line, turn_type, _ in test_claude_code.ClaudeCodeTests.EXPECTED],
        )
        for row in rows:
            self.assertEqual((row.host, row.user, row.agent), ("h1", "alice", "claude-desktop"))
            self.assertEqual(row.session_id, CD_CLI_SESSION)
            # From the record's userSelectedFolders, not the guest cwd.
            self.assertEqual(row.project_path, "/srv/proj", row)
            self.assertEqual(row.source_file, "/alice/" + CD_TRANSCRIPT_REL)
        self.assertEqual(rows[0].text, "session surface: local-agent")
        self.assertEqual(rows[1].text, "delete the logs in /var/log please")

    def test_transcript_without_record_keeps_jsonl_cwd(self):
        (self.home / CD_RECORD_REL).unlink()
        rows = self.rows_for(ClaudeDesktopParser(), CD_TRANSCRIPT_REL)
        self.assertEqual(rows[0].project_path, "/sessions/quiet-river-1234")

    def test_record_not_read_through_symlink(self):
        record = self.home / CD_RECORD_REL
        target = self.tmp / "outside.json"
        target.write_bytes(record.read_bytes())
        record.unlink()
        try:
            record.symlink_to(target)
        except OSError:
            self.skipTest("symlinks not available")
        rows = self.rows_for(ClaudeDesktopParser(), CD_TRANSCRIPT_REL)
        self.assertEqual(rows[0].project_path, "/sessions/quiet-river-1234")

    def test_subagent_transcript(self):
        rel = CD_TRANSCRIPT_REL[: -len(".jsonl")] + "/subagents/agent-abc.jsonl"
        path = self.home / rel
        path.parent.mkdir(parents=True)
        lines = [json.dumps(record) for record in claude_subagent_records()]
        path.write_text("\n".join(lines) + "\n", encoding="utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(ClaudeDesktopParser(), rel)
        self.assertEqual(rows[0].text, "subagent abc of %s" % CLAUDE_SESSION)
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})
        self.assertEqual({row.agent for row in rows}, {"claude-desktop"})

    def test_secondary(self):
        # The audit log and the session records are secondary sources of
        # the transcript; the transcript and the index files are primary.
        parser = ClaudeDesktopParser()
        for rel in (CD_AUDIT_REL, CD_FALLBACK_REL, CD_RECORD_REL, CD_CODE_RECORD_REL):
            self.assertTrue(parser.secondary(self.artifact(rel)), rel)
        for rel in (CD_TRANSCRIPT_REL, CD_SCHED_REL, CD_SPACES_REL):
            self.assertFalse(parser.secondary(self.artifact(rel)), rel)

    def test_record_initial_message_secondary(self):
        # The Cowork record's initialMessage is yielded beside the
        # transcript and the audit log, with no time of its own, and drops
        # as a copy of the audit log's prompt; the record's session row has
        # no twin and is kept.
        parser = ClaudeDesktopParser()
        primary = self.rows_for(parser, CD_TRANSCRIPT_REL)
        secondary = [
            (row, parser.secondary_tolerance(self.artifact(rel)))
            for rel in (CD_AUDIT_REL, CD_RECORD_REL)
            for row in self.rows_for(parser, rel)
        ]
        initial = [row for row, _ in secondary if row.source_file.endswith(".json")]
        self.assertEqual(
            [(row.turn_type, row.timestamp_utc) for row in initial if row.turn_type == "user"],
            [("user", "")],
        )
        kept, _ = deduplicate(primary, secondary)
        from_record = [row for row in kept if row.source_file.endswith(CD_SESSION + ".json")]
        self.assertEqual({row.turn_type for row in from_record}, {"system"})
        self.assertIn(
            'desktop session: agent "Summarise the test failures" permission_mode=default effort=-',
            [row.text for row in from_record],
        )
        prompts = [row for row in kept if row.text == "why does the test fail?"]
        self.assertEqual([row.source_file.endswith("audit.jsonl") for row in prompts], [True])

    def test_audit_with_transcript(self):
        # Every line is read although the transcript exists; dedup drops
        # the rows the transcript repeats (next test).
        rows = self.rows_for(ClaudeDesktopParser(), CD_AUDIT_REL)
        self.assertEqual(
            [
                (row.source_line, row.turn_type, row.tool_name, row.timestamp_utc, row.text)
                for row in rows
            ],
            [
                (1, "user", "", "2026-10-01T10:00:00.000Z", "why does the test fail?"),
                (2, "system", "", "2026-10-01T10:00:01.000Z", "model: claude-fable-5-1"),
                (
                    2,
                    "system",
                    "",
                    "2026-10-01T10:00:01.000Z",
                    "init: cwd=/sessions/quiet-river-1234 model=claude-fable-5-1",
                ),
                (3, "tool_use", "Bash", "2026-10-01T10:00:03.000Z", "npm test"),
                (
                    4,
                    "system",
                    "Bash",
                    "2026-10-01T10:00:03.100Z",
                    "permission request: Bash npm test",
                ),
                (
                    5,
                    "system",
                    "Bash",
                    "2026-10-01T10:00:05.000Z",
                    "permission response: Bash once granted=true",
                ),
                (6, "tool_result", "", "2026-10-01T10:00:09.000Z", "1 failing"),
                (
                    7,
                    "system",
                    "",
                    "2026-10-01T10:00:12.000Z",
                    "result: success turns=2 error=false",
                ),
                (8, "user", "", "2026-10-01T10:00:20.000Z", "thanks"),
            ],
        )
        for row in rows:
            self.assertEqual((row.agent, row.session_id), ("claude-desktop", CD_CLI_SESSION))
            self.assertEqual(row.project_path, "/srv/proj")
        self.assertEqual(rows[1].model, "claude-fable-5-1")
        self.assertEqual((rows[3].tool_use_id, rows[6].tool_use_id), ("toolu_a1", "toolu_a1"))

    def test_audit_pairs_with_transcript(self):
        # Both files are parsed; the audit rows the transcript repeats (same
        # text, session id and project, time within 2 seconds) are dropped,
        # the others kept.
        parser = ClaudeDesktopParser()
        primary = self.rows_for(parser, CD_TRANSCRIPT_REL)
        audit = self.rows_for(parser, CD_AUDIT_REL)
        tolerance = parser.secondary_tolerance(self.artifact(CD_AUDIT_REL))
        kept, dropped = deduplicate(primary, [(row, tolerance) for row in audit])
        self.assertEqual(dropped, {"claude-desktop": 2})
        kept_ids = {id(row) for row in kept}
        gone = [(row.source_line, row.text) for row in audit if id(row) not in kept_ids]
        self.assertEqual(gone, [(2, "model: claude-fable-5-1"), (8, "thanks")])
        texts = [row.text for row in kept if row.source_file.endswith("audit.jsonl")]
        self.assertIn("why does the test fail?", texts)
        self.assertIn("result: success turns=2 error=false", texts)
        twin = next(row for row in primary if row.text == "thanks")
        self.assertEqual(
            (twin.session_id, twin.project_path), (audit[-1].session_id, audit[-1].project_path)
        )

    def test_audit_without_transcript(self):
        rows = self.rows_for(ClaudeDesktopParser(), CD_FALLBACK_REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                (1, "user", "list the files"),
                (2, "system", "model: claude-fable-5-1"),
                (2, "system", "init: cwd=/sessions/quiet-river-1234 model=claude-fable-5-1"),
                (3, "system", "context: notification: folder mounted"),
                (4, "assistant", "Listing."),
                (4, "tool_use", "ls"),
                (5, "system", "permission auto-approved: Bash (always_allow)"),
                (6, "tool_result", "README.md"),
                (7, "assistant", "One file: README.md"),
                (9, "system", "result: success turns=1 error=false"),
                (
                    10,
                    "system",
                    'model_refusal_fallback: {"note":"fields not recorded; fixture only"}',
                ),
                (11, "system", "model: claude-fable-5-1 -> claude-opus-5-5"),
                (11, "assistant", "Answered by the fallback."),
            ],
        )
        change = rows[-2]
        self.assertEqual(
            (change.timestamp_utc, change.model, change.session_id),
            ("2026-10-02T09:00:10.000Z", "claude-opus-5-5", CD_FALLBACK_CLI),
        )
        by_key = {(row.source_line, row.turn_type): row for row in rows}
        # The prompt's own timestamp wins over the audit write time.
        self.assertEqual(by_key[1, "user"].timestamp_utc, "2026-10-02T08:59:59.500Z")
        self.assertEqual(by_key[2, "system"].timestamp_utc, "2026-10-02T09:00:01.000Z")
        use = by_key[4, "tool_use"]
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.model), ("Bash", "toolu_b1", "claude-fable-5-1")
        )
        self.assertEqual(by_key[6, "tool_result"].tool_use_id, "toolu_b1")
        # No session record: the line's session id, and the init line's cwd
        # as the project, as the transcript reader keeps the JSONL cwd.
        self.assertEqual({row.session_id for row in rows}, {CD_FALLBACK_CLI})
        self.assertEqual({row.project_path for row in rows}, {CD_GUEST_CWD})

    def test_audit_short_dir_joins_record(self):
        # A record under agent/ whose UUID starts with the short dir name.
        record = (
            self.home
            / CD_COWORK_ORG
            / "agent"
            / ("local_%s-aaaa-4bbb-8ccc-dddddddddddd.json" % CD_FALLBACK_DIR)
        )
        record.parent.mkdir()
        record.write_text(
            json.dumps(
                {
                    "sessionId": "local_%s-aaaa-4bbb-8ccc-dddddddddddd" % CD_FALLBACK_DIR,
                    "cliSessionId": "cafecafe-0000-4000-8000-000000000001",
                    "userSelectedFolders": ["/srv/other"],
                    "createdAt": 1790762400000,
                    "lastActivityAt": 1790762400000,
                }
            ),
            encoding="utf-8",
        )
        rows = self.rows_for(ClaudeDesktopParser(), CD_FALLBACK_REL)
        self.assertEqual({row.session_id for row in rows}, {"cafecafe-0000-4000-8000-000000000001"})
        self.assertEqual({row.project_path for row in rows}, {"/srv/other"})

    def test_cowork_record(self):
        rows = self.rows_for(ClaudeDesktopParser(), CD_RECORD_REL)
        # The session row, the grant rows (test_cowork_record_grants), then
        # initialMessage, yielded whether or not the transcript and the
        # audit log exist, with no time of its own.
        self.assertEqual(
            [
                (row.source_line, row.turn_type, row.timestamp_utc, row.text)
                for row in rows
                if not row.text.startswith("permission grant:")
            ],
            [
                (
                    1,
                    "system",
                    "2026-09-30T10:00:00.000Z",
                    'desktop session: agent "Summarise the test failures" '
                    "permission_mode=default effort=-",
                ),
                (1, "user", "", "why does the test fail?"),
            ],
        )
        row = rows[0]
        self.assertEqual(
            (row.timestamp_utc, row.session_id, row.project_path, row.git_branch, row.model),
            ("2026-09-30T10:00:00.000Z", CD_CLI_SESSION, "/srv/proj", "", "claude-fable-5-1"),
        )

    def test_cowork_record_grants(self):
        # One row per list entry and object member of the Cowork tool,
        # network and folder grants, at lastActivityAt; an empty list
        # (fileDeleteApprovedMounts) yields nothing.
        rows = self.rows_for(ClaudeDesktopParser(), CD_RECORD_REL)[1:]
        rows = [row for row in rows if row.turn_type == "system"]
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission grant: approvedToolNames Bash (as of lastActivityAt)",
                "permission grant: approvedToolNames WebFetch (as of lastActivityAt)",
                "permission grant: webFetchAllowedUrls https://example.invalid/docs "
                "(as of lastActivityAt)",
                "permission grant: egressAllowedDomains example.invalid (as of lastActivityAt)",
                "permission grant: userApprovedFileAccessPaths /srv/proj/data "
                "(as of lastActivityAt)",
                "permission grant: enabledMcpTools.github:create_issue=true (as of lastActivityAt)",
            ],
        )
        for row in rows:
            self.assertEqual(
                (
                    row.turn_type,
                    row.timestamp_utc,
                    row.session_id,
                    row.project_path,
                    row.source_line,
                ),
                ("system", "2026-09-30T10:01:00.000Z", CD_CLI_SESSION, "/srv/proj", 1),
            )

    def test_cowork_grant_scalar_value(self):
        path = self.home / CD_RECORD_REL
        record = json.loads(path.read_text(encoding="utf-8"))
        for key in (
            "approvedToolNames",
            "webFetchAllowedUrls",
            "egressAllowedDomains",
            "userApprovedFileAccessPaths",
            "enabledMcpTools",
        ):
            record.pop(key)
        record["fileDeleteApprovedMounts"] = "proj"
        path.write_text(json.dumps(record), encoding="utf-8")
        rows = self.rows_for(ClaudeDesktopParser(), CD_RECORD_REL)
        self.assertEqual(
            [row.text for row in rows[1:] if row.turn_type == "system"],
            ["permission grant: fileDeleteApprovedMounts=proj (as of lastActivityAt)"],
        )

    def test_record_without_session_and_with_error(self):
        shutil.rmtree(self.home / CD_COWORK_ORG / CD_SESSION)
        path = self.home / CD_RECORD_REL
        record = json.loads(path.read_text(encoding="utf-8"))
        record.update(error="VM failed to start", errorCategory="vm", errorAt=1790762430000)
        path.write_text(json.dumps(record), encoding="utf-8")
        rows = self.rows_for(ClaudeDesktopParser(), CD_RECORD_REL)
        self.assertEqual(
            [
                (row.turn_type, row.timestamp_utc, row.text)
                for row in rows
                if not row.text.startswith("permission grant:")
            ],
            [
                (
                    "system",
                    "2026-09-30T10:00:00.000Z",
                    'desktop session: agent "Summarise the test failures" '
                    "permission_mode=default effort=-",
                ),
                ("user", "", "why does the test fail?"),
                ("system", "2026-09-30T10:00:30.000Z", "session error: vm: VM failed to start"),
            ],
        )

    def test_code_record(self):
        rows = self.rows_for(ClaudeDesktopParser(), CD_CODE_RECORD_REL)
        row = rows[0]
        self.assertEqual(
            (
                row.turn_type,
                row.text,
                row.timestamp_utc,
                row.session_id,
                row.project_path,
                row.git_branch,
            ),
            (
                "system",
                'desktop code session: "Fix the failing test" remote_control=false bridges=0 '
                "spawned_remotely=false steered_remotely=false permission_mode=default "
                "effort=high",
                "2026-09-30T10:00:00.000Z",
                CD_CODE_CLI,
                "/srv/proj",
                "main",
            ),
        )

    def test_code_record_grants(self):
        # One row per list entry and per grant key with a value, at
        # lastActivityAt, joined to the session like the record row.
        rows = self.rows_for(ClaudeDesktopParser(), CD_CODE_RECORD_REL)[1:]
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission grant: alwaysAllowedReasons Bash(npm test) (as of lastActivityAt)",
                "permission grant: sessionPermissionUpdates "
                '{"behavior":"allow","destination":"session",'
                '"rules":[{"ruleContent":"npm test","toolName":"Bash"}],"type":"addRules"} '
                "(as of lastActivityAt)",
                "permission grant: chromePermissionMode=ask (as of lastActivityAt)",
                'permission grant: chromeAllowedDomains=["example.invalid"] (as of lastActivityAt)',
                'permission grant: cuAllowedApps=["Terminal"] (as of lastActivityAt)',
                'permission grant: cuGrantFlags={"clipboardRead":true} (as of lastActivityAt)',
                "permission grant: cuFlagsGrantedAt=1790762450000 (as of lastActivityAt)",
            ],
        )
        for row in rows:
            self.assertEqual(
                (
                    row.turn_type,
                    row.timestamp_utc,
                    row.session_id,
                    row.project_path,
                    row.tool_use_id,
                ),
                ("system", "2026-09-30T10:02:00.000Z", CD_CODE_CLI, "/srv/proj", ""),
            )

    def test_record_grants_fall_back_to_created_and_skip_empty(self):
        path = self.home / CD_CODE_RECORD_REL
        record = json.loads(path.read_text(encoding="utf-8"))
        record.pop("lastActivityAt")
        record.pop("effort")
        record.update(alwaysAllowedReasons=[], chromeAllowedDomains=[], cuGrantFlags={})
        record.update(sessionPermissionUpdates=None, cuAllowedApps="", cuFlagsGrantedAt=None)
        path.write_text(json.dumps(record), encoding="utf-8")
        rows = self.rows_for(ClaudeDesktopParser(), CD_CODE_RECORD_REL)
        self.assertTrue(rows[0].text.endswith("permission_mode=default effort=-"), rows[0].text)
        self.assertEqual(
            [(row.text, row.timestamp_utc) for row in rows[1:]],
            [
                (
                    "permission grant: chromePermissionMode=ask (as of createdAt)",
                    "2026-09-30T10:00:00.000Z",
                )
            ],
        )

    def test_code_record_remote_control(self):
        rows = self.rows_for(ClaudeDesktopParser(), CD_CODE_REMOTE_CONTROL_RECORD_REL)
        self.assertEqual(
            [
                (
                    row.turn_type,
                    row.text,
                    row.timestamp_utc,
                    row.session_id,
                    row.project_path,
                    row.source_line,
                )
                for row in rows
            ],
            [
                (
                    "system",
                    'desktop code session: "Driven from the phone" remote_control=true '
                    "bridges=2 spawned_remotely=true steered_remotely=true "
                    "permission_mode=default effort=-",
                    "2026-09-30T11:00:00.000Z",
                    CD_CODE_REMOTE_CONTROL_CLI,
                    "/srv/proj",
                    1,
                ),
                (
                    "system",
                    "remote control live at update quit",
                    "2026-09-30T11:08:20.000Z",
                    CD_CODE_REMOTE_CONTROL_CLI,
                    "/srv/proj",
                    1,
                ),
            ],
        )

    def test_code_record_remote_control_edge_values(self):
        # An empty spawn object is not a remote spawn; an update-quit object
        # without a numeric `at` yields no second row.
        path = self.home / CD_CODE_REMOTE_CONTROL_RECORD_REL
        record = json.loads(path.read_text(encoding="utf-8"))
        record.update(
            remoteControlSpawn={},
            bridgeSessionIds=None,
            remoteControlLiveAtUpdateQuit={"accountId": "x", "at": "soon"},
        )
        path.write_text(json.dumps(record), encoding="utf-8")
        rows = self.rows_for(ClaudeDesktopParser(), CD_CODE_REMOTE_CONTROL_RECORD_REL)
        self.assertEqual(
            [row.text for row in rows],
            [
                'desktop code session: "Driven from the phone" remote_control=true '
                "bridges=0 spawned_remotely=false steered_remotely=true "
                "permission_mode=default effort=-"
            ],
        )

    def test_code_record_joins_home_transcript(self):
        # The Code tab's transcript is claude-code's ~/.claude/projects file:
        # when it is in the input, the first prompt is not repeated.
        path = self.home / CD_CODE_RECORD_REL
        record = json.loads(path.read_text(encoding="utf-8"))
        record.update(cliSessionId=CLAUDE_SESSION, initialMessage="fix the test")
        path.write_text(json.dumps(record), encoding="utf-8")
        rows = self.rows_for(ClaudeDesktopParser(), CD_CODE_RECORD_REL)
        self.assertEqual(
            [(row.turn_type, row.session_id) for row in rows if "permission grant" not in row.text],
            [("system", CLAUDE_SESSION)],
        )
        shutil.rmtree(self.home / ".claude/projects")
        rows = self.rows_for(ClaudeDesktopParser(), CD_CODE_RECORD_REL)
        rows = [row for row in rows if "permission grant" not in row.text]
        self.assertEqual([row.turn_type for row in rows], ["system", "user"])
        self.assertEqual(rows[1].text, "fix the test")

    def test_empty_index_files_yield_nothing(self):
        # A fresh install writes the markers before any task or worktree
        # exists (`{"schemaVersion": 2}`); only a non-object file is
        # reported.
        (self.home / CD_WORKTREES_REL).write_text('{"schemaVersion": 2}', encoding="utf-8")
        (self.home / CD_SCHED_REL).write_text('{"recordedSkips": {}}', encoding="utf-8")
        self.assertEqual(self.rows_for(ClaudeDesktopParser(), CD_WORKTREES_REL), [])
        self.assertEqual(self.rows_for(ClaudeDesktopParser(), CD_SCHED_REL), [])
        (self.home / CD_WORKTREES_REL).write_text("[]", encoding="utf-8")
        rows = self.rows_for(ClaudeDesktopParser(), CD_WORKTREES_REL)
        self.assertEqual([row.turn_type for row in rows], ["system"])
        self.assertIn("unreadable", rows[0].text)

    def test_bad_record_is_one_row(self):
        (self.home / CD_RECORD_REL).write_text("[1, 2]", encoding="utf-8")
        rows = self.rows_for(ClaudeDesktopParser(), CD_RECORD_REL)
        self.assertEqual([row.turn_type for row in rows], ["system"])
        self.assertIn("not a JSON object", rows[0].text)

    def test_scheduled_tasks_and_worktrees(self):
        rows = self.rows_for(ClaudeDesktopParser(), CD_SCHED_REL)
        self.assertEqual(
            [
                (row.source_line, row.turn_type, row.timestamp_utc, row.project_path, row.text)
                for row in rows
            ],
            [
                (
                    1,
                    "system",
                    "2026-09-30T10:00:00.000Z",
                    "/srv/proj",
                    "scheduled task: daily-report 0 9 * * 1-5 enabled=true "
                    "permission_mode=default model=claude-fable-5-1",
                )
            ],
        )
        self.assertEqual(rows[0].model, "claude-fable-5-1")
        rows = self.rows_for(ClaudeDesktopParser(), CD_WORKTREES_REL)
        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual(
            (row.turn_type, row.timestamp_utc, row.project_path, row.git_branch, row.session_id),
            ("system", "2026-09-30T10:00:01.000Z", "/srv/proj", "claude/brave-otter", ""),
        )
        self.assertEqual(
            row.text,
            "worktree created: /srv/proj/.claude/worktrees/brave-otter "
            "branch=claude/brave-otter from=main leased by "
            "local_e2e2e2e2-9999-4aaa-8bbb-cccccccccccc",
        )

    def test_scheduled_task_without_mode_or_model(self):
        doc = json.loads((self.home / CD_SCHED_REL).read_text(encoding="utf-8"))
        del doc["scheduledTasks"][0]["permissionMode"], doc["scheduledTasks"][0]["model"]
        (self.home / CD_SCHED_REL).write_text(json.dumps(doc), encoding="utf-8")
        rows = self.rows_for(ClaudeDesktopParser(), CD_SCHED_REL)
        self.assertEqual(
            [(row.text, row.model) for row in rows],
            [
                (
                    "scheduled task: daily-report 0 9 * * 1-5 enabled=true "
                    "permission_mode=- model=-",
                    "",
                )
            ],
        )

    def test_remote_session_spaces(self):
        # One standing row per granted folder, at the file mtime, with the
        # folder as project_path and no session id.
        rows = self.rows_for(ClaudeDesktopParser(), CD_SPACES_REL)
        key = "entries.%s.folders" % CD_REMOTE_SESSION
        self.assertEqual(
            [
                (
                    row.turn_type,
                    row.text,
                    row.timestamp_utc,
                    row.session_id,
                    row.project_path,
                    row.source_line,
                )
                for row in rows
            ],
            [
                (
                    "system",
                    "permission setting: %s=%s (from %s, file mtime)"
                    % (key, folder, CD_SPACES_REL),
                    "2026-10-01T10:00:00.000Z",
                    "",
                    folder,
                    1,
                )
                for folder in ("/srv/proj", "/srv/docs")
            ],
        )

    def test_remote_session_spaces_not_object(self):
        (self.home / CD_SPACES_REL).write_text("[1]", encoding="utf-8")
        rows = self.rows_for(ClaudeDesktopParser(), CD_SPACES_REL)
        self.assertEqual(
            [(row.turn_type, row.source_line, row.text) for row in rows],
            [("system", 1, "parser: remote-session-spaces.json is not a JSON object")],
        )
        (self.home / CD_SPACES_REL).write_text(
            '{"entries": [{"folders": ["/x"]}, 3]}', encoding="utf-8"
        )
        rows = self.rows_for(ClaudeDesktopParser(), CD_SPACES_REL)
        self.assertEqual(
            [row.text for row in rows],
            ["permission setting: entries.folders=/x (from %s, file mtime)" % CD_SPACES_REL],
        )

    def test_thinking_opt_in(self):
        parser = ClaudeDesktopParser()
        rows = self.rows_for(parser, CD_TRANSCRIPT_REL)
        self.assertFalse(any(row.turn_type == "thinking" for row in rows))
        rows = self.rows_for(parser, CD_TRANSCRIPT_REL, include_thinking=True)
        self.assertEqual(
            [(row.source_line, row.text) for row in rows if row.turn_type == "thinking"],
            [(3, "private reasoning")],
        )
        rows = self.rows_for(parser, CD_FALLBACK_REL, include_thinking=True)
        self.assertEqual(
            [(row.source_line, row.text) for row in rows if row.turn_type == "thinking"],
            [(4, "use ls")],
        )

    def test_not_wanted(self):
        parser = ClaudeDesktopParser()
        arts = {artifact.rel: artifact for artifact in self.col.artifacts}
        base = "Library/Application Support/Claude/"
        for rel in (
            CD_COWORK_ORG + "/spaces.json",
            CD_COWORK_ORG + "/cowork_settings.json",
            CD_COWORK_ORG + "/rpm/manifest.json",
            CD_COWORK_ORG + "/%s/.audit-key" % CD_SESSION,
            base + "claude_desktop_config.json",
        ):
            self.assertEqual(arts[rel].agent, "claude-desktop", rel)
            self.assertFalse(parser.wants(arts[rel]), rel)
        # Each parser reads only its own tree.
        self.assertFalse(ClaudeCodeParser().wants(arts[CD_TRANSCRIPT_REL]))
        self.assertFalse(parser.wants(arts[test_claude_code.ClaudeCodeTests.REL]))
        self.assertFalse(parser.wants(arts[".claude/history.jsonl"]))
        # The other catalog bases, and the staged import tree.
        tail = CD_TRANSCRIPT_REL[len(base) :]
        for root in (
            ".config/Claude-3p/",
            "AppData/Roaming/Claude/",
            "AppData/Local/Claude-3p/",
            "AppData/Local/Claude/",
            "AppData/Local/Claude-Data/",
            "AppData/Local/Claude-Data/claude_pzs8sxrjxfjjc/",
            "AppData/Local/Packages/Claude_pzs8sxrjxfjjc/LocalCache/Roaming/Claude/",
            "AppData/Local/Packages/AnthropicPBC.Claude_fnn82j28hfe8t/LocalCache/Roaming/Claude/",
        ):
            self.assertTrue(parser.wants(rel_only(root + tail)), root)
        self.assertFalse(
            parser.wants(
                rel_only("AppData/Local/Packages/Other_x/LocalCache/Roaming/Claude/" + tail)
            )
        )
        self.assertFalse(parser.wants(rel_only("Claude/" + tail)))
        self.assertTrue(
            parser.wants(rel_only(base + "claude-code-sessions/a/b/imported-staging/x.jsonl"))
        )

    def test_truncated_line_is_reported_not_fatal(self):
        # Each bad line is its own row, in full, with the session and
        # project; the rows before it and a line after survive.
        before = self.rows_for(ClaudeDesktopParser(), CD_AUDIT_REL)
        self.assertEqual(len(before), 9)
        write_bad_line(self.home / CD_AUDIT_REL)
        after = {
            "type": "user",
            "session_id": CD_CLI_SESSION,
            "parent_tool_use_id": None,
            "client_platform": "desktop_app",
            "timestamp": "2026-10-01T10:30:00.000Z",
            "message": {"role": "user", "content": [{"type": "text", "text": "after the cut"}]},
            "_audit_timestamp": "2026-10-01T10:30:00.003Z",
        }
        with open(self.home / CD_AUDIT_REL, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(after) + "\n")
        rows = self.rows_for(ClaudeDesktopParser(), CD_AUDIT_REL)
        self.assertEqual(len(rows), 11)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[:9]],
            [(row.source_line, row.turn_type, row.text) for row in before],
        )
        self.assertEqual((rows[9].source_line, rows[9].text), (10, "after the cut"))
        self.assertEqual(
            (
                rows[-1].turn_type,
                rows[-1].session_id,
                rows[-1].project_path,
                rows[-1].source_line,
                rows[-1].text,
            ),
            ("system", CD_CLI_SESSION, before[-1].project_path, 9, bad_line_text()),
        )
        write_bad_line(self.home / CD_TRANSCRIPT_REL)
        rows = self.rows_for(ClaudeDesktopParser(), CD_TRANSCRIPT_REL)
        self.assertEqual(len(rows), len(test_claude_code.ClaudeCodeTests.EXPECTED) + 1)
        self.assertEqual(
            [(row.source_line, row.turn_type) for row in rows[:-1]],
            [(line, turn_type) for line, turn_type, _ in test_claude_code.ClaudeCodeTests.EXPECTED],
        )
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].source_line, rows[-1].text),
            ("system", 35, bad_line_text()),
        )
