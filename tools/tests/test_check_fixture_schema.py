"""Tests for tools/check_fixture_schema.py on a synthetic repository and fixtures package."""

from __future__ import annotations

import contextlib
import io
import pathlib
import sqlite3

import check_fixture_schema as checker
import drift_common

CATALOG_TEXT = """\
# home artifacts (agent|glob relative to home)
alpha|.alpha
beta|.beta
gamma|.gamma
# project artifacts (relative to each discovered project)
alpha|.alpha
project|AGENTS.md
# excluded unless --full
# credential files (flagged secret, skipped with --no-secrets)
# docker volumes (agent|volume name glob)
"""

INIT = '''\
"""Synthetic fixtures."""

from .alpha import build_alpha
from .beta import build_beta as beta_home
from .gamma import build_gamma

HOME_BUILDERS = (
    (build_alpha, ""),
    (beta_home, ""),
    (build_gamma, ""),
)


def build_home(home, with_noise=True):
    """Write every synthetic agent's state, a noise file and a project."""
    for builder, directory in HOME_BUILDERS:
        builder(home / directory)
    (home / "Documents").mkdir(parents=True)
    (home / "Documents/notes.txt").write_text("noise\\n")
    (home / "dev/repo/.alpha").mkdir(parents=True)
    (home / "dev/repo/.alpha/project.json").write_text('{"projectKey": 1}')
    (home / "dev/repo/AGENTS.md").write_text("# instructions\\n")
    return home
'''

COMMON = '''\
"""Helpers the synthetic modules share."""

import json


def write_jsonl(path, records):
    """Write one JSON line per record."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("".join(json.dumps(record) + "\\n" for record in records))
'''

ALPHA = '''\
"""Synthetic alpha state, its document not named."""

import json
import sqlite3

import zstandard

from .common import write_jsonl


def alpha_records():
    """Records with no provenance line."""
    return [
        {"type": "known", "documentedKey": 1, "cwd": "/srv/proj"},
        {"type": "unknown", "strayKey": 2, "cwd": "/home/alice/other"},
    ]


def alpha_helper():
    """Return a record another module borrows."""
    return {"helperKey": 1}


def build_alpha(home):
    """Write the alpha store."""
    base = home / ".alpha"
    write_jsonl(base / "history.jsonl", alpha_records())
    (base / "config.yaml").write_text("documentedKey: 1\\nyamlStray: 2\\n# commentKey: 3\\n")
    (base / "config.toml").write_text("[tomlTable]\\ntomlStray = 1\\n")
    data = json.dumps({"zstdStray": 1}).encode() + b"\\n"
    (base / "log.jsonl.zst").write_bytes(zstandard.ZstdCompressor().compress(data))
    connection = sqlite3.connect(base / "store.db")
    connection.execute("CREATE TABLE sessions (id TEXT, strayColumn TEXT)")
    row = ("s1", json.dumps({"columnKey": 1}))
    connection.execute("INSERT INTO sessions VALUES (?, ?)", row)
    connection.commit()
    connection.close()
'''

BETA = '''\
"""Synthetic beta state of research/beta.md.

Paths: `/home/alice/declared`
"""

import json

from .alpha import alpha_helper


def beta_records():
    """Records of the beta store.

    Source: research/beta.md#record-schema; build 1.0
    """
    return [
        {"count": 3, "mode": "fast", "cwd": "/home/alice/declared/sub", "collectionOnly": 1},
        {"count": "three", "mode": "medium"},
    ]


def write_beta_extra(path):
    """Write a file whose Source line names a missing anchor.

    Source: research/beta.md#no-such-heading; build 1.0
    """
    path.write_text(json.dumps(alpha_helper()))


def build_beta(home):
    """Write the beta store."""
    (home / ".beta").mkdir(parents=True)
    (home / ".beta/state.json").write_text(json.dumps(beta_records()))
'''

GAMMA = '''\
"""Synthetic gamma state of research/gamma.md.

Lineage: alpha
"""

import json

from .alpha import alpha_helper


def build_gamma(home):
    """Write the gamma store from alpha's helper."""
    (home / ".gamma").mkdir(parents=True)
    (home / ".gamma/state.json").write_text(json.dumps(alpha_helper()))
'''

ALPHA_DOCUMENT = """\
# alpha

Intro.

## Parsing

### Record schema

Lines hold `documentedKey`, `type` (`known`), `cwd`, `helperKey` and `projectKey`; the store has
`sessions` with `id`, and `config.yaml` and `config.toml` keys.

```sql
CREATE TABLE sessions (id TEXT, fencedColumn TEXT);
```

### Parser plan

#### Fixtures

```jsonl
{"cwd": "/srv/proj", "documentedKey": 1, "type": "known"}
{"documentedKey": 9, "strayKey": 1}
```
"""

