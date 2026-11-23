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

// gooseSessionsRel is where Goose keeps its session database.
const gooseSessionsRel = ".local/share/goose/sessions/sessions.db"

// gooseRepeatedSession returns a Goose sessions.db whose sessions table
// holds the session id s1 twice, as only damage writes: a row in
// /proj/new with model new-model, and an earlier one in /proj/old with
// old-model; and two messages of the session a day after the later row,
// one naming no model and one naming its own.
func gooseRepeatedSession(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "sessions", SQL: "CREATE TABLE sessions (id TEXT, name TEXT, working_dir TEXT, " +
			"created_at TIMESTAMP, provider_name TEXT, model_config_json TEXT)", Rows: [][]any{
			{"s1", "new", "/proj/new", "2026-06-01 00:00:00", "p", `{"model_name":"new-model"}`},
			{"s1", "old", "/proj/old", "2026-01-01 00:00:00", "p", `{"model_name":"old-model"}`}}},
		sqlitetest.Table{Name: "messages", SQL: "CREATE TABLE messages (id INTEGER PRIMARY KEY, session_id TEXT, " +
			"role TEXT, content_json TEXT, created_timestamp INTEGER, metadata_json TEXT)", Rows: [][]any{
			{1, "s1", "user", `[{"type":"text","text":"the question"}]`, 1780358400, "{}"},
			{2, "s1", "assistant", `[{"type":"text","text":"the answer"}]`, 1780358401,
				`{"inference":{"provider":"q","resolvedModel":"own-model"}}`}}})
}

// gooseRun runs timeline over a collection holding the Goose database
// given, with the options given, and returns each row of timeline.jsonl
// that has a session as "time session project model text".
func gooseRun(t *testing.T, data string, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "goose", gooseSessionsRel, data}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parserregistry.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline", "--agent", "goose"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, TimelineFile), "session_id", "timestamp_utc", "session_id",
		"project_path", "model", "text")
}

// The rows of the Goose session above: each stored row's own, at its time
// with its project and model, and the session's messages.
const (
	gooseShown    = "parser: sessions holds 2 rows of this row's id; its messages are shown once, with "
	gooseOld      = "2026-01-01T00:00:00.000Z s1 /proj/old p/old-model "
	gooseNew      = "2026-06-01T00:00:00.000Z s1 /proj/new p/new-model "
	gooseQuestion = "2026-06-02T00:00:00.000Z s1 "
	gooseAnswer   = "2026-06-02T00:00:01.000Z s1 "
)

func TestTheMessagesOfARepeatedGooseSessionGoWithTheFirstRowTheFilterKeeps(t *testing.T) {
	t.Parallel()
	ownRows := func(stored string, chosen bool) []string {
		which := "another of them"
		if chosen {
			which = "this row"
		}
		name, model := "old", "old-model"
		if stored == gooseNew {
			name, model = "new", "new-model"
		}
		return []string{stored + "session start | " + name, stored + gooseShown + which,
			stored + "model: " + model + " provider=p"}
	}
	messages := func(project, model string) []string {
		return []string{gooseQuestion + project + " " + model + " the question",
			gooseAnswer + project + " q/own-model model: own-model provider=q",
			gooseAnswer + project + " q/own-model the answer"}
	}
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"without a filter they go with the first row by created_at", nil,
			slices.Concat(ownRows(gooseOld, true), ownRows(gooseNew, false), messages("/proj/old", "p/old-model"))},
		{"a window past the first row moves them to the second", []string{"--since", "2026-03-01"},
			slices.Concat(ownRows(gooseNew, true), messages("/proj/new", "p/new-model"))},
		{"a pattern only the second row's start matches moves them to the second",
			[]string{"--match", "session start \\| new|the question"},
			[]string{gooseNew + "session start | new", gooseQuestion + "/proj/new p/new-model the question"}},
		{"a pattern no stored row's rows match keeps them with the first", []string{"--match", "the question"},
			[]string{gooseQuestion + "/proj/old p/old-model the question"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := gooseRun(t, gooseRepeatedSession(t), test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}
