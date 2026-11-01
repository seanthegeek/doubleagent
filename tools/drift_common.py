"""Share the finding, the baseline and the readers of the drift checkers.

The drift checkers (`tools/check_<name>.py`) compare the research documents with the catalog
tables, the smoke tests, the parsers and the analyzer fixtures. Each finding not in the baseline
fails, so new drift fails CI while the drift recorded in the baseline expires agent by agent.

- A finding's baseline key is `<checker>|<kind>|<detail>`, its detail's whitespace runs
  collapsed to one space; the key is a line of `tools/drift-baseline/<agent>.txt`,
  `unattributed.txt` when the finding names no agent (`-`); one key per line, sorted, no comments.
- A checker reads only the baseline lines whose first field is its own name.
- A finding whose agent and key are not a baseline line is new, and fails.
- A baseline line of the checker that no finding produced is stale, and fails with
  `stale: remove "<key>" from tools/drift-baseline/<agent>.txt`.
- A baseline line with fewer than three fields belongs to no checker; every checker fails on it.
- `--update-baseline` rewrites the checker's own lines of every agent file from the run's
  findings, deletes a file left empty, keeps every other checker's lines, and exits 0; it drops
  a malformed line with `<file>:<n>: dropped malformed baseline line: <text>` on stderr.

A checker defines `run(root)`, yielding `Finding`s for the repository at `root`, and ends with
`sys.exit(main_with_baseline("check_example", run))`; `tools/tests/example_checker.py` is the
worked example. A checker run as `python3 tools/check_<name>.py` has `tools/` on its path, so
it imports this module, which imports `check_doc_links`, `check_research_structure` and, with
`analyzer/` added to the path, `doubleagent.catalog`, all standard library only.
"""

from __future__ import annotations

import argparse
import ast
import functools
import importlib
import pathlib
import re
import sys
from collections.abc import Callable, Iterable, Iterator
from dataclasses import dataclass, field
from types import ModuleType
from typing import TYPE_CHECKING

import check_doc_links
import check_research_structure

if TYPE_CHECKING:
    from analyzer.doubleagent.catalog import Catalog

ROOT = pathlib.Path(__file__).resolve().parent.parent
ANALYZER = ROOT / "analyzer"
BASELINE = pathlib.PurePosixPath("tools/drift-baseline")
# The stem of the baseline file holding the findings of no agent, `-`.
UNATTRIBUTED = "unattributed"
CATALOG_TEXT = pathlib.PurePosixPath("analyzer/doubleagent/catalog.txt")
AGENTS_PAGE = pathlib.PurePosixPath("docs/agents.md")
RESEARCH_INDEX = pathlib.PurePosixPath("research/README.md")
PARSERS = pathlib.PurePosixPath("analyzer/doubleagent/parsers")
# Files under research/ that are not one catalog agent's document.
NOT_AGENT_DOCUMENTS = frozenset({"README.md", "frameworks.md", "shared.md"})
TABLE_NAMES = ("home", "project", "exclude", "secret", "docker")
# The parser class attributes `agents()` resolves; see `ParserClass`.
PARSER_ATTRIBUTES = (
    "agent",
    "name",
    "reads_agents",
    "records_sync",
    "records_relay",
    "records_origin",
    "has_secondary",
)

CHECKER_NAME = re.compile(r"^check_[a-z0-9_]+$")
KIND = re.compile(r"^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$")
AGENT_NAME = re.compile(r"^(?:-|[a-z0-9][a-z0-9._-]*)$")
AGENTS_TABLE_HEADER = re.compile(r"^\|\s*Agent\s*\|\s*Status\s*\|")
TABLE_SEPARATOR = re.compile(r"^\|[\s:|-]+\|$")
SURVIVAL_TABLE_HEADER = re.compile(r"^\|\s*Agent\s*\|\s*Transcripts live in\s*\|")
FIRST_CODE_SPAN = re.compile(r"`([^`]+)`")
# Characters a negated glob class may give a sample path, in order of preference.
SAMPLE_CHARACTERS = "xabcdefghijklmnopqrstuvwyz0123456789"


def catalog_module() -> ModuleType:
    """Return `doubleagent.catalog`, putting `analyzer/` on the import path first."""
    if str(ANALYZER) not in sys.path:
        sys.path.insert(0, str(ANALYZER))
    return importlib.import_module("doubleagent.catalog")


def collapse(text: str) -> str:
    """Return `text` with each whitespace run made one space and the ends trimmed."""
    return " ".join(text.split())


# --- Findings and the baseline ------------------------------------------------------------


@dataclass(frozen=True)
class Finding:
    """One disagreement a checker found.

    `agent` is a catalog agent name, `shared`, `project` or `-`; `kind` a kebab-case token fixed
    per rule; `detail` the string that keeps the finding stable across unrelated edits (a glob,
    a key, a literal, a file name), never a line number or a sentence. `path` is relative to
    the repository root and `line` is 0 when unknown; `message` is shown, never keyed.
    """

    agent: str
    kind: str
    detail: str
    path: str
    line: int = 0
    message: str = ""

    def __post_init__(self) -> None:
        """Reject an agent, kind or detail that cannot form a baseline line."""
        if not AGENT_NAME.match(self.agent):
            raise ValueError("finding agent %r is not a file name" % self.agent)
        if not KIND.match(self.kind):
            raise ValueError("finding kind %r is not kebab-case" % self.kind)
        if not collapse(self.detail):
            raise ValueError("finding of kind %r has an empty detail" % self.kind)

    def key(self, checker: str) -> str:
        """Return the baseline key, `<checker>|<kind>|<detail>`."""
        return "%s|%s|%s" % (checker, self.kind, collapse(self.detail))

    def render(self) -> str:
        """Return the output line, `<path>:<line>: <kind> [<agent>]: <detail>[ - <message>]`."""
        location = "%s:%d" % (self.path, self.line) if self.line else self.path
        text = "%s: %s [%s]: %s" % (location, self.kind, self.agent, collapse(self.detail))
        return text + (" - %s" % self.message if self.message else "")


