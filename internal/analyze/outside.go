package analyze

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/reason"
)

// The analyst's output and inputs are compared by what they name, never by
// their text alone: an existing path stands for its entry (a link to a
// directory for the directory), two paths are one file when os.SameFile
// says so, and a missing output stands for its nearest existing ancestor,
// which lies inside an input directory when it or a directory above it,
// climbed through the file system's own parent entries, is the input. A
// path is taken as it is opened: on Unix a ".." after a link leads up from
// the link's target, so the text is never cleaned (absolutePath,
// outputParent). maxClimb bounds the climb: no real path is deeper.
const maxClimb = 4096

// errClimb is the refusal of an output whose climb passed maxClimb. A
// climb that fails on the way (a path grown past the system's limit, a
// parent that cannot be read) cannot tell either, and refuses with the
// error met.
var errClimb = errors.New("the climb to the root passed its bound")

// untold returns the end of the refusal of an output whose place the climb
// could not tell: why, after the rule the refusal states.
func untold(err error) string {
	return " (where it lies could not be told: " + reason.For(err) + ")"
}

// absolutePath returns path made absolute against the working directory as
// the platform resolves it (absoluteText); "" when the working directory
// cannot be read.
func absolutePath(path string) string {
	if filepath.IsAbs(path) {
		return absoluteText(path, "")
	}
	working, err := os.Getwd()
	if err != nil {
		return ""
	}
	return absoluteText(path, working)
}

// insideDirectory reports whether path, existing or not, is the directory
// directory or lies below it, by the rule above, with the error the climb
// met, or errClimb, when it cannot tell.
func insideDirectory(path, directory string) (bool, error) {
	path, directory = absolutePath(path), absolutePath(directory)
	if path == "" || directory == "" {
		return false, nil
	}
	target, err := namedEntry(directory)
	if err != nil || !target.IsDir() {
		return false, nil
	}
	current := nearestExisting(path)
	if current == "" {
		return false, nil
	}
	info, err := namedEntry(current)
	for range maxClimb {
		if err != nil {
			return false, err
		}
		if os.SameFile(info, target) {
			return true, nil
		}
		parent := parentEntry(current)
		parentInfo, parentErr := namedEntry(parent)
		if parentErr == nil && os.SameFile(parentInfo, info) {
			return false, nil
		}
		current, info, err = parent, parentInfo, parentErr
	}
	return false, errClimb
}

// nearestExisting returns path or its nearest ancestor that exists, by
// lstat, or "" when none does; each ancestor is outputParent's.
func nearestExisting(path string) string {
	for {
		if _, err := osfile.Lstat(path); err == nil {
			return path
		}
		parent := outputParent(path)
		if parent == path || parent == "" {
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
