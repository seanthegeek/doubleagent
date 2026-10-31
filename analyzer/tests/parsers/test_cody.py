"""Tests of the Cody parser."""

import json

from doubleagent import cli
from doubleagent.dedup import deduplicate
from doubleagent.inputs import Artifact, Home, open_input
from doubleagent.parsers import Options, by_agent
from doubleagent.parsers.cody import CodyParser, chat_time, recover_history
from doubleagent.timeutil import to_utc
from fixtures import (
    CODY_ACCOUNT,
    CODY_AGENTIC_CHAT,
    CODY_CHAT,
    CODY_JB_REL,
    CODY_MODEL,
    CODY_MODEL_SWITCHED,
    CODY_TOKEN,
    CODY_VSCDB_REL,
    CODY_VSCODE_CHAT,
    TWINNY_VSCDB_REL,
    write_bad_line,
)

from .base import ParserBase


class CodyTests(ParserBase):
    """Test the Cody parser."""

    def test_recovered_keys_take_json_whitespace_and_ascii_digits_only(self):
        date = "Wed, 01 Oct 2026 08:00:00 GMT"
        template = '{"acct": {"chat": {"%s"%s:%s{"interactions": [], "chatTitle": "t"}, '
        chats, _, _ = recover_history(template % (date, " \n", "\t"))
        self.assertEqual(list(chats["acct"]["chat"]), [date])
        for space in map(chr, (0x00A0, 0x2028, 0x0085)):
            chats, _, _ = recover_history(template % (date, space, ""))
            self.assertEqual(chats, {}, repr(space))
        chats, _, _ = recover_history(
            template % (date.replace("01 Oct", "0%s Oct" % chr(0x0661)), "", "")
        )
        self.assertEqual(chats, {})

    def test_rows(self):
        rows = self.rows_for(CodyParser(), CODY_JB_REL)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "user",
                "system",
                "assistant",
                "system",
                "user",
                "system",
                "assistant",
                "tool_use",
                "system",
                "tool_result",
                "system",
                "assistant",
                "system",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.source_line, row.git_branch),
                ("h1", "alice", "cody", 1, ""),
            )
        first, user, model, asst = rows[:4]
        self.assertEqual({row.session_id for row in rows[:4]}, {CODY_CHAT})
        self.assertEqual({row.timestamp_utc for row in rows[:4]}, {"2026-10-03T10:00:00.000Z"})
        self.assertEqual({row.timestamp_utc for row in rows[4:]}, {"2026-10-03T10:30:00.000Z"})
        self.assertEqual((model.text, model.model), ("model: " + CODY_MODEL, CODY_MODEL))
        # Each chat tracks its own model, so the agentic chat starts again.
        self.assertEqual(rows[6].text, "model: " + CODY_MODEL)
        self.assertEqual(rows[6].session_id, CODY_AGENTIC_CHAT)
        decision = rows[9]
        self.assertEqual(
            (decision.text, decision.tool_use_id, decision.tool_name),
            (
                "permission decision: confirmation state=success title=Run terminal command"
                "\npytest -q\nAllow Cody to run this command?",
                "toolu_01",
                "run_terminal_command",
            ),
        )
        self.assertEqual(
            (rows[11].text, rows[11].model),
            ("model: %s -> %s" % (CODY_MODEL, CODY_MODEL_SWITCHED), CODY_MODEL_SWITCHED),
        )
        self.assertEqual(
            first.text, "chat: Fix flaky test | account=%s | interactions=1" % CODY_ACCOUNT
        )
        self.assertEqual(
            user.text, "why is test_login flaky?\n[context: /srv/proj/tests/test_login.py]"
        )
        self.assertEqual((user.model, user.project_path), ("", ""))
        self.assertEqual(
            (asst.text, asst.model),
            ("It depends on wall-clock time.", "anthropic::2024-10-22::claude-sonnet-4-latest"),
        )
        use, result = rows[8], rows[10]
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text),
            ("run_terminal_command", "toolu_01", '{"command":"pytest -q"}'),
        )
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.text, result.session_id),
            ("run_terminal_command", "toolu_01", "3 passed", CODY_AGENTIC_CHAT),
        )
        self.assertEqual(rows[-1].text, "error: rate limit exceeded")

    def test_vscode_state_row(self):
        rows = self.rows_for(CodyParser(), CODY_VSCDB_REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [
                ("system", "chat:  | account=%s | interactions=1" % CODY_ACCOUNT),
                ("user", "explain build.sh"),
                ("system", "model: anthropic::2024-10-22::claude-sonnet-4-latest"),
                ("assistant", "It runs make."),
            ],
        )
        self.assertEqual(
            {(row.agent, row.session_id, row.source_line, row.timestamp_utc) for row in rows},
            {("cody", CODY_VSCODE_CHAT, 0, "2026-10-03T09:00:00.000Z")},
        )
        self.assertNotIn(CODY_TOKEN, " ".join(row.text for row in rows))

    def test_thinking_opt_in(self):
        # Cody records no reasoning; the option must not invent rows.
        plain = self.rows_for(CodyParser(), CODY_JB_REL)
        self.assertEqual(self.rows_for(CodyParser(), CODY_JB_REL, include_thinking=True), plain)

    def test_chat_time_and_account_collision(self):
        self.assertEqual(
            chat_time("Sat, 03 Oct 2026 10:00:00 GMT", None), "2026-10-03T10:00:00.000Z"
        )
        self.assertEqual(
            chat_time("0f0e-uuid", "Sat, 03 Oct 2026 11:00:00 GMT"), "2026-10-03T11:00:00.000Z"
        )
        self.assertEqual(chat_time("0f0e-uuid", None), "")
        history = json.loads((self.home / CODY_JB_REL).read_text(encoding="utf-8"))
        history["https://example.org/-bob"] = {
            "chat": {CODY_CHAT: history[CODY_ACCOUNT]["chat"][CODY_CHAT]}
        }
        (self.home / CODY_JB_REL).write_text(json.dumps(history), encoding="utf-8")
        sids = {row.session_id for row in self.rows_for(CodyParser(), CODY_JB_REL)}
        self.assertEqual(
            sids,
            {
                CODY_ACCOUNT + "/" + CODY_CHAT,
                "https://example.org/-bob/" + CODY_CHAT,
                CODY_AGENTIC_CHAT,
            },
        )

    def test_not_wanted(self):
        for rel in (
            ".local/share/Cody-nodejs/user-settings.json",
            ".config/Code/User/globalStorage/rjmacarthy.twinny/twinny-providers.json",
        ):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertFalse(CodyParser().wants(art), rel)
        # A state.vscdb that is not SQLite is one row; one with no Cody row yields nothing.
        self.assertEqual(
            [
                row.text
                for row in self.rows_for(
                    CodyParser(), ".config/Code/User/globalStorage/state.vscdb"
                )
            ],
            ["parser: state.vscdb is not a SQLite database"],
        )
        self.assertEqual(self.rows_for(CodyParser(), TWINNY_VSCDB_REL), [])

    def test_truncated_line_is_reported_not_fatal(self):
        write_bad_line(self.home / CODY_JB_REL)
        rows = self.rows_for(CodyParser(), CODY_JB_REL)
        self.assertEqual(len(rows), 15)
        self.assertEqual(
            [row.turn_type for row in rows[:4]], ["system", "user", "system", "assistant"]
        )
        self.assertEqual(rows[-1].turn_type, "system")
        self.assertIn("parser: chat history did not parse; recovered 2 chat(s)", rows[-1].text)

    def test_cut_mid_chat_keeps_earlier_interactions(self):
        text = (self.home / CODY_JB_REL).read_text(encoding="utf-8")
        cut = text.index('"All three pass.')
        (self.home / CODY_JB_REL).write_text(text[:cut], encoding="utf-8")
        rows = self.rows_for(CodyParser(), CODY_JB_REL)
        texts = [row.text for row in rows]
        self.assertIn("It depends on wall-clock time.", texts)
        self.assertIn("Running tests.", texts)
        self.assertNotIn("All three pass.", texts)
        self.assertEqual(rows[-1].turn_type, "system")

    def test_interaction_cut_mid_object_is_one_unparseable_record_row(self):
        first = {
            "humanMessage": {"speaker": "human", "text": "first question"},
            "assistantMessage": {"speaker": "assistant", "text": "first answer"},
        }
        text = (
            '{"%s": {"chat": {"%s": {"id": "%s", "chatTitle": "Damaged", "interactions": [%s, '
            '{"humanMessage": {"speaker": "human", "text": "cut her'
            % (CODY_ACCOUNT, CODY_CHAT, CODY_CHAT, json.dumps(first))
        )
        (self.home / CODY_JB_REL).write_text(text, encoding="utf-8")
        rows = self.rows_for(CodyParser(), CODY_JB_REL)
        texts = [row.text for row in rows]
        self.assertIn("first question", texts)
        self.assertIn("first answer", texts)
        bad = [row for row in rows if row.text.startswith("parser: unparseable record")]
        self.assertEqual(len(bad), 1)
        self.assertTrue(bad[0].text.startswith("parser: unparseable record 2 ("))
        self.assertTrue(
            bad[0].text.endswith(': {"humanMessage": {"speaker": "human", "text": "cut her')
        )
        self.assertEqual(bad[0].session_id, CODY_CHAT)
        self.assertEqual(bad[0].turn_type, "system")
        self.assertEqual(bad[0].timestamp_utc, "")
        self.assertIn("parser: chat history did not parse; recovered 1 chat(s)", rows[-1].text)

    def deep_rows(self):
        from fixtures import CODY_DEEP_REL, build_cody_deep

        build_cody_deep(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        return self.rows_for(CodyParser(), CODY_DEEP_REL)

    def test_rejected_confirmation_says_command_rejected(self):
        # A refusal is saved as state error, error.message "Command rejected".
        from fixtures import CODY_DEEP_CHAT, CODY_REJECTED_STEP

        rows = self.deep_rows()
        self.assertEqual(
            [row.turn_type for row in rows],
            ["system", "user", "system", "system", "tool_use", "system", "assistant"],
        )
        rejected = rows[2]
        self.assertEqual(
            (rejected.text, rejected.tool_use_id, rejected.session_id, rejected.source_line),
            (
                "permission decision: rejected confirmation state=error"
                " error=Command rejected title=Terminal\nrm -rf build",
                CODY_REJECTED_STEP,
                CODY_DEEP_CHAT,
                1,
            ),
        )
        self.assertEqual(rejected.timestamp_utc, "2026-10-03T11:00:00.000Z")
        # The rows before and after the decisions still come out.
        self.assertEqual(rows[1].text, "what changed?")
        self.assertEqual((rows[4].tool_name, rows[4].text), ("Terminal", "ls -la"))
        self.assertEqual(rows[-1].text, "Only README.md changed.")

    def test_pending_confirmation_is_not_called_an_approval(self):
        # An approval never completes the step, so pending is either.
        from fixtures import CODY_PENDING_STEP

        pending = self.deep_rows()[3]
        self.assertEqual(
            (pending.text, pending.tool_use_id),
            (
                "permission decision: approved or unanswered confirmation state=pending"
                " title=Terminal\nls -la",
                CODY_PENDING_STEP,
            ),
        )

    def test_other_confirmation_error_is_printed_not_interpreted(self):
        history = json.loads((self.home / CODY_JB_REL).read_text(encoding="utf-8"))
        agentic = history[CODY_ACCOUNT]["chat"][CODY_AGENTIC_CHAT]
        step = agentic["interactions"][0]["assistantMessage"]["processes"][0]
        step.update(state="error", error={"name": "Error", "message": "aborted"})
        (self.home / CODY_JB_REL).write_text(json.dumps(history), encoding="utf-8")
        rows = self.rows_for(CodyParser(), CODY_JB_REL)
        self.assertEqual(
            rows[9].text,
            "permission decision: confirmation state=error error=aborted"
            " title=Run terminal command\npytest -q\nAllow Cody to run this command?",
        )

    # ---- settings, model preferences, legacy JetBrains history ----

    def settings_col(self):
        from fixtures import build_cody_settings

        build_cody_settings(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")

    def test_settings_permission_rows(self):
        from fixtures import CODY_MCP_TOKEN, CODY_SETTINGS_REL

        self.settings_col()
        art = self.artifact(CODY_SETTINGS_REL)
        self.assertEqual(art.agent, "vscode")
        self.assertIn(CodyParser, [type(parser) for parser in cli.parsers_for(art, by_agent())])
        rows = self.rows_for(CodyParser(), CODY_SETTINGS_REL)
        source = " (from %s, file mtime)" % CODY_SETTINGS_REL
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: cody.agentic.context.experimentalOptions.shell.allow"
                '=["git","ls"]' + source,
                "permission setting: cody.agentic.context.experimentalOptions.shell.block"
                '=["sudo"]' + source,
                "permission setting: cody.agentic.context.experimentalShell=true" + source,
                "permission setting: cody.chat.agenticContext=false" + source,
                'permission setting: cody.mcpServers.github.disabledTools=["delete_repo"]' + source,
                "permission setting: cody.mcpServers.web.disabled=true" + source,
            ],
        )
        mtime = to_utc((self.home / CODY_SETTINGS_REL).lstat().st_mtime)
        self.assertEqual(
            {
                (row.turn_type, row.agent, row.session_id, row.timestamp_utc, row.source_line)
                for row in rows
            },
            {("system", "cody", "", mtime, 1)},
        )
        self.assertNotIn(CODY_MCP_TOKEN, " ".join(row.text for row in rows))
        self.assertFalse(CodyParser().secondary(art))

    def test_settings_without_cody_yield_nothing(self):
        from fixtures import CODY_SETTINGS_REL

        self.settings_col()
        path = self.home / CODY_SETTINGS_REL
        path.write_text('{"editor.fontSize": 14}\n', encoding="utf-8")
        self.assertEqual(self.rows_for(CodyParser(), CODY_SETTINGS_REL), [])
        # A broken settings file is the editor's problem unless it names Cody.
        path.write_text('{"editor.fontSize": 14', encoding="utf-8")
        self.assertEqual(self.rows_for(CodyParser(), CODY_SETTINGS_REL), [])
        path.write_text('{"cody.chat.agenticContext": tru', encoding="utf-8")
        rows = self.rows_for(CodyParser(), CODY_SETTINGS_REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("system", "parser: settings.json is not a JSON object")],
        )

    def test_model_preferences_file(self):
        from fixtures import CODY_PREFS_REL

        self.settings_col()
        rows = self.rows_for(CodyParser(), CODY_PREFS_REL)
        source = "endpoint=https://sourcegraph.com/ (from %s, file mtime)" % CODY_PREFS_REL
        self.assertEqual(
            [(row.text, row.model) for row in rows],
            [
                (
                    "model setting: selected.chat=%s %s" % (CODY_MODEL_SWITCHED, source),
                    CODY_MODEL_SWITCHED,
                ),
                ("model setting: defaults.chat=%s %s" % (CODY_MODEL, source), CODY_MODEL),
                ("model setting: defaults.edit=%s %s" % (CODY_MODEL, source), CODY_MODEL),
            ],
        )
        mtime = to_utc((self.home / CODY_PREFS_REL).lstat().st_mtime)
        self.assertEqual({(row.session_id, row.timestamp_utc) for row in rows}, {("", mtime)})
        (self.home / CODY_PREFS_REL).write_text('{"https://', encoding="utf-8")
        self.assertEqual(
            [row.text for row in self.rows_for(CodyParser(), CODY_PREFS_REL)],
            ["parser: cody-model-preferences is not a JSON object"],
        )

    def test_model_preferences_in_vscode_row(self):
        self.settings_col()
        rows = self.rows_for(CodyParser(), CODY_VSCDB_REL)
        # The chat rows come first, unchanged.
        self.assertEqual([row.text for row in rows[:2]][1], "explain build.sh")
        source = "(from %s cody-model-preferences, file mtime)" % CODY_VSCDB_REL
        self.assertEqual(
            [row.text for row in rows[4:]],
            [
                "model setting: selected.chat=%s endpoint=https://sourcegraph.com/ %s"
                % (CODY_MODEL_SWITCHED, source),
                "model setting: defaults.chat=%s endpoint=https://sourcegraph.com/ %s"
                % (CODY_MODEL, source),
                "model setting: defaults.edit=%s endpoint=https://sourcegraph.com/ %s"
                % (CODY_MODEL, source),
            ],
        )
        self.assertEqual({row.source_line for row in rows[4:]}, {0})

    def test_not_wanted_beside_new_inputs(self):
        self.settings_col()
        for rel in (
            ".config/VSCodium/User/keybindings.json",
            ".local/share/Cody-nodejs/JetBrains-globalState/cody-anonymous-user-id",
        ):
            self.assertFalse(CodyParser().wants(self.artifact(rel)), rel)

    def legacy_artifact(self, text=None):
        from fixtures import CODY_LEGACY_REL, CODY_LEGACY_XML

        project = self.tmp / "srv" / "proj"
        path = project / CODY_LEGACY_REL
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(CODY_LEGACY_XML if text is None else text, encoding="utf-8")
        (project / ".idea/workspace.xml").write_text("<project />", encoding="utf-8")
        home = Home(disk_path=project, original="/srv/proj", user="alice", host="h1", project=True)
        return Artifact(path, "/srv/proj/" + CODY_LEGACY_REL, CODY_LEGACY_REL, "cody", home)

    def test_legacy_history_rows(self):
        from fixtures import CODY_LEGACY_ACCOUNT, CODY_LEGACY_ID, CODY_LEGACY_UPDATED

        art = self.legacy_artifact()
        parser = CodyParser()
        self.assertTrue(parser.wants(art))
        self.assertTrue(parser.secondary(art))
        self.assertTrue(parser.has_secondary)
        workspace = Artifact(
            art.disk_path.parent / "workspace.xml",
            "/srv/proj/.idea/workspace.xml",
            ".idea/workspace.xml",
            "project",
            art.home,
        )
        self.assertFalse(parser.wants(workspace))
        # A collector before 1.12.0 records the file as `project`; the
        # bundled project catalog names it `cody`.
        self.assertEqual(self.cat.project_agent_for(".idea/cody_history.xml"), "cody")
        rows = list(parser.parse(art, Options()))
        self.assertEqual(
            [(row.turn_type, row.session_id, row.text) for row in rows],
            [
                (
                    "system",
                    CODY_LEGACY_UPDATED,
                    "chat: %s | account=%s | messages=3 (legacy JetBrains history)"
                    % (CODY_LEGACY_ID, CODY_LEGACY_ACCOUNT),
                ),
                ("user", CODY_LEGACY_UPDATED, "hi\nthere"),
                ("system", CODY_LEGACY_UPDATED, "model: " + CODY_MODEL),
                ("assistant", CODY_LEGACY_UPDATED, "hello"),
                ("user", CODY_LEGACY_UPDATED, "unpaired question"),
                (
                    "system",
                    "2024-02-01T09:00:00.000001",
                    "chat: 1f8b7034-9fa8-488a-a13e-09c52677008b | account=%s"
                    " | messages=1 (legacy JetBrains history)" % CODY_LEGACY_ACCOUNT,
                ),
                ("user", "2024-02-01T09:00:00.000001", "second chat"),
            ],
        )
        self.assertEqual(rows[0].timestamp_utc, "2024-01-31T01:06:18.524Z")
        self.assertEqual((rows[3].model, rows[1].model), (CODY_MODEL, ""))
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})
        self.assertEqual({(row.agent, row.source_line) for row in rows}, {("cody", 0)})

    def test_legacy_history_dedups_against_migrated_chat(self):
        # The migration copied complete pairs into the agent store with
        # updatedAt as the chat id; only the unpaired message is recovered.
        from fixtures import CODY_LEGACY_UPDATED

        art = self.legacy_artifact()
        migrated = {
            "https://sourcegraph.com/-alice": {
                "chat": {
                    CODY_LEGACY_UPDATED: {
                        "id": CODY_LEGACY_UPDATED,
                        "lastInteractionTimestamp": CODY_LEGACY_UPDATED,
                        "interactions": [
                            {
                                "humanMessage": {
                                    "speaker": "human",
                                    "text": "hi\nthere",
                                    "model": CODY_MODEL,
                                },
                                "assistantMessage": {
                                    "speaker": "assistant",
                                    "text": "hello",
                                    "model": CODY_MODEL,
                                },
                            }
                        ],
                    }
                }
            }
        }
        (self.home / CODY_JB_REL).write_text(json.dumps(migrated), encoding="utf-8")
        primary = self.rows_for(CodyParser(), CODY_JB_REL)
        legacy = list(CodyParser().parse(art, Options()))
        kept, dropped = deduplicate(primary, [(row, 2.0) for row in legacy])
        recovered = [row.text for row in kept[len(primary) :] if row.turn_type != "system"]
        self.assertEqual(recovered, ["unpaired question", "second chat"])
        self.assertEqual(dropped["cody"], 3)

    def test_legacy_history_cut_keeps_earlier_chats(self):
        from fixtures import CODY_LEGACY_XML

        cut = CODY_LEGACY_XML.index(
            '<speaker value="HUMAN" />\n                      <text value="second'
        )
        rows = list(CodyParser().parse(self.legacy_artifact(CODY_LEGACY_XML[:cut]), Options()))
        texts = [row.text for row in rows]
        self.assertIn("hello", texts)
        self.assertIn("unpaired question", texts)
        self.assertNotIn("second chat", texts)
        self.assertEqual(rows[-1].turn_type, "system")
        self.assertTrue(
            rows[-1].text.startswith("parser: cody_history.xml did not parse; read 1 chat(s) (")
        )
        self.assertEqual((rows[-1].project_path, rows[-1].timestamp_utc), ("/srv/proj", ""))
        # A file that is not XML at all is one row, not an exception.
        rows = list(CodyParser().parse(self.legacy_artifact("not xml"), Options()))
        self.assertEqual(len(rows), 1)
        self.assertIn("read 0 chat(s)", rows[0].text)
