# shellcheck shell=sh
# steps.sh: the step kinds that read and answer an agent's dialogs, and the
# step bookkeeping round.sh runs between steps, sourced by round.sh after
# screen.sh on the analyst machine and by the round pipeline's tests.
#
# Calls drive, capture_screen, wait_for, state_get, state_set, round_log,
# die, guest_copy, guest_shell, end_session and paste_source, which round.sh
# and vm.sh define and a test replaces with stubs. Reads the globals CELL,
# HERE, GUEST_OS, STEP_NUMBER, STEP_NAME, STEP_KIND, STEP_TEXT, STEP_EXPECT,
# AGENT_MARKER, AGENT_OPTION, AGENT_INPUT_GUARD, TURN_SECONDS,
# TERMINAL_COLS, TERMINAL_ROWS, ROUND_STOP_AFTER and ROUND_ENV, and sets
# STEP_NOTE.

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
# file $1 and print it; returns 2 on an unread screen, 1 on a failed
# press, for the caller to note (it runs in a command substitution).
press_down_saved() {
  press_times Down 1 || return 1
  _pds_screen=$(capture_screen) || return 2
  printf '%s\n' "$_pds_screen" >"$1"
  printf '%s\n' "$_pds_screen"
}

# Print the live part of the screen read from stdin (live_screen) by the
# agent's input marker AGENT_MARKER, the part where an open dialog is drawn.
live_part() {
  live_screen "${AGENT_MARKER:-}"
}

# Print what places the cursor on the live part of the screen read from
# stdin: every line matching the agent's AGENT_OPTION pattern (the
# highlighted option), box-drawing characters stripped, or that whole part
# when no line matches. Two screens with the same key have the cursor on
# the same option, so a line outside the dialog that comes and goes (Gemini
# CLI's true-colour warning, shown at the first screen and gone after a
# key) changes no reading.
option_key() {
  _ok_screen=$(live_part)
  _ok_lines=
  [ -z "${AGENT_OPTION:-}" ] ||
    _ok_lines=$(printf '%s\n' "$_ok_screen" | strip_box | grep -E -- "$AGENT_OPTION")
  if [ -n "$_ok_lines" ]; then printf '%s\n' "$_ok_lines"; else printf '%s\n' "$_ok_screen"; fi
}

# Read every option of the open dialog: press Down, at most $1 times, saving
# each settled screen as screens/NN-name.option-K.txt, until the cursor is
# on the start option again (it wrapped) or where it was before (it
# stopped), by option_key, then return the cursor where it started
# (return_to_start). Sets STEP_NOTE, _ret_end (wrapped, stopped or limit),
# _ret_options (the options read: at the press limit a lower bound, since
# one more may follow), _ret_first (the screen at the start) and
# _ret_returned (yes when the cursor is back where it started).
read_options() {
  _ret_first=$(capture_screen) || {
    screen_unread
    return 1
  }
  _rop_first_key=$(printf '%s\n' "$_ret_first" | option_key)
  _rop_previous=$_rop_first_key
  _rop_moves=0
  _rop_end=limit
  while [ "$_rop_moves" -lt "$1" ]; do
    _rop_moves=$((_rop_moves + 1))
    _rop_screen=$(press_down_saved \
      "$CELL/screens/$STEP_NUMBER-$STEP_NAME.option-$_rop_moves.txt")
    case $? in
      0) ;;
      2)
        screen_unread
        return 1
        ;;
      *) return 1 ;;
    esac
    _rop_key=$(printf '%s\n' "$_rop_screen" | option_key)
    [ "$_rop_key" != "$_rop_first_key" ] || _rop_end=wrapped
    [ "$_rop_key" != "$_rop_previous" ] || _rop_end=stopped
    [ "$_rop_end" = limit ] || break
    _rop_previous=$_rop_key
  done
  # A limit leaves one option unread at least.
  _ret_options=$_rop_moves
  [ "$_rop_end" != limit ] || _ret_options=$((_rop_moves + 1))
  _ret_end=$_rop_end
  STEP_NOTE="$_rop_moves presses of Down, the cursor $_rop_end"
  return_to_start "$_rop_moves"
}

