"""Tests of the Gemini CLI parser."""

import json
import os
import stat
import sys
import unittest
from typing import ClassVar

from test_errors import CAN_DENY, DENY_REASON

from doubleagent.cli import parsers_for
from doubleagent.dedup import deduplicate
from doubleagent.inputs import Artifact, Home, open_input
from doubleagent.parsers import Options, by_agent
from doubleagent.parsers.gemini_cli import HISTORY_OBJECT_PATTERN, GeminiCliParser, resolve_project
from doubleagent.timeutil import to_utc
from fixtures import (
    GEMINI_BUG_MS,
    GEMINI_BUG_REL,
    GEMINI_CHECKPOINT_REL,
    GEMINI_HASH,
    GEMINI_LEGACY,
    GEMINI_REL,
    GEMINI_RESUMED,
    GEMINI_SESSION,
    GEMINI_SOURCES_MTIME,
    GEMINI_SUBAGENT,
    GEMINI_TRUST_MTIME,
    TRUST_JSONC,
    bad_line_text,
    build_gemini_cli_sources,
    write_bad_line,
)

from .base import ParserBase

OPTIONS = Options(include_thinking=True)


def _parse_with(test, parser_class, locked: str = "") -> list:
    """Parse every file a new `parser_class` is offered, with `locked` at mode 000 if given."""
    parser = parser_class()
    groups = by_agent()
    offered = [
        artifact
        for artifact in test.col.artifacts
        if any(type(candidate) is parser_class for candidate in parsers_for(artifact, groups))
        and parser.wants(artifact)
    ]
    path = test.home / locked if locked else None
    mode = stat.S_IMODE(path.stat().st_mode) if path else 0
    if path:
        path.chmod(0)
    try:
        return [row for artifact in offered for row in parser.parse(artifact, OPTIONS)]
    finally:
        if path:
            path.chmod(mode)


def _unreadable_rows(rows: list) -> list:
    """Return the rows saying a file could not be read."""
    return [row for row in rows if "could not be read" in row.text]


