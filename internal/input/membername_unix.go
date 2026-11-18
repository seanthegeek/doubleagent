//go:build !windows

package input

import (
	"errors"
	"syscall"
)

// hostMemberName reports whether this system stores a member name's
// component as written; on POSIX any name checkElement accepts may be
// tried, and a refusal is the system's error (nameRefused).
func hostMemberName(string) bool { return true }

// nameRefused reports whether err is the system refusing a name it cannot
// hold: one longer than the file system stores, or (macOS) one that is not
// valid UTF-8.
func nameRefused(err error) bool {
	return errors.Is(err, syscall.ENAMETOOLONG) || errors.Is(err, syscall.EILSEQ)
}
