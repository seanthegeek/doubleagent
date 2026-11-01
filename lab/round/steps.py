#!/usr/bin/env python3
"""Check an agent's round definition and print its step table.

    python3 -I steps.py check [--steps CELL/steps.tsv] agents/<agent>
    python3 -I steps.py table agents/<agent>
    python3 -I steps.py renumber CELL

An agent's directory holds `agent.conf`, `install.sh`, `roots` and `steps.tsv`; a step line is
`NN<TAB>name<TAB>kind<TAB>text<TAB>expect<TAB>claim`, and lines starting with `#` and the header
line (`NN` first) are skipped. `check` prints one `<file>:<line>: <problem>` per problem and exits
1 when there is any; `table` prints the steps as a Markdown table; `renumber` moves a cell's
repeat lines (copies of a missed step, numbered like it) after the last step that ran and points
the cell's `next` at them. The rules are the ones round.sh relies on: every field present, numbers
zero-padded to one width and rising from 02 (00 and 01 are the baseline and install listings), a
known kind with the text it needs, a session open for every step that types into one, and an
expectation grep -E, PowerShell and Python all read the same way.

Standard library only.
"""

from __future__ import annotations

import argparse
import dataclasses
import pathlib
import re
import sys

KINDS = (
    "agent",
    "collect",
    "command",
    "idle",
    "keys",
    "options",
    "paste",
    "pause",
    "prompt",
    "reboot",
    "stop",
    "type",
)
NEEDS_SESSION = {"keys", "options", "paste", "prompt", "type"}
NO_TEXT = {"collect", "reboot", "stop"}
EXPECT_WORDS = {"-", "exit", "marker", "none", "quiet"}
KEY_NAMES = (
    "Up|Down|Left|Right|Home|End|PageUp|PageDown|Insert|Delete|Backspace|Tab|Enter|Space|Escape"
    "|F[1-9]|F1[0-2]|[A-Za-z0-9]"
)
KEY_RE = re.compile(r"(?:(?:Ctrl|Shift|Alt|Meta)\+)*(?:%s)" % KEY_NAMES)
NUMBER_RE = re.compile(r"\d{2,3}")
NAME_RE = re.compile(r"[a-z0-9]+(?:-[a-z0-9]+)*")
# Escapes that grep -E, .NET and Python do not agree on.
NON_PORTABLE_RE = re.compile(r"\\[dDwWsSbB]")
REQUIRED_FILES = ("agent.conf", "install.sh", "roots", "steps.tsv")


@dataclasses.dataclass(frozen=True)
class Step:
    """One line of a steps file."""

    line: int
    number: str
    name: str
    kind: str
    text: str
    expect: str
    claim: str

    @property
    def label(self) -> str:
        """Return the listing name, `NN-name`."""
        return "%s-%s" % (self.number, self.name)


def parse_steps(source: str) -> tuple[list[Step], list[tuple[int, str]]]:
    """Return the steps of a steps file's text and the (line, problem) pairs of its bad lines."""
    steps: list[Step] = []
    problems: list[tuple[int, str]] = []
    for line_number, line in enumerate(source.split("\n"), start=1):
        line = line.rstrip("\r")
        if not line.strip() or line.startswith("#") or line.startswith("NN\t"):
            continue
        fields = line.split("\t")
        if len(fields) != 6 or any(not field.strip() for field in fields):
            problems.append((line_number, "want six non-empty tab-separated fields"))
            continue
        steps.append(Step(line_number, *(field.strip() for field in fields)))
    return steps, problems


def expect_problem(expect: str) -> str | None:
    """Return why an expectation cannot be used, or None when it can."""
    if expect in EXPECT_WORDS:
        return None
    if NON_PORTABLE_RE.search(expect):
        return "expect %r uses an escape grep -E does not read" % expect
    try:
        re.compile(expect)
    except re.error as error:
        return "expect %r is not a regular expression: %s" % (expect, error)
    return None


