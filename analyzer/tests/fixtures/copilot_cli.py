"""Synthetic GitHub Copilot CLI state for the analyzer tests."""

from __future__ import annotations

import os
from pathlib import Path

from .common import _json_file, _wal_database

# GitHub Copilot CLI: the sample records of
# research/copilot-cli.md, Parser plan, verbatim, plus the `/model` switch to
# Auto seen on a Linux install: a `model_picker` change and its two
# `session.info` lines. The last line is cut mid-write, as a log
# whose session was still running.
COPILOT_SESSION = "5e55a0a1-0000-4000-8000-000000000001"
COPILOT_DIR = ".copilot/session-state/%s/" % COPILOT_SESSION
COPILOT_EVENTS_REL = COPILOT_DIR + "events.jsonl"
COPILOT_WORKSPACE_REL = COPILOT_DIR + "workspace.yaml"
COPILOT_SNAPSHOTS_REL = COPILOT_DIR + "rewind-file-snapshots/index.json"
COPILOT_STORE_REL = ".copilot/session-store.db"
COPILOT_HISTORY_REL = ".copilot/command-history-state.json"
COPILOT_PERMISSIONS_REL = ".copilot/permissions-config.json"
COPILOT_SETTINGS_REL = ".copilot/settings.json"
COPILOT_SIDEBAR_REL = (
    ".copilot/sidebar-sessions-state/"
    "6b9af143446a411aefa3edb76e4693799bd7513e7ed8d919962d6713d3d3b972.json"
)
COPILOT_LOG_REL = ".copilot/logs/process-1790845199000-4242.log"
COPILOT_SETTINGS_MTIME = 1790856000  # 1 October 12:00:00 UTC

