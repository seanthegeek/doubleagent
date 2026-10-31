"""Every file a parser reads that exists but cannot be read is a `parser:` row.

For each parser in `ALL`, the files it opens while parsing the artifacts it `wants()` in the
fixture home (the wanted files and the side files beside them, found through the interpreter's
`open` audit event) are made unreadable one at a time. The parser must still finish, and some
`parser:` row must name the file, or its database for a SQLite sidecar, and carry the system's
reason. `unreadable_baseline.txt` lists the open gaps, one `<agent> <kind> <file>` line each; a
gap not listed fails, and so does a listed line the parser no longer gives, so the file empties
as parsers are fixed. The kinds are `escaped` (the OSError left the parser, so the timeline gets
a `problem:` line and loses the rest of that artifact), `no-reason` (a row names the file but
without the reason, such as the not-a-JSON-object row) and `silent` (no row names the file).
"""

import os
import shutil
import stat
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace

from test_errors import CAN_DENY, DENY_REASON

from doubleagent import catalog
from doubleagent.cli import parsers_for
from doubleagent.inputs import open_input
from doubleagent.parsers import ALL, Options, by_agent
from fixtures import build_home

BASELINE = Path(__file__).with_name("unreadable_baseline.txt")
SQLITE_SIDECARS = ("-wal", "-shm", "-journal")
REASON = "permission denied"

# The paths opened while `_OPENED` is a set; an audit hook cannot be removed, so it stays
# installed for the session and does nothing while `_OPENED` is None.
_OPENED: set | None = None
_HOOKED = False


def _audit(event, arguments):
    if _OPENED is not None and event == "open" and isinstance(arguments[0], (str, bytes, Path)):
        _OPENED.add(os.fsdecode(arguments[0]))


def _parse_all(parser, artifacts) -> tuple[list, BaseException | None]:
    rows: list = []
    try:
        for artifact in artifacts:
            rows.extend(parser.parse(artifact, Options(include_thinking=True)))
    except OSError as error:
        return rows, error
    return rows, None


def _files_read(parser, artifacts, home: Path) -> list[str]:
    """Return the regular files under `home` the parser opens, relative to it, sorted."""
    global _OPENED
    _OPENED = set()
    try:
        _parse_all(parser, artifacts)
        opened = _OPENED
    finally:
        _OPENED = None
    files = set()
    for path in opened:
        real = Path(os.path.realpath(path))
        if real.is_relative_to(home) and real.is_file():
            files.add(real.relative_to(home).as_posix())
    return sorted(files)


def _gap(rel: str, rows: list, error: BaseException | None) -> str:
    """Return the gap kind for `rel` made unreadable, or "" when a row reports it."""
    if error is not None:
        return "escaped"
    names = {rel, rel.rsplit("/", 1)[-1]}
    for suffix in SQLITE_SIDECARS:
        if rel.endswith(suffix):
            names.add(rel[: -len(suffix)].rsplit("/", 1)[-1])
    naming = [
        row
        for row in rows
        if row.text.startswith("parser:")
        and (any(name in row.text for name in names) or row.source_file.endswith("/" + rel))
    ]
    if any(REASON in row.text.lower() for row in naming):
        return ""
    return "no-reason" if naming else "silent"


def _gaps(scratch: Path) -> set[str]:
    home = build_home(scratch / "home" / "alice").resolve()
    collection = open_input(scratch / "home", catalog.load(), host="h1")
    groups = by_agent()
    gaps = set()
    for parser in ALL:
        artifacts = [
            artifact
            for artifact in collection.artifacts
            if parser in parsers_for(artifact, groups) and parser.wants(artifact)
        ]
        for rel in _files_read(parser, artifacts, home):
            path = home / rel
            mode = stat.S_IMODE(path.stat().st_mode)
            path.chmod(0)
            try:
                rows, error = _parse_all(parser, artifacts)
            finally:
                path.chmod(mode)
            kind = _gap(rel, rows, error)
            if kind:
                gaps.add("%s %s %s" % (parser.agent, kind, rel))
    return gaps


def _baseline() -> set[str]:
    lines = BASELINE.read_text(encoding="utf-8").split("\n")
    return {line for line in lines if line and not line.startswith("#")}


def test_every_unreadable_file_is_a_parser_row():
    if not CAN_DENY:
        raise unittest.SkipTest(DENY_REASON)
    global _HOOKED
    if not _HOOKED:
        sys.addaudithook(_audit)
        _HOOKED = True
    scratch = Path(tempfile.mkdtemp(prefix="cac-unreadable-test-"))
    try:
        gaps = _gaps(scratch)
    finally:
        shutil.rmtree(scratch, ignore_errors=True)
    baseline = _baseline()
    new = sorted(gaps - baseline)
    stale = sorted(baseline - gaps)
    assert not new, "unreadable file not reported, fix the parser:\n" + "\n".join(new)
    assert not stale, "fixed, remove from %s:\n%s" % (BASELINE.name, "\n".join(stale))


def test_gap_kinds():
    rows_named = [_row("parser: a.json is not a JSON object")]
    cases = (
        ("d/a.json", [], PermissionError(13, "Permission denied"), "escaped"),
        ("d/a.json", rows_named, None, "no-reason"),
        ("d/b.json", rows_named, None, "silent"),
        ("d/x.db-wal", [_row("parser: x.db could not be read: Permission denied")], None, ""),
        ("d/a.json", [_row("parser: unreadable a.json: Permission denied")], None, ""),
        ("d/a.json", [_row("parser: unreadable: Permission denied", "/h/d/a.json")], None, ""),
    )
    for rel, rows, error, kind in cases:
        assert _gap(rel, rows, error) == kind, (rel, [row.text for row in rows], error, kind)


def _row(text: str, source_file: str = "") -> SimpleNamespace:
    return SimpleNamespace(text=text, source_file=source_file)
