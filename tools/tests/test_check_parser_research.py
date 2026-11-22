"""Tests for tools/check_parser_research.py on synthetic parser facts and research documents."""

from __future__ import annotations

import pathlib
import textwrap

import check_parser_research as checker
from check_parser_research import Fact, Facts, ParserFacts, Pattern

ALPHA_PACKAGE = "example.test/alpha"
SHARED_PACKAGE = "example.test/shared"
BETA_PACKAGE = "example.test/beta"


def write(path: pathlib.Path, text: str) -> pathlib.Path:
    """Write dedented `text` to `path`, creating its directory, and return the path."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(textwrap.dedent(text), encoding="utf-8")
    return path


def document(agent: str, parsing: str, fixtures: str = "None.") -> str:
    """Return a research document whose Parsing part holds `parsing` and the given Fixtures."""
    return "\n\n".join(
        [
            "# %s" % agent,
            "A test agent.",
            "## Source and evidence",
            "## Collection",
            "Collected: `.%s/notes.md` and `model: ` outside Parsing." % agent,
            "## Parsing",
            "### Record schema",
            parsing,
            "### Parser plan",
            "#### Fixtures",
            fixtures,
            "## Confidence",
            "High.",
            "",
        ]
    )


SURVIVAL = """\
# Research

| Agent | Transcripts live in | Survives | History | Directory | Second copy |
| --- | --- | --- | --- | --- | --- |
| `alpha` | `.alpha/logs/` | yes | `history.jsonl`, secondary | none | none |
"""


def repository(
    root: pathlib.Path, alpha: str, beta: str = "", index: str = SURVIVAL
) -> pathlib.Path:
    """Write `research/alpha.md`, `research/beta.md` when given, and the index; return `root`."""
    write(root / "research/alpha.md", alpha)
    if beta:
        write(root / "research/beta.md", beta)
    write(root / "research/README.md", index)
    return root


def pattern(text: str, regex: bool = True, secondary: bool = False) -> Pattern:
    """Return a pattern of the alpha package at line 1."""
    return Pattern(text, regex, secondary, "alpha.go", 1, ALPHA_PACKAGE)


def facts(*items: Fact, patterns: tuple[Pattern, ...] = (), beta: bool = False) -> Facts:
    """Return the facts of an alpha parser reading with the shared package, and of beta when set.

    Beta reads with the shared package too.
    """
    parsers = [
        ParserFacts(
            "alpha", ALPHA_PACKAGE, ALPHA_PACKAGE, (ALPHA_PACKAGE, SHARED_PACKAGE), patterns
        )
    ]
    if beta:
        parsers.append(ParserFacts("beta", BETA_PACKAGE, BETA_PACKAGE, (SHARED_PACKAGE,), ()))
    return Facts(tuple(parsers), items)


def fact(kind: str, text: str, prefix: str = "", package: str = ALPHA_PACKAGE) -> Fact:
    """Return a fact of a package at line 7."""
    return Fact(package, kind, text, prefix or text, "alpha.go", 7)


def found(root: pathlib.Path, given: Facts) -> list[tuple[str, str, str]]:
    """Return (agent, kind, detail) of every finding."""
    return [(item.agent, item.kind, item.detail) for item in checker.findings(root, given)]


def test_python_pattern_reads_go_anchors_and_drops_the_case_flag():
    assert checker.python_pattern(r"(?i)\A\.alpha/x\.json\z") == r"^\.alpha/x\.json$"
    assert checker.python_pattern(r"^\.alpha/x$") == r"^\.alpha/x$"


def test_read_facts_keeps_only_resolved_patterns():
    data = {
        "parsers": [
            {
                "agent": "alpha",
                "home": ALPHA_PACKAGE,
                "package": ALPHA_PACKAGE,
                "packages": [ALPHA_PACKAGE],
                "patterns": [
                    {"text": r"\Aa\z", "regex": True, "secondary": True, "resolved": True}
                    | {"file": "a.go", "line": 2, "package": ALPHA_PACKAGE},
                    {"text": "{}/b", "regex": False, "secondary": False, "resolved": False}
                    | {"file": "a.go", "line": 3, "package": ALPHA_PACKAGE},
                ],
            }
        ],
        "facts": [
            {"package": ALPHA_PACKAGE, "kind": "key", "text": "type", "prefix": "type"}
            | {"file": "a.go", "line": 4}
        ],
    }
    read = checker.read_facts(data)
    assert [item.text for item in read.parsers[0].patterns] == ["^a$"]
    assert read.facts == (Fact(ALPHA_PACKAGE, "key", "type", "type", "a.go", 4),)


def test_expansions_spell_out_alternations_optional_and_capture_groups():
    assert checker.expansions(r"(?:\.a|\.b/c)/x\.json") == [r"\.a/x\.json", r"\.b/c/x\.json"]
    assert checker.expansions(r"(?:^|/)x") == ["x", "/x"]
    assert checker.expansions(r"(?P<base>a|b)/y") == ["(?P<base>a|b)/y", "a/y", "b/y"]
    assert checker.expansions(r"a(?:b/)*c") == ["a(?:b/)*c"]


def test_documented_facts_give_no_finding(tmp_path):
    parsing = (
        "`type` and `message.model` keys; rows `model: <id>` and"
        " `parser: unparseable line (no header)`."
    )
    given = facts(
        fact("key", "type"),
        fact("key", "model"),
        fact("text", "model: {}", "model: "),
        fact("message", "unparseable line (no header)"),
    )
    assert found(repository(tmp_path, document("alpha", parsing)), given) == []


def test_each_undocumented_fact_gives_its_finding(tmp_path):
    given = facts(
        fact("key", "sessionId"),
        fact("text", "permission decision: {}", "permission decision: "),
        fact("message", "record {} has no id"),
    )
    root = repository(tmp_path, document("alpha", "`type`, `model: <id>`, `record`."))
    assert found(root, given) == [
        ("alpha", "key-undocumented", "sessionId"),
        ("alpha", "row-text-undocumented", "permission decision:"),
        ("alpha", "parser-message-undocumented", "record {} has no id"),
    ]


def test_a_key_must_be_a_whole_word_of_the_parsing_part(tmp_path):
    root = repository(tmp_path, document("alpha", "`ai-kind` and `origin.kind`."))
    assert found(root, facts(fact("key", "kind"), fact("key", "ai-kind"))) == []
    root = repository(tmp_path, document("alpha", "`ai-kind`."))
    assert found(root, facts(fact("key", "kind"), fact("key", "ai"))) == [
        ("alpha", "key-undocumented", "kind"),
        ("alpha", "key-undocumented", "ai"),
    ]


def test_a_message_needs_its_pieces_in_order_in_one_span(tmp_path):
    root = repository(tmp_path, document("alpha", "`has no id` and `record <n>`."))
    given = facts(fact("message", "record {} has no id"))
    assert found(root, given) == [("alpha", "parser-message-undocumented", "record {} has no id")]
    root = repository(tmp_path, document("alpha", "`parser: record <n> has no id`."))
    assert found(root, given) == []


def test_a_fact_outside_the_parsing_part_is_undocumented(tmp_path):
    root = repository(tmp_path, document("alpha", "Nothing."))
    given = facts(fact("text", "model: {}", "model: "))
    assert found(root, given) == [("alpha", "row-text-undocumented", "model:")]


def test_a_shared_package_fact_is_documented_by_any_reader_and_goes_to_the_first(tmp_path):
    shared = fact("key", "sharedKey", package=SHARED_PACKAGE)
    root = repository(tmp_path, document("alpha", "None."), document("beta", "`sharedKey`."))
    assert found(root, facts(shared, beta=True)) == []
    root = repository(tmp_path, document("alpha", "None."), document("beta", "None."))
    assert found(root, facts(shared, beta=True)) == [("alpha", "key-undocumented", "sharedKey")]


def test_patterns_are_documented_by_their_text_or_their_segments(tmp_path):
    parsing = "`^\\.alpha/logs/[^/]+\\.jsonl$`, `.alpha/history.jsonl`, `.beta/<id>/x.json`."
    root = repository(tmp_path, document("alpha", parsing))
    documented = (
        pattern(r"^\.alpha/logs/[^/]+\.jsonl$"),
        pattern(r"^\.alpha/history\.jsonl$"),
        pattern(r"^(?:\.gamma|\.beta)/[0-9a-f]+/x\.json$"),
        pattern(".alpha/history.jsonl", regex=False),
        pattern("/history.jsonl", regex=False),
    )
    assert found(root, facts(patterns=documented)) == []
    undocumented = (pattern(r"^\.alpha/other\.json$"), pattern(".alpha/state.db", regex=False))
    assert found(root, facts(patterns=undocumented)) == [
        ("alpha", "pattern-undocumented", r"^\.alpha/other\.json$"),
        ("alpha", "pattern-undocumented", ".alpha/state.db"),
    ]


def test_a_fixtures_span_documents_no_pattern(tmp_path):
    root = repository(tmp_path, document("alpha", "None.", fixtures="`.alpha/sessions/a1.json`"))
    given = facts(patterns=(pattern(r"^\.alpha/sessions/[^/]+\.json$"),))
    assert found(root, given) == [
        ("alpha", "pattern-undocumented", r"^\.alpha/sessions/[^/]+\.json$")
    ]


def test_secondary_patterns_must_be_named_by_the_survival_row(tmp_path):
    parsing = "`^\\.alpha/history\\.jsonl$` and `^\\.alpha/mirror/[^/]+\\.db$`."
    root = repository(tmp_path, document("alpha", parsing))
    listed = pattern(r"^\.alpha/history\.jsonl$", secondary=True)
    unlisted = pattern(r"^\.alpha/mirror/[^/]+\.db$", secondary=True)
    assert found(root, facts(patterns=(listed,))) == []
    assert found(root, facts(patterns=(listed, unlisted))) == [
        ("alpha", "secondary-unlisted", r"^\.alpha/mirror/[^/]+\.db$")
    ]
    root = repository(tmp_path, document("alpha", parsing), index="# Research\n")
    assert found(root, facts(patterns=(listed,))) == [("alpha", "survival-row-missing", "alpha")]


def test_the_baseline_holds_the_findings_and_a_stale_line_fails(tmp_path, capsys):
    root = repository(tmp_path, document("alpha", "None."))
    given = facts(fact("key", "sessionId"))
    assert checker.main([], root, given) == 1
    assert "key-undocumented [alpha]: sessionId" in capsys.readouterr().out
    assert checker.main(["--update-baseline"], root, given) == 0
    baseline = (root / "tools/drift-baseline/alpha.txt").read_text(encoding="utf-8")
    assert baseline == "check_parser_research|key-undocumented|sessionId\n"
    assert checker.main([], root, given) == 0
    assert checker.main([], root, facts()) == 1
    assert "stale: remove" in capsys.readouterr().out


def test_a_failed_facts_run_is_an_error_not_a_pass(tmp_path, capsys):
    repository(tmp_path, document("alpha", "None."))
    assert checker.main([], tmp_path) == 2
    assert "check_parser_research: go test" in capsys.readouterr().err
