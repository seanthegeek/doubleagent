package copilotdesktop

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// keysTables returns the projects, workspaces, aliases and sessions tables
// of a data.db, the key columns declared with the collations given, each
// holding the rows given; the builder writes each value as given, so a
// number stays a number in a TEXT column, as only damage leaves one.
func keysTables(collation string, projects, workspaces, aliases, sessions [][]any) []sqlitetest.Table {
	return []sqlitetest.Table{
		{Name: "projects", SQL: "CREATE TABLE projects (id TEXT" + collation + ", name TEXT, main_repo_path TEXT, " +
			"created_at TEXT)", Rows: projects},
		{Name: "workspaces", SQL: "CREATE TABLE workspaces (id TEXT" + collation + ", project_id TEXT, name TEXT, " +
			"created_at TEXT)", Rows: workspaces},
		{Name: "workspace_session_aliases", SQL: "CREATE TABLE workspace_session_aliases (session_id TEXT" +
			collation + ", workspace_id TEXT, created_at TEXT)", Rows: aliases},
		{Name: "sessions", SQL: "CREATE TABLE sessions (id TEXT, title TEXT, created_at TEXT)", Rows: sessions},
	}
}

// parseTables builds data.db from tables, parses it and returns each row
// as "session project text".
func parseTables(t *testing.T, tables []sqlitetest.Table) []string {
	t.Helper()
	data, err := sqlitetest.Build(tables...)
	if err != nil {
		t.Fatal(err)
	}
	const rel = ".copilot/data.db"
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "copilot-desktop",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var rows []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) {
		rows = append(rows, row.SessionID+" "+row.ProjectPath+" "+row.Text)
	}); err != nil {
		t.Fatal(err)
	}
	return rows
}

// sessionRowsOf returns the rows of rows that are a session's row.
func sessionRowsOf(rows []string) []string {
	var found []string
	for _, row := range rows {
		if strings.Contains(row, "desktop session:") {
			found = append(found, row)
		}
	}
	return found
}

// A key joins only what SQL's = would join between the two columns: NULL
// joins nothing, empty text only empty text, a number never its text, and
// text in a column declared COLLATE NOCASE joins its other cases.
func TestAKeyJoinsOnlyWhatTheColumnsCompareEqual(t *testing.T) {
	const session = "desktop session: t type= model=- auto_approve= permission_mode=- remote_control= archived=false"
	project := []any{"p1", "proj", "/proj", "2026-01-01T00:00:00Z"}
	tests := []struct {
		name      string
		collation string
		workspace []any
		alias     []any
		want      string
	}{
		{"a session with no alias takes no workspace whose id is NULL", "", []any{nil, "p1", "w", "2026-01-01T00:00:00Z"},
			[]any{"other", "w1", "2026-01-01T00:00:00Z"}, "s1  " + session},
		{"a NULL workspace_id finds no workspace whose id is empty text", "",
			[]any{"", "p1", "w", "2026-01-01T00:00:00Z"}, []any{"s1", nil, "2026-01-01T00:00:00Z"}, "s1  " + session},
		{"empty text finds the workspace whose id is empty text", "",
			[]any{"", "p1", "w", "2026-01-01T00:00:00Z"}, []any{"s1", "", "2026-01-01T00:00:00Z"}, "s1 /proj " + session},
		{"a number finds no workspace whose id is its text", "",
			[]any{"1", "p1", "w", "2026-01-01T00:00:00Z"}, []any{"s1", int64(1), "2026-01-01T00:00:00Z"}, "s1  " + session},
		{"another case finds no workspace under BINARY", "",
			[]any{"W1", "p1", "w", "2026-01-01T00:00:00Z"}, []any{"s1", "w1", "2026-01-01T00:00:00Z"}, "s1  " + session},
		{"another case finds the workspace under NOCASE", " COLLATE NOCASE",
			[]any{"W1", "p1", "w", "2026-01-01T00:00:00Z"}, []any{"s1", "w1", "2026-01-01T00:00:00Z"},
			"s1 /proj " + session},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := sessionRowsOf(parseTables(t, keysTables(test.collation, [][]any{project}, [][]any{test.workspace},
				[][]any{test.alias}, [][]any{{"s1", "t", "2026-01-02T00:00:00Z"}})))
			if !slices.Equal(rows, []string{test.want}) {
				t.Errorf("rows %q, want %q", rows, test.want)
			}
		})
	}
}

// A key is repeated only when SQL's = finds two of its stored rows equal:
// a number and its text are two keys, and so are two cases of a text
// unless the column is declared COLLATE NOCASE.
func TestAKeyRepeatsOnlyWhenTheColumnComparesItsRowsEqual(t *testing.T) {
	tests := []struct {
		name      string
		collation string
		first     any
		second    any
		repeated  bool
	}{
		{"a number and its text", "", int64(1), "1", false},
		{"two cases under BINARY", "", "P", "p", false},
		{"two cases under NOCASE", " COLLATE NOCASE", "P", "p", true},
		{"two NULL ids", "", nil, nil, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := parseTables(t, keysTables(test.collation, [][]any{
				{test.first, "a", "/proj/a", "2026-01-01T00:00:00Z"}, {test.second, "b", "/proj/b", "2026-02-01T00:00:00Z"}},
				nil, nil, nil))
			notices := 0
			for _, row := range rows {
				if strings.Contains(row, "parser: projects holds") {
					notices++
				}
			}
			if got := notices == 2; got != test.repeated {
				t.Errorf("%d parser: rows in %q, repeated %v", notices, rows, test.repeated)
			}
		})
	}
}
