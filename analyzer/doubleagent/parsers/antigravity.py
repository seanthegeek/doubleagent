"""Antigravity CLI (`agy`) and desktop app conversations, histories and permissions.

Reads, under each data directory `.gemini/<dir>/` (`antigravity-cli` for the CLI,
`antigravity` for the desktop app, `antigravity-ide`, `antigravity-backup`): the
conversation databases `conversations/<id>.db` (SQLite with WAL sidecars, protobuf blobs)
and `conversation_summaries.db`; the brain mirror `brain/<id>/.system_generated/logs/`
(`transcript_full.jsonl`, `transcript.jsonl` or their `chunks/`, one copy read per
conversation; a secondary source); the CLI's `history.jsonl` (a secondary source); and the
standing permission files `.gemini/config/config.json`, `config/projects/<id>.json`,
`config/sidecars/<name>/sidecar.json` and `antigravity-cli/settings.json`. It yields
user, assistant, thinking, tool_use, tool_result and system rows, with model, permission
grant, permission decision and permission setting rows.

Fields read: `trajectory_meta` `cascade_id` and `source`; `trajectory_metadata_blob.data`;
`steps` `idx`, `step_type`, `status`, `metadata`, `step_payload` and `permissions`;
`gen_metadata.data`; the `conversation_summaries` columns `conversation_id`, `title`,
`preview`, `step_count`, `last_modified_time`, `workspace_uris`, `status`, `agent_name`,
`parent_conversation_id`; the brain lines' `type`, `source`, `status`, `created_at`,
`content`, `thinking`, `tool_calls[]`, `error`, `truncated_fields`; the history's
`display`, `timestamp`, `workspace`, `conversationId`; and the permission keys
`standing_entries` names. `SCHEMA` lists every protobuf field read.

The protobuf messages come from the descriptors embedded in the `agy` binary (Linux
x86-64), which match those of `agy` 1.2.16 and the desktop app's `language_server` 2.23.0
(macOS arm64); the brain value forms are from `agy` 1.3.6. The shapes were checked against
real installs. `analyzer/tools/proto_descriptors.py` re-extracts the descriptors from a new
release. See `research/antigravity.md`.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterator
from pathlib import Path
from urllib.parse import unquote, urlparse

from ..inputs import Artifact
from ..model import Row, compact
from ..protobuf import Schema, decode
from ..sqlite_util import open_copy, sqlite_problem, table_names
from ..timeutil import to_utc
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_mtime,
    iter_jsonl,
    line_text,
    setting_value,
    text_of,
    track_change,
)

BASE = ".gemini/antigravity-cli/"
# The four data directories under ~/.gemini that hold the same stores: the
# CLI's, the desktop app's, the separately installed IDE's and the backup the
# app takes when it migrates pre-2.0 IDE data. Group 1 is the directory name.
DATA_DIR = r"^\.gemini/(antigravity(?:-cli|-ide|-backup)?)/"
CONVERSATION_PATTERN = re.compile(DATA_DIR + r"conversations/[^/]+\.db$")
SUMMARIES_PATTERN = re.compile(DATA_DIR + r"conversation_summaries\.db$")
HISTORY_REL = BASE + "history.jsonl"
# The brain mirror: per conversation, the step transcript in its
# complete form, its shortened twin, or either split into numbered chunks.
# Group 2 is the conversation id; group 3 the file form, or groups 4 and 5
# the chunked form and the chunk number.
BRAIN_PATTERN = re.compile(
    DATA_DIR + r"brain/([^/]+)/\.system_generated/logs/"
    r"(?:(transcript(?:_full)?)\.jsonl|chunks/(transcript(?:_full)?)/(\d+)\.jsonl)$"
)
FULL = "transcript_full"
SHORT = "transcript"
# How far a brain row's time may be from the database row's for the timeline
# to pair them: the mirror records only `created_at`, the database reader
# stamps assistant, tool_use and tool_result rows with `completed_at`.
BRAIN_TOLERANCE_SECONDS = 600.0
# Standing permission files.
CONFIG_REL = ".gemini/config/config.json"
PROJECT_PATTERN = re.compile(r"^\.gemini/config/projects/[^/]+\.json$")
SIDECAR_PATTERN = re.compile(r"^\.gemini/config/sidecars/[^/]+/sidecar\.json$")
CLI_SETTINGS_REL = BASE + "settings.json"

# exa.cortex_pb.PermissionScope, lower-cased without the PERMISSION_SCOPE_
# prefix: how long a permission grant holds.
PERMISSION_SCOPES = {
    0: "unspecified",
    1: "once",
    2: "conversation",
    3: "workspace",
    4: "global",
    5: "project",
}

# exa.cortex_pb.CortexTrajectorySource, lower-cased without the prefix: the
# surface that started the conversation (trajectory_meta.source).
TRAJECTORY_SOURCES = {
    0: "unspecified",
    1: "cascade_client",
    2: "explain_problem",
    3: "refactor_function",
    4: "eval",
    5: "eval_task",
    6: "async_prr",
    7: "async_cf",
    8: "async_sl",
    9: "async_prd",
    10: "async_cm",
    12: "interactive_cascade",
    13: "replay",
    15: "sdk",
    16: "subagent",
    17: "cli",
    18: "jetbox",
    19: "agent_api",
    20: "cider",
    21: "gemini_app",
    22: "vscode",
    23: "visual_studio",
}

# exa.cortex_pb.CortexStepType, lower-cased without the prefix.
STEP_TYPES = {
    0: "unspecified",
    1: "dummy",
    2: "finish",
    3: "plan_input",
    4: "mquery",
    5: "code_action",
    6: "git_commit",
    7: "grep_search",
    8: "view_file",
    9: "list_directory",
    10: "compile",
    13: "view_code_item",
    14: "user_input",
    15: "planner_response",
    17: "error_message",
    21: "run_command",
    23: "checkpoint",
    24: "propose_code",
    25: "find",
    27: "suggested_responses",
    28: "command_status",
    29: "memory",
    31: "read_url_content",
    32: "view_content_chunk",
    33: "search_web",
    34: "retrieve_memory",
    38: "mcp_tool",
    39: "manager_feedback",
    40: "tool_call_proposal",
    41: "tool_call_choice",
    42: "trajectory_choice",
    45: "clipboard",
    47: "view_file_outline",
    49: "post_pr_review",
    51: "list_resources",
    52: "read_resource",
    53: "lint_diff",
    54: "find_all_references",
    55: "brain_update",
    56: "open_browser_url",
    57: "run_extension_code",
    59: "proposal_feedback",
    60: "trajectory_search",
    61: "execute_browser_javascript",
    62: "list_browser_pages",
    63: "capture_browser_screenshot",
    64: "click_browser_pixel",
    65: "read_terminal",
    66: "capture_browser_console_logs",
    67: "read_browser_page",
    68: "browser_get_dom",
    73: "code_search",
    74: "browser_input",
    75: "browser_move_mouse",
    76: "browser_select_option",
    77: "browser_scroll_up",
    78: "browser_scroll_down",
    79: "browser_click_element",
    80: "browser_press_key",
    81: "task_boundary",
    82: "notify_user",
    83: "code_acknowledgement",
    84: "internal_search",
    85: "browser_subagent",
    86: "file_change",
    87: "move",
    88: "browser_scroll",
    89: "knowledge_generation",
    90: "ephemeral_message",
    91: "generate_image",
    92: "delete_directory",
    93: "compile_applet",
    94: "install_applet_dependencies",
    95: "install_applet_package",
    96: "browser_resize_window",
    97: "browser_drag_pixel_to_pixel",
    98: "conversation_history",
    99: "knowledge_artifacts",
    100: "send_command_input",
    101: "system_message",
    102: "wait",
    103: "agency_tool_call",
    104: "cider_agent_dummy",
    105: "build_cleaner",
    106: "blaze_build_targets",
    107: "blaze_test_targets",
    108: "set_up_firebase",
    109: "moma",
    110: "restart_dev_server",
    111: "deploy_firebase",
    112: "shell_exec",
    113: "browser_mouse_wheel",
    114: "lint_applet",
    116: "ki_insertion",
    117: "retrieve_content",
    118: "critique",
    119: "findings",
    120: "browser_mouse_up",
    121: "browser_mouse_down",
    122: "workspace_api",
    123: "browser_list_network_requests",
    124: "browser_get_network_request",
    125: "browser_refresh_page",
    126: "edit_notebook",
    127: "invoke_subagent",
    128: "write_blob",
    129: "read_notebook",
    130: "propose_ai_comments",
    131: "start_code_review",
    132: "generic",
    133: "set_up_cloud_sql",
    134: "execute_notebook",
    135: "cloud_sql_update_schema",
    136: "rpc_action",
    137: "cloud_sql_execute_sql",
    138: "ask_question",
    139: "directory_rules",
    140: "tool_search",
}

# exa.cortex_pb.CortexStepStatus
STEP_STATUS = {
    0: "unspecified",
    1: "pending",
    2: "running",
    3: "done",
    4: "invalid",
    5: "cleared",
    6: "canceled",
    7: "error",
    8: "generating",
    9: "waiting",
    11: "queued",
    12: "interrupted",
}

STEP_SOURCE_USER_IMPLICIT = 3  # exa.cortex_pb.CortexStepSource

# Field numbers from the embedded descriptors; only the fields the parser reads.
SCHEMA: Schema = {
    "Step": {
        1: ("type", "int"),
        4: ("status", "int"),
        5: ("metadata", "CortexStepMetadata"),
        10: ("code_action", "CortexStepCodeAction"),
        12: ("finish", "CortexStepFinish"),
        13: ("grep_search", "CortexStepGrepSearch"),
        14: ("view_file", "CortexStepViewFile"),
        15: ("list_directory", "CortexStepListDirectory"),
        19: ("user_input", "CortexStepUserInput"),
        20: ("planner_response", "CortexStepPlannerResponse"),
        23: ("write_to_file", "CortexStepWriteToFile"),
        24: ("error_message", "CortexStepErrorMessage"),
        28: ("run_command", "CortexStepRunCommand"),
        30: ("checkpoint", "CortexStepCheckpoint"),
        31: ("error", "CortexErrorDetails"),
        34: ("find", "CortexStepFind"),
        37: ("command_status", "CortexStepCommandStatus"),
        38: ("memory", "CortexStepMemory"),
        40: ("read_url_content", "CortexStepReadUrlContent"),
        42: ("search_web", "CortexStepSearchWeb"),
        47: ("mcp_tool", "CortexStepMcpTool"),
        93: ("task_boundary", "CortexStepTaskBoundary"),
        94: ("notify_user", "CortexStepNotifyUser"),
        98: ("file_change", "CortexStepFileChange"),
        103: ("ephemeral_message", "CortexStepEphemeralMessage"),
        111: ("conversation_history", "CortexStepConversationHistory"),
        113: ("send_command_input", "CortexStepSendCommandInput"),
        114: ("system_message", "CortexStepSystemMessage"),
        116: ("agency_tool_call", "CortexStepAgencyToolCall"),
        127: ("shell_exec", "CortexStepShellExec"),
        133: ("permissions", "TrajectoryPermissions"),
        140: ("generic", "CortexStepGeneric"),
        143: ("invoke_subagent", "CortexStepInvokeSubagent"),
        154: ("ask_question", "CortexStepAskQuestion"),
        156: ("directory_rules", "CortexStepDirectoryRules"),
    },
    "CortexStepMetadata": {
        1: ("created_at", "ts"),
        3: ("source", "int"),
        4: ("tool_call", "ChatToolCall"),
        7: ("finished_generating_at", "ts"),
        8: ("completed_at", "ts"),
        12: ("execution_id", "str"),
        30: ("tool_summary", "str"),
        31: ("tool_action", "str"),
        32: ("started_at", "ts"),
    },
    "ChatToolCall": {
        1: ("id", "str"),
        2: ("name", "str"),
        3: ("arguments_json", "str"),
        9: ("original_name", "str"),
    },
    "CortexStepUserInput": {
        1: ("query", "str"),
        2: ("user_response", "str"),
        6: ("is_queued_message", "bool"),
    },
    "CortexStepPlannerResponse": {
        1: ("response", "str"),
        3: ("thinking", "str"),
        6: ("message_id", "str"),
        7: ("+tool_calls", "ChatToolCall"),
        8: ("modified_response", "str"),
        16: ("raw_thinking", "str"),
    },
    "CortexStepGeneric": {1: ("+args", "KV"), 2: ("result", "GenericStepResult")},
    "KV": {1: ("key", "str"), 2: ("value", "str")},
    "GenericStepResult": {1: ("result", "str"), 7: ("full_output_uri", "str")},
    "CortexStepRunCommand": {
        1: ("command", "str"),
        2: ("cwd", "str"),
        4: ("stdout", "str"),
        5: ("stderr", "str"),
        6: ("exit_code", "int"),
        14: ("user_rejected", "bool"),
        21: ("combined_output", "RunCommandOutput"),
        23: ("command_line", "str"),
        34: ("ran_in_sandbox", "bool"),
        37: ("timed_out", "bool"),
    },
    "RunCommandOutput": {1: ("full", "str"), 2: ("truncated", "str")},
    "CortexStepCommandStatus": {
        1: ("command_id", "str"),
        3: ("stdout", "str"),
        4: ("stderr", "str"),
        5: ("exit_code", "int"),
        9: ("combined", "str"),
    },
    "CortexStepShellExec": {
        1: ("command", "str"),
        2: ("exit_code", "int"),
        3: ("output", "str"),
        4: ("error_message", "str"),
    },
    "CortexStepSendCommandInput": {
        1: ("command_id", "str"),
        2: ("input", "str"),
        6: ("terminate", "bool"),
    },
    "CortexStepViewFile": {
        1: ("absolute_path_uri", "str"),
        2: ("start_line", "int"),
        3: ("end_line", "int"),
        17: ("is_skill_file", "bool"),
    },
    "CortexStepListDirectory": {1: ("directory_path_uri", "str")},
    "CortexStepGrepSearch": {
        1: ("query", "str"),
        10: ("command_run", "str"),
        11: ("search_path_uri", "str"),
    },
    "CortexStepFind": {
        1: ("pattern", "str"),
        9: ("command_run", "str"),
        10: ("search_directory", "str"),
    },
    "CortexStepWriteToFile": {1: ("target_file_uri", "str"), 4: ("file_created", "bool")},
    "CortexStepFileChange": {
        1: ("absolute_path_uri", "str"),
        2: ("file_change_type", "int"),
        5: ("instruction", "str"),
    },
    "CortexStepCodeAction": {
        1: ("action_spec", "ActionSpec"),
        8: ("code_instruction", "str"),
        26: ("description", "str"),
    },
    "ActionSpec": {
        1: ("command", "ActionSpecCommand"),
        2: ("create_file", "ActionSpecPath"),
        4: ("delete_file", "ActionSpecDeleteFile"),
        5: ("sed", "ActionSpecSed"),
    },
    "ActionSpecCommand": {1: ("instruction", "str"), 4: ("file", "PathScopeItem")},
    "ActionSpecPath": {1: ("instruction", "str"), 2: ("path", "PathScopeItem")},
    "ActionSpecDeleteFile": {1: ("path", "PathScopeItem")},
    "ActionSpecSed": {3: ("path", "PathScopeItem")},
    "PathScopeItem": {1: ("absolute_path_migrate_me_to_uri", "str"), 5: ("absolute_uri", "str")},
    "CortexStepReadUrlContent": {1: ("url", "str"), 3: ("resolved_url", "str")},
    "CortexStepSearchWeb": {1: ("query", "str"), 5: ("summary", "str")},
    "CortexStepMcpTool": {
        1: ("server_name", "str"),
        2: ("tool_call", "ChatToolCall"),
        3: ("result_string", "str"),
        7: ("user_rejected", "bool"),
    },
    "CortexStepAgencyToolCall": {1: ("agent_name", "str"), 2: ("function_name", "str")},
    "CortexStepInvokeSubagent": {
        1: ("subagent_name", "str"),
        2: ("prompt", "str"),
        5: ("conversation_id", "str"),
        10: ("+results", "SubagentResult"),
    },
    "SubagentResult": {1: ("conversation_id", "str")},
    "CortexStepAskQuestion": {1: ("+questions", "AskQuestionEntry")},
    "AskQuestionEntry": {
        1: ("question", "str"),
        4: ("+selected_option_ids", "str"),
        5: ("write_in_response", "str"),
        6: ("skipped", "bool"),
    },
    "CortexStepCheckpoint": {
        4: ("user_intent", "str"),
        5: ("session_summary", "str"),
        10: ("conversation_title", "str"),
    },
    "CortexStepMemory": {1: ("memory_id", "str"), 3: ("action", "int")},
    "CortexStepFinish": {2: ("output_string", "str")},
    "CortexStepErrorMessage": {3: ("error", "CortexErrorDetails")},
    "CortexErrorDetails": {
        1: ("user_error_message", "str"),
        2: ("short_error", "str"),
        3: ("full_error", "str"),
    },
    "CortexStepSystemMessage": {1: ("message", "str"), 3: ("event_type", "str")},
    "CortexStepEphemeralMessage": {1: ("content", "str")},
    "CortexStepTaskBoundary": {
        1: ("task_name", "str"),
        2: ("task_status", "str"),
        3: ("task_summary", "str"),
    },
    "CortexStepNotifyUser": {2: ("notification_content", "str")},
    "CortexStepDirectoryRules": {2: ("+rule_file_uris", "str")},
    "CortexStepConversationHistory": {1: ("content", "str")},
    "TrajectoryPermissions": {
        1: ("+file_access_permissions", "FileAccessPermission"),
        2: ("+permission_grants", "PermissionGrant"),
    },
    "PermissionGrant": {
        1: ("resource", "PermissionResource"),
        2: ("allow", "bool"),
        3: ("scope", "int"),
        4: ("user_deny_instruction", "str"),
        5: ("action", "str"),
        6: ("target", "str"),
    },
    "PermissionResource": {1: ("action", "str"), 2: ("target", "str")},
    "FileAccessPermission": {
        1: ("path", "str"),
        2: ("is_directory", "bool"),
        3: ("allow", "bool"),
        4: ("scope", "int"),
        5: ("from_current_step", "bool"),
    },
    "CortexTrajectoryMetadata": {
        1: ("+workspaces", "CortexWorkspaceMetadata"),
        2: ("created_at", "ts"),
        5: ("parent_conversation_id", "str"),
        6: ("root_conversation_id", "str"),
        7: ("+workspace_uris", "str"),
        17: ("nesting_depth", "int"),
        18: ("project_id", "str"),
    },
    "CortexWorkspaceMetadata": {
        1: ("workspace_folder_absolute_uri", "str"),
        2: ("git_root_absolute_uri", "str"),
        4: ("branch_name", "str"),
    },
    "CortexStepGeneratorMetadata": {
        1: ("chat_model", "ChatModelMetadata"),
        2: ("+step_indices", "int"),
    },
    "ChatModelMetadata": {
        19: ("response_model", "str"),
        21: ("model_display_name", "str"),
        22: ("response_model_full", "str"),
    },
}

# The oneof step kinds: every Step field but the scalars, the metadata and
# the permissions that sit beside the step kind.
PAYLOAD_KINDS = [
    name
    for number, (name, kind) in SCHEMA["Step"].items()
    if kind not in ("int", "CortexStepMetadata", "TrajectoryPermissions")
]

# Which JSON argument best identifies a proposed tool call, tried in order.
ARG_KEYS = (
    "CommandLine",
    "Command",
    "AbsolutePath",
    "TargetFile",
    "AbsolutePathUri",
    "Path",
    "DirectoryPath",
    "Url",
    "Query",
    "SearchPath",
    "Pattern",
    "Prompt",
    "Instruction",
    "TaskName",
    "Name",
)


def uri_to_path(uri: str) -> str:
    """Return the local path of a `file:` URI (a Windows drive path without its slash)."""
    if not uri:
        return ""
    if not uri.startswith("file:"):
        return uri
    path = unquote(urlparse(uri).path)
    if re.match(r"^/[A-Za-z]:", path):
        path = path[1:]
    return path


def tool_args_summary(arguments_json: str) -> str:
    """Return the argument that names a tool call's action (`ARG_KEYS`), else the JSON."""
    try:
        args = json.loads(arguments_json) if arguments_json else {}
    except ValueError:
        return arguments_json
    if isinstance(args, dict):
        for k in ARG_KEYS:
            value = args.get(k)
            if value not in (None, "", [], {}):
                return value if isinstance(value, str) else compact_json(value)
    return compact_json(args)


