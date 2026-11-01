"""Tests for tools/drift_common.py, the drift checkers' shared module, on synthetic inputs."""

from __future__ import annotations

import pathlib
import textwrap

import drift_common
import example_checker
from drift_common import Finding

CATALOG_TEXT = """\
# home artifacts (agent|glob relative to home)
# Alpha vendor
alpha|.alpha
alpha|.alpha.json*
# Beta tool keeps two bases
beta|.config/beta
beta|.config/beta/sub
gamma|.gamma/nested
# project artifacts (relative to each discovered project)
alpha|.alpha
project|AGENTS.md
# excluded unless --full
.alpha/cache
# Beta caches
.config/beta/[Cc]ache
# credential files (flagged secret, skipped with --no-secrets)
.alpha/auth.json
# docker volumes (agent|volume name glob)
# Gamma's volume, mounted at /a0/usr
gamma|*gamma_usr
"""


def write(path: pathlib.Path, text: str) -> pathlib.Path:
    """Write dedented `text` to `path`, creating its directory, and return the path."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(textwrap.dedent(text), encoding="utf-8")
    return path


def raises_value_error(call) -> bool:
    """Return whether calling `call` raises ValueError."""
    try:
        call()
    except ValueError:
        return True
    return False


def baseline_text(root: pathlib.Path, agent: str) -> str:
    """Return the baseline file of `agent` under `root`, or "" when there is none."""
    path = root / "tools" / "drift-baseline" / ("%s.txt" % agent)
    return path.read_text(encoding="utf-8") if path.is_file() else ""


def run_with(findings: list[Finding]):
    """Return a checker `run` that yields `findings` whatever the root."""

    def run(_root: pathlib.Path) -> list[Finding]:
        return list(findings)

    return run


ALPHA = Finding("alpha", "line-unnamed", ".alpha/x", "analyzer/doubleagent/catalog.txt", 3)
BETA = Finding("beta", "line-unnamed", ".config/beta", "analyzer/doubleagent/catalog.txt", 5)
NO_AGENT = Finding("-", "exclusion-unreachable", "tmp/x", "analyzer/doubleagent/catalog.txt")


# --- Finding ---------------------------------------------------------------------------------


def test_finding_key_collapses_whitespace_in_detail():
    finding = Finding("alpha", "credential-wording", "not  flagged\n `a b`", "research/alpha.md")
    assert finding.key("check_tables") == "check_tables|credential-wording|not flagged `a b`"


def test_finding_rejects_agent_kind_and_detail_that_cannot_form_a_line():
    cases = [
        ("Alpha", "line-unnamed", "x"),
        ("a/b", "line-unnamed", "x"),
        ("alpha", "Line_Unnamed", "x"),
        ("alpha", "line-", "x"),
        ("alpha", "line-unnamed", " \n "),
    ]
    for agent, kind, detail in cases:

        def build(agent: str = agent, kind: str = kind, detail: str = detail) -> Finding:
            return Finding(agent, kind, detail, "f")

        assert raises_value_error(build), (agent, kind, detail)
    for agent in ("-", "shared", "project", "claude-code", "open-interpreter"):
        assert Finding(agent, "line-unnamed", "x", "f").agent == agent


def test_finding_render():
    assert ALPHA.render() == ("analyzer/doubleagent/catalog.txt:3: line-unnamed [alpha]: .alpha/x")
    with_message = Finding("-", "tmp-kind", "a  b", "f.md", 0, "why")
    assert with_message.render() == "f.md: tmp-kind [-]: a b - why"


# --- Baseline ---------------------------------------------------------------------------------


def test_new_finding_fails(tmp_path, capsys):
    status = drift_common.main_with_baseline("check_tables", run_with([ALPHA]), [], tmp_path)
    assert status == 1
    assert capsys.readouterr().out.splitlines() == [ALPHA.render()]


def test_baselined_finding_passes(tmp_path, capsys):
    write(tmp_path / "tools/drift-baseline/alpha.txt", "check_tables|line-unnamed|.alpha/x\n")
    status = drift_common.main_with_baseline("check_tables", run_with([ALPHA]), [], tmp_path)
    assert status == 0
    assert capsys.readouterr().out == ""


def test_baseline_key_in_another_agents_file_does_not_match(tmp_path):
    write(tmp_path / "tools/drift-baseline/beta.txt", "check_tables|line-unnamed|.alpha/x\n")
    assert drift_common.main_with_baseline("check_tables", run_with([ALPHA]), [], tmp_path) == 1


def test_stale_line_fails_with_remove_message(tmp_path, capsys):
    write(
        tmp_path / "tools/drift-baseline/alpha.txt",
        "check_tables|line-unnamed|.alpha/gone\ncheck_tables|line-unnamed|.alpha/x\n",
    )
    status = drift_common.main_with_baseline("check_tables", run_with([ALPHA]), [], tmp_path)
    assert status == 1
    assert capsys.readouterr().out.splitlines() == [
        'tools/drift-baseline/alpha.txt:1: stale: remove "check_tables|line-unnamed|.alpha/gone"'
        " from tools/drift-baseline/alpha.txt"
    ]


def test_other_checkers_lines_are_neither_matched_nor_stale(tmp_path, capsys):
    write(tmp_path / "tools/drift-baseline/alpha.txt", "check_smoke|fixture-missing|.alpha\n")
    assert drift_common.main_with_baseline("check_tables", run_with([]), [], tmp_path) == 0
    assert drift_common.main_with_baseline("check_tables", run_with([ALPHA]), [], tmp_path) == 1
    assert "stale" not in capsys.readouterr().out


def test_malformed_line_fails(tmp_path, capsys):
    write(tmp_path / "tools/drift-baseline/alpha.txt", "\nno fields here\n")
    assert drift_common.main_with_baseline("check_tables", run_with([]), [], tmp_path) == 1
    assert capsys.readouterr().out.splitlines() == [
        "tools/drift-baseline/alpha.txt:2: malformed baseline line: no fields here"
    ]


def test_unsorted_line_fails_for_every_checker(tmp_path, capsys):
    lines = "check_tables|line-unnamed|.alpha/x\ncheck_smoke|fixture-missing|.alpha\n"
    write(tmp_path / "tools/drift-baseline/alpha.txt", lines)
    assert drift_common.main_with_baseline("check_other", run_with([]), [], tmp_path) == 1
    assert capsys.readouterr().out.splitlines() == [
        "tools/drift-baseline/alpha.txt:2: unsorted: check_smoke|fixture-missing|.alpha"
        " sorts before line 1"
    ]


def test_duplicate_line_fails(tmp_path, capsys):
    key = "check_tables|line-unnamed|.alpha/x"
    write(tmp_path / "tools/drift-baseline/alpha.txt", "%s\n%s\n" % (key, key))
    assert drift_common.main_with_baseline("check_tables", run_with([ALPHA]), [], tmp_path) == 1
    assert capsys.readouterr().out.splitlines() == [
        "tools/drift-baseline/alpha.txt:2: duplicate: %s repeats line 1" % key
    ]


def test_update_sorts_and_deduplicates_every_line(tmp_path):
    key = "check_tables|line-unnamed|.alpha/x"
    other = "check_smoke|fixture-missing|.alpha"
    write(tmp_path / "tools/drift-baseline/alpha.txt", "%s\n%s\n%s\n" % (key, other, other))
    arguments = ["--update-baseline"]
    drift_common.main_with_baseline("check_tables", run_with([ALPHA]), arguments, tmp_path)
    assert baseline_text(tmp_path, "alpha") == "%s\n%s\n" % (other, key)
    assert drift_common.main_with_baseline("check_tables", run_with([ALPHA]), [], tmp_path) == 0


def test_update_writes_sorted_keys_per_agent_and_unattributed_file_for_no_agent(tmp_path):
    later = Finding("alpha", "credential-unflagged", ".alpha/auth.json", "research/alpha.md")
    findings = [ALPHA, later, ALPHA, NO_AGENT, BETA]
    arguments = ["--update-baseline"]
    assert (
        drift_common.main_with_baseline("check_tables", run_with(findings), arguments, tmp_path)
        == 0
    )
    assert baseline_text(tmp_path, "alpha") == (
        "check_tables|credential-unflagged|.alpha/auth.json\ncheck_tables|line-unnamed|.alpha/x\n"
    )
    assert baseline_text(tmp_path, "beta") == "check_tables|line-unnamed|.config/beta\n"
    assert baseline_text(tmp_path, "unattributed") == "check_tables|exclusion-unreachable|tmp/x\n"
    assert not (tmp_path / "tools/drift-baseline/-.txt").exists()
    assert drift_common.main_with_baseline("check_tables", run_with(findings), [], tmp_path) == 0


def test_update_removes_stale_line_and_keeps_other_checkers_lines(tmp_path):
    write(
        tmp_path / "tools/drift-baseline/alpha.txt",
        "check_smoke|fixture-missing|.alpha\ncheck_tables|line-unnamed|.alpha/gone\n",
    )
    drift_common.main_with_baseline(
        "check_tables", run_with([ALPHA]), ["--update-baseline"], tmp_path
    )
    assert baseline_text(tmp_path, "alpha") == (
        "check_smoke|fixture-missing|.alpha\ncheck_tables|line-unnamed|.alpha/x\n"
    )
    assert drift_common.main_with_baseline("check_tables", run_with([ALPHA]), [], tmp_path) == 0


def test_update_drops_a_malformed_line_and_names_it(tmp_path, capsys):
    write(
        tmp_path / "tools/drift-baseline/alpha.txt",
        "check_smoke|fixture-missing|.alpha\nno fields here\n",
    )
    write(tmp_path / "tools/drift-baseline/beta.txt", "\nalso|malformed\n")
    drift_common.main_with_baseline(
        "check_tables", run_with([ALPHA]), ["--update-baseline"], tmp_path
    )
    assert capsys.readouterr().err.splitlines()[:2] == [
        "tools/drift-baseline/alpha.txt:2: dropped malformed baseline line: no fields here",
        "tools/drift-baseline/beta.txt:2: dropped malformed baseline line: also|malformed",
    ]
    assert baseline_text(tmp_path, "alpha") == (
        "check_smoke|fixture-missing|.alpha\ncheck_tables|line-unnamed|.alpha/x\n"
    )
    assert not (tmp_path / "tools/drift-baseline/beta.txt").exists()
    assert drift_common.main_with_baseline("check_tables", run_with([ALPHA]), [], tmp_path) == 0


def test_update_deletes_emptied_file_and_keeps_shared_one(tmp_path):
    write(tmp_path / "tools/drift-baseline/beta.txt", "check_tables|line-unnamed|.config/beta\n")
    write(tmp_path / "tools/drift-baseline/unattributed.txt", "check_smoke|check-unparsed|x\n")
    drift_common.main_with_baseline("check_tables", run_with([]), ["--update-baseline"], tmp_path)
    assert not (tmp_path / "tools/drift-baseline/beta.txt").exists()
    assert baseline_text(tmp_path, "unattributed") == "check_smoke|check-unparsed|x\n"


def test_unattributed_file_holds_the_findings_of_no_agent(tmp_path, capsys):
    write(tmp_path / "tools/drift-baseline/unattributed.txt", "check_tables|kind-x|gone\n")
    run = run_with([NO_AGENT])
    assert drift_common.main_with_baseline("check_tables", run, [], tmp_path) == 1
    output = capsys.readouterr().out
    assert "exclusion-unreachable [-]: tmp/x" in output
    assert "from tools/drift-baseline/unattributed.txt" in output
    assert drift_common.baseline_file("-") == "tools/drift-baseline/unattributed.txt"
    assert drift_common.file_agent("unattributed") == "-"
    assert drift_common.file_agent("alpha") == "alpha"


def test_update_creates_no_directory_without_findings(tmp_path):
    drift_common.main_with_baseline("check_tables", run_with([]), ["--update-baseline"], tmp_path)
    assert not (tmp_path / "tools" / "drift-baseline").exists()


def test_checker_name_must_be_check_name(tmp_path):
    for name in ("tables", "check-tables", "Check_tables"):

        def call(name: str = name) -> int:
            return drift_common.main_with_baseline(name, run_with([]), [], tmp_path)

        assert raises_value_error(call), name


def test_example_checker_form(tmp_path, capsys):
    write(tmp_path / "research/alpha.md", "# alpha\n\nIntro.\n")
    write(tmp_path / "research/beta.md", "Intro first.\n\n# beta\n")
    write(tmp_path / "research/README.md", "# Research\n")
    assert example_checker.main([], tmp_path) == 1
    assert capsys.readouterr().out.splitlines() == [
        "research/beta.md:1: title-mismatch [beta]: beta.md"
    ]
    assert example_checker.main(["--update-baseline"], tmp_path) == 0
    assert baseline_text(tmp_path, "beta") == "check_example|title-mismatch|beta.md\n"
    write(tmp_path / "research/beta.md", "# beta\n")
    assert example_checker.main([], tmp_path) == 1
    assert 'stale: remove "check_example|title-mismatch|beta.md"' in capsys.readouterr().out
    assert example_checker.main(["--update-baseline"], tmp_path) == 0
    assert not (tmp_path / "tools/drift-baseline/beta.txt").exists()


# --- Research documents -------------------------------------------------------------------------

DOCUMENT = """\
# alpha

