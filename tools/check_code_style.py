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
- a comment or docstring names no issue or pull request number and no date, a copyright line aside;
- a run of whole-line comments is at most ten lines long, the file header excepted; a
  docstring or Go doc comment ten, a module docstring thirty, a Go function body 60
  (`GO_LONG_FUNCTIONS` excepted); a Go package comment is in `doc.go`, which holds nothing
  else, at most 80 lines, though a `_test.go` file keeps its own of up to thirty lines;
- a Go test file under `cmd/` or `internal/` is named for a Go file of its package, and in a Go
  file there a function or method sits above those that name it, circles and shared names aside.

Quoted text on an sh, PowerShell or Go line is neither a comment nor scanned for names, and a
comment line inside an sh here-document is skipped. ruff's pydocstyle rules check the form of
the Python docstrings; this check does not.

Usage: tools/check_code_style.py [FILE...]. Every failure is printed as `path:line: message`,
then the count on stderr; the exit status is 1 when there is any failure, else 0.
"""

from __future__ import annotations

import argparse
import ast
import pathlib
import re
import subprocess
import sys
import tokenize
from collections.abc import Callable, Collection, Iterator
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
MAX_DOC_FILE_COMMENT_LINES = 80
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
GO_VERB = re.compile(r"%(?:%|[-+# 0]*(?:\d+|\*)?(?:\.(?:\d+|\*)?)?(?:\[\d+\])?[a-zA-Z])")
GO_FORMAT_CALLS = {
    "Sprintf": "`+`",
    "Errorf": "`errors.New` and `+`, or wrap an error among the values with %w (or reason.Wrap)",
}
GO_FORMAT_MESSAGE = "fmt.%s only joins strings (every verb is %%s, or %%v of err); use %s"
GO_NO_VERB_CALLS = {"Sprintf": "the string itself", "Errorf": "`errors.New`"}
GO_NO_VERB_MESSAGE = "fmt.%s has no verb to fill; use %s"
GO_ERRORS_NEW_FORMAT_MESSAGE = (
    "errors.New(fmt.Sprintf(...)) makes an error from a format; use fmt.Errorf"
)
GO_NO_COMMENT_MESSAGE = "%s %r has no comment directly above it"
GO_DROPPED_CAUSE_MESSAGE = (
    "errors.New makes an error from another error's text and drops it; wrap it with %w"
)
GO_ERRORF_DROPPED_MESSAGE = (
    "fmt.Errorf formats an error with a verb other than %w and drops it from the chain;"
    " wrap it with %w (or reason.Wrap)"
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
# A function body is the lines between its braces; internal/AGENTS.md says it stays under 40,
# never over 60.
GO_MAX_FUNCTION_LINES = 60
GO_PACKAGE_COMMENT_MESSAGE = "a package comment belongs in doc.go"
GO_DOC_FILE_MESSAGE = "doc.go holds only the package comment and the package clause"
GO_LONG_FUNCTION_MESSAGE = "%s %r has a body of %d lines, over the limit of %d"
# The build suffixes a Go file or test file name may end with; one is dropped before names are
# compared, so `outside_unix_test.go` tests `outside_unix.go` or `outside.go`.
GO_BUILD_SUFFIXES = ("windows", "unix", "linux", "darwin", "other", "race", "norace")
GO_TESTED_DIRECTORIES = {"cmd", "internal"}
GO_TEST_FILE_NAME_MESSAGE = (
    "test file is not named for a Go file of its package:"
    " name it <file>_test.go or <file>_<topic>_test.go"
)
GO_ORDER_MESSAGE = "%s %r is defined below %r, which names it; define it above"
# Functions over the limit that stay as they are: each is one reviewed list that is audited as a
# whole, and reads best unbroken. Each entry is a file of the repository and a function name. An
# entry that no longer names a function over the limit fails the tests.
GO_LONG_FUNCTIONS = {
    ("internal/follow/confine_sites_test.go", "siteClasses"),
    ("internal/follow/follow_guard_lists_test.go", "reviewedImports"),
    ("internal/follow/follow_guard_lists_test.go", "allowedCalls"),
}
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


def third_party(path: str) -> bool:
    """Report whether a repository path holds source vendored from another project."""
    return path.startswith("vendor/")


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
    files = {ROOT / path for path in listing.split("\0") if path and not third_party(path)}
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


def comment_text_findings(line: int, text: str) -> Iterator[Finding]:
    """Yield the issue-reference and date findings of one comment or docstring line."""
    if COPYRIGHT_LINE.search(text):
        return
    for match in ISSUE_REFERENCE.finditer(text):
        yield line, "comment names an issue or pull request: %r" % match.group(0)
    for match in DATE.finditer(text):
        yield line, "comment holds a date: %r" % match.group(0)


def comment_block_findings(lines: list[str]) -> Iterator[Finding]:
    """Yield a finding for every run of whole-line `#` comments over the cap.

    The run that opens the file (its header) is exempt.
    """
    start = None
    for index, text in enumerate([*lines, ""], start=1):
        is_comment = text.lstrip().startswith("#")
        if is_comment and start is None:
            start = index
        elif not is_comment and start is not None:
            length = index - start
            if length > MAX_COMMENT_BLOCK and start > 2:
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
            yield from comment_text_findings(first_line + offset, text)
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
                yield from comment_text_findings(token.start[0], token.string)
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
    for index, text in enumerate(lines, start=1):
        comment = split_comment(text)[1]
        if not comment or text.lstrip().startswith("#!"):
            continue
        yield from comment_text_findings(index, comment)
    yield from comment_block_findings(lines)


def check_powershell(path: pathlib.Path, source: str) -> Iterator[Finding]:
    """Yield the findings of one PowerShell file."""
    lines = source.splitlines()
    for index, text in enumerate(lines, start=1):
        code, comment = split_comment(text)
        for match in PS_VARIABLE.finditer(code):
            name = match.group(1)
            message = None if name.lower() in PS_AUTOMATIC else name_message(name)
            if message:
                yield index, message
        if comment:
            yield from comment_text_findings(index, comment)
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


def go_code_tokens(tokens: list[GoToken]) -> list[GoToken]:
    """Return a Go file's tokens without its comments, each first when no code precedes it.

    A token is marked as starting its line when no other code token comes before it there: a
    comment before a token on the same line (`/* doc */ run = func() {`) does not stop the
    token from starting the line's code, so every check that looks for a statement's first
    token reads the line as if the comment were not there.
    """
    code: list[GoToken] = []
    for token in tokens:
        if token.kind not in {"linecomment", "blockcomment"}:
            code.append(token._replace(first=not code or code[-1].end_line != token.line))
    return code


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


def go_split(
    tokens: list[GoToken], start: int, end: int, separator: str = ","
) -> list[list[GoToken]]:
    """Split the tokens between `start` and `end` (exclusive) at their top-level `separator`."""
    groups: list[list[GoToken]] = [[]]
    depth = 0
    for token in tokens[start:end]:
        if token.kind == "punct" and token.text in "([{":
            depth += 1
        elif token.kind == "punct" and token.text in ")]}":
            depth -= 1
        if token.kind == "punct" and token.text == separator and depth == 0:
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


def go_starts_field(tokens: list[GoToken], index: int, inline: bool) -> bool:
    """Return True when the token at `index` of a struct body may start a field's names."""
    before = tokens[index - 1].text
    return before != "," and (tokens[index].first or (inline and before in {"{", ";"}))


