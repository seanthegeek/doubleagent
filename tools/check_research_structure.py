"""Check that every research document follows the structure and conventions AGENTS.md fixes.

Each agent's document under `research/` (not README.md or frameworks.md) follows "Research
document structure" in AGENTS.md. These rules are hard failures, never baselined: the H1 is
`# <file stem>` with an intro paragraph after it; the H2s are `SECTIONS` in order, Collection
holds `COLLECTION_H3` and Parsing `PARSING_H3` or prose alone; the `FIXED` subsections sit under
their heading in order, Parser plan's eight first; no paragraph opens with a bold label; no line
assumes two documents per agent (`SPLIT_PHRASE`); no confidence statement stands outside
`## Confidence`; no prose line is longer than `--width` (100) unless it holds a URL or is one
code span. These give findings against the baseline under `tools/drift-baseline/`, and an
Observed bullet opening "No real install." or "No real install:" is exempt from the first three:

- `observed-label-missing`: an Observed bullet not opening with a cell label (`observed_label`).
- `observed-cell-duplicate`: two Observed bullets of one document with the same label.
- `latest-version-missing`: an Observed bullet with no version token (`version_tokens`).
- `version-without-change`: a version phrase (`version_phrases`) in Collection or Parsing outside
  Format versions, whose version Format versions does not name, with no link near (`link_near`).
- `duplicate-fact`: one sentence key (`sentence_key`) holding the same code span in two `###`
  sections of one part.
- `bare-doc-reference`: a sentence naming documentation (`documentation_reference`) with no link.

`--before DIR` prints each URL, code span, fenced line, number, bold label, Observed label and
key-table row (`key_table_rows`) of the earlier copy (`DIR/<name>`, else `DIR/collectors/<name>`
and `DIR/analyzer/<name>`) that the document lacks, as `path: lost <kind>: <text>`.

Usage: tools/check_research_structure.py [--before DIR] [--width N] [--update-baseline] [FILE...]

A hard failure prints as `path[:line]: message`. Exit 1 on one, a new finding, or a stale or
malformed baseline line, else 0; `--update-baseline` takes no FILE and keeps hard failures.
"""

from __future__ import annotations

import argparse
import importlib.util
import pathlib
import re
import sys
from dataclasses import dataclass

ROOT = pathlib.Path(__file__).resolve().parent.parent
RESEARCH = ROOT / "research"
SKIP = {"README.md", "frameworks.md"}
CHECKER = "check_research_structure"

SECTIONS = ("Source and evidence", "Collection", "Parsing", "Confidence")
COLLECTION_H3 = (
    "Per-user storage",
    "Credentials",
    "Exclusions",
    "Project-local files",
    "Where the project path is recorded",
)
PARSING_H3 = (
    "Transcript stores",
    "Record schema",
    "Joins and timestamps",
    "Other stores",
    "Format versions",
    "Secrets",
    "Parser plan",
)
PLAN_H4 = (
    "Permission rows",
    "Model rows",
    "Past-use rows and secondary sources",
    "Sync, relay and origin rows",
    "Scratch directory rows",
    "Deletion rows",
    "Bad-line rows",
    "Fixtures",
)
# Fixed subsections, keyed by the heading they sit under.
FIXED = {
    "Source and evidence": ("Observed", "Source reviews"),
    "Per-user storage": ("Scratch directories",),
    "Transcript stores": ("What survives a delete",),
    "Parser plan": PLAN_H4,
}

