"""Tests of the Aider parser."""

import unittest

from test_errors import CAN_DENY, DENY_REASON

from doubleagent import cli
from doubleagent.inputs import open_input
from doubleagent.parsers import Options
from doubleagent.parsers.aider import (
    HEADER_PATTERN,
    INPUT_TIMESTAMP_PATTERN,
    LLM_PATTERN,
    AiderParser,
    read_input_history,
)
from fixtures import build_aider, write_bad_line

from .base import ParserBase


class AiderTests(ParserBase):
    """Test the Aider parser."""

    def test_marker_lines_take_ascii_digits_and_space_or_tab(self):
        lines = (
            (HEADER_PATTERN, "# aider chat started at 2026-10-01 08:00:00"),
            (INPUT_TIMESTAMP_PATTERN, "# 2026-10-01 08:00:00.123456"),
            (LLM_PATTERN, "TO LLM 2026-10-01T08:00:00"),
        )
        for pattern, line in lines:
            self.assertTrue(pattern.match(line + " \t"), line)
            for space in map(chr, (0x00A0, 0x2028, 0x0085)):
                self.assertIsNone(pattern.match(line + space), repr(line + space))
            self.assertIsNone(pattern.match(line.replace("0", chr(0x0660))), line)
            self.assertIsNone(pattern.match(line.replace("1", chr(0x0661))), line)

    def test_input_history_line_of_unicode_space_is_reported(self):
        path = self.tmp / "input.history"
        for space in map(chr, (0x00A0, 0x2028, 0x0085)):
            path.write_text("# 2026-10-01 08:00:00\n+hi\n%s\n \t\n" % space, encoding="utf-8")
            errors: list = []
            self.assertEqual(read_input_history(path, errors), [(1, "2026-10-01 08:00:00", "hi")])
            self.assertEqual(
                errors, [(3, "unparseable line (not a FileHistory line)", space)], repr(space)
            )

    CHAT = ".aider.chat.history.md"
    SESSION1 = "/alice/.aider.chat.history.md#2026-10-02 12:00:00"
    SESSION2 = "/alice/.aider.chat.history.md#2026-10-02 13:00:00"

    def test_secondary(self):
        # The input and LLM histories are secondary sources, the chat history
        # is the primary store.
        parser = AiderParser()
        self.assertFalse(parser.secondary(self.artifact(self.CHAT)))
        self.assertTrue(parser.secondary(self.artifact(".aider.input.history")))
        self.assertTrue(parser.secondary(self.artifact(".aider.llm.history")))

    def test_deduplicated(self):
        """Drop the input-history rows the chat history timed.

        The one whose chat line is gone and the LLM rows are kept.
        """
        rows, _, problems, deduplicated = cli.collect_rows(self.col, Options(), ["aider"])
        self.assertEqual(problems, [])
        self.assertEqual(deduplicated, {"aider": 2})
        users = [row for row in rows if row.turn_type == "user"]
        self.assertEqual(
            [(row.text, row.timestamp_utc, row.session_id, row.source_file) for row in users],
            [
                (
                    "rename foo to bar",
                    "2026-10-02T12:00:05.123Z",
                    self.SESSION1,
                    "/alice/" + self.CHAT,
                ),
                ("/add b.py", "2026-10-02T13:00:02.000Z", self.SESSION2, "/alice/" + self.CHAT),
                ("<blank>", "2026-10-02T13:00:02.000Z", self.SESSION2, "/alice/" + self.CHAT),
                (
                    "/model claude-opus-4-5",
                    "2026-10-02T13:00:02.000Z",
                    self.SESSION2,
                    "/alice/" + self.CHAT,
                ),
                (
                    "delete the migrations folder",
                    "2026-10-02T13:05:00.000Z",
                    self.SESSION2,
                    "/alice/.aider.input.history",
                ),
            ],
        )
        self.assertEqual(sum(row.source_file == "/alice/.aider.llm.history" for row in rows), 2)

    def test_rows(self):
        all_rows = self.rows_for(AiderParser(), self.CHAT)
        for row in all_rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.project_path),
                ("h1", "alice", "aider", "/alice"),
            )
            self.assertEqual((row.git_branch, row.tool_use_id), ("", ""))
        # the model and permission rows are tested below
        rows = [
            row for row in all_rows if not row.text.startswith(("model: ", "permission decision: "))
        ]
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "tool_result",
                "user",
                "assistant",
                "tool_result",
                "system",
                "tool_result",
                "user",
                "tool_result",
                "user",
                "user",
                "tool_result",
            ],
        )
        self.assertTrue(all(row.model == "" for row in rows))
        start, announce, user, asst, tool, _start2, _announce2, add, added, blank = rows[:10]
        self.assertEqual(
            [row.session_id for row in rows], [self.SESSION1] * 5 + [self.SESSION2] * 7
        )
        self.assertEqual(start.timestamp_utc, "2026-10-02T12:00:00.000Z")
        self.assertTrue(announce.text.startswith("Aider v0.86.1\nMain model: "))
        # the prompt takes its time from the matching input-history entry, and later rows inherit it
        self.assertEqual(
            (user.text, user.timestamp_utc), ("rename foo to bar", "2026-10-02T12:00:05.123Z")
        )
        self.assertEqual(asst.timestamp_utc, "2026-10-02T12:00:05.123Z")
        self.assertTrue(asst.text.startswith("Here is the change:\n\na.py\n<<<<<<< SEARCH"))
        self.assertTrue(asst.text.endswith(">>>>>>> REPLACE"))
        self.assertEqual(
            tool.text,
            "Applied edit to a.py\nCommit abc1234 refactor: rename foo to bar\npytest -q\n"
            "Run shell command? (Y)es/(N)o/(D)on't ask again [Yes]: n",
        )
        self.assertEqual((add.text, add.timestamp_utc), ("/add b.py", "2026-10-02T13:00:02.000Z"))
        self.assertEqual(added.text, "Added b.py to the chat")
        self.assertEqual(blank.text, "<blank>")
        self.assertEqual(
            [row.source_line for row in rows], [2, 4, 9, 11, 20, 25, 27, 31, 32, 34, 36, 37]
        )

    def test_model_rows(self):
        """Give a `model:` row per role at the first announcement and on each change."""
        rows = [
            row for row in self.rows_for(AiderParser(), self.CHAT) if row.text.startswith("model: ")
        ]
        self.assertEqual(
            [(row.turn_type, row.text, row.model, row.source_line, row.session_id) for row in rows],
            [
                (
                    "system",
                    "model: anthropic/claude-sonnet-4-5",
                    "anthropic/claude-sonnet-4-5",
                    5,
                    self.SESSION1,
                ),
                (
                    "system",
                    "model: anthropic/claude-haiku-4-5 (weak model)",
                    "anthropic/claude-haiku-4-5",
                    6,
                    self.SESSION1,
                ),
                # a new session starts afresh; `Model:` means no separate weak model
                ("system", "model: gpt-4o", "gpt-4o", 28, self.SESSION2),
                # the /model switch re-announces
                (
                    "system",
                    "model: gpt-4o -> claude-opus-4-5",
                    "claude-opus-4-5",
                    38,
                    self.SESSION2,
                ),
                (
                    "system",
                    "model: claude-sonnet-4-5 (editor model)",
                    "claude-sonnet-4-5",
                    39,
                    self.SESSION2,
                ),
                (
                    "system",
                    "model: gpt-4o -> claude-haiku-4-5 (weak model)",
                    "claude-haiku-4-5",
                    40,
                    self.SESSION2,
                ),
            ],
        )
        self.assertEqual(rows[3].timestamp_utc, "2026-10-02T13:00:02.000Z")

    def test_model_line_outside_announcement(self):
        """A `Weak model:` line is read only beside a main-model line."""
        path = self.home / self.CHAT
        with open(path, "a", encoding="utf-8") as handle:
            handle.write("\n#### what is the weak model?  \n> Weak model: not-an-announcement  \n")
        rows = self.rows_for(AiderParser(), self.CHAT)
        self.assertFalse(any("not-an-announcement" in row.text for row in rows if row.model))
        self.assertEqual(sum(row.text.startswith("model: ") for row in rows), 6)

    def test_permission_rows(self):
        """Give each confirm_ask line a `permission decision:` row after its result.

        A y or n answer may come from --yes-always.
        """
        rows = self.rows_for(AiderParser(), self.CHAT)
        index = [i for i, row in enumerate(rows) if row.text.startswith("permission decision: ")]
        self.assertEqual(
            [(rows[i].text, rows[i].source_line, rows[i].session_id) for i in index],
            [
                (
                    "permission decision: no (Run shell command?) answer=n default=Yes;"
                    " may be automatic (--yes-always)",
                    23,
                    self.SESSION1,
                ),
                (
                    "permission decision: no, don't ask again (Add URL to the chat?)"
                    " answer=d default=Yes",
                    43,
                    self.SESSION2,
                ),
            ],
        )
        for i in index:
            self.assertEqual(rows[i].turn_type, "system")
        self.assertEqual(rows[index[0]].timestamp_utc, "2026-10-02T12:00:05.123Z")
        self.assertEqual(rows[index[0] - 1].turn_type, "tool_result")

    def test_thinking_opt_in(self):
        # Aider records no reasoning; the flag must not change the rows.
        self.assertEqual(
            len(self.rows_for(AiderParser(), self.CHAT, include_thinking=True)),
            len(self.rows_for(AiderParser(), self.CHAT)),
        )

    def test_input_and_llm_history(self):
        rows = self.rows_for(AiderParser(), ".aider.input.history")
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc, row.session_id) for row in rows],
            [
                ("user", "rename foo to bar", "2026-10-02T12:00:05.123Z", self.SESSION1),
                ("user", "/add b.py", "2026-10-02T13:00:02.000Z", self.SESSION2),
                ("user", "delete the migrations folder", "2026-10-02T13:05:00.000Z", self.SESSION2),
            ],
        )
        self.assertEqual({row.project_path for row in rows}, {"/alice"})
        rows = self.rows_for(AiderParser(), ".aider.llm.history")
        self.assertEqual([row.turn_type for row in rows], ["system", "assistant"])
        self.assertEqual(
            rows[0].text,
            "TO LLM: 2 message(s) (SYSTEM 1, USER 1); last user message: rename foo to bar",
        )
        self.assertEqual(
            (rows[1].text, rows[1].timestamp_utc),
            ("Here is the change:\n\na.py", "2026-10-02T12:00:09.000Z"),
        )
        self.assertTrue(all(row.session_id == self.SESSION1 for row in rows))

    def test_config_not_wanted(self):
        art = next(artifact for artifact in self.col.artifacts if artifact.rel == ".aider.conf.yml")
        self.assertFalse(AiderParser().wants(art))

    def test_truncated_line(self):
        path = self.home / ".aider.input.history"
        write_bad_line(path)
        with open(path, "a", encoding="utf-8") as handle:
            handle.write("\n# 2026-10-02 13:06:00.000000\n+after the cut\n")
        rows = self.rows_for(AiderParser(), ".aider.input.history")
        self.assertEqual(
            [row.turn_type for row in rows], ["user", "user", "user", "user", "system"]
        )
        self.assertEqual(
            (rows[2].text, rows[3].text), ("delete the migrations folder", "after the cut")
        )
        self.assertEqual(rows[-1].source_line, 10)
        self.assertEqual(
            rows[-1].text,
            'parser: unparseable line (not a FileHistory line): {"type": "user", "truncated": tr',
        )

    def test_project_artifacts_from_manifest(self):
        """Keep agent aider on repository files the collector attributes to `project`.

        The collector gives those files the project as the home.
        """
        import json as _json

        root = self.tmp / "collected"
        build_aider(root / "fs/srv/proj")
        (root / "collection.json").write_text(_json.dumps({"hostname": "h2"}), encoding="utf-8")
        with open(root / "manifest.jsonl", "w", encoding="utf-8") as handle:
            for name in (
                ".aider.chat.history.md",
                ".aider.input.history",
                ".aider.llm.history",
                ".aider.conf.yml",
            ):
                handle.write(
                    _json.dumps(
                        {
                            "user": "alice",
                            "home": "/srv/proj",
                            "agent": "project",
                            "path": "/srv/proj/" + name,
                            "archive_path": "fs/srv/proj/" + name,
                            "type": "file",
                            "status": "collected",
                            "secret": name.endswith(".yml"),
                        }
                    )
                    + "\n"
                )
        col = open_input(root, self.cat)
        self.assertEqual(col.kind, "collected")
        # Agent `project` rows are re-attributed from the bundled project
        # catalog, and their base is marked as a project.
        self.assertEqual(
            {artifact.rel: artifact.agent for artifact in col.artifacts if artifact.home.project},
            {
                rel: self.cat.project_agent_for(rel) or "project"
                for rel in (
                    ".aider.chat.history.md",
                    ".aider.input.history",
                    ".aider.llm.history",
                    ".aider.conf.yml",
                )
            },
        )
        rows, _, problems, deduplicated = cli.collect_rows(col, Options(), [])
        self.assertEqual(problems, [])
        # 12 chat rows, 6 model and 2 permission rows, 3 input-history rows less
        # the 2 the chat rows repeat, 2 LLM rows.
        self.assertEqual(len(rows), 12 + 6 + 2 + 3 - 2 + 2)
        self.assertEqual(deduplicated, {"aider": 2})
        self.assertEqual(
            {(row.host, row.user, row.agent, row.project_path) for row in rows},
            {("h2", "alice", "aider", "/srv/proj")},
        )
        self.assertEqual(
            {row.session_id for row in rows},
            {
                "/srv/proj/.aider.chat.history.md#2026-10-02 12:00:00",
                "/srv/proj/.aider.chat.history.md#2026-10-02 13:00:00",
            },
        )
        self.assertFalse(
            AiderParser().wants(
                next(artifact for artifact in col.artifacts if artifact.rel == ".aider.conf.yml")
            )
        )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_input_history_is_one_row(self):
        parser = AiderParser()
        target = ".aider.input.history"
        others = [
            artifact
            for artifact in self.col.artifacts
            if parser.wants(artifact) and artifact.rel != target
        ]
        locked = self.home / target
        locked.chmod(0)
        try:
            rows = self.rows_for(parser, target)
            later = [row for artifact in others for row in parser.parse(artifact, Options())]
        finally:
            locked.chmod(0o600)
        reported = [row for row in rows if row.text.startswith("parser:")]
        self.assertEqual(
            [
                (row.turn_type, row.text, row.source_line, row.timestamp_utc, row.session_id)
                for row in reported
            ],
            [
                (
                    "system",
                    "parser: %s could not be read (Permission denied)" % locked.name,
                    0,
                    "",
                    "",
                )
            ],
        )
        self.assertEqual(
            reported[0].project_path, str(self.artifact(target).original).rsplit("/", 1)[0]
        )
        self.assertEqual(len(rows), 1)
        self.assertTrue(any(not row.text.startswith("parser:") for row in later))
