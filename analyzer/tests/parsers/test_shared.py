"""Tests of the helpers the parsers share and of the outputs across parsers."""

import io
import json
import shutil
import tempfile
import unittest
from pathlib import Path
from typing import ClassVar

from doubleagent import catalog, cli, protobuf, sqlite_util
from doubleagent.inputs import Artifact, Home, open_input
from doubleagent.model import compact
from doubleagent.parsers import Options, by_agent
from doubleagent.timeutil import to_utc
from fixtures import (
    AGY_APP_CONVERSATION,
    AGY_CONVERSATION,
    CLAUDE_SESSION,
    CODEX_SESSION,
    CODY_VSCDB_REL,
    CODY_VSCODE_CHAT,
    CRUSH_SESSION,
    GEMINI_LEGACY,
    GEMINI_RESUMED,
    GEMINI_SESSION,
    GEMINI_SUBAGENT,
    GOOSE_SESSION,
    KIRO_EXPORT_SESSION,
    KIRO_SESSION,
    KIRO_SHELL_SESSION,
    OI_IMPORTED,
    OI_SESSION,
    OPENHANDS_CLI_CONV,
    OPENHANDS_CONV,
    SHELLGPT_CHAT,
    VSCODE_LEGACY_SESSION,
    VSCODE_SESSION,
    ZED_EXTERNAL,
    ZED_THREAD,
    _vscdb,
    cody_vscode_state,
)

from .base import ParserBase, read_jsonl


class TimeTests(unittest.TestCase):
    """Test `to_utc` on the timestamp forms the stores write."""

    def test_formats(self):
        self.assertEqual(to_utc("2026-10-01T10:00:00.000Z"), "2026-10-01T10:00:00.000Z")
        self.assertEqual(to_utc("2026-10-01T12:00:00+02:00"), "2026-10-01T10:00:00.000Z")
        self.assertEqual(to_utc(1790848800000), "2026-10-01T10:00:00.000Z")  # ms
        self.assertEqual(to_utc(1790848800), "2026-10-01T10:00:00.000Z")  # s
        self.assertEqual(to_utc("1790848800"), "2026-10-01T10:00:00.000Z")
        # go-sqlite3 trims trailing fraction zeros; Go keeps nanoseconds.
        self.assertEqual(to_utc("2026-03-01 09:00:00.5-08:00"), "2026-03-01T17:00:00.500Z")
        self.assertEqual(to_utc("2026-03-01 09:00:01.25-08:00"), "2026-03-01T17:00:01.250Z")
        self.assertEqual(to_utc("2026-03-01T09:00:00.123456789Z"), "2026-03-01T09:00:00.123Z")
        self.assertEqual(to_utc(None), "")
        self.assertEqual(to_utc("garbage"), "")


class JsonlOutputTests(unittest.TestCase):
    """Test the text compaction and the one-line JSONL records."""

    def test_text_keeps_inner_whitespace(self):
        self.assertEqual(compact("  line one\n    indented\n"), "line one\n    indented")
        self.assertEqual(compact(None), "")

    def test_one_record_per_line(self):
        for text in ("a\nb\r\nc", "a\u2028b\u2029c", "lone \ud800 surrogate", "caf\u00e9"):
            line = cli.jsonl_line({"text": text})
            self.assertEqual(len(line.splitlines()), 1, repr(text))
            self.assertEqual(json.loads(line), {"text": text})
        self.assertIn("caf\u00e9", cli.jsonl_line({"text": "caf\u00e9"}))


