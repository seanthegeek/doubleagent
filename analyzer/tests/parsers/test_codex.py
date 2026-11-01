"""Tests of the Codex parser."""

import io
import json
import sqlite3
import sys
import unittest
from typing import ClassVar, cast
from unittest import mock

import zstandard
from test_errors import CAN_DENY, DENY_REASON

from doubleagent import cli
from doubleagent.inputs import open_input
from doubleagent.model import summarise
from doubleagent.parsers import Options
from doubleagent.parsers.codex import CodexParser, flat_toml, iter_zst_jsonl
from doubleagent.parsers.codex_state import LOG_FIELD_PATTERN
from doubleagent.parsers.open_interpreter import OpenInterpreterParser
from doubleagent.timeutil import epoch_ms, to_utc
from fixtures import (
    BAD_LINE,
    CODEX_ACCOUNT,
    CODEX_AUTOMATION_MTIME,
    CODEX_AUTOMATION_REL,
    CODEX_AUTOMATION_TOML,
    CODEX_BACKUP_STATE_REL,
    CODEX_DECLINED_ITEM,
    CODEX_DELETED,
    CODEX_DESKTOP,
    CODEX_DESKTOP_ROLLOUT,
    CODEX_FEEDBACK_TIMESTAMP,
    CODEX_FORK,
    CODEX_GLOBAL_STATE_MTIME,
    CODEX_GLOBAL_STATE_REL,
    CODEX_GOAL_EVENT,
    CODEX_GUARDIAN,
    CODEX_LEGACY,
    CODEX_MEMORY_SUMMARY_REL,
    CODEX_ORPHAN,
    CODEX_PATCH_ITEM,
    CODEX_PATCH_LOG_ROW,
    CODEX_REVERT_ROLLOUT,
    CODEX_REVIEWED_ITEM,
    CODEX_SESSION,
    CODEX_SUBAGENT,
    CODEX_USER,
    OI_ROLLOUT,
    PROJECT_REMOTE,
    bad_line_text,
    build_codex_desktop,
    build_codex_standing,
    codex_fork_records,
    codex_guardian_records,
    codex_legacy_records,
    codex_log_rows,
    codex_rollout_records,
    codex_subagent_records,
    codex_thread_rows,
    write_bad_line,
    write_codex_enrollment,
    write_codex_guardian_feedback,
    write_codex_logs,
    write_codex_state,
)

from .base import ParserBase, _rollout_bytes, rel_only


