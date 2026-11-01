"""Tests of tools/check_branch_coverage.py on synthetic diffs, reports and packages."""

from __future__ import annotations

import importlib.util
import pathlib
import subprocess
import unittest

import check_branch_coverage as gate

HAS_COVERAGE = importlib.util.find_spec("coverage") is not None
TOY = '''"""A toy parser module with one branch the tests take one way only."""


def kind(value):
    """Return the kind of a value."""
    if value:
        return "set"
    return "unset"
'''
TOY_TEST = '''"""Takes the toy branch one way only."""

from doubleagent.parsers import toy


def test_set():
    assert toy.kind(1) == "set"
'''


def test_touched_lines_reads_new_side_of_each_hunk():
    diff = "\n".join(
        [
            "diff --git a/analyzer/doubleagent/parsers/one.py b/analyzer/doubleagent/parsers/one.py",
            "--- a/analyzer/doubleagent/parsers/one.py",
            "+++ b/analyzer/doubleagent/parsers/one.py",
            "@@ -3 +3 @@ def first():",
            "-old",
            "+new",
            "@@ -10,0 +11,3 @@",
            "+a",
            "+b",
            "+c",
            "@@ -20,2 +23,0 @@",
            "-gone",
            "-gone",
            "--- a/analyzer/doubleagent/parsers/two.py",
            "+++ /dev/null",
            "@@ -1,2 +0,0 @@",
            "-x",
            "-y",
        ]
    )
    assert gate.touched_lines(diff) == {"analyzer/doubleagent/parsers/one.py": {3, 11, 12, 13}}


def test_touched_lines_of_a_new_file_are_every_line():
    diff = "--- /dev/null\n+++ b/analyzer/doubleagent/parsers/new.py\n@@ -0,0 +1,4 @@\n+a\n"
    assert gate.touched_lines(diff) == {"analyzer/doubleagent/parsers/new.py": {1, 2, 3, 4}}


def test_is_parser_module_takes_only_modules_directly_under_the_parsers():
    cases = {
        "analyzer/doubleagent/parsers/codex.py": True,
        "analyzer/doubleagent/parsers/__init__.py": True,
        "analyzer/doubleagent/parsers/sub/inner.py": False,
        "analyzer/doubleagent/cli.py": False,
        "analyzer/doubleagent/parsers/notes.md": False,
    }
    for path, expected in cases.items():
        assert gate.is_parser_module(path) is expected, path


def test_pragma_without_reason_is_a_finding_and_with_reason_is_not():
    source = "\n".join(
        [
            "if a:  # pragma: no branch",
            "if b:  # pragma: no branch: the loader always sets it",
            "if c:  # PRAGMA no branch",
            "if d:",
        ]
    )
    findings = gate.pragma_findings("p.py", source, None)
    assert [str(finding) for finding in findings] == [
        "p.py:1: pragma: no branch without a reason",
        "p.py:3: pragma: no branch without a reason",
    ]


def test_pragma_rule_checks_only_the_touched_lines():
    source = "if a:  # pragma: no branch\nif b:  # pragma: no branch\n"
    assert [finding.line for finding in gate.pragma_findings("p.py", source, {2})] == [2]


def test_branch_findings_format_and_scope():
    source = "\n".join(
        ["def f(a):", "    if a:", "        return 1", "    while a:", "        a -= 1"]
    )
    missing = [[2, 3], [4, -1], [4, 5]]
    findings = gate.branch_findings("p.py", missing, {2, 4}, source)
    assert [str(finding) for finding in findings] == [
        "p.py:2: branch to 3 never taken",
        "p.py:4: branch to exit never taken",
        "p.py:4: branch to 5 never taken",
    ]
    assert gate.branch_findings("p.py", missing, {3}, source) == []
    assert len(gate.branch_findings("p.py", missing, None, source)) == 3


