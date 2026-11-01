"""Parse ShellGPT chat and REPL sessions, settings, functions and response cache.

Follows `research/shellgpt.md`. Validated against source, TheR1D/shell_gpt at
a082bd5 (1.5.1) and the pre-1.5.0 function-call shape at 6bd0bde, with a
synthetic fixture; no real install.

Files read:

- `chat_cache/<chat id>`: one JSON array of OpenAI chat messages. Fields:
  `role`, `content`, `tool_calls[].{id, function.{name, arguments}}`,
  `tool_call_id`, and the legacy `function_call.{name, arguments}` answered by
  `{"role": "function", "name", "content"}`. A legacy call has no id: it gets
  `function_call_<array index>`, shared with the `function` message after it.
- `.config/shell_gpt/.sgptrc`: `KEY=value` lines. Only `PERMISSION_KEYS` and
  `MODEL_KEY` reach a row, since the file holds the API key.
- `.config/shell_gpt/functions/*.py`: one `permission setting:` row per file,
  at its modification time; the code is not read.
- `shell_gpt/cache/<md5>`: one response string per file, a secondary source
  (a file that repeats the chat's answers) whose rows have no time or session.

Rows are `system`, `user`, `assistant`, `tool_use` and `tool_result`. A chat
file records no time, model, working directory or branch; the session id is
the file name and `source_line` is 1, since the file is one line.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterator

from ..inputs import Artifact
from ..model import Row, compact
from .base import Options, Parser, error_rows, file_bytes, file_mtime, text_of

CHAT_PATTERN = re.compile(r"(^|/)chat_cache/[^/]+$")
SGPTRC_PATTERN = re.compile(r"(^|/)\.config/shell_gpt/\.sgptrc$")
FUNCTION_PATTERN = re.compile(r"(^|/)\.config/shell_gpt/functions/[^/]+\.py$")
CACHE_PATTERN = re.compile(r"(^|/)shell_gpt/cache/[0-9a-f]{32}$")
# The `.sgptrc` keys read; every other key (OPENAI_API_KEY above all) is skipped.
PERMISSION_KEYS = ("OPENAI_USE_FUNCTIONS", "DEFAULT_EXECUTE_SHELL_CMD", "SHELL_INTERACTION")
MODEL_KEY = "DEFAULT_MODEL"


def _dict_or_empty(value) -> dict:
    return value if isinstance(value, dict) else {}


def load_messages(raw: str) -> tuple[list[dict], str]:
    """Decode a chat file into its messages and an error text ("" when whole).

    A whole array parses at once; a truncated one is decoded element by
    element so the messages before the cut survive.
    """
    try:
        data = json.loads(raw)
    except ValueError as error:
        reason = str(error)
    else:
        if isinstance(data, list):
            return [message for message in data if isinstance(message, dict)], ""
        return [], "not a JSON array"
    text = raw.lstrip()
    if not text.startswith("["):
        return [], reason
    decoder = json.JSONDecoder()
    out: list[dict] = []
    position = 1
    while True:
        while position < len(text) and text[position] in " \t\r\n,":
            position += 1
        if position >= len(text) or text[position] == "]":
            break
        try:
            value, position = decoder.raw_decode(text, position)
        except ValueError:
            break
        if isinstance(value, dict):
            out.append(value)
    return out, "truncated after %d message(s): %s" % (len(out), reason)


def call_text(arguments) -> str:
    """Return a tool call's `shell_command` argument, else its arguments as text."""
    if isinstance(arguments, str):
        try:
            parsed = json.loads(arguments)
        except ValueError:
            return arguments
        if isinstance(parsed, dict) and isinstance(parsed.get("shell_command"), str):
            return parsed["shell_command"]
        return arguments
    if isinstance(arguments, dict):
        if isinstance(arguments.get("shell_command"), str):
            return arguments["shell_command"]
        return json.dumps(arguments, ensure_ascii=False, sort_keys=True)
    return "" if arguments is None else str(arguments)


def read_sgptrc(raw: str, errors: list) -> list[tuple[str, str]]:
    """Return (key, value) for each of `PERMISSION_KEYS` and `MODEL_KEY` in file order.

    The lines are read as ShellGPT's `Config._read` does. A non-blank line
    with no `=` appends `(line, message, "")` to `errors`: the raw line is
    never kept, since the file holds the API key.
    """
    out = []
    lines = raw.replace("\r\n", "\n").replace("\r", "\n").split("\n")
    for line_number, line in enumerate(lines, 1):
        if not line.strip() or line.startswith("#"):
            continue
        if "=" not in line:
            errors.append(
                (
                    line_number,
                    'unparseable line %d (no "=" separator; not shown, the file holds the API key)'
                    % line_number,
                    "",
                )
            )
            continue
        key, value = line.strip().split("=", 1)
        if key in PERMISSION_KEYS or key == MODEL_KEY:
            out.append((key, value))
    return out


