#!/usr/bin/env python3
"""Check and reflow the paragraphs of Markdown files to a width.

Usage: tools/reflow_markdown.py [--check] [--width N] [--keep-mtime] [-r | --recurse] PATH...

A PATH is a file. One holding `*`, `?` or `[` that names no file is a pattern expanded here, `**`
included, to the `.md` and `.markdown` files it matches; one matching none is an error. A
directory needs `-r`, which takes those files below it. Suffixes match in any case. A symlinked
directory below the working directory is never followed: a PATH through one is an error and a
pattern skips it. A symlinked file is rewritten through to its target. Paths resolving to one
file are read once (names differing only in case are two paths).

Without `--check` each file whose reflow differs is rewritten, keeping its mode (not its owner,
extended attributes or ACLs), and its path printed; a file with other hard links or without write
permission is refused, and `--keep-mtime` keeps its access and modification times. Rewriting
needs Python-Markdown and markdown-it-py (`python3 -m pip install markdown markdown-it-py`) and
writes nothing without both. `--check` never writes: it prints a diff per changed file and a
summary, warning when the renderers are missing. `--width` (default 80, at least 20) counts code
points. Exit status: 0 when nothing changes or every rewrite succeeds, 1 when `--check` finds a
change, 2 on an error, a refused file or missing renderers; the worst status wins.

Only paragraphs are reflowed, at top level, in list items and in blockquotes, and only the
whitespace between their words changes (line breaks, space runs collapsed, continuation indents):
no wrapped line starts with block syntax, and whitespace in a code span, an HTML tag, a `<pre>` or
`<code>` element, a link destination or image alt text is kept. Code, front matter, headings,
tables, HTML blocks and hard-break lines are kept. A construct only one dialect knows (math,
admonitions, attribute and definition lists, abbreviations, footnotes), a tab, template tags, an
unmatched backtick or a neighbour the dialects read differently leaves its paragraph unchanged.
Line ends, the final newline and a byte order mark are kept. The output must parse to the same
blocks and words and reflow to itself, then render the same, apart from whitespace, under both
renderers (Python-Markdown with common extensions, markdown-it-py as CommonMark with tables).
"""

from __future__ import annotations

import argparse
import difflib
import functools
import glob
import importlib
import os
import re
import stat
import sys
import tempfile
import types
from collections.abc import Callable
from dataclasses import dataclass, field
from html.parser import HTMLParser

DEFAULT_WIDTH = 80
MIN_WIDTH = 20
MARKDOWN_SUFFIXES = (".md", ".markdown")
BYTE_ORDER_MARK = "\ufeff"
# The leaves whose own lines decide where they end, so they take every line their containers
# pass on.
ACCEPTING = {"fence", "indented code", "html"}
CONTAINERS = {"document", "quote", "item"}

ATX_HEADING = re.compile(r"#{1,6}(?:[ \t]|$)")
FENCE = re.compile(r"`{3,}|~{3,}")
# A fence Python-Markdown also reads as one: at the start of the line, with at most one word or
# an attribute list after it. It reads any other as backticks in paragraph text.
PYTHON_MARKDOWN_FENCE = re.compile(r"(?:`{3,}|~{3,})[ ]*(?:\{[^}\n]*\}|\.?[\w#.+-]*)[ ]*$")
FENCE_CLOSE = re.compile(r"(`{3,}|~{3,})[ \t]*$")
THEMATIC_BREAK = re.compile(r"(?:(?:\*[ \t]*){3,}|(?:-[ \t]*){3,}|(?:_[ \t]*){3,})$")
SETEXT_UNDERLINE = re.compile(r"(?:=+|-+)[ \t]*$")
LIST_MARKER = re.compile(r"(?:[-+*]|\d+[.)])[ \t]")
BULLET = re.compile(r"[-+*](?=[ \t]|$)")
# Any digit count: CommonMark stops at nine digits, Python-Markdown does not.
ORDERED = re.compile(r"(\d+)[.)](?=[ \t]|$)")
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
# Line starts only one dialect reads as a block (Python-Markdown and MkDocs extensions, math,
# and a reference definition interrupting a paragraph). They never open a block here; they
# leave the paragraph holding them unchanged.
ONE_DIALECT_START = re.compile(
    r"!!!|\?\?\?|\[\^[^\]]*\]:|\*\[|:(?:[ \t]|$)|\{[^}]*\}[ \t]*$|\[TOC\][ \t]*$|\$\$|\\\[|\\\]"
)
DEFINITION = re.compile(r":(?:[ \t]|$)")
# Whole lines only one dialect reads as a block: an attribute list, a table of contents.
WHOLE_LINE_START = re.compile(r"\{[^}]*\}[ \t]*$|\[TOC\][ \t]*$")
# Line starts Python-Markdown or CommonMark could read as a block inside a paragraph: a heading
# with or without a space, a quote, a list item of any number, a setext underline, a fence,
# a pipe, which Python-Markdown may read as a table row, and `<!`, an HTML comment or declaration.
EITHER_BLOCK_START = re.compile(
    r"#|>|<!|[-+*](?:[ \t]|$)|\d+[.)](?:[ \t]|$)|(?:=+|-+)[ \t]*$|`{3}|~{3}|\|"
)
# A `[label]:` shape over the rest of a paragraph, since a label may span lines.
LABEL_COLON = re.compile(r"\[(?:[^\[\]\\]|\\.)*\]:")
# Text a wrapped line may not start with: every block start above and what other parsers read
# as one (any `<`, `|`, `:` or `{`).
HAZARD = re.compile(
    r"#|>|[-+*](?:[ \t]|$)|\d+[.)](?:[ \t]|$)|(?:=+|-+)[ \t]*$|\||`{3}|~{3}"
    r"|<|\[\^|:|!!!|\?\?\?|\{|\*\[|\$\$|\\\[|\\\]|\[TOC\]"
)
# A backtick inside a link destination: the dialects disagree on whether the code span or the
# link wins.
BACKTICK_DESTINATION = re.compile(r"\]\([^)]*`")
# Link text and a label with whitespace between: Python-Markdown may read a reference link.
SPACED_REFERENCE = re.compile(r"\][ \t\n]+\[")
TEMPLATE = re.compile(r"\{%|%\}|\{\{|\}\}")
TRAILING_BREAK_TAG = re.compile(r"<br[ \t]*/?>[ \t]*$", re.I)
BREAK_TAG_END = re.compile(r"<br[ \t]*/?>$", re.I)
AUTOLINK_START = re.compile(r"<[A-Za-z][A-Za-z0-9+.-]{1,31}:")
# Elements whose content keeps its whitespace when rendered.
PRESERVED_ELEMENTS = ("pre", "code", "textarea", "script", "style")
PRESERVED_OPEN = re.compile(r"<(pre|code|textarea|script|style)(?=[\s/>])", re.I)
RAW_ELEMENT_TAG = re.compile(r"<(/?)(pre|code|textarea|script|style)(?=[\s/>])[^>]*>", re.I)
ASCII_WHITESPACE = " \t"
HTML_WHITESPACE = re.compile(r"[ \t\n\r\f]+")
RENDERED_BLOCKS = {
    "blockquote",
    "dd",
    "div",
    "dl",
    "dt",
    "h1",
    "h2",
    "h3",
    "h4",
    "h5",
    "h6",
    "hr",
    "li",
    "ol",
    "p",
    "table",
    "tbody",
    "td",
    "th",
    "thead",
    "tr",
    "ul",
}
RENDERER_NAMES = ("Python-Markdown", "markdown-it-py")
INSTALL_RENDERERS = "python3 -m pip install markdown markdown-it-py"
PYTHON_MARKDOWN_EXTENSIONS = (
    "fenced_code",
    "tables",
    "attr_list",
    "md_in_html",
    "toc",
    "admonition",
    "def_list",
    "footnotes",
    "abbr",
)


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


