//go:build unix

package imagetest

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// removeTree deletes path, first making every directory under it listable
// and removable, since a test may leave one with mode 000.
func removeTree(path string) {
	_ = os.Chmod(path, 0o700)
	_ = filepath.WalkDir(path, func(entry string, info fs.DirEntry, err error) error {
		if info != nil && info.IsDir() {
			_ = os.Chmod(entry, 0o700)
		}
		return nil
	})
	_ = os.RemoveAll(path)
}

// symlink creates a link at path to target, making its directory first, and
// removes it at the end.
func (f *fixture) symlink(target, path string) {
	f.test.Helper()
	f.dir(filepath.Dir(path))
	f.noteNew(path)
	if err := os.Symlink(target, path); err != nil {
		f.test.Fatal(err)
	}
}

// chmod sets the mode of path and puts its old mode back at the end.
func (f *fixture) chmod(path string, mode os.FileMode) {
	f.test.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		f.test.Fatal(err)
	}
	f.test.Cleanup(func() { _ = os.Chmod(path, info.Mode().Perm()) })
	if err := os.Chmod(path, mode); err != nil {
		f.test.Fatal(err)
	}
}

// isRoot reports whether the test runs as root.
func isRoot() bool {
	return os.Geteuid() == 0
}

// requireNonRoot skips the test when it runs as root, which ignores mode 000.
func requireNonRoot(test testing.TB) {
	test.Helper()
	if isRoot() {
		test.Skip("running as root: mode 000 does not stop it")
	}
}
