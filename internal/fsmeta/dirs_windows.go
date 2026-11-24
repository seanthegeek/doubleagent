package fsmeta

import (
	"errors"
	"os"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// errNotDirectory is the error a path that is not a directory gives. It is
// not syscall.ENOTDIR, which Windows words as a missing path, so an
// error_read row for a file reads as one.
var errNotDirectory = errors.New("not a directory")

// openToList opens path to list it, following a link or junction.
func openToList(path string) (*os.File, error) {
	return osfile.Open(path)
}

// EnterError is always nil: Windows lets every account pass through a
// directory it cannot list.
func EnterError(string) error {
	return nil
}
