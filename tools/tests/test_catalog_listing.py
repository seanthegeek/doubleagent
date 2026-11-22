"""Tests for the catalog listing reader the drift checkers share."""

from __future__ import annotations

import catalog_listing

LISTING = """\
# home artifacts (agent|glob relative to home)
alpha|.alpha
alpha-star|.alph*
# a vendor comment that is no heading
alpha-nested|.alpha/nested
beta|.config/beta*
# project artifacts (relative to each discovered project)
alpha|.alpha
pi|.pi
pi-plan|.pi/approved-plan.md
# excluded unless --full
.alpha/cache
# credential files (flagged secret, skipped with --no-secrets)
.alpha/auth.json
# docker volumes (agent|volume name glob)
gamma|*gamma_data
# relocation variables (agent|VARIABLE|home-relative path it replaces)
alpha|ALPHA_HOME|.alpha
# startup files searched for them (scope|glob; scope home, system or project)
home|.profile
"""


def test_parse_puts_each_line_in_the_table_its_heading_opens():
    catalog = catalog_listing.parse(LISTING)
    assert [(entry.agent, entry.glob) for entry in catalog.entries] == [
        ("alpha", ".alpha"),
        ("alpha-star", ".alph*"),
        ("alpha-nested", ".alpha/nested"),
        ("beta", ".config/beta*"),
    ]
    assert [entry.agent for entry in catalog.project_entries] == ["alpha", "pi", "pi-plan"]
    assert catalog.excludes == [".alpha/cache"]
    assert catalog.secrets == [".alpha/auth.json"]
    assert [(entry.agent, entry.glob) for entry in catalog.docker_volumes] == [
        ("gamma", "*gamma_data")
    ]
    assert catalog.relocation_vars == [
        catalog_listing.RelocationVariable("alpha", "ALPHA_HOME", ".alpha")
    ]
    assert catalog.startup_files == [catalog_listing.StartupFile("home", ".profile")]


def test_parse_reads_crlf_lines_and_skips_blank_ones():
    catalog = catalog_listing.parse(LISTING.replace("\n", "\r\n").replace("beta|", "\r\nbeta|"))
    globs = [entry.glob for entry in catalog.entries]
    assert globs == [".alpha", ".alph*", ".alpha/nested", ".config/beta*"]


def test_the_deepest_home_entry_claims_a_path_and_the_first_breaks_a_tie():
    catalog = catalog_listing.parse(LISTING)
    cases = [
        (".alpha/sessions/one.jsonl", "alpha"),
        (".alphabet/one.jsonl", "alpha-star"),
        (".alpha/nested/one.jsonl", "alpha-nested"),
        (".config/beta-two/x", "beta"),
        (".config/other", ""),
        (".ALPHA/nested", ""),
    ]
    for path, agent in cases:
        assert catalog.claims_home_path(path) == agent, path


def test_a_windows_path_splits_on_backslashes_and_ignores_case():
    catalog = catalog_listing.parse(LISTING)
    assert catalog.claims_home_path(".ALPHA\\Nested\\x", windows=True) == "alpha-nested"
    assert catalog.claims_home_path(".alpha/nested/x", windows=True) == ""


def test_the_deepest_project_entry_claims_a_project_path():
    catalog = catalog_listing.parse(LISTING)
    assert catalog.project_agent_for(".pi/approved-plan.md") == "pi-plan"
    assert catalog.project_agent_for(".pi\\other") == "pi"
    assert catalog.project_agent_for("AGENTS.md") == ""


def test_a_glob_segment_becomes_a_regex_that_never_crosses_a_slash():
    cases = [
        ("*.json", "[^/]*\\.json"),
        ("a?c", "a[^/]c"),
        ("[!.]*", "[^.][^/]*"),
        ("[a-c]x", "[a-c]x"),
        ("a[b", "a\\[b"),
    ]
    for segment, pattern in cases:
        assert catalog_listing.glob_segment_to_regex(segment).pattern == pattern, segment