# Press Up one press at a time, at most $1 times, until the cursor is on
# the option it started on (the option_key of _ret_first, the screen where
# the reading started); a count would be wrong for a list that stops at its
# top or a reading begun below the first option. When the cursor never
# gets there, append `cursor not returned` to STEP_NOTE and set the
# choose_missed mark, since it is then on an option nobody chose; sets
# _ret_returned. Returns 1 when a drive call fails or a screen cannot be
# read (screen_unread).
return_to_start() {
  _rts_left=$1
  _rts_first=$(printf '%s\n' "$_ret_first" | option_key)
  _ret_returned=no
  _rts_key=$(cursor_key) || return 1
  while [ "$_rts_key" != "$_rts_first" ] && [ "$_rts_left" -gt 0 ]; do
    press_times Up 1 || return 1
    _rts_left=$((_rts_left - 1))
    _rts_key=$(cursor_key) || return 1
  done
  if [ "$_rts_key" = "$_rts_first" ]; then
    _ret_returned=yes
    return 0
  fi
  STEP_NOTE="$STEP_NOTE; cursor not returned"
  state_set choose_missed yes
}

# Print the option_key of the current screen; on an unread screen, note it
# (screen_unread) and return 1.
cursor_key() {
  _ck_screen=$(capture_screen) || {
    screen_unread
    return 1
  }
  printf '%s\n' "$_ck_screen" | option_key
}

# Return 0 when the grep -E pattern $1 (`-` for the agent's AGENT_OPTION)
# is on the live part of the current screen (live_part), its box-drawing
# characters stripped; 1 when it is not; 2 when the screen could not be
# read.
dialog_on_screen() {
  _dos_pattern=$1
  [ "$_dos_pattern" != - ] || _dos_pattern=${AGENT_OPTION:-}
  [ -n "$_dos_pattern" ] || return 1
  _dos_screen=$(capture_screen) || return 2
  printf '%s\n' "$_dos_screen" | live_part | strip_box | grep -Eq -- "$_dos_pattern"
}

# Fail a step that reads a dialog when the screen could not be read:
# STEP_NOTE says so, the choose_missed mark is set, since the dialog's
# state is unknown, and the step returns 1 through the caller.
screen_unread() {
  STEP_NOTE='the screen could not be read'
  state_set choose_missed yes
}

# The `dialog` kind: when the step's pattern is on the screen, read the
# options and answer with the first; otherwise note that no dialog opened,
# which is a finding and not a miss.
answer_dialog() {
  dialog_on_screen "$STEP_TEXT"
  case $? in
    1)
      STEP_NOTE='no dialog'
      return 0
      ;;
    2)
      screen_unread
      return 1
      ;;
  esac
  read_options "$OPTION_PRESSES" || return 1
  [ "$_ret_returned" = yes ] || {
    STEP_NOTE="$STEP_NOTE; not answered"
    return 2
  }
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
# (no key sent), or when no option matches or the cursor ends on a line the
# pattern does not match; then press Escape once (choose_missed_escape), so
# the dialog is closed rather than left for a later key.
choose_option() {
  dialog_on_screen -
  case $? in
    1)
      STEP_NOTE='missed: no dialog open'
      return 2
      ;;
    2)
      screen_unread
      return 1
      ;;
  esac
  read_options "$OPTION_PRESSES" || return 1
  _co_index=0
  _co_line=$(printf '%s\n' "$_ret_first" | live_part | highlighted_line "${AGENT_OPTION:-}")
  while ! printf '%s\n' "$_co_line" | grep -Eq -- "$STEP_TEXT"; do
    _co_index=$((_co_index + 1))
    if [ "$_co_index" -ge "$_ret_options" ]; then
      choose_missed_escape "missed: no option matches $STEP_TEXT"
      return
    fi
    _co_line=$(live_part <"$CELL/screens/$STEP_NUMBER-$STEP_NAME.option-$_co_index.txt" |
      highlighted_line "${AGENT_OPTION:-}")
  done
  press_times Down "$_co_index" || return 1
  _co_line=$(capture_screen | live_part | highlighted_line "${AGENT_OPTION:-}")
  STEP_NOTE="cursor on \"$_co_line\""
  printf '%s\n' "$_co_line" | grep -Eq -- "$STEP_TEXT" && return 0
  choose_missed_escape "missed: the cursor ended on \"$_co_line\", not $STEP_TEXT"
}

# Close the dialog a `choose` could not place the cursor in with one
# Escape, set STEP_NOTE to the reason $1 and what was sent, and return 2;
# return 1 when the drive call fails.
choose_missed_escape() {
  drive keys Escape </dev/null >/dev/null || return 1
  STEP_NOTE="$1; Escape sent after the miss"
  return 2
}