BETA_DOCUMENT = """\
# beta

## Collection

`collectionOnly` is named here only, outside Parsing.

## Parsing

### Record schema

| Key | Type | Values | Builds |
| --- | --- | --- | --- |
| `count` | number | | 1.0 |
| `mode` | string | `fast`, `medium`, `slow` | 1.0 |
| `cwd` | string | | 1.0 |

### Parser plan

#### Fixtures

```json
{"count": 4}
```
"""


def write(path: pathlib.Path, text: str) -> None:
    """Write `text` to `path`, creating its directory."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def make_repository(root: pathlib.Path) -> pathlib.Path:
    """Write the synthetic catalog, documents and fixtures package under `root`."""
    write(root / "analyzer/doubleagent/catalog.txt", CATALOG_TEXT)
    fixtures = root / "analyzer/tests/fixtures"
    for name, text in (
        ("__init__", INIT),
        ("common", COMMON),
        ("alpha", ALPHA),
        ("beta", BETA),
        ("gamma", GAMMA),
    ):
        write(fixtures / ("%s.py" % name), text)
    write(root / "research/alpha.md", ALPHA_DOCUMENT)
    write(root / "research/beta.md", BETA_DOCUMENT)
    return root


def findings_of(root: pathlib.Path) -> set[tuple[str, str, str]]:
    """Return (agent, kind, detail) of every finding on the synthetic repository."""
    with contextlib.redirect_stderr(io.StringIO()):
        return {
            (finding.agent, finding.kind, finding.detail)
            for finding in checker.run(make_repository(root))
        }


def run_main(root: pathlib.Path, arguments: list[str]) -> tuple[int, str]:
    """Run the checker's main on `root`; return its status and stdout."""
    output = io.StringIO()
    with contextlib.redirect_stdout(output), contextlib.redirect_stderr(io.StringIO()):
        status = checker.main(arguments, root)
    return status, output.getvalue()


# --- Rules -----------------------------------------------------------------------------------


def test_builder_provenance_missing(tmp_path):
    found = findings_of(tmp_path)
    assert ("alpha", "builder-provenance-missing", "alpha_records") in found
    assert ("beta", "builder-provenance-missing", "write_beta_extra") in found
    assert ("beta", "builder-provenance-missing", "beta_records") not in found


def test_builder_provenance_reaches_every_unregistered_public_function(tmp_path):
    found = findings_of(tmp_path)
    assert ("alpha", "builder-provenance-missing", "alpha_helper") in found
    for name in ("build_alpha", "build_beta", "build_gamma"):
        agent = name.split("_")[1]
        assert (agent, "builder-provenance-missing", name) not in found, name


def test_registered_builders_follow_import_aliases(tmp_path):
    root = make_repository(tmp_path)
    expected = {("alpha", "build_alpha"), ("beta", "build_beta"), ("gamma", "build_gamma")}
    assert checker.registered_builders(root) == expected
    assert checker.registered_builders(tmp_path / "empty") == frozenset()


def test_registered_builders_exempt_only_their_own_module(tmp_path):
    root = make_repository(tmp_path)
    modules = checker.fixture_modules(root, {"alpha", "beta", "gamma"})
    alpha = next(module for module in modules if module.stem == "alpha")
    documents = {"alpha": root / "research/alpha.md", "beta": root / "research/beta.md"}
    registered = frozenset({("beta", "alpha_records"), ("alpha", "alpha_helper")})
    details = {
        finding.detail
        for finding in checker.provenance_findings(root, alpha, documents, registered)
        if finding.kind == "builder-provenance-missing"
    }
    assert "alpha_records" in details
    assert "alpha_helper" not in details


def test_module_provenance_missing(tmp_path):
    found = findings_of(tmp_path)
    assert ("alpha", "module-provenance-missing", "alpha") in found
    assert ("beta", "module-provenance-missing", "beta") not in found
    assert ("gamma", "module-provenance-missing", "gamma") not in found


def test_module_provenance_needs_the_whole_document_path(tmp_path):
    for named, expected in (("research/alpha.md.", False), ("old-research/alpha.md.", True)):
        root = make_repository(tmp_path / named)
        module = root / "analyzer/tests/fixtures/alpha.py"
        text = module.read_text(encoding="utf-8")
        module.write_text(text.replace("its document not named.", named), encoding="utf-8")
        with contextlib.redirect_stderr(io.StringIO()):
            found = {(finding.agent, finding.kind) for finding in checker.run(root)}
        assert (("alpha", "module-provenance-missing") in found) is expected, named


def test_key_undocumented(tmp_path):
    found = findings_of(tmp_path)
    for key in ("strayKey", "yamlStray", "tomlStray", "tomlTable", "zstdStray", "strayColumn"):
        assert ("alpha", "key-undocumented", key) in found, key
    for key in ("documentedKey", "type", "cwd", "sessions", "id", "projectKey", "commentKey"):
        assert ("alpha", "key-undocumented", key) not in found, key
    assert ("alpha", "key-undocumented", "columnKey") in found


