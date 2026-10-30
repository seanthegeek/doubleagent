"""Synthetic OpenHands state for the analyzer tests."""

from __future__ import annotations

import json
from pathlib import Path

from .common import _json_file

OPENHANDS_CONV = "0f0e0d0c111140008000000000000001"
OPENHANDS_CLI_CONV = "0f0e0d0c111140008000000000000002"
OPENHANDS_LEGACY = "legacy-sid-1"


def openhands_meta(cwd="/srv/proj"):
    return {
        "id": "0f0e0d0c-1111-4000-8000-000000000001",
        "title": "List files",
        "created_at": "2026-10-01T10:00:00Z",
        "updated_at": "2026-10-01T10:00:05Z",
        "workspace": {"kind": "LocalWorkspace", "working_dir": cwd},
        "secrets": {"GITHUB_TOKEN": "**********"},
    }


def openhands_base_state(conv_id, cwd="/srv/proj"):
    return {
        "id": conv_id,
        "execution_status": "finished",
        "agent": {
            "kind": "Agent",
            "llm": {"model": "litellm_proxy/claude-sonnet-4-5", "api_key": "**********"},
        },
        "workspace": {"kind": "LocalWorkspace", "working_dir": cwd},
    }


def openhands_events(prefix="6b1d5c2e-0000-4000-8000-00000000000", day="2026-10-01T12:00:"):
    return [
        {
            "kind": "SystemPromptEvent",
            "id": prefix + "0",
            "timestamp": day + "00.500000",
            "source": "agent",
            "system_prompt": {"type": "text", "text": "You are OpenHands agent."},
            "tools": [{"kind": "TerminalTool"}, {"kind": "FileEditorTool"}],
        },
        {
            "kind": "MessageEvent",
            "id": prefix + "1",
            "timestamp": day + "01.000000",
            "source": "user",
            "llm_message": {
                "role": "user",
                "content": [{"type": "text", "text": "list the files"}],
            },
            "activated_skills": [],
            "extended_content": [],
        },
        {
            "kind": "ActionEvent",
            "id": prefix + "2",
            "timestamp": day + "03.000000",
            "source": "agent",
            "thought": [{"type": "text", "text": "I will run ls."}],
            "action": {"kind": "TerminalAction", "command": "ls"},
            "tool_name": "terminal",
            "tool_call_id": "call_1",
            "tool_call": {
                "id": "call_1",
                "name": "terminal",
                "arguments": '{"command":"ls"}',
                "origin": "completion",
            },
            "llm_response_id": "resp_1",
            "security_risk": "LOW",
        },
        {
            "kind": "ObservationEvent",
            "id": prefix + "3",
            "timestamp": day + "04.000000",
            "source": "environment",
            "tool_name": "terminal",
            "tool_call_id": "call_1",
            "action_id": prefix + "2",
            "observation": {
                "kind": "TerminalObservation",
                "content": [{"type": "text", "text": "README.md"}],
                "is_error": False,
                "command": "ls",
                "exit_code": 0,
            },
        },
        {
            "kind": "ActionEvent",
            "id": prefix + "4",
            "timestamp": day + "05.000000",
            "source": "agent",
            "thought": [],
            "reasoning_content": "check the readme",
            "action": {
                "kind": "FileEditorAction",
                "command": "view",
                "path": "/srv/proj/README.md",
            },
            "tool_name": "file_editor",
            "tool_call_id": "call_2",
            "tool_call": {
                "id": "call_2",
                "name": "file_editor",
                "arguments": '{"command":"view","path":"/srv/proj/README.md"}',
                "origin": "completion",
            },
        },
        {
            "kind": "UserRejectObservation",
            "id": prefix + "5",
            "timestamp": day + "06.000000",
            "source": "user",
            "tool_name": "file_editor",
            "tool_call_id": "call_2",
            "action_id": prefix + "4",
            "rejection_reason": "not that file",
            "rejection_source": "user",
        },
        {
            "kind": "MessageEvent",
            "id": prefix + "6",
            "timestamp": day + "07.000000",
            "source": "agent",
            "llm_message": {
                "role": "assistant",
                "content": [{"type": "text", "text": "The project has a README."}],
            },
            "activated_skills": [],
            "extended_content": [],
        },
    ]


