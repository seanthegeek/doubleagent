"""The base class and helpers the parser test modules share."""

import json
import shutil
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from typing import cast

from doubleagent import catalog
from doubleagent.inputs import Artifact, open_input
from doubleagent.parsers import Options
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
