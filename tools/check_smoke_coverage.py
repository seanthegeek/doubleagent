#!/usr/bin/env python3
"""Check that both smoke tests exercise every catalog table line, naming each row by full path.

`collectors/tests/smoke.sh` and `collectors/tests/smoke.ps1` are read as text and evaluated only
as far as their fixture writes, `run`/`Run` blocks and `check`/`Check` lines need, so the
checker knows which image paths exist and what each check asserts about which manifest row. A
path is taken relative to each collection base it lies under: a user home, a project directory
or a volume's `_data`. Every rule holds in each file; a finding's detail is `<file> <subject>`.

- `secret-check-missing`: a `SECRET_GLOBS` line with no check, in a run without `--no-secrets`
  or `--full`, asserting `secret: true` on a path the glob matches.
- `nosecrets-check-missing`: a `SECRET_GLOBS` line with no check, in a `--no-secrets` run,
  asserting `skipped_secret` on a path the glob matches.
- `exclusion-check-missing`: an `EXCLUDES` line with no check, in a run without `--full`,
  asserting `skipped_excluded` on a path the glob matches.
- `fixture-missing`: a `CATALOG` or `PROJECT_CATALOG` line that matches no fixture path; its
  subject is the table, `home` or `project`, then the glob, as one glob may be in both.
- `fixture-name-undocumented`: a fixture file a table line claims, outside every exclusion,
  whose relative path no code span of the claiming agent's research document names.
- `check-by-suffix`: a `smoke.ps1` check naming a row by a suffix: `-like '*...'` or `EndsWith`
  on a row's path, or a row argument whose first segment is no directory of the image root.
- `framework-nosecrets-missing`: an agent with a `secret` case in the frameworks case list
  (`CASES_FW`, `$CasesFrameworks`) and no `--no-secrets` check of `skipped_secret` on one.
- `check-path-unresolved`: a variable in a check or fixture path that no assignment from
  literals resolves, once per variable.
- `check-unparsed`: a check whose name, expression or row argument no reader form accepts.

Usage: tools/check_smoke_coverage.py [--update-baseline]
"""

from __future__ import annotations

import fnmatch
import functools
import pathlib
import posixpath
import re
import sys
import warnings
from collections.abc import Callable, Iterable, Iterator
from dataclasses import dataclass, field, replace

import drift_common

CHECKER = "check_smoke_coverage"
SMOKE_SH = "collectors/tests/smoke.sh"
SMOKE_PS1 = "collectors/tests/smoke.ps1"
# Both smoke tests build their images under one temporary work directory, whose path is only
# known at run time; the readers stand this token in for it.
WORK = "@work"
# The frameworks case list of each smoke test, as the variable holding it (PowerShell names are
# compared in lower case).
FRAMEWORK_CASES = {SMOKE_SH: "CASES_FW", SMOKE_PS1: "casesframeworks"}
# First-level image directories that are system trees, never a project directory.
SYSTEM_DIRECTORIES = frozenset(
    {"Users", "home", "root", "usr", "var", "etc", "srv", "nonexistent", "ProgramData", "Windows"}
)
# A reader writes a piece of a value it cannot know as the unresolved name between two U+0001.
MISSING = re.compile("\x01([^\x01]*)\x01")
RUN_BANNER = re.compile(r"^#\s*----")


# --- The model both readers fill ------------------------------------------------------------


@dataclass(frozen=True)
class RowReference:
    """A manifest row a check names: the image path and the argument as the check wrote it."""

    path: str
    argument: str


@dataclass(frozen=True)
class Assertion:
    """What one check asserts about the manifest row of one image path.

    `secret` is True or False when the check asserts the row's `secret` field, else None;
    `status` the status it asserts, or ""; `negated` marks `!=`, `-ne` or a leading `!`.
    """

    row: RowReference
    secret: bool | None = None
    status: str = ""
    negated: bool = False


@dataclass(frozen=True)
class Run:
    """A collector run a smoke test makes, from its `run`/`Run` line to the next one."""

    name: str
    line: int
    no_secrets: bool
    full: bool


@dataclass
class CheckLine:
    """One check: its name, its line and what it asserts."""

    name: str
    line: int
    assertions: list[Assertion]


@dataclass
class Smoke:
    """What one smoke test writes into its images and what its checks assert.

    `fixtures` maps each image path written to its first line, `files` holds those written
    as files; `rows` holds every row a check or a row variable names, with its line;
    `values` the first value each variable was given.
    """

    path: str
    case_insensitive: bool
    fixtures: dict[str, int] = field(default_factory=dict)
    files: set[str] = field(default_factory=set)
    checks: list[CheckLine] = field(default_factory=list)
    runs: list[Run] = field(default_factory=list)
    banners: list[int] = field(default_factory=list)
    rows: list[tuple[int, RowReference]] = field(default_factory=list)
    suffix_patterns: list[tuple[int, str]] = field(default_factory=list)
    unresolved: dict[str, int] = field(default_factory=dict)
    unparsed: list[tuple[int, str]] = field(default_factory=list)
    values: dict[str, object] = field(default_factory=dict)

    @property
    def label(self) -> str:
        """Return the file name the findings name, `smoke.sh` or `smoke.ps1`."""
        return posixpath.basename(self.path)

    def add_fixture(self, path: str, line: int, is_file: bool) -> None:
        """Record an image path the test writes; paths outside every image are ignored."""
        if image_path(path) is None:
            return
        path = path.rstrip("/")
        self.fixtures.setdefault(path, line)
        if is_file:
            self.files.add(path)

    def note_unresolved(self, names: Iterable[str], line: int) -> None:
        """Record each variable a path needed and could not resolve, at its first line."""
        for name in names:
            self.unresolved.setdefault(name, line)

    def run_at(self, line: int) -> Run | None:
        """Return the run whose block holds a line, or None.

        A run's block runs from its `run`/`Run` line to the next one or the next `# ----`
        banner, whichever comes first.
        """
        current = None
        for candidate in self.runs:
            if candidate.line <= line:
                current = candidate
        if current is None:
            return None
        if any(current.line < banner <= line for banner in self.banners):
            return None
        return current


def image_path(path: str) -> tuple[str, str] | None:
    """Return (image, path inside it) for a path under an image root, else None.

    An image root is a directory of the work directory whose name holds `root` (`root`,
    `root71`, `linkroot`); `@work/out/default` is in no image.
    """
    parts = path.split("/", 2)
    if len(parts) < 3 or parts[0] != WORK or "root" not in parts[1]:
        return None
    return "%s/%s" % (WORK, parts[1]), parts[2].rstrip("/")


def missing_names(text: str) -> list[str]:
    """Return the names a reader marked as unresolved in an expanded text, in order."""
    return MISSING.findall(text)


def assertion_from_text(row: RowReference, text: str, negated: bool = False) -> Assertion:
    """Read what a manifest-line pattern asserts: `"secret":true|false` and `"status":"<s>"`."""
    secret = True if '"secret":true' in text else False if '"secret":false' in text else None
    status = re.search(r'"status":"([a-z_]+)"', text)
    return Assertion(row, secret, status.group(1) if status else "", negated)


# --- sh: words, commands and expansion ------------------------------------------------------


@dataclass(frozen=True)
class ShellToken:
    """A token of sh source: a `word` (quotes kept), an `operator` or a `newline`.

    A `<<` operator carries the here-document's body in `heredoc`.
    """

    kind: str
    text: str
    line: int
    heredoc: str | None = None


SHELL_OPERATORS = ("&&", "||", ";;", "<<-", "<<", ">>", ">&", ">|", ";", "|", "&", ">", "<")
SHELL_OPERATORS += ("(", ")")
WORD_BREAK = " \t\r\n;&|<>()"


def _double_quote_end(text: str, index: int) -> int:
    """Return the index after the `"` closing a double-quoted string that starts at `index`."""
    while index < len(text):
        character = text[index]
        if character == "\\":
            index += 2
        elif character == '"':
            return index + 1
        elif text.startswith("$(", index):
            index = _parenthesis_end(text, index + 2)
        elif character == "`":
            index = _backquote_end(text, index + 1)
        else:
            index += 1
    return index


def _parenthesis_end(text: str, index: int) -> int:
    """Return the index after the `)` closing a group whose body starts at `index`."""
    depth = 1
    while index < len(text):
        character = text[index]
        if character == "\\":
            index += 2
            continue
        if character == "'":
            close = text.find("'", index + 1)
            index = len(text) if close < 0 else close + 1
            continue
        if character == '"':
            index = _double_quote_end(text, index + 1)
            continue
        depth += 1 if character == "(" else -1 if character == ")" else 0
        index += 1
        if depth == 0:
            return index
    return index


def _backquote_end(text: str, index: int) -> int:
    """Return the index after the backquote closing a substitution whose body starts here."""
    while index < len(text) and text[index] != "`":
        index += 2 if text[index] == "\\" else 1
    return index + 1


def _word_end(text: str, index: int) -> int:
    """Return the index where the sh word starting at `index` ends."""
    while index < len(text):
        character = text[index]
        if character in WORD_BREAK:
            return index
        if character == "\\":
            index += 2
        elif character == "'":
            close = text.find("'", index + 1)
            index = len(text) if close < 0 else close + 1
        elif character == '"':
            index = _double_quote_end(text, index + 1)
        elif character == "`":
            index = _backquote_end(text, index + 1)
        elif text.startswith("$(", index):
            index = _parenthesis_end(text, index + 2)
        elif text.startswith("${", index):
            close = text.find("}", index + 2)
            index = len(text) if close < 0 else close + 1
        else:
            index += 1
    return index


def _heredoc_body(text: str, index: int, label: str, strip_tabs: bool) -> tuple[str, int]:
    """Return a here-document's body and the index after its closing label line."""
    lines: list[str] = []
    while index < len(text):
        end = text.find("\n", index)
        end = len(text) if end < 0 else end
        line = text[index:end]
        index = end + 1
        if (line.lstrip("\t") if strip_tabs else line) == label:
            break
        lines.append(line)
    return "".join(line + "\n" for line in lines), index


def shell_tokens(text: str) -> list[ShellToken]:
    """Split sh source into words, operators and newlines, as the shell reads it.

    Quotes, `$(...)`, `${...}` and backquotes stay inside their word; a `#` starting a word
    begins a comment; a backslash-newline joins lines; the lines after a `<<LABEL` operator
    up to the line `LABEL` become its `heredoc`; digits right before `>` or `<` (a file
    descriptor) are dropped.
    """
    tokens: list[ShellToken] = []
    pending: list[tuple[int, str, bool]] = []
    index, line = 0, 1
    while index < len(text):
        character = text[index]
        if character in " \t\r":
            index += 1
        elif text.startswith("\\\n", index):
            index, line = index + 2, line + 1
        elif character == "\n":
            tokens.append(ShellToken("newline", "\n", line))
            index, line = index + 1, line + 1
            for position, label, strip_tabs in pending:
                body, after = _heredoc_body(text, index, label, strip_tabs)
                line += text.count("\n", index, after)
                index = after
                tokens[position] = replace(tokens[position], heredoc=body)
            pending = []
        elif character == "#":
            end = text.find("\n", index)
            index = len(text) if end < 0 else end
        else:
            operator = next((item for item in SHELL_OPERATORS if text.startswith(item, index)), "")
            if operator:
                tokens.append(ShellToken("operator", operator, line))
                index += len(operator)
                if operator in ("<<", "<<-"):
                    while index < len(text) and text[index] in " \t":
                        index += 1
                    end = _word_end(text, index)
                    label = text[index:end].replace("'", "").replace('"', "")
                    pending.append((len(tokens) - 1, label, operator == "<<-"))
                    index = end
                continue
            end = _word_end(text, index)
            word = text[index:end]
            if not (word.isdigit() and end < len(text) and text[end] in "<>"):
                tokens.append(ShellToken("word", word, line))
            line += word.count("\n")
            index = end
    return tokens