@dataclass
class BaselineLines:
    """The baseline lines one checker owns, and the lines no checker owns.

    `own` maps (agent, key) to the line number of the key in `<agent>.txt`; `malformed` holds
    one `<file>:<line>: malformed baseline line: <text>` per line with fewer than three fields.
    """

    own: dict[tuple[str, str], int] = field(default_factory=dict)
    malformed: list[str] = field(default_factory=list)


def baseline_file(agent: str) -> str:
    """Return the baseline file of an agent relative to the repository root."""
    return "%s/%s.txt" % (BASELINE, UNATTRIBUTED if agent == "-" else agent)


def file_agent(stem: str) -> str:
    """Return the agent a baseline file's stem stands for: `-` for `UNATTRIBUTED`."""
    return "-" if stem == UNATTRIBUTED else stem


def line_owner(line: str) -> str:
    """Return the checker a baseline line belongs to: its first field, or "" when malformed."""
    fields = line.split("|", 2)
    return fields[0] if len(fields) == 3 else ""


def load_baseline(root: pathlib.Path, checker: str) -> BaselineLines:
    """Return the lines of `checker` in every baseline file under `root`, and the malformed ones."""
    found = BaselineLines()
    directory = root / BASELINE
    if not directory.is_dir():
        return found
    for path in sorted(directory.glob("*.txt")):
        for number, line in enumerate(path.read_text(encoding="utf-8").split("\n"), 1):
            line = line.rstrip("\r")
            if not line:
                continue
            owner = line_owner(line)
            if not owner:
                agent = file_agent(path.stem)
                found.malformed.append(
                    "%s:%d: malformed baseline line: %s" % (baseline_file(agent), number, line)
                )
            elif owner == checker:
                found.own.setdefault((file_agent(path.stem), line), number)
    return found


def compare(
    checker: str, findings: Iterable[Finding], baseline: BaselineLines
) -> tuple[list[Finding], list[str]]:
    """Return the new findings, in run order, and one stale line message per unused baseline key."""
    produced: set[tuple[str, str]] = set()
    new: list[Finding] = []
    for finding in findings:
        pair = (finding.agent, finding.key(checker))
        produced.add(pair)
        if pair not in baseline.own:
            new.append(finding)
    stale = [
        '%s:%d: stale: remove "%s" from %s'
        % (baseline_file(agent), number, key, baseline_file(agent))
        for (agent, key), number in sorted(baseline.own.items())
        if (agent, key) not in produced
    ]
    return new, stale


def update_baseline(root: pathlib.Path, checker: str, findings: Iterable[Finding]) -> int:
    """Rewrite the lines of `checker` in every baseline file from `findings`.

    Every other checker's lines stay and a malformed line, which no checker owns, is dropped
    with a message on stderr; a file left empty is deleted; the directory is created only when
    a line is written. Returns the number of lines `checker` now owns.
    """
    wanted: dict[str, set[str]] = {}
    for finding in findings:
        wanted.setdefault(finding.agent, set()).add(finding.key(checker))
    directory = root / BASELINE
    files = directory.glob("*.txt") if directory.is_dir() else []
    present = {file_agent(path.stem) for path in files}
    for agent in sorted(present | set(wanted)):
        path = root / baseline_file(agent)
        old = path.read_text(encoding="utf-8") if path.is_file() else ""
        kept: set[str] = set()
        for number, raw in enumerate(old.split("\n"), 1):
            line = raw.rstrip("\r")
            owner = line_owner(line) if line.strip() else checker
            if not owner:
                message = "%s:%d: dropped malformed baseline line: %s"
                print(message % (baseline_file(agent), number, line), file=sys.stderr)
            elif owner != checker:
                kept.add(line)
        lines = sorted(kept | wanted.get(agent, set()))
        text = "".join(line + "\n" for line in lines)
        if not lines:
            path.unlink(missing_ok=True)
        elif text != old:
            directory.mkdir(parents=True, exist_ok=True)
            path.write_text(text, encoding="utf-8")
    return sum(len(keys) for keys in wanted.values())


def main_with_baseline(
    checker: str,
    run: Callable[[pathlib.Path], Iterable[Finding]],
    arguments: list[str] | None = None,
    root: pathlib.Path | None = None,
    description: str = "",
) -> int:
    """Run a checker against the baseline and return its exit status.

    Args:
        checker: The checker's file stem, `check_<name>`, the first field of its baseline lines.
        run: Called with the repository root; yields the checker's findings.
        arguments: The command-line arguments, by default `sys.argv[1:]`.
        root: The repository root, by default this checkout's; a test passes a temporary one.
        description: The `--help` summary, usually the first line of the checker's docstring.
    """
    if not CHECKER_NAME.match(checker):
        raise ValueError("checker name %r is not check_<name>" % checker)
    parser = argparse.ArgumentParser(prog=checker, description=description or None)
    parser.add_argument(
        "--update-baseline",
        action="store_true",
        help="rewrite this checker's lines under %s from this run's findings" % BASELINE,
    )
    options = parser.parse_args(arguments)
    root = ROOT if root is None else root
    findings = list(run(root))
    if options.update_baseline:
        count = update_baseline(root, checker, findings)
        print("%s: baseline holds %d line(s)" % (checker, count), file=sys.stderr)
        return 0
    baseline = load_baseline(root, checker)
    new, stale = compare(checker, findings, baseline)
    for line in [finding.render() for finding in new] + stale + baseline.malformed:
        print(line)
    print(
        "%s: %d finding(s), %d new, %d stale baseline line(s), %d malformed"
        % (checker, len(findings), len(new), len(stale), len(baseline.malformed)),
        file=sys.stderr,
    )
    return 1 if new or stale or baseline.malformed else 0


