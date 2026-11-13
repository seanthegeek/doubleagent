#!/usr/bin/env python3
"""Check the naming and comment rules of the Python, sh, PowerShell and Go files.

The rules are the "Code style" section of AGENTS.md. In every tracked file and every untracked
one git does not ignore that is Python (`.py`), PowerShell (`.ps1`), Go (`.go`) or sh (`.sh`,
or a `#!` line naming a shell), or in the FILE arguments, it checks that:

- a bound name is at least three characters long and spells its words out: a name, or one
  underscore-separated segment of it, in `ABBREVIATIONS` fails. In Python a bound name is an
  assignment, loop, comprehension, `with`, `except` or walrus target, a parameter, a function or
  a class; a class-body field declaration (`at: str`) is exempt, since it mirrors an interface
  key. In sh it is an assigned, `for` or `read` variable, after its temporaries prefix
  (`_cp_` in `_cp_user`) or `_ret_` is removed; in PowerShell a variable after `$` or
  `$script:`, automatic variables excepted; in Go a declared variable or constant, a `:=`
  target, a parameter, a named result, a receiver, a function, a method, a type, a struct field
  or an interface method, split into words at its capitals, with the exemptions "Go standards"
  lists (`GO_ALLOWED_NAMES`). `ALLOWED_SHORT_NAMES` and `INTERFACE_NAMES` list the exceptions;
- a comment or docstring names no issue or pull request number and no date; a copyright line
  and the smoke tests' dated fixture-block labels (`# YYYY-MM-DD <topic> (#n)`) are exempt;
- a run of whole-line comments is at most ten lines long, the file header and a block holding
  the collectors' reader's guide excepted; a docstring is at most ten lines, a module's thirty;
  a Go doc comment (the `//` run directly above a declaration) is at most ten lines.

Quoted text on an sh, PowerShell or Go line is neither a comment nor scanned for names, and a
comment line inside an sh here-document is skipped. ruff's pydocstyle rules check the form of
the Python docstrings; this check does not.

Usage: tools/check_code_style.py [FILE...]

Every failure is printed as `path:line: message`, then the count on stderr; the exit status is 1
when there is any failure, else 0.
"""

from __future__ import annotations

import argparse
import ast
import pathlib
import re
import subprocess
import sys
import tokenize
from collections.abc import Callable, Iterator
from typing import NamedTuple

ROOT = pathlib.Path(__file__).resolve().parent.parent

# `TZ` is the environment variable's own name.
ALLOWED_SHORT_NAMES = {"_", "i", "j", "k", "TZ"}
# Names an interface fixes: pytest passes a fixture only to a parameter spelled as its name.
INTERFACE_NAMES = {"tmp_path"}
# Abbreviations of a word the name should spell out, matched as the whole name or as one
# underscore-separated segment of it (`msg_id`); a trailing underscore marks a builtin's
# name with the underscore added to dodge shadowing (`dict_`).
ABBREVIATIONS = {
    "arr",
    "attr",
    "buf",
    "cfg",
    "cmd",
    "cnt",
    "con",
    "ctx",
    "cur",
    "db",
    "desc",
    "dict_",
    "dir_",
    "elem",
    "err",
    "evt",
    "fh",
    "fn",
    "hist",
    "idx",
    "len_",
    "msg",
    "num",
    "obj",
    "obs",
    "pat",
    "prev",
    "proj",
    "rc",
    "rec",
    "req",
    "res",
    "ret",
    "rx",
    "sess",
    "sid",
    "src",
    "str_",
    "tid",
    "tmp",
    "ts",
    "val",
    "ws",
}
MAX_COMMENT_BLOCK = 10
MAX_DOCSTRING_LINES = 10
MAX_MODULE_DOCSTRING_LINES = 30
MIN_NAME_LENGTH = 3

ISSUE_REFERENCE = re.compile(
    r"(?:(?<=[\s(])#\d{1,5}\b|\b(?:issue|issues|PR|pull request|discussion)s? \d{1,5}\b)"
)
# An ISO date, a month and year, or a bare year; `U+2028` is a code point, not a year.
DATE = re.compile(
    r"\b\d{4}-\d{2}-\d{2}\b"
    r"|\b(?:January|February|March|April|May|June|July|August|September|October|November"
    r"|December) 20\d\d\b"
    r"|(?<![+\w.-])20[2-9]\d\b"
)
COPYRIGHT_LINE = re.compile(r"Copyright \d{4}")
FIXTURE_LABEL = re.compile(r"^\s*#\s*(?:---\s*)?\d{4}-\d{2}-\d{2}\b")
READERS_GUIDE = "Reader's guide"