# Succeed unless the choose_missed mark is set (see note_choose); then set
# STEP_NOTE and return 2, for a step about to press Enter, which would
# answer a dialog with an option the table did not mean.
enter_allowed() {
  [ "$(state_get choose_missed)" = yes ] || return 0
  STEP_NOTE="${STEP_NOTE:+$STEP_NOTE; }the choose step before missed"
  return 2
}

# The `keys` kind; keys that include Enter wait for enter_allowed.
press_step_keys() {
  case ,$STEP_TEXT, in
    *Enter,*) enter_allowed || return 2 ;;
  esac
  if ! drive keys "$STEP_TEXT" </dev/null >/dev/null; then
    STEP_NOTE='the keys call failed; its effect on an open dialog is unknown'
    return 1
  fi
  wait_for "$STEP_EXPECT" quiet >/dev/null || return 2
}

# Keep the state's choose_missed mark after the step just run, whose exit
# code is $1. The rule: a failed drive call sent to a dialog that may be
# open sets the mark, since what it did to the dialog is unknown; so does a
# `choose` that did not place the cursor. A `keys` or `secret` step that
# failed (exit code 1) is such a call, and a typing step's recovery Escape
# sets it in input_free itself. A `choose` that placed the cursor, a
# `dialog` that read and answered a dialog, an `agent`, `stop` or `reboot`
# step (a new or closed session), and a recovery Escape that closed the
# dialog clear it; every other step keeps it, an `options` step included,
# whose reading leaves the cursor on an option nobody chose.
note_choose() {
  _nc_mark=$(state_get choose_missed)
  _nc_new=$_nc_mark
  case $STEP_KIND in
    choose) if [ "$1" -eq 0 ]; then _nc_new=; else _nc_new=yes; fi ;;
    dialog) case $STEP_NOTE in answered*) _nc_new= ;; esac ;;
    agent | stop | reboot) _nc_new= ;;
    keys | secret) [ "$1" -ne 1 ] || _nc_new=yes ;;
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

# Return 0 when a dialog is open: the agent's AGENT_OPTION pattern (a
# highlighted option) or AGENT_INPUT_GUARD pattern (an input dialog with no
# option, such as a key prompt) shows on the live part of the screen
# (live_part), box-drawing characters stripped; 1 when none does; 2 when
# the screen could not be read, which is not the same as no dialog.
dialog_open() {
  _do_screen=$(capture_screen) || return 2
  _do_screen=$(printf '%s\n' "$_do_screen" | live_part | strip_box)
  for _do_pattern in "${AGENT_OPTION:-}" "${AGENT_INPUT_GUARD:-}"; do
    [ -n "$_do_pattern" ] || continue
    printf '%s\n' "$_do_screen" | grep -Eq -- "$_do_pattern" && return 0
  done
  return 1
}

# Succeed when the input is free to type into. A dialog the table did not
# foresee (dialog_open) gets one Escape, which cancels a confirmation and
# grants nothing, then a settled screen and a second look, so it costs the
# step a note rather than every step after it its typing; the input then
# free also clears the choose_missed mark, since the dialog whose cursor it
# guarded is closed. Whatever leaves the dialog's state unknown or a dialog
# open (a failed Escape call, an unread screen, a dialog still open, as
# Gemini CLI's key prompt gives way to its method picker) sets the mark, so
# no Enter answers it, and returns 1 (failed) or 2 (missed).
input_free() {
  dialog_open
  case $? in
    1) return 0 ;;
    2) screen_unread; return 1 ;;
  esac
  drive keys Escape </dev/null >/dev/null ||
    { input_unknown 'the Escape call failed; its effect on the open dialog is unknown'; return 1; }
  wait_for quiet quiet >/dev/null || :
  dialog_open
  case $? in
    0)
      STEP_NOTE='a dialog is open after an Escape'
      state_set choose_missed yes
      return 2
      ;;
    2) input_unknown 'the screen could not be read after the Escape'; return 1 ;;
  esac
  STEP_NOTE='a dialog was open; Escape closed it'
  [ "$(state_get choose_missed)" != yes ] || state_set choose_missed ''
}

# Set STEP_NOTE to $1 and the choose_missed mark, for a recovery whose
# outcome on the dialog cannot be known; the caller then returns 1.
input_unknown() {
  STEP_NOTE=$1
  state_set choose_missed yes
}

# Append the missed-expectation note to STEP_NOTE and return 2, for a step
# whose wait ended without its expectation.
expectation_missed() {
  STEP_NOTE="${STEP_NOTE:+$STEP_NOTE; }expectation not met within ${TURN_SECONDS:-300}s"
  return 2
}

