package sqlitefile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// UnreadableError reports a database file or one of its sidecars that
// cannot be opened or read. Path is the file's path; Error names only its
// base name, as the analyzer's messages do.
type UnreadableError struct {
	Path string
	Err  error
}

// Error returns "<file name> could not be read (<reason>)".
func (e *UnreadableError) Error() string {
	reason := e.Err
	var pathError *fs.PathError
	if errors.As(reason, &pathError) {
		reason = pathError.Err
	}
	return fmt.Sprintf("%s could not be read (%v)", filepath.Base(e.Path), reason)
}

// Unwrap returns the underlying error.
func (e *UnreadableError) Unwrap() error {
	return e.Err
}

// errSymbolicLink is the reason a symbolic link is not read.
var errSymbolicLink = errors.New("a symbolic link, not followed")

// errNotRegular is the reason a directory or special file is not read.
var errNotRegular = errors.New("not a regular file")

// IsSQLite reports whether path is a regular file, not a symbolic link,
// whose first 16 bytes are the SQLite header string.
func IsSQLite(path string) bool {
	header, err := readHeader(path)
	return err == nil && string(header) == headerString
}

// Problem returns why OpenPath cannot open the database at path, or nil:
// ErrNotDatabase for a readable file that does not start with the SQLite
// header (a 0-byte file is an empty database, no problem), or an
// *UnreadableError for the file or the first of its -wal, -shm and -journal
// sidecars that cannot be read. A sidecar that is absent, a directory or
// another special file is skipped, as a copy of the database would leave it
// out; one that is a symbolic link is a problem, since it is never read.
func Problem(path string) error {
	header, err := readHeader(path)
	if err != nil {
		return &UnreadableError{Path: path, Err: err}
	}
	if len(header) > 0 && string(header) != headerString {
		return ErrNotDatabase
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if err := checkSidecar(path + suffix); err != nil {
			return &UnreadableError{Path: path + suffix, Err: err}
		}
	}
	return nil
}

// readHeader returns up to the first 16 bytes of the regular file at path.
func readHeader(path string) ([]byte, error) {
	if err := checkEntry(path); err != nil {
		return nil, err
	}
	file, section, err := openRegular(path, true)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	header := make([]byte, len(headerString))
	n, err := io.ReadFull(section, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return header[:n], nil
}

// checkEntry reports a path whose own entry is a symbolic link or not a
// regular file, in words that do not repeat the path.
func checkEntry(path string) error {
	info, err := osfile.Lstat(path)
	switch {
	case err != nil:
		return err
	case info.Mode()&fs.ModeSymlink != 0:
		return errSymbolicLink
	case !info.Mode().IsRegular():
		return errNotRegular
	}
	return nil
}

// checkSidecar opens a sidecar to show it can be read; an absent one, or
// one that is not a regular file or a link, is no problem.
func checkSidecar(path string) error {
	err := checkEntry(path)
	switch {
	case errors.Is(err, fs.ErrNotExist) || errors.Is(err, errNotRegular):
		return nil
	case err != nil:
		return err
	}
	file, _, err := openRegular(path, false)
	if err != nil || file == nil {
		return err
	}
	return file.Close()
}
