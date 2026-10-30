"""Tests for the convention rules and options of tools/check_research_structure.py."""

from __future__ import annotations

import pathlib

import check_research_structure as checker

FOOTER = """\
### Source reviews

None.

## Collection

### Per-user storage

{storage}

#### Scratch directories

None known.

### Credentials

{credentials}

### Exclusions

None.

### Project-local files

None.

### Where the project path is recorded

None.

## Parsing

### Transcript stores

None.

#### What survives a delete

None.

### Record schema

{schema}

### Joins and timestamps

None.

### Other stores

None.

### Format versions

{formats}

### Secrets

None.

### Parser plan

None.

#### Permission rows

The agent records none.

#### Model rows

The agent records none.

#### Past-use rows and secondary sources

The agent records none.

#### Sync, relay and origin rows

The agent records none.

#### Scratch directory rows

The agent records none.

#### Deletion rows

The agent records none.

#### Bad-line rows

The agent records none.

#### Fixtures

None.

## Confidence

- High.
"""

GOOD_OBSERVED = "- **macOS CLI**: build 1.2.3, 2026-10-01, one listing per step."


def document(
    observed: str = GOOD_OBSERVED,
    storage: str = "The home holds `.alpha`.",
    credentials: str = "None.",
    schema: str = "None.",
    formats: str = "None.",
    intro: str = "Alpha is a test agent.",
) -> str:
    """Return a research document for agent `alpha` that passes every hard rule."""
    head = "# alpha\n\n%s\n\n## Source and evidence\n\nSource.\n\n### Observed\n\n%s\n\n" % (
        intro,
        observed,
    )
    return head + FOOTER.format(
        storage=storage, credentials=credentials, schema=schema, formats=formats
    )


def write_document(root: pathlib.Path, text: str, name: str = "alpha.md") -> pathlib.Path:
    """Write a research document under `root/research` and return its path."""
    path = root / "research" / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")
    return path


def run(root: pathlib.Path, capsys, *arguments: str) -> tuple[int, str]:
    """Run the checker on `root` and return its exit status and its stdout."""
    status = checker.main(list(arguments), root)
    return status, capsys.readouterr().out


def kinds(output: str) -> list[str]:
    """Return the finding kind of each finding line of the checker's output."""
    return [line.split(": ")[1].split(" ")[0] for line in output.splitlines() if " [" in line]


def baseline(root: pathlib.Path, agent: str = "alpha") -> pathlib.Path:
    """Return the baseline file of `agent` under `root`."""
    return root / "tools" / "drift-baseline" / ("%s.txt" % agent)


# --- A clean document and the hard rules -------------------------------------------------------


def test_clean_document_passes(tmp_path, capsys):
    write_document(tmp_path, document())
    status, output = run(tmp_path, capsys)
    assert status == 0, output
    assert kinds(output) == []


def test_hard_rule_still_fails_and_is_never_baselined(tmp_path, capsys):
    long_line = "word " * 30
    write_document(tmp_path, document(intro=long_line.strip()))
    status, output = run(tmp_path, capsys)
    assert status == 1
    assert "prose line longer than 100 characters" in output
    status, output = run(tmp_path, capsys, "--update-baseline")
    assert status == 1
    assert "prose line longer than 100 characters" in output
    assert not (tmp_path / "tools" / "drift-baseline").exists()


# --- The rules ---------------------------------------------------------------------------------


def test_observed_label_missing(tmp_path, capsys):
    write_document(tmp_path, document(observed="- macOS, CLI 1.2.3, 2026-10-01."))
    status, output = run(tmp_path, capsys)
    assert status == 1
    assert kinds(output) == ["observed-label-missing"]
    assert "observed-label-missing [alpha]: macOS, CLI 1.2.3, 2026-10-01." in output


def test_observed_label_missing_exempts_no_real_install(tmp_path, capsys):
    for text in ("- No real install.", "- No real install: nothing was run."):
        write_document(tmp_path, document(observed=text))
        status, output = run(tmp_path, capsys)
        assert (status, kinds(output)) == (0, []), text


