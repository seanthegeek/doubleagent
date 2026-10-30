"""Tests of the VS Code parser."""

import io
import json

from doubleagent import cli
from doubleagent.inputs import open_input
from doubleagent.parsers.vscode import MutationError, VsCodeParser, apply_mutation
from doubleagent.parsers.vscode import confirmed_text as vscode_confirmed_text
from doubleagent.timeutil import to_utc
from fixtures import (
    BAD_LINE,
    VSCODE_AGENTS_CWD,
    VSCODE_AGENTS_WORKSPACE,
    VSCODE_CLOUD_SESSIONS_MTIME,
    VSCODE_CLOUD_SESSIONS_REL,
    VSCODE_CLOUD_TASK,
    VSCODE_EDITING_REL,
    VSCODE_GRANTS_MTIME,
    VSCODE_LEGACY_SESSION,
    VSCODE_MULTIROOT_FILE,
    VSCODE_MULTIROOT_WORKSPACE,
    VSCODE_OLD_TASK,
    VSCODE_OTHER_MACHINE_SESSION,
    VSCODE_PROFILE_SETTINGS_REL,
    VSCODE_SESSION,
    VSCODE_STATE_REL,
    VSCODE_WORKSPACE,
    VSCODE_WORKSPACE_STATE_REL,
    _vscdb,
    bad_line_text,
    build_vscode_agents_window,
    write_bad_line,
)

from .base import ParserBase, read_jsonl, rel_only


