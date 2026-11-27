package sqlitefile

import (
	"errors"

	"github.com/seanthegeek/doubleagent/internal/reason"
)

// UnreadableError reports a database file or one of its sidecars that
// cannot be opened or read. Path is the file's path, kept for a log; Error
// names only its base name, as the analysis commands' messages do.
type UnreadableError struct {
	Path string
	Err  error
}

// Error returns "<file name> could not be read (<reason>)", the reason
// worded by reason.For as for every other file that cannot be read.
func (e *UnreadableError) Error() string {
	return reason.Unreadable(e.Path, e.Err)
}

// Unwrap returns the underlying error.
func (e *UnreadableError) Unwrap() error {
	return e.Err
}

// errSymbolicLink is the reason a symbolic link is not read.
var errSymbolicLink = errors.New("a symbolic link, not followed")

// errNotRegular is the reason a directory or special file is not read.
var errNotRegular = errors.New("not a regular file")
