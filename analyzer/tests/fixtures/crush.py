"""Synthetic Crush state for the analyzer tests."""

from __future__ import annotations

import json
from pathlib import Path

from .common import _wal_database

CRUSH_SESSION = "6f1c0001"

# Columns of internal/db/migrations at crush ca6ae26 that the parser reads.
CRUSH_SCHEMA = """
CREATE TABLE sessions (id TEXT PRIMARY KEY, parent_session_id TEXT, title TEXT NOT NULL,
  message_count INTEGER NOT NULL DEFAULT 0, prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0, cost REAL NOT NULL DEFAULT 0.0,
  updated_at INTEGER NOT NULL, created_at INTEGER NOT NULL, summary_message_id TEXT, todos TEXT, channel TEXT);
CREATE TABLE messages (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, role TEXT NOT NULL, parts TEXT NOT NULL DEFAULT '[]',
  model TEXT, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, finished_at INTEGER, provider TEXT,
  is_summary_message INTEGER DEFAULT 0 NOT NULL, prism_model_id TEXT, prism_model_name TEXT);
CREATE TABLE files (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, path TEXT NOT NULL, content TEXT NOT NULL,
  version INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE read_files (session_id TEXT NOT NULL, path TEXT NOT NULL, read_at INTEGER NOT NULL,
  PRIMARY KEY (path, session_id));
CREATE TABLE mcp_disabled_servers (name TEXT PRIMARY KEY);
CREATE TABLE mcp_enabled_servers (name TEXT PRIMARY KEY);
"""


def crush_records():
    """The INSERT statements of research/crush.md, Parser plan, and a model change.

    The INSERT statements of research/crush.md, Parser plan, plus a final assistant message with
    reasoning, text and an abnormal finish, answered by another provider's model so the session has
    a model change.
    """
    return [
        "INSERT INTO sessions(id,title,message_count,prompt_tokens,completion_tokens,cost,updated_at,created_at) "
        "VALUES('6f1c0001','Fix bug',3,10,5,0.01,1760000005,1760000001)",
        "INSERT INTO messages(id,session_id,role,parts,model,provider,created_at,updated_at,finished_at,is_summary_message) VALUES "
        """('m1','6f1c0001','user','[{"type":"text","data":{"text":"list files"}},{"type":"finish","data":{"reason":"stop","time":0}}]','claude-sonnet-4','anthropic',1760000001,1760000001,NULL,0),"""
        """('m2','6f1c0001','assistant','[{"type":"tool_call","data":{"id":"call_x1","name":"bash","input":"{\\"command\\":\\"ls\\"}","provider_executed":false,"finished":true}},{"type":"finish","data":{"reason":"tool_use","time":1760000003}}]','claude-sonnet-4','anthropic',1760000002,1760000003,1760000003,0),"""
        """('m3','6f1c0001','tool','[{"type":"tool_result","data":{"tool_call_id":"call_x1","name":"bash","content":"a.txt","data":"","mime_type":"","metadata":"","is_error":false}},{"type":"finish","data":{"reason":"stop","time":0}}]','','',1760000003,1760000003,NULL,0)""",
        "INSERT INTO messages(id,session_id,role,parts,model,provider,created_at,updated_at,finished_at,is_summary_message) VALUES "
        """('m4','6f1c0001','assistant','[{"type":"reasoning","data":{"thinking":"one file only","signature":"sig","started_at":1760000004,"finished_at":1760000004}},{"type":"text","data":{"text":"There is one file, a.txt."}},{"type":"finish","data":{"reason":"max_tokens","time":1760000005}}]','gpt-5','openai',1760000004,1760000005,1760000005,0)""",
    ]


