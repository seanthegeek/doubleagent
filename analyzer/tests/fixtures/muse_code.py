"""Synthetic Muse Code state for the analyzer tests."""

from __future__ import annotations

import json
from pathlib import Path

from .common import _json_file, _jsonl

MUSE_SESSION = "01a0f000-0000-7000-8000-000000000001"
MUSE_CHILD = "0f3c2b1a-5d6e-4f70-8a9b-0c1d2e3f4a5b"
MUSE_RUN = "0b7e6f5a-4c3d-4e2f-9a1b-2c3d4e5f6a7b"
MUSE_MODEL = "muse-spark-1.3-contributor"
MUSE_MODEL_2 = "muse-spark-1.3-fast"
MUSE_TRUST_REL = ".config/muse/trust.json"
MUSE_DIR = ".local/share/muse/sessions/2026/10/01/%s/" % MUSE_SESSION
MUSE_REL = MUSE_DIR + "session.jsonl"
MUSE_CHILD_REL = MUSE_DIR + "subagent/%s/session.jsonl" % MUSE_CHILD
MUSE_HISTORY_REL = ".local/share/muse/tui-history.jsonl"
MUSE_T0 = 1790845200000000  # 1 October 09:00:00 UTC in microseconds


def _muse_env(session_id, seq, t_us, ptype, payload, version=1, cause=None, group="8000"):
    return {
        "schema_version": 1,
        "id": "00000000-0000-4000-%s-%012d" % (group, seq),
        "stream": {"kind": "session", "id": session_id},
        "sequence": seq,
        "recorded_at": t_us,
        "record_type": "event",
        "durability": "durable",
        "causation_id": cause,
        "payload_type": ptype,
        "payload_schema_version": version,
        "payload": payload,
    }


def _muse_run(session_id, seq, t_us, run_id, event, source_seq, group="8000"):
    return _muse_env(
        session_id,
        seq,
        t_us,
        "runtime.session",
        {
            "event": event,
            "kind": "run",
            "run_id": run_id,
            "source_run_record_id": "10000000-0000-4000-8000-%012d" % source_seq,
            "source_run_record_sequence": source_seq,
        },
        group=group,
    )


def _muse_frame(session_id, frame, group="8000"):
    children = [
        _muse_env(
            session_id,
            1,
            MUSE_T0,
            "runtime.session.permission_format_declared",
            {"format": "profile_v1", "schema_version": 1},
            group=group,
        ),
        _muse_env(
            session_id,
            2,
            MUSE_T0,
            "runtime.session.permission_profile_committed",
            {
                "actor": {"id": None, "kind": "runtime"},
                "cause": "new_session_default",
                "permission_epoch": 1,
                "resolved_snapshot": {
                    "approval": "on_request",
                    "reviewer": "auto_review",
                    "schema_version": 1,
                },
                "schema_version": 1,
                "source": {"display_name": "Auto-review", "id": ":auto-review", "kind": "built_in"},
            },
            group=group,
        ),
    ]
    return {
        "retained_frame": "session_permission_transaction",
        "frame_schema_version": 1,
        "outer_log_ordinal": 1,
        "transaction_id": "a1b2c3d4-0000-4000-8000-%012d" % frame,
        "children": [
            {"child_index": i, "record_json": json.dumps(child, separators=(",", ":"))}
            for i, child in enumerate(children)
        ],
        "content_sha256": "sha256:" + "0" * 64,
    }


def _muse_model(session_id, seq, t_us, run_id, group="8000"):
    return _muse_env(
        session_id,
        seq,
        t_us,
        "run.model.configured",
        {
            "kind": "run_model",
            "record": {
                "command_id": run_id,
                "display_label": MUSE_MODEL,
                "model_id": MUSE_MODEL,
                "profile_id": "tbh",
                "provider_id": "meta",
                "run_stream": {"id": run_id, "kind": "run"},
                "source": "startup",
            },
        },
        group=group,
    )


