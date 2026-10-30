"""Synthetic Agent Zero state for the analyzer tests."""

from __future__ import annotations

import json
from pathlib import Path

AGENT_ZERO_CHAT = "AbCd1234"
AGENT_ZERO_REL = "agent-zero/usr/chats/%s/chat.json" % AGENT_ZERO_CHAT
AGENT_ZERO_LONG = "x" * 600


def _a0_message(mid, from_ai, content, metadata=None):
    return {
        "_cls": "Message",
        "id": mid,
        "ai": from_ai,
        "content": content,
        "metadata": metadata or {},
        "sequence": 0,
        "summary": "",
        "tokens": 5,
    }


def agent_zero_history():
    """Agent Zero's history of research/agent-zero.md, Parser plan, extended.

    Agent 0's history from research/agent-zero.md, Parser plan, with the assistant message carrying
    the id of the `agent` log item that streamed it (agent.py:529-534) and its provider model, a
    second tool result long enough to be saved to messages/1.txt, and a later assistant message from
    another provider model (a model change).
    """
    ai_reply = json.dumps(
        {
            "thoughts": ["list"],
            "headline": "Listing",
            "tool_name": "code_execution_tool",
            "tool_args": {"runtime": "terminal", "code": "ls"},
        }
    )
    return {
        "_cls": "History",
        "counter": 5,
        "bulks": [],
        "topics": [],
        "current": {
            "_cls": "Topic",
            "summary": "",
            "messages": [
                _a0_message("u1", False, {"user_message": "list files"}),
                _a0_message(
                    "a1",
                    True,
                    ai_reply,
                    {
                        "responses": {
                            "response_id": "resp_1",
                            "provider_model_key": "openrouter/anthropic/claude-sonnet-4",
                            "usage": {"input": 10, "output": 20},
                        }
                    },
                ),
                _a0_message(
                    "t1", False, {"tool_name": "code_execution_tool", "tool_result": "a.txt"}
                ),
                _a0_message(
                    "t2",
                    False,
                    {
                        "tool_name": "code_execution_tool",
                        "tool_result": AGENT_ZERO_LONG,
                        "file": "/a0/usr/chats/%s/messages/1.txt" % AGENT_ZERO_CHAT,
                    },
                ),
                _a0_message(
                    "a2",
                    True,
                    json.dumps({"thoughts": ["done"], "headline": "Checking again"}),
                    {
                        "responses": {
                            "response_id": "resp_2",
                            "provider_model_key": "openai/gpt-5",
                            "usage": {"input": 30, "output": 5},
                        }
                    },
                ),
            ],
        },
    }


def agent_zero_logs():
    return [
        {
            "no": 0,
            "id": "u1",
            "type": "user",
            "heading": "",
            "content": "list files",
            "kvps": {"attachments": []},
            "timestamp": 1790856001.0,
            "agentno": 0,
        },
        {
            "no": 1,
            "id": "a1",
            "type": "agent",
            "heading": "icon://network_intelligence A0: Listing",
            "content": "",
            "kvps": {
                "step": "Writing terminal command... (2)",
                "thoughts": ["list"],
                "headline": "Listing",
                "tool_name": "code_execution_tool",
                "tool_args": {"runtime": "terminal", "code": "ls"},
                "reasoning": "the user wants a listing",
            },
            "timestamp": 1790856002.0,
            "agentno": 0,
        },
        {
            "no": 2,
            "id": "t1",
            "type": "tool",
            "heading": "A0: Using tool 'code_execution_tool'",
            "content": "a.txt",
            "kvps": {"runtime": "terminal", "code": "ls", "_tool_name": "code_execution_tool"},
            "timestamp": 1790856003.0,
            "agentno": 0,
        },
        {
            "no": 3,
            "id": "t2",
            "type": "tool",
            "heading": "A0: Using tool 'code_execution_tool'",
            "content": AGENT_ZERO_LONG[:100] + "\n\n<< 500 Characters hidden >>\n\n",
            "kvps": {
                "runtime": "terminal",
                "code": "cat big.log",
                "_tool_name": "code_execution_tool",
            },
            "timestamp": 1790856004.0,
            "agentno": 0,
        },
        {
            "no": 4,
            "id": None,
            "type": "warning",
            "heading": "icon://warning Rate limit",
            "content": "retrying in 5s",
            "kvps": {},
            "timestamp": 1790856004.5,
            "agentno": 0,
        },
        {
            "no": 5,
            "id": "r1",
            "type": "response",
            "heading": "A0: Responding",
            "content": "There is one file, a.txt.",
            "kvps": {},
            "timestamp": 1790856005.0,
            "agentno": 0,
        },
        {
            "no": 6,
            "id": "a2",
            "type": "agent",
            "heading": "A0: Checking again",
            "content": "",
            "kvps": {"thoughts": ["done"], "headline": "Checking again"},
            "timestamp": 1790856006.0,
            "agentno": 0,
        },
    ]


