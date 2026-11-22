#!/usr/bin/env python3
"""Check the parsers against the Parsing part of their research documents.

`tools/parserfacts` (`go test`, `FACTS_TEST`) reads the registered Go parsers with the type
checker and writes, per parser, the file patterns of its kinds and secondary sources, and per
parser package the record keys, row texts and problem messages it passes to the kit (its package
comment says which arguments count). "Documented" means in the code spans and fenced lines of the Parsing part of
the research document of an agent whose parser reads with that package: its own, or one whose
package imports it.

- `row-text-undocumented`: a row text's constant start is in no Parsing span or fenced line.
- `parser-message-undocumented`: no one Parsing span or fenced line holds a problem message's
  constant pieces, in order; a message worded by the kit is left to the key it names.
- `pattern-undocumented`: a file pattern equals no Parsing span or fenced line, both unescaped,
  and no path or glob span outside Fixtures has its count of segments, each matched by the
  pattern's glob form there (`segment_form`); a regex by any of its `expansions`. A pattern
  with a piece known only at run time is not checked.
- `key-undocumented`: a key, column or table the parser reads is no Parsing word.
- `secondary-unlisted`, `survival-row-missing`: the agent's survival row (`secondary_findings`).

Usage: tools/check_parser_research.py [--update-baseline], with `go` on the PATH.
"""

from __future__ import annotations

import itertools
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
from collections.abc import Iterable, Iterator
from dataclasses import dataclass, replace

import drift_common

CHECKER = "check_parser_research"
PARSING = ("Parsing",)
PARSER_PLAN = ("Parsing", "Parser plan")
FIXTURES = (*PARSER_PLAN, "Fixtures")
# The test that writes the facts to the file `FACTS_VARIABLE` names; `-tags` spells every tag
# the project's builds use, since it replaces GOFLAGS' own.
FACTS_TEST = "TestWriteTheRegisteredParsersFacts"
FACTS_COMMAND = ("go", "test", "-tags=noasm,nounsafe", "-count=1", "-run", "^%s$" % FACTS_TEST)
FACTS_PACKAGE = "./tools/parserfacts"
FACTS_VARIABLE = "PARSERFACTS_OUT"
# The pieces of a fact's text the code fills at run time.
HOLE = "{}"
KEY_FORM = re.compile(r"^[A-Za-z_$@][A-Za-z0-9_$@.-]*$")
WORD = re.compile(r"[A-Za-z0-9_$@-]+")
SQLITE_ROW_IDS = frozenset({"rowid", "_rowid", "oid"})
PATTERN_SIGNS = re.compile(r"\\|^\^|\$$|\(\?|\[\^")
CODE_SPAN = re.compile(r"(`+)(.+?)\1", re.DOTALL)
# A regex quantifier, lazy or not, that makes the piece before it variable.
QUANTIFIER = re.compile(r"(?:[*+?]|\{\d*(?:,\d*)?\})\??")
# What a path or glob span segment holds in place of a variable piece.
SPAN_WILDCARD = re.compile(r"<[^>]*>|[*?]")
# A word of a survival table row that may be a file name: dots kept, a trailing one dropped.
FILE_WORD = re.compile(r"[A-Za-z0-9_$@.-]*[A-Za-z0-9_$@-]")
# What a survival row name's placeholder is filled with, each tried in turn: a word, a number,
# a hex hash, a UUID and file-name timestamps with and without milliseconds, the forms a file
# pattern's variable pieces take.
PLACEHOLDER_FILLS = (
    "x",
    "0",
    "0" * 32,
    "00000000-0000-0000-0000-000000000000",
    "2026-01-01T00-00-00Z",
    "2026-01-01T00-00-00.000Z",
    "2026-01-01T00-00-00-000Z",
    "2026-01-01T00-00-00_000Z",
    "2026-01-01",
)
# The survival row's cells from the third on are searched: the agent and the transcript store
# cells name no secondary source.
SURVIVAL_SEARCHED_CELLS = 2
# A run of letters of a name, which `names_selector_tail` replaces to test that it matters.
LETTER_RUN = re.compile(r"[A-Za-z]+")
# Last dotted segments that make a code span a file name rather than a key.
FILE_EXTENSIONS = frozenset(
    {"json", "jsonl", "md", "db", "sqlite", "sqlite3", "txt", "toml", "yaml", "yml", "log"}
    | {"vscdb", "pb", "key", "py", "ts", "js", "lock", "xml", "env", "html", "csv", "zst"}
)


