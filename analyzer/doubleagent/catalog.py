"""Load the artifact catalog the analyzer shares with the collectors.

`catalog.txt` is a verbatim copy of `collect-agent-artifacts.sh --list` and is
the analyzer's detection database: the same `agent|glob` table that tells the
collectors what to gather tells the analyzer what a loose directory contains.
`collectors/tests/catalog-sync.sh` fails when the copy drifts. Regenerate it
with:

    collectors/collect-agent-artifacts.sh --list > analyzer/doubleagent/catalog.txt

Glob semantics match the collectors: catalog globs do not cross `/`, exclusion
and secret globs do, `[...]` classes work everywhere.
"""

from __future__ import annotations

import os
import re
from collections.abc import Iterator
from dataclasses import dataclass, field
from pathlib import Path

from .textlines import ASCII_SPACE, split_lines

CATALOG_FILE = Path(__file__).with_name("catalog.txt")

# The headings `print_catalog` (collect-agent-artifacts.sh) and
# `Write-CatalogList` (Collect-AgentArtifacts.ps1) print above each table of
# `--list`, less the leading `# `. Only a comment equal to one, case and
# spacing included, switches the table, so a vendor comment inside a table
# that starts with the same words stays a comment.
SECTION_HEADINGS = {
    "home artifacts (agent|glob relative to home)": "home",
    "project artifacts (relative to each discovered project)": "project",
    "excluded unless --full": "exclude",
    "credential files (flagged secret, skipped with --no-secrets)": "secret",
    "docker volumes (agent|volume name glob)": "docker",
}


@dataclass
class Entry:
    """One `agent|glob` line of the catalog, with each path segment as a regex."""

    agent: str
    glob: str
    segments: tuple[str, ...]
    regexes: tuple[re.Pattern[str], ...]

    @property
    def first_segment(self) -> str:
        """The glob's first path segment."""
        return self.segments[0]


@dataclass
class Catalog:
    """The catalog's tables: home and project entries, exclusions, secrets and volumes."""

    entries: list[Entry] = field(default_factory=list)
    project_entries: list[Entry] = field(default_factory=list)
    excludes: list[str] = field(default_factory=list)
    secrets: list[str] = field(default_factory=list)
    docker_volumes: list[Entry] = field(default_factory=list)  # agent|volume name glob
    text: str = ""

    @property
    def agents(self) -> list[str]:
        """The agent names of the home entries, in catalog order."""
        seen: dict[str, None] = {}
        for entry in self.entries:
            seen.setdefault(entry.agent)
        return list(seen)

    def first_segment_regexes(self) -> list[re.Pattern[str]]:
        """Return one regex per distinct first segment of the home entries."""
        seen: dict[str, re.Pattern[str]] = {}
        for entry in self.entries:
            seen.setdefault(entry.first_segment, entry.regexes[0])
        return list(seen.values())

    def project_agent_for(self, rel: str) -> str:
        """Return the agent of the project entry that claims a path, or `""`.

        The most specific matching entry wins (`.pi/approved-plan.md` before
        `.pi`), then the first in catalog order, as the collectors attribute a
        file. A file the catalog leaves to no single agent (`AGENTS.md`) gives
        `project`.

        Args:
            rel: A path relative to a project directory.
        """
        parts = [part for part in rel.replace("\\", "/").split("/") if part]
        best: Entry | None = None
        for entry in self.project_entries:
            depth = len(entry.segments)
            if depth > len(parts) or (best is not None and depth <= len(best.segments)):
                continue
            if all(
                pattern.fullmatch(part)
                for pattern, part in zip(entry.regexes, parts[:depth], strict=True)
            ):
                best = entry
        return best.agent if best is not None else ""

    def claims_home_path(self, rel: str, windows: bool = False) -> str:
        """Return the agent of the home entry that claims a path, or `""`.

        An entry claims the path when each of its segment regexes matches the
        path's segment at the same place, so `.copilot` claims
        `.copilot/chats/<id>`; the most specific entry wins.

        Args:
            rel: A path relative to a home.
            windows: Split on backslashes and match without regard to case.
        """
        separator = "\\" if windows else "/"
        parts = [part for part in rel.split(separator) if part]
        flags = re.IGNORECASE if windows else 0
        best: Entry | None = None
        for entry in self.entries:
            depth = len(entry.segments)
            if depth > len(parts) or (best is not None and depth <= len(best.segments)):
                continue
            pairs = zip(entry.regexes, parts[:depth], strict=True)
            if all(re.fullmatch(pattern.pattern, part, flags) for pattern, part in pairs):
                best = entry
        return best.agent if best is not None else ""

    def nested_matches(self, agent_dir: Path, onerror=None) -> Iterator[tuple[Entry, Path]]:
        """Yield (entry, path) for the entries inside a directory their first segment names.

        This finds `.gemini/antigravity-cli` when a `.gemini` directory is
        given as the root.
        """
        listing_cache: dict[Path, list[str]] = {}
        for entry in self.entries:
            if len(entry.segments) < 2 or entry.regexes[0].fullmatch(agent_dir.name) is None:
                continue
            for hit in _expand(agent_dir, entry.regexes[1:], listing_cache, onerror):
                yield entry, hit

    def matches_in_home(self, home: Path, onerror=None) -> Iterator[tuple[Entry, Path]]:
        """Yield (entry, path) for every catalog glob that exists under a home.

        Expansion goes segment by segment through directory listings, so the
        cost follows the catalog, not the size of the tree. Symlinks are never
        followed. A directory that cannot be listed matches nothing and its
        OSError is passed to `onerror` once.
        """
        listing_cache: dict[Path, list[str]] = {}
        for entry in self.entries:
            for hit in _expand(home, entry.regexes, listing_cache, onerror):
                yield entry, hit


