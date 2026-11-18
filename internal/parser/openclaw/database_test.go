package openclaw

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
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
	reader := &agentDatabase{writer: &writer{Parser: New(), artifact: artifact,
		emit: func(row timeline.Row) { rows = append(rows, row) }}, database: database,
		store: newDatabaseState(), windows: windowSet{byID: map[string]sqlitefile.Row{}}}
	if err := reader.read(); err != nil {
		t.Fatal(err)
	}
	return rows, reader.queries
}

func TestTheColdArchiveChunksAreReadOnceWhateverTheNumberOfArchives(t *testing.T) {
	_, few := readAgent(t, "cold_archives_2.db")
	_, many := readAgent(t, "cold_archives_8.db")
	if few != many {
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
