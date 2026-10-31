"""Tests of the bundled catalog: glob matching, parsing and agreement with the docs."""

import re
import subprocess
import unittest
from pathlib import Path

from doubleagent import catalog
from doubleagent.parsers import NO_TRANSCRIPTS, by_agent

REPO = Path(__file__).resolve().parents[2]
COLLECTOR = REPO / "collectors" / "collect-agent-artifacts.sh"


class GlobTests(unittest.TestCase):
    """Catalog glob segments compile to the regular expressions the catalog rules need."""

    def pattern(self, glob):
        return catalog.glob_segment_to_regex(glob)

    def test_star_does_not_cross_slash(self):
        self.assertIsNotNone(self.pattern("*.jsonl").fullmatch("a.jsonl"))
        self.assertIsNone(self.pattern("*").fullmatch("a/b"))

    def test_question_and_class(self):
        self.assertIsNotNone(self.pattern("rollout-?.jsonl").fullmatch("rollout-1.jsonl"))
        self.assertIsNotNone(self.pattern("[Zz]ed").fullmatch("Zed"))
        self.assertIsNotNone(self.pattern("[Zz]ed").fullmatch("zed"))
        self.assertIsNone(self.pattern("[Zz]ed").fullmatch("Xed"))
        self.assertIsNotNone(self.pattern("[!a]bc").fullmatch("xbc"))
        self.assertIsNone(self.pattern("[!a]bc").fullmatch("abc"))

    def test_literal_dots_and_spaces(self):
        self.assertIsNotNone(self.pattern("Application Support").fullmatch("Application Support"))
        self.assertIsNone(self.pattern(".claude.json*").fullmatch("xclaudejson"))
        self.assertIsNotNone(self.pattern(".claude.json*").fullmatch(".claude.json.backup"))


class CatalogTests(unittest.TestCase):
    """The bundled catalog parses and matches the collector's `--list` output."""

    def setUp(self):
        self.cat = catalog.load()

    def test_sections_parsed(self):
        self.assertGreater(len(self.cat.entries), 100)
        self.assertGreater(len(self.cat.project_entries), 30)
        self.assertGreater(len(self.cat.excludes), 50)
        self.assertGreater(len(self.cat.secrets), 50)
        self.assertIn("claude-code", self.cat.agents)
        self.assertIn("codex-cli", self.cat.agents)
        # Each project entry names a catalog agent, or `project` for a file
        # several agents read (AGENTS.md).
        agents = set(self.cat.agents) | {"project"}
        self.assertTrue(all(entry.agent in agents for entry in self.cat.project_entries))

    def test_every_entry_has_agent_and_glob(self):
        for entry in self.cat.entries:
            self.assertTrue(entry.agent and entry.glob and entry.segments, entry)

    @unittest.skipUnless(COLLECTOR.exists(), "collector script not present")
    def test_bundled_copy_matches_collector_list(self):
        out = subprocess.run(
            ["sh", str(COLLECTOR), "--list"], capture_output=True, text=True, check=True
        ).stdout
        self.assertEqual(
            out,
            self.cat.text,
            "catalog.txt drifted; run: collectors/collect-agent-artifacts.sh --list > analyzer/doubleagent/catalog.txt",
        )


def test_lines_end_at_lf_or_crlf_only():
    # A glob holding a Unicode line break stays one line; ASCII space alone is blank.
    for separator in map(chr, (0x2028, 0x0085, 0x000C)):
        text = "first-agent|.first%sx\r\n \t\nsecond-agent|.second\n" % separator
        parsed = catalog.parse(text)
        globs = [entry.glob for entry in parsed.entries]
        assert globs == [".first%sx" % separator, ".second"], repr(separator)
    # A line of NO-BREAK SPACE is not blank: it is read as any other line
    # lacking `|` is, never skipped.
    for line in ("no-separator", chr(0x00A0)):
        parsed = catalog.parse("first-agent|.first\n%s\n" % line)
        assert len(parsed.entries) == 2, repr(line)
        assert parsed.entries[1].agent == line, repr(line)


if __name__ == "__main__":
    unittest.main()


