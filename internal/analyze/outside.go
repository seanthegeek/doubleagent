package analyze

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The rule: timeline and fleet never write inside an input directory, known
// by the entry its own read reached. The output's path is taken as the
// command's calls take it (placed) and followed as the file system follows
// it (resolve). It is inside when where it leads, or a directory above that,
// is that entry by os.SameFile, which also matches another letter case.
// Where that cannot be told, the output is refused. An archive holds nothing.

// errNotFollowed is resolve's stop at an entry it cannot enter.
var errNotFollowed = errors.New("the check enters only directories, and links and junctions to them")

// errBelowFile is resolve's stop at a regular file with a name after it.
var errBelowFile = errors.New("a name follows a regular file")

// place is where resolve's path leads: its path and its entry, nil when it
// does not exist yet or is a root whose entry cannot be read.
type place struct {
	path string
	info fs.FileInfo
}

// resolve follows path name by name from the root, or for a relative path
// from the working directory, whose text is never put in front: it can be
// longer than the system takes. A ".." steps up from the place reached, a
// missing name is kept as the write will create it, and a link or junction
// is replaced by where followLink says it leads. A name after a regular file
// stops with errBelowFile, leaving the write to report "not a directory".
func resolve(path string) (place, error) {
	volume := filepath.VolumeName(path)
	current := place{path: "."}
	if filepath.IsAbs(path) {
		current.path = volume + string(filepath.Separator)
	}
	if info, err := osfile.Lstat(current.path); err == nil {
		current.info = info
	}
	for name := range strings.SplitSeq(path[len(volume):], string(filepath.Separator)) {
		if name == "" || name == "." {
			continue
		}
		if current.info != nil && !current.info.IsDir() {
			if current.info.Mode().IsRegular() {
				return current, &fs.PathError{Op: "resolve", Path: current.path, Err: errBelowFile}
			}
			return current, &fs.PathError{Op: "resolve", Path: current.path, Err: errNotFollowed}
		}
		next := filepath.Join(current.path, name)
		info, err := osfile.Lstat(next)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			current = place{path: next}
			continue
		case err != nil:
			return current, err
		case info.Mode()&fs.ModeSymlink != 0 || info.Mode()&fs.ModeIrregular != 0 && !info.IsDir():
			// os.Lstat marks a Windows junction irregular, neither a
			// directory nor a link.
			if next, err = followLink(next); err != nil {
				return current, err
			}
			if info, err = osfile.Lstat(next); err != nil {
				return current, err
			}
		}
		current = place{path: next, info: info}
	}
	return current, nil
}

// insideDirectory reports whether path, existing or not, is directory, the
// input's own entry, or lies below it, or the error that kept it from
// telling. An input that is not a directory holds nothing.
func insideDirectory(path string, directory fs.FileInfo) (bool, error) {
	if directory == nil || !directory.IsDir() {
		return false, nil
	}
	placedPath, err := placed(path)
	if err != nil {
		return false, err
	}
	output, err := resolve(placedPath)
	switch {
	case errors.Is(err, errBelowFile):
		return false, nil
	case err != nil:
		return false, err
	}
	for current := output.path; ; {
		info, err := osfile.Lstat(current)
		parent, top := above(current, info)
		switch {
		case err == nil && os.SameFile(info, directory):
			return true, nil
		case err != nil && !errors.Is(err, fs.ErrNotExist) && !top:
			return false, err
		case top:
			return false, nil
		}
		current = parent
	}
}

// above returns the directory above current, whose entry is info (nil when
// unreadable), and whether current is the top of the file system. A relative
// path, which resolve leaves cleaned and free of links, steps up by name and
// then by "..", until ".." is current itself, so a working directory longer
// than the system takes is climbed without its text.
func above(current string, info fs.FileInfo) (string, bool) {
	if filepath.IsAbs(current) {
		parent := filepath.Dir(current)
		return parent, parent == current
	}
	if !onlyUp(current) {
		return filepath.Dir(current), false
	}
	parent := filepath.Join(current, "..")
	if info == nil {
		return parent, true
	}
	parentInfo, err := osfile.Lstat(parent)
	return parent, err == nil && os.SameFile(info, parentInfo)
}

// onlyUp reports whether a cleaned relative path is "." or only ".." names.
func onlyUp(path string) bool {
	for name := range strings.SplitSeq(path, string(filepath.Separator)) {
		if name != ".." {
			return path == "."
		}
	}
	return true
}

// sameFile reports whether two existing paths name one file, neither
// followed when it is a link to a file.
func sameFile(first, second string) bool {
	firstInfo, firstErr := osfile.Lstat(first)
	secondInfo, secondErr := osfile.Lstat(second)
	return firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo)
}
