"""Tests of input detection: loose trees, disk images, collections and archives."""

import hashlib
import io
import json
import os
import shutil
import subprocess
import tarfile
import tempfile
import unittest
import zipfile
from pathlib import Path

from doubleagent import catalog, cli, inputs, model
from doubleagent.errors import InputError
from doubleagent.inputs import open_input
from fixtures import (
    PROJECT_GIT_CONFIG,
    PROJECT_GIT_HEAD,
    PROJECT_GIT_REMOTES,
    PROJECT_REMOTE,
    build_home,
    build_image,
)

REPO = Path(__file__).resolve().parents[2]
COLLECTOR = REPO / "collectors" / "collect-agent-artifacts.sh"


class LooseDetectTests(unittest.TestCase):
    """Detection in a loose tree or disk image, without a manifest."""

    def setUp(self):
        self.cat = catalog.load()
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-analyzer-test-"))

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def agents_by_user(self, col):
        out = {}
        for artifact in col.artifacts:
            out.setdefault(artifact.home.user, set()).add(artifact.agent)
        return out

    def test_image_layout_finds_every_home(self):
        root = build_image(self.tmp / "image")
        col = open_input(root, self.cat)
        self.assertEqual(col.kind, "loose")
        by_user = self.agents_by_user(col)
        self.assertEqual(set(by_user), {"alice", "bob", "carol"})
        expected_alice = {
            "claude-code",
            "codex-cli",
            "gemini-cli",
            "antigravity",
            "vscode",
            "cline",
            "qwen-code",
            "kiro",
            "crush",
            "goose",
            "continue",
            "aider",
            "zed",
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
            "copilot-cli",
            "copilot-desktop",
        }
        self.assertLessEqual(expected_alice, by_user["alice"])
        self.assertEqual(by_user["carol"], {"codex-cli"})
        homes = {home.user: home for home in col.homes}
        self.assertEqual(homes["alice"].original, "/home/alice")
        self.assertEqual(homes["carol"].original, "/Users/carol")
        self.assertTrue(all(home.inferred for home in col.homes))

    def test_nested_entry_wins_over_enclosing_entry(self):
        home = build_home(self.tmp / "home" / "alice")
        for root in (self.tmp / "home", home / ".gemini"):
            col = open_input(root, self.cat)
            by_rel = {artifact.rel: artifact.agent for artifact in col.artifacts}
            self.assertEqual(by_rel[".gemini/antigravity-cli/history.jsonl"], "antigravity", root)
            self.assertEqual(by_rel[".gemini/settings.json"], "gemini-cli", root)
            self.assertEqual(
                sum(
                    1
                    for artifact in col.artifacts
                    if artifact.rel == ".gemini/antigravity-cli/history.jsonl"
                ),
                1,
            )
        col = open_input(self.tmp / "home", self.cat)
        by_rel = {artifact.rel: artifact.agent for artifact in col.artifacts}
        self.assertEqual(
            by_rel[".config/Code/User/globalStorage/saoudrizwan.claude-dev/state/taskHistory.json"],
            "cline",
        )
        self.assertEqual(by_rel[".config/Code/User/globalStorage/state.vscdb"], "vscode")

    def test_copilot_desktop_nested_in_copilot_cli(self):
        # Issue 147: the app's store is a nested file entry inside the CLI's
        # .copilot; the app's sessions stay with the CLI.
        home = self.tmp / "home" / "alice"
        events = ".copilot/session-state/5d404e14-0000-4000-8000-000000000001/events.jsonl"
        files = {
            ".copilot/data.db": "copilot-desktop",
            ".copilot/data.db-wal": "copilot-desktop",
            ".copilot/run/ws.token": "copilot-desktop",
            ".copilot/chats/2026-10-16/x-5d404e14/.keep": "copilot-desktop",
            ".copilot/logs/github-app.1.log": "copilot-desktop",
            ".local/share/com.github.githubapp/localstorage/tauri_localhost_0.localstorage": (
                "copilot-desktop"
            ),
            ".github-copilot-cli/logs/foundry.core20261016.log": "copilot-desktop",
            events: "copilot-cli",
            ".copilot/logs/process-1-2.log": "copilot-cli",
            ".copilot/session-store.db": "copilot-cli",
        }
        for rel in files:
            path = home / rel
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("x", encoding="utf-8")
        for root in (self.tmp / "home", home / ".copilot"):
            col = open_input(root, self.cat)
            by_rel = {artifact.rel: artifact.agent for artifact in col.artifacts}
            for rel, agent in files.items():
                if root == home / ".copilot" and not rel.startswith(".copilot/"):
                    continue
                self.assertEqual(by_rel.get(rel), agent, (root, rel))
            self.assertEqual(
                sum(1 for artifact in col.artifacts if artifact.rel == ".copilot/data.db"), 1
            )

    def test_copilot_desktop_manifest_agent_kept(self):
        # With a manifest the collector's attribution stands: a 1.13.0
        # collection made before the copilot-desktop entry attributed the
        # store to copilot-cli, and the analyzer does not re-attribute it.
        col_dir = self.tmp / "col"
        rows = []
        for rel, agent in (
            (".copilot/data.db", "copilot-desktop"),
            (".copilot/session-state/s1/events.jsonl", "copilot-cli"),
        ):
            path = "/home/alice/" + rel
            disk = col_dir / ("fs" + path)
            disk.parent.mkdir(parents=True, exist_ok=True)
            disk.write_text("x", encoding="utf-8")
            rows.append(
                {
                    "path": path,
                    "home": "/home/alice",
                    "user": "alice",
                    "agent": agent,
                    "type": "file",
                    "status": "collected",
                    "size": 1,
                    "archive_path": "fs" + path,
                }
            )
        (col_dir / "collection.json").write_text('{"hostname": "h1"}', encoding="utf-8")

        def detect(store_agent):
            rows[0]["agent"] = store_agent
            with open(col_dir / "manifest.jsonl", "w", encoding="utf-8") as handle:
                for row in rows:
                    handle.write(json.dumps(row) + "\n")
            col = open_input(col_dir, self.cat)
            self.assertEqual(col.kind, "collected")
            return {artifact.rel: artifact.agent for artifact in col.artifacts}

        for store_agent in ("copilot-desktop", "copilot-cli"):
            by_rel = detect(store_agent)
            self.assertEqual(by_rel[".copilot/data.db"], store_agent)
            self.assertEqual(by_rel[".copilot/session-state/s1/events.jsonl"], "copilot-cli")

    def test_framework_agents_detected(self):
        # CrewAI's run database sits in a data
        # directory named after the project folder (a mid-path `*` entry),
        # its Windows directory is the class [Cc]rew[Aa][Ii], CAMEL shares
        # ~/.camel with Apache Camel JBang, and the legacy Open Interpreter
        # directories have spaces in their names. The two Windows spellings
        # sit in separate homes, because a case-insensitive filesystem
        # (macOS, Windows) would fold them into one directory.
        home = self.tmp / "home" / "alice"
        files = {
            ".local/share/myproj/latest_kickoff_task_outputs.db": "crewai",
            ".local/share/opencode/flow_states.db": "crewai",
            ".local/share/opencode/opencode.db": "opencode",
            "AppData/Local/CrewAI/myproj/latest_kickoff_task_outputs.db": "crewai",
            ".camel/gmail_token.json": "camel-ai",
            ".config/Open Interpreter Terminal/conversations/a.json": "open-interpreter",
            ".config/pydantic-clai2/sessions.db": "pydantic-clai",
            ".n8n/database.sqlite": "n8n",
            ".flowise/database.sqlite": "flowise",
            ".autogenstudio/autogen04203.db": "autogen-studio",
            ".metagpt/config2.yaml": "metagpt",
            ".cache/langflow/secret_key": "langflow",
            ".config/difyctl/config.yml": "dify",
        }
        for rel in [*files, ".camel/4242-status.json", ".local/share/myproj/notes.txt"]:
            path = home / rel
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("x")
        lower = self.tmp / "home" / "bob" / "AppData/Local/crewai/credentials/tokens.enc"
        lower.parent.mkdir(parents=True)
        lower.write_text("x")
        col = open_input(self.tmp / "home", self.cat)
        by_rel = {artifact.rel: artifact.agent for artifact in col.artifacts}
        for rel, agent in files.items():
            self.assertEqual(by_rel.get(rel), agent, rel)
        self.assertEqual(by_rel.get("AppData/Local/crewai/credentials/tokens.enc"), "crewai")
        self.assertNotIn(".camel/4242-status.json", by_rel)
        self.assertNotIn(".local/share/myproj/notes.txt", by_rel)
        self.assertEqual(
            sum(
                1
                for artifact in col.artifacts
                if artifact.rel == ".local/share/opencode/flow_states.db"
            ),
            1,
        )

    def test_claude_code_cli_cache_detected(self):
        # The CLI's MCP server logs in its env-paths
        # cache directory; the Windows entry names the children of Cache/.
        home = self.tmp / "home" / "alice"
        line = {"cwd": "", "debug": "MCP server: connected", "sessionId": "s", "timestamp": "t"}
        files = [
            "Library/Caches/claude-cli-nodejs/-Users-alice-proj/mcp-logs-terminal/"
            "2026-10-12T02-23-51-709Z.jsonl",
            "AppData/Local/claude-cli-nodejs/Cache/C--Users-alice-proj/mcp-logs-ccd-host/"
            "2026-10-12T02-23-51-709Z.jsonl",
        ]
        for rel in files:
            path = home / rel
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(json.dumps(line) + "\n")
        col = open_input(self.tmp / "home", self.cat)
        by_rel = {artifact.rel: artifact.agent for artifact in col.artifacts}
        for rel in files:
            self.assertEqual(by_rel.get(rel), "claude-code", rel)

    def test_project_dirs_inside_home_are_not_homes(self):
        root = build_image(self.tmp / "image")
        col = open_input(root, self.cat)
        self.assertFalse(any("dev/proj" in (home.original or "") for home in col.homes))
        # The .claude inside the project belongs to nobody at home level.
        self.assertFalse(any("dev/proj/.claude" in artifact.original for artifact in col.artifacts))

    def test_shared_only_directory_is_not_a_home(self):
        repo = self.tmp / "repo"
        repo.mkdir()
        (repo / "AGENTS.md").write_text("x", encoding="utf-8")
        (repo / ".env").write_text("A=1", encoding="utf-8")
        col = open_input(repo, self.cat)
        self.assertEqual(col.homes, [])
        self.assertEqual(col.artifacts, [])

    def test_bare_home_directory(self):
        home = build_home(self.tmp / "alice")
        col = open_input(home, self.cat)
        self.assertEqual(len(col.homes), 1)
        # The root itself gives no clue who owns it; --user fills this in.
        self.assertEqual(col.homes[0].user, "")
        self.assertEqual(col.homes[0].original, "")
        rels = {artifact.rel for artifact in col.artifacts if artifact.agent == "claude-code"}
        self.assertIn(".claude/history.jsonl", rels)
        self.assertIn(".claude/settings.json", rels)
        # A config home moved with CLAUDE_CONFIG_DIR; not claude-code-router.
        self.assertIn(".claude-work/history.jsonl", rels)
        self.assertFalse(
            any(artifact.rel.startswith(".claude-code-router/") for artifact in col.artifacts)
        )

    def test_agent_directory_as_root(self):
        home = build_home(self.tmp / "home" / "alice")
        col = open_input(home / ".claude", self.cat)
        self.assertEqual(len(col.homes), 1)
        self.assertEqual(col.homes[0].user, "alice")
        self.assertEqual({artifact.agent for artifact in col.artifacts}, {"claude-code"})
        self.assertTrue(any("claude-code directory" in note for note in col.notes))
        # .claude/backups-style nested matches must not create extra homes
        self.assertTrue(all(artifact.rel.startswith(".claude/") for artifact in col.artifacts))

    def test_symlinked_directories_are_not_followed(self):
        home = build_home(self.tmp / "home" / "alice")
        outside = self.tmp / "outside"
        outside.mkdir()
        (outside / "secret.jsonl").write_text("{}", encoding="utf-8")
        try:
            os.symlink(outside, home / ".claude" / "linked")
        except OSError:
            self.skipTest("symlinks not permitted")
        col = open_input(self.tmp / "home", self.cat)
        self.assertFalse(any("linked" in artifact.rel for artifact in col.artifacts))

    def test_fs_without_manifest(self):
        root = self.tmp / "extracted"
        build_image(root / "fs")
        col = open_input(root, self.cat)
        self.assertTrue(any("fs/ found without manifest" in note for note in col.notes))
        self.assertEqual({home.user for home in col.homes}, {"alice", "bob", "carol"})

    def test_cli_detect_json_and_user_override(self):
        home = build_home(self.tmp / "x")
        import contextlib
        import io

        buffer = io.StringIO()
        with contextlib.redirect_stdout(buffer):
            exit_code = cli.main(
                ["detect", str(home / ".codex"), "--json", "--user", "mallory", "--host", "h1"]
            )
        self.assertEqual(exit_code, 0)
        out = json.loads(buffer.getvalue())
        self.assertEqual(out["kind"], "loose")
        self.assertEqual(out["host"], "h1")
        self.assertEqual(out["homes"][0]["user"], "mallory")
        self.assertEqual(out["homes"][0]["agents"][0]["agent"], "codex-cli")
        self.assertTrue(out["homes"][0]["agents"][0]["parser"])
        # Loose input marks no home as a project.
        self.assertEqual(out["projects"], [])

    def test_missing_input(self):
        self.assertEqual(cli.main(["detect", str(self.tmp / "nope")]), 2)


