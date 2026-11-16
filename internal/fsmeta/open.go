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

// Opener opens the files a run reads as evidence (each collected file's
// copy, project discovery's and the relocation search's reads), and decides
// whether an open asks Windows to hold the file's last-access time, which
// takes the right to change its attributes. ReadOnly (--no-preserve-atime)
// never asks; NTFSUpdatesOff (the NTFS setting has updates off) opens a file
// on a local NTFS volume without the right (holdsAccessTime); a nil or zero
// Opener always asks. On Unix systems every open is read-only. Report, when
// set, receives the path given to Open and the reason (verifyHold) for each
// file read on Windows without a hold the opener wanted. Admit (set on an
// image run on Windows) judges each handle Open and OpenDir return.
type Opener struct {
	ReadOnly       bool
	NTFSUpdatesOff bool
	Report         func(path, reason string)
	Admit          func(path string, handle *os.File) error
}

// NewOpener returns the opener of a run.
func NewOpener(readOnly, ntfsUpdatesOff bool) *Opener {
	return &Opener{ReadOnly: readOnly, NTFSUpdatesOff: ntfsUpdatesOff}
}

// holdsAccessTime reports whether a file on a local volume holding
// fileSystem is opened with the right to hold its access time. Never under
// --no-preserve-atime (readOnly); otherwise always, unless the NTFS setting
// has updates off and the volume is NTFS: the setting says nothing about
// another file system (ReFS, as on a Dev Drive, FAT or exFAT), nor about a
// remote volume or one whose file system could not be read (""), so those
// are taken to update access times.
func holdsAccessTime(readOnly, ntfsUpdatesOff bool, fileSystem string) bool {
	return !readOnly && (!ntfsUpdatesOff || fileSystem != "NTFS")
}

// admitted returns handle, just opened on path, when the opener's Admit lets
// it through, and otherwise closes it and returns Admit's error.
func (o *Opener) admitted(path string, handle *os.File) (*os.File, error) {
	if o == nil || o.Admit == nil {
		return handle, nil
	}
	if err := o.Admit(path, handle); err != nil {
		handle.Close()
		return nil, err
	}
	return handle, nil
}

// OpenDir opens the directory path to list it, as os.Open does, and asks
// opener's Admit about the handle (a nil opener asks nothing). Every listing
// of an image directory opens through it.
func OpenDir(opener *Opener, path string) (*os.File, error) {
	handle, err := osfile.Open(path)
	if err != nil {
		return nil, err
	}
	return opener.admitted(path, handle)
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
