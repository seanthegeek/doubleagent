"""Tests of `tools/check_fork_budget.py` on synthetic text; it runs no collector and no strace."""

from __future__ import annotations

import calendar
import json
import pathlib
import shutil
import time
from collections import Counter

import check_fork_budget
import diff_smoke_rows
from check_fork_budget import Budget, Measurement, Process
from diff_smoke_rows import DiffError

START = float(calendar.timegm(time.strptime("2030-01-01T00:00:00Z", "%Y-%m-%dT%H:%M:%SZ")))
FORK = "clone(child_stack=NULL, flags=CLONE_CHILD_CLEARTID|CLONE_CHILD_SETTID|SIGCHLD"
THREAD = "clone3({flags=CLONE_VM|CLONE_THREAD|CLONE_SETTLS, exit_signal=0}"
PROGRAM_TEXT = "# shared helpers\n" + "x=1\n" * 60
STAGE = "/w/out/.stage-host_agent-artifacts"
ROOT = "/w/root"
HOME = "/w/root/home/alice"
MANIFEST = "/w/out/host.manifest.jsonl"


def line(process_id: int, offset: float, call: str) -> str:
    """Return one trace line, `offset` seconds after the start."""
    return f"{process_id} {START + offset:.6f} {call}"


def fork(process_id: int, offset: float, child: int) -> str:
    """Return a finished clone that returned `child`."""
    return line(process_id, offset, f"{FORK}, child_tidptr=0xe) = {child}")


def execve(process_id: int, offset: float, program: str, arguments: list[str]) -> str:
    """Return a finished execve that succeeded."""
    listed = ", ".join(json.dumps(argument) for argument in arguments)
    return line(process_id, offset, f'execve("{program}", [{listed}], 0xc /* 3 vars */) = 0')


def end(process_id: int, offset: float) -> str:
    """Return the exit line of a process."""
    return line(process_id, offset, "+++ exited with 0 +++")


def trace(*lines: str) -> list[Process]:
    """Return the processes of a trace made of `lines`."""
    return check_fork_budget.read_trace("\n".join(lines) + "\n")


def classes_of(*lines: str) -> list[str]:
    """Return the class of every process of a trace, in order of birth."""
    processes = trace(*lines)
    classes = check_fork_budget.classify(processes)
    return [classes[process] for process in processes]


def shell_worker(child: int, positional: list[str]) -> list[str]:
    """Return the lines of the main shell forking a worker shell with `positional`."""
    return [
        fork(1, 0.1, child),
        execve(child, 0.2, "/usr/bin/dash", ["dash", "-c", PROGRAM_TEXT, "sh", *positional]),
    ]


def test_a_clone_returning_a_child_makes_a_process_of_the_child_under_the_parent():
    processes = trace(execve(1, 0, "/usr/bin/dash", ["dash", "c.sh"]), fork(1, 0.1, 2))
    assert [process.process_id for process in processes] == [1, 2]
    assert processes[1].parent is processes[0] and processes[0].children == [processes[1]]
    assert processes[0].program == "/usr/bin/dash" and processes[1].program == ""


def test_clone3_and_vfork_fork_and_a_thread_is_no_process():
    processes = trace(
        line(1, 0, "clone3({flags=CLONE_VM|CLONE_VFORK, exit_signal=SIGCHLD}, 88) = 2"),
        line(1, 0.1, "vfork() = 3"),
        line(1, 0.2, f"{THREAD}, 88) = 4"),
        line(4, 0.3, 'execve("/usr/bin/true", ["true"], 0xc) = 0'),
    )
    assert [process.process_id for process in processes] == [1, 2, 3]


def test_a_thread_whose_lines_precede_its_clone_result_is_no_process():
    processes = trace(
        line(1, 0, f"{THREAD} <unfinished ...>"),
        line(4, 0.1, "--- SIGCHLD {si_signo=SIGCHLD} ---"),
        line(1, 0.2, "<... clone3 resumed>, 88) = 4"),
        line(4, 0.3, "exit(0) = ?"),
    )
    assert [process.process_id for process in processes] == [1]


