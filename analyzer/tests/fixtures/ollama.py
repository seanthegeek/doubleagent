"""Synthetic Ollama state for the analyzer tests."""

from __future__ import annotations

from pathlib import Path

from .common import _wal_database

OLLAMA_CHAT = "0190a000-0000-7000-8000-00000000c001"
OLLAMA_DATABASE_REL = "Library/Application Support/Ollama/db.sqlite"
OLLAMA_WINDOWS_DATABASE_REL = "AppData/Local/Ollama/db.sqlite"
OLLAMA_USER_NAME = "Alice Example"
OLLAMA_USER_EMAIL = "alice@example.invalid"
OLLAMA_ATTACHMENT = b"hello"
OLLAMA_MANIFEST_REL = ".ollama/models/manifests/registry.ollama.ai/library/gemma4/e4b"

# app/store/database.go at ollama v0.35.1 (b0c1ca4), schema version 19
# (research/ollama.md, Parser plan). OLLAMA_SETTINGS_16 is the settings table
# before migrations 16 to 19 added onboarding_version, claude_desktop_used
# and codex_desktop_used; the transcript tables are the same in both.
OLLAMA_SETTINGS_19 = """
CREATE TABLE settings (id INTEGER PRIMARY KEY CHECK (id = 1), device_id TEXT NOT NULL DEFAULT '',
  working_dir TEXT NOT NULL DEFAULT '', selected_model TEXT NOT NULL DEFAULT '',
  onboarding_version INTEGER NOT NULL DEFAULT 0, claude_desktop_used BOOLEAN NOT NULL DEFAULT 0,
  codex_desktop_used BOOLEAN NOT NULL DEFAULT 0, schema_version INTEGER NOT NULL DEFAULT 19);
"""
OLLAMA_SETTINGS_16 = """
CREATE TABLE settings (id INTEGER PRIMARY KEY CHECK (id = 1), device_id TEXT NOT NULL DEFAULT '',
  working_dir TEXT NOT NULL DEFAULT '', selected_model TEXT NOT NULL DEFAULT '',
  schema_version INTEGER NOT NULL DEFAULT 16);
"""
OLLAMA_SCHEMA = """
CREATE TABLE chats (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, browser_state TEXT);
CREATE TABLE messages (id INTEGER PRIMARY KEY AUTOINCREMENT, chat_id TEXT NOT NULL, role TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '', thinking TEXT NOT NULL DEFAULT '', stream BOOLEAN NOT NULL DEFAULT 0,
  model_name TEXT, model_cloud BOOLEAN, model_ollama_host BOOLEAN,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  thinking_time_start TIMESTAMP, thinking_time_end TIMESTAMP, tool_result TEXT);
CREATE TABLE tool_calls (id INTEGER PRIMARY KEY AUTOINCREMENT, message_id INTEGER NOT NULL, type TEXT NOT NULL,
  function_name TEXT NOT NULL, function_arguments TEXT NOT NULL, function_result TEXT);
CREATE TABLE attachments (id INTEGER PRIMARY KEY AUTOINCREMENT, message_id INTEGER NOT NULL,
  filename TEXT NOT NULL, data BLOB NOT NULL);
CREATE TABLE users (name TEXT NOT NULL DEFAULT '', email TEXT NOT NULL DEFAULT '',
  plan TEXT NOT NULL DEFAULT '', cached_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
"""


def ollama_records(schema_version: int = 19):
    """The INSERT statements of research/ollama.md, Parser plan."""
    chat = OLLAMA_CHAT
    return [
        "INSERT INTO settings (id, device_id, schema_version) "
        "VALUES (1, '0190a000-0000-7000-8000-00000000d001', %d)" % schema_version,
        "INSERT INTO users VALUES ('%s', '%s', 'free', '2026-03-01 08:59:00-08:00')"
        % (OLLAMA_USER_NAME, OLLAMA_USER_EMAIL),
        "INSERT INTO chats VALUES ('%s', 'Release notes', '2026-03-01 09:00:00.5-08:00', NULL)"
        % chat,
        "INSERT INTO messages (id, chat_id, role, content, thinking, model_name, created_at, "
        "updated_at, tool_result) VALUES "
        "(1, '%s', 'user', 'summarise https://example.invalid/notes', '', NULL, "
        "'2026-03-01 09:00:00.5-08:00', '2026-03-01 09:00:00.5-08:00', NULL),"
        "(2, '%s', 'assistant', '', 'need to fetch the page', 'gemma4:e4b', "
        "'2026-03-01 09:00:01.25-08:00', '2026-03-01 09:00:02-08:00', NULL),"
        "(3, '%s', 'tool', 'Release 2.0 adds X.', '', NULL, "
        "'2026-03-01 09:00:03-08:00', '2026-03-01 09:00:03-08:00', "
        """'{"title":"Notes","content":"Release 2.0 adds X."}'),"""
        "(4, '%s', 'assistant', 'Release 2.0 adds X.', '', 'qwen3:8b', "
        "'2026-03-01 09:00:04.125-08:00', '2026-03-01 09:00:05-08:00', NULL)"
        % (chat, chat, chat, chat),
        "INSERT INTO tool_calls VALUES (1, 2, 'function', 'web_fetch', "
        """'{"url":"https://example.invalid/notes"}', NULL)""",
        "INSERT INTO attachments VALUES (1, 1, 'notes.txt', X'%s')" % OLLAMA_ATTACHMENT.hex(),
    ]


