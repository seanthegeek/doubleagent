package input

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

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

func TestPathOfFindsANameTheDiskSpellsInAnotherCase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, asked, want string
		onDisk            []string
	}{
		{"a directory on the way down", ".crush/crush.db", ".Crush/crush.db", []string{".Crush/crush.db"}},
		{"the last name", ".crush/crush.db-wal", ".crush/crush.db-WAL", []string{".crush/crush.db-WAL"}},
		{"both", ".openclaw/sessions.json", ".OPENCLAW/Sessions.json", []string{".OPENCLAW/Sessions.json"}},
		{"the exact spelling wins over a fold-equal neighbour", ".crush/a.json", ".crush/a.json",
			[]string{".crush/A.json", ".crush/a.json"}},
		{"a name of two fold-equal entries stays as built", ".crush/a.JSON", ".crush/a.JSON",
			[]string{".crush/A.json", ".crush/a.json"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			skipWhereCaseIsIgnored(t, root)
			for _, rel := range test.onDisk {
				writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), "x")
			}
			path, ok := (&Home{DiskPath: root}).PathOf(test.asked)
			wantEqual(t, "PathOf", path, filepath.Join(root, filepath.FromSlash(test.want)))
			wantEqual(t, "PathOf found", ok, true)
		})
	}
}

func TestPathOfFindsAStagedNameTheDiskSpellsInAnotherCase(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skipWhereCaseIsIgnored(t, root)
	staged := filepath.Join(root, "staged")
	writeFile(t, filepath.Join(staged, "Main", "Sessions.json"), "x")
	home := &Home{DiskPath: filepath.Join(root, "home"), StagedPaths: map[string]string{".openclaw": staged}}
	path, ok := home.PathOf(".openclaw/main/sessions.json")
	wantEqual(t, "PathOf", path, filepath.Join(staged, "Main", "Sessions.json"))
	wantEqual(t, "PathOf found", ok, true)
}

func TestALinkFoundInAnotherCaseIsRefusedNotFollowed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skipWhereCaseIsIgnored(t, root)
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "crush.db"), "x")
	home := filepath.Join(root, "home")
	writeFile(t, filepath.Join(home, "keep"), "x")
	if err := os.Symlink(outside, filepath.Join(home, ".CRUSH")); err != nil {
		t.Skip("cannot make a symbolic link here:", err)
	}
	if path, ok := (&Home{DiskPath: home}).PathOf(".crush/crush.db"); ok {
		t.Errorf("PathOf through a link found in another case = %s", path)
	}
	if path, ok := Below(home, ".crush", "crush.db"); ok {
		t.Errorf("Below through a link found in another case = %s", path)
	}
}

// countingList returns a lister that counts its listings in count.
func countingList(count *int) func(string) osfile.Spellings {
	return func(directory string) osfile.Spellings {
		*count++
		return osfile.ListSpellings(directory)
	}
}

func TestPathOfListsADirectoryOnceForManyAbsentNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".crush", "crush.db"), "x")
	listings := 0
	home := &Home{DiskPath: root, list: countingList(&listings)}
	for _, suffix := range []string{"-wal", "-journal", "-shm", "-x", "-y"} {
		home.PathOf(".crush/crush.db" + suffix)
	}
	wantEqual(t, "listings for absent names of one directory", listings, 1)
	home.PathOf(".crush/crush.db")
	home.PathOf(".crush")
	wantEqual(t, "listings once names that exist are asked too", listings, 1)
	fresh := 0
	present := &Home{DiskPath: root, list: countingList(&fresh)}
	present.PathOf(".crush/crush.db")
	wantEqual(t, "listings for a name that exists", fresh, 0)
}

func TestAManifestPathIsNeitherRespelledNorLookedAtMore(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "fs", "home", "a", "F"), "x")
	var looked []string
	collection := &Collection{Root: root, lstat: func(path string) (fs.FileInfo, error) {
		looked = append(looked, path)
		return osfile.Lstat(path)
	}}
	path, err := collection.inside("fs", "home", "a", "f")
	opened(t, err)
	wantEqual(t, "path", path, filepath.Join(root, "fs", "home", "a", "f"))
	// One look per directory on the way down, the root and the last element
	// not looked at.
	wantEqual(t, "looks", len(looked), 3)
	path, err = collection.inside("fs", "Home", "a", "f")
	opened(t, err)
	wantEqual(t, "a missing directory's path", path, filepath.Join(root, "fs", "Home", "a", "f"))
}