def test_an_unfinished_clone_and_its_resumption_are_one_fork():
    processes = trace(
        line(1, 0, f"{FORK} <unfinished ...>"),
        execve(2, 0.1, "/usr/bin/cp", ["cp", "a", "b"]),
        line(1, 0.2, "<... clone resumed>, child_tidptr=0xe) = 2"),
    )
    assert [process.process_id for process in processes] == [1, 2]
    child = processes[1]
    assert child.parent is processes[0] and child.program == "/usr/bin/cp"
    assert child.birth == START, "a child is born when the fork started"


def test_an_unfinished_execve_and_its_resumption_are_one_exec():
    processes = trace(
        fork(1, 0, 2),
        line(
            2, 0.1, 'execve("/usr/bin/sed", ["sed", "-n", "p"], 0xc /* 3 vars */ <unfinished ...>'
        ),
        fork(1, 0.2, 3),
        line(2, 0.3, "<... execve resumed>) = 0"),
    )
    assert processes[1].program == "/usr/bin/sed"
    assert processes[1].arguments == ["sed", "-n", "p"]


def test_a_failed_execve_is_no_exec():
    processes = trace(
        fork(1, 0, 2),
        line(2, 0.1, 'execve("/usr/local/bin/cp", ["cp"], 0xc) = -1 ENOENT (No such file)'),
        line(2, 0.2, 'execve("/bin/cp", ["cp"], 0xc <unfinished ...>'),
        line(2, 0.3, "<... execve resumed>) = -1 ENOENT (No such file or directory)"),
    )
    assert processes[1].program == ""


def test_a_pid_reused_after_its_process_ended_is_a_second_process():
    processes = trace(
        fork(1, 0, 2),
        execve(2, 0.1, "/usr/bin/cp", ["cp"]),
        end(2, 0.2),
        execve(2, 0.3, "/usr/bin/mv", ["mv"]),
        fork(1, 0.25, 2),
    )
    assert [process.process_id for process in processes] == [1, 2, 2]
    assert [process.program for process in processes[1:]] == ["/usr/bin/cp", "/usr/bin/mv"]
    assert processes[2].parent is processes[0]
    assert processes[0].children == processes[1:]


def test_a_pid_reused_after_a_child_that_exited_before_the_fork_result_is_a_second_process():
    processes = trace(
        execve(1, 0, "/usr/bin/dash", ["dash", "c.sh"]),
        line(1, 0.1, f"{FORK}, child_tidptr=0xe <unfinished ...>"),
        end(2, 0.2),
        line(1, 0.3, "<... clone resumed>) = 2"),
        execve(2, 0.4, "/usr/bin/mv", ["mv"]),
        fork(1, 0.5, 2),
    )
    assert [process.process_id for process in processes] == [1, 2, 2]
    assert [process.program for process in processes[1:]] == ["", "/usr/bin/mv"]


def test_a_thread_id_reused_by_a_process_after_the_thread_exited_is_a_process():
    processes = trace(
        execve(1, 0, "/usr/bin/dash", ["dash", "c.sh"]),
        line(1, 0.1, f"{THREAD}, 88) = 4"),
        end(4, 0.2),
        execve(4, 0.3, "/usr/bin/true", ["true"]),
        fork(1, 0.4, 4),
    )
    assert [process.process_id for process in processes] == [1, 4]
    assert processes[1].program == "/usr/bin/true" and processes[1].parent is processes[0]


def test_a_thread_that_exited_before_its_clone_result_frees_its_id():
    before_any_line = [line(1, 0.1, f"{THREAD} <unfinished ...>"), end(4, 0.2)]
    after_a_signal = [
        line(1, 0.1, f"{THREAD} <unfinished ...>"),
        line(4, 0.15, "--- SIGCHLD {si_signo=SIGCHLD} ---"),
        end(4, 0.2),
    ]
    for case in (before_any_line, after_a_signal):
        processes = trace(
            execve(1, 0, "/usr/bin/dash", ["dash", "c.sh"]),
            *case,
            line(1, 0.3, "<... clone3 resumed>, 88) = 4"),
            execve(4, 0.4, "/usr/bin/true", ["true"]),
            fork(1, 0.5, 4),
        )
        assert [process.process_id for process in processes] == [1, 4], case
        assert processes[1].program == "/usr/bin/true", case


