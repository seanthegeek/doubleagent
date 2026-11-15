//go:build parity && windows

package parity

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// settleTree makes the tree under root quiescent, as a collected disk is,
// before a collector reads it. NTFS copies a directory's newer write time
// into its parent's index entry, which a directory listing reports, when a
// handle on the directory is closed, but not always: until then the
// collector that runs first sees the stale entry and its own listing may
// settle it for the second. Each directory is opened and closed once, then
// each one whose listing entry still differs from the directory has its
// write time set to the value it already has and is closed, a change NTFS
// carries into the parent's entry, a few rounds at most; the test fails on a
// directory whose entry still differs.
func settleTree(t *testing.T, root string) {
	t.Helper()
	directories := treeDirectories(t, root)
	for _, directory := range directories {
		if handle, err := os.Open(directory); err == nil {
			_ = handle.Close()
		}
	}
	var unsettled []string
	for round := 0; round < 5; round++ {
		if unsettled = unsettledDirectories(directories); len(unsettled) == 0 {
			return
		}
		for _, directory := range unsettled {
			rewriteWriteTime(directory)
		}
		time.Sleep(time.Duration(round+1) * 20 * time.Millisecond)
	}
	t.Fatalf("the listing entry of %s still holds another write time than the directory", unsettled[0])
}

// rewriteWriteTime sets the directory's write time to the value it already
// has. The tree is a fixture the harness built, not evidence, so writing to
// it is allowed. os.Chtimes leaves a zero access time unchanged and opens
// the path with backup semantics, following a link, which treeDirectories
// never returns: a junction or directory symlink is not a directory to
// WalkDir. A refusal (the fixtures with a deny rule) is left to the check.
func rewriteWriteTime(directory string) {
	if status, err := os.Stat(directory); err == nil {
		_ = os.Chtimes(directory, time.Time{}, status.ModTime())
	}
}

// treeDirectories returns root and every directory below it, not entering
// a reparse point.
func treeDirectories(t *testing.T, root string) []string {
	t.Helper()
	var directories []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.IsDir() {
			directories = append(directories, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return directories
}

// unsettledDirectories returns each directory whose entry in its parent's
// listing reports another write time than the directory itself. A
// directory that cannot be read is left to the collectors.
func unsettledDirectories(directories []string) []string {
	var unsettled []string
	listings := map[string]map[string]time.Time{}
	for _, directory := range directories {
		parent := filepath.Dir(directory)
		if _, read := listings[parent]; !read {
			listings[parent] = listingTimes(parent)
		}
		listed, ok := listings[parent][filepath.Base(directory)]
		status, err := os.Stat(directory)
		if ok && err == nil && !listed.Equal(status.ModTime()) {
			unsettled = append(unsettled, directory)
		}
	}
	return unsettled
}

// listingTimes returns the write time of each entry of directory as its
// listing reports it.
func listingTimes(directory string) map[string]time.Time {
	times := map[string]time.Time{}
	entries, _ := os.ReadDir(directory)
	for _, entry := range entries {
		if status, err := entry.Info(); err == nil {
			times[entry.Name()] = status.ModTime()
		}
	}
	return times
}