def test_observed_cell_duplicate(tmp_path, capsys):
    observed = GOOD_OBSERVED + "\n- **macOS CLI**: build 1.2.4, 2026-10-02."
    write_document(tmp_path, document(observed=observed))
    status, output = run(tmp_path, capsys)
    assert status == 1
    assert kinds(output) == ["observed-cell-duplicate"]
    assert "observed-cell-duplicate [alpha]: macOS CLI - the cell of line" in output


def test_observed_cells_with_distinct_labels_pass(tmp_path, capsys):
    observed = GOOD_OBSERVED + "\n- **Windows CLI**: build 1.2.4, 2026-10-02."
    write_document(tmp_path, document(observed=observed))
    status, output = run(tmp_path, capsys)
    assert (status, kinds(output)) == (0, []), output


def test_latest_version_missing(tmp_path, capsys):
    observed = "- **macOS CLI**: 2026-10-01 ([issue 5](https://example.com/issues/5.1)). " + (
        "Earlier builds observed: 1.2.2 (2026-09-01)."
    )
    write_document(tmp_path, document(observed=observed))
    status, output = run(tmp_path, capsys)
    assert status == 1
    assert kinds(output) == ["latest-version-missing"]
    assert "latest-version-missing [alpha]: macOS CLI" in output


def test_latest_version_present_passes(tmp_path, capsys):
    observed = GOOD_OBSERVED + " Earlier builds observed: 1.2.2 (2026-09-01)."
    write_document(tmp_path, document(observed=observed))
    status, output = run(tmp_path, capsys)
    assert (status, kinds(output)) == (0, []), output


def test_earlier_builds_malformed(tmp_path, capsys):
    cases = (
        GOOD_OBSERVED + " Earlier builds observed: 1.2.2 (September 2026).",
        GOOD_OBSERVED + " Earlier builds observed: 1.2.2 (2026-09-01). Then a listing.",
    )
    for observed in cases:
        write_document(tmp_path, document(observed=observed))
        status, output = run(tmp_path, capsys)
        assert status == 1, observed
        assert kinds(output) == ["earlier-builds-malformed"], observed
        assert "earlier-builds-malformed [alpha]: macOS CLI - " in output, observed


def test_earlier_builds_placed_forms():
    accepted = (
        "**macOS CLI**: 1.2.3. Earlier builds observed: 1.2.2 (2026-09-01).",
        "**macOS CLI**: 1.2.3. Earlier builds observed: 1.2.2 (2026-09-01), 1.2.1\n(2026-08-01).",
        "**macOS CLI**: 1.2.3, no earlier build.",
    )
    for bullet in accepted:
        assert checker.earlier_builds_placed(bullet), bullet
    rejected = (
        "**macOS CLI**: 1.2.3. Earlier builds observed: 1.2.2.",
        "**macOS CLI**: Earlier builds observed: 1.2.2 (2026-09-01). Then 1.2.3.",
        "**macOS CLI**: 1.2.3. Earlier builds observed: 1.2.2 (2026-09-01)",
    )
    for bullet in rejected:
        assert not checker.earlier_builds_placed(bullet), bullet


def test_version_without_change(tmp_path, capsys):
    write_document(tmp_path, document(schema="The `id` key is written since 2.1.309."))
    status, output = run(tmp_path, capsys)
    assert status == 1
    assert kinds(output) == ["version-without-change"]
    detail = "version-without-change [alpha]: The id key is written since 2.1.309. - "
    assert detail in output


def test_version_without_change_detail_is_each_sentence(tmp_path, capsys):
    schema = "The `id` key is written since 2.1.309. The `name` key is too, since 2.1.309."
    write_document(tmp_path, document(schema=schema))
    findings = checker.convention_findings(
        tmp_path / "research/alpha.md",
        (tmp_path / "research/alpha.md").read_text(encoding="utf-8"),
        checker.load_link_check(),
        tmp_path,
    )
    assert [finding.detail for finding in findings] == [
        "The id key is written since 2.1.309.",
        "The name key is too, since 2.1.309.",
    ]


