//go:build unix

package fsmeta

import (
	"os"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// errNotDirectory is the error a path that is not a directory gives, the
// system's own, so the text is the one the system gives.
var errNotDirectory error = syscall.ENOTDIR

// openToList opens path to list it, following a link; O_DIRECTORY refuses
// anything else, so a named pipe at path never blocks the open.
func openToList(path string) (*os.File, error) {
	return osfile.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY, 0)
}

// EnterError returns the error looking up path/. gives, nil when the
// effective user may enter the directory path: the kernel checks the right
// to search a directory before it looks up any name in it.
func EnterError(path string) error {
	_, err := os.Lstat(path + "/.")
	return err
}
