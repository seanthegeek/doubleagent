"""Tests of the Hermes parser."""

import json
import sqlite3
import unittest
from dataclasses import replace as dc_replace

from test_errors import CAN_DENY, DENY_REASON

from doubleagent import cli
from doubleagent.dedup import deduplicate
from doubleagent.inputs import open_input
from doubleagent.parsers import Options
from doubleagent.parsers.hermes import (
    CRON_PROMPT_FRAME_PATTERN,
    HermesParser,
    decode_content,
    read_flat_list,
    yaml_scalar,
)
from fixtures import (
    BAD_LINE,
    HERMES_CHILD,
    HERMES_DIVERTED,
    HERMES_DIVERTED_REL,
    HERMES_PASTES_REL,
    HERMES_PRUNED,
    HERMES_SESSION,
    HERMES_SESSION_JSONL_REL,
    HERMES_SNAPSHOT,
    HERMES_SNAPSHOT_OLD_REL,
    HERMES_SNAPSHOT_REL,
    bad_line_text,
    hermes_pastes_records,
    write_bad_line,
)

from .base import ParserBase, model_or_permission_row, rel_only


class HermesTests(ParserBase):
    """Test the Hermes parser."""

    def test_yaml_comment_and_mapping_after_a_tab(self):
        self.assertEqual(yaml_scalar("a\t# note"), "a")
        self.assertIsNone(yaml_scalar("a:\tb"))
        self.assertEqual(read_flat_list(["key:\t# note", "  - x\t# note"], "key"), ([(2, "x")], ""))
        for space in map(chr, (0x00A0, 0x2028, 0x0085)):
            self.assertEqual(yaml_scalar("a%s# b" % space), "a%s# b" % space, repr(space))
            self.assertEqual(yaml_scalar("a:%sb" % space), "a:%sb" % space, repr(space))

    def test_cron_frame_takes_ascii_digits_only(self):
        frame = "**Prompt Characters:** %s\n## Prompt\n\n"
        self.assertTrue(CRON_PROMPT_FRAME_PATTERN.search(frame % "3"))
        self.assertIsNone(CRON_PROMPT_FRAME_PATTERN.search(frame % chr(0x0663)))

    REL = ".hermes/state.db"

    def test_rows(self):
        rows = [
            row
            for row in self.rows_for(HermesParser(), self.REL)
            if not model_or_permission_row(row)
        ]
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "system",
                "user",
                "tool_use",
                "tool_result",
                "assistant",
                "user",
                "user",
                "user",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.project_path),
                ("h1", "alice", "hermes", "/srv/proj"),
            )
            self.assertTrue(row.timestamp_utc.endswith("Z"), row)
        start, child, user, use, result, asst, rewound, image, sub = rows
        self.assertEqual(
            (start.session_id, start.text, start.timestamp_utc),
            (HERMES_SESSION, "session start | List files | source=cli", "2026-10-01T12:00:00.000Z"),
        )
        self.assertEqual(
            (child.session_id, child.text),
            (
                HERMES_CHILD,
                "session start | Check disk | source=subagent | parent=%s | end=completed"
                % HERMES_SESSION,
            ),
        )
        self.assertEqual(
            (user.text, user.timestamp_utc, user.git_branch, user.model),
            ("list files", "2026-10-01T12:00:01.250Z", "main", "anthropic/claude-sonnet-4"),
        )
        self.assertEqual((use.tool_name, use.tool_use_id, use.text), ("terminal", "call_1", "ls"))
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.text),
            ("terminal", "call_1", "a.txt\nb.txt"),
        )
        self.assertEqual(asst.text, "Two files: a.txt, b.txt.")
        self.assertEqual(rewound.text, "[rewound] delete them instead")
        self.assertEqual(image.text, "what is in this screenshot?\n[image]")
        self.assertEqual(
            (sub.session_id, sub.git_branch, sub.model, sub.project_path),
            (HERMES_CHILD, "", "openai/gpt-5", "/srv/proj"),
        )  # cwd empty: git_repo_root
        self.assertEqual([row.source_line for row in rows[2:]], [1, 2, 3, 4, 5, 6, 7])

    def test_thinking_opt_in(self):
        rows = self.rows_for(HermesParser(), self.REL, include_thinking=True)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows if row.turn_type == "thinking"],
            [("thinking", "User wants a listing.")],
        )
        self.assertNotIn(
            "thinking", [row.turn_type for row in self.rows_for(HermesParser(), self.REL)]
        )

    def test_model_rows(self):
        rows = self.rows_for(HermesParser(), self.REL)
        state = [row for row in rows if model_or_permission_row(row)]
        self.assertEqual(
            [
                (row.session_id, row.text, row.timestamp_utc, row.model, row.source_line)
                for row in state
            ],
            [
                # session_model_usage main-loop rows by first_seen; the
                # title_generation task row is skipped
                (
                    HERMES_SESSION,
                    "model: anthropic/claude-sonnet-4 provider=openrouter",
                    "2026-10-01T12:00:02.500Z",
                    "anthropic/claude-sonnet-4",
                    1,
                ),
                (
                    HERMES_SESSION,
                    "model: anthropic/claude-sonnet-4 -> anthropic/claude-opus-4 provider=openrouter",
                    "2026-10-01T12:00:04.000Z",
                    "anthropic/claude-opus-4",
                    2,
                ),
                # no usage rows: sessions.model at started_at
                (
                    HERMES_CHILD,
                    "model: openai/gpt-5",
                    "2026-10-01T12:15:00.000Z",
                    "openai/gpt-5",
                    2,
                ),
            ],
        )
        for row in state:
            self.assertEqual((row.turn_type, row.project_path), ("system", "/srv/proj"))
        self.assertEqual(rows[1], state[0])  # right after the session's start row
        self.assertEqual(rows[4], state[2])

    def test_database_without_usage_table_uses_session_model(self):
        database = self.tmp / "nousage" / "state.db"
        database.parent.mkdir()
        connection = sqlite3.connect(str(database))
        connection.executescript(
            "CREATE TABLE sessions (id TEXT, source TEXT, model TEXT, started_at REAL);"
            "CREATE TABLE messages (id INTEGER PRIMARY KEY, session_id TEXT, role TEXT,"
            " content TEXT, timestamp REAL);"
            "INSERT INTO sessions VALUES ('s1','cli','m1',1790856000.0);"
        )
        connection.commit()
        connection.close()
        art = dc_replace(
            next(artifact for artifact in self.col.artifacts if artifact.rel == self.REL),
            disk_path=database,
        )
        rows = list(HermesParser().parse(art, Options()))
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("system", "session start | source=cli"), ("system", "model: m1")],
        )

    def test_diverted_jsonl_transcript(self):
        rows = self.rows_for(HermesParser(), HERMES_DIVERTED_REL)
        self.assertEqual(
            [(row.turn_type, row.tool_name, row.tool_use_id, row.text) for row in rows],
            [
                ("user", "", "", "show the env"),
                ("tool_use", "read_file", "call_9", "/srv/proj/.env"),
                ("tool_result", "read_file", "call_9", "TOKEN=x"),
            ],
        )
        self.assertEqual(
            {(row.session_id, row.project_path) for row in rows}, {(HERMES_DIVERTED, "")}
        )
        self.assertEqual(rows[0].timestamp_utc, "2026-10-01T13:00:00.000Z")

    def test_paths_and_noise(self):
        parser = HermesParser()
        for rel in (
            ".hermes/profiles/work/state.db",
            ".hermes_alt/state.db",
            "AppData/Local/hermes/state.db",
            "AppData/Local/hermes/profiles/x/sessions/abc.jsonl",
            ".hermes/state-snapshots/1/state.db",
            "AppData/Local/hermes/profiles/x/state-snapshots/20261001-120000/state.db",
            ".hermes/config.yaml",
            ".hermes/profiles/work/config.yaml",
            ".hermes/pairing/telegram-approved.json",
            "AppData/Local/hermes/platforms/pairing/discord-approved.json",
            ".hermes/.hermes_history",
            ".hermes/cron/output/a1b2/2026-10-01_09-00-00.md",
            ".hermes/response_store.db",
            "trajectory_samples.jsonl",
            "failed_trajectories.jsonl",
            ".config/Hermes/connections.json",
            "Library/Application Support/Hermes/connections.json",
            "AppData/Roaming/Hermes/connections.json",
        ):
            self.assertTrue(parser.wants(rel_only(rel)), rel)
        for rel in (
            ".hermes/state-snapshots/1/manifest.json",
            ".hermes/state-snapshots/1/projects.db",
            ".hermes/state-snapshots/state.db",
            ".hermes/state-snapshots/1/response_store.db",
            ".hermes/sessions/abc.json",
            ".hermes/pairing/telegram-pending.json",
            ".hermes/cron/jobs.json",
            ".config/Hermes/connection.json",
            "srv/proj/trajectory_samples.jsonl",
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)
        for artifact in self.col.artifacts:
            if artifact.agent == "hermes" and artifact.rel.endswith(
                ("auth.json", "sessions.json", "manifest.json", "-wal", "-shm")
            ):
                self.assertFalse(parser.wants(artifact), artifact.rel)
        self.assertEqual(decode_content('\x00json:{"text":"x"}'), {"text": "x"})

    def test_suffixed_homes_are_wanted(self):
        """`HERMES_DATA_DIR_SUFFIX` is appended to the home and the desktop directory unseparated."""
        parser = HermesParser()
        for rel in (
            ".hermes-channel-build-canary/state.db",
            ".hermes-canary/profiles/work/state.db",
            ".hermes-canary/state-snapshots/20261001-120000/state.db",
            "AppData/Local/hermes-canary/sessions/abc.jsonl",
            "AppData/Local/hermes-canary/.hermes_history",
            ".config/Hermes-canary/connections.json",
            "Library/Application Support/Hermes-canary/connections.json",
            "AppData/Roaming/Hermes-canary/connections.json",
        ):
            self.assertTrue(parser.wants(rel_only(rel)), rel)
        for rel in (
            ".hermes-canary/auth.json",
            ".config/Hermes-canary/connection.json",
            ".config/Hermes-canary/sub/connections.json",
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)

    def test_older_schema_without_active_or_reasoning(self):
        database = self.tmp / "old" / "state.db"
        database.parent.mkdir()
        connection = sqlite3.connect(str(database))
        connection.executescript(
            "CREATE TABLE messages (id INTEGER PRIMARY KEY, session_id TEXT, role TEXT, content TEXT,"
            " tool_call_id TEXT, tool_calls TEXT, timestamp REAL);"
            "INSERT INTO messages VALUES (1,'s1','user','hi',NULL,NULL,1790856000.0);"
        )
        connection.commit()
        connection.close()
        art = dc_replace(
            next(artifact for artifact in self.col.artifacts if artifact.rel == self.REL),
            disk_path=database,
        )
        rows = list(HermesParser().parse(art, Options(include_thinking=True)))
        self.assertEqual(
            [(row.session_id, row.turn_type, row.text) for row in rows], [("s1", "user", "hi")]
        )

    def test_truncated_line_is_reported_not_fatal(self):
        write_bad_line(self.home / HERMES_DIVERTED_REL)
        with open(self.home / HERMES_DIVERTED_REL, "a", encoding="utf-8") as handle:
            handle.write(
                "\n"
                + '{"role": "user", "content": "after the cut", "timestamp": 1790859603.0}'
                + "\n"
            )
        rows = self.rows_for(HermesParser(), HERMES_DIVERTED_REL)
        self.assertEqual(
            [(row.source_line, row.turn_type) for row in rows],
            [(1, "user"), (2, "tool_use"), (3, "tool_result"), (5, "user"), (4, "system")],
        )
        self.assertEqual(rows[3].text, "after the cut")
        self.assertEqual(rows[-1].text, bad_line_text())
        self.assertEqual(rows[-1].session_id, HERMES_DIVERTED)

    def test_not_sqlite_is_reported(self):
        (self.home / self.REL).write_text("not a database", encoding="utf-8")
        rows = self.rows_for(HermesParser(), self.REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("system", "parser: not a SQLite database")],
        )

    def test_secondary(self):
        parser = HermesParser()
        self.assertFalse(parser.secondary(rel_only(self.REL)))
        self.assertFalse(parser.secondary(rel_only(".hermes/profiles/work/state.db")))
        for rel in (HERMES_SNAPSHOT_REL, HERMES_DIVERTED_REL, HERMES_SESSION_JSONL_REL):
            self.assertTrue(parser.secondary(rel_only(rel)), rel)
            self.assertEqual(parser.secondary_tolerance(rel_only(rel)), 2.0)

    def test_snapshot_rows_copy_the_live_rows(self):
        snap = self.rows_for(HermesParser(), HERMES_SNAPSHOT_REL)
        self.assertEqual(
            (snap[0].turn_type, snap[0].text, snap[0].timestamp_utc, snap[0].session_id),
            (
                "system",
                "state snapshot %s: older copy of state.db" % HERMES_SNAPSHOT,
                "2026-10-01T12:00:04.000Z",
                "",
            ),
        )

        def key(row):
            return (
                row.turn_type,
                row.text,
                row.session_id,
                row.project_path,
                row.timestamp_utc,
                row.tool_use_id,
            )

        live = {key(row) for row in self.rows_for(HermesParser(), self.REL)}
        main = [row for row in snap if row.session_id == HERMES_SESSION]
        self.assertEqual(
            [(row.turn_type, row.text) for row in main],
            [
                ("system", "session start | List files | source=cli"),
                ("system", "model: anthropic/claude-sonnet-4 provider=openrouter"),
                ("user", "list files"),
                ("tool_use", "ls"),
                ("tool_result", "a.txt\nb.txt"),
                ("assistant", "Two files: a.txt, b.txt."),
            ],
        )
        for row in main:  # copies: same session, cwd and message times
            self.assertIn(key(row), live)
        pruned = [row for row in snap if row.session_id == HERMES_PRUNED]
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc, row.project_path) for row in pruned],
            [
                (
                    "system",
                    "session start | Audit logs | source=cli | end=user_exit",
                    "2026-09-20T09:00:00.000Z",
                    "/srv/proj",
                ),
                (
                    "system",
                    "model: anthropic/claude-sonnet-4",
                    "2026-09-20T09:00:00.000Z",
                    "/srv/proj",
                ),
                ("user", "summarise the audit log", "2026-09-20T09:00:01.000Z", "/srv/proj"),
                (
                    "assistant",
                    "The log shows three failed logins.",
                    "2026-09-20T09:00:02.000Z",
                    "/srv/proj",
                ),
            ],
        )
        self.assertEqual(len(snap), 1 + len(main) + len(pruned))
        old = self.rows_for(HermesParser(), HERMES_SNAPSHOT_OLD_REL)
        self.assertEqual(old[0].timestamp_utc, "2026-10-01T11:00:00.000Z")
        self.assertEqual({row.session_id for row in old[1:]}, {HERMES_PRUNED})

    def test_snapshot_id_without_time(self):
        art = dc_replace(
            next(
                artifact for artifact in self.col.artifacts if artifact.rel == HERMES_SNAPSHOT_REL
            ),
            rel=".hermes/state-snapshots/manual/state.db",
        )
        row = next(iter(HermesParser().parse(art, Options())))
        self.assertEqual(
            (row.text, row.timestamp_utc), ("state snapshot manual: older copy of state.db", "")
        )

    def test_corrupt_snapshot_is_reported(self):
        (self.home / HERMES_SNAPSHOT_REL).write_bytes(b"\x00" * 64)
        rows = self.rows_for(HermesParser(), HERMES_SNAPSHOT_REL)
        self.assertEqual(
            [row.text for row in rows],
            [
                "state snapshot %s: older copy of state.db" % HERMES_SNAPSHOT,
                "parser: not a SQLite database",
            ],
        )

    def test_session_divert_beside_the_live_session(self):
        rows = self.rows_for(HermesParser(), HERMES_SESSION_JSONL_REL)
        self.assertEqual(
            [
                (row.turn_type, row.text, row.session_id, row.project_path, row.timestamp_utc)
                for row in rows
            ],
            [
                (
                    "user",
                    "what is in this screenshot?\n[image]",
                    HERMES_SESSION,
                    "",
                    "2026-10-01T12:00:06.000Z",
                ),
                ("user", "and the hidden files?", HERMES_SESSION, "", "2026-10-01T12:00:07.000Z"),
            ],
        )
        live = [row for row in self.rows_for(HermesParser(), self.REL) if row.text == rows[0].text]
        self.assertEqual(
            [(row.session_id, row.timestamp_utc) for row in live],
            [(HERMES_SESSION, "2026-10-01T12:00:06.000Z")],
        )

    # -- debug report uploads ---------------------------------------------------

    PASTE_TEXT = "debug report uploaded: %s (from pastes/pending.json, expire_at minus 6 hours)"

    def test_debug_report_rows(self):
        parser = HermesParser()
        self.assertFalse(parser.records_sync)
        self.assertTrue(
            parser.wants(rel_only("AppData/Local/hermes/profiles/x/pastes/pending.json"))
        )
        self.assertFalse(parser.secondary(self.artifact(HERMES_PASTES_REL)))
        rows = self.rows_for(parser, HERMES_PASTES_REL)
        self.assertEqual(
            [
                (row.turn_type, row.session_id, row.source_line, row.timestamp_utc, row.text)
                for row in rows
            ],
            [
                (
                    "system",
                    "",
                    1,
                    "2026-10-01T11:20:00.500Z",
                    self.PASTE_TEXT % "https://paste.rs/AbCdE",
                ),
                (
                    "system",
                    "",
                    2,
                    "2026-10-01T11:21:00.000Z",
                    self.PASTE_TEXT % "https://paste.rs/FgHiJ",
                ),
            ],
        )
        self.assertFalse([row for row in rows if row.text.startswith("session synced:")])

    def test_empty_pastes_file_has_no_rows(self):
        (self.home / HERMES_PASTES_REL).write_text("[]", encoding="utf-8")
        self.assertEqual(self.rows_for(HermesParser(), HERMES_PASTES_REL), [])

    def test_pastes_file_not_a_list_is_one_row(self):
        path = self.home / HERMES_PASTES_REL
        path.write_text('{"url": "https://paste.rs/x"}', encoding="utf-8")
        rows = self.rows_for(HermesParser(), HERMES_PASTES_REL)
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [("system", "parser: pastes/pending.json is not a JSON list")],
        )
        path.write_text('[{"url": "https://paste.rs/x", "expire', encoding="utf-8")
        rows = self.rows_for(HermesParser(), HERMES_PASTES_REL)
        self.assertEqual(len(rows), 1)
        self.assertTrue(rows[0].text.startswith("parser: pastes/pending.json did not parse ("))

    def test_bad_paste_entry_is_reported_not_fatal(self):
        recs = [*hermes_pastes_records(), {"expire_at": 1}]
        (self.home / HERMES_PASTES_REL).write_text(json.dumps(recs), encoding="utf-8")
        rows = self.rows_for(HermesParser(), HERMES_PASTES_REL)
        self.assertEqual(len(rows), 3)
        self.assertEqual(
            (rows[-1].source_line, rows[-1].text),
            (3, 'parser: unparseable record 3 (not an object with a url): {"expire_at":1}'),
        )

    # -- model-switch markers and malformed tool_calls --------------------------

    def switch_rows(self):
        from fixtures import build_hermes_switch

        database = self.tmp / "switch" / "state.db"
        build_hermes_switch(database)
        art = dc_replace(self.artifact(self.REL), disk_path=database)
        return list(HermesParser().parse(art, Options()))

    def test_model_switch_markers_are_model_rows(self):
        from fixtures import HERMES_SWITCH

        rows = self.switch_rows()
        self.assertEqual(
            [
                (row.turn_type, row.text, row.timestamp_utc, row.model, row.source_line)
                for row in rows
                if row.turn_type != "system" or row.text.startswith(("model", "session"))
            ],
            [
                ("system", "session start | Switch models | source=tui",
                 "2026-10-01T17:00:00.000Z", "anthropic/claude-opus-4", 1),
                ("system", "model: anthropic/claude-sonnet-4 provider=openrouter",
                 "2026-10-01T17:00:02.000Z", "anthropic/claude-sonnet-4", 1),
                ("user", "hello", "2026-10-01T17:00:01.000Z", "anthropic/claude-opus-4", 1),
                ("assistant", "Hi.", "2026-10-01T17:00:02.000Z", "anthropic/claude-opus-4", 2),
                # the superseded marker (active=0): a model row, not [rewound]
                ("system",
                 "model: anthropic/claude-sonnet-4 -> openai/gpt-5 provider=openrouter",
                 "2026-10-01T17:00:10.000Z", "openai/gpt-5", 3),
                ("user", "list files", "2026-10-01T17:00:11.000Z", "anthropic/claude-opus-4", 4),
                ("assistant", "bad json", "2026-10-01T17:00:12.000Z",
                 "anthropic/claude-opus-4", 5),
                ("tool_use", "pwd", "2026-10-01T17:00:14.000Z", "anthropic/claude-opus-4", 7),
                # no provider in the marker
                ("system", "model: openai/gpt-5 -> anthropic/claude-opus-4",
                 "2026-10-01T17:00:20.000Z", "anthropic/claude-opus-4", 8),
                # a marker text of another form keeps its kind and text
                ("system", "model_switch: [System: model changed]",
                 "2026-10-01T17:00:21.000Z", "anthropic/claude-opus-4", 9),
                ("user", "after the switches", "2026-10-01T17:00:30.000Z",
                 "anthropic/claude-opus-4", 10),
            ],
        )  # fmt: skip
        self.assertFalse([row for row in rows if "[rewound]" in row.text])
        self.assertFalse(
            [row for row in rows if row.turn_type == "user" and "active model" in row.text]
        )
        self.assertEqual(
            {(row.session_id, row.project_path) for row in rows}, {(HERMES_SWITCH, "/srv/proj")}
        )

    def test_model_switch_without_usage_or_earlier_marker(self):
        """Give `model: <new>` for a marker with no earlier marker or usage row.

        The marker names only the new model.
        """
        from fixtures import HERMES_SWITCH_MARKER, HERMES_SWITCH_SCHEMA

        database = self.tmp / "switch1" / "state.db"
        database.parent.mkdir()
        connection = sqlite3.connect(str(database))
        connection.executescript(HERMES_SWITCH_SCHEMA)
        connection.execute(
            "INSERT INTO messages (session_id,role,content,timestamp,display_kind)"
            " VALUES ('s1','user',?,1790874010.0,'model_switch')",
            (HERMES_SWITCH_MARKER % ("openai/gpt-5", " via provider openai"),),
        )
        connection.commit()
        connection.close()
        art = dc_replace(self.artifact(self.REL), disk_path=database)
        rows = list(HermesParser().parse(art, Options()))
        self.assertEqual(
            [(row.turn_type, row.text, row.model) for row in rows],
            [("system", "model: openai/gpt-5 provider=openai", "openai/gpt-5")],
        )

    def test_malformed_tool_calls_are_reported_in_full(self):
        from fixtures import (
            HERMES_BAD_CALLS_ENTRY,
            HERMES_BAD_CALLS_JSON,
            HERMES_BAD_CALLS_SCALAR,
            HERMES_SWITCH,
        )

        try:
            json.loads(HERMES_BAD_CALLS_JSON)
            reason = ""
        except ValueError as error:
            reason = str(error)
        rows = self.switch_rows()
        texts = [row.text for row in rows]
        bad = [row for row in rows if row.text.startswith("parser:")]
        self.assertEqual(
            [(row.turn_type, row.source_line, row.timestamp_utc, row.text) for row in bad],
            [
                (
                    "system",
                    5,
                    "",
                    "parser: unparseable record 5 (tool_calls is not valid JSON: %s): %s"
                    % (reason, HERMES_BAD_CALLS_JSON),
                ),
                (
                    "system",
                    6,
                    "",
                    "parser: unparseable record 6 (tool_calls is not a list): %s"
                    % HERMES_BAD_CALLS_SCALAR,
                ),
                (
                    "system",
                    7,
                    "",
                    "parser: unparseable record 7 (tool_calls entry is not an object): %s"
                    % HERMES_BAD_CALLS_ENTRY,
                ),
            ],
        )
        for row in bad:
            self.assertEqual((row.session_id, row.project_path), (HERMES_SWITCH, "/srv/proj"))
        # the rows of the same messages, and those before and after, still come out
        self.assertEqual(texts.index(bad[0].text), texts.index("bad json") + 1)
        self.assertEqual(texts.index(bad[2].text), texts.index("pwd") + 1)
        self.assertLess(texts.index("list files"), texts.index("bad json"))
        self.assertLess(texts.index(bad[2].text), texts.index("after the switches"))

    def test_malformed_tool_calls_in_divert_line(self):
        """Report a divert line's tool_calls of a type other than a list, with the value.

        The report sits beside the bad-line rows.
        """
        path = self.home / HERMES_DIVERTED_REL
        with open(path, "a", encoding="utf-8") as handle:
            handle.write(
                json.dumps({"role": "assistant", "content": "x", "tool_calls": "nope"}) + "\n"
            )
        write_bad_line(path)
        rows = self.rows_for(HermesParser(), HERMES_DIVERTED_REL)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text) for row in rows],
            [
                (1, "user", "show the env"),
                (2, "tool_use", "/srv/proj/.env"),
                (3, "tool_result", "TOKEN=x"),
                (4, "assistant", "x"),
                (4, "system", 'parser: unparseable record 4 (tool_calls is not valid JSON: '
                 'Expecting value: line 1 column 1 (char 0)): nope'),
                (5, "system", bad_line_text()),
            ],
        )  # fmt: skip
        self.assertEqual({row.session_id for row in rows}, {HERMES_DIVERTED})

    def test_volume_home(self):
        """Read a Hermes home collected from a Docker named volume.

        Its files sit at the base's root; the parser reads them as it reads a home's
        `.hermes`.
        """
        from fixtures import HERMES_VOLUME_HOME, build_hermes_volume

        parser = HermesParser()
        for rel in (
            "state.db",
            "profiles/work/state.db",
            "sessions/abc.jsonl",
            "state-snapshots/20261001-120000/state.db",
            "pastes/pending.json",
            "config.yaml",
            "response_store.db",
            ".hermes_history",
            "pairing/telegram-approved.json",
            "cron/output/a1b2/2026-10-01_09-00-00.md",
        ):
            self.assertTrue(parser.wants(rel_only(rel)), rel)
        for rel in (
            "auth.json",
            "state.db-wal",
            "pairing/telegram-pending.json",
            "sessions/abc.json",
            "state-snapshots/state.db",
            ".config/Hermes/state.db",
            "x/state.db",
        ):
            self.assertFalse(parser.wants(rel_only(rel)), rel)
        self.assertTrue(parser.secondary(rel_only("sessions/abc.jsonl")))
        self.assertFalse(parser.secondary(rel_only("state.db")))

        root = self.tmp / "volume"
        build_hermes_volume(root)
        col = open_input(root, self.cat)
        self.assertEqual(col.kind, "collected")
        wanted = sorted(
            artifact.rel
            for artifact in col.artifacts
            if artifact.agent == "hermes" and parser.wants(artifact)
        )
        self.assertEqual(
            wanted,
            sorted(
                [
                    "state.db",
                    "sessions/%s.jsonl" % HERMES_DIVERTED,
                    "pastes/pending.json",
                    "config.yaml",  # no command_allowlist: no rows
                ]
            ),
        )
        rows, _, problems, _ = cli.collect_rows(col, Options(), [])
        self.assertEqual(problems, [])
        self.assertEqual(
            {(row.host, row.user, row.agent) for row in rows}, {("h2", "docker", "hermes")}
        )
        main = [row for row in rows if row.session_id == HERMES_SESSION]
        self.assertIn("list files", [row.text for row in main])
        self.assertEqual(
            {row.project_path for row in main if row.turn_type == "user"}, {"/opt/data/workspace"}
        )
        self.assertIn(
            "show the env", [row.text for row in rows if row.session_id == HERMES_DIVERTED]
        )
        self.assertEqual(
            len([row for row in rows if row.text.startswith("debug report uploaded:")]), 2
        )
        self.assertTrue(
            all(artifact.home.original == HERMES_VOLUME_HOME for artifact in col.artifacts)
        )

    # -- relay, permissions, deletion, past use, origin, scratch ----------------
    # Source-derived from hermes-agent 86fa9e6; no real install has shown these.

    def hermes_art(self, rel, path, project=""):
        """Return an artifact for the file at `path` with `rel` as its path.

        The artifact sits in a project base `project` when one is given.
        """
        art = dc_replace(self.artifact(self.REL), disk_path=path, rel=rel, original=str(path))
        if project:
            art = dc_replace(art, home=dc_replace(art.home, project=True, original=project))
        return art

    def hermes_file(self, rel, text):
        path = self.tmp / "h208" / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def gateway_rows(self):
        from fixtures import build_hermes_gateway

        database = self.tmp / "gateway" / "state.db"
        build_hermes_gateway(database)
        return list(HermesParser().parse(self.hermes_art(".hermes/state.db", database), Options()))

    def test_gateway_session_rows(self):
        from fixtures import HERMES_IMPORTED, HERMES_TG, HERMES_WORKTREE

        parser = HermesParser()
        self.assertTrue(parser.records_relay)
        rows = self.gateway_rows()
        by_session = {}
        for row in rows:
            if row.turn_type == "system" and not row.text.startswith(("parser:", "permission d")):
                by_session.setdefault(row.session_id, []).append((row.text, row.timestamp_utc))
        self.assertEqual(
            by_session[HERMES_TG],
            [
                ("session start | Ops | source=telegram | archived=1", "2026-10-01T18:00:00.000Z"),
                ("model: openai/gpt-5", "2026-10-01T18:00:00.000Z"),
                (
                    "session relayed: telegram chat -100123 type group thread - user 42 name "
                    "alice_tg display Ops chat (from state.db sessions, started_at)",
                    "2026-10-01T18:00:00.000Z",
                ),
                (
                    "permission setting: yolo_mode=true (from state.db sessions.model_config, "
                    "started_at)",
                    "2026-10-01T18:00:00.000Z",
                ),
            ],
        )
        self.assertEqual(
            by_session[HERMES_IMPORTED][0][0],
            "session start | Imported from Claude Code: fix tests | source=claude-code | "
            "imported from claude-code /home/alice/.claude/projects/-srv-proj/c1.jsonl session c1",
        )
        self.assertFalse([text for text, _ in by_session[HERMES_IMPORTED] if "relayed" in text])
        self.assertEqual(
            by_session[HERMES_WORKTREE][-1],
            (
                "scratch directory: /srv/proj/.worktrees/feat (git worktree the agent created "
                "in /srv/proj, branch hermes/feat)",
                "2026-10-01T18:20:00.000Z",
            ),
        )
        relayed = [row for row in rows if row.text.startswith("session relayed:")]
        self.assertEqual([row.session_id for row in relayed], [HERMES_TG])

    def test_terminal_permission_decisions(self):
        rows = self.gateway_rows()
        texts = [(row.turn_type, row.tool_use_id, row.text) for row in rows if row.tool_use_id]
        self.assertEqual(
            [text for text in texts if text[0] == "system"],
            [
                (
                    "system",
                    "call_a",
                    "permission decision: blocked BLOCKED by smart approval: deletes system files",
                ),
                (
                    "system",
                    "call_b",
                    "permission decision: approved (Command required approval (restarts a "
                    "service) and was approved by the user.)",
                ),
                ("system", "call_c", "permission decision: pending pattern=pipe_to_shell "
                 "smart_denied=false"),
            ],
        )  # fmt: skip
        for i, text in enumerate(texts):  # each right after its call's tool_result
            if text[0] == "system":
                self.assertEqual((texts[i - 1][0], texts[i - 1][1]), ("tool_result", text[1]))
        perm = next(
            row for row in rows if row.tool_use_id == "call_a" and row.turn_type == "system"
        )
        self.assertEqual(
            (perm.tool_name, perm.timestamp_utc), ("terminal", "2026-10-01T18:00:03.000Z")
        )

    def test_deletion_generation_and_room_rows(self):
        from fixtures import HERMES_DELETED, HERMES_SWEPT

        rows = self.gateway_rows()
        deleted = [row for row in rows if row.text.startswith("session deleted:")]
        tail = " (from state.db state_meta gateway.retired_session.v1, retired_at)"
        self.assertEqual(
            [(row.session_id, row.text, row.timestamp_utc) for row in deleted],
            [
                (HERMES_DELETED, "session deleted: %s%s" % (HERMES_DELETED, tail),
                 "2026-10-01T18:40:00.500Z"),
                (HERMES_SWEPT, "session deleted: %s sweep%s" % (HERMES_SWEPT, tail),
                 "2026-10-01T18:41:00.000Z"),
            ],
        )  # fmt: skip
        peers = [row for row in rows if row.text.startswith("relay peer:")]
        self.assertEqual(
            [(row.text, row.timestamp_utc, row.session_id) for row in peers],
            [
                (
                    "relay peer: telegram agent:main:telegram:group:-100123 generation 2 "
                    "(from state.db conversation_generations)",
                    "",
                    "",
                )
            ],
        )
        rooms = [row for row in rows if row.text.startswith("bot mode room")]
        self.assertEqual(
            [(row.turn_type, row.text, row.timestamp_utc) for row in rooms],
            [
                ("system", "bot mode room Planning (room1) seq 1 message.user actor user:u1 "
                 "(Alice): plan the release", "2026-10-01T19:00:01.000Z"),
                ("system", "bot mode room Planning (room1) seq 2 message.member actor "
                 "member:m1 (-): Draft plan ready.", "2026-10-01T19:00:02.000Z"),
                ("system", 'bot mode room Planning (room1) seq 4 room.renamed actor system:s '
                 '(-): {"name":"Release"}', "2026-10-01T19:00:04.000Z"),
            ],
        )  # fmt: skip

    def test_gateway_bad_records_are_reported(self):
        from fixtures import HERMES_BADORIGIN

        def reason(text):
            try:
                json.loads(text)
            except ValueError as error:
                return str(error)
            return ""

        rows = self.gateway_rows()
        bad = [row for row in rows if row.text.startswith("parser:")]
        self.assertEqual(
            [(row.session_id, row.source_line, row.timestamp_utc, row.text) for row in bad],
            [
                (HERMES_BADORIGIN, 4, "", "parser: unparseable record 4 (sessions.origin_json: "
                 "%s): {bad" % reason("{bad")),
                ("broken", 4, "", "parser: unparseable record 4 (state_meta.value: %s): {nope"
                 % reason("{nope")),
                ("", 3, "", 'parser: unparseable record 3 (hosted_room_events.payload_json: '
                 '%s): {"name": "Plan' % reason('{"name": "Plan')),
            ],
        )  # fmt: skip
        self.assertEqual(rows[-3:], bad)  # after every other row of the file
        self.assertIn(
            "session start | Bad | source=cli",
            [row.text for row in rows if row.session_id == HERMES_BADORIGIN],
        )

    def test_prompt_history(self):
        from fixtures import HERMES_HISTORY_TEXT

        rel = ".hermes/.hermes_history"
        path = self.hermes_file(rel, HERMES_HISTORY_TEXT)
        art = self.hermes_art(rel, path)
        parser = HermesParser()
        self.assertTrue(parser.secondary(art))
        self.assertEqual(parser.secondary_tolerance(art), 14 * 3600 + 2.0)
        rows = list(parser.parse(art, Options()))
        self.assertEqual(
            [
                (row.source_line, row.turn_type, row.text, row.timestamp_utc, row.session_id)
                for row in rows
            ],
            [
                (2, "user", "list files", "2026-10-01T12:00:01.250Z", ""),
                (5, "system", "slash command: /model openai/gpt-5", "2026-10-01T14:00:05.123Z", ""),
                (8, "user", "first line\nsecond line", "2026-10-01T14:01:00.000Z", ""),
            ],
        )
        # the TUI's UTC entry pairs with the state.db prompt it repeats
        live = self.rows_for(parser, self.REL)
        kept, dropped = deduplicate(live, [(row, parser.secondary_tolerance(art)) for row in rows])
        self.assertEqual(dropped["hermes"], 1)
        self.assertEqual([row.text for row in kept[len(live) :]], [row.text for row in rows[1:]])

    def test_prompt_history_truncated_line(self):
        from fixtures import HERMES_HISTORY_TEXT

        rel = ".hermes/.hermes_history"
        path = self.hermes_file(rel, HERMES_HISTORY_TEXT)
        write_bad_line(path)
        with open(path, "a", encoding="utf-8") as handle:
            handle.write("\n# 2026-10-01 15:00:00.000\n+after the cut\n")
        rows = list(HermesParser().parse(self.hermes_art(rel, path), Options()))
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [
                ("user", "list files"),
                ("system", "slash command: /model openai/gpt-5"),
                ("user", "first line\nsecond line"),
                ("user", "after the cut"),
                ("system", "parser: unparseable line (not a FileHistory line): " + BAD_LINE),
            ],
        )

    def test_cron_run_files(self):
        from fixtures import (
            HERMES_CRON_JOB,
            HERMES_CRON_PROMPT,
            HERMES_CRON_REPLY,
            hermes_cron_docs,
        )

        parser = HermesParser()
        out = {}
        for name, doc in hermes_cron_docs().items():
            rel = ".hermes/cron/output/%s/%s" % (HERMES_CRON_JOB, name)
            art = self.hermes_art(rel, self.hermes_file(rel, doc))
            self.assertTrue(parser.secondary(art))
            self.assertEqual(parser.secondary_tolerance(art), 14 * 3600 + 2.0)
            out[name] = [
                (row.turn_type, row.text, row.timestamp_utc) for row in parser.parse(art, Options())
            ]
        head = "cron run: job %s (from cron/output, file name time, host-local)" % HERMES_CRON_JOB
        self.assertEqual(
            out["2026-10-01_09-00-00.md"],
            [
                ("system", head, "2026-10-01T09:00:00.000Z"),
                ("user", HERMES_CRON_PROMPT, "2026-10-01T09:00:00.000Z"),
                ("assistant", HERMES_CRON_REPLY, "2026-10-01T09:00:00.000Z"),
            ],
        )
        self.assertEqual(
            [entry[:2] for entry in out["2026-09-30_09-00-00.md"]],
            [("system", head), ("user", "Summarise yesterday."), ("assistant", "Nothing new.")],
        )
        self.assertEqual(
            [entry[:2] for entry in out["2026-09-29_09-00-00.md"]],
            [
                ("system", head),
                (
                    "system",
                    "cron run output: # Cron Job: Daily summary (FAILED)\n\n"
                    "Error: provider unreachable",
                ),
            ],
        )

    def test_response_store(self):
        from fixtures import HERMES_API_SESSION, build_hermes_responses

        rel = ".hermes/response_store.db"
        path = self.tmp / "h208" / rel
        build_hermes_responses(path)
        art = self.hermes_art(rel, path)
        parser = HermesParser()
        self.assertTrue(parser.secondary(art))
        rows = list(parser.parse(art, Options()))
        head = (
            "api server response %s status=completed model=hermes-agent conversation=%s"
            " (from response_store.db responses, created_at)"
        )
        reason = ""
        try:
            json.loads("{cut")
        except ValueError as error:
            reason = str(error)
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text, row.timestamp_utc, row.session_id) for row in rows],
            [
                (2, "system", head % ("resp_1", "-"), "2026-10-01T20:00:00.000Z",
                 HERMES_API_SESSION),
                (2, "user", "what is 2+2?", "", HERMES_API_SESSION),
                (2, "assistant", "4", "", HERMES_API_SESSION),
                (1, "system", head % ("resp_2", "math"), "2026-10-01T20:00:10.000Z",
                 HERMES_API_SESSION),
                (1, "user", "and 3+3?", "", HERMES_API_SESSION),
                (1, "assistant", "6", "", HERMES_API_SESSION),
                (3, "system", "parser: unparseable record 3 (responses.data: %s): {cut" % reason,
                 "", ""),
            ],
        )  # fmt: skip

    def test_pairing_and_allowlist_settings(self):
        from fixtures import HERMES_CONFIG_YAML, HERMES_PAIRING

        parser = HermesParser()
        rel = ".hermes/pairing/telegram-approved.json"
        art = self.hermes_art(rel, self.hermes_file(rel, json.dumps(HERMES_PAIRING)))
        self.assertFalse(parser.secondary(art))
        rows = list(parser.parse(art, Options()))
        self.assertEqual(
            [(row.source_line, row.text, row.timestamp_utc, row.session_id) for row in rows],
            [
                (1, "permission setting: pairing.telegram.42 user_name=alice_tg (from "
                 "pairing/telegram-approved.json, approved_at)", "2026-10-01T12:00:00.000Z", ""),
                (2, 'parser: unparseable record 2 (not an object): {"43":"not an object"}', "",
                 ""),
            ],
        )  # fmt: skip
        rel = ".hermes/config.yaml"
        art = self.hermes_art(rel, self.hermes_file(rel, HERMES_CONFIG_YAML))
        rows = list(parser.parse(art, Options()))
        tail = " (from config.yaml, file mtime)"
        self.assertEqual(
            [(row.source_line, row.text) for row in rows],
            [
                (5, "permission setting: command_allowlist=rm -rf /tmp/*" + tail),
                (6, "permission setting: command_allowlist=git push --force*" + tail),
                (7, "permission setting: command_allowlist=echo 'hi'" + tail),
            ],
        )
        self.assertTrue(all(row.timestamp_utc.endswith("Z") for row in rows))

    def test_allowlist_that_is_not_a_flat_list(self):
        parser = HermesParser()
        rel = ".hermes/config.yaml"
        cases = {
            "command_allowlist:\n  - ok\n  - key: value\n": (
                ["permission setting: command_allowlist=ok (from config.yaml, file mtime)"],
                "line 3: not a string item of command_allowlist",
            ),
            "command_allowlist: [a, b]\n": ([], "line 1: command_allowlist is not a block list"),
            "command_allowlist:\n  - one\n    two\n": (
                ["permission setting: command_allowlist=one (from config.yaml, file mtime)"],
                "line 3: not a string item of command_allowlist",
            ),
        }
        for text, (settings, problem) in cases.items():
            art = self.hermes_art(rel, self.hermes_file(rel, text))
            rows = [row.text for row in parser.parse(art, Options())]
            self.assertEqual(
                rows,
                [
                    *settings,
                    "parser: config.yaml command_allowlist is not a flat list (%s)" % problem,
                ],
            )
        for text in ("model: x\n", "command_allowlist: []\nother: 1\n", "command_allowlist:\n"):
            art = self.hermes_art(rel, self.hermes_file(rel, text))
            self.assertEqual(list(parser.parse(art, Options())), [], text)

    def test_allowlist_line_ends_and_trims(self):
        from doubleagent.parsers.hermes import read_flat_list, yaml_scalar
        from doubleagent.textlines import split_lines

        key = "command_allowlist"
        parser = HermesParser()
        rel = ".hermes/config.yaml"

        def patterns(text):
            art = self.hermes_art(rel, self.hermes_file(rel, text))
            return [
                (row.source_line, row.text.split(" (from")[0])
                for row in parser.parse(art, Options())
            ]

        prefix = "permission setting: command_allowlist="
        self.assertEqual(patterns("command_allowlist:\n  - a\u2028b\n"), [(2, prefix + "a\u2028b")])
        self.assertEqual(patterns("command_allowlist:\n  - rm\u00a0\n"), [(2, prefix + "rm\u00a0")])
        with_lf = "model: x\ncommand_allowlist:\n  - one\n  - 'two'\nother: 1\n"
        self.assertEqual(patterns(with_lf.replace("\n", "\r\n")), patterns(with_lf))
        self.assertEqual(patterns(with_lf), [(3, prefix + "one"), (4, prefix + "two")])
        self.assertEqual(yaml_scalar("x\u00a0"), "x\u00a0")
        # A U+00A0 is not YAML indentation or a blank after the key.
        self.assertEqual(
            read_flat_list(split_lines("command_allowlist:\n  - one\n\u00a0- two\n"), key),
            ([(2, "one")], ""),
        )
        self.assertEqual(
            read_flat_list(split_lines("command_allowlist:\u00a0\n  - one\n"), key),
            ([], "line 1: command_allowlist is not a block list"),
        )
        self.assertIsNone(yaml_scalar("'x'\u00a0"))

    def test_desktop_connections(self):
        from fixtures import HERMES_CONNECTIONS

        parser = HermesParser()
        rel = ".config/Hermes/connections.json"
        art = self.hermes_art(rel, self.hermes_file(rel, json.dumps(HERMES_CONNECTIONS)))
        self.assertFalse(parser.secondary(art))
        rows = list(parser.parse(art, Options()))
        tail = " (from connections.json, file mtime)"
        self.assertEqual(
            [(row.source_line, row.text, row.session_id) for row in rows],
            [
                (2, "remote backend: remote Homelab https://hermes.example.net" + tail, ""),
                (3, "remote backend: ssh Build box ci@build:22" + tail, ""),
                (4, "remote backend: cloud Cloud https://x.cloud.example org=acme name=prod"
                 + tail, ""),
                (0, "remote backend quarantined by the app: missing label" + tail, ""),
                (5, 'parser: unparseable record 5 (not an object): "not an object"', ""),
            ],
        )  # fmt: skip
        self.assertFalse([row for row in rows if "not-real" in row.text])

    @unittest.skipUnless(CAN_DENY, DENY_REASON)
    def test_unreadable_files_are_one_row(self):
        # A settings, pastes, pairing, connections or cron file that is there but cannot be read
        # is one `could not be read` row, never a `did not parse` row or a problem line.
        from fixtures import HERMES_CONFIG_YAML, HERMES_CONNECTIONS, HERMES_PAIRING

        parser = HermesParser()
        cron = ".hermes/cron/output/a1b2/2026-10-01_09-00-00.md"
        cases = (
            (".hermes/pastes/pending.json", "[]", []),
            (".hermes/config.yaml", HERMES_CONFIG_YAML, []),
            (".hermes/pairing/telegram-approved.json", json.dumps(HERMES_PAIRING), []),
            (".config/Hermes/connections.json", json.dumps(HERMES_CONNECTIONS), []),
            (cron, "## Prompt\n\nhi\n", ["cron run: job a1b2"]),
        )
        for rel, text, kept in cases:
            path = self.hermes_file(rel, text)
            path.chmod(0)
            try:
                rows = list(parser.parse(self.hermes_art(rel, path), Options()))
                database = self.rows_for(parser, self.REL)
            finally:
                path.chmod(0o600)
            problems = [row for row in rows if row.text.startswith("parser:")]
            self.assertEqual(
                [
                    (row.turn_type, row.text, row.source_line, row.timestamp_utc, row.session_id)
                    for row in problems
                ],
                [
                    (
                        "system",
                        "parser: %s could not be read (Permission denied)" % path.name,
                        0,
                        "",
                        "",
                    )
                ],
                rel,
            )
            self.assertEqual(problems[0].project_path, "", rel)
            others = [row.text for row in rows if not row.text.startswith("parser:")]
            self.assertEqual(len(others), len(kept), rel)
            for text_start, other in zip(kept, others, strict=True):
                self.assertTrue(other.startswith(text_start), (rel, other))
            self.assertTrue(any(not row.text.startswith("parser:") for row in database), rel)

    def test_trajectory_file(self):
        from fixtures import _jsonl, hermes_trajectory_records

        parser = HermesParser()
        rel = "trajectory_samples.jsonl"
        path = self.tmp / "h208" / "proj" / rel
        _jsonl(path, hermes_trajectory_records())
        art = self.hermes_art(rel, path, project="/srv/proj")
        self.assertTrue(parser.secondary(art))
        rows = list(parser.parse(art, Options()))
        line2 = json.dumps(hermes_trajectory_records()[1])
        self.assertEqual(
            [(row.source_line, row.turn_type, row.text, row.timestamp_utc) for row in rows],
            [
                (1, "system", "trajectory: model=anthropic/claude-sonnet-4 completed=true (from "
                 "trajectory_samples.jsonl, timestamp host-local)", "2026-10-01T12:00:05.000Z"),
                (1, "system", "trajectory system prompt: You are a function calling AI model.",
                 ""),
                (1, "user", "list files", ""),
                (1, "assistant", "<tool_call>ls</tool_call>", ""),
                (1, "tool_result", "<tool_response>a.txt</tool_response>", ""),
                (1, "assistant", "One file.", ""),
                (2, "system", "parser: unparseable line (conversations is not a list): " + line2,
                 ""),
            ],
        )  # fmt: skip
        self.assertEqual({row.project_path for row in rows}, {"/srv/proj"})
        self.assertEqual({row.session_id for row in rows}, {""})

    def test_trajectory_truncated_line(self):
        from fixtures import _jsonl, hermes_trajectory_records

        rel = "failed_trajectories.jsonl"
        path = self.tmp / "h208" / "proj" / rel
        _jsonl(path, hermes_trajectory_records()[:1])
        write_bad_line(path)
        with open(path, "a", encoding="utf-8") as handle:
            handle.write("\n" + json.dumps(hermes_trajectory_records()[0]) + "\n")
        rows = list(HermesParser().parse(self.hermes_art(rel, path, "/srv/proj"), Options()))
        self.assertEqual([row.text for row in rows if row.turn_type == "user"], ["list files"] * 2)
        self.assertEqual(rows[-1].text, bad_line_text())
        self.assertEqual((rows[-1].source_line, rows[-1].project_path), (2, "/srv/proj"))
