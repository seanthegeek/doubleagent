#!/usr/bin/env python3
"""Check the analyzer test fixtures against the Parsing part of each agent's research document.

A fixture module is a file of `analyzer/tests/fixtures/` other than `__init__.py` and
`common.py`; its agent is `module_agent`'s. A name is documented when `is_documented` finds it
in a code span of the Parsing part or a fenced line outside Parser plan's Fixtures (a sample
never documents a key), of the file's agent's document or of an agent whose fixture module that
agent's modules import a name from (`lineage_agents`). Rules 3 to 6 and 8 read only the files
`build_home` leaves on disk, so a record a later builder overwrites is not checked.

1. `builder-provenance-missing`: a public builder (`is_public_builder`) whose docstring has no
   line `source_line` accepts, naming an agent document and one of its heading anchors.
2. `module-provenance-missing`: a fixture module whose docstring does not name
   `research/<agent>.md` for its own agent as a whole path token (`names_document`).
3. `key-undocumented`: a dict key, SQLite table or column name, or YAML or TOML key that is
   a field name (`is_field_name`) in a file of an agent and is not documented;
   `file-unclaimed`: a file no catalog entry claims (`claiming_agent`).
4. `enum-undocumented`: a value (`is_enum_value`) of a key in `ENUM_KEYS` not documented.
5. `type-mismatch`: a value whose `json_type` is not in its key's `Type` cell, or not among
   the code spans of its `Values` cell, in a key table of Record schema or Other stores.
6. `substitution-undeclared`: a project path (`project_path`) outside `/srv/proj` and the
   `declared_paths` of the agent's fixture modules.
7. `cross-agent-builder`: a call to a function imported from another agent's fixture module
   whose agent `lineage` does not list.
8. `sample-equals-builder`: a JSON value in a fenced block under Parser plan's Fixtures whose
   key-sorted form equals a record `build_home` wrote.

A key table's header row is `| Key | Type | Values | Since |` (or `Builds`): Key a code span,
Type `JSON_TYPES` names joined by ` or `, `,` or `/`, Values code spans (empty for any).

Usage: tools/check_fixture_schema.py [--update-baseline], with the analyzer's requirements.
"""

from __future__ import annotations

import ast
import dataclasses
import functools
import hashlib
import importlib.util
import json
import pathlib
import re
import sqlite3
import sys
import tempfile
import urllib.parse
from collections.abc import Iterator
from dataclasses import dataclass, field
from types import ModuleType
from typing import TYPE_CHECKING

import check_doc_links
import drift_common
import zstandard

if TYPE_CHECKING:
    from analyzer.doubleagent.catalog import Catalog

