//go:build unix

package relocation

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// physicalDirectory returns the physical path of directory ("" for the
// root), every link resolved, as `cd -P DIR && pwd -P` prints it; ok is false
// where cd would fail: no such directory, or no search permission on it.
func physicalDirectory(directory string) (physical string, ok bool) {
	if directory == "" {
		directory = "/"
	}
	physical, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(physical)
	if err != nil || !info.IsDir() || !canEnter(physical) {
		return "", false
	}
	return physical, true
}

// canEnter reports whether the effective user may search directory, as `cd`
// needs.
func canEnter(directory string) bool {
	return unix.Faccessat(unix.AT_FDCWD, directory, unix.X_OK, unix.AT_EACCESS) == nil
}
