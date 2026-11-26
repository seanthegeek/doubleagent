package input

import (
	"errors"

	"github.com/seanthegeek/doubleagent/internal/reason"
)

// errNotDirectory and errIsDirectory are the reasons given for a path whose
// type the operation could not use, without asking the system.
var (
	errNotDirectory = errors.New("not a directory")
	errIsDirectory  = errors.New("is a directory")
)

// describeError returns "<path>: <reason>" for a problem line.
func describeError(path string, err error) string {
	return path + ": " + reason.For(err)
}
