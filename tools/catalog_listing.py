"""Parse the catalog listing into its tables, for the drift checkers.

The listing is `internal/catalog/catalog.txt`, the file the binary embeds and `--list` prints:
a `# <heading>` line opens each table, and the lines below it are `agent|glob`, a bare glob
(in the exclusion and secret tables, `volume|glob` for one matched only in a Docker volume),
`agent|VARIABLE|replaces` or `scope|glob`. Glob semantics follow the catalog's own: catalog
globs do not cross `/`, exclusion and secret globs do, and `[...]` classes work everywhere.
Only the standard library is used, so every checker can import this module as it stands.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field

# The heading above each table of the listing, less the leading `# `. Only a comment equal to
# one, case and spacing included, switches the table, so a vendor comment inside a table that
# starts with the same words stays a comment.
SECTION_HEADINGS = {
    "home artifacts (agent|glob relative to home)": "home",
    "project artifacts (relative to each discovered project)": "project",
    "excluded unless --full": "exclude",
    "credential files (flagged secret, skipped with --no-secrets)": "secret",
    "docker volumes (agent|volume name glob)": "docker",
    "relocation variables (agent|VARIABLE|home-relative path it replaces)": "relocation",
    "startup files searched for them (scope|glob; scope home, system or project)": "startup",
}

# The prefix of an exclusion or secret line matched only below a Docker volume's `_data`.
VOLUME_PREFIX = "volume|"

# The ASCII white space a blank listing line may hold.
ASCII_SPACE = " \t\r\f\v"


@dataclass
class Entry:
    """One `agent|glob` line of the catalog, with each path segment as a regex."""

    agent: str
    glob: str
    segments: tuple[str, ...]
    regexes: tuple[re.Pattern[str], ...]


@dataclass(frozen=True)
class RelocationVariable:
    """One `agent|VARIABLE|replaces` line: an environment variable that moves an agent's state."""

    agent: str
    variable: str
    replaces: str


@dataclass(frozen=True)
class StartupFile:
    """One `scope|glob` line: a file that sets an environment, in its scope."""

    scope: str
    glob: str


@dataclass
class Catalog:
    """The catalog's tables: home and project entries, exclusions, secrets, volumes and hints.

    `excludes` and `secrets` hold the globs matched below every base, `volume_excludes` and
    `volume_secrets` those of the `volume|glob` lines, without the prefix.
    """

    entries: list[Entry] = field(default_factory=list)
    project_entries: list[Entry] = field(default_factory=list)
    excludes: list[str] = field(default_factory=list)
    secrets: list[str] = field(default_factory=list)
    volume_excludes: list[str] = field(default_factory=list)
    volume_secrets: list[str] = field(default_factory=list)
    docker_volumes: list[Entry] = field(default_factory=list)
    relocation_vars: list[RelocationVariable] = field(default_factory=list)
    startup_files: list[StartupFile] = field(default_factory=list)
    text: str = ""

    def project_agent_for(self, relative: str) -> str:
        """Return the agent of the project entry that claims a path, or `""`.

        The most specific matching entry wins (`.pi/approved-plan.md` before `.pi`), then the
        first in the listing, which is kept sorted by agent and then glob, the order the
        collector puts the table in, so this attributes a file as the collector does.
        """
        parts = [part for part in relative.replace("\\", "/").split("/") if part]
        return _claiming_agent(self.project_entries, parts)

    def claims_home_path(self, relative: str, windows: bool = False) -> str:
        """Return the agent of the home entry that claims a path, or `""`.

        An entry claims the path when each of its segment regexes matches the path's segment at
        the same place, ASCII case ignored; the most specific entry wins. With `windows`, the path
        is split on backslashes.
        """
        separator = "\\" if windows else "/"
        parts = [part for part in relative.split(separator) if part]
        return _claiming_agent(self.entries, parts)


def _claiming_agent(entries: list[Entry], parts: list[str]) -> str:
    """Return the agent of the deepest entry whose segments match the path's first parts."""
    best: Entry | None = None
    for entry in entries:
        depth = len(entry.segments)
        if depth > len(parts) or (best is not None and depth <= len(best.segments)):
            continue
        pairs = zip(entry.regexes, parts[:depth], strict=True)
        if all(pattern.fullmatch(part) for pattern, part in pairs):
            best = entry
    return best.agent if best is not None else ""


def glob_segment_to_regex(segment: str) -> re.Pattern[str]:
    """Convert one glob path segment to a regex.

    `*` and `?` never match `/`; a `[...]` class passes through, with `!` meaning negation as
    in sh; an unclosed `[` is a literal bracket. ASCII case is ignored, as the program ignores
    it.
    """
    pieces: list[str] = []
    index = 0
    while index < len(segment):
        character = segment[index]
        if character == "*":
            pieces.append("[^/]*")
        elif character == "?":
            pieces.append("[^/]")
        elif character == "[" and segment.find("]", index + 1) != -1:
            close = segment.find("]", index + 1)
            body = segment[index + 1 : close]
            if body.startswith("!"):
                body = "^" + body[1:]
            pieces.append("[" + body.replace("\\", "\\\\") + "]")
            index = close
        else:
            pieces.append(re.escape(character))
        index += 1
    return re.compile("".join(pieces), re.IGNORECASE | re.ASCII)


def _make_entry(agent: str, glob: str) -> Entry:
    """Return the `Entry` of one `agent|glob` line."""
    segments = tuple(segment for segment in glob.split("/") if segment)
    regexes = tuple(glob_segment_to_regex(segment) for segment in segments)
    return Entry(agent, glob, segments, regexes)


def _split_lines(text: str) -> list[str]:
    """Split text at LF or CRLF only, without the terminators or an empty final line."""
    lines = text.split("\n")
    if lines and lines[-1] == "":
        lines.pop()
    return [line[:-1] if line.endswith("\r") else line for line in lines]


def parse(text: str) -> Catalog:
    """Parse the text of the catalog listing into its tables."""
    catalog = Catalog(text=text)
    section = "home"
    for line in _split_lines(text):
        if not line.strip(ASCII_SPACE):
            continue
        if line.startswith("#"):
            section = SECTION_HEADINGS.get(line.removeprefix("# "), section)
            continue
        _add_line(catalog, section, line)
    return catalog


def _add_line(catalog: Catalog, section: str, line: str) -> None:
    """Add one listing line to the table `section` of `catalog`."""
    agent, _, rest = line.partition("|")
    if section == "home":
        catalog.entries.append(_make_entry(agent, rest))
    elif section == "project":
        catalog.project_entries.append(_make_entry(agent, rest))
    elif section in ("exclude", "secret"):
        glob = line.removeprefix(VOLUME_PREFIX)
        volume = glob != line
        tables = {
            ("exclude", False): catalog.excludes,
            ("exclude", True): catalog.volume_excludes,
            ("secret", False): catalog.secrets,
            ("secret", True): catalog.volume_secrets,
        }
        tables[section, volume].append(glob)
    elif section == "docker":
        catalog.docker_volumes.append(_make_entry(agent, rest))
    elif section == "relocation":
        variable, _, replaces = rest.partition("|")
        catalog.relocation_vars.append(RelocationVariable(agent, variable, replaces))
    elif section == "startup":
        catalog.startup_files.append(StartupFile(agent, rest))
