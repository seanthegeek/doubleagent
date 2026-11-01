"""Inputs that cannot be read.

The reason is printed, the exit code is 2 for the input itself, and anything unreadable inside an
input is a `parser:` row from the parser that reads it, or a problem line where no parser does,
never a traceback and never silence.
"""

import ast
import contextlib
import io
import json
import os
import shutil
import stat
import tarfile
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from doubleagent import cli
from doubleagent.parsers.claude_code import ClaudeCodeParser
from fixtures import CLAUDE_SESSION, build_home

CAN_DENY = os.name != "nt" and hasattr(os, "geteuid") and os.geteuid() != 0
DENY_REASON = "chmod does not deny access as root or on Windows"

SESSION_REL = ".claude/projects/-srv-proj/" + CLAUDE_SESSION + ".jsonl"


def run_cli(argv):
    out, error = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(out), contextlib.redirect_stderr(error):
        exit_code = cli.main(argv)
    return exit_code, out.getvalue(), error.getvalue()


def build_collected(root: Path, extra_rows=()) -> Path:
    """A minimal extracted collection.

    One Claude Code transcript under fs/home/alice with its manifest row, plus any extra rows given.
    """
    home = build_home(root / "src", with_noise=False)
    dest = root / "col" / "fs/home/alice" / SESSION_REL
    dest.parent.mkdir(parents=True)
    shutil.copyfile(home / SESSION_REL, dest)
    rows = [manifest_row(SESSION_REL), *extra_rows]
    col = root / "col"
    with open(col / "manifest.jsonl", "w", encoding="utf-8") as handle:
        for row in rows:
            handle.write(json.dumps(row) + "\n")
    (col / "collection.json").write_text('{"hostname": "h1"}', encoding="utf-8")
    return col


def manifest_row(rel: str) -> dict:
    return {
        "path": "/home/alice/" + rel,
        "home": "/home/alice",
        "user": "alice",
        "agent": "claude-code",
        "type": "file",
        "status": "collected",
        "size": 1,
        "archive_path": "fs/home/alice/" + rel,
    }


def make_archive(source: Path, dest: Path) -> Path:
    with tarfile.open(dest, "w:gz") as archive:
        for entry in sorted(source.iterdir()):
            archive.add(entry, arcname=entry.name)
    return dest


class ErrorTestBase(unittest.TestCase):
    """A temporary directory and a way to make a path unreadable for the test."""

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-errors-test-"))
        self._locked: list[Path] = []

    def tearDown(self):
        for path in self._locked:
            os.chmod(path, stat.S_IRWXU)
        shutil.rmtree(self.tmp, ignore_errors=True)

    def lock(self, path: Path) -> None:
        os.chmod(path, 0)
        self._locked.append(path)

    def assertInputError(self, argv, path, reason):
        exit_code, out, error = run_cli(argv)
        self.assertEqual(exit_code, 2, out + error)
        self.assertEqual(error, "error: %s: %s\n" % (path, reason))
        self.assertNotIn("Traceback", out + error)


class MissingInputTests(ErrorTestBase):
    """An input that does not exist is an error with exit code 2."""

    def test_detect(self):
        path = self.tmp / "nope.tar.gz"
        self.assertInputError(["detect", str(path)], path, "no such file or directory")

    def test_timeline(self):
        path = self.tmp / "nope.tar.gz"
        self.assertInputError(
            ["timeline", str(path), "-o", str(self.tmp / "out")], path, "no such file or directory"
        )
        self.assertFalse((self.tmp / "out").exists())

    def test_inventory(self):
        path = self.tmp / "nope.txt"
        self.assertInputError(
            ["inventory", str(path), "-o", str(self.tmp / "f.csv")],
            path,
            "no such file or directory",
        )
        self.assertFalse((self.tmp / "f.csv").exists())


