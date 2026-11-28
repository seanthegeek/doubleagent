package cody

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// holderChat is one chat of a test history: its date key, as the index
// chatDate takes, its title, and an interaction placed after its first,
// which a damaged first leaves unread.
type holderChat struct {
	date         int
	title, after string
}

// holderMember is a top-level member of a test history: "chat" for the
// history's own chats, "input" for its prompt list, else an account key.
type holderMember struct {
	key   string
	chats []holderChat
}

// holderHistory returns a history's text, the interaction of the chat
// titled damaged written so that it does not decode.
func holderHistory(members []holderMember, damaged string) string {
	chatsText := func(chats []holderChat) string {
		var texts []string
		for _, chat := range chats {
			interaction := `{"humanMessage":{"speaker":"human","text":"hi ` + chat.title + `"}}`
			if chat.title == damaged {
				interaction = `{"humanMessage": ]}`
			}
			if chat.after != "" {
				interaction += "," + chat.after
			}
			texts = append(texts, `"`+chatDate(chat.date)+`":{"id":"`+chatDate(chat.date)+`","chatTitle":"`+
				chat.title+`","interactions":[`+interaction+`]}`)
		}
		return "{" + strings.Join(texts, ",") + "}"
	}
	var texts []string
	for _, member := range members {
		switch member.key {
		case "chat":
			texts = append(texts, `"chat":`+chatsText(member.chats))
		case "input":
			texts = append(texts, `"input":["p"]`)
		default:
			texts = append(texts, `"`+member.key+`":{"chat":`+chatsText(member.chats)+`,"input":[]}`)
		}
	}
	return "{" + strings.Join(texts, ",") + "}"
}

// holderOutcome is what a parse of a history gives that a recovery must
// keep: each chat's heading less its interaction count and its session id,
// the session ids of the damaged-record rows, and the count of chats the
// recovery message gives, -1 for a history that decodes.
type holderOutcome struct {
	headings, sessions, recordSessions []string
	recovered                          int
}

// recoveredCount reads the count of a recovery message.
var recoveredCount = regexp.MustCompile(`recovered ([0-9]+) chat\(s\)`)

// outcomeOf returns the holderOutcome of a parse's rows; message is the
// recovery message when the rows lack it.
func outcomeOf(rows []timeline.Row, message string) holderOutcome {
	outcome := holderOutcome{recovered: -1}
	for _, row := range rows {
		switch {
		case strings.HasPrefix(row.Text, "chat: "):
			heading, _, _ := strings.Cut(row.Text, " | interactions=")
			outcome.headings = append(outcome.headings, heading)
			outcome.sessions = append(outcome.sessions, row.SessionID)
		case strings.HasPrefix(row.Text, "parser: unparseable record"):
			outcome.recordSessions = append(outcome.recordSessions, row.SessionID)
		case strings.HasPrefix(row.Text, "parser: chat history did not parse"):
			message = row.Text
		}
	}
	if found := recoveredCount.FindStringSubmatch(message); found != nil {
		outcome.recovered, _ = strconv.Atoi(found[1])
	}
	return outcome
}

// stateOutcome returns the holderOutcome of a state value's text, read as
// stateDatabase reads a value that decodes and one that does not.
func stateOutcome(t *testing.T, text string) holderOutcome {
	t.Helper()
	artifact := &input.Artifact{Original: "/h/state.vscdb", Rel: "state.vscdb", Home: &input.Home{DiskPath: t.TempDir()}}
	parser := New()
	var rows []timeline.Row
	emit := func(row timeline.Row) { rows = append(rows, row) }
	var problems parserkit.Problems
	state, err := parserkit.DecodeJSON(artifact, text)
	if err == nil {
		parser.historyRows(artifact, state.Get(historyKey), 0, parserkit.Options{}, emit, &problems)
		return outcomeOf(rows, "")
	}
	history, message, recovered := recoverHistory(artifact, text)
	parser.historyRows(artifact, history, 0, parserkit.Options{}, emit, &problems)
	parser.recoveredRows(artifact, recovered, emit)
	return outcomeOf(rows, message)
}

