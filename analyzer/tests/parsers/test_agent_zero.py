"""Tests of the Agent Zero parser."""

import json
import os

from doubleagent.inputs import open_input
from doubleagent.parsers import Options
from doubleagent.parsers.agent_zero import AgentZeroParser, recover_logs
from fixtures import (
    AGENT_ZERO_ACP_REL,
    AGENT_ZERO_BOT_TOKEN,
    AGENT_ZERO_CHAT,
    AGENT_ZERO_CONNECTOR_ENV_REL,
    AGENT_ZERO_LONG,
    AGENT_ZERO_ORCHESTRATOR_REL,
    AGENT_ZERO_OVERRIDE_KEY,
    AGENT_ZERO_POLICY_REL,
    AGENT_ZERO_PROJECT_POLICY_REL,
    AGENT_ZERO_REL,
    AGENT_ZERO_RESTORE_TOKEN,
    AGENT_ZERO_TASK_TOKEN,
    AGENT_ZERO_TASKS_REL,
    AGENT_ZERO_TG_CHAT,
    AGENT_ZERO_TG_DELETED,
    AGENT_ZERO_TG_REL,
    AGENT_ZERO_TG_STATE_REL,
    AGENT_ZERO_TOGGLE_REL,
    agent_zero_chat,
    agent_zero_history,
    agent_zero_relayed_chat,
    agent_zero_tasks,
)

from .base import ParserBase, rel_only


