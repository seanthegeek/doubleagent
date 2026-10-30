#!/usr/bin/env python3
"""Check the parsers against the Parsing part of their research documents.

Each `parsers.ALL` class is read with `ast` and compared with its own agent's document. Its
literals are its effective set's: its module and each base's in `parsers/`, whole, and what those
import from another parsers module (`from .x import name`, `x.name`, in functions too) with the
top-level names that uses; never `base.py`. "Documented" means in the code spans and fenced lines
of the part named, in the parser's own document or, for a module of another parser's effective
set, in that parser's agent document (rules 1 to 4).

- `row-text-undocumented`: a row text's constant prefix is in no Parsing part span or fenced line.
- `parser-message-undocumented`: a `parser: ` literal's constant pieces, in order, are in no
  Parsing part span or fenced line.
- `pattern-undocumented`: a file pattern of the parser equals no Parsing part span or fenced line,
  both unescaped, and no path or glob span outside Fixtures has its count of segments, each
  matched by the pattern's glob form there (`regex_components`, `*` any text): a segment with a
  quantifier, class, group or `.*` only by one with `*`, `?` or `<...>`, a fixed one by its
  literal; a top-level alternation by a branch; a pattern that does not compile by nothing.
- `key-undocumented`: a key the parser reads, or a `select` column or table, is no Parsing word;
  a named group `(?P<name>...)` of its literals read from a match's groups is no key read.
- `flag-undocumented`: `records_sync`, `records_relay` or `records_origin` is True while Sync,
  relay and origin rows says "The agent records none." or "Not stated.", or `has_secondary` is True
  while Past-use rows and secondary sources says so.
- `flag-unset`: that subsection names a `session synced:`, `session relayed:` or `session origin:`
  row or a secondary source in a sentence neither denying nor only supposing it, its flag False.
- `not-read-contradicted`: a name Parsing says is not read is a key read or a file patterns match.
- `docstring-fact-undocumented`: a module docstring span is a row text or key in no span or fenced
  line of the document, a pattern the whole document does not document by rule 3's test, or a
  `key_value` with a key or value Parsing lacks.

Usage: tools/check_parser_research.py [--update-baseline]
"""

from __future__ import annotations

import ast
import itertools
import pathlib
import re
import sys
from collections.abc import Iterable, Iterator
from dataclasses import dataclass, field
from typing import TypeGuard

import drift_common

CHECKER = "check_parser_research"
PARSING = ("Parsing",)
PARSER_PLAN = ("Parsing", "Parser plan")
FIXTURES = (*PARSER_PLAN, "Fixtures")
SYNC_SECTION = "Sync, relay and origin rows"
PAST_USE_SECTION = "Past-use rows and secondary sources"
PARSER_PREFIX = "parser: "
SELECTOR_METHODS = ("wants", "secondary")
REGEX_FUNCTIONS = frozenset({"compile", "match", "search", "fullmatch"})
KEY_READ_METHODS = frozenset({"get", "pop"})
# The methods of a regex match whose results hold its groups, not record keys.
MATCH_GROUP_METHODS = frozenset({"groupdict", "group"})
NAMED_GROUP = re.compile(r"\(\?P<([A-Za-z_][A-Za-z0-9_]*)>")
# The flag a `session <label>:` row of the Sync, relay and origin rows subsection calls for.
SYNC_ROW_FLAGS = {
    "session synced:": "records_sync",
    "session relayed:": "records_relay",
    "session origin:": "records_origin",
}
STATES_NONE = ("The agent records none.", "Not stated.")

ROW_TEXT = re.compile(r"^[a-z][a-z0-9 _/-]*:( |$)")
ROW_TEXT_SETTING = re.compile(r"^[A-Za-z][^:\n]{0,60}: .*?(?<![\w.])[A-Za-z_][\w.]*=")
PLACEHOLDER = re.compile(r"\{[^{}]*\}|%(?:\([^)]*\))?[-#0 +]*\d*(?:\.\d+)?[sdrifxXeEgGc]| <")
KEY_FORM = re.compile(r"^[A-Za-z_$@][A-Za-z0-9_$@.-]*$")
KEY_VALUE = re.compile(r"""^([A-Za-z_$@][A-Za-z0-9_$@.-]*): (?:"([^"]+)"|'([^']+)')$""")
WORD = re.compile(r"[A-Za-z0-9_$@-]+")
SQL_SELECT = re.compile(r"\bselect\s+(.*?)\s+from\s", re.IGNORECASE | re.DOTALL)
SQL_TABLE = re.compile(r"\b(?:from|join)\s+[\"'`\[]?([A-Za-z_][A-Za-z0-9_]*)", re.IGNORECASE)
SQL_IDENTIFIER = re.compile(r"[\"'`\[]?([A-Za-z_][A-Za-z0-9_]*)[\"'`\]]?(\s*\()?")
SQL_ALIAS = re.compile(r"\s+as\s+\S+\s*$", re.IGNORECASE)
# A table name or alias qualifying a column (`d.` of `d.created_at`, `"d".` of `"d".*`).
SQL_QUALIFIER = re.compile(r"[\"'`\[]?[A-Za-z_][A-Za-z0-9_]*[\"'`\]]?\s*\.\s*")
SQL_WORDS = frozenset(
    {"as", "distinct", "all", "case", "when", "then", "else", "end", "null", "and", "or", "not"}
    | {"rowid", "_rowid", "oid", "count", "length", "max", "min", "sum", "coalesce", "instr"}
)
SQLITE_ROW_IDS = frozenset({"rowid", "_rowid", "oid"})
PATTERN_SIGNS = re.compile(r"\\|^\^|\$$|\(\?|\[\^")
NOT_READ = re.compile(r"(?<!was )(?<!were )\b(?:does )?not read\b", re.IGNORECASE)
LIST_ITEM = re.compile(r"^\s*(?:[-*]|\d+\.)\s")
CLAUSE_END = re.compile(r"[;:]|\.(?:\s|$)")
READ_EXCEPTION = re.compile(r"\((?=[^()]*\bread\b)(?![^()]*\bnot read\b)[^()]*\)")
SENTENCE_END = re.compile(r"(?<=\.)\s+(?=[A-Z`(\[])")
CODE_SPAN = re.compile(r"(`+)(.+?)\1", re.DOTALL)
# A regex quantifier, lazy or not, that makes the piece before it variable.
QUANTIFIER = re.compile(r"(?:[*+?]|\{\d*(?:,\d*)?\})\??")
# What a path or glob span segment holds in place of a variable piece.
SPAN_WILDCARD = re.compile(r"<[^>]*>|[*?]")
# Words that make a sentence deny or only suppose a row.
NEGATION = re.compile(r"\b(?:no|not|never|none|neither|nor|without|would|could)\b", re.IGNORECASE)
SECONDARY_NAMED = re.compile(r"\b(?:is|are|as)\s+(?:a\s+|the\s+)?secondary\b", re.IGNORECASE)
# Last dotted segments that make a code span a file name rather than a key.
FILE_EXTENSIONS = frozenset(
    {"json", "jsonl", "md", "db", "sqlite", "sqlite3", "txt", "toml", "yaml", "yml", "log"}
    | {"vscdb", "pb", "key", "py", "ts", "js", "lock", "xml", "env", "html", "csv", "zst"}
)