Intro with `intro-span`.

## Collection

### Credentials

The file `.alpha/auth.json` and a span wrapped `across
two lines`.

```json
{"`not-a-span`": 1}
## not a heading
```

#### macOS

`mac-one`

#### macOS

`mac-two`

## Parsing

~~~~text
inner
~~~
still inner
~~~~

### Parser plan

#### Fixtures

`fixture-span`
"""


def test_research_sections_keys_are_heading_paths_below_the_h1(tmp_path):
    sections = drift_common.research_sections(write(tmp_path / "alpha.md", DOCUMENT))
    assert list(sections) == [
        "",
        "Collection",
        "Collection/Credentials",
        "Collection/Credentials/macOS",
        "Collection/Credentials/macOS (2)",
        "Parsing",
        "Parsing/Parser plan",
        "Parsing/Parser plan/Fixtures",
    ]
    assert sections["Collection/Credentials/macOS (2)"].titles == (
        "Collection",
        "Credentials",
        "macOS",
    )
    assert [section.level for section in sections.values()] == [1, 2, 3, 4, 4, 2, 3, 4]


def test_research_sections_line_ranges(tmp_path):
    sections = drift_common.research_sections(write(tmp_path / "alpha.md", DOCUMENT))
    assert (sections[""].start, sections[""].end) == (1, 4)
    assert (sections["Collection/Credentials"].start, sections["Collection/Credentials"].end) == (
        7,
        16,
    )
    last = sections["Parsing/Parser plan/Fixtures"]
    assert (last.start, last.end) == (35, 38)


def test_research_sections_code_spans_outside_fences_collapsed_with_lines(tmp_path):
    sections = drift_common.research_sections(write(tmp_path / "alpha.md", DOCUMENT))
    spans = sections["Collection/Credentials"].code_spans
    assert [(span.text, span.line) for span in spans] == [
        (".alpha/auth.json", 9),
        ("across two lines", 9),
    ]
    assert [span.text for span in sections[""].code_spans] == ["intro-span"]


def test_research_sections_fences_and_prose(tmp_path):
    sections = drift_common.research_sections(write(tmp_path / "alpha.md", DOCUMENT))
    credentials = sections["Collection/Credentials"]
    assert [(fence.language, fence.line) for fence in credentials.fences] == [("json", 12)]
    assert credentials.fences[0].lines == ((13, '{"`not-a-span`": 1}'), (14, "## not a heading"))
    assert credentials.prose[0] == (7, "### Credentials")
    assert all("not-a-span" not in line for _, line in credentials.prose)
    parsing = sections["Parsing"]
    assert parsing.fences[0].lines == ((28, "inner"), (29, "~~~"), (30, "still inner"))


def test_section_line_of_offset(tmp_path):
    sections = drift_common.research_sections(write(tmp_path / "alpha.md", DOCUMENT))
    credentials = sections["Collection/Credentials"]
    assert credentials.line_of(credentials.text.index("two lines")) == 10


def test_within_returns_a_part_and_its_subsections(tmp_path):
    sections = drift_common.research_sections(write(tmp_path / "alpha.md", DOCUMENT))
    assert [section.key for section in drift_common.within(sections, "Parsing")] == [
        "Parsing",
        "Parsing/Parser plan",
        "Parsing/Parser plan/Fixtures",
    ]
    assert drift_common.within(sections, "Collection", "Exclusions") == []


def test_fenced_block_left_open_runs_to_the_end():
    blocks = drift_common.fenced_blocks("a\n```sh\nb\n")
    assert [(block.language, block.line, block.lines) for block in blocks] == [
        ("sh", 2, ((3, "b"), (4, "")))
    ]


# --- Catalog tables -------------------------------------------------------------------------------


def test_tables_split_as_catalog_parse_does(tmp_path):
    found = drift_common.tables(write(tmp_path / "catalog.txt", CATALOG_TEXT))
    parsed = found.catalog
    assert [line.glob for line in found.home] == [entry.glob for entry in parsed.entries]
    assert [line.glob for line in found.project] == [entry.glob for entry in parsed.project_entries]
    assert [line.glob for line in found.exclude] == parsed.excludes
    assert [line.glob for line in found.secret] == parsed.secrets
    assert [line.glob for line in found.docker] == [entry.glob for entry in parsed.docker_volumes]
    assert [line.table for line in found.lines()] == ["home"] * 5 + ["project"] * 2 + [
        "exclude"
    ] * 2 + ["secret", "docker"]


def test_tables_agents_and_line_numbers(tmp_path):
    found = drift_common.tables(write(tmp_path / "catalog.txt", CATALOG_TEXT))
    assert [(line.agent, line.line) for line in found.home] == [
        ("alpha", 3),
        ("alpha", 4),
        ("beta", 6),
        ("beta", 7),
        ("gamma", 8),
    ]
    assert [(line.agent, line.glob, line.line) for line in found.exclude] == [
        ("", ".alpha/cache", 13),
        ("", ".config/beta/[Cc]ache", 15),
    ]
    assert (found.docker[0].agent, found.docker[0].glob) == ("gamma", "*gamma_usr")


def test_table_line_comment_is_the_last_block_above_it_in_its_table(tmp_path):
    found = drift_common.tables(write(tmp_path / "catalog.txt", CATALOG_TEXT))
    comments = [line.comment for line in found.home]
    assert comments == [("Alpha vendor",)] * 2 + [("Beta tool keeps two bases",)] * 3
    assert [line.comment for line in found.project] == [(), ()]
    assert [line.comment for line in found.exclude] == [(), ("Beta caches",)]
    assert found.docker[0].comment == ("Gamma's volume, mounted at /a0/usr",)


def test_table_rejects_an_unknown_name(tmp_path):
    found = drift_common.tables(write(tmp_path / "catalog.txt", CATALOG_TEXT))
    try:
        found.table("homes")
    except KeyError:
        return
    raise AssertionError("an unknown table name was accepted")


def test_sample_path_forms():
    cases = {
        ".alpha/*.json": ".alpha/x.json",
        ".alpha-?/auth": ".alpha-x/auth",
        ".config/beta/[Cc]ache": ".config/beta/Cache",
        "hermes-agent/[!.]*": "hermes-agent/xx",
        "dir/[!x]y": "dir/ay",
        "dir/[a-z]": "dir/a",
        "dir/[unclosed": "dir/[unclosed",
        "plain/path": "plain/path",
    }
    for glob, sample in cases.items():
        assert drift_common.sample_path(glob) == sample, glob


def test_sample_path_empty_stars_forms():
    cases = {
        ".openclaw*/dev": ".openclaw/dev",
        ".alpha/*.json": ".alpha/.json",
        ".openclaw*/agents/*/tmp": ".openclaw/agents/x/tmp",
        "*/User": "x/User",
        "*": "x",
        ".alpha-?/auth": ".alpha-x/auth",
        "hermes-agent/[!.]*": "hermes-agent/x",
    }
    for glob, sample in cases.items():
        assert drift_common.sample_path(glob, empty_stars=True) == sample, glob


def test_attribute_glob_most_specific_home_entry_or_dash(tmp_path):
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", CATALOG_TEXT)).catalog
    cases = {
        ".alpha/auth.json": "alpha",
        ".alpha.json*": "alpha",
        ".config/beta/[Cc]ache": "beta",
        ".config/beta/sub/x": "beta",
        ".gamma/nested/*": "gamma",
        ".gamma/other": "-",
        "*/User/globalStorage": "-",
        "tmp/playwright": "-",
    }
    for glob, agent in cases.items():
        assert drift_common.attribute_glob(glob, catalog, tmp_path) == agent, glob


def test_attribute_glob_project_entry_before_star_and_volume_steps(tmp_path):
    text = "# home artifacts (agent|glob relative to home)\nalpha|.alpha\n"
    text += "# project artifacts (relative to each discovered project)\nproject|.git/worktrees\n"
    text += "alpha|.alpha\n# docker volumes (agent|volume name glob)\ngamma|*gamma_data\n"
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", text)).catalog
    write(tmp_path / "research/gamma.md", "# gamma\n\n## Collection\n\n`.git/worktrees/x/index`\n")
    cases = {
        ".git/worktrees/*/index": "project",
        ".alpha/cache": "alpha",
        "tmp/playwright": "-",
    }
    for glob, agent in cases.items():
        assert drift_common.attribute_glob(glob, catalog, tmp_path) == agent, glob


def test_attribute_glob_star_glob_claimed_by_project_before_reached_home_entry(tmp_path):
    text = "# home artifacts (agent|glob relative to home)\nhomey|.config/.projx\n"
    text += "homey|.config/.homey\nhomez|*/.both\n"
    text += "# project artifacts (relative to each discovered project)\n"
    text += "projx|*/.projx\nprojz|*/.both\n"
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", text)).catalog
    assert drift_common.star_remainders("*/.projx/cache", catalog) == [("homey", "cache")]
    cases = {
        "*/.projx/cache": "projx",
        "*/.homey/cache": "homey",
        "*/.both/cache": "homez",
    }
    for glob, agent in cases.items():
        assert drift_common.attribute_glob(glob, catalog, tmp_path) == agent, glob
    assert drift_common.project_claim("*/.homey/cache", catalog) == ""


def test_attribute_glob_tries_empty_stars_before_x(tmp_path):
    text = "delta|.delta\ndelta|.delta-*\nepsilon|.delta/eps\nzeta|.zeta-x\n"
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", text)).catalog
    cases = {
        ".delta*/bin": "delta",
        ".delta/eps*/bin": "epsilon",
        ".zeta-*/bin": "zeta",
        ".delta-*": "delta",
    }
    for glob, agent in cases.items():
        assert drift_common.attribute_glob(glob, catalog, tmp_path) == agent, glob


STAR_CATALOG = """\
# home artifacts (agent|glob relative to home)
editor|.config/Code/User
editor|Library/Application Support/Code/User
other|.config/Other/User
plugin|.config/*/User/globalStorage/vendor.plugin
desk|.config/Desk
cli|Applications/Desk Helper.app
# docker volumes (agent|volume name glob)
gamma|*gamma_usr
delta|*delta_data
"""


def test_attribute_glob_star_glob_goes_to_the_closest_entry_or_dash(tmp_path):
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", STAR_CATALOG)).catalog
    cases = {
        "*/User/globalStorage/vendor.plugin/cache": "plugin",
        "*/Code/User/globalStorage/x": "editor",
        "*/User/globalStorage/ms-*": "-",
        "*/Desk*/Cache": "-",
        "*/Nowhere/Cache": "-",
    }
    for glob, agent in cases.items():
        assert drift_common.attribute_glob(glob, catalog, tmp_path) == agent, glob


