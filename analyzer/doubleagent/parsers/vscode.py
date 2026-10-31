"""Parse VS Code chat sessions (GitHub Copilot Chat and other participants) and chat state.

Follows `research/vscode.md`. Validated against source, microsoft/vscode at
d7622a529314 and 4d22f4f922eb (model options, permission levels, standing
grants, settings, the GitHub account and the chat editing state), with a
synthetic fixture; no real install. The store is VS Code's own, so the parser
covers every data directory the catalog attributes to `vscode` (`USER_DIR`).
Files read, under `User/`:

- `workspaceStorage/<hash>/chatSessions/<id>.jsonl` (1.109 and later) or
  `.json`, `globalStorage/emptyWindowChatSessions/` and
  `globalStorage/transferredChatSessions/`. A `.jsonl` file is a mutation
  log (`kind` 0 sets, 1 sets at path `k`, 2 pushes after truncating to `i`,
  3 deletes) replayed by `apply_mutation`. Session fields: `sessionId`,
  `creationDate`, `responderUsername`, `initialLocation`, `customTitle`,
  `workingDirectory`, `requests[]`; request fields: `timestamp`, `message`,
  `variableData`, `modelId`, `modelConfiguration`, `modeInfo`,
  `confirmation`, `isSystemInitiated`, `isCanceled`, `response[]`,
  `responseTimestamp`, `result.errorDetails`. The project is
  `workingDirectory`, else the sibling `workspace.json` `folder`.
- `globalStorage/github.copilot-chat/cloudSessions.json` and the
  `GitHub.copilot-chat` row of `state.vscdb` (`ownedTasks`,
  `userSelectedRepositories`): `session synced:`, `session origin:` and
  `source host repository:` rows, so `records_sync` and `records_origin` are set.
- Every profile's and workspace's `state.vscdb`: the `GRANT_KEYS`,
  `OPT_IN_KEY` and `ALLOWLIST_KEY` grants and, in a profile, the GitHub
  account usage keys (`USAGE_KEY_PATTERN`); `secret://` rows are never read.
- A profile's `settings.json` (JSONC): the auto-approve keys and the default model.
- `chatEditingSessions/<key>/state.json`, a secondary source (a file
  repeating which files a request edited) that survives the chat's delete.
"""

from __future__ import annotations

import json
import re
import sqlite3
from collections.abc import Iterator
from pathlib import Path

from .. import sqlite_util
from ..errors import DoubleAgentError
from ..inputs import Artifact
from ..model import Row, compact
from ..timeutil import to_utc
from ..vscode_state import read_items
from .antigravity import uri_to_path
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_mtime,
    line_text,
    setting_value,
    track_change,
)
from .gemini_cli import parse_jsonc

USER_DIR = (
    r"^(?:(?:\.config|Library/Application Support|AppData/Roaming)/(?:Code[^/]*|VSCodium[^/]*|Positron|Trae[^/]*)"
    r"|\.vscode-server[^/]*/data|\.vscodium-server[^/]*/data|\.positron-server/data)/User/"
)
SESSION_PATTERN = re.compile(
    USER_DIR + r"(?:workspaceStorage/[^/]+/chatSessions/[^/]+\.jsonl?"
    r"|globalStorage/emptyWindowChatSessions/[^/]+\.jsonl?"
    r"|globalStorage/transferredChatSessions/[^/]+\.json)$"
)

CLOUD_SESSIONS_PATTERN = re.compile(
    USER_DIR + r"globalStorage/github\.copilot-chat/cloudSessions\.json$"
)
# A `state.vscdb` of the default profile (`globalStorage/`, also the
# application scope), of another profile (`profiles/<id>/globalStorage/`,
# the Agents window's under `profiles/builtin/agents/`) or of a workspace
# (`workspaceStorage/<hash>/`).
STATE_PATTERN = re.compile(
    USER_DIR + r"(?P<scope>globalStorage|profiles/(?:builtin/)?[^/]+/globalStorage"
    r"|workspaceStorage/[^/]+)/state\.vscdb$"
)
SETTINGS_PATTERN = re.compile(USER_DIR + r"(?:profiles/(?:builtin/)?[^/]+/)?settings\.json$")
EDITING_PATTERN = re.compile(
    USER_DIR + r"workspaceStorage/[^/]+/chatEditingSessions/(?P<key>[^/]+)/state\.json$"
)
COPILOT_STATE_KEY = "GitHub.copilot-chat"
OWNED_TASKS_KEY = "github.copilot.cloudAgent.ownedTasks"
SELECTED_REPOS_KEY = "userSelectedRepositories"
# `ToolConfirmStore` keys (languageModelToolsConfirmationService.ts:240-244
# at 4d22f4f), each in workspace and profile scope.
GRANT_KEYS = (
    "chat/autoconfirm",
    "chat/autoconfirm-post",
    "chat/servers/autoconfirm",
    "chat/servers/autoconfirm-post",
    "chat/autoconfirm-combination",
)
OPT_IN_KEY = "chat.tools.global.autoApprove.optIn"
ALLOWLIST_KEY = "chat.externalPath.workspaceAllowlist"
# `<providerId>-<account label>-usages` and `-mcpserver-usages`, application
# scope (authenticationUsageService.ts:103, authenticationMcpUsageService.ts:101).
USAGE_KEY_PATTERN = re.compile(r"^(github(?:-enterprise)?)-(.+?)(-mcpserver)?-usages$")
SETTINGS_PERMISSION_PATTERN = re.compile(
    r"^chat\.tools\..*[Aa]utoApprov|^chat\.permissions\.default$"
)
DEFAULT_MODEL_KEY = "chat.defaultModel"
# `ModifiedFileEntryState`, a const enum serialised as its index
# (chatEditingService.ts:353-357 at 4d22f4f).
EDIT_STATES = ("Modified", "Accepted", "Rejected")


