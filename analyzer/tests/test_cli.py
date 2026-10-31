"""Tests of the `detect` and `timeline` commands' output.

The stdout summary, inventory.json, the session and project marks of sessions.jsonl, project
attribution, deduplication, Windows project paths, repository metadata and scratch directories.
"""

import contextlib
import io
import json
import os
import shutil
import sqlite3
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from doubleagent import VERSION, catalog, cli
from doubleagent.dedup import deduplicate
from doubleagent.inputs import open_input
from doubleagent.model import (
    Row,
    SessionSummary,
    canonical_project_path,
    project_key,
    summarise,
)
from doubleagent.parsers import ALL, Options, by_agent
from fixtures import (
    AGENT_ZERO_CHAT,
    AGENT_ZERO_TG_CHAT,
    AGY_APP_CONVERSATION,
    AGY_ORPHAN,
    CHATGPT_CONVERSATION,
    CHATGPT_TPP_CONVERSATION,
    CLAUDE_309_SESSION,
    CLAUDE_BRIDGE_SESSION,
    CLAUDE_SESSION,
    CLINE_BLOB_DELETED,
    CLINE_BLOB_SESSION,
    CLINE_QUEUE_FAILED,
    CLINE_QUEUE_PENDING,
    CLINE_SESSION,
    CLINE_TASK,
    CODEX_DESKTOP_ROLLOUT,
    CODEX_GLOBAL_STATE_REL,
    CODEX_ORPHAN,
    CODEX_SESSION,
    CONTINUE_SESSION,
    COPILOT_EVENT_LINES,
    COPILOT_SESSION,
    COPILOT_WORKSPACE_YAML,
    CURSOR_CLOUD,
    CURSOR_CLOUD_LISTED,
    CURSOR_LIVE,
    GOOSE_GATEWAY_SESSION,
    GOOSE_SESSION,
    HERMES_SESSION,
    KILO_SESSION,
    KILO_TASK,
    LETTA_CONV,
    LETTA_LOCAL_AGENT_CONV,
    NANOBOT_DATABASE_KEY,
    NANOBOT_KEY,
    OI_SESSION,
    OPENCLAW_RESET_SESSION,
    OPENCLAW_SESSION,
    OPENCODE_PRIVATE,
    OPENCODE_SHARED,
    PI_BUG_SESSION,
    PI_SESSION,
    PROJECT_GIT_CONFIG,
    PROJECT_GIT_HEAD,
    PROJECT_GIT_REMOTES,
    PROJECT_MIRROR,
    PROJECT_PUSH_URL,
    PROJECT_REMOTE,
    PROJECT_UPSTREAM,
    QWEN_ARCHIVED,
    QWEN_DAEMON_SESSION,
    QWEN_SESSION,
    VSCODE_CLOUD_TASK,
    VSCODE_LEGACY_SESSION,
    VSCODE_OLD_TASK,
    VSCODE_SESSION,
    build_cline_blob_upload,
    build_cline_sync_queue,
    build_home,
    codex_rollout_records,
    goose_gateway_records,
)

# `outside_projects` with no row and no session outside a project.
EMPTY_OUTSIDE = {
    "rows": 0,
    "sessions": 0,
    "agents": {},
    "synced": "",
    "relayed": "",
    "origin": "",
    "recovered": "",
}
# The trailing keys of a `projects.<path>` entry and of `outside_projects`.
PROJECT_KEYS = [
    "synced",
    "relayed",
    "origin",
    "recovered",
]
# The last keys of a project or scratch directory entry of inventory.json,
# after the marks; `outside_projects` has none of them.
REPO_KEYS = ["remotes", "head", "recorded_remotes"]


def run_timeline(root: Path, out: Path) -> tuple[int, str]:
    buffer = io.StringIO()
    with contextlib.redirect_stdout(buffer), contextlib.redirect_stderr(io.StringIO()):
        exit_code = cli.main(["timeline", str(root), "-o", str(out), "--host", "h1"])
    return exit_code, buffer.getvalue()


def agents_line(stdout: str) -> str:
    lines = [line for line in stdout.splitlines() if line.startswith("agents:")]
    assert len(lines) == 1, stdout
    return lines[0]


class AgentsSummaryTests(unittest.TestCase):
    """The `agents:` line of the timeline summary, whichever of its lists is empty."""

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-cli-test-"))
        self.home = self.tmp / "home" / "alice"

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def write(self, rel: str, text: str) -> None:
        path = self.home / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")

    def claude_history(self) -> None:
        self.write(
            ".claude/history.jsonl",
            json.dumps({"display": "hello", "timestamp": 1790848800000, "project": "/srv/proj"})
            + "\n",
        )

    def windsurf_state(self) -> None:
        self.write(".windsurf/mcp.json", "{}")

    def test_text(self):
        summary = cli.agents_summary
        self.assertEqual(
            summary(True, ["claude-code", "antigravity"], ["windsurf", "windsurf"], []),
            "parsed: antigravity, claude-code; detected but not parsed: windsurf",
        )
        self.assertEqual(
            summary(True, ["claude-code"], [], []),
            "parsed: claude-code; detected but not parsed: none",
        )
        self.assertEqual(
            summary(True, [], ["windsurf"], []),
            "parsed: none; detected but not parsed: windsurf",
        )
        self.assertEqual(
            summary(True, [], [], ["antigravity"]),
            "parsed: none; detected but not parsed: none; parser found no rows: antigravity",
        )
        self.assertEqual(summary(False, [], [], []), "none were found")
        self.assertEqual(
            summary(True, [], ["windsurf"], [], ["copilot", "copilot", "shared"]),
            "parsed: none; detected but not parsed: windsurf; collected only: copilot, shared",
        )
        self.assertEqual(
            summary(True, [], [], ["antigravity"], ["copilot"]),
            "parsed: none; detected but not parsed: none; collected only: copilot;"
            " parser found no rows: antigravity",
        )

    def test_both_lists(self):
        self.claude_history()
        self.windsurf_state()
        exit_code, out = run_timeline(self.tmp / "home", self.tmp / "out")
        self.assertEqual(exit_code, 0, out)
        self.assertEqual(
            agents_line(out),
            "agents:    parsed: claude-code; detected but not parsed: windsurf",
        )

    def test_collected_only(self):
        # copilot is in NO_TRANSCRIPTS: detected, no parser, not a backlog.
        self.claude_history()
        self.windsurf_state()
        self.write(".config/github-copilot/apps.json", "{}")
        exit_code, out = run_timeline(self.tmp / "home", self.tmp / "out")
        self.assertEqual(exit_code, 0, out)
        self.assertEqual(
            agents_line(out),
            "agents:    parsed: claude-code; detected but not parsed: windsurf;"
            " collected only: copilot",
        )

    def test_nothing_unparsed(self):
        self.claude_history()
        exit_code, out = run_timeline(self.tmp / "home", self.tmp / "out")
        self.assertEqual(exit_code, 0, out)
        self.assertEqual(
            agents_line(out),
            "agents:    parsed: claude-code; detected but not parsed: none",
        )
        self.assertNotIn("only none", out)

    def test_nothing_parsed(self):
        self.windsurf_state()
        exit_code, out = run_timeline(self.tmp / "home", self.tmp / "out")
        self.assertEqual(exit_code, 1, out)  # no rows
        self.assertEqual(
            agents_line(out),
            "agents:    parsed: none; detected but not parsed: windsurf",
        )

    def test_parser_found_no_rows(self):
        # The agent is detected and has a parser, but its files hold nothing:
        # an empty summaries database and an empty conversations directory.
        self.write(".gemini/antigravity-cli/conversation_summaries.db", "")
        (self.home / ".gemini/antigravity-cli/conversations").mkdir()
        self.windsurf_state()
        exit_code, out = run_timeline(self.tmp / "home", self.tmp / "out")
        self.assertEqual(exit_code, 1, out)  # no rows
        self.assertEqual(
            agents_line(out),
            "agents:    parsed: none; detected but not parsed: windsurf;"
            " parser found no rows: antigravity",
        )
        self.assertNotIn("none were found", out)

    def test_agent_left_out_by_filter_is_not_no_rows(self):
        self.claude_history()
        self.write(".gemini/antigravity-cli/conversation_summaries.db", "")
        buffer = io.StringIO()
        with contextlib.redirect_stdout(buffer), contextlib.redirect_stderr(io.StringIO()):
            exit_code = cli.main(
                [
                    "timeline",
                    str(self.tmp / "home"),
                    "-o",
                    str(self.tmp / "out"),
                    "--agent",
                    "claude-code",
                ]
            )
        self.assertEqual(exit_code, 0, buffer.getvalue())
        self.assertEqual(
            agents_line(buffer.getvalue()),
            "agents:    parsed: claude-code; detected but not parsed: none",
        )

    def test_no_agent_data(self):
        self.write("Documents/notes.txt", "not an agent file\n")
        exit_code, out = run_timeline(self.tmp / "home", self.tmp / "out")
        self.assertEqual(exit_code, 1, out)  # no rows
        self.assertEqual(agents_line(out), "agents:    none were found")


def read_jsonl(path: Path) -> list[dict]:
    return [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines()]


class InventoryJsonTests(unittest.TestCase):
    """inventory.json.

    Counts per project and outside any project, and the agent lists, written by `timeline`.
    """

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-inv-json-test-"))

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def timeline(self, root: Path, *extra: str) -> tuple[int, str, Path]:
        out = self.tmp / "out"
        buffer = io.StringIO()
        with contextlib.redirect_stdout(buffer), contextlib.redirect_stderr(io.StringIO()):
            exit_code = cli.main(["timeline", str(root), "-o", str(out), "--host", "h1", *extra])
        return exit_code, buffer.getvalue(), out

    def check_totals(self, out: Path) -> dict:
        inv = json.loads((out / "inventory.json").read_text(encoding="utf-8"))
        rows = read_jsonl(out / "timeline.jsonl")
        sessions = read_jsonl(out / "sessions.jsonl")
        # The version is the first key, so a reader sees it before the counts.
        self.assertEqual(next(iter(inv)), "doubleagent_version")
        self.assertEqual(inv["doubleagent_version"], VERSION)
        self.assertEqual(inv["rows"], len(rows))
        self.assertEqual(inv["sessions"], len(sessions))
        groups = dict(inv["projects"])
        self.assertNotIn("", groups)
        self.assertNotIn("", inv["scratch_directories"])
        groups.update(inv["scratch_directories"])
        groups[""] = inv["outside_projects"]
        # Every row and session lands in exactly one group, and each
        # group's totals are the sum of its agents'.
        self.assertEqual(sum(group["rows"] for group in groups.values()), len(rows))
        self.assertEqual(sum(group["sessions"] for group in groups.values()), len(sessions))
        for path, group in groups.items():
            self.assertEqual(
                sum(counts["rows"] for counts in group["agents"].values()), group["rows"], path
            )
            self.assertEqual(
                sum(counts["sessions"] for counts in group["agents"].values()), group["sessions"]
            )
            for agent, counts in group["agents"].items():
                want = [
                    row for row in rows if row["project_path"] == path and row["agent"] == agent
                ]
                self.assertEqual(sum(counts["turn_types"].values()), len(want), (path, agent))
                for turn_type, count in counts["turn_types"].items():
                    self.assertEqual(count, sum(1 for row in want if row["turn_type"] == turn_type))
        return inv

    def test_full_fixture(self):
        home = self.tmp / "home" / "alice"
        build_home(home)
        (home / ".windsurf").mkdir(exist_ok=True)
        (home / ".windsurf/mcp.json").write_text("{}", encoding="utf-8")
        exit_code, out, path = self.timeline(self.tmp / "home")
        self.assertEqual(exit_code, 0, out)
        inv = self.check_totals(path)
        self.assertIn("/srv/proj", inv["projects"])
        self.assertIn("claude-code", inv["projects"]["/srv/proj"]["agents"])
        self.assertIn("claude-code", inv["parsed"])
        self.assertIn("windsurf", inv["detected_not_parsed"])
        self.assertEqual(inv["detected_not_parsed"], sorted(inv["detected_not_parsed"]))
        # build_home has the Copilot token store, a NO_TRANSCRIPTS agent.
        self.assertIn("copilot", inv["collected_only"])
        self.assertNotIn("copilot", inv["detected_not_parsed"])
        self.assertEqual(inv["collected_only"], sorted(inv["collected_only"]))
        line = next(candidate for candidate in out.splitlines() if candidate.startswith("agents:"))
        self.assertIn("; collected only: copilot", line)
        self.assertNotIn("copilot", line.split("detected but not parsed:")[1].split(";")[0])
        self.assertEqual(list(inv["projects"]), sorted(inv["projects"]))

    def test_with_and_without_project(self):
        home = self.tmp / "home" / "alice"
        (home / ".claude").mkdir(parents=True)
        lines = [
            {"display": "in a project", "timestamp": 1790848800000, "project": "/srv/proj"},
            {"display": "no project", "timestamp": 1790848860000},
        ]
        (home / ".claude/history.jsonl").write_text(
            "".join(json.dumps(record) + "\n" for record in lines), encoding="utf-8"
        )
        exit_code, out, path = self.timeline(self.tmp / "home")
        self.assertEqual(exit_code, 0, out)
        inv = self.check_totals(path)
        self.assertEqual(list(inv["projects"]), ["/srv/proj"])
        self.assertEqual(inv["projects"]["/srv/proj"]["rows"], 1)
        self.assertEqual(inv["outside_projects"]["rows"], 1)
        self.assertEqual(
            inv["outside_projects"]["agents"]["claude-code"]["turn_types"], {"user": 1}
        )
        self.assertEqual(inv["detected_not_parsed"], [])
        self.assertEqual(inv["no_rows"], [])
        self.assertEqual(inv["collected_only"], [])

    def test_filtered(self):
        home = self.tmp / "home" / "alice"
        build_home(home)
        exit_code, out, path = self.timeline(self.tmp / "home", "--match", "zzz-no-such-text")
        self.assertEqual(exit_code, 1, out)  # no rows
        inv = self.check_totals(path)
        self.assertEqual(inv["rows"], 0)
        self.assertEqual(inv["projects"], {})
        self.assertEqual(inv["outside_projects"], EMPTY_OUTSIDE)
        # `parsed` names the agents with rows before filtering, as stdout does.
        self.assertIn("claude-code", inv["parsed"])


