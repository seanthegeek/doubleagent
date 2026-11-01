"""little-coder's own files.

little-coder is a launcher and extension set for pi, so its transcripts are pi session files
parsed by `pi.py`; this parser reads what little-coder writes itself:

* `.pi/agent/little-coder-prompt-history.json`: a JSON array of prompt strings, oldest first,
  with no time or session; `source_line` is the array index.
* `.little-coder/checkpoints/<session file name>/<sanitised path>`: the pre-edit copy of each
  file a session first edited, or an empty `<name>.absent` sentinel. One `system` row per
  directory, emitted on its first file, at the earliest file mtime; the session id comes from
  the directory name and the project from the pi session header when it was collected.

The retired v0.0.x Python releases kept their own state in `.little-coder/`:

* Saved sessions `sessions/**/*.json` (`session_id`, `saved_at` in host local time, emitted as
  if UTC, `messages` with `role`, `content`, `images`, `tool_calls[{id, name, input}]`,
  `tool_call_id`, `name`; `turn_count`, `total_input_tokens`, `total_output_tokens`);
  `sessions/history.json` (`sessions[]`) and `sessions/mr_sessions/session_latest.json` repeat
  the daily copy and are secondary. A file cut mid-write keeps the messages before the cut.
* `input_history.txt`, the readline history (secondary, `user` rows matched on text alone).
* `config.json`: only `cloudsave_last_gist_id`, `telegram_chat_id` and `permission_mode` are
  read, never a token or a key.
* `checkpoints/<session id>/snapshots.json`: `id`, `session_id`, `created_at`, `turn_count`,
  `user_prompt_preview`, `file_backups{<path>: {backup_filename}}`.

Validated against source, itayinbarr/little-coder at 89d4fa0a (pinning pi 0.83.x) and the
v0.0.5 tag at 424f7a4b, with a synthetic fixture; no real install. See
`research/little-coder.md`.
"""

from __future__ import annotations

import json
import os
import re
import stat
from collections.abc import Iterator
from pathlib import Path

from ..errors import unreadable_message
from ..inputs import Artifact
from ..model import Row, compact
from ..timeutil import to_utc
from .base import (
    Options,
    Parser,
    error_rows,
    file_bytes,
    file_mtime,
    iter_lines,
    line_text,
    setting_value,
    text_of,
)
from .cline_legacy import iter_array_at, iter_json_array, read_text
from .pi import args_summary, read_session

HISTORY_REL = ".pi/agent/little-coder-prompt-history.json"
CHECKPOINT_PATTERN = re.compile(r"^\.little-coder/checkpoints/([^/]+)/[^/]+$")
SESSION_FILE_PATTERN = re.compile(r"_([^_/]+)\.jsonl$")

# The v0.0.x Python layout under ~/.little-coder.
LEGACY_SESSION_PATTERN = re.compile(r"^\.little-coder/sessions/(?:[^/]+/)*[^/]+\.json$")
LEGACY_HISTORY_REL = ".little-coder/sessions/history.json"
LEGACY_LATEST_REL = ".little-coder/sessions/mr_sessions/session_latest.json"
LEGACY_INPUT_REL = ".little-coder/input_history.txt"
LEGACY_CONFIG_REL = ".little-coder/config.json"
LEGACY_SNAPSHOTS_PATTERN = re.compile(r"^\.little-coder/checkpoints/([^/]+)/snapshots\.json$")
LEGACY_DENIED = "Denied: user rejected this operation"
SESSIONS_KEY_PATTERN = re.compile(r'"sessions"[ \t\n\r]*:[ \t\n\r]*')
MESSAGES_KEY_PATTERN = re.compile(r'"messages"[ \t\n\r]*:[ \t\n\r]*')
SESSION_ID_PATTERN = re.compile(r'"session_id"[ \t\n\r]*:[ \t\n\r]*"([^"\\]*)"')
SAVED_AT_PATTERN = re.compile(r'"saved_at"[ \t\n\r]*:[ \t\n\r]*"([^"\\]*)"')


def _regular_files(directory: Path) -> list[os.DirEntry]:
    """Return the regular files of a directory sorted by name, `[]` when it cannot be listed."""
    try:
        return sorted(
            (entry for entry in os.scandir(directory) if entry.is_file(follow_symlinks=False)),
            key=lambda entry: entry.name,
        )
    except OSError:
        return []


