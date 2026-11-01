#!/usr/bin/env python3
"""Show which smoke-image paths a change to the sh collector's tables moves between manifest rows.

The exclusion and secret globs match relative to every collection base and their `*` crosses
`/`, so their reach is easy to misjudge by reading them. This tool lets the collector itself
decide: it builds the main fake image of `collectors/tests/smoke.sh` once
(`SMOKE_IMAGE_ONLY=1`), the image its first `run` collects, runs the collector of `BASE` and
of `HEAD` over it in image mode with the command line the smoke test's `run` uses, and
compares the two `manifest.jsonl` files, row by row per path and base.

It prints the table lines added and removed between the two scripts' `--list`, then every
image path whose rows changed in `status`, `agent` or `secret`, or that only one manifest has,
grouped under each changed line whose glob matches the path relative to a base whose rows
changed, and `unattributed` when none does. A `CATALOG` or `PROJECT_CATALOG` glob matches the
leading segments of the path relative to its base, segment by segment; an `EXCLUDES` glob
matches that relative path or one of its parent directories, a `SECRET_GLOBS` glob the
relative path itself, both with `*` crossing `/`; a `DOCKER_VOLUMES` glob matches the name of
the volume whose `_data` is one of the path's bases. The image holds only the smoke test's
main fixtures, so an overmatch shows only where a fixture path exists.

The image is built from the working tree's smoke test. `HEAD` defaults to the working tree's
collector; a commit is read with `git show`. The run needs no tool beyond git and the shell.

Usage: tools/diff_smoke_rows.py [--shell SHELL] [--fail] BASE [HEAD]

Exit status: 0, or 1 with `--fail` when any row changed; 2 when a step cannot run.
"""

from __future__ import annotations

import argparse
import fnmatch
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
from collections.abc import Iterable
from dataclasses import dataclass

import drift_common

ROOT = drift_common.ROOT
COLLECTOR = pathlib.PurePosixPath("collectors/collect-agent-artifacts.sh")
SMOKE_TEST = ROOT / "collectors" / "tests" / "smoke.sh"
TABLE_LABELS = {
    "home": "CATALOG",
    "project": "PROJECT_CATALOG",
    "exclude": "EXCLUDES",
    "secret": "SECRET_GLOBS",
    "docker": "DOCKER_VOLUMES",
}
NOTE = (
    "note: the image holds only the smoke test's main fixtures, so a glob that matches more"
    " than it should shows here only where a fixture path exists to catch it."
)
Facts = tuple[str, str, str, bool]


class DiffError(RuntimeError):
    """A step of the comparison could not run."""


@dataclass(frozen=True)
class TableChange:
    """One table line added (`+`) or removed (`-`) between the two scripts."""

    sign: str
    table: str
    line: str

    def glob(self) -> str:
        """Return the line's glob, without the `agent|` field of the agent tables."""
        if self.table in ("home", "project", "docker"):
            return self.line.partition("|")[2]
        return self.line

    def label(self) -> str:
        """Return the line as printed: sign, table name and line."""
        return f"{self.sign} {TABLE_LABELS[self.table]} {self.line}"


@dataclass(frozen=True)
class Row:
    """The fields of one manifest row this tool compares, with the base the path is under."""

    path: str
    home: str
    status: str
    agent: str
    secret: bool

    def facts(self) -> Facts:
        """Return the compared fields: base, status, agent and secret flag."""
        return (self.home, self.status, self.agent, self.secret)


@dataclass(frozen=True)
class RowChange:
    """One path whose rows differ between the manifests under the bases in `homes`.

    A path can have rows under two bases, such as a home and a project inside it; `homes`,
    `before` and `after` hold only the bases whose rows changed, and an empty `before` or
    `after` means that manifest has no row under them.
    """

    path: str
    homes: tuple[str, ...]
    before: tuple[Facts, ...]
    after: tuple[Facts, ...]


