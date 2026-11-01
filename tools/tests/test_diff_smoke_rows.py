"""Tests of `tools/diff_smoke_rows.py` on synthetic listings and manifests; no collector runs."""

from __future__ import annotations

import json

import diff_smoke_rows
from diff_smoke_rows import DiffError, Row, RowChange, TableChange

HOME = "/w/root/home/alice"
VOLUME = "/w/root/var/lib/docker/volumes/tabby_data/_data"

LISTING_BEFORE = """# home artifacts (agent|glob relative to home)
# Anthropic
claude-code|.claude
openclaw|.openclaw
# project artifacts (relative to each discovered project)
claude-code|.claude
# excluded unless --full
.claude/cache
# credential files (flagged secret, skipped with --no-secrets)
.claude/.credentials.json
# docker volumes (agent|volume name glob)
agent-zero|*a0_usr
tabby|*tabby_data
"""

LISTING_AFTER = """# home artifacts (agent|glob relative to home)
# Anthropic
claude-code|.claude
claude-code|.claude/projects
# project artifacts (relative to each discovered project)
claude-code|.claude
# excluded unless --full
.claude/cache
.openclaw/*/tmp
# credential files (flagged secret, skipped with --no-secrets)
.claude/.credentials.json
.claude/*.key
# docker volumes (agent|volume name glob)
tabby|*tabby_data
agent-zero|*a0_usr
"""


def manifest_line(path: str, status: str, agent: str, secret: bool, home: str = HOME) -> str:
    """Return one manifest row with the fields the tool reads and one it ignores."""
    row = {"user": "alice", "home": home, "agent": agent, "path": path, "size": 3}
    row.update({"secret": secret, "status": status})
    return json.dumps(row)


def change(path: str, home: str = HOME) -> RowChange:
    """Return a row change of `path`, collected before and absent after."""
    return RowChange(path, (home,), ((home, "collected", "x", False),), ())


def test_parse_tables_keeps_each_table_in_order_without_comments():
    tables = diff_smoke_rows.parse_tables(LISTING_BEFORE)
    assert tables == {
        "home": ["claude-code|.claude", "openclaw|.openclaw"],
        "project": ["claude-code|.claude"],
        "exclude": [".claude/cache"],
        "secret": [".claude/.credentials.json"],
        "docker": ["agent-zero|*a0_usr", "tabby|*tabby_data"],
    }


def test_diff_tables_lists_removed_then_added_lines_per_table():
    changes, reordered = diff_smoke_rows.diff_tables(
        diff_smoke_rows.parse_tables(LISTING_BEFORE), diff_smoke_rows.parse_tables(LISTING_AFTER)
    )
    assert [change.label() for change in changes] == [
        "- CATALOG openclaw|.openclaw",
        "+ CATALOG claude-code|.claude/projects",
        "+ EXCLUDES .openclaw/*/tmp",
        "+ SECRET_GLOBS .claude/*.key",
    ]
    assert reordered


def test_diff_tables_of_one_listing_is_empty():
    tables = diff_smoke_rows.parse_tables(LISTING_BEFORE)
    assert diff_smoke_rows.diff_tables(tables, tables) == ([], False)


def test_read_manifest_takes_the_compared_fields():
    text = "\n".join(
        [
            manifest_line(f"{HOME}/.claude/a", "collected", "claude-code", False),
            manifest_line(f"{HOME}/.claude/.credentials.json", "collected", "claude-code", True),
            "",
        ]
    )
    rows = diff_smoke_rows.read_manifest(text)
    assert rows == [
        Row(f"{HOME}/.claude/a", HOME, "collected", "claude-code", False),
        Row(f"{HOME}/.claude/.credentials.json", HOME, "collected", "claude-code", True),
    ]


def test_read_manifest_raises_on_a_line_that_is_not_json():
    try:
        diff_smoke_rows.read_manifest('{"path": "a"}\n{"path": \n')
    except DiffError as error:
        assert "manifest line 2" in str(error)
    else:
        raise AssertionError("a truncated manifest line was accepted")


