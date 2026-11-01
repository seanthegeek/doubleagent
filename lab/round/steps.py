#!/usr/bin/env python3
"""Check an agent's round definition and print its step table.

    python3 -I steps.py check [--steps CELL/steps.tsv] agents/<agent>
    python3 -I steps.py table agents/<agent>
    python3 -I steps.py renumber CELL/steps.tsv

An agent's directory holds `agent.conf`, `install.sh`, `roots` and `steps.tsv`; a step line is
`NN<TAB>name<TAB>kind<TAB>text<TAB>expect<TAB>claim`, and lines starting with `#` and the header
line (`NN` first) are skipped. `check` prints one `<file>:<line>: <problem>` per problem and exits
1 when there is any; `table` prints the steps as a Markdown table; `renumber` rewrites a steps file
so that a line inserted after a step, numbered like it, and every line after it rise by one. The
rules are the ones round.sh relies on: every field present, numbers zero-padded to one width and
rising from 02 (00 and 01 are the baseline and install listings), a known kind with the text it
needs, a session open for every step that types into one, and an expectation grep -E, PowerShell
and Python all read the same way.

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
    "paste",
    "pause",
    "prompt",
    "reboot",
    "stop",
    "type",
)
NEEDS_SESSION = {"keys", "paste", "prompt", "type"}
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


def renumber(source: str) -> str:
    """Return a steps file's text renumbered from the first number that does not rise.

    The numbers before that line keep theirs, so steps that have run keep their listing names;
    from it on each step takes the previous number plus one, at the table's width.
    """
    lines = source.split("\n")
    previous = 1
    width = 0
    renumbering = False
    for index, line in enumerate(lines):
        if not line.strip() or line.startswith("#") or line.startswith("NN\t"):
            continue
        number, separator, rest = line.partition("\t")
        if not separator or not number.strip().isdigit():
            continue
        width = width or len(number.strip())
        if renumbering or int(number) <= previous:
            renumbering = True
            number = "%0*d" % (width, previous + 1)
        previous = int(number)
        lines[index] = number + separator + rest
    return "\n".join(lines)


def main(arguments: list[str] | None = None) -> int:
    """Run the command line; return the exit status."""
    parser = argparse.ArgumentParser(description=(__doc__ or "").split("\n")[0])
    parser.add_argument("command", choices=("check", "table", "renumber"))
    parser.add_argument("--steps", type=pathlib.Path, help="a steps file other than the agent's")
    parser.add_argument(
        "path", type=pathlib.Path, help="an agent directory, or for renumber a file"
    )
    options = parser.parse_args(arguments)
    if options.command == "renumber":
        options.path.write_text(
            renumber(options.path.read_text(encoding="utf-8")), encoding="utf-8"
        )
        return 0
    steps, problems = load(options.path, options.steps)
    if options.command == "table":
        sys.stdout.write(markdown_table(steps))
    for problem in problems:
        print(problem, file=sys.stderr)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
