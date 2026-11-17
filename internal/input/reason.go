package input

import (
	"errors"
	"io/fs"
	"strings"
)

// errNotDirectory and errIsDirectory are the reasons given for a path whose
// type the operation could not use, without asking the system.
var (
	errNotDirectory = errors.New("not a directory")
	errIsDirectory  = errors.New("is a directory")
)

// errorReason returns the reason a problem line or note gives for err,
// without the path it names: a fixed lower-case phrase for a permission or
// missing-file error, else the innermost error's own text lower-cased. It
// is the one place the wording is chosen, so that the shared wording of the
// operating-system errors can replace it.
func errorReason(err error) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "permission denied"
	case errors.Is(err, fs.ErrNotExist):
		return "no such file or directory"
	}
	var pathError *fs.PathError
	if errors.As(err, &pathError) {
		err = pathError.Err
	}
	return strings.ToLower(err.Error())
}

// describeError returns "<path>: <reason>" for a problem line.
func describeError(path string, err error) string {
	return path + ": " + errorReason(err)
}
