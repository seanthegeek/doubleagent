"""Tests of lib/steps.sh, the dialog step kinds, run under sh and dash against stubbed calls.

The stubs replace drive, capture_screen, wait_for, state_get, state_set and round_log. They keep
a scripted dialog in a directory: its option labels, the cursor, whether Down wraps or stops at
the last option, and every drive call, so a test reads what the step pressed and where the
cursor ended.
"""

from __future__ import annotations

import pathlib
import subprocess

ROUND = pathlib.Path(__file__).resolve().parent.parent
SHELLS = [shell for shell in ("/bin/sh", "/bin/dash") if pathlib.Path(shell).is_file()]
STUBS = r"""
drive() {
  printf '%s\n' "$*" >>"$STUB/drive.log"
  [ ! -f "$STUB/fail" ] || return 1
  [ "$1" != keys ] || stub_keys "$2"
}
stub_keys() {
  _sk_rest=$1,
  while [ -n "$_sk_rest" ]; do
    _sk_key=${_sk_rest%%,*}
    _sk_rest=${_sk_rest#*,}
    _sk_cursor=$(cat "$STUB/cursor")
    _sk_last=$(($(wc -l <"$STUB/labels") - 1))
    case $_sk_key in
      Down)
        if [ "$_sk_cursor" -lt "$_sk_last" ]; then _sk_cursor=$((_sk_cursor + 1))
        elif [ "$(cat "$STUB/mode")" = wrap ]; then _sk_cursor=0; fi ;;
      Up) [ "$_sk_cursor" -eq 0 ] || _sk_cursor=$((_sk_cursor - 1)) ;;
    esac
    printf '%s\n' "$_sk_cursor" >"$STUB/cursor"
  done
}
capture_screen() {
  _cs_count=$(($(cat "$STUB/captures") + 1))
  printf '%s\n' "$_cs_count" >"$STUB/captures"
  _cs_cursor=$(cat "$STUB/cursor")
  if [ -f "$STUB/drift" ] && [ "$_cs_count" -gt "$(cat "$STUB/drift")" ]; then
    _cs_cursor=$((_cs_cursor + 1))
  fi
  printf '> Type your message\n'
  _cs_index=0
  while IFS= read -r _cs_label; do
    if [ "$_cs_index" -eq "$_cs_cursor" ]; then printf '│ ● %s │\n' "$_cs_label"
    else printf '│   %s │\n' "$_cs_label"; fi
    _cs_index=$((_cs_index + 1))
  done <"$STUB/labels"
}
wait_for() { return "$(cat "$STUB/wait")"; }
state_get() { sed -n "s/^$1=//p" "$STUB/state" | tail -n 1; }
state_set() { printf '%s=%s\n' "$1" "$2" >>"$STUB/state"; }
round_log() { printf '%s\n' "$*" >>"$STUB/round.log"; }
"""
REPORT = (
    'code=$?; printf "code=%s\\nnote=%s\\noptions=%s\\n" "$code" "$STEP_NOTE" "${_ret_options:-}"'
)


def make_stub(tmp_path: pathlib.Path, labels: list[str], mode: str = "stop") -> pathlib.Path:
    """Return a stub directory holding a dialog with `labels`, the cursor on the first."""
    stub = tmp_path / "stub"
    (stub / "screens").mkdir(parents=True, exist_ok=True)
    (stub / "labels").write_text("".join(label + "\n" for label in labels), encoding="utf-8")
    for name, value in (("cursor", "0"), ("mode", mode), ("captures", "0"), ("wait", "0")):
        (stub / name).write_text(value + "\n", encoding="utf-8")
    for name in ("state", "drive.log", "round.log"):
        (stub / name).write_text("", encoding="utf-8")
    return stub