def text_problem(step: Step, paste_dirs: list[pathlib.Path]) -> str | None:
    """Return why a step's text does not fit its kind, or None when it does."""
    if step.kind in NO_TEXT:
        return None if step.text == "-" else "%s takes `-` as its text" % step.kind
    if step.kind == "idle":
        if not step.text.isdigit() or not 0 < int(step.text) <= 86400:
            return "idle takes a number of seconds"
    elif step.kind == "options":
        if not step.text.isdigit() or not 0 < int(step.text) <= 30:
            return "options takes the most Down presses, 1 to 30"
    elif step.kind == "keys":
        bad = [key for key in step.text.split(",") if not KEY_RE.fullmatch(key)]
        if bad:
            return "unknown key name(s): %s" % ", ".join(bad)
    elif step.kind == "paste":
        file_name = step.text.split(" ", 1)[0]
        if not any((directory / file_name).is_file() for directory in paste_dirs):
            return "no paste file %s" % file_name
    elif step.kind == "agent":
        words = step.text.split()
        if words[0].startswith("in:") and len(words) < 2:
            return "agent needs a command after %s" % words[0]
    return None


def check_steps(steps: list[Step], paste_dirs: list[pathlib.Path]) -> list[tuple[int, str]]:
    """Return the (line, problem) pairs of steps that break a rule round.sh relies on."""
    problems: list[tuple[int, str]] = []
    previous = 1
    width = 0
    session_open = False
    for step in steps:
        found: list[str] = []
        if not NUMBER_RE.fullmatch(step.number):
            found.append("number %r is not two or three digits" % step.number)
        elif width and len(step.number) != width:
            found.append(
                "number %s is not %d digits wide like the first step's" % (step.number, width)
            )
        elif int(step.number) <= previous:
            found.append("number %s does not rise from %02d" % (step.number, previous))
        else:
            previous = int(step.number)
            width = width or len(step.number)
        if not NAME_RE.fullmatch(step.name):
            found.append("name %r is not lower-case words joined by hyphens" % step.name)
        if step.kind not in KINDS:
            found.append("unknown kind %r" % step.kind)
        else:
            message = text_problem(step, paste_dirs)
            if message:
                found.append(message)
        message = expect_problem(step.expect)
        if message:
            found.append(message)
        if step.kind in NEEDS_SESSION | {"stop"} and not session_open:
            found.append("%s with no agent session open" % step.kind)
        if step.kind == "agent":
            session_open = True
        elif step.kind in ("stop", "reboot"):
            session_open = False
        problems.extend((step.line, message) for message in found)
    return problems


def load(
    agent_dir: pathlib.Path, steps_file: pathlib.Path | None = None
) -> tuple[list[Step], list[str]]:
    """Return an agent directory's steps and every problem found in it, as printable lines.

    `steps_file` replaces the directory's own `steps.tsv`, as a cell's copy does.
    """
    problems = [
        "%s: missing" % (agent_dir / name)
        for name in REQUIRED_FILES
        if not (agent_dir / name).is_file()
    ]
    steps_file = steps_file or agent_dir / "steps.tsv"
    if not steps_file.is_file():
        return [], problems
    steps, bad_lines = parse_steps(steps_file.read_text(encoding="utf-8"))
    paste_dirs = [agent_dir / "pastes", agent_dir.parent.parent / "pastes"]
    bad_lines += check_steps(steps, paste_dirs)
    problems += ["%s:%d: %s" % (steps_file, line, message) for line, message in sorted(bad_lines)]
    return steps, problems


def markdown_table(steps: list[Step]) -> str:
    """Return the steps as a Markdown table of number, name, kind, text and claim."""
    rows = ["| NN | Step | Kind | Text | Claim tested |", "| --- | --- | --- | --- | --- |"]
    for step in steps:
        text = step.text.replace("|", "\\|")
        claim = step.claim.replace("|", "\\|")
        rows.append(
            "| %s | %s | %s | `%s` | %s |" % (step.number, step.name, step.kind, text, claim)
        )
    return "\n".join(rows) + "\n"


class RenumberError(ValueError):
    """A steps file that `renumber` refuses to rewrite."""