# The `prompt` and `type` kinds: when the input is free, type the step's
# text, press the key $1 after it unless $1 is empty (and then only when
# enter_allowed), and wait for the step's expectation, `-` standing for $2.
send_text() {
  input_free || return
  if [ -n "$1" ]; then enter_allowed || return 2; fi
  printf '%s' "$STEP_TEXT" | drive type >/dev/null || return 1
  if [ -n "$1" ]; then drive keys "$1" </dev/null >/dev/null || return 1; fi
  wait_for "$STEP_EXPECT" "$2" >/dev/null || expectation_missed
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
  input_free || return
  _ct_count=$STEP_TEXT
  if [ "$_ct_count" = - ]; then
    _ct_text=$(previous_type_text "$STEP_NUMBER")
    _ct_count=${#_ct_text}
  fi
  press_times Backspace "$_ct_count" || return 1
  STEP_NOTE="${STEP_NOTE:+$STEP_NOTE; }$_ct_count presses of Backspace"
  wait_for "$STEP_EXPECT" quiet >/dev/null || expectation_missed
}

# The `paste` kind: when the input is free and enter_allowed, copy the
# paste file into the VM, send it as one bracketed paste, type the text
# after the file name, press Enter and wait.
send_paste() {
  input_free || return
  enter_allowed || return 2
  _sp_file=${STEP_TEXT%% *}
  _sp_rest=${STEP_TEXT#"$_sp_file"}
  _sp_rest=${_sp_rest# }
  guest_copy "$(paste_source "$_sp_file")" pastes
  drive paste "pastes/$_sp_file" </dev/null >/dev/null || return 1
  if [ -n "$_sp_rest" ]; then printf '%s' "$_sp_rest" | drive type >/dev/null || return 1; fi
  drive keys Enter </dev/null >/dev/null || return 1
  wait_for "$STEP_EXPECT" marker >/dev/null || expectation_missed
}

# Print the credentials file the `secret` kind reads: ROUND_ENV, else
# `.env` beside round.sh (HERE), a file of NAME=value lines the operator
# writes and git ignores.
secrets_file() {
  printf '%s\n' "${ROUND_ENV:-${HERE:-.}/.env}"
}

# Print the value of the variable $1 in secrets_file: the rest of its last
# `NAME=value` line, as written, a trailing carriage return removed;
# nothing when the file or the line is absent.
secret_value() {
  [ -f "$(secrets_file)" ] || return 0
  sed -n "s/^$1=//p" "$(secrets_file)" | tail -n 1 | tr -d '\r'
}

# Print the name of every variable that a `secret` step, or a secret:NAME
# token in an `agent` or `command` step, of the steps file $1 numbered $2
# or above reads and secrets_file leaves without a value, one per line and
# once each, so a run stops before its first step rather than at the step.
missing_secrets() {
  awk -F '	' -v from="$2" '$0 !~ /^#/ && NF > 0 && $1 != "NN" && $1 + 0 >= from + 0 {
    if ($3 == "secret" && !seen[$4]++) print $4
    if ($3 != "agent" && $3 != "command") next
    text = $4
    while (match(text, /secret:[A-Z][A-Z0-9_]*/)) {
      name = substr(text, RSTART + 7, RLENGTH - 7)
      if (!seen[name]++) print name
      text = substr(text, RSTART + RLENGTH)
    }
  }' "$1" |
    while IFS= read -r _ms_name; do
      [ -n "$(secret_value "$_ms_name")" ] || printf '%s\n' "$_ms_name"
    done
}

# The `secret` kind: type the value of the variable STEP_TEXT names in
# secrets_file into the open input dialog, press Enter and wait for the
# step's expectation (`-` is the input marker). The value goes to the
# drive call's stdin here, never to a command line, the log or STEP_NOTE;
# in the guest it is the argument of one short-lived tui-test process,
# since `type` takes its text as an argument, and the agent's dialog may
# echo it (lab/round/README.md, "The step table"). Nothing is typed, and
# the step is missed, unless AGENT_INPUT_GUARD is set and shows on the live
# part of the screen (the dialog that takes the credential), or while the
# choose_missed mark blocks Enter (enter_allowed).
send_secret() {
  if [ -z "${AGENT_INPUT_GUARD:-}" ]; then
    STEP_NOTE='no AGENT_INPUT_GUARD pattern to find the input dialog by'
    return 2
  fi
  dialog_on_screen "$AGENT_INPUT_GUARD"
  case $? in
    1)
      STEP_NOTE='no input dialog open'
      return 2
      ;;
    2)
      screen_unread
      return 1
      ;;
  esac
  enter_allowed || return 2
  _ss_value=$(secret_value "$STEP_TEXT")
  [ -n "$_ss_value" ] || die "$STEP_TEXT has no value in $(secrets_file)"
  printf '%s' "$_ss_value" | drive type >/dev/null || return 1
  drive keys Enter </dev/null >/dev/null || return 1
  STEP_NOTE="typed $STEP_TEXT from $(secrets_file)"
  wait_for "$STEP_EXPECT" marker >/dev/null || expectation_missed
}

