"""Tests of the Zed parser."""

import json
import sqlite3

import zstandard

from doubleagent.parsers import Options
from doubleagent.parsers.antigravity import file_mtime
from doubleagent.parsers.zed import ZedParser
from fixtures import (
    PROJECT_REMOTE,
    ZED_EXTERNAL,
    ZED_TEMP_DIR,
    ZED_THREAD,
    ZED_WORKTREE,
    ZED_WORKTREE_THREAD,
    zed_thread_record,
)

from .base import ParserBase, model_or_permission_row, rel_only


class ZedTests(ParserBase):
    """Test the Zed parser."""

    REL = ".local/share/zed/threads/threads.db"
    SIDEBAR = ".local/share/zed/db/0-stable/db.sqlite"

    def _insert(self, thread_id, data_type, data, updated="2026-10-03T12:00:00+00:00"):
        connection = sqlite3.connect(str(self.home / self.REL))
        connection.execute(
            "INSERT INTO threads(id,summary,updated_at,data_type,data,folder_paths) VALUES (?,?,?,?,?,?)",
            (thread_id, "extra", updated, data_type, data, "/srv/other"),
        )
        connection.commit()
        connection.close()

    def test_rows(self):
        rows = [
            row
            for row in self.rows_for(ZedParser(), self.REL)
            if not model_or_permission_row(row) and not row.text.startswith("scratch directory:")
        ]
        self.assertEqual(
            [row.turn_type for row in rows],
            [
                "system",
                "system",
                "user",
                "tool_use",
                "tool_result",
                "tool_use",
                "tool_result",
                "assistant",
                "system",
                "system",
                "system",
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.host, row.user, row.agent, row.session_id), ("h1", "alice", "zed", ZED_THREAD)
            )
            self.assertEqual(
                (row.project_path, row.git_branch, row.model),
                ("/srv/proj", "main", "anthropic/claude-sonnet-4-5"),
            )
            self.assertEqual(row.source_line, 1)
        start, remote, user, use, result, _use2, _result2, asst, resume, compaction, draft = rows
        # the snapshot's git_state.remote_url, at the thread-start time
        self.assertEqual(
            (remote.text, remote.timestamp_utc),
            ("git remote: " + PROJECT_REMOTE, start.timestamp_utc),
        )
        # the blob's draft_prompt (source-derived, 9cd14da) follows the messages
        self.assertEqual(draft.text, "unsent draft: and then lint (thread blob, as of updated_at)")
        self.assertEqual(draft.timestamp_utc, "2026-10-03T09:01:05.000Z")
        self.assertEqual(start.timestamp_utc, "2026-10-03T09:00:00.000Z")  # project snapshot time
        self.assertEqual(start.text, "thread start: Fix flaky test version=0.3.0")
        self.assertEqual(
            user.timestamp_utc, "2026-10-03T09:01:05.000Z"
        )  # thread updated_at, approximate
        self.assertEqual(user.text, "run the tests\n[mention] file:///srv/proj/README.md")
        self.assertEqual(
            (use.tool_name, use.tool_use_id, use.text),
            ("terminal", "toolu_01", '{"command":"pytest -q"}'),
        )
        self.assertEqual(
            (result.tool_name, result.tool_use_id, result.text),
            ("terminal", "toolu_01", "3 passed"),
        )
        self.assertEqual(asst.text, "All 3 tests pass.")
        self.assertEqual((resume.text, compaction.text), ("resume", "compaction: ran the tests"))

    def test_thinking_opt_in(self):
        rows = self.rows_for(ZedParser(), self.REL, include_thinking=True)
        self.assertEqual([row.text for row in rows if row.turn_type == "thinking"], ["use pytest"])

    def test_sidebar_external_threads(self):
        rows = [row for row in self.rows_for(ZedParser(), self.SIDEBAR) if "agent=" in row.text]
        self.assertEqual(len(rows), 1)
        row = rows[0]
        self.assertEqual(
            (row.turn_type, row.session_id, row.project_path), ("system", ZED_EXTERNAL, "/srv/proj")
        )
        self.assertEqual(row.timestamp_utc, "2026-10-03T10:00:00.000Z")
        self.assertIn("agent=claude-code", row.text)

    def test_database_sqlite_stores(self):
        # Source-derived (zed-industries/zed at 9cd14da): the native sidebar
        # state, project trust, draft prompts, the worktree registry, the
        # archived worktrees and the sidebar terminals of db.sqlite.
        art = self.artifact(self.SIDEBAR)
        mtime = file_mtime(art)
        rows = list(ZedParser().parse(art, Options()))
        rel = self.SIDEBAR
        self.assertEqual(
            [(row.timestamp_utc, row.session_id, row.project_path, row.text) for row in rows],
            [
                (
                    "2026-10-03T09:01:05.000Z",
                    ZED_THREAD,
                    "/srv/proj",
                    "sidebar thread: title=Fix flaky test archived=false interacted_at=-"
                    " main_worktree=-",
                ),
                (
                    "2026-10-03T10:00:00.000Z",
                    ZED_EXTERNAL,
                    "/srv/proj",
                    "external agent thread (no transcript in threads.db): agent=claude-code"
                    " title=External thread",
                ),
                (
                    "2026-10-03T12:00:00.000Z",
                    ZED_WORKTREE_THREAD,
                    ZED_WORKTREE,
                    "sidebar thread: title=Fix tests in a worktree archived=true"
                    " interacted_at=2026-10-03T11:59:00.000Z main_worktree=/srv/proj",
                ),
                (
                    mtime,
                    "",
                    "/srv/proj",
                    "permission setting: trusted_worktrees./srv/proj=true (from %s"
                    " trusted_worktrees, file mtime)" % rel,
                ),
                (
                    mtime,
                    "",
                    "/home/alice/remote",
                    "permission setting: trusted_worktrees./home/alice/remote=true"
                    " host=alice@build.example (from %s trusted_worktrees, file mtime)" % rel,
                ),
                (
                    "2026-10-03T11:00:00.000Z",
                    "",
                    "/srv/proj",
                    "unsent draft: refactor the parser\n[mention] file:///srv/proj/main.py",
                ),
                (
                    mtime,
                    "",
                    "",
                    "unsent draft: an orphan draft (from %s agent_draft_prompts, file mtime)" % rel,
                ),
                (
                    "2026-10-03T11:00:00.500Z",
                    "",
                    ZED_WORKTREE,
                    "scratch directory: %s (Zed-created git worktree of /srv/proj) (from %s"
                    " created_git_worktrees, created_at)" % (ZED_WORKTREE, rel),
                ),
                (
                    mtime,
                    ZED_WORKTREE_THREAD,
                    "/srv/proj",
                    "archived worktree: %s of /srv/proj branch=fix-tests staged=a1b2c3"
                    " unstaged=d4e5f6 original=0a0b0c ref=refs/archived-worktrees/1 (from %s"
                    " archived_git_worktrees, file mtime)" % (ZED_WORKTREE, rel),
                ),
                (
                    "2026-10-03T10:30:00.000Z",
                    "",
                    "/srv/proj",
                    "sidebar terminal: zsh cwd=/srv/proj",
                ),
            ],
        )
        for row in rows:
            self.assertEqual(row.turn_type, "system")
        self.assertFalse(ZedParser().secondary(art))

    def test_bad_kv_value_is_reported_not_fatal(self):
        # A scoped_kv_store value that does not decode is its own parser:
        # row, carried in full, after the file's other rows.
        connection = sqlite3.connect(str(self.home / self.SIDEBAR))
        connection.execute(
            "INSERT INTO scoped_kv_store(namespace,key,value) VALUES (?,?,?)",
            ("agent_draft_prompts", "cut-draft", '[{"type":"text","text":"cut'),
        )
        connection.execute(
            "INSERT INTO scoped_kv_store(namespace,key,value) VALUES (?,?,?)",
            ("created_git_worktrees", "local\n/srv/wt2", '{"created_at_nanos":1}'),
        )
        connection.commit()
        connection.close()
        rows = self.rows_for(ZedParser(), self.SIDEBAR)
        self.assertEqual(len(rows), 12)
        self.assertTrue(rows[9].text.startswith("sidebar terminal: "))
        self.assertEqual(
            [(row.turn_type, row.source_line, row.timestamp_utc, row.text) for row in rows[10:]],
            [
                (
                    "system",
                    5,
                    "",
                    "parser: unparseable record 5 (agent_draft_prompts cut-draft: JSONDecodeError:"
                    " Unterminated string starting at: line 1 column 24 (char 23)):"
                    ' [{"type":"text","text":"cut',
                ),
                (
                    "system",
                    6,
                    "",
                    "parser: unparseable record 6 (created_git_worktrees: KeyError:"
                    " 'created_at_seconds'):"
                    ' {"created_at_nanos":1}',
                ),
            ],
        )

    def test_settings_rows(self):
        # Source-derived (zed-industries/zed at 9cd14da): standing
        # permission entries of the user settings file, JSONC.
        rel = ".config/zed/settings.json"
        art = self.artifact(rel)
        rows = list(ZedParser().parse(art, Options()))
        tail = " (from %s, file mtime)" % rel
        self.assertEqual(
            [row.text for row in rows],
            [
                "permission setting: agent.tool_permissions.default=confirm" + tail,
                "permission setting: agent.tool_permissions.tools.terminal.always_allow="
                '{"pattern":"^cargo test"}' + tail,
                "permission setting: agent.tool_permissions.tools.terminal.always_deny="
                '{"case_sensitive":false,"pattern":"^rm -rf"}' + tail,
                "permission setting: agent.sandbox_permissions.network_hosts=*.npmjs.org" + tail,
                "permission setting: agent.sandbox_permissions.allow_unsandboxed=false" + tail,
                "permission setting: agent_servers.claude-acp.default_mode=plan" + tail,
                "permission setting: session.trust_all_worktrees=false" + tail,
                "permission setting: stable.agent.tool_permissions.default=allow" + tail,
            ],
        )
        for row in rows:
            self.assertEqual(
                (row.turn_type, row.session_id, row.project_path, row.timestamp_utc),
                ("system", "", "", file_mtime(art)),
            )

    def test_settings_not_object_and_empty(self):
        rel = ".config/zed/settings.json"
        (self.home / rel).write_text('{"agent": ', encoding="utf-8")
        self.assertEqual(
            [row.text for row in self.rows_for(ZedParser(), rel)],
            ["parser: %s is not a JSON object" % rel],
        )
        (self.home / rel).write_text("\n", encoding="utf-8")
        self.assertEqual(self.rows_for(ZedParser(), rel), [])

    def test_permission_decisions(self):
        rows = self.rows_for(ZedParser(), self.REL)
        i = next(
            index for index, row in enumerate(rows) if row.text == "permission decision: denied"
        )
        self.assertEqual(
            (rows[i - 1].turn_type, rows[i - 1].tool_use_id), ("tool_result", "toolu_02")
        )
        self.assertEqual(
            (rows[i].turn_type, rows[i].tool_name, rows[i].tool_use_id, rows[i].session_id),
            ("system", "terminal", "toolu_02", ZED_THREAD),
        )
        # the approved call has no decision row
        self.assertEqual(
            [row.tool_use_id for row in rows if row.text.startswith("permission decision")],
            ["toolu_02"],
        )
        results = {
            "t1": "Blocked by global default: deny",
            "t2": "Command blocked by security rule for terminal tool",
            "t3": "fetch tool is disabled",
            "t4": "No such file or directory",
        }
        thread = {
            "version": "0.3.0",
            "messages": [
                {
                    "Agent": {
                        "content": [],
                        "tool_results": {
                            k: {
                                "tool_use_id": k,
                                "tool_name": "terminal",
                                "is_error": True,
                                "content": [{"Text": value}],
                            }
                            for k, value in results.items()
                        },
                    }
                }
            ],
        }
        self._insert("auto", "json", json.dumps(thread).encode("utf-8"))
        rows = [row for row in self.rows_for(ZedParser(), self.REL) if row.session_id == "auto"]
        self.assertEqual(
            [(row.tool_use_id, row.text) for row in rows if row.turn_type == "system"][1:],
            [
                ("t1", "permission decision: auto_denied reason=Blocked by global default: deny"),
                (
                    "t2",
                    "permission decision: auto_denied reason=Command blocked by security rule"
                    " for terminal tool",
                ),
                ("t3", "permission decision: auto_denied reason=fetch tool is disabled"),
            ],
        )

    def test_git_remote_is_the_projects_own(self):
        # A multi-folder thread whose project (folder_paths /srv/other) has
        # no remote while another folder does: no `git remote:` row, never
        # the other repository's URL.
        thread = zed_thread_record("/srv/other")
        snaps = thread["initial_project_snapshot"]["worktree_snapshots"]
        snaps[0]["git_state"]["remote_url"] = None
        snaps.append(
            {
                "worktree_path": "/srv/proj",
                "git_state": {
                    "remote_url": PROJECT_REMOTE,
                    "head_sha": "d4",
                    "current_branch": "x",
                },
            }
        )
        self._insert("multi", "json", json.dumps(thread).encode("utf-8"))
        rows = [row for row in self.rows_for(ZedParser(), self.REL) if row.session_id == "multi"]
        self.assertTrue(rows)
        self.assertEqual({row.project_path for row in rows}, {"/srv/other"})
        self.assertEqual([row.text for row in rows if row.text.startswith("git remote:")], [])

    def test_scratch_rows(self):
        rows = [
            row
            for row in self.rows_for(ZedParser(), self.REL)
            if row.text.startswith("scratch directory:")
        ]
        self.assertEqual(
            [(row.session_id, row.project_path, row.timestamp_utc, row.text) for row in rows],
            [
                (
                    "",
                    ZED_TEMP_DIR,
                    "2026-10-03T09:01:05.000Z",
                    "scratch directory: %s (sandboxed terminal temp directory of thread %s)"
                    " (thread blob, as of updated_at)" % (ZED_TEMP_DIR, ZED_THREAD),
                )
            ],
        )

    def test_wanted_files(self):
        art = next(
            artifact for artifact in self.col.artifacts if artifact.rel == ".config/zed/keymap.json"
        )
        self.assertFalse(ZedParser().wants(art))
        for rel in (
            "AppData/Local/Zed/threads/threads.db",
            "Library/Application Support/Zed/db/0-preview/db.sqlite",
            ".var/app/dev.zed.Zed/data/zed/threads/threads.db",
            ".config/zed/settings.json",
            ".config/zed/global_settings.json",
            "AppData/Roaming/Zed/settings.json",
            ".var/app/dev.zed.ZedPreview/config/zed/settings.json",
        ):
            self.assertTrue(ZedParser().wants(rel_only(rel)), rel)
        for rel in (
            self.REL + "-journal",
            self.SIDEBAR + "-wal",
            ".config/zed/settings_backup.json",
            ".zed/settings.json",
        ):
            self.assertFalse(ZedParser().wants(rel_only(rel)), rel)

    def test_corrupt_blob_is_reported_not_fatal(self):
        blob = zstandard.ZstdCompressor().compress(b'{"version":"0.3.0","messages":[]}' * 50)
        cut = blob[: len(blob) // 2]
        self._insert("bad-thread", "zstd", cut)
        rows = self.rows_for(ZedParser(), self.REL)
        self.assertEqual(len([row for row in rows if row.session_id == ZED_THREAD]), 18)
        bad = [row for row in rows if row.session_id == "bad-thread"]
        self.assertEqual(len(bad), 1)
        self.assertEqual(
            (bad[0].turn_type, bad[0].project_path, bad[0].source_line, bad[0].timestamp_utc),
            ("system", "/srv/other", 2, ""),
        )
        # a full-record parser: row, the stored blob as hex
        self.assertEqual(
            bad[0].text,
            "parser: unparseable record 2 (JSONDecodeError: Expecting value: line 1 column 1"
            " (char 0); stored zstd blob as hex): " + cut.hex(),
        )

    def test_json_and_legacy_versions(self):
        legacy = {
            "version": "0.2.0",
            "summary": "old",
            "updated_at": "2026-01-01T00:00:00Z",
            "messages": [
                {
                    "id": 0,
                    "role": "user",
                    "segments": [{"type": "text", "text": "hello"}],
                    "tool_uses": [],
                    "tool_results": [],
                },
                {
                    "id": 1,
                    "role": "assistant",
                    "segments": [
                        {"type": "thinking", "text": "hmm"},
                        {"type": "text", "text": "hi"},
                    ],
                    "tool_uses": [{"id": "t1", "name": "grep", "input": {"regex": "x"}}],
                    "tool_results": [
                        {"tool_use_id": "t1", "is_error": True, "content": "no match"}
                    ],
                },
            ],
        }
        self._insert("legacy-thread", "json", json.dumps(legacy).encode("utf-8"))
        rows = [
            row for row in self.rows_for(ZedParser(), self.REL) if row.session_id == "legacy-thread"
        ]
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [
                ("system", "thread start: extra version=0.2.0"),
                ("user", "hello"),
                ("assistant", "hi"),
                ("tool_use", '{"regex":"x"}'),
                ("tool_result", "[error] no match"),
            ],
        )
        self.assertEqual(rows[1].timestamp_utc, "2026-10-03T12:00:00.000Z")

    def test_model_and_thread_grant_rows(self):
        rows = self.rows_for(ZedParser(), self.REL)
        state = [
            row
            for row in rows
            if model_or_permission_row(row) and not row.text.startswith("permission decision")
        ]
        self.assertEqual(
            [row.text for row in state],
            [
                "model: claude-sonnet-4-5 provider=anthropic",
                "permission setting: profile=write (thread blob, as of updated_at)",
                "permission setting: sandbox_grants.write_paths=/srv/proj/build"
                " (thread blob, as of updated_at)",
                'permission setting: sandbox_grants.write_paths={"requested":"/tmp/out",'
                '"resolved":"/private/tmp/out"} (thread blob, as of updated_at)',
                "permission setting: sandbox_grants.network_hosts=*.npmjs.org"
                " (thread blob, as of updated_at)",
                "permission setting: sandbox_grants.unsandboxed=true"
                " (thread blob, as of updated_at)",
            ],
        )
        # the model row follows the thread-start and git remote rows at
        # their time; the grants carry the thread id and the thread's
        # updated_at, and come before the first message
        self.assertTrue(rows[1].text.startswith("git remote: "), rows[1].text)
        self.assertEqual(rows[2], state[0])
        self.assertEqual(state[0].timestamp_utc, "2026-10-03T09:00:00.000Z")
        for row in state:
            self.assertEqual(
                (row.turn_type, row.session_id, row.project_path, row.source_line),
                ("system", ZED_THREAD, "/srv/proj", 1),
            )
            self.assertEqual(row.model, "anthropic/claude-sonnet-4-5")
        for row in state[1:]:
            self.assertEqual(row.timestamp_utc, "2026-10-03T09:01:05.000Z")
        # then the sandbox temp directory row, then the first message
        self.assertTrue(rows[len(state) + 2].text.startswith("scratch directory:"))
        self.assertEqual(rows[len(state) + 3].turn_type, "user")

    def test_thread_without_model_or_grants_has_no_state_rows(self):
        self._insert("bare", "json", b'{"version":"0.3.0","messages":[]}')
        rows = [row for row in self.rows_for(ZedParser(), self.REL) if row.session_id == "bare"]
        self.assertEqual([row.text for row in rows], ["thread start: extra version=0.3.0"])

    def test_not_sqlite_is_one_parser_row(self):
        # a threads.db or db.sqlite that is not SQLite yields one
        # whole-file row instead of nothing
        for rel in (self.REL, self.SIDEBAR):
            (self.home / rel).write_bytes(b"not a database\n")
            rows = self.rows_for(ZedParser(), rel)
            self.assertEqual(
                [(row.turn_type, row.text, row.session_id) for row in rows],
                [("system", "parser: not a SQLite database", "")],
                rel,
            )

    def test_bad_json_blob_is_carried_in_full(self):
        # a `json` row that is not a JSON object, and one cut
        # mid-record, each carry their stored text; the fixture thread
        # before them still yields every row
        self._insert("list-thread", "json", b'["not", "an", "object"]')
        self._insert("cut-thread", "json", b'{"version":"0.3.0","messages":[{"User"')
        rows = self.rows_for(ZedParser(), self.REL)
        self.assertEqual(len([row for row in rows if row.session_id == ZED_THREAD]), 18)
        by_session = {row.session_id: row for row in rows if row.session_id != ZED_THREAD}
        self.assertEqual(
            by_session["list-thread"].text,
            'parser: unparseable record 2 (ValueError: not a JSON object): ["not", "an", "object"]',
        )
        self.assertEqual(
            by_session["cut-thread"].text,
            "parser: unparseable record 3 (JSONDecodeError: Expecting ':' delimiter: line 1"
            ' column 39 (char 38)): {"version":"0.3.0","messages":[{"User"',
        )
        for session_id in ("list-thread", "cut-thread"):
            row = by_session[session_id]
            self.assertEqual(
                (row.turn_type, row.project_path, row.timestamp_utc), ("system", "/srv/other", "")
            )

    def test_unknown_variants_are_full_record_rows(self):
        from fixtures import zed_unknown_variants_record

        self._insert(
            "odd-thread", "json", json.dumps(zed_unknown_variants_record()).encode("utf-8")
        )
        rows = [
            row for row in self.rows_for(ZedParser(), self.REL) if row.session_id == "odd-thread"
        ]
        self.assertEqual(
            [(row.turn_type, row.text) for row in rows],
            [
                ("system", "thread start: Unknown variants version=0.3.0"),
                ("user", "before the unknown"),
                ("assistant", "agent text"),
                ("user", "after the unknown"),
                (
                    "system",
                    "parser: unparseable record 2 (unknown message variant):"
                    ' {"Checkpoint":{"id":"c1","label":"naïve"}}',
                ),
                (
                    "system",
                    "parser: unparseable record 3 (unknown Agent content variant, content[1]):"
                    ' {"Citation":{"url":"https://example.com"}}',
                ),
                (
                    "system",
                    "parser: unparseable record 3 (unknown Agent content variant, content[2]): 42",
                ),
                (
                    "system",
                    "parser: unparseable record 3 (Agent tool_results entry toolu_x is not an"
                    ' object): "not an object"',
                ),
                ("system", "parser: unparseable record 4 (unknown message variant): 7"),
                (
                    "system",
                    "parser: unparseable record 5 (unknown Compaction variant):"
                    ' {"Compaction":{"Remote":{"id":"r1"}}}',
                ),
                (
                    "system",
                    "parser: unparseable record 6 (unknown segment variant, segments[0]):"
                    ' {"type":"audio","data":"AAA="}',
                ),
            ],
        )
        for row in rows:
            self.assertEqual((row.project_path, row.source_line), ("/srv/other", 2))
        for row in rows[4:]:
            self.assertEqual(row.timestamp_utc, "")
