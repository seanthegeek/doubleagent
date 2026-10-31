"""Synthetic OpenClaw state for the analyzer tests, from research/openclaw.md.

Paths: `/srv/scratch`
"""

from __future__ import annotations

import json
from pathlib import Path

import zstandard

from .common import PROJECT_REMOTE, _jsonl, _wal_database

# -- OpenClaw (openclaw/openclaw at fc3490a, older shapes at 3b16db7 and 146284a) ---------
OPENCLAW_SESSION = "7d0c2a8e-1111-4000-8000-000000000001"
OPENCLAW_KEY = "agent:main:telegram:default:direct:123456789"
OPENCLAW_RESET_SESSION = "7d0c2a8e-1111-4000-8000-000000000002"
OPENCLAW_COLD_SESSION = "7d0c2a8e-1111-4000-8000-000000000003"
OPENCLAW_CHUNKED_SESSION = "7d0c2a8e-1111-4000-8000-000000000004"
OPENCLAW_LEGACY = "7d0c2a8e-1111-4000-8000-0000000000aa"
OPENCLAW_DELETED = "7d0c2a8e-1111-4000-8000-0000000000dd"
OPENCLAW_TOKEN = "sk-ant-oat01-FIXTURE-OPENCLAW-TOKEN-0f9e8d"
OPENCLAW_AGENT = ".openclaw/agents/main"
OPENCLAW_CHECKPOINT = ".checkpoint.0b5e1c2d-3f4a-4b5c-8d6e-7f8091a2b3c4.jsonl"
OPENCLAW_REPAIR_BAK = ".jsonl.pre-doctor-branch-repair-2026-09-20T08-10-00-000Z.bak"

# Columns of src/state/openclaw-agent-schema.sql at 3b16db7 that the parser
# reads (CHECK constraints and unrelated columns left out).
OPENCLAW_SCHEMA = """
CREATE TABLE session_windows (session_id TEXT NOT NULL PRIMARY KEY, session_key TEXT NOT NULL,
  previous_session_id TEXT, reason TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL,
  started_at INTEGER, ended_at INTEGER, status TEXT, chat_type TEXT, channel TEXT, account_id TEXT,
  model_provider TEXT, model TEXT, parent_session_key TEXT, spawned_by TEXT, display_name TEXT,
  hook_external_content_source TEXT);
CREATE TABLE transcript_events (session_id TEXT NOT NULL, seq INTEGER NOT NULL, event_json TEXT,
  created_at INTEGER NOT NULL, event_zstd BLOB, event_utf8_bytes INTEGER, navigation_json TEXT,
  PRIMARY KEY (session_id, seq));
CREATE TABLE session_transcript_archives (session_id TEXT NOT NULL, generation TEXT NOT NULL,
  session_key TEXT NOT NULL, reason TEXT NOT NULL, encoding TEXT NOT NULL, archive_blob BLOB NOT NULL,
  archive_sha256 TEXT NOT NULL, archive_name TEXT NOT NULL UNIQUE, created_at INTEGER NOT NULL,
  published_at INTEGER, PRIMARY KEY (session_id, generation));
CREATE TABLE session_transcript_cold_archives (session_id TEXT NOT NULL PRIMARY KEY, generation TEXT NOT NULL,
  archive_name TEXT NOT NULL UNIQUE, archive_sha256 TEXT NOT NULL, event_count INTEGER NOT NULL,
  raw_bytes INTEGER NOT NULL, archive_bytes INTEGER NOT NULL, last_seq INTEGER NOT NULL,
  archived_at INTEGER NOT NULL, storage TEXT NOT NULL, archive_blob BLOB);
CREATE TABLE session_transcript_cold_archive_chunks (session_id TEXT NOT NULL,
  chunk_index INTEGER NOT NULL, archive_name TEXT NOT NULL UNIQUE, archive_sha256 TEXT NOT NULL,
  first_seq INTEGER NOT NULL, last_seq INTEGER NOT NULL, event_count INTEGER NOT NULL,
  raw_bytes INTEGER NOT NULL, archive_bytes INTEGER NOT NULL, archive_blob BLOB,
  PRIMARY KEY (session_id, chunk_index)) WITHOUT ROWID;
CREATE TABLE auth_profile_store (store_key TEXT NOT NULL PRIMARY KEY, store_json TEXT NOT NULL,
  updated_at INTEGER NOT NULL);
"""