SH_TEMPORARY_PREFIX = re.compile(r"^_(?:[a-z]{1,4}|ret)_")
SH_ASSIGNMENT = re.compile(r"(?:^|[;{]|&&|\|\|)\s*([A-Za-z_][A-Za-z0-9_]*)=")
SH_FOR = re.compile(r"\bfor\s+([A-Za-z_][A-Za-z0-9_]*)\b")
SH_READ = re.compile(r"\bread\s+(?:-r\s+)?([^|;&<>()]*)$")
SH_HEREDOC = re.compile(r"<<-?\s*'?([A-Za-z_][A-Za-z0-9_]*)'?")
SH_IDENTIFIER = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")
PS_VARIABLE = re.compile(r"\$(?:script:|global:|local:)?([A-Za-z_][A-Za-z0-9_]*)")
GO_TOKEN = re.compile(
    r"""(?P<newline>\n)
    |(?P<space>[ \t\r\f]+)
    |(?P<linecomment>//[^\n]*)
    |(?P<blockcomment>/\*.*?\*/)
    |(?P<raw>`[^`]*`)
    |(?P<string>"(?:\\.|[^"\\\n])*")
    |(?P<char>'(?:\\.|[^'\\\n])+')
    |(?P<number>\.?\d[\w.]*)
    |(?P<ident>[^\W\d]\w*)
    |(?P<punct>:=|\.\.\.|<-|&&|\|\||.)""",
    re.DOTALL | re.VERBOSE,
)
GO_WORD = re.compile(r"[A-Z]+(?![a-z])|[A-Z]?[a-z]+|\d+")
GO_KEYWORDS = {
    "break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough",
    "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range",
    "return", "select", "struct", "switch", "type", "var",
}  # fmt: skip
# Names Go allows short: a throwaway, the error and the context, a comma-ok flag, and an index
# or count of a few lines.
GO_ALLOWED_NAMES = {"_", "ok", "err", "ctx", "i", "j", "k", "n"}
GO_TEST_NAMES = {"t", "b"}
GO_IO_PARAMETERS = {"w": "io.Writer", "r": "io.Reader"}
GO_MAX_RECEIVER_LENGTH = 2
# A token that ends a line without ending the declaration that started on it.
GO_CONTINUATION = {",", "+", "-", "*", "/", "%", "&", "|", "^", "=", ".", "&&", "||", "<", ">"}

PS_AUTOMATIC = {
    "_",
    "args",
    "input",
    "error",
    "host",
    "home",
    "pid",
    "profile",
    "matches",
    "psitem",
}

Finding = tuple[int, str]