def test_a_short_call_padded_to_the_result_column_is_read():
    processes = trace(
        execve(1, 0, "/usr/bin/dash", ["dash", "c.sh"]),
        line(1, 0.1, "vfork()" + " " * 33 + "= 2"),
        line(1, 0.2, "fork()" + " " * 34 + "= 3"),
        line(1, 0.3, "vfork()" + " " * 33 + "= -1 EAGAIN (Resource temporarily unavailable)"),
    )
    assert [process.process_id for process in processes] == [1, 2, 3]
    assert check_fork_budget.call_result("exit(0) = ?") is None
    assert check_fork_budget.call_result("<... clone resumed>) = 2") == 2


def test_a_child_that_exited_before_the_fork_result_is_one_process():
    processes = trace(
        execve(1, 0, "/usr/bin/dash", ["dash", "c.sh"]),
        line(1, 0.1, f"{FORK}, child_tidptr=0xe <unfinished ...>"),
        execve(2, 0.2, "/usr/bin/true", ["true"]),
        end(2, 0.3),
        line(1, 0.4, "<... clone resumed>) = 2"),
    )
    assert [process.process_id for process in processes] == [1, 2]
    assert processes[1].program == "/usr/bin/true"
    assert processes[1].parent is processes[0] and processes[0].children == [processes[1]]


def test_a_padded_pid_and_a_signal_line_are_read():
    processes = trace(
        "7  1.000000 clone(child_stack=NULL, flags=SIGCHLD) = 12",
        "12 1.100000 --- SIGPIPE {si_signo=SIGPIPE} ---",
        "12 1.200000 +++ killed by SIGPIPE +++",
        "not a trace line",
    )
    assert [process.process_id for process in processes] == [7, 12]
    assert processes[1].ended


def test_read_arguments_unescapes_and_keeps_the_truncation_marker():
    call = (
        r'execve("/usr/bin/awk", ["awk", "a\"b\\c\nd\t\101\x42", "long text"..., ...],'
        " 0xc /* 3 vars */) = 0"
    )
    assert check_fork_budget.read_arguments(call) == ["awk", 'a"b\\c\nd\tAB', "long text..."]


def test_read_arguments_of_a_list_strace_could_not_read_is_empty():
    assert check_fork_budget.read_arguments('execve("/bin/x", 0x1234, 0xc) = 0') == []


def test_each_worker_kind_by_its_argument_shape():
    cases = {
        "worker:STAGE_FILES": [STAGE, ROOT, "alice", HOME, "claude-code", MANIFEST, f"{HOME}/a"],
        "worker:EXCLUDED_PATHS": [ROOT, "alice", HOME, "claude-code", MANIFEST, f"{HOME}/a"],
        "worker:UNREADABLE_DIRS": ["alice", HOME, "claude-code", MANIFEST, f"{HOME}/d"],
        "worker:INVENTORY_FILES": ["claude-code", f"{HOME}/a", f"{HOME}/b"],
        "worker:HINT_SEARCH": [f"{HOME}/.bashrc", f"{HOME}/.profile"],
        "worker:unknown": ["alice"],
    }
    for expected, positional in cases.items():
        classes = classes_of(*shell_worker(2, positional))
        assert classes[1] == expected, f"{expected}: {classes}"


def test_short_program_printf_and_plain_programs():
    classes = classes_of(
        fork(1, 0, 2),
        execve(2, 0.1, "/bin/sh", ["sh", "-c", "exit 0"]),
        fork(1, 0.2, 3),
        execve(3, 0.3, "/usr/bin/printf", ["printf", "%s\n" + "y" * 50, "value"]),
        fork(1, 0.4, 4),
        execve(4, 0.5, "/usr/bin/cp", ["cp", "-p", "a", "b"]),
    )
    assert classes == ["main shell", "sh -c (short)", "printf %s\\n" + "y" * 37, "exec:cp"]


