"""The pytest plugin: patch the analyzer's entry points while the suite runs and record each call.

Loaded with `-p recorder.plugin` and active only then. Each test's temporary directories are made
under the padded root, with names drawn from a sequence seeded by the test's node id, so a run
records the same paths every time. The recording never changes what a call returns or raises:
a parse is consumed lazily, as its caller consumes it, and its rows are copied as they pass.
"""

from __future__ import annotations

import inspect
import os
import random
import sys
import tempfile
import types
from collections.abc import Callable, Generator
from datetime import UTC, datetime
from pathlib import Path
from typing import Any
from unittest import mock

import pytest

# A plugin named by -p loads before pytest puts the analyzer on the import path for the first
# test package; the suite would add the same entry a moment later.
ANALYZER = str(Path(__file__).resolve().parents[2])
if ANALYZER not in sys.path:
    sys.path.insert(0, ANALYZER)

from . import cases, corpus  # noqa: E402

PARSER_METHODS = ("parse", "wants", "secondary", "secondary_tolerance")
COMMANDS = ("detect", "timeline", "inventory", "catalog")
WRAPPED = "__recorder_wrapped__"


class NameSequence:
    """Temporary names as `tempfile` draws them, from a generator seeded per test."""

    characters = "abcdefghijklmnopqrstuvwxyz0123456789_"

    def __init__(self) -> None:
        """Start seeded with the empty node id."""
        self.rng = random.Random("")

    def __iter__(self) -> NameSequence:
        """Return the sequence itself, as `tempfile` expects of its name source."""
        return self

    def __next__(self) -> str:
        """Return the next eight-character name."""
        return "".join(self.rng.choices(self.characters, k=8))


class Tee:
    """A text stream that writes to the stream it replaced and keeps a copy."""

    def __init__(self, stream: Any) -> None:
        """Wrap `stream`."""
        self.stream = stream
        self.parts: list[str] = []

    def write(self, text: str) -> int:
        """Write `text` to the wrapped stream and keep it."""
        self.parts.append(text)
        return self.stream.write(text)

    def __getattr__(self, name: str) -> Any:
        """Pass every other attribute through to the wrapped stream."""
        return getattr(self.stream, name)


class Command:
    """The state of one `cli.main` run being recorded."""

    def __init__(self, argv: list[str]) -> None:
        """Start a run of `argv` with nothing read from the clock."""
        self.argv = argv
        self.clock: list[str] = []