# --- Research documents ---------------------------------------------------------------------


@dataclass(frozen=True)
class CodeSpan:
    """An inline code span: its text with whitespace runs collapsed, and its first line."""

    text: str
    line: int


@dataclass(frozen=True)
class Fence:
    """A fenced code block: the info string's first word, the opening line, its inner lines."""

    language: str
    line: int
    lines: tuple[tuple[int, str], ...]


@dataclass(frozen=True)
class Section:
    """The lines from one heading to the next heading of any level.

    `key` is the heading path below the H1 joined by `/` (`Parsing/Parser plan/Fixtures`), ""
    for the H1 and intro, with ` (2)` appended to a repeated path; `titles` holds its parts.
    `start` is the heading's line and `end` the section's last. `text` is the section with each
    fenced line blanked, so `line_of(offset)` gives an offset's line number; `prose`
    holds its non-blank lines, the heading's included, with their numbers.
    """

    key: str
    titles: tuple[str, ...]
    level: int
    start: int
    end: int
    text: str
    prose: tuple[tuple[int, str], ...]
    code_spans: tuple[CodeSpan, ...]
    fences: tuple[Fence, ...]

    def line_of(self, offset: int) -> int:
        """Return the line number of an offset into `text`."""
        return self.start + self.text.count("\n", 0, offset)


def fenced_blocks(text: str) -> list[Fence]:
    """Return the fenced code blocks of a Markdown text, opened and closed as `strip_fences` does."""
    blocks: list[Fence] = []
    opener = ""
    language = ""
    start = 0
    inner: list[tuple[int, str]] = []
    for number, line in enumerate(text.split("\n"), 1):
        match = check_doc_links.FENCE.match(line)
        if opener:
            marker = match.group(1) if match else ""
            closing = marker[:1] == opener[:1] and len(marker) >= len(opener)
            if closing and not line.strip().lstrip(opener[0]):
                blocks.append(Fence(language, start, tuple(inner)))
                opener = ""
            else:
                inner.append((number, line))
        elif match:
            opener, start, inner = match.group(1), number, []
            info = line[match.end() :].split()
            language = info[0] if info else ""
    if opener:
        blocks.append(Fence(language, start, tuple(inner)))
    return blocks


def _section_starts(stripped: str) -> list[tuple[int, int, tuple[str, ...]]]:
    """Return (line, level, heading path) of the intro and of each heading level 2 and deeper."""
    starts: list[tuple[int, int, tuple[str, ...]]] = [(1, 1, ())]
    stack: dict[int, str] = {}
    for number, level, title in check_research_structure.headings(stripped):
        if level == 1:
            stack = {}
            starts.append((number, 1, ()))
            continue
        stack = {depth: name for depth, name in stack.items() if depth < level}
        stack[level] = title
        starts.append((number, level, tuple(stack[depth] for depth in sorted(stack))))
    if len(starts) > 1 and starts[1][0] == 1:
        starts.pop(0)
    return starts


def research_sections(path: pathlib.Path) -> dict[str, Section]:
    """Split a Markdown document into its sections, keyed by heading path, in document order.

    Headings and code spans are read outside fences (`check_doc_links.strip_fences`); a code
    span's text has its whitespace runs collapsed, so a span wrapped across lines matches.
    """
    text = path.read_text(encoding="utf-8")
    stripped_lines = check_doc_links.strip_fences(text).split("\n")
    blocks = fenced_blocks(text)
    starts = _section_starts("\n".join(stripped_lines))
    sections: dict[str, Section] = {}
    for index, (start, level, titles) in enumerate(starts):
        end = starts[index + 1][0] - 1 if index + 1 < len(starts) else len(stripped_lines)
        body = "\n".join(stripped_lines[start - 1 : end])
        spans = tuple(
            CodeSpan(collapse(match.group(2)), start + body.count("\n", 0, match.start()))
            for match in check_research_structure.CODE_SPAN.finditer(body)
        )
        prose = tuple(
            (start + offset, line) for offset, line in enumerate(body.split("\n")) if line.strip()
        )
        key = base = "/".join(titles)
        repeat = 1
        while key in sections:
            repeat += 1
            key = "%s (%d)" % (base, repeat)
        fences = tuple(block for block in blocks if start <= block.line <= end)
        sections[key] = Section(key, titles, level, start, end, body, prose, spans, fences)
    return sections


def within(sections: dict[str, Section], *titles: str) -> list[Section]:
    """Return the sections whose heading path begins with `titles`, in document order.

    `within(sections, "Collection")` is the Collection part, its heading's own lines included;
    `within(sections, "Parsing", "Parser plan")` the Parser plan and its subsections.
    """
    return [section for section in sections.values() if section.titles[: len(titles)] == titles]


# --- Catalog tables -------------------------------------------------------------------------


@dataclass(frozen=True)
class TableLine:
    """One line of a catalog table.

    `table` is one of `TABLE_NAMES`; `agent` is "" in the `exclude` and `secret` tables, whose
    line is the glob itself. `comment` holds the last run of comment lines above the line in its
    table, less their `# `, with other table lines allowed between; the table heading is not one.
    """

    table: str
    agent: str
    glob: str
    line: int
    comment: tuple[str, ...]


@dataclass
class Tables:
    """The five catalog tables, each a list in file order, and the parsed `Catalog`."""

    catalog: Catalog
    home: list[TableLine] = field(default_factory=list)
    project: list[TableLine] = field(default_factory=list)
    exclude: list[TableLine] = field(default_factory=list)
    secret: list[TableLine] = field(default_factory=list)
    docker: list[TableLine] = field(default_factory=list)

    def table(self, name: str) -> list[TableLine]:
        """Return the table named `name`, one of `TABLE_NAMES`."""
        if name not in TABLE_NAMES:
            raise KeyError(name)
        return getattr(self, name)

    def lines(self) -> list[TableLine]:
        """Return every table line, table by table in `TABLE_NAMES` order."""
        return [line for name in TABLE_NAMES for line in self.table(name)]


