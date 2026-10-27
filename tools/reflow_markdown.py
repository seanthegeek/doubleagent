#!/usr/bin/env python3
"""Check and reflow the paragraphs of Markdown files to a width.

Usage: tools/reflow_markdown.py [--check] [--width N] [--keep-mtime] [-r | --recurse] PATH...

A PATH is a file. One holding `*`, `?` or `[` that names no file is expanded here (`**`
included), so a shell that does not expand globs behaves like one that does; a pattern matching
nothing is an error. A directory needs `-r`, which takes every `.md` and `.markdown` file below
it without following symlinked directories. Each file is processed once.

Without `--check` each file whose reflow differs is rewritten in place and its path printed;
`--keep-mtime` keeps its access and modification times. With `--check` nothing is written: a
unified diff is printed per file that would change, then a summary. `--width` sets the target
width (default 80, at least 20). The exit status is 0 when nothing would change or every
rewrite succeeded, 1 when `--check` found a file to change, 2 on an error (a bad path, an
unreadable, non-UTF-8 or mixed line-ending file, or a refused reflow); the worst status wins.

Only paragraph text is reflowed, at top level, in list items and in blockquotes; a word, code
span, autolink, inline HTML tag or link destination is never split, and no wrapped line starts
with text that Markdown would read as block syntax. Left byte for byte: front matter, fenced and
indented code, headings, thematic breaks, tables, HTML blocks and comments, link reference,
footnote and abbreviation definitions, admonitions, attribute lists, definition lists, math
blocks, a paragraph holding template tags (`{%`, `{{`), a raw HTML tag spanning lines, a
`[label]:` line or a trailing `<br>`, a paragraph one of those blocks interrupts, and each line
ending in a hard break (the text between such lines is filled on its own). Each file keeps its
line ends (LF or CRLF), final newline and byte order mark. Before anything is written or
reported, the output is parsed again and the file is refused unless its blocks, the words of
each paragraph and a second reflow are unchanged.
"""

from __future__ import annotations

import argparse
import difflib
import glob
import os
import re
import sys
import tempfile
from dataclasses import dataclass, field

DEFAULT_WIDTH = 80
MIN_WIDTH = 20
MARKDOWN_SUFFIXES = (".md", ".markdown")
BYTE_ORDER_MARK = "﻿"
# The containers and leaves whose own lines decide where they end, so they take every line
# their containers pass on.
ACCEPTING = {"fence", "indented code", "html", "math"}
CONTAINERS = {"document", "quote", "item"}

