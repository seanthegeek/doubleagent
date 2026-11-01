"""Synthetic pi state for the analyzer tests.

The records follow the Record schema and Other stores of research/pi.md, from
earendil-works/pi at 2c8d1fe (coding-agent 1.4.0). One session file is
little-coder's, written into pi's directory as its Wrapper attribution
subsection says.

Paths: `/srv/untrusted`
Lineage: little-coder
"""

from __future__ import annotations

import base64
import io
import json
import os
import zipfile
from pathlib import Path

from .common import BAD_LINE, _json, _jsonl, write_bad_line
from .little_coder import LC_FILE, little_coder_session_records

# ---- pi, little-coder, letta (pi session format) --------------------------------

PI_SESSION = "0199a1b2-7c3d-7e4f-8a5b-6c7d8e9f0a1b"
PI_FORK = "0199a1b2-7c3d-7e4f-8a5b-6c7d8e9f0a2c"
PI_DIR = ".pi/agent/sessions/--srv-proj--/"
PI_FILE = "2026-10-01T09-00-00-000Z_%s.jsonl" % PI_SESSION
PI_FORK_FILE = "2026-10-01T09-30-00-000Z_%s.jsonl" % PI_FORK


PI_EXPERIMENTAL = "exp-session-1"
PI_TRUST = ".pi/agent/trust.json"
PI_TRUST_MTIME = 1790852000  # 1 October 10:53:20 UTC