# --- The facts of the Go parsers ---------------------------------------------------------------


@dataclass(frozen=True)
class Pattern:
    r"""A file pattern of a parser: a regex, or a path compared with the artifact's `rel`.

    A regex's `text` has Go's `\A` and `\z` read as `^` and `$` and a leading `(?i)` dropped,
    the form a document writes; `secondary` says the parser's Secondary method tests it, and
    `package` is the import path of the package that writes it.
    """

    text: str
    is_regex: bool
    secondary: bool
    path: str
    line: int
    package: str

    @property
    def regex(self) -> bool:
        """Return whether the pattern is a regular expression."""
        return self.is_regex


@dataclass(frozen=True)
class Fact:
    """A key, row text or problem message a parser package passes to the kit."""

    package: str
    kind: str
    text: str
    prefix: str
    path: str
    line: int


@dataclass(frozen=True)
class ParserFacts:
    """A registered parser: its agent, its home and type packages, and its file patterns."""

    agent: str
    home: str
    package: str
    packages: tuple[str, ...]
    patterns: tuple[Pattern, ...]


@dataclass(frozen=True)
class Facts:
    """The output of `tools/parserfacts`, read."""

    parsers: tuple[ParserFacts, ...]
    facts: tuple[Fact, ...]


def python_pattern(text: str) -> str:
    r"""Return a Go file pattern in the form a document writes: `\A` as `^`, `\z` as `$`.

    A leading `(?i)` flag is dropped, since a document gives the path, not the matching mode.
    """
    text = text.removeprefix("(?i)")
    if text.startswith("\\A"):
        text = "^" + text[2:]
    if text.endswith("\\z"):
        text = text[:-2] + "$"
    return text


def read_facts(data: dict) -> Facts:
    """Return the facts of a decoded `tools/parserfacts` report."""
    parsers = tuple(
        ParserFacts(
            item["agent"],
            item["home"],
            item["package"],
            tuple(item.get("packages") or ()),
            tuple(
                Pattern(
                    python_pattern(found["text"]) if found["regex"] else found["text"],
                    found["regex"],
                    found["secondary"],
                    found["file"],
                    found["line"],
                    found["package"],
                )
                for found in item.get("patterns") or ()
                if found["resolved"] and found["text"]
            ),
        )
        for item in data.get("parsers") or ()
    )
    facts = tuple(
        Fact(
            item["package"], item["kind"], item["text"], item["prefix"], item["file"], item["line"]
        )
        for item in data.get("facts") or ()
    )
    return Facts(parsers, facts)


def go_facts(root: pathlib.Path) -> Facts:
    """Run the facts test of `tools/parserfacts` in the repository at `root`; return its facts.

    Raises:
        RuntimeError: the test could not run, failed or wrote no report; its output is the message.
    """
    command = (*FACTS_COMMAND, FACTS_PACKAGE)
    with tempfile.TemporaryDirectory() as directory:
        out = pathlib.Path(directory) / "facts.json"
        environment = dict(os.environ, **{FACTS_VARIABLE: str(out)})
        try:
            done = subprocess.run(
                command, cwd=root, env=environment, capture_output=True, text=True, check=False
            )
        except OSError as error:
            raise RuntimeError("%s: %s" % (" ".join(command), error)) from error
        if done.returncode != 0 or not out.is_file():
            output = (done.stdout + done.stderr).strip()
            raise RuntimeError("%s failed: %s" % (" ".join(command), output))
        return read_facts(json.loads(out.read_text(encoding="utf-8")))


# --- Patterns and the document ----------------------------------------------------------------


def unescape_pattern(text: str) -> str:
    r"""Return a pattern with each backslash before a character other than a letter or digit removed.

    `\.` reads `.` and `\-` reads `-`, while `\d` and `\s` stay, so a document writing a pattern
    unescaped still names it; a leading `~/` is dropped, as a document writes a home path.
    """
    return re.sub(r"\\([^A-Za-z0-9])", r"\1", text.removeprefix("~/"))


def is_key(text: str) -> bool:
    """Return whether a string has the form of a record key, a table or a column name.

    It is a word of letters, digits, `_`, `$`, `@`, `.` and `-`, starting with neither a digit
    nor punctuation other than `_`, `$` and `@`; `type`, `message.model`, `ai-title` and `$type`
    pass, a line break, `<command-name>`, `a b`, `2x` and the empty string fail.
    """
    return bool(KEY_FORM.match(text))