class Recorder:
    """The recording run: the corpus, the snapshots and what the current test has done."""

    def __init__(self, store: corpus.Corpus, work: Path) -> None:
        """Record into `store`; `work` is removed when the session ends."""
        self.corpus = store
        self.work = work
        self.snapshots = corpus.Snapshotter(store)
        self.names = NameSequence()
        self.node = ""
        self.seq = 0
        self.depth = 0
        self.command: Command | None = None
        self.entries: dict[str, corpus.Snapshot] = {}
        # The tree and times of each combination of cached snapshots, by their ids; the
        # snapshots are held so that no id is reused while its combination is stored.
        self.stored: dict[tuple[int, ...], tuple[str, str]] = {}
        self.stored_snapshots: list[corpus.Snapshot] = []
        # True while the trees cannot change (the driver's parses), so snapshots are reused.
        self.frozen = False
        self.labels: dict[int, tuple[object, str]] = {}
        self.class_counts: dict[str, int] = {}
        self.call_counts: dict[str, int] = {}
        self.written: set[str] = set()
        self.patches: list[str] = []
        self.monkeypatched: list[str] = []
        self.nodes: list[str] = []
        self.nodes_with_cases: set[str] = set()

    # ---- tests ----------------------------------------------------------------------------

    def start_test(self, node: str) -> None:
        """Begin a test: name it, reseed the temporary names and forget the cached snapshots."""
        self.node = node
        self.nodes.append(node)
        # One seed for every test, so a test's directories get the names the last test's had and
        # the fixture files that hold their own paths are the same bytes from test to test.
        self.names.rng = random.Random("")
        self.forget_snapshots()
        self.monkeypatched.clear()

    def end_test(self) -> None:
        """End a test: forget its snapshots and its monkeypatches."""
        self.forget_snapshots()
        self.monkeypatched.clear()
        self.node = ""

    # ---- trees ----------------------------------------------------------------------------

    def tree(self, paths: list[Any], fresh: bool, check: str = "") -> tuple[str | None, str | None]:
        """Return the tree and times of the root entries `paths` lie under, or two Nones.

        Inside a command the snapshot of an entry is taken once. Outside, `fresh` retakes it when
        any path it walked changed status, and `check` names the one path whose change retakes it.
        """
        tops = {corpus.top_of(self.corpus.root, os.fspath(path)) for path in paths if path}
        tops.discard("")
        if not tops:
            return None, None
        settled = self.command is not None or self.frozen
        stale = bool(check) and not settled and not self.snapshots.unchanged(check)
        snapshots = []
        for top in sorted(tops, key=os.fsencode):
            cached = self.entries.get(top)
            if cached is None or stale or (fresh and not settled and not cached.current()):
                self.entries[top] = self.snapshots.root_entry(top)
            snapshots.append(self.entries[top])
        key = tuple(id(snapshot) for snapshot in snapshots)
        if key not in self.stored:
            entries = [snapshot.entry for snapshot in snapshots if snapshot.entry is not None]
            mtimes: dict[str, int] = {}
            for snapshot in snapshots:
                mtimes.update(snapshot.mtimes)
            self.stored[key] = (self.corpus.tree(entries), self.corpus.times(mtimes))
            self.stored_snapshots.extend(snapshots)
        return self.stored[key]

    def forget_snapshots(self) -> None:
        """Drop the cached snapshots, so the next case takes its own."""
        self.entries.clear()
        self.stored.clear()
        self.stored_snapshots.clear()

    # ---- cases ----------------------------------------------------------------------------

    def write(self, group: str, case: dict[str, Any], dedupe: bool = False) -> None:
        """Write a case with its node, sequence and active patches; skip a repeated one."""
        self.seq += 1
        case["node"] = self.node
        patched = self.patches + self.monkeypatched
        if patched:
            case["patched"] = sorted(set(patched))
        portable = self.corpus.portable(case)
        if dedupe:
            key = corpus.dumps({**portable, "call": None})
            if key in self.written:
                return
            self.written.add(key)
        portable["seq"] = self.seq
        if group == "inputs" or portable.get("method") == "parse":
            # Most fixture homes are opened and parsed many times alike, so a result is a blob.
            result = portable.pop("result", None)
            if result is not None:
                portable["result_blob"] = self.corpus.blob(corpus.dumps(result).encode("utf-8"))
        self.nodes_with_cases.add(self.node)
        self.corpus.write_case(group, portable)

    def label(self, parser: object) -> str:
        """Return the instance label of a parser and count this call on it."""
        known = self.labels.get(id(parser))
        if known is None or known[0] is not parser:
            name = type(parser).__name__
            self.class_counts[name] = self.class_counts.get(name, 0) + 1
            known = (parser, "%s#%d" % (name, self.class_counts[name]))
            self.labels[id(parser)] = known
        return known[1]

    def parser_case(self, parser: Any, method: str, artifact: Any) -> dict[str, Any]:
        """Return the fields every parser case shares."""
        label = self.label(parser)
        self.call_counts[label] = self.call_counts.get(label, 0) + 1
        return {
            "method": method,
            "agent": getattr(parser, "agent", ""),
            "class": type(parser).__name__,
            "module": type(parser).__module__,
            "instance": label,
            "call": self.call_counts[label],
            "artifact": cases.artifact_fields(artifact),
        }

    def parser_group(self, parser: Any) -> str:
        """Return the case file of a parser's agent."""
        return "parsers/%s" % (getattr(parser, "agent", "") or "unknown")

    def parse(
        self, original: Callable, parser: Any, artifact: Any, options: Any
    ) -> Generator[Any, None, None]:
        """Run a parse lazily, recording its rows, its end and any exception."""
        case = self.parser_case(parser, "parse", artifact)
        case["options"] = dict(vars(options)) if hasattr(options, "__dict__") else repr(options)
        case["tree"], case["times"] = self.tree([getattr(artifact, "disk_path", "")], fresh=True)
        rows: list[Any] = []
        result: dict[str, Any] = {"rows": rows, "completed": False, "error": None}
        try:
            self.depth += 1
            try:
                iterator = iter(original(parser, artifact, options))
            finally:
                self.depth -= 1
            while True:
                self.depth += 1
                try:
                    row = next(iterator)
                except StopIteration:
                    result["completed"] = True
                    return
                finally:
                    self.depth -= 1
                rows.append(cases.row_fields(row))
                yield row
        except GeneratorExit:
            raise
        except BaseException as error:
            result["error"] = cases.error_fields(error)
            raise
        finally:
            case["result"] = result
            self.write(self.parser_group(parser), case)

    def decide(self, original: Callable, method: str, parser: Any, artifact: Any) -> Any:
        """Run a wants-style method and record its value or exception."""
        case = self.parser_case(parser, method, artifact)
        disk_path = os.fspath(getattr(artifact, "disk_path", "") or "")
        case["tree"], case["times"] = self.tree([disk_path], fresh=False, check=disk_path)
        self.depth += 1
        try:
            value = original(parser, artifact)
        except BaseException as error:
            case["result"] = {"error": cases.error_fields(error)}
            raise
        else:
            case["result"] = {"value": value}
        finally:
            self.depth -= 1
            if "result" in case:
                self.write(self.parser_group(parser), case, dedupe=True)
        return value

    def open_input(self, original: Callable, args: tuple, kwargs: dict) -> Any:
        """Run `open_input` and record the collection it returned or the exception it raised."""
        names = ("path", "catalog", "work_dir", "host")
        bound = dict(zip(names, args, strict=False)) | kwargs
        case: dict[str, Any] = {key: bound.get(key) for key in ("path", "work_dir", "host")}
        paths = [bound.get("path"), bound.get("work_dir")]
        case["tree"], case["times"] = self.tree(paths, fresh=True)
        try:
            collection = original(*args, **kwargs)
        except BaseException as error:
            case["error"] = cases.error_fields(error)
            self.write("inputs", case)
            raise
        case["result"] = cases.collection_fields(collection)
        self.write("inputs", case)
        return collection

    def main(self, original: Callable, argv: list[str] | None) -> Any:
        """Run `cli.main` and record its streams, exit, clock and the files it wrote."""
        arguments = list(sys.argv[1:] if argv is None else argv)
        cwd = os.getcwd()
        paths = [cwd, *(token.split("=", 1)[-1] for token in arguments)]
        outer = self.command
        self.command = Command(arguments)
        self.forget_snapshots()
        before, times = self.tree(paths, fresh=True)
        stdout, stderr = Tee(sys.stdout), Tee(sys.stderr)
        sys.stdout, sys.stderr = stdout, stderr  # type: ignore[assignment]
        case: dict[str, Any] = {"argv": arguments, "cwd": cwd, "tree": before, "times": times}
        try:
            code = original(argv)
            case["exit"] = code
            return code
        except SystemExit as error:
            case["exit"] = error.code
            raise
        except BaseException as error:
            case["error"] = cases.error_fields(error)
            raise
        finally:
            sys.stdout, sys.stderr = stdout.stream, stderr.stream
            case["clock"] = self.command.clock
            case["stdout"] = "".join(stdout.parts)
            case["stderr"] = "".join(stderr.parts)
            self.command = outer
            self.forget_snapshots()
            self.outputs(case, before, self.tree(paths, fresh=True)[0])
            command = arguments[0] if arguments and arguments[0] in COMMANDS else "other"
            self.write("commands/" + command, case)

    def outputs(self, case: dict[str, Any], before: str | None, after: str | None) -> None:
        """Fill in the files a command created, changed or removed."""
        trees = self.corpus.trees
        old = corpus.tree_files(trees, before) if before else {}
        new = corpus.tree_files(trees, after) if after else {}
        outputs: dict[str, Any] = {}
        for path, (kind, value) in sorted(new.items()):
            if old.get(path) != (kind, value):
                outputs[path] = value if kind == "file" else {"link": value}
        case["outputs"] = outputs
        case["removed"] = sorted(path for path in old if path not in new)

    def now(self) -> datetime:
        """Return the current time, kept in the running command's clock."""
        moment = datetime.now(UTC)
        if self.command is not None:
            self.command.clock.append(moment.isoformat())
        return moment


