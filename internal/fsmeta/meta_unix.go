//go:build unix

package fsmeta

import "strconv"

// ZeroMeta is the metadata written for a path that cannot be described: every number 0 and mode "0".
func ZeroMeta() Meta {
	return Meta{Mode: "0"}
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