@dataclass(eq=False)
class Block:
    """A container (document, quote, list item) or a leaf block and the lines it holds."""

    kind: str
    marker: str = ""
    content_indent: int = 0
    lines: list[int] = field(default_factory=list)
    starts: list[int] = field(default_factory=list)
    children: list[Block] = field(default_factory=list)
    chain: list[Block] = field(default_factory=list)
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


class Parser:
    """A conservative CommonMark block parser that records which lines each block holds.

    It follows the line-by-line algorithm of the CommonMark specification's appendix: open
    containers are matched first, then new block starts, then lazy paragraph continuation.
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
                block.chain = [self.document]
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
            block.chain = list(self.stack)
            self.leaves.append(block)
            self.tip = block

    def open_leaf(
        self,
        kind: str,
        number: int,
        keep_open: bool = False,
        closer: str = "",
        end: re.Pattern[str] | None = None,
    ) -> None:
        """Add a leaf of `kind` holding line `number`, left open only when `keep_open`."""
        self.open_block(Block(kind, lines=[number], closer=closer, end=end))
        if not keep_open:
            self.tip = None

    def continue_accepting(self, tip: Block, line: Line, number: int) -> bool:
        """Give `line` to an open fence, code or HTML block; False when the block ends first."""
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
        elif tip.end is not None:
            tip.lines.append(number)
            if tip.end.search(line.text, line.index):
                self.tip = None
            return True
        elif index < len(line.text):
            tip.lines.append(number)
            return True
        self.tip = None
        return False

    def add_paragraph_line(self, block: Block, line: Line, number: int) -> None:
        """Add a continuation line to the paragraph `block`."""
        _columns, index = leading_whitespace(line)
        if line.tabbed or "\t" in line.text[line.index : index]:
            block.verbatim = True
        block.lines.append(number)
        block.starts.append(index)

    def open_paragraph(self, line: Line, number: int) -> None:
        """Start a paragraph at `line`."""
        _columns, index = leading_whitespace(line)
        block = Block("paragraph", lines=[number], starts=[index])
        block.first_prefix = line.text[:index]
        block.continuation_prefix = "".join(
            text if kind == "quote" else " " * len(text) for kind, text in line.segments
        )
        if line.tabbed or "\t" in block.first_prefix:
            block.verbatim = True
        self.open_block(block)

    def only_definitions(self, block: Block) -> bool:
        """Say whether a paragraph holds only link reference definitions so far.

        markdown-it reads those as definitions, not a paragraph, so a list may start after them.
        """
        lines = paragraph_contents(block, self.texts)
        return all(LABEL_COLON.match(line) or line.startswith(('"', "'", "(")) for line in lines)

    def open_item(self, number: int, line: Line, columns: int, index: int) -> bool:
        """Start a list item when `line` has a marker at `index`; False when it has none."""
        text = line.text
        match = BULLET.match(text, index) or ORDERED.match(text, index)
        if not match:
            return False
        tip = self.tip
        interrupting = (
            tip is not None
            and tip.kind == "paragraph"
            and self.matched == len(self.stack)
            and not self.only_definitions(tip)
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
        self.open_block(
            Block(
                "item",
                marker=match.group()[-1],
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
            if tip is not None and tip.kind == "table" and fresh and all_matched:
                tip.lines.append(number)
                return True
            return False

    def start_leaf(
        self, number: int, rest: str, interrupting: bool, lazy: bool, tip: Block | None
    ) -> bool:
        """Open the CommonMark leaf block `rest` starts, other than a paragraph or code.

        `tip` is the open leaf when no container started on this line, else None.
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
            if "|" in rest and DELIMITER_ROW.match(rest):
                tip.kind = "table"
                tip.lines.append(number)
                return True
        if THEMATIC_BREAK.match(rest):
            self.open_leaf("thematic break", number)
            return True
        return False


