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
// item. With no log item of the id, every message of it gives its rows
// without a time, as a trimmed log's history does.
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
				"each gives its own rows, at a log item of the id or, with no such item, without a time"}},
		{"two later messages", []string{"FIRST", "SECOND", "THIRD"}, item, []string{"user 7 FIRST",
			"user 7 SECOND", "user 7 THIRD", "system 0 parser: history of agent 0 holds 2 messages repeating " +
				"an earlier message's id; each gives its own rows, at a log item of the id or, with no such item, " +
				"without a time"}},
		{"no log item of the id", []string{"FIRST", "LATER"}, "", []string{"user 0 FIRST", "user 0 LATER",
			"system 0 parser: history of agent 0 holds 1 message repeating an earlier message's id; " +
				"each gives its own rows, at a log item of the id or, with no such item, without a time"}},
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
				want := "2026-06-01T00:00:00.000Z"
				if test.logs == "" {
					want = ""
				}
				if row.TurnType == timeline.TurnUser && row.TimestampUTC != want {
					t.Errorf("row %q is at %q, want %q", row.Text, row.TimestampUTC, want)
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

// A later history message of a repeated id holds no place in the chat's
// sequence of models, so its model row names only its own model, never a
// previous one, and the rows after it still name the change the log
// gives.
func TestALaterHistoryMessageNamesOnlyItsOwnModel(t *testing.T) {
	message := func(messageID, reply, model string) string {
		return `{"_cls":"Message","id":"` + messageID + `","ai":true,"content":"` + reply + `",` +
			`"metadata":{"responses":{"provider_model_key":"` + model + `"}}}`
	}
	history := `{"_cls":"History","current":{"_cls":"Topic","messages":[` + message("a", "reply one", "m1") + "," +
		message("b", "reply two", "m2") + "," + message("a", "later reply", "m3") + `]}}`
	chat := `{"id":"ctx","agents":[{"number":0,"history":` + jsonvalue.NewString(history).JSON() +
		`}],"log":{"logs":[{"no":0,"type":"agent","id":"a","timestamp":1000000000,"content":""},` +
		`{"no":1,"type":"agent","id":"b","timestamp":1800000000,"content":""}]}}`
	home := t.TempDir()
	const rel = "agent-zero/usr/chats/ctx/chat.json"
	writeFile(t, home, rel, chat)
	artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/h/" + rel,
		Rel: rel, Home: &input.Home{DiskPath: home}}
	var got []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) {
		if row.TurnType != timeline.TurnSystem || strings.HasPrefix(row.Text, "model: ") {
			got = append(got, row.TimestampUTC[:4]+" "+row.Text)
		}
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{"2001 model: m1", "2001 reply one", "2027 model: m1 -> m2", "2027 reply two",
		"2001 model: m3", "2001 later reply"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
