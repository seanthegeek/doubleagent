"""Synthetic PearAI state for the analyzer tests."""

from __future__ import annotations

from pathlib import Path

from .common import _json, _jsonl, _vscdb
from .roo_code import roo_ui_records

PEARAI_SESSION = "9b1c2d3e-0000-4000-8000-000000000001"
PEARAI_SEARCH_SESSION = "9b1c2d3e-0000-4000-8000-000000000002"
PEARAI_TASK = "1791036000000"
PEARAI_UI_TASK = "1791039600000"
PEARAI_GS = ".config/PearAI/User/globalStorage/"
PEARAI_ROO = PEARAI_GS + "pearai.pearai-roo-cline/"


def pearai_session():
    return {
        "history": [
            {
                "message": {"role": "user", "content": "why does build.sh fail"},
                "contextItems": [
                    {
                        "content": "set -e\nmake all",
                        "name": "build.sh",
                        "description": "/srv/proj/build.sh",
                        "id": {"providerTitle": "file", "itemId": "build.sh"},
                    }
                ],
            },
            {
                "message": {"role": "assistant", "content": "The make target is missing."},
                "contextItems": [],
                "promptLogs": [
                    {
                        "completionOptions": {"model": "pearai_model"},
                        "prompt": "<user>why does build.sh fail",
                        "completion": "The make target is missing.",
                    }
                ],
            },
            {"message": {"role": "user", "content": "add the target"}, "contextItems": []},
            {
                "message": {"role": "assistant", "content": "Added `all:` to the Makefile."},
                "contextItems": [],
                "promptLogs": [
                    {
                        "completionOptions": {"model": "pearai_model_2"},
                        "prompt": "<user>add the target",
                        "completion": "Added `all:` to the Makefile.",
                    }
                ],
            },
        ],
        "perplexityHistory": [],
        "title": "why does build.sh fail",
        "sessionId": PEARAI_SESSION,
        "workspaceDirectory": "/srv/proj",
    }


def pearai_search_session():
    return {
        "history": [],
        "perplexityHistory": [
            {
                "message": {
                    "role": "user",
                    "content": [{"type": "text", "text": "latest make release"}],
                },
                "contextItems": [],
            },
            {
                "message": {"role": "assistant", "content": "GNU make 4.4.1."},
                "contextItems": [],
                "citations": [{"url": "https://www.gnu.org/software/make/", "title": "GNU Make"}],
            },
        ],
        "title": "latest make release",
        "sessionId": PEARAI_SEARCH_SESSION,
        "workspaceDirectory": "/srv/proj",
    }


def pearai_index():
    return [
        {
            "sessionId": PEARAI_SESSION,
            "title": "why does build.sh fail",
            "dateCreated": "1791032400000",
            "workspaceDirectory": "/srv/proj",
            "integrationType": "continue",
        },
        {
            "sessionId": PEARAI_SEARCH_SESSION,
            "title": "latest make release",
            "dateCreated": "1791032500000",
            "workspaceDirectory": "/srv/proj",
            "integrationType": "perplexity",
        },
    ]


def pearai_api_records(start=1791036000000):
    return [
        {
            "role": "user",
            "content": [
                {"type": "text", "text": "<task>\nrun the tests\n</task>"},
                {
                    "type": "text",
                    "text": "<environment_details>\n# Current Working Directory "
                    "(/srv/proj)\n</environment_details>",
                },
            ],
            "ts": start,
        },
        {
            "role": "assistant",
            "content": [
                {
                    "type": "text",
                    "text": "<thinking>use npm</thinking>\nI will run them.\n<execute_command>\n<command>npm test</command>\n"
                    "</execute_command>",
                }
            ],
            "ts": start + 3000,
        },
        {
            "role": "user",
            "content": [
                {"type": "text", "text": "[execute_command for 'npm test'] Result:"},
                {"type": "text", "text": "12 passing"},
            ],
            "ts": start + 9000,
        },
        {
            "role": "assistant",
            "content": [
                {
                    "type": "text",
                    "text": "<attempt_completion>\n<result>All 12 tests pass.</result>\n</attempt_completion>",
                }
            ],
            "ts": start + 10000,
        },
    ]