ATX_HEADING = re.compile(r"#{1,6}(?:[ \t]|$)")
FENCE = re.compile(r"`{3,}|~{3,}")
FENCE_CLOSE = re.compile(r"(`{3,}|~{3,})[ \t]*$")
THEMATIC_BREAK = re.compile(r"(?:(?:\*[ \t]*){3,}|(?:-[ \t]*){3,}|(?:_[ \t]*){3,})$")
SETEXT_UNDERLINE = re.compile(r"(?:=+|-+)[ \t]*$")
BULLET = re.compile(r"[-+*](?=[ \t]|$)")
ORDERED = re.compile(r"(\d{1,9})[.)](?=[ \t]|$)")
DELIMITER_ROW = re.compile(r"\|?[ \t]*:?-+:?[ \t]*(?:\|[ \t]*:?-+:?[ \t]*)*\|?[ \t]*$")
BLOCK_TAGS = (
    "address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|"
    "details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|"
    "h1|h2|h3|h4|h5|h6|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|nav|"
    "noframes|ol|optgroup|option|p|param|search|section|summary|table|tbody|td|tfoot|th|"
    "thead|title|tr|track|ul"
)
ATTRIBUTE = (
    r"(?:[ \t]+[A-Za-z_:][A-Za-z0-9_.:-]*(?:[ \t]*=[ \t]*(?:[^ \t\"'=<>`]+|'[^']*'|\"[^\"]*\"))?)"
)
# The seven HTML block starts of the CommonMark 0.31.2 specification, with the end condition
# of the first five; the last two end at a blank line.
HTML_STARTS = (
    (
        re.compile(r"<(?:pre|script|style|textarea)(?:[ \t>]|$)", re.I),
        re.compile(r"</(?:pre|script|style|textarea)>", re.I),
    ),
    (re.compile(r"<!--"), re.compile(r"-->")),
    (re.compile(r"<\?"), re.compile(r"\?>")),
    (re.compile(r"<![A-Za-z]"), re.compile(r">")),
    (re.compile(r"<!\[CDATA\["), re.compile(r"\]\]>")),
    (re.compile(r"</?(?:%s)(?:[ \t]|/?>|$)" % BLOCK_TAGS, re.I), None),
    (
        re.compile(
            r"(?:<[A-Za-z][A-Za-z0-9-]*%s*[ \t]*/?>|</[A-Za-z][A-Za-z0-9-]*[ \t]*>)[ \t]*$"
            % ATTRIBUTE
        ),
        None,
    ),
)
# Python-Markdown and MkDocs block syntax CommonMark does not know; each is copied unchanged.
EXTENSION_STARTS = (
    ("admonition", re.compile(r"!!!|\?\?\?")),
    ("footnote", re.compile(r"\[\^[^\]]*\]:")),
    ("abbreviation", re.compile(r"\*\[")),
    ("definition", re.compile(r":(?:[ \t]|$)")),
    ("attribute list", re.compile(r"\{(?![{%<])[^}]*\}[ \t]*$")),
    ("table of contents", re.compile(r"\[TOC\][ \t]*$")),
    ("link reference", re.compile(r"\[(?:[^\[\]\\]|\\.)+\]:")),
)
MATH_OPENERS = (("$$", "$$"), ("\\[", "\\]"))
# A `[label]:` shape over the rest of a paragraph, since a label may span lines.
LABEL_COLON = re.compile(r"\[(?:[^\[\]\\]|\\.)*\]:")
# Text a wrapped line may not start with: the block starts above and what other parsers read
# as one (any `#`, `<`, `|`, `:` or `{`, an ordered item of any number, an empty item).
HAZARD = re.compile(
    r"#|>|[-+*](?:[ \t]|$)|\d{1,9}[.)](?:[ \t]|$)|(?:=+|-+)[ \t]*$|\||`{3}|~{3}"
    r"|<|\[\^|:|!!!|\?\?\?|\{|\*\[|\$\$|\\\[|\[TOC\]"
)
TEMPLATE = re.compile(r"\{%|%\}|\{\{|\}\}")
TRAILING_BREAK_TAG = re.compile(r"<br[ \t]*/?>[ \t]*$", re.I)
ASCII_WHITESPACE = " \t"


class ReflowError(RuntimeError):
    """A file the tool cannot read, decode or reflow safely."""


@dataclass
class Line:
    """One source line and how far the open containers have consumed it."""

    text: str
    index: int = 0
    column: int = 0
    tabbed: bool = False
    segments: list[tuple[str, str]] = field(default_factory=list)


@dataclass
class Block:
    """A container (document, quote, list item) or a leaf block and the lines it holds."""

    kind: str
    label: str = ""
    marker: str = ""
    content_indent: int = 0
    lines: list[int] = field(default_factory=list)
    starts: list[int] = field(default_factory=list)
    children: list[Block] = field(default_factory=list)
    verbatim: bool = False
    has_content: bool = False
    closer: str = ""
    end: re.Pattern[str] | None = None
    first_prefix: str = ""
    continuation_prefix: str = ""


def ascii_strip(text: str) -> str:
    """Return `text` without leading and trailing ASCII spaces and tabs."""
    return text.strip(ASCII_WHITESPACE)


def leading_whitespace(line: Line) -> tuple[int, int]:
    """Return the columns of whitespace at the line's cursor and the index after it."""
    column = line.column
    index = line.index
    text = line.text
    while index < len(text) and text[index] in ASCII_WHITESPACE:
        column += 4 - column % 4 if text[index] == "\t" else 1
        index += 1
    return column - line.column, index


def consume_columns(line: Line, count: int) -> None:
    """Move the cursor over `count` columns of whitespace; a tab marks the line as tabbed."""
    target = line.column + count
    while line.column < target and line.index < len(line.text):
        character = line.text[line.index]
        if character == "\t":
            line.tabbed = True
            line.column += 4 - line.column % 4
        elif character == " ":
            line.column += 1
        else:
            break
        line.index += 1


def html_start(text: str) -> tuple[int, re.Pattern[str] | None] | None:
    """Return the CommonMark HTML block kind (1 to 7) `text` starts, with its end pattern."""
    for kind, (start, end) in enumerate(HTML_STARTS, 1):
        if start.match(text):
            return kind, end
    return None