def _path_of(scope: dict | None) -> str:
    if not scope:
        return ""
    return uri_to_path(
        scope.get("absolute_uri") or scope.get("absolute_path_migrate_me_to_uri") or ""
    )


def _output(payload: dict) -> str:
    combined = payload.get("combined_output") or {}
    return (
        combined.get("full")
        or combined.get("truncated")
        or payload.get("combined")
        or payload.get("stdout")
        or payload.get("stderr")
        or payload.get("output")
        or ""
    )


def _join(*parts: str) -> str:
    return " | ".join(part for part in parts if part)


def _bool(value: object) -> str:
    return "true" if value else "false"


def _list(value: object) -> list:
    return value if isinstance(value, list) else []


def _dict(value: object) -> dict:
    return value if isinstance(value, dict) else {}


def _read_json(artifact: Artifact) -> object:
    """The parsed file, or None when it cannot be read or parsed."""
    try:
        return json.loads(artifact.disk_path.read_bytes().decode("utf-8", errors="replace"))
    except (OSError, ValueError):
        return None


def _regular(path: Path) -> bool:
    """True for a regular file, never through a symlink."""
    return path.is_file() and not path.is_symlink()


def _chunks(logs: Path, form: str) -> list[Path]:
    """The chunk files of `form` under `logs/chunks`, in chunk order."""
    directory = logs / "chunks" / form
    try:
        names = [
            path for path in directory.iterdir() if path.suffix == ".jsonl" and path.stem.isdigit()
        ]
    except OSError:
        return []
    return sorted((path for path in names if _regular(path)), key=lambda path: int(path.stem))