class GitDetectTests(unittest.TestCase):
    """`remotes` and `head` of each project in `detect --json`."""

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-gitdetect-test-"))

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def collection(self) -> Path:
        """An extracted collection with two collected projects.

        /srv/proj with its repository files, /srv/plain with only AGENTS.md.
        """
        root = self.tmp / "col"
        files = [
            ("/srv/proj", ".git/config", PROJECT_GIT_CONFIG),
            ("/srv/proj", ".git/HEAD", PROJECT_GIT_HEAD),
            ("/srv/plain", "AGENTS.md", "# x\n"),
            ("/home/alice", ".codex/history.jsonl", ""),
        ]
        rows = []
        for base, rel, text in files:
            path = base + "/" + rel
            disk = root / ("fs" + path)
            disk.parent.mkdir(parents=True, exist_ok=True)
            disk.write_text(text, encoding="utf-8")
            agent = "project" if base.startswith("/srv/") else "codex-cli"
            rows.append(
                {
                    "user": "alice",
                    "home": base,
                    "agent": agent,
                    "path": path,
                    "archive_path": "fs" + path,
                    "type": "file",
                    "status": "collected",
                    "size": len(text),
                }
            )
        (root / "manifest.jsonl").write_text(
            "".join(json.dumps(record) + "\n" for record in rows), encoding="utf-8"
        )
        summary = {"hostname": "h1", "projects": ["/srv/plain", "/srv/proj"]}
        (root / "collection.json").write_text(json.dumps(summary), encoding="utf-8")
        return root

    def test_remotes_and_head(self):
        import contextlib
        import io

        buffer = io.StringIO()
        with contextlib.redirect_stdout(buffer):
            exit_code = cli.main(["detect", str(self.collection()), "--json"])
        self.assertEqual(exit_code, 0, buffer.getvalue())
        projects = {
            project["path"]: project for project in json.loads(buffer.getvalue())["projects"]
        }
        self.assertEqual(sorted(projects), ["/srv/plain", "/srv/proj"])
        for project in projects.values():
            self.assertEqual(list(project)[-2:], ["remotes", "head"])
        self.assertEqual(projects["/srv/proj"]["remotes"], PROJECT_GIT_REMOTES)
        self.assertEqual(projects["/srv/proj"]["head"], "main")
        self.assertEqual(projects["/srv/plain"]["remotes"], [])
        self.assertEqual(projects["/srv/plain"]["head"], "")


