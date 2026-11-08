#!/usr/bin/env python3
"""Count the processes the sh collector forks per manifest row and compare them with the budget.

A `$(...)`, a pipeline element or an external command added on a per-file path changes no
output, so the smoke tests pass either way; only the run gets slower. Wall time is noisy, but
the number of processes forked over a fixed image under a fixed shell is the same on every run,
so this tool counts them. It builds the main fake image of `collectors/tests/smoke.sh` once
(`SMOKE_IMAGE_ONLY=1`), runs the working tree's collector over it with the smoke test's command
line under `strace -f`, and reads the trace, the manifest and the collector log.

It prints the processes forked (threads are not counted), those that executed a program, the
fork-only rest, the manifest rows and the two ratios forks per row and execs per row; a table of
process classes per phase of the run, read from the log's timestamps (each program by name, each
worker shell by kind, `printf` by format, a fork-only subshell by the programs its children
ran, by having only fork-only children, or else by its parent); the double forks of a function
that runs a program inside `$(...)`, by program; then the verdict. With `--base REF` it also
runs that commit's collector and adds the change per class, head minus base.

The budget, `collectors/tests/fork-budget.txt`, holds `forks_per_row` and `execs_per_row`. A
measured ratio more than 2 % above its budget is `over`, one more than 2 % below it is
`stale`, so an improvement is recorded as deliberately as a regression. `--update-budget`
rewrites the file from the run. The budget is defined for dash: under another shell the tool
prints the report and skips the comparison. It needs strace, so it runs on Linux.

Usage: tools/check_fork_budget.py [--shell SHELL] [--base REF] [--update-budget]

Exit status: 0; 1 when a ratio is over or stale, or the budget file is missing; 2 when a step
cannot run (no strace, no shell, a malformed budget file, `--update-budget` under another
shell, a failed image build, `git show` or collector run, a run leaving no manifest or log).
"""

from __future__ import annotations

import argparse
import bisect
import calendar
import math
import pathlib
import posixpath
import re
import shutil
import sys
import tempfile
import time
from collections import Counter
from dataclasses import dataclass, field

import diff_smoke_rows
from diff_smoke_rows import DiffError

BUDGET_NAME = "collectors/tests/fork-budget.txt"
BUDGET_FILE = diff_smoke_rows.ROOT / BUDGET_NAME
BUDGET_KEYS = ("forks_per_row", "execs_per_row")
BUDGET_HEADER = (
    "# The sh collector's fork budget: the processes it forks (forks_per_row) and the programs\n"
    "# it executes (execs_per_row) per manifest row, under dash over the sh smoke test's main\n"
    "# image, as tools/check_fork_budget.py measures them. The numbers move only in the commit\n"
    "# that changes the cost, whose body gives the measured cause.\n"
)
TOLERANCE = 0.02
TRACED_CALLS = "trace=execve,clone,clone3,vfork,fork"
# Long enough to keep a worker's stage directory and paths whole, and to tell a worker's
# program text from a short `-c` program.
STRING_LIMIT = "256"
FORK_CALLS = frozenset({"clone", "clone3", "vfork", "fork"})
# Under the traced calls above, only strace's own `+++ exited` and `+++ killed` lines end a process.
END_CALLS = ("+++ ",)
UNFINISHED = " <unfinished ...>"
# strace pads the pid to the width of the widest pid seen so far.
TRACE_LINE = re.compile(r"(\d+) +(\d+(?:\.\d+)?) (.*)")
RESULT = re.compile(r"\)\s+=\s+(\S+)")
SHELLS = frozenset({"ash", "bash", "busybox", "dash", "ksh", "mksh", "posh", "sh", "zsh"})
LONG_PROGRAM = 100
MAIN_SHELL = "main shell"
PHASES = ("startup", "homes", "docker", "projects", "hints", "archive", "done")
PHASE_MARKERS = (
    ("done:", "done"),
    ("Archiving", "archive"),
    ("Relocation hints", "hints"),
    ("sweep skipped", "hints"),
    ("Project ", "projects"),
    ("Docker", "docker"),
    ("User ", "homes"),
)
ESCAPES = {"n": "\n", "t": "\t", "r": "\r", "v": "\v", "f": "\f"}


