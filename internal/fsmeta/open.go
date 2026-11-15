package fsmeta

import (
	"errors"
	"io/fs"
	"os"
)

// ErrNotRegular is the error OpenRegular wraps when path is not a regular
// file.
var ErrNotRegular = errors.New("not a regular file")

// OpenRegular opens path for reading when it is a regular file. It refuses a
// symlink, directory, pipe, socket or device by its lstat, then opens without
// following a symlink at the last component and without waiting on a pipe,
// and checks the open handle again, so a file swapped for a link or a pipe
// between the lstat and the open is refused too. Every error names path.
func OpenRegular(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, &fs.PathError{Op: "open", Path: path, Err: ErrNotRegular}
	}
	handle, err := os.OpenFile(path, os.O_RDONLY|openRegularFlags, 0)
	if err != nil {
		return nil, err
	}
	opened, err := handle.Stat()
	if err != nil || !opened.Mode().IsRegular() {
		handle.Close()
		return nil, &fs.PathError{Op: "open", Path: path, Err: ErrNotRegular}
	}
	return handle, nil
}