def go_fields(tokens: list[GoToken], open_index: int, inline: bool = False) -> Iterator[GoBinding]:
    """Yield the field names of the struct body that opens at `open_index`.

    A field starts its line; with `inline`, one after the `{` or a `;` on the same line
    (`struct{ name string }`) counts too.
    """
    close = go_close(tokens, open_index)
    depth = 0
    for index in range(open_index + 1, close):
        token = tokens[index]
        if token.kind == "punct" and token.text in "([{":
            depth += 1
        elif token.kind == "punct" and token.text in ")]}":
            depth -= 1
        elif depth == 0 and go_starts_field(tokens, index, inline):
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
    code = go_code_tokens(tokens)
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
        # Go spells an error value with the word err (closeErr, errNotFound),
        # and staticcheck requires that form of a package's error variables.
        if segment.lower() != "err" and segment.lower() in ABBREVIATIONS:
            return "name %r abbreviates a word (%r)" % (name, segment.lower())
    return None


def go_comment_findings(tokens: list[GoToken], file_name: str) -> Iterator[Finding]:
    """Yield the issue-reference, date, length and placement findings of a Go file's comments.

    `file_name` is the file's base name, which decides where its package comment may live.
    """
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
            yield from comment_text_findings(token.line + offset, text)
            if token.first:
                comment_lines.append(token.line + offset)
    first_code_line = min(code_lines, default=0)
    run: list[int] = []
    for line in [*sorted(comment_lines), 0]:
        if run and line != run[-1] + 1:
            if run[-1] + 1 == first_code_line:
                yield from go_package_comment_findings(run, file_name)
            else:
                yield from go_run_findings(run, run[-1] + 1 in code_lines, first_code_line)
            run = []
        if line:
            run.append(line)


def go_package_comment_findings(run: list[int], file_name: str) -> Iterator[Finding]:
    """Yield the finding of a package comment, the run of comments directly above `package`.

    In `doc.go` it may run to MAX_DOC_FILE_COMMENT_LINES. A test file keeps its own, under
    the cap of a Python module docstring; in any other file it belongs in `doc.go`, and one
    finding says so, with its length when that is also over the cap.
    """
    if file_name == "doc.go":
        if len(run) > MAX_DOC_FILE_COMMENT_LINES:
            message = "doc comment of %d lines is longer than %d"
            yield run[0], message % (len(run), MAX_DOC_FILE_COMMENT_LINES)
        return
    cap = MAX_MODULE_DOCSTRING_LINES
    if file_name.endswith("_test.go"):
        if len(run) > cap:
            yield run[0], "doc comment of %d lines is longer than %d" % (len(run), cap)
        return
    message = GO_PACKAGE_COMMENT_MESSAGE
    if len(run) > cap:
        message = "package comment of %d lines is longer than %d; %s" % (len(run), cap, message)
    yield run[0], message


def go_doc_file_findings(tokens: list[GoToken], file_name: str) -> Iterator[Finding]:
    """Yield a finding at the first code after the package clause of a `doc.go` file."""
    if file_name != "doc.go":
        return
    code = go_code_tokens(tokens)
    if len(code) > 2:
        yield code[2].line, GO_DOC_FILE_MESSAGE