COPILOT_EVENT_LINES = [
    '{"id":"e0000000-0000-4000-8000-000000000001","parentId":null,"timestamp":"2026-10-01T09:00:00.000Z","type":"session.start","data":{"sessionId":"5e55a0a1-0000-4000-8000-000000000001","version":1,"producer":"copilot-agent","copilotVersion":"1.0.98","startTime":"2026-10-01T08:59:59.000Z","context":{"cwd":"/srv/proj","gitRoot":"/srv/proj","branch":"main"},"alreadyInUse":false,"remoteSteerable":false,"contextTier":null}}',
    '{"id":"e0000000-0000-4000-8000-000000000002","parentId":"e0000000-0000-4000-8000-000000000001","timestamp":"2026-10-01T09:00:00.100Z","type":"session.model_change","data":{"newModel":"gpt-6.1-sol","cause":"initial_resolution","source":"automatic","reasoningEffort":"medium","contextTier":null}}',
    '{"id":"e0000000-0000-4000-8000-000000000003","parentId":"e0000000-0000-4000-8000-000000000002","timestamp":"2026-10-01T09:00:01.000Z","type":"system.message","data":{"content":"You are the GitHub Copilot CLI.\\nFollow the user\'s instructions.","role":"system","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","metadata":{"variables":{"gitCommitTrailer":{"clientName":"copilot-cli","coauthorEnabled":true,"sessionId":"5e55a0a1-0000-4000-8000-000000000001"}}}}}',
    '{"id":"e0000000-0000-4000-8000-000000000004","parentId":"e0000000-0000-4000-8000-000000000003","timestamp":"2026-10-01T09:00:01.200Z","type":"user.message","data":{"content":"fix the failing test","transformedContent":"<current_datetime>2026-10-01T09:00:01Z</current_datetime>\\nfix the failing test","messageId":"2b2b2b2b-0000-4000-8000-000000000001","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","parentAgentTaskId":"3c3c3c3c-0000-4000-8000-000000000001","turnId":"0","delivery":"idle","responsesReasoning":{"effort":"medium","initialEffort":"medium","model":"gpt-6.1-sol"},"supportedNativeDocumentMimeTypes":[]}}',
    '{"id":"e0000000-0000-4000-8000-000000000005","parentId":"e0000000-0000-4000-8000-000000000004","timestamp":"2026-10-01T09:00:01.300Z","type":"assistant.turn_start","data":{"interactionId":"1a1a1a1a-0000-4000-8000-000000000001","turnId":"0"}}',
    '{"id":"e0000000-0000-4000-8000-000000000006","parentId":null,"timestamp":"2026-10-01T09:00:01.400Z","type":"model.turn_started","data":{"kind":"turn_started","model":"gpt-4o-mini","modelInfo":{"id":"gpt-4o-mini","name":"GPT-4o mini","object":"model","version":"gpt-4o-mini-2024-07-18","provider":{"id":"copilot","kind":"copilot","label":"GitHub Copilot"}},"timestampMs":1790845201400,"turn":0}}',
    '{"id":"e0000000-0000-4000-8000-000000000007","parentId":"e0000000-0000-4000-8000-000000000006","timestamp":"2026-10-01T09:00:01.410Z","type":"model.model_call_started","data":{"kind":"model_call_started","model":"gpt-4o-mini","modelInfo":{"id":"gpt-4o-mini","name":"GPT-4o mini","object":"model","version":"gpt-4o-mini-2024-07-18","provider":{"id":"copilot","kind":"copilot","label":"GitHub Copilot"}},"timestampMs":1790845201410,"turn":0}}',
    '{"id":"e0000000-0000-4000-8000-000000000008","parentId":"e0000000-0000-4000-8000-000000000007","timestamp":"2026-10-01T09:00:01.420Z","type":"model.messages_snapshot","data":{"kind":"messages_snapshot","messages":[{"role":"system","content":"Generate a session title for this message."},{"role":"user","content":"fix the failing test"}]}}',
    '{"id":"e0000000-0000-4000-8000-000000000009","parentId":"e0000000-0000-4000-8000-000000000008","timestamp":"2026-10-01T09:00:01.430Z","type":"model.captured_assignment_context","data":{"kind":"captured_assignment_context","assignmentContext":"exp-a:1;exp-b:0;"}}',
    '{"id":"e0000000-0000-4000-8000-000000000010","parentId":"e0000000-0000-4000-8000-000000000009","timestamp":"2026-10-01T09:00:02.000Z","type":"model.message","data":{"callId":"4d4d4d4d-0000-4000-8000-000000000001","kind":"message","message":{"apiCallId":"chatcmpl-0001","content":"Fix failing test","outputTokens":4,"providerId":"copilot","refusal":null,"role":"assistant"},"modelCall":{"api_endpoint":"/chat/completions","api_id":"chatcmpl-0001","client_request_id":"req-0001","initiator":"agent","model":"gpt-4o-mini","request_id":"5f5f5f5f-0000-4000-8000-000000000001","rte":false,"service_request_id":"6a6a6a6a-0000-4000-8000-000000000001","transport":"http"},"turn":0}}',
    '{"id":"e0000000-0000-4000-8000-000000000011","parentId":"e0000000-0000-4000-8000-000000000010","timestamp":"2026-10-01T09:00:02.010Z","type":"model.response","data":{"callId":"4d4d4d4d-0000-4000-8000-000000000001","kind":"response","modelCall":{"api_id":"chatcmpl-0001","model":"gpt-4o-mini"},"response":{"content":"Fix failing test","providerId":"copilot","refusal":null,"role":"assistant"},"turn":0}}',
    '{"id":"e0000000-0000-4000-8000-000000000012","parentId":"e0000000-0000-4000-8000-000000000011","timestamp":"2026-10-01T09:00:02.020Z","type":"model.model_call_success","data":{"callId":"4d4d4d4d-0000-4000-8000-000000000001","kind":"model_call_success","modelCall":{"api_endpoint":"/chat/completions","api_id":"chatcmpl-0001","client_request_id":"req-0001","initiator":"agent","model":"gpt-4o-mini","request_id":"5f5f5f5f-0000-4000-8000-000000000001","rte":false,"service_request_id":"6a6a6a6a-0000-4000-8000-000000000001","transport":"http"},"modelCallDurationMs":590,"modelDisplayName":"GPT-4o mini","provider":{"id":"copilot","kind":"copilot","label":"GitHub Copilot"}}}',
    '{"id":"e0000000-0000-4000-8000-000000000013","parentId":"e0000000-0000-4000-8000-000000000012","timestamp":"2026-10-01T09:00:02.030Z","type":"model.turn_ended","data":{"kind":"turn_ended","model":"gpt-4o-mini","modelInfo":{"id":"gpt-4o-mini","name":"GPT-4o mini","object":"model","version":"gpt-4o-mini-2024-07-18","provider":{"id":"copilot","kind":"copilot","label":"GitHub Copilot"}},"timestampMs":1790845202030,"turn":0}}',
    '{"id":"e0000000-0000-4000-8000-000000000014","parentId":"e0000000-0000-4000-8000-000000000013","timestamp":"2026-10-01T09:00:03.000Z","type":"assistant.message","data":{"content":"","messageId":"7b7b7b7b-0000-4000-8000-000000000001","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","originatingMessageId":"2b2b2b2b-0000-4000-8000-000000000001","model":"gpt-6.1-sol","providerId":"copilot","turnId":"0","reasoningText":"Run the tests first.","reasoningOpaque":"rs_AAAA","encryptedContent":"gAAAAA","reasoningBlocks":{"provider":"openai-responses","blocks":[{"type":"reasoning","id":"rs_AAAA","summary":[{"type":"summary_text","text":"Run the tests first."}],"encrypted_content":"gAAAAA"}]},"apiCallId":"resp_0001","rte":false,"serverTools":{"provider":"openai-responses"},"toolRequests":[{"type":"function","name":"bash","toolCallId":"call_01","toolTitle":"Run shell command","arguments":{"command":"npm test","description":"Run the test suite"},"intentionSummary":"Run the test suite"}]}}',
    '{"id":"e0000000-0000-4000-8000-000000000015","parentId":"e0000000-0000-4000-8000-000000000014","timestamp":"2026-10-01T09:00:03.100Z","type":"permission.requested","data":{"requestId":"8c8c8c8c-0000-4000-8000-000000000001","ownerSessionId":"5e55a0a1-0000-4000-8000-000000000001","agentMode":"interactive","permissionMode":"manual","permissionRequest":{"kind":"shell","toolCallId":"call_01","intention":"Run the test suite","fullCommandText":"npm test","shellType":"bash","commands":[{"identifier":"npm test","readOnly":false,"allowMatchIdentifier":"npm test"}],"commandSegments":[{"identifier":"npm test","fullCommandText":"npm test"}],"possiblePaths":[],"possibleUrls":[],"hasWriteFileRedirection":false,"canOfferSessionApproval":true},"promptRequest":{"kind":"commands","toolCallId":"call_01","intention":"Run the test suite","fullCommandText":"npm test","commandIdentifiers":["npm test"]}}}',
    '{"id":"e0000000-0000-4000-8000-000000000016","parentId":"e0000000-0000-4000-8000-000000000015","timestamp":"2026-10-01T09:00:06.000Z","type":"permission.completed","data":{"requestId":"8c8c8c8c-0000-4000-8000-000000000001","toolCallId":"call_01","decisionSource":"human_response","result":{"kind":"approved-for-location","locationKey":"/srv/proj","approval":{"kind":"commands","commandIdentifiers":["npm test"]}}}}',
    '{"id":"e0000000-0000-4000-8000-000000000017","parentId":"e0000000-0000-4000-8000-000000000016","timestamp":"2026-10-01T09:00:06.100Z","type":"tool.execution_start","data":{"toolCallId":"call_01","toolName":"bash","toolTitle":"Run shell command","arguments":{"command":"npm test","description":"Run the test suite"},"model":"gpt-6.1-sol","turnId":"0"}}',
    '{"id":"e0000000-0000-4000-8000-000000000018","parentId":"e0000000-0000-4000-8000-000000000017","timestamp":"2026-10-01T09:00:08.000Z","type":"tool.execution_complete","data":{"toolCallId":"call_01","success":false,"result":{"content":"1 failing","detailedContent":"1 failing\\n  date.test.js: expected 2026"},"error":{"code":"failure","message":"Command exited with code 1"},"shellExecution":{"exitCode":1},"toolTelemetry":{"metrics":{},"properties":{}},"model":"gpt-6.1-sol","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","turnId":"0","rte":false}}',
    '{"id":"e0000000-0000-4000-8000-000000000019","parentId":"e0000000-0000-4000-8000-000000000018","timestamp":"2026-10-01T09:00:08.100Z","type":"assistant.turn_end","data":{"turnId":"0"}}',
    '{"id":"e0000000-0000-4000-8000-000000000020","parentId":"e0000000-0000-4000-8000-000000000019","timestamp":"2026-10-01T09:00:08.200Z","type":"assistant.turn_start","data":{"interactionId":"1a1a1a1a-0000-4000-8000-000000000001","turnId":"1"}}',
    '{"id":"e0000000-0000-4000-8000-000000000021","parentId":"e0000000-0000-4000-8000-000000000020","timestamp":"2026-10-01T09:00:10.000Z","type":"assistant.message","data":{"content":"","messageId":"7b7b7b7b-0000-4000-8000-000000000002","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","originatingMessageId":"2b2b2b2b-0000-4000-8000-000000000001","model":"gpt-6.1-sol","providerId":"copilot","turnId":"1","apiCallId":"resp_0002","rte":false,"toolRequests":[{"type":"custom","name":"apply_patch","toolCallId":"custom_0000000000000000000000000001","toolTitle":"Apply patch","arguments":"*** Begin Patch\\n*** Update File: /srv/proj/date.test.js\\n-2025\\n+2026\\n*** End Patch\\n","intentionSummary":"Update the fixture year"}]}}',
    '{"id":"e0000000-0000-4000-8000-000000000022","parentId":"e0000000-0000-4000-8000-000000000021","timestamp":"2026-10-01T09:00:10.100Z","type":"permission.requested","data":{"requestId":"8c8c8c8c-0000-4000-8000-000000000002","ownerSessionId":"5e55a0a1-0000-4000-8000-000000000001","agentMode":"interactive","permissionMode":"manual","permissionRequest":{"kind":"write","toolCallId":"custom_0000000000000000000000000001","intention":"Update the fixture year","fileName":"/srv/proj/date.test.js","diff":"-2025\\n+2026\\n","canOfferSessionApproval":true},"promptRequest":{"kind":"write","toolCallId":"custom_0000000000000000000000000001","intention":"Update the fixture year","fileName":"/srv/proj/date.test.js"}}}',
    '{"id":"e0000000-0000-4000-8000-000000000023","parentId":"e0000000-0000-4000-8000-000000000022","timestamp":"2026-10-01T09:00:12.000Z","type":"permission.completed","data":{"requestId":"8c8c8c8c-0000-4000-8000-000000000002","toolCallId":"custom_0000000000000000000000000001","decisionSource":"human_response","result":{"kind":"approved"}}}',
    '{"id":"e0000000-0000-4000-8000-000000000024","parentId":"e0000000-0000-4000-8000-000000000023","timestamp":"2026-10-01T09:00:12.100Z","type":"tool.execution_start","data":{"toolCallId":"custom_0000000000000000000000000001","toolName":"apply_patch","toolTitle":"Apply patch","arguments":{"input":"*** Begin Patch\\n*** Update File: /srv/proj/date.test.js\\n-2025\\n+2026\\n*** End Patch\\n"},"model":"gpt-6.1-sol","turnId":"1"}}',
    '{"id":"e0000000-0000-4000-8000-000000000025","parentId":"e0000000-0000-4000-8000-000000000024","timestamp":"2026-10-01T09:00:12.500Z","type":"tool.execution_complete","data":{"toolCallId":"custom_0000000000000000000000000001","success":true,"result":{"content":"Modified 1 file: /srv/proj/date.test.js"},"toolTelemetry":{"metrics":{},"properties":{}},"model":"gpt-6.1-sol","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","turnId":"1","rte":false}}',
    '{"id":"e0000000-0000-4000-8000-000000000026","parentId":"e0000000-0000-4000-8000-000000000025","timestamp":"2026-10-01T09:00:12.600Z","type":"assistant.turn_end","data":{"turnId":"1"}}',
    '{"id":"e0000000-0000-4000-8000-000000000027","parentId":"e0000000-0000-4000-8000-000000000026","timestamp":"2026-10-01T09:00:12.700Z","type":"assistant.turn_start","data":{"interactionId":"1a1a1a1a-0000-4000-8000-000000000001","turnId":"2"}}',
    '{"id":"e0000000-0000-4000-8000-000000000028","parentId":"e0000000-0000-4000-8000-000000000027","timestamp":"2026-10-01T09:00:15.000Z","type":"assistant.message","data":{"content":"The fixture year was stale; it now reads 2026.","messageId":"7b7b7b7b-0000-4000-8000-000000000003","interactionId":"1a1a1a1a-0000-4000-8000-000000000001","originatingMessageId":"2b2b2b2b-0000-4000-8000-000000000001","model":"gpt-6.1-sol","providerId":"copilot","turnId":"2","phase":"final_answer","apiCallId":"resp_0003","rte":false,"toolRequests":[]}}',
    '{"id":"e0000000-0000-4000-8000-000000000029","parentId":"e0000000-0000-4000-8000-000000000028","timestamp":"2026-10-01T09:00:15.100Z","type":"assistant.turn_end","data":{"turnId":"2"}}',
    '{"id":"e0000000-0000-4000-8000-000000000030","parentId":"e0000000-0000-4000-8000-000000000029","timestamp":"2026-10-01T09:00:15.200Z","type":"session.usage_record","data":{"usage":{"model":"gpt-6.1-sol","inputTokens":900,"outputTokens":40,"initiator":"agent","interactionType":"conversation-agent","apiEndpoint":"ws:/responses","transport":"websocket","isByok":false,"accounting":{"sequence":1,"sourceSessionId":"5e55a0a1-0000-4000-8000-000000000001","usageId":"u-0001"}}}}',
    '{"id":"e0000000-0000-4000-8000-000000000031","parentId":"e0000000-0000-4000-8000-000000000030","timestamp":"2026-10-01T09:00:20.000Z","type":"session.model_change","data":{"previousModel":"gpt-6.1-sol","newModel":"auto","previousReasoningEffort":"medium","reasoningEffort":null,"contextTier":null,"source":"model_picker"}}',
    '{"id":"e0000000-0000-4000-8000-000000000032","parentId":"e0000000-0000-4000-8000-000000000031","timestamp":"2026-10-01T09:00:20.395Z","type":"session.info","data":{"message":"Model changed from GitHub Copilot \u2022 GPT-6.1 Sol (medium) to GitHub Copilot \u2022 Auto for this session","infoType":"model"}}',
    '{"id":"e0000000-0000-4000-8000-000000000033","parentId":"e0000000-0000-4000-8000-000000000032","timestamp":"2026-10-01T09:00:20.396Z","type":"session.info","data":{"message":"Auto routing profile change to Balance requested.","infoType":"model"}}',
    '{"id":"e0000000-0000-4000-8000-000000000034","parentId":"e0000000-0000-4000-8000-000000000033","timestamp":"2026-10-01T09:00:30.000Z","type":"session.shutdown","data":{"shutdownType":"routine","sessionStartTime":1790845199000,"totalApiDurationMs":9000,"eventsFileSizeBytes":20000,"codeChanges":{"filesModified":["/srv/proj/date.test.js"],"linesAdded":1,"linesRemoved":1},"modelMetrics":{},"currentModel":"gpt-6.1-sol"}}',
    '{"id":"e0000000-0000-4000-8000-000000000035","parentId":"e0000000-0000-4000-8000-000000000034","timestamp":"2026-10-01T09:01:00.000Z","type":"user.message","data":{"content":"and run',
]

