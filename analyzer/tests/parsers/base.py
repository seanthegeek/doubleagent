"""The base class and helpers the parser test modules share."""

import json
import shutil
import stat
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from typing import cast

from doubleagent import catalog
from doubleagent.cli import parsers_for
from doubleagent.inputs import Artifact, open_input
from doubleagent.parsers import Options, by_agent
from fixtures import build_home


def read_jsonl(path: Path) -> list[dict]:
    with open(path, encoding="utf-8", newline="") as handle:
        return [json.loads(line) for line in handle]


class ParserBase(unittest.TestCase):
    """Build the fixture home and open it as an input for each test."""

    def setUp(self):
        self.cat = catalog.load()
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-analyzer-test-"))
        self.home = build_home(self.tmp / "home" / "alice")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

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
