package fsmeta

import (
	"io/fs"
	"os"
	"strings"
	"sync"
	"unsafe"

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
	for _, access := range accessList(o.asksForRight(path)) {
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

// asksForRight reports whether an open of path asks for the right to set the
// file's times, so it can hold the access time: always for a nil opener,
// else as holdsAccessTime decides. The file's file system is looked up
// only when the decision depends on it.
func (o *Opener) asksForRight(path string) bool {
	if o == nil {
		return true
	}
	fileSystem := ""
	if o.NTFSUpdatesOff && !o.ReadOnly && o.fileSystemOf != nil {
		fileSystem = o.fileSystemOf(path)
	}
	return holdsAccessTime(o.ReadOnly, o.NTFSUpdatesOff, fileSystem)
}

// newVolumeLookup returns a lookup of the name of the file system the file
// at path is on ("NTFS", "ReFS", "FAT32", "exFAT"), or "" when that is not
// known: the file cannot be opened, a call fails, or the file is on a remote
// volume, whose file system is the server's and which the local NTFS
// setting does not govern. It asks the file itself, through a handle opened
// with no access right (which reads nothing, so moves no access time) and
// without following a link at the last component, so a junction or symlink
// in a middle component counts for the volume it leads to. Names are kept
// for the run by volume serial number.
func newVolumeLookup() func(path string) string {
	var mutex sync.Mutex
	fileSystems := map[uint32]string{}
	return func(path string) string {
		name, err := windows.UTF16PtrFromString(syspath.Extended(path))
		if err != nil {
			return ""
		}
		handle, err := openNoFollow(name)
		if err != nil {
			return ""
		}
		defer windows.CloseHandle(handle)
		var data windows.ByHandleFileInformation
		if onRemoteVolume(handle) || windows.GetFileInformationByHandle(handle, &data) != nil {
			return ""
		}
		mutex.Lock()
		defer mutex.Unlock()
		if _, ok := fileSystems[data.VolumeSerialNumber]; !ok {
			fileSystems[data.VolumeSerialNumber] = handleFileSystem(handle)
		}
		return fileSystems[data.VolumeSerialNumber]
	}
}

// fileRemoteProtocolInfo is the FILE_REMOTE_PROTOCOL_INFO structure, which
// golang.org/x/sys does not define, laid out as Microsoft's winbase.h
// declares it; only its size and the call's success matter here.
type fileRemoteProtocolInfo struct {
	StructureVersion         uint16
	StructureSize            uint16
	Protocol                 uint32
	ProtocolMajorVersion     uint16
	ProtocolMinorVersion     uint16
	ProtocolRevision         uint16
	Reserved                 uint16
	Flags                    uint32
	GenericReserved          [8]uint32
	ProtocolSpecificReserved [16]uint32
	ProtocolSpecific         [16]uint32
}

// onRemoteVolume reports whether the file handle is open on is reached
// through a network redirector (a mapped drive, a share, the loopback
// \\localhost\C$ included): the remote protocol query succeeds for such a
// file. A file whose final path cannot be read in its drive form, or reads
// as a UNC path, counts as remote too. Any doubt falls on the side of
// holding the access time.
func onRemoteVolume(handle windows.Handle) bool {
	var protocol fileRemoteProtocolInfo
	if windows.GetFileInformationByHandleEx(handle, windows.FileRemoteProtocolInfo,
		(*byte)(unsafe.Pointer(&protocol)), uint32(unsafe.Sizeof(protocol))) == nil {
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

// handleFileSystem returns the file system name of the volume the file
// handle is open on holds, or "" when it cannot be read.
func handleFileSystem(handle windows.Handle) string {
	fileSystem := make([]uint16, windows.MAX_PATH+1)
	if windows.GetVolumeInformationByHandle(handle, nil, 0, nil, nil, nil, &fileSystem[0],
		uint32(len(fileSystem))) != nil {
		return ""
	}
	return windows.UTF16ToString(fileSystem)
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
