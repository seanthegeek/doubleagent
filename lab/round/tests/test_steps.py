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
    text += line("09", "bad-secret", "secret", "gemini key", "-", "c")
    assert problems_of(text, tmp_path) == [
        "agent needs a command after in:lab/proj",
        "unknown kind 'dance'",
        "idle takes a number of seconds",
        "unknown key name(s): Hyper+Q",
        "no paste file missing.txt",
        "stop takes `-` as its text",
        "name 'Bad_Name' is not lower-case words joined by hyphens",
        "secret takes the upper-case name of a variable of the credentials file",
        "secret with no agent session open",
    ]


def test_secret_kind(tmp_path):
    text = line("02", "key", "secret", "GEMINI_API_KEY", "-", "c")
    assert problems_of(text, tmp_path) == ["secret with no agent session open"]
    text = line("02", "start", "agent", "gemini", "-", "c") + text.replace("02", "03")
    assert problems_of(text, tmp_path) == []


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


def simulated_cell(tmp_path: pathlib.Path) -> pathlib.Path:
    """02 ran, 03 missed, 04 and 05 ran, 06 not yet; a repeat of 03 copied just after it."""
    table = "NN\tname\tkind\ttext\texpect\tclaim\n" + line(
        "02", "start", "agent", "gemini", "-", "c"
    )
    table += line("03", "ask", "prompt", "hi", "-", "c")
    table += line("03", "ask-again", "prompt", "hi, and say yes", "-", "c")
    table += line("04", "next", "prompt", "go", "-", "c") + line(
        "05", "more", "prompt", "on", "-", "c"
    )
    table += line("06", "later", "stop", "-", "-", "c")
    (tmp_path / "steps.tsv").write_text(table, encoding="utf-8")
    (tmp_path / "steps.log").write_text(
        "02-start\tagent\tdone\n03-ask\tprompt\tmissed\n"
        "04-next\tprompt\tdone\n05-more\tprompt\tdone\n",
        encoding="utf-8",
    )
    (tmp_path / "state").write_text("next=06\npaused=\n", encoding="utf-8")
    return tmp_path


def test_renumber_puts_the_repeat_after_the_last_step_that_ran(tmp_path):
    cell = simulated_cell(tmp_path)
    assert steps.renumber_cell(cell) == "06"
    rows = [row.split("\t")[:2] for row in (cell / "steps.tsv").read_text().split("\n")[1:] if row]
    assert rows == [
        ["02", "start"],
        ["03", "ask"],
        ["04", "next"],
        ["05", "more"],
        ["06", "ask-again"],
        ["07", "later"],
    ]
    assert (cell / "state").read_text().split("\n")[-2] == "next=06"
    parsed, problems = steps.parse_steps((cell / "steps.tsv").read_text())
    assert problems == [] and steps.check_steps(parsed, []) == []


def test_renumber_refusals(tmp_path):
    cell = simulated_cell(tmp_path)
    (cell / "state").write_text("paused=03\n", encoding="utf-8")
    cases = [(cell, "waits for the person")]
    for case, message in cases:
        try:
            steps.renumber_cell(case)
        except steps.RenumberError as error:
            assert message in str(error), error
        else:
            raise AssertionError("no refusal: %s" % message)
    text = line("02", "a", "idle", "1", "-", "c") + line("03", "b", "idle", "1", "-", "c")
    for ran, source, message in (
        (set(), text, "no step has run"),
        ({2}, text, "no repeat line"),
        (
            {98},
            line("98", "a", "idle", "1", "-", "c")
            + line("98", "b", "idle", "1", "-", "c")
            + line("99", "c", "idle", "1", "-", "c"),
            "outgrow",
        ),
    ):
        try:
            steps.renumber(source, ran)
        except steps.RenumberError as error:
            assert message in str(error), error
        else:
            raise AssertionError("no refusal: %s" % message)


def test_options_kind(tmp_path):
    text = line("02", "start", "agent", "gemini", "-", "c") + line(
        "03", "read", "options", "12", "-", "c"
    )
    text += line("04", "bad", "options", "many", "-", "c")
    assert problems_of(text, tmp_path) == ["options takes the most Down presses, 1 to 30"]


def test_dialog_and_choose_kinds(tmp_path):
    text = line("02", "start", "agent", "gemini", "Trust folder", "c")
    text += line("03", "trust", "dialog", "Trust folder", "-", "c")
    text += line("04", "web", "dialog", "-", "marker", "c")
    text += line("05", "grant", "choose", "Allow for this session", "-", "c")
    text += line("06", "grant-enter", "keys", "Enter", "marker", "c")
    assert problems_of(text, tmp_path) == []
    text += line("07", "no-text", "choose", "-", "-", "c")
    text += line("08", "bad-dialog", "dialog", "(open", "-", "c")
    text += line("09", "bad-choose", "choose", r"\d option", "-", "c")
    found = problems_of(text, tmp_path)
    assert found[0] == "choose takes the option's text as a grep -E pattern"
    assert found[1].startswith("dialog pattern '(open' is not a regular expression")
    assert found[2] == "choose pattern '\\\\d option' uses an escape grep -E does not read"
    assert len(found) == 3


def test_dialog_and_choose_need_a_session(tmp_path):
    text = line("02", "early", "dialog", "-", "-", "c") + line(
        "03", "pick", "choose", "Yes", "-", "c"
    )
    assert problems_of(text, tmp_path) == [
        "dialog with no agent session open",
        "choose with no agent session open",
    ]