ROW_MARKER = "\x02row\x02"


def _remove_pattern(value: str, operator: str, pattern: str) -> str:
    """Apply `${NAME%pattern}` and its `%%`, `#` and `##` forms to a value."""
    positions = range(len(value) + 1)
    if operator == "%":
        cuts = [value[:i] for i in reversed(positions) if fnmatch.fnmatchcase(value[i:], pattern)]
    elif operator == "%%":
        cuts = [value[:i] for i in positions if fnmatch.fnmatchcase(value[i:], pattern)]
    elif operator == "#":
        cuts = [value[i:] for i in positions if fnmatch.fnmatchcase(value[:i], pattern)]
    else:
        cuts = [value[i:] for i in reversed(positions) if fnmatch.fnmatchcase(value[:i], pattern)]
    return cuts[0] if cuts else value


def _substitute(inner: str, variables: dict[str, str]) -> str:
    """Return the value of a command substitution `$(inner)` the reader can know.

    `$(dirname "path")` is the path's directory and `$(row "path")` a row reference; any other
    command gives a missing-name marker naming the command.
    """
    words = [token.text for token in shell_tokens(inner)]
    if len(words) == 2 and words[0] in ("dirname", "row"):
        value = shell_expand(words[1], variables)
        if missing_names(value):
            return value
        return posixpath.dirname(value) or "." if words[0] == "dirname" else ROW_MARKER + value
    return "\x01$(%s)\x01" % (words[0] if words else "")


def _expand_parameter(raw: str, index: int, variables: dict[str, str]) -> tuple[str, int]:
    """Expand the `$` expression at `index`; return its value and the index after it."""
    if raw.startswith("$((", index):
        return "\x01$((...))\x01", _parenthesis_end(raw, index + 2)
    if raw.startswith("$(", index):
        end = _parenthesis_end(raw, index + 2)
        return _substitute(raw[index + 2 : end - 1], variables), end
    if raw.startswith("${", index):
        close = raw.find("}", index + 2)
        close = len(raw) if close < 0 else close
        body = raw[index + 2 : close]
        match = re.match(r"([A-Za-z_][A-Za-z0-9_]*)(:-|%%|%|##|#)?(.*)$", body, re.S)
        if not match:
            return "\x01${%s}\x01" % body, close + 1
        name, operator, word = match.groups()
        value = variables.get(name)
        if operator == ":-":
            return (value if value else shell_expand(word, variables)), close + 1
        if value is None:
            return "\x01%s\x01" % name, close + 1
        if operator:
            value = _remove_pattern(value, operator, shell_expand(word, variables))
        return value, close + 1
    match = re.compile(r"[A-Za-z_][A-Za-z0-9_]*").match(raw, index + 1)
    if match:
        value = variables.get(match.group(0))
        return (value if value is not None else "\x01%s\x01" % match.group(0)), match.end()
    following = raw[index + 1 : index + 2]
    if following and (following.isdigit() or following in "?#@*$!-"):
        return "\x01$%s\x01" % following, index + 2
    return "$", index + 1


def shell_expand(raw: str, variables: dict[str, str]) -> str:
    """Expand one sh word as the shell would, without field splitting or globbing.

    Single quotes keep their text; double quotes and bare text expand `$NAME`, `${NAME}`,
    `${NAME%pattern}` (and `%%`, `#`, `##`, `:-`) and `$(dirname ...)`; a backslash quotes the
    next character, inside double quotes only before `$`, a backquote, `"`, a backslash or a
    newline.
    A name `variables` lacks, another command substitution or a special parameter is kept as
    a marker `missing_names` returns.
    """
    out: list[str] = []
    index, quoted = 0, False
    while index < len(raw):
        character = raw[index]
        if character == "'" and not quoted:
            close = raw.find("'", index + 1)
            close = len(raw) if close < 0 else close
            out.append(raw[index + 1 : close])
            index = close + 1
        elif character == '"':
            quoted, index = not quoted, index + 1
        elif character == "\\":
            following = raw[index + 1 : index + 2]
            if not quoted or (following and following in '$`"\\\n'):
                out.append("" if following == "\n" else following)
                index += 2
            else:
                out.append("\\")
                index += 1
        elif character == "$":
            value, index = _expand_parameter(raw, index, variables)
            out.append(value)
        elif character == "`":
            out.append("\x01`...`\x01")
            index = _backquote_end(raw, index + 1)
        else:
            out.append(character)
            index += 1
    return "".join(out)


@dataclass
class ShellCommand:
    """A simple command: its assignments, words and redirections, at its first line."""

    line: int
    assignments: list[ShellToken] = field(default_factory=list)
    words: list[ShellToken] = field(default_factory=list)
    redirects: list[tuple[ShellToken, ShellToken | None]] = field(default_factory=list)


@dataclass
class ShellCompound:
    """A `for`, `while`, `if`, `case`, group or function definition, with its parts.

    `lists` holds the command lists in source order (conditions and bodies); a `case` keeps
    each arm's patterns beside its body in `arms`; `redirects` are those after its end.
    """

    kind: str
    line: int
    name: str = ""
    words: list[ShellToken] = field(default_factory=list)
    lists: list[list[list[tuple[str, ShellPipeline]]]] = field(default_factory=list)
    arms: list[tuple[list[ShellToken], list[list[tuple[str, ShellPipeline]]]]] = field(
        default_factory=list
    )
    redirects: list[tuple[ShellToken, ShellToken | None]] = field(default_factory=list)


@dataclass
class ShellPipeline:
    """Commands joined by `|`, with a leading `!` noted."""

    commands: list[ShellCommand | ShellCompound]
    negated: bool = False


ShellList = list[list[tuple[str, ShellPipeline]]]
REDIRECTS = {">", ">>", ">|", ">&", "<", "<<", "<<-"}
ASSIGNMENT = re.compile(r"^([A-Za-z_][A-Za-z0-9_]*)=")


class _ShellParser:
    """A recursive-descent reader of the sh grammar the smoke test uses."""

    def __init__(self, tokens: list[ShellToken]) -> None:
        """Start at the first token."""
        self.tokens = tokens
        self.position = 0

    def peek(self) -> ShellToken | None:
        """Return the next token, or None at the end."""
        return self.tokens[self.position] if self.position < len(self.tokens) else None

    def take(self) -> ShellToken:
        """Consume and return the next token."""
        self.position += 1
        return self.tokens[self.position - 1]

    def at_word(self) -> bool:
        """Return whether the next token is a word."""
        token = self.peek()
        return token is not None and token.kind == "word"

    def next_is(self, kind: str, *texts: str) -> bool:
        """Return whether the next token has this kind and one of these texts."""
        token = self.peek()
        return token is not None and token.kind == kind and token.text in texts

    def skip(self, *texts: str) -> None:
        """Consume newlines and the given operators."""
        while self.next_is("newline", "\n") or self.next_is("operator", *texts):
            self.take()

    def command_list(self, stops: set[str]) -> ShellList:
        """Read and-or lists up to a reserved word or operator in `stops`, or the end."""
        items: ShellList = []
        while True:
            self.skip(";", "&")
            token = self.peek()
            if token is None or token.text in stops:
                return items
            start = self.position
            items.append(self.and_or(stops))
            if self.position == start:
                self.take()

    def and_or(self, stops: set[str]) -> list[tuple[str, ShellPipeline]]:
        """Read pipelines joined by `&&` and `||`."""
        parts = [("", self.pipeline(stops))]
        while self.next_is("operator", "&&", "||"):
            operator = self.take().text
            self.skip()
            parts.append((operator, self.pipeline(stops)))
        return parts

    def pipeline(self, stops: set[str]) -> ShellPipeline:
        """Read commands joined by `|`."""
        negated = self.next_is("word", "!")
        if negated:
            self.take()
        commands = [self.command(stops)]
        while self.next_is("operator", "|"):
            self.take()
            self.skip()
            commands.append(self.command(stops))
        return ShellPipeline(commands, negated)

    def command(self, stops: set[str]) -> ShellCommand | ShellCompound:
        """Read one simple or compound command."""
        token = self.peek()
        if token is not None and token.kind == "word":
            reader = {
                "for": self.for_loop,
                "while": self.while_loop,
                "until": self.while_loop,
                "if": self.if_clause,
                "case": self.case_clause,
                "{": self.group,
            }.get(token.text)
            if reader:
                compound = reader()
                compound.redirects = self.redirects()
                return compound
            following = self.tokens[self.position + 1 : self.position + 3]
            if [item.text for item in following] == ["(", ")"]:
                self.position += 3
                self.skip()
                self.command(stops)
                return ShellCompound("function", token.line, token.text)
        if self.next_is("operator", "("):
            line = self.take().line
            body = self.command_list({")"})
            if self.next_is("operator", ")"):
                self.take()
            return ShellCompound("group", line, lists=[body], redirects=self.redirects())
        return self.simple(stops)

    def redirects(self) -> list[tuple[ShellToken, ShellToken | None]]:
        """Read the redirections after a compound command."""
        found: list[tuple[ShellToken, ShellToken | None]] = []
        while self.next_is("operator", *REDIRECTS):
            operator = self.take()
            target = None
            if operator.text not in ("<<", "<<-") and self.at_word():
                target = self.take()
            found.append((operator, target))
        return found

    def simple(self, stops: set[str]) -> ShellCommand:
        """Read a simple command: assignments, words and redirections."""
        token = self.peek()
        command = ShellCommand(token.line if token else 0)
        while (token := self.peek()) is not None and token.kind != "newline":
            if token.kind == "operator":
                if token.text not in REDIRECTS:
                    break
                operator = self.take()
                target = None
                if operator.text not in ("<<", "<<-") and self.at_word():
                    target = self.take()
                command.redirects.append((operator, target))
                continue
            self.take()
            if not command.words and ASSIGNMENT.match(token.text):
                command.assignments.append(token)
            else:
                command.words.append(token)
        return command

    def expect(self, word: str) -> None:
        """Consume a reserved word if it is next."""
        self.skip(";", "&")
        if self.next_is("word", word):
            self.take()

    def for_loop(self) -> ShellCompound:
        """Read `for NAME in WORDS; do LIST done`."""
        line = self.take().line
        name = self.take().text if self.at_word() else ""
        words: list[ShellToken] = []
        if self.next_is("word", "in"):
            self.take()
            while (token := self.peek()) is not None and token.kind == "word":
                words.append(self.take())
        self.expect("do")
        body = self.command_list({"done"})
        self.expect("done")
        return ShellCompound("for", line, name, words, [body])

    def while_loop(self) -> ShellCompound:
        """Read `while LIST; do LIST done`."""
        line = self.take().line
        condition = self.command_list({"do"})
        self.expect("do")
        body = self.command_list({"done"})
        self.expect("done")
        return ShellCompound("while", line, lists=[condition, body])

    def if_clause(self) -> ShellCompound:
        """Read `if LIST; then LIST [elif ...] [else LIST] fi`."""
        line = self.take().line
        lists = [self.command_list({"then"})]
        self.expect("then")
        lists.append(self.command_list({"elif", "else", "fi"}))
        while self.next_is("word", "elif"):
            self.take()
            lists.append(self.command_list({"then"}))
            self.expect("then")
            lists.append(self.command_list({"elif", "else", "fi"}))
        if self.next_is("word", "else"):
            self.take()
            lists.append(self.command_list({"fi"}))
        self.expect("fi")
        return ShellCompound("if", line, lists=lists)

    def case_clause(self) -> ShellCompound:
        """Read `case WORD in PATTERN) LIST ;; ... esac`."""
        line = self.take().line
        subject = [self.take()] if self.at_word() else []
        self.expect("in")
        compound = ShellCompound("case", line, words=subject)
        while True:
            self.skip()
            if self.peek() is None or self.next_is("word", "esac"):
                break
            if self.next_is("operator", "("):
                self.take()
            patterns: list[ShellToken] = []
            while (token := self.peek()) is not None and not self.next_is("operator", ")"):
                token = self.take()
                if token.kind == "word":
                    patterns.append(token)
            if self.peek() is not None:
                self.take()
            body = self.command_list({";;", "esac"})
            if self.next_is("operator", ";;"):
                self.take()
            compound.arms.append((patterns, body))
        self.expect("esac")
        return compound

    def group(self) -> ShellCompound:
        """Read `{ LIST }`."""
        line = self.take().line
        body = self.command_list({"}"})
        self.expect("}")
        return ShellCompound("group", line, lists=[body])