def looks_like_pattern(text: str) -> bool:
    r"""Return whether a code span is a regular expression.

    It holds a backslash, starts with `^`, ends with `$`, or holds `(?` or `[^`:
    `^\.claude/history\.jsonl$` and `[^/]+\.json` pass, `history.jsonl` and `~/.claude` fail.
    """
    return bool(PATTERN_SIGNS.search(text))


def regex_components(pattern: str) -> list[str]:
    r"""Return a path regex split at each `/` outside a group or class, anchors dropped.

    `^\.claude/projects/[^/]+/.*\.jsonl$` gives `\.claude`, `projects`, `[^/]+` and
    `.*\.jsonl`; a `/` inside `(?:a/b|c)` or `[^/]` splits nothing.
    """
    body = pattern.removeprefix("^").removesuffix("$")
    depth = 0
    in_class = False
    cuts = [-1]
    index = 0
    while index < len(body):
        character = body[index]
        if character == "\\":
            index += 2
            continue
        if in_class:
            in_class = character != "]"
        elif character == "[":
            in_class = True
        elif character == "(":
            depth += 1
        elif character == ")":
            depth -= 1
        elif character == "/" and depth == 0:
            cuts.append(index)
        index += 1
    cuts.append(len(body))
    return [body[low + 1 : high] for low, high in itertools.pairwise(cuts)]


def _group_end(text: str, index: int) -> int:
    """Return the offset after the group opening at `index`, skipping escapes and classes."""
    depth = 0
    in_class = False
    while index < len(text):
        character = text[index]
        if character == "\\":
            index += 2
            continue
        if in_class:
            in_class = character != "]"
        elif character == "[":
            in_class = True
        elif character == "(":
            depth += 1
        elif character == ")":
            depth -= 1
            if depth == 0:
                return index + 1
        index += 1
    return len(text)


def _class_end(text: str, index: int) -> int:
    """Return the offset after the class opening at `index`; a `]` first in it is a member."""
    start = index + 2 if text[index + 1 : index + 2] == "^" else index + 1
    close = text.find("]", start + 1)
    return close + 1 if close > 0 else len(text)


def segment_form(segment: str) -> tuple[str, bool]:
    r"""Return a regex segment as a glob, each variable piece as `*`, and whether it has one.

    A piece is variable when it is a class (`[^/]`, `\d`), a group, or carries a quantifier; a
    lone `.` and an escaped punctuation character read as themselves. `[^/]+\.jsonl` gives
    `*.jsonl`, `local_[0-9a-f-]{36}\.json` gives `local_*.json` and `\.qwen` gives `.qwen`.
    """
    out: list[str] = []
    variable = False
    index = 0
    while index < len(segment):
        character = segment[index]
        text, varies, end = character, False, index + 1
        if character == "\\" and index + 1 < len(segment):
            text, end = segment[index + 1], index + 2
            varies = text.isalnum()
        elif character == "[":
            varies, end = True, _class_end(segment, index)
        elif character == "(":
            varies, end = True, _group_end(segment, index)
        quantifier = QUANTIFIER.match(segment, end)
        if quantifier:
            varies, end = True, quantifier.end()
        out.append("*" if varies else text)
        variable = variable or varies
        index = end
    return re.sub(r"\*+", "*", "".join(out)), variable


def top_level_branches(pattern: str) -> list[str]:
    """Return the branches of a pattern's top-level alternation, or the pattern alone.

    The alternation is a `|` outside every group, or one inside a group spanning the whole
    pattern less its anchors: `^(?:a/b|c)$` gives `a/b` and `c`, whose segment counts differ,
    so `pattern-undocumented` compares each branch with a span on its own.
    """
    body = pattern.removeprefix("^").removesuffix("$")
    if body.startswith("(") and _group_end(body, 0) == len(body):
        inner = body[3:-1] if body.startswith("(?:") else body[1:-1]
        if not inner.startswith("?"):
            branches = _split_top_level(inner)
            if len(branches) > 1:
                return branches
    branches = _split_top_level(pattern)
    return [item.removeprefix("^").removesuffix("$") for item in branches]


