#!/usr/bin/env python3
"""Check the catalog tables against the Collection part of the research documents.

`research/<agent>.md` owns its agent's table lines and `research/shared.md` the `shared` and
`project` ones; an exclusion or secret glob's owners are `glob_owners`, its agent the first.

- `line-unnamed`: a table line no Collection code span of its owners names (`names_glob`), by
  its glob or its remainder below an owner's entry (`base_remainders`, `star_remainders`);
  its detail is `<table> <glob>`, as one glob may be in several tables.
- `credential-unflagged`: a file a Credentials section names (`credential_paths`) that no
  secret glob flags on its own base or a sibling base of one platform (`sibling_bases`: the same
  last segment, case kept where `case_distinguishes`), unless a Parsing-part Secrets code span
  names the path (`secrets_store`) or says it is not a credential (`denies_credential`).
- `secret-under-exclusion`: a secret glob each of whose sample paths an exclusion prunes.
- `exclusion-unreachable`, `secret-unreachable`: an exclusion or secret glob with no owner
  (`Repository.owners`, `glob_owners`): no entry reaches it, no volume document claims it.
- `volume-glob-unnamed`: a glob a docker-volume agent's document owns that no code span of its
  volume subsections (`is_volume_section`) names, once per such owner.
- `volume-twin-missing`: an exclusion below a directory a docker volume is `mounted at`
  (`mount_path`, `volume_home_remainders`) with no volume-relative twin (`volume_twin`).
- `case-overlap`: two agents' home entries whose first differing segment differs only by case,
  or whose first segments differ and can still match one path (`case_overlap`).
- `credential-wording`: a `WORDING_PHRASES` phrase in a Credentials section.
- `document-missing`, `catalog-missing`: a catalog agent with no research document, and back.
- `agents-row-missing`, `agents-row-orphan`: a catalog agent, `project` aside, with no
  `docs/agents.md` row, and a row naming no catalog agent.
- `parsing-fact-under-collection`: a Collection code span that is a row text (`is_row_text`)
  or a key the Record schema also names (`is_key_span`) and no path names (`path_names`),
  outside the project-path section.

Usage: tools/check_tables_research.py [--update-baseline]
"""

from __future__ import annotations

import functools
import pathlib
import re
import sys
from collections.abc import Iterable, Iterator
from dataclasses import dataclass
from typing import TYPE_CHECKING

import check_research_structure
import drift_common
from drift_common import (
    Finding,
    Section,
    TableLine,
    crossing_regex,
    is_volume_section,
    names_glob,
    normalise_span,
    segment_regex,
    star_remainders,
)

if TYPE_CHECKING:
    from analyzer.doubleagent.catalog import Catalog

CHECKER = "check_tables_research"
CATALOG_PATH = str(drift_common.CATALOG_TEXT)
SHARED_DOCUMENT = "shared.md"
# Table agents owned by research/shared.md, not by a document of their own name.
SHARED_AGENTS = ("shared", "project")
# Agents the agents page lists no row for, since `project` is not an agent.
ROWLESS_AGENTS = frozenset({"project"})
WORDING_PHRASES = ("not flagged", "collected unflagged", "may embed", "can embed", "left unflagged")
# Extensions that make a one-segment code span a file name rather than a key.
FILE_EXTENSIONS = frozenset(
    {
        *("json", "jsonl", "json5", "jsonc", "yaml", "yml", "toml", "ini", "cfg", "conf"),
        *("env", "db", "sqlite", "sqlite3", "vscdb", "txt", "md", "log", "key", "pem", "crt"),
        *("lock", "enc", "bak", "plist", "xml", "pb", "ldb", "sh", "ps1", "js", "mjs", "ts"),
        *("py", "html", "zip", "gz", "tar", "zst", "csv", "token", "cookies", "netrc"),
    }
)
URL = re.compile(r"^(?:[a-z][a-z0-9+.-]*://|www\.)", re.I)
# Characters a code span holding a path never has: assignments, calls, templates, quotes.
NOT_PATH_CHARACTERS = re.compile(r"[=(){}\"'|;,]|^\$|^<|:(?!\\)")
KEY = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*$")
# Phrases saying a file named before them in the sentence is not a credential.
NOT_A_CREDENTIAL = re.compile(
    r"\b(?:is\s+not\s+one(?!\s+of\b)|is\s+not\s+a\s+credential|not\s+a\s+credential\s+file"
    r"|(?:holds|has|contains|stores)\s+no\s+credentials?)\b",
    re.I,
)
# A clause ends at these marks; a denial further than CLAUSE_WORDS words away is about
# something else.
CLAUSE_BREAK = re.compile(r"[,;:]")
# A link's target, whose URL holds a colon that is no clause break.
LINK_TARGET = re.compile(r"\]\([^)]*\)")
CLAUSE_WORDS = 4
# The end of a sentence: a stop before whitespace, a blank line or the next list item.
SENTENCE_END = re.compile(r"[.!?](?=\s|$)|\n[ \t]*\n|\n[ \t]*(?:[-*+]|\d+[.)])[ \t]")
MOUNTED_AT = re.compile(r"\bmounted at\s+(/\S*)")
# The platform and the kinds of state of a home-relative directory, by its leading segments, so a
# credential under one form of a tool's directory is expected under its other platforms' forms.
# A tool's own directory is `home`; the first pattern that matches wins.
BASE_FORMS = (
    (re.compile(r"^\.config/"), "linux", {"config"}),
    (re.compile(r"^\.local/share/"), "linux", {"data"}),
    (re.compile(r"^\.local/state/"), "linux", {"state"}),
    (re.compile(r"^\.cache/"), "linux", {"cache"}),
    (re.compile(r"^\.var/app/[^/]+/config/"), "flatpak", {"config"}),
    (re.compile(r"^\.var/app/[^/]+/data/"), "flatpak", {"data"}),
    (re.compile(r"^Library/Caches/"), "macos", {"cache"}),
    (re.compile(r"^Library/Logs/"), "macos", {"logs"}),
    (re.compile(r"^Library/Application Support/"), "macos", {"home", "config", "data", "state"}),
    (re.compile(r"^AppData/Local/(?:Temp|Programs)/"), "windows", set()),
    (re.compile(r"^AppData/(?:Roaming|Local)/"), "windows", {"home", "config", "data", "cache"}),
    (re.compile(r"^\.[^/]+$"), "unix", {"home"}),
)
# A dot directory directly under the home is the tool's directory on Linux and macOS alike.
PLATFORMS_COVERED = {"unix": {"linux", "macos"}}


