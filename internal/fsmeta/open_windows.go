package fsmeta

import (
	"io/fs"
	"os"
	"strings"

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
	handle, err := o.openHolding(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: err}
	}
	return os.NewFile(uintptr(handle), syspath.ToNative(path)), nil
}

// openHolding tries each access accessList gives in turn and returns the
// first handle, or the last refusal. A handle holding the right to set the
// file's times asks that reads through it not move the file's access time;
// one opened for reading only cannot ask, and a volume that records access
// times moves it.
func (o *Opener) openHolding(name *uint16) (windows.Handle, error) {
	var handle windows.Handle
	var err error
	for _, access := range accessList(o.asksForRight()) {
		if handle, err = createSource(name, access); err != nil {
			continue
		}
		if access&windows.FILE_WRITE_ATTRIBUTES != 0 {
			// A file system that refuses the request still gets its file read.
			_ = HoldTimes(handle, false)
		}
		return handle, nil
	}
	return handle, err
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

// remoteProtocolInfoSize is the size of the buffer given to the
// FileRemoteProtocolInfo query: at least as large as FILE_REMOTE_PROTOCOL_INFO
// in every layout Microsoft has published (180 bytes as its documentation
// declares the structure; golang.org/x/sys does not define it). Only the
// query's success is used.
const remoteProtocolInfoSize = 180

// onRemoteVolume reports whether the file handle is open on is reached
// through a network redirector (a mapped drive, a share, the loopback
// \\localhost\C$ included): the remote protocol query succeeds for such a
// file. A file whose final path cannot be read in its drive form, or reads
// as a UNC path, counts as remote too. Any doubt falls on the side of
// holding the access time.
func onRemoteVolume(handle windows.Handle) bool {
	var protocol [remoteProtocolInfoSize]byte
	if windows.GetFileInformationByHandleEx(handle, windows.FileRemoteProtocolInfo,
		&protocol[0], uint32(len(protocol))) == nil {
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