class AgentZeroTests(ParserBase):
    """Test the Agent Zero parser."""

    def test_recover_logs_takes_json_whitespace_only(self):
        for space in (" ", "\t", "\n", "\r\n"):
            text = '{"logs"%s:%s[{"a": 1}' % (space, space)
            self.assertEqual(recover_logs(text), [{"a": 1}], repr(space))
        for space in map(chr, (0x00A0, 0x2028, 0x0085)):
            text = '{"logs"%s: [{"a": 1}' % space
            self.assertEqual(recover_logs(text), [], repr(space))

    REL = AGENT_ZERO_REL

    def test_rows(self):
        rows = self.rows_for(AgentZeroParser(), self.REL)
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
                "assistant",
                "system",
                "assistant",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "agent-zero", AGENT_ZERO_CHAT),
            )
            self.assertEqual((row.project_path, row.git_branch), ("/a0/usr/projects/demo", ""))
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        start, user, model, asst, use, result, use2, result2, warn, resp, change, asst2 = rows
        self.assertEqual(
            (model.text, model.model, model.timestamp_utc, model.source_line),
            (
                "model: openrouter/anthropic/claude-sonnet-4",
                "openrouter/anthropic/claude-sonnet-4",
                "2026-10-01T12:00:02.000Z",
                1,
            ),
        )
        self.assertEqual(
            (change.text, change.model, change.timestamp_utc, change.source_line),
            (
                "model: openrouter/anthropic/claude-sonnet-4 -> openai/gpt-5",
                "openai/gpt-5",
                "2026-10-01T12:00:06.000Z",
                6,
            ),
        )
        self.assertEqual((asst2.text, asst2.model), ("Checking again\ndone", "openai/gpt-5"))
        self.assertEqual(
            (start.text, start.timestamp_utc),
            (
                "chat start | List files | type=user | profile=agent0 | project=demo",
                "2026-10-01T12:00:00.000Z",
            ),
        )
        self.assertEqual(
            (user.text, user.timestamp_utc), ("list files", "2026-10-01T12:00:01.000Z")
        )
        self.assertEqual(
            (asst.text, asst.model), ("Listing\nlist", "openrouter/anthropic/claude-sonnet-4")
        )
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text), ("code_execution_tool", "t1", "ls")
        )
        self.assertEqual((result.tool_use_id, result.text), ("t1", "a.txt"))
        self.assertEqual((use2.text, result2.tool_use_id), ("cat big.log", "t2"))
        self.assertEqual(
            result2.text, AGENT_ZERO_LONG + " (from file)"
        )  # messages/1.txt over the cut log
        self.assertEqual(warn.text, "warning: Rate limit | retrying in 5s")
        self.assertEqual(
            (resp.text, resp.timestamp_utc),
            ("There is one file, a.txt.", "2026-10-01T12:00:05.000Z"),
        )
        self.assertEqual([row.source_line for row in rows], [0, 0, 1, 1, 2, 2, 3, 3, 4, 5, 6, 6])

    def test_thinking_opt_in(self):
        rows = self.rows_for(AgentZeroParser(), self.REL, include_thinking=True)
        self.assertEqual(
            [row.text for row in rows if row.turn_type == "thinking"], ["the user wants a listing"]
        )

    def test_history_used_when_file_absent_and_log_cut(self):
        (self.home / "agent-zero/usr/chats" / AGENT_ZERO_CHAT / "messages/1.txt").unlink()
        rows = self.rows_for(AgentZeroParser(), self.REL)
        self.assertEqual(rows[7].text, AGENT_ZERO_LONG)

    def test_trimmed_log_adds_history_without_timestamps(self):
        history = agent_zero_history()
        history["bulks"] = [{"_cls": "Bulk", "summary": "earlier: set up the repo", "records": []}]
        logs = [
            {
                "no": number,
                "id": "x%d" % number,
                "type": "info",
                "heading": "",
                "content": "tick %d" % number,
                "kvps": {},
                "timestamp": 1790856100.0 + number,
                "agentno": 0,
            }
            for number in range(1000)
        ]
        (self.home / self.REL).write_text(
            json.dumps(agent_zero_chat(logs=logs, history=history)), encoding="utf-8"
        )
        rows = self.rows_for(AgentZeroParser(), self.REL)
        self.assertEqual(
            rows[1].text,
            "log trimmed to its last 1000 items; earlier turns follow from the agent "
            "history without timestamps",
        )
        early = rows[2:12]
        self.assertEqual(
            [(row.turn_type, row.timestamp_utc) for row in early],
            [
                ("system", ""),
                ("user", ""),
                ("system", ""),
                ("assistant", ""),
                ("tool_use", ""),
                ("tool_result", ""),
                ("tool_result", ""),
                ("system", ""),
                ("assistant", ""),
                ("system", "2026-10-01T12:01:40.000Z"),
            ],
        )
        self.assertEqual(early[0].text, "[summary] earlier: set up the repo")
        self.assertEqual(early[2].text, "model: openrouter/anthropic/claude-sonnet-4")
        self.assertEqual(
            (early[3].text, early[3].model),
            ("Listing\nlist", "openrouter/anthropic/claude-sonnet-4"),
        )
        self.assertEqual((early[4].tool_name, early[4].text), ("code_execution_tool", "ls"))
        self.assertEqual(
            early[7].text, "model: openrouter/anthropic/claude-sonnet-4 -> openai/gpt-5"
        )
        self.assertEqual(early[9].text, "info: tick 0")
        self.assertEqual(len(rows), 2 + 9 + 1000)

    def test_subordinate_agent_tracks_its_own_model(self):
        chat = agent_zero_chat()
        sub = {
            "_cls": "History",
            "counter": 1,
            "bulks": [],
            "topics": [],
            "current": {
                "_cls": "Topic",
                "summary": "",
                "messages": [
                    {
                        "_cls": "Message",
                        "id": "b1",
                        "ai": True,
                        "content": json.dumps({"headline": "Sub step"}),
                        "metadata": {"responses": {"provider_model_key": "ollama/qwen3"}},
                    }
                ],
            },
        }
        chat["agents"].append({"number": 1, "history": json.dumps(sub)})
        chat["log"]["logs"].append(
            {
                "no": 7,
                "id": "b1",
                "type": "agent",
                "content": "",
                "kvps": {"headline": "Sub step"},
                "timestamp": 1790856007.0,
                "agentno": 1,
            }
        )
        (self.home / self.REL).write_text(json.dumps(chat), encoding="utf-8")
        rows = self.rows_for(AgentZeroParser(), self.REL)
        self.assertEqual(
            [(row.turn_type, row.text, row.model) for row in rows[-2:]],
            [
                ("system", "model: ollama/qwen3 agent=1", "ollama/qwen3"),
                ("assistant", "[agent 1] Sub step", "ollama/qwen3"),
            ],
        )
        # Agent 0's tracker is untouched: its change row is still the only one.
        self.assertEqual(sum(1 for row in rows if row.text.startswith("model: ")), 3)

    def test_paths_and_noise(self):
        parser = AgentZeroParser()
        for rel in (
            "agent-zero/inst-1/usr/chats/Zz/chat.json",
            "Desktop/agent-zero/usr/chats/Zz/chat.json",
            "agent-zero/usr/chats/Zz.json",
            "agent-zero/tmp/chats/Zz/chat.json",
        ):
            self.assertTrue(parser.wants(rel_only(rel)), rel)
        for artifact in self.col.artifacts:
            if artifact.agent == "agent-zero" and artifact.rel not in (
                self.REL,
                AGENT_ZERO_TG_REL,
                AGENT_ZERO_TASKS_REL,
            ):
                self.assertFalse(parser.wants(artifact), artifact.rel)
        self.assertTrue(
            any(artifact.rel.endswith("messages/1.txt") for artifact in self.col.artifacts)
        )

    def test_truncated_file_is_reported_not_fatal(self):
        raw = json.dumps(agent_zero_chat())
        cut = raw.index('"id": "t2"')  # mid-file, inside the fourth log item
        (self.home / self.REL).write_text(raw[: cut + 20], encoding="utf-8")
        rows = self.rows_for(AgentZeroParser(), self.REL)
        self.assertEqual(rows[0].turn_type, "system")
        self.assertTrue(rows[0].text.startswith("parser: chat.json did not parse"), rows[0].text)
        self.assertEqual(
            [row.turn_type for row in rows[1:]], ["user", "assistant", "tool_use", "tool_result"]
        )
        self.assertEqual({row.session_id for row in rows}, {AGENT_ZERO_CHAT})
        self.assertEqual(rows[4].text, "a.txt")
        self.assertEqual(recover_logs('{"logs": [{"no": 0}, {"no": 1'), [{"no": 0}])

    # -- remote copies ----------------------------------------------------------

    def _relayed_rows(self, chat=None):
        path = self.home / AGENT_ZERO_TG_REL
        if chat is not None:
            path.write_text(json.dumps(chat), encoding="utf-8")
        os.utime(path, (1790863300, 1790863300))
        return self.rows_for(AgentZeroParser(), AGENT_ZERO_TG_REL)

    def test_stored_response_and_telegram_rows(self):
        self.assertTrue(AgentZeroParser.records_sync)
        rows = self._relayed_rows()
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows[1:3]],
            [
                (
                    "system",
                    AGENT_ZERO_TG_CHAT,
                    "2026-10-01T14:01:40.000Z",
                    "session synced: provider response resp_02 previous resp_01 model"
                    " openai/gpt-5 (2 response ids) agent=0 (from chat.json"
                    " agents[].data.responses_state, file mtime)",
                ),
                (
                    "system",
                    AGENT_ZERO_TG_CHAT,
                    "2026-10-01T14:01:40.000Z",
                    "session relayed: telegram bot opsbot chat 987654321 user 123456789"
                    " @alice_ops (from chat.json data, file mtime)",
                ),
            ],
        )
        self.assertTrue(rows[0].text.startswith("chat start"))
        self.assertEqual([row.turn_type for row in rows[3:]], ["user", "assistant"])
        # The local-mode subordinate agent recorded ids but stored nothing.
        self.assertEqual(len([row for row in rows if row.text.startswith("session synced:")]), 1)

    def test_bot_token_never_emitted(self):
        rows = self._relayed_rows()
        self.assertFalse(
            [
                row
                for row in rows
                if AGENT_ZERO_BOT_TOKEN in "|".join(map(str, row.as_dict().values()))
            ]
        )

    def test_chat_without_remote_copy_has_no_rows(self):
        rows = self.rows_for(AgentZeroParser(), self.REL)
        self.assertFalse(
            [row for row in rows if row.text.startswith(("session synced:", "session relayed:"))]
        )

    def test_whatsapp_and_email_rows(self):
        chat = agent_zero_relayed_chat()
        for agent in chat["agents"]:
            agent["data"] = {}
        chat["data"] = {
            "wa_chat_id": "15551234567@s.whatsapp.net",
            "wa_sender_number": "+15551234567",
            "email_thread_id": "<thread-01@example.com>",
            "email_sender": "",
        }
        rows = self._relayed_rows(chat)
        self.assertEqual(
            [row.text for row in rows[1:3]],
            [
                "session relayed: whatsapp chat 15551234567@s.whatsapp.net number +15551234567"
                " (from chat.json data, file mtime)",
                "session relayed: email thread <thread-01@example.com> from -"
                " (from chat.json data, file mtime)",
            ],
        )
        self.assertFalse([row for row in rows if row.text.startswith("session synced:")])

    # -- permissions, models, past use, origin ----------------------------------

    def _extras(self):
        from fixtures import build_agent_zero_extras

        build_agent_zero_extras(self.home)
        for rel in (
            AGENT_ZERO_ACP_REL,
            AGENT_ZERO_TG_STATE_REL,
            AGENT_ZERO_POLICY_REL,
            AGENT_ZERO_CONNECTOR_ENV_REL,
        ):
            os.utime(self.home / rel, (1790877600, 1790877600))
        self.col = open_input(self.tmp / "home", self.cat, host="h1")

    def test_flags_and_new_paths(self):
        parser = AgentZeroParser()
        self.assertTrue(AgentZeroParser.has_secondary)
        self.assertTrue(AgentZeroParser.records_origin)
        for rel, secondary in (
            ("agent-zero/inst-1/usr/scheduler/tasks.json", True),
            ("agent-zero/tmp/scheduler/tasks.json", True),
            ("scheduler/tasks.json", True),
            ("plugins/_telegram_integration/state.json", True),
            ("agent-zero/usr/plugins/_tool_access/config.json", False),
            ("agents/dev/plugins/_tool_access/config.json", False),
            (
                "agent-zero/usr/projects/p/.a0proj/agents/dev/plugins/_tool_access/config.json",
                False,
            ),
            ("Desktop/agent-zero/usr/plugins/_orchestrator/config.json", False),
            ("agent-zero/usr/plugins/_infection_check/.toggle-0", False),
            (".agent-zero/.env", False),
        ):
            self.assertTrue(parser.wants(rel_only(rel)), rel)
            self.assertEqual(parser.secondary(rel_only(rel)), secondary, rel)
        for rel in (
            "agent-zero/usr/plugins/_telegram_integration/config.json",
            "agent-zero/usr/plugins/_infection_check/config.json",
            "agent-zero/usr/plugins/_memory/.toggle-1",
            "agent-zero/usr/scheduler/tasks.yaml",
            "agent-zero/usr/.env",
            ".agent-zero/session_cookies.json",
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)
        self.assertEqual(parser.secondary_tolerance(rel_only("scheduler/tasks.json")), 60.0)

    def test_scheduler_tasks(self):
        rows = self.rows_for(AgentZeroParser(), AGENT_ZERO_TASKS_REL)
        self.assertEqual(
            [
                (row.turn_type, row.session_id, row.timestamp_utc, row.project_path, row.text)
                for row in rows
            ],
            [
                (
                    "system",
                    AGENT_ZERO_CHAT,
                    "2026-10-01T11:00:00.000Z",
                    "/a0/usr/projects/demo",
                    "scheduled task: Nightly listing type=scheduled state=idle",
                ),
                ("user", AGENT_ZERO_CHAT, "", "/a0/usr/projects/demo", "## Task:\nlist files"),
                (
                    "assistant",
                    AGENT_ZERO_CHAT,
                    "2026-10-01T12:00:05.500Z",
                    "/a0/usr/projects/demo",
                    "There is one file, a.txt.",
                ),
                (
                    "system",
                    "TaskUuid02",
                    "2026-10-02T09:00:00.000Z",
                    "",
                    "scheduled task: Webhook probe type=adhoc state=error",
                ),
                ("user", "TaskUuid02", "", "", "## Task:\nprobe the webhook"),
                (
                    "system",
                    "TaskUuid02",
                    "2026-10-02T09:01:00.000Z",
                    "",
                    "scheduled task failed: connection refused",
                ),
            ],
        )
        self.assertTrue(AgentZeroParser().secondary(self.artifact(AGENT_ZERO_TASKS_REL)))
        joined = "|".join(str(value) for row in rows for value in row.as_dict().values())
        self.assertNotIn(AGENT_ZERO_TASK_TOKEN, joined)
        self.assertNotIn("You run on a schedule.", joined)

    def test_scheduler_bad_record_and_bad_file(self):
        path = self.home / AGENT_ZERO_TASKS_REL
        tasks = agent_zero_tasks()
        path.write_text(json.dumps({"tasks": [tasks[0], "oops", tasks[1]]}), encoding="utf-8")
        rows = self.rows_for(AgentZeroParser(), AGENT_ZERO_TASKS_REL)
        # The records before and after the bad one are still read.
        self.assertEqual(
            [row.text for row in rows if row.text.startswith("scheduled task:")],
            [
                "scheduled task: Nightly listing type=scheduled state=idle",
                "scheduled task: Webhook probe type=adhoc state=error",
            ],
        )
        self.assertEqual(
            (rows[-1].text, rows[-1].source_line),
            ('parser: unparseable record 2 (not a JSON object): "oops"', 2),
        )
        path.write_text('{"tasks": [', encoding="utf-8")
        rows = self.rows_for(AgentZeroParser(), AGENT_ZERO_TASKS_REL)
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("parser: scheduler tasks file not JSON ("))

    def test_permission_decisions_and_chat_settings(self):
        self._extras()
        mtime = "2026-10-01T18:00:00.000Z"
        rows = self.rows_for(AgentZeroParser(), AGENT_ZERO_ACP_REL)
        settings = [(row.text, row.project_path, row.timestamp_utc) for row in rows[1:5]]
        self.assertEqual(
            settings,
            [
                ("permission mode: act (acp, as of file mtime)", "/home/alice/src/webapp", mtime),
                (
                    "model: requested openai/gpt-5 (acp, as of file mtime)",
                    "/home/alice/src/webapp",
                    mtime,
                ),
                (
                    "model: requested chat=openrouter/anthropic/claude-sonnet-4"
                    " utility=ollama/qwen3 (chat override, as of file mtime)",
                    "/home/alice/src/webapp",
                    mtime,
                ),
                (
                    "scratch directory: /a0/usr/workdir/acp (acp container transport,"
                    " from chat.json data.workdir_path, as of file mtime)",
                    "/a0/usr/workdir/acp",
                    mtime,
                ),
            ],
        )
        # The editor's workspace is the chat's project path.
        self.assertEqual(rows[0].project_path, "/home/alice/src/webapp")
        self.assertEqual(rows[5].project_path, "/home/alice/src/webapp")
        decisions = [
            (row.text, row.tool_name, row.timestamp_utc, row.source_line)
            for row in rows
            if row.text.startswith("permission decision:")
        ]
        self.assertEqual(
            decisions,
            [
                (
                    "permission decision: blocked tool=code_execution_tool profile=developer",
                    "code_execution_tool",
                    "2026-10-01T15:53:22.000Z",
                    1,
                ),
                (
                    "permission decision: denied automatically (provider MCP approval request)",
                    "mcp_approval_request",
                    "2026-10-01T15:53:23.000Z",
                    2,
                ),
                (
                    "permission decision: passed (infection check)",
                    "",
                    "2026-10-01T15:53:24.000Z",
                    3,
                ),
                (
                    "permission decision: terminated (infection check)",
                    "",
                    "2026-10-01T15:53:25.000Z",
                    4,
                ),
            ],
        )
        # Each follows the warning row it reads.
        for i, row in enumerate(rows):
            if row.text.startswith("permission decision:"):
                self.assertTrue(rows[i - 1].text.startswith("warning: "), rows[i - 1].text)
        joined = "|".join(str(value) for row in rows for value in row.as_dict().values())
        self.assertNotIn(AGENT_ZERO_OVERRIDE_KEY, joined)

    def test_override_preset_and_plain_chat(self):
        chat = agent_zero_chat()
        chat["data"]["chat_model_override"] = {"preset_name": "Fast"}
        (self.home / self.REL).write_text(json.dumps(chat), encoding="utf-8")
        rows = self.rows_for(AgentZeroParser(), self.REL)
        self.assertEqual(
            rows[1].text, "model: requested preset Fast (chat override, as of file mtime)"
        )
        self.assertEqual(rows[1].project_path, "/a0/usr/projects/demo")
        chat["data"]["chat_model_override"] = None
        (self.home / self.REL).write_text(json.dumps(chat), encoding="utf-8")
        rows = self.rows_for(AgentZeroParser(), self.REL)
        self.assertFalse(
            [row for row in rows if row.text.startswith(("model: requested", "scratch"))]
        )

    def test_telegram_deleted_chat(self):
        self._extras()
        rows = self.rows_for(AgentZeroParser(), AGENT_ZERO_TG_STATE_REL)
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    AGENT_ZERO_TG_DELETED,
                    "2026-10-01T18:00:00.000Z",
                    "session deleted: telegram bot opsbot chat 777 user 555 ctxid "
                    "%s (from state.json, no chat.json in the input, file mtime)"
                    % AGENT_ZERO_TG_DELETED,
                )
            ],
        )
        self.assertTrue(AgentZeroParser().secondary(self.artifact(AGENT_ZERO_TG_STATE_REL)))
        (self.home / AGENT_ZERO_TG_STATE_REL).write_text("{", encoding="utf-8")
        rows = self.rows_for(AgentZeroParser(), AGENT_ZERO_TG_STATE_REL)
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("parser: state.json not JSON ("), rows[0].text)

    def test_standing_settings(self):
        self._extras()
        rows = self.rows_for(AgentZeroParser(), AGENT_ZERO_POLICY_REL)
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: tool_access.%s profile=developer (from %s, file mtime)"
                % (entry, AGENT_ZERO_POLICY_REL)
                for entry in (
                    "mode=custom",
                    "default=allow",
                    "mcp_default=block",
                    "blocked=code_execution_tool",
                    "blocked=mcp:github.create_issue",
                )
            ],
        )
        self.assertEqual(
            {(row.session_id, row.project_path, row.timestamp_utc) for row in rows},
            {("", "", "2026-10-01T18:00:00.000Z")},
        )
        rows = self.rows_for(AgentZeroParser(), AGENT_ZERO_PROJECT_POLICY_REL)
        self.assertEqual(
            [(row.text, row.project_path) for row in rows],
            [
                (
                    "permission setting: tool_access.mode=inherit project=demo (from %s, file"
                    " mtime)" % AGENT_ZERO_PROJECT_POLICY_REL,
                    "/a0/usr/projects/demo",
                )
            ],
        )
        rows = self.rows_for(AgentZeroParser(), AGENT_ZERO_ORCHESTRATOR_REL)
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: orchestrator.%s (from %s, file mtime)"
                % (entry, AGENT_ZERO_ORCHESTRATOR_REL)
                for entry in (
                    "codex.bypass_sandbox=true",
                    "claude.permission_mode=bypassPermissions",
                    "claude.allowed_tools=Bash,Read,Edit",
                    "hermes.yolo=false",
                )
            ],
        )
        rows = self.rows_for(AgentZeroParser(), AGENT_ZERO_TOGGLE_REL)
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: infection_check=enabled (from %s, file mtime)"
                % AGENT_ZERO_TOGGLE_REL
            ],
        )
        (self.home / AGENT_ZERO_POLICY_REL).write_text("[1]", encoding="utf-8")
        rows = self.rows_for(AgentZeroParser(), AGENT_ZERO_POLICY_REL)
        self.assertEqual(
            [row.text for row in rows], ["parser: %s is not a JSON object" % AGENT_ZERO_POLICY_REL]
        )

    def test_connector_origin_reads_two_keys_only(self):
        self._extras()
        rows = self.rows_for(AgentZeroParser(), AGENT_ZERO_CONNECTOR_ENV_REL)
        self.assertEqual(
            [(row.turn_type, row.session_id, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "system",
                    "RemCtx01",
                    "2026-10-01T18:00:00.000Z",
                    "session origin: remote agent-zero instance https://a0.example.net context"
                    " RemCtx01 (from .agent-zero/.env, file mtime)",
                )
            ],
        )
        joined = "|".join(str(value) for row in rows for value in row.as_dict().values())
        for secret in (AGENT_ZERO_RESTORE_TOKEN, "localhost:50080", "no equals sign"):
            self.assertNotIn(secret, joined)

    def test_extras_noise_not_wanted(self):
        self._extras()
        parser = AgentZeroParser()
        noise = self.artifact("agent-zero/usr/plugins/_telegram_integration/config.json")
        self.assertFalse(parser.wants(noise))
        self.assertTrue(parser.wants(self.artifact(AGENT_ZERO_TOGGLE_REL)))

    # -- Docker volume layout ---------------------------------------------------

    def _volume(self):
        from fixtures import build_agent_zero_volume

        col = open_input(build_agent_zero_volume(self.tmp / "volumes"), self.cat)
        self.assertEqual(col.kind, "collected")
        return {(artifact.home.original, artifact.rel): artifact for artifact in col.artifacts}

    def test_volume_paths_selected(self):
        from fixtures import (
            AGENT_ZERO_LAUNCHER_VOLUME,
            AGENT_ZERO_LEGACY_REL,
            AGENT_ZERO_VOLUME,
            AGENT_ZERO_VOLUME_REL,
        )

        arts = self._volume()
        parser = AgentZeroParser()
        self.assertEqual(
            sorted(k for k, artifact in arts.items() if parser.wants(artifact)),
            sorted(
                [
                    (AGENT_ZERO_LAUNCHER_VOLUME, AGENT_ZERO_LEGACY_REL),
                    (AGENT_ZERO_VOLUME, AGENT_ZERO_VOLUME_REL),
                ]
            ),
        )
        self.assertEqual({artifact.agent for artifact in arts.values()}, {"agent-zero"})
        self.assertIn((AGENT_ZERO_VOLUME, "settings.json"), arts)
        self.assertTrue(parser.wants(rel_only("chats/Zz/chat.json")))
        self.assertTrue(parser.wants(rel_only("chats/Zz.json")))
        for rel in ("chats/Zz/messages/1.txt", "projects/demo/chats/Zz/chat.json", "chat.json"):
            self.assertFalse(parser.wants(rel_only(rel)), rel)

    def test_volume_chat_rows_match_home_install(self):
        from fixtures import AGENT_ZERO_VOLUME, AGENT_ZERO_VOLUME_REL

        art = self._volume()[(AGENT_ZERO_VOLUME, AGENT_ZERO_VOLUME_REL)]
        rows = list(AgentZeroParser().parse(art, Options()))
        home_rows = self.rows_for(AgentZeroParser(), self.REL)
        self.assertEqual(
            [
                (row.turn_type, row.timestamp_utc, row.text, row.model, row.tool_use_id)
                for row in rows
            ],
            [
                (row.turn_type, row.timestamp_utc, row.text, row.model, row.tool_use_id)
                for row in home_rows
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id, row.project_path),
                ("h3", "docker", "agent-zero", AGENT_ZERO_CHAT, "/a0/usr/projects/demo"),
            )
        # The long tool result comes from the volume's messages/<n>.txt.
        self.assertIn(AGENT_ZERO_LONG + " (from file)", [row.text for row in rows])

    def test_volume_legacy_chat_rows(self):
        from fixtures import (
            AGENT_ZERO_LAUNCHER_VOLUME,
            AGENT_ZERO_LEGACY_CHAT,
            AGENT_ZERO_LEGACY_REL,
        )

        art = self._volume()[(AGENT_ZERO_LAUNCHER_VOLUME, AGENT_ZERO_LEGACY_REL)]
        rows = list(AgentZeroParser().parse(art, Options()))
        self.assertEqual([row.turn_type for row in rows], ["system", "user", "assistant"])
        self.assertEqual({row.session_id for row in rows}, {AGENT_ZERO_LEGACY_CHAT})
        self.assertEqual(
            [row.text for row in rows[1:]], ["status of the backups?", "All backups ran."]
        )

    def test_volume_truncated_chat_is_reported_not_fatal(self):
        from fixtures import AGENT_ZERO_VOLUME, AGENT_ZERO_VOLUME_REL

        art = self._volume()[(AGENT_ZERO_VOLUME, AGENT_ZERO_VOLUME_REL)]
        raw = json.dumps(agent_zero_chat())
        cut = raw.index('"id": "t2"')
        art.disk_path.write_text(raw[: cut + 20], encoding="utf-8")
        rows = list(AgentZeroParser().parse(art, Options()))
        self.assertTrue(rows[0].text.startswith("parser: chat.json did not parse"), rows[0].text)
        self.assertEqual(
            [row.turn_type for row in rows[1:]], ["user", "assistant", "tool_use", "tool_result"]
        )
        self.assertEqual(
            {(row.user, row.session_id) for row in rows}, {("docker", AGENT_ZERO_CHAT)}
        )