# A synthetic catalog for the project tests, so they do not depend on which
# agent the bundled project catalog gives a file: `.pi/approved-plan.md`
# nests inside `.pi`, and `AGENTS.md` is left to `project`.
SYNTHETIC_CATALOG = """# home artifacts (agent|glob relative to home)
claude-code|.claude
pi|.pi

# project artifacts (relative to each discovered project)
claude-code|.claude
project|AGENTS.md
pi|.pi
pi-plans|.pi/approved-plan.md
aider|.aider.*
"""

CLAUDE_LOCAL_SETTINGS = {"permissions": {"allow": ["Bash(make lint)"]}}


class Collected:
    """An extracted collection written row by row.

    Each file goes under fs/ and gets a collected manifest row.
    """

    def __init__(self, root: Path, projects: list[str] | None):
        self.root = root
        (root / "fs").mkdir(parents=True)
        summary: dict = {"hostname": "h1"}
        if projects is not None:
            summary["projects"] = projects
        (root / "collection.json").write_text(json.dumps(summary), encoding="utf-8")
        self.rows: list[dict] = []

    def add(self, user: str, home: str, agent: str, rel: str, text: str) -> None:
        path = home.rstrip("/") + "/" + rel
        disk = self.root / ("fs" + path)
        disk.parent.mkdir(parents=True, exist_ok=True)
        disk.write_text(text, encoding="utf-8")
        self.rows.append(
            {
                "user": user,
                "home": home,
                "agent": agent,
                "path": path,
                "archive_path": "fs" + path,
                "type": "file",
                "status": "collected",
                "size": len(text.encode("utf-8")),
            }
        )

    def write(self) -> Path:
        with open(self.root / "manifest.jsonl", "w", encoding="utf-8") as handle:
            for row in self.rows:
                handle.write(json.dumps(row) + "\n")
        return self.root


class ProjectAttributionTests(unittest.TestCase):
    """Project bases and the agent of their files."""

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-proj-test-"))
        self.cat = catalog.parse(SYNTHETIC_CATALOG)

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_project_agent_for(self):
        project_agent_for = self.cat.project_agent_for
        self.assertEqual(project_agent_for(".claude/settings.local.json"), "claude-code")
        self.assertEqual(project_agent_for("AGENTS.md"), "project")
        self.assertEqual(project_agent_for(".pi/settings.json"), "pi")
        self.assertEqual(
            project_agent_for(".pi/approved-plan.md"), "pi-plans"
        )  # the nested entry wins
        self.assertEqual(project_agent_for(".aider.chat.history.md"), "aider")
        self.assertEqual(project_agent_for("README.md"), "")
        self.assertEqual(project_agent_for(".claude\\settings.json"), "claude-code")

    def test_marker_from_collection_json(self):
        collected = Collected(self.tmp / "col", ["/srv/proj"])
        collected.add("alice", "/home/alice", "claude-code", ".claude/history.jsonl", "")
        collected.add("alice", "/srv/proj", "claude-code", ".claude/settings.local.json", "{}")
        collected.add("alice", "/srv/proj", "project", "AGENTS.md", "# x\n")
        col = open_input(collected.write(), self.cat)
        try:
            marks = {home.original: home.project for home in col.homes}
            agents = {artifact.original: artifact.agent for artifact in col.artifacts}
        finally:
            col.cleanup()
        self.assertEqual(marks, {"/home/alice": False, "/srv/proj": True})
        self.assertEqual(agents["/srv/proj/.claude/settings.local.json"], "claude-code")
        self.assertEqual(agents["/srv/proj/AGENTS.md"], "project")

    def test_old_collection_reattributed(self):
        # A collector before 1.12.0: every project row says `project`, and
        # collection.json has no `projects` key.
        collected = Collected(self.tmp / "col", None)
        collected.add("alice", "/home/alice", "pi", ".pi/agent/settings.json", "{}")
        for rel in (
            ".claude/settings.local.json",
            "AGENTS.md",
            ".pi/approved-plan.md",
            ".pi/notes.txt",
            ".aider.input.history",
            "unknown.txt",
        ):
            collected.add("alice", "/srv/proj", "project", rel, "{}")
        col = open_input(collected.write(), self.cat)
        try:
            marks = {home.original: home.project for home in col.homes}
            agents = {
                artifact.rel: artifact.agent
                for artifact in col.artifacts
                if artifact.home.original == "/srv/proj"
            }
            table = {(row["home"], row["agent"]): row["files"] for row in cli.detect_table(col)}
        finally:
            col.cleanup()
        self.assertEqual(marks, {"/home/alice": False, "/srv/proj": True})
        self.assertEqual(
            agents,
            {
                ".claude/settings.local.json": "claude-code",
                "AGENTS.md": "project",
                ".pi/approved-plan.md": "pi-plans",
                ".pi/notes.txt": "pi",
                ".aider.input.history": "aider",
                "unknown.txt": "project",
            },
        )
        self.assertEqual(
            table,
            {
                ("/home/alice", "pi"): 1,
                ("/srv/proj", "claude-code"): 1,
                ("/srv/proj", "project"): 2,
                ("/srv/proj", "pi-plans"): 1,
                ("/srv/proj", "pi"): 1,
                ("/srv/proj", "aider"): 1,
            },
        )

    def test_routing(self):
        collected = Collected(self.tmp / "col", ["/srv/proj"])
        collected.add(
            "alice",
            "/srv/proj",
            "claude-code",
            ".claude/settings.local.json",
            json.dumps(CLAUDE_LOCAL_SETTINGS),
        )
        collected.add("alice", "/srv/proj", "project", "AGENTS.md", "# x\n")
        col = open_input(collected.write(), self.cat)
        try:
            parsers = by_agent()
            routed = {
                artifact.rel: cli.parsers_for(artifact, parsers) for artifact in col.artifacts
            }
            rows, _, problems, _ = cli.collect_rows(col, Options(), [])
        finally:
            col.cleanup()
        self.assertIs(routed["AGENTS.md"], ALL)
        self.assertEqual(
            {parser.agent for parser in routed[".claude/settings.local.json"]}, {"claude-code"}
        )
        self.assertEqual(problems, [])
        # The project-scoped settings row names the project, as it did when
        # the manifest said `project`.
        self.assertEqual(
            [(row.agent, row.project_path, row.turn_type) for row in rows],
            [("claude-code", "/srv/proj", "system")],
        )


class TimelineProjectStatusTests(unittest.TestCase):
    """`status` of each inventory.json project, and the stdout file lines."""

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-status-test-"))

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def build(self) -> Path:
        collected = Collected(self.tmp / "col", ["/srv/proj", "/srv/empty"])
        history = [
            {"display": "in proj", "timestamp": 1790848800000, "project": "/srv/proj"},
            {"display": "gone", "timestamp": 1790848860000, "project": "/srv/gone"},
            {"display": "nowhere", "timestamp": 1790848920000},
        ]
        collected.add(
            "alice",
            "/home/alice",
            "claude-code",
            ".claude/history.jsonl",
            "".join(json.dumps(record) + "\n" for record in history),
        )
        collected.add("alice", "/srv/proj", "project", "AGENTS.md", "# x\n")
        collected.add("alice", "/srv/empty", "project", "AGENTS.md", "# y\n")
        return collected.write()

    def test_status_values(self):
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(self.build(), out)
        self.assertEqual(exit_code, 0, stdout)
        inv = json.loads((out / "inventory.json").read_text(encoding="utf-8"))
        projects = inv["projects"]
        self.assertEqual(list(projects), ["/srv/empty", "/srv/gone", "/srv/proj"])
        self.assertEqual(
            {k: project["status"] for k, project in projects.items()},
            {"/srv/empty": "collected", "/srv/gone": "referenced", "/srv/proj": "collected"},
        )
        # A collected project with no rows has zero counts; `status` comes
        # first and the shared files last.
        self.assertEqual(
            projects["/srv/empty"],
            {
                "status": "collected",
                "rows": 0,
                "sessions": 0,
                "agents": {},
                "shared_files": 1,
                "synced": "",
                "relayed": "",
                "origin": "",
                "recovered": "",
                "remotes": [],
                "head": "",
                "recorded_remotes": [],
            },
        )
        self.assertEqual(projects["/srv/proj"]["rows"], 1)
        self.assertEqual(projects["/srv/gone"]["rows"], 1)
        self.assertEqual(
            list(projects["/srv/proj"]),
            [
                "status",
                "rows",
                "sessions",
                "agents",
                "shared_files",
                "synced",
                "relayed",
                "origin",
                "recovered",
                "remotes",
                "head",
                "recorded_remotes",
            ],
        )
        self.assertNotIn("status", inv["outside_projects"])
        self.assertEqual(list(inv["outside_projects"])[-4:], PROJECT_KEYS)
        self.assertEqual(inv["outside_projects"]["rows"], 1)
        self.assertEqual(inv["rows"], 3)
        # The home's files are under `homes`, never under outside_projects.
        home = inv["homes"][0]
        self.assertEqual((home["user"], home["home"]), ("alice", "/home/alice"))
        self.assertEqual(
            [(agent["agent"], agent["files"]) for agent in home["agents"]], [("claude-code", 1)]
        )
        self.assertNotIn("files", inv["outside_projects"]["agents"]["claude-code"])

    def test_image_root_removed(self):
        # In image mode collection.json names `<root>/srv/proj`, while the
        # host's transcripts say `/srv/proj`: both are one project.
        strip = cli.strip_image_root
        image = {"mode": "image", "root": "/mnt/image/"}
        self.assertEqual(strip("/mnt/image/srv/proj", image), "/srv/proj")
        self.assertEqual(strip("/mnt/imagex/srv/proj", image), "/mnt/imagex/srv/proj")
        self.assertEqual(strip("/srv/proj", image), "/srv/proj")
        self.assertEqual(
            strip("/mnt/image/srv/proj", {"mode": "live", "root": "/mnt/image"}),
            "/mnt/image/srv/proj",
        )
        self.assertEqual(strip("/srv/proj", {"mode": "live", "root": "/"}), "/srv/proj")

    def test_stdout_names_every_file(self):
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(self.build(), out)
        self.assertEqual(exit_code, 0, stdout)
        lines = stdout.splitlines()
        self.assertIn("rows:      3 in %s" % (out / "timeline.jsonl"), lines)
        self.assertIn("sessions:  0 in %s" % (out / "sessions.jsonl"), lines)
        self.assertIn(
            "inventory: %s (homes, projects, scratch directories, counts per agent, problems)"
            % (out / "inventory.json"),
            lines,
        )
        # A history file and no transcript: nothing to drop, still printed.
        self.assertIn("dropped:   0 duplicate rows from secondary sources", lines)
        heads = [line.split(":")[0] for line in lines if not line.startswith(" ")]
        self.assertEqual(heads[-5:], ["dropped", "agents", "rows", "sessions", "inventory"])
        self.assertNotIn("detect", heads)
        self.assertEqual(
            sorted(path.name for path in out.iterdir()),
            ["inventory.json", "sessions.jsonl", "timeline.jsonl"],
        )