def test_key_documented_by_schema_fence_not_by_sample(tmp_path):
    make_repository(tmp_path)
    module = tmp_path / "analyzer/tests/fixtures/alpha.py"
    module.write_text(module.read_text().replace("strayColumn", "fencedColumn"))
    with contextlib.redirect_stderr(io.StringIO()):
        found = {(item.kind, item.detail) for item in checker.run(tmp_path)}
    assert ("key-undocumented", "fencedColumn") not in found
    assert ("key-undocumented", "strayKey") in found


def test_key_undocumented_reads_only_the_parsing_part(tmp_path):
    found = findings_of(tmp_path)
    assert ("beta", "key-undocumented", "collectionOnly") in found
    assert ("beta", "key-undocumented", "count") not in found
    assert ("beta", "key-undocumented", "cwd") not in found
    assert not any(agent == "gamma" for agent, kind, _ in found if kind == "key-undocumented")


def test_key_documented_by_the_document_of_an_imported_module(tmp_path):
    state = '    (home / ".beta/state.json").write_text(json.dumps(beta_records()))\n'
    borrowed = state + '    (home / ".beta/extra.json").write_text(json.dumps(alpha_helper()))\n'
    literal = state + '    (home / ".beta/extra.json").write_text(\'{"helperKey": 1}\')\n'
    own = BETA.replace("from .alpha import alpha_helper\n", "").replace(state, literal)
    own = own.replace("json.dumps(alpha_helper())", "'{}'")
    found = {}
    for name, text in (("imported", BETA.replace(state, borrowed)), ("own", own)):
        root = make_repository(tmp_path / name)
        write(root / "analyzer/tests/fixtures/beta.py", text)
        with contextlib.redirect_stderr(io.StringIO()):
            found[name] = {(item.agent, item.kind, item.detail) for item in checker.run(root)}
    assert ("beta", "key-undocumented", "helperKey") not in found["imported"]
    assert ("beta", "cross-agent-builder", "alpha_helper") in found["imported"]
    assert ("beta", "key-undocumented", "helperKey") in found["own"]


def test_lineage_agents_follow_imports_not_declarations(tmp_path):
    modules = checker.fixture_modules(make_repository(tmp_path), {"alpha", "beta", "gamma"})
    assert checker.lineage_agents(modules) == {
        "alpha": set(),
        "beta": {"alpha"},
        "gamma": {"alpha"},
    }


def test_file_unclaimed(tmp_path):
    found = findings_of(tmp_path)
    assert ("-", "file-unclaimed", "Documents/notes.txt") in found
    unclaimed = {detail for _, kind, detail in found if kind == "file-unclaimed"}
    assert unclaimed == {"Documents/notes.txt"}


def test_enum_undocumented(tmp_path):
    found = findings_of(tmp_path)
    assert ("alpha", "enum-undocumented", "type=unknown") in found
    assert ("alpha", "enum-undocumented", "type=known") not in found


def test_type_mismatch(tmp_path):
    found = findings_of(tmp_path)
    assert ("beta", "type-mismatch", "count:string") in found
    assert ("beta", "type-mismatch", "count:number") not in found
    assert ("beta", "type-mismatch", "mode=fast") not in found
    assert ("beta", "type-mismatch", "mode=medium") not in found
    assert not any(kind == "type-mismatch" for agent, kind, _ in found if agent == "alpha")


def test_type_mismatch_value_outside_values_cell(tmp_path):
    make_repository(tmp_path)
    document = tmp_path / "research/beta.md"
    document.write_text(document.read_text().replace("`medium`, ", ""), encoding="utf-8")
    with contextlib.redirect_stderr(io.StringIO()):
        found = {(item.kind, item.detail) for item in checker.run(tmp_path)}
    assert ("type-mismatch", "mode=medium") in found
    assert ("type-mismatch", "mode=fast") not in found


TWO_TABLES_DOCUMENT = """\
# beta

## Parsing

### Record schema

#### Turn lines

| Key | Type | Values | Builds |
| --- | --- | --- | --- |
| `type` | string | `user`, `assistant` | 1.0 |
| `count` | number | | 1.0 |

### Other stores

#### Index

| Key | Type | Values | Builds |
| --- | --- | --- | --- |
| `type` | string | `entry` | 1.0 |
| `count` | string | | 1.0 |
"""


def two_table_findings(tmp_path, records, document=TWO_TABLES_DOCUMENT):
    """Return (detail, line) of the type mismatches of records against a document's tables."""
    write(tmp_path / "research/beta.md", document)
    parsed = checker.agent_document(tmp_path, tmp_path / "research/beta.md")
    built = checker.BuiltFile(".beta/state.json", "beta", records=records)
    found = checker.file_findings(built, parsed, [], set())
    return {(item.detail, item.line) for item in found if item.kind == "type-mismatch"}


def table_line(text):
    """Return the line number of a row of the two-table document."""
    return TWO_TABLES_DOCUMENT.split("\n").index(text) + 1


def test_type_mismatch_passes_when_one_row_accepts_the_value(tmp_path):
    records: list[object] = [{"type": "entry", "count": "three"}, {"type": "user", "count": 3}]
    assert two_table_findings(tmp_path, records) == set()


