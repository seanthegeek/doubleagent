// Package follow is the one place the binary follows a link in the last
// component of a path derived from the image root: a stat, a directory or
// regular-file test, an access check, a directory listing test, a read of a
// whole file or a resolution to the physical directory. Each function takes
// the run's root check and asks it about the full path before following
// anything; when the path leads out of the image root it follows nothing
// and returns an *OutOfRootError naming the link, which the caller handles.
// A live run passes a nil check, and the functions follow as the operating
// system does. Calls that never follow (lstat, readlink, listing a directory
// whose own entry was typed a directory) stay where they are. A test in this
// package fails on any other call in the tree of a function that follows.
//
// The test reads package-level function references only, so it cannot see:
// a method that follows (fsmeta.Opener.OpenRegular opens with O_NOFOLLOW or
// FILE_FLAG_OPEN_REPARSE_POINT and refuses a link; hostcmd.Plutil runs plutil
// on a hint file whose own lstat typed it a regular file, under checked
// paths); a call through an interface
// or a function value (glob's and the walk's are this package's own
// functions or the root check); or a new following export of an owner
// package (fsmeta, osfile, stage), whose reviewer must add it to the list.
package follow

import (
	"errors"
	"io/fs"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/stage"
)

// Check reports whether path leads out of the image root, and the link that
// decides it; walk.RootCheck provides it, nil on a live run.
type Check func(path string) (link string, leaves bool)

// OutOfRootError says that Path leads out of the image root at Link, so
// nothing was followed.
type OutOfRootError struct {
	Path string
	Link string
}

// Error describes the refusal.
func (e *OutOfRootError) Error() string {
	return e.Path + ": leads out of the image root at " + e.Link
}

// LeavesRoot returns the error that says path leads out of the image root,
// or nil when check is nil or lets it through.
func LeavesRoot(check Check, path string) error {
	if check == nil {
		return nil
	}
	if link, leaves := check(path); leaves {
		return &OutOfRootError{Path: path, Link: link}
	}
	return nil
}

// OutOfRoot returns the link of err when it is an *OutOfRootError.
func OutOfRoot(err error) (string, bool) {
	var refusal *OutOfRootError
	if errors.As(err, &refusal) {
		return refusal.Link, true
	}
	return "", false
}

// Stat is os.Stat of path, the last component followed.
func Stat(check Check, path string) (fs.FileInfo, error) {
	if err := LeavesRoot(check, path); err != nil {
		return nil, err
	}
	return osfile.Stat(path)
}

// IsDirectory is test -d: whether path is a directory or a link to one.
func IsDirectory(check Check, path string) (bool, error) {
	info, err := Stat(check, path)
	if _, out := OutOfRoot(err); out {
		return false, err
	}
	return err == nil && info.IsDir(), nil
}

// IsRegularFile is test -f: whether path is a regular file or a link to one.
func IsRegularFile(check Check, path string) (bool, error) {
	info, err := Stat(check, path)
	if _, out := OutOfRoot(err); out {
		return false, err
	}
	return err == nil && info.Mode().IsRegular(), nil
}

// ReadFile returns the bytes of the file path names, the last component
// followed, as test -f and cat follow it.
func ReadFile(check Check, path string) ([]byte, error) {
	if err := LeavesRoot(check, path); err != nil {
		return nil, err
	}
	return osfile.ReadFile(path)
}

// DirReadError is fsmeta.DirReadError of path: why the directory it names
// cannot be listed or entered. A path that leads out of the image root is
// reported as failed, with the refusal as its reason and in err, so a caller
// that drops err cannot read it as readable.
func DirReadError(check Check, path string) (reason string, failed bool, err error) {
	if err := LeavesRoot(check, path); err != nil {
		return err.Error(), true, err
	}
	reason, failed = fsmeta.DirReadError(path)
	return reason, failed, nil
}

// PhysicalDir is stage.PhysicalDir of path: the directory it resolves to,
// every link followed.
func PhysicalDir(check Check, path string) (string, error) {
	if err := LeavesRoot(check, path); err != nil {
		return "", err
	}
	return stage.PhysicalDir(path)
}

// OtherReparsePoint reports whether path is a reparse point other than a
// symlink or junction (fsmeta.Meta.IsOtherReparsePoint), whose target, if any,
// os.Readlink cannot read: the root check refuses it, since a filter driver
// on the analyst's machine may act on it, but a log line must not call it a
// symlink that leads out. Never on Unix.
func OtherReparsePoint(path string) bool {
	meta, err := fsmeta.Lstat(path)
	return err == nil && meta.IsOtherReparsePoint()
}

// OtherReparseReason is the end of a log line for a path OtherReparsePoint
// reports.
const OtherReparseReason = "is a reparse point other than a symlink or junction; " +
	"nothing behind it was looked at"
