# shellcheck shell=sh
# steps.sh: the step kinds that read and answer an agent's dialogs, and the
# step bookkeeping round.sh runs between steps, sourced by round.sh after
# screen.sh on the analyst machine and by the round pipeline's tests.
#
# Calls drive, capture_screen, wait_for, state_get, state_set, round_log,
# guest_copy and paste_source, which round.sh and vm.sh define and a test
# replaces with stubs. Reads the
# globals CELL, STEP_NUMBER, STEP_NAME, STEP_KIND, STEP_TEXT, STEP_EXPECT,
# AGENT_OPTION, AGENT_INPUT_GUARD, TURN_SECONDS and ROUND_STOP_AFTER, and
# sets STEP_NOTE.

# The globals above come from round.sh, which also calls every function.
# shellcheck disable=SC2153,SC2154,SC2329

# The most Down presses a `dialog` or `choose` step makes to read a dialog.
: "${OPTION_PRESSES:=12}"

# Print a step number without its leading zeros, so `08` compares as 8.
number_value() {
  printf '%s\n' "$1" | sed 's/^0*\([0-9]\)/\1/'
}

# Press the key $1 $2 times in one drive call and wait for the screen to
# settle; nothing when $2 is 0. Only a failed drive call fails: a screen
# that never settles is read as it stands, as the next capture shows it.
press_times() {
  _pt_count=$2
  _pt_keys=
  while [ "$_pt_count" -gt 0 ]; do
    _pt_keys=${_pt_keys:+$_pt_keys,}$1
    _pt_count=$((_pt_count - 1))
  done
  [ -n "$_pt_keys" ] || return 0
  drive keys "$_pt_keys" </dev/null >/dev/null || return 1
  wait_for quiet quiet >/dev/null || :
}

# Press Down once, wait for the screen to settle, save the screen as the
# file $1 and print it.
press_down_saved() {
  press_times Down 1 || return 1
  _pds_screen=$(capture_screen)
  printf '%s\n' "$_pds_screen" >"$1"
  printf '%s\n' "$_pds_screen"
}

# Read every option of the open dialog: press Down, at most $1 times, saving
# each settled screen as screens/NN-name.option-K.txt, until the screen is
# the first one again (the cursor wrapped) or the same as before (it
# stopped), then press Up back to the first option. Sets STEP_NOTE,
# _ret_end (wrapped, stopped or limit), _ret_options (the options read: at
# the press limit a lower bound, since one more may follow) and _ret_first
# (the screen on the first option).
read_options() {
  _ret_first=$(capture_screen)
  _rop_previous=$_ret_first
  _rop_moves=0
  _rop_end=limit
  while [ "$_rop_moves" -lt "$1" ]; do
    _rop_moves=$((_rop_moves + 1))
    _rop_screen=$(press_down_saved \
      "$CELL/screens/$STEP_NUMBER-$STEP_NAME.option-$_rop_moves.txt") || return 1
    [ "$_rop_screen" != "$_ret_first" ] || _rop_end=wrapped
    [ "$_rop_screen" != "$_rop_previous" ] || _rop_end=stopped
    [ "$_rop_end" = limit ] || break
    _rop_previous=$_rop_screen
  done
  # A wrap is back on the first option; a stop is one Up per move that
  # changed the screen; a limit is one Up per move and one option unread.
  case $_rop_end in
    wrapped) _ret_options=$_rop_moves && _rop_back=0 ;;
    stopped) _ret_options=$_rop_moves && _rop_back=$((_rop_moves - 1)) ;;
    *) _ret_options=$((_rop_moves + 1)) && _rop_back=$_rop_moves ;;
  esac
  press_times Up "$_rop_back" || return 1
  _ret_end=$_rop_end
  STEP_NOTE="$_rop_moves presses of Down, the cursor $_rop_end"
}

# Succeed when the grep -E pattern $1 (`-` for the agent's AGENT_OPTION) is
# on the current screen, its box-drawing characters stripped.
dialog_on_screen() {
  _dos_pattern=$1
  [ "$_dos_pattern" != - ] || _dos_pattern=${AGENT_OPTION:-}
  [ -n "$_dos_pattern" ] || return 1
  capture_screen | strip_box | grep -Eq -- "$_dos_pattern"
}