def parse_tables(listing: str) -> dict[str, list[str]]:
    """Return the five tables of a `--list` output, each a list of lines in order."""
    catalog = drift_common.catalog_module().parse(listing)
    return {
        "home": [f"{entry.agent}|{entry.glob}" for entry in catalog.entries],
        "project": [f"{entry.agent}|{entry.glob}" for entry in catalog.project_entries],
        "exclude": list(catalog.excludes),
        "secret": list(catalog.secrets),
        "docker": [f"{entry.agent}|{entry.glob}" for entry in catalog.docker_volumes],
    }


def diff_tables(
    before: dict[str, list[str]], after: dict[str, list[str]]
) -> tuple[list[TableChange], bool]:
    """Return the lines removed and added per table, and whether the volume lines were reordered.

    Order matters only in `DOCKER_VOLUMES`, where the first matching line wins, so the flag
    compares the order of the volume lines both scripts have.
    """
    changes: list[TableChange] = []
    for table in TABLE_LABELS:
        old_lines, new_lines = before.get(table, []), after.get(table, [])
        old_set, new_set = set(old_lines), set(new_lines)
        changes.extend(TableChange("-", table, line) for line in old_lines if line not in new_set)
        changes.extend(TableChange("+", table, line) for line in new_lines if line not in old_set)
    old_docker, new_docker = before.get("docker", []), after.get("docker", [])
    common = set(old_docker) & set(new_docker)
    reordered = [line for line in old_docker if line in common] != [
        line for line in new_docker if line in common
    ]
    return changes, reordered


def read_manifest(text: str) -> list[Row]:
    """Return the rows of a `manifest.jsonl` text; a line that is not a JSON object raises."""
    rows: list[Row] = []
    for number, line in enumerate(text.split("\n"), 1):
        if not line.strip():
            continue
        try:
            record = json.loads(line)
        except json.JSONDecodeError as error:
            raise DiffError(f"manifest line {number} is not JSON ({error})") from error
        if not isinstance(record, dict):
            raise DiffError(f"manifest line {number} is not a JSON object")
        rows.append(
            Row(
                str(record.get("path", "")),
                str(record.get("home", "")),
                str(record.get("status", "")),
                str(record.get("agent", "")),
                record.get("secret") is True,
            )
        )
    return rows


def diff_rows(before: Iterable[Row], after: Iterable[Row]) -> list[RowChange]:
    """Return each path whose rows under some base differ or exist on one side, sorted by path.

    Rows are compared per path and base, so facts trading places between two bases of one
    path are a change of both bases.
    """
    old_rows: dict[tuple[str, str], list[Facts]] = {}
    new_rows: dict[tuple[str, str], list[Facts]] = {}
    for rows, index in ((before, old_rows), (after, new_rows)):
        for row in rows:
            index.setdefault((row.path, row.home), []).append(row.facts())
    changed: dict[str, list[str]] = {}
    for key in sorted(set(old_rows) | set(new_rows)):
        if sorted(old_rows.get(key, [])) != sorted(new_rows.get(key, [])):
            changed.setdefault(key[0], []).append(key[1])
    changes: list[RowChange] = []
    for path, homes in changed.items():
        old_facts = sorted(facts for home in homes for facts in old_rows.get((path, home), []))
        new_facts = sorted(facts for home in homes for facts in new_rows.get((path, home), []))
        changes.append(RowChange(path, tuple(homes), tuple(old_facts), tuple(new_facts)))
    return changes


def relative_path(path: str, home: str) -> str:
    """Return `path` relative to its collection base `home`, `""` for the base itself."""
    if path == home:
        return ""
    prefix = home.rstrip("/") + "/"
    return path[len(prefix) :] if path.startswith(prefix) else path


def volume_name(home: str) -> str:
    """Return the volume name of a base `.../volumes/<name>/_data`, else `""`."""
    parts = home.rstrip("/").split("/")
    if len(parts) >= 2 and parts[-1] == "_data":
        return parts[-2]
    return ""