# ---- patching ---------------------------------------------------------------------------------


def _wrap_method(recorder: Recorder, owner: type, name: str) -> None:
    """Replace `owner.name` with a recording wrapper, once."""
    original = owner.__dict__.get(name)
    if not isinstance(original, types.FunctionType) or getattr(original, WRAPPED, False):
        return
    # A parse that is not a generator (the base class's, which raises) runs eagerly, so a lazy
    # recording wrapper would move its exception; it is left as it is.
    if name == "parse" and not inspect.isgeneratorfunction(original):
        return
    function: Callable = original

    def parse_wrapper(self, artifact, opts):
        if recorder.depth:
            return function(self, artifact, opts)
        return recorder.parse(function, self, artifact, opts)

    def decide_wrapper(self, artifact):
        if recorder.depth:
            return function(self, artifact)
        return recorder.decide(function, name, self, artifact)

    wrapper = parse_wrapper if name == "parse" else decide_wrapper
    wrapper.__name__ = original.__name__
    wrapper.__qualname__ = original.__qualname__
    wrapper.__doc__ = original.__doc__
    setattr(wrapper, WRAPPED, True)
    setattr(owner, name, wrapper)


def _wrap_parser_class(recorder: Recorder, owner: type) -> None:
    """Wrap every recorded method a parser class defines itself."""
    for name in PARSER_METHODS:
        _wrap_method(recorder, owner, name)


def _all_subclasses(base: type) -> list[type]:
    """Return every subclass of `base`, at any depth."""
    found: list[type] = []
    pending = [base]
    while pending:
        for child in pending.pop().__subclasses__():
            if child not in found:
                found.append(child)
                pending.append(child)
    return found


