package sqlitefile

import (
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

// copySpelled copies a test database into directory, and its sidecar of
// suffix under the name spelled, and returns the database's path.
func copySpelled(t *testing.T, directory, name, suffix, spelled string) string {
	t.Helper()
	path := copyTestdata(t, directory, name)
	if err := osfile.WriteFile(filepath.Join(directory, spelled), testdataBytes(t, name+suffix), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// countedOpen opens path through OpenPath, counting the listings asked for,
// and returns the rows of table t, the files opened and the count.
func countedOpen(t *testing.T, path string) (int, int, int) {
	t.Helper()
	listings := 0
	database, err := OpenPath(path, func(directory string) osfile.Spellings {
		listings++
		return osfile.ListSpellings(directory)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Error(err)
		}
	}()
	rows, err := countRows(database.tableRows(rootOf(t, database, "t")))
	if err != nil {
		t.Fatal(err)
	}
	return rows, len(database.closers), listings
}

func TestOpenPathReadsASidecarSpelledInAnotherCase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, database, suffix, spelled string
		rows                            int
	}{
		{"the log's rows are read", "log_rows.db", "-wal", "log_rows.db-WAL", 5},
		{"a journal is read", "journal_hot.db", "-journal", "journal_hot.db-Journal", 12},
		{"a sidecar whose database part is spelled otherwise is found", "log_rows.db", "-wal", "LOG_ROWS.DB-wal", 5},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			skipWhereCaseIsIgnored(t, directory)
			rows, files, listings := countedOpen(t, copySpelled(t, directory, test.database, test.suffix, test.spelled))
			// The other sidecar is absent too, so it asks for the listing
			// again; the run's store answers that from the first listing.
			if rows != test.rows || files != 2 || listings != 2 {
				t.Errorf("rows = %d from %d files after %d listings, want %d from 2 after two", rows, files, listings,
					test.rows)
			}
		})
	}
}

func TestOpenPathAsksForAListingOnlyForAnAbsentSidecar(t *testing.T) {
	t.Parallel()
	both := copyTestdata(t, t.TempDir(), "log_rows.db", "-wal")
	if err := osfile.WriteFile(both+"-journal", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if rows, files, listings := countedOpen(t, both); rows != 5 || files != 3 || listings != 0 {
		t.Errorf("rows = %d from %d files after %d listings, want 5 from 3 after none", rows, files, listings)
	}
	none := copyTestdata(t, t.TempDir(), "log_rows.db")
	if rows, files, listings := countedOpen(t, none); rows != 0 || files != 1 || listings != 2 {
		t.Errorf("rows = %d from %d files after %d listings, want 0 from 1 after one for each absent sidecar",
			rows, files, listings)
	}
}
