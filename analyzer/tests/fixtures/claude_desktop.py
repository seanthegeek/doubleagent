"""Synthetic Claude Desktop state for the analyzer tests."""

from __future__ import annotations

import json
import os
from pathlib import Path

from .claude_code import claude_session_records
from .common import _jsonl

# -- Claude Desktop (app bundle 2.31226.1) ------------------------------------------
CD_BASE = "Library/Application Support/Claude"
CD_ACCT = "aaaaaaaa-0000-4000-8000-000000000001"
CD_ORG = "bbbbbbbb-0000-4000-8000-000000000002"
CD_COWORK_ORG = "%s/local-agent-mode-sessions/%s/%s" % (CD_BASE, CD_ACCT, CD_ORG)
CD_CODE_ORG = "%s/claude-code-sessions/%s/%s" % (CD_BASE, CD_ACCT, CD_ORG)
CD_SESSION = "local_c0c0c0c0-1111-4222-8333-444444444444"
CD_CLI_SESSION = "d1d1d1d1-5555-4666-8777-888888888888"
CD_GUEST_CWD = "/sessions/quiet-river-1234"
CD_RECORD_REL = "%s/%s.json" % (CD_COWORK_ORG, CD_SESSION)
CD_TRANSCRIPT_REL = "%s/%s/.claude/projects/session/%s.jsonl" % (
    CD_COWORK_ORG,
    CD_SESSION,
    CD_CLI_SESSION,
)
CD_AUDIT_REL = "%s/%s/audit.jsonl" % (CD_COWORK_ORG, CD_SESSION)
# A session dir in the short (eight hex) form holding only an audit log.
CD_FALLBACK_DIR = "f3f3f3f3"
CD_FALLBACK_CLI = "d2d2d2d2-5555-4666-8777-888888888888"
CD_FALLBACK_REL = "%s/%s/audit.jsonl" % (CD_COWORK_ORG, CD_FALLBACK_DIR)
CD_CODE_SESSION = "local_e2e2e2e2-9999-4aaa-8bbb-cccccccccccc"
# Its own CLI id: a Code-tab record that joined the claude-code fixture
# session would put two agents under one session id in sessions.jsonl.
CD_CODE_CLI = "11111111-2222-4333-8444-555555555555"
CD_CODE_RECORD_REL = "%s/%s.json" % (CD_CODE_ORG, CD_CODE_SESSION)
# A Code-tab record with the persisted Remote Control state set (bundle
# serializer keys, research/claude-desktop.md, Record schema; fixture only).
CD_CODE_REMOTE_CONTROL_SESSION = "local_e3e3e3e3-9999-4aaa-8bbb-cccccccccccc"
CD_CODE_REMOTE_CONTROL_CLI = "e3e3e3e3-5555-4666-8777-888888888888"
CD_CODE_REMOTE_CONTROL_RECORD_REL = "%s/%s.json" % (CD_CODE_ORG, CD_CODE_REMOTE_CONTROL_SESSION)
CD_SCHED_REL = "%s/scheduled-tasks.json" % CD_COWORK_ORG
CD_WORKTREES_REL = "%s/git-worktrees.json" % CD_BASE
CD_SPACES_REL = "%s/remote-session-spaces.json" % CD_COWORK_ORG
CD_REMOTE_SESSION = "0123456789abcdef0123456789abcdef"
CD_SPACES_MTIME = 1790848800  # 1 October 10:00:00 UTC


def claude_desktop_record():
    return {
        "sessionId": CD_SESSION,
        "processName": "quiet-river-1234",
        "cliSessionId": CD_CLI_SESSION,
        "cwd": CD_GUEST_CWD,
        "userSelectedFolders": ["/srv/proj"],
        "createdAt": 1790762400000,
        "lastActivityAt": 1790762460000,
        "model": "claude-fable-5-1",
        "permissionMode": "default",
        "isArchived": False,
        "title": "Summarise the test failures",
        "vmProcessName": "quiet-river-1234",
        "initialMessage": "why does the test fail?",
        "sessionType": "agent",
        "emailAddress": "user@example.invalid",
        # Cowork grants: "arrays and objects" in the bundle, the
        # type of each key not determined; these values are fixture only.
        "approvedToolNames": ["Bash", "WebFetch"],
        "webFetchAllowedUrls": ["https://example.invalid/docs"],
        "egressAllowedDomains": ["example.invalid"],
        "userApprovedFileAccessPaths": ["/srv/proj/data"],
        "enabledMcpTools": {"github:create_issue": True},
        "fileDeleteApprovedMounts": [],
    }


