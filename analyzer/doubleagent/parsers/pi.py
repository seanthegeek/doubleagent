"""Parse pi coding agent sessions, trust settings, debug snapshots and crash records.

Follows `research/pi.md`. Validated against source, earendil-works/pi at
2c8d1feb (coding-agent 1.4.0, ahead of tag v1.4.0 at 4a080252), with the
older shapes of v0.83.0 (845d6ff1) and 1.0.2 (20038712) kept; a synthetic
fixture, no real install. Files read:

- `.pi/agent/sessions/--<cwd>--/<time>_<id>.jsonl` and legacy
  `.pi/agent/*.jsonl`: a `type: "session"` header (`version`, `id`,
  `timestamp`, `cwd`, `parentSession`), then entries with `type`, `id`,
  `timestamp`; `message` entries carry `message.role`, `content`,
  `provider`, `model`, `responseModel`, `stopReason`, `errorMessage`,
  `toolCallId`, `toolName`, `isError` and the `bashExecution` fields. Rows
  follow file order, not the active branch. A fork's entries copied from a
  parent file in the same home are dropped.
- `.pi/agent/trust.json` and the `defaultProjectTrust` key of
  `.pi/agent/settings.json`: `permission setting:` rows at the file's time.
- `.pi/agent/pi-debug.log`, a secondary source (a file repeating the open
  session's messages, with no session id) after `DEBUG_MARKER`.
- `.pi/agent/crashes.json`: one `crash:` row per record.
- `.pi/agent/experimental/sessions/<id>/meta.json`: one row per session;
  its `session.sqlite` transcript is not parsed.

A `pi.bug-report` custom entry gives `session synced:` (an upload) or
`bug report exported:` (a zip), so `records_sync` is set; its `hint` is
never printed. An error `toolResult` with one of little-coder's refusal
texts gets a `permission decision: denied` row. The session reader
(`read_session`, `entry_rows`) is shared with `letta.py` and `little_coder.py`.
"""

from __future__ import annotations

import json
import os
import re
from collections.abc import Iterator
from pathlib import Path

from ..inputs import Artifact
from ..model import Row, compact
from ..textlines import JSON_SPACE
from ..timeutil import to_utc
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_mtime,
    iter_jsonl,
    iter_lines,
    line_text,
    setting_value,
    text_of,
    track_change,
)
from .cline_legacy import iter_json_array, read_text

SESSION_PATTERN = re.compile(r"^\.pi/agent/(?:sessions/[^/]+/)?[^/]+\.jsonl$")
TRUST_REL = ".pi/agent/trust.json"
SETTINGS_REL = ".pi/agent/settings.json"
DEBUG_REL = ".pi/agent/pi-debug.log"
CRASHES_REL = ".pi/agent/crashes.json"
DEBUG_MARKER = "=== Agent messages (JSONL) ==="
DEBUG_HEADER = "Debug output at "
# The debug log's messages carry their own `timestamp`, which for an
# assistant message is when its stream started; the session file stamps the
# entry when the message ended.
DEBUG_TOLERANCE_SECONDS = 3600.0
SESSION_NAME_PATTERN = re.compile(r"_([^_/]+)\.jsonl$")
# The refusal texts of little-coder's permission-gate extension.
GATE_REFUSALS = ("edit cancelled by user", "command cancelled by user")
GATE_PREFIX = "shell whitelist:"
META_PATTERN = re.compile(r"^\.pi/agent/experimental/sessions/([^/]+)/meta\.json$")

ARG_KEYS = ("command", "path", "file_path", "pattern", "code", "url", "query")
SKIPPED_ENTRIES = ("custom", "label", "usage")
BUG_REPORT = "pi.bug-report"


class SessionFile:
    """Hold one pi-format session file's header, entries and unparseable lines.

    The header is empty when line 1 is not one; the entries after it are
    (line, record) pairs.
    """

    def __init__(self) -> None:
        """Start an empty session file."""
        self.header: dict = {}
        self.header_line = 0
        self.entries: list[tuple[int, dict]] = []
        self.errors: list = []


