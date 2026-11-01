"""Claude Code transcripts, history, process registry, settings and Remote Control files.

Reads, under `~/.claude` or a `CLAUDE_CONFIG_DIR` home `~/.claude-<name>`: the transcripts
`projects/<slug>/<session>.jsonl` and `<session>/subagents/[<dir>/]agent-<id>.jsonl`, with
their `<session>/tool-results/<name>.txt` outputs; `ccr-tip.json` and `bridge-pointer.json`
under `projects/` (Remote Control); `history.jsonl` (a secondary source) with its
`paste-cache/`; the process registry `sessions/<pid>.json`; the user and project
`settings[.local].json`; and `~/.claude.json`. It yields user, assistant, thinking,
tool_use, tool_result and system rows, with surface, origin, sync, permission, model,
prompt trigger, title, scratch directory and setting rows.

Fields read: the envelope `type`, `timestamp`, `sessionId`, `cwd`, `gitBranch`,
`entrypoint`, `agentId`, `isMeta`, `remoteSourced`; `message.model` and `message.content`
blocks (`text`, `thinking`, `redacted_thinking`, `tool_use`, `tool_result`,
`tool_reference`), `requestedModel`, `wireToolInputs`, `isApiErrorMessage`,
`apiErrorStatus`, `error`; on `user` records `origin.kind`, `isCompactSummary`,
`interruptedMessageId`, `promptSource`, `turnOrigin`, `permissionMode`,
`permissionDecision`, `toolDenialKind`, `userFeedback`, `toolUseResult.persistedOutputPath`;
on `system` records `subtype`, `content`, `durationMs`, `messageCount`, `url`, `cron`,
`taskId`; on attachments `type`, `prompt`, `origin.kind`, `hookName`, `content`,
`filename`, `snapshot.scratchpadDirectory`; the state records in `TITLE_FIELDS` and
`permission-mode`, `relocated`, `pr-link`, `queue-operation`, `continued-in`,
`bridge-session`; the history's `display`, `pastedContents`, `project`, `sessionId`,
`timestamp`; the registry's `pid`, `sessionId`, `cwd`, `startedAt`, `kind`, `status`,
`entrypoint`, `bridgeSessionId`, `hostSessionId`; and the settings and `.claude.json` keys
the constants below name. No other settings key is read: several can hold credentials.

Validated against the shipped packages and real installs of 2.1.286 to 2.1.310 on Linux,
macOS and Windows (the CLI and the VS Code extension). See `research/claude-code.md`.
"""

from __future__ import annotations

import json
import re
from collections.abc import Iterator
from pathlib import Path

from ..dedup import MATCH_FORMS, MATCH_TOLERANCES
from ..inputs import Artifact
from ..model import ORIGIN_PREFIX, SCRATCH_PREFIX, SYNCED_PREFIX, Row, compact
from ..timeutil import to_utc
from .base import (
    Options,
    Parser,
    compact_json,
    error_rows,
    file_bytes,
    file_mtime,
    iter_jsonl,
    setting_value,
    text_of,
)

# The sync row of a teleported cloud session, beside its origin row: the
# vendor holds the conversation the `remoteSourced` records replay.
TELEPORT_SYNCED = (
    SYNCED_PREFIX
    + " cloud session (teleported: remoteSourced records replayed from the vendor's copy)"
)

SESSION_PATTERN = re.compile(r"^\.claude(?:-[^/]+)?/projects/[^/]+/.*\.jsonl$")
SUBAGENT_PATTERN = re.compile(r"/subagents/(?:[^/]+/)*agent-([^/]+)\.jsonl$")
HISTORY_PATTERN = re.compile(r"^\.claude(?:-[^/]+)?/history\.jsonl$")
REGISTRY_PATTERN = re.compile(r"^\.claude(?:-[^/]+)?/sessions/\d+\.json$")
SETTINGS_PATTERN = re.compile(r"^\.claude(?:-[^/]+)?/settings(?:\.local)?\.json$")
GLOBAL_CONFIG_PATTERN = re.compile(r"^(?:\.claude-[^/]+/)?\.claude\.json$")
# Remote Control sidecars beside the transcripts: `ccr-tip.json` ("ccr" is
# Claude Code Remote) in a session's directory, `bridge-pointer.json` in a project's.
CCR_TIP_PATTERN = re.compile(r"^\.claude(?:-[^/]+)?/projects/[^/]+/([^/]+)/ccr-tip\.json$")
BRIDGE_POINTER_PATTERN = re.compile(r"^\.claude(?:-[^/]+)?/projects/[^/]+/bridge-pointer\.json$")
# Settings `permissions` keys read, the lists first (the 2.1.296 settings schema).
PERMISSION_LISTS = ("allow", "deny", "ask", "additionalDirectories")
PERMISSION_SCALARS = ("defaultMode", "disableBypassPermissionsMode")
# `.claude.json` `projects.<path>` flags read beside `allowedTools`.
PROJECT_FLAGS = ("hasTrustDialogAccepted", "hasClaudeMdExternalIncludesApproved")
# The settings key that sets the retention sweep's period, and the
# `.claude.json` cache of the GitHub app connection.
RETENTION_KEY = "cleanupPeriodDays"
GITHUB_STATUS_KEY = "githubWebConnectionStatusCache"
# `.claude.json` Remote Control and cloud keys: the machine id (a UUID, never
# printed), the per-project spawn mode and the flag set when a cloud session is
# created from the CLI.
REMOTE_CONTROL_MACHINE_KEY = "remoteControlMachineId"
REMOTE_CONTROL_SPAWN_KEY = "remoteControlSpawnMode"
CLOUD_ENV_KEY = "hasRemoteEnvironment"
# The settings key that holds the model alias (`fable`, `opus`).
MODEL_KEY = "model"
# A history line's paste placeholder, `[Pasted text #<n> +<m> lines]` or
# `[Pasted text #<n>]`, and the `paste-cache/<contentHash>.txt` name.
PASTE_PATTERN = re.compile(r"\[Pasted text #([0-9]+)(?: \+[0-9]+ lines)?\]")
PASTE_HASH_PATTERN = re.compile(r"^[0-9A-Za-z]+$")
# The parts of a slash command's tag form in a transcript record.
COMMAND_NAME_PATTERN = re.compile(r"<command-name>(.*?)</command-name>", re.S)
COMMAND_ARGS_PATTERN = re.compile(r"<command-args>(.*?)</command-args>", re.S)
ASCII_SPACE_PATTERN = re.compile(r"[ \t\r\n\f\v]+")
# The wrapper 2.1.310 puts around a paste in the transcript's prompt,
# `<pasted_content id="<4 hex>">` ... `</pasted_content>`.
PASTE_WRAPPER_PATTERN = re.compile(r"</?pasted_content(?:[ \t\r\n][^>]*)?>")
# A history line that is a slash command as typed (`/model`, `/rename x`).
HISTORY_COMMAND_PATTERN = re.compile(r"^/[A-Za-z][A-Za-z0-9:_-]*(?:[ \t]|$)")
# How far a history command line may be from its transcript record: a
# command that opens a picker, an overlay or a flow (`/model`, `/help`,
# `/install-github-app`) is written to the transcript when it completes, which
# can be many seconds after the history line; a typed prompt is within the
# default two seconds.
COMMAND_TOLERANCE_SECONDS = 600.0
PERSISTED_MARK = "<persisted-output>"
PERSISTED_PATTERN = re.compile(
    r"Full output saved to: ([^\r\n]*?tool-results[/\\][^/\\\r\n]+?\.txt)"
)