class _ContinueLoop(Exception):
    """Raised by `continue` inside a loop the reader runs."""


@dataclass
class _ShellState:
    """The variables, written texts and pending case assertion while a smoke.sh is read."""

    variables: dict[str, str] = field(default_factory=lambda: {"WORK": WORK})
    texts: dict[str, str] = field(default_factory=dict)
    case_assertion: Assertion | None = None


class _ShellReader:
    """Run the parts of smoke.sh that write fixtures, start runs and make checks."""

    def __init__(self, smoke: Smoke) -> None:
        """Start with only the work directory known."""
        self.smoke = smoke
        self.state = _ShellState()

    def expand(self, token: ShellToken) -> str:
        """Expand a word with the variables known now."""
        return shell_expand(token.text, self.state.variables)

    def assign(self, token: ShellToken) -> None:
        """Set a variable; the work directory stays the reader's token."""
        name, _, raw = token.text.partition("=")
        if name == "WORK":
            return
        value = shell_expand(raw, self.state.variables)
        if missing_names(value):
            value = "\x01%s\x01" % name
        self.state.variables[name] = value
        self.smoke.values.setdefault(name, value)

    def fixture(self, token: ShellToken, is_file: bool) -> None:
        """Record a written path, or the variables it could not resolve."""
        value = self.expand(token)
        names = missing_names(value)
        if names:
            if image_path(value.split("\x01", 1)[0]) is not None or value.startswith("\x01"):
                self.smoke.note_unresolved(names, token.line)
            return
        self.smoke.add_fixture(value, token.line, is_file)

    def run_list(self, items: ShellList) -> None:
        """Run and-or lists: `&&` and `||` follow a test the reader can decide."""
        for and_or in items:
            result: bool | None = None
            for operator, pipeline in and_or:
                if (operator == "&&" and result is False) or (operator == "||" and result):
                    continue
                if operator == "||" and result is None:
                    continue
                result = self.run_pipeline(pipeline)

    def run_pipeline(self, pipeline: ShellPipeline) -> bool | None:
        """Run each command of a pipeline; a `printf` piped into `while read` feeds its loop."""
        commands = pipeline.commands
        data = None
        if len(commands) == 2 and isinstance(commands[1], ShellCompound):
            data = self.printed_text(commands[0])
        result = None
        for command in commands:
            if isinstance(command, ShellCompound):
                self.run_compound(command, data)
            else:
                result = self.run_simple(command)
        if result is not None and pipeline.negated:
            result = not result
        return result if len(commands) == 1 else None

    def printed_text(self, command: ShellCommand | ShellCompound) -> str | None:
        """Return what a `printf` of one word with the format `%s` and a newline prints, or None."""
        if not isinstance(command, ShellCommand) or len(command.words) != 3:
            return None
        if [self.expand(word) for word in command.words[:2]] != ["printf", "%s\\n"]:
            return None
        return self.expand(command.words[2]) + "\n"

    def run_compound(self, compound: ShellCompound, data: str | None = None) -> None:
        """Run a loop, every branch of an `if`, the matching `case` arm or a group."""
        if compound.kind == "for":
            values = [self.expand(word) for word in compound.words]
            names = [name for value in values for name in missing_names(value)]
            if names:
                self.smoke.note_unresolved(names, compound.line)
                return
            for value in values:
                self.state.variables[compound.name] = value
                self.loop_body(compound.lists[0])
        elif compound.kind == "while":
            self.run_while(compound, data)
        elif compound.kind == "case":
            self.run_case(compound)
        elif compound.kind != "function":
            for items in compound.lists:
                self.run_list(items)

    def loop_body(self, body: ShellList) -> None:
        """Run a loop body once, stopping at `continue`."""
        try:
            self.run_list(body)
        except _ContinueLoop:
            pass

    def run_while(self, compound: ShellCompound, data: str | None) -> None:
        """Run a `while read` loop once per line of its data, any other `while` once."""
        condition, body = compound.lists[0], compound.lists[1]
        reads = [
            command
            for and_or in condition
            for _, pipeline in and_or
            for command in pipeline.commands
            if isinstance(command, ShellCommand) and command.words
            if command.words[0].text == "read"
        ]
        if not reads:
            self.run_list(condition)
            self.loop_body(body)
            return
        if data is None:
            data = self.redirected_text(compound)
        if data is None:
            return
        read = reads[0]
        separators = None
        for assignment in read.assignments:
            if assignment.text.startswith("IFS="):
                separators = shell_expand(assignment.text[4:], self.state.variables)
        names = [word.text for word in read.words[1:] if not word.text.startswith("-")]
        lines = data.split("\n")
        for line in lines[:-1] if data.endswith("\n") else lines:
            for name, value in zip(names, read_fields(line, separators, len(names)), strict=True):
                self.state.variables[name] = value
            self.loop_body(body)

    def redirected_text(self, compound: ShellCompound) -> str | None:
        """Return the text a loop reads through `done <FILE`, if the test wrote it."""
        for operator, target in compound.redirects:
            if operator.text == "<" and target is not None:
                path = self.expand(target)
                if path in self.state.texts:
                    return self.state.texts[path]
                self.smoke.note_unresolved([target.text], target.line)
        return None

    def run_case(self, compound: ShellCompound) -> None:
        """Run a `case`: on a row reference, each arm calling `pass` asserts its pattern."""
        subject = self.expand(compound.words[0]) if compound.words else "\x01case\x01"
        if subject.startswith(ROW_MARKER):
            row = RowReference(subject[len(ROW_MARKER) :], compound.words[0].text)
            for patterns, body in compound.arms:
                text = "|".join(self.expand(pattern) for pattern in patterns)
                self.state.case_assertion = assertion_from_text(row, text)
                self.run_list(body)
            self.state.case_assertion = None
            return
        known = not missing_names(subject)
        for patterns, body in compound.arms:
            values = [self.expand(pattern) for pattern in patterns]
            if not known or any(fnmatch.fnmatchcase(subject, value) for value in values):
                self.run_list(body)
                if known:
                    return

    def run_simple(self, command: ShellCommand) -> bool | None:
        """Run a simple command the reader understands; return a decided test's result."""
        if not command.words:
            for assignment in command.assignments:
                self.assign(assignment)
            return None
        name = self.expand(command.words[0])
        arguments = command.words[1:]
        if name == "check":
            self.add_check(command)
        elif name == "run" and arguments:
            texts = [self.expand(word) for word in arguments]
            run = Run(texts[0], command.line, "--no-secrets" in texts, "--full" in texts)
            self.smoke.runs.append(run)
        elif name == "pass" and self.state.case_assertion is not None and arguments:
            check_name = self.expand(arguments[0])
            self.smoke.checks.append(
                CheckLine(check_name, command.line, [self.state.case_assertion])
            )
            self.smoke.rows.append((command.line, self.state.case_assertion.row))
        elif name == "continue":
            raise _ContinueLoop
        elif name in ("[", "test"):
            return shell_test([self.expand(word) for word in arguments])
        elif name == "mkdir":
            for word in arguments:
                if not word.text.startswith("-"):
                    self.fixture(word, False)
        elif name in ("touch", "mkfifo"):
            for word in arguments:
                if not word.text.startswith("-"):
                    self.fixture(word, True)
        elif name in ("ln", "cp") and arguments:
            self.fixture(arguments[-1], True)
        self.write_redirects(command, name)
        return None

    def write_redirects(self, command: ShellCommand, name: str) -> None:
        """Record each file a `>` or `>>` writes, and the text `cat <<` or `printf` gives it."""
        heredoc = next((item.heredoc for item, _ in command.redirects if item.heredoc), None)
        for operator, target in command.redirects:
            if operator.text not in (">", ">>", ">|") or target is None:
                continue
            path = self.expand(target)
            if path == "/dev/null":
                continue
            if name == "cat" and heredoc is not None:
                self.state.texts[path] = heredoc
            printed = self.printed_text(command)
            if printed is not None:
                self.state.texts[path] = printed
            self.fixture(target, True)

    def add_check(self, command: ShellCommand) -> None:
        """Record a `check "name" "expression"` line and what its expression asserts."""
        names = [MISSING.sub(r"$\1", self.expand(word)) for word in command.words[1:2]]
        name = drift_common.collapse(names[0] if names else "") or "check"
        if len(command.words) != 3:
            self.smoke.unparsed.append((command.line, name))
            return
        expression = self.expand(command.words[2])
        assertions, unparsed = shell_check_assertions(expression)
        if unparsed:
            self.smoke.unparsed.append((command.line, name))
        kept: list[Assertion] = []
        for assertion in assertions:
            self.smoke.rows.append((command.line, assertion.row))
            names = missing_names(assertion.row.path)
            if names:
                self.smoke.note_unresolved(names, command.line)
            else:
                kept.append(assertion)
        self.smoke.checks.append(CheckLine(name, command.line, kept))