def read_session(path) -> SessionFile:
    """Read a pi-format session file into its header, entries and bad lines."""
    session = SessionFile()
    first = True
    for line_number, record in iter_jsonl(path, session.errors):
        if first:
            first = False
            if record.get("type") == "session":
                session.header, session.header_line = record, line_number
                continue
        session.entries.append((line_number, record))
    return session


def entry_keys(path) -> set[tuple[str, str]]:
    """Return the (id, timestamp) of every entry, for fork deduplication."""
    out: set[tuple[str, str]] = set()
    for _, record in read_session(path).entries:
        if record.get("id"):
            out.add((str(record.get("id")), str(record.get("timestamp") or "")))
    return out


def args_summary(args) -> str:
    """Return the most identifying tool argument, else the arguments as JSON."""
    if isinstance(args, dict):
        for key in ARG_KEYS:
            value = args.get(key)
            if value not in (None, ""):
                return value if isinstance(value, str) else compact_json(value)
        return compact_json(args)
    if args in (None, ""):
        return ""
    return args if isinstance(args, str) else compact_json(args)


def _label(prefix: str, text: str) -> str:
    return "%s: %s" % (prefix, text) if text else prefix


def model_text(seen: dict, key: str, model: str, provider: str, requested: str = "") -> str:
    """Return the `model:` row text when `model` is new or changed for `key`, else "".

    The text ends with ` provider=<provider>` and, when the provider reported
    a concrete model other than the one asked for, ` requested=<model asked
    for>`.
    """
    text = track_change("model", seen, key, model)
    if text and provider:
        text += " provider=%s" % provider
    if text and requested and requested != model:
        text += " requested=%s" % requested
    return text


def switch_text(seen: dict, key: str, model: str, provider: str) -> str:
    """Return the row text of a `model_change` entry, the user's own switch.

    That is the `model_text` row, or, when the entry names the model already
    in force, the plain `model: <id> provider=<provider>` row, so the switch
    keeps a row.
    """
    text = model_text(seen, key, model, provider)
    if not text and model:
        text = "model: %s" % model + (" provider=%s" % provider if provider else "")
    return text


def entry_rows(
    parser: Parser,
    artifact: Artifact,
    session: SessionFile,
    opts: Options,
    session_id: str = "",
    project_path: str = "",
    skip: set[tuple[str, str]] | None = None,
) -> Iterator[Row]:
    """Yield the timeline rows of a pi-format session's entries.

    `skip` holds the (id, timestamp) keys to drop: the entries a fork copied
    from its parent.
    """
    skip = skip or set()
    seen: dict = {}  # the model in use, for the `model:` rows
    for line_number, record in session.entries:
        if skip and (str(record.get("id") or ""), str(record.get("timestamp") or "")) in skip:
            continue
        entry_type = record.get("type")
        raw_message = record.get("message")
        message: dict = raw_message if isinstance(raw_message, dict) else {}

        def base(model: str = "") -> Row:
            row = parser.base_row(artifact)
            row.source_line = line_number
            row.timestamp_utc = to_utc(record.get("timestamp")) or to_utc(message.get("timestamp"))
            row.session_id = session_id
            row.project_path = project_path
            row.model = model
            return row

        def emit(row: Row, turn: str, text: str, tool_name: str = "", tool_use_id: str = "") -> Row:
            row.turn_type = turn
            row.tool_name = tool_name
            row.tool_use_id = tool_use_id
            row.text = compact(text)
            return row

        if entry_type == "message":
            yield from _message_rows(message, record, base, emit, opts, seen, session_id)
        elif entry_type == "model_change":
            model = str(record.get("modelId") or "")
            text = switch_text(seen, session_id, model, str(record.get("provider") or ""))
            if text:
                yield emit(base(model), "system", text)
        elif entry_type == "thinking_level_change":
            yield emit(
                base(), "system", "thinking_level_change: %s" % (record.get("thinkingLevel") or "")
            )
        elif entry_type == "compaction":
            yield emit(
                base(),
                "system",
                _label(
                    "compaction: tokensBefore=%s" % (record.get("tokensBefore") or ""),
                    str(record.get("summary") or ""),
                ),
            )
        elif entry_type == "branch_summary":
            yield emit(
                base(),
                "system",
                _label(
                    "branch_summary from %s" % (record.get("fromId") or ""),
                    str(record.get("summary") or ""),
                ),
            )
        elif entry_type == "custom_message":
            yield emit(
                base(),
                "system",
                _label(
                    "custom_message %s" % (record.get("customType") or ""),
                    text_of(record.get("content")),
                ),
            )
        elif entry_type == "context_edit":
            replacement = record.get("replacement")
            what = (
                text_of(replacement.get("content"))
                if isinstance(replacement, dict)
                else "omitted from context"
            )
            yield emit(
                base(),
                "system",
                _label("context_edit target=%s" % (record.get("targetId") or ""), what),
            )
        elif entry_type == "session_info":
            yield emit(base(), "system", "session_info: name=%s" % (record.get("name") or ""))
        elif entry_type == "custom" and record.get("customType") == BUG_REPORT:
            text = bug_report_text(record.get("data"))
            if text:
                yield emit(base(), "system", text)
        # Other custom entries, label, usage and unknown types carry no
        # conversation content.