class ArchiveTests(unittest.TestCase):
    """Round trip through the sh collector when a POSIX shell is available.

    Collector archive in, attributed artifacts and timeline out.
    """

    def setUp(self):
        self.cat = catalog.load()
        self.tmp = Path(tempfile.mkdtemp(prefix="cac-analyzer-test-"))

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def _collect(self):
        if not COLLECTOR.exists() or shutil.which("sh") is None or shutil.which("tar") is None:
            self.skipTest("collector or sh/tar not available")
        root = build_image(self.tmp / "image")
        out = self.tmp / "out"
        out.mkdir()
        result = subprocess.run(
            ["sh", str(COLLECTOR), "-r", str(root), "-o", str(out), "-q"],
            capture_output=True,
            text=True,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        archives = list(out.glob("*.tar.gz"))
        self.assertEqual(len(archives), 1, list(out.iterdir()))
        return archives[0], out

    def test_archive_attribution_comes_from_manifest(self):
        archive, _out = self._collect()
        col = open_input(archive, self.cat)
        try:
            self.assertEqual(col.kind, "archive")
            self.assertTrue(col.host)
            self.assertTrue(any("sha256 verified" in note for note in col.notes), col.notes)
            users = {
                artifact.home.user
                for artifact in col.artifacts
                if artifact.agent in ("claude-code", "codex-cli")
            }
            self.assertEqual(users, {"alice", "bob", "carol"})
            self.assertFalse(any(home.inferred for home in col.homes))
            # every attributed file exists on disk after extraction
            for artifact in col.artifacts:
                self.assertTrue(artifact.disk_path.is_file(), artifact.disk_path)
        finally:
            col.cleanup()
        self.assertFalse(Path(col.root).exists())

    def test_extracted_directory_and_timeline_cli(self):
        archive, _out = self._collect()
        extracted = self.tmp / "extracted"
        extracted.mkdir()
        with tarfile.open(archive) as archive_file:
            if hasattr(tarfile, "data_filter"):
                archive_file.extractall(extracted, filter="data")
            else:
                archive_file.extractall(extracted)
        col = open_input(extracted, self.cat)
        self.assertEqual(col.kind, "collected")
        import contextlib
        import io

        buffer = io.StringIO()
        with contextlib.redirect_stdout(buffer):
            exit_code = cli.main(["timeline", str(extracted), "-o", str(self.tmp / "tl")])
        self.assertEqual(exit_code, 0, buffer.getvalue())
        lines = (self.tmp / "tl" / "timeline.jsonl").read_text(encoding="utf-8").split("\n")
        self.assertEqual(lines[-1], "")
        rows = [json.loads(line) for line in lines[:-1]]
        self.assertGreater(len(rows), 20)
        self.assertEqual(list(rows[0]), model.TIMELINE_COLUMNS)
        self.assertIsInstance(rows[0]["source_line"], int)
        sessions = (self.tmp / "tl" / "sessions.jsonl").read_text(encoding="utf-8").splitlines()
        self.assertEqual(list(json.loads(sessions[0])), model.SESSION_COLUMNS)
        self.assertTrue((self.tmp / "tl" / "inventory.json").exists())
        # No detect.json: inventory.json holds what it held.
        self.assertFalse((self.tmp / "tl" / "detect.json").exists())
        # host comes from collection.json, users from the manifest
        self.assertLessEqual({row["user"] for row in rows}, {"alice", "bob", "carol"})
        # The collector gathers /srv/proj's repository files, and the
        # agents' own records of its remote join them.
        inv = json.loads((self.tmp / "tl" / "inventory.json").read_text(encoding="utf-8"))
        project = inv["projects"]["/srv/proj"]
        self.assertEqual(project["status"], "collected")
        self.assertEqual(
            (project["remotes"], project["head"], project["recorded_remotes"]),
            (PROJECT_GIT_REMOTES, "main", [PROJECT_REMOTE]),
        )


def test_manifest_lines_take_json_whitespace_only(tmp_path):
    root = tmp_path / "collection"
    (root / "fs").mkdir(parents=True)
    record = json.dumps({"path": "/x", "status": "collected", "type": "file"})
    for space in map(chr, (0x00A0, 0x2028, 0x0085)):
        (root / "manifest.jsonl").write_text(
            " %s\t\r\n%s%s\n" % (record, space, record), encoding="utf-8", newline=""
        )
        collection = open_input(root, catalog.load())
        assert "1 manifest rows did not parse" in collection.notes, (repr(space), collection.notes)


def test_sidecar_hash_ends_at_ascii_whitespace(tmp_path):
    archive = tmp_path / "c.tar.gz"
    archive.write_bytes(b"archive")
    digest = hashlib.sha256(b"archive").hexdigest()
    sidecar = tmp_path / "c.tar.gz.sha256"
    sidecar.write_text("%s  c.tar.gz\r\n" % digest.upper(), encoding="utf-8")
    assert inputs._verify_sidecar(archive, sidecar).startswith("archive sha256 verified")
    for space in map(chr, (0x00A0, 0x2028, 0x0085)):
        sidecar.write_text("%s%sc.tar.gz\n" % (digest, space), encoding="utf-8")
        assert inputs._verify_sidecar(archive, sidecar).startswith("WARNING"), repr(space)


def _tar_with(path: Path, members: list[tuple[tarfile.TarInfo, bytes]]) -> None:
    with tarfile.open(path, "w:gz") as archive:
        for info, data in members:
            info.size = len(data)
            archive.addfile(info, io.BytesIO(data) if data else None)


def test_extract_archive_writes_nothing_through_a_link(tmp_path):
    outside = tmp_path / "outside"
    outside.mkdir()
    link = tarfile.TarInfo("./fs/home/user/.config/x")
    link.type = tarfile.SYMTYPE
    link.linkname = str(outside)
    payload = tarfile.TarInfo("./fs/home/user/.config/x/authorized_keys")
    archive = tmp_path / "c.tar.gz"
    _tar_with(archive, [(link, b""), (payload, b"ssh-ed25519 planted")])
    dest = tmp_path / "dest"
    dest.mkdir()
    inputs.extract_archive(archive, dest)
    assert list(outside.iterdir()) == []
    written = dest / "fs" / "home" / "user" / ".config" / "x"
    assert not written.is_symlink() and (written / "authorized_keys").is_file()


def test_extract_archive_keeps_escaping_members_out(tmp_path):
    # The data filter refuses `..` and strips an absolute path's leading `/`; the older
    # fallback drops both. Either way nothing lands outside the destination.
    cases = [
        ("../escape.txt", tmp_path / "escape.txt"),
        ("/abs-escape.txt", Path("/abs-escape.txt")),
    ]
    for number, (name, outside) in enumerate(cases):
        archive = tmp_path / "c.tar.gz"
        _tar_with(archive, [(tarfile.TarInfo(name), b"x")])
        dest = tmp_path / ("dest-%d" % number)
        dest.mkdir()
        try:
            inputs.extract_archive(archive, dest)
        except InputError:
            pass
        assert not outside.exists(), name
        for path in dest.rglob("*"):
            assert path.resolve().is_relative_to(dest.resolve()), (name, path)


def test_extract_archive_drops_escaping_zip_members(tmp_path):
    archive = tmp_path / "c.zip"
    with zipfile.ZipFile(archive, "w") as zip_file:
        for name in ("../escape.txt", "/abs.txt", "C:/drive.txt", "fs/kept.txt"):
            zip_file.writestr(name, b"x")
    dest = tmp_path / "dest"
    dest.mkdir()
    inputs.extract_archive(archive, dest)
    assert sorted(path.relative_to(dest).as_posix() for path in dest.rglob("*")) == [
        "fs",
        "fs/kept.txt",
    ]
    assert not (tmp_path / "escape.txt").exists()


def test_extract_archive_refuses_other_files(tmp_path):
    other = tmp_path / "notes.txt"
    other.write_text("not an archive", encoding="utf-8")
    try:
        inputs.extract_archive(other, tmp_path)
    except InputError as error:
        assert "neither a tar nor a zip archive" in str(error)
    else:
        raise AssertionError("no InputError")


if __name__ == "__main__":
    unittest.main()
