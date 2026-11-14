//go:build unix

package relocation

import "golang.org/x/sys/unix"

// canEnter reports whether the effective user may search directory, as `cd`
// needs.
func canEnter(directory string) bool {
	return unix.Faccessat(unix.AT_FDCWD, directory, unix.X_OK, unix.AT_EACCESS) == nil
}