# The rows `timeline` drops from build_home's secondary sources, per agent, because a
# primary store or another secondary source holds the same event; the comment on each
# count names the copies dropped.
FIXTURE_DEDUPLICATED = {
    "agent-zero": 1,  # scheduler tasks.json: the task chat's reply
    "aider": 2,  # input history "rename foo to bar", "/add b.py", at the chat rows' times
    "antigravity": 14,  # history 1; brain mirror 7 (CLI) + 6 (app), within 600 s
    "claude-code": 2,  # .claude and .claude-work history.jsonl
    "claude-desktop": 3,  # audit.jsonl "thanks" and model row, record initialMessage
    "cline": 2,  # session-search.db and user_input_history.jsonl "fix tests"
    "codex-cli": 12,  # history.jsonl 1, thread_history_1.sqlite 8, state_5.sqlite 3
    "continue": 3,  # input_history.json "rename foo", chatFeedback prompt and reply
    "copilot-cli": 6,  # session-store.db 3, rewind snapshot 1, command history 2
    "cursor": 3,  # agent transcript: two prompts and a reply the bubbles hold
    "gemini-cli": 1,  # "list files in src"
    "goose": 1,  # history.txt "run the tests", no timestamp
    "hermes": 11,  # snapshot copies and the divert's screenshot prompt
    "kiro": 1,  # .cli_bash_history "list the files here"
    "letta": 4,  # transcript.jsonl turn: prompt, tool_use, tool_result, reply
    "little-coder": 17,  # v0.0.x session_latest.json 8, history.json 8, input_history 1
    "muse-code": 1,  # tui-history.jsonl "fix the failing test", no timestamp
    "nanobot": 11,  # four WebUI rows, CLI "hello", frozen JSONL 6 (sessions.sqlite3)
    "open-interpreter": 3,  # history.jsonl "list the files", state_5.sqlite 2
    "openclaw": 11,  # .jsonl.deleted.* 1, pre-doctor .bak 7, checkpoint 3
    "opencode": 2,  # opencode-next.db: the imported session's start row and prompt
    "openhands": 1,  # CLI prompt_history.json "list the files"
    "pearai": 4,  # output log and dev_data/chat.jsonl: "add the target" and its reply
    "pi": 4,  # pi-debug.log: prompt, model row, tool_use, tool_result
    "qwen-code": 1,  # "run the tests"
    "roo-code": 2,  # cli-history.json "rename the helper" and "yes", no timestamp
    "shellgpt": 1,  # response cache: the answer the chat file holds
    # completion prompt and choice, threads 7 and 3 in the backups, and
    # each backup's copy of the GitHub integration row
    "tabby": 10,
}


