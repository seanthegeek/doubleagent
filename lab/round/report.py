#!/usr/bin/env python3
"""Write the report of one round cell as Markdown.

    python3 -I report.py CELL

Reads what round.sh left in the cell directory: `state`, `steps.log`, the listings, each
collection's manifest under `collection/<label>/` and the analyzer's `timeline.jsonl` under
`analysis/<label>/`. Writes `CELL/report.md` and prints its path. Per step it gives the status and
the listing diff's counts and top groups; per collection the manifest's rows per status, the error
rows, the `skipped_excluded` rows and the unflagged-key list of AGENTS.md "Validation and testing"
(collected rows not flagged `secret: true` whose file name holds key, token, secret, cred, cookie
or auth); per analysis the rows per session and per row form, marking the forms an earlier
collection of the cell lacked, and every `parser:` row.

No value from a file is printed: sessions are numbered, paths have UUIDs and hex runs redacted,
and a `parser:` row is shown without the line it quotes. Standard library only; imports classify.py
from beside it.
"""

from __future__ import annotations

import argparse
import collections
import itertools
import json
import pathlib
import sys
from collections.abc import Mapping

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

import classify  # beside this file, which python3 -I leaves off the path

UNFLAGGED_WORDS = ("key", "token", "secret", "cred", "cookie", "auth")
STEP_FIELDS = ("label", "kind", "status", "started", "finished", "note")


def read_state(cell: pathlib.Path) -> dict[str, str]:
    """Return the cell's state, the last value of each key."""
    state: dict[str, str] = {}
    path = cell / "state"
    if path.is_file():
        for line in path.read_text(encoding="utf-8").split("\n"):
            key, separator, value = line.partition("=")
            if separator:
                state[key] = value
    return state


def read_steps(cell: pathlib.Path) -> list[dict[str, str]]:
    """Return the lines of steps.log as dicts of STEP_FIELDS."""
    path = cell / "steps.log"
    if not path.is_file():
        return []
    steps = []
    for line in path.read_text(encoding="utf-8").split("\n"):
        if line:
            fields = line.split("\t") + [""] * len(STEP_FIELDS)
            steps.append(dict(zip(STEP_FIELDS, fields[: len(STEP_FIELDS)], strict=False)))
    return steps


def listing_summary(before: pathlib.Path, after: pathlib.Path) -> tuple[str, str]:
    """Return the counts cell and the top groups cell of one listing diff."""
    difference = classify.compare(
        classify.load_listing(str(before)), classify.load_listing(str(after))
    )
    counts = "+%d -%d ~%d" % (
        len(difference.added),
        len(difference.removed),
        len(difference.changed),
    )
    if difference.new_tops or difference.gone_tops:
        counts += " top %d" % (len(difference.new_tops) + len(difference.gone_tops))
    if difference.states:
        counts += " state %d" % len(difference.states)
    groups = collections.Counter(
        classify.group_key(path)
        for path in difference.added + difference.removed + difference.changed
    )
    top = ", ".join(
        "`%s` %d" % (classify.redact(group), count) for group, count in groups.most_common(3)
    )
    return counts, top


def steps_section(cell: pathlib.Path, steps: list[dict[str, str]]) -> list[str]:
    """Return the report lines of the steps table and the missed and failed steps."""
    listings = sorted((cell / "listings").glob("*.txt"))
    previous = {after.stem: before for before, after in itertools.pairwise(listings)}
    lines = [
        "## Steps",
        "",
        "| Step | Kind | Status | Listing diff | Most changed |",
        "| --- | --- | --- | --- | --- |",
    ]
    for step in steps:
        counts, top = "", ""
        after = cell / "listings" / (step["label"] + ".txt")
        if after.is_file() and step["label"] in previous:
            counts, top = listing_summary(previous[step["label"]], after)
        lines.append(
            "| %s | %s | %s | %s | %s |"
            % (step["label"], step["kind"], step["status"], counts, top)
        )
    unfinished = [step for step in steps if step["status"] not in ("done",)]
    lines += ["", "Missed or failed steps, each a finding to repeat or explain:", ""]
    lines += [
        "- %s (%s): %s %s" % (step["label"], step["kind"], step["status"], step["note"])
        for step in unfinished
    ]
    if not unfinished:
        lines.append("- none")
    return [*lines, ""]


def read_jsonl(path: pathlib.Path) -> list[dict[str, object]]:
    """Return the JSON objects of a JSONL file; a line that does not decode is skipped."""
    records = []
    for line in path.read_text(encoding="utf-8", errors="replace").split("\n"):
        try:
            record = json.loads(line)
        except ValueError:
            continue
        if isinstance(record, dict):
            records.append(record)
    return records