class VsCodeTests(ParserBase):
    """Test the VS Code (Copilot Chat) parser."""

    REL = VSCODE_WORKSPACE + "/chatSessions/%s.jsonl" % VSCODE_SESSION
    LEGACY = (
        ".vscode-server/data/User/globalStorage/emptyWindowChatSessions/%s.json"
        % VSCODE_LEGACY_SESSION
    )

    def test_rows(self):
        rows = self.rows_for(VsCodeParser(), self.REL)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "system",
                "system",
                "system",
                "user",
                "tool_use",
                "tool_result",
                "system",
                "assistant",
                "user",
                "assistant",
                "system",
                "system",
                "system",
                "system",
                "user",
                "system",
                "assistant",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "vscode", VSCODE_SESSION),
            )
            self.assertEqual((row.project_path, row.git_branch), ("/srv/proj", ""))
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        self.assertEqual([row.source_line for row in rows], [1] * 9 + [2, 2, 2] + [6] * 6)
        start, model, mode, level, user, use, result, decision, asst, user2, asst2 = rows[:11]
        self.assertEqual(
            (model.text, model.model, model.timestamp_utc),
            ("model: copilot/gpt-4.1", "copilot/gpt-4.1", "2026-10-03T11:00:01.000Z"),
        )
        self.assertEqual(
            (mode.text, mode.timestamp_utc),
            ("permission mode: agent", "2026-10-03T11:00:01.000Z"),
        )
        # modeInfo.permissionLevel, source-derived.
        self.assertEqual(
            (level.text, level.timestamp_utc),
            ("permission level: default", "2026-10-03T11:00:01.000Z"),
        )
        self.assertEqual(
            (decision.text, decision.tool_use_id, decision.tool_name, decision.timestamp_utc),
            (
                "permission decision: confirmed=UserAction(4) selectedButton=Allow"
                " tool_source=internal",
                "call_a",
                "run_in_terminal",
                "2026-10-03T11:00:02.000Z",
            ),
        )
        confirm, model2, mode2, level2, user3, chosen, asst3 = rows[11:]
        self.assertEqual(
            (confirm.text, confirm.timestamp_utc),
            (
                'permission decision: confirmation isUsed=true buttons=["Allow","Dismiss"]'
                " title=Run npm install?\nThis installs the packages.",
                "2026-10-03T11:00:11.000Z",
            ),
        )
        self.assertEqual(
            (model2.text, model2.model, model2.timestamp_utc),
            (
                # modelConfiguration rides with the model.
                "model: copilot/gpt-4.1 -> copilot/claude-sonnet-4 reasoningEffort=high",
                "copilot/claude-sonnet-4",
                "2026-10-03T11:00:20.000Z",
            ),
        )
        self.assertEqual(mode2.text, "permission mode: agent -> ask")
        self.assertEqual(level2.text, "permission level: default -> autoApprove")
        self.assertEqual(user3.text, 'Allow: "Run npm install?"')
        self.assertEqual(
            (chosen.text, chosen.timestamp_utc),
            ("permission decision: confirmation=Allow", "2026-10-03T11:00:20.000Z"),
        )
        self.assertEqual(asst3.text, "Installed.")
        self.assertEqual(
            start.text, "session start: responder=GitHub Copilot location=panel title=Run tests"
        )
        self.assertEqual(start.timestamp_utc, "2026-10-03T11:00:00.000Z")
        self.assertEqual(
            (user.text, user.model, user.timestamp_utc),
            ("run tests\n[attached: test_a.py]", "copilot/gpt-4.1", "2026-10-03T11:00:01.000Z"),
        )
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text), ("run_in_terminal", "call_a", "pytest")
        )
        self.assertEqual(
            (result.tool_use_id, result.text, result.timestamp_utc),
            ("call_a", "3 passed", "2026-10-03T11:00:02.000Z"),
        )
        self.assertEqual(
            asst.text, "All tests pass in test_a.py."
        )  # markdown run with an inline reference
        self.assertEqual(
            (user2.text, user2.source_line), ("thanks", 2)
        )  # pushed by the second log line
        self.assertEqual(
            (asst2.text, asst2.timestamp_utc), ("You're welcome.", "2026-10-03T11:00:11.000Z")
        )

    def test_confirmed_forms(self):
        # A pre-1.104 boolean, a denial with its source, and a reason.
        self.assertEqual(
            vscode_confirmed_text({"isConfirmed": False}), "permission decision: confirmed=false"
        )
        self.assertEqual(
            vscode_confirmed_text(
                {"isConfirmed": {"type": 0, "source": "user"}, "source": {"type": "mcp"}}
            ),
            "permission decision: confirmed=Denied(0) source=user tool_source=mcp",
        )
        self.assertEqual(
            vscode_confirmed_text({"isConfirmed": {"type": 1, "reason": {"value": "read only"}}}),
            "permission decision: confirmed=ConfirmationNotNeeded(1) reason=read only",
        )
        self.assertEqual(vscode_confirmed_text({"isConfirmed": None}), "")
        self.assertEqual(vscode_confirmed_text({}), "")

    def test_thinking_opt_in(self):
        rows = self.rows_for(VsCodeParser(), self.REL, include_thinking=True)
        self.assertEqual([row.text for row in rows if row.turn_type == "thinking"], ["Need pytest"])

    def test_legacy_json_session(self):
        rows = self.rows_for(VsCodeParser(), self.LEGACY)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [
                ("system", "session start: responder=GitHub Copilot location=panel"),
                ("system", "model: copilot/gpt-4o"),
                ("user", "where is the config?"),
                ("assistant", "It is in config.toml."),
                ("system", "error: Rate limited"),
            ],
        )
        self.assertTrue(
            all(
                row.session_id == VSCODE_LEGACY_SESSION and row.project_path == "/srv/proj"
                for row in rows
            )
        )
        self.assertEqual(rows[2].model, "copilot/gpt-4o")

    def test_not_wanted(self):
        for rel in (
            VSCODE_WORKSPACE + "/workspace.json",
            VSCODE_WORKSPACE + "/chatEditingSessions/%s/contents/aaaaaaa" % VSCODE_SESSION,
        ):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertFalse(VsCodeParser().wants(art), rel)
        for rel in (
            ".config/Code/User/keybindings.json",
            ".config/Code/User/profiles/p1/mcp.json",
            ".config/Code/User/workspaceStorage/x/chatEditingSessions/s/contents/state.json",
            ".config/Code/User/globalStorage/github.copilot-chat/state.vscdb",
        ):
            self.assertFalse(VsCodeParser().wants(rel_only(rel)), rel)
        for rel in (
            ".config/Code/User/settings.json",
            ".config/Code/User/profiles/p1/settings.json",
            ".config/Code/User/profiles/builtin/agents/globalStorage/state.vscdb",
            "AppData/Roaming/Code/User/workspaceStorage/x/state.vscdb",
            ".vscode-server/data/User/workspaceStorage/x/chatEditingSessions/s/state.json",
            "Library/Application Support/Code - Insiders/User/workspaceStorage/x/chatSessions/s.json",
            "AppData/Roaming/VSCodium/User/globalStorage/transferredChatSessions/s.json",
            ".config/Positron/User/globalStorage/emptyWindowChatSessions/s.jsonl",
            ".config/Trae CN/User/workspaceStorage/x/chatSessions/s.jsonl",
            ".vscodium-server-insiders/data/User/workspaceStorage/x/chatSessions/s.jsonl",
            ".positron-server/data/User/globalStorage/emptyWindowChatSessions/s.json",
        ):
            self.assertTrue(VsCodeParser().wants(rel_only(rel)), rel)
        for rel in (
            ".config/Cursor/User/workspaceStorage/x/chatSessions/s.jsonl",
            ".config/Code/User/globalStorage/transferredChatSessions/s.jsonl",
        ):
            self.assertFalse(VsCodeParser().wants(rel_only(rel)), rel)

    def test_truncated_line_is_reported_not_fatal(self):
        clean = [(row.turn_type, row.text) for row in self.rows_for(VsCodeParser(), self.REL)]
        write_bad_line(self.home / self.REL)
        rows = self.rows_for(VsCodeParser(), self.REL)
        self.assertEqual([(row.turn_type, row.text) for row in rows[:-1]], clean)
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].source_line, rows[-1].session_id, rows[-1].text),
            ("system", 7, VSCODE_SESSION, bad_line_text()),
        )

    def test_bad_lines_mid_log_are_reported_in_full(self):
        # A line that does not decode and one whose mutation cannot apply,
        # between valid lines: each is its own row, the rest still replay.
        clean = [(row.turn_type, row.text) for row in self.rows_for(VsCodeParser(), self.REL)]
        path = self.home / self.REL
        lines = path.read_bytes().split(b"\n")
        stray = '{"kind": 1, "k": ["nowhere", "x"], "v": 1}'
        lines[1:1] = [BAD_LINE.encode(), stray.encode() + b"\r"]
        path.write_bytes(b"\n".join(lines))
        rows = self.rows_for(VsCodeParser(), self.REL)
        self.assertEqual([(row.turn_type, row.text) for row in rows[:-2]], clean)
        self.assertEqual(
            [(row.turn_type, row.source_line, row.text) for row in rows[-2:]],
            [
                ("system", 2, bad_line_text()),
                (
                    "system",
                    3,
                    "parser: mutation not applied (path ['nowhere'] not found): " + stray,
                ),
            ],
        )

    def test_mutation_log(self):
        state = apply_mutation(None, {"kind": 0, "v": {"a": [1, 2, 3], "b": {"c": 1}}})
        state = apply_mutation(state, {"kind": 2, "k": ["a"], "v": [9], "i": 1})
        state = apply_mutation(state, {"kind": 1, "k": ["b", "c"], "v": 2})
        state = apply_mutation(state, {"kind": 3, "k": ["b"]})
        state = apply_mutation(state, {"kind": 2, "k": ["new"], "v": [1]})
        self.assertEqual(state, {"a": [1, 9], "new": [1]})
        with self.assertRaises(MutationError):
            apply_mutation(state, {"kind": 1, "k": ["missing", "x"], "v": 1})

    def _reopen_with_agents_window(self):
        build_vscode_agents_window(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")

    def test_agents_window_project_is_working_directory(self):
        # The Agents window's workspace file is never the project.
        self._reopen_with_agents_window()
        rel = VSCODE_AGENTS_WORKSPACE + "/chatSessions/%s.jsonl" % VSCODE_SESSION
        rows = self.rows_for(VsCodeParser(), rel)
        self.assertTrue(rows)
        self.assertEqual({row.project_path for row in rows}, {VSCODE_AGENTS_CWD})
        agents_file = self.home / ".config/Code/User/agent-sessions.code-workspace"
        self.assertEqual(
            rows[0].text,
            "session start: responder=GitHub Copilot location=panel title=Run tests"
            " workspace=%s" % agents_file,
        )

    def test_multiroot_workspace_file_is_not_project(self):
        self._reopen_with_agents_window()
        rel = VSCODE_MULTIROOT_WORKSPACE + "/chatSessions/%s.json" % VSCODE_LEGACY_SESSION
        rows = self.rows_for(VsCodeParser(), rel)
        self.assertTrue(rows)
        self.assertEqual({row.project_path for row in rows}, {""})
        self.assertTrue(rows[0].text.endswith(" workspace=" + VSCODE_MULTIROOT_FILE), rows[0])

    def test_working_directory_before_workspace_folder(self):
        workspace_file = self.home / VSCODE_WORKSPACE / "workspace.json"
        workspace_file.write_text(json.dumps({"folder": "file:///srv/other"}), encoding="utf-8")
        rows = self.rows_for(VsCodeParser(), self.REL)
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})
        self.assertNotIn("workspace=", rows[0].text)

    def test_folder_when_no_working_directory(self):
        path = self.home / self.REL
        lines = path.read_text(encoding="utf-8").splitlines()
        first = json.loads(lines[0])
        del first["v"]["workingDirectory"]
        lines[0] = json.dumps(first)
        path.write_text("\n".join(lines) + "\n", encoding="utf-8")
        rows = self.rows_for(VsCodeParser(), self.REL)
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})

    def test_bad_workspace_json(self):
        clean = [(row.turn_type, row.text) for row in self.rows_for(VsCodeParser(), self.REL)]
        (self.home / VSCODE_WORKSPACE / "workspace.json").write_text("{not json", encoding="utf-8")
        rows = self.rows_for(VsCodeParser(), self.REL)
        self.assertEqual([(row.turn_type, row.text) for row in rows[:-1]], clean)
        last = rows[-1]
        self.assertEqual((last.turn_type, last.source_line, last.timestamp_utc), ("system", 0, ""))
        self.assertTrue(last.text.startswith("parser: workspace.json not read ("), last.text)
        self.assertEqual((last.session_id, last.project_path), (VSCODE_SESSION, "/srv/proj"))
        (self.home / VSCODE_WORKSPACE / "workspace.json").write_text("[1]", encoding="utf-8")
        rows = self.rows_for(VsCodeParser(), self.REL)
        self.assertEqual([(row.turn_type, row.text) for row in rows[:-1]], clean)
        self.assertEqual(rows[-1].text, "parser: workspace.json is not a JSON object")