def test_fork_only_classes_by_children_and_parent():
    classes = classes_of(
        fork(1, 0, 2),
        fork(2, 0.1, 3),
        execve(3, 0.2, "/usr/bin/date", ["date", "-u"]),
        fork(1, 0.3, 4),
        fork(4, 0.4, 5),
        fork(1, 0.5, 6),
        *shell_worker(7, [f"{HOME}/.bashrc"]),
        fork(7, 0.6, 8),
    )
    assert classes == [
        "main shell",
        "fork->date",
        "exec:date",
        "fork->(fork-only children)",
        "fork-only, parent fork->(fork-only children)",
        "fork-only, parent main shell",
        "worker:HINT_SEARCH",
        "fork-only, parent worker:HINT_SEARCH",
    ]


def test_a_fork_only_process_names_up_to_three_programs_of_its_children():
    lines = [fork(1, 0, 2)]
    for child, program in enumerate(["tr", "awk", "grep", "cut", "awk"], 3):
        lines += [fork(2, 0.1, child), execve(child, 0.2, f"/usr/bin/{program}", [program])]
    assert classes_of(*lines)[1] == "fork->awk+cut+grep"


LOG = """2030-01-01T00:00:00Z collect-agent-artifacts starting
2030-01-01T00:00:10Z User alice (/w/root/home/alice)
2030-01-01T00:00:11Z [claude-code] /w/root/home/alice/.claude
2030-01-01T00:00:20Z Docker volumes in /w/root/var/lib/docker/volumes
2030-01-01T00:00:30Z Project [alice] /w/root/srv/proj
2030-01-01T00:00:40Z Relocation hints for alice (/w/root/home/alice)
2030-01-01T00:00:41Z Project [alice] /w/root/srv/other
2030-01-01T00:00:42Z sweep skipped: home /w/root/opt lies outside the user home roots
2030-01-01T00:00:43Z relocation hint: /w/root/etc/profile:3:names CODEX_HOME (codex)
2030-01-01T00:00:45Z Archiving
2030-01-01T00:00:50Z done: /w/out/host.tar.gz (10 bytes, sha256 0)
not a log line
"""


def test_phases_carry_forward_from_each_marker():
    phases = check_fork_budget.read_phases(LOG)
    assert phases.names == [
        "homes",
        "docker",
        "projects",
        "hints",
        "projects",
        "hints",
        "hints",
        "archive",
        "done",
    ]
    births: dict[float, str] = {
        5: "startup",
        10: "homes",
        15: "homes",
        25: "docker",
        35: "projects",
    }
    births.update({40.5: "hints", 41.5: "projects", 43: "hints", 46: "archive", 55: "done"})
    # A birth in the very second of a marker line belongs to that marker's phase.
    births[phases.stamps[0] - START] = phases.names[0]
    for offset, expected in births.items():
        got = check_fork_budget.phase_at(phases, START + offset)
        assert got == expected, f"{offset}: {got}"


def test_measure_counts_classes_per_phase_and_double_forks():
    processes = trace(
        execve(1, 0, "/usr/bin/dash", ["dash", "c.sh"]),
        fork(1, 1, 2),
        fork(2, 1.1, 3),
        execve(3, 1.2, "/usr/bin/date", ["date"]),
        fork(1, 12, 4),
        execve(4, 12.1, "/usr/bin/cp", ["cp"]),
        fork(1, 51, 5),
    )
    measurement = check_fork_budget.measure(processes, check_fork_budget.read_phases(LOG), 2)
    assert (measurement.forked, measurement.executed, measurement.rows) == (4, 2, 2)
    assert measurement.classes == {
        "fork->date": Counter({"startup": 1}),
        "exec:date": Counter({"startup": 1}),
        "exec:cp": Counter({"homes": 1}),
        "fork-only, parent main shell": Counter({"done": 1}),
    }
    assert measurement.double_forks == Counter({"date": 1})
    assert measurement.ratios() == Budget(2.0, 1.0)


def test_measure_without_processes_raises():
    try:
        check_fork_budget.measure([], check_fork_budget.Phases([], []), 2)
    except DiffError as error:
        assert "no process" in str(error)
    else:
        raise AssertionError("an empty trace was measured")


def test_measure_without_rows_raises():
    try:
        check_fork_budget.measure(trace(fork(1, 0, 2)), check_fork_budget.Phases([], []), 0)
    except DiffError as error:
        assert "no row" in str(error)
    else:
        raise AssertionError("a run without manifest rows was measured")