CHECKER = "check_fixture_schema"
FIXTURES = pathlib.PurePosixPath("analyzer/tests/fixtures")
SHARED_MODULES = frozenset({"__init__", "common"})
SUBSTITUTE_PATH = "/srv/proj"
FIXTURES_SECTION = ("Parsing", "Parser plan", "Fixtures")
ENUM_KEYS = frozenset({"type", "role", "kind", "status", "level", "source", "mode", "decision"})
JSON_TYPES = ("string", "number", "boolean", "object", "array", "null")
ZSTD_MAGIC = b"\x28\xb5\x2f\xfd"
SIDECAR_SUFFIXES = ("-wal", "-shm", "-journal")
# Key words naming a directory a session ran in; see `project_path`.
PROJECT_KEY_WORDS = ("cwd", "project", "workspace", "folder", "root", "repo", "dir")
SOURCE_LINE = re.compile(
    r"^Source: research/(?P<agent>[a-z0-9][a-z0-9._-]*)\.md#(?P<anchor>[^\s;]+); "
    r"(?:build|source) (?P<token>\S+)$"
)
# A character a path holds, so one beside `research/<agent>.md` makes it part of a longer path.
PATH_CHARACTER = r"[A-Za-z0-9_/\\-]"
RESEARCH_DOCUMENT = re.compile(r"research/([a-z0-9][a-z0-9._-]*)\.md")
CODE_SPAN = re.compile(r"`([^`]+)`")
# A docstring line opening with a label of its own, such as `Source:` or `Lineage:`.
LABEL = re.compile(r"^[A-Z][A-Za-z]*:")
YAML_KEY = re.compile(
    r"""^\s*(?:-\s+)?"""
    r"""(?:"([^"]+)"|'([^']+)'|([A-Za-z_$][\w.$-]*))\s*:(?:\s|$)"""
)
TOML_TABLE = re.compile(r"^\s*\[\[?\s*([^\[\]]+?)\s*\]\]?\s*(?:#.*)?$")
TOML_PART_FORM = r"""(?:"[^"]*"|'[^']*'|[A-Za-z0-9_-]+)"""
TOML_KEY = re.compile(r"^\s*(%s(?:\s*\.\s*%s)*)\s*=" % (TOML_PART_FORM, TOML_PART_FORM))
TOML_PART = re.compile(r""""([^"]*)"|'([^']*)'|([A-Za-z0-9_-]+)""")
SPAN_TOKEN = re.compile(r"[A-Za-z0-9_$@.:/-]+")
FIELD_NAME = re.compile(r"^[A-Za-z_$@][A-Za-z0-9_$@.:-]{0,63}$")
HEX_ID = re.compile(r"^[0-9a-fA-F]{8,}(?:-[0-9a-fA-F]{4,})*$")
ENVIRONMENT = re.compile(r"^[A-Z][A-Z0-9_]{2,}$")
DATA_KEY_PREFIXES = ("toolu_", "msg_", "call_")
# Keys whose value maps names the user or the agent chose (servers, variables, sessions) to data.
MAP_CONTAINERS = frozenset(
    {
        *("env", "environment", "headers", "mcpServers", "mcp_servers", "servers", "models"),
        *("providers", "profiles", "agents", "channels", "plugins", "extensions", "skills"),
        *("tools", "tasks", "sessions", "projects", "workspaces", "worktrees", "sidecars"),
    }
)
ENUM_VALUE = re.compile(r"^[A-Za-z][A-Za-z0-9_.:-]{0,63}$")
KEY_TABLE_HEADER = re.compile(r"^\|\s*Key\s*\|\s*Type\s*\|\s*Values\s*\|\s*(?:Since|Builds)\s*\|")
TYPE_SEPARATOR = re.compile(r"\s+or\s+|[,/]")
WINDOWS_PATH = re.compile(r"^[A-Za-z]:[\\/]")


# --- Heuristics -----------------------------------------------------------------------------


def is_public_builder(name: str) -> bool:
    """Return whether a function is a public builder: no leading `_`, `*records` or `write_*`."""
    return not name.startswith("_") and (name.endswith("records") or name.startswith("write_"))


def source_line(line: str) -> tuple[str, str, str] | None:
    """Return (agent, anchor, version or hash) of a docstring line in the `Source:` form, or None.

    The form is `Source: research/<agent>.md#<anchor>; build <version>` or
    `...; source <short hash>`, the whole line after its indentation.
    """
    match = SOURCE_LINE.match(line.strip())
    return (match["agent"], match["anchor"], match["token"]) if match else None


def module_agent(stem: str, docstring: str, documents: set[str]) -> str:
    """Return a fixture module's agent.

    The first `research/<agent>.md` its docstring names that is an agent document, else the
    stem with `_` as `-` when that is a document, else that less its last `-` segment when that
    is one (`continue_dev` gives `continue`), else the stem with `_` as `-`.
    """
    for name in RESEARCH_DOCUMENT.findall(docstring):
        if name in documents:
            return name
    hyphenated = stem.replace("_", "-")
    shorter = hyphenated.rsplit("-", 1)[0]
    if hyphenated not in documents and shorter in documents:
        return shorter
    return hyphenated


def names_document(docstring: str, agent: str) -> bool:
    """Return whether a docstring names `research/<agent>.md` as a whole path token.

    The token is bounded by characters no path holds (`PATH_CHARACTER`): no `.` before it, and
    one after it only when no path character follows, so `research/foo-bar.md` and
    `old-research/bar.md` do not name `bar`'s document, while `(research/bar.md).` does.
    """
    token = re.escape("research/%s.md" % agent)
    path = PATH_CHARACTER
    bounded = r"(?<!%s)(?<!\.)%s(?!%s|\.%s)" % (path, token, path, path)
    return re.search(bounded, docstring) is not None


