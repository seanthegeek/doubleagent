"""Synthetic Tabby state for the analyzer tests."""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path

from .common import _json, _jsonl, _vscdb, _wal_database

# Tabby: ee/tabby-db/schema/schema.sql at 21b2904 (research/tabby.md, Record schema),
# trimmed to the tables the parser reads plus every table that holds a secret,
# so the tests can prove those are never read.
TABBY_SCHEMA = """
CREATE TABLE registration_token(id INTEGER PRIMARY KEY AUTOINCREMENT, token VARCHAR(255) NOT NULL,
  created_at TIMESTAMP DEFAULT(DATETIME('now')), updated_at TIMESTAMP DEFAULT(DATETIME('now')));
CREATE TABLE users(id INTEGER PRIMARY KEY AUTOINCREMENT, email VARCHAR(150) NOT NULL COLLATE NOCASE,
  is_admin BOOLEAN NOT NULL DEFAULT 0, created_at TIMESTAMP DEFAULT(DATETIME('now')),
  updated_at TIMESTAMP DEFAULT(DATETIME('now')), auth_token VARCHAR(128) NOT NULL, active BOOLEAN NOT NULL DEFAULT 1,
  password_encrypted VARCHAR(128), avatar BLOB DEFAULT NULL, name VARCHAR(255));
CREATE TABLE refresh_tokens(id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL,
  token VARCHAR(255) NOT NULL COLLATE NOCASE, expires_at TIMESTAMP NOT NULL, created_at TIMESTAMP DEFAULT(DATETIME('now')));
CREATE TABLE integrations(id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, kind TEXT NOT NULL, display_name TEXT NOT NULL,
  access_token TEXT NOT NULL, api_base TEXT, error TEXT, created_at TIMESTAMP NOT NULL DEFAULT(DATETIME('now')),
  updated_at TIMESTAMP NOT NULL DEFAULT(DATETIME('now')), synced BOOLEAN NOT NULL DEFAULT FALSE);
CREATE TABLE email_setting(id INTEGER PRIMARY KEY AUTOINCREMENT, smtp_username VARCHAR(255) NOT NULL,
  smtp_password VARCHAR(255) NOT NULL, smtp_server VARCHAR(255) NOT NULL, from_address VARCHAR(255) NOT NULL,
  encryption VARCHAR(255) NOT NULL DEFAULT 'ssltls', auth_method VARCHAR(255) NOT NULL DEFAULT 'plain',
  smtp_port INTEGER NOT NULL DEFAULT 25);
CREATE TABLE oauth_credential(id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, provider TEXT NOT NULL,
  client_id VARCHAR(256) NOT NULL, client_secret VARCHAR(64) NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT(DATETIME('now')),
  updated_at TIMESTAMP NOT NULL DEFAULT(DATETIME('now')), config_url VARCHAR(256), config_scopes VARCHAR(256));
CREATE TABLE ldap_credential(id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, host STRING NOT NULL,
  port INTEGER NOT NULL DEFAULT 389, bind_dn STRING NOT NULL, bind_password STRING NOT NULL, base_dn STRING NOT NULL,
  user_filter STRING NOT NULL, encryption STRING NOT NULL DEFAULT 'none', skip_tls_verify BOOLEAN NOT NULL DEFAULT FALSE,
  email_attribute STRING NOT NULL DEFAULT 'email', name_attribute STRING,
  created_at TIMESTAMP NOT NULL DEFAULT(DATETIME('now')), updated_at TIMESTAMP NOT NULL DEFAULT(DATETIME('now')));
CREATE TABLE user_events(id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, kind TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT(DATETIME('now')), payload BLOB NOT NULL);
CREATE TABLE threads(id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, is_ephemeral BOOLEAN NOT NULL, user_id INTEGER NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT(DATETIME('now')), updated_at TIMESTAMP NOT NULL DEFAULT(DATETIME('now')),
  relevant_questions BLOB);
CREATE TABLE thread_messages(id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, thread_id INTEGER NOT NULL, role TEXT NOT NULL,
  content TEXT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT(DATETIME('now')),
  updated_at TIMESTAMP NOT NULL DEFAULT(DATETIME('now')), code_source_id VARCHAR(255), attachment BLOB NOT NULL DEFAULT '{}');
CREATE TABLE provided_repositories(id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, integration_id INTEGER NOT NULL,
  vendor_id TEXT NOT NULL, name TEXT NOT NULL, git_url TEXT NOT NULL, active BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT(DATETIME('now')), updated_at TIMESTAMP NOT NULL DEFAULT(DATETIME('now')), refs TEXT);
CREATE TABLE pages(id integer PRIMARY KEY AUTOINCREMENT NOT NULL, author_id integer NOT NULL, title text, content text,
  created_at timestamp NOT NULL DEFAULT(DATETIME('now')), updated_at timestamp NOT NULL DEFAULT(DATETIME('now')),
  code_source_id VARCHAR(255));
CREATE TABLE page_sections(id integer PRIMARY KEY AUTOINCREMENT NOT NULL, page_id integer NOT NULL, title text NOT NULL,
  content text, "position" integer NOT NULL, created_at timestamp NOT NULL DEFAULT(DATETIME('now')),
  updated_at timestamp NOT NULL DEFAULT(DATETIME('now')), attachment BLOB NOT NULL DEFAULT '{}');
"""

