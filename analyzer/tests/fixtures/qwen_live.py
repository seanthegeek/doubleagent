"""Synthetic Qwen Live state for the analyzer tests.

The call log, memory library and its `meta.json` follow the Record schema of
research/qwen-live.md, from QwenLM/qwen-code at 835b080 (`qwen-live` 0.1.0). No parser reads
them: the agent is detected but not parsed.
"""

from __future__ import annotations

from pathlib import Path

from .common import _json_file, _jsonl, _wal_database

QWEN_LIVE_RUN = "live-1793520000000-5c4d3e2f"
QWEN_LIVE_CALL = "call-0b1c"
QWEN_LIVE_LOG_REL = ".qwen-live/sessions/%s.jsonl" % QWEN_LIVE_RUN
QWEN_LIVE_LIBRARY_REL = ".qwen-live/memories/default/dialogue.db"

# The two memory tables a call's dialogue lands in, with the columns of
# packages/qwen-live/src/memory/schema.ts at 835b080 (research/qwen-live.md, Memory library).
QWEN_LIVE_SCHEMA = """
CREATE TABLE library_sessions (session_id TEXT PRIMARY KEY, session_name TEXT,
  first_seen_at TEXT NOT NULL, last_seen_at TEXT NOT NULL, n_turns INTEGER NOT NULL DEFAULT 0,
  n_segments INTEGER NOT NULL DEFAULT 0, active INTEGER NOT NULL DEFAULT 1, deactivated_at TEXT,
  deactivate_reason TEXT);
CREATE TABLE turns (id INTEGER PRIMARY KEY, session_id TEXT NOT NULL, turn_idx INTEGER NOT NULL,
  user_text TEXT NOT NULL, user_ts TEXT NOT NULL, user_epoch INTEGER NOT NULL,
  asst_text TEXT NOT NULL, asst_ts TEXT NOT NULL, asst_epoch INTEGER NOT NULL,
  interrupted INTEGER NOT NULL DEFAULT 0, seg_id INTEGER, UNIQUE(session_id, turn_idx));
"""


def qwen_live_call_records(call_id: str = QWEN_LIVE_CALL):
    """Return a call log's lines: a call that hands one task to a backend and ends.

    Source: research/qwen-live.md#call-log; source 835b080
    """
    start = 1793520000000
    payloads = [
        (
            "session.start",
            {
                "callId": call_id,
                "epoch": 2,
                "mode": "new",
                "backends": "gemini",
                "model": "qwen3.5-omni-flash-realtime",
                "voice": "Ethan",
            },
        ),
        ("session.start", {"phase": "realtime_ready"}),
        ("transcript.user", {"text": "have gemini list the open issues"}),
        ("tool.call", {"name": "handoff", "callId": "fc_77", "args": '{"task":"list issues"}'}),
        ("tool.result", {"name": "handoff", "callId": "fc_77", "ok": True, "receipt": "{}"}),
        ("transcript.assistant", {"text": "Gemini is on it.", "direct": True}),
        ("session.end", {"callId": call_id}),
    ]
    return [
        {"ts": start + 1500 * index, "seq": index + 1, "type": kind, "payload": payload}
        for index, (kind, payload) in enumerate(payloads)
    ]


def build_qwen_live(home: Path) -> None:
    """Write Qwen Live's call log and a default memory library holding the same call's turn.

    Source: research/qwen-live.md#memory-library; source 835b080
    """
    _jsonl(home / QWEN_LIVE_LOG_REL, qwen_live_call_records())
    _json_file(
        home / ".qwen-live/memories/default/meta.json",
        {
            "version": 1,
            "id": "default",
            "name": "Home",
            "created_at": "2026-11-01T08:00:00.000Z",
            "updated_at": "2026-11-01T08:00:09.000Z",
        },
    )
    _wal_database(
        home / QWEN_LIVE_LIBRARY_REL,
        QWEN_LIVE_SCHEMA,
        [
            "INSERT INTO library_sessions (session_id, first_seen_at, last_seen_at, n_turns) "
            "VALUES ('%s', '2026-11-01T08:00:00.000Z', '2026-11-01T08:00:09.000Z', 1)"
            % QWEN_LIVE_CALL,
            "INSERT INTO turns (session_id, turn_idx, user_text, user_ts, user_epoch, asst_text, "
            "asst_ts, asst_epoch) VALUES ('%s', 0, 'have gemini list the open issues', "
            "'2026-11-01 09:00:03', 1793520003, 'Gemini is on it.', '2026-11-01 09:00:07', "
            "1793520007)" % QWEN_LIVE_CALL,
        ],
    )