def _split_top_level(text: str) -> list[str]:
    """Return `text` split at each `|` outside a group or class."""
    pieces: list[str] = []
    start = 0
    index = 0
    while index < len(text):
        character = text[index]
        if character == "\\":
            index += 2
            continue
        if character == "[":
            index = _class_end(text, index)
            continue
        if character == "(":
            index = _group_end(text, index)
            continue
        if character == "|":
            pieces.append(text[start:index])
            start = index + 1
        index += 1
    pieces.append(text[start:])
    return pieces


def span_segment_documents(span_segment: str, form: str, variable: bool) -> bool:
    """Return whether a path or glob span segment documents a pattern segment's `segment_form`.

    A fixed segment is documented by its literal text. A variable one is documented by a span
    segment holding `*`, `?` or `<...>`, read with each of those as `*`, that the form matches
    when each `*` of the form stands for any text: `<id>.json` documents `*.json`, `a.json` not.
    """
    if not variable:
        return span_segment == form
    if not SPAN_WILDCARD.search(span_segment):
        return False
    glob = re.sub(r"\*+", "*", SPAN_WILDCARD.sub("*", span_segment))
    regex = ".*".join(re.escape(piece) for piece in form.split("*"))
    return re.fullmatch(regex, glob) is not None


def selector_basenames(selector: Pattern) -> list[str]:
    r"""Return the file names a selector's last component is, for a match against a row's words.

    A plain path gives its last segment; a regex gives, per top-level branch, its last
    `regex_components` component read by `segment_form` when it has no variable piece:
    `^\.codex/history\.jsonl$` gives `history.jsonl`, `^\.codex/[^/]+\.jsonl$` nothing, and a
    pattern that does not compile nothing.
    """
    text = selector.text
    if not selector.regex:
        name = text.rstrip("/").rsplit("/", 1)[-1]
        return [name] if name else []
    try:
        re.compile(text)
    except re.error:
        return []
    found: list[str] = []
    for branch in top_level_branches(text):
        components = regex_components(branch)
        form, variable = segment_form(components[-1]) if components else ("", True)
        if form and not variable:
            found.append(form)
    return list(dict.fromkeys(found))


def names_selector_tail(selector: Pattern, name: str) -> bool:
    r"""Return whether a regex's last components match a name only because of its own words.

    The name's `<...>` and `*` placeholders are filled from `PLACEHOLDER_FILLS` and a leading
    `~/` dropped; for some fill and some k, the pattern's last k `regex_components` match the
    name whole, and fail once each letter run of the name outside its placeholders and its file
    extension is replaced. A group may hold a `/`, so k is not the name's segment count.
    `transcript_full.jsonl` names `logs/(transcript(?:_full)?)\.jsonl`; `history.jsonl` does
    not name `[^/]+\.jsonl`.
    """
    if not selector.regex:
        return False
    pieces = re.split(r"(<[^>]*>|\*+)", name.removeprefix("~/").rstrip("/"))
    # `re.split` with one group puts the placeholders at odd indexes, so `literal` is never empty.
    literal = pieces[::2]
    last = literal[-1]
    _, dot, extension = last.rpartition(".")
    keep = dot + extension if dot and extension.lower() in FILE_EXTENSIONS else ""
    changed = [
        LETTER_RUN.sub("qqq", piece) for piece in [*literal[:-1], last[: len(last) - len(keep)]]
    ]
    changed[-1] += keep
    components = regex_components(selector.text)
    try:
        tails = [
            re.compile("/".join(components[-count:])) for count in range(1, len(components) + 1)
        ]
    except re.error:
        return False
    for fill, tail in itertools.product(PLACEHOLDER_FILLS, tails):
        whole = fill.join(literal)
        if whole and tail.fullmatch(whole) and not tail.fullmatch(fill.join(changed)):
            return True
    return False


def holds_pieces(text: str, pieces: list[str]) -> bool:
    """Return whether `text` holds each of `pieces`, in order and not overlapping."""
    start = 0
    for piece in pieces:
        found = text.find(piece, start)
        if found < 0:
            return False
        start = found + len(piece)
    return True


def corpus(sections: list[drift_common.Section]) -> Corpus:
    """Return the corpus of the code spans and fenced lines of `sections`.

    The code spans of the Parser plan's Fixtures subsection name sample files, so they are no
    path or glob span (`span_paths`).
    """
    texts: list[str] = []
    for section in sections:
        texts.extend(span.text for span in section.code_spans)
        texts.extend(
            drift_common.collapse(line) for fence in section.fences for _, line in fence.lines
        )
    spans = [span.text for section in sections for span in section.code_spans]
    outside = [
        span.text
        for section in sections
        if section.titles[: len(FIXTURES)] != FIXTURES
        for span in section.code_spans
    ]
    return corpus_of(texts, spans, outside)


