//go:build unix

package collect

import (
	"os"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/relocation"
	"github.com/seanthegeek/doubleagent/internal/summary"
	"github.com/seanthegeek/doubleagent/internal/volumes"
	"golang.org/x/sys/unix"
)

// privilegeWarning is the start-log warning of a live run that is not root.
const privilegeWarning = "WARNING: not running as root; other users' homes will probably be unreadable"

// The access checks resolveOutDir makes, as test(1) -x and -w decide them.
const (
	accessEnter = unix.X_OK
	accessWrite = unix.W_OK
)

// restrictUmask sets the process umask to 077, so every file and directory
// the run creates is readable by its owner only.
func restrictUmask() { unix.Umask(0o077) }

// canAccess reports whether the effective user may access path with mode,
// following a link to its target.
func canAccess(path string, mode uint32) bool {
	return unix.Faccessat(unix.AT_FDCWD, path, mode, unix.AT_EACCESS) == nil
}

// liveRoot reports whether root, absolute and canonical, names the live
// system: -r / is treated as no root at all.
func liveRoot(root string) bool { return root == "/" }

// outDirWritable reports whether the run may create files in dir, as test
// -w decides.
func outDirWritable(dir string) bool { return canAccess(dir, accessWrite) }

// runIdentity is the account the run runs as: its effective user ID.
func runIdentity() summary.Identity { return summary.Identity{UID: os.Getuid()} }

// identityText is the account part of the first log line.
func identityText(identity summary.Identity) string { return "uid=" + strconv.Itoa(identity.UID) }

// privileged reports whether identity can read every user's home: root.
func privileged(identity summary.Identity) bool { return identity.UID == 0 }

// systemBases are where the volume step looks for the system volume
// directories: under root, which is "" in a live run.
func systemBases(root string) volumes.Bases {
	return volumes.Bases{System: root, ProgramData: root + "/ProgramData"}
}

// registryReader is the registry the relocation step reads: none off
// Windows.
func registryReader() relocation.Registry { return nil }

// systemName is what uname reports: the kernel's name, release and machine.
type systemName struct {
	name    string
	release string
	machine string
}

// uname returns the fields `uname -s`, `uname -r` and `uname -m` print, empty
// when the call fails.
func uname() systemName {
	var names unix.Utsname
	if err := unix.Uname(&names); err != nil {
		return systemName{}
	}
	return systemName{name: unix.ByteSliceToString(names.Sysname[:]),
		release: unix.ByteSliceToString(names.Release[:]),
		machine: unix.ByteSliceToString(names.Machine[:])}
}

// hostname returns the host's name as hostname(1) prints it, or "unknown".
func hostname() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "unknown"
	}
	return name
}
