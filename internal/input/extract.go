package input

import (
	"archive/zip"
	"compress/bzip2"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The signatures a zip archive starts with: a local file header, or the
// end record of an archive with no member (the format's specification,
// APPNOTE.TXT, sections 4.3.7 and 4.3.16).
const (
	zipLocalSignature = "PK\x03\x04"
	zipEmptySignature = "PK\x05\x06"
)

// extractArchive extracts the zip or tar archive at path into destination,
// a new empty directory, and returns a problem line for each member it
// skipped. The archive's kind is told from its first bytes, never its end:
// a file that starts as a zip and that archive/zip reads is a zip, any
// other file is tried as a tar. A file that is neither, or a damaged
// archive, is an *Error; a member that cannot be written is the
// operating-system error.
func extractArchive(ctx context.Context, path, destination string) ([]string, error) {
	file, err := osfile.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: path, Err: err}
	}
	out := newExtraction(destination, path)
	magic := make([]byte, 4)
	n, _ := file.ReadAt(magic, 0)
	if start := string(magic[:n]); start == zipLocalSignature || start == zipEmptySignature {
		archive, err := zip.NewReader(file, info.Size())
		if err == nil {
			archive.RegisterDecompressor(zipBzip2Method, func(r io.Reader) io.ReadCloser {
				return io.NopCloser(bzip2.NewReader(r))
			})
			return out.problems, extractZip(ctx, archive.File, path, out)
		}
		if !errors.Is(err, zip.ErrFormat) {
			return nil, cannotExtract(path, zipReason(err))
		}
	}
	return out.problems, extractTar(ctx, file, path, out)
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
	// archive is the archive's path, which a problem names.
	archive string
	// made holds the slash paths of the directories already checked.
	made map[string]bool
	// problems holds a line for each member skipped.
	problems []string
}

// newExtraction returns an extraction of archive into root.
func newExtraction(root, archive string) *extraction {
	return &extraction{root: root, archive: archive, made: map[string]bool{}}
}

// skip records that the member called name was not extracted, and why.
func (e *extraction) skip(name, reason string) {
	e.problems = append(e.problems, e.archive+": member "+name+" not extracted: "+reason)
}

// The reasons a member is skipped for its name.
const (
	reasonLeavesDirectory = "its name leads out of the extraction directory"
	reasonNoFile          = "its name names no file"
	reasonNotAName        = "its name holds a part this system cannot use as a file name"
)

// partsReason returns the reason parts, the components of a member's name,
// cannot be extracted, or "": each must pass checkElement, the rule every
// path built from the evidence's text passes.
func partsReason(parts []string) string {
	for _, part := range parts {
		if checkElement(part) != nil {
			return reasonNotAName
		}
	}
	return ""
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