def test_read_budget_skips_comments_and_reads_both_keys():
    text = "# a comment\n\n  # another\nforks_per_row 7.22\nexecs_per_row 4.21\n"
    assert check_fork_budget.read_budget(text) == Budget(7.22, 4.21)


def test_read_budget_raises_on_a_malformed_or_missing_line():
    cases = [
        "forks_per_row 7.22\nexecs_per_row\n",
        "forks_per_row 7.22\nexecs_per_row four\n",
        "forks_per_row 7.22\nexecs_per_row 4.21\nwall_time 3\n",
        "forks_per_row 7.22\nforks_per_row 7.30\nexecs_per_row 4.21\n",
        "forks_per_row 7.22\nexecs_per_row 0\n",
        "forks_per_row 7.22\nexecs_per_row nan\n",
        "forks_per_row inf\nexecs_per_row 4.21\n",
        "forks_per_row 7.22\n",
    ]
    for text in cases:
        try:
            check_fork_budget.read_budget(text)
        except DiffError:
            continue
        raise AssertionError(f"accepted: {text!r}")


def test_render_budget_round_trips_with_two_decimals_and_its_header():
    text = check_fork_budget.render_budget(Budget(7.2249, 4.216))
    assert text.startswith(check_fork_budget.BUDGET_HEADER)
    assert text.endswith("forks_per_row 7.22\nexecs_per_row 4.22\n")
    assert check_fork_budget.read_budget(text) == Budget(7.22, 4.22)


def test_compare_budget_at_both_edges_of_the_tolerance():
    budget = Budget(10.0, 4.0)
    passing = [Budget(10.19, 4.0), Budget(10.0, 4.0 * 1.019), Budget(10.0 / 1.019, 4.0)]
    for measured in passing:
        assert check_fork_budget.compare_budget(measured, budget) == [], measured
    over = check_fork_budget.compare_budget(Budget(10.21, 4.0), budget)
    assert over == ["budget: forks per row 10.21 is over 10.00 by 2.1 %"]
    stale = check_fork_budget.compare_budget(Budget(10.0, 4.0 / 1.021), budget)
    assert stale == [
        "budget: execs per row 3.92 is stale: more than 2 % below 4.00; run --update-budget"
    ]
    both = check_fork_budget.compare_budget(Budget(10.0 * 0.979, 4.0 * 1.021), budget)
    assert [failure.split()[6] for failure in both] == ["stale:", "over"]


def measurement(classes: dict[str, Counter[str]], rows: int = 10) -> Measurement:
    """Return a measurement of `classes`, counting a class starting `exec:` as exec'd."""
    forked = sum(sum(counts.values()) for counts in classes.values())
    executed = sum(sum(counts.values()) for name, counts in classes.items() if "exec:" in name)
    return Measurement(forked, executed, rows, classes, Counter({"date": 2}))


def test_render_report_prints_totals_table_by_total_and_double_forks():
    head = measurement({"exec:cp": Counter({"homes": 3}), "exec:date": Counter({"done": 5})})
    report = check_fork_budget.render_report(head, None, "")
    lines = report.split("\n")
    assert lines[0] == (
        "processes forked 8; exec'd 8; fork-only 0; manifest rows 10;"
        " forks per row 0.80; execs per row 0.80"
    )
    assert lines[2].split() == ["class", "total", *check_fork_budget.PHASES]
    assert lines[3].split() == ["exec:date", "5", "0", "0", "0", "0", "0", "0", "5"]
    assert lines[4].split() == ["exec:cp", "3", "0", "3", "0", "0", "0", "0", "0"]
    assert lines[6:8] == [
        "fork-only processes whose one child exec'd (a function inside $(...)):",
        "  date 2",
    ]


def test_render_report_with_a_base_adds_its_totals_and_the_change():
    head = measurement({"exec:cp": Counter({"homes": 3}), "exec:date": Counter({"done": 5})})
    base = measurement({"exec:cp": Counter({"homes": 3}), "exec:tr": Counter({"hints": 4})})
    lines = check_fork_budget.render_report(head, base, "origin/main").split("\n")
    assert lines[1].startswith("base origin/main: processes forked 7; exec'd 7;")
    assert lines[3].split()[-1] == "change"
    rows = [row.split() for row in lines[4:7]]
    assert [(row[0], row[1], row[-1]) for row in rows] == [
        ("exec:date", "5", "+5"),
        ("exec:cp", "3", "0"),
        ("exec:tr", "0", "-4"),
    ]