def test_type_mismatch_allowed_by_no_table_names_the_first_row(tmp_path):
    records: list[object] = [{"type": "system", "count": True}]
    expected = {
        ("type=system", table_line("| `type` | string | `user`, `assistant` | 1.0 |")),
        ("count:boolean", table_line("| `count` | number | | 1.0 |")),
    }
    assert two_table_findings(tmp_path, records) == expected


def test_type_mismatch_type_and_value_met_by_different_rows(tmp_path):
    document = TWO_TABLES_DOCUMENT.replace("| `type` | string | `entry` |", "| `type` | number | |")
    found = two_table_findings(tmp_path, [{"type": "system"}], document)
    line = table_line("| `type` | string | `user`, `assistant` | 1.0 |")
    assert found == {("type=system", line)}


def test_type_mismatch_met_by_no_row_gives_both_details(tmp_path):
    document = TWO_TABLES_DOCUMENT.replace(
        "| `type` | string | `user`, `assistant` |", "| `type` | number | `one` |"
    ).replace("| `type` | string | `entry` |", "| `type` | boolean | `yes` |")
    found = two_table_findings(tmp_path, [{"type": "system"}], document)
    line = table_line("| `type` | string | `user`, `assistant` | 1.0 |")
    assert found == {("type:string", line), ("type=system", line)}


SCOPED_DOCUMENT = """\
# beta

## Parsing

### Record schema

#### Index

Each `index/<id>.json` holds one entry.

| Key | Type | Values | Builds |
| --- | --- | --- | --- |
| `type` | string | | 1.0 |
| `count` | number | | 1.0 |

#### Turn lines

`~/.beta/history.jsonl` holds one turn per line.

| Key | Type | Values | Builds |
| --- | --- | --- | --- |
| `type` | string | `user`, `assistant`, `index.json` | 1.0 |

#### Shared fields

Every store repeats these.

| Key | Type | Values | Builds |
| --- | --- | --- | --- |
| `count` | string | | 1.0 |
"""


def scoped_findings(tmp_path, relative, records, document=SCOPED_DOCUMENT):
    """Return (detail, line) of the type mismatches of one file beside an index file."""
    write(tmp_path / "research/beta.md", document)
    parsed = checker.agent_document(tmp_path, tmp_path / "research/beta.md")
    files = [
        checker.BuiltFile(relative, "beta", records=records),
        checker.BuiltFile(".beta/index/a1.json", "beta", records=[{"type": "entry"}]),
    ]
    found = checker.agent_findings("beta", files, parsed, [])
    return {(item.detail, item.line) for item in found if item.kind == "type-mismatch"}


def scoped_line(text):
    """Return the line number of a row of the scoped document."""
    return SCOPED_DOCUMENT.split("\n").index(text) + 1


def test_type_mismatch_consults_only_the_sections_naming_the_file(tmp_path):
    found = scoped_findings(tmp_path, ".beta/history.jsonl", [{"type": "system"}])
    turn_row = scoped_line("| `type` | string | `user`, `assistant`, `index.json` | 1.0 |")
    assert found == {("type=system", turn_row)}


def test_type_mismatch_names_the_first_consulted_row(tmp_path):
    shared_type = "| `type` | string | `user` | 1.0 |"
    document = SCOPED_DOCUMENT.replace(
        "| `count` | string | | 1.0 |", "| `count` | string | | 1.0 |\n" + shared_type
    )
    found = scoped_findings(tmp_path, ".beta/history.jsonl", [{"type": "system"}], document)
    lines = document.split("\n")
    index_row = lines.index("| `type` | string | | 1.0 |") + 1
    turn_row = lines.index("| `type` | string | `user`, `assistant`, `index.json` | 1.0 |") + 1
    shared_row = lines.index(shared_type) + 1
    assert index_row < turn_row < shared_row
    assert found == {("type=system", turn_row)}


def test_type_mismatch_consults_every_row_when_no_section_names_the_file(tmp_path):
    assert scoped_findings(tmp_path, ".beta/other.json", [{"type": "system"}]) == set()


def test_type_mismatch_consults_sections_naming_no_built_file(tmp_path):
    turn_count = "| `count` | number | | 2.0 |"
    document = SCOPED_DOCUMENT.replace(
        "`index.json` | 1.0 |", "`index.json` | 1.0 |\n" + turn_count
    )
    records: list[object] = [{"count": "three"}]
    assert scoped_findings(tmp_path, ".beta/history.jsonl", records, document) == set()
    scoped = document.replace("Every store repeats these.", "`index/<id>.json` too.")
    found = scoped_findings(tmp_path / "scoped", ".beta/history.jsonl", records, scoped)
    assert found == {("count:string", scoped.split("\n").index(turn_count) + 1)}


HOME_FORMS_DOCUMENT = """\
# beta

## Parsing

### Record schema

#### Stores

`C:\\Users\\<user>\\.beta\\history.jsonl`, `/home/<user>/.beta/b.jsonl`, `sessions/`,
`${XDG_DATA_HOME}/beta/c.db` and `~/` hold the records of `type`.

| Key | Type | Values | Builds |
| --- | --- | --- | --- |
| `type` | string | | 1.0 |
"""


