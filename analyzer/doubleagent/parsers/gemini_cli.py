"""Parse Gemini CLI sessions, past-use stores and standing permission files.

Reads under `~/.gemini` or `~/.cache/.gemini`: the session files `tmp/<slug>/chats/**/*.jsonl`
and legacy `.json`; as secondary sources `logs.json`, `shell_history`, `checkpoint-<tag>.json`,
`bug-report-history-<ms>.json`, the activity log `logs/session-<id>.jsonl`, the tool checkpoints
`checkpoints/*.json` and a copied `gemini-client-error-<type>-<time>.json`; as primary stores
the `memory/` files and `images/clipboard-<ms>.<ext>`. Standing grants: `trustedFolders.json`,
`settings.json` (`SETTING_LISTS`, `SETTING_SCALARS`, `general.sessionRetention`),
`policies/*.toml` and `sandbox.toml` (Python 3.11 or later), `trusted_hooks.json` and
`acknowledgments/agents.json`.

Fields the parser depends on:

- session: `sessionId`, `projectHash`, `startTime`, `lastUpdated`, `summary`, `directories`,
  `kind`; `$set`, `$patch` (`id`, `content`, `toolCalls`, `updates`, `removeIds`, `orderIds`)
  and `$rewindTo`, replayed as `chatRecordingService.ts` does; message `id`, `timestamp`,
  `type`, `content`, `model`, `thoughts[]`, `toolCalls[]` (`id`, `name`, `args`, `result`,
  `status`, `timestamp`, `displayName`);
- `logs.json`: `sessionId`, `timestamp`, `message`; a history copy's `role`, `parts`;
- activity log: `type`, `payload`, `sessionId`, `timestamp`; payload `content`, `id`, `method`,
  `url`, `body` (`contents`, `request`), `response` (`status`, `durationMs`, `body` with each
  chunk's `candidates[0].content`, under `response` for Code Assist), `error`;
- tool checkpoint `clientHistory`, `commitHash`, `toolCall`, `messageId`; error report `error`
  (`message`, `stack`), `context` (`requestContents`); `sandbox.toml` `modes`, `commands`.

Rewind, summary and removal rows inherit the latest time seen before them. `project_path` is
`tmp/<slug>/.project_root`, else the `projects.json` key of the slug, else the key whose sha256
is the hash directory or `projectHash`, else `projectHash`. Validated against
google-gemini/gemini-cli source at 6814e00 (0.67.0-nightly.20261020; v0.66.0 writes the same
shapes) with a synthetic fixture; no real install. See `research/gemini-cli.md`.
"""

from __future__ import annotations

import hashlib
import json
import os
import re
import sys
from collections import OrderedDict
from collections.abc import Iterator
from pathlib import Path
from urllib.parse import unquote

from ..errors import unreadable_message
from ..inputs import Artifact
from ..model import Row, compact
from ..timeutil import to_utc
from ..toml_util import load_toml
from . import kilo_code  # module import: kilo_code imports this module too
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_bytes,
    file_mtime,
    iter_jsonl,
    line_text,
    setting_value,
    track_change,
)

# The runtime directory: `.gemini`, or `.cache/.gemini` under the macOS sandbox.
BASE = r"^(?P<base>(?:\.cache/)?\.gemini)/"
# The per-project directory `tmp/<slug>`, which holds the sessions and their past-use copies.
TEMP_BASE = BASE + r"tmp/(?P<slug>[^/]+)/"
SESSION_PATTERN = re.compile(TEMP_BASE + r"chats/(?:.*/)?[^/]+\.jsonl?$")
LOGS_PATTERN = re.compile(TEMP_BASE + r"logs\.json$")
TRUSTED_PATTERN = re.compile(BASE + r"trustedFolders\.json$")
SHELL_PATTERN = re.compile(TEMP_BASE + r"shell_history$")
CHECKPOINT_PATTERN = re.compile(TEMP_BASE + r"checkpoint-(?P<tag>[^/]+)\.json$")
BUG_PATTERN = re.compile(TEMP_BASE + r"bug-report-history-(?P<ms>[0-9]+)\.json$")
# Past-use stores beside the session files. A tool checkpoint's name opens with
# `new Date().toISOString()`, `:` written `-` and `.` written `_` (checkpointUtils.ts).
ACTIVITY_PATTERN = re.compile(TEMP_BASE + r"logs/session-(?P<session>[^/]+)\.jsonl$")
TOOL_CHECKPOINT_PATTERN = re.compile(
    TEMP_BASE
    + r"checkpoints/(?P<time>[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2}_[0-9]{3}Z)-"
    + r"[^/]+\.json$"
)
MEMORY_PATTERN = re.compile(TEMP_BASE + r"memory/(?P<name>.+)$")
CLIPBOARD_PATTERN = re.compile(
    TEMP_BASE + r"images/clipboard-(?P<ms>[0-9]+)\.(?:png|jpg|jpeg|webp|heic|heif)$"
)
# An error report, which the CLI writes to the system temporary directory; the
# analyst places a copy in the runtime directory or a project's `tmp/<slug>`. Its
# name ends with `toISOString()`, `:` and `.` both written `-` (errorReporting.ts).
ERROR_REPORT_NAME = (
    r"gemini-client-error-(?P<type>[^/]+)-"
    r"(?P<time>[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2}-[0-9]{3}Z)\.json$"
)
ERROR_REPORT_PATTERN = re.compile(BASE + ERROR_REPORT_NAME)
PROJECT_ERROR_REPORT_PATTERN = re.compile(TEMP_BASE + ERROR_REPORT_NAME)
NAME_TIME_PATTERN = re.compile(
    r"^(?P<date>[0-9]{4}-[0-9]{2}-[0-9]{2})T(?P<hour>[0-9]{2})-(?P<minute>[0-9]{2})-"
    r"(?P<second>[0-9]{2})[-_](?P<millisecond>[0-9]{3})Z$"
)
# The separator of a prompt id, `<session id>########<prompt count>` (useGeminiStream.ts).
PROMPT_ID_SEPARATOR = "########"
# Standing permission files. `settings.json` and the policy files are read in
# the home's `.gemini` and in a project's; the other two exist only in the home.
SETTINGS_PATTERN = re.compile(r"^\.gemini/settings\.json$")
POLICY_PATTERN = re.compile(r"^\.gemini/policies/[^/]+\.toml(?:\.bak)?$")
HOOKS_PATTERN = re.compile(r"^\.gemini/trusted_hooks\.json$")
AGENTS_ACK_PATTERN = re.compile(r"^\.gemini/acknowledgments/agents\.json$")
# The sandbox's standing grants, a policy file of the home only.
SANDBOX_POLICY = ".gemini/policies/sandbox.toml"
# A `--worktree` checkout inside a project, which a session's project path names.
WORKTREE_PATTERN = re.compile(r"^.*[/\\]\.gemini[/\\]worktrees[/\\](?P<name>[^/\\]+)$")
# The start of a `/chat save` checkpoint as `JSON.stringify({history, authType})`
# writes it, for reading the history array of a file cut mid-write.
HISTORY_OBJECT_PATTERN = re.compile(r'[ \t\n\r]*\{[ \t\n\r]*"history"[ \t\n\r]*:[ \t\n\r]*')

