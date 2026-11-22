"""Tests for tools/check_doc_links.py, the term-of-art check, on synthetic Markdown."""

from __future__ import annotations

import check_doc_links as links


def term_names(text: str) -> list[str]:
    """Return the terms of art `text` uses without introducing them."""
    spans = [(match.start(1), match.end(1)) for match in links.INLINE_LINK.finditer(text)]
    return [message.split('"')[1] for _, message in links.term_problems(text, spans)]


def test_multi_word_term_split_across_lines_is_introduced_by_its_link() -> None:
    for term in ("secondary source", "nested entry", "listing diff", "Observed bullet"):
        first, second = term.split(" ")
        text = "Intro [%s\n%s](other.md#x) then %s.\n" % (first, second, term)
        assert term_names(text) == [], "link text split after %r" % first


def test_multi_word_term_split_across_lines_is_still_a_use() -> None:
    text = "A page about a secondary\nsource and a nested\nentry.\n"
    assert term_names(text) == ["secondary source", "nested entry"]


def test_unlinked_term_on_one_line_is_reported() -> None:
    assert term_names("A secondary source here.\n") == ["secondary source"]


def test_blank_line_between_the_words_is_not_a_term() -> None:
    text = "The copy is secondary\n\nSource files follow.\n"
    assert term_names(text) == []


PAGE = "# Differences\n\n### Exception 47\n\nText.\n\n### Difference 102\n\nText.\n\n### Difference 230\n"


def test_a_cited_number_the_page_defines_passes() -> None:
    sources = [("internal/walk/x_test.go", "// The home (exception 47) and difference 102.\n")]
    assert links.citation_problems(PAGE, sources) == []


def test_a_cited_number_the_page_lacks_is_reported_where_it_is_cited() -> None:
    sources = [("internal/a.go", "package a\n\n// ordered by thread id (difference 55).\n")]
    assert links.citation_problems(PAGE, sources) == [
        "internal/a.go:3: difference 55 is not defined on %s" % links.DIFFERENCES_PAGE
    ]


def test_each_number_of_a_list_is_checked() -> None:
    sources = [("internal/a.go", "// differences 102, 230 and 260, or 231 to 232\n")]
    found = [problem.split(": ", 1)[1] for problem in links.citation_problems(PAGE, sources)]
    assert found == [
        "difference %s is not defined on %s" % (number, links.DIFFERENCES_PAGE)
        for number in ("260", "231", "232")
    ]


def test_a_citation_broken_across_comment_lines_is_read() -> None:
    sources = [("internal/a.go", "// the dotted home, as exception\n// 48 says\n")]
    assert links.citation_problems(PAGE, sources) == [
        "internal/a.go:1: exception 48 is not defined on %s" % links.DIFFERENCES_PAGE
    ]


def test_a_citation_needs_a_heading_of_its_own_kind() -> None:
    sources = [("cmd/doubleagent/main.go", "# Exception 102 and Difference 47\n")]
    assert len(links.citation_problems(PAGE, sources)) == 2


def test_words_that_only_contain_the_kinds_are_not_citations() -> None:
    sources = [
        ("internal/a.go", "// exceptional 4, the differences between 2 files, Exception47\n")
    ]
    assert links.citation_problems(PAGE, sources) == []


def test_a_number_the_page_defines_twice_is_reported() -> None:
    page = PAGE + "\n### Exception 47\n\n### Exception 230\n"
    assert links.citation_problems(page, []) == [
        "%s:13: number 47 is defined twice (first at line 3)" % links.DIFFERENCES_PAGE,
        "%s:15: number 230 is defined twice (first at line 11)" % links.DIFFERENCES_PAGE,
    ]


def test_a_heading_inside_a_fence_defines_nothing() -> None:
    page = "```text\n### Exception 1\n```\n"
    sources = [("internal/a.go", "// exception 1\n")]
    assert len(links.citation_problems(page, sources)) == 1


def test_only_code_outside_vendor_is_read() -> None:
    cases = {
        "cmd/doubleagent/main.go": True,
        "internal/walk/walk_test.go": True,
        "internal/sqlitefile/testdata/generate.py": True,
        "internal/analyze/golden_test.go": True,
        "vendor/golang.org/x/sys/unix/a.go": False,
        "docs/differences-from-the-scripts.md": False,
    }
    for path, wanted in cases.items():
        assert links.cites_numbers(path) == wanted, path


def test_a_number_after_a_citation_without_a_separator_is_not_cited() -> None:
    sources = [("internal/a.go", "// difference 102\n// 2026 rows, exception 47 48\n")]
    assert links.citation_problems(PAGE, sources) == []


def test_every_agents_and_claude_file_is_a_rules_file() -> None:
    cases = {
        "AGENTS.md": True,
        "CLAUDE.md": True,
        "internal/AGENTS.md": True,
        "internal/parser/CLAUDE.md": True,
        "research/AGENTS.md": True,
        "research/claude-code.md": False,
        "docs/agents.md": False,
        "docs/AGENTS.md.txt": False,
    }
    for path, wanted in cases.items():
        assert links.is_rules_file(path) == wanted, path


def test_a_link_to_the_differences_page_is_recognised_from_any_directory() -> None:
    root = links.ROOT
    cases = [
        (root / "docs" / "CHANGELOG.md", "differences-from-the-scripts.md", True),
        (root / "docs" / "index.md", "differences-from-the-scripts.md#exception-1", True),
        (root / "AGENTS.md", "docs/differences-from-the-scripts.md", True),
        (root / "internal" / "AGENTS.md", "../docs/differences-from-the-scripts.md", True),
        (root / "lab" / "README.md", "/docs/differences-from-the-scripts.md", True),
        (root / "docs" / "index.md", "collect.md", False),
        (root / "docs" / "index.md", "#differences-from-the-scripts", False),
        (root / "docs" / "index.md", "https://example.com/differences-from-the-scripts.md", False),
    ]
    for source, target, wanted in cases:
        assert links.links_differences_page(source, target) == wanted, (source, target)
