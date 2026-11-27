package imagetest

import (
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// rewriteWriteTime sets the directory's write time to the value it already
// has. osfile.Chtimes leaves a zero access time unchanged. A refusal (a
// directory with a deny rule) is left to the check.
func rewriteWriteTime(directory string) {
	if status, err := osfile.Stat(directory); err == nil {
		_ = osfile.Chtimes(directory, time.Time{}, status.ModTime())
	}
}

// treeDirectories returns root and every plain directory below it, not
// entering a link, a junction or a directory that cannot be listed.
func treeDirectories(root string) []string {
	directories := []string{root}
	entries, err := osfile.ReadDir(root)
	if err != nil {
		return directories
	}
	for _, entry := range entries {
		if entry.Type() == fs.ModeDir {
			directories = append(directories, treeDirectories(filepath.Join(root, entry.Name()))...)
		}
	}
	return directories
}

// listingTimes returns the write time of each entry of directory as its
// listing reports it.
func listingTimes(directory string) map[string]time.Time {
	times := map[string]time.Time{}
	entries, _ := osfile.ReadDir(directory)
	for _, entry := range entries {
		if status, err := entry.Info(); err == nil {
			times[entry.Name()] = status.ModTime()
		}
	}
	return times
}

// unsettledDirectories returns each directory whose entry in its parent's
// listing reports another write time than the directory itself. A
// directory that cannot be read is left to the run under test.
func unsettledDirectories(directories []string) []string {
	var unsettled []string
	listings := map[string]map[string]time.Time{}
	for _, directory := range directories {
		parent := filepath.Dir(directory)
		if _, read := listings[parent]; !read {
			listings[parent] = listingTimes(parent)
		}
		listed, ok := listings[parent][filepath.Base(directory)]
		status, err := osfile.Stat(directory)
		if ok && err == nil && !listed.Equal(status.ModTime()) {
			unsettled = append(unsettled, directory)
		}
	}
	return unsettled
}

// settle makes the tree under root quiescent, as a collected disk is,
// before a run reads it. NTFS copies a directory's newer write time
// into its parent's index entry, which a directory listing reports, when a
// handle on the directory is closed, but not always: until then a first
// listing sees the stale entry and may itself settle it, so a second
// listing reports another time. Each directory is opened and closed once, then
// each one whose listing entry still differs from the directory has its
// write time set to the value it already has, a change NTFS carries into the
// parent's entry, a few rounds at most; the test fails on a directory whose
// entry still differs. Tests that change the image call Settle again.
func settle(test testing.TB, root string) {
	test.Helper()
	directories := treeDirectories(root)
	for _, directory := range directories {
		if handle, err := osfile.Open(directory); err == nil {
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
	if unsettled = unsettledDirectories(directories); len(unsettled) != 0 {
		test.Fatalf("the listing entry of %s still holds another write time than the directory", unsettled[0])
	}
}

// Settle settles the tree under root again after a test changed it, as Build
// settles the image it makes.
func Settle(test testing.TB, root string) {
	test.Helper()
	settle(test, root)
}