def test_star_remainders_only_the_leading_star_crosses_a_slash(tmp_path):
    text = "# home artifacts (agent|glob relative to home)\nalpha|.e/User/x/*/plugin\n"
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", text)).catalog
    assert drift_common.star_remainders("*/User/x/*/plugin/y", catalog) == [("alpha", "y")]
    assert drift_common.star_remainders("*/User/x/*/y", catalog) == []
    assert drift_common.star_remainders(".e/User/x", catalog) == []


def test_closest_agent_forms():
    assert drift_common.closest_agent([("a", "x/y"), ("b", "y")]) == "b"
    assert drift_common.closest_agent([("a", ""), ("b", "y")]) == "a"
    assert drift_common.closest_agent([("a", "y"), ("a", "z"), ("b", "x/y")]) == "a"
    assert drift_common.closest_agent([("a", "y"), ("b", "z")]) == "-"
    assert drift_common.closest_agent([]) == "-"


def test_attribute_glob_volume_glob_goes_to_the_one_volume_document_naming_it(tmp_path):
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", STAR_CATALOG)).catalog
    gamma = "# gamma\n\n## Collection\n\n`tmp/playwright` and `/a0/usr/models` and `both`.\n"
    delta = "# delta\n\n## Collection\n\n`both` and `cache`.\n\n## Parsing\n\n`sessions`\n"
    write(tmp_path / "research" / "gamma.md", gamma)
    write(tmp_path / "research" / "delta.md", delta)
    cases = {
        "tmp/playwright": "gamma",
        "cache/*": "delta",
        "both": "-",
        "sessions": "-",
        "models": "-",
        "unnamed/dir": "-",
    }
    for glob, agent in cases.items():
        assert drift_common.attribute_glob(glob, catalog, tmp_path) == agent, glob


