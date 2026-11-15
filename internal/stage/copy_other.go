//go:build !(darwin || linux)

package stage

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// openSource opens path for reading, refusing a symlink first, since this
// platform has no flag that refuses one at open.
func openSource(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, &fs.PathError{Op: "open", Path: path, Err: notRegular{}}
	}
	return os.Open(path)
}

// createDest creates or truncates the staged copy, refusing a symlink
// there first.
func createDest(path string) (*os.File, error) {
	if info, err := os.Lstat(path); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return nil, &fs.PathError{Op: "open", Path: path, Err: notRegular{}}
	}
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
}

// preserve gives the copy the source's modification time, the one time this
// platform's standard library sets.
func preserve(_ *os.File, dest string, info fs.FileInfo) error {
	return os.Chtimes(dest, info.ModTime(), info.ModTime())
}

// PhysicalDir returns dir with every symlink resolved, or an error when it is
// not a directory.
func PhysicalDir(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", &fs.PathError{Op: "chdir", Path: dir, Err: syscall.ENOTDIR}
	}
	return resolved, nil
}