class SyncedSessionTests(unittest.TestCase):
    """The `synced` and `relayed` marks of sessions, projects and the summary.

    `sessions.jsonl` `synced` and `relayed`, the inventory's `synced_sessions` and
    `relayed_sessions` keys and the `sessions:` line.
    """

    UNSYNCED = "5e55a0a1-0000-4000-8000-000000000002"

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-synced-test-"))
        home = build_home(self.tmp / "home" / "alice")
        # A second Copilot CLI session whose `mc_` keys are null: not synced.
        unsynced = home / ".copilot/session-state" / self.UNSYNCED / "workspace.yaml"
        unsynced.parent.mkdir(parents=True)
        text = COPILOT_WORKSPACE_YAML.replace(COPILOT_SESSION, self.UNSYNCED)
        lines = [
            k + ": null" if k.startswith("mc_") else k + ":" + value
            for k, _, value in (line.partition(":") for line in text.splitlines())
        ]
        unsynced.write_text("\n".join(lines) + "\n", encoding="utf-8")
        # An empty log beside it: a session with no events.jsonl is a pool
        # session, whose cwd is a scratch directory.
        (unsynced.parent / "events.jsonl").write_text("", encoding="utf-8")
        # A Goose gateway session, relayed through Telegram.
        connection = sqlite3.connect(str(home / ".local/share/goose/sessions/sessions.db"))
        for sql in goose_gateway_records():
            connection.execute(sql)
        connection.commit()
        connection.close()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_synced(self):
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(self.tmp / "home", out)
        self.assertEqual(exit_code, 0, stdout)
        sessions = read_jsonl(out / "sessions.jsonl")
        for session in sessions:
            self.assertEqual(
                list(session)[-4:], ["synced", "relayed", "origin", "recovered"], session
            )
        by_id = {
            (session["agent"], session["session_id"]): session["synced"] for session in sessions
        }
        self.assertEqual(by_id[("copilot-cli", COPILOT_SESSION)], "yes")
        self.assertEqual(by_id[("copilot-cli", self.UNSYNCED)], "no")
        # Claude Code records Remote Control as sync: the 2.1.309 session had a
        # bridge, the older one did not.
        self.assertEqual(by_id[("claude-code", CLAUDE_309_SESSION)], "yes")
        self.assertEqual(by_id[("claude-code", CLAUDE_SESSION)], "no")
        # `synced` is "" for every agent whose parser records no sync state,
        # and "yes" or "no" for the others.
        sync_agents = {parser.agent for parser in ALL if parser.records_sync}
        self.assertEqual(
            {value for (agent, _), value in by_id.items() if agent not in sync_agents}, {""}
        )
        self.assertEqual(
            {value for (agent, _), value in by_id.items() if agent in sync_agents}, {"yes", "no"}
        )
        synced = sum(1 for value in by_id.values() if value == "yes")
        relayed = sum(1 for session in sessions if session["relayed"] == "yes")
        remote = sum(1 for session in sessions if session["origin"] == "remote")
        recovered = sum(1 for session in sessions if session["recovered"] in ("yes", "partial"))
        keys = ["synced_sessions", "relayed_sessions", "remote_sessions", "recovered_sessions"]
        inv = json.loads((out / "inventory.json").read_text(encoding="utf-8"))
        self.assertEqual(list(inv)[-5:-1], keys)
        self.assertEqual(inv["synced_sessions"], synced)
        self.assertEqual(inv["relayed_sessions"], relayed)
        self.assertEqual(inv["remote_sessions"], remote)
        self.assertEqual(inv["recovered_sessions"], recovered)
        for home in inv["homes"]:
            for agent_entry in home["agents"]:
                self.assertEqual(list(agent_entry)[-4:], keys, agent_entry)
        agents = {agent["agent"]: agent for home in inv["homes"] for agent in home["agents"]}
        self.assertEqual(agents["copilot-cli"]["synced_sessions"], 1)
        # The 2.1.309 transcript's bridge and the registry-only process whose
        # `sessions/<pid>.json` names a bridge id.
        self.assertEqual(agents["claude-code"]["synced_sessions"], 2)
        self.assertEqual(by_id[("claude-code", CLAUDE_BRIDGE_SESSION)], "yes")
        line = next(
            candidate for candidate in stdout.splitlines() if candidate.startswith("sessions:")
        )
        self.assertTrue(
            line.endswith(
                "sessions.jsonl (%d synced, %d relayed, %d remote, %d recovered)"
                % (synced, relayed, remote, recovered)
            ),
            line,
        )

    def test_origin(self):
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(self.tmp / "home", out)
        self.assertEqual(exit_code, 0, stdout)
        by_id = {
            (session["agent"], session["session_id"]): session["origin"]
            for session in read_jsonl(out / "sessions.jsonl")
        }
        # The two Copilot cloud agent tasks began on GitHub's servers, and
        # the two ChatGPT desktop account conversations on chatgpt.com.
        for key, want in (
            (("vscode", "copilot-cloud-agent:/task/" + VSCODE_CLOUD_TASK), "remote"),
            (("vscode", "copilot-cloud-agent:/task/" + VSCODE_OLD_TASK), "remote"),
            (("vscode", VSCODE_SESSION), "local"),
            (("chatgpt-desktop", CHATGPT_CONVERSATION), "remote"),
            (("chatgpt-desktop", CHATGPT_TPP_CONVERSATION), "remote"),
            (("cursor", CURSOR_LIVE), "local"),
            (("cursor", CURSOR_CLOUD), "remote"),
            (("cursor", CURSOR_CLOUD_LISTED), "remote"),
            (("copilot-cli", COPILOT_SESSION), ""),
            # Claude Code records origin; its fixture session is a local CLI one.
            (("claude-code", CLAUDE_SESSION), "local"),
        ):
            self.assertEqual(by_id[key], want, key)
        origin_agents = {parser.agent for parser in ALL if parser.records_origin}
        # Cline records origin; its fixture sessions are local.
        self.assertEqual(
            origin_agents,
            {"vscode", "chatgpt-desktop", "cursor", "agent-zero", "cline", "claude-code"},
        )
        self.assertEqual(
            {value for (agent, _), value in by_id.items() if agent not in origin_agents}, {""}
        )
        self.assertEqual(
            {value for (agent, _), value in by_id.items() if agent == "vscode"}, {"remote", "local"}
        )
        self.assertEqual(
            {value for (agent, _), value in by_id.items() if agent == "chatgpt-desktop"}, {"remote"}
        )
        inv = json.loads((out / "inventory.json").read_text(encoding="utf-8"))
        self.assertEqual(inv["remote_sessions"], 6)
        agents = {agent["agent"]: agent for home in inv["homes"] for agent in home["agents"]}
        self.assertEqual(agents["vscode"]["remote_sessions"], 2)
        self.assertEqual(agents["chatgpt-desktop"]["remote_sessions"], 2)
        self.assertEqual(agents["cursor"]["remote_sessions"], 2)
        self.assertEqual(agents["chatgpt-desktop"]["synced_sessions"], 2)
        self.assertEqual(agents["copilot-cli"]["remote_sessions"], 0)
        line = next(
            candidate for candidate in stdout.splitlines() if candidate.startswith("sessions:")
        )
        self.assertIn(", 6 remote", line)

    def test_recovered(self):
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(self.tmp / "home", out)
        self.assertEqual(exit_code, 0, stdout)
        sessions = read_jsonl(out / "sessions.jsonl")
        by_id = {
            (session["agent"], session["session_id"]): session["recovered"] for session in sessions
        }
        # CODEX_ORPHAN's rollout and AGY_ORPHAN's database are gone, so only
        # secondary sources hold them; COPILOT_SESSION's store keeps turns
        # its events.jsonl lost; AGY_APP_CONVERSATION's primary holds all.
        for key, want in (
            (("codex-cli", CODEX_ORPHAN), "yes"),
            (("antigravity", AGY_ORPHAN), "yes"),
            (("copilot-cli", COPILOT_SESSION), "partial"),
            (("antigravity", AGY_APP_CONVERSATION), "no"),
            # Only `system` rows of a secondary source (a Codex thread record,
            # a Qwen logs.json model row) beside a complete transcript: "no".
            (("codex-cli", CODEX_SESSION), "no"),
            (("open-interpreter", OI_SESSION), "no"),
            (("qwen-code", QWEN_SESSION), "no"),
            # The chat editing state is a secondary source, and
            # holds only `system` rows: "no".
            (("vscode", VSCODE_SESSION), "no"),
            # The scheduler task's logged prompt survives only in tasks.json.
            (("agent-zero", AGENT_ZERO_CHAT), "partial"),
        ):
            self.assertEqual(by_id[key], want, key)
        secondary_agents = {parser.agent for parser in ALL if parser.has_secondary}
        self.assertEqual(
            {value for (agent, _), value in by_id.items() if agent not in secondary_agents}, {""}
        )
        self.assertEqual(
            {value for (agent, _), value in by_id.items() if agent in secondary_agents},
            {"yes", "partial", "no"},
        )
        # Row level: a surviving secondary row is marked, a primary row not.
        rows = read_jsonl(out / "timeline.jsonl")
        for row in rows:
            self.assertEqual(list(row)[-1], "recovered", row)
        orphan = [row for row in rows if row["session_id"] == CODEX_ORPHAN]
        self.assertTrue(orphan)
        # Its events are marked; its `system` rows (the thread record) are not.
        events = [row for row in orphan if row["turn_type"] != "system"]
        self.assertTrue(events)
        self.assertEqual({row["recovered"] for row in events}, {"yes"})
        self.assertEqual({row["recovered"] for row in orphan if row["turn_type"] == "system"}, {""})
        primary = [row for row in rows if row["session_id"] == AGY_APP_CONVERSATION]
        self.assertTrue(primary)
        self.assertEqual({row["recovered"] for row in primary}, {""})
        # Counts: `yes` and `partial` together.
        recovered = sum(1 for value in by_id.values() if value in ("yes", "partial"))
        inv = json.loads((out / "inventory.json").read_text(encoding="utf-8"))
        self.assertEqual(inv["recovered_sessions"], recovered)
        agents = {agent["agent"]: agent for home in inv["homes"] for agent in home["agents"]}
        self.assertEqual(agents["antigravity"]["recovered_sessions"], 2)
        self.assertEqual(agents["vscode"]["recovered_sessions"], 0)
        line = next(
            candidate for candidate in stdout.splitlines() if candidate.startswith("sessions:")
        )
        self.assertTrue(line.endswith(", %d recovered)" % recovered), line)
        # Projects: all, some, none of a secondary-reading agent's sessions,
        # and "" for no sessions or no agent with a secondary source.
        projects = inv["projects"]
        for path, entry in projects.items():
            self.assertEqual(list(entry)[-7:], PROJECT_KEYS + REPO_KEYS, path)
            for name, agent_entry in entry["agents"].items():
                self.assertNotIn("recovered_sessions", agent_entry, (path, name))
        agy = projects["/home/u/proj"]
        self.assertEqual(agy["recovered"], "partial")
        self.assertEqual(projects["/sessions/quiet-river-1234"]["recovered"], "yes")
        # nanobot's workspace is inside its own state directory, a scratch
        # directory by the catalog rule. Its memory journal's
        # sessions are not recovered; the cron and trigger run records'
        # session, gone from every store, is.
        self.assertNotIn("/alice/.nanobot/workspace", projects)
        scratch = inv["scratch_directories"]
        self.assertEqual(scratch["/alice/.nanobot/workspace"]["recovered"], "partial")
        self.assertEqual(projects["/a0/usr/projects/demo"]["recovered"], "partial")
        self.assertEqual(projects["/srv/proj2"]["recovered"], "")
        # The recovered sessions with no project: AGY_ORPHAN is one.
        outside = inv["outside_projects"]
        self.assertEqual(list(outside)[-4:], PROJECT_KEYS)
        self.assertEqual(outside["recovered"], "partial")
        self.assertNotIn("recovered_sessions", outside)
        self.assertNotIn("recovered_sessions", outside["agents"]["antigravity"])

    def test_project_marks(self):
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(self.tmp / "home", out)
        self.assertEqual(exit_code, 0, stdout)
        projects = json.loads((out / "inventory.json").read_text(encoding="utf-8"))["projects"]

        def marks(path):
            return [projects[path][k] for k in PROJECT_KEYS]

        # Agent Zero: the Telegram chat is synced and relayed, the other chat
        # neither; both are of local origin, and the other chat keeps a
        # scheduler prompt only tasks.json holds.
        self.assertEqual(marks("/a0/usr/projects/demo"), ["partial", "partial", "local", "partial"])
        self.assertNotIn("synced_sessions", projects["/a0/usr/projects/demo"])
        # /srv/proj holds VS Code sessions, none of remote origin.
        self.assertEqual(marks("/srv/proj"), ["partial", "partial", "local", "partial"])
        # Letta records sync and Goose relay, neither marks the /srv/old session.
        self.assertEqual(marks("/srv/old")[:2], ["no", "no"])
        # No session, and agents that record none of the marks.
        self.assertEqual(marks("/srv/proj2"), ["", "", "", ""])
        # The two Copilot cloud agent tasks and the two ChatGPT desktop account
        # conversations have no project path, so they are summarised outside
        # projects, beside local VS Code sessions without one and synced
        # ones: origin "mixed", synced "partial" by the rules.
        outside = json.loads((out / "inventory.json").read_text(encoding="utf-8"))[
            "outside_projects"
        ]
        self.assertEqual([outside[k] for k in PROJECT_KEYS], ["partial", "no", "mixed", "partial"])
        self.assertNotIn("remote_sessions", outside)
        self.assertNotIn("remote_sessions", outside["agents"]["vscode"])

    def test_project_mark_values(self):
        def session(origin):
            summary = SessionSummary("h", "u", "vscode", origin)
            summary.origin = origin
            return summary

        mark = next(entry for entry in cli.PROJECT_MARKS if entry[0] == "origin")
        remote, local = session("remote"), session("local")
        self.assertEqual(cli.project_mark([remote, remote], mark, {"vscode"}), "remote")
        self.assertEqual(cli.project_mark([remote, local], mark, {"vscode"}), "mixed")
        self.assertEqual(cli.project_mark([local], mark, {"vscode"}), "local")
        self.assertEqual(cli.project_mark([session("")], mark, {"copilot-cli"}), "")
        self.assertEqual(cli.project_mark([], mark, {"vscode"}), "")

    def test_recovered_ignores_parser_rows(self):
        # A secondary file with a bad line beside a complete primary session:
        # its `parser:` row survives deduplication but is no lost event.
        def row(source, turn_type, text):
            return Row(
                host="h",
                user="u",
                agent="claude-code",
                session_id="s",
                turn_type=turn_type,
                text=text,
                timestamp_utc="2026-10-01T10:00:00.000Z",
                source_file=source,
            )

        primary = [row("/t.jsonl", "user", "run the tests")]
        bad = row("/history.jsonl", "system", "parser: unparseable line (bad JSON): {")
        secondary = [(row("/history.jsonl", "user", "run the tests"), 2.0), (bad, 2.0)]
        kept, dropped = deduplicate(primary, secondary)
        self.assertEqual(dropped, {"claude-code": 1})
        self.assertIn(bad, kept)
        self.assertEqual([row.recovered for row in kept], ["", ""])
        agents = frozenset({"claude-code"})
        got = summarise(kept, frozenset(), frozenset(), frozenset(), agents)
        self.assertEqual([session.recovered for session in got], ["no"])
        # A session whose only rows are `parser:` rows is never "yes".
        bad.recovered = "yes"
        only = summarise([bad], frozenset(), frozenset(), frozenset(), agents)
        self.assertEqual([session.recovered for session in only], ["no"])
        self.assertEqual([session.recovered for session in summarise([bad])], [""])

    def test_relayed(self):
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(self.tmp / "home", out)
        self.assertEqual(exit_code, 0, stdout)
        sessions = read_jsonl(out / "sessions.jsonl")
        by_id = {
            (session["agent"], session["session_id"]): session["relayed"] for session in sessions
        }
        # The Agent Zero Telegram chat, the Goose gateway session, the two
        # Qwen Code channel routes, the three nanobot sessions keyed to or
        # routed through a messaging channel, the OpenClaw Telegram
        # conversation and the Letta channel route are
        # relayed; the other sessions of those agents are not. Crush and
        # little-coder record relay, but no fixture
        # session of theirs is relayed: little-coder's row names no session.
        for key, want in (
            (("agent-zero", AGENT_ZERO_TG_CHAT), "yes"),
            (("agent-zero", AGENT_ZERO_CHAT), "no"),
            (("goose", GOOSE_GATEWAY_SESSION), "yes"),
            (("goose", GOOSE_SESSION), "no"),
            (("qwen-code", QWEN_SESSION), "yes"),
            (("qwen-code", QWEN_DAEMON_SESSION), "yes"),
            (("qwen-code", QWEN_ARCHIVED), "no"),
            (("hermes", HERMES_SESSION), "no"),
            (("nanobot", NANOBOT_KEY), "yes"),
            (("nanobot", NANOBOT_DATABASE_KEY), "yes"),
            (("nanobot", "unified:default"), "yes"),
            (("nanobot", "websocket:chat-1"), "no"),
            (("openclaw", OPENCLAW_SESSION), "yes"),
            (("openclaw", OPENCLAW_RESET_SESSION), "no"),
            (("letta", "conv-9f"), "yes"),
            (("letta", "local-conv-1"), "no"),
            (("crush", "6f1c0001"), "no"),
            (("little-coder", "c0ffee12"), "no"),
            (("copilot-cli", COPILOT_SESSION), ""),
            (("claude-code", CLAUDE_SESSION), ""),
        ):
            self.assertEqual(by_id[key], want, key)
        relay_agents = {parser.agent for parser in ALL if parser.records_relay}
        self.assertEqual(
            relay_agents,
            {
                "agent-zero",
                "crush",
                "goose",
                "hermes",
                "letta",
                "little-coder",
                "nanobot",
                "openclaw",
                "qwen-code",
            },
        )
        self.assertEqual(
            {value for (agent, _), value in by_id.items() if agent not in relay_agents}, {""}
        )
        self.assertEqual(
            {value for (agent, _), value in by_id.items() if agent in relay_agents}, {"yes", "no"}
        )
        inv = json.loads((out / "inventory.json").read_text(encoding="utf-8"))
        self.assertEqual(inv["relayed_sessions"], 9)
        agents = {agent["agent"]: agent for home in inv["homes"] for agent in home["agents"]}
        self.assertEqual(agents["agent-zero"]["relayed_sessions"], 1)
        self.assertEqual(agents["goose"]["relayed_sessions"], 1)
        self.assertEqual(agents["qwen-code"]["relayed_sessions"], 2)
        self.assertEqual(agents["nanobot"]["relayed_sessions"], 3)
        self.assertEqual(agents["openclaw"]["relayed_sessions"], 1)
        self.assertEqual(agents["letta"]["relayed_sessions"], 1)
        self.assertEqual(agents["crush"]["relayed_sessions"], 0)
        self.assertEqual(agents["little-coder"]["relayed_sessions"], 0)
        self.assertEqual(agents["copilot-cli"]["relayed_sessions"], 0)
        line = next(
            candidate for candidate in stdout.splitlines() if candidate.startswith("sessions:")
        )
        self.assertIn(", 9 relayed", line)

    def test_synced_codex_and_cline(self):
        # A `/feedback` report marks a Codex or Open Interpreter thread, and
        # `enterprise.blobUpload` or the upload queue a Cline session, as
        # synced.
        build_cline_blob_upload(self.tmp / "home" / "alice")
        build_cline_sync_queue(self.tmp / "home" / "alice")
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(self.tmp / "home", out)
        self.assertEqual(exit_code, 0, stdout)
        by_id = {
            (session["agent"], session["session_id"]): session["synced"]
            for session in read_jsonl(out / "sessions.jsonl")
        }
        for key, synced in (
            (("codex-cli", CODEX_SESSION), "yes"),
            (("codex-cli", CODEX_ORPHAN), "no"),
            (("open-interpreter", OI_SESSION), "yes"),
            (("cline", CLINE_BLOB_SESSION), "yes"),
            (("cline", CLINE_BLOB_DELETED), "yes"),
            (("cline", CLINE_SESSION), "no"),
            (("cline", CLINE_TASK), "yes"),
            (("cline", CLINE_QUEUE_PENDING), "no"),
            (("cline", CLINE_QUEUE_FAILED), "no"),
            (("continue", CONTINUE_SESSION), ""),
            (("qwen-code", QWEN_SESSION), ""),
        ):
            self.assertEqual(by_id[key], synced, key)

    def test_summarise_values(self):
        rows = [
            Row(host="h", user="u", agent="claude-code", session_id="s", turn_type="user"),
            Row(host="h", user="u", agent="copilot-cli", session_id="c", turn_type="user"),
        ]
        got = {
            session.agent: session.synced for session in summarise(rows, frozenset({"copilot-cli"}))
        }
        self.assertEqual(got, {"claude-code": "", "copilot-cli": "no"})
        self.assertEqual({session.synced for session in summarise(rows)}, {""})
        got = {
            session.agent: (session.synced, session.relayed)
            for session in summarise(rows, frozenset(), frozenset({"copilot-cli"}))
        }
        self.assertEqual(got, {"claude-code": ("", ""), "copilot-cli": ("", "no")})
        self.assertEqual({session.relayed for session in summarise(rows)}, {""})
        got = {
            session.agent: session.origin
            for session in summarise(rows, frozenset(), frozenset(), frozenset({"copilot-cli"}))
        }
        self.assertEqual(got, {"claude-code": "", "copilot-cli": "local"})
        self.assertEqual({session.origin for session in summarise(rows)}, {""})
        remote = Row(
            host="h",
            user="u",
            agent="copilot-cli",
            session_id="c",
            turn_type="system",
            text="session origin: remote GitHub cloud agent",
        )
        got = {session.agent: session.origin for session in summarise([*rows, remote])}
        self.assertEqual(got, {"claude-code": "", "copilot-cli": "remote"})
        relayed = Row(
            host="h",
            user="u",
            agent="copilot-cli",
            session_id="c",
            turn_type="system",
            text="session relayed: channel telegram",
        )
        got = {session.agent: session.relayed for session in summarise([*rows, relayed])}
        self.assertEqual(got, {"claude-code": "", "copilot-cli": "yes"})


