#!/usr/bin/env python3
"""Verify the links, section references and external references in the Markdown documents.

The rules are the link and term rules under "Conventions" in AGENTS.md. In every tracked
Markdown file and every untracked one git does not ignore, or in the FILE arguments, it checks
that:

- a link or reference definition with no URL scheme resolves to a file or directory relative
  to the citing file, and a `#fragment` into a Markdown file is one of that file's heading
  anchors as GitHub derives them (`heading_anchors()`);
- no `section <n>` appears, and no issue or pull request number outside link text; the
  changelogs' `[#<n>]` form and a number inside a code span are not matched;
- no bare URL, host path such as `github.com/owner/repo` or commit hash is left in the text,
  and no link text is a URL; a decimal word after `@` is a byte offset, not a hash;
- outside AGENTS.md, CLAUDE.md and the research documents, a term of art in `TERMS` is
  introduced (by a heading, a linked first use or a parenthesised gloss) before it is used;
- AGENTS.md and CLAUDE.md link none of this repository's issues or pull requests.

Fenced code blocks and inline code spans are ignored before matching. The documentation site
(mkdocs/hooks.py) gives every heading the anchor `heading_anchors()` returns, so a link this
check accepts also works there.

Usage: tools/check_doc_links.py [FILE...]

Every failure is printed as `path:line: message`; the exit status is 1 when any check failed.
"""

from __future__ import annotations

import argparse
import pathlib
import re
import subprocess
import sys
import urllib.parse

ROOT = pathlib.Path(__file__).resolve().parent.parent

FENCE = re.compile(r"^[ \t]*(`{3,}|~{3,})")
CODE_SPAN = re.compile(r"(`+)(?!`)(.+?)(?<!`)\1(?!`)", re.S)
# Link text may hold one level of nested brackets; the target may be <...>
# and may be followed by a title.
INLINE_LINK = re.compile(
    r"\[((?:[^\[\]]|\[[^\[\]]*\])*)\]\(\s*(<[^>\n]*>|[^\s()]+(?:\([^\s()]*\)[^\s()]*)*)"
    r"(?:\s+(?:\"[^\"]*\"|'[^']*'|\([^)]*\)))?\s*\)"
)
REF_DEF = re.compile(r"^ {0,3}\[[^\]]+\]:[ \t]*(<[^>\n]*>|\S+)", re.M)
SCHEME = re.compile(r"^[A-Za-z][A-Za-z0-9+.-]*:")
SECTION = re.compile(r"\bsections?\s+\d+", re.I)
ISSUE = re.compile(
    r"(?<![\w/#\[-])(?:(?:issues?|pull requests?|PRs?|discussions?)\s+#?\d+|#\d+)(?![\w-])",
    re.I,
)
ATX = re.compile(r"^ {0,3}(#{1,6})[ \t]+(.*?)(?:[ \t]+#+)?[ \t]*$")
SETEXT = re.compile(r"^ {0,3}(=+|-+)[ \t]*$")
HTML_ID = re.compile(r"<a\s[^>]*\b(?:id|name)=\"([^\"]+)\"", re.I)
URL = re.compile(r"\bhttps?://[^\s<>()\[\]]+", re.I)
HOST_PATH = re.compile(
    r"(?<![\w/@.-])(?:[a-z0-9-]+\.)+(?:com|io|org|net|dev|ai|co|google)/[^\s)\],;]+"
)
# A commit hash left as text: 7 to 40 lowercase hex characters after "at ",
# "commit " or "@", not part of a longer word (so a 64-character sha256 never
# matches). Purely decimal words are byte offsets, rejected in main().
BARE_COMMIT = re.compile(r"(?:\bat |\bcommit |@)([0-9a-f]{7,40})(?![0-9A-Za-z_])")
AUTOLINK = re.compile(r"<[A-Za-z][A-Za-z0-9+.-]*:[^\s<>]*>")
LINK_IN_HEADING = re.compile(r"!?\[((?:[^\[\]]|\[[^\[\]]*\])*)\]\([^)]*\)")
# Terms AGENTS.md defines and uses as terms of art (a cell is one surface on
# one platform of a real-install round, and so on). A page written for a
# reader may rely on one only once it has introduced it: a heading that holds
# the term, a first use that is link text pointing at the definition, or a
# first use followed by a parenthesised gloss. Each entry is the regex of the
# term's forms and the name the message prints.
TERMS: tuple[tuple[str, str], ...] = (
    (r"cells?", "cell"),
    (r"rounds?", "round"),
    (r"surfaces?", "surface"),
    (r"Observed bullets?", "Observed bullet"),
    (r"listing diffs?", "listing diff"),
    (r"secondary sources?", "secondary source"),
    (r"nested entr(?:y|ies)", "nested entry"),
)
# Phrases in which a term has its plain sense and needs no introduction: a
# spreadsheet's or a table's cell. Matched against the text that ends with the
# term, lower-cased.
TERM_PLAIN: dict[str, tuple[str, ...]] = {"cell": ("per cell", "table cell", "table's cell")}
# The documents that define the terms or whose outline AGENTS.md fixes.
TERM_EXEMPT_FILES = ("AGENTS.md", "CLAUDE.md")
TERM_EXEMPT_DIRS = ("research/",)
# The files that state rules without their history: a link to one of this
# repository's issues or pull requests is provenance, which they do not carry.
NO_PROVENANCE_FILES = ("AGENTS.md", "CLAUDE.md")
PROVENANCE_LINK = re.compile(
    r"^<?https?://github\.com/seanthegeek/doubleagent/(?:issues|pull)/\d+", re.I
)


