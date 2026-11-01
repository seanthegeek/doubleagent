#!/bin/sh
# round.sh: run one real-install round cell, one agent on one lab VM, from
# the analyst machine. lab/round/README.md describes the cell directory, the
# step kinds and the pause protocol.
#
#   lab/round/round.sh AGENT VM CELL start    revert, start, push, baseline,
#                                             install, then the steps
#   lab/round/round.sh AGENT VM CELL resume   finish a paused step, go on
#   lab/round/round.sh AGENT VM CELL collect  collect, analyze, report
#   lab/round/round.sh AGENT VM CELL finish [--force]
#                                             last listing, revert, clean up
#   lab/round/round.sh AGENT VM CELL status
#
# AGENT names lab/round/agents/AGENT, VM a prlctl VM with lab/round/vms/VM.conf,
# and CELL a directory outside the repository that receives the evidence.
# Exit 3 means a pause step is waiting for the person.

# The VM and agent settings come from the .conf files sourced below.
# shellcheck disable=SC2154
set -u
HERE=$(cd "$(dirname "$0")" && pwd)
REPO=$(cd "$HERE/../.." && pwd)
TAB=$(printf '\t')
TUI_TEST_VERSION=0.1.1
AGENT=${1:?usage: round.sh AGENT VM CELL [start|resume|collect|finish [--force]|status]}
VM_NAME=${2:?VM name}
CELL_ARGUMENT=${3:?cell directory}
ACTION=${4:-start}
FORCE=${5:-}
HOST_PYTHON=${ROUND_PYTHON:-python3}

# shellcheck source=lab/round/lib/vm.sh
. "$HERE/lib/vm.sh"

AGENT_DIR=$HERE/agents/$AGENT
[ -d "$AGENT_DIR" ] || die "no agent definition $AGENT_DIR"
[ -f "$HERE/vms/$VM_NAME.conf" ] || die "no VM definition $HERE/vms/$VM_NAME.conf"
# shellcheck source=/dev/null
. "$HERE/vms/$VM_NAME.conf"
# shellcheck source=/dev/null
. "$AGENT_DIR/agent.conf"
mkdir -p "$CELL_ARGUMENT" || die "cannot create $CELL_ARGUMENT"
CELL=$(cd "$CELL_ARGUMENT" && pwd)
case "$CELL/" in
  "$REPO/"*) die "the cell directory must be outside the repository: $CELL" ;;
esac

utc_now() { date -u +%Y-%m-%dT%H:%M:%SZ; }

# The cell's state is a list of KEY=VALUE lines; the last line of a key wins.
state_get() {
  [ -f "$CELL/state" ] || return 0
  sed -n "s/^$1=//p" "$CELL/state" | tail -n 1
}

state_set() {
  printf '%s=%s\n' "$1" "$2" >>"$CELL/state"
}

# Run one of the standard-library Python tools beside this script.
host_tool() {
  _ht_script=$1
  shift
  python3 -I "$HERE/$_ht_script" "$@"
}

# Print a step number without its leading zeros, so `08` compares as 8.
number_value() {
  printf '%s\n' "$1" | sed 's/^0*\([0-9]\)/\1/'
}

# The step numbers of the cell's table, in order. The state names steps by
# number, never by position, so a repeat step inserted into the cell's copy
# (see the README) shifts nothing that has run.
step_numbers() {
  awk -F "$TAB" '$0 !~ /^#/ && NF > 0 && $1 != "NN" { print $1 }' "$CELL/steps.tsv"
}

# Print the number of the step after step $1, or `done`.
next_step_after() {
  awk -F "$TAB" -v after="$1" '$0 !~ /^#/ && NF > 0 && $1 != "NN" && $1 + 0 > after + 0 {
    print $1; found = 1; exit } END { if (!found) print "done" }' "$CELL/steps.tsv"
}

