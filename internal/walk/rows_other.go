//go:build !windows

package walk

import (
	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

// inventoryLogsReadErrors says --inventory logs no read failed: line: a
// directory the inventory walk cannot list gives its find: line alone.
const inventoryLogsReadErrors = false

// collectedSize is the size a collected file's row records: its lstat size
// before the copy.
func collectedSize(file plannedFile) int64 {
	return file.meta.Size
}

// excludedMeta returns the type and metadata of an excluded path's row, and
// whether it is sized as a directory: a directory, or a link to one (test -d), with every other number 0, anything
// else as a file with its lstat. In image mode a path that leads out of the
// root is not followed: its row describes the link itself, whatever the
// link points at.
func excludedMeta(check follow.Check, path string) (string, fsmeta.Meta, bool) {
	if isDir, _ := follow.IsDirectory(check, path); isDir {
		return "dir", fsmeta.ZeroMeta(), true
	}
	meta, _ := lstatOrZero(path)
	return "file", meta, false
}

// isVisibleDir reports whether path is a directory, or a link to one, that
// can be seen (test -d), the test that gives an error_read row its owner.
func isVisibleDir(check follow.Check, path string) bool {
	isDir, _ := follow.IsDirectory(check, path)
	return isDir
}