def _session_header(home: Path, name: str) -> tuple[dict | None, list]:
    """Return the header of the pi session file `name` in the same home, or None, and its failure.

    The failure is the whole-file `errors` entries: one when the sessions
    directory or a project directory under it cannot be read, or the file
    exists but cannot be read; its bad lines are left to the pi parser.
    """
    sessions = home / ".pi/agent/sessions"
    if sessions.is_symlink() or not sessions.is_dir():
        return None, []
    try:
        directories = sorted(sessions.iterdir())
    except OSError as error:
        return None, [(0, unreadable_message(sessions, error))]
    for directory in directories:
        if directory.is_symlink() or not directory.is_dir():
            continue
        session_path = directory / name
        # lstat rather than is_file: a project directory that cannot be searched
        # must become a row, and pathlib's is_file swallows that error on some
        # Python versions and raises it on others.
        try:
            found = os.lstat(session_path)
        except FileNotFoundError:
            continue
        except OSError as error:
            return None, [(0, unreadable_message(directory, error))]
        if not stat.S_ISREG(found.st_mode):
            continue
        session = read_session(session_path)
        # A two-tuple is a whole-file entry (the file could not be read); a bad
        # line is a three-tuple the pi parser reports itself.
        failure = [entry for entry in session.errors if len(entry) == 2]
        return session.header or None, failure
    return None, []


def _dash(value) -> str:
    """Return a value as row text, `-` when absent or empty."""
    return "-" if value in (None, "") else setting_value(value)