# Load step $1 into the globals STEP_NUMBER, STEP_NAME, STEP_KIND, STEP_TEXT
# and STEP_EXPECT; the claim is for the reader.
load_step() {
  _ls_line=$(awk -F "$TAB" -v want="$1" '$0 !~ /^#/ && NF > 0 && $1 != "NN" &&
    $1 + 0 == want + 0 { print; exit }' "$CELL/steps.tsv")
  [ -n "$_ls_line" ] || die "no step $1 in $CELL/steps.tsv"
  IFS=$TAB read -r STEP_NUMBER STEP_NAME STEP_KIND STEP_TEXT STEP_EXPECT _ <<EOF_STEP_LINE
$_ls_line
EOF_STEP_LINE
}

drive() {
  guest_tool drive "$(state_get session)" "$@"
}

capture_screen() {
  [ -n "$(state_get session)" ] || return 0
  drive screen </dev/null 2>/dev/null | tr -d '\r'
}

# Take the listing $1 and diff it against the previous one. The survey's
# own output is kept until its status is known, then stripped of CRs.
take_listing() {
  _tl_file=$CELL/listings/$1.txt
  round_log "listing $1"
  guest_tool survey <"$CELL/roots" >"$_tl_file.raw" || die "listing $1 failed"
  tr -d '\r' <"$_tl_file.raw" >"$_tl_file" && rm -f "$_tl_file.raw"
  _tl_previous=$(state_get listing)
  if [ -n "$_tl_previous" ]; then
    host_tool classify.py --redact "$CELL/listings/$_tl_previous.txt" "$_tl_file" \
      >"$CELL/diffs/$1.txt"
  fi
  state_set listing "$1"
}

# Wait for a step's expectation: none, exit (the program ends), quiet (the
# screen settles), marker or `-` (the agent's input marker), or a regex.
wait_for() {
  _wf_expect=$1
  [ "$_wf_expect" = - ] && _wf_expect=$2
  _wf_limit=${TURN_SECONDS:-300}
  case $_wf_expect in
    none) return 0 ;;
    exit) SSH_SECONDS=$((_wf_limit + 60)) drive wait-exit "$_wf_limit" </dev/null ;;
    quiet)
      printf '\n%s\n' "${AGENT_BUSY:-}" |
        SSH_SECONDS=$((_wf_limit + 60)) drive wait-idle "${QUIET_SECONDS:-3}" "$_wf_limit"
      ;;
    marker)
      printf '%s\n%s\n' "$AGENT_MARKER" "${AGENT_BUSY:-}" |
        SSH_SECONDS=$((_wf_limit + 60)) drive wait-idle "${QUIET_SECONDS:-3}" "$_wf_limit"
      ;;
    *)
      printf '%s\n%s\n' "$_wf_expect" "${AGENT_BUSY:-}" |
        SSH_SECONDS=$((_wf_limit + 60)) drive wait-idle "${QUIET_SECONDS:-3}" "$_wf_limit"
      ;;
  esac
}

# Save the session's recording, stop it and forget it.
end_session() {
  _es_session=$(state_get session)
  [ -n "$_es_session" ] || return 0
  drive log </dev/null >"$CELL/casts/$_es_session.cast" 2>/dev/null || true
  round_log "stop session $_es_session"
  drive stop </dev/null >/dev/null 2>&1 || true
  state_set session ''
}

paste_source() {
  if [ -f "$AGENT_DIR/pastes/$1" ]; then
    printf '%s\n' "$AGENT_DIR/pastes/$1"
  elif [ -f "$HERE/pastes/$1" ]; then
    printf '%s\n' "$HERE/pastes/$1"
  else
    die "no paste file $1"
  fi
}

# Append a steps.log line: label, kind, status, started, finished, note.
# The start time also marks which collections came after the last step.
log_step() {
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$@" >>"$CELL/steps.log"
  state_set last_step_started "$4"
}

# Write the screen before ($2) and after ($3) the step $1; the after part
# is left out while a pause waits for the person.
write_screens() {
  {
    printf '%s\n%s\n' "--- before $1" "$2"
    if [ "$#" -gt 2 ]; then printf '%s\n%s\n' "--- after $1" "$3"; fi
  } >"$CELL/screens/$1.txt"
}

