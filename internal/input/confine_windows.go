package input

import (
	"io/fs"
	"syscall"
)

// isLink reports whether info describes a symbolic link or any other
// reparse point: a junction, a mount point, or a file a storage driver
// stands in for, each of which can lead somewhere other than its name.
func isLink(info fs.FileInfo) bool {
	if info.Mode()&fs.ModeSymlink != 0 {
		return true
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
