package fsmeta

import (
	"errors"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// errNotDirectory is the error a path that is not a directory gives. It is
// not syscall.ENOTDIR, which Windows words as a missing path, so an
// error_read row for a file reads as one.
var errNotDirectory = errors.New("not a directory")

// DirReadError says why path cannot be listed, as the PowerShell collector's
// Get-DirReadError does by listing it, with Go's error text in native path
// form. It reports failed false for a directory that can be listed. A link
// or junction is followed, because the callers give it directories and homes
// that may be links. A directory that can be listed can always be entered,
// since Windows grants every user the right to pass through directories.
// Only the first line of the text is kept, since a path can hold a newline.
func DirReadError(path string) (reason string, failed bool) {
	info, statError := osfile.Stat(path)
	if statError != nil || !info.IsDir() {
		return nativeFirstLine(notDirectoryError(path)), true
	}
	handle, err := osfile.Open(path)
	if err == nil {
		_, err = handle.ReadDir(-1)
		_ = handle.Close()
	}
	if err != nil {
		return nativeFirstLine(err), true
	}
	return "", false
}

// nativeFirstLine is the first line of err's text with its paths in native
// form.
func nativeFirstLine(err error) string {
	return firstLine(syspath.NativeErrors(err).Error())
}
