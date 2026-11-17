package input

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// extractArchive extracts the zip or tar archive at path into destination,
// a new empty directory. A file is a zip when it ends with a zip directory
// (isZipFile), else a tar when its first header reads. Neither is an *Error, and so is a damaged archive; a member that
// cannot be written is the operating-system error.
func extractArchive(path, destination string) error {
	file, err := osfile.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return &fs.PathError{Op: "stat", Path: path, Err: err}
	}
	if isZipFile(file, info.Size()) {
		return extractZip(file, info.Size(), path, newExtraction(destination))
	}
	return extractTar(file, path, newExtraction(destination))
}

// notAnArchive returns the error for a file that is neither archive.
func notAnArchive(path string) error {
	return &Error{Message: path + " is neither a tar nor a zip archive"}
}

// cannotExtract returns the error for an archive that is damaged or uses a
// feature the extractor refuses.
func cannotExtract(path, reason string) error {
	return &Error{Message: path + ": cannot extract: " + reason}
}

// extraction writes the members of one archive below its root. Every
// directory it makes is checked with lstat to be a real directory before a
// member goes into it, and every file is created new (O_EXCL), so no member
// is ever written through a link, whatever the earlier members were.
type extraction struct {
	root string
	// made holds the slash paths of the directories already checked.
	made map[string]bool
}

// newExtraction returns an extraction into root.
func newExtraction(root string) *extraction {
	return &extraction{root: root, made: map[string]bool{}}
}

// pathOf returns the native path of the member components parts.
func (e *extraction) pathOf(parts []string) string {
	return filepath.Join(append([]string{e.root}, parts...)...)
}

// makeDirectories makes each directory of parts that does not exist yet. A
// component that exists as anything but a real directory is an error.
func (e *extraction) makeDirectories(parts []string) error {
	for depth := 1; depth <= len(parts); depth++ {
		key := joinSlash(parts[:depth])
		if e.made[key] {
			continue
		}
		path := e.pathOf(parts[:depth])
		info, err := osfile.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			if err := osfile.MkdirAll(path, 0o777); err != nil {
				return err
			}
			info, err = osfile.Lstat(path)
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return &fs.PathError{Op: "mkdir", Path: path, Err: errNotDirectory}
		}
		e.made[key] = true
	}
	return nil
}

// writeFile creates the file parts names, its directories first, and copies
// contents into it. A file an earlier member wrote at the same path is
// replaced, as a later member replaces an earlier one; a directory there is
// an error. modified, when not zero, becomes the file's access and
// modification time.
func (e *extraction) writeFile(parts []string, perm fs.FileMode, contents io.Reader,
	modified time.Time) error {
	if len(parts) == 0 {
		return &fs.PathError{Op: "open", Path: e.root, Err: errIsDirectory}
	}
	if err := e.makeDirectories(parts[:len(parts)-1]); err != nil {
		return err
	}
	path := e.pathOf(parts)
	out, err := e.createNew(path, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, contents)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return &fs.PathError{Op: "close", Path: path, Err: closeErr}
	}
	if !modified.IsZero() {
		return osfile.Chtimes(path, modified, modified)
	}
	return nil
}

// createNew creates path exclusively, first removing a regular file there.
func (e *extraction) createNew(path string, perm fs.FileMode) (*os.File, error) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	out, err := osfile.OpenFile(path, flags, perm)
	if !errors.Is(err, fs.ErrExist) {
		return out, err
	}
	info, statErr := osfile.Lstat(path)
	if statErr != nil {
		return nil, statErr
	}
	if info.IsDir() {
		return nil, &fs.PathError{Op: "open", Path: path, Err: errIsDirectory}
	}
	if !info.Mode().IsRegular() {
		return nil, err
	}
	if err := osfile.Remove(path); err != nil {
		return nil, err
	}
	return osfile.OpenFile(path, flags, perm)
}

// setTime gives the directory parts the access and modification time
// modified.
func (e *extraction) setTime(parts []string, modified time.Time) error {
	return osfile.Chtimes(e.pathOf(parts), modified, modified)
}
