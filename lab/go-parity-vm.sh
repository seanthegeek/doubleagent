#!/bin/sh
# go-parity-vm.sh: run the Go collector's parity tests on the Windows lab VM,
# from the analyst machine. Cross-compiles the doubleagent binary and the
# parity test binary (plus any extra package test binaries) for Windows on
# ARM, reverts the VM to its clean snapshot, ships the files and the
# collectors, runs the tests inside the guest, pulls the logs back and
# reverts the VM again.
#
#   lab/go-parity-vm.sh [--dry-run] CELL [PACKAGE...]
#
# CELL is a scratch directory outside the repository that receives the
# binaries, parity.log and parity.rc. Each PACKAGE (for example fsmeta or
# fsmeta.test.exe) names a directory under internal/ whose test binary runs
# in the guest the same way and leaves PACKAGE.log and PACKAGE.rc in CELL.
# --dry-run prints every host and guest command and runs none of them: it
# does not touch prlctl, ssh or scp, and it builds nothing.
#
# The tree shipped to the guest is `git archive HEAD collectors`, so
# uncommitted changes to the collectors are not tested.

# The VM settings come from vms/Windows.conf, sourced below.
# shellcheck disable=SC2154
set -u
HERE=$(cd "$(dirname "$0")" && pwd)
REPO=$(cd "$HERE/.." && pwd)

DRY_RUN=0
if [ "${1:-}" = --dry-run ]; then
  DRY_RUN=1
  shift
fi
CELL=${1:?usage: go-parity-vm.sh [--dry-run] CELL [PACKAGE...]}
shift
VM_NAME=Windows

# shellcheck source=lab/round/lib/vm.sh
. "$REPO/lab/round/lib/vm.sh"
# shellcheck source=/dev/null
. "$REPO/lab/round/vms/Windows.conf"

