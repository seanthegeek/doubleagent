"""Sourcegraph Cody chat history, settings and model preferences.

Reads the `AccountKeyedChatHistory` JSON, `{"<endpoint>-<username>": {"chat": {"<chatID>":
transcript}}}`, from the `sourcegraph.cody-ai` row of an editor's `state.vscdb` (its
`cody-local-chatHistory-v2` member) or the JetBrains plugin's
`Cody-nodejs/[Data/]JetBrains-globalState/cody-local-chatHistory-v2` file; the
`cody-model-preferences` value (the row's member or the JetBrains file); the editor's
`User/settings.json` (JSONC); and a project's `.idea/cody_history.xml`, the pre-agent
plugin's history (a secondary source). The catalog attributes the editor files to the
editor, so `reads_agents` lists `vscode_state.EDITOR_AGENTS`.

Fields read: a transcript's `id`, `chatTitle`, `lastInteractionTimestamp` and
`interactions[]` `humanMessage` and `assistantMessage`; a message's `speaker`, `text`,
`model`, `contextFiles[].uri`, `error.message`, `content[]` `tool_call` and `tool_result`
parts and `processes[]` (`type`, `id`, `title`, `description`, `content`, `state`,
`error.message`); the preferences' `selected` and `defaults`; the settings' `cody.agentic.*`,
`cody.chat.agenticContext` and each `cody.mcpServers.<server>` `disabled` and
`disabledTools`, never its `headers`, `env`, `url`, `command` or `args`; and the XML's
`accountId`, `internalId`, `updatedAt`, `llm/model`, `speaker` and `text` values.

There is no per-message timestamp: every row takes the chat id, which is the creation time
as `Date.toUTCString()`, else `lastInteractionTimestamp`. A history that does not parse is
walked chat by chat, and the XML is read incrementally, so the chats before the damage are
still yielded. A refused confirmation step is `state` `error` with `Command rejected`; an
approved one stays `pending`, which cannot be told from an unanswered one.

Validated against source (sourcegraph/cody-public-snapshot at 8e20ac6c) with a synthetic
fixture; no real install. See `research/cody.md`.
"""

from __future__ import annotations

import json
import re
import xml.etree.ElementTree as ET
from collections.abc import Iterator
from email.utils import parsedate_to_datetime

from ..errors import unreadable_message
from ..inputs import Artifact
from ..model import Row, compact
from ..timeutil import to_utc
from ..vscode_state import EDITOR_AGENTS, STATE_DATABASE_PATTERN, read_item
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_bytes,
    file_mtime,
    setting_value,
    text_of,
    track_change,
)
from .cline_legacy import iter_array_at, read_text
from .gemini_cli import parse_jsonc

JETBRAINS_PATTERN = re.compile(
    r"(?:^|/)Cody-nodejs/(?:[^/]+/)?JetBrains-globalState/cody-local-chatHistory-v2$"
)
JETBRAINS_PREFERENCES_PATTERN = re.compile(
    r"(?:^|/)Cody-nodejs/(?:[^/]+/)?JetBrains-globalState/cody-model-preferences$"
)
SETTINGS_PATTERN = re.compile(r"(?:^|/)User/settings\.json$")
LEGACY_PATTERN = re.compile(r"(?:^|/)\.idea/cody_history\.xml$")
ITEM_KEY = "sourcegraph.cody-ai"
HISTORY_KEY = "cody-local-chatHistory-v2"
PREFS_KEY = "cody-model-preferences"
# Flat `settings.json` keys whose leaves are permission settings.
PERMISSION_KEYS = (
    "cody.agentic.context.experimentalOptions",
    "cody.agentic.context.experimentalShell",
    "cody.chat.agenticContext",
)
MCP_KEY = "cody.mcpServers"
MCP_FIELDS = ("disabled", "disabledTools")
REJECTED = "Command rejected"

