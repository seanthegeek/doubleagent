"""Define the parser interface and the helpers every parser shares.

A parser owns one agent name from the catalog, says which of that agent's
artifacts it reads, and yields timeline rows from each.
"""

from __future__ import annotations

import json
from collections.abc import Iterable, Iterator
from dataclasses import dataclass

from ..dedup import TOLERANCE_SECONDS
from ..errors import unreadable_message
from ..inputs import Artifact
from ..model import Row
from ..timeutil import to_utc


@dataclass
class Options:
    """The timeline options a parser honours."""

    include_thinking: bool = False


class Parser:
    """The base class of every parser; a subclass sets `agent` and overrides `wants` and `parse`."""

    agent: str = ""
    name: str = ""
    # Other catalog agents whose artifacts this parser is also offered: an
    # extension whose chats are rows of the editor's own state.vscdb, which
    # the catalog attributes to `vscode` or to a fork such as `cursor`, lists
    # those editors here (`vscode_state.EDITOR_AGENTS`).
    reads_agents: tuple[str, ...] = ()
    # True when the agent's format records whether a session is synced to a
    # remote account, and the parser yields a `system` row starting
    # `session synced:` for a synced one. `sessions.jsonl` then says
    # `synced` "no" for a session without that row rather than "".
    records_sync: bool = False
    # True when the agent's format records whether a session was carried
    # through a messaging platform (Telegram, WhatsApp, email), and the
    # parser yields a `system` row starting `session relayed:` for such a
    # session. `sessions.jsonl` then says `relayed` "no" for a session
    # without that row rather than "".
    records_relay: bool = False
    # True when the agent's format records whether a session began on the
    # vendor's servers and is only listed locally, and the parser yields a
    # `system` row starting `session origin: remote` for such a session.
    # `sessions.jsonl` then says `origin` "local" for a session without that
    # row rather than "".
    records_origin: bool = False
    # True when `secondary()` can return True for some artifact, so the agent
    # reads at least one secondary source; `sessions.jsonl` then says
    # `recovered` "no" for a session with no recovered row rather than "".
    # `secondary()` needs an artifact to answer, so this says it per class.
    has_secondary: bool = False

    def wants(self, artifact: Artifact) -> bool:
        """Return whether this parser reads the artifact, chosen by `artifact.rel`."""
        raise NotImplementedError

    def parse(self, artifact: Artifact, opts: Options) -> Iterator[Row]:
        """Yield the timeline rows of one artifact."""
        raise NotImplementedError

    def secondary(self, artifact: Artifact) -> bool:
        """Return whether the artifact is a secondary source of this parser.

        A secondary source repeats events of a store the parser also reads (a
        per-prompt history, a mirror). Its rows are kept unless the primary
        store yields the same event (`dedup.deduplicate`).
        """
        return False

    def secondary_tolerance(self, artifact: Artifact) -> float:
        """Return how many seconds a secondary row's time may differ from its primary row's.

        The default suits a history line written when the prompt was
        submitted; a parser whose secondary source stamps an event later than
        the primary store returns a wider window and says why.
        """
        return TOLERANCE_SECONDS

    def base_row(self, artifact: Artifact) -> Row:
        """Return a row with host, user, agent and `source_file` filled in."""
        return Row(
            host=artifact.home.host,
            user=artifact.home.user,
            agent=self.agent,
            source_file=artifact.original,
        )


def iter_lines(path, errors: list) -> Iterator[tuple[int, bytes]]:
    r"""Yield (line_number, raw) for every line of a file as read, terminator included.

    Lines end at LF, so a CRLF line keeps its `\r\n` and a last line without
    one is yielded as it stands. A file that cannot be opened is one entry
    (0, `unreadable_message`) in `errors`; one that fails while being read is
    one entry at the last line read, after the lines before it. Nothing
    raises.
    """
    line_number = 0
    try:
        with open(path, "rb") as handle:
            for raw in handle:
                line_number += 1
                yield line_number, raw
    except OSError as error:
        errors.append((line_number, unreadable_message(path, error)))


def file_bytes(path, errors: list) -> bytes | None:
    """Return a file's whole content, or None after appending its unreadable entry.

    The entry is (0, `unreadable_message`) for the OSError that stopped the
    open or the read. Nothing raises.
    """
    try:
        with open(path, "rb") as handle:
            return handle.read()
    except OSError as error:
        errors.append((0, unreadable_message(path, error)))
        return None


def iter_jsonl(path, errors: list) -> Iterator[tuple[int, dict]]:
    """Yield (line_number, record) for each JSON object line of a file.

    A bad line, one that does not decode or decodes to something other than
    an object, is appended to `errors` as (line_number, message, raw_text)
    and skipped, so a transcript cut mid-write keeps its earlier turns.
    `raw_text` is the line as read less its terminator (`line_text`). The
    lines come from `iter_lines`, which reports a file that cannot be opened
    or read; nothing raises.
    """
    yield from iter_jsonl_lines(iter_lines(path, errors), errors)