@dataclass(eq=False)
class Process:
    """One process of the trace: its pid, birth time, parent, first program and children."""

    process_id: int
    birth: float
    parent: Process | None = None
    program: str = ""
    arguments: list[str] = field(default_factory=list)
    children: list[Process] = field(default_factory=list)
    ended: bool = False


@dataclass(frozen=True)
class Budget:
    """The two ratios of a run, or of the budget file."""

    forks_per_row: float
    execs_per_row: float


@dataclass
class Measurement:
    """What one traced run counted: totals, manifest rows, classes per phase and double forks."""

    forked: int
    executed: int
    rows: int
    classes: dict[str, Counter[str]]
    double_forks: Counter[str]

    def ratios(self) -> Budget:
        """Return the forks and execs per manifest row."""
        return Budget(self.forked / self.rows, self.executed / self.rows)


@dataclass(frozen=True)
class _PendingCall:
    start: float
    text: str


class TraceReader:
    """Read an `strace -f -ttt` log line by line into processes, in order of birth.

    A pid names a new process at every fork returning it, so a reused pid is a second
    process. A child's lines, its exit included, can precede the parent's fork result; such
    a child is adopted when the result arrives. A thread (`CLONE_THREAD`) is not a process,
    and its id is a process's again once it has exited.
    """

    def __init__(self) -> None:
        """Start with no process."""
        self.processes: list[Process] = []
        self.current: dict[int, Process] = {}
        self.pending: dict[int, _PendingCall] = {}
        self.threads: set[int] = set()
        # Ids whose exit line came before any other line of theirs.
        self.exited: set[int] = set()

    def feed(self, line: str) -> None:
        """Read one trace line, `PID TIMESTAMP CALL`, joining a call split in two."""
        match = TRACE_LINE.fullmatch(line.rstrip("\r"))
        if match is None:
            return
        process_id, start, call = int(match[1]), float(match[2]), match[3]
        if not call.startswith(END_CALLS):
            self._process(process_id, start)
        if call.startswith("<... ") and " resumed>" in call:
            pending = self.pending.pop(process_id, None)
            if pending is None:
                return
            start, call = pending.start, pending.text + call.split(" resumed>", 1)[1]
        elif call.endswith(UNFINISHED):
            self.pending[process_id] = _PendingCall(start, call[: -len(UNFINISHED)])
            return
        self._complete(process_id, start, call)

    def _complete(self, process_id: int, start: float, call: str) -> None:
        if call.startswith(END_CALLS):
            process = self.current.get(process_id)
            if process is not None:
                process.ended = True
            elif process_id not in self.threads:
                self.exited.add(process_id)
            self.threads.discard(process_id)
            return
        name = call.partition("(")[0]
        result = call_result(call)
        if name in FORK_CALLS and result is not None and result > 0:
            if "CLONE_THREAD" in call:
                self._thread(result)
            else:
                self._fork(process_id, result, start)
        elif name == "execve" and result == 0:
            process = self._process(process_id, start)
            if process is not None and not process.program:
                process.program = read_quoted(call, len("execve("))[0]
                process.arguments = read_arguments(call)

    def _process(self, process_id: int, start: float) -> Process | None:
        if process_id in self.threads:
            return None
        process = self.current.get(process_id)
        if process is None or process.ended:
            process = Process(process_id, start)
            self.processes.append(process)
            self.current[process_id] = process
            self.exited.discard(process_id)
        return process

    def _thread(self, thread_id: int) -> None:
        early = self.current.pop(thread_id, None)
        if early is not None and early.parent is None and early is not self.processes[0]:
            self.processes.remove(early)
        exited_before = thread_id in self.exited or (early is not None and early.ended)
        self.exited.discard(thread_id)
        if not exited_before:
            self.threads.add(thread_id)

    def _fork(self, parent_id: int, child_id: int, start: float) -> None:
        parent = self._process(parent_id, start)
        if parent is None:
            return
        self.threads.discard(child_id)
        child = self.current.get(child_id)
        root = self.processes[0]
        if child is None or child.parent is not None or child is root:
            child = Process(child_id, start)
            child.ended = child_id in self.exited
            self.processes.append(child)
            self.current[child_id] = child
        self.exited.discard(child_id)
        child.parent = parent
        child.birth = min(child.birth, start)
        parent.children.append(child)