def declared_paths(docstring: str) -> list[str]:
    """Return the project paths a docstring declares: each code span of a `Paths:` line.

    A `Paths:` line continues on each following line indented at least as far as its text
    after `Paths:`, up to a blank line or a line opening with another label (`Source:`).
    """
    found: list[str] = []
    column = -1
    for line in docstring.split("\n"):
        text = line.lstrip(" \t")
        indent = len(line) - len(text)
        if text.startswith("Paths:"):
            after = text[len("Paths:") :]
            column = indent + len("Paths:") + len(after) - len(after.lstrip(" \t"))
        elif column < 0 or not text or LABEL.match(text) or indent < column:
            column = -1
            continue
        found.extend(CODE_SPAN.findall(line))
    return found


def lineage(docstring: str) -> set[str]:
    """Return the agents a line `Lineage: <agent>[, <agent>...]` of a docstring names."""
    agents: set[str] = set()
    for line in docstring.split("\n"):
        text = line.strip()
        if text.startswith("Lineage:"):
            agents.update(name for name in re.split(r"[\s,]+", text[8:]) if name)
    return agents


def yaml_keys(line: str) -> list[str]:
    """Return the key a YAML line maps: `key:`, `key: value`, `- key: value` or a quoted key.

    A comment, a key with no space after its colon (`http://x`) and a list item without a key
    give none.
    """
    match = YAML_KEY.match(line)
    if not match or line.lstrip().startswith("#"):
        return []
    return [next(group for group in match.groups() if group is not None)]


def toml_keys(line: str) -> list[str]:
    """Return the key parts a TOML line names: `key = v`, `a.b = v`, `[table]`, `[[a.b]]`.

    Each dotted part is a key; quoted parts lose their quotes. A comment or `= value` gives none.
    """
    if line.lstrip().startswith("#"):
        return []
    match = TOML_TABLE.match(line) or TOML_KEY.match(line)
    if not match:
        return []
    parts = TOML_PART.finditer(match[1])
    return [next(group for group in part.groups() if group is not None) for part in parts]


def json_type(value: object) -> str:
    """Return the JSON type name of a decoded value: `boolean` before `number`; other is ""."""
    if value is None:
        return "null"
    for kind, python in (
        ("boolean", bool),
        ("number", (int, float)),
        ("string", str),
        ("object", dict),
        ("array", list),
    ):
        if isinstance(value, python):
            return kind
    return ""


def table_types(cell: str) -> set[str]:
    """Return the JSON type names a key table's Type cell gives; other words are ignored."""
    words = (word.strip(" `").lower() for word in TYPE_SEPARATOR.split(cell))
    return {word for word in words if word in JSON_TYPES}


def is_enum_value(value: object) -> bool:
    """Return whether a value can be an enum token: a string, a letter then word characters.

    Accepted: `user`, `tool_use`, `allow-once`, `mcp.call`, `a:b`, up to 64 characters.
    Rejected: text with spaces, paths, URLs, numbers, ids beginning with a digit, "".
    """
    return isinstance(value, str) and bool(ENUM_VALUE.match(value))


def project_path(key: str, value: object) -> str:
    r"""Return the path a value names when its key names a project directory, else "".

    The key holds one of `PROJECT_KEY_WORDS`, case-folded (`cwd`, `workspaceRoots`,
    `project_path`); the value is an absolute POSIX path, a drive path (`C:\x`, `C:/x`) or a
    `file://` URI, given back decoded (`file:///C:/x` as `C:/x`). A relative path is none.
    """
    folded = key.lower()
    if not isinstance(value, str) or not any(word in folded for word in PROJECT_KEY_WORDS):
        return ""
    if value.startswith("file://"):
        value = urllib.parse.unquote(urllib.parse.urlsplit(value).path)
        value = value[1:] if WINDOWS_PATH.match(value[1:]) else value
    if WINDOWS_PATH.match(value) or (value.startswith("/") and value[1:2] not in ("", "/")):
        return value
    return ""


def is_substitute(path: str, allowed: list[str]) -> bool:
    r"""Return whether a path is one of `allowed` or below one, `/` and `\` alike."""
    normal = path.replace("\\", "/").rstrip("/")
    for base in allowed:
        root = base.replace("\\", "/").rstrip("/")
        if normal == root or normal.startswith(root + "/"):
            return True
    return False


