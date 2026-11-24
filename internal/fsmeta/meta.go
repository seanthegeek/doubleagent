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
	Size  int64
	Mtime int64
	Atime int64
	Ctime int64
	Btime int64
	UID   uint32
	GID   uint32
	Mode  string // permission bits with setuid, setgid and sticky, octal: "644"
	Kind  Kind
	Dev   uint64
	Ino   uint64
	Nlink uint64
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

// MarkedDirectory reports whether m describes a directory or, on Windows,
// any entry with the directory attribute: a junction, a directory symlink or
// another reparse point made as a directory, such as a cloud placeholder.
func (m Meta) MarkedDirectory() bool {
	return m.Kind == Dir || m.Attributes&directoryAttribute != 0
}

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