# JSON whitespace is space, tab, LF and CR; `_COLON` is a key's colon with it.
_SPACE = r"[ \t\n\r]*"
_COLON = _SPACE + ":" + _SPACE
_DATE_KEY = (
    r'(?a)"((?:Mon|Tue|Wed|Thu|Fri|Sat|Sun), \d{2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2} GMT)"'
    + _COLON
    + r"\{"
)
_ACCOUNT_KEY = r'"([^"\\]+)"' + _COLON + r"\{" + _SPACE + '"chat"' + _COLON + r"\{"


def chat_time(chat_id: str, fallback) -> str:
    """Return the UTC time of a `toUTCString()` chat id, else of `fallback`."""
    for candidate in (chat_id, fallback):
        if isinstance(candidate, str) and candidate:
            try:
                return to_utc(parsedate_to_datetime(candidate).isoformat())
            except (TypeError, ValueError, IndexError):
                pass
            timestamp = to_utc(candidate)
            if timestamp:
                return timestamp
    return ""


def recover_history(text: str) -> tuple[dict, str, list]:
    """Return the chats that can be read from a damaged history text, a message and the errors.

    Each chat is one whose key is a `toUTCString()` date, with the interactions that decode.
    Each error is `iter_array_at`'s (position, message, raw) for an interaction that does not
    decode, followed by the session id `CodyParser` gives its chat.
    """
    out: dict[str, dict] = {}
    found_errors: list[tuple[str, str, tuple]] = []
    marks = [
        (found.start(), "account", found.group(1)) for found in re.finditer(_ACCOUNT_KEY, text)
    ]
    marks += [(found.end(), "chat", found.group(1)) for found in re.finditer(_DATE_KEY, text)]
    marks.sort()
    account = ""
    chats = 0
    for i, (pos, kind, name) in enumerate(marks):
        if kind == "account":
            account = name
            continue
        end = marks[i + 1][0] if i + 1 < len(marks) else len(text)
        match = re.compile(r'"interactions"' + _COLON).search(text, pos, end)
        interactions = []
        if match:
            errors: list = []
            interactions = [
                value
                for _, value in iter_array_at(text, match.end(), errors)
                if isinstance(value, dict)
            ]
            found_errors.extend((account, name, entry) for entry in errors)
        title = re.compile(r'"chatTitle"' + _COLON + r'("(?:[^"\\]|\\.)*")').search(text, pos, end)
        chat = {"id": name, "interactions": interactions}
        if title:
            try:
                chat["chatTitle"] = json.loads(title.group(1))
            except ValueError:
                pass
        out.setdefault(account, {"chat": {}})["chat"][name] = chat
        chats += 1
    counts: dict[str, int] = {}
    for account_data in out.values():
        for chat_id in account_data["chat"]:
            counts[chat_id] = counts.get(chat_id, 0) + 1
    recovered_errors = [
        (*entry, "%s/%s" % (account, name) if counts[name] > 1 else name)
        for account, name, entry in found_errors
    ]
    return out, "chat history did not parse; recovered %d chat(s)" % chats, recovered_errors


def _fs_path(uri) -> str:
    if isinstance(uri, str):
        return uri
    if isinstance(uri, dict):
        for k in ("fsPath", "path", "external"):
            if isinstance(uri.get(k), str) and uri[k]:
                return uri[k]
    return ""


def _confirmation_head(process: dict) -> str:
    """Return the first line of the permission row of a `confirmation` process step.

    A refusal completes the step with `Error('Command rejected')`, so it is
    saved as `state` `error` with that `error.message`; an approval never
    completes the step, so `pending` is an approval or an unanswered request.
    """
    state = process.get("state")
    error = process.get("error")
    message = error.get("message") if isinstance(error, dict) else error
    message = setting_value(message) if message else ""
    if state == "error" and message == REJECTED:
        decision = "rejected"
    elif state == "pending":
        decision = "approved or unanswered"
    else:
        decision = "confirmation"
    head = "permission decision: %s" % decision
    if decision != "confirmation":
        head += " confirmation"
    if state:
        head += " state=%s" % setting_value(state)
    if message:
        head += " error=%s" % message
    if process.get("title"):
        head += " title=%s" % setting_value(process["title"])
    return head