def build_crush(home: Path) -> None:
    """Write Crush's per-project store, project registry and config noise.

    Crush run in the home directory, so its per-project store is ~/.crush/crush.db, plus the global
    project registry and config noise.
    """
    _wal_database(home / ".crush/crush.db", CRUSH_SCHEMA, crush_records())
    (home / ".crush/crush.json").write_text(
        '{"providers":{"anthropic":{"api_key":"sk-test"}}}', encoding="utf-8"
    )
    reg = home / ".local/share/crush"
    reg.mkdir(parents=True, exist_ok=True)
    (reg / "projects.json").write_text(
        json.dumps(
            {
                "projects": [
                    {
                        "path": "/srv/proj",
                        "data_dir": "/srv/proj/.crush",
                        "last_accessed": "2026-10-01T12:00:00Z",
                    }
                ]
            }
        ),
        encoding="utf-8",
    )
    (home / ".config/crush").mkdir(parents=True, exist_ok=True)
    (home / ".config/crush/crush.json").write_text("{}", encoding="utf-8")
    # The data-directory lock (internal/db/datadirlock.go at 7f799fb): noise.
    (home / ".crush/crush.lock").write_text(
        '{"pid":4242,"version":"v0.80.0","started_at":"2026-10-01T12:00:00Z"}', encoding="utf-8"
    )


CRUSH_HYPER_SESSION = "6f1c0002"


def crush_hyper_records():
    """A session routed through Hyper's Prism router.

    The assistant messages request `prism-auto` from provider `hyper` and record the model that
    served each step in `prism_model_id`, the column of migration 20260902000000 at crush 7f799fb
    (internal/agent/agent.go 1085 and 1914-1929). The user message stores no model, as
    createUserMessage writes it. The third step has no Prism headers, so its `prism_model_id` is
    NULL. The requested id `prism-auto` is invented; no real install has shown a Hyper turn. The
    Crush tests apply these on top of `crush_records()` so the default session's rows stay as they
    are.
    """
    cols = (
        "id,session_id,role,parts,model,provider,created_at,updated_at,finished_at,"
        "prism_model_id,prism_model_name"
    )
    return [
        "INSERT INTO sessions(id,title,message_count,prompt_tokens,completion_tokens,cost,updated_at,created_at) "
        "VALUES('6f1c0002','Hyper',4,20,8,0,1760000110,1760000100)",
        "INSERT INTO messages(%s) VALUES "
        % cols
        + """('h1','6f1c0002','user','[{"type":"text","data":{"text":"route this"}}]',NULL,NULL,1760000100,1760000100,NULL,NULL,NULL),"""
        """('h2','6f1c0002','assistant','[{"type":"text","data":{"text":"first step"}},{"type":"finish","data":{"reason":"end_turn","time":1760000102}}]','prism-auto','hyper',1760000101,1760000102,1760000102,'claude-sonnet-4-5','Claude Sonnet 4.5'),"""
        """('h3','6f1c0002','assistant','[{"type":"text","data":{"text":"second step"}},{"type":"finish","data":{"reason":"end_turn","time":1760000104}}]','prism-auto','hyper',1760000103,1760000104,1760000104,'gpt-5-mini','GPT-5 mini'),"""
        """('h4','6f1c0002','assistant','[{"type":"text","data":{"text":"third step"}},{"type":"finish","data":{"reason":"end_turn","time":1760000106}}]','prism-auto','hyper',1760000105,1760000106,1760000106,NULL,NULL)""",
    ]


CRUSH_PERM_SESSION = "6f1c0003"
CRUSH_CHANNEL_SESSION = "6f1c0004"