# --- Heuristics ------------------------------------------------------------------------------


def base_remainders(glob: str, bases: Iterable[TableLine]) -> list[str]:
    """Return the rest of a glob below each base of the agent whose segments it begins with.

    A base begins the glob when each of its segment regexes matches the glob's segment at the
    same place, sampled with empty stars, so `.hermes*/models` has the remainder `models`
    below `.hermes`; a glob equal in depth to a base has none.
    """
    parts = glob.split("/")
    samples = [drift_common.sample_path(part, empty_stars=True) for part in parts]
    found: list[str] = []
    for base in bases:
        depth = base.glob.count("/") + 1
        prefix = "/".join(samples[:depth])
        if depth < len(parts) and segment_regex(base.glob).fullmatch(prefix):
            found.append("/".join(parts[depth:]))
    return found


def is_path_span(text: str) -> bool:
    r"""Return whether a code span is a file or directory path rather than a key or a URL.

    A path holds `/` or `\`, or is one segment ending in an extension of `FILE_EXTENSIONS`,
    or a dot file (`.env`, `.netrc`); a URL, and a span holding `=`, a bracket, a quote, `|`,
    `;`, `,` or a colon other than a drive's, is not, nor one starting with `$` or `<`.
    """
    if not text or URL.match(text) or NOT_PATH_CHARACTERS.search(text):
        return False
    if "/" in text or "\\" in text:
        return True
    if " " in text:
        return False
    if re.fullmatch(r"\.[A-Za-z0-9_][\w.-]*", text):
        return True
    extension = text.rsplit(".", 1)[-1].lower() if "." in text else ""
    return extension in FILE_EXTENSIONS


def base_form(glob: str) -> tuple[str, set[str]]:
    """Return the platform and the kinds of state of a home-relative directory (`BASE_FORMS`).

    `.config/Claude` is (`linux`, {`config`}), `.hermes` (`unix`, {`home`}) and
    `AppData/Local/hermes` (`windows`, {`home`, ...}); a directory no pattern matches
    (`Claude`, `Documents/Claude`) has none, nor a Windows temporary or installed-program
    directory (`AppData/Local/Temp/claude`, `AppData/Local/Programs/muse`).
    """
    for pattern, platform, kinds in BASE_FORMS:
        if pattern.match(glob):
            return platform, set(kinds)
    return "", set()


def base_segment(glob: str, keep_case: bool) -> str:
    """Return a home entry's last segment with a leading `.` dropped, case-folded unless kept."""
    segment = glob.rsplit("/", 1)[-1].lstrip(".")
    return segment if keep_case else segment.casefold()


def case_distinguishes(entries: Iterable[TableLine]) -> bool:
    """Return whether case tells an agent's base directories apart.

    It does when two home entries on one platform (`base_form`) end in segments equal
    case-folded but not as written (`AppData/Local/hermes` beside `AppData/Roaming/Hermes`),
    the way the catalog separates surfaces; otherwise Windows capitalises what POSIX spells
    lower (`.ollama`, `AppData/Local/Ollama`) and siblings compare case-folded.
    """
    forms: dict[tuple[str, str], set[str]] = {}
    for entry in entries:
        platform = base_form(entry.glob)[0]
        if not platform:
            continue
        segment = base_segment(entry.glob, True)
        seen = forms.setdefault((platform, segment.casefold()), set())
        seen.add(segment)
        if len(seen) > 1:
            return True
    return False


