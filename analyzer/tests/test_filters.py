"""Tests of the time and text filters of `timeline`."""

import contextlib
import io
import json
import re
import shutil
import tempfile
import unittest
from datetime import UTC, datetime
from pathlib import Path
from typing import ClassVar

from doubleagent import cli
from doubleagent.filters import BoundError, RowFilter, parse_bound
from doubleagent.model import Row
from fixtures import build_home

NOW = datetime(2026, 3, 31, 14, 5, 9, 123000, tzinfo=UTC)


class ParseBoundTests(unittest.TestCase):
    """`parse_bound` reads the `--since` and `--until` forms and rejects the rest."""

    def both(self, text):
        return parse_bound(text, now=NOW), parse_bound(text, end=True, now=NOW)

    def test_absolute_forms(self):
        cases = {
            "2026-10-01": ("2026-10-01T00:00:00.000Z", "2026-10-01T23:59:59.999Z"),
            "2026-10": ("2026-10-01T00:00:00.000Z", "2026-10-31T23:59:59.999Z"),
            "2024-02": ("2024-02-01T00:00:00.000Z", "2024-02-29T23:59:59.999Z"),
            "2026": ("2026-01-01T00:00:00.000Z", "2026-12-31T23:59:59.999Z"),
            "2026-10-01T09": ("2026-10-01T09:00:00.000Z", "2026-10-01T09:59:59.999Z"),
            "2026-10-01T09:30": ("2026-10-01T09:30:00.000Z", "2026-10-01T09:30:59.999Z"),
            "2026-10-01 09:30": ("2026-10-01T09:30:00.000Z", "2026-10-01T09:30:59.999Z"),
            "2026-10-01 09:30:15": ("2026-10-01T09:30:15.000Z", "2026-10-01T09:30:15.999Z"),
            "2026-10-01T09:30:15.25": ("2026-10-01T09:30:15.250Z", "2026-10-01T09:30:15.250Z"),
            "2026-10-01T09:30Z": ("2026-10-01T09:30:00.000Z", "2026-10-01T09:30:59.999Z"),
            "2026-10-01T09:30+02:00": ("2026-10-01T07:30:00.000Z", "2026-10-01T07:30:59.999Z"),
            "2026-10-01T09:30-0500": ("2026-10-01T14:30:00.000Z", "2026-10-01T14:30:59.999Z"),
            "2026-10-01T09:30+02": ("2026-10-01T07:30:00.000Z", "2026-10-01T07:30:59.999Z"),
            "2026-10-01 09:30 +02:00": ("2026-10-01T07:30:00.000Z", "2026-10-01T07:30:59.999Z"),
            "2026-10-01 09:30:15 -05:00": ("2026-10-01T14:30:15.000Z", "2026-10-01T14:30:15.999Z"),
            "2026-10-01 09:30 UTC": ("2026-10-01T09:30:00.000Z", "2026-10-01T09:30:59.999Z"),
            "20261001T093015": ("2026-10-01T09:30:15.000Z", "2026-10-01T09:30:15.999Z"),
            "2026-W40-1": ("2026-09-28T00:00:00.000Z", "2026-09-28T23:59:59.999Z"),
            "2026-274": ("2026-10-01T00:00:00.000Z", "2026-10-01T23:59:59.999Z"),
            "1759312800": ("2025-10-01T10:00:00.000Z", "2025-10-01T10:00:00.000Z"),
            "1759312800000": ("2025-10-01T10:00:00.000Z", "2025-10-01T10:00:00.000Z"),
        }
        for text, want in cases.items():
            self.assertEqual(self.both(text), want, text)

    def test_day_words(self):
        self.assertEqual(
            self.both("today"), ("2026-03-31T00:00:00.000Z", "2026-03-31T23:59:59.999Z")
        )
        self.assertEqual(
            self.both("Yesterday"), ("2026-03-30T00:00:00.000Z", "2026-03-30T23:59:59.999Z")
        )
        self.assertEqual(self.both("now"), ("2026-03-31T14:05:09.123Z",) * 2)

    def test_relative(self):
        cases = {
            "45m": "2026-03-31T13:20:09.123Z",
            "45 min": "2026-03-31T13:20:09.123Z",
            "45 minutes ago": "2026-03-31T13:20:09.123Z",
            "36h": "2026-03-30T02:05:09.123Z",
            "3d": "2026-03-28T14:05:09.123Z",
            "  3D   ago ": "2026-03-28T14:05:09.123Z",
            "2w": "2026-03-17T14:05:09.123Z",
            # Month ends clamp: 31 March less a month is 28 February.
            "1mo": "2026-02-28T14:05:09.123Z",
            "6 months ago": "2025-09-30T14:05:09.123Z",
            "1y": "2025-03-31T14:05:09.123Z",
            "2 years": "2024-03-31T14:05:09.123Z",
        }
        for text, want in cases.items():
            self.assertEqual(self.both(text), (want, want), text)

    def test_rejected(self):
        rejected = ("", "3x", "last friday", "10/01/2026", "2026-13-01", "2026-10-01T25:00")
        out_of_range = ("99999y", "9" * 5000 + "d", "9" * 20)
        for text in (*rejected, *out_of_range):
            with self.assertRaises(BoundError, msg=text) as caught:
                parse_bound(text, now=NOW)
            self.assertIn("; accepted:", str(caught.exception), text)
        with self.assertRaisesRegex(BoundError, "month must be in 1..12"):
            parse_bound("2026-13-01")
        with self.assertRaisesRegex(BoundError, "out of range"):
            parse_bound("99999y", now=NOW)
        # Past Python's integer conversion digit limit, the same error as any other size.
        with self.assertRaisesRegex(BoundError, "out of range; accepted:"):
            parse_bound("9" * 5000 + "d", now=NOW)


