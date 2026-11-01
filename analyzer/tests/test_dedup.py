"""dedup.deduplicate: secondary-source rows dropped when a primary store yields the same event."""

import json
import shutil
import tempfile
import unittest
from collections import Counter
from pathlib import Path

from doubleagent import catalog, cli
from doubleagent.dedup import TOLERANCE_SECONDS, deduplicate
from doubleagent.inputs import open_input
from doubleagent.model import Row
from doubleagent.parsers import Options
from fixtures import (
    CODEX_SESSION,
    bad_line_text,
    codex_history_records,
    codex_rollout_records,
    write_bad_line,
)

SESSION_ID = "11111111-2222-4333-8444-555555555555"
BACKUP_A = "/home/alice/.app/backups/1/history.jsonl"
BACKUP_B = "/home/alice/.app/backups/2/history.jsonl"
BACKUP_C = "/home/alice/.app/backups/3/history.jsonl"


def row(timestamp="2026-10-01T10:00:00.000Z", **fields) -> Row:
    result = Row(
        timestamp_utc=timestamp,
        host="h1",
        user="alice",
        agent="claude-code",
        session_id=SESSION_ID,
        project_path="/srv/proj",
        turn_type="user",
        text="delete the logs",
        source_file="/home/alice/.claude/projects/-srv-proj/%s.jsonl" % SESSION_ID,
        source_line=2,
    )
    for key, value in fields.items():
        setattr(result, key, value)
    return result


def history_row(timestamp="2026-10-01T10:00:00.000Z", **fields) -> Row:
    fields.setdefault("source_file", "/home/alice/.claude/history.jsonl")
    fields.setdefault("source_line", 1)
    return row(timestamp, **fields)


def dedup(primary, secondary, tolerance=TOLERANCE_SECONDS):
    """`deduplicate` with every secondary row at the same tolerance."""
    return deduplicate(primary, [(secondary_row, tolerance) for secondary_row in secondary])