class MutationError(DoubleAgentError):
    """A mutation-log record that cannot be applied to the session state."""


def apply_mutation(state, record: dict):
    """Apply one mutation-log record to `state` and return the new state."""
    kind = record.get("kind")
    if kind == 0:
        return record.get("v")
    path = record.get("k")
    if not isinstance(path, list) or (not path and kind != 2):
        raise MutationError("bad path %r" % (path,))
    if kind == 2 and not path:
        target = state
    else:
        parent = _walk(state, path[:-1])
        key = path[-1]
        if kind == 1:
            _set(parent, key, record.get("v"))
            return state
        if kind == 3:
            try:
                del parent[key]
            except (KeyError, IndexError, TypeError):
                pass
            return state
        if kind != 2:
            raise MutationError("unknown kind %r" % (kind,))
        try:
            target = parent[key]
        except (KeyError, IndexError, TypeError):
            target = None
        if target is None:
            target = []
            _set(parent, key, target)
    if not isinstance(target, list):
        raise MutationError("push to non-list at %r" % (path,))
    if isinstance(record.get("i"), int):
        del target[record["i"] :]
    items = record.get("v")
    target.extend(items if isinstance(items, list) else [items])
    return state


def _walk(state, path):
    current = state
    for key in path:
        try:
            current = current[key]
        except (KeyError, IndexError, TypeError):
            raise MutationError("path %r not found" % (path,)) from None
    return current


def _set(parent, key, value):
    if isinstance(parent, list) and isinstance(key, int):
        if key == len(parent):
            parent.append(value)
        elif 0 <= key < len(parent):
            parent[key] = value
        else:
            raise MutationError("index %r out of range" % (key,))
    elif isinstance(parent, dict):
        parent[key] = value
    else:
        raise MutationError("cannot set %r" % (key,))


def md_text(value) -> str:
    """Return the text of an IMarkdownString (`{value}`), a plain string or a list of either."""
    if value is None:
        return ""
    if isinstance(value, str):
        return value
    if isinstance(value, dict):
        text = value.get("value")
        return text if isinstance(text, str) else ""
    if isinstance(value, list):
        return "\n".join(md_text(item) for item in value)
    return str(value)


def uri_text(uri) -> str:
    """Return a URI as a path or URI string.

    The URI is serialized (`{scheme, path, ...}` or `{$mid, fsPath}`), a URI
    string, or a location `{uri, range}`.
    """
    if isinstance(uri, str):
        return uri_to_path(uri)
    if isinstance(uri, dict):
        if "uri" in uri:
            return uri_text(uri["uri"])
        for key in ("fsPath", "path", "external"):
            if isinstance(uri.get(key), str):
                return uri[key]
    return compact_json(uri) if uri is not None else ""


def tool_call_text(part: dict) -> str:
    """Return a tool invocation's command line or raw input, else its invocation message."""
    specific_data = part.get("toolSpecificData")
    if isinstance(specific_data, dict):
        command_line = specific_data.get("commandLine")
        if isinstance(command_line, dict) and (
            command_line.get("userEdited") or command_line.get("original")
        ):
            return str(command_line.get("userEdited") or command_line.get("original"))
        if "rawInput" in specific_data:
            raw = specific_data["rawInput"]
            return raw if isinstance(raw, str) else compact_json(raw)
    result_details = part.get("resultDetails")
    if (
        isinstance(result_details, dict)
        and isinstance(result_details.get("input"), str)
        and result_details["input"]
    ):
        return result_details["input"]
    return md_text(part.get("invocationMessage"))