OPENHANDS_WT_CONV = "0f0e0d0c111140008000000000000003"
OPENHANDS_WT_DIR = "/tmp/conversation-worktrees/%s/proj" % OPENHANDS_WT_CONV
# sha256 of "/srv/proj", the CLI's directory name for that project.
OPENHANDS_PROJECT_HASH = "6b9af143446a411aefa3edb76e4693799bd7513e7ed8d919962d6713d3d3b972"


def openhands_wt_meta():
    """`meta.json` of an OpenHands conversation started with worktree isolation.

    meta.json of a conversation started with worktree isolation, so its working directory is under
    the Agent Server's default worktree root.
    """
    meta = openhands_meta(OPENHANDS_WT_DIR)
    meta.update(
        id="0f0e0d0c-1111-4000-8000-000000000003",
        title="Fix the tests",
        created_at="2026-10-03T09:00:00Z",
        updated_at="2026-10-03T09:00:09Z",
    )
    return meta


def openhands_wt_base_state():
    """`base_state.json` with standing permissions, a model and token usage.

    base_state.json with the standing permission fields, a model set over REST after the last
    response, and per-response token usage (SDK 33d8aa2 `conversation/state.py` and
    `llm/utils/metrics.py`).
    """
    state = openhands_base_state(OPENHANDS_WT_CONV, OPENHANDS_WT_DIR)
    state["agent"]["llm"]["model"] = "litellm_proxy/claude-opus-4-1"
    state["confirmation_policy"] = {
        "kind": "ConfirmRisky",
        "threshold": "HIGH",
        "confirm_unknown": True,
    }
    state["security_analyzer"] = {"kind": "LLMSecurityAnalyzer"}
    state["blocked_actions"] = {"8a2f0c1d-0000-4000-8000-000000000006": "rm is blocked by hook"}
    usages = [
        ("litellm_proxy/claude-sonnet-4-5", "resp_w1"),
        ("openai/gpt-5-mini", "resp_w2"),
        ("litellm_proxy/claude-haiku-4-5", "resp_w3"),
    ]
    state["stats"] = {
        "usage_to_metrics": {
            "agent": {
                "model_name": "litellm_proxy/claude-sonnet-4-5",
                "accumulated_cost": 0.0,
                "costs": [],
                "response_latencies": [],
                "token_usages": [
                    {
                        "model": model,
                        "prompt_tokens": 10,
                        "completion_tokens": 5,
                        "response_id": response_id,
                    }
                    for model, response_id in usages
                ],
            }
        }
    }
    return state


def openhands_wt_events(prefix="8a2f0c1d-0000-4000-8000-00000000000", day="2026-10-03T11:00:"):
    """OpenHands security verdicts, a model switch and a hook block.

    A security analyzer's verdicts, the agent's own model switch and a hook block (SDK 33d8aa2
    `event/security.py`, `tool/builtins/switch_llm.py`).
    """

    def action(number, sec, tool, call, command, response):
        return {
            "kind": "ActionEvent",
            "id": prefix + str(number),
            "timestamp": day + sec,
            "source": "agent",
            "thought": [],
            "action": {"kind": "TerminalAction", "command": command},
            "tool_name": tool,
            "tool_call_id": call,
            "tool_call": {"id": call, "name": tool, "arguments": "{}", "origin": "completion"},
            "llm_response_id": response,
        }

    def analysis(number, sec, action_n, risk, details=None):
        return {
            "kind": "SecurityAnalysisEvent",
            "id": prefix + str(number),
            "timestamp": day + sec,
            "source": "environment",
            "analyzer": "LLMSecurityAnalyzer",
            "policy": "ConfirmRisky",
            "risks": {prefix + str(action_n): risk},
            "details": {prefix + str(action_n): details} if details else {},
        }

    return [
        {
            "kind": "MessageEvent",
            "id": prefix + "0",
            "timestamp": day + "01.000000",
            "source": "user",
            "llm_message": {"role": "user", "content": [{"type": "text", "text": "fix the tests"}]},
        },
        action(1, "02.000000", "terminal", "call_w1", "pytest", "resp_w1"),
        analysis(2, "02.500000", 1, "LOW", {"rationale": "runs the test suite"}),
        {
            "kind": "ObservationEvent",
            "id": prefix + "3",
            "timestamp": day + "03.000000",
            "source": "environment",
            "tool_name": "terminal",
            "tool_call_id": "call_w1",
            "action_id": prefix + "1",
            "observation": {
                "kind": "TerminalObservation",
                "content": [{"type": "text", "text": "2 failed"}],
                "is_error": False,
            },
        },
        action(4, "04.000000", "switch_llm", "call_w2", "", "resp_w1"),
        {
            "kind": "ObservationEvent",
            "id": prefix + "5",
            "timestamp": day + "05.000000",
            "source": "environment",
            "tool_name": "switch_llm",
            "tool_call_id": "call_w2",
            "action_id": prefix + "4",
            "observation": {
                "kind": "SwitchLLMObservation",
                "content": [{"type": "text", "text": "Switched LLM profile to 'fast'"}],
                "is_error": False,
                "profile_name": "fast",
                "reason": "cheaper",
                "active_model": "openai/gpt-5-mini",
            },
        },
        action(6, "06.000000", "terminal", "call_w3", "rm -rf build", "resp_w2"),
        analysis(7, "06.500000", 6, "HIGH"),
        {
            "kind": "UserRejectObservation",
            "id": prefix + "8",
            "timestamp": day + "07.000000",
            "source": "environment",
            "tool_name": "terminal",
            "tool_call_id": "call_w3",
            "action_id": prefix + "6",
            "rejection_reason": "rm is blocked by hook",
            "rejection_source": "hook",
        },
        {
            "kind": "MessageEvent",
            "id": prefix + "9",
            "timestamp": day + "08.000000",
            "source": "agent",
            "llm_message": {"role": "assistant", "content": [{"type": "text", "text": "Done."}]},
            "llm_response_id": "resp_w3",
        },
    ]