def tables(path: pathlib.Path | None = None) -> Tables:
    """Parse a catalog listing, by default `analyzer/doubleagent/catalog.txt`, into its tables.

    A comment equal to a heading of `catalog.SECTION_HEADINGS` switches the table, as
    `catalog.parse` does; other comments and blank lines belong to no table.
    """
    catalog = catalog_module()
    text = (path or ROOT / CATALOG_TEXT).read_text(encoding="utf-8")
    found = Tables(catalog=catalog.parse(text))
    table = "home"
    comment: list[str] = []
    after_line = False
    for number, raw in enumerate(text.split("\n"), 1):
        line = raw.rstrip("\r")
        if not line.strip():
            continue
        if line.startswith("#"):
            heading = catalog.SECTION_HEADINGS.get(line.removeprefix("# "))
            if heading is not None:
                table, comment, after_line = heading, [], False
                continue
            if after_line:
                comment, after_line = [], False
            comment.append(line.removeprefix("#").removeprefix(" "))
            continue
        agent, glob = ("", line) if table in ("exclude", "secret") else line.split("|", 1)
        found.table(table).append(TableLine(table, agent, glob, number, tuple(comment)))
        after_line = True
    return found


@functools.cache
def _bundled_catalog() -> Catalog:
    """Return the parsed bundled catalog, read once."""
    return tables().catalog


def sample_path(glob: str, empty_stars: bool = False) -> str:
    """Return a path the glob matches: `*` and `?` as `x`, a `[...]` class as its first member.

    With `empty_stars`, a `*` sharing its segment with other characters matches nothing
    instead (`.openclaw*` gives `.openclaw`). A negated class `[!...]` gives the first character
    of `SAMPLE_CHARACTERS` it does not exclude; a `[` with no closing `]` stays itself, as
    `glob_segment_to_regex` reads it.
    """
    out: list[str] = []
    index = 0
    while index < len(glob):
        character = glob[index]
        close = glob.find("]", index + 1) if character == "[" else -1
        if character == "*" and empty_stars and not _whole_segment(glob, index):
            pass
        elif character in "*?":
            out.append("x")
        elif close > index + 1:
            body = glob[index + 1 : close]
            if body.startswith("!"):
                excluded = re.compile("[%s]" % body[1:].replace("\\", "\\\\"))
                allowed = (char for char in SAMPLE_CHARACTERS if not excluded.match(char))
                out.append(next(allowed, "x"))
            else:
                out.append(body[0])
            index = close
        else:
            out.append(character)
        index += 1
    return "".join(out)


def _whole_segment(glob: str, index: int) -> bool:
    """Return whether the character at `index` is a path segment of its own."""
    before = glob[index - 1] if index else "/"
    after = glob[index + 1] if index + 1 < len(glob) else "/"
    return before == "/" and after == "/"


# Leading forms of a code span that name the home, and what they stand for under it.
HOME_PREFIXES = (
    ("~/", ""),
    ("~\\", ""),
    ("$HOME/", ""),
    ("${HOME}/", ""),
    ("%USERPROFILE%\\", ""),
    ("%APPDATA%\\", "AppData/Roaming/"),
    ("%LOCALAPPDATA%\\", "AppData/Local/"),
    ("$XDG_CONFIG_HOME/", ".config/"),
    ("$XDG_DATA_HOME/", ".local/share/"),
    ("$XDG_STATE_HOME/", ".local/state/"),
    ("$XDG_CACHE_HOME/", ".cache/"),
    ("./", ""),
)


def normalise_span(text: str) -> str:
    r"""Return a code span as a home-relative path, the form catalog globs take.

    A leading home form of `HOME_PREFIXES` (`~/`, `$HOME/`, `./`) is dropped and a Windows or
    XDG variable becomes the directory it names (`%APPDATA%\` is `AppData/Roaming/`);
    backslashes become slashes and one trailing `/` is dropped.
    """
    for prefix, replacement in HOME_PREFIXES:
        if text.startswith(prefix):
            text = replacement + text[len(prefix) :]
            break
    text = text.replace("\\", "/")
    return text[:-1] if text.endswith("/") and len(text) > 1 else text


@functools.cache
def segment_regex(glob: str) -> re.Pattern[str]:
    """Return a regex for a catalog glob whose `*`, `?` and classes never cross `/`."""
    catalog = catalog_module()
    parts = [catalog.glob_segment_to_regex(part).pattern for part in glob.split("/")]
    return re.compile("/".join(parts))


@functools.cache
def crossing_regex(glob: str) -> re.Pattern[str]:
    """Return a regex for an exclusion or secret glob, whose `*` and `?` cross `/` as in sh."""
    out: list[str] = []
    index = 0
    while index < len(glob):
        character = glob[index]
        close = glob.find("]", index + 1) if character == "[" else -1
        if character == "*":
            out.append(".*")
        elif character == "?":
            out.append(".")
        elif close > index + 1:
            body = glob[index + 1 : close]
            body = "^" + body[1:] if body.startswith("!") else body
            out.append("[%s]" % body.replace("\\", "\\\\"))
            index = close
        else:
            out.append(re.escape(character))
        index += 1
    return re.compile("".join(out))


