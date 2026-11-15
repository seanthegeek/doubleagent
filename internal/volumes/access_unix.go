//go:build unix

package volumes

import "golang.org/x/sys/unix"

// canRead reports whether the effective user may read path, as test -r does,
// following a link to its target.
func canRead(path string) bool {
	return unix.Faccessat(unix.AT_FDCWD, path, unix.R_OK, unix.AT_EACCESS) == nil
}

// canEnter reports whether the effective user may search path, as test -x
// does, following a link to its target.
func canEnter(path string) bool {
	return unix.Faccessat(unix.AT_FDCWD, path, unix.X_OK, unix.AT_EACCESS) == nil
}