def line_matches(change: TableChange, path: str, home: str) -> bool:
    """Return whether the changed line's glob reaches `path`, collected under base `home`."""
    glob = change.glob()
    if change.table == "docker":
        name = volume_name(home)
        return bool(name) and fnmatch.fnmatchcase(name, glob)
    relative = relative_path(path, home)
    if not relative:
        return False
    parts = relative.split("/")
    if change.table in ("home", "project"):
        segments = [segment for segment in glob.split("/") if segment]
        return len(parts) >= len(segments) and all(
            fnmatch.fnmatchcase(part, segment)
            for part, segment in zip(parts[: len(segments)], segments, strict=True)
        )
    if change.table == "secret":
        return fnmatch.fnmatchcase(relative, glob)
    return any(fnmatch.fnmatchcase("/".join(parts[:end]), glob) for end in range(1, len(parts) + 1))


def attribute(
    table_changes: list[TableChange], row_changes: list[RowChange]
) -> list[tuple[TableChange | None, list[RowChange]]]:
    """Group each changed path under every changed line matching it under a changed base.

    A path no changed line matches goes under `None`. The groups follow the order of
    `table_changes`, the unattributed group last; a group with no path is left out.
    """
    groups: dict[TableChange | None, list[RowChange]] = {change: [] for change in table_changes}
    groups[None] = []
    for row_change in row_changes:
        matched = [
            change
            for change in table_changes
            if any(line_matches(change, row_change.path, home) for home in row_change.homes)
        ]
        for change in matched or [None]:
            groups[change].append(row_change)
    return [(change, paths) for change, paths in groups.items() if paths]


def describe_facts(facts: tuple[Facts, ...], image_root: str) -> str:
    """Return a path's row facts as printed, `(no row)` when the manifest has none."""
    if not facts:
        return "(no row)"
    return "; ".join(
        f"{status} agent={agent or '-'} secret={'true' if secret else 'false'}"
        f" base={display_path(home, image_root)}"
        for home, status, agent, secret in facts
    )


def display_path(path: str, image_root: str) -> str:
    """Return `path` with the image root removed, as the path reads on the imaged host."""
    prefix = image_root.rstrip("/")
    return path[len(prefix) :] if prefix and path.startswith(prefix + "/") else path


def render_report(
    base_name: str,
    head_name: str,
    table_changes: list[TableChange],
    reordered: bool,
    row_changes: list[RowChange],
    image_root: str,
) -> str:
    """Return the printed report: the table-line diff, the path groups and the closing note."""
    lines = [f"table lines changed from {base_name} to {head_name}:"]
    lines.extend(f"  {change.label()}" for change in table_changes)
    if not table_changes:
        lines.append("  (none)")
    if reordered:
        lines.append("  DOCKER_VOLUMES lines reordered: the first matching line wins")
    lines.append("")
    lines.append(f"paths whose rows changed: {len(row_changes)}")
    for change, paths in attribute(table_changes, row_changes):
        lines.append("")
        lines.append(change.label() if change else "unattributed")
        for row_change in paths:
            lines.append(
                f"  {display_path(row_change.path, image_root)}: "
                f"{describe_facts(row_change.before, image_root)} -> "
                f"{describe_facts(row_change.after, image_root)}"
            )
    lines.append("")
    lines.append(NOTE)
    return "\n".join(lines) + "\n"


def collector_command(
    shell: str, script: str, image_root: str, output: str
) -> tuple[dict[str, str], list[str]]:
    """Return the environment additions and arguments of one image-mode collector run.

    They are those of the smoke test's `run`: the shell runs the script and, through
    `COLLECTOR_SH`, its `find -exec` workers.
    """
    arguments = [shell, script, "-r", image_root, "-o", output, "-q", "--max-file-size", "1"]
    return {"COLLECTOR_SH": shell}, arguments


def image_command(shell: str) -> tuple[dict[str, str], list[str]]:
    """Return the environment additions and arguments that build the smoke image alone."""
    return {"SMOKE_IMAGE_ONLY": "1"}, [shell, str(SMOKE_TEST), shell]


