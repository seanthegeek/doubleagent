"""Tests of the Cursor parser."""

import json
import os
import sqlite3
from typing import ClassVar

from doubleagent import cli
from doubleagent.model import summarise
from doubleagent.parsers import Options
from doubleagent.parsers.cursor import (
    TAG_PATTERN,
    WRAPPER_PATTERN,
    CursorParser,
    github_repository,
    run_mode,
    tag_time,
    unwrap_query,
    uri_path,
)
from fixtures import (
    CURSOR_ARCHIVED,
    CURSOR_ATTEMPT_REL,
    CURSOR_CLOUD,
    CURSOR_CLOUD_BODY,
    CURSOR_CLOUD_LISTED,
    CURSOR_CLOUD_REPO,
    CURSOR_DELETED,
    CURSOR_DELETED_TRANSCRIPT_REL,
    CURSOR_DRAFT,
    CURSOR_EMPTY_HEAD,
    CURSOR_LIVE,
    CURSOR_MCP_CALL,
    CURSOR_MTIME,
    CURSOR_SEARCH_REL,
    CURSOR_SECRET,
    CURSOR_STATE_REL,
    CURSOR_TRACKING_REL,
    CURSOR_TRANSCRIPT_REL,
    bad_line_text,
    cursor_mcp_bubble,
    cursor_review_bubbles,
    cursor_stopped_bubbles,
)

from .base import ParserBase, rel_only


