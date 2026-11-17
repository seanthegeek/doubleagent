package geminicli

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// A session file is evidence: whatever its records hold, the replay and
// the rows of the messages it leaves must not panic.
func FuzzSessionReplay(fuzz *testing.F) {
	for _, seed := range []string{
		`{"sessionId":"s","projectHash":"h","startTime":"2026-10-01T09:00:00Z","kind":"main"}` + "\n" +
			`{"id":"u1","type":"user","content":[{"text":"<hook_context>x</hook_context>hi"}]}` + "\n" +
			`{"id":"g1","type":"gemini","content":[{"functionCall":{"id":"c","args":{}}}],` +
			`"toolCalls":[{"id":"c","status":"cancelled","result":[{"functionResponse":{"id":"c"}}]}]}`,
		`{"$patch":{"id":"g1","toolCalls":[{"id":"c","result":"r"}],"removeIds":["u1"],"orderIds":["g1"]}}`,
		`{"$set":{"messages":[{"id":"a"}],"summary":"s"}}` + "\n" + `{"$rewindTo":"zz"}`,
	} {
		fuzz.Add(seed, true)
	}
	fuzz.Fuzz(func(t *testing.T, text string, thinking bool) {
		state := newReplay()
		var problems parser.Problems
		for line, record := range parser.JSONRecords(parser.SplitLines([]byte(text)), &problems) {
			state.apply(line, record)
		}
		models := parser.Tracker{}
		answered := answeredCalls(&state.messages)
		for _, messageID := range state.messages.order {
			entry := state.messages.byID[messageID]
			turn := messageTurn{owner: New(), artifact: &input.Artifact{Home: &input.Home{}},
				options: parser.Options{IncludeThinking: thinking}, line: entry.line}
			if len(turn.rows(entry.message, models, answered)) == 0 {
				t.Errorf("message %q gave no row", messageID)
			}
		}
	})
}

// A shell history or a checkpoint's tag is evidence too.
func FuzzShellHistoryAndTags(fuzz *testing.F) {
	for _, seed := range []string{"ls\\\n-la\n\xff\n", "a%20b%e2%82%zz", "%"} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		var problems parser.Problems
		for _, entry := range shellHistoryEntries([]byte(text), &problems) {
			if entry.line < 1 {
				t.Errorf("command %q at line %d", entry.command, entry.line)
			}
		}
		_ = unquote(text)
		historyItems(text, &problems)
	})
}