def code_files() -> list[pathlib.Path]:
    """Return the tracked and unignored files of the repository this check reads, sorted."""
    listing = subprocess.run(
        ["git", "-C", str(ROOT), "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout
    # The vendored Go modules are third-party source, audited as a dependency,
    # never held to this project's style.
    files = {ROOT / path for path in listing.split("\0") if path and not path.startswith("vendor/")}
    return sorted(path for path in files if path.is_file() and language_of(path))


def language_of(path: pathlib.Path) -> str | None:
    """Return "python", "sh", "powershell" or "go" for a file this check reads, else None."""
    suffixes = {".py": "python", ".ps1": "powershell", ".sh": "sh", ".go": "go"}
    if path.suffix in suffixes:
        return suffixes[path.suffix]
    try:
        with path.open("rb") as handle:
            first_line = handle.readline()
    except OSError:
        return None
    words = first_line.split()
    if first_line.startswith(b"#!") and words and b"sh" in words[0]:
        return "sh"
    return None


def name_message(name: str) -> str | None:
    """Return the failure message for a bound name that is too short or abbreviated, else None."""
    if name in ALLOWED_SHORT_NAMES or name in INTERFACE_NAMES:
        return None
    if len(name) < MIN_NAME_LENGTH:
        return "name %r is shorter than %d characters" % (name, MIN_NAME_LENGTH)
    for segment in name.lower().strip("_").split("_"):
        if segment in ABBREVIATIONS:
            return "name %r abbreviates a word (%r)" % (name, segment)
    return None


def target_names(node: ast.AST) -> Iterator[str]:
    """Yield the names an assignment target binds, unpacking tuples, lists and starred names."""
    if isinstance(node, ast.Name):
        yield node.id
    elif isinstance(node, (ast.Tuple, ast.List)):
        for element in node.elts:
            yield from target_names(element)
    elif isinstance(node, ast.Starred):
        yield from target_names(node.value)


def field_declarations(tree: ast.AST) -> set[int]:
    """Return the ids of the class-body annotations without a value, which the name rule exempts."""
    return {
        id(statement)
        for node in ast.walk(tree)
        if isinstance(node, ast.ClassDef)
        for statement in node.body
        if isinstance(statement, ast.AnnAssign) and statement.value is None
    }


def names_bound_by(node: ast.AST) -> list[str]:
    """Return the names one Python syntax node binds."""
    if isinstance(node, ast.Assign):
        return [name for target in node.targets for name in target_names(target)]
    targeted = (ast.AnnAssign, ast.AugAssign, ast.For, ast.AsyncFor, ast.comprehension)
    if isinstance(node, (*targeted, ast.NamedExpr)):
        return list(target_names(node.target))
    if isinstance(node, ast.withitem) and node.optional_vars is not None:
        return list(target_names(node.optional_vars))
    if isinstance(node, ast.ExceptHandler) and node.name:
        return [node.name]
    if isinstance(node, ast.arg):
        return [node.arg]
    if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
        return [node.name]
    return []


def line_of(node: ast.AST) -> int:
    """Return the line a syntax node starts on, from its first child for nodes without one."""
    if isinstance(node, ast.withitem):
        return node.context_expr.lineno
    if isinstance(node, ast.comprehension):
        return node.target.lineno
    return getattr(node, "lineno", 0)


def python_binding_names(tree: ast.AST) -> Iterator[Finding]:
    """Yield (line, name) for every name a Python module binds, field declarations excepted."""
    exempt = field_declarations(tree)
    for node in ast.walk(tree):
        if id(node) in exempt:
            continue
        for name in names_bound_by(node):
            yield line_of(node), name


def comment_text_findings(line: int, text: str, is_fixture_label: bool) -> Iterator[Finding]:
    """Yield the issue-reference and date findings of one comment or docstring line."""
    if COPYRIGHT_LINE.search(text) or is_fixture_label:
        return
    for match in ISSUE_REFERENCE.finditer(text):
        yield line, "comment names an issue or pull request: %r" % match.group(0)
    for match in DATE.finditer(text):
        yield line, "comment holds a date: %r" % match.group(0)


def comment_block_findings(lines: list[str]) -> Iterator[Finding]:
    """Yield a finding for every run of whole-line `#` comments over the cap.

    The run that opens the file (its header) and a run holding the reader's guide are exempt.
    """
    start = None
    for index, text in enumerate([*lines, ""], start=1):
        is_comment = text.lstrip().startswith("#")
        if is_comment and start is None:
            start = index
        elif not is_comment and start is not None:
            length = index - start
            block = "\n".join(lines[start - 1 : index - 1])
            if length > MAX_COMMENT_BLOCK and start > 2 and READERS_GUIDE not in block:
                message = "comment block of %d lines is longer than %d"
                yield start, message % (length, MAX_COMMENT_BLOCK)
            start = None


def docstring_findings(tree: ast.AST) -> Iterator[Finding]:
    """Yield the text and length findings of every docstring in a Python module."""
    documented = (ast.Module, ast.ClassDef, ast.FunctionDef, ast.AsyncFunctionDef)
    for node in ast.walk(tree):
        if not isinstance(node, documented):
            continue
        docstring = ast.get_docstring(node, clean=False)
        if not docstring or not node.body:
            continue
        first_line = node.body[0].lineno
        docstring_lines = docstring.splitlines()
        for offset, text in enumerate(docstring_lines):
            yield from comment_text_findings(first_line + offset, text, False)
        is_module = isinstance(node, ast.Module)
        cap = MAX_MODULE_DOCSTRING_LINES if is_module else MAX_DOCSTRING_LINES
        if len(docstring_lines) > cap:
            message = "docstring of %d lines is longer than %d"
            yield first_line, message % (len(docstring_lines), cap)


def check_python(path: pathlib.Path, source: str) -> Iterator[Finding]:
    """Yield the findings of one Python file."""
    tree = ast.parse(source, filename=str(path))
    for line, name in python_binding_names(tree):
        message = name_message(name)
        if message:
            yield line, message
    yield from docstring_findings(tree)
    tokens = tokenize.generate_tokens(iter(source.splitlines(keepends=True)).__next__)
    try:
        for token in tokens:
            if token.type == tokenize.COMMENT:
                yield from comment_text_findings(token.start[0], token.string, False)
    except tokenize.TokenError:
        pass
    yield from comment_block_findings(source.splitlines())


def split_comment(text: str) -> tuple[str, str]:
    """Split one sh or PowerShell line into its code and its trailing comment.

    A `#` inside single or double quotes is text, not a comment, and the quoted text is blanked
    in the code part so the name scan never reads it. A string spanning lines is not tracked, so
    its later lines are read as code.
    """
    quote = None
    code: list[str] = []
    for index, character in enumerate(text):
        if quote:
            if character == quote:
                quote = None
                code.append(character)
            continue
        if character in "'\"":
            quote = character
        elif character == "#":
            return "".join(code), text[index:]
        code.append(character)
    return "".join(code), ""


def sh_line_names(code: str) -> Iterator[str]:
    """Yield the variables one line of sh code assigns, loops over or reads into."""
    for match in SH_ASSIGNMENT.finditer(code):
        yield match.group(1)
    for match in SH_FOR.finditer(code):
        yield match.group(1)
    read = SH_READ.search(code.strip())
    if read:
        yield from (name for name in read.group(1).split() if SH_IDENTIFIER.match(name))


def sh_binding_names(lines: list[str]) -> Iterator[Finding]:
    """Yield (line, name) for every variable an sh file binds, here-documents included.

    A comment line inside a here-document is skipped, since the program the here-document feeds
    reads it as a comment too.
    """
    heredoc_end: str | None = None
    for index, text in enumerate(lines, start=1):
        is_comment = text.lstrip().startswith("#")
        if heredoc_end is not None and text.strip() == heredoc_end:
            heredoc_end = None
            continue
        code = "" if is_comment else split_comment(text.strip())[0]
        if heredoc_end is None:
            opened = SH_HEREDOC.search(code)
            heredoc_end = opened.group(1) if opened else None
        for name in sh_line_names(code):
            yield index, name


def check_sh(path: pathlib.Path, source: str) -> Iterator[Finding]:
    """Yield the findings of one sh file."""
    lines = source.splitlines()
    for line, name in sh_binding_names(lines):
        message = name_message(SH_TEMPORARY_PREFIX.sub("", name))
        if message:
            yield line, message
    is_smoke = path.name.startswith("smoke.")
    for index, text in enumerate(lines, start=1):
        comment = split_comment(text)[1]
        if not comment or text.lstrip().startswith("#!"):
            continue
        is_label = is_smoke and bool(FIXTURE_LABEL.match(text))
        yield from comment_text_findings(index, comment, is_label)
    yield from comment_block_findings(lines)


def check_powershell(path: pathlib.Path, source: str) -> Iterator[Finding]:
    """Yield the findings of one PowerShell file."""
    lines = source.splitlines()
    is_smoke = path.name.startswith("smoke.")
    for index, text in enumerate(lines, start=1):
        code, comment = split_comment(text)
        for match in PS_VARIABLE.finditer(code):
            name = match.group(1)
            message = None if name.lower() in PS_AUTOMATIC else name_message(name)
            if message:
                yield index, message
        if comment:
            is_label = is_smoke and bool(FIXTURE_LABEL.match(text))
            yield from comment_text_findings(index, comment, is_label)
    yield from comment_block_findings(lines)


class GoToken(NamedTuple):
    """One Go token: its kind, text, first and last line, and whether it starts its line."""

    kind: str
    text: str
    line: int
    end_line: int
    first: bool


class GoBinding(NamedTuple):
    """A name a Go file binds: where, which, what sort of name, and its type as written."""

    line: int
    name: str
    role: str
    type_text: str


def go_tokens(source: str) -> list[GoToken]:
    """Split Go source into tokens, comments included and white space dropped."""
    tokens: list[GoToken] = []
    line = 1
    last_token_line = 0
    for match in GO_TOKEN.finditer(source):
        kind = match.lastgroup or "punct"
        text = match.group()
        end_line = line + text.count("\n")
        if kind == "newline":
            line += 1
        elif kind != "space":
            tokens.append(GoToken(kind, text, line, end_line, line != last_token_line))
            last_token_line = end_line
            line = end_line
    return tokens


def go_is_name(token: GoToken) -> bool:
    """Return True for an identifier that is not a keyword."""
    return token.kind == "ident" and token.text not in GO_KEYWORDS


def go_close(tokens: list[GoToken], start: int) -> int:
    """Return the index of the bracket that closes the one at `start`, or the last token."""
    depth = 0
    for index in range(start, len(tokens)):
        if tokens[index].kind != "punct":
            continue
        if tokens[index].text in "([{":
            depth += 1
        elif tokens[index].text in ")]}":
            depth -= 1
            if depth == 0:
                return index
    return len(tokens) - 1


def go_split(tokens: list[GoToken], start: int, end: int) -> list[list[GoToken]]:
    """Split the tokens between `start` and `end` (exclusive) at their top-level commas."""
    groups: list[list[GoToken]] = [[]]
    depth = 0
    for token in tokens[start:end]:
        if token.kind == "punct" and token.text in "([{":
            depth += 1
        elif token.kind == "punct" and token.text in ")]}":
            depth -= 1
        if token.kind == "punct" and token.text == "," and depth == 0:
            groups.append([])
        else:
            groups[-1].append(token)
    return [group for group in groups if group]


def go_is_named_group(group: list[GoToken]) -> bool:
    """Return True for a parameter group of the form `name Type`, not a bare type."""
    if len(group) < 2 or not go_is_name(group[0]):
        return False
    if group[1].text == ".":
        return False
    if group[1].text == "[":
        return len(group) > 2 and (group[2].text == "]" or group[2].kind == "number")
    return True


def go_parameters(tokens: list[GoToken], open_index: int, role: str) -> tuple[list[GoBinding], int]:
    """Return the names a parameter or result list binds, and the index of its closing bracket."""
    close = go_close(tokens, open_index)
    groups = go_split(tokens, open_index + 1, close)
    if not any(go_is_named_group(group) for group in groups):
        return [], close
    bindings: list[GoBinding] = []
    type_text = ""
    for group in reversed(groups):
        if go_is_named_group(group):
            type_text = "".join(token.text for token in group[1:])
            bindings.append(GoBinding(group[0].line, group[0].text, role, type_text))
        elif go_is_name(group[0]) and len(group) == 1:
            bindings.append(GoBinding(group[0].line, group[0].text, role, type_text))
    return bindings, close


def go_signature(tokens: list[GoToken], open_index: int) -> Iterator[GoBinding]:
    """Yield the parameters and named results of the signature whose `(` is at `open_index`."""
    if open_index >= len(tokens) or tokens[open_index].text != "(":
        return
    bindings, close = go_parameters(tokens, open_index, "parameter")
    yield from bindings
    if close + 1 < len(tokens) and tokens[close + 1].text == "(":
        yield from go_parameters(tokens, close + 1, "parameter")[0]


def go_function(tokens: list[GoToken], index: int) -> Iterator[GoBinding]:
    """Yield the names a `func` at `index` binds: receiver, name, parameters and results."""
    after = index + 1
    if after >= len(tokens):
        return
    if tokens[after].text == "(":
        close = go_close(tokens, after)
        named = go_is_name(tokens[close + 1]) if close + 1 < len(tokens) else False
        if named and close + 2 < len(tokens) and tokens[close + 2].text in "([":
            yield from go_parameters(tokens, after, "receiver")[0]
            after = close + 1
        else:
            yield from go_signature(tokens, after)
            return
    if go_is_name(tokens[after]):
        yield GoBinding(tokens[after].line, tokens[after].text, "function", "")
        after += 1
        if after < len(tokens) and tokens[after].text == "[":
            after = go_close(tokens, after) + 1
        yield from go_signature(tokens, after)


def go_names_at(tokens: list[GoToken], index: int, role: str) -> tuple[list[GoBinding], int]:
    """Read `a, b` at `index`; return the names and the index after them."""
    bindings: list[GoBinding] = []
    while index < len(tokens) and go_is_name(tokens[index]):
        bindings.append(GoBinding(tokens[index].line, tokens[index].text, role, ""))
        if index + 1 < len(tokens) and tokens[index + 1].text == ",":
            index += 2
        else:
            return bindings, index + 1
    return bindings, index


def go_block_starts(tokens: list[GoToken], open_index: int) -> Iterator[int]:
    """Yield the index of the first token of every entry of a parenthesised declaration block."""
    close = go_close(tokens, open_index)
    depth = 0
    for index in range(open_index + 1, close):
        token = tokens[index]
        if token.kind == "punct" and token.text in "([{":
            depth += 1
        elif token.kind == "punct" and token.text in ")]}":
            depth -= 1
        elif depth == 0 and token.first and tokens[index - 1].text not in GO_CONTINUATION:
            yield index


def go_declaration(tokens: list[GoToken], index: int) -> Iterator[GoBinding]:
    """Yield the names a `var`, `const` or `type` at `index` binds."""
    keyword = tokens[index].text
    role = "type" if keyword == "type" else "variable"
    if index + 1 >= len(tokens):
        return
    if tokens[index + 1].text == "(":
        starts = list(go_block_starts(tokens, index + 1))
    else:
        starts = [index + 1]
    for start in starts:
        names = go_names_at(tokens, start, role)[0]
        yield from names[:1] if keyword == "type" else names


def go_short_declaration(tokens: list[GoToken], index: int) -> Iterator[GoBinding]:
    """Yield the names on the left of the `:=` at `index`."""
    cursor = index - 1
    while cursor >= 0 and go_is_name(tokens[cursor]):
        token = tokens[cursor]
        yield GoBinding(token.line, token.text, "variable", "")
        if cursor >= 1 and tokens[cursor - 1].text == ",":
            cursor -= 2
        else:
            return


def go_names_a_type(tokens: list[GoToken], after: int, close: int) -> bool:
    """Return True when the type of a struct field follows its names, else it is embedded."""
    if after >= close or tokens[after].line != tokens[after - 1].line:
        return False
    following = tokens[after]
    if following.kind == "string" or following.text in {".", ";"}:
        return False
    if following.text == "[" and tokens[after - 2].text != ",":
        return tokens[after + 1].text == "]" or tokens[after + 1].kind == "number"
    return True


def go_fields(tokens: list[GoToken], open_index: int) -> Iterator[GoBinding]:
    """Yield the field names of the struct body that opens at `open_index`."""
    close = go_close(tokens, open_index)
    depth = 0
    for index in range(open_index + 1, close):
        token = tokens[index]
        if token.kind == "punct" and token.text in "([{":
            depth += 1
        elif token.kind == "punct" and token.text in ")]}":
            depth -= 1
        elif depth == 0 and token.first and tokens[index - 1].text != ",":
            names, after = go_names_at(tokens, index, "field")
            if names and go_names_a_type(tokens, after, close):
                yield from names


def go_interface_methods(tokens: list[GoToken], open_index: int) -> Iterator[GoBinding]:
    """Yield the method names, parameters and results of the interface body at `open_index`."""
    close = go_close(tokens, open_index)
    depth = 0
    for index in range(open_index + 1, close):
        token = tokens[index]
        if token.kind == "punct" and token.text in "([{":
            depth += 1
        elif token.kind == "punct" and token.text in ")]}":
            depth -= 1
        elif depth == 0 and token.first and go_is_name(token) and tokens[index + 1].text == "(":
            yield GoBinding(token.line, token.text, "function", "")
            yield from go_signature(tokens, index + 1)


def go_bindings(tokens: list[GoToken]) -> Iterator[GoBinding]:
    """Yield every name a Go file binds."""
    code = [token for token in tokens if token.kind not in {"linecomment", "blockcomment"}]
    for index, token in enumerate(code):
        if token.text == ":=":
            yield from go_short_declaration(code, index)
        elif token.kind != "ident":
            continue
        elif token.text == "func":
            yield from go_function(code, index)
        elif token.text in {"var", "const", "type"}:
            yield from go_declaration(code, index)
        elif token.text in {"struct", "interface"} and index + 1 < len(code):
            if code[index + 1].text == "{" and token.text == "struct":
                yield from go_fields(code, index + 1)
            elif code[index + 1].text == "{":
                yield from go_interface_methods(code, index + 1)


def go_name_message(binding: GoBinding, is_test: bool) -> str | None:
    """Return the failure message for a Go name that is too short or abbreviated, else None."""
    name = binding.name
    if name in GO_ALLOWED_NAMES or (is_test and name in GO_TEST_NAMES):
        return None
    if binding.role == "receiver" and len(name) <= GO_MAX_RECEIVER_LENGTH:
        return None
    if GO_IO_PARAMETERS.get(name) == binding.type_text and binding.role == "parameter":
        return None
    if len(name) < MIN_NAME_LENGTH:
        return "name %r is shorter than %d characters" % (name, MIN_NAME_LENGTH)
    for segment in GO_WORD.findall(name):
        if segment.lower() in ABBREVIATIONS:
            return "name %r abbreviates a word (%r)" % (name, segment.lower())
    return None


def go_comment_findings(tokens: list[GoToken]) -> Iterator[Finding]:
    """Yield the issue-reference, date and length findings of the comments of a Go file."""
    comments = [token for token in tokens if token.kind in {"linecomment", "blockcomment"}]
    code_lines = {
        line
        for token in tokens
        if token.kind not in {"linecomment", "blockcomment"}
        for line in range(token.line, token.end_line + 1)
    }
    comment_lines: list[int] = []
    for token in comments:
        for offset, text in enumerate(token.text.splitlines() or [""]):
            yield from comment_text_findings(token.line + offset, text, False)
            if token.first:
                comment_lines.append(token.line + offset)
    first_code_line = min(code_lines, default=0)
    run: list[int] = []
    for line in [*sorted(comment_lines), 0]:
        if run and line != run[-1] + 1:
            yield from go_run_findings(run, run[-1] + 1 in code_lines, first_code_line)
            run = []
        if line:
            run.append(line)


def go_run_findings(run: list[int], is_doc: bool, first_code_line: int) -> Iterator[Finding]:
    """Yield the length finding of one run of whole-line Go comments, if it is too long."""
    if len(run) <= MAX_COMMENT_BLOCK:
        return
    if is_doc:
        message = "doc comment of %d lines is longer than %d"
    elif run[0] < first_code_line or first_code_line == 0:
        return
    else:
        message = "comment block of %d lines is longer than %d"
    yield run[0], message % (len(run), MAX_COMMENT_BLOCK)


def check_go(path: pathlib.Path, source: str) -> Iterator[Finding]:
    """Yield the findings of one Go file."""
    tokens = go_tokens(source)
    is_test = path.name.endswith("_test.go")
    for binding in go_bindings(tokens):
        message = go_name_message(binding, is_test)
        if message:
            yield binding.line, message
    yield from go_comment_findings(tokens)


CHECKERS: dict[str, Callable[[pathlib.Path, str], Iterator[Finding]]] = {
    "python": check_python,
    "sh": check_sh,
    "powershell": check_powershell,
    "go": check_go,
}


def display(path: pathlib.Path) -> str:
    """Return `path` relative to the repository root when it is inside it."""
    try:
        return str(path.resolve().relative_to(ROOT))
    except ValueError:
        return str(path)


def check_file(path: pathlib.Path) -> list[str]:
    """Return the formatted findings of one file, none for a file in no checked language."""
    language = language_of(path)
    if language is None:
        return []
    source = path.read_text(encoding="utf-8", errors="surrogateescape")
    findings = sorted(set(CHECKERS[language](path, source)))
    return ["%s:%d: %s" % (display(path), line, message) for line, message in findings]


def main(arguments: list[str] | None = None) -> int:
    """Check the files given, or every code file of the repository, and return the exit status."""
    parser = argparse.ArgumentParser(description=(__doc__ or "").split("\n")[0])
    parser.add_argument("files", nargs="*", type=pathlib.Path)
    files = parser.parse_args(arguments).files or code_files()
    failures = [failure for path in files for failure in check_file(path)]
    for failure in failures:
        print(failure)
    if failures:
        print("%d finding(s)" % len(failures), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
