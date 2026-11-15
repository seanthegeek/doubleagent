package fsmeta

import (
	"io/fs"
	"os"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// Open opens path for reading through its own reparse point, never
// following one, sharing it with every other opener as Copy-FileShared
// does (os.Open shares no delete access); the caller checks what
// it opened, so a symlink swapped in after its lstat is refused as not a
// regular file. Each access accessList gives is tried in turn and the last
// refusal is returned. An open holding the right to set the file's times
// asks that reads through it not move the file's access time; an open for
// reading only cannot ask, and a volume that records access times moves it.
func (o *Opener) Open(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: err}
	}
	flags := uint32(windows.FILE_FLAG_OPEN_REPARSE_POINT | windows.FILE_FLAG_SEQUENTIAL_SCAN)
	var handle windows.Handle
	for _, access := range accessList(o.asksForRight()) {
		handle, err = windows.CreateFile(name, access, shareAll, nil, windows.OPEN_EXISTING,
			flags, 0)
		if err != nil {
			continue
		}
		if access&windows.FILE_WRITE_ATTRIBUTES != 0 {
			// A file system that refuses the request still gets its file read.
			_ = HoldTimes(handle, false)
		}
		break
	}
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: err}
	}
	return os.NewFile(uintptr(handle), syspath.ToNative(path)), nil
}

// asksForRight reports whether an open asks for the right to set the file's
// times, so it can hold the access time: always for a nil opener, never
// under ReadOnly or where the NTFS setting has updates off.
func (o *Opener) asksForRight() bool {
	return o == nil || (!o.ReadOnly && !o.NTFSUpdatesOff)
}

// accessList returns the access rights Open asks for, in the order it tries
// them. With the right, the first open also asks to set the file's times,
// and where that is refused (an access list granting reading alone, a
// write-protected volume) the file is opened for reading only. Without it,
// reading alone is asked for from the start, so a program opening many
// files with a right to change them does not alarm the host's security
// software for nothing.
func accessList(withRight bool) []uint32 {
	if !withRight {
		return []uint32{windows.GENERIC_READ}
	}
	return []uint32{windows.GENERIC_READ | windows.FILE_WRITE_ATTRIBUTES, windows.GENERIC_READ}
}

// HoldTimes asks the file system not to change the file's access time, and
// its write time too when write is set, for I/O through handle until it
// closes. SetFileTime takes a time whose two halves are both 0xFFFFFFFF as
// that request; a volume with last-access updates on would otherwise record
// the reads.
func HoldTimes(handle windows.Handle, write bool) error {
	hold := windows.Filetime{LowDateTime: 0xFFFFFFFF, HighDateTime: 0xFFFFFFFF}
	var writeTime *windows.Filetime
	if write {
		writeTime = &hold
	}
	return windows.SetFileTime(handle, nil, &hold, writeTime)
}
