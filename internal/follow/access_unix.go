//go:build unix

package follow

import (
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"golang.org/x/sys/unix"
)

// CanRead reports whether the effective user may read path, as test -r does,
// following a link to its target.
func CanRead(check Check, _ *fsmeta.Opener, path string) (bool, error) {
	if err := LeavesRoot(check, path); err != nil {
		return false, err
	}
	return unix.Faccessat(unix.AT_FDCWD, path, unix.R_OK, unix.AT_EACCESS) == nil, nil
}

// CanEnter reports whether the effective user may search path, as test -x
// does, following a link to its target.
func CanEnter(check Check, path string) (bool, error) {
	if err := LeavesRoot(check, path); err != nil {
		return false, err
	}
	return unix.Faccessat(unix.AT_FDCWD, path, unix.X_OK, unix.AT_EACCESS) == nil, nil
}
