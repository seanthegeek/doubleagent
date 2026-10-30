"""Kilo Code sessions.

Kilo Code's CLI, TUI and server are an OpenCode fork with the same SQLite and JSON storage, so
this parser is `OpenCodeParser` (see `opencode.py` for the tables, fields and row mapping)
pointed at Kilo's paths: `.local/share/kilo/kilo.db`, `kilo-<channel>.db` and
`opencode-<channel>.db`, the `storage/` JSON tree, and the terminal UI's
`.local/state/kilo/prompt-history.jsonl` (secondary) and `prompt-stash.jsonl`; its
`session.json` is not read. It also reads Kilo's own files:

* `<VS Code user dir>/User/globalStorage/kilocode.kilo-code/tasks/<id>/
  api_conversation_history.json`, the pre-fork extension's tasks: a JSON array of Anthropic
  `MessageParam` records (`role`, `content` blocks `text`, `image`, `tool_use`, `tool_result`
  with `is_error`, `thinking`; `ts`, `isSummary`, top-level `type: "reasoning"` with `summary`
  or `reasoning_content`), with the task's `workspace`, `task`, `ts`, `mode` and
  `parentTaskId` from `history_item.json`, else the `tasks/_index.json` `entries`. The session
  id is the directory name.
* `.local/share/kilo/storage/session_share/<sessionID>.json` (`id`, `ingestPath`, `url`).
* `.config/kilo/{kilo.jsonc,kilo.json,opencode.jsonc,opencode.json,config.json}`: the
  `permission` key, read as JSONC.
* `.local/state/kilo-sandbox-policy/<hash>/<hash>.json` and
  `.local/state/kilo-sandbox-preference/<hash>.json`: sandbox settings.
* `.local/share/kilo/retention/state.json` (`at`, `deleted`, `scanned`, `skippedActive`, `failed`,
  `durationMs`, `cancelled`, `reclaimedBytes`): the last automatic deletion pass.
* `.local/share/kilo/memory/<folder>/sessions/<file>.md`, project memory session digests, with
  the folder's `manifest.json` `canonical` as the project path.
* `.local/share/kilo/log/<time>.log`: the four remote control lines (`REMOTE_PATTERN`) and
  nothing else.

Validated against source, Kilo-Org/kilocode at 0a9faef (7.8.25), with a synthetic fixture; no
real install. See `research/kilo-code.md`.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterator
from pathlib import Path

from ..inputs import Artifact
from ..model import Row, compact
from ..timeutil import to_utc
from . import gemini_cli  # module import: gemini_cli imports this module too
from .base import (
    Options,
    compact_json,
    error_rows,
    file_mtime,
    line_text,
    setting_value,
    tail_text,
    text_of,
)
from .opencode import OpenCodeParser, tool_summary

# Searched for, under any editor's user directory; `is_task_file` checks the match starts a
# path segment.
TASK_PATTERN = re.compile(
    r"User/globalStorage/kilocode\.kilo-code/tasks/([^/]+)/api_conversation_history\.json$"
)
# The global config files an "always" approval writes the `permission` key of,
# the first that exists in this order (packages/opencode/src/config/config.ts
# globalConfigFile at 87cc35c).
CONFIG_PATTERN = re.compile(
    r"^\.config/kilo/(?:kilo\.jsonc|kilo\.json|opencode\.jsonc|opencode\.json|config\.json)$"
)
# Beside the state directory, not inside it (`path.dirname(Global.Path.state)`).
SANDBOX_POLICY_PATTERN = re.compile(
    r"^\.local/state/kilo-sandbox-policy/[^/.][^/]*/[^/.][^/]*\.json$"
)
SANDBOX_PREFERENCE_PATTERN = re.compile(r"^\.local/state/kilo-sandbox-preference/[^/.][^/]*\.json$")
# A log line: `<LEVEL> <UTC time to the second> +<n>ms <key>=<value>... <message>`.
LOG_HEAD_PATTERN = re.compile(
    r"^(?:DEBUG|INFO|WARN|ERROR)\s+(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d) \+\d+ms "
)
REMOTE_MESSAGES = (
    "remote connection enabled",
    "remote connection disabled",
    "remote session started",
    "remote session ended",
)
REMOTE_PATTERN = {
    "remote connection enabled": re.compile(
        r" connected=(?P<connected>\S+) remote connection enabled$"
    ),
    "remote connection disabled": re.compile(r" remote connection disabled$"),
    "remote session started": re.compile(
        r" sessionID=(?P<sid>\S+) startedAt=(?P<started>\S+)(?: model=(?P<model>\S+))?"
        r"(?: directory=(?P<directory>.+?))? remote session started$"
    ),
    "remote session ended": re.compile(
        r" sessionID=(?P<sid>\S+) startedAt=(?P<started>\S+) durationMs=(?P<duration>\d+)"
        r" reason=(?P<reason>\S+) remote session ended$"
    ),
}


def iter_json_array(raw: str, errors: list) -> list[object]:
    """Decode a JSON array element by element, so a truncated file keeps its leading elements.

    A record that does not decode ends the walk and is appended to `errors` as (position,
    `unparseable record <position> (<reason>)`, the text from the record's first character to the
    end of `raw`); a file that is not an array, an array cut off between records and data after the
    closing bracket are (position, message, "").
    """
    decoder = json.JSONDecoder()
    items: list[object] = []
    i = 0
    length = len(raw)

    def skip(j: int) -> int:
        while j < length and raw[j] in " \t\r\n":
            j += 1
        return j

    i = skip(i)
    if i >= length or raw[i] != "[":
        errors.append((1, "not a JSON array", ""))
        return items
    i = skip(i + 1)
    if i < length and raw[i] == "]":
        return items
    while i < length:
        k = len(items) + 1
        try:
            decoded, end = decoder.raw_decode(raw, i)
        except ValueError as error:
            errors.append((k, "unparseable record %d (%s)" % (k, error), tail_text(raw, i)))
            return items
        items.append(decoded)
        i = skip(end)
        if i < length and raw[i] == ",":
            i = skip(i + 1)
            continue
        if i < length and raw[i] == "]":
            if skip(i + 1) < length:
                errors.append((k + 1, "unexpected data after the array", ""))
            return items
        break
    errors.append((len(items) + 1, "array cut off after record %d" % len(items), ""))
    return items


def is_task_file(rel: str) -> bool:
    """Return True for a pre-fork task's `api_conversation_history.json` under any user directory."""
    match = TASK_PATTERN.search(rel)
    return match is not None and (match.start() == 0 or rel[match.start() - 1] == "/")


