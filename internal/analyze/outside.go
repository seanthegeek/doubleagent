package analyze

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The analyst's output and inputs are compared by what they name, never by
// their text alone: an existing path stands for its entry (a link to a
// directory for the directory, as the analyst chose it), and two paths are
// one file when os.SameFile says so, which also catches another spelling
// and a hard link. An output that does not exist yet stands for its nearest
// existing ancestor, and lies inside an input directory when that ancestor
// or a directory above it, climbed through the file system's own parent
// entries, is the input. maxClimb bounds the climb: no real path is deeper.
const maxClimb = 4096

// absolutePath returns path made absolute against the working directory
// and cleaned, as text; "" when the working directory cannot be read.
func absolutePath(path string) string {
	if !filepath.IsAbs(path) {
		working, err := os.Getwd()
		if err != nil {
			return ""
		}
		path = filepath.Join(working, path)
	}
	return filepath.Clean(path)
}

// insideDirectory reports whether path, existing or not, is the directory
// directory or lies below it, by the rule above.
func insideDirectory(path, directory string) bool {
	path, directory = absolutePath(path), absolutePath(directory)
	if path == "" || directory == "" {
		return false
	}
	target, err := namedEntry(directory)
	if err != nil || !target.IsDir() {
		return false
	}
	current := nearestExisting(path)
	if current == "" {
		return false
	}
	info, err := namedEntry(current)
	for range maxClimb {
		if err != nil {
			return false
		}
		if os.SameFile(info, target) {
			return true
		}
		parent := parentEntry(current)
		parentInfo, parentErr := namedEntry(parent)
		if parentErr == nil && os.SameFile(parentInfo, info) {
			return false
		}
		current, info, err = parent, parentInfo, parentErr
	}
	return false
}

// nearestExisting returns path or its nearest ancestor that exists, by
// lstat, or "" when none does.
func nearestExisting(path string) string {
	for {
		if _, err := osfile.Lstat(path); err == nil {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ""
		}
		path = parent
	}
}

// sameFile reports whether two existing paths name one file, neither
// followed when it is a link to a file.
func sameFile(first, second string) bool {
	firstInfo, firstErr := osfile.Lstat(first)
	secondInfo, secondErr := osfile.Lstat(second)
	return firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo)
}

// isLink reports whether path's own entry is a symbolic link.
func isLink(path string) bool {
	info, err := osfile.Lstat(path)
	return err == nil && info.Mode()&fs.ModeSymlink != 0
}