def test_version_with_format_entry_link_or_range_passes(tmp_path, capsys):
    cases = (
        {"schema": "The `id` key is written since 2.1.309.", "formats": "- 2.1.309 adds `id`."},
        {"schema": "The `id` key is written since 2.1.309\n([notes](https://example.com))."},
        {"schema": "The `id` key changed between 2.1.296 and 2.1.309."},
        {"schema": "Collected since collector 1.5.0."},
    )
    for case in cases:
        write_document(tmp_path, document(**case))
        status, output = run(tmp_path, capsys)
        assert (status, kinds(output)) == (0, []), case


def test_version_phrase_in_format_versions_or_source_and_evidence_passes(tmp_path, capsys):
    write_document(
        tmp_path,
        document(observed=GOOD_OBSERVED + " Written since 1.2.0.", formats="Since 1.1.0 only."),
    )
    status, output = run(tmp_path, capsys)
    assert (status, kinds(output)) == (0, []), output


def test_duplicate_fact(tmp_path, capsys):
    fact = "The file `auth.json` holds the access token."
    write_document(tmp_path, document(storage=fact, credentials=fact))
    status, output = run(tmp_path, capsys)
    assert status == 1
    assert kinds(output) == ["duplicate-fact"]
    detail = "duplicate-fact [alpha]: The file auth.json holds the access token. - `auth.json`"
    assert detail in output


def test_duplicate_fact_absent_for_other_span_section_part_or_short_key(tmp_path, capsys):
    fact = "The file `auth.json` holds the access token."
    cases = (
        {"storage": fact, "credentials": "The file `token.json` holds the access token."},
        {"storage": fact, "schema": fact},
        {"storage": fact + " " + fact},
        {"storage": "Holds `a`.", "credentials": "Holds `a`."},
    )
    for case in cases:
        write_document(tmp_path, document(**case))
        status, output = run(tmp_path, capsys)
        assert (status, kinds(output)) == (0, []), case


def test_bare_doc_reference(tmp_path, capsys):
    write_document(tmp_path, document(storage="The vendor documentation names `.alpha`."))
    status, output = run(tmp_path, capsys)
    assert status == 1
    assert kinds(output) == ["bare-doc-reference"]
    assert (
        'bare-doc-reference [alpha]: The vendor documentation names .alpha. - names "documentation"'
        in (output)
    )


def test_linked_doc_reference_passes(tmp_path, capsys):
    storage = "The [vendor documentation](https://example.com/docs#paths) names `.alpha`."
    write_document(tmp_path, document(storage=storage))
    status, output = run(tmp_path, capsys)
    assert (status, kinds(output)) == (0, []), output


# --- The baseline ------------------------------------------------------------------------------


def test_baselined_finding_passes_and_stale_line_fails(tmp_path, capsys):
    write_document(tmp_path, document(observed="- macOS, CLI 1.2.3."))
    assert run(tmp_path, capsys, "--update-baseline")[0] == 0
    key = "check_research_structure|observed-label-missing|macOS, CLI 1.2.3."
    assert baseline(tmp_path).read_text(encoding="utf-8") == key + "\n"
    assert run(tmp_path, capsys)[0] == 0
    write_document(tmp_path, document())
    status, output = run(tmp_path, capsys)
    assert status == 1
    assert 'stale: remove "%s" from tools/drift-baseline/alpha.txt' % key in output


def test_update_baseline_removes_stale_line_and_keeps_other_checkers(tmp_path, capsys):
    write_document(tmp_path, document())
    path = baseline(tmp_path)
    path.parent.mkdir(parents=True)
    stale = "check_research_structure|observed-label-missing|macOS, CLI 1.2.3."
    other = "check_tables_research|line-unnamed|.alpha"
    path.write_text("%s\n%s\n" % (stale, other), encoding="utf-8")
    assert run(tmp_path, capsys)[0] == 1
    assert run(tmp_path, capsys, "--update-baseline")[0] == 0
    assert path.read_text(encoding="utf-8") == other + "\n"
    assert run(tmp_path, capsys)[0] == 0


def test_file_argument_leaves_other_documents_baseline_lines_alone(tmp_path, capsys):
    write_document(tmp_path, document())
    beta = write_document(tmp_path, document().replace("# alpha", "# beta"), "beta.md")
    path = baseline(tmp_path)
    path.parent.mkdir(parents=True)
    path.write_text("check_research_structure|observed-label-missing|macOS x\n", encoding="utf-8")
    assert run(tmp_path, capsys, str(beta))[0] == 0
    assert run(tmp_path, capsys)[0] == 1