def extension_start(text: str) -> str:
    """Return the name of the extension block `text` starts, or an empty string."""
    for label, pattern in EXTENSION_STARTS:
        if pattern.match(text):
            return label
    return ""


def math_closer(text: str) -> str:
    """Return the closing delimiter of the math block `text` opens, or an empty string."""
    for opener, closer in MATH_OPENERS:
        if text.startswith(opener):
            return closer
    return ""


class Parser:
    """A conservative CommonMark block parser that records which lines each block holds.

    It follows the line-by-line algorithm of the CommonMark specification's appendix: open
    containers are matched first, then new block starts, then lazy paragraph continuation.
    Python-Markdown extension blocks start wherever a line looks like one.
    """

    def __init__(self, texts: list[str]):
        """Hold `texts`, the file's lines without their line ends."""
        self.texts = texts
        self.document = Block("document")
        self.stack: list[Block] = [self.document]
        self.tip: Block | None = None
        self.matched = 1
        self.leaves: list[Block] = []

    def parse(self) -> Block:
        """Parse every line and return the document block."""
        for number in range(self.front_matter(), len(self.texts)):
            self.add_line(number, Line(self.texts[number]))
        return self.document

    def front_matter(self) -> int:
        """Record a YAML or TOML front matter block at the start and return the next line."""
        if not self.texts:
            return 0
        first = self.texts[0].rstrip(ASCII_WHITESPACE)
        closers = {"---": ("---", "..."), "+++": ("+++",)}.get(first)
        if not closers:
            return 0
        for number in range(1, len(self.texts)):
            if self.texts[number].rstrip(ASCII_WHITESPACE) in closers:
                block = Block("front matter", lines=list(range(number + 1)))
                self.document.children.append(block)
                self.leaves.append(block)
                return number + 1
        return 0

    def continues(self, container: Block, line: Line) -> bool:
        """Consume the container's marker or indent from `line` and say whether it matched."""
        columns, index = leading_whitespace(line)
        start = line.index
        if container.kind == "quote":
            if columns > 3 or index >= len(line.text) or line.text[index] != ">":
                return False
            line.column += columns + 1
            line.index = index + 1
            if line.index < len(line.text) and line.text[line.index] in ASCII_WHITESPACE:
                consume_columns(line, 1)
            line.segments.append(("quote", line.text[start : line.index]))
            return True
        if index == len(line.text):
            if not container.has_content:
                return False
            line.index = index
            return True
        if columns < container.content_indent:
            return False
        consume_columns(line, container.content_indent)
        line.segments.append(("item", line.text[start : line.index]))
        return True

    def open_block(self, block: Block) -> None:
        """Close the unmatched containers and the tip, then add `block` to the container."""
        del self.stack[self.matched :]
        self.tip = None
        if block.kind != "blank":
            for container in self.stack:
                container.has_content = True
        self.stack[-1].children.append(block)
        if block.kind in CONTAINERS:
            self.stack.append(block)
            self.matched = len(self.stack)
        else:
            self.leaves.append(block)
            self.tip = block

    def open_leaf(
        self,
        kind: str,
        number: int,
        keep_open: bool = False,
        closer: str = "",
        end: re.Pattern[str] | None = None,
    ) -> Block:
        """Add a leaf of `kind` holding line `number`, left open only when `keep_open`."""
        block = Block(kind, lines=[number], closer=closer, end=end)
        self.open_block(block)
        if not keep_open:
            self.tip = None
        return block

    def continue_accepting(self, tip: Block, line: Line, number: int) -> bool:
        """Give `line` to an open fence, code, HTML or math block; False when it ends first."""
        columns, index = leading_whitespace(line)
        rest = line.text[index:]
        if tip.kind == "fence":
            tip.lines.append(number)
            close = FENCE_CLOSE.match(rest)
            if (
                columns <= 3
                and close
                and close.group(1).startswith(tip.closer[0])
                and len(close.group(1)) >= len(tip.closer)
            ):
                self.tip = None
            return True
        if tip.kind == "indented code":
            if index == len(line.text) or columns >= 4:
                tip.lines.append(number)
                return True
        elif tip.kind == "html":
            if tip.end is not None:
                tip.lines.append(number)
                if tip.end.search(line.text, line.index):
                    self.tip = None
                return True
            if index < len(line.text):
                tip.lines.append(number)
                return True
        else:
            tip.lines.append(number)
            if tip.closer in line.text[line.index :]:
                self.tip = None
            return True
        self.tip = None
        return False

    def add_paragraph_line(self, block: Block, line: Line, number: int) -> None:
        """Add a continuation line to the paragraph-like `block`."""
        _columns, index = leading_whitespace(line)
        if line.tabbed or "\t" in line.text[line.index : index]:
            block.verbatim = True
        block.lines.append(number)
        block.starts.append(index)

    def open_paragraph(self, line: Line, number: int, label: str = "") -> Block:
        """Start a paragraph-like leaf at `line`; a `label` names an extension kept verbatim."""
        _columns, index = leading_whitespace(line)
        block = Block("paragraph", label=label, verbatim=bool(label), lines=[number])
        block.starts = [index]
        block.first_prefix = line.text[:index]
        block.continuation_prefix = "".join(
            text if kind == "quote" else " " * len(text) for kind, text in line.segments
        )
        if line.tabbed or "\t" in block.first_prefix:
            block.verbatim = True
        self.open_block(block)
        return block

    def open_item(self, number: int, line: Line, columns: int, index: int) -> bool:
        """Start a list item when `line` has a marker at `index`; False when it has none."""
        text = line.text
        match = BULLET.match(text, index) or ORDERED.match(text, index)
        if not match:
            return False
        tip = self.tip
        interrupting = (
            tip is not None and tip.kind == "paragraph" and self.matched == len(self.stack)
        )
        after = Line(text, match.end(), line.column + columns + match.end() - index)
        spaces, content = leading_whitespace(after)
        empty = content == len(text)
        if interrupting and (empty or (match.re is ORDERED and int(match.group(1)) != 1)):
            return False
        used = 1 if empty or spaces >= 5 else spaces
        start = line.index
        line.column = after.column
        line.index = match.end()
        consume_columns(line, used)
        line.segments.append(("item", text[start : line.index]))
        marker = match.group()[-1]
        self.open_block(
            Block(
                "item",
                marker=marker,
                content_indent=columns + len(match.group()) + used,
                lines=[number],
            )
        )
        return True

    def add_line(self, number: int, line: Line) -> None:
        """Place line `number` in the block tree."""
        self.matched = 1
        for container in self.stack[1:]:
            if not self.continues(container, line):
                break
            self.matched += 1
        all_matched = self.matched == len(self.stack)
        tip = self.tip
        if all_matched and tip is not None and tip.kind in ACCEPTING:
            if self.continue_accepting(tip, line, number):
                return
            tip = None
        if self.start_blocks(number, line, all_matched, tip):
            return
        _columns, index = leading_whitespace(line)
        blank = index == len(line.text)
        tip = self.tip
        if tip is not None and tip.kind == "paragraph" and not blank:
            self.add_paragraph_line(tip, line, number)
        elif blank:
            self.open_leaf("blank", number)
        else:
            self.open_paragraph(line, number)

    def start_blocks(self, number: int, line: Line, all_matched: bool, tip: Block | None) -> bool:
        """Open the containers and the leaf `line` starts; True when a leaf took the line."""
        paragraph_tip = tip is not None and tip.kind == "paragraph"
        started = False
        while True:
            columns, index = leading_whitespace(line)
            rest = line.text[index:]
            if not ascii_strip(rest):
                return False
            fresh = not started
            lazy = paragraph_tip and fresh and not all_matched
            interrupting = paragraph_tip and fresh and all_matched
            if columns >= 4:
                if paragraph_tip and fresh:
                    return False
                if tip is not None and tip.kind == "table" and fresh and all_matched:
                    tip.lines.append(number)
                    return True
                consume_columns(line, 4)
                self.open_leaf("indented code", number, keep_open=True)
                return True
            if rest.startswith(">"):
                line.column += columns
                start = line.index
                line.index = index + 1
                line.column += 1
                if line.index < len(line.text) and line.text[line.index] in ASCII_WHITESPACE:
                    consume_columns(line, 1)
                line.segments.append(("quote", line.text[start : line.index]))
                self.open_block(Block("quote", lines=[number]))
                started = True
                continue
            if self.start_leaf(number, rest, interrupting, lazy, tip if fresh else None):
                return True
            if self.open_item(number, line, columns, index):
                started = True
                continue
            label = extension_start(rest)
            if label or math_closer(rest):
                if paragraph_tip and fresh and tip is not None:
                    tip.verbatim = True
                if lazy:
                    return False
                self.start_extension(number, line, rest, label)
                return True
            return False

    def start_leaf(
        self, number: int, rest: str, interrupting: bool, lazy: bool, tip: Block | None
    ) -> bool:
        """Open the CommonMark leaf block `rest` starts, other than a paragraph or code.

        `tip` is the open leaf when no container started on this line, else None; a table takes
        every line that starts no other leaf.
        """
        if ATX_HEADING.match(rest):
            self.open_leaf("heading", number)
            return True
        fence = FENCE.match(rest)
        if fence and not (fence.group()[0] == "`" and "`" in rest[fence.end() :]):
            self.open_leaf("fence", number, keep_open=True, closer=fence.group())
            return True
        html = html_start(rest)
        if html and (html[0] < 7 or not (interrupting or lazy)):
            self.open_leaf("html", number, keep_open=True, end=html[1])
            if html[1] is not None and html[1].search(rest, 1):
                self.tip = None
            return True
        if interrupting and tip is not None:
            if SETEXT_UNDERLINE.match(rest):
                tip.kind = "heading"
                tip.lines.append(number)
                self.tip = None
                return True
            if "|" in rest and DELIMITER_ROW.match(rest) and tip.label == "":
                tip.kind = "table"
                tip.lines.append(number)
                return True
        if THEMATIC_BREAK.match(rest):
            self.open_leaf("thematic break", number)
            return True
        if tip is not None and tip.kind == "table" and not lazy and self.matched == len(self.stack):
            tip.lines.append(number)
            return True
        return False

    def start_extension(self, number: int, line: Line, rest: str, label: str) -> None:
        """Open the math block or verbatim extension block `rest` starts."""
        closer = math_closer(rest)
        if closer and not label:
            self.open_leaf("math", number, keep_open=True, closer=closer)
            if closer in rest[len(closer) :]:
                self.tip = None
            return
        if label == "definition":
            del self.stack[self.matched :]
            siblings = [child for child in self.stack[-1].children if child.kind != "blank"]
            if siblings and siblings[-1].kind == "paragraph":
                siblings[-1].verbatim = True
        self.open_paragraph(line, number, label)