def claude_desktop_transcript_records():
    """The claude-code fixture as the Cowork VM's CLI writes it.

    Guest cwd, `entrypoint` local-agent and the record's `cliSessionId`.
    """
    out = []
    for record in claude_session_records(cwd=CD_GUEST_CWD):
        record = dict(record)
        if "sessionId" in record:
            record["sessionId"] = CD_CLI_SESSION
        if "entrypoint" in record:
            record["entrypoint"] = "local-agent"
        out.append(record)
    return out


def _cd_hmac(number: int) -> str:
    return "%064x" % number


def claude_desktop_audit_records():
    """The research document's Parser plan, verbatim (HMAC values and tool ids are placeholders).

    Line 1's prompt has no transcript twin; line 8's and the `init` model repeat the transcript's
    rows. The `npm test` call is not the transcript's, so its id is not the transcript's `toolu_01`:
    tool rows pair on their id.
    """
    session_id = CD_CLI_SESSION
    return [
        {
            "type": "user",
            "uuid": "00000000-0000-4000-8000-0000000000a1",
            "session_id": session_id,
            "parent_tool_use_id": None,
            "client_platform": "desktop_app",
            "timestamp": "2026-10-01T10:00:00.000Z",
            "message": {
                "role": "user",
                "content": [{"type": "text", "text": "why does the test fail?"}],
            },
            "_audit_timestamp": "2026-10-01T10:00:00.004Z",
            "_audit_hmac": _cd_hmac(1),
        },
        {
            "type": "system",
            "subtype": "init",
            "session_id": session_id,
            "cwd": CD_GUEST_CWD,
            "model": "claude-fable-5-1",
            "_audit_timestamp": "2026-10-01T10:00:01.000Z",
            "_audit_hmac": _cd_hmac(2),
        },
        {
            "type": "assistant",
            "uuid": "00000000-0000-4000-8000-0000000000a2",
            "session_id": session_id,
            "parent_tool_use_id": None,
            "message": {
                "model": "claude-fable-5-1",
                "role": "assistant",
                "content": [
                    {
                        "type": "tool_use",
                        "id": "toolu_a1",
                        "name": "Bash",
                        "input": {"command": "npm test"},
                    }
                ],
            },
            "_audit_timestamp": "2026-10-01T10:00:03.000Z",
            "_audit_hmac": _cd_hmac(3),
        },
        {
            "type": "system",
            "subtype": "permission_request",
            "uuid": "00000000-0000-4000-8000-0000000000a3",
            "session_id": session_id,
            "tool_name": "Bash",
            "tool_input": {"command": "npm test"},
            "_audit_timestamp": "2026-10-01T10:00:03.100Z",
            "_audit_hmac": _cd_hmac(4),
        },
        {
            "type": "system",
            "subtype": "permission_response",
            "uuid": "00000000-0000-4000-8000-0000000000a3",
            "session_id": session_id,
            "tool_name": "Bash",
            "decision": "once",
            "granted": True,
            "_audit_timestamp": "2026-10-01T10:00:05.000Z",
            "_audit_hmac": _cd_hmac(5),
        },
        {
            "type": "user",
            "uuid": "00000000-0000-4000-8000-0000000000a4",
            "session_id": session_id,
            "parent_tool_use_id": None,
            "message": {
                "role": "user",
                "content": [
                    {"type": "tool_result", "tool_use_id": "toolu_a1", "content": "1 failing"}
                ],
            },
            "_audit_timestamp": "2026-10-01T10:00:09.000Z",
            "_audit_hmac": _cd_hmac(6),
        },
        {
            "type": "result",
            "subtype": "success",
            "duration_ms": 9000,
            "duration_api_ms": 4000,
            "is_error": False,
            "num_turns": 2,
            "session_id": session_id,
            "uuid": "00000000-0000-4000-8000-0000000000a5",
            "_audit_timestamp": "2026-10-01T10:00:12.000Z",
            "_audit_hmac": _cd_hmac(7),
        },
        # Issue 122: a prompt the transcript also records (its last user
        # line, "thanks" at 10:00:20), so the secondary row pairs with it.
        {
            "type": "user",
            "uuid": "00000000-0000-4000-8000-0000000000a6",
            "session_id": session_id,
            "parent_tool_use_id": None,
            "client_platform": "desktop_app",
            "timestamp": "2026-10-01T10:00:20.000Z",
            "message": {"role": "user", "content": [{"type": "text", "text": "thanks"}]},
            "_audit_timestamp": "2026-10-01T10:00:20.003Z",
            "_audit_hmac": _cd_hmac(8),
        },
    ]