FUNCTIONS = (ast.FunctionDef, ast.AsyncFunctionDef, ast.Lambda)
COMPREHENSIONS = (ast.ListComp, ast.SetComp, ast.GeneratorExp, ast.DictComp)


# --- Literals ---------------------------------------------------------------------------------


@dataclass(frozen=True)
class Literal:
    """A string of a parser module: its text, an f-string's values as `{}`, and where it is."""

    text: str
    path: pathlib.Path
    line: int


@dataclass(frozen=True)
class Selector:
    """A file pattern a parser selects files with: a regex, or a path compared to `rel`."""

    literal: Literal
    is_regex: bool


class ModuleIndex:
    """The modules of the analyzer package, each parsed with `ast` once."""

    def __init__(self, package: pathlib.Path) -> None:
        """Index the package at `package` (`analyzer/doubleagent`)."""
        self.package = package
        self.trees: dict[pathlib.Path, ast.Module] = {}
        self.bindings_cache: dict[pathlib.Path, dict[str, tuple[pathlib.Path, ast.AST]]] = {}

    def tree(self, path: pathlib.Path) -> ast.Module:
        """Return the parsed module at `path`."""
        if path not in self.trees:
            self.trees[path] = ast.parse(path.read_text(encoding="utf-8"), str(path))
        return self.trees[path]

    def imported(self, path: pathlib.Path, node: ast.ImportFrom) -> pathlib.Path | None:
        """Return the module file a relative import in the module at `path` names, or None."""
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

    def bindings(self, path: pathlib.Path) -> dict[str, tuple[pathlib.Path, ast.AST]]:
        """Return each top-level name of a module and where its value or definition is.

        An assignment maps to its value, a function or class to its definition, and a relative
        `from ... import` to the binding in the imported module; the last binding wins.
        """
        if path in self.bindings_cache:
            return self.bindings_cache[path]
        found: dict[str, tuple[pathlib.Path, ast.AST]] = {}
        self.bindings_cache[path] = found
        for statement in self.tree(path).body:
            if isinstance(statement, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
                found[statement.name] = (path, statement)
            elif isinstance(statement, ast.Assign):
                for target in statement.targets:
                    if isinstance(target, ast.Name):
                        found[target.id] = (path, statement.value)
            elif isinstance(statement, ast.AnnAssign) and statement.value is not None:
                if isinstance(statement.target, ast.Name):
                    found[statement.target.id] = (path, statement.value)
            elif isinstance(statement, ast.ImportFrom) and statement.module:
                module = self.imported(path, statement)
                for alias in statement.names if module else ():
                    bound = self.bindings(module).get(alias.name) if module else None
                    if bound is not None:
                        found[alias.asname or alias.name] = bound
        return found

    def fold(self, path: pathlib.Path, node: ast.AST | None, depth: int = 0) -> str | None:
        """Return the string an expression always has, or None.

        It folds a string constant, `+` of foldable strings, an f-string without values and a
        name bound at module level (or imported) to a foldable string.
        """
        if node is None or depth > 20:
            return None
        if isinstance(node, ast.Constant):
            return node.value if isinstance(node.value, str) else None
        if isinstance(node, ast.BinOp) and isinstance(node.op, ast.Add):
            left = self.fold(path, node.left, depth + 1)
            right = self.fold(path, node.right, depth + 1)
            return None if left is None or right is None else left + right
        if isinstance(node, ast.JoinedStr):
            parts = [self.fold(path, value, depth + 1) for value in node.values]
            return None if None in parts else "".join(part or "" for part in parts)
        if isinstance(node, ast.Name):
            bound = self.bindings(path).get(node.id)
            return self.fold(bound[0], bound[1], depth + 1) if bound else None
        return None


def docstring_nodes(tree: ast.AST) -> set[int]:
    """Return the ids of the docstring constants of a module and its classes and functions."""
    found: set[int] = set()
    for node in ast.walk(tree):
        owners = (ast.Module, ast.ClassDef, ast.FunctionDef, ast.AsyncFunctionDef)
        if isinstance(node, owners) and node.body:
            first = node.body[0]
            value = first.value if isinstance(first, ast.Expr) else None
            if isinstance(value, ast.Constant) and isinstance(value.value, str):
                found.add(id(value))
    return found


def string_literals(path: pathlib.Path, nodes: Iterable[ast.AST], skip: set[int]) -> list[Literal]:
    """Return the strings under `nodes`, docstrings and an f-string's parts left out.

    An f-string is one literal whose values read `{}`; the constant parts inside it are not
    literals of their own.
    """
    found: list[Literal] = []
    inside: set[int] = set()
    for root in nodes:
        for node in ast.walk(root):
            if isinstance(node, ast.JoinedStr):
                pieces = []
                for value in node.values:
                    inside.add(id(value))
                    text = value.value if isinstance(value, ast.Constant) else None
                    pieces.append(text if isinstance(text, str) else "{}")
                found.append(Literal("".join(pieces), path, node.lineno))
            elif isinstance(node, ast.Constant) and isinstance(node.value, str):
                if id(node) not in skip and id(node) not in inside:
                    found.append(Literal(node.value, path, node.lineno))
    return found


def _used_names(node: ast.AST) -> set[str]:
    """Return the plain names an AST node loads or calls."""
    return {item.id for item in ast.walk(node) if isinstance(item, ast.Name)}


def closure(index: ModuleIndex, path: pathlib.Path, names: Iterable[str]) -> list[ast.AST]:
    """Return the top-level definitions of `names` in a module and every top-level name they use."""
    bindings = index.bindings(path)
    wanted = [name for name in names if name in bindings]
    seen: set[str] = set()
    nodes: list[ast.AST] = []
    while wanted:
        name = wanted.pop()
        if name in seen:
            continue
        seen.add(name)
        where, node = bindings[name]
        if where != path:
            continue
        nodes.append(node)
        wanted.extend(sorted(_used_names(node) & set(bindings) - seen))
    return nodes


@dataclass
class EffectiveSet:
    """The AST nodes whose literals count for a parser, by module, and the literals themselves."""

    nodes: dict[pathlib.Path, list[ast.AST]] = field(default_factory=dict)

    def add(self, path: pathlib.Path, nodes: Iterable[ast.AST]) -> None:
        """Add nodes of the module at `path`, each once."""
        known = {id(node) for node in self.nodes.get(path, [])}
        self.nodes.setdefault(path, []).extend(node for node in nodes if id(node) not in known)

    def literals(self, index: ModuleIndex) -> list[Literal]:
        """Return the string literals of every node, module by module."""
        found: list[Literal] = []
        for path, nodes in self.nodes.items():
            found.extend(string_literals(path, nodes, docstring_nodes(index.tree(path))))
        return found


def effective_set(
    index: ModuleIndex, parser: drift_common.ParserClass, parsers: pathlib.Path
) -> EffectiveSet:
    """Return the effective set of a parser class, `base.py` excluded.

    Its own module and each base's module count whole; each other parsers-package module that
    one of them imports from, at any depth, counts with the imported definitions and the
    top-level names they use (`from .x import name`, or `x.name` after `from . import x`).
    """
    base = parsers / "base.py"
    found = EffectiveSet()
    modules = [module for module in parser.modules if module != base]
    for module in modules:
        found.add(module, index.tree(module).body)
    for module in modules:
        for imported, names in imported_names(index, module, parsers).items():
            if imported not in modules and imported != base:
                found.add(imported, closure(index, imported, names))
    return found


def imported_names(
    index: ModuleIndex, path: pathlib.Path, parsers: pathlib.Path
) -> dict[pathlib.Path, list[str]]:
    """Return each parsers-package module the module at `path` imports, with the names used."""
    tree = index.tree(path)
    found: dict[pathlib.Path, list[str]] = {}
    for node in ast.walk(tree):
        if not isinstance(node, ast.ImportFrom) or node.level != 1:
            continue
        if node.module:
            module = index.imported(path, node)
            if module is not None and module.parent == parsers:
                found.setdefault(module, []).extend(alias.name for alias in node.names)
            continue
        for alias in node.names:
            module = parsers / ("%s.py" % alias.name)
            local = alias.asname or alias.name
            used = [
                item.attr
                for item in ast.walk(tree)
                if isinstance(item, ast.Attribute)
                and isinstance(item.value, ast.Name)
                and item.value.id == local
            ]
            if module.is_file():
                found.setdefault(module, []).extend(used)
    return found


# --- The heuristics ---------------------------------------------------------------------------


def is_row_text(text: str) -> bool:
    """Return whether a literal is a row text.

    A row text is a lowercase label, a colon and a space or the end
    (`^[a-z][a-z0-9 _/-]*:( |$)`), or any label of up to 60 characters, a colon, a space and,
    later, a `<word>=` setting; a `parser: ` message is rule 2's, not a row text.
    """
    if text.startswith(PARSER_PREFIX):
        return False
    return bool(ROW_TEXT.match(text) or ROW_TEXT_SETTING.match(text))


def key_value(text: str) -> tuple[str, str] | None:
    """Return (key, value) of a code span of the form `<key>: "<value>"` or `<key>: '<value>'`.

    `type: "session"` gives (`type`, `session`); `type: session`, `model: a -> b` and an empty
    quoted value give None. Such a span is a fact about a record, never a row text.
    """
    match = KEY_VALUE.match(text)
    return (match.group(1), match.group(2) or match.group(3)) if match else None


def constant_prefix(text: str) -> str:
    """Return a literal up to its first placeholder (`{...}`, `%s` and kin, or ` <`), collapsed."""
    match = PLACEHOLDER.search(text)
    return drift_common.collapse(text[: match.start()] if match else text)


def constant_pieces(text: str) -> list[str]:
    """Return the non-blank constant pieces of a literal between its placeholders, collapsed."""
    pieces = (drift_common.collapse(piece) for piece in PLACEHOLDER.split(text))
    return [piece for piece in pieces if piece]


def holds_pieces(text: str, pieces: list[str]) -> bool:
    """Return whether `text` holds each of `pieces`, in order and not overlapping."""
    start = 0
    for piece in pieces:
        found = text.find(piece, start)
        if found < 0:
            return False
        start = found + len(piece)
    return True


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


def sql_names(text: str) -> list[str]:
    """Return the columns of each `select` list and the tables after `from` and `join` in `text`.

    A column is an identifier of a list item less its `as` alias, its table qualifier and any
    function name: `c.channel` gives `channel` and `d.*` nothing, so a table alias defined after
    a table name (`from x d`, `join x as d`) is never a name. `*`, `rowid`, keywords and `%s`
    placeholders are not names either. Matching ignores case.
    """
    selects = list(SQL_SELECT.finditer(text))
    names: list[str] = []
    for match in selects:
        for item in match.group(1).split(","):
            item = SQL_ALIAS.sub("", PLACEHOLDER.sub(" ", item))
            item = SQL_QUALIFIER.sub("", item)
            for identifier in SQL_IDENTIFIER.finditer(item):
                if not identifier.group(2):
                    names.append(identifier.group(1))
    if selects:
        names.extend(match.group(1) for match in SQL_TABLE.finditer(text))
    return [name for name in dict.fromkeys(names) if name.lower() not in SQL_WORDS]


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


def selector_matches_name(selector: Selector, name: str) -> bool:
    r"""Return whether a file pattern matches a file or directory a document names.

    In the name a `<...>` placeholder and a `*` read `x` and a leading `~/` is dropped; a name
    with neither a `/` nor a dot is no file. A directory (ending in `/`) matches a pattern
    holding its last segment as a component. A file name of k segments matches a regex whose
    last k `regex_components` match it whole, a bare name only when the last component holds it
    unescaped (`[^/]+\.json` names no file), and a path that ends in it.
    """
    name = re.sub(r"<[^>]*>|\*+", "x", name.removeprefix("~/"))
    segments = [segment for segment in name.split("/") if segment]
    text = selector.literal.text
    if not segments or ("/" not in name and "." not in name):
        return False
    if name.endswith("/"):
        source = unescape_pattern(text) if selector.is_regex else text
        components = re.split(r"[/^]", source)
        return segments[-1] in (components if selector.is_regex else components[:-1])
    if not selector.is_regex:
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


def mask_code_spans(text: str) -> str:
    """Return `text` with each code span's inside made `x`, its backticks kept, offsets unchanged."""
    return CODE_SPAN.sub(
        lambda match: (
            match.group(1) + "x" * (len(match.group(0)) - 2 * len(match.group(1))) + match.group(1)
        ),
        text,
    )


def text_units(text: str) -> list[tuple[int, int]]:
    """Return the (start, end) offsets of the sentences, list items and table cells of a text.

    Code spans are masked first, so their punctuation splits nothing. A table line splits into
    its cells; a blank line or a line starting a list item (`- `, `* `, `1. `) starts a
    paragraph; a paragraph splits after a `.` followed by a space and a capital, backtick or
    bracket.
    """
    masked = mask_code_spans(text)
    units: list[tuple[int, int]] = []
    paragraph_start = -1
    offset = 0
    lines = masked.split("\n")
    for line in [*lines, ""]:
        line_end = offset + len(line)
        stripped = line.strip()
        starts_item = bool(LIST_ITEM.match(line))
        if paragraph_start >= 0 and (not stripped or stripped.startswith("|") or starts_item):
            units.extend(_sentences(masked, paragraph_start, offset - 1))
            paragraph_start = -1
        if stripped.startswith("|"):
            cell_start = offset
            for match in re.finditer(r"\|", line):
                units.append((cell_start, offset + match.start()))
                cell_start = offset + match.end()
            units.append((cell_start, line_end))
        elif stripped and paragraph_start < 0:
            paragraph_start = offset
        offset = line_end + 1
    return [(start, end) for start, end in units if masked[start:end].strip()]


def _sentences(masked: str, start: int, end: int) -> list[tuple[int, int]]:
    """Return the sentence offsets of the paragraph `masked[start:end]`."""
    found: list[tuple[int, int]] = []
    cursor = start
    for match in SENTENCE_END.finditer(masked, start, end):
        found.append((cursor, match.start()))
        cursor = match.end()
    found.append((cursor, end))
    return found


def not_read_spans(section: drift_common.Section) -> list[drift_common.CodeSpan]:
    """Return the code spans a section says are not read.

    In each unit of `text_units` holding "not read" or "does not read": a unit starting "Not
    read" gives every span after the phrase, less a parenthesis saying something is read; a
    phrase inside a parenthesis gives the span just before it; "does not read" gives the spans
    after it to the clause's end (`;`, `:` or `.`); any other gives the spans of its clause
    (from `;`, `:`, `.` or `,`) before it ("`x` and `y` are not read"), or only the last for
    "is not read" ("`config.json` `key` is not read" names the key). "was not read" and
    "were not read" report what a researcher did not read, so they count for nothing.
    """
    text = section.text
    masked = mask_code_spans(text)
    spans = [(match.start(), match.end(), match.group(2)) for match in CODE_SPAN.finditer(text)]
    found: list[drift_common.CodeSpan] = []
    for start, end in text_units(text):
        for phrase in NOT_READ.finditer(masked, start, end):
            chosen = _chosen_spans(masked, spans, start, end, phrase)
            found.extend(
                drift_common.CodeSpan(drift_common.collapse(body), section.line_of(where))
                for where, _, body in chosen
            )
    return found


def _chosen_spans(
    masked: str,
    spans: list[tuple[int, int, str]],
    start: int,
    end: int,
    phrase: re.Match,
) -> list[tuple[int, int, str]]:
    """Return the spans of the unit `masked[start:end]` that one "not read" phrase is about."""
    inside = [item for item in spans if start <= item[0] and item[1] <= end]
    unit = masked[start:end]
    opening = masked.rfind("(", start, phrase.start())
    if unit.lstrip().startswith("Not read"):
        exceptions = [
            (start + match.start(), start + match.end()) for match in READ_EXCEPTION.finditer(unit)
        ]
        return [
            item
            for item in inside
            if item[0] >= phrase.end()
            and not any(low <= item[0] < high for low, high in exceptions)
        ]
    if opening >= 0 and masked.find(")", opening, phrase.start()) < 0:
        before = [item for item in inside if item[1] <= opening]
        return before[-1:]
    if phrase.group(0).lower().startswith("does"):
        clause = CLAUSE_END.search(masked, phrase.end(), end)
        stop = clause.start() if clause else end
        return [item for item in inside if phrase.end() <= item[0] and item[1] <= stop]
    marks = (";", ":", ". ", ", ")
    clause_start = max(masked.rfind(mark, start, phrase.start()) for mark in marks)
    clause_start = max(clause_start, start)
    before = [item for item in inside if clause_start <= item[0] and item[1] <= phrase.start()]
    singular = masked[clause_start : phrase.start()].endswith(" is ")
    return before[-1:] if singular else before


def states_none(text: str) -> bool:
    """Return whether a fixed subsection says "The agent records none." or "Not stated."."""
    collapsed = drift_common.collapse(text)
    return any(phrase in collapsed for phrase in STATES_NONE)


# --- The document -----------------------------------------------------------------------------


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

    def documents_pattern(self, pattern: str) -> bool:
        r"""Return whether the corpus documents a file pattern, as `pattern-undocumented` says.

        `^\.qwen/trustedFolders\.json$` is documented by `.qwen/trustedFolders.json` and
        `^\.pi/.*$` by `.pi/*`, never by `.pi/probe/x.jsonl`; a pattern that does not compile
        is documented by nothing.
        """
        if unescape_pattern(pattern) in self.patterns:
            return True
        try:
            re.compile(pattern)
        except re.error:
            return False
        branches = top_level_branches(pattern)
        if len(branches) > 1:
            return any(self.documents_pattern(branch) for branch in branches)
        forms = [segment_form(segment) for segment in regex_components(pattern)]
        for path in self.paths:
            segments = path.split("/")
            if len(segments) == len(forms) and all(
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


@dataclass
class Document:
    """A research document read for this check: its sections and two corpora."""

    path: pathlib.Path
    sections: dict[str, drift_common.Section]
    parsing: Corpus
    whole: Corpus

    def subsection(self, title: str) -> drift_common.Section | None:
        """Return the Parser plan subsection with this title, or None."""
        found = drift_common.within(self.sections, *PARSER_PLAN, title)
        return found[0] if found else None


def read_document(path: pathlib.Path) -> Document:
    """Return the document at `path` with its Parsing and whole corpora."""
    sections = drift_common.research_sections(path)
    return Document(
        path,
        sections,
        corpus(drift_common.within(sections, *PARSING)),
        corpus(list(sections.values())),
    )


# --- What a parser reads ----------------------------------------------------------------------


def class_definitions(
    index: ModuleIndex, parser: drift_common.ParserClass
) -> list[tuple[pathlib.Path, ast.ClassDef]]:
    """Return the class then its bases, nearest first, each with its module."""
    found: list[tuple[pathlib.Path, ast.ClassDef]] = []
    for name in (parser.class_name, *parser.bases):
        for module in parser.modules:
            node = next(
                (
                    item
                    for item in index.tree(module).body
                    if isinstance(item, ast.ClassDef) and item.name == name
                ),
                None,
            )
            if node is not None:
                found.append((module, node))
                break
    return found


def find_method(
    classes: list[tuple[pathlib.Path, ast.ClassDef]], name: str
) -> tuple[pathlib.Path, ast.FunctionDef] | None:
    """Return the nearest definition of a method in the class and its bases, or None."""
    for module, node in classes:
        for item in node.body:
            if isinstance(item, ast.FunctionDef) and item.name == name:
                return module, item
    return None


def selector_functions(
    classes: list[tuple[pathlib.Path, ast.ClassDef]], parsers: pathlib.Path
) -> list[tuple[pathlib.Path, ast.FunctionDef]]:
    """Return the nearest `wants` and `secondary` outside `base.py`, and the methods they call."""
    found: list[tuple[pathlib.Path, ast.FunctionDef]] = []
    for name in SELECTOR_METHODS:
        method = find_method(classes, name)
        if method is None or method[0] == parsers / "base.py":
            continue
        found.append(method)
        for node in ast.walk(method[1]):
            function = node.func if isinstance(node, ast.Call) else None
            if not isinstance(function, ast.Attribute):
                continue
            owner = function.value
            helper = find_method(classes, function.attr)
            is_self = isinstance(owner, ast.Name) and owner.id == "self"
            if is_self and helper is not None and helper not in found:
                found.append(helper)
    return found


def _selectors_of_value(
    index: ModuleIndex, path: pathlib.Path, value: ast.AST, line: int
) -> list[Selector]:
    """Return the selectors a bound value is: a compiled regex, a path, or a tuple of paths."""
    if isinstance(value, ast.Call) and _is_regex_call(value):
        text = index.fold(path, value.args[0]) if value.args else None
        return [Selector(Literal(text, path, line), True)] if text is not None else []
    if isinstance(value, (ast.Tuple, ast.List, ast.Set)):
        items = [index.fold(path, item) for item in value.elts]
        return [Selector(Literal(item, path, line), False) for item in items if item]
    text = index.fold(path, value)
    return [Selector(Literal(text, path, line), False)] if text else []


def _is_regex_call(node: ast.AST) -> bool:
    """Return whether a node calls `re.compile`, `re.match`, `re.search` or `re.fullmatch`."""
    if not isinstance(node, ast.Call) or not isinstance(node.func, ast.Attribute):
        return False
    owner = node.func.value
    return isinstance(owner, ast.Name) and owner.id == "re" and node.func.attr in REGEX_FUNCTIONS


def selectors(
    index: ModuleIndex, parser: drift_common.ParserClass, parsers: pathlib.Path
) -> list[Selector]:
    """Return the file patterns of a parser.

    They are the regexes passed to `re.compile`, `re.match`, `re.search` or `re.fullmatch` in
    `wants` or `secondary` or in a method they call on `self`, or bound at module or class level
    to a name those methods use; and the strings those methods compare (`==`, `in`,
    `startswith`, `endswith`), directly or through such a name. A pattern built at run time
    (`TEMPLATE % prefix`) is not folded and not checked.
    """
    classes = class_definitions(index, parser)
    attributes: dict[str, tuple[pathlib.Path, ast.AST, int]] = {}
    for module, node in reversed(classes):
        for item in node.body:
            if isinstance(item, ast.Assign):
                for target in item.targets:
                    if isinstance(target, ast.Name):
                        attributes[target.id] = (module, item.value, item.lineno)
    found: list[Selector] = []
    for module, function in selector_functions(classes, parsers):
        bindings = index.bindings(module)
        for node in ast.walk(function):
            if isinstance(node, ast.Call) and _is_regex_call(node):
                found.extend(_selectors_of_value(index, module, node, node.lineno))
            elif isinstance(node, ast.Name) and node.id in bindings:
                where, value = bindings[node.id]
                line = getattr(value, "lineno", node.lineno)
                if not isinstance(value, (ast.FunctionDef, ast.ClassDef)):
                    found.extend(_selectors_of_value(index, where, value, line))
            elif isinstance(node, ast.Attribute) and node.attr in attributes:
                is_self = isinstance(node.value, ast.Name) and node.value.id == "self"
                if is_self:
                    where, value, line = attributes[node.attr]
                    found.extend(_selectors_of_value(index, where, value, line))
            elif isinstance(node, (ast.Compare, ast.Call)):
                found.extend(_compared_paths(module, node))
    unique: dict[tuple[str, bool], Selector] = {}
    for selector in found:
        unique.setdefault((selector.literal.text, selector.is_regex), selector)
    return [selector for selector in unique.values() if _is_path_like(selector)]


def _compared_paths(path: pathlib.Path, node: ast.AST) -> list[Selector]:
    """Return the string constants a comparison or a `startswith`/`endswith` call names."""
    if isinstance(node, ast.Call):
        is_affix = isinstance(node.func, ast.Attribute) and node.func.attr in (
            "startswith",
            "endswith",
        )
        candidates = node.args if is_affix else []
    elif isinstance(node, ast.Compare):
        candidates = [node.left, *node.comparators]
    else:
        candidates = []
    found: list[Selector] = []
    for candidate in candidates:
        items = candidate.elts if isinstance(candidate, (ast.Tuple, ast.List, ast.Set)) else []
        for item in items or [candidate]:
            if isinstance(item, ast.Constant) and isinstance(item.value, str) and item.value:
                found.append(Selector(Literal(item.value, path, item.lineno), False))
    return found


def _is_path_like(selector: Selector) -> bool:
    """Return whether a selector can name a file: a regex, or a string holding `/` or a dot."""
    text = selector.literal.text
    return selector.is_regex or "/" in text or "." in text


def key_reads(index: ModuleIndex, effective: EffectiveSet) -> list[Literal]:
    """Return the keys the effective set reads, and the columns and tables its `select`s name.

    A key read is the first argument of `.get` or `.pop`, a subscript, or the left side of `in`
    or `not in` against anything but a literal string or collection, when it folds to a string
    of the key form (`is_key`); SQLite's own `rowid` and its `_rowid` alias are not keys, nor is
    a named group of a literal read on a match's groups (`match_group_reads`).
    """
    found: list[Literal] = []
    literals = effective.literals(index)
    groups = {name for literal in literals for name in NAMED_GROUP.findall(literal.text)}
    for path, nodes in effective.nodes.items():
        for root in nodes:
            on_groups = match_group_reads(root)
            for node in ast.walk(root):
                candidate = _read_key_node(node)
                text = index.fold(path, candidate) if candidate is not None else None
                if text in groups and id(node) in on_groups:
                    continue
                if text is not None and is_key(text) and text.lower() not in SQLITE_ROW_IDS:
                    found.append(Literal(text, path, getattr(candidate, "lineno", 0)))
    for literal in literals:
        if "select" in literal.text.lower():
            found.extend(
                Literal(name, literal.path, literal.line) for name in sql_names(literal.text)
            )
    return found


def match_group_reads(root: ast.AST) -> set[int]:
    """Return the ids of the key reads under `root` whose object holds a regex match's groups.

    The object of a subscript, a method call or a membership test is a `.groupdict()` or
    `.group(...)` call, or a name whose every binding in the same function, or outside any
    function, is one: `fields.groupdict()["sid"]`, `groups.get("sid")`, `"sid" in groups`; a
    name later rebound to a record, by unpacking, a `with` target or an `async for` too, is
    no match's groups; storing into one of its items does not rebind it. A comprehension is
    its own scope: it sees the names around it less those its own targets bind, while a
    walrus inside it binds in the function around it.
    """
    found: set[int] = set()
    scopes = dict.fromkeys(
        [root, *(node for node in ast.walk(root) if isinstance(node, FUNCTIONS))]
    )
    for scope in scopes:
        nodes = list(_scope_nodes(scope))
        bindings: dict[str, list[bool]] = {}
        for node in nodes:
            for name, from_match in _bindings(node):
                bindings.setdefault(name, []).append(from_match)
            if isinstance(node, COMPREHENSIONS):
                for name, from_match in _walrus_bindings(node):
                    bindings.setdefault(name, []).append(from_match)
        bound = {name for name, forms in bindings.items() if all(forms)}
        _classify_reads(nodes, bound, found)
    return found


def _walrus_bindings(comprehension: ast.AST) -> list[tuple[str, bool]]:
    """Return the bindings of the walrus expressions inside a comprehension, nested ones too.

    A `:=` inside a comprehension binds its name in the containing function, not in the
    comprehension, so these bindings belong to the enclosing scope.
    """
    found: list[tuple[str, bool]] = []
    pending = list(ast.iter_child_nodes(comprehension))
    while pending:
        node = pending.pop()
        if isinstance(node, FUNCTIONS):
            continue
        if isinstance(node, ast.NamedExpr):
            found.extend(_bindings(node))
        pending.extend(ast.iter_child_nodes(node))
    return found


def _classify_reads(nodes: list[ast.AST], bound: set[str], found: set[int]) -> None:
    """Add to `found` the reads among `nodes` whose object is a match's groups or a `bound` name.

    A comprehension among the nodes is classified on its own, with the names its generators
    bind removed from `bound`.
    """
    for node in nodes:
        if isinstance(node, COMPREHENSIONS):
            own = {
                name for generator in node.generators for name in _stored_names(generator.target)
            }
            inner = [child for child in _scope_nodes(node) if child is not node]
            _classify_reads(inner, bound - own, found)
            continue
        if isinstance(node, ast.Subscript):
            holder = node.value
        elif isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute):
            holder = node.func.value
        elif _is_membership(node):
            holder = node.comparators[0]
        else:
            continue
        if _is_match_groups(holder) or (isinstance(holder, ast.Name) and holder.id in bound):
            found.add(id(node))


def _is_membership(node: ast.AST) -> TypeGuard[ast.Compare]:
    """Return whether `node` is a one-operator `in` or `not in` comparison."""
    return (
        isinstance(node, ast.Compare)
        and len(node.ops) == 1
        and isinstance(node.ops[0], (ast.In, ast.NotIn))
    )


def _bindings(node: ast.AST) -> list[tuple[str, bool]]:
    """Return the names a statement binds, each with whether its value is a match's groups.

    An assignment, annotated assignment or walrus binds a plain-name target to its value; a
    name inside an unpacking target (`groups, other = ...`), a `for` or `async for` target, a
    `with ... as` name and a function parameter bind to something else.
    """
    if isinstance(node, (ast.Assign, ast.AnnAssign, ast.NamedExpr)):
        if node.value is None:
            return []
        targets = node.targets if isinstance(node, ast.Assign) else [node.target]
        from_match = _is_match_groups(node.value)
        found: list[tuple[str, bool]] = []
        for target in targets:
            if isinstance(target, ast.Name):
                found.append((target.id, from_match))
            else:
                found.extend((name, False) for name in _stored_names(target))
        return found
    if isinstance(node, (ast.For, ast.AsyncFor)):
        return [(name, False) for name in _stored_names(node.target)]
    if isinstance(node, ast.withitem) and node.optional_vars is not None:
        return [(name, False) for name in _stored_names(node.optional_vars)]
    if isinstance(node, ast.arg):
        return [(node.arg, False)]
    return []


def _stored_names(target: ast.AST) -> list[str]:
    """Return the names a target binds: its `Name` nodes in store context, not the containers.

    `groups["sid"] = x` stores into `groups` without rebinding it, so `groups` is not returned;
    `(groups, other)` returns both names.
    """
    return [
        name.id
        for name in ast.walk(target)
        if isinstance(name, ast.Name) and isinstance(name.ctx, ast.Store)
    ]


def _scope_nodes(scope: ast.AST) -> Iterator[ast.AST]:
    """Yield the nodes below `scope`, the scope itself included, never inside a nested scope.

    A nested function is left out; a comprehension is yielded but not entered, since it is a
    scope of its own (`_classify_reads`).
    """
    pending = [scope]
    while pending:
        node = pending.pop()
        yield node
        for child in ast.iter_child_nodes(node):
            if isinstance(child, COMPREHENSIONS):
                yield child
            elif not isinstance(child, FUNCTIONS):
                pending.append(child)


def _is_match_groups(node: ast.AST) -> bool:
    """Return whether a node is a call of a `MATCH_GROUP_METHODS` method."""
    return (
        isinstance(node, ast.Call)
        and isinstance(node.func, ast.Attribute)
        and node.func.attr in MATCH_GROUP_METHODS
    )


def _read_key_node(node: ast.AST) -> ast.AST | None:
    """Return the expression a node reads a key with, or None when it reads none."""
    if isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute) and node.args:
        return node.args[0] if node.func.attr in KEY_READ_METHODS else None
    if isinstance(node, ast.Subscript) and isinstance(node.ctx, ast.Load):
        return node.slice
    if isinstance(node, ast.Compare) and len(node.ops) == 1:
        literal_side = (ast.Constant, ast.Tuple, ast.List, ast.Set, ast.Dict)
        is_membership = isinstance(node.ops[0], (ast.In, ast.NotIn))
        if is_membership and not isinstance(node.comparators[0], literal_side):
            return node.left
    return None


