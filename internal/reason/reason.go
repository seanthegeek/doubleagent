// Package reason turns an error that stopped a read into the short text the
// analysis commands print in a `parser:` row, a `problem:` line or an error
// message.
//
// The common conditions of the operating system get one fixed lower-case
// reason each, the same on Linux, macOS and Windows: "permission denied",
// "no such file or directory", "is a directory", "not a directory", "too
// many levels of symbolic links" and "file name too long". Any other error
// gives the text of its innermost cause, without the operation and path Go
// puts in front of it.
//
// err.Error() names the operation and path and words a condition as each
// system does, so one denied file reads differently on each platform; this
// package gives one wording everywhere.
package reason

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// The fixed reasons. For tests the platform's own conditions (a directory,
// not a directory, a link loop, a long name) first, then permission, then
// a missing file.
const (
	PermissionDenied = "permission denied"
	NotFound         = "no such file or directory"
	IsDirectory      = "is a directory"
	NotDirectory     = "not a directory"
	LinkLoop         = "too many levels of symbolic links"
	NameTooLong      = "file name too long"
)

// For returns the reason err gives: one of the fixed reasons for an
// operating-system error the package classifies, else the text of the
// innermost error under every *fs.PathError, *os.LinkError and
// *os.SyscallError of its chain, else err's own text. A nil err gives "".
func For(err error) string {
	if err == nil {
		return ""
	}
	if fixed := platformReason(err); fixed != "" {
		return fixed
	}
	switch {
	case errors.Is(err, fs.ErrPermission):
		return PermissionDenied
	case errors.Is(err, fs.ErrNotExist):
		return NotFound
	}
	return innermost(err).Error()
}

// Unreadable returns "<file name> could not be read (<reason>)" for the
// error that stopped a read of path, the file name being path's last
// element.
func Unreadable(path string, err error) string {
	return filepath.Base(path) + " could not be read (" + For(err) + ")"
}

// Describe returns "<path>: <reason>" for a problem line or an error
// message, the path being the one the error names when it names one (the
// first of a *os.LinkError's two), else path.
func Describe(path string, err error) string {
	var pathError *fs.PathError
	var linkError *os.LinkError
	switch {
	case errors.As(err, &pathError) && pathError.Path != "":
		path = pathError.Path
	case errors.As(err, &linkError) && linkError.Old != "":
		path = linkError.Old
	}
	return path + ": " + For(err)
}

// innermost returns the error under the last *fs.PathError, *os.LinkError
// or *os.SyscallError in err's chain, taking each in turn until none is
// left, or err itself when it has none.
func innermost(err error) error {
	for {
		var pathError *fs.PathError
		var linkError *os.LinkError
		var syscallError *os.SyscallError
		switch {
		case errors.As(err, &pathError):
			err = pathError.Err
		case errors.As(err, &linkError):
			err = linkError.Err
		case errors.As(err, &syscallError):
			err = syscallError.Err
		default:
			return err
		}
	}
}