def tokenize(text: str) -> tuple[list[str], bool]:
    """Split paragraph text at ASCII whitespace outside atomic inline constructs.

    Code spans, autolinks and inline HTML, `](...)` link destinations with their titles and
    any `{...}` (an attribute list) are atomic. Returns the tokens and whether an atomic construct
    holds a line break.
    """
    tokens: list[str] = []
    current: list[str] = []
    spans_line = False
    index = 0
    length = len(text)
    while index < length:
        character = text[index]
        end = 0
        if character in " \t\n":
            if current:
                tokens.append("".join(current))
                current = []
            index += 1
            continue
        if character == "\\" and index + 1 < length and text[index + 1] not in " \t\n":
            end = index + 2
        elif character == "`":
            end = code_span_end(text, index)
        elif (
            character == "<"
            and index + 1 < length
            and (text[index + 1].isalpha() or text[index + 1] in "/!?")
        ):
            closer = "-->" if text.startswith("<!--", index) else ">"
            found = text.find(closer, index + 1)
            end = found + len(closer) if found >= 0 else index + 1
        elif character == "]" and text.startswith("(", index + 1):
            close = link_destination_end(text, index + 1)
            end = close + 1 if close >= 0 else index + 1
        elif character == "{":
            found = text.find("}", index + 1)
            end = found + 1 if found >= 0 else index + 1
        else:
            end = index + 1
        piece = text[index:end]
        if "\n" in piece:
            spans_line = True
        current.append(piece)
        index = end
    if current:
        tokens.append("".join(current))
    return tokens, spans_line