def bug_report_text(data) -> str:
    """Return the row text of a `pi.bug-report` entry's `data`.

    That is `session synced:` for an upload, `bug report exported:` for a
    zip and "" for any other `delivery`. `hint` is not printed.
    """
    if not isinstance(data, dict):
        return ""

    def field_text(key: str) -> str:
        value = data.get(key)
        return "-" if value in (None, "") else setting_value(value)

    source = "(from session entry %s, entry timestamp)" % BUG_REPORT
    if data.get("delivery") == "upload":
        return (
            "session synced: bug report %s uploaded to radius.pi.dev transcript=%s summary=%s %s"
            % (
                field_text("id"),
                field_text("sessionIncluded"),
                field_text("summaryIncluded"),
                source,
            )
        )
    if data.get("delivery") == "zip":
        return "bug report exported: %s path=%s %s" % (field_text("id"), field_text("path"), source)
    return ""


def _message_rows(
    message: dict, record: dict, base, emit, opts: Options, seen: dict, session_id: str
) -> Iterator[Row]:
    role = message.get("role")
    if role == "user":
        text = text_of(message.get("content"))
        if text:
            yield emit(base(), "user", text)
    elif role == "assistant":
        requested = str(message.get("model") or "")
        model = str(message.get("responseModel") or "") or requested
        text = model_text(seen, session_id, model, str(message.get("provider") or ""), requested)
        if text:
            yield emit(base(model), "system", text)
        content = message.get("content")
        if isinstance(content, str):
            content = [{"type": "text", "text": content}]
        for part in content if isinstance(content, list) else []:
            if not isinstance(part, dict):
                continue
            part_type = part.get("type")
            if part_type == "text" and part.get("text"):
                yield emit(base(model), "assistant", str(part.get("text")))
            elif part_type == "thinking" and opts.include_thinking:
                text = str(part.get("thinking") or "")
                if part.get("redacted") and not text:
                    text = "[redacted]"
                if text:
                    yield emit(base(model), "thinking", text)
            elif part_type == "toolCall":
                name = str(part.get("name") or "")
                yield emit(
                    base(model),
                    "tool_use",
                    args_summary(part.get("arguments")),
                    tool_name=name,
                    tool_use_id=str(part.get("id") or ""),
                )
        if message.get("stopReason") in ("error", "aborted"):
            yield emit(
                base(model),
                "system",
                _label(
                    "assistant stopReason=%s" % message.get("stopReason"),
                    str(message.get("errorMessage") or ""),
                ),
            )
    elif role == "toolResult":
        reason = text_of(message.get("content"))
        text = "[error] " + reason if message.get("isError") else reason
        tool_name = str(message.get("toolName") or "")
        tool_use_id = str(message.get("toolCallId") or "")
        yield emit(base(), "tool_result", text, tool_name=tool_name, tool_use_id=tool_use_id)
        if message.get("isError") and (reason in GATE_REFUSALS or reason.startswith(GATE_PREFIX)):
            yield emit(
                base(),
                "system",
                "permission decision: denied (%s)" % reason,
                tool_name=tool_name,
                tool_use_id=tool_use_id,
            )
    elif role == "bashExecution":
        entry_id = str(record.get("id") or "")
        yield emit(
            base(),
            "tool_use",
            str(message.get("command") or ""),
            tool_name="bash",
            tool_use_id=entry_id,
        )
        out = str(message.get("output") or "")
        notes = []
        if message.get("exitCode") not in (None, 0):
            notes.append("exit %s" % message.get("exitCode"))
        if message.get("cancelled"):
            notes.append("cancelled")
        if message.get("truncated"):
            notes.append("truncated")
        if notes:
            out = (out + "\n" if out else "") + "[%s]" % ", ".join(notes)
        yield emit(base(), "tool_result", out, tool_name="bash", tool_use_id=entry_id)
    elif role == "system":
        yield emit(base(), "system", _label("system prompt", text_of(message.get("content"))))
    elif role == "custom":
        yield emit(
            base(),
            "system",
            _label(
                "custom %s" % (message.get("customType") or ""), text_of(message.get("content"))
            ),
        )
    elif role in ("branchSummary", "compactionSummary"):
        yield emit(base(), "system", _label(role, str(message.get("summary") or "")))


