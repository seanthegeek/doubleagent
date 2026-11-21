package agentzero

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// repeatsChat returns a chat.json whose main agent's history holds a user
// message of the id "u" for each prompt, and whose log is logs.
func repeatsChat(prompts []string, logs string) string {
	var messages []string
	for _, prompt := range prompts {
		messages = append(messages, `{"_cls":"Message","id":"u","ai":false,"content":{"user_message":"`+prompt+`"}}`)
	}
	history := `{"_cls":"History","current":{"_cls":"Topic","messages":[` + strings.Join(messages, ",") + `]}}`
	return `{"id":"ctx","agents":[{"number":0,"history":` + jsonvalue.NewString(history).JSON() +
		`}],"log":{"logs":[` + logs + `]}}`
}

// Every history message whose id an earlier message has gives its own rows
// at the time and source line of the log item of the id, after the item's,
// and one problem row per agent counts them; the first message joins the
// item. With no log item of the id, no message of it is shown, and the
// problem row still counts them.
func TestAHistoryMessageRepeatingAnIDGivesItsOwnRowsAndAProblem(t *testing.T) {
	const item = `{"no":7,"type":"user","id":"u","timestamp":1780272000,"content":""}`
	tests := []struct {
		name    string
		prompts []string
		logs    string
		want    []string
	}{
		{"one later message", []string{"FIRST", "LATER"}, item, []string{"user 7 FIRST", "user 7 LATER",
			"system 0 parser: history of agent 0 holds 1 message repeating an earlier message's id; " +
				"each gives its own rows at a log item of the id"}},
		{"two later messages", []string{"FIRST", "SECOND", "THIRD"}, item, []string{"user 7 FIRST",
			"user 7 SECOND", "user 7 THIRD", "system 0 parser: history of agent 0 holds 2 messages repeating " +
				"an earlier message's id; each gives its own rows at a log item of the id"}},
		{"no log item of the id", []string{"FIRST", "LATER"}, "", []string{"system 0 parser: history of " +
			"agent 0 holds 1 message repeating an earlier message's id; each gives its own rows at a log item " +
			"of the id"}},
		{"an id with one message gives no problem", []string{"FIRST"}, item, []string{"user 7 FIRST"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			const rel = "agent-zero/usr/chats/ctx/chat.json"
			writeFile(t, home, rel, repeatsChat(test.prompts, test.logs))
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/h/" + rel,
				Rel: rel, Home: &input.Home{DiskPath: home}}
			var got []string
			err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) {
				if row.Text == "chat start" {
					return
				}
				if row.TurnType == timeline.TurnUser && row.TimestampUTC != "2026-06-01T00:00:00.000Z" {
					t.Errorf("row %q is at %q, want the log item's time", row.Text, row.TimestampUTC)
				}
				got = append(got, row.TurnType+" "+strconv.Itoa(row.SourceLine)+" "+row.Text)
			})
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
