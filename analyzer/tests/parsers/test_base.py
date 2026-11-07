"""Tests of the fixture home guard in `ParserBase`."""

import unittest
from pathlib import Path

from fixtures import CLAUDE_SESSION, CODEX_GLOBAL_STATE_REL, build_codex_desktop

from .base import ParserBase


class _GuardedHome(ParserBase):
    """A ParserBase subclass driven by hand, so the guard is tested without test ordering."""

    extra_builders = (build_codex_desktop,)


class ParserBaseGuardTests(unittest.TestCase):
    """The class-level home is reused until a test changes it, then rebuilt."""

    def setUp(self):
        _GuardedHome.setUpClass()
        self.addCleanup(_GuardedHome.tearDownClass)
        self.first_tmp = _GuardedHome.tmp
        self.transcript = (
            _GuardedHome.home / ".claude/projects/-srv-proj" / (CLAUDE_SESSION + ".jsonl")
        )

    def _next_test(self) -> Path:
        _GuardedHome().setUp()
        return _GuardedHome.tmp

    def test_extra_builders_files_are_in_the_pristine_home(self):
        self.assertTrue((_GuardedHome.home / CODEX_GLOBAL_STATE_REL).is_file())
        self.assertIn(
            (_GuardedHome.home / CODEX_GLOBAL_STATE_REL).relative_to(_GuardedHome.tmp),
            {Path(entry[0]) for entry in _GuardedHome._pristine},
        )

    def test_untouched_home_is_kept(self):
        self.assertEqual(self._next_test(), self.first_tmp)

    def test_same_size_rewrite_rebuilds(self):
        original = self.transcript.read_bytes()
        self.transcript.write_bytes(bytes(reversed(original)))
        self.assertNotEqual(self._next_test(), self.first_tmp)
        rebuilt = _GuardedHome.home / self.transcript.relative_to(self.first_tmp / "home" / "alice")
        self.assertEqual(rebuilt.read_bytes(), original)

    def test_chmod_rebuilds(self):
        self.transcript.chmod(0)
        self.addCleanup(lambda: self.transcript.exists() and self.transcript.chmod(0o644))
        self.assertNotEqual(self._next_test(), self.first_tmp)

    def test_new_file_outside_the_home_rebuilds(self):
        (_GuardedHome.tmp / "out.txt").write_text("x", encoding="utf-8")
        self.assertNotEqual(self._next_test(), self.first_tmp)

    def test_opened_input_change_rebuilds(self):
        _GuardedHome.col.artifacts[0].rel = "changed"
        self.assertNotEqual(self._next_test(), self.first_tmp)
        self.assertNotEqual(_GuardedHome.col.artifacts[0].rel, "changed")
