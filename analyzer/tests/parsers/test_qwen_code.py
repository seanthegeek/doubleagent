"""Tests of the Qwen Code parser."""

import json
import os
import unittest
from dataclasses import replace

from test_errors import CAN_DENY, DENY_REASON

from doubleagent.dedup import deduplicate
from doubleagent.inputs import Artifact, open_input
from doubleagent.parsers import Options
from doubleagent.parsers.qwen_code import (
    QwenCodeParser,
    args_summary,
    injected_origin,
    quoted_group_messages,
)
from doubleagent.timeutil import to_utc
from fixtures import (
    QWEN_ARCHIVED,
    QWEN_AUDIT_REL,
    QWEN_DAEMON_HASH,
    QWEN_DAEMON_SESSION,
    QWEN_GROUP_HISTORY_REL,
    QWEN_GROUP_REL,
    QWEN_GROUP_SESSION,
    QWEN_GROUP_TIMES,
    QWEN_HARNESS_REL,
    QWEN_HARNESS_SESSION,
    QWEN_ORGANIZATION_REL,
    QWEN_PAST_USE_MTIME,
    QWEN_PR_REL,
    QWEN_SESSION,
    QWEN_SUMMARY_REL,
    QWEN_TEMPORARY,
    QWEN_TODO_DEFAULT_REL,
    QWEN_TODO_REL,
    QWEN_TOKEN_USAGE_REL,
    QWEN_TRUST_MTIME,
    QWEN_WORKER_REL,
    QWEN_WORKER_SESSION,
    QWEN_WORKER_USAGE_REL,
    bad_line_text,
    qwen_group_history_records,
    qwen_group_session_records,
    write_bad_line,
)
from fixtures.common import _jsonl

from .base import ParserBase, kept_rows, parse_with, rel_only, unreadable_rows

BOB_DEPLOY = "group message: [Bob] the deploy is broken"
# A session of another chat, quoting the same text at another time.
QWEN_OTHER_CHAT_SESSION = "e5f6a7b8-4444-4000-8000-0000000000aa"