class VsCodeSyncTests(ParserBase):
    """Test Copilot Chat's session sync and cloud agent records."""

    SOURCE = "(from User/globalStorage/github.copilot-chat/cloudSessions.json, file mtime)"
    TASKS = (
        "(from state.vscdb GitHub.copilot-chat github.copilot.cloudAgent.ownedTasks, recordedAt)"
    )

    def test_cloud_sessions_rows(self):
        self.assertTrue(VsCodeParser.records_sync)
        rows = self.rows_for(VsCodeParser(), VSCODE_CLOUD_SESSIONS_REL)
        timestamp = to_utc(VSCODE_CLOUD_SESSIONS_MTIME)
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    VSCODE_SESSION,
                    timestamp,
                    "session synced: remote session cs-0001 task ct-0001 " + self.SOURCE,
                ),
                (
                    "system",
                    VSCODE_OTHER_MACHINE_SESSION,
                    timestamp,
                    "session synced: remote session cs-0002 task - " + self.SOURCE,
                ),
            ],
        )

    def test_cloud_sessions_not_a_json_object(self):
        for raw in ("[]", '{"a": '):
            (self.home / VSCODE_CLOUD_SESSIONS_REL).write_text(raw, encoding="utf-8")
            rows = self.rows_for(VsCodeParser(), VSCODE_CLOUD_SESSIONS_REL)
            self.assertEqual(len(rows), 1)
            self.assertEqual((rows[0].turn_type, rows[0].session_id), ("system", ""))
            self.assertTrue(
                rows[0].text.startswith("parser: cloudSessions.json is not a JSON object"),
                rows[0].text,
            )

    def test_owned_tasks_rows(self):
        rows = self.rows_for(VsCodeParser(), VSCODE_STATE_REL)
        rows = [row for row in rows if row.text.startswith("session ")]
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    "copilot-cloud-agent:/task/" + VSCODE_CLOUD_TASK,
                    "2025-10-02T10:16:40.000Z",
                    "session synced: cloud agent task %s application=vscode %s"
                    % (VSCODE_CLOUD_TASK, self.TASKS),
                ),
                (
                    "system",
                    "copilot-cloud-agent:/task/" + VSCODE_CLOUD_TASK,
                    "2025-10-02T10:16:40.000Z",
                    "session origin: remote GitHub cloud agent " + self.TASKS,
                ),
                (
                    "system",
                    "copilot-cloud-agent:/task/" + VSCODE_OLD_TASK,
                    "2025-10-01T06:26:40.000Z",
                    "session synced: cloud agent task %s application=- %s"
                    % (VSCODE_OLD_TASK, self.TASKS),
                ),
                (
                    "system",
                    "copilot-cloud-agent:/task/" + VSCODE_OLD_TASK,
                    "2025-10-01T06:26:40.000Z",
                    "session origin: remote GitHub cloud agent " + self.TASKS,
                ),
            ],
        )
        self.assertEqual([row for row in rows if "gho_not_real" in row.text], [])

    def test_profile_state_grants_and_source_host(self):
        # Source-derived: the Copilot Chat repository selection, the
        # GitHub account's usage records and the profile-scope grants.
        rows = self.rows_for(VsCodeParser(), VSCODE_STATE_REL)
        rest = [row for row in rows if not row.text.startswith("session ")]
        timestamp = to_utc(VSCODE_GRANTS_MTIME)
        source = "(from User/globalStorage/state.vscdb, file mtime)"
        self.assertEqual(
            [
                (row.turn_type, row.session_id, row.project_path, row.timestamp_utc, row.text)
                for row in rest
            ],
            [
                (
                    "system",
                    "",
                    "",
                    "2026-10-03T10:56:40.000Z",
                    "source host repository: octocat/hello-world type=cloud agent selection"
                    " (from state.vscdb GitHub.copilot-chat userSelectedRepositories, timestamp)",
                ),
                (
                    "system",
                    "",
                    "",
                    "2026-10-03T10:58:20.000Z",
                    "source host: github.com account octocat extension=GitHub.copilot-chat"
                    " scopes=read:user,user:email (from state.vscdb github-octocat-usages,"
                    " lastUsed)",
                ),
                (
                    "system",
                    "",
                    "",
                    "2026-10-03T10:59:10.000Z",
                    "source host: github.com account octocat mcp_server=github scopes=repo"
                    " (from state.vscdb github-octocat-mcpserver-usages, lastUsed)",
                ),
                (
                    "system",
                    "",
                    "",
                    timestamp,
                    "permission setting: chat/autoconfirm-post.read_file=Read File " + source,
                ),
                (
                    "system",
                    "",
                    "",
                    timestamp,
                    "permission setting: chat.tools.global.autoApprove.optIn=true " + source,
                ),
            ],
        )
        self.assertEqual([row for row in rows if "gho_not_real" in row.text], [])

    def test_workspace_state_grants(self):
        rows = self.rows_for(VsCodeParser(), VSCODE_WORKSPACE_STATE_REL)
        source = "(from User/workspaceStorage/abc123/state.vscdb, file mtime)"
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: chat/autoconfirm.run_in_terminal=true " + source,
                "permission setting: chat/autoconfirm.mcp_github_create_issue="
                + json.dumps(
                    {"arguments": '{"repo":"x"}', "label": "Create issue"},
                    separators=(",", ":"),
                )
                + " "
                + source,
                "permission setting: chat/servers/autoconfirm.github=true " + source,
                "permission setting: chat.externalPath.workspaceAllowlist=/srv/shared " + source,
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.turn_type, row.session_id, row.project_path, row.timestamp_utc),
                ("system", "", "/srv/proj", to_utc(VSCODE_GRANTS_MTIME)),
            )

    def test_grant_of_wrong_type_is_reported(self):
        _vscdb(self.home / VSCODE_WORKSPACE_STATE_REL, {"chat/autoconfirm-post": "7"})
        rows = self.rows_for(VsCodeParser(), VSCODE_WORKSPACE_STATE_REL)
        self.assertEqual(len(rows), 5)
        self.assertEqual(
            rows[-1].text,
            "parser: ItemTable value for chat/autoconfirm-post is not a JSON object or array",
        )

    def test_settings_rows(self):
        rows = self.rows_for(VsCodeParser(), VSCODE_PROFILE_SETTINGS_REL)
        source = " (from User/profiles/-5f3a1b2c/settings.json, file mtime)"
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: chat.tools.terminal.autoApprove.npm test=true" + source,
                "permission setting: chat.tools.terminal.autoApprove.rm=false" + source,
                "permission setting: chat.tools.edits.autoApprove={}" + source,
                "permission setting: chat.permissions.default=autoApprove" + source,
                "model setting: chat.defaultModel=copilot/gpt-5" + source,
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.turn_type, row.session_id, row.timestamp_utc),
                ("system", "", to_utc(VSCODE_GRANTS_MTIME)),
            )
        # The default profile's settings name no chat key: no row.
        self.assertEqual(self.rows_for(VsCodeParser(), ".config/Code/User/settings.json"), [])

    def test_settings_not_an_object(self):
        (self.home / VSCODE_PROFILE_SETTINGS_REL).write_text("[1, 2", encoding="utf-8")
        rows = self.rows_for(VsCodeParser(), VSCODE_PROFILE_SETTINGS_REL)
        self.assertEqual(len(rows), 1)
        self.assertTrue(
            rows[0].text.startswith(
                "parser: User/profiles/-5f3a1b2c/settings.json is not a JSON object ("
            ),
            rows[0].text,
        )

    def test_editing_state_rows(self):
        self.assertTrue(VsCodeParser.has_secondary)
        self.assertTrue(VsCodeParser().secondary(self.artifact(VSCODE_EDITING_REL)))
        self.assertFalse(VsCodeParser().secondary(self.artifact(VSCODE_WORKSPACE_STATE_REL)))
        rows = self.rows_for(VsCodeParser(), VSCODE_EDITING_REL)
        tail = " (from chatEditingSessions state.json, file mtime)"
        self.assertEqual(
            [
                (row.turn_type, row.session_id, row.project_path, row.timestamp_utc, row.text)
                for row in rows
            ],
            [
                (
                    "system",
                    VSCODE_SESSION,
                    "/srv/proj",
                    to_utc(VSCODE_GRANTS_MTIME),
                    "edit state: Accepted /srv/proj/test_a.py request=request_1"
                    " model=copilot/gpt-4.1" + tail,
                ),
                (
                    "system",
                    VSCODE_SESSION,
                    "/srv/proj",
                    to_utc(VSCODE_GRANTS_MTIME),
                    "edit state: Rejected /srv/proj/old.py request=request_1"
                    " model=copilot/gpt-4.1 deleted=true" + tail,
                ),
            ],
        )

    def test_editing_state_bad_file(self):
        for raw, want in (
            ('{"version": 2, "recentSnap', "parser: file is not valid JSON ("),
            ("[]", "parser: state.json is not a JSON object"),
        ):
            (self.home / VSCODE_EDITING_REL).write_text(raw, encoding="utf-8")
            rows = self.rows_for(VsCodeParser(), VSCODE_EDITING_REL)
            self.assertEqual(len(rows), 1, raw)
            self.assertEqual(
                (rows[0].session_id, rows[0].project_path), (VSCODE_SESSION, "/srv/proj")
            )
            self.assertTrue(rows[0].text.startswith(want), rows[0].text)

    def test_state_without_copilot_key_yields_nothing(self):
        # Cody's and Twinny's databases, and the plain-text placeholder.
        for rel in (
            ".config/VSCodium/User/globalStorage/state.vscdb",
            ".vscode-server/data/User/globalStorage/state.vscdb",
            ".config/Code/User/globalStorage/state.vscdb",
        ):
            self.assertTrue(VsCodeParser().wants(self.artifact(rel)), rel)
            self.assertEqual(self.rows_for(VsCodeParser(), rel), [], rel)

    def test_damaged_state_is_one_row(self):
        path = self.home / VSCODE_STATE_REL
        path.write_bytes(b"SQLite format 3\x00" + b"\x00" * 200)
        rows = self.rows_for(VsCodeParser(), VSCODE_STATE_REL)
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("parser: state.vscdb unreadable"), rows[0].text)

    def test_session_without_entry_has_no_sync_row(self):
        for rel in (
            VSCODE_WORKSPACE + "/chatSessions/" + VSCODE_SESSION + ".jsonl",
            ".vscode-server/data/User/globalStorage/emptyWindowChatSessions/"
            + VSCODE_LEGACY_SESSION
            + ".json",
        ):
            rows = self.rows_for(VsCodeParser(), rel)
            self.assertEqual(
                [row for row in rows if row.text.startswith("session synced:")], [], rel
            )

    def test_sessions_synced(self):
        import contextlib

        out = self.tmp / "out"
        with contextlib.redirect_stdout(io.StringIO()):
            cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), "--agent", "vscode"])
        synced = {row["session_id"]: row["synced"] for row in read_jsonl(out / "sessions.jsonl")}
        self.assertEqual(synced[VSCODE_SESSION], "yes")
        self.assertEqual(synced[VSCODE_OTHER_MACHINE_SESSION], "yes")
        self.assertEqual(synced[VSCODE_LEGACY_SESSION], "no")
        # A task that began on GitHub's servers is synced too: GitHub holds it.
        self.assertEqual(synced["copilot-cloud-agent:/task/" + VSCODE_CLOUD_TASK], "yes")
        self.assertEqual(synced["copilot-cloud-agent:/task/" + VSCODE_OLD_TASK], "yes")
        origin = {row["session_id"]: row["origin"] for row in read_jsonl(out / "sessions.jsonl")}
        self.assertEqual(origin["copilot-cloud-agent:/task/" + VSCODE_CLOUD_TASK], "remote")
        self.assertEqual(origin["copilot-cloud-agent:/task/" + VSCODE_OLD_TASK], "remote")
        self.assertEqual(origin[VSCODE_SESSION], "local")
        self.assertEqual(origin[VSCODE_LEGACY_SESSION], "local")
        self.assertTrue(VsCodeParser.records_origin)