def brain_copy(path: Path, form: str, chunk_form: str) -> bytes | None:
    """Return the bytes of the brain transcript `path` stands for, or None.

    None means another copy of the same transcript is the one read. The copies, in order
    of preference: `transcript_full.jsonl`; its chunks, concatenated (a line cut at a
    chunk boundary is whole again); the shortened `transcript.jsonl`; its chunks. A
    chunked form is read through its first chunk present.
    """
    logs = path.parent if form else path.parent.parent.parent
    full = logs / (FULL + ".jsonl")
    if form == FULL:
        return path.read_bytes()
    if form == SHORT:
        if _regular(full) or _chunks(logs, FULL):
            return None
        return path.read_bytes()
    chunks = _chunks(logs, chunk_form)
    if not chunks or chunks[0].name != path.name:
        return None
    if _regular(full):
        return None
    if chunk_form == SHORT and (_chunks(logs, FULL) or _regular(logs / (SHORT + ".jsonl"))):
        return None
    return b"".join(chunk.read_bytes() for chunk in chunks)


def _brain_args(tool_call: dict) -> str:
    """Return a brain tool call's arguments as a JSON string.

    The arguments are `arguments_json` as the database records it, else `args` or
    `arguments`, an object or a string.
    """
    for key in ("arguments_json", "args", "arguments"):
        value = tool_call.get(key)
        if isinstance(value, str) and value:
            return value
        if isinstance(value, (dict, list)):
            return json.dumps(value, ensure_ascii=False)
    return ""