def _leaves(prefix: str, value) -> list[tuple[str, object]]:
    """Return (dotted key, value) per leaf of a settings value; an empty object is a leaf."""
    if isinstance(value, dict) and value:
        out: list[tuple[str, object]] = []
        for k, child in value.items():
            out.extend(_leaves("%s.%s" % (prefix, k), child))
        return out
    return [(prefix, value)]


def settings_entries(doc: dict) -> list[tuple[str, object]]:
    """Return the permission entries of an editor `settings.json`.

    They are every leaf of `PERMISSION_KEYS`, and `disabled` and `disabledTools` of each
    MCP server (never its headers, env, url or command).
    """
    out: list[tuple[str, object]] = []
    for key in PERMISSION_KEYS:
        if key in doc:
            out.extend(_leaves(key, doc[key]))
    servers = doc.get(MCP_KEY)
    for name, server in servers.items() if isinstance(servers, dict) else ():
        if isinstance(server, dict):
            for field in MCP_FIELDS:
                if field in server:
                    out.append(("%s.%s.%s" % (MCP_KEY, name, field), server[field]))
    return out


def pref_entries(preferences) -> list[tuple[str, str, str, str]]:
    """Return (kind, usage, model id, endpoint) per entry of `cody-model-preferences`.

    The user's `selected` entries come before the server's `defaults`.
    """
    out: list[tuple[str, str, str, str]] = []
    for endpoint, entry in preferences.items() if isinstance(preferences, dict) else ():
        if not isinstance(entry, dict):
            continue
        for kind in ("selected", "defaults"):
            usages = entry.get(kind)
            for usage, model in usages.items() if isinstance(usages, dict) else ():
                if isinstance(model, str) and model:
                    out.append((kind, str(usage), model, str(endpoint)))
    return out


def _value(element, tag: str) -> str:
    """Return the `value` attribute of `element`'s direct child `tag`, or ""."""
    child = element.find(tag)
    return (child.get("value") or "") if child is not None else ""