# Start the agent with the step's command line; a first word `in:<dir>`
# names the working directory below the home (`in:.` for the home).
start_agent() {
  end_session
  _sa_directory=lab/proj
  _sa_line=$1
  case $_sa_line in
    in:*)
      _sa_directory=${_sa_line%% *}
      _sa_directory=${_sa_directory#in:}
      _sa_line=${_sa_line#* }
      ;;
  esac
  state_set session "s$STEP_NUMBER"
  round_log "start session s$STEP_NUMBER in ~/$_sa_directory: $_sa_line"
  printf '%s\n' "$_sa_line" |
    drive start "${TERMINAL_COLS:-120}" "${TERMINAL_ROWS:-40}" "$_sa_directory"
}

# Run the kind-specific part of the loaded step; return 1 when a command
# failed and 2 when the step's expectation was not met in time.
run_kind() {
  case $STEP_KIND in
    prompt)
      printf '%s' "$STEP_TEXT" | drive type >/dev/null || return 1
      drive keys Enter </dev/null >/dev/null || return 1
      wait_for "$STEP_EXPECT" marker >/dev/null || return 2
      ;;
    type)
      printf '%s' "$STEP_TEXT" | drive type >/dev/null || return 1
      wait_for "$STEP_EXPECT" quiet >/dev/null || return 2
      ;;
    keys)
      drive keys "$STEP_TEXT" </dev/null >/dev/null || return 1
      wait_for "$STEP_EXPECT" quiet >/dev/null || return 2
      ;;
    paste)
      _rk_file=${STEP_TEXT%% *}
      _rk_rest=${STEP_TEXT#"$_rk_file"}
      _rk_rest=${_rk_rest# }
      guest_copy "$(paste_source "$_rk_file")" pastes
      drive paste "pastes/$_rk_file" </dev/null >/dev/null || return 1
      if [ -n "$_rk_rest" ]; then printf '%s' "$_rk_rest" | drive type >/dev/null || return 1; fi
      drive keys Enter </dev/null >/dev/null || return 1
      wait_for "$STEP_EXPECT" marker >/dev/null || return 2
      ;;
    command)
      round_log "command: $STEP_TEXT"
      printf '%s\n' "$STEP_TEXT" |
        guest_shell >"$CELL/screens/$STEP_NUMBER-$STEP_NAME.command.txt" 2>&1 || return 1
      ;;
    agent)
      start_agent "$STEP_TEXT" >/dev/null || return 1
      wait_for "$STEP_EXPECT" marker >/dev/null || return 2
      ;;
    stop) end_session ;;
    idle) sleep "$STEP_TEXT" ;;
    reboot)
      end_session
      vm_reboot 30
      ;;
    collect) collect_into "$STEP_NUMBER-$STEP_NAME" ;;
    *) die "unknown step kind $STEP_KIND" ;;
  esac
}

# Run step number $1: screens, the step, the listing and its diff, and its
# line in steps.log. A pause step snapshots the VM and exits 3.
run_step() {
  load_step "$1"
  _rs_label=$STEP_NUMBER-$STEP_NAME
  state_set next "$STEP_NUMBER"
  _rs_started=$(utc_now)
  _rs_before=$(capture_screen)
  if [ "$STEP_KIND" = pause ]; then
    write_screens "$_rs_label" "$_rs_before"
    printf '%s\n' "$_rs_before"
    printf '\nround: step %s needs you:\n  %s\n' "$_rs_label" \
      "$(printf '%s' "$STEP_TEXT" | sed "s/{session}/$(state_get session)/g")"
    vm_snapshot "claude-$AGENT-$STEP_NUMBER"
    state_set paused "$STEP_NUMBER"
    state_set paused_label "$_rs_label"
    state_set paused_at "$_rs_started"
    printf 'round: then run: %s %s %s %s resume\n' "$0" "$AGENT" "$VM_NAME" "$CELL"
    exit 3
  fi
  round_log "step $_rs_label ($STEP_KIND)"
  _rs_status=done
  _rs_note=
  run_kind
  case $? in
    0) ;;
    2)
      _rs_status=missed
      _rs_note="expectation not met within ${TURN_SECONDS:-300}s"
      ;;
    *)
      _rs_status=failed
      _rs_note='a command of the step failed; see round.log'
      ;;
  esac
  write_screens "$_rs_label" "$_rs_before" "$(capture_screen)"
  take_listing "$_rs_label"
  log_step "$_rs_label" "$STEP_KIND" "$_rs_status" "$_rs_started" "$(utc_now)" "$_rs_note"
  round_log "step $_rs_label $_rs_status"
  state_set next "$(next_step_after "$STEP_NUMBER")"
}