def ran_numbers(steps_log: str) -> set[int]:
    """Return the numbers of the steps a cell's steps.log records, run or missed."""
    numbers = set()
    for line in steps_log.split("\n"):
        number = line.split("-", 1)[0]
        if number.isdigit():
            numbers.add(int(number))
    return numbers


def renumber(source: str, ran: set[int]) -> tuple[str, str]:
    """Return a cell's steps file with its repeat lines moved after the last step that ran.

    A repeat line is one whose number does not rise above the line before it (a copy of a missed
    step, numbered like it). Steps at or below the last number in `ran` keep their numbers; the
    repeats, then every step not yet run, take the numbers after it. Comment lines move to the
    top. Returns the text and the first repeat's number as written.

    Raises:
        RenumberError: no step has run, there is no repeat line, or a number outgrows the width.
    """
    if not ran:
        raise RenumberError("no step has run yet, so there is nothing to repeat")
    last_ran = max(ran)
    kept_lines: list[str] = []
    steps_kept: list[tuple[int, str]] = []
    repeats: list[str] = []
    previous = 1
    width = 0
    for line in source.split("\n"):
        number, separator, rest = line.partition("\t")
        if line.startswith("#") or line.startswith("NN\t") or not separator or not number.isdigit():
            if line.strip():
                kept_lines.append(line)
            continue
        width = width or len(number)
        if int(number) <= previous:
            repeats.append(rest)
            continue
        previous = int(number)
        steps_kept.append((int(number), rest))
    if not repeats:
        raise RenumberError("no repeat line: copy the missed step's line, keeping its number")
    done = ["%0*d\t%s" % (width, number, rest) for number, rest in steps_kept if number <= last_ran]
    later = repeats + [rest for number, rest in steps_kept if number > last_ran]
    first = last_ran + 1
    if first + len(later) - 1 >= 10**width:
        raise RenumberError("the numbers would outgrow the table's width of %d digits" % width)
    renumbered = ["%0*d\t%s" % (width, first + offset, rest) for offset, rest in enumerate(later)]
    return "\n".join(kept_lines + done + renumbered) + "\n", "%0*d" % (width, first)


def renumber_cell(cell: pathlib.Path) -> str:
    """Rewrite a cell's steps.tsv with its repeat steps next, set `next` to the first; return it.

    Raises:
        RenumberError: the cell waits for a person step, or `renumber` refuses the file.
    """
    state = (cell / "state").read_text(encoding="utf-8") if (cell / "state").is_file() else ""
    paused = [line[7:] for line in state.split("\n") if line.startswith("paused=")]
    if paused and paused[-1]:
        raise RenumberError("step %s waits for the person: resume first" % paused[-1])
    log = cell / "steps.log"
    ran = ran_numbers(log.read_text(encoding="utf-8")) if log.is_file() else set()
    steps_file = cell / "steps.tsv"
    text, label = renumber(steps_file.read_text(encoding="utf-8"), ran)
    steps_file.write_text(text, encoding="utf-8")
    with (cell / "state").open("a", encoding="utf-8") as handle:
        handle.write("next=%s\n" % label)
    return label


def main(arguments: list[str] | None = None) -> int:
    """Run the command line; return the exit status."""
    parser = argparse.ArgumentParser(description=(__doc__ or "").split("\n")[0])
    parser.add_argument("command", choices=("check", "table", "renumber"))
    parser.add_argument("--steps", type=pathlib.Path, help="a steps file other than the agent's")
    parser.add_argument(
        "path", type=pathlib.Path, help="an agent directory, or for renumber a cell directory"
    )
    options = parser.parse_args(arguments)
    if options.command == "renumber":
        try:
            label = renumber_cell(options.path)
        except RenumberError as error:
            print("renumber: %s" % error, file=sys.stderr)
            return 1
        print("renumber: the repeat is step %s; resume runs it next" % label)
        return 0
    steps, problems = load(options.path, options.steps)
    if options.command == "table":
        sys.stdout.write(markdown_table(steps))
    for problem in problems:
        print(problem, file=sys.stderr)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
