package agentzero

import (
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// decodedOrNull returns the JSON value of text, null when it is not JSON.
func decodedOrNull(text string) jsonvalue.Value {
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		return jsonvalue.NewNull()
	}
	return value
}

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

// namedFileText returns the text of the message file a history message's
// content names, as a log item of it reaches the file (groupKeys and
// historyFile): "" when it names none.
func namedFileText(state *chat, content jsonvalue.Value) string {
	ref, named := state.messageFileRef(content)
	if !named {
		return ""
	}
	return state.messageFileText(ref)
}

// asWritten returns rows as a run without a filter writes them: each group
// of a repeated id with its first parent (timeline.Attach), and the shared
// texts written out (timeline.Resolve).
func asWritten(rows []timeline.Row) []timeline.Row {
	rows = timeline.Attach(rows, nil)
	timeline.Resolve(rows)
	return rows
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
			subject.Work = parserkit.CountingWork()
			var rows []timeline.Row
			if err := subject.Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
				t.Fatal(err)
			}
			rows = asWritten(rows)
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

// A message file two names open is read once, and both are given its
// text; which log item shows it is the group's choice (joinedRow).
func TestAMessageFileNamedTwiceIsReadOnce(t *testing.T) {
	directory := t.TempDir()
	writeFile(t, directory, "chats/ctx/messages/1.txt", "kept")
	messages, ok := parserkit.DirectoryAt(&input.Artifact{}, filepath.Join(directory, "chats", "ctx", "messages"))
	if !ok {
		t.Fatal("the messages directory was refused")
	}
	work := parserkit.CountingWork()
	state := &chat{messages: messages, work: work}
	var got []string
	for _, name := range []string{"1.txt", `x\1.txt`, "missing.txt", "missing.txt"} {
		content := jsonvalue.NewObject(jsonvalue.Member{Key: "file", Value: jsonvalue.NewString(name)})
		got = append(got, namedFileText(state, content))
	}
	if want := []string{"kept", "kept", "", ""}; !slices.Equal(got, want) || work.Count("message files read") != 1 {
		t.Errorf("got %q after %d reads, want %q after one", got, work.Count("message files read"), want)
	}
}

// joinChat returns a chat.json whose history holds a user message "u" and
// a tool message "t", each with its text in the history, and whose log is
// logs.
func joinChat(logs string) string {
	history := `{"_cls":"History","current":{"_cls":"Topic","messages":[` +
		`{"_cls":"Message","id":"u","ai":false,"content":{"user_message":"HISTORY-USER-TEXT"}},` +
		`{"_cls":"Message","id":"t","ai":false,"content":{"tool_name":"x","tool_result":"HISTORY-TOOL-RESULT"}}]}}`
	return `{"id":"ctx","agents":[{"number":0,"history":` + jsonvalue.NewString(history).JSON() +
		`}],"log":{"logs":[` + logs + `]}}`
}

