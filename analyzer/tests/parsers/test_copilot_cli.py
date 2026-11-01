"""Tests of the GitHub Copilot CLI parser."""

import json
import sqlite3
from typing import ClassVar

from doubleagent import cli
from doubleagent.inputs import open_input
from doubleagent.parsers import Options
from doubleagent.parsers.antigravity import file_mtime
from doubleagent.parsers.copilot_cli import (
    LOG_TIME_PATTERN,
    CopilotCliParser,
    request_target,
    strip_jsonc,
)
from doubleagent.parsers.copilot_cli import result_text as copilot_result_text
from fixtures import (
    BAD_LINE,
    COPILOT_DIR,
    COPILOT_EVENT_LINES,
    COPILOT_EVENTS_REL,
    COPILOT_HISTORY_REL,
    COPILOT_LOG_REL,
    COPILOT_MC_LAST_EVENT,
    COPILOT_MC_SESSION,
    COPILOT_MC_TASK,
    COPILOT_PERMISSIONS_REL,
    COPILOT_SESSION,
    COPILOT_SETTINGS_REL,
    COPILOT_SIDEBAR_REL,
    COPILOT_SNAPSHOTS_REL,
    COPILOT_STORE_REL,
    COPILOT_WORKSPACE_REL,
    bad_line_text,
    write_bad_line,
)

from .base import ParserBase, rel_only