# One distinct value per plaintext secret column; none may reach any output.
TABBY_SECRETS = (
    "auth_REDACT_tok",
    "pw_hash_REDACT",
    "regtok_REDACT",
    "refresh_REDACT",
    "ghp_REDACT_integration",
    "smtp_REDACT_pw",
    "oauth_REDACT_secret",
    "ldap_REDACT_bind",
)


def tabby_secret_records():
    secrets = TABBY_SECRETS
    return [
        "INSERT INTO users(id,email,is_admin,auth_token,active,name,password_encrypted) "
        "VALUES(1,'alice@example.com',1,'%s',1,'Alice','%s')" % (secrets[0], secrets[1]),
        "INSERT INTO registration_token(id,token) VALUES(1,'%s')" % secrets[2],
        "INSERT INTO refresh_tokens(user_id,token,expires_at) VALUES(1,'%s','2026-12-01 00:00:00')"
        % secrets[3],
        "INSERT INTO integrations(id,kind,display_name,access_token,created_at,updated_at) "
        "VALUES(1,'github','acme','%s','2026-09-01 09:00:00','2026-09-01 09:00:00')" % secrets[4],
        "INSERT INTO email_setting(smtp_username,smtp_password,smtp_server,from_address) "
        "VALUES('mailer','%s','smtp.example.com','tabby@example.com')" % secrets[5],
        "INSERT INTO oauth_credential(provider,client_id,client_secret) VALUES('github','cid','%s')"
        % secrets[6],
        "INSERT INTO ldap_credential(host,bind_dn,bind_password,base_dn,user_filter) "
        "VALUES('ldap.example.com','cn=admin','%s','dc=example','(uid=%%s)')" % secrets[7],
    ]


TABBY_SELECT_PAYLOAD = (
    '{\n  "select": {\n    "completion_id": "cmpl-7f3a",\n    "choice_index": 0,\n'
    '    "view_id": "view-1",\n    "elapsed": 1377\n  }\n}'
)