def code_span_end(text: str, start: int) -> int:
    """Return the index after the code span opening at `start`, or -1 when it has no closer."""
    run_end = start
    while run_end < len(text) and text[run_end] == "`":
        run_end += 1
    run = run_end - start
    search = run_end
    while True:
        found = text.find("`" * run, search)
        if found < 0:
            return -1
        after = found + run
        if (after == len(text) or text[after] != "`") and text[found - 1] != "`":
            return after
        while after < len(text) and text[after] == "`":
            after += 1
        search = after


def backtick_run_end(text: str, start: int) -> int:
    """Return the index after the run of backticks at `start`."""
    index = start
    while index < len(text) and text[index] == "`":
        index += 1
    return index


def without_code_spans(text: str) -> tuple[str, bool]:
    """Return `text` with its code spans blanked, and whether a backtick run is unmatched."""
    pieces: list[str] = []
    unmatched = False
    index = 0
    while index < len(text):
        character = text[index]
        if character == "\\":
            pieces.append(text[index : index + 2])
            index += 2
        elif character == "`":
            end = code_span_end(text, index)
            if end < 0:
                unmatched = True
                end = backtick_run_end(text, index)
                pieces.append(text[index:end])
            else:
                pieces.append(" " * (end - index))
            index = end
        else:
            pieces.append(character)
            index += 1
    return "".join(pieces), unmatched


def tag_end(text: str, start: int) -> int:
    """Return the index after the HTML tag, comment or autolink at `start`, or -1.

    Quoted attribute values are honoured, so a `>` inside one does not end the tag.
    """
    if text.startswith("<!--", start):
        found = text.find("-->", start + 4)
        return found + 3 if found >= 0 else -1
    if AUTOLINK_START.match(text, start):
        found = text.find(">", start)
        return found + 1 if found >= 0 else -1
    index = start + 1
    while index < len(text):
        character = text[index]
        if character in "\"'":
            close = text.find(character, index + 1)
            if close < 0:
                return -1
            index = close + 1
            continue
        if character == ">":
            return index + 1
        index += 1
    return -1


def element_end(text: str, start: int) -> int:
    """Return the index after the HTML tag at `start` and, for `<pre>` and kin, its content.

    Returns -1 when the tag, or a preserved element's closing tag, is missing.
    """
    end = tag_end(text, start)
    opener = PRESERVED_OPEN.match(text, start)
    if end < 0 or opener is None or text[end - 2] == "/":
        return end
    closing = re.compile(r"</%s[ \t\n]*>" % opener.group(1), re.I).search(text, end)
    return closing.end() if closing else -1


def bracket_end(text: str, start: int) -> int:
    """Return the index of the `]` closing the bracket at `start`, or -1."""
    depth = 0
    index = start
    while index < len(text):
        character = text[index]
        if character == "\\":
            index += 2
            continue
        if character == "[":
            depth += 1
        elif character == "]":
            depth -= 1
            if depth == 0:
                return index
        index += 1
    return -1


def atomic_end(text: str, index: int) -> int:
    """Return the end of the construct at `index` that may not be split, or -1 if unsafe.

    A code span, an HTML tag (or a `<pre>`, `<code>`, `<textarea>`, `<script>` or `<style>`
    element), an autolink, an image's alt text, a link destination with its title and a
    `{...}` attribute list are atomic; any other character is its own piece.
    """
    character = text[index]
    following = text[index + 1] if index + 1 < len(text) else ""
    if character == "\\" and following not in " \t\n":
        return index + 2
    if character == "`":
        end = code_span_end(text, index)
        return end if end >= 0 else -1
    if character == "<" and (following.isalpha() or following in "/!?"):
        end = element_end(text, index)
        if end < 0 and PRESERVED_OPEN.match(text, index):
            return -1
        return end if end >= 0 else index + 1
    if character == "!" and following == "[":
        close = bracket_end(text, index + 1)
        return close if close >= 0 else index + 1
    if character == "]" and following == "(":
        close = link_destination_end(text, index + 1)
        if close < 0:
            # Not a CommonMark destination, but another dialect may read one up to the `)`.
            close = text.find(")", index + 2)
        return close + 1 if close >= 0 else index + 1
    if character == "{":
        found = text.find("}", index + 1)
        return found + 1 if found >= 0 else index + 1
    return index + 1


