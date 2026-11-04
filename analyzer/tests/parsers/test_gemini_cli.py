"""Tests of the Gemini CLI parser."""

import json
import os
import unittest
from typing import ClassVar

from test_errors import CAN_DENY, DENY_REASON

from doubleagent.dedup import deduplicate
from doubleagent.inputs import Artifact, Home, open_input
from doubleagent.parsers import Options
from doubleagent.parsers.gemini_cli import (
    CHECKPOINT_PATTERN,
    HISTORY_OBJECT_PATTERN,
    TOOL_CHECKPOINT_PATTERN,
    GeminiCliParser,
    context_kind,
    name_time,
    resolve_project,
)
from doubleagent.timeutil import to_utc
from fixtures import (
    GEMINI_ACTIVITY_REL,
    GEMINI_BUG_MS,
    GEMINI_BUG_REL,
    GEMINI_CANCELLED_ERROR,
    GEMINI_CHECKPOINT_REL,
    GEMINI_CLIPBOARD_MS,
    GEMINI_CLIPBOARD_REL,
    GEMINI_ERROR_REPORT_REL,
    GEMINI_HASH,
    GEMINI_LEGACY,
    GEMINI_LOOP_PROMPT,
    GEMINI_MEMORY_MTIME,
    GEMINI_PNG,
    GEMINI_REL,
    GEMINI_RESUMED,
    GEMINI_SESSION,
    GEMINI_SOURCES_MTIME,
    GEMINI_SUBAGENT,
    GEMINI_TOOL_CHECKPOINT_REL,
    GEMINI_TRUST_MTIME,
    TRUST_JSONC,
    bad_line_text,
    build_gemini_cli_sources,
    gemini_error_report,
    gemini_response,
    gemini_tool_checkpoint,
    write_bad_line,
)
from fixtures.gemini_cli import (
    GEMINI_CONTEXT_TEXT,
    GEMINI_HINTS_TEXT,
    GEMINI_HOOK_TEXT,
    GEMINI_LOADED_CONTEXT_TEXT,
    GEMINI_PLAN_EXIT_TEXT,
    GEMINI_PROMPT_HOOK_TEXT,
    GEMINI_SESSION_CONTEXT,
    GEMINI_SNAPSHOT_REPLY,
    GEMINI_SNAPSHOT_TEXT,
    GEMINI_STEERING_TEXT,
)

from .base import ParserBase, kept_rows, parse_with, rel_only, unreadable_rows

# The texts shell mode and a background completion write (useExecutionLifecycle.ts,
# fastAckHelper.ts).
SHELL_TEXT = (
    "I ran the following shell command:\n```sh\nls\n```\n\n"
    "This produced the following result:\n```\nsrc\n```"
)
BACKGROUND_TEXT = (
    "Background execution update:\n<background_output>\nbuild done\n</background_output>"
)