# --- The rules --------------------------------------------------------------------------------


@dataclass
class Context:
    """What every rule of one parser reads."""

    root: pathlib.Path
    parsers: pathlib.Path
    index: ModuleIndex
    parser: drift_common.ParserClass
    document: Document
    literals: list[Literal]
    keys: list[Literal]
    selectors: list[Selector]
    code_names: frozenset[str]
    # The agent documents of the other parsers whose effective set holds each module.
    owners: dict[pathlib.Path, list[Document]]

    def documents_for(self, literal: Literal) -> list[Document]:
        """Return the documents that may document a literal: the parser's own, then others.

        A literal of the parser's own module has its own document only; one of a base or reader
        module also has the documents of the other parsers whose effective set holds that module.
        """
        if literal.path == self.parser.module:
            return [self.document]
        return [self.document, *self.owners.get(literal.path, [])]

    def relative(self, path: pathlib.Path) -> str:
        """Return a path relative to the repository root, in POSIX form."""
        return path.relative_to(self.root).as_posix()

    def finding(
        self, kind: str, detail: str, literal: Literal, message: str
    ) -> drift_common.Finding:
        """Return a finding of this parser at a literal's module and line."""
        return drift_common.Finding(
            self.parser.agent, kind, detail, self.relative(literal.path), literal.line, message
        )