# `settings.json` keys that hold standing permission or sandbox state
# (`settingsSchema.ts`): one row per list entry, one row per scalar.
SETTING_LISTS = (
    "tools.sandboxAllowedPaths",
    "tools.core",
    "tools.allowed",
    "tools.confirmationRequired",
    "tools.exclude",
    "mcp.allowed",
    "mcp.excluded",
    "security.allowedExtensions",
)
SETTING_SCALARS = (
    "general.defaultApprovalMode",
    "tools.sandbox",
    "tools.sandboxNetworkAccess",
    "security.toolSandboxing",
    "security.disableYoloMode",
    "security.disableAlwaysAllow",
    "security.enablePermanentToolApproval",
    "security.autoAddToPolicyByDefault",
    "security.blockGitExtensions",
    "security.folderTrust.enabled",
    "security.enableConseca",
)
RETENTION_KEYS = ("enabled", "maxAge", "maxCount", "minRetention")
HEX64_PATTERN = re.compile(r"^[0-9a-f]{64}$")

MESSAGE_TYPES = ("user", "gemini", "info", "error", "warning")
FAILED_STATUSES = ("error", "cancelled")
DECISION_STATUSES = ("cancelled", "awaiting_approval")


# ---- content helpers ----------------------------------------------------------


def _parts(content) -> list:
    if content is None:
        return []
    if isinstance(content, list):
        return content
    return [content]


def part_text(content, thoughts: bool = False) -> str:
    """Return the text of a PartListUnion.

    Thought parts are included only when `thoughts` is set, and then only they are.
    """
    out = []
    for part in _parts(content):
        if isinstance(part, str):
            if not thoughts:
                out.append(part)
            continue
        if not isinstance(part, dict):
            continue
        if bool(part.get("thought")) != thoughts:
            continue
        if isinstance(part.get("text"), str):
            out.append(part["text"])
        elif thoughts:
            continue
        elif isinstance(part.get("functionResponse"), dict):
            out.append(response_text(part["functionResponse"]))
        elif isinstance(part.get("inlineData"), dict):
            out.append("[inlineData %s]" % (part["inlineData"].get("mimeType") or ""))
        elif isinstance(part.get("fileData"), dict):
            out.append("[fileData %s]" % (part["fileData"].get("fileUri") or ""))
    return "\n".join(chunk for chunk in out if chunk)


def response_text(function_response: dict) -> str:
    """Return the text of a `functionResponse`: its `output`, `error` or `content`, else JSON."""
    response = function_response.get("response")
    if isinstance(response, dict):
        for key in ("output", "error", "content"):
            value = response.get(key)
            if isinstance(value, str) and value:
                return value
    if isinstance(response, str):
        return response
    return compact_json(response)


def result_text(result) -> str:
    """Return the text of a tool call's `result`."""
    if result is None:
        return ""
    if isinstance(result, str):
        return result
    texts = []
    for part in _parts(result):
        if isinstance(part, dict) and isinstance(part.get("functionResponse"), dict):
            texts.append(response_text(part["functionResponse"]))
        elif isinstance(part, dict) and isinstance(part.get("text"), str):
            texts.append(part["text"])
        elif isinstance(part, str):
            texts.append(part)
        else:
            texts.append(compact_json(part))
    return "\n".join(chunk for chunk in texts if chunk)


def tool_text(call: dict) -> str:
    """Return the `tool_use` text of a tool call, with any distinct display name."""
    arguments = call.get("args")
    arguments_text = compact_json(arguments) if arguments not in (None, "") else ""
    label = call.get("displayName")
    if label and label != call.get("name"):
        return "%s: %s" % (label, arguments_text) if arguments_text else str(label)
    return arguments_text


# ---- project path -------------------------------------------------------------


def _read_small(path: Path, errors: list) -> str | None:
    """Return a side file's text, or None when it is absent, a symlink or unreadable.

    Only an unreadable file leaves its entry in `errors`; an absent one is
    silent, since both side files are optional.
    """
    try:
        if os.path.islink(str(path)) or not path.is_file():
            return None
    except OSError:
        return None
    content = file_bytes(path, errors)
    return None if content is None else content.decode("utf-8", errors="replace")


def resolve_project(base: Path, slug: str, errors: list, project_hash: str = "") -> str:
    """Return the project path for `tmp/<slug>` under the `.gemini` directory `base`.

    A side file (`tmp/<slug>/.project_root`, `projects.json`) that exists but
    cannot be read leaves its entry in `errors` for the caller's rows.
    """
    root = _read_small(base / "tmp" / slug / ".project_root", errors)
    if root and root.strip():
        return root.strip()
    projects = {}
    raw = _read_small(base / "projects.json", errors)
    if raw:
        try:
            data = json.loads(raw)
            if isinstance(data, dict) and isinstance(data.get("projects"), dict):
                projects = data["projects"]
        except ValueError:
            pass
    for path, project_slug in projects.items():
        if project_slug == slug:
            return str(path)
    wanted = {
        digest for digest in (slug if HEX64_PATTERN.match(slug) else "", project_hash) if digest
    }
    if wanted:
        for path in projects:
            if hashlib.sha256(str(path).encode("utf-8")).hexdigest() in wanted:
                return str(path)
    return project_hash


# ---- trusted folders ----------------------------------------------------------


def strip_json_comments(text: str) -> str:
    """Return `text` with `//` and `/* */` comments outside strings removed.

    This is what the `strip-json-comments` package does, through which Gemini CLI and Qwen Code
    read `trustedFolders.json`.
    """
    out = []
    i, length = 0, len(text)
    in_string = False
    while i < length:
        character = text[i]
        if in_string:
            out.append(character)
            if character == "\\" and i + 1 < length:
                out.append(text[i + 1])
                i += 2
                continue
            if character == '"':
                in_string = False
            i += 1
        elif character == '"':
            in_string = True
            out.append(character)
            i += 1
        elif text.startswith("//", i):
            j = text.find("\n", i)
            i = length if j < 0 else j
        elif text.startswith("/*", i):
            j = text.find("*/", i + 2)
            i = length if j < 0 else j + 2
        else:
            out.append(character)
            i += 1
    return "".join(out)


def strip_trailing_commas(text: str) -> str:
    """Return `text` without the commas before a `}` or `]` outside strings.

    Only whitespace may come between such a comma and the bracket. This is what
    `jsonc-parser` with `allowTrailingComma` accepts. Run it after `strip_json_comments`.
    """
    out = []
    i, length = 0, len(text)
    in_string = False
    while i < length:
        character = text[i]
        if in_string:
            out.append(character)
            if character == "\\" and i + 1 < length:
                out.append(text[i + 1])
                i += 2
                continue
            if character == '"':
                in_string = False
        elif character == '"':
            in_string = True
            out.append(character)
        elif character == ",":
            j = i + 1
            while j < length and text[j] in " \t\r\n":
                j += 1
            if j >= length or text[j] not in "}]":
                out.append(character)
        else:
            out.append(character)
        i += 1
    return "".join(out)