def _read_json(path: Path):
    """Return the decoded JSON of a file, or None when it cannot be read or decoded."""
    try:
        with open(path, "rb") as handle:
            return json.loads(handle.read().decode("utf-8", errors="replace"))
    except (OSError, ValueError):
        return None


def _manifest_root(path: Path, errors: list) -> str:
    """Return the `canonical` project root of a memory folder's `manifest.json`, or "".

    An absent manifest, or one without `canonical` (Kilo writes it only with the project's
    identity), is silent; one that cannot be read or is not a JSON object appends (0, message)
    to `errors`.
    """
    if not path.is_file() or path.is_symlink():
        return ""
    document = _read_json(path)
    if not isinstance(document, dict):
        errors.append((0, "memory manifest.json is not a JSON object"))
        return ""
    canonical = document.get("canonical")
    return canonical if isinstance(canonical, str) else ""


class KiloCodeParser(OpenCodeParser):
    """Parse Kilo Code's OpenCode stores, legacy extension tasks and its own settings and logs."""

    agent = "kilo-code"
    name = "kilo-code"
    data_dir = ".local/share/kilo"
    database_prefixes = ("kilo", "opencode")
    # Kilo Code has no `session_v2` table and no pre-release database that a
    # later build imports, so every database under its directory,
    # `opencode-next.db` included, is primary. Its one secondary source is the
    # terminal UI's `prompt-history.jsonl` (see `opencode.py`).
    has_secondary = True
    next_database = ""
    state_dir = ".local/state/kilo"
    pinned_sessions = False

    def __init__(self) -> None:
        """Compile the patterns of Kilo's data-directory files."""
        super().__init__()
        data_dir = re.escape(self.data_dir)
        self.share_rx = re.compile(r"^%s/storage/session_share/([^/]+)\.json$" % data_dir)
        self.log_rx = re.compile(r"^%s/log/([^/]+\.log)$" % data_dir)
        self.retention_rx = re.compile(r"^%s/retention/state\.json$" % data_dir)
        self.digest_rx = re.compile(r"^%s/memory/[^/]+/sessions/[^/]+\.md$" % data_dir)

    def wants(self, artifact: Artifact) -> bool:
        """Return True for the Kilo Code files this parser reads."""
        rel = artifact.rel
        return bool(
            is_task_file(rel)
            or self.share_rx.match(rel)
            or self.log_rx.match(rel)
            or self.retention_rx.match(rel)
            or self.digest_rx.match(rel)
            or CONFIG_PATTERN.match(rel)
            or SANDBOX_POLICY_PATTERN.match(rel)
            or SANDBOX_PREFERENCE_PATTERN.match(rel)
        ) or super().wants(artifact)

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Kilo Code file."""
        rel = artifact.rel
        match = self.share_rx.match(rel)
        if is_task_file(rel):
            yield from self._parse_task(artifact, opts)
        elif match:
            yield from self._parse_share(artifact, match.group(1))
        elif self.log_rx.match(rel):
            yield from self._parse_log(artifact)
        elif self.retention_rx.match(rel):
            yield from self._parse_retention(artifact)
        elif self.digest_rx.match(rel):
            yield from self._parse_digest(artifact)
        elif CONFIG_PATTERN.match(rel):
            yield from self._parse_config(artifact)
        elif SANDBOX_POLICY_PATTERN.match(rel) or SANDBOX_PREFERENCE_PATTERN.match(rel):
            yield from self._parse_sandbox(artifact)
        else:
            yield from super().parse(artifact, opts)

    def _setting_row(
        self, artifact: Artifact, timestamp: str, text: str, session_id: str = ""
    ) -> Row:
        """Return a `system` row at `timestamp` for a standing setting or a whole-file problem."""
        row = self.base_row(artifact)
        row.session_id, row.source_line, row.turn_type = session_id, 1, "system"
        row.timestamp_utc = timestamp
        row.text = compact(text)
        return row

    # -- standing permission state ----------------------------------------------------
    def _parse_config(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `permission setting:` row per rule of a global config file's `permission` key.

        Rows read `permission.<permission>.<pattern>=<action>`, `permission.<permission>=<action>`
        for a rule given as one action, or `permission=<action>`, at the file's modification time;
        an "always" approval writes `{<permission>: {<pattern>: "allow"}}` and `null` is Kilo's
        delete sentinel. A file that does not decode as JSONC is one `parser:` row.
        """
        timestamp = file_mtime(artifact)
        try:
            document = gemini_cli.parse_jsonc(artifact.disk_path.read_bytes())
        except (OSError, ValueError) as error:
            yield self._setting_row(
                artifact, timestamp, "parser: %s could not be decoded (%s)" % (artifact.rel, error)
            )
            return
        if not isinstance(document, dict):
            yield self._setting_row(
                artifact, timestamp, "parser: %s is not a JSON object" % artifact.rel
            )
            return
        if "permission" not in document:
            return
        source = "(from %s, file mtime)" % artifact.rel
        permission = document["permission"]
        if not isinstance(permission, dict):
            yield self._setting_row(
                artifact,
                timestamp,
                "permission setting: permission=%s %s" % (setting_value(permission), source),
            )
            return
        for name, rule in permission.items():
            if isinstance(rule, dict):
                for pattern, action in rule.items():
                    yield self._setting_row(
                        artifact,
                        timestamp,
                        "permission setting: permission.%s.%s=%s %s"
                        % (name, pattern, setting_value(action), source),
                    )
            else:
                yield self._setting_row(
                    artifact,
                    timestamp,
                    "permission setting: permission.%s=%s %s" % (name, setting_value(rule), source),
                )

    def _parse_sandbox(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the `permission setting: sandbox.<key>=<value>` rows of a sandbox file.

        A policy file (`{enabled, mode, allowedHosts, writablePaths, version}`) gives a row per key,
        a preference file (`true` or `false`) one `sandbox.enabled` row; both at the file's
        modification time with no `session_id`, since the file names are hashes. A file that is not
        the expected JSON is one `parser:` row.
        """
        timestamp = file_mtime(artifact)
        source = "(from %s, file mtime)" % artifact.rel
        document = _read_json(artifact.disk_path)
        if SANDBOX_PREFERENCE_PATTERN.match(artifact.rel):
            if isinstance(document, bool):
                yield self._setting_row(
                    artifact,
                    timestamp,
                    "permission setting: sandbox.enabled=%s %s" % (setting_value(document), source),
                )
            else:
                yield self._setting_row(
                    artifact, timestamp, "parser: %s is not true or false" % artifact.rel
                )
            return
        if not isinstance(document, dict):
            yield self._setting_row(
                artifact, timestamp, "parser: %s is not a JSON object" % artifact.rel
            )
            return
        for key, value in document.items():
            yield self._setting_row(
                artifact,
                timestamp,
                "permission setting: sandbox.%s=%s %s" % (key, setting_value(value), source),
            )

    # -- deletion -------------------------------------------------------------------
    def _parse_retention(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the `sessions deleted by retention:` row of `retention/state.json`, at its `at`.

        The counts of the last automatic deletion pass, `-` for an absent count, with ` cancelled=`
        and ` reclaimedBytes=` when present and no `session_id`. A file that is not a JSON object is
        one `parser:` row.
        """
        document = _read_json(artifact.disk_path)
        if not isinstance(document, dict):
            yield self._setting_row(
                artifact, file_mtime(artifact), "parser: retention/state.json is not a JSON object"
            )
            return
        parts = [
            "%s=%s" % (k, setting_value(document[k]) if document.get(k) is not None else "-")
            for k in ("deleted", "scanned", "skippedActive", "failed", "durationMs")
        ]
        parts += [
            "%s=%s" % (k, setting_value(document[k]))
            for k in ("cancelled", "reclaimedBytes")
            if document.get(k) is not None
        ]
        yield self._setting_row(
            artifact,
            to_utc(document.get("at")),
            "sessions deleted by retention: %s (from retention/state.json, at)" % " ".join(parts),
        )

    # -- project memory -------------------------------------------------------------
    def _parse_digest(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the `memory digest:` row of one project memory session digest, at its `Updated:`.

        The session id is the `# Session <id>` heading's, the summary every line after
        `## Summary`, and `project_path` the `canonical` key of the folder's `manifest.json`. A
        digest without a `## Summary` line, or a manifest that does not decode, is a `parser:` row.
        """
        errors: list = []
        project = _manifest_root(artifact.disk_path.parent.parent / "manifest.json", errors)
        with open(artifact.disk_path, "rb") as handle:
            lines = [line_text(raw) for raw in handle]
        fields = {"# Session ": "", "Updated: ": "", "Topic: ": ""}
        summary_at = -1
        for number, line in enumerate(lines):
            if line.strip(" \t") == "## Summary":
                summary_at = number
                break
            for prefix in fields:
                if line.startswith(prefix) and not fields[prefix]:
                    fields[prefix] = line[len(prefix) :].strip(" \t")
        session_id = fields["# Session "]
        if summary_at < 0:
            errors.append((0, "%s is not a memory digest (no ## Summary line)" % artifact.rel))
            yield from error_rows(self, artifact, errors, session_id, project)
            return
        summary = "\n".join(lines[summary_at + 1 :]).strip(" \t\n")
        row = self.base_row(artifact)
        row.session_id, row.project_path = session_id, project
        row.source_line, row.turn_type = summary_at + 2, "system"
        row.timestamp_utc = to_utc(fields["Updated: "])
        row.text = "memory digest: %s: %s (from %s, Updated)" % (
            fields["Topic: "] or "-",
            summary,
            artifact.rel,
        )
        yield row
        yield from error_rows(self, artifact, errors, session_id, project)

    # -- remote control -------------------------------------------------------------
    def _parse_log(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `remote control:` row per remote control line of `log/<time>.log`.

        Connection enabled and disabled at the line's time, session started at its `startedAt` with
        the session and directory, session ended at the line's time with the session. Remote control
        lets Kilo Cloud drive sessions on this host while the transcript stays in `kilo.db`, so
        these are not sync rows. A line ending with one of the messages but lacking its fields is a
        `parser: unparseable line` row; every other line is not read.
        """
        name = artifact.rel.rsplit("/", 1)[-1]
        errors: list = []
        with open(artifact.disk_path, "rb") as handle:
            for line_number, raw in enumerate(handle, 1):
                line = line_text(raw)
                message = next(
                    (candidate for candidate in REMOTE_MESSAGES if line.endswith(" " + candidate)),
                    "",
                )
                if not message:
                    continue
                row = self._remote_row(artifact, name, line_number, line, message)
                if row is None:
                    errors.append(
                        (line_number, "unparseable line (%s without its fields)" % message, line)
                    )
                else:
                    yield row
        yield from error_rows(self, artifact, errors)

    def _remote_row(self, artifact: Artifact, name: str, line_number: int, line: str, message: str):
        """Return the row of one remote control line, or None when the line lacks its fields."""
        head = LOG_HEAD_PATTERN.match(line)
        fields = REMOTE_PATTERN[message].search(line) if head else None
        if head is None or fields is None:
            return None
        groups = fields.groupdict()
        row = self.base_row(artifact)
        row.source_line, row.turn_type = line_number, "system"
        row.timestamp_utc = to_utc(head.group(1))
        if message == "remote connection enabled":
            text = "connection enabled connected=%s" % groups["connected"]
            basis = "line time"
        elif message == "remote connection disabled":
            text, basis = "connection disabled", "line time"
        elif message == "remote session started":
            row.session_id = groups["sid"]
            row.project_path = groups.get("directory") or ""
            row.timestamp_utc = to_utc(groups["started"]) or row.timestamp_utc
            text = "session started model=%s directory=%s" % (
                groups.get("model") or "-",
                groups.get("directory") or "-",
            )
            basis = "startedAt"
        else:
            row.session_id = groups["sid"]
            text = "session ended reason=%s durationMs=%s startedAt=%s" % (
                groups["reason"],
                groups["duration"],
                groups["started"],
            )
            basis = "line time"
        row.text = compact("remote control: %s (from log/%s, %s)" % (text, name, basis))
        return row

    # -- cloud session marker -------------------------------------------------------
    def _parse_share(self, artifact: Artifact, session_id: str) -> Iterator[Row]:
        """Yield the `session synced:` row of a cloud session marker, at the file's mtime.

        A file that is not a JSON object gives one `parser:` row instead.
        """
        document = _read_json(artifact.disk_path)
        row = self.base_row(artifact)
        row.session_id, row.source_line, row.turn_type = session_id, 1, "system"
        row.timestamp_utc = file_mtime(artifact)
        rel = "storage/session_share/%s.json" % session_id
        if not isinstance(document, dict):
            row.text = "parser: %s is not a JSON object" % rel
            yield row
            return

        def value_text(key: str) -> str:
            value = document.get(key)
            return str(value) if value not in (None, "") else "-"

        row.text = compact(
            "session synced: kilo session %s ingest %s share %s (from %s, file mtime)"
            % (value_text("id"), value_text("ingestPath"), value_text("url"), rel)
        )
        yield row

    # -- legacy Roo-style tasks -----------------------------------------------------
    @staticmethod
    def _history_item(task_dir: Path, task_id: str) -> dict:
        """Return a legacy task's `HistoryItem`, from `history_item.json` or `tasks/_index.json`."""
        item = None
        item_path = task_dir / "history_item.json"
        if item_path.is_file() and not item_path.is_symlink():
            item = _read_json(item_path)
        if not isinstance(item, dict):
            index_path = task_dir.parent / "_index.json"
            data = (
                _read_json(index_path)
                if index_path.is_file() and not index_path.is_symlink()
                else None
            )
            entries = data.get("entries") if isinstance(data, dict) else None
            for entry in entries if isinstance(entries, list) else []:
                if isinstance(entry, dict) and entry.get("id") == task_id:
                    item = entry
                    break
        return item if isinstance(item, dict) else {}

    def _parse_task(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of a legacy `api_conversation_history.json` task.

        The history item, when found, is a `task:` row at `source_line` 0; each record's rows follow
        at its array position, timed by its `ts`, else the task's. A message's text blocks become
        one row placed where the first of them was.
        """
        task_dir = artifact.disk_path.parent
        task_id = task_dir.name
        item = self._history_item(task_dir, task_id)
        project = str(item.get("workspace") or "")
        task_time = to_utc(item.get("ts"))
        if not task_time and task_id.isdigit():
            task_time = to_utc(int(task_id))

        def base(line: int, row_time: str) -> Row:
            row = self.base_row(artifact)
            row.source_line = line
            row.session_id = task_id
            row.project_path = project
            row.timestamp_utc = row_time
            return row

        if item:
            row = base(0, task_time)
            row.turn_type = "system"
            row.text = compact(
                " ".join(
                    part
                    for part in (
                        "task:",
                        str(item.get("task") or ""),
                        "mode=%s" % item["mode"] if item.get("mode") else "",
                        "status=%s" % item["status"] if item.get("status") else "",
                        "parent=%s" % item["parentTaskId"] if item.get("parentTaskId") else "",
                    )
                    if part
                ),
            )
            yield row

        with open(artifact.disk_path, "rb") as handle:
            raw = handle.read().decode("utf-8", errors="replace")
        errors: list = []
        items = iter_json_array(raw, errors)
        for position, message in enumerate(items, 1):
            if not isinstance(message, dict):
                continue
            timestamp = to_utc(message.get("ts")) or task_time

            def make_row(turn: str, text) -> Row:
                row = base(position, timestamp)
                row.turn_type = turn
                row.text = compact(text)
                return row

            if message.get("type") == "reasoning":
                if opts.include_thinking:
                    text = text_of(message.get("summary")) or str(
                        message.get("reasoning_content") or message.get("text") or ""
                    )
                    if text:
                        yield make_row("thinking", text)
                continue
            role = message.get("role")
            content = message.get("content")
            if message.get("isSummary"):
                yield make_row("system", "summary: " + text_of(content))
                continue
            if isinstance(content, str):
                content = [{"type": "text", "text": content}]
            if not isinstance(content, list):
                continue
            # The text blocks of one message become one row, placed where the
            # first of them was so it keeps its order relative to tool blocks.
            texts: list[str] = []
            text_at = -1
            out: list[Row] = []
            for block in content:
                if not isinstance(block, dict):
                    continue
                block_type = block.get("type")
                if block_type in ("text", "image"):
                    block_text = "[image]" if block_type == "image" else (block.get("text") or "")
                    if block_text.lstrip().startswith("<environment_details>"):
                        out.append(make_row("system", block_text))
                    elif block_text:
                        if text_at < 0:
                            text_at = len(out)
                        texts.append(block_text)
                elif block_type == "tool_use":
                    row = make_row("tool_use", tool_summary(block.get("input")))
                    row.tool_name = str(block.get("name") or "")
                    row.tool_use_id = str(block.get("id") or "")
                    out.append(row)
                elif block_type == "tool_result":
                    text = text_of(block.get("content"))
                    row = make_row(
                        "tool_result", ("[error] " + text) if block.get("is_error") else text
                    )
                    row.tool_use_id = str(block.get("tool_use_id") or "")
                    out.append(row)
                elif block_type in ("thinking", "reasoning"):
                    block_text = block.get("thinking") or block.get("text") or ""
                    if opts.include_thinking and block_text:
                        out.append(make_row("thinking", block_text))
                else:
                    out.append(
                        make_row("system", "%s: %s" % (block_type or "block", compact_json(block)))
                    )
            if texts:
                out.insert(
                    text_at,
                    make_row(
                        "user"
                        if role == "user"
                        else "assistant"
                        if role == "assistant"
                        else "system",
                        "\n".join(texts),
                    ),
                )
            yield from out
        yield from error_rows(self, artifact, errors, task_id, project)