def go_run_findings(run: list[int], is_doc: bool, first_code_line: int) -> Iterator[Finding]:
    """Yield the length finding of one run of whole-line Go comments, if it is too long."""
    cap = MAX_COMMENT_BLOCK
    if len(run) <= cap:
        return
    if is_doc:
        message = "doc comment of %d lines is longer than %d"
    elif run[0] < first_code_line or first_code_line == 0:
        return
    else:
        message = "comment block of %d lines is longer than %d"
    yield run[0], message % (len(run), cap)


def go_literal_text(tokens: list[GoToken], start: int) -> tuple[str, int] | None:
    """Return the string literals joined by `+` from `start`, and the index after them.

    None when there is no string literal there.
    """
    parts: list[str] = []
    index = start
    while index < len(tokens) and (
        tokens[index].kind in ("string", "raw") or tokens[index].text == "+"
    ):
        if tokens[index].kind in ("string", "raw"):
            parts.append(tokens[index].text[1:-1])
        index += 1
    if not parts:
        return None
    return "".join(parts), index


def go_string_constants(tokens: list[GoToken]) -> dict[str, str]:
    """Return the text of each constant the file declares as string literals alone.

    `const layout = "%s: %s"` and the same entry in a `const (...)` block count; a constant of
    another type, or one whose value is more than literals joined by `+`, does not.
    """
    code = go_code_tokens(tokens)
    constants: dict[str, str] = {}
    for index, token in enumerate(code):
        if token.text != "const" or index + 1 >= len(code):
            continue
        if code[index + 1].text == "(":
            starts = list(go_block_starts(code, index + 1))
        else:
            starts = [index + 1]
        for start in starts:
            names, after = go_names_at(code, start, "variable")
            if len(names) != 1 or after >= len(code):
                continue
            if code[after].text == "string":
                after += 1
            if after >= len(code) or code[after].text != "=":
                continue
            literal = go_literal_text(code, after + 1)
            if literal is None:
                continue
            text, end = literal
            ends_entry = end >= len(code) or code[end].line != code[end - 1].end_line
            if ends_entry or code[end].text in (")", ";"):
                constants[names[0].name] = text
    return constants


def go_format_argument(tokens: list[GoToken], start: int, constants: dict[str, str]) -> str | None:
    """Return the format text of the first argument at `start`, None when it is not known.

    The format is known when the argument is string literals joined by `+`, or the name of a
    string constant of the same file.
    """
    literal = go_literal_text(tokens, start)
    if literal is not None:
        text, end = literal
        return text if end < len(tokens) and tokens[end].text in (",", ")") else None
    name = tokens[start]
    if name.kind == "ident" and name.text in constants and tokens[start + 1].text in (",", ")"):
        return constants[name.text]
    return None


def go_is_call(tokens: list[GoToken], index: int, package: str, names: Collection[str]) -> bool:
    """Return True when the tokens at `index` start a call `package.Name(` of one of `names`."""
    return (
        index >= 2
        and index + 1 < len(tokens)
        and tokens[index].text in names
        and tokens[index + 1].text == "("
        and tokens[index - 1].text == "."
        and tokens[index - 2].text == package
    )


def go_plain_verbs(call: str, verbs: list[str], arguments: list[list[GoToken]]) -> bool:
    """Return True when every verb only copies text into the result.

    A verb does when it is a plain `%s`, or, in `fmt.Sprintf`, a `%v` whose argument is the
    error `err`, which writes `err.Error()`. `arguments` are the call's arguments after the
    format; in `fmt.Errorf` the error is wrapped with `%w` instead.
    """
    for position, verb in enumerate(verbs):
        if verb == "%s":
            continue
        argument = arguments[position] if position < len(arguments) else []
        is_error = len(argument) == 1 and argument[0].text == "err"
        if not (call == "Sprintf" and verb == "%v" and is_error):
            return False
    return True


GO_EXPORTED_ERROR = re.compile(r"^Err[A-Z]")
GO_ERROR_NAME = re.compile(r"Err$|^err[A-Z]|^Err[A-Z]")
GO_ERROR_MAKERS = (("errors", "New"), ("errors", "Join"), ("fmt", "Errorf"))


GO_NAME_LIST_STARTS = {"(", ",", "{", ";", "var"}


def go_name_list(code: list[GoToken], last: int) -> list[GoToken]:
    """Return the names of the comma-separated list that ends at the name at `last`.

    In `readErr, writeErr error` the list that ends at `writeErr` holds both names. A name
    joins the list only when it follows `(`, `,`, `{`, `;` or `var`, or starts a line; the type
    of the group before (`int` in `count int, closeErr error`) follows a name and does not.
    """
    names = [code[last]]
    index = last
    while index >= 2 and code[index - 1].text == "," and code[index - 2].kind == "ident":
        member = code[index - 2]
        if not (member.first or index < 3 or code[index - 3].text in GO_NAME_LIST_STARTS):
            break
        names.append(member)
        index -= 2
    return names


def go_item_ranges(code: list[GoToken], start: int, end: int) -> list[tuple[int, int]]:
    """Return the (start, end) token ranges of the items between `start` and `end`, split at `,`."""
    ranges: list[tuple[int, int]] = []
    depth = 0
    item = start
    for index in range(start, end):
        token = code[index]
        if token.kind == "punct" and token.text in "([{":
            depth += 1
        elif token.kind == "punct" and token.text in ")]}":
            depth -= 1
        elif token.kind == "punct" and token.text == "," and depth == 0:
            ranges.append((item, index))
            item = index + 1
    if item < end:
        ranges.append((item, end))
    return ranges