class MissingFromCollectionTests(ErrorTestBase):
    """A manifest row whose file is absent from fs/ is one problem line."""

    def setUp(self):
        super().setUp()
        gone = manifest_row(".claude/projects/-srv-proj/gone.jsonl")
        self.col = build_collected(self.tmp, [gone])
        self.line = "/home/alice/.claude/projects/-srv-proj/gone.jsonl: missing from the collection"

    def check(self, path):
        exit_code, out, error = run_cli(["timeline", str(path), "-o", str(self.tmp / "tl")])
        self.assertEqual(exit_code, 0, out + error)
        self.assertEqual(out.count("problem:   " + self.line + "\n"), 1, out)
        rows = (self.tmp / "tl" / "timeline.jsonl").read_text(encoding="utf-8").splitlines()
        self.assertGreater(len(rows), 0)
        inv = json.loads((self.tmp / "tl" / "inventory.json").read_text(encoding="utf-8"))
        self.assertEqual(inv["problems"], [self.line])
        exit_code, out, error = run_cli(["detect", str(path)])
        self.assertEqual(exit_code, 0, out + error)
        self.assertIn("problem: " + self.line + "\n", out)

    def test_extracted_collection(self):
        self.check(self.col)

    def test_archive(self):
        self.check(make_archive(self.col, self.tmp / "x_agent-artifacts.tar.gz"))


@unittest.skipUnless(CAN_DENY, DENY_REASON)
class PermissionTests(ErrorTestBase):
    """An unreadable input is an error; an unreadable file inside one is a problem line."""

    def test_unreadable_archive(self):
        archive = make_archive(build_collected(self.tmp), self.tmp / "y.tar.gz")
        self.lock(archive)
        self.assertInputError(["detect", str(archive)], archive, "permission denied")
        self.assertInputError(
            ["timeline", str(archive), "-o", str(self.tmp / "out")], archive, "permission denied"
        )

    def test_archive_in_untraversable_directory(self):
        locked = self.tmp / "locked"
        locked.mkdir()
        archive = make_archive(build_collected(self.tmp), locked / "x.tar.gz")
        self.lock(locked)
        self.assertInputError(["detect", str(archive)], archive, "permission denied")
        self.assertInputError(
            ["timeline", str(archive), "-o", str(self.tmp / "out")], archive, "permission denied"
        )

    def test_unreadable_input_directory(self):
        home = build_home(self.tmp / "alice")
        self.lock(home)
        self.assertInputError(["detect", str(home)], home, "permission denied")

    def test_unreadable_manifest(self):
        col = build_collected(self.tmp)
        self.lock(col / "manifest.jsonl")
        self.assertInputError(["detect", str(col)], col / "manifest.jsonl", "permission denied")

    def test_unreadable_inventory_input(self):
        path = self.tmp / "ws01.txt"
        path.write_text("", encoding="utf-8")
        self.lock(path)
        self.assertInputError(
            ["inventory", str(path), "-o", str(self.tmp / "f.csv")], path, "permission denied"
        )

    def test_unreadable_directory_in_loose_tree(self):
        root = self.tmp / "image"
        build_home(root / "home/alice")
        build_home(root / "home/bob")
        locked_home = root / "home/bob"
        locked_agent = root / "home/alice/.codex"
        self.lock(locked_home)
        self.lock(locked_agent)
        exit_code, out, error = run_cli(["detect", str(root)])
        self.assertEqual(exit_code, 0, out + error)
        self.assertIn("problem: %s: permission denied\n" % locked_home, out)
        self.assertIn("problem: %s: permission denied\n" % locked_agent, out)
        self.assertEqual(out.count("problem:"), 2, out)
        exit_code, out, error = run_cli(["detect", str(root), "--json"])
        self.assertEqual(
            sorted(json.loads(out)["problems"]),
            sorted("%s: permission denied" % path for path in (locked_home, locked_agent)),
        )
        exit_code, out, error = run_cli(["timeline", str(root), "-o", str(self.tmp / "tl")])
        self.assertEqual(exit_code, 0, out + error)
        self.assertIn("problem:   %s: permission denied\n" % locked_agent, out)

    def test_unreadable_transcript_is_a_parser_row(self):
        col = build_collected(self.tmp)
        self.lock(col / "fs/home/alice" / SESSION_REL)
        exit_code, out, error = run_cli(["timeline", str(col), "-o", str(self.tmp / "tl")])
        self.assertEqual(exit_code, 0, out + error)
        self.assertNotIn("problem:", out)
        lines = (self.tmp / "tl" / "timeline.jsonl").read_text(encoding="utf-8").splitlines()
        self.assertEqual(
            [json.loads(line)["text"] for line in lines],
            ["parser: %s could not be read (Permission denied)" % Path(SESSION_REL).name],
        )

    def test_unreadable_file_problem_names_path_once(self):
        # A reader that lets the OSError leave the parser loses the artifact
        # to one problem line naming its path.
        col = build_collected(self.tmp)
        denied = PermissionError(13, "Permission denied")
        with mock.patch.object(ClaudeCodeParser, "parse", side_effect=denied):
            exit_code, out, error = run_cli(["timeline", str(col), "-o", str(self.tmp / "tl")])
        self.assertEqual(exit_code, 1, out + error)  # the only transcript could not be read
        self.assertNotIn("Traceback", out + error)
        self.assertIn("problem:   /home/alice/%s: permission denied\n" % SESSION_REL, out)
        self.assertNotIn("Errno", out)
        # The parser problem is in inventory.json as well.
        inv = json.loads((self.tmp / "tl" / "inventory.json").read_text(encoding="utf-8"))
        self.assertEqual(inv["problems"], ["/home/alice/%s: permission denied" % SESSION_REL])


