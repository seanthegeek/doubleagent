"""Synthetic Cody state for the analyzer tests."""

from __future__ import annotations

import json
from pathlib import Path

from .common import _vscdb

# ---- VS Code state.vscdb rows (Cody, Twinny) and PearAI ----------------------------
# Shapes from research/cody.md, twinny.md and pearai.md.


CODY_ACCOUNT = "https://sourcegraph.com/-alice"
CODY_CHAT = "Sat, 03 Oct 2026 10:00:00 GMT"
CODY_AGENTIC_CHAT = "Sat, 03 Oct 2026 10:30:00 GMT"
CODY_VSCODE_CHAT = "Sat, 03 Oct 2026 09:00:00 GMT"
CODY_MODEL = "anthropic::2024-10-22::claude-sonnet-4-latest"
CODY_MODEL_SWITCHED = "anthropic::2024-10-22::claude-3-5-haiku-latest"
CODY_JB_REL = ".local/share/Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2"
CODY_VSCDB_REL = ".config/VSCodium/User/globalStorage/state.vscdb"
CODY_TOKEN = "sgp_cody_secret_not_real"


def cody_records():
    """The JetBrains AccountKeyedChatHistory: the research fixture chat and its agentic variant."""
    chat = {
        "id": CODY_CHAT,
        "chatTitle": "Fix flaky test",
        "lastInteractionTimestamp": CODY_CHAT,
        "interactions": [
            {
                "humanMessage": {
                    "speaker": "human",
                    "text": "why is test_login flaky?",
                    "intent": "chat",
                    "contextFiles": [
                        {
                            "type": "file",
                            "source": "user",
                            "uri": {
                                "$mid": 1,
                                "fsPath": "/srv/proj/tests/test_login.py",
                                "path": "/srv/proj/tests/test_login.py",
                                "scheme": "file",
                            },
                        }
                    ],
                },
                "assistantMessage": {
                    "speaker": "assistant",
                    "model": CODY_MODEL,
                    "text": "It depends on wall-clock time.",
                    "intent": "chat",
                },
            }
        ],
    }
    agentic = {
        "id": CODY_AGENTIC_CHAT,
        "chatTitle": "run the tests",
        "lastInteractionTimestamp": CODY_AGENTIC_CHAT,
        "interactions": [
            {
                "humanMessage": {"speaker": "human", "text": "run the tests", "intent": "agentic"},
                "assistantMessage": {
                    "speaker": "assistant",
                    "model": CODY_MODEL,
                    "text": "Running tests.",
                    "content": [
                        {"type": "text", "text": "Running tests."},
                        {
                            "type": "tool_call",
                            "tool_call": {
                                "id": "toolu_01",
                                "name": "run_terminal_command",
                                "arguments": '{"command":"pytest -q"}',
                            },
                        },
                    ],
                    "processes": [
                        {
                            "type": "confirmation",
                            "id": "toolu_01",
                            "title": "Run terminal command",
                            "description": "pytest -q",
                            "content": "Allow Cody to run this command?",
                            "state": "success",
                        },
                        {
                            "type": "tool",
                            "id": "toolu_01",
                            "title": "run_terminal_command",
                            "content": "pytest -q",
                            "state": "success",
                        },
                    ],
                },
            },
            {
                "humanMessage": {
                    "speaker": "human",
                    "text": "",
                    "content": [
                        {
                            "type": "tool_result",
                            "tool_result": {"id": "toolu_01", "content": "3 passed"},
                        }
                    ],
                },
                "assistantMessage": {
                    "speaker": "assistant",
                    "model": CODY_MODEL_SWITCHED,
                    "text": "All three pass.",
                    "error": {"name": "RateLimitError", "message": "rate limit exceeded"},
                },
            },
        ],
    }
    return {CODY_ACCOUNT: {"chat": {CODY_CHAT: chat, CODY_AGENTIC_CHAT: agentic}}}


def cody_vscode_state():
    """The `sourcegraph.cody-ai` global state value of a VS Code install."""
    return {
        "cody-local-chatHistory-v2": {
            CODY_ACCOUNT: {
                "chat": {
                    CODY_VSCODE_CHAT: {
                        "id": CODY_VSCODE_CHAT,
                        "lastInteractionTimestamp": CODY_VSCODE_CHAT,
                        "interactions": [
                            {
                                "humanMessage": {"speaker": "human", "text": "explain build.sh"},
                                "assistantMessage": {
                                    "speaker": "assistant",
                                    "model": CODY_MODEL,
                                    "text": "It runs make.",
                                },
                            }
                        ],
                    }
                }
            }
        },
        "cody-anonymous-user-id": "anon-1",
    }


