"""Tests for lab/round/tools/shape.py, the value-free shape dumper, on synthetic files."""

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

TOOL = Path(__file__).resolve().parents[1] / "tools" / "shape.py"

UUID = "3f2b8c1e-9a4d-4e6f-8b7a-1c2d3e4f5a6b"
SECRET = "sk-ant-FAKE0000"
ABS = "/Users/alice/projects/secret-repo"
URL = "https://internal.example.com/callback?code=abc"
PROMPT = "please refactor the billing module"


def shape(*args):
    result = subprocess.run(
        [sys.executable, "-I", str(TOOL), *map(str, args)],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        raise AssertionError("shape.py exited %d: %s" % (result.returncode, result.stderr))
    # The `# <root>` header names the test's own temporary directory, whose random
    # name can contain a token a leak check looks for; it is not under test.
    return "".join(line for line in result.stdout.splitlines(True) if not line.startswith("# "))


class ShapeBase(unittest.TestCase):
    """A temporary root for the files a shape test writes."""

    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)


class JsonTests(ShapeBase):
    """The shape of a JSON file: keys and value types, never values."""

    def setUp(self):
        super().setUp()
        doc = {
            "token": SECRET,
            "account": {"id": UUID, "home": ABS, "created": "2026-10-01T12:00:00.000Z"},
            "endpoint": URL,
            "state": "idle",
            "count": 3,
            "enabled": True,
            "nothing": None,
            "items": [{"name": "first item", "size": 1}, {"name": "second", "size": 2}],
            "empty": [],
            "deep": {"a": {"b": {"c": 1}}},
            "auth": {"api_key": "sk-x", "cookie": "abc", "mode": "live"},
        }
        (self.root / "config.json").write_text(json.dumps(doc), encoding="utf-8")

    def test_keys_and_classes(self):
        out = shape(self.root / "config.json")
        for line in (
            "== config.json (",
            "  object",
            "    token: str(len 15)",
            "    account: object",
            "      id: str<uuid>",
            "      home: str<abs-path>",
            "      created: str<iso-date>",
            "    endpoint: str<url>",
            "    count: number",
            "    enabled: bool",
            "    nothing: null",
            "    items: [2 x object]",
            "      name: str(len 10)",
            "      size: number",
            "    empty: []",
        ):
            self.assertIn(line, out)

    def test_no_values(self):
        out = shape(self.root / "config.json")
        self.assertNotIn("sk-ant", out)
        self.assertNotIn(SECRET, out)
        self.assertNotIn(UUID, out)
        self.assertNotIn(ABS, out)
        self.assertNotIn("alice", out)
        self.assertNotIn(URL, out)
        self.assertNotIn("example.com", out)
        self.assertNotIn("first item", out)

    def test_depth(self):
        out = shape("--depth", "2", self.root / "config.json")
        self.assertIn("    deep: object", out)
        self.assertIn("      a: {...}", out)
        self.assertNotIn("b:", out)

    def test_enums(self):
        default = shape(self.root / "config.json")
        self.assertIn("    state: str(len 4)", default)
        self.assertNotIn("idle", default)
        enums = shape("--enums", self.root / "config.json")
        self.assertIn("    state: str=idle", enums)
        # The secret is too long and has an upper-case letter: never an enum.
        self.assertNotIn("sk-ant", enums)
        # Short values under credential-like keys keep the length form, even
        # when the key is an ancestor (`auth` guards `mode` too).
        self.assertIn("      api_key: str(len 4)", enums)
        self.assertIn("      cookie: str(len 3)", enums)
        self.assertIn("      mode: str(len 4)", enums)
        self.assertNotIn("sk-x", enums)
        self.assertNotIn("str=abc", enums)