def corpus_of(
    texts: Iterable[str], spans: Iterable[str], path_spans: Iterable[str] | None = None
) -> Corpus:
    """Return the corpus of code span and fenced line texts, `spans` the code spans among them.

    `path_spans` are the spans that may name a path or glob, by default all of `spans`.
    """
    kept = tuple(text for text in texts if text)
    joined = "\n".join(kept)
    spans = frozenset(spans)
    return Corpus(
        kept,
        spans,
        joined,
        frozenset(WORD.findall(joined)),
        frozenset(unescape_pattern(text) for text in kept),
        span_paths(spans if path_spans is None else path_spans),
    )


def span_paths(spans: Iterable[str]) -> frozenset[str]:
    """Return each code span naming a path or glob, as a home-relative path.

    A span names one when it holds a `/` or a dot and is no regex (`looks_like_pattern`); it is
    read as `normalise_span` reads it, so `~/.qwen/projects/*/chats/*.jsonl` gives
    `.qwen/projects/*/chats/*.jsonl`.
    """
    return frozenset(
        drift_common.normalise_span(span)
        for span in spans
        if ("/" in span or "." in span) and not looks_like_pattern(span)
    )


def selector_matches_name(selector: Pattern, name: str) -> bool:
    r"""Return whether a file pattern matches a file or directory a document names.

    In the name a `<...>` placeholder and a `*` read `x` and a leading `~/` is dropped; a name
    with neither a `/` nor a dot is no file. A directory (ending in `/`) matches a pattern
    holding its last segment as a component. A file name of k segments matches a regex whose
    last k `regex_components` match it whole, a bare name only when the last component holds it
    unescaped (`[^/]+\.json` names no file), and a path that ends in it.
    """
    name = re.sub(r"<[^>]*>|\*+", "x", name.removeprefix("~/"))
    segments = [segment for segment in name.split("/") if segment]
    text = selector.text
    if not segments or ("/" not in name and "." not in name):
        return False
    if name.endswith("/"):
        source = unescape_pattern(text) if selector.regex else text
        components = re.split(r"[/^]", source)
        return segments[-1] in (components if selector.regex else components[:-1])
    if not selector.regex:
        whole = "/".join(segments)
        return text == whole or text.endswith("/" + whole)
    components = regex_components(text)
    if len(components) < len(segments):
        return False
    if len(segments) == 1 and segments[0] not in unescape_pattern(components[-1]):
        return False
    try:
        tail = "/".join(components[-len(segments) :])
        return re.fullmatch(tail, "/".join(segments)) is not None
    except re.error:
        return False


# The longest list of concrete branches `expansions` gives one pattern.
MAX_EXPANSIONS = 64
# Escapes that match a place rather than a character.
ANCHOR_ESCAPES = frozenset({"\\A", "\\z", "\\Z", "\\b", "\\B"})
# The flags and name a group may open with, which `expansions` drops.
GROUP_OPENING = re.compile(r"^\?(?:P?<[A-Za-z_][A-Za-z0-9_]*>|[imsU-]*:|[imsU-]+$)")


def expansions(pattern: str) -> list[str]:
    r"""Return a regex's branches with every alternation and optional group spelt out.

    A group with no quantifier or `?` becomes each of its branches (and, for `?`, nothing), a
    capture group also itself; a group repeated by `*`, `+` or a count stays as written; anchors are dropped. So
    `^(?:\.copilot|\.config/\.copilot)/x\.json$` gives `\.copilot/x\.json` and
    `\.config/\.copilot/x\.json`, and `(?:^|/)a$` gives `a` and `/a`. At most
    `MAX_EXPANSIONS` branches are given.
    """
    found: list[str] = []
    for branch in _split_top_level(pattern):
        found.extend(_expand_sequence(branch))
    return list(dict.fromkeys(found))[:MAX_EXPANSIONS]


