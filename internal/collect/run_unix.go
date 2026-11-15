//go:build unix

package collect

import (
	"os"

	"golang.org/x/sys/unix"
)

// The access checks resolveOutDir and the inventory make, as test(1) -x, -w
// and -r decide them.
const (
	accessEnter = unix.X_OK
	accessWrite = unix.W_OK
	accessRead  = unix.R_OK
)

// restrictUmask sets the process umask to 077, so every file and directory
// the run creates is readable by its owner only, as the sh collector's
// `umask 077` makes them.
func restrictUmask() { unix.Umask(0o077) }

// canAccess reports whether the effective user may access path with mode,
// following a link to its target.
func canAccess(path string, mode uint32) bool {
	return unix.Faccessat(unix.AT_FDCWD, path, mode, unix.AT_EACCESS) == nil
}

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