def build_cody(home: Path) -> None:
    jetbrains_history = home / CODY_JB_REL
    jetbrains_history.parent.mkdir(parents=True, exist_ok=True)
    jetbrains_history.write_text(json.dumps(cody_records()), encoding="utf-8")
    (home / ".local/share/Cody-nodejs/user-settings.json").write_text(
        '{"cody.serverEndpoint": "x"}', encoding="utf-8"
    )
    _vscdb(
        home / CODY_VSCDB_REL,
        {
            "sourcegraph.cody-ai": cody_vscode_state(),
            'secret://{"extensionId":"sourcegraph.cody-ai","key":"cody.access-token"}': CODY_TOKEN,
        },
    )


# Deep Cody terminal confirmations, from sourcegraph/cody-public-snapshot
# at 8e20ac6c1460c08b0db581c0204658112a246eda: `CodyTool.ts:166-181` adds one
# `confirmation` step per command (id `<tool tag>-<uuid>`, title the tool's title,
# content the command); `ProcessManager.ts:51-61` completes a refused one with
# `errorToChatError(new Error('Command rejected'))` (`messages.ts:253-260`), and
# an approved one is never completed, so it stays `pending` (`DeepCody.ts:176`).
# The steps are saved on the human message (`ChatController.ts:943-946`). The
# file sits in the plugin's `Cody-nodejs/Data/` layout so `build_home` is unchanged.
CODY_DEEP_REL = ".local/share/Cody-nodejs/Data/JetBrains-globalState/cody-local-chatHistory-v2"
CODY_DEEP_CHAT = "Sat, 03 Oct 2026 11:00:00 GMT"
CODY_REJECTED_STEP = "TOOLCLI-0b5c1e2a-1111-4000-8000-000000000001"
CODY_PENDING_STEP = "TOOLCLI-0b5c1e2a-1111-4000-8000-000000000002"


def cody_deep_records():
    """A Deep Cody chat with one refused and one approved terminal command."""
    chat = {
        "id": CODY_DEEP_CHAT,
        "chatTitle": "what changed?",
        "lastInteractionTimestamp": CODY_DEEP_CHAT,
        "interactions": [
            {
                "humanMessage": {
                    "speaker": "human",
                    "text": "what changed?",
                    "intent": "chat",
                    "processes": [
                        {
                            "type": "confirmation",
                            "id": CODY_REJECTED_STEP,
                            "title": "Terminal",
                            "content": "rm -rf build",
                            "state": "error",
                            "error": {
                                "isChatErrorGuard": "isChatErrorGuard",
                                "message": "Command rejected",
                                "name": "Error",
                            },
                        },
                        {
                            "type": "confirmation",
                            "id": CODY_PENDING_STEP,
                            "title": "Terminal",
                            "content": "ls -la",
                            "state": "pending",
                        },
                        {
                            "type": "tool",
                            "id": "TOOLCLI",
                            "title": "Terminal",
                            "content": "ls -la",
                            "state": "success",
                        },
                    ],
                },
                "assistantMessage": {
                    "speaker": "assistant",
                    "model": CODY_MODEL,
                    "text": "Only README.md changed.",
                },
            }
        ],
    }
    return {CODY_ACCOUNT: {"chat": {CODY_DEEP_CHAT: chat}}}


# Settings, model preferences and the legacy JetBrains history,
# from sourcegraph/cody-public-snapshot at 8e20ac6c1460c08b0db581c0204658112a246eda:
# the settings keys from `vscode/package.json:1136-1140` and `1176-1213` and the MCP
# server schema `MCPManager.ts:20-45`; `cody-model-preferences` from
# `LocalStorageProvider.ts:37` and `modelsService.ts:198-219`; the XML from the
# serialisation `HistoryStateTest.kt:37-67`, with `llm` (`ChatState.kt:24`,
# `LLMState.kt:8-18`) and the component wrapper the IDE writes around it.
CODY_SETTINGS_REL = ".config/VSCodium/User/settings.json"
CODY_PREFS_REL = ".local/share/Cody-nodejs/JetBrains-globalState/cody-model-preferences"
CODY_MCP_TOKEN = "ghp_cody_mcp_secret_not_real"
CODY_LEGACY_REL = ".idea/cody_history.xml"
CODY_LEGACY_ACCOUNT = "VXNlkjoxEFU3NjE="
CODY_LEGACY_UPDATED = "2024-01-31T01:06:18.524621"
CODY_LEGACY_ID = "0f8b7034-9fa8-488a-a13e-09c52677008a"