def names_glob(glob: str, spans: Iterable[str], deeper: bool = False) -> bool:
    """Return whether a normalised code span names a glob.

    A span names it when it equals the glob or is a concrete path the glob matches segment by
    segment (`glob_segment_to_regex`), so `.claude-work/projects` names `.claude-*/projects`
    and `a0_usr` names `*a0_usr`. A glob ending in `/*` is also named by its directory
    (`mcp-tokens` names `mcp-tokens/*`). With `deeper`, a path below a match names it too
    (`.local/share/claude/versions` names `.local/share/claude`); without, it does not.
    """
    spans = list(spans)
    pattern = segment_regex(glob)
    if any(span == glob or pattern.fullmatch(span) for span in spans):
        return True
    if glob.endswith("/*") and names_glob(glob[:-2], spans):
        return True
    if not deeper:
        return False
    depth = glob.count("/") + 1
    return any(
        span.count("/") >= depth and pattern.fullmatch("/".join(span.split("/")[:depth]))
        for span in spans
    )


def star_remainders(glob: str, catalog: Catalog) -> list[tuple[str, str]]:
    """Return (agent, remainder) for each home entry a glob opening `*/` reaches.

    Only the leading star crosses `/`: the glob reaches an entry when the entry's sample path,
    alone or with a tail of the glob's segments appended, ends in segments the rest of the glob
    matches one by one; the entry must match at least the glob's first segment after the star,
    and the shortest such tail is the remainder below it: `*/Claude*/Cache` reaches
    `Library/Application Support/Claude` with the remainder `Cache`, while `*/Other/Cache` does
    not, nor does `*/User/x/*/y` reach `.e/User/x/a/b` by a middle star spanning `a/b`.
    """
    if not glob.startswith("*/"):
        return []
    pattern = re.compile("(?:.*/)?" + segment_regex(glob[2:]).pattern)
    parts = glob.split("/")
    tails = [""] + ["/".join(parts[start:]) for start in range(len(parts) - 1, 1, -1)]
    found: list[tuple[str, str]] = []
    for entry in catalog.entries:
        sample = sample_path(entry.glob, empty_stars=True)
        for tail in tails:
            path = sample + "/" + sample_path(tail, True) if tail else sample
            if pattern.fullmatch(path):
                found.append((entry.agent, tail))
                break
    return found


def is_volume_section(section: Section) -> bool:
    """Return whether a section's heading or first paragraph holds the word "volume"."""
    lines = section.text.split("\n")
    paragraph: list[str] = []
    for line in lines[1:]:
        if line.strip():
            paragraph.append(line)
        elif paragraph:
            break
    return "volume" in (lines[0] + " " + " ".join(paragraph)).lower()


@functools.cache
def collection_spans_by_depth(
    path: pathlib.Path, volume_only: bool = False
) -> dict[int, tuple[str, ...]]:
    """Return a document's normalised Collection code spans by their count of `/`, read once.

    With `volume_only`, only the spans of its Per-user storage volume subsections
    (`is_volume_section`). A missing document has none. A glob's segments never cross `/`, so
    only spans of its own depth, or one less for a glob ending in `/*`, can name it
    (`names_glob`).
    """
    if not path.is_file():
        return {}
    sections = research_sections(path)
    if volume_only:
        chosen = [
            section
            for section in within(sections, "Collection", "Per-user storage")
            if is_volume_section(section)
        ]
    else:
        chosen = within(sections, "Collection")
    found: dict[int, list[str]] = {}
    for section in chosen:
        for span in section.code_spans:
            text = normalise_span(span.text)
            found.setdefault(text.count("/"), []).append(text)
    return {depth: tuple(spans) for depth, spans in found.items()}


def document_names_glob(path: pathlib.Path, glob: str, volume_only: bool = False) -> bool:
    """Return whether a code span of a document's Collection part names a glob (`names_glob`).

    With `volume_only`, only a span of a Per-user storage volume subsection counts.
    """
    by_depth = collection_spans_by_depth(path, volume_only)
    depth = glob.count("/")
    return names_glob(glob, by_depth.get(depth, ()) + by_depth.get(depth - 1, ()))


def volume_section_names_glob(path: pathlib.Path, glob: str) -> bool:
    """Return whether a document's Per-user storage volume subsections name a volume glob.

    The one test both volume ownership (`glob_owners`) and `volume-glob-unnamed` use. It is the
    strict form, `names_glob` without `deeper`: a span describing an agent's own layout below
    a path (`profiles/<name>/...`) does not name the bare glob above it (`profiles/*`), so it
    never makes that agent an owner of another agent's glob.
    """
    return document_names_glob(path, glob, volume_only=True)


def home_claim(glob: str, catalog: Catalog) -> str:
    """Return the agent of the home entry claiming a sample path of a glob, or "".

    The claim is `Catalog.claims_home_path`, so the most specific entry wins; the sample with
    `empty_stars` is tried before the one with every `*` as `x`.
    """
    for sample in (sample_path(glob, empty_stars=True), sample_path(glob)):
        owner = catalog.claims_home_path(sample)
        if owner:
            return owner
    return ""


def project_claim(glob: str, catalog: Catalog) -> str:
    """Return the agent of the project entry claiming a sample path of a glob, or "".

    The claim is `Catalog.project_agent_for`, so the most specific entry wins; the sample with
    `empty_stars` is tried before the one with every `*` as `x`: `.git/worktrees/*/index` is
    `project`'s through the entry `.git/worktrees`, while `tmp/playwright` is no one's.
    """
    for sample in (sample_path(glob, empty_stars=True), sample_path(glob)):
        owner = catalog.project_agent_for(sample)
        if owner:
            return owner
    return ""


def closest_agents(reached: list[tuple[str, str]]) -> list[str]:
    """Return the agents whose (agent, remainder) pairs have the fewest remainder segments, sorted.

    An empty remainder has none; no pair gives no agent.
    """
    depths = [(rest.count("/") + 1 if rest else 0, agent) for agent, rest in reached]
    fewest = min((depth for depth, _ in depths), default=-1)
    return sorted({agent for depth, agent in depths if depth == fewest})


