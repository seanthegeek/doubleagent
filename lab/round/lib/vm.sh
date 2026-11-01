# shellcheck shell=sh
# vm.sh: the host half of a round cell, sourced by round.sh on the analyst
# machine. It drives Parallels Desktop's prlctl (revert to the clean
# snapshot, start, snapshot, stop) and reaches the guest over ssh with the
# cell's own ssh config file, written from the address prlctl reports.
# Every call that can hang is bounded with `bounded`.
#
# Reads the globals CELL (the cell directory), VM_NAME (the prlctl name of
# the VM) and, from vms/<VM>.conf, GUEST_OS (linux, macos or windows),
# GUEST_USER and CLEAN_SNAPSHOT_NAME.

# The globals above come from round.sh, which also calls every function.
# shellcheck disable=SC2154,SC2329
PRLCTL=${PRLCTL:-prlctl}
if ! command -v "$PRLCTL" >/dev/null 2>&1; then
  PRLCTL='/Applications/Parallels Desktop.app/Contents/MacOS/prlctl'
fi

# Print a line to stderr and append it, with the UTC time, to the cell's
# round.log, the record of every command the round sent to the VM.
round_log() {
  printf 'round: %s\n' "$*" >&2
  if [ -n "${CELL:-}" ] && [ -d "$CELL" ]; then
    printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" >>"$CELL/round.log"
  fi
}

die() {
  round_log "error: $*"
  exit 1
}

# Run a command with a wall-clock limit of $1 seconds. macOS has no
# timeout(1); perl's alarm survives the exec, so the command is killed with
# SIGALRM (exit 142) when the limit is reached.
bounded() {
  _bd_limit=$1
  shift
  # shellcheck disable=SC2016 # perl's own variables, not the shell's
  perl -e 'alarm shift @ARGV; exec @ARGV or die "exec $ARGV[0]: $!\n"' "$_bd_limit" "$@"
}

prl() {
  bounded "${PRLCTL_SECONDS:-600}" "$PRLCTL" "$@" </dev/null
}

# The status word prlctl prints for the VM: running, stopped, suspended,
# paused or another of its states.
vm_status() {
  prl status "$VM_NAME" | awk '{ print $NF }'
}

# Print the names of the running VMs other than $VM_NAME, one per line.
vm_others_running() {
  prl list -H -o status,name | awk -v self="$VM_NAME" '
    $1 == "running" { sub(/^[^ ]+ +/, ""); if ($0 != self) print }'
}

# Print the id of the snapshot named CLEAN_SNAPSHOT_NAME, compared without
# case; fail unless exactly one snapshot has that name. CLEAN_SNAPSHOT_ID,
# when set, is used as given.
vm_clean_snapshot_id() {
  if [ -n "${CLEAN_SNAPSHOT_ID:-}" ]; then
    printf '%s\n' "$CLEAN_SNAPSHOT_ID"
    return 0
  fi
  _cs_ids=$(prl snapshot-list "$VM_NAME" -j | awk -v want="$CLEAN_SNAPSHOT_NAME" '
    /^[[:space:]]*"\{[0-9a-fA-F-]+\}": \{/ { split($0, part, "\""); snapshot_id = part[2] }
    /^[[:space:]]*"name": / {
      line = $0; sub(/^[^:]*: *"/, "", line); sub(/",?[[:space:]]*$/, "", line)
      if (tolower(line) == tolower(want)) print snapshot_id
    }')
  _cs_count=$(printf '%s\n' "$_cs_ids" | grep -c . || true)
  [ "$_cs_count" -eq 1 ] ||
    die "expected one snapshot named '$CLEAN_SNAPSHOT_NAME' on $VM_NAME, found $_cs_count"
  printf '%s\n' "$_cs_ids"
}

vm_revert_clean() {
  _rc_snapshot=$(vm_clean_snapshot_id) || exit 1
  round_log "prlctl snapshot-switch $VM_NAME -i $_rc_snapshot (clean snapshot)"
  prl snapshot-switch "$VM_NAME" -i "$_rc_snapshot" >/dev/null || die "snapshot-switch failed"
}

vm_start() {
  _vs_status=$(vm_status)
  case $_vs_status in
    running) ;;
    suspended | paused)
      round_log "prlctl resume $VM_NAME"
      prl resume "$VM_NAME" >/dev/null || die "resume failed"
      ;;
    *)
      round_log "prlctl start $VM_NAME"
      prl start "$VM_NAME" >/dev/null || die "start failed"
      ;;
  esac
}