def openhands_prompt_history():
    """The CLI's prompt_history.json (OpenHands-CLI 0403f1a `stores/prompt_history.py`).

    A JSON array, oldest first, naive local times. The first entry repeats the CLI conversation's
    prompt; the second is in no conversation.
    """
    return [
        {"text": "list the files", "timestamp": "2026-10-02T08:30:01.000000"},
        {"text": "and the hidden files?", "timestamp": "2026-10-02T08:45:00.000000"},
    ]


def _openhands_conversation(cdir: Path, events) -> None:
    for index, event in enumerate(events):
        path = cdir / "events" / ("event-%05d-%s.json" % (index, event["id"]))
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(event, separators=(",", ":")), encoding="utf-8")
    (cdir / "events" / (".eventlog-len-%d.marker" % len(events))).write_text("", encoding="utf-8")


def build_openhands(home: Path) -> None:
    openhands = home / ".openhands"
    conv = openhands / "agent-canvas/dev_conversations" / OPENHANDS_CONV
    _openhands_conversation(conv, openhands_events())
    _json_file(conv / "meta.json", openhands_meta())
    _json_file(conv / "base_state.json", openhands_base_state(OPENHANDS_CONV))
    # CLI conversation with no meta.json: naive times are emitted as if UTC.
    cli = openhands / "conversations" / OPENHANDS_CLI_CONV
    _openhands_conversation(
        cli, openhands_events("7c2e6d3f-0000-4000-8000-00000000000", "2026-10-02T08:30:")[:4]
    )
    _json_file(cli / "base_state.json", openhands_base_state(OPENHANDS_CLI_CONV))
    # A worktree conversation with the permission and model records.
    worktree_conversation = openhands / "agent-canvas/dev_conversations" / OPENHANDS_WT_CONV
    _openhands_conversation(worktree_conversation, openhands_wt_events())
    _json_file(worktree_conversation / "meta.json", openhands_wt_meta())
    _json_file(worktree_conversation / "base_state.json", openhands_wt_base_state())
    # The CLI's prompt history for /srv/proj, a secondary source.
    _json_file(
        openhands / "projects" / OPENHANDS_PROJECT_HASH / "prompt_history.json",
        openhands_prompt_history(),
    )
    legacy = openhands / "sessions" / OPENHANDS_LEGACY / "events"
    for index in range(3):
        _json_file(
            legacy / ("%d.json" % index),
            {
                "id": index,
                "timestamp": "2026-03-01T09:00:0%d.000000" % index,
                "source": "user",
                "action": "message",
                "args": {"content": "hi"},
            },
        )
    _json_file(
        openhands / "settings.json",
        {"llm_model": "anthropic/claude-sonnet-4-5", "llm_api_key": "sk-not-real"},
    )
    _json_file(openhands / "secrets.json", {"custom_secrets": {"X": {"secret": "not-real"}}})
