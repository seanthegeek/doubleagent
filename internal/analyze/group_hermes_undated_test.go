package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserregistry"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// hermesUndatedRepeat returns a Hermes state.db whose sessions table holds
// one session id twice, as only damage writes: a dated row in /proj/dated,
// and a row with no started_at in /proj/undated; and a message of the
// session a day after the dated row.
func hermesUndatedRepeat(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "sessions", SQL: "CREATE TABLE sessions (id TEXT, source TEXT, model TEXT, " +
			"started_at REAL, title TEXT, cwd TEXT, git_branch TEXT)", Rows: [][]any{
			{"s1", "cli", "model-dated", 1780272000.0, "dated", "/proj/dated", "branch-dated"},
			{"s1", "cli", "model-undated", nil, "undated", "/proj/undated", "branch-undated"}}},
		sqlitetest.Table{Name: "messages", SQL: "CREATE TABLE messages (id INTEGER PRIMARY KEY, session_id TEXT, " +
			"role TEXT, content TEXT, timestamp REAL)", Rows: [][]any{
			{1, "s1", "user", "the question", 1780358400.0}}})
}

// hermesUndatedRun runs timeline over a collection holding the database
// above with the options given, and returns each row with a session as
// "time project branch model text".
func hermesUndatedRun(t *testing.T, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "hermes", ".hermes/state.db", hermesUndatedRepeat(t)}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parserregistry.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline", "--match", "session start|question"}, options...), collection,
		output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, TimelineFile), "session_id", "timestamp_utc", "project_path",
		"git_branch", "model", "text")
}

// The sessions rows are read by started_at, so a row with none comes
// first; --keep-undated keeps it beside a window, so the session's
// messages go with it, as with any first stored row the filters keep.
func TestAnUndatedHermesSessionRowKeptByKeepUndatedTakesTheSessionsMessages(t *testing.T) {
	const (
		undated  = " /proj/undated branch-undated model-undated "
		dated    = "2026-06-01T00:00:00.000Z /proj/dated branch-dated model-dated session start | dated | source=cli"
		question = "2026-06-02T00:00:00.000Z "
	)
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"a window alone keeps the dated row, which the message goes with", []string{"--since", "2026-03-01"},
			[]string{dated, question + "/proj/dated branch-dated model-dated the question"}},
		{"a window with --keep-undated keeps the undated row too, which the message goes with",
			[]string{"--since", "2026-03-01", "--keep-undated"}, []string{dated,
				question + "/proj/undated branch-undated model-undated the question",
				undated + "session start | undated | source=cli"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := hermesUndatedRun(t, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}