# Print the VM's first IPv4 address from `prlctl list -f`, or nothing while
# the guest tools have not reported one.
vm_address() {
  prl list -f -H -o ip,name | awk -v self="$VM_NAME" '{
      address = $1; name = $0; sub(/^[^ ]+ +/, "", name)
      if (name == self && address != "-") { split(address, first, ","); print first[1]; exit }
    }'
}

vm_write_sshconfig() {
  cat >"$CELL/sshconfig" <<EOF_SSHCONFIG
# Written by round.sh from \`prlctl list -f\` for VM $VM_NAME.
Host guest
  HostName $1
  User $GUEST_USER
  IdentityFile ${ROUND_SSH_KEY:-$HOME/.ssh/doubleagent_lab}
  IdentitiesOnly yes
  UserKnownHostsFile ${ROUND_KNOWN_HOSTS:-$CELL/known_hosts}
  StrictHostKeyChecking accept-new
  ServerAliveInterval 15
  ServerAliveCountMax 4
EOF_SSHCONFIG
}

# Wait until the guest answers over ssh, rewriting the ssh config from the
# current address before each try; $1 tries, five seconds apart.
vm_wait_ssh() {
  _ws_tries=0
  while [ "$_ws_tries" -lt "$1" ]; do
    _ws_tries=$((_ws_tries + 1))
    _ws_address=$(vm_address)
    if [ -n "$_ws_address" ]; then
      vm_write_sshconfig "$_ws_address"
      if bounded 20 ssh -F "$CELL/sshconfig" -o ConnectTimeout=5 -o BatchMode=yes guest exit \
        </dev/null >/dev/null 2>&1; then
        round_log "ssh answers at $_ws_address after $_ws_tries tries"
        return 0
      fi
    fi
    sleep 5
  done
  return 1
}

# Run a command in the guest; stdin is /dev/null unless the caller
# redirects it. The remote command string is joined from the arguments.
guest_ssh() {
  bounded "${SSH_SECONDS:-900}" ssh -F "$CELL/sshconfig" -o BatchMode=yes guest "$@"
}

# The lab directory in the guest, as scp and the guest scripts name it.
guest_lab() {
  case $GUEST_OS in
    windows) printf 'C:/Users/%s/lab\n' "$GUEST_USER" ;;
    *) printf 'lab\n' ;;
  esac
}

guest_lab_native() {
  case $GUEST_OS in
    windows) printf 'C:\\Users\\%s\\lab\n' "$GUEST_USER" ;;
    *) printf 'lab\n' ;;
  esac
}

# Copy local files into the guest's lab directory: guest_copy FILE... DEST,
# DEST relative to the lab directory ("." for the directory itself).
guest_copy() {
  _gc_count=$#
  _gc_destination=$(eval "printf '%s' \"\${$_gc_count}\"")
  while [ "$#" -gt 1 ]; do
    round_log "scp $(basename "$1") -> $(guest_lab_native)/$_gc_destination"
    bounded 600 scp -q -F "$CELL/sshconfig" -o BatchMode=yes "$1" \
      "guest:$(guest_lab)/$_gc_destination" </dev/null || die "scp of $1 failed"
    shift
  done
}

# Refuse an argument that the remote shell (sh, or cmd.exe on Windows)
# could split or interpret; anything else goes through stdin.
check_word() {
  case $1 in
    '' | *[!A-Za-z0-9._,:+=/@-]*) die "unsafe argument for the guest: '$1'" ;;
  esac
}

# Run one of the guest scripts: guest_tool NAME ARG..., NAME being survey,
# drive or make-project. stdin passes through.
guest_tool() {
  _gt_name=$1
  shift
  for _gt_word in "$@"; do check_word "$_gt_word"; done
  case $GUEST_OS in
    windows)
      guest_ssh powershell -NoProfile -ExecutionPolicy Bypass \
        -File "$(guest_lab_native)\\$_gt_name.ps1" "$@"
      ;;
    *) guest_ssh sh "lab/$_gt_name.sh" "$@" ;;
  esac
}