case $CELL in
  /*) ;;
  *) die "the cell directory must be an absolute path: $CELL" ;;
esac
case "$CELL/" in
  "$REPO/"*) die "the cell directory must be outside the repository: $CELL" ;;
esac

# Print a command that --dry-run does not run.
dry() {
  printf 'dry-run: %s\n' "$*"
}

# Run a host command, or only print it under --dry-run.
host_run() {
  if [ "$DRY_RUN" -eq 1 ]; then
    dry "$*"
  else
    "$@"
  fi
}

if [ "$DRY_RUN" -eq 1 ]; then
  # Replace every function that would reach prlctl, ssh or scp with one that
  # prints what it would do. A guest script on stdin is printed too.
  # shellcheck disable=SC2317,SC2329 # called through the shared functions
  vm_others_running() { dry "prlctl list -H -o status,name   (refuse if a VM other than $VM_NAME runs)" >&2; }
  # shellcheck disable=SC2317,SC2329
  vm_revert_clean() { dry "prlctl snapshot-switch $VM_NAME -i <id of snapshot '$CLEAN_SNAPSHOT_NAME'>"; }
  # shellcheck disable=SC2317,SC2329
  vm_start() { dry "prlctl start $VM_NAME   (resume when suspended or paused)"; }
  # shellcheck disable=SC2317,SC2329
  vm_wait_ssh() { dry "wait up to $1 tries, five seconds apart, for ssh to answer in $VM_NAME"; }
  # shellcheck disable=SC2317,SC2329
  vm_status() { printf 'running\n'; }
  # shellcheck disable=SC2317,SC2329
  vm_stop() { dry "prlctl stop $VM_NAME   (--kill when it stays running)"; }
  # shellcheck disable=SC2317,SC2329
  guest_ssh() {
    dry "ssh guest (limit ${SSH_SECONDS:-900}s) $*"
    sed 's/^/dry-run:   | /'
  }
  # shellcheck disable=SC2317,SC2329
  guest_copy() {
    _gc_count=$#
    _gc_destination=$(eval "printf '%s' \"\${$_gc_count}\"")
    while [ "$#" -gt 1 ]; do
      dry "scp $1 -> guest:$(guest_lab)/$_gc_destination"
      shift
    done
  }
fi

# Copy one file from the guest's parity directory into the cell.
pull_file() {
  if [ "$DRY_RUN" -eq 1 ]; then
    dry "scp guest:$(guest_lab)/parity/$1 -> $CELL/$1"
    return 0
  fi
  bounded 600 scp -q -F "$CELL/sshconfig" -o BatchMode=yes \
    "guest:$(guest_lab)/parity/$1" "$CELL/$1" </dev/null
}

# Stop and revert the VM once it has been touched; runs at every exit, so a
# failed step cannot leave the guest dirty.
VM_TOUCHED=0
cleanup() {
  trap - EXIT
  [ "$VM_TOUCHED" -eq 1 ] || return 0
  vm_stop
  vm_revert_clean
}

# The guest-side name of a package argument: fsmeta.test.exe and fsmeta both
# give fsmeta.
package_name() {
  _pn_name=${1%.exe}
  printf '%s\n' "${_pn_name%.test}"
}

# Cross-compile into the cell: the collector, the parity test binary, and
# one test binary for each extra package.
build_binaries() {
  cd "$REPO" || die "cannot enter $REPO"
  round_log "building the Windows ARM64 binaries into $CELL"
  host_run env CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build \
    -o "$CELL/doubleagent.exe" ./cmd/doubleagent || die "go build failed"
  host_run env GOOS=windows GOARCH=arm64 go test -c -tags parity \
    -o "$CELL/parity.test.exe" ./internal/collect/parity || die "parity test build failed"
  for _bb_argument in "$@"; do
    _bb_name=$(package_name "$_bb_argument")
    check_word "$_bb_name"
    host_run env GOOS=windows GOARCH=arm64 go test -c \
      -o "$CELL/$_bb_name.test.exe" "./internal/$_bb_name" ||
      die "test build of internal/$_bb_name failed"
  done
  host_run git archive --format=zip -o "$CELL/tree.zip" HEAD collectors ||
    die "git archive failed"
}

# Run one test binary in the guest, with its output in NAME.log and its exit
# code in NAME.rc, both in the guest's parity directory. $1 is NAME, $2 is
# 1 to set PARITY_LIVE (the two live registry cases of the parity tests).
guest_run_tests() {
  _gr_native="$(guest_lab_native)\\parity"
  {
    printf "\$parity = '%s'\n" "$_gr_native"
    printf "\$name = '%s'\n" "$1"
    printf "\$live = '%s'\n" "$2"
    cat <<'EOF_GUEST'
$env:PARITY_REPO = Join-Path $parity 'repo'
$env:PARITY_BINARY = Join-Path $parity 'doubleagent.exe'
if ($live -eq '1') { $env:PARITY_LIVE = '1' }
Set-Location $parity
$exe = Join-Path $parity "$name.test.exe"
$log = Join-Path $parity "$name.log"
cmd /c "`"$exe`" -test.v -test.timeout 90m -test.parallel 2 > `"$log`" 2>&1"
Set-Content -Encoding ascii -Path (Join-Path $parity "$name.rc") -Value $LASTEXITCODE
EOF_GUEST
  } | SSH_SECONDS=5400 guest_ssh powershell -NoProfile -ExecutionPolicy Bypass -Command -
  # A timeout or a lost connection ends the run; the missing .rc file below
  # then reports FAIL.
  return 0
}

# Pull NAME.log and NAME.rc, print the log's tail and PASS or FAIL. Returns
# 1 for FAIL, which includes a missing exit code.
report_tests() {
  _rt_code=
  pull_file "$1.log" || round_log "could not pull $1.log"
  if pull_file "$1.rc"; then
    if [ "$DRY_RUN" -eq 1 ]; then
      _rt_code=0
    else
      _rt_code=$(tr -d '\r \n' <"$CELL/$1.rc")
    fi
  fi
  if [ "$DRY_RUN" -eq 1 ]; then
    dry "tail -n 40 $CELL/$1.log"
  elif [ -f "$CELL/$1.log" ]; then
    printf -- '--- last lines of %s.log\n' "$1"
    tail -n 40 "$CELL/$1.log"
  fi
  if [ "$_rt_code" = 0 ]; then
    printf '%s: PASS\n' "$1"
    return 0
  fi
  printf '%s: FAIL (exit code %s)\n' "$1" "${_rt_code:-missing}"
  return 1
}

# Prepare the guest's parity directory, ship the files and unpack the tree.
guest_prepare() {
  _gp_native="$(guest_lab_native)\\parity"
  {
    printf "\$parity = '%s'\n" "$_gp_native"
    cat <<'EOF_GUEST'
if (Test-Path $parity) { Remove-Item -Recurse -Force $parity }
New-Item -ItemType Directory -Force -Path $parity | Out-Null
EOF_GUEST
  } | guest_ssh powershell -NoProfile -ExecutionPolicy Bypass -Command - ||
    die "preparing the guest directory failed"
  guest_copy "$CELL/doubleagent.exe" "$CELL/parity.test.exe" "$CELL/tree.zip" parity
  for _gp_argument in "$@"; do
    _gp_name=$(package_name "$_gp_argument")
    guest_copy "$CELL/$_gp_name.test.exe" parity
  done
  {
    printf "\$parity = '%s'\n" "$_gp_native"
    cat <<'EOF_GUEST'
Expand-Archive -Force -Path (Join-Path $parity 'tree.zip') -DestinationPath (Join-Path $parity 'repo')
EOF_GUEST
  } | guest_ssh powershell -NoProfile -ExecutionPolicy Bypass -Command - ||
    die "unpacking the tree in the guest failed"
}

main() {
  [ "$DRY_RUN" -eq 1 ] || mkdir -p "$CELL" || die "cannot create $CELL"
  _mn_others=$(vm_others_running)
  [ -z "$_mn_others" ] || die "another VM is running: $_mn_others"
  # Build before the VM is touched, so a compile error costs no revert.
  build_binaries "$@"
  trap cleanup EXIT
  VM_TOUCHED=1
  vm_revert_clean
  vm_start
  vm_wait_ssh 60 || die "no ssh answer from $VM_NAME"
  guest_prepare "$@"
  _mn_failed=0
  guest_run_tests parity 1
  report_tests parity || _mn_failed=1
  for _mn_argument in "$@"; do
    _mn_name=$(package_name "$_mn_argument")
    guest_run_tests "$_mn_name" 0
    report_tests "$_mn_name" || _mn_failed=1
  done
  printf 'Remove-Item -Recurse -Force "%s"\n' "$(guest_lab_native)\\parity" |
    guest_ssh powershell -NoProfile -ExecutionPolicy Bypass -Command - ||
    round_log "could not remove the guest directory; the revert discards it"
  cleanup
  exit "$_mn_failed"
}

main "$@"
