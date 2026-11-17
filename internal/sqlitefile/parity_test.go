//go:build parity

package sqlitefile

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// fixtureScript dumps the analyzer's fixture databases; see its docstring.
const fixtureScript = "testdata/fixture_tables.py"

// fixtureDump runs fixtureScript under the interpreter PARITY_PYTHON names,
// skipping the test when the variable is unset, and returns the directory
// holding the fixture home and its dump.
func fixtureDump(t *testing.T) string {
	t.Helper()
	python := os.Getenv("PARITY_PYTHON")
	if python == "" {
		t.Skip("PARITY_PYTHON is not set; it names the Python 3 interpreter the oracle runs on")
	}
	analyzer, err := filepath.Abs(filepath.Join("..", "..", "analyzer"))
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	command := exec.Command(python, "-I", fixtureScript, analyzer, output)
	// The fixture builders make scratch directories of their own.
	command.Env = append(os.Environ(), "TMPDIR="+output, "TMP="+output, "TEMP="+output)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("%s: %v\n%s", fixtureScript, err, stderr.String())
	}
	return output
}

func TestFixtureDatabasesReadAsSQLiteReadsThem(t *testing.T) {
	output := fixtureDump(t)
	databases := loadDump(t, filepath.Join(output, "fixtures.txt"))
	if len(databases) < 20 {
		t.Fatalf("the fixture dump holds %d databases, fewer than the fixtures build", len(databases))
	}
	tables := 0
	for _, expected := range databases {
		t.Run(expected.name+" reads as SQLite reads it", func(t *testing.T) {
			database, err := OpenPath(filepath.Join(output, "home", filepath.FromSlash(expected.name)))
			if expected.err != "" {
				if err == nil {
					_, err = database.TableNames()
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
			tables += len(expected.tables)
		})
	}
	t.Logf("compared %d tables of %d databases", tables, len(databases))
}