class QwenCodeTests(ParserBase):
    """Test the Qwen Code parser."""

    REL = ".qwen/projects/-srv-proj/chats/%s.jsonl" % QWEN_SESSION
    LOGS = ".qwen/tmp/%s/logs.json" % QWEN_TEMPORARY

    def test_secondary(self):
        # The history file is a secondary source, the transcript is not.
        parser = QwenCodeParser()
        self.assertTrue(parser.secondary(self.artifact(self.LOGS)))
        self.assertFalse(parser.secondary(self.artifact(self.REL)))

    def test_rows(self):
        rows = self.rows_for(QwenCodeParser(), self.REL)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "user",
                "assistant",
                "tool_use",
                "tool_use",
                "tool_result",
                "tool_result",
                "system",
                "system",
                "system",
                "tool_use",
                "tool_result",
                "system",
                "system",
            ],
        )
        for row in rows[:13]:
            self.assertEqual((row.host, row.user, row.agent), ("h1", "alice", "qwen-code"))
            self.assertEqual(
                (row.session_id, row.project_path, row.git_branch),
                (QWEN_SESSION, "/srv/proj", "main"),
            )
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        model, user, asst, shell, read, shout, readout, slash, title = rows[:9]
        self.assertEqual(
            (model.text, model.model, model.timestamp_utc),
            (
                "model: qwen3-coder-plus auth=qwen-oauth",
                "qwen3-coder-plus",
                "2026-10-01T09:59:59.000Z",
            ),
        )
        self.assertEqual(
            (user.text, user.timestamp_utc), ("run the tests", "2026-10-01T10:00:00.000Z")
        )
        self.assertEqual((asst.text, asst.model), ("Running tests.", "qwen3-coder-plus"))
        self.assertEqual(
            (shell.tool_name, shell.tool_use_id, shell.text, shell.model),
            ("run_shell_command", "call_abc123", "npm test", "qwen3-coder-plus"),
        )
        self.assertEqual((read.tool_name, read.text), ("read_file", "/srv/proj/.env"))
        self.assertEqual(
            (shout.tool_name, shout.tool_use_id, shout.text),
            ("run_shell_command", "call_abc123", "12 passing"),
        )
        self.assertEqual(
            (readout.tool_use_id, readout.text),
            ("call_def456", "[error permission_denied] permission denied"),
        )
        self.assertEqual(slash.text, "slash_command: invocation /compress")
        self.assertEqual(title.text, "custom_title: Run tests")
        self.assertEqual(
            [row.source_line for row in rows], [1, 2, 3, 3, 3, 4, 5, 6, 7, 8, 8, 9, 9, 1]
        )

    def test_model_change_and_decision_rows(self):
        rows = self.rows_for(QwenCodeParser(), self.REL)
        change, use, result, decision = rows[9:13]
        self.assertEqual(
            (change.turn_type, change.text, change.model, change.timestamp_utc),
            (
                "system",
                "model: qwen3-coder-plus -> qwen3-vl-plus",
                "qwen3-vl-plus",
                "2026-10-01T10:01:00.000Z",
            ),
        )
        self.assertEqual((use.tool_use_id, use.model), ("call_ghi789", "qwen3-vl-plus"))
        self.assertEqual(result.text, "[cancelled] cancelled by user")  # prefix kept
        self.assertEqual(
            (
                decision.turn_type,
                decision.text,
                decision.tool_name,
                decision.tool_use_id,
                decision.timestamp_utc,
                decision.source_line,
            ),
            (
                "system",
                "permission decision: cancelled",
                "run_shell_command",
                "call_ghi789",
                "2026-10-01T10:01:05.000Z",
                9,
            ),
        )

    def test_start_row_without_session_model(self):
        path = self.home / self.REL
        lines = path.read_text(encoding="utf-8").splitlines(keepends=True)
        path.write_text("".join(lines[1:]), encoding="utf-8")  # drop session_model
        rows = self.rows_for(QwenCodeParser(), self.REL)
        models = [(row.text, row.timestamp_utc) for row in rows if row.text.startswith("model:")]
        self.assertEqual(
            models,
            [
                ("model: qwen3-coder-plus", "2026-10-01T10:00:03.000Z"),
                ("model: qwen3-coder-plus -> qwen3-vl-plus", "2026-10-01T10:01:00.000Z"),
            ],
        )
        self.assertEqual([row.turn_type for row in rows[:3]], ["user", "system", "assistant"])

    def test_trusted_folders(self):
        rel = ".qwen/trustedFolders.json"
        rows = self.rows_for(QwenCodeParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.text, row.project_path) for row in rows],
            [
                (
                    "system",
                    "permission setting: trustedFolders./srv/proj=TRUST_FOLDER "
                    "(from .qwen/trustedFolders.json, file mtime)",
                    "/srv/proj",
                ),
                (
                    "system",
                    "permission setting: trustedFolders./srv=DO_NOT_TRUST "
                    "(from .qwen/trustedFolders.json, file mtime)",
                    "/srv",
                ),
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.timestamp_utc, row.session_id, row.source_line),
                (to_utc(QWEN_TRUST_MTIME), "", 1),
            )
        (self.home / rel).write_text("[1, 2]", encoding="utf-8")
        rows = self.rows_for(QwenCodeParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("system", "parser: .qwen/trustedFolders.json is not a JSON object")],
        )

    def test_thinking_opt_in(self):
        rows = self.rows_for(QwenCodeParser(), self.REL, include_thinking=True)
        self.assertEqual(
            [row.text for row in rows if row.turn_type == "thinking"], ["Plan: run npm test."]
        )

    def test_archive_and_logs(self):
        rows = self.rows_for(
            QwenCodeParser(), ".qwen/projects/-srv-proj/chats/archive/%s.jsonl" % QWEN_ARCHIVED
        )
        self.assertEqual(
            [(row.turn_type, row.session_id) for row in rows], [("user", QWEN_ARCHIVED)]
        )
        rows = self.rows_for(QwenCodeParser(), self.LOGS)
        self.assertEqual([row.turn_type for row in rows], ["user", "system"])
        self.assertEqual(
            (rows[0].text, rows[0].session_id, rows[0].project_path),
            ("run the tests", QWEN_SESSION, ""),
        )
        self.assertEqual(
            rows[1].text, "model: qwen3-coder-plus -> qwen3-vl-plus reason=vision_auto_switch"
        )
        self.assertEqual(rows[1].model, "qwen3-vl-plus")
        self.assertEqual([row.source_line for row in rows], [1, 2])

    def test_sidecars_config_and_credentials_not_wanted(self):
        parser = QwenCodeParser()
        wanted = {
            artifact.rel
            for artifact in self.col.artifacts
            if artifact.agent == "qwen-code" and parser.wants(artifact)
        }
        self.assertEqual(
            wanted,
            {
                self.REL,
                self.LOGS,
                ".qwen/projects/-srv-proj/chats/archive/%s.jsonl" % QWEN_ARCHIVED,
                ".qwen/trustedFolders.json",
                ".qwen/settings.json",
                self.CHANNELS,
                self.ROUTES,
                QWEN_TOKEN_USAGE_REL,
                QWEN_GROUP_HISTORY_REL,
                QWEN_GROUP_REL,
                QWEN_TODO_REL,
                QWEN_TODO_DEFAULT_REL,
                QWEN_ORGANIZATION_REL,
                QWEN_PR_REL,
                QWEN_AUDIT_REL,
                QWEN_SUMMARY_REL,
                QWEN_HARNESS_REL,
                QWEN_WORKER_REL,
                QWEN_WORKER_USAGE_REL,
            },
        )
        # The ledger and runtime sidecars, the GitHub queue beside the audit
        # and the managed H2 database are left unread.
        for rel in (
            ".qwen/projects/-srv-proj/chats/%s.ledger.jsonl" % QWEN_SESSION,
            ".qwen/projects/-srv-proj/chats/%s.runtime.json" % QWEN_SESSION,
            QWEN_AUDIT_REL.replace("github-audit.jsonl", "github-inbound-tasks.json"),
            ".qwen/managed-local/database/managed.mv.db",
        ):
            self.assertFalse(parser.wants(self.artifact(rel)), rel)

    def test_truncated_last_line_is_reported_not_fatal(self):
        # The bad line is its own row, in full; the rows before
        # it and a line written after it survive.
        from fixtures import qwen_session_records

        before = self.rows_for(QwenCodeParser(), self.REL)
        self.assertEqual(len(before), 14)
        write_bad_line(self.home / self.REL)
        after: dict = dict(
            qwen_session_records()[1],
            uuid="q9",
            message={"role": "user", "parts": [{"text": "after the cut"}]},
            systemPayload={"displayText": "after the cut"},
        )
        with open(self.home / self.REL, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(after) + "\n")
        rows = self.rows_for(QwenCodeParser(), self.REL)
        self.assertEqual(len(rows), 16)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[:13]],
            [(row.source_line, row.turn_type, row.text) for row in before[:13]],
        )
        self.assertEqual(
            [(row.source_line, row.turn_type, row.session_id, row.text) for row in rows[13:]],
            [
                (11, "user", QWEN_SESSION, "after the cut"),
                (1, "system", QWEN_SESSION, before[13].text),
                (10, "system", QWEN_SESSION, bad_line_text()),
            ],
        )

    def test_worktree_sidecar_row(self):
        # The session with a .worktree.json sidecar names its worktree once, at
        # its first record; the archived session, without one, names none.
        from fixtures import QWEN_WORKTREE

        rows = self.rows_for(QwenCodeParser(), self.REL)
        scratch = [row for row in rows if row.text.startswith("scratch directory:")]
        self.assertEqual(
            [
                (
                    row.turn_type,
                    row.text,
                    row.session_id,
                    row.project_path,
                    row.timestamp_utc,
                    row.source_line,
                )
                for row in scratch
            ],
            [
                (
                    "system",
                    "scratch directory: %s (enter_worktree fix-tests)" % QWEN_WORKTREE,
                    QWEN_SESSION,
                    QWEN_WORKTREE,
                    "2026-10-01T09:59:59.000Z",
                    1,
                )
            ],
        )
        archived = ".qwen/projects/-srv-proj/chats/archive/%s.jsonl" % QWEN_ARCHIVED
        self.assertFalse(
            [
                row
                for row in self.rows_for(QwenCodeParser(), archived)
                if row.text.startswith("scratch directory:") or row.text.startswith("parser:")
            ]
        )

    def test_worktree_sidecar_not_an_object(self):
        sidecar = "%s.worktree.json" % QWEN_SESSION
        (self.home / self.REL).with_name(sidecar).write_text("[1]", encoding="utf-8")
        rows = self.rows_for(QwenCodeParser(), self.REL)
        self.assertFalse([row for row in rows if row.text.startswith("scratch directory:")])
        self.assertEqual(
            [
                (row.source_line, row.session_id, row.project_path, row.timestamp_utc, row.text)
                for row in rows
                if row.text.startswith("parser:")
            ],
            [(0, QWEN_SESSION, "/srv/proj", "", "parser: %s is not a JSON object" % sidecar)],
        )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_worktree_sidecar_unreadable(self):
        sidecar = "%s.worktree.json" % QWEN_SESSION
        path = (self.home / self.REL).with_name(sidecar)
        path.chmod(0)
        try:
            rows = self.rows_for(QwenCodeParser(), self.REL)
        finally:
            path.chmod(0o600)
        self.assertEqual(
            [(row.source_line, row.session_id, row.text) for row in rows if "parser:" in row.text],
            [(0, QWEN_SESSION, "parser: %s could not be read (Permission denied)" % sidecar)],
        )
        self.assertEqual(len(rows), 14)  # every transcript row is still yielded

    def test_truncated_logs_keep_earlier_entries(self):
        # The broken element is carried from where its decode started to
        # the end of the file; the entries before it survive.
        path = self.home / self.LOGS
        data = path.read_text(encoding="utf-8")
        cut = data[: data.index('"model_switch"')]
        path.write_text(cut, encoding="utf-8")
        start = cut.rindex("{")
        try:
            json.JSONDecoder().raw_decode(cut, start)
        except ValueError as error:
            reason = str(error)
        else:
            self.fail("the cut element parsed")
        rows = self.rows_for(QwenCodeParser(), self.LOGS)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                (1, "user", "run the tests"),
                (2, "system", "parser: unparseable record 2 (%s): %s" % (reason, cut[start:])),
            ],
        )

    def test_logs_not_an_array(self):
        (self.home / self.LOGS).write_text('{"type": "user"}', encoding="utf-8")
        rows = self.rows_for(QwenCodeParser(), self.LOGS)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [(0, "system", "parser: not a JSON array")],
        )

    def test_args_summary(self):
        self.assertEqual(args_summary({"command": "ls"}), "ls")
        self.assertEqual(args_summary({"other": 1}), '{"other":1}')
        self.assertEqual(args_summary(None), "")

    def permission_rows(self, rel):
        """Return the rows of `rel` from the permission fixture.

        The fixture holds the permission, approval-mode and usage records; the
        collection is reopened after it is built.
        """
        from fixtures import build_qwen_permissions

        build_qwen_permissions(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        return self.rows_for(QwenCodeParser(), rel)

    def test_execution_context_and_decision_rows(self):
        # executionContext model and approval mode on change, the daemon's
        # session_approval_mode record, and the ui_telemetry decision.
        from fixtures import QWEN_PERM_REL, QWEN_PERM_SESSION

        rows = self.permission_rows(QWEN_PERM_REL)
        others = [row for row in rows if row.turn_type != "tool_use"]
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text, row.model) for row in others],
            [
                (1, "system", "model: qwen3-coder-plus auth=qwen-oauth", "qwen3-coder-plus"),
                (1, "system", "permission mode: default", ""),
                (1, "user", "clean the build", ""),
                (3, "system", others[3].text, ""),
                (3, "system", "permission decision: auto_accept (tool_call ui_telemetry)", ""),
                (4, "system", "permission mode: default -> plan prePlanMode=default", ""),
                (
                    5,
                    "system",
                    "model: qwen3-coder-plus -> qwen3-max auth=qwen-oauth",
                    "qwen3-max",
                ),
                (5, "system", "permission mode: plan -> yolo", ""),
                (5, "user", "now ship it", ""),
            ],
        )
        self.assertTrue(others[3].text.startswith("ui_telemetry: "), others[3].text)
        use = next(row for row in rows if row.turn_type == "tool_use")
        decision = others[4]
        self.assertEqual(
            (decision.tool_name, decision.tool_use_id, decision.timestamp_utc),
            ("run_shell_command", use.tool_use_id, "2026-10-03T09:00:09.000Z"),
        )
        self.assertEqual(use.tool_use_id, "call_p1")
        self.assertEqual({row.session_id for row in rows}, {QWEN_PERM_SESSION})

    def test_decision_row_bare_event_name(self):
        # The ToolCallEvent class's bare default name `tool_call` is matched
        # as well as `qwen-code.tool_call`.
        from fixtures import QWEN_BARE_REL, QWEN_BARE_SESSION

        rows = self.permission_rows(QWEN_BARE_REL)
        decisions = [row for row in rows if row.text.startswith("permission decision:")]
        self.assertEqual(
            [(row.text, row.tool_name, row.tool_use_id, row.timestamp_utc) for row in decisions],
            [
                (
                    "permission decision: reject (tool_call ui_telemetry)",
                    "write_file",
                    "call_b1",
                    "2026-10-03T10:00:05.000Z",
                )
            ],
        )
        self.assertEqual({row.session_id for row in rows}, {QWEN_BARE_SESSION})

    def test_settings_permission_rows(self):
        # Standing rules and the approval mode of settings.json, read as
        # JSONC; nothing else in the file is read.
        from fixtures import QWEN_SETTINGS_MTIME

        rel = ".qwen/settings.json"
        rows = self.permission_rows(rel)
        self.assertEqual(
            [(row.turn_type, row.text, row.session_id, row.source_line) for row in rows],
            [
                (
                    "system",
                    "permission setting: permissions.%s (from .qwen/settings.json, file mtime)"
                    % entry,
                    "",
                    1,
                )
                for entry in (
                    "allow=Bash(git *)",
                    "allow=ReadFileTool",
                    "deny=Bash(rm *)",
                )
            ]
            + [
                (
                    "system",
                    "permission setting: tools.approvalMode=auto-edit"
                    " (from .qwen/settings.json, file mtime)",
                    "",
                    1,
                )
            ],
        )
        self.assertEqual({row.timestamp_utc for row in rows}, {to_utc(QWEN_SETTINGS_MTIME)})
        self.assertFalse([row for row in rows if "secret" in row.text])
        self.assertFalse(QwenCodeParser().secondary(self.artifact(rel)))
        # A whole-file row: no line and no time of its own.
        (self.home / rel).write_text("[1]", encoding="utf-8")
        self.assertEqual(
            [
                (row.source_line, row.timestamp_utc, row.text)
                for row in self.rows_for(QwenCodeParser(), rel)
            ],
            [(0, "", "parser: .qwen/settings.json is not a JSON object")],
        )
        # The fixture home's empty settings file yields nothing.
        (self.home / rel).write_text("{}", encoding="utf-8")
        self.assertEqual(self.rows_for(QwenCodeParser(), rel), [])

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_standing_files_are_not_misread_as_wrong_shape(self):
        for rel in (".qwen/settings.json", ".qwen/trustedFolders.json"):
            path = self.home / rel
            path.chmod(0)
            try:
                rows = self.rows_for(QwenCodeParser(), rel)
            finally:
                path.chmod(0o600)
            self.assertEqual(
                [(row.source_line, row.turn_type, row.text) for row in rows],
                [(0, "system", "parser: %s could not be read (Permission denied)" % rel[6:])],
                rel,
            )

    def test_usage_record(self):
        # usage_record.jsonl is a secondary source that keeps a deleted
        # session's summary.
        from fixtures import QWEN_DELETED_SESSION, QWEN_USAGE_REL

        rows = self.permission_rows(QWEN_USAGE_REL)
        self.assertTrue(QwenCodeParser().secondary(self.artifact(QWEN_USAGE_REL)))
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
                (
                    1,
                    "system",
                    "2026-10-02T09:00:00.000Z",
                    QWEN_DELETED_SESSION,
                    "/srv/proj",
                    "session usage: project /srv/proj duration 3600000ms models"
                    " qwen3-coder-plus tools run_shell_command=2,read_file=1"
                    " (from usage_record.jsonl, startTime)",
                )
            ],
        )

    def test_usage_record_truncated_line(self):
        # The cut line is its own row; the lines before and after it are
        # still read.
        from fixtures import QWEN_USAGE_REL, qwen_usage_records

        self.permission_rows(QWEN_USAGE_REL)
        path = self.home / QWEN_USAGE_REL
        write_bad_line(path)
        after = dict(qwen_usage_records()[0], sessionId="after-cut", models={}, tools={})
        with open(path, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(after) + "\n")
        rows = self.rows_for(QwenCodeParser(), QWEN_USAGE_REL)
        self.assertEqual(
            [(row.source_line, row.session_id) for row in rows],
            [(1, rows[0].session_id), (3, "after-cut"), (2, "")],
        )
        self.assertTrue(
            rows[1].text.endswith("models - tools - (from usage_record.jsonl, startTime)")
        )
        self.assertEqual(rows[2].text, bad_line_text())

    def test_usage_and_settings_wanted_registry_not(self):
        parser = QwenCodeParser()
        self.permission_rows(".qwen/settings.json")
        reg = self.home / ".qwen/sessions/4242.json"
        reg.parent.mkdir(parents=True, exist_ok=True)
        reg.write_text('{"sessionId": "x", "ipcToken": "x"}', encoding="utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        self.assertTrue(parser.wants(self.artifact(".qwen/usage_record.jsonl")))
        self.assertTrue(parser.wants(self.artifact(".qwen/settings.json")))
        self.assertFalse(parser.wants(self.artifact(".qwen/sessions/4242.json")))
        self.assertFalse(parser.wants(self.artifact(".qwen/oauth_creds.json")))

    CHANNELS = ".qwen/channels/sessions.json"
    ROUTES = ".qwen/channels/daemon/%s/routes.json" % QWEN_DAEMON_HASH

    def test_channel_routes(self):
        # A session driven from a messaging platform is relayed there.
        parser = QwenCodeParser()
        rows = self.rows_for(parser, self.CHANNELS) + self.rows_for(parser, self.ROUTES)
        self.assertEqual(
            [
                (
                    row.turn_type,
                    row.timestamp_utc,
                    row.session_id,
                    row.project_path,
                    row.source_line,
                    row.text,
                )
                for row in rows
            ],
            [
                (
                    "system",
                    "2026-10-01T10:00:00.000Z",
                    QWEN_SESSION,
                    "/srv/proj",
                    1,
                    "session relayed: channel telegram chat -1001234 sender 42 thread -"
                    " (from channels/sessions.json, startedAt)",
                ),
                (
                    "system",
                    "2026-10-07T10:00:00.000Z",
                    QWEN_DAEMON_SESSION,
                    "/srv/proj",
                    1,
                    "session relayed: channel github chat octo/proj#7 sender octocat thread c-991"
                    " (from channels/daemon/%s/routes.json, file mtime)" % QWEN_DAEMON_HASH,
                ),
            ],
        )
        self.assertFalse(parser.records_sync)
        self.assertFalse(parser.secondary(self.artifact(self.CHANNELS)))
        self.assertFalse(parser.wants(self.artifact(".qwen/channels/1791367200000-4242-ab12.tmp")))

    def test_session_without_route_has_no_relay_row(self):
        rel = ".qwen/projects/-srv-proj/chats/archive/%s.jsonl" % QWEN_ARCHIVED
        rows = self.rows_for(QwenCodeParser(), rel)
        self.assertFalse([row for row in rows if row.text.startswith("session relayed:")])

    def test_channel_routes_not_an_object(self):
        path = self.home / self.CHANNELS
        path.write_text('{"telegram:1": {"sessionId": "s', encoding="utf-8")
        self.assertEqual(
            [(row.source_line, row.text) for row in self.rows_for(QwenCodeParser(), self.CHANNELS)],
            [(0, "parser: channels/sessions.json is not a JSON object")],
        )
        path.write_text(
            json.dumps({"a": [], "b": {"sessionId": "s1", "target": {"channelName": "qq"}}}),
            encoding="utf-8",
        )
        self.assertEqual(
            [(row.source_line, row.text) for row in self.rows_for(QwenCodeParser(), self.CHANNELS)],
            [
                (
                    2,
                    "session relayed: channel qq chat - sender - thread - (from"
                    " channels/sessions.json, file mtime)",
                ),
                (1, "parser: entry a is not a JSON object"),
            ],
        )

    def test_trusted_folders_jsonc(self):
        # A byte-order mark, comments and trailing commas, which Qwen Code's
        # parseJsoncObject accepts, decode; a comma inside a string is kept. A
        # file still undecodable is the whole-file row.
        from fixtures import TRUST_JSONC

        rel = ".qwen/trustedFolders.json"
        path = self.home / rel
        path.write_bytes(TRUST_JSONC)
        os.utime(path, (QWEN_TRUST_MTIME, QWEN_TRUST_MTIME))
        rows = self.rows_for(QwenCodeParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.text, row.project_path, row.source_line) for row in rows],
            [
                (
                    "system",
                    "permission setting: trustedFolders./srv/a,]=TRUST_FOLDER "
                    "(from .qwen/trustedFolders.json, file mtime)",
                    "/srv/a,]",
                    1,
                ),
                (
                    "system",
                    "permission setting: trustedFolders./srv/b=DO_NOT_TRUST "
                    "(from .qwen/trustedFolders.json, file mtime)",
                    "/srv/b",
                    1,
                ),
            ],
        )
        self.assertEqual({row.timestamp_utc for row in rows}, {to_utc(QWEN_TRUST_MTIME)})
        path.write_bytes(TRUST_JSONC.replace(b'"DO_NOT_TRUST",', b'"DO_NOT_TRUST",,'))
        self.assertEqual(
            [
                (row.turn_type, row.text, row.source_line)
                for row in self.rows_for(QwenCodeParser(), rel)
            ],
            [("system", "parser: .qwen/trustedFolders.json is not a JSON object", 1)],
        )

    def injected_rows(self, bad_line=False):
        """Return the rows of the injected-records session.

        The session is built and the collection reopened; with `bad_line`, a cut line
        and a cron record are written after it.
        """
        from fixtures import QWEN_INJECTED_REL, build_qwen_injected, qwen_injected_records

        build_qwen_injected(self.home)
        path = self.home / QWEN_INJECTED_REL
        if bad_line:
            write_bad_line(path)
            after: dict = dict(qwen_injected_records()[0], uuid="i6")
            after["message"] = {"role": "user", "parts": [{"text": "after the cut"}]}
            with open(path, "a", encoding="utf-8") as handle:
                handle.write("\n" + json.dumps(after) + "\n")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        return self.rows_for(QwenCodeParser(), QWEN_INJECTED_REL)

    def test_injected_user_records_are_system_rows(self):
        # Cron and notification records (provenance system) and another
        # agent's message or mention (provenance external_agent) are not
        # prompts the user typed; the user's own mention is.
        from fixtures import QWEN_INJECTED_SESSION

        rows = self.injected_rows()
        self.assertEqual(
            [(row.source_line, row.turn_type, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    1,
                    "system",
                    "2026-10-02T09:00:00.000Z",
                    "cron (provenance=system): check the nightly build",
                ),
                (
                    2,
                    "system",
                    "2026-10-02T09:01:00.000Z",
                    "notification (provenance=system): Background shell finished",
                ),
                (3, "user", "2026-10-02T09:02:00.000Z", "@Reviewer look at the diff"),
                (
                    4,
                    "system",
                    "2026-10-02T09:03:00.000Z",
                    "agent_message (provenance=external_agent agent=Reviewer"
                    " agentId=agt-reviewer): The diff deletes the tests.",
                ),
                (
                    5,
                    "system",
                    "2026-10-02T09:04:00.000Z",
                    "agent_mention (provenance=external_agent agent=Reviewer"
                    " agentId=agt-reviewer): @Fixer restore the tests",
                ),
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.session_id, row.project_path, row.git_branch),
                (QWEN_INJECTED_SESSION, "/srv/proj", "main"),
            )

    def test_injected_origin_without_provenance(self):
        # A cron record that lost its provenance is still not the user's;
        # a plain user record without one is.
        self.assertEqual(injected_origin({"type": "user", "subtype": "cron"}), "cron")
        self.assertEqual(injected_origin({"type": "user"}), "")
        self.assertEqual(
            injected_origin(
                {"type": "user", "subtype": "goal_runtime", "provenance": "goal_runtime"}
            ),
            "goal_runtime (provenance=goal_runtime)",
        )

    def test_injected_records_truncated_line(self):
        # The cut line is its own row in full; the records before it and
        # the one written after it still come out.
        from fixtures import QWEN_INJECTED_SESSION

        rows = self.injected_rows(bad_line=True)
        self.assertEqual(len(rows), 7)
        self.assertEqual([row.source_line for row in rows[:5]], [1, 2, 3, 4, 5])
        self.assertEqual(
            [
                (row.source_line, row.turn_type, row.session_id, row.timestamp_utc, row.text)
                for row in rows[5:]
            ],
            [
                (
                    7,
                    "system",
                    QWEN_INJECTED_SESSION,
                    "2026-10-02T09:00:00.000Z",
                    "cron (provenance=system): after the cut",
                ),
                (6, "system", QWEN_INJECTED_SESSION, "", bad_line_text()),
            ],
        )
        self.assertEqual(rows[6].project_path, "/srv/proj")

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_file_is_one_row(self):
        unlocked = parse_with(self, QwenCodeParser)
        # (file made unreadable, the session id of its row)
        for rel, session_id in (
            (".qwen/channels/sessions.json", ""),
            (
                ".qwen/tmp/3f0a9c0000000000000000000000000000000000000000000000000000000000/logs.json",
                "",
            ),
            (QWEN_TOKEN_USAGE_REL, ""),
            (QWEN_GROUP_HISTORY_REL, ""),
            (QWEN_TODO_REL, QWEN_SESSION),
            (QWEN_ORGANIZATION_REL, ""),
            (QWEN_PR_REL, QWEN_SESSION),
            (QWEN_AUDIT_REL, ""),
            (QWEN_SUMMARY_REL, ""),
            (QWEN_WORKER_REL, ""),
        ):
            rows = parse_with(self, QwenCodeParser, rel)
            text = "parser: %s could not be read (Permission denied)" % rel.split("/")[-1]
            self.assertEqual(
                [
                    (
                        row.source_file.endswith(rel),
                        row.source_line,
                        row.timestamp_utc,
                        row.session_id,
                        row.text,
                    )
                    for row in unreadable_rows(rows)
                ],
                [(True, 0, "", session_id, text)],
                rel,
            )
            # Every other file of the parser still gives its rows.
            self.assertEqual(kept_rows(rows, rel), kept_rows(unlocked, rel), rel)

    def rows_of(self, rel):
        """Return `(source_line, turn_type, timestamp_utc, session_id, text)` of a file's rows."""
        return [
            (row.source_line, row.turn_type, row.timestamp_utc, row.session_id, row.text)
            for row in self.rows_for(QwenCodeParser(), rel)
        ]

    def test_token_usage_rows(self):
        # One row per response, primary, with its session and model.
        parser = QwenCodeParser()
        self.assertFalse(parser.secondary(self.artifact(QWEN_TOKEN_USAGE_REL)))
        rows = self.rows_for(parser, QWEN_TOKEN_USAGE_REL)
        self.assertEqual(
            [(row.source_line, row.timestamp_utc, row.session_id, row.model) for row in rows],
            [
                (1, "2026-10-01T10:00:03.000Z", QWEN_SESSION, "qwen3-coder-plus"),
                (2, "2026-10-01T10:01:00.000Z", QWEN_SESSION, ""),
            ],
        )
        # `unknown`, a response naming no model, stays in the text only.
        self.assertTrue(rows[1].text.startswith("token usage: model unknown auth unknown"))
        self.assertEqual(
            rows[0].text,
            "token usage: model qwen3-coder-plus auth qwen-oauth source main tokens input 200"
            " output 30 cached 0 thoughts 12 total 242 api 2900ms (from token-usage-2026-10.jsonl)",
        )
        self.assertEqual({row.turn_type for row in rows}, {"system"})
        self.assertFalse([row for row in rows if row.text.startswith("model:")])

    def test_token_usage_truncated_line(self):
        path = self.home / QWEN_TOKEN_USAGE_REL
        write_bad_line(path)
        with open(path, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps({"sessionId": "after-cut"}) + "\n")
        rows = self.rows_of(QWEN_TOKEN_USAGE_REL)
        self.assertEqual([row[0] for row in rows], [1, 2, 4, 3])
        self.assertEqual(
            rows[2][3:],
            (
                "after-cut",
                "token usage: model - auth - source - tokens input - output - cached - thoughts -"
                " total - api - (from token-usage-2026-10.jsonl)",
            ),
        )
        self.assertEqual(rows[3][4], bad_line_text())

    def test_group_history_rows(self):
        # The history is a secondary source with no session or project; each
        # message names the `clear` of its key that followed it, if any.
        parser = QwenCodeParser()
        self.assertTrue(parser.secondary(self.artifact(QWEN_GROUP_HISTORY_REL)))
        rows = self.rows_for(parser, QWEN_GROUP_HISTORY_REL)
        cleared = " (cleared 2026-10-03T08:06:00.000Z)"
        self.assertEqual(
            [(row.source_line, row.timestamp_utc, row.text) for row in rows],
            [
                (1, "2026-10-03T08:00:00.000Z", BOB_DEPLOY + cleared),
                (2, "2026-10-03T08:05:00.000Z", "group message: [43] rollback?" + cleared),
                (3, "2026-10-03T08:06:00.000Z", self.CLEARED[1]),
                (4, "2026-10-03T08:20:00.000Z", self.WAITING[1]),
            ],
        )
        self.assertEqual({(row.session_id, row.project_path) for row in rows}, {("", "")})
        self.assertEqual({row.turn_type for row in rows}, {"system"})

    def test_group_history_bad_lines(self):
        path = self.home / QWEN_GROUP_HISTORY_REL
        with open(path, "a", encoding="utf-8") as handle:
            handle.write(json.dumps({"type": "message", "key": "k"}) + "\n")
        write_bad_line(path)
        with open(path, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(qwen_group_history_records()[2]) + "\n")
        rows = self.rows_of(QWEN_GROUP_HISTORY_REL)
        self.assertEqual([row[0] for row in rows], [1, 2, 3, 4, 7, 5, 6])
        self.assertEqual(
            rows[5][4],
            'parser: unparseable line (not a group history record): {"key":"k","type":"message"}',
        )
        self.assertEqual(rows[6][4], bad_line_text())
        # The clear written after the cut still closes the waiting message.
        self.assertTrue(rows[3][4].endswith("(cleared 2026-10-03T08:06:00.000Z)"), rows[3][4])
        # A later clear without a time still closes it, at no known time.
        records = qwen_group_history_records()
        records[2].pop("recordedAt")
        _jsonl(path, records)
        rows = self.rows_of(QWEN_GROUP_HISTORY_REL)
        self.assertEqual(rows[0][4], BOB_DEPLOY + " (cleared, time not recorded)")
        self.assertEqual(rows[3][4], self.WAITING[1])
        self.assertEqual(rows[2][2], "")
        self.write_group(history=records)
        self.assertIn(
            ("telegram-group-history.jsonl", BOB_DEPLOY + " (cleared, time not recorded)"),
            self.group_dedup(),
        )

    def test_quoted_group_messages(self):
        # The transcript's channel prompt gives one row per quoted line,
        # after the prompt's own row and at its time.
        rows = self.rows_of(QWEN_GROUP_REL)
        self.assertEqual(
            [(row[0], row[1], row[4]) for row in rows[1:]],
            [
                (2, "user", rows[1][4]),
                (2, "system", "group message: [Bob] the deploy is broken"),
                (2, "system", "group message: [43] rollback?"),
            ],
        )
        self.assertEqual({row[2] for row in rows[1:]}, {"2026-10-03T08:06:00.500Z"})
        self.assertEqual({row[3] for row in rows}, {QWEN_GROUP_SESSION})
        self.assertEqual(
            rows[0][1:], ("user", "2026-10-03T07:59:00.000Z", QWEN_GROUP_SESSION, "hello")
        )
        # A block missing its closing marker, or holding a line of another
        # form, is not the channel's quote.
        block = "[Chat messages since your last reply - for context]\n- [a] b\n"
        self.assertEqual(quoted_group_messages(block + "\nhi"), [])
        self.assertEqual(
            quoted_group_messages(block + "x\n\n[Current message - respond to this]\nhi"), []
        )
        self.assertEqual(
            quoted_group_messages("ctx\n\n" + block + "\n[Current message - respond to this]\nhi"),
            ["group message: [a] b"],
        )
        self.assertEqual(
            quoted_group_messages("> " + block + "\n[Current message - respond to this]\nhi"), []
        )

    def group_dedup(self):
        """Return `(file name, text)` of the group history rows deduplication keeps.

        The transcripts' rows are primary and the history files' secondary,
        as the analyzer passes them.
        """
        parser = QwenCodeParser()
        primary = []
        for path in sorted((self.home / ".qwen/projects/-srv-proj/chats").glob("*.jsonl")):
            if not path.name.endswith(".ledger.jsonl"):
                primary += self.rows_for(parser, path.relative_to(self.home).as_posix())
        secondary = []
        for path in sorted((self.home / ".qwen/channels").glob("*-group-history.jsonl")):
            artifact = self.artifact(path.relative_to(self.home).as_posix())
            tolerance = parser.secondary_tolerance(artifact)
            secondary += [(row, tolerance) for row in parser.parse(artifact, Options())]
        kept, _ = deduplicate(primary, secondary)
        return [(row.source_file.rsplit("/", 1)[-1], row.text) for row in kept[len(primary) :]]

    def write_group(self, history=None, session=None, extra=None):
        """Rewrite the group history and its channel session, and reopen the collection.

        `extra` maps a further file, relative to the home, to its records.
        """
        if history is not None:
            _jsonl(self.home / QWEN_GROUP_HISTORY_REL, history)
        if session is not None:
            _jsonl(self.home / QWEN_GROUP_REL, session)
        for rel, records in (extra or {}).items():
            _jsonl(self.home / rel, records)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")

    def quote_at(self, timestamp, lines=("- [Bob] the deploy is broken",), session_id=""):
        """Return a channel session whose one prompt quotes `lines` at `timestamp`."""
        record = qwen_group_session_records()[1]
        record["timestamp"] = timestamp
        if session_id:
            record["sessionId"] = session_id
        record["message"]["parts"][0]["text"] = (
            "[Chat messages since your last reply - for context]\n%s\n\n"
            "[Current message - respond to this]\n@bot ?" % "\n".join(lines)
        )
        return [record]

    CLEARED = (
        "telegram-group-history.jsonl",
        'group history cleared: ["telegram", "-1001234", null]'
        " (from telegram-group-history.jsonl, recordedAt)",
    )
    WAITING = ("telegram-group-history.jsonl", "group message: [Bob] fixed now (waiting)")
    DEPLOY = (
        "telegram-group-history.jsonl",
        BOB_DEPLOY + " (cleared 2026-10-03T08:06:00.000Z)",
    )
    ROLLBACK = (
        "telegram-group-history.jsonl",
        "group message: [43] rollback? (cleared 2026-10-03T08:06:00.000Z)",
    )

    def test_group_history_dedup_drops_only_quoted_messages(self):
        # The two messages the prompt quotes are dropped; the waiting one and
        # the clear row only the history holds are kept.
        self.assertEqual(self.group_dedup(), [self.CLEARED, self.WAITING])

    def test_group_history_kept_without_its_transcript(self):
        (self.home / QWEN_GROUP_REL).unlink()
        self.write_group()
        self.assertEqual(
            self.group_dedup(), [self.DEPLOY, self.ROLLBACK, self.CLEARED, self.WAITING]
        )

    def test_group_history_matched_at_its_clear(self):
        # The quote matches within 2 seconds of the key's clear, before or
        # after it, never at the message's own time.
        for timestamp, matched in (
            ("2026-10-03T08:05:59.000Z", True),
            ("2026-10-03T08:05:57.000Z", False),
            ("2026-10-03T08:06:01.500Z", True),
            ("2026-10-03T08:06:03.000Z", False),
            ("2026-10-03T08:00:00.000Z", False),
        ):
            self.write_group(session=self.quote_at(timestamp))
            kept = self.group_dedup()
            self.assertEqual(self.DEPLOY not in kept, matched, timestamp)
            self.assertIn(self.ROLLBACK, kept, timestamp)

    def test_group_history_waiting_message_matches_nothing(self):
        # No clear follows the waiting message, so no prompt quoted it, even
        # one with its text written after it.
        session = self.quote_at("2026-10-03T08:20:00.500Z", ("- [Bob] fixed now",))
        self.write_group(session=session)
        self.assertIn(self.WAITING, self.group_dedup())

    def test_group_history_quote_long_after_the_clear(self):
        # The same text quoted by another chat's session an hour after this
        # chat's clear is out of the window; its own session's quote is in it.
        other = self.quote_at("2026-10-03T09:00:00.000Z", session_id=QWEN_OTHER_CHAT_SESSION)
        self.write_group(
            session=[],
            extra={".qwen/projects/-srv-proj/chats/%s.jsonl" % QWEN_OTHER_CHAT_SESSION: other},
        )
        self.assertIn(self.DEPLOY, self.group_dedup())
        self.write_group(session=qwen_group_session_records())
        self.assertNotIn(self.DEPLOY, self.group_dedup())

    def test_group_history_any_scope_within_the_clear(self):
        # The chat's key never reaches the transcript, so another chat's
        # session quoting the same text within 2 seconds of this clear takes it.
        other = self.quote_at("2026-10-03T08:06:01.000Z", session_id=QWEN_OTHER_CHAT_SESSION)
        self.write_group(
            session=[],
            extra={".qwen/projects/-srv-proj/chats/%s.jsonl" % QWEN_OTHER_CHAT_SESSION: other},
        )
        self.assertNotIn(self.DEPLOY, self.group_dedup())

    def test_group_history_earlier_and_later_claimants(self):
        # Bob says the same thing again after the first quote, and a second
        # prompt quotes the repeat. Each line takes the quote at its own clear.
        history = qwen_group_history_records()[:3]
        repeat = json.loads(json.dumps(history[0]))
        repeat["entry"].update(messageId="m-2", timestamp=QWEN_GROUP_TIMES[2])
        clear = dict(history[2], recordedAt=QWEN_GROUP_TIMES[2] + 60000)
        later = (
            "telegram-group-history.jsonl",
            BOB_DEPLOY + " (cleared 2026-10-03T08:21:00.000Z)",
        )
        quotes = self.quote_at("2026-10-03T08:06:00.500Z") + self.quote_at(
            "2026-10-03T08:21:00.200Z"
        )
        self.write_group(history=[*history, repeat, clear], session=quotes)
        kept = self.group_dedup()
        self.assertNotIn(self.DEPLOY, kept)
        self.assertNotIn(later, kept)
        # The later claimant missing: the earlier line still matches its own quote.
        self.write_group(history=history, session=quotes)
        self.assertNotIn(self.DEPLOY, self.group_dedup())
        # The earlier claimant missing: the repeat takes only its own quote.
        self.write_group(history=[repeat, clear], session=quotes[:1])
        self.assertIn(later, self.group_dedup())

    def test_group_history_multiline_message(self):
        # The channel quotes a line break as a space; whitespace runs match as one.
        history = qwen_group_history_records()
        history[0]["entry"]["text"] = "the deploy\n\nis   broken"
        self.write_group(history=history)
        self.assertNotIn(
            (
                "telegram-group-history.jsonl",
                "group message: [Bob] the deploy\n\nis   broken (cleared 2026-10-03T08:06:00.000Z)",
            ),
            self.group_dedup(),
        )
        self.assertEqual(len(self.group_dedup()), 2)

    def test_group_history_sanitised_quote_keeps_both(self):
        # The channel quotes `"` as a space; the analyzer never blanks what a
        # member typed, so the history line and the quote both stay.
        history = qwen_group_history_records()
        history[0]["entry"]["text"] = 'the "deploy" is broken'
        session = qwen_group_session_records()
        part = session[1]["message"]["parts"][0]
        part["text"] = part["text"].replace("the deploy is broken", "the  deploy  is broken")
        self.write_group(history=history, session=session)
        kept = self.group_dedup()
        self.assertIn(
            (
                "telegram-group-history.jsonl",
                'group message: [Bob] the "deploy" is broken (cleared 2026-10-03T08:06:00.000Z)',
            ),
            kept,
        )
        self.assertEqual(len(kept), 3)

    def test_group_history_same_key_and_another_key(self):
        # A copy of the file holds the same key's lines: one event each. Another
        # channel's file is another key, cleared at its own time: other events.
        records = qwen_group_history_records()
        other = json.loads(json.dumps(records[:3]))
        for record in other:
            record["key"] = json.dumps(["qq", "-1001234", None])
        other[2]["recordedAt"] += 3600000
        self.write_group(
            extra={
                ".qwen/channels/telegram-copy-group-history.jsonl": records,
                ".qwen/channels/qq-group-history.jsonl": other,
            }
        )
        kept = self.group_dedup()
        self.assertEqual(
            [name for name, text in kept if text.startswith(BOB_DEPLOY)],
            ["qq-group-history.jsonl"],
        )
        self.assertEqual(
            [name for name, text in kept if text.startswith("group message: [43]")],
            ["qq-group-history.jsonl"],
        )
        # The waiting line no prompt quoted is in both copies: one event, shown once.
        self.assertEqual(len([entry for entry in kept if entry[1] == self.WAITING[1]]), 1)

    def test_todo_rows(self):
        rows = self.rows_for(QwenCodeParser(), QWEN_TODO_REL)
        self.assertEqual(
            [(row.source_line, row.session_id, row.text) for row in rows],
            [
                (
                    1,
                    QWEN_SESSION,
                    "todo: [completed] Run the test suite (id t1, blocked by -, from todos/%s.json,"
                    " file mtime)" % QWEN_SESSION,
                ),
                (
                    2,
                    QWEN_SESSION,
                    "todo: [in_progress] Fix the failing test (id t2, blocked by -, from"
                    " todos/%s.json, file mtime)" % QWEN_SESSION,
                ),
                (
                    3,
                    QWEN_SESSION,
                    "todo: [pending] Open a pull request (id t3, blocked by t2, from"
                    " todos/%s.json, file mtime)" % QWEN_SESSION,
                ),
            ],
        )
        self.assertEqual({row.timestamp_utc for row in rows}, {to_utc(QWEN_PAST_USE_MTIME)})
        self.assertEqual(self.rows_for(QwenCodeParser(), QWEN_TODO_DEFAULT_REL), [])

    def test_todo_file_errors(self):
        path = self.home / QWEN_TODO_REL
        path.write_text(
            json.dumps({"todos": ["x", {"content": "c"}], "sessionId": "default"}), encoding="utf-8"
        )
        self.assertEqual(
            [(row[0], row[3], row[4]) for row in self.rows_of(QWEN_TODO_REL)],
            [
                (
                    2,
                    "",
                    "todo: [-] c (id -, blocked by -, from todos/%s.json, file mtime)"
                    % QWEN_SESSION,
                ),
                (1, "", "parser: todo 1 is not a JSON object"),
            ],
        )
        path.write_text("[]", encoding="utf-8")
        self.assertEqual(
            [(row[0], row[3], row[4]) for row in self.rows_of(QWEN_TODO_REL)],
            [(0, QWEN_SESSION, "parser: %s.json is not a JSON object" % QWEN_SESSION)],
        )

    def test_session_organization_rows(self):
        rows = self.rows_of(QWEN_ORGANIZATION_REL)
        self.assertEqual(
            rows,
            [
                (
                    1,
                    "system",
                    "2026-10-01T11:05:00.000Z",
                    QWEN_SESSION,
                    "session organization: group Release color green pinned"
                    " 2026-10-01T11:05:00.000Z (from session-organization.v1.json, updatedAt)",
                ),
                (
                    2,
                    "system",
                    "2026-10-01T11:06:00.000Z",
                    QWEN_ARCHIVED,
                    "session organization: group - color - pinned no"
                    " (from session-organization.v1.json, updatedAt)",
                ),
            ],
        )
        path = self.home / QWEN_ORGANIZATION_REL
        path.write_text(json.dumps({"groups": [], "sessions": {"s1": 3}}), encoding="utf-8")
        self.assertEqual(
            [(row[0], row[4]) for row in self.rows_of(QWEN_ORGANIZATION_REL)],
            [(1, "parser: session s1 is not a JSON object")],
        )
        path.write_text("[]", encoding="utf-8")
        self.assertEqual(
            [(row[0], row[4]) for row in self.rows_of(QWEN_ORGANIZATION_REL)],
            [(0, "parser: session-organization.v1.json is not a JSON object")],
        )

    def test_pull_request_rows(self):
        name = "%s.pr.json" % QWEN_SESSION
        rows = self.rows_of(QWEN_PR_REL)
        self.assertEqual(
            rows,
            [
                (
                    1,
                    "system",
                    "2026-10-01T10:30:00.000Z",
                    QWEN_SESSION,
                    "source host: github.com repository alice/proj pull request"
                    " https://github.com/alice/proj/pull/12 state merged source create issues 7"
                    " (from %s, createdAt)" % name,
                ),
                (
                    2,
                    "system",
                    "2026-10-01T10:40:00.000Z",
                    QWEN_SESSION,
                    "source host: github.com repository upstream/proj pull request"
                    " https://github.com/upstream/proj/pull/3 state - source - issues -"
                    " (from %s, createdAt)" % name,
                ),
            ],
        )
        path = self.home / QWEN_PR_REL
        path.write_text(json.dumps({"prs": [1, {"url": "not a url"}]}), encoding="utf-8")
        self.assertEqual(
            [(row[0], row[3], row[4]) for row in self.rows_of(QWEN_PR_REL)],
            [
                (
                    2,
                    QWEN_SESSION,
                    "source host: - repository - pull request not a url state - source - issues -"
                    " (from %s, createdAt)" % name,
                ),
                (1, QWEN_SESSION, "parser: pull request 1 is not a JSON object"),
            ],
        )
        path.write_text("{", encoding="utf-8")
        self.assertEqual(
            [(row[0], row[3], row[4]) for row in self.rows_of(QWEN_PR_REL)],
            [(0, QWEN_SESSION, "parser: %s is not a JSON object" % name)],
        )

    def test_pull_requests_of_an_archived_session(self):
        archived = ".qwen/projects/-srv-proj/chats/archive/%s.pr.json" % QWEN_ARCHIVED
        path = self.home / archived
        path.write_text(
            json.dumps(
                {"prs": [{"number": 1, "url": "https://gitlab.com/a/b/-/merge_requests/1"}]}
            ),
            encoding="utf-8",
        )
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_of(archived)
        self.assertEqual(
            [(row[2], row[3], row[4].split(" pull request")[0]) for row in rows],
            [("", QWEN_ARCHIVED, "source host: gitlab.com repository a/b")],
        )

    def test_github_audit_rows(self):
        name = QWEN_AUDIT_REL.rsplit("/", 1)[-1]
        rows = self.rows_of(QWEN_AUDIT_REL)
        self.assertEqual(
            rows,
            [
                (
                    1,
                    "system",
                    "2026-10-07T10:00:05.000Z",
                    QWEN_DAEMON_SESSION,
                    "source host: github repository alice/proj actor octocat publication posted"
                    " trigger issue_comment number 7 comment"
                    " https://github.com/alice/proj/issues/7#issuecomment-5001 (from %s, at)"
                    % name,
                ),
                (
                    2,
                    "system",
                    "2026-10-07T10:02:00.000Z",
                    QWEN_DAEMON_SESSION,
                    "source host: github repository alice/proj actor - publication failed"
                    " trigger - number - comment - failure delivery: HTTP 403 (from %s, at)" % name,
                ),
            ],
        )
        self.assertFalse([row for row in rows if "0" * 64 in row[4]])
        write_bad_line(self.home / QWEN_AUDIT_REL)
        rows = self.rows_of(QWEN_AUDIT_REL)
        self.assertEqual([(row[0], row[4]) for row in rows[2:]], [(3, bad_line_text())])
        self.assertEqual(len(rows), 3)

    def test_project_summary_row(self):
        # The whole file, at its time, in the directory holding `.qwen/`.
        rows = self.rows_for(QwenCodeParser(), QWEN_SUMMARY_REL)
        artifact = self.artifact(QWEN_SUMMARY_REL)
        self.assertEqual(
            [
                (row.source_line, row.timestamp_utc, row.session_id, row.project_path, row.text)
                for row in rows
            ],
            [
                (
                    1,
                    to_utc(QWEN_PAST_USE_MTIME),
                    "",
                    artifact.home.original,
                    "project summary (from .qwen/PROJECT_SUMMARY.md, file mtime): # Project"
                    " summary\n\n- tests fixed",
                )
            ],
        )
        project = Artifact(
            disk_path=artifact.disk_path,
            original="/srv/proj/.qwen/PROJECT_SUMMARY.md",
            rel=QWEN_SUMMARY_REL,
            agent="qwen-code",
            home=replace(artifact.home, original="/srv/proj", project=True),
        )
        self.assertTrue(QwenCodeParser().wants(project))
        self.assertEqual(
            [row.project_path for row in QwenCodeParser().parse(project, Options())],
            ["/srv/proj"],
        )

    def test_managed_local_transcripts(self):
        # The harness's and the workers' transcripts are read as the home's.
        for rel, session_id in (
            (QWEN_HARNESS_REL, QWEN_HARNESS_SESSION),
            (QWEN_WORKER_REL, QWEN_WORKER_SESSION),
        ):
            self.assertEqual(
                [
                    (row.turn_type, row.session_id, row.project_path, row.text)
                    for row in self.rows_for(QwenCodeParser(), rel)
                ],
                [("user", session_id, "/srv/proj", "summarise the repository")],
                rel,
            )
            self.assertFalse(QwenCodeParser().secondary(self.artifact(rel)))
        # Settings under the worker's home are not the home's settings file.
        worker = ".qwen/managed-local/worker-home/"
        parser = QwenCodeParser()
        self.assertTrue(parser.wants(rel_only(worker + "tmp/abc/logs.json")))
        self.assertTrue(parser.secondary(rel_only(worker + "tmp/abc/logs.json")))
        self.assertTrue(parser.wants(rel_only(worker + "todos/x.json")))
        self.assertFalse(parser.wants(rel_only(worker + "settings.json")))
        # The workers' `$QWEN_HOME` keeps its own usage records; the harness,
        # which moves only its runtime base, has none.
        self.assertTrue(parser.wants(rel_only(worker + "usage_record.jsonl")))
        self.assertTrue(parser.secondary(rel_only(worker + "usage_record.jsonl")))
        harness = ".qwen/managed-local/harness/usage_record.jsonl"
        self.assertFalse(parser.wants(rel_only(harness)))
        rows = self.rows_for(parser, QWEN_WORKER_USAGE_REL)
        self.assertEqual(
            [(row.session_id, row.project_path) for row in rows],
            [(QWEN_WORKER_SESSION, "/srv/proj")],
        )
        self.assertTrue(rows[0].text.startswith("session usage: project /srv/proj"))
        self.assertFalse(parser.wants(rel_only(worker + "channels/sessions.json")))
        self.assertFalse(
            parser.wants(rel_only(".qwen/managed-local/projects/a/chats/%s.jsonl" % QWEN_SESSION))
        )