def test_a_slash_command_needs_its_subcommand_step_first(tmp_path):
    text = line("02", "start", "agent", "gemini", "-", "c")
    text += line("03", "early", "prompt", "/auth signout", "-", "c")
    text += line("04", "sign-in", "pause", "sign in with /login, then /exit", "-", "c")
    text += line("05", "chat", "type", "/chat", "quiet", "c")
    assert problems_of(text, tmp_path) == [
        "/auth is used before a `type` step of '/auth ' reads its subcommands",
        "/login is used before a `type` step of '/login ' reads its subcommands",
        "/exit is used before a `type` step of '/exit ' reads its subcommands",
        "/chat is used before a `type` step of '/chat ' reads its subcommands",
    ]


def test_a_subcommand_step_lets_the_command_through(tmp_path):
    text = line("02", "start", "agent", "gemini", "-", "c") + "".join(
        line("%02d" % (3 + 2 * offset), name + "-subcommands", "type", "/%s " % name, "quiet", "c")
        + line("%02d" % (4 + 2 * offset), name + "-subcommands-clear", "clear", "-", "quiet", "c")
        for offset, name in enumerate(("auth", "login", "exit", "chat"))
    )
    text += line("11", "sign-out", "prompt", "/auth signout", "-", "c")
    text += line(
        "12", "sign-in", "pause", "run ~/lab/tui-test/tui-test, sign in with /login", "-", "c"
    )
    text += line("13", "leave", "pause", "type /exit in $HOME/lab/proj", "-", "c")
    text += line("14", "chat", "type", "/chat", "quiet", "c")
    text += line("15", "windows", "pause", "open C:/tools/run, then resume", "-", "c")
    assert problems_of(text, tmp_path) == []


def test_a_subcommand_step_needs_a_clear_step_right_after(tmp_path):
    text = line("02", "start", "agent", "gemini", "-", "c")
    text += line("03", "auth-subcommands", "type", "/auth ", "quiet", "c")
    text += line("04", "help", "prompt", "what is 1 + 1?", "-", "c")
    text += line("05", "model-subcommands", "type", "/model ", "quiet", "c")
    assert problems_of(text, tmp_path) == [
        "'/auth ' is not followed by a `clear` step",
        "'/model ' is not followed by a `clear` step",
    ]


def test_a_drive_letter_is_no_slash_command(tmp_path):
    text = line("02", "start", "agent", "gemini", "-", "c")
    text += line("03", "run", "pause", "open c:/tools and then /users", "-", "c")
    assert problems_of(text, tmp_path) == [
        "/users is used before a `type` step of '/users ' reads its subcommands"
    ]


def test_a_type_step_keeps_its_trailing_space():
    parsed, _ = steps.parse_steps(line("02", "auth-subcommands", "type", "/auth ", "quiet", "c"))
    assert parsed[0].text == "/auth "


def test_clear_kind(tmp_path):
    text = line("02", "start", "agent", "gemini", "-", "c")
    text += line("03", "early-clear", "clear", "-", "-", "c")
    text += line("04", "typed", "type", "?", "quiet", "c")
    text += line("05", "after", "clear", "-", "quiet", "c")
    text += line("06", "counted", "clear", "4", "quiet", "c")
    text += line("07", "bad", "clear", "many", "quiet", "c")
    assert problems_of(text, tmp_path) == [
        "clear `-` with no `type` step before it",
        "clear takes `-` or the number of Backspace presses, 1 to 200",
    ]
    assert problems_of(line("02", "lonely", "clear", "2", "-", "c"), tmp_path) == [
        "clear with no agent session open"
    ]


def test_agent_takes_environment_assignments_before_its_command(tmp_path):
    for text in ("NAME=value gemini", "in:. GEMINI_API_KEY='secret:GEMINI_API_KEY' gemini"):
        steps_text = line("02", "start", "agent", text, "-", "c")
        assert problems_of(steps_text, tmp_path) == [], text
    text = line("02", "start", "agent", "NAME=value", "-", "c")
    text += line("03", "here", "agent", "in:. A=1 B=2", "-", "c")
    assert problems_of(text, tmp_path) == [
        "agent needs a command after NAME=value",
        "agent needs a command after B=2",
    ]


def test_agent_assignments_are_shell_words(tmp_path):
    accepted = ["A='x y' gemini", 'A="x y" B=1 gemini', "in:. A=x\\ y gemini"]
    for text in accepted:
        steps_text = line("02", "start", "agent", text, "-", "c")
        assert problems_of(steps_text, tmp_path) == [], text
    text = line("02", "start", "agent", "API_KEY='secret:API_KEY' LABEL='two words'", "-", "c")
    text += line("03", "here", "agent", "in:. A=x\\ y", "-", "c")
    text += line("04", "open", "agent", "A='unterminated gemini", "-", "c")
    assert problems_of(text, tmp_path) == [
        "agent needs a command after LABEL=two words",
        "agent needs a command after A=x y",
        "agent text is not shell words (No closing quotation)",
    ]


def test_secret_tokens_name_an_upper_case_variable(tmp_path):
    text = line("02", "start", "agent", "KEY=secret:lower gemini", "-", "c")
    text += line("03", "write", "command", "echo secret:API_KEY secret: >f", "-", "c")
    text += line("04", "fine", "command", "echo secret:API_KEY_2 >f", "-", "c")
    text += line("05", "said", "prompt", "what is secret:lower?", "-", "c")
    assert problems_of(text, tmp_path) == [
        "secret:lower is not the upper-case name of a variable of the credentials file",
        "secret: is not the upper-case name of a variable of the credentials file",
    ]