def test_section_paths_take_home_prefixed_and_directory_spans(tmp_path):
    write(tmp_path / "doc.md", HOME_FORMS_DOCUMENT)
    sections = drift_common.research_sections(tmp_path / "doc.md")
    paths = checker.section_paths(drift_common.within(sections, "Parsing"))
    assert paths == {
        "Parsing/Record schema/Stores": [
            "C:\\Users\\<user>\\.beta\\history.jsonl",
            "/home/<user>/.beta/b.jsonl",
            "sessions/",
            "${XDG_DATA_HOME}/beta/c.db",
        ]
    }


def test_section_paths_skip_key_table_cells(tmp_path):
    write(tmp_path / "doc.md", SCOPED_DOCUMENT)
    sections = drift_common.research_sections(tmp_path / "doc.md")
    paths = checker.section_paths(drift_common.within(sections, "Parsing"))
    assert paths == {
        "Parsing/Record schema/Index": ["index/<id>.json"],
        "Parsing/Record schema/Turn lines": ["~/.beta/history.jsonl"],
        "Parsing/Record schema/Shared fields": [],
    }
    rows, _ = checker.key_tables(drift_common.within(sections, "Parsing"))
    assert [row.section for row in rows["count"]] == [
        "Parsing/Record schema/Index",
        "Parsing/Record schema/Shared fields",
    ]


def test_names_file_forms():
    cases = [
        ("index/<id>.json", ".beta/index/a1.json", True),
        ("<id>.json", ".beta/index/a1.json", True),
        ("*.jsonl", ".beta/history.jsonl", True),
        ("sessions/*/<id>.jsonl", ".beta/sessions/p/a1.jsonl", True),
        ("~/.beta/history.jsonl", ".beta/history.jsonl", True),
        ("<home>/.beta/history.jsonl", ".beta/history.jsonl", True),
        ("%APPDATA%\\Beta\\state.json", "AppData/Roaming/Beta/state.json", True),
        ("C:\\Users\\<user>\\.beta\\history.jsonl", ".beta/history.jsonl", True),
        ("/home/<user>/.beta/history.jsonl", ".beta/history.jsonl", True),
        ("/Users/alice/.beta/history.jsonl", ".beta/history.jsonl", True),
        ("$XDG_DATA_HOME/beta/state.db", ".local/share/beta/state.db", True),
        ("${XDG_DATA_HOME}/beta/state.db", ".local/share/beta/state.db", True),
        ("sessions/", ".beta/sessions/p/a1.jsonl", True),
        (".beta/sessions/", ".beta/sessions/p/a1.jsonl", True),
        ("sessions/<id>/", ".beta/sessions/p/a1.jsonl", True),
        (".beta/sessions/", ".beta/sessions", True),
        ("~/.local/state/opencode", ".local/state/opencode/kv.json", True),
        ("%LOCALAPPDATA%\\OpenAI\\Codex", "AppData/Local/OpenAI/Codex/log.txt", True),
        ("state/opencode", ".local/state/opencode-two/kv.json", False),
        ("history.jsonl", ".beta/history.jsonl", True),
        ("ory.jsonl", ".beta/history.jsonl", False),
        (".beta/history.jsonl", "x.beta/history.jsonl", False),
        ("history.jsonl", ".beta/history.jsonl.bak", False),
        ("index/<id>.json", ".beta/index/sub/a1.json", False),
        ("<owner>/<repo>", ".beta/history.jsonl", False),
        ("/model", ".beta/model", False),
    ]
    for span, relative, expected in cases:
        assert checker.names_file(span, relative) is expected, (span, relative)


BLOB_DOCUMENT = """\
# beta

## Parsing

### Record schema

| Key | Type | Values | Builds |
| --- | --- | --- | --- |
| `data` | blob or null | | 1.0 |
"""


def blob_mismatches(tmp_path, document):
    """Return the type mismatches of a SQLite table with a BLOB column against a document."""
    database = tmp_path / "home/.beta/store.db"
    database.parent.mkdir(parents=True)
    connection = sqlite3.connect(database)
    connection.execute("CREATE TABLE archive (data BLOB)")
    connection.executemany("INSERT INTO archive VALUES (?)", [(b"\x00\x01",), (None,)])
    connection.commit()
    connection.close()
    built = checker.BuiltFile(".beta/store.db", "beta")
    checker.read_built_file(database, built)
    write(tmp_path / "research/beta.md", document)
    parsed = checker.agent_document(tmp_path, tmp_path / "research/beta.md")
    found = checker.file_findings(built, parsed, [], set())
    return {item.detail for item in found if item.kind == "type-mismatch"}


def test_type_mismatch_blob_column(tmp_path):
    assert blob_mismatches(tmp_path / "blob", BLOB_DOCUMENT) == set()
    document = BLOB_DOCUMENT.replace("blob or null", "string or null")
    assert blob_mismatches(tmp_path / "string", document) == {"data:blob"}


