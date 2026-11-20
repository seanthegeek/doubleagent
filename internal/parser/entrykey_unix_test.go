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