def test_diff_rows_reports_status_agent_secret_and_one_sided_paths():
    same = Row("/w/root/same", HOME, "collected", "a", False)
    before = [
        same,
        Row("/w/root/status", HOME, "collected", "a", False),
        Row("/w/root/agent", HOME, "collected", "a", False),
        Row("/w/root/secret", HOME, "collected", "a", False),
        Row("/w/root/gone", HOME, "collected", "a", False),
    ]
    after = [
        same,
        Row("/w/root/status", HOME, "skipped_excluded", "a", False),
        Row("/w/root/agent", HOME, "collected", "b", False),
        Row("/w/root/secret", HOME, "collected", "a", True),
        Row("/w/root/new", HOME, "collected", "a", False),
    ]
    changes = diff_smoke_rows.diff_rows(before, after)
    assert [item.path for item in changes] == [
        "/w/root/agent",
        "/w/root/gone",
        "/w/root/new",
        "/w/root/secret",
        "/w/root/status",
    ]
    gone = changes[1]
    assert gone.before == ((HOME, "collected", "a", False),) and gone.after == ()


def test_diff_rows_reports_facts_that_trade_places_between_two_bases():
    project = f"{HOME}/proj"
    path = f"{project}/tmp"
    before = [
        Row(path, HOME, "collected", "project", False),
        Row(path, project, "skipped_excluded", "project", False),
    ]
    after = [
        Row(path, HOME, "skipped_excluded", "project", False),
        Row(path, project, "collected", "project", False),
    ]
    changes = diff_smoke_rows.diff_rows(before, after)
    assert [(item.path, item.homes) for item in changes] == [(path, (HOME, project))]


def test_a_line_matching_only_under_an_unchanged_base_does_not_attribute():
    project = f"{HOME}/proj"
    path = f"{project}/tmp/x"
    unchanged = Row(path, project, "collected", "project", False)
    before = [Row(path, HOME, "collected", "claude-code", False), unchanged]
    after = [Row(path, HOME, "collected", "codex-cli", False), unchanged]
    changes = diff_smoke_rows.diff_rows(before, after)
    assert [item.homes for item in changes] == [(HOME,)]
    exclusion = TableChange("+", "exclude", "tmp")
    assert diff_smoke_rows.attribute([exclusion], changes) == [(None, changes)]


def test_diff_rows_ignores_row_order_of_a_path_with_two_rows():
    first = Row("/w/root/p", HOME, "collected", "a", False)
    second = Row("/w/root/p", "/w/root/srv/proj", "collected", "project", False)
    assert diff_smoke_rows.diff_rows([first, second], [second, first]) == []


def test_catalog_glob_matches_leading_segments_and_does_not_cross_a_slash():
    line = TableChange("+", "home", "claude-code|.claude/proj*")
    assert diff_smoke_rows.line_matches(line, f"{HOME}/.claude/projects/s1.jsonl", HOME)
    assert diff_smoke_rows.line_matches(line, f"{HOME}/.claude/projects", HOME)
    assert not diff_smoke_rows.line_matches(line, f"{HOME}/.claude", HOME)
    crossing = TableChange("+", "home", "x|.claude*s1.jsonl")
    assert not diff_smoke_rows.line_matches(crossing, f"{HOME}/.claude/projects/s1.jsonl", HOME)


def test_exclusion_glob_crosses_a_slash_and_matches_a_parent_directory():
    line = TableChange("+", "exclude", ".openclaw/*/tmp")
    assert diff_smoke_rows.line_matches(line, f"{HOME}/.openclaw/a/b/tmp", HOME)
    assert diff_smoke_rows.line_matches(line, f"{HOME}/.openclaw/a/tmp/x.log", HOME)
    assert not diff_smoke_rows.line_matches(line, f"{HOME}/.openclaw/tmp", HOME)
    assert not diff_smoke_rows.line_matches(line, "/w/root/home/bob/.openclaw/a/tmp", HOME)


def test_secret_glob_matches_the_relative_path_only():
    line = TableChange("+", "secret", ".claude/*.key")
    assert diff_smoke_rows.line_matches(line, f"{HOME}/.claude/sessions/a.key", HOME)
    assert not diff_smoke_rows.line_matches(line, f"{HOME}/.claude/a.key/inner", HOME)
    assert not diff_smoke_rows.line_matches(line, HOME, HOME)