# Run every step from number $1 on; ROUND_STOP_AFTER, compared as a number,
# ends the run after that step.
run_steps() {
  _rn_from=$(number_value "$1")
  _rn_stop=$(number_value "${ROUND_STOP_AFTER:-}")
  # shellcheck disable=SC2046 # step numbers are digits only
  for _rn_number in $(step_numbers); do
    [ "$(number_value "$_rn_number")" -ge "$_rn_from" ] || continue
    run_step "$_rn_number"
    if [ -n "$_rn_stop" ] && [ "$(number_value "$_rn_number")" -eq "$_rn_stop" ]; then
      round_log "stopped after step $_rs_label (ROUND_STOP_AFTER); resume goes on"
      return 0
    fi
  done
  state_set next done
  round_log "all steps done; run collect, then finish"
}

# Download the pinned tui-test release asset once per cell and check it
# against the VM file's sha256.
fetch_tui_test() {
  _ft_file=$CELL/tools/$TUI_TEST_ASSET
  if [ ! -f "$_ft_file" ]; then
    round_log "download tui-test $TUI_TEST_VERSION $TUI_TEST_ASSET"
    bounded 300 curl -fsSL -o "$_ft_file" \
      "https://github.com/microsoft/tui-test/releases/download/$TUI_TEST_VERSION/$TUI_TEST_ASSET" ||
      die "tui-test download failed"
  fi
  _ft_sum=$(shasum -a 256 "$_ft_file" | awk '{ print $1 }')
  state_set tui_test_sha256 "$_ft_sum"
  [ "$_ft_sum" = "$TUI_TEST_SHA256" ] || die "tui-test sha256 $_ft_sum is not $TUI_TEST_SHA256"
}

# Copy the guest scripts, the collector and tui-test into the VM. The shape
# tool stays on the analyst machine, which runs it over the pulled archive.
push_tools() {
  case $GUEST_OS in
    windows)
      _pt_lab=$(guest_lab_native)
      printf 'New-Item -ItemType Directory -Force -Path "%s", "%s" | Out-Null\n' \
        "$_pt_lab\\pastes" "$_pt_lab\\tui-test" | guest_shell >/dev/null ||
        die "mkdir in the guest failed"
      guest_copy "$HERE/lib/survey.ps1" "$HERE/lib/drive.ps1" "$HERE/lib/make-project.ps1" \
        "$REPO/collectors/Collect-AgentArtifacts.ps1" .
      guest_copy "$CELL/tools/$TUI_TEST_ASSET" tui-test
      printf 'Expand-Archive -Force -Path "%s" -DestinationPath "%s"\n' \
        "$_pt_lab\\tui-test\\$TUI_TEST_ASSET" "$_pt_lab\\tui-test" | guest_shell >/dev/null ||
        die "tui-test unpack failed"
      ;;
    *)
      guest_ssh 'mkdir -p lab/pastes lab/tui-test' </dev/null || die "mkdir in the guest failed"
      guest_copy "$HERE/lib/survey.sh" "$HERE/lib/drive.sh" "$HERE/lib/make-project.sh" \
        "$REPO/collectors/collect-agent-artifacts.sh" .
      guest_copy "$CELL/tools/$TUI_TEST_ASSET" tui-test
      guest_ssh "cd lab/tui-test && tar xzf $TUI_TEST_ASSET && chmod 755 tui-test" </dev/null ||
        die "tui-test unpack failed"
      ;;
  esac
}

