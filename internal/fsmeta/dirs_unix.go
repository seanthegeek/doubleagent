//go:build unix

package fsmeta

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// errNotDirectory is the error a path that is not a directory gives, the
// system's own, so the text is the one the system gives.
var errNotDirectory error = syscall.ENOTDIR

// DirReadError says why path cannot be listed or entered, with Go's error
// text (exception 1). It reports failed false for a directory that can
// be both listed and entered. The tests follow a symlink, as test(1) does,
// because the callers give it directories and homes that may be links. Only
// the first line of the text is kept, since a path can hold a newline. It
// opens no handle, so there is nothing for an opener's Admit to judge.
func DirReadError(_ *Opener, path string) (reason string, failed bool, refused error) {
	info, statError := os.Stat(path)
	switch {
	case statError != nil || !info.IsDir():
		return firstLine(notDirectoryError(path).Error()), true, nil
	case !canAccess(path, unix.R_OK):
		return firstLine(openError(path)), true, nil
	case !canAccess(path, unix.X_OK):
		return firstLine(enterError(path)), true, nil
	}
	return "", false, nil
}

// canAccess reports whether the effective user may access path with the
// given unix.R_OK or unix.X_OK bits, as test(1) -r and -x decide, following
// a link to its target.
func canAccess(path string, mode uint32) bool {
	return unix.Faccessat(unix.AT_FDCWD, path, mode, unix.AT_EACCESS) == nil
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