def parse_jsonc(raw: bytes):
    """Return the value of a JSONC document as Qwen Code's `parseJsoncObject` reads it.

    A leading byte-order mark is dropped, comments stripped and trailing commas allowed. Gemini CLI
    writes plain JSON and strips comments only, so its files read the same.

    Raises:
        ValueError: the text still does not decode.
    """
    text = raw.decode("utf-8", errors="replace")
    if text.startswith("\ufeff"):
        text = text[1:]
    return json.loads(strip_trailing_commas(strip_json_comments(text)))


def trusted_folder_rows(parser: Parser, artifact: Artifact) -> Iterator[Row]:
    """Yield one `permission setting: trustedFolders.<path>=<value>` row per key of the file.

    The file is a `trustedFolders.json` of Gemini CLI or Qwen Code; the rows are at its
    modification time, and a file that cannot be read or is not a JSON object is one row saying
    so. It is read with `parse_jsonc`, so a byte-order mark and trailing commas, which Qwen Code
    accepts, decode.
    """
    errors: list = []
    content = file_bytes(artifact.disk_path, errors)
    if content is None:
        yield from error_rows(parser, artifact, errors)
        return
    timestamp = file_mtime(artifact)
    try:
        document = parse_jsonc(content)
    except ValueError:
        document = None

    def row(text: str, project: str = "") -> Row:
        new_row = parser.base_row(artifact)
        new_row.source_line = 1
        new_row.turn_type = "system"
        new_row.timestamp_utc = timestamp
        new_row.project_path = project
        new_row.text = compact(text)
        return new_row

    if not isinstance(document, dict):
        yield row("parser: %s is not a JSON object" % artifact.rel)
        return
    for path, value in document.items():
        yield row(
            "permission setting: trustedFolders.%s=%s (from %s, file mtime)"
            % (path, setting_value(value), artifact.rel),
            str(path),
        )


def _dotted(document: dict, key: str):
    """The value at the dotted `key` of nested objects, or None."""
    value: object = document
    for part in key.split("."):
        if not isinstance(value, dict):
            return None
        value = value.get(part)
    return value


def _rule_text(rule: dict, source: str) -> str:
    """Return the row text of one `[[rule]]` table: decision, tool, then its other keys."""
    fields = [
        "permission setting: policy",
        setting_value(rule.get("decision", "-")),
        setting_value(rule.get("toolName", "-")),
    ]
    fields += [
        "%s=%s" % (key, setting_value(value))
        for key, value in rule.items()
        if key not in ("decision", "toolName")
    ]
    fields.append(source)
    return " ".join(fields)


def _sandbox_texts(document: dict, source: str) -> list[str]:
    """Return the row texts of `sandbox.toml`: one per `modes` table, one per `commands` grant.

    Each table's keys follow in file order, as `SandboxPolicyManager.saveConfig` wrote them.
    """
    out = []
    for label, table in (("mode", document.get("modes")), ("command", document.get("commands"))):
        if not isinstance(table, dict):
            continue
        for name, grant in table.items():
            if not isinstance(grant, dict):
                continue
            fields = ["permission setting: sandbox %s" % label, str(name)]
            fields += ["%s=%s" % (key, setting_value(value)) for key, value in grant.items()]
            fields.append(source)
            out.append(" ".join(fields))
    return out


def shell_history_entries(raw: bytes, errors: list) -> list[tuple[int, str]]:
    """Return (first line, command) for each command of a `shell_history` file.

    The file is read as `readHistoryFile` in `useShellHistory.ts` reads it: lines split on LF or
    CRLF, blank lines skipped, and a line whose predecessor ends in an odd number of backslashes
    joined to it with a space, the last backslash dropped. A line that is not valid UTF-8 goes to
    `errors` and is skipped.
    """
    out: list[tuple[int, str]] = []
    current, first = "", 0
    for line_number, line in enumerate(raw.split(b"\n"), 1):
        line = line[:-1] if line.endswith(b"\r") else line
        try:
            text = line.decode("utf-8")
        except UnicodeDecodeError:
            errors.append((line_number, "unparseable line (not valid UTF-8)", line_text(line)))
            continue
        if not text.strip():
            continue
        tail = len(current) - len(current.rstrip("\\"))
        if current and tail % 2:
            current = current[:-1] + " " + text
        else:
            if current:
                out.append((first, current))
            current, first = text, line_number
    if current:
        out.append((first, current))
    return out


def history_items(text: str, errors: list) -> tuple[list, dict]:
    """Return the `Content[]` items of a history copy and its checkpoint object, or {}.

    A `/chat save` checkpoint is `{history, authType}` or a legacy bare array, a `/bug` history
    export a bare array. A file cut mid-write keeps its complete leading items
    (`iter_json_array`); a file that is neither form is one error.
    """
    try:
        document = json.loads(text)
    except ValueError as error:
        if text.lstrip().startswith("["):
            return kilo_code.iter_json_array(text, errors), {}
        match = HISTORY_OBJECT_PATTERN.match(text)
        if match:
            return kilo_code.iter_json_array(text[match.end() :], errors), {}
        errors.append((1, "unparseable chat history (%s)" % error, ""))
        return [], {}
    if isinstance(document, list):
        return document, {}
    if isinstance(document, dict) and isinstance(document.get("history"), list):
        return document["history"], document
    errors.append((1, "not a chat history: neither a JSON array nor an object with history", ""))
    return [], {}


def name_time(text: str) -> str:
    """Return the timeline time of an ISO time a file name spells with `-` or `_`, or `""`."""
    match = NAME_TIME_PATTERN.match(text)
    if not match:
        return ""
    return to_utc(
        "%sT%s:%s:%s.%sZ" % match.group("date", "hour", "minute", "second", "millisecond")
    )


def url_without_query(url: str) -> str:
    """Return a request URL up to its query string, which can carry an API key."""
    return url.split("?", 1)[0].split("#", 1)[0]


def request_contents(body: object) -> list:
    """Return the `Content[]` a model request body sends, or [] for any other body.

    The Gemini API body holds `contents`, the Code Assist body `request.contents`
    (`code_assist/converter.ts`); any other body, a token refresh among them, yields no rows.
    """
    if not isinstance(body, str) or not body.lstrip().startswith("{"):
        return []
    try:
        document = json.loads(body)
    except ValueError:
        return []
    if not isinstance(document, dict):
        return []
    inner = document.get("request")
    contents = inner.get("contents") if isinstance(inner, dict) else document.get("contents")
    return contents if isinstance(contents, list) else []


def is_chat_request(url: str) -> bool:
    """Return whether a request URL is a chat turn, the one method `GeminiChat` calls.

    Utility requests (loop detection, routing, next-speaker checks, compression, token counts)
    call `generateContent` or `countTokens` instead, with a trimmed history and an internal prompt.
    """
    return url_without_query(url).endswith(":streamGenerateContent")


