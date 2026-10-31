"""Split text into lines and trim it as the formats the analyzer reads specify.

A reader of a configuration or log format ends a line at LF or CRLF only
and trims only ASCII whitespace, the project's rule for a specified
format. `str.splitlines()` also splits at U+000B, U+000C, U+001C to
U+001E, U+0085, U+2028 and U+2029, and `str.strip()` with no argument also
takes U+00A0, U+2028 and the other Unicode spaces, so either would cut or
shorten a value that holds one. A lone CR stays inside its line.
"""

from __future__ import annotations

# The whitespace a line-level trim takes once the terminator is gone.
ASCII_SPACE = " \t\r\f\v"

# Python's ASCII whitespace, the padding a value such as a timestamp may carry.
ASCII_WHITESPACE = " \t\n\r\f\v"

# The whitespace JSON allows between tokens.
JSON_SPACE = " \t\n\r"

# The whitespace of TOML and of YAML: space and tab only.
SPACE_OR_TAB = " \t"


def split_lines(text: str) -> list[str]:
    """Split text at LF or CRLF only, without the terminators.

    A terminating LF gives no empty final line; a byte-order mark is kept.
    """
    lines = text.split("\n")
    if lines and lines[-1] == "":
        lines.pop()
    return [line[:-1] if line.endswith("\r") else line for line in lines]


def without_terminator(line: str) -> str:
    """Return a line read from a file less its LF or CRLF terminator, if it has one."""
    if not line.endswith("\n"):
        return line
    return line[:-2] if line.endswith("\r\n") else line[:-1]