def claude_desktop_fallback_records():
    """An audit log whose session dir has no transcript: every line counts.

    Line numbers are what ClaudeDesktopTests.test_audit_fallback asserts.
    """
    session_id = CD_FALLBACK_CLI

    def line(number, second, **record):
        record.setdefault("session_id", session_id)
        record["_audit_timestamp"] = "2026-10-02T09:00:%s.000Z" % second
        record["_audit_hmac"] = _cd_hmac(100 + number)
        return record

    return [
        line(
            1,
            "00",
            type="user",
            uuid="00000000-0000-4000-8000-0000000000b1",
            parent_tool_use_id=None,
            client_platform="desktop_app",
            timestamp="2026-10-02T08:59:59.500Z",
            message={"role": "user", "content": [{"type": "text", "text": "list the files"}]},
        ),  # 1
        line(2, "01", type="system", subtype="init", cwd=CD_GUEST_CWD, model="claude-fable-5-1"),
        line(
            3,
            "02",
            type="user",
            uuid="00000000-0000-4000-8000-0000000000b2",
            parent_tool_use_id=None,
            client_platform="desktop_app",
            isSynthetic=True,
            message={"role": "user", "content": [{"type": "text", "text": "folder mounted"}]},
        ),  # 3
        line(
            4,
            "03",
            type="assistant",
            uuid="00000000-0000-4000-8000-0000000000b3",
            parent_tool_use_id=None,
            message={
                "model": "claude-fable-5-1",
                "role": "assistant",
                "content": [
                    {"type": "thinking", "thinking": "use ls", "signature": "x"},
                    {"type": "text", "text": "Listing."},
                    {
                        "type": "tool_use",
                        "id": "toolu_b1",
                        "name": "Bash",
                        "input": {"command": "ls"},
                    },
                ],
            },
        ),  # 4
        line(
            5,
            "04",
            type="system",
            subtype="permission_auto_approved",
            tool_name="Bash",
            source="always_allow",
        ),  # 5
        line(
            6,
            "05",
            type="user",
            uuid="00000000-0000-4000-8000-0000000000b4",
            parent_tool_use_id=None,
            message={
                "role": "user",
                "content": [
                    {"type": "tool_result", "tool_use_id": "toolu_b1", "content": "README.md"}
                ],
            },
        ),  # 6
        line(
            7,
            "06",
            type="assistant",
            uuid="00000000-0000-4000-8000-0000000000b5",
            parent_tool_use_id=None,
            message={
                "model": "claude-fable-5-1",
                "role": "assistant",
                "content": [{"type": "text", "text": "One file: README.md"}],
            },
        ),  # 7
        line(8, "07", type="prompt_suggestion", suggestion="open it"),  # 8: skipped
        line(
            9,
            "08",
            type="result",
            subtype="success",
            duration_ms=8000,
            duration_api_ms=3000,
            is_error=False,
            num_turns=1,
            uuid="00000000-0000-4000-8000-0000000000b6",
        ),  # 9
        # The subtype is in the bundle, its fields are not recorded.
        line(
            10,
            "09",
            type="system",
            subtype="model_refusal_fallback",
            note="fields not recorded; fixture only",
        ),  # 10
        line(
            11,
            "10",
            type="assistant",
            uuid="00000000-0000-4000-8000-0000000000b7",
            parent_tool_use_id=None,
            message={
                "model": "claude-opus-5-5",
                "role": "assistant",
                "content": [{"type": "text", "text": "Answered by the fallback."}],
            },
        ),  # 11
    ]


def claude_desktop_code_record():
    return {
        "sessionId": CD_CODE_SESSION,
        "cliSessionId": CD_CODE_CLI,
        "cwd": "/srv/proj",
        "originCwd": "/srv/proj",
        "branch": "main",
        "createdAt": 1790762400000,
        "lastActivityAt": 1790762520000,
        "model": "claude-fable-5-1",
        "isArchived": False,
        "title": "Fix the failing test",
        "permissionMode": "default",
        # Per-session grants: the entries' inner shape is not
        # determined (real installs showed both lists empty); fixture only.
        "effort": "high",
        "alwaysAllowedReasons": ["Bash(npm test)"],
        "sessionPermissionUpdates": [
            {
                "type": "addRules",
                "rules": [{"toolName": "Bash", "ruleContent": "npm test"}],
                "behavior": "allow",
                "destination": "session",
            }
        ],
        "chromePermissionMode": "ask",
        "chromeAllowedDomains": ["example.invalid"],
        "cuAllowedApps": ["Terminal"],
        "cuGrantFlags": {"clipboardRead": True},
        "cuFlagsGrantedAt": 1790762450000,
    }