def muse_session_records():
    """The main Muse fixture of research/muse-code.md, Parser plan, extended.

    The main fixture of research/muse-code.md, Parser plan, less the cut line (tests append one with
    `write_bad_line`), plus a `session.opened.observed` record with a `security_mode`, an automated
    review verdict between the approval request and its decision, and a `/model` change after the
    run. The `security_mode`, `outcome`, `risk_level` and `user_authorization` values are synthetic:
    the research records the keys, not their values.
    """
    session, run, start = MUSE_SESSION, MUSE_RUN, MUSE_T0
    bash = {
        "chunk_id": "exec-1-1",
        "command": "npm test",
        "description": "Run the test suite",
        "exit_code": 1,
        "terminal_status": "completed",
        "output": "1 failing",
        "original_output_bytes": 9,
        "original_output_tokens": 3,
        "truncated": False,
    }
    effect = {
        "call_id": "call_01",
        "effect_id": "30000000-0000-7000-8000-000000000001",
        "model_call_index": 0,
        "task_id": "30000000-0000-7000-8000-000000000001",
        "task_stream": {"id": "30000000-0000-7000-8000-000000000001", "kind": "task"},
    }
    return [
        _muse_frame(session, 1),
        _muse_env(
            session,
            3,
            start + 100000,
            "runtime.session.metadata",
            {
                "kind": "metadata",
                "record": {
                    "build": {"semver": "1.4.2", "sha": "0123abcd45"},
                    "model_id": MUSE_MODEL,
                    "provider_id": "meta",
                    "tool_surface_version": "2",
                    "web_search_mode": "client",
                    "workspace_root": "/srv/proj",
                },
            },
        ),
        _muse_env(
            session,
            4,
            start + 200000,
            "session.workspace_branch.observed",
            {
                "kind": "workspace_branch",
                "record": {
                    "command_id": session,
                    "commit": "0123456789ab",
                    "reference": {"kind": "branch", "name": "main"},
                    "vcs": "git",
                    "workspace_root": "/srv/proj",
                },
            },
        ),
        _muse_env(
            session,
            5,
            start + 250000,
            "session.opened.observed",
            {
                "kind": "session_opened",
                "record": {
                    "resume": False,
                    "security_mode": "example_mode",
                    "session_id": session,
                    "workspace_kind": "git",
                },
            },
        ),
        _muse_env(
            session,
            5,
            start + 300000,
            "session.name.changed",
            {
                "authority_id": "d0d0d0d0-0000-4000-8000-000000000001",
                "new_name": "quiet-lyra",
                "operation_id": "e0e0e0e0-0000-4000-8000-000000000001",
                "previous_name": None,
                "session_id": session,
                "source": "automatic",
            },
            cause="f0f0f0f0-0000-4000-8000-000000000001",
        ),
        _muse_model(session, 6, start + 900000, run),
        _muse_env(
            session,
            7,
            start + 1000000,
            "runtime.user_intent.accepted",
            {
                "bindings": [],
                "delivery_policy": "session_current",
                "intent_id": run,
                "model_messages": [{"content": [{"kind": "text", "text": "fix the failing test"}]}],
                "refill_blocks": [{"kind": "text", "text": "fix the failing test"}],
                "semantic_kind": {"kind": "chat"},
                "source_session_id": session,
                "surface": "main",
                "wake_policy": "start_once",
            },
        ),
        _muse_run(
            session,
            8,
            start + 1000000,
            run,
            {"kind": "started", "prompt": "fix the failing test"},
            1,
        ),
        _muse_run(
            session,
            9,
            start + 3000000,
            run,
            {
                "kind": "reasoning_summary_delta",
                "message_id": "20000000-0000-4000-8000-000000000001",
                "summary_index": 0,
                "text": "Running the tests first.",
            },
            5,
        ),
        _muse_run(
            session,
            10,
            start + 3100000,
            run,
            {
                "kind": "reasoning_summary_committed",
                "message_id": "20000000-0000-4000-8000-000000000001",
                "provider_item_id": "rs_0001:rs_0002",
                "response_id": "resp_0001",
                "text": "Running the tests first.",
            },
            6,
        ),
        _muse_run(
            session,
            11,
            start + 3200000,
            run,
            {
                "encrypted_content": "Q-AAAA",
                "kind": "reasoning_committed",
                "message_id": "20000000-0000-4000-8000-000000000002",
                "provider_item_id": "rs_0001:rs_0002",
                "response_id": "resp_0001",
                "text": "",
            },
            7,
        ),
        _muse_run(
            session,
            12,
            start + 3300000,
            run,
            {
                "duration_ms": 2300,
                "finish_reason": "tool_calls",
                "kind": "model_completed",
                "model": MUSE_MODEL,
                "usage": {
                    "cache_read_tokens": 0,
                    "cache_write_tokens": 0,
                    "cached_tokens": 0,
                    "input_tokens": 900,
                    "output_tokens": 40,
                    "reasoning_tokens": 12,
                },
            },
            8,
        ),
        _muse_run(
            session,
            13,
            start + 3400000,
            run,
            {
                "kind": "assistant_tool_calls_committed",
                "message_id": "20000000-0000-4000-8000-000000000003",
                "response_id": "resp_0001",
                "tool_calls": [
                    {
                        "args": json.dumps(
                            {"command": "npm test", "description": "Run the test suite"}
                        ),
                        "call_id": "call_01",
                        "id": "fc_01",
                        "name": "bash",
                    }
                ],
            },
            9,
        ),
        _muse_env(
            session,
            14,
            start + 3500000,
            "tool_batch.effect.started",
            {
                "kind": "tool_batch_effect",
                "record": dict(
                    effect,
                    kind="started",
                    parallel_profile={"kind": "ineligible"},
                    tool_name="bash",
                ),
                "run_id": run,
            },
        ),
        {
            "retained_marker": "omitted_live_only",
            "schema_version": 1,
            "stream": {"kind": "session", "id": session},
            "position": {"id": "40000000-0000-4000-8000-000000000015", "sequence": 15},
            "omitted_record": {
                "record_type": "status",
                "durability": "ephemeral",
                "payload_type": "runtime.session",
                "payload_schema_version": 1,
                "payload_kind": "task",
                "omission_class": "task_tool_delta_v1",
            },
        },
        _muse_env(
            session,
            16,
            start + 4400000,
            "tool_batch.effect.terminal",
            {
                "kind": "tool_batch_effect",
                "record": dict(
                    effect,
                    kind="terminal",
                    outcome={
                        "kind": "completed",
                        "output_ref_count": 0,
                        "task_completion": {"kind": "terminal", "terminal": {"kind": "completed"}},
                    },
                ),
                "run_id": run,
            },
        ),
        _muse_run(
            session,
            17,
            start + 4500000,
            run,
            {
                "batch_id": "20000000-0000-4000-8000-000000000003",
                "kind": "tool_result_batch_committed",
                "results": [
                    {"text": json.dumps(bash), "tool_call_id": "call_01", "tool_call_index": 0}
                ],
            },
            14,
        ),
        _muse_env(
            session,
            18,
            start + 5000000,
            "runtime.session",
            {
                "event": {
                    "approval_subject": {
                        "kind": "tool_action",
                        "network": {
                            "host": "registry.example.org",
                            "port": 443,
                            "protocol": "https",
                        },
                        "origin": {"kind": "url", "url": "https://registry.example.org/pkg"},
                        "tool_name": "network",
                    },
                    "kind": "requested",
                    "pending_action_id": "50000000-0000-7000-8000-000000000001",
                    "raw_args": "https registry.example.org:443",
                    "task_id": "30000000-0000-7000-8000-000000000002",
                    "tool_call_id": "call_02",
                    "tool_name": "network",
                },
                "kind": "approval",
                "run_id": run,
            },
            version=3,
        ),
        _muse_env(
            session,
            19,
            start + 5900000,
            "runtime.session",
            {
                "event": {
                    "assessment": {
                        "outcome": "allow",
                        "rationale": "Fetching a package index is low risk.",
                        "risk_level": "low",
                        "user_authorization": "not_required",
                    },
                    "completed_at_ms": 1790845205900,
                    "duration_ms": 900,
                    "kind": "automated_review_completed",
                    "status": "completed",
                    "usage": {},
                },
                "kind": "approval",
                "run_id": run,
            },
            version=3,
        ),
        _muse_env(
            session,
            19,
            start + 6000000,
            "runtime.session",
            {
                "event": {
                    "amendment": None,
                    "decision": "approved",
                    "decision_source": {
                        "kind": "llm_judge",
                        "review_id": "60000000-0000-4000-8000-000000000001",
                    },
                    "kind": "decision_applied",
                    "pending_action_id": "50000000-0000-7000-8000-000000000001",
                    "policy_result": "allow",
                    "session_stream": {"id": session, "kind": "session"},
                },
                "kind": "approval",
                "run_id": run,
            },
            version=3,
        ),
        _muse_run(
            session,
            20,
            start + 7000000,
            run,
            {
                "child_session_id": MUSE_CHILD,
                "child_session_log_path": "subagent/%s/session.jsonl" % MUSE_CHILD,
                "generation_id": 1,
                "kind": "memory_reminder_child_session_linked",
                "parent_run_id": run,
                "parent_session_id": session,
                "reminder_agent_id": "verify-reminder",
                "task_id": "30000000-0000-7000-8000-000000000003",
                "task_stream": {"id": "30000000-0000-7000-8000-000000000003", "kind": "task"},
            },
            20,
        ),
        _muse_run(
            session,
            21,
            start + 9000000,
            run,
            {
                "kind": "assistant_message_committed",
                "message_id": "20000000-0000-4000-8000-000000000004",
                "provider_item_id": "msg_0001",
                "response_id": "resp_0002",
                "text": "The test fails because the fixture date is stale.",
            },
            30,
        ),
        _muse_run(
            session,
            22,
            start + 9100000,
            run,
            {
                "eot_gate_ms": 3,
                "kind": "terminal",
                "reason": None,
                "terminal": "completed",
                "time_to_first_token_ms": 800,
                "turn_duration_ms": 8100,
            },
            31,
        ),
        _muse_env(
            session,
            23,
            start + 15000000,
            "runtime.model_reconfigure.completed",
            {
                "kind": "model_reconfigure",
                "record": {
                    "effective": {"model_id": MUSE_MODEL_2},
                    "previous": {"model_id": MUSE_MODEL},
                },
            },
        ),
        _muse_env(
            session,
            23,
            start + 20000000,
            "session.end",
            {
                "kind": "session_end",
                "record": {
                    "exit_reason": "clean",
                    "schema_version": 1,
                    "session_id": session,
                    "uptime_ms": 20000,
                },
            },
        ),
    ]


