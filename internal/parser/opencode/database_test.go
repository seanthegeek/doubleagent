package opencode

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The databases under testdata are written by SQLite from the SQL in
// testdata/fixtures.py.

const databaseRel = ".local/share/opencode/opencode.db"

// parseDatabase copies the named testdata database into a new home at
// databaseRel and returns the text of every row the parser gives for it.
func parseDatabase(t *testing.T, name string) []string {
	t.Helper()
	return parseDatabaseWith(t, name, secrets.Setting{})
}

// parseDatabaseWith is parseDatabase in a run that shows secrets as
// setting says.
func parseDatabaseWith(t *testing.T, name string, setting secrets.Setting) []string {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(databaseRel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + databaseRel, Rel: databaseRel,
		Agent: "opencode", Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: setting},
		Manifest: jsonvalue.NewNull()}
	var texts []string
	err = New().Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) })
	if err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestTheFirstLayoutOfThePermissionTableLeavesTheTranscriptWhole(t *testing.T) {
	want := parseDatabase(t, "no_permission_table.db")
	if !slices.Contains(want, "hello there") {
		t.Fatalf("the database without the table gives %q, without its message", want)
	}
	for _, name := range []string{"first_layout_permission.db", "first_layout_permission_with_times.db"} {
		t.Run("a database with "+strings.TrimSuffix(name, ".db")+" reads as one without the table",
			func(t *testing.T) {
				if got := parseDatabase(t, name); !slices.Equal(got, want) {
					t.Errorf("got %q, want %q", got, want)
				}
			})
	}
}

// A NULL id equals nothing in SQL, not even another NULL, so a session
// whose project_id is NULL has no project, though a project row's id is
// NULL too.
func TestANullProjectIDMatchesNoProject(t *testing.T) {
	nullProject := sqlitefile.Row{}
	reading := &store{projects: map[string]sqlitefile.Row{}}
	reading.addProject(nullProject)
	if len(reading.projects) != 0 {
		t.Errorf("a project with a NULL id was indexed: %v", reading.projects)
	}
	reading.projects[parser.ColumnKey(sqlitefile.Value{})] = nullProject
	if got := reading.worktreeOf(sqlitefile.Value{}); got != "" {
		t.Errorf("a NULL project_id found the worktree %q", got)
	}
}