def read_trace(text: str) -> list[Process]:
    """Return the processes of a trace text in order of birth, the traced command first."""
    reader = TraceReader()
    for line in text.split("\n"):
        reader.feed(line)
    return reader.processes


def call_result(call: str) -> int | None:
    """Return the number a finished call returned, None for `?` or no result.

    strace pads a short call such as `vfork()` out to its result column, so any run of
    spaces may stand between the `)` and the `=`; the last result on the line is the call's.
    """
    results = RESULT.findall(call)
    if not results:
        return None
    try:
        return int(results[-1])
    except ValueError:
        return None


def read_quoted(text: str, start: int) -> tuple[str, int]:
    """Return the strace string literal at `start` unescaped, and the index after it.

    A literal strace cut short is followed by `...`; the value then ends in `...` too.
    """
    characters: list[str] = []
    index = start + 1
    while index < len(text) and text[index] != '"':
        character = text[index]
        if character == "\\" and index + 1 < len(text):
            escaped, index = _unescape(text, index + 1)
            characters.append(escaped)
            continue
        characters.append(character)
        index += 1
    index += 1
    if text.startswith("...", index):
        characters.append("...")
        index += 3
    return "".join(characters), index


def _unescape(text: str, index: int) -> tuple[str, int]:
    character = text[index]
    if character in ESCAPES:
        return ESCAPES[character], index + 1
    if character == "x":
        digits = text[index + 1 : index + 3]
        if len(digits) == 2 and all(digit in "0123456789abcdefABCDEF" for digit in digits):
            return chr(int(digits, 16)), index + 3
    end = index
    while end < len(text) and end < index + 3 and text[end] in "01234567":
        end += 1
    if end > index:
        return chr(int(text[index:end], 8)), end
    return character, index + 1


def read_arguments(call: str) -> list[str]:
    """Return the argument list of an `execve("PROGRAM", [...], ...)` call, unescaped.

    An element strace omitted (`...` in place of the list's tail) is left out.
    """
    _, index = read_quoted(call, len("execve("))
    if not call.startswith(", [", index):
        return []
    index += 3
    arguments: list[str] = []
    while index < len(call) and call[index] != "]":
        if call[index] == '"':
            value, index = read_quoted(call, index)
            arguments.append(value)
        elif call.startswith("...", index):
            index += 3
        elif call[index] in ", ":
            index += 1
        else:
            break
    return arguments


def worker_kind(positional: list[str]) -> str:
    """Return the worker a `-c` program is, from its positional arguments after `$0`.

    The shapes are the first lines of the collector's worker here-documents: a stage
    directory first, a root then a user, a user then a home then an agent, an agent alone
    before the files, or files only.
    """
    paths = ["/" in value for value in positional]
    if positional and "/.stage-" in positional[0]:
        return "STAGE_FILES"
    if len(paths) >= 2 and paths[0] and not paths[1]:
        return "EXCLUDED_PATHS"
    if len(paths) >= 4 and not paths[0] and paths[1] and not paths[2] and paths[3]:
        return "UNREADABLE_DIRS"
    if len(paths) >= 2 and not paths[0] and all(paths[1:]):
        return "INVENTORY_FILES"
    if paths and all(paths):
        return "HINT_SEARCH"
    return "unknown"


def printable(text: str) -> str:
    """Return `text` with control characters and backslashes escaped, for one table cell."""
    return text.encode("unicode_escape").decode("ascii")


def exec_class(process: Process) -> str:
    """Return the class of a process that executed a program."""
    name = posixpath.basename(process.program)
    arguments = process.arguments
    if name in SHELLS and len(arguments) > 2 and arguments[1] == "-c":
        if len(arguments[2]) > LONG_PROGRAM:
            return f"worker:{worker_kind(arguments[4:])}"
        return "sh -c (short)"
    if name == "printf":
        return f"printf {printable(arguments[1][:40]) if len(arguments) > 1 else ''}".rstrip()
    return f"exec:{printable(name)}"


