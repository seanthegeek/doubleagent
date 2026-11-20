"""Measure what SQLite reads from a record too short to hold a column with a DEFAULT.

Run it with the analyzer's interpreter, from any directory:

    analyzer/.venv/bin/python -I internal/sqlitefile/testdata/default_values.py

A record is shorter than its table when the row was written before an
ALTER TABLE ... ADD COLUMN, and SQLite then reads the column's DEFAULT. For
each declared type (one per affinity) and each group of DEFAULT clauses,
this script creates a table `t(id)` in a scratch database, writes one row,
then adds one column per DEFAULT clause with ALTER TABLE, which leaves the
row's record holding only `id`. It reads `typeof` and `quote` of every
added column from that row (`hex` for a text, since quote ends one at a
NUL, and `-0.0` for a negative zero, which quote writes without its sign).

It writes `default_values.jsonl.gz` beside itself: JSON lines, compressed
with gzip. The first line names the SQLite version and the declared types;
each other line is one clause, `{"default": <the text after DEFAULT>,
"results": [[<typeof>, <quote or hex>], ...]}`, with one reading per
declared type in that order, null where SQLite refused the column. The Go
tests compare the reader's Column.Default with that file. A path given as
the one argument is written instead, to compare another SQLite's readings.
"""

from __future__ import annotations

import gzip
import json
import sqlite3
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
DECLARED_TYPES = ["INTEGER", "REAL", "NUMERIC", "TEXT", ""]
CAST_TYPES = ["INTEGER", "REAL", "NUMERIC", "TEXT", "BLOB"]
# SQLite's default limit on the columns of a table is 2000.
COLUMNS_PER_TABLE = 1500
COMPACT = (",", ":")
# typeof and quote of a value, except a text, written as hex since quote
# ends one at its first NUL, and a negative zero, which quote writes 0.0.
READ = (
    "SELECT typeof({0}), CASE WHEN typeof({0}) = 'text' THEN hex({0})"
    " WHEN typeof({0}) = 'real' AND {0} = 0 AND atan2({0}, -1) < 0 THEN '-0.0'"
    " ELSE quote({0}) END FROM t"
)


def quoted(text: str) -> str:
    """Return `text` as an SQL string literal."""
    return "'" + text.replace("'", "''") + "'"


def integer_edges() -> list[int]:
    """Integers at both ends of the 64-bit range: the ends, and 1, 2 and 1024 or so past them."""
    edges = []
    for end in (9223372036854775807, -9223372036854775808):
        for step in (-1025, -1024, -1023, -2, -1, 0, 1, 2, 1023, 1024, 1025):
            edges.append(end + step)
    return edges


def number_literals() -> list[str]:
    """Numeric literals: integers, hexadecimal, reals with large and tiny exponents."""
    literals = list(dict.fromkeys(str(abs(number)) for number in integer_edges()))
    literals += ["0x0", "0x1", "0x7fffffff", "0x80000000", "0xffffffff", "0x7fffffffffffffff"]
    literals += ["0x8000000000000000", "0x8000000000000001", "0xffffffffffffffff", "0XAB"]
    literals += ["0", "00012", "1_000", "1e308", "1.7976931348623157e308", "1e309", "4.9e-324"]
    literals += ["2.2250738585072014e-308", "1e-400", "0.0", "1.5", ".5", "5.", "9.3e18", "9.2e18"]
    literals += ["9.223372036854775807e18", "9.223372036854775808e18", "1e18", "1e19", "1E+2"]
    literals += ["1e-2", "1.0e0", "1E2", "1.9e1", "9223372036854775807.0", "9223372036854775806.0"]
    literals += ["9223372036854775805.0", "9223372036854774784.0", "9223372036854775296.0"]
    literals += ["9223372036854775808.0", "123456789012345678901234567890", "2147483647"]
    literals += [
        "1_0",
        "9.223372036854776e18",
        "2147483648",
        "4294967296",
        "1e999",
        "0x10000000000000000",
        "0x0000000000000000001",
    ]
    return literals


def text_values() -> list[str]:
    """Texts: numbers as text, spacing, signs, exponents, hexadecimal and words."""
    texts = []
    for number in integer_edges():
        texts += [str(number), "%d.0" % number, " %d " % number]
    texts += ["0x10", "0X1F", "0x", "1e", "1e+", "1e-", "1.", ".", ".5", "5.", "", " ", "NaN"]
    texts += ["nan", "Inf", "inf", "Infinity", "-Infinity", "+Inf", "1e999", "-1e999", "1e308"]
    texts += ["1e309", " -0", "-0", "0.0", "-0.0", "+0", "00012", "12abc", "\t12\n", "12\r"]
    texts += ["\x0b12", "\x0c12", "+12", "-12", "+-12", "--12", "1e5 ", "  1.5e3  ", "1e3"]
    texts += ["1.9e1", "9223372036854775807.5", "-9223372036854775808.5", "1.0e0", "1_000"]
    texts += ["123456789012345678901234567890", "-123456789012345678901234567890", "0.1e-400"]
    texts += ["1e-400", "4.9e-324", "2.2250738585072014e-308", "abc", "12 34", "1,000", "٣"]
    texts += ["12e", "-9.223372036854775808e18", "9.223372036854775808e18", "1e18", "1e19"]
    texts += ["-1e19", "0e0", "1E2", "1.5e", ".e1", "1.e1", " 1 2", "  -0012x", "TRUE", "9.3e18"]
    texts += ["-9.3e18", "9223372036854775806.0", "9223372036854775805.0", "1.5e3", "12"]
    texts += [" 9223372036854775808"]
    return texts


