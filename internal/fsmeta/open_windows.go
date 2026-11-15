package fsmeta

import (
	"io/fs"
	"os"
	"strings"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// Open opens path for reading through its own reparse point, never
// following one, shared with every other opener as Copy-FileShared shares
// (os.Open shares no delete access); the caller checks what it opened.
// Where the NTFS setting has updates off, the file is first opened for
// reading alone and that same handle is asked for its volume: a file on a
// local NTFS volume is returned on it. Otherwise the handle is closed and
// the file opened with openHolding, so a link retargeted between the two
// opens lands on a handle that holds the time. The first open reads
// nothing: Microsoft's SetFileTime page names writing, reading and running
// as what the access time records, and asks for the hold after the open.
func (o *Opener) Open(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: err}
	}
	if o.judgesVolume() {
		if handle, err := createSource(name, windows.GENERIC_READ); err == nil {
			if !holdsAccessTime(false, true, localFileSystem(handle)) {
				return os.NewFile(uintptr(handle), syspath.ToNative(path)), nil
			}
			windows.CloseHandle(handle)
		}
	}
	handle, err := o.openHolding(path, name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: err}
	}
	return os.NewFile(uintptr(handle), syspath.ToNative(path)), nil
}

// openHolding tries each access accessList gives in turn and returns the
// first handle, or the last refusal. A handle holding the right to set the
// file's times asks that reads through it not move the file's access time
// (holdOn); one opened for reading only cannot ask, and a volume that
// records access times moves it, so a read-only open taken after the open
// with the right was refused is reported under path.
func (o *Opener) openHolding(path string, name *uint16) (windows.Handle, error) {
	var handle windows.Handle
	var err error
	for i, access := range accessList(o.asksForRight()) {
		if handle, err = createSource(name, access); err != nil {
			continue
		}
		if access&windows.FILE_WRITE_ATTRIBUTES != 0 {
			return o.holdOn(path, name, handle)
		}
		if i > 0 {
			o.report(path, reasonRightRefused)
		}
		return handle, nil
	}
	return handle, err
}

// holdOn asks the file system to hold the access time of the file open on
// handle and checks that it did (verifyHold), reporting under path any
// reason the hold is not in place. Where the hold was misread, handle is
// closed and the file name names is opened again for reading only.
func (o *Opener) holdOn(path string, name *uint16, handle windows.Handle) (windows.Handle, error) {
	outcome := verifyHold(holdSteps{
		accessTime: func() (uint64, error) { return accessTimeOn(handle) },
		hold:       func() error { return HoldTimes(handle, false) },
		restore:    func(accessTime uint64) error { return setAccessTime(handle, accessTime) },
	})
	if outcome.reason != "" {
		o.report(path, outcome.reason)
	}
	if !outcome.reopen {
		return handle, nil
	}
	windows.CloseHandle(handle)
	return createSource(name, windows.GENERIC_READ)
}

// report passes path and reason to Report, when there is one.
func (o *Opener) report(path, reason string) {
	if o != nil && o.Report != nil {
		o.Report(path, reason)
	}
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
// accessTime and leaves its creation and write times alone (nil).
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
// through a network redirector (a mapped drive, a share, the loopback
// \\localhost\C$ included): the remote protocol query succeeds for such a
// file. A file whose final path cannot be read in its drive form, or reads
// as a UNC path, counts as remote too. Any doubt falls on the side of
// holding the access time.
func onRemoteVolume(handle windows.Handle) bool {
	// Words, not bytes, so the buffer has the 4-byte alignment the
	// structure's fields need.
	var protocol [remoteProtocolInfoSize / 4]uint32
	if windows.GetFileInformationByHandleEx(handle, windows.FileRemoteProtocolInfo,
		(*byte)(unsafe.Pointer(&protocol[0])), remoteProtocolInfoSize) == nil {
		return true
	}
	final, ok := finalDOSPath(handle)
	return !ok || strings.HasPrefix(final, `\\?\UNC\`)
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