# session-store.db, schema version 38 as research/copilot-cli.md, Other stores,
# lists its tables; only the columns named there.
COPILOT_STORE_SCHEMA = """
CREATE TABLE sessions (id TEXT PRIMARY KEY, cwd TEXT, repository TEXT, host_type TEXT,
  branch TEXT, summary TEXT, created_at TEXT, updated_at TEXT);
CREATE TABLE turns (session_id TEXT, turn_index INTEGER, user_message TEXT,
  assistant_response TEXT, timestamp TEXT, UNIQUE (session_id, turn_index));
CREATE TABLE session_files (session_id TEXT, file_path TEXT, tool_name TEXT,
  turn_index INTEGER, first_seen_at TEXT, UNIQUE (session_id, file_path));
CREATE TABLE session_refs (session_id TEXT, ref_type TEXT, ref_value TEXT,
  turn_index INTEGER, created_at TEXT);
CREATE TABLE checkpoints (session_id TEXT, checkpoint_number INTEGER, title TEXT,
  overview TEXT, history TEXT, work_done TEXT, technical_details TEXT, important_files TEXT,
  next_steps TEXT, created_at TEXT);
CREATE TABLE forge_trajectory_events (session_id TEXT, tool_call_id TEXT, turn_index INTEGER,
  event_type TEXT, command TEXT, output TEXT, exit_code INTEGER, event_key TEXT,
  event_value TEXT, created_at TEXT);
CREATE TABLE assistant_usage_events (session_id TEXT, model TEXT, total_nano_aiu INTEGER);
"""