// A history message's text joins the first log item of its id that shows
// it, so an item of another kind naming the id first, which shows only its
// own content, leaves the text to the next; the text is in exactly one
// row, and every tool item of the id carries the history's tool name.
func TestAHistoryMessageJoinsTheFirstLogItemThatShowsIt(t *testing.T) {
	tests := []struct {
		name, logs string
		want       []string
	}{
		{"a response naming a user message first leaves its text to the user item",
			`{"type":"response","id":"u","content":"r"},{"type":"user","id":"u","content":"own"}`,
			[]string{"assistant  r", "user  HISTORY-USER-TEXT"}},
		{"an info item naming a tool message first leaves its result to the tool item",
			`{"type":"info","id":"t","content":"i"},{"type":"tool","id":"t","content":"own"}`,
			[]string{"system  info: i", "tool_use x {}", "tool_result x HISTORY-TOOL-RESULT"}},
		{"a later item of the id shows its own content with the tool name",
			`{"type":"tool","id":"t","content":"first"},{"type":"tool","id":"t","content":"own"}`,
			[]string{"tool_use x {}", "tool_result x HISTORY-TOOL-RESULT", "tool_use x {}", "tool_result x own"}},
		{"a tool item naming a user message leaves its text to the user item",
			`{"type":"tool","id":"u","content":"own"},{"type":"user","id":"u","content":"later"}`,
			[]string{"tool_use tool {}", "tool_result tool own", "user  HISTORY-USER-TEXT"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			const rel = "agent-zero/usr/chats/ctx/chat.json"
			writeFile(t, home, rel, joinChat(test.logs))
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/h/" + rel,
				Rel: rel, Home: &input.Home{DiskPath: home}}
			var rows []timeline.Row
			if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, row := range asWritten(rows) {
				if row.TurnType != timeline.TurnSystem || !strings.HasPrefix(row.Text, "chat ") {
					got = append(got, row.TurnType+" "+row.ToolName+" "+row.Text)
				}
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// replyChat returns a chat.json whose history holds an assistant reply
// "a", a JSON object of about size bytes that names no headline or
// thoughts, and whose log names it in items agent items, each with kvps.
func replyChat(items, size int, kvps string) string {
	reply := jsonvalue.NewString(`{"tool_name":"","pad":"` + strings.Repeat("X", size) + `"}`).JSON()
	history := `{"_cls":"History","current":{"_cls":"Topic","messages":[` +
		`{"_cls":"Message","id":"a","ai":true,"content":` + reply + `}]}}`
	item := `{"type":"agent","id":"a","content":"own","kvps":` + kvps + `},`
	return `{"id":"ctx","agents":[{"number":0,"history":` + jsonvalue.NewString(history).JSON() +
		`}],"log":{"logs":[` + strings.TrimSuffix(strings.Repeat(item, items), ",") + `]}}`
}

// An assistant reply that no agent item shows (it has no headline, or the
// items carry their own) is read as the agent's JSON response at most once
// for its id, however many items name it, and not at all while the items
// carry their own headline: the decodes are counted, and so are the bytes
// the parse allocates, which grow with the items, not with the items times
// the reply.
func TestAHistoryReplyIsDecodedOncePerID(t *testing.T) {
	const size = 50_000
	tests := []struct {
		name, kvps string
		decodes    int
	}{
		{"items without a headline read the reply once", `{}`, 1},
		{"items with their own headline never read it", `{"headline":"own headline"}`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allocated := map[int]int64{}
			for _, items := range []int{20, 80} {
				home := t.TempDir()
				const rel = "agent-zero/usr/chats/ctx/chat.json"
				writeFile(t, home, rel, replyChat(items, size, test.kvps))
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)),
					Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
				subject := New()
				subject.Work = parserkit.CountingWork()
				rows := 0
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				if err := subject.Parse(artifact, parserkit.Options{}, func(timeline.Row) { rows++ }); err != nil {
					t.Fatal(err)
				}
				runtime.ReadMemStats(&after)
				allocated[items] = int64(after.TotalAlloc - before.TotalAlloc)
				if decodes := subject.Work.Count("history replies decoded"); decodes != test.decodes || rows != items+1 {
					t.Errorf("%d items gave %d rows after %d decodes, want %d rows after %d", items, rows,
						decodes, items+1, test.decodes)
				}
			}
			// One more decode per item would add 60 copies of the reply.
			if growth := allocated[80] - allocated[20]; growth > 10*size {
				t.Errorf("60 more items allocated %d more bytes, more than ten copies of the %d-byte reply",
					growth, size)
			}
		})
	}
}

// subordinateChat returns a chat.json whose history holds a user message
// "u" of size bytes, and whose log names it in items user items of
// agent 1, each with an attachment.
func subordinateChat(items, size int) string {
	history := `{"_cls":"History","current":{"_cls":"Topic","messages":[` +
		`{"_cls":"Message","id":"u","ai":false,"content":{"user_message":"` + strings.Repeat("U", size) + `"}}]}}`
	item := `{"type":"user","id":"u","agentno":1,"content":"own","kvps":{"attachments":["a.txt"]}},`
	return `{"id":"ctx","agents":[{"number":0,"history":` + jsonvalue.NewString(history).JSON() +
		`}],"log":{"logs":[` + strings.TrimSuffix(strings.Repeat(item, items), ",") + `]}}`
}

// Every item of an id that shows its history message holds the text it
// shows with it, for the filters to choose from; the text, led by the
// agent and followed by the attachments, is built once for the id, so the
// bytes the parse allocates grow with the items, not with the items times
// the message, and before the filters choose only the first item shows it.
func TestAHistoryTextIsBuiltOncePerIDForAllItsItems(t *testing.T) {
	const size = 50_000
	allocated := map[int]int64{}
	for _, items := range []int{20, 80} {
		home := t.TempDir()
		const rel = "agent-zero/usr/chats/ctx/chat.json"
		writeFile(t, home, rel, subordinateChat(items, size))
		artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)),
			Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
		var rows []timeline.Row
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
			t.Fatal(err)
		}
		runtime.ReadMemStats(&after)
		allocated[items] = int64(after.TotalAlloc - before.TotalAlloc)
		var texts []string
		for _, row := range asWritten(rows) {
			texts = append(texts, row.Text)
		}
		want := "[agent 1] " + strings.Repeat("U", size) + " [attachments: a.txt]"
		if len(texts) != items+1 || texts[1] != want || texts[2] != "[agent 1] own [attachments: a.txt]" {
			t.Errorf("%d items gave %d rows, want %d, the first with the history text", items, len(texts), items+1)
		}
	}
	// Building the text again for each item would add 60 copies of it.
	if growth := allocated[80] - allocated[20]; growth > 10*size {
		t.Errorf("60 more items allocated %d more bytes, more than ten copies of the %d-byte message", growth, size)
	}
}
