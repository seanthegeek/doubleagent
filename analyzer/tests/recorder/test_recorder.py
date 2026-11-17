"""Tests of the recorder's corpus writer, snapshots and parse recording."""

import json
import os
import random
import shutil
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace

from recorder import corpus
from recorder.plugin import NameSequence, Recorder


def read_cases(directory: Path, group: str) -> list[dict]:
    with open(directory / (group + ".jsonl"), encoding="utf-8") as handle:
        return [json.loads(line) for line in handle]


class TaggedTextTests(unittest.TestCase):
    """Text that JSON cannot carry as UTF-8 is written in the hex form."""

    def test_forms(self):
        cases = [
            ("plain text is kept", corpus.tagged_text("café"), "café"),
            (
                "a lone surrogate keeps its UTF-8 form",
                corpus.tagged_text("\ud83dx"),
                {"hex": "eda0bd78"},
            ),
            ("a UTF-8 name is kept", corpus.tagged_name("notes.txt"), "notes.txt"),
            (
                "a name of other bytes keeps its bytes",
                corpus.tagged_name(os.fsdecode(b"\xffa")),
                {"hex": "ff61"},
            ),
        ]
        for name, got, want in cases:
            with self.subTest(name):
                self.assertEqual(got, want)


class CorpusTests(unittest.TestCase):
    """The root placeholder, blobs, trees and the refusal to clear a foreign directory."""

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="recorder-test-"))
        self.length = len(os.fsencode(os.path.realpath(self.tmp))) + 40
        self.root = corpus.make_work_root(self.tmp / "work", self.length)
        self.store = corpus.Corpus(self.tmp / "corpus", self.root, "/repo")

    def tearDown(self):
        corpus.remove_work(self.tmp / "work")
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_the_root_is_padded_to_the_placeholder_length(self):
        self.assertEqual(len(os.fsencode(self.root)), self.length)
        self.assertEqual(len(corpus.ROOT_PLACEHOLDER), corpus.ROOT_LENGTH)
        self.assertEqual(len(corpus.placeholder(self.length)), self.length)

    def test_case_text_carries_tokens_and_blobs_the_padded_placeholder(self):
        text = self.store.portable({"path": Path(self.root) / "x", "repo": "/repo/analyzer"})
        self.assertEqual(text, {"path": "{ROOT}/x", "repo": "{REPO}/analyzer"})
        digest = self.store.blob(("at %s/x" % self.root).encode())
        data = (self.tmp / "corpus" / "blobs" / digest).read_bytes()
        self.assertEqual(data, ("at %s/x" % corpus.placeholder(self.length)).encode())

    def test_a_directory_holding_other_files_is_not_cleared(self):
        foreign = self.tmp / "foreign"
        foreign.mkdir()
        (foreign / "keep.txt").write_text("mine", encoding="utf-8")
        with self.assertRaises(corpus.RecorderError):
            corpus.Corpus(foreign, self.root, "")
        self.assertTrue((foreign / "keep.txt").exists())

    def test_a_denied_file_keeps_the_bytes_of_the_earlier_snapshot(self):
        top = Path(self.root) / "top"
        top.mkdir()
        (top / "secret").write_text("hidden", encoding="utf-8")
        snapshots = corpus.Snapshotter(self.store)
        snapshots.root_entry("top")
        os.chmod(top / "secret", 0)
        try:
            snapshot = snapshots.root_entry("top")
        finally:
            os.chmod(top / "secret", 0o600)
        assert snapshot.entry is not None
        tree = self.store.tree([snapshot.entry])
        files = corpus.tree_files(self.store.trees, tree)
        self.assertEqual(list(files), ["top/secret"])
        entry = self.store.trees[snapshot.entry["tree"]][0]
        self.assertEqual(entry["mode"], 0)
        blob = self.tmp / "corpus" / "blobs" / files["top/secret"][1]
        self.assertEqual(blob.read_bytes(), b"hidden")
        self.assertEqual(sorted(snapshot.mtimes), ["top", "top/secret"])

    def test_an_entry_of_an_unsearchable_directory_keeps_its_last_snapshot(self):
        top = Path(self.root) / "top"
        (top / "sessions").mkdir(parents=True)
        (top / "sessions" / "a.jsonl").write_text("{}", encoding="utf-8")
        snapshots = corpus.Snapshotter(self.store)
        before = snapshots.root_entry("top")
        os.chmod(top, 0o600)
        try:
            after = snapshots.root_entry("top")
        finally:
            os.chmod(top, 0o700)
        assert before.entry is not None and after.entry is not None
        self.assertEqual(after.entry["mode"], 0o600)
        self.assertEqual(after.entry["tree"], before.entry["tree"])

    def test_top_of_names_the_first_component_under_the_root(self):
        self.assertEqual(corpus.top_of(self.root, self.root + "/a/b"), "a")
        self.assertEqual(corpus.top_of(self.root, "/elsewhere/a"), "")


class FakeParser:
    """A parser stand-in whose parse yields two rows, then raises when told to."""

    agent = "fake"

    def parse(self, artifact, opts):
        yield SimpleNamespace(text="one")
        yield SimpleNamespace(text="two")
        if opts.fail:
            raise ValueError("broken")


class ParseRecordingTests(unittest.TestCase):
    """A recorded parse returns what the parser yields and records how it ended."""

    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="recorder-test-"))
        length = len(os.fsencode(os.path.realpath(self.tmp))) + 40
        root = corpus.make_work_root(self.tmp / "work", length)
        self.directory = self.tmp / "corpus"
        self.recorder = Recorder(corpus.Corpus(self.directory, root, ""), self.tmp / "work")
        self.recorder.start_test("node-1")

    def tearDown(self):
        self.recorder.corpus.close([])
        corpus.remove_work(self.tmp / "work")
        shutil.rmtree(self.tmp, ignore_errors=True)

    def parse(self, fail: bool = False):
        artifact = SimpleNamespace(rel="a.jsonl")
        parser = FakeParser()
        return self.recorder.parse(FakeParser.parse, parser, artifact, SimpleNamespace(fail=fail))

    def test_ends_are_recorded(self):
        rows = list(self.parse())
        self.assertEqual([row.text for row in rows], ["one", "two"])
        stopped = self.parse()
        next(stopped)
        stopped.close()
        with self.assertRaises(ValueError):
            list(self.parse(fail=True))
        self.recorder.corpus.close([])
        results = [
            json.loads((self.directory / "blobs" / case["result_blob"]).read_text(encoding="utf-8"))
            for case in read_cases(self.directory, "parsers/fake")
        ]
        self.assertEqual(
            [(len(result["rows"]), result["completed"], result["error"]) for result in results],
            [
                (2, True, None),
                (1, False, None),
                (2, False, {"type": "builtins.ValueError", "text": "broken"}),
            ],
        )

    def test_names_repeat_per_test(self):
        sequence = NameSequence()
        sequence.rng = random.Random("node-1")
        first = [next(sequence) for _ in range(3)]
        sequence.rng = random.Random("node-1")
        self.assertEqual([next(sequence) for _ in range(3)], first)


if __name__ == "__main__":
    unittest.main()