# The `dialog` kind: when the step's pattern is on the screen, read the
# options and answer with the first; otherwise note that no dialog opened,
# which is a finding and not a miss.
answer_dialog() {
  if ! dialog_on_screen "$STEP_TEXT"; then
    STEP_NOTE='no dialog'
    return 0
  fi
  read_options "$OPTION_PRESSES" || return 1
  drive keys Enter </dev/null >/dev/null || return 1
  _ad_count="$_ret_options options"
  [ "$_ret_end" != limit ] || _ad_count="at least $_ad_count"
  STEP_NOTE="answered with the first option, $_ad_count"
  wait_for "$STEP_EXPECT" marker >/dev/null && return 0
  STEP_NOTE="$STEP_NOTE; expectation not met within ${TURN_SECONDS:-300}s"
  return 2
}

# The `choose` kind: read the open dialog's options, then press Down until
# the highlighted line matches the step's pattern, leaving the Enter to the
# next step. Return 2 with the reason in STEP_NOTE when no dialog is open
# (no key sent), when no option matches (the cursor back on the first) or
# when the cursor ends on a line the pattern does not match.
choose_option() {
  dialog_on_screen - || {
    STEP_NOTE='missed: no dialog open'
    return 2
  }
  read_options "$OPTION_PRESSES" || return 1
  _co_index=0
  _co_line=$(printf '%s\n' "$_ret_first" | highlighted_line "${AGENT_OPTION:-}")
  while ! printf '%s\n' "$_co_line" | grep -Eq -- "$STEP_TEXT"; do
    _co_index=$((_co_index + 1))
    if [ "$_co_index" -ge "$_ret_options" ]; then
      STEP_NOTE="missed: no option matches $STEP_TEXT"
      return 2
    fi
    _co_line=$(highlighted_line "${AGENT_OPTION:-}" \
      <"$CELL/screens/$STEP_NUMBER-$STEP_NAME.option-$_co_index.txt")
  done
  press_times Down "$_co_index" || return 1
  _co_line=$(capture_screen | highlighted_line "${AGENT_OPTION:-}")
  STEP_NOTE="cursor on \"$_co_line\""
  printf '%s\n' "$_co_line" | grep -Eq -- "$STEP_TEXT" && return 0
  STEP_NOTE="missed: the cursor ended on \"$_co_line\", not $STEP_TEXT"
  return 2
}

# Succeed unless the choose_missed mark is set (see note_choose); then set
# STEP_NOTE and return 2, for a step about to press Enter, which would
# answer a dialog with an option the table did not mean.
enter_allowed() {
  [ "$(state_get choose_missed)" = yes ] || return 0
  STEP_NOTE='the choose step before missed'
  return 2
}

# The `keys` kind; keys that include Enter wait for enter_allowed.
press_step_keys() {
  case ,$STEP_TEXT, in
    *Enter,*) enter_allowed || return 2 ;;
  esac
  drive keys "$STEP_TEXT" </dev/null >/dev/null || return 1
  wait_for "$STEP_EXPECT" quiet >/dev/null || return 2
}

# Keep the state's choose_missed mark after the step just run, whose exit
# code is $1. A `choose` that did not place the cursor (a miss or a failed
# drive call) sets it; a `choose` that did, a `dialog` that read and
# answered a dialog, an `options` step that read one (the cursor wrapped or
# stopped, so the next Enter is meant for it), and an `agent`, `stop` or
# `reboot` step (a new or closed session) clear it; every other step keeps
# it.
note_choose() {
  _nc_mark=$(state_get choose_missed)
  _nc_new=$_nc_mark
  case $STEP_KIND in
    choose) if [ "$1" -eq 0 ]; then _nc_new=; else _nc_new=yes; fi ;;
    dialog) case $STEP_NOTE in answered*) _nc_new= ;; esac ;;
    options) case $STEP_NOTE in *wrapped | *stopped) _nc_new= ;; esac ;;
    agent | stop | reboot) _nc_new= ;;
  esac
  [ "$_nc_mark" = "$_nc_new" ] || state_set choose_missed "$_nc_new"
}

# Succeed, logging so, when ROUND_STOP_AFTER names step number $1 (compared
# as numbers); $2 is the step's label.
stop_after_reached() {
  [ -n "${ROUND_STOP_AFTER:-}" ] || return 1
  [ "$(number_value "$1")" -eq "$(number_value "$ROUND_STOP_AFTER")" ] || return 1
  round_log "stopped after step $2 (ROUND_STOP_AFTER); resume goes on"
}

