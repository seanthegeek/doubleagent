"""Tests of report.py, the Markdown report of a round cell."""

from __future__ import annotations

import json
import pathlib

import report

SESSION = "1f0e9a2c-3b4d-4e5f-8a9b-0c1d2e3f4a5b"


def write_jsonl(path: pathlib.Path, rows: list[dict[str, object]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("".join(json.dumps(row) + "\n" for row in rows), encoding="utf-8")


def make_cell(cell: pathlib.Path) -> None:
    (cell / "listings").mkdir(parents=True)
    (cell / "state").write_text(
        "agent=gemini-cli\nvm=Ubuntu\nversion=0.66.0\npaused_label=06-sign-in\npaused_label=\n",
        encoding="utf-8",
    )
    (cell / "steps.log").write_text(
        "02-launch\tagent\tdone\tt1\tt2\t\n03-help\tprompt\tmissed\tt2\tt3\texpectation not met\n",
        encoding="utf-8",
    )
    (cell / "listings" / "01-install.txt").write_text("", encoding="utf-8")
    (cell / "listings" / "02-launch.txt").write_text(
        "file\t9\t644\tt\t/home/sean/.gemini/tmp/proj/logs.json\n", encoding="utf-8"
    )
    collection = cell / "collection" / "53-collect-before-delete"
    write_jsonl(
        collection / "host_x_agent-artifacts.manifest.jsonl",
        [
            {
                "path": "/home/sean/.gemini/settings.json",
                "status": "collected",
                "secret": True,
                "type": "file",
                "size": 9,
            },
            {
                "path": "/home/sean/.gemini/oauth_token.json",
                "status": "collected",
                "secret": False,
                "type": "file",
                "size": 5,
            },
            {
                "path": "/home/sean/.gemini/tmp/proj/%s.json" % SESSION,
                "status": "skipped_excluded",
                "type": "dir",
                "size": 3,
            },
            {"path": "/root", "status": "error_read", "type": "dir", "size": 0},
        ],
    )
    write_jsonl(
        cell / "analysis" / "53-collect-before-delete" / "timeline.jsonl",
        [
            {
                "agent": "gemini-cli",
                "session_id": SESSION,
                "turn_type": "user",
                "text": "what is 17 + 25?",
            },
            {
                "agent": "gemini-cli",
                "session_id": SESSION,
                "turn_type": "system",
                "text": "model: gemini-x",
            },
            {
                "agent": "gemini-cli",
                "session_id": "",
                "turn_type": "system",
                "text": "parser: unparseable line (bad): secret text here",
            },
        ],
    )


def test_report_sections(tmp_path):
    make_cell(tmp_path)
    text = report.build_report(tmp_path)
    assert text.startswith("# Round cell gemini-cli on Ubuntu\n")
    assert "- version: 0.66.0" in text
    assert "- waiting for the person at: nothing" in text
    assert "| 02-launch | agent | done | +1 -0 ~0 | `~/.gemini/tmp` 1 |" in text
    assert "- 03-help (prompt): missed expectation not met" in text
    assert "Rows per status: collected 2, error_read 1, skipped_excluded 1" in text
    assert "Error rows: 1\n- `/root` dir 0" in text
    assert "Unflagged-key rows: 1\n- `/home/sean/.gemini/oauth_token.json` file 5" in text
    assert "| gemini-cli | S1 | 2 |" in text
    assert "| `system model:` | 1 | yes |" in text


def test_report_prints_no_session_id_or_parser_line_text(tmp_path):
    make_cell(tmp_path)
    text = report.build_report(tmp_path)
    assert SESSION not in text
    assert "secret text here" not in text
    assert "- parser: unparseable line (bad) ..." in text


def test_row_forms():
    cases = [
        ({"turn_type": "user", "text": "hi: there"}, "user"),
        (
            {"turn_type": "system", "text": "permission decision: cancelled"},
            "system permission decision:",
        ),
        (
            {"turn_type": "system", "text": "session start: kind=main\nmore"},
            "system session start:",
        ),
        (
            {"turn_type": "system", "text": "a very long sentence without any colon at all"},
            "system a very",
        ),
    ]
    for row, expected in cases:
        assert report.row_form(row) == expected, row


def test_new_forms_are_marked_against_earlier_collections(tmp_path):
    first = tmp_path / "first.jsonl"
    write_jsonl(first, [{"turn_type": "user", "session_id": "a"}])
    lines, forms = report.analysis_lines(first, set())
    assert "| `user` | 1 | yes |" in lines
    lines, _ = report.analysis_lines(first, forms)
    assert "| `user` | 1 |  |" in lines


def test_empty_cell(tmp_path):
    text = report.build_report(tmp_path)
    assert "No collection yet" in text
    assert "- none" in text
