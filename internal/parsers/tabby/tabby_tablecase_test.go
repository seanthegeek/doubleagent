package tabby

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit/parsertest"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// rowsOfTableCaseDatabase copies the named testdata database, written by
// testdata/table_case.py, into a new home at the database's path and
// returns the text of every row the parser gives for it.
func rowsOfTableCaseDatabase(t *testing.T, name string) []string {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return parseAsBy(t, parsertest.Parse, secrets.Setting{}, "tabby", ".tabby/ee/db.sqlite", string(data))
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

// SQL matches a column name in any ASCII case, so a database whose columns
// are spelled in another case gives the rows it gives as written.
func TestAColumnSpelledInAnotherCaseIsRead(t *testing.T) {
	want := rowsOfTableCaseDatabase(t, "column_case.db")
	if len(want) == 0 {
		t.Fatal("the database as written gave no rows")
	}
	if got := rowsOfTableCaseDatabase(t, "column_case_respelled.db"); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
