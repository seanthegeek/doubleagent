"""Tests of the Crush parser."""

import json
import unittest
from pathlib import Path

from test_errors import CAN_DENY, DENY_REASON

from doubleagent.inputs import open_input
from doubleagent.parsers import Options
from doubleagent.parsers.crush import CrushParser, project_of
from fixtures import CRUSH_SESSION

from .base import ParserBase


class CrushTests(ParserBase):
    """Test the Crush parser."""

    REL = ".crush/crush.db"

    def test_rows(self):
        rows = self.rows_for(CrushParser(), self.REL)
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "system",
                "user",
                "tool_use",
                "tool_result",
                "system",
                "assistant",
                "system",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id),
                ("h1", "alice", "crush", CRUSH_SESSION),
            )
            self.assertEqual(
                (row.project_path, row.git_branch), ("/alice", "")
            )  # the directory holding .crush
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        start, model, user, use, result, switch, asst, finish = rows
        self.assertEqual(
            start.text, "session start | Fix bug | 3 messages | tokens in=10 out=5 | cost=0.01"
        )
        self.assertEqual(
            (model.text, model.model, model.timestamp_utc, model.source_line),
            (
                "model: claude-sonnet-4 provider=anthropic",
                "anthropic/claude-sonnet-4",
                "2025-10-09T08:53:21.000Z",
                1,
            ),
        )
        # The tool message (m3) names no model and is not a change.
        self.assertEqual(
            (switch.text, switch.model, switch.timestamp_utc, switch.source_line),
            (
                "model: claude-sonnet-4 -> gpt-5 provider=openai",
                "openai/gpt-5",
                "2025-10-09T08:53:24.000Z",
                4,
            ),
        )
        self.assertEqual(asst.model, "openai/gpt-5")
        self.assertEqual(
            (user.text, user.model, user.timestamp_utc),
            ("list files", "anthropic/claude-sonnet-4", "2025-10-09T08:53:21.000Z"),
        )
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text), ("bash", "call_x1", '{"command":"ls"}')
        )
        self.assertEqual(
            use.timestamp_utc, "2025-10-09T08:53:23.000Z"
        )  # finished_at, not created_at
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.text), ("bash", "call_x1", "a.txt")
        )
        self.assertEqual(asst.text, "There is one file, a.txt.")
        self.assertEqual(
            (finish.text, finish.timestamp_utc), ("finish: max_tokens", "2025-10-09T08:53:25.000Z")
        )

    def test_rows_live_only_in_the_wal(self):
        art = next(artifact for artifact in self.col.artifacts if artifact.rel == self.REL)
        import sqlite3

        connection = sqlite3.connect("file:%s?immutable=1" % art.disk_path.as_posix(), uri=True)
        try:
            self.assertEqual(connection.execute("select count(*) from messages").fetchone()[0], 0)
        finally:
            connection.close()
        self.assertTrue(Path(str(art.disk_path) + "-wal").is_file())

    def test_thinking_opt_in(self):
        rows = self.rows_for(CrushParser(), self.REL, include_thinking=True)
        self.assertEqual(
            [row.text for row in rows if row.turn_type == "thinking"], ["one file only"]
        )

    def test_projects_registry(self):
        rows = self.rows_for(CrushParser(), ".local/share/crush/projects.json")
        self.assertEqual(len(rows), 1)
        self.assertEqual(
            (rows[0].turn_type, rows[0].project_path, rows[0].timestamp_utc),
            ("system", "/srv/proj", "2026-10-01T12:00:00.000Z"),
        )
        self.assertIn("data_dir=/srv/proj/.crush", rows[0].text)

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_projects_is_reported(self):
        rel = ".local/share/crush/projects.json"
        path = self.home / rel
        parser = CrushParser()
        rows = []
        path.chmod(0)
        try:
            for artifact in self.col.artifacts:
                if artifact.agent == "crush" and parser.wants(artifact):
                    rows.extend(parser.parse(artifact, Options()))
        finally:
            path.chmod(0o600)
        problems = [row for row in rows if row.source_file.endswith(rel)]
        self.assertEqual(
            [(row.turn_type, row.text) for row in problems],
            [("system", "parser: projects.json could not be read: Permission denied")],
        )
        # The session database beside it is still parsed.
        self.assertEqual(len([row for row in rows if row.source_file.endswith(self.REL)]), 8)

    def test_project_path_from_original(self):
        self.assertEqual(project_of("/srv/proj/.crush/crush.db"), "/srv/proj")
        self.assertEqual(project_of("C:\\Users\\a\\repo\\.crush\\crush.db"), "C:\\Users\\a\\repo")
        self.assertEqual(project_of(".crush/crush.db"), "")

    def test_sidecars_and_lock_not_wanted(self):
        parser = CrushParser()
        seen = 0
        for artifact in self.col.artifacts:
            if artifact.agent == "crush" and artifact.rel.endswith(("-wal", "-shm", "crush.lock")):
                self.assertFalse(parser.wants(artifact), artifact.rel)
                seen += 1
        self.assertGreaterEqual(seen, 3)
        for rel in (".crush/crush.json", ".config/crush/crush.json"):
            self.assertTrue(parser.wants(self.artifact(rel)), rel)

    def test_bad_parts_is_reported_not_fatal(self):
        import sqlite3

        database = self.home / self.REL
        connection = sqlite3.connect(str(database))
        connection.execute(
            "INSERT INTO messages(id,session_id,role,parts,model,provider,created_at,updated_at) "
            "VALUES ('m5','6f1c0001','assistant','[{\"type\":\"text\",\"data\":{\"text\":\"cut','m','p',1760000006,1760000006)"
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(CrushParser(), self.REL)
        # m5 names model "m" from provider "p", so its parse error follows a model row.
        self.assertEqual(len(rows), 10)
        self.assertEqual(rows[-2].text, "model: gpt-5 -> m provider=p")
        self.assertEqual(rows[-1].turn_type, "system")
        self.assertIn("message m5 parts", rows[-1].text)
        self.assertEqual(rows[2].text, "list files")

    def test_not_sqlite_is_reported(self):
        (self.home / self.REL).write_bytes(b"not a database")
        rows = self.rows_for(CrushParser(), self.REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("system", "parser: not a SQLite database")],
        )

    def _insert(self, statements):
        import sqlite3

        connection = sqlite3.connect(str(self.home / self.REL))
        for sql in statements:
            connection.execute(sql)
        connection.commit()
        connection.close()

    def test_bad_parts_carries_the_value_in_full(self):
        # The stored parts value is the row's text, unshortened, and the
        # messages before and after it are still yielded.
        bad = '[{"type":"text","data":{"text":"cut' + " tail" * 60
        insert = (
            "INSERT INTO messages(id,session_id,role,parts,model,provider,created_at,updated_at) "
            "VALUES ('%s','6f1c0001','%s','%s','gpt-5','openai',%d,%d)"
        )
        after = '[{"type":"text","data":{"text":"after"}}]'
        self._insert(
            [
                insert % ("m5", "assistant", bad, 1760000006, 1760000006),
                insert % ("m6", "user", after, 1760000007, 1760000007),
                insert % ("m7", "user", '{"type":"text"}', 1760000008, 1760000008),
            ]
        )
        rows = self.rows_for(CrushParser(), self.REL)
        texts = [row.text for row in rows]
        self.assertEqual(texts[2], "list files")
        self.assertIn("There is one file, a.txt.", texts)
        bad_row = next(
            row for row in rows if row.source_line == 5 and row.text.startswith("parser:")
        )
        self.assertTrue(
            bad_row.text.startswith("parser: unparseable record 5 (message m5 parts: "),
            bad_row.text,
        )
        self.assertTrue(bad_row.text.endswith("): " + bad), bad_row.text)
        self.assertEqual(
            (bad_row.turn_type, bad_row.timestamp_utc, bad_row.session_id, bad_row.project_path),
            ("system", "", CRUSH_SESSION, "/alice"),
        )
        self.assertEqual(texts[texts.index(bad_row.text) + 1], "after")
        self.assertEqual(
            rows[-1].text,
            'parser: unparseable record 7 (message m7 parts: not a JSON array): {"type":"text"}',
        )
        self.assertEqual((rows[-1].source_line, rows[-1].timestamp_utc), (7, ""))

    def test_hyper_prism_model_rows(self):
        # prism_model_id is the served model, model the request.
        from fixtures import CRUSH_HYPER_SESSION, crush_hyper_records

        self._insert(crush_hyper_records())
        rows = [
            row
            for row in self.rows_for(CrushParser(), self.REL)
            if row.session_id == CRUSH_HYPER_SESSION
        ]
        models = [
            (row.text, row.model, row.source_line) for row in rows if row.text.startswith("model:")
        ]
        self.assertEqual(
            models,
            [
                (
                    "model: claude-sonnet-4-5 provider=hyper requested=prism-auto",
                    "hyper/claude-sonnet-4-5",
                    6,
                ),
                (
                    "model: claude-sonnet-4-5 -> gpt-5-mini provider=hyper requested=prism-auto",
                    "hyper/gpt-5-mini",
                    7,
                ),
            ],
        )
        # The third step has no prism_model_id and repeats the request: no row.
        third = next(row for row in rows if row.text == "third step")
        self.assertEqual(third.model, "hyper/prism-auto")
        first = next(row for row in rows if row.text == "first step")
        self.assertEqual(first.model, "hyper/claude-sonnet-4-5")

    def _perm_rows(self):
        from fixtures import crush_permission_records

        self._insert(crush_permission_records())
        return self.rows_for(CrushParser(), self.REL)

    def test_permission_decisions(self):
        # A user denial and hook decisions follow their result.
        rows = self._perm_rows()
        pairs = [
            (rows[i - 1].text, row.text, row.tool_use_id, row.tool_name, row.timestamp_utc)
            for i, row in enumerate(rows)
            if row.text.startswith("permission decision:")
        ]
        self.assertEqual(
            pairs,
            [
                (
                    "[error] User denied permission",
                    "permission decision: denied by user",
                    "call_d1",
                    "bash",
                    "2025-10-09T08:56:41.000Z",
                ),
                (
                    "[error] Tool call blocked by hook. Reason: no rm",
                    "permission decision: deny by hook guard reason=no rm",
                    "call_d2",
                    "bash",
                    "2025-10-09T08:56:42.000Z",
                ),
                (
                    "file contents",
                    "permission decision: allow by hook trust-view",
                    "call_d3",
                    "view",
                    "2025-10-09T08:56:43.000Z",
                ),
                (
                    "[error] Turn halted by hook. Reason: stop",
                    "permission decision: halt by hook",
                    "call_d4",
                    "bash",
                    "2025-10-09T08:56:44.000Z",
                ),
            ],
        )
        for row in rows:
            if row.text.startswith("permission decision:"):
                self.assertEqual((row.turn_type, row.session_id), ("system", "6f1c0003"))

    def test_relay_rows(self):
        from fixtures import CRUSH_CHANNEL_SESSION

        self.assertTrue(CrushParser.records_relay)
        rows = [row for row in self._perm_rows() if row.session_id == CRUSH_CHANNEL_SESSION]
        relays = [
            (row.text, row.timestamp_utc) for row in rows if row.text.startswith("session relayed:")
        ]
        # One row from sessions.channel, one for the first push only.
        self.assertEqual(
            relays,
            [
                (
                    "session relayed: channel signal (from crush.db sessions.channel, "
                    "as of updated_at)",
                    "2025-10-09T08:58:30.000Z",
                ),
                (
                    "session relayed: channel signal sender +15550100 group g-77 "
                    "(from crush.db message c1)",
                    "2025-10-09T08:58:21.000Z",
                ),
            ],
        )
        texts = [row.text for row in rows]
        self.assertEqual(
            texts.index(relays[1][0]) - 1,
            texts.index(next(text for text in texts if text.startswith("<channel "))),
        )

    def test_deleted_parent_row(self):
        rows = [row for row in self._perm_rows() if row.text.startswith("session deleted:")]
        self.assertEqual(
            [(row.session_id, row.text, row.timestamp_utc, row.project_path) for row in rows],
            [
                (
                    "6f1c0099",
                    "session deleted: 6f1c0099 (parent_session_id of sub-agent session "
                    "m9$$call_t1 names no session in crush.db; time is the sub-agent's "
                    "updated_at)",
                    "2025-10-09T09:00:10.000Z",
                    "/alice",
                )
            ],
        )

    def test_mcp_and_file_rows(self):
        rows = self._perm_rows()
        mcp = [row for row in rows if row.text.startswith("permission setting:")]
        self.assertEqual(
            [row.text for row in mcp],
            [
                "permission setting: mcp_enabled_servers=docs (from .crush/crush.db, file mtime)",
                "permission setting: mcp_disabled_servers=github (from .crush/crush.db, file mtime)",
            ],
        )
        self.assertEqual({(row.session_id, row.project_path) for row in mcp}, {("", "/alice")})
        files = [
            (row.session_id, row.text, row.timestamp_utc)
            for row in rows
            if row.text.startswith(("file version:", "file read:"))
        ]
        self.assertEqual(
            files,
            [
                (
                    "6f1c0003",
                    "file version: /srv/proj/a.txt version=1 bytes=5 (from crush.db files)",
                    "2025-10-09T08:56:45.000Z",
                ),
                (
                    "6f1c0003",
                    "file read: /srv/proj/b.txt (from crush.db read_files, read_at)",
                    "2025-10-09T08:56:46.000Z",
                ),
            ],
        )

    def test_config_standing_rows(self):
        from fixtures import CRUSH_CONFIG_JSON

        rel = ".crush/crush.json"
        (self.home / rel).write_text(json.dumps(CRUSH_CONFIG_JSON), encoding="utf-8")
        rows = self.rows_for(CrushParser(), rel)
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: permissions.allowed_tools=view (from .crush/crush.json, file mtime)",
                "permission setting: permissions.allowed_tools=bash:ls (from .crush/crush.json, file mtime)",
                "permission setting: options.disabled_tools=sourcegraph (from .crush/crush.json, file mtime)",
                "permission setting: mcp.docs.disabled_tools=delete (from .crush/crush.json, file mtime)",
                "permission setting: mcp.docs.enabled_tools=search (from .crush/crush.json, file mtime)",
                'permission setting: hooks.PreToolUse={"command":"./guard.sh","matcher":"bash",'
                '"name":"guard"} (from .crush/crush.json, file mtime)',
            ],
        )
        self.assertEqual(
            {(row.turn_type, row.session_id, row.project_path) for row in rows},
            {("system", "", "/alice")},
        )
        self.assertTrue(all(row.timestamp_utc.endswith("Z") for row in rows))
        self.assertFalse(any("sk-crush-secret" in row.text for row in rows))
        # The global config holds no permission key: no rows.
        self.assertEqual(self.rows_for(CrushParser(), ".config/crush/crush.json"), [])

    def test_config_not_json_is_one_row(self):
        rel = ".config/crush/crush.json"
        (self.home / rel).write_text("{not json", encoding="utf-8")
        rows = self.rows_for(CrushParser(), rel)
        self.assertEqual(
            [(row.turn_type, row.text, row.project_path) for row in rows],
            [("system", "parser: .config/crush/crush.json is not a JSON object", "")],
        )

    def test_crushrc_standing_rows(self):
        from fixtures import CRUSHRC

        rel = ".config/crush/crushrc"
        (self.home / rel).write_text(CRUSHRC, encoding="utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        self.assertTrue(CrushParser().wants(self.artifact(rel)))
        rows = self.rows_for(CrushParser(), rel)
        self.assertEqual(
            [(row.text, row.source_line) for row in rows],
            [
                (
                    "permission setting: permissions.allowed_tools=view "
                    "(from .config/crush/crushrc line 3, file mtime)",
                    3,
                ),
                (
                    "permission setting: permissions.allowed_tools=bash:ls "
                    "(from .config/crush/crushrc line 3, file mtime)",
                    3,
                ),
                (
                    "permission setting: options.disabled_tools=sourcegraph "
                    "(from .config/crush/crushrc line 4, file mtime)",
                    4,
                ),
            ],
        )
        self.assertFalse(any("sk-rc-secret" in row.text for row in rows))

    def test_crushrc_bad_line_is_reported(self):
        from fixtures import CRUSHRC

        rel = ".config/crush/crushrc"
        lines = CRUSHRC.split("\n")
        lines.insert(3, "permissions allow 'unclosed")
        (self.home / rel).write_text("\n".join(lines), encoding="utf-8")
        self.col = open_input(self.tmp / "home", self.cat, host="h1")
        rows = self.rows_for(CrushParser(), rel)
        self.assertEqual(
            [row.source_line for row in rows if row.text.startswith("permission setting:")],
            [3, 3, 5],
        )
        self.assertEqual(
            [(row.turn_type, row.text, row.source_line, row.timestamp_utc) for row in rows[3:]],
            [
                (
                    "system",
                    "parser: unparseable line (No closing quotation): permissions allow 'unclosed",
                    4,
                    "",
                )
            ],
        )