def write_ollama_database(path: Path, schema_version: int = 19) -> None:
    settings = OLLAMA_SETTINGS_19 if schema_version >= 19 else OLLAMA_SETTINGS_16
    _wal_database(path, settings + OLLAMA_SCHEMA, ollama_records(schema_version))


OLLAMA_CONFIG = (
    '{"integrations":{"claude":{"models":["gemma4:e4b"]},'
    '"claude-desktop":{"models":["gemma4:e4b"],"automode":true}},'
    '"last_model":"gemma4:e4b","last_selection":"claude"}'
)
OLLAMA_CD_PROFILE_REL = (
    "Library/Application Support/Claude-3p/configLibrary/00000000-0000-4000-8000-000000000114.json"
)
OLLAMA_CD_PROFILE = """{
  "autoModeEnabled": true,
  "chatTabEnabled": true,
  "coworkEgressAllowedHosts": [
    "*"
  ],
  "deploymentDisplayName": "Ollama",
  "disableDeploymentModeChooser": true,
  "disableEssentialTelemetry": true,
  "disableNonessentialTelemetry": true,
  "inferenceGatewayApiKey": "ollama",
  "inferenceGatewayAuthScheme": "bearer",
  "inferenceGatewayBaseUrl": "http://127.0.0.1:11435",
  "inferenceProvider": "gateway"
}
"""


def build_ollama(home: Path) -> None:
    write_ollama_database(home / OLLAMA_DATABASE_REL)
    write_ollama_database(home / OLLAMA_WINDOWS_DATABASE_REL, schema_version=16)
    ollama = home / ".ollama"
    ollama.mkdir(parents=True, exist_ok=True)
    (ollama / "history").write_text("why is the sky blue\n/set nohistory\n/bye\n", encoding="utf-8")
    # The launch configuration: the claude-desktop integration's automode key
    # is a standing permission setting (research/ollama.md,
    # Permission rows), the claude integration has none.
    (ollama / "config.json").write_text(OLLAMA_CONFIG, encoding="utf-8")
    # The Claude Desktop third-party profile the launcher writes, under a
    # claude-desktop catalog base (cmd/launch/claude_desktop.go at 6787222).
    profile = home / OLLAMA_CD_PROFILE_REL
    profile.parent.mkdir(parents=True, exist_ok=True)
    profile.write_text(OLLAMA_CD_PROFILE, encoding="utf-8")
    (profile.parent / "_meta.json").write_text(
        '{"appliedId":"00000000-0000-4000-8000-000000000114",'
        '"entries":[{"id":"00000000-0000-4000-8000-000000000114","name":"Ollama"}]}',
        encoding="utf-8",
    )
    # Noise the parser must not want: the configuration's backup, the
    # onboarding marker, the key pair, a server log and a model manifest,
    # which the collector collects once .ollama/models/blobs alone is excluded.
    (ollama / "backup").mkdir(exist_ok=True)
    (ollama / "backup/config.json.1772355600").write_text(
        '{"integrations":{},"last_selection":"claude"}', encoding="utf-8"
    )
    (ollama / "onboarding-v1.completed").write_text("", encoding="utf-8")
    (ollama / "id_ed25519").write_text("not a real key\n", encoding="utf-8")
    (ollama / "logs").mkdir(exist_ok=True)
    (ollama / "logs/server.log").write_text(
        "[GIN] 2026/03/01 - 09:00:00 | 200 |\n", encoding="utf-8"
    )
    manifest = home / OLLAMA_MANIFEST_REL
    manifest.parent.mkdir(parents=True, exist_ok=True)
    manifest.write_text(
        '{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json"}',
        encoding="utf-8",
    )