def row_text_findings(context: Context) -> Iterator[drift_common.Finding]:
    """Yield `row-text-undocumented` per row text whose prefix the Parsing part lacks."""
    document = context.relative(context.document.path)
    for literal in context.literals:
        if not is_row_text(literal.text):
            continue
        prefix = constant_prefix(literal.text)
        documents = context.documents_for(literal)
        if prefix and not any(item.parsing.documents_prefix(prefix) for item in documents):
            message = "no code span or fenced line of the Parsing part of %s holds it" % document
            yield context.finding("row-text-undocumented", prefix, literal, message)


def parser_message_findings(context: Context) -> Iterator[drift_common.Finding]:
    """Yield `parser-message-undocumented` per `parser: ` literal the Parsing part lacks."""
    document = context.relative(context.document.path)
    for literal in context.literals:
        if not literal.text.startswith(PARSER_PREFIX):
            continue
        pieces = constant_pieces(literal.text)
        documents = context.documents_for(literal)
        if not any(item.parsing.documents_pieces(pieces) for item in documents):
            message = "its constant pieces are in no code span of the Parsing part of %s" % document
            detail = drift_common.collapse(literal.text)
            yield context.finding("parser-message-undocumented", detail, literal, message)


def pattern_findings(context: Context) -> Iterator[drift_common.Finding]:
    """Yield `pattern-undocumented` per file pattern the Parsing part does not document."""
    document = context.relative(context.document.path)
    for selector in context.selectors:
        documents = context.documents_for(selector.literal)
        if not any(item.parsing.documents_pattern(selector.literal.text) for item in documents):
            message = "no code span or fenced line of the Parsing part of %s names it" % document
            yield context.finding(
                "pattern-undocumented", selector.literal.text, selector.literal, message
            )