# Run the agent's install script in the guest and record the version line
# it prints (`version: <text>`).
install_agent() {
  case $GUEST_OS in
    windows)
      [ -f "$AGENT_DIR/install.ps1" ] || die "no install.ps1 for $AGENT"
      guest_copy "$AGENT_DIR/install.ps1" .
      round_log "install.ps1"
      guest_ssh powershell -NoProfile -ExecutionPolicy Bypass \
        -File "$(guest_lab_native)\\install.ps1" </dev/null >"$CELL/install.log" 2>&1
      ;;
    *)
      guest_copy "$AGENT_DIR/install.sh" .
      round_log "install.sh"
      # A login shell first, so the installer sees the PATH a terminal has.
      case $GUEST_OS in
        macos) guest_ssh 'exec zsh -lc "sh lab/install.sh"' ;;
        *) guest_ssh 'exec bash -lc "sh lab/install.sh"' ;;
      esac </dev/null >"$CELL/install.log" 2>&1
      ;;
  esac
  _ia_code=$?
  tr -d '\r' <"$CELL/install.log" >"$CELL/install.log.new" &&
    mv "$CELL/install.log.new" "$CELL/install.log"
  [ "$_ia_code" -eq 0 ] || die "install failed with exit $_ia_code; see $CELL/install.log"
  state_set version "$(sed -n 's/^version: //p' "$CELL/install.log" | tail -n 1)"
  state_set install_source "$(sed -n 's/^source: //p' "$CELL/install.log" | tail -n 1)"
}

# Print the names of the collector's files in the guest's lab/out.
guest_out_names() {
  case $GUEST_OS in
    windows)
      _go_raw=$(printf 'Get-ChildItem -Name -File "%s"\n' "$(guest_lab_native)\\out" |
        guest_shell) || die "listing the collector's files failed"
      printf '%s\n' "$_go_raw" | tr -d '\r'
      ;;
    *) guest_ssh 'ls lab/out' </dev/null || die "listing the collector's files failed" ;;
  esac
}

# Run the collector in the guest as the lab user, pull its files into
# collection/$1, delete the guest's copy, then run the analyzer and the
# shape tool on the analyst machine and rewrite the report.
collect_into() {
  _ci_destination=$CELL/collection/$1
  mkdir -p "$_ci_destination" "$CELL/analysis" "$CELL/shapes"
  _ci_started=$(utc_now)
  round_log "collector run $1"
  case $GUEST_OS in
    windows)
      guest_ssh powershell -NoProfile -ExecutionPolicy Bypass \
        -File "$(guest_lab_native)\\Collect-AgentArtifacts.ps1" \
        -OutputDir "$(guest_lab_native)\\out" -Quiet </dev/null >"$_ci_destination/stdout.txt" 2>&1
      ;;
    *)
      # shellcheck disable=SC2016 # $HOME is the guest's
      guest_ssh 'sh lab/collect-agent-artifacts.sh -o "$HOME/lab/out" -q' \
        </dev/null >"$_ci_destination/stdout.txt" 2>&1
      ;;
  esac || die "the collector failed; see $_ci_destination/stdout.txt"
  _ci_names=$(guest_out_names) || exit 1
  # shellcheck disable=SC2086 # one collector file name per word, checked below
  for _ci_name in $_ci_names; do
    check_word "$_ci_name"
    bounded 900 scp -q -F "$CELL/sshconfig" -o BatchMode=yes "guest:$(guest_lab)/out/$_ci_name" \
      "$_ci_destination/" </dev/null || die "pulling $_ci_name failed"
  done
  case $GUEST_OS in
    windows)
      printf 'Remove-Item -Recurse -Force "%s"\n' "$(guest_lab_native)\\out" |
        guest_shell >/dev/null
      ;;
    *)
      # shellcheck disable=SC2016 # $HOME is the guest's
      guest_ssh 'rm -rf "$HOME/lab/out"' </dev/null
      ;;
  esac
  analyze_collection "$1"
  state_set collected_at "$_ci_started"
  host_tool report.py "$CELL" >/dev/null
}

