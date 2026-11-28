package cody

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The accounts and the file of the typed-prompt tests.
const (
	typedFile    = "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2"
	typedAccount = "https://x.example-u"
	typedOther   = "https://y.example-v"
)

// typedChat returns a chat member under key whose one interaction holds the
// human message's members, and the rows it gives in account (none for
// the chats with no account).
func typedChat(key, human, account string) (string, []string) {
	heading := "chat:  | interactions=1"
	if account != "" {
		heading = "chat:  | account=" + account + " | interactions=1"
	}
	return `"` + key + `":{"id":"` + key + `","interactions":[{"humanMessage":{"speaker":"human",` + human + `}}]}`,
		[]string{heading}
}

func TestATypedPromptGivesARowOnlyWhenNoChatOfItsScopeHoldsIt(t *testing.T) {
	t.Parallel()
	accountChat, accountRows := typedChat("c1", `"text":"q1"`, typedAccount)
	accountRows = append(accountRows, "q1")
	unkeyedChat, unkeyedRows := typedChat("c1", `"text":"q1"`, "")
	unkeyedRows = append(unkeyedRows, "q1")
	otherHeading := "typed prompts: account=" + typedOther + " | entries="
	heading := "typed prompts: account=" + typedAccount + " | entries="
	tests := []struct {
		name, history string
		want          []string
	}{
		{"a deleted chat's prompt", `{"` + typedAccount + `":{"chat":{},"input":["delete the audit logs"]}}`,
			[]string{heading + "1", "delete the audit logs"}},
		{"a prompt its account's chat holds", `{"` + typedAccount + `":{"chat":{` + accountChat +
			`},"input":["q1","q2"]}}`, append(slices.Clone(accountRows), heading+"2", "q2")},
		{"a prompt the chats with no account hold", `{"chat":{` + unkeyedChat + `},"input":["q1","q3"]}`,
			append(slices.Clone(unkeyedRows), "typed prompts: entries=2", "q3")},
		{"a prompt with no account against an account's chat", `{"chat":{},"input":["q1"],"` + typedAccount +
			`":{"chat":{` + accountChat + `}}}`, append([]string{"typed prompts: entries=1", "q1"}, accountRows...)},
		{"an account's prompt against the chats with no account", `{"chat":{` + unkeyedChat + `},"input":[],"` +
			typedOther + `":{"chat":{},"input":["q1"]}}`, append(slices.Clone(unkeyedRows), otherHeading+"1", "q1")},
		{"an account's prompt against another account's chat", `{"` + typedAccount + `":{"chat":{` + accountChat +
			`}},"` + typedOther + `":{"chat":{},"input":["q1"]}}`, append(slices.Clone(accountRows), otherHeading+"1", "q1")},
		{"a prompt the assistant's message holds", `{"` + typedAccount + `":{"chat":{"c1":{"id":"c1","interactions":` +
			`[{"assistantMessage":{"speaker":"assistant","text":"q1"}}]}},"input":["q1"]}}`,
			[]string{"chat:  | account=" + typedAccount + " | interactions=1", "q1", heading + "1", "q1"}},
		{"an empty list", `{"` + typedAccount + `":{"chat":{},"input":[]}}`, nil},
		{"a prompt of white space", `{"` + typedAccount + `":{"chat":{},"input":[" "]}}`, []string{heading + "1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := parseFile(t, typedFile, []byte(test.history)); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestATypedPromptMatchesTheTextOrDisplayTextOfOneMessage(t *testing.T) {
	t.Parallel()
	heading := "typed prompts: account=" + typedAccount + " | entries="
	tests := []struct {
		name, human, input string
		want               []string
	}{
		{"the text", `"text":"q1"`, `["q1"]`, []string{"q1", heading + "1"}},
		{"the displayText of a search", `"text":"","displayText":"/s needle"`, `["/s needle"]`, []string{heading + "1"}},
		{"the displayText of a cut text", `"text":"long que","displayText":"long question"`, `["long question"]`,
			[]string{"long que", heading + "1"}},
		{"neither", `"text":"q1","displayText":"q1 shown"`, `["q2"]`, []string{"q1", heading + "1", "q2"}},
		{"a text differing in case or space", `"text":"q1"`, `["Q1"," q1"]`, []string{"q1", heading + "2", "Q1", "q1"}},
		{"the same prompt twice with one message", `"text":"q1"`, `["q1","q1"]`, []string{"q1", heading + "2", "q1"}},
		{"both forms of one message", `"text":"long que","displayText":"long question"`, `["long que","long question"]`,
			[]string{"long que", heading + "2", "long question"}},
		{"an object entry", `"text":"q1"`, `[{"inputText":"q1","inputContextFiles":[{"uri":{"fsPath":"/srv/proj/a.py"}}]}]`,
			[]string{"q1", heading + "1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			chat, rows := typedChat("c1", test.human, typedAccount)
			history := `{"` + typedAccount + `":{"chat":{` + chat + `},"input":` + test.input + `}}`
			want := append(rows, test.want...)
			if got := parseFile(t, typedFile, []byte(history)); !slices.Equal(got, want) {
				t.Errorf("rows %q, want %q", got, want)
			}
		})
	}
}

func TestEachMessageOfAScopeTakesOneTypedPromptInListOrder(t *testing.T) {
	t.Parallel()
	first, _ := typedChat("c1", `"text":"q"`, typedAccount)
	second, _ := typedChat("c2", `"text":"q"`, typedAccount)
	history := `{"` + typedAccount + `":{"chat":{` + first + "," + second + `},"input":["b","q","q","q","a"]}}`
	want := []string{"chat:  | account=" + typedAccount + " | interactions=1", "q",
		"chat:  | account=" + typedAccount + " | interactions=1", "q",
		"typed prompts: account=" + typedAccount + " | entries=5", "b", "q", "a"}
	if got := parseFile(t, typedFile, []byte(history)); !slices.Equal(got, want) {
		t.Errorf("rows %q, want %q", got, want)
	}
}

func TestATypedPromptListGivesStringAndObjectEntries(t *testing.T) {
	t.Parallel()
	heading := "typed prompts: account=" + typedAccount + " | entries="
	tests := []struct {
		name, input string
		want        []string
	}{
		{"strings", `["s1","s2"]`, []string{heading + "2", "s1", "s2"}},
		{"objects", `[{"inputText":"o1","inputContextFiles":[]},{"inputText":"o2","inputContextFiles":` +
			`[{"uri":{"fsPath":"/srv/proj/a.py"}},{"uri":{"path":"/srv/proj/b.py"}}]}]`,
			[]string{heading + "2", "o1", "o2\n[context: /srv/proj/a.py, /srv/proj/b.py]"}},
		{"a mixed list", `["s1",{"inputText":"o1"},"s2"]`, []string{heading + "3", "s1", "o1", "s2"}},
		{"an object with no text", `[{"inputContextFiles":[{"uri":{"fsPath":"/srv/proj/a.py"}}]}]`,
			[]string{heading + "1"}},
		{"a list that is not an array", `"x"`, []string{"parser: input is not a JSON array"}},
		{"entries of another kind", `["s1",3,null]`,
			[]string{heading + "3", "s1", "parser: input holds 2 entries that are not JSON strings"}},
		{"a text that is not a string", `[{"inputText":5},{"inputText":"o1","inputContextFiles":"x"}]`,
			[]string{heading + "2", "o1", "parser: inputText is not a JSON string",
				"parser: inputContextFiles is not a JSON array"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			history := `{"` + typedAccount + `":{"chat":{},"input":` + test.input + `}}`
			if got := parseFile(t, typedFile, []byte(history)); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestATypedPromptRowIsRecoveredWithNoTimeOrSession(t *testing.T) {
	t.Parallel()
	chat, _ := typedChat("Mon, 01 Jan 2024 10:00:00 GMT", `"text":"q1"`, typedAccount)
	history := `{"` + typedAccount + `":{"chat":{` + chat + `},"input":["q2"]}}`
	rows := parseRows(t, typedFile, []byte(history))
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4", len(rows))
	}
	for i, row := range rows {
		wantRecovered := map[bool]string{true: "yes", false: ""}[i == 3]
		if row.Recovered != wantRecovered || row.SourceLine != 1 {
			t.Errorf("row %d %q: recovered %q line %d, want %q line 1", i, row.Text, row.Recovered, row.SourceLine,
				wantRecovered)
		}
	}
	for _, row := range rows[2:] {
		if row.TimestampUTC != "" || row.SessionID != "" {
			t.Errorf("row %q: time %q session %q, want neither", row.Text, row.TimestampUTC, row.SessionID)
		}
	}
	if rows[3].TurnType != "user" || rows[2].TurnType != "system" {
		t.Errorf("turn types %q and %q, want system and user", rows[2].TurnType, rows[3].TurnType)
	}
}

// typedWork parses a history of n chats whose one message is typed twice,
// once in each of its forms, and returns the messages looked at and the
// recovered rows.
func typedWork(t *testing.T, n int) (looked, recovered int) {
	t.Helper()
	var chats, typed []string
	for i := range n {
		chat, _ := typedChat("c"+strconv.Itoa(i), `"text":"q","displayText":"q shown"`, typedAccount)
		chats = append(chats, chat)
		typed = append(typed, `"q"`, `"q shown"`)
	}
	history := `{"` + typedAccount + `":{"chat":{` + strings.Join(chats, ",") + `},"input":[` +
		strings.Join(typed, ",") + `]}}`
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(typedFile))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(history), 0o600); err != nil {
		t.Fatal(err)
	}
	subject := New()
	subject.Work = parserkit.CountingWork()
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + typedFile, Rel: typedFile,
		Home: &input.Home{DiskPath: home}}
	if err := subject.Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
		if row.Recovered == "yes" {
			recovered++
		}
	}); err != nil {
		t.Fatal(err)
	}
	return subject.Work.Count("typed claims looked at"), recovered
}

// Prompts typed again and again take their messages from one queue per
// text, so the messages looked at grow with the list, not with the list
// times the messages holding one text.
func TestTypedPromptsLookAtEachMessageOnce(t *testing.T) {
	t.Parallel()
	for _, n := range []int{8, 64} {
		if looked, recovered := typedWork(t, n); looked > 2*n || recovered != n {
			t.Errorf("%d messages: looked at %d, recovered %d rows; want at most %d and %d", n, looked, recovered,
				2*n, n)
		}
	}
}
