package fsmeta

import (
	"io/fs"
	"os"
	"strings"
	"time"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// Open opens path for reading through its own reparse point, never
// following one, shared with every other opener as Copy-FileShared shares;
// the caller checks what it opened. Each handle is admitted (Admit) before
// anything else is done with it (sourceOpen). Where the NTFS setting has
// updates off, a file on a local NTFS volume is read through a first open
// for reading alone; otherwise the file is opened again with the right to
// hold its access time, so a link retargeted between the two opens lands on
// a handle that holds it. The first open reads nothing: Microsoft's
// SetFileTime page asks for the hold after the open.
func (o *Opener) Open(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: err}
	}
	steps := o.sourceSteps(path, name)
	if o.judgesVolume() {
		handle, err := steps.first(windows.GENERIC_READ)
		if _, failedOpen := err.(windows.Errno); err != nil && !failedOpen {
			return nil, err // refused by Admit
		}
		if err == nil {
			if !holdsAccessTime(false, true, localFileSystem(windows.Handle(handle))) {
				return os.NewFile(handle, syspath.ToNative(path)), nil
			}
			windows.CloseHandle(windows.Handle(handle))
		}
	}
	handle, err := steps.run(accessList(o.asksForRight()))
	if errno, isErrno := err.(windows.Errno); isErrno {
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: errno}
	}
	if err != nil {
		return nil, err
	}
	return os.NewFile(handle, syspath.ToNative(path)), nil
}

// sourceSteps returns the steps of an evidence open of path, named name in
// the form CreateFile takes.
func (o *Opener) sourceSteps(path string, name *uint16) sourceOpen {
	return sourceOpen{
		create: func(access uint32) (uintptr, error) {
			handle, err := createSource(name, access)
			return uintptr(handle), err
		},
		admit:     func(handle uintptr) error { return o.admit(path, handle) },
		release:   func(handle uintptr) { windows.CloseHandle(windows.Handle(handle)) },
		holdRight: func(access uint32) bool { return access&windows.FILE_WRITE_ATTRIBUTES != 0 },
		hold:      func(handle uintptr) string { return o.holdOn(windows.Handle(handle)) },
		report: func(handle uintptr, reason string) {
			o.report(path, windows.Handle(handle), reason)
		},
	}
}

// holdOn asks the file system to hold the access time of the file open on
// handle, checks that it did (verifyHold) and returns the reason the hold
// is not in place, or "". The file is read through handle whatever the
// outcome: a misread hold has nothing left to update once set back.
func (o *Opener) holdOn(handle windows.Handle) string {
	return verifyHold(holdSteps{
		accessTime: func() (uint64, error) { return accessTimeOn(handle) },
		hold:       func() error { return HoldTimes(handle, false) },
		restore:    func(accessTime uint64) error { return setAccessTime(handle, accessTime) },
		now:        currentFiletime,
	})
}

// report passes path and reason to Report, when there is one and the
// reason is reported on the volume of the file open on handle
// (reportsOnVolume).
func (o *Opener) report(path string, handle windows.Handle, reason string) {
	if o == nil || o.Report == nil {
		return
	}
	if reportsOnVolume(reason, func() bool { return onReadOnlyVolume(handle) }) {
		o.Report(path, reason)
	}
}

// onReadOnlyVolume reports whether handle's file lies on a local volume
// that is read-only (FILE_READ_ONLY_VOLUME), where no read can move an
// access time; a volume with no drive letter, such as a shadow copy, counts.
// A volume reached through a network redirector (throughRedirector) never
// counts: its flags are the server's description of the share, and the
// server's own disk may still record the read. A volume whose flags cannot
// be read counts as writable.
func onReadOnlyVolume(handle windows.Handle) bool {
	var flags uint32
	err := windows.GetVolumeInformationByHandle(handle, nil, 0, nil, nil, &flags, nil, 0)
	return err == nil && flags&windows.FILE_READ_ONLY_VOLUME != 0 && !throughRedirector(handle)
}

// currentFiletime reads this machine's clock as the count a FILETIME holds.
func currentFiletime() uint64 {
	now := windows.NsecToFiletime(time.Now().UnixNano())
	return uint64(now.HighDateTime)<<32 | uint64(now.LowDateTime)
}

// accessTimeOn reads the last-access time of the file open on handle, as
// the 64-bit count a FILETIME holds.
func accessTimeOn(handle windows.Handle) (uint64, error) {
	var data windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &data); err != nil {
		return 0, err
	}
	accessed := data.LastAccessTime
	return uint64(accessed.HighDateTime)<<32 | uint64(accessed.LowDateTime), nil
}

