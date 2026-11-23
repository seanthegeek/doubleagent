package walk

import (
	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"golang.org/x/sys/windows"
)

// inventoryLogsReadErrors says --inventory logs the read failed: line of a
// directory the walk cannot list, writing no row.
const inventoryLogsReadErrors = true

// collectedSize is the size a collected file's row records: the bytes
// copied, since a file another process
// writes may have grown since its lstat.
func collectedSize(file plannedFile) int64 {
	return file.written
}

// excludedMeta returns the type and metadata of an excluded path's row, and
// whether it is sized as a directory. A link (a symlink or junction), never
// followed, is a symlink row with its own lstat; another path Windows marks
// a directory is a dir row with its own times and attributes; anything else
// a file row.
func excludedMeta(path string) (string, fsmeta.Meta, bool) {
	meta, _ := lstatOrZero(path)
	if meta.Kind == fsmeta.Symlink {
		return "symlink", meta, false
	}
	if meta.Attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return "dir", meta, true
	}
	return "file", meta, false
}

// isVisibleDir reports whether path's own entry has the directory attribute:
// Directory.Exists
// reads the entry without following a link, so a junction whose target is
// gone still gets its owner, the junction's own.
func isVisibleDir(_ follow.Check, path string) bool {
	meta, ok := lstatOrZero(path)
	return ok && meta.Attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
}