def run_process(arguments: list[str], extra_environment: dict[str, str]) -> str:
    """Run a command from the repository root and return its stdout; a failure raises."""
    environment = dict(os.environ, **extra_environment)
    try:
        completed = subprocess.run(
            arguments, cwd=ROOT, env=environment, capture_output=True, text=True, check=False
        )
    except OSError as error:
        raise DiffError(f"{arguments[0]} could not be run ({error})") from error
    if completed.returncode != 0:
        raise DiffError(
            f"{' '.join(arguments[:3])} exited {completed.returncode}: {completed.stderr.strip()}"
        )
    return completed.stdout


def write_collector(revision: str | None, directory: pathlib.Path, name: str) -> str:
    """Write the collector of `revision` (the working tree's when None) and return its path."""
    if revision is None:
        return str(ROOT / COLLECTOR)
    script = directory / f"{name}.sh"
    script.write_text(run_process(["git", "show", f"{revision}:{COLLECTOR}"], {}))
    return str(script)


def collect(shell: str, script: str, image_root: str, output: pathlib.Path) -> list[Row]:
    """Run one collector over the image and return its manifest rows."""
    output.mkdir(parents=True)
    environment, arguments = collector_command(shell, script, image_root, str(output))
    run_process(arguments, environment)
    manifests = sorted(output.glob("*.manifest.jsonl"))
    if len(manifests) != 1:
        raise DiffError(f"{output}: expected one manifest, found {len(manifests)}")
    return read_manifest(manifests[0].read_text(encoding="utf-8"))


def build_image(shell: str) -> pathlib.Path:
    """Build the smoke image and return its work directory, the last line the test prints.

    A printed `cac-smoke.*` directory without the image's `root/` is removed before the error,
    so a build without its image leaves nothing behind; any other printed path is never removed.
    """
    environment, arguments = image_command(shell)
    printed = [line for line in run_process(arguments, environment).split("\n") if line.strip()]
    work = pathlib.Path(printed[-1]) if printed else None
    if work is not None and work.name.startswith("cac-smoke.") and work.is_dir():
        if (work / "root").is_dir():
            return work
        shutil.rmtree(work)
    raise DiffError(f"the smoke test printed no image work directory: {printed[-1:]}")


def compare(base: str, head: str | None, shell: str) -> tuple[str, int]:
    """Build the image, run both collectors and return the report and the changed-path count."""
    work = build_image(shell)
    try:
        image_root = str(work / "root")
        with tempfile.TemporaryDirectory(prefix="diff-smoke-rows.") as scratch:
            directory = pathlib.Path(scratch)
            scripts = [
                write_collector(base, directory, "base"),
                write_collector(head, directory, "head"),
            ]
            listings = [run_process([shell, script, "--list"], {}) for script in scripts]
            table_changes, reordered = diff_tables(*(parse_tables(text) for text in listings))
            before = collect(shell, scripts[0], image_root, directory / "base-out")
            after = collect(shell, scripts[1], image_root, directory / "head-out")
            row_changes = diff_rows(before, after)
    finally:
        shutil.rmtree(work)
    head_name = head if head is not None else "the working tree"
    report = render_report(base, head_name, table_changes, reordered, row_changes, image_root)
    return report, len(row_changes)


def main(argv: list[str] | None = None) -> int:
    """Run the comparison and print its report."""
    parser = argparse.ArgumentParser(
        description="Show which smoke-image paths a table change moves between manifest rows."
    )
    parser.add_argument("base", help="the commit to compare from")
    parser.add_argument("head", nargs="?", help="the commit to compare to; the working tree")
    parser.add_argument("--shell", default="/bin/sh", help="the shell running the collectors")
    parser.add_argument("--fail", action="store_true", help="exit 1 when any row changed")
    arguments = parser.parse_args(argv)
    try:
        report, changed = compare(arguments.base, arguments.head, arguments.shell)
    except DiffError as error:
        print(f"diff_smoke_rows: {error}", file=sys.stderr)
        return 2
    sys.stdout.write(report)
    return 1 if arguments.fail and changed else 0


if __name__ == "__main__":
    sys.exit(main())