analyze_collection() {
  _ac_destination=$CELL/collection/$1
  (cd "$_ac_destination" && shasum -a 256 -c ./*.sha256 >sha256-check.txt 2>&1) ||
    round_log "sha256 check of $1 failed; see sha256-check.txt"
  _ac_archive=
  for _ac_file in "$_ac_destination"/*.tar.gz "$_ac_destination"/*.zip; do
    [ -f "$_ac_file" ] && _ac_archive=$_ac_file && break
  done
  [ -n "$_ac_archive" ] || die "no archive in $_ac_destination"
  round_log "analyzer timeline over $(basename "$_ac_archive")"
  PYTHONPATH="$REPO/analyzer" "$HOST_PYTHON" -m doubleagent timeline "$_ac_archive" \
    -o "$CELL/analysis/$1" --include-thinking >"$CELL/analysis/$1.stdout.txt" 2>&1 ||
    round_log "analyzer exit $? for $1; see analysis/$1.stdout.txt"
  mkdir -p "$_ac_destination/extract"
  case $_ac_archive in
    *.zip) unzip -q -o "$_ac_archive" -d "$_ac_destination/extract" ;;
    *) tar xzf "$_ac_archive" -C "$_ac_destination/extract" ;;
  esac
  _ac_tree=$(find "$_ac_destination/extract" -type d -name fs -prune | head -n 1)
  [ -n "$_ac_tree" ] || _ac_tree=$_ac_destination/extract
  python3 -I "$REPO/analyzer/tools/shape.py" --enums "$_ac_tree" >"$CELL/shapes/$1.txt" 2>&1 ||
    round_log "shape tool exit $? for $1"
}

# The name of the last listing: one past the table's last step number, at
# the table's own width.
next_listing_number() {
  awk -F "$TAB" '$0 !~ /^#/ && NF > 0 && $1 != "NN" { last = $1 + 0; width = length($1) }
    END { printf "%0" width "d\n", last + 1 }' "$CELL/steps.tsv"
}

# Refuse to start a VM while another one runs: the lab host has room for one.
refuse_other_vms() {
  _ro_others=$(vm_others_running)
  [ -z "$_ro_others" ] || die "another VM is running: $_ro_others"
}

start_cell() {
  [ ! -f "$CELL/state" ] || die "cell $CELL is already started; use resume, collect or finish"
  host_tool steps.py check "$AGENT_DIR" || die "the steps file has errors"
  refuse_other_vms
  mkdir -p "$CELL/listings" "$CELL/diffs" "$CELL/screens" "$CELL/casts" "$CELL/tools"
  # The cell keeps its own copy of the step table and the roots, so an edit
  # in the repository never shifts a paused cell.
  cp "$AGENT_DIR/steps.tsv" "$AGENT_DIR/roots" "$CELL/"
  state_set agent "$AGENT"
  state_set vm "$VM_NAME"
  state_set started "$(utc_now)"
  fetch_tui_test
  vm_revert_clean
  vm_start
  vm_wait_ssh 60 || die "no ssh answer from $VM_NAME"
  push_tools
  guest_tool drive warmup selftest </dev/null >"$CELL/tools/selftest.txt" 2>&1 ||
    die "tui-test selftest failed; see $CELL/tools/selftest.txt"
  state_set tui_test "$(head -n 1 "$CELL/tools/selftest.txt" | tr -d '\r')"
  guest_tool make-project </dev/null >"$CELL/tools/make-project.txt" 2>&1 ||
    die "make-project failed"
  take_listing 00-baseline
  install_agent
  take_listing 01-install
  run_steps "$(step_numbers | head -n 1)"
}

resume_cell() {
  [ -f "$CELL/state" ] || die "cell $CELL is not started"
  host_tool steps.py check --steps "$CELL/steps.tsv" "$AGENT_DIR" || die "the steps file has errors"
  [ "$(vm_status)" = running ] || refuse_other_vms
  vm_start
  vm_wait_ssh 60 || die "no ssh answer from $VM_NAME"
  _rsm_paused=$(state_get paused)
  if [ -n "$_rsm_paused" ]; then
    load_step "$_rsm_paused"
    printf '%s\n%s\n' "--- after $STEP_NUMBER-$STEP_NAME" "$(capture_screen)" \
      >>"$CELL/screens/$STEP_NUMBER-$STEP_NAME.txt"
    take_listing "$STEP_NUMBER-$STEP_NAME"
    log_step "$STEP_NUMBER-$STEP_NAME" pause done "$(state_get paused_at)" "$(utc_now)" \
      'person step'
    state_set paused ''
    state_set paused_label ''
    state_set next "$(next_step_after "$STEP_NUMBER")"
  fi
  _rsm_next=$(state_get next)
  [ "$_rsm_next" != done ] || {
    round_log "every step has run; run collect, then finish"
    return 0
  }
  run_steps "${_rsm_next:-$(step_numbers | head -n 1)}"
}

# Say why finish must not revert the VM yet, or nothing when it may: every
# step has run, none waits for the person, and a collection was taken after
# the last step began.
finish_blocker() {
  if [ -n "$(state_get paused)" ]; then
    printf 'step %s waits for the person\n' "$(state_get paused_label)"
  elif [ "$(state_get next)" != done ]; then
    printf 'steps from %s on have not run\n' "$(state_get next)"
  elif [ -z "$(state_get collected_at)" ]; then
    printf 'no collection was pulled\n'
  elif expr "$(state_get collected_at)" \< "$(state_get last_step_started)" >/dev/null; then
    printf 'the last collection is older than the last step\n'
  fi
}

finish_cell() {
  [ -f "$CELL/state" ] || die "cell $CELL is not started"
  _fc_blocker=$(finish_blocker)
  if [ -n "$_fc_blocker" ]; then
    [ "$FORCE" = --force ] ||
      die "not finishing: $_fc_blocker; run collect first, or finish --force to discard"
    round_log "finish --force although $_fc_blocker"
  fi
  if [ "$(vm_status)" = running ] && vm_wait_ssh 6; then
    end_session
    take_listing "$(next_listing_number)-finish"
  fi
  vm_stop
  vm_revert_clean
  vm_delete_own_snapshots
  vm_stop
  state_set finished "$(utc_now)"
  host_tool report.py "$CELL" >/dev/null
  printf 'round: %s reverted to its clean snapshot and stopped.\n' "$VM_NAME"
  awk -F "$TAB" '$3 != "done" { printf "round: step %s ended %s %s\n", $1, $3, $6 }' \
    "$CELL/steps.log" 2>/dev/null
  if [ -n "$(state_get paused)" ]; then
    load_step "$(state_get paused)"
    printf 'round: the person step %s-%s was never completed: %s\n' \
      "$STEP_NUMBER" "$STEP_NAME" "$STEP_TEXT"
  fi
  printf 'round: report %s/report.md\n' "$CELL"
  printf 'round: %s holds what the agent showed and the credentials it wrote;%s\n' \
    "$CELL" ' delete it once the round is written up.'
}

case $ACTION in
  start) start_cell ;;
  resume) resume_cell ;;
  collect)
    [ -f "$CELL/state" ] || die "cell $CELL is not started"
    vm_wait_ssh 6 || die "no ssh answer from $VM_NAME"
    collect_into "collect-$(date -u +%Y%m%dT%H%M%SZ)"
    printf 'round: report %s/report.md\n' "$CELL"
    ;;
  finish) finish_cell ;;
  status)
    cat "$CELL/state" 2>/dev/null
    cat "$CELL/steps.log" 2>/dev/null
    ;;
  *) die "unknown action $ACTION" ;;
esac