def test_glob_owners_volume_subsections_before_the_collection_part(tmp_path):
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", STAR_CATALOG)).catalog
    volume = (
        "### Per-user storage\n\n#### Docker volume\n\nThe `%s` volume holds `both` and `.ssh/*`.\n"
    )
    gamma = "# gamma\n\n## Collection\n\n" + volume % "a0_gamma_usr"
    delta = "# delta\n\n## Collection\n\n### Per-user storage\n\n`both`, `~/.ssh/` and `solo`.\n"
    write(tmp_path / "research" / "gamma.md", gamma)
    write(tmp_path / "research" / "delta.md", delta)
    cases = {
        ".ssh/*": ["gamma"],
        "both": ["gamma"],
        "solo": ["delta"],
        "unnamed/dir": [],
        "*/Code/User/globalStorage/x": ["editor"],
        "*/User/globalStorage/ms-*": [],
    }
    for glob, owners in cases.items():
        assert drift_common.glob_owners(glob, catalog, tmp_path) == owners, glob
    unnamed = "# delta\n\n## Collection\n\n" + volume.replace("`%s` ", "")
    write(tmp_path / "research" / "delta.md", unnamed)
    drift_common.collection_spans_by_depth.cache_clear()
    assert drift_common.glob_owners("both", catalog, tmp_path) == ["gamma"]
    write(
        tmp_path / "research" / "delta.md", "# delta\n\n## Collection\n\n" + volume % "delta_data"
    )
    drift_common.collection_spans_by_depth.cache_clear()
    assert drift_common.glob_owners("both", catalog, tmp_path) == ["delta", "gamma"]


