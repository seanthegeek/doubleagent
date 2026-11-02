# shellcheck shell=sh
# steps.sh: the step kinds that read and answer an agent's dialogs, and the
# step bookkeeping round.sh runs between steps, sourced by round.sh after
# screen.sh on the analyst machine and by the round pipeline's tests.
#
# Calls drive, capture_screen, wait_for, state_get, state_set and round_log,
# which round.sh and vm.sh define and a test replaces with stubs. Reads the
# globals CELL, STEP_NUMBER, STEP_NAME, STEP_KIND, STEP_TEXT, STEP_EXPECT,
# AGENT_OPTION, TURN_SECONDS and ROUND_STOP_AFTER, and sets STEP_NOTE.

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
# _ret_options (the number of options read) and _ret_first (the screen on
# the first option).
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
  STEP_NOTE="answered with the first option, $_ret_options options"
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

# The `keys` kind; after a `choose` that missed or failed it sends nothing,
# since the cursor is not where the table meant the keys to act.
press_step_keys() {
  if [ "$(state_get choose_missed)" = yes ]; then
    STEP_NOTE='missed: no key sent, the choose step before it missed'
    return 2
  fi
  drive keys "$STEP_TEXT" </dev/null >/dev/null || return 1
  wait_for "$STEP_EXPECT" quiet >/dev/null || return 2
}

# Record in the state whether the step just run was a `choose` that did not
# place the cursor (any exit code $1 but 0: a miss or a failed drive call),
# so the `keys` step after it sends no Enter to an option the table did not
# mean; any other step clears the mark.
note_choose() {
  _nc_missed=
  [ "$STEP_KIND" = choose ] && [ "$1" -ne 0 ] && _nc_missed=yes
  [ "$(state_get choose_missed)" = "$_nc_missed" ] || state_set choose_missed "$_nc_missed"
}

# Succeed, logging so, when ROUND_STOP_AFTER names step number $1 (compared
# as numbers); $2 is the step's label.
stop_after_reached() {
  [ -n "${ROUND_STOP_AFTER:-}" ] || return 1
  [ "$(number_value "$1")" -eq "$(number_value "$ROUND_STOP_AFTER")" ] || return 1
  round_log "stopped after step $2 (ROUND_STOP_AFTER); resume goes on"
}
