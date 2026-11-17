package agentzero

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// writeFile writes text to the slash-separated rel below root, making its
// directories.
func writeFile(t *testing.T, root, rel, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// telegramTexts returns the row texts of a Telegram state file naming one
// context id, with the files of outside written in the home first.
func telegramTexts(t *testing.T, contextID string, outside []string) []string {
	t.Helper()
	home := t.TempDir()
	const rel = "plugins/_telegram_integration/state.json"
	state := pyjson.NewObject(pyjson.Member{Key: "bot:user:chat", Value: pyjson.NewString(contextID)})
	writeFile(t, home, rel, `{"chats": `+pyjson.Compact(state)+`}`)
	for _, name := range outside {
		writeFile(t, home, name, `{"id": "x"}`)
	}
	artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)),
		Original: "/alice/" + rel, Rel: rel, Home: &input.Home{DiskPath: home, User: "alice"}}
	var texts []string
	err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) })
	if err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestAContextIDThatIsNotAFileNameIsADeletedChat(t *testing.T) {
	tests := []struct {
		name      string
		contextID string
		outside   []string
		deleted   bool
	}{
		{"an id of .. reads nothing above the chats", "..", []string{"chat.json", ".json"}, true},
		{"an id climbing out of chats reads nothing there", "../escape",
			[]string{"escape/chat.json", "escape.json"}, true},
		{"an id holding a slash reads nothing below another chat", "a/b",
			[]string{"chats/a/b/chat.json", "chats/a/b.json"}, true},
		{"an id holding a backslash reads nothing", `a\b`, []string{`chats/a\b/chat.json`}, true},
		{"an empty id is skipped as before", "", []string{"chats/chat.json", "chats/.json"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			present := telegramTexts(t, test.contextID, test.outside)
			absent := telegramTexts(t, test.contextID, nil)
			if !slices.Equal(present, absent) {
				t.Errorf("with the outside files %q, without them %q", present, absent)
			}
			if got := len(present) == 1; got != test.deleted {
				t.Errorf("rows %q, want a deleted-session row: %v", present, test.deleted)
			}
		})
	}
}

func TestAMessageFileThatIsNotAFileNameIsNotRead(t *testing.T) {
	directory := t.TempDir()
	messages := filepath.Join(directory, "chats", "ctx", "messages")
	writeFile(t, directory, "chats/ctx/messages/1.txt", "kept")
	writeFile(t, directory, "chats/ctx/x", "outside")
	for _, name := range []string{"..", ".", "x/", "../x", `..\x`, "a\x00b"} {
		t.Run(name, func(t *testing.T) {
			state := &chat{messagesDir: messages, hasMessages: true}
			content := pyjson.NewObject(pyjson.Member{Key: "file", Value: pyjson.NewString(name)})
			got := state.messageFile(content)
			if got != "" || state.failure != nil || len(state.problems) > 0 {
				t.Errorf("got %q, failure %v, problems %v", got, state.failure, state.problems)
			}
		})
	}
	state := &chat{messagesDir: messages, hasMessages: true}
	content := pyjson.NewObject(pyjson.Member{Key: "file", Value: pyjson.NewString(`a\1.txt`)})
	if got := state.messageFile(content); got != "kept" {
		t.Errorf("a plain name after a backslash gave %q", got)
	}
}
