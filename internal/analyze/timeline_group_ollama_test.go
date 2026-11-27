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

// ollamaRepeatedDatabase returns an Ollama db.sqlite whose messages
// repeat two ids, as only damage writes: an old assistant message and a
// new one five months later, each with its own model, sharing the id of
// one tool call, and an old and a new user message sharing the id of one
// attachment.
func ollamaRepeatedDatabase(t *testing.T) string {
	t.Helper()
	message := func(messageID int, role, content, model, created string) []any {
		return []any{messageID, "c", role, content, "", model, created}
	}
	data, err := sqlitetest.Build(
		sqlitetest.Table{Name: "messages", SQL: "CREATE TABLE messages (id INTEGER, chat_id TEXT, role TEXT, " +
			"content TEXT, thinking TEXT, model_name TEXT, created_at TEXT)", Rows: [][]any{
			message(1, "assistant", "old answer", "model-old", "2026-01-01 00:00:00"),
			message(1, "assistant", "new answer", "model-new", "2026-06-01 00:00:00"),
			message(2, "user", "old question", "", "2026-01-01 00:00:01"),
			message(2, "user", "new question", "", "2026-06-01 00:00:01")}},
		sqlitetest.Table{Name: "tool_calls", SQL: "CREATE TABLE tool_calls (id INTEGER, message_id INTEGER, " +
			"function_name TEXT, function_arguments TEXT)", Rows: [][]any{{1, 1, "search", `{"query":"the call"}`}}},
		sqlitetest.Table{Name: "attachments", SQL: "CREATE TABLE attachments (id INTEGER, message_id INTEGER, " +
			"filename TEXT, data BLOB)", Rows: [][]any{{1, 2, "notes.txt", []byte{0}}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// ollamaRepeatedRun runs timeline over a collection holding the repeated
// ids' database with the options given, and returns each row with a
// session as "time session model tool_use_id text".
func ollamaRepeatedRun(t *testing.T, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "ollama", "Library/Application Support/Ollama/db.sqlite",
			ollamaRepeatedDatabase(t)}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, timelineFile), "session_id", "timestamp_utc", "session_id", "model",
		"tool_use_id", "text")
}

func TestTheCallsAndAttachmentsOfARepeatedOllamaMessageGoWithTheFirstMessageTheFilterKeeps(t *testing.T) {
	t.Parallel()
	const (
		old       = "2026-01-01T00:00:00.000Z c model-old "
		current   = "2026-06-01T00:00:00.000Z c model-new "
		oldUser   = "2026-01-01T00:00:01.000Z c  "
		newUser   = "2026-06-01T00:00:01.000Z c  "
		attached  = "\n[attachment: notes.txt, 1 bytes]"
		callOld   = old + "c:1 the call"
		callNew   = current + "c:1 the call"
		oldAnswer = old + " old answer"
		newAnswer = current + " new answer"
	)
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"without a filter they go with the first message of the id", nil, []string{
			old + " model: model-old", oldAnswer, callOld, oldUser + " old question" + attached,
			current + " model: model-old -> model-new", newAnswer, newUser + " new question"}},
		{"a window that keeps only the later messages moves them there", []string{"--since", "2026-03-01"},
			[]string{callNew, current + " model: model-old -> model-new", newAnswer,
				newUser + " new question" + attached}},
		{"a window that keeps only the earlier messages keeps them there", []string{"--until", "2026-03-01"},
			[]string{old + " model: model-old", oldAnswer, callOld, oldUser + " old question" + attached}},
		{"a pattern only the later messages match moves them there", []string{"--match", "new answer|new question|call"},
			[]string{callNew, newAnswer, newUser + " new question" + attached}},
		{"a pattern only the call and the attachment match leaves them with the first", []string{"--match", "call|notes"},
			[]string{callOld, oldUser + " old question" + attached}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := ollamaRepeatedRun(t, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
			joined := strings.Join(rows, "\n")
			if calls, files := strings.Count(joined, "the call"), strings.Count(joined, "notes.txt"); calls != 1 ||
				files != 1 {
				t.Errorf("the call is shown %d times and the attachment %d, want each once", calls, files)
			}
		})
	}
}