COPILOT_STORE_ROWS = [
    "INSERT INTO sessions VALUES ('5e55a0a1-0000-4000-8000-000000000001', '/srv/proj', NULL,"
    " NULL, NULL, 'Fix failing test', '2026-10-01T09:00:00.000Z', '2026-10-01T09:00:30.000Z')",
    "INSERT INTO turns (session_id, turn_index, user_message, assistant_response, timestamp)"
    " VALUES ('5e55a0a1-0000-4000-8000-000000000001', 0, 'fix the failing test',"
    " 'The fixture year was stale; it now reads 2026.', '2026-10-01T09:00:15.200Z')",
    "INSERT INTO turns (session_id, turn_index, user_message, assistant_response, timestamp)"
    " VALUES ('5e55a0a1-0000-4000-8000-000000000001', 1, 'and run lint', NULL,"
    " '2026-10-01T09:01:05.000Z')",
    "INSERT INTO session_files (session_id, file_path, tool_name, turn_index, first_seen_at)"
    " VALUES ('5e55a0a1-0000-4000-8000-000000000001', '/srv/proj/date.test.js', 'apply_patch',"
    " 0, '2026-10-01T09:00:12.500Z')",
]

# The research document's sample, with the three `mc_` keys set as a
# resumed session synced to the account wrote them on Windows 1.0.98 (issue
# 151); the values are synthetic UUIDs. The file's mtime is the resume.
COPILOT_MC_TASK = "1c1c1c1c-0000-4000-8000-000000000001"
COPILOT_MC_SESSION = "2d2d2d2d-0000-4000-8000-000000000001"
COPILOT_MC_LAST_EVENT = "3e3e3e3e-0000-4000-8000-000000000001"
COPILOT_WORKSPACE_MTIME = 1790845290  # 1 October 09:01:30 UTC
COPILOT_WORKSPACE_YAML = """\
id: 5e55a0a1-0000-4000-8000-000000000001
cwd: /srv/proj
client_name: copilot-cli
name: Fix failing test
user_named: false
summary_count: 0
fork_count: 0
created_at: 2026-10-01T09:00:00.000Z
updated_at: 2026-10-01T09:00:30.000Z
remote_steerable: false
mc_task_id: %s
mc_session_id: %s
mc_last_event_id: %s
""" % (COPILOT_MC_TASK, COPILOT_MC_SESSION, COPILOT_MC_LAST_EVENT)