class SyncedSessionOpenClawAgentZeroTests(unittest.TestCase):
    """`synced` for the OpenClaw share and the Agent Zero responses.

    `synced` for the OpenClaw public share and the Agent Zero stored responses, and none from the
    Hermes and Goose upload rows.
    """

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-synced-153-test-"))
        build_home(self.tmp / "home" / "alice")

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_synced(self):
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(self.tmp / "home", out)
        self.assertEqual(exit_code, 0, stdout)
        by_id = {
            (session["agent"], session["session_id"]): session["synced"]
            for session in read_jsonl(out / "sessions.jsonl")
        }
        self.assertEqual(by_id[("openclaw", OPENCLAW_SESSION)], "yes")
        self.assertEqual(by_id[("openclaw", OPENCLAW_RESET_SESSION)], "no")
        self.assertEqual(by_id[("agent-zero", AGENT_ZERO_TG_CHAT)], "yes")
        self.assertEqual(by_id[("agent-zero", AGENT_ZERO_CHAT)], "no")
        self.assertEqual(by_id[("hermes", HERMES_SESSION)], "")
        self.assertEqual(
            {value for (agent, _), value in by_id.items() if agent in ("hermes", "goose")}, {""}
        )
        inv = json.loads((out / "inventory.json").read_text(encoding="utf-8"))
        agents = {agent["agent"]: agent for home in inv["homes"] for agent in home["agents"]}
        self.assertEqual(agents["openclaw"]["synced_sessions"], 1)
        self.assertEqual(agents["agent-zero"]["synced_sessions"], 1)
        self.assertEqual(agents["hermes"]["synced_sessions"], 0)
        # The Telegram chat is relayed as well as synced; the build_home
        # fixture has no Goose gateway session, so Goose counts none.
        relayed = {
            (session["agent"], session["session_id"]): session["relayed"]
            for session in read_jsonl(out / "sessions.jsonl")
        }
        self.assertEqual(relayed[("agent-zero", AGENT_ZERO_TG_CHAT)], "yes")
        self.assertEqual(relayed[("agent-zero", AGENT_ZERO_CHAT)], "no")
        # Hermes records relay; the CLI fixture session is not one.
        self.assertEqual(relayed[("hermes", HERMES_SESSION)], "no")
        self.assertEqual(agents["agent-zero"]["relayed_sessions"], 1)
        self.assertEqual(agents["goose"]["relayed_sessions"], 0)
        self.assertEqual(agents["qwen-code"]["relayed_sessions"], 2)
        # The OpenClaw Telegram conversation and the Letta
        # channel route are relayed too.
        self.assertEqual(agents["openclaw"]["relayed_sessions"], 1)
        self.assertEqual(agents["letta"]["relayed_sessions"], 1)
        self.assertEqual(agents["nanobot"]["relayed_sessions"], 3)
        self.assertEqual(inv["relayed_sessions"], 8)


class SyncedSessionB3Tests(unittest.TestCase):
    """`sessions.jsonl` `synced` for the OpenCode, Kilo Code, pi, Letta and VS Code sync rows.

    One fixture session of each says "yes", one without the marker "no".
    """

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-synced-b3-test-"))
        build_home(self.tmp / "home" / "alice")

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_synced(self):
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(self.tmp / "home", out)
        self.assertEqual(exit_code, 0, stdout)
        by_id = {
            (session["agent"], session["session_id"]): session["synced"]
            for session in read_jsonl(out / "sessions.jsonl")
        }
        for key, want in (
            (("opencode", OPENCODE_SHARED), "yes"),
            (("opencode", OPENCODE_PRIVATE), "no"),
            (("kilo-code", KILO_SESSION), "yes"),
            (("kilo-code", KILO_TASK), "no"),
            (("pi", PI_BUG_SESSION), "yes"),
            (("pi", PI_SESSION), "no"),
            (("letta", LETTA_CONV), "yes"),
            (("letta", LETTA_LOCAL_AGENT_CONV), "no"),
            (("vscode", VSCODE_SESSION), "yes"),
            (("vscode", VSCODE_LEGACY_SESSION), "no"),
            (("vscode", "copilot-cloud-agent:/task/" + VSCODE_CLOUD_TASK), "yes"),
            (("vscode", "copilot-cloud-agent:/task/" + VSCODE_OLD_TASK), "yes"),
        ):
            self.assertEqual(by_id[key], want, key)
        agents = {parser.agent for parser in ALL if parser.records_sync}
        self.assertLessEqual({"opencode", "kilo-code", "pi", "letta", "vscode"}, agents)


class DeduplicationTimelineTests(unittest.TestCase):
    """`timeline` drops secondary-source rows a primary store also records.

    `timeline` drops secondary-source rows that a primary store also records, and counts them on
    stdout and in inventory.json.
    """

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-dedup-cli-test-"))
        build_home(self.tmp / "home" / "alice")

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def timeline(self, *extra: str) -> tuple[str, list[dict], dict]:
        out = self.tmp / "out"
        buffer = io.StringIO()
        with contextlib.redirect_stdout(buffer), contextlib.redirect_stderr(io.StringIO()):
            exit_code = cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), *extra])
        self.assertEqual(exit_code, 0, buffer.getvalue())
        inv = json.loads((out / "inventory.json").read_text(encoding="utf-8"))
        return buffer.getvalue(), read_jsonl(out / "timeline.jsonl"), inv

    def test_history_line_dropped(self):
        stdout, rows, inv = self.timeline()
        text = "delete the logs in /var/log please"
        same = {
            row["source_file"]
            for row in rows
            if row["agent"] == "claude-code" and row["text"] == text
        }
        # The session's own user row survives (and the fork that copies the
        # prompt, two seconds later); the history line is gone.
        self.assertIn("/alice/.claude/projects/-srv-proj/%s.jsonl" % CLAUDE_SESSION, same)
        self.assertEqual([source for source in same if source.endswith("history.jsonl")], [])
        # The orphan history line, whose session file is gone, is kept.
        orphan = [
            row for row in rows if row["text"] == "an older prompt whose session file is gone"
        ]
        self.assertEqual(len(orphan), 1)
        self.assertTrue(orphan[0]["source_file"].endswith("/.claude/history.jsonl"))
        self.assertEqual(inv["deduplicated"], FIXTURE_DEDUPLICATED)
        self.assertEqual(
            list(inv)[-7:],
            [
                "deduplicated",
                "collected_only",
                "synced_sessions",
                "relayed_sessions",
                "remote_sessions",
                "recovered_sessions",
                "scratch_directories",
            ],
        )
        self.assertEqual(inv["rows"], len(rows))
        total = sum(FIXTURE_DEDUPLICATED.values())
        self.assertIn(
            "dropped:   %d duplicate rows from secondary sources (%s)"
            % (total, ", ".join("%s %d" % pair for pair in sorted(FIXTURE_DEDUPLICATED.items()))),
            stdout.splitlines(),
        )

    def test_kept_secondary_rows(self):
        """Rows that only a secondary source holds stay beside the primary rows.

        The Antigravity CLI history's "now the tests" (the conversation database holds only
        "clean the logs"), the Gemini logs.json prompt a rewind removed from the session and
        the Copilot CLI prompt the cut log lost stay; the Muse prompt history's undated copy
        matches on text alone and leaves the transcript's row.
        """
        _, rows, _ = self.timeline()
        texts = {(row["agent"], row["text"]) for row in rows if row["turn_type"] == "user"}
        for kept in (
            ("antigravity", "now the tests"),
            ("gemini-cli", "delete everything in /srv/proj"),
            ("muse-code", "fix the failing test"),
            ("copilot-cli", "and run lint"),
        ):
            self.assertIn(kept, texts)

    def test_agent_counts_only_that_agent(self):
        stdout, _, inv = self.timeline("--agent", "claude-code")
        self.assertEqual(inv["deduplicated"], {"claude-code": 2})
        self.assertIn(
            "dropped:   2 duplicate rows from secondary sources (claude-code 2)",
            stdout.splitlines(),
        )

    def test_row_filter_after_rule(self):
        # --match applies to the kept rows and leaves the count unchanged;
        # the dropped: line follows the filtered: line.
        stdout, rows, inv = self.timeline("--match", "logs")
        self.assertEqual(inv["deduplicated"], FIXTURE_DEDUPLICATED)
        heads = [line.split(":")[0] for line in stdout.splitlines() if not line.startswith(" ")]
        self.assertEqual(heads[heads.index("filtered") + 1 : heads.index("agents")], ["dropped"])
        text = "delete the logs in /var/log please"
        same = [
            row["source_file"]
            for row in rows
            if row["agent"] == "claude-code" and row["text"] == text
        ]
        self.assertTrue(same)
        self.assertEqual([source for source in same if source.endswith("history.jsonl")], [])


class WindowsProjectPathTests(unittest.TestCase):
    """One Windows project folder under several spellings is one project.

    One Windows project folder recorded under several spellings is one `project_path` and one
    inventory.json project.
    """

    # The spellings a Windows Antigravity install recorded (CLI 1.3.5 and
    # app 2.23.1) and the folders collection.json listed.
    SPELLINGS = ("c:/Users/sean/demo", "C:/users/sean/demo2", "C:\\users\\sean\\demo2")
    COLLECTED = ("C:\\Users\\sean\\demo", "C:\\users\\sean\\demo2")

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-winpath-test-"))

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_canonical_project_path(self):
        cases = {
            "c:/Users/sean/demo": "C:\\Users\\sean\\demo",
            "C:\\users\\sean\\demo2": "C:\\users\\sean\\demo2",
            "C:/users//sean\\\\demo2": "C:\\users\\sean\\demo2",
            "C:\\Users\\sean\\demo\\": "C:\\Users\\sean\\demo",
            "c:/Users/sean/demo/": "C:\\Users\\sean\\demo",
            "C:\\": "C:\\",
            "C:/": "C:\\",
            "c:\\\\": "C:\\",
            "\\\\?\\C:\\x": "C:\\x",
            "\\\\?\\c:/x/": "C:\\x",
            "\\\\?\\UNC\\srv\\share\\x": "\\\\srv\\share\\x",
            "\\\\?\\unc\\srv\\share": "\\\\srv\\share",
            "\\\\srv/share//x/": "\\\\srv\\share\\x",
            "\\\\srv\\Share\\X": "\\\\srv\\Share\\X",
            # Not Windows forms: unchanged.
            "/srv/a\\b": "/srv/a\\b",
            "/srv/proj/": "/srv/proj/",
            "//srv/share": "//srv/share",
            "C:": "C:",
            "C:relative": "C:relative",
            "\\\\": "\\\\",
            "": "",
        }
        for raw, want in cases.items():
            self.assertEqual(canonical_project_path(raw), want, raw)
            # Idempotent.
            self.assertEqual(canonical_project_path(want), want, want)

    def test_project_key(self):
        self.assertEqual(project_key("c:/Users/sean/demo"), "c:\\users\\sean\\demo")
        self.assertEqual(project_key("C:\\USERS\\sean\\Demo\\"), "c:\\users\\sean\\demo")
        self.assertEqual(project_key("\\\\?\\UNC\\Srv\\Share"), "\\\\srv\\share")
        # A POSIX path is compared exactly: case and backslashes count.
        self.assertEqual(project_key("/srv/Proj"), "/srv/Proj")
        self.assertEqual(project_key("/srv/a\\b"), "/srv/a\\b")
        self.assertEqual(project_key(""), "")

    def rows(self) -> list[Row]:
        return [
            Row(
                timestamp_utc="2026-10-14T10:00:0%d.000Z" % i,
                agent="antigravity",
                session_id="s%d" % i,
                project_path=path,
                turn_type="user",
            )
            for i, path in enumerate(self.SPELLINGS)
        ]

    def test_inventory_groups_spellings(self):
        rows = self.rows()
        sessions = summarise(rows)
        files = {
            "C:\\Users\\sean\\demo": {
                "agents": {"antigravity": {"files": 2, "bytes": 30}},
                "shared_files": 1,
            }
        }
        out = cli.project_inventory(rows, sessions, list(self.COLLECTED), files)
        self.assertEqual(list(out), ["C:\\Users\\sean\\demo", "C:\\users\\sean\\demo2"])
        demo, demo2 = out["C:\\Users\\sean\\demo"], out["C:\\users\\sean\\demo2"]
        self.assertEqual((demo["status"], demo["rows"], demo["sessions"]), ("collected", 1, 1))
        self.assertEqual((demo2["status"], demo2["rows"], demo2["sessions"]), ("collected", 2, 2))
        self.assertEqual(demo["agents"]["antigravity"]["files"], 2)
        self.assertEqual(demo["shared_files"], 1)
        self.assertEqual(demo2["agents"]["antigravity"]["files"], 0)

    def test_inventory_first_spelling_names_referenced(self):
        rows = [
            Row(agent="a", project_path="D:\\Work\\x", turn_type="user"),
            Row(agent="a", project_path="d:\\work\\X", turn_type="user"),
            Row(agent="a", project_path="/srv/x", turn_type="user"),
            Row(agent="a", project_path="/srv/X", turn_type="user"),
        ]
        out = cli.project_inventory(rows, [], [], {})
        self.assertEqual(list(out), ["/srv/X", "/srv/x", "D:\\Work\\x"])
        self.assertEqual(out["D:\\Work\\x"]["rows"], 2)
        self.assertEqual(out["D:\\Work\\x"]["status"], "referenced")

    def test_timeline_canonical(self):
        # A parser's Windows path comes out canonical in timeline.jsonl and
        # sessions.jsonl, and matches the collected folder.
        collected = Collected(self.tmp / "col", ["c:/Users/sean/demo/"])
        history = [
            {"display": "one", "timestamp": 1790848800000, "project": "c:/Users/sean/demo"},
            {"display": "two", "timestamp": 1790848860000, "project": "C:\\USERS\\sean\\demo"},
            {"display": "posix", "timestamp": 1790848920000, "project": "/srv/a\\b/"},
        ]
        collected.add(
            "sean",
            "/home/sean",
            "claude-code",
            ".claude/history.jsonl",
            "".join(json.dumps(record) + "\n" for record in history),
        )
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(collected.write(), out)
        self.assertEqual(exit_code, 0, stdout)
        paths = [row["project_path"] for row in read_jsonl(out / "timeline.jsonl")]
        self.assertEqual(paths, ["C:\\Users\\sean\\demo", "C:\\USERS\\sean\\demo", "/srv/a\\b/"])
        inv = json.loads((out / "inventory.json").read_text(encoding="utf-8"))
        self.assertEqual(list(inv["projects"]), ["/srv/a\\b/", "C:\\Users\\sean\\demo"])
        demo = inv["projects"]["C:\\Users\\sean\\demo"]
        self.assertEqual((demo["status"], demo["rows"]), ("collected", 2))
        self.assertEqual(inv["projects"]["/srv/a\\b/"]["status"], "referenced")


