package analyze

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The rule: timeline and fleet never write inside an input directory.
// Each path is made absolute as the command's own calls take it
// (absolute), then followed from its root as the file system follows it
// (resolve), except that on Windows no link or junction is followed. The
// output is inside when the place it leads to, or a directory above it, is
// the input: the same text, or by os.SameFile, which also matches another
// letter case on macOS or Windows. Each step up is filepath.Dir of an
// absolute path, so the text only gets shorter. Where the output's place
// cannot be told, it is refused as such; an input that is a file (an
// archive) or missing holds nothing.

// errNotFollowed is the reason a path cannot be followed past an entry:
// one that is neither a directory nor a link, or on Windows a link
// (followLink). A Windows junction is never followed: os.Lstat gives it
// neither the directory nor the link mode (fileStat.mode in
// os/types_windows.go).
var errNotFollowed = errors.New("the check enters only directories, and symbolic links outside Windows")

// errBelowFile is resolve's stop at a regular file with a name after it.
var errBelowFile = errors.New("a name follows a regular file")

// place is where resolve's path leads: its absolute path and its entry,
// nil when it does not exist yet or is a root whose entry cannot be read.
type place struct {
	path    string
	info    fs.FileInfo
	missing bool
}

// resolve follows path, already absolute, name by name: a directory is
// entered, a ".." steps up from the place reached so far, and a name not
// there yet is kept as written, as the write will create it. A link is
// replaced by the place followLink gives for it. At a regular file with a
// name after it, resolve stops with errBelowFile: the write fails there
// with "not a directory", naming the file, so the check leaves that error
// to the write.
func resolve(path string) (place, error) {
	volume := filepath.VolumeName(path)
	current := place{path: volume + string(filepath.Separator)}
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
			current = place{path: next, missing: true}
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
// from telling. directory is given as the command reads it; one that is
// missing, or reached through a link to nothing, is left to the read to
// report.
func insideDirectory(path, directory string) (bool, error) {
	target, err := resolvedAbsolute(directory)
	switch {
	case errors.Is(err, errBelowFile), errors.Is(err, fs.ErrNotExist),
		err == nil && (target.missing || target.info != nil && target.info.Mode().IsRegular()):
		return false, nil
	case err != nil:
		return false, err
	case target.info != nil && !target.info.IsDir():
		return false, &fs.PathError{Op: "resolve", Path: target.path, Err: errNotFollowed}
	}
	output, err := resolvedAbsolute(path)
	switch {
	case errors.Is(err, errBelowFile):
		return false, nil
	case err != nil:
		return false, err
	}
	for current := output.path; ; current = filepath.Dir(current) {
		if current == target.path {
			return true, nil
		}
		info, err := osfile.Lstat(current)
		root := filepath.Dir(current) == current
		switch {
		case err == nil && target.info != nil && os.SameFile(info, target.info):
			return true, nil
		case err != nil && !errors.Is(err, fs.ErrNotExist) && !root:
			return false, err
		}
		if root {
			return false, nil
		}
	}
}

// resolvedAbsolute returns where path leads, made absolute and resolved.
func resolvedAbsolute(path string) (place, error) {
	absolutePath, err := absolute(path)
	if err != nil {
		return place{}, err
	}
	return resolve(absolutePath)
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
