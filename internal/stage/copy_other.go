//go:build !(darwin || linux || windows)

package stage

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// createDest creates or truncates the staged copy, refusing a symlink
// there first.
func createDest(path string) (*os.File, error) {
	if info, err := os.Lstat(path); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return nil, &fs.PathError{Op: "open", Path: path, Err: notRegular{}}
	}
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
}

// createCopy creates the staged copy of path at dest (createDest).
func (Stager) createCopy(_ [2]uint64, dest string) (*os.File, error) {
	return createDest(dest)
}

// occupantOfPath reports false: these systems' staging directory is not
// checked for folded case.
func (Stager) occupantOfPath(string) (string, bool) {
	return "", false
}

// checksPlaces reports false: these systems' staging directory is not
// checked for folded case.
func (Stager) checksPlaces() bool {
	return false
}

// handleIdentity returns zero: no place is checked here.
func handleIdentity(*os.File, fs.FileInfo) [2]uint64 {
	return [2]uint64{}
}

// MarkCaseSensitive does nothing: these systems have no per-directory case
// setting, and whether the staging directory folds case is asked with
// FoldsCase.
func MarkCaseSensitive(string) error {
	return nil
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