def closest_agent(reached: list[tuple[str, str]]) -> str:
    """Return the one agent `closest_agents` gives, or `-` when it gives none or several."""
    closest = closest_agents(reached)
    return closest[0] if len(closest) == 1 else "-"


def glob_owners(
    glob: str, catalog: Catalog | None = None, root: pathlib.Path | None = None
) -> list[str]:
    """Return the agents owning a table glob, by the first step that names any, or none.

    1. The home, then the project entry claiming a sample path (`home_claim`, `project_claim`).
    2. For a glob opening `*/`, the agent of the closest home entry it reaches
       (`star_remainders`), when no other agent's is as close.
    3. For any other glob, volume-relative and so applied in every volume, each `DOCKER_VOLUMES`
       agent whose volume subsections name it (`volume_section_names_glob`), sorted by name;
       when none's do, the one agent whose Collection part names it (`volume_agents_naming`),
       and no owner when several do. `catalog` defaults to the bundled one, `root` to this checkout.
    """
    catalog = catalog or _bundled_catalog()
    owner = home_claim(glob, catalog) or project_claim(glob, catalog)
    if owner:
        return [owner]
    if glob.startswith("*/"):
        closest = closest_agent(star_remainders(glob, catalog))
        return [] if closest == "-" else [closest]
    in_volume = volume_agents_naming(glob, catalog, root, volume_only=True)
    if in_volume:
        return sorted(in_volume)
    naming = volume_agents_naming(glob, catalog, root)
    return naming if len(naming) == 1 else []


def attribute_glob(
    glob: str, catalog: Catalog | None = None, root: pathlib.Path | None = None
) -> str:
    """Return the agent a finding about a table glob goes to: its first owner, or `-`.

    The owners are `glob_owners`; a volume-relative glob several agents share goes to the
    first of them by name, so its finding lands in one baseline file.
    """
    owners = glob_owners(glob, catalog, root)
    return owners[0] if owners else "-"


def volume_agents_naming(
    glob: str, catalog: Catalog, root: pathlib.Path | None = None, volume_only: bool = False
) -> list[str]:
    """Return the `DOCKER_VOLUMES` agents whose documents under `root` name a glob, in order.

    A document names it when a code span of its Collection part does (`document_names_glob`),
    with `volume_only` one of its Per-user storage volume subsections.
    """
    research = (ROOT if root is None else root) / "research"
    volume_agents = dict.fromkeys(entry.agent for entry in catalog.docker_volumes)
    return [
        agent
        for agent in volume_agents
        if (volume_section_names_glob if volume_only else document_names_glob)(
            research / ("%s.md" % agent), glob
        )
    ]


# --- Agents ---------------------------------------------------------------------------------


@dataclass(frozen=True)
class AgentRow:
    """One row of the table in `docs/agents.md`: its four cells, stripped, and its line."""

    agent: str
    status: str
    validated: str
    notes: str
    line: int


@dataclass(frozen=True)
class ParserClass:
    """A class `parsers.ALL` instantiates, with the attributes an instance of it has.

    The `PARSER_ATTRIBUTES` fields are read with `ast`, never by importing: each base's class
    body, the class's own, then the nearest `__init__`'s `self.<name> = ...` lines.
    `unresolved` names an attribute whose expression `_ParserPackage.evaluate` cannot reduce;
    it keeps its inherited value. `modules` is the class's module then each base's;
    `imported_modules` every parsers-package module its module imports from (`from .x import`),
    `base.py` included.
    """

    agent: str
    name: str
    reads_agents: tuple[str, ...]
    records_sync: bool
    records_relay: bool
    records_origin: bool
    has_secondary: bool
    class_name: str
    module: pathlib.Path
    line: int
    bases: tuple[str, ...]
    modules: tuple[pathlib.Path, ...]
    imported_modules: tuple[pathlib.Path, ...]
    unresolved: tuple[str, ...]


@dataclass
class Agents:
    """The agents each source knows, each a dict keyed by agent name in source order.

    `catalog` holds each agent's home, project and docker table lines; `documents` each
    `research/<agent>.md` but `NOT_AGENT_DOCUMENTS`; `rows` the agents-page rows; `parsers`
    the `parsers.ALL` classes by their resolved `agent`.
    """

    catalog: dict[str, list[TableLine]]
    documents: dict[str, pathlib.Path]
    rows: dict[str, AgentRow]
    parsers: dict[str, list[ParserClass]]


def agents_page_rows(path: pathlib.Path) -> dict[str, AgentRow]:
    """Return the rows of the first `| Agent | Status |` table of a page, keyed by agent.

    The agent is the first cell's first code span, or the cell's text when it has none; the
    notes cell is the rest of the row, so a `|` inside it stays. A repeated agent keeps its
    first row.
    """
    rows: dict[str, AgentRow] = {}
    in_table = False
    for number, raw in enumerate(path.read_text(encoding="utf-8").split("\n"), 1):
        line = raw.strip()
        if not in_table:
            in_table = bool(AGENTS_TABLE_HEADER.match(line))
            continue
        if not line.startswith("|"):
            break
        if TABLE_SEPARATOR.match(line):
            continue
        cells = [cell.strip() for cell in line.strip("|").split("|", 3)]
        cells += [""] * (4 - len(cells))
        span = FIRST_CODE_SPAN.search(cells[0])
        agent = span.group(1) if span else cells[0]
        rows.setdefault(agent, AgentRow(agent, cells[1], cells[2], cells[3], number))
    return rows