# Which tool_use input field best identifies the action, per tool name.
TOOL_KEYS = {
    "Bash": ("command",),
    "Read": ("file_path",),
    "Write": ("file_path",),
    "Edit": ("file_path",),
    "MultiEdit": ("file_path",),
    "NotebookEdit": ("notebook_path",),
    "Glob": ("pattern", "path"),
    "Grep": ("pattern", "path"),
    "WebFetch": ("url",),
    "WebSearch": ("query",),
    "Agent": ("description", "prompt"),
    "Task": ("description", "prompt"),
    "Skill": ("skill", "args"),
    "LS": ("path",),
}

# Text markers at the start of a user record's text, and the label of the
# system row they become.
TEXT_MARKERS = (
    ("<task-notification>", "task notification"),
    ("<command-name>", "slash command"),
    ("<command-message>", "slash command"),
    ("<local-command-stdout>", "command output"),
    ("<local-command-caveat>", "context"),
    ("<system-reminder>", "context"),
    ("<fork-boilerplate>", "subagent task"),
)
FORK_MARK = "<fork-boilerplate>"

# `origin.kind` values that are not the person, and their labels.
ORIGIN_LABELS = {
    "task-notification": "task notification",
    "peer": "peer message",
    "coordinator": "coordinator message",
    "auto-continuation": "auto-continuation",
}

TITLE_FIELDS = {"ai-title": "aiTitle", "custom-title": "customTitle", "agent-name": "agentName"}

# `promptSource` and `turnOrigin` values of a prompt the person typed; any
# other value gets a `prompt trigger:` row.
TYPED_SOURCES = ("", "typed")
HUMAN_ORIGINS = ("", "human")

# Envelope fields of a system record, left out of its detail JSON.
SYSTEM_ENVELOPE = frozenset(
    (
        "type",
        "subtype",
        "uuid",
        "parentUuid",
        "logicalParentUuid",
        "timestamp",
        "sessionId",
        "session_id",
        "cwd",
        "gitBranch",
        "version",
        "isSidechain",
        "userType",
        "entrypoint",
        "slug",
        "agentId",
        "isMeta",
        "content",
        "sessionKind",
    )
)

TRANSCRIPT_TYPES = ("user", "assistant", "system", "attachment")

# `user` record keys appended to the `permission decision:` row, in this
# order; `userFeedback` is free text, so it comes last.
DENIAL_KEYS = ("toolDenialKind", "userFeedback")


def tool_summary(name: str, tool_input) -> str:
    """Return the input field that names a tool call's action (`TOOL_KEYS`), else its JSON."""
    if not isinstance(tool_input, dict):
        return compact_json(tool_input)
    keys = TOOL_KEYS.get(name)
    if keys:
        values = [str(tool_input[k]) for k in keys if tool_input.get(k) not in (None, "")]
        if values:
            return " | ".join(values)
    for k in ("command", "file_path", "path", "pattern", "url", "query", "description"):
        if tool_input.get(k) not in (None, ""):
            return str(tool_input[k])
    return compact_json(tool_input)


def result_text(content) -> str:
    """Return a tool_result's text; a `tool_reference` block is its tool name."""
    if isinstance(content, list):
        parts = []
        for block in content:
            if isinstance(block, dict) and block.get("type") == "tool_reference":
                parts.append(str(block.get("tool_name") or ""))
            else:
                parts.append(text_of([block]))
        return "\n".join(part for part in parts if part)
    return text_of(content)


def marker_label(text: str) -> str:
    """Return the system-row label of a text marker at the start of `text`, or ""."""
    head = text.lstrip()
    for mark, label in TEXT_MARKERS:
        if head.startswith(mark):
            return label
    return ""


def session_dir(artifact: Artifact) -> Path:
    """Return the directory that holds a transcript's sidecars.

    That is `<session>/` next to a session file, or the directory above `subagents/` for
    a subagent file.
    """
    path = artifact.disk_path
    if SUBAGENT_PATTERN.search(artifact.rel):
        directory = path.parent
        while directory.name != "subagents" and directory != directory.parent:
            directory = directory.parent
        return directory.parent
    return path.with_suffix("")


