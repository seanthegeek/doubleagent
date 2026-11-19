package analyze

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The rule: timeline and fleet never write inside an input directory,
// known by the entry its own read reached, however its path was spelled.
// The output's path is taken as the command's calls take it (placed) and
// followed from its start as the file system follows it (resolve), except
// that on Windows no link or junction is followed. It is inside when the
// place it leads to, or a directory above it (above), is that entry
// by os.SameFile, which also matches another letter case on macOS or
// Windows; where that place cannot be told, it is refused as such. Nothing
// about the input is judged: an archive holds nothing, and an input the
// read could not reach has already stopped the command with its own error.

// errNotFollowed is the reason a path cannot be followed past an entry:
// one that is neither a directory nor a link, or on Windows a link
// (followLink). A Windows junction is never followed: os.Lstat gives it
// neither the directory nor the link mode (fileStat.mode in
// os/types_windows.go).
var errNotFollowed = errors.New("the check enters only directories, and symbolic links outside Windows")

// errBelowFile is resolve's stop at a regular file with a name after it.
var errBelowFile = errors.New("a name follows a regular file")

// place is where resolve's path leads: its path and its entry, nil when it
// does not exist yet or is a root whose entry cannot be read.
type place struct {
	path string
	info fs.FileInfo
}

// resolve follows path name by name from the root, or from the working
// directory for a relative path, which is never written in front of it:
// that could make the text longer than the system takes. A directory is
// entered, a ".." steps up from the place reached so far, and a name not
// there yet is kept as written, as the write will create it. A link is
// replaced by the place followLink gives for it. At a regular file with a
// name after it, resolve stops with errBelowFile: the write fails there
// with "not a directory", naming the file, so the check leaves that error
// to the write.
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
		case info.Mode()&fs.ModeSymlink != 0:
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

// insideDirectory reports whether path, existing or not, is the directory
// directory or lies below it, by the rule above, or the error that kept it
// from telling where path leads. directory is the input's own entry as its
// read reached it; one that is not a directory holds nothing.
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
// it could not be read), as text that names it, and whether current is
// already the top of the file system. An absolute path steps up by
// filepath.Dir, so its text only gets shorter. A relative path, which
// resolve leaves cleaned and free of links, steps up by filepath.Dir until
// only ".." names are left, then by one ".." more, until ".." is current
// itself: so a working directory longer than the system takes is climbed
// without its text.
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

// isLink reports whether path's own entry is a symbolic link.
func isLink(path string) bool {
	info, err := osfile.Lstat(path)
	return err == nil && info.Mode()&fs.ModeSymlink != 0
}