def key_findings(context: Context) -> Iterator[drift_common.Finding]:
    """Yield `key-undocumented` per key, column or table the Parsing part does not name."""
    document = context.relative(context.document.path)
    for literal in context.keys:
        documents = context.documents_for(literal)
        if not any(item.parsing.documents_key(literal.text) for item in documents):
            message = "no code span or fenced line of the Parsing part of %s names it" % document
            yield context.finding("key-undocumented", literal.text, literal, message)


def flag_findings(context: Context) -> Iterator[drift_common.Finding]:
    """Yield `flag-undocumented` and `flag-unset` from the two fixed Parser plan subsections."""
    parser = context.parser
    document = context.relative(context.document.path)
    class_literal = Literal(parser.class_name, parser.module, parser.line)
    owners = (
        (SYNC_SECTION, ("records_sync", "records_relay", "records_origin")),
        (PAST_USE_SECTION, ("has_secondary",)),
    )
    for title, flags in owners:
        section = context.document.subsection(title)
        if section is None:
            continue
        body = "\n".join(text for line, text in section.prose if line != section.start)
        named = named_flags(section)
        for flag in flags:
            value = getattr(parser, flag)
            if value and states_none(body):
                message = "%s says the agent records none in %s" % (title, document)
                yield context.finding("flag-undocumented", flag, class_literal, message)
            elif not value and flag in named:
                message = "%s of %s names its row, the flag is False" % (title, document)
                yield drift_common.Finding(
                    parser.agent, "flag-unset", flag, document, named[flag], message
                )