# Succeed when the input is free to type into: no dialog is open, as the
# agent's AGENT_OPTION pattern (a highlighted option) or AGENT_INPUT_GUARD
# pattern (an input dialog with no option, such as a key prompt) shows on
# the screen, box-drawing characters stripped. Otherwise set STEP_NOTE and
# return 2: a prompt never belongs in a dialog, which the tables answer
# with their own steps, and one typed into a key prompt can become the key.
input_free() {
  _inf_screen=$(capture_screen | strip_box)
  for _inf_pattern in "${AGENT_OPTION:-}" "${AGENT_INPUT_GUARD:-}"; do
    [ -n "$_inf_pattern" ] || continue
    if printf '%s\n' "$_inf_screen" | grep -Eq -- "$_inf_pattern"; then
      STEP_NOTE='a dialog is open'
      return 2
    fi
  done
}

# The `prompt` and `type` kinds: when the input is free, type the step's
# text, press the key $1 after it unless $1 is empty (and then only when
# enter_allowed), and wait for the step's expectation, `-` standing for $2.
send_text() {
  input_free || return 2
  if [ -n "$1" ]; then enter_allowed || return 2; fi
  printf '%s' "$STEP_TEXT" | drive type >/dev/null || return 1
  if [ -n "$1" ]; then drive keys "$1" </dev/null >/dev/null || return 1; fi
  wait_for "$STEP_EXPECT" "$2" >/dev/null || return 2
}

# Print the text of the last `type` step of the cell's steps.tsv numbered
# below $1, its trailing spaces kept, for a `clear` step to count.
previous_type_text() {
  awk -F '	' -v before="$1" '$0 !~ /^#/ && NF > 0 && $1 != "NN" &&
    $1 + 0 < before + 0 && $3 == "type" { text = $4 } END { print text }' "$CELL/steps.tsv"
}

# The `clear` kind: press Backspace STEP_TEXT times or, for `-`, once per
# character of the text of the nearest `type` step before this one
# (previous_type_text), in one drive call, then wait for a settled screen.
# Backspace, since a double Escape needs two presses closer together than
# drive sends them, and on an empty input opens a rewind picker. Nothing is
# pressed while a dialog is open (input_free).
clear_typed() {
  input_free || return 2
  _ct_count=$STEP_TEXT
  if [ "$_ct_count" = - ]; then
    _ct_text=$(previous_type_text "$STEP_NUMBER")
    _ct_count=${#_ct_text}
  fi
  press_times Backspace "$_ct_count" || return 1
  STEP_NOTE="$_ct_count presses of Backspace"
  wait_for "$STEP_EXPECT" quiet >/dev/null || return 2
}

# The `paste` kind: when the input is free and enter_allowed, copy the
# paste file into the VM, send it as one bracketed paste, type the text
# after the file name, press Enter and wait.
send_paste() {
  input_free || return 2
  enter_allowed || return 2
  _sp_file=${STEP_TEXT%% *}
  _sp_rest=${STEP_TEXT#"$_sp_file"}
  _sp_rest=${_sp_rest# }
  guest_copy "$(paste_source "$_sp_file")" pastes
  drive paste "pastes/$_sp_file" </dev/null >/dev/null || return 1
  if [ -n "$_sp_rest" ]; then printf '%s' "$_sp_rest" | drive type >/dev/null || return 1; fi
  drive keys Enter </dev/null >/dev/null || return 1
  wait_for "$STEP_EXPECT" marker >/dev/null || return 2
}

# Set _ret_status (done, missed or failed) and _ret_note for the step just
# run, from its exit code $1 (0, 2 for a missed expectation, else failed)
# and STEP_NOTE, and keep the choose mark (note_choose).
step_outcome() {
  note_choose "$1"
  _ret_status='done'
  _ret_note=$STEP_NOTE
  case $1 in
    0) ;;
    2)
      _ret_status=missed
      _ret_note=${STEP_NOTE:-"expectation not met within ${TURN_SECONDS:-300}s"}
      ;;
    *)
      _ret_status=failed
      _ret_note='a command of the step failed; see round.log'
      ;;
  esac
}