def claude_desktop_code_remote_control_record():
    return {
        "sessionId": CD_CODE_REMOTE_CONTROL_SESSION,
        "cliSessionId": CD_CODE_REMOTE_CONTROL_CLI,
        "cwd": "/srv/proj",
        "originCwd": "/srv/proj",
        "branch": "main",
        "createdAt": 1790766000000,
        "lastActivityAt": 1790766600000,
        "model": "claude-fable-5-1",
        "isArchived": False,
        "title": "Driven from the phone",
        "permissionMode": "default",
        "remoteControlUserEnabled": True,
        "remoteControlUserRequested": True,
        "remoteControlUserToggled": True,
        "remoteControlAutoEligible": False,
        "bridgeSessionIds": ["cse_aaaa0001", "cse_aaaa0002"],
        "remoteControlSpawn": {
            "ccrSessionId": "cse_aaaa0001",
            "detachedAt": 1790766300000,
            "rcChild": True,
            "projectThreadChild": False,
        },
        "steeredByRemoteClient": True,
        "remoteControlLiveAtUpdateQuit": {
            "at": 1790766500000,
            "accountId": "aaaaaaaa-0000-4000-8000-000000000001",
        },
    }


def claude_desktop_scheduled_tasks():
    return {
        "scheduledTasks": [
            {
                "id": "daily-report",
                "cronExpression": "0 9 * * 1-5",
                "enabled": True,
                "filePath": "/home/user/Claude/Scheduled/daily-report/SKILL.md",
                "createdAt": 1790762400000,
                "cwd": "/srv/proj",
                "permissionMode": "default",
                "model": "claude-fable-5-1",
            }
        ],
        "recordedSkips": {},
        "sundayAliasBoundaryStamped": True,
        "dayFieldsOrBoundaryStamped": True,
    }


def claude_desktop_worktrees():
    return {
        "schemaVersion": 2,
        "worktrees": {
            "brave-otter": {
                "name": "brave-otter",
                "path": "/srv/proj/.claude/worktrees/brave-otter",
                "leasedBy": CD_CODE_SESSION,
                "baseRepo": "/srv/proj",
                "branch": "claude/brave-otter",
                "sourceBranch": "main",
                "createdAt": 1790762401000,
            }
        },
        "untrackedDirGc": {"cwds": {}, "roots": {}, "sightings": {}},
    }


def claude_desktop_remote_spaces():
    """The research document's Record schema.

    `entries[]` with `sessionId` (32 hex) and `folders[]`; observed shape, fixture values.
    """
    return {"entries": [{"sessionId": CD_REMOTE_SESSION, "folders": ["/srv/proj", "/srv/docs"]}]}


def build_claude_desktop(home: Path) -> None:
    cowork = home / CD_COWORK_ORG
    cowork.mkdir(parents=True, exist_ok=True)
    (home / CD_RECORD_REL).write_text(
        json.dumps(claude_desktop_record(), indent=2), encoding="utf-8"
    )
    _jsonl(home / CD_TRANSCRIPT_REL, claude_desktop_transcript_records())
    _jsonl(home / CD_AUDIT_REL, claude_desktop_audit_records())
    _jsonl(home / CD_FALLBACK_REL, claude_desktop_fallback_records())
    code = home / CD_CODE_RECORD_REL
    code.parent.mkdir(parents=True, exist_ok=True)
    code.write_text(json.dumps(claude_desktop_code_record()), encoding="utf-8")
    (home / CD_CODE_REMOTE_CONTROL_RECORD_REL).write_text(
        json.dumps(claude_desktop_code_remote_control_record()), encoding="utf-8"
    )
    (home / CD_SCHED_REL).write_text(json.dumps(claude_desktop_scheduled_tasks()), encoding="utf-8")
    (home / CD_WORKTREES_REL).write_text(json.dumps(claude_desktop_worktrees()), encoding="utf-8")
    (home / CD_SPACES_REL).write_text(json.dumps(claude_desktop_remote_spaces()), encoding="utf-8")
    os.utime(home / CD_SPACES_REL, (CD_SPACES_MTIME, CD_SPACES_MTIME))
    # Noise the parser must not want.
    (cowork / "spaces.json").write_text('{"spaces":[]}', encoding="utf-8")
    (cowork / "cowork_settings.json").write_text("{}", encoding="utf-8")
    (cowork / "rpm").mkdir(exist_ok=True)
    (cowork / "rpm/manifest.json").write_text(
        '{"lastUpdated":1790762400000,"plugins":[]}', encoding="utf-8"
    )
    (cowork / CD_SESSION / ".audit-key").write_bytes(b"not a real key")
    (home / CD_BASE / "claude_desktop_config.json").write_text(
        '{"mcpServers":{}}', encoding="utf-8"
    )
