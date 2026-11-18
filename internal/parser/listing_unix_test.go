//go:build unix

package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAListingAnswersAsALookPerNameWouldAtLinksAndUnlistedDirectories(t *testing.T) {
	directory := listedDirectory(t)
	target := filepath.Join(t.TempDir(), "elsewhere.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(directory.Path(), "linked.json")); err != nil {
		t.Fatal(err)
	}
	if _, ok := ListingOf(directory).RegularFile("linked.json"); ok {
		t.Error("a link to a regular file was taken for one")
	}
	// A directory that may be searched but not read cannot be listed; each
	// name is then looked at on its own, as before.
	if err := os.Chmod(directory.Path(), 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory.Path(), 0o700) })
	if _, err := os.ReadDir(directory.Path()); err == nil {
		t.Skip("the directory can still be listed, as it can by the superuser")
	}
	listing := ListingOf(directory)
	if _, ok := listing.RegularFile("session.json"); !ok {
		t.Error("a file of a directory that cannot be listed was not found")
	}
	if _, ok := listing.Subdirectory("project"); !ok {
		t.Error("a directory of a directory that cannot be listed was not found")
	}
}
