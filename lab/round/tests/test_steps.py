"""Tests of steps.py, the checker of an agent's round definition."""

from __future__ import annotations

import pathlib

import steps

ROUND = pathlib.Path(__file__).resolve().parent.parent


def problems_of(text: str, tmp_path: pathlib.Path) -> list[str]:
    parsed, bad = steps.parse_steps(text)
    bad += steps.check_steps(parsed, [tmp_path])
    return [message for _, message in bad]


def line(*fields: str) -> str:
    return "\t".join(fields) + "\n"


def test_good_table_has_no_problem(tmp_path):
    (tmp_path / "short.txt").write_text("x", encoding="utf-8")
    text = (
        "NN\tname\tkind\ttext\texpect\tclaim\n"
        "# a comment\n"
        + line("02", "launch", "agent", "in:. gemini --worktree lab", "quiet", "c")
        + line("03", "ask", "prompt", "what is 17 + 25?", "Allow once|Type your message", "c")
        + line("04", "keys", "keys", "Down,Shift+Tab,Ctrl+C,F12", "-", "c")
        + line("05", "paste", "paste", "short.txt Summarise it.", "marker", "c")
        + line("06", "stop", "stop", "-", "-", "c")
        + line("07", "wait", "idle", "60", "none", "c")
        + line("08", "run", "command", "ls", "-", "c")
        + line("10", "boot", "reboot", "-", "-", "c")
    )
    assert problems_of(text, tmp_path) == []


def test_numbers_must_rise_from_two(tmp_path):
    text = line("01", "a", "idle", "1", "-", "c") + line("05", "b", "idle", "1", "-", "c")
    text += line("05", "c", "idle", "1", "-", "c") + line("7", "d", "idle", "1", "-", "c")
    found = problems_of(text, tmp_path)
    assert found == [
        "number 01 does not rise from 01",
        "number 05 does not rise from 05",
        "number '7' is not two or three digits",
    ]


def test_every_field_present(tmp_path):
    found = problems_of("02\tname\tprompt\thi\t-\n02\tname\tprompt\t \t-\tc\n", tmp_path)
    assert found == ["want six non-empty tab-separated fields"] * 2


def test_session_needed_before_typing(tmp_path):
    text = line("02", "early", "prompt", "hi", "-", "c")
    text += line("03", "start", "agent", "gemini", "-", "c") + line(
        "04", "end", "stop", "-", "-", "c"
    )
    text += line("05", "late", "keys", "Enter", "-", "c") + line(
        "06", "again", "stop", "-", "-", "c"
    )
    found = problems_of(text, tmp_path)
    assert found == [
        "prompt with no agent session open",
        "keys with no agent session open",
        "stop with no agent session open",
    ]


def test_reboot_closes_the_session(tmp_path):
    text = line("02", "start", "agent", "gemini", "-", "c") + line(
        "03", "boot", "reboot", "-", "-", "c"
    )
    text += line("04", "after", "type", "?", "quiet", "c")
    assert problems_of(text, tmp_path) == ["type with no agent session open"]


def test_kind_and_text_rules(tmp_path):
    text = line("02", "start", "agent", "in:lab/proj", "-", "c")
    text += line("03", "bad-kind", "dance", "x", "-", "c")
    text += line("04", "bad-idle", "idle", "soon", "-", "c")
    text += line("05", "bad-key", "keys", "Down,Hyper+Q", "-", "c")
    text += line("06", "no-paste", "paste", "missing.txt", "-", "c")
    text += line("07", "stop-text", "stop", "now", "-", "c")
    text += line("08", "Bad_Name", "idle", "1", "-", "c")
    assert problems_of(text, tmp_path) == [
        "agent needs a command after in:lab/proj",
        "unknown kind 'dance'",
        "idle takes a number of seconds",
        "unknown key name(s): Hyper+Q",
        "no paste file missing.txt",
        "stop takes `-` as its text",
        "name 'Bad_Name' is not lower-case words joined by hyphens",
    ]


def test_expect_must_be_portable(tmp_path):
    text = line("02", "start", "agent", "gemini", r"\d+ files", "c")
    text += line("03", "open", "agent", "gemini", "(unclosed", "c")
    found = problems_of(text, tmp_path)
    assert found[0] == "expect '\\\\d+ files' uses an escape grep -E does not read"
    assert found[1].startswith("expect '(unclosed' is not a regular expression")


def test_markdown_table_escapes_pipes():
    parsed, _ = steps.parse_steps(line("02", "ask", "prompt", "a | b", "-", "x | y"))
    table = steps.markdown_table(parsed)
    assert "| 02 | ask | prompt | `a \\| b` | x \\| y |" in table


def test_shipped_agents_pass():
    for agent_dir in sorted((ROUND / "agents").iterdir()):
        _, found = steps.load(agent_dir)
        assert found == [], agent_dir.name


def test_missing_files_are_reported(tmp_path):
    _, found = steps.load(tmp_path)
    assert len(found) == len(steps.REQUIRED_FILES)


def test_one_number_width_per_table(tmp_path):
    text = line("98", "a", "idle", "1", "-", "c") + line("099", "b", "idle", "1", "-", "c")
    text += line("100", "c", "idle", "1", "-", "c")
    found = problems_of(text, tmp_path)
    assert found == [
        "number 099 is not 2 digits wide like the first step's",
        "number 100 is not 2 digits wide like the first step's",
    ]


def test_renumber_shifts_from_the_inserted_line():
    text = "NN\tname\n# note\n" + line("02", "a", "idle", "1", "-", "c")
    text += line("03", "ask", "prompt", "hi", "-", "c") + line(
        "03", "ask-again", "prompt", "hi!", "-", "c"
    )
    text += line("04", "next", "idle", "1", "-", "c") + line("05", "last", "idle", "1", "-", "c")
    numbers = [row.split("\t")[0] for row in steps.renumber(text).split("\n")[2:] if row]
    assert numbers == ["02", "03", "04", "05", "06"]
    assert steps.renumber(steps.renumber(text)) == steps.renumber(text)
