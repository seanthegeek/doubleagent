package agentzero

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	state := jsonvalue.NewObject(jsonvalue.Member{Key: "bot:user:chat", Value: jsonvalue.NewString(contextID)})
	writeFile(t, home, rel, `{"chats": `+state.JSON()+`}`)
	for _, name := range outside {
		writeFile(t, home, name, `{"id": "x"}`)
	}
	artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)),
		Original: "/alice/" + rel, Rel: rel, Home: &input.Home{DiskPath: home, User: "alice"}}
	var texts []string
	err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) })
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
	writeFile(t, directory, "chats/ctx/messages/1.txt", "kept")
	writeFile(t, directory, "chats/ctx/x", "outside")
	messages, ok := parserkit.DirectoryAt(filepath.Join(directory, "chats", "ctx", "messages"))
	if !ok {
		t.Fatal("the messages directory was refused")
	}
	for _, name := range []string{"..", ".", "x/", "../x", `..\x`, "a\x00b"} {
		t.Run(name, func(t *testing.T) {
			state := &chat{messages: messages}
			content := jsonvalue.NewObject(jsonvalue.Member{Key: "file", Value: jsonvalue.NewString(name)})
			got := namedFileText(state, content)
			if got != "" || state.failure != nil || len(state.problems) > 0 {
				t.Errorf("got %q, failure %v, problems %v", got, state.failure, state.problems)
			}
		})
	}
	state := &chat{messages: messages}
	content := jsonvalue.NewObject(jsonvalue.Member{Key: "file", Value: jsonvalue.NewString(`a\1.txt`)})
	if got := namedFileText(state, content); got != "kept" {
		t.Errorf("a plain name after a backslash gave %q", got)
	}
}

func TestAStateFileAtTheShortestPathLooksForChatsTwoLevelsUp(t *testing.T) {
	tests := []struct {
		name    string
		outside []string
		deleted bool
	}{
		{"no chat file gives a deleted-session row", nil, true},
		{"a chat directory hides the row", []string{"chats/ctx/chat.json"}, false},
		{"an older flat chat file hides the row", []string{"chats/ctx.json"}, false},
		{"a chat in another directory does not", []string{"other/chats/ctx/chat.json"}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			texts := telegramTexts(t, "ctx", test.outside)
			want := 0
			if test.deleted {
				want = 1
			}
			if len(texts) != want {
				t.Fatalf("rows %q, want %d", texts, want)
			}
			if test.deleted && texts[0] != "session deleted: telegram bot bot chat chat user user ctxid ctx "+
				"(from state.json, no chat.json in the input, file mtime)" {
				t.Errorf("text %q", texts[0])
			}
		})
	}
}

// connectorTexts returns the row texts of a connector .env holding text.
func connectorTexts(t *testing.T, text string) []string {
	t.Helper()
	home := t.TempDir()
	const rel = ".agent-zero/.env"
	writeFile(t, home, rel, text)
	artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)),
		Original: "/alice/" + rel, Rel: rel, Home: &input.Home{DiskPath: home, User: "alice"}}
	var texts []string
	err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) })
	if err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestTheConnectorEnvGivesItsSettingsAndTheLastContext(t *testing.T) {
	const origin = "session origin: remote agent-zero instance h:5080 context c1 " +
		"(from .agent-zero/.env, file mtime)"
	const tail = " (from .agent-zero/.env, file mtime)"
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"LF lines", "AGENT_ZERO_LAST_CONTEXT_HOST=h:5080\nAGENT_ZERO_LAST_CONTEXT_ID=c1\n",
			[]string{"connector settings: AGENT_ZERO_LAST_CONTEXT_HOST=h:5080 AGENT_ZERO_LAST_CONTEXT_ID=c1" + tail,
				origin}},
		{"CRLF lines with quotes and spaces", " AGENT_ZERO_LAST_CONTEXT_HOST = \"h:5080\"\r\n" +
			"AGENT_ZERO_LAST_CONTEXT_ID='c1'\r\nAGENT_ZERO_TOKEN=secret\r\n",
			[]string{"connector settings: AGENT_ZERO_LAST_CONTEXT_HOST=h:5080 AGENT_ZERO_LAST_CONTEXT_ID=c1 " +
				"AGENT_ZERO_TOKEN=secret" + tail, origin}},
		{"no final LF", "AGENT_ZERO_LAST_CONTEXT_ID=c1\nAGENT_ZERO_LAST_CONTEXT_HOST=h:5080",
			[]string{"connector settings: AGENT_ZERO_LAST_CONTEXT_ID=c1 AGENT_ZERO_LAST_CONTEXT_HOST=h:5080" + tail,
				origin}},
		{"a missing id gives no origin", "AGENT_ZERO_LAST_CONTEXT_HOST=h:5080\n",
			[]string{"connector settings: AGENT_ZERO_LAST_CONTEXT_HOST=h:5080" + tail}},
		{"an empty value gives no origin", "AGENT_ZERO_LAST_CONTEXT_HOST=h:5080\nAGENT_ZERO_LAST_CONTEXT_ID=\n",
			[]string{"connector settings: AGENT_ZERO_LAST_CONTEXT_HOST=h:5080 AGENT_ZERO_LAST_CONTEXT_ID=" + tail}},
		{"a file without a setting gives no row", "# a comment\n=value\n\n", nil},
		{"a commented-out setting is not a setting", "# OLD_KEY=old\n  #AGENT_ZERO_LAST_CONTEXT_ID=c0\n" +
			"AGENT_ZERO_TOKEN=t\n", []string{"connector settings: AGENT_ZERO_TOKEN=t" + tail}},
		{"an export prefix stays in the name, as the connector reads it", "export AGENT_ZERO_TOKEN=t\n",
			[]string{"connector settings: export AGENT_ZERO_TOKEN=t" + tail}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := connectorTexts(t, test.text); !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