// setAccessTime sets the last-access time of the file open on handle to
// accessTime, passing no creation or write time (nil). It may also update
// the file's change time (setting the value a file already had did on one
// Windows machine and not on another) and may add a change-journal record.
func setAccessTime(handle windows.Handle, accessTime uint64) error {
	value := windows.Filetime{LowDateTime: uint32(accessTime), HighDateTime: uint32(accessTime >> 32)}
	return windows.SetFileTime(handle, nil, &value, nil)
}

// createSource opens the file name names with access, through its own
// reparse point, shared with every other opener, for sequential reading.
func createSource(name *uint16, access uint32) (windows.Handle, error) {
	flags := uint32(windows.FILE_FLAG_OPEN_REPARSE_POINT | windows.FILE_FLAG_SEQUENTIAL_SCAN)
	return windows.CreateFile(name, access, shareAll, nil, windows.OPEN_EXISTING, flags, 0)
}

// judgesVolume reports whether the open depends on the file's volume: the
// NTFS setting has updates off and --no-preserve-atime was not given.
func (o *Opener) judgesVolume() bool {
	return o != nil && o.NTFSUpdatesOff && !o.ReadOnly
}

// asksForRight reports whether openHolding asks for the right to set the
// file's times, so it can hold the access time: always for a nil opener,
// never under --no-preserve-atime.
func (o *Opener) asksForRight() bool {
	return o == nil || !o.ReadOnly
}

// localFileSystem returns the name of the file system of the volume the
// file handle is open on ("NTFS", "ReFS", "FAT32", "exFAT"), or "" when it
// is on a remote volume, whose file system is the server's and which the
// local setting does not govern, or when it cannot be read.
func localFileSystem(handle windows.Handle) string {
	if onRemoteVolume(handle) {
		return ""
	}
	fileSystem := make([]uint16, windows.MAX_PATH+1)
	if windows.GetVolumeInformationByHandle(handle, nil, 0, nil, nil, nil, &fileSystem[0],
		uint32(len(fileSystem))) != nil {
		return ""
	}
	return windows.UTF16ToString(fileSystem)
}

// remoteProtocolInfoSize is the size in bytes of the buffer given to the
// FileRemoteProtocolInfo query, a multiple of 4. golang.org/x/sys does not
// define FILE_REMOTE_PROTOCOL_INFO. Microsoft's documentation page declares
// it with both a 64-byte reserved block and a 64-byte protocol-specific
// block, 180 bytes in all; a layout with only one of the two is 116 bytes.
// The buffer is at least as large as either. Only the query's success is
// used.
const remoteProtocolInfoSize = 180

// onRemoteVolume reports whether the file handle is open on is reached
// through a network redirector (throughRedirector), or its final path
// cannot be read in its drive form or reads as a UNC path. Any doubt falls
// on the side of holding the access time.
func onRemoteVolume(handle windows.Handle) bool {
	if throughRedirector(handle) {
		return true
	}
	final, ok := finalDOSPath(handle)
	return !ok || strings.HasPrefix(final, `\\?\UNC\`)
}

// throughRedirector reports whether the file handle is open on is reached
// through a network redirector (a mapped drive, a share, the loopback
// \\localhost\C$ included): the remote protocol query succeeds for such a
// file and fails for one on a local volume.
func throughRedirector(handle windows.Handle) bool {
	// Words, not bytes, so the buffer has the 4-byte alignment the
	// structure's fields need.
	var protocol [remoteProtocolInfoSize / 4]uint32
	return windows.GetFileInformationByHandleEx(handle, windows.FileRemoteProtocolInfo,
		(*byte)(unsafe.Pointer(&protocol[0])), remoteProtocolInfoSize) == nil
}

// finalDOSPath returns the path Windows holds for the file handle is open
// on, in the drive-letter or UNC form (flags 0, VOLUME_NAME_DOS), growing
// the buffer to the length the first call asks for.
func finalDOSPath(handle windows.Handle) (string, bool) {
	size := uint32(windows.MAX_PATH)
	for {
		buffer := make([]uint16, size)
		n, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], size, 0)
		if err != nil {
			return "", false
		}
		if n < size {
			return windows.UTF16ToString(buffer[:n]), true
		}
		size = n
	}
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

// admitFile asks Admit about file, just opened on path, closing it when
// refused; it is admitted's check for a caller that keeps its own handle.
func (o *Opener) admitFile(path string, file *os.File) error {
	_, err := o.admitted(path, file)
	return err
}