// holderHistories returns the histories whose chats a recovery must place
// as the whole parse does, by name.
func holderHistories() map[string][]holderMember {
	one, two, three := holderChat{1, "one", ""}, holderChat{2, "two", ""}, holderChat{3, "three", ""}
	sameTop, sameOther := holderChat{1, "two", ""}, holderChat{1, "three", ""}
	account, other := "https://x.example-u", "https://y.example-v"
	histories := map[string][]holderMember{
		"the history's own chat before an account": {{"chat", []holderChat{two}}, {"input", nil},
			{account, []holderChat{one}}},
		"the history's own chat after an account": {{account, []holderChat{one}}, {"chat", []holderChat{two}},
			{"input", nil}},
		"the history's own chat between two accounts": {{account, []holderChat{one}}, {"chat", []holderChat{two}},
			{"input", nil}, {other, []holderChat{three}}},
		"no chat of the history's own":          {{account, []holderChat{one}}, {other, []holderChat{three}}},
		"an empty chat of its own after":        {{account, []holderChat{one}}, {"chat", nil}, {"input", nil}},
		"an empty chat of its own before":       {{"chat", nil}, {"input", nil}, {account, []holderChat{one}}},
		"the history's own chat alone":          {{"chat", []holderChat{two}}, {"input", nil}},
		"one key in its own chat, then account": {{"chat", []holderChat{sameTop}}, {account, []holderChat{one}}},
		"one key in an account, then its own":   {{account, []holderChat{one}}, {"chat", []holderChat{sameTop}}},
		"one key under two accounts":            {{account, []holderChat{one}}, {other, []holderChat{sameOther}}},
		"one key in two accounts and its own": {{account, []holderChat{one}}, {"chat", []holderChat{sameTop}},
			{other, []holderChat{sameOther}}},
	}
	// An interaction can hold an object member named "chat" anywhere: a
	// mention's data is stored as its provider gave it.
	nested := map[string]string{
		"a chat member after another member":           `{"type":"openctx","mention":{"uri":"u","data":{"kind":"k","chat":{"id":1}}}}`,
		"a chat member first in an object in an array": `{"type":"openctx","mention":{"uri":"u","data":[{"chat":{"id":1}}]}}`,
		"a chat member inside editorState":             `{"type":"file","uri":"u"}],"editorState":{"v":1,"chat":{"id":1}},"x":[0`,
	}
	for shape, item := range nested {
		after := `{"humanMessage":{"speaker":"human","text":"q","contextFiles":[` + item + `]}}`
		holding := []holderChat{{1, "one", after}, {2, "two", ""}}
		top := []holderChat{{2, "top", ""}}
		histories[shape+" in an account's chat"] = []holderMember{{account, holding}}
		histories[shape+", the history's own chat before"] = []holderMember{{"chat", top}, {account, holding}}
		histories[shape+", the history's own chat after"] = []holderMember{{account, holding}, {"chat", top}}
	}
	return histories
}

func TestARecoveredChatKeepsTheAccountAndSessionOfTheWholeParse(t *testing.T) {
	const jetBrains = "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2"
	stores := []struct {
		name    string
		wrap    func(history string) string
		outcome func(t *testing.T, text string) holderOutcome
	}{
		{"a JetBrains file", func(history string) string { return history },
			func(t *testing.T, text string) holderOutcome {
				return outcomeOf(parseRows(t, jetBrains, []byte(text)), "")
			}},
		{"a state value", func(history string) string { return `{"` + historyKey + `":` + history + `}` }, stateOutcome},
	}
	for name, members := range holderHistories() {
		for _, store := range stores {
			whole := store.outcome(t, store.wrap(holderHistory(members, "")))
			if whole.recovered != -1 || len(whole.recordSessions) != 0 {
				t.Fatalf("%s, %s: the whole history did not decode: %+v", name, store.name, whole)
			}
			damages := map[string]string{"the last brace cut": ""} // by the title of the damaged chat
			for _, member := range members {
				if len(member.chats) > 0 {
					damages["a damaged interaction in "+member.key] = member.chats[0].title
				}
			}
			for damage, title := range damages {
				t.Run(name+", "+store.name+", "+damage, func(t *testing.T) {
					t.Parallel()
					text := store.wrap(holderHistory(members, title))
					var want []string
					if title == "" {
						text = strings.TrimSuffix(text, "}")
					} else {
						index := slices.IndexFunc(whole.headings, func(heading string) bool {
							return heading == "chat: "+title || strings.HasPrefix(heading, "chat: "+title+" | ")
						})
						if index < 0 {
							t.Fatalf("no chat titled %q among the whole history's chats %q", title, whole.headings)
						}
						want = []string{whole.sessions[index]}
					}
					got := store.outcome(t, text)
					if !slices.Equal(got.headings, whole.headings) || !slices.Equal(got.sessions, whole.sessions) {
						t.Errorf("chats %q in sessions %q\nwant %q in sessions %q", got.headings, got.sessions,
							whole.headings, whole.sessions)
					}
					if got.recovered != len(got.headings) {
						t.Errorf("the message says %d chat(s) were recovered; %d give rows", got.recovered, len(got.headings))
					}
					if !slices.Equal(got.recordSessions, want) {
						t.Errorf("the damaged records carry sessions %q, want %q", got.recordSessions, want)
					}
				})
			}
		}
	}
}