def run_detect(root: Path, *extra: str) -> tuple[int, str]:
    buffer = io.StringIO()
    with contextlib.redirect_stdout(buffer), contextlib.redirect_stderr(io.StringIO()):
        exit_code = cli.main(["detect", str(root), *extra])
    return exit_code, buffer.getvalue()


AIDER_HISTORY = "# 2026-10-01\n+ hi\n"

# project_agents() for ProjectAgentsTests.build, new or old style.
PROJECT_AGENTS = {
    "/srv/proj": {
        "users": ["alice"],
        "agents": {
            "aider": {"files": 1, "bytes": len(AIDER_HISTORY)},
            "claude-code": {"files": 1, "bytes": 2},
        },
        "shared_files": 1,
        "shared_bytes": 4,
    },
    "/srv/shared": {"users": ["alice"], "agents": {}, "shared_files": 1, "shared_bytes": 9},
}


# A second Codex thread, in a project the collection does not hold.
GONE_SESSION = "aaaaaaaa-bbbb-4ccc-8ddd-000000000253"
GONE_REMOTE = "https://example.com/team/gone.git"


def gone_rollout() -> str:
    """The fixture rollout as a thread of `/srv/gone` whose session_meta names `GONE_REMOTE`."""
    records = codex_rollout_records(cwd="/srv/gone")
    records[0]["payload"]["id"] = GONE_SESSION
    records[0]["payload"]["git"]["repository_url"] = GONE_REMOTE
    for entry in records[1:]:
        if entry["type"] == "turn_context":
            entry["payload"]["cwd"] = "/srv/gone"
    return "".join(json.dumps(record) + "\n" for record in records)


class GitRemotesTests(unittest.TestCase):
    """The repository of each collected project and the remotes the agents recorded.

    The repository of each collected project in `detect` and inventory.json, and the remotes the
    agents recorded.
    """

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-remotes-test-"))

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def build(self) -> Path:
        collected = Collected(self.tmp / "col", ["/srv/proj", "/srv/bare"])
        sessions = ".codex/sessions/2026/10/02/"
        collected.add(
            "alice",
            "/home/alice",
            "codex-cli",
            sessions + "rollout-2026-10-02T09-00-00-%s.jsonl" % CODEX_SESSION,
            "".join(json.dumps(record) + "\n" for record in codex_rollout_records()),
        )
        collected.add(
            "alice",
            "/home/alice",
            "codex-cli",
            sessions + "rollout-2026-10-02T10-00-00-%s.jsonl" % GONE_SESSION,
            gone_rollout(),
        )
        collected.add("alice", "/srv/proj", "project", ".git/config", PROJECT_GIT_CONFIG)
        collected.add("alice", "/srv/proj", "project", ".git/HEAD", PROJECT_GIT_HEAD)
        collected.add("alice", "/srv/bare", "project", "AGENTS.md", "# x\n")
        return collected.write()

    def test_detect_json(self):
        exit_code, out = run_detect(self.build(), "--json")
        self.assertEqual(exit_code, 0, out)
        projects = {project["path"]: project for project in json.loads(out)["projects"]}
        for project in projects.values():
            self.assertEqual(list(project)[-2:], ["remotes", "head"])
        self.assertEqual(projects["/srv/proj"]["remotes"], PROJECT_GIT_REMOTES)
        self.assertEqual(projects["/srv/proj"]["head"], "main")
        # The .git files are shared files, never an agent.
        self.assertEqual(projects["/srv/proj"]["agents"], [])
        self.assertEqual(projects["/srv/proj"]["shared_files"], 2)
        self.assertEqual(
            (projects["/srv/bare"]["remotes"], projects["/srv/bare"]["head"]), ([], "")
        )

    def test_detect_text(self):
        exit_code, out = run_detect(self.build())
        self.assertEqual(exit_code, 0, out)
        lines = [line for line in out.splitlines() if line.lstrip().startswith("remotes:")]
        self.assertEqual(
            lines,
            [
                "  remotes: origin %s %s (push %s), upstream %s"
                % (
                    PROJECT_REMOTE,
                    PROJECT_MIRROR,
                    PROJECT_PUSH_URL,
                    PROJECT_UPSTREAM,
                )
            ],
        )

    def test_inventory(self):
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(self.build(), out)
        self.assertEqual(exit_code, 0, stdout)
        inv = json.loads((out / "inventory.json").read_text(encoding="utf-8"))
        projects = inv["projects"]
        self.assertEqual(sorted(projects), ["/srv/bare", "/srv/gone", "/srv/proj"])
        for path, entry in projects.items():
            self.assertEqual(list(entry)[-3:], REPO_KEYS, path)
        project, gone, bare = projects["/srv/proj"], projects["/srv/gone"], projects["/srv/bare"]
        self.assertEqual(
            (project["remotes"], project["head"], project["recorded_remotes"]),
            (PROJECT_GIT_REMOTES, "main", [PROJECT_REMOTE]),
        )
        # A referenced project: no repository files, but the remote the
        # agent recorded survives the directory.
        self.assertEqual(gone["status"], "referenced")
        self.assertEqual(
            (gone["remotes"], gone["head"], gone["recorded_remotes"]), ([], "", [GONE_REMOTE])
        )
        self.assertEqual((bare["remotes"], bare["head"], bare["recorded_remotes"]), ([], "", []))
        for key in REPO_KEYS:
            self.assertNotIn(key, inv["outside_projects"])

    def test_unreadable_repository_file_is_a_problem(self):
        # An unreadable .git/config is a problem: line on stdout, printed
        # before the counts, and in inventory.json and detect --json, not
        # a project without remotes.
        if os.name == "nt" or not hasattr(os, "geteuid") or os.geteuid() == 0:
            self.skipTest("chmod does not deny access as root or on Windows")
        col = self.build()
        config = col / "fs/srv/proj/.git/config"
        os.chmod(config, 0)
        try:
            out = self.tmp / "out"
            exit_code, stdout = run_timeline(col, out)
            rc2, detect = run_detect(col, "--json")
        finally:
            os.chmod(config, 0o600)
        self.assertEqual((exit_code, rc2), (0, 0), stdout)
        line = [
            candidate
            for candidate in stdout.splitlines()
            if candidate.startswith("problem:   /srv/proj/.git/config: ")
        ]
        self.assertEqual(len(line), 1, stdout)
        self.assertLess(stdout.index("problem:   /srv/proj"), stdout.index("rows:"), stdout)
        inv = json.loads((out / "inventory.json").read_text(encoding="utf-8"))
        self.assertEqual(
            [
                problem
                for problem in inv["problems"]
                if problem.startswith("/srv/proj/.git/config: ")
            ],
            [line[0][len("problem:   ") :]],
        )
        self.assertEqual(
            (inv["projects"]["/srv/proj"]["remotes"], inv["projects"]["/srv/proj"]["head"]),
            ([], "main"),
        )
        det = json.loads(detect)
        self.assertEqual(
            len([problem for problem in det["problems"] if "/.git/config: " in problem]),
            1,
            det["problems"],
        )

    def test_recorded_remotes_follow_the_filters(self):
        out = self.tmp / "out"
        buffer = io.StringIO()
        with contextlib.redirect_stdout(buffer), contextlib.redirect_stderr(io.StringIO()):
            exit_code = cli.main(
                ["timeline", str(self.build()), "-o", str(out), "--match", "list the home"]
            )
        self.assertEqual(exit_code, 0, buffer.getvalue())
        inv = json.loads((out / "inventory.json").read_text(encoding="utf-8"))
        self.assertEqual(inv["projects"]["/srv/proj"]["recorded_remotes"], [])
        self.assertEqual(inv["projects"]["/srv/proj"]["remotes"], PROJECT_GIT_REMOTES)