def persisted_text(
    artifact: Artifact, stub: str, path: str = "", errors: list | None = None
) -> str:
    """Return the full text of a persisted tool output's file.

    Returns the stub with a note when the file is not in this session's `tool-results/`
    or is a symlink, or with `[persisted output could not be read (<reason>)]` when it is
    there but cannot be read, its unreadable entry added to `errors`. `path` is the line's
    `toolUseResult.persistedOutputPath` (2.1.309); without it the path is read from a
    `<persisted-output>` stub's text.
    """
    if not path:
        match = PERSISTED_PATTERN.search(stub)
        if not match:
            return stub
        path = match.group(1)
    parts = re.split(r"[/\\]", path)
    name = parts[-1]
    session_directory = session_dir(artifact)
    note = "%s\n[persisted output not found: tool-results/%s]" % (stub, name)
    if (
        len(parts) < 3
        or parts[-2] != "tool-results"
        or parts[-3] != session_directory.name
        or name in ("", ".", "..")
    ):
        return note
    results_directory = session_directory / "tool-results"
    output_file = results_directory / name
    try:
        if (
            session_directory.is_symlink()
            or results_directory.is_symlink()
            or output_file.is_symlink()
            or not output_file.is_file()
        ):
            return note
    except OSError:
        return note
    failures: list = []
    data = file_bytes(output_file, failures)
    if data is None:
        message = failures[0][1]
        if errors is not None:
            errors.extend(failures)
        # The entry reads `<name> could not be read (<reason>)`; the note keeps the reason.
        reason = message.partition(" could not be read (")[2][:-1]
        return "%s\n[persisted output could not be read (%s)]" % (stub, reason)
    return data.decode("utf-8", errors="replace")


def match_form(row: Row) -> tuple[str, str]:
    """Return the `(turn_type, text)` a row is deduplicated under (`dedup.MATCH_FORMS`).

    A slash command, typed in `history.jsonl` and in tag form in the transcript (a
    `slash command:` or `local_command:` row), matches as the `user` text `/<name>` plus
    ` <args>` when `<command-args>` is not empty. A prompt matches with the transcript's
    `<pasted_content id="...">` wrapper removed and each run of ASCII whitespace collapsed
    to one space, since the history holds the bare paste. Every other row matches as it is.
    """
    text = row.text
    if row.turn_type == "system":
        if text.startswith(("slash command: ", "local_command: ")):
            name = COMMAND_NAME_PATTERN.search(text)
            if name:
                command = name.group(1).strip(" \t\r\n")
                if not command.startswith("/"):
                    command = "/" + command
                args = COMMAND_ARGS_PATTERN.search(text)
                if args and args.group(1).strip(" \t\r\n"):
                    command += " " + args.group(1).strip(" \t\r\n")
                return "user", _squash(command)
        return row.turn_type, text
    if row.turn_type == "user":
        return "user", _squash(PASTE_WRAPPER_PATTERN.sub(" ", text))
    return row.turn_type, text


def _squash(text: str) -> str:
    """Return `text` with each run of ASCII whitespace collapsed to one space, trimmed."""
    return ASCII_SPACE_PATTERN.sub(" ", text).strip(" ")


def match_tolerance(row: Row) -> float | None:
    """Return the forward time window of a history row that is a slash command as typed.

    That is `COMMAND_TOLERANCE_SECONDS`, how long after the line its transcript record may
    be written; any other row gets None, the source's own window (`dedup.MATCH_TOLERANCES`).
    """
    if row.turn_type == "user" and HISTORY_COMMAND_PATTERN.match(row.text):
        return COMMAND_TOLERANCE_SECONDS
    return None


def read_paste(history: Path, content_hash: str) -> tuple[str | None, str]:
    """Return `(text, problem)` for `paste-cache/<content_hash>.txt` beside `history`.

    Returns the file's text and ""; `(None, "")` when the name is not a plain hash, the
    file is missing, or the directory or file is a symlink, which is never followed; and
    `(None, "<message>")` when a file that is there could not be read, since a read
    failure is evidence, not an absent cache entry.
    """
    if not PASTE_HASH_PATTERN.match(content_hash):
        return None, ""
    cache = history.parent / "paste-cache"
    cache_file = cache / (content_hash + ".txt")
    try:
        if cache.is_symlink() or cache_file.is_symlink() or not cache_file.is_file():
            return None, ""
        return cache_file.read_bytes().decode("utf-8", errors="replace"), ""
    except OSError as error:
        return None, "paste-cache/%s.txt unreadable (%s)" % (content_hash, error)


def expand_pastes(
    history: Path, display: str, pasted, errors: list | None = None, line: int = 0
) -> str:
    """Return `display` with each `[Pasted text #<n> ...]` placeholder expanded.

    The text is `pastedContents.<n>.content` when the paste is kept inline, else
    `paste-cache/<contentHash>.txt`; a placeholder whose text is in neither place is kept
    as written. A cache file that is there but cannot be read keeps the placeholder too and
    adds `(line, message, "")` to `errors`, which `error_rows` reports.
    """
    if not isinstance(pasted, dict) or not pasted:
        return display

    def replace_paste(match: re.Match) -> str:
        entry = pasted.get(match.group(1))
        if not isinstance(entry, dict) or entry.get("type", "text") != "text":
            return match.group(0)
        content = entry.get("content")
        if isinstance(content, str):
            return content
        content_hash = entry.get("contentHash")
        text, problem = (
            read_paste(history, content_hash) if isinstance(content_hash, str) else (None, "")
        )
        if problem and errors is not None:
            errors.append((line, problem, ""))
        return match.group(0) if text is None else text

    return PASTE_PATTERN.sub(replace_paste, display)


class _Prescan:
    """Hold what `_parse_session` needs to know about a transcript before its first row.

    That is the lines delivering each queued prompt, whether the file is a fork, the first
    cwd and branch, the `(line, url)` of each `bridge_status` record and `(line, id)` of
    each `bridge-session` record, whether an `sdk-cli` `user` record has `promptSource`
    `sdk` (a Remote Control spawn), and the line of the last `remoteSourced` record (a
    teleported cloud session; 0 when none), which ends the replayed block.
    """

    __slots__ = (
        "branch",
        "bridge_lines",
        "bridge_urls",
        "cwd",
        "delivered",
        "fork",
        "remote_last",
        "sdk_spawn",
    )

    def __init__(self) -> None:
        self.delivered: dict[str, list[int]] = {}
        self.fork = False
        self.cwd = ""
        self.branch = ""
        self.bridge_urls: list[tuple[int, str]] = []
        self.bridge_lines: list[tuple[int, str | None]] = []
        self.sdk_spawn = False
        self.remote_last = 0


