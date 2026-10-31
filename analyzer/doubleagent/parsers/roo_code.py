"""Parse Roo Code tasks, their permission decisions, standing settings and CLI history.

Follows `research/roo-code.md`. Validated against source, RooCodeInc/Roo-Code at
b867ec91 (Roo Code Cloud keys at v3.53.0, 44f6bba2), with a synthetic fixture;
no real install.

Roo Code forked Cline and keeps its per-task layout, so the task files are
read by the shared mapping of `cline_legacy.py`, whose docstring lists their
fields. Roots: the extension's `User/globalStorage/rooveterinaryinc.roo-cline/`
and the CLI's `.vscode-mock/global-storage/`. Files read:

- `tasks/<taskId>/` transcript files, as in Cline; `ui_messages.json` asks
  with `isAnswered: true` and API history results opening with
  `formatResponse`'s `{"status": ...}` JSON give `permission decision:` rows.
- `tasks/<taskId>/history_item.json` (`HistoryItem`; `workspace` is the
  project path, `mode` gives a `permission mode:` row) and `tasks/_index.json`
  (only entries whose `history_item.json` is gone).
- Global state (the `EXTENSION_KEYS` row of `state.vscdb` or the CLI's
  `global-state.json`): `STANDING_KEYS` as `permission setting:` rows, and
  the Roo Code Cloud settings cache (`user-settings`, `ORG_SYNC_KEYS`) as
  `sync setting:` rows. Workspace state: `roo.retryQueue[]`, one
  `cloud upload queued:` row per request, never its headers or body.
- `.roo/cli-settings.json` (`CLI_SETTING_KEYS`), `settings/mcp_settings.json`
  and a project's `.roo/mcp.json` (`mcpServers.<name>.alwaysAllow` only).
- `.roo/cli-history.json` (`entries[]`), a secondary source (a file repeating
  the tasks' prompts): one `user` row per entry, with no time, task or project.

No model name or git branch is recorded (`apiConfigName` is a profile name).
"""

from __future__ import annotations

import dataclasses
import json
import re
from collections.abc import Iterator
from pathlib import Path

from ..inputs import Artifact
from ..model import Row, compact
from ..timeutil import to_utc
from ..vscode_state import EDITOR_AGENTS, STATE_DATABASE_PATTERN, read_items
from .base import Options, error_rows, file_mtime, setting_value, text_of
from .cline_legacy import (
    API_FILES,
    UI_FILE,
    LegacyTaskParser,
    iter_array_at,
    iter_json_array,
    load_json,
    read_text,
)

ROOT = r"(?:(?:.*/)?User/globalStorage/rooveterinaryinc\.roo-cline|\.vscode-mock/global-storage)"
ITEM_PATTERN = re.compile(r"^%s/tasks/[^/]+/history_item\.json$" % ROOT, re.I)
INDEX_PATTERN = re.compile(r"^%s/tasks/_index\.json$" % ROOT, re.I)
MCP_PATTERN = re.compile(r"^%s/settings/mcp_settings\.json$" % ROOT, re.I)
CLI_HISTORY_REL = ".roo/cli-history.json"
CLI_SETTINGS_REL = ".roo/cli-settings.json"
PROJECT_MCP_REL = ".roo/mcp.json"
GLOBAL_STATE_REL = ".vscode-mock/global-storage/global-state.json"
WORKSPACE_STATE_PATTERN = re.compile(
    r"^\.vscode-mock/workspace-storage/[^/]+/workspace-state\.json$"
)
WORKSPACE_DATABASE_PATTERN = re.compile(r"(?:^|/)User/workspaceStorage/[^/]+/state\.vscdb$")
ENTRIES_KEY_PATTERN = re.compile(r'"entries"[ \t\n\r]*:[ \t\n\r]*(?=\[)')
ITEM_FILE = "history_item.json"

