//go:build !windows

package walk

import (
	"os"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

// collectedSize is the size a collected file's row records: its lstat size
// before the copy, as the sh collector's stat line gives it.
func collectedSize(file plannedFile) int64 {
	return file.meta.Size
}

// excludedMeta returns the type and metadata of an excluded path's row, and
// whether it is sized as a directory, as EXCLUDED_PATHS_WORKER writes it: a
// directory, or a link to one (test -d), with every other number 0, anything
// else as a file with its lstat.
func excludedMeta(path string) (string, fsmeta.Meta, bool) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return "dir", fsmeta.ZeroMeta(), true
	}
	meta, _ := lstatOrZero(path)
	return "file", meta, false
}