class ProjectAgentsTests(unittest.TestCase):
    """The agents used on each home and collected project.

    The agents used on each home and collected project, in `detect`, `detect --json` and
    inventory.json; `project` is never shown as an agent.
    """

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-projagents-test-"))
        self.cat = catalog.parse(SYNTHETIC_CATALOG)

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def build(self, new: bool, name: str = "col") -> Path:
        # new: collector 1.12.0, collection.json `projects` and agent-named
        # rows; old: no `projects` key and every project row `project`.
        collected = Collected(self.tmp / name, ["/srv/proj", "/srv/shared"] if new else None)
        collected.add("alice", "/home/alice", "claude-code", ".claude/settings.json", "{}")
        files = [
            ("/srv/proj", "claude-code", ".claude/settings.local.json", "{}"),
            ("/srv/proj", "aider", ".aider.input.history", AIDER_HISTORY),
            ("/srv/proj", "project", "AGENTS.md", "# x\n"),
            ("/srv/shared", "project", "AGENTS.md", "# shared\n"),
        ]
        for home, agent, rel, text in files:
            collected.add("alice", home, agent if new else "project", rel, text)
        return collected.write()

    def agents_of(self, root: Path) -> dict[str, cli.ProjectAgents]:
        col = open_input(root, self.cat)
        try:
            return cli.project_agents(col)
        finally:
            col.cleanup()

    def test_project_agents(self):
        self.assertEqual(self.agents_of(self.build(True)), PROJECT_AGENTS)

    def test_old_collection_agrees(self):
        self.assertEqual(self.agents_of(self.build(False, "old")), PROJECT_AGENTS)

    def test_detect_json(self):
        exit_code, out = run_detect(self.build(True), "--json")
        self.assertEqual(exit_code, 0, out)
        doc = json.loads(out)
        self.assertEqual(
            list(doc),
            [
                "input",
                "kind",
                "host",
                "notes",
                "homes",
                "projects",
                "problems",
            ],
        )
        self.assertEqual(
            doc["homes"],
            [
                {
                    "user": "alice",
                    "home": "/home/alice",
                    "inferred": False,
                    "agents": [{"agent": "claude-code", "files": 1, "bytes": 2, "parser": True}],
                }
            ],
        )
        self.assertEqual(
            doc["projects"],
            [
                {
                    "path": "/srv/proj",
                    "users": ["alice"],
                    "agents": [
                        {"agent": "aider", "files": 1, "bytes": 18, "parser": True},
                        {"agent": "claude-code", "files": 1, "bytes": 2, "parser": True},
                    ],
                    "shared_files": 1,
                    "shared_bytes": 4,
                    "remotes": [],
                    "head": "",
                },
                {
                    "path": "/srv/shared",
                    "users": ["alice"],
                    "agents": [],
                    "shared_files": 1,
                    "shared_bytes": 9,
                    "remotes": [],
                    "head": "",
                },
            ],
        )
        # --files still names every file with its attributed agent.
        exit_code, out = run_detect(self.build(True, "col2"), "--json", "--files")
        doc = json.loads(out)
        self.assertEqual(list(doc)[-1], "files")
        self.assertIn(
            {"user": "alice", "agent": "project", "path": "/srv/shared/AGENTS.md"}, doc["files"]
        )

    def test_detect_text(self):
        root = self.build(True)
        exit_code, out = run_detect(root)
        self.assertEqual(exit_code, 0, out)
        lines = out.splitlines()
        # The header lines and the first table.
        homes_fmt = "%-14s %-32s %-18s %8s %12s  %s"
        self.assertEqual(
            lines[:7],
            [
                "input:   %s" % root,
                "kind:    collected",
                "host:    h1",
                "",
                homes_fmt % ("user", "home", "agent", "files", "bytes", "parser"),
                homes_fmt % ("alice", "/home/alice", "claude-code", 1, 2, "yes"),
                "",
            ],
        )
        fmt = "%-40s %-14s %-18s %8s %12s  %s"
        head = fmt % ("project", "users", "agent", "files", "bytes", "parser")
        self.assertIn(head, lines)
        index = lines.index(head)
        self.assertEqual(lines[index - 1], "")
        self.assertEqual(
            lines[index + 1 : index + 5],
            [
                fmt % ("/srv/proj", "alice", "aider", 1, 18, "yes"),
                fmt % ("", "", "claude-code", 1, 2, "yes"),
                (fmt % ("", "", "(shared)", 1, 4, "")).rstrip(),
                (fmt % ("/srv/shared", "alice", "(shared)", 1, 9, "")).rstrip(),
            ],
        )
        self.assertEqual(len(lines), index + 5)
        # The first table holds the user home only; `project` is no agent.
        self.assertNotIn("/srv/", "\n".join(lines[:index]))
        self.assertNotIn(" project ", out)

    def test_detect_text_first_table(self):
        # The first table has one line per user, home
        # and agent in the order the first file was met, which is not the
        # sorted order of homes[].agents.
        collected = Collected(self.tmp / "col", ["/srv/proj"])
        collected.add("alice", "/home/alice", "codex-cli", ".codex/config.toml", "x=1\n")
        collected.add("bob", "/home/bob", "claude-code", ".claude/settings.json", "{}")
        collected.add("alice", "/srv/proj", "project", "AGENTS.md", "# x\n")
        collected.add("alice", "/home/alice", "aider", ".aider.conf.yml", "x\n")
        root = collected.write()
        exit_code, out = run_detect(root)
        self.assertEqual(exit_code, 0, out)
        fmt = "%-14s %-32s %-18s %8s %12s  %s"
        lines = out.splitlines()
        index = lines.index(fmt % ("user", "home", "agent", "files", "bytes", "parser"))
        self.assertEqual(lines[index - 1], "")
        self.assertEqual(
            lines[index + 1 : index + 5],
            [
                fmt % ("alice", "/home/alice", "codex-cli", 1, 4, "yes"),
                fmt % ("bob", "/home/bob", "claude-code", 1, 2, "yes"),
                fmt % ("alice", "/home/alice", "aider", 1, 2, "yes"),
                "",
            ],
        )
        exit_code, json_output = run_detect(root, "--json")
        homes = json.loads(json_output)["homes"]
        self.assertEqual([home["user"] for home in homes], ["alice", "bob"])
        self.assertEqual([agent["agent"] for agent in homes[0]["agents"]], ["aider", "codex-cli"])

    def test_detect_text_without_projects(self):
        collected = Collected(self.tmp / "col", [])
        collected.add("alice", "/home/alice", "claude-code", ".claude/settings.json", "{}")
        exit_code, out = run_detect(collected.write())
        self.assertEqual(exit_code, 0, out)
        self.assertNotIn("project", out)

    def test_detect_json_matches_inventory(self):
        root = self.build(True)
        # A file missing from fs/ is a problem met while opening the input;
        # an aider history the parser cannot read is a parser problem.
        manifest = root / "manifest.jsonl"
        gone = {
            "user": "alice",
            "home": "/home/alice",
            "agent": "claude-code",
            "path": "/home/alice/.claude/gone.json",
            "archive_path": "fs/home/alice/.claude/gone.json",
            "type": "file",
            "status": "collected",
            "size": 1,
        }
        with open(manifest, "a", encoding="utf-8") as handle:
            handle.write(json.dumps(gone) + "\n")
        exit_code, out = run_detect(root, "--json")
        self.assertEqual(exit_code, 0, out)
        doc = json.loads(out)
        aider = by_agent()["aider"][0]
        denied = PermissionError(13, "Permission denied")
        with mock.patch.object(aider, "parse", side_effect=denied):
            exit_code, stdout = run_timeline(root, self.tmp / "out")
        self.assertEqual(exit_code, 1, stdout)  # no rows; inventory.json is written anyway
        inv = json.loads((self.tmp / "out" / "inventory.json").read_text(encoding="utf-8"))
        self.assertEqual(
            list(inv),
            [
                "doubleagent_version",
                "input",
                "kind",
                "host",
                "notes",
                "problems",
                "rows",
                "sessions",
                "parsed",
                "detected_not_parsed",
                "no_rows",
                "homes",
                "projects",
                "outside_projects",
                "deduplicated",
                "collected_only",
                "synced_sessions",
                "relayed_sessions",
                "remote_sessions",
                "recovered_sessions",
                "scratch_directories",
            ],
        )
        for key in ("input", "kind", "host", "notes"):
            self.assertEqual(inv[key], doc[key], key)
        # inventory.json's homes are detect's plus `synced_sessions`
        # and `relayed_sessions`, `remote_sessions` and `recovered_sessions`.
        for home in inv["homes"]:
            for agent in home["agents"]:
                agent.pop("recovered_sessions")
                self.assertEqual(agent.pop("remote_sessions"), 0)
                self.assertEqual(agent.pop("relayed_sessions"), 0)
                self.assertEqual(agent.pop("synced_sessions"), 0)
        self.assertEqual(inv["homes"], doc["homes"])
        self.assertEqual(inv["kind"], "collected")
        self.assertEqual(
            doc["problems"], ["/home/alice/.claude/gone.json: missing from the collection"]
        )
        self.assertEqual(
            inv["problems"],
            doc["problems"] + ["/srv/proj/.aider.input.history: permission denied"],
        )
        self.assertFalse((self.tmp / "out" / "detect.json").exists())

    def timeline_inventory(self, root: Path, want_exit_code: int = 0) -> dict:
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(root, out)
        self.assertEqual(exit_code, want_exit_code, stdout)
        return json.loads((out / "inventory.json").read_text(encoding="utf-8"))

    def test_inventory(self):
        collected = Collected(self.tmp / "col", ["/srv/proj", "/srv/shared"])
        history = [
            {"display": "in proj", "timestamp": 1790848800000, "project": "/srv/proj"},
            {"display": "gone", "timestamp": 1790848860000, "project": "/srv/gone"},
        ]
        collected.add(
            "alice",
            "/home/alice",
            "claude-code",
            ".claude/history.jsonl",
            "".join(json.dumps(record) + "\n" for record in history),
        )
        collected.add("alice", "/srv/proj", "windsurf", ".windsurf/rules/x.md", "x\n")
        collected.add("alice", "/srv/proj", "project", "AGENTS.md", "# x\n")
        collected.add("alice", "/srv/shared", "project", "AGENTS.md", "# y\n")
        inv = self.timeline_inventory(collected.write())
        self.assertEqual(
            list(inv),
            [
                "doubleagent_version",
                "input",
                "kind",
                "host",
                "notes",
                "problems",
                "rows",
                "sessions",
                "parsed",
                "detected_not_parsed",
                "no_rows",
                "homes",
                "projects",
                "outside_projects",
                "deduplicated",
                "collected_only",
                "synced_sessions",
                "relayed_sessions",
                "remote_sessions",
                "recovered_sessions",
                "scratch_directories",
            ],
        )
        self.assertEqual(inv["problems"], [])
        projects = inv["projects"]
        for path, entry in projects.items():
            self.assertEqual(
                list(entry),
                [
                    "status",
                    "rows",
                    "sessions",
                    "agents",
                    "shared_files",
                    "synced",
                    "relayed",
                    "origin",
                    "recovered",
                    "remotes",
                    "head",
                    "recorded_remotes",
                ],
                path,
            )
        project = projects["/srv/proj"]
        # Rows and no files; files and no rows.
        self.assertEqual(
            project["agents"]["claude-code"],
            {
                "rows": 1,
                "sessions": 0,
                "turn_types": {"user": 1},
                "files": 0,
                "bytes": 0,
            },
        )
        self.assertEqual(
            project["agents"]["windsurf"],
            {
                "rows": 0,
                "sessions": 0,
                "turn_types": {},
                "files": 1,
                "bytes": 2,
            },
        )
        self.assertEqual(project["shared_files"], 1)
        self.assertEqual(
            projects["/srv/gone"],
            {
                "status": "referenced",
                "rows": 1,
                "sessions": 0,
                "agents": {
                    "claude-code": {
                        "rows": 1,
                        "sessions": 0,
                        "turn_types": {"user": 1},
                        "files": 0,
                        "bytes": 0,
                    }
                },
                "shared_files": 0,
                "synced": "",
                "relayed": "",
                "origin": "",
                "recovered": "",
                "remotes": [],
                "head": "",
                "recorded_remotes": [],
            },
        )
        self.assertEqual(
            projects["/srv/shared"],
            {
                "status": "collected",
                "rows": 0,
                "sessions": 0,
                "agents": {},
                "shared_files": 1,
                "synced": "",
                "relayed": "",
                "origin": "",
                "recovered": "",
                "remotes": [],
                "head": "",
                "recorded_remotes": [],
            },
        )
        self.assertEqual(inv["outside_projects"], EMPTY_OUTSIDE)
        # windsurf is in no home, only in a project: still detected.
        self.assertEqual(inv["detected_not_parsed"], ["windsurf"])
        # A history file and no transcript: nothing to drop.
        self.assertEqual(inv["deduplicated"], {})

    def test_outside_projects_has_no_file_counts(self):
        collected = Collected(self.tmp / "col", [])
        history = [{"display": "nowhere", "timestamp": 1790848920000}]
        collected.add(
            "alice",
            "/home/alice",
            "claude-code",
            ".claude/history.jsonl",
            "".join(json.dumps(record) + "\n" for record in history),
        )
        inv = self.timeline_inventory(collected.write())
        self.assertEqual(inv["projects"], {})
        self.assertEqual(
            inv["outside_projects"]["agents"]["claude-code"],
            {
                "rows": 1,
                "sessions": 0,
                "turn_types": {"user": 1},
            },
        )
        # The history line has no session, so every mark reads "".
        outside = inv["outside_projects"]
        self.assertEqual([outside[k] for k in PROJECT_KEYS], ["", "", "", ""])

    def test_inventory_image_root(self):
        collected = Collected(self.tmp / "col", ["/mnt/image/srv/proj"])
        summary = {
            "hostname": "h1",
            "mode": "image",
            "root": "/mnt/image",
            "projects": ["/mnt/image/srv/proj"],
        }
        (collected.root / "collection.json").write_text(json.dumps(summary), encoding="utf-8")
        collected.add("alice", "/mnt/image/srv/proj", "windsurf", ".windsurf/rules/x.md", "x\n")
        inv = self.timeline_inventory(collected.write(), want_exit_code=1)  # no rows
        self.assertEqual(list(inv["projects"]), ["/srv/proj"])
        self.assertEqual(
            inv["projects"]["/srv/proj"]["agents"]["windsurf"],
            {
                "rows": 0,
                "sessions": 0,
                "turn_types": {},
                "files": 1,
                "bytes": 2,
            },
        )

    def test_loose_has_none(self):
        home = self.tmp / "home" / "alice"
        (home / ".claude").mkdir(parents=True)
        (home / ".claude/settings.json").write_text("{}", encoding="utf-8")
        self.assertEqual(self.agents_of(self.tmp / "home"), {})
        exit_code, out = run_detect(self.tmp / "home", "--json")
        self.assertEqual(exit_code, 0, out)
        self.assertEqual(json.loads(out)["projects"], [])