# The extension's global and workspace state is one `ItemTable` row keyed
# by `<publisher>.<name>` (src/package.json: `RooVeterinaryInc`,
# `roo-cline`); the lower-case form is read too in case an editor
# normalises it.
EXTENSION_KEYS = ("RooVeterinaryInc.roo-cline", "rooveterinaryinc.roo-cline")
# Global-state keys that grant or limit what runs without asking
# (packages/types/src/global-settings.ts:97-118 and :192 at b867ec91), and
# the two VS Code settings the CLI's shim keeps in the same file.
STANDING_KEYS = (
    "autoApprovalEnabled",
    "alwaysAllowReadOnly",
    "alwaysAllowReadOnlyOutsideWorkspace",
    "alwaysAllowWrite",
    "alwaysAllowWriteOutsideWorkspace",
    "alwaysAllowWriteProtected",
    "alwaysAllowMcp",
    "alwaysAllowModeSwitch",
    "alwaysAllowSubtasks",
    "alwaysAllowExecute",
    "alwaysAllowFollowupQuestions",
    "allowedCommands",
    "deniedCommands",
    "commandTimeoutAllowlist",
    "allowedMaxRequests",
    "allowedMaxCost",
    "mode",
    "roo-cline.allowedCommands",
    "roo-cline.deniedCommands",
)
CLI_SETTING_KEYS = ("requireApproval", "dangerouslySkipPermissions", "mode")
# Roo Code Cloud (releases up to 3.53.0): cached account settings that
# decide whether task messages were uploaded.
ORG_SYNC_KEYS = (
    "recordTaskMessages",
    "enableTaskSharing",
    "allowPublicTaskSharing",
    "taskShareExpirationDays",
    "allowMembersViewAllTasks",
    "workspaceTaskVisibility",
)
RETRY_QUEUE_KEY = "roo.retryQueue"