def tabby_records():
    """The INSERT statements of research/tabby.md, Parser plan, with one token each.

    The INSERT statements of research/tabby.md, Parser plan, with the research's single fixture
    token replaced by one per secret column, and a user_events row whose payload is the pretty JSON
    event_logger.rs writes.
    """
    return [
        *tabby_secret_records(),
        "INSERT INTO threads(id,is_ephemeral,user_id,created_at,updated_at,relevant_questions) "
        """VALUES(7,0,1,'2026-10-01 10:00:00','2026-10-01 10:00:09','["How is the cache invalidated?"]')""",
        "INSERT INTO thread_messages(id,thread_id,role,content,created_at,updated_at,attachment) VALUES "
        """(1,7,'user','Where is the cache cleared?','2026-10-01 10:00:00','2026-10-01 10:00:00','{"code":null,"client_code":[{"filepath":"src/cache.rs","start_line":10,"content":"fn clear()"}],"doc":null}'),"""
        """(2,7,'assistant','In `clear()` in src/cache.rs.','2026-10-01 10:00:02','2026-10-01 10:00:09','{"code":[{"git_url":"https://github.com/acme/app","commit":"abc123","language":"rust","filepath":"src/cache.rs","content":"fn clear() {}","start_line":10}],"client_code":null,"doc":null}')""",
        "INSERT INTO user_events(id,user_id,kind,created_at,payload) VALUES(1,1,'select','2026-10-01 10:00:01',"
        "'%s')" % TABBY_SELECT_PAYLOAD,
        # The event log's completion line as the database logger stores it
        # (event_logger.rs:79-109): `ts` truncated to the second.
        "INSERT INTO user_events(id,user_id,kind,created_at,payload) "
        "VALUES(2,1,'completion','2025-10-01 10:00:00','%s')" % TABBY_COMPLETION_PAYLOAD,
        # An AI-written page made from thread 7 (page.rs:92-133; schema.sql:267-288).
        "INSERT INTO pages(id,author_id,title,content,created_at,updated_at,code_source_id) "
        "VALUES(4,1,'Cache invalidation','How the cache is cleared.','2026-10-01 11:00:00',"
        "'2026-10-01 11:00:30','acme-app')",
        "INSERT INTO page_sections(id,page_id,title,content,position,created_at,attachment) VALUES "
        """(9,4,'Clearing','Call `clear()`.',0,'2026-10-01 11:00:10','{"code":[{"git_url":"https://github.com/acme/app","filepath":"src/cache.rs","start_line":10}]}'),"""
        "(10,4,'Expiry','Entries expire after an hour.',1,'2026-10-01 11:00:20','{}')",
        # The repository list cached from integration 1 (third_party.rs:278-300).
        "INSERT INTO provided_repositories(id,integration_id,vendor_id,name,git_url,active,"
        "created_at,updated_at) VALUES(3,1,'101','acme/app','https://github.com/acme/app',1,"
        "'2026-09-01 09:05:00','2026-09-02 09:05:00')",
    ]


# A pre-0.25 backup: deprecated per-kind attachment columns, no `attachment`,
# and an ephemeral thread the live database has since deleted.
TABBY_BACKUP_SCHEMA = TABBY_SCHEMA.replace(
    "code_source_id VARCHAR(255), attachment BLOB NOT NULL DEFAULT '{}'",
    "code_source_id VARCHAR(255), code_attachments BLOB, client_code_attachments BLOB, doc_attachments BLOB",
)
TABBY_REL = ".tabby/ee/db.sqlite"
TABBY_BACKUP_REL = ".tabby/ee/db.backup-20260915.sqlite"
TABBY_BACKUP2_REL = ".tabby/ee/db.backup-20261005.sqlite"
TABBY_EVENTS_REL = ".tabby/events/2025-10-01.json"


def tabby_backup_records():
    return [
        *tabby_secret_records(),
        "INSERT INTO threads(id,is_ephemeral,user_id,created_at,updated_at) "
        "VALUES(3,1,1,'2026-09-10 08:00:00','2026-09-10 08:00:04')",
        "INSERT INTO thread_messages(id,thread_id,role,content,created_at,updated_at,code_attachments) VALUES "
        "(1,3,'user','dump the users table','2026-09-10 08:00:00','2026-09-10 08:00:00',NULL),"
        """(2,3,'assistant','Run `select * from users`.','2026-09-10 08:00:04','2026-09-10 08:00:04','[{"git_url":"https://github.com/acme/ops","filepath":"db.sql","content":"select 1","language":"sql","start_line":null}]')""",
    ]