if __name__ == "__main__":
    unittest.main()


class ErrorListsReportedTests(unittest.TestCase):
    """Every error list a parser fills is read again.

    It yields its `system` row, is returned or is passed on: a list created empty and only handed to
    a reader drops its bad lines silently. A pre-scan that discards errors passes a literal `[]`
    instead, because the same parser reports them when it parses the file in full.
    """

    PARSERS = Path(__file__).resolve().parent.parent / "doubleagent" / "parsers"

    def test_every_error_list_is_read_again(self):
        silent = []
        for path in sorted(self.PARSERS.glob("*.py")):
            tree = ast.parse(path.read_text(encoding="utf-8"), str(path))
            for func in ast.walk(tree):
                if not isinstance(func, (ast.FunctionDef, ast.AsyncFunctionDef)):
                    continue
                for name, line in _empty_error_lists(func):
                    loads = sum(
                        1
                        for node in ast.walk(func)
                        if isinstance(node, ast.Name)
                        and node.id == name
                        and isinstance(node.ctx, ast.Load)
                    )
                    if loads < 2:
                        silent.append("%s:%d %s()" % (path.name, line, func.name))
        self.assertEqual(silent, [], "error lists filled but never reported")


def _empty_error_lists(func):
    """(name, line) of each local error list in `func` assigned an empty list.

    (name, line) of each local in `func` whose name ends in `errors` and that is assigned an empty
    list literal, annotated or not.
    """
    for node in ast.walk(func):
        if isinstance(node, ast.Assign):
            targets, value = node.targets, node.value
        elif isinstance(node, ast.AnnAssign) and node.value is not None:
            targets, value = [node.target], node.value
        else:
            continue
        if not (isinstance(value, ast.List) and not value.elts):
            continue
        for target in targets:
            if isinstance(target, ast.Name) and target.id.endswith("errors"):
                yield target.id, node.lineno