def agent_zero_chat(logs=None, history=None):
    return {
        "id": AGENT_ZERO_CHAT,
        "name": "List files",
        "created_at": "2026-10-01T14:00:00+02:00",
        "type": "user",
        "last_message": "2026-10-01T14:00:05+02:00",
        "streaming_agent": 0,
        "agent_profile": "agent0",
        "data": {"project": "demo"},
        "output_data": {},
        "agents": [
            {
                "number": 0,
                "agent_profile": "agent0",
                "data": {},
                "history": json.dumps(
                    history if history is not None else agent_zero_history(), separators=(",", ":")
                ),
            }
        ],
        "log": {
            "guid": "6b0f0c1e-0000-4000-8000-000000000001",
            "progress": "",
            "progress_no": 5,
            "logs": logs if logs is not None else agent_zero_logs(),
        },
    }


def build_agent_zero(home: Path) -> None:
    """Write a docker-compose style Agent Zero install in `~/agent-zero`.

    A docker-compose style install in ~/agent-zero, with the long tool result file, and settings and
    secrets noise.
    """
    usr = home / "agent-zero/usr"
    chat = usr / "chats" / AGENT_ZERO_CHAT
    chat.mkdir(parents=True, exist_ok=True)
    (chat / "chat.json").write_text(
        json.dumps(agent_zero_chat(), ensure_ascii=False), encoding="utf-8"
    )
    (chat / "messages").mkdir(exist_ok=True)
    (chat / "messages/1.txt").write_text(AGENT_ZERO_LONG + " (from file)", encoding="utf-8")
    (usr / "settings.json").write_text('{"chat_model_provider": "openrouter"}', encoding="utf-8")
    (usr / "secrets.env").write_text("API_KEY_OPENROUTER=not-real\n", encoding="utf-8")
    (usr / "scheduler").mkdir(exist_ok=True)
    (usr / AGENT_ZERO_TASKS_REL.split("usr/", 1)[1]).write_text(
        json.dumps({"tasks": agent_zero_tasks()}), encoding="utf-8"
    )


# Scheduler tasks: `usr/scheduler/tasks.json` as the scheduler
# writes it, `{"tasks": [...]}` (agent0ai/agent-zero ff2a5b9
# helpers/task_scheduler.py:163-175, 512-519, 584). The scheduled task is
# bound to AGENT_ZERO_CHAT and its last result repeats the chat's response;
# its prompt is logged as `## Task:\n<prompt>` (task_scheduler.py:945-949),
# which the chat does not hold, so that row survives as recovered. The ad hoc
# task's chat is gone and it last failed. `token` is a trigger credential the
# parser must not print. Source-derived, not observed on any real install.
AGENT_ZERO_TASKS_REL = "agent-zero/usr/scheduler/tasks.json"
AGENT_ZERO_TASK_TOKEN = "4242424242424242424"


def agent_zero_tasks():
    return [
        {
            "uuid": "TaskUuid01",
            "context_id": AGENT_ZERO_CHAT,
            "state": "idle",
            "name": "Nightly listing",
            "system_prompt": "You run on a schedule.",
            "prompt": "list files",
            "attachments": [],
            "project_name": "demo",
            "project_color": None,
            "created_at": "2026-10-01T11:00:00Z",
            "updated_at": "2026-10-01T12:00:05.500000Z",
            "last_run": "2026-10-01T12:00:05.500000Z",
            "last_result": "There is one file, a.txt.",
            "type": "scheduled",
            "schedule": {"minute": "0", "hour": "2", "day": "*", "month": "*", "weekday": "*"},
        },
        {
            "uuid": "TaskUuid02",
            "context_id": "TaskUuid02",
            "state": "error",
            "name": "Webhook probe",
            "system_prompt": "",
            "prompt": "probe the webhook",
            "attachments": [],
            "project_name": None,
            "created_at": "2026-10-02T09:00:00Z",
            "updated_at": "2026-10-02T09:01:00Z",
            "last_run": "2026-10-02T09:01:00Z",
            "last_result": "ERROR: connection refused",
            "type": "adhoc",
            "token": AGENT_ZERO_TASK_TOKEN,
        },
    ]


