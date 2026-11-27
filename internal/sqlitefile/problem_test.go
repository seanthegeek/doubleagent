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

// openProblem opens the database at path with OpenPath, closes it, and
// returns the error opening met.
func openProblem(t *testing.T, path string) error {
	t.Helper()
	database, err := OpenPath(path, osfile.ListSpellings)
	if err == nil {
		if closeErr := database.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}
	return err
}

func TestOpenPathJudgesTheHeader(t *testing.T) {
	directory := t.TempDir()
	database := writeFile(t, directory, "sessions.db", testdataBytes(t, "page_1024.db"))
	cases := []struct {
		name    string
		path    string
		problem string
	}{
		{"a database has no problem", database, ""},
		{"a text file is not a database", writeFile(t, directory, "text.db", []byte("not a database\n")),
			ErrNotDatabase.Error()},
		{"a short file is not a database", writeFile(t, directory, "short.db", []byte("SQLite")),
			ErrNotDatabase.Error()},
		{"an empty file is an empty database", writeFile(t, directory, "empty.db", nil), ""},
		{"a missing file cannot be read", filepath.Join(directory, "absent.db"),
			"absent.db could not be read"},
		{"a directory cannot be read", directory, "could not be read (not a regular file)"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := openProblem(t, test.path)
			if test.problem == "" && err != nil || test.problem != "" &&
				(err == nil || !strings.Contains(err.Error(), test.problem)) {
				t.Errorf("OpenPath(, osfile.ListSpellings) = %v, want %q", err, test.problem)
			}
		})
	}
}

func TestOpenPathSkipsSidecarsACopyWouldSkip(t *testing.T) {
	directory := t.TempDir()
	database := writeFile(t, directory, "sessions.db", testdataBytes(t, "page_1024.db"))
	for _, suffix := range []string{"-wal", "-journal"} {
		writeFile(t, directory, "sessions.db"+suffix, nil)
	}
	if err := openProblem(t, database); err != nil {
		t.Errorf("readable empty sidecars: OpenPath(, osfile.ListSpellings) = %v", err)
	}
	other := writeFile(t, directory, "other.db", testdataBytes(t, "page_1024.db"))
	if err := osfile.MkdirAll(other+"-wal", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := openProblem(t, other); err != nil {
		t.Errorf("a directory sidecar: OpenPath(, osfile.ListSpellings) = %v, want none", err)
	}
	var unreadable *UnreadableError
	wrapped := &UnreadableError{Path: "/x/a.db-wal", Err: errors.New("disk gone")}
	if !errors.As(error(wrapped), &unreadable) || wrapped.Error() != "a.db-wal could not be read (disk gone)" {
		t.Errorf("UnreadableError text = %q", wrapped.Error())
	}
}