# `ToolConfirmKind`, a const enum serialised as its number
# (chatService.ts:870-877 at d7622a529314).
TOOL_CONFIRM_KINDS = (
    "Denied",
    "ConfirmationNotNeeded",
    "Setting",
    "LmServicePerTool",
    "UserAction",
    "Skipped",
)


def confirmed_text(part: dict) -> str:
    """Return the `permission decision:` text of a tool invocation's `isConfirmed`, else ""."""
    confirmed = part.get("isConfirmed")
    if confirmed is None:
        return ""
    if isinstance(confirmed, dict):
        kind = confirmed.get("type")
        if (
            isinstance(kind, int)
            and not isinstance(kind, bool)
            and 0 <= kind < len(TOOL_CONFIRM_KINDS)
        ):
            value = "%s(%d)" % (TOOL_CONFIRM_KINDS[kind], kind)
        else:
            value = setting_value(kind)
        pairs = [
            "%s=%s" % (key, setting_value(item))
            for key, item in confirmed.items()
            if key not in ("type", "reason") and item is not None
        ]
        reason = md_text(confirmed.get("reason"))
    else:
        value, pairs, reason = setting_value(confirmed), [], ""
    source = part.get("source")
    if isinstance(source, dict) and source.get("type"):
        pairs.append("tool_source=%s" % setting_value(source["type"]))
    text = " ".join(["permission decision: confirmed=" + value, *pairs])
    return "%s reason=%s" % (text, reason) if reason else text


def confirmation_part_text(part: dict) -> str:
    """Return the `permission decision:` text of a `confirmation` response part."""
    pairs = ["permission decision: confirmation"]
    if "isUsed" in part:
        pairs.append("isUsed=%s" % setting_value(bool(part.get("isUsed"))))
    if isinstance(part.get("buttons"), list):
        pairs.append("buttons=%s" % compact_json(part["buttons"]))
    if part.get("title"):
        pairs.append("title=%s" % setting_value(part["title"]))
    message = md_text(part.get("message"))
    return " ".join(pairs) + ("\n" + message if message else "")


def tool_result_text(part: dict) -> str:
    """Return a tool invocation's result text, `[error] ` first when it failed."""
    result_details = part.get("resultDetails")
    text = ""
    is_error = bool(part.get("resultError"))
    if isinstance(result_details, dict):
        out = result_details.get("output")
        if isinstance(out, list):
            values = []
            for output in out:
                if isinstance(output, dict):
                    value = output.get("value", output.get("text"))
                    values.append(value if isinstance(value, str) else compact_json(value))
                elif isinstance(output, str):
                    values.append(output)
            text = "\n".join(values)
        elif out is not None:
            text = out if isinstance(out, str) else compact_json(out)
        is_error = is_error or bool(result_details.get("isError"))
    elif isinstance(result_details, list):
        text = "\n".join(uri_text(uri) for uri in result_details)
    if part.get("resultError"):
        error = part["resultError"]
        text = "\n".join(
            chunk
            for chunk in (md_text(error) if not isinstance(error, bool) else "", text)
            if chunk
        )
    if not text:
        text = md_text(part.get("pastTenseMessage"))
    if not text and part.get("isComplete") is False:
        text = "[incomplete]"
    return ("[error] " + text) if is_error else text


def user_rel(rel: str) -> str:
    """Return `rel` from its `User/` component on, as row text names the file."""
    position = rel.find("/User/")
    return rel[position + 1 :] if position >= 0 else rel


def grant_texts(values: dict, problems: list) -> Iterator[str]:
    """Yield the `permission setting:` texts, without the source, of one `state.vscdb`'s grants.

    One per entry of each `ToolConfirmStore` key (an object keyed by tool or
    server id whose value is `true`, a label or `{label, arguments}`, or the
    older array of ids), the global auto-approve opt-in, and one per
    external path allowed in the workspace. A value of another JSON type is
    added to `problems`.
    """
    for key in GRANT_KEYS:
        value = values.get(key)
        if value is None or isinstance(value, str):
            continue  # absent, or not JSON (read_items reported it)
        if isinstance(value, dict):
            for tool_id, entry in value.items():
                yield "permission setting: %s.%s=%s" % (key, tool_id, setting_value(entry))
        elif isinstance(value, list):
            for tool_id in value:
                yield "permission setting: %s.%s=true" % (key, setting_value(tool_id))
        else:
            problems.append("ItemTable value for %s is not a JSON object or array" % key)
    if OPT_IN_KEY in values:
        yield "permission setting: %s=%s" % (OPT_IN_KEY, setting_value(values[OPT_IN_KEY]))
    allow = values.get(ALLOWLIST_KEY)
    if isinstance(allow, list):
        for entry in allow:
            text = uri_to_path(entry) if isinstance(entry, str) else setting_value(entry)
            yield "permission setting: %s=%s" % (ALLOWLIST_KEY, text)
    elif allow is not None and not isinstance(allow, str):
        problems.append("ItemTable value for %s is not a JSON array" % ALLOWLIST_KEY)


