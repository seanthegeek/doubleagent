package sqlitefile

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// copyTestdata copies a test database and the sidecars named by suffixes
// into directory and returns the copy's path.
func copyTestdata(t *testing.T, directory, name string, suffixes ...string) string {
	t.Helper()
	target := filepath.Join(directory, name)
	for _, suffix := range append([]string{""}, suffixes...) {
		data := testdataBytes(t, name+suffix)
		if err := osfile.WriteFile(target+suffix, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return target
}

// tableCount opens path and counts the rows of table t.
func tableCount(t *testing.T, path string) (int, error) {
	t.Helper()
	database, err := OpenPath(path, osfile.ListSpellings)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Error(err)
		}
	}()
	return countRows(database.tableRows(rootOf(t, database, "t")))
}

func TestOpenPathReadsSidecars(t *testing.T) {
	cases := []struct {
		name     string
		database string
		suffixes []string
		rows     int
	}{
		{"rows that live only in the log are read", "log_rows.db", []string{"-wal"}, 5},
		{"without its log the database has only its schema", "log_rows.db", nil, 0},
		{"a hot journal is played back", "journal_hot.db", []string{"-journal"}, 12},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			path := copyTestdata(t, t.TempDir(), test.database, test.suffixes...)
			count, err := tableCount(t, path)
			if err != nil || count != test.rows {
				t.Errorf("rows = %d, %v; want %d", count, err, test.rows)
			}
		})
	}
}

func TestOpenPathSkipsADirectorySidecar(t *testing.T) {
	path := copyTestdata(t, t.TempDir(), "log_rows.db")
	if err := osfile.MkdirAll(path+"-wal", 0o700); err != nil {
		t.Fatal(err)
	}
	if count, err := tableCount(t, path); err != nil || count != 0 {
		t.Errorf("rows = %d, %v; want 0 with the log a directory", count, err)
	}
}

func TestOpenPathWithoutTheFileIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	if _, err := OpenPath(path, osfile.ListSpellings); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("OpenPath(, osfile.ListSpellings) error = %v, want fs.ErrNotExist", err)
	}
	if _, err := OpenPath(t.TempDir(), osfile.ListSpellings); err == nil {
		t.Error("OpenPath(, osfile.ListSpellings) of a directory succeeded")
	}
}

func TestOpenPathReportsANonDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.db")
	if err := osfile.WriteFile(path, []byte("these are not the pages you are looking for"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenPath(path, osfile.ListSpellings); !errors.Is(err, ErrNotDatabase) {
		t.Errorf("OpenPath(, osfile.ListSpellings) error = %v, want ErrNotDatabase", err)
	}
}