def classify(processes: list[Process]) -> dict[Process, str]:
    """Return the class of every process, the first one being the main shell."""
    classes: dict[Process, str] = {}
    if processes:
        classes[processes[0]] = MAIN_SHELL
    for process in processes:
        _class_of(process, classes)
    return classes


def _class_of(process: Process, classes: dict[Process, str]) -> str:
    known = classes.get(process)
    if known is not None:
        return known
    exec_children = [child for child in process.children if child.program]
    if process.program:
        kind = exec_class(process)
    elif exec_children:
        names = sorted({posixpath.basename(child.program) for child in exec_children})
        kind = "fork->" + "+".join(printable(name) for name in names[:3])
    elif process.children:
        kind = "fork->(fork-only children)"
    elif process.parent is None:
        kind = "fork-only, parent unknown"
    else:
        parent_kind = _class_of(process.parent, classes)
        kind = f"fork-only, parent {parent_kind}"
    classes[process] = kind
    return kind


@dataclass(frozen=True)
class Phases:
    """The phase marker lines of a collector log: their epoch seconds, in order, and their names."""

    stamps: list[float]
    names: list[str]


def read_phases(text: str) -> Phases:
    """Return the phase marker lines of a collector log, in order.

    Log lines are `YYYY-MM-DDTHH:MM:SSZ TEXT`; a line that is no marker is left out.
    """
    phases = Phases([], [])
    for line in text.split("\n"):
        stamp, _, message = line.rstrip("\r").partition(" ")
        try:
            seconds = calendar.timegm(time.strptime(stamp, "%Y-%m-%dT%H:%M:%SZ"))
        except ValueError:
            continue
        phase = next((name for marker, name in PHASE_MARKERS if message.startswith(marker)), "")
        if not phase and "relocation hint:" in message:
            phase = "hints"
        if phase:
            phases.stamps.append(float(seconds))
            phases.names.append(phase)
    return phases


def phase_at(phases: Phases, birth: float) -> str:
    """Return the phase of the last marker at or before `birth`, `startup` before the first.

    The log has second precision, so a process born in the same second as a marker line may
    land on either side of it.
    """
    index = bisect.bisect_right(phases.stamps, birth)
    return phases.names[index - 1] if index else PHASES[0]


def measure(processes: list[Process], phases: Phases, rows: int) -> Measurement:
    """Return the counts of one run: every process but the first, by class and phase."""
    if len(processes) < 1:
        raise DiffError("the trace holds no process")
    if rows < 1:
        raise DiffError("the manifest holds no row")
    classes = classify(processes)
    table: dict[str, Counter[str]] = {}
    double_forks: Counter[str] = Counter()
    forked = processes[1:]
    for process in forked:
        table.setdefault(classes[process], Counter())[phase_at(phases, process.birth)] += 1
        children = process.children
        if not process.program and len(children) == 1 and children[0].program:
            double_forks[printable(posixpath.basename(children[0].program))] += 1
    executed = sum(1 for process in forked if process.program)
    return Measurement(len(forked), executed, rows, table, double_forks)


def read_budget(text: str) -> Budget:
    """Return the budget of a budget file's text; a malformed or missing line raises."""
    values: dict[str, float] = {}
    for number, raw_line in enumerate(text.split("\n"), 1):
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split()
        try:
            value = float(parts[1]) if len(parts) == 2 else 0.0
        except ValueError:
            value = 0.0
        if not math.isfinite(value):
            value = 0.0
        if parts[0] not in BUDGET_KEYS or parts[0] in values or value <= 0:
            raise DiffError(f"{BUDGET_NAME} line {number} is not `<key> <number>`: {line}")
        values[parts[0]] = value
    missing = [key for key in BUDGET_KEYS if key not in values]
    if missing:
        raise DiffError(f"{BUDGET_NAME} lacks {', '.join(missing)}")
    return Budget(values["forks_per_row"], values["execs_per_row"])


def render_budget(budget: Budget) -> str:
    """Return the budget file's text for `budget`, its header first."""
    return (
        f"{BUDGET_HEADER}forks_per_row {budget.forks_per_row:.2f}\n"
        f"execs_per_row {budget.execs_per_row:.2f}\n"
    )


