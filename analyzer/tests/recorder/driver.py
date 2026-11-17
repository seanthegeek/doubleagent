"""Record the cases no single test makes, after the suite: the full fixture home and the examples.

The full home (`fixtures.build_home`) is opened as a loose input and every artifact goes through
every parser that wants it, once without and once with thinking rows. The example collection of
`analyzer/tools/example_collection.py` goes through `detect` (plain, `--json`, `--files`),
`timeline` (unfiltered, and with the filters of the documented example) and `inventory`. The
cases carry the node ids `recorder::full-home` and `recorder::example-collection`.
"""

from __future__ import annotations

import contextlib
import importlib.util
import io
import os
import tempfile
from pathlib import Path
from types import ModuleType
from typing import TYPE_CHECKING

from doubleagent import catalog, cli
from doubleagent.inputs import open_input
from doubleagent.parsers import Options, by_agent
from fixtures import build_home

if TYPE_CHECKING:
    from .plugin import Recorder

TOOLS = Path(__file__).resolve().parents[2] / "tools"


def _load_tool(name: str) -> ModuleType:
    """Import a module of `analyzer/tools` by its file name."""
    spec = importlib.util.spec_from_file_location("recorder_" + name, TOOLS / (name + ".py"))
    if spec is None or spec.loader is None:
        raise ImportError("cannot load %s" % name)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def record_full_home(recorder: Recorder) -> None:
    """Parse every artifact of the full fixture home with every parser that wants it."""
    recorder.start_test("recorder::full-home")
    try:
        work = Path(tempfile.mkdtemp(prefix="full-home-"))
        build_home(work / "home" / "alice")
        collection = open_input(work / "home", catalog.load(), host="h1")
        groups = by_agent()
        recorder.frozen = True
        for include_thinking in (False, True):
            options = Options(include_thinking=include_thinking)
            for artifact in collection.artifacts:
                for parser in cli.parsers_for(artifact, groups):
                    if parser.wants(artifact):
                        list(parser.parse(artifact, options))
    finally:
        recorder.frozen = False
        recorder.end_test()


def record_example_collection(recorder: Recorder) -> None:
    """Run the commands over the example collection the output documentation shows."""
    recorder.start_test("recorder::example-collection")
    examples = _load_tool("example_collection")
    regenerate = _load_tool("regenerate_examples")
    old = os.getcwd()
    try:
        work = Path(tempfile.mkdtemp(prefix="example-"))
        collection = str(examples.build_collection(work))
        os.chdir(work)
        runs = [
            ["detect", collection],
            ["detect", collection, "--json"],
            ["detect", collection, "--files"],
            ["timeline", collection, "-o", str(work / "out")],
            ["timeline", collection, "-o", str(work / "shown"), *regenerate.TIMELINE_FILTERS],
            ["inventory", collection, "-o", str(work / "fleet.csv")],
        ]
        for argv in runs:
            with (
                contextlib.redirect_stdout(io.StringIO()),
                contextlib.redirect_stderr(io.StringIO()),
            ):
                cli.main(argv)
    finally:
        os.chdir(old)
        recorder.end_test()


def run(recorder: Recorder) -> None:
    """Record both extra sets of cases."""
    record_full_home(recorder)
    record_example_collection(recorder)