class TimelineTests(ParserBase):
    """Test the timeline command's outputs over the fixture home."""

    def test_cli_timeline_outputs(self):
        out = self.tmp / "out"
        buffer = io.StringIO()
        import contextlib

        with contextlib.redirect_stdout(buffer):
            exit_code = cli.main(
                ["timeline", str(self.tmp / "home"), "-o", str(out), "--host", "h1"]
            )
        self.assertEqual(exit_code, 0, buffer.getvalue())
        rows = read_jsonl(out / "timeline.jsonl")
        timestamps = [row["timestamp_utc"] for row in rows if row["timestamp_utc"]]
        self.assertEqual(timestamps, sorted(timestamps))
        # Subset checks: each parser branch adds its own agents and sessions.
        expected_agents = {
            "claude-code",
            "claude-desktop",
            "codex-cli",
            "antigravity",
            "qwen-code",
            "kiro",
            "gemini-cli",
            "crush",
            "goose",
            "zed",
            "vscode",
            "cline",
            "roo-code",
            "tabby",
            "openhands",
            "shellgpt",
            "pi",
            "little-coder",
            "letta",
            "hermes",
            "agent-zero",
            "open-interpreter",
            "openclaw",
            "nanobot",
            "cody",
            "twinny",
            "pearai",
            "muse-code",
            "ollama",
        }
        self.assertLessEqual(expected_agents, {row["agent"] for row in rows})
        self.assertEqual({row["host"] for row in rows}, {"h1"})
        sessions = {row["session_id"]: row for row in read_jsonl(out / "sessions.jsonl")}
        expected_sessions = {
            CLAUDE_SESSION,
            CODEX_SESSION,
            AGY_CONVERSATION,
            AGY_APP_CONVERSATION,
            "99999999-0000-4000-8000-000000000000",
            "e5f6a7b8-4444-4000-8000-000000000011",
            "0a0b0c0d-4444-4000-8000-000000000022",
            KIRO_SESSION,
            KIRO_EXPORT_SESSION,
            KIRO_SHELL_SESSION,
            GEMINI_SESSION,
            GEMINI_RESUMED,
            GEMINI_LEGACY,
            GEMINI_SUBAGENT,
            CRUSH_SESSION,
            GOOSE_SESSION,
            "20260301_090000",
            ZED_THREAD,
            ZED_EXTERNAL,
            VSCODE_SESSION,
            VSCODE_LEGACY_SESSION,
            OPENHANDS_CONV,
            OPENHANDS_CLI_CONV,
            SHELLGPT_CHAT,
            "0199a1b2-7c3d-7e4f-8a5b-6c7d8e9f0a1b",
            "local-conv-1",
            "conv-9f",
            "20261001_120000_a1b2c3d4",
            "20261001_130000_0badf00d",
            "AbCd1234",
            OI_SESSION,
            OI_IMPORTED,
            "cmpl-7f3a",
            "7",  # tabby completion and thread
            "7d0c2a8e-1111-4000-8000-000000000001",
            "telegram:123456789",
            "Sat, 03 Oct 2026 10:00:00 GMT",
            "7d1c2b3a-1111-4000-8000-000000000001",
            "9b1c2d3e-0000-4000-8000-000000000001",
            "1791036000000",
            "01a0f000-0000-7000-8000-000000000001",  # muse-code
            "0f3c2b1a-5d6e-4f70-8a9b-0c1d2e3f4a5b",  # muse-code subagent
            "0190a000-0000-7000-8000-00000000c001",  # ollama desktop chat
        }
        self.assertLessEqual(expected_sessions, set(sessions))
        session = sessions[CLAUDE_SESSION]
        self.assertEqual(session["models"], ["claude-fable-5-1"])
        # Four calls in the session file and the fork call in a subagent file,
        # which carries the parent's session id.
        self.assertEqual(session["tool_calls"], 5)
        self.assertTrue(
            session["source_file"].endswith(CLAUDE_SESSION + ".jsonl"), session["source_file"]
        )
        self.assertEqual(session["first_timestamp_utc"], "2026-10-01T10:00:00.000Z")
        self.assertEqual(sessions[CODEX_SESSION]["models"], ["gpt-5-codex", "gpt-5.5-codex"])

    def test_agent_filter(self):
        out = self.tmp / "out"
        import contextlib

        with contextlib.redirect_stdout(io.StringIO()):
            cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), "--agent", "codex-cli"])
        rows = read_jsonl(out / "timeline.jsonl")
        self.assertEqual({row["agent"] for row in rows}, {"codex-cli"})