def openclaw_records(session_id=OPENCLAW_SESSION, cwd="/srv/proj"):
    """A session's header and entries in the writer's entry shapes, cwd /srv/proj.

    Source: research/openclaw.md#entries; source fc3490a
    """
    return [
        {
            "type": "session",
            "version": 4,
            "id": session_id,
            "timestamp": "2026-10-01T09:00:00.000Z",
            "cwd": cwd,
        },
        {
            "type": "message",
            "id": "a1b2c3d4",
            "parentId": None,
            "timestamp": "2026-10-01T09:00:01.000Z",
            "message": {
                "role": "user",
                "content": [{"type": "text", "text": "check disk space on the server"}],
                "timestamp": 1790845201000,
            },
        },
        {
            "type": "message",
            "id": "b2c3d4e5",
            "parentId": "a1b2c3d4",
            "timestamp": "2026-10-01T09:00:04.000Z",
            "message": {
                "role": "assistant",
                "content": [
                    {
                        "type": "thinking",
                        "thinking": "Use exec with df.",
                        "thinkingSignature": "sig-opaque",
                    },
                    {"type": "text", "text": "Checking."},
                    {
                        "type": "toolCall",
                        "id": "call_01",
                        "name": "exec",
                        "arguments": {"command": "df -h"},
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
            "id": "c3d4e5f6",
            "parentId": "b2c3d4e5",
            "timestamp": "2026-10-01T09:00:05.000Z",
            "message": {
                "role": "toolResult",
                "toolCallId": "call_01",
                "toolName": "exec",
                "content": [{"type": "text", "text": "/dev/sda1  50G  20G  30G  40% /"}],
                "isError": False,
                "timestamp": 1790845205000,
            },
        },
        {
            "type": "model_change",
            "id": "d4e5f6a7",
            "parentId": "c3d4e5f6",
            "timestamp": "2026-10-01T09:01:00.000Z",
            "provider": "openai",
            "modelId": "gpt-5",
        },
    ]


def openclaw_legacy_records(session_id=OPENCLAW_LEGACY):
    """A legacy version 3 transcript: runtime context, a `!` command, a compaction, an error.

    Source: research/openclaw.md#message-roles; source fc3490a
    """
    return [
        {
            "type": "session",
            "version": 3,
            "id": session_id,
            "timestamp": "2026-09-20T08:00:00.000Z",
            "cwd": "/srv/proj",
        },
        {
            "type": "message",
            "id": "e1",
            "parentId": None,
            "timestamp": "2026-09-20T08:00:01.000Z",
            "message": {
                "role": "user",
                "content": "<runtime>channel=telegram</runtime>",
                "runtimeContext": {"retained": True},
                "timestamp": 1789891201000,
            },
        },
        {
            "type": "message",
            "id": "e2",
            "parentId": "e1",
            "timestamp": "2026-09-20T08:00:02.000Z",
            "message": {
                "role": "bashExecution",
                "command": "uptime",
                "output": "up 3 days",
                "exitCode": 0,
                "cancelled": False,
                "truncated": False,
                "timestamp": 1789891202000,
            },
        },
        {
            "type": "compaction",
            "id": "e3",
            "parentId": "e2",
            "timestamp": "2026-09-20T08:05:00.000Z",
            "summary": "checked uptime",
            "firstKeptEntryId": "e2",
            "tokensBefore": 5000,
        },
        {
            "type": "message",
            "id": "e4",
            "parentId": "e3",
            "message": {
                "role": "assistant",
                "content": [{"type": "text", "text": "partial"}],
                "model": "gpt-5",
                "stopReason": "error",
                "errorMessage": "rate limited",
                "timestamp": 1789891510000,
            },
        },
    ]


# Session sync: session_nodes (src/state/openclaw-agent-schema.sql:16-49
# at 3b16db7, the columns the parser reads) with a `publicShare` grant
# (src/config/sessions/types.ts:627-628) on the live session and none on the
# reset one, and state/openclaw.sqlite backup_runs
# (src/state/openclaw-state-schema.sql:1843-1849) with manifests shaped by
# src/state/backup-run-records.contract.ts:98-124.
OPENCLAW_SHARE_ID = "0a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f6071"
OPENCLAW_STATE_DATABASE = ".openclaw/state/openclaw.sqlite"
OPENCLAW_BACKUP_KEY = "backups/main/2026-10-01T10-20-00Z-0a1b2c3d.tar.gz"
OPENCLAW_BACKUP_COMMIT = "9f8e7d6c5b4a39281706f5e4d3c2b1a098765432"
OPENCLAW_NODES_SCHEMA = """
CREATE TABLE session_nodes (session_key TEXT NOT NULL PRIMARY KEY, current_session_id TEXT NOT NULL,
  entry_json TEXT NOT NULL, updated_at INTEGER NOT NULL, created_via TEXT, label TEXT);
"""
# Relay, past-use and deletion tables: src/state/openclaw-agent-schema.sql
# at 146284a, the columns the parser reads. The full-text index is read from the FTS5
# content table `session_transcript_fts_content` (`id`, then `c0` to `c4` in the
# column order of lines 809-816) as a plain table.
OPENCLAW_PAST_USE_SCHEMA = """
CREATE TABLE conversations (conversation_id TEXT NOT NULL PRIMARY KEY, channel TEXT NOT NULL,
  account_id TEXT NOT NULL, kind TEXT NOT NULL, peer_id TEXT NOT NULL, delivery_target TEXT NOT NULL,
  thread_id TEXT, label TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE session_conversations (session_id TEXT NOT NULL, conversation_id TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'primary', first_seen_at INTEGER NOT NULL, last_seen_at INTEGER NOT NULL,
  PRIMARY KEY (session_id, conversation_id, role));
CREATE TABLE conversation_deliveries (operation_id TEXT NOT NULL PRIMARY KEY,
  operation_kind TEXT NOT NULL, conversation_id TEXT NOT NULL, source_session_key TEXT,
  message_hash TEXT NOT NULL, status TEXT NOT NULL, platform_message_id TEXT, rejection_error TEXT,
  reply_message_id TEXT, reply_text TEXT, reply_timestamp INTEGER, created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL);
CREATE TABLE session_pending_inputs (seq INTEGER PRIMARY KEY AUTOINCREMENT,
  input_id TEXT NOT NULL UNIQUE, session_key TEXT NOT NULL, session_id TEXT NOT NULL,
  run_id TEXT NOT NULL, message_json TEXT NOT NULL, state TEXT NOT NULL,
  accepted_at INTEGER NOT NULL, consumed_event_id TEXT);
CREATE TABLE session_transcript_fts_content (id INTEGER PRIMARY KEY, c0, c1, c2, c3, c4);
CREATE TABLE memory_session_tombstones (session_id TEXT NOT NULL PRIMARY KEY,
  agent_id TEXT NOT NULL, reason TEXT NOT NULL, created_at INTEGER NOT NULL);
"""
OPENCLAW_STATE_SCHEMA = """
CREATE TABLE backup_runs (id TEXT NOT NULL PRIMARY KEY, created_at INTEGER NOT NULL,
  archive_path TEXT NOT NULL, status TEXT NOT NULL, manifest_json TEXT NOT NULL) STRICT;
CREATE TABLE exec_approvals_config (config_key TEXT NOT NULL PRIMARY KEY, raw_json TEXT NOT NULL,
  has_socket_token INTEGER NOT NULL, updated_at_ms INTEGER NOT NULL);
CREATE TABLE operator_approvals (approval_id TEXT NOT NULL PRIMARY KEY, kind TEXT NOT NULL,
  status TEXT NOT NULL, presentation_json TEXT NOT NULL, source_agent_id TEXT,
  source_session_key TEXT, source_session_id TEXT, source_tool_call_id TEXT,
  source_tool_name TEXT, created_at_ms INTEGER NOT NULL, decision TEXT, terminal_reason TEXT,
  resolved_at_ms INTEGER, resolver_kind TEXT);
CREATE TABLE operator_approval_standing_grants (grant_id TEXT NOT NULL PRIMARY KEY,
  minted_by_approval_id TEXT NOT NULL, agent_id TEXT NOT NULL, cron_job_id TEXT NOT NULL,
  operation_binding TEXT NOT NULL, created_at_ms INTEGER NOT NULL, expires_at_ms INTEGER,
  revoked_at_ms INTEGER, use_count INTEGER NOT NULL DEFAULT 0);
CREATE TABLE channel_pairing_allow_entries (channel_key TEXT NOT NULL, account_id TEXT NOT NULL,
  entry TEXT NOT NULL, sort_order INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE channel_ingress_events (queue_name TEXT NOT NULL, event_id TEXT NOT NULL,
  channel_id TEXT NOT NULL, account_id TEXT NOT NULL, status TEXT NOT NULL,
  payload_json TEXT NOT NULL, received_at INTEGER NOT NULL);
CREATE TABLE github_publication_requests (request_id TEXT NOT NULL PRIMARY KEY,
  session_id TEXT NOT NULL, session_key TEXT NOT NULL, agent_id TEXT NOT NULL,
  identity_source TEXT NOT NULL, identity_profile_id TEXT, identity_account_id INTEGER NOT NULL,
  identity_login TEXT NOT NULL, status TEXT NOT NULL, repository TEXT, branch TEXT NOT NULL,
  pull_request_url TEXT, created_at_ms INTEGER NOT NULL, updated_at_ms INTEGER NOT NULL);
CREATE TABLE projects (id TEXT NOT NULL PRIMARY KEY, display_name TEXT NOT NULL,
  repo_root TEXT NOT NULL, origin_url TEXT,
  source TEXT NOT NULL CHECK (source IN ('registered', 'cloned')),
  created_at_ms INT NOT NULL, updated_at_ms INT NOT NULL);
"""
OPENCLAW_SOCKET_TOKEN = "FIXTURE-OPENCLAW-SOCKET-TOKEN-5c4b3a"
OPENCLAW_FTS_ONLY = "prompt only the search index still holds"


def openclaw_past_use_inserts():
    """The OpenClaw past-use rows: channels, deliveries and tombstones.

    A Telegram conversation linked to the live session and a Control UI one linked to the reset
    session; a delivery with a reply; a cancelled prompt that never reached the transcript, one
    consumed by a transcript entry the database holds and one consumed by an entry it lost; two
    full-text index rows, one of an entry the transcript holds; and a memory tombstone for the
    deleted session.
    """

    def prompt(text):
        return json.dumps({"role": "user", "content": [{"type": "text", "text": text}]})

    return [
        "INSERT INTO conversations VALUES ('conv-tg','telegram','default','direct','123456789',"
        "'telegram:123456789',NULL,NULL,1790845200000,1790845260000)",
        "INSERT INTO conversations VALUES ('conv-ui','webchat','default','direct','operator',"
        "'webchat:main',NULL,NULL,1790846000000,1790846000000)",
        "INSERT INTO session_conversations VALUES ('%s','conv-tg','primary',1790845200500,"
        "1790845260000)" % OPENCLAW_SESSION,
        "INSERT INTO session_conversations VALUES ('%s','conv-ui','primary',1790846000000,"
        "1790846000000)" % OPENCLAW_RESET_SESSION,
        "INSERT INTO conversation_deliveries VALUES ('op-1','turn','conv-tg','%s','%s','replied',"
        "'tg-msg-77',NULL,'tg-msg-78','thanks, looks fine',1790845290000,1790845206000,"
        "1790845290000)" % (OPENCLAW_KEY, "a" * 64),
        "INSERT INTO session_pending_inputs VALUES (1,'in-1','%s','%s','run-1','%s','cancelled',"
        "1790845250000,NULL)" % (OPENCLAW_KEY, OPENCLAW_SESSION, prompt("also restart nginx")),
        "INSERT INTO session_pending_inputs VALUES (2,'in-2','%s','%s','run-2','%s','queued',"
        "1790845201000,'a1b2c3d4')"
        % (OPENCLAW_KEY, OPENCLAW_SESSION, prompt("check disk space on the server")),
        "INSERT INTO session_pending_inputs VALUES (3,'in-3','%s','%s','run-3','%s','queued',"
        "1790845230000,'lost-entry')"
        % (OPENCLAW_KEY, OPENCLAW_SESSION, prompt("show the biggest directories")),
        "INSERT INTO session_transcript_fts_content VALUES (1,'check disk space on the server',"
        "'%s','a1b2c3d4','user',1790845201000)" % OPENCLAW_SESSION,
        "INSERT INTO session_transcript_fts_content VALUES (2,'%s','%s','gone-1','user',"
        "1790840000000)" % (OPENCLAW_FTS_ONLY, OPENCLAW_DELETED),
        "INSERT INTO memory_session_tombstones VALUES ('%s','main','forgotten',1790930000000)"
        % OPENCLAW_DELETED,
    ]


def openclaw_state_inserts():
    """Permission records of OpenClaw's `state/openclaw.sqlite`.

    Permission records of state/openclaw.sqlite (src/state/openclaw-state-schema.sql at 146284a,
    lines 356-475, 512-529 and 794-801) and one inbound channel message (lines 1273-1294); the exec
    approvals file keeps the socket token, which the parser never reads.
    """
    raw = {
        "version": 1,
        "socket": {"path": "/home/alice/.openclaw/exec.sock", "token": OPENCLAW_SOCKET_TOKEN},
        "defaults": {"security": "allowlist", "ask": "on-miss"},
        "agents": {
            "main": {
                "autoAllowSkills": True,
                "allowlist": [{"pattern": "df", "source": "allow-always"}],
                "mcpTools": [
                    {"server": "github", "tool": "search", "source": "allow-always", "addedAt": 1}
                ],
            }
        },
    }
    return [
        "INSERT INTO exec_approvals_config VALUES ('default','%s',1,1790845100000)"
        % json.dumps(raw),
        "INSERT INTO operator_approvals VALUES ('appr-1','exec','allowed','{}','main','%s','%s',"
        "'call_01','exec',1790845204100,'allow-once','user',1790845204500,'device')"
        % (OPENCLAW_KEY, OPENCLAW_SESSION),
        "INSERT INTO operator_approvals VALUES ('appr-2','exec','pending','{}','main','%s','%s',"
        "'call_02','exec',1790845300000,NULL,NULL,NULL,NULL)" % (OPENCLAW_KEY, OPENCLAW_SESSION),
        "INSERT INTO operator_approval_standing_grants VALUES ('grant-1','appr-1','main','cron-7',"
        "'exec:df',1790845205000,NULL,NULL,3)",
        "INSERT INTO channel_pairing_allow_entries VALUES ('telegram','default','123456789',0,"
        "1790845000000)",
        "INSERT INTO channel_ingress_events VALUES ('inbound','ev-1','telegram','default',"
        "'completed','{\"text\":\"check disk space on the server\"}',1790845200900)",
        # src/state/openclaw-state-schema.sql:2410-2482 at 146284a.
        "INSERT INTO github_publication_requests VALUES ('req-1','%s','%s','main',"
        "'system-configured','profile-1',4242,'octo-bot','published','octo/infra',"
        "'openclaw/disk-check','https://github.com/octo/infra/pull/7',1790845400000,"
        "1790845460000)" % (OPENCLAW_SESSION, OPENCLAW_KEY),
        # src/state/openclaw-state-schema.sql:1901-1909 at 146284a: a
        # registered repository with its origin, and one without.
        "INSERT INTO projects VALUES ('p-1','proj','/srv/proj','%s','registered',"
        "1790845100500,1790845100500)" % PROJECT_REMOTE,
        "INSERT INTO projects VALUES ('p-2','scratch','/srv/scratch',NULL,'registered',"
        "1790845100600,1790845100600)",
    ]


def openclaw_node_inserts():
    """A published live session and an unpublished reset session."""
    shared = {
        "sessionId": OPENCLAW_SESSION,
        "updatedAt": 1790845260000,
        "publicShare": {
            "id": OPENCLAW_SHARE_ID,
            "sessionId": OPENCLAW_SESSION,
            "createdAt": 1790845320000,
        },
        # src/config/sessions/types.ts:60-83, 481-516 and 532 at 146284a.
        "delivery": {
            "kind": "external",
            "route": {"channel": "telegram", "accountId": "default"},
            "context": {},
            "origin": {"provider": "telegram", "chatType": "direct", "from": "123456789"},
        },
        "providerOverride": "openai",
        "modelOverride": "gpt-5",
        "modelOverrideSource": "user",
        "elevatedLevel": "on",
        "execHost": "gateway",
        "toolOverrides": {"mcpServers": {"github": False}},
        "sendPolicy": "allow",
    }
    # Archived from the session list (types.ts:300-305); its delivery is the
    # Control UI, which is not a platform.
    plain = {
        "sessionId": OPENCLAW_RESET_SESSION,
        "updatedAt": 1790847000000,
        "delivery": {
            "kind": "external",
            "route": {"channel": "webchat"},
            "context": {},
            "origin": {"provider": "webchat"},
        },
        "archivedAt": 1790848000000,
        "archivedBy": {"type": "human", "id": "operator-1"},
        "archiveReason": "manual",
    }
    return [
        "INSERT INTO session_nodes VALUES ('%s','%s','%s',1790845320000,'channel',NULL)"
        % (OPENCLAW_KEY, OPENCLAW_SESSION, json.dumps(shared)),
        "INSERT INTO session_nodes VALUES ('agent:main:main','%s','%s',1790847000000,'operator',NULL)"
        % (OPENCLAW_RESET_SESSION, json.dumps(plain)),
    ]


def openclaw_backup_runs():
    """(id, created_at, archive_path, status, manifest).

    An offsite archive upload and a pushed Git backup, which yield rows, and a Git backup whose push
    failed, a local archive and a failed upload, which do not.
    """
    location = {
        "name": "r2-offsite",
        "provider": "cloudflare-r2",
        "locationId": "loc-01",
        "key": OPENCLAW_BACKUP_KEY,
        "namespace": "main",
        "plaintextBytes": 81920,
        "storedBytes": 40960,
    }
    return [
        (
            "run-01",
            1790850000000,
            "/home/alice/.openclaw/backups/2026-10-01T10-20-00Z.tar.gz",
            "ok",
            {
                "kind": "archive",
                "target": "r2-offsite",
                "namespace": "main",
                "bytes": 81920,
                "location": location,
                "retention": {"kept": 1, "deleted": 0},
            },
        ),
        (
            "run-02",
            1790852400000,
            "/home/alice/openclaw-backup.git",
            "ok",
            {"kind": "git", "target": OPENCLAW_BACKUP_COMMIT, "namespace": "main"},
        ),
        (
            "run-03",
            1790856000000,
            "/home/alice/openclaw-backup.git",
            "ok",
            {"kind": "git", "target": "1" * 40, "namespace": "main", "pushFailed": True},
        ),
        (
            "run-04",
            1790859600000,
            "/home/alice/.openclaw/backups/local.tar.gz",
            "ok",
            {"kind": "archive", "namespace": "main"},
        ),
        (
            "run-05",
            1790863200000,
            "/home/alice/.openclaw/backups/failed.tar.gz",
            "failed",
            {"kind": "archive", "target": "r2-offsite", "error": "upload refused"},
        ),
    ]


def build_openclaw_state(home: Path) -> None:
    inserts = [
        "INSERT INTO backup_runs VALUES ('%s',%d,'%s','%s','%s')"
        % (rid, created, path, status, json.dumps(manifest))
        for rid, created, path, status, manifest in openclaw_backup_runs()
    ]
    _wal_database(
        home / OPENCLAW_STATE_DATABASE, OPENCLAW_STATE_SCHEMA, inserts + openclaw_state_inserts()
    )


def _openclaw_zstd(raw: bytes) -> bytes:
    """zstd-compressed bytes, as OpenClaw writes them."""
    return zstandard.ZstdCompressor(level=1, write_checksum=True).compress(raw)


def _openclaw_jsonl(records) -> bytes:
    return "".join(json.dumps(record) + "\n" for record in records).encode("utf-8")


def openclaw_chunked_cold_inserts():
    """A `sqlite`-stored cold archive split into two chunks, each with its own header.

    The parent row keeps no blob; each chunk holds one entry of the generation.
    """
    records = openclaw_records(OPENCLAW_CHUNKED_SESSION)[:2]
    chunks = []
    for index, record in enumerate(records):
        lines = [
            {
                "kind": "header",
                "version": 1,
                "sessionId": OPENCLAW_CHUNKED_SESSION,
                "generation": "g5",
            },
            {
                "kind": "event",
                "row": {
                    "seq": index,
                    "event_json": json.dumps(record),
                    "created_at": 1790846000000,
                },
            },
        ]
        raw = _openclaw_jsonl(lines)
        chunks.append((index, raw, _openclaw_zstd(raw)))
    inserts = [
        "INSERT INTO session_transcript_cold_archives VALUES ('%s','g5','%s.jsonl.zst','%s',2,%d,"
        "%d,1,1790851000000,'sqlite',NULL)"
        % (
            OPENCLAW_CHUNKED_SESSION,
            "d" * 64,
            "d" * 64,
            sum(len(raw) for _, raw, _ in chunks),
            sum(len(blob) for _, _, blob in chunks),
        )
    ]
    for index, raw, blob in reversed(chunks):
        inserts.append(
            "INSERT INTO session_transcript_cold_archive_chunks VALUES ('%s',%d,'%s.jsonl.zst',"
            "'%s',%d,%d,1,%d,%d,X'%s')"
            % (
                OPENCLAW_CHUNKED_SESSION,
                index,
                str(index + 5) * 64,
                str(index + 5) * 64,
                index,
                index,
                len(raw),
                len(blob),
                blob.hex(),
            )
        )
    return inserts


def build_openclaw(home: Path) -> None:
    agent = home / OPENCLAW_AGENT
    inserts = [
        "INSERT INTO session_windows VALUES ('%s','%s',NULL,'initial',1790845200000,1790845260000,1790845200000,"
        "NULL,'running','direct','telegram','default','anthropic','claude-sonnet-4-5',NULL,NULL,'Disk check',"
        "NULL)" % (OPENCLAW_SESSION, OPENCLAW_KEY),
        "INSERT INTO auth_profile_store VALUES ('anthropic:default','%s',1790845200000)"
        % json.dumps(
            {
                "version": 1,
                "profiles": {
                    "anthropic:default": {
                        "type": "oauth",
                        "provider": "anthropic",
                        "access": OPENCLAW_TOKEN,
                        "refresh": OPENCLAW_TOKEN + "-refresh",
                        "expires": 1790931600000,
                    }
                },
            }
        ),
    ]
    for seq, record in enumerate(openclaw_records()):
        raw = json.dumps(record)
        compressed = _openclaw_zstd(raw.encode("utf-8")) if record.get("id") == "b2c3d4e5" else None
        created = 1790845200000 + seq * 1000
        if compressed is not None:
            inserts.append(
                "INSERT INTO transcript_events VALUES ('%s',%d,NULL,%d,X'%s',%d,'{\"version\":1}')"
                % (OPENCLAW_SESSION, seq, created, compressed.hex(), len(raw))
            )
        else:
            inserts.append(
                "INSERT INTO transcript_events VALUES ('%s',%d,'%s',%d,NULL,NULL,NULL)"
                % (OPENCLAW_SESSION, seq, raw.replace("'", "''"), created)
            )
    reset = _openclaw_jsonl(
        [
            *openclaw_records(OPENCLAW_RESET_SESSION)[:2],
            {
                "type": "reset",
                "id": "r1",
                "parentId": "a1b2c3d4",
                "timestamp": "2026-10-01T09:30:00.000Z",
                "reason": "reset",
            },
        ]
    )
    inserts.append(
        "INSERT INTO session_transcript_archives VALUES ('%s','g1','%s','reset','identity',X'%s','%s',"
        "'%s.jsonl.reset.2026-10-01T09-30-00.000Z',1790847000000,NULL)"
        % (OPENCLAW_RESET_SESSION, OPENCLAW_KEY, reset.hex(), "0" * 64, OPENCLAW_RESET_SESSION)
    )
    # Published: the same zstd bytes are the .jsonl.deleted.*.zst file below.
    deleted = _openclaw_jsonl(openclaw_records(OPENCLAW_DELETED)[:2])
    inserts.append(
        "INSERT INTO session_transcript_archives VALUES ('%s','g2','%s','deleted','zstd',X'%s','%s',"
        "'%s.jsonl.deleted.2026-10-02T08-00-00.000Z.zst',1790928000000,1790928000500)"
        % (
            OPENCLAW_DELETED,
            OPENCLAW_KEY,
            _openclaw_zstd(deleted).hex(),
            "1" * 64,
            OPENCLAW_DELETED,
        )
    )
    cold = [
        {"kind": "header", "version": 1, "sessionId": OPENCLAW_COLD_SESSION, "generation": "g0"}
    ]
    for seq, record in enumerate(openclaw_records(OPENCLAW_COLD_SESSION)[:2]):
        cold.append(
            {
                "kind": "event",
                "row": {"seq": seq, "event_json": json.dumps(record), "created_at": 1790845200000},
            }
        )
        cold.append(
            {
                "kind": "identity",
                "row": {
                    "event_id": record["id"],
                    "seq": seq,
                    "event_type": record["type"],
                    "parent_id": record.get("parentId"),
                    "message_idempotency_key": None,
                    "created_at": 1790845200000,
                },
            }
        )
    cold_raw = _openclaw_jsonl(cold)
    cold_blob = _openclaw_zstd(cold_raw)
    inserts.append(
        "INSERT INTO session_transcript_cold_archives VALUES ('%s','g0','%s.jsonl.zst','%s',2,%d,%d,1,"
        "1790850000000,'sqlite',X'%s')"
        % (
            OPENCLAW_COLD_SESSION,
            "c" * 64,
            "2" * 64,
            len(cold_raw),
            len(cold_blob),
            cold_blob.hex(),
        )
    )
    inserts += openclaw_chunked_cold_inserts()
    _wal_database(
        agent / "agent/openclaw-agent.sqlite",
        OPENCLAW_SCHEMA + OPENCLAW_NODES_SCHEMA + OPENCLAW_PAST_USE_SCHEMA,
        inserts + openclaw_node_inserts() + openclaw_past_use_inserts(),
    )
    sessions = agent / "sessions"
    _jsonl(sessions / (OPENCLAW_LEGACY + ".jsonl"), openclaw_legacy_records())
    (sessions / "sessions.json").write_text(
        json.dumps({OPENCLAW_KEY: {"sessionId": OPENCLAW_LEGACY, "updatedAt": 1789891510000}}),
        encoding="utf-8",
    )
    name = OPENCLAW_DELETED + ".jsonl.deleted.2026-10-02T08-00-00.000Z"
    (sessions / (name + ".zst")).write_bytes(_openclaw_zstd(deleted))
    # Secondary copies: a pre-compaction checkpoint holds the
    # entries before the compaction; a pre-doctor branch-repair backup holds
    # the whole file before the repair, with an abandoned branch the
    # repaired file dropped.
    _jsonl(sessions / (OPENCLAW_LEGACY + OPENCLAW_CHECKPOINT), openclaw_legacy_records()[:3])
    _jsonl(
        sessions / (OPENCLAW_LEGACY + OPENCLAW_REPAIR_BAK),
        [
            *openclaw_legacy_records(),
            {
                "type": "message",
                "id": "e9",
                "parentId": "e1",
                "timestamp": "2026-09-20T08:00:30.000Z",
                "message": {
                    "role": "user",
                    "content": "abandoned branch prompt",
                    "timestamp": 1789891230000,
                },
            },
        ],
    )
    # Noise the parser must skip: a trajectory (another schema) and a
    # `.migrated` file (not confirmed as a transcript).
    _jsonl(
        sessions / (OPENCLAW_LEGACY + ".trajectory.jsonl"),
        [{"traceSchema": "openclaw-trajectory", "schemaVersion": 1}],
    )
    _jsonl(sessions / (OPENCLAW_LEGACY + ".jsonl.migrated"), openclaw_legacy_records())
    (agent / "agent/auth-profiles.json").write_text(
        json.dumps({"anthropic:default": {"key": OPENCLAW_TOKEN}}), encoding="utf-8"
    )
    (home / ".openclaw/openclaw.json").write_text('{"agents": {}}', encoding="utf-8")