def read_fields(line: str, separators: str | None, count: int) -> list[str]:
    """Split a line into `count` fields as `read` does with `IFS` set to `separators`.

    Unset `IFS` splits on blanks and trims them; an empty one keeps the line whole; other
    characters each end a field; the last field keeps the rest of the line.
    """
    if count <= 0:
        return []
    if separators is None:
        fields = line.strip(" \t").split(None, count - 1)
    elif separators == "":
        fields = [line]
    else:
        fields = re.split("[%s]" % re.escape(separators), line, maxsplit=count - 1)
    return (fields + [""] * count)[:count]


def shell_test(arguments: list[str]) -> bool | None:
    """Decide `[ -n X ]`, `[ -z X ]`, `[ A = B ]` and `[ A != B ]`; None for anything else."""
    if arguments and arguments[-1] == "]":
        arguments = arguments[:-1]
    if any(missing_names(argument) for argument in arguments):
        return None
    if len(arguments) == 2 and arguments[0] in ("-n", "-z"):
        return (arguments[1] != "") == (arguments[0] == "-n")
    if len(arguments) == 3 and arguments[1] in ("=", "!="):
        return (arguments[0] == arguments[2]) == (arguments[1] == "=")
    return None


SHELL_ARGUMENT = r"""(?:"[^"]*"|[^\s|"';&)]+)"""
ROW_GREP = re.compile(r"""(!\s*)?\brow\s+(%s)\s*\|\s*grep\s+-qF?\s+'([^']*)'""" % SHELL_ARGUMENT)
STATUS_COMPARE = re.compile(r"""\$\(status_of\s+(%s)\)"\s*(!?=)\s*([a-z_]+)""" % SHELL_ARGUMENT)
ROW_ABSENT = re.compile(r"""\[\s+-z\s+"\$\((?:row|status_of)\s+(%s)\)"\s+\]""" % SHELL_ARGUMENT)
ROW_READ = re.compile(r"""\brow\s+(%s)\s*\|\s*sed\b""" % SHELL_ARGUMENT)
ROW_CALL = re.compile(r"\b(?:row|status_of)\s+")


def shell_check_assertions(expression: str) -> tuple[list[Assertion], bool]:
    """Return what a smoke.sh check expression asserts, and whether a row call was unread.

    Accepted forms: `row "P" | grep -q 'PATTERN'` (a leading `!` negates it),
    `"$(status_of "P")" = STATUS` or `!=`, `[ -z "$(row "P")" ]` (an absence, asserting
    nothing), and `row "P" | sed` (a value read). A `row` or `status_of` call in any other form
    makes the check unparsed. Each path is expanded again, as the check's `eval` would.
    """
    assertions: list[Assertion] = []
    consumed: set[int] = set()
    for match in ROW_GREP.finditer(expression):
        row = RowReference(shell_expand(match.group(2), {}), match.group(2))
        assertions.append(assertion_from_text(row, match.group(3), bool(match.group(1))))
        consumed.add(match.start(0) + (len(match.group(1)) if match.group(1) else 0))
    for match in STATUS_COMPARE.finditer(expression):
        row = RowReference(shell_expand(match.group(1), {}), match.group(1))
        assertions.append(Assertion(row, None, match.group(3), match.group(2) == "!="))
        consumed.add(match.start(0) + 2)
    for match in list(ROW_ABSENT.finditer(expression)) + list(ROW_READ.finditer(expression)):
        call = ROW_CALL.search(expression, match.start(0))
        if call:
            consumed.add(call.start())
    unparsed = any(call.start() not in consumed for call in ROW_CALL.finditer(expression))
    return assertions, unparsed


def read_shell_smoke(text: str, path: str = SMOKE_SH) -> Smoke:
    """Read a smoke.sh: its fixtures, runs, checks and the variables they needed."""
    smoke = Smoke(path, case_insensitive=False)
    smoke.banners = [
        number for number, line in enumerate(text.split("\n"), 1) if RUN_BANNER.match(line)
    ]
    parser = _ShellParser(shell_tokens(text))
    _ShellReader(smoke).run_list(parser.command_list(set()))
    return smoke


# --- PowerShell: tokens, groups and values --------------------------------------------------


@dataclass(frozen=True)
class PowerShellToken:
    """A token of PowerShell source.

    `kind` is `newline`, `string` (`text` is its value), `expandable` (a double-quoted
    string; `text` is its raw body), `variable` (`text` is the lower-case name less any
    `script:` scope), `word` or `punct`; `start` and `end` are offsets into the source.
    """

    kind: str
    text: str
    line: int
    start: int
    end: int


@dataclass
class PowerShellGroup:
    """A bracketed group: `(`, `@(`, `$(`, `{`, `@{` or `[`, with its children."""

    opener: str
    children: list[PowerShellToken | PowerShellGroup]
    line: int
    start: int
    end: int


PowerShellNode = PowerShellToken | PowerShellGroup
POWERSHELL_PUNCTUATION = ("@(", "@{", "$(", "::", "+=", "(", ")", "{", "}", "[", "]", ",", ";")
POWERSHELL_PUNCTUATION += ("|", "=", "+", "!")
POWERSHELL_WORD_BREAK = " \t\r\n(){}[],;|=+'\"$#`"
ESCAPES = {"n": "\n", "t": "\t", "r": "\r", "0": "\0", "a": "\a"}


def _powershell_string_end(text: str, index: int) -> int:
    """Return the index after the `"` closing a double-quoted string starting at `index`."""
    while index < len(text):
        character = text[index]
        if character == "`" or text.startswith('""', index):
            index += 2
        elif character == '"':
            return index + 1
        elif text.startswith("$(", index):
            index = _powershell_group_end(text, index + 2)
        else:
            index += 1
    return index


def _powershell_group_end(text: str, index: int) -> int:
    """Return the index after the `)` closing a subexpression whose body starts at `index`."""
    depth = 1
    while index < len(text):
        character = text[index]
        if character == "'":
            close = text.find("'", index + 1)
            index = len(text) if close < 0 else close + 1
            continue
        if character == '"':
            index = _powershell_string_end(text, index + 1)
            continue
        depth += 1 if character == "(" else -1 if character == ")" else 0
        index += 1
        if depth == 0:
            return index
    return index


def _variable_name(raw: str) -> str:
    """Return a PowerShell variable name in lower case, less a `script:` or `global:` scope."""
    name = raw.lower()
    for scope in ("script:", "global:", "local:"):
        name = name.removeprefix(scope)
    return name


def powershell_tokens(text: str) -> list[PowerShellToken]:
    """Split PowerShell source into tokens.

    Single-quoted strings (`''` is a quote) and `@'...'@` here-strings give their value;
    double-quoted strings and `@"..."@` keep their raw body for interpolation; `#` comments,
    `<#...#>` blocks and a backtick before a newline are dropped; `$name`, `${name}` and
    `$script:name` are variables; a `.` right after `)`, `]` or a variable is punctuation.
    """
    tokens: list[PowerShellToken] = []
    index, line = 0, 1

    def add(kind: str, value: str, start: int, end: int) -> None:
        tokens.append(PowerShellToken(kind, value, line, start, end))

    while index < len(text):
        character, start = text[index], index
        if character in " \t\r":
            index += 1
        elif text.startswith("`\n", index) or text.startswith("`\r\n", index):
            index = text.index("\n", index) + 1
            line += 1
        elif character == "\n":
            add("newline", "\n", start, index + 1)
            index, line = index + 1, line + 1
        elif text.startswith("<#", index):
            end = text.find("#>", index)
            end = len(text) if end < 0 else end + 2
            line += text.count("\n", index, end)
            index = end
        elif character == "#":
            end = text.find("\n", index)
            index = len(text) if end < 0 else end
        elif text.startswith("@'\n", index) or text.startswith('@"\n', index):
            quote = text[index + 1]
            close = text.find("\n%s@" % quote, index + 2)
            close = len(text) if close < 0 else close
            body = text[index + 3 : close]
            index = close + 3
            add("string" if quote == "'" else "expandable", body, start, index)
            line += text.count("\n", start, index)
        elif character == "'":
            index = index + 1
            while index < len(text):
                if text.startswith("''", index):
                    index += 2
                elif text[index] == "'":
                    break
                else:
                    index += 1
            body = text[start + 1 : index].replace("''", "'")
            index += 1
            add("string", body, start, index)
            line += text.count("\n", start, index)
        elif character == '"':
            index = _powershell_string_end(text, index + 1)
            add("expandable", text[start + 1 : index - 1], start, index)
            line += text.count("\n", start, index)
        elif character == "$" and text.startswith("${", index):
            close = text.find("}", index)
            index = close + 1
            add("variable", _variable_name(text[start + 2 : close]), start, index)
        elif character == "$" and re.match(r"[A-Za-z_?]", text[index + 1 : index + 2]):
            match = re.compile(r"[A-Za-z0-9_?]+(?::[A-Za-z0-9_]+)?").match(text, index + 1)
            index = match.end() if match else index + 1
            add("variable", _variable_name(text[start + 1 : index]), start, index)
        else:
            follows = bool(tokens) and tokens[-1].end == index
            follows = follows and (tokens[-1].kind == "variable" or tokens[-1].text in (")", "]"))
            if character == "." and follows:
                add("punct", ".", start, index + 1)
                index += 1
                continue
            punctuation = next(
                (item for item in POWERSHELL_PUNCTUATION if text.startswith(item, index)), ""
            )
            if punctuation:
                index += len(punctuation)
                add("punct", punctuation, start, index)
                continue
            while index < len(text) and text[index] not in POWERSHELL_WORD_BREAK:
                index += 1
            index = max(index, start + 1)
            add("word", text[start:index], start, index)
    return tokens


CLOSERS = {"(": ")", "@(": ")", "$(": ")", "{": "}", "@{": "}", "[": "]"}


def powershell_groups(tokens: list[PowerShellToken]) -> list[PowerShellNode]:
    """Nest tokens into bracketed groups; a closer with no matching opener closes the nearest."""
    root: list[PowerShellNode] = []
    stack: list[PowerShellGroup] = []
    for token in tokens:
        target = stack[-1].children if stack else root
        if token.kind == "punct" and token.text in CLOSERS:
            stack.append(PowerShellGroup(token.text, [], token.line, token.start, token.end))
            target.append(stack[-1])
        elif token.kind == "punct" and token.text in (")", "}", "]") and stack:
            stack.pop().end = token.end
        else:
            target.append(token)
    return root


def powershell_statements(nodes: list[PowerShellNode]) -> list[list[PowerShellNode]]:
    """Split a node list into statements at newlines and semicolons outside groups."""
    statements: list[list[PowerShellNode]] = [[]]
    for node in nodes:
        is_token = isinstance(node, PowerShellToken)
        if is_token and node.kind in ("newline", "punct") and node.text in ("\n", ";"):
            statements.append([])
            continue
        statements[-1].append(node)
    return [statement for statement in statements if statement]


@dataclass(frozen=True)
class Unknown:
    """A value the reader cannot know, with the variables that made it so."""

    names: tuple[str, ...] = ()


PowerShellValue = str | tuple[str, ...] | RowReference | Unknown


def _join(left: str, right: str) -> str:
    """Join two path pieces with one slash, backslashes made slashes."""
    left, right = left.replace("\\", "/"), right.replace("\\", "/")
    return left.rstrip("/") + "/" + right.lstrip("/") if left else right