def setting_texts(key: str, value) -> Iterator[str]:
    """Yield the `permission setting:` texts of one `settings.json` key.

    A non-empty object value gives one per entry (`<key>.<entry>=<value>`),
    any other value one (`<key>=<value>`).
    """
    if isinstance(value, dict) and value:
        for entry, setting in value.items():
            yield "permission setting: %s.%s=%s" % (key, entry, setting_value(setting))
    else:
        yield "permission setting: %s=%s" % (key, setting_value(value))


def edit_state_text(entry: dict) -> str:
    """Return the `edit state:` text of one snapshot entry of a chat editing `state.json`."""
    state = entry.get("state")
    if isinstance(state, int) and not isinstance(state, bool) and 0 <= state < len(EDIT_STATES):
        name = EDIT_STATES[state]
    else:
        name = setting_value(state)
    info = entry.get("telemetryInfo")
    info = info if isinstance(info, dict) else {}
    text = "edit state: %s %s request=%s model=%s" % (
        name,
        uri_to_path(str(entry.get("resource") or "")) or "-",
        info.get("requestId") or "-",
        info.get("modelId") or "-",
    )
    if entry.get("isDeleted"):
        text += " deleted=true"
    return text + " (from chatEditingSessions state.json, file mtime)"


class VsCodeParser(Parser):
    """Parse VS Code chat sessions, cloud session markers, state databases and settings."""

    agent = "vscode"
    name = "vscode"
    records_sync = True
    records_origin = True
    has_secondary = True

    def wants(self, artifact: Artifact) -> bool:
        """Return True for a chat session, `cloudSessions.json`, state, settings or edit state."""
        rel = artifact.rel
        return bool(
            SESSION_PATTERN.match(rel)
            or CLOUD_SESSIONS_PATTERN.match(rel)
            or STATE_PATTERN.match(rel)
            or SETTINGS_PATTERN.match(rel)
            or EDITING_PATTERN.match(rel)
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for a chat editing state, which repeats which files a request edited."""
        return bool(EDITING_PATTERN.match(artifact.rel))

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one VS Code file."""
        if CLOUD_SESSIONS_PATTERN.match(artifact.rel):
            yield from self._parse_cloud_sessions(artifact)
            return
        if STATE_PATTERN.match(artifact.rel):
            yield from self._parse_state(artifact)
            return
        if SETTINGS_PATTERN.match(artifact.rel):
            yield from self._parse_settings(artifact)
            return
        if EDITING_PATTERN.match(artifact.rel):
            yield from self._parse_editing(artifact)
            return
        request_lines: dict[int, int] = {}
        errors: list = []
        if artifact.rel.endswith(".jsonl"):
            state = self._replay(artifact, request_lines, errors)
        else:
            try:
                with open(artifact.disk_path, "rb") as handle:
                    state = json.loads(handle.read().decode("utf-8", errors="replace"))
            except ValueError as error:
                state = None
                errors.append((0, "file is not valid JSON (%s)" % error, ""))
        if not isinstance(state, dict):
            state = {}
        session_id = str(state.get("sessionId") or Path(artifact.rel).name.rsplit(".", 1)[0])
        folder, workspace_file = self._workspace_folder(artifact, errors)
        project = uri_to_path(str(state.get("workingDirectory") or "")) or folder

        def base(turn_type: str, text: str, timestamp, line: int, model: str = "") -> Row:
            row = self.base_row(artifact)
            row.session_id = session_id
            row.project_path = project
            row.timestamp_utc = to_utc(timestamp)
            row.source_line = line
            row.model = model
            row.turn_type = turn_type
            row.text = compact(text)
            return row

        if state:
            yield base(
                "system",
                " ".join(
                    part
                    for part in (
                        "session start:",
                        "responder=%s" % state["responderUsername"]
                        if state.get("responderUsername")
                        else "",
                        "location=%s" % state["initialLocation"]
                        if state.get("initialLocation")
                        else "",
                        "title=%s" % state["customTitle"] if state.get("customTitle") else "",
                        "workspace=%s" % workspace_file if workspace_file else "",
                    )
                    if part
                ),
                state.get("creationDate"),
                request_lines.get(-1, 0),
            )
        requests = state.get("requests")
        if not isinstance(requests, list):
            requests = []
        seen: dict = {}
        for i, request in enumerate(requests):
            if isinstance(request, dict):
                yield from self._request_rows(request, request_lines.get(i, 0), base, opts, seen)
        yield from error_rows(self, artifact, errors, session_id, project)

    def _system_row(self, artifact: Artifact, session_id: str, timestamp: str, text: str) -> Row:
        row = self.base_row(artifact)
        row.session_id, row.timestamp_utc, row.turn_type = session_id, timestamp, "system"
        row.text = compact(text)
        return row

    def _parse_cloud_sessions(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one sync row per `cloudSessions.json` entry at the file's mtime.

        A file that is not a JSON object gives one `parser:` row.
        """
        timestamp = file_mtime(artifact)
        try:
            with open(artifact.disk_path, "rb") as handle:
                doc = json.loads(handle.read().decode("utf-8", errors="replace"))
        except ValueError as error:
            doc = error
        if not isinstance(doc, dict):
            why = " (%s)" % doc if isinstance(doc, ValueError) else ""
            row = self._system_row(
                artifact, "", timestamp, "parser: cloudSessions.json is not a JSON object" + why
            )
            row.source_line = 1
            yield row
            return
        for key, entry in doc.items():
            entry = entry if isinstance(entry, dict) else {}
            row = self._system_row(
                artifact,
                str(key),
                timestamp,
                "session synced: remote session %s task %s (from "
                "User/globalStorage/github.copilot-chat/cloudSessions.json, file mtime)"
                % (entry.get("cloudSessionId") or "-", entry.get("cloudTaskId") or "-"),
            )
            row.source_line = 1
            yield row

    def _parse_state(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the rows of a profile's or a workspace's `state.vscdb`.

        The cloud agent task, selected repository and source-host rows come
        first (profile scope only), then one `permission setting:` row per
        standing grant at the file's mtime, then a `parser:` row per problem
        reading the database or a value.
        """
        scope = STATE_PATTERN.match(artifact.rel)
        workspace = bool(scope and scope.group("scope").startswith("workspaceStorage/"))
        keys = [*GRANT_KEYS, OPT_IN_KEY, ALLOWLIST_KEY]
        if not workspace:
            keys.append(COPILOT_STATE_KEY)
        values, problems = read_items(artifact.disk_path, keys)
        project, errors = "", []
        if workspace:
            project = self._workspace_json(artifact.disk_path.parent, errors)[0]
        state = values.get(COPILOT_STATE_KEY)
        if isinstance(state, dict):
            yield from self._owned_task_rows(artifact, state.get(OWNED_TASKS_KEY))
            yield from self._selected_repo_rows(artifact, state.get(SELECTED_REPOS_KEY))
        if not workspace:
            usages, more = self._usage_items(artifact.disk_path)
            problems.extend(more)
            yield from self._source_host_rows(artifact, usages)
        timestamp, source = file_mtime(artifact), user_rel(artifact.rel)
        for text in grant_texts(values, problems):
            row = self._system_row(
                artifact, "", timestamp, "%s (from %s, file mtime)" % (text, source)
            )
            row.project_path = project
            yield row
        for problem in problems:
            row = self._system_row(artifact, "", "", "parser: " + problem)
            row.project_path = project
            yield row
        yield from error_rows(self, artifact, errors, "", project)

    def _selected_repo_rows(self, artifact: Artifact, repositories) -> Iterator[Row]:
        """Yield a `source host repository:` row per repository picked for a cloud agent task."""
        source = "(from state.vscdb %s %s, timestamp)" % (COPILOT_STATE_KEY, SELECTED_REPOS_KEY)
        for entry in repositories if isinstance(repositories, list) else ():
            if isinstance(entry, dict) and entry.get("name"):
                yield self._system_row(
                    artifact,
                    "",
                    to_utc(entry.get("timestamp")),
                    "source host repository: %s type=cloud agent selection %s"
                    % (setting_value(entry["name"]), source),
                )

    @staticmethod
    def _usage_items(path: Path) -> tuple[list[tuple[str, object]], list[str]]:
        """Return `(key, decoded value)` of each GitHub authentication usage row, and problems.

        `secret://` rows never match the pattern. A database `read_items`
        could not open gives nothing here, since it reported it.
        """
        if path.is_symlink() or sqlite_util.sqlite_problem(path):
            return [], []
        try:
            with sqlite_util.open_copy(path) as connection:
                if "ItemTable" not in sqlite_util.table_names(connection):
                    return [], []
                sql = "select key, value from ItemTable where key like 'github%-usages'"
                found = [(row[0], row[1]) for row in connection.execute(sql)]
        except (sqlite3.DatabaseError, OSError):
            return [], []
        out: list[tuple[str, object]] = []
        problems: list[str] = []
        for key, value in found:
            if not isinstance(key, str) or not USAGE_KEY_PATTERN.match(key):
                continue
            if isinstance(value, memoryview):
                value = value.tobytes()
            if isinstance(value, (bytes, bytearray)):
                value = bytes(value).decode("utf-8", errors="replace")
            try:
                out.append((key, json.loads(str(value))))
            except ValueError as error:
                problems.append("ItemTable value for %s is not valid JSON (%s)" % (key, error))
        return out, problems

    def _source_host_rows(self, artifact: Artifact, usages) -> Iterator[Row]:
        """Yield a `source host:` row per extension or MCP server that used a GitHub account.

        The row is at the usage's `lastUsed`. The account label is the GitHub
        login; a GitHub Enterprise label ends ` - <host><path>`.
        """
        for key, entries in usages:
            match = USAGE_KEY_PATTERN.match(key)
            if not match or not isinstance(entries, list):
                continue
            provider, label, mcp = match.group(1), match.group(2), match.group(3)
            host, login = "github.com", label
            if provider == "github-enterprise":
                login, _, host = label.rpartition(" - ")
                if not login:
                    login, host = label, provider
            for usage in entries:
                if not isinstance(usage, dict):
                    continue
                scopes = usage.get("scopes")
                yield self._system_row(
                    artifact,
                    "",
                    to_utc(usage.get("lastUsed")),
                    "source host: %s account %s %s=%s scopes=%s (from state.vscdb %s, lastUsed)"
                    % (
                        host,
                        login,
                        "mcp_server" if mcp else "extension",
                        setting_value(usage.get("mcpServerId" if mcp else "extensionId") or "-"),
                        ",".join(str(scope) for scope in scopes)
                        if isinstance(scopes, list)
                        else "-",
                        key,
                    ),
                )

    def _parse_settings(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the standing permission and default model rows of a profile's `settings.json`.

        A `permission setting:` row per auto-approve setting or entry and for
        the default permission level, and a `model setting:` row for the
        default model, at the file's mtime; one `parser:` row when the file
        is not a JSON object.
        """
        timestamp, source = file_mtime(artifact), user_rel(artifact.rel)
        try:
            doc = parse_jsonc(artifact.disk_path.read_bytes())
        except (OSError, ValueError) as error:
            doc = error
        if not isinstance(doc, dict):
            why = " (%s)" % doc if isinstance(doc, Exception) else ""
            text = "parser: %s is not a JSON object%s" % (source, why)
            yield self._system_row(artifact, "", "", text)
            return
        for key, value in doc.items():
            if key == DEFAULT_MODEL_KEY:
                texts = ["model setting: %s=%s" % (key, setting_value(value))]
            elif SETTINGS_PERMISSION_PATTERN.match(key):
                texts = list(setting_texts(key, value))
            else:
                continue
            for text in texts:
                row = self._system_row(
                    artifact, "", timestamp, "%s (from %s, file mtime)" % (text, source)
                )
                row.source_line = 1
                yield row

    def _parse_editing(self, artifact: Artifact) -> Iterator[Row]:
        """Yield an `edit state:` row per file of a chat editing state's latest snapshot.

        The rows are at the file's mtime, with the folder's name as the
        session id and the workspace's folder as the project; a whole-file
        `parser:` row when it does not decode or is not a JSON object.
        """
        match = EDITING_PATTERN.match(artifact.rel)
        session_id = match.group("key") if match else ""
        errors: list = []
        project = self._workspace_json(artifact.disk_path.parent.parent.parent, errors)[0]
        doc = None
        try:
            with open(artifact.disk_path, "rb") as handle:
                doc = json.loads(handle.read().decode("utf-8", errors="replace"))
        except ValueError as error:
            errors.append((0, "file is not valid JSON (%s)" % error, ""))
        else:
            if not isinstance(doc, dict):
                errors.append((0, "state.json is not a JSON object", ""))
        snapshot = doc.get("recentSnapshot") if isinstance(doc, dict) else None
        entries = snapshot.get("entries") if isinstance(snapshot, dict) else None
        timestamp = file_mtime(artifact)
        for entry in entries if isinstance(entries, list) else ():
            if isinstance(entry, dict):
                row = self._system_row(artifact, session_id, timestamp, edit_state_text(entry))
                row.project_path, row.source_line = project, 1
                yield row
        yield from error_rows(self, artifact, errors, session_id, project)

    def _owned_task_rows(self, artifact: Artifact, tasks) -> Iterator[Row]:
        """Yield a `session synced:` and a `session origin: remote` row per owned cloud task."""
        for task_id, entry in tasks.items() if isinstance(tasks, dict) else ():
            if isinstance(entry, dict):
                recorded, application = entry.get("recordedAt"), entry.get("application")
            else:
                recorded, application = entry, None
            session_id, timestamp = "copilot-cloud-agent:/task/%s" % task_id, to_utc(recorded)
            source = "(from state.vscdb %s %s, recordedAt)" % (COPILOT_STATE_KEY, OWNED_TASKS_KEY)
            yield self._system_row(
                artifact,
                session_id,
                timestamp,
                "session synced: cloud agent task %s application=%s %s"
                % (task_id, application or "-", source),
            )
            yield self._system_row(
                artifact,
                session_id,
                timestamp,
                "session origin: remote GitHub cloud agent " + source,
            )

    def _replay(self, artifact: Artifact, request_lines: dict[int, int], errors: list):
        """Replay a `.jsonl` mutation log and return the session state.

        A line that does not decode, or whose mutation does not apply, is
        appended to `errors` as (line, reason, line text) and skipped;
        `iter_jsonl` is not used because a mutation error needs the line's
        text too.
        """
        state = None
        with open(artifact.disk_path, "rb") as handle:
            for line_number, raw in enumerate(handle, 1):
                stripped = raw.strip()
                if not stripped:
                    continue
                try:
                    record = json.loads(stripped.decode("utf-8", errors="replace"))
                except ValueError as error:
                    errors.append((line_number, "unparseable line (%s)" % error, line_text(raw)))
                    continue
                if not isinstance(record, dict):
                    continue
                try:
                    state = apply_mutation(state, record)
                except MutationError as error:
                    errors.append(
                        (line_number, "mutation not applied (%s)" % error, line_text(raw))
                    )
                    continue
                self._track(record, state, line_number, request_lines)
        return state

    @staticmethod
    def _track(record: dict, state, line_number: int, request_lines: dict[int, int]) -> None:
        """Remember the log line that created each request."""
        kind, path = record.get("kind"), record.get("k") or []
        if kind == 0:
            request_lines.clear()
            request_lines[-1] = line_number
            requests = state.get("requests") if isinstance(state, dict) else None
            for i in range(len(requests) if isinstance(requests, list) else 0):
                request_lines[i] = line_number
        elif path == ["requests"]:
            requests = state.get("requests") if isinstance(state, dict) else None
            total = len(requests) if isinstance(requests, list) else 0
            start = 0
            if kind == 2:
                start = (
                    record["i"]
                    if isinstance(record.get("i"), int)
                    else total - len(record.get("v") or [])
                )
            for i in list(request_lines):
                if i >= start:
                    del request_lines[i]
            for i in range(max(start, 0), total):
                request_lines[i] = line_number
        elif kind == 1 and len(path) == 2 and path[0] == "requests" and isinstance(path[1], int):
            request_lines[path[1]] = line_number

    @staticmethod
    def _workspace_folder(artifact: Artifact, errors: list) -> tuple[str, str]:
        """Return the sibling `workspace.json`'s `folder` and `workspace` as paths.

        Returns `(folder, workspace_file)`, either empty when absent; only
        the folder may become the project path. A
        `workspace.json` that cannot be read or is not a JSON object adds
        one whole-file entry to `errors`.
        """
        parts = artifact.rel.split("/")
        if len(parts) < 4 or parts[-2] != "chatSessions" or parts[-4] != "workspaceStorage":
            return "", ""
        return VsCodeParser._workspace_json(artifact.disk_path.parent.parent, errors)

    @staticmethod
    def _workspace_json(workspace_dir: Path, errors: list) -> tuple[str, str]:
        """Return `(folder, workspace_file)` from `<ws_dir>/workspace.json`.

        As `_workspace_folder` describes; also read for a workspace's
        `state.vscdb` and chat editing state.
        """
        workspace_file = workspace_dir / "workspace.json"
        try:
            if workspace_file.is_symlink() or not workspace_file.is_file():
                return "", ""
            with open(workspace_file, "rb") as handle:
                data = json.loads(handle.read().decode("utf-8", errors="replace"))
        except (OSError, ValueError) as error:
            errors.append((0, "workspace.json not read (%s)" % error, ""))
            return "", ""
        if not isinstance(data, dict):
            errors.append((0, "workspace.json is not a JSON object", ""))
            return "", ""
        return (
            uri_to_path(str(data.get("folder") or "")),
            uri_to_path(str(data.get("workspace") or "")),
        )

    def _request_rows(
        self, request: dict, line: int, base, opts: Options, seen: dict
    ) -> Iterator[Row]:
        model = str(request.get("modelId") or "")
        request_timestamp = request.get("timestamp")
        # One file is one session, so the tracked keys need no session id.
        # The model's options (`modelConfiguration`) are tracked with it.
        config = request.get("modelConfiguration")
        pairs = ""
        if model and isinstance(config, dict):
            pairs = " ".join("%s=%s" % (key, setting_value(config[key])) for key in sorted(config))
        change = track_change("model", seen, "model", model + (" " + pairs if pairs else ""))
        if change:
            yield base("system", change, request_timestamp, line, model)
        mode_info = request.get("modeInfo")
        if not isinstance(mode_info, dict):
            mode_info = {}
        mode = mode_info.get("telemetryModeId")
        change = track_change("permission mode", seen, "mode", setting_value(mode) if mode else "")
        if change:
            yield base("system", change, request_timestamp, line, model)
        level = mode_info.get("permissionLevel")
        change = track_change(
            "permission level", seen, "level", setting_value(level) if level else ""
        )
        if change:
            yield base("system", change, request_timestamp, line, model)
        response_timestamp = request.get("responseTimestamp") or request_timestamp
        message = request.get("message")
        text = (
            message
            if isinstance(message, str)
            else (message.get("text") if isinstance(message, dict) else "") or ""
        )
        names = []
        variable_data = request.get("variableData")
        for variable in (
            variable_data.get("variables") if isinstance(variable_data, dict) else None
        ) or []:
            if isinstance(variable, dict) and variable.get("name"):
                names.append(str(variable["name"]))
        if names:
            text = "%s\n[attached: %s]" % (text, ", ".join(names))
        if request.get("isSystemInitiated"):
            yield base("system", "system initiated: " + text, request_timestamp, line, model)
        else:
            yield base("user", text, request_timestamp, line, model)
        if isinstance(request.get("confirmation"), str) and request["confirmation"]:
            yield base(
                "system",
                "permission decision: confirmation=" + request["confirmation"],
                request_timestamp,
                line,
                model,
            )

        response = request.get("response")
        if isinstance(response, (str, dict)):
            response = [response]
        run: list[str] = []

        def flush():
            joined = "".join(run).strip()
            del run[:]
            if joined:
                return base("assistant", joined, response_timestamp, line, model)
            return None

        for part in response or []:
            if isinstance(part, str):
                run.append(part)
                continue
            if not isinstance(part, dict):
                continue
            kind = part.get("kind")
            if kind is None and "value" in part:
                run.append(md_text(part))
                continue
            if kind == "markdownContent":
                run.append(md_text(part.get("content")))
                continue
            if kind == "inlineReference":
                run.append(str(part.get("name") or uri_text(part.get("inlineReference"))))
                continue
            row = flush()
            if row:
                yield row
            if kind == "thinking":
                if opts.include_thinking:
                    thought = md_text(part.get("value"))
                    if thought:
                        yield base("thinking", thought, response_timestamp, line, model)
            elif kind == "toolInvocationSerialized":
                tool_use_id = str(part.get("toolCallId") or "")
                tool_name = str(part.get("toolId") or "")
                use = base("tool_use", tool_call_text(part), response_timestamp, line, model)
                use.tool_name, use.tool_use_id = tool_name, tool_use_id
                yield use
                result_row = base(
                    "tool_result", tool_result_text(part), response_timestamp, line, model
                )
                result_row.tool_name, result_row.tool_use_id = tool_name, tool_use_id
                yield result_row
                decision = confirmed_text(part)
                if decision:
                    decision_row = base("system", decision, response_timestamp, line, model)
                    decision_row.tool_name, decision_row.tool_use_id = tool_name, tool_use_id
                    yield decision_row
            elif kind == "confirmation":
                yield base("system", confirmation_part_text(part), response_timestamp, line, model)
            elif kind == "textEditGroup":
                yield base(
                    "system", "edit: " + uri_text(part.get("uri")), response_timestamp, line, model
                )
        row = flush()
        if row:
            yield row
        result = request.get("result")
        error = result.get("errorDetails") if isinstance(result, dict) else None
        if isinstance(error, dict) and error.get("message"):
            yield base("system", "error: %s" % error["message"], response_timestamp, line, model)
        elif request.get("isCanceled"):
            yield base("system", "canceled", response_timestamp, line, model)


__all__ = [
    "VsCodeParser",
    "apply_mutation",
    "confirmation_part_text",
    "confirmed_text",
    "tool_call_text",
    "tool_result_text",
]