def table_rows(text: str, header: re.Pattern[str]) -> Iterator[tuple[int, list[str]]]:
    """Yield (line offset, cells) of each row of the tables `header` opens in `text`.

    A line `header` matches, once stripped, yields (offset, []) and opens a table; each later
    line starting `|` is a row, split at every `|`, its cells stripped, the separator row left
    out; any other line closes the table.
    """
    in_table = False
    for offset, line in enumerate(text.split("\n")):
        stripped = line.strip()
        if header.match(stripped):
            in_table = True
            yield offset, []
            continue
        if not in_table or not stripped.startswith("|"):
            in_table = False
            continue
        if not TABLE_SEPARATOR.match(stripped):
            yield offset, [cell.strip() for cell in stripped.strip("|").split("|")]


@dataclass(frozen=True)
class SurvivalRow:
    """One row of the survival table of `research/README.md`: its agent, cells and line."""

    agent: str
    cells: tuple[str, ...]
    line: int


def survival_rows(root: pathlib.Path | None = None) -> dict[str, SurvivalRow]:
    """Return the rows of the survival table, keyed by agent, the first row of an agent kept.

    The table is the one whose header starts `| Agent | Transcripts live in |`; a row's agent
    is its first cell's first code span, or the cell's text when it has none. An absent index
    gives no rows.
    """
    path = (ROOT if root is None else root) / RESEARCH_INDEX
    if not path.is_file():
        return {}
    rows: dict[str, SurvivalRow] = {}
    for offset, cells in table_rows(path.read_text(encoding="utf-8"), SURVIVAL_TABLE_HEADER):
        if not cells:
            continue
        span = FIRST_CODE_SPAN.search(cells[0])
        agent = span.group(1) if span else cells[0]
        rows.setdefault(agent, SurvivalRow(agent, tuple(cells), offset + 1))
    return rows


def agent_documents(root: pathlib.Path | None = None) -> dict[str, pathlib.Path]:
    """Return each `research/<agent>.md` but `NOT_AGENT_DOCUMENTS`, keyed by agent, sorted."""
    research = (ROOT if root is None else root) / "research"
    return {
        path.stem: path
        for path in sorted(research.glob("*.md"))
        if path.name not in NOT_AGENT_DOCUMENTS
    }


def agents(root: pathlib.Path | None = None) -> Agents:
    """Return the agents of the catalog, the research documents, the agents page and the parsers."""
    root = ROOT if root is None else root
    catalog: dict[str, list[TableLine]] = {}
    found = tables(root / CATALOG_TEXT)
    for line in found.home + found.project + found.docker:
        catalog.setdefault(line.agent, []).append(line)
    documents = agent_documents(root)
    parsers: dict[str, list[ParserClass]] = {}
    for parser in parser_classes(root / PARSERS):
        parsers.setdefault(parser.agent, []).append(parser)
    return Agents(catalog, documents, agents_page_rows(root / AGENTS_PAGE), parsers)


# A value `_ParserPackage.evaluate` cannot reduce to a literal.
UNRESOLVED = object()


class _ParserPackage:
    """The modules of the parsers package, each parsed with `ast` once."""

    def __init__(self) -> None:
        """Start with no module parsed."""
        self.trees: dict[pathlib.Path, ast.Module] = {}

    def tree(self, path: pathlib.Path) -> ast.Module:
        """Return the parsed module at `path`."""
        if path not in self.trees:
            self.trees[path] = ast.parse(path.read_text(encoding="utf-8"), str(path))
        return self.trees[path]

    def imported(self, path: pathlib.Path, node: ast.ImportFrom) -> pathlib.Path | None:
        """Return the file a relative `from ... import` in the module at `path` names, or None."""
        if node.level == 0:
            return None
        package = path.parent
        for _ in range(node.level - 1):
            package = package.parent
        target = package.joinpath(*node.module.split(".")) if node.module else package
        for candidate in (target.with_suffix(".py"), target / "__init__.py"):
            if candidate.is_file():
                return candidate
        return None

    def binding(self, path: pathlib.Path, name: str) -> tuple[pathlib.Path, ast.AST] | None:
        """Return where a module-level name is bound, (module, value or ClassDef), or None.

        The last top-level class, assignment or relative import of the name wins, as at run time.
        """
        found: tuple[pathlib.Path, ast.AST] | None = None
        for statement in self.tree(path).body:
            if isinstance(statement, ast.ClassDef) and statement.name == name:
                found = (path, statement)
            elif isinstance(statement, ast.Assign) and _binds(statement.targets, name):
                found = (path, statement.value)
            elif isinstance(statement, ast.AnnAssign) and _binds([statement.target], name):
                found = (path, statement.value) if statement.value is not None else found
            elif isinstance(statement, ast.ImportFrom):
                for alias in statement.names:
                    module = self.imported(path, statement)
                    if (alias.asname or alias.name) == name:
                        found = self.binding(module, alias.name) if module else None
        return found

    def evaluate(self, path: pathlib.Path, node: ast.AST, attributes: dict[str, object]) -> object:
        """Reduce an expression of the module at `path` to a value, or `UNRESOLVED`.

        It reduces a literal, a tuple or list of reducible items, a name bound at module level
        or imported to a reducible value, `self.<name>` from `attributes`, `not` and a single
        `==` or `!=`; anything else is `UNRESOLVED`.
        """
        if isinstance(node, ast.Constant):
            return node.value
        if isinstance(node, (ast.Tuple, ast.List)):
            items = [self.evaluate(path, item, attributes) for item in node.elts]
            return UNRESOLVED if any(item is UNRESOLVED for item in items) else tuple(items)
        if isinstance(node, ast.Name):
            bound = self.binding(path, node.id)
            if bound is None or isinstance(bound[1], ast.ClassDef):
                return UNRESOLVED
            return self.evaluate(bound[0], bound[1], {})
        if isinstance(node, ast.Attribute) and isinstance(node.value, ast.Name):
            return attributes.get(node.attr, UNRESOLVED) if node.value.id == "self" else UNRESOLVED
        if isinstance(node, ast.UnaryOp) and isinstance(node.op, ast.Not):
            operand = self.evaluate(path, node.operand, attributes)
            return UNRESOLVED if operand is UNRESOLVED else not operand
        if isinstance(node, ast.Compare) and len(node.ops) == 1:
            left = self.evaluate(path, node.left, attributes)
            right = self.evaluate(path, node.comparators[0], attributes)
            if UNRESOLVED in (left, right) or not isinstance(node.ops[0], (ast.Eq, ast.NotEq)):
                return UNRESOLVED
            return (left == right) == isinstance(node.ops[0], ast.Eq)
        return UNRESOLVED

    def ancestry(
        self, path: pathlib.Path, node: ast.ClassDef
    ) -> list[tuple[pathlib.Path, ast.ClassDef]]:
        """Return the class then its bases, depth first and left to right, each once."""
        order = [(path, node)]
        for base in node.bases:
            bound = self.binding(path, base.id) if isinstance(base, ast.Name) else None
            if bound is not None and isinstance(bound[1], ast.ClassDef):
                for item in self.ancestry(bound[0], bound[1]):
                    if all(item[1] is not known[1] for known in order):
                        order.append(item)
        return order