def permission_texts(permissions: dict) -> list[str]:
    """Return one `permission grant:` text per entry of a TrajectoryPermissions."""
    out = []
    for grant in permissions.get("permission_grants", []):
        resource = grant.get("resource") or {}
        scope = grant.get("scope", 0)
        text = "permission grant: action=%s target=%s allow=%s scope=%s" % (
            grant.get("action") or resource.get("action", ""),
            grant.get("target") or resource.get("target", ""),
            _bool(grant.get("allow")),
            PERMISSION_SCOPES.get(scope, str(scope)),
        )
        if grant.get("user_deny_instruction"):
            text += " deny_instruction=" + grant["user_deny_instruction"]
        out.append(text)
    for file_access in permissions.get("file_access_permissions", []):
        scope = file_access.get("scope", 0)
        text = "permission grant: path=%s is_directory=%s allow=%s scope=%s" % (
            uri_to_path(file_access.get("path", "")),
            _bool(file_access.get("is_directory")),
            _bool(file_access.get("allow")),
            PERMISSION_SCOPES.get(scope, str(scope)),
        )
        if file_access.get("from_current_step"):
            text += " from_current_step=true"
        out.append(text)
    return out


def standing_entries(rel: str, doc: dict) -> list[tuple[str, object, str]]:
    """Return (key, value, project_path) for each permission entry of a standing file.

    Only these keys are read: `userSettings.globalPermissionGrants.allow` and `.deny`,
    `permissionGrantsV2Migrated`, `sandboxEnabledAtV2Migration` and each sidecar's `enabled`
    and `resolvedPermissions.accessGrants` (config.json); `permissionGrants` and the first
    `projectResources.resources[].folderUri` (a project); `agent_permissions.access_grants`
    (a sidecar); `permissions.allow`, `.deny` and `trustedWorkspaces` (settings.json).
    """
    out: list[tuple[str, object, str]] = []
    if rel == CONFIG_REL:
        user_settings = _dict(doc.get("userSettings"))
        grants = _dict(user_settings.get("globalPermissionGrants"))
        for list_key in ("allow", "deny"):
            for value in _list(grants.get(list_key)):
                out.append(("userSettings.globalPermissionGrants." + list_key, value, ""))
        for k in ("permissionGrantsV2Migrated", "sandboxEnabledAtV2Migration"):
            if k in user_settings:
                out.append(("userSettings." + k, user_settings[k], ""))
        for name, sidecar in _dict(doc.get("sidecars")).items():
            sidecar = _dict(sidecar)
            if "enabled" in sidecar:
                out.append(("sidecars.%s.enabled" % name, sidecar["enabled"], ""))
            for value in _list(_dict(sidecar.get("resolvedPermissions")).get("accessGrants")):
                out.append(("sidecars.%s.resolvedPermissions.accessGrants" % name, value, ""))
    elif PROJECT_PATTERN.match(rel):
        folder = ""
        for resource in _list(_dict(doc.get("projectResources")).get("resources")):
            uri = _dict(resource).get("folderUri")
            if isinstance(uri, str) and uri:
                folder = uri_to_path(uri)
                break
        project_grants = _dict(doc.get("permissionGrants"))
        inner = _dict(project_grants.get("permissionGrants"))
        for list_key in ("allow", "deny"):
            for value in _list(inner.get(list_key)):
                out.append(("permissionGrants.permissionGrants." + list_key, value, folder))
        if "v2Migrated" in project_grants:
            out.append(("permissionGrants.v2Migrated", project_grants["v2Migrated"], folder))
    elif SIDECAR_PATTERN.match(rel):
        for value in _list(_dict(doc.get("agent_permissions")).get("access_grants")):
            out.append(("agent_permissions.access_grants", value, ""))
    elif rel == CLI_SETTINGS_REL:
        perms = _dict(doc.get("permissions"))
        for list_key in ("allow", "deny"):
            for value in _list(perms.get(list_key)):
                out.append(("permissions." + list_key, value, ""))
        for value in _list(doc.get("trustedWorkspaces")):
            path = uri_to_path(value) if isinstance(value, str) else ""
            out.append(("trustedWorkspace", path or value, path))
    return out