def tokenize(text: str) -> tuple[list[str], bool]:
    """Split paragraph text at ASCII whitespace outside the constructs `atomic_end` names.

    Returns the tokens and whether the text is unsafe to refill: an atomic construct holds a
    line break, a backtick run or a `<pre>`-like element is unmatched.
    """
    tokens: list[str] = []
    current: list[str] = []
    unsafe = False
    index = 0
    while index < len(text):
        if text[index] in " \t\n":
            if current:
                tokens.append("".join(current))
                current = []
            index += 1
            continue
        end = atomic_end(text, index)
        if end < 0:
            unsafe = True
            end = backtick_run_end(text, index) if text[index] == "`" else index + 1
        piece = text[index:end]
        if "\n" in piece:
            unsafe = True
        current.append(piece)
        index = end
    if current:
        tokens.append("".join(current))
    return tokens, unsafe


def link_destination_end(text: str, start: int) -> int:
    """Return the index of the `)` closing the link destination opening at `start`, or -1.

    A destination holding a backtick is not one, since a code span binds more tightly.
    """
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
    if index < length and text[index] == ")" and "`" not in text[start:index]:
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


def whole_line_block(line: str) -> bool:
    """Say whether `line`, as a whole, reads as a block: an underline, a break, a table row."""
    if SETEXT_UNDERLINE.match(line) or THEMATIC_BREAK.match(line):
        return True
    if "|" in line and DELIMITER_ROW.match(line):
        return True
    kind = html_start(line)
    return bool(WHOLE_LINE_START.match(line) or (kind and kind[0] == 7))


def ends_in_break(token: str) -> bool:
    """Say whether a line ending in `token` would end in a hard break or a `<br>` tag."""
    return token.endswith("\\") or bool(BREAK_TAG_END.search(token))


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
    forbidden = {index for index in range(1, len(tokens)) if ends_in_break(tokens[index - 1])}
    while True:
        starts = greedy_starts(tokens, first_room, room, forbidden)
        stops = [*starts[1:], len(tokens)]
        bad = [
            start
            for start, stop in zip(starts[1:], stops[1:], strict=True)
            if starts_block(tokens, start, stop)
        ]
        if not bad and len(starts) > 1 and whole_line_block(" ".join(tokens[: stops[0]])):
            bad = [starts[1]]
        if not bad:
            return [" ".join(tokens[start:stop]) for start, stop in zip(starts, stops, strict=True)]
        forbidden.add(bad[0])


def paragraph_contents(block: Block, texts: list[str]) -> list[str]:
    """Return each line of the paragraph after its container prefix and indent."""
    return [texts[number][start:] for number, start in zip(block.lines, block.starts, strict=True)]


def break_lines(contents: list[str]) -> list[bool]:
    """Say for each line whether it ends in a hard line break.

    The last line counts too: a dialect that reads the next line into the same paragraph sees
    the break.
    """
    return [content.endswith("  ") or content.endswith("\\") for content in contents]


def either_block_start(content: str, rest: str) -> bool:
    """Say whether a line `content` inside a paragraph could open a block in either dialect."""
    if EITHER_BLOCK_START.match(content) or THEMATIC_BREAK.match(content):
        return True
    if "|" in content and DELIMITER_ROW.match(content):
        return True
    if ONE_DIALECT_START.match(content) or LABEL_COLON.match(rest):
        return True
    kind = html_start(content)
    return kind is not None and kind[0] < 7


def keeps_verbatim(contents: list[str]) -> bool:
    """Say whether paragraph text holds something a refill could change the meaning of."""
    text = "\n".join(contents)
    if TEMPLATE.search(text) or without_code_spans(text)[1] or BACKTICK_DESTINATION.search(text):
        return True
    # A `-->` before any `<!--` closes a comment Python-Markdown may have opened earlier.
    if SPACED_REFERENCE.search(text) or "-->" in text.split("<!--", 1)[0]:
        return True
    if "\t" in text:
        # A tab's width depends on its column, which a refill moves.
        return True
    for index, content in enumerate(contents):
        if TRAILING_BREAK_TAG.search(content):
            return True
        stripped = content.lstrip(ASCII_WHITESPACE)
        if either_block_start(stripped, "\n".join(contents[index:])):
            return True
    return tokenize(text)[1]


def blank_like(text: str) -> bool:
    """Say whether a line holds only whitespace and blockquote markers."""
    return not text.strip(" \t>")


def started_containers(leaf: Block, number: int) -> set[str]:
    """Return the kinds of the containers holding `leaf` that start on line `number`."""
    return {
        container.kind
        for container in leaf.chain
        if container.kind != "document" and container.lines[0] == number
    }


def container_kinds(leaf: Block) -> set[str]:
    """Return the kinds of the containers holding `leaf`."""
    return {container.kind for container in leaf.chain}


def unknown_lists(block: Block, found: set[int]) -> bool:
    """Add to `found` each container holding a `)` list item, which Python-Markdown lacks."""
    unknown = block.kind == "item" and block.marker == ")"
    for child in block.children:
        if child.kind in CONTAINERS and unknown_lists(child, found):
            unknown = True
    if unknown:
        found.add(id(block))
    return unknown


def hidden_lists(block: Block, texts: list[str], owners: dict[int, Block], found: set[int]) -> None:
    """Add to `found` each list item Python-Markdown would read as paragraph text.

    It opens a list only after a blank line or on the line after another item's text.
    """
    for child in block.children:
        if child.kind not in CONTAINERS:
            continue
        first = child.lines[0]
        if child.kind == "item" and first > 0 and not blank_like(texts[first - 1]):
            previous = owners.get(first - 1)
            if (
                previous is None
                or previous.kind != "paragraph"
                or "item" not in container_kinds(previous)
            ):
                found.add(id(child))
        hidden_lists(child, texts, owners, found)