def _row(timestamp, text="", turn_type="user", session="s1", tool_use_id=""):
    return Row(
        timestamp_utc=timestamp,
        agent="claude-code",
        session_id=session,
        turn_type=turn_type,
        text=text,
        tool_use_id=tool_use_id,
    )


class RowFilterTests(unittest.TestCase):
    """`RowFilter` keeps the rows in the window and the matches with their tool partners."""

    ROWS: ClassVar[list[Row]] = [
        _row("2026-10-01T08:59:59.999Z", "before"),
        _row("2026-10-01T09:00:00.000Z", "first"),
        _row("2026-10-01T09:00:01.000Z", "curl http://x", "tool_use", tool_use_id="t1"),
        _row("2026-10-01T09:00:02.000Z", "200 OK", "tool_result", tool_use_id="t1"),
        _row("2026-10-01T09:00:03.000Z", "ls", "tool_use", session="s2", tool_use_id="t1"),
        _row("2026-10-01T10:00:00.000Z", "last"),
        _row("2026-10-01T10:00:00.001Z", "after"),
        _row("", "undated CURL"),
    ]

    def test_window_is_inclusive(self):
        row_filter = RowFilter("2026-10-01T09:00:00.000Z", "2026-10-01T10:00:00.000Z")
        got = [row.text for row in row_filter.apply(self.ROWS)]
        self.assertEqual(got, ["first", "curl http://x", "200 OK", "ls", "last"])
        self.assertEqual((row_filter.dropped_window, row_filter.dropped_undated), (2, 1))

    def test_keep_undated(self):
        row_filter = RowFilter("2026-10-01T09:00:00.000Z", keep_undated=True)
        self.assertIn("undated CURL", [row.text for row in row_filter.apply(self.ROWS)])
        self.assertEqual(row_filter.dropped_undated, 0)

    def test_match_keeps_tool_partner_in_same_session(self):
        row_filter = RowFilter(patterns=[re.compile("curl")])
        got = [(row.session_id, row.text) for row in row_filter.apply(self.ROWS)]
        # The result of t1 in s1 comes along; the t1 of session s2 does not.
        self.assertEqual(got, [("s1", "curl http://x"), ("s1", "200 OK")])
        self.assertEqual(row_filter.dropped_match, len(self.ROWS) - 2)

    def test_match_ignore_case_and_or(self):
        row_filter = RowFilter(patterns=[re.compile("CURL", re.IGNORECASE), re.compile("^last$")])
        got = [row.text for row in row_filter.apply(self.ROWS)]
        self.assertEqual(got, ["curl http://x", "200 OK", "last", "undated CURL"])

    def test_inactive_filter_keeps_everything(self):
        row_filter = RowFilter()
        self.assertFalse(row_filter.active)
        self.assertEqual(row_filter.apply(self.ROWS), self.ROWS)