class CodyParser(Parser):
    """Parse Cody's chat history, model preferences, settings and legacy JetBrains history."""

    agent = "cody"
    name = "cody"
    reads_agents = EDITOR_AGENTS
    has_secondary = True

    def wants(self, artifact: Artifact) -> bool:
        """Return True for an editor state database, a JetBrains Cody file or settings."""
        rel = artifact.rel
        return any(
            pattern.search(rel)
            for pattern in (
                STATE_DATABASE_PATTERN,
                JETBRAINS_PATTERN,
                JETBRAINS_PREFERENCES_PATTERN,
                SETTINGS_PATTERN,
                LEGACY_PATTERN,
            )
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for the legacy JetBrains `.idea/cody_history.xml`."""
        return bool(LEGACY_PATTERN.search(artifact.rel))

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Cody store."""
        if SETTINGS_PATTERN.search(artifact.rel):
            yield from self._settings_rows(artifact)
            return
        if JETBRAINS_PREFERENCES_PATTERN.search(artifact.rel):
            yield from self._prefs_file_rows(artifact)
            return
        if LEGACY_PATTERN.search(artifact.rel):
            yield from self._legacy_rows(artifact)
            return
        problems: list[str] = []
        recovered_errors: list = []
        state = None
        if JETBRAINS_PATTERN.search(artifact.rel):
            line = 1
            read_errors: list = []
            text = read_text(artifact.disk_path, read_errors)
            if text is None:
                yield from error_rows(self, artifact, read_errors)
                return
            history = self._decode(text, problems, recovered_errors)
        else:
            line = 0
            state, problems = read_item(artifact.disk_path, ITEM_KEY)
            if state is None:
                for problem in problems:
                    yield self._system(artifact, "parser: " + problem, line)
                return
            if isinstance(state, str):  # value did not parse; read_item noted it
                history, _, recovered_errors = recover_history(state)
            else:
                history = state.get(HISTORY_KEY) if isinstance(state, dict) else None
                if isinstance(history, str):
                    history = self._decode(history, problems, recovered_errors)
        if isinstance(history, dict):
            yield from self._history_rows(artifact, history, line, opts)
        for entry in recovered_errors:
            yield from error_rows(self, artifact, [entry[:3]], entry[3])
        if isinstance(state, dict):
            source = "%s %s" % (artifact.rel, PREFS_KEY)
            yield from self._prefs_rows(artifact, state.get(PREFS_KEY), source, line)
        for problem in problems:
            yield self._system(artifact, "parser: " + problem, line)

    def _settings_rows(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the `permission setting:` rows of an editor's `User/settings.json`."""
        errors: list = []
        raw = file_bytes(artifact.disk_path, errors)
        if raw is None:
            yield from error_rows(self, artifact, errors)
            return
        try:
            doc = parse_jsonc(raw)
        except ValueError:
            doc = None
        if not isinstance(doc, dict):
            # The file is the editor's; it concerns Cody only if it names Cody.
            if b'"cody.' in raw:
                yield self._system(artifact, "parser: settings.json is not a JSON object", 1)
            return
        timestamp = file_mtime(artifact)
        for key, value in settings_entries(doc):
            row = self._system(
                artifact,
                compact(
                    "permission setting: %s=%s (from %s, file mtime)"
                    % (key, setting_value(value), artifact.rel)
                ),
                1,
            )
            row.timestamp_utc = timestamp
            yield row

    def _prefs_file_rows(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the `model setting:` rows of `JetBrains-globalState/cody-model-preferences`."""
        errors: list = []
        text = read_text(artifact.disk_path, errors)
        if text is None:
            yield from error_rows(self, artifact, errors)
            return
        try:
            preferences = json.loads(text)
        except ValueError:
            preferences = None
        if not isinstance(preferences, dict):
            yield self._system(artifact, "parser: %s is not a JSON object" % PREFS_KEY, 1)
            return
        yield from self._prefs_rows(artifact, preferences, artifact.rel, 1)

    def _prefs_rows(self, artifact: Artifact, preferences, source: str, line: int) -> Iterator[Row]:
        timestamp = file_mtime(artifact)
        for kind, usage, model, endpoint in pref_entries(preferences):
            row = self._system(
                artifact,
                "model setting: %s.%s=%s endpoint=%s (from %s, file mtime)"
                % (kind, usage, model, endpoint, source),
                line,
            )
            row.timestamp_utc = timestamp
            row.model = model
            yield row

    def _legacy_rows(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the rows of a JetBrains project's `.idea/cody_history.xml`.

        The file is read incrementally, so the chats before a damaged part survive it.
        """
        project = artifact.home.original if artifact.home.project else ""
        stack: list[str] = []
        account = ""
        chats = 0
        try:
            with open(artifact.disk_path, "rb") as handle:
                for event, element in ET.iterparse(handle, events=("start", "end")):
                    if event == "start":
                        stack.append(element.tag)
                        if element.tag == "AccountData":
                            account = ""
                        continue
                    stack.pop()
                    if element.tag == "accountId" and stack and stack[-1] == "AccountData":
                        account = element.get("value") or ""
                    elif element.tag == "AccountData":
                        account = ""
                    elif element.tag == "chat":
                        chats += 1
                        yield from self._legacy_chat(artifact, element, account, project)
                        element.clear()
        except OSError as error:
            failure = [(0, unreadable_message(artifact.disk_path, error))]
            yield from error_rows(self, artifact, failure, "", project)
        except ET.ParseError as error:
            row = self._system(
                artifact,
                "parser: cody_history.xml did not parse; read %d chat(s) (%s)" % (chats, error),
                0,
            )
            row.project_path = project
            yield row

    def _legacy_chat(self, artifact: Artifact, chat, account: str, project: str) -> Iterator[Row]:
        internal = _value(chat, "internalId")
        updated = _value(chat, "updatedAt")
        account = account or _value(chat, "accountId")
        model = ""
        llm = chat.find("llm")
        for model_element in llm.iter("model") if llm is not None else ():
            if model_element.get("value"):
                model = model_element.get("value") or ""
                break
        model = model or _value(chat, "model")
        holder = chat.find("messages")
        messages = list(holder.iter("message")) if holder is not None else []
        timestamp = chat_time(updated, None)

        def make_row(turn: str, text: str, model_id: str = "") -> Row:
            new_row = self.base_row(artifact)
            new_row.timestamp_utc = timestamp
            new_row.session_id = updated or internal
            new_row.project_path = project
            new_row.source_line = 0
            new_row.turn_type = turn
            new_row.model = model_id
            new_row.text = compact(text)
            return new_row

        yield make_row(
            "system",
            "chat: %s | account=%s | messages=%d (legacy JetBrains history)"
            % (internal, account or "-", len(messages)),
        )
        model_shown = False
        for message in messages:
            speaker, text = _value(message, "speaker"), _value(message, "text")
            if speaker == "ASSISTANT":
                if model and not model_shown:
                    model_shown = True
                    yield make_row("system", "model: " + model, model)
                if text:
                    yield make_row("assistant", text, model)
            elif speaker == "HUMAN":
                if text:
                    yield make_row("user", text)
            elif text:
                yield make_row("system", "message speaker=%s: %s" % (speaker or "-", text))

    @staticmethod
    def _decode(text: str, problems: list[str], recovered_errors: list):
        try:
            return json.loads(text)
        except ValueError as error:
            history, problem, errors = recover_history(text)
            problems.append("%s (%s)" % (problem, error))
            recovered_errors.extend(errors)
            return history

    def _system(self, artifact: Artifact, text: str, line: int) -> Row:
        row = self.base_row(artifact)
        row.turn_type = "system"
        row.source_line = line
        row.text = text
        return row

    def _history_rows(
        self, artifact: Artifact, history: dict, line: int, opts: Options
    ) -> Iterator[Row]:
        seen: dict[str, int] = {}
        for account_data in history.values():
            chats = account_data.get("chat") if isinstance(account_data, dict) else None
            for chat_id in chats if isinstance(chats, dict) else {}:
                seen[chat_id] = seen.get(chat_id, 0) + 1
        for account, account_data in history.items():
            chats = account_data.get("chat") if isinstance(account_data, dict) else None
            if not isinstance(chats, dict):
                continue
            for chat_id, chat in chats.items():
                if not isinstance(chat, dict):
                    continue
                session_id = str(chat.get("id") or chat_id)
                if seen.get(chat_id, 0) > 1:
                    session_id = "%s/%s" % (account, session_id)
                yield from self._chat_rows(artifact, account, session_id, chat, line, opts)

    def _chat_rows(
        self,
        artifact: Artifact,
        account: str,
        session_id: str,
        chat: dict,
        line: int,
        opts: Options,
    ) -> Iterator[Row]:
        timestamp = chat_time(str(chat.get("id") or ""), chat.get("lastInteractionTimestamp"))
        interactions = chat.get("interactions")
        interactions = (
            [entry for entry in interactions if isinstance(entry, dict)]
            if isinstance(interactions, list)
            else []
        )

        def make_row(turn: str, text, model: str = "", name: str = "", tool_id: str = "") -> Row:
            new_row = self.base_row(artifact)
            new_row.timestamp_utc = timestamp
            new_row.session_id = session_id
            new_row.source_line = line
            new_row.turn_type = turn
            new_row.model = model
            new_row.tool_name = name
            new_row.tool_use_id = tool_id
            new_row.text = compact(text)
            return new_row

        yield make_row(
            "system",
            "chat: %s | account=%s | interactions=%d"
            % (chat.get("chatTitle") or "", account, len(interactions)),
        )
        names: dict[str, str] = {}
        seen: dict = {}
        for interaction in interactions:
            for key in ("humanMessage", "assistantMessage"):
                message = interaction.get(key)
                if isinstance(message, dict):
                    yield from self._message_rows(message, make_row, names, seen)

    def _message_rows(
        self, message: dict, make_row, names: dict[str, str], seen: dict
    ) -> Iterator[Row]:
        speaker = message.get("speaker")
        model = str(message.get("model") or "") if speaker == "assistant" else ""
        # `seen` is per chat, so the tracked key needs no session id.
        model_change = track_change("model", seen, "model", model)
        if model_change:
            yield make_row("system", model_change, model)
        text = message.get("text")
        text = text if isinstance(text, str) else text_of(text)
        parts = message.get("content")
        if not isinstance(parts, list):
            parts = []
        if not text:
            text = "\n".join(
                str(text_part.get("text"))
                for text_part in parts
                if isinstance(text_part, dict)
                and text_part.get("type") == "text"
                and text_part.get("text")
            )
        if speaker == "human":
            files = [
                _fs_path(entry.get("uri"))
                for entry in message.get("contextFiles") or []
                if isinstance(entry, dict)
            ]
            files = [entry for entry in files if entry]
            if files:
                text = "%s\n[context: %s]" % (text, ", ".join(files))
            if text:
                yield make_row("user", text)
        elif text:
            yield make_row("assistant" if speaker == "assistant" else "system", text, model)
        call_ids = set()
        for part in parts:
            if not isinstance(part, dict):
                continue
            if part.get("type") == "tool_call" and isinstance(part.get("tool_call"), dict):
                tool_call = part["tool_call"]
                tool_id, name = str(tool_call.get("id") or ""), str(tool_call.get("name") or "")
                call_ids.add(tool_id)
                names[tool_id] = name
                args = tool_call.get("arguments")
                yield make_row(
                    "tool_use",
                    args if isinstance(args, str) else compact_json(args),
                    model,
                    name,
                    tool_id,
                )
            elif part.get("type") == "tool_result" and isinstance(part.get("tool_result"), dict):
                tool_result = part["tool_result"]
                tool_id = str(tool_result.get("id") or "")
                content = tool_result.get("content")
                yield make_row(
                    "tool_result",
                    content
                    if isinstance(content, str)
                    else text_of(content)
                    if isinstance(content, list)
                    else compact_json(content),
                    model,
                    names.get(tool_id, ""),
                    tool_id,
                )
        for process in message.get("processes") or []:
            if isinstance(process, dict) and process.get("type") == "confirmation":
                tool_id = str(process.get("id") or "")
                lines = [_confirmation_head(process)] + [
                    setting_value(process[k]) for k in ("description", "content") if process.get(k)
                ]
                yield make_row("system", "\n".join(lines), model, names.get(tool_id, ""), tool_id)
                continue
            if not isinstance(process, dict) or process.get("type") != "tool":
                continue
            tool_id = str(process.get("id") or "")
            if tool_id and tool_id in call_ids:
                continue
            name = str(process.get("title") or "")
            names[tool_id] = name
            body = process.get("content")
            body = body if isinstance(body, str) else compact_json(body) if body is not None else ""
            if process.get("state") and process.get("state") != "success":
                body = "[%s] %s" % (process["state"], body)
            yield make_row("tool_use", body, model, name, tool_id)
        error = message.get("error")
        if isinstance(error, dict) and error.get("message"):
            yield make_row("system", "error: %s" % error["message"], model)


__all__ = ["CodyParser", "chat_time", "recover_history"]
