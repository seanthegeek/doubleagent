"""Read extension global state out of a VS Code family `state.vscdb`.

`<editor User dir>/globalStorage/state.vscdb` is SQLite with one table,
`ItemTable(key TEXT, value BLOB)`. An extension's whole `globalState` is one
row whose `key` is the extension id (`sourcegraph.cody-ai`,
`rjmacarthy.twinny`, `pearai.pearai-roo-cline`) and whose `value` is UTF-8
JSON text; see microsoft/vscode at d7622a529314,
`src/vs/platform/extensionManagement/common/extensionStorage.ts:159-176`.
SecretStorage rows have keys starting `secret://` and hold encrypted
credentials; they are never read here, whatever key a caller asks for.

The database is opened through `sqlite_util.open_copy`, so the `-wal`
sidecar is applied and the evidence copy is never opened. A file that
cannot be read, or whose sidecar cannot, a file that is not SQLite, a
damaged SQLite file and a value that is not JSON are each reported in the
returned problem list.
"""

from __future__ import annotations

import json
import re
import sqlite3
from collections.abc import Iterable
from pathlib import Path

from . import sqlite_util

STATE_DATABASE_PATTERN = re.compile(r"(?:^|/)User/globalStorage/state\.vscdb$")
# Catalog agents whose `User/` directory holds a VS Code family state.vscdb.
# An extension installed in a fork writes its global state to the fork's
# database, which the catalog attributes to the fork, so parsers for such
# extensions list all of these in `reads_agents`.
EDITOR_AGENTS = ("vscode", "cursor", "windsurf", "pearai", "kiro", "antigravity")
SECRET_PREFIX = "secret://"
VALUE_PROBLEM = "ItemTable value for "


def _text(value) -> str:
    if isinstance(value, memoryview):
        value = value.tobytes()
    if isinstance(value, (bytes, bytearray)):
        return bytes(value).decode("utf-8", errors="replace")
    return "" if value is None else str(value)


def read_items(path: Path, keys: Iterable[str]) -> tuple[dict[str, object], list[str]]:
    """Return the decoded JSON values of the requested `ItemTable` keys.

    A value that is not JSON is returned as its text and noted in the
    problems, and so is a database `sqlite_util.sqlite_problem` rejects,
    under its own file name. An absent file gives nothing. `secret://` keys
    are never read.

    Returns:
        The values of the keys present and the problems.
    """
    wanted = [key for key in keys if isinstance(key, str) and not key.startswith(SECRET_PREFIX)]
    path = Path(path)
    out: dict[str, object] = {}
    problems: list[str] = []
    if not wanted or path.is_symlink() or not path.is_file():
        return out, problems
    problem = sqlite_util.sqlite_problem(path)
    if problem == sqlite_util.NOT_SQLITE:
        return out, ["%s is not a SQLite database" % path.name]
    if problem:
        return out, [problem]
    try:
        with sqlite_util.open_copy(path) as connection:
            if "ItemTable" not in sqlite_util.table_names(connection):
                return out, problems
            sql = "select key, value from ItemTable where key in (%s)" % ",".join("?" * len(wanted))
            rows = [(row[0], row[1]) for row in connection.execute(sql, wanted)]
    except (sqlite3.DatabaseError, OSError) as error:
        return out, ["state.vscdb unreadable: %s" % error]
    for key, value in rows:
        if not isinstance(key, str) or key.startswith(SECRET_PREFIX):
            continue
        text = _text(value)
        try:
            out[key] = json.loads(text)
        except ValueError as error:
            out[key] = text
            problems.append("%s%s is not valid JSON (%s)" % (VALUE_PROBLEM, key, error))
    return out, problems


def shown_problem(problem: str) -> str:
    """Return the problem a row shows, without a JSON decoder detail that may quote the value."""
    if problem.startswith(VALUE_PROBLEM):
        return problem.split(" (", 1)[0]
    return problem


def read_item(path: Path, key: str) -> tuple[object, list[str]]:
    """Return the decoded JSON value of one `ItemTable` key, or None, and the problems."""
    values, problems = read_items(path, [key])
    return values.get(key), problems


__all__ = [
    "SECRET_PREFIX",
    "STATE_DATABASE_PATTERN",
    "VALUE_PROBLEM",
    "read_item",
    "read_items",
    "shown_problem",
]