def describe_payload(kind: str, payload: dict) -> str:
    """Return one line describing a tool step's result or a system event."""
    if kind == "run_command":
        return _join(
            payload.get("command_line") or payload.get("command") or "",
            "[rejected by user]" if payload.get("user_rejected") else "",
            "[timed out]" if payload.get("timed_out") else "",
            "exit=%s" % payload["exit_code"] if "exit_code" in payload else "",
            _output(payload),
        )
    if kind in ("command_status", "shell_exec"):
        return _join(
            payload.get("command", ""),
            "exit=%s" % payload["exit_code"] if "exit_code" in payload else "",
            _output(payload) or payload.get("error_message", ""),
        )
    if kind == "send_command_input":
        return "input to %s: %s%s" % (
            payload.get("command_id", ""),
            payload.get("input", ""),
            " [terminate]" if payload.get("terminate") else "",
        )
    if kind == "view_file":
        line_range = ""
        if payload.get("start_line") or payload.get("end_line"):
            line_range = ":%s-%s" % (payload.get("start_line", ""), payload.get("end_line", ""))
        return uri_to_path(payload.get("absolute_path_uri", "")) + line_range
    if kind == "list_directory":
        return uri_to_path(payload.get("directory_path_uri", ""))
    if kind == "grep_search":
        return _join(
            payload.get("query", ""),
            uri_to_path(payload.get("search_path_uri", "")),
            payload.get("command_run", ""),
        )
    if kind == "find":
        return _join(
            payload.get("pattern", ""),
            payload.get("search_directory", ""),
            payload.get("command_run", ""),
        )
    if kind == "write_to_file":
        return uri_to_path(payload.get("target_file_uri", "")) + (
            " [created]" if payload.get("file_created") else ""
        )
    if kind == "file_change":
        return _join(
            uri_to_path(payload.get("absolute_path_uri", "")), payload.get("instruction", "")
        )
    if kind == "code_action":
        spec = payload.get("action_spec") or {}
        action_kind = path = ""
        for key in ("command", "create_file", "delete_file", "sed"):
            action = spec.get(key)
            if action:
                action_kind, path = key, _path_of(action.get("file") or action.get("path"))
                break
        return _join(
            action_kind, path, payload.get("description") or payload.get("code_instruction", "")
        )
    if kind == "read_url_content":
        return payload.get("resolved_url") or payload.get("url", "")
    if kind == "search_web":
        return payload.get("query", "")
    if kind == "mcp_tool":
        tool_call = payload.get("tool_call") or {}
        return _join(
            "%s/%s" % (payload.get("server_name", ""), tool_call.get("name", "")),
            "[rejected by user]" if payload.get("user_rejected") else "",
            payload.get("result_string", ""),
        )
    if kind == "agency_tool_call":
        return "%s.%s" % (payload.get("agent_name", ""), payload.get("function_name", ""))
    if kind == "invoke_subagent":
        children = " ".join(
            result.get("conversation_id", "")
            for result in payload.get("results", [])
            if result.get("conversation_id")
        )
        return _join(
            payload.get("subagent_name", ""),
            payload.get("prompt", ""),
            "children: " + children if children else "",
        )
    if kind == "ask_question":
        answers = []
        for question in payload.get("questions", []):
            answer = (
                question.get("write_in_response")
                or " ".join(question.get("selected_option_ids", []))
                or ("skipped" if question.get("skipped") else "")
            )
            answers.append(question.get("question", "") + (" -> " + answer if answer else ""))
        return "; ".join(answers)
    if kind == "generic":
        return (payload.get("result") or {}).get("result", "")
    if kind == "checkpoint":
        return "checkpoint: " + (
            payload.get("conversation_title")
            or payload.get("session_summary")
            or payload.get("user_intent", "")
        )
    if kind == "system_message":
        return "%s: %s" % (
            payload.get("event_type") or "system_message",
            payload.get("message", ""),
        )
    if kind == "ephemeral_message":
        return "ephemeral: " + payload.get("content", "")
    if kind == "task_boundary":
        return "task %s [%s]: %s" % (
            payload.get("task_name", ""),
            payload.get("task_status", ""),
            payload.get("task_summary", ""),
        )
    if kind == "notify_user":
        return "notify user: " + payload.get("notification_content", "")
    if kind in ("error", "error_message"):
        error = payload.get("error", {}) if kind == "error_message" else payload
        return "error: " + (
            error.get("short_error")
            or error.get("user_error_message")
            or error.get("full_error", "")
        )
    if kind == "finish":
        return "finish: " + payload.get("output_string", "")
    if kind == "memory":
        return "memory %s action=%s" % (payload.get("memory_id", ""), payload.get("action", ""))
    if kind == "directory_rules":
        return "directory rules: " + " ".join(
            uri_to_path(uri) for uri in payload.get("rule_file_uris", [])
        )
    if kind == "conversation_history":
        return "conversation history injected (%d chars)" % len(payload.get("content", ""))
    return compact_json(payload)