def code_span_end(text: str, start: int) -> int:
    """Return the index after the code span opening at `start`, or after its backtick run."""
    run_end = start
    while run_end < len(text) and text[run_end] == "`":
        run_end += 1
    run = run_end - start
    search = run_end
    while True:
        found = text.find("`" * run, search)
        if found < 0:
            return run_end
        after = found + run
        if (after == len(text) or text[after] != "`") and text[found - 1] != "`":
            return after
        while after < len(text) and text[after] == "`":
            after += 1
        search = after


def link_destination_end(text: str, start: int) -> int:
    """Return the index of the `)` closing the link destination opening at `start`, or -1."""
    index = start + 1
    length = len(text)

    def skip_space(position: int) -> int:
        while position < length and text[position] in " \t\n":
            position += 1
        return position

    index = skip_space(index)
    if index < length and text[index] == "<":
        while index < length and text[index] not in ">\n":
            index += 2 if text[index] == "\\" else 1
        if index >= length or text[index] != ">":
            return -1
        index += 1
    else:
        depth = 0
        while index < length and text[index] not in " \t\n":
            character = text[index]
            if character == "\\":
                index += 2
                continue
            if character == "(":
                depth += 1
            elif character == ")":
                if depth == 0:
                    break
                depth -= 1
            index += 1
    index = skip_space(index)
    if index < length and text[index] in "\"'(":
        closer = ")" if text[index] == "(" else text[index]
        index += 1
        while index < length and text[index] != closer:
            index += 2 if text[index] == "\\" else 1
        if index >= length:
            return -1
        index = skip_space(index + 1)
    if index < length and text[index] == ")":
        return index
    return -1


