package fsmeta

import (
	"strings"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// shareAll lets another process read, write, rename or delete the file
// while a handle of ours is open, so opening it never fails on sharing.
const shareAll = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE

// noFollowFlags open a directory as well as a file, and open a reparse
// point (symlink, junction, socket) itself rather than what it points to.
const noFollowFlags = windows.FILE_FLAG_BACKUP_SEMANTICS | windows.FILE_FLAG_OPEN_REPARSE_POINT

// fileAttributeTagInfo is the FILE_ATTRIBUTE_TAG_INFO structure, which
// golang.org/x/sys does not define: the attributes and the reparse tag.
type fileAttributeTagInfo struct {
	FileAttributes uint32
	ReparseTag     uint32
}

// ZeroMeta is the metadata written for a path that cannot be described:
// every number 0 and mode empty, as the PowerShell collector writes mode.
func ZeroMeta() Meta {
	return Meta{}
}

// Lstat returns the metadata of path itself, never of a link's or
// junction's target. Size, times, attributes and the reparse tag come from
// the directory entry, the data Get-ChildItem and every Windows listing
// show: NTFS updates a directory's own record before the entry its parent
// holds, so a handle can report a newer write time than the listing. The
// file identity (Dev, Ino, Nlink) is only on the file's record and comes
// from a handle opened with no access right, which Windows grants where
// reading the content is denied; it is 0 when the open fails (a file the
// system holds open exclusively).
func Lstat(path string) (Meta, error) {
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		return Meta{}, pathError("lstat", syspath.ToNative(path), err)
	}
	if meta, err := lstatFind(path, name); err == nil {
		addIdentity(&meta, name)
		return meta, nil
	}
	// No directory lists a drive or share root, so FindFirstFile has no entry
	// for one; it also fails on a file whose directory the account may not
	// list but whose file it may open. The handle describes those.
	meta, err := lstatHandle(name)
	if err != nil {
		return Meta{}, pathError("lstat", syspath.ToNative(path), err)
	}
	return meta, nil
}

// openNoFollow opens the file name names itself, a directory or a reparse
// point included, with no access right.
func openNoFollow(name *uint16) (windows.Handle, error) {
	return windows.CreateFile(name, 0, shareAll, nil, windows.OPEN_EXISTING, noFollowFlags, 0)
}

// lstatHandle describes the file name names through a handle on it, with
// the volume serial number and file index that identify the file.
func lstatHandle(name *uint16) (Meta, error) {
	handle, err := openNoFollow(name)
	if err != nil {
		return Meta{}, err
	}
	defer windows.CloseHandle(handle)
	var data windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &data); err != nil {
		return Meta{}, err
	}
	var tag fileAttributeTagInfo
	if data.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		err := windows.GetFileInformationByHandleEx(handle, windows.FileAttributeTagInfo,
			(*byte)(unsafe.Pointer(&tag)), uint32(unsafe.Sizeof(tag)))
		if err != nil {
			return Meta{}, err
		}
	}
	size := int64(data.FileSizeHigh)<<32 | int64(data.FileSizeLow)
	meta := metaOf(data.FileAttributes, tag.ReparseTag, size,
		[3]windows.Filetime{data.LastWriteTime, data.LastAccessTime, data.CreationTime})
	setIdentity(&meta, &data)
	return meta, nil
}

// addIdentity sets the identity of meta from a handle on the file name
// names, leaving it 0 when the file cannot be opened or read.
func addIdentity(meta *Meta, name *uint16) {
	handle, err := openNoFollow(name)
	if err != nil {
		return
	}
	defer windows.CloseHandle(handle)
	var data windows.ByHandleFileInformation
	if windows.GetFileInformationByHandle(handle, &data) == nil {
		setIdentity(meta, &data)
	}
}

// setIdentity copies the volume serial number, file index and link count
// of data into meta as Dev, Ino and Nlink.
func setIdentity(meta *Meta, data *windows.ByHandleFileInformation) {
	meta.Dev = uint64(data.VolumeSerialNumber)
	meta.Ino = uint64(data.FileIndexHigh)<<32 | uint64(data.FileIndexLow)
	meta.Nlink = uint64(data.NumberOfLinks)
}

// lstatFind describes the file at path, whose extended form name names,
// from its directory entry, which carries no file identity. For a reparse
// point the entry's first reserved field holds the reparse tag. FindFirstFile
// reads its last component as a pattern, so a name holding one of the
// characters the directory query matches with (no Win32 file name holds
// one) is refused rather than matched against another entry.
func lstatFind(path string, name *uint16) (Meta, error) {
	if strings.ContainsAny(path[strings.LastIndexAny(path, `/\`)+1:], `*?<>"`) {
		return Meta{}, windows.ERROR_INVALID_NAME
	}
	var data windows.Win32finddata
	handle, err := windows.FindFirstFile(name, &data)
	if err != nil {
		return Meta{}, err
	}
	_ = windows.FindClose(handle)
	var tag uint32
	if data.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		tag = data.Reserved0
	}
	size := int64(data.FileSizeHigh)<<32 | int64(data.FileSizeLow)
	return metaOf(data.FileAttributes, tag, size,
		[3]windows.Filetime{data.LastWriteTime, data.LastAccessTime, data.CreationTime}), nil
}

// metaOf builds the metadata of a file from its attribute bits, reparse tag,
// size and its write, access and creation times, in that order. Windows has
// no change time, owner number or permission bits, so Ctime, UID and GID are
// 0 and Mode is empty, as the PowerShell collector writes them.
func metaOf(attributes, tag uint32, size int64, times [3]windows.Filetime) Meta {
	return Meta{
		Size:       size,
		Mtime:      epochSeconds(times[0]),
		Atime:      epochSeconds(times[1]),
		Btime:      epochSeconds(times[2]),
		Kind:       kindOfAttributes(attributes, tag),
		Attributes: attributes,
	}
}

// kindOfAttributes maps attribute bits and a reparse tag to a Kind: a
// symlink or a junction is a Symlink, as the PowerShell collector's LinkType
// test makes both one; any other reparse point (an AF_UNIX socket, an app
// execution alias, a cloud placeholder) is Other and never descended.
func kindOfAttributes(attributes, tag uint32) Kind {
	switch {
	case attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0:
	case tag == windows.IO_REPARSE_TAG_SYMLINK || tag == windows.IO_REPARSE_TAG_MOUNT_POINT:
		return Symlink
	default:
		return Other
	}
	if attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return Dir
	}
	return File
}

// epochSeconds converts a file time, 100-nanosecond intervals since 1601,
// to Unix seconds. It divides in those intervals because Filetime's
// Nanoseconds overflows for times after 2262.
func epochSeconds(time windows.Filetime) int64 {
	intervals := int64(time.HighDateTime)<<32 | int64(time.LowDateTime)
	return (intervals - 116444736000000000) / 10000000
}