class TimelineFilterCliTests(unittest.TestCase):
    """The filter options of `timeline` on the shared fixture home."""

    @classmethod
    def setUpClass(cls):
        cls.tmp = Path(tempfile.mkdtemp(prefix="cac-analyzer-test-"))
        build_home(cls.tmp / "home")

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.tmp, ignore_errors=True)

    def run_timeline(self, name, *extra):
        out = self.tmp / name
        buffer, error = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(buffer), contextlib.redirect_stderr(error):
            exit_code = cli.main(["timeline", str(self.tmp / "home"), "-o", str(out), *extra])
        rows, sessions = [], []
        if (out / "timeline.jsonl").exists():
            with open(out / "timeline.jsonl", encoding="utf-8") as handle:
                rows = [json.loads(line) for line in handle]
            with open(out / "sessions.jsonl", encoding="utf-8") as handle:
                sessions = [json.loads(line) for line in handle]
        return exit_code, rows, sessions, buffer.getvalue(), error.getvalue()

    def test_day_window(self):
        exit_code, rows, sessions, out, _ = self.run_timeline(
            "day", "--since", "2026-10-01", "--until", "2026-10-01"
        )
        self.assertEqual(exit_code, 0, out)
        self.assertTrue(rows)
        self.assertTrue(all(row["timestamp_utc"].startswith("2026-10-01T") for row in rows))
        self.assertIn("window:    2026-10-01T00:00:00.000Z to 2026-10-01T23:59:59.999Z", out)
        self.assertRegex(out, r"filtered:  \d+ outside the window, \d+ without a timestamp")
        # Sessions are summarised from the kept rows only.
        for session in sessions:
            self.assertTrue(session["first_timestamp_utc"].startswith("2026-10-01T"), session)

    def test_keep_undated(self):
        _, rows, _, _, _ = self.run_timeline("undated", "--since", "2000", "--keep-undated")
        _, all_rows, _, _, _ = self.run_timeline("all")
        self.assertEqual(len(rows), len(all_rows))
        self.assertTrue(any(not row["timestamp_utc"] for row in rows))

    def test_match(self):
        exit_code, rows, _, out, _ = self.run_timeline("match", "--match", "RM -RF", "-i")
        self.assertEqual(exit_code, 0, out)
        self.assertTrue(rows)
        hits = [row for row in rows if re.search("rm -rf", row["text"], re.IGNORECASE)]
        self.assertTrue(hits)
        partners = [row for row in rows if row not in hits]
        self.assertTrue(all(row["tool_use_id"] for row in partners), partners)
        self.assertIn("match:     RM -RF (ignore case)", out)

    def test_empty_window_exits_1(self):
        exit_code, rows, _, _, _ = self.run_timeline("none", "--since", "1990", "--until", "1990")
        self.assertEqual((exit_code, rows), (1, []))

    def test_bad_values_exit_2_before_reading_input(self):
        for extra in (
            ["--since", "last friday"],
            ["--until", "2026-13-01"],
            ["--since", "2026-10-02", "--until", "2026-10-01"],
            ["--match", "("],
        ):
            exit_code, _, _, _, error = self.run_timeline("bad", *extra)
            self.assertEqual(exit_code, 2, extra)
            self.assertTrue(error.startswith("error: "), error)
            self.assertFalse((self.tmp / "bad").exists(), extra)


if __name__ == "__main__":
    unittest.main()
