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
// cannot be listed or entered. err is set only when path leads out of the
// image root.
func DirReadError(check Check, path string) (reason string, failed bool, err error) {
	if err := LeavesRoot(check, path); err != nil {
		return "", false, err
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

// reparsePointAttribute is FILE_ATTRIBUTE_REPARSE_POINT; Attributes is 0 on
// Unix.
const reparsePointAttribute = 0x400

// IsLink reports whether meta, an lstat, describes a link a path can run
// through or a reparse point of any kind, which nothing here follows.
func IsLink(meta fsmeta.Meta) bool {
	return meta.Kind == fsmeta.Symlink || meta.Attributes&reparsePointAttribute != 0
}

// NotALink reports whether path is a reparse point that is not a link (a
// cloud-file placeholder, a deduplicated file, an AF_UNIX socket): the root
// check refuses it, since a filter driver on the analyst's machine may act on
// it, but it leads nowhere, so a log line must not call it a link. Never on
// Unix.
func NotALink(path string) bool {
	meta, err := fsmeta.Lstat(path)
	return err == nil && meta.Kind != fsmeta.Symlink && meta.Attributes&reparsePointAttribute != 0
}

// NotALinkReason is the end of a log line for a path NotALink reports.
const NotALinkReason = "is a reparse point that is not a link; nothing behind it was looked at"