def starts_block(tokens: list[str], start: int, stop: int) -> bool:
    """Say whether a wrapped line of `tokens[start:stop]` would read as block syntax."""
    line = " ".join(tokens[start:stop])
    if HAZARD.match(line) or THEMATIC_BREAK.match(line):
        return True
    if "|" in line and DELIMITER_ROW.match(line):
        return True
    return bool(LABEL_COLON.match(" ".join(tokens[start:])))


def greedy_starts(tokens: list[str], first_room: int, room: int, forbidden: set[int]) -> list[int]:
    """Return the token index each line starts at, never breaking before a forbidden index.

    When the token that overflows may not start a line, the previous word moves down with it
    if it may; otherwise the line overflows.
    """
    starts = [0]
    length = len(tokens[0])
    budget = first_room
    for index in range(1, len(tokens)):
        size = len(tokens[index])
        if length + 1 + size <= budget:
            length += 1 + size
        elif index not in forbidden:
            starts.append(index)
            length = size
            budget = room
        elif index - 1 > starts[-1] and index - 1 not in forbidden:
            starts.append(index - 1)
            length = len(tokens[index - 1]) + 1 + size
            budget = room
        else:
            length += 1 + size
    return starts


def fill(tokens: list[str], first_room: int, room: int) -> list[str]:
    """Fill `tokens` into lines of at most `room` characters, the first `first_room`."""
    forbidden = {index for index in range(1, len(tokens)) if tokens[index - 1].endswith("\\")}
    while True:
        starts = greedy_starts(tokens, first_room, room, forbidden)
        stops = [*starts[1:], len(tokens)]
        bad = [
            start
            for start, stop in zip(starts[1:], stops[1:], strict=True)
            if starts_block(tokens, start, stop)
        ]
        if not bad:
            return [" ".join(tokens[start:stop]) for start, stop in zip(starts, stops, strict=True)]
        forbidden.add(bad[0])


def paragraph_contents(block: Block, texts: list[str]) -> list[str]:
    """Return each line of the paragraph after its container prefix and indent."""
    return [texts[number][start:] for number, start in zip(block.lines, block.starts, strict=True)]


def break_lines(contents: list[str]) -> list[bool]:
    """Say for each line whether it ends in a hard line break."""
    return [
        index < len(contents) - 1 and (content.endswith("  ") or content.endswith("\\"))
        for index, content in enumerate(contents)
    ]


def keeps_verbatim(contents: list[str]) -> bool:
    """Say whether paragraph text holds something the reflow must not touch."""
    text = "\n".join(contents)
    if TEMPLATE.search(text):
        return True
    for index, content in enumerate(contents):
        trailing = content[len(content.rstrip(ASCII_WHITESPACE)) :]
        if "\t" in trailing or TRAILING_BREAK_TAG.search(content):
            return True
        if LABEL_COLON.match("\n".join(contents[index:])):
            return True
    return tokenize(text)[1]