def _word_text(node: PowerShellNode) -> str:
    """Return a word or punctuation token's text in lower case, or "" for anything else."""
    if isinstance(node, PowerShellToken) and node.kind in ("punct", "word"):
        return node.text.lower()
    return ""


def _split_on(nodes: list[PowerShellNode], separator: str) -> list[list[PowerShellNode]]:
    """Split nodes at top-level punctuation or a word equal to `separator`, any case."""
    parts: list[list[PowerShellNode]] = [[]]
    for node in nodes:
        if _word_text(node) == separator:
            parts.append([])
        else:
            parts[-1].append(node)
    return parts


def node_text(nodes: list[PowerShellNode]) -> str:
    """Rebuild the text of nodes from their tokens, one space between tokens.

    A string is single-quoted again, a variable given back its `$`; used where the nodes may
    come from a subexpression, whose offsets are not into the file.
    """
    pieces: list[str] = []
    for node in nodes:
        if isinstance(node, PowerShellGroup):
            pieces.append(node.opener + " " + node_text(node.children) + " " + CLOSERS[node.opener])
        elif node.kind == "string":
            pieces.append("'%s'" % node.text.replace("'", "''"))
        elif node.kind == "expandable":
            pieces.append('"%s"' % node.text)
        elif node.kind == "variable":
            pieces.append("$" + node.text)
        elif node.kind != "newline":
            pieces.append(node.text)
    return " ".join(pieces)


class _PowerShellReader:
    """Evaluate the parts of smoke.ps1 that write fixtures, start runs and make checks."""

    def __init__(self, smoke: Smoke, text: str) -> None:
        """Start with only the work directory known."""
        self.smoke = smoke
        self.text = text
        self.variables: dict[str, PowerShellValue] = {"work": WORK}
        self.helpers: dict[str, tuple[list[str], str]] = {}

    def note(self, value: PowerShellValue, line: int, fallback: str) -> None:
        """Record the variables an unknown value needed, or the text when it names none."""
        if isinstance(value, Unknown):
            self.smoke.note_unresolved(value.names or (fallback,), line)

    def lookup(self, name: str) -> PowerShellValue:
        """Return a variable's value, Unknown naming it when it has none."""
        value = self.variables.get(name)
        if value is None or isinstance(value, Unknown):
            return Unknown((name,))
        return value

    def evaluate(self, nodes: list[PowerShellNode]) -> PowerShellValue:
        """Return the value of an expression: pipes, `-split`, `+`, then a single term."""
        nodes = [
            node
            for node in nodes
            if not (isinstance(node, PowerShellToken) and node.kind == "newline")
        ]
        if not nodes:
            return Unknown()
        stages = _split_on(nodes, "|")
        if len(stages) > 1:
            return self.pipeline(stages)
        split = _split_on(nodes, "-split")
        if len(split) == 2:
            return self.split(split[0], split[1])
        terms = _split_on(nodes, "+")
        if len(terms) > 1:
            values = [self.evaluate(term) for term in terms]
            unknown = [value for value in values if isinstance(value, Unknown)]
            if unknown:
                return Unknown(tuple(name for value in unknown for name in value.names))
            if all(isinstance(value, str) for value in values):
                return "".join(str(value) for value in values)
            if all(isinstance(value, tuple) for value in values):
                return tuple(item for value in values if isinstance(value, tuple) for item in value)
            return Unknown()
        return self.term(nodes)

    def pipeline(self, stages: list[list[PowerShellNode]]) -> PowerShellValue:
        """Evaluate `VALUE | Where-Object { $_ -ne '' }`, which drops empty items."""
        value = self.evaluate(stages[0])
        for stage in stages[1:]:
            text = node_text(stage)
            if not isinstance(value, tuple) or not re.match(
                r"Where-Object\s*\{\s*\$_\s+-ne\s+''\s*\}$", text
            ):
                return value if isinstance(value, Unknown) else Unknown()
            value = tuple(item for item in value if item != "")
        return value

    def split(self, left: list[PowerShellNode], right: list[PowerShellNode]) -> PowerShellValue:
        """Evaluate `VALUE -split 'regex'` with an optional `, limit`."""
        value = self.evaluate(left)
        arguments = _split_on(right, ",")
        separator = self.evaluate(arguments[0])
        if isinstance(value, Unknown):
            return value
        if not isinstance(value, str) or not isinstance(separator, str):
            return Unknown()
        limit = 0
        if len(arguments) > 1:
            limit_text = node_text(arguments[1])
            limit = int(limit_text) - 1 if limit_text.isdigit() else 0
        return tuple(re.split(separator, value, maxsplit=max(limit, 0)))

    def source(self, nodes: list[PowerShellNode]) -> str:
        """Return the source text a node list spans."""
        if not nodes:
            return ""
        return self.text[nodes[0].start : nodes[-1].end]

    def interpolate(self, raw: str) -> PowerShellValue:
        """Expand a double-quoted string's body: backtick escapes, `$name` and `$(...)`."""
        out: list[str] = []
        names: list[str] = []
        index = 0
        while index < len(raw):
            character = raw[index]
            if character == "`" and index + 1 < len(raw):
                out.append(ESCAPES.get(raw[index + 1], raw[index + 1]))
                index += 2
                continue
            if raw.startswith('""', index):
                out.append('"')
                index += 2
                continue
            value: PowerShellValue | None = None
            if raw.startswith("$(", index):
                end = _powershell_group_end(raw, index + 2)
                inner = raw[index + 2 : end - 1]
                value = self.evaluate(powershell_groups(powershell_tokens(inner)))
                index = end
            else:
                match = re.compile(
                    r"\$(?:\{([^}]*)\}|([A-Za-z_][A-Za-z0-9_]*(?::[A-Za-z0-9_]+)?))"
                ).match(raw, index)
                if match:
                    value = self.lookup(_variable_name(match.group(1) or match.group(2)))
                    index = match.end()
            if value is None:
                out.append(character)
                index += 1
            elif isinstance(value, str):
                out.append(value)
            elif isinstance(value, tuple):
                out.append(" ".join(value))
            else:
                names.extend(value.names if isinstance(value, Unknown) else ())
                if not isinstance(value, Unknown) or not value.names:
                    names.append("$(...)")
        return Unknown(tuple(names)) if names else "".join(out)

    def term(self, nodes: list[PowerShellNode]) -> PowerShellValue:
        """Evaluate one term, with an optional `[index]` after it."""
        if len(nodes) >= 2 and isinstance(nodes[-1], PowerShellGroup) and nodes[-1].opener == "[":
            base = self.term(nodes[:-1])
            index_text = node_text(nodes[-1].children)
            if isinstance(base, tuple) and index_text.isdigit():
                position = int(index_text)
                return base[position] if position < len(base) else ""
            return base if isinstance(base, Unknown) else Unknown()
        first = nodes[0]
        if len(nodes) == 1 and isinstance(first, PowerShellToken):
            if first.kind == "string":
                return first.text
            if first.kind == "expandable":
                return self.interpolate(first.text)
            if first.kind == "variable":
                return self.lookup(first.text)
            return Unknown()
        if isinstance(first, PowerShellGroup) and len(nodes) == 1:
            if first.opener == "@(":
                return self.array(first.children)
            if first.opener in ("(", "$("):
                return self.evaluate(first.children)
            return Unknown()
        return self.command(nodes)

    def array(self, children: list[PowerShellNode]) -> PowerShellValue:
        """Evaluate `@(...)`: its items, separated by commas or lines, flattened."""
        items: list[str] = []
        for statement in powershell_statements(children):
            has_split = len(_split_on(statement, "-split")) > 1
            for part in [statement] if has_split else _split_on(statement, ","):
                if not part:
                    continue
                value = self.evaluate(part)
                if isinstance(value, Unknown):
                    return value
                if isinstance(value, RowReference):
                    return Unknown()
                items.extend(value if isinstance(value, tuple) else (value,))
        return tuple(items)

    def command(self, nodes: list[PowerShellNode]) -> PowerShellValue:
        """Evaluate `Join-Path`, `ImagePath`, `LinkImagePath`, `Row` or `[IO.Path]::Combine`."""
        first = nodes[0]
        if isinstance(first, PowerShellGroup) and first.opener == "[":
            tail = [node for node in nodes[1:] if isinstance(node, PowerShellToken)]
            group = nodes[-1]
            if [token.text for token in tail] == ["::", "Combine"] and isinstance(
                group, PowerShellGroup
            ):
                values = [self.evaluate(part) for part in _split_on(group.children, ",")]
                return functools.reduce(_join_values, values)
            return Unknown()
        if not isinstance(first, PowerShellToken) or first.kind != "word":
            return Unknown()
        name = first.text.lower()
        arguments = [self.evaluate([node]) for node in nodes[1:]]
        if name == "join-path" and len(arguments) == 2:
            return _join_values(arguments[0], arguments[1])
        if name in ("imagepath", "linkimagepath") and len(arguments) == 1:
            root = self.lookup("root" if name == "imagepath" else "linkroot")
            return _join_values(root, arguments[0])
        if name == "row" and len(nodes) == 2:
            return self.row(nodes[1], first.line)
        return Unknown()

    def row(self, node: PowerShellNode, line: int) -> PowerShellValue:
        """Return the row reference `Row ARGUMENT` names under the current image root."""
        value = self.evaluate([node])
        argument = node_text([node])
        if not isinstance(value, str):
            self.note(value, line, argument)
            return Unknown()
        root = self.lookup("root")
        if not isinstance(root, str):
            return Unknown()
        reference = RowReference(_join(root, value), argument)
        self.smoke.rows.append((line, reference))
        return reference

    def execute(self, statements: list[list[PowerShellNode]]) -> None:
        """Run statements in order."""
        for statement in statements:
            self.statement(statement)

    def statement(self, nodes: list[PowerShellNode]) -> None:
        """Run one statement the reader understands."""
        first = nodes[0]
        keyword = (
            first.text.lower()
            if isinstance(first, PowerShellToken) and first.kind == "word"
            else ""
        )
        handler = {
            "foreach": self.foreach,
            "if": self.branches,
            "try": self.branches,
            "switch": self.switch,
            "function": self.function,
            "check": self.check,
            "run": self.run,
            "runlinkimage": self.run,
            "inventoryrun": self.run,
            "makefile": self.make_file,
            "makesizedfile": self.make_file,
            "makelinkfile": self.make_file,
            "makelink": self.make_file,
            "new-item": self.new_item,
        }.get(keyword)
        if handler:
            handler(nodes)
            return
        texts = [_word_text(node) for node in nodes]
        equals = next((position for position, word in enumerate(texts) if word in ("=", "+=")), 0)
        targets = [node for node in nodes[:equals] if _word_text(node) != ","]
        variables = [
            node
            for node in targets
            if isinstance(node, PowerShellToken) and node.kind == "variable"
        ]
        if equals and len(variables) == len(targets):
            self.assign(variables, nodes[equals], nodes[equals + 1 :])
            return
        self.io_call(nodes)

    def assign(
        self, targets: list[PowerShellToken], operator: PowerShellNode, nodes: list[PowerShellNode]
    ) -> None:
        """Assign a value to one variable, or the items of a split to several."""
        value = self.evaluate(nodes)
        if isinstance(operator, PowerShellToken) and operator.text == "+=":
            value = self.evaluate_append(targets[0].text, value)
        if len(targets) == 1:
            name = targets[0].text
            if name == "work":
                return
            if isinstance(value, Unknown) and not value.names:
                value = Unknown((name,))
            self.variables[name] = value
            self.smoke.values.setdefault(name, value)
            return
        items = value if isinstance(value, tuple) else ()
        for position, target in enumerate(targets):
            if isinstance(value, Unknown):
                self.variables[target.text] = Unknown((target.text,))
            elif position < len(items):
                rest = (
                    items[position:]
                    if position == len(targets) - 1 and len(items) > len(targets)
                    else None
                )
                self.variables[target.text] = rest if rest else items[position]
            else:
                self.variables[target.text] = ""

    def evaluate_append(self, name: str, value: PowerShellValue) -> PowerShellValue:
        """Return a variable's value with `+=` applied."""
        current = self.lookup(name)
        if isinstance(current, tuple) and isinstance(value, tuple):
            return current + value
        if isinstance(current, str) and isinstance(value, str):
            return current + value
        return Unknown((name,))

    def condition(self, node: PowerShellNode) -> bool | None:
        """Decide `(A -eq B)` or `(A -ne B)` when both sides are known strings."""
        if not isinstance(node, PowerShellGroup):
            return None
        for operator in ("-eq", "-ne"):
            sides = _split_on(node.children, operator)
            if len(sides) == 2:
                left, right = self.evaluate(sides[0]), self.evaluate(sides[1])
                if isinstance(left, str) and isinstance(right, str):
                    return (left.lower() == right.lower()) == (operator == "-eq")
        return None

    def branches(self, nodes: list[PowerShellNode]) -> None:
        """Run an `if` chain's branch when every condition is decided, else every block."""
        blocks: list[tuple[bool | None, PowerShellGroup]] = []
        condition: bool | None = True
        for node in nodes:
            if isinstance(node, PowerShellGroup) and node.opener == "(":
                condition = self.condition(node)
            elif isinstance(node, PowerShellGroup) and node.opener == "{":
                blocks.append((condition, node))
                condition = True
        if all(decided is not None for decided, _ in blocks) and _word_text(nodes[0]) == "if":
            for decided, block in blocks:
                if decided:
                    self.execute(powershell_statements(block.children))
                    return
            return
        for _, block in blocks:
            self.execute(powershell_statements(block.children))

    def switch(self, nodes: list[PowerShellNode]) -> None:
        """Run the `switch` arm whose label equals the subject, or every arm when unknown."""
        groups = [node for node in nodes if isinstance(node, PowerShellGroup)]
        if len(groups) < 2:
            return
        subject = self.evaluate(groups[0].children)
        for arm in powershell_statements(groups[1].children):
            label = self.evaluate(arm[:1])
            body = arm[-1]
            if not isinstance(body, PowerShellGroup):
                continue
            if not isinstance(subject, str) or (
                isinstance(label, str) and label.lower() == subject.lower()
            ):
                self.execute(powershell_statements(body.children))

    def foreach(self, nodes: list[PowerShellNode]) -> None:
        """Run a `foreach ($item in VALUE) { ... }` body once per item."""
        if (
            len(nodes) < 3
            or not isinstance(nodes[1], PowerShellGroup)
            or not isinstance(nodes[2], PowerShellGroup)
        ):
            return
        header = nodes[1].children
        if len(header) < 3 or not isinstance(header[0], PowerShellToken):
            return
        value = self.evaluate(header[2:])
        if isinstance(value, (Unknown, RowReference)):
            self.note(
                value if isinstance(value, Unknown) else Unknown(),
                nodes[0].line,
                self.source(header[2:]),
            )
            return
        for item in value if isinstance(value, tuple) else (value,):
            self.variables[header[0].text] = item
            self.execute(powershell_statements(nodes[2].children))

    def function(self, nodes: list[PowerShellNode]) -> None:
        """Record a function's parameters and body, for the row helpers checks call."""
        if len(nodes) < 3 or not isinstance(nodes[1], PowerShellToken):
            return
        parameters: list[str] = []
        if isinstance(nodes[2], PowerShellGroup) and nodes[2].opener == "(":
            parameters = [
                token.text
                for token in nodes[2].children
                if isinstance(token, PowerShellToken) and token.kind == "variable"
            ]
        body = nodes[-1]
        if isinstance(body, PowerShellGroup) and body.opener == "{":
            self.helpers[nodes[1].text.lower()] = (parameters, self.text[body.start : body.end])

    def run(self, nodes: list[PowerShellNode]) -> None:
        """Start a run block; its flags come from the hashtable after the name."""
        name = self.evaluate(nodes[1:2]) if len(nodes) > 1 else ""
        options = self.source(nodes[2:])
        self.smoke.runs.append(
            Run(
                name if isinstance(name, str) else _word_text(nodes[0]),
                nodes[0].line,
                bool(re.search(r"\bNoSecrets\s*=\s*\$true", options, re.I)),
                bool(re.search(r"\bFull\s*=\s*\$true", options, re.I)),
            )
        )

    def make_file(self, nodes: list[PowerShellNode]) -> None:
        """Record the file `MakeFile`, `MakeSizedFile`, `MakeLinkFile` or `MakeLink` writes."""
        name = _word_text(nodes[0])
        position = 2 if name == "makelink" else 1
        if len(nodes) <= position:
            return
        node = nodes[position]
        value = self.evaluate([node])
        root = self.lookup("root" if name in ("makefile", "makesizedfile") else "linkroot")
        self.fixture(_join_values(root, value), node, True)

    def fixture(self, value: PowerShellValue, node: PowerShellNode, is_file: bool) -> None:
        """Record a written image path, or the variables it could not resolve."""
        if isinstance(value, str):
            self.smoke.add_fixture(value, node.line, is_file)
        else:
            self.note(
                value if isinstance(value, Unknown) else Unknown(), node.line, node_text([node])
            )

    def new_item(self, nodes: list[PowerShellNode]) -> None:
        """Record the directory or link `New-Item -Path PATH` makes."""
        words = [node.text.lower() if isinstance(node, PowerShellToken) else "" for node in nodes]
        if "-path" not in words or words.index("-path") + 1 >= len(nodes):
            return
        node = nodes[words.index("-path") + 1]
        is_link = "symboliclink" in words or "junction" in words
        self.fixture(self.evaluate([node]), node, is_link)

    def io_call(self, nodes: list[PowerShellNode]) -> None:
        """Record the path `[IO.File]::WriteAllText(PATH, ...)` or `CreateDirectory(PATH)` writes."""
        for position, node in enumerate(nodes[:-1]):
            if not isinstance(node, PowerShellToken) or node.text not in (
                "WriteAllText",
                "CreateDirectory",
            ):
                continue
            group = nodes[position + 1]
            if isinstance(group, PowerShellGroup):
                first = _split_on(group.children, ",")[0]
                if first:
                    self.fixture(self.evaluate(first), first[0], node.text == "WriteAllText")

    def check(self, nodes: list[PowerShellNode]) -> None:
        """Record a `Check NAME { BLOCK }` and what its block asserts."""
        name_value = self.evaluate(nodes[1:2])
        name = name_value if isinstance(name_value, str) else node_text(nodes[1:2])
        name = drift_common.collapse(name) or "Check"
        block = nodes[-1]
        if len(nodes) != 3 or not isinstance(block, PowerShellGroup) or block.opener != "{":
            self.smoke.unparsed.append((nodes[0].line, name))
            return
        text = self.text[block.start + 1 : block.end - 1]
        assertions, unparsed = self.block_assertions(text, block.line)
        if unparsed:
            self.smoke.unparsed.append((block.line, name))
        self.smoke.checks.append(CheckLine(name, block.line, assertions))
        for match in PATH_SUFFIX.finditer(text):
            self.smoke.suffix_patterns.append(
                (block.line, match.group("pattern") or match.group("ending"))
            )

    def argument_row(self, argument: str, line: int) -> RowReference | None:
        """Return the row an argument of `Row`, `StatusOf` or a row helper names."""
        tokens = powershell_tokens(argument)
        reference = self.row(powershell_groups(tokens)[0], line) if tokens else Unknown()
        if isinstance(reference, RowReference):
            return reference
        return None

    def block_assertions(self, text: str, line: int) -> tuple[list[Assertion], bool]:
        """Return what a Check block asserts (see `powershell_check_assertions`)."""
        return powershell_check_assertions(text, line, self)