def crush_permission_records():
    """Records for the Crush permission, relay, deletion and file rows.

    From research/crush.md at crush 7f799fb: a user denial (internal/agent/tools/tools.go 78-84), a
    hook denial and a hook approval carrying `metadata.hook` (internal/agent/hooked_tool.go 62-140,
    internal/hooks/hooks.go 26-40), a session bound to an MCP channel with a pushed `<channel>` user
    message (internal/agent/tools/mcp/channel.go 124-150), a sub-agent session whose parent was
    deleted (session.go 116-168), a `files` snapshot, a `read_files` entry and per-project MCP
    overrides. Hook names, reasons and the channel are invented. The tests add these to
    `crush_records()`.
    """
    hook_deny = json.dumps(
        {
            "hook": {
                "hook_count": 1,
                "decision": "deny",
                "reason": "no rm",
                "hooks": [{"name": "guard", "matcher": "bash", "decision": "deny"}],
            }
        }
    )
    hook_allow = json.dumps(
        {
            "hook": {
                "hook_count": 2,
                "decision": "allow",
                "hooks": [
                    {"name": "lint", "decision": "none"},
                    {"name": "trust-view", "matcher": "view", "decision": "allow"},
                ],
            }
        }
    )

    def result(call, name, content, meta, error):
        return json.dumps(
            [
                {
                    "type": "tool_result",
                    "data": {
                        "tool_call_id": call,
                        "name": name,
                        "content": content,
                        "metadata": meta,
                        "is_error": error,
                    },
                }
            ]
        ).replace("'", "''")

    cols = "id,session_id,role,parts,model,provider,created_at,updated_at,finished_at"
    channel_text = (
        '<channel source="signal" group="g-77" sender="+15550100">deploy status?</channel>'
    )
    user_push = json.dumps([{"type": "text", "data": {"text": channel_text}}])
    return [
        "INSERT INTO sessions(id,title,message_count,prompt_tokens,completion_tokens,cost,updated_at,created_at) "
        "VALUES('6f1c0003','Perms',6,0,0,0,1760000210,1760000200)",
        "INSERT INTO sessions(id,title,message_count,prompt_tokens,completion_tokens,cost,updated_at,created_at,channel) "
        "VALUES('6f1c0004','Signal',2,0,0,0,1760000310,1760000300,'signal')",
        "INSERT INTO sessions(id,parent_session_id,title,message_count,prompt_tokens,completion_tokens,cost,updated_at,created_at) "
        "VALUES('m9$$call_t1','6f1c0099','Task',0,0,0,0,1760000410,1760000400)",
        "INSERT INTO messages(%s) VALUES " % cols
        + "('p1','6f1c0003','tool','%s','','',1760000201,1760000201,NULL),"
        % result("call_d1", "bash", "User denied permission", "", True)
        + "('p2','6f1c0003','tool','%s','','',1760000202,1760000202,NULL),"
        % result("call_d2", "bash", "Tool call blocked by hook. Reason: no rm", hook_deny, True)
        + "('p3','6f1c0003','tool','%s','','',1760000203,1760000203,NULL),"
        % result("call_d3", "view", "file contents", hook_allow, False)
        + "('p4','6f1c0003','tool','%s','','',1760000204,1760000204,NULL),"
        % result("call_d4", "bash", "Turn halted by hook. Reason: stop", "", True)
        + "('c1','6f1c0004','user','%s',NULL,NULL,1760000301,1760000301,NULL),"
        % user_push.replace("'", "''")
        + "('c2','6f1c0004','user','%s',NULL,NULL,1760000302,1760000302,NULL)"
        % user_push.replace("'", "''"),
        "INSERT INTO files(id,session_id,path,content,version,created_at,updated_at) "
        "VALUES('f1','6f1c0003','/srv/proj/a.txt','hello',1,1760000205,1760000205)",
        "INSERT INTO read_files(session_id,path,read_at) VALUES('6f1c0003','/srv/proj/b.txt',1760000206)",
        "INSERT INTO mcp_disabled_servers(name) VALUES('github')",
        "INSERT INTO mcp_enabled_servers(name) VALUES('docs')",
    ]


# A crush.json with every standing permission key the parser reads beside a
# provider key it must never print (internal/config/config.go 224-225,
# 431-433, 471, 817-826, 865-869 at crush 7f799fb).
CRUSH_CONFIG_JSON = {
    "providers": {"anthropic": {"api_key": "sk-crush-secret"}},
    "permissions": {"allowed_tools": ["view", "bash:ls"]},
    "options": {"disabled_tools": ["sourcegraph"]},
    "mcp": {"docs": {"type": "stdio", "disabled_tools": ["delete"], "enabled_tools": ["search"]}},
    "hooks": {"PreToolUse": [{"name": "guard", "matcher": "bash", "command": "./guard.sh"}]},
}

# A crushrc shell config (internal/shellconfig/permissions.go 10-78 at crush
# 7f799fb): one allow line with two tools, one deny line, and lines the
# parser must ignore.
CRUSHRC = """# crush shell config
export ANTHROPIC_API_KEY=sk-rc-secret
permissions allow view 'bash:ls'
  permissions deny sourcegraph  # hide it
permissions list
echo permissions allow nothing
"""