def strip_fences(text: str) -> str:
    """Blank every line of a fenced code block, keeping line numbers."""
    out = []
    fence = ""
    for line in text.split("\n"):
        match = FENCE.match(line)
        if fence:
            closing = match and match.group(1)[0] == fence[0] and len(match.group(1)) >= len(fence)
            if closing and not line.strip().lstrip(fence[0]):
                fence = ""
            out.append("")
        elif match:
            fence = match.group(1)
            out.append("")
        else:
            out.append(line)
    return "\n".join(out)


def blank_code_spans(text: str) -> str:
    """Replace each inline code span with spaces, keeping offsets and newlines."""
    return CODE_SPAN.sub(lambda match: re.sub(r"[^\n]", " ", match.group(0)), text)


def blank(text: str, spans: list[tuple[int, int]]) -> str:
    """Replace each span with spaces, keeping offsets and newlines."""
    chars = list(text)
    for start, end in spans:
        for i in range(start, end):
            if chars[i] != "\n":
                chars[i] = " "
    return "".join(chars)


def anchor(heading: str) -> str:
    """Return the anchor GitHub gives a heading's text, before repeats are numbered."""
    text = LINK_IN_HEADING.sub(r"\1", heading)
    text = text.replace("`", "").strip().lower()
    text = re.sub(r"[^\w\- ]", "", text)
    return text.replace(" ", "-")


def heading_anchors(text: str) -> list[str]:
    """Return the GitHub anchor of every heading in a Markdown text, in order."""
    lines = strip_fences(text).split("\n")
    seen: dict[str, int] = {}
    found: list[str] = []
    for i, line in enumerate(lines):
        match = ATX.match(line)
        heading = None
        if match:
            heading = match.group(2)
        elif (
            i > 0
            and SETEXT.match(line)
            and lines[i - 1].strip()
            and not ATX.match(lines[i - 1])
            and not (
                line.strip().startswith("-") and lines[i - 1].lstrip().startswith(("-", "*", "|"))
            )
        ):
            heading = lines[i - 1]
        if heading is None:
            continue
        slug = anchor(heading)
        count = seen.get(slug, 0)
        seen[slug] = count + 1
        found.append(slug if count == 0 else "%s-%d" % (slug, count))
    return found


def anchors_of(path: pathlib.Path, cache: dict[pathlib.Path, set[str]]) -> set[str]:
    """Return the heading anchors and HTML ids of the Markdown file `path`, cached."""
    if path in cache:
        return cache[path]
    text = path.read_text(encoding="utf-8")
    found = set(heading_anchors(text))
    found.update(HTML_ID.findall(strip_fences(text)))
    cache[path] = found
    return found