def _join_values(left: PowerShellValue, right: PowerShellValue) -> PowerShellValue:
    """Join two path values, Unknown when either is."""
    if isinstance(left, str) and isinstance(right, str):
        return _join(left, right)
    names = [name for value in (left, right) if isinstance(value, Unknown) for name in value.names]
    return Unknown(tuple(names))


ARGUMENT = r"""(?:'(?:[^']|'')*'|"(?:[^"`]|`.)*"|\$[\w:]+(?:\[\d+\])?)"""
STATUS_OF = re.compile(r"\(StatusOf\s+(%s)\)\s*-(eq|ne)\s*'([a-z_]+)'" % ARGUMENT, re.I)
ROW_PROPERTY = re.compile(
    r"\(Row\s+(%s)\)\.(secret|status)\s*-(eq|ne)\s*(\$true|\$false|'[a-z_]+')" % ARGUMENT, re.I
)
ROW_ASSIGNED = re.compile(r"\$([\w:]+)\s*=\s*Row\s+(%s)" % ARGUMENT, re.I)
VARIABLE_PROPERTY = re.compile(
    r"\$([\w:]+)\.(secret|status)\s*-(eq|ne)\s*(\$true|\$false|'[a-z_]+')", re.I
)
POWERSHELL_ROW_CALL = re.compile(r"(?<![\w$-])(?:Row|StatusOf)\s+(%s)?" % ARGUMENT, re.I)
PATH_SUFFIX = re.compile(
    r"\.path\b[^{}]*?(?:-like\s+(['\"])(?P<pattern>\*[^'\"]*)\1"
    r"|\.EndsWith\(\s*(['\"])(?P<ending>[^'\"]*)\3)",
    re.I,
)


def _facet(row: RowReference, field_name: str, operator: str, value: str) -> Assertion:
    """Return the assertion `row.<field> -eq|-ne <value>` makes."""
    negated = operator.lower() == "ne"
    if field_name.lower() == "secret":
        return Assertion(row, value.lower() == "$true", "", negated)
    return Assertion(row, None, value.strip("'"), negated)


