//go:build !(darwin || linux || windows)

package stage

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// The binary is built for Linux, macOS and Windows only (.github/go-build.sh),
// and fsmeta has no Lstat elsewhere, so no build compiles this file today.

// createDest creates the staged copy exclusively.
func createDest(dest string) (createdFile, error) {
	file, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	return createdFile{file: file}, err
}

// handleIdentity returns zero: no place is checked here.
func handleIdentity(*os.File, fs.FileInfo) [2]uint64 {
	return [2]uint64{}
}

// MarkCaseSensitive does nothing: these systems have no per-directory case
// setting, and whether the staging directory takes two names for one is
// asked with FoldsCase.
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