class ProjectRoutingTests(unittest.TestCase):
    """Test project attribution through a round trip of the sh collector.

    Files the collector finds inside a discovered repository have the project
    directory as their home, marked `project`. Their agent is the one the project
    catalog names (re-attributed by the analyzer for a collection that says
    `project`), so Crush and Aider rows come out of them; a shared file such as
    `AGENTS.md` keeps agent `project`.
    """

    def setUp(self):
        self.cat = catalog.load()
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-analyzer-test-"))

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def _collect(self) -> Path:
        import subprocess

        collector = (
            Path(__file__).resolve().parents[3] / "collectors" / "collect-agent-artifacts.sh"
        )
        if not collector.exists() or shutil.which("sh") is None or shutil.which("tar") is None:
            self.skipTest("collector or sh/tar not available")
        from fixtures import CRUSH_SCHEMA, _wal_database, build_aider, build_image, crush_records

        root = build_image(self.tmp / "image")
        # Claude Code's history names /srv/proj, so the collector discovers it.
        project = root / "srv/proj"
        build_aider(project)
        _wal_database(project / ".crush/crush.db", CRUSH_SCHEMA, crush_records())
        (project / "AGENTS.md").write_text("# shared instructions\n", encoding="utf-8")
        out = self.tmp / "out"
        out.mkdir()
        result = subprocess.run(
            ["sh", str(collector), "-r", str(root), "-o", str(out), "-q"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        archives = list(out.glob("*.tar.gz"))
        self.assertEqual(len(archives), 1, list(out.iterdir()))
        return archives[0]

    def _timeline(self, archive: Path, *extra: str):
        import contextlib

        out = self.tmp / ("tl%d" % len(list(self.tmp.glob("tl*"))))
        buffer = io.StringIO()
        with contextlib.redirect_stdout(buffer):
            exit_code = cli.main(["timeline", str(archive), "-o", str(out), *list(extra)])
        self.assertEqual(exit_code, 0, buffer.getvalue())
        return read_jsonl(out / "timeline.jsonl"), buffer.getvalue()

    def test_project_artifacts_reach_their_parsers(self):
        archive = self._collect()
        col = open_input(archive, self.cat)
        try:
            project = {
                artifact.rel: artifact.agent
                for artifact in col.artifacts
                if artifact.home.project and artifact.home.original.endswith("/srv/proj")
            }
            bases = {artifact.home.original for artifact in col.artifacts if artifact.home.project}
        finally:
            col.cleanup()
        # Homes are never project bases; /srv/proj (under the image root) is one.
        self.assertTrue(any(base.endswith("/srv/proj") for base in bases), bases)
        self.assertFalse(any(base.endswith(("/home/alice", "/home/bob")) for base in bases), bases)
        self.assertIn(".crush/crush.db", project)
        self.assertIn(".aider.chat.history.md", project)
        # Whatever agent the bundled project catalog names for each file.
        self.assertEqual(project, {rel: self.cat.project_agent_for(rel) for rel in project})
        self.assertEqual(project["AGENTS.md"], "project")

        rows, summary = self._timeline(archive)
        from_project = [row for row in rows if "/srv/proj/" in row["source_file"]]
        self.assertEqual({row["agent"] for row in from_project}, {"crush", "aider"})
        self.assertTrue(
            any(
                row["source_file"].endswith("/srv/proj/.crush/crush.db")
                and row["turn_type"] == "user"
                for row in from_project
            )
        )
        self.assertTrue(
            any(
                row["source_file"].endswith("/srv/proj/.aider.chat.history.md")
                and row["turn_type"] == "user"
                for row in from_project
            )
        )
        self.assertNotIn("project", {row["agent"] for row in rows})
        self.assertNotIn("parsed project", summary)

        # --agent filters on the parser's agent, so repository files come along.
        rows, _ = self._timeline(archive, "--agent", "aider")
        self.assertEqual({row["agent"] for row in rows}, {"aider"})
        self.assertTrue(
            any(row["source_file"].endswith("/srv/proj/.aider.chat.history.md") for row in rows)
        )

    def test_detect_does_not_call_project_parsed(self):
        archive = self._collect()
        col = open_input(archive, self.cat)
        try:
            table = cli.detect_table(col)
        finally:
            col.cleanup()
        project = [row for row in table if row["agent"] == "project"]
        self.assertTrue(project)  # AGENTS.md, shared by several agents
        self.assertFalse(any(row["parser"] for row in project))
        self.assertNotIn("project", by_agent())


class ProtobufTests(unittest.TestCase):
    """Test the protobuf encoder and decoder against a small schema."""

    SCHEMA: ClassVar[dict] = {
        "M": {
            1: ("a", "str"),
            2: ("+n", "int"),
            3: ("sub", "N"),
            4: ("t", "ts"),
            5: ("f", "bool"),
            6: ("+subs", "N"),
        },
        "N": {1: ("x", "str"), 2: ("d", "double")},
    }

    def test_round_trip(self):
        value = {
            "a": "hi",
            "n": [1, 2, 300],
            "sub": {"x": "y", "d": 1.5},
            "t": 1790848800.5,
            "f": True,
            "subs": [{"x": "p"}, {"x": "q"}],
        }
        encoded = protobuf.encode(value, "M", self.SCHEMA)
        out = protobuf.decode(encoded, "M", self.SCHEMA)
        self.assertEqual(out["a"], "hi")
        self.assertEqual(out["n"], [1, 2, 300])
        self.assertEqual(out["sub"], {"x": "y", "d": 1.5})
        self.assertEqual(out["t"], "2026-10-01T10:00:00.500Z")
        self.assertTrue(out["f"])
        self.assertEqual([sub["x"] for sub in out["subs"]], ["p", "q"])

    def test_unknown_fields_are_skipped(self):
        wide = {"M": dict(self.SCHEMA["M"]), "N": self.SCHEMA["N"]}
        wide["M"].update({9: ("extra", "str"), 10: ("more", "N")})
        encoded = protobuf.encode({"a": "x", "extra": "ignored", "more": {"x": "z"}}, "M", wide)
        self.assertEqual(protobuf.decode(encoded, "M", self.SCHEMA), {"a": "x"})

    def test_truncated_buffer_raises(self):
        encoded = protobuf.encode({"a": "hello"}, "M", self.SCHEMA)
        with self.assertRaises(ValueError):
            protobuf.decode(encoded[:-2], "M", self.SCHEMA)


class SqliteUtilTests(unittest.TestCase):
    """Test that SQLite evidence is read from a copy, its WAL sidecar included."""

    def test_copy_leaves_original_untouched_and_reads_wal(self):
        import sqlite3

        temporary = Path(tempfile.mkdtemp(prefix="cac-analyzer-test-"))
        try:
            database = temporary / "t.db"
            writer = sqlite3.connect(str(database))
            writer.execute("pragma journal_mode=wal")
            writer.execute("create table t (x)")
            writer.execute("insert into t values (1)")
            writer.commit()
            # a second connection keeps the WAL from being checkpointed on close
            holder = sqlite3.connect(str(database))
            holder.execute("select count(*) from t").fetchone()
            writer.execute("insert into t values (2)")
            writer.commit()
            before = database.read_bytes()
            self.assertTrue((temporary / "t.db-wal").exists())
            with sqlite_util.open_copy(database) as connection:
                self.assertEqual(connection.execute("select count(*) from t").fetchone()[0], 2)
            self.assertEqual(database.read_bytes(), before)
            holder.close()
            writer.close()
        finally:
            shutil.rmtree(temporary, ignore_errors=True)


class ReadsAgentsRoutingTests(ParserBase):
    """Test the routing of a `vscode` state.vscdb through `reads_agents`.

    The file goes to the Cody, Twinny, Roo Code and Tabby parsers as well as to the
    VS Code parser, and nothing else goes to them.
    """

    def test_state_vscdb_offered_to_extension_parsers(self):
        parsers = by_agent()
        art = next(artifact for artifact in self.col.artifacts if artifact.rel == CODY_VSCDB_REL)
        self.assertEqual(art.agent, "vscode")
        offered = cli.parsers_for(art, parsers)
        self.assertEqual(
            {parser.agent for parser in offered}, {"vscode", "cody", "twinny", "roo-code", "tabby"}
        )
        self.assertEqual(len(offered), len(set(map(id, offered))))
        # The VS Code parser reads the Copilot Chat row of the same database,
        # which Cody's has none of.
        vscode = [parser for parser in offered if parser.agent == "vscode"]
        self.assertTrue(all(parser.wants(art) for parser in vscode))
        self.assertEqual([row for parser in vscode for row in parser.parse(art, Options())], [])
        # A codex-cli file is offered to its own parser and, through
        # ChatGPT Desktop's `reads_agents`, to that one, never to the
        # extension parsers.
        other = next(artifact for artifact in self.col.artifacts if artifact.agent == "codex-cli")
        self.assertEqual(
            {parser.agent for parser in cli.parsers_for(other, parsers)},
            {"codex-cli", "chatgpt-desktop"},
        )
        other = next(artifact for artifact in self.col.artifacts if artifact.agent == "claude-code")
        self.assertEqual(
            {parser.agent for parser in cli.parsers_for(other, parsers)}, {"claude-code"}
        )

    def test_cli_attributes_rows_to_the_extension(self):
        rows, _counts, problems, _ = cli.collect_rows(self.col, Options(), [])
        self.assertEqual(problems, [])
        from_vscdb = {
            row.agent for row in rows if row.source_file.endswith("/User/globalStorage/state.vscdb")
        }
        # `vscode` rows come from the Copilot Chat cloud agent tasks,
        # `cursor` rows from Cursor's own state.vscdb, `pearai` rows from
        # PearAI's, `roo-code` permission settings and `tabby` inline-edit
        # command history.
        self.assertEqual(
            from_vscdb, {"cody", "cursor", "pearai", "roo-code", "tabby", "twinny", "vscode"}
        )
        rows, _, _, _ = cli.collect_rows(self.col, Options(), ["twinny"])
        self.assertEqual({row.agent for row in rows}, {"twinny"})

    def test_fork_state_vscdb_reaches_cody(self):
        # Cody installed in Cursor writes to Cursor's state.vscdb, which the
        # catalog attributes to `cursor`, not `vscode`.
        rel = ".config/Cursor/User/globalStorage/state.vscdb"
        _vscdb(self.home / rel, {"sourcegraph.cody-ai": cody_vscode_state()})
        col = open_input(self.tmp / "home", self.cat, host="h1")
        art = next(artifact for artifact in col.artifacts if artifact.rel == rel)
        self.assertEqual(art.agent, "cursor")
        self.assertIn("cody", {parser.agent for parser in cli.parsers_for(art, by_agent())})
        rows, _, problems, _ = cli.collect_rows(col, Options(), ["cody"])
        self.assertEqual(problems, [])
        from_cursor = [row for row in rows if row.source_file.endswith(rel)]
        self.assertTrue(from_cursor)
        self.assertEqual({row.agent for row in from_cursor}, {"cody"})
        self.assertIn(CODY_VSCODE_CHAT, {row.session_id for row in from_cursor})


class HasSecondaryTests(ParserBase):
    """Test that `Parser.has_secondary` agrees with `secondary()` over the fixture.

    `sessions.jsonl` `recovered` reads "no" rather than "" only for the agents it
    names, so a parser whose `secondary()` can return True needs it set, and one
    whose never does must not set it.
    """

    def test_has_secondary_matches_fixture(self):
        from doubleagent.parsers import ALL
        from fixtures import CODY_LEGACY_REL, CODY_LEGACY_XML, build_kilo_state, build_twinny_415

        # Secondary fixtures that build_home does not write: Kilo's terminal
        # UI prompt history, Twinny 4.3.15's agent-changes records and log,
        # and the Cody JetBrains plugin's project-local history.
        build_kilo_state(self.home)
        build_twinny_415(self.tmp / "home" / "bob")  # a home of its own
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        project = self.tmp / "srv" / "proj"
        (project / CODY_LEGACY_REL).parent.mkdir(parents=True, exist_ok=True)
        (project / CODY_LEGACY_REL).write_text(CODY_LEGACY_XML, encoding="utf-8")
        home = Home(disk_path=project, original="/srv/proj", user="alice", host="h1", project=True)
        legacy = Artifact(
            project / CODY_LEGACY_REL, "/srv/proj/" + CODY_LEGACY_REL, CODY_LEGACY_REL, "cody", home
        )
        parsers = by_agent()
        seen = set()
        for artifact in [*self.col.artifacts, legacy]:
            for parser in cli.parsers_for(artifact, parsers):
                if parser.wants(artifact) and parser.secondary(artifact):
                    seen.add(type(parser).__name__)
        declared = {type(parser).__name__ for parser in ALL if parser.has_secondary}
        self.assertEqual(declared - seen, set(), "has_secondary set, no secondary fixture file")
        self.assertEqual(seen - declared, set(), "secondary() True, has_secondary not set")
        self.assertIn("CodexParser", declared)