# Standing settings, editor sessions and the connector, built only by
# AgentZeroTests; source-derived, not observed on any real install. Shapes from
# agent0ai/agent-zero ff2a5b9 (plugins/_a0_acp/api/session.py:12-20, 131-150;
# plugins/_model_config/api/model_override.py:58;
# plugins/_a0_connector/api/v1/model_switcher.py:27-46, 211;
# plugins/_tool_access/default_config.yaml:1-5; plugins/_orchestrator/default_config.yaml:7-37;
# helpers/plugins.py:60-61; plugins/_telegram_integration/helpers/handler.py:57-64;
# agent.py:1325-1346; helpers/tool_policy.py:197-208;
# plugins/_infection_check/helpers/checker.py:180-197) and agent0ai/a0-connector
# 3741c88 (src/agent_zero_cli/config.py:13-14).
AGENT_ZERO_ACP_CHAT = "AcpChat1"
AGENT_ZERO_ACP_REL = "agent-zero/usr/chats/%s/chat.json" % AGENT_ZERO_ACP_CHAT
AGENT_ZERO_TG_DELETED = "GoneTg01"
AGENT_ZERO_TG_STATE_REL = "agent-zero/usr/plugins/_telegram_integration/state.json"
AGENT_ZERO_POLICY_REL = "agent-zero/usr/agents/developer/plugins/_tool_access/config.json"
AGENT_ZERO_PROJECT_POLICY_REL = (
    "agent-zero/usr/projects/demo/.a0proj/plugins/_tool_access/config.json"
)
AGENT_ZERO_ORCHESTRATOR_REL = "agent-zero/usr/plugins/_orchestrator/config.json"
AGENT_ZERO_TOGGLE_REL = "agent-zero/usr/plugins/_infection_check/.toggle-1"
AGENT_ZERO_CONNECTOR_ENV_REL = ".agent-zero/.env"
AGENT_ZERO_OVERRIDE_KEY = "sk-fixture-override-key"
AGENT_ZERO_RESTORE_TOKEN = "fixture-restore-token"


def agent_zero_acp_chat():
    blocked = (
        "Traceback (most recent call last):\n  ...\nhelpers.errors.RepairableException: "
        'Tool "code_execution_tool" is blocked for agent profile "developer".'
    )
    logs = [
        {"no": 0, "id": "u1", "type": "user", "heading": "", "content": "run the tests",
         "kvps": {}, "timestamp": 1790870001.0, "agentno": 0},
        {"no": 1, "id": "w1", "type": "warning", "heading": "", "content": blocked,
         "kvps": {}, "timestamp": 1790870002.0, "agentno": 0},
        {"no": 2, "id": None, "type": "warning", "heading": "Responses MCP approval required",
         "content": "Responses MCP approval request received. Agent Zero denied it because "
         "provider-hosted MCP approval requires explicit user approval.",
         "kvps": {}, "timestamp": 1790870003.0, "agentno": 0},
        {"no": 3, "id": None, "type": "warning",
         "heading": "Infection check: clarification passed",
         "content": "Safety concern:\nreads a .env", "kvps": {}, "timestamp": 1790870004.0,
         "agentno": 0},
        {"no": 4, "id": "x1", "type": "warning", "heading": "Infection check: TERMINATED",
         "content": "Malicious behavior detected.", "kvps": {}, "timestamp": 1790870005.0,
         "agentno": 0},
    ]  # fmt: skip
    chat = agent_zero_chat(
        logs=logs,
        history={"_cls": "History", "bulks": [], "topics": [], "current": {"messages": []}},
    )
    chat["id"] = AGENT_ZERO_ACP_CHAT
    chat["name"] = "webapp"
    chat["data"] = {
        "project": "demo",
        "acp_session": True,
        "acp_cwd": "/home/alice/src/webapp",
        "acp_additional_directories": ["/home/alice/src/shared"],
        "acp_mode": "act",
        "acp_transport": "container",
        "acp_model_id": "openai/gpt-5",
        "acp_config_options": {},
        "workdir_path": "/a0/usr/workdir/acp",
        "chat_model_override": {
            "chat": {
                "provider": "openrouter",
                "name": "anthropic/claude-sonnet-4",
                "api_key": AGENT_ZERO_OVERRIDE_KEY,
            },
            "utility": {"provider": "ollama", "name": "qwen3"},
        },
    }
    return chat