def lazy_items(leaves: list[Block], texts: list[str], found: set[int]) -> None:
    """Add to `found` each list item holding a lazy line, one indented less than its content.

    Python-Markdown may end the list at that line and read the rest of the item differently.
    """
    for leaf in leaves:
        items = [container for container in leaf.chain if container.kind == "item"]
        if leaf.kind != "paragraph" or not items:
            continue
        column = sum(container.content_indent for container in leaf.chain)
        for number in leaf.lines[1:]:
            text = texts[number]
            if len(text) - len(text.lstrip(" ")) < column and not text.lstrip(" ").startswith(">"):
                found.update(id(item) for item in items)
                break


def containers_disagree(leaf: Block, first_line: str, unknown: set[int]) -> bool:
    """Say whether Python-Markdown could read the containers holding a paragraph differently.

    It knows no `)` list marker, so nothing in a container holding one is safe; it nests
    blocks in list items by four-column steps, so a quote marker inside a list item may be
    text to it; and it may read a first line indented four columns or more as code unless that
    is both the item's content column and four columns per list level.
    """
    if any(id(container) in unknown for container in leaf.chain[1:]):
        return True
    kinds = [container.kind for container in leaf.chain]
    if "quote" in kinds and "item" in kinds[: len(kinds) - kinds[::-1].index("quote")]:
        return True
    indent = len(first_line) - len(first_line.lstrip(" "))
    column = sum(container.content_indent for container in leaf.chain)
    return indent >= 4 and not indent == column == 4 * kinds.count("item")


def dialects_disagree(
    leaf: Block, owners: dict[int, Block], texts: list[str], unknown: set[int]
) -> bool:
    """Say whether the lines around a paragraph could give it another block structure.

    A list item or blockquote that starts beside the paragraph with no blank line between,
    where the neighbour is not inside a container of that kind, an indented code or HTML block
    right before it, or a setext underline after it, may be read differently by
    Python-Markdown, which needs a blank line before a list; so may the containers
    `containers_disagree` names.
    """
    first = leaf.lines[0]
    last = leaf.lines[-1]
    if containers_disagree(leaf, texts[first], unknown):
        return True
    if first > 0 and not blank_like(texts[first - 1]):
        before = owners.get(first - 1)
        if before is not None and before.kind in ("indented code", "html"):
            return True
    started = started_containers(leaf, first)
    if started and first > 0 and not blank_like(texts[first - 1]):
        previous = owners.get(first - 1)
        if previous is None or not started <= container_kinds(previous):
            return True
    following = last + 1
    if following < len(texts) and not blank_like(texts[following]):
        if SETEXT_UNDERLINE.match(texts[following].lstrip(" \t>")):
            return True
        neighbour = owners.get(following)
        if neighbour is not None:
            after = started_containers(neighbour, following)
            if after and not after <= container_kinds(leaf):
                return True
    return False


def raw_element_depths(text: str, depths: dict[str, int]) -> bool:
    """Update the open `<pre>`-like element counts from `text`; True when they changed."""
    before = dict(depths)
    for match in RAW_ELEMENT_TAG.finditer(text):
        name = match.group(2).lower()
        depths[name] = max(0, depths[name] + (-1 if match.group(1) else 1))
    return depths != before


def definition_terms(number: int, texts: list[str], owners: dict[int, Block]) -> list[Block]:
    """Return the blocks of a definition line and of the terms Python-Markdown gives it.

    The terms are every line of the run of non-blank lines before it, past blank lines,
    reference, footnote and abbreviation definitions and indented lines.
    """
    found = [owners[number]] if number in owners else []
    line = number - 1
    # Python-Markdown removes reference, footnote and abbreviation definitions, with their
    # indented lines, before it reads the blocks.
    while line >= 0 and (
        not ascii_strip(texts[line])
        or LABEL_COLON.match(ascii_strip(texts[line]).lstrip("*"))
        or texts[line].startswith("    ")
    ):
        line -= 1
    while line >= 0 and ascii_strip(texts[line]):
        if line in owners:
            found.append(owners[line])
        line -= 1
    return found


def table_rows(table: Block, texts: list[str], owners: dict[int, Block]) -> list[Block]:
    """Return the blocks after a table up to the next blank line: Python-Markdown's rows."""
    found = []
    line = table.lines[-1] + 1
    while line < len(texts) and ascii_strip(texts[line]):
        if line in owners:
            found.append(owners[line])
        line += 1
    return found


def line_run(leaf: Block, texts: list[str], owners: dict[int, Block]) -> list[Block]:
    """Return the blocks of the run of non-blank lines holding `leaf`.

    Python-Markdown may join such a run into one block, so a backtick left open in one of its
    paragraphs can close in another.
    """
    first = leaf.lines[0]
    last = leaf.lines[-1]
    while first > 0 and ascii_strip(texts[first - 1]):
        first -= 1
    while last + 1 < len(texts) and ascii_strip(texts[last + 1]):
        last += 1
    return [owners[line] for line in range(first, last + 1) if line in owners]


