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
// whether it is sized as a directory. A link, never followed, is a symlink
// row with its own lstat; a real directory a dir row with every other number
// 0; anything else a file row with its lstat.
func excludedMeta(path string) (string, fsmeta.Meta, bool) {
	meta, _ := lstatOrZero(path)
	switch meta.Kind {
	case fsmeta.Symlink:
		return "symlink", meta, false
	case fsmeta.Dir:
		return "dir", fsmeta.ZeroMeta(), true
	}
	return "file", meta, false
}

// isVisibleDir reports whether path is a directory, or a link to one, that
// can be seen (test -d), the test that gives an error_read row its owner.
func isVisibleDir(check follow.Check, path string) bool {
	isDir, _ := follow.IsDirectory(check, path)
	return isDir
}
