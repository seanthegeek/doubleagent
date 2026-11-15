package fsmeta

import (
	"errors"
	"io/fs"
	"os"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// ErrNotRegular is the error OpenRegular wraps when path is not a regular
// file.
var ErrNotRegular = errors.New("not a regular file")

// Opener opens the files a run reads as evidence: the copy of each
// collected file and every read of a file the run searches (project
// discovery, the relocation search and sweep). It is the one place that
// decides whether an open asks Windows to leave the file's last-access time
// as it was, which takes the right to change the file's attributes.
// ReadOnly, set by --no-preserve-atime, never asks for that right.
// NTFSUpdatesOff says the system's NTFS setting has last-access updates off,
// so a read moves no access time and the right is not asked for. A nil
// Opener, or the zero one, always asks. On Linux, macOS and other Unix
// systems every open is read-only and the fields change nothing.
type Opener struct {
	ReadOnly       bool
	NTFSUpdatesOff bool
}

// NewOpener returns the opener of a run.
func NewOpener(readOnly, ntfsUpdatesOff bool) *Opener {
	return &Opener{ReadOnly: readOnly, NTFSUpdatesOff: ntfsUpdatesOff}
}

// OpenRegular opens path for reading when it is a regular file. It refuses a
// symlink, directory, pipe, socket or device by its lstat, then opens through
// Open, which follows no symlink or reparse point at the last component and
// waits on no pipe, and checks the open handle again, so a file swapped for
// a link or a pipe between the lstat and the open is refused too. Every
// error names path.
func (o *Opener) OpenRegular(path string) (*os.File, error) {
	info, err := osfile.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: ErrNotRegular}
	}
	handle, err := o.Open(path)
	if err != nil {
		return nil, err
	}
	opened, err := handle.Stat()
	if err != nil || !opened.Mode().IsRegular() {
		handle.Close()
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: ErrNotRegular}
	}
	return handle, nil
}