class ShellGptParser(Parser):
    """Parse ShellGPT chat files, settings, functions and cached responses."""

    agent = "shellgpt"
    name = "shellgpt"
    has_secondary = True

    def wants(self, artifact: Artifact) -> bool:
        """Return True for a chat, settings, function or response cache file."""
        rel = artifact.rel
        return bool(
            CHAT_PATTERN.search(rel)
            or SGPTRC_PATTERN.search(rel)
            or FUNCTION_PATTERN.search(rel)
            or CACHE_PATTERN.search(rel)
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for a response cache file, which repeats chat answers."""
        return bool(CACHE_PATTERN.search(artifact.rel))

    def _system(self, artifact: Artifact, timestamp: str, text: str) -> Row:
        row = self.base_row(artifact)
        row.timestamp_utc = timestamp
        row.turn_type = "system"
        row.source_line = 1
        row.text = text
        return row

    def _parse_sgptrc(self, artifact: Artifact) -> Iterator[Row]:
        timestamp = file_mtime(artifact)
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        raw = "" if content is None else content.decode("utf-8", errors="replace")
        for key, value in read_sgptrc(raw, errors):
            if key == MODEL_KEY:
                text = "model: %s (configured default, from %s, file mtime)" % (value, artifact.rel)
                row = self._system(artifact, timestamp, text)
                row.model = value
            else:
                text = "permission setting: %s=%s (from %s, file mtime)" % (
                    key,
                    value,
                    artifact.rel,
                )
                row = self._system(artifact, timestamp, text)
            yield row
        yield from error_rows(self, artifact, errors)

    def _parse_function(self, artifact: Artifact) -> Iterator[Row]:
        directory = artifact.rel.rsplit("/", 1)[0]
        text = "permission setting: function %s (from %s, file mtime)" % (
            artifact.disk_path.name,
            directory,
        )
        yield self._system(artifact, file_mtime(artifact), text)

    def _parse_cache(self, artifact: Artifact) -> Iterator[Row]:
        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors)
            return
        text = content.decode("utf-8", errors="replace")
        if not text.strip():
            return
        row = self.base_row(artifact)
        row.turn_type = "assistant"
        row.source_line = 1
        row.text = compact(text)
        yield row

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one ShellGPT file."""
        rel = artifact.rel
        if SGPTRC_PATTERN.search(rel):
            yield from self._parse_sgptrc(artifact)
            return
        if FUNCTION_PATTERN.search(rel):
            yield from self._parse_function(artifact)
            return
        if CACHE_PATTERN.search(rel):
            yield from self._parse_cache(artifact)
            return
        session_id = artifact.disk_path.name

        def make_row(turn_type: str, text: str, tool_name: str = "", tool_use_id: str = "") -> Row:
            row = self.base_row(artifact)
            row.session_id = session_id
            row.turn_type = turn_type
            row.tool_name, row.tool_use_id = tool_name, tool_use_id
            row.source_line = 1
            row.text = compact(text)
            return row

        errors: list = []
        content = file_bytes(artifact.disk_path, errors)
        if content is None:
            yield from error_rows(self, artifact, errors, session_id)
            return
        messages, error = load_messages(content.decode("utf-8", errors="replace"))
        if not any("role" in message for message in messages):
            yield make_row(
                "system", "parser: not a ShellGPT chat file" + (": " + error if error else "")
            )
            return

        names = {}  # tool call id -> function name
        pending = ("", "")  # (synthetic id, name) of a legacy function_call awaiting its result
        for index, message in enumerate(messages):
            role = str(message.get("role") or "")
            content = text_of(message.get("content"))
            previous, pending = pending, ("", "")
            if role in ("system", "user"):
                yield make_row(role, content)
            elif role == "assistant":
                if content.strip():
                    yield make_row("assistant", content)
                for call in message.get("tool_calls") or []:
                    call = _dict_or_empty(call)
                    function = _dict_or_empty(call.get("function"))
                    call_id, name = str(call.get("id") or ""), str(function.get("name") or "")
                    names[call_id] = name
                    yield make_row("tool_use", call_text(function.get("arguments")), name, call_id)
                function_call = message.get("function_call")
                if isinstance(function_call, dict):
                    pending = ("function_call_%d" % index, str(function_call.get("name") or ""))
                    yield make_row(
                        "tool_use",
                        call_text(function_call.get("arguments")),
                        pending[1],
                        pending[0],
                    )
            elif role == "tool":
                call_id = str(message.get("tool_call_id") or "")
                yield make_row("tool_result", content, names.get(call_id, ""), call_id)
            elif role == "function":
                yield make_row(
                    "tool_result", content, str(message.get("name") or previous[1]), previous[0]
                )
            else:
                yield make_row("system", "%s: %s" % (role or "message", content))
        if error:
            yield make_row("system", "parser: " + error)
