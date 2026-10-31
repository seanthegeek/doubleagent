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