def go_assignment_left(code: list[GoToken], operator: int) -> list[tuple[int, int]]:
    """Return the token ranges of the comma-separated items left of the assignment at `operator`.

    The items are the names, selectors and indexed values that start the statement: the walk
    back from the operator stops at a keyword (`var`, `if`), a `;`, an enclosing bracket, or a
    line that does not continue a list with a `,`.
    """
    index = operator - 1
    depth = 0
    start = operator
    while index >= 0:
        token = code[index]
        if token.kind == "punct" and token.text in ")]}":
            depth += 1
        elif token.kind == "punct" and token.text in "([{":
            depth -= 1
            if depth < 0:
                break
        elif depth == 0 and not (
            (token.kind == "ident" and token.text not in GO_KEYWORDS)
            or token.text in {",", ".", "*"}
        ):
            break
        start = index
        if depth == 0 and token.first and (index == 0 or code[index - 1].text != ","):
            break
        index -= 1
    return go_item_ranges(code, start, operator)


def go_assignment_right(code: list[GoToken], operator: int) -> list[tuple[int, int]]:
    """Return the token ranges of the comma-separated items right of the assignment at `operator`.

    The statement ends at a `;`, at a closing bracket that is not its own, or at a line end
    that does not continue with a `,` or an operator.
    """
    depth = 0
    end = operator + 1
    while end < len(code):
        token = code[end]
        if depth == 0 and (token.text == ";" or token.text in ")]}"):
            break
        if depth == 0 and token.first and end > operator + 1:
            before = code[end - 1]
            if before.kind != "punct" or before.text in ")]}":
                break
        if token.kind == "punct" and token.text in "([{":
            depth += 1
        elif token.kind == "punct" and token.text in ")]}":
            depth -= 1
        end += 1
    return go_item_ranges(code, operator + 1, end)


def go_is_error_maker_call(code: list[GoToken], start: int) -> bool:
    """Return True when the tokens at `start` begin a call of `errors.New`, `errors.Join` or `fmt.Errorf`."""
    return any(go_is_call(code, start + 2, package, {name}) for package, name in GO_ERROR_MAKERS)


def go_assigned_errors(code: list[GoToken], operator: int) -> Iterator[str]:
    """Yield the names that the assignment at `operator` gives the result of an error maker.

    The names on the left are paired with the values on the right by position, and only when
    both sides have the same number of items: a call on the right that returns several values
    cannot be paired from the tokens.
    """
    left = go_assignment_left(code, operator)
    right = go_assignment_right(code, operator)
    if len(left) != len(right):
        return
    for (target, target_end), (value, _) in zip(left, right, strict=True):
        is_name = target_end - target == 1 and code[target].kind == "ident"
        if is_name and go_is_error_maker_call(code, value):
            yield code[target].text


def go_declared_errors(tokens: list[GoToken]) -> set[str]:
    """Return the names ending in `Err` or starting with `err` or `Err` and a capital that are errors.

    A name is listed when the file declares it with the type `error` (`var closeErr error`, a
    parameter, a result or a struct field) or assigns it the result of `errors.New`,
    `errors.Join` or `fmt.Errorf`. Every name of a group that shares a type is declared with it
    (`readErr, writeErr error`); in `count, readErr := 1, errors.New("x")` names and values are
    paired by position when there are as many of one as of the other. A name assigned from any
    other call (`closeErr := file.Close()`) is not listed. Names are not told apart by scope.
    """
    code = go_code_tokens(tokens)
    names: set[str] = set()
    for index in range(1, len(code) - 1):
        token = code[index]
        if code[index - 1].kind == "ident" and token.kind == "ident" and token.text == "error":
            names.update(name.text for name in go_name_list(code, index - 1))
        is_assignment = token.text == ":=" or (token.text == "=" and code[index + 1].text != "=")
        if is_assignment:
            names.update(go_assigned_errors(code, index))
    return {name for name in names if GO_ERROR_NAME.search(name)}


def go_is_selector_chain(argument: list[GoToken]) -> bool:
    """Return True when the tokens are a name, or names joined by dots (`current.err`)."""
    return (
        len(argument) % 2 == 1
        and all(token.kind == "ident" for token in argument[::2])
        and all(token.text == "." for token in argument[1::2])
    )


def go_names_an_error(argument: list[GoToken], declared: set[str]) -> bool:
    """Return True when a call argument, or one `+` operand of one, is an error.

    It is only when the whole of it is: a call of `.Error()`; the name `err` or a chain ending
    in `err` (`current.err`); a chain of two or more names ending in one that starts with `Err`
    and a capital (`fs.ErrNotExist`); or a name or chain ending in a name `go_declared_errors`
    lists, which includes a bare `ErrGone` the file declares as an error. A bare `ErrGone` the
    file does not declare may be a constant, a type or another package's value, and is left
    alone, as is an error only inside a larger expression (`wrap(err)`, `list[0]`).
    """
    if len(argument) >= 4 and [token.text for token in argument[-4:]] == [".", "Error", "(", ")"]:
        return True
    if not go_is_selector_chain(argument):
        return False
    last = argument[-1].text
    if last == "err":
        return True
    if len(argument) > 1 and GO_EXPORTED_ERROR.match(last):
        return True
    return last in declared