def reflow_paragraph(block: Block, texts: list[str], width: int) -> list[str]:
    """Return the paragraph's lines filled to `width`, or its lines unchanged."""
    original = [texts[number] for number in block.lines]
    contents = paragraph_contents(block, texts)
    if block.verbatim or keeps_verbatim(contents):
        return original
    breaks = break_lines(contents)
    output: list[str] = []
    index = 0
    while index < len(contents):
        if breaks[index]:
            output.append(original[index])
            index += 1
            continue
        stop = index
        while stop < len(contents) and not breaks[stop]:
            stop += 1
        words = " ".join(ascii_strip(content) for content in contents[index:stop])
        tokens = tokenize(words)[0]
        if stop < len(contents) and tokens[-1].endswith("\\"):
            return original
        first = block.first_prefix if index == 0 else block.continuation_prefix
        rest = block.continuation_prefix
        lines = fill(tokens, width - len(first), width - len(rest))
        output.append(first + lines[0])
        output.extend(rest + line for line in lines[1:])
        index = stop
    return output


def reflow_lines(texts: list[str], width: int) -> list[str]:
    """Return the lines of a document with every paragraph reflowed to `width`."""
    parser = Parser(texts)
    parser.parse()
    output: list[str] = []
    for leaf in parser.leaves:
        if leaf.kind == "paragraph":
            output.extend(reflow_paragraph(leaf, texts, width))
        else:
            output.extend(texts[number] for number in leaf.lines)
    return output


def outline(texts: list[str]) -> list[tuple[int, tuple]]:
    """Return each block of the parsed document with its first line and what must not change.

    A container gives its kind, list marker and depth; a reflowable paragraph its words and its
    hard-break lines; any other leaf its lines.
    """
    entries: list[tuple[int, tuple]] = []

    def visit(block: Block, depth: int) -> None:
        for child in block.children:
            if child.kind in CONTAINERS:
                entries.append((child.lines[0], (depth, child.kind, child.marker)))
                visit(child, depth + 1)
                continue
            contents = paragraph_contents(child, texts) if child.kind == "paragraph" else []
            if contents and not (child.verbatim or keeps_verbatim(contents)):
                words = tuple(word for word in re.split(r"[ \t]+", " ".join(contents)) if word)
                kept = tuple(
                    ascii_strip(content)
                    for content, hard in zip(contents, break_lines(contents), strict=True)
                    if hard
                )
                entries.append((child.lines[0], (depth, "paragraph", words, kept)))
            else:
                lines = tuple(texts[number] for number in child.lines)
                entries.append((child.lines[0], (depth, child.kind, child.label, lines)))

    parser = Parser(texts)
    visit(parser.parse(), 0)
    return entries


def check_reflow(texts: list[str], output: list[str], width: int) -> None:
    """Raise ReflowError unless `output` keeps the blocks and words of `texts` and is stable."""
    before = outline(texts)
    after = outline(output)
    for (_line, old), (new_line, new) in zip(before, after, strict=False):
        if old == new:
            continue
        where = "line %d of the output" % (new_line + 1)
        if old[:2] == new[:2] and old[1] == "paragraph":
            raise ReflowError("the words of the paragraph at %s changed" % where)
        raise ReflowError("the block structure changed at %s" % where)
    if len(before) != len(after):
        raise ReflowError("the number of blocks changed from %d to %d" % (len(before), len(after)))
    if reflow_lines(output, width) != output:
        raise ReflowError("reflowing the output again changes it")


def reflow_text(text: str, width: int = DEFAULT_WIDTH) -> str:
    """Return LF-terminated Markdown `text` reflowed to `width`, after the safety check."""
    if not text:
        return text
    final_newline = text.endswith("\n")
    texts = (text[:-1] if final_newline else text).split("\n")
    output = reflow_lines(texts, width)
    check_reflow(texts, output, width)
    return "\n".join(output) + ("\n" if final_newline else "")


def decode(data: bytes) -> tuple[str, str, str]:
    """Return the LF text of file bytes, its byte order mark and its line end."""
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError as error:
        raise ReflowError("not UTF-8 (%s)" % error.reason) from error
    mark = BYTE_ORDER_MARK if text.startswith(BYTE_ORDER_MARK) else ""
    text = text[len(mark) :]
    carriage = text.count("\r")
    if not carriage:
        return text, mark, "\n"
    crlf = text.count("\r\n")
    if crlf != carriage or crlf != text.count("\n"):
        raise ReflowError("mixed line ends (LF, CRLF or a bare CR)")
    return text.replace("\r\n", "\n"), mark, "\r\n"


