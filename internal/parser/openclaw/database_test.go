package openclaw

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
	"github.com/seanthegeek/doubleagent/internal/parser/zed"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The databases under testdata are written by SQLite from the rows in
// testdata/fixtures.py.

const agentRel = ".openclaw/agents/a/agent/openclaw-agent.sqlite"

// agentArtifact copies the named testdata database into a new home as an
// agent database and returns the artifact.
func agentArtifact(t *testing.T, name string) *input.Artifact {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(agentRel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return &input.Artifact{DiskPath: path, Original: "/home/u/" + agentRel, Rel: agentRel, Agent: "openclaw",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
}

// readAgent reads the named testdata agent database as the parser does and
// returns its rows and the number of queries run.
func readAgent(t *testing.T, name string) ([]timeline.Row, int) {
	t.Helper()
	artifact := agentArtifact(t, name)
	database, err := sqlitefile.OpenPath(artifact.DiskPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var rows []timeline.Row
	subject := New()
	subject.Work = parser.CountingWork()
	reader := &agentDatabase{writer: &writer{Parser: subject, artifact: artifact,
		emit: func(row timeline.Row) { rows = append(rows, row) }}, database: database,
		store: newDatabaseState(), windows: windowSet{byID: map[string]sqlitefile.Row{}}}
	if err := reader.read(); err != nil {
		t.Fatal(err)
	}
	return rows, subject.Work.Count("queries")
}

func TestTheColdArchiveChunksAreReadOnceWhateverTheNumberOfArchives(t *testing.T) {
	_, few := readAgent(t, "cold_archives_2.db")
	_, many := readAgent(t, "cold_archives_8.db")
	if few == 0 || few != many {
		t.Errorf("2 archives ran %d queries and 8 ran %d; the count must not grow with the archives", few, many)
	}
}

func TestEachColdArchiveReadsItsSessionsChunksInChunkOrder(t *testing.T) {
	rows, _ := readAgent(t, "cold_chunks.db")
	var got []string
	for _, row := range rows {
		if row.TurnType == timeline.TurnUser {
			got = append(got, row.SessionID+" "+row.Text)
		}
	}
	want := []string{"sa a first", "sa a second", "sb b first", "sb b second", "sc c whole"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAnEventPastTheDatabasesBudgetIsAProblemRow(t *testing.T) {
	const decoded = 150000 // each event of zstd_events.db decompresses to 150000 bytes
	artifact := agentArtifact(t, "zstd_events.db")
	database, err := sqlitefile.OpenPath(artifact.DiskPath)
	if err != nil {
		t.Fatal(err)
	}
	total := int(database.PageCount()) * database.PageSize() * zed.BudgetRatio
	database.Close()
	if total < 2*decoded || total >= 3*decoded {
		t.Fatalf("the fixture's budget %d does not hold exactly two events", total)
	}
	tests := []struct {
		name  string
		floor int
		want  string
	}{
		{"the third event passes the budget of the database's size", 0,
			fmt.Sprintf("parser: event seq 2 could not be decompressed (zstd: not decompressed: this database's "+
				"blobs reached the limit of %d bytes decompressed in all)", total)},
		{"the floor holds all three events", zed.BudgetFloor, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			subject := New()
			subject.budgetFloor = test.floor
			var problems []string
			if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
				if strings.HasPrefix(row.Text, parser.ProblemPrefix) {
					problems = append(problems, row.Text)
				}
			}); err != nil {
				t.Fatal(err)
			}
			if test.want == "" && len(problems) > 0 || test.want != "" && !slices.Equal(problems, []string{test.want}) {
				t.Errorf("got %q, want %q", problems, test.want)
			}
		})
	}
}

func TestAnExecApprovalsConfigThatIsNotJSONGivesOnlyThePosition(t *testing.T) {
	const stateRel = ".openclaw/state/openclaw.sqlite"
	data, err := osfile.ReadFile(filepath.Join("testdata", "bad_exec_config.db"))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(stateRel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + stateRel, Rel: stateRel, Agent: "openclaw",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var got []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { got = append(got, row.Text) }); err != nil {
		t.Fatal(err)
	}
	want := []string{"parser: exec_approvals_config raw_json unparseable (invalid JSON at line 1, column 22; " +
		"text withheld, it holds the approvals socket token)"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