def go_argument_text(argument: list[GoToken]) -> str:
    """Return the token text of a call argument, to compare two arguments for sameness."""
    return " ".join(token.text for token in argument)


def go_errorf_drops_error(
    verbs: list[str], arguments: list[list[GoToken]], declared: set[str]
) -> bool:
    """Return True when a `fmt.Errorf` formats an error with a verb other than `%w`.

    A verb that copies the error's words (`%v`, `%s`, `%q`) leaves the error out of the chain
    `errors.Is` and `errors.As` follow. `%T` writes the error's type name and `%p` its address,
    not its words, so those two verbs are left alone. An argument is an error as
    `go_names_an_error` says; one whose whole text is also given to a `%w` of the same call
    stays in the chain and passes. A format with an explicit argument index (`%[2]v`) or a
    width or precision taken from an argument (`%*d`) is not paired and is left alone.
    """
    if any("[" in verb or "*" in verb for verb in verbs):
        return False
    pairs = list(zip(verbs, arguments, strict=False))
    wrapped = {go_argument_text(argument) for verb, argument in pairs if verb == "%w"}
    return any(
        verb != "%w"
        and not verb.endswith(("T", "p"))
        and go_names_an_error(argument, declared)
        and go_argument_text(argument) not in wrapped
        for verb, argument in pairs
    )


def go_format_message(
    call: str,
    verbs: list[str],
    arguments: list[list[GoToken]],
    in_errors_new: bool,
    declared: set[str],
) -> str | None:
    """Return the finding for a `fmt.Sprintf` or `fmt.Errorf` call, None when it passes."""
    if in_errors_new:
        return GO_ERRORS_NEW_FORMAT_MESSAGE
    if call == "Errorf" and go_errorf_drops_error(verbs, arguments, declared):
        return GO_ERRORF_DROPPED_MESSAGE
    if not verbs:
        return GO_NO_VERB_MESSAGE % (call, GO_NO_VERB_CALLS[call])
    if go_plain_verbs(call, verbs, arguments):
        return GO_FORMAT_MESSAGE % (call, GO_FORMAT_CALLS[call])
    return None


def go_format_findings(tokens: list[GoToken]) -> Iterator[Finding]:
    """Yield a finding for each `fmt.Sprintf` or `fmt.Errorf` that only joins strings.

    A format whose verbs are all a plain `%s` converts, quotes and pads nothing, so the
    strings are joined with `+` (and the error made with `errors.New`). A `%s` or `%v` of an
    error, or a `%s` of a value with a String method, is written as its `Error()` or `String()`
    call. A format with no verb (`%%` is not one) is the text itself, and
    `errors.New(fmt.Sprintf(...))` is `fmt.Errorf`. The format is read from a string literal or
    from a string constant the same file declares.
    """
    code = go_code_tokens(tokens)
    constants = go_string_constants(tokens)
    declared = go_declared_errors(tokens)
    for index in range(2, len(code) - 2):
        token = code[index]
        if not go_is_call(code, index, "fmt", GO_FORMAT_CALLS):
            continue
        format_text = go_format_argument(code, index + 2, constants)
        if format_text is None:
            continue
        verbs = [verb for verb in GO_VERB.findall(format_text) if verb != "%%"]
        arguments = go_split(code, index + 2, go_close(code, index + 1))[1:]
        in_errors_new = token.text == "Sprintf" and go_is_call(code, index - 4, "errors", {"New"})
        message = go_format_message(token.text, verbs, arguments, in_errors_new, declared)
        if message:
            yield token.line, message


def go_dropped_cause_findings(tokens: list[GoToken]) -> Iterator[Finding]:
    """Yield a finding for each `errors.New` whose text is made from an error.

    A `+` operand of the argument that is an error (see `go_names_an_error`) copies an error's
    words into a new error and drops the error itself, so `errors.Is` and `errors.As` no longer
    find it. Library code wraps it with `%w` instead.
    """
    code = go_code_tokens(tokens)
    declared = go_declared_errors(tokens)
    for index in range(2, len(code) - 1):
        if not go_is_call(code, index, "errors", {"New"}):
            continue
        close = go_close(code, index + 1)
        operands = go_split(code, index + 2, close, "+")
        if any(go_names_an_error(operand, declared) for operand in operands):
            yield code[index].line, GO_DROPPED_CAUSE_MESSAGE


def go_declared_functions(tokens: list[GoToken]) -> Iterator[tuple[GoToken, str, bool]]:
    """Yield the `func` token, name and whether it has a receiver of each top-level declaration.

    A declaration's `func` starts its line outside every bracket; a function literal is inside a
    body or a declaration, or follows `=` or another operator.
    """
    code = go_code_tokens(tokens)
    for index, name, has_receiver in go_declarations_in(code):
        yield code[index], name, has_receiver


def go_declarations_in(code: list[GoToken]) -> Iterator[tuple[int, str, bool]]:
    """Yield the `func` index, name and whether it has a receiver of each top-level declaration.

    `code` is a file's tokens as `go_code_tokens` returns them.
    """
    depth = 0
    for index, token in enumerate(code):
        if token.kind == "punct" and token.text in "([{":
            depth += 1
        elif token.kind == "punct" and token.text in ")]}":
            depth -= 1
        is_start = depth == 0 and token.first and token.text == "func" and index + 1 < len(code)
        if not is_start or (index > 0 and code[index - 1].text in GO_CONTINUATION):
            continue
        after = index + 1
        has_receiver = code[after].text == "("
        if has_receiver:
            after = go_close(code, after) + 1
        if after < len(code) and go_is_name(code[after]):
            yield index, code[after].text, has_receiver