def _binds(targets: list[ast.expr], name: str) -> bool:
    """Return whether one of the assignment targets is the plain name `name`."""
    return any(isinstance(target, ast.Name) and target.id == name for target in targets)


def _assignment(statement: ast.stmt) -> tuple[ast.expr, ast.expr] | None:
    """Return (target, value) of a one-target assignment with a value, or None."""
    if isinstance(statement, ast.Assign) and len(statement.targets) == 1:
        return statement.targets[0], statement.value
    if isinstance(statement, ast.AnnAssign) and statement.value is not None:
        return statement.target, statement.value
    return None


def _class_attributes(
    package: _ParserPackage, ancestry: list[tuple[pathlib.Path, ast.ClassDef]]
) -> tuple[dict[str, object], list[str]]:
    """Return the `PARSER_ATTRIBUTES` an instance has, and the names left unresolved."""
    assignments: list[tuple[pathlib.Path, str, ast.expr]] = []
    for path, node in reversed(ancestry):
        for target, value in filter(None, map(_assignment, node.body)):
            if isinstance(target, ast.Name):
                assignments.append((path, target.id, value))
    initializers = [
        (path, statement)
        for path, node in ancestry
        for statement in node.body
        if isinstance(statement, ast.FunctionDef) and statement.name == "__init__"
    ]
    for path, function in initializers[:1]:
        for target, value in filter(None, map(_assignment, function.body)):
            owner = target.value if isinstance(target, ast.Attribute) else None
            is_self = isinstance(owner, ast.Name) and owner.id == "self"
            if is_self and isinstance(target, ast.Attribute):
                assignments.append((path, target.attr, value))
    values: dict[str, object] = {}
    unresolved: list[str] = []
    for path, name, expression in assignments:
        if name not in PARSER_ATTRIBUTES:
            continue
        value = package.evaluate(path, expression, values)
        if value is UNRESOLVED:
            unresolved.append(name)
        else:
            values[name] = value
    return values, unresolved


def parser_classes(directory: pathlib.Path) -> list[ParserClass]:
    """Return the classes the `ALL` list of the parsers package at `directory` instantiates."""
    package = _ParserPackage()
    registry = directory / "__init__.py"
    calls: list[str] = []
    for target, value in filter(None, map(_assignment, package.tree(registry).body)):
        if isinstance(target, ast.Name) and target.id == "ALL" and isinstance(value, ast.List):
            calls = [
                item.func.id
                for item in value.elts
                if isinstance(item, ast.Call) and isinstance(item.func, ast.Name)
            ]
    found: list[ParserClass] = []
    for class_name in calls:
        bound = package.binding(registry, class_name)
        if bound is not None and isinstance(bound[1], ast.ClassDef):
            found.append(_parser_class(package, directory, bound[0], bound[1]))
    return found


def _parser_class(
    package: _ParserPackage, directory: pathlib.Path, path: pathlib.Path, node: ast.ClassDef
) -> ParserClass:
    """Return the `ParserClass` of the class `node` defined in the module at `path`."""
    ancestry = package.ancestry(path, node)
    values, unresolved = _class_attributes(package, ancestry)
    modules: dict[pathlib.Path, None] = dict.fromkeys(module for module, _ in ancestry)
    imported: dict[pathlib.Path, None] = {}
    for statement in package.tree(path).body:
        if not isinstance(statement, ast.ImportFrom) or statement.level != 1:
            continue
        # `from . import x` names the module x itself, not a name inside the package.
        names = [] if statement.module else [alias.name for alias in statement.names]
        modules_named = [directory / ("%s.py" % name) for name in names]
        for module in modules_named or [package.imported(path, statement)]:
            if module is not None and module.is_file() and module != path:
                imported.setdefault(module)
    return ParserClass(
        agent=str(values.get("agent", "")),
        name=str(values.get("name", "")),
        reads_agents=tuple(str(item) for item in _as_tuple(values.get("reads_agents", ()))),
        records_sync=bool(values.get("records_sync", False)),
        records_relay=bool(values.get("records_relay", False)),
        records_origin=bool(values.get("records_origin", False)),
        has_secondary=bool(values.get("has_secondary", False)),
        class_name=node.name,
        module=path,
        line=node.lineno,
        bases=tuple(base.name for _, base in ancestry[1:]),
        modules=tuple(modules),
        imported_modules=tuple(imported),
        unresolved=tuple(dict.fromkeys(unresolved)),
    )


def _as_tuple(value: object) -> tuple[object, ...]:
    """Return a tuple value as it is and anything else as a one-item tuple."""
    return value if isinstance(value, tuple) else (value,)