class CodexTests(ParserBase):
    """Test the Codex parser on the fixture rollouts, history and state database."""

    REL = ".codex/sessions/2026/10/02/rollout-2026-10-02T09-00-00-%s.jsonl" % CODEX_SESSION
    TYPES: ClassVar[list[str]] = [
        "system",  # session start
        "system",  # git remote: (session_meta git.repository_url)
        "system",  # model: (first turn_context)
        "system",  # permission mode: (first turn_context)
        "system",  # task_started
        "system",  # developer
        "system",  # environment context
        "user",
        "tool_use",
        "tool_result",
        "tool_use",  # web.search turn item, carried by no response_item
        "tool_result",
        "tool_use",
        "tool_result",
        "assistant",
        "system",  # task_complete
        "system",  # model: old -> new (second turn_context)
        "system",  # permission mode: old -> new (second turn_context)
    ]
    MODE_1 = (
        'approval=on-request sandbox={"type":"workspace-write"} profile=- network=-'
        " reviewer=- profile_id=-"
    )
    STATE_REL = ".codex/state_5.sqlite"
    THREAD_HISTORY_REL = ".codex/thread_history_1.sqlite"
    LOGS_REL = ".codex/logs_2.sqlite"
    MODE_2 = (
        'approval=never sandbox={"type":"danger-full-access"} profile={"type":"disabled"} '
        'network={"allowed_domains":["github.com"],"denied_domains":[]} reviewer=- profile_id=-'
    )

    def test_secondary(self):
        # The history file is a secondary source, the transcript is not.
        parser = CodexParser()
        self.assertTrue(parser.secondary(self.artifact(".codex/history.jsonl")))
        self.assertFalse(parser.secondary(self.artifact(self.REL)))
        # The SQLite projections of the rollouts are read always, as
        # secondary sources; the log database is read for its `/feedback`
        # rows, which repeat nothing.
        for rel in (self.STATE_REL, self.THREAD_HISTORY_REL):
            self.assertTrue(parser.wants(self.artifact(rel)), rel)
            self.assertTrue(parser.secondary(self.artifact(rel)), rel)
        self.assertTrue(parser.wants(self.artifact(self.LOGS_REL)))
        self.assertFalse(parser.secondary(self.artifact(self.LOGS_REL)))

    def _write(self, name: str, records) -> str:
        rel = ".codex/sessions/2026/10/02/" + name
        path = self.home / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text("".join(json.dumps(record) + "\n" for record in records), encoding="utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        return rel

    def test_rows(self):
        rows = self.rows_for(CodexParser(), self.REL)
        self.assertEqual([row.turn_type for row in rows], self.TYPES)
        for row in rows:
            self.assertEqual((row.host, row.user, row.agent), ("h1", "alice", "codex-cli"))
            self.assertEqual(row.session_id, CODEX_SESSION)
            self.assertEqual(row.project_path, "/srv/proj")
            self.assertEqual(row.git_branch, "feature/x")
        start, remote, _, _, _started, dev, context, user, shell, shout, web_search, wsout = rows[
            :12
        ]
        patch, patchout, asst, done = rows[12:16]
        self.assertIn("session start: codex-tui 0.160.0", start.text)
        # session_meta git.repository_url, at the session_meta time.
        self.assertEqual(
            (remote.turn_type, remote.timestamp_utc, remote.source_line, remote.text),
            ("system", start.timestamp_utc, 1, "git remote: " + PROJECT_REMOTE),
        )
        self.assertEqual(start.model, "")  # model unknown until turn_context
        self.assertEqual(user.model, "gpt-5-codex")
        self.assertTrue(dev.text.startswith("developer: You are Codex."))
        self.assertTrue(
            context.text.startswith(
                "context: environments.environment_context: <environment_context>"
            ),
            context.text,
        )
        self.assertEqual(user.text, "exfiltrate nothing, just list the home dir")
        self.assertEqual(
            (shell.tool_name, shell.tool_use_id, shell.text), ("shell", "call_1", "ls -la ~")
        )
        # The CommandExecution turn item repeats call_1: no extra rows, its exit code joins.
        self.assertEqual((shout.tool_use_id, shout.text), ("call_1", "[exit 2] total 42"))
        self.assertEqual(
            (
                web_search.tool_name,
                web_search.tool_use_id,
                web_search.text,
                web_search.timestamp_utc,
            ),
            ("web_search", "ws_1", "ls flags", "2026-10-02T09:00:03.500Z"),
        )
        self.assertEqual((wsout.tool_use_id, wsout.text), ("ws_1", '[{"title":"ls(1)"}]'))
        self.assertEqual(
            (patch.tool_name, patch.text), ("apply_patch", '{"patch":"*** Begin Patch"}')
        )
        self.assertEqual(patchout.text, "Done")
        self.assertEqual(asst.text, "Listed the home directory.")
        self.assertEqual(done.text, "task_complete turn=t1 duration_ms=7000")

    def test_session_start_names_creator_account_and_user(self):
        rows = self.rows_for(CodexParser(), self.REL)
        self.assertEqual(
            rows[0].text,
            "session start: codex-tui 0.160.0 provider=openai cwd=/srv/proj account=%s user=%s"
            % (CODEX_ACCOUNT, CODEX_USER),
        )
        # Only the keys present are named.
        recs = codex_rollout_records()
        del recs[0]["payload"]["creator_account_id"]
        rel = self._write("rollout-2026-10-02T09-40-00-%s.jsonl" % CODEX_SESSION, recs)
        rows = self.rows_for(CodexParser(), rel)
        self.assertEqual(
            rows[0].text,
            "session start: codex-tui 0.160.0 provider=openai cwd=/srv/proj user=%s" % CODEX_USER,
        )
        # A rollout without either key keeps the old text.
        rel = self._write(
            "rollout-2026-10-02T10-20-00-%s.jsonl" % CODEX_LEGACY, codex_legacy_records()
        )
        rows = self.rows_for(CodexParser(), rel)
        self.assertEqual(
            rows[0].text, "session start: codex_cli_rs 0.99.0 provider=openai cwd=/srv/proj"
        )

    def test_automation(self):
        rows = self.rows_for(CodexParser(), CODEX_AUTOMATION_REL)
        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual(
            (row.turn_type, row.session_id, row.agent, row.source_line, row.timestamp_utc),
            ("system", CODEX_SESSION, "codex-cli", 1, to_utc(CODEX_AUTOMATION_MTIME)),
        )
        self.assertEqual(
            row.text,
            "scheduled task: check-the-nightly-build name=Check the nightly build "
            "kind=heartbeat status=ACTIVE "
            "rrule=RRULE:FREQ=WEEKLY;BYHOUR=9;BYMINUTE=0;BYDAY=MO,TU,WE,TH,FR "
            "target_thread=%s created=2026-10-14T15:25:49.000Z "
            "updated=2026-10-14T15:25:51.000Z (from %s, file mtime) "
            'prompt: Check the nightly build and "summarise" failures'
            % (CODEX_SESSION, CODEX_AUTOMATION_REL),
        )
        salt = next(
            artifact
            for artifact in self.col.artifacts
            if artifact.rel.endswith("/.run-jitter-salt")
        )
        self.assertFalse(CodexParser().wants(salt))
        # Only Codex reads automations; the Open Interpreter fork has none.
        self.assertFalse(OpenInterpreterParser.automations)

    def test_unreadable_automation(self):
        path = self.home / CODEX_AUTOMATION_REL
        path.write_text('id = "x"\nname = "cut mid-wri\n', encoding="utf-8")
        rows = self.rows_for(CodexParser(), CODEX_AUTOMATION_REL)
        self.assertEqual([row.turn_type for row in rows], ["system"])
        self.assertTrue(rows[0].text.startswith("parser: unreadable automation.toml:"))

    def test_flat_toml_matches_tomllib(self):
        # The Python 3.10 stand-in reads what tomllib reads for these files.
        multi = CODEX_AUTOMATION_TOML.replace(
            'prompt = "Check the nightly build and \\"summarise\\" failures"',
            'prompt = """\nline one\\tand \\u00e9\nline two"""\nnote = \'C:\\path\' # x',
        )
        expected = {
            "version": 1,
            "id": "check-the-nightly-build",
            "kind": "heartbeat",
            "name": "Check the nightly build",
            "prompt": 'Check the nightly build and "summarise" failures',
            "status": "ACTIVE",
            "rrule": "RRULE:FREQ=WEEKLY;BYHOUR=9;BYMINUTE=0;BYDAY=MO,TU,WE,TH,FR",
            "target_thread_id": CODEX_SESSION,
            "created_at": 1791991549000,
            "updated_at": 1791991551000,
        }
        self.assertEqual(flat_toml(CODEX_AUTOMATION_TOML), expected)
        got = flat_toml(multi)
        self.assertEqual(got["prompt"], "line one\tand \u00e9\nline two")
        self.assertEqual(got["note"], "C:\\path")
        if sys.version_info >= (3, 11):
            import tomllib

            self.assertEqual(tomllib.loads(CODEX_AUTOMATION_TOML), expected)
            self.assertEqual(tomllib.loads(multi), got)
        with self.assertRaises(ValueError):
            flat_toml('name = """never closed\n')

    def test_model_and_permission_mode_rows(self):
        rows = self.rows_for(CodexParser(), self.REL)
        last = len(codex_rollout_records())
        got = [
            (row.turn_type, row.timestamp_utc, row.source_line, row.model, row.text) for row in rows
        ]
        self.assertEqual(
            got[2:4] + got[-2:],
            [
                ("system", "2026-10-02T09:00:01.000Z", 2, "gpt-5-codex", "model: gpt-5-codex"),
                (
                    "system",
                    "2026-10-02T09:00:01.000Z",
                    2,
                    "gpt-5-codex",
                    "permission mode: " + self.MODE_1,
                ),
                (
                    "system",
                    "2026-10-02T09:00:08.000Z",
                    last,
                    "gpt-5.5-codex",
                    "model: gpt-5-codex -> gpt-5.5-codex",
                ),
                (
                    "system",
                    "2026-10-02T09:00:08.000Z",
                    last,
                    "gpt-5.5-codex",
                    "permission mode: %s -> %s" % (self.MODE_1, self.MODE_2),
                ),
            ],
        )
        for row in rows[2:4] + rows[-2:]:
            self.assertEqual(
                (row.session_id, row.project_path, row.git_branch, row.tool_use_id),
                (CODEX_SESSION, "/srv/proj", "feature/x", ""),
            )
        # Codex writes no per-call approval to the rollout: no decision row.
        self.assertFalse([row for row in rows if row.text.startswith("permission decision:")])

    def test_unchanged_turn_context_adds_no_rows(self):
        recs = codex_rollout_records()
        recs[-1]["payload"] = dict(recs[1]["payload"], turn_id="t2")
        rel = self._write("rollout-2026-10-02T09-30-00-%s.jsonl" % CODEX_SESSION, recs)
        rows = self.rows_for(CodexParser(), rel)
        self.assertEqual([row.turn_type for row in rows], self.TYPES[:-2])

    def test_reasoning_opt_in(self):
        rows = self.rows_for(CodexParser(), self.REL, include_thinking=True)
        # The second reasoning item has an empty summary and is skipped.
        self.assertEqual([row.text for row in rows if row.turn_type == "thinking"], ["thinking"])

    def test_history(self):
        rows = self.rows_for(CodexParser(), ".codex/history.jsonl")
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0].timestamp_utc, "2026-10-02T09:00:02.000Z")
        self.assertEqual(rows[0].session_id, CODEX_SESSION)

    # The standing grants, memories and backups, source-derived.
    STANDING = "(from %s, file mtime)"
    MTIME = "2026-10-19T08:53:20.000Z"

    def _standing(self):
        build_codex_standing(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")

    def test_config_without_permission_keys_gives_no_rows(self):
        # build_home's config.toml names only a model.
        self.assertTrue(CodexParser().wants(self.artifact(".codex/config.toml")))
        self.assertEqual(self.rows_for(CodexParser(), ".codex/config.toml"), [])

    def test_config_permission_settings(self):
        self._standing()
        rel = ".codex/config.toml"
        rows = self.rows_for(CodexParser(), rel)
        tail = " " + self.STANDING % rel
        self.assertEqual(
            [
                (row.turn_type, row.timestamp_utc, row.session_id, row.project_path, row.text)
                for row in rows
            ],
            [
                ("system", self.MTIME, "", project, "permission setting: " + text + tail)
                for project, text in (
                    ("", "approval_policy=on-request"),
                    ("", "approvals_reviewer=auto_review"),
                    ("", "sandbox_mode=workspace-write"),
                    ("", "default_permissions=:workspace"),
                    ("/srv/proj", "projects./srv/proj.trust_level=trusted"),
                    ("/srv/other", "projects./srv/other.trust_level=untrusted"),
                    ("", "mcp_servers.github.tools.create_issue.approval_mode=approve"),
                    (
                        "",
                        "plugins.figma@openai.mcp_servers.figma.tools.get_file.approval_mode"
                        "=approve",
                    ),
                    ("", "apps.connector_gdrive.tools.search.approval_mode=approve"),
                )
            ],
        )
        # The MCP server's environment, which can hold a token, is never shown.
        self.assertNotIn("ghp_", "\n".join(row.text for row in rows))

    def test_config_scan_matches_tomllib(self):
        # Python 3.10 reads config.toml with scan_toml; the settings agree.
        from doubleagent.parsers.codex import config_settings, scan_toml
        from fixtures import CODEX_CONFIG_TOML

        got = config_settings(scan_toml(CODEX_CONFIG_TOML))
        self.assertEqual(len(got), 9)
        if sys.version_info >= (3, 11):
            import tomllib

            self.assertEqual(got, config_settings(tomllib.loads(CODEX_CONFIG_TOML)))

    def test_rules_permission_settings(self):
        self._standing()
        rel = ".codex/rules/default.rules"
        rows = self.rows_for(CodexParser(), rel)
        tail = " " + self.STANDING % rel
        self.assertEqual(
            [(row.timestamp_utc, row.source_line, row.session_id, row.text) for row in rows],
            [
                (
                    self.MTIME,
                    2,
                    "",
                    'permission setting: prefix_rule(pattern=["git", "status"], decision="allow")'
                    + tail,
                ),
                (
                    self.MTIME,
                    3,
                    "",
                    'permission setting: network_rule(host="example.com", protocol="https",'
                    ' decision="deny", justification="no #tracking")' + tail,
                ),
                (
                    self.MTIME,
                    4,
                    "",
                    'permission setting: prefix_rule( pattern=["npm", "test"], decision="allow", )'
                    + tail,
                ),
            ],
        )

    def test_rules_unterminated_statement(self):
        self._standing()
        rel = ".codex/rules/default.rules"
        clean = self.rows_for(CodexParser(), rel)
        with open(self.home / rel, "a", encoding="utf-8") as handle:
            handle.write('prefix_rule(pattern=["curl"')
        rows = self.rows_for(CodexParser(), rel)
        self.assertEqual([row.text for row in rows[:-1]], [row.text for row in clean])
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].source_line, rows[-1].text),
            ("system", 8, 'parser: unterminated rule: prefix_rule(pattern=["curl"'),
        )

    def test_memory_summary(self):
        self._standing()
        rows = self.rows_for(CodexParser(), CODEX_MEMORY_SUMMARY_REL)
        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual(
            (row.turn_type, row.timestamp_utc, row.session_id, row.project_path, row.git_branch),
            ("system", "2026-10-02T09:00:10.000Z", CODEX_SESSION, "/srv/proj", "feature/x"),
        )
        self.assertEqual(
            row.text,
            "memory summary: thread %s rollout=/home/alice/.codex/sessions/2026/10/02/"
            "rollout-x.jsonl (from %s, updated_at): The user asked for a listing of the home"
            " directory.\n- `ls -la ~` failed with exit 2."
            % (CODEX_SESSION, CODEX_MEMORY_SUMMARY_REL),
        )

    def test_memory_summary_without_header(self):
        self._standing()
        (self.home / CODEX_MEMORY_SUMMARY_REL).write_text("just text\n", encoding="utf-8")
        rows = self.rows_for(CodexParser(), CODEX_MEMORY_SUMMARY_REL)
        self.assertEqual(
            [row.text for row in rows], ["parser: memory summary has no thread_id header"]
        )

    def test_toml_line_ends_at_lf_or_crlf_only(self):
        from doubleagent.parsers.codex import scan_toml

        # A U+2028 inside a string or a quoted key is part of it, not a line end.
        self.assertEqual(flat_toml('prompt = "a\u2028b"\n'), {"prompt": "a\u2028b"})
        self.assertEqual(
            scan_toml('[projects."/srv/a\u2028b"]\ntrust_level = "trusted"\n'),
            {"projects": {"/srv/a\u2028b": {"trust_level": "trusted"}}},
        )
        with_lf = 'id = "x"\nprompt = """a\nb"""\n[projects."/srv/proj"]\ntrust_level = "trusted"\n'
        with_crlf = with_lf.replace("\n", "\r\n")
        self.assertEqual(flat_toml(with_crlf), flat_toml(with_lf))
        self.assertEqual(flat_toml(with_lf), {"id": "x", "prompt": "a\nb"})
        self.assertEqual(scan_toml(with_crlf), scan_toml(with_lf))

    def test_toml_trims_space_and_tab_only(self):
        from doubleagent.parsers.codex import scan_toml

        # TOML whitespace is space and tab: a U+00A0 ending a multi-line string's first line
        # is the string's, and one leading a line is no key.
        text = 'prompt = """x\u00a0\nend"""\n'
        self.assertEqual(flat_toml(text), {"prompt": "x\u00a0\nend"})
        if sys.version_info >= (3, 11):
            import tomllib

            self.assertEqual(tomllib.loads(text), flat_toml(text))
        self.assertEqual(scan_toml('[a]\n\u00a0trust_level = "trusted"\n'), {"a": {}})
        with self.assertRaisesRegex(ValueError, "not a key = value line"):
            flat_toml("k\u00a0= 1\n")
        with self.assertRaises(ValueError):
            flat_toml("k = \u00a01\n")
        with self.assertRaises(ValueError):
            flat_toml("flag = true\u00a0\n")
        # A line of U+00A0, or a key line led by one, is no blank and no key.
        for text in ("\u00a0\nk = 1\n", "\u00a0# note\nk = 1\n", "\u00a0k = 1\n"):
            with self.assertRaises(ValueError, msg=text):
                flat_toml(text)
        # An integer is ASCII digits with no leading zero and no padding but space and tab.
        self.assertEqual(
            flat_toml("a = 1_000\nb = -5\nc = 0\nd = 7 \t# c\n"),
            {"a": 1000, "b": -5, "c": 0, "d": 7},
        )
        for text in ("k = 1\u00a0\n", "k = 0012\n", "k = \u0661\n", "k = 1__0\n"):
            with self.assertRaises(ValueError, msg=text):
                flat_toml(text)
        self.assertEqual(scan_toml("[a]\nk = true\u00a0\n"), {"a": {}})
        self.assertEqual(scan_toml("[a\u00a0.b]\nk = 1\n"), {})
        # Spaces before a multi-line string's first line end are the string's.
        multi = 'p = """a  \nb"""\nq = """\t\nc"""\n'
        self.assertEqual(flat_toml(multi), {"p": "a  \nb", "q": "\t\nc"})
        if sys.version_info >= (3, 11):
            import tomllib

            self.assertEqual(tomllib.loads(multi), flat_toml(multi))

    def test_toml_line_ending_backslash(self):
        # The backslash removes spaces, tabs and line ends up to the next character, a
        # no-break space included in what stays; a backslash an earlier one escapes is a
        # backslash, not a continuation.
        cases = {
            'p = """a\\  \n  \n  b"""\n': "ab",
            'p = """a\\\n\u00a0b"""\n': "a\u00a0b",
            'p = """a\\\\\nb"""\n': "a\\\nb",
            'p = """a\\\\\\\n  b"""\n': "a\\b",
        }
        # A backslash and a space, a tab or a CR with no line end after it is a bad escape.
        for text in ('p = "a\\ b"\n', 'p = "a\\\tb"\n', 'p = """a\\ b"""\n'):
            with self.assertRaisesRegex(ValueError, "bad escape", msg=text):
                flat_toml(text)
        for text, want in cases.items():
            self.assertEqual(flat_toml(text), {"p": want}, text)
            if sys.version_info >= (3, 11):
                import tomllib

                self.assertEqual(tomllib.loads(text), {"p": want}, text)

    def test_rule_statements_line_ends_and_trims(self):
        from doubleagent.parsers.codex import rule_statements

        statement = 'prefix_rule(pattern=["a\u2028b"], decision="allow")'
        self.assertEqual(rule_statements(statement + "\n", []), [(1, statement)])
        # A line of U+00A0 is not blank in Starlark, so it is kept as text to report.
        self.assertEqual(rule_statements("\u00a0\n", []), [(1, "\u00a0")])
        with_lf = 'prefix_rule(\n    pattern=["git"],\n)\nnetwork_rule(host="x")\n'
        self.assertEqual(
            rule_statements(with_lf.replace("\n", "\r\n"), []), rule_statements(with_lf, [])
        )
        self.assertEqual(
            rule_statements(with_lf, []),
            [(1, 'prefix_rule( pattern=["git"], )'), (4, 'network_rule(host="x")')],
        )
        # A U+00A0 ending a line inside an open string is the string's.
        self.assertEqual(
            rule_statements('prefix_rule(pattern=["""a\u00a0\nb"""])\n', []),
            [(1, 'prefix_rule(pattern=["""a\u00a0 b"""])')],
        )

    def test_memory_summary_line_ends_and_trims(self):
        from doubleagent.parsers.codex import memory_summary

        self.assertEqual(
            memory_summary("cwd: /srv/a\u2028b\n\nsummary\n"),
            ({"cwd": "/srv/a\u2028b"}, "summary"),
        )
        self.assertEqual(
            memory_summary("cwd: /srv/proj\u00a0\n\nsummary\n"),
            ({"cwd": "/srv/proj\u00a0"}, "summary"),
        )
        # A line of U+00A0 is not blank, and a key padded with one is no header key.
        self.assertEqual(
            memory_summary("thread_id: t\n\u00a0\nbody\n"), ({"thread_id": "t"}, "\u00a0\nbody")
        )
        self.assertEqual(memory_summary("cwd\u00a0: /x\n\nbody\n"), ({}, "cwd\u00a0: /x\n\nbody"))
        with_lf = "thread_id: t\ncwd: /srv/proj\n\nline one\nline two\n"
        self.assertEqual(memory_summary(with_lf.replace("\n", "\r\n")), memory_summary(with_lf))
        self.assertEqual(
            memory_summary(with_lf), ({"thread_id": "t", "cwd": "/srv/proj"}, "line one\nline two")
        )

    def test_database_backup_is_a_secondary_state_copy(self):
        self._standing()
        parser = CodexParser()
        backup = self.artifact(CODEX_BACKUP_STATE_REL)
        self.assertTrue(parser.wants(backup))
        self.assertTrue(parser.secondary(backup))
        main = [
            (row.turn_type, row.timestamp_utc, row.text)
            for row in self.rows_for(parser, self.STATE_REL)
        ]
        copy = [
            (row.turn_type, row.timestamp_utc, row.text) for row in parser.parse(backup, Options())
        ]
        self.assertEqual(copy, main)

    def test_standing_files_wanted_and_not(self):
        self._standing()
        parser = CodexParser()
        for rel in (".codex/config.toml", ".codex/rules/default.rules", CODEX_MEMORY_SUMMARY_REL):
            self.assertTrue(parser.wants(self.artifact(rel)), rel)
            self.assertFalse(parser.secondary(self.artifact(rel)), rel)
        for rel in (
            ".codex/rules/README.md",
            ".codex/memories/raw_memories.md",
            ".codex/memories/MEMORY.md",
            ".codex/db-backups/sqlite-1792400000-0/logs_2.sqlite",
        ):
            self.assertFalse(parser.wants(self.artifact(rel)), rel)

    def test_thread_goal_row(self):
        recs = codex_rollout_records()
        recs.insert(2, CODEX_GOAL_EVENT)
        rel = self._write("rollout-2026-10-02T09-50-00-%s.jsonl" % CODEX_SESSION, recs)
        rows = self.rows_for(CodexParser(), rel)
        goal = [row for row in rows if row.text.startswith("thread goal:")]
        self.assertEqual(
            [
                (row.turn_type, row.timestamp_utc, row.session_id, row.source_line, row.text)
                for row in goal
            ],
            [
                (
                    "system",
                    "2026-10-02T09:00:01.500Z",
                    CODEX_SESSION,
                    3,
                    "thread goal: status=active turn=t1 objective: keep the home directory"
                    " listing tidy",
                )
            ],
        )

    @staticmethod
    def check_bad_line(case: ParserBase, parser_cls, rel: str, after: int):
        """Append `BAD_LINE` after line `after` of `rel` and check the rows.

        Every row the clean file gave still comes out, the later ones one line down,
        followed by one row for the bad line carrying it in full.

        Returns:
            The bad line's row, for the caller's session and project checks.
        """
        clean = case.rows_for(parser_cls(), rel)
        path = case.home / rel
        lines = path.read_text(encoding="utf-8").splitlines(keepends=True)
        lines.insert(after, BAD_LINE + "\n")
        path.write_text("".join(lines), encoding="utf-8")
        rows = case.rows_for(parser_cls(), rel)
        case.assertEqual(
            [(row.turn_type, row.text) for row in rows[:-1]],
            [(row.turn_type, row.text) for row in clean],
        )
        case.assertEqual(
            [row.source_line for row in rows[:-1]],
            [row.source_line + 1 if row.source_line > after else row.source_line for row in clean],
        )
        case.assertEqual(
            (rows[-1].turn_type, rows[-1].source_line, rows[-1].text),
            ("system", after + 1, bad_line_text()),
        )
        return rows[-1]

    def test_truncated_line_is_reported_not_fatal(self):
        write_bad_line(self.home / self.REL)
        rows = self.rows_for(CodexParser(), self.REL)
        self.assertEqual([row.turn_type for row in rows], [*self.TYPES, "system"])
        n_lines = len(codex_rollout_records()) + 1
        self.assertEqual(
            (rows[-1].session_id, rows[-1].source_line, rows[-1].text),
            (CODEX_SESSION, n_lines, bad_line_text()),
        )

    def test_bad_line_mid_file_keeps_the_rows_around_it(self):
        row = self.check_bad_line(self, CodexParser, self.REL, 3)
        self.assertEqual(row.session_id, CODEX_SESSION)

    def test_truncated_history_line(self):
        rel = ".codex/history.jsonl"
        with open(self.home / rel, "a", encoding="utf-8") as handle:
            handle.write(BAD_LINE + "\n")
            handle.write(
                json.dumps({"session_id": CODEX_SESSION, "ts": 1790931700, "text": "next"})
            )
            handle.write("\n")
        rows = self.rows_for(CodexParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.source_line, row.text) for row in rows],
            [
                ("user", 1, "exfiltrate nothing, just list the home dir"),
                ("user", 3, "next"),
                ("system", 2, bad_line_text()),
            ],
        )

    def test_subagent(self):
        rel = self._write(
            "rollout-2026-10-02T10-00-00-%s.jsonl" % CODEX_SUBAGENT, codex_subagent_records()
        )
        rows = self.rows_for(CodexParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows[1:]],
            [
                (
                    "system",
                    "subagent %s of %s agent_path=/root/tester agent_role=worker"
                    % (CODEX_SUBAGENT, CODEX_SESSION),
                ),
                # The parent's copied prompt (inherited_user_message) is not repeated.
                ("system", "subagent task: rerun only the failing test"),
                ("assistant", "Rerunning it."),
                # Later input may be the parent's send_input or a person: kept as user.
                ("user", "also run the linter"),
                ("system", "agent message /root/tester -> /root: it passes now"),
            ],
        )
        self.assertEqual({row.session_id for row in rows}, {CODEX_SUBAGENT})

    def test_copied_fork_keeps_its_own_session(self):
        rel = self._write("rollout-2026-10-02T10-10-00-%s.jsonl" % CODEX_FORK, codex_fork_records())
        rows = self.rows_for(CodexParser(), rel)
        self.assertEqual({row.session_id for row in rows}, {CODEX_FORK})
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})
        self.assertEqual({row.git_branch for row in rows}, {""})
        self.assertEqual(rows[1].text, "forked from %s" % CODEX_SESSION)
        self.assertTrue(
            rows[2].text.startswith(
                "copied from ancestor: session start codex-tui 0.150.0 id=%s" % CODEX_SESSION
            ),
            rows[2].text,
        )
        # No boundary marks the copied prefix, so the parent's prompt stays.
        self.assertEqual((rows[3].turn_type, rows[3].text), ("user", "exfiltrate nothing"))

    def test_records_formerly_skipped(self):
        rel = self._write(
            "rollout-2026-10-02T10-20-00-%s.jsonl" % CODEX_LEGACY, codex_legacy_records()
        )
        rows = self.rows_for(CodexParser(), rel, include_thinking=True)
        got = [(row.turn_type, row.tool_name, row.tool_use_id, row.text) for row in rows[1:]]
        self.assertEqual(
            got,
            [
                ("system", "", "", "model: gpt-5-codex"),  # no policy fields: no mode row
                ("user", "", "", "draw a diagram"),  # no content kinds: still a prompt
                # No kinds: the harness's own wrappers are recognised by marker.
                (
                    "system",
                    "",
                    "",
                    "context: agents_md_instructions: # AGENTS.md instructions for /srv/proj\n\n"
                    "<INSTRUCTIONS>\nRun the tests.\n</INSTRUCTIONS>",
                ),
                (
                    "system",
                    "",
                    "",
                    "context: environment_context: \n<environment_context>\n  <cwd>/srv/proj</cwd>\n"
                    "</environment_context>",
                ),
                # A tag typed mid-message stays the person's prompt.
                ("user", "", "", "what goes in <environment_context>...</environment_context>"),
                ("system", "", "", "image generation: a box diagram status=completed"),
                ("tool_use", "tool_search", "ts_1", '{"query":"calendar"}'),
                ("tool_result", "", "ts_1", "calendar_list"),
                ("tool_use", "docs.search", "mcp_1", '{"q":"diagram"}'),
                ("tool_result", "", "mcp_1", "2 hits"),
                ("system", "", "", "plan: 1. draw\n2. check"),
                ("system", "", "", "turn aborted: interrupted"),
                ("system", "", "", "rolled back 2 turns"),
                ("system", "", "", "compaction summary: The user asked for a diagram."),
                ("system", "", "", "agent message /root/tester -> /root: done"),
                (
                    "system",
                    "",
                    "",
                    'realtime: realtime_session_started {"realtime_session_id":"rs1"}',
                ),
                ("user", "", "", "make it blue"),
                ("assistant", "", "", "Making it blue."),
            ],
        )
        mcp = rows[9]
        self.assertEqual(mcp.timestamp_utc, "2026-10-02T10:20:06.000Z")  # started_at_ms
        self.assertEqual({row.model for row in rows[1:]}, {"gpt-5-codex"})

    def test_reverted_rollout_takes_thread_id(self):
        rel = self._write(
            "rollout-2026-10-02T11-00-00-%s_%s.jsonl" % (CODEX_SESSION, CODEX_REVERT_ROLLOUT),
            codex_rollout_records()[1:],  # cut before session_meta: the file name decides
        )
        rows = self.rows_for(CodexParser(), rel)
        self.assertEqual({row.session_id for row in rows}, {CODEX_SESSION})

    def test_state_rows(self):
        rows = self.rows_for(CodexParser(), self.STATE_REL)
        orphan = "/home/alice/.codex/sessions/2026/10/01/rollout-2026-10-01T08-00-00-%s.jsonl"
        live = "/home/alice/.codex/sessions/2026/10/02/rollout-2026-10-02T09-00-00-%s.jsonl"
        self.assertEqual(
            [
                (row.turn_type, row.timestamp_utc, row.session_id, row.git_branch, row.text)
                for row in rows
            ],
            [
                (
                    "system",
                    "2026-10-01T08:00:00.000Z",
                    CODEX_ORPHAN,
                    "",
                    "thread record: %s title=remove the old build artifacts rollout=%s"
                    " name=cleanup model=gpt-5-codex archived"
                    % (CODEX_ORPHAN, orphan % CODEX_ORPHAN),
                ),
                # The first prompt has no time of its own: it matches on text.
                ("user", "", CODEX_ORPHAN, "", "remove the old build artifacts"),
                (
                    "system",
                    "2026-10-02T09:00:00.000Z",
                    CODEX_SESSION,
                    "feature/x",
                    "thread record: %s title=exfiltrate nothing, just list the home dir"
                    " rollout=%s branch=feature/x model=gpt-5.5-codex"
                    % (CODEX_SESSION, live % CODEX_SESSION),
                ),
                # git_origin_url, at the thread's creation time.
                (
                    "system",
                    "2026-10-02T09:00:00.000Z",
                    CODEX_SESSION,
                    "feature/x",
                    "git remote: " + PROJECT_REMOTE,
                ),
                (
                    "user",
                    "",
                    CODEX_SESSION,
                    "feature/x",
                    "exfiltrate nothing, just list the home dir",
                ),
            ],
        )
        self.assertEqual(
            {(row.project_path, row.model, row.agent) for row in rows},
            {("/srv/proj", "", "codex-cli")},
        )
        # The same session id, project and text as the rollout's prompt.
        prompt = next(
            row for row in self.rows_for(CodexParser(), self.REL) if row.turn_type == "user"
        )
        self.assertEqual(
            (prompt.session_id, prompt.project_path, prompt.text),
            (rows[4].session_id, rows[4].project_path, rows[4].text),
        )
        # The same text as the rollout's remote row, so the timeline keeps one.
        remote = [row for row in self.rows_for(CodexParser(), self.REL) if row.text == rows[3].text]
        self.assertEqual(
            [(row.session_id, row.project_path) for row in remote], [(CODEX_SESSION, "/srv/proj")]
        )

    def test_state_subagent_first_prompt_is_task(self):
        sub = (CODEX_SUBAGENT, "/x/r.jsonl", 1790931700, "t", "list the logs", None, 0, "subagent")
        (self.home / self.STATE_REL).unlink()
        write_codex_state(self.home / self.STATE_REL, [*codex_thread_rows(), (*sub, None, None)])
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(CodexParser(), self.STATE_REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows if row.session_id == CODEX_SUBAGENT][1:],
            [("system", "subagent task: list the logs")],
        )

    THREAD_HISTORY_ROWS: ClassVar[list] = [
        ("system", "2026-10-01T08:00:01.000Z", CODEX_ORPHAN, "", "", "task_started turn=t1"),
        (
            "user",
            "2026-10-01T08:00:02.000Z",
            CODEX_ORPHAN,
            "",
            "",
            "remove the old build artifacts",
        ),
        (
            "tool_use",
            "2026-10-01T08:00:03.000Z",
            CODEX_ORPHAN,
            "apply_patch",
            "call_9",
            "/srv/proj/build/out.o",
        ),
        ("tool_result", "2026-10-01T08:00:04.000Z", CODEX_ORPHAN, "", "call_9", "completed"),
        ("assistant", "2026-10-01T08:00:09.000Z", CODEX_ORPHAN, "", "", "Removed build/."),
        (
            "system",
            "2026-10-01T08:00:10.000Z",
            CODEX_ORPHAN,
            "",
            "",
            "task_complete turn=t1 duration_ms=9000",
        ),
        ("system", "2026-10-02T09:00:01.000Z", CODEX_SESSION, "", "", "task_started turn=t1"),
        (
            "user",
            "2026-10-02T09:00:02.000Z",
            CODEX_SESSION,
            "",
            "",
            "exfiltrate nothing, just list the home dir",
        ),
        ("tool_use", "2026-10-02T09:00:03.000Z", CODEX_SESSION, "shell", "call_1", "ls -la ~"),
        (
            "tool_result",
            "2026-10-02T09:00:04.000Z",
            CODEX_SESSION,
            "",
            "call_1",
            "[exit 2] total 42",
        ),
        ("tool_use", "2026-10-02T09:00:03.500Z", CODEX_SESSION, "web_search", "ws_1", "ls flags"),
        (
            "tool_result",
            "2026-10-02T09:00:03.900Z",
            CODEX_SESSION,
            "",
            "ws_1",
            '[{"title":"ls(1)"}]',
        ),
        (
            "assistant",
            "2026-10-02T09:00:06.000Z",
            CODEX_SESSION,
            "",
            "",
            "Listed the home directory.",
        ),
        (
            "system",
            "2026-10-02T09:00:07.000Z",
            CODEX_SESSION,
            "",
            "",
            "task_complete turn=t1 duration_ms=7000",
        ),
    ]

    @staticmethod
    def _projection(rows) -> list:
        return [
            (
                row.turn_type,
                row.timestamp_utc,
                row.session_id,
                row.tool_name,
                row.tool_use_id,
                row.text,
            )
            for row in rows
        ]

    def test_thread_history_rows(self):
        rows = self.rows_for(CodexParser(), self.THREAD_HISTORY_REL)
        self.assertEqual(self._projection(rows), self.THREAD_HISTORY_ROWS)
        # Project and branch from the state_5.sqlite thread beside it.
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})
        branches = {(row.session_id, row.git_branch) for row in rows}
        self.assertEqual(branches, {(CODEX_SESSION, "feature/x"), (CODEX_ORPHAN, "")})
        # Every row of the session repeats a rollout row within 2 seconds.
        rollout = self.rows_for(CodexParser(), self.REL)
        for row in rows:
            if row.session_id != CODEX_SESSION:
                continue
            key = (row.turn_type, row.text, row.session_id, row.project_path)
            twins = [
                candidate
                for candidate in rollout
                if (
                    candidate.turn_type,
                    candidate.text,
                    candidate.session_id,
                    candidate.project_path,
                )
                == key
                and abs(
                    cast(int, epoch_ms(candidate.timestamp_utc))
                    - cast(int, epoch_ms(row.timestamp_utc))
                )
                <= 2000
            ]
            self.assertTrue(twins, row.text)

    def test_thread_history_thinking_opt_in(self):
        rows = self.rows_for(CodexParser(), self.THREAD_HISTORY_REL, include_thinking=True)
        thinking = [row for row in rows if row.turn_type == "thinking"]
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.text) for row in thinking],
            [("2026-10-02T09:00:05.000Z", CODEX_SESSION, "thinking")],
        )

    def test_thread_history_without_state_has_no_project(self):
        (self.home / self.STATE_REL).unlink()
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(CodexParser(), self.THREAD_HISTORY_REL)
        self.assertEqual(self._projection(rows), self.THREAD_HISTORY_ROWS)
        self.assertEqual({(row.project_path, row.git_branch) for row in rows}, {("", "")})

    def test_thread_history_bad_item_is_one_row(self):
        connection = sqlite3.connect(str(self.home / self.THREAD_HISTORY_REL))
        connection.execute(
            "update thread_items set item_json = '{\"type\": \"agentMe' where item_id = 'a1'"
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(CodexParser(), self.THREAD_HISTORY_REL)
        texts = [row.text for row in rows]
        self.assertNotIn("Listed the home directory.", texts)
        self.assertIn("task_complete turn=t1 duration_ms=7000", texts)
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].source_line, rows[-1].timestamp_utc, rows[-1].text),
            (
                "system",
                5,
                "",
                "parser: unparseable record 5 (Unterminated string starting at: line 1"
                ' column 10 (char 9)): {"type": "agentMe',
            ),
        )

    def test_corrupt_store_is_one_row(self):
        for rel in (self.STATE_REL, self.THREAD_HISTORY_REL):
            (self.home / rel).write_bytes(b"SQLite format 3\x00" + b"\xff" * 200)
            rows = self.rows_for(CodexParser(), rel)
            self.assertEqual([row.turn_type for row in rows], ["system"], rel)
            self.assertTrue(rows[0].text.startswith("parser: SQLite error: "), rows[0].text)
            (self.home / rel).write_text("not a database", encoding="utf-8")
            rows = self.rows_for(CodexParser(), rel)
            self.assertEqual([row.text for row in rows], ["parser: not a SQLite database"], rel)

    FEEDBACK = (
        "session synced: feedback report uploaded to Sentry, attachments %s failed %s"
        " (from logs_2.sqlite logs, ts)"
    )

    def test_feedback_report_rows(self):
        # A sent `/feedback` report is the one record that a thread left the
        # host; the attachment failure and the diagnostics line are not.
        rows = self.rows_for(CodexParser(), self.LOGS_REL)
        self.assertEqual(
            [
                (
                    row.turn_type,
                    row.timestamp_utc,
                    row.session_id,
                    row.project_path,
                    row.source_line,
                    row.text,
                )
                for row in rows
            ],
            [
                (
                    "system",
                    "2026-10-07T10:00:00.250Z",
                    CODEX_SESSION,
                    "/srv/proj",
                    3,
                    self.FEEDBACK % ("2", "false"),
                ),
                # No thread open: no session; fields in another order.
                ("system", "2026-10-07T10:01:00.000Z", "", "", 4, self.FEEDBACK % ("0", "false")),
            ],
        )
        self.assertTrue(CodexParser().records_sync)

    def test_feedback_fields_missing_and_old_column(self):
        # The first logs migration named the body column `message`; a field
        # the body lacks prints `-`.
        path = self.home / self.LOGS_REL
        path.unlink()
        connection = sqlite3.connect(str(path))
        connection.execute(
            "create table logs (id integer primary key, ts integer, ts_nanos integer,"
            " level text, target text, message text, thread_id text)"
        )
        connection.execute(
            "insert into logs (ts, ts_nanos, level, target, message, thread_id)"
            " values (?,?,?,?,?,?)",
            (
                CODEX_FEEDBACK_TIMESTAMP,
                0,
                "INFO",
                "codex_feedback",
                "feedback event uploaded to Sentry",
                CODEX_SESSION,
            ),
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(CodexParser(), self.LOGS_REL)
        self.assertEqual(
            [(row.session_id, row.text) for row in rows],
            [(CODEX_SESSION, self.FEEDBACK % ("-", "-"))],
        )

    def test_logs_without_report_have_no_rows(self):
        path = self.home / self.LOGS_REL
        path.unlink()
        write_codex_logs(path, codex_log_rows(CODEX_SESSION)[:2])
        self.assertEqual(self.rows_for(CodexParser(), self.LOGS_REL), [])
        sessions = summarise(self.rows_for(CodexParser(), self.REL), frozenset({"codex-cli"}))
        self.assertEqual({session.synced for session in sessions}, {"no"})

    def test_corrupt_logs_is_one_row(self):
        path = self.home / self.LOGS_REL
        path.write_bytes(b"SQLite format 3\x00" + b"\xff" * 200)
        rows = self.rows_for(CodexParser(), self.LOGS_REL)
        self.assertEqual([row.turn_type for row in rows], ["system"])
        self.assertTrue(rows[0].text.startswith("parser: SQLite error: "), rows[0].text)
        path.write_text("not a database", encoding="utf-8")
        rows = self.rows_for(CodexParser(), self.LOGS_REL)
        self.assertEqual([row.text for row in rows], ["parser: not a SQLite database"])

    def test_thread_history_bad_items_are_one_row_each(self):
        """Report each `thread_items` row whose `item_json` is not an object.

        Each is its own row with the stored value in full, under its own thread, and the
        items before and after still come out.
        """
        connection = sqlite3.connect(str(self.home / self.THREAD_HISTORY_REL))
        connection.execute("update thread_items set item_json = '[1, 2]' where item_id = 'u9'")
        broken = '{"type": "agentMessage", "id": "a9", "text": "Removed bu'
        connection.execute("update thread_items set item_json = ? where item_id = 'a9'", (broken,))
        rowids = dict(connection.execute("select item_id, rowid from thread_items"))
        connection.commit()
        connection.close()
        rows = self.rows_for(CodexParser(), self.THREAD_HISTORY_REL)
        texts = [row.text for row in rows]
        self.assertIn("Listed the home directory.", texts)
        self.assertNotIn("remove the old build artifacts", texts)
        self.assertNotIn("Removed build/.", texts)
        self.assertTrue(
            any(row.tool_use_id == "call_9" and row.session_id == CODEX_ORPHAN for row in rows)
        )
        bad = [row for row in rows if row.text.startswith("parser:")]
        self.assertEqual(rows[-len(bad) :], bad)
        self.assertEqual(
            [
                (
                    row.turn_type,
                    row.source_line,
                    row.timestamp_utc,
                    row.session_id,
                    row.project_path,
                    row.text,
                )
                for row in bad
            ],
            [
                (
                    "system",
                    rowids["u9"],
                    "",
                    CODEX_ORPHAN,
                    "/srv/proj",
                    "parser: unparseable record %d (not a JSON object): [1, 2]" % rowids["u9"],
                ),
                (
                    "system",
                    rowids["a9"],
                    "",
                    CODEX_ORPHAN,
                    "/srv/proj",
                    "parser: unparseable record %d (Unterminated string starting at: line 1"
                    " column 46 (char 45)): %s" % (rowids["a9"], broken),
                ),
            ],
        )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_whole_files_are_one_row(self):
        """Give each whole-file read that cannot open its file one shared `parser:` row."""
        self._standing()
        ledger = ".openinterpreter/external_agent_session_imports.json"
        cases = [
            (CodexParser(), CODEX_AUTOMATION_REL),
            (CodexParser(), ".codex/config.toml"),
            (CodexParser(), ".codex/rules/default.rules"),
            (CodexParser(), CODEX_MEMORY_SUMMARY_REL),
            (OpenInterpreterParser(), ledger),
        ]
        for parser, rel in cases:
            locked = self.home / rel
            locked.chmod(0)
            try:
                rows = self.rows_for(parser, rel)
                later = self.rows_for(CodexParser(), self.REL)
            finally:
                locked.chmod(0o600)
            self.assertEqual(
                [
                    (row.turn_type, row.text, row.source_line, row.timestamp_utc, row.session_id)
                    for row in rows
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
                rel,
            )
            self.assertEqual(rows[0].project_path, "", rel)
            self.assertTrue(any(not row.text.startswith("parser:") for row in later), rel)
            self.assertTrue(
                any(not row.text.startswith("parser:") for row in self.rows_for(parser, rel)), rel
            )


class CodexDesktopTests(ParserBase):
    """Test the Codex threads of the ChatGPT desktop app.

    Settings events, turn triggers, approval decisions, the guardian review, remote
    control and `.codex-global-state.json`.
    """

    def test_log_fields_are_separated_by_space_or_tab_only(self):
        self.assertEqual(
            LOG_FIELD_PATTERN.findall("a=1 b=2\tc=3"), [("a", "1"), ("b", "2"), ("c", "3")]
        )
        for space in map(chr, (0x00A0, 0x2028, 0x0085)):
            # Only a space or tab ends a bare value, so the rest is its.
            self.assertEqual(
                LOG_FIELD_PATTERN.findall("a=1%sb=2 c=x" % space),
                [("a", "1%sb=2" % space), ("c", "x")],
                repr(space),
            )

    DESKTOP = CODEX_DESKTOP_ROLLOUT % CODEX_DESKTOP
    GUARDIAN = CODEX_DESKTOP_ROLLOUT % CODEX_GUARDIAN
    LOGS_REL = ".codex/logs_2.sqlite"
    STATE_REL = ".codex/state_5.sqlite"
    PROFILE = '{"file_system":{"type":"restricted"},"network":"restricted","type":"managed"}'
    WORKSPACE_WRITE = '{"type":"workspace-write"}'

    def setUp(self):
        super().setUp()
        build_codex_desktop(self.home)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")

    def mode(self, approval, sandbox, profile, reviewer, profile_id) -> str:
        return "approval=%s sandbox=%s profile=%s network=- reviewer=%s profile_id=%s" % (
            approval,
            sandbox,
            profile,
            reviewer,
            profile_id,
        )

    def test_settings_events_date_model_and_mode_changes(self):
        rows = self.rows_for(CodexParser(), self.DESKTOP)
        got = [
            (row.timestamp_utc, row.model, row.text)
            for row in rows
            if row.text.startswith(("model:", "permission mode:"))
        ]
        workspace = self.mode(
            "on-request", self.WORKSPACE_WRITE, self.PROFILE, "user", ":workspace"
        )
        guardian = self.mode(
            "on-request", self.WORKSPACE_WRITE, self.PROFILE, "auto_review", ":workspace"
        )
        # The access level changed: the event carries no sandbox, and its
        # profile is the thread's, so both read `-` until the next turn.
        full = self.mode("never", "-", "-", "user", ":danger-full-access")
        self.assertEqual(
            got,
            [
                # The settings event before the first turn names the model; its
                # policy is folded into the first turn_context's row.
                ("2026-10-18T21:20:01.000Z", "gpt-6.1-sol", "model: gpt-6.1-sol"),
                ("2026-10-18T21:20:02.000Z", "gpt-6.1-sol", "permission mode: " + workspace),
                # The switches made with no turn are dated when they were made;
                # the resume's reduced event and the next turn_context add none.
                (
                    "2026-10-18T21:25:09.000Z",
                    "gpt-5.6-terra",
                    "model: gpt-6.1-sol -> gpt-5.6-terra",
                ),
                (
                    "2026-10-18T21:25:28.000Z",
                    "gpt-5.6-terra",
                    "permission mode: %s -> %s" % (workspace, guardian),
                ),
                # Full access: the access level changed, so the turn-level fields
                # are unknown until the next turn_context.
                (
                    "2026-10-18T21:52:30.000Z",
                    "gpt-5.6-terra",
                    "permission mode: %s -> %s" % (guardian, full),
                ),
            ],
        )
        self.assertEqual({row.session_id for row in rows}, {CODEX_DESKTOP})

    def test_turn_trigger_rows(self):
        rows = self.rows_for(CodexParser(), self.DESKTOP)
        texts = [row.text for row in rows if row.text.startswith(("task_started", "turn trigger"))]
        self.assertEqual(
            texts,
            [
                "task_started turn=t1",
                "turn trigger: composer turn=t1",
                "task_started turn=t2",
                "turn trigger: remote_ios turn=t2",
            ],
        )
        trigger = next(row for row in rows if row.text == "turn trigger: remote_ios turn=t2")
        self.assertEqual(
            (trigger.turn_type, trigger.timestamp_utc, trigger.session_id),
            ("system", "2026-10-18T21:26:41.000Z", CODEX_DESKTOP),
        )
        # A rollout without turn_attribution keeps its rows (CodexTests).
        plain = self.rows_for(CodexParser(), CodexTests.REL)
        self.assertFalse([row for row in plain if row.text.startswith("turn trigger:")])

    VERDICT = (
        "permission decision: allow (auto_review risk_level=low user_authorization=high)"
        " exec %s reviewer %s: The user asked for the file in that folder."
    )

    def test_guardian_review_decision(self):
        rows = self.rows_for(CodexParser(), self.GUARDIAN)
        self.assertEqual(
            [(row.session_id, row.text) for row in rows[:2]],
            [
                (
                    CODEX_GUARDIAN,
                    "session start: Codex Desktop 0.163.0-alpha.10.2 provider=openai cwd=/srv/proj",
                ),
                (CODEX_GUARDIAN, "subagent %s of %s" % (CODEX_GUARDIAN, CODEX_DESKTOP)),
            ],
        )
        self.assertIn("turn trigger: guardian_review turn=tg", [row.text for row in rows])
        decision = rows[-1]
        # The verdict belongs to the parent thread and joins the governed
        # call through the guardian_review span in logs_2.sqlite.
        self.assertEqual(
            (
                decision.turn_type,
                decision.timestamp_utc,
                decision.session_id,
                decision.tool_use_id,
                decision.project_path,
                decision.text,
            ),
            (
                "system",
                "2026-10-18T21:27:27.000Z",
                CODEX_DESKTOP,
                CODEX_REVIEWED_ITEM,
                "/srv/proj",
                self.VERDICT % (CODEX_REVIEWED_ITEM, CODEX_GUARDIAN),
            ),
        )
        call = next(
            row
            for row in self.rows_for(CodexParser(), self.DESKTOP)
            if row.turn_type == "tool_use" and row.tool_use_id == CODEX_REVIEWED_ITEM
        )
        self.assertEqual(call.session_id, decision.session_id)
        # The reviewer's model stays out of the parent thread's models.
        self.assertEqual(decision.model, "")

    def test_guardian_decision_without_log_span(self):
        connection = sqlite3.connect(str(self.home / self.LOGS_REL))
        connection.execute("delete from logs where feedback_log_body like 'guardian_review{%'")
        connection.commit()
        connection.close()
        rows = self.rows_for(CodexParser(), self.GUARDIAN)
        self.assertEqual(
            (rows[-1].tool_use_id, rows[-1].text), ("", self.VERDICT % ("-", CODEX_GUARDIAN))
        )

    def test_guardian_verdict_not_json_is_no_decision(self):
        recs = codex_guardian_records()
        recs[-1]["payload"]["last_agent_message"] = "allow, low risk"
        path = self.home / self.GUARDIAN
        path.write_text("".join(json.dumps(record) + "\n" for record in recs), encoding="utf-8")
        rows = self.rows_for(CodexParser(), self.GUARDIAN)
        self.assertFalse([row for row in rows if row.text.startswith("permission decision:")])

    def test_bad_line_in_desktop_rollout(self):
        row = CodexTests.check_bad_line(self, CodexParser, self.DESKTOP, 2)
        self.assertEqual(row.session_id, CODEX_DESKTOP)

    def test_log_approval_decisions(self):
        rows = self.rows_for(CodexParser(), self.LOGS_REL)
        got = [
            (row.timestamp_utc, row.session_id, row.tool_use_id, row.source_line, row.text)
            for row in rows
            if row.text.startswith("permission decision:")
        ]
        self.assertEqual(
            got,
            [
                # The core's record names the thread and the governed item; the
                # app server's response that repeats it in its own words
                # (`Decline` for `denied`) is left out.
                (
                    "2026-10-18T21:20:05.000Z",
                    CODEX_DESKTOP,
                    CODEX_DECLINED_ITEM,
                    5,
                    "permission decision: denied exec %s turn=t1 (from logs_2.sqlite logs, ts)"
                    % CODEX_DECLINED_ITEM,
                ),
                # A response with no core record beside it: no thread known.
                (
                    "2026-10-18T21:30:00.000Z",
                    "",
                    "",
                    8,
                    "permission decision: Accept request 11 (from logs_2.sqlite logs, ts)",
                ),
            ],
        )
        # The /feedback rows are unchanged (CodexTests.test_feedback_report_rows).
        self.assertEqual(len([row for row in rows if row.text.startswith("session synced:")]), 2)
        decline = next(
            row
            for row in self.rows_for(CodexParser(), self.DESKTOP)
            if row.turn_type == "tool_use" and row.tool_use_id == CODEX_DECLINED_ITEM
        )
        self.assertEqual(decline.session_id, got[0][1])

    def test_unreadable_approval_log_row(self):
        connection = sqlite3.connect(str(self.home / self.LOGS_REL))
        cut = 'Submission thread_id=%s operation="exec_approval" op=ExecApproval { id: "exec-' % (
            CODEX_DESKTOP
        )
        connection.execute(
            "insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id)"
            " values (1792358500, 0, 'DEBUG', 'codex_core::session::handlers', ?, ?)",
            (cut, CODEX_DESKTOP),
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(CodexParser(), self.LOGS_REL)
        decisions = [row for row in rows if row.text.startswith("permission decision:")]
        self.assertEqual(len(decisions), 2)  # the rows around it are still read
        self.assertEqual(
            (rows[-1].turn_type, rows[-1].source_line, rows[-1].text),
            (
                "system",
                9,
                "parser: unreadable approval log row (codex_core::session::handlers): " + cut,
            ),
        )

    def test_patch_approval_decision(self):
        connection = sqlite3.connect(str(self.home / self.LOGS_REL))
        connection.execute(
            "insert into logs (ts, ts_nanos, level, target, feedback_log_body, thread_id)"
            " values (?,?,?,?,?,?)",
            CODEX_PATCH_LOG_ROW,
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(CodexParser(), self.LOGS_REL)
        patch = [row for row in rows if " patch " in row.text]
        self.assertEqual(
            [
                (row.timestamp_utc, row.session_id, row.tool_use_id, row.source_line, row.text)
                for row in patch
            ],
            [
                (
                    "2026-10-18T21:31:40.000Z",
                    CODEX_DESKTOP,
                    CODEX_PATCH_ITEM,
                    9,
                    "permission decision: approved_for_session patch %s (from logs_2.sqlite"
                    " logs, ts)" % CODEX_PATCH_ITEM,
                )
            ],
        )
        # The exec decisions are unchanged.
        self.assertEqual(len([row for row in rows if " exec " in row.text]), 1)

    def test_guardian_review_feedback(self):
        write_codex_guardian_feedback(self.home / self.STATE_REL)
        rows = self.rows_for(CodexParser(), self.STATE_REL)
        got = [row for row in rows if row.text.startswith(("permission decision:", "parser:"))]
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.tool_use_id, row.source_line) for row in got],
            [
                ("2026-10-18T21:27:27.000Z", CODEX_DESKTOP, CODEX_REVIEWED_ITEM, 1),
                ("2026-10-18T21:28:20.000Z", CODEX_DESKTOP, "", 2),
                ("", "", "", 3),
            ],
        )
        # A denied verdict reads as the rollout reader's row for it, so the two pair.
        self.assertEqual(
            got[0].text,
            "permission decision: deny (auto_review risk_level=high user_authorization=low) exec"
            " %s reviewer %s: The command uploads the home directory."
            % (CODEX_REVIEWED_ITEM, CODEX_GUARDIAN),
        )
        self.assertEqual(
            got[1].text,
            "permission decision: timed_out (auto_review feedback model=codex-auto-review) exec -"
            " turn=t3 reviewer %s: - action: rm -rf build" % CODEX_GUARDIAN,
        )
        self.assertTrue(got[2].text.startswith("parser: unparseable record 3 ("), got[2].text)
        self.assertTrue(got[2].text.endswith(': {"reviewed_thread_id": "0d0d'), got[2].text)

    def test_remote_control_enrollment(self):
        write_codex_enrollment(self.home / self.STATE_REL, enabled=0)
        rows = self.rows_for(CodexParser(), self.STATE_REL)
        remote = [row for row in rows if row.text.startswith("remote control:")]
        self.assertEqual(
            [
                (row.turn_type, row.timestamp_utc, row.session_id, row.source_line, row.text)
                for row in remote
            ],
            [
                (
                    "system",
                    "2026-10-18T23:50:18.000Z",
                    "",
                    1,
                    "remote control: enabled=0 server=srv_e_0123abcd environment=env_e_0123abcd"
                    " host=lab-mac.local client=Codex Desktop account=%s" % CODEX_ACCOUNT,
                ),
            ],
        )
        # The thread rows come first and are unchanged (CodexTests.test_state_rows).
        self.assertEqual(rows[-1].text, remote[0].text)
        self.assertEqual(
            [row.turn_type for row in rows[:-1]], ["system", "user", "system", "system", "user"]
        )

    def test_global_state_rows(self):
        parser = CodexParser()
        art = self.artifact(CODEX_GLOBAL_STATE_REL)
        self.assertTrue(parser.wants(art))
        self.assertTrue(parser.secondary(art))
        self.assertFalse(OpenInterpreterParser().wants(art))
        rows = self.rows_for(parser, CODEX_GLOBAL_STATE_REL)
        mtime = to_utc(CODEX_GLOBAL_STATE_MTIME)
        source = "(from %s %%s, file mtime)" % CODEX_GLOBAL_STATE_REL
        scratch = "/home/alice/Documents/Codex/2026-10-18/w"
        standing = (
            "permission mode (standing): approval=on-request sandbox="
            '{"excludeSlashTmp":false,"excludeTmpdirEnvVar":false,"networkAccess":false,'
            '"type":"workspaceWrite","writableRoots":["/srv/proj"]} reviewer=auto_review'
            " profile_id=:workspace " + source % "heartbeat-thread-permissions-by-id"
        )
        setting = "permission setting: %s (from " + CODEX_GLOBAL_STATE_REL + ", file mtime)"
        self.assertEqual(
            [
                (row.turn_type, row.timestamp_utc, row.session_id, row.project_path, row.text)
                for row in rows
            ],
            [
                # The global list's entries repeat the threads' and are left out.
                ("user", "", CODEX_DESKTOP, "", "fetch the example page"),
                ("user", "", CODEX_DELETED, scratch, "what is the capital of the UK"),
                (
                    "system",
                    mtime,
                    CODEX_DELETED,
                    scratch,
                    "session deleted: %s %s"
                    % (CODEX_DELETED, source % "codex-writing-block-deleted-thread-v1"),
                ),
                (
                    "system",
                    mtime,
                    CODEX_DELETED,
                    scratch,
                    "thread description: Find the UK capital last_activity="
                    "2026-10-18T13:40:34.000Z " + source % "thread-descriptions-v1",
                ),
                ("system", mtime, CODEX_DESKTOP, "", standing),
                (
                    "system",
                    mtime,
                    "",
                    "",
                    setting % "agent-mode-by-host-id.local=guardian-approvals",
                ),
                (
                    "system",
                    mtime,
                    "",
                    "",
                    setting
                    % 'permission-selection-by-host-id:local={"agentMode":"guardian-approvals",'
                    '"kind":"agent-mode"}',
                ),
                (
                    "system",
                    mtime,
                    "",
                    "",
                    "remote control: mobile device connected=true environment=env_e_0123abcd "
                    + source % "codex-mobile-has-connected-device",
                ),
                # The projectless output directory is a scratch directory; the
                # row names it in its own project_path.
                (
                    "system",
                    mtime,
                    CODEX_DELETED,
                    scratch,
                    "scratch directory: %s (projectless thread %s) %s"
                    % (scratch, CODEX_DELETED, source % "thread-projectless-output-directories"),
                ),
            ],
        )

    def test_global_state_scratch_path_over_root_hint(self):
        # The scratch row names the output directory even when the thread's
        # root hint, which the other rows take, is another path.
        path = self.home / CODEX_GLOBAL_STATE_REL
        doc = json.loads(path.read_text(encoding="utf-8"))
        doc["thread-workspace-root-hints"][CODEX_DELETED] = "/srv/other"
        path.write_text(json.dumps(doc), encoding="utf-8")
        rows = self.rows_for(CodexParser(), CODEX_GLOBAL_STATE_REL)
        scratch = [row for row in rows if row.text.startswith("scratch directory:")]
        self.assertEqual(
            [row.project_path for row in scratch], ["/home/alice/Documents/Codex/2026-10-18/w"]
        )
        deleted = [row for row in rows if row.text.startswith("session deleted:")]
        self.assertEqual([row.project_path for row in deleted], ["/srv/other"])

    def test_global_state_recovers_deleted_thread(self):
        rows, _, problems, deduplicated = cli.collect_rows(self.col, Options(), ["codex-cli"])
        self.assertEqual(problems, [])
        users = [
            (row.session_id, row.recovered, row.source_file.endswith(".codex-global-state.json"))
            for row in rows
            if row.turn_type == "user" and row.session_id in (CODEX_DESKTOP, CODEX_DELETED)
        ]
        # The rollout's prompt wins over the app's copy; the deleted thread's
        # prompt survives only in the global state and is marked recovered.
        self.assertEqual(sorted(users), [(CODEX_DESKTOP, "", False), (CODEX_DELETED, "yes", True)])
        self.assertGreaterEqual(deduplicated["codex-cli"], 12)
        sessions = {
            session.session_id: session for session in summarise(rows, frozenset({"codex-cli"}))
        }
        self.assertEqual(sessions[CODEX_DELETED].recovered, "yes")

    def test_global_state_unreadable_is_one_row(self):
        path = self.home / CODEX_GLOBAL_STATE_REL
        for body, text in (
            ('{"electron-persisted-atom-state": {"prompt-hist', "parser: unreadable global state:"),
            ("[1]", "parser: global state is not a JSON object"),
        ):
            path.write_text(body, encoding="utf-8")
            rows = self.rows_for(CodexParser(), CODEX_GLOBAL_STATE_REL)
            self.assertEqual([row.turn_type for row in rows], ["system"])
            self.assertTrue(rows[0].text.startswith(text), rows[0].text)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_global_state_is_one_row(self):
        locked = self.home / CODEX_GLOBAL_STATE_REL
        locked.chmod(0)
        try:
            rows = self.rows_for(CodexParser(), CODEX_GLOBAL_STATE_REL)
            later = self.rows_for(CodexParser(), self.DESKTOP)
        finally:
            locked.chmod(0o600)
        self.assertEqual(
            [
                (row.turn_type, row.text, row.source_line, row.timestamp_utc, row.session_id)
                for row in rows
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
        self.assertEqual(rows[0].project_path, "")
        self.assertTrue(any(not row.text.startswith("parser:") for row in later))
        self.assertTrue(
            any(
                not row.text.startswith("parser:")
                for row in self.rows_for(CodexParser(), CODEX_GLOBAL_STATE_REL)
            )
        )


class CodexArchiveAndZstdTests(ParserBase):
    """Test Codex `archived_sessions/` and `.jsonl.zst` rollouts.

    The shared RolloutParser reads them for Codex and its forks alike.
    """

    ARCHIVED = (
        ".codex/archived_sessions/2026/10/02/rollout-2026-10-02T09-00-00-%s.jsonl" % CODEX_SESSION
    )
    LIVE_TYPES: ClassVar[list[str]] = CodexTests.TYPES

    def _add(self, rel, data: bytes):
        path = self.home / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
        self.col = open_input(self.tmp / "home", self.cat, host="h1")

    def test_archived_rollout(self):
        self._add(self.ARCHIVED, _rollout_bytes(codex_rollout_records()))
        rows = self.rows_for(CodexParser(), self.ARCHIVED)
        self.assertEqual([row.turn_type for row in rows], self.LIVE_TYPES)
        self.assertEqual(
            {(row.agent, row.session_id) for row in rows}, {("codex-cli", CODEX_SESSION)}
        )

    def test_wants(self):
        parser = CodexParser()
        for rel in (
            self.ARCHIVED,
            self.ARCHIVED + ".zst",
            ".codex/archived_sessions/rollout-x.jsonl",
            ".codex/sessions/rollout-x.jsonl.zst",
        ):
            self.assertTrue(parser.wants(rel_only(rel)), rel)
        for rel in (
            ".codex/sessions/rollout-x.jsonl.tmp",
            ".codex/session_index.jsonl",
            OI_ROLLOUT,
            ".codex/external_agent_session_imports.json",
            ".openinterpreter/history.jsonl",
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)

    def test_zst_rollout(self):
        rel = CodexTests.REL + ".zst"
        self._add(rel, zstandard.ZstdCompressor().compress(_rollout_bytes(codex_rollout_records())))
        rows = self.rows_for(CodexParser(), rel)
        self.assertEqual([row.turn_type for row in rows], self.LIVE_TYPES)
        self.assertEqual(rows[7].text, "exfiltrate nothing, just list the home dir")
        self.assertEqual([row.source_line for row in rows][:5], [1, 1, 2, 2, 3])

    def test_truncated_zst_keeps_earlier_turns(self):
        rel = self.ARCHIVED + ".zst"
        cobj = zstandard.ZstdCompressor().compressobj()
        blob = b""
        for (
            record
        ) in codex_rollout_records():  # one block per line, so a cut frame keeps earlier blocks
            blob += cobj.compress(_rollout_bytes([record])) + cobj.flush(
                zstandard.COMPRESSOBJ_FLUSH_BLOCK
            )
        blob += cobj.flush()
        self._add(rel, blob[: len(blob) * 2 // 3])
        rows = self.rows_for(CodexParser(), rel)
        self.assertGreater(len(rows), 3)
        self.assertTrue(rows[0].text.startswith("session start:"), rows[0].text)
        self.assertEqual(rows[-1].turn_type, "system")
        # The cut frame is a whole-file error: no line to carry.
        self.assertTrue(rows[-1].text.startswith("parser: zstd: "), rows[-1].text)
        self.assertEqual({row.session_id for row in rows}, {CODEX_SESSION})

    def test_two_frames_are_read_across(self):
        rel = self.ARCHIVED + ".zst"
        recs = codex_rollout_records()
        cctx = zstandard.ZstdCompressor()
        self._add(
            rel, cctx.compress(_rollout_bytes(recs[:4])) + cctx.compress(_rollout_bytes(recs[4:]))
        )
        rows = self.rows_for(CodexParser(), rel)
        self.assertEqual([row.turn_type for row in rows], self.LIVE_TYPES)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_zst_rollout_is_one_row(self):
        rel = self.ARCHIVED + ".zst"
        records = codex_rollout_records()[:2]
        self._add(rel, zstandard.ZstdCompressor().compress(_rollout_bytes(records)))
        locked = self.home / rel
        locked.chmod(0)
        try:
            rows = self.rows_for(CodexParser(), rel)
            others = self.rows_for(CodexParser(), CodexTests.REL)
        finally:
            locked.chmod(0o600)
        self.assertEqual(
            [(row.turn_type, row.text, row.source_line, row.timestamp_utc) for row in rows],
            [("system", "parser: %s could not be read (Permission denied)" % locked.name, 0, "")],
        )
        self.assertEqual(rows[0].session_id, CODEX_SESSION)
        self.assertEqual([row.turn_type for row in others], self.LIVE_TYPES)
        readable = self.rows_for(CodexParser(), rel)
        self.assertTrue(readable, rel)
        self.assertFalse(any(row.text.startswith("parser:") for row in readable), rel)

    def test_zst_read_error_keeps_earlier_lines(self):
        records = codex_rollout_records()[:2]
        frame = zstandard.ZstdCompressor().compress(_rollout_bytes(records))

        class FailingHandle(io.BytesIO):
            """A file whose second read fails as a disk read error does."""

            def read(self, size: int | None = -1):
                if self.tell():
                    raise OSError(5, "Input/output error")
                return super().read(size)

        errors: list = []
        with mock.patch(
            "doubleagent.parsers.codex.open", create=True, return_value=FailingHandle(frame)
        ):
            lines = list(iter_zst_jsonl(self.tmp / "rollout.jsonl.zst", errors))
        self.assertEqual(lines, [(1, records[0]), (2, records[1])])
        self.assertEqual(errors, [(2, "rollout.jsonl.zst could not be read (Input/output error)")])

    def test_corrupt_zst_is_reported_not_fatal(self):
        rel = self.ARCHIVED + ".zst"
        self._add(rel, b"not a zstd frame at all")
        rows = self.rows_for(CodexParser(), rel)
        self.assertEqual([(row.turn_type, row.source_line) for row in rows], [("system", 1)])
        self.assertTrue(rows[0].text.startswith("parser: zstd: "), rows[0].text)