def reply_content(body: object) -> dict | None:
    """Return the model reply a chat response body holds as one `Content`, or None.

    The body is the server-sent event stream as received, one `data: <JSON>` line per chunk, or
    one JSON object. Each chunk is a `GenerateContentResponse`, under `response` for Code Assist
    (`code_assist/converter.ts`); the parts of the first candidate's `content` of every chunk are
    joined in order, consecutive text parts of one kind into one.
    """
    if not isinstance(body, str):
        return None
    stripped = body.strip(" \t\r\n")
    if stripped.startswith("{"):
        payloads = [stripped]
    else:
        payloads = [line[5:] for line in body.split("\n") if line.startswith("data:")]
    parts: list = []
    for payload in payloads:
        try:
            chunk = json.loads(payload)
        except ValueError:
            continue
        if isinstance(chunk, dict) and isinstance(chunk.get("response"), dict):
            chunk = chunk["response"]
        candidates = chunk.get("candidates") if isinstance(chunk, dict) else None
        first = candidates[0] if isinstance(candidates, list) and candidates else None
        content = first.get("content") if isinstance(first, dict) else None
        for part in _parts(content.get("parts")) if isinstance(content, dict) else []:
            previous = parts[-1] if parts else None
            if (
                isinstance(part, dict)
                and isinstance(part.get("text"), str)
                and isinstance(previous, dict)
                and isinstance(previous.get("text"), str)
                and bool(previous.get("thought")) == bool(part.get("thought"))
            ):
                parts[-1] = dict(previous, text=previous["text"] + part["text"])
            else:
                parts.append(part)
    return {"role": "model", "parts": parts} if parts else None


def _or_dash(value: object) -> str:
    """Return a value as row text, `-` when it is absent."""
    return "-" if value is None or value == "" else str(value)


class _ChatChains:
    """The turns an activity log's chat requests and replies add, each emitted once.

    A chain is the run of chat requests of one session whose `contents` open with the same
    item: the main conversation, or one subagent's. A request's items after the longest prefix
    it shares with the chain's previous request are new; a reply is new unless the chain's next
    request carries a model item at the position after the previous request's contents.
    """

    def __init__(self) -> None:
        self.contents: dict[tuple[str, str], list] = {}  # chain -> its last request's contents
        self.requests: dict[str, tuple[str, str]] = {}  # request id -> its chain
        self.pending: dict[tuple[str, str], tuple[int, str, dict]] = {}  # chain -> its reply

    def request(
        self, request_id: str, session_id: str, line: int, contents: list
    ) -> list[tuple[int, str, dict]]:
        """Return (line, session id, item) for each new item of a chat request and its chain."""
        if not contents:
            return []
        chain = (session_id, compact_json(contents[0]))
        earlier = self.contents.get(chain, [])
        found = []
        held = self.pending.pop(chain, None)
        size = len(earlier)
        carried = (
            len(contents) > size
            and contents[:size] == earlier
            and isinstance(contents[size], dict)
            and contents[size].get("role") == "model"
        )
        if held is not None and not carried:
            found.append(held)
        shared = 0
        while shared < min(len(contents), size) and contents[shared] == earlier[shared]:
            shared += 1
        found += [(line, session_id, item) for item in contents[shared:] if isinstance(item, dict)]
        self.contents[chain] = contents
        self.requests[request_id] = chain
        return found

    def reply(self, request_id: str, session_id: str, line: int, reply: dict) -> None:
        """Hold the reply to a chat request until the chain's next request shows if it repeats."""
        chain = self.requests.get(request_id)
        if chain is not None:
            self.pending[chain] = (line, session_id, reply)

    def flush(self) -> list[tuple[int, str, dict]]:
        """Return the replies no later request repeated."""
        held = list(self.pending.values())
        self.pending.clear()
        return held


# ---- parser -------------------------------------------------------------------