def markdown_files() -> list[pathlib.Path]:
    """Return the tracked and unignored Markdown files of the repository, sorted."""
    out = subprocess.run(
        ["git", "-C", str(ROOT), "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout
    files = {ROOT / path for path in out.split("\0") if path.endswith(".md")}
    return sorted(path for path in files if path.is_file())


def line_of(text: str, offset: int) -> int:
    """Return the line number of `offset` in `text`."""
    return text.count("\n", 0, offset) + 1


def check_target(
    source: pathlib.Path, target: str, cache: dict[pathlib.Path, set[str]]
) -> str | None:
    """Return the problem of a link `target` in the file `source`, or None when it resolves."""
    if target.startswith("<") and target.endswith(">"):
        target = target[1:-1]
    if SCHEME.match(target):
        return None
    path_part, _, fragment = target.partition("#")
    path_part = urllib.parse.unquote(path_part)
    if not path_part:
        dest = source
    elif path_part.startswith("/"):
        dest = ROOT / path_part.lstrip("/")
    else:
        dest = source.parent / path_part
    if not dest.exists():
        return "link target does not exist: %s" % target
    if (
        fragment
        and dest.is_file()
        and dest.suffix.lower() == ".md"
        and urllib.parse.unquote(fragment) not in anchors_of(dest, cache)
    ):
        return "no heading for #%s in %s" % (fragment, display(dest))
    return None


def display(path: pathlib.Path) -> str:
    """Return `path` relative to the repository root when it is inside it."""
    try:
        return str(path.resolve().relative_to(ROOT))
    except ValueError:
        return str(path)


def term_problems(text: str, link_text: list[tuple[int, int]]) -> list[tuple[int, str]]:
    """Return the offset and message of each term of art `text` uses before introducing it.

    `text` has its fences stripped and its code spans and link destinations blanked. A term is
    introduced when a heading holds it, or when its first use in a non-plain sense is inside one
    of the `link_text` spans or followed by `(`.
    """
    headings = "\n".join(line for line in text.splitlines() if ATX.match(line))
    out: list[tuple[int, str]] = []
    for pattern, name in TERMS:
        # Markdown renders one line break inside a phrase as a space; a blank line ends it.
        gap = r"(?:[ \t]+|[ \t]*\n[ \t]*)"
        term_pattern = re.compile(r"\b%s\b" % pattern.replace(" ", gap), re.I)
        if term_pattern.search(headings):
            continue
        for match in term_pattern.finditer(text):
            lead = " ".join(text[max(0, match.start() - 16) : match.end()].lower().split())
            if any(lead.endswith(phrase) for phrase in TERM_PLAIN.get(name, ())):
                continue
            if any(start <= match.start() and match.end() <= end for start, end in link_text):
                break
            if re.match(r"[ \t]*\(", text[match.end() : match.end() + 3]):
                break
            out.append(
                (
                    match.start(),
                    'term of art "%s" is not introduced: define it, link its definition, '
                    "or use plain words" % name,
                )
            )
            break
    return out


def main() -> int:
    """Check the files given, or every Markdown file, and return the exit status."""
    parser = argparse.ArgumentParser(description=(__doc__ or "").split("\n")[0])
    parser.add_argument("files", nargs="*", type=lambda argument: pathlib.Path(argument).resolve())
    args = parser.parse_args()
    files = args.files or markdown_files()
    cache: dict[pathlib.Path, set[str]] = {}
    links = bad = 0
    for path in files:
        rel = display(path)
        text = blank_code_spans(strip_fences(path.read_text(encoding="utf-8")))
        link_text: list[tuple[int, int]] = []
        targets: list[tuple[int, str]] = []
        hidden: list[tuple[int, int]] = []
        # The link destinations, reference targets and autolinks alone: what
        # the term check blanks, since a word inside a target is not displayed.
        unseen: list[tuple[int, int]] = []
        for match in INLINE_LINK.finditer(text):
            link_text.append((match.start(1), match.end(1)))
            targets.append((match.start(2), match.group(2)))
            hidden.append((match.start(1), match.end(1)))
            hidden.append((match.end(1), match.end(0)))
            unseen.append((match.end(1), match.end(0)))
            label = " ".join(match.group(1).split())
            if URL.search(label) or HOST_PATH.search(label):
                print("%s:%d: link text is a URL: %s" % (rel, line_of(text, match.start(1)), label))
                bad += 1
        for match in REF_DEF.finditer(text):
            targets.append((match.start(1), match.group(1)))
            hidden.append((match.start(1), match.end(1)))
            unseen.append((match.start(1), match.end(1)))
        for match in AUTOLINK.finditer(text):
            hidden.append((match.start(), match.end()))
            unseen.append((match.start(), match.end()))
        prose = blank(text, hidden)
        for pattern, what in ((URL, "URL"), (HOST_PATH, "host path")):
            for match in pattern.finditer(prose):
                print(
                    "%s:%d: %s is not a link: %s"
                    % (rel, line_of(prose, match.start()), what, match.group(0))
                )
                bad += 1
        for match in BARE_COMMIT.finditer(prose):
            if match.group(1).isdigit():
                continue
            print(
                "%s:%d: bare commit hash %s" % (rel, line_of(prose, match.start(1)), match.group(1))
            )
            bad += 1
        for offset, target in sorted(targets):
            links += 1
            problem = check_target(path, target, cache)
            if problem:
                print("%s:%d: %s" % (rel, line_of(text, offset), problem))
                bad += 1
            if rel in NO_PROVENANCE_FILES and PROVENANCE_LINK.match(target):
                print(
                    "%s:%d: issue or pull request link in a file that states rules, "
                    "not history: %s" % (rel, line_of(text, offset), target.strip("<>"))
                )
                bad += 1
        for match in ISSUE.finditer(text):
            if any(start <= match.start() and match.end() <= end for start, end in link_text):
                continue
            words = " ".join(match.group(0).split())
            print(
                "%s:%d: issue reference is not a link: %s"
                % (rel, line_of(text, match.start()), words)
            )
            bad += 1
        for match in SECTION.finditer(text):
            if any(start <= match.start() and match.end() <= end for start, end in link_text):
                continue
            words = " ".join(match.group(0).split())
            print(
                "%s:%d: section reference is not a link: %s"
                % (rel, line_of(text, match.start()), words)
            )
            bad += 1
        if rel not in TERM_EXEMPT_FILES and not rel.startswith(TERM_EXEMPT_DIRS):
            for offset, problem in term_problems(blank(text, unseen), link_text):
                print("%s:%d: %s" % (rel, line_of(text, offset), problem))
                bad += 1
    if bad:
        print("%d problems in %d links checked in %d files" % (bad, links, len(files)))
        return 1
    print("%d links checked in %d files" % (links, len(files)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
