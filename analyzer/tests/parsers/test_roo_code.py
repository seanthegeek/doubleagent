"""Tests of the Roo Code parser."""

import json
from typing import ClassVar

from doubleagent.inputs import Artifact, Home
from doubleagent.parsers import Options
from doubleagent.parsers.antigravity import file_mtime
from doubleagent.parsers.cline import ClineParser
from doubleagent.parsers.roo_code import ENTRIES_KEY_PATTERN, RooCodeParser
from fixtures import (
    ROO_CLI_TASK,
    ROO_CLOUD_TOKEN,
    ROO_GONE_TASK,
    ROO_TASK,
    ROO_VSCDB_REL,
    ROO_WORKSPACE_STATE_REL,
    ROO_WORKSPACE_VSCDB_REL,
    write_bad_line,
)

from .base import ParserBase

ROO_GS = ".config/Code/User/globalStorage/rooveterinaryinc.roo-cline/"


class RooCodeTests(ParserBase):
    """Test the Roo Code parser."""

    def test_entries_key_takes_json_whitespace_only(self):
        self.assertTrue(ENTRIES_KEY_PATTERN.search('{"entries" \r\n:\t['))
        for space in map(chr, (0x00A0, 0x2028, 0x0085)):
            text = '{"entries"%s: [' % space
            self.assertIsNone(ENTRIES_KEY_PATTERN.search(text), repr(text))

    UI_MESSAGES = ROO_GS + "tasks/%s/ui_messages.json" % ROO_TASK
    CLI_API = ".vscode-mock/global-storage/tasks/%s/api_conversation_history.json" % ROO_CLI_TASK

    def test_rows(self):
        rows = self.rows_for(RooCodeParser(), self.UI_MESSAGES)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "user",
                "system",
                "tool_use",
                "system",
                "tool_use",
                "tool_result",
                "system",
                "assistant",
                "user",
                "assistant",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "roo-code", ROO_TASK),
            )
            self.assertEqual((row.project_path, row.git_branch, row.model), ("/srv/proj", "", ""))
        user, api, diff, approved, command, out, condense, ask, feedback, _done = rows
        # The tool ask marked isAnswered (approved, by a click or automatically).
        self.assertEqual(
            approved.text,
            "permission decision: approved isAnswered=true (user or auto-approval)",
        )
        self.assertEqual(
            (approved.tool_use_id, approved.tool_name, approved.timestamp_utc),
            (diff.tool_use_id, "appliedDiff", diff.timestamp_utc),
        )
        self.assertEqual(
            (user.text, user.timestamp_utc), ("rename the helper", "2026-10-03T10:03:20.000Z")
        )
        self.assertIn("apiProtocol=anthropic", api.text)
        self.assertEqual((diff.tool_name, diff.text), ("appliedDiff", "src/util.py | -old\n+new"))
        self.assertEqual((out.tool_use_id, out.text), (command.tool_use_id, "1 passed"))
        self.assertTrue(condense.text.startswith("condense_context: prevContextTokens=9000"))
        self.assertIn("Renamed helper; tests pass.", condense.text)
        self.assertEqual(ask.text, "Commit now? | yes; no")
        self.assertEqual(feedback.text, "yes")

    def test_thinking_opt_in(self):
        rows = self.rows_for(RooCodeParser(), self.CLI_API)
        self.assertNotIn("thinking", [row.turn_type for row in rows])
        rows = self.rows_for(RooCodeParser(), self.CLI_API, include_thinking=True)
        self.assertEqual(
            [row.text for row in rows if row.turn_type == "thinking"], ["check the runner"]
        )

    GS_API = ROO_GS + "tasks/%s/api_conversation_history.json" % ROO_TASK
    DENIED = "permission decision: denied feedback=leave conftest alone"
    APPROVED = "permission decision: approved feedback=ok"

    def test_api_history_only_without_ui_messages(self):
        # Beside ui_messages.json only the decision rows come from it.
        rows = self.rows_for(RooCodeParser(), self.GS_API)
        self.assertEqual(
            [(row.turn_type, row.tool_use_id, row.tool_name, row.text) for row in rows],
            [
                ("system", "toolu_01B", "write_to_file", self.DENIED),
                ("system", "toolu_01C", "read_file", self.APPROVED),
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.session_id, row.project_path, row.timestamp_utc),
                (ROO_TASK, "/srv/proj", "2026-10-03T10:05:05.000Z"),
            )
        rows = self.rows_for(RooCodeParser(), self.CLI_API)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "user",
                "tool_use",
                "tool_result",
                "tool_use",
                "tool_use",
                "tool_result",
                "system",
                "tool_result",
                "system",
            ],
        )
        self.assertEqual(
            [row.timestamp_utc for row in rows[:3]],  # Roo records carry their own ts
            ["2026-10-03T10:05:00.000Z", "2026-10-03T10:05:02.000Z", "2026-10-03T10:05:03.000Z"],
        )
        self.assertEqual(
            {(row.session_id, row.project_path) for row in rows}, {(ROO_CLI_TASK, "/srv/proj")}
        )
        self.assertEqual((rows[1].tool_use_id, rows[2].tool_use_id), ("toolu_01A", "toolu_01A"))
        # Each decision follows its tool_result and joins its call.
        self.assertEqual(
            [(row.tool_use_id, row.tool_name, row.text) for row in rows[5:]],
            [
                ("toolu_01B", "", '{"status":"denied","feedback":"leave conftest alone"}'),
                ("toolu_01B", "write_to_file", self.DENIED),
                ("toolu_01C", "", '{"status":"approved","feedback":"ok"}\n\n[pytest]'),
                ("toolu_01C", "read_file", self.APPROVED),
            ],
        )

    def test_api_decisions_truncated_beside_ui_messages(self):
        # The API history is read for decisions even beside ui_messages.json,
        # so its bad records are reported there too.
        write_bad_line(self.home / self.GS_API)
        rows = self.rows_for(RooCodeParser(), self.GS_API)
        self.assertEqual(rows[-1].turn_type, "system")
        self.assertTrue(rows[-1].text.startswith("parser: "), rows[-1].text)
        self.assertEqual((rows[-1].session_id, rows[-1].project_path), (ROO_TASK, "/srv/proj"))
        self.assertEqual([row.text for row in rows[:-1]], [self.DENIED, self.APPROVED])

    def test_history_item_and_index(self):
        rows = self.rows_for(RooCodeParser(), ROO_GS + "tasks/%s/history_item.json" % ROO_TASK)
        self.assertEqual(len(rows), 2)
        self.assertTrue(rows[0].text.startswith("task: rename the helper"))
        self.assertIn("mode=code", rows[0].text)
        # HistoryItem.mode, the task's mode slug, is its permission mode.
        self.assertEqual(
            (rows[1].turn_type, rows[1].text, rows[1].session_id, rows[1].timestamp_utc),
            ("system", "permission mode: code", ROO_TASK, rows[0].timestamp_utc),
        )
        rows = self.rows_for(RooCodeParser(), ROO_GS + "tasks/_index.json")
        self.assertEqual(
            [(row.session_id, row.text.split(":")[0]) for row in rows],
            [(ROO_GONE_TASK, "task"), (ROO_GONE_TASK, "permission mode")],
        )  # tasks on disk are not repeated
        rows = self.rows_for(RooCodeParser(), ROO_GS + "tasks/%s/task_metadata.json" % ROO_TASK)
        self.assertEqual(
            [row.text.split(":")[0] for row in rows], ["file roo_read", "file roo_edit"]
        )

    def test_secrets_not_wanted(self):
        art = next(
            artifact
            for artifact in self.col.artifacts
            if artifact.rel == ".vscode-mock/global-storage/secrets.json"
        )
        self.assertFalse(RooCodeParser().wants(art))
        self.assertFalse(ClineParser().wants(art))

    CLI_HISTORY = ".roo/cli-history.json"

    def test_secondary(self):
        parser = RooCodeParser()
        self.assertTrue(parser.secondary(self.artifact(self.CLI_HISTORY)))
        for rel in (self.UI_MESSAGES, self.CLI_API, ROO_GS + "tasks/_index.json"):
            self.assertFalse(parser.secondary(self.artifact(rel)), rel)
        self.assertEqual(parser.secondary_tolerance(self.artifact(self.CLI_HISTORY)), 2.0)

    def test_cli_history_rows(self):
        parser = RooCodeParser()
        self.assertTrue(parser.wants(self.artifact(self.CLI_HISTORY)))
        rows = self.rows_for(parser, self.CLI_HISTORY)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                (1, "user", "rename the helper"),
                (2, "user", "yes"),
                (3, "user", "/new"),
                (6, "user", "summarise the open issues"),
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
                    row.timestamp_utc,
                ),
                ("h1", "alice", "roo-code", "", "", ""),
            )
            self.assertTrue(row.source_file.endswith(self.CLI_HISTORY), row.source_file)
        # The settings file beside it is read for permission state only.
        self.assertFalse(parser.secondary(self.artifact(".roo/cli-settings.json")))

    def test_cli_history_truncated(self):
        write_bad_line(self.home / self.CLI_HISTORY)
        rows = self.rows_for(RooCodeParser(), self.CLI_HISTORY)
        self.assertEqual([row.turn_type for row in rows], ["user"] * 4 + ["system"])
        self.assertEqual(
            rows[-1].text, "parser: cli-history.json unparseable after its entries array"
        )

    def test_cli_history_malformed(self):
        path = self.home / self.CLI_HISTORY
        text = '{"version": 1, "entries": ["rename the helper", "ye'
        path.write_text(text, encoding="utf-8")
        try:
            json.JSONDecoder().raw_decode(text, text.rindex('"ye'))
        except ValueError as error:
            reason = str(error)
        else:
            raise AssertionError("cut entry parses")
        rows = self.rows_for(RooCodeParser(), self.CLI_HISTORY)
        self.assertEqual(
            [(row.turn_type, row.source_line, row.text) for row in rows],
            [
                ("user", 1, "rename the helper"),
                ("system", 2, 'parser: unparseable record 2 (%s): "ye' % reason),
            ],
        )
        path.write_text('{"version": 1}', encoding="utf-8")
        rows = self.rows_for(RooCodeParser(), self.CLI_HISTORY)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("system", "parser: cli-history.json has no entries array")],
        )

    def test_truncated_line_is_reported_not_fatal(self):
        write_bad_line(self.home / self.UI_MESSAGES)
        rows = self.rows_for(RooCodeParser(), self.UI_MESSAGES)
        self.assertEqual(len(rows), 11)
        self.assertEqual(rows[-1].turn_type, "system")
        self.assertIn("parser:", rows[-1].text)
        self.assertEqual(sum(row.text.startswith("parser:") for row in rows), 1)

    # ---- standing settings, Roo Code Cloud (source-derived) -------------------

    GLOBAL_STATE = ".vscode-mock/global-storage/global-state.json"
    STANDING: ClassVar[list[str]] = [
        "autoApprovalEnabled=true",
        "alwaysAllowReadOnly=true",
        "alwaysAllowExecute=false",
        "allowedCommands=git status",
        "allowedCommands=npm test",
        "deniedCommands=rm -rf",
        "mode=architect",
        "roo-cline.allowedCommands=git status",
    ]
    SYNC: ClassVar[list[str]] = [
        "user-settings.settings.taskSyncEnabled=true",
        "organization-settings.cloudSettings.recordTaskMessages=true",
        "organization-settings.cloudSettings.enableTaskSharing=false",
    ]

    def _expected_state(self, source):
        cloud = "%s, Roo Code Cloud settings cache" % source
        return [
            "permission setting: %s (from %s, file mtime)" % (setting, source)
            for setting in self.STANDING
        ] + ["sync setting: %s (from %s, file mtime)" % (setting, cloud) for setting in self.SYNC]

    def test_global_state_rows(self):
        parser = RooCodeParser()
        art = self.artifact(self.GLOBAL_STATE)
        self.assertTrue(parser.wants(art))
        self.assertFalse(parser.secondary(art))
        rows = self.rows_for(parser, self.GLOBAL_STATE)
        self.assertEqual([row.text for row in rows], self._expected_state(self.GLOBAL_STATE))
        for row in rows:
            self.assertEqual((row.turn_type, row.session_id), ("system", ""))
            self.assertEqual(row.timestamp_utc, file_mtime(art))
        self.assertNotIn("customInstructions", " ".join(row.text for row in rows))

    def test_state_vscdb_rows(self):
        parser = RooCodeParser()
        self.assertIn("vscode", parser.reads_agents)
        art = self.artifact(ROO_VSCDB_REL)
        self.assertEqual(art.agent, "vscode")
        self.assertTrue(parser.wants(art))
        rows = self.rows_for(parser, ROO_VSCDB_REL)
        source = "%s ItemTable RooVeterinaryInc.roo-cline" % ROO_VSCDB_REL
        self.assertEqual([row.text for row in rows], self._expected_state(source))
        # The fake state.vscdb beside the extension's tasks is not SQLite.
        self.assertEqual(
            self.rows_for(parser, ".config/Code/User/globalStorage/state.vscdb"),
            [],
        )

    def test_cli_settings_rows(self):
        rows = self.rows_for(RooCodeParser(), ".roo/cli-settings.json")
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: requireApproval=true (from .roo/cli-settings.json, file mtime)",
                "permission setting: mode=code (from .roo/cli-settings.json, file mtime)",
            ],
        )

    def test_mcp_always_allow_rows(self):
        parser = RooCodeParser()
        rel = ROO_GS + "settings/mcp_settings.json"
        rows = self.rows_for(parser, rel)
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: mcpServers.github.alwaysAllow=%s (from %s, file mtime)"
                % (tool, rel)
                for tool in ("list_issues", "get_issue")
            ],
        )
        self.assertEqual({row.project_path for row in rows}, {""})
        self.assertNotIn("ghp-not-a-real-token", " ".join(row.text for row in rows))
        # A project's .roo/mcp.json carries the project.
        art = self.artifact(".roo/mcp.json")
        project = Artifact(
            art.disk_path,
            "/srv/proj/.roo/mcp.json",
            art.rel,
            art.agent,
            Home(art.home.disk_path, "/srv/proj", "alice", "h1", project=True),
        )
        rows = list(parser.parse(project, Options()))
        self.assertEqual(len(rows), 2)
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})

    def test_bad_settings_files(self):
        for rel, name in (
            (".roo/cli-settings.json", "cli-settings.json"),
            (self.GLOBAL_STATE, "global-state.json"),
            (ROO_WORKSPACE_STATE_REL, "workspace-state.json"),
            (".roo/mcp.json", "mcp.json"),
        ):
            write_bad_line(self.home / rel)
            rows = self.rows_for(RooCodeParser(), rel)
            self.assertEqual(
                [(row.turn_type, row.text) for row in rows],
                [("system", "parser: %s is not a JSON object" % name)],
                rel,
            )

    def test_retry_queue_rows(self):
        parser = RooCodeParser()
        expected = (
            "cloud upload queued: Task Message task=%s type=telemetry retries=2 "
            "url=https://app.roocode.com/api/events lastError=fetch failed "
            "(Roo Code Cloud retry queue, from %s roo.retryQueue, request timestamp)"
        )
        for rel, source in (
            (ROO_WORKSPACE_STATE_REL, ROO_WORKSPACE_STATE_REL),
            (
                ROO_WORKSPACE_VSCDB_REL,
                ROO_WORKSPACE_VSCDB_REL + " ItemTable RooVeterinaryInc.roo-cline",
            ),
        ):
            self.assertTrue(parser.wants(self.artifact(rel)), rel)
            rows = self.rows_for(parser, rel)
            self.assertEqual([row.text for row in rows], [expected % (ROO_TASK, source)], rel)
            self.assertEqual(
                (rows[0].turn_type, rows[0].session_id, rows[0].timestamp_utc),
                ("system", ROO_TASK, "2026-10-03T10:03:20.500Z"),
            )
            # Neither the bearer token nor the message body is written.
            self.assertNotIn(ROO_CLOUD_TOKEN, rows[0].text)
            self.assertNotIn("rename the helper", rows[0].text)
        self.assertFalse(parser.records_sync)