def span_names(spans: list[str]) -> set[str]:
    """Return every name the code spans give: the span, its tokens and their dotted parts.

    A token is a run of letters, digits and `_$@.:/-` (`message.content[]` gives
    `message.content`, `message` and `content`); a quoted name in a span (`"a b"`) is one too.
    """
    names: set[str] = set()
    for span in spans:
        names.add(span)
        names.update(re.findall(r'"([^"]+)"', span))
        for token in SPAN_TOKEN.findall(span):
            names.add(token)
            names.update(part for part in re.split(r"[.:/]", token) if part)
    return names


def is_field_name(key: str, container: str = "") -> bool:
    """Return whether a dict key or column is a field name, not data used as a map key.

    Accepted: a letter, `_`, `$` or `@`, then letters, digits and `_$@.-`, up to 64 characters
    (`sessionId`, `x-api-key`). Rejected: "", a key with a space, `/`, `=`, `,` or `:`, one
    beginning with a digit, a hex id of eight or more digits, alone or in dashed groups, an
    environment variable name (`GITHUB_TOKEN`), one opening `toolu_`, `msg_` or `call_`, and any
    key of a map a `MAP_CONTAINERS` key holds: env, environment, headers, mcpServers, mcp_servers,
    servers, models, providers, profiles, agents, channels, plugins, extensions, skills, tools,
    tasks, sessions, projects, workspaces, worktrees, sidecars.
    """
    if container in MAP_CONTAINERS or ":" in key or key.startswith(DATA_KEY_PREFIXES):
        return False
    return bool(FIELD_NAME.match(key)) and not HEX_ID.match(key) and not ENVIRONMENT.match(key)


def is_documented(name: str, names: set[str]) -> bool:
    """Return whether a key, table, column or value is among `span_names`' names."""
    return name in names


# --- Fixture modules ------------------------------------------------------------------------


@dataclass
class FixtureModule:
    """One fixture module: its file, stem, agent, docstring and syntax tree."""

    path: pathlib.Path
    stem: str
    agent: str
    docstring: str
    tree: ast.Module


def fixture_modules(root: pathlib.Path, documents: set[str]) -> list[FixtureModule]:
    """Return the fixture modules under `root` but `SHARED_MODULES`, sorted by file name."""
    modules: list[FixtureModule] = []
    for path in sorted((root / FIXTURES).glob("*.py")):
        if path.stem in SHARED_MODULES:
            continue
        tree = ast.parse(path.read_text(encoding="utf-8"), str(path))
        docstring = ast.get_docstring(tree) or ""
        agent = module_agent(path.stem, docstring, documents)
        modules.append(FixtureModule(path, path.stem, agent, docstring, tree))
    return modules


@functools.cache
def _anchors(path: pathlib.Path) -> frozenset[str]:
    """Return the heading anchors of a document."""
    return frozenset(check_doc_links.heading_anchors(path.read_text(encoding="utf-8")))


def provenance_findings(
    root: pathlib.Path, module: FixtureModule, documents: dict[str, pathlib.Path]
) -> Iterator[drift_common.Finding]:
    """Yield the module's `module-provenance-missing` and `builder-provenance-missing` findings."""
    relative = module.path.relative_to(root).as_posix()
    if not names_document(module.docstring, module.agent):
        yield drift_common.Finding(
            module.agent, "module-provenance-missing", module.stem, relative, 1
        )
    for node in module.tree.body:
        if not isinstance(node, ast.FunctionDef) or not is_public_builder(node.name):
            continue
        lines = (ast.get_docstring(node) or "").split("\n")
        sources = [found for found in map(source_line, lines) if found]
        if not any(
            agent in documents and anchor in _anchors(documents[agent])
            for agent, anchor, _ in sources
        ):
            yield drift_common.Finding(
                module.agent,
                "builder-provenance-missing",
                node.name,
                relative,
                node.lineno,
                "no Source: line naming a heading of an agent document",
            )


def lineage_findings(
    root: pathlib.Path, module: FixtureModule, agents: dict[str, str]
) -> Iterator[drift_common.Finding]:
    """Yield a `cross-agent-builder` finding per call into another agent's fixture module."""
    imported = imported_names(module, agents)
    allowed = lineage(module.docstring) | {module.agent}
    reported: set[str] = set()
    for node in ast.walk(module.tree):
        if not isinstance(node, ast.Call):
            continue
        name, owner = _callee(node.func, imported)
        if owner and agents[owner] not in allowed and name not in reported:
            reported.add(name)
            relative = module.path.relative_to(root).as_posix()
            message = "imported from %s.py; declare Lineage: %s" % (owner, agents[owner])
            yield drift_common.Finding(
                module.agent, "cross-agent-builder", name, relative, node.lineno, message
            )