def named_flags(section: drift_common.Section) -> dict[str, int]:
    """Return each flag whose row a section names affirmatively, with the line of the first.

    A code span starting `session synced:`, `session relayed:` or `session origin:` names the
    row of `records_sync`, `records_relay` or `records_origin`, and "is a secondary", "are
    secondary" or "as a secondary" that of `has_secondary`; a sentence of `text_units`
    holding a negation ("no", "not", "never", "none", "neither", "nor", "without") or a
    supposition ("would", "could") names nothing ("yields no `session synced:` row").
    """
    text = section.text
    masked = mask_code_spans(text)
    units = text_units(text)
    places = [
        (flag, match.start())
        for match in CODE_SPAN.finditer(text)
        for label, flag in SYNC_ROW_FLAGS.items()
        if drift_common.collapse(match.group(2)).startswith(label)
    ]
    places.extend(("has_secondary", match.start()) for match in SECONDARY_NAMED.finditer(masked))
    found: dict[str, int] = {}
    for flag, offset in sorted(places, key=lambda place: place[1]):
        unit = next((masked[low:high] for low, high in units if low <= offset < high), "")
        if flag not in found and not NEGATION.search(unit):
            found[flag] = section.line_of(offset)
    return found


def not_read_findings(context: Context) -> Iterator[drift_common.Finding]:
    """Yield `not-read-contradicted` per name said not to be read that the parser reads."""
    document = context.relative(context.document.path)
    keys = {literal.text: literal for literal in context.keys}
    for section in drift_common.within(context.document.sections, *PARSING):
        for span in not_read_spans(section):
            literal = keys.get(span.text)
            if literal is not None:
                where = "%s:%d" % (context.relative(literal.path), literal.line)
                message = "the parser reads this key at %s" % where
            else:
                selector = next(
                    (item for item in context.selectors if selector_matches_name(item, span.text)),
                    None,
                )
                if selector is None:
                    continue
                where = "%s:%d" % (context.relative(selector.literal.path), selector.literal.line)
                message = "the file pattern %r at %s matches it" % (selector.literal.text, where)
            yield drift_common.Finding(
                context.parser.agent,
                "not-read-contradicted",
                span.text,
                document,
                span.line,
                message,
            )