def go_is_test_function(name: str) -> bool:
    """Return True for a name `go test` runs: Test, Benchmark, Fuzz or Example and a non-lower."""
    for prefix in ("Test", "Benchmark", "Fuzz", "Example"):
        rest = name.removeprefix(prefix)
        if rest != name and not rest[:1].islower():
            return True
    return False


def go_function_comment_findings(tokens: list[GoToken], is_test: bool) -> Iterator[Finding]:
    """Yield a finding for each function or method declaration with no comment above it.

    The comment ends on the line directly above the `func`, past any `//go:` directive lines;
    a blank line between them ends it. `main` and `init`, which the package comment and the
    embedded file they check explain, and in a test file the functions `go test` runs, are
    exempt.
    """
    comment_lines: set[int] = set()
    directive_lines: set[int] = set()
    for token in tokens:
        if token.kind in {"linecomment", "blockcomment"} and token.first:
            lines = directive_lines if token.text.startswith("//go:") else comment_lines
            lines.update(range(token.line, token.end_line + 1))
    for token, name, has_receiver in go_declared_functions(tokens):
        if not has_receiver and (
            name in {"main", "init"} or (is_test and go_is_test_function(name))
        ):
            continue
        line = token.line - 1
        while line in directive_lines:
            line -= 1
        if line not in comment_lines:
            kind = "method" if has_receiver else "function"
            yield token.line, GO_NO_COMMENT_MESSAGE % (kind, name)


def go_body_open(code: list[GoToken], start: int) -> int | None:
    """Return the index of the `{` that opens the body of the `func` at `start`, None if none.

    The signature's brackets are skipped, and so is the `{` of a `struct` or `interface` type
    in it. A declaration with no body ends where the next declaration starts.
    """
    depth = 0
    index = start + 1
    while index < len(code):
        token = code[index]
        if token.kind == "punct" and token.text in "([":
            depth += 1
        elif token.kind == "punct" and token.text in ")]":
            depth -= 1
        elif depth == 0 and token.text == "{":
            if code[index - 1].text not in {"struct", "interface"}:
                return index
            index = go_close(code, index)
        elif depth == 0 and token.first and token.text in {"func", "type", "var", "const"}:
            return None
        index += 1
    return None


def go_package_literals(code: list[GoToken]) -> Iterator[tuple[int, str]]:
    """Yield the index of the `func` and the name of each function literal a package variable holds.

    That is `var run = func() {` at the top level or inside a `var (` block, the same with the
    `func` on the line after the `=` or in parentheses, and a literal among several values
    (`var run, stop = func() {`, `}, func() {}`), where each name is paired with its value by
    position. A variable with a type written (`var run handler = func() {`) is not read, and a
    literal inside a function body is part of its body.
    """
    openers: list[int] = []
    for index, token in enumerate(code):
        if token.kind == "punct" and token.text in "([{":
            openers.append(index)
        elif token.kind == "punct" and token.text in ")]}" and openers:
            openers.pop()
        elif token.text == "=" and go_is_package_var_context(code, openers):
            yield from go_literals_of_assignment(code, index)


def go_is_package_var_context(code: list[GoToken], openers: list[int]) -> bool:
    """Return True when the open brackets are none (top level) or one `var (` block."""
    if not openers:
        return True
    opener = openers[0]
    return (
        len(openers) == 1
        and code[opener].text == "("
        and opener > 0
        and code[opener - 1].text == "var"
    )


def go_literals_of_assignment(code: list[GoToken], operator: int) -> Iterator[tuple[int, str]]:
    """Yield the (`func` index, name) of each literal the `var` assignment at `operator` holds."""
    left = go_assignment_left(code, operator)
    right = go_assignment_right(code, operator)
    if not left or len(left) != len(right):
        return
    head = left[0][0]
    names = all(end - start == 1 and code[start].kind == "ident" for start, end in left)
    if not names or not (code[head].first or (head > 0 and code[head - 1].text == "var")):
        return
    for (target, _), (value, value_end) in zip(left, right, strict=True):
        while value < value_end and code[value].text == "(":
            value += 1
        if value < value_end and code[value].text == "func":
            yield value, code[target].text


def go_long_functions(tokens: list[GoToken]) -> Iterator[tuple[int, str, bool, int]]:
    """Yield the line, name, whether a method and body length of each top-level function.

    That is each declared function and method, and each function literal a package variable
    holds, named by the variable. The length is the number of lines between the braces of the
    body.
    """
    code = go_code_tokens(tokens)
    starts = list(go_declarations_in(code))
    starts += [(index, name, False) for index, name in go_package_literals(code)]
    for start, name, has_receiver in sorted(starts):
        opening = go_body_open(code, start)
        if opening is not None:
            closing = go_close(code, opening)
            yield (
                code[start].line,
                name,
                has_receiver,
                code[closing].line - code[opening].end_line - 1,
            )


