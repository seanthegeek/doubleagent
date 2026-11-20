package agentzero

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// repeatedIDChat returns a chat.json whose history holds a user message of
// size bytes under the id "u" and a tool result kept in messages/big.txt
// under "t", and whose log names both ids pairs times.
func repeatedIDChat(pairs, size int) string {
	history := jsonvalue.NewObject(
		jsonvalue.Member{Key: "_cls", Value: jsonvalue.NewString("History")},
		jsonvalue.Member{Key: "current", Value: decodedOrNull(`{"_cls":"Topic","messages":[` +
			`{"_cls":"Message","id":"u","ai":false,"content":{"user_message":"` + strings.Repeat("U", size) + `"}},` +
			`{"_cls":"Message","id":"t","ai":false,"content":{"tool_name":"x","file":"big.txt"}}]}`)})
	pair := `{"type":"user","id":"u"},{"type":"tool","id":"t"},`
	return `{"id":"ctx","agents":[{"number":0,"history":` + jsonvalue.NewString(history.JSON()).JSON() +
		`}],"log":{"logs":[` + strings.TrimSuffix(strings.Repeat(pair, pairs), ",") + `]}}`
}

// decodedOrNull returns the JSON value of text, null when it is not JSON.
func decodedOrNull(text string) jsonvalue.Value {
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		return jsonvalue.NewNull()
	}
	return value
}

func TestAHistoryMessageJoinsOnlyTheFirstLogItemNamingIt(t *testing.T) {
	const size = 100000
	for _, pairs := range []int{100, 400} {
		t.Run(fmt.Sprintf("%d pairs", pairs), func(t *testing.T) {
			home := t.TempDir()
			const rel = "agent-zero/usr/chats/ctx/chat.json"
			chatText := repeatedIDChat(pairs, size)
			writeFile(t, home, rel, chatText)
			writeFile(t, home, "agent-zero/usr/chats/ctx/messages/big.txt", strings.Repeat("F", size))
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/h/" + rel,
				Rel: rel, Home: &input.Home{DiskPath: home}}
			subject := New()
			subject.Work = parser.CountingWork()
			var rows []timeline.Row
			if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
				t.Fatal(err)
			}
			text := 0
			for _, row := range rows[1:] {
				text += len(row.Text)
			}
			t.Logf("%d pairs: chat.json %d bytes and a %d-byte message file; row text %d bytes, %d file reads",
				pairs, len(chatText), size, text, subject.Work.Count("message files read"))
			// The first user row and the first tool result hold the texts,
			// each tool call its empty arguments, "{}".
			if want := 2*size + 2*pairs; text != want || len(rows) != 1+3*pairs {
				t.Errorf("%d rows after the start hold %d bytes of text, want %d rows and %d bytes",
					len(rows)-1, text, 3*pairs, want)
			}
			if reads := subject.Work.Count("message files read"); reads != 1 {
				t.Errorf("the message file was read %d times, want once", reads)
			}
			last := rows[len(rows)-1]
			if last.TurnType != timeline.TurnToolResult || last.ToolUseID != "t" || last.Text != "" {
				t.Errorf("the last row is %+v, want an empty result of t", last)
			}
		})
	}
}

func TestAMessageFileNamedTwiceIsReadOnce(t *testing.T) {
	directory := t.TempDir()
	writeFile(t, directory, "chats/ctx/messages/1.txt", "kept")
	messages, ok := parser.DirectoryAt(filepath.Join(directory, "chats", "ctx", "messages"))
	if !ok {
		t.Fatal("the messages directory was refused")
	}
	work := parser.CountingWork()
	state := &chat{messages: messages, work: work}
	var got []string
	for _, name := range []string{"1.txt", `x\1.txt`, "missing.txt", "missing.txt"} {
		content := jsonvalue.NewObject(jsonvalue.Member{Key: "file", Value: jsonvalue.NewString(name)})
		got = append(got, state.messageFile(content))
	}
	if want := []string{"kept", "", "", ""}; !slices.Equal(got, want) || work.Count("message files read") != 1 {
		t.Errorf("got %q after %d reads, want %q after one", got, work.Count("message files read"), want)
	}
}
