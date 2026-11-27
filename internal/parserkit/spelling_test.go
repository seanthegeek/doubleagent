package parserkit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
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
			return Under(artifact, filepath.Join(home, ".openclaw"), "sessions", "sessions.json")
		}, filepath.Join(sessions, "Sessions.json")},
		{"Under without a home", func() (string, bool) {
			return Under(loose, filepath.Join(home, ".openclaw"), "sessions", "sessions.json")
		}, filepath.Join(sessions, "Sessions.json")},
		{"Sidecar", func() (string, bool) { return Sidecar(artifact, database, "-wal") }, database + "-WAL"},
		{"Sidecar without a home", func() (string, bool) { return Sidecar(loose, database, "-wal") },
			database + "-WAL"},
		{"Sidecar of a file absent in every case", func() (string, bool) {
			return Sidecar(artifact, database, "-journal")
		}, database + "-journal"},
	}
	// The cases share one home, whose kept listings no lock guards: they
	// run one after another, as a parse does.
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
	if path, ok := Under(&input.Artifact{}, root, ".crush", "crush.db"); ok {
		t.Errorf("Under through a link found in another case = %s", path)
	}
}

func TestADirectorysChildIsSpelledAsItsListingSpellsIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeEmpty(t, root, "Sessions.json", "\u00e9t\u00e9.json")
	for _, artifact := range []*input.Artifact{{}, {Home: &input.Home{DiskPath: root}}} {
		directory, ok := DirectoryAt(artifact, root)
		if !ok {
			t.Fatal("a real directory was refused")
		}
		for _, test := range []struct{ child, want string }{
			{"sessions.json", "Sessions.json"},
			{"Sessions.json", "Sessions.json"},
			{"absent.json", "absent.json"},
			{"\u00c9T\u00c9.json", "\u00c9T\u00c9.json"},
		} {
			if got, _ := directory.Child(test.child); got != filepath.Join(root, test.want) {
				t.Errorf("Child(%s) = %s, want %s", test.child, got, filepath.Join(root, test.want))
			}
		}
	}
}

func TestADirectorysChildOfTwoFoldEqualEntriesIsAsBuilt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skipWhereCaseIsIgnored(t, root)
	writeEmpty(t, root, "A.json", "a.json")
	directory, _ := DirectoryAt(&input.Artifact{}, root)
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
	directory, ok := DirectoryAt(&input.Artifact{Home: &input.Home{DiskPath: root}}, root)
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

// homeArtifacts returns count artifacts of one home at root, as the
// artifacts of one run's directory share it.
func homeArtifacts(root string, count int) []*input.Artifact {
	home := &input.Home{DiskPath: root}
	artifacts := make([]*input.Artifact, count)
	for i := range artifacts {
		artifacts[i] = &input.Artifact{Home: home, Rel: "f", DiskPath: filepath.Join(root, "f")}
	}
	return artifacts
}

// The kept listing is seen by its staleness: a name added after the
// directory was first listed is not found in another case, as a second
// listing would find it.
func TestEveryArtifactOfAHomeDirectoryAtAndChildListsADirectoryOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeEmpty(t, root, "present.json")
	artifacts := homeArtifacts(root, 2)
	first, _ := DirectoryAt(artifacts[0], root)
	first.Child("absent.json")
	writeEmpty(t, root, "Later.json")
	second, _ := DirectoryAt(artifacts[1], root)
	if got, _ := second.Child("later.json"); got != filepath.Join(root, "later.json") {
		t.Errorf("a second artifact's Directory listed the directory again: Child = %s", got)
	}
	fresh, _ := DirectoryAt(&input.Artifact{Home: &input.Home{DiskPath: root}}, root)
	if got, _ := fresh.Child("later.json"); got != filepath.Join(root, "Later.json") {
		t.Errorf("another home's Directory did not list the directory: Child = %s", got)
	}
}

func TestEveryRouteOfAHomeListsADirectoryOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skipWhereCaseIsIgnored(t, root)
	writeEmpty(t, root, "crush.db")
	artifacts := homeArtifacts(root, 4)
	Under(artifacts[0], root, "absent.json")
	writeEmpty(t, root, "SESSIONS.JSON", "crush.db-WAL", "INDEX.JSON")
	directory, _ := DirectoryAt(artifacts[1], root)
	child, _ := directory.Child("sessions.json")
	sidecar, _ := Sidecar(artifacts[2], filepath.Join(root, "crush.db"), "-wal")
	under, _ := Under(artifacts[3], root, "index.json")
	for _, got := range []string{child, sidecar, under} {
		if filepath.Base(got) != textlines.LowerASCII(filepath.Base(got)) {
			t.Errorf("a route listed the directory again: %s", got)
		}
	}
}

func TestEveryDatabaseOfAHomeListsItsDirectoryOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skipWhereCaseIsIgnored(t, root)
	path := filepath.Join(root, "log_rows.db")
	if err := osfile.WriteFile(path, fixtureBytes(t, "log_rows.db", false), 0o600); err != nil {
		t.Fatal(err)
	}
	rowCount := func(artifact *input.Artifact) int {
		database, err := OpenDatabase(artifact, path)
		if err != nil {
			t.Fatal(err)
		}
		defer database.Close()
		table, err := database.Table("t")
		if err != nil {
			t.Fatal(err)
		}
		rows, err := table.Select().All()
		if err != nil {
			t.Fatal(err)
		}
		return len(rows)
	}
	artifacts := homeArtifacts(root, 2)
	alone := rowCount(artifacts[0])
	if err := osfile.WriteFile(path+"-WAL", fixtureBytes(t, "log_rows.db-wal", false), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := rowCount(artifacts[1]); got != alone {
		t.Errorf("a second database open listed the directory again: %d rows, want %d", got, alone)
	}
	if got := rowCount(&input.Artifact{Home: &input.Home{DiskPath: root}}); got == alone {
		t.Errorf("another home's open did not read the log found in another case: %d rows", got)
	}
}

func TestAListedChildDirectoryIsNotListedAgainForItsChildren(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeEmpty(t, root, "one/Sessions.json")
	parent, _ := DirectoryAt(&input.Artifact{Home: &input.Home{DiskPath: root}}, root)
	children := ListChildren(parent, []string{"one"}, nil, Work{}, "listings")
	child := children.Directory(children.Holders("Sessions.json")[0])
	writeEmpty(t, root, "one/Later.json")
	if got, _ := child.Child("later.json"); got != filepath.Join(root, "one", "later.json") {
		t.Errorf("a directory ListChildren listed was listed again: Child = %s", got)
	}
	if got, _ := child.Child("sessions.json"); got != filepath.Join(root, "one", "Sessions.json") {
		t.Errorf("Child = %s", got)
	}
}
