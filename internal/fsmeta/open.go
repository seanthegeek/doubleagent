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

// Opener opens the files a run reads as evidence (the copy of each collected
// file, and project discovery's and the relocation search's reads), and is
// the one place that decides whether an open asks Windows to hold the
// file's last-access time, which takes the right to change its attributes.
// ReadOnly (--no-preserve-atime) never asks. NTFSUpdatesOff says the NTFS
// setting has last-access updates off, so a file on NTFS is opened without
// the right (holdsAccessTime). A nil or zero Opener always asks. On Linux,
// macOS and other Unix systems every open is read-only.
type Opener struct {
	ReadOnly       bool
	NTFSUpdatesOff bool
	// fileSystemOf names the file system of a path's volume, "" when it
	// cannot be read or the Opener was not made by NewOpener.
	fileSystemOf func(path string) string
}

// NewOpener returns the opener of a run, with its own cache of the file
// system of each volume it opens a file on.
func NewOpener(readOnly, ntfsUpdatesOff bool) *Opener {
	return &Opener{ReadOnly: readOnly, NTFSUpdatesOff: ntfsUpdatesOff,
		fileSystemOf: newVolumeLookup()}
}

// holdsAccessTime reports whether an open of a file on a volume holding
// fileSystem asks for the right to hold its access time. Never under
// --no-preserve-atime (readOnly); otherwise always, unless the NTFS setting
// has updates off and the volume is NTFS: the setting says nothing about
// another file system (ReFS, as on a Dev Drive, FAT or exFAT), nor about a
// volume whose file system could not be read (""), so those are taken to
// update access times.
func holdsAccessTime(readOnly, ntfsUpdatesOff bool, fileSystem string) bool {
	return !readOnly && (!ntfsUpdatesOff || fileSystem != "NTFS")
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