CODY_SETTINGS_TEXT = (
    "{\n"
    "  // editor settings with comments and a trailing comma\n"
    '  "editor.fontSize": 14,\n'
    '  "cody.chat.agenticContext": false,\n'
    '  "cody.agentic.context.experimentalShell": true,\n'
    '  "cody.agentic.context.experimentalOptions": {\n'
    '    "shell": {"allow": ["git", "ls"], "block": ["sudo"]}\n'
    "  },\n"
    '  "cody.mcpServers": {\n'
    '    "github": {"command": "npx", "args": ["gh-mcp"],\n'
    '      "env": {"GITHUB_TOKEN": "' + CODY_MCP_TOKEN + '"},\n'
    '      "disabledTools": ["delete_repo"]},\n'
    '    "web": {"url": "https://mcp.example/sse", "disabled": true,\n'
    '      "headers": {"Authorization": "Bearer ' + CODY_MCP_TOKEN + '"}},\n'
    "  },\n"
    "}\n"
)


def cody_model_preferences():
    return {
        "https://sourcegraph.com/": {
            "defaults": {"chat": CODY_MODEL, "edit": CODY_MODEL},
            "selected": {"chat": CODY_MODEL_SWITCHED},
        }
    }


CODY_LEGACY_XML = """<project version="4">
  <component name="ChatHistory">
    <accountData>
      <list>
        <AccountData>
          <accountId value="%(account)s" />
          <chats>
            <list>
              <chat>
                <internalId value="%(id)s" />
                <llm>
                  <llm>
                    <model value="%(model)s" />
                    <provider value="Anthropic" />
                  </llm>
                </llm>
                <messages>
                  <list>
                    <message>
                      <speaker value="HUMAN" />
                      <text value="hi&#10;there" />
                    </message>
                    <message>
                      <speaker value="ASSISTANT" />
                      <text value="hello" />
                    </message>
                    <message>
                      <speaker value="HUMAN" />
                      <text value="unpaired question" />
                    </message>
                  </list>
                </messages>
                <updatedAt value="%(updated)s" />
              </chat>
              <chat>
                <internalId value="1f8b7034-9fa8-488a-a13e-09c52677008b" />
                <messages>
                  <list>
                    <message>
                      <speaker value="HUMAN" />
                      <text value="second chat" />
                    </message>
                  </list>
                </messages>
                <updatedAt value="2024-02-01T09:00:00.000001" />
              </chat>
            </list>
          </chats>
        </AccountData>
      </list>
    </accountData>
  </component>
</project>
""" % {
    "account": CODY_LEGACY_ACCOUNT,
    "id": CODY_LEGACY_ID,
    "model": CODY_MODEL,
    "updated": CODY_LEGACY_UPDATED,
}


def build_cody_settings(home: Path) -> None:
    """Write the Cody editor settings and model preferences.

    Write the editor settings, the JetBrains model preferences and the VS Code row's preferences;
    not part of `build_home`, a test adds them. A keybindings file beside settings.json is noise the
    parser skips.
    """
    path = home / CODY_SETTINGS_REL
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(CODY_SETTINGS_TEXT, encoding="utf-8")
    (path.parent / "keybindings.json").write_text("[]\n", encoding="utf-8")
    prefs = home / CODY_PREFS_REL
    prefs.parent.mkdir(parents=True, exist_ok=True)
    prefs.write_text(json.dumps(cody_model_preferences()), encoding="utf-8")
    (prefs.parent / "cody-anonymous-user-id").write_text('"anon-1"', encoding="utf-8")
    state = cody_vscode_state()
    state["cody-model-preferences"] = cody_model_preferences()
    _vscdb(home / CODY_VSCDB_REL, {"sourcegraph.cody-ai": state})


def build_cody_deep(home: Path) -> None:
    """Write the Deep Cody history; not part of `build_home`, a test adds it."""
    path = home / CODY_DEEP_REL
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(cody_deep_records()), encoding="utf-8")
