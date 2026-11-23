package parserkit

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// listedParent makes a directory holding the directories "a" and "b", "a"
// holding the file "session.json" and the directory "inner", "b" holding
// "session.json" and "other.json", and the file "file.json" beside them,
// and returns it.
func listedParent(t *testing.T) Directory {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{"a/session.json", "b/session.json", "b/other.json", "file.json"} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := osfile.MkdirAll(filepath.Join(root, "a", "inner"), 0o700); err != nil {
		t.Fatal(err)
	}
	directory, ok := DirectoryAt(root)
	if !ok {
		t.Fatal("the test directory is not a plain directory")
	}
	return directory
}

func TestChildListingsFindANameByItsBytesInTheDirectoriesHoldingIt(t *testing.T) {
	children := ListChildren(listedParent(t), []string{"a", "b", "file.json", "absent", "../a"},
		nil, Work{}, "listings")
	tests := []struct {
		name, entry string
		want        []int
	}{
		{"a file of two directories is in both, in order", "session.json", []int{0, 1}},
		{"a file of one directory is in it", "other.json", []int{1}},
		{"a directory is no holder", "inner", nil},
		{"a name in another case is not found", "Session.json", nil},
		{"an absent name is not found", "absent.json", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := children.Holders(test.entry); !slices.Equal(got, test.want) {
				t.Errorf("holders %v, want %v", got, test.want)
			}
		})
	}
	if base := filepath.Base(children.Directory(1).Path()); base != "b" || len(children.Failures()) > 0 {
		t.Errorf("place 1 is %q and the failures %v, want b and none", base, children.Failures())
	}
}

func TestChildListingsFoldTheNamesFiledAndLookedUp(t *testing.T) {
	children := ListChildren(listedParent(t), []string{"a", "b"}, strings.ToLower, Work{}, "listings")
	if got := children.Holders("SESSION.JSON"); !slices.Equal(got, []int{0, 1}) {
		t.Errorf("holders %v, want [0 1]", got)
	}
}
