package opencode

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/parsertest"
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
	var texts []string
	for _, row := range parseDatabaseRows(t, name, setting) {
		texts = append(texts, row.Text)
	}
	return texts
}

// parseDatabaseRows copies the named testdata database into a new home at
// databaseRel and returns every row the parser gives for it in a run that
// shows secrets as setting says.
func parseDatabaseRows(t *testing.T, name string, setting secrets.Setting) []timeline.Row {
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
		Agent: "opencode", Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: secrets.NewRecorder(setting)},
		Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	err = parsertest.Parse(t, New(), artifact, parser.Options{}, func(row timeline.Row) { rows = append(rows, row) })
	if err != nil {
		t.Fatal(err)
	}
	return rows
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

// A message id is unique in OpenCode's tables, so a repeated one is damage:
// the message's parts are shown once, with the first row of the id in the
// order the rows are read, and the later rows of the id give their own rows
// without them and one problem row for the table, so the rows grow with
// the table rather than with the square of it.
func TestARepeatedMessageIDShowsItsPartsOnce(t *testing.T) {
	tests := []struct {
		name, database, table string
		parts, repeats        int
	}{
		{"10 rows of one id in session_message give each part once", "repeated_message_id_10.db",
			"session_message", 10, 9},
		{"40 rows of one id in session_message give each part once", "repeated_message_id_40.db",
			"session_message", 40, 39},
		{"10 rows of one id in message give each part once", "repeated_v1_message_id_10.db", "message", 10, 9},
		{"40 rows of one id in message give each part once", "repeated_v1_message_id_40.db", "message", 40, 39},
		{"three spellings of one id that the column compares as one give its parts once",
			"caseless_message_id.db", "message", 2, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseDatabase(t, test.database)
			want := []string{"session start: Repeat directory=/work/proj"}
			for i := range test.parts {
				want = append(want, fmt.Sprintf("part %d", i))
			}
			want = append(want, fmt.Sprintf("parser: %s holds %d rows repeating an earlier row's id; "+
				"its parts are shown once", test.table, test.repeats))
			if !slices.Equal(got, want) {
				t.Errorf("%d rows ending %q, want %d rows ending %q", len(got), got[len(got)-1], len(want),
					want[len(want)-1])
			}
		})
	}
}

// A message's own working directory is the project of its rows when its
// session gives none, and never that of another message of the session.
func TestEachMessageTakesItsOwnWorkingDirectory(t *testing.T) {
	want := map[string]string{"session start: t": "", "from msg_1": "/a", "from msg_2": "/b", "from msg_3": ""}
	rows := parseDatabaseRows(t, "message_directories.db", secrets.Setting{})
	if len(rows) != len(want) {
		t.Fatalf("%d rows %+v, want %d", len(rows), rows, len(want))
	}
	for _, row := range rows {
		if project, ok := want[row.Text]; !ok || row.ProjectPath != project {
			t.Errorf("row %q has project %q, want %q", row.Text, row.ProjectPath, project)
		}
	}
}

func TestPermissionEntriesThatAreNotRulesAreCountedInOneRow(t *testing.T) {
	want := []string{
		"session start: t2 directory=/work/two",
		"session start: t1 directory=/work/one (only in the 1.x session table)",
		"permission setting: session read *=allow (from opencode.db session_v2.permission, time_updated)",
		"permission setting: session bash ls=ask (from opencode.db session.permission, time_updated)",
		"parser: session_v2.permission holds 3 entries that are not permission rules",
		"parser: session.permission holds 1 entry that is not a permission rule",
	}
	if got := parseDatabase(t, "permission_entries.db"); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