def test_branch_on_a_line_with_a_reasoned_pragma_is_exempt():
    source = "if a:  # pragma: no branch: never empty\n    pass\n"
    assert gate.branch_findings("p.py", [[1, 2]], None, source) == []


def test_missing_branches_keys_reports_by_repository_path(tmp_path):
    analyzer = tmp_path / "analyzer"
    report = {
        "files": {
            "doubleagent/parsers/toy.py": {"missing_branches": [[6, 7]]},
            str(analyzer / "doubleagent/parsers/other.py"): {"missing_branches": []},
            "/elsewhere/outside.py": {"missing_branches": [[1, 2]]},
        }
    }
    assert gate.missing_branches(report, tmp_path, analyzer) == {
        "analyzer/doubleagent/parsers/toy.py": [[6, 7]],
        "analyzer/doubleagent/parsers/other.py": [],
    }


def write_toy_repository(root: pathlib.Path) -> None:
    """Write an analyzer tree with one toy parser module and a test taking one branch way."""
    parsers = root / gate.PARSERS
    parsers.mkdir(parents=True)
    (root / "analyzer/doubleagent/__init__.py").write_text("", encoding="utf-8")
    (parsers / "__init__.py").write_text("", encoding="utf-8")
    (parsers / "toy.py").write_text(TOY, encoding="utf-8")
    tests = root / "analyzer/tests"
    tests.mkdir()
    (tests / "test_toy.py").write_text(TOY_TEST, encoding="utf-8")
    (root / "analyzer/pytest.ini").write_text("[pytest]\ntestpaths = tests\n", encoding="utf-8")


def test_all_reports_the_untaken_branch_of_a_synthetic_package(tmp_path, capsys):
    if not HAS_COVERAGE:
        raise unittest.SkipTest("coverage.py is not installed")
    write_toy_repository(tmp_path)
    status = gate.main(["--all"], root=tmp_path)
    output = capsys.readouterr().out
    assert status == 1, output
    assert "analyzer/doubleagent/parsers/toy.py:6: branch to 8 never taken" in output, output


def git(root: pathlib.Path, *arguments: str) -> None:
    """Run git in `root` with a fixed identity, failing the test on an error."""
    identity = ["-c", "user.name=Test", "-c", "user.email=test@example.invalid"]
    subprocess.run(["git", *identity, *arguments], cwd=root, check=True, capture_output=True)


def test_base_mode_checks_only_lines_changed_since_the_merge_base(tmp_path, capsys):
    if not HAS_COVERAGE:
        raise unittest.SkipTest("coverage.py is not installed")
    write_toy_repository(tmp_path)
    git(tmp_path, "init", "-q")
    git(tmp_path, "add", ".")
    git(tmp_path, "commit", "-q", "-m", "toy")
    assert gate.main(["--base", "HEAD"], root=tmp_path) == 0
    assert "no parser module changed" in capsys.readouterr().out
    toy = tmp_path / gate.PARSERS / "toy.py"
    toy.write_text(TOY.replace('return "unset"', 'return "empty"'), encoding="utf-8")
    assert gate.main(["--base", "HEAD"], root=tmp_path) == 0, capsys.readouterr().out
    toy.write_text(TOY.replace("if value:", "if value is not None:"), encoding="utf-8")
    status = gate.main(["--base", "HEAD"], root=tmp_path)
    output = capsys.readouterr().out
    assert status == 1, output
    assert output.strip() == "analyzer/doubleagent/parsers/toy.py:6: branch to 8 never taken"


def test_a_module_the_suite_never_imports_is_not_measured(tmp_path, capsys):
    if not HAS_COVERAGE:
        raise unittest.SkipTest("coverage.py is not installed")
    write_toy_repository(tmp_path)
    (tmp_path / gate.PARSERS / "unused.py").write_text("VALUE = 1\n", encoding="utf-8")
    assert gate.main(["--all"], root=tmp_path) == 1
    output = capsys.readouterr().out
    assert "analyzer/doubleagent/parsers/unused.py:1: not measured" in output, output
