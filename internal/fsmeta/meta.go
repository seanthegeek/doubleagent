// Package fsmeta reads the file metadata the manifest records: on Unix the
// fields the sh collector takes from stat(1), the unreadable-directory test
// of get_dir_read_error_var and the block-count size of du -sk; on Windows
// the fields, owner, attributes and sizes the PowerShell collector reads.
//
// os.Lstat and fs.FileInfo carry a name, size, mode and modification time;
// the rest is behind Sys, in a type that differs by platform. Meta has the
// fields the manifest records on every platform: access, change and birth
// time (statx on Linux), owner and group, device, inode, link count,
// allocated blocks, the Win32 attributes and the junction flag. On Windows
// the entry is opened without following a reparse point to read them. The
// package also opens files without following a link and holds the access
// time on Windows, which os.Open does neither.
package fsmeta

import (
	"io/fs"
)

// Kind is the type of a file as lstat reports it.
type Kind int

// The kinds of file Lstat tells apart.
const (
	Other Kind = iota
	File
	Dir
	Symlink
	Socket
	Fifo
	Device
)

// Meta is what lstat says about a path itself, never about a link's target.
// Times are epoch seconds; Btime is 0 where the platform cannot report it.
type Meta struct {
	Size   int64
	Mtime  int64
	Atime  int64
	Ctime  int64
	Btime  int64
	UID    uint32
	GID    uint32
	Mode   string // permission bits with setuid, setgid and sticky, octal: "644"
	Kind   Kind
	Dev    uint64
	Ino    uint64
	Nlink  uint64
	Blocks int64 // 512-byte blocks allocated
	// Attributes holds the Win32 file attribute bits (FILE_ATTRIBUTE_*); 0 on
	// Unix.
	Attributes uint32
	// Junction is set for a Windows mount point (a junction), whose target
	// Windows does not put in place of its name when it later resolves a
	// relative symbolic link; false on Unix.
	Junction bool
}

// reparsePointAttribute is FILE_ATTRIBUTE_REPARSE_POINT, never set on Unix.
const reparsePointAttribute = 0x400

// IsLink reports whether m describes a symlink, a junction or any other
// reparse point: an entry a path may run through elsewhere, which nothing
// that must not follow a link enters.
func (m Meta) IsLink() bool {
	return m.Kind == Symlink || m.Attributes&reparsePointAttribute != 0
}

// IsOtherReparsePoint reports whether m describes a reparse point other than
// a symlink or junction (a cloud-file placeholder, a deduplicated file, an
// AF_UNIX socket, a WSL symlink, an app execution alias). Never on Unix.
func (m Meta) IsOtherReparsePoint() bool {
	return m.Kind != Symlink && m.Attributes&reparsePointAttribute != 0
}

// directoryAttribute is FILE_ATTRIBUTE_DIRECTORY, which Windows also sets on
// a symlink made to a directory and on a junction.
const directoryAttribute = 0x10

// DirectoryLink reports whether m describes a link Windows made as a
// directory link (a directory symlink or a junction), which must be recreated
// as one. It is false on Unix, where Attributes is 0.
func (m Meta) DirectoryLink() bool {
	return m.Kind == Symlink && m.Attributes&directoryAttribute != 0
}

// pathError wraps err as the *fs.PathError the Go standard library would
// return for the operation, so the manifest's error text is Go's.
func pathError(operation, path string, err error) error {
	return &fs.PathError{Op: operation, Path: path, Err: err}
}