def docstring_facts(text: str, code_names: frozenset[str]) -> list[tuple[str, int]]:
    """Return each docstring code span that is a key-value, row text, pattern or key, with offset.

    A key span is not a file name (its last dotted segment a file extension such as `jsonl`)
    and its first segment names no function, class, constant or parameter of the analyzer.
    """
    found: list[tuple[str, int]] = []
    for match in CODE_SPAN.finditer(text):
        span = drift_common.collapse(match.group(2))
        fact = key_value(span) or is_row_text(span) or span.startswith(PARSER_PREFIX)
        if fact or looks_like_pattern(span):
            found.append((span, match.start()))
        elif is_key(span):
            segments = span.split(".")
            is_file = len(segments) > 1 and segments[-1].lower() in FILE_EXTENSIONS
            if not is_file and segments[0] not in code_names and not span.isupper():
                found.append((span, match.start()))
    return found


def docstring_findings(context: Context) -> Iterator[drift_common.Finding]:
    """Yield `docstring-fact-undocumented` per docstring fact the document does not hold."""
    tree = context.index.tree(context.parser.module)
    docstring = ast.get_docstring(tree, clean=False)
    if not docstring or not tree.body:
        return
    start = tree.body[0].lineno
    whole = context.document.whole
    document = context.relative(context.document.path)
    for span, offset in docstring_facts(docstring, context.code_names):
        pair = key_value(span)
        if pair:
            parsing = context.document.parsing
            documented = all(parsing.documents_key(word) for word in pair)
        elif span.startswith(PARSER_PREFIX):
            documented = whole.documents_pieces(constant_pieces(span))
        elif is_row_text(span):
            documented = whole.documents_prefix(constant_prefix(span))
        elif looks_like_pattern(span):
            documented = whole.documents_pattern(span)
        else:
            documented = whole.documents_key(span)
        if not documented:
            literal = Literal(span, context.parser.module, start + docstring.count("\n", 0, offset))
            message = "%s does not document it" % document
            yield context.finding("docstring-fact-undocumented", span, literal, message)