def sibling_bases(
    base: TableLine, entries: Iterable[TableLine], named: Iterable[str]
) -> list[TableLine]:
    """Return the other platform forms of a base directory among an agent's home entries.

    A sibling ends in the base's last segment with a leading `.` dropped, compared as written
    when case tells the agent's directories apart (`case_distinguishes`) and case-folded
    otherwise; it is on a platform the base's does not cover (`PLATFORMS_COVERED`), shares a
    kind of state with the base (`base_form`), and is named by a span of the Per-user storage
    section (`names_glob`): `.config/Claude` and `Library/Application Support/Claude` are
    siblings, and `.ollama` and `AppData/Local/Ollama`; `.config/zed` and `.local/share/zed` are
    not, nor `.hermes` and `AppData/Roaming/Hermes` beside `AppData/Local/hermes`.
    """
    spans = list(named)
    entries = list(entries)
    keep_case = case_distinguishes(entries)
    last = base_segment(base.glob, keep_case)
    platform, kinds = base_form(base.glob)
    covered = PLATFORMS_COVERED.get(platform, {platform})
    found: list[TableLine] = []
    for entry in entries:
        if base_segment(entry.glob, keep_case) != last:
            continue
        other_platform, other_kinds = base_form(entry.glob)
        if covered & PLATFORMS_COVERED.get(other_platform, {other_platform}):
            continue
        if kinds & other_kinds and names_glob(entry.glob, spans):
            found.append(entry)
    return found


def is_file_span(text: str) -> bool:
    """Return whether a path span names files: its last segment a file name or a glob.

    The last segment ends in an extension of `FILE_EXTENSIONS` (`auth.json`), is a dot name
    (`.credentials.json`, `.env`) or holds `*` (`ide/*`); `.cline/data` names a directory.
    """
    last = text.replace("\\", "/").rstrip("/").rsplit("/", 1)[-1]
    if "*" in last or re.fullmatch(r"\.[A-Za-z0-9_][\w.-]*", last):
        return True
    return "." in last and last.rsplit(".", 1)[-1].lower() in FILE_EXTENSIONS


def excluded(path: str, exclusions: Iterable[str]) -> bool:
    """Return whether an exclusion glob matches a home-relative path or one of its ancestors."""
    parts = path.split("/")
    ancestors = ["/".join(parts[:depth]) for depth in range(1, len(parts) + 1)]
    return any(
        crossing_regex(glob).fullmatch(ancestor) for glob in exclusions for ancestor in ancestors
    )


def secret_samples(glob: str) -> list[str]:
    """Return the sample paths of a glob, with empty stars and with each star as `x`."""
    return list(
        dict.fromkeys([drift_common.sample_path(glob, True), drift_common.sample_path(glob)])
    )


def mount_path(comment: Iterable[str]) -> str:
    """Return the container path a docker volume comment says it is `mounted at`, or "".

    The comment's lines are joined, so a path wrapped onto the next line is found; trailing
    `.`, `,`, `;` and `)` are dropped.
    """
    match = MOUNTED_AT.search(" ".join(comment))
    return match.group(1).rstrip(".,;)") if match else ""


def volume_home_remainders(glob: str, mount: str) -> list[str]:
    """Return the parts of an exclusion below each directory a volume may be mounted as.

    A directory is a proper prefix of the glob whose last segment, sampled with empty stars,
    ends in the mount path's last segment: `.ollama/models` under `/root/.ollama` gives
    `models`, `agent-zero*/usr/x` under `/a0/usr` gives `x`, and `a/langflow/langflow/x` under
    `/app/langflow` gives `langflow/x` and `x`. A glob with no such directory gives none.
    """
    last = mount.rstrip("/").rsplit("/", 1)[-1]
    parts = glob.split("/")
    return [
        "/".join(parts[depth:])
        for depth in range(1, len(parts))
        if last and drift_common.sample_path(parts[depth - 1], empty_stars=True).endswith(last)
    ]


def is_volume_relative(glob: str, catalog: Catalog) -> bool:
    """Return whether an exclusion is volume-relative: no home entry claims it, no `*/` leads it."""
    return not glob.startswith("*/") and not drift_common.home_claim(glob, catalog)