def pi_session_records(cwd="/srv/proj"):
    """Return a session's header and entries: two model changes, a tool call and a `!command`.

    Source: research/pi.md#entries; source 2c8d1fe
    """
    return [
        {
            "type": "session",
            "version": 3,
            "id": PI_SESSION,
            "timestamp": "2026-10-01T09:00:00.000Z",
            "cwd": cwd,
        },
        {
            "type": "model_change",
            "id": "1a2b3c4d",
            "parentId": None,
            "timestamp": "2026-10-01T09:00:00.100Z",
            "provider": "anthropic",
            "modelId": "claude-sonnet-4-5",
        },
        {
            "type": "message",
            "id": "2b3c4d5e",
            "parentId": "1a2b3c4d",
            "timestamp": "2026-10-01T09:00:01.000Z",
            "message": {
                "role": "user",
                "content": "fix the failing test",
                "timestamp": 1790845201000,
            },
        },
        {
            "type": "message",
            "id": "3c4d5e6f",
            "parentId": "2b3c4d5e",
            "timestamp": "2026-10-01T09:00:04.000Z",
            "message": {
                "role": "assistant",
                "content": [
                    {"type": "thinking", "thinking": "run the tests first"},
                    {
                        "type": "toolCall",
                        "id": "toolu_01",
                        "name": "bash",
                        "arguments": {"command": "npm test"},
                    },
                ],
                "api": "anthropic-messages",
                "provider": "anthropic",
                "model": "claude-sonnet-4-5",
                "usage": {
                    "input": 900,
                    "output": 40,
                    "cacheRead": 0,
                    "cacheWrite": 0,
                    "totalTokens": 940,
                    "cost": {"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0, "total": 0},
                },
                "stopReason": "toolUse",
                "timestamp": 1790845204000,
            },
        },
        {
            "type": "message",
            "id": "4d5e6f70",
            "parentId": "3c4d5e6f",
            "timestamp": "2026-10-01T09:00:07.000Z",
            "message": {
                "role": "toolResult",
                "toolCallId": "toolu_01",
                "toolName": "bash",
                "content": [{"type": "text", "text": "1 failing"}],
                "isError": False,
                "timestamp": 1790845207000,
            },
        },
        {
            "type": "message",
            "id": "5e6f7081",
            "parentId": "4d5e6f70",
            "timestamp": "2026-10-01T09:00:09.000Z",
            "message": {
                "role": "bashExecution",
                "command": "git status",
                "output": "M src/a.ts",
                "exitCode": 0,
                "cancelled": False,
                "truncated": False,
                "timestamp": 1790845209000,
            },
        },
        {
            "type": "label",
            "id": "70819203",
            "parentId": "5e6f7081",
            "timestamp": "2026-10-01T09:00:09.500Z",
            "targetId": "2b3c4d5e",
            "label": "start",
        },
        {
            "type": "session_info",
            "id": "6f708192",
            "parentId": "5e6f7081",
            "timestamp": "2026-10-01T09:00:10.000Z",
            "name": "test fix",
        },
        {
            "type": "model_change",
            "id": "8192a3b5",
            "parentId": "6f708192",
            "timestamp": "2026-10-01T09:00:11.000Z",
            "provider": "openai",
            "modelId": "gpt-5",
        },
    ]


def pi_fork_records(parent_path):
    """Return a fork of `pi_session_records` that copies its first three entries verbatim.

    Source: research/pi.md#joins-and-timestamps; source 2c8d1fe
    """
    parent = pi_session_records()
    return [
        {
            "type": "session",
            "version": 3,
            "id": PI_FORK,
            "timestamp": "2026-10-01T09:30:00.000Z",
            "cwd": "/srv/proj",
            "parentSession": parent_path,
        },
        *parent[1:4],
        {
            "type": "message",
            "id": "8192a3b4",
            "parentId": "3c4d5e6f",
            "timestamp": "2026-10-01T09:30:05.000Z",
            "message": {
                "role": "assistant",
                "content": [{"type": "text", "text": "Trying another way."}],
                "provider": "anthropic",
                "model": "claude-sonnet-4-5",
                "stopReason": "stop",
                "timestamp": 1790847005000,
            },
        },
    ]


def build_pi(home: Path) -> None:
    """Write pi's session files, trust and settings, debug log, crash log and noise file.

    Source: research/pi.md#per-user-storage; source 2c8d1fe
    """
    _jsonl(home / PI_DIR / PI_FILE, pi_session_records())
    _jsonl(home / PI_DIR / PI_FORK_FILE, pi_fork_records("/home/alice/" + PI_DIR + PI_FILE))
    _jsonl(home / PI_DIR / LC_FILE, little_coder_session_records())
    _json(
        home / ".pi/agent/experimental/sessions" / PI_EXPERIMENTAL / "meta.json",
        {"createdAt": 1790856000000, "cwd": "/srv/proj"},
    )
    # A file the parser must not want: the MCP server log.
    (home / PI_MCP_LOG).write_text("[2026-10-01T09:00:02.000Z] [fs] ready\n", encoding="utf-8")
    _json(
        home / PI_SETTINGS,
        {"defaultProvider": "anthropic", "defaultProjectTrust": "always"},
    )
    os.utime(home / PI_SETTINGS, (PI_TRUST_MTIME, PI_TRUST_MTIME))
    _json(home / PI_TRUST, {"/srv/proj": True, "/srv/untrusted": False})
    os.utime(home / PI_TRUST, (PI_TRUST_MTIME, PI_TRUST_MTIME))
    _jsonl(home / PI_DIR / PI_BUG_FILE, pi_bug_report_records())
    (home / PI_DEBUG_LOG).write_text(pi_debug_log_text(), encoding="utf-8")
    _json(home / PI_CRASHES, pi_crash_records())
    _jsonl(home / PI_DIR / PI_KINDS_FILE, pi_entry_kinds_records())
    _jsonl(home / PI_V1_FILE, pi_v1_records())
    (home / PI_TUI_CRASH).write_text(pi_tui_crash_text(), encoding="utf-8")


# Secondary and past-use sources beside the sessions (research/pi.md, Debug log and crash log): the `/debug` snapshot of the
# open session's messages and the recent-crash log.
PI_SETTINGS = ".pi/agent/settings.json"
PI_DEBUG_LOG = ".pi/agent/pi-debug.log"
PI_CRASHES = ".pi/agent/crashes.json"
PI_DELETED_PROMPT = "a prompt whose session file was deleted"
PI_MCP_LOG = ".pi/agent/mcp.log"


def pi_debug_log_text():
    """Return a `/debug` snapshot: three of the session's messages, a deleted prompt, two summaries.

    Source: research/pi.md#debug-log-and-crash-log; source 2c8d1fe
    """
    messages = [
        # The PI_SESSION turn: the user prompt, the assistant's call (whose
        # own timestamp, the stream start, is before the entry's), its result.
        {"role": "user", "content": "fix the failing test", "timestamp": 1790845201000},
        {
            "role": "assistant",
            "content": [
                {
                    "type": "toolCall",
                    "id": "toolu_01",
                    "name": "bash",
                    "arguments": {"command": "npm test"},
                }
            ],
            "provider": "anthropic",
            "model": "claude-sonnet-4-5",
            "stopReason": "toolUse",
            "timestamp": 1790845202000,
        },
        {
            "role": "toolResult",
            "toolCallId": "toolu_01",
            "toolName": "bash",
            "content": [{"type": "text", "text": "1 failing"}],
            "isError": False,
            "timestamp": 1790845207000,
        },
        # A prompt no collected session holds.
        {"role": "user", "content": PI_DELETED_PROMPT, "timestamp": 1790845300000},
        # The roles pi builds in memory from summary entries.
        {
            "role": "compactionSummary",
            "summary": "the turns before",
            "tokensBefore": 40210,
            "timestamp": 1790845310000,
        },
        {
            "role": "branchSummary",
            "summary": "the other branch",
            "fromId": "3c4d5e6f",
            "timestamp": 1790845320000,
        },
    ]
    lines = [
        "Debug output at 2026-10-01T09:02:00.000Z",
        "Terminal: 120x40",
        "Total lines: 2",
        "",
        "=== All rendered lines with visible widths ===",
        '[0] (w=20) "fix the failing test"',
        '[1] (w=9) "1 failing"',
        "",
        "=== Agent messages (JSONL) ===",
        *(json.dumps(message) for message in messages),
        "",
    ]
    return "\n".join(lines)


def pi_crash_records():
    """Return `crashes.json` with one announced crash naming the session file.

    Source: research/pi.md#debug-log-and-crash-log; source 2c8d1fe
    """
    return [
        {
            "timestamp": "2026-10-01T09:05:00.000Z",
            "version": "1.3.0",
            "kind": "uncaught_exception",
            "message": "Cannot read properties of undefined",
            "stack": "TypeError: Cannot read properties of undefined\n    at run (ext.ts:3:1)",
            "sessionFile": "/home/alice/" + PI_DIR + PI_FILE,
            "cwd": "/srv/proj",
            "notified": True,
        }
    ]


# Session sync: a session reported with `/bug` twice, once
# uploaded and once exported to a zip (research/pi.md, Record schema).
PI_BUG_SESSION = "0199a1b2-7c3d-7e4f-8a5b-6c7d8e9f0b3d"
PI_BUG_FILE = "2026-10-01T13-00-00-000Z_%s.jsonl" % PI_BUG_SESSION
PI_BUG_REPORT = "0199a1b2-9999-7000-8000-00000000b001"
PI_BUG_ZIP = "0199a1b2-9999-7000-8000-00000000b002"


def pi_bug_report_records(cwd="/srv/proj"):
    """Return a session reported with `/bug` twice, uploaded then exported to a zip.

    Source: research/pi.md#bug-report-entry; source 2c8d1fe
    """

    def report(entry_id, parent, timestamp, data):
        return {
            "type": "custom",
            "id": entry_id,
            "parentId": parent,
            "timestamp": timestamp,
            "customType": "pi.bug-report",
            "data": data,
        }

    return [
        {
            "type": "session",
            "version": 3,
            "id": PI_BUG_SESSION,
            "timestamp": "2026-10-01T13:00:00.000Z",
            "cwd": cwd,
        },
        {
            "type": "message",
            "id": "b1000001",
            "parentId": None,
            "timestamp": "2026-10-01T13:00:01.000Z",
            "message": {"role": "user", "content": "the build hangs", "timestamp": 1790859601000},
        },
        report(
            "b1000002",
            "b1000001",
            "2026-10-01T13:00:30.000Z",
            {
                "id": PI_BUG_REPORT,
                "createdAt": "2026-10-01T13:00:29.000Z",
                "hint": "build hangs on step 3",
                "sessionIncluded": True,
                "summaryIncluded": False,
                "delivery": "upload",
            },
        ),
        report(
            "b1000003",
            "b1000002",
            "2026-10-01T13:01:00.000Z",
            {
                "id": PI_BUG_ZIP,
                "createdAt": "2026-10-01T13:00:59.000Z",
                "hint": None,
                "sessionIncluded": False,
                "summaryIncluded": False,
                "delivery": "zip",
                "path": "/srv/proj/pi-bug-report-%s.zip" % PI_BUG_ZIP,
            },
        ),
        {
            "type": "custom",
            "id": "b1000004",
            "parentId": "b1000003",
            "timestamp": "2026-10-01T13:01:30.000Z",
            "customType": "some-extension",
            "data": {},
        },
    ]


# Every other entry type and session-file message role the parser gives a row,
# the skipped `usage` entry, both stop-reason rows, redacted thinking and a
# failed `!command` (research/pi.md, Entries and Message roles). The in-memory
# summary roles are in the debug log.
PI_KINDS_SESSION = "0199a1b2-7c3d-7e4f-8a5b-6c7d8e9f0c4e"
PI_KINDS_FILE = "2026-10-01T14-00-00-000Z_%s.jsonl" % PI_KINDS_SESSION


def pi_entry_kinds_records(cwd="/srv/proj"):
    """Return a session with each remaining entry type and session-file role.

    Source: research/pi.md#message-roles; source 2c8d1fe
    """

    def entry(entry_id, parent, second, **fields):
        return {
            "type": fields.pop("type", "message"),
            "id": entry_id,
            "parentId": parent,
            "timestamp": "2026-10-01T14:00:%02d.000Z" % second,
            **fields,
        }

    def message(second, **fields):
        return dict(fields, timestamp=1790863200000 + second * 1000)

    return [
        {
            "type": "session",
            "version": 3,
            "id": PI_KINDS_SESSION,
            "timestamp": "2026-10-01T14:00:00.000Z",
            "cwd": cwd,
        },
        entry(
            "c1000001",
            None,
            1,
            message=message(1, role="system", content="You are pi."),
        ),
        entry("c1000002", "c1000001", 2, type="thinking_level_change", thinkingLevel="high"),
        entry(
            "c1000003",
            "c1000002",
            3,
            message=message(
                3,
                role="assistant",
                content=[
                    {
                        "type": "toolCall",
                        "id": "call_ps1",
                        "name": "powershell",
                        "arguments": {"command": "Get-ChildItem", "timeout": 30},
                    }
                ],
                api="openai-responses",
                provider="openai",
                model="gpt-5.1-codex",
                stopReason="deferred",
                durationMs=900,
            ),
        ),
        entry(
            "c1000004",
            "c1000003",
            4,
            message=message(
                4,
                role="toolResult",
                toolCallId="call_ps1",
                toolName="powershell",
                content=[{"type": "text", "text": "Tool execution was blocked"}],
                isError=True,
                durationMs=2,
            ),
        ),
        entry(
            "c1000005",
            "c1000004",
            5,
            message=message(
                5,
                role="assistant",
                content=[],
                api="openai-responses",
                provider="openai",
                model="gpt-5.1-codex",
                stopReason="error",
                errorMessage="429 rate limited",
            ),
        ),
        entry(
            "c1000006",
            "c1000005",
            6,
            type="compaction",
            summary="earlier turns",
            firstKeptEntryId="c1000004",
            tokensBefore=51234,
        ),
        entry(
            "c1000007",
            "c1000006",
            7,
            type="branch_summary",
            fromId="c1000003",
            summary="the abandoned branch",
        ),
        entry(
            "c1000008",
            "c1000007",
            8,
            type="custom_message",
            customType="notes",
            content="remember the deadline",
            display=True,
        ),
        entry(
            "c1000009",
            "c1000008",
            9,
            type="context_edit",
            targetId="c1000004",
            replacement={"content": "blocked"},
        ),
        entry(
            "c100000a", "c1000009", 10, type="context_edit", targetId="c1000005", replacement=None
        ),
        entry(
            "c100000b",
            "c100000a",
            11,
            message=message(
                11, role="custom", customType="hook", content="hook text", display=False
            ),
        ),
        entry(
            "c100000c",
            "c100000b",
            12,
            type="usage",
            kind="cache_warm",
            provider="openai",
            model="gpt-5.1-codex",
            usage={"input": 10, "output": 0, "totalTokens": 10},
        ),
        entry(
            "c100000d",
            "c100000c",
            13,
            message=message(
                13,
                role="assistant",
                content=[
                    {
                        "type": "thinking",
                        "thinking": "",
                        "thinkingSignature": "c2lnbmF0dXJl",
                        "redacted": True,
                    }
                ],
                api="openai-responses",
                provider="openai",
                model="gpt-5.1-codex",
                stopReason="aborted",
            ),
        ),
        entry(
            "c100000e",
            "c100000d",
            14,
            message=message(
                14,
                role="bashExecution",
                command="make test",
                output="partial",
                cancelled=True,
                truncated=True,
                fullOutputPath="/tmp/pi-bash-0123456789abcdef.log",
            ),
        ),
    ]


# A version 1 file in the legacy location, which pi has not started since to
# move: no `version`, no entry ids (research/pi.md, Format versions).
PI_V1_SESSION = "0199a1b2-7c3d-7e4f-8a5b-6c7d8e9f0d5f"
PI_V1_FILE = ".pi/agent/2026-09-01T08-00-00-000Z_%s.jsonl" % PI_V1_SESSION


def pi_v1_records(cwd="/srv/proj"):
    """Return a version 1 session: a header with no `version` and one prompt without ids.

    Source: research/pi.md#format-versions; source 2c8d1fe
    """
    return [
        {
            "type": "session",
            "id": PI_V1_SESSION,
            "timestamp": "2026-09-01T08:00:00.000Z",
            "cwd": cwd,
        },
        {
            "type": "message",
            "timestamp": "2026-09-01T08:00:05.000Z",
            "message": {"role": "user", "content": "an old prompt", "timestamp": 1788249605000},
        },
    ]


# Copies of sessions in a project directory (research/pi.md, Project-local
# files): an HTML export of PI_SESSION, an HTML and a JSONL export of a
# session whose file was deleted, and a bug report zip of PI_BUG_SESSION.
PI_EXPORT_DIR = "dev/pi-webapp"
PI_GONE_SESSION = "0199a1b2-7c3d-7e4f-8a5b-6c7d8e9f0e60"
PI_GONE_PROMPT = "a prompt only the exports hold"
PI_HTML = "pi-session-%s.html" % PI_FILE[: -len(".jsonl")]
PI_GONE_HTML = "pi-session-2026-10-02T08-00-00-000Z_%s.html" % PI_GONE_SESSION
PI_GONE_JSONL = "session-2026-10-02T08-10-00-000Z.jsonl"
PI_BUG_ZIP_FILE = "pi-bug-report-%s.zip" % PI_BUG_ZIP
PI_TUI_CRASH = ".pi/agent/pi-tui-crash.log"


def pi_gone_records(cwd="/srv/proj"):
    """Return a deleted session's header and entries: a prompt, two replies on two branches.

    Source: research/pi.md#entries; source 2c8d1fe
    """
    return [
        {
            "type": "session",
            "version": 3,
            "id": PI_GONE_SESSION,
            "timestamp": "2026-10-02T08:00:00.000Z",
            "cwd": cwd,
        },
        {
            "type": "message",
            "id": "e1000001",
            "parentId": None,
            "timestamp": "2026-10-02T08:00:02.000Z",
            "message": {"role": "user", "content": PI_GONE_PROMPT, "timestamp": 1790928002000},
        },
        # The abandoned branch: a reply the user went back from.
        {
            "type": "message",
            "id": "e1000002",
            "parentId": "e1000001",
            "timestamp": "2026-10-02T08:00:05.000Z",
            "message": {
                "role": "assistant",
                "content": [{"type": "text", "text": "the first answer"}],
                "provider": "openai",
                "model": "gpt-5",
                "stopReason": "stop",
                "timestamp": 1790928003000,
            },
        },
        {
            "type": "message",
            "id": "e1000003",
            "parentId": "e1000001",
            "timestamp": "2026-10-02T08:01:00.000Z",
            "message": {
                "role": "assistant",
                "content": [{"type": "text", "text": "the second answer"}],
                "provider": "openai",
                "model": "gpt-5",
                "stopReason": "stop",
                "timestamp": 1790928058000,
            },
        },
    ]


def pi_html_export_text(records, leaf_id):
    """Return an HTML export page embedding a session's header and every entry as base64 JSON.

    Source: research/pi.md#html-export; source 2c8d1fe
    """
    data = {
        "header": records[0],
        "entries": records[1:],
        "leafId": leaf_id,
        "systemPrompt": "You are pi.",
        "tools": [{"name": "bash", "description": "Run a command", "parameters": {}}],
    }
    payload = base64.b64encode(json.dumps(data).encode("utf-8")).decode("ascii")
    return (
        "<!DOCTYPE html>\n<html>\n<head><style>body { margin: 0; }</style></head>\n<body>\n"
        '  <script id="session-data" type="application/json">%s</script>\n'
        "  <script>/* viewer */</script>\n</body>\n</html>\n" % payload
    )


def pi_branch_export_records(records, branch_ids, exported_at) -> list[dict]:
    """Return a JSONL export of one branch: a new header at the export time, parents re-chained.

    Source: research/pi.md#jsonl-export; source 2c8d1fe
    """
    header = dict(records[0], timestamp=exported_at)
    header.pop("parentSession", None)
    out: list[dict] = [header]
    parent = None
    for record in records[1:]:
        if record.get("id") in branch_ids:
            out.append(dict(record, parentId=parent))
            parent = record["id"]
    return out


def pi_bug_zip_members():
    """Return a bug report zip's members by name: report, diagnostics, summary and session.

    Source: research/pi.md#bug-report-archive; source 2c8d1fe
    """
    branch = pi_branch_export_records(
        pi_bug_report_records(), ("b1000001",), "2026-10-01T13:00:58.000Z"
    )
    branch.append(
        {
            "type": "custom",
            "customType": "pi.share",
            "id": "5a1e0c2f",
            "parentId": "b1000001",
            "timestamp": "2026-10-01T13:00:58.000Z",
            "data": {"systemPrompt": "You are pi.", "tools": []},
        }
    )
    report = {
        "schemaVersion": 1,
        "id": PI_BUG_ZIP,
        "createdAt": "2026-10-01T13:00:59.000Z",
        "hint": None,
        "environment": {"version": "1.4.0", "platform": "linux", "arch": "x64"},
        "session": {
            "id": PI_BUG_SESSION,
            "included": True,
            "summaryIncluded": True,
            "messageCount": 1,
            "cwd": "/srv/proj",
        },
        "model": {"provider": "openai", "id": "gpt-5"},
        "settings": {"global": {"httpProxy": "<redacted>"}, "project": {}},
    }
    diagnostics = {
        "schemaVersion": 1,
        "sessionId": PI_BUG_SESSION,
        "entryCount": 3,
        "assistantMessageCount": 1,
        "assistant": [
            {
                "entryId": "b10000aa",
                "timestamp": "2026-10-01T13:00:20.000Z",
                "provider": "openai",
                "model": "gpt-5",
                "stopReason": "error",
                "errorMessage": "socket hang up",
                "diagnostics": [],
            }
        ],
        "crashes": [
            {
                "timestamp": "2026-10-01T12:58:00.000Z",
                "version": "1.4.0",
                "kind": "fatal_error",
                "message": "EPIPE: broken pipe, write",
                "stack": None,
                "sessionFile": "/home/alice/" + PI_DIR + PI_BUG_FILE,
                "cwd": "/srv/proj",
            }
        ],
    }
    session = "".join(json.dumps(record) + "\n" for record in branch) + BAD_LINE
    return {
        "report.json": json.dumps(report, indent=2) + "\n",
        "diagnostics.json": json.dumps(diagnostics, indent=2) + "\n",
        "session.jsonl": session,
        "summary.md": "## What the user was doing\nRunning the build.\n",
    }


def pi_zip_bytes(members):
    """Return a zip archive of `members`, deflated as pi's own zip writer stores them.

    Source: research/pi.md#bug-report-archive; source 2c8d1fe
    """
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, text in members.items():
            archive.writestr(name, text)
    return buffer.getvalue()


def pi_tui_crash_text():
    """Return the terminal UI crash log: the time, the widths and the rendered screen lines.

    Source: research/pi.md#terminal-ui-crash-log; source 2c8d1fe
    """
    lines = [
        "Crash at 2026-10-01T09:06:30.000Z",
        "Terminal width: 40",
        "Line 1 visible width: 52",
        "",
        "=== All rendered lines ===",
        "[0] (w=20) fix the failing test",
        "[1] (w=52) a status line from an extension that ran past the edge",
        "",
    ]
    return "\n".join(lines)


def build_pi_exports(project: Path) -> None:
    """Write the HTML, JSONL and bug report exports pi leaves in a working directory.

    Source: research/pi.md#project-local-files; source 2c8d1fe
    """
    project.mkdir(parents=True, exist_ok=True)
    (project / PI_HTML).write_text(
        pi_html_export_text(pi_session_records(), "8192a3b5"), encoding="utf-8"
    )
    gone = pi_gone_records()
    (project / PI_GONE_HTML).write_text(pi_html_export_text(gone, "e1000003"), encoding="utf-8")
    _jsonl(
        project / PI_GONE_JSONL,
        pi_branch_export_records(gone, ("e1000001", "e1000003"), "2026-10-02T08:10:00.000Z"),
    )
    write_bad_line(project / PI_GONE_JSONL)
    (project / PI_BUG_ZIP_FILE).write_bytes(pi_zip_bytes(pi_bug_zip_members()))