def imported_names(module: FixtureModule, agents: dict[str, str]) -> dict[str, str]:
    """Return each name a module binds from another fixture module, mapped to that module's stem.

    `from .x import f` binds `f` to `x` and `from . import x` binds `x` to `x`, for a stem `x`
    of `agents`; other imports bind none.
    """
    imported: dict[str, str] = {}
    for node in module.tree.body:
        if isinstance(node, ast.ImportFrom) and node.level == 1 and node.module in agents:
            for alias in node.names:
                imported[alias.asname or alias.name] = node.module
        elif isinstance(node, ast.ImportFrom) and node.level == 1 and not node.module:
            for alias in node.names:
                if alias.name in agents:
                    imported[alias.asname or alias.name] = alias.name
    return imported


def lineage_agents(modules: list[FixtureModule]) -> dict[str, set[str]]:
    """Return, per agent, the agents whose fixture modules its own modules import a name from.

    The import relation is `imported_names`', the one rule 7 reads, whether or not a `Lineage:`
    line declares it; an agent never lists itself.
    """
    agents = {module.stem: module.agent for module in modules}
    found: dict[str, set[str]] = {}
    for module in modules:
        owners = {agents[stem] for stem in imported_names(module, agents).values()}
        found.setdefault(module.agent, set()).update(owners - {module.agent})
    return found


def _callee(function: ast.expr, imported: dict[str, str]) -> tuple[str, str]:
    """Return (function name, fixture module stem) of a call into an imported module, or blanks."""
    if isinstance(function, ast.Name) and function.id in imported:
        return function.id, imported[function.id]
    if (
        isinstance(function, ast.Attribute)
        and isinstance(function.value, ast.Name)
        and function.value.id in imported
        and imported[function.value.id] == function.value.id
    ):
        return function.attr, function.value.id
    return "", ""


# --- Built files ----------------------------------------------------------------------------


@dataclass
class BuiltFile:
    """A file `build_home` wrote: its path from the home, agent, names and decoded records.

    `names` holds the SQLite table and column names and the YAML and TOML keys; `records` each
    decoded JSON value (a JSON file, a JSONL line, a JSON text or zstd column of a row);
    `rows` each SQLite row as a dict of column name to value.
    """

    relative: str
    agent: str
    names: list[str] = field(default_factory=list)
    records: list[object] = field(default_factory=list)
    rows: list[dict[str, object]] = field(default_factory=list)


def load_fixtures(root: pathlib.Path) -> ModuleType:
    """Import the fixtures package under `root` by its path, with `root/analyzer` importable."""
    directory = root / FIXTURES
    analyzer = str(root / "analyzer")
    if analyzer not in sys.path:
        sys.path.append(analyzer)
    name = "drift_fixtures_%s" % hashlib.sha256(str(directory).encode()).hexdigest()[:12]
    if name in sys.modules:
        return sys.modules[name]
    spec = importlib.util.spec_from_file_location(
        name, directory / "__init__.py", submodule_search_locations=[str(directory)]
    )
    if spec is None or spec.loader is None:
        raise ImportError("cannot import the fixtures package at %s" % directory)
    package = importlib.util.module_from_spec(spec)
    sys.modules[name] = package
    try:
        spec.loader.exec_module(package)
    except BaseException:
        del sys.modules[name]
        raise
    return package


def sqlite_util_module() -> ModuleType:
    """Return `doubleagent.sqlite_util`, with `analyzer/` on the path as `drift_common` puts it."""
    drift_common.catalog_module()
    return importlib.import_module("doubleagent.sqlite_util")


def claiming_agent(relative: str, catalog: Catalog) -> str:
    """Return the agent of a home entry claiming a path, else of a project entry claiming a suffix.

    A suffix is the path less one or more leading segments (`dev/x/.claude/settings.json`
    tries `x/.claude/settings.json`, then `.claude/settings.json`); "" when nothing claims it.
    """
    owner = catalog.claims_home_path(relative)
    parts = relative.split("/")
    for start in range(1, len(parts)):
        if owner:
            break
        owner = catalog.project_agent_for("/".join(parts[start:]))
    return owner