def compare_budget(measured: Budget, budget: Budget) -> list[str]:
    """Return one line per ratio over or stale against the budget; empty when both pass."""
    failures: list[str] = []
    pairs = (
        ("forks per row", measured.forks_per_row, budget.forks_per_row),
        ("execs per row", measured.execs_per_row, budget.execs_per_row),
    )
    for label, value, limit in pairs:
        if value > limit * (1 + TOLERANCE):
            excess = (value / limit - 1) * 100
            failures.append(f"budget: {label} {value:.2f} is over {limit:.2f} by {excess:.1f} %")
        elif value < limit * (1 - TOLERANCE):
            failures.append(
                f"budget: {label} {value:.2f} is stale: more than {TOLERANCE * 100:.0f} % below"
                f" {limit:.2f}; run --update-budget"
            )
    return failures


def totals_line(measurement: Measurement) -> str:
    """Return the totals line of one run."""
    ratios = measurement.ratios()
    return (
        f"processes forked {measurement.forked}; exec'd {measurement.executed};"
        f" fork-only {measurement.forked - measurement.executed};"
        f" manifest rows {measurement.rows}; forks per row {ratios.forks_per_row:.2f};"
        f" execs per row {ratios.execs_per_row:.2f}"
    )


def class_table(head: Measurement, base: Measurement | None) -> list[str]:
    """Return the class table, by total descending, with a `change` column when there is a base."""
    names = set(head.classes) | (set(base.classes) if base else set())
    totals = {name: sum(head.classes.get(name, Counter()).values()) for name in names}
    ordered = sorted(names, key=lambda name: (-totals[name], name))
    width = max([len("class"), *(len(name) for name in ordered)])
    header = f"{'class':<{width}} {'total':>8}" + "".join(f" {phase:>8}" for phase in PHASES)
    lines = [header + (f" {'change':>8}" if base else "")]
    for name in ordered:
        counts = head.classes.get(name, Counter())
        line = f"{name:<{width}} {totals[name]:>8}"
        line += "".join(f" {counts.get(phase, 0):>8}" for phase in PHASES)
        if base:
            change = totals[name] - sum(base.classes.get(name, Counter()).values())
            line += f" {f'{change:+d}' if change else '0':>8}"
        lines.append(line)
    return lines


def render_report(head: Measurement, base: Measurement | None, base_name: str) -> str:
    """Return the report before the verdict: totals, class table and double forks."""
    lines = [totals_line(head)]
    if base:
        lines.append(f"base {base_name}: {totals_line(base)}")
    lines.append("")
    lines.extend(class_table(head, base))
    lines.append("")
    lines.append("fork-only processes whose one child exec'd (a function inside $(...)):")
    ordered = sorted(head.double_forks.items(), key=lambda item: (-item[1], item[0]))
    lines.extend(f"  {name} {count}" for name, count in ordered)
    if not ordered:
        lines.append("  (none)")
    return "\n".join(lines) + "\n"


def load_budget(shell: str, update: bool) -> Budget | None:
    """Return the budget file's budget, or None when the run will not compare or the file is absent.

    It is read before the run, so a malformed file fails before the minute the run takes.
    """
    if update or posixpath.basename(shell) != "dash" or not BUDGET_FILE.is_file():
        return None
    return read_budget(BUDGET_FILE.read_text(encoding="utf-8"))


def verdict(
    measurement: Measurement, shell: str, update: bool, budget: Budget | None
) -> tuple[list[str], int]:
    """Return the verdict lines and exit status; with `update`, write the budget file first."""
    if posixpath.basename(shell) != "dash":
        return [f"budget: defined for dash, not compared under {shell}"], 0
    ratios = measurement.ratios()
    if update:
        BUDGET_FILE.write_text(render_budget(ratios), encoding="utf-8")
        return [
            f"budget: wrote {BUDGET_NAME} (forks per row {ratios.forks_per_row:.2f},"
            f" execs per row {ratios.execs_per_row:.2f})"
        ], 0
    if budget is None:
        return [f"budget: {BUDGET_NAME} is missing; run --update-budget"], 1
    failures = compare_budget(ratios, budget)
    if failures:
        return failures, 1
    return [
        f"budget: ok (forks per row {ratios.forks_per_row:.2f} of {budget.forks_per_row:.2f},"
        f" execs per row {ratios.execs_per_row:.2f} of {budget.execs_per_row:.2f})"
    ], 0