def _replace_everywhere(old: Any, new: Any) -> None:
    """Point every loaded module's attribute that holds `old` at `new`."""
    for module in list(sys.modules.values()):
        namespace = getattr(module, "__dict__", None)
        if not isinstance(namespace, dict):
            continue
        for key, value in list(namespace.items()):
            if value is old:
                setattr(module, key, new)


def install(recorder: Recorder) -> None:
    """Patch the parsers, `open_input`, `cli.main`, the clock and the temporary names."""
    from doubleagent import cli, filters, inputs
    from doubleagent.parsers.base import Parser

    for owner in [Parser, *_all_subclasses(Parser)]:
        _wrap_parser_class(recorder, owner)

    def init_subclass(cls, **kwargs):
        super(Parser, cls).__init_subclass__(**kwargs)
        _wrap_parser_class(recorder, cls)

    Parser.__init_subclass__ = classmethod(init_subclass)  # type: ignore[assignment]

    open_input = inputs.open_input

    def recording_open_input(*args, **kwargs):
        return recorder.open_input(open_input, args, kwargs)

    _replace_everywhere(open_input, recording_open_input)
    main = cli.main

    def recording_main(argv=None):
        return recorder.main(main, argv)

    _replace_everywhere(main, recording_main)
    parse_bound = filters.parse_bound

    def recording_parse_bound(text, end=False, now=None):
        return parse_bound(text, end, now if now is not None else recorder.now())

    _replace_everywhere(parse_bound, recording_parse_bound)
    _watch_patches(recorder)
    tempfile.tempdir = recorder.corpus.root
    tempfile._name_sequence = recorder.names  # type: ignore[attr-defined]


def _watch_patches(recorder: Recorder) -> None:
    """Keep the targets of the active `mock.patch` and `monkeypatch` calls in the recorder."""
    patch_class: Any = mock._patch  # type: ignore[attr-defined]
    enter, leave = patch_class.__enter__, patch_class.__exit__

    def target_name(patch: Any) -> str:
        target = patch.getter()
        return "%s.%s" % (getattr(target, "__name__", type(target).__name__), patch.attribute)

    def recording_enter(self):
        recorder.patches.append(target_name(self))
        return enter(self)

    def recording_exit(self, *exc_info):
        name = target_name(self)
        if name in recorder.patches:
            recorder.patches.remove(name)
        return leave(self, *exc_info)

    patch_class.__enter__, patch_class.__exit__ = recording_enter, recording_exit
    setattr_original = pytest.MonkeyPatch.setattr

    def recording_setattr(self, target, *args, **kwargs):
        if isinstance(target, str):
            shown = target
        else:
            attribute = args[0] if args else kwargs.get("name", "")
            shown = "%s.%s" % (getattr(target, "__name__", type(target).__name__), attribute)
        recorder.monkeypatched.append(shown)
        return setattr_original(self, target, *args, **kwargs)

    pytest.MonkeyPatch.setattr = recording_setattr  # type: ignore[method-assign]


# ---- hooks --------------------------------------------------------------------------------------

RECORDER_KEY = pytest.StashKey[Recorder]()


def pytest_configure(config: pytest.Config) -> None:
    """Start the recording: open the corpus, make the padded root and patch the analyzer."""
    # Every worker of a parallel run would write the same corpus files.
    if config.getoption("numprocesses", default=None) or os.environ.get("PYTEST_XDIST_WORKER"):
        raise pytest.UsageError("recorder.plugin records one process: run it with -n 0")
    target = os.environ.get("PARITY_CORPUS", "")
    if not target:
        raise pytest.UsageError("recorder.plugin needs PARITY_CORPUS, the corpus directory")
    directory = Path(target).absolute()
    work = Path(os.environ.get("PARITY_RECORD_WORK", "") or str(directory) + ".work").absolute()
    repo = str(Path(ANALYZER).parent)
    root = corpus.make_work_root(work)
    recorder = Recorder(corpus.Corpus(directory, root, repo), work)
    config.stash[RECORDER_KEY] = recorder
    install(recorder)


@pytest.hookimpl(wrapper=True)
def pytest_runtest_protocol(item: pytest.Item, nextitem: pytest.Item | None):
    """Name the running test for the cases it produces."""
    recorder = item.config.stash[RECORDER_KEY]
    recorder.start_test(item.nodeid)
    try:
        return (yield)
    finally:
        recorder.end_test()


def pytest_sessionfinish(session: pytest.Session) -> None:
    """Run the driver, write the summary and remove the work directory."""
    from . import driver

    recorder = session.config.stash.get(RECORDER_KEY, None)
    if recorder is None:
        return
    driver.run(recorder)
    silent = [node for node in recorder.nodes if node not in recorder.nodes_with_cases]
    recorder.corpus.close(silent)
    tempfile.tempdir = None
    corpus.remove_work(recorder.work)