class LittleCoderParser(Parser):
    """Parse little-coder's prompt history, checkpoints and the v0.0.x Python layout."""

    agent = "little-coder"
    name = "little-coder"
    has_secondary = True
    records_sync = True
    records_relay = True

    def wants(self, artifact: Artifact) -> bool:
        """Return True for the little-coder files this parser reads."""
        rel = artifact.rel
        return (
            rel in (HISTORY_REL, LEGACY_INPUT_REL, LEGACY_CONFIG_REL)
            or bool(CHECKPOINT_PATTERN.match(rel))
            or bool(LEGACY_SESSION_PATTERN.match(rel))
        )

    def secondary(self, artifact: Artifact) -> bool:
        """Return True for the v0.0.x files that repeat a saved session or a prompt."""
        return artifact.rel in (LEGACY_HISTORY_REL, LEGACY_LATEST_REL, LEGACY_INPUT_REL)

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the rows of one little-coder file."""
        rel = artifact.rel
        if rel == HISTORY_REL:
            yield from self._parse_history(artifact, opts)
        elif rel == LEGACY_INPUT_REL:
            yield from self._parse_input_history(artifact)
        elif rel == LEGACY_CONFIG_REL:
            yield from self._parse_legacy_config(artifact)
        elif rel == LEGACY_HISTORY_REL:
            yield from self._parse_legacy_history(artifact)
        elif LEGACY_SESSION_PATTERN.match(rel):
            yield from self._parse_legacy_session(artifact)
        elif LEGACY_SNAPSHOTS_PATTERN.match(rel) and not SESSION_FILE_PATTERN.search(
            rel.split("/")[2]
        ):
            yield from self._parse_legacy_snapshots(artifact)
        else:
            yield from self._parse_checkpoints(artifact, opts)

    def _parse_history(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield one `user` row per prompt of the prompt history."""
        errors: list = []
        for position, prompt in iter_json_array(artifact.disk_path, errors):
            if not isinstance(prompt, str) or not prompt:
                continue
            row = self.base_row(artifact)
            row.source_line = position
            row.turn_type = "user"
            row.text = compact(prompt)
            yield row
        yield from error_rows(self, artifact, errors)

    def _parse_checkpoints(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the `system` row of a checkpoint directory, on the directory's first file."""
        match = CHECKPOINT_PATTERN.match(artifact.rel)
        assert match is not None  # wants() accepted only matching paths
        set_name = match.group(1)
        files = _regular_files(artifact.disk_path.parent)
        if not files or files[0].name != artifact.disk_path.name:
            return  # the set's row comes from its first file
        mtimes: list[float] = []
        names: list[tuple[str, bool]] = []
        for entry in files:
            try:
                mtimes.append(entry.stat(follow_symlinks=False).st_mtime)
            except OSError:
                pass
            absent = entry.name.endswith(".absent")
            names.append((entry.name[: -len(".absent")] if absent else entry.name, absent))
        match = SESSION_FILE_PATTERN.search(set_name)
        row = self.base_row(artifact)
        row.source_line = 0
        row.turn_type = "system"
        row.session_id = match.group(1) if match else ""
        header, failure = (
            _session_header(artifact.home.disk_path, set_name) if match else (None, [])
        )
        if header:
            row.session_id = str(header.get("id") or row.session_id)
            row.project_path = str(header.get("cwd") or "")
        row.timestamp_utc = to_utc(min(mtimes)) if mtimes else ""
        listed = "; ".join(
            file_name + (" (did not exist)" if was_absent else "")
            for file_name, was_absent in names
        )
        what = "pi session %s" % set_name if match else "no session file (%s)" % set_name
        row.text = compact(
            "little-coder checkpoints for %s: %d pre-edit file(s)%s: %s"
            % (
                what,
                len(names),
                "" if header or failure else ", session file not collected",
                listed,
            ),
        )
        yield row
        yield from error_rows(self, artifact, failure, row.session_id, row.project_path)

    # ---- the v0.0.x Python layout -------------------------------------------

    def _parse_input_history(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one `user` row per line of `input_history.txt`, with no time."""
        errors: list = []
        for line_number, raw in iter_lines(artifact.disk_path, errors):
            text = line_text(raw)
            if not text.strip():
                continue
            row = self.base_row(artifact)
            row.source_line, row.turn_type, row.text = line_number, "user", compact(text)
            yield row
        yield from error_rows(self, artifact, errors)

    def _parse_legacy_config(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the gist, Telegram chat and permission mode rows of `config.json`."""
        errors: list = []
        data = file_bytes(artifact.disk_path, errors)
        if data is None:
            yield from error_rows(self, artifact, errors)
            return
        try:
            document = json.loads(data.decode("utf-8", errors="replace"))
        except ValueError:
            document = None
        timestamp = file_mtime(artifact)
        if not isinstance(document, dict):
            row = self.base_row(artifact)
            row.source_line, row.turn_type, row.timestamp_utc = 1, "system", timestamp
            row.text = "parser: %s is not a JSON object" % artifact.rel
            yield row
            return
        source = "(from %s, file mtime)" % artifact.rel
        texts = []
        if document.get("cloudsave_last_gist_id") not in (None, ""):
            texts.append(
                "session synced: gist %s %s" % (_dash(document["cloudsave_last_gist_id"]), source)
            )
        if document.get("telegram_chat_id") not in (None, "", 0):
            texts.append(
                "session relayed: telegram chat %s %s"
                % (_dash(document["telegram_chat_id"]), source)
            )
        if document.get("permission_mode") not in (None, ""):
            texts.append(
                "permission setting: permission_mode=%s %s"
                % (_dash(document["permission_mode"]), source)
            )
        for text in texts:
            row = self.base_row(artifact)
            row.source_line, row.turn_type, row.timestamp_utc = 1, "system", timestamp
            row.text = compact(text)
            yield row

    def _parse_legacy_history(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the rows of `sessions/history.json`, every session saved at exit."""
        errors: list = []
        text = read_text(artifact.disk_path, errors)
        document = None
        if text is not None:
            try:
                document = json.loads(text)
            except ValueError:
                document = None
        if isinstance(document, dict) and isinstance(document.get("sessions"), list):
            sessions = enumerate(document["sessions"], 1)
        elif text is None:
            sessions = iter(())
        else:
            # Cut mid-write: walk the sessions array as far as it parses.
            match = SESSIONS_KEY_PATTERN.search(text)
            if match:
                sessions = iter_array_at(text, match.end(), errors)
            else:
                errors.append((0, "history.json has no sessions array", ""))
                sessions = iter(())
        for position, session in sessions:
            if not isinstance(session, dict):
                continue
            yield from self._legacy_rows(
                artifact,
                session,
                enumerate(session.get("messages") or [], 1),
                str(session.get("session_id") or ""),
                str(session.get("saved_at") or ""),
                position,
            )
        yield from error_rows(self, artifact, errors)

    def _parse_legacy_session(self, artifact: Artifact) -> Iterator[Row]:
        """Yield the rows of one saved session file (daily, `/save`, or `session_latest.json`)."""
        errors: list = []
        text = read_text(artifact.disk_path, errors)
        document = None
        if text is not None:
            try:
                document = json.loads(text)
            except ValueError:
                document = None
        if isinstance(document, dict):
            yield from self._legacy_rows(
                artifact,
                document,
                enumerate(document.get("messages") or [], 1),
                str(document.get("session_id") or ""),
                str(document.get("saved_at") or ""),
            )
        elif text is not None:
            # Cut mid-write: the keys before `messages` and the messages
            # array as far as it parses.
            match = MESSAGES_KEY_PATTERN.search(text)
            if match:
                head = text[: match.start()]
                session_id = SESSION_ID_PATTERN.search(head)
                saved = SAVED_AT_PATTERN.search(head)
                yield from self._legacy_rows(
                    artifact,
                    {},
                    iter_array_at(text, match.end(), errors),
                    session_id.group(1) if session_id else "",
                    saved.group(1) if saved else "",
                )
            else:
                errors.append((0, "session file is not a JSON object", ""))
        yield from error_rows(self, artifact, errors)

    def _legacy_rows(
        self,
        artifact: Artifact,
        session: dict,
        messages,
        session_id: str,
        saved_at: str,
        session_position: int = 0,
    ) -> Iterator[Row]:
        """Yield a `session saved:` row, then each message's rows at the session's `saved_at`.

        `session_position` is the session's position in `history.json`, 0 for a session file.
        """
        timestamp = to_utc(saved_at)

        def make_row(
            source_line: int, turn: str, text: str, tool_name: str = "", tool_use_id: str = ""
        ) -> Row:
            row = self.base_row(artifact)
            row.source_line = session_position or source_line
            row.timestamp_utc = timestamp
            row.session_id = session_id
            row.turn_type = turn
            row.tool_name = tool_name
            row.tool_use_id = tool_use_id
            row.text = compact(text)
            return row

        yield make_row(
            0,
            "system",
            "session saved: little-coder v0.0.x session %s turn_count=%s input_tokens=%s "
            "output_tokens=%s (saved_at %s, host local time)"
            % (
                _dash(session_id),
                _dash(session.get("turn_count")),
                _dash(session.get("total_input_tokens")),
                _dash(session.get("total_output_tokens")),
                _dash(saved_at),
            ),
        )
        for position, message in messages:
            if not isinstance(message, dict):
                continue
            role = message.get("role")
            text = text_of(message.get("content"))
            if role == "user":
                if message.get("images"):
                    text = (text + "\n" if text else "") + "[%d image(s)]" % len(message["images"])
                if text:
                    yield make_row(position, "user", text)
            elif role == "assistant":
                if text:
                    yield make_row(position, "assistant", text)
                calls = message.get("tool_calls")
                for tool_call in calls if isinstance(calls, list) else []:
                    if isinstance(tool_call, dict):
                        yield make_row(
                            position,
                            "tool_use",
                            args_summary(tool_call.get("input")),
                            str(tool_call.get("name") or ""),
                            str(tool_call.get("id") or ""),
                        )
            elif role == "tool":
                name = str(message.get("name") or "")
                call_id = str(message.get("tool_call_id") or "")
                yield make_row(position, "tool_result", text, name, call_id)
                if text == LEGACY_DENIED:
                    yield make_row(
                        position, "system", "permission decision: denied (%s)" % text, name, call_id
                    )

    def _parse_legacy_snapshots(self, artifact: Artifact) -> Iterator[Row]:
        """Yield one row per snapshot of `checkpoints/<id>/snapshots.json`."""
        session_id = artifact.rel.split("/")[2]
        errors: list = []
        for position, snapshot in iter_json_array(artifact.disk_path, errors):
            if not isinstance(snapshot, dict):
                continue
            backups = snapshot.get("file_backups")
            files = []
            for path, backup in (backups if isinstance(backups, dict) else {}).items():
                name = backup.get("backup_filename") if isinstance(backup, dict) else None
                files.append("%s (%s)" % (path, "backup %s" % name if name else "did not exist"))
            row = self.base_row(artifact)
            row.source_line, row.turn_type = position, "system"
            row.timestamp_utc = to_utc(snapshot.get("created_at"))
            row.session_id = str(snapshot.get("session_id") or session_id)
            row.text = compact(
                "little-coder v0.0.x checkpoint %s: turn_count=%s prompt=%s files=%s "
                "(created_at host local time)"
                % (
                    _dash(snapshot.get("id")),
                    _dash(snapshot.get("turn_count")),
                    _dash(snapshot.get("user_prompt_preview")),
                    "; ".join(files) or "-",
                )
            )
            yield row
        yield from error_rows(self, artifact, errors)