def test_substitution_undeclared(tmp_path):
    found = findings_of(tmp_path)
    assert ("alpha", "substitution-undeclared", "/home/alice/other") in found
    assert ("alpha", "substitution-undeclared", "/srv/proj") not in found
    assert ("beta", "substitution-undeclared", "/home/alice/declared/sub") not in found


def test_cross_agent_builder(tmp_path):
    found = findings_of(tmp_path)
    assert ("beta", "cross-agent-builder", "alpha_helper") in found
    assert ("gamma", "cross-agent-builder", "alpha_helper") not in found


def test_sample_equals_builder(tmp_path):
    found = findings_of(tmp_path)
    assert ("alpha", "sample-equals-builder", ".alpha/history.jsonl") in found
    assert not any(kind == "sample-equals-builder" for agent, kind, _ in found if agent == "beta")


def test_findings_are_stable_details(tmp_path):
    for agent, kind, detail in findings_of(tmp_path):
        assert "\n" not in detail and not detail.isdigit(), (agent, kind, detail)


# --- Baseline --------------------------------------------------------------------------------


def test_stale_baseline_entry_fails(tmp_path):
    make_repository(tmp_path)
    assert run_main(tmp_path, ["--update-baseline"])[0] == 0
    assert run_main(tmp_path, [])[0] == 0
    stale = "check_fixture_schema|key-undocumented|goneKey"
    path = tmp_path / "tools/drift-baseline/alpha.txt"
    path.write_text(path.read_text() + stale + "\n", encoding="utf-8")
    status, output = run_main(tmp_path, [])
    assert status == 1
    assert 'stale: remove "%s"' % stale in output


def test_update_baseline_removes_stale_entry(tmp_path):
    make_repository(tmp_path)
    stale = "check_fixture_schema|key-undocumented|goneKey"
    write(tmp_path / "tools/drift-baseline/alpha.txt", stale + "\n")
    assert run_main(tmp_path, ["--update-baseline"])[0] == 0
    assert stale not in (tmp_path / "tools/drift-baseline/alpha.txt").read_text()
    assert run_main(tmp_path, [])[0] == 0


def test_new_finding_fails_without_baseline(tmp_path):
    make_repository(tmp_path)
    status, output = run_main(tmp_path, [])
    assert status == 1
    assert "key-undocumented [alpha]: strayKey" in output


# --- Heuristics ------------------------------------------------------------------------------


def test_is_public_builder_forms():
    registered = frozenset({"build_alpha"})
    builders = ("alpha_records", "write_alpha_state", "build_alpha_extra", "alpha_rows")
    for name in (*builders, "alpha_header", "claude_309_records"):
        assert checker.is_public_builder(name, registered), name
    for name in ("build_alpha", "_alpha_records", "_write_x", "_uuid7"):
        assert not checker.is_public_builder(name, registered), name


def test_source_line_forms():
    accepted = {
        "Source: research/alpha.md#record-schema; build 2.1.310": (
            "alpha",
            "record-schema",
            "2.1.310",
        ),
        "    Source: research/codex-cli.md#fixtures; source 1a2b3c4": (
            "codex-cli",
            "fixtures",
            "1a2b3c4",
        ),
    }
    for line, expected in accepted.items():
        assert checker.source_line(line) == expected, line
    for line in (
        "Source: research/alpha.md#record-schema",
        "Source: research/alpha.md; build 1.0",
        "Source: research/alpha.md#x; version 1.0",
        "Source: research/alpha.md#x; build ",
        "Source: alpha.md#x; build 1.0",
        "The source: research/alpha.md#x; build 1.0",
    ):
        assert checker.source_line(line) is None, line


def test_module_agent_forms():
    documents = {"continue", "codex-cli", "claude-code"}
    assert checker.module_agent("codex_cli", "", documents) == "codex-cli"
    assert checker.module_agent("continue_dev", "", documents) == "continue"
    assert (
        checker.module_agent("x", "State of research/claude-code.md.", documents) == "claude-code"
    )
    assert checker.module_agent("zed", "State of research/missing.md.", documents) == "zed"
    assert checker.module_agent("kilo_code", "", documents) == "kilo-code"


def test_names_document_forms():
    named = (
        ("Built from research/bar.md.", "bar"),
        ("See `research/bar.md#record-schema`.", "bar"),
        ("(research/bar.md)", "bar"),
        ("Built from research/foo-bar.md.", "foo-bar"),
    )
    unnamed = (
        ("Built from research/foo-bar.md.", "bar"),
        ("Built from research/foo-bar.md.", "foo"),
        ("Built from old-research/bar.md.", "bar"),
        ("Built from .research/bar.md.", "bar"),
        ("Built from research/bar.mdx.", "bar"),
        ("Built from research/bar.md.bak.", "bar"),
        ("Built from research/bar.md/notes.", "bar"),
    )
    for docstring, agent in named:
        assert checker.names_document(docstring, agent), (docstring, agent)
    for docstring, agent in unnamed:
        assert not checker.names_document(docstring, agent), (docstring, agent)


