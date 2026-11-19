package copilotdesktop

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/parsertest"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// rowsOfTableCaseDatabase copies the named testdata database, written by
// testdata/table_case.py, into a new home at the database's path and
// returns the text of every row the parser gives for it.
func rowsOfTableCaseDatabase(t *testing.T, name string) []string {
	t.Helper()
	return rowsOfDatabase(t, name, secrets.Setting{})
}

// rowsOfDatabase copies the named testdata database into a new home at the
// database's path and returns the text of every row the parser gives for
// it in a run that shows secrets as setting says.
func rowsOfDatabase(t *testing.T, name string, setting secrets.Setting) []string {
	t.Helper()
	const rel = ".copilot/data.db"
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
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "copilot-desktop",
		Home:     &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: secrets.NewRecorder(setting)},
		Manifest: jsonvalue.NewNull()}
	var texts []string
	if err := parsertest.Parse(t, New(), artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
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