RULES = (
    row_text_findings,
    parser_message_findings,
    pattern_findings,
    key_findings,
    flag_findings,
    not_read_findings,
    docstring_findings,
)


def code_names(index: ModuleIndex, package: pathlib.Path) -> frozenset[str]:
    """Return every module, function, class, parameter and assigned name of the analyzer package.

    An assigned name is a variable or an attribute stored anywhere (`self.ledger_name = ...`, a
    class attribute), so a docstring naming code (`records_sync`, `vscode_state.EDITOR_AGENTS`)
    is not taken for a record key.
    """
    names: set[str] = set()
    for path in sorted(package.rglob("*.py")):
        names.add(path.stem)
        for node in ast.walk(index.tree(path)):
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
                names.add(node.name)
            elif isinstance(node, ast.arg):
                names.add(node.arg)
            elif isinstance(node, ast.Name) and isinstance(node.ctx, ast.Store):
                names.add(node.id)
            elif isinstance(node, ast.Attribute) and isinstance(node.ctx, ast.Store):
                names.add(node.attr)
        for name in index.bindings(path):
            names.add(name)
    return frozenset(names)


def run(root: pathlib.Path) -> Iterator[drift_common.Finding]:
    """Yield the findings of every parser class against its agent's research document."""
    parsers = root / drift_common.PARSERS
    package = parsers.parent
    index = ModuleIndex(package)
    names = code_names(index, package)
    paths = drift_common.agent_documents(root)
    classes = [item for item in drift_common.parser_classes(parsers) if item.agent in paths]
    read = {item.agent: read_document(paths[item.agent]) for item in classes}
    effective = {id(item): effective_set(index, item, parsers) for item in classes}
    holders: dict[pathlib.Path, list[str]] = {}
    for item in classes:
        for module in effective[id(item)].nodes:
            holders.setdefault(module, []).append(item.agent)
    seen: set[tuple[str, str, str]] = set()
    for parser in classes:
        owners = {
            module: [read[agent] for agent in dict.fromkeys(agents) if agent != parser.agent]
            for module, agents in holders.items()
        }
        context = Context(
            root,
            parsers,
            index,
            parser,
            read[parser.agent],
            effective[id(parser)].literals(index),
            key_reads(index, effective[id(parser)]),
            selectors(index, parser, parsers),
            names,
            owners,
        )
        for rule in RULES:
            for finding in rule(context):
                identity = (finding.agent, finding.kind, drift_common.collapse(finding.detail))
                if identity not in seen:
                    seen.add(identity)
                    yield finding


def main(arguments: list[str] | None = None, root: pathlib.Path | None = None) -> int:
    """Run the check against the baseline and return the exit status."""
    summary = (__doc__ or "").split("\n")[0]
    return drift_common.main_with_baseline(CHECKER, run, arguments, root, summary)


if __name__ == "__main__":
    sys.exit(main())