COPILOT_SETTINGS = """\
{
  // user settings
  "allowedUrls": ["https://registry.example.org"]
}
"""

COPILOT_LOG = """\
2026-10-01T08:59:59.100Z [INFO] Starting Copilot CLI: 1.0.98
2026-10-01T08:59:59.400Z [INFO] Signed in to https://github.com as octo-user.
2026-10-01T08:59:59.900Z [INFO] Workspace initialized: 5e55a0a1-0000-4000-8000-000000000001
"""


def copilot_snapshot_index() -> dict:
    """Copilot CLI's `rewind-file-snapshots/index.json`.

    `rewind-file-snapshots/index.json` with the snapshot taken before the first prompt (its
    `eventId` is that `user.message` line's `id`), in the key layout of research/copilot-cli.md,
    Other stores; the file key is a synthetic hex digest and the images are left out.
    """
    return {
        "schema": 1,
        "snapshots": [
            {
                "snapshotId": "9d9d9d9d-0000-4000-8000-000000000001",
                "eventId": "e0000000-0000-4000-8000-000000000004",
                "timestamp": "2026-10-01T09:00:01.150Z",
                "userMessage": "fix the failing test",
                "fileCount": 1,
                "linesAdded": 1,
                "linesRemoved": 1,
                "backupFiles": [],
                "files": {
                    "a1" * 8: {
                        "path": "/srv/proj/date.test.js",
                        "linesAdded": 1,
                        "linesRemoved": 1,
                        "preimage": None,
                        "postimage": None,
                    }
                },
            }
        ],
    }