# Synthetic ids and paths for the scratch directory tests.
POOL_A = "9a9a9a9a-0000-4000-8000-000000000001"
POOL_B = "9a9a9a9a-0000-4000-8000-000000000002"
CHAT = "9a9a9a9a-0000-4000-8000-000000000003"
INSTALL_DIR = "C:\\Users\\sean\\AppData\\Local\\Programs\\GitHub Copilot"
CODEX_SCRATCH = "/home/u/Documents/Codex/2026-10-18/w"
COPILOT_CHAT_DIR = "/home/u/.copilot/chats/" + CHAT
COPILOT_CLONE = "/home/u/.copilot/repos/demo"
SCRATCH_ENTRY_KEYS = [
    "status",
    "rows",
    "sessions",
    "agents",
    "shared_files",
    "synced",
    "relayed",
    "origin",
    "recovered",
    "remotes",
    "head",
    "recorded_remotes",
]


def pool_workspace(session_id: str, cwd: str) -> str:
    """A Copilot Desktop pool session's workspace.yaml: never prompted."""
    return (
        "id: %s\ncwd: %s\nclient_name: github/autopilot\nuser_named: false\n"
        "summary_count: 0\nfork_count: 0\ncreated_at: 2026-10-17T10:00:00.000Z\n"
        % (session_id, cwd)
    )


class ScratchDirectoryTests(unittest.TestCase):
    """inventory.json `scratch_directories`.

    The agent's own working directories, kept apart from `projects`.
    """

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-scratch-test-"))

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def files(self) -> list[tuple[str, str, str]]:
        """(agent, path relative to the home, text) of every file."""
        rollout = CODEX_DESKTOP_ROLLOUT % CODEX_SESSION
        records = codex_rollout_records(cwd=CODEX_SCRATCH)
        state = {"thread-projectless-output-directories": {CODEX_SESSION: CODEX_SCRATCH}}
        chat_events = [line.replace("/srv/proj", COPILOT_CHAT_DIR) for line in COPILOT_EVENT_LINES]
        chat_workspace = COPILOT_WORKSPACE_YAML.replace("/srv/proj", COPILOT_CHAT_DIR)
        chat_workspace = chat_workspace.replace(COPILOT_SESSION, CHAT)
        history = {"display": "in proj", "timestamp": 1790848800000, "project": "/srv/proj"}
        session_dir = ".copilot/session-state/%s/"
        return [
            # (a) A projectless Codex thread: the rollout's cwd is the
            # directory the global state names.
            ("codex-cli", rollout, "".join(json.dumps(record) + "\n" for record in records)),
            ("codex-cli", CODEX_GLOBAL_STATE_REL, json.dumps(state)),
            # (b) Two pool sessions in the install directory, spelled in
            # two cases.
            (
                "copilot-cli",
                session_dir % POOL_A + "workspace.yaml",
                pool_workspace(POOL_A, INSTALL_DIR),
            ),
            (
                "copilot-cli",
                session_dir % POOL_B + "workspace.yaml",
                pool_workspace(POOL_B, INSTALL_DIR.lower()),
            ),
            # (c) A prompted Copilot session in the app's own chat directory.
            ("copilot-cli", session_dir % CHAT + "events.jsonl", "\n".join(chat_events)),
            ("copilot-cli", session_dir % CHAT + "workspace.yaml", chat_workspace),
            # (d) An ordinary project.
            ("claude-code", ".claude/history.jsonl", json.dumps(history) + "\n"),
        ]

    def collected(self) -> Path:
        collected = Collected(self.tmp / "col", ["/srv/proj", CODEX_SCRATCH, COPILOT_CLONE])
        for agent, rel, text in self.files():
            collected.add("u", "/home/u", agent, rel, text)
        return collected.write()

    def loose(self) -> Path:
        home = self.tmp / "u"  # the input is the home itself: no original path
        for _agent, rel, text in self.files():
            path = home / rel
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(text, encoding="utf-8")
        return home

    def inventory(self, root: Path) -> tuple[dict, str]:
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(root, out)
        self.assertEqual(exit_code, 0, stdout)
        return json.loads((out / "inventory.json").read_text(encoding="utf-8")), stdout

    def check_sums(self, inv: dict) -> None:
        groups = [*inv["projects"].values(), *inv["scratch_directories"].values()]
        groups.append(inv["outside_projects"])
        self.assertEqual(sum(group["rows"] for group in groups), inv["rows"])
        self.assertEqual(sum(group["sessions"] for group in groups), inv["sessions"])
        keys = {project_key(k) for k in inv["scratch_directories"]}
        self.assertFalse(keys & {project_key(k) for k in inv["projects"]})

    def test_collected(self):
        inv, stdout = self.inventory(self.collected())
        self.assertEqual(list(inv)[-1], "scratch_directories")
        self.check_sums(inv)
        scratch = inv["scratch_directories"]
        # The first spelling seen names the install directory's group.
        self.assertEqual(
            sorted(scratch), sorted([CODEX_SCRATCH, COPILOT_CHAT_DIR, COPILOT_CLONE, INSTALL_DIR])
        )
        self.assertEqual(list(inv["projects"]), ["/srv/proj"])
        # A clone the collector listed and no row names: the catalog rule.
        self.assertEqual(
            (scratch[COPILOT_CLONE]["status"], scratch[COPILOT_CLONE]["rows"]), ("collected", 0)
        )
        for path, entry in scratch.items():
            self.assertEqual(list(entry), SCRATCH_ENTRY_KEYS, path)
        # Listed by the collector as a project, still a scratch directory.
        self.assertEqual(scratch[CODEX_SCRATCH]["status"], "collected")
        self.assertEqual(scratch[CODEX_SCRATCH]["sessions"], 1)
        self.assertEqual(scratch[COPILOT_CHAT_DIR]["status"], "referenced")
        self.assertEqual(scratch[COPILOT_CHAT_DIR]["sessions"], 1)
        pool = scratch[INSTALL_DIR]
        self.assertEqual((pool["status"], pool["sessions"]), ("referenced", 2))
        self.assertEqual(pool["agents"]["copilot-cli"]["turn_types"], {"system": 4})
        self.assertEqual(inv["projects"]["/srv/proj"]["status"], "collected")
        self.assertIn(
            "inventory: %s (homes, projects, scratch directories, counts per agent, problems)"
            % (self.tmp / "out" / "inventory.json"),
            stdout.splitlines(),
        )
        # The rows keep their project_path in the timeline.
        rows = read_jsonl(self.tmp / "out" / "timeline.jsonl")
        scratch_rows = [row for row in rows if row["text"].startswith("scratch directory:")]
        self.assertIn(
            (
                CODEX_SCRATCH,
                "scratch directory: %s (projectless thread %s) (from %s"
                " thread-projectless-output-directories, file mtime)"
                % (CODEX_SCRATCH, CODEX_SESSION, CODEX_GLOBAL_STATE_REL),
            ),
            [(row["project_path"], row["text"]) for row in scratch_rows],
        )
        self.assertEqual(len(scratch_rows), 3)

    def test_loose_parser_rule_only(self):
        # Without a home path on the source host the catalog rule cannot
        # apply, so the chat directory is a project; the parser rows still
        # mark the other two.
        inv, _ = self.inventory(self.loose())
        self.check_sums(inv)
        self.assertEqual(sorted(inv["scratch_directories"]), sorted([CODEX_SCRATCH, INSTALL_DIR]))
        self.assertEqual(sorted(inv["projects"]), sorted([COPILOT_CHAT_DIR, "/srv/proj"]))
        self.assertEqual(inv["scratch_directories"][CODEX_SCRATCH]["status"], "referenced")

    def test_catalog_rule(self):
        cat = catalog.parse(
            SYNTHETIC_CATALOG
            + "\n# home artifacts (agent|glob relative to home)\ncursor|.config/[Cc]ursor\n"
        )
        claims = cat.claims_home_path
        self.assertEqual(claims(".claude/projects/x"), "claude-code")
        self.assertEqual(claims(".claude"), "claude-code")
        self.assertEqual(claims(".config/Cursor/x"), "cursor")
        self.assertEqual(claims(".config/myproject"), "")
        self.assertEqual(claims(".Claude/x"), "")
        self.assertEqual(claims("\\.CLAUDE\\x", windows=True), "claude-code")
        scratch = cli.home_scratch_key
        self.assertTrue(scratch("/home/u/.claude/x", "/home/u", cat))
        self.assertTrue(scratch("/home/u/.claude", "/home/u/", cat))
        self.assertFalse(scratch("/home/u", "/home/u", cat))
        self.assertFalse(scratch("/home/user/.claude", "/home/u", cat))
        self.assertFalse(scratch("/home/u/.config/myproject", "/home/u", cat))
        windows_home = "C:\\Users\\sean"
        self.assertTrue(scratch(project_key("C:\\Users\\Sean\\.Claude\\x"), windows_home, cat))
        self.assertTrue(scratch(project_key("c:/users/sean/.config/CURSOR"), windows_home, cat))
        self.assertFalse(scratch(project_key("C:\\Users\\sean\\src\\app"), windows_home, cat))


# An agent's own worktrees inside a project, one per agent
# whose source makes them there, beside paths the rule must leave alone.
WORKTREE_PROJECT = "/srv/proj"
AGENT_WORKTREES = [
    "/srv/proj/.claude/worktrees/feat",
    "/srv/proj/.codex/worktrees/c1",
    "/srv/proj/.gemini/worktrees/g1",
    "/srv/proj/.kilo/worktrees/k1",
    "/srv/proj/.qwen/worktrees/q1",
    "C:\\Users\\u\\src\\app\\.Kilo\\worktrees\\k2",
]
NOT_WORKTREES = ["/srv/proj/src", "/srv/proj/.agents/x", "/srv/proj/.kilo"]


class ProjectEntryScratchTests(unittest.TestCase):
    """The project catalog entry rule for `scratch_directories`.

    A path under a directory one agent's project catalog entry names inside a project is that
    agent's own working directory.
    """

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-worktree-test-"))

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def history(self) -> str:
        """Claude Code history lines naming every POSIX path above."""
        paths = [WORKTREE_PROJECT, *AGENT_WORKTREES[:-1], *NOT_WORKTREES]
        return "".join(
            json.dumps({"display": "p%d" % i, "timestamp": 1790848800000 + i, "project": path})
            + "\n"
            for i, path in enumerate(paths)
        )

    def inventory(self, root: Path) -> dict:
        out = self.tmp / "out"
        exit_code, stdout = run_timeline(root, out)
        self.assertEqual(exit_code, 0, stdout)
        return json.loads((out / "inventory.json").read_text(encoding="utf-8"))

    def test_rule(self):
        cat = catalog.load()
        scratch = cli.project_scratch_key
        for path in AGENT_WORKTREES:
            self.assertTrue(scratch(project_key(path), cat), path)
        for path in NOT_WORKTREES:
            self.assertFalse(scratch(project_key(path), cat), path)
        # The entry needs a project before it.
        self.assertFalse(scratch("/.claude/worktrees/x", cat))
        # POSIX paths are compared exactly.
        self.assertFalse(scratch("/srv/proj/.Claude/worktrees/x", cat))
        synthetic = catalog.parse(SYNTHETIC_CATALOG)
        self.assertTrue(scratch("/srv/p/.pi/x", synthetic))
        self.assertFalse(scratch("/srv/p/AGENTS.md/x", synthetic))

    def test_collected(self):
        collected = Collected(self.tmp / "col", [WORKTREE_PROJECT, AGENT_WORKTREES[-1]])
        collected.add("u", "/home/u", "claude-code", ".claude/history.jsonl", self.history())
        inv = self.inventory(collected.write())
        scratch = inv["scratch_directories"]
        self.assertEqual(sorted(scratch), sorted(AGENT_WORKTREES))
        self.assertEqual(sorted(inv["projects"]), sorted([WORKTREE_PROJECT, *NOT_WORKTREES]))
        # Listed by the collector only: the rule tests its project list too.
        windows = scratch[AGENT_WORKTREES[-1]]
        self.assertEqual((windows["status"], windows["rows"]), ("collected", 0))
        claude = scratch[AGENT_WORKTREES[0]]
        self.assertEqual((claude["status"], claude["rows"]), ("referenced", 1))

    def test_loose(self):
        # The rule needs no home path, so it holds when the input is the
        # home itself.
        home = self.tmp / "u"
        (home / ".claude").mkdir(parents=True)
        (home / ".claude" / "history.jsonl").write_text(self.history(), encoding="utf-8")
        inv = self.inventory(home)
        self.assertEqual(sorted(inv["scratch_directories"]), sorted(AGENT_WORKTREES[:-1]))
        self.assertEqual(sorted(inv["projects"]), sorted([WORKTREE_PROJECT, *NOT_WORKTREES]))


if __name__ == "__main__":
    unittest.main()
