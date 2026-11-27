package fsmeta

import (
	"errors"
	"io"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// ListError returns the error opening the directory path to list it, a link
// followed, and reading its first entry gives, nil when both succeed (an
// empty directory included); refused is opener's Admit's refusal of the
// handle. The entry is read only when the directory can be entered
// (EnterError): reading one that cannot would move its access time though
// nothing below it is collected, so such a directory counts as listable
// once it opens.
func ListError(opener *Opener, path string) (err, refused error) {
	handle, err := openToList(path)
	if err != nil {
		return err, nil
	}
	if handle, refused = opener.admitted(path, handle); refused != nil {
		return nil, refused
	}
	defer handle.Close()
	if EnterError(path) != nil {
		return nil, nil
	}
	if _, err := handle.ReadDir(1); err != nil && !errors.Is(err, io.EOF) {
		return err, nil
	}
	return nil, nil
}

// notDirectoryError is the error for a path that is not a directory: the
// type error when something is there (a file, or a dangling link), else the
// error Lstat gives for the missing path.
func notDirectoryError(path string) error {
	_, err := Lstat(path)
	if err == nil {
		return pathError("open", syspath.ToNative(path), errNotDirectory)
	}
	return err
}

// firstLine returns text up to its first newline.
func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

// nativeFirstLine is the first line of err's text with its paths in native
// form.
func nativeFirstLine(err error) string {
	return firstLine(syspath.NativeErrors(err).Error())
}

// DirReadError says why path cannot be listed or entered, with the error
// text of the operation that failed, its paths in native form (exception 1):
// a stat that finds no directory, opening it (ListError), the right to enter
// it (EnterError), then reading one entry (ListError). It reports failed
// false for a directory that passes them all. A link is followed, because
// the callers give it directories and homes that may be links. Only the
// first line of the text is kept, since a path can hold a newline. A handle
// opener's Admit refuses comes back as refused, with no reason.
func DirReadError(opener *Opener, path string) (reason string, failed bool, refused error) {
	info, err := osfile.Stat(path)
	if err != nil || !info.IsDir() {
		return nativeFirstLine(notDirectoryError(path)), true, nil
	}
	if err, refused := ListError(opener, path); refused != nil {
		return "", true, refused
	} else if err != nil {
		return nativeFirstLine(err), true, nil
	}
	if err := EnterError(path); err != nil {
		return nativeFirstLine(err), true, nil
	}
	return "", false, nil
}
