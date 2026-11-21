package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// ollamaMovedDatabase returns an Ollama db.sqlite whose chat c holds
// message id 1 three times, as only damage writes: with content and the
// model M1 on the first day, then twice with no content or thinking, with
// M2 and M3 on the next two days; then message 2 with content and M4 on
// the fourth day. Message 1 has one tool call.
func ollamaMovedDatabase(t *testing.T) string {
	t.Helper()
	message := func(messageID int, content, model, created string) []any {
		return []any{messageID, "c", "assistant", content, "", model, created}
	}
	data, err := sqlitetest.Build(
		sqlitetest.Table{Name: "messages", SQL: "CREATE TABLE messages (id INTEGER, chat_id TEXT, role TEXT, " +
			"content TEXT, thinking TEXT, model_name TEXT, created_at TEXT)", Rows: [][]any{
			message(1, "first", "M1", "2026-01-01 00:00:00"),
			message(1, "", "M2", "2026-01-02 00:00:00"),
			message(1, "", "M3", "2026-01-03 00:00:00"),
			message(2, "later", "M4", "2026-01-04 00:00:00")}},
		sqlitetest.Table{Name: "tool_calls", SQL: "CREATE TABLE tool_calls (id INTEGER, message_id INTEGER, " +
			"function_name TEXT, function_arguments TEXT)", Rows: [][]any{{1, 1, "search", `{"query":"the call"}`}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// ollamaMovedRun runs timeline over a collection holding the database
// above with the options given, and returns each row with a session as
// "time model text".
func ollamaMovedRun(t *testing.T, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "ollama", "Library/Application Support/Ollama/db.sqlite",
			ollamaMovedDatabase(t)}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, TimelineFile), "session_id", "timestamp_utc", "model", "text")
}

// When one id has two messages with no content or thinking, the chat's
// next model: row reads from the last of them when the calls go with it,
// and from the chat's tracked model otherwise, also when they go with the
// earlier one: a row shown only when the group goes with none of several
// parents is a form the grouping lacks. This pins that limit, which
// research/ollama.md and analyzer/docs/output.md state.
func TestTheNextModelRowAfterSeveralEmptyOllamaMessagesOfAnIDReadsFromTheLastOfThem(t *testing.T) {
	const (
		first  = "2026-01-01T00:00:00.000Z "
		second = "2026-01-02T00:00:00.000Z "
		third  = "2026-01-03T00:00:00.000Z "
		fourth = "2026-01-04T00:00:00.000Z "
	)
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"without a filter the calls go with the first message", nil, []string{first + "M1 model: M1",
			first + "M1 first", first + "M1 the call", fourth + "M4 model: M1 -> M4", fourth + "M4 later"}},
		{"a window from the last empty message reads from it", []string{"--since", "2026-01-03"}, []string{
			third + "M3 the call", third + "M3 model: M1 -> M3", fourth + "M4 model: M3 -> M4", fourth + "M4 later"}},
		{"a window from the earlier empty message reads from the tracked model", []string{"--since", "2026-01-02"},
			[]string{second + "M2 the call", second + "M2 model: M1 -> M2", fourth + "M4 model: M1 -> M4",
				fourth + "M4 later"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := ollamaMovedRun(t, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}