def decode_text(text: str, whole: bool) -> list[object]:
    """Return the JSON values of a text: the whole text when it parses, else each line that does.

    With `whole` False only lines are tried, as for a `.jsonl` file.
    """
    if whole:
        try:
            return [json.loads(text)]
        except ValueError:
            pass
    values: list[object] = []
    for line in re.split(r"\r?\n", text):
        if line.strip()[:1] in ("{", "["):
            try:
                values.append(json.loads(line))
            except ValueError:
                continue
    return values


def decode_bytes(data: bytes, name: str) -> list[object]:
    """Return the JSON values of a file's bytes, decompressing zstd first."""
    if data.startswith(ZSTD_MAGIC):
        data = zstandard.ZstdDecompressor().decompressobj().decompress(data)
        name = name.removesuffix(".zst")
    text = data.decode("utf-8", errors="replace")
    if name.endswith((".jsonl", ".ndjson")):
        return decode_text(text, whole=False)
    return decode_text(text, whole=True) if text.lstrip()[:1] in ("{", "[") else []


def decode_column(value: object) -> list[object]:
    """Return the JSON value a column holds as JSON text or zstd-compressed JSON, else none."""
    if isinstance(value, bytes) and value.startswith(ZSTD_MAGIC):
        return decode_bytes(value, "")
    if isinstance(value, str) and value.lstrip()[:1] in ("{", "["):
        return decode_text(value, whole=True)[:1]
    return []


def read_sqlite(path: pathlib.Path, built: BuiltFile) -> None:
    """Add a database's table and column names, and each row's columns as a record.

    FTS shadow tables (a virtual table's name, `_` and a suffix) and `sqlite_` tables are
    skipped; a file SQLite cannot read (a fixture of a damaged database) adds nothing.
    """
    sqlite_util = sqlite_util_module()
    with sqlite_util.open_copy(path) as connection:
        try:
            master = connection.execute("select name, sql from sqlite_master where type='table'")
            virtual = [
                name for name, sql in master if (sql or "").upper().startswith("CREATE VIRTUAL")
            ]
            tables = sorted(sqlite_util.table_names(connection))
        except sqlite3.DatabaseError:
            return
        for table in tables:
            shadow = any(table.startswith(name + "_") for name in virtual)
            if not table.startswith("sqlite_") and not shadow:
                read_table(connection, table, built)


def read_table(connection: sqlite3.Connection, table: str, built: BuiltFile) -> None:
    """Add a table's name, its column names, and each row as a record beside its JSON columns."""
    built.names.append(table)
    try:
        cursor = connection.execute('select * from "%s"' % table.replace('"', '""'))
        columns = [column[0] for column in cursor.description]
        rows = [tuple(row) for row in cursor]
    except sqlite3.DatabaseError:
        return
    built.names.extend(columns)
    for row in rows:
        built.rows.append(dict(zip(columns, row, strict=True)))
        for value in row:
            built.records.extend(decode_column(value))


def read_built_file(path: pathlib.Path, built: BuiltFile) -> None:
    """Add a file's names and records by its form: SQLite, YAML, TOML, JSON, JSONL or zstd."""
    sqlite_util = sqlite_util_module()
    name = path.name
    if sqlite_util.is_sqlite(path):
        read_sqlite(path, built)
        return
    data = path.read_bytes()
    text = data.decode("utf-8", errors="replace")
    if name.endswith((".yaml", ".yml")):
        built.names.extend(key for line in text.split("\n") for key in yaml_keys(line))
    elif name.endswith(".toml"):
        built.names.extend(key for line in text.split("\n") for key in toml_keys(line))
    else:
        built.records.extend(decode_bytes(data, name))


def built_files(root: pathlib.Path, catalog: Catalog) -> list[BuiltFile]:
    """Build the fixture home under `root` in a temporary directory and read every file."""
    package = load_fixtures(root)
    found: list[BuiltFile] = []
    with tempfile.TemporaryDirectory(prefix="drift-fixtures-") as scratch:
        home = pathlib.Path(scratch) / "home"
        package.build_home(home)
        for path in sorted(home.rglob("*")):
            if path.is_symlink() or not path.is_file():
                continue
            relative = path.relative_to(home).as_posix()
            built = BuiltFile(relative, claiming_agent(relative, catalog))
            found.append(built)
            if built.agent and not relative.endswith(SIDECAR_SUFFIXES):
                read_built_file(path, built)
    return found