def test_declared_paths_forms():
    docstring = "Intro.\n\nPaths: `/home/alice/proj`, `C:\\Users\\alice\\proj`\nPaths `/x`\n"
    assert checker.declared_paths(docstring) == ["/home/alice/proj", "C:\\Users\\alice\\proj"]
    assert checker.declared_paths("No paths: `/x` here.") == []


def test_declared_paths_read_a_wrapped_paths_line():
    docstring = (
        "Intro `/intro`.\n\n"
        "Paths: `/home/alice/proj`,\n"
        "       `C:\\Users\\alice\\proj` and\n"
        "         `/Users/alice/proj`.\n"
        "         see `/prose` for the rest\n"
        "         `/after-the-prose`\n"
        "      `/under-indented`\n"
        "       `/after-the-break`\n"
        "\n"
        "  Paths: `/indented`\n"
        "         `/continued`\n"
        "         Source: `/a-label`\n"
        "         `/after-the-label`\n"
        "Paths: `/last`\n"
        "\n"
        "       `/after-a-blank-line`\n"
    )
    assert checker.declared_paths(docstring) == [
        "/home/alice/proj",
        "C:\\Users\\alice\\proj",
        "/Users/alice/proj",
        "/indented",
        "/continued",
        "/last",
    ]


def test_lineage_forms():
    assert checker.lineage("Intro.\n\nLineage: codex-cli, opencode\n") == {"codex-cli", "opencode"}
    assert checker.lineage("Lineage: roo-code") == {"roo-code"}
    assert checker.lineage("Its lineage: roo-code") == set()


def test_yaml_keys_forms():
    accepted = {
        "key: value": "key",
        "key:": "key",
        "  nested_key: 1": "nested_key",
        "- item_key: x": "item_key",
        '"quoted key": 1': "quoted key",
        "'single': 1": "single",
    }
    for line, key in accepted.items():
        assert checker.yaml_keys(line) == [key], line
    for line in ("# comment: x", "- http://example.com", "key:value", "- plain item", ""):
        assert checker.yaml_keys(line) == [], line


def test_toml_keys_forms():
    accepted = {
        "key = 1": ["key"],
        "a.b = 1": ["a", "b"],
        '"quoted.key" = 1': ["quoted.key"],
        "[table]": ["table"],
        "[[array.of]]": ["array", "of"],
        '[projects."/srv/proj"]': ["projects", "/srv/proj"],
    }
    for line, keys in accepted.items():
        assert checker.toml_keys(line) == keys, line
    for line in ("# key = 1", "= 1", "plain text", ""):
        assert checker.toml_keys(line) == [], line


def test_json_type_forms():
    cases = [
        (None, "null"),
        (True, "boolean"),
        (3, "number"),
        (2.5, "number"),
        ("x", "string"),
        ({}, "object"),
        ([], "array"),
        (b"x", "blob"),
        (object(), ""),
    ]
    for value, kind in cases:
        assert checker.json_type(value) == kind, value


def test_table_types_forms():
    assert checker.table_types("string") == {"string"}
    assert checker.table_types("string or null") == {"string", "null"}
    assert checker.table_types("`number`, `boolean`") == {"number", "boolean"}
    assert checker.table_types("object/array") == {"object", "array"}
    assert checker.table_types("blob or null") == {"blob", "null"}
    assert checker.table_types("integer") == set()


def test_key_tables_forms(tmp_path):
    write(tmp_path / "doc.md", BETA_DOCUMENT)
    sections = drift_common.within(drift_common.research_sections(tmp_path / "doc.md"), "Parsing")
    rows, count = checker.key_tables(sections)
    assert count == 1
    assert len(rows["mode"]) == 1
    assert rows["mode"][0].types == {"string"}
    assert rows["mode"][0].values == {"fast", "medium", "slow"}
    assert rows["count"][0].values == set()
    write(tmp_path / "two.md", TWO_TABLES_DOCUMENT)
    sections = drift_common.within(drift_common.research_sections(tmp_path / "two.md"), "Parsing")
    rows, count = checker.key_tables(sections)
    assert count == 2
    assert [row.values for row in rows["type"]] == [{"user", "assistant"}, {"entry"}]
    assert [row.types for row in rows["count"]] == [{"number"}, {"string"}]
    assert rows["type"][0].line < rows["type"][1].line
    other = BETA_DOCUMENT.replace("| Key | Type | Values | Builds |", "| Key | Type | Notes |")
    write(tmp_path / "other.md", other)
    sections = drift_common.within(drift_common.research_sections(tmp_path / "other.md"), "Parsing")
    assert checker.key_tables(sections) == ({}, 0)
    since = BETA_DOCUMENT.replace("| Builds |", "| Since |")
    write(tmp_path / "since.md", since)
    sections = drift_common.within(drift_common.research_sections(tmp_path / "since.md"), "Parsing")
    assert checker.key_tables(sections)[1] == 1