def run_steps(shell: str, stub: pathlib.Path, script: str, text: str = "-") -> dict[str, str]:
    """Run `script` after sourcing the libraries and stubs; return its code, note and options."""
    setup = (
        '. "$1"; . "$2"; %s\nSTUB=$3; CELL=$3; STEP_NUMBER=05; STEP_NAME=pick; STEP_KIND=choose\n'
        "STEP_TEXT=$4; STEP_EXPECT=-; AGENT_OPTION='^ *● '; OPTION_PRESSES=12\n" % STUBS
    )
    result = subprocess.run(
        [
            shell,
            "-c",
            setup + script + "\n" + REPORT,
            "steps-test",
            str(ROUND / "lib" / "screen.sh"),
            str(ROUND / "lib" / "steps.sh"),
            str(stub),
            text,
        ],
        capture_output=True,
        check=True,
        text=True,
        encoding="utf-8",
    )
    found = dict(line.split("=", 1) for line in result.stdout.split("\n") if "=" in line)
    found["cursor"] = (stub / "cursor").read_text(encoding="utf-8").strip()
    found["drive"] = (stub / "drive.log").read_text(encoding="utf-8")
    return found


LABELS = ["Allow once", "Allow for this session", "No"]


def test_read_options_after_a_wrap(tmp_path):
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, LABELS, "wrap"), "read_options 12")
        assert found["options"] == "3" and "Up" not in found["drive"], shell
        assert found["note"] == "3 presses of Down, the cursor wrapped", shell


def test_read_options_after_a_stop(tmp_path):
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, LABELS), "read_options 12")
        assert found["options"] == "3" and found["drive"].endswith("keys Up,Up\n"), shell
        assert found["cursor"] == "0" and found["note"].endswith("the cursor stopped"), shell


def test_read_options_at_the_limit(tmp_path):
    labels = ["one", "two", "three", "four", "five"]
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, labels), "read_options 2")
        assert found["options"] == "3" and found["drive"].endswith("keys Up,Up\n"), shell
        assert found["cursor"] == "0" and found["note"].endswith("the cursor limit"), shell


def test_answer_dialog_answers_with_the_first_option(tmp_path):
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, LABELS, "wrap"), "answer_dialog")
        assert found["code"] == "0" and found["drive"].endswith("keys Enter\n"), shell
        assert found["note"] == "answered with the first option, 3 options", shell
        assert found["cursor"] == "0", shell


def test_answer_dialog_without_a_dialog_sends_nothing(tmp_path):
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, []), "answer_dialog")
        assert found["code"] == "0" and found["note"] == "no dialog", shell
        assert found["drive"] == "", shell


def test_answer_dialog_with_the_expectation_missed(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, LABELS, "wrap")
        (stub / "wait").write_text("1\n", encoding="utf-8")
        found = run_steps(shell, stub, "answer_dialog")
        assert found["code"] == "2" and "expectation not met" in found["note"], shell
        assert found["note"].startswith("answered with the first option, 3 options"), shell


def test_choose_option_on_the_first_option(tmp_path):
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, LABELS), "choose_option", "Allow once")
        assert found["code"] == "0" and found["cursor"] == "0", shell
        assert found["note"] == 'cursor on "● Allow once"', shell
        assert "Enter" not in found["drive"], shell


def test_choose_option_on_the_third_option(tmp_path):
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, LABELS), "choose_option", "No$")
        assert found["code"] == "0" and found["cursor"] == "2", shell
        assert found["note"] == 'cursor on "● No"', shell
        assert found["drive"].endswith("keys Down,Down\n"), shell


def test_choose_option_without_a_dialog(tmp_path):
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, []), "choose_option", "Allow once")
        assert found["code"] == "2" and found["note"] == "missed: no dialog open", shell
        assert found["drive"] == "", shell


def test_choose_option_with_no_matching_option(tmp_path):
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, LABELS), "choose_option", "Always")
        assert found["code"] == "2" and found["cursor"] == "0", shell
        assert found["note"] == "missed: no option matches Always", shell