def muse_subagent_records():
    child, start, group = MUSE_CHILD, MUSE_T0, "9000"
    return [
        _muse_frame(child, 2, group=group),
        _muse_env(
            child,
            3,
            start + 7100000,
            "runtime.session.metadata",
            {"kind": "metadata", "record": {"model_id": MUSE_MODEL, "provider_id": "meta"}},
            group=group,
        ),
        _muse_model(child, 4, start + 7100000, child, group=group),
        _muse_run(
            child,
            5,
            start + 7200000,
            child,
            {"kind": "started", "prompt": "You are a reminder observer for the main agent."},
            1,
            group=group,
        ),
        _muse_run(
            child,
            6,
            start + 8000000,
            child,
            {
                "kind": "assistant_tool_calls_committed",
                "message_id": "20000000-0000-4000-8000-000000000010",
                "response_id": "resp_0010",
                "tool_calls": [
                    {
                        "args": json.dumps(
                            {"decision": "none", "next_step": None, "reason": "Tests were run."}
                        ),
                        "call_id": "call_10",
                        "id": "fc_10",
                        "name": "submit_reminder_decision",
                    }
                ],
            },
            13,
            group=group,
        ),
        _muse_run(
            child,
            7,
            start + 8100000,
            child,
            {
                "batch_id": "20000000-0000-4000-8000-000000000010",
                "kind": "tool_result_batch_committed",
                "results": [
                    {
                        "text": "reminder decision recorded",
                        "tool_call_id": "call_10",
                        "tool_call_index": 0,
                    }
                ],
            },
            16,
            group=group,
        ),
        _muse_run(
            child,
            8,
            start + 8199999,
            child,
            {
                "kind": "terminal",
                "reason": None,
                "terminal": "completed",
                "time_to_first_token_ms": 500,
                "turn_duration_ms": 1000,
            },
            18,
            group=group,
        ),
    ]