def manifest_lines(directory: pathlib.Path) -> list[str]:
    """Return the report lines of one collection's manifest."""
    manifests = sorted(directory.glob("*.manifest.jsonl"))
    if not manifests:
        return ["No manifest in `%s`." % directory.name, ""]
    rows = read_jsonl(manifests[0])
    statuses = collections.Counter(str(row.get("status", "")) for row in rows)
    lines = [
        "Rows per status: " + ", ".join("%s %d" % item for item in sorted(statuses.items())),
        "",
    ]
    errors = [row for row in rows if str(row.get("status", "")).startswith("error")]
    excluded = [row for row in rows if row.get("status") == "skipped_excluded"]
    unflagged = [
        row
        for row in rows
        if row.get("status") == "collected"
        and row.get("secret") is not True
        and any(
            word in pathlib.PurePath(str(row.get("path", ""))).name.lower()
            for word in UNFLAGGED_WORDS
        )
    ]
    for title, chosen in (
        ("Error rows", errors),
        ("skipped_excluded rows", excluded),
        ("Unflagged-key rows", unflagged),
    ):
        lines.append("%s: %d" % (title, len(chosen)))
        lines += [
            "- `%s` %s %s"
            % (classify.redact(str(row.get("path", ""))), row.get("type", ""), row.get("size", ""))
            for row in chosen
        ]
        lines.append("")
    return lines


def row_form(row: Mapping[str, object]) -> str:
    """Return a timeline row's form: its turn type, and for a system row its text's label."""
    turn_type = str(row.get("turn_type", ""))
    if turn_type != "system":
        return turn_type
    head = str(row.get("text", "")).split("\n", 1)[0]
    label, separator, _ = head.partition(":")
    if separator and len(label) <= 40:
        return "system %s:" % label
    return "system %s" % " ".join(head.split()[:2])


def analysis_lines(timeline: pathlib.Path, earlier_forms: set[str]) -> tuple[list[str], set[str]]:
    """Return the report lines of one timeline and the row forms it holds."""
    rows = read_jsonl(timeline)
    sessions: dict[tuple[str, str], int] = collections.Counter()
    numbers: dict[str, int] = {}
    forms: collections.Counter[str] = collections.Counter()
    problems = []
    for row in rows:
        session = str(row.get("session_id", ""))
        if session and session not in numbers:
            numbers[session] = len(numbers) + 1
        sessions[
            (str(row.get("agent", "")), "S%d" % numbers[session] if session else "no session")
        ] += 1
        forms[row_form(row)] += 1
        text = str(row.get("text", ""))
        if text.startswith("parser:"):
            # `parser: unparseable line (<reason>): <line>`: the line itself is left out.
            problems.append(": ".join(text.split("\n", 1)[0].split(": ", 2)[:2]))
    lines = [
        "Rows: %d in %d sessions." % (len(rows), len(numbers)),
        "",
        "| Agent | Session | Rows |",
        "| --- | --- | --- |",
    ]
    lines += [
        "| %s | %s | %d |" % (agent, session, count)
        for (agent, session), count in sorted(sessions.items())
    ]
    lines += ["", "| Row form | Rows | New in this collection |", "| --- | --- | --- |"]
    for form, count in sorted(forms.items()):
        lines.append("| `%s` | %d | %s |" % (form, count, "" if form in earlier_forms else "yes"))
    lines += ["", "`parser:` rows: %d" % len(problems)]
    lines += ["- %s ..." % classify.redact(problem) for problem in problems]
    return [*lines, ""], set(forms)


def collections_section(cell: pathlib.Path) -> list[str]:
    """Return the report lines of every collection and its analysis, oldest first."""
    lines = ["## Collections", ""]
    labels = sorted(path.name for path in (cell / "collection").glob("*") if path.is_dir())
    if not labels:
        return [*lines, "No collection yet: run `round.sh ... collect`.", ""]
    seen_forms: set[str] = set()
    for label in labels:
        lines += ["### %s" % label, ""]
        lines += manifest_lines(cell / "collection" / label)
        timeline = cell / "analysis" / label / "timeline.jsonl"
        if timeline.is_file():
            more, forms = analysis_lines(timeline, seen_forms)
            lines += more
            seen_forms |= forms
        else:
            lines += ["No analyzer output: see `analysis/%s.stdout.txt`." % label, ""]
        shapes = cell / "shapes" / (label + ".txt")
        lines += [
            "Shapes: `%s`" % shapes.relative_to(cell) if shapes.is_file() else "Shapes: none",
            "",
        ]
    return lines


def build_report(cell: pathlib.Path) -> str:
    """Return the whole report of a cell."""
    state = read_state(cell)
    steps = read_steps(cell)
    lines = ["# Round cell %s on %s" % (state.get("agent", "?"), state.get("vm", "?")), ""]
    for key in ("version", "install_source", "tui_test", "tui_test_sha256", "started", "finished"):
        lines.append("- %s: %s" % (key.replace("_", " "), state.get(key) or "-"))
    lines += ["- waiting for the person at: %s" % (state.get("paused_label") or "nothing"), ""]
    lines += steps_section(cell, steps)
    lines += collections_section(cell)
    return "\n".join(lines).rstrip("\n") + "\n"


def main(arguments: list[str] | None = None) -> int:
    """Run the command line; return the exit status."""
    parser = argparse.ArgumentParser(description=(__doc__ or "").split("\n")[0])
    parser.add_argument("cell", type=pathlib.Path)
    options = parser.parse_args(arguments)
    report = options.cell / "report.md"
    report.write_text(build_report(options.cell), encoding="utf-8")
    print(report)
    return 0


if __name__ == "__main__":
    sys.exit(main())