def test_project_claim_forms(tmp_path):
    text = "# project artifacts (relative to each discovered project)\nproject|.git/worktrees\n"
    text += "pi|.pi\npi|.pi/approved-plan.md\n"
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", text)).catalog
    assert drift_common.project_claim(".git/worktrees/*/index", catalog) == "project"
    assert drift_common.project_claim(".pi*/x", catalog) == "pi"
    assert drift_common.project_claim("tmp/playwright", catalog) == ""


def test_home_claim_forms(tmp_path):
    catalog = drift_common.tables(write(tmp_path / "catalog.txt", CATALOG_TEXT)).catalog
    assert drift_common.home_claim(".config/beta/sub/x", catalog) == "beta"
    assert drift_common.home_claim(".alpha*/x", catalog) == "alpha"
    assert drift_common.home_claim("tmp/playwright", catalog) == ""


def test_document_names_glob_reads_only_the_collection_part(tmp_path):
    document = write(tmp_path / "doc.md", "# a\n\n## Collection\n\n`~/x/y`\n\n## Parsing\n\n`z`\n")
    assert drift_common.document_names_glob(document, "x/y")
    assert drift_common.document_names_glob(document, "x/y/*")
    assert not drift_common.document_names_glob(document, "z")
    assert not drift_common.document_names_glob(tmp_path / "missing.md", "x/y")