def write_file(path: str, data: bytes, keep_times: bool) -> None:
    """Replace the file at `path` with `data` through a temporary file, keeping its mode."""
    real = os.path.realpath(path)
    status = os.stat(real)
    directory, name = os.path.split(real)
    handle, temporary = tempfile.mkstemp(prefix="." + name + ".", suffix=".tmp", dir=directory)
    try:
        with os.fdopen(handle, "wb") as stream:
            stream.write(data)
        os.chmod(temporary, status.st_mode & 0o7777)
        os.replace(temporary, real)
    except BaseException:
        if os.path.exists(temporary):
            os.unlink(temporary)
        raise
    if keep_times:
        os.utime(real, ns=(status.st_atime_ns, status.st_mtime_ns))


def process_file(path: str, width: int, check: bool, keep_times: bool) -> int:
    """Reflow or check one file and return its exit status."""
    try:
        with open(path, "rb") as stream:
            data = stream.read()
    except OSError as error:
        raise ReflowError("cannot read (%s)" % (error.strerror or error)) from error
    text, mark, newline = decode(data)
    result = reflow_text(text, width)
    if result == text:
        return 0
    if check:
        diff = difflib.unified_diff(
            text.splitlines(keepends=True),
            result.splitlines(keepends=True),
            "a/" + path,
            "b/" + path,
        )
        sys.stdout.writelines(line if line.endswith("\n") else line + "\n" for line in diff)
        return 1
    try:
        write_file(path, (mark + result.replace("\n", newline)).encode("utf-8"), keep_times)
    except OSError as error:
        raise ReflowError("cannot write (%s)" % (error.strerror or error)) from error
    print(path)
    return 0


def expand_paths(arguments: list[str], recurse: bool) -> tuple[list[str], list[str]]:
    """Return the files the PATH arguments name, once each, and the errors they gave."""
    files: list[str] = []
    errors: list[str] = []
    seen: set[str] = set()

    def add(path: str) -> None:
        real = os.path.realpath(path)
        if real not in seen:
            seen.add(real)
            files.append(path)

    for argument in arguments:
        names = [argument]
        if not os.path.lexists(argument) and any(mark in argument for mark in "*?["):
            names = sorted(glob.glob(argument, recursive=True))
            if not names:
                errors.append("%s: no file matches the pattern" % argument)
                continue
        for name in names:
            if os.path.isdir(name):
                if not recurse:
                    errors.append("%s: is a directory; pass -r to recurse into it" % name)
                    continue
                found = []
                for root, _directories, filenames in os.walk(name):
                    found.extend(
                        os.path.join(root, filename)
                        for filename in filenames
                        if filename.lower().endswith(MARKDOWN_SUFFIXES)
                    )
                for path in sorted(found):
                    add(path)
            elif os.path.isfile(name):
                add(name)
            else:
                errors.append("%s: no such file" % name)
    return files, errors


def main(argv: list[str] | None = None) -> int:
    """Run the command line and return the exit status."""
    parser = argparse.ArgumentParser(description=(__doc__ or "").split("\n")[0])
    parser.add_argument("--check", action="store_true", help="write nothing; print diffs")
    parser.add_argument("--width", type=int, default=DEFAULT_WIDTH, help="target width")
    parser.add_argument(
        "--keep-mtime", action="store_true", help="keep a rewritten file's access and mtime"
    )
    parser.add_argument("-r", "--recurse", action="store_true", help="recurse into directories")
    parser.add_argument("paths", nargs="+", metavar="PATH")
    args = parser.parse_args(argv)
    if args.width < MIN_WIDTH:
        parser.error("--width must be at least %d" % MIN_WIDTH)
    files, errors = expand_paths(args.paths, args.recurse)
    status = 2 if errors else 0
    for message in errors:
        print("error: " + message, file=sys.stderr)
    changed = 0
    for path in files:
        try:
            result = process_file(path, args.width, args.check, args.keep_mtime)
        except ReflowError as error:
            print("error: %s: %s" % (path, error), file=sys.stderr)
            result = 2
        changed += result == 1
        status = max(status, result)
    if args.check:
        print("%d of %d files would be reflowed" % (changed, len(files)))
    return status


if __name__ == "__main__":
    sys.exit(main())
