#!/usr/bin/env python3
"""Check that the analyzer suite takes both ways of every branch a parser change touches.

AGENTS.md asks a parser's fixture for the present and the absent form of every key the parser
branches on. This gate measures it with coverage.py in branch mode: it finds the parser modules
(`analyzer/doubleagent/parsers/*.py`) that differ between the working tree and the merge base
of `--base` with HEAD, untracked ones included, runs the analyzer suite once from `analyzer/`
under `coverage run --branch` restricted to those files, reads `coverage json`, and reports each
branch the suite never took from a statement whose header the change added or modified (every
line of an untracked module; with `--all`, every parser module and every line):

    <file>:<line>: branch to <line> never taken
    <file>:<line>: branch to exit never taken

`exit` is the branch that leaves the function. coverage.py places a branch on the first line
of its statement and honours a pragma on any line of a statement's header (up to its closing
colon), so a touched line of a header checks the whole header. A line carrying
`# pragma: no branch: <reason>` is exempt, as coverage.py itself skips it; a `no branch` or
`no cover` pragma in any spelling coverage.py accepts, on a checked line, without a colon and a
reason after it is a finding, `pragma: no <kind> without a reason`, since AGENTS.md wants every
suppression to give one and `no cover` drops its whole block, branches included. A changed
module the suite never imports is one finding, `not measured: the suite never imports it`.

Usage: tools/check_branch_coverage.py [--base REF] [--all]

The exit status is 0 when no module changed or nothing is found, 1 on a finding, 2 when git,
the suite or coverage.py fails. Findings and the line saying no module changed go to stdout;
the summary and the error of a failed step go to stderr.
"""

from __future__ import annotations

import argparse
import ast
import bisect
import io
import json
import pathlib
import re
import subprocess
import sys
import tempfile
import tokenize
from collections.abc import Iterable, Sequence
from dataclasses import dataclass

