"""Dump every table of every fixture database, and the parsers' queries over them.

The parity-tagged Go test runs it with the analyzer's interpreter:

    python -I fixture_tables.py <analyzer directory> <output directory>

It builds the analyzer's full fixture home (`fixtures.build_home`) in
`<output directory>/home`, then writes `<output directory>/fixtures.txt`: a
`db <path under the home>` section per SQLite file, in the form generate.py
writes tables.txt (every table in rowid order, then each parser query the
database can run), or a section of one `error` line when SQLite cannot read
the database at all. Each database is read from its own scratch copy with
its sidecars, as the analyzer reads one.
"""

from __future__ import annotations

import importlib.util
import sqlite3
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
HEADER = b"SQLite format 3\x00"


def load_generator():
    """Load generate.py from this directory, which `-I` keeps off the import path."""
    spec = importlib.util.spec_from_file_location("generate", HERE / "generate.py")
    if spec is None or spec.loader is None:
        raise ImportError("generate.py not found beside fixture_tables.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def sqlite_files(home: Path) -> list[Path]:
    """The regular files under `home` that start with the SQLite header, in path order."""
    found = []
    for path in sorted(home.rglob("*")):
        if path.is_symlink() or not path.is_file():
            continue
        with open(path, "rb") as handle:
            if handle.read(16) == HEADER:
                found.append(path)
    return found


def main() -> None:
    """Build the fixture home and write fixtures.txt."""
    analyzer, output = Path(sys.argv[1]), Path(sys.argv[2])
    sys.path[:0] = [str(analyzer), str(analyzer / "tests")]
    import fixtures  # importable only once the path is set

    generate = load_generator()
    home = output / "home"
    fixtures.build_home(home)
    lines = ["# Written by fixture_tables.py from SQLite %s." % sqlite3.sqlite_version]
    for number, path in enumerate(sqlite_files(home)):
        scratch = output / "scratch" / str(number)
        scratch.mkdir(parents=True)
        name = path.relative_to(home).as_posix()
        try:
            section = generate.dump_queries(path, scratch)
        except sqlite3.Error as error:
            section = ["db " + path.name, "error " + str(error)]
        lines += ["db " + name, *section[1:]]
    (output / "fixtures.txt").write_text("\n".join(lines) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
