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
// into its parent's index entry, which a directory listing reports, only
// when a handle on the directory is closed; until then the collector that
// runs first sees the stale entry and its own listing settles it for the
// second, so the two would report different times for one directory. Each
// directory is opened and closed once (WalkDir never enters a reparse
// point), then every directory's listing entry is checked against the
// directory itself, a few rounds at most; the test fails on a directory
// whose entry still differs.
func settleTree(t *testing.T, root string) {
	t.Helper()
	unsettled := ""
	for round := 0; round < 5; round++ {
		directories := treeDirectories(t, root)
		for _, directory := range directories {
			if handle, err := os.Open(directory); err == nil {
				_ = handle.Close()
			}
		}
		if unsettled = firstUnsettled(directories); unsettled == "" {
			return
		}
	}
	t.Fatalf("the listing entry of %s still holds an older write time than the directory", unsettled)
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

// firstUnsettled returns the first directory whose entry in its parent's
// listing reports another write time than the directory itself, or "". A
// directory that cannot be read is left to the collectors.
func firstUnsettled(directories []string) string {
	listings := map[string]map[string]time.Time{}
	for _, directory := range directories {
		parent := filepath.Dir(directory)
		if _, read := listings[parent]; !read {
			listings[parent] = listingTimes(parent)
		}
		listed, ok := listings[parent][filepath.Base(directory)]
		status, err := os.Stat(directory)
		if ok && err == nil && !listed.Equal(status.ModTime()) {
			return directory
		}
	}
	return ""
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