ATX = re.compile(r"^(#{1,6})\s+(.*?)\s*#*\s*$")
BOLD_LEAD = re.compile(r"^\*\*[^*]+\*\*[.:]?(\s|$)")
URL = re.compile(r"https?://[^\s)\]>]+")
CODE_SPAN = re.compile(r"(?<!`)(`+)(?!`)(.+?)(?<!`)\1(?!`)", re.S)
ONLY_CODE_SPAN = re.compile(r"^\s*(`+)(?!`)(?:[^`]|(?!\1(?!`))`+(?!`))+\1(?!`)[).,;:]*\s*$")
BOLD = re.compile(r"\*\*([^*\n]+)\*\*")
NUMBER = re.compile(r"(?<![\w/.-])\d[\d,.]*(?:\s?(?:[KMGT]i?B|B|bytes|ms|s|%))?(?![\w/.-])")
# Wording that assumes the two documents each agent had before the merge.
SPLIT_PHRASE = re.compile(
    r"\b(?:the )?(?:collectors?|analyzer|parser) (?:document|review|research document)s?\b"
    r"|\bthe two [\w-]+ documents\b"
    r"|\bcollector (?:concern|class)\b",
    re.I,
)
# A confidence paragraph or bullet; it belongs in `## Confidence` only.
CONFIDENCE_LEAD = re.compile(r"^\s*(?:[-*]\s+)?(?:\*\*)?Confidence(?:\*\*)?\s*[.:]")

# The conventions' heuristics; each function below that uses one states its rule.
PLATFORMS = ("macOS", "Windows", "Linux", "FreeBSD", "OpenBSD")
LABEL_LEAD = re.compile(r"^\*\*(?P<label>[^*:]+?)(?P<inside>[:.])?\*\*(?P<outside>[:.])?(?=\s|$)")
LABEL_TEXT = re.compile(r"^(?:%s)(?: [^\s*:]+)+$" % "|".join(PLATFORMS))
NO_INSTALL = re.compile(r"^No real install[.:](?:\s|$)")
VERSION_TOKEN = re.compile(r"\d+\.\d+(?:\.\d+)?")
EARLIER_BUILDS = re.compile(
    r"Earlier builds observed: (?P<items>[^,()]+ \(\d{4}-\d{2}-\d{2}\)"
    r"(?:, [^,()]+ \(\d{4}-\d{2}-\d{2}\))*)\.(?=\s|$)"
)
EARLIER_BUILD_ITEM = re.compile(r"(?:^|, )(?P<build>[^,()]+?) \((?P<date>\d{4}-\d{2}-\d{2})\)")
VERSION_PHRASE = re.compile(
    r"\b(?:[Ss]ince|[Aa]dded in|[Ii]ntroduced in|[Nn]ew in)"
    r"(?: (?:version|build))?(?: (?!(?i:collector|analyzer)\b)[A-Z][\w-]*)?"
    r" `?v?(?P<version>\d+\.\d+(?:\.\d+)*)"
)
INLINE_LINK = re.compile(r"\[[^\]]*\]\([^)\s]+[^)]*\)")
REFERENCE_LINK = re.compile(r"\[[^\]]+\]\[[^\]]*\]")
LINK_TARGET = re.compile(r"\]\([^)]*\)")
DOCUMENTATION = re.compile(r"\b(?:documentation|docs page|changelogs?|release notes)\b", re.I)
README = re.compile(r"\bREADME\b")
LIST_ITEM = re.compile(r"^\s*(?:[-*+]|\d+[.)])\s+")
SENTENCE_END = re.compile(r"(?<=[.!?])\s+(?=[A-Z`\[(*\"])")
KEY_TABLE_HEADER = re.compile(
    r"^\|\s*Key\s*\|\s*Type\s*\|\s*Values\s*\|\s*(?:Since|Builds)\s*\|\s*$"
)
TABLE_SEPARATOR = re.compile(r"^\|[\s:|-]+\|\s*$")
# A duplicate-fact sentence key needs this many words, so "| | string |" matches nothing.
SENTENCE_KEY_WORDS = 3


@dataclass(frozen=True)
class Span:
    """A heading's reach: its path of titles below the H1, its level, and its line range.

    `end` is the last line before the next heading of the same or a higher level, so a
    `###` span holds its `####` subsections.
    """

    titles: tuple[str, ...]
    level: int
    start: int
    end: int


