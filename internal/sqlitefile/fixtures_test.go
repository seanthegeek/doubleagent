package sqlitefile

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// The fixture home's SQLite databases and their sidecars, and the dump of
// what SQLite reads from each in testdata/fixture_tables.txt, are fixed
// fixtures: no script in the tree builds them again.

// archivedFiles returns the files of testdata/fixture_databases.tar.gz by
// their slash-separated names.
func archivedFiles(t *testing.T) map[string][]byte {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", "fixture_databases.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return files
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag != tar.TypeReg {
			t.Fatalf("%s: not a regular file", header.Name)
		}
		if files[header.Name], err = io.ReadAll(reader); err != nil {
			t.Fatal(err)
		}
	}
}

// unpackDatabases writes the files of testdata/fixture_databases.tar.gz
// under directory.
func unpackDatabases(t *testing.T, directory string) {
	t.Helper()
	for name, data := range archivedFiles(t) {
		target := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFixtureDatabasesReadAsSQLiteReadsThem(t *testing.T) {
	home := t.TempDir()
	unpackDatabases(t, home)
	databases := loadDump(t, filepath.Join("testdata", "fixture_tables.txt"))
	if len(databases) < 20 {
		t.Fatalf("the fixture dump holds %d databases, fewer than the fixtures build", len(databases))
	}
	for _, expected := range databases {
		t.Run(expected.name+" reads as SQLite reads it", func(t *testing.T) {
			database, err := OpenPath(filepath.Join(home, filepath.FromSlash(expected.name)))
			if expected.err != "" {
				if err == nil {
					_, err = database.HasTable("")
					database.Close()
				}
				if err == nil {
					t.Errorf("read without error; SQLite reports %q", expected.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			compareTables(t, database, expected)
			compareSites(t, database, expected)
		})
	}
}