class DeduplicateTests(unittest.TestCase):
    """`deduplicate` on synthetic primary and secondary rows."""

    def kept_secondary(self, primary, secondary, tolerance=TOLERANCE_SECONDS):
        kept, _ = dedup(primary, secondary, tolerance)
        return kept[len(primary) :]

    def test_match_by_session(self):
        primary_row = row()
        kept, dropped = dedup([primary_row], [history_row(project_path="/elsewhere")])
        self.assertEqual(kept, [primary_row])
        self.assertEqual(dropped, Counter({"claude-code": 1}))

    def test_match_one_millisecond_before_the_epoch(self):
        # -1 ms is a time like any other, not the NO_MATCH marker.
        moment = "1969-12-31T23:59:59.999Z"
        kept, dropped = dedup([row(moment)], [history_row(moment)])
        self.assertEqual(len(kept), 1)
        self.assertEqual(dropped, Counter({"claude-code": 1}))

    def test_match_by_project_without_session(self):
        primary_row = row()
        kept, dropped = dedup([primary_row], [history_row(session_id="")])
        self.assertEqual(kept, [primary_row])
        self.assertEqual(dropped["claude-code"], 1)

    def test_no_match_by_project_without_session_other_project(self):
        history = history_row(session_id="", project_path="/srv/other")
        self.assertEqual(self.kept_secondary([row()], [history]), [history])

    def test_session_differs_project_same(self):
        history = history_row(session_id="99999999-0000-4000-8000-000000000000")
        self.assertEqual(self.kept_secondary([row()], [history]), [history])

    def test_one_to_one_nearest(self):
        # A transcript cut after three of five identical prompts keeps two
        # of the five history lines, and each primary row takes the history
        # line nearest to it.
        primary = [
            row("2026-10-01T10:00:%02d.500Z" % second, text="continue", source_line=second)
            for second in (10, 20, 30)
        ]
        secondary = [
            history_row("2026-10-01T10:00:%02d.000Z" % second, text="continue", source_line=second)
            for second in (50, 10, 20, 30, 40)
        ]
        kept, dropped = dedup(primary, secondary)
        self.assertEqual(kept[:3], primary)
        self.assertEqual([kept_row.source_line for kept_row in kept[3:]], [50, 40])
        self.assertEqual(dropped, Counter({"claude-code": 3}))

    def test_nearest_candidate_wins(self):
        # Secondary rows go in timestamp order, each to the nearest unused
        # primary row: the first line takes the row 0.3 s away, not the one
        # 1.5 s away, which leaves the second line (3.5 s from it) unmatched.
        primary = [row("2026-10-01T10:00:00.000Z"), row("2026-10-01T10:00:01.800Z")]
        first = history_row("2026-10-01T10:00:01.500Z", source_line=1)
        second = history_row("2026-10-01T10:00:03.500Z", source_line=2)
        self.assertEqual(self.kept_secondary(primary, [second, first]), [second])

    def test_tolerance_boundary(self):
        self.assertEqual(TOLERANCE_SECONDS, 2.0)
        primary_row = row("2026-10-01T10:00:00.000Z")
        self.assertEqual(
            self.kept_secondary([primary_row], [history_row("2026-10-01T10:00:02.000Z")]), []
        )
        self.assertEqual(
            self.kept_secondary([primary_row], [history_row("2026-10-01T09:59:58.000Z")]), []
        )
        late = history_row("2026-10-01T10:00:02.001Z")
        self.assertEqual(self.kept_secondary([primary_row], [late]), [late])
        early = history_row("2026-10-01T09:59:57.999Z")
        self.assertEqual(self.kept_secondary([primary_row], [early]), [early])

    def test_timestamped_never_matches_undated_primary(self):
        history = history_row()
        self.assertEqual(self.kept_secondary([row("")], [history]), [history])
        self.assertEqual(self.kept_secondary([row("")], [history], 3600.0), [history])

    def test_undated_matches_by_session(self):
        primary_row = row()
        kept, dropped = dedup([primary_row], [history_row("", project_path="/elsewhere")])
        self.assertEqual(kept, [primary_row])
        self.assertEqual(dropped, Counter({"claude-code": 1}))
        history = history_row("", session_id="99999999-0000-4000-8000-000000000000")
        self.assertEqual(self.kept_secondary([primary_row], [history]), [history])

    def test_undated_matches_by_project(self):
        primary_row = row()
        self.assertEqual(self.kept_secondary([primary_row], [history_row("", session_id="")]), [])
        history = history_row("", session_id="", project_path="/srv/other")
        self.assertEqual(self.kept_secondary([primary_row], [history]), [history])

    def test_undated_without_session_or_project_matches_any(self):
        # A text-only history records neither: any primary row of that
        # host, user and agent with the text pairs with it.
        primary_row = row(project_path="/srv/other")
        self.assertEqual(
            self.kept_secondary([primary_row], [history_row("", session_id="", project_path="")]),
            [],
        )
        history = history_row("", session_id="", project_path="", user="bob")
        self.assertEqual(self.kept_secondary([primary_row], [history]), [history])

    def test_undated_text_differs(self):
        history = history_row("", text="delete the logs!")
        self.assertEqual(self.kept_secondary([row()], [history]), [history])

    def test_undated_both_sides(self):
        self.assertEqual(self.kept_secondary([row("")], [history_row("")]), [])

    def test_undated_one_to_one(self):
        # Three undated history lines and two primary rows: two drop, the
        # last in input order is kept; the dated primary row is taken first.
        primary = [row("", source_line=5), row("2026-10-01T10:00:00.000Z", source_line=3)]
        secondary = [history_row("", source_line=line) for line in (1, 2, 3)]
        kept, dropped = dedup(primary, secondary)
        self.assertEqual(kept[:2], primary)
        self.assertEqual([kept_row.source_line for kept_row in kept[2:]], [3])
        self.assertEqual(dropped, Counter({"claude-code": 2}))

    def test_undated_takes_earliest(self):
        # The undated row takes the earliest-timestamped unused candidate.
        late = row("2026-10-01T11:00:00.000Z", source_line=9)
        early = row("2026-10-01T09:00:00.000Z", source_line=4)
        dated = history_row("2026-10-01T11:00:01.000Z", source_line=2)
        undated = history_row("", source_line=1)
        # The dated line pairs with `late`, so the undated one takes `early`;
        # with only `late` there, the undated line is kept.
        self.assertEqual(self.kept_secondary([late, early], [undated, dated]), [])
        self.assertEqual(self.kept_secondary([late], [undated, dated]), [undated])

    def test_timestamped_before_undated(self):
        # The undated row comes first in input order but must not steal the
        # only candidate the timestamped row pairs with.
        primary_row = row()
        undated = history_row("", source_line=1)
        dated = history_row("2026-10-01T10:00:01.000Z", source_line=2)
        self.assertEqual(self.kept_secondary([primary_row], [undated, dated]), [undated])

    def test_per_row_tolerance(self):
        primary_row = row("2026-10-01T10:00:00.000Z")
        history = history_row("2026-10-01T10:05:00.000Z")
        self.assertEqual(self.kept_secondary([primary_row], [history], 600.0), [])
        self.assertEqual(self.kept_secondary([primary_row], [history]), [history])
        # Two rows in one call, each with its own window.
        wide = history_row("2026-10-01T10:05:00.000Z", source_line=1)
        narrow = history_row("2026-10-01T10:05:00.000Z", source_line=2, text="x")
        primary = [primary_row, row("2026-10-01T10:00:00.000Z", text="x")]
        kept, dropped = deduplicate(primary, [(wide, 600.0), (narrow, 2.0)])
        self.assertEqual(kept[2:], [narrow])
        self.assertEqual(dropped, Counter({"claude-code": 1}))

    def test_undated_copies_collapse(self):
        # Two backups of one history, no primary store: the first copy stays.
        copy_a = history_row("", source_line=1, source_file=BACKUP_A)
        copy_b = history_row("", source_line=1, source_file=BACKUP_B)
        kept, dropped = dedup([], [copy_a, copy_b])
        self.assertEqual(kept, [copy_a])
        self.assertEqual(dropped, Counter({"claude-code": 1}))

    def test_timestamped_copies_within_tolerance(self):
        copy_a = history_row("2026-10-01T10:00:00.000Z", source_file=BACKUP_A)
        copy_b = history_row("2026-10-01T10:00:01.500Z", source_file=BACKUP_B)
        self.assertEqual(dedup([], [copy_a, copy_b])[0], [copy_a])
        far = history_row("2026-10-01T10:00:02.001Z", source_file=BACKUP_B)
        self.assertEqual(dedup([], [copy_a, far])[0], [copy_a, far])
        # The tolerance is that of the row being tested.
        self.assertEqual(deduplicate([], [(copy_a, 2.0), (far, 600.0)])[0], [copy_a])

    def test_earliest_copy_kept(self):
        # Time decides among timestamped copies, not input order: the
        # earlier copy is kept wherever it comes in the input.
        late = history_row("2026-10-01T10:00:01.000Z", source_file=BACKUP_A)
        early = history_row("2026-10-01T10:00:00.000Z", source_file=BACKUP_B)
        self.assertEqual(dedup([], [late, early])[0], [early])

    def test_copies_of_primary_event_collapse(self):
        # Three copies of one primary event in two files: the first takes
        # the primary row, the other two collapse onto it and onto each
        # other, and only the primary row is left.
        primary_row = row()
        copy_a = history_row(source_file=BACKUP_A)
        copy_b = history_row("2026-10-01T10:00:00.500Z", source_file=BACKUP_B)
        copy_c = history_row("2026-10-01T10:00:01.000Z", source_file=BACKUP_A, source_line=2)
        kept, dropped = dedup([primary_row], [copy_a, copy_b, copy_c])
        self.assertEqual(kept, [primary_row])
        self.assertEqual(dropped, Counter({"claude-code": 3}))
        # Undated copies the same way, in either input order.
        undated_a = history_row("", source_file=BACKUP_A)
        undated_b = history_row("", source_file=BACKUP_B)
        for secondary in ([undated_a, undated_b], [undated_b, undated_a]):
            kept, dropped = dedup([primary_row], secondary)
            self.assertEqual(kept, [primary_row])
            self.assertEqual(dropped, Counter({"claude-code": 2}))

    def test_undated_copy_of_dated_dropped(self):
        # An undated copy pairs with a dated one, whichever comes first in
        # the input, and the dated copy is the one kept.
        dated = history_row("2026-10-01T10:00:00.000Z", source_file=BACKUP_A)
        undated = history_row("", source_file=BACKUP_B)
        for secondary in ([dated, undated], [undated, dated]):
            kept, dropped = dedup([], secondary)
            self.assertEqual(kept, [dated])
            self.assertEqual(dropped, Counter({"claude-code": 1}))

    def test_most_specific_undated_copy_takes_primary(self):
        # Two undated copies of one event, an unscoped history line first in
        # the input and a session-scoped index row second, and the primary
        # row: the session-scoped copy takes the primary row, the unscoped
        # line collapses onto it, and both drop.
        primary_row = row()
        loose = history_row("", session_id="", project_path="", source_file=BACKUP_A)
        scoped = history_row("", source_file=BACKUP_B)
        kept, dropped = dedup([primary_row], [loose, scoped])
        self.assertEqual(kept, [primary_row])
        self.assertEqual(dropped, Counter({"claude-code": 2}))

    def test_broader_scope_among_secondary(self):
        # A history line with a project and no session and a mirror line
        # with a session and no project, both copies of one primary event:
        # the history line takes the primary row (project scope), and the
        # mirror line, finding nothing under its session, meets it under its
        # identity alone.
        primary_row = row()
        history = history_row(session_id="", source_file=BACKUP_A)
        mirror = history_row("2026-10-01T10:00:00.300Z", project_path="", source_file=BACKUP_B)
        kept, dropped = dedup([primary_row], [history, mirror])
        self.assertEqual(kept, [primary_row])
        self.assertEqual(dropped, Counter({"claude-code": 2}))
        # Under a broader scope a candidate of another session or project is
        # never taken: "yes" in two sessions of two backups is two events.
        other = "99999999-0000-4000-8000-000000000000"
        session_a = history_row(text="yes", source_file=BACKUP_A)
        session_b = history_row(text="yes", session_id=other, source_file=BACKUP_B)
        self.assertEqual(dedup([], [session_a, session_b])[0], [session_a, session_b])
        project_a = history_row(text="yes", session_id="", source_file=BACKUP_A)
        project_b = history_row(text="yes", project_path="/srv/other", source_file=BACKUP_B)
        self.assertEqual(dedup([], [project_a, project_b])[0], [project_a, project_b])
        # The primary pass stays strict: a row with a session never takes a
        # primary row of no session under a broader key.
        scoped = history_row(session_id=other, project_path="")
        self.assertEqual(self.kept_secondary([row(session_id="")], [scoped]), [scoped])

    def test_tool_rows_pair_on_id(self):
        for turn_type in ("tool_use", "tool_result"):
            primary_row = row(
                turn_type=turn_type, tool_use_id="call_1", text='{"cmd": ["ls", "-la"]}'
            )
            same_id = history_row(turn_type=turn_type, tool_use_id="call_1", text="ls -la")
            kept, dropped = dedup([primary_row], [same_id])
            self.assertEqual(kept, [primary_row], turn_type)
            self.assertEqual(dropped, Counter({"claude-code": 1}), turn_type)
            other_id = history_row(turn_type=turn_type, tool_use_id="call_2", text=primary_row.text)
            self.assertEqual(self.kept_secondary([primary_row], [other_id]), [other_id], turn_type)
            # When either row has no id, text decides: a secondary row
            # without one pairs with the primary row with one.
            no_id = history_row(turn_type=turn_type, text=primary_row.text)
            self.assertEqual(self.kept_secondary([primary_row], [no_id]), [], turn_type)
            other_text = history_row(turn_type=turn_type, text="ls")
            self.assertEqual(
                self.kept_secondary([primary_row], [other_text]), [other_text], turn_type
            )
            p_no_id = row(turn_type=turn_type, text="ls -la")
            self.assertEqual(
                self.kept_secondary([p_no_id], [history_row(turn_type=turn_type, text="ls -la")]),
                [],
            )
            # The id never stands in for a user row's text.
            user_row = history_row(tool_use_id="call_1", text="other")
            self.assertEqual(
                self.kept_secondary([row(tool_use_id="call_1")], [user_row]), [user_row]
            )
            # Copies among secondary rows pair on the id too.
            copy_a = history_row(
                turn_type=turn_type, tool_use_id="call_1", text="a", source_file=BACKUP_A
            )
            copy_b = history_row(
                turn_type=turn_type, tool_use_id="call_1", text="b", source_file=BACKUP_B
            )
            self.assertEqual(dedup([], [copy_a, copy_b])[0], [copy_a], turn_type)

    def test_copies_one_to_one_and_chained(self):
        # Two backups each holding "continue" twice, a minute apart, keep
        # two rows; three copies of one event keep one.
        first_time, second_time = "2026-10-01T10:00:00.000Z", "2026-10-01T10:01:00.000Z"
        first_a, second_a, first_b, second_b = (
            history_row(timestamp, text="continue", source_line=line, source_file=source)
            for line, timestamp, source in (
                (1, first_time, BACKUP_A),
                (2, second_time, BACKUP_A),
                (1, first_time, BACKUP_B),
                (2, second_time, BACKUP_B),
            )
        )
        self.assertEqual(dedup([], [first_a, second_a, first_b, second_b])[0], [first_a, second_a])
        copy_a, copy_b, copy_c = (
            history_row("", source_file=source) for source in (BACKUP_A, BACKUP_B, BACKUP_C)
        )
        kept, dropped = dedup([], [copy_a, copy_b, copy_c])
        self.assertEqual(kept, [copy_a])
        self.assertEqual(dropped, Counter({"claude-code": 2}))

    def test_repeats_in_one_source_kept(self):
        # Two "yes" lines in one undated history with no primary store are
        # two events the file records: both stay. The same two lines in two
        # files are one event: the first stays.
        copy_a = history_row("", text="yes", source_line=1, source_file=BACKUP_A)
        copy_b = history_row("", text="yes", source_line=3, source_file=BACKUP_A)
        kept, dropped = dedup([], [copy_a, copy_b])
        self.assertEqual(kept, [copy_a, copy_b])
        self.assertEqual(dropped, Counter())
        copy_c = history_row("", text="yes", source_line=1, source_file=BACKUP_B)
        self.assertEqual(dedup([], [copy_a, copy_c])[0], [copy_a])
        stamped = [
            history_row(
                "2026-10-01T10:00:00.000Z", text="yes", source_line=line, source_file=BACKUP_A
            )
            for line in (1, 2)
        ]
        self.assertEqual(dedup([], stamped)[0], stamped)

    def test_other_fields_never_match(self):
        for field, value in (
            ("turn_type", "system"),
            ("text", "delete the logs!"),
            ("user", "bob"),
            ("host", "h2"),
            ("agent", "codex-cli"),
        ):
            history = history_row(**{field: value})
            self.assertEqual(self.kept_secondary([row()], [history]), [history], field)

    def test_primary_never_dropped(self):
        # Two identical primary rows stay, with their own source lines.
        first = row(source_line=2)
        second = row(source_line=9)
        kept, dropped = dedup([first, second], [history_row()])
        self.assertEqual(kept, [first, second])
        self.assertEqual(sum(dropped.values()), 1)
        kept, dropped = dedup([first, second], [])
        self.assertEqual(kept, [first, second])
        self.assertEqual(dropped, Counter())

    def test_counter_per_agent(self):
        primary = [row(), row(agent="antigravity"), row(agent="codex-cli")]
        secondary = [
            history_row(),
            history_row(agent="antigravity"),
            history_row(agent="codex-cli", text="x"),
        ]
        _, dropped = dedup(primary, secondary)
        self.assertEqual(dropped, Counter({"claude-code": 1, "antigravity": 1}))

    def test_result_order(self):
        # Every primary row in input order, then the unmatched secondary
        # rows in input order, not timestamp order.
        primary_b = row("2026-10-01T10:00:05.000Z", text="b")
        primary_a = row("2026-10-01T10:00:00.000Z", text="a")
        late = history_row("2026-10-01T11:00:00.000Z", text="late")
        copy_of_a = history_row("2026-10-01T10:00:00.000Z", text="a")
        early = history_row("2026-10-01T09:00:00.000Z", text="early")
        kept, _ = dedup([primary_b, primary_a], [late, copy_of_a, early])
        self.assertEqual(kept, [primary_b, primary_a, late, early])