def glob_segment_to_regex(segment: str) -> re.Pattern[str]:
    """Convert one glob path segment to a regex.

    `*` and `?` never match `/`; a `[...]` class passes through, with `!`
    meaning negation as in sh.
    """
    out = []
    i = 0
    while i < len(segment):
        character = segment[i]
        if character == "*":
            out.append("[^/]*")
        elif character == "?":
            out.append("[^/]")
        elif character == "[":
            j = segment.find("]", i + 1)
            if j == -1:
                out.append(re.escape(character))
            else:
                body = segment[i + 1 : j]
                if body.startswith("!"):
                    body = "^" + body[1:]
                body = body.replace("\\", "\\\\")
                out.append("[" + body + "]")
                i = j
        else:
            out.append(re.escape(character))
        i += 1
    return re.compile("".join(out))


def _expand(
    base: Path,
    regexes: tuple[re.Pattern[str], ...],
    cache: dict[Path, list[str]],
    onerror=None,
) -> Iterator[Path]:
    if not regexes:
        yield base
        return
    names = cache.get(base)
    if names is None:
        try:
            names = os.listdir(base)
        except OSError as error:
            names = []
            if onerror is not None:
                onerror(error)
        cache[base] = names
    pattern = regexes[0]
    rest = regexes[1:]
    for name in names:
        if pattern.fullmatch(name) is None:
            continue
        child = base / name
        if rest:
            if child.is_dir() and not child.is_symlink():
                yield from _expand(child, rest, cache, onerror)
        else:
            yield child


def _make_entry(agent: str, glob: str) -> Entry:
    segments = tuple(segment for segment in glob.split("/") if segment)
    return Entry(
        agent, glob, segments, tuple(glob_segment_to_regex(segment) for segment in segments)
    )


def load(path: Path = CATALOG_FILE) -> Catalog:
    """Read and parse a catalog file, by default the bundled one."""
    text = path.read_text(encoding="utf-8")
    return parse(text)


def parse(text: str) -> Catalog:
    """Parse the text of `collect-agent-artifacts.sh --list` into its tables."""
    catalog = Catalog(text=text)
    section = "home"
    for line in split_lines(text):
        if not line.strip(ASCII_SPACE):
            continue
        if line.startswith("#"):
            section = SECTION_HEADINGS.get(line.removeprefix("# "), section)
            continue
        if section == "home":
            agent, _, glob = line.partition("|")
            catalog.entries.append(_make_entry(agent, glob))
        elif section == "project":
            agent, _, glob = line.partition("|")
            catalog.project_entries.append(_make_entry(agent, glob))
        elif section == "exclude":
            catalog.excludes.append(line)
        elif section == "docker":
            agent, _, glob = line.partition("|")
            catalog.docker_volumes.append(_make_entry(agent, glob))
        else:
            catalog.secrets.append(line)
    return catalog
