#!/usr/bin/env python3
"""Diff two listings of a lab VM and group what one round step wrote.

    python3 -I classify.py [--redact] listings/03-sign-in.txt listings/04-question.txt

Reads the tab-separated lines lib/survey.sh and lib/survey.ps1 print, `<kind> <size> <mode>
<mtime> <path>`, and prints, grouped by the first two path components under the home (else under
the root):

  ADDED    paths only in the later listing
  REMOVED  paths only in the earlier one
  CHANGED  paths whose size, mode or modification time moved; a directory only when its mode did
  TOP      top-level names that appeared or went away (the `top` lines): a directory no survey root
           covers, to add to the agent's roots file
  STATE    credential-store items and process counts that changed

No file content is read. With --redact, UUIDs and runs of 32 or more hex digits in a path print as
`<uuid>` and `<hex>`, so a diff can be quoted where a session id must not appear. Standard library
only.
"""

from __future__ import annotations

import argparse
import dataclasses
import re
import sys
from collections import defaultdict
from collections.abc import Iterable

UUID_RE = re.compile(r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}")
HEX_RE = re.compile(r"(?<![0-9a-fA-F])[0-9a-fA-F]{32,}(?![0-9a-fA-F])")
HOME_RE = re.compile(r"^(?:[A-Za-z]:)?/(?:Users|home)/[^/]+/(.*)$")
STATE_KINDS = frozenset({"keychain", "process"})


@dataclasses.dataclass(frozen=True)
class Entry:
    """One path of a listing, as the survey printed it."""

    kind: str
    size: str
    mode: str
    mtime: str


@dataclasses.dataclass
class Listing:
    """A parsed listing: the entries by path, the top-level names and the state lines."""

    entries: dict[str, Entry] = dataclasses.field(default_factory=dict)
    tops: set[str] = dataclasses.field(default_factory=set)
    states: dict[tuple[str, str], str] = dataclasses.field(default_factory=dict)


@dataclasses.dataclass
class Difference:
    """What changed between two listings."""

    added: list[str]
    removed: list[str]
    changed: list[str]
    new_tops: list[str]
    gone_tops: list[str]
    states: list[tuple[str, str, str, str]]

    def is_empty(self) -> bool:
        """Return True when nothing changed."""
        return not (
            self.added
            or self.removed
            or self.changed
            or self.new_tops
            or self.gone_tops
            or self.states
        )


def parse_listing(lines: Iterable[str]) -> Listing:
    """Parse the lines of one listing; a line of another shape is skipped."""
    listing = Listing()
    for raw in lines:
        fields = raw.rstrip("\r\n").split("\t")
        if fields[0] == "top" and len(fields) == 6:
            listing.tops.add(fields[5])
        elif fields[0] in STATE_KINDS and len(fields) == 5:
            listing.states[(fields[0], fields[4])] = fields[1]
        elif len(fields) == 5:
            kind, size, mode, mtime, path = fields
            listing.entries[path] = Entry(kind, size, mode, mtime)
    return listing


def load_listing(path: str) -> Listing:
    """Read and parse one listing file."""
    with open(path, encoding="utf-8", errors="replace") as handle:
        return parse_listing(handle)


def compare(before: Listing, after: Listing) -> Difference:
    """Return what changed from `before` to `after`."""
    common = set(before.entries) & set(after.entries)
    changed = []
    for path in sorted(common):
        old, new = before.entries[path], after.entries[path]
        if new.kind == "dir" and old.kind == "dir":
            if old.mode != new.mode:
                changed.append(path)
        elif old != new:
            changed.append(path)
    states = [
        (key[0], key[1], before.states.get(key, "-"), after.states.get(key, "-"))
        for key in sorted(set(before.states) | set(after.states))
        if before.states.get(key) != after.states.get(key)
    ]
    return Difference(
        added=sorted(set(after.entries) - set(before.entries)),
        removed=sorted(set(before.entries) - set(after.entries)),
        changed=changed,
        new_tops=sorted(after.tops - before.tops),
        gone_tops=sorted(before.tops - after.tops),
        states=states,
    )


def group_key(path: str) -> str:
    """Return the first two path components under the home (`~/.gemini/tmp`), else the root's."""
    text = path.split(" -> ", 1)[0].replace("\\", "/")
    match = HOME_RE.match(text)
    if match:
        components = [part for part in match.group(1).split("/") if part]
        return "~/" + "/".join(components[:2])
    components = [part for part in text.split("/") if part]
    return "/" + "/".join(components[:2])


def redact(path: str) -> str:
    """Replace UUIDs and long hex runs in a path with their class."""
    return HEX_RE.sub("<hex>", UUID_RE.sub("<uuid>", path))


def human_size(size: str) -> str:
    """Return a byte count as B, KB, MB or GB; any other text unchanged."""
    if not size.isdigit():
        return size
    amount = float(size)
    for unit in ("B", "KB", "MB"):
        if amount < 1024:
            return "%d%s" % (amount, unit) if unit == "B" else "%.1f%s" % (amount, unit)
        amount /= 1024
    return "%.1fGB" % amount


def render_section(
    title: str, paths: list[str], table: dict[str, Entry], redacting: bool
) -> list[str]:
    """Return the lines of one ADDED, REMOVED or CHANGED section."""
    if not paths:
        return []
    groups: dict[str, list[str]] = defaultdict(list)
    for path in paths:
        groups[group_key(path)].append(path)
    lines = ["== %s: %d" % (title, len(paths))]
    for group in sorted(groups):
        members = groups[group]
        total = sum(int(table[path].size) for path in members if table[path].size.isdigit())
        lines.append("  %s  (%d entries, %s)" % (group, len(members), human_size(str(total))))
        for path in members:
            entry = table[path]
            shown = redact(path) if redacting else path
            lines.append(
                "    %-4s %9s %5s  %s  %s"
                % (entry.kind, human_size(entry.size), entry.mode, entry.mtime, shown)
            )
    return lines


def render(difference: Difference, before: Listing, after: Listing, redacting: bool) -> str:
    """Return the printable diff."""
    lines = render_section("ADDED", difference.added, after.entries, redacting)
    lines += render_section("REMOVED", difference.removed, before.entries, redacting)
    lines += render_section("CHANGED", difference.changed, after.entries, redacting)
    if difference.new_tops or difference.gone_tops:
        lines.append("== TOP: names no survey root covers; add a root if the agent wrote them")
        lines += [
            "    + %s" % (redact(path) if redacting else path) for path in difference.new_tops
        ]
        lines += [
            "    - %s" % (redact(path) if redacting else path) for path in difference.gone_tops
        ]
    if difference.states:
        lines.append("== STATE: %d" % len(difference.states))
        for kind, name, old, new in difference.states:
            lines.append("    %s %s: %s -> %s" % (kind, name, old, new))
    if difference.is_empty():
        lines.append("no change")
    return "\n".join(lines) + "\n"


def main(arguments: list[str] | None = None) -> int:
    """Run the command line; return the exit status."""
    parser = argparse.ArgumentParser(description=(__doc__ or "").split("\n")[0])
    parser.add_argument("--redact", action="store_true", help="print UUIDs and hex runs as classes")
    parser.add_argument("before")
    parser.add_argument("after")
    options = parser.parse_args(arguments)
    before = load_listing(options.before)
    after = load_listing(options.after)
    sys.stdout.write(render(compare(before, after), before, after, options.redact))
    return 0


if __name__ == "__main__":
    sys.exit(main())