# --- --before ----------------------------------------------------------------------------------


def test_before_reports_a_lost_observed_label(tmp_path, capsys):
    earlier = document(observed=GOOD_OBSERVED + "\n- **Windows CLI**: build 1.2.3.")
    write_document(tmp_path / "old", earlier)
    write_document(tmp_path, document())
    status, output = run(tmp_path, capsys, "--before", str(tmp_path / "old" / "research"))
    assert status == 1
    assert "research/alpha.md: lost observed label: Windows CLI" in output
    assert "lost observed label: macOS CLI" not in output


def test_before_reports_a_lost_key_table_row(tmp_path, capsys):
    header = "| Key | Type | Values | Since |\n| --- | --- | --- | --- |\n"
    earlier = header + "| `id` | string | | 1.0 |\n| `name` | string | | 1.0 |"
    old = tmp_path / "old" / "collectors" / "alpha.md"
    old.parent.mkdir(parents=True)
    old.write_text(document(schema=earlier), encoding="utf-8")
    write_document(tmp_path, document(schema=header + "| `name` | string | | 1.0 |"))
    status, output = run(tmp_path, capsys, "--before", str(tmp_path / "old"))
    assert status == 1
    assert "research/alpha.md: lost key row: `id`" in output
    assert "lost key row: `name`" not in output


# --- The heuristics ----------------------------------------------------------------------------


def test_observed_label_forms():
    accepted = {
        "**macOS CLI**: x": "macOS CLI",
        "**Windows desktop app**: x": "Windows desktop app",
        "**Linux lab container:** x": "Linux lab container",
        "**FreeBSD CLI**:": "FreeBSD CLI",
    }
    for text, label in accepted.items():
        assert checker.observed_label(text) == label, text
    rejected = (
        "**Windows, CLI** (issue 83): x",
        "**macOS CLI** x",
        "**macOS CLI:**: x",
        "**Windows desktop app**. x",
        "**macOS VS Code extension.** x",
        "**Ubuntu CLI**: x",
        "**macOS**: x",
        "macOS CLI: x",
        "- **macOS CLI**: x",
    )
    for text in rejected:
        assert checker.observed_label(text) is None, text


def test_earlier_builds_forms():
    one = "x. Earlier builds observed: 2.1.296 (2026-10-13)."
    assert checker.earlier_builds(one) == [("2.1.296", "2026-10-13")]
    two = "Earlier builds observed: CLI 2.1.296 (2026-10-13), 2.1.295\n(2026-10-11)."
    assert checker.earlier_builds(two) == [("CLI 2.1.296", "2026-10-13"), ("2.1.295", "2026-10-11")]
    rejected = (
        "Earlier builds observed: 2.1.296.",
        "Earlier builds observed: 2.1.296 (October 2026).",
        "Earlier builds observed: 2.1.296 (2026-10-13)",
        "Earlier builds: 2.1.296 (2026-10-13).",
    )
    for text in rejected:
        assert checker.earlier_builds(text) is None, text


def test_version_tokens_forms():
    assert checker.version_tokens("CLI 2.1.310, kernel 7.0") == ["2.1.310", "7.0"]
    assert checker.version_tokens("macOS 26 build 25G83 on 2026-10-23") == []
    assert checker.version_tokens("[issue](https://x.com/v1.2/5) and https://y.org/3.4") == []
    assert checker.version_tokens("x. Earlier builds observed: 1.2.2 (2026-09-01).") == []


def test_version_phrases_forms():
    accepted = {
        "written since 2.1.309.": ("since 2.1.309", "2.1.309"),
        "Added in CLI 1.3 and": ("Added in CLI 1.3", "1.3"),
        "since `v0.30.7` the": ("since v0.30.7", "0.30.7"),
        "introduced in version 4.0.1": ("introduced in version 4.0.1", "4.0.1"),
        "New in build 26.1013.40934.": ("New in build 26.1013.40934", "26.1013.40934"),
    }
    for line, phrase in accepted.items():
        assert checker.version_phrases(line) == [phrase], line
    rejected = (
        "since collector 1.5.0",
        "Since analyzer 0.17.0",
        "changed between 2.1.296 and 2.1.309",
        "since the 2.1 line",
        "since 2026-10-13",
        "added in the last two builds 2.1",
        "are unchanged since `v0.30.7`",
        "Unchanged since 4.3.5",
    )
    for line in rejected:
        assert checker.version_phrases(line) == [], line