def volume_twin(glob: str, remainders: Iterable[str], volume_exclusions: Iterable[str]) -> bool:
    """Return whether a volume-relative exclusion is the twin of a home exclusion.

    A twin prunes one of the home exclusion's remainders below a mount directory
    (`volume_home_remainders`), by equality or as its ancestor (`excluded`), or is a
    segment-aligned suffix of the home exclusion: `profile_pictures` twins
    `AppData/Local/langflow/langflow/Cache/profile_pictures`, `models` twins
    `.ollama/models/blobs`; `pictures` and `Cache/other` twin neither.
    """
    volume_exclusions = list(volume_exclusions)
    samples = [drift_common.sample_path(rest, empty_stars=True) for rest in remainders]
    if any(excluded(sample, volume_exclusions) for sample in samples):
        return True
    return any(glob.endswith("/" + twin) for twin in volume_exclusions)


def nested(outer: str, inner: str) -> bool:
    """Return whether one home glob lies inside the other's match, as a nested entry does.

    `inner` is nested in `outer` when it is deeper and each of `outer`'s segment regexes matches
    `inner`'s segment at the same place, sampled with empty stars, as
    `Catalog.nested_matches` finds `.gemini/antigravity-cli` under `.gemini`.
    """
    outer_parts, inner_parts = outer.split("/"), inner.split("/")
    if len(inner_parts) <= len(outer_parts):
        return False
    prefix = "/".join(
        drift_common.sample_path(part, empty_stars=True) for part in inner_parts[: len(outer_parts)]
    )
    return bool(segment_regex(outer).fullmatch(prefix))


def case_overlap(first: str, second: str) -> bool:
    """Return whether two home globs differ at a segment only by case, or by a prefix at the first.

    At the first segment that differs, an overlap is the two equal when case-folded
    (`.config/PearAI`, `.config/pearai`); when that is the first segment, also two globs that
    can match a common path (`common_path`: `.hermes*`, `.hermes_x`). No differing segment, a
    nested entry relation either way (`nested`), a prefix deeper down (`AppData/Local`,
    `AppData/LocalLow`) and disjoint globs (`.vscode*/argv.json`, `.vscode/cody.json`) are none.
    """
    if nested(first, second) or nested(second, first):
        return False
    pairs = zip(first.split("/"), second.split("/"), strict=False)
    for depth, (left, right) in enumerate(pairs):
        if left == right:
            continue
        if left.casefold() == right.casefold():
            return True
        return depth == 0 and common_path(first, second)
    return False


def common_path(first: str, second: str) -> bool:
    """Return whether two home globs can match one path, compared case-folded.

    They can when a sample path of one (`sample_path`, both star forms) is matched whole by the
    other's segment regexes, either way round, so globs of different depths never can:
    `.hermes*` and `.hermes_x` can, as `.alpha` and `.alpha*`; `.vscode` and `.vscode-server*`
    cannot, nor `.vscode*/argv.json` and `.vscode/cody.json`.
    """
    for one, other in ((first, second), (second, first)):
        pattern = segment_regex(other.casefold())
        for sample in (drift_common.sample_path(one, True), drift_common.sample_path(one)):
            if pattern.fullmatch(sample.casefold()):
                return True
    return False


def is_row_text(text: str) -> bool:
    """Return whether a code span is a timeline row text: it holds a space and ends with `:`."""
    return text.endswith(":") and " " in text


def is_key_span(text: str) -> bool:
    """Return whether a code span is a compound record key.

    A key is an identifier or a dotted path of them that is compound: it holds `_` or `.`, or
    a lower-case letter before a capital (`session_id`, `data.project`, `fullOutputPath`). A
    single word (`cwd`, `mode`), which may as well be a directory or a product name, is not;
    nor a file name whose last part is in `FILE_EXTENSIONS` (`state.db`), a dot file, a path or
    an all-capital constant (`API_KEY`).
    """
    if not KEY.match(text) or text.isupper():
        return False
    if "." in text and text.rsplit(".", 1)[-1].lower() in FILE_EXTENSIONS:
        return False
    return "_" in text or "." in text or bool(re.search(r"[a-z][A-Z]", text))


def path_names(spans: Iterable[str]) -> set[str]:
    """Return each segment of the path spans (`is_path_span`) and each segment's stem.

    `dev_data/0.2.0/chatFeedback.jsonl` gives `dev_data`, `0.2.0`, `chatFeedback.jsonl` and
    `chatFeedback`; a key span (`session_id`) gives nothing. A span among them names a file or
    directory, not a key.
    """
    names: set[str] = set()
    for span in spans:
        if not is_path_span(span):
            continue
        for segment in normalise_span(span).split("/"):
            names.update(name for name in (segment, segment.rsplit(".", 1)[0]) if name)
    return names