def tabby_event_records():
    """The event log lines of research/tabby.md, Parser plan, and one more.

    The two event log lines of research/tabby.md, Parser plan, plus a chat_completion event, whose
    body is empty (routes/chat.rs:83).
    """
    return [
        {
            "user": "1",
            "ts": 1759312800123,
            "event": {
                "completion": {
                    "completion_id": "cmpl-7f3a",
                    "language": "python",
                    "prompt": "def add(a, b):\n    ",
                    "segments": {
                        "prefix": "def add(a, b):\n    ",
                        "suffix": "\n",
                        "git_url": "https://github.com/acme/app",
                        "filepath": "app/math.py",
                    },
                    "choices": [{"index": 0, "text": "return a + b"}],
                    "user_agent": "tabby-agent/1.9",
                }
            },
        },
        {
            "user": "1",
            "ts": 1759312801500,
            "event": {
                "select": {
                    "completion_id": "cmpl-7f3a",
                    "choice_index": 0,
                    "view_id": "view-1",
                    "elapsed": 1377,
                }
            },
        },
        {"user": None, "ts": 1759312802000, "event": {"chat_completion": {}}},
    ]


TABBY_COMPLETION_PAYLOAD = json.dumps(tabby_event_records()[0]["event"], indent=2)


def tabby_backup2_records():
    """A later backup in the current schema.

    The ephemeral thread of the first backup, copied again, and the live database's thread 7.
    """
    count = len(tabby_secret_records())
    return [
        *tabby_secret_records(),
        "INSERT INTO threads(id,is_ephemeral,user_id,created_at,updated_at) "
        "VALUES(3,1,1,'2026-09-10 08:00:00','2026-09-10 08:00:04')",
        "INSERT INTO thread_messages(id,thread_id,role,content,created_at,updated_at,attachment) VALUES "
        """(5,3,'user','dump the users table','2026-09-10 08:00:00','2026-09-10 08:00:00','{}'),"""
        """(6,3,'assistant','Run `select * from users`.','2026-09-10 08:00:04','2026-09-10 08:00:04','{"code":[{"git_url":"https://github.com/acme/ops","filepath":"db.sql","content":"select 1","language":"sql","start_line":null}]}')""",
        # Thread 7 and its messages, as the live database holds them.
        *tabby_records()[count : count + 2],
    ]


TABBY_CONFIG_REL = ".tabby/config.toml"
# config.rs:259-342: an http chat model with its key, and a local completion model.
TABBY_CONFIG = """[model.chat.http]
kind = "openai/chat"
model_name = "gpt-4o"
api_endpoint = "https://api.openai.com/v1"
api_key = "sk-tabby-test"
supported_models = ["gpt-4o", "gpt-4o-mini"]

[model.completion.local]
model_id = "StarCoder-1B"

[[repositories]]
git_url = "file:///srv/proj"
"""
TABBY_CLIENT_TOKEN = "auth_client_REDACT"
TABBY_ENDPOINT_TOKEN = "auth_vscode_REDACT"
TABBY_INTELLIJ_REL = (
    ".config/JetBrains/IntelliJIdea2025.2/options/intellij-tabby-command-history.xml"
)
# CommandHistory.kt:5-31: the `history` list of the CommandHistory component,
# newest first, in the IntelliJ platform's list serialisation.
TABBY_INTELLIJ_HISTORY = """<application>
  <component name="com.tabbyml.intellijtabby.inlineChat.CommandHistory">
    <option name="history">
      <list>
        <option value="add docstrings" />
        <option value="convert to async" />
      </list>
    </option>
  </component>
</application>
"""
TABBY_VSCDB_REL = "AppData/Roaming/Code/User/globalStorage/state.vscdb"


def tabby_vscode_state() -> dict:
    """The extension's global state (Config.ts:41-66, 139-145).

    Server records with an endpoint token, and `edit.recentlyCommand`, newest first, each entry
    `JSON.stringify({command, context})` (quickPick.ts:344-351), the last one a bare string.
    """
    return {
        "server.serverRecords": {"http://localhost:8080": {"token": TABBY_ENDPOINT_TOKEN}},
        "edit.recentlyCommand": [
            json.dumps(
                {
                    "command": "rename @file to snake case",
                    "context": [
                        {
                            "referrer": "@file",
                            "uri": "file:///srv/proj/app/math.py",
                            "range": {
                                "start": {"line": 0, "character": 0},
                                "end": {"line": 3, "character": 0},
                            },
                        }
                    ],
                }
            ),
            json.dumps({"command": "add type hints"}),
            "fix the typo",
        ],
    }


