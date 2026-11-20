//go:build unix

package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeyOfNeverFollowsALink(t *testing.T) {
	directory := writeEntryFiles(t, "a.json")
	link := filepath.Join(directory, "link.json")
	if err := os.Symlink(filepath.Join(directory, "a.json"), link); err != nil {
		t.Fatal(err)
	}
	if KeyOf(link) == KeyOf(filepath.Join(directory, "a.json")) {
		t.Error("a link has the key of the file it points to")
	}
}

// A device and inode number name a file only while it exists, so two
// paths of one file that are not case spellings of each other, such as two
// hard links, keep two keys, as a new file that takes a deleted one's
// number elsewhere does.
func TestKeyOfKeepsTwoNamesOfOneFileApartUnlessTheyDifferOnlyInCase(t *testing.T) {
	directory := writeEntryFiles(t, "a.json")
	other := filepath.Join(directory, "b.json")
	if err := os.Link(filepath.Join(directory, "a.json"), other); err != nil {
		t.Fatal(err)
	}
	if KeyOf(other) == KeyOf(filepath.Join(directory, "a.json")) {
		t.Error("two links of one file share a key")
	}
}