# Run a shell command the guest reads on stdin, the way a person types it
# in a terminal, in the sample project when it exists: a login shell on
# Linux and macOS (so PATH has what the installer added), Windows
# PowerShell 5.1 on Windows.
guest_shell() {
  case $GUEST_OS in
    windows)
      {
        printf '%s\n' "Set-Location -ErrorAction SilentlyContinue \"\$env:USERPROFILE\\lab\\proj\""
        cat
      } | guest_ssh powershell -NoProfile -ExecutionPolicy Bypass -Command -
      ;;
    macos) guest_ssh 'cd ~/lab/proj 2>/dev/null; exec zsh -l -s' ;;
    *) guest_ssh 'cd ~/lab/proj 2>/dev/null; exec bash -l -s' ;;
  esac
}

# Take a snapshot named $1 and record its id in the cell, so finish deletes
# it and only it.
vm_snapshot() {
  round_log "prlctl snapshot $VM_NAME -n $1"
  _sn_output=$(prl snapshot "$VM_NAME" -n "$1") || die "snapshot $1 failed"
  _sn_snapshot=$(printf '%s\n' "$_sn_output" | sed -n 's/.*\({[0-9a-fA-F-]*}\).*/\1/p' | head -n 1)
  [ -n "$_sn_snapshot" ] || die "no snapshot id in prlctl's answer for $1"
  printf '%s %s\n' "$_sn_snapshot" "$1" >>"$CELL/snapshots"
}

vm_delete_own_snapshots() {
  [ -f "$CELL/snapshots" ] || return 0
  while read -r _ds_snapshot _ds_name; do
    [ -n "$_ds_snapshot" ] || continue
    round_log "prlctl snapshot-delete $VM_NAME -i $_ds_snapshot ($_ds_name)"
    prl snapshot-delete "$VM_NAME" -i "$_ds_snapshot" >/dev/null ||
      round_log "could not delete $_ds_snapshot"
  done <"$CELL/snapshots"
  mv "$CELL/snapshots" "$CELL/snapshots.deleted"
}

# Print a value that changes at every boot of the guest.
vm_boot_id() {
  case $GUEST_OS in
    windows)
      printf '%s\n' '(Get-CimInstance Win32_OperatingSystem).LastBootUpTime.ToString("o")' |
        guest_shell 2>/dev/null | tr -d '\r'
      ;;
    *)
      guest_ssh 'cat /proc/sys/kernel/random/boot_id 2>/dev/null || sysctl -n kern.boottime' \
        </dev/null 2>/dev/null
      ;;
  esac
}

# Reboot the guest from inside, as a person would, and wait until it is
# back with a new boot id; up to $1 tries of the ssh wait.
vm_reboot() {
  _rb_before=$(vm_boot_id)
  [ -n "$_rb_before" ] || die "could not read the boot id before the reboot"
  round_log "reboot from inside the guest"
  case $GUEST_OS in
    windows) printf 'shutdown /r /t 0\n' | SSH_SECONDS=30 guest_shell >/dev/null 2>&1 || true ;;
    macos) SSH_SECONDS=30 guest_ssh 'sudo -n shutdown -r now' </dev/null >/dev/null 2>&1 || true ;;
    *) SSH_SECONDS=30 guest_ssh 'sudo -n systemctl reboot' </dev/null >/dev/null 2>&1 || true ;;
  esac
  _rb_round=0
  while [ "$_rb_round" -lt "$1" ]; do
    _rb_round=$((_rb_round + 1))
    sleep 10
    vm_wait_ssh 6 || continue
    _rb_after=$(vm_boot_id)
    if [ -n "$_rb_after" ] && [ "$_rb_after" != "$_rb_before" ]; then
      round_log "guest back after the reboot"
      return 0
    fi
  done
  die "the guest did not come back with a new boot id"
}

vm_stop() {
  [ "$(vm_status)" = running ] || return 0
  round_log "prlctl stop $VM_NAME"
  PRLCTL_SECONDS=300 prl stop "$VM_NAME" >/dev/null 2>&1 || true
  if [ "$(vm_status)" = running ]; then
    round_log "prlctl stop $VM_NAME --kill"
    prl stop "$VM_NAME" --kill >/dev/null || die "stop --kill failed"
  fi
}