class JsonlTests(ShapeBase):
    """The shape of a JSONL file: line types, key sets and enum tokens."""

    def setUp(self):
        super().setUp()
        lines = [
            {
                "type": "user",
                "uuid": UUID,
                "message": {"role": "user", "content": [{"type": "text", "text": PROMPT}]},
            },
            {
                "type": "assistant",
                "message": {
                    "role": "assistant",
                    "content": [
                        {"type": "text", "text": "ok"},
                        {"type": "tool_use", "name": "Bash", "input": {"command": "ls"}},
                    ],
                },
            },
            {"type": "system", "subtype": "compact_boundary", "content": PROMPT},
            {"type": "system", "subtype": "api_error", "error": {"status": 529}, "level": "x"},
        ]
        body = "".join(json.dumps(record) + "\n" for record in lines)
        body += '{"type": "user", "message": "cut mid-wri\n'
        (self.root / "s.jsonl").write_text(body, encoding="utf-8")

    def test_summary(self):
        out = shape(self.root / "s.jsonl")
        self.assertIn("== s.jsonl (", out)
        self.assertIn("jsonl, 5 lines)", out)
        self.assertRegex(out, r"  type=user: 1 \(\d+ bytes\)\n    keys: message, type, uuid\n")
        self.assertIn("    content blocks: text\n", out)
        self.assertRegex(out, r"  type=assistant: 1 \(\d+ bytes\)\n    keys: message, type\n")
        self.assertIn("    message.content[].name: Bash=1\n", out)
        self.assertIn("    content blocks: text, tool_use\n", out)
        self.assertRegex(
            out, r"  type=system: 2 \(\d+ bytes\)\n    keys: content, error, level, subtype, type\n"
        )
        self.assertIn("    subtype: api_error=1, compact_boundary=1\n", out)
        self.assertIn("    subtype=api_error: 1\n      keys: error, level, subtype, type\n", out)
        self.assertIn("    subtype=compact_boundary: 1\n      keys: content, subtype, type\n", out)
        self.assertIn("  bad lines 1\n", out)

    def test_no_content(self):
        out = shape(self.root / "s.jsonl")
        self.assertNotIn(PROMPT, out)
        self.assertNotIn("billing", out)
        self.assertNotIn("command", out)
        self.assertNotIn(UUID, out)
        self.assertNotIn("cut mid", out)

    def test_ids_and_paths_in_enum_values_redacted(self):
        # Values at the enum paths that fit the token shape but are identifiers
        # print as their class, never verbatim.
        hexid = "0123456789abcdef0123456789abcdef"
        lines = [
            {"type": "system", "subtype": UUID},
            {
                "type": "user",
                "sessionKind": hexid,
                "entrypoint": "https://example.test/cb",
                "message": {"role": "user", "model": "/Users/alice/private-model", "content": []},
            },
        ]
        body = "".join(json.dumps(record) + "\n" for record in lines)
        (self.root / "ids.jsonl").write_text(body, encoding="utf-8")
        out = shape(self.root / "ids.jsonl")
        self.assertIn("    subtype: <uuid>=1\n", out)
        self.assertIn("    sessionKind: <hex>=1\n", out)
        self.assertIn("    entrypoint: <url>=1\n", out)
        self.assertIn("    message.model: <abs-path>=1\n", out)
        for raw in (UUID, hexid, "example.test", "private-model", "alice"):
            self.assertNotIn(raw, out)

    def test_untyped_lines(self):
        (self.root / "h.jsonl").write_text(
            '{"display": "x", "ts": 1}\n{"ts": 2, "display": "y"}\n', encoding="utf-8"
        )
        out = shape(self.root / "h.jsonl")
        self.assertRegex(out, r"  keys\{display,ts\}: 2 \(\d+ bytes\)\n    keys: display, ts\n")