# --- Agents ---------------------------------------------------------------------------------

AGENTS_PAGE = """\
# Agents

| Not | The table |
| --- | --- |

| Agent | Status | Validated against | Notes |
| --- | --- | --- | --- |
| [`alpha`](#alpha) | parsed | source; fixture only | [research](../research/alpha.md) |
| beta | backlog | Source | lines `beta|.config/beta` and more |
| [`alpha`](#alpha) | collected | twice | ignored |

| Agent | Status | Validated against | Notes |
| --- | --- | --- | --- |
| [`later`](#later) | parsed | x | y |
"""


def test_agents_page_rows(tmp_path):
    rows = drift_common.agents_page_rows(write(tmp_path / "agents.md", AGENTS_PAGE))
    assert list(rows) == ["alpha", "beta"]
    assert rows["alpha"] == drift_common.AgentRow(
        "alpha", "parsed", "source; fixture only", "[research](../research/alpha.md)", 8
    )
    assert rows["beta"].notes == "lines `beta|.config/beta` and more"


PARSERS = {
    "__init__.py": """\
        from .alpha import AlphaParser
        from .base import Parser
        from .beta import BetaParser, BetaChildParser

        ALL: list[Parser] = [AlphaParser(), BetaParser(), BetaChildParser()]
        """,
    "base.py": """\
        class Parser:
            agent: str = ""
            name: str = ""
            reads_agents: tuple[str, ...] = ()
            records_sync: bool = False
            records_relay: bool = False
            records_origin: bool = False
            has_secondary: bool = False
        """,
    "alpha.py": """\
        from ..editors import EDITOR_AGENTS
        from . import beta
        from .base import Parser


        class AlphaParser(Parser):
            agent = "alpha"
            reads_agents = EDITOR_AGENTS
            has_secondary = True
            records_relay = compute()
        """,
    "beta.py": """\
        from .base import Parser


        class BetaParser(Parser):
            agent = "beta"
            records_sync = True

            def __init__(self) -> None:
                self.records_origin = self.agent == "beta"
                self.records_relay = not self.records_sync


        class BetaChildParser(BetaParser):
            agent = "beta-child"
            reads_agents = ("beta",)
        """,
}