class IterJsonlTests(unittest.TestCase):
    """The shared JSONL reader reports every line it does not yield."""

    def test_non_object_line_is_an_error(self):
        # Issue 228: a line that is valid JSON but not an object (an array,
        # a string, a number, null) was skipped silently.
        from doubleagent.parsers.base import iter_jsonl

        lines = [b'{"a": 1}', b"[1, 2]", b'"text"', b"42", b"null", b"{bad", b'{"b": 2}']
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "t.jsonl"
            path.write_bytes(b"\r\n".join(lines) + b"\n")
            errors: list = []
            records = list(iter_jsonl(path, errors))
        self.assertEqual(records, [(1, {"a": 1}), (7, {"b": 2})])
        self.assertEqual(
            [(line, raw) for line, _message, raw in errors],
            [(2, "[1, 2]"), (3, '"text"'), (4, "42"), (5, "null"), (6, "{bad")],
        )
        self.assertEqual(
            [message for _line, message, _raw in errors[:4]],
            ["unparseable line (not a JSON object)"] * 4,
        )
        self.assertTrue(errors[4][1].startswith("unparseable line (Expecting"))


def test_iter_jsonl_unreadable_file_is_one_entry(tmp_path):
    if not CAN_DENY:
        raise unittest.SkipTest(DENY_REASON)
    from doubleagent.parsers.base import iter_jsonl

    path = tmp_path / "t.jsonl"
    path.write_bytes(b'{"a": 1}\n')
    path.chmod(0)
    errors: list = []
    try:
        records = list(iter_jsonl(path, errors))
    finally:
        path.chmod(stat.S_IRUSR | stat.S_IWUSR)
    assert records == []
    assert errors == [(0, "t.jsonl could not be read (Permission denied)")]


class _FailingHandle(io.BytesIO):
    """A file whose second line fails to read as a disk read error does."""

    def __next__(self):
        if self.tell():
            raise OSError(5, "Input/output error")
        return super().__next__()


def test_iter_jsonl_read_error_keeps_earlier_lines(tmp_path):
    from doubleagent.parsers.base import iter_jsonl

    handle = _FailingHandle(b'{"a": 1}\n{"b": 2}\n')
    errors: list = []
    with mock.patch("doubleagent.parsers.base.open", create=True, return_value=handle):
        records = list(iter_jsonl(tmp_path / "t.jsonl", errors))
    assert records == [(1, {"a": 1})]
    assert errors == [(1, "t.jsonl could not be read (Input/output error)")]


@contextlib.contextmanager
def _deny(path: Path):
    """Make `path` unreadable for the block, then readable again."""
    if not CAN_DENY:
        raise unittest.SkipTest(DENY_REASON)
    path.chmod(0)
    try:
        yield
    finally:
        path.chmod(stat.S_IRUSR | stat.S_IWUSR)


def test_iter_lines_numbers_lines_and_keeps_terminators(tmp_path):
    from doubleagent.parsers.base import iter_lines

    cases = [
        (b"a\nb\r\nc", [(1, b"a\n"), (2, b"b\r\n"), (3, b"c")]),
        (b"a\n", [(1, b"a\n")]),
        (b"x\ry\n\n", [(1, b"x\ry\n"), (2, b"\n")]),
        (b"", []),
    ]
    for content, expected in cases:
        path = tmp_path / "t.txt"
        path.write_bytes(content)
        errors: list = []
        assert list(iter_lines(path, errors)) == expected, content
        assert errors == [], content


def test_iter_lines_unreadable_file_is_one_entry(tmp_path):
    from doubleagent.parsers.base import iter_lines

    path = tmp_path / "t.txt"
    path.write_bytes(b"a\nb\n")
    errors: list = []
    with _deny(path):
        lines = list(iter_lines(path, errors))
    assert lines == []
    assert errors == [(0, "t.txt could not be read (Permission denied)")]


