package parser

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// listedDirectory makes a directory holding the file "session.json" and the
// directory "project", and returns it.
func listedDirectory(t *testing.T) Directory {
	t.Helper()
	root := t.TempDir()
	if err := osfile.WriteFile(filepath.Join(root, "session.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := osfile.MkdirAll(filepath.Join(root, "project"), 0o700); err != nil {
		t.Fatal(err)
	}
	directory, ok := DirectoryAt(root)
	if !ok {
		t.Fatal("the test directory is not a plain directory")
	}
	return directory
}

func TestAListingFindsANameByItsExactBytes(t *testing.T) {
	listing := ListingOf(listedDirectory(t))
	tests := []struct {
		name, entry          string
		regular, isDirectory bool
	}{
		{"a regular file is found", "session.json", true, false},
		{"a directory is found as one", "project", false, true},
		{"a name in another case is not found", "Session.json", false, false},
		{"an absent name is not found", "absent.json", false, false},
		{"a name that is not one component is not found", "../session.json", false, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, regular := listing.RegularFile(test.entry)
			_, isDirectory := listing.Subdirectory(test.entry)
			if regular != test.regular || isDirectory != test.isDirectory {
				t.Errorf("RegularFile %v and Subdirectory %v, want %v and %v", regular, isDirectory,
					test.regular, test.isDirectory)
			}
		})
	}
}
