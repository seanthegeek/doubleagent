package imagetest

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// fixture builds files in a built image for one test and takes them away
// again when the test ends, so the next test sees the image as Build made it.
// Its done gives everything it made the default modification time and puts
// back the time of each directory it made something in, so expected output
// can keep times; the times are put back again after the removal.
type fixture struct {
	test    testing.TB
	work    string
	made    []string
	parents map[string]time.Time
}

// restoreParents puts back the time of each directory the fixture made
// something in, deepest first.
func (f *fixture) restoreParents() {
	parents := make([]string, 0, len(f.parents))
	for parent := range f.parents {
		parents = append(parents, parent)
	}
	slices.Sort(parents)
	for _, parent := range slices.Backward(parents) {
		_ = setEntryTime(parent, f.parents[parent])
	}
}

// newFixture returns a fixture for the test over the image whose work
// directory is work.
func newFixture(test testing.TB, work string) *fixture {
	build := &fixture{test: test, work: work, parents: map[string]time.Time{}}
	// Registered first, so it runs last, after every removal.
	test.Cleanup(build.restoreParents)
	return build
}

// madeBelow reports whether path lies below an entry the fixture made.
func (f *fixture) madeBelow(path string) bool {
	for _, made := range f.made {
		if rest, found := strings.CutPrefix(path, made); found && strings.HasPrefix(rest, string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// noteNew records that the fixture makes path, which does not exist yet,
// and, unless the fixture made its directory too, the time that directory
// holds before that.
func (f *fixture) noteNew(path string) {
	if f.madeBelow(path) {
		return
	}
	parent := filepath.Dir(path)
	if _, seen := f.parents[parent]; !seen {
		if info, err := osfile.Lstat(parent); err == nil {
			f.parents[parent] = info.ModTime()
		}
	}
	f.made = append(f.made, path)
	f.test.Cleanup(func() { removeTree(path) })
}

// firstMissing returns the shortest prefix of path that does not exist, or ""
// when path exists.
func firstMissing(path string) string {
	missing := ""
	for current := path; ; current = filepath.Dir(current) {
		if _, err := osfile.Lstat(current); err == nil {
			return missing
		}
		missing = current
		if filepath.Dir(current) == current {
			return missing
		}
	}
}

// dir creates each directory, and removes the outermost new one at the end.
func (f *fixture) dir(paths ...string) {
	f.test.Helper()
	for _, path := range paths {
		if missing := firstMissing(path); missing != "" {
			f.noteNew(missing)
		}
		if err := osfile.MkdirAll(path, 0o755); err != nil {
			f.test.Fatal(err)
		}
	}
}

// write writes content to path without registering its removal, for a file
// below a directory the fixture already removes.
func (f *fixture) write(path, content string) {
	f.test.Helper()
	if err := osfile.WriteFile(path, []byte(content), 0o644); err != nil {
		f.test.Fatal(err)
	}
}

// file writes content to path, making its directory first, and removes what
// it made at the end.
func (f *fixture) file(path, content string) {
	f.test.Helper()
	f.dir(filepath.Dir(path))
	if firstMissing(path) != "" {
		f.noteNew(path)
	}
	f.write(path, content)
}

// touch sets the modification time of path and puts the old one back at the
// end.
func (f *fixture) touch(path string, when time.Time) {
	f.test.Helper()
	info, err := osfile.Lstat(path)
	if err != nil {
		f.test.Fatal(err)
	}
	f.test.Cleanup(func() { _ = osfile.Chtimes(path, info.ModTime(), info.ModTime()) })
	if err := osfile.Chtimes(path, when, when); err != nil {
		f.test.Fatal(err)
	}
}

// treePaths returns path and, for a plain directory (not a link or a
// junction), every path below it. A directory that cannot be listed is
// returned alone, which is why a setup calls done before it shuts one.
func treePaths(path string) []string {
	paths := []string{path}
	info, err := osfile.Lstat(path)
	if err != nil || info.Mode().Type() != fs.ModeDir {
		return paths
	}
	entries, _ := osfile.ReadDir(path)
	for _, entry := range entries {
		paths = append(paths, treePaths(filepath.Join(path, entry.Name()))...)
	}
	return paths
}

// done gives every entry the fixture made the default modification time,
// deepest first, puts back the times of the directories it made them in,
// and settles the image.
func (f *fixture) done() {
	f.test.Helper()
	var paths []string
	for _, made := range f.made {
		paths = append(paths, treePaths(made)...)
	}
	slices.Sort(paths)
	for _, path := range slices.Backward(slices.Compact(paths)) {
		if err := setEntryTime(path, DefaultModTime()); err != nil {
			f.test.Fatalf("%s: %v", path, err)
		}
	}
	f.restoreParents()
	settle(f.test, f.work)
}

// join returns the native path of the slash-separated names below base.
func join(base string, names ...string) string {
	return filepath.Join(append([]string{base}, filepath.FromSlash(filepath.Join(names...)))...)
}