def _expand_sequence(text: str) -> list[str]:
    """Return the expansions of a regex with no top-level `|`."""
    results = [""]
    index = 0
    while index < len(text):
        start = index
        character = text[index]
        choices: list[str]
        if character == "\\":
            index += 2
            atom = text[start:index]
            choices = [""] if atom in ANCHOR_ESCAPES else [atom]
        elif character == "[":
            index = _class_end(text, index)
            choices = [text[start:index]]
        elif character == "(":
            index = _group_end(text, index)
            inner = GROUP_OPENING.sub("", text[start + 1 : index - 1], count=1)
            quantifier = QUANTIFIER.match(text, index)
            if quantifier and quantifier.group(0) not in ("?", "??"):
                index = quantifier.end()
                choices = [text[start:index]]
            else:
                # A capture group stands for a value a document may write as a placeholder.
                capture = not text.startswith("(?", start) or text.startswith("(?P<", start)
                choices = ([text[start:index]] if capture else []) + expansions(inner)
                if quantifier:
                    index = quantifier.end()
                    choices.append("")
        else:
            index += 1
            choices = [""] if character in "^$" else [character]
        results = [left + right for left in results for right in choices][:MAX_EXPANSIONS]
    return results


@dataclass(frozen=True)
class Corpus:
    """The code spans and fenced lines of some sections, joined, and their words."""

    texts: tuple[str, ...]
    spans: frozenset[str]
    joined: str
    words: frozenset[str]
    patterns: frozenset[str]
    paths: frozenset[str]

    def documents_prefix(self, prefix: str) -> bool:
        """Return whether a code span or fenced line holds `prefix`.

        A bare label prefix, `<label>:`, is also documented by a code span that is the label
        alone, the form a coverage table uses ("label `slash command`").
        """
        if prefix in self.joined:
            return True
        return prefix.endswith(":") and prefix[:-1] in self.spans

    def documents_pieces(self, pieces: list[str]) -> bool:
        """Return whether one code span or fenced line holds every piece, in order."""
        return any(holds_pieces(text, pieces) for text in self.texts)

    def documents_pattern(self, pattern: str, is_regex: bool = True) -> bool:
        r"""Return whether the corpus documents a file pattern, as `pattern-undocumented` says.

        `^\.qwen/trustedFolders\.json$` is documented by `.qwen/trustedFolders.json` and
        `^\.pi/.*$` by `.pi/*`, never by `.pi/probe/x.jsonl`; a regex is documented when one of
        its `expansions` is. A path is compared segment by segment, a leading `/` (a suffix the
        parser tests) letting a longer span end in it and a trailing one (a prefix) start with
        it; a pattern that does not compile is documented by nothing.
        """
        if unescape_pattern(pattern) in self.patterns:
            return True
        if not is_regex:
            forms = [(segment, False) for segment in pattern.strip("/").split("/")]
            place = "end" if pattern.startswith("/") else "start" if pattern.endswith("/") else ""
            return self.documents_forms(forms, place)
        try:
            re.compile(pattern)
        except re.error:
            return False
        for branch in expansions(pattern):
            place = ""
            for opening in ("/", ".*/"):
                if branch.startswith(opening):
                    place, branch = "end", branch[len(opening) :]
            forms = [segment_form(segment) for segment in regex_components(branch)]
            if self.documents_forms(forms, place):
                return True
        return False

    def documents_forms(self, forms: list[tuple[str, bool]], place: str = "") -> bool:
        """Return whether a path span has the segment forms, whole or at `place` (`end`, `start`)."""
        count = len(forms)
        for path in self.paths:
            segments = path.split("/")
            if place == "end":
                segments = segments[-count:]
            elif place == "start":
                segments = segments[:count]
            if len(segments) == count and all(
                span_segment_documents(segment, form, variable)
                for segment, (form, variable) in zip(segments, forms, strict=True)
            ):
                return True
        return False

    def documents_key(self, key: str) -> bool:
        """Return whether a key is a word of the code spans and fenced lines.

        A word is bounded by characters other than letters, digits, `_`, `$`, `@` and `-`, so
        `kind` is a word of `origin.kind` and not of `ai-kind`; a dotted key must appear whole.
        """
        if key in self.words:
            return True
        if not WORD.fullmatch(key):
            bounded = r"(?<![A-Za-z0-9_$@-])%s(?![A-Za-z0-9_$@-])" % re.escape(key)
            return re.search(bounded, self.joined) is not None
        return False


def constant_pieces(text: str) -> list[str]:
    """Return the non-blank constant pieces of a fact's text between its `{}` holes, collapsed."""
    pieces = (drift_common.collapse(piece) for piece in text.split(HOLE))
    return [piece for piece in pieces if piece]