def plain_spans(section: Section) -> list[tuple[str, int, int]]:
    """Return a section's code spans outside link texts: (text, start offset, end offset).

    A span that is a link's text (`[`fb9e3c8`](https://...)`), a citation, is skipped, so
    `[`416-417`](...)` is not taken for a fact; the text has its whitespace runs collapsed.
    """
    found: list[tuple[str, int, int]] = []
    for match in check_research_structure.CODE_SPAN.finditer(section.text):
        start, end = match.start(), match.end()
        if section.text[start - 1 : start] == "[" and section.text[end : end + 2] == "](":
            continue
        found.append((drift_common.collapse(match.group(2)), start, end))
    return found


def denies_credential(section: Section, end: int) -> bool:
    """Return whether the sentence of a code span ending at `end` says it is not a credential.

    A `NOT_A_CREDENTIAL` phrase in the span's own clause, at most `CLAUSE_WORDS` words after
    it with no `,`, `;` or `:` and no other code span outside a link text (`plain_spans`)
    between, before the sentence's end (`SENTENCE_END`), denies it: "`.crush/crush.lock` is not
    one"; "`auth.json` is the token file; the rest holds no credentials" does not.
    """
    stop = SENTENCE_END.search(section.text, end)
    phrase = NOT_A_CREDENTIAL.search(section.text, end, stop.start() if stop else len(section.text))
    if phrase is None:
        return False
    between = LINK_TARGET.sub("]", section.text[end : phrase.start()])
    if CLAUSE_BREAK.search(between) or len(between.split()) > CLAUSE_WORDS:
        return False
    return not any(end <= start < phrase.start() for _, start, _ in plain_spans(section))


# --- The repository --------------------------------------------------------------------------


@dataclass
class Repository:
    """What every rule reads: the tables, the agents and each document's sections."""

    root: pathlib.Path
    tables: drift_common.Tables
    agents: drift_common.Agents
    sections: dict[str, dict[str, Section]]

    def document(self, agent: str) -> str:
        """Return the document owning an agent's lines, relative to the root."""
        return "research/%s" % (SHARED_DOCUMENT if agent in SHARED_AGENTS else agent + ".md")

    def collection(self, agent: str, *titles: str) -> list[Section]:
        """Return the Collection part of the document owning an agent, or a section of it."""
        sections = self.sections.get(self.document(agent), {})
        return drift_common.within(sections, "Collection", *titles)

    @functools.cached_property
    def collection_spans(self) -> dict[str, list[str]]:
        """Return each document's normalised Collection code spans, keyed by its path."""
        return {
            document: [
                normalise_span(span.text)
                for section in drift_common.within(sections, "Collection")
                for span in section.code_spans
            ]
            for document, sections in self.sections.items()
        }

    @functools.cached_property
    def glob_owners(self) -> dict[str, list[str]]:
        """Return the owners of each exclusion and secret glob (`glob_owners`), computed once."""
        catalog = self.tables.catalog
        return {
            line.glob: drift_common.glob_owners(line.glob, catalog, self.root)
            for line in self.tables.exclude + self.tables.secret
        }

    @functools.cached_property
    def glob_agents(self) -> dict[str, str]:
        """Return the agent of each exclusion and secret glob: its first owner, or `-`."""
        return {glob: owners[0] if owners else "-" for glob, owners in self.glob_owners.items()}

    def owners(self, line: TableLine) -> list[str]:
        """Return the agents whose documents may name a table line.

        That is the line's agent, or the glob's owners (`glob_owners`), several for a shared
        volume-relative glob; for an unowned glob opening `*/`, every agent whose entry it
        reaches closest (`closest_agents`). A glob nothing reaches has none.
        """
        if line.agent:
            return [line.agent]
        owners = self.glob_owners[line.glob]
        if owners:
            return owners
        reached = star_remainders(line.glob, self.tables.catalog)
        return drift_common.closest_agents(reached)

    def entries(self, agent: str) -> list[TableLine]:
        """Return an agent's home table lines."""
        return [line for line in self.tables.home if line.agent == agent]


def load(root: pathlib.Path) -> Repository:
    """Read the tables, the agents and the sections of every research document under `root`."""
    tables = drift_common.tables(root / drift_common.CATALOG_TEXT)
    agents = drift_common.agents(root)
    paths = list(agents.documents.values())
    shared = root / "research" / SHARED_DOCUMENT
    if shared.is_file():
        paths.append(shared)
    sections = {
        path.relative_to(root).as_posix(): drift_common.research_sections(path) for path in paths
    }
    return Repository(root, tables, agents, sections)


def line_agent(repository: Repository, line: TableLine) -> str:
    """Return a table line's agent, `attribute_glob` for an exclusion or secret glob."""
    return line.agent or repository.glob_agents[line.glob]


# --- Rules -----------------------------------------------------------------------------------