def home_file(artifact: Artifact, original: str, marker: str) -> Path | None:
    """Return the collected copy of `original`, a path on the source host, or None.

    The copy is looked for inside the artifact's home, relative to the
    home's original path, then from the last `marker` segment (loose input
    whose home path is not the host's). Symlinks are never followed.
    """
    path = original.replace("\\", "/")
    rels = []
    home = artifact.home.original.replace("\\", "/").rstrip("/")
    if home and path.startswith(home + "/"):
        rels.append(path[len(home) + 1 :])
    marker_at = path.rfind("/" + marker)
    if marker_at >= 0:
        rels.append(path[marker_at + 1 :])
    root = artifact.home.disk_path
    for rel in rels:
        parts = [part for part in rel.split("/") if part]
        if not parts or ".." in parts:
            continue
        current = root
        safe = True
        for part in parts:
            current = current / part
            if current.is_symlink():
                safe = False
                break
        if safe and current.is_file():
            return current
    return None


class PiParser(Parser):
    """Parse pi's session files, trust settings, debug log, crashes and session metadata."""

    agent = "pi"
    name = "pi"
    records_sync = True
    has_secondary = True

    def __init__(self) -> None:
        """Start with an empty cache of parent sessions' entry keys."""
        self._parent_cache: dict[Path, set[tuple[str, str]]] = {}

    def wants(self, artifact: Artifact) -> bool:
        """Return True for a session, trust, settings, debug, crash or metadata file."""
        return artifact.rel in (TRUST_REL, SETTINGS_REL, DEBUG_REL, CRASHES_REL) or bool(
            SESSION_PATTERN.match(artifact.rel) or META_PATTERN.match(artifact.rel)
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for the debug log, which repeats the open session's messages."""
        return artifact.rel == DEBUG_REL

    def secondary_tolerance(self, artifact: Artifact) -> float:
        """Return the debug log's wider match window, `DEBUG_TOLERANCE_SECONDS`."""
        if artifact.rel == DEBUG_REL:
            return DEBUG_TOLERANCE_SECONDS
        return super().secondary_tolerance(artifact)

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one pi file."""
        if artifact.rel == TRUST_REL:
            yield from self._parse_trust(artifact)
            return
        if artifact.rel == SETTINGS_REL:
            yield from self._parse_settings(artifact)
            return
        if artifact.rel == DEBUG_REL:
            yield from self._parse_debug_log(artifact, opts)
            return
        if artifact.rel == CRASHES_REL:
            yield from self._parse_crashes(artifact)
            return
        match = META_PATTERN.match(artifact.rel)
        if match:
            yield from self._parse_meta(artifact, match.group(1), opts)
            return
        session = read_session(artifact.disk_path)
        header = session.header
        if not header:
            # Not a session file to pi either; only its bad lines are reported.
            yield from error_rows(self, artifact, session.errors)
            return
        session_id = str(header.get("id") or "")
        cwd = str(header.get("cwd") or "")
        parent = str(header.get("parentSession") or "")

        row = self.base_row(artifact)
        row.source_line = session.header_line
        row.timestamp_utc = to_utc(header.get("timestamp"))
        row.session_id, row.project_path, row.turn_type = session_id, cwd, "system"
        text = "session start: version=%s cwd=%s" % (header.get("version") or 1, cwd)
        if parent:
            text += " parentSession=%s" % parent
        row.text = compact(text)
        yield row

        skip: set[tuple[str, str]] = set()
        if parent:
            parent_file = home_file(artifact, parent, ".pi/")
            if parent_file is not None and parent_file != artifact.disk_path:
                if parent_file not in self._parent_cache:
                    self._parent_cache[parent_file] = entry_keys(parent_file)
                skip = self._parent_cache[parent_file]
        dropped = 0
        if skip:
            dropped = sum(
                1
                for _, record in session.entries
                if (str(record.get("id") or ""), str(record.get("timestamp") or "")) in skip
            )
            if dropped:
                row = self.base_row(artifact)
                row.source_line = session.header_line
                row.timestamp_utc = to_utc(header.get("timestamp"))
                row.session_id, row.project_path, row.turn_type = session_id, cwd, "system"
                row.text = compact(
                    "fork: %d entries copied from the parent session %s omitted; "
                    "they are in the parent's rows" % (dropped, parent),
                )
                yield row

        yield from entry_rows(
            self, artifact, session, opts, session_id, cwd, skip if dropped else None
        )

        why = little_coder_markers(artifact, session)
        if why:
            row = self.base_row(artifact)
            row.source_line = session.header_line
            row.timestamp_utc = to_utc(header.get("timestamp"))
            row.session_id, row.project_path, row.turn_type = session_id, cwd, "system"
            row.text = compact(
                "session driven by little-coder (pi launcher): %s" % "; ".join(why),
            )
            yield row
        yield from error_rows(self, artifact, session.errors, session_id, cwd)

    def _parse_trust(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one standing row per project path of `trust.json`, at the file's mtime."""
        errors: list = []
        doc = json_object(artifact, errors)
        if doc is None:
            yield from error_rows(self, artifact, errors)
            return
        timestamp = file_mtime(artifact)
        for path, value in doc.items():
            row = self.base_row(artifact)
            row.source_line, row.turn_type, row.timestamp_utc = 1, "system", timestamp
            row.project_path = str(path)
            row.text = compact(
                "permission setting: %s=%s (from %s, file mtime)"
                % (path, setting_value(value), artifact.rel)
            )
            yield row

    def _parse_settings(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the `defaultProjectTrust` standing row of `settings.json`, at the file's mtime."""
        errors: list = []
        doc = json_object(artifact, errors)
        if doc is None:
            yield from error_rows(self, artifact, errors)
            return
        value = doc.get("defaultProjectTrust")
        if value in (None, ""):
            return
        row = self.base_row(artifact)
        row.source_line, row.turn_type, row.timestamp_utc = 1, "system", file_mtime(artifact)
        row.text = compact(
            "permission setting: defaultProjectTrust=%s (from %s, file mtime)"
            % (setting_value(value), artifact.rel)
        )
        yield row

    def _parse_debug_log(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the messages after the JSONL marker of `pi-debug.log`, with no session id.

        The screen lines before the marker are not read. A log that cannot be
        opened is its one unreadable row, with no snapshot row.
        """
        session = SessionFile()
        written = ""
        in_messages = False
        read_any = False
        for line_number, raw in iter_lines(artifact.disk_path, session.errors):
            read_any = True
            line = line_text(raw)
            if line_number == 1 and line.startswith(DEBUG_HEADER):
                written = to_utc(line[len(DEBUG_HEADER) :].strip())
            if not in_messages:
                in_messages = line == DEBUG_MARKER
                continue
            if not line.strip(JSON_SPACE):
                continue
            try:
                message = json.loads(line)
            except ValueError as error:
                session.errors.append((line_number, "unparseable line (%s)" % error, line))
                continue
            if not isinstance(message, dict):
                session.errors.append((line_number, "unparseable line (not a JSON object)", line))
                continue
            session.entries.append((line_number, {"type": "message", "message": message}))
        if not read_any and session.errors:
            yield from error_rows(self, artifact, session.errors)
            return
        row = self.base_row(artifact)
        row.source_line, row.turn_type, row.timestamp_utc = 1, "system", written
        row.text = compact(
            "debug snapshot: /debug wrote %d message(s) of the open session, which the log "
            "does not name (from %s, Debug output at line)" % (len(session.entries), artifact.rel)
        )
        yield row
        yield from entry_rows(self, artifact, session, opts)
        yield from error_rows(self, artifact, session.errors)

    def _parse_crashes(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one row per crash record of `crashes.json`, at its own time."""
        errors: list = []
        for index, record in iter_json_array(artifact.disk_path, errors):
            if not isinstance(record, dict):
                message = "unparseable record %d (not a JSON object)" % index
                errors.append((index, message, compact_json(record)))
                continue
            session_file = str(record.get("sessionFile") or "")
            match = SESSION_NAME_PATTERN.search(session_file.replace("\\", "/"))
            row = self.base_row(artifact)
            row.source_line, row.turn_type = index, "system"
            row.timestamp_utc = to_utc(record.get("timestamp"))
            row.session_id = match.group(1) if match else ""
            row.project_path = str(record.get("cwd") or "")
            text = "crash: %s %s version=%s sessionFile=%s (from %s, record timestamp)" % (
                _dash(record.get("kind")),
                _dash(record.get("message")),
                _dash(record.get("version")),
                _dash(record.get("sessionFile")),
                artifact.rel,
            )
            if record.get("stack"):
                text += "\n" + str(record.get("stack"))
            row.text = compact(text)
            yield row
        yield from error_rows(self, artifact, errors)

    def _parse_meta(self, artifact: Artifact, session_id: str, opts: Options) -> Iterator[Row]:
        errors: list = []
        meta = json_object(artifact, errors)
        if meta is None:
            yield from error_rows(self, artifact, errors, session_id)
            return
        row = self.base_row(artifact)
        row.session_id, row.turn_type, row.source_line = session_id, "system", 1
        row.timestamp_utc = to_utc(meta.get("createdAt"))
        row.project_path = str(meta.get("cwd") or "")
        row.text = compact(
            "experimental durable session created: cwd=%s (transcript in session.sqlite, "
            "not parsed)" % row.project_path,
        )
        yield row


def json_object(artifact: Artifact, errors: list) -> dict | None:
    """Return a whole-file JSON object, or None after appending its whole-file entry.

    A leading byte-order mark is dropped, as pi drops it from `trust.json`
    and `settings.json`. A file that cannot be read gets `read_text`'s entry;
    one that is not a JSON object gets `<rel> is not a JSON object`.
    """
    text = read_text(artifact.disk_path, errors)
    if text is None:
        return None
    try:
        doc = json.loads(text)
    except ValueError:
        doc = None
    if not isinstance(doc, dict):
        errors.append((0, "%s is not a JSON object" % artifact.rel))
        return None
    return doc


def _dash(value) -> str:
    """Return a record value as row text, `-` when it is absent or empty."""
    return "-" if value in (None, "") else setting_value(value)


def little_coder_markers(artifact: Artifact, session: SessionFile) -> list[str]:
    """Return why a pi session looks driven by little-coder, empty when it does not."""
    types: dict[str, None] = {}
    llamacpp = False
    for _, record in session.entries:
        if record.get("type") == "custom_message":
            custom_type = str(record.get("customType") or "")
            if custom_type.startswith("lc-"):
                types.setdefault(custom_type)
        elif record.get("type") == "message":
            message = record.get("message")
            if (
                isinstance(message, dict)
                and message.get("role") == "assistant"
                and message.get("provider") == "llamacpp"
            ):
                llamacpp = True
    why = []
    if types:
        why.append("custom_message types " + ", ".join(types))
    if llamacpp:
        why.append("assistant provider llamacpp")
    checkpoint = (
        artifact.home.disk_path / ".little-coder" / "checkpoints" / os.path.basename(artifact.rel)
    )
    if not checkpoint.is_symlink() and checkpoint.is_dir():
        why.append("checkpoints in ~/.little-coder/checkpoints/%s" % checkpoint.name)
    return why