// ollamaMovedModelDatabase returns an Ollama db.sqlite whose messages
// repeat the id of one tool call, as only damage writes: an old assistant
// message with content and model-old, and a new one five months later
// with model-new and no content or thinking of its own, followed in the
// chat by a message of the next model a month after it.
func ollamaMovedModelDatabase(t *testing.T, next string) string {
	t.Helper()
	message := func(messageID int, content, model, created string) []any {
		return []any{messageID, "c", "assistant", content, "", model, created}
	}
	data, err := sqlitetest.Build(
		sqlitetest.Table{Name: "messages", SQL: "CREATE TABLE messages (id INTEGER, chat_id TEXT, role TEXT, " +
			"content TEXT, thinking TEXT, model_name TEXT, created_at TEXT)", Rows: [][]any{
			message(1, "old answer", "model-old", "2026-01-01 00:00:00"),
			message(1, "", "model-new", "2026-06-01 00:00:00"),
			message(2, "next answer", next, "2026-07-01 00:00:00")}},
		sqlitetest.Table{Name: "tool_calls", SQL: "CREATE TABLE tool_calls (id INTEGER, message_id INTEGER, " +
			"function_name TEXT, function_arguments TEXT)", Rows: [][]any{{1, 1, "search", `{"query":"the call"}`}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// ollamaMovedModelRun runs timeline over ollamaMovedModelDatabase with the
// next model and the options given, and returns each row with a session
// as "time model text".
func ollamaMovedModelRun(t *testing.T, next string, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "ollama", "Library/Application Support/Ollama/db.sqlite",
			ollamaMovedModelDatabase(t, next)}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, timelineFile), "session_id", "timestamp_utc", "model", "text")
}

// When the calls of a repeated id are shown with a later message that has
// no content or thinking of its own, that message's model is named by a
// `model:` row there, as for any record of a new model, and the next model
// row of the chat reads from it; without a filter the rows are as before.
func TestAnOllamaMessageThatTakesTheCallsAlonePutsItsModelInAModelRow(t *testing.T) {
	t.Parallel()
	const (
		january = "2026-01-01T00:00:00.000Z model-old "
		june    = "2026-06-01T00:00:00.000Z model-new "
		julyNew = "2026-07-01T00:00:00.000Z model-new "
		julyOld = "2026-07-01T00:00:00.000Z model-old "
	)
	tests := []struct {
		name, next string
		options    []string
		want       []string
	}{
		{"without a filter the calls and the model row stay with the first message", "model-new", nil,
			[]string{january + "model: model-old", january + "old answer", january + "the call",
				julyNew + "model: model-old -> model-new", julyNew + "next answer"}},
		{"a window that keeps only the later message names its model there", "model-new",
			[]string{"--since", "2026-03-01"},
			[]string{june + "the call", june + "model: model-old -> model-new", julyNew + "next answer"}},
		{"a next message of the earlier model names the change back", "model-old",
			[]string{"--since", "2026-03-01"},
			[]string{june + "the call", june + "model: model-old -> model-new",
				julyOld + "model: model-new -> model-old", julyOld + "next answer"}},
		{"a next message of a third model names the change from the later one", "model-third",
			[]string{"--since", "2026-03-01"},
			[]string{june + "the call", june + "model: model-old -> model-new",
				"2026-07-01T00:00:00.000Z model-third model: model-new -> model-third",
				"2026-07-01T00:00:00.000Z model-third next answer"}},
		{"a window past the later message keeps the next message's model row", "model-new",
			[]string{"--since", "2026-06-15"},
			[]string{julyNew + "model: model-old -> model-new", julyNew + "next answer"}},
		{"a window that keeps only the earlier message keeps the calls there", "model-new",
			[]string{"--until", "2026-03-01"},
			[]string{january + "model: model-old", january + "old answer", january + "the call"}},
		{"a pattern only the later model matches moves the calls there", "model-new",
			[]string{"--match", "-> model-new|call"},
			[]string{june + "the call", june + "model: model-old -> model-new"}},
		{"without a filter a next message of the earlier model needs no model row", "model-old", nil,
			[]string{january + "model: model-old", january + "old answer", january + "the call",
				julyOld + "next answer"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := ollamaMovedModelRun(t, test.next, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}
