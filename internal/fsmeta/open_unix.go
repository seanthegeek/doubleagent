//go:build unix

package fsmeta

import (
	"os"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// Open opens path for reading, failing on a symlink at the last component
// rather than following it, and returning at once on a pipe with no writer
// rather than waiting for one; the caller checks what it opened. The open is
// read-only whatever the opener says: these systems offer no request to
// hold the access time, which the mount options decide.
func (o *Opener) Open(path string) (*os.File, error) {
	return osfile.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