# Set _ret_expanded to the text $1 with every secret:NAME token (NAME an
# upper-case variable of secrets_file) replaced by that variable's value,
# verbatim; die when a token's variable has no value or holds a single
# quote, which would end the quotes the table puts around the token and
# let the guest's shell echo the line into a screen file. The text is
# split at each token in sh, never by sed, since a value may hold
# characters a sed pattern or replacement treats as special.
# Temporaries _exs_.
expand_secrets() {
  _exs_rest=$1
  _ret_expanded=
  while :; do
    case $_exs_rest in
      *secret:*) ;;
      *) break ;;
    esac
    _exs_before=${_exs_rest%%secret:*}
    _exs_rest=${_exs_rest#*secret:}
    _exs_name=$(expr "X$_exs_rest" : 'X\([A-Z][A-Z0-9_]*\)')
    if [ -z "$_exs_name" ]; then
      _ret_expanded=${_ret_expanded}${_exs_before}secret:
      continue
    fi
    _exs_value=$(secret_value "$_exs_name")
    [ -n "$_exs_value" ] || die "$_exs_name has no value in $(secrets_file)"
    case $_exs_value in
      *\'*) die "the value of $_exs_name in $(secrets_file) holds a single quote" ;;
    esac
    _ret_expanded=$_ret_expanded$_exs_before$_exs_value
    _exs_rest=${_exs_rest#"$_exs_name"}
  done
  _ret_expanded=$_ret_expanded$_exs_rest
}

# Split the `agent` step text $1 into _ret_directory, the directory below
# the home an opening `in:<dir>` word names (lab/proj without one), and
# _ret_command, the command line after it. On a guest other than Windows a
# line opening with a NAME=value word gets `env ` in front, since drive.sh
# runs the line's first word as the program; a Windows table writes the
# PowerShell form itself. Reads GUEST_OS; temporaries _acl_.
agent_command_line() {
  _ret_directory=lab/proj
  _ret_command=$1
  case $_ret_command in
    in:*)
      _ret_directory=${_ret_command%% *}
      _ret_directory=${_ret_directory#in:}
      _ret_command=${_ret_command#* }
      _ret_command=${_ret_command#"${_ret_command%%[! ]*}"}
      ;;
  esac
  [ "${GUEST_OS:-}" != windows ] || return 0
  _acl_word=${_ret_command%% *}
  case $_acl_word in
    *=*) ;;
    *) return 0 ;;
  esac
  case ${_acl_word%%=*} in
    '' | [!A-Za-z_]* | *[!A-Za-z0-9_]*) return 0 ;;
  esac
  _ret_command="env $_ret_command"
}

# The `agent` kind: stop the open session and start the step text $1
# (agent_command_line) as session s<STEP_NUMBER>. The log keeps the line
# as written, secret:NAME tokens included; only the drive call's stdin
# gets their values (expand_secrets), as a `secret` step's value travels.
start_agent() {
  agent_command_line "$1"
  expand_secrets "$_ret_command"
  end_session
  state_set session "s$STEP_NUMBER"
  round_log "start session s$STEP_NUMBER in ~/$_ret_directory: $_ret_command"
  printf '%s\n' "$_ret_expanded" |
    drive start "${TERMINAL_COLS:-120}" "${TERMINAL_ROWS:-40}" "$_ret_directory"
}

# The `command` kind: run STEP_TEXT in the guest's shell, its output saved
# as screens/<step>.command.txt. The log keeps the text as written; only
# the shell's stdin gets the secret:NAME values (expand_secrets).
run_command() {
  expand_secrets "$STEP_TEXT"
  round_log "command: $STEP_TEXT"
  printf '%s\n' "$_ret_expanded" |
    guest_shell >"$CELL/screens/$STEP_NUMBER-$STEP_NAME.command.txt" 2>&1
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
      _ret_note="${STEP_NOTE:+$STEP_NOTE; }a command of the step failed; see round.log"
      ;;
  esac
}
