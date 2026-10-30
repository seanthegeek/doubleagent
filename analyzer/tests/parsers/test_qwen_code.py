"""Tests of the Qwen Code parser."""

import json
import os

from doubleagent.inputs import open_input
from doubleagent.parsers.qwen_code import QwenCodeParser, args_summary, injected_origin
from doubleagent.timeutil import to_utc
from fixtures import (
    QWEN_ARCHIVED,
    QWEN_DAEMON_HASH,
    QWEN_DAEMON_SESSION,
    QWEN_SESSION,
    QWEN_TEMPORARY,
    QWEN_TRUST_MTIME,
    bad_line_text,
    write_bad_line,
)

from .base import ParserBase


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
            ],
        )
        for row in rows:
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
        self.assertEqual([row.source_line for row in rows], [1, 2, 3, 3, 3, 4, 5, 6, 7, 8, 8, 9, 9])

    def test_model_change_and_decision_rows(self):
        rows = self.rows_for(QwenCodeParser(), self.REL)
        change, use, result, decision = rows[9:]
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
            },
        )

    def test_truncated_last_line_is_reported_not_fatal(self):
        # The bad line is its own row, in full; the rows before
        # it and a line written after it survive.
        from fixtures import qwen_session_records

        before = self.rows_for(QwenCodeParser(), self.REL)
        self.assertEqual(len(before), 13)
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
        self.assertEqual(len(rows), 15)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[:13]],
            [(row.source_line, row.turn_type, row.text) for row in before],
        )
        self.assertEqual(
            [(row.source_line, row.turn_type, row.session_id, row.text) for row in rows[13:]],
            [
                (11, "user", QWEN_SESSION, "after the cut"),
                (10, "system", QWEN_SESSION, bad_line_text()),
            ],
        )

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
        (self.home / rel).write_text("[1]", encoding="utf-8")
        self.assertEqual(
            [row.text for row in self.rows_for(QwenCodeParser(), rel)],
            ["parser: .qwen/settings.json is not a JSON object"],
        )
        # The fixture home's empty settings file yields nothing.
        (self.home / rel).write_text("{}", encoding="utf-8")
        self.assertEqual(self.rows_for(QwenCodeParser(), rel), [])

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