def powershell_check_assertions(
    text: str, line: int, reader: _PowerShellReader
) -> tuple[list[Assertion], bool]:
    """Return what a smoke.ps1 Check block asserts, and whether a row call was unread.

    Accepted forms: `(StatusOf ARG) -eq|-ne 'status'`, `(Row ARG).secret|status -eq VALUE`,
    `$name.secret|status -eq VALUE` on a variable holding `Row ARG` (set in the block or before
    it), and a call of a helper whose body sets `$row = Row $parameter` and compares
    `$row.secret` or `$row.status`. ARG is a quoted string or a variable; a `Row` or `StatusOf`
    call whose argument is anything else makes the check unparsed.
    """
    assertions: list[Assertion] = []
    local: dict[str, RowReference] = {}
    for match in ROW_ASSIGNED.finditer(text):
        reference = reader.argument_row(match.group(2), line)
        if reference:
            local[_variable_name(match.group(1))] = reference
    for match in STATUS_OF.finditer(text):
        reference = reader.argument_row(match.group(1), line)
        if reference:
            assertions.append(_facet(reference, "status", match.group(2), match.group(3)))
    for match in ROW_PROPERTY.finditer(text):
        reference = reader.argument_row(match.group(1), line)
        if reference:
            assertions.append(_facet(reference, *match.group(2, 3, 4)))
    for match in VARIABLE_PROPERTY.finditer(text):
        name = _variable_name(match.group(1))
        value = local.get(name) or reader.variables.get(name)
        if isinstance(value, RowReference):
            assertions.append(_facet(value, *match.group(2, 3, 4)))
    assertions.extend(helper_assertions(text, line, reader))
    unparsed = any(match.group(1) is None for match in POWERSHELL_ROW_CALL.finditer(text))
    return assertions, unparsed


def helper_assertions(text: str, line: int, reader: _PowerShellReader) -> list[Assertion]:
    """Return what the calls of row helper functions in a Check block assert.

    A helper is a function whose body sets `$row = Row $parameter`; it asserts `secret` when
    the body compares `$row.secret` with `$true`, `$false` or a parameter (then the call's
    argument there), and a status when it compares `$row.status` with a literal. A function
    whose body compares a path with `-like "*/$parameter"` names its row by a suffix.
    """
    found: list[Assertion] = []
    for name, (parameters, body) in reader.helpers.items():
        if name in ("row", "statusof"):
            continue
        row_parameter = re.search(r"\$row\s*=\s*Row\s+\$(\w+)", body, re.I)
        suffix_parameter = re.search(r"-like\s+\"\*/?\$(\w+)\"", body, re.I)
        if not row_parameter and not suffix_parameter:
            continue
        call = re.compile(
            r"(?<![\w$-])%s\s+(%s)((?:\s+(?:%s|\$true|\$false))*)"
            % (re.escape(name), ARGUMENT, ARGUMENT),
            re.I,
        )
        for match in call.finditer(text):
            later = re.findall(r"%s|\$true|\$false" % ARGUMENT, match.group(2))
            arguments = [match.group(1), *later]
            if suffix_parameter:
                reader.smoke.suffix_patterns.append((line, match.group(1).strip("'\"")))
                continue
            reference = reader.argument_row(match.group(1), line)
            if reference is None:
                continue
            secret = re.search(r"\$row\.secret\s*-eq\s*(\$\w+)", body, re.I)
            status = re.search(r"\$row\.status\s*-eq\s*'([a-z_]+)'", body, re.I)
            secret_value: bool | None = None
            if secret:
                word = secret.group(1).lower()
                if word in ("$true", "$false"):
                    secret_value = word == "$true"
                elif word[1:] in [parameter.lower() for parameter in parameters]:
                    position = [parameter.lower() for parameter in parameters].index(word[1:])
                    if position < len(arguments) and arguments[position].lower() in (
                        "$true",
                        "$false",
                    ):
                        secret_value = arguments[position].lower() == "$true"
            found.append(Assertion(reference, secret_value, status.group(1) if status else ""))
    return found


def powershell_value(
    text: str, variables: dict[str, PowerShellValue] | None = None
) -> PowerShellValue:
    """Evaluate a PowerShell expression as the smoke.ps1 reader does.

    Known: quoted strings (a double-quoted one with backtick escapes, `$name` and `$(...)`),
    variables (names in lower case), `@(...)` arrays, `-split 'regex'` with an optional limit,
    `+` on strings or arrays, `[N]` indexing, `| Where-Object { $_ -ne '' }`, `Join-Path A B`,
    `ImagePath X` (under `$Root`) and `[System.IO.Path]::Combine(...)`. Anything else gives
    `Unknown`, naming the variables that made it so.
    """
    reader = _PowerShellReader(Smoke(SMOKE_PS1, case_insensitive=True), text)
    reader.variables.update(variables or {})
    return reader.evaluate(powershell_groups(powershell_tokens(text)))


def read_powershell_smoke(text: str, path: str = SMOKE_PS1) -> Smoke:
    """Read a smoke.ps1: its fixtures, runs, checks and the variables they needed."""
    smoke = Smoke(path, case_insensitive=True)
    smoke.banners = [
        number for number, line in enumerate(text.split("\n"), 1) if RUN_BANNER.match(line)
    ]
    reader = _PowerShellReader(smoke, text)
    reader.execute(powershell_statements(powershell_groups(powershell_tokens(text))))
    return smoke


# --- Collection bases and glob matching -----------------------------------------------------


@dataclass(frozen=True)
class Base:
    """A collection base a path lies under: `home`, `project` or `volume`, and the path below it.

    `volume` names the volume for a `volume` base.
    """

    kind: str
    relative: str
    volume: str = ""


def collection_bases(path: str, claims: Callable[[str], str]) -> list[Base]:
    """Return each collection base an image path lies under, with the path relative to it.

    A home is `home/<user>`, `Users/<user>`, `root` or `usr/share/<name>`; a volume is the
    `_data` directory of a `volumes/<name>` directory; a project is `srv/<name>`, any other
    first-level directory but `SYSTEM_DIRECTORIES`, or a directory inside a home that no home
    entry claims (`claims` returns the claiming agent of a home-relative path, or "").
    """
    located = image_path(path)
    if located is None:
        return []
    parts = located[1].split("/")
    bases: list[Base] = []
    if "volumes" in parts:
        position = parts.index("volumes")
        if len(parts) > position + 3 and parts[position + 2] == "_data":
            bases.append(Base("volume", "/".join(parts[position + 3 :]), parts[position + 1]))
    home_depth = {"home": 2, "Users": 2, "root": 1}.get(parts[0], 0)
    if parts[:2] == ["usr", "share"]:
        home_depth = 3
    if home_depth and len(parts) > home_depth:
        home_parts = parts[home_depth:]
        bases.append(Base("home", "/".join(home_parts)))
        for depth in range(1, len(home_parts)):
            if claims("/".join(home_parts[:depth])):
                break
            bases.append(Base("project", "/".join(home_parts[depth:])))
    if parts[0] == "srv" and len(parts) > 2:
        bases.append(Base("project", "/".join(parts[2:])))
    elif parts[0] not in SYSTEM_DIRECTORIES and len(parts) > 1:
        bases.append(Base("project", "/".join(parts[1:])))
    return bases


def subtree_glob_regex(glob: str, case_insensitive: bool) -> re.Pattern[str]:
    """Compile an `EXCLUDES` or `SECRET_GLOBS` glob, whose `*` and `?` also match `/`.

    Both collectors match these globs against the whole path below a collection base (`find
    -path` and a `case` pattern in sh, `-like` in PowerShell, which ignores case); a `[...]`
    class passes through with `!` as negation.
    """
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
    return re.compile("".join(out), re.IGNORECASE if case_insensitive else 0)


def segment_regexes(glob: str, case_insensitive: bool) -> list[re.Pattern[str]]:
    """Compile a `CATALOG` or `PROJECT_CATALOG` glob segment by segment (`*` stops at `/`)."""
    catalog = drift_common.catalog_module()
    flags = re.IGNORECASE if case_insensitive else 0
    return [
        re.compile(catalog.glob_segment_to_regex(segment).pattern, flags)
        for segment in glob.split("/")
        if segment
    ]


def entry_claims(regexes: list[re.Pattern[str]], relative: str) -> bool:
    """Return whether a catalog entry's segments match the first segments of a path."""
    parts = [part for part in relative.split("/") if part]
    if len(regexes) > len(parts):
        return False
    return all(regex.fullmatch(part) for regex, part in zip(regexes, parts, strict=False))


SPAN_PREFIXES = ("~/", "$HOME/", "%APPDATA%/", "%LOCALAPPDATA%/", "./")
PLACEHOLDER = re.compile(r"<[^<>/]*>|\{[^{}/]*\}|(?<![^/])(?:YYYY|MM|DD)(?![^/])")


def span_glob(span: str) -> str:
    """Normalise a code span to a path glob as the tables-against-research rule does.

    Backslashes become slashes; a leading `~/`, `$HOME/`, `%APPDATA%/`, `%LOCALAPPDATA%/` or
    `./` and a trailing `/` are dropped; a placeholder (`<id>`, `{uuid}`, or a whole
    segment `YYYY`, `MM` or `DD`) becomes `*`.
    """
    text = span.strip().replace("\\", "/")
    for prefix in SPAN_PREFIXES:
        if text.startswith(prefix):
            text = text[len(prefix) :]
            break
    return PLACEHOLDER.sub("*", text.rstrip("/"))


@functools.cache
def _span_regexes(span: str, case_insensitive: bool) -> tuple[re.Pattern[str], ...]:
    """Return the segment regexes of a normalised code span; none for wildcards or a bad glob."""
    glob = span_glob(span)
    if not glob.strip("*?/[]"):
        return ()
    # A span such as `[[repositories]]` reads as a nested class, which `re` warns about.
    with warnings.catch_warnings():
        warnings.simplefilter("ignore", FutureWarning)
        try:
            return tuple(segment_regexes(glob, case_insensitive))
        except re.error:
            return ()


def span_names_path(span: str, relative: str, case_insensitive: bool = False) -> bool:
    """Return whether a code span names a relative path by its last segments, glob by glob.

    The span, normalised by `span_glob`, matches when each of its segments, read as a catalog
    glob segment, matches the path's segment at the same place counted from the end: so
    `history.jsonl`, `.claude/history.jsonl` and `<slug>/<id>.jsonl` each name a path ending
    that way. A span of wildcards alone (`*`, `<id>`) names nothing.
    """
    regexes = _span_regexes(span, case_insensitive)
    parts = [part for part in relative.split("/") if part]
    if not regexes or len(regexes) > len(parts):
        return False
    tail = parts[len(parts) - len(regexes) :]
    return all(regex.fullmatch(part) for regex, part in zip(regexes, tail, strict=True))


# --- Rules ----------------------------------------------------------------------------------


@dataclass
class _Context:
    """What every rule of one smoke file needs: the tables, documents and the reader's result."""

    root: pathlib.Path
    smoke: Smoke
    tables: drift_common.Tables
    documents: dict[str, pathlib.Path]
    bases: dict[str, list[Base]] = field(default_factory=dict)
    spans: dict[str, list[str] | None] = field(default_factory=dict)

    def claims(self, relative: str) -> str:
        """Return the agent whose home entry claims a home-relative path, or ""."""
        if self.smoke.case_insensitive:
            windows = relative.replace("/", "\\")
            return self.tables.catalog.claims_home_path(windows, windows=True)
        return self.tables.catalog.claims_home_path(relative)

    def bases_of(self, path: str) -> list[Base]:
        """Return the collection bases of an image path, computed once."""
        if path not in self.bases:
            self.bases[path] = collection_bases(path, self.claims)
        return self.bases[path]

    def finding(
        self, agent: str, kind: str, subject: str, line: int, message: str
    ) -> drift_common.Finding:
        """Return a finding on this smoke file, its detail `<file> <subject>`."""
        detail = "%s %s" % (self.smoke.label, subject)
        return drift_common.Finding(agent, kind, detail, self.smoke.path, line, message)