def test_choose_option_with_the_cursor_ending_elsewhere(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, [*LABELS, "Cancel"])
        # One capture for the dialog check, five while reading: the last one drifts.
        (stub / "drift").write_text("6\n", encoding="utf-8")
        found = run_steps(shell, stub, "choose_option", "No$")
        assert found["code"] == "2", shell
        assert found["note"] == 'missed: the cursor ended on "● Cancel", not No$', shell


def keys_after_choose(shell: str, stub: pathlib.Path, text: str, failing: bool) -> dict[str, str]:
    """Run a choose step, note its outcome, then a keys step pressing Enter."""
    fail = 'touch "$STUB/fail"; ' if failing else ""
    script = (
        '%schoose_option; note_choose $?; rm -f "$STUB/fail"; : >"$STUB/drive.log"\n'
        "STEP_KIND=keys; STEP_TEXT=Enter; STEP_NOTE=; press_step_keys" % fail
    )
    return run_steps(shell, stub, script, text)


def test_press_step_keys_after_a_missed_choose_sends_nothing(tmp_path):
    for shell in SHELLS:
        found = keys_after_choose(shell, make_stub(tmp_path, LABELS), "Always", False)
        assert found["code"] == "2" and found["drive"] == "", shell
        assert found["note"].startswith("missed: no key sent"), shell


def test_press_step_keys_after_a_failed_choose_sends_nothing(tmp_path):
    for shell in SHELLS:
        found = keys_after_choose(shell, make_stub(tmp_path, LABELS), "No$", True)
        assert found["code"] == "2" and found["drive"] == "", shell


def test_press_step_keys_after_a_placed_choose_presses_enter(tmp_path):
    for shell in SHELLS:
        found = keys_after_choose(shell, make_stub(tmp_path, LABELS), "No$", False)
        assert found["code"] == "0" and found["drive"] == "keys Enter\n", shell


def test_stop_after_reached_at_the_paused_step(tmp_path):
    cases = [("9", "0"), ("09", "0"), ("10", "1"), ("", "1")]
    for shell in SHELLS:
        for stop_after, expected in cases:
            stub = make_stub(tmp_path, [])
            script = "ROUND_STOP_AFTER=%s; stop_after_reached 09 09-sign-in" % stop_after
            found = run_steps(shell, stub, script)
            assert found["code"] == expected, (shell, stop_after)
            logged = (stub / "round.log").read_text(encoding="utf-8")
            assert ("stopped after step 09-sign-in" in logged) is (expected == "0"), shell


TABLE = (
    "NN\tname\tkind\ttext\texpect\tclaim\n"
    "02\tstart\tagent\tgemini\t-\tc\n"
    "03\tauth-subcommands\ttype\t/auth \tquiet\tc\n"
    "04\tauth-subcommands-clear\tclear\t-\tquiet\tc\n"
    "05\tmodel-subcommands\ttype\t/model \tquiet\tc\n"
    "06\tmodel-subcommands-clear\tclear\t-\tquiet\tc\n"
)


def test_clear_counts_the_nearest_typed_text(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, [])
        (stub / "steps.tsv").write_text(TABLE, encoding="utf-8")
        cases = [("04", "6", "/auth "), ("06", "7", "/model ")]
        for number, count, typed in cases:
            (stub / "drive.log").write_text("", encoding="utf-8")
            script = "STEP_KIND=clear; STEP_NUMBER=%s; clear_typed" % number
            found = run_steps(shell, stub, script)
            assert found["code"] == "0", (shell, number)
            assert found["note"] == "%s presses of Backspace" % count, (shell, typed)
            assert found["drive"] == "keys %s\n" % ",".join(["Backspace"] * int(count)), shell


def test_clear_with_a_number_and_a_missed_settle(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, [])
        (stub / "wait").write_text("1\n", encoding="utf-8")
        found = run_steps(shell, stub, "STEP_KIND=clear; clear_typed", "3")
        assert found["code"] == "2" and found["drive"] == "keys Backspace,Backspace,Backspace\n"
