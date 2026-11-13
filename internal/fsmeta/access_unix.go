//go:build unix

package fsmeta

import "golang.org/x/sys/unix"

// canAccess reports whether the effective user may access path with the
// given unix.R_OK or unix.X_OK bits, as test(1) -r and -x decide, following
// a link to its target.
func canAccess(path string, mode uint32) bool {
	return unix.Faccessat(unix.AT_FDCWD, path, mode, unix.AT_EACCESS) == nil
}

const (
	readBit    = unix.R_OK
	executeBit = unix.X_OK
)