class NoTranscriptsTests(unittest.TestCase):
    """The docs' `collected` agents and NO_TRANSCRIPTS agree.

    The `collected` rows of the table in docs/agents.md and the NO_TRANSCRIPTS set the analyzer
    reports as collected only must agree.
    """

    def collected_rows(self) -> set[str]:
        text = (REPO / "docs" / "agents.md").read_text(encoding="utf-8")
        # The table sits under the page's intro, before its first section.
        table = text.split("\n## ", 1)[0]
        out = set()
        for line in table.splitlines():
            # A row is | [`agent`](#agent) | status | validated | notes |.
            cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
            name = re.match(r"\[`([^`]+)`\]\(#", cells[0])
            if name and len(cells) > 1 and cells[1] == "collected":
                out.add(name.group(1))
        return out

    def test_table_matches_set(self):
        rows = self.collected_rows()
        self.assertTrue(rows)
        self.assertEqual(rows, set(NO_TRANSCRIPTS))

    def test_members_are_catalog_agents_without_parser(self):
        agents = set(catalog.load().agents)
        parsers = by_agent()
        for agent in NO_TRANSCRIPTS:
            self.assertIn(agent, agents)
            self.assertNotIn(agent, parsers)


def _heading(section):
    """Return the `--list` heading line of one section, from the mapping in `catalog.py`."""
    for text, name in catalog.SECTION_HEADINGS.items():
        if name == section:
            return f"# {text}"
    raise KeyError(section)


LOOKALIKE_COMMENTS = [
    ("# excluded by the vendor's installer, see the research document", "", ""),
    ("# project artifacts the vendor documents", "", ""),
    ("# credential files are kept in the keychain", "", ""),
    ("# docker volumes are not created by this tool", "", ""),
    ("", "# home artifacts moved with the config directory", ""),
    ("", "", "# Home Artifacts (agent|glob relative to home)"),
]


def test_comment_starting_with_heading_words_keeps_section():
    """A comment opening with a heading's words, or differing from one in case, is ignored."""
    for home_comment, project_comment, docker_comment in LOOKALIKE_COMMENTS:
        text = "\n".join(
            [
                _heading("home"),
                "first-agent|.first",
                home_comment,
                "second-agent|.second",
                _heading("project"),
                "first-agent|.first-project",
                project_comment,
                "second-agent|.second-project",
                _heading("exclude"),
                ".first/cache",
                _heading("docker"),
                "first-agent|*first_data",
                docker_comment,
                "second-agent|*second_data",
            ]
        )
        parsed = catalog.parse(text)
        comment = home_comment or project_comment or docker_comment
        assert [entry.glob for entry in parsed.entries] == [".first", ".second"], comment
        assert [entry.glob for entry in parsed.project_entries] == [
            ".first-project",
            ".second-project",
        ], comment
        assert parsed.excludes == [".first/cache"], comment
        assert parsed.secrets == [], comment
        volumes = [entry.glob for entry in parsed.docker_volumes]
        assert volumes == ["*first_data", "*second_data"], comment


def test_each_exact_heading_switches_section():
    text = "\n".join(
        [
            _heading("home"),
            "home-agent|.home-agent",
            _heading("project"),
            "project-agent|.project-agent",
            "",
            _heading("exclude"),
            ".home-agent/cache",
            "",
            _heading("secret"),
            ".home-agent/auth.json",
            "",
            _heading("docker"),
            "volume-agent|*volume_data",
        ]
    )
    parsed = catalog.parse(text)
    assert [entry.glob for entry in parsed.entries] == [".home-agent"]
    assert [entry.glob for entry in parsed.project_entries] == [".project-agent"]
    assert parsed.excludes == [".home-agent/cache"]
    assert parsed.secrets == [".home-agent/auth.json"]
    assert [entry.glob for entry in parsed.docker_volumes] == ["*volume_data"]


def test_bundled_catalog_holds_each_heading_once_in_order():
    """A changed heading in the collectors fails here instead of emptying a table."""
    headings = {f"# {text}" for text in catalog.SECTION_HEADINGS}
    lines = catalog.CATALOG_FILE.read_text(encoding="utf-8").split("\n")
    found = [line for line in lines if line in headings]
    assert found == [f"# {text}" for text in catalog.SECTION_HEADINGS]