def neighbour_runs(leaves: list[Block], texts: list[str], owners: dict[int, Block]) -> set[int]:
    """Return the ids of the blocks in a run of lines Python-Markdown may join differently.

    Those are the lines after a table up to a blank line, the run around a fence it does not
    know, a block with an unmatched backtick or a table delimiter row, and a definition with
    its terms.
    """
    found: set[int] = set()
    for leaf in leaves:
        raw = "\n".join(texts[number] for number in leaf.lines)
        first = texts[leaf.lines[0]]
        unknown_fence = leaf.kind == "fence" and not PYTHON_MARKDOWN_FENCE.match(first)
        open_backtick = leaf.kind not in ("fence", "indented code") and without_code_spans(raw)[1]
        if leaf.kind == "table":
            found.update(id(block) for block in table_rows(leaf, texts, owners))
        elif unknown_fence or open_backtick:
            found.update(id(block) for block in line_run(leaf, texts, owners))
    for number, text in enumerate(texts):
        stripped = text.lstrip(" \t>")
        if DEFINITION.match(stripped):
            found.update(id(leaf) for leaf in definition_terms(number, texts, owners))
        if "|" in stripped and DELIMITER_ROW.match(stripped) and not LIST_MARKER.match(stripped):
            # Python-Markdown may read a table here whatever the containers around it.
            found.update(id(leaf) for leaf in line_run(owners[number], texts, owners))
    return found


def html_leaves_open(leaf: Block, texts: list[str], depths: dict[str, int]) -> bool:
    """Update the open `<pre>`-like counts from an HTML block; True when one stays open.

    A fence line inside the block counts as open too: CommonMark reads it as HTML, while
    Python-Markdown may end the block before it and open a fence there.
    """
    raw_element_depths("\n".join(texts[number] for number in leaf.lines), depths)
    fence = any(FENCE.match(texts[number].lstrip(" \t>")) for number in leaf.lines)
    return fence or any(depths.values())


def verbatim_paragraphs(document: Block, leaves: list[Block], texts: list[str]) -> set[int]:
    """Return the ids of the paragraph leaves to copy unchanged."""
    owners = {number: leaf for leaf in leaves for number in leaf.lines}
    unknown: set[int] = set()
    unknown_lists(document, unknown)
    hidden_lists(document, texts, owners, unknown)
    lazy_items(leaves, texts, unknown)
    verbatim = neighbour_runs(leaves, texts, owners)
    depths: dict[str, int] = {name: 0 for name in PRESERVED_ELEMENTS}
    # Once a `<pre>`-like element stays open past its block, or an HTML block holds a fence
    # line, the dialects may close them in different places: no later paragraph is safe.
    unclosed = False
    for leaf in leaves:
        if leaf.kind == "html":
            unclosed = html_leaves_open(leaf, texts, depths) or unclosed
            continue
        if leaf.kind in ("heading", "table"):
            raw = "\n".join(texts[number] for number in leaf.lines)
            raw_element_depths(without_code_spans(raw)[0], depths)
            unclosed = unclosed or any(depths.values())
            continue
        if leaf.kind != "paragraph":
            continue
        inside = unclosed or any(depths.values())
        raw = "\n".join(texts[number] for number in leaf.lines)
        changed = raw_element_depths(without_code_spans(raw)[0], depths)
        unclosed = unclosed or any(depths.values())
        if (
            leaf.verbatim
            or inside
            or changed
            or keeps_verbatim(paragraph_contents(leaf, texts))
            or dialects_disagree(leaf, owners, texts, unknown)
        ):
            verbatim.add(id(leaf))
    return verbatim


def reflow_paragraph(block: Block, texts: list[str], width: int) -> list[str]:
    """Return the paragraph's lines filled to `width`, or its lines unchanged."""
    original = [texts[number] for number in block.lines]
    contents = paragraph_contents(block, texts)
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
        if stop < len(contents) and ends_in_break(tokens[-1]):
            return original
        first = block.first_prefix if index == 0 else block.continuation_prefix
        rest = block.continuation_prefix
        lines = fill(tokens, width - len(first), width - len(rest))
        output.append(first + lines[0])
        output.extend(rest + line for line in lines[1:])
        index = stop
    return output


def parse(texts: list[str]) -> tuple[Block, list[Block], set[int]]:
    """Parse `texts` and return the document, its leaves and the verbatim paragraph ids."""
    parser = Parser(texts)
    document = parser.parse()
    return document, parser.leaves, verbatim_paragraphs(document, parser.leaves, texts)


def reflow_lines(texts: list[str], width: int) -> list[str]:
    """Return the lines of a document with every paragraph reflowed to `width`."""
    _document, leaves, verbatim = parse(texts)
    output: list[str] = []
    for leaf in leaves:
        if leaf.kind == "paragraph" and id(leaf) not in verbatim:
            output.extend(reflow_paragraph(leaf, texts, width))
        else:
            output.extend(texts[number] for number in leaf.lines)
    return output


def outline(texts: list[str]) -> list[tuple[int, tuple]]:
    """Return each block of the parsed document with its first line and what must not change.

    A container gives its kind, list marker and depth; a reflowed paragraph its words and its
    hard-break lines; any other leaf, an unchanged paragraph included, its lines.
    """
    document, _leaves, verbatim = parse(texts)
    entries: list[tuple[int, tuple]] = []

    def visit(block: Block, depth: int) -> None:
        for child in block.children:
            if child.kind in CONTAINERS:
                entries.append((child.lines[0], (depth, child.kind, child.marker)))
                visit(child, depth + 1)
            elif child.kind == "paragraph" and id(child) not in verbatim:
                contents = paragraph_contents(child, texts)
                words = tuple(word for word in re.split(r"[ \t]+", " ".join(contents)) if word)
                kept = tuple(
                    ascii_strip(content)
                    for content, hard in zip(contents, break_lines(contents), strict=True)
                    if hard
                )
                entries.append((child.lines[0], (depth, "paragraph", words, kept)))
            else:
                kind = "verbatim paragraph" if child.kind == "paragraph" else child.kind
                lines = tuple(texts[number] for number in child.lines)
                entries.append((child.lines[0], (depth, kind, lines)))

    visit(document, 0)
    return entries


