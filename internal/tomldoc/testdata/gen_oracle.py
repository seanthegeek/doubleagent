"""Write oracle.txt: TOML documents and what the Python analyzer makes of them.

Run from the repository root with the analyzer's interpreter:

    analyzer/.venv/bin/python -I internal/tomldoc/testdata/gen_oracle.py

Each line is `label<TAB>hex of the document's UTF-8<TAB>structure<TAB>settings`.
`structure` is `ERR` when tomllib rejects the document, else a rendering of the
dict in key order (see `render`). `settings` is, for each top-level key, the
hex of `setting_value(value)`, joined by `;`, or `-` when Python would write the
repr of a container holding a date or time.
"""

import datetime
import struct
import sys
import tomllib
from pathlib import Path

sys.path.insert(0, str(Path("analyzer").resolve()))
sys.path.insert(0, str(Path("analyzer/tests").resolve()))

from doubleagent.parsers.base import setting_value
from fixtures import CODEX_AUTOMATION_TOML, CODEX_CONFIG_TOML, GEMINI_POLICY_TOML
from fixtures.gemini_cli import GEMINI_SANDBOX_TOML
from fixtures.tabby import TABBY_CONFIG


def hexed(text):
    """Return the hex of a string's UTF-8."""
    return text.encode("utf-8").hex()


def render(value):
    """Return the text form of a decoded value."""
    if isinstance(value, dict):
        return "{" + ";".join(f"{hexed(key)}={render(item)}" for key, item in value.items()) + "}"
    if isinstance(value, list):
        return "[" + ",".join(render(item) for item in value) + "]"
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, int):
        return str(value)
    if isinstance(value, float):
        if value != value:
            return "fnan"
        return "f" + struct.pack(">d", value).hex()
    if isinstance(value, datetime.datetime):
        kind = "dt" if value.tzinfo else "ndt"
        return f"{kind}:{hexed(str(value))}"
    if isinstance(value, datetime.date):
        return f"date:{hexed(str(value))}"
    if isinstance(value, datetime.time):
        return f"time:{hexed(str(value))}"
    return "s" + hexed(value)


def holds_temporal(value):
    """Return True when a container holds a date or time."""
    if isinstance(value, dict):
        return any(holds_temporal(item) for item in value.values())
    if isinstance(value, list):
        return any(holds_temporal(item) for item in value)
    return isinstance(value, (datetime.date, datetime.time))


def settings(document):
    """Return the settings column for a document."""
    parts = []
    for value in document.values():
        if isinstance(value, (dict, list)) and holds_temporal(value):
            return "-"
        parts.append(hexed(setting_value(value)))
    return ";".join(parts)