def unnamed_lines(repository: Repository) -> Iterator[Finding]:
    """Yield `line-unnamed` for each table line its owners' Collection parts never name.

    A line is checked against its owners' documents only (`Repository.owners`); a line with
    none is left to `exclusion-unreachable` and `secret-unreachable`. It also counts as named
    by its remainder below one of an owner's home entries (`base_remainders`), the form a
    document uses inside its own directory listing, or, opening `*/`, below an owner's entry it
    reaches (`star_remainders`). A home or project line is also named by a path below it
    (`names_glob` with `deeper`). The finding goes to `line_agent`, `-` for a `*/` glob several
    agents reach equally closely.
    """
    catalog = repository.tables.catalog
    for line in repository.tables.lines():
        owners = repository.owners(line)
        documents = [repository.document(owner) for owner in owners]
        if not owners or not all(document in repository.sections for document in documents):
            continue
        deeper = line.table in ("home", "project")
        reached = star_remainders(line.glob, catalog)
        if any(named_by(repository, line.glob, owner, deeper, reached) for owner in owners):
            continue
        agent = line_agent(repository, line)
        message = "the %s line is named in no Collection code span of %s" % (
            line.table,
            " or ".join(documents),
        )
        detail = "%s %s" % (line.table, line.glob)
        yield Finding(agent, "line-unnamed", detail, CATALOG_PATH, line.line, message)


def named_by(
    repository: Repository,
    glob: str,
    owner: str,
    deeper: bool,
    reached: list[tuple[str, str]],
) -> bool:
    """Return whether an owner's Collection spans name a glob, whole or by a remainder.

    The remainders are the glob's below each of the owner's home entries (`base_remainders`)
    and below each of its entries the glob reaches from a leading `*/` (`reached`).
    """
    spans = repository.collection_spans[repository.document(owner)]
    if names_glob(glob, spans, deeper):
        return True
    rests = base_remainders(glob, repository.entries(owner))
    rests += [rest for agent, rest in reached if agent == owner]
    return any(names_glob(rest, spans) for rest in rests if rest)


def secrets_store(path: str, secrets_spans: Iterable[str]) -> bool:
    """Return whether a credential path names a store the Secrets section lists for redaction.

    A store mixing credentials with evidence is collected unflagged by design, its credential
    fields named in the Parsing part's Secrets section: a normalised Secrets span whose sample
    path (`sample_path` with empty stars) equals the path exempts it.
    """
    return any(drift_common.sample_path(text, empty_stars=True) == path for text in secrets_spans)


def unflagged_credentials(repository: Repository) -> Iterator[Finding]:
    """Yield `credential-unflagged` for each credential file no secret glob flags on a platform."""
    secrets = [crossing_regex(line.glob) for line in repository.tables.secret]
    for agent, path in repository.agents.documents.items():
        entries = repository.entries(agent)
        sections = repository.sections[repository.document(agent)]
        secrets_spans = [
            normalise_span(span.text)
            for section in drift_common.within(sections, "Parsing", "Secrets")
            for span in section.code_spans
        ]
        named = [
            normalise_span(span.text)
            for section in repository.collection(agent, "Per-user storage")
            for span in section.code_spans
        ]
        relative = path.relative_to(repository.root).as_posix()
        for section in repository.collection(agent, "Credentials"):
            for match in check_research_structure.CODE_SPAN.finditer(section.text):
                if denies_credential(section, match.end()):
                    continue
                span = drift_common.collapse(match.group(2))
                line = section.line_of(match.start())
                for group in credential_paths(normalise_span(span), entries, named):
                    if any(secret.fullmatch(tested) for tested in group for secret in secrets):
                        continue
                    if any(secrets_store(tested, secrets_spans) for tested in group):
                        continue
                    message = "no secret glob flags %s" % " or ".join(group)
                    yield Finding(agent, "credential-unflagged", group[0], relative, line, message)


def credential_paths(span: str, entries: list[TableLine], named: list[str]) -> list[list[str]]:
    """Return the home-relative paths a credential span names, one group per platform.

    The span must be a path naming files (`is_path_span`, `is_file_span`) strictly below one of
    the agent's home entries, or a file entry itself; the deepest such entry is its base. The
    first group is the span itself; its remainder below the base, under each sibling base
    (`sibling_bases`), gives one more group per platform, any one path of which a secret glob
    must flag. A sample replaces each wildcard.
    """
    if not is_path_span(span) or not is_file_span(span):
        return []
    sample = drift_common.sample_path(span, empty_stars=True)
    parts = sample.split("/")
    reaching = [
        entry
        for entry in entries
        if entry.glob.count("/") < len(parts)
        and segment_regex(entry.glob).fullmatch("/".join(parts[: entry.glob.count("/") + 1]))
    ]
    if not reaching:
        return []
    base = max(reaching, key=lambda entry: entry.glob.count("/"))
    remainder = "/".join(parts[base.glob.count("/") + 1 :])
    if not remainder:
        last = parts[-1]
        has_extension = "." in last[1:] and last.rsplit(".", 1)[-1].lower() in FILE_EXTENSIONS
        return [[sample]] if has_extension else []
    platforms: dict[str, list[str]] = {}
    for sibling in sibling_bases(base, entries, named):
        prefix = drift_common.sample_path(sibling.glob, empty_stars=True)
        platforms.setdefault(base_form(sibling.glob)[0], []).append(prefix + "/" + remainder)
    return [[sample], *platforms.values()]