def test_verdict_lines(tmp_path, monkeypatch):
    budget_file = tmp_path / "fork-budget.txt"
    monkeypatch.setattr(check_fork_budget, "BUDGET_FILE", budget_file)
    run = measurement({"exec:cp": Counter({"homes": 20})})
    lines, status = check_fork_budget.verdict(run, "posh", False, None)
    assert (lines, status) == (["budget: defined for dash, not compared under posh"], 0)
    budget = check_fork_budget.load_budget("dash", False)
    lines, status = check_fork_budget.verdict(run, "dash", False, budget)
    assert status == 1 and lines == [
        "budget: collectors/tests/fork-budget.txt is missing; run --update-budget"
    ]
    lines, status = check_fork_budget.verdict(run, "/usr/bin/dash", True, None)
    assert status == 0 and budget_file.read_text().endswith(
        "forks_per_row 2.00\nexecs_per_row 2.00\n"
    )
    budget = check_fork_budget.load_budget("dash", False)
    lines, status = check_fork_budget.verdict(run, "dash", False, budget)
    assert (lines, status) == (
        ["budget: ok (forks per row 2.00 of 2.00, execs per row 2.00 of 2.00)"],
        0,
    )
    budget_file.write_text("forks_per_row 1.50\nexecs_per_row 2.00\n")
    budget = check_fork_budget.load_budget("dash", False)
    lines, status = check_fork_budget.verdict(run, "dash", False, budget)
    assert status == 1 and lines == ["budget: forks per row 2.00 is over 1.50 by 33.3 %"]


def fake_runs(tmp_path, monkeypatch, children: dict[str, int]) -> pathlib.Path:
    """Patch the image build, the collector paths and strace, and return the image work directory.

    Each fake run's main shell forks `children[name]` copies, the named collector `head` or
    `base`, and writes a manifest of two rows and a log.
    """
    work = tmp_path / "cac-smoke.test"
    monkeypatch.setattr(shutil, "which", lambda program: f"/usr/bin/{program}")

    def build_image(shell: str) -> pathlib.Path:
        (work / "root").mkdir(parents=True, exist_ok=True)
        return work

    monkeypatch.setattr(diff_smoke_rows, "build_image", build_image)
    monkeypatch.setattr(diff_smoke_rows, "write_collector", lambda revision, directory, name: name)

    def run_process(arguments: list[str], extra: dict[str, str]) -> str:
        trace_path = pathlib.Path(arguments[arguments.index("-o") + 1])
        output = pathlib.Path(arguments[len(arguments) - arguments[::-1].index("-o")])
        script = arguments[arguments.index(extra["COLLECTOR_SH"], 1) + 1]
        lines = [execve(1, 0, "/usr/bin/dash", ["dash", script])]
        for child in range(2, children[script] + 2):
            lines += [fork(1, 11, child), execve(child, 11.5, "/usr/bin/cp", ["cp"])]
        trace_path.write_text("\n".join(lines) + "\n")
        (output / "host.log").write_text(LOG)
        (output / "host.manifest.jsonl").write_text('{"path": "/a"}\n{"path": "/b"}\n')
        return ""

    monkeypatch.setattr(diff_smoke_rows, "run_process", run_process)
    return work


def test_main_measures_compares_with_a_base_and_removes_the_image(tmp_path, monkeypatch, capsys):
    work = fake_runs(tmp_path, monkeypatch, {"head": 4, "base": 2})
    budget_file = tmp_path / "fork-budget.txt"
    budget_file.write_text("forks_per_row 2.00\nexecs_per_row 2.00\n")
    monkeypatch.setattr(check_fork_budget, "BUDGET_FILE", budget_file)
    assert check_fork_budget.main(["--base", "origin/main"]) == 0
    output = capsys.readouterr().out.split("\n")
    assert output[0].startswith("processes forked 4; exec'd 4; fork-only 0; manifest rows 2;")
    assert output[1].startswith("base origin/main: processes forked 2;")
    assert output[4].split() == ["exec:cp", "4", "0", "4", "0", "0", "0", "0", "0", "+2"]
    assert output[-2] == "budget: ok (forks per row 2.00 of 2.00, execs per row 2.00 of 2.00)"
    assert not work.exists()


