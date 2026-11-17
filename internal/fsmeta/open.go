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
	Admit          func(path string, handle uintptr) error
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

// admit asks the opener's Admit about handle, the system handle just opened
// on path, and returns its refusal; nil when there is no Admit.
func (o *Opener) admit(path string, handle uintptr) error {
	if o == nil || o.Admit == nil {
		return nil
	}
	return o.Admit(path, handle)
}

// admitted returns file, just opened on path, when the opener's Admit lets
// it through, and otherwise closes it and returns Admit's error. The handle
// is read through SyscallConn, which leaves the file's mode as it is (Fd
// would make a non-blocking Unix descriptor blocking).
func (o *Opener) admitted(path string, file *os.File) (*os.File, error) {
	if o == nil || o.Admit == nil {
		return file, nil
	}
	raw, err := file.SyscallConn()
	if err == nil {
		controlErr := raw.Control(func(handle uintptr) { err = o.admit(path, handle) })
		if err == nil {
			err = controlErr
		}
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

// sourceOpen is the order of one evidence open, kept in one place so that
// admission is always the first thing done with a new handle: create opens
// the file with an access right, admit judges the bare handle (Admit),
// release closes a refused one, and only an admitted handle is held
// (hold, for an access that carries the right to set times) or reported.
// It is compiled on every platform, so every runner tests the order.
type sourceOpen struct {
	create    func(access uint32) (uintptr, error)
	admit     func(handle uintptr) error
	release   func(handle uintptr)
	holdRight func(access uint32) bool
	hold      func(handle uintptr) string
	report    func(handle uintptr, reason string)
}

// first opens with access and admits the handle, closing a refused one.
func (steps sourceOpen) first(access uint32) (uintptr, error) {
	handle, _, err := steps.admittedOpen(access)
	return handle, err
}

// admittedOpen opens with access and admits the handle, closing a refused
// one; opened says whether the open itself succeeded, so a caller can tell
// a failed open (try the next access) from a refusal (stop).
func (steps sourceOpen) admittedOpen(access uint32) (handle uintptr, opened bool, err error) {
	handle, err = steps.create(access)
	if err != nil {
		return 0, false, err
	}
	if err := steps.admit(handle); err != nil {
		steps.release(handle)
		return 0, true, err
	}
	return handle, true, nil
}

// run tries each access in turn and returns the first admitted handle, held
// when its access carries the right (the reason, "" when held, reported)
// and reported when the right was refused; a refusal by admit ends the run,
// since every access names the same file.
func (steps sourceOpen) run(accesses []uint32) (uintptr, error) {
	var err error
	for i, access := range accesses {
		handle, opened, openErr := steps.admittedOpen(access)
		if !opened {
			err = openErr
			continue
		}
		if openErr != nil {
			return 0, openErr
		}
		if steps.holdRight(access) {
			steps.report(handle, steps.hold(handle))
		} else if i > 0 {
			steps.report(handle, reasonRightRefused)
		}
		return handle, nil
	}
	return 0, err
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