def go_function_length_findings(path: pathlib.Path, tokens: list[GoToken]) -> Iterator[Finding]:
    """Yield a finding for each function or method whose body is over `GO_MAX_FUNCTION_LINES`.

    A function listed in `GO_LONG_FUNCTIONS` for this file is passed over.
    """
    for line, name, has_receiver, length in go_long_functions(tokens):
        if length > GO_MAX_FUNCTION_LINES and (go_relative(path), name) not in GO_LONG_FUNCTIONS:
            kind = "method" if has_receiver else "function"
            yield line, GO_LONG_FUNCTION_MESSAGE % (kind, name, length, GO_MAX_FUNCTION_LINES)


class GoFunction(NamedTuple):
    """A top-level function or method: its `func` index, name, and the braces of its body.

    `body_open` and `body_close` are token indices, both None for a declaration with no body.
    """

    start: int
    name: str
    has_receiver: bool
    body_open: int | None
    body_close: int | None


def go_functions(code: list[GoToken]) -> list[GoFunction]:
    """Return the top-level functions and methods of a file's code tokens, in file order."""
    functions: list[GoFunction] = []
    for start, name, has_receiver in go_declarations_in(code):
        opening = go_body_open(code, start)
        closing = None if opening is None else go_close(code, opening)
        functions.append(GoFunction(start, name, has_receiver, opening, closing))
    return functions


def go_named_functions(code: list[GoToken], functions: list[GoFunction]) -> list[list[int]]:
    """Return, for each function, the positions in `functions` of the functions its body names.

    A body, function literals included, names a method with its name after a `.`, as
    `go_method_target` reads it, unless the `.` follows an import name of the file that the
    function does not bind (`go_is_qualified`), and a plain function with its bare name, unless the function binds
    that name itself (a receiver, parameter, result or local, anywhere in it) or the name is a
    label or a field key (`go_is_label_or_key`). A name declared by more than one method, or by
    more than one plain function, of the file (two types' `Close`) names none.
    """
    methods = go_unique_names(functions, has_receiver=True)
    plain = go_unique_names(functions, has_receiver=False)
    fields = go_field_names(code)
    imports = go_import_names(code)
    named: list[list[int]] = []
    for function in functions:
        targets: set[int] = set()
        if function.body_open is not None and function.body_close is not None:
            bound = go_names_bound_in(code[function.start : function.body_close + 1])
            for index in range(function.body_open + 1, function.body_close):
                if code[index].kind != "ident":
                    continue
                if code[index - 1].text == "." and go_is_qualified(code, index, imports - bound):
                    target = None
                elif code[index - 1].text == ".":
                    target = go_method_target(code, index, methods, fields)
                elif code[index].text in bound or go_is_label_or_key(code, index):
                    target = None
                else:
                    target = plain.get(code[index].text)
                if target is not None:
                    targets.add(target)
        named.append(sorted(targets))
    return named


def go_import_names(code: list[GoToken]) -> set[str]:
    """Return the names a file's imports are used by: each explicit name, else the path's last.

    The last element of the path skips a major-version suffix (`/v2`); an import named `_` or
    `.` gives no name. The package a path holds may be named otherwise, so this is by spelling.
    """
    names: set[str] = set()
    for index, token in enumerate(code):
        if token.text != "import" or index + 1 >= len(code):
            continue
        end = go_close(code, index + 1) if code[index + 1].text == "(" else index + 3
        for position in range(index + 1, min(end, len(code))):
            if code[position].kind not in {"string", "raw"}:
                continue
            before = code[position - 1]
            if before.kind == "ident" and before.text != "import":
                names.add(before.text)
            elif before.text != ".":
                elements = code[position].text[1:-1].split("/")
                if len(elements) > 1 and re.fullmatch(r"v\d+", elements[-1]):
                    elements.pop()
                names.add(elements[-1])
    names.discard("_")
    return names


def go_is_qualified(code: list[GoToken], index: int, imports: set[str]) -> bool:
    """Return True when the name after the `.` at `index` is qualified by an import name.

    The left operand must be that one name (`strings.Join`), not a chain (`row.strings.Join`).
    """
    left = code[index - 2]
    return left.kind == "ident" and left.text in imports and code[index - 3].text != "."


def go_field_names(code: list[GoToken]) -> set[str]:
    """Return the field names of every struct type of a file, one-line ones included."""
    return {
        binding.name
        for index in range(len(code) - 1)
        if code[index].text == "struct" and code[index + 1].text == "{"
        for binding in go_fields(code, index + 1, inline=True)
    }


def go_method_target(
    code: list[GoToken], index: int, methods: dict[str, int], fields: set[str]
) -> int | None:
    """Return the method a name after a `.` at `index` names, None when it names none.

    `methods` maps the file's method names that no other method shares to their positions, and
    `fields` holds the field names of every struct type of the file. A name that is also a
    field and is not called (no `(` follows it) is read as the field, not a method value.
    """
    name = code[index].text
    if name in fields and code[index + 1].text != "(":
        return None
    return methods.get(name)


def go_is_label_or_key(code: list[GoToken], index: int) -> bool:
    """Return True when the name at `index` is a label or a composite literal's field key.

    That is a name followed by `:` (not `:=`), or one after `break`, `continue` or `goto`.
    """
    return code[index + 1].text == ":" or code[index - 1].text in {"break", "continue", "goto"}


def go_unique_names(functions: list[GoFunction], has_receiver: bool) -> dict[str, int]:
    """Return the position of each method (or plain function) whose name no other one shares."""
    positions: dict[str, list[int]] = {}
    for position, function in enumerate(functions):
        if function.has_receiver == has_receiver:
            positions.setdefault(function.name, []).append(position)
    return {name: places[0] for name, places in positions.items() if len(places) == 1}