def secrets_under_exclusions(repository: Repository) -> Iterator[Finding]:
    """Yield `secret-under-exclusion` for each secret glob an exclusion always prunes."""
    exclusions = [line.glob for line in repository.tables.exclude]
    for line in repository.tables.secret:
        if all(excluded(sample, exclusions) for sample in secret_samples(line.glob)):
            agent = line_agent(repository, line)
            message = "an exclusion matches every path of this secret glob, so it is never flagged"
            yield Finding(
                agent, "secret-under-exclusion", line.glob, CATALOG_PATH, line.line, message
            )


def unreachable_globs(repository: Repository) -> Iterator[Finding]:
    """Yield `exclusion-unreachable` or `secret-unreachable` for each glob with no owner.

    A volume-relative glob several volume subsections name is owned by each of those agents;
    one no volume subsection names and several Collection parts do has none, and the message
    names those agents.
    """
    unreached = "no home or project entry reaches it and no docker-volume agent's document names it"
    catalog = repository.tables.catalog
    for line in repository.tables.exclude + repository.tables.secret:
        if repository.owners(line):
            continue
        naming = drift_common.volume_agents_naming(line.glob, catalog, repository.root)
        several = "the documents of %s all name it, so none owns it" % ", ".join(naming)
        message = several if naming else unreached
        kind = "exclusion-unreachable" if line.table == "exclude" else "secret-unreachable"
        yield Finding("-", kind, line.glob, CATALOG_PATH, line.line, message)


def unnamed_volume_globs(repository: Repository) -> Iterator[Finding]:
    """Yield `volume-glob-unnamed` for each volume-relative glob its volume section never names.

    The glob is one docker-volume agents' documents own (`glob_owners`, neither a home nor a
    project entry reaching it); each owner's Per-user storage subsections naming one of its
    volumes (`is_volume_section`) must name it in a code span (`volume_section_names_glob`), and each
    owner whose do not gets its own finding.
    """
    catalog = repository.tables.catalog
    for line in repository.tables.exclude + repository.tables.secret:
        if not is_volume_relative(line.glob, catalog):
            continue
        if drift_common.project_claim(line.glob, catalog):
            continue
        for agent in repository.glob_owners[line.glob]:
            yield from _unnamed_volume_glob(repository, line, agent)


def _unnamed_volume_glob(repository: Repository, line: TableLine, agent: str) -> Iterator[Finding]:
    """Yield `volume-glob-unnamed` for one owner unless its volume subsections name the glob."""
    volume_globs = drift_common.agent_volume_globs(repository.tables.catalog, agent)
    sections = [
        section
        for section in repository.collection(agent, "Per-user storage")
        if is_volume_section(section, volume_globs)
    ]
    if drift_common.volume_section_names_glob(
        repository.root / repository.document(agent), line.glob, volume_globs
    ):
        return
    names = ", ".join("`%s`" % glob for glob in volume_globs)
    looked = ", ".join(section.key for section in sections) or (
        "none, no subsection names a volume matching %s" % names
    )
    message = "no Per-user storage volume subsection of %s names it (looked in: %s)" % (
        repository.document(agent),
        looked,
    )
    detail = "%s %s" % (line.table, line.glob)
    yield Finding(agent, "volume-glob-unnamed", detail, CATALOG_PATH, line.line, message)


def missing_volume_twins(repository: Repository) -> Iterator[Finding]:
    """Yield `volume-twin-missing` for each home exclusion of a mounted volume with no twin."""
    catalog = repository.tables.catalog
    volume_exclusions = [
        line.glob for line in repository.tables.exclude if is_volume_relative(line.glob, catalog)
    ]
    mounts = {
        (line.agent, mount_path(line.comment))
        for line in repository.tables.docker
        if mount_path(line.comment)
    }
    for agent, mount in sorted(mounts):
        for line in repository.tables.exclude:
            if is_volume_relative(line.glob, catalog) or line_agent(repository, line) != agent:
                continue
            remainders = volume_home_remainders(line.glob, mount)
            if not remainders or volume_twin(line.glob, remainders, volume_exclusions):
                continue
            message = "the volume mounted at %s has no exclusion %s" % (mount, remainders[-1])
            yield Finding(agent, "volume-twin-missing", line.glob, CATALOG_PATH, line.line, message)