@dataclass
class Document:
    """A research document read for this check: its sections and its Parsing corpus."""

    path: pathlib.Path
    parsing: Corpus


def read_document(path: pathlib.Path) -> Document:
    """Return the document at `path` with its Parsing corpus."""
    sections = drift_common.research_sections(path)
    return Document(path, corpus(drift_common.within(sections, *PARSING)))


# --- The rules --------------------------------------------------------------------------------


@dataclass
class Context:
    """What every rule reads: the facts, the documents and who reads with each package."""

    root: pathlib.Path
    facts: Facts
    documents: dict[str, Document]
    # The agents whose parser reads with each package, in registry order.
    owners: dict[str, list[str]]
    survival: dict[str, drift_common.SurvivalRow]

    def documents_for(self, agent: str, package: str) -> list[Document]:
        """Return the documents that may document a fact: the agent's, then the package owners'."""
        agents = dict.fromkeys([agent, *self.owners.get(package, [])])
        return [self.documents[name] for name in agents if name in self.documents]

    def agent_of(self, package: str) -> str:
        """Return the agent a package's fact belongs to: the parser whose home or type it is.

        A package that is no parser's own, a reader several parsers share, goes to the first of
        its owners by name, so its findings land in one baseline file.
        """
        for parser in self.facts.parsers:
            if package == parser.home:
                return parser.agent
        for parser in self.facts.parsers:
            if package == parser.package:
                return parser.agent
        owners = sorted(self.owners.get(package, []))
        return owners[0] if owners else "-"

    def document_name(self, agent: str) -> str:
        """Return the research document of an agent relative to the repository root."""
        return "research/%s.md" % agent


def fact_findings(context: Context) -> Iterator[drift_common.Finding]:
    """Yield `row-text-undocumented`, `parser-message-undocumented` and `key-undocumented`."""
    for fact in context.facts.facts:
        agent = context.agent_of(fact.package)
        if agent == "-":
            continue
        documents = context.documents_for(agent, fact.package)
        if not documents:
            continue
        message = "no code span or fenced line of the Parsing part of %s holds it"
        message %= context.document_name(agent)
        if fact.kind == "text":
            prefix = drift_common.collapse(fact.prefix)
            if prefix and not any(item.parsing.documents_prefix(prefix) for item in documents):
                yield finding(agent, "row-text-undocumented", prefix, fact, message)
        elif fact.kind == "message":
            pieces = constant_pieces(fact.text)
            if pieces and not any(item.parsing.documents_pieces(pieces) for item in documents):
                detail = drift_common.collapse(fact.text)
                yield finding(agent, "parser-message-undocumented", detail, fact, message)
        elif fact.kind == "key" and is_key(fact.text) and fact.text.lower() not in SQLITE_ROW_IDS:
            if not any(item.parsing.documents_key(fact.text) for item in documents):
                yield finding(agent, "key-undocumented", fact.text, fact, message)


def finding(agent: str, kind: str, detail: str, fact: Fact, message: str) -> drift_common.Finding:
    """Return a finding of an agent at a fact's file and line."""
    return drift_common.Finding(agent, kind, detail, fact.path, fact.line, message)


def pattern_findings(context: Context) -> Iterator[drift_common.Finding]:
    """Yield `pattern-undocumented` per file pattern the Parsing part does not document.

    A pattern that is no regex names a file only when it holds a `/` or a dot.
    """
    for parser in context.facts.parsers:
        documents = context.documents_for(parser.agent, "")
        if parser.agent not in context.documents:
            continue
        seen: set[str] = set()
        for pattern in parser.patterns:
            if pattern.text in seen or not (
                pattern.is_regex or "/" in pattern.text or "." in pattern.text
            ):
                continue
            seen.add(pattern.text)
            readers = documents + context.documents_for(parser.agent, pattern.package)[1:]
            if any(
                item.parsing.documents_pattern(pattern.text, pattern.is_regex) for item in readers
            ):
                continue
            message = "no code span or fenced line of the Parsing part of %s names it"
            yield drift_common.Finding(
                parser.agent,
                "pattern-undocumented",
                pattern.text,
                pattern.path,
                pattern.line,
                message % context.document_name(parser.agent),
            )