def test_is_enum_value_forms():
    for value in ("user", "tool_use", "allow-once", "mcp.call", "a:b", "INFO"):
        assert checker.is_enum_value(value), value
    for value in ("two words", "/srv/proj", "https://x", "3", "0abc", "", 3, None, "x" * 65):
        assert not checker.is_enum_value(value), value


def test_project_path_forms():
    accepted = {
        ("cwd", "/srv/proj"): "/srv/proj",
        ("workspaceRoots", "/home/alice/x"): "/home/alice/x",
        ("project_path", "C:\\Users\\alice\\proj"): "C:\\Users\\alice\\proj",
        ("folderUri", "file:///home/alice/my%20proj"): "/home/alice/my proj",
        ("rootUri", "file:///C:/Users/alice/proj"): "C:/Users/alice/proj",
    }
    for (key, value), path in accepted.items():
        assert checker.project_path(key, value) == path, (key, value)
    for key, value in (
        ("file_path", "/srv/other/x.py"),
        ("cwd", "relative/dir"),
        ("cwd", "//server/share"),
        ("cwd", "/"),
        ("cwd", 3),
        ("command", "/bin/ls"),
    ):
        assert checker.project_path(key, value) == "", (key, value)


def test_is_substitute_forms():
    allowed = ["/srv/proj", "C:\\Users\\alice\\proj"]
    for path in ("/srv/proj", "/srv/proj/sub", "C:/Users/alice/proj/x", "C:\\Users\\alice\\proj"):
        assert checker.is_substitute(path, allowed), path
    for path in ("/srv/proj2", "/srv", "/home/alice/proj"):
        assert not checker.is_substitute(path, allowed), path


def test_span_names_forms():
    names = checker.span_names(["message.content[]", '{"type": "user"}', "a b", "x/y:z"])
    for name in ("message.content", "message", "content", "type", "user", "a b", "x", "y", "z"):
        assert checker.is_documented(name, names), name
    for name in ("content[]", "mess", "a b c", "y:z"):
        assert not checker.is_documented(name, names), name


def test_is_field_name_forms():
    for key in ("sessionId", "$set", "chat.tools.autoApprove", "x-api-key", "_meta", "@type"):
        assert checker.is_field_name(key), key
    for key in (
        "",
        "two words",
        "/srv/proj",
        "Sat, 03 Oct 2026 10:00:00 GMT",
        "a=b",
        "1abc",
        "a1b2c3d4-0000-4000-8000-000000000001",
        "deadbeefcafe",
        "x" * 65,
    ):
        assert not checker.is_field_name(key), key


def test_is_field_name_data_key_forms():
    for key in ("GITHUB_TOKEN", "ANTHROPIC_API_KEY", "github:create_issue", "toolu_01"):
        assert not checker.is_field_name(key), key
    for key in ("msg_01abc", "call_x"):
        assert not checker.is_field_name(key), key
    for container in ("env", "headers", "mcpServers", "sessions", "worktrees", "sidecars"):
        assert not checker.is_field_name("brave-otter", container), container
        assert not checker.is_field_name("weekly-check", container), container
    for key, container in (("command", "github"), ("Id", ""), ("ID", ""), ("message", "")):
        assert checker.is_field_name(key, container), (key, container)
    assert checker.is_field_name("sessionId", "session")


def test_walk_names_the_container_of_each_key():
    record = {
        "mcpServers": {"github": {"command": "x", "env": {"GITHUB_TOKEN": "y"}}},
        "items": [{"type": "a"}],
    }
    found = [(key, container) for key, _, in_list, container in checker.walk(record) if not in_list]
    assert found == [
        ("mcpServers", ""),
        ("github", "mcpServers"),
        ("command", "github"),
        ("env", "github"),
        ("GITHUB_TOKEN", "env"),
        ("items", ""),
        ("type", ""),
    ]


def test_claiming_agent_forms():
    catalog = drift_common.catalog_module().parse(CATALOG_TEXT)
    assert checker.claiming_agent(".alpha/x.json", catalog) == "alpha"
    assert checker.claiming_agent("dev/repo/.alpha/project.json", catalog) == "alpha"
    assert checker.claiming_agent("dev/repo/AGENTS.md", catalog) == "project"
    assert checker.claiming_agent("Documents/notes.txt", catalog) == ""


def test_decode_text_forms():
    assert checker.decode_text('{"a": 1}', whole=True) == [{"a": 1}]
    assert checker.decode_text('{"a": 1}\nnot json\n[2]\n', whole=True) == [{"a": 1}, [2]]
    assert checker.decode_text('{"a": 1}', whole=False) == [{"a": 1}]
    assert checker.decode_text("plain text", whole=True) == []


def test_run_reports_file_and_key_table_counts(tmp_path):
    make_repository(tmp_path)
    errors = io.StringIO()
    with contextlib.redirect_stderr(errors):
        list(checker.run(tmp_path))
    assert "build_home wrote 10 file(s); 1 document(s) hold a key table" in errors.getvalue()