def build_tabby(home: Path) -> None:
    """A Tabby server's home.

    The live WAL database, a pre-migration backup and a later one holding copies of the same
    threads, one day of the event log, and the config file as noise.
    """
    _wal_database(home / TABBY_REL, TABBY_SCHEMA, tabby_records())
    for rel, schema, records in (
        (TABBY_BACKUP_REL, TABBY_BACKUP_SCHEMA, tabby_backup_records()),
        (TABBY_BACKUP2_REL, TABBY_SCHEMA, tabby_backup2_records()),
    ):
        connection = sqlite3.connect(str(home / rel))
        connection.executescript(schema)
        for sql in records:
            connection.execute(sql)
        connection.commit()
        connection.close()
    _jsonl(home / TABBY_EVENTS_REL, tabby_event_records())
    (home / TABBY_CONFIG_REL).write_text(TABBY_CONFIG, encoding="utf-8")
    # The client's config holds the user's token: noise the parser must not want.
    client = home / ".tabby-client/agent/config.toml"
    client.parent.mkdir(parents=True, exist_ok=True)
    client.write_text('[server]\ntoken = "%s"\n' % TABBY_CLIENT_TOKEN, encoding="utf-8")
    intellij_history = home / TABBY_INTELLIJ_REL
    intellij_history.parent.mkdir(parents=True, exist_ok=True)
    intellij_history.write_text(TABBY_INTELLIJ_HISTORY, encoding="utf-8")
    (intellij_history.parent / "intellij-tabby.xml").write_text(
        '<application><component name="com.tabbyml.intellijtabby.settings.SettingsService">'
        '<option name="serverEndpoint" value="http://localhost:8080" /></component></application>',
        encoding="utf-8",
    )
    _vscdb(home / TABBY_VSCDB_REL, {"TabbyML.vscode-tabby": tabby_vscode_state()})


# A Tabby server root in a Docker named volume: the catalog's
# `tabby|*tabby*` DOCKER_VOLUMES line collects the volume whole with its
# `_data` directory as the base (user `docker`), and the images set
# TABBY_ROOT=/data (docker/Dockerfile.cuda:88 at 21b2904), so a volume
# mounted there holds the server root itself: `ee/db.sqlite` at the base.
TABBY_VOLUME_HOME = "/var/lib/docker/volumes/tabby_data/_data"
TABBY_VOLUME_FILES = (
    "ee/db.sqlite",
    "ee/db.sqlite-wal",
    "ee/db.sqlite-shm",
    "ee/db.backup-20260915.sqlite",
    "events/2025-10-01.json",
    "config.toml",
    "usage_anonymous_id",
)


def build_tabby_volume(root: Path) -> None:
    """Write a collector output directory holding only the Tabby volume.

    A collector output directory (collection.json, manifest.jsonl and fs/) holding only the Tabby
    volume: the main fixture's server root with `.tabby/` stripped, and the telemetry id as noise.
    """
    import shutil

    base = root / ("fs" + TABBY_VOLUME_HOME)
    home = root / "build"
    build_tabby(home)
    for rel in TABBY_VOLUME_FILES:
        source = home / ".tabby" / rel
        dst = base / rel
        dst.parent.mkdir(parents=True, exist_ok=True)
        if source.exists():
            dst.write_bytes(source.read_bytes())
        else:
            dst.write_text("6f1c2a3b-0000-4000-8000-000000000001\n", encoding="utf-8")
    shutil.rmtree(home)
    _json(root / "collection.json", {"hostname": "h2"})
    _jsonl(
        root / "manifest.jsonl",
        [
            {
                "user": "docker",
                "home": TABBY_VOLUME_HOME,
                "agent": "tabby",
                "path": TABBY_VOLUME_HOME + "/" + rel,
                "archive_path": "fs" + TABBY_VOLUME_HOME + "/" + rel,
                "type": "file",
                "status": "collected",
                "secret": rel == "config.toml",
            }
            for rel in TABBY_VOLUME_FILES
        ],
    )