def load_link_check():
    """Import tools/check_doc_links.py, the oracle for fences and headings."""
    path = ROOT / "tools" / "check_doc_links.py"
    spec = importlib.util.spec_from_file_location("check_doc_links", path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def research_files(research: pathlib.Path = RESEARCH) -> list[pathlib.Path]:
    """Return the agents' research documents, sorted."""
    return [path for path in sorted(research.glob("*.md")) if path.name not in SKIP]


def is_agent_document(path: pathlib.Path, research: pathlib.Path = RESEARCH) -> bool:
    """Return whether `path` is an agent's document directly under `research`."""
    return path.resolve().parent == research.resolve() and path.name not in SKIP


def relative(path: pathlib.Path, root: pathlib.Path):
    """Return `path` relative to `root` when it is under it, else `path`."""
    return path.relative_to(root) if path.is_relative_to(root) else path


def headings(text: str) -> list[tuple[int, int, str]]:
    """(line number, level, text) of every ATX heading outside fences."""
    found = []
    for number, line in enumerate(text.split("\n"), 1):
        match = ATX.match(line)
        if match:
            found.append((number, len(match.group(1)), match.group(2).strip()))
    return found


def parents(heads: list[tuple[int, int, str]]) -> dict[str, list[str]]:
    """Each heading's direct children (level + 1), keyed by the parent's text."""
    owner: dict[str, list[str]] = {}
    stack: dict[int, str] = {}
    for _, level, title in heads:
        stack[level] = title
        for deeper in [k for k in stack if k > level]:
            del stack[deeper]
        parent = stack.get(level - 1, "")
        owner.setdefault(parent, []).append(title)
    return owner


def check_outline(rel, owner: dict[str, list[str]], sections: list[str]) -> list[str]:
    """Return the problems of the H2 headings `sections` and the `###` headings under them."""
    problems = []
    if sections != list(SECTIONS):
        problems.append(
            "%s: H2 headings must be, in order: %s; found: %s"
            % (rel, " | ".join(SECTIONS), " | ".join(sections) or "none")
        )
    if owner.get("Collection", []) != list(COLLECTION_H3):
        problems.append(
            "%s: Collection must hold, in order: %s; found: %s"
            % (rel, " | ".join(COLLECTION_H3), " | ".join(owner.get("Collection", [])) or "none")
        )
    parsing = owner.get("Parsing", [])
    if parsing and parsing != list(PARSING_H3):
        problems.append(
            "%s: Parsing must hold, in order: %s (or prose with no ###); found: %s"
            % (rel, " | ".join(PARSING_H3), " | ".join(parsing))
        )
    if owner.get("Confidence"):
        problems.append("%s: `Confidence` takes no subheadings" % rel)
    return problems


def check_fixed(rel, owner: dict[str, list[str]], present: set[str]) -> list[str]:
    """Return the problems of the fixed subsections under the `present` headings."""
    problems = []
    for section, fixed in FIXED.items():
        if section not in present:
            continue
        have = owner.get(section, [])
        missing = [title for title in fixed if title not in have]
        if missing:
            problems.append(
                "%s: `%s` lacks the fixed subsection(s): %s" % (rel, section, ", ".join(missing))
            )
            continue
        if [title for title in have if title in fixed] != list(fixed):
            problems.append(
                "%s: `%s` fixed subsections out of order; want %s"
                % (rel, section, ", ".join(fixed))
            )
        if section == "Parser plan" and have[: len(fixed)] != list(fixed):
            problems.append(
                "%s: the eight fixed row subsections of `Parser plan` come before any other" % rel
            )
    return problems


def check_structure(
    path: pathlib.Path,
    text: str,
    width: int,
    link_check,
    root: pathlib.Path = ROOT,
) -> list[str]:
    """Return every structure problem of the research document `path` holding `text`."""
    if not is_agent_document(path, root / "research"):
        return []
    rel = relative(path, root)
    problems: list[str] = []
    stripped = link_check.strip_fences(text)
    heads = headings(stripped)
    top_heads = [head for head in heads if head[1] == 1]
    if len(top_heads) != 1 or top_heads[0][2] != path.stem:
        problems.append(
            "%s:%d: H1 must be `# %s`" % (rel, top_heads[0][0] if top_heads else 1, path.stem)
        )
    owner = parents(heads)
    problems += check_outline(rel, owner, [head[2] for head in heads if head[1] == 2])
    problems += check_fixed(rel, owner, {head[2] for head in heads})
    lines = stripped.split("\n")
    previous_blank = True
    section = ""
    for number, line in enumerate(lines, 1):
        if line.startswith("## "):
            section = line[3:].strip()
        in_prose = line.strip() and not line.lstrip().startswith(("|", "-", "*", ">", "#", "["))
        if previous_blank and in_prose and BOLD_LEAD.match(line):
            problems.append(
                "%s:%d: bold label used as a heading; make it a heading" % (rel, number)
            )
        split = SPLIT_PHRASE.search(line)
        if split:
            problems.append(
                '%s:%d: "%s" assumes two documents; the document speaks as one'
                % (rel, number, split.group(0))
            )
        if section != SECTIONS[3] and CONFIDENCE_LEAD.match(line):
            problems.append(
                "%s:%d: confidence statement outside `## Confidence`; move it there" % (rel, number)
            )
        long = in_prose and not line.lstrip()[:2].rstrip(".").isdigit() and len(line) > width
        if long and not URL.search(line) and not ONLY_CODE_SPAN.match(line):
            problems.append("%s:%d: prose line longer than %d characters" % (rel, number, width))
        previous_blank = not line.strip()
    first_h2 = next((head[0] for head in heads if head[1] == 2), None)
    if first_h2 is not None and top_heads:
        intro = [line for line in lines[top_heads[0][0] : first_h2 - 1] if line.strip()]
        if not intro:
            problems.append(
                "%s: no intro paragraph between the H1 and `## Source and evidence`" % rel
            )
    return problems


# --- The conventions' heuristics ----------------------------------------------------------


def observed_label(bullet: str) -> str | None:
    """Return the cell label an Observed bullet opens with, or None.

    A label is bold text naming a platform of `PLATFORMS` and one or more surface words
    (`**macOS CLI**`, `**Windows desktop app**`, `**Linux lab container**`), with one colon or
    period inside or right after the closing `**`. A comma after the platform, a missing colon
    or period and any other first word reject it.
    """
    match = LABEL_LEAD.match(bullet)
    if not match or bool(match.group("inside")) == bool(match.group("outside")):
        return None
    label = " ".join(match.group("label").split())
    return label if LABEL_TEXT.match(label) else None


def earlier_builds(bullet: str) -> list[tuple[str, str]] | None:
    """Return the (build, date) pairs of a bullet's earlier-builds sentence, or None.

    The sentence is `Earlier builds observed: <build> (<YYYY-MM-DD>), <build> (<YYYY-MM-DD>).`
    with one or more pairs separated by `, `; a build holds no comma or parenthesis.
    """
    match = EARLIER_BUILDS.search(" ".join(bullet.split()))
    if not match:
        return None
    return [
        (item.group("build"), item.group("date"))
        for item in EARLIER_BUILD_ITEM.finditer(match.group("items"))
    ]


def version_tokens(bullet: str) -> list[str]:
    """Return the version tokens (two or three dotted numbers, `2.1` or `2.1.310`) of a bullet.

    Link targets, bare URLs and the earlier-builds sentence are removed first, so an issue URL
    or an earlier build is no latest version; a date (`YYYY-MM-DD`) is no version token.
    """
    text = URL.sub(" ", LINK_TARGET.sub("] ", " ".join(bullet.split())))
    text = EARLIER_BUILDS.sub(" ", text)
    return VERSION_TOKEN.findall(text)


def version_phrases(line: str) -> list[tuple[str, str]]:
    """Return the (phrase, version) pairs of the version phrases on a line.

    A phrase is "since", "added in", "introduced in" or "new in", then optionally "version" or
    "build", then optionally one capitalised product word, then a version, perhaps in a code
    span and with a `v`; it is returned without backticks: `since 2.1.309`, `Added in CLI 1.3`,
    ``since `v0.30.7` ``. A lower-case word between ("since the 2.1 line"), "since collector
    1.5.0" or "since analyzer 0.17.0" (this project's tools) and "changed between <build> and
    <build>" do not count.
    """
    return [
        (match.group(0).replace("`", ""), match.group("version"))
        for match in VERSION_PHRASE.finditer(line)
    ]


def has_link(text: str) -> bool:
    """Return whether `text` holds an inline or reference link, an autolink or a bare URL."""
    return bool(INLINE_LINK.search(text) or REFERENCE_LINK.search(text) or URL.search(text))


def link_near(lines: list[str], index: int) -> bool:
    """Return whether `lines[index]` or the line after it holds a link (`has_link`)."""
    return any(has_link(line) for line in lines[index : index + 2])


def names_version(text: str, version: str) -> bool:
    """Return whether `text` names `version` whole: `2.1.30` is not named by `2.1.309`."""
    return bool(re.search(r"(?<![\d.])v?%s(?![\d]|\.\d)" % re.escape(version), text))


def sentence_key(sentence: str) -> str:
    """Return a sentence less its code spans, list marker and link targets, whitespace collapsed.

    A list item "The file `a.json` holds [the key](#key)." gives `The file holds the key.`;
    two sentences with the same key state one fact when they also share a code span.
    """
    text = LIST_ITEM.sub("", CODE_SPAN.sub(" ", sentence))
    text = re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", text)
    return " ".join(text.split()).replace(" .", ".").replace(" ,", ",")


def documentation_reference(sentence: str) -> str | None:
    """Return the documentation phrase a sentence names outside its code spans, or None.

    The phrases are "documentation", "docs page", "changelog(s)" and "release notes" in any
    case, and "README" in capitals; `README.md` in a code span and a lower-case "readme" do
    not count.
    """
    text = CODE_SPAN.sub(" ", sentence)
    match = DOCUMENTATION.search(text) or README.search(text)
    return match.group(0) if match else None


def key_table_rows(lines: list[str]) -> list[str]:
    """Return the first cell of each row of every `| Key | Type | Values | Since |` table.

    `Builds` may stand for `Since`; the cell is returned with its whitespace collapsed.
    """
    rows: list[str] = []
    in_table = False
    for line in lines:
        if KEY_TABLE_HEADER.match(line):
            in_table = True
        elif in_table and line.lstrip().startswith("|"):
            if not TABLE_SEPARATOR.match(line.strip()):
                rows.append(" ".join(line.strip().strip("|").split("|")[0].split()))
        else:
            in_table = False
    return rows


# --- Reading a document by heading, paragraph and sentence -------------------------------


def heading_spans(heads: list[tuple[int, int, str]], total: int) -> list[Span]:
    """Return the reach of each heading of level 2 or deeper, in document order."""
    spans: list[Span] = []
    stack: dict[int, str] = {}
    for index, (number, level, title) in enumerate(heads):
        stack = {depth: name for depth, name in stack.items() if depth < level}
        stack[level] = title
        after = [head[0] for head in heads[index + 1 :] if head[1] <= level]
        if level > 1:
            titles = tuple(stack[depth] for depth in sorted(stack) if depth > 1)
            spans.append(Span(titles, level, number, (after[0] - 1) if after else total))
    return spans


def span_of(spans: list[Span], *titles: str) -> Span | None:
    """Return the first span whose title path is `titles`, or None."""
    return next((span for span in spans if span.titles == titles), None)


def observed_bullets(lines: list[str], span: Span) -> list[tuple[int, str]]:
    """Return (line, text) of each top-level list item in an Observed span.

    An item runs to the next item, heading or unindented line; its indented lines, blank lines
    between them included, continue it.
    """
    bullets: list[tuple[int, list[str]]] = []
    in_item = False
    for number in range(span.start + 1, span.end + 1):
        line = lines[number - 1]
        if re.match(r"^[-*] ", line):
            bullets.append((number, [line[2:]]))
            in_item = True
        elif in_item and (not line.strip() or line[:1] == " "):
            bullets[-1][1].append(line)
        elif line.strip():
            in_item = False
    return [(number, " ".join(" ".join(text).split())) for number, text in bullets]


def paragraphs(lines: list[str], first: int, last: int) -> list[tuple[int, str]]:
    """Return (line, text) of each paragraph between two line numbers, inclusive.

    A paragraph ends at a blank line, a heading or a list item's start; each table row is a
    paragraph of its own and a table separator row none.
    """
    found: list[tuple[int, list[str]]] = []
    joining = False
    for number in range(first, last + 1):
        line = lines[number - 1]
        stripped = line.strip()
        if not stripped or ATX.match(line) or TABLE_SEPARATOR.match(stripped):
            joining = False
        elif stripped.startswith("|"):
            found.append((number, [stripped]))
            joining = False
        elif joining and not LIST_ITEM.match(line):
            found[-1][1].append(stripped)
        else:
            found.append((number, [stripped]))
            joining = True
    return [(number, " ".join(text)) for number, text in found]


def sentences(paragraph: str) -> list[str]:
    """Split a paragraph into sentences at `.`, `!` or `?` before a capital, code span or link.

    A full stop inside a code span or link target never splits.
    """
    masked = LINK_TARGET.sub(lambda match: "]" + "x" * (len(match.group(0)) - 1), paragraph)
    masked = CODE_SPAN.sub(lambda match: "`" + "x" * (len(match.group(0)) - 2) + "`", masked)
    pieces: list[str] = []
    start = 0
    for match in SENTENCE_END.finditer(masked):
        pieces.append(paragraph[start : match.start()])
        start = match.end()
    pieces.append(paragraph[start:])
    return [piece for piece in pieces if piece.strip()]


# --- The convention rules -----------------------------------------------------------------


def observed_findings(lines: list[str], spans: list[Span]) -> list[tuple[str, str, int, str]]:
    """Return (kind, detail, line, message) of the three Observed bullet rules."""
    span = span_of(spans, "Source and evidence", "Observed")
    found: list[tuple[str, str, int, str]] = []
    seen: dict[str, int] = {}
    for number, bullet in observed_bullets(lines, span) if span else []:
        if NO_INSTALL.match(bullet):
            continue
        label = observed_label(bullet)
        opening = label or " ".join(bullet.split()[:8])
        if label is None:
            found.append(
                ("observed-label-missing", opening, number, "open with **<Platform> <surface>**:")
            )
        elif label.casefold() in seen:
            message = "the cell of line %d again; keep one bullet per cell" % seen[label.casefold()]
            found.append(("observed-cell-duplicate", label, number, message))
        else:
            seen[label.casefold()] = number
        if not version_tokens(bullet):
            message = "name the latest version observed in this cell"
            found.append(("latest-version-missing", opening, number, message))
    return found


def version_findings(lines: list[str], spans: list[Span]) -> list[tuple[str, str, int, str]]:
    """Return (kind, detail, line, message) of each version phrase with no change recorded."""
    formats = span_of(spans, "Parsing", "Format versions")
    format_text = "\n".join(lines[formats.start - 1 : formats.end]) if formats else ""
    found: list[tuple[str, str, int, str]] = []
    for span in spans:
        if span.level != 2 or span.titles[0] not in ("Collection", "Parsing"):
            continue
        for number in range(span.start, span.end + 1):
            if formats and formats.start <= number <= formats.end:
                continue
            for phrase, version in version_phrases(lines[number - 1]):
                if names_version(format_text, version) or link_near(lines, number - 1):
                    continue
                message = "add a Format versions entry or a citation, or write a range"
                found.append(("version-without-change", phrase, number, message))
    return found


def duplicate_findings(lines: list[str], spans: list[Span]) -> list[tuple[str, str, int, str]]:
    """Return (kind, detail, line, message) of each sentence stated in two `###` sections."""
    found: list[tuple[str, str, int, str]] = []
    for part in ("Collection", "Parsing"):
        first_seen: dict[str, tuple[str, int, set[str]]] = {}
        for span in spans:
            if span.level != 3 or span.titles[0] != part:
                continue
            for number, paragraph in paragraphs(lines, span.start + 1, span.end):
                for sentence in sentences(paragraph):
                    code = {" ".join(body.split()) for _, body in CODE_SPAN.findall(sentence)}
                    key = sentence_key(sentence)
                    if not code or len(re.findall(r"[A-Za-z]+", key)) < SENTENCE_KEY_WORDS:
                        continue
                    earlier = first_seen.setdefault(key, (span.titles[1], number, code))
                    shared = sorted(code & earlier[2])
                    if earlier[0] != span.titles[1] and shared:
                        message = "`%s` also under %s, line %d" % (
                            shared[0],
                            earlier[0],
                            earlier[1],
                        )
                        found.append(("duplicate-fact", key, number, message))
    return found


def reference_findings(lines: list[str]) -> list[tuple[str, str, int, str]]:
    """Return (kind, detail, line, message) of each sentence naming documentation unlinked."""
    found: list[tuple[str, str, int, str]] = []
    for number, paragraph in paragraphs(lines, 1, len(lines)):
        for sentence in sentences(paragraph):
            phrase = documentation_reference(sentence)
            if phrase and not has_link(sentence):
                message = 'names "%s"; link the page at the claim' % phrase
                found.append(("bare-doc-reference", sentence_key(sentence), number, message))
    return found


def convention_findings(path: pathlib.Path, text: str, link_check, root: pathlib.Path) -> list:
    """Return the `drift_common.Finding`s of the convention rules for one research document."""
    import drift_common  # imported here: drift_common imports this module

    if not is_agent_document(path, root / "research"):
        return []
    lines = link_check.strip_fences(text).split("\n")
    spans = heading_spans(headings("\n".join(lines)), len(lines))
    found = observed_findings(lines, spans) + version_findings(lines, spans)
    found += duplicate_findings(lines, spans) + reference_findings(lines)
    agent = path.stem if drift_common.AGENT_NAME.match(path.stem) else "-"
    rel = relative(path, root).as_posix()
    return [
        drift_common.Finding(agent, kind, detail, rel, line, message)
        for kind, detail, line, message in sorted(found, key=lambda item: item[2])
    ]


# --- Invariants a rewrite keeps -----------------------------------------------------------


def tokens(text: str, link_check) -> dict[str, set[str]]:
    """The facts a rewrite must keep: URLs, code spans, fenced lines, numbers, bold labels.

    A code span is compared with its whitespace collapsed, so one wrapped
    across lines matches; a span also counts as kept when its text is a
    fenced line. Observed labels and key-table rows are kept too.
    """
    fenced: set[str] = set()
    in_fence = False
    for line in text.split("\n"):
        if line.lstrip().startswith(("```", "~~~")):
            in_fence = not in_fence
            continue
        if in_fence and line.strip():
            fenced.add(" ".join(line.split()))
    prose = link_check.strip_fences(text)
    spans = {" ".join(body.split()) for _, body in CODE_SPAN.findall(prose)}
    lines = prose.split("\n")
    observed = span_of(
        heading_spans(headings(prose), len(lines)), "Source and evidence", "Observed"
    )
    bullets = observed_bullets(lines, observed) if observed else []
    return {
        "url": {url.rstrip(".,;:") for url in URL.findall(prose)},
        "code span": spans | fenced,
        "fenced line": fenced,
        "number": {match.group(0).strip() for match in NUMBER.finditer(prose)},
        "bold label": {bold.strip().rstrip(".:") for bold in BOLD.findall(prose)},
        "observed label": {label for _, bullet in bullets if (label := observed_label(bullet))},
        "key row": set(key_table_rows(lines)),
    }


def check_invariants(
    path: pathlib.Path,
    text: str,
    before: pathlib.Path,
    link_check,
    root: pathlib.Path = ROOT,
) -> list[str]:
    """Return a `lost` line per fact the copies under `before` hold and `text` lacks."""
    if not is_agent_document(path, root / "research"):
        return []
    rel = relative(path, root)
    olds = [before / path.name]
    if not olds[0].is_file():
        olds = [before / "collectors" / path.name, before / "analyzer" / path.name]
    olds = [candidate for candidate in olds if candidate.is_file()]
    if not olds:
        return ["%s: no pre-merge copy under %s" % (rel, before)]
    old: dict[str, set[str]] = {}
    for old_copy in olds:
        for kind, items in tokens(old_copy.read_text(encoding="utf-8"), link_check).items():
            old.setdefault(kind, set()).update(items)
    new = tokens(text, link_check)
    problems = []
    for kind in new:
        for item in sorted(old.get(kind, set()) - new[kind]):
            problems.append("%s: lost %s: %s" % (rel, kind, item))
    return problems


def report_findings(findings: list, update: bool, checked: set[str] | None, root) -> int:
    """Compare `findings` with the baseline, or rewrite it, and return 1 when the check fails.

    `checked` holds the agents of the FILE arguments, whose baseline lines alone can be stale;
    None means every document was checked.
    """
    import drift_common  # imported here: drift_common imports this module

    if update:
        count = drift_common.update_baseline(root, CHECKER, findings)
        print("%s: baseline holds %d line(s)" % (CHECKER, count), file=sys.stderr)
        return 0
    baseline = drift_common.load_baseline(root, CHECKER)
    if checked is not None:
        baseline.own = {pair: line for pair, line in baseline.own.items() if pair[0] in checked}
    new, stale = drift_common.compare(CHECKER, findings, baseline)
    for line in [finding.render() for finding in new] + stale + baseline.malformed:
        print(line)
    print(
        "%s: %d finding(s), %d new, %d stale baseline line(s), %d malformed"
        % (CHECKER, len(findings), len(new), len(stale), len(baseline.malformed)),
        file=sys.stderr,
    )
    return 1 if new or stale or baseline.malformed else 0


def main(arguments: list[str] | None = None, root: pathlib.Path | None = None) -> int:
    """Check the files given, or every research document, and return the exit status."""
    root = ROOT if root is None else root
    parser = argparse.ArgumentParser(description=(__doc__ or "").split("\n")[0])
    parser.add_argument("--before", type=pathlib.Path, help="earlier copies of the documents")
    parser.add_argument("--width", type=int, default=100, help="longest prose line allowed")
    parser.add_argument(
        "--update-baseline",
        action="store_true",
        help="rewrite this checker's lines under tools/drift-baseline from this run's findings",
    )
    parser.add_argument("files", nargs="*", type=lambda argument: pathlib.Path(argument).resolve())
    options = parser.parse_args(arguments)
    if options.update_baseline and options.files:
        parser.error("--update-baseline checks every document and takes no FILE")
    link_check = load_link_check()
    files = options.files or research_files(root / "research")
    problems: list[str] = []
    findings: list = []
    for path in files:
        text = path.read_text(encoding="utf-8")
        problems += check_structure(path, text, options.width, link_check, root)
        if options.before:
            problems += check_invariants(path, text, options.before.resolve(), link_check, root)
        findings += convention_findings(path, text, link_check, root)
    for problem in problems:
        print(problem)
    print("%d files checked, %d problems" % (len(files), len(problems)))
    checked = {path.stem for path in files} if options.files else None
    failed = report_findings(findings, options.update_baseline, checked, root)
    return 1 if problems or failed else 0


if __name__ == "__main__":
    sys.exit(main())
