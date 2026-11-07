"""The base class and helpers the parser test modules share."""

import copy
import hashlib
import json
import os
import shutil
import stat
import tempfile
import unittest
from collections.abc import Callable
from pathlib import Path
from types import SimpleNamespace
from typing import ClassVar, cast

from doubleagent import catalog
from doubleagent.cli import parsers_for
from doubleagent.inputs import Artifact, Collection, open_input
from doubleagent.parsers import Options, by_agent
from fixtures import build_home


def read_jsonl(path: Path) -> list[dict]:
    with open(path, encoding="utf-8", newline="") as handle:
        return [json.loads(line) for line in handle]


def _snapshot(root: Path) -> list[tuple]:
    """Return every entry under `root` with its mode, size, times, inode and content digest.

    Links are recorded, never followed, and a regular file's bytes are digested, so a test that
    changed anything under `root` (a write, an append, a chmod, a new file or a removed one)
    gives a different snapshot.
    """
    entries = []
    for directory, dirnames, filenames in os.walk(root):
        for name in [*dirnames, *filenames]:
            path = os.path.join(directory, name)
            status = os.lstat(path)
            digest = ""
            if stat.S_ISREG(status.st_mode):
                try:
                    with open(path, "rb") as handle:
                        digest = hashlib.sha256(handle.read()).hexdigest()
                except OSError:
                    digest = "unreadable"
            entries.append(
                (
                    os.path.relpath(path, root),
                    status.st_mode,
                    status.st_size,
                    status.st_mtime_ns,
                    status.st_ctime_ns,
                    status.st_ino,
                    digest,
                )
            )
    return sorted(entries)


class ParserBase(unittest.TestCase):
    """Build the fixture home and open it as an input, once per class.

    The home is rebuilt before a test when an earlier test changed the tree, the opened input
    or the catalog. A subclass lists in `extra_builders` the functions that add files to the home before it is
    opened, so those files are part of the unchanged tree.
    """

    extra_builders: ClassVar[tuple[Callable[[Path], None], ...]] = ()
    cat: catalog.Catalog
    tmp: Path
    home: Path
    col: Collection
    _pristine: ClassVar[list[tuple]]
    _pristine_col: ClassVar[Collection]
    _pristine_cat: ClassVar[catalog.Catalog]

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls.cat = catalog.load()
        cls._build()

    @classmethod
    def _build(cls):
        cls.tmp = Path(tempfile.mkdtemp(prefix="cac-analyzer-test-"))
        cls.home = build_home(cls.tmp / "home" / "alice")
        for builder in cls.extra_builders:
            builder(cls.home)
        cls.col = open_input(cls.tmp / "home", cls.cat, host="h1")
        cls._pristine = _snapshot(cls.tmp)
        cls._pristine_col = copy.deepcopy(cls.col)
        cls._pristine_cat = copy.deepcopy(cls.cat)

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.tmp, ignore_errors=True)
        super().tearDownClass()

    def setUp(self):
        cls = type(self)
        if (
            cls.col != cls._pristine_col
            or cls.cat != cls._pristine_cat
            or _snapshot(cls.tmp) != cls._pristine
        ):
            shutil.rmtree(cls.tmp, ignore_errors=True)
            cls._build()

    def artifact(self, rel):
        return next(artifact for artifact in self.col.artifacts if artifact.rel == rel)

    def rows_for(self, parser, rel, **options):
        return list(parser.parse(self.artifact(rel), Options(**options)))


def rel_only(rel: str) -> Artifact:
    """A stand-in artifact carrying only `rel`, the one attribute wants() reads."""
    return cast(Artifact, SimpleNamespace(rel=rel))


def model_or_permission_row(row) -> bool:
    """Return whether `row` is a `model:` or `permission ...:` system row."""
    return row.turn_type == "system" and row.text.startswith(("model: ", "permission "))


def _rollout_bytes(records) -> bytes:
    return "".join(json.dumps(record) + "\n" for record in records).encode("utf-8")


def parse_with(test, parser_class, locked: str = "") -> list:
    """Parse every file a new `parser_class` is offered, with `locked` at mode 000 if given.

    Thinking rows are included. `test` is a `ParserBase` whose fixture home holds the files.
    """
    parser = parser_class()
    groups = by_agent()
    offered = [
        artifact
        for artifact in test.col.artifacts
        if any(type(candidate) is parser_class for candidate in parsers_for(artifact, groups))
        and parser.wants(artifact)
    ]
    path = test.home / locked if locked else None
    mode = stat.S_IMODE(path.stat().st_mode) if path else 0
    if path:
        path.chmod(0)
    try:
        options = Options(include_thinking=True)
        return [row for artifact in offered for row in parser.parse(artifact, options)]
    finally:
        if path:
            path.chmod(mode)


def unreadable_rows(rows: list) -> list:
    """Return the rows saying a file could not be read."""
    return [row for row in rows if "could not be read" in row.text]


def kept_rows(rows: list, without: str = "") -> list:
    """Return each row's identifying fields, less the unreadable rows and those of `without`."""
    return [
        (row.source_file, row.source_line, row.turn_type, row.text, row.project_path)
        for row in rows
        if "could not be read" not in row.text
        and not (without and row.source_file.endswith(without))
    ]