def check_reflow(texts: list[str], output: list[str], width: int) -> None:
    """Raise ReflowError unless `output` keeps the blocks and words of `texts` and is stable."""
    before = outline(texts)
    after = outline(output)
    for (_line, old), (new_line, new) in zip(before, after, strict=False):
        if old == new:
            continue
        where = "line %d of the output" % (new_line + 1)
        if old[:2] != new[:2]:
            raise ReflowError(
                "the block structure changed at %s: a %s is now a %s" % (where, old[1], new[1])
            )
        if old[1] == "paragraph" and old[2] != new[2]:
            raise ReflowError("the words of the paragraph at %s changed" % where)
        raise ReflowError("the %s at %s changed" % (old[1], where))
    if len(before) != len(after):
        raise ReflowError("the number of blocks changed from %d to %d" % (len(before), len(after)))
    if reflow_lines(output, width) != output:
        raise ReflowError("reflowing the output again changes it")


class RenderedEvents(HTMLParser):
    """The tags, attributes, text and comments of rendered HTML, in order."""

    def __init__(self) -> None:
        """Start with no events."""
        super().__init__(convert_charrefs=True)
        self.events: list[tuple] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        """Record a start tag with its attributes exactly."""
        self.events.append(("start", tag, tuple(attrs)))

    def handle_startendtag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        """Record a self-closing tag."""
        self.events.append(("start", tag, tuple(attrs)))
        self.events.append(("end", tag))

    def handle_endtag(self, tag: str) -> None:
        """Record an end tag."""
        self.events.append(("end", tag))

    def handle_data(self, data: str) -> None:
        """Record text, merged with the text before it."""
        if self.events and self.events[-1][0] == "text":
            data = self.events.pop()[1] + data
        self.events.append(("text", data))

    def handle_comment(self, data: str) -> None:
        """Record a comment, whitespace collapsed, since a reader never sees it."""
        self.events.append(("comment", HTML_WHITESPACE.sub(" ", data)))

    def handle_decl(self, decl: str) -> None:
        """Record a declaration exactly."""
        self.events.append(("declaration", decl))

    def handle_pi(self, data: str) -> None:
        """Record a processing instruction exactly."""
        self.events.append(("processing instruction", data))

    def unknown_decl(self, data: str) -> None:
        """Record a CDATA section or other declaration exactly."""
        self.events.append(("declaration", data))


def html_events(html: str) -> list[tuple]:
    """Return rendered HTML as events, whitespace collapsed outside preserved elements.

    Whitespace at the edge of a block element's text is dropped, since a reflow may move a line
    break there.
    """
    parser = RenderedEvents()
    preserving = 0
    events: list[tuple] = []
    parser.feed(html)
    parser.close()
    for event in parser.events:
        if event[0] == "start" and event[1] in PRESERVED_ELEMENTS:
            preserving += 1
        elif event[0] == "end" and event[1] in PRESERVED_ELEMENTS and preserving:
            preserving -= 1
        if event[0] == "text" and not preserving:
            event = ("text", HTML_WHITESPACE.sub(" ", event[1]))
        events.append(event)
    trimmed: list[tuple] = []
    for index, event in enumerate(events):
        if event[0] == "text" and event[1].strip(" ") != event[1]:
            text = event[1]
            if index == 0 or events[index - 1][1] in RENDERED_BLOCKS:
                text = text.lstrip(" ")
            if index + 1 == len(events) or events[index + 1][1] in RENDERED_BLOCKS:
                text = text.rstrip(" ")
            if not text:
                continue
            event = ("text", text)
        trimmed.append(event)
    return trimmed


def python_markdown_renderer(module: types.ModuleType, extensions: list[str]) -> Callable:
    """Return a function rendering Markdown with the `markdown` module and `extensions`."""

    def render(text: str) -> str:
        return module.Markdown(extensions=extensions).convert(text)

    return render


@functools.lru_cache(maxsize=1)
def renderers() -> tuple[tuple[str, Callable[[str], str]], ...]:
    """Return the Markdown renderers that import here, by name.

    They are imported by name at run time because neither is a dependency: the tool runs, and
    type-checks, without them.
    """
    found: list[tuple[str, Callable[[str], str]]] = []
    try:
        markdown = importlib.import_module("markdown")
    except ImportError:
        markdown = None
    if markdown is not None:
        extensions = []
        for name in PYTHON_MARKDOWN_EXTENSIONS:
            try:
                importlib.import_module("markdown.extensions." + name)
            except ImportError:
                continue
            extensions.append(name)
        found.append(("Python-Markdown", python_markdown_renderer(markdown, extensions)))
    try:
        markdown_it = importlib.import_module("markdown_it")
    except ImportError:
        markdown_it = None
    if markdown_it is not None:
        engine = markdown_it.MarkdownIt("commonmark").enable("table")
        found.append(("markdown-it-py", engine.render))
    return tuple(found)


