package walk

import (
	"io/fs"
	"syscall"
)

// descends reports whether the walk lists entry as a directory: a directory
// that is no reparse point. Go types a junction as irregular, but a reparse
// point of another kind (a cloud placeholder) as a directory, and the walk
// enters no reparse point of any kind.
func descends(entry fs.DirEntry) bool {
	if !entry.IsDir() {
		return false
	}
	info, err := entry.Info()
	if err != nil {
		return false
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return !ok || data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0
}