class RooCodeParser(LegacyTaskParser):
    """Parse Roo Code's task files, editor state, settings and CLI history."""

    agent = "roo-code"
    name = "roo-code"
    has_secondary = True
    # The extension's global and workspace state are rows of the editor's
    # state.vscdb, which the catalog attributes to the editor.
    reads_agents = EDITOR_AGENTS
    ROOT = ROOT
    PROJECT_KEY = "workspace"

    def wants(self, artifact: Artifact) -> bool:
        """Return True for a task, index, settings, state or CLI history file."""
        rel = artifact.rel
        return bool(
            self.task_match(artifact)
            or ITEM_PATTERN.match(rel)
            or INDEX_PATTERN.match(rel)
            or MCP_PATTERN.match(rel)
            or WORKSPACE_STATE_PATTERN.match(rel)
            or rel in (CLI_HISTORY_REL, CLI_SETTINGS_REL, PROJECT_MCP_REL, GLOBAL_STATE_REL)
            or STATE_DATABASE_PATTERN.search(rel)
            or WORKSPACE_DATABASE_PATTERN.search(rel)
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for the CLI's input history, which repeats task prompts."""
        return artifact.rel == CLI_HISTORY_REL

    def history_item(self, task_dir: Path, root: Path, task_id: str) -> dict | None:
        """Return the task's `history_item.json`, else its `_index.json` entry, else None."""
        item = load_json(task_dir / ITEM_FILE)
        if isinstance(item, dict):
            return item
        index = self.cached_json(root / "tasks" / "_index.json")
        entries = index.get("entries") if isinstance(index, dict) else None
        if isinstance(entries, list):
            for entry in entries:
                if isinstance(entry, dict) and str(entry.get("id")) == task_id:
                    return entry
        return None

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Roo Code file."""
        rel = artifact.rel
        if self.task_match(artifact):
            yield from self.parse_task_file(artifact, opts)
        elif ITEM_PATTERN.match(rel):
            item = load_json(artifact.disk_path)
            if isinstance(item, dict):
                yield from self._history_rows(artifact, item, 1, opts)
            else:
                yield from error_rows(
                    self,
                    artifact,
                    [(0, "history_item.json is not a JSON object", "")],
                    artifact.disk_path.parent.name,
                )
        elif INDEX_PATTERN.match(rel):
            yield from self._parse_index(artifact, opts)
        elif rel == CLI_HISTORY_REL:
            yield from self._parse_cli_history(artifact)
        elif MCP_PATTERN.match(rel) or rel == PROJECT_MCP_REL:
            yield from self._parse_mcp(artifact)
        elif rel == CLI_SETTINGS_REL:
            yield from self._parse_cli_settings(artifact)
        elif rel == GLOBAL_STATE_REL:
            state = self._json_object(artifact)
            if state is None:
                yield self._bad_file(artifact, "global-state.json is not a JSON object")
            else:
                yield from self._global_state_rows(artifact, state, rel)
        elif WORKSPACE_STATE_PATTERN.match(rel):
            state = self._json_object(artifact)
            if state is None:
                yield self._bad_file(artifact, "workspace-state.json is not a JSON object")
            else:
                yield from self._retry_queue_rows(artifact, state, rel)
        elif STATE_DATABASE_PATTERN.search(rel) or WORKSPACE_DATABASE_PATTERN.search(rel):
            yield from self._parse_state_database(artifact)

    # ---- per-call permission decisions ---------------------------------------

    def parse_task_file(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield a task file's shared rows, with the permission decisions beside them."""
        name = artifact.disk_path.name.lower()
        if name == UI_FILE:
            yield from self._ui_with_decisions(artifact, opts)
        elif name in API_FILES:
            yield from self._api_with_decisions(artifact, opts)
        else:
            yield from super().parse_task_file(artifact, opts)

    def _ui_with_decisions(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the shared `ui_messages.json` rows with each approval after its call.

        A `tool` ask marked `isAnswered: true` was approved by a click or
        automatically. The pre-scan's errors are dropped: the shared reader
        reports them.
        """
        answered = {
            index
            for index, message in iter_json_array(artifact.disk_path, [])
            if isinstance(message, dict)
            and message.get("type") == "ask"
            and message.get("ask") == "tool"
            and message.get("isAnswered") is True
        }
        for row in self._parse_ui(artifact, self.context(artifact), opts):
            yield row
            if row.turn_type == "tool_use" and row.source_line in answered:
                answered.discard(row.source_line)
                yield system_like(row, APPROVED_TEXT)

    def _api_with_decisions(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the API history's permission decisions, with its rows when it is alone.

        Beside `ui_messages.json` only the decision rows and the file's
        bad-record rows are yielded, since the transcript rows come from that
        file; alone, the shared rows come with each decision after its
        `tool_result` row.
        """
        context = self.context(artifact)
        if (artifact.disk_path.parent / UI_FILE).is_file():
            errors: list = []
            for index, timestamp, tool_use_id, name, text in api_decisions(
                artifact.disk_path, errors
            ):
                row = self._row(artifact, context, index, to_utc(timestamp) or context.start)
                row.tool_use_id = tool_use_id
                row.tool_name = name
                yield system_like(row, text)
            yield from error_rows(self, artifact, errors, context.task_id, context.project_path)
            return
        pending: dict[str, list[tuple[str, str]]] = {}
        for _index, _timestamp, tool_use_id, name, text in api_decisions(artifact.disk_path, []):
            pending.setdefault(tool_use_id, []).append((name, text))
        for row in self._parse_api(artifact, context, opts):
            yield row
            if row.turn_type == "tool_result" and pending.get(row.tool_use_id):
                name, text = pending[row.tool_use_id].pop(0)
                decision = system_like(row, text)
                decision.tool_name = decision.tool_name or name
                yield decision

    # ---- task index entries ---------------------------------------------------

    def _history_rows(
        self, artifact: Artifact, item: dict, index: int, opts: Options
    ) -> Iterator[Row]:
        """Yield the shared task row, then `permission mode: <mode>` from its `mode`.

        The mode slug decides the tool groups the task may use.
        """
        row = self.history_row(artifact, item, index, opts)
        yield row
        mode = item.get("mode")
        if isinstance(mode, str) and mode:
            yield system_like(row, "permission mode: %s" % mode)

    def _parse_cli_history(self, artifact: Artifact) -> Iterator[Row]:
        errors: list = []
        data = load_json(artifact.disk_path)
        if isinstance(data, dict) and isinstance(data.get("entries"), list):
            entries = enumerate(data["entries"], 1)
        else:
            # Cut mid-write (the CLI rewrites the whole file per prompt): walk
            # the entries array as far as it parses.
            text = read_text(artifact.disk_path) if data is None else None
            match = ENTRIES_KEY_PATTERN.search(text) if text is not None else None
            if match and text is not None:
                entries = iter_array_at(text, match.end(), errors)
            else:
                errors.append((0, "cli-history.json has no entries array", ""))
                entries = iter(())
        for index, entry in entries:
            if not isinstance(entry, str) or not entry.strip():
                continue
            row = self.base_row(artifact)
            row.source_line = index
            row.turn_type = "user"
            row.text = entry
            yield row
        if data is None and not errors:
            errors.append((0, "cli-history.json unparseable after its entries array", ""))
        yield from error_rows(self, artifact, errors)

    def _parse_index(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        index = load_json(artifact.disk_path)
        if not isinstance(index, dict) or not isinstance(index.get("entries"), list):
            yield from error_rows(self, artifact, [(0, "_index.json has no entries array", "")])
            return
        tasks = artifact.disk_path.parent
        for position, entry in enumerate(index["entries"], 1):
            if not isinstance(entry, dict):
                continue
            task_id = str(entry.get("id") or "")
            if task_id and (tasks / task_id / ITEM_FILE).is_file():
                continue
            yield from self._history_rows(artifact, entry, position, opts)

    # ---- standing settings ----------------------------------------------------

    @staticmethod
    def _json_object(artifact: Artifact) -> dict | None:
        data = load_json(artifact.disk_path)
        return data if isinstance(data, dict) else None

    def _bad_file(self, artifact: Artifact, message: str) -> Row:
        row = self.base_row(artifact)
        row.turn_type = "system"
        row.timestamp_utc = file_mtime(artifact)
        row.text = "parser: " + message
        return row

    def _setting_row(
        self, artifact: Artifact, label: str, key: str, value, source: str, project: str = ""
    ) -> Row:
        row = self.base_row(artifact)
        row.source_line = 1
        row.turn_type = "system"
        row.timestamp_utc = file_mtime(artifact)
        row.project_path = project
        row.text = compact(
            "%s: %s=%s (from %s, file mtime)" % (label, key, setting_value(value), source)
        )
        return row

    def _settings_rows(self, artifact: Artifact, doc: dict, keys, source: str) -> Iterator[Row]:
        """One `permission setting:` row per scalar, or per entry of a list."""
        for key in keys:
            value = doc.get(key)
            if value is None:
                continue
            for item in value if isinstance(value, list) else [value]:
                yield self._setting_row(artifact, "permission setting", key, item, source)

    def _global_state_rows(self, artifact: Artifact, state: dict, source: str) -> Iterator[Row]:
        """Yield the standing permission settings, then the Roo Code Cloud settings cache.

        The cache (releases up to 3.53.0) exists only while the extension was
        signed in and says whether task messages were uploaded.
        """
        yield from self._settings_rows(artifact, state, STANDING_KEYS, source)
        cloud_source = "%s, Roo Code Cloud settings cache" % source
        user = state.get("user-settings")
        if isinstance(user, dict):
            config = user.get("settings")
            value = config.get("taskSyncEnabled") if isinstance(config, dict) else None
            yield self._setting_row(
                artifact,
                "sync setting",
                "user-settings.settings.taskSyncEnabled",
                "unset (defaults to true)" if value is None else value,
                cloud_source,
            )
        organization = state.get("organization-settings")
        cloud = organization.get("cloudSettings") if isinstance(organization, dict) else None
        if isinstance(cloud, dict):
            for key in ORG_SYNC_KEYS:
                if cloud.get(key) is not None:
                    yield self._setting_row(
                        artifact,
                        "sync setting",
                        "organization-settings.cloudSettings." + key,
                        cloud[key],
                        cloud_source,
                    )

    def _parse_cli_settings(self, artifact: Artifact) -> Iterator[Row]:
        doc = self._json_object(artifact)
        if doc is None:
            yield self._bad_file(artifact, "cli-settings.json is not a JSON object")
            return
        yield from self._settings_rows(artifact, doc, CLI_SETTING_KEYS, artifact.rel)

    def _parse_mcp(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one row per tool name of `mcpServers.<name>.alwaysAllow`.

        Nothing else is read: the server entries can hold `env` and `headers`.
        """
        doc = self._json_object(artifact)
        if doc is None:
            yield self._bad_file(artifact, "%s is not a JSON object" % artifact.disk_path.name)
            return
        project = artifact.home.original if artifact.home.project else ""
        servers = doc.get("mcpServers")
        for name, config in servers.items() if isinstance(servers, dict) else ():
            tools = config.get("alwaysAllow") if isinstance(config, dict) else None
            for tool in tools if isinstance(tools, list) else ():
                yield self._setting_row(
                    artifact,
                    "permission setting",
                    "mcpServers.%s.alwaysAllow" % name,
                    tool,
                    artifact.rel,
                    project,
                )

    def _parse_state_database(self, artifact: Artifact) -> Iterator[Row]:
        values, problems = read_items(artifact.disk_path, EXTENSION_KEYS)
        workspace = bool(WORKSPACE_DATABASE_PATTERN.search(artifact.rel))
        for key in EXTENSION_KEYS:
            state = values.get(key)
            if state is None:
                continue
            if not isinstance(state, dict):
                if not any(key in problem for problem in problems):
                    problems.append("%s value is not a JSON object" % key)
                continue
            source = "%s ItemTable %s" % (artifact.rel, key)
            if workspace:
                yield from self._retry_queue_rows(artifact, state, source)
            else:
                yield from self._global_state_rows(artifact, state, source)
        for problem in problems:
            # never echo the value: the workspace state's retry queue holds a
            # bearer token
            yield self._bad_file(artifact, problem.split(" (", 1)[0])

    # ---- Roo Code Cloud retry queue -------------------------------------------

    def _retry_queue_rows(self, artifact: Artifact, state: dict, source: str) -> Iterator[Row]:
        """Yield one row per request the Roo Code Cloud client queued after a failed upload.

        The request's headers carry a bearer token and its body the event, so
        only the event type, the task id, the URL and the retry state are
        written.
        """
        queue = state.get(RETRY_QUEUE_KEY)
        for index, request in enumerate(queue if isinstance(queue, list) else [], 1):
            if not isinstance(request, dict):
                continue
            event = queued_event(request)
            properties = event.get("properties")
            task_id = properties.get("taskId") if isinstance(properties, dict) else None
            row = self.base_row(artifact)
            row.source_line = index
            row.turn_type = "system"
            row.timestamp_utc = to_utc(request.get("timestamp"))
            row.session_id = str(task_id or "")
            parts = [
                "cloud upload queued: %s" % (event.get("type") or request.get("operation") or "-"),
                "task=%s" % (task_id or "-"),
                "type=%s" % (request.get("type") or "-"),
                "retries=%s" % request.get("retryCount", "-"),
                "url=%s" % (request.get("url") or "-"),
            ]
            if request.get("lastError"):
                parts.append("lastError=%s" % request["lastError"])
            row.text = compact(
                "%s (Roo Code Cloud retry queue, from %s %s, request timestamp)"
                % (" ".join(parts), source, RETRY_QUEUE_KEY)
            )
            yield row


APPROVED_TEXT = "permission decision: approved isAnswered=true (user or auto-approval)"


def system_like(like: Row, text: str) -> Row:
    """Return a `system` row with `text` and every other field of `like`."""
    return dataclasses.replace(like, turn_type="system", text=compact(text))


def queued_event(request: dict) -> dict:
    """Return the JSON event body of a queued request, or {} when it is not a JSON object."""
    options = request.get("options")
    body = options.get("body") if isinstance(options, dict) else None
    if not isinstance(body, str):
        return {}
    try:
        event = json.loads(body)
    except ValueError:
        return {}
    return event if isinstance(event, dict) else {}


def api_decisions(path: Path, errors: list) -> Iterator[tuple[int, object, str, str, str]]:
    """Yield each permission decision a `tool_result` of the API history records.

    The result's text opens with the JSON `formatResponse` writes for a denial
    or an approval with feedback: `{"status":"denied","message":...}`,
    `{"status":"denied","feedback":...}` or `{"status":"approved",
    "feedback":...}`, the last followed by the tool output.

    Yields:
        (record position, record `ts`, tool_use_id, tool name, row text).
    """
    names: dict[str, str] = {}
    decoder = json.JSONDecoder()
    for index, message in iter_json_array(path, errors):
        if not isinstance(message, dict) or not isinstance(message.get("content"), list):
            continue
        for block in message["content"]:
            if not isinstance(block, dict):
                continue
            if block.get("type") == "tool_use":
                names[str(block.get("id") or "")] = str(block.get("name") or "")
                continue
            if block.get("type") != "tool_result":
                continue
            body = text_of(block.get("content")).lstrip()
            if not body.startswith("{"):
                continue
            try:
                status, _end = decoder.raw_decode(body)
            except ValueError:
                continue
            if not isinstance(status, dict) or status.get("status") not in ("denied", "approved"):
                continue
            text = "permission decision: %s" % status["status"]
            if status.get("feedback") is not None:
                text += " feedback=%s" % setting_value(status["feedback"])
            tool_use_id = str(block.get("tool_use_id") or "")
            yield index, message.get("ts"), tool_use_id, names.get(tool_use_id, ""), text