def test_link_near_forms():
    lines = ["since 2.1", "see [notes](https://x.com)", "plain", "plain", "<https://x.com>"]
    assert checker.link_near(lines, 0)
    assert checker.link_near(lines, 1)
    assert not checker.link_near(lines, 2)
    assert checker.link_near(lines, 3)
    for text in ("[a][ref]", "https://x.com/a", "[Format versions](#format-versions)"):
        assert checker.has_link(text), text
    for text in ("[a] b", "`https` scheme", "(a)[b]"):
        assert not checker.has_link(text), text


def test_names_version_forms():
    assert checker.names_version("- 2.1.309 adds a key", "2.1.309")
    assert checker.names_version("- v0.30.7: new", "0.30.7")
    assert not checker.names_version("- 2.1.3090 adds", "2.1.309")
    assert not checker.names_version("- 12.1.309 adds", "2.1.309")
    assert not checker.names_version("- 2.1.309.1 adds", "2.1.309")


def test_sentence_key_forms():
    sentence = "- The file `a.json` holds [the key](#key) , `b`."
    assert checker.sentence_key(sentence) == "The file holds the key,."
    assert checker.sentence_key("1. Holds   `x`\n  too.") == "Holds too."
    assert checker.sentence_key(sentence, keep_code=True) == "The file a.json holds the key, b."
    assert checker.sentence_key("Rewrites `.a/b`.", keep_code=True) == "Rewrites .a/b."


def test_sentences_forms():
    paragraph = "Reads `a.b. C` here. Then [x](y.md#a. B) too! `z` ends. e.g. lower stays."
    assert checker.sentences(paragraph) == [
        "Reads `a.b. C` here.",
        "Then [x](y.md#a. B) too!",
        "`z` ends. e.g. lower stays.",
    ]


def test_documentation_reference_forms():
    accepted = {
        "The vendor documentation says": "documentation",
        "per the Docs page": "Docs page",
        "the Changelogs list": "Changelogs",
        "its release notes for": "release notes",
        "the README names": "README",
        "the kiro.dev documentation says so": "documentation",
        "the paths come from the documentation, not from source": "documentation",
        "Some official documentation was consulted.": "documentation",
    }
    for sentence, phrase in accepted.items():
        assert checker.documentation_reference(sentence) == phrase, sentence
    rejected = (
        "the file `README.md` holds",
        "a readme file",
        "documented in",
        "docs/ folder",
        "No vendor documentation names it.",
        "it is not in the changelog",
        "a build without release notes",
        "a key (binary changelog; observed)",
        '"marketplace cache ... staging directories" (str changelog).',
        "the strings changelog of the build names it",
        "The embedded changelog's newest heading is `1.2`.",
        "the bundled README lists it",
        "the collector documentation names it as a manual step",
        "the analyzer documentation and the project documentation",
        "No official OpenAI desktop app and CLI documentation was found.",
        "`.claude/cache` (the changelog and the model catalog), `.claude/x`",
        "Levels are L1 (source) and L2 (kiro.dev documentation).",
    )
    for sentence in rejected:
        assert checker.documentation_reference(sentence) is None, sentence


def test_key_table_rows_forms():
    lines = [
        "| Key | Type | Values | Builds |",
        "| --- | --- | --- | --- |",
        "| `id` |  string | | 1.0 |",
        "|  `a  b` | number | | |",
        "",
        "| Key | Type | Meaning |",
        "| --- | --- | --- |",
        "| `other` | string | x |",
        "| Key | Type | Values | Since |",
        "|---|---|---|---|",
        "| `since` | bool | | 2.0 |",
        "text",
        "| `after` | x | | |",
    ]
    assert checker.key_table_rows(lines) == ["`id`", "`a b`", "`since`"]