class CopilotCliTests(ParserBase):
    """Test the Copilot CLI parser."""

    def test_strip_jsonc_trailing_comma_takes_json_whitespace_only(self):
        # A comma before another space is no trailing comma, and one inside a
        # string is the string's.
        for space in (" ", "\t", "\n", "\r\n", " // c\n", " /* c */ "):
            text = '{"a": [1,%s],%s}' % (space, space)
            self.assertEqual(json.loads(strip_jsonc(text)), {"a": [1]}, repr(text))
        for space in map(chr, (0x00A0, 0x2028, 0x0085)):
            text = "[1,%s]" % space
            self.assertEqual(strip_jsonc(text), text, repr(text))
        text = '{"x": "a, ]", "y": "b,}"}'
        self.assertEqual(strip_jsonc(text), text)

    def test_log_time_takes_ascii_digits_only(self):
        line = "2026-10-01T08:59:59.400Z [INFO] x"
        self.assertTrue(LOG_TIME_PATTERN.match(line))
        self.assertIsNone(LOG_TIME_PATTERN.match(line.replace("1", chr(0x0661), 1)))

    # (source_line, turn_type, text) for every row of the fixture log
    # without thinking (research/copilot-cli.md, Parser plan, expected rows).
    EXPECTED: ClassVar[list[tuple[int, str, str]]] = [
        (1, "system", "session start: copilot 1.0.98 producer=copilot-agent cwd=/srv/proj"),
        (
            2,
            "system",
            "model: gpt-6.1-sol cause=initial_resolution source=automatic effort=medium",
        ),
        (
            3,
            "system",
            "system prompt: You are the GitHub Copilot CLI.\nFollow the user's instructions.",
        ),
        (4, "user", "fix the failing test"),
        (10, "system", "utility model gpt-4o-mini: Fix failing test"),
        (14, "tool_use", "npm test"),
        (15, "system", "permission request: shell npm test intention=Run the test suite"),
        (15, "system", "permission mode: manual agent=interactive"),
        (
            16,
            "system",
            "permission decision: approved-for-location source=human_response"
            ' location=/srv/proj approval={"commandIdentifiers":["npm test"],"kind":"commands"}',
        ),
        (
            18,
            "tool_result",
            "[exit 1] 1 failing\n  date.test.js: expected 2026\nCommand exited with code 1",
        ),
        (
            21,
            "tool_use",
            "*** Begin Patch\n*** Update File: /srv/proj/date.test.js\n-2025\n+2026\n*** End Patch",
        ),
        (
            22,
            "system",
            "permission request: write /srv/proj/date.test.js intention=Update the fixture year",
        ),
        (23, "system", "permission decision: approved source=human_response"),
        (25, "tool_result", "Modified 1 file: /srv/proj/date.test.js"),
        (28, "assistant", "The fixture year was stale; it now reads 2026."),
        (31, "system", "model: gpt-6.1-sol -> auto source=model_picker"),
        (
            32,
            "system",
            "session.info model: Model changed from GitHub Copilot \u2022 GPT-6.1 Sol (medium)"
            " to GitHub Copilot \u2022 Auto for this session",
        ),
        (33, "system", "session.info model: Auto routing profile change to Balance requested."),
        (34, "system", "session end: routine files modified: /srv/proj/date.test.js"),
        (35, "system", bad_line_text(COPILOT_EVENT_LINES[-1])),
    ]
    PATCH_ID = "custom_0000000000000000000000000001"

    def write_events(self, *records):
        """The fixture log without its cut line, then `records`, one per line."""
        lines = [*COPILOT_EVENT_LINES[:-1], *(json.dumps(record) for record in records)]
        (self.home / COPILOT_EVENTS_REL).write_text("\n".join(lines) + "\n", encoding="utf-8")

    def test_secondary(self):
        parser = CopilotCliParser()
        for rel in (COPILOT_STORE_REL, COPILOT_SNAPSHOTS_REL):
            self.assertTrue(parser.secondary(self.artifact(rel)), rel)
            self.assertEqual(parser.secondary_tolerance(self.artifact(rel)), 3600.0)
        self.assertTrue(parser.secondary(self.artifact(COPILOT_HISTORY_REL)))
        self.assertEqual(parser.secondary_tolerance(self.artifact(COPILOT_HISTORY_REL)), 2.0)
        for rel in (COPILOT_EVENTS_REL, COPILOT_WORKSPACE_REL, COPILOT_PERMISSIONS_REL):
            self.assertFalse(parser.secondary(self.artifact(rel)), rel)

    def test_rows(self):
        rows = self.rows_for(CopilotCliParser(), COPILOT_EVENTS_REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows], self.EXPECTED
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id, row.project_path, row.git_branch),
                ("h1", "alice", "copilot-cli", COPILOT_SESSION, "/srv/proj", "main"),
            )
        self.assertEqual(rows[0].timestamp_utc, "2026-10-01T09:00:00.000Z")
        self.assertEqual(rows[0].model, "")
        self.assertEqual({row.model for row in rows[1:4] + rows[5:-5]}, {"gpt-6.1-sol"})
        # The utility call names its own model and does not change the session's.
        self.assertEqual(rows[4].model, "gpt-4o-mini")
        # The `/model` switch to Auto, then its two notices with the new model.
        switch = rows[-5:-2]
        self.assertEqual({row.model for row in switch + rows[-2:-1]}, {"auto"})
        self.assertEqual(
            [(row.turn_type, row.tool_name, row.tool_use_id) for row in switch],
            [("system", "", "")] * 3,
        )
        self.assertEqual(
            [row.timestamp_utc for row in switch],
            ["2026-10-01T09:00:20.000Z", "2026-10-01T09:00:20.395Z", "2026-10-01T09:00:20.396Z"],
        )
        self.assertEqual(rows[-1].timestamp_utc, "")
        by_line = {(row.source_line, row.turn_type): row for row in rows}
        for line in (14, 15, 16, 18):
            row = next(row for row in rows if row.source_line == line)
            self.assertEqual((row.tool_name, row.tool_use_id), ("bash", "call_01"), line)
        for line in (21, 22, 23, 25):
            row = next(row for row in rows if row.source_line == line)
            self.assertEqual((row.tool_name, row.tool_use_id), ("apply_patch", self.PATCH_ID), line)
        self.assertEqual(by_line[(16, "system")].timestamp_utc, "2026-10-01T09:00:06.000Z")

    def test_thinking_opt_in(self):
        rows = self.rows_for(CopilotCliParser(), COPILOT_EVENTS_REL, include_thinking=True)
        thinking = [
            (row.source_line, row.text, row.model) for row in rows if row.turn_type == "thinking"
        ]
        self.assertEqual(thinking, [(14, "Run the tests first.", "gpt-6.1-sol")])
        self.assertEqual(len(rows), len(self.EXPECTED) + 1)
        # Without reasoningText the block summaries are used.
        self.write_events(
            {
                "id": "e1",
                "timestamp": "2026-10-01T09:02:00.000Z",
                "type": "assistant.message",
                "data": {
                    "content": "ok",
                    "model": "gpt-6.1-sol",
                    "reasoningBlocks": {
                        "blocks": [{"summary": [{"text": "One."}, {"text": "Two."}]}]
                    },
                },
            },
            {
                "id": "e2",
                "timestamp": "2026-10-01T09:02:01.000Z",
                "type": "assistant.reasoning",
                "data": {"reasoningId": "r1", "content": "Three."},
            },
        )
        rows = self.rows_for(CopilotCliParser(), COPILOT_EVENTS_REL, include_thinking=True)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows[-3:]],
            [("assistant", "ok"), ("thinking", "One.\n\nTwo."), ("thinking", "Three.")],
        )
        rows = self.rows_for(CopilotCliParser(), COPILOT_EVENTS_REL)
        self.assertEqual([row.turn_type for row in rows].count("thinking"), 0)

    def test_other_events(self):
        def event(second, etype, data, **extra):
            return {
                "id": "f%03d" % second,
                "timestamp": "2026-10-01T10:00:%02d.000Z" % second,
                "type": etype,
                "data": data,
                **extra,
            }

        self.write_events(
            event(1, "session.resume", {"context": {"cwd": "/srv/other", "branch": "dev"}}),
            event(
                2,
                "session.model_change",
                {"previousModel": "auto", "newModel": "auto", "source": "model_command"},
            ),
            event(
                3,
                "session.model_change",
                {"newModel": "claude-fable-5-1", "source": "model_picker"},
            ),
            event(
                4,
                "session.auto_mode_resolved",
                {"chosenModel": "gpt-6.2", "routingMethod": "router"},
            ),
            event(
                5, "session.permissions_changed", {"previousMode": "manual", "mode": "allow-all"}
            ),
            event(6, "session.mode_changed", {"previousMode": "interactive", "newMode": "plan"}),
            event(7, "user.message", {"content": "keep going", "isAutopilotContinuation": True}),
            event(8, "user.message", {"content": "inspect the logs"}, agentId="agent-1"),
            event(9, "user.message", {"content": "run the review", "source": "skill-review"}),
            event(
                10,
                "tool.user_requested",
                {"toolCallId": "call_u1", "toolName": "bash", "arguments": {"command": "ls"}},
            ),
            event(
                11,
                "tool.execution_start",
                {"toolCallId": "call_u1", "toolName": "bash", "arguments": {"command": "ls"}},
            ),
            event(
                12,
                "tool.execution_start",
                {
                    "toolCallId": "call_s1",
                    "toolName": "view",
                    "arguments": {"path": "/srv/other/a.txt"},
                    "model": "gpt-6.2",
                },
            ),
            event(
                13,
                "permission.requested",
                {
                    "permissionRequest": {
                        "kind": "mcp",
                        "toolCallId": "call_s1",
                        "serverName": "web",
                        "toolName": "fetch",
                    },
                    "permissionMode": "allow-all",
                    "agentMode": "plan",
                },
            ),
            event(
                14,
                "permission.completed",
                {
                    "toolCallId": "call_s1",
                    "decisionSource": "human_response",
                    "result": {"kind": "denied-interactively-by-user", "feedback": "not now"},
                },
            ),
            event(
                15,
                "permission.carriedForward",
                {"toolCallId": "call_s1", "decisionSource": "authorization_carry_forward"},
            ),
            event(
                16,
                "tool.execution_complete",
                {"toolCallId": "call_s1", "success": False, "error": {"message": "denied"}},
            ),
            event(17, "session.context_changed", {"cwd": "/srv/third", "branch": "feature"}),
            event(18, "session.compaction_complete", {"summaryContent": "Summary of the work."}),
            event(19, "abort", {"reason": "user_initiated"}),
            event(20, "session.task_complete", {"summary": "All done."}),
            event(21, "subagent.started", {"toolCallId": "call_x", "agentName": "explore"}),
            event(22, "hook.start", {"other": 1}),
            event(23, "assistant.turn_start", {"turnId": "9"}),
            event(24, "model.turn_started", {"model": "gpt-4o-mini"}),
            event(25, "some.future.type", {"x": 1}),
            event(
                26,
                "session.info",
                {
                    "infoType": "tip",
                    "message": "Try /plan.",
                    "url": "https://docs.example.org/plan",
                    "tip": "Press Shift+Tab",
                },
            ),
            event(
                27,
                "session.shutdown",
                {
                    "shutdownType": "error",
                    "errorReason": "network",
                    "codeChanges": {"filesModified": []},
                },
            ),
        )
        rows = self.rows_for(CopilotCliParser(), COPILOT_EVENTS_REL)[len(self.EXPECTED) - 1 :]
        got = [(row.turn_type, row.text, row.tool_name, row.tool_use_id, row.model) for row in rows]
        self.assertEqual(
            got,
            [
                ("system", "session resumed", "", "", "auto"),
                ("system", "model: auto -> auto source=model_command", "", "", "auto"),
                (
                    "system",
                    "model: auto -> claude-fable-5-1 source=model_picker",
                    "",
                    "",
                    "claude-fable-5-1",
                ),
                ("system", "model: claude-fable-5-1 -> gpt-6.2 (auto, router)", "", "", "gpt-6.2"),
                ("system", "permission mode: manual -> allow-all", "", "", "gpt-6.2"),
                ("system", "agent mode: interactive -> plan", "", "", "gpt-6.2"),
                ("system", "auto-continuation: keep going", "", "", "gpt-6.2"),
                ("system", "subagent task: inspect the logs", "", "", "gpt-6.2"),
                ("system", "skill-review: run the review", "", "", "gpt-6.2"),
                ("tool_use", "user requested: ls", "bash", "call_u1", "gpt-6.2"),
                ("tool_use", "/srv/other/a.txt", "view", "call_s1", "gpt-6.2"),
                # The two change events updated the tracked mode: no repeat.
                ("system", "permission request: mcp web/fetch", "view", "call_s1", "gpt-6.2"),
                (
                    "system",
                    "permission decision: denied-interactively-by-user source=human_response"
                    " feedback=not now",
                    "view",
                    "call_s1",
                    "gpt-6.2",
                ),
                (
                    "system",
                    "permission decision: carried-forward source=authorization_carry_forward",
                    "view",
                    "call_s1",
                    "gpt-6.2",
                ),
                ("tool_result", "denied", "view", "call_s1", "gpt-6.2"),
                ("system", "compaction summary: Summary of the work.", "", "", "gpt-6.2"),
                ("system", "abort: user_initiated", "", "", "gpt-6.2"),
                ("system", "session.task_complete: All done.", "", "", "gpt-6.2"),
                ("system", "subagent.started: explore", "", "", "gpt-6.2"),
                # No hookType and no input: the placeholders, never data.
                ("system", "hook - start: {}", "", "", "gpt-6.2"),
                (
                    "system",
                    "session.info tip: Try /plan. url=https://docs.example.org/plan"
                    " tip=Press Shift+Tab",
                    "",
                    "",
                    "gpt-6.2",
                ),
                ("system", "session end: error network files modified: none", "", "", "gpt-6.2"),
            ],
        )
        self.assertEqual((rows[0].project_path, rows[0].git_branch), ("/srv/other", "dev"))
        self.assertEqual((rows[-1].project_path, rows[-1].git_branch), ("/srv/third", "feature"))

    def test_result_text(self):
        # detailedContent is the complete text, unless content is longer (a
        # skill invocation puts the concise text in detailedContent).
        self.assertEqual(
            copilot_result_text({"result": {"content": "a", "detailedContent": "a b c"}}), "a b c"
        )
        self.assertEqual(
            copilot_result_text({"result": {"content": "x" * 50, "detailedContent": "short"}}),
            "x" * 50,
        )
        self.assertEqual(
            copilot_result_text({"result": {"content": "ok"}, "shellExecution": {"exitCode": 0}}),
            "ok",
        )
        self.assertEqual(request_target({"serverName": "web", "toolName": "fetch"}), "web/fetch")
        self.assertEqual(request_target({"kind": "memory"}), "")
        self.assertEqual(
            json.loads(strip_jsonc('{"a": "http://x", /* c */ "b": [1,], // d\n}')),
            {"a": "http://x", "b": [1]},
        )

    def test_secondary_sources_deduplicated(self):
        rows, _, _, dropped = cli.collect_rows(self.col, Options(), ["copilot-cli"])
        # The snapshot's prompt takes the log's prompt (the nearer time);
        # the database turn's prompt then collapses onto the snapshot's copy;
        # the turn's reply pairs with the log's reply 0.2 s earlier; the
        # sessions row's name with workspace.yaml's; both history lines go,
        # "fix the failing test" onto the log's prompt and "and run lint" onto
        # the database turn that is its only other copy.
        self.assertEqual(dropped["copilot-cli"], 6)
        store = "/alice/" + COPILOT_STORE_REL
        kept = [(row.turn_type, row.text) for row in rows if row.source_file == store]
        self.assertEqual(
            kept,
            [
                ("system", "file changed: /srv/proj/date.test.js tool=apply_patch"),
                ("user", "and run lint"),
            ],
        )
        snap = "/alice/" + COPILOT_SNAPSHOTS_REL
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows if row.source_file == snap],
            [("system", "rewind snapshot: 1 files +1 -1: /srv/proj/date.test.js")],
        )
        # The typed `/model` is in no other source: its history line stays,
        # undated.
        history = "/alice/" + COPILOT_HISTORY_REL
        kept_history = [
            (row.turn_type, row.text, row.timestamp_utc)
            for row in rows
            if row.source_file == history
        ]
        self.assertEqual(kept_history, [("user", "/model", "")])
        users = [row for row in rows if row.turn_type == "user"]
        self.assertEqual(
            [(row.text, row.source_file) for row in users],
            [
                ("fix the failing test", "/alice/" + COPILOT_EVENTS_REL),
                ("and run lint", store),
                ("/model", history),
            ],
        )

    def test_log_deleted_store_recovers(self):
        (self.home / COPILOT_EVENTS_REL).unlink()
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows, _, _, dropped = cli.collect_rows(self.col, Options(), ["copilot-cli"])
        texts = [
            (row.turn_type, row.text) for row in rows if row.turn_type in ("user", "assistant")
        ]
        # The snapshot keeps the first prompt, the database both turns, and
        # the history the typed `/model`, undated and so last.
        self.assertEqual(
            texts,
            [
                ("user", "fix the failing test"),
                ("assistant", "The fixture year was stale; it now reads 2026."),
                ("user", "and run lint"),
                ("user", "/model"),
            ],
        )
        self.assertEqual(dropped["copilot-cli"], 4)

    def test_store_rows(self):
        rows = self.rows_for(CopilotCliParser(), COPILOT_STORE_REL)
        self.assertEqual(
            [(row.source_line, row.timestamp_utc, row.turn_type, row.text) for row in rows],
            [
                (1, "2026-10-01T09:00:00.000Z", "system", "session name: Fix failing test"),
                (1, "2026-10-01T09:00:15.200Z", "user", "fix the failing test"),
                (
                    1,
                    "2026-10-01T09:00:15.200Z",
                    "assistant",
                    "The fixture year was stale; it now reads 2026.",
                ),
                (2, "2026-10-01T09:01:05.000Z", "user", "and run lint"),
                (
                    1,
                    "2026-10-01T09:00:12.500Z",
                    "system",
                    "file changed: /srv/proj/date.test.js tool=apply_patch",
                ),
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.session_id, row.project_path, row.model), (COPILOT_SESSION, "/srv/proj", "")
            )

    def test_store_other_tables(self):
        # Checkpoints, references and Forge events: empty on the real
        # install, so their shapes are bundle-derived.
        connection = sqlite3.connect(str(self.home / COPILOT_STORE_REL))
        session = COPILOT_SESSION
        connection.execute(
            "insert into checkpoints values (?, 1, 'Fix the test', 'The overview', NULL,"
            " 'Edited a file', NULL, NULL, 'Run lint', '2026-10-01T09:05:00.000Z')",
            (session,),
        )
        connection.execute(
            "insert into session_refs values (?, 'pr', '#12', 0, '2026-10-01T09:06:00.000Z')",
            (session,),
        )
        connection.execute(
            "insert into forge_trajectory_events values (?, 'call_01', 0, NULL, 'npm test',"
            " '1 failing', 1, NULL, NULL, '2026-10-01T09:00:08.000Z')",
            (session,),
        )
        connection.execute(
            "insert into forge_trajectory_events values (NULL, NULL, NULL, NULL, NULL, NULL,"
            " NULL, NULL, NULL, NULL)"
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(CopilotCliParser(), COPILOT_STORE_REL)[5:]
        self.assertEqual(
            [(row.turn_type, row.text, row.tool_use_id) for row in rows],
            [
                ("system", "reference: pr #12", ""),
                (
                    "system",
                    "checkpoint 1: Fix the test\noverview: The overview\nwork_done: Edited a file"
                    "\nnext_steps: Run lint",
                    "",
                ),
                ("tool_use", "npm test", "call_01"),
                ("tool_result", "[exit 1] 1 failing", "call_01"),
            ],
        )
        # The forge pair joins the log's call on its id and drops.
        _, _, _, dropped = cli.collect_rows(self.col, Options(), ["copilot-cli"])
        self.assertEqual(dropped["copilot-cli"], 8)

    def test_bad_store(self):
        (self.home / COPILOT_STORE_REL).write_text("not sqlite", encoding="utf-8")
        rows = self.rows_for(CopilotCliParser(), COPILOT_STORE_REL)
        self.assertEqual(
            [(row.turn_type, row.text, row.source_line, row.timestamp_utc) for row in rows],
            [("system", "parser: not a SQLite database", 0, "")],
        )

    def test_workspace_and_snapshot(self):
        rows = self.rows_for(CopilotCliParser(), COPILOT_WORKSPACE_REL)
        self.assertEqual(
            [
                (row.turn_type, row.text, row.timestamp_utc, row.session_id, row.project_path)
                for row in rows
            ],
            [
                (
                    "system",
                    "session name: Fix failing test",
                    "2026-10-01T09:00:00.000Z",
                    COPILOT_SESSION,
                    "/srv/proj",
                ),
                (
                    "system",
                    "session metadata: user_named=false forks=0 compactions=0 client=copilot-cli",
                    "2026-10-01T09:00:00.000Z",
                    COPILOT_SESSION,
                    "/srv/proj",
                ),
                (
                    "system",
                    self.SYNCED,
                    "2026-10-01T09:01:30.000Z",
                    COPILOT_SESSION,
                    "/srv/proj",
                ),
            ],
        )
        rows = self.rows_for(CopilotCliParser(), COPILOT_SNAPSHOTS_REL)
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc) for row in rows],
            [
                ("user", "fix the failing test", "2026-10-01T09:00:01.150Z"),
                (
                    "system",
                    "rewind snapshot: 1 files +1 -1: /srv/proj/date.test.js",
                    "2026-10-01T09:00:01.150Z",
                ),
            ],
        )
        self.assertEqual(
            {(row.session_id, row.project_path) for row in rows}, {(COPILOT_SESSION, "/srv/proj")}
        )

    SYNCED = (
        "session synced: remote session %s task %s last event %s remote_steerable=false"
        " (from workspace.yaml, file mtime)"
        % (COPILOT_MC_SESSION, COPILOT_MC_TASK, COPILOT_MC_LAST_EVENT)
    )

    def test_workspace_synced(self):
        # The `mc_` keys a resumed, synced session carries: one row at the
        # file's mtime; an empty key prints `-`, and a null or absent
        # `mc_session_id` (a session that is not synced) gives none.
        path = self.home / COPILOT_WORKSPACE_REL
        yaml = path.read_text(encoding="utf-8")
        rows = self.rows_for(CopilotCliParser(), COPILOT_WORKSPACE_REL)
        self.assertEqual(
            [
                (row.text, row.timestamp_utc)
                for row in rows
                if row.text.startswith("session synced:")
            ],
            [(self.SYNCED, "2026-10-01T09:01:30.000Z")],
        )
        cases = (
            (
                yaml.replace("mc_task_id: " + COPILOT_MC_TASK, "mc_task_id:").replace(
                    "remote_steerable: false\n", ""
                ),
                "session synced: remote session %s task - last event %s remote_steerable=-"
                " (from workspace.yaml, file mtime)" % (COPILOT_MC_SESSION, COPILOT_MC_LAST_EVENT),
            ),
            (yaml.replace("mc_session_id: " + COPILOT_MC_SESSION, "mc_session_id: null"), None),
            (yaml.replace("mc_session_id: " + COPILOT_MC_SESSION + "\n", ""), None),
        )
        for text, want in cases:
            path.write_text(text, encoding="utf-8")
            rows = self.rows_for(CopilotCliParser(), COPILOT_WORKSPACE_REL)
            synced = [row.text for row in rows if row.text.startswith("session synced:")]
            self.assertEqual(synced, [want] if want else [], text)
            self.assertEqual(len(rows), 3 if want else 2)

    SCRATCH = (
        "scratch directory: %s (working directory of a session the app pre-created and"
        " never prompted: no events.jsonl)"
    )

    def test_workspace_scratch_directory(self):
        # A session with an events.jsonl gives no scratch row.
        rows = self.rows_for(CopilotCliParser(), COPILOT_WORKSPACE_REL)
        self.assertEqual([row for row in rows if row.text.startswith("scratch directory:")], [])
        # A pool session: workspace.yaml and no events.jsonl. The row follows
        # the metadata row with its columns, the cwd as project_path.
        (self.home / COPILOT_EVENTS_REL).unlink()
        rows = self.rows_for(CopilotCliParser(), COPILOT_WORKSPACE_REL)
        self.assertEqual(
            [row.text.split(":")[0] for row in rows],
            ["session name", "session metadata", "scratch directory", "session synced"],
        )
        meta, scratch = rows[1], rows[2]
        self.assertEqual(scratch.text, self.SCRATCH % "/srv/proj")
        self.assertEqual(
            (scratch.turn_type, scratch.timestamp_utc, scratch.session_id, scratch.project_path),
            ("system", meta.timestamp_utc, meta.session_id, "/srv/proj"),
        )
        # Without a cwd there is no directory to name.
        path = self.home / COPILOT_WORKSPACE_REL
        path.write_text(
            path.read_text(encoding="utf-8").replace("cwd: /srv/proj\n", ""), encoding="utf-8"
        )
        rows = self.rows_for(CopilotCliParser(), COPILOT_WORKSPACE_REL)
        self.assertEqual([row for row in rows if row.text.startswith("scratch directory:")], [])

    def test_workspace_client(self):
        # The desktop app's sessions say `github/autopilot`; a file
        # without the key gives an empty value.
        path = self.home / COPILOT_WORKSPACE_REL
        yaml = path.read_text(encoding="utf-8")
        for value, line in (
            ("github/autopilot", "client_name: github/autopilot\n"),
            ("", ""),
        ):
            path.write_text(yaml.replace("client_name: copilot-cli\n", line), encoding="utf-8")
            rows = self.rows_for(CopilotCliParser(), COPILOT_WORKSPACE_REL)
            self.assertEqual(
                rows[1].text,
                "session metadata: user_named=false forks=0 compactions=0 client=" + value,
            )

    def test_desktop_app_events(self):
        # The Copilot Desktop app's bundled CLI: the allowAll* form of
        # session.permissions_changed and hook.start/hook.end pairs.
        def event(second, etype, data):
            return {
                "id": "d%03d" % second,
                "timestamp": "2026-10-01T11:00:%02d.000Z" % second,
                "type": etype,
                "data": data,
            }

        prompt = {"cwd": "/srv/proj", "prompt": "list the files\n  please", "sessionId": "s"}
        self.write_events(
            event(
                1,
                "session.permissions_changed",
                {
                    "previousAllowAllPermissions": False,
                    "allowAllPermissions": True,
                    "previousAllowAllPermissionMode": "off",
                    "allowAllPermissionMode": "on",
                },
            ),
            event(
                2,
                "hook.start",
                {"hookInvocationId": "h1", "hookType": "userPromptSubmitted", "input": prompt},
            ),
            event(
                3,
                "hook.end",
                {
                    "hookInvocationId": "h1",
                    "hookType": "userPromptSubmitted",
                    "success": True,
                    "output": {},
                },
            ),
            event(
                4,
                "hook.start",
                {
                    "hookInvocationId": "h2",
                    "hookType": "preToolUse",
                    "input": {"toolName": "bash", "toolArgs": {"command": "ls"}},
                    "parentToolCallId": "call_d1",
                },
            ),
            event(
                5,
                "hook.end",
                {
                    "hookInvocationId": "h2",
                    "hookType": "preToolUse",
                    "success": False,
                    "error": {"message": "hook timed out"},
                    "parentToolCallId": "call_d1",
                },
            ),
            # No permissionMode: the tracked allow-all mode is not repeated.
            event(
                6,
                "permission.requested",
                {
                    "permissionRequest": {
                        "kind": "shell",
                        "toolCallId": "call_d1",
                        "fullCommandText": "ls",
                    }
                },
            ),
            event(7, "session.permissions_changed", {"allowAllPermissions": False}),
        )
        rows = self.rows_for(CopilotCliParser(), COPILOT_EVENTS_REL)[len(self.EXPECTED) - 1 :]
        self.assertEqual(
            [(row.turn_type, row.text, row.tool_name, row.tool_use_id) for row in rows],
            [
                ("system", "permission mode: allow-all off -> on", "", ""),
                (
                    "system",
                    "hook userPromptSubmitted start: "
                    '{"cwd":"/srv/proj","prompt":"list the files\\n  please","sessionId":"s"}',
                    "",
                    "",
                ),
                ("system", "hook userPromptSubmitted end: success=true output={}", "", ""),
                (
                    "system",
                    'hook preToolUse start: {"toolArgs":{"command":"ls"},"toolName":"bash"}',
                    "",
                    "call_d1",
                ),
                (
                    "system",
                    "hook preToolUse end: success=false output={}"
                    ' error={"message":"hook timed out"}',
                    "",
                    "call_d1",
                ),
                ("system", "permission request: shell ls", "", "call_d1"),
                ("system", "permission mode: allow-all - -> -", "", ""),
            ],
        )

    def test_sidebar_fills_project(self):
        # Without workspace.yaml the snapshot takes its project from the
        # sidebar file; without a sessions.cwd the database does too.
        (self.home / COPILOT_WORKSPACE_REL).unlink()
        connection = sqlite3.connect(str(self.home / COPILOT_STORE_REL))
        connection.execute("update sessions set cwd = NULL")
        connection.commit()
        connection.close()
        self.assertFalse(CopilotCliParser().wants(rel_only(COPILOT_SIDEBAR_REL[:-5] + ".txt")))
        self.assertEqual(self.rows_for(CopilotCliParser(), COPILOT_SIDEBAR_REL), [])
        for rel in (COPILOT_SNAPSHOTS_REL, COPILOT_STORE_REL):
            rows = self.rows_for(CopilotCliParser(), rel)
            self.assertEqual({row.project_path for row in rows}, {"/srv/proj"}, rel)

    def test_history(self):
        rows = self.rows_for(CopilotCliParser(), COPILOT_HISTORY_REL)
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
                ("user", "and run lint", "", "", "", 1),
                ("user", "/model", "", "", "", 2),
                ("user", "fix the failing test", "", "", "", 3),
            ],
        )

    def test_standing_grants(self):
        art = self.artifact(COPILOT_PERMISSIONS_REL)
        rows = list(CopilotCliParser().parse(art, Options()))
        mtime = file_mtime(art)
        self.assertEqual(mtime, "2026-10-01T12:00:00.000Z")
        self.assertEqual(
            [(row.turn_type, row.text, row.project_path) for row in rows],
            [
                (
                    "system",
                    'permission setting: locations./srv/proj.tool_approvals={"kind":"write"}'
                    " (from .copilot/permissions-config.json, file mtime)",
                    "/srv/proj",
                ),
                (
                    "system",
                    "permission setting: locations./srv/proj.tool_approvals="
                    '{"commandIdentifiers":["npm test"],"kind":"commands"}'
                    " (from .copilot/permissions-config.json, file mtime)",
                    "/srv/proj",
                ),
            ],
        )
        rows += self.rows_for(CopilotCliParser(), COPILOT_SETTINGS_REL)
        self.assertEqual(
            rows[-1].text,
            "permission setting: allowedUrls=https://registry.example.org"
            " (from .copilot/settings.json, file mtime)",
        )
        for row in rows:
            self.assertEqual((row.timestamp_utc, row.session_id, row.source_line), (mtime, "", 1))

    def test_unreadable_settings(self):
        (self.home / COPILOT_SETTINGS_REL).write_text("{ // cut", encoding="utf-8")
        rows = self.rows_for(CopilotCliParser(), COPILOT_SETTINGS_REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("system", "parser: settings.json is not a JSON object")],
        )
        self.assertTrue(rows[0].timestamp_utc)

    def test_log(self):
        rows = self.rows_for(CopilotCliParser(), COPILOT_LOG_REL)
        self.assertEqual(
            [
                (row.source_line, row.timestamp_utc, row.turn_type, row.text, row.session_id)
                for row in rows
            ],
            [
                (
                    2,
                    "2026-10-01T08:59:59.400Z",
                    "system",
                    "signed in: https://github.com as octo-user",
                    COPILOT_SESSION,
                )
            ],
        )

    def test_workspace_yaml_line_ends_and_trims(self):
        from doubleagent.parsers.copilot_cli import read_flat_yaml

        path = self.home / "workspace-line-ends.yaml"
        path.write_text(
            "cwd: /srv/proj\u00a0\nsummary: a\u2028b\nname: n\n", encoding="utf-8", newline=""
        )
        self.assertEqual(
            read_flat_yaml(path), {"cwd": "/srv/proj\u00a0", "summary": "a\u2028b", "name": "n"}
        )
        path.write_text("cwd\u00a0: /srv/proj\n", encoding="utf-8", newline="")
        self.assertEqual(read_flat_yaml(path), {"cwd\u00a0": "/srv/proj"})
        with_lf = "id: x\ncwd: '/srv/proj'\nsummary: ~\n"
        path.write_text(with_lf, encoding="utf-8", newline="")
        expected = read_flat_yaml(path)
        self.assertEqual(expected, {"id": "x", "cwd": "/srv/proj", "summary": None})
        path.write_text(with_lf.replace("\n", "\r\n"), encoding="utf-8", newline="")
        self.assertEqual(read_flat_yaml(path), expected)

    def test_log_line_ends_and_trims(self):
        from doubleagent.parsers.copilot_cli import LOG_TIME_PATTERN, SIGNED_IN_PATTERN

        # The log's spacing is space and tab: a no-break space after the login is the login's.
        login = SIGNED_IN_PATTERN.search("Signed in to https://github.com as octo\u00a0 ")
        self.assertEqual(login and login.group(2), "octo\u00a0")
        self.assertIsNone(LOG_TIME_PATTERN.match("2026-10-01T08:59:59.400Z\u00a0[INFO] x"))
        self.assertIsNone(LOG_TIME_PATTERN.match("2026-10-01T08:59:59.400\u00a0Z [INFO] x"))
        # The time is digits, colons and a decimal point only.
        self.assertIsNone(LOG_TIME_PATTERN.match("2026-10-01T08h59:59Z [INFO] x"))
        path = self.home / COPILOT_LOG_REL
        with_lf = path.read_text(encoding="utf-8")
        expected = [
            (row.source_line, row.timestamp_utc, row.text)
            for row in self.rows_for(CopilotCliParser(), COPILOT_LOG_REL)
        ]
        path.write_text(with_lf.replace("\n", "\r\n"), encoding="utf-8", newline="")
        got = self.rows_for(CopilotCliParser(), COPILOT_LOG_REL)
        self.assertEqual([(row.source_line, row.timestamp_utc, row.text) for row in got], expected)
        # A U+2028 before the sign-in keeps it on its timestamped line.
        path.write_text(
            "2026-10-01T08:59:59.400Z [INFO] note\u2028Signed in to https://github.com as"
            " octo-user\n",
            encoding="utf-8",
        )
        got = self.rows_for(CopilotCliParser(), COPILOT_LOG_REL)
        self.assertEqual(
            [(row.source_line, row.timestamp_utc, row.text) for row in got],
            [(1, "2026-10-01T08:59:59.400Z", "signed in: https://github.com as octo-user")],
        )

    def test_not_wanted(self):
        parser = CopilotCliParser()
        for rel in (
            ".copilot/config.json",
            ".copilot/open-sessions-state.json",
            ".copilot/session-store.db-wal",
            COPILOT_DIR + "rewind-file-snapshots/tracking.json",
            COPILOT_DIR + "checkpoints/index.md",
        ):
            art = self.artifact(rel)
            self.assertEqual(art.agent, "copilot-cli")
            self.assertFalse(parser.wants(art), rel)
        # The XDG-era homes, and nothing outside a Copilot home.
        for rel in (
            ".local/state/.copilot/session-state/%s/events.jsonl" % COPILOT_SESSION,
            ".config/.copilot/permissions-config.json",
            ".local/state/.copilot/session-store.db",
        ):
            self.assertTrue(parser.wants(rel_only(rel)), rel)
        for rel in (
            "proj/.copilot/settings.json",
            ".copilot/session-state/not-a-session/events.jsonl",
            ".copilot/logs/session-1.log",
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)

    def test_truncated_line_is_reported_not_fatal(self):
        rows = self.rows_for(CopilotCliParser(), COPILOT_EVENTS_REL)
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].source_line, rows[-1].session_id, rows[-1].project_path),
            ("system", 35, COPILOT_SESSION, "/srv/proj"),
        )
        self.assertEqual(
            rows[-2].text, "session end: routine files modified: /srv/proj/date.test.js"
        )
        self.assertEqual(rows[-1].git_branch, "main")
        write_bad_line(self.home / COPILOT_EVENTS_REL)
        rows = self.rows_for(CopilotCliParser(), COPILOT_EVENTS_REL)
        self.assertEqual(len(rows), len(self.EXPECTED))
        self.assertEqual(rows[-1].text, bad_line_text(COPILOT_EVENT_LINES[-1] + BAD_LINE))
        # A bad line between good ones: its own row, every other row kept.
        lines = [*COPILOT_EVENT_LINES[:-1]]
        lines.insert(10, BAD_LINE)
        (self.home / COPILOT_EVENTS_REL).write_text("\n".join(lines) + "\n", encoding="utf-8")
        rows = self.rows_for(CopilotCliParser(), COPILOT_EVENTS_REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows[:-1]],
            [(turn, text) for _, turn, text in self.EXPECTED[:-1]],
        )
        self.assertEqual((rows[-1].source_line, rows[-1].text), (11, bad_line_text()))
        self.write_events()
        write_bad_line(self.home / COPILOT_EVENTS_REL)
        rows = self.rows_for(CopilotCliParser(), COPILOT_EVENTS_REL)
        self.assertEqual((rows[-1].source_line, rows[-1].text), (35, bad_line_text()))
        self.write_events()
        self.assertNotIn("parser:", self.rows_for(CopilotCliParser(), COPILOT_EVENTS_REL)[-1].text)