def base_values() -> list[str]:
    """Every literal measured, as SQL text: numbers, strings, blobs and keywords."""
    values = number_literals()
    values += [quoted(text) for text in text_values()]
    values += ["x'3132'", "x''", "x'2d39323233333732303336383534373735383038'", "x'312e35'"]
    values += ["x'20313220'", "x'00'", "x'31'", "x'616263'", "NULL", "TRUE", "FALSE"]
    return values


def expressions() -> list[str]:
    """The literals, each negated and with a plus sign, and the numbers negated twice."""
    values = base_values()
    found = list(values)
    found += ["-" + value for value in values if value not in ("NULL", "TRUE", "FALSE")]
    found += ["-(-%s)" % value for value in number_literals()]
    found += ["+" + value for value in values if value not in ("NULL", "TRUE", "FALSE")]
    found += ["- " + value for value in ("5", "'12'", "x'31'")]
    return found


def is_term(expression: str) -> bool:
    """Report whether the grammar takes `expression` after DEFAULT without parentheses."""
    return not expression.startswith("-(")


def null_tests() -> list[str]:
    """Tests for NULL of literals, signs, parentheses and other tests, each form of the test."""
    operands = ["1", "-1", "+1", "- 1", "-(1)", "(1)", "1.5", "0x10", "'a'", "''", "- 'a'", "x'00'"]
    operands += [
        "NULL",
        "TRUE",
        "CAST(1 AS TEXT)",
        "(1+1)",
        "CURRENT_TIME",
        "1 IS NULL",
        "(1 NOTNULL)",
    ]
    suffixes = [" IS NULL", " ISNULL", " NOTNULL", " NOT NULL", " IS NOT NULL"]
    found = ["(%s%s)" % (operand, suffix) for operand in operands for suffix in suffixes]
    found += ["(-(1 IS NULL))", "(-(1 NOTNULL))", "(- 1 NOTNULL)", "(CAST(1 IS NULL AS TEXT))"]
    found += [
        "(CAST(1 NOTNULL AS REAL))",
        "(1 ISNULL NOTNULL ISNULL)",
        "(1 IS NULL + 1)",
        "(+1 NOTNULL)",
    ]
    return found


def deep_nesting() -> list[str]:
    """Values nested deeper than SQLite 3.51 parses: parentheses, signs and CASTs."""
    found = []
    for depth in (150, 1000, 2400):
        found.append("(" * depth + "7" + ")" * depth)
        found.append("(" + "+ " * depth + "7)")
    for depth in (150, 998):
        found.append("(" + "- " * depth + "7)")
        found.append("(" + "CAST(" * depth + "7" + " AS INTEGER)" * depth + ")")
    return found


def clauses() -> list[str]:
    """Every DEFAULT clause measured, as the text after the keyword DEFAULT."""
    found = ["abc", '"abc"', "[abc]", "`abc`"]
    for expression in expressions():
        if is_term(expression):
            found.append(expression)
        found.append("(%s)" % expression)
        found += ["(CAST(%s AS %s))" % (expression, cast) for cast in CAST_TYPES]
    for value in ("'1e3'", "1.5", "-0x8000000000000000", "'12abc'", "x'312e35'", "'abc'"):
        found += [
            "(CAST(CAST(%s AS %s) AS %s))" % (value, inner, outer)
            for inner in CAST_TYPES
            for outer in CAST_TYPES
        ]
    found += null_tests() + deep_nesting()
    return list(dict.fromkeys(found))


def measure(database: Path, declared: str, group: list[str]) -> list[list[str] | None]:
    """Add one column per clause to a one-row table and read each back from that row."""
    connection = sqlite3.connect(database, isolation_level=None)
    connection.execute("CREATE TABLE t(id)")
    connection.execute("INSERT INTO t VALUES (1)")
    added = []
    for number, clause in enumerate(group):
        column = "c%d" % number
        try:
            connection.execute(
                "ALTER TABLE t ADD COLUMN %s %s DEFAULT %s" % (column, declared, clause)
            )
        except sqlite3.Error:
            added.append(None)
            continue
        added.append(column)
    results: list[list[str] | None] = []
    for column in added:
        if column is None:
            results.append(None)
            continue
        row = connection.execute(READ.format(column)).fetchone()
        results.append([row[0], row[1]])
    connection.close()
    database.unlink()
    return results


def main() -> None:
    """Measure every clause under every declared type and write the readings."""
    every = clauses()
    readings: dict[str, list[list[str] | None]] = {clause: [] for clause in every}
    with tempfile.TemporaryDirectory() as scratch:
        for declared in DECLARED_TYPES:
            for start in range(0, len(every), COLUMNS_PER_TABLE):
                group = every[start : start + COLUMNS_PER_TABLE]
                database = Path(scratch) / "short.db"
                results = measure(database, declared, group)
                for number, clause in enumerate(group):
                    readings[clause].append(results[number])
    lines = [
        json.dumps({"sqlite": sqlite3.sqlite_version, "types": DECLARED_TYPES}, separators=COMPACT)
    ]
    for clause in every:
        if any(readings[clause]):
            lines.append(
                json.dumps({"default": clause, "results": readings[clause]}, separators=COMPACT)
            )
    output = Path(sys.argv[1]) if len(sys.argv) > 1 else HERE / "default_values.jsonl.gz"
    text = "\n".join(lines) + "\n"
    output.write_bytes(gzip.compress(text.encode("utf-8"), mtime=0))


if __name__ == "__main__":
    main()
