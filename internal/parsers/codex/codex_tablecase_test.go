package codex

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// rowsOfDatabaseAt copies the named testdata database into a new home at
// rel and returns the text of every row the parser gives for it.
func rowsOfDatabaseAt(t *testing.T, rel, name string) []string {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "codex",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var texts []string
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

// rowsOfTableCaseDatabase copies the named testdata database, written by
// testdata/table_case.py, into a new home at the database's path and
// returns the text of every row the parser gives for it.
func rowsOfTableCaseDatabase(t *testing.T, name string) []string {
	t.Helper()
	return rowsOfDatabaseAt(t, ".codex/state_5.sqlite", name)
}

// SQL matches a table name in any ASCII case, so a database whose tables
// are spelled in another case gives the rows it gives as written.
func TestATableSpelledInAnotherCaseIsRead(t *testing.T) {
	want := rowsOfTableCaseDatabase(t, "table_case.db")
	if len(want) == 0 {
		t.Fatal("the database as written gave no rows")
	}
	if got := rowsOfTableCaseDatabase(t, "table_case_respelled.db"); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// SQL matches a column name in any ASCII case, so a state or logs database
// whose columns are spelled in another case gives the rows it gives as
// written.
func TestAColumnSpelledInAnotherCaseIsRead(t *testing.T) {
	for _, rel := range []string{".codex/state_5.sqlite", ".codex/logs_1.sqlite"} {
		t.Run(rel, func(t *testing.T) {
			want := rowsOfDatabaseAt(t, rel, "column_case.db")
			if len(want) == 0 {
				t.Fatal("the database as written gave no rows")
			}
			if got := rowsOfDatabaseAt(t, rel, "column_case_respelled.db"); !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}
