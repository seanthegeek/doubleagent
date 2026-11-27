package parserkit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// skipWhereCaseIsIgnored skips the test when dir's file system takes two
// names that differ only in ASCII case for one, where a name found in
// another case is found as built and the test proves nothing. It is
// ignoresCase of internal/walk's tests, which this package cannot import.
func skipWhereCaseIsIgnored(t *testing.T, dir string) {
	t.Helper()
	probe := filepath.Join(dir, "case-probe")
	if err := os.WriteFile(probe, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(probe)
	if _, err := os.Lstat(filepath.Join(dir, "CASE-PROBE")); err == nil {
		t.Skip("this disk ignores letter case, so a name in another case is found as built")
	}
}

// writeEmpty makes an empty file at each rel below root, with its
// directories.
func writeEmpty(t *testing.T, root string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTheKitsPathsFindANameTheDiskSpellsInAnotherCase(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skipWhereCaseIsIgnored(t, root)
	home := filepath.Join(root, "home")
	writeEmpty(t, home, ".openclaw/Sessions/s.jsonl", ".openclaw/Sessions/Sessions.json",
		".openclaw/Index.json", ".crush/crush.db", ".crush/crush.db-WAL")
	artifact := homeArtifact(root, ".openclaw/Sessions/s.jsonl", "home/.openclaw/Sessions/s.jsonl", nil)
	loose := &input.Artifact{Rel: artifact.Rel, DiskPath: artifact.DiskPath}
	sessions := filepath.Join(home, ".openclaw", "Sessions")
	database := filepath.Join(home, ".crush", "crush.db")
	tests := []struct {
		name string
		call func() (string, bool)
		want string
	}{
		{"Beside", func() (string, bool) { return Beside(artifact, "sessions.json") },
			filepath.Join(sessions, "Sessions.json")},
		{"Above", func() (string, bool) { return Above(artifact, 1, "index.json") },
			filepath.Join(home, ".openclaw", "Index.json")},
		{"Above without a home", func() (string, bool) { return Above(loose, 1, "index.json") },
			filepath.Join(home, ".openclaw", "Index.json")},
		{"Under, a directory and the last name", func() (string, bool) {
			return Under(filepath.Join(home, ".openclaw"), "sessions", "sessions.json")
		}, filepath.Join(sessions, "Sessions.json")},
		{"Sidecar", func() (string, bool) { return Sidecar(database, "-wal") }, database + "-WAL"},
		{"Sidecar of a file absent in every case", func() (string, bool) { return Sidecar(database, "-journal") },
			database + "-journal"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got, ok := test.call(); got != test.want || !ok {
				t.Errorf("%s = %s, %v; want %s", test.name, got, ok, test.want)
			}
		})
	}
}

func TestUnderRefusesALinkFoundInAnotherCase(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skipWhereCaseIsIgnored(t, root)
	outside := t.TempDir()
	writeEmpty(t, outside, "crush.db")
	if err := os.Symlink(outside, filepath.Join(root, ".CRUSH")); err != nil {
		t.Skip("cannot make a symbolic link here:", err)
	}
	if path, ok := Under(root, ".crush", "crush.db"); ok {
		t.Errorf("Under through a link found in another case = %s", path)
	}
}

func TestADirectorysChildIsSpelledAsItsListingSpellsIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeEmpty(t, root, "Sessions.json", "été.json")
	directory, ok := DirectoryAt(root)
	if !ok {
		t.Fatal("a real directory was refused")
	}
	for _, test := range []struct{ child, want string }{
		{"sessions.json", "Sessions.json"},
		{"Sessions.json", "Sessions.json"},
		{"absent.json", "absent.json"},
		{"ÉTÉ.json", "ÉTÉ.json"},
	} {
		if got, _ := directory.Child(test.child); got != filepath.Join(root, test.want) {
			t.Errorf("Child(%s) = %s, want %s", test.child, got, filepath.Join(root, test.want))
		}
	}
}

func TestADirectorysChildOfTwoFoldEqualEntriesIsAsBuilt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skipWhereCaseIsIgnored(t, root)
	writeEmpty(t, root, "A.json", "a.json")
	directory, _ := DirectoryAt(root)
	for _, child := range []string{"a.json", "A.json", "a.JSON"} {
		if got, _ := directory.Child(child); got != filepath.Join(root, child) {
			t.Errorf("Child(%s) = %s", child, got)
		}
	}
}

func TestADirectoryThatCannotBeListedGivesChildrenAsBuilt(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "gone")
	writeEmpty(t, root, "Sessions.json")
	directory, ok := DirectoryAt(root)
	if !ok {
		t.Fatal("a real directory was refused")
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if got, _ := directory.Child("sessions.json"); got != filepath.Join(root, "sessions.json") {
		t.Errorf("Child of a directory that cannot be listed = %s", got)
	}
}

func TestADirectoryIsListedOnceForAllItsChildren(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeEmpty(t, root, "present.json")
	directory, _ := DirectoryAt(root)
	listings := 0
	directory.names.list = func(path string) osfile.Spellings {
		listings++
		return osfile.ListSpellings(path)
	}
	copied := directory
	for _, child := range []string{"present.json", "a.json", "b.json", "c-wal", "d-journal"} {
		directory.Child(child)
		copied.Child(child)
	}
	if listings != 1 {
		t.Errorf("listings for ten children of one directory = %d, want 1", listings)
	}
}

func TestAListedChildDirectoryIsNotListedAgainForItsChildren(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeEmpty(t, root, "one/Sessions.json")
	parent, _ := DirectoryAt(root)
	children := ListChildren(parent, []string{"one"}, nil, Work{}, "listings")
	child := children.Directory(children.Holders("Sessions.json")[0])
	child.names.list = func(string) osfile.Spellings {
		t.Error("a directory ListChildren listed was listed again")
		return nil
	}
	if got, _ := child.Child("sessions.json"); got != filepath.Join(root, "one", "Sessions.json") {
		t.Errorf("Child = %s", got)
	}
}