class CursorTests(ParserBase):
    """Test the Cursor IDE conversations.

    The shapes follow the Parser plan of research/cursor.md.
    """

    def test_timestamp_wrapper_takes_ascii_whitespace_only(self):
        wrapper = "<timestamp>t</timestamp>%s<user_query>q</user_query>"
        self.assertTrue(WRAPPER_PATTERN.match(wrapper % "\n"))
        for space in map(chr, (0x00A0, 0x2028, 0x0085)):
            self.assertIsNone(WRAPPER_PATTERN.match(wrapper % space), repr(space))

    def test_tag_takes_intl_spaces_before_the_meridiem(self):
        # The tag is JavaScript `Intl` en-US output, which since ICU 72 puts a
        # NARROW NO-BREAK SPACE before AM or PM; a NO-BREAK SPACE is read too.
        tag = "Thursday, Oct 1, 2026, 6:00%sAM (UTC-4)"
        for space in (" ", chr(0x202F), chr(0x00A0)):
            self.assertTrue(TAG_PATTERN.match(tag % space), repr(space))
            self.assertEqual(tag_time(tag % space), "2026-10-01T10:00:00.000Z", repr(space))

    COMPOSER = "(from state.vscdb composerState, file mtime, written late by the app)"
    REASON = "reason=Not in allowlist: curl, head"
    DELETED = (
        "conversation deleted (header kept; time is the header's lastUpdatedAt)"
        " (isArchived is set by the delete)"
    )
    CLOUD_ORIGIN = "session origin: remote cursor cloud agent"
    CLOUD_SOURCE = "(from state.vscdb cloudAgentRepository.agents, rowUpdatedAtMs)"
    REPOS_SOURCE = "(from state.vscdb cloudEnvironmentTemplate.githubRepos, file mtime)"
    CACHED = "cached conversation text (conversation-search.db, source=cloud-cache): "
    MCP_AUTH = (
        "permission decision: accepted mcp auth for cloud agent %s tool_call=%s"
        " (from state.vscdb cloudAgentRepository.mcpAuthResolution, file mtime)"
        % (CURSOR_CLOUD, CURSOR_MCP_CALL)
    )
    PLUGINS_SOURCE = "(from state.vscdb cursor.plugins.installedIds, file mtime)"
    # The MCP server and plugin rows of state.vscdb (Cursor 3.26.13 on
    # Windows): scopes in name order, the attempt scope skipped; the
    # plugin lists in key order.
    MCP_ROWS: ClassVar[list[tuple[str, str, str, str]]] = [
        (
            "2026-10-01T14:00:00.000Z",
            "",
            "system",
            "mcp server: plugin-acme-tracker-tracker url=https://tracker.example.test/mcp"
            " (from state.vscdb mcpOAuth.global, file mtime)",
        ),
        (
            "2026-10-01T10:08:20.000Z",
            "",
            "system",
            "mcp server: plugin-calendar-calendar"
            " url=https://calendar.mcp.example.test/mcp"
            " (from state.vscdb mcpOAuth.global, mcp_oauth_updated_at_ms)"
            " tokens=stored (ciphertext, not read)",
        ),
        (
            "2026-10-01T14:00:00.000Z",
            "",
            "system",
            "plugin installed: id=12345678 sources=user scope=file:///srv/proj " + PLUGINS_SOURCE,
        ),
        (
            "2026-10-01T14:00:00.000Z",
            "",
            "system",
            "plugin installed: id=12345678 sources=user scope=no-workspace " + PLUGINS_SOURCE,
        ),
    ]
    SYNCED = (
        "session synced: cursor cloud agent %s (vendor copy; from state.vscdb"
        " cloudAgentRepository.agents)"
    )
    # The cloud agent rows of state.vscdb: the agent list under the
    # URL-encoded account id sorts first; the agent with a cached
    # conversation leaves its origin row to conversation-search.db.
    CLOUD_ROWS: ClassVar[list[tuple[str, str, str, str]]] = [
        ("2026-10-01T13:03:20.000Z", CURSOR_CLOUD_LISTED, "system", CLOUD_ORIGIN),
        ("2026-10-01T13:03:20.000Z", CURSOR_CLOUD_LISTED, "system", SYNCED % CURSOR_CLOUD_LISTED),
        (
            "2026-10-01T13:03:20.000Z",
            CURSOR_CLOUD_LISTED,
            "system",
            "model: grok-4.7 requested=default",
        ),
        (
            "2026-10-01T13:03:20.000Z",
            CURSOR_CLOUD_LISTED,
            "system",
            "source host: github.com alice-gh/web (cloud agent)",
        ),
        (
            "2026-10-01T13:05:20.000Z",
            CURSOR_CLOUD_LISTED,
            "system",
            "cloud agent: Summarise the web repository repo=github.com/alice-gh/web status=1"
            " workflowStatus=1 archived=false killed=false vmStarted=false " + CLOUD_SOURCE,
        ),
        ("2026-10-01T12:46:40.000Z", CURSOR_CLOUD, "system", SYNCED % CURSOR_CLOUD),
        ("2026-10-01T12:46:40.000Z", CURSOR_CLOUD, "system", "model: grok-4.7"),
        (
            "2026-10-01T12:46:40.000Z",
            CURSOR_CLOUD,
            "system",
            "source host: github.com alice-gh/demo (cloud agent)",
        ),
        (
            "2026-10-01T12:48:40.000Z",
            CURSOR_CLOUD,
            "system",
            "cloud agent: Describe the demo repository repo=github.com/alice-gh/demo status=2"
            " workflowStatus=2 archived=true killed=false vmStarted=true " + CLOUD_SOURCE,
        ),
        ("2026-10-01T14:00:00.000Z", CURSOR_CLOUD, "system", MCP_AUTH),
        (
            "2026-10-01T14:00:00.000Z",
            "",
            "system",
            "source host repository: https://github.com/alice-gh/demo type=github " + REPOS_SOURCE,
        ),
        (
            "2026-10-01T14:00:00.000Z",
            "",
            "system",
            "source host repository: alice-gh/web type=github " + REPOS_SOURCE,
        ),
    ]
    CALL = "call-t%d-%d\nfc_t%d_%d"
    CURL = '{"command": "curl -s example.test | head"}'
    GITHUB = (
        "source host: github account alice-gh scopes=repo,workflow,user:email,read:user"
        " (from state.vscdb key github-alice-gh)"
    )
    ENROLLED = (
        "remote agent host enrolled: lab-vm"
        " (from state.vscdb remoteAgentHost.enrolled.auth0|user_01, file mtime;"
        " on by default, a paired device leaves no local record)"
    )
    # (timestamp_utc, session_id, turn_type, text) of every state.vscdb row
    # without --include-thinking, in the order the parser yields them.
    EXPECTED: ClassVar[list[tuple[str, str, str, str]]] = [
        ("2026-10-01T09:59:00.000Z", CURSOR_LIVE, "system", "session name: Add subtract"),
        ("2026-10-01T10:00:00.000Z", CURSOR_LIVE, "system", "model: grok-4.6"),
        ("2026-10-01T10:00:00.000Z", CURSOR_LIVE, "user", "Add a subtract function"),
        ("2026-10-01T10:00:02.000Z", CURSOR_LIVE, "tool_use", '{"path": "/srv/proj/calc.py"}'),
        (
            "2026-10-01T10:00:02.500Z",
            CURSOR_LIVE,
            "tool_result",
            '{"contents": "def add(a, b): ..."}',
        ),
        ("2026-10-01T10:00:05.000Z", CURSOR_LIVE, "assistant", "Added subtract()."),
        ("2026-10-01T10:01:00.000Z", CURSOR_LIVE, "system", "model: grok-4.6 -> grok-4.7"),
        ("2026-10-01T10:01:00.000Z", CURSOR_LIVE, "user", "Fetch the page with curl"),
        ("2026-10-01T10:01:01.000Z", CURSOR_LIVE, "tool_use", CURL),
        (
            "2026-10-01T10:01:01.000Z",
            CURSOR_LIVE,
            "system",
            "permission decision: run outcome=success approval=user " + REASON,
        ),
        ("2026-10-01T10:01:01.000Z", CURSOR_LIVE, "tool_result", '{"output": "ok"}'),
        ("2026-10-01T10:01:10.000Z", CURSOR_LIVE, "tool_use", CURL),
        (
            "2026-10-01T10:01:10.000Z",
            CURSOR_LIVE,
            "system",
            "permission decision: skip outcome=rejected approval=user " + REASON,
        ),
        (
            "2026-10-01T10:01:10.000Z",
            CURSOR_LIVE,
            "tool_result",
            '[rejected] {"error": "skipped"}',
        ),
        ("2026-10-01T10:01:20.000Z", CURSOR_LIVE, "tool_use", CURL),
        (
            "2026-10-01T10:01:20.000Z",
            CURSOR_LIVE,
            "system",
            "permission decision: allowlistCommands outcome=success approval=user "
            + REASON
            + " allowlisted=curl,head",
        ),
        ("2026-10-01T10:01:20.000Z", CURSOR_LIVE, "tool_result", '{"output": "ok"}'),
        ("2026-10-01T10:20:00.000Z", CURSOR_ARCHIVED, "system", "session name: Read the README"),
        ("2026-10-01T10:20:00.000Z", CURSOR_ARCHIVED, "system", "model: grok-4.7"),
        ("2026-10-01T10:20:00.000Z", CURSOR_ARCHIVED, "user", "What does the README say?"),
        ("2026-10-01T10:20:02.000Z", CURSOR_ARCHIVED, "assistant", "It describes calc.py."),
        ("2026-10-01T10:21:00.000Z", CURSOR_ARCHIVED, "user", "And the LICENSE?"),
        ("2026-10-01T10:21:02.000Z", CURSOR_ARCHIVED, "assistant", "There is none."),
        ("2026-10-01T12:00:00.000Z", CURSOR_ARCHIVED, "system", "conversation archived"),
        ("2026-10-01T12:30:00.000Z", CURSOR_DRAFT, "system", "unsent draft: draft not sent"),
        ("2026-10-01T13:00:00.000Z", CURSOR_DELETED, "system", DELETED),
        (
            "",
            CURSOR_DELETED,
            "user",
            "<context>conversation %s</context>\n<user_query>c3 deleted question</user_query>"
            % CURSOR_DELETED,
        ),
        (
            "2026-10-01T14:00:00.000Z",
            "",
            "system",
            "permission setting: yoloCommandAllowlist=curl " + COMPOSER,
        ),
        (
            "2026-10-01T14:00:00.000Z",
            "",
            "system",
            "permission setting: yoloCommandAllowlist=head " + COMPOSER,
        ),
        (
            "2026-10-01T14:00:00.000Z",
            "",
            "system",
            "permission setting: run mode agent=Allowlist (with Sandbox) autoRun=true"
            " smartModeAutoRun=false fullAutoRun=false sandbox=true " + COMPOSER,
        ),
        (
            "2026-10-01T14:00:00.000Z",
            "",
            "system",
            "permission setting: run mode plan=Ask Every Time autoRun=false"
            " smartModeAutoRun=false fullAutoRun=false sandbox=true " + COMPOSER,
        ),
        (
            "2026-10-01T14:00:00.000Z",
            "",
            "system",
            "permission setting: yoloOutsideWorkspaceDisabled=true " + COMPOSER,
        ),
        (
            "2026-10-01T10:00:30.000Z",
            "",
            "system",
            "model last used: grok-4.7"
            " (from state.vscdb aiSettings.modelLastUsedAt, written late by the app)",
        ),
        ("2026-10-01T10:06:40.000Z", "", "system", GITHUB),
        ("2026-10-01T14:00:00.000Z", "", "system", ENROLLED),
        *CLOUD_ROWS,
        *MCP_ROWS,
    ]

    def state_rows(self, **options):
        return self.rows_for(CursorParser(), CURSOR_STATE_REL, **options)

    def write_state(self, sql: str, params=()) -> None:
        connection = sqlite3.connect(str(self.home / CURSOR_STATE_REL))
        connection.execute(sql, params)
        connection.commit()
        connection.close()

    def test_rows(self):
        rows = self.state_rows()
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.turn_type, row.text) for row in rows],
            self.EXPECTED,
        )
        for row in rows:
            self.assertEqual((row.host, row.user, row.agent), ("h1", "alice", "cursor"))
            self.assertEqual(row.source_file, "/alice/" + CURSOR_STATE_REL)
            project = "/srv/proj" if row.session_id[:1] == "c" else ""
            if row.text.startswith("plugin installed:") and "file:///srv/proj" in row.text:
                project = "/srv/proj"
            if row.session_id[:3] == "bc-":
                project = (
                    CURSOR_CLOUD_REPO
                    if row.session_id == CURSOR_CLOUD
                    else "github.com/alice-gh/web"
                )
            self.assertEqual(row.project_path, project)
            self.assertEqual(row.git_branch, "")
            self.assertEqual(
                row.recovered, "yes" if row.turn_type == "user" and not row.timestamp_utc else ""
            )

    def test_model_column_and_tool_ids(self):
        rows = [row for row in self.state_rows() if row.session_id == CURSOR_LIVE]
        # The tracked model of the turn's user bubble; the session name
        # comes before the first.
        self.assertEqual([row.model for row in rows], [""] + ["grok-4.6"] * 5 + ["grok-4.7"] * 11)
        tools = [row for row in rows if row.tool_use_id]
        self.assertEqual(
            [(row.turn_type, row.tool_name, row.tool_use_id) for row in tools[:2]],
            [("tool_use", "read_file_v2", "tc1"), ("tool_result", "read_file_v2", "tc1")],
        )
        # Each reviewed command: tool_use, its decision and its result, on
        # the call id; the auto-run read has no decision row.
        for k, (line, i) in enumerate(((2, 0), (3, 1), (4, 2))):
            group = tools[2 + 3 * k : 5 + 3 * k]
            self.assertEqual(
                [(row.turn_type, row.tool_name) for row in group],
                [
                    ("tool_use", "run_terminal_command_v2"),
                    ("system", "run_terminal_command_v2"),
                    ("tool_result", "run_terminal_command_v2"),
                ],
            )
            self.assertEqual({row.tool_use_id for row in group}, {self.CALL % (line, i, line, i)})
        self.assertEqual(len(tools), 11)

    def test_thinking_opt_in(self):
        self.assertNotIn("thinking", {row.turn_type for row in self.state_rows()})
        rows = self.state_rows(include_thinking=True)
        thinking = [row for row in rows if row.turn_type == "thinking"]
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.text, row.model) for row in thinking],
            [("2026-10-01T10:00:01.000Z", CURSOR_LIVE, "Read calc.py first.", "grok-4.6")],
        )
        self.assertEqual(
            [row for row in rows if row.turn_type != "thinking"][3].turn_type, "tool_use"
        )
        self.assertEqual(len(rows), len(self.EXPECTED) + 1)

    def test_drafts_and_deleted(self):
        rows = self.state_rows()
        by_session = {}
        for row in rows:
            by_session.setdefault(row.session_id, []).append(row.text)
        # A draft yields only its unsent text; a header-only draft (no
        # location history, not archived) is not a deleted conversation.
        self.assertEqual(by_session[CURSOR_DRAFT], ["unsent draft: draft not sent"])
        self.assertNotIn(CURSOR_EMPTY_HEAD, by_session)
        # The deleted conversation: its header and the JSON blob that names
        # it; the protobuf blob and the live conversation's blob give nothing.
        self.assertEqual(len(by_session[CURSOR_DELETED]), 2)
        self.assertEqual(sum("live" in text for text in by_session[CURSOR_LIVE]), 0)
        self.assertFalse(any("You are an agent" in row.text for row in rows))

    def test_transcript(self):
        parser = CursorParser()
        art = self.artifact(CURSOR_TRANSCRIPT_REL)
        self.assertTrue(parser.secondary(art))
        self.assertEqual(parser.secondary_tolerance(art), 60.0)
        rows = list(parser.parse(art, Options(include_thinking=True)))
        self.assertEqual(
            [
                (row.timestamp_utc, row.turn_type, row.tool_name, row.text, row.source_line)
                for row in rows
            ],
            [
                ("2026-10-01T10:00:00.000Z", "user", "", "Add a subtract function", 1),
                ("", "assistant", "", "Added subtract().", 2),
                ("2026-10-01T10:01:00.000Z", "user", "", "Fetch the page with curl", 4),
                (
                    "",
                    "tool_use",
                    "Shell",
                    '{"command":"curl -s example.test | head","description":"Fetch the page",'
                    '"required_permissions":["full_network"]}',
                    5,
                ),
                ("", "system", "", bad_line_text(), 7),
            ],
        )
        for row in rows:
            self.assertEqual((row.session_id, row.project_path), (CURSOR_LIVE, "/srv/proj"))
            self.assertEqual((row.model, row.tool_use_id), ("", ""))

    def test_truncated_line(self):
        rel = CURSOR_TRANSCRIPT_REL
        after = {"role": "user", "message": {"content": [{"type": "text", "text": "after"}]}}
        with open(self.home / rel, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(after) + "\n")
        rows = self.rows_for(CursorParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows][-3:],
            [
                ("tool_use", rows[3].text),
                ("user", "after"),
                ("system", bad_line_text()),
            ],
        )
        self.assertEqual(rows[0].text, "Add a subtract function")
        bad = rows[-1]
        self.assertEqual(
            (bad.session_id, bad.project_path, bad.timestamp_utc), (CURSOR_LIVE, "/srv/proj", "")
        )
        # A wrapper that does not parse keeps the whole text, with no time.
        self.assertEqual(rows[-2].timestamp_utc, "")

    def test_tag_time(self):
        self.assertEqual(
            tag_time("Thursday, Oct 1, 2026, 6:00 AM (UTC-4)"), "2026-10-01T10:00:00.000Z"
        )
        self.assertEqual(
            tag_time("Thursday, Oct 1, 2026, 12:05 AM (UTC)"), "2026-10-01T00:05:00.000Z"
        )
        self.assertEqual(
            tag_time("Thursday, Oct 1, 2026, 12:05 PM (UTC+5:30)"), "2026-10-01T06:35:00.000Z"
        )
        self.assertEqual(
            tag_time("Wednesday, Sep 30, 2026, 9:30 PM (UTC-4)"), "2026-10-01T01:30:00.000Z"
        )
        self.assertEqual(tag_time("yesterday"), "")
        self.assertEqual(tag_time("Thursday, Foo 1, 2026, 6:00 AM (UTC-4)"), "")
        self.assertEqual(
            unwrap_query(
                "<timestamp>Thursday, Oct 1, 2026, 6:00 AM (UTC-4)</timestamp>\n<user_query>a\nb</user_query>"
            ),
            ("a\nb", "2026-10-01T10:00:00.000Z"),
        )
        wrapped = "<timestamp>soon</timestamp>\n<user_query>q</user_query>"
        self.assertEqual(unwrap_query(wrapped), (wrapped, ""))

    def test_run_mode(self):
        auto_run = {"autoRun": True}
        self.assertEqual(run_mode({"autoRun": False}, True), "Ask Every Time")
        self.assertEqual(run_mode(auto_run, False), "Allowlist")
        self.assertEqual(run_mode(auto_run, True), "Allowlist (with Sandbox)")
        self.assertEqual(run_mode({**auto_run, "smartModeAutoRun": True}, False), "Auto-review")
        self.assertEqual(
            run_mode({**auto_run, "smartModeAutoRun": True}, True), "Auto-review (with Sandbox)"
        )
        self.assertEqual(run_mode({"autoRun": False, "fullAutoRun": True}, True), "Run Everything")

    def test_github_repository(self):
        """Read the repository from a parsed host, not from a string prefix.

        Only a URL whose host is github.com names one.
        """
        self.assertEqual(github_repository("github.com/owner/demo"), "owner/demo")
        self.assertEqual(github_repository("https://github.com/owner/demo"), "owner/demo")
        self.assertEqual(github_repository("https://github.com/owner/demo/"), "owner/demo")
        self.assertEqual(github_repository("github.com"), "")
        self.assertEqual(github_repository("origin.cursor.com/git/ns/tmp-0123456789abcdef"), "")
        self.assertEqual(github_repository("github.com.example.net/owner/demo"), "")
        self.assertEqual(github_repository("example.net/github.com/owner/demo"), "")
        self.assertEqual(github_repository("https://example.net/?u=github.com/owner/demo"), "")
        self.assertEqual(github_repository(""), "")
        cases = {
            "github.com/o/r": "o/r",
            "https://github.com/o/r.git": "o/r",
            "https://user:pw@github.com/o/r": "o/r",
            "https://github.com:443/o/r": "o/r",
            "github.com.evil.example/o/r": "",
            "notgithub.com/o/r": "",
            "https://github.com@evil.example/o/r": "",
        }
        for url, expected in cases.items():
            self.assertEqual(github_repository(url), expected, url)

    def test_deduplication_and_recovery(self):
        rows, _, problems, dropped = cli.collect_rows(self.col, Options(), ["cursor"])
        self.assertEqual(problems, [])
        # The transcript's two prompts and its reply repeat the bubbles.
        self.assertEqual(dropped["cursor"], 3)
        transcript = [row for row in rows if row.source_file.endswith(CURSOR_TRANSCRIPT_REL)]
        self.assertEqual(
            [(row.turn_type, row.tool_name, row.recovered) for row in transcript],
            [("tool_use", "Shell", "yes"), ("system", "", "")],
        )
        gone = [row for row in rows if row.session_id == CURSOR_DELETED]
        self.assertEqual(
            [(row.timestamp_utc, row.turn_type, row.text[:22], row.recovered) for row in gone],
            [
                ("2026-10-01T01:30:00.000Z", "user", "c3 deleted question", "yes"),
                (
                    "2026-10-01T01:31:00.000Z",
                    "system",
                    "ai code: /srv/proj/cal",
                    "",
                ),
                ("2026-10-01T13:00:00.000Z", "system", self.DELETED[:22], ""),
                ("", "assistant", "Answer kept only here.", "yes"),
                ("", "user", "<context>conversation ", "yes"),
            ],
        )
        self.assertEqual({row.project_path for row in gone}, {"/srv/proj"})
        self.assertEqual(
            {row.source_file.rsplit("/", 1)[-1] for row in gone},
            {
                "state.vscdb",
                "ai-code-tracking.db",
                CURSOR_DELETED_TRANSCRIPT_REL.rsplit("/", 1)[-1],
            },
        )

    def test_sessions(self):
        rows, _, _, _ = cli.collect_rows(self.col, Options(), ["cursor"])
        sessions = summarise(
            rows, cli.SYNC_AGENTS, cli.RELAY_AGENTS, cli.ORIGIN_AGENTS, cli.SECONDARY_AGENTS
        )
        got = {
            session.session_id: (
                session.origin,
                session.recovered,
                session.synced,
                session.user_turns,
                session.tool_calls,
            )
            for session in sessions
            if session.agent == "cursor"
        }
        self.assertEqual(got[CURSOR_LIVE], ("local", "partial", "no", 2, 5))
        self.assertEqual(got[CURSOR_ARCHIVED], ("local", "no", "no", 2, 0))
        # Every event row of the deleted conversation is recovered; its
        # system rows (the header, the ai-tracking row) are not counted.
        self.assertEqual(got[CURSOR_DELETED][:2], ("local", "yes"))
        self.assertEqual(got[CURSOR_DELETED][3], 2)

    def test_side_databases(self):
        rows = self.rows_for(CursorParser(), CURSOR_TRACKING_REL)
        self.assertEqual(
            [
                (row.timestamp_utc, row.session_id, row.model, row.git_branch, row.text)
                for row in rows
            ],
            [
                (
                    "2026-10-01T10:00:03.000Z",
                    CURSOR_LIVE,
                    "grok-4.6",
                    "",
                    "ai code: /srv/proj/calc.py source=composer model=grok-4.6 request=r1",
                ),
                (
                    "2026-10-01T01:31:00.000Z",
                    CURSOR_DELETED,
                    "grok-4.6",
                    "",
                    "ai code: /srv/proj/calc.py source=composer model=grok-4.6 request=r9",
                ),
                (
                    "2026-10-01T10:00:06.000Z",
                    CURSOR_LIVE,
                    "grok-4.6",
                    "",
                    "conversation summary: Add subtract: Added subtract().",
                ),
                (
                    "2026-10-01T10:01:30.000Z",
                    CURSOR_LIVE,
                    "grok-4.7",
                    "",
                    "file deleted by agent: old.py model=grok-4.7",
                ),
                (
                    "2026-10-01T10:06:40.000Z",
                    "",
                    "",
                    "main",
                    "scored commit: abc1234 branch=main +4 -0",
                ),
            ],
        )
        self.assertEqual([row.project_path for row in rows], ["/srv/proj"] * 4 + [""])
        rows = self.rows_for(CursorParser(), CURSOR_SEARCH_REL)
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.project_path, row.text) for row in rows],
            [
                (
                    "2026-10-01T10:05:00.000Z",
                    CURSOR_LIVE,
                    "/srv/proj",
                    "conversation index: Add subtract source=local archived=0",
                ),
                (
                    "2026-10-01T10:05:00.000Z",
                    CURSOR_LIVE,
                    "/srv/proj",
                    "cached conversation text (conversation-search.db, source=local):"
                    " Add a subtract function",
                ),
                (
                    "2026-10-01T12:00:00.000Z",
                    CURSOR_ARCHIVED,
                    "/srv/proj",
                    "conversation index: Read the README source=local archived=1",
                ),
                ("2026-10-01T12:47:40.000Z", CURSOR_CLOUD, CURSOR_CLOUD_REPO, self.CLOUD_ORIGIN),
                (
                    "2026-10-01T12:47:40.000Z",
                    CURSOR_CLOUD,
                    CURSOR_CLOUD_REPO,
                    "conversation index: Describe the demo repository source=cloud-cache"
                    " archived=1",
                ),
                (
                    "2026-10-01T12:47:40.000Z",
                    CURSOR_CLOUD,
                    CURSOR_CLOUD_REPO,
                    self.CACHED + CURSOR_CLOUD_BODY,
                ),
            ],
        )
        self.assertEqual([row.model for row in rows], ["", "", ""] + ["grok-4.7"] * 3)
        self.assertFalse(CursorParser().secondary(self.artifact(CURSOR_SEARCH_REL)))

    def test_settings_files(self):
        texts = [row.text for row in self.rows_for(CursorParser(), ".cursor/cli-config.json")]
        source = "(from .cursor/cli-config.json, file mtime)"
        self.assertEqual(
            texts,
            [
                "permission setting: approvalMode=allowlist " + source,
                "permission setting: permissions.allow=Shell(ls) " + source,
                "permission setting: sandbox.mode=disabled " + source,
            ],
        )
        rows = self.rows_for(CursorParser(), ".cursor/permissions.json")
        source = "(from .cursor/permissions.json, file mtime)"
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: allowlists.mcp=github:list_issues " + source,
                "permission setting: allowlists.terminal=git status " + source,
                "permission setting: approvalMode=allowlist " + source,
                "permission setting: allowInstructions=read-only git commands " + source,
                "permission setting: blockInstructions=anything that pushes " + source,
            ],
        )
        self.assertEqual({row.timestamp_utc for row in rows}, {"2026-10-01T14:00:00.000Z"})
        (self.home / ".cursor/cli-config.json").write_text("{not json", encoding="utf-8")
        rows = self.rows_for(CursorParser(), ".cursor/cli-config.json")
        self.assertEqual(len(rows), 1)
        self.assertTrue(
            rows[0].text.startswith("parser: unreadable .cursor/cli-config.json: "), rows[0].text
        )
        (self.home / ".cursor/permissions.json").write_text("[]", encoding="utf-8")
        rows = self.rows_for(CursorParser(), ".cursor/permissions.json")
        self.assertEqual(
            [row.text for row in rows], ["parser: .cursor/permissions.json is not a JSON object"]
        )

    def test_not_wanted(self):
        parser = CursorParser()
        for rel in (
            ".cursor/statsig-cache.json",
            ".cursor/mcp.json",
            ".cursor/projects/srv-proj/mcps/cursor-ide-browser/SERVER_METADATA.json",
        ):
            self.assertFalse(parser.wants(self.artifact(rel)), rel)
            self.assertEqual(self.artifact(rel).agent, "cursor")
        self.assertTrue(parser.wants(self.artifact(CURSOR_ATTEMPT_REL)))
        self.assertEqual(self.artifact(CURSOR_ATTEMPT_REL).agent, "cursor")
        for rel in (
            CURSOR_STATE_REL + "-wal",
            ".config/Cursor/User/workspaceStorage/0123/state.vscdb",
            ".config/cursor/User/globalStorage/state.vscdb",
            ".config/Code/User/globalStorage/state.vscdb",
            ".cursor/chats/ab/s1/store.db",
            ".cursor/projects/p/agent-transcripts/c.jsonl",
            ".cursor/projects/empty-window/agent-tools/a0000001.txt",
            ".cursor/plugins/cache/cursor-public/google-calendar/0123abcd/mcp.json",
            "AppData/Roaming/Cursor/User/globalStorage/mcp-oauth-attempts/x/a.json",
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)
        for rel in (
            ".config/Cursor/User/globalStorage/state.vscdb",
            "AppData/Roaming/Cursor/User/globalStorage/conversation-search.db",
            ".cursor/projects/p/agent-transcripts/c/c.jsonl",
            "AppData/Roaming/Cursor/User/globalStorage/mcp-oauth-attempts/a.json",
        ):
            self.assertTrue(parser.wants(rel_only(rel)), rel)

    def test_pending_prompt_and_editor_allowlist(self):
        """Tell a prompt still open from the editor window's allow-list add.

        A prompt still open when the store was copied is not a decision; the allow-list
        add (`selectedOption` `run`) is an allow-always decision by its
        `appliedAllowlistOptionIds`.
        """
        connection = sqlite3.connect(str(self.home / CURSOR_STATE_REL))
        key = "composerData:" + CURSOR_LIVE
        data = json.loads(
            connection.execute("SELECT value FROM cursorDiskKV WHERE key = ?", (key,)).fetchone()[0]
        )
        for head, bubble in cursor_review_bubbles():
            data["fullConversationHeadersOnly"].append(head)
            connection.execute(
                "INSERT INTO cursorDiskKV VALUES (?, ?)",
                ("bubbleId:%s:%s" % (CURSOR_LIVE, head["bubbleId"]), json.dumps(bubble)),
            )
        connection.execute(
            "UPDATE cursorDiskKV SET value = ? WHERE key = ?", (json.dumps(data), key)
        )
        connection.commit()
        connection.close()
        rows = [row for row in self.state_rows() if row.session_id == CURSOR_LIVE]
        pending = [row for row in rows if row.tool_use_id == self.CALL % (5, 3, 5, 3)]
        self.assertEqual(
            [(row.turn_type, row.text) for row in pending],
            [
                ("tool_use", '{"command": "python3 -c \'print(1 + 1)\'"}'),
                (
                    "system",
                    "permission prompt pending: run_terminal_command_v2"
                    " reason=Not in allowlist: python3 options=python3",
                ),
            ],
        )
        self.assertEqual(pending[1].timestamp_utc, "2026-10-01T10:01:30.000Z")
        self.assertFalse(
            [row for row in pending if row.text.startswith("permission decision:")],
            "a pending prompt's preset selectedOption is not a decision",
        )
        added = [row for row in rows if row.tool_use_id == self.CALL % (6, 4, 6, 4)]
        self.assertEqual(
            [(row.turn_type, row.text) for row in added[1:]],
            [
                (
                    "system",
                    "permission decision: run outcome=success approval=-"
                    " reason=Not in allowlist: python3 allowlisted=python3",
                ),
                ("tool_result", '{"output": "2"}'),
            ],
        )

    def test_source_host_and_remote_agent_host(self):
        """Name the GitHub account from its key and the remote agent host at file time.

        The sign-in's time is the latest usage; no `secret://` value reaches any row.
        """
        rows = self.state_rows()
        hosts = [
            row
            for row in rows
            if row.text.startswith(("source host:", "remote agent host")) and not row.session_id
        ]
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.project_path, row.text) for row in hosts],
            [
                ("2026-10-01T10:06:40.000Z", "", "", self.GITHUB),
                ("2026-10-01T14:00:00.000Z", "", "", self.ENROLLED),
            ],
        )
        for row in self.state_rows(include_thinking=True):
            self.assertNotIn(CURSOR_SECRET, row.text)
            self.assertNotIn("computerKey", row.text)
            self.assertNotIn("github.auth", row.text)
        # No usages: no scopes and the file time; not enrolled: no row; a
        # `github-` key that is not an account list is skipped.
        self.write_state("DELETE FROM ItemTable WHERE key = ?", ("github-alice-gh-usages",))
        self.write_state(
            "UPDATE ItemTable SET value = ? WHERE key = ?",
            (b"false", "remoteAgentHost.enrolled.auth0|user_01"),
        )
        self.write_state("INSERT INTO ItemTable VALUES (?, ?)", ("github-other.setting", b"1"))
        state = self.home / CURSOR_STATE_REL
        os.utime(state, (CURSOR_MTIME, CURSOR_MTIME))
        hosts = [
            (row.timestamp_utc, row.text)
            for row in self.state_rows()
            if row.text.startswith(("source host:", "remote agent host", "parser:"))
            and not row.session_id
        ]
        self.assertEqual(
            hosts,
            [
                (
                    "2026-10-01T14:00:00.000Z",
                    "source host: github account alice-gh scopes=-"
                    " (from state.vscdb key github-alice-gh)",
                )
            ],
        )

    def test_missing_bubble_and_bad_values(self):
        self.write_state(
            "DELETE FROM cursorDiskKV WHERE key = ?", ("bubbleId:%s:u4" % CURSOR_ARCHIVED,)
        )
        self.write_state(
            "UPDATE cursorDiskKV SET value = ? WHERE key = ?",
            ("{cut", "bubbleId:%s:a3/s:0" % CURSOR_ARCHIVED),
        )
        rows = [row for row in self.state_rows() if row.session_id == CURSOR_ARCHIVED]
        texts = [row.text for row in rows]
        self.assertIn("parser: missing bubble u4", texts)
        self.assertNotIn("It describes calc.py.", texts)
        self.assertIn("There is none.", texts)
        bad = rows[-1]
        self.assertTrue(
            bad.text.startswith(
                "parser: unparseable record 14 (bubbleId:%s:a3/s:0: " % CURSOR_ARCHIVED
            ),
            bad.text,
        )
        self.assertTrue(bad.text.endswith("): {cut"), bad.text)
        self.assertEqual((bad.source_line, bad.project_path), (14, "/srv/proj"))

    def test_remote_origin(self):
        connection = sqlite3.connect(str(self.home / CURSOR_STATE_REL))
        (value,) = connection.execute(
            "SELECT value FROM composerHeaders WHERE composerId = ?", (CURSOR_ARCHIVED,)
        ).fetchone()
        head = json.loads(value)
        head["agentLocation"]["type"] = "cloud"
        connection.execute(
            "UPDATE composerHeaders SET value = ? WHERE composerId = ?",
            (json.dumps(head), CURSOR_ARCHIVED),
        )
        connection.commit()
        connection.close()
        rows = [row for row in self.state_rows() if row.session_id == CURSOR_ARCHIVED]
        self.assertEqual(
            (rows[1].timestamp_utc, rows[1].text),
            ("2026-10-01T10:20:00.000Z", "session origin: remote cloud"),
        )

    def test_cloud_agents(self):
        """Read a cloud agent as a remote-origin session.

        Its list entry and its cached conversation give the session; the cached body is
        one system row in full, never a user or assistant row, as Cursor 3.26.13 on
        Windows writes it.
        """
        rows, _, problems, _ = cli.collect_rows(self.col, Options(), ["cursor"])
        self.assertEqual(problems, [])
        cloud = [row for row in rows if row.session_id == CURSOR_CLOUD]
        self.assertEqual({row.turn_type for row in cloud}, {"system"})
        self.assertEqual([row.text for row in cloud].count(self.CLOUD_ORIGIN), 1)
        self.assertIn(self.CACHED + CURSOR_CLOUD_BODY, [row.text for row in cloud])
        self.assertEqual(
            {(row.project_path, row.model) for row in cloud}, {(CURSOR_CLOUD_REPO, "grok-4.7")}
        )
        listed = [row.text for row in rows if row.session_id == CURSOR_CLOUD_LISTED]
        self.assertEqual(listed.count(self.CLOUD_ORIGIN), 1)
        sessions = summarise(
            rows, cli.SYNC_AGENTS, cli.RELAY_AGENTS, cli.ORIGIN_AGENTS, cli.SECONDARY_AGENTS
        )
        got = {
            session.session_id: (
                session.origin,
                session.synced,
                session.user_turns,
                session.project_path,
            )
            for session in sessions
            if session.agent == "cursor"
        }
        self.assertEqual(got[CURSOR_CLOUD], ("remote", "yes", 0, CURSOR_CLOUD_REPO))
        self.assertEqual(got[CURSOR_CLOUD_LISTED], ("remote", "yes", 0, "github.com/alice-gh/web"))
        self.assertEqual(got[CURSOR_LIVE][:2], ("local", "no"))
        self.assertEqual([row.text for row in cloud].count(self.SYNCED % CURSOR_CLOUD), 1)
        for row in self.state_rows():
            self.assertNotIn(CURSOR_SECRET, row.text)
            self.assertNotIn("connectionToken", row.text)
            self.assertNotIn("bcCachedDetails", row.text)
            self.assertNotIn("serverPinnedBcIds", row.text)

    def test_cloud_agents_without_search_or_list(self):
        """Read the origin rows from the state database alone.

        Without conversation-search.db every listed agent gets its origin row; a list
        value that is not a JSON list is skipped, a cut one is a bad record; no fts
        content, no cached text row.
        """
        (self.home / CURSOR_SEARCH_REL).unlink()
        rows = [row for row in self.state_rows() if row.session_id == CURSOR_CLOUD]
        self.assertEqual(rows[0].text, self.CLOUD_ORIGIN)
        self.assertEqual(rows[0].timestamp_utc, "2026-10-01T12:46:40.000Z")
        key = "cloudAgentRepository.agents.auth0%7Cuser_02"
        self.write_state("UPDATE ItemTable SET value = ? WHERE key = ?", (b'{"bcId": "x"}', key))
        self.write_state(
            "UPDATE ItemTable SET value = ? WHERE key = ?",
            (b'[{"bcId": "bc-cut"', "cloudAgentRepository.agents.auth0|user_01"),
        )
        rows = self.state_rows()
        # The MCP auth resolution keeps its session; its project and model
        # came from the list.
        cloud = [row for row in rows if row.session_id.startswith("bc-")]
        self.assertEqual(
            [(row.text, row.project_path, row.model) for row in cloud], [(self.MCP_AUTH, "", "")]
        )
        bad = [row for row in rows if row.text.startswith("parser:")]
        self.assertEqual(len(bad), 1)
        self.assertTrue(
            bad[0].text.startswith(
                "parser: unparseable record %d (cloudAgentRepository.agents.auth0|user_01: "
                % bad[0].source_line
            ),
            bad[0].text,
        )
        self.assertTrue(bad[0].text.endswith('): [{"bcId": "bc-cut"'), bad[0].text)
        connection = sqlite3.connect(str(self.home / CURSOR_STATE_REL))
        connection.execute("DROP TABLE ItemTable")
        connection.commit()
        connection.close()
        self.assertFalse([row for row in self.state_rows() if "cloud" in row.text])

    def test_mcp_servers_and_plugins(self):
        """Read MCP OAuth scopes from key names and one row per plugin list entry.

        A secret key's value is never read; a folder scope is the project path. The
        shapes are those Cursor 3.26.13 on Windows writes.
        """
        rows = self.state_rows()
        got = [
            (row.timestamp_utc, row.session_id, row.turn_type, row.text)
            for row in rows
            if row.text.startswith(("mcp server:", "plugin installed:"))
        ]
        self.assertEqual(got, self.MCP_ROWS)
        self.assertEqual(
            [row.project_path for row in rows if row.text.startswith("plugin installed:")],
            ["/srv/proj", ""],
        )
        for row in self.state_rows(include_thinking=True):
            self.assertNotIn(CURSOR_SECRET, row.text)
            self.assertNotIn("Buffer", row.text)
            self.assertNotIn("oauth-attempt", row.text)
            self.assertNotIn("mcp_code_verifier", row.text)
            self.assertNotIn("extensionMigrationComplete", row.text)
        self.assertEqual(
            uri_path("file:///c%3A/Users/alice/proj/demo"), "c:\\Users\\alice\\proj\\demo"
        )
        self.assertEqual(uri_path("file:///home/alice/my%20proj"), "/home/alice/my proj")
        # A plugin list that is not a list is skipped, a cut one is a bad
        # record; an empty one yields nothing.
        self.write_state(
            "UPDATE ItemTable SET value = ? WHERE key = ?",
            (b"{}", "cursor.plugins.installedIds.no-team|no-workspace"),
        )
        self.write_state(
            "UPDATE ItemTable SET value = ? WHERE key = ?",
            (b'[{"id": "1"', "cursor.plugins.installedIds.no-team|file:///srv/proj"),
        )
        self.write_state(
            "INSERT INTO ItemTable VALUES (?, ?)", ("cursor.plugins.installedIds.t|x", b"[]")
        )
        rows = self.state_rows()
        self.assertFalse([row for row in rows if row.text.startswith("plugin installed:")])
        bad = [row.text for row in rows if row.text.startswith("parser:")]
        self.assertEqual(len(bad), 1)
        self.assertTrue(bad[0].endswith('): [{"id": "1"'), bad[0])

    def test_mcp_oauth_attempt(self):
        """Give the attempt file one row at its `createdAtMs`.

        Without it the row takes the file time and says so; a file that is not JSON is
        one `parser:` row.
        """
        parser = CursorParser()
        self.assertTrue(parser.wants(self.artifact(CURSOR_ATTEMPT_REL)))
        self.assertFalse(parser.secondary(self.artifact(CURSOR_ATTEMPT_REL)))
        name = CURSOR_ATTEMPT_REL.rsplit("/", 1)[1]
        rows = self.rows_for(parser, CURSOR_ATTEMPT_REL)
        self.assertEqual(
            [
                (row.timestamp_utc, row.session_id, row.project_path, row.turn_type, row.text)
                for row in rows
            ],
            [
                (
                    "2026-10-01T10:08:00.000Z",
                    "",
                    "",
                    "system",
                    "mcp oauth attempt: server=https://calendar.mcp.example.test/mcp"
                    " client_id=client-0123456789abcdef workspace=empty-window"
                    " (from mcp-oauth-attempts/%s)" % name,
                )
            ],
        )
        path = self.home / CURSOR_ATTEMPT_REL
        path.write_text(json.dumps({"serverUrl": "https://x.test/mcp"}), encoding="utf-8")
        os.utime(path, (CURSOR_MTIME, CURSOR_MTIME))
        rows = self.rows_for(parser, CURSOR_ATTEMPT_REL)
        self.assertEqual(
            [(row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "2026-10-01T14:00:00.000Z",
                    "mcp oauth attempt: server=https://x.test/mcp client_id=- workspace=-"
                    " (from mcp-oauth-attempts/%s, file mtime)" % name,
                )
            ],
        )
        path.write_text('{"serverUrl": "https://x', encoding="utf-8")
        rows = self.rows_for(parser, CURSOR_ATTEMPT_REL)
        self.assertEqual(len(rows), 1)
        self.assertTrue(
            rows[0].text.startswith("parser: unreadable %s: " % CURSOR_ATTEMPT_REL), rows[0].text
        )
        path.write_text("[]", encoding="utf-8")
        self.assertEqual(
            [row.text for row in self.rows_for(parser, CURSOR_ATTEMPT_REL)],
            ["parser: %s is not a JSON object" % CURSOR_ATTEMPT_REL],
        )

    def test_mcp_auth_resolution(self):
        """Join the cloud agent's MCP auth answer to its tool call as a decision."""
        rows = [
            row for row in self.state_rows() if row.text.startswith("permission decision: accepted")
        ]
        self.assertEqual(
            [
                (row.session_id, row.tool_use_id, row.project_path, row.model, row.text)
                for row in rows
            ],
            [(CURSOR_CLOUD, CURSOR_MCP_CALL, CURSOR_CLOUD_REPO, "grok-4.7", self.MCP_AUTH)],
        )

    def test_stopped_turn(self):
        """Count a call in flight when the user stopped the turn as a decision.

        It is not a pending prompt; a queued, cancelled call has no permission row
        (Cursor 3.26.13 on Windows).
        """
        connection = sqlite3.connect(str(self.home / CURSOR_STATE_REL))
        key = "composerData:" + CURSOR_LIVE
        data = json.loads(
            connection.execute("SELECT value FROM cursorDiskKV WHERE key = ?", (key,)).fetchone()[0]
        )
        for head, bubble in cursor_stopped_bubbles():
            data["fullConversationHeadersOnly"].append(head)
            connection.execute(
                "INSERT INTO cursorDiskKV VALUES (?, ?)",
                ("bubbleId:%s:%s" % (CURSOR_LIVE, head["bubbleId"]), json.dumps(bubble)),
            )
        connection.execute(
            "UPDATE cursorDiskKV SET value = ? WHERE key = ?", (json.dumps(data), key)
        )
        connection.commit()
        connection.close()
        rows = [row for row in self.state_rows() if row.tool_use_id in ("tc9", "tc10")]
        self.assertEqual(
            [(row.timestamp_utc, row.tool_use_id, row.turn_type, row.text) for row in rows],
            [
                ("2026-10-01T10:02:00.000Z", "tc9", "tool_use", '{"command": "dir"}'),
                (
                    "2026-10-01T10:02:00.000Z",
                    "tc9",
                    "system",
                    "permission decision: stopped by the user (reviewData.status None,"
                    " additionalData.status rejected)",
                ),
                ("2026-10-01T10:02:00.000Z", "tc9", "tool_result", "[rejected]"),
                ("2026-10-01T10:02:01.000Z", "tc10", "tool_use", '{"command": "type README.md"}'),
            ],
        )
        self.assertFalse([row for row in rows if "pending" in row.text])

    def test_mcp_tool_bubble(self):
        """Give an MCP call approved under Allowlist no permission row.

        It is a tool_use and a tool_result, since the bubble records no decision (Cursor
        3.26.13 on Windows); `toolCallBinary` is not read.
        """
        connection = sqlite3.connect(str(self.home / CURSOR_STATE_REL))
        key = "composerData:" + CURSOR_LIVE
        data = json.loads(
            connection.execute("SELECT value FROM cursorDiskKV WHERE key = ?", (key,)).fetchone()[0]
        )
        head, bubble = cursor_mcp_bubble()
        data["fullConversationHeadersOnly"].append(head)
        connection.execute(
            "INSERT INTO cursorDiskKV VALUES (?, ?)",
            ("bubbleId:%s:%s" % (CURSOR_LIVE, head["bubbleId"]), json.dumps(bubble)),
        )
        connection.execute(
            "UPDATE cursorDiskKV SET value = ? WHERE key = ?", (json.dumps(data), key)
        )
        connection.commit()
        connection.close()
        call = bubble["toolFormerData"]["toolCallId"]
        rows = [row for row in self.state_rows() if row.tool_use_id == call]
        self.assertEqual(
            [
                (row.timestamp_utc, row.turn_type, row.tool_name, row.model, row.text)
                for row in rows
            ],
            [
                (
                    "2026-10-01T10:01:50.000Z",
                    "tool_use",
                    "mcp-google-calendar-list_events",
                    "grok-4.7",
                    '{"calendarId": "primary"}',
                ),
                (
                    "2026-10-01T10:01:51.000Z",
                    "tool_result",
                    "mcp-google-calendar-list_events",
                    "grok-4.7",
                    '{"events": [{"summary": "Standup"}]}',
                ),
            ],
        )
        self.assertFalse([row for row in self.state_rows() if "Q0FMTEJJTkFSWQ" in row.text])

    def test_search_without_fts_content(self):
        connection = sqlite3.connect(str(self.home / CURSOR_SEARCH_REL))
        connection.execute("DROP TABLE conversation_fts_content")
        connection.commit()
        connection.close()
        texts = [row.text for row in self.rows_for(CursorParser(), CURSOR_SEARCH_REL)]
        self.assertEqual(len(texts), 4)
        self.assertNotIn("source=local)", " ".join(texts))
        self.assertFalse([text for text in texts if text.startswith("cached conversation text")])
        self.assertIn(self.CLOUD_ORIGIN, texts)

    def test_deleted_header_not_archived(self):
        """Give a deleted header that is not archived the row without the isArchived clause.

        The header is kept by its location history.
        """
        self.write_state(
            "UPDATE composerHeaders SET isArchived = 0, value = json_set(value, '$.isArchived',"
            " json('false')) WHERE composerId = ?",
            (CURSOR_DELETED,),
        )
        texts = [row.text for row in self.state_rows() if row.session_id == CURSOR_DELETED]
        self.assertIn(self.DELETED.split(" (isArchived")[0], texts)
        self.assertFalse(
            [text for text in texts if "isArchived" in text or text == "conversation archived"]
        )

    def test_unreadable_database(self):
        database = self.home / CURSOR_STATE_REL
        database.write_text("not sqlite", encoding="utf-8")
        rows = self.state_rows()
        self.assertEqual(
            [(row.turn_type, row.text, row.session_id) for row in rows],
            [("system", "parser: not a SQLite database", "")],
        )
        database.write_bytes(b"SQLite format 3\x00" + b"\xff" * 200)
        rows = self.state_rows()
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("parser: SQLite error: "), rows[0].text)
        # Without the state database the transcript still parses, with no
        # project path.
        database.unlink()
        rows = self.rows_for(CursorParser(), CURSOR_DELETED_TRANSCRIPT_REL)
        self.assertEqual(
            [(row.turn_type, row.project_path) for row in rows], [("user", ""), ("assistant", "")]
        )