def build_agent_zero_extras(home: Path) -> None:
    usr = home / "agent-zero/usr"
    files = {
        AGENT_ZERO_ACP_REL: json.dumps(agent_zero_acp_chat()),
        AGENT_ZERO_TG_STATE_REL: json.dumps(
            {
                "chats": {
                    "opsbot:123456789:987654321": AGENT_ZERO_TG_CHAT,
                    "opsbot:555:777": AGENT_ZERO_TG_DELETED,
                }
            }
        ),
        AGENT_ZERO_POLICY_REL: json.dumps(
            {
                "mode": "custom",
                "default": "allow",
                "mcp_default": "block",
                "allowed": [],
                "blocked": ["code_execution_tool", "mcp:github.create_issue"],
            }
        ),
        AGENT_ZERO_PROJECT_POLICY_REL: json.dumps({"mode": "inherit"}),
        AGENT_ZERO_ORCHESTRATOR_REL: json.dumps(
            {
                "a0": {"binary": "a0", "host": ""},
                "codex": {"binary": "codex", "model": "", "bypass_sandbox": True},
                "claude": {
                    "binary": "claude",
                    "permission_mode": "bypassPermissions",
                    "allowed_tools": "Bash,Read,Edit",
                },
                "hermes": {"binary": "hermes", "yolo": False},
            }
        ),
        AGENT_ZERO_TOGGLE_REL: "",
        # Noise the parser must not want: the Telegram bot's configuration.
        "agent-zero/usr/plugins/_telegram_integration/config.json": json.dumps(
            {"bots": [{"name": "opsbot", "token": AGENT_ZERO_BOT_TOKEN}]}
        ),
        AGENT_ZERO_CONNECTOR_ENV_REL: (
            "AGENT_ZERO_HOST=http://localhost:50080\n"
            "AGENT_ZERO_COMPUTER_USE_RESTORE_TOKEN=%s\n"
            "this line has no equals sign\n"
            "AGENT_ZERO_LAST_CONTEXT_HOST=https://a0.example.net\n"
            "AGENT_ZERO_LAST_CONTEXT_ID='RemCtx01'\n" % AGENT_ZERO_RESTORE_TOKEN
        ),
    }
    for rel, text in files.items():
        path = home / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
    assert usr.is_dir()


# Session sync: a Telegram chat whose main agent chained a turn
# on a provider-stored response, and a subordinate agent in local mode
# (ids recorded, nothing stored). Keys from research/agent-zero.md, Record schema
# (agent.py:1062-1082, plugins/_telegram_integration/helpers/constants.py:5-15
# at agent-zero e3051fb).
AGENT_ZERO_TG_CHAT = "TgChat01"
AGENT_ZERO_TG_REL = "agent-zero/usr/chats/%s/chat.json" % AGENT_ZERO_TG_CHAT
AGENT_ZERO_BOT_TOKEN = "123456:AA-FIXTURE-TELEGRAM-BOT-TOKEN"


def agent_zero_relayed_chat():
    chat = agent_zero_chat(
        logs=[
            {
                "no": 0,
                "id": "u1",
                "type": "user",
                "heading": "",
                "content": "status of the backups?",
                "kvps": {},
                "timestamp": 1790863200.0,
                "agentno": 0,
            },
            {
                "no": 1,
                "id": "r1",
                "type": "response",
                "heading": "",
                "content": "All backups ran.",
                "kvps": {},
                "timestamp": 1790863205.0,
                "agentno": 0,
            },
        ],
        history={"_cls": "History", "bulks": [], "topics": [], "current": {"messages": []}},
    )
    chat["id"] = AGENT_ZERO_TG_CHAT
    chat["name"] = "Telegram: @alice_ops"
    chat["data"] = {
        "project": "demo",
        "telegram_bot": "opsbot",
        "telegram_chat_id": 987654321,
        "telegram_chat_type": "private",
        "telegram_user_id": 123456789,
        "telegram_username": "alice_ops",
        "telegram_bot_cfg": {"name": "opsbot", "token": AGENT_ZERO_BOT_TOKEN},
    }
    chat["agents"][0]["data"] = {
        "responses_state": {
            "response_id": "resp_02",
            "previous_response_id": "resp_01",
            "provider_model_key": "openai/gpt-5",
            "history_counter": 4,
            "response_ids": ["resp_01", "resp_02"],
        }
    }
    sub = dict(chat["agents"][0])
    sub["number"] = 1
    sub["data"] = {
        "responses_state": {
            "response_id": "resp_l1",
            "previous_response_id": "",
            "provider_model_key": "codex/gpt-5-codex",
            "history_counter": 1,
            "response_ids": ["resp_l1"],
        }
    }
    chat["agents"].append(sub)
    return chat