def write_repository(root: pathlib.Path) -> None:
    """Write a synthetic repository with a catalog, documents, an agents page and parsers."""
    write(root / "analyzer/doubleagent/catalog.txt", CATALOG_TEXT)
    write(root / "analyzer/doubleagent/editors.py", 'EDITOR_AGENTS = ("vscode", "cursor")\n')
    for name, text in PARSERS.items():
        write(root / "analyzer/doubleagent/parsers" / name, text)
    for name in ("alpha.md", "beta.md", "README.md", "frameworks.md", "shared.md"):
        write(root / "research" / name, "# %s\n" % name[:-3])
    write(root / "docs/agents.md", AGENTS_PAGE)


def test_agents_reads_every_source(tmp_path):
    write_repository(tmp_path)
    found = drift_common.agents(tmp_path)
    assert list(found.catalog) == ["alpha", "beta", "gamma", "project"]
    assert [line.table for line in found.catalog["alpha"]] == ["home", "home", "project"]
    assert [line.table for line in found.catalog["gamma"]] == ["home", "docker"]
    assert list(found.documents) == ["alpha", "beta"]
    assert list(found.rows) == ["alpha", "beta"]
    assert list(found.parsers) == ["alpha", "beta", "beta-child"]


def test_parser_classes_resolve_inherited_literals_and_names(tmp_path):
    write_repository(tmp_path)
    alpha = drift_common.agents(tmp_path).parsers["alpha"][0]
    assert alpha.class_name == "AlphaParser"
    assert alpha.reads_agents == ("vscode", "cursor")
    assert (alpha.has_secondary, alpha.records_sync, alpha.records_origin) == (True, False, False)
    assert alpha.bases == ("Parser",)
    parsers = tmp_path / "analyzer/doubleagent/parsers"
    assert alpha.modules == (parsers / "alpha.py", parsers / "base.py")
    assert alpha.imported_modules == (parsers / "beta.py", parsers / "base.py")


def test_parser_classes_record_an_unresolved_attribute_with_its_inherited_value(tmp_path):
    write_repository(tmp_path)
    alpha = drift_common.agents(tmp_path).parsers["alpha"][0]
    assert alpha.unresolved == ("records_relay",)
    assert alpha.records_relay is False


def test_parser_classes_apply_the_nearest_init_against_the_subclass(tmp_path):
    write_repository(tmp_path)
    parsers = drift_common.agents(tmp_path).parsers
    beta = parsers["beta"][0]
    child = parsers["beta-child"][0]
    assert (beta.records_origin, beta.records_relay, beta.records_sync) == (True, False, True)
    assert (child.records_origin, child.records_sync) == (False, True)
    assert child.reads_agents == ("beta",)
    assert child.bases == ("BetaParser", "Parser")
    assert child.line == 13
