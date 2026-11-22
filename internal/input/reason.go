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

// errorReason returns the reason a problem line or note gives for err,
// without the path it names: the fixed wording every command shares.
func errorReason(err error) string {
	return reason.For(err)
}

// describeError returns "<path>: <reason>" for a problem line.
func describeError(path string, err error) string {
	return path + ": " + errorReason(err)
}