def iter_jsonl_lines(
    lines: Iterable[tuple[int, bytes]], errors: list
) -> Iterator[tuple[int, dict]]:
    """Yield (line_number, record) for each JSON object line of (line_number, raw) pairs.

    The lines are as `iter_lines` yields them, from a file or from bytes held
    in memory; bad lines go to `errors` as `iter_jsonl` says.
    """
    for line_number, raw in lines:
        stripped = raw.strip()
        if not stripped:
            continue
        try:
            record = json.loads(stripped.decode("utf-8", errors="replace"))
        except ValueError as error:
            errors.append((line_number, "unparseable line (%s)" % error, line_text(raw)))
            continue
        if not isinstance(record, dict):
            message = "unparseable line (not a JSON object)"
            errors.append((line_number, message, line_text(raw)))
            continue
        yield line_number, record


def line_text(raw: bytes) -> str:
    r"""Return a line read in binary mode as error row text.

    The line is decoded as UTF-8 with replacement and loses its trailing
    `\n` and then `\r`, nothing else.
    """
    if raw.endswith(b"\n"):
        raw = raw[:-1]
    if raw.endswith(b"\r"):
        raw = raw[:-1]
    return raw.decode("utf-8", errors="replace")


def tail_text(text: str, start: int) -> str:
    """Return text from `start` to its end as error row text, as `line_text` treats a line.

    A JSON array reader passes the offset of the record it failed to decode:
    a broken record has no boundary of its own, so everything after it goes
    too.
    """
    tail = text[start:]
    if tail.endswith("\n"):
        tail = tail[:-1]
    if tail.endswith("\r"):
        tail = tail[:-1]
    return tail


def error_rows(
    parser: Parser, artifact: Artifact, errors: list, session_id: str = "", project_path: str = ""
) -> Iterator[Row]:
    """Yield one `system` row per entry of `errors`, in list order.

    An entry is (line, message, raw) or (line, message). The row text is
    `parser: <message>: <raw>`, the raw line or record exactly as the reader
    stored it, or `parser: <message>` when raw is empty (a whole-file error). The
    row carries the given `session_id` and `project_path` and `source_line`
    set to the line number, 0 when there is none.
    """
    for entry in errors:
        line, message = entry[0], entry[1]
        raw = entry[2] if len(entry) > 2 else ""
        row = parser.base_row(artifact)
        row.session_id = session_id
        row.project_path = project_path
        row.turn_type = "system"
        row.source_line = line if isinstance(line, int) else 0
        if raw:
            row.text = "parser: %s: %s" % (message, raw)
        else:
            row.text = "parser: %s" % message
        yield row


def text_of(content) -> str:
    """Return the text of a chat message's content: a string, or a list of typed blocks."""
    if content is None:
        return ""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for block in content:
            if isinstance(block, str):
                parts.append(block)
            elif isinstance(block, dict):
                text = block.get("text")
                if isinstance(text, str):
                    parts.append(text)
                elif block.get("type") == "image":
                    parts.append("[image]")
        return "\n".join(parts)
    if isinstance(content, dict):
        text = content.get("text")
        return text if isinstance(text, str) else json.dumps(content, ensure_ascii=False)
    return str(content)


def compact_json(value) -> str:
    """Return a value as compact JSON with sorted keys, or as `str()` when it is not JSON."""
    try:
        text = json.dumps(value, ensure_ascii=False, separators=(",", ":"), sort_keys=True)
    except (TypeError, ValueError):
        text = str(value)
    return text


def file_mtime(artifact: Artifact) -> str:
    """Return the file's modification time as a timeline timestamp.

    The manifest's pre-copy `mtime` is used when the collection has one, else
    the file's own. Rows for standing settings carry this time.
    """
    mtime = (artifact.manifest or {}).get("mtime")
    if isinstance(mtime, (int, float)) and not isinstance(mtime, bool) and mtime > 0:
        return to_utc(mtime)
    try:
        return to_utc(artifact.disk_path.lstat().st_mtime)
    except OSError:
        return ""


def setting_value(value) -> str:
    """Return a setting as row text: `true` or `false`, a string as it is, else compact JSON."""
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, str):
        return value
    return compact_json(value)


def track_change(label: str, seen: dict, key, value: str) -> str:
    """Return the `system` row text for a change of a tracked state, or `""`.

    The text is `<label>: <value>` the first time `key` (a session or thread
    id) has a value and `<label>: <old> -> <new>` on a change; an unchanged or
    empty value gives `""`. `seen` is the parser's per-file dict.
    """
    if not value:
        return ""
    old = seen.get(key)
    if old == value:
        return ""
    seen[key] = value
    return "%s: %s -> %s" % (label, old, value) if old else "%s: %s" % (label, value)
