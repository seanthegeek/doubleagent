package cody

import (
	"slices"
	"strings"
	"testing"
)

// unkeyedChat is one chat as a history keeps it, under its date key.
const unkeyedChat = `"Mon, 01 Jan 2024 10:00:00 GMT":{"id":"Mon, 01 Jan 2024 10:00:00 GMT",` +
	`"lastInteractionTimestamp":"Mon, 01 Jan 2024 10:00:00 GMT","interactions":[{"humanMessage":` +
	`{"speaker":"human","text":"explain this"},"assistantMessage":{"speaker":"assistant","text":"ok"}}]}`

func TestAHistoryFromBeforeAccountKeysGivesItsChatsWithNoAccount(t *testing.T) {
	const jetBrains = "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2"
	chatRows := func(heading string) []string { return []string{heading, "explain this", "ok"} }
	keyed := chatRows("chat:  | account=https://x.example-u | interactions=1")
	unkeyed := chatRows("chat:  | interactions=1")
	tests := []struct {
		name, history string
		want          []string
	}{
		{"a history with no account", `{"chat":{` + unkeyedChat + `},"input":["explain this"]}`,
			append(slices.Clone(unkeyed), "typed prompts: entries=1")},
		{"a history moved under an account", `{"https://x.example-u":{"chat":{` + unkeyedChat +
			`},"input":["explain this"]},"chat":{},"input":[]}`,
			append(slices.Clone(keyed), "typed prompts: account=https://x.example-u | entries=1")},
		{"an account-keyed history", `{"https://x.example-u":{"chat":{` + unkeyedChat + `}}}`, keyed},
		{"an account whose key ends in -chat", `{"https://x.example-chat":{"chat":{` + unkeyedChat + `}}}`,
			chatRows("chat:  | account=https://x.example-chat | interactions=1")},
		{"an input of another kind", `{"chat":{` + unkeyedChat + `},"input":"x"}`,
			append(slices.Clone(unkeyed), "parser: input is not a JSON array")},
		{"a member that is neither an account nor the input", `{"https://x.example-u":{"chat":{` + unkeyedChat +
			`}},"other":[1]}`, append(keyed, "parser: chat history holds 1 entry that is not a JSON object")},
		{"a chat with no account that is not an object", `{"chat":"oops","input":[]}`,
			[]string{"parser: chat is not a JSON object"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseFile(t, jetBrains, []byte(test.history)); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
		t.Run(test.name+", damaged", func(t *testing.T) {
			got := parseFile(t, jetBrains, []byte(strings.TrimSuffix(test.history, "}")))
			if len(got) == 0 || !strings.HasPrefix(got[len(got)-1], "parser: chat history did not parse") {
				t.Fatalf("rows %q, want the recovered history's problem last", got)
			}
			// The recovery rebuilds a damaged history chat by chat, so its input lists give no row.
			if want := slices.DeleteFunc(slices.Clone(test.want), func(text string) bool {
				return strings.HasPrefix(text, "parser: ") || strings.HasPrefix(text, "typed prompts: ")
			}); !slices.Equal(got[:len(got)-1], want) {
				t.Errorf("rows %q, want %q", got[:len(got)-1], want)
			}
		})
	}
}

func TestAChatKeyWithAndWithoutAnAccountKeepsTwoSessions(t *testing.T) {
	history := `{"chat":{` + unkeyedChat + `},"https://x.example-u":{"chat":{` + unkeyedChat + `}}}`
	var sessions []string
	for _, row := range parseRows(t, "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2", []byte(history)) {
		if !slices.Contains(sessions, row.SessionID) {
			sessions = append(sessions, row.SessionID)
		}
	}
	want := []string{"Mon, 01 Jan 2024 10:00:00 GMT", "https://x.example-u/Mon, 01 Jan 2024 10:00:00 GMT"}
	if !slices.Equal(sessions, want) {
		t.Errorf("sessions %q, want %q", sessions, want)
	}
}