def build_agent_zero_relayed(home: Path) -> None:
    path = home / AGENT_ZERO_TG_REL
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(agent_zero_relayed_chat()), encoding="utf-8")


# Docker volume layout: the README's `docker run -v a0_usr:/a0/usr`
# (Compose names it `<project>_a0_usr`) and the A0 Launcher's
# `a0-launcher-<slug>-usr` volume are mounted at /a0/usr, and the collector
# takes each volume's `_data` as the home, so the chats sit at `chats/` with
# no `agent-zero/usr/` prefix (agent0ai/agent-zero e3051fb README.md:92;
# agent0ai/a0-launcher b0333d8 shell/docker_manager/index.js:948-951). The
# Compose volume holds the folder layout with its long tool result file, the
# launcher volume a v0.8.0 legacy `chats/<ctxid>.json`; `settings.json` is
# the noise file. Source-derived, not observed on any real install.
AGENT_ZERO_VOLUME = "/var/lib/docker/volumes/proj_a0_usr/_data"
AGENT_ZERO_VOLUME_REL = "chats/%s/chat.json" % AGENT_ZERO_CHAT
AGENT_ZERO_LAUNCHER_VOLUME = "/var/lib/docker/volumes/a0-launcher-demo-usr/_data"
AGENT_ZERO_LEGACY_CHAT = "Legacy01"
AGENT_ZERO_LEGACY_REL = "chats/%s.json" % AGENT_ZERO_LEGACY_CHAT


def agent_zero_legacy_chat():
    chat = agent_zero_relayed_chat()
    chat["id"] = AGENT_ZERO_LEGACY_CHAT
    chat["name"] = "Legacy chat"
    chat["data"] = {}
    chat["agents"] = chat["agents"][:1]
    chat["agents"][0]["data"] = {}
    return chat


def build_agent_zero_volume(root: Path) -> Path:
    """Write a collected input of two Agent Zero volumes under `root`.

    A collected (manifest) input of two Agent Zero volumes under `root`, attributed as the collector
    writes volume rows (user `docker`, home the volume's `_data`); returns `root`.
    """
    files = {
        (AGENT_ZERO_VOLUME, AGENT_ZERO_VOLUME_REL): json.dumps(
            agent_zero_chat(), ensure_ascii=False
        ),
        (AGENT_ZERO_VOLUME, "chats/%s/messages/1.txt" % AGENT_ZERO_CHAT): AGENT_ZERO_LONG
        + " (from file)",
        (AGENT_ZERO_VOLUME, "settings.json"): '{"chat_model_provider": "openrouter"}',
        (AGENT_ZERO_LAUNCHER_VOLUME, AGENT_ZERO_LEGACY_REL): json.dumps(agent_zero_legacy_chat()),
    }
    root.mkdir(parents=True, exist_ok=True)
    (root / "collection.json").write_text(json.dumps({"hostname": "h3"}), encoding="utf-8")
    with open(root / "manifest.jsonl", "w", encoding="utf-8") as handle:
        for (volume, rel), text in files.items():
            path = volume + "/" + rel
            disk = root / ("fs" + path)
            disk.parent.mkdir(parents=True, exist_ok=True)
            disk.write_text(text, encoding="utf-8")
            row = {
                "user": "docker",
                "home": volume,
                "agent": "agent-zero",
                "path": path,
                "archive_path": "fs" + path,
                "type": "file",
                "status": "collected",
                "secret": rel == "settings.json",
            }
            handle.write(json.dumps(row) + "\n")
    return root