def test_glob_matches_relative_to_a_project_and_a_volume_base():
    project = "/w/root/srv/proj"
    exclusion = TableChange("+", "exclude", "tmp")
    assert diff_smoke_rows.line_matches(exclusion, f"{project}/tmp/a", project)
    assert diff_smoke_rows.line_matches(exclusion, f"{VOLUME}/tmp/a", VOLUME)
    assert not diff_smoke_rows.line_matches(exclusion, f"{HOME}/.x/tmp/a", HOME)


def test_volume_line_matches_the_volume_name_of_the_base():
    line = TableChange("-", "docker", "tabby|*tabby_data")
    assert diff_smoke_rows.line_matches(line, f"{VOLUME}/events/a.json", VOLUME)
    assert diff_smoke_rows.line_matches(line, VOLUME, VOLUME)
    assert not diff_smoke_rows.line_matches(line, f"{HOME}/tabby_data/a", HOME)


def test_attribute_groups_by_line_and_leaves_the_rest_unattributed():
    exclusion = TableChange("+", "exclude", ".openclaw/*/tmp")
    secret = TableChange("+", "secret", ".openclaw/*/tmp/*")
    unused = TableChange("-", "home", "nobody|.nothing")
    inside = change(f"{HOME}/.openclaw/a/tmp/x")
    elsewhere = change(f"{HOME}/.other/file")
    groups = diff_smoke_rows.attribute([exclusion, secret, unused], [inside, elsewhere])
    assert groups == [(exclusion, [inside]), (secret, [inside]), (None, [elsewhere])]


def test_render_report_prints_the_diff_the_groups_and_the_note():
    exclusion = TableChange("+", "exclude", ".openclaw/*/tmp")
    moved = RowChange(
        f"{HOME}/.openclaw/a/tmp",
        (HOME,),
        ((HOME, "collected", "openclaw", False),),
        ((HOME, "skipped_excluded", "openclaw", False),),
    )
    report = diff_smoke_rows.render_report("main", "HEAD", [exclusion], False, [moved], "/w/root")
    assert report.splitlines() == [
        "table lines changed from main to HEAD:",
        "  + EXCLUDES .openclaw/*/tmp",
        "",
        "paths whose rows changed: 1",
        "",
        "+ EXCLUDES .openclaw/*/tmp",
        "  /home/alice/.openclaw/a/tmp: collected agent=openclaw secret=false base=/home/alice"
        " -> skipped_excluded agent=openclaw secret=false base=/home/alice",
        "",
        diff_smoke_rows.NOTE,
    ]


def test_render_report_of_no_change_says_none_and_the_reordering():
    report = diff_smoke_rows.render_report("main", "HEAD", [], True, [], "/w/root")
    assert "  (none)" in report
    assert "DOCKER_VOLUMES lines reordered" in report
    assert "paths whose rows changed: 0" in report
    assert report.endswith(diff_smoke_rows.NOTE + "\n")


def test_describe_facts_of_a_missing_row():
    assert diff_smoke_rows.describe_facts((), "/w/root") == "(no row)"
    facts = ((VOLUME, "skipped_unmatched_volume", "", False), (HOME, "collected", "a", True))
    assert diff_smoke_rows.describe_facts(facts, "/w/root") == (
        "skipped_unmatched_volume agent=- secret=false"
        " base=/var/lib/docker/volumes/tabby_data/_data;"
        " collected agent=a secret=true base=/home/alice"
    )


def test_collector_command_is_the_smoke_test_run_line():
    environment, arguments = diff_smoke_rows.collector_command(
        "dash", "/s/collect.sh", "/w/root", "/o/base"
    )
    assert environment == {"COLLECTOR_SH": "dash"}
    assert arguments == [
        "dash",
        "/s/collect.sh",
        "-r",
        "/w/root",
        "-o",
        "/o/base",
        "-q",
        "--max-file-size",
        "1",
    ]


def test_image_command_sets_the_switch_and_passes_the_shell():
    environment, arguments = diff_smoke_rows.image_command("dash")
    assert environment == {"SMOKE_IMAGE_ONLY": "1"}
    assert arguments == ["dash", str(diff_smoke_rows.SMOKE_TEST), "dash"]


