package fsmeta

import (
	"os"
	"strings"
	"syscall"
)

// DirReadError says why path cannot be listed or entered, as the sh
// collector's get_dir_read_error_var does, with Go's error text where the
// sh collector prints ls's. It reports failed false for a directory that can
// be both listed and entered. The tests follow a symlink, as test(1) does,
// because the callers give it directories and homes that may be links. Only
// the first line of the text is kept, since a path can hold a newline.
func DirReadError(path string) (reason string, failed bool) {
	info, statError := os.Stat(path)
	switch {
	case statError != nil || !info.IsDir():
		return firstLine(notDirectoryError(path)), true
	case !canAccess(path, readBit):
		return firstLine(openError(path)), true
	case !canAccess(path, executeBit):
		return firstLine(enterError(path)), true
	}
	return "", false
}

// notDirectoryError is the text for a path that is not a directory: the
// type error when something is there (a file, or a dangling link), else the
// error lstat gives for the missing path.
func notDirectoryError(path string) string {
	_, err := os.Lstat(path)
	if err == nil {
		return pathError("open", path, syscall.ENOTDIR).Error()
	}
	return err.Error()
}

// openError is the error opening path for listing gives. When the open
// succeeds although the access test said no, it is the permission error.
func openError(path string) string {
	handle, err := os.Open(path)
	if err == nil {
		_ = handle.Close()
		return pathError("open", path, syscall.EACCES).Error()
	}
	return err.Error()
}

// enterError is the error looking up path/. gives, or the permission error
// when the lookup succeeds although the access test said no.
func enterError(path string) string {
	_, err := os.Lstat(path + "/.")
	if err == nil {
		return pathError("lstat", path+"/.", syscall.EACCES).Error()
	}
	return err.Error()
}

// firstLine returns text up to its first newline.
func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}
