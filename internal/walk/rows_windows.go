package walk

import (
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"golang.org/x/sys/windows"
)

// inventoryLogsReadErrors says --inventory logs the read failed: line of a
// directory the walk cannot list: Write-ReadErrorRow logs before it returns
// under -Inventory, writing no row.
const inventoryLogsReadErrors = true

// collectedSize is the size a collected file's row records: the bytes
// copied, as Copy-FileShared returns them, since a file another process
// writes may have grown since its lstat.
func collectedSize(file plannedFile) int64 {
	return file.written
}

// excludedMeta returns the type and metadata of an excluded path's row, and
// whether it is sized as a directory, as Add-ExcludedPath writes it: the
// path's own times and attributes, as a dir when Windows marks it a
// directory (a junction or directory symlink included), else as a file.
func excludedMeta(path string) (string, fsmeta.Meta, bool) {
	meta, _ := lstatOrZero(path)
	if meta.Attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return "dir", meta, true
	}
	return "file", meta, false
}

// isVisibleDir reports whether path's own entry has the directory attribute,
// the Test-PathQuiet Container test of Write-ReadErrorRow: Directory.Exists
// reads the entry without following a link, so a junction whose target is
// gone still gets its owner, the junction's own.
func isVisibleDir(path string) bool {
	meta, ok := lstatOrZero(path)
	return ok && meta.Attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
}