class RedactionTests(ShapeBase):
    """Ids, paths and e-mail addresses print as their class, never as themselves."""

    def test_uuid_key_and_path(self):
        directory = self.root / "agent" / UUID
        directory.mkdir(parents=True)
        hexname = "a" * 40
        doc = {
            "dxt:allowlistCache:%s" % UUID: {"ok": True},
            hexname: 1,
            ABS: {"trusted": True},
            "/Users/bob/other": {"trusted": False},
            "alice@example.com": 2,
        }
        (directory / ("%s.json" % UUID)).write_text(json.dumps(doc), encoding="utf-8")
        out = shape(self.root / "agent")
        self.assertIn("== <uuid>/<uuid>.json (", out)
        self.assertIn("    dxt:allowlistCache:<uuid>: object", out)
        self.assertIn("    <hex>: number", out)
        self.assertIn("    <abs-path> (x 2): object", out)
        self.assertIn("    <email>: number", out)
        for text in (UUID, hexname, "alice", "bob", "example.com"):
            self.assertNotIn(text, out)

    def test_openai_ids_and_array_keys(self):
        user = "user-" + "Ab1" * 8
        org = "org-" + "Cd2" * 8
        directory = self.root / user
        directory.mkdir()
        doc = {
            json.dumps([user, UUID]): 1,
            json.dumps(["local", "client-new-thread:" + UUID, "win32:/mnt/c/users/bob/x.py"]): 2,
            json.dumps(["local", "C:\\Users\\bob\\x.py", 3, None]): 3,
            "win32:/mnt/c/users/bob/y.py": 4,
            "%s:%s" % (org, user): 5,
            "[not json": 6,
            "local:C:\\Users\\bob\\.codex": 7,
            "host:/home/bob/.codex": 8,
        }
        (directory / "state.json").write_text(json.dumps(doc), encoding="utf-8")
        out = shape(self.root)
        self.assertIn("== <user-id>/state.json (", out)
        self.assertIn('    ["<user-id>","<uuid>"]: number', out)
        self.assertIn('    ["local","client-new-thread:<uuid>","<abs-path>"]: number', out)
        self.assertIn('    ["local","<abs-path>","<int>","<NoneType>"]: number', out)
        self.assertIn("    <abs-path>: number", out)
        self.assertIn("    <org-id>:<user-id>: number", out)
        self.assertIn("    [not json: number", out)
        self.assertIn("    host:<abs-path>: number", out)
        self.assertIn("    local:<abs-path>: number", out)
        for text in (user, org, UUID, "bob", "mnt"):
            self.assertNotIn(text, out)

    def test_relative_paths(self):
        (self.root / "a" / "b").mkdir(parents=True)
        (self.root / "a" / "b" / "c.json").write_text("{}", encoding="utf-8")
        out = shape(self.root / "a")
        self.assertIn("== b/c.json (2 bytes, json)\n  {}\n", out)


class WalkTests(ShapeBase):
    """The directory walk: skipped subtrees, the size limit and symlinks it never follows."""

    def setUp(self):
        super().setUp()
        top = self.root / "top"
        (top / "Cache").mkdir(parents=True)
        (top / "Cache" / "data_0.json").write_text("{}", encoding="utf-8")
        (top / "logs").mkdir()
        (top / "logs" / "a.json").write_text("{}", encoding="utf-8")
        (top / "big.json").write_text(json.dumps({"k": "v" * 200}), encoding="utf-8")
        (top / "small.json").write_text('{"k": 1}', encoding="utf-8")
        (top / "key.txt").write_text("deadbeef" * 4 + "\n", encoding="utf-8")
        (top / "blob.bin").write_bytes(b"\x00\xff\xfe")
        outside = self.root / "outside"
        outside.mkdir()
        (outside / "hidden.json").write_text('{"leak": 1}', encoding="utf-8")
        self.top = top
        try:
            os.symlink(outside, top / "linkdir")
        except OSError:
            self.linked = False
        else:
            self.linked = True

    def test_default_skip_and_files(self):
        out = shape(self.top)
        self.assertIn("== Cache/ (directory, skipped)", out)
        self.assertNotIn("data_0", out)
        self.assertIn("== logs/a.json (", out)
        self.assertIn("== key.txt (33 bytes, not JSON, hex)", out)
        self.assertIn("== blob.bin (3 bytes, not JSON, binary)", out)
        names = [line.split(" (")[0] for line in out.splitlines() if line.startswith("== ")]
        self.assertEqual(names, sorted(names))

    def test_skip_option(self):
        out = shape("--skip", "logs", self.top)
        self.assertIn("== logs/ (directory, skipped)", out)
        self.assertNotIn("a.json", out)

    def test_max_size(self):
        out = shape("--max-size", "100", self.top)
        self.assertIn("== big.json (", out)
        self.assertIn("bytes, skipped (large))", out)
        self.assertIn("== small.json (8 bytes, json)", out)

    def test_symlink_not_followed(self):
        if not self.linked:
            self.skipTest("cannot create symlinks here")
        out = shape(self.top)
        self.assertIn("== linkdir (symlink, not followed)", out)
        self.assertNotIn("hidden.json", out)
        self.assertNotIn("leak", out)

    def test_missing_path(self):
        out = shape(self.root / "missing", self.top)
        self.assertIn("unreadable (FileNotFoundError)", out)
        self.assertIn("== small.json (", out)


if __name__ == "__main__":
    unittest.main()