def _kept(rows: list, without: str = "") -> list:
    """Return each row's identifying fields, less the unreadable rows and those of `without`."""
    return [
        (row.source_file, row.source_line, row.turn_type, row.text, row.project_path)
        for row in rows
        if "could not be read" not in row.text
        and not (without and row.source_file.endswith(without))
    ]


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
        types = [row.turn_type for row in rows]
        self.assertEqual(
            types,
            [
                "system",
                "user",
                "system",
                "assistant",
                "tool_use",
                "tool_result",
                "user",
                "assistant",
                "tool_use",
                "tool_result",
                "system",
                "system",
                "tool_use",
                "tool_result",
                "system",
                "user",
                "system",
                "system",
            ],
        )
        for row in rows:
            self.assertEqual((row.host, row.user, row.agent), ("h1", "alice", "gemini-cli"))
            self.assertEqual((row.project_path, row.git_branch), ("/srv/proj", ""))
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        (
            start,
            user1,
            model1,
            reply1,
            listing,
            lsout,
            _u3,
            reply3,
            read_file,
            rfout,
            info,
            model2,
            shell,
            shout,
            dec,
            user4,
            summ,
            rewind,
        ) = rows
        self.assertEqual(
            (
                model1.text,
                model1.model,
                model1.timestamp_utc,
                model1.session_id,
                model1.source_line,
            ),
            (
                "model: gemini-2.5-pro",
                "gemini-2.5-pro",
                "2026-10-01T09:00:07.000Z",
                GEMINI_SESSION,
                4,
            ),
        )
        self.assertEqual(
            (model2.text, model2.model, model2.timestamp_utc, model2.source_line),
            (
                "model: gemini-2.5-pro -> gemini-2.5-flash",
                "gemini-2.5-flash",
                "2026-10-01T09:00:24.000Z",
                13,
            ),
        )
        self.assertEqual(
            (dec.text, dec.tool_name, dec.tool_use_id, dec.timestamp_utc),
            (
                "permission decision: cancelled",
                "run_shell_command",
                "run_shell_command-2",
                "2026-10-01T09:00:24.000Z",
            ),
        )
        self.assertIn("session start: kind=main", start.text)
        self.assertEqual((user1.text, user1.session_id), ("list files in src", GEMINI_SESSION))
        self.assertEqual(
            (reply1.text, reply1.model, reply1.source_line), ("Listing now.", "gemini-2.5-pro", 4)
        )  # re-appended record
        self.assertEqual(
            (listing.tool_name, listing.tool_use_id),
            ("list_directory", "list_directory-1759309207000"),
        )
        self.assertEqual(listing.text, 'ReadFolder: {"path":"/srv/proj/src"}')
        self.assertEqual(listing.timestamp_utc, "2026-10-01T09:00:07.100Z")
        self.assertEqual(
            (lsout.tool_use_id, lsout.text), ("list_directory-1759309207000", "main.ts\nutil.ts")
        )
        self.assertNotIn("delete everything", " ".join(row.text for row in rows))  # rewound
        self.assertEqual(reply3.text, "Here is util.ts.")  # $patch content
        self.assertEqual(rfout.text, "export const x = 1")  # $patch toolCalls result
        self.assertEqual(info.text, "info: Request cancelled.")
        self.assertEqual(
            (shell.text, shell.model), ('Shell: {"command":"curl http://x"}', "gemini-2.5-flash")
        )
        self.assertEqual(shout.text, "[cancelled]")
        self.assertEqual(user4.session_id, GEMINI_RESUMED)  # $set.sessionId on resume
        self.assertEqual(read_file.session_id, GEMINI_SESSION)
        self.assertEqual(
            (summ.text, summ.timestamp_utc, summ.source_line),
            ("summary: List src files", "2026-10-01T09:00:08.000Z", 5),
        )
        self.assertEqual(
            (rewind.text, rewind.source_line), ("rewind to msg-u2: 2 message(s) dropped", 8)
        )

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
        self.assertEqual([row.turn_type for row in rows], ["system", "user"])
        self.assertIn("kind=subagent", rows[0].text)
        self.assertEqual(rows[1].session_id, GEMINI_SUBAGENT)

    def test_awaiting_approval_without_result(self):
        rel = ".gemini/tmp/proj/chats/%s/%s.jsonl" % (GEMINI_SESSION, GEMINI_SUBAGENT)
        call = {
            "id": "run_shell_command-9",
            "name": "run_shell_command",
            "args": {"command": "make deploy"},
            "status": "awaiting_approval",
            "timestamp": "2026-10-01T09:00:32.000Z",
        }
        message = {
            "id": "s-g1",
            "timestamp": "2026-10-01T09:00:31.000Z",
            "type": "gemini",
            "content": "",
            "model": "gemini-2.5-pro",
            "toolCalls": [call],
        }
        with open(self.home / rel, "a", encoding="utf-8") as handle:
            handle.write(json.dumps(message) + "\n")
        rows = self.rows_for(GeminiCliParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows[2:]],
            [
                ("system", "model: gemini-2.5-pro"),
                ("tool_use", '{"command":"make deploy"}'),
                ("system", "permission decision: awaiting_approval"),
            ],
        )
        dec = rows[-1]
        self.assertEqual(
            (dec.tool_name, dec.tool_use_id, dec.timestamp_utc, dec.session_id),
            (
                "run_shell_command",
                "run_shell_command-9",
                "2026-10-01T09:00:32.000Z",
                GEMINI_SUBAGENT,
            ),
        )

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
        self.assertEqual(len(self.rows_for(GeminiCliParser(), self.REL)), 18)

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
        self.assertEqual(len([row for row in rows if row.source_file.endswith(self.REL)]), 18)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_legacy_session_is_reported(self):
        rows = self._rows_with_unreadable(self.LEGACY)
        name = self.LEGACY.split("/")[-1]
        problems = [row for row in rows if row.source_file.endswith(self.LEGACY)]
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text, row.project_path) for row in problems],
            [(0, "system", "parser: %s could not be read (Permission denied)" % name, "/srv/proj")],
        )
        self.assertEqual(len([row for row in rows if row.source_file.endswith(self.REL)]), 18)

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
            self.assertEqual(len([row for row in rows if row.source_file.endswith(self.REL)]), 18)

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
        self.assertEqual(len(before), 18)
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
        self.assertEqual(len(rows), 20)
        added = [row for row in rows if row.source_line == 17]
        self.assertEqual([(row.turn_type, row.text) for row in added], [("user", "after the cut")])
        self.assertEqual(
            [
                (row.source_line, row.turn_type, row.text)
                for row in rows[:-1]
                if row.source_line != 17
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
                16,
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
        if sys.version_info < (3, 11):
            self.assertEqual(
                [row.text for row in rows],
                [
                    "parser: .gemini/policies/auto-saved.toml not read: its [[rule]] tables "
                    "need Python 3.11 or later (tomllib)"
                ],
            )
            return
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: policy allow run_shell_command priority=2.95 "
                'commandPrefix=["git","status"]' + source,
                "permission setting: policy allow search priority=2.95 mcpName=docs "
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

    def test_project_settings_and_policy(self):
        # A project's .gemini/settings.json and workspace policy file: the
        # rows carry the project directory.
        project = self.tmp / "srv" / "proj"
        (project / ".gemini/policies").mkdir(parents=True)
        (project / ".gemini/settings.json").write_text(
            json.dumps({"tools": {"confirmationRequired": ["write_file"]}}), encoding="utf-8"
        )
        (project / ".gemini/policies/auto-saved.toml").write_text(
            '[[rule]]\ndecision = "allow"\npriority = 2.95\ntoolName = "write_file"\n',
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
        if sys.version_info >= (3, 11):
            rel = ".gemini/policies/auto-saved.toml"
            art = Artifact(project / rel, "/srv/proj/" + rel, rel, "gemini-cli", home)
            self.assertEqual(
                [(row.text, row.project_path) for row in parser.parse(art, Options())],
                [
                    (
                        "permission setting: policy allow write_file priority=2.95 "
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
        (2, "tool_use", '{"path":"/srv/proj/src"}'),
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
            ("list_directory", "list_directory-1759309207000") * 2,
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
        unlocked = _parse_with(self, GeminiCliParser)
        subagent = "c3d4e5f6-0000-4000-8000-000000000003.jsonl"
        # (side file made unreadable, the files reading it, the project their rows get)
        for locked, readers, project in (
            (
                ".gemini/tmp/proj/.project_root",
                ["logs.json", "shell_history", self.REL, subagent],
                "/srv/proj",
            ),
            (".gemini/projects.json", [self.LEGACY], GEMINI_HASH),
        ):
            rows = _parse_with(self, GeminiCliParser, locked)
            text = "parser: %s could not be read (Permission denied)" % locked.split("/")[-1]
            reported = _unreadable_rows(rows)
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
        rows = _parse_with(self, GeminiCliParser, ".gemini/tmp/proj/.project_root")
        self.assertEqual(_kept(rows), _kept(unlocked))