def test_main_exits_1_over_the_budget_and_when_the_budget_file_is_missing(tmp_path, monkeypatch):
    fake_runs(tmp_path, monkeypatch, {"head": 6})
    budget_file = tmp_path / "fork-budget.txt"
    monkeypatch.setattr(check_fork_budget, "BUDGET_FILE", budget_file)
    assert check_fork_budget.main([]) == 1
    budget_file.write_text("forks_per_row 2.00\nexecs_per_row 2.00\n")
    assert check_fork_budget.main([]) == 1
    assert check_fork_budget.main(["--update-budget"]) == 0
    assert check_fork_budget.main([]) == 0


def test_main_exits_1_on_a_stale_budget_and_2_on_a_malformed_one(tmp_path, monkeypatch, capsys):
    fake_runs(tmp_path, monkeypatch, {"head": 4})
    budget_file = tmp_path / "fork-budget.txt"
    monkeypatch.setattr(check_fork_budget, "BUDGET_FILE", budget_file)
    budget_file.write_text("forks_per_row 2.10\nexecs_per_row 2.00\n")
    assert check_fork_budget.main([]) == 1
    assert "forks per row 2.00 is stale: more than 2 % below 2.10" in capsys.readouterr().out
    budget_file.write_text("forks_per_row two\nexecs_per_row 2.00\n")
    assert check_fork_budget.main([]) == 2
    assert "line 1 is not `<key> <number>`" in capsys.readouterr().err


def test_main_exits_2_when_updating_the_budget_under_another_shell(monkeypatch, capsys):
    monkeypatch.setattr(shutil, "which", lambda program: f"/usr/bin/{program}")
    assert check_fork_budget.main(["--shell", "posh", "--update-budget"]) == 2
    assert capsys.readouterr().err == "check_fork_budget: the budget is defined for dash\n"


def test_main_exits_2_without_strace_or_the_shell(monkeypatch, capsys):
    monkeypatch.setattr(shutil, "which", lambda program: None)
    assert check_fork_budget.main([]) == 2
    assert capsys.readouterr().err == (
        "check_fork_budget: needs strace: run it on a Linux host or in CI\n"
    )
    monkeypatch.setattr(
        shutil, "which", lambda program: "/usr/bin/strace" if program == "strace" else None
    )
    assert check_fork_budget.main(["--shell", "posh"]) == 2
    assert "shell posh not found" in capsys.readouterr().err


def test_main_exits_2_when_a_run_leaves_no_manifest(tmp_path, monkeypatch, capsys):
    fake_runs(tmp_path, monkeypatch, {"head": 1})
    fake_run = diff_smoke_rows.run_process

    def run_without_manifest(arguments: list[str], extra: dict[str, str]) -> str:
        fake_run(arguments, extra)
        output = pathlib.Path(arguments[len(arguments) - arguments[::-1].index("-o")])
        (output / "host.manifest.jsonl").unlink()
        return ""

    monkeypatch.setattr(diff_smoke_rows, "run_process", run_without_manifest)
    assert check_fork_budget.main([]) == 2
    assert "expected one *.manifest.jsonl, found 0" in capsys.readouterr().err


def test_main_exits_2_and_removes_the_image_when_a_run_fails(tmp_path, monkeypatch):
    work = fake_runs(tmp_path, monkeypatch, {"head": 1})

    def failing_run(arguments: list[str], extra: dict[str, str]) -> str:
        raise DiffError("strace exited 1")

    monkeypatch.setattr(diff_smoke_rows, "run_process", failing_run)
    assert check_fork_budget.main([]) == 2
    assert not work.exists()


def test_strace_prefix_traces_the_process_calls_into_the_named_file():
    prefix = check_fork_budget.strace_command(pathlib.Path("/w/trace.log"))
    assert prefix[0] == "strace" and prefix[-2:] == ["-o", "/w/trace.log"]
    assert "-f" in prefix and "trace=execve,clone,clone3,vfork,fork" in prefix