CASES = {
    "an empty document": "",
    "only a comment": "# nothing here\n",
    "scalars in order": 'b = 1\na = "x"\nc = true\nd = 1.5\n',
    "a string with escapes": 'a = "tab\\there \\u00e9 \\U0001F600 \\"q\\" \\\\"\n',
    "a literal string": "a = 'C:\\path'\n",
    "a multi-line string": 'a = """\nline one\nline two"""\n',
    "a multi-line literal string": "a = '''\nraw \\n text\n'''\n",
    "a line-ending backslash": 'a = """one \\\n    two"""\n',
    "hex octal and binary integers": "a = 0xDEADbeef\nb = 0o755\nc = 0b1010\n",
    "integers with underscores and signs": "a = 1_000\nb = -5\nc = +7\nd = 0\n",
    "the largest int64": "a = 9223372036854775807\nb = -9223372036854775808\n",
    "floats": "a = 3.14\nb = -0.0\nc = 1e10\nd = 6.02e+23\ne = 1E-7\nf = 1_0.5\n",
    "floats at the exponent switch": "a = 1e15\nb = 1e16\nc = 0.0001\nd = 0.00001\ne = 123456789012345678.0\n",
    "special floats": "a = inf\nb = -inf\nc = nan\nd = +inf\n",
    "booleans": "a = true\nb = false\n",
    "offset datetimes": "a = 1979-05-27T07:32:00Z\nb = 1979-05-27 07:32:00+05:30\nc = 1979-05-27T00:32:00.999999-07:00\n",
    "a datetime with a long fraction": "a = 1979-05-27T07:32:00.123456789Z\nb = 1979-05-27T07:32:00.5Z\n",
    "local datetime date and time": "a = 1979-05-27T07:32:00\nb = 1979-05-27\nc = 07:32:00\nd = 07:32:00.5\ne = 1979-05-27 07:32:00.000001\n",
    "arrays": 'a = [1, 2, 3]\nb = []\nc = ["x", \'y\']\nd = [[1, 2], ["a"]]\ne = [1, 2,]\n',
    "an array of mixed types": 'a = [1, "two", 3.0, true]\n',
    "an array across lines with comments": "a = [\n  1, # one\n  2,\n]\n",
    "a table": "[t]\nx = 1\ny = 2\n",
    "tables in file order": "[zeta]\nk = 1\n[alpha]\nk = 2\n[mid]\nk = 3\n",
    "an implicit table created before its parent": "[a.b]\nx = 1\n[a]\ny = 2\n[c]\nz = 3\n",
    "dotted keys": "a.b.c = 1\na.b.d = 2\ne.f = 3\na.g = 4\n",
    "dotted keys inside a table": "[t]\nx.y = 1\nz = 2\nx.w = 3\n",
    "quoted keys": '"a b" = 1\n"é" = 2\n"" = 3\n\'lit.key\' = 4\n[x."y.z"]\nk = 5\n',
    "a quoted key table path": '[projects."/home/u/proj"]\ntrust_level = "trusted"\n[projects."/srv/other"]\ntrust_level = "untrusted"\n',
    "inline tables": "a = { x = 1, y = 2 }\nb = {}\nc = { z = { w = 1 } }\n",
    "an inline table with dotted keys": "a = { b.c = 1, b.d = 2, e = 3 }\n",
    "an array of tables": "[[r]]\nb = 1\na = 2\n[[r]]\nc = 3\n[[r]]\n",
    "an array of tables with sub tables": "[[a]]\nx = 1\n[a.sub]\ny = 2\n[[a]]\nx = 3\n[a.sub]\ny = 4\n[[a.list]]\nz = 5\n[[a.list]]\nz = 6\n",
    "nested arrays of tables": "[[a]]\n[[a.b]]\nk = 1\n[[a.b]]\nk = 2\n[[a]]\n[[a.b]]\nk = 3\n",
    "arrays of tables beside tables": "[x]\n1 = 1\n[[y]]\nk = 1\n[z]\n2 = 2\n[[y]]\nk = 2\n",
    "inline tables in an array": 'r = [{ decision = "allow", toolName = "x" }, { toolName = "y", decision = "deny", extra = 1 }]\n',
    "inline tables in an array of one key": "r = [{ b = 1 }, { a = 2 }]\n",
    "empty inline tables in an array": "r = [{}, {}]\n",
    "an array of mixed inline tables and scalars": "r = [1, { b = 1, a = 2 }, [3]]\n",
    "an inline table in an inline table": "r = { b = { d = 1, c = 2 }, a = 3 }\n",
    "a table in an array of tables with an inline array": "[[a]]\nlist = [{ z = 1, y = 2 }]\n[[a]]\nlist = [{ x = 3 }]\n",
    "an array with a nested inline array": "a = [[{ b = 1, a = 2 }]]\n",
    "a key after a table array element": "[[a]]\nx = 1\n[b]\ny = 2\n[[a]]\nx = 3\n",
    "a duplicate key": "a = 1\na = 2\n",
    "a duplicate table": "[a]\n[a]\n",
    "a table redefined as a value": "a = 1\n[a]\n",
    "a missing value": "a =\n",
    "an unclosed string": 'a = "never closed\n',
    "an unclosed multi-line string": 'a = """never closed\n',
    "a leading zero": "a = 0012\n",
    "a bad escape": 'a = "\\q"\n',
    "a bare word": "a = abc\n",
    "a control character in a string": 'a = "x\x01y"\n',
    "a key with no value": "a\n",
    "a trailing comma in an inline table": "a = { b = 1, }\n",
    "a newline in an inline table": "a = { b = 1,\n c = 2 }\n",
    "a bad date": "a = 2024-13-45\n",
    "a leading zero in a time": "a = 7:32:00\n",
    "a time without seconds": "a = 07:32\n",
    "a hex escape": 'a = "\\x41"\n',
    "an escape letter e": 'a = "\\e"\n',
    "a no-break space beside a key": "k\u00a0= 1\n",
    "a no-break space beside a value": "k = \u00a01\n",
    "a no-break space ending a multi-line line": 'p = """x\u00a0\nend"""\n',
    "a line separator in a string": 'prompt = "a\u2028b"\n',
    "windows line endings": 'id = "x"\r\nprompt = """a\r\nb"""\r\n[projects."/srv/proj"]\r\ntrust_level = "trusted"\r\n',
    "a UTF-8 byte order mark": "\ufeffa = 1\n",
    "a null character": "a = 1\x00\n",
    "an integer beyond int64": "a = 9223372036854775808\n",
    "an unpadded float with trailing dot": "a = 1.\n",
    "an underscore doubled": "a = 1__0\n",
    "a table header with spaces": "[ a . b ]\nk = 1\n",
    "a table header with a no-break space": "[a\u00a0.b]\nk = 1\n",
    "the fixture of a Codex automation": CODEX_AUTOMATION_TOML,
    "the fixture of a Codex config": CODEX_CONFIG_TOML,
    "the fixture of a Gemini policy": GEMINI_POLICY_TOML,
    "the fixture of a Gemini sandbox policy": GEMINI_SANDBOX_TOML,
    "the fixture of a Tabby config": TABBY_CONFIG,
}


def main():
    """Write oracle.txt beside this script."""
    lines = []
    for label, text in CASES.items():
        try:
            document = tomllib.loads(text)
        except tomllib.TOMLDecodeError:
            lines.append(f"{label}\t{hexed(text)}\tERR\t-")
            continue
        lines.append(f"{label}\t{hexed(text)}\t{render(document)}\t{settings(document)}")
    (Path(__file__).parent / "oracle.txt").write_text("\n".join(lines) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
