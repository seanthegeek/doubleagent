package sqlitefile

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// writeFile writes data to name under directory and returns its path.
func writeFile(t *testing.T, directory, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestProblemAndIsSQLiteReadTheHeader(t *testing.T) {
	directory := t.TempDir()
	database := writeFile(t, directory, "sessions.db", testdataBytes(t, "page_1024.db"))
	cases := []struct {
		name     string
		path     string
		problem  string
		isSQLite bool
	}{
		{"a database has no problem", database, "", true},
		{"a text file is not a database", writeFile(t, directory, "text.db", []byte("not a database\n")),
			ErrNotDatabase.Error(), false},
		{"a short file is not a database", writeFile(t, directory, "short.db", []byte("SQLite")),
			ErrNotDatabase.Error(), false},
		{"an empty file is an empty database", writeFile(t, directory, "empty.db", nil), "", false},
		{"a missing file cannot be read", filepath.Join(directory, "absent.db"),
			"absent.db could not be read", false},
		{"a directory cannot be read", directory, "could not be read (not a regular file)", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := Problem(test.path)
			if test.problem == "" && err != nil || test.problem != "" &&
				(err == nil || !strings.Contains(err.Error(), test.problem)) {
				t.Errorf("Problem() = %v, want %q", err, test.problem)
			}
			if got := IsSQLite(test.path); got != test.isSQLite {
				t.Errorf("IsSQLite() = %v, want %v", got, test.isSQLite)
			}
		})
	}
}

func TestProblemSkipsSidecarsACopyWouldSkip(t *testing.T) {
	directory := t.TempDir()
	database := writeFile(t, directory, "sessions.db", testdataBytes(t, "page_1024.db"))
	for _, suffix := range []string{"-wal", "-journal"} {
		writeFile(t, directory, "sessions.db"+suffix, nil)
	}
	if err := Problem(database); err != nil {
		t.Errorf("readable empty sidecars: Problem() = %v", err)
	}
	other := writeFile(t, directory, "other.db", testdataBytes(t, "page_1024.db"))
	if err := osfile.MkdirAll(other+"-wal", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Problem(other); err != nil {
		t.Errorf("a directory sidecar: Problem() = %v, want none", err)
	}
	var unreadable *UnreadableError
	wrapped := &UnreadableError{Path: "/x/a.db-wal", Err: errors.New("disk gone")}
	if !errors.As(error(wrapped), &unreadable) || wrapped.Error() != "a.db-wal could not be read (disk gone)" {
		t.Errorf("UnreadableError text = %q", wrapped.Error())
	}
}