ROOT = pathlib.Path(__file__).resolve().parent.parent
ANALYZER = "analyzer"
PARSERS = "analyzer/doubleagent/parsers"
DEFAULT_BASE = "origin/main"
# The new side of a unified diff hunk header: `+start` or `+start,count`; a count of 0 is a
# pure deletion, which touches no line of the new file.
HUNK_HEADER = re.compile(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@")
# coverage.py's default `partial_branches` and `exclude_lines` patterns in one; group 1 is the
# pragma's kind, and a reason is a colon and text right after the match.
PRAGMA = re.compile(r"#\s*(?:pragma|PRAGMA)[:\s]?\s*(?:no|NO)\s*(branch|BRANCH|cover|COVER)")
REASON = re.compile(r"\s*:\s*\S")
# The fields of a compound statement that hold its body rather than its header.
BODY_FIELDS = frozenset({"body", "orelse", "finalbody", "handlers", "cases"})


class GateError(RuntimeError):
    """A git, suite or coverage.py step that failed, with the message to print."""


@dataclass(frozen=True)
class Finding:
    """One reported line: the repository-relative file, the line and what is wrong there."""

    path: str
    line: int
    message: str

    def __str__(self) -> str:
        """Return the `<file>:<line>: <message>` form the gate prints."""
        return f"{self.path}:{self.line}: {self.message}"


def run_git(root: pathlib.Path, arguments: Sequence[str]) -> str:
    """Run git in `root` and return its stdout.

    Raises:
        GateError: git exits non-zero.
    """
    completed = subprocess.run(
        ["git", *arguments], cwd=root, capture_output=True, text=True, check=False
    )
    if completed.returncode != 0:
        raise GateError(f"git {' '.join(arguments)} failed: {completed.stderr.strip()}")
    return completed.stdout


def is_parser_module(path: str) -> bool:
    """Return True for a repository-relative path of a module directly under the parsers."""
    parent, _, name = path.rpartition("/")
    return parent == PARSERS and name.endswith(".py")


def changed_modules(root: pathlib.Path, base: str) -> tuple[str, list[str], list[str]]:
    """Return the merge base of `base` and HEAD, the modules changed since, and the untracked.

    The comparison is with the working tree, so uncommitted edits count; a deleted module is
    left out, as there is nothing left to measure, and an ignored file is no module.
    """
    merge_base = run_git(root, ["merge-base", base, "HEAD"]).strip()
    listing = run_git(root, ["diff", "--name-only", "--diff-filter=d", merge_base, "--", PARSERS])
    untracked = run_git(root, ["ls-files", "--others", "--exclude-standard", "--", PARSERS])
    changed = sorted(path for path in listing.splitlines() if is_parser_module(path))
    new = sorted(path for path in untracked.splitlines() if is_parser_module(path))
    return merge_base, changed, new


def all_modules(root: pathlib.Path) -> list[str]:
    """Return every parser module, repository-relative and sorted."""
    return sorted(path.relative_to(root).as_posix() for path in (root / PARSERS).glob("*.py"))


def touched_lines(diff: str) -> dict[str, set[int]]:
    """Return the new-file line numbers each file's hunks add or modify in a `-U0` diff.

    A file is named by the `+++ b/<path>` line; `+++ /dev/null` (a deletion) names none.
    """
    lines: dict[str, set[int]] = {}
    current: set[int] | None = None
    for text in diff.split("\n"):
        if text.startswith("+++ "):
            target = text[4:]
            current = lines.setdefault(target[2:], set()) if target.startswith("b/") else None
            continue
        match = HUNK_HEADER.match(text)
        if match and current is not None:
            start = int(match.group(1))
            count = 1 if match.group(2) is None else int(match.group(2))
            current.update(range(start, start + count))
    return lines


def diff_lines(root: pathlib.Path, merge_base: str, modules: Sequence[str]) -> dict[str, set[int]]:
    """Return the lines of `modules` the working tree changed since `merge_base`."""
    # The prefixes are fixed so a user's diff.noprefix or diff.mnemonicPrefix leaves the
    # `+++ b/` lines touched_lines reads.
    options = ["-U0", "--no-color", "--no-ext-diff", "--src-prefix=a/", "--dst-prefix=b/"]
    diff = run_git(root, ["diff", *options, merge_base, "--", *modules])
    return touched_lines(diff)


def header_colons(source: str) -> list[tuple[int, int]]:
    """Return the position, `(line, column)`, of every `:` outside brackets, in order."""
    colons = []
    depth = 0
    for token in tokenize.generate_tokens(io.StringIO(source).readline):
        if token.type != tokenize.OP:
            continue
        if token.string in "([{":
            depth += 1
        elif token.string in ")]}":
            depth -= 1
        elif token.string == ":" and depth == 0:
            colons.append(token.start)
    return colons


def header_end(parts: Iterable[ast.AST], start: tuple[int, int], colons: list) -> int:
    """Return the line of the colon closing a header whose expressions are `parts`.

    The colon is the first one outside brackets after both `start` and the end of every
    expression, so a lambda or slice inside the header is passed over.
    """
    position = start
    for part in parts:
        for node in ast.walk(part):
            end_line = getattr(node, "end_lineno", None)
            if end_line is not None:
                position = max(position, (end_line, getattr(node, "end_col_offset", 0) or 0))
    index = bisect.bisect_left(colons, position)
    return colons[index][0] if index < len(colons) else position[0]


def statement_spans(source: str) -> list[tuple[int, int]]:
    """Return the first and last line of every statement header of `source`.

    A simple statement is all header; a compound one's header ends at its closing colon, so the
    blank, comment and body lines below it are outside, and a `case` clause has its own header.
    Source that does not parse has no spans.
    """
    try:
        tree = ast.parse(source)
        colons = header_colons(source)
    except (SyntaxError, tokenize.TokenError):
        return []
    spans = []
    for node in ast.walk(tree):
        if isinstance(node, ast.match_case):
            parts = [node.pattern] + ([node.guard] if node.guard else [])
            first = node.pattern.lineno
            spans.append((first, header_end(parts, (first, 0), colons)))
            continue
        if not isinstance(node, ast.stmt):
            continue
        fields = dict(ast.iter_fields(node))
        if not BODY_FIELDS & fields.keys():
            spans.append((node.lineno, node.end_lineno or node.lineno))
            continue
        decorators = fields.pop("decorator_list", None) or []
        parts = [
            value
            for name, field in fields.items()
            if name not in BODY_FIELDS
            for value in (field if isinstance(field, list) else [field])
            if isinstance(value, ast.AST)
        ]
        first = min([node.lineno] + [decorator.lineno for decorator in decorators])
        spans.append((first, header_end(parts, (node.lineno, node.col_offset), colons)))
    return spans


def checked_lines(source: str, lines: set[int] | None) -> set[int] | None:
    """Return `lines` with every line of each statement header they touch added; None stays."""
    if lines is None:
        return None
    checked = set(lines)
    for first, last in statement_spans(source):
        span = range(first, last + 1)
        if any(number in lines for number in span):
            checked.update(span)
    return checked


def pragma_findings(path: str, source: str, lines: set[int] | None) -> list[Finding]:
    """Return a finding per `no branch` or `no cover` pragma without a reason on a checked line.

    `lines` holds the checked line numbers; None checks every line.
    """
    findings = []
    for number, text in enumerate(source.split("\n"), start=1):
        if lines is not None and number not in lines:
            continue
        match = PRAGMA.search(text)
        if match and not REASON.match(text, match.end()):
            kind = match.group(1).lower()
            findings.append(Finding(path, number, f"pragma: no {kind} without a reason"))
    return findings


def has_branch_reason(text: str) -> bool:
    """Return True when `text` carries a `no branch` pragma with a reason."""
    match = PRAGMA.search(text)
    return bool(match and match.group(1).lower() == "branch" and REASON.match(text, match.end()))


def branch_findings(
    path: str, missing: Iterable[Sequence[int]], lines: set[int] | None, source: str
) -> list[Finding]:
    """Return a finding per missed branch, `[from, to]`, whose source line is checked.

    `lines` holds the checked line numbers, None every line; a negative `to` is coverage.py's
    exit from the function. A line exempted by a pragma with a reason is skipped.
    """
    source_lines = source.split("\n")
    findings = []
    for start, target in missing:
        if lines is not None and start not in lines:
            continue
        text = source_lines[start - 1] if 0 < start <= len(source_lines) else ""
        if has_branch_reason(text):
            continue
        where = "exit" if target < 0 else str(target)
        findings.append(Finding(path, start, f"branch to {where} never taken"))
    return findings


def missing_branches(report: dict, root: pathlib.Path, analyzer: pathlib.Path) -> dict:
    """Return each measured file's `missing_branches`, keyed by repository-relative path.

    coverage.py names a file relative to the directory it ran in when the file lies below it.
    """
    branches = {}
    for name, entry in report.get("files", {}).items():
        path = pathlib.Path(name)
        absolute = path if path.is_absolute() else analyzer / path
        try:
            relative = absolute.resolve().relative_to(root.resolve()).as_posix()
        except ValueError:
            continue
        branches[relative] = entry.get("missing_branches", [])
    return branches


def measure(root: pathlib.Path, modules: Sequence[str]) -> dict:
    """Run the analyzer suite once under branch coverage of `modules` and return the JSON report.

    `--include` names the files, since `--source` with the package's `__init__` would measure
    the whole package.

    Raises:
        GateError: the suite fails or coverage.py writes no report.
    """
    analyzer = root / ANALYZER
    include = ",".join(str((root / module).resolve()) for module in modules)
    with tempfile.TemporaryDirectory(prefix="branch-coverage-") as work:
        data_file = str(pathlib.Path(work) / "coverage.data")
        report_file = pathlib.Path(work) / "coverage.json"
        coverage = [sys.executable, "-m", "coverage"]
        run = [*coverage, "run", "--branch", f"--data-file={data_file}", f"--include={include}"]
        suite = subprocess.run([*run, "-m", "pytest", "-q"], cwd=analyzer, check=False)
        if suite.returncode != 0:
            raise GateError(f"the analyzer suite failed under coverage (exit {suite.returncode})")
        export = [*coverage, "json", f"--data-file={data_file}", "-o", str(report_file)]
        written = subprocess.run(export, cwd=analyzer, capture_output=True, text=True, check=False)
        if written.returncode != 0 or not report_file.exists():
            message = (written.stdout + written.stderr).strip()
            raise GateError(f"coverage json failed (exit {written.returncode}): {message}")
        return json.loads(report_file.read_text(encoding="utf-8"))


def check(root: pathlib.Path, base: str, every_line: bool) -> tuple[list[str], list[Finding]]:
    """Return the modules checked and the findings, sorted by file and line."""
    lines: dict[str, set[int] | None]
    if every_line:
        modules = all_modules(root)
        lines = dict.fromkeys(modules)
    else:
        merge_base, changed, untracked = changed_modules(root, base)
        lines = dict(diff_lines(root, merge_base, changed)) if changed else {}
        lines.update(dict.fromkeys(untracked))
        modules = sorted({*changed, *untracked})
    if not modules:
        return [], []
    branches = missing_branches(measure(root, modules), root, root / ANALYZER)
    findings: list[Finding] = []
    for module in modules:
        source = (root / module).read_text(encoding="utf-8")
        checked = checked_lines(source, lines.get(module, set()))
        findings += pragma_findings(module, source, checked)
        if module in branches:
            findings += branch_findings(module, branches[module], checked, source)
        elif checked is None or checked:
            findings.append(Finding(module, 1, "not measured: the suite never imports it"))
    findings.sort(key=lambda finding: (finding.path, finding.line, finding.message))
    return modules, findings


def main(argv: Sequence[str] | None = None, root: pathlib.Path = ROOT) -> int:
    """Run the gate and return the exit status."""
    parser = argparse.ArgumentParser(description=(__doc__ or "").split("\n")[0])
    parser.add_argument("--base", default=DEFAULT_BASE, help="the ref to diff against")
    parser.add_argument("--all", action="store_true", help="every parser module, every line")
    options = parser.parse_args(argv)
    try:
        modules, findings = check(root, options.base, options.all)
    except GateError as error:
        print(f"check_branch_coverage: {error}", file=sys.stderr)
        return 2
    if not modules:
        print(f"check_branch_coverage: no parser module changed against {options.base}")
        return 0
    for finding in findings:
        print(finding)
    print(
        f"check_branch_coverage: {len(modules)} module(s), {len(findings)} finding(s)",
        file=sys.stderr,
    )
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main())