def secondary_findings(context: Context) -> Iterator[drift_common.Finding]:
    """Yield `secondary-unlisted` per secondary pattern the survival row names no file of.

    The row is the agent's in the survival table of `research/README.md`, read from its third
    cell (`SURVIVAL_SEARCHED_CELLS`). A pattern is named by a code span there that
    `selector_matches_name` or `names_selector_tail` accepts, never one ending in `/`, since a
    parent directory names no file; or by a word equal to one of its `selector_basenames`; the
    pattern's `expansions` are tried as well as the pattern. A parser with such a pattern and no
    row yields one `survival-row-missing` instead.
    """
    index = drift_common.RESEARCH_INDEX.as_posix()
    for parser in context.facts.parsers:
        if parser.agent not in context.documents:
            continue
        patterns = list({item.text: item for item in parser.patterns if item.secondary}.values())
        row = context.survival.get(parser.agent)
        if patterns and row is None:
            message = "the parser reads %d secondary file pattern(s); the survival table has no row"
            yield drift_common.Finding(
                parser.agent,
                "survival-row-missing",
                parser.agent,
                index,
                0,
                message % len(patterns),
            )
            continue
        if row is not None:
            yield from _unlisted(parser.agent, patterns, row)


def _unlisted(
    agent: str, patterns: list[Pattern], row: drift_common.SurvivalRow
) -> Iterator[drift_common.Finding]:
    """Yield `secondary-unlisted` per pattern a survival row names no file of."""
    index = drift_common.RESEARCH_INDEX.as_posix()
    text = " | ".join(row.cells[SURVIVAL_SEARCHED_CELLS:])
    spans = [
        span
        for span in (drift_common.collapse(match.group(2)) for match in CODE_SPAN.finditer(text))
        if not span.endswith("/")
    ]
    words = set(FILE_WORD.findall(text))
    for pattern in patterns:
        branches = [pattern]
        if pattern.is_regex:
            branches += [replace(pattern, text=branch) for branch in expansions(pattern.text)]
        names = list(dict.fromkeys(name for item in branches for name in selector_basenames(item)))
        if any(
            selector_matches_name(item, span) or names_selector_tail(item, span)
            for item in branches
            for span in spans
        ):
            continue
        if any(name in words for name in names):
            continue
        looked = ", ".join(repr(name) for name in names) or "none, its last component varies"
        message = (
            "the survival row of %s names no file the secondary pattern at %s:%d matches"
            " (no code span matches it; file names looked for among its words: %s)"
        ) % (agent, pattern.path, pattern.line, looked)
        yield drift_common.Finding(
            agent, "secondary-unlisted", pattern.text, index, row.line, message
        )


RULES = (fact_findings, pattern_findings, secondary_findings)


def owners_of(facts: Facts) -> dict[str, list[str]]:
    """Return, per package, the agents whose parser reads with it, in registry order."""
    owners: dict[str, list[str]] = {}
    for parser in facts.parsers:
        for package in dict.fromkeys([parser.home, parser.package, *parser.packages]):
            owners.setdefault(package, []).append(parser.agent)
    return owners


def findings(root: pathlib.Path, facts: Facts) -> Iterator[drift_common.Finding]:
    """Yield the findings of the parsers' facts against the research documents under `root`.

    Each (agent, kind, detail) is yielded once.
    """
    paths = drift_common.agent_documents(root)
    agents = {parser.agent for parser in facts.parsers}
    context = Context(
        root,
        facts,
        {agent: read_document(paths[agent]) for agent in sorted(agents) if agent in paths},
        owners_of(facts),
        drift_common.survival_rows(root),
    )
    seen: set[tuple[str, str, str]] = set()
    for rule in RULES:
        for found in rule(context):
            identity = (found.agent, found.kind, drift_common.collapse(found.detail))
            if identity not in seen:
                seen.add(identity)
                yield found


def run(root: pathlib.Path) -> Iterator[drift_common.Finding]:
    """Yield the findings of every registered parser against its agent's research document."""
    return findings(root, go_facts(root))


def main(
    arguments: list[str] | None = None, root: pathlib.Path | None = None, facts: Facts | None = None
) -> int:
    """Run the check against the baseline and return the exit status.

    `facts` stands in for the report of `tools/parserfacts`, for a test's synthetic parsers.
    """
    summary = (__doc__ or "").split("\n")[0]
    check = run if facts is None else lambda where: findings(where, facts)
    try:
        return drift_common.main_with_baseline(CHECKER, check, arguments, root, summary)
    except RuntimeError as error:
        print("%s: %s" % (CHECKER, error), file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
