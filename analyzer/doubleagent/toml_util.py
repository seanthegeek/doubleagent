"""Read TOML documents with `tomllib`, or with stand-ins on Python 3.10.

Reads the text of a TOML document a parser hands it (a settings, policy or automation file)
and returns the dict `tomllib.loads` gives. `tomllib` is in the standard library from Python
3.11; on 3.10 two readers of a single-line subset stand in: `flat_toml` reads the top-level
`key = value` lines, and `scan_toml` the single-line entries of every table.

A parser calls `load_toml` for a document whose top level it needs and `load_config_toml` for
one whose tables it needs; each is `tomllib.loads` on 3.11 and later. A parser that must know
the stand-in is in use (the top level only, on 3.10) compares `load_toml` with `flat_toml`.
"""

from __future__ import annotations

import re
import sys

from .textlines import SPACE_OR_TAB, split_lines

TOML_KEY_PATTERN = re.compile(r"([A-Za-z0-9_-]+)[ \t]*=[ \t]*(.*)")
TOML_BASIC_PATTERN = re.compile(r'"((?:[^"\\]|\\.)*)"')
# One escape of a basic string, the line-ending backslash with the spaces,
# tabs and LF or CRLF line ends it removes included, so a backslash that an
# earlier backslash escapes is never read as one.
TOML_ESCAPE_PATTERN = re.compile(
    r"\\(u[0-9a-fA-F]{4}|U[0-9a-fA-F]{8}|[ \t]*(?:\r?\n[ \t]*)+|.)", re.S
)
TOML_INTEGER_PATTERN = re.compile(r"[+-]?(?:0|[1-9](?:_?[0-9])*)")
TOML_ESCAPES = {"b": "\b", "t": "\t", "n": "\n", "f": "\f", "r": "\r", '"': '"', "\\": "\\"}


def _toml_unescape(body: str) -> str:
    """The escapes of a TOML basic string; a line-ending backslash joins lines."""

    def one(match: re.Match) -> str:
        escape = match.group(1)
        if escape[0] in "uU" and len(escape) > 1:
            return chr(int(escape[1:], 16))
        if "\n" in escape:
            return ""
        if escape not in TOML_ESCAPES:
            raise ValueError("bad escape \\%s" % escape)
        return TOML_ESCAPES[escape]

    return TOML_ESCAPE_PATTERN.sub(one, body)


def _toml_value(raw: str) -> object:
    """Return one single-line TOML value of the kinds `automation.toml` holds.

    Raises:
        ValueError: the value is not a basic or literal string, an integer or a boolean.
    """
    if raw.startswith('"'):
        match = TOML_BASIC_PATTERN.match(raw)
        if not match:
            raise ValueError("unterminated string")
        return _toml_unescape(match.group(1))
    if raw.startswith("'"):
        end = raw.find("'", 1)
        if end < 0:
            raise ValueError("unterminated string")
        return raw[1:end]
    word = raw.split("#", 1)[0].strip(SPACE_OR_TAB)
    if word in ("true", "false"):
        return word == "true"
    # int() alone would also take Unicode whitespace, non-ASCII digits and leading zeros.
    if not TOML_INTEGER_PATTERN.fullmatch(word):
        raise ValueError("not a TOML value: %r" % word)
    return int(word.replace("_", ""))


def flat_toml(text: str) -> dict:
    """Read the top-level `key = value` lines of a TOML document, for Python 3.10.

    Multi-line basic and literal strings are joined; the first table header ends the top level.

    Raises:
        ValueError: a line cannot be read.
    """
    out: dict = {}
    lines = iter(split_lines(text))
    for line in lines:
        stripped = line.strip(SPACE_OR_TAB)
        if not stripped or stripped.startswith("#"):
            continue
        if stripped.startswith("["):
            break
        match = TOML_KEY_PATTERN.fullmatch(line.lstrip(SPACE_OR_TAB))
        if not match:
            raise ValueError("not a key = value line")
        key, raw = match.group(1), match.group(2)
        quote = raw[:3]
        if quote not in ('"""', "'''"):
            out[key] = _toml_value(raw.rstrip(SPACE_OR_TAB))
            continue
        # The whitespace before the first line's end is the string's.
        body = raw[3:]
        while quote not in body:
            more = next(lines, None)
            if more is None:
                raise ValueError("unterminated multi-line string")
            body += "\n" + more
        body = body[: body.index(quote)]
        body = body[1:] if body.startswith("\n") else body
        out[key] = body if quote == "'''" else _toml_unescape(body)
    return out


TOML_PART_PATTERN = re.compile(r"""[ \t]*("(?:[^"\\]|\\.)*"|'[^']*'|[A-Za-z0-9_-]+)[ \t]*""")


def _toml_dotted(text: str) -> list[str] | None:
    """Return the parts of a TOML dotted key, or None when the text is not one.

    Quotes are removed and basic-string escapes applied: `projects."/srv/proj"` is
    `["projects", "/srv/proj"]`.
    """
    parts: list[str] = []
    position = 0
    while True:
        match = TOML_PART_PATTERN.match(text, position)
        if not match:
            return None
        part = match.group(1)
        if part.startswith('"'):
            part = _toml_unescape(part[1:-1])
        elif part.startswith("'"):
            part = part[1:-1]
        parts.append(part)
        position = match.end()
        if position == len(text):
            return parts
        if text[position] != ".":
            return None
        position += 1


def scan_toml(text: str) -> dict:
    """Return the single-line `key = value` entries of every table of a TOML document, for 3.10.

    The entries are nested dicts. A value `_toml_value` cannot read (an array, an inline table, a
    date), an `[[array]]` table and a multi-line string are left out, so the scan never raises.
    """
    out: dict = {}
    table = out
    lines = iter(split_lines(text))
    for line in lines:
        stripped = line.strip(SPACE_OR_TAB)
        if not stripped or stripped.startswith("#"):
            continue
        if stripped.startswith("["):
            close = stripped.find("]")
            parts = None if stripped.startswith("[[") else _toml_dotted(stripped[1:close])
            table = {}
            if parts and close > 0:
                node = out
                for part in parts:
                    child = node.setdefault(part, {})
                    node = child if isinstance(child, dict) else {}
                table = node
            continue
        key, separator, raw = stripped.partition("=")
        raw = raw.strip(SPACE_OR_TAB)
        quote = raw[:3]
        if quote in ('"""', "'''"):
            body = raw[3:]
            while quote not in body:
                more = next(lines, None)
                if more is None:
                    break
                body += "\n" + more
            continue
        parts = _toml_dotted(key.strip(SPACE_OR_TAB)) if separator else None
        if not parts:
            continue
        try:
            value = _toml_value(raw)
        except ValueError:
            continue
        node = table
        for part in parts[:-1]:
            child = node.setdefault(part, {})
            node = child if isinstance(child, dict) else {}
        node[parts[-1]] = value
    return out


if sys.version_info >= (3, 11):
    import tomllib

    def load_toml(text: str) -> dict:
        """Read a TOML document with `tomllib`."""
        return tomllib.loads(text)

    def load_config_toml(text: str) -> dict:
        """Read a `config.toml` document with `tomllib`."""
        return tomllib.loads(text)

else:
    load_toml = flat_toml
    load_config_toml = scan_toml