def build_copilot_cli(home: Path) -> None:
    events = home / COPILOT_EVENTS_REL
    events.parent.mkdir(parents=True, exist_ok=True)
    events.write_text("\n".join(COPILOT_EVENT_LINES), encoding="utf-8")
    (home / COPILOT_WORKSPACE_REL).write_text(COPILOT_WORKSPACE_YAML, encoding="utf-8")
    os.utime(home / COPILOT_WORKSPACE_REL, (COPILOT_WORKSPACE_MTIME, COPILOT_WORKSPACE_MTIME))
    _json_file(home / COPILOT_SNAPSHOTS_REL, copilot_snapshot_index())
    _wal_database(home / COPILOT_STORE_REL, COPILOT_STORE_SCHEMA, COPILOT_STORE_ROWS)
    # Newest first; `/model` was typed between the two prompts.
    _json_file(
        home / COPILOT_HISTORY_REL,
        {"commandHistory": ["and run lint", "/model", "fix the failing test"]},
    )
    _json_file(
        home / COPILOT_PERMISSIONS_REL,
        {
            "locations": {
                "/srv/proj": {
                    "tool_approvals": [
                        {"kind": "write"},
                        {"kind": "commands", "commandIdentifiers": ["npm test"]},
                    ]
                }
            }
        },
    )
    (home / COPILOT_SETTINGS_REL).write_text(COPILOT_SETTINGS, encoding="utf-8")
    for rel in (COPILOT_PERMISSIONS_REL, COPILOT_SETTINGS_REL):
        os.utime(home / rel, (COPILOT_SETTINGS_MTIME, COPILOT_SETTINGS_MTIME))
    _json_file(
        home / COPILOT_SIDEBAR_REL,
        {"schemaVersion": 1, "cwd": "/srv/proj", "sessionIds": [COPILOT_SESSION]},
    )
    (home / COPILOT_LOG_REL).parent.mkdir(parents=True, exist_ok=True)
    (home / COPILOT_LOG_REL).write_text(COPILOT_LOG, encoding="utf-8")
    # Noise the parser must not want (research/copilot-cli.md, Parser plan).
    (home / ".copilot/config.json").write_text("{}", encoding="utf-8")
    (home / ".copilot/open-sessions-state.json").write_text("{}", encoding="utf-8")
    _json_file(home / COPILOT_DIR / "rewind-file-snapshots/tracking.json", {"schema": 1})
    (home / COPILOT_DIR / "checkpoints").mkdir(parents=True, exist_ok=True)
    (home / COPILOT_DIR / "checkpoints/index.md").write_text(
        "| # | Title |\n| --- | --- |\n", encoding="utf-8"
    )