def test_iter_lines_read_error_keeps_earlier_lines(tmp_path):
    from doubleagent.parsers.base import iter_lines

    handle = _FailingHandle(b"a\nb\n")
    errors: list = []
    with mock.patch("doubleagent.parsers.base.open", create=True, return_value=handle):
        lines = list(iter_lines(tmp_path / "t.txt", errors))
    assert lines == [(1, b"a\n")]
    assert errors == [(1, "t.txt could not be read (Input/output error)")]


def test_file_bytes_returns_the_content(tmp_path):
    from doubleagent.parsers.base import file_bytes

    path = tmp_path / "t.json"
    path.write_bytes(b'\xef\xbb\xbf{"a": 1}\r\n')
    errors: list = []
    assert file_bytes(path, errors) == b'\xef\xbb\xbf{"a": 1}\r\n'
    assert errors == []


def test_file_bytes_unreadable_file_is_one_entry(tmp_path):
    from doubleagent.parsers.base import file_bytes

    path = tmp_path / "t.json"
    path.write_bytes(b"{}")
    errors: list = []
    with _deny(path):
        content = file_bytes(path, errors)
    assert content is None
    assert errors == [(0, "t.json could not be read (Permission denied)")]


def test_read_text_drops_a_bom_and_nothing_else(tmp_path):
    from doubleagent.parsers.cline_legacy import read_text

    cases = [
        (b'\xef\xbb\xbf["a"]\r\n', '["a"]\r\n'),
        (b'["a"]', '["a"]'),
        (b"\xef\xbb\xbf\xef\xbb\xbfx", "﻿x"),
        (b"\xff", "�"),
    ]
    for content, expected in cases:
        path = tmp_path / "t.json"
        path.write_bytes(content)
        errors: list = []
        assert read_text(path, errors) == expected, content
        assert errors == [], content


def test_read_text_missing_file_and_symlink_are_silent(tmp_path):
    from doubleagent.parsers.cline_legacy import read_text

    target = tmp_path / "target.json"
    target.write_bytes(b"[]")
    link = tmp_path / "link.json"
    try:
        link.symlink_to(target)
    except (OSError, NotImplementedError) as error:
        raise unittest.SkipTest("cannot create a symlink: %s" % error) from error
    for path in (tmp_path / "missing.json", link, tmp_path):
        errors: list = []
        assert read_text(path, errors) is None, path
        assert errors == [], path


def test_legacy_readers_report_an_unreadable_file(tmp_path):
    from doubleagent.parsers.cline_legacy import iter_json_array, load_json, read_text

    path = tmp_path / "t.json"
    path.write_bytes(b'["a"]')
    readers = [
        ("read_text", lambda errors: read_text(path, errors), None),
        ("iter_json_array", lambda errors: list(iter_json_array(path, errors)), []),
        ("load_json", lambda errors: load_json(path, errors), None),
    ]
    with _deny(path):
        for name, reader, empty in readers:
            errors: list = []
            assert reader(errors) == empty, name
            assert errors == [(0, "t.json could not be read (Permission denied)")], name


def test_load_json_leaves_a_decode_failure_to_the_caller(tmp_path):
    from doubleagent.parsers.cline_legacy import load_json

    path = tmp_path / "t.json"
    path.write_bytes(b'{"a": ')
    errors: list = []
    assert load_json(path, errors) is None
    assert errors == []


def test_cached_json_reports_an_unreadable_file_on_every_call(tmp_path):
    from doubleagent.parsers.cline import ClineParser

    parser = ClineParser()
    path = tmp_path / "t.json"
    path.write_bytes(b'[{"id": "1"}]')
    errors: list = []
    assert parser.cached_json(path, errors) == [{"id": "1"}]
    assert parser.cached_json(tmp_path / "missing.json", errors) is None
    assert errors == []
    locked = tmp_path / "locked.json"
    locked.write_bytes(b"[]")
    with _deny(locked):
        for call in (1, 2):
            errors = []
            assert parser.cached_json(locked, errors) is None, call
            assert errors == [(0, "locked.json could not be read (Permission denied)")], call