def pearai_permission_records(start):
    """Three Roo 3.15.3 approval exchanges of PearAI.

    Three Roo 3.15.3 approval exchanges (PearAI-Roo-Code at 0b6df736,
    src/core/prompts/responses.ts:8-14, src/core/Cline.ts:1342-1386): a denial with feedback, a bare
    denial and an approval with a note.
    """

    def call(timestamp, body):
        return {"role": "assistant", "content": [{"type": "text", "text": body}], "ts": timestamp}

    def result(timestamp, *texts):
        return {
            "role": "user",
            "content": [{"type": "text", "text": text} for text in texts],
            "ts": timestamp,
        }

    return [
        call(
            start,
            "<write_to_file>\n<path>build/out.txt</path>\n<content>x</content>\n</write_to_file>",
        ),
        result(
            start + 2000,
            "[write_to_file for 'build/out.txt'] Result:",
            "The user denied this operation and provided the following feedback:\n"
            "<feedback>\nnot in build/\n</feedback>",
        ),
        call(
            start + 3000, "<execute_command>\n<command>rm -rf build</command>\n</execute_command>"
        ),
        result(
            start + 5000,
            "[execute_command for 'rm -rf build'] Result:",
            "The user denied this operation.",
        ),
        call(start + 6000, "<execute_command>\n<command>make all</command>\n</execute_command>"),
        result(
            start + 9000,
            "[execute_command for 'make all'] Result:",
            "The user approved this operation and provided the following context:\n"
            "<feedback>\nuse -j4\n</feedback>",
            "[execute_command for 'make all'] Result:",
            "built",
        ),
    ]


def pearai_permission_ui_asks(start):
    """The ui_messages.json asks of the three PearAI calls.

    The ui_messages.json asks of the same three calls, each before the API message that answers it.
    """
    return [
        {"ts": start + 1000, "type": "ask", "ask": "tool", "text": '{"tool":"newFileCreated"}'},
        {"ts": start + 4000, "type": "ask", "ask": "command", "text": "rm -rf build"},
        {"ts": start + 7000, "type": "ask", "ask": "command", "text": "make all"},
    ]


# The `PearAI` output channel as the editor writes it (pearai-submodule at
# 51eceef6, extensions/vscode/src/extension/VsCodeExtension.ts:147-155 and
# core/llm/index.ts:247-261, :373-384, :551-554, :586-587).
PEARAI_LOG_REL = (
    ".config/PearAI/logs/20261003T130000/window1/exthost/"
    "output_logging_20261003T130001/1-PearAI.log"
)
PEARAI_LOG_SEP = ("=" * 74 + "\n") * 2


def pearai_log_settings(model, prompt):
    return "%sSettings:\ncontextLength: 8192\nmodel: %s\nmaxTokens: 4096\n\n%s\n\n%s" % (
        PEARAI_LOG_SEP,
        model,
        "#" * 44,
        prompt,
    )


def pearai_log_completion(text):
    return "%sCompletion:\n\n%s\n\n" % (PEARAI_LOG_SEP, text)


def pearai_output_log():
    """Three requests.

    A turn of the fixture session, a turn whose session is gone, and an autocomplete request with
    another model.
    """
    return "".join(
        (
            pearai_log_settings(
                "pearai_model_2",
                "<system>\nYou are PearAI.\n\n<user>\nwhy does build.sh fail\n\n"
                "<assistant>\nThe make target is missing.\n\n<user>\nadd the target\n\n",
            ),
            pearai_log_completion("Added `all:` to the Makefile."),
            pearai_log_settings("pearai_model_2", "<user>\ndelete the old logs\n\n"),
            pearai_log_completion("Removed logs/."),
            pearai_log_settings("starcoder2:3b", "def add(a, b):\n<FIM>\n"),
            pearai_log_completion("    return a + b"),
        )
    )