class GeminiCliParser(Parser):
    """Parse Gemini CLI's sessions, prompt logs, history copies and standing permission files."""

    agent = "gemini-cli"
    name = "gemini-cli"
    has_secondary = True

    def wants(self, artifact: Artifact) -> bool:
        """Return whether the artifact is one of the files this parser reads."""
        rel = artifact.rel
        if HOOKS_PATTERN.match(rel) or AGENTS_ACK_PATTERN.match(rel):
            return not artifact.home.project
        return bool(
            SESSION_PATTERN.match(rel)
            or LOGS_PATTERN.match(rel)
            or TRUSTED_PATTERN.match(rel)
            or SHELL_PATTERN.match(rel)
            or CHECKPOINT_PATTERN.match(rel)
            or BUG_PATTERN.match(rel)
            or SETTINGS_PATTERN.match(rel)
            or POLICY_PATTERN.match(rel)
            or ACTIVITY_PATTERN.match(rel)
            or TOOL_CHECKPOINT_PATTERN.match(rel)
            or MEMORY_PATTERN.match(rel)
            or CLIPBOARD_PATTERN.match(rel)
            or ERROR_REPORT_PATTERN.match(rel)
            or PROJECT_ERROR_REPORT_PATTERN.match(rel)
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return whether the artifact repeats events of the session files."""
        rel = artifact.rel
        return bool(
            LOGS_PATTERN.match(rel)
            or SHELL_PATTERN.match(rel)
            or CHECKPOINT_PATTERN.match(rel)
            or BUG_PATTERN.match(rel)
            or ACTIVITY_PATTERN.match(rel)
            or TOOL_CHECKPOINT_PATTERN.match(rel)
            or ERROR_REPORT_PATTERN.match(rel)
            or PROJECT_ERROR_REPORT_PATTERN.match(rel)
        )

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one artifact, by the file it is."""
        rel = artifact.rel
        if match := ERROR_REPORT_PATTERN.match(rel) or PROJECT_ERROR_REPORT_PATTERN.match(rel):
            yield from self._parse_error_report(artifact, opts, match)
        elif match := ACTIVITY_PATTERN.match(rel):
            yield from self._parse_activity_log(artifact, opts, match)
        elif match := TOOL_CHECKPOINT_PATTERN.match(rel):
            yield from self._parse_tool_checkpoint(artifact, opts, match)
        elif match := MEMORY_PATTERN.match(rel):
            yield from self._parse_memory_file(artifact, match)
        elif match := CLIPBOARD_PATTERN.match(rel):
            yield from self._parse_clipboard_image(artifact, match)
        elif TRUSTED_PATTERN.match(rel):
            yield from trusted_folder_rows(self, artifact)
        elif SETTINGS_PATTERN.match(rel):
            yield from self._parse_settings(artifact)
        elif POLICY_PATTERN.match(rel):
            yield from self._parse_policy(artifact)
        elif HOOKS_PATTERN.match(rel) or AGENTS_ACK_PATTERN.match(rel):
            yield from self._parse_project_grants(artifact)
        elif match := LOGS_PATTERN.match(rel):
            yield from self._parse_logs(artifact, opts, match)
        elif match := SHELL_PATTERN.match(rel):
            yield from self._parse_shell_history(artifact, match)
        elif match := CHECKPOINT_PATTERN.match(rel) or BUG_PATTERN.match(rel):
            yield from self._parse_history_copy(artifact, opts, match)
        elif match := SESSION_PATTERN.match(rel):
            yield from self._parse_session(artifact, opts, match)

    # -- standing permission files --

    def _standing(self, artifact: Artifact, timestamp: str, project: str, text: str) -> Row:
        row = self.base_row(artifact)
        row.source_line = 1
        row.turn_type = "system"
        row.timestamp_utc = timestamp
        row.project_path = project
        row.text = compact(text)
        return row

    def _json_object(self, artifact: Artifact, jsonc: bool) -> tuple[dict | None, Row | None]:
        """The file's JSON object, or None and the row saying it cannot be read or is not one."""
        errors: list = []
        raw = file_bytes(artifact.disk_path, errors)
        if raw is None:
            return None, next(error_rows(self, artifact, errors))
        try:
            document = (
                parse_jsonc(raw) if jsonc else json.loads(raw.decode("utf-8", errors="replace"))
            )
        except ValueError:
            document = None
        if isinstance(document, dict):
            return document, None
        text = "parser: %s is not a JSON object" % artifact.rel
        return None, self._standing(artifact, file_mtime(artifact), "", text)

    def _parse_settings(self, artifact: Artifact) -> Iterator[Row]:
        # settings.json is read through strip-json-comments (settings.ts).
        document, bad = self._json_object(artifact, jsonc=True)
        if document is None:
            if bad is not None:
                yield bad
            return
        timestamp = file_mtime(artifact)
        project = artifact.home.original if artifact.home.project else ""
        source = "(from %s, file mtime)" % artifact.rel

        def setting(key: str, value: object) -> Row:
            text = "permission setting: %s=%s %s" % (key, setting_value(value), source)
            return self._standing(artifact, timestamp, project, text)

        for key in SETTING_LISTS:
            value = _dotted(document, key)
            if isinstance(value, list):
                for entry in value:
                    yield setting(key, entry)
            elif value is not None:
                yield setting(key, value)
        for key in SETTING_SCALARS:
            value = _dotted(document, key)
            if value is not None:
                yield setting(key, value)
        for key in RETENTION_KEYS:
            value = _dotted(document, "general.sessionRetention." + key)
            if value is not None:
                text = "session retention: general.sessionRetention.%s=%s %s" % (
                    key,
                    setting_value(value),
                    source,
                )
                yield self._standing(artifact, timestamp, project, text)

    def _parse_policy(self, artifact: Artifact) -> Iterator[Row]:
        timestamp = file_mtime(artifact)
        project = artifact.home.original if artifact.home.project else ""
        if sys.version_info < (3, 11):
            # flat_toml, the 3.10 stand-in, reads top-level keys only and a
            # policy file holds nothing but tables.
            text = "parser: %s not read: its [[rule]] tables need Python 3.11 or later (tomllib)"
            yield self._standing(artifact, timestamp, project, text % artifact.rel)
            return
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors, "", project)
            return
        try:
            document = load_toml(content.decode("utf-8", errors="replace"))
        except ValueError as error:
            text = "parser: unreadable %s: %s" % (artifact.rel, error)
            yield self._standing(artifact, timestamp, project, text)
            return
        source = "(from %s, file mtime)" % artifact.rel
        rules = document.get("rule")
        if isinstance(rules, list):
            for rule in rules:
                if isinstance(rule, dict):
                    yield self._standing(artifact, timestamp, project, _rule_text(rule, source))
        elif rules is not None:
            yield self._standing(
                artifact, timestamp, project, "parser: %s rule is not an array" % artifact.rel
            )
        if artifact.rel == SANDBOX_POLICY and not artifact.home.project:
            for text in _sandbox_texts(document, source):
                yield self._standing(artifact, timestamp, project, text)

    def _parse_project_grants(self, artifact: Artifact) -> Iterator[Row]:
        # Both files are written with JSON.stringify and read with JSON.parse.
        document, bad = self._json_object(artifact, jsonc=False)
        if document is None:
            if bad is not None:
                yield bad
            return
        timestamp = file_mtime(artifact)
        source = "(from %s, file mtime)" % artifact.rel
        hooks = bool(HOOKS_PATTERN.match(artifact.rel))
        for project, value in document.items():
            if hooks:
                entries = value if isinstance(value, list) else [value]
                pairs = [("trustedHooks", hook) for hook in entries]
            elif isinstance(value, dict):
                pairs = [
                    ("acknowledgedAgents." + str(agent_name), digest)
                    for agent_name, digest in value.items()
                ]
            else:
                pairs = [("acknowledgedAgents", value)]
            for key, entry in pairs:
                text = "permission setting: %s=%s %s" % (key, setting_value(entry), source)
                yield self._standing(artifact, timestamp, str(project), text)

    # -- shell_history, /chat save checkpoints and /bug history exports --

    def _parse_shell_history(self, artifact: Artifact, match) -> Iterator[Row]:
        errors: list = []
        project = resolve_project(self._base_dir(artifact, match), match.group("slug"), errors)
        raw = file_bytes(artifact.disk_path, errors) or b""
        for line, command in shell_history_entries(raw, errors):
            row = self.base_row(artifact)
            row.source_line = line
            row.project_path = project
            row.turn_type = "user"
            row.text = compact(command)
            yield row
        yield from error_rows(self, artifact, errors, "", project)

    def _parse_history_copy(self, artifact: Artifact, opts: Options, match) -> Iterator[Row]:
        errors: list = []
        project = resolve_project(self._base_dir(artifact, match), match.group("slug"), errors)
        content = file_bytes(artifact.disk_path, errors)
        text = "" if content is None else content.decode("utf-8", errors="replace")
        items, document = history_items(text, errors) if text else ([], {})
        if "tag" in match.groupdict():
            header = self._standing(
                artifact,
                file_mtime(artifact),
                project,
                "chat checkpoint: tag %s authType=%s (from %s, file mtime)"
                % (unquote(match.group("tag")), document.get("authType") or "-", artifact.rel),
            )
        else:
            header = self._standing(
                artifact,
                to_utc(int(match.group("ms"))),
                project,
                "bug report history: exported (from %s, time in the file name)" % artifact.rel,
            )
        header.source_line = 0
        yield header
        for i, item in enumerate(items, 1):
            if not isinstance(item, dict):
                errors.append(
                    (i, "unparseable record %d (not a JSON object)" % i, compact_json(item))
                )
                continue
            yield from self._content_rows(artifact, opts, item, i, project)
        yield from error_rows(self, artifact, errors, "", project)

    def _content_rows(
        self,
        artifact: Artifact,
        opts: Options,
        item: dict,
        index: int,
        project: str,
        session_id: str = "",
    ) -> Iterator[Row]:
        """Yield the rows of one `Content` ({role, parts}) of a history copy, with no time.

        The text parts are one `user` or `assistant` row, thought parts `thinking` rows when asked
        for, and each `functionCall` or `functionResponse` part a `tool_use` or `tool_result` row.
        """

        def row(turn: str, text: str) -> Row:
            new_row = self.base_row(artifact)
            new_row.source_line = index
            new_row.session_id = session_id
            new_row.project_path = project
            new_row.turn_type = turn
            new_row.text = compact(text)
            return new_row

        parts = _parts(item.get("parts"))
        texts, thoughts, tools = [], [], []
        for part in parts:
            if isinstance(part, str):
                texts.append(part)
            elif not isinstance(part, dict):
                continue
            elif isinstance(part.get("functionCall"), dict):
                function_call = part["functionCall"]
                tool_row = row(
                    "tool_use",
                    compact_json(function_call.get("args")) if function_call.get("args") else "",
                )
                tool_row.tool_name, tool_row.tool_use_id = (
                    str(function_call.get("name") or ""),
                    str(function_call.get("id") or ""),
                )
                tools.append(tool_row)
            elif isinstance(part.get("functionResponse"), dict):
                function_response = part["functionResponse"]
                tool_row = row("tool_result", response_text(function_response))
                tool_row.tool_name, tool_row.tool_use_id = (
                    str(function_response.get("name") or ""),
                    str(function_response.get("id") or ""),
                )
                tools.append(tool_row)
            elif isinstance(part.get("text"), str):
                (thoughts if part.get("thought") else texts).append(part["text"])
            elif isinstance(part.get("inlineData"), dict):
                texts.append("[inlineData %s]" % (part["inlineData"].get("mimeType") or ""))
            elif isinstance(part.get("fileData"), dict):
                texts.append("[fileData %s]" % (part["fileData"].get("fileUri") or ""))
        if opts.include_thinking:
            for thought in thoughts:
                yield row("thinking", thought)
        text = "\n".join(chunk for chunk in texts if chunk)
        if text:
            yield row("user" if item.get("role") == "user" else "assistant", text)
        yield from tools

    def _base_dir(self, artifact: Artifact, match) -> Path:
        return Path(artifact.home.disk_path) / match.group("base")

    def _system(
        self,
        artifact: Artifact,
        line: int,
        timestamp: str,
        session_id: str,
        project: str,
        text: str,
        opts: Options,
    ) -> Row:
        row = self.base_row(artifact)
        row.source_line = line
        row.timestamp_utc = timestamp
        row.session_id = session_id
        row.project_path = project
        row.turn_type = "system"
        row.text = compact(text)
        return row

    # -- activity log, tool checkpoints, memory, clipboard images and error reports --

    def _project_of(self, artifact: Artifact, match, errors: list) -> str:
        """Return the project of a file under `tmp/<slug>/`, or `""` for one outside it."""
        if "slug" not in match.groupdict():
            return ""
        return resolve_project(self._base_dir(artifact, match), match.group("slug"), errors)

    def _parse_activity_log(self, artifact: Artifact, opts: Options, match) -> Iterator[Row]:
        """Yield the rows of an activity log `logs/session-<id>.jsonl`.

        A `console` record is one `console <type>: <content>` row; a `network` record naming
        a method is a `network request:` row, one with a `response` a `network response:` row,
        one with an `error` a `network error:` row, and a streamed `chunk` none. Header values
        and non-chat bodies are never printed; the turns of chat requests and replies are item
        rows with no time (`_ChatChains`).
        """
        errors: list = []
        project = self._project_of(artifact, match, errors)
        requests: dict[str, tuple[str, str]] = {}  # request id -> (method, url)
        chains = _ChatChains()

        def items(found: list[tuple[int, str, dict]]) -> Iterator[Row]:
            for line, session_id, item in found:
                yield from self._content_rows(artifact, opts, item, line, project, session_id)

        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            payload, kind = record.get("payload"), record.get("type")
            if not isinstance(payload, dict) or kind not in ("console", "network"):
                message = "unparseable line (not an activity log record)"
                errors.append((line_number, message, compact_json(record)))
                continue
            session_id = str(record.get("sessionId") or "")
            where = (artifact, line_number, to_utc(record.get("timestamp")), session_id, project)
            if kind == "console":
                text = "console %s: %s" % (payload.get("type") or "-", payload.get("content") or "")
                yield self._system(*where, text, opts)
                continue
            request_id = str(payload.get("id") or "")
            if isinstance(payload.get("method"), str):
                method, url = payload["method"], str(payload.get("url") or "")
                requests[request_id] = (method, url_without_query(url))
                text = "network request: %s %s (request %s)" % (
                    method,
                    url_without_query(url),
                    request_id,
                )
                yield self._system(*where, text, opts)
                if is_chat_request(url):
                    contents = request_contents(payload.get("body"))
                    found = chains.request(request_id, session_id, line_number, contents)
                    yield from items(found)
                continue
            method, url = requests.get(request_id, ("-", "-"))
            response = payload.get("response")
            if isinstance(response, dict):
                text = "network response: %s %s status %s in %s ms (request %s)" % (
                    method,
                    url,
                    _or_dash(response.get("status")),
                    _or_dash(response.get("durationMs")),
                    request_id,
                )
                yield self._system(*where, text, opts)
                reply = reply_content(response.get("body"))
                if reply is not None:
                    chains.reply(request_id, session_id, line_number, reply)
            elif payload.get("error"):
                text = "network error: %s %s: %s (request %s)" % (
                    method,
                    url,
                    payload["error"],
                    request_id,
                )
                yield self._system(*where, text, opts)
        yield from items(chains.flush())
        yield from error_rows(self, artifact, errors, match.group("session"), project)

    def _json_file(self, artifact: Artifact, errors: list, label: str):
        """Return a whole-JSON file's value, or None after appending why it is not one."""
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            return None
        try:
            return json.loads(content.decode("utf-8", errors="replace"))
        except ValueError as error:
            errors.append((0, "unparseable %s (%s)" % (label, error), ""))
            return None

    def _parse_tool_checkpoint(self, artifact: Artifact, opts: Options, match) -> Iterator[Row]:
        """Yield a `tool checkpoint:` row, then the `clientHistory` items as rows with no time."""
        errors: list = []
        project = self._project_of(artifact, match, errors)
        document = self._json_file(artifact, errors, "tool checkpoint")
        if document is not None and not isinstance(document, dict):
            errors.append((0, "not a tool checkpoint: not a JSON object", ""))
        if not isinstance(document, dict):
            yield from error_rows(self, artifact, errors, "", project)
            return
        prompt_id = str(document.get("messageId") or "")
        session_id = (
            prompt_id.split(PROMPT_ID_SEPARATOR)[0] if PROMPT_ID_SEPARATOR in prompt_id else ""
        )
        tool_call = document.get("toolCall")
        tool_call = tool_call if isinstance(tool_call, dict) else {}
        arguments = tool_call.get("args")
        file_path = arguments.get("file_path") if isinstance(arguments, dict) else None
        text = "tool checkpoint: %s %s commit %s prompt %s (from %s, time in the file name)" % (
            tool_call.get("name") or "-",
            file_path or "-",
            document.get("commitHash") or "-",
            prompt_id or "-",
            artifact.rel,
        )
        yield self._system(
            artifact, 0, name_time(match.group("time")), session_id, project, text, opts
        )
        history = document.get("clientHistory")
        for i, item in enumerate(history if isinstance(history, list) else [], 1):
            if not isinstance(item, dict):
                errors.append(
                    (i, "unparseable record %d (not a JSON object)" % i, compact_json(item))
                )
                continue
            yield from self._content_rows(artifact, opts, item, i, project, session_id)
        yield from error_rows(self, artifact, errors, session_id, project)

    def _parse_memory_file(self, artifact: Artifact, match) -> Iterator[Row]:
        """Yield one `memory file:` row holding the file's text, at its modification time."""
        errors: list = []
        project = self._project_of(artifact, match, errors)
        content = file_bytes(artifact.disk_path, errors)
        if content is not None:
            try:
                body = content.decode("utf-8")
            except UnicodeDecodeError:
                errors.append((0, "%s is not UTF-8 text" % match.group("name"), ""))
            else:
                text = "memory file: %s (from %s, file mtime)\n%s" % (
                    match.group("name"),
                    artifact.rel,
                    body,
                )
                yield self._standing(artifact, file_mtime(artifact), project, text)
        yield from error_rows(self, artifact, errors, "", project)

    def _parse_clipboard_image(self, artifact: Artifact, match) -> Iterator[Row]:
        """Yield one `clipboard image:` row naming the file and its size, never its content."""
        errors: list = []
        project = self._project_of(artifact, match, errors)
        size = (artifact.manifest or {}).get("size")
        if not isinstance(size, int) or isinstance(size, bool):
            try:
                size = artifact.disk_path.lstat().st_size
            except OSError as error:
                errors.append((0, unreadable_message(artifact.disk_path, error)))
                size = "-"
        text = "clipboard image: %s %s bytes (from %s, time in the file name)" % (
            artifact.rel.rsplit("/", 1)[-1],
            size,
            artifact.rel,
        )
        yield self._standing(artifact, to_utc(int(match.group("ms"))), project, text)
        yield from error_rows(self, artifact, errors, "", project)

    def _parse_error_report(self, artifact: Artifact, opts: Options, match) -> Iterator[Row]:
        """Yield an `error report:` row, then the request's history items as rows with no time.

        The `context` is the `Content[]` sent, or an object whose `requestContents` holds it.
        """
        errors: list = []
        project = self._project_of(artifact, match, errors)
        document = self._json_file(artifact, errors, "error report")
        error = document.get("error") if isinstance(document, dict) else None
        if document is not None and not isinstance(error, dict):
            errors.append((0, "not an error report: no error object", ""))
        if not isinstance(error, dict):
            yield from error_rows(self, artifact, errors, "", project)
            return
        text = "error report: %s: %s (from %s, time in the file name)" % (
            match.group("type"),
            error.get("message") or "",
            artifact.rel,
        )
        if isinstance(error.get("stack"), str) and error["stack"]:
            text += "\n" + error["stack"]
        timestamp = name_time(match.group("time"))
        yield self._system(artifact, 0, timestamp, "", project, text, opts)
        context = document.get("context") if isinstance(document, dict) else None
        if isinstance(context, dict):
            context = context.get("requestContents")
        for i, item in enumerate(context if isinstance(context, list) else [], 1):
            if not isinstance(item, dict):
                errors.append(
                    (i, "unparseable record %d (not a JSON object)" % i, compact_json(item))
                )
                continue
            yield from self._content_rows(artifact, opts, item, i, project)
        yield from error_rows(self, artifact, errors, "", project)

    # -- logs.json --

    def _parse_logs(self, artifact: Artifact, opts: Options, match) -> Iterator[Row]:
        errors: list = []
        project = resolve_project(self._base_dir(artifact, match), match.group("slug"), errors)
        content = file_bytes(artifact.disk_path, errors)
        if content is not None:
            yield from self._log_rows(artifact, opts, project, content)
        yield from error_rows(self, artifact, errors, "", project)

    def _log_rows(
        self, artifact: Artifact, opts: Options, project: str, content: bytes
    ) -> Iterator[Row]:
        """Yield one `user` row per record of `logs.json`, or the row saying it is not an array."""
        try:
            data = json.loads(content.decode("utf-8", errors="replace"))
        except ValueError as error:
            yield self._system(
                artifact, 0, "", "", project, "parser: unparseable logs.json: %s" % error, opts
            )
            return
        if not isinstance(data, list):
            yield self._system(
                artifact, 0, "", "", project, "parser: logs.json is not a JSON array", opts
            )
            return
        for i, record in enumerate(data):
            if not isinstance(record, dict):
                continue
            row = self.base_row(artifact)
            row.source_line = i + 1  # array index, 1-based
            row.timestamp_utc = to_utc(record.get("timestamp"))
            row.session_id = str(record.get("sessionId") or "")
            row.project_path = project
            row.turn_type = "user"
            row.text = compact(record.get("message"))
            yield row

    # -- sessions --

    def _parse_session(self, artifact: Artifact, opts: Options, match) -> Iterator[Row]:
        base = self._base_dir(artifact, match)
        slug = match.group("slug")
        if artifact.rel.endswith(".json"):
            records, errors = self._legacy_records(artifact)
        else:
            records, errors = self._jsonl_records(artifact)

        metadata: dict = {}
        # id -> [record, line, session_id]
        messages: OrderedDict[str, list] = OrderedDict()
        starts: list[tuple[int, str, str, str]] = []  # (line, ts, session_id, text)
        events: list[tuple[int, str, str, str]] = []
        session_id = ""
        latest_timestamp = ""

        def put(message: dict, line: int) -> None:
            message_id = message["id"]
            if message_id in messages:
                messages[message_id][0] = message
                messages[message_id][1] = line
            else:
                messages[message_id] = [message, line, session_id]

        def drop(ids: list) -> int:
            count = 0
            for i in ids:
                if messages.pop(i, None) is not None:
                    count += 1
            return count

        for line_number, record in records:
            if isinstance(record.get("$rewindTo"), str):
                target = record["$rewindTo"]
                keys = list(messages)
                cut = keys.index(target) if target in messages else 0
                dropped = drop(keys[cut:])
                events.append(
                    (
                        line_number,
                        latest_timestamp,
                        session_id,
                        "rewind to %s: %d message(s) dropped" % (target, dropped),
                    )
                )
            elif isinstance(record.get("$patch"), dict):
                patch = record["$patch"]
                if isinstance(patch.get("id"), str):
                    self._apply_patch(messages, patch)
                for patch_update in patch.get("updates") or []:
                    if isinstance(patch_update, dict) and isinstance(patch_update.get("id"), str):
                        self._apply_patch(messages, patch_update)
                removed = [i for i in (patch.get("removeIds") or []) if isinstance(i, str)]
                if removed:
                    dropped = drop(removed)
                    events.append(
                        (
                            line_number,
                            latest_timestamp,
                            session_id,
                            "patch removed %d message(s)" % dropped,
                        )
                    )
                order = [
                    i for i in (patch.get("orderIds") or []) if isinstance(i, str) and i in messages
                ]
                if order:
                    for i in order:
                        messages.move_to_end(i)
            elif isinstance(record.get("id"), str):
                timestamp = to_utc(record.get("timestamp"))
                latest_timestamp = max(latest_timestamp, timestamp)
                put(record, line_number)
            elif isinstance(record.get("$set"), dict):
                changes = record["$set"]
                if isinstance(changes.get("sessionId"), str) and changes["sessionId"]:
                    session_id = changes["sessionId"]
                if isinstance(changes.get("messages"), list):
                    messages.clear()
                    for message in changes["messages"]:
                        if isinstance(message, dict) and isinstance(message.get("id"), str):
                            latest_timestamp = max(
                                latest_timestamp, to_utc(message.get("timestamp"))
                            )
                            put(message, line_number)
                metadata.update({key: value for key, value in changes.items() if key != "messages"})
                if changes.get("summary"):
                    timestamp = to_utc(changes.get("lastUpdated")) or latest_timestamp
                    events.append(
                        (line_number, timestamp, session_id, "summary: %s" % changes["summary"])
                    )
            elif isinstance(record.get("sessionId"), str):
                metadata.update({key: value for key, value in record.items() if key != "messages"})
                session_id = record["sessionId"] or session_id
                timestamp = to_utc(record.get("startTime"))
                latest_timestamp = max(latest_timestamp, timestamp)
                text = "session start: kind=%s projectHash=%s" % (
                    record.get("kind") or "main",
                    record.get("projectHash") or "",
                )
                if record.get("directories"):
                    text += " directories=%s" % compact_json(record["directories"])
                starts.append((line_number, timestamp, session_id, text))
                for message in record.get("messages") or []:  # legacy ConversationRecord
                    if isinstance(message, dict) and isinstance(message.get("id"), str):
                        latest_timestamp = max(latest_timestamp, to_utc(message.get("timestamp")))
                        put(message, line_number)

        project = resolve_project(base, slug, errors, str(metadata.get("projectHash") or ""))
        # Session start, then the surviving messages in replay order, then
        # summaries, rewinds and removals.
        for line, timestamp, record_session_id, text in starts:
            yield self._system(artifact, line, timestamp, record_session_id, project, text, opts)
        worktree = WORKTREE_PATTERN.match(project)
        if worktree and starts:
            line, timestamp, record_session_id, _ = starts[0]
            text = "scratch directory: %s (gemini --worktree %s)" % (
                project,
                worktree.group("name"),
            )
            yield self._system(artifact, line, timestamp, record_session_id, project, text, opts)
        models: dict = {}  # session id -> model in force, for the model rows
        for message, line, record_session_id in messages.values():
            yield from self._message_rows(
                artifact, opts, message, line, record_session_id or session_id, project, models
            )
        for line, timestamp, record_session_id, text in events:
            yield self._system(artifact, line, timestamp, record_session_id, project, text, opts)
        yield from error_rows(self, artifact, errors, session_id, project)

    @staticmethod
    def _apply_patch(messages: OrderedDict[str, list], patch: dict) -> None:
        entry = messages.get(patch["id"])
        if entry is None:
            return
        message = dict(entry[0])
        if "content" in patch and patch["content"] is not None:
            message["content"] = patch["content"]
        if (
            isinstance(patch.get("toolCalls"), list)
            and message.get("type") == "gemini"
            and isinstance(message.get("toolCalls"), list)
        ):
            calls = [
                dict(call) if isinstance(call, dict) else call for call in message["toolCalls"]
            ]
            for tool_patch in patch["toolCalls"]:
                if not isinstance(tool_patch, dict) or "result" not in tool_patch:
                    continue
                for call in calls:
                    if isinstance(call, dict) and call.get("id") == tool_patch.get("id"):
                        call["result"] = tool_patch["result"]
                        break
            message["toolCalls"] = calls
        entry[0] = message

    @staticmethod
    def _jsonl_records(artifact: Artifact):
        errors: list = []
        return list(iter_jsonl(artifact.disk_path, errors)), errors

    def _legacy_records(self, artifact: Artifact):
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            return [], errors
        try:
            record = json.loads(content.decode("utf-8", errors="replace"))
        except ValueError as error:
            return [], [(1, "unparseable session file: %s" % error, "")]
        if not isinstance(record, dict) or not isinstance(record.get("sessionId"), str):
            return [], [(1, "not a ConversationRecord", "")]
        return [(1, record)], []

    def _message_rows(
        self,
        artifact: Artifact,
        opts: Options,
        message: dict,
        line: int,
        session_id: str,
        project: str,
        models: dict,
    ) -> list[Row]:
        message_type = message.get("type")
        timestamp = to_utc(message.get("timestamp"))
        model = str(message.get("model") or "") if message_type == "gemini" else ""

        def row(turn: str, text: str, when: str = "") -> Row:
            new_row = self.base_row(artifact)
            new_row.source_line = line
            new_row.timestamp_utc = when or timestamp
            new_row.session_id = session_id
            new_row.project_path = project
            new_row.turn_type = turn
            new_row.model = model
            new_row.text = compact(text)
            return new_row

        out: list[Row] = []
        content = message.get("content")
        if message_type == "user":
            out.append(row("user", part_text(content)))
            return out
        if message_type != "gemini":
            out.append(row("system", "%s: %s" % (message_type or "message", part_text(content))))
            return out

        change = track_change("model", models, session_id, model)
        if change:
            out.append(row("system", change))

        if opts.include_thinking:
            for thought_entry in message.get("thoughts") or []:
                if not isinstance(thought_entry, dict):
                    continue
                subject, description = (
                    thought_entry.get("subject") or "",
                    thought_entry.get("description") or "",
                )
                out.append(
                    row(
                        "thinking",
                        "%s: %s" % (subject, description) if subject else description,
                        to_utc(thought_entry.get("timestamp")),
                    )
                )
            thought = part_text(content, thoughts=True)
            if thought:
                out.append(row("thinking", thought))
        text = part_text(content)
        if text:
            out.append(row("assistant", text))

        calls = [call for call in (message.get("toolCalls") or []) if isinstance(call, dict)]
        seen = {str(call.get("id") or "") for call in calls}
        # "Modern" sessions may carry functionCall parts in content instead.
        for part in _parts(content):
            function_call = part.get("functionCall") if isinstance(part, dict) else None
            if isinstance(function_call, dict) and str(function_call.get("id") or "") not in seen:
                calls.append(
                    {
                        "id": function_call.get("id"),
                        "name": function_call.get("name"),
                        "args": function_call.get("args"),
                    }
                )
        for call in calls:
            call_id = str(call.get("id") or "")
            name = str(call.get("name") or "")
            when = to_utc(call.get("timestamp"))
            use = row("tool_use", tool_text(call), when)
            use.tool_name, use.tool_use_id = name, call_id
            out.append(use)
            status = call.get("status") or ""
            if call.get("result") is not None or status in FAILED_STATUSES:
                text = result_text(call.get("result"))
                if status in FAILED_STATUSES:
                    text = "[%s] %s" % (status, text) if text else "[%s]" % status
                result_row = row("tool_result", text, when)
                result_row.tool_name, result_row.tool_use_id = name, call_id
                out.append(result_row)
            if status in DECISION_STATUSES:
                decision = row("system", "permission decision: %s" % status, when)
                decision.tool_name, decision.tool_use_id = name, call_id
                out.append(decision)
        if not out:
            # An empty gemini turn (no text, no tools) still marks a model call.
            out.append(row("assistant", ""))
        return out