def _top_directories(smoke: Smoke) -> dict[str, set[str]]:
    """Return the first-level directories of each image, from the paths the test writes."""
    found: dict[str, set[str]] = {}
    for path in smoke.fixtures:
        located = image_path(path)
        if located:
            found.setdefault(located[0], set()).add(located[1].split("/")[0])
    return found


def suffix_rows(smoke: Smoke) -> set[RowReference]:
    """Return the rows a smoke.ps1 names by a suffix: a first segment no image directory has."""
    if not smoke.case_insensitive:
        return set()
    tops = _top_directories(smoke)
    found: set[RowReference] = set()
    for _, row in smoke.rows:
        located = image_path(row.path)
        if located is None or located[1].split("/")[0] not in tops.get(located[0], set()):
            found.add(row)
    return found


def _counted(context: _Context, wanted: Callable[[Assertion, Run], bool]) -> list[list[str]]:
    """Return the base-relative paths of each assertion `wanted` accepts in its run.

    A negated assertion, one outside every run and one on a row named by a suffix never count.
    """
    suffixes = suffix_rows(context.smoke)
    paths: list[list[str]] = []
    for check in context.smoke.checks:
        run = context.smoke.run_at(check.line)
        for assertion in check.assertions:
            if run is None or assertion.negated or assertion.row in suffixes:
                continue
            if wanted(assertion, run):
                paths.append([base.relative for base in context.bases_of(assertion.row.path)])
    return paths


def _covered(regex: re.Pattern[str], paths: list[list[str]]) -> bool:
    """Return whether the regex matches some base-relative path of some assertion."""
    return any(regex.fullmatch(relative) for relatives in paths for relative in relatives)


def _secret_flagged(assertion: Assertion, run: Run) -> bool:
    """Accept `secret: true` in a run with the default secret and exclusion options."""
    return assertion.secret is True and not run.no_secrets and not run.full


def _secret_skipped(assertion: Assertion, run: Run) -> bool:
    """Accept `skipped_secret` in a `--no-secrets` run."""
    return assertion.status == "skipped_secret" and run.no_secrets


def _excluded_status(assertion: Assertion, run: Run) -> bool:
    """Accept `skipped_excluded` in a run without `--full`."""
    return assertion.status == "skipped_excluded" and not run.full


CHECK_RULES = (
    ("secret", "secret-check-missing", _secret_flagged, "no default-run check asserts secret"),
    ("secret", "nosecrets-check-missing", _secret_skipped, "no --no-secrets check asserts it"),
    ("exclude", "exclusion-check-missing", _excluded_status, "no check asserts skipped_excluded"),
)


def check_findings(context: _Context) -> Iterator[drift_common.Finding]:
    """Yield the secret, no-secrets and exclusion findings: one per table line no check covers."""
    insensitive = context.smoke.case_insensitive
    for table, kind, wanted, message in CHECK_RULES:
        paths = _counted(context, wanted)
        for line in context.tables.table(table):
            if not _covered(subtree_glob_regex(line.glob, insensitive), paths):
                catalog = context.tables.catalog
                agent = drift_common.attribute_glob(line.glob, catalog, context.root)
                text = "%s on a path this glob matches (catalog line %d)" % (message, line.line)
                yield context.finding(agent, kind, line.glob, 0, text)


def fixture_findings(context: _Context) -> Iterator[drift_common.Finding]:
    """Yield a `fixture-missing` finding per home or project line no fixture path matches."""
    insensitive = context.smoke.case_insensitive
    relatives: dict[str, set[str]] = {"home": set(), "project": set()}
    for path in context.smoke.fixtures:
        for base in context.bases_of(path):
            if base.kind in relatives:
                relatives[base.kind].add(base.relative)
    for table in ("home", "project"):
        for line in context.tables.table(table):
            regexes = segment_regexes(line.glob, insensitive)
            if not any(entry_claims(regexes, relative) for relative in relatives[table]):
                text = "no fixture path matches this %s line (catalog line %d)" % (table, line.line)
                subject = "%s %s" % (table, line.glob)
                yield context.finding(line.agent, "fixture-missing", subject, 0, text)


def _owner(context: _Context, base: Base) -> str:
    """Return the agent whose table line claims a path under a base, or ""."""
    if base.kind == "home":
        return context.claims(base.relative)
    if base.kind == "project":
        return context.tables.catalog.project_agent_for(base.relative)
    for line in context.tables.docker:
        if fnmatch.fnmatchcase(base.volume, line.glob):
            return line.agent
    return ""


def _document_spans(context: _Context, agent: str) -> list[str] | None:
    """Return the code spans of an agent's research document, or None when it has none.

    The `shared` and `project` lines belong to `research/shared.md`.
    """
    if agent not in context.spans:
        name = "shared" if agent in ("shared", "project") else agent
        path = context.documents.get(name, context.root / "research" / ("%s.md" % name))
        spans = None
        if path.is_file():
            sections = drift_common.research_sections(path).values()
            spans = [span.text for section in sections for span in section.code_spans]
        context.spans[agent] = spans
    return context.spans[agent]


def _excluded(regex: re.Pattern[str] | None, relative: str) -> bool:
    """Return whether an exclusion matches a path or one of its directories."""
    if regex is None:
        return False
    parts = relative.split("/")
    return any(regex.fullmatch("/".join(parts[:depth])) for depth in range(1, len(parts) + 1))


def _exclusion_regex(context: _Context) -> re.Pattern[str] | None:
    """Return one regex matching what any `EXCLUDES` glob matches, or None without any."""
    insensitive = context.smoke.case_insensitive
    patterns = [
        subtree_glob_regex(line.glob, insensitive).pattern for line in context.tables.exclude
    ]
    if not patterns:
        return None
    combined = "|".join("(?:%s)" % pattern for pattern in patterns)
    return re.compile(combined, re.IGNORECASE if insensitive else 0)


def name_findings(context: _Context) -> Iterator[drift_common.Finding]:
    """Yield a `fixture-name-undocumented` finding per fixture file no span of its document names.

    The first base whose table line claims the file decides its agent; a file under an
    exclusion is skipped, since the collector never records its name.
    """
    insensitive = context.smoke.case_insensitive
    exclusion = _exclusion_regex(context)
    for path in sorted(context.smoke.files):
        owned = [(base, _owner(context, base)) for base in context.bases_of(path)]
        owned = [(base, agent) for base, agent in owned if agent]
        if not owned or _excluded(exclusion, owned[0][0].relative):
            continue
        base, agent = owned[0]
        spans = _document_spans(context, agent)
        if spans is None or any(
            span_names_path(span, base.relative, insensitive) for span in spans
        ):
            continue
        text = "no code span of the %s document names this %s fixture" % (agent, base.kind)
        line = context.smoke.fixtures[path]
        yield context.finding(agent, "fixture-name-undocumented", base.relative, line, text)


def suffix_findings(context: _Context) -> Iterator[drift_common.Finding]:
    """Yield a `check-by-suffix` finding per row argument or path pattern, once each."""
    rows = suffix_rows(context.smoke)
    named = [(line, row.argument.strip("'\"")) for line, row in context.smoke.rows if row in rows]
    seen: set[str] = set()
    for line, subject in sorted(named + context.smoke.suffix_patterns):
        if subject not in seen:
            seen.add(subject)
            text = "a check names a row by this suffix, not by its full image path"
            yield context.finding("-", "check-by-suffix", subject, line, text)


def framework_secrets(smoke: Smoke, claims: Callable[[str], str]) -> dict[str, set[str]]:
    """Return the home-relative paths of each agent's `secret` cases in the frameworks list.

    The list is the first value of `CASES_FW` (smoke.sh) or `$CasesFrameworks` (smoke.ps1):
    lines `kind|agent|path` with the path relative to the home the fixture loop writes to.
    """
    value = smoke.values.get(FRAMEWORK_CASES.get(smoke.path, ""), "")
    text = "\n".join(value) if isinstance(value, tuple) else value if isinstance(value, str) else ""
    secrets: dict[str, set[str]] = {}
    for case in text.split("\n"):
        kind, _, rest = case.strip().partition("|")
        relative = rest.partition("|")[2]
        if kind == "secret" and relative:
            agent = claims(relative) or "-"
            secrets.setdefault(agent, set()).add(
                relative.lower() if smoke.case_insensitive else relative
            )
    return secrets


def framework_findings(context: _Context) -> Iterator[drift_common.Finding]:
    """Yield a `framework-nosecrets-missing` finding per framework no skipped_secret check covers."""
    secrets = framework_secrets(context.smoke, context.claims)
    if not secrets:
        return
    skipped = {
        relative for relatives in _counted(context, _secret_skipped) for relative in relatives
    }
    if context.smoke.case_insensitive:
        skipped = {relative.lower() for relative in skipped}
    for agent, relatives in secrets.items():
        if not relatives & skipped:
            text = "no --no-secrets check asserts skipped_secret on its frameworks secret cases"
            yield context.finding(agent, "framework-nosecrets-missing", agent, 0, text)


def reader_findings(context: _Context) -> Iterator[drift_common.Finding]:
    """Yield the `check-path-unresolved` and `check-unparsed` findings, once per subject."""
    for name, line in sorted(context.smoke.unresolved.items(), key=lambda item: item[1]):
        text = "no assignment from literals gives this variable a value here"
        yield context.finding("-", "check-path-unresolved", name, line, text)
    seen: set[str] = set()
    for line, name in context.smoke.unparsed:
        if name not in seen:
            seen.add(name)
            text = "the reader accepts no form of this check's name, expression or row argument"
            yield context.finding("-", "check-unparsed", name, line, text)


READERS: dict[str, Callable[[str, str], Smoke]] = {
    SMOKE_SH: read_shell_smoke,
    SMOKE_PS1: read_powershell_smoke,
}
RULES = (
    check_findings,
    fixture_findings,
    name_findings,
    suffix_findings,
    framework_findings,
    reader_findings,
)


def run(root: pathlib.Path) -> Iterator[drift_common.Finding]:
    """Yield every finding of both smoke tests against the catalog tables under `root`."""
    tables = drift_common.tables(root / drift_common.CATALOG_TEXT)
    documents = drift_common.agent_documents(root)
    for relative, reader in READERS.items():
        path = root / relative
        if not path.is_file():
            continue
        smoke = reader(path.read_text(encoding="utf-8"), relative)
        context = _Context(root, smoke, tables, documents)
        for rule in RULES:
            yield from rule(context)


def main(arguments: list[str] | None = None, root: pathlib.Path | None = None) -> int:
    """Run the check against the baseline and return the exit status."""
    summary = (__doc__ or "").split("\n")[0]
    return drift_common.main_with_baseline(CHECKER, run, arguments, root, summary)


if __name__ == "__main__":
    sys.exit(main())
