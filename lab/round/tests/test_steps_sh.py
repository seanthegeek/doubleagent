"""Tests of lib/steps.sh, the dialog step kinds, run under sh and dash against stubbed calls.

The stubs replace drive, capture_screen, wait_for, state_get, state_set, round_log, die,
guest_copy, guest_shell, end_session and paste_source; drive and guest_shell keep the text a
session start or a guest command received on stdin.
They keep a scripted dialog in a directory: its option labels, the cursor, whether Down wraps
or stops at the last option, whether Up moves it, whether Escape closes it, a banner shown for
the first captures only, the capture after which the screen cannot be read, and every drive
call, so a test reads what the step pressed and where the cursor ended. The screen draws the
input line as Gemini CLI does, `│ > Type your message │` when empty or `│ > <text> │` with the
text of `typed_input`, the prefix that of `input_prefix` when present, and only while no
dialog is live (no labels), since Gemini CLI draws a dialog in place of its input box. `stale`
holds an old dialog box and a past prompt, printed above the rest as a restart leaves them in
the scrollback, `extra` lines above the dialog (a title), and `lower_list` the unfocused second
list a two-list dialog draws below the focused one.
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
  if [ -f "$STUB/fail_on" ]; then
    case "$*" in $(cat "$STUB/fail_on")) return 1 ;; esac
  fi
  [ "$1" != keys ] || stub_keys "$2"
  [ "$1" != type ] || cat >>"$STUB/typed"
  [ "$1" != start ] || cat >>"$STUB/started"
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
      Up) [ -f "$STUB/no_up" ] || [ "$_sk_cursor" -eq 0 ] || _sk_cursor=$((_sk_cursor - 1)) ;;
      Escape) [ ! -f "$STUB/escape_closes" ] || { : >"$STUB/labels"; rm -f "$STUB/extra"; } ;;
    esac
    printf '%s\n' "$_sk_cursor" >"$STUB/cursor"
  done
}
capture_screen() {
  _cs_count=$(($(cat "$STUB/captures") + 1))
  printf '%s\n' "$_cs_count" >"$STUB/captures"
  if [ -f "$STUB/capture_fails_from" ] && [ "$_cs_count" -gt "$(cat "$STUB/capture_fails_from")" ]; then
    return 1
  fi
  _cs_cursor=$(cat "$STUB/cursor")
  if [ -f "$STUB/drift" ] && [ "$_cs_count" -gt "$(cat "$STUB/drift")" ]; then
    _cs_cursor=$((_cs_cursor + 1))
  fi
  [ ! -f "$STUB/stale" ] || cat "$STUB/stale"
  if [ ! -s "$STUB/labels" ]; then
    _cs_prefix='>'
    _cs_text='Type your message'
    [ ! -f "$STUB/input_prefix" ] || _cs_prefix=$(cat "$STUB/input_prefix")
    [ ! -f "$STUB/typed_input" ] || _cs_text=$(cat "$STUB/typed_input")
    printf '│ %s %s │\n' "$_cs_prefix" "$_cs_text"
  fi
  [ ! -f "$STUB/extra" ] || cat "$STUB/extra"
  if [ -f "$STUB/banner_until" ] && [ "$_cs_count" -le "$(cat "$STUB/banner_until")" ]; then
    printf 'Warning: True color (24-bit) support not detected.\n'
  fi
  [ ! -f "$STUB/status" ] || printf 'status %s\n' "$_cs_count"
  _cs_index=0
  while IFS= read -r _cs_label; do
    if [ "$_cs_index" -eq "$_cs_cursor" ]; then printf '│ ● %s │\n' "$_cs_label"
    else printf '│   %s │\n' "$_cs_label"; fi
    _cs_index=$((_cs_index + 1))
  done <"$STUB/labels"
  [ ! -f "$STUB/lower_list" ] || cat "$STUB/lower_list"
}
wait_for() { return "$(cat "$STUB/wait")"; }
state_get() { sed -n "s/^$1=//p" "$STUB/state" | tail -n 1; }
state_set() { printf '%s=%s\n' "$1" "$2" >>"$STUB/state"; }
round_log() { printf '%s\n' "$*" >>"$STUB/round.log"; }
die() { printf 'die: %s\n' "$*" >>"$STUB/round.log"; exit 1; }
guest_copy() { printf 'copy %s\n' "$*" >>"$STUB/drive.log"; }
paste_source() { printf '%s\n' "$1"; }
guest_shell() { cat >>"$STUB/shell"; }
end_session() { printf 'end session\n' >>"$STUB/round.log"; }
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
        "STEP_TEXT=$4; STEP_EXPECT=-; AGENT_OPTION='^ *● '; OPTION_PRESSES=12\n"
        "AGENT_LIVE_AFTER='^[>!*] |is restarting to apply the trust changes'\n" % STUBS
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
        assert found["options"] == "3" and found["drive"].endswith("keys Up\nkeys Up\n"), shell
        assert found["cursor"] == "0" and found["note"].endswith("the cursor stopped"), shell


def test_read_options_at_the_limit(tmp_path):
    labels = ["one", "two", "three", "four", "five"]
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, labels), "read_options 2")
        assert found["options"] == "3" and found["drive"].endswith("keys Up\nkeys Up\n"), shell
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
        assert found["note"] == "missed: no option matches Always; Escape sent after the miss"
        assert found["drive"].endswith("keys Up\nkeys Up\nkeys Escape\n"), shell


def test_choose_option_with_the_cursor_ending_elsewhere(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, [*LABELS, "Cancel"])
        # Captures: the dialog check, five while reading, four returning: the next drifts.
        (stub / "drift").write_text("10\n", encoding="utf-8")
        found = run_steps(shell, stub, "choose_option", "No$")
        assert found["code"] == "2", shell
        note = 'missed: the cursor ended on "● Cancel", not No$; Escape sent after the miss'
        assert found["note"] == note and found["drive"].endswith("keys Escape\n"), shell


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
        assert found["note"] == "the choose step before missed", shell


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


def typed_text(stub: pathlib.Path) -> str:
    """Return what the stubbed drive was given to type."""
    typed = stub / "typed"
    return typed.read_text(encoding="utf-8") if typed.is_file() else ""


def test_a_prompt_with_no_dialog_open_types(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, [])
        found = run_steps(shell, stub, "STEP_KIND=prompt; send_text Enter marker", "what is 3?")
        assert found["code"] == "0" and found["drive"] == "type\nkeys Enter\n", shell
        assert typed_text(stub) == "what is 3?", shell
        (stub / "typed").unlink()


def test_a_prompt_with_an_option_dialog_open_types_nothing(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, LABELS)
        found = run_steps(shell, stub, "STEP_KIND=prompt; send_text Enter marker", "hello")
        assert found["code"] == "2" and found["note"] == "a dialog is open after an Escape", shell
        assert found["drive"] == "keys Escape\n" and typed_text(stub) == "", shell
        assert "choose_missed=yes" in (stub / "state").read_text(encoding="utf-8"), shell
        found = run_steps(shell, stub, "STEP_KIND=keys; STEP_TEXT=Enter; press_step_keys")
        assert found["code"] == "2" and found["note"] == "the choose step before missed", shell


def test_an_unread_screen_stops_the_step_and_sets_the_mark(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, LABELS)
        (stub / "typed").unlink(missing_ok=True)
        (stub / "capture_fails_from").write_text("0\n", encoding="utf-8")
        found = run_steps(shell, stub, "STEP_KIND=prompt; send_text Enter marker", "hello")
        assert found["code"] == "1" and found["note"] == "the screen could not be read", shell
        assert found["drive"] == "" and typed_text(stub) == "", shell
        assert "choose_missed=yes" in (stub / "state").read_text(encoding="utf-8"), shell
        stub = make_stub(tmp_path, LABELS)
        (stub / "escape_closes").write_text("", encoding="utf-8")
        (stub / "capture_fails_from").write_text("1\n", encoding="utf-8")
        found = run_steps(shell, stub, "STEP_KIND=prompt; send_text Enter marker", "hello")
        expected = "the screen could not be read after the Escape"
        assert found["code"] == "1" and found["note"] == expected, shell
        assert found["drive"] == "keys Escape\n" and typed_text(stub) == "", shell
        assert "choose_missed=yes" in (stub / "state").read_text(encoding="utf-8"), shell


def test_a_dialog_whose_reading_cannot_return_sends_no_enter(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, ["Allow once", "Allow for this session", "No", "Cancel"])
        (stub / "no_up").write_text("", encoding="utf-8")
        found = run_steps(shell, stub, "answer_dialog")
        assert found["code"] == "2" and "Enter" not in found["drive"], shell
        assert found["note"].endswith("; cursor not returned; not answered"), shell
        assert "choose_missed=yes" in (stub / "state").read_text(encoding="utf-8"), shell


def test_an_unread_screen_during_a_reading_fails_it(tmp_path):
    for shell in SHELLS:
        for fails_from in ("1", "3"):
            stub = make_stub(tmp_path, LABELS)
            (stub / "capture_fails_from").write_text(fails_from + "\n", encoding="utf-8")
            found = run_steps(shell, stub, "read_options 12")
            assert found["code"] == "1" and found["note"] == "the screen could not be read", (
                shell,
                fails_from,
            )
            assert "choose_missed=yes" in (stub / "state").read_text(encoding="utf-8"), shell


def test_an_unread_screen_fails_the_dialog_choose_and_secret_kinds(tmp_path):
    for shell in SHELLS:
        for script in ("answer_dialog", "choose_option", SECRET):
            stub = (
                secret_stub(tmp_path, "AIza") if script == SECRET else make_stub(tmp_path, LABELS)
            )
            (stub / "typed").unlink(missing_ok=True)
            (stub / "capture_fails_from").write_text("0\n", encoding="utf-8")
            found = run_steps(shell, stub, script, "No$")
            assert found["code"] == "1" and found["note"] == "the screen could not be read", (
                shell,
                script,
            )
            assert found["drive"] == "" and typed_text(stub) == "", (shell, script)
            assert "choose_missed=yes" in (stub / "state").read_text(encoding="utf-8"), shell


def test_a_prompt_closes_an_unforeseen_dialog_with_escape_and_types(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, LABELS)
        (stub / "escape_closes").write_text("", encoding="utf-8")
        found = run_steps(shell, stub, "STEP_KIND=prompt; send_text Enter marker", "hello")
        assert found["code"] == "0" and typed_text(stub) == "hello", shell
        assert found["note"] == "a dialog was open; Escape closed it", shell
        assert found["drive"] == "keys Escape\ntype\nkeys Enter\n", shell
        stub = make_stub(tmp_path, LABELS)
        (stub / "typed").unlink(missing_ok=True)
        (stub / "escape_closes").write_text("", encoding="utf-8")
        found = run_steps(shell, stub, "STEP_KIND=clear; clear_typed", "4")
        assert found["code"] == "0" and "Escape closed it; 4 presses" in found["note"], shell


def test_a_failed_escape_call_sets_the_mark_and_fails_the_step(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, LABELS)
        (stub / "typed").unlink(missing_ok=True)
        (stub / "fail_on").write_text("keys Escape\n", encoding="utf-8")
        script = "STEP_KIND=prompt; send_text Enter marker; _c=$?; note_choose $_c; (exit $_c)"
        found = run_steps(shell, stub, script, "hello")
        assert found["code"] == "1" and typed_text(stub) == "", shell
        expected = "the Escape call failed; its effect on the open dialog is unknown"
        assert found["note"] == expected, shell
        assert "choose_missed=yes" in (stub / "state").read_text(encoding="utf-8"), shell
        (stub / "fail_on").unlink()
        (stub / "escape_closes").write_text("", encoding="utf-8")
        found = run_steps(shell, stub, "STEP_KIND=prompt; send_text Enter marker", "hello")
        assert found["code"] == "0" and typed_text(stub) == "hello", shell


def test_an_escape_that_frees_the_input_clears_the_mark_and_notes_a_miss(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, LABELS)
        (stub / "typed").unlink(missing_ok=True)
        (stub / "escape_closes").write_text("", encoding="utf-8")
        (stub / "state").write_text("choose_missed=yes\n", encoding="utf-8")
        found = run_steps(shell, stub, "STEP_KIND=prompt; send_text Enter marker", "hello")
        assert found["code"] == "0" and typed_text(stub) == "hello", shell
        assert (stub / "state").read_text(encoding="utf-8").endswith("choose_missed=\n"), shell
        stub = make_stub(tmp_path, LABELS)
        (stub / "escape_closes").write_text("", encoding="utf-8")
        (stub / "wait").write_text("2\n", encoding="utf-8")
        found = run_steps(shell, stub, "STEP_KIND=prompt; send_text Enter marker", "hello")
        expected = "a dialog was open; Escape closed it; expectation not met within 300s"
        assert found["code"] == "2" and found["note"] == expected, shell
        stub = make_stub(tmp_path, [])
        (stub / "state").write_text("choose_missed=yes\n", encoding="utf-8")
        found = run_steps(shell, stub, "STEP_KIND=prompt; send_text Enter marker", "hello")
        assert found["code"] == "2" and found["note"] == "the choose step before missed", shell


def test_a_type_step_with_an_input_dialog_open_types_nothing(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, [])
        footer = "│ (Press Enter to submit, Esc to cancel, Ctrl+C to clear stored key) │\n"
        (stub / "extra").write_text(footer, encoding="utf-8")
        script = "AGENT_INPUT_GUARD='clear stored key'; STEP_KIND=type; send_text '' quiet"
        found = run_steps(shell, stub, script, "/auth ")
        assert found["code"] == "2" and found["note"] == "a dialog is open after an Escape", shell
        assert found["drive"] == "keys Escape\n" and typed_text(stub) == "", shell
        found = run_steps(shell, stub, "STEP_KIND=type; send_text '' quiet", "/auth ")
        assert found["code"] == "0" and typed_text(stub) == "/auth ", shell
        (stub / "typed").unlink()


def mark_then(shell: str, stub: pathlib.Path, middle: str, keys: str) -> dict[str, str]:
    """Run a choose that misses, then `middle`, then a keys step pressing `keys`."""
    script = (
        "choose_option; note_choose $?\n%s\n"
        ': >"$STUB/drive.log"; STEP_KIND=keys; STEP_TEXT=%s; STEP_NOTE=; press_step_keys'
        % (middle, keys)
    )
    return run_steps(shell, stub, script, "Always")


def test_the_mark_persists_past_other_steps_and_blocks_only_enter(tmp_path):
    middle = "STEP_KIND=options; STEP_NOTE=; note_choose 0; STEP_KIND=keys; note_choose 0"
    for shell in SHELLS:
        found = mark_then(shell, make_stub(tmp_path, LABELS), middle, "Down,Enter")
        assert found["code"] == "2" and found["drive"] == "", shell
        found = mark_then(shell, make_stub(tmp_path, LABELS), middle, "Down")
        assert found["code"] == "0" and found["drive"] == "keys Down\n", shell


def test_an_answered_dialog_or_a_new_session_clears_the_mark(tmp_path):
    clearing = [
        "STEP_KIND=dialog; STEP_TEXT=-; STEP_NOTE=; answer_dialog; note_choose $?",
        "STEP_KIND=agent; STEP_NOTE=; note_choose 0",
        "STEP_KIND=stop; STEP_NOTE=; note_choose 0",
    ]
    for shell in SHELLS:
        for middle in clearing:
            found = mark_then(shell, make_stub(tmp_path, LABELS), middle, "Enter")
            assert found["code"] == "0" and found["drive"] == "keys Enter\n", (shell, middle)


def test_a_dialog_step_that_found_no_dialog_keeps_the_mark(tmp_path):
    middle = (
        'STEP_KIND=dialog; STEP_TEXT=-; : >"$STUB/labels"; STEP_NOTE=; answer_dialog; '
        "note_choose $?"
    )
    for shell in SHELLS:
        found = mark_then(shell, make_stub(tmp_path, LABELS), middle, "Enter")
        assert found["code"] == "2" and found["drive"] == "", shell


def test_answer_dialog_at_the_press_limit_says_at_least(tmp_path):
    labels = ["one", "two", "three", "four", "five"]
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, labels), "OPTION_PRESSES=2; answer_dialog")
        assert found["note"] == "answered with the first option, at least 3 options", shell


def test_clear_with_a_dialog_open_presses_nothing(tmp_path):
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, LABELS), "STEP_KIND=clear; clear_typed", "4")
        assert found["code"] == "2" and found["note"] == "a dialog is open after an Escape", shell
        assert found["drive"] == "keys Escape\n", shell


def test_a_paste_with_a_dialog_open_sends_nothing(tmp_path):
    expected = "copy short.txt pastes\npaste pastes/short.txt\ntype\nkeys Enter\n"
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, LABELS), "send_paste", "short.txt Sum it.")
        assert found["code"] == "2" and found["note"] == "a dialog is open after an Escape", shell
        assert found["drive"] == "keys Escape\n", shell
        stub = make_stub(tmp_path, [])
        found = run_steps(shell, stub, "send_paste", "short.txt Sum it.")
        assert found["code"] == "0" and found["drive"] == expected, shell
        assert typed_text(stub) == "Sum it.", shell
        (stub / "typed").unlink()


def test_a_prompt_or_paste_while_the_mark_is_set_sends_nothing(tmp_path):
    cases = [
        ("STEP_KIND=prompt; STEP_TEXT=hi; STEP_NOTE=; send_text Enter marker", ""),
        ("STEP_KIND=paste; STEP_TEXT='short.txt Sum'; STEP_NOTE=; send_paste", ""),
        ("STEP_KIND=type; STEP_TEXT='/auth '; STEP_NOTE=; send_text '' quiet", "type\n"),
    ]
    for shell in SHELLS:
        for step, drive_log in cases:
            stub = make_stub(tmp_path, LABELS)
            script = (
                'choose_option; note_choose $?; : >"$STUB/labels"; : >"$STUB/drive.log"\n' + step
            )
            found = run_steps(shell, stub, script, "Always")
            assert found["drive"] == drive_log, (shell, step)
            if not drive_log:
                assert found["code"] == "2", (shell, step)
                assert found["note"] == "the choose step before missed", (shell, step)


def test_a_reboot_or_a_placed_choose_clears_the_mark(tmp_path):
    clearing = [
        "STEP_KIND=reboot; STEP_NOTE=; note_choose 0",
        "STEP_KIND=choose; STEP_TEXT='No$'; STEP_NOTE=; choose_option; note_choose $?",
    ]
    for shell in SHELLS:
        for middle in clearing:
            found = mark_then(shell, make_stub(tmp_path, LABELS), middle, "Enter")
            assert found["code"] == "0" and found["drive"] == "keys Enter\n", (shell, middle)


def test_an_options_step_never_clears_the_mark(tmp_path):
    middles = [
        "STEP_KIND=options; STEP_NOTE=; read_options 12; note_choose $?",
        "STEP_KIND=options; STEP_NOTE='0 presses of Down, the cursor limit'; note_choose 0",
    ]
    for shell in SHELLS:
        for middle in middles:
            found = mark_then(shell, make_stub(tmp_path, LABELS), middle, "Enter")
            assert found["code"] == "2" and found["drive"] == "", (shell, middle)


def test_step_outcome_maps_each_exit_code(tmp_path):
    cases = [
        ("0", "read it", "done", "read it"),
        ("2", "", "missed", "expectation not met within 300s"),
        ("2", "a dialog is open", "missed", "a dialog is open"),
        ("1", "", "failed", "a command of the step failed; see round.log"),
        (
            "1",
            "a dialog was open; Escape closed it",
            "failed",
            "a dialog was open; Escape closed it; a command of the step failed; see round.log",
        ),
    ]
    for shell in SHELLS:
        for code, note, status, expected in cases:
            script = (
                "STEP_KIND=prompt; STEP_NOTE='%s'; step_outcome %s; "
                'STEP_NOTE="$_ret_status|$_ret_note"' % (note, code)
            )
            found = run_steps(shell, make_stub(tmp_path, []), script)
            assert found["note"] == "%s|%s" % (status, expected), (shell, code, note)


def test_step_outcome_keeps_the_choose_mark(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, [])
        run_steps(shell, stub, "STEP_KIND=choose; STEP_NOTE=; step_outcome 2")
        assert (stub / "state").read_text(encoding="utf-8") == "choose_missed=yes\n", shell


def failing(
    shell: str, tmp_path: pathlib.Path, labels: list[str], fail_on: str, script: str, text: str
) -> dict[str, str]:
    """Run `script` with the stub drive failing on calls matching the case pattern `fail_on`."""
    stub = make_stub(tmp_path, labels)
    (stub / "fail_on").write_text(fail_on + "\n", encoding="utf-8")
    return run_steps(shell, stub, script, text)


def test_a_failed_drive_call_fails_each_kind(tmp_path):
    cases = [
        (LABELS, "keys Enter", "answer_dialog", "-"),
        (LABELS, "keys Down,Down", "choose_option", "No$"),
        (LABELS, "keys Escape", "choose_option", "Always"),
        (LABELS, "keys Up*", "read_options 12", "-"),
        ([], "keys Enter", "STEP_KIND=keys; press_step_keys", "Enter"),
        ([], "type", "send_text Enter marker", "hello"),
        ([], "keys Enter", "send_text Enter marker", "hello"),
        ([], "paste*", "send_paste", "short.txt Sum it."),
        ([], "keys Backspace*", "STEP_KIND=clear; clear_typed", "3"),
    ]
    for shell in SHELLS:
        for labels, fail_on, script, text in cases:
            found = failing(shell, tmp_path, labels, fail_on, script, text)
            assert found["code"] == "1", (shell, fail_on, script)


def test_a_missed_wait_misses_each_typing_kind(tmp_path):
    cases = [
        ("STEP_KIND=keys; press_step_keys", "Down"),
        ("send_text Enter marker", "hello"),
        ("send_paste", "short.txt Sum it."),
    ]
    for shell in SHELLS:
        for script, text in cases:
            stub = make_stub(tmp_path, [])
            (stub / "wait").write_text("1\n", encoding="utf-8")
            found = run_steps(shell, stub, script, text)
            assert found["code"] == "2", (shell, script)


def test_a_paste_with_no_text_after_the_file_types_nothing(tmp_path):
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, []), "send_paste", "short.txt")
        assert found["code"] == "0", shell
        assert found["drive"] == "copy short.txt pastes\npaste pastes/short.txt\nkeys Enter\n"


def test_dialog_on_screen_without_an_option_pattern(tmp_path):
    for shell in SHELLS:
        script = "unset AGENT_OPTION; dialog_on_screen -"
        assert run_steps(shell, make_stub(tmp_path, LABELS), script)["code"] == "1", shell
        found = run_steps(shell, make_stub(tmp_path, LABELS), "unset AGENT_OPTION; answer_dialog")
        assert found["note"] == "no dialog" and found["drive"] == "", shell


def test_a_failed_enter_after_a_placed_choose_blocks_the_next_enter(tmp_path):
    script = (
        "choose_option; note_choose $?\n"
        'printf "keys Enter\\n" >"$STUB/fail_on"; STEP_KIND=keys; STEP_TEXT=Enter; STEP_NOTE=\n'
        'press_step_keys; note_choose $?; rm -f "$STUB/fail_on"; : >"$STUB/drive.log"\n'
        "STEP_NOTE=; press_step_keys"
    )
    for shell in SHELLS:
        found = run_steps(shell, make_stub(tmp_path, LABELS), script, "No$")
        assert found["code"] == "2" and found["drive"] == "", shell
        assert found["note"] == "the choose step before missed", shell


def test_read_options_returns_to_an_option_below_the_first(tmp_path):
    labels = ["one", "two", "three", "four"]
    for shell in SHELLS:
        stub = make_stub(tmp_path, labels)
        (stub / "cursor").write_text("1\n", encoding="utf-8")
        found = run_steps(shell, stub, "read_options 12")
        assert found["cursor"] == "1" and "cursor not returned" not in found["note"], shell
        assert found["drive"].endswith("keys Up\nkeys Up\n"), shell


def test_a_reading_that_cannot_get_back_sets_the_mark(tmp_path):
    labels = ["one", "two", "three", "four"]
    for shell in SHELLS:
        stub = make_stub(tmp_path, labels)
        # A list Up does not move in: the cursor stays on the last option read.
        (stub / "no_up").write_text("", encoding="utf-8")
        found = run_steps(shell, stub, "read_options 3")
        assert found["note"].endswith("; cursor not returned"), shell
        assert found["drive"].count("keys Up\n") == 3, shell
        assert "choose_missed=yes" in (stub / "state").read_text(encoding="utf-8"), shell


def test_a_line_outside_the_dialog_changes_no_reading(tmp_path):
    """A banner gone after the first key, or a line that changes at every capture."""
    for shell in SHELLS:
        stub = make_stub(tmp_path, LABELS, "wrap")
        (stub / "banner_until").write_text("1\n", encoding="utf-8")
        found = run_steps(shell, stub, "read_options 12")
        assert found["note"] == "3 presses of Down, the cursor wrapped", shell
        assert found["options"] == "3" and "Up" not in found["drive"], shell
        stub = make_stub(tmp_path, LABELS)
        (stub / "banner_until").write_text("1\n", encoding="utf-8")
        (stub / "status").write_text("", encoding="utf-8")
        found = run_steps(shell, stub, "read_options 12")
        assert found["note"] == "3 presses of Down, the cursor stopped", shell
        assert found["cursor"] == "0" and found["drive"].endswith("keys Up\nkeys Up\n"), shell
        assert "choose_missed" not in (stub / "state").read_text(encoding="utf-8"), shell


def test_option_key_without_an_option_pattern_is_the_screen(tmp_path):
    for shell in SHELLS:
        stub = make_stub(tmp_path, LABELS)
        (stub / "status").write_text("", encoding="utf-8")
        script = "AGENT_OPTION=; read_options 12"
        found = run_steps(shell, stub, script)
        assert found["note"].endswith("the cursor limit; cursor not returned"), shell


def found_nothing_typed(stub: pathlib.Path) -> bool:
    """Return whether the stubbed drive received no call at all."""
    return (stub / "drive.log").read_text(encoding="utf-8") == ""


def secret_stub(tmp_path: pathlib.Path, value: str, dialog: bool = True) -> pathlib.Path:
    """Return a stub with the key dialog on the screen and a credentials file holding `value`."""
    stub = make_stub(tmp_path, [])
    for leftover in ("typed", "fail_on"):
        (stub / leftover).unlink(missing_ok=True)
    if dialog:
        (stub / "extra").write_text("Enter your key (clear stored key)\n", encoding="utf-8")
    (stub / "env").write_text("# keys\nOTHER=x\nGEMINI_API_KEY=%s\r\n" % value, encoding="utf-8")
    return stub


SECRET = (
    "ROUND_ENV=$STUB/env; AGENT_INPUT_GUARD='clear stored key'; STEP_KIND=secret\n"
    "STEP_TEXT=GEMINI_API_KEY; send_secret"
)


def test_send_secret_types_the_value_and_enter(tmp_path):
    for shell in SHELLS:
        stub = secret_stub(tmp_path, "AIza test=key")
        found = run_steps(shell, stub, SECRET)
        assert found["code"] == "0" and typed_text(stub) == "AIza test=key", shell
        assert found["drive"] == "type\nkeys Enter\n", shell
        assert found["note"] == "typed GEMINI_API_KEY from %s/env" % stub, shell
        assert "AIza" not in found["note"] + (stub / "round.log").read_text(encoding="utf-8")


def test_send_secret_without_the_input_dialog_types_nothing(tmp_path):
    for shell in SHELLS:
        stub = secret_stub(tmp_path, "AIza", dialog=False)
        found = run_steps(shell, stub, SECRET)
        assert found["code"] == "2" and found["note"] == "no input dialog open", shell
        assert found["drive"] == "" and not (stub / "typed").exists(), shell


def test_send_secret_without_a_guard_pattern_types_nothing(tmp_path):
    for shell in SHELLS:
        stub = secret_stub(tmp_path, "AIza")
        found = run_steps(shell, stub, SECRET.replace("'clear stored key'", "''"))
        assert found["code"] == "2" and found["drive"] == "", shell
        assert found["note"] == "no AGENT_INPUT_GUARD pattern to find the input dialog by", shell


def test_send_secret_with_the_expectation_missed_keeps_its_note(tmp_path):
    for shell in SHELLS:
        stub = secret_stub(tmp_path, "AIza")
        (stub / "wait").write_text("2\n", encoding="utf-8")
        found = run_steps(shell, stub, SECRET)
        assert found["code"] == "2" and typed_text(stub) == "AIza", shell
        expected = "typed GEMINI_API_KEY from %s/env; expectation not met within 300s" % stub
        assert found["note"] == expected, shell


def test_a_failed_secret_call_sets_the_mark(tmp_path):
    for call in ("type", "keys Enter"):
        for shell in SHELLS:
            stub = secret_stub(tmp_path, "AIza")
            (stub / "fail_on").write_text(call + "\n", encoding="utf-8")
            found = run_steps(shell, stub, SECRET + "; note_choose $?")
            assert "choose_missed=yes" in (stub / "state").read_text(encoding="utf-8"), (
                shell,
                call,
            )
            assert found["drive"].endswith(call + "\n"), (shell, call)
            stub = secret_stub(tmp_path, "AIza")
            found = run_steps(shell, stub, SECRET + "; note_choose $?")
            assert "choose_missed" not in (stub / "state").read_text(encoding="utf-8"), shell


def test_send_secret_while_the_mark_is_set_types_nothing(tmp_path):
    for shell in SHELLS:
        stub = secret_stub(tmp_path, "AIza")
        (stub / "state").write_text("choose_missed=yes\n", encoding="utf-8")
        found = run_steps(shell, stub, SECRET)
        assert found["code"] == "2" and found["note"] == "the choose step before missed", shell
        assert found["drive"] == "", shell


def test_send_secret_with_an_empty_value_dies(tmp_path):
    for shell in SHELLS:
        stub = secret_stub(tmp_path, "")
        try:
            run_steps(shell, stub, SECRET)
        except subprocess.CalledProcessError as error:
            assert error.returncode == 1, shell
        else:
            raise AssertionError("send_secret did not die under " + shell)
        logged = (stub / "round.log").read_text(encoding="utf-8")
        assert logged == "die: GEMINI_API_KEY has no value in %s/env\n" % stub, shell
        assert found_nothing_typed(stub), shell


def test_missing_secrets_names_the_steps_ahead(tmp_path):
    table = (
        "NN\tname\tkind\ttext\texpect\tclaim\n"
        "02\tkey\tsecret\tGEMINI_API_KEY\t-\tc\n"
        "03\ttoken\tsecret\tOTHER\t-\tc\n"
        "04\tlater\tsecret\tLATER\t-\tc\n"
    )
    (tmp_path / "steps.tsv").write_text(table, encoding="utf-8")
    for shell in SHELLS:
        stub = secret_stub(tmp_path, "")
        script = 'ROUND_ENV=$STUB/env; missing_secrets "%s" 3 | tr "\\n" ","' % (
            tmp_path / "steps.tsv"
        )
        found = run_steps(shell, stub, 'printf "missing=%s\\n" "$(' + script + ')"')
        assert found["missing"] == "LATER,", shell
        found = run_steps(
            shell, stub, 'printf "missing=%s\\n" "$(' + script.replace(" 3 ", " 2 ") + ')"'
        )
        assert found["missing"] == "GEMINI_API_KEY,LATER,", shell


def token_stub(tmp_path: pathlib.Path, *lines: str) -> pathlib.Path:
    """Return a stub whose credentials file holds `lines`, each `NAME=value`."""
    stub = make_stub(tmp_path, [])
    for leftover in ("started", "shell"):
        (stub / leftover).unlink(missing_ok=True)
    (stub / "env").write_text("".join(line + "\n" for line in lines), encoding="utf-8")
    return stub


EXPAND = (
    'ROUND_ENV=$STUB/env; expand_secrets "$STEP_TEXT"; printf "expanded=%s\\n" "$_ret_expanded"'
)


def test_expand_secrets_replaces_each_token(tmp_path):
    cases = [
        ("gemini", "gemini"),
        ("KEY=secret:API_KEY gemini", "KEY=a/b&c\\1 gemini"),
        ("secret:API_KEY:secret:OTHER_2.", "a/b&c\\1:two words."),
        ("keep secret:lower and secret: alone", "keep secret:lower and secret: alone"),
    ]
    for shell in SHELLS:
        for text, expected in cases:
            stub = token_stub(tmp_path, "API_KEY=a/b&c\\1", "OTHER_2=two words")
            found = run_steps(shell, stub, EXPAND, text)
            assert found["code"] == "0" and found["expanded"] == expected, (shell, text)


def test_expand_secrets_with_an_empty_value_dies(tmp_path):
    for shell in SHELLS:
        stub = token_stub(tmp_path, "API_KEY=", "OTHER=x")
        try:
            run_steps(shell, stub, EXPAND, "secret:OTHER secret:API_KEY")
        except subprocess.CalledProcessError as error:
            assert error.returncode == 1, shell
        else:
            raise AssertionError("expand_secrets did not die under " + shell)
        logged = (stub / "round.log").read_text(encoding="utf-8")
        assert logged == "die: API_KEY has no value in %s/env\n" % stub, shell


def test_expand_secrets_with_a_single_quote_in_the_value_dies(tmp_path):
    for shell in SHELLS:
        stub = token_stub(tmp_path, "API_KEY=ab'cd")
        try:
            run_steps(shell, stub, EXPAND, "KEY='secret:API_KEY' gemini")
        except subprocess.CalledProcessError as error:
            assert error.returncode == 1, shell
        else:
            raise AssertionError("expand_secrets did not die under " + shell)
        logged = (stub / "round.log").read_text(encoding="utf-8")
        expected = "die: the value of API_KEY in %s/env holds a single quote\n" % stub
        assert logged == expected, shell


def test_missing_secrets_reads_the_tokens_of_agent_and_command_steps(tmp_path):
    table = (
        "NN\tname\tkind\ttext\texpect\tclaim\n"
        "02\tstart\tagent\tin:. KEY=secret:AGENT_KEY OTHER=secret:FILLED gemini\t-\tc\n"
        "03\twrite\tcommand\tprintf '%s %s' secret:COMMAND_KEY secret:AGENT_KEY >f\t-\tc\n"
        "04\task\tprompt\tsecret:NOT_READ\t-\tc\n"
    )
    (tmp_path / "steps.tsv").write_text(table, encoding="utf-8")
    script = 'ROUND_ENV=$STUB/env; missing_secrets "%s" 2 | tr "\\n" ","' % (tmp_path / "steps.tsv")
    for shell in SHELLS:
        stub = token_stub(tmp_path, "FILLED=x")
        found = run_steps(shell, stub, 'printf "missing=%s\\n" "$(' + script + ')"')
        assert found["missing"] == "AGENT_KEY,COMMAND_KEY,", shell


COMMAND_LINE = (
    'agent_command_line "$STEP_TEXT"\n'
    'printf "directory=%s\\ncommand=%s\\n" "$_ret_directory" "$_ret_command"'
)


def test_agent_command_line_splits_the_directory_and_prefixes_env(tmp_path):
    cases = [
        ("linux", "gemini --debug", "lab/proj", "gemini --debug"),
        ("linux", "in:. gemini", ".", "gemini"),
        ("macos", "A_1=$HOME/x gemini", "lab/proj", "env A_1=$HOME/x gemini"),
        ("linux", "in:lab/b KEY=v X=y gemini", "lab/b", "env KEY=v X=y gemini"),
        ("linux", "in:.  KEY=v gemini", ".", "env KEY=v gemini"),
        ("linux", "1X=v gemini", "lab/proj", "1X=v gemini"),
        ("linux", "gemini --flag=x", "lab/proj", "gemini --flag=x"),
        ("windows", "in:. $env:KEY='v'; gemini", ".", "$env:KEY='v'; gemini"),
        ("windows", "KEY=v gemini", "lab/proj", "KEY=v gemini"),
    ]
    for shell in SHELLS:
        for guest, text, directory, command in cases:
            script = "GUEST_OS=%s; %s" % (guest, COMMAND_LINE)
            found = run_steps(shell, make_stub(tmp_path, []), script, text)
            assert found["directory"] == directory, (shell, guest, text)
            assert found["command"] == command, (shell, guest, text)


def test_start_agent_logs_the_token_and_sends_the_value(tmp_path):
    for shell in SHELLS:
        stub = token_stub(tmp_path, "API_KEY=AIza/&x")
        script = 'ROUND_ENV=$STUB/env; GUEST_OS=linux; start_agent "$STEP_TEXT"'
        found = run_steps(shell, stub, script, "in:. KEY='secret:API_KEY' gemini")
        assert found["code"] == "0" and found["drive"] == "start 120 40 .\n", shell
        started = (stub / "started").read_text(encoding="utf-8")
        assert started == "env KEY='AIza/&x' gemini\n", shell
        logged = (stub / "round.log").read_text(encoding="utf-8")
        assert logged == "end session\nstart session s05 in ~/.: env KEY='secret:API_KEY' gemini\n"
        assert "AIza" not in logged + (stub / "state").read_text(encoding="utf-8"), shell


def test_run_command_logs_the_token_and_sends_the_value(tmp_path):
    for shell in SHELLS:
        stub = token_stub(tmp_path, "API_KEY=AIza/&x")
        text = "printf 'K=%s\\n' 'secret:API_KEY' >f"
        found = run_steps(shell, stub, "ROUND_ENV=$STUB/env; run_command", text)
        assert found["code"] == "0", shell
        shell_input = (stub / "shell").read_text(encoding="utf-8")
        assert shell_input == "printf 'K=%s\\n' 'AIza/&x' >f\n", shell
        logged = (stub / "round.log").read_text(encoding="utf-8")
        assert logged == "command: " + text + "\n" and "AIza" not in found["note"], shell
        assert (stub / "screens" / "05-pick.command.txt").is_file(), shell


STALE_BOX = "│ ● 1. Trust folder (proj) │\n│   2. Don't trust │\n"
STALE_NOTICE = "Gemini CLI is restarting to apply the trust changes...\n"
STALE_PROMPT = "> what is 3?\n"
STALE = STALE_BOX + STALE_NOTICE + STALE_PROMPT


def stale_stub(
    tmp_path: pathlib.Path, labels: list[str], mode: str = "stop", scrollback: str = STALE
) -> pathlib.Path:
    """Return a stub with `scrollback` (an old dialog box and what followed it) above the screen.

    Each test passes the scrollback whose last line is the one cut it proves: the box alone leaves
    the input line as the only cut, the box and a past prompt the prompt, the box and the notice
    the notice.
    """
    stub = make_stub(tmp_path, labels, mode)
    (stub / "stale").write_text(scrollback, encoding="utf-8")
    (stub / "typed").unlink(missing_ok=True)
    return stub


def test_a_prompt_below_a_dialog_in_the_scrollback_types(tmp_path):
    for shell in SHELLS:
        stub = stale_stub(tmp_path, [], scrollback=STALE_BOX)
        found = run_steps(shell, stub, "STEP_KIND=prompt; send_text Enter marker", "what is 5?")
        assert found["code"] == "0" and found["note"] == "", shell
        assert found["drive"] == "type\nkeys Enter\n" and typed_text(stub) == "what is 5?", shell


def test_a_typed_input_below_a_dialog_in_the_scrollback_is_free(tmp_path):
    """The placeholder is gone once text is typed; the input line alone still cuts the box."""
    for shell in SHELLS:
        stub = stale_stub(tmp_path, [], scrollback=STALE_BOX)
        (stub / "typed_input").write_text("/help ", encoding="utf-8")
        assert run_steps(shell, stub, "dialog_open")["code"] == "1", shell
        found = run_steps(shell, stub, "STEP_KIND=clear; clear_typed", "6")
        assert found["code"] == "0" and found["note"] == "6 presses of Backspace", shell
        assert found["drive"] == "keys %s\n" % ",".join(["Backspace"] * 6), shell


def test_choose_and_dialog_find_no_dialog_in_the_scrollback(tmp_path):
    for shell in SHELLS:
        found = run_steps(shell, stale_stub(tmp_path, []), "choose_option", "Trust folder")
        assert found["code"] == "2" and found["note"] == "missed: no dialog open", shell
        assert found["drive"] == "", shell
        found = run_steps(shell, stale_stub(tmp_path, []), "STEP_KIND=dialog; answer_dialog")
        assert found["code"] == "0" and found["note"] == "no dialog", shell
        assert found["drive"] == "", shell


def test_a_type_step_with_the_mark_set_and_a_scrollback_dialog_types(tmp_path):
    for shell in SHELLS:
        stub = stale_stub(tmp_path, [])
        (stub / "state").write_text("choose_missed=yes\n", encoding="utf-8")
        found = run_steps(shell, stub, "STEP_KIND=type; send_text '' quiet", "/auth ")
        assert found["code"] == "0" and found["drive"] == "type\n", shell
        assert typed_text(stub) == "/auth ", shell
        assert (stub / "state").read_text(encoding="utf-8") == "choose_missed=yes\n", shell


def test_a_live_dialog_below_a_past_prompt_is_read_not_the_stale_box(tmp_path):
    """The past prompt alone, no notice above it, cuts the stale box."""
    scrollback = STALE_BOX + STALE_PROMPT
    for shell in SHELLS:
        stub = stale_stub(tmp_path, LABELS, scrollback=scrollback)
        found = run_steps(shell, stub, "choose_option", "No$")
        assert found["code"] == "0" and found["cursor"] == "2", shell
        assert found["note"] == 'cursor on "● No"', shell
        assert found["drive"].endswith("keys Down,Down\n"), shell
        option = (stub / "screens" / "05-pick.option-1.txt").read_text(encoding="utf-8")
        assert option.startswith(scrollback), shell
        stub = stale_stub(tmp_path, LABELS, "wrap", scrollback=scrollback)
        found = run_steps(shell, stub, "answer_dialog")
        assert found["note"] == "answered with the first option, 3 options", shell
        assert found["cursor"] == "0" and found["drive"].endswith("keys Enter\n"), shell


SETTINGS = ["Vim Mode", "Auto Update", "Hide Banner"]
SCOPES = ["1. User Settings", "2. Workspace Settings", "3. System Settings"]


def settings_stub(tmp_path: pathlib.Path, focus: str) -> pathlib.Path:
    """Return a stub drawing /settings with the focus on its `settings` or `scope` list.

    The focused section's title carries `> `, the other's two spaces; the settings list has no
    numbers and marks its item with ● only while focused, the scope list numbers its items only
    while focused and always marks the selected one.
    """
    if focus == "settings":
        stub = stale_stub(tmp_path, SETTINGS)
        (stub / "extra").write_text("│ > Settings │\n", encoding="utf-8")
        lower = "│   Apply To │\n│ ● User Settings │\n│   Workspace Settings │\n"
        (stub / "lower_list").write_text(lower, encoding="utf-8")
    else:
        stub = stale_stub(tmp_path, SCOPES)
        above = "│   Settings │\n│   Vim Mode │\n│   Auto Update │\n│ > Apply To │\n"
        (stub / "extra").write_text(above, encoding="utf-8")
    return stub


def test_a_two_list_dialog_is_read_by_its_focused_list(tmp_path):
    """The unfocused scope list below keeps its own constant ● line, as /settings draws it."""
    for shell in SHELLS:
        found = run_steps(shell, settings_stub(tmp_path, "settings"), "read_options 12")
        assert found["options"] == "3" and found["cursor"] == "0", shell
        assert found["note"] == "3 presses of Down, the cursor stopped", shell
        found = run_steps(shell, settings_stub(tmp_path, "settings"), "choose_option", "Auto")
        assert found["code"] == "0" and found["cursor"] == "1", shell
        assert found["note"] == 'cursor on "● Auto Update"', shell


def test_a_focused_scope_list_is_read_below_its_marker(tmp_path):
    for shell in SHELLS:
        found = run_steps(shell, settings_stub(tmp_path, "scope"), "read_options 12")
        assert found["options"] == "3" and found["cursor"] == "0", shell
        found = run_steps(shell, settings_stub(tmp_path, "scope"), "choose_option", "Workspace")
        assert found["code"] == "0" and found["cursor"] == "1", shell
        assert found["note"] == 'cursor on "● 2. Workspace Settings"', shell
        found = run_steps(shell, settings_stub(tmp_path, "scope"), "answer_dialog")
        assert found["note"] == "answered with the first option, 3 options", shell


def test_without_a_live_after_pattern_a_dialog_anywhere_counts(tmp_path):
    for shell in SHELLS:
        stub = stale_stub(tmp_path, [])
        script = "AGENT_LIVE_AFTER=; STEP_KIND=prompt; send_text Enter marker"
        found = run_steps(shell, stub, script, "hello")
        assert found["code"] == "2" and found["note"] == "a dialog is open after an Escape", shell
        assert found["drive"] == "keys Escape\n" and typed_text(stub) == "", shell
        script = "AGENT_LIVE_AFTER=; choose_option"
        found = run_steps(shell, stale_stub(tmp_path, []), script, "Trust")
        assert found["code"] == "0" and found["cursor"] == "0", shell


RESTART = (
    "│ ● 1. Trust folder (proj) │\n"
    "│   2. Trust parent folder (lab) │\n"
    "│   3. Don't trust │\n"
    " Gemini CLI is restarting to apply the trust changes...\n"
    "  GEMINI CLI banner\n"
)
SIGN_IN = ["1. Sign in with Google", "2. Use Gemini API Key", "3. Vertex AI"]


def restart_stub(tmp_path: pathlib.Path, mode: str = "stop") -> pathlib.Path:
    """Return a stub drawn after a trust restart: the old trust box, the notice, a live dialog."""
    stub = make_stub(tmp_path, SIGN_IN, mode)
    (stub / "stale").write_text(RESTART, encoding="utf-8")
    return stub


def test_a_dialog_after_a_trust_restart_is_read_below_the_notice(tmp_path):
    """No input line and no past prompt: the restart notice alone cuts the old trust box."""
    for shell in SHELLS:
        assert run_steps(shell, restart_stub(tmp_path), "dialog_open")["code"] == "0", shell
        found = run_steps(shell, restart_stub(tmp_path), "choose_option", "Gemini API Key")
        assert found["code"] == "0" and found["cursor"] == "1", shell
        assert found["note"] == 'cursor on "● 2. Use Gemini API Key"', shell
        found = run_steps(shell, restart_stub(tmp_path, "wrap"), "answer_dialog")
        assert found["note"] == "answered with the first option, 3 options", shell
        assert found["cursor"] == "0" and found["drive"].endswith("keys Enter\n"), shell


def test_a_yolo_input_line_below_a_dialog_in_the_scrollback_is_free(tmp_path):
    for shell in SHELLS:
        for typed in ("", "/help "):
            stub = stale_stub(tmp_path, [], scrollback=STALE_BOX)
            (stub / "input_prefix").write_text("*", encoding="utf-8")
            if typed:
                (stub / "typed_input").write_text(typed, encoding="utf-8")
            assert run_steps(shell, stub, "dialog_open")["code"] == "1", (shell, typed)
            found = run_steps(shell, stub, "STEP_KIND=prompt; send_text Enter marker", "hi")
            assert found["code"] == "0" and found["drive"] == "type\nkeys Enter\n", shell