def overlapping_cases(repository: Repository) -> Iterator[Finding]:
    """Yield `case-overlap` for each pair of agents' home entries `case_overlap` reports."""
    entries = repository.tables.home
    for later_index, later in enumerate(entries):
        for earlier in entries[:later_index]:
            if earlier.agent == later.agent or not case_overlap(earlier.glob, later.glob):
                continue
            detail = "%s %s" % (earlier.glob, later.glob)
            message = "overlaps %s's entry by case or prefix" % earlier.agent
            yield Finding(later.agent, "case-overlap", detail, CATALOG_PATH, later.line, message)


def credential_wording(repository: Repository) -> Iterator[Finding]:
    """Yield `credential-wording` for each `WORDING_PHRASES` phrase in a Credentials section.

    The detail is the phrase and the nearest code span before it in the section, link texts
    left out (`plain_spans`), so two occurrences in one section stay apart.
    """
    phrases = re.compile("|".join(re.escape(phrase) for phrase in WORDING_PHRASES), re.I)
    for agent in repository.agents.documents:
        document = repository.document(agent)
        for section in repository.collection(agent, "Credentials"):
            spans = plain_spans(section)
            for match in phrases.finditer(section.text):
                before = [text for text, _, end in spans if end <= match.start()]
                nearest = " `%s`" % before[-1] if before else ""
                detail = match.group(0).lower() + nearest
                line = section.line_of(match.start())
                message = "say whether the file is flagged secret, and by which glob"
                yield Finding(agent, "credential-wording", detail, document, line, message)


def presence(repository: Repository) -> Iterator[Finding]:
    """Yield the presence findings: documents, table lines and agents-page rows."""
    agents = repository.agents
    for agent, lines in agents.catalog.items():
        document = repository.document(agent)
        if document not in repository.sections:
            message = "the catalog agent has no %s" % document
            yield Finding(agent, "document-missing", document, CATALOG_PATH, lines[0].line, message)
        if agent not in agents.rows and agent not in ROWLESS_AGENTS:
            message = "the catalog agent has no row in the agents page table"
            yield Finding(agent, "agents-row-missing", agent, CATALOG_PATH, lines[0].line, message)
    for agent, path in agents.documents.items():
        if agent not in agents.catalog:
            relative = path.relative_to(repository.root).as_posix()
            message = "no table line names this agent"
            yield Finding(agent, "catalog-missing", path.name, relative, 1, message)
    page = str(drift_common.AGENTS_PAGE)
    for agent, row in agents.rows.items():
        if agent not in agents.catalog:
            name = agent if drift_common.AGENT_NAME.match(agent) else "-"
            message = "the row names no catalog agent"
            yield Finding(name, "agents-row-orphan", agent, page, row.line, message)


def parsing_facts(repository: Repository) -> Iterator[Finding]:
    """Yield `parsing-fact-under-collection` for each row text or schema key in the Collection.

    "Where the project path is recorded" names its file and key by rule, so its spans are not
    checked for keys; nor is a span a path span of the document holds as a segment or a file's
    stem (`path_names`); link texts are left out (`plain_spans`).
    """
    for agent in repository.agents.documents:
        document = repository.document(agent)
        sections = repository.sections[document]
        schema = {
            span.text
            for section in drift_common.within(sections, "Parsing", "Record schema")
            for span in section.code_spans
        }
        file_names = path_names(
            span.text for section in sections.values() for span in section.code_spans
        )
        for section in drift_common.within(sections, "Collection"):
            project_path = section.titles[1:2] == ("Where the project path is recorded",)
            for text, start, _ in plain_spans(section):
                is_key = not project_path and text in schema and is_key_span(text)
                is_key = is_key and text not in file_names
                if is_row_text(text) or is_key:
                    form = "row text" if is_row_text(text) else "Record schema key"
                    message = "a %s belongs in the Parsing part" % form
                    line = section.line_of(start)
                    kind = "parsing-fact-under-collection"
                    yield Finding(agent, kind, text, document, line, message)


RULES = (
    unnamed_lines,
    unflagged_credentials,
    secrets_under_exclusions,
    unreachable_globs,
    unnamed_volume_globs,
    missing_volume_twins,
    overlapping_cases,
    credential_wording,
    presence,
    parsing_facts,
)


def run(root: pathlib.Path) -> Iterator[Finding]:
    """Yield every finding of every rule for the repository at `root`, each key once."""
    repository = load(root)
    seen: set[tuple[str, str]] = set()
    for rule in RULES:
        for finding in rule(repository):
            pair = (finding.agent, finding.key(CHECKER))
            if pair not in seen:
                seen.add(pair)
                yield finding


def main(arguments: list[str] | None = None, root: pathlib.Path | None = None) -> int:
    """Run the check against the baseline and return the exit status."""
    summary = (__doc__ or "").split("\n")[0]
    return drift_common.main_with_baseline(CHECKER, run, arguments, root, summary)


if __name__ == "__main__":
    sys.exit(main())
