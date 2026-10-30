"""Tests of the Twinny parser."""

import io
import json
import sqlite3

from doubleagent import cli, vscode_state
from doubleagent.inputs import open_input
from doubleagent.parsers import Options
from doubleagent.parsers.twinny import TwinnyParser
from fixtures import (
    BAD_LINE,
    CODY_TOKEN,
    CODY_VSCDB_REL,
    TWINNY_415_CHANGE,
    TWINNY_415_CHANGES_REL,
    TWINNY_415_CONVERSATION,
    TWINNY_415_EMAIL,
    TWINNY_415_LOG_DIR,
    TWINNY_415_LOG_OLD_REL,
    TWINNY_415_LOG_REL,
    TWINNY_415_REL,
    TWINNY_415_WORKSPACE,
    TWINNY_415_WORKSPACE_DATABASE_REL,
    TWINNY_ACTIVE,
    TWINNY_API_KEY,
    TWINNY_CONVERSATION,
    TWINNY_SAID_CONVERSATION,
    TWINNY_SAID_REL,
    TWINNY_VSCDB_REL,
    _vscdb,
    build_twinny_415,
    build_twinny_said,
    twinny_said_records,
    write_bad_line,
)

from .base import ParserBase


class TwinnyTests(ParserBase):
    """Test the Twinny parser."""

    def test_rows(self):
        # The rows live only in the -wal sidecar: read through sqlite_util.
        self.assertTrue((self.home / (TWINNY_VSCDB_REL + "-wal")).is_file())
        rows = self.rows_for(TwinnyParser(), TWINNY_VSCDB_REL)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "user",
                "system",
                "assistant",
                "tool_use",
                "tool_result",
                "tool_use",
                "tool_result",
                "system",
                "system",
                "assistant",
                "tool_use",
                "tool_result",
                "system",
                "tool_use",
                "tool_result",
                "system",
                "system",
                "user",
            ],
        )
        for row in rows[:17]:
            self.assertEqual(
                (row.agent, row.session_id, row.timestamp_utc, row.source_line, row.project_path),
                ("twinny", TWINNY_CONVERSATION, "2026-10-03T12:00:05.000Z", 0, ""),
            )
        self.assertEqual(rows[0].text, "conversation: List the tests | messages=3")
        self.assertEqual(
            (rows[2].text, rows[2].model), ("model: qwen2.5-coder:7b", "qwen2.5-coder:7b")
        )
        self.assertEqual((rows[3].text, rows[3].model), ("Two tests fail.", "qwen2.5-coder:7b"))
        self.assertEqual(
            (rows[4].tool_name, rows[4].tool_use_id, rows[4].text),
            ("run_command", "step-1", '{"command":"npm test"}'),
        )
        self.assertEqual((rows[5].tool_use_id, rows[5].text), ("step-1", "2 failing"))
        self.assertEqual(rows[7].text, "[failed] permission denied")
        self.assertEqual(rows[8].text, 'secret shield withheld: [{"count":1,"kind":"aws-key"}]')
        self.assertEqual(
            (rows[9].text, rows[9].model),
            ("model: qwen2.5-coder:7b -> qwen2.5-coder:14b", "qwen2.5-coder:14b"),
        )
        self.assertEqual((rows[11].text, rows[12].text), ("rm -rf build", "[waiting]"))
        self.assertEqual(
            (rows[13].text, rows[13].tool_use_id, rows[13].tool_name),
            ("permission decision: waiting approval=command", "step-3", "run_command"),
        )
        self.assertEqual(rows[15].text, "[skipped] skipped by the user")
        self.assertEqual(
            (rows[16].text, rows[16].tool_use_id, rows[16].tool_name),
            ("permission decision: skipped", "step-4", "delete_file"),
        )
        self.assertEqual(
            {row.session_id for row in rows[17:]}, {TWINNY_ACTIVE}
        )  # unsaved active conversation
        self.assertEqual(rows[18].text, "unsaved question")

    def test_thinking_opt_in(self):
        rows = self.rows_for(TwinnyParser(), TWINNY_VSCDB_REL, include_thinking=True)
        self.assertEqual(
            [row.text for row in rows if row.turn_type == "thinking"], ["run the suite"]
        )
        self.assertEqual(
            rows[rows.index(next(row for row in rows if row.turn_type == "thinking")) + 1].text,
            "Two tests fail.",
        )

    def test_api_key_never_emitted(self):
        out = self.tmp / "out"
        import contextlib

        with contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(
                cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), "--host", "h1"]), 0
            )
        for name in ("timeline.jsonl", "sessions.jsonl"):
            text = (out / name).read_text(encoding="utf-8")
            self.assertNotIn(TWINNY_API_KEY, text, name)
            self.assertNotIn(CODY_TOKEN, text, name)
        self.assertIn(TWINNY_CONVERSATION, (out / "sessions.jsonl").read_text(encoding="utf-8"))
        # Nor from a value the parser cannot decode.
        database = self.home / ".config/Code - Insiders/User/globalStorage/state.vscdb"
        _vscdb(
            database,
            {"rjmacarthy.twinny": '{"twinny.inference-providers":{"apiKey":"%s"' % TWINNY_API_KEY},
        )
        col = open_input(self.tmp / "home", self.cat, host="h1")
        art = next(
            artifact
            for artifact in col.artifacts
            if artifact.rel == ".config/Code - Insiders/User/globalStorage/state.vscdb"
        )
        rows = list(TwinnyParser().parse(art, Options()))
        self.assertEqual([row.turn_type for row in rows], ["system"])
        self.assertTrue(
            rows[0].text.startswith(
                "parser: ItemTable value for rjmacarthy.twinny is not valid JSON"
            )
        )
        self.assertNotIn(TWINNY_API_KEY, rows[0].text)

    def test_secret_rows_never_read(self):
        database = self.home / TWINNY_VSCDB_REL
        key = 'secret://{"extensionId":"rjmacarthy.twinny","key":"gateway"}'
        self.assertEqual(vscode_state.read_item(database, key), (None, []))
        values, _ = vscode_state.read_items(database, [key, "rjmacarthy.twinny"])
        self.assertEqual(list(values), ["rjmacarthy.twinny"])

    def test_not_wanted(self):
        art = next(
            artifact
            for artifact in self.col.artifacts
            if artifact.rel
            == ".config/Code/User/globalStorage/rjmacarthy.twinny/twinny-providers.json"
        )
        self.assertEqual(art.agent, "twinny")
        self.assertFalse(TwinnyParser().wants(art))
        self.assertEqual(
            self.rows_for(TwinnyParser(), ".config/Code/User/globalStorage/state.vscdb"), []
        )
        self.assertEqual(self.rows_for(TwinnyParser(), CODY_VSCDB_REL), [])

    def test_truncated_line_is_reported_not_fatal(self):
        # A damaged SQLite file is one system row, not an exception.
        database = self.home / CODY_VSCDB_REL
        data = database.read_bytes()
        database.write_bytes(data[:100] + b"\x00" * (len(data) - 100))
        rows = self.rows_for(TwinnyParser(), CODY_VSCDB_REL)
        self.assertEqual([row.turn_type for row in rows], ["system"])
        self.assertIn("parser: state.vscdb unreadable", rows[0].text)

    def _said_rows(self, home):
        col = open_input(home, self.cat, host="h1")
        art = next(artifact for artifact in col.artifacts if artifact.rel == TWINNY_SAID_REL)
        return list(TwinnyParser().parse(art, Options()))

    def test_said_step_is_user_row(self):
        # Since 4.3.5 a message the user sends mid-reply is a `said: true`
        # tool step; it is a user row in its place, not a tool call.
        home = self.tmp / "said-home"
        build_twinny_said(home)
        rows = self._said_rows(home)
        self.assertEqual(
            [(row.turn_type, row.tool_name, row.tool_use_id, row.text) for row in rows],
            [
                ("system", "", "", "conversation: Install deps | messages=2"),
                ("user", "", "", "install the dependencies"),
                ("system", "", "", "model: qwen2.5-coder:7b"),
                ("assistant", "", "", "Installed with pnpm."),
                ("tool_use", "run_command", "step-1", '{"command":"npm install"}'),
                ("tool_result", "run_command", "step-1", "added 3 packages"),
                ("user", "", "", "use pnpm instead"),
                ("user", "", "", ""),
                ("user", "", "", '["and","add lodash"]'),
                ("tool_use", "run_command", "step-2", '{"command":"pnpm install"}'),
                ("tool_result", "run_command", "step-2", "done"),
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.agent, row.session_id, row.timestamp_utc, row.project_path),
                ("twinny", TWINNY_SAID_CONVERSATION, "2026-10-03T12:03:20.000Z", ""),
            )
        self.assertEqual({row.model for row in rows if row.turn_type == "user"}, {""})
        self.assertNotIn("said", {row.tool_name for row in rows})

    def test_said_value_cut_is_reported(self):
        # The same store with its value cut mid-write: one whole-file row and
        # no exception (the store is a single JSON value, not lines).
        home = self.tmp / "said-home"
        build_twinny_said(home)
        value = json.dumps(twinny_said_records())
        connection = sqlite3.connect(str(home / TWINNY_SAID_REL))
        connection.execute(
            "UPDATE ItemTable SET value = ? WHERE key = 'rjmacarthy.twinny'",
            (value[: len(value) // 2].encode("utf-8"),),
        )
        connection.commit()
        connection.close()
        rows = self._said_rows(home)
        self.assertEqual([row.turn_type for row in rows], ["system"])
        self.assertTrue(
            rows[0].text.startswith(
                "parser: ItemTable value for rjmacarthy.twinny is not valid JSON"
            )
        )

    # ---- 4.3.15 records beside the conversations -------------------------

    def _415(self):
        home = self.tmp / "twinny-415"
        build_twinny_415(home)
        col = open_input(home, self.cat, host="h1")
        return home, {artifact.rel: artifact for artifact in col.artifacts}

    def test_standing_grants_team_policy_and_changes(self):
        _, arts = self._415()
        art = arts[TWINNY_415_REL]
        rows = list(TwinnyParser().parse(art, Options()))
        source = " (from state.vscdb rjmacarthy.twinny, file mtime)"
        reply = "reply=" + TWINNY_415_CHANGE
        policy = "permission setting: twinny.teamPolicy."
        self.assertEqual(
            [row.text for row in rows],
            [
                "conversation: Fix the tests | messages=2",
                "fix the failing tests",
                "model: qwen2.5-coder:7b",
                "Fixed both.",
                "file changed: modified src/a.ts +3 -1 " + reply,
                "file changed: created logo.png +0 -0 %s noUndo=true" % reply,
                "permission mode: agentMode=true" + source,
                "permission setting: autoRunCommands=false" + source,
                "permission setting: alwaysRunCommands=npm test" + source,
                "permission setting: alwaysRunCommands=git status" + source,
                "permission setting: alwaysRunMcpTools=github/list_issues" + source,
                policy + "url=https://gateway.example.invalid" + source,
                policy + "policy.teamOnly=true" + source,
                policy + "policy.agent.commands=ask" + source,
                policy + "policy.agent.blockedPaths=secrets/" + source,
                policy + "policy.agent.blockedPaths=.env" + source,
                policy + "providerIds=team-0a1b-chat" + source,
                policy + "fetchedAt=2026-10-03T09:00:00.000Z" + source,
            ],
        )
        for row in rows[4:6]:
            self.assertEqual(
                (row.turn_type, row.session_id, row.timestamp_utc, row.project_path),
                ("system", TWINNY_415_CONVERSATION, "2026-10-03T12:05:00.000Z", ""),
            )
        for row in rows[6:]:
            self.assertEqual(
                (row.turn_type, row.session_id, row.timestamp_utc, row.project_path),
                ("system", "", "2026-10-03T12:10:00.000Z", ""),
            )
        # The ChatGPT plan profile and the selected model are never emitted.
        text = "\n".join(row.text for row in rows)
        self.assertNotIn(TWINNY_415_EMAIL, text)
        self.assertNotIn("selectedModel", text)
        self.assertFalse(TwinnyParser().secondary(art))

    def test_agent_changes_record_is_secondary(self):
        _, arts = self._415()
        art = arts[TWINNY_415_CHANGES_REL]
        self.assertEqual(art.agent, "twinny")
        self.assertTrue(TwinnyParser().wants(art))
        self.assertTrue(TwinnyParser().secondary(art))
        self.assertTrue(TwinnyParser.has_secondary)
        rows = list(TwinnyParser().parse(art, Options()))
        reply = "reply=" + TWINNY_415_CHANGE
        when = "2026-10-03T12:04:58.000Z"
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
                    when,
                    "file changed: modified src/a.ts +3 -1 %s undone=true" % reply,
                ),
                (
                    "system",
                    "",
                    "/srv/proj",
                    when,
                    "file changed: created logo.png +0 -0 %s noUndo=true" % reply,
                ),
            ],
        )
        # The text before the reply is never emitted.
        self.assertNotIn("export const a", "\n".join(row.text for row in rows))

    def test_agent_changes_record_cut_is_reported(self):
        home, arts = self._415()
        path = home / TWINNY_415_CHANGES_REL
        path.write_text(path.read_text(encoding="utf-8")[:40], encoding="utf-8")
        rows = list(TwinnyParser().parse(arts[TWINNY_415_CHANGES_REL], Options()))
        self.assertEqual([row.turn_type for row in rows], ["system"])
        self.assertTrue(
            rows[0].text.startswith("parser: agent-changes record is not valid JSON ("),
            rows[0].text,
        )

    def test_log_channel_rows(self):
        _, arts = self._415()
        art = arts[TWINNY_415_LOG_REL]
        self.assertTrue(TwinnyParser().secondary(art))
        rows = list(TwinnyParser().parse(art, Options()))
        source = " (from Twinny.log, log line time, host-local)"
        self.assertEqual(
            [
                (row.turn_type, row.session_id, row.timestamp_utc, row.source_line, row.text)
                for row in rows
            ],
            [
                (
                    "system",
                    "",
                    "2026-10-03T14:00:01.250Z",
                    1,
                    "log info: Chat → qwen2.5-coder:7b (Ollama) · 2 messages, 41 chars" + source,
                ),
                (
                    "system",
                    "",
                    "2026-10-03T14:00:01.251Z",
                    2,
                    "log debug: Chat messages"
                    + source
                    + "\n[user]\nwhich tests fail?\n\n[assistant]\nTwo tests fail.",
                ),
                (
                    "system",
                    "",
                    "2026-10-03T14:00:06.400Z",
                    8,
                    "log info: Chat ← 5.1s · 15 chars" + source,
                ),
            ],
        )
        old = list(TwinnyParser().parse(arts[TWINNY_415_LOG_OLD_REL], Options()))
        self.assertEqual(
            [row.text for row in old],
            [
                "log warning: Chat failed: connection refused"
                " (from Twinny.log_1, log line time, host-local)"
            ],
        )

    def test_log_truncated_line_is_reported_not_fatal(self):
        home, arts = self._415()
        log = home / TWINNY_415_LOG_REL
        write_bad_line(log)
        with open(log, "a", encoding="utf-8") as handle:
            handle.write(
                "\n    orphan continuation\n2026-10-03 14:00:07.000 [info] after the cut\n"
            )
        rows = list(TwinnyParser().parse(arts[TWINNY_415_LOG_REL], Options()))
        self.assertEqual(
            [row.text.split(" (from ")[0] for row in rows[:4]],
            [
                "log info: Chat → qwen2.5-coder:7b (Ollama) · 2 messages, 41 chars",
                "log debug: Chat messages",
                "log info: Chat ← 5.1s · 15 chars",
                "log info: after the cut",
            ],
        )
        self.assertEqual(
            [(row.text, row.source_line, row.session_id, row.timestamp_utc) for row in rows[4:]],
            [
                ("parser: unparseable line (no log line header): " + BAD_LINE, 9, "", ""),
                # An indented line after a bad line has no entry to join.
                (
                    "parser: unparseable line (no log line header):     orphan continuation",
                    10,
                    "",
                    "",
                ),
            ],
        )

    def test_workspace_review_repository(self):
        _, arts = self._415()
        art = arts[TWINNY_415_WORKSPACE_DATABASE_REL]
        self.assertEqual(art.agent, "vscode")
        rows = list(TwinnyParser().parse(art, Options()))
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    "",
                    "2026-10-03T12:10:00.000Z",
                    "source host repository: https://github.com/acme/widgets type=review"
                    " (from workspaceStorage state.vscdb rjmacarthy.twinny, file mtime)",
                )
            ],
        )
        self.assertFalse(TwinnyParser().secondary(art))

    def test_not_wanted_415(self):
        _, arts = self._415()
        exthost = TWINNY_415_LOG_DIR.rsplit("/", 1)[0] + "/exthost.log"
        for rel in (TWINNY_415_WORKSPACE + "/workspace.json", exthost):
            self.assertFalse(TwinnyParser().wants(arts[rel]), rel)