if __name__ == "__main__":
    unittest.main()


class ParserRowTests(unittest.TestCase):
    """A `parser:` row is never dropped as a secondary copy and never takes one.

    A `parser:` row describes its own `source_file`, so it is never dropped as a secondary copy and
    never takes one.
    """

    def test_parser_rows_never_paired_unit(self):
        text = "parser: unreadable (Permission denied)"
        primary_row = row(timestamp="", turn_type="system", text=text)
        history = history_row(timestamp="", turn_type="system", text=text)
        backup_history = history_row(
            timestamp="", turn_type="system", text=text, source_file=BACKUP_A
        )
        kept, dropped = dedup([primary_row, row()], [history, backup_history, history_row()])
        self.assertEqual(kept, [primary_row, row(), history, backup_history])
        self.assertEqual(dropped, Counter({"claude-code": 1}))

    def test_rollout_and_history_with_same_bad_line(self):
        temporary = Path(tempfile.mkdtemp(prefix="cac-analyzer-test-"))
        self.addCleanup(shutil.rmtree, temporary, ignore_errors=True)
        home = temporary / "home" / "alice"
        rollout = (
            home
            / ".codex/sessions/2026/10/02"
            / ("rollout-2026-10-02T09-00-00-%s.jsonl" % CODEX_SESSION)
        )
        history = home / ".codex/history.jsonl"
        # The source_file of a loose input is the path under its root.
        rollout_source = "/" + str(rollout.relative_to(temporary / "home"))
        history_source = "/" + str(history.relative_to(temporary / "home"))
        for path, records in (
            (rollout, codex_rollout_records()),
            (history, codex_history_records()),
        ):
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(
                "".join(json.dumps(record) + "\n" for record in records), encoding="utf-8"
            )
            write_bad_line(path)
        col = open_input(temporary / "home", catalog.load(), host="h1")
        rows, _, problems, dropped = cli.collect_rows(col, Options(), ["codex-cli"])
        self.assertEqual(problems, [])
        bad = [parsed for parsed in rows if parsed.text == bad_line_text()]
        self.assertEqual(
            sorted(parsed.source_file for parsed in bad), sorted([history_source, rollout_source])
        )
        # The history's one prompt repeats the rollout's and is still dropped.
        self.assertEqual(dropped, Counter({"codex-cli": 1}))
        prompt = codex_history_records()[0]["text"]
        self.assertEqual(
            [
                parsed.source_file
                for parsed in rows
                if parsed.turn_type == "user" and parsed.text == prompt
            ],
            [rollout_source],
        )