class GeminiCliTests(ParserBase):
    """Test the Gemini CLI parser."""

    def test_history_object_takes_json_whitespace_only(self):
        self.assertTrue(HISTORY_OBJECT_PATTERN.match(' \r\n{\t"history" : '))
        for space in map(chr, (0x00A0, 0x2028, 0x0085)):
            for text in (space + '{"history": ', '{"history"%s: ' % space):
                self.assertIsNone(HISTORY_OBJECT_PATTERN.match(text), repr(text))

    REL = GEMINI_REL
    LEGACY = ".gemini/tmp/%s/chats/session-2026-09-30T08-00-b2c3d4e5.json" % GEMINI_HASH

    def test_secondary(self):
        # The history file is a secondary source, the transcript is not.
        parser = GeminiCliParser()
        self.assertTrue(parser.secondary(self.artifact(".gemini/tmp/proj/logs.json")))
        self.assertFalse(parser.secondary(self.artifact(self.REL)))

    def test_rows(self):
        rows = self.rows_for(GeminiCliParser(), self.REL)
        readme_id = "read_file__read_file_1759309225000_0"
        denied = "[Operation Cancelled] Reason: User denied execution."
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                (1, "system", "session start: kind=main projectHash=" + GEMINI_HASH),
                (2, "system", "context: session_context: " + GEMINI_CONTEXT_TEXT),
                (3, "system", "context: hook_context: " + GEMINI_HOOK_TEXT),
                (4, "user", "list files in src"),
                (4, "system", "context: hook_context: " + GEMINI_PROMPT_HOOK_TEXT),
                (6, "system", "model: gemini-2.5-pro"),
                (6, "assistant", "Listing now."),
                (6, "tool_use", 'ReadFolder: {"dir_path":"/srv/proj/src"}'),
                (6, "tool_result", "main.ts\nutil.ts"),
                (15, "user", "show util.ts"),
                (17, "assistant", "Here is util.ts."),
                (17, "tool_use", 'ReadFile: {"file_path":"/srv/proj/src/util.ts"}'),
                (17, "tool_result", "export const x = 1"),
                (19, "system", "info: Request cancelled."),
                (21, "system", "model: gemini-2.5-pro -> gemini-2.5-flash"),
                (21, "tool_use", 'Shell: {"command":"curl http://x"}'),
                (21, "tool_result", "[cancelled] " + GEMINI_CANCELLED_ERROR),
                (21, "system", "permission decision: cancelled"),
                (24, "tool_use", 'ReadFile: {"file_path":"/srv/proj/README.md"}'),
                (26, "system", "context: steering_update: " + GEMINI_STEERING_TEXT),
                (26, "tool_result", "# proj"),
                (27, "system", "context: state_snapshot: " + GEMINI_SNAPSHOT_TEXT),
                (28, "assistant", GEMINI_SNAPSHOT_REPLY),
                (29, "system", "context: plan_mode_exit: " + GEMINI_PLAN_EXIT_TEXT),
                (31, "user", "resume work"),
                (33, "system", "model: gemini-2.5-flash"),
                (33, "tool_use", 'ReadFile: {"file_path":"/srv/proj/missing.ts"}'),
                (33, "tool_result", "[error] File not found: /srv/proj/missing.ts"),
                (33, "tool_use", 'Shell: {"command":"make deploy"}'),
                (33, "tool_result", "[cancelled] " + denied),
                (33, "system", "permission decision: cancelled"),
                (8, "system", "summary: List src files"),
                (13, "system", "rewind to msg-g2: 2 message(s) dropped"),
                (14, "system", "rewind to msg-u2: 1 message(s) dropped"),
            ],
        )
        for row in rows:
            self.assertEqual((row.host, row.user, row.agent), ("h1", "alice", "gemini-cli"))
            self.assertEqual((row.project_path, row.git_branch), ("/srv/proj", ""))
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        by_text = {(row.source_line, row.text): row for row in rows}
        model1 = by_text[(6, "model: gemini-2.5-pro")]
        self.assertEqual(
            (model1.model, model1.timestamp_utc, model1.session_id),
            ("gemini-2.5-pro", "2026-10-01T09:00:07.000Z", GEMINI_SESSION),
        )
        model2 = by_text[(21, "model: gemini-2.5-pro -> gemini-2.5-flash")]
        self.assertEqual(
            (model2.model, model2.timestamp_utc), ("gemini-2.5-flash", "2026-10-01T09:00:24.000Z")
        )
        decision = by_text[(21, "permission decision: cancelled")]
        self.assertEqual(
            (decision.tool_name, decision.tool_use_id, decision.timestamp_utc),
            (
                "run_shell_command",
                "run_shell_command__run_shell_command_1759309224000_0",
                "2026-10-01T09:00:24.000Z",
            ),
        )
        denied_id = "run_shell_command__run_shell_command_1759309266000_0"
        denied_rows = [row for row in rows if row.tool_use_id == denied_id]
        self.assertEqual(
            [(row.turn_type, row.tool_name, row.timestamp_utc) for row in denied_rows],
            [
                (kind, "run_shell_command", "2026-10-01T09:01:05.200Z")
                for kind in ("tool_use", "tool_result", "system")
            ],
        )
        context = by_text[(2, "context: session_context: " + GEMINI_CONTEXT_TEXT)]
        self.assertEqual(
            (context.timestamp_utc, context.session_id, context.tool_use_id),
            ("2026-10-01T09:00:00.000Z", GEMINI_SESSION, ""),
        )
        user1 = by_text[(4, "list files in src")]
        self.assertEqual((user1.session_id, user1.model), (GEMINI_SESSION, ""))
        listing = by_text[(6, 'ReadFolder: {"dir_path":"/srv/proj/src"}')]
        self.assertEqual(
            (listing.tool_name, listing.tool_use_id, listing.timestamp_utc),
            (
                "list_directory",
                "list_directory__list_directory_1759309207000_0",
                "2026-10-01T09:00:07.100Z",
            ),
        )
        lsout = by_text[(6, "main.ts\nutil.ts")]
        self.assertEqual(lsout.tool_use_id, "list_directory__list_directory_1759309207000_0")
        # The tool-response messages whose call holds a result or a failed status give no row,
        # so each of those calls has one result row; the one whose call's result a `$patch`
        # cleared gives the result row, joined on the call's id.
        self.assertFalse([row for row in rows if row.source_line in (7, 22)])
        results = [row.tool_use_id for row in rows if row.turn_type == "tool_result"]
        self.assertEqual(sorted(results), sorted(set(results)))
        readme = by_text[(26, "# proj")]
        self.assertEqual(
            (readme.tool_name, readme.tool_use_id, readme.timestamp_utc, readme.model),
            ("read_file", readme_id, "2026-10-01T09:00:26.000Z", ""),
        )
        readme_call = by_text[(24, 'ReadFile: {"file_path":"/srv/proj/README.md"}')]
        self.assertEqual(readme_call.tool_use_id, readme_id)
        self.assertNotIn("delete everything", " ".join(row.text for row in rows))  # rewound
        self.assertEqual(by_text[(28, GEMINI_SNAPSHOT_REPLY)].model, "")
        self.assertEqual(
            by_text[(31, "resume work")].session_id, GEMINI_RESUMED
        )  # $set.sessionId on resume
        self.assertEqual(by_text[(17, "Here is util.ts.")].session_id, GEMINI_SESSION)
        summary = by_text[(8, "summary: List src files")]
        self.assertEqual(summary.timestamp_utc, "2026-10-01T09:00:08.000Z")

    def test_thinking_opt_in(self):
        rows = self.rows_for(GeminiCliParser(), self.REL, include_thinking=True)
        think = [row for row in rows if row.turn_type == "thinking"]
        self.assertEqual(
            [(row.text, row.timestamp_utc) for row in think],
            [("Plan: Use the ls tool.", "2026-10-01T09:00:06.500Z")],
        )

    def test_legacy_json_and_hash_directory(self):
        rows = self.rows_for(GeminiCliParser(), self.LEGACY)
        self.assertEqual([row.turn_type for row in rows], ["system", "user", "system", "assistant"])
        self.assertTrue(all(row.session_id == GEMINI_LEGACY for row in rows))
        self.assertTrue(
            all(row.project_path == "/srv/proj" for row in rows)
        )  # sha256 match in projects.json
        self.assertEqual(rows[2].text, "model: gemini-2.0-flash")
        self.assertEqual((rows[3].text, rows[3].model), ("Hi.", "gemini-2.0-flash"))
        rows = self.rows_for(GeminiCliParser(), self.LEGACY, include_thinking=True)
        self.assertEqual([row.text for row in rows if row.turn_type == "thinking"], ["weighing it"])

    def test_subagent_session(self):
        rel = ".gemini/tmp/proj/chats/%s/%s.jsonl" % (GEMINI_SESSION, GEMINI_SUBAGENT)
        rows = self.rows_for(GeminiCliParser(), rel)
        call_id = "read_file__read_file_1759309230400_0"
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text, row.tool_use_id) for row in rows[1:]],
            [
                (2, "system", "context: loaded_context: " + GEMINI_LOADED_CONTEXT_TEXT.strip(), ""),
                (2, "user", "investigate", ""),
                (3, "system", "model: gemini-2.5-flash", ""),
                (3, "tool_use", '{"file_path":"/srv/proj/package-lock.json"}', call_id),
                (3, "tool_result", "lockfileVersion 3", call_id),
                # The hints before the tool's response are a context row; the response, which
                # the call's `result` holds, gives none.
                (4, "system", "context: user_hints: " + GEMINI_HINTS_TEXT, ""),
            ],
        )
        self.assertIn("kind=subagent", rows[0].text)
        self.assertEqual({row.session_id for row in rows}, {GEMINI_SUBAGENT})

    def test_trusted_folders(self):
        rel = ".gemini/trustedFolders.json"
        rows = self.rows_for(GeminiCliParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.text, row.project_path) for row in rows],
            [
                (
                    "system",
                    "permission setting: trustedFolders./srv/proj=TRUST_FOLDER "
                    "(from .gemini/trustedFolders.json, file mtime)",
                    "/srv/proj",
                ),
                (
                    "system",
                    "permission setting: trustedFolders./srv/other=TRUST_PARENT "
                    "(from .gemini/trustedFolders.json, file mtime)",
                    "/srv/other",
                ),
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.timestamp_utc, row.session_id, row.source_line, row.agent),
                (to_utc(GEMINI_TRUST_MTIME), "", 1, "gemini-cli"),
            )
        (self.home / rel).write_text('{"/srv/proj": ', encoding="utf-8")
        rows = self.rows_for(GeminiCliParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc) for row in rows],
            [
                (
                    "system",
                    "parser: .gemini/trustedFolders.json is not a JSON object",
                    rows[0].timestamp_utc,
                )
            ],
        )
        # The session file still yields every row.
        self.assertEqual(len(self.rows_for(GeminiCliParser(), self.REL)), 34)

    def test_history(self):
        rows = self.rows_for(GeminiCliParser(), ".gemini/tmp/proj/logs.json")
        self.assertEqual([row.turn_type for row in rows], ["user", "user"])
        self.assertEqual(rows[1].text, "delete everything in /srv/proj")  # kept although rewound
        self.assertEqual((rows[0].session_id, rows[0].project_path), (GEMINI_SESSION, "/srv/proj"))
        self.assertEqual(rows[0].timestamp_utc, "2026-10-01T09:00:05.000Z")

    def _rows_with_unreadable(self, rel):
        """Parse every Gemini CLI file of the home with `rel` at mode 000."""
        path = self.home / rel
        parser = GeminiCliParser()
        rows = []
        path.chmod(0)
        try:
            for artifact in self.col.artifacts:
                if artifact.agent == "gemini-cli" and parser.wants(artifact):
                    rows.extend(parser.parse(artifact, Options()))
        finally:
            path.chmod(0o600)
        return rows

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_logs_is_reported(self):
        rel = ".gemini/tmp/proj/logs.json"
        rows = self._rows_with_unreadable(rel)
        problems = [row for row in rows if row.source_file.endswith(rel)]
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text, row.project_path) for row in problems],
            [(0, "system", "parser: logs.json could not be read (Permission denied)", "/srv/proj")],
        )
        # The transcript beside it is still parsed.
        self.assertEqual(len([row for row in rows if row.source_file.endswith(self.REL)]), 34)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_legacy_session_is_reported(self):
        rows = self._rows_with_unreadable(self.LEGACY)
        name = self.LEGACY.split("/")[-1]
        problems = [row for row in rows if row.source_file.endswith(self.LEGACY)]
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text, row.project_path) for row in problems],
            [(0, "system", "parser: %s could not be read (Permission denied)" % name, "/srv/proj")],
        )
        self.assertEqual(len([row for row in rows if row.source_file.endswith(self.REL)]), 34)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_standing_files_are_not_misread_as_wrong_shape(self):
        self.sources()
        for rel in (
            ".gemini/settings.json",
            ".gemini/trustedFolders.json",
            ".gemini/trusted_hooks.json",
        ):
            rows = self._rows_with_unreadable(rel)
            problems = [
                (row.source_line, row.timestamp_utc, row.text)
                for row in rows
                if row.source_file.endswith(rel)
            ]
            text = "parser: %s could not be read (Permission denied)" % rel.split("/")[-1]
            self.assertEqual(problems, [(0, "", text)], rel)
            self.assertEqual(len([row for row in rows if row.source_file.endswith(self.REL)]), 34)

    def test_project_fallbacks(self):
        base = self.home / ".gemini"
        errors: list = []
        self.assertEqual(resolve_project(base, "proj", errors), "/srv/proj")  # .project_root
        (base / "tmp/proj/.project_root").unlink()
        self.assertEqual(resolve_project(base, "proj", errors), "/srv/proj")  # projects.json slug
        self.assertEqual(resolve_project(base, "other", errors, GEMINI_HASH), "/srv/proj")
        unresolved = resolve_project(base, "other", errors, "f" * 64)
        self.assertEqual(unresolved, "f" * 64)
        self.assertEqual(errors, [])

    def test_noise_not_wanted(self):
        parser = GeminiCliParser()
        self.sources()
        for rel in (
            ".gemini/oauth_creds.json",
            ".gemini/projects.json",
            ".gemini/tmp/proj/.project_root",
            ".gemini/antigravity-cli/history.jsonl",
            ".gemini/policy_integrity.json",
            ".gemini/tmp/proj/checkpoints/2026-10-01T09_00_07_100Z-util.ts-write_file.json",
            ".gemini/tmp/proj/bug-memory-1790856000000.heapsnapshot",
        ):
            art = next(artifact for artifact in self.col.artifacts if artifact.rel == rel)
            self.assertFalse(parser.wants(art), rel)
        # trusted_hooks.json and the acknowledged agents live in the home only.
        project = self.tmp / "srv" / "proj"
        home = Home(disk_path=project, original="/srv/proj", user="alice", host="h1", project=True)
        for rel in (".gemini/trusted_hooks.json", ".gemini/acknowledgments/agents.json"):
            art = Artifact(project / rel, "/srv/proj/" + rel, rel, "gemini-cli", home)
            self.assertFalse(parser.wants(art), rel)

    def test_truncated_last_line_is_reported_not_fatal(self):
        # The bad line is its own row, in full, with the session and
        # project; the rows before it and a line written after it survive.
        before = self.rows_for(GeminiCliParser(), self.REL)
        self.assertEqual(len(before), 34)
        write_bad_line(self.home / self.REL)
        after = {
            "id": "msg-u9",
            "timestamp": "2026-10-01T09:30:00.000Z",
            "type": "user",
            "content": [{"text": "after the cut"}],
        }
        with open(self.home / self.REL, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(after) + "\n")
        rows = self.rows_for(GeminiCliParser(), self.REL)
        self.assertEqual(len(rows), 36)
        added = [row for row in rows if row.source_line == 36]
        self.assertEqual([(row.turn_type, row.text) for row in added], [("user", "after the cut")])
        self.assertEqual(
            [
                (row.source_line, row.turn_type, row.text)
                for row in rows[:-1]
                if row.source_line != 36
            ],
            [(row.source_line, row.turn_type, row.text) for row in before],
        )
        self.assertEqual(
            (
                rows[-1].source_line,
                rows[-1].turn_type,
                rows[-1].session_id,
                rows[-1].project_path,
                rows[-1].text,
            ),
            (
                35,
                "system",
                GEMINI_RESUMED,  # the session id in force at the end of the file
                before[-1].project_path,
                bad_line_text(),
            ),
        )
        # A legacy whole-file JSON that does not parse has no line to carry.
        bad = '{"sessionId": "x", "messages": ['
        (self.home / self.LEGACY).write_text(bad, encoding="utf-8")
        try:
            json.loads(bad)
        except ValueError as error:
            reason = str(error)
        else:
            self.fail("the legacy file parsed")
        rows = self.rows_for(GeminiCliParser(), self.LEGACY)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [(1, "system", "parser: unparseable session file: %s" % reason)],
        )

    def test_trusted_folders_jsonc(self):
        # A byte-order mark, comments and trailing commas, which Qwen Code's
        # parseJsoncObject accepts, decode; a comma inside a string is kept. A
        # file still undecodable is the whole-file row.
        rel = ".gemini/trustedFolders.json"
        path = self.home / rel
        path.write_bytes(TRUST_JSONC)
        os.utime(path, (GEMINI_TRUST_MTIME, GEMINI_TRUST_MTIME))
        rows = self.rows_for(GeminiCliParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.text, row.project_path, row.source_line) for row in rows],
            [
                (
                    "system",
                    "permission setting: trustedFolders./srv/a,]=TRUST_FOLDER "
                    "(from .gemini/trustedFolders.json, file mtime)",
                    "/srv/a,]",
                    1,
                ),
                (
                    "system",
                    "permission setting: trustedFolders./srv/b=DO_NOT_TRUST "
                    "(from .gemini/trustedFolders.json, file mtime)",
                    "/srv/b",
                    1,
                ),
            ],
        )
        self.assertEqual({row.timestamp_utc for row in rows}, {to_utc(GEMINI_TRUST_MTIME)})
        path.write_bytes(TRUST_JSONC.replace(b'"DO_NOT_TRUST",', b'"DO_NOT_TRUST",,'))
        self.assertEqual(
            [
                (row.turn_type, row.text, row.source_line)
                for row in self.rows_for(GeminiCliParser(), rel)
            ],
            [("system", "parser: .gemini/trustedFolders.json is not a JSON object", 1)],
        )

    # -- standing permission files and past-use copies --

    def sources(self):
        """Write the permission files and history copies and reopen the input."""
        build_gemini_cli_sources(self.home / ".gemini")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")

    def test_new_secondary_sources(self):
        self.sources()
        parser = GeminiCliParser()
        for rel in (".gemini/tmp/proj/shell_history", GEMINI_CHECKPOINT_REL, GEMINI_BUG_REL):
            self.assertTrue(parser.wants(self.artifact(rel)), rel)
            self.assertTrue(parser.secondary(self.artifact(rel)), rel)
        for rel in (
            ".gemini/settings.json",
            ".gemini/policies/auto-saved.toml",
            ".gemini/trusted_hooks.json",
            ".gemini/acknowledgments/agents.json",
        ):
            self.assertTrue(parser.wants(self.artifact(rel)), rel)
            self.assertFalse(parser.secondary(self.artifact(rel)), rel)

    def test_settings_rows(self):
        self.sources()
        rows = self.rows_for(GeminiCliParser(), ".gemini/settings.json")
        source = " (from .gemini/settings.json, file mtime)"
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: tools.allowed=run_shell_command(git)" + source,
                "permission setting: tools.allowed=run_shell_command(npm test)" + source,
                "permission setting: tools.exclude=web_fetch" + source,
                "permission setting: general.defaultApprovalMode=auto_edit" + source,
                "permission setting: tools.sandbox=docker" + source,
                "permission setting: security.disableYoloMode=true" + source,
                "permission setting: security.folderTrust.enabled=true" + source,
                "session retention: general.sessionRetention.enabled=true" + source,
                "session retention: general.sessionRetention.maxAge=7d" + source,
            ],
        )
        for row in rows:
            self.assertEqual(
                (
                    row.turn_type,
                    row.timestamp_utc,
                    row.session_id,
                    row.project_path,
                    row.source_line,
                ),
                ("system", to_utc(GEMINI_SOURCES_MTIME), "", "", 1),
            )
        self.assertNotIn("secret", " ".join(row.text for row in rows))
        # The empty settings file build_home writes yields nothing; a broken
        # one is one row.
        (self.home / ".gemini/settings.json").write_text("{}", encoding="utf-8")
        self.assertEqual(self.rows_for(GeminiCliParser(), ".gemini/settings.json"), [])
        (self.home / ".gemini/settings.json").write_text('{"tools": ', encoding="utf-8")
        self.assertEqual(
            [row.text for row in self.rows_for(GeminiCliParser(), ".gemini/settings.json")],
            ["parser: .gemini/settings.json is not a JSON object"],
        )

    def test_policy_rows(self):
        self.sources()
        rel = ".gemini/policies/auto-saved.toml"
        rows = self.rows_for(GeminiCliParser(), rel)
        source = " (from .gemini/policies/auto-saved.toml, file mtime)"
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: policy allow run_shell_command priority=950 "
                'commandPrefix=["git","status"]' + source,
                "permission setting: policy allow search priority=950 mcpName=docs "
                'modes=["autoEdit"]' + source,
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.turn_type, row.timestamp_utc, row.session_id, row.project_path),
                ("system", to_utc(GEMINI_SOURCES_MTIME), "", ""),
            )
        # A corrupt file (the CLI then copies it to auto-saved.toml.bak) is one row.
        (self.home / rel).write_text("[[rule]\n", encoding="utf-8")
        rows = self.rows_for(GeminiCliParser(), rel)
        self.assertEqual(len(rows), 1)
        self.assertTrue(
            rows[0].text.startswith("parser: unreadable .gemini/policies/auto-saved.toml: "),
            rows[0].text,
        )

    def test_sandbox_policy_rows(self):
        self.sources()
        rel = ".gemini/policies/sandbox.toml"
        rows = self.rows_for(GeminiCliParser(), rel)
        source = " (from .gemini/policies/sandbox.toml, file mtime)"
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: sandbox mode plan network=false readonly=true "
                "approvedTools=[] allowOverrides=true" + source,
                "permission setting: sandbox mode default network=false readonly=false "
                "approvedTools=[] allowOverrides=true" + source,
                "permission setting: sandbox mode accepting_edits network=false readonly=false "
                'approvedTools=["sed","grep"] allowOverrides=true' + source,
                "permission setting: sandbox command cargo "
                'allowed_paths=["/srv/proj/target","/srv/proj/.cargo"] allow_network=false'
                + source,
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.turn_type, row.timestamp_utc, row.session_id, row.project_path),
                ("system", to_utc(GEMINI_SOURCES_MTIME), "", ""),
            )
        # A project's sandbox.toml is an ordinary policy file: its tables give no row.
        project = self.tmp / "srv" / "proj"
        (project / ".gemini/policies").mkdir(parents=True)
        (project / ".gemini" / "policies" / "sandbox.toml").write_bytes(
            (self.home / rel).read_bytes()
        )
        home = Home(disk_path=project, original="/srv/proj", user="alice", host="h1", project=True)
        art = Artifact(project / rel, "/srv/proj/" + rel, rel, "gemini-cli", home)
        self.assertEqual(list(GeminiCliParser().parse(art, Options())), [])

    def test_worktree_session_scratch_row(self):
        # The main fixture session's project is not a worktree: no scratch row.
        rows = self.rows_for(GeminiCliParser(), self.REL)
        self.assertFalse([row for row in rows if row.text.startswith("scratch directory:")])
        session_id = "d4e5f6a7-0000-4000-8000-000000000004"
        # A POSIX project root, and a Windows one, which the CLI writes with
        # backslashes and lower-cased.
        cases = (
            ("fix-1", "/srv/proj/.gemini/worktrees/fix-1"),
            ("win-fix-1", "c:\\users\\a\\proj\\.gemini\\worktrees\\fix-1"),
        )
        for slug_name, worktree in cases:
            slug = self.home / ".gemini/tmp" / slug_name
            (slug / "chats").mkdir(parents=True)
            (slug / ".project_root").write_text(worktree, encoding="utf-8")
            records = [
                {
                    "sessionId": session_id,
                    "projectHash": "0" * 64,
                    "startTime": "2026-10-02T10:00:00.000Z",
                    "lastUpdated": "2026-10-02T10:00:00.000Z",
                    "kind": "main",
                },
                {
                    "id": "w-u1",
                    "timestamp": "2026-10-02T10:00:05.000Z",
                    "type": "user",
                    "content": "fix it",
                },
            ]
            rel = ".gemini/tmp/%s/chats/session-2026-10-02T10-00-d4e5f6a7.jsonl" % slug_name
            (self.home / rel).write_text(
                "".join(json.dumps(record) + "\n" for record in records), encoding="utf-8"
            )
            self.col = open_input(self.tmp / "home", self.cat, host="h1")
            rows = self.rows_for(GeminiCliParser(), rel)
            self.assertEqual(
                [(row.turn_type, row.text) for row in rows[:3]],
                [
                    ("system", "session start: kind=main projectHash=" + "0" * 64),
                    ("system", "scratch directory: %s (gemini --worktree fix-1)" % worktree),
                    ("user", "fix it"),
                ],
                worktree,
            )
            scratch = rows[1]
            self.assertEqual(
                (
                    scratch.timestamp_utc,
                    scratch.session_id,
                    scratch.project_path,
                    scratch.source_line,
                ),
                ("2026-10-02T10:00:00.000Z", session_id, worktree, 1),
                worktree,
            )

    def test_cancelled_call_without_response_of_an_older_build(self):
        """A build before 0.65.0 recorded a cancelled call with a null or empty `result`."""
        session_id = "d4e5f6a7-0000-4000-8000-000000000005"
        calls = [
            {
                "id": "run_shell_command__run_shell_command_1759395605000_%d" % index,
                "name": "run_shell_command",
                "args": {"command": "make %d" % index},
                "result": result,
                "status": "cancelled",
                "timestamp": "2026-10-02T10:00:05.000Z",
                "displayName": "Shell",
            }
            for index, result in enumerate((None, []))
        ]
        records = [
            {
                "sessionId": session_id,
                "projectHash": GEMINI_HASH,
                "startTime": "2026-10-02T10:00:00.000Z",
                "lastUpdated": "2026-10-02T10:00:00.000Z",
            },
            {
                "id": "o-g1",
                "timestamp": "2026-10-02T10:00:05.000Z",
                "type": "gemini",
                "content": "",
                "model": "gemini-2.5-pro",
                "toolCalls": calls,
            },
        ]
        rel = ".gemini/tmp/proj/chats/session-2026-10-02T10-00-d4e5f6a7.jsonl"
        (self.home / rel).write_text(
            "".join(json.dumps(record) + "\n" for record in records), encoding="utf-8"
        )
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(GeminiCliParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.text, row.tool_use_id) for row in rows if row.tool_use_id],
            [
                (kind, text, call["id"])
                for call in calls
                for kind, text in (
                    ("tool_use", 'Shell: {"command":"%s"}' % call["args"]["command"]),
                    ("tool_result", "[cancelled]"),
                    ("system", "permission decision: cancelled"),
                )
            ],
        )

    def test_project_settings_and_policy(self):
        # A project's .gemini/settings.json and workspace policy file: the
        # rows carry the project directory.
        project = self.tmp / "srv" / "proj"
        (project / ".gemini/policies").mkdir(parents=True)
        (project / ".gemini/settings.json").write_text(
            json.dumps({"tools": {"confirmationRequired": ["write_file"]}}), encoding="utf-8"
        )
        (project / ".gemini/policies/auto-saved.toml").write_text(
            '[[rule]]\ndecision = "allow"\npriority = 950\ntoolName = "write_file"\n',
            encoding="utf-8",
        )
        home = Home(disk_path=project, original="/srv/proj", user="alice", host="h1", project=True)
        parser = GeminiCliParser()
        rel = ".gemini/settings.json"
        art = Artifact(project / rel, "/srv/proj/" + rel, rel, "gemini-cli", home)
        self.assertTrue(parser.wants(art))
        self.assertEqual(
            [(row.text, row.project_path) for row in parser.parse(art, Options())],
            [
                (
                    "permission setting: tools.confirmationRequired=write_file "
                    "(from .gemini/settings.json, file mtime)",
                    "/srv/proj",
                )
            ],
        )
        rel = ".gemini/policies/auto-saved.toml"
        art = Artifact(project / rel, "/srv/proj/" + rel, rel, "gemini-cli", home)
        self.assertEqual(
            [(row.text, row.project_path) for row in parser.parse(art, Options())],
            [
                (
                    "permission setting: policy allow write_file priority=950 "
                    "(from .gemini/policies/auto-saved.toml, file mtime)",
                    "/srv/proj",
                )
            ],
        )

    def test_trusted_hooks_and_acknowledged_agents(self):
        self.sources()
        rows = self.rows_for(GeminiCliParser(), ".gemini/trusted_hooks.json")
        self.assertEqual(
            [(row.text, row.project_path, row.timestamp_utc) for row in rows],
            [
                (
                    "permission setting: trustedHooks=lint:npm run lint "
                    "(from .gemini/trusted_hooks.json, file mtime)",
                    "/srv/proj",
                    to_utc(GEMINI_SOURCES_MTIME),
                )
            ],
        )
        rows = self.rows_for(GeminiCliParser(), ".gemini/acknowledgments/agents.json")
        self.assertEqual(
            [(row.text, row.project_path) for row in rows],
            [
                (
                    "permission setting: acknowledgedAgents.reviewer=0123abcd "
                    "(from .gemini/acknowledgments/agents.json, file mtime)",
                    "/srv/proj",
                )
            ],
        )
        (self.home / ".gemini/trusted_hooks.json").write_text("[]", encoding="utf-8")
        self.assertEqual(
            [row.text for row in self.rows_for(GeminiCliParser(), ".gemini/trusted_hooks.json")],
            ["parser: .gemini/trusted_hooks.json is not a JSON object"],
        )

    def test_shell_history(self):
        self.sources()
        rel = ".gemini/tmp/proj/shell_history"
        rows = self.rows_for(GeminiCliParser(), rel)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [(1, "user", "ls -la"), (2, "user", "git status"), (3, "user", "make test")],
        )
        for row in rows:
            self.assertEqual(
                (row.timestamp_utc, row.session_id, row.project_path), ("", "", "/srv/proj")
            )

    def test_shell_history_bad_line(self):
        # A line that is not UTF-8 is its own row, in full; the commands
        # before and after it are kept.
        self.sources()
        rel = ".gemini/tmp/proj/shell_history"
        (self.home / rel).write_bytes(b"ls -la\nbad \xff line\ngit status")
        rows = self.rows_for(GeminiCliParser(), rel)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                (1, "user", "ls -la"),
                (3, "user", "git status"),
                (2, "system", "parser: unparseable line (not valid UTF-8): bad � line"),
            ],
        )
        self.assertEqual((rows[-1].project_path, rows[-1].timestamp_utc), ("/srv/proj", ""))

    CHECKPOINT_ROWS: ClassVar[list[tuple[int, str, str]]] = [
        (1, "user", "list files in src"),
        (2, "assistant", "Listing now."),
        (2, "tool_use", '{"dir_path":"/srv/proj/src"}'),
        (3, "tool_result", "main.ts\nutil.ts"),
        (4, "user", "delete everything in /srv/proj"),
        (5, "assistant", "No."),
    ]

    def test_chat_checkpoint(self):
        self.sources()
        rows = self.rows_for(GeminiCliParser(), GEMINI_CHECKPOINT_REL)
        head, rest = rows[0], rows[1:]
        self.assertEqual(
            (head.turn_type, head.text, head.timestamp_utc, head.source_line, head.project_path),
            (
                "system",
                "chat checkpoint: tag my tag authType=oauth-personal "
                "(from .gemini/tmp/proj/checkpoint-my%20tag.json, file mtime)",
                to_utc(GEMINI_SOURCES_MTIME),
                0,
                "/srv/proj",
            ),
        )
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rest], self.CHECKPOINT_ROWS
        )
        for row in rest:
            self.assertEqual(
                (row.timestamp_utc, row.session_id, row.project_path), ("", "", "/srv/proj")
            )
        call, result = rest[2], rest[3]
        self.assertEqual(
            (call.tool_name, call.tool_use_id, result.tool_name, result.tool_use_id),
            ("list_directory", "list_directory__list_directory_1759309207000_0") * 2,
        )
        thinking = self.rows_for(GeminiCliParser(), GEMINI_CHECKPOINT_REL, include_thinking=True)
        self.assertEqual(
            [(row.source_line, row.text) for row in thinking if row.turn_type == "thinking"],
            [(5, "weighing it")],
        )
        # Deduplicated against the session file: the prompt, the reply and
        # the tool call and result it also holds are dropped; the rewound
        # prompt and the last reply, which only the checkpoint keeps, stay.
        primary = self.rows_for(GeminiCliParser(), self.REL)
        kept, dropped = deduplicate(primary, [(row, 2.0) for row in rest])
        self.assertEqual(dropped["gemini-cli"], 4)
        self.assertEqual(
            [row.text for row in kept[len(primary) :]], ["delete everything in /srv/proj", "No."]
        )

    def test_chat_checkpoint_cut_and_legacy(self):
        self.sources()
        path = self.home / GEMINI_CHECKPOINT_REL
        text = path.read_text(encoding="utf-8")
        cut = text.index('"delete everything')
        path.write_text(text[:cut], encoding="utf-8")
        rows = self.rows_for(GeminiCliParser(), GEMINI_CHECKPOINT_REL)
        # The three complete items before the cut survive; the cut item is
        # one row carrying the text from its start to the end of the file.
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[1:-1]],
            self.CHECKPOINT_ROWS[:4],
        )
        self.assertEqual(rows[0].text.split(" authType=")[1].split(" ")[0], "-")
        last = rows[-1]
        self.assertEqual((last.source_line, last.turn_type), (4, "system"))
        self.assertTrue(last.text.startswith("parser: unparseable record 4 ("), last.text)
        self.assertTrue(last.text.endswith('"text": '), last.text)
        # The legacy form, a bare array, with a record that is not an object.
        path.write_text(json.dumps([{"role": "user", "parts": [{"text": "hi"}]}, 7]), "utf-8")
        rows = self.rows_for(GeminiCliParser(), GEMINI_CHECKPOINT_REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[1:]],
            [
                (1, "user", "hi"),
                (2, "system", "parser: unparseable record 2 (not a JSON object): 7"),
            ],
        )
        path.write_text('"neither"', encoding="utf-8")
        rows = self.rows_for(GeminiCliParser(), GEMINI_CHECKPOINT_REL)
        self.assertEqual(
            [row.text for row in rows[1:]],
            ["parser: not a chat history: neither a JSON array nor an object with history"],
        )

    def test_bug_report_history(self):
        self.sources()
        rows = self.rows_for(GeminiCliParser(), GEMINI_BUG_REL)
        head = rows[0]
        self.assertEqual(
            (head.turn_type, head.text, head.timestamp_utc, head.source_line),
            (
                "system",
                "bug report history: exported (from %s, time in the file name)" % GEMINI_BUG_REL,
                to_utc(GEMINI_BUG_MS),
                0,
            ),
        )
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[1:]],
            self.CHECKPOINT_ROWS[:3],
        )
        # Cut between records: the items before the cut and one row.
        path = self.home / GEMINI_BUG_REL
        text = path.read_text(encoding="utf-8")
        path.write_text(text[: text.index("},\n") + 2], encoding="utf-8")
        rows = self.rows_for(GeminiCliParser(), GEMINI_BUG_REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[1:]],
            [
                (1, "user", "list files in src"),
                (2, "system", "parser: array cut off after record 1"),
            ],
        )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_project_side_file_is_a_row_at_each_reader(self):
        unlocked = parse_with(self, GeminiCliParser)
        subagent = "c3d4e5f6-0000-4000-8000-000000000003.jsonl"
        # (side file made unreadable, the files reading it, the project their rows get)
        for locked, readers, project in (
            (
                ".gemini/tmp/proj/.project_root",
                [
                    "logs.json",
                    "shell_history",
                    self.REL,
                    subagent,
                    GEMINI_ACTIVITY_REL,
                    GEMINI_TOOL_CHECKPOINT_REL,
                    GEMINI_CLIPBOARD_REL,
                    "MEMORY.md",
                    "SKILL.md",
                    "extraction.patch",
                ],
                "/srv/proj",
            ),
            (".gemini/projects.json", [self.LEGACY], GEMINI_HASH),
        ):
            rows = parse_with(self, GeminiCliParser, locked)
            text = "parser: %s could not be read (Permission denied)" % locked.split("/")[-1]
            reported = unreadable_rows(rows)
            self.assertEqual(
                [
                    (row.source_line, row.timestamp_utc, row.project_path, row.text)
                    for row in reported
                ],
                [(0, "", project, text)] * len(readers),
                locked,
            )
            self.assertEqual(
                sorted(row.source_file.split("/")[-1] for row in reported),
                sorted(reader.split("/")[-1] for reader in readers),
                locked,
            )
        # The project path still resolves through projects.json, so no other row changes.
        rows = parse_with(self, GeminiCliParser, ".gemini/tmp/proj/.project_root")
        self.assertEqual(kept_rows(rows), kept_rows(unlocked))

    # -- activity log, tool checkpoints, memory, clipboard images and error reports --

    PROJECT_ERROR_REPORT_REL = (
        ".gemini/tmp/proj/gemini-client-error-generateContent-api-2026-10-01T09-00-40-500Z.json"
    )

    def test_past_use_stores_wanted(self):
        parser = GeminiCliParser()
        for rel, secondary in (
            (GEMINI_ACTIVITY_REL, False),
            (GEMINI_TOOL_CHECKPOINT_REL, True),
            (GEMINI_ERROR_REPORT_REL, True),
            (".gemini/tmp/proj/memory/MEMORY.md", False),
            (".gemini/tmp/proj/memory/.inbox/private/extraction.patch", False),
            (GEMINI_CLIPBOARD_REL, False),
        ):
            self.assertTrue(parser.wants(self.artifact(rel)), rel)
            self.assertEqual(parser.secondary(self.artifact(rel)), secondary, rel)
        self.assertFalse(parser.wants(self.artifact(".gemini/tmp/proj/images/notes.txt")))
        # A `/chat save` checkpoint and a tool checkpoint never take each other's file.
        self.assertIsNone(TOOL_CHECKPOINT_PATTERN.match(GEMINI_CHECKPOINT_REL))
        self.assertIsNone(CHECKPOINT_PATTERN.match(GEMINI_TOOL_CHECKPOINT_REL))
        for rel in (
            ".gemini/tmp/proj/checkpoints/notes.json",
            ".gemini/tmp/proj/images/clipboard-1.gif",
            ".gemini/tmp/proj/logs/other.jsonl",
            ".gemini/elsewhere/gemini-client-error-x-2026-10-01T09-00-40-500Z.json",
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)

    CODE_ASSIST = "https://cloudcode-pa.googleapis.com/v1internal:"
    GEMINI_API = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro:"
    ACTIVITY_ROWS: ClassVar[list[tuple[int, str, str]]] = [
        (1, "2026-10-01T09:00:05.000Z", "console warn: Slow response"),
        (
            2,
            "2026-10-01T09:00:05.000Z",
            "network request: POST %sstreamGenerateContent (request k3x9q)" % CODE_ASSIST,
        ),
        (
            4,
            "2026-10-01T09:00:06.800Z",
            "network response: POST %sstreamGenerateContent status 200 in 1800 ms "
            "(request k3x9q)" % CODE_ASSIST,
        ),
        (
            5,
            "2026-10-01T09:00:06.900Z",
            "network request: POST %sgenerateContent (request u5v6w)" % CODE_ASSIST,
        ),
        (
            6,
            "2026-10-01T09:00:07.100Z",
            "network response: POST %sgenerateContent status 200 in - ms (request u5v6w)"
            % CODE_ASSIST,
        ),
        (
            7,
            "2026-10-01T09:00:07.150Z",
            "network request: POST https://oauth2.googleapis.com/token (request t4r8z)",
        ),
        (
            8,
            "2026-10-01T09:00:07.190Z",
            "network error: POST https://oauth2.googleapis.com/token: socket hang up "
            "(request t4r8z)",
        ),
        (
            9,
            "2026-10-01T09:00:07.200Z",
            "network request: POST %sstreamGenerateContent (request p7w2m)" % GEMINI_API,
        ),
        (
            10,
            "2026-10-01T09:00:08.100Z",
            "network response: POST %sstreamGenerateContent status 200 in 900 ms "
            "(request p7w2m)" % GEMINI_API,
        ),
        (
            11,
            "2026-10-01T09:00:08.200Z",
            "network request: GET https://play.googleapis.com/log (request g1h2j)",
        ),
    ]

    def test_activity_log_rows(self):
        rows = self.rows_for(GeminiCliParser(), GEMINI_ACTIVITY_REL)
        self.assertEqual(
            [(row.source_line, row.timestamp_utc, row.text) for row in rows], self.ACTIVITY_ROWS
        )
        for row in rows:
            self.assertEqual(
                (row.turn_type, row.session_id, row.project_path),
                ("system", GEMINI_SESSION, "/srv/proj"),
            )
        # No header value, query string, body or conversation turn reaches a row.
        for secret in (
            "[REDACTED]",
            "redacted",
            "refresh_token",
            "alt=sse",
            GEMINI_LOOP_PROMPT,
            "list files in src",
            "Found main.ts",
        ):
            self.assertFalse([row for row in rows if secret in row.text], secret)
        # The chunk record (line 3) gives no row.
        self.assertNotIn(3, [row.source_line for row in rows])

    def _write_activity(self, records):
        path = self.home / GEMINI_ACTIVITY_REL
        path.write_text("".join(json.dumps(record) + "\n" for record in records), "utf-8")

    def test_activity_log_bad_lines(self):
        path = self.home / GEMINI_ACTIVITY_REL
        before = self.rows_for(GeminiCliParser(), GEMINI_ACTIVITY_REL)
        with open(path, "a", encoding="utf-8") as handle:
            handle.write('{"type": "trace", "payload": {}}\n[1]\n')
        write_bad_line(path)
        console = {"type": "console", "payload": {"type": "log"}, "sessionId": GEMINI_SESSION}
        with open(path, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(console) + "\n")
        rows = self.rows_for(GeminiCliParser(), GEMINI_ACTIVITY_REL)
        self.assertEqual(rows[: len(before)], before)
        self.assertEqual(
            [(row.source_line, row.text) for row in rows[len(before) :]],
            [
                (15, "console log:"),
                (
                    12,
                    'parser: unparseable line (not an activity log record): {"payload":{},'
                    '"type":"trace"}',
                ),
                (13, "parser: unparseable line (not a JSON object): [1]"),
                (14, bad_line_text()),
            ],
        )
        for row in rows[-3:]:
            self.assertEqual(
                (row.session_id, row.project_path, row.timestamp_utc),
                (GEMINI_SESSION, "/srv/proj", ""),
            )

    def test_activity_log_response_without_values(self):
        self._write_activity([gemini_response("z", 1790845205000, {"body": "not json"})])
        rows = self.rows_for(GeminiCliParser(), GEMINI_ACTIVITY_REL)
        self.assertEqual(
            [row.text for row in rows], ["network response: - - status - in - ms (request z)"]
        )

    def _deduplicated(self, rel, primary_rel=None):
        """Return the rows of `rel` kept after deduplication, and the count dropped."""
        primary = self.rows_for(GeminiCliParser(), primary_rel) if primary_rel else []
        secondary = self.rows_for(GeminiCliParser(), rel)
        kept, dropped = deduplicate(primary, [(row, 2.0) for row in secondary])
        return kept[len(primary) :], dropped.get("gemini-cli", 0)

    def test_tool_checkpoint_of_another_session_pairs_with_nothing(self):
        path = self.home / GEMINI_TOOL_CHECKPOINT_REL
        document = json.loads(path.read_text("utf-8"))
        path.write_text(json.dumps(dict(document, messageId="other########1")), "utf-8")
        kept, dropped = self._deduplicated(GEMINI_TOOL_CHECKPOINT_REL, self.REL)
        self.assertEqual(dropped, 0)
        self.assertEqual({row.session_id for row in kept}, {"other"})

    def test_error_report_of_another_project_is_not_dropped(self):
        # A copy under another `tmp/<slug>` takes that project and pairs with no row of
        # /srv/proj's session; the copy at the base, with no project, does.
        moved = self.home / ".gemini/tmp/other"
        moved.mkdir(parents=True)
        (moved / ".project_root").write_text("/srv/other", encoding="utf-8")
        rel = ".gemini/tmp/other/" + GEMINI_ERROR_REPORT_REL.split("/")[-1]
        (self.home / rel).write_text(json.dumps(gemini_error_report()), "utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        kept, dropped = self._deduplicated(rel, self.REL)
        self.assertEqual((dropped, {row.project_path for row in kept}), (0, {"/srv/other"}))
        _, dropped = self._deduplicated(GEMINI_ERROR_REPORT_REL, self.REL)
        self.assertEqual(dropped, 2)

    def test_equal_copies_across_files_are_kept_once(self):
        # With the session file gone, the tool checkpoint and the error report both hold the
        # environment context and the first prompt with no time; each is kept once, each file's
        # own turns too.
        secondary = self.rows_for(GeminiCliParser(), GEMINI_TOOL_CHECKPOINT_REL) + self.rows_for(
            GeminiCliParser(), GEMINI_ERROR_REPORT_REL
        )
        kept, dropped = deduplicate([], [(row, 2.0) for row in secondary])
        self.assertEqual(dropped["gemini-cli"], 2)
        context = "context: session_context: " + GEMINI_CONTEXT_TEXT
        self.assertEqual([row.text for row in kept].count(context), 1)
        texts = [row.text for row in kept if row.turn_type in ("user", "assistant")]
        for text in ("list files in src", "Editing util.ts.", "why did the request fail?"):
            self.assertEqual(texts.count(text), 1, text)

    def test_relocated_project_still_pairs_on_the_session(self):
        # The same tool checkpoint under a slug whose project moved still joins on its session.
        moved = self.home / ".gemini/tmp/moved"
        (moved / "checkpoints").mkdir(parents=True)
        (moved / ".project_root").write_text("/srv/moved", encoding="utf-8")
        name = GEMINI_TOOL_CHECKPOINT_REL.split("/")[-1]
        source = self.home / GEMINI_TOOL_CHECKPOINT_REL
        (moved / "checkpoints" / name).write_bytes(source.read_bytes())
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rel = ".gemini/tmp/moved/checkpoints/" + name
        kept, dropped = self._deduplicated(rel, self.REL)
        self.assertEqual(dropped, 5)
        self.assertEqual({row.project_path for row in kept}, {"/srv/moved"})

    def test_context_kind_of_each_opening(self):
        directory = "- **Workspace Directories:**\n  - /srv/proj\n- **Directory Structure:**\n"
        ide_full = (
            "Here is the user's editor context as a JSON object. This is for your information "
            'only.\n```json\n{"activeFile": {"path": "/srv/proj/src/util.ts"}}\n```'
        )
        ide_delta = (
            "Here is a summary of changes in the user's editor context, in JSON format. This is "
            'for your information only.\n```json\n{"changes": {}}\n```'
        )
        for text, kind in (
            (GEMINI_CONTEXT_TEXT, "session_context"),
            ("  \n" + GEMINI_HOOK_TEXT, "hook_context"),
            (
                "<state_snapshot>\n<overall_goal>x</overall_goal>\n</state_snapshot>",
                "state_snapshot",
            ),
            (GEMINI_SNAPSHOT_TEXT, "state_snapshot"),
            (GEMINI_PLAN_EXIT_TEXT, "plan_mode_exit"),
            (directory, "directory_context"),
            (ide_full, "ide_context"),
            (ide_delta, "ide_context"),
            # The person's texts, a tag or a phrase typed inside one among them.
            ("<scratchpad>notes, no snapshot</scratchpad>", ""),
            ("please explain <session_context> to me", ""),
            ("Plan approved. Switching to Default mode (edits will require confirmation).", ""),
            ("user has manually exited plan mode. switching to default", ""),
            ("Workspace Directories: /srv/proj", ""),
            ("Here is the user's editor context", ""),
            (SHELL_TEXT, "shell_command"),
            (GEMINI_STEERING_TEXT, "steering_update"),
            (BACKGROUND_TEXT, "background_update"),
            (GEMINI_HINTS_TEXT, "user_hints"),
            (GEMINI_LOADED_CONTEXT_TEXT, "loaded_context"),
            ("loaded_context: the notes I wrote", ""),
            ("user hints: lowercase is typed", ""),
            ("Hints: none", ""),
            ("Please continue.", ""),
            ("/help", ""),
            ("? what changed", ""),
            ("", ""),
        ):
            self.assertEqual(context_kind(text), kind, text)

    def _append_session_messages(self, rel, contents):
        """Append one `user` message per content to the session file `rel`."""
        with open(self.home / rel, "a", encoding="utf-8") as handle:
            for i, content in enumerate(contents, 1):
                message = {
                    "id": "x-u%d" % i,
                    "timestamp": "2026-10-01T09:00:3%d.000Z" % i,
                    "type": "user",
                    "content": content,
                }
                handle.write(json.dumps(message) + "\n")

    def test_user_message_parts_become_runs(self):
        rel = ".gemini/tmp/proj/chats/%s/%s.jsonl" % (GEMINI_SESSION, GEMINI_SUBAGENT)
        self._append_session_messages(
            rel,
            [
                # A non-interactive start puts a hook's context before the prompt in one text.
                [{"text": "<hook_context>Ticket 42.</hook_context>\n\nfix the build"}],
                [{"text": "<hook_context>cut off"}],
                [
                    {"text": "look at this"},
                    {"inlineData": {"mimeType": "image/png", "data": "iVBORw0K"}},
                    {"fileData": {"fileUri": "file:///srv/proj/a.pdf", "mimeType": "x"}},
                    7,
                    {"executableCode": {"code": "print(1)"}},
                    {"text": "hidden", "thought": True},
                    {"text": "and this"},
                    {"text": GEMINI_HOOK_TEXT},
                    {"text": GEMINI_PROMPT_HOOK_TEXT},
                    {"text": ""},
                ],
                [],
                # Shell mode's record of a command, and a background completion sent before a
                # tool's response, whose call this file does not hold.
                [{"text": SHELL_TEXT}],
                [
                    {"text": BACKGROUND_TEXT},
                    {"functionResponse": {"id": "c9", "name": "grep", "response": {"output": "x"}}},
                ],
            ],
        )
        rows = self.rows_for(GeminiCliParser(), rel)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[7:]],
            [
                (5, "system", "context: hook_context: <hook_context>Ticket 42.</hook_context>"),
                (5, "user", "fix the build"),
                (6, "system", "context: hook_context: <hook_context>cut off"),
                (
                    7,
                    "user",
                    "look at this\n[inlineData image/png]\n[fileData file:///srv/proj/a.pdf]\n"
                    "and this",
                ),
                (
                    7,
                    "system",
                    "context: hook_context: %s\n%s" % (GEMINI_HOOK_TEXT, GEMINI_PROMPT_HOOK_TEXT),
                ),
                (8, "user", ""),
                (9, "system", "context: shell_command: " + SHELL_TEXT),
                (10, "system", "context: background_update: " + BACKGROUND_TEXT),
                (10, "tool_result", "x"),
            ],
        )
        self.assertEqual((rows[-1].tool_name, rows[-1].tool_use_id), ("grep", "c9"))
        self.assertEqual({row.session_id for row in rows}, {GEMINI_SUBAGENT})

    def test_many_hook_contexts_in_one_text_give_one_run(self):
        """Hook blocks far past the recursion limit split off in one pass, then the prompt."""
        rel = ".gemini/tmp/proj/chats/%s/%s.jsonl" % (GEMINI_SESSION, GEMINI_SUBAGENT)
        blocks = ["<hook_context>h%d</hook_context>" % i for i in range(1500)]
        # A closing tag in a text that does not open with a hook's context splits nothing.
        typed = "why does </hook_context> appear? <hook_context>x</hook_context> then"
        self._append_session_messages(
            rel, [[{"text": "".join(blocks) + "the prompt"}], [{"text": typed}]]
        )
        rows = self.rows_for(GeminiCliParser(), rel)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[7:]],
            [
                (5, "system", "context: hook_context: " + "\n".join(blocks)),
                (5, "user", "the prompt"),
                (6, "user", typed),
            ],
        )

    def test_tool_response_media_joins_its_result(self):
        """Media after a `functionResponse` is that tool's; media before the first is the user's.

        A response whose call already holds a `result` gives no row, its media included.
        """
        rel = ".gemini/tmp/proj/chats/%s/%s.jsonl" % (GEMINI_SESSION, GEMINI_SUBAGENT)

        def response(call_id, output):
            return {"functionResponse": {"id": call_id, "name": "read", "response": output}}

        png = {"inlineData": {"mimeType": "image/png", "data": "iVBORw0K"}}
        gif = {"inlineData": {"mimeType": "image/gif", "data": "R0lGOD"}}
        pdf = {"fileData": {"fileUri": "file:///srv/proj/b.pdf", "mimeType": "application/pdf"}}
        self._append_session_messages(
            rel,
            [
                [response("c20", {"output": "one"}), png],
                [gif, response("c21", {"output": "two"}), pdf],
                [response("c22", {"output": "three"}), png, response("c23", {}), gif, pdf],
                [response("c24", {"output": "four"}), png],
            ],
        )
        answered = {
            "id": "x-g1",
            "timestamp": "2026-10-01T09:00:40.000Z",
            "type": "gemini",
            "content": "",
            "toolCalls": [{"id": "c24", "name": "read", "args": {}, "result": "four"}],
        }
        with open(self.home / rel, "a", encoding="utf-8") as handle:
            handle.write(json.dumps(answered) + "\n")
        rows = self.rows_for(GeminiCliParser(), rel)
        self.assertEqual(
            [
                (row.source_line, row.turn_type, row.tool_use_id, row.text)
                for row in rows
                if row.source_line in (5, 6, 7, 8)
            ],
            [
                (5, "tool_result", "c20", "one\n[inlineData image/png]"),
                (6, "user", "", "[inlineData image/gif]"),
                (6, "tool_result", "c21", "two\n[fileData file:///srv/proj/b.pdf]"),
                (7, "tool_result", "c22", "three\n[inlineData image/png]"),
                (
                    7,
                    "tool_result",
                    "c23",
                    "{}\n[inlineData image/gif]\n[fileData file:///srv/proj/b.pdf]",
                ),
            ],
        )
        self.assertEqual(
            [(row.turn_type, row.tool_use_id) for row in rows if row.source_line == 9],
            [("tool_use", "c24"), ("tool_result", "c24")],
        )

    def test_history_tool_response_media_joins_its_result(self):
        """A history item's media after a `functionResponse` joins that tool's result row."""
        self.sources()
        items = [
            {"role": "model", "parts": [{"functionCall": {"id": "c30", "name": "read"}}]},
            {
                "role": "user",
                "parts": [
                    {"inlineData": {"mimeType": "image/gif", "data": "R0lGOD"}},
                    {"functionResponse": {"id": "c30", "name": "read", "response": {}}},
                    {"inlineData": {"mimeType": "image/png", "data": "iVBORw0K"}},
                ],
            },
        ]
        (self.home / GEMINI_BUG_REL).write_text(json.dumps(items), "utf-8")
        rows = self.rows_for(GeminiCliParser(), GEMINI_BUG_REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.tool_use_id, row.text) for row in rows[1:]],
            [
                (1, "tool_use", "c30", ""),
                (2, "user", "", "[inlineData image/gif]"),
                (2, "tool_result", "c30", "{}\n[inlineData image/png]"),
            ],
        )

    def test_context_item_is_a_system_row_in_each_history_copy(self):
        """A context item is a `context:` row in the history copies and the session file.

        The environment context in the first and in a later position, and a hook's context, each
        give one; a model item opening with a tag stays an assistant row. `logs.json` holds typed
        prompts only and is not covered.
        """
        self.sources()
        items = [
            GEMINI_SESSION_CONTEXT,
            {"role": "user", "parts": [{"text": "list files in src"}]},
            {"role": "user", "parts": [{"text": GEMINI_HOOK_TEXT}]},
            {"role": "model", "parts": [{"text": "<session_context> is set."}]},
            GEMINI_SESSION_CONTEXT,
            {"role": "user", "parts": ["a string part", GEMINI_HOOK_TEXT]},
        ]
        expected = [
            (1, "system", "context: session_context: " + GEMINI_CONTEXT_TEXT),
            (2, "user", "list files in src"),
            (3, "system", "context: hook_context: " + GEMINI_HOOK_TEXT),
            (4, "assistant", "<session_context> is set."),
            (5, "system", "context: session_context: " + GEMINI_CONTEXT_TEXT),
            (6, "user", "a string part"),
            (6, "system", "context: hook_context: " + GEMINI_HOOK_TEXT),
        ]
        checkpoint = dict(gemini_tool_checkpoint(), clientHistory=items)
        report = dict(gemini_error_report(), context=items)
        for rel, document in (
            (GEMINI_CHECKPOINT_REL, {"history": items, "authType": "oauth-personal"}),
            (GEMINI_BUG_REL, items),
            (GEMINI_TOOL_CHECKPOINT_REL, checkpoint),
            (GEMINI_ERROR_REPORT_REL, report),
        ):
            (self.home / rel).write_text(json.dumps(document), "utf-8")
            rows = self.rows_for(GeminiCliParser(), rel)
            self.assertEqual(
                [(row.source_line, row.turn_type, row.text) for row in rows[1:]], expected, rel
            )
            self.assertEqual({row.timestamp_utc for row in rows[1:]}, {""}, rel)
        # The session file's own first message gives the same row, at its time.
        rows = self.rows_for(GeminiCliParser(), self.REL)
        self.assertEqual(
            [
                (row.source_line, row.text)
                for row in rows
                if row.text.startswith("context: session_context:")
            ],
            [(2, "context: session_context: " + GEMINI_CONTEXT_TEXT)],
        )

    def test_context_row_the_session_file_lacks_is_kept_unrecovered(self):
        # The copy the session file holds is dropped. A compression patches the session file's
        # context message with the regenerated text, so a checkpoint written before it keeps an
        # earlier text the file lacks: that row stays, a system row never marked recovered.
        kept, _ = self._deduplicated(GEMINI_TOOL_CHECKPOINT_REL, self.REL)
        self.assertFalse([row for row in kept if row.text.startswith("context:")])
        earlier = GEMINI_CONTEXT_TEXT.replace("linux", "darwin")
        document = gemini_tool_checkpoint()
        document["clientHistory"][0] = {"role": "user", "parts": [{"text": earlier}]}
        (self.home / GEMINI_TOOL_CHECKPOINT_REL).write_text(json.dumps(document), "utf-8")
        kept, dropped = self._deduplicated(GEMINI_TOOL_CHECKPOINT_REL, self.REL)
        self.assertEqual(dropped, 4)
        context = [row for row in kept if row.text.startswith("context:")]
        self.assertEqual(
            [(row.turn_type, row.text, row.recovered) for row in context],
            [("system", "context: session_context: " + earlier, "")],
        )

    def test_error_report_context_item_that_is_no_object_is_one_bad_record(self):
        report = gemini_error_report()
        report["context"].insert(2, "stray")
        (self.home / GEMINI_ERROR_REPORT_REL).write_text(json.dumps(report), "utf-8")
        rows = self.rows_for(GeminiCliParser(), GEMINI_ERROR_REPORT_REL)
        bad = [row for row in rows if row.text.startswith("parser: ")]
        self.assertEqual(
            [(row.source_line, row.text) for row in bad],
            [(3, 'parser: unparseable record 3 (not a JSON object): "stray"')],
        )
        self.assertIn("why did the request fail?", [row.text for row in rows])

    def test_name_time_of_a_malformed_name_is_empty(self):
        self.assertEqual(name_time("not-a-time"), "")
        self.assertEqual(name_time(""), "")

    def test_tool_checkpoint_rows(self):
        rows = self.rows_for(GeminiCliParser(), GEMINI_TOOL_CHECKPOINT_REL)
        head = rows[0]
        self.assertEqual(
            (head.source_line, head.timestamp_utc, head.turn_type, head.text),
            (
                0,
                "2026-10-01T09:00:21.500Z",
                "system",
                "tool checkpoint: replace /srv/proj/src/util.ts commit "
                "4f1c2a9e0b7d6c5a4b3e2f1d0c9b8a7f6e5d4c3b prompt %s########1 "
                "(from %s, time in the file name)" % (GEMINI_SESSION, GEMINI_TOOL_CHECKPOINT_REL),
            ),
        )
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows[1:]],
            [
                (1, "system", "context: session_context: " + GEMINI_CONTEXT_TEXT),
                (2, "user", "list files in src"),
                (3, "assistant", "Listing now."),
                (3, "tool_use", '{"dir_path":"/srv/proj/src"}'),
                (4, "tool_result", "main.ts\nutil.ts"),
                (5, "assistant", "Editing util.ts."),
            ],
        )
        for row in rows:
            self.assertEqual((row.session_id, row.project_path), (GEMINI_SESSION, "/srv/proj"))
        self.assertEqual({row.timestamp_utc for row in rows[1:]}, {""})
        # Deduplicated against the session file: only the reply it lacks stays.
        kept, dropped = self._deduplicated(GEMINI_TOOL_CHECKPOINT_REL, self.REL)
        self.assertEqual(dropped, 5)
        self.assertEqual(
            [row.text for row in kept if row.turn_type != "system"], ["Editing util.ts."]
        )

    def test_tool_checkpoint_without_values_and_bad_files(self):
        path = self.home / GEMINI_TOOL_CHECKPOINT_REL
        path.write_text(
            json.dumps({"toolCall": {"name": "write_file", "args": {}}, "clientHistory": [3]}),
            "utf-8",
        )
        rows = self.rows_for(GeminiCliParser(), GEMINI_TOOL_CHECKPOINT_REL)
        self.assertEqual(
            [(row.source_line, row.session_id, row.text) for row in rows],
            [
                (
                    0,
                    "",
                    "tool checkpoint: write_file - commit - prompt - "
                    "(from %s, time in the file name)" % GEMINI_TOOL_CHECKPOINT_REL,
                ),
                (1, "", "parser: unparseable record 1 (not a JSON object): 3"),
            ],
        )
        path.write_text("[]", "utf-8")
        self.assertEqual(
            [row.text for row in self.rows_for(GeminiCliParser(), GEMINI_TOOL_CHECKPOINT_REL)],
            ["parser: not a tool checkpoint: not a JSON object"],
        )
        cut = '{"history": ['
        path.write_text(cut, "utf-8")
        reason = _json_reason(cut)
        rows = self.rows_for(GeminiCliParser(), GEMINI_TOOL_CHECKPOINT_REL)
        self.assertEqual(
            [(row.source_line, row.text, row.project_path) for row in rows],
            [(0, "parser: unparseable tool checkpoint (%s)" % reason, "/srv/proj")],
        )

    def test_memory_rows(self):
        rel = ".gemini/tmp/proj/memory/skills/lint/SKILL.md"
        rows = self.rows_for(GeminiCliParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.timestamp_utc, row.session_id, row.project_path) for row in rows],
            [("system", to_utc(GEMINI_MEMORY_MTIME), "", "/srv/proj")],
        )
        self.assertEqual(
            rows[0].text,
            "memory file: skills/lint/SKILL.md (from %s, file mtime)\n"
            "Run the linter before committing." % rel,
        )
        (self.home / rel).write_bytes(b"bad \xff text")
        self.assertEqual(
            [row.text for row in self.rows_for(GeminiCliParser(), rel)],
            ["parser: skills/lint/SKILL.md is not UTF-8 text"],
        )

    def test_clipboard_image_row(self):
        rows = self.rows_for(GeminiCliParser(), GEMINI_CLIPBOARD_REL)
        self.assertEqual(
            [(row.turn_type, row.timestamp_utc, row.project_path, row.text) for row in rows],
            [
                (
                    "system",
                    to_utc(GEMINI_CLIPBOARD_MS),
                    "/srv/proj",
                    "clipboard image: clipboard-%d.png %d bytes "
                    "(from %s, time in the file name)"
                    % (GEMINI_CLIPBOARD_MS, len(GEMINI_PNG), GEMINI_CLIPBOARD_REL),
                ),
            ],
        )

    def test_error_report_rows(self):
        rows = self.rows_for(GeminiCliParser(), GEMINI_ERROR_REPORT_REL)
        head = rows[0]
        self.assertEqual(
            (head.source_line, head.timestamp_utc, head.project_path, head.text),
            (
                0,
                "2026-10-01T09:00:30.000Z",
                "",
                "error report: Turn.run-sendMessageStream: fetch failed "
                "(from %s, time in the file name)\nTypeError: fetch failed\n    at fetch"
                % GEMINI_ERROR_REPORT_REL,
            ),
        )
        self.assertEqual(
            [
                (row.source_line, row.timestamp_utc, row.session_id, row.turn_type, row.text)
                for row in rows[1:]
            ],
            [
                (1, "", "", "system", "context: session_context: " + GEMINI_CONTEXT_TEXT),
                (2, "", "", "user", "list files in src"),
                (3, "", "", "user", "why did the request fail?"),
            ],
        )
        # The context and the prompt the session file holds are dropped; the failed prompt,
        # which only the report keeps, stays.
        kept, dropped = self._deduplicated(GEMINI_ERROR_REPORT_REL, self.REL)
        self.assertEqual(dropped, 2)
        self.assertEqual([row.text for row in kept[1:]], ["why did the request fail?"])
        kept, dropped = self._deduplicated(GEMINI_ERROR_REPORT_REL)
        self.assertEqual((dropped, len(kept)), (0, 4))

    def test_error_report_request_contents_and_project(self):
        # A `generateContent-api` report holds its history under `requestContents`; one copied
        # into `tmp/<slug>` takes that project. A report without a stack has one line.
        rel = self.PROJECT_ERROR_REPORT_REL
        report = {
            "error": {"message": "quota"},
            "context": {"requestContents": [{"role": "user", "parts": [{"text": "hi"}]}]},
        }
        (self.home / rel).write_text(json.dumps(report), "utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(GeminiCliParser(), rel)
        self.assertEqual(
            [(row.timestamp_utc, row.project_path, row.turn_type, row.text) for row in rows],
            [
                (
                    "2026-10-01T09:00:40.500Z",
                    "/srv/proj",
                    "system",
                    "error report: generateContent-api: quota (from %s, time in the file name)"
                    % rel,
                ),
                ("", "/srv/proj", "user", "hi"),
            ],
        )
        for text, expected in (
            ('{"context": []}', "parser: not an error report: no error object"),
            ("{", "parser: unparseable error report (%s)" % _json_reason("{")),
        ):
            (self.home / rel).write_text(text, "utf-8")
            self.assertEqual(
                [row.text for row in self.rows_for(GeminiCliParser(), rel)], [expected]
            )

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_past_use_stores_are_reported(self):
        for rel in (
            GEMINI_ACTIVITY_REL,
            GEMINI_TOOL_CHECKPOINT_REL,
            GEMINI_ERROR_REPORT_REL,
            ".gemini/tmp/proj/memory/MEMORY.md",
        ):
            rows = self._rows_with_unreadable(rel)
            problems = [
                (row.source_line, row.timestamp_utc, row.text)
                for row in rows
                if row.source_file.endswith(rel)
            ]
            text = "parser: %s could not be read (Permission denied)" % rel.split("/")[-1]
            self.assertEqual(problems, [(0, "", text)], rel)
            self.assertEqual(len([row for row in rows if row.source_file.endswith(self.REL)]), 34)


def _json_reason(text: str) -> str:
    """Return the reason the running Python's json module gives for `text`."""
    try:
        json.loads(text)
    except ValueError as error:
        return str(error)
    raise AssertionError("text parses: %r" % text)