class AntigravityParser(Parser):
    """Parse Antigravity conversations, the brain mirror, histories and permission files."""

    agent = "antigravity"
    name = "antigravity"
    has_secondary = True

    def wants(self, artifact: Artifact) -> bool:
        """Return True for a conversation, summaries, brain, history or permission file."""
        return (
            artifact.rel == HISTORY_REL
            or bool(BRAIN_PATTERN.match(artifact.rel))
            or bool(SUMMARIES_PATTERN.match(artifact.rel))
            or bool(CONVERSATION_PATTERN.match(artifact.rel))
            or self._standing(artifact.rel)
        )

    @staticmethod
    def _standing(rel: str) -> bool:
        return (
            rel in (CONFIG_REL, CLI_SETTINGS_REL)
            or bool(PROJECT_PATTERN.match(rel))
            or bool(SIDECAR_PATTERN.match(rel))
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for the CLI history and the brain mirror."""
        return artifact.rel == HISTORY_REL or bool(BRAIN_PATTERN.match(artifact.rel))

    def secondary_tolerance(self, artifact: Artifact) -> float:
        """Return the wider brain window: it stamps steps at creation, not completion."""
        if BRAIN_PATTERN.match(artifact.rel):
            return BRAIN_TOLERANCE_SECONDS
        return super().secondary_tolerance(artifact)

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Antigravity file."""
        if artifact.rel == HISTORY_REL:
            yield from self._parse_history(artifact, opts)
        elif BRAIN_PATTERN.match(artifact.rel):
            yield from self._parse_brain(artifact, opts)
        elif self._standing(artifact.rel):
            yield from self._parse_standing(artifact)
        elif SUMMARIES_PATTERN.match(artifact.rel):
            yield from self._parse_summaries(artifact, opts)
        else:
            yield from self._parse_conversation(artifact, opts)

    # -- history.jsonl ------------------------------------------------------------
    def _parse_history(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            row = self.base_row(artifact)
            row.source_line = line_number
            row.timestamp_utc = to_utc(record.get("timestamp"))
            row.project_path = uri_to_path(str(record.get("workspace") or ""))
            row.session_id = str(record.get("conversationId") or "")
            row.turn_type = "user"
            row.text = compact(record.get("display"))
            yield row
        yield from error_rows(self, artifact, errors)

    # -- brain/<uuid>/.system_generated/logs ------------------------------------
    def _parse_brain(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        match = BRAIN_PATTERN.match(artifact.rel)
        if not match:
            return
        blob = brain_copy(artifact.disk_path, match.group(3) or "", match.group(4) or "")
        if blob is None:
            return
        session_id = match.group(2)
        errors: list = []
        calls: list[dict] = []
        for line_number, raw in enumerate(blob.split(b"\n"), 1):
            stripped = raw.strip()
            if not stripped:
                continue
            try:
                record = json.loads(stripped.decode("utf-8", errors="replace"))
            except ValueError as error:
                errors.append((line_number, "unparseable line (%s)" % error, line_text(raw)))
                continue
            if isinstance(record, dict):
                yield from self._brain_rows(artifact, line_number, record, session_id, calls, opts)
        yield from error_rows(self, artifact, errors, session_id)

    def _brain_rows(
        self,
        artifact: Artifact,
        line_number: int,
        record: dict,
        session_id: str,
        calls: list[dict],
        opts: Options,
    ) -> Iterator[Row]:
        """Yield the rows of one brain line as `_step_rows` renders the same step kind.

        `calls` holds the tool calls of the last PLANNER_RESPONSE not yet matched to a
        GENERIC line.
        """
        kind = str(record.get("type") or "")
        status = str(record.get("status") or "").lower()
        content = text_of(record.get("content"))
        truncated = [str(field) for field in _list(record.get("truncated_fields"))]

        def base(turn_type: str, text: str, field: str = "content") -> Row:
            row = self.base_row(artifact)
            row.source_line = line_number
            row.session_id = session_id
            row.timestamp_utc = to_utc(record.get("created_at"))
            row.turn_type = turn_type
            if field in truncated:
                text += " [truncated: %s]" % field
            row.text = compact(text)
            return row

        if kind == "USER_INPUT":
            implicit = record.get("source") == "USER_IMPLICIT"
            calls.clear()
            yield base("system" if implicit else "user", content)
            return
        if kind == "PLANNER_RESPONSE":
            thinking = text_of(record.get("thinking"))
            if thinking and opts.include_thinking:
                yield base("thinking", thinking, "thinking")
            if content:
                yield base("assistant", content)
            calls[:] = [_dict(raw_call) for raw_call in _list(record.get("tool_calls"))]
            for tool_call in calls:
                row = base("tool_use", tool_args_summary(_brain_args(tool_call)), "tool_calls")
                row.tool_name = str(tool_call.get("name") or "")
                row.tool_use_id = str(tool_call.get("id") or "")
                yield row
            return
        if kind == "SYSTEM_MESSAGE":
            text = "system_message: " + content
        elif kind == "ERROR_MESSAGE":
            error = record.get("error")
            if isinstance(error, dict):
                error = (
                    error.get("short_error")
                    or error.get("user_error_message")
                    or error.get("full_error")
                )
            text = "error: " + (error if isinstance(error, str) and error else content)
        else:
            text = content or "step %s" % kind.lower()
        if status and status != "done":
            text = "[%s] %s" % (status, text)
        if kind in ("SYSTEM_MESSAGE", "ERROR_MESSAGE") or not calls:
            yield base("system", text)
            return
        tool_call = calls.pop(0)
        row = base("tool_result", text)
        row.tool_name = str(tool_call.get("name") or "")
        row.tool_use_id = str(tool_call.get("id") or "")
        yield row

    # -- standing permission files ----------------------------------------------
    def _parse_standing(self, artifact: Artifact) -> Iterator[Row]:
        doc = _read_json(artifact)
        mtime = file_mtime(artifact)
        if not isinstance(doc, dict):
            row = self.base_row(artifact)
            row.source_line = 1
            row.timestamp_utc = mtime
            row.turn_type = "system"
            row.text = "parser: unreadable %s" % artifact.rel
            yield row
            return
        for position, (key, value, project) in enumerate(standing_entries(artifact.rel, doc), 1):
            row = self.base_row(artifact)
            row.source_line = position
            row.timestamp_utc = mtime
            row.project_path = project
            row.turn_type = "system"
            row.text = compact(
                "permission setting: %s=%s (from %s, file mtime)"
                % (key, setting_value(value), artifact.rel)
            )
            yield row

    # -- conversation_summaries.db ----------------------------------------------
    def _parse_summaries(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        problem = sqlite_problem(artifact.disk_path)
        if problem:
            yield from error_rows(self, artifact, [(0, problem)])
            return
        with open_copy(artifact.disk_path) as connection:
            if "conversation_summaries" not in table_names(connection):
                return
            for position, summary in enumerate(
                connection.execute(
                    "select * from conversation_summaries order by last_modified_time"
                ),
                1,
            ):
                summary = dict(summary)
                row = self.base_row(artifact)
                row.source_line = position
                row.session_id = str(summary.get("conversation_id") or "")
                row.timestamp_utc = to_utc(summary.get("last_modified_time"))
                try:
                    uris = json.loads(summary.get("workspace_uris") or "[]")
                except ValueError:
                    uris = []
                row.project_path = uri_to_path(uris[0]) if uris else ""
                row.turn_type = "system"
                row.text = compact(
                    _join(
                        "conversation summary",
                        summary.get("title") or summary.get("preview") or "",
                        str(summary.get("status") or ""),
                        "%s steps" % summary.get("step_count", 0),
                        "agent=%s" % summary["agent_name"] if summary.get("agent_name") else "",
                        "parent=%s" % summary["parent_conversation_id"]
                        if summary.get("parent_conversation_id")
                        else "",
                    ),
                )
                yield row

    # -- conversations/<id>.db -----------------------------------------------------
    def _parse_conversation(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        problem = sqlite_problem(artifact.disk_path)
        if problem:
            yield from error_rows(self, artifact, [(0, problem)], artifact.disk_path.name[:-3])
            return
        with open_copy(artifact.disk_path) as connection:
            tables = table_names(connection)
            if "steps" not in tables:
                return
            session_id = artifact.disk_path.name[:-3]
            project_path = branch = source = ""
            match = CONVERSATION_PATTERN.match(artifact.rel)
            data_dir = match.group(1) if match else ""
            if "trajectory_meta" in tables:
                trajectory = connection.execute("select * from trajectory_meta limit 1").fetchone()
                if trajectory:
                    session_id = trajectory["cascade_id"] or session_id
                    if trajectory["source"] is not None:
                        source = TRAJECTORY_SOURCES.get(
                            trajectory["source"], str(trajectory["source"])
                        )
            if "trajectory_metadata_blob" in tables:
                blob = connection.execute(
                    "select data from trajectory_metadata_blob limit 1"
                ).fetchone()
                if blob and blob[0]:
                    try:
                        meta = decode(blob[0], "CortexTrajectoryMetadata", SCHEMA)
                    except ValueError:
                        meta = {}
                    workspace = (meta.get("workspaces") or [{}])[0]
                    project_path = uri_to_path(
                        workspace.get("workspace_folder_absolute_uri")
                        or (meta.get("workspace_uris") or [""])[0]
                    )
                    branch = workspace.get("branch_name", "")
                    if meta.get("created_at"):
                        row = self._row(artifact, 0, session_id, project_path, branch)
                        row.timestamp_utc = meta["created_at"]
                        row.turn_type = "system"
                        row.text = compact(
                            _join(
                                "session start",
                                "source=%s" % source if source else "",
                                "data_dir=%s" % data_dir if data_dir else "",
                                "project_id=%s" % meta["project_id"]
                                if meta.get("project_id")
                                else "",
                                "parent=%s" % meta["parent_conversation_id"]
                                if meta.get("parent_conversation_id")
                                else "",
                            ),
                        )
                        yield row
            models: dict[int, str] = {}
            if "gen_metadata" in tables:
                for (data,) in connection.execute("select data from gen_metadata"):
                    try:
                        generator = decode(data or b"", "CortexStepGeneratorMetadata", SCHEMA)
                    except ValueError:
                        continue
                    chat_model = generator.get("chat_model") or {}
                    model = (
                        chat_model.get("response_model")
                        or chat_model.get("model_display_name")
                        or ""
                    )
                    for index in generator.get("step_indices", []):
                        models[index] = model
            seen_models: dict = {}
            bad = 0
            columns = {
                column_info[1] for column_info in connection.execute("pragma table_info(steps)")
            }
            permissions_column = (
                "permissions" if "permissions" in columns else "NULL AS permissions"
            )
            for step_row in connection.execute(
                "select idx, step_type, status, metadata, step_payload, %s from steps order by idx"
                % permissions_column
            ):
                index = step_row["idx"]
                try:
                    step = (
                        decode(step_row["step_payload"], "Step", SCHEMA)
                        if step_row["step_payload"]
                        else {}
                    )
                    if "metadata" not in step and step_row["metadata"]:
                        step["metadata"] = decode(
                            step_row["metadata"], "CortexStepMetadata", SCHEMA
                        )
                except ValueError:
                    bad += 1
                    continue
                model = models.get(index, "")
                change = track_change("model", seen_models, session_id, model)
                if change:
                    # The model at the conversation's first generated step and
                    # on every change, before that step's own rows.
                    metadata = step.get("metadata") or {}
                    row = self._row(artifact, index, session_id, project_path, branch)
                    row.timestamp_utc = metadata.get("created_at") or metadata.get(
                        "completed_at", ""
                    )
                    row.turn_type = "system"
                    row.model = model
                    row.text = change
                    yield row
                yield from self._step_rows(
                    artifact,
                    index,
                    step_row["step_type"],
                    step_row["status"],
                    step,
                    session_id,
                    project_path,
                    branch,
                    model,
                    opts,
                )
                yield from self._permission_rows(
                    artifact, index, step, step_row["permissions"], session_id, project_path, branch
                )
            if bad:
                row = self._row(artifact, 0, session_id, project_path, branch)
                row.turn_type = "system"
                row.text = "parser: %d step(s) could not be decoded" % bad
                yield row

    def _row(
        self, artifact: Artifact, index: int, session_id: str, project_path: str, branch: str
    ) -> Row:
        row = self.base_row(artifact)
        row.source_line = index
        row.session_id = session_id
        row.project_path = project_path
        row.git_branch = branch
        return row

    def _permission_rows(
        self,
        artifact: Artifact,
        index: int,
        step: dict,
        column: bytes | None,
        session_id: str,
        project_path: str,
        branch: str,
    ) -> Iterator[Row]:
        """Yield one `system` row per grant in a step's permissions.

        The permissions are the `steps.permissions` column, else Step field 133 of the
        payload.
        """
        permissions = step.get("permissions") or {}
        if column:
            try:
                permissions = decode(column, "TrajectoryPermissions", SCHEMA)
            except ValueError:
                row = self._row(artifact, index, session_id, project_path, branch)
                row.turn_type = "system"
                row.text = "parser: the permissions of step %d could not be decoded" % index
                yield row
                return
        texts = permission_texts(permissions)
        if not texts:
            return
        metadata = step.get("metadata") or {}
        tool_call = metadata.get("tool_call") or {}
        timestamp = (
            metadata.get("completed_at")
            or metadata.get("finished_generating_at")
            or metadata.get("created_at", "")
        )
        for text in texts:
            row = self._row(artifact, index, session_id, project_path, branch)
            row.timestamp_utc = timestamp
            row.turn_type = "system"
            row.tool_name = tool_call.get("name") or tool_call.get("original_name", "")
            row.tool_use_id = tool_call.get("id", "")
            row.text = compact(text)
            yield row

    def _step_rows(
        self,
        artifact: Artifact,
        index: int,
        step_type: int,
        status: int,
        step: dict,
        session_id: str,
        project_path: str,
        branch: str,
        model: str,
        opts: Options,
    ) -> Iterator[Row]:
        metadata = step.get("metadata") or {}
        created = metadata.get("created_at", "")
        completed = (
            metadata.get("completed_at") or metadata.get("finished_generating_at") or created
        )
        kind = next((k for k in PAYLOAD_KINDS if k in step), "")
        payload = step.get(kind) or {}
        status_name = STEP_STATUS.get(status, str(status))
        type_name = STEP_TYPES.get(step_type, str(step_type))

        def base(timestamp: str) -> Row:
            row = self._row(artifact, index, session_id, project_path, branch)
            row.timestamp_utc = timestamp
            row.model = model
            return row

        if kind == "user_input":
            row = base(created)
            row.turn_type = (
                "system" if metadata.get("source") == STEP_SOURCE_USER_IMPLICIT else "user"
            )
            text = payload.get("query") or payload.get("user_response") or ""
            if payload.get("is_queued_message"):
                text = "[queued] " + text
            row.text = compact(text)
            yield row
            return

        if kind == "planner_response":
            thinking = payload.get("raw_thinking") or payload.get("thinking")
            if thinking and opts.include_thinking:
                row = base(created)
                row.turn_type = "thinking"
                row.text = compact(thinking)
                yield row
            text = payload.get("modified_response") or payload.get("response")
            if text:
                row = base(completed)
                row.turn_type = "assistant"
                row.text = compact(text)
                yield row
            for tool_call in payload.get("tool_calls", []):
                row = base(completed)
                row.turn_type = "tool_use"
                row.tool_name = tool_call.get("name") or tool_call.get("original_name", "")
                row.tool_use_id = tool_call.get("id", "")
                row.text = compact(tool_args_summary(tool_call.get("arguments_json", "")))
                yield row
            return

        tool_call = metadata.get("tool_call")
        text = describe_payload(kind, payload) if kind else "step %s" % type_name
        if status_name != "done":
            text = "[%s] %s" % (status_name, text)
        if kind == "run_command" and payload.get("ran_in_sandbox"):
            text += " sandbox=true"
        if tool_call:
            row = base(completed)
            row.turn_type = "tool_result"
            row.tool_name = tool_call.get("name") or tool_call.get("original_name", "")
            row.tool_use_id = tool_call.get("id", "")
        else:
            row = base(created)
            row.turn_type = "system"
        row.text = compact(text)
        yield row
        if kind in ("run_command", "mcp_tool") and payload.get("user_rejected"):
            # The user's rejection of the call as its own row, beside the
            # result that also carries `[rejected by user]`.
            decision = base(row.timestamp_utc)
            decision.turn_type = "system"
            decision.tool_name = row.tool_name or (payload.get("tool_call") or {}).get("name", "")
            decision.tool_use_id = row.tool_use_id
            decision.text = "permission decision: user_rejected"
            yield decision
