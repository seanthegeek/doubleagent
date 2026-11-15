//go:build parity

package parity

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixture builds files in the smoke image and takes them away again when the
// test ends, so the next case sees the image as the smoke test built it.
type fixture struct {
	t *testing.T
}

// removeTree deletes path, first making every directory under it listable
// and removable, since a case may leave one with mode 000.
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

// created registers the removal of path when the test ends.
func (f fixture) created(path string) {
	f.t.Cleanup(func() { removeTree(path) })
}

// dir creates each directory, and removes the outermost new one at the end.
func (f fixture) dir(paths ...string) {
	f.t.Helper()
	for _, path := range paths {
		if missing := firstMissing(path); missing != "" {
			f.created(missing)
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			f.t.Fatal(err)
		}
	}
}

// firstMissing returns the shortest prefix of path that does not exist, or ""
// when path exists.
func firstMissing(path string) string {
	missing := ""
	for current := path; ; current = filepath.Dir(current) {
		if _, err := os.Lstat(current); err == nil {
			return missing
		}
		missing = current
		if filepath.Dir(current) == current {
			return missing
		}
	}
}

// file writes content to path, making its directory first.
func (f fixture) file(path, content string) {
	f.t.Helper()
	f.dir(filepath.Dir(path))
	f.created(path)
	f.write(path, content)
}

// write writes content to path without registering its removal, for a file
// below a directory the fixture already removes.
func (f fixture) write(path, content string) {
	f.t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// symlink creates a link at path to target, making its directory first.
func (f fixture) symlink(target, path string) {
	f.t.Helper()
	f.dir(filepath.Dir(path))
	f.created(path)
	if err := os.Symlink(target, path); err != nil {
		f.t.Fatal(err)
	}
}

// chmod sets the mode of path and puts its old mode back at the end.
func (f fixture) chmod(path string, mode os.FileMode) {
	f.t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { _ = os.Chmod(path, info.Mode().Perm()) })
	if err := os.Chmod(path, mode); err != nil {
		f.t.Fatal(err)
	}
}

// touch sets the modification time of path and puts the old one back at the end.
func (f fixture) touch(path string, when time.Time) {
	f.t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { _ = os.Chtimes(path, info.ModTime(), info.ModTime()) })
	if err := os.Chtimes(path, when, when); err != nil {
		f.t.Fatal(err)
	}
}

// isRoot reports whether the test runs as root.
func isRoot() bool {
	return os.Geteuid() == 0
}

// requireNonRoot skips the test when it runs as root, which ignores mode 000.
func requireNonRoot(t *testing.T) {
	t.Helper()
	if isRoot() {
		t.Skip("running as root: mode 000 does not stop it")
	}
}

// join returns the path of name below base, which may use "/" separators.
func join(base string, names ...string) string {
	return filepath.Join(append([]string{base}, names...)...)
}

// nonEmptyLines returns the non-empty lines of text.
func nonEmptyLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