def test_a_path_under_two_bases_is_matched_relative_to_the_second_base():
    project = "/w/root/home/alice/proj"
    path = f"{project}/tmp/x"
    under_home = Row(path, HOME, "collected", "project", False)
    under_project = Row(path, project, "collected", "project", False)
    excluded = Row(path, project, "skipped_excluded", "project", False)
    changes = diff_smoke_rows.diff_rows([under_home, under_project], [under_home, excluded])
    assert [item.homes for item in changes] == [(project,)]
    exclusion = TableChange("+", "exclude", "tmp")
    assert not diff_smoke_rows.line_matches(exclusion, path, HOME)
    assert diff_smoke_rows.attribute([exclusion], changes) == [(exclusion, changes)]


def fake_image(tmp_path, monkeypatch, after_rows):
    """Stub the image build, the collector writes and runs; return the fake work directory.

    The base collector's manifest has one row; the head's is `after_rows`.
    """
    work = tmp_path / "cac-smoke.test"
    (work / "root").mkdir(parents=True)
    before_rows = [Row("/w/root/a", HOME, "collected", "a", False)]
    outputs = iter([before_rows, after_rows])
    monkeypatch.setattr(diff_smoke_rows, "build_image", lambda shell: work)
    monkeypatch.setattr(diff_smoke_rows, "write_collector", lambda revision, directory, name: name)
    monkeypatch.setattr(diff_smoke_rows, "run_process", lambda arguments, extra: LISTING_BEFORE)
    monkeypatch.setattr(diff_smoke_rows, "collect", lambda *arguments: next(outputs))
    return work


def test_main_exits_0_without_a_change_and_removes_the_image(tmp_path, monkeypatch):
    work = fake_image(tmp_path, monkeypatch, [Row("/w/root/a", HOME, "collected", "a", False)])
    assert diff_smoke_rows.main(["--fail", "main"]) == 0
    assert not work.exists()


def test_main_exits_1_with_fail_only_when_a_row_changed(tmp_path, monkeypatch):
    changed = [Row("/w/root/a", HOME, "skipped_excluded", "a", False)]
    work = fake_image(tmp_path, monkeypatch, changed)
    assert diff_smoke_rows.main(["--fail", "main"]) == 1
    assert not work.exists()
    work = fake_image(tmp_path, monkeypatch, changed)
    assert diff_smoke_rows.main(["main"]) == 0


def test_main_exits_2_and_removes_the_image_when_a_collector_run_fails(tmp_path, monkeypatch):
    work = fake_image(tmp_path, monkeypatch, [])

    def failing_collect(*arguments):
        raise DiffError("collector exited 2")

    monkeypatch.setattr(diff_smoke_rows, "collect", failing_collect)
    assert diff_smoke_rows.main(["main"]) == 2
    assert not work.exists()


def test_build_image_removes_a_printed_work_directory_without_its_image(tmp_path, monkeypatch):
    work = tmp_path / "cac-smoke.empty"
    work.mkdir()
    monkeypatch.setattr(diff_smoke_rows, "run_process", lambda arguments, extra: f"{work}\n")
    try:
        diff_smoke_rows.build_image("dash")
    except DiffError:
        pass
    else:
        raise AssertionError("a work directory without root/ was accepted")
    assert not work.exists()


def test_build_image_returns_a_printed_work_directory_with_its_image(tmp_path, monkeypatch):
    work = tmp_path / "cac-smoke.full"
    (work / "root").mkdir(parents=True)
    printed = f"note: no socket\n{work}\n"
    monkeypatch.setattr(diff_smoke_rows, "run_process", lambda arguments, extra: printed)
    assert diff_smoke_rows.build_image("dash") == work
    assert (work / "root").is_dir()


def test_build_image_never_removes_a_printed_path_outside_a_smoke_work_directory(
    tmp_path, monkeypatch
):
    other = tmp_path / "somewhere"
    other.mkdir()
    monkeypatch.setattr(diff_smoke_rows, "run_process", lambda arguments, extra: f"{other}\n")
    try:
        diff_smoke_rows.build_image("dash")
    except DiffError:
        pass
    else:
        raise AssertionError("a directory without the cac-smoke. prefix was accepted")
    assert other.is_dir()


def test_main_exits_2_when_the_shell_does_not_exist(tmp_path):
    missing = str(tmp_path / "no-such-shell")
    assert diff_smoke_rows.main(["--shell", missing, "main"]) == 2