class ClaudeCodeParser(Parser):
    """Parse Claude Code's transcripts and the history, registry and settings beside them."""

    agent = "claude-code"
    name = "claude-code"
    has_secondary = True
    # A Remote Control bridge puts the session on claude.ai.
    records_sync = True

    def __init__(self) -> None:
        """Set `records_origin` for claude-code only, not its claude-desktop subclass."""
        # A Remote Control spawn or a teleported cloud session began on
        # claude.ai; the desktop's origin is not established, so it keeps "".
        self.records_origin = self.agent == "claude-code"

    def wants(self, artifact: Artifact) -> bool:
        """Return True for a transcript, history, registry, settings or sidecar file."""
        rel = artifact.rel
        return any(
            pattern.match(rel)
            for pattern in (
                HISTORY_PATTERN,
                SESSION_PATTERN,
                REGISTRY_PATTERN,
                SETTINGS_PATTERN,
                GLOBAL_CONFIG_PATTERN,
                CCR_TIP_PATTERN,
                BRIDGE_POINTER_PATTERN,
            )
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for `history.jsonl`."""
        return bool(HISTORY_PATTERN.match(artifact.rel))

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one Claude Code file."""
        if HISTORY_PATTERN.match(artifact.rel):
            yield from self._parse_history(artifact, opts)
        elif REGISTRY_PATTERN.match(artifact.rel):
            yield from self._parse_registry(artifact)
        elif SETTINGS_PATTERN.match(artifact.rel):
            yield from self._parse_settings(artifact)
        elif GLOBAL_CONFIG_PATTERN.match(artifact.rel):
            yield from self._parse_global_config(artifact)
        elif CCR_TIP_PATTERN.match(artifact.rel):
            yield from self._parse_ccr_tip(artifact)
        elif BRIDGE_POINTER_PATTERN.match(artifact.rel):
            yield from self._parse_bridge_pointer(artifact)
        else:
            yield from self._parse_session(artifact, opts)

    def _parse_history(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            row = self.base_row(artifact)
            row.source_line = line_number
            row.timestamp_utc = to_utc(record.get("timestamp"))
            row.session_id = str(record.get("sessionId") or "")
            row.project_path = str(record.get("project") or "")
            row.turn_type = "user"
            display = record.get("display")
            if isinstance(display, str):
                display = expand_pastes(
                    artifact.disk_path, display, record.get("pastedContents"), errors, line_number
                )
            row.text = compact(display)
            yield row
        yield from error_rows(self, artifact, errors)

    def _parse_ccr_tip(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the `session synced:` row of a session's `ccr-tip.json`.

        The file's `eventId` and `updatedAt` name the last claude.ai event the local side of
        a Remote Control session processed; the row is at the file's modification time.
        """
        match = CCR_TIP_PATTERN.match(artifact.rel)
        session = match.group(1) if match else ""
        doc, bad = self._settings_doc(artifact, "ccr-tip.json")
        if doc is None:
            if bad is not None:
                bad.session_id = session
                yield bad
            return
        row = self._file_row(artifact)
        row.session_id = session
        row.text = compact(
            "%s remote event %s at %s (from %s, file mtime)"
            % (
                SYNCED_PREFIX,
                self._value_text(doc, "eventId"),
                self._value_text(doc, "updatedAt"),
                artifact.rel,
            )
        )
        yield row

    def _parse_bridge_pointer(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the `remote control:` row of a project's `bridge-pointer.json`.

        The file names the Remote Control environment last enabled for the project
        (`environmentId`, `sessionId`, `source`). The row is at the file's modification time;
        `sessionId` is the bridge's, not a transcript's, so `session_id` stays empty.
        """
        doc, bad = self._settings_doc(artifact, "bridge-pointer.json")
        if doc is None:
            if bad is not None:
                yield bad
            return
        row = self._file_row(artifact)
        row.text = compact(
            "remote control: environment %s session %s source=%s (from %s, file mtime)"
            % (
                self._value_text(doc, "environmentId"),
                self._value_text(doc, "sessionId"),
                self._value_text(doc, "source"),
                artifact.rel,
            )
        )
        yield row

    @staticmethod
    def _value_text(doc: dict, key: str) -> str:
        """Return a JSON value as row text, `-` when missing or empty."""
        value = doc.get(key)
        return "-" if value in (None, "") else setting_value(value)

    def _file_row(self, artifact: Artifact) -> Row:
        """Return a `system` row at the file's modification time."""
        row = self.base_row(artifact)
        row.source_line = 1
        row.turn_type = "system"
        row.timestamp_utc = file_mtime(artifact)
        return row

    def _parse_registry(self, artifact: Artifact) -> Iterator[Row]:
        row = self.base_row(artifact)
        row.source_line = 1
        row.turn_type = "system"
        errors: list = []
        data = file_bytes(artifact.disk_path, errors)
        if data is None:
            yield from error_rows(self, artifact, errors)
            return
        try:
            record = json.loads(data.decode("utf-8", errors="replace"))
        except ValueError:
            record = None
        if not isinstance(record, dict):
            row.text = "parser: session registry file is not a JSON object"
            yield row
            return

        def field_text(key: str, missing: str) -> str:
            value = record.get(key)
            return missing if value in (None, "") else str(value)

        row.timestamp_utc = to_utc(record.get("startedAt"))
        row.session_id = field_text("sessionId", "")
        row.project_path = field_text("cwd", "")
        row.text = "claude-code process %s %s status=%s entrypoint=%s bridge=%s host_session=%s" % (
            field_text("pid", artifact.disk_path.stem),
            field_text("kind", "-"),
            field_text("status", "-"),
            field_text("entrypoint", "-"),
            field_text("bridgeSessionId", "none"),
            field_text("hostSessionId", "none"),
        )
        yield row
        bridge = record.get("bridgeSessionId")
        if isinstance(bridge, str) and bridge:
            sync = self.base_row(artifact)
            sync.source_line = 1
            sync.turn_type = "system"
            sync.timestamp_utc = file_mtime(artifact)
            sync.session_id = row.session_id
            sync.project_path = row.project_path
            sync.text = "%s remote control bridge %s url - (from %s, file mtime)" % (
                SYNCED_PREFIX,
                bridge,
                artifact.rel,
            )
            yield sync

    def _standing_row(
        self,
        artifact: Artifact,
        timestamp: str,
        project: str,
        key: str,
        value,
        label: str = "permission setting",
    ) -> Row:
        row = self.base_row(artifact)
        row.source_line = 1
        row.turn_type = "system"
        row.timestamp_utc = timestamp
        row.project_path = project
        row.text = compact(
            "%s: %s=%s (from %s, file mtime)" % (label, key, setting_value(value), artifact.rel)
        )
        return row

    def _settings_doc(self, artifact: Artifact, what: str) -> tuple[dict | None, Row | None]:
        """Return the file as a JSON object, or a `system` row saying why it is not one.

        A file that cannot be read gives the shared unreadable row; one that reads but is
        not a JSON object gives `parser: <what> is not a JSON object`.
        """
        errors: list = []
        data = file_bytes(artifact.disk_path, errors)
        if data is None:
            return None, next(error_rows(self, artifact, errors))
        try:
            doc = json.loads(data.decode("utf-8", errors="replace"))
        except ValueError:
            doc = None
        if isinstance(doc, dict):
            return doc, None
        row = self.base_row(artifact)
        row.source_line = 1
        row.turn_type = "system"
        row.timestamp_utc = file_mtime(artifact)
        row.text = "parser: %s is not a JSON object" % what
        return None, row

    def _parse_settings(self, artifact: Artifact) -> Iterator[Row]:
        doc, bad = self._settings_doc(artifact, "settings file")
        if doc is None:
            if bad is not None:
                yield bad
            return
        timestamp = file_mtime(artifact)
        project = artifact.home.original if artifact.home.project else ""
        permissions = doc.get("permissions")
        if isinstance(permissions, dict):
            for key in PERMISSION_LISTS:
                entries = permissions.get(key)
                if isinstance(entries, list):
                    for entry in entries:
                        yield self._standing_row(
                            artifact, timestamp, project, "permissions." + key, entry
                        )
            for key in PERMISSION_SCALARS:
                if permissions.get(key) is not None:
                    yield self._standing_row(
                        artifact, timestamp, project, "permissions." + key, permissions[key]
                    )
        if doc.get(RETENTION_KEY) is not None:
            yield self._standing_row(
                artifact, timestamp, project, RETENTION_KEY, doc[RETENTION_KEY], "retention setting"
            )
        if isinstance(doc.get(MODEL_KEY), str) and doc[MODEL_KEY]:
            yield self._standing_row(
                artifact, timestamp, project, MODEL_KEY, doc[MODEL_KEY], "model setting"
            )

    def _parse_global_config(self, artifact: Artifact) -> Iterator[Row]:
        doc, bad = self._settings_doc(artifact, ".claude.json")
        if doc is None:
            if bad is not None:
                yield bad
            return
        timestamp = file_mtime(artifact)
        projects = doc.get("projects")
        for path, project in projects.items() if isinstance(projects, dict) else ():
            if not isinstance(project, dict):
                continue
            tools = project.get("allowedTools")
            if isinstance(tools, list):
                for entry in tools:
                    yield self._standing_row(artifact, timestamp, str(path), "allowedTools", entry)
            for key in PROJECT_FLAGS:
                if project.get(key) is not None:
                    yield self._standing_row(artifact, timestamp, str(path), key, project[key])
            spawn = project.get(REMOTE_CONTROL_SPAWN_KEY)
            if spawn not in (None, ""):
                row = self._file_row(artifact)
                row.timestamp_utc = timestamp
                row.project_path = str(path)
                row.text = compact(
                    "remote control: spawn mode %s for %s (from %s, file mtime)"
                    % (setting_value(spawn), path, artifact.rel)
                )
                yield row
        github = doc.get(GITHUB_STATUS_KEY)
        if isinstance(github, dict):
            # accountUuid and orgUuid are identity values: never printed.
            status = github.get("status")
            row = self.base_row(artifact)
            row.source_line = 1
            row.turn_type = "system"
            row.timestamp_utc = timestamp
            row.text = compact(
                "source host: github status=%s (from %s, file mtime)"
                % ("-" if status in (None, "") else setting_value(status), artifact.rel)
            )
            yield row
        if doc.get(REMOTE_CONTROL_MACHINE_KEY) not in (None, ""):
            # The machine id is an identity value: never printed.
            row = self._file_row(artifact)
            row.timestamp_utc = timestamp
            row.text = "remote control: machine id set (from %s, file mtime)" % artifact.rel
            yield row
        if doc.get(CLOUD_ENV_KEY) is not None:
            row = self._file_row(artifact)
            row.timestamp_utc = timestamp
            row.text = compact(
                "cloud: %s=%s (from %s, file mtime)"
                % (CLOUD_ENV_KEY, setting_value(doc[CLOUD_ENV_KEY]), artifact.rel)
            )
            yield row

    @staticmethod
    def _prescan(artifact: Artifact) -> _Prescan:
        """Return the `_Prescan` of a transcript file.

        A queue enqueue points at the line that delivers its prompt, state records before
        any transcript record take the first cwd and branch, and a `bridge-session` record
        takes its URL from the `bridge_status` lines its neighbours bound. The errors are
        discarded: `_parse_session` parses and reports on the same file in full.
        """
        prescan = _Prescan()

        def deliver(text: str, at_line: int) -> None:
            if text:
                prescan.delivered.setdefault(text, []).append(at_line)

        for line_number, record in iter_jsonl(artifact.disk_path, []):
            record_type = record.get("type")
            if not prescan.cwd and record.get("cwd"):
                prescan.cwd = str(record.get("cwd"))
            if not prescan.branch and record.get("gitBranch"):
                prescan.branch = str(record.get("gitBranch"))
            if record.get("remoteSourced") is True:
                prescan.remote_last = line_number
            if (
                record_type == "system"
                and record.get("subtype") == "bridge_status"
                and record.get("url")
            ):
                prescan.bridge_urls.append((line_number, str(record.get("url"))))
            if record_type == "bridge-session":
                bridge_id = record.get("bridgeSessionId")
                prescan.bridge_lines.append(
                    (line_number, bridge_id if isinstance(bridge_id, str) and bridge_id else None)
                )
            if record_type == "attachment":
                attachment = record.get("attachment") or {}
                if isinstance(attachment, dict) and attachment.get("type") == "queued_command":
                    deliver(text_of(attachment.get("prompt")), line_number)
            elif record_type == "user":
                if record.get("entrypoint") == "sdk-cli" and record.get("promptSource") == "sdk":
                    prescan.sdk_spawn = True
                content = (record.get("message") or {}).get("content")
                if isinstance(content, str):
                    deliver(content, line_number)
                elif isinstance(content, list):
                    for block in content:
                        if isinstance(block, dict) and block.get("type") == "text":
                            block_text = str(block.get("text") or "")
                            deliver(block_text, line_number)
                            if block_text.lstrip().startswith(FORK_MARK):
                                prescan.fork = True
        return prescan

    @staticmethod
    def _origin_text(record, prescan: _Prescan) -> str:
        """Return the `session origin:` text a record marks, or "".

        A mark is an `sdk-cli` record in a Remote Control spawn's file, a `remoteSourced`
        record (a teleported cloud turn), or a record of the VS Code extension.
        """
        entrypoint = record.get("entrypoint")
        if entrypoint == "sdk-cli" and prescan.sdk_spawn:
            return ORIGIN_PREFIX + " (Remote Control, entrypoint sdk-cli)"
        if record.get("remoteSourced") is True:
            return ORIGIN_PREFIX + " (teleport, remoteSourced)"
        if entrypoint == "claude-vscode":
            return "session origin: local (VS Code extension)"
        return ""

    def _parse_session(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        errors: list = []
        prescan = self._prescan(artifact)
        delivered = prescan.delivered
        subagent = SUBAGENT_PATTERN.search(artifact.rel)
        file_agent_id = subagent.group(1) if subagent else ""
        header_done = not subagent
        task_done = not subagent or prescan.fork
        session_id = ""
        last_timestamp = ""
        last_cwd = prescan.cwd
        last_branch = prescan.branch
        seen_origin: set = set()
        relocated = ""
        seen_pr: set = set()
        seen_title: set = set()
        modes: dict[str, str] = {}
        models: dict[str, str] = {}
        surface = ""
        seen_scratch: set = set()
        bridges: dict[str, tuple] = {}
        seen_sync: set = set()

        for line_number, record in iter_jsonl(artifact.disk_path, errors):
            record_type = record.get("type")
            record_session_id = record.get("sessionId") or record.get("session_id")
            if record_session_id:
                session_id = str(record_session_id)
            timestamp = to_utc(record.get("timestamp"))
            if timestamp:
                last_timestamp = timestamp
            if record.get("cwd"):
                last_cwd = str(record.get("cwd"))
            if record.get("gitBranch"):
                last_branch = str(record.get("gitBranch"))

            def base(timestamp=timestamp) -> Row:
                row = self.base_row(artifact)
                row.source_line = line_number
                row.timestamp_utc = timestamp
                row.session_id = session_id
                row.project_path = relocated or last_cwd
                row.git_branch = last_branch
                return row

            def state_row(text: str) -> Row:
                row = base(timestamp or last_timestamp)
                row.turn_type = "system"
                row.text = compact(text)
                return row

            if not header_done and record_type in TRANSCRIPT_TYPES:
                header_done = True
                row = base()
                row.turn_type = "system"
                row.text = "subagent %s of %s" % (
                    record.get("agentId") or file_agent_id,
                    session_id,
                )
                yield row

            entrypoint = record.get("entrypoint")
            if isinstance(entrypoint, str) and entrypoint and entrypoint != surface:
                yield state_row(
                    "session surface: %s -> %s" % (surface, entrypoint)
                    if surface
                    else "session surface: " + entrypoint
                )
                surface = entrypoint
            origin = self._origin_text(record, prescan) if record_type in TRANSCRIPT_TYPES else ""
            if origin and (session_id, origin) not in seen_origin:
                seen_origin.add((session_id, origin))
                yield state_row(origin)
                if origin.endswith("(teleport, remoteSourced)"):
                    # The conversation is held on the vendor's servers and
                    # replayed here, so the session is synced as well.
                    yield state_row(TELEPORT_SYNCED)

            mode = (
                record.get("permissionMode") if record_type in ("user", "permission-mode") else None
            )
            if isinstance(mode, str) and mode and modes.get(session_id) != mode:
                old = modes.get(session_id)
                modes[session_id] = mode
                yield state_row(
                    "permission mode: %s -> %s" % (old, mode) if old else "permission mode: " + mode
                )
            if record_type == "assistant":
                model_row = self._model_row(record, base, models, session_id)
                if model_row is not None:
                    yield model_row

            if record_type == "user":
                yield from self._user(record, base, artifact, opts, not task_done, errors)
                task_done = True
            elif record_type == "assistant":
                yield from self._assistant(record, base, opts)
            elif record_type == "system":
                row = base()
                row.turn_type = "system"
                subtype = str(record.get("subtype") or "system")
                detail = text_of(record.get("content"))
                if subtype == "turn_duration":
                    detail = "%s ms, %s messages" % (
                        record.get("durationMs"),
                        record.get("messageCount"),
                    )
                elif subtype == "scheduled_task_fire":
                    row.text = compact(
                        "scheduled task fired: %s cron=%s"
                        % (self._value_text(record, "taskId"), self._value_text(record, "cron"))
                    )
                    yield row
                    continue
                elif not detail:
                    rest = {k: value for k, value in record.items() if k not in SYSTEM_ENVELOPE}
                    detail = compact_json(rest) if rest else ""
                row.text = compact("%s: %s" % (subtype, detail) if detail else subtype)
                yield row
            elif record_type == "attachment":
                row = self._attachment(record, base)
                if row is not None:
                    yield row
                scratch = self._scratchpad(record)
                if scratch and (session_id, scratch) not in seen_scratch:
                    seen_scratch.add((session_id, scratch))
                    if line_number <= prescan.remote_last:
                        # A teleport replays the cloud session's records
                        # before the local process writes its own, so this
                        # environment is the cloud machine's: its scratchpad
                        # is not on this host and marks the session's origin.
                        yield state_row(
                            "%s (teleport, cloud scratchpad %s)" % (ORIGIN_PREFIX, scratch)
                        )
                        continue
                    row = state_row(
                        "%s %s (session scratchpad, from the environment attachment)"
                        % (SCRATCH_PREFIX, scratch)
                    )
                    row.project_path = scratch
                    yield row
            elif record_type == "continued-in":
                yield state_row(
                    "session continued in: %s" % (record.get("continuedInSessionId") or "-")
                )
            elif record_type == "bridge-session":
                yield from self._bridge(
                    record,
                    line_number,
                    state_row,
                    session_id,
                    bridges,
                    seen_sync,
                    prescan.bridge_urls,
                    prescan.bridge_lines,
                )
            elif record_type in TITLE_FIELDS:
                title = str(record.get(TITLE_FIELDS[record_type]) or "")
                if title and (session_id, title) not in seen_title:
                    seen_title.add((session_id, title))
                    yield state_row("session title: " + title)
            elif record_type == "relocated":
                new_cwd = str(record.get("relocatedCwd") or "")
                if new_cwd:
                    relocated = new_cwd
                    yield state_row("cwd changed: " + new_cwd)
            elif record_type == "pr-link":
                url = str(record.get("prUrl") or record.get("prNumber") or "")
                if (session_id, url) not in seen_pr:
                    seen_pr.add((session_id, url))
                    yield state_row("pr-link: " + url)
            elif record_type == "queue-operation":
                content = text_of(record.get("content"))
                lines = delivered.get(content) if record.get("operation") == "enqueue" else None
                if lines:
                    delivered_at = next(
                        (
                            delivered_line
                            for delivered_line in lines
                            if delivered_line > line_number
                        ),
                        lines[0],
                    )
                    yield state_row("prompt queued: delivered at line %d" % delivered_at)
                    continue
                yield state_row("queue %s: %s" % (record.get("operation"), content))
        yield from error_rows(self, artifact, errors, session_id)

    @staticmethod
    def _model_row(record, base, models: dict, session_id: str) -> Row | None:
        """Return the `model:` row of a session's first or changed `message.model`, or None."""
        message = record.get("message")
        model = str(message.get("model") or "") if isinstance(message, dict) else ""
        if not model or model == "<synthetic>" or record.get("isApiErrorMessage"):
            return None
        old = models.get(session_id)
        if old == model:
            return None
        models[session_id] = model
        text = "model: %s -> %s" % (old, model) if old else "model: " + model
        requested = record.get("requestedModel")
        if requested not in (None, "") and str(requested) != model:
            text += " requested=%s" % requested
        row = base()
        row.turn_type = "system"
        row.model = model
        row.text = compact(text)
        return row

    @staticmethod
    def _bridge(
        record, line_number: int, state_row, session_id: str, bridges, seen_sync, urls, lines
    ) -> Iterator[Row]:
        """Yield the rows of a `bridge-session` record.

        That is `remote control: bridge session ...` when the bridge id or sequence number
        changes, and `session synced:` the first time a session names a bridge id. Its URL
        is that of the first `bridge_status` record (`urls`) at or after the line, else the
        nearest before, among those between the previous and next `bridge-session` record
        of another id (`lines`, a detach counting), else `-`.
        """
        bridge = record.get("bridgeSessionId")
        sequence = record.get("lastSequenceNum")
        if bridges.get(session_id) != (bridge, sequence):
            bridges[session_id] = (bridge, sequence)
            yield state_row(
                "remote control: bridge session %s lastSequenceNum=%s"
                % (bridge or "none", "-" if sequence in (None, "") else sequence)
            )
        if bridge and (session_id, bridge) not in seen_sync:
            seen_sync.add((session_id, bridge))
            start = max(
                (
                    other_line
                    for other_line, other_bridge in lines
                    if other_line < line_number and other_bridge != bridge
                ),
                default=0,
            )
            end = min(
                (
                    other_line
                    for other_line, other_bridge in lines
                    if other_line > line_number and other_bridge != bridge
                ),
                default=None,
            )
            after = [
                status_url
                for other_line, status_url in urls
                if other_line >= line_number and (end is None or other_line < end)
            ]
            before = [
                status_url for other_line, status_url in urls if start < other_line < line_number
            ]
            url = after[0] if after else (before[-1] if before else "-")
            yield state_row("%s remote control bridge %s url %s" % (SYNCED_PREFIX, bridge, url))

    @staticmethod
    def _scratchpad(record) -> str:
        """Return an `environment` attachment's `snapshot.scratchpadDirectory`, or ""."""
        attachment = record.get("attachment")
        if not isinstance(attachment, dict) or attachment.get("type") != "environment":
            return ""
        snapshot = attachment.get("snapshot")
        path = snapshot.get("scratchpadDirectory") if isinstance(snapshot, dict) else None
        return path if isinstance(path, str) else ""

    def _user(
        self, record, base, artifact: Artifact, opts: Options, is_task: bool, errors: list
    ) -> Iterator[Row]:
        yield from self._user_blocks(record, base, artifact, is_task, errors)
        source = record.get("promptSource")
        origin = record.get("turnOrigin")
        source = "" if source is None else str(source)
        origin = "" if origin is None else str(origin)
        if source not in TYPED_SOURCES or origin not in HUMAN_ORIGINS:
            row = base()
            row.turn_type = "system"
            row.text = compact(
                "prompt trigger: promptSource=%s turnOrigin=%s" % (source or "-", origin or "-")
            )
            yield row
        decision = record.get("permissionDecision")
        denial = any(record.get(k) not in (None, "") for k in DENIAL_KEYS)
        if decision is not None or denial:
            yield self._permission_row(record, decision, base)

    @staticmethod
    def _permission_row(record, decision, base) -> Row:
        """Return the `permission decision:` row of a `user` record (2.1.295 and later)."""
        row = base()
        row.turn_type = "system"
        content = (record.get("message") or {}).get("content")
        if isinstance(content, list):
            first = next(
                (
                    block
                    for block in content
                    if isinstance(block, dict) and block.get("type") == "tool_result"
                ),
                None,
            )
            if first is not None:
                row.tool_use_id = str(first.get("tool_use_id") or "")
        denial = "".join(
            " %s=%s" % (k, setting_value(record[k]))
            for k in DENIAL_KEYS
            if record.get(k) not in (None, "")
        )
        if decision is None and record.get("toolDenialKind") not in (None, ""):
            # A `No` at the permission dialog (with or without the Tab
            # amendment) and a declined plan interrupt the turn and write no
            # decision record, only the denial kind (2.1.310).
            row.text = compact("permission decision: reject (interrupted)" + denial)
            return row
        if decision is None:
            decision = {}
        if not isinstance(decision, dict):
            row.text = compact("permission decision: " + compact_json(decision) + denial)
            return row

        def field_text(key: str) -> str:
            value = decision.get(key)
            return "-" if value in (None, "") else str(value)

        text = "permission decision: %s source=%s reason=%s" % (
            field_text("decision"),
            field_text("source"),
            field_text("reasonType"),
        )
        if set(decision) - {"decision", "source", "reasonType"}:
            text += " " + compact_json(decision)
        row.text = compact(text + denial)
        return row

    def _user_blocks(
        self, record, base, artifact: Artifact, is_task: bool, errors: list
    ) -> Iterator[Row]:
        content = (record.get("message") or {}).get("content")
        origin = record.get("origin") or {}
        kind = origin.get("kind") if isinstance(origin, dict) else None
        if record.get("isCompactSummary"):
            record_label = "compaction summary"
        elif is_task:
            record_label = "subagent task"
        elif record.get("interruptedMessageId"):
            record_label = "interrupted"
        else:
            record_label = ""

        def text_row(text: str) -> Row:
            label = (
                record_label
                or marker_label(text)
                or ORIGIN_LABELS.get(str(kind), "")
                or ("context" if record.get("isMeta") else "")
            )
            row = base()
            if label:
                row.turn_type = "system"
                text = "%s: %s" % (label, text)
            else:
                row.turn_type = "user"
            row.text = compact(text)
            return row

        if not isinstance(content, list):
            yield text_row(text_of(content))
            return
        # 2.1.309 names the persisted output's file in `toolUseResult`; the
        # key belongs to the line's one tool call, so it is used only when
        # the line has one tool_result block.
        tool_use_result = record.get("toolUseResult")
        saved = (
            tool_use_result.get("persistedOutputPath")
            if isinstance(tool_use_result, dict)
            else None
        )
        results = [
            candidate
            for candidate in content
            if isinstance(candidate, dict) and candidate.get("type") == "tool_result"
        ]
        if not isinstance(saved, str) or len(results) != 1:
            saved = ""
        for block in content:
            if not isinstance(block, dict):
                continue
            block_type = block.get("type")
            if block_type == "tool_result":
                row = base()
                row.turn_type = "tool_result"
                row.tool_use_id = str(block.get("tool_use_id") or "")
                text = result_text(block.get("content"))
                if saved:
                    text = persisted_text(artifact, text, saved, errors)
                elif text.lstrip().startswith(PERSISTED_MARK):
                    text = persisted_text(artifact, text, errors=errors)
                if block.get("is_error"):
                    text = "[error] " + text
                row.text = compact(text)
                yield row
            elif block_type in ("text", "image"):
                yield text_row(text_of([block]))

    def _assistant(self, record, base, opts: Options) -> Iterator[Row]:
        message = record.get("message") or {}
        model = str(message.get("model") or "")
        content = message.get("content")
        if record.get("isApiErrorMessage") or model == "<synthetic>":
            row = base()
            row.turn_type = "system"
            status = " ".join(
                str(value)
                for value in (record.get("apiErrorStatus"), record.get("error"))
                if value not in (None, "")
            )
            text = text_of(content)
            row.text = compact(
                "api error: %s" % ": ".join(part for part in (status, text) if part),
            )
            yield row
            return
        wire = record.get("wireToolInputs")
        blocks = (
            content if isinstance(content, list) else [{"type": "text", "text": text_of(content)}]
        )
        for block in blocks:
            if not isinstance(block, dict):
                continue
            block_type = block.get("type")
            if block_type == "tool_use":
                row = base()
                row.turn_type = "tool_use"
                row.model = model
                row.tool_name = str(block.get("name") or "")
                row.tool_use_id = str(block.get("id") or "")
                tool_input = block.get("input")
                if row.tool_name == "Bash" and isinstance(wire, dict):
                    sent = wire.get(row.tool_use_id)
                    if isinstance(sent, dict) and sent.get("command"):
                        tool_input = sent
                row.text = compact(tool_summary(row.tool_name, tool_input))
                yield row
            elif block_type == "text":
                text = block.get("text")
                if not text:
                    continue
                row = base()
                row.turn_type = "assistant"
                row.model = model
                row.text = compact(text)
                yield row
            elif block_type in ("thinking", "redacted_thinking") and opts.include_thinking:
                if block_type == "thinking":
                    text = block.get("thinking")
                    if not text:
                        continue
                else:
                    text = "[redacted]"
                row = base()
                row.turn_type = "thinking"
                row.model = model
                row.text = compact(text)
                yield row

    @staticmethod
    def _attachment(record, base) -> Row | None:
        attachment = record.get("attachment") or {}
        if not isinstance(attachment, dict):
            return None
        attachment_type = attachment.get("type")
        if attachment_type == "queued_command":
            text = text_of(attachment.get("prompt"))
            origin = attachment.get("origin") or {}
            kind = origin.get("kind") if isinstance(origin, dict) else None
            row = base()
            if kind == "human" and not attachment.get("isMeta"):
                label = marker_label(text)
            else:
                label = marker_label(text) or ORIGIN_LABELS.get(str(kind), "") or "queued command"
            if label:
                row.turn_type = "system"
                text = "%s: %s" % (label, text)
            else:
                row.turn_type = "user"
            row.text = compact(text)
            return row
        if attachment_type == "hook_system_message":
            row = base()
            row.turn_type = "system"
            name = str(attachment.get("hookName") or attachment.get("hookEvent") or "")
            body = text_of(attachment.get("content"))
            row.text = compact("hook: %s" % ": ".join(part for part in (name, body) if part))
            return row
        if attachment_type == "edited_text_file":
            row = base()
            row.turn_type = "system"
            row.text = compact("edited file: %s" % (attachment.get("filename") or ""))
            return row
        return None


# A history line and its transcript record differ in form for slash
# commands and pastes, and a command's record can come long after its
# history line; `dedup.deduplicate` matches them under `match_form` and
# `match_tolerance`.
MATCH_FORMS[ClaudeCodeParser.agent] = match_form
MATCH_TOLERANCES[ClaudeCodeParser.agent] = match_tolerance