def walk(
    value: object, key: str = "", container: str = ""
) -> Iterator[tuple[str, object, bool, str]]:
    """Yield (key, value, in a list, container) for every dict item and list item under a key.

    The container is the key whose value is the dict holding the item: "" at the top and for a
    dict that is a list's item, whose keys are its own fields.
    """
    if isinstance(value, dict):
        for name, item in value.items():
            yield str(name), item, False, container
            yield from walk(item, str(name), str(name))
    elif isinstance(value, list):
        for item in value:
            if key and not isinstance(item, (dict, list)):
                yield key, item, True, ""
            yield from walk(item, key, "")


def canonical(value: object) -> str:
    """Return a JSON value's text with its keys sorted and no spaces."""
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False)


# --- Documents ------------------------------------------------------------------------------


@dataclass
class KeyRow:
    """One row of a key table: its JSON type names, its allowed values, its line."""

    types: set[str]
    values: set[str]
    line: int


@dataclass
class AgentDocument:
    """What the checks read from one agent document's Parsing part."""

    path: str
    names: set[str]
    key_rows: dict[str, KeyRow]
    samples: list[tuple[str, int]]
    key_tables: int


def key_tables(sections: list[drift_common.Section]) -> tuple[dict[str, KeyRow], int]:
    """Return the rows of every key table in the sections, keyed by key, and the table count."""
    rows: dict[str, KeyRow] = {}
    count = 0
    for section in sections:
        in_table = False
        for offset, line in enumerate(section.text.split("\n")):
            text = line.strip()
            if KEY_TABLE_HEADER.match(text):
                in_table, count = True, count + 1
                continue
            if not in_table or not text.startswith("|"):
                in_table = False
                continue
            cells = [cell.strip() for cell in text.strip("|").split("|")]
            if len(cells) < 3 or drift_common.TABLE_SEPARATOR.match(text):
                continue
            spans = CODE_SPAN.findall(cells[0])
            key = spans[0] if spans else cells[0]
            values = set(CODE_SPAN.findall(cells[2]))
            rows[key] = KeyRow(table_types(cells[1]), values, section.start + offset)
    return rows, count


def sample_values(sections: list[drift_common.Section]) -> list[tuple[str, int]]:
    """Return (key-sorted JSON text, line) of each fenced block and fenced line that parses."""
    found: list[tuple[str, int]] = []
    for section in sections:
        for fence in section.fences:
            lines = [text for _, text in fence.lines]
            for value in decode_text("\n".join(lines), whole=True)[:1]:
                found.append((canonical(value), fence.line))
            for number, text in fence.lines:
                if text.strip()[:1] in ("{", "["):
                    found.extend((canonical(value), number) for value in decode_text(text, False))
    return found


def agent_document(root: pathlib.Path, path: pathlib.Path) -> AgentDocument:
    """Return the Parsing part's names, key tables and Fixtures samples of a document."""
    sections = drift_common.research_sections(path)
    parsing = drift_common.within(sections, "Parsing")
    texts = [span.text for section in parsing for span in section.code_spans]
    texts += [
        line
        for section in parsing
        if section.titles[:3] != FIXTURES_SECTION
        for fence in section.fences
        for _, line in fence.lines
    ]
    names = span_names(texts)
    tables = drift_common.within(sections, "Parsing", "Record schema")
    tables += drift_common.within(sections, "Parsing", "Other stores")
    rows, count = key_tables(tables)
    fixtures = drift_common.within(sections, *FIXTURES_SECTION)
    relative = path.relative_to(root).as_posix()
    return AgentDocument(relative, names, rows, sample_values(fixtures), count)


# --- Rules on the built files ---------------------------------------------------------------