def pearai_dev_chat():
    return [
        {
            "completionOptions": {"model": "pearai_model_2"},
            "prompt": "<user>\nadd the target\n\n",
            "completion": "Added `all:` to the Makefile.",
            "feedback": True,
            "sessionId": PEARAI_SESSION,
        },
        {
            "completionOptions": {"model": "pearai_model"},
            "prompt": "<user>\nwhat broke the deploy\n\n",
            "completion": "The DNS record.",
            "feedback": False,
            "sessionId": "9b1c2d3e-0000-4000-8000-0000000000ff",
        },
    ]


def pearai_tokens():
    return [
        {
            "model": "pearai_model",
            "provider": "pearai_server",
            "promptTokens": 120,
            "generatedTokens": 9,
        },
        {
            "model": "pearai_model_2",
            "provider": "pearai_server",
            "promptTokens": 180,
            "generatedTokens": 11,
        },
    ]


def build_pearai(home: Path) -> None:
    sessions = home / ".pearai/sessions"
    _json(sessions / (PEARAI_SESSION + ".json"), pearai_session())
    _json(sessions / (PEARAI_SEARCH_SESSION + ".json"), pearai_search_session())
    _json(sessions / "sessions.json", pearai_index())
    (home / ".pearai/config.json").write_text('{"models": []}', encoding="utf-8")
    roo = home / PEARAI_ROO
    task_start, ui_task_start = int(PEARAI_TASK), int(PEARAI_UI_TASK)
    _json(
        roo / "tasks" / PEARAI_TASK / "api_conversation_history.json",
        pearai_api_records() + pearai_permission_records(task_start + 20000),
    )
    _json(roo / "tasks" / PEARAI_TASK / "task_metadata.json", {"files_in_context": []})
    _json(
        roo / "tasks" / PEARAI_UI_TASK / "ui_messages.json",
        roo_ui_records(ui_task_start) + pearai_permission_ui_asks(ui_task_start + 20000),
    )
    _json(
        roo / "tasks" / PEARAI_UI_TASK / "api_conversation_history.json",
        pearai_api_records(ui_task_start) + pearai_permission_records(ui_task_start + 20000),
    )
    log = home / PEARAI_LOG_REL
    log.parent.mkdir(parents=True, exist_ok=True)
    log.write_text(pearai_output_log(), encoding="utf-8")
    (log.parent / "2-Git.log").write_text("> fetch\n", encoding="utf-8")  # noise
    dev = home / ".pearai/dev_data"
    _jsonl(dev / "chat.jsonl", pearai_dev_chat())
    _jsonl(dev / "tokens_generated.jsonl", pearai_tokens())
    _jsonl(dev / "autocomplete.jsonl", [{"accepted": True}])  # noise
    _vscdb(
        home / PEARAI_GS / "state.vscdb",
        {
            "pearai.pearai-roo-cline": {
                "taskHistory": [
                    {
                        "id": PEARAI_TASK,
                        "number": 1,
                        "ts": 1791036010000,
                        "task": "run the tests",
                        "tokensIn": 900,
                        "tokensOut": 40,
                        "totalCost": 0.002,
                        "workspace": "/srv/proj",
                    },
                    {
                        # a Creator task whose directory is not in the collection
                        "id": "1791037000000",
                        "number": 2,
                        "ts": 1791037000000,
                        "task": "make a todo app",
                        "tokensIn": 300,
                        "tokensOut": 20,
                        "totalCost": 0.001,
                        "workspace": "/srv/proj",
                        "creatorModeConfig": {
                            "creatorMode": True,
                            "newProjectType": "webapp",
                            "newProjectPath": "/srv/proj",
                        },
                    },
                ],
                "autoApprovalEnabled": True,
                "alwaysAllowReadOnly": True,
                "alwaysAllowExecute": False,
                "allowedCommands": ["npm test", "npm run lint"],
                "currentApiConfigName": "default",
            }
        },
    )