def go_names_bound_in(code: list[GoToken]) -> set[str]:
    """Return the names a function declaration binds besides its own: receiver, parameters, locals.

    `code` is the declaration's code tokens, from `func` to the closing brace of its body.
    """
    roles = {"receiver", "parameter", "variable", "type"}
    return {binding.name for binding in go_bindings(code) if binding.role in roles}


def go_circles(named: list[list[int]]) -> list[int]:
    """Return a circle number for each function, shared by the functions of one circle.

    `named` is what `go_named_functions` returns. The numbers are those of the strongly connected
    components (the largest groups in which each function reaches every other by naming) that
    Tarjan's algorithm finds.
    """
    circle = [-1] * len(named)
    order = [-1] * len(named)
    lowest = [0] * len(named)
    stack: list[int] = []
    counter = [0, 0]

    def visit(position: int) -> None:
        order[position] = lowest[position] = counter[0]
        counter[0] += 1
        stack.append(position)
        for target in named[position]:
            if order[target] == -1:
                visit(target)
                lowest[position] = min(lowest[position], lowest[target])
            elif circle[target] == -1:
                lowest[position] = min(lowest[position], order[target])
        if lowest[position] == order[position]:
            while True:
                member = stack.pop()
                circle[member] = counter[1]
                if member == position:
                    break
            counter[1] += 1

    for position in range(len(named)):
        if order[position] == -1:
            visit(position)
    return circle


def go_is_ordered_file(path: pathlib.Path) -> bool:
    """Return True for a Go file of `cmd/` or `internal/`, where helper-before-caller holds."""
    parts = pathlib.PurePath(go_relative(path)).parts
    return bool(parts) and parts[0] in GO_TESTED_DIRECTORIES and "vendor" not in parts


def go_order_findings(path: pathlib.Path, tokens: list[GoToken]) -> Iterator[Finding]:
    """Yield a finding for each function or method defined below a function that names it.

    The finding is on the function's `func` line and names the first function above it that
    names it. Functions of one circle (see `go_circles`) keep any order among themselves.
    """
    if not go_is_ordered_file(path):
        return
    code = go_code_tokens(tokens)
    functions = go_functions(code)
    named = go_named_functions(code, functions)
    circle = go_circles(named)
    first_caller: dict[int, int] = {}
    for caller, targets in enumerate(named):
        for target in targets:
            if target > caller and circle[target] != circle[caller]:
                first_caller.setdefault(target, caller)
    for target, caller in sorted(first_caller.items()):
        function = functions[target]
        kind = "method" if function.has_receiver else "function"
        message = GO_ORDER_MESSAGE % (kind, function.name, functions[caller].name)
        yield code[function.start].line, message


def go_relative(path: pathlib.Path) -> str:
    """Return `path` as a `/`-separated path from the repository root when it is inside it."""
    return pathlib.PurePath(display(path)).as_posix()


def go_name_stem(file_name: str) -> str:
    """Return a Go file's name without `.go` or `_test.go` and one trailing build suffix."""
    stem = file_name.removesuffix(".go").removesuffix("_test")
    for suffix in GO_BUILD_SUFFIXES:
        if stem.endswith("_" + suffix):
            return stem.removesuffix("_" + suffix)
    return stem


def go_test_file_name_findings(
    path: pathlib.Path, root: pathlib.Path | None = None
) -> Iterator[Finding]:
    """Yield a finding for a test file of `root` not named for a Go file of its package.

    Only a file under `cmd/` or `internal/` is checked. Its stem must equal the stem of a
    non-test Go file of its directory, other than `doc.go`, or start with that stem and `_`. A
    directory with no such file is exempt, and so is anything under `vendor/` or a `testdata`
    directory. `root` is the repository unless given.
    """
    try:
        parts = path.resolve().relative_to((root or ROOT).resolve()).parts
    except ValueError:
        return
    if not path.name.endswith("_test.go") or "testdata" in parts or "vendor" in parts:
        return
    if not parts or parts[0] not in GO_TESTED_DIRECTORIES:
        return
    stems = {
        go_name_stem(sibling.name)
        for sibling in path.parent.glob("*.go")
        if sibling.is_file() and not sibling.name.endswith("_test.go") and sibling.name != "doc.go"
    }
    test_stem = go_name_stem(path.name)
    if stems and not any(test_stem == stem or test_stem.startswith(stem + "_") for stem in stems):
        yield 1, GO_TEST_FILE_NAME_MESSAGE


def check_go(path: pathlib.Path, source: str) -> Iterator[Finding]:
    """Yield the findings of one Go file."""
    tokens = go_tokens(source)
    is_test = path.name.endswith("_test.go")
    for binding in go_bindings(tokens):
        message = go_name_message(binding, is_test)
        if message:
            yield binding.line, message
    yield from go_comment_findings(tokens, path.name)
    yield from go_doc_file_findings(tokens, path.name)
    yield from go_format_findings(tokens)
    yield from go_dropped_cause_findings(tokens)
    yield from go_function_comment_findings(tokens, is_test)
    yield from go_function_length_findings(path, tokens)
    yield from go_test_file_name_findings(path)
    yield from go_order_findings(path, tokens)


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