def build_muse_code(home: Path) -> None:
    _jsonl(home / MUSE_REL, muse_session_records())
    _jsonl(home / MUSE_CHILD_REL, muse_subagent_records())
    _jsonl(
        home / MUSE_HISTORY_REL,
        ["fix the failing test", {"project": "/srv/proj", "session": MUSE_SESSION}],
    )
    # Noise the parser must not want: the automated approval reviewer's log
    # (same envelope, synthetic clock) and the settings file.
    review = _muse_run(
        "7a7a7a7a-0000-4000-8000-000000000001",
        1,
        1780531500000000,
        "7a7a7a7a-0000-4000-8000-000000000002",
        {"kind": "started", "prompt": '{"approval_review_context": {}}'},
        1,
    )
    review["stream"]["kind"] = "approval.review.runtime"
    _jsonl(home / MUSE_DIR / "approval-review/7a7a7a7a-0000-4000-8000-000000000001.jsonl", [review])
    (home / ".config/muse").mkdir(parents=True, exist_ok=True)
    (home / ".config/muse/settings.json").write_text("{}", encoding="utf-8")
    # Trusted projects (research/muse-code.md, Per-user storage).
    _json_file(
        home / MUSE_TRUST_REL,
        {
            "schema_version": 1,
            "projects": {
                "/srv/proj": {"decision": "trusted"},
                "/srv/other": {"decision": "trusted"},
            },
        },
    )
