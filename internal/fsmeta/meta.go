// Package fsmeta reads the file metadata the manifest records: the fields
// the sh collector takes from stat(1), the unreadable-directory test of
// get_dir_read_error_var, and the block-count size of du -sk.
package fsmeta

import (
	"io/fs"
	"strconv"
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
}

// modeString writes mode & 07777 in octal without a leading zero.
func modeString(mode uint32) string {
	return strconv.FormatUint(uint64(mode&0o7777), 8)
}

// kindOf maps the file-type bits of an lstat mode to a Kind.
func kindOf(mode uint32) Kind {
	switch mode & 0o170000 {
	case 0o100000:
		return File
	case 0o040000:
		return Dir
	case 0o120000:
		return Symlink
	case 0o140000:
		return Socket
	case 0o010000:
		return Fifo
	case 0o060000, 0o020000:
		return Device
	}
	return Other
}

// pathError wraps err as the *fs.PathError the Go standard library would
// return for the operation, so the manifest's error text is Go's.
func pathError(op, path string, err error) error {
	return &fs.PathError{Op: op, Path: path, Err: err}
}
