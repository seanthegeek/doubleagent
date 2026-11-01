#!/bin/sh
# drive.sh: the guest half of a round's terminal driver on Linux and macOS.
# It wraps microsoft/tui-test, whose daemon keeps a session alive between
# calls, so round.sh sends one verb per ssh call and reads the screen in
# between. Text always arrives on stdin, never as an argument.
#
#   sh drive.sh SESSION start COLS ROWS DIR   < the command line; DIR below the home
#   sh drive.sh SESSION keys KEY[,KEY...]     Enter, Down, Shift+Tab, Ctrl+C, ...
#   sh drive.sh SESSION type                  < text, typed without Enter
#   sh drive.sh SESSION paste FILE            FILE (below ~/lab) as one bracketed paste
#   sh drive.sh SESSION screen                the rendered screen
#   sh drive.sh SESSION wait-idle QUIET MAX   < marker regex, busy regex
#   sh drive.sh SESSION wait-exit MAX
#   sh drive.sh SESSION log                   the asciicast of the session
#   sh drive.sh SESSION stop
#   sh drive.sh warmup selftest               start and close a throwaway session
#
# wait-idle succeeds once the screen has not changed for QUIET seconds while
# the marker matches and the busy pattern does not (either may be empty);
# it gives up after MAX seconds with exit 1.

set -u
tui=$HOME/lab/tui-test/tui-test
# tui-test keeps a session's asciicast only when its config names a
# recording directory and turns tracing on; relative directories resolve
# beside the file, so both stay under ~/lab, outside every survey root.
config=$HOME/lab/tui-test/tui-test.toml
session=${1:?session}
verb=${2:?verb}
shift 2
[ -x "$tui" ] || {
  printf 'drive: %s is missing\n' "$tui" >&2
  exit 2
}

terminal() {
  "$tui" --session "$session" "$@"
}

# Start the command line read from stdin in a terminal of $1 columns and $2
# rows in the directory $3, inside a login shell that is also interactive,
# so PATH and the startup files are those of a person's terminal.
start_session() {
  IFS= read -r _ss_line || [ -n "${_ss_line:-}" ] || {
    printf 'drive: no command line on stdin\n' >&2
    exit 2
  }
  _ss_cols=$1
  _ss_rows=$2
  _ss_directory=$3
  case $(uname -s) in
    Darwin) _ss_shell=zsh ;;
    *) _ss_shell=bash ;;
  esac
  # The command line comes from the round's own steps file, quoted as sh.
  eval "set -- $_ss_line"
  # shellcheck disable=SC2016 # $0 and $@ belong to the login shell
  terminal run --config "$config" --cols "$_ss_cols" --rows "$_ss_rows" \
    --cwd "$_ss_directory" -- "$_ss_shell" -ilc 'exec "$0" "$@"' "$@"
}

press_keys() {
  _pk_saved_ifs=$IFS
  IFS=,
  # shellcheck disable=SC2086 # split the comma-separated key names
  set -- $1
  IFS=$_pk_saved_ifs
  for _pk_key in "$@"; do
    terminal key press "$_pk_key" || exit 1
    sleep 1
  done
}

# One bracketed paste of the file $1 below the lab directory, line feeds
# sent as carriage returns as a terminal sends them.
paste_file() {
  _pf_file=$HOME/lab/$1
  [ -f "$_pf_file" ] || {
    printf 'drive: no paste file %s\n' "$_pf_file" >&2
    exit 2
  }
  _pf_data=$(
    printf '\033[200~'
    tr '\n' '\r' <"$_pf_file"
    printf '\033[201~'
  )
  terminal write "$_pf_data"
}

wait_idle() {
  _wi_quiet=$1
  _wi_limit=$2
  IFS= read -r _wi_marker || _wi_marker=
  IFS= read -r _wi_busy || _wi_busy=
  _wi_seconds=0
  _wi_still=0
  _wi_last=
  while [ "$_wi_seconds" -lt "$_wi_limit" ]; do
    _wi_screen=$(terminal text 2>/dev/null) || _wi_screen=
    _wi_sum=$(printf '%s' "$_wi_screen" | cksum)
    if [ "$_wi_sum" = "$_wi_last" ]; then
      _wi_still=$((_wi_still + 1))
    else
      _wi_still=0
      _wi_last=$_wi_sum
    fi
    _wi_ready=yes
    if [ -n "$_wi_marker" ] && ! printf '%s\n' "$_wi_screen" | grep -Eq -- "$_wi_marker"; then
      _wi_ready=no
    fi
    if [ -n "$_wi_busy" ] && printf '%s\n' "$_wi_screen" | grep -Eq -- "$_wi_busy"; then
      _wi_ready=no
    fi
    if [ "$_wi_ready" = yes ] && [ "$_wi_still" -ge "$_wi_quiet" ]; then
      exit 0
    fi
    _wi_seconds=$((_wi_seconds + 1))
    sleep 1
  done
  exit 1
}

# Ctrl+C twice (most agents ask for a second one), then TERM, then kill,
# each followed by a bounded wait for the program to exit; then close.
stop_session() {
  for _st_step in key-Ctrl+C key-Ctrl+C signal-TERM kill; do
    case $_st_step in
      key-*) terminal key press "${_st_step#key-}" >/dev/null 2>&1 ;;
      signal-*) terminal signal "${_st_step#signal-}" >/dev/null 2>&1 ;;
      kill)
        terminal kill >/dev/null 2>&1
        break
        ;;
    esac
    if terminal wait exit --timeout 5000 >/dev/null 2>&1; then
      break
    fi
  done
  terminal close >/dev/null 2>&1 || true
}

# Print the version, write the config, then start and close a throwaway
# session, so the daemon's own files exist before the baseline listing.
selftest() {
  "$tui" --version
  printf '[recording]\ndirectory = "recordings"\n\n[trace]\nmode = "on"\ndirectory = "traces"\n' \
    >"$config"
  terminal run --config "$config" --cols 80 --rows 24 -- sh -c 'printf "drive ready\n"; sleep 30'
  terminal expect text 'drive ready' --timeout 10000 >/dev/null
  _sf_code=$?
  terminal close >/dev/null 2>&1 || true
  exit "$_sf_code"
}

case $verb in
  start) start_session "${1:?cols}" "${2:?rows}" "$HOME/${3:?directory below the home}" ;;
  keys) press_keys "${1:?keys}" ;;
  type) terminal type "$(cat)" ;;
  paste) paste_file "${1:?file}" ;;
  screen) terminal text ;;
  wait-idle) wait_idle "${1:?quiet seconds}" "${2:?maximum seconds}" ;;
  wait-exit) terminal wait exit --timeout "$((${1:?maximum seconds} * 1000))" ;;
  log) terminal get-recording --config "$config" ;;
  stop) stop_session ;;
  selftest) selftest ;;
  *)
    printf 'drive: unknown verb %s\n' "$verb" >&2
    exit 2
    ;;
esac