def render_difference(before: str, after: str) -> str:
    """Return how the renderings of `before` and `after` differ, or an empty string."""
    for name, render in renderers():
        old = html_events(render(before))
        new = html_events(render(after))
        if old == new:
            continue
        index = next(
            (
                number
                for number, pair in enumerate(zip(old, new, strict=False))
                if pair[0] != pair[1]
            ),
            min(len(old), len(new)),
        )
        shown = old[index] if index < len(old) else "the end"
        return "%s renders it differently, first at %r" % (name, shown)
    return ""


def reflow_text(text: str, width: int = DEFAULT_WIDTH) -> str:
    """Return LF-terminated Markdown `text` reflowed to `width`, after the safety checks."""
    if not text:
        return text
    final_newline = text.endswith("\n")
    texts = (text[:-1] if final_newline else text).split("\n")
    output = reflow_lines(texts, width)
    check_reflow(texts, output, width)
    result = "\n".join(output) + ("\n" if final_newline else "")
    if result != text:
        difference = render_difference(text, result)
        if difference:
            raise ReflowError(difference)
    return result


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


def split_lines(text: str) -> list[str]:
    """Split LF text into lines that keep their line ends."""
    parts = text.split("\n")
    lines = [part + "\n" for part in parts[:-1]]
    if parts[-1]:
        lines.append(parts[-1] + "\n")
    return lines


def write_file(real: str, data: bytes, status: os.stat_result, keep_times: bool) -> None:
    """Replace the file `real` with `data` through a temporary file, keeping its mode."""
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
    real = os.path.realpath(path)
    try:
        status = os.stat(real)
        if not stat.S_ISREG(status.st_mode):
            raise ReflowError("not a regular file")
        with open(real, "rb") as stream:
            data = stream.read()
    except OSError as error:
        raise ReflowError("cannot read (%s)" % (error.strerror or error)) from error
    text, mark, newline = decode(data)
    result = reflow_text(text, width)
    if result == text:
        return 0
    if check:
        diff = difflib.unified_diff(
            split_lines(text), split_lines(result), "a/" + path, "b/" + path
        )
        sys.stdout.writelines(line if line.endswith("\n") else line + "\n" for line in diff)
        return 1
    if status.st_nlink > 1:
        raise ReflowError("has %d hard links, which a rewrite would split" % status.st_nlink)
    if not os.access(real, os.W_OK):
        raise ReflowError("not writable")
    try:
        write_file(real, (mark + result.replace("\n", newline)).encode("utf-8"), status, keep_times)
    except OSError as error:
        raise ReflowError("cannot write (%s)" % (error.strerror or error)) from error
    print(path)
    return 0


def through_symlinked_directory(path: str) -> bool:
    """Say whether `path` passes through a symlinked directory below the working directory.

    The directories above the working directory (macOS's `/var`, say) are the system's, not
    the tree being reflowed, and are not checked.
    """
    here = os.path.abspath(os.getcwd())
    directory = os.path.dirname(os.path.abspath(path))
    while directory.startswith(here + os.sep):
        if os.path.islink(directory):
            return True
        directory = os.path.dirname(directory)
    return False


def directory_files(directory: str) -> list[str]:
    """Return the Markdown files below `directory`, sorted, never following a symlinked one."""
    found = []
    for root, _directories, filenames in os.walk(directory):
        found.extend(
            os.path.join(root, filename)
            for filename in filenames
            if filename.lower().endswith(MARKDOWN_SUFFIXES)
        )
    return sorted(found)


def expand_paths(arguments: list[str], recurse: bool) -> tuple[list[str], list[str]]:
    """Return the files the PATH arguments name, once each, and the errors they gave.

    A pattern takes only the Markdown files it matches and skips any match reached through a
    symlinked directory; a PATH reached through one is an error.
    """
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
        pattern = not os.path.lexists(argument) and any(mark in argument for mark in "*?[")
        if pattern:
            names = [
                name
                for name in sorted(glob.glob(argument, recursive=True))
                if not through_symlinked_directory(name)
                and not os.path.isdir(name)
                and name.lower().endswith(MARKDOWN_SUFFIXES)
            ]
            if not names:
                errors.append("%s: no Markdown file matches the pattern" % argument)
                continue
        for name in names:
            if os.path.isdir(name) and os.path.islink(name):
                errors.append("%s: a symlinked directory, which is not followed" % name)
            elif through_symlinked_directory(name):
                errors.append("%s: inside a symlinked directory, which is not followed" % name)
            elif os.path.isdir(name):
                if not recurse:
                    errors.append("%s: is a directory; pass -r to recurse into it" % name)
                    continue
                for path in directory_files(name):
                    add(path)
            elif os.path.isfile(name):
                add(name)
            elif os.path.exists(name):
                errors.append("%s: not a regular file" % name)
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
    installed = {name for name, _render in renderers()}
    if installed != set(RENDERER_NAMES):
        if not args.check:
            print(
                "error: rewriting needs Python-Markdown and markdown-it-py to prove the reflow "
                "renders the same: " + INSTALL_RENDERERS,
                file=sys.stderr,
            )
            return 2
        print(
            "warning: without Python-Markdown and markdown-it-py --check cannot prove the "
            "reflow renders the same: " + INSTALL_RENDERERS,
            file=sys.stderr,
        )
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