def strace_command(trace: pathlib.Path) -> list[str]:
    """Return the strace prefix that writes the process calls of a run to `trace`."""
    # Not -qq: the exit messages it would drop tell the reader when a pid is free for reuse.
    return ["strace", "-f", "-ttt", "-s", STRING_LIMIT, "-e", TRACED_CALLS, "-o", str(trace)]


def only_file(directory: pathlib.Path, pattern: str) -> pathlib.Path:
    """Return the one file of `directory` matching `pattern`; none or several raises."""
    matches = sorted(directory.glob(pattern))
    if len(matches) != 1:
        raise DiffError(f"{directory}: expected one {pattern}, found {len(matches)}")
    return matches[0]


def traced_run(shell: str, script: str, image_root: str, directory: pathlib.Path) -> Measurement:
    """Run one collector over the image under strace and return what it counted."""
    output = directory / "out"
    output.mkdir(parents=True)
    trace = directory / "trace.log"
    environment, arguments = diff_smoke_rows.collector_command(
        shell, script, image_root, str(output)
    )
    diff_smoke_rows.run_process(strace_command(trace) + arguments, environment)
    manifest = only_file(output, "*.manifest.jsonl").read_text(encoding="utf-8")
    log = only_file(output, "*.log").read_text(encoding="utf-8", errors="replace")
    processes = read_trace(trace.read_text(encoding="utf-8", errors="replace"))
    return measure(processes, read_phases(log), len(diff_smoke_rows.read_manifest(manifest)))


def measure_runs(shell: str, base: str | None) -> tuple[Measurement, Measurement | None]:
    """Build the image, run the working tree's collector and, with `base`, that commit's."""
    work = diff_smoke_rows.build_image(shell)
    try:
        image_root = str(work / "root")
        with tempfile.TemporaryDirectory(prefix="check-fork-budget.") as scratch:
            directory = pathlib.Path(scratch)
            script = diff_smoke_rows.write_collector(None, directory, "head")
            head = traced_run(shell, script, image_root, directory / "head")
            base_measurement = None
            if base is not None:
                script = diff_smoke_rows.write_collector(base, directory, "base")
                base_measurement = traced_run(shell, script, image_root, directory / "base")
    finally:
        shutil.rmtree(work)
    return head, base_measurement


def main(argv: list[str] | None = None) -> int:
    """Measure the run, print the report and the verdict, and return the exit status."""
    parser = argparse.ArgumentParser(
        description="Count the sh collector's processes per manifest row against the budget."
    )
    parser.add_argument("--shell", default="dash", help="the shell running the collector")
    parser.add_argument("--base", help="a commit whose collector the change per class is from")
    parser.add_argument("--update-budget", action="store_true", help="rewrite the budget file")
    arguments = parser.parse_args(argv)
    if shutil.which("strace") is None:
        print("check_fork_budget: needs strace: run it on a Linux host or in CI", file=sys.stderr)
        return 2
    if shutil.which(arguments.shell) is None:
        print(f"check_fork_budget: shell {arguments.shell} not found", file=sys.stderr)
        return 2
    if arguments.update_budget and posixpath.basename(arguments.shell) != "dash":
        print("check_fork_budget: the budget is defined for dash", file=sys.stderr)
        return 2
    try:
        budget = load_budget(arguments.shell, arguments.update_budget)
        head, base = measure_runs(arguments.shell, arguments.base)
        lines, status = verdict(head, arguments.shell, arguments.update_budget, budget)
    except DiffError as error:
        print(f"check_fork_budget: {error}", file=sys.stderr)
        return 2
    sys.stdout.write(render_report(head, base, arguments.base or ""))
    sys.stdout.write("\n" + "\n".join(lines) + "\n")
    return status


if __name__ == "__main__":
    sys.exit(main())