def file_findings(
    built: BuiltFile, document: AgentDocument, allowed: list[str], seen: set[tuple[str, str]]
) -> Iterator[drift_common.Finding]:
    """Yield the findings of rules 3 to 6 for one built file, each (kind, detail) once per agent."""

    def report(kind: str, detail: str, line: int = 0) -> Iterator[drift_common.Finding]:
        # A blank key would make a blank detail, which no baseline line can hold.
        detail = detail if detail.strip() else json.dumps(detail)
        if (kind, detail) not in seen:
            seen.add((kind, detail))
            message = "in %s" % built.relative
            yield drift_common.Finding(built.agent, kind, detail, document.path, line, message)

    for name in built.names:
        if is_field_name(name) and not is_documented(name, document.names):
            yield from report("key-undocumented", name)
    for record in [*built.records, *built.rows]:
        for key, value, in_list, container in walk(record):
            field_name = is_field_name(key, container)
            if not in_list and field_name and not is_documented(key, document.names):
                yield from report("key-undocumented", key)
            enum = key in ENUM_KEYS and is_enum_value(value)
            if enum and not is_documented(str(value), document.names):
                yield from report("enum-undocumented", "%s=%s" % (key, value))
            path = project_path(key, value)
            if path and not is_substitute(path, [SUBSTITUTE_PATH, *allowed]):
                yield from report("substitution-undeclared", path)
            row = document.key_rows.get(key)
            if row is None or in_list:
                continue
            if row.types and json_type(value) not in row.types:
                yield from report("type-mismatch", "%s:%s" % (key, json_type(value)), row.line)
            if row.values and isinstance(value, str) and value not in row.values:
                yield from report("type-mismatch", "%s=%s" % (key, value), row.line)


def sample_findings(
    agent: str, files: list[BuiltFile], document: AgentDocument
) -> Iterator[drift_common.Finding]:
    """Yield a `sample-equals-builder` finding per built file a Fixtures sample copies from."""
    written: dict[str, str] = {}
    for built in files:
        for record in built.records:
            if isinstance(record, (dict, list)) and record:
                written.setdefault(canonical(record), built.relative)
    reported: set[str] = set()
    for text, line in document.samples:
        relative = written.get(text)
        if relative and relative not in reported:
            reported.add(relative)
            message = "a Fixtures sample equals a record build_home wrote"
            yield drift_common.Finding(
                agent, "sample-equals-builder", relative, document.path, line, message
            )


def run(root: pathlib.Path) -> Iterator[drift_common.Finding]:
    """Yield every finding of the eight rules for the repository at `root`."""
    documents = drift_common.agent_documents(root)
    shared = root / "research" / "shared.md"
    if shared.is_file():
        documents.update(shared=shared, project=shared)
    modules = fixture_modules(root, set(documents))
    agents = {module.stem: module.agent for module in modules}
    for module in modules:
        yield from provenance_findings(root, module, documents)
        yield from lineage_findings(root, module, agents)
    catalog = drift_common.tables(root / drift_common.CATALOG_TEXT).catalog
    files = built_files(root, catalog)
    by_agent: dict[str, list[BuiltFile]] = {}
    for built in files:
        if not built.agent:
            location = (FIXTURES / "__init__.py").as_posix()
            message = "build_home wrote a file no catalog entry claims"
            yield drift_common.Finding("-", "file-unclaimed", built.relative, location, 0, message)
        else:
            by_agent.setdefault(built.agent, []).append(built)
    parsed = {agent: agent_document(root, path) for agent, path in documents.items()}
    lineages = lineage_agents(modules)
    for agent, agent_files in sorted(by_agent.items()):
        if agent in parsed:
            names = set(parsed[agent].names)
            for other in sorted(lineages.get(agent, set()) & set(parsed)):
                names |= parsed[other].names
            document = dataclasses.replace(parsed[agent], names=names)
            yield from agent_findings(agent, agent_files, document, modules)
    with_tables = len({item.path for item in parsed.values() if item.key_tables})
    print(
        "%s: build_home wrote %d file(s); %d document(s) hold a key table"
        % (CHECKER, len(files), with_tables),
        file=sys.stderr,
    )


def agent_findings(
    agent: str, files: list[BuiltFile], document: AgentDocument, modules: list[FixtureModule]
) -> Iterator[drift_common.Finding]:
    """Yield the findings of rules 3 to 6 and 8 for the files one agent's entries claim."""
    allowed = [
        found
        for module in modules
        if module.agent == agent
        for found in declared_paths(module.docstring)
    ]
    seen: set[tuple[str, str]] = set()
    for built in files:
        yield from file_findings(built, document, allowed, seen)
    yield from sample_findings(agent, files, document)


def main(arguments: list[str] | None = None, root: pathlib.Path | None = None) -> int:
    """Run the check against the baseline and return the exit status."""
    summary = (__doc__ or "").split("\n")[0]
    return drift_common.main_with_baseline(CHECKER, run, arguments, root, summary)


if __name__ == "__main__":
    sys.exit(main())
