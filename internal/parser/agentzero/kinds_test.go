package agentzero

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseProblems writes text at rel under a new home, parses it and returns
// the texts of the rows that report a problem.
func parseProblems(t *testing.T, rel, text string) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
	var problems []string
	emit := func(row timeline.Row) {
		if strings.HasPrefix(row.Text, parser.ProblemPrefix) {
			problems = append(problems, row.Text)
		}
	}
	if err := New().Parse(artifact, parser.Options{}, emit); err != nil {
		t.Fatal(err)
	}
	return problems
}

func TestAChatMemberOfTheWrongKindIsReported(t *testing.T) {
	const rel = "agent-zero/usr/chats/ctx/chat.json"
	tests := []struct {
		name, members string
		want          []string
	}{
		{"log.logs as an object", `"log": {"logs": {"0": {"type": "user", "content": "hi"}}}`,
			[]string{"parser: log.logs is not a JSON array"}},
		{"log.logs as a string", `"log": {"logs": "x"}`, []string{"parser: log.logs is not a JSON array"}},
		{"log.logs as a number", `"log": {"logs": 4}`, []string{"parser: log.logs is not a JSON array"}},
		{"log as a list", `"log": []`, []string{"parser: log is not a JSON object"}},
		{"agents as an object", `"agents": {"number": 0}`, []string{"parser: agents is not a JSON array"}},
		{"agents as a string", `"agents": "x"`, []string{"parser: agents is not a JSON array"}},
		{"log.logs holding entries that are not objects", `"log": {"logs": [1, {"type": "user"}, "x"]}`,
			[]string{"parser: log.logs holds 2 entries that are not JSON objects"}},
		{"agents holding an entry that is not an object", `"agents": [{"number": 0}, [1]]`,
			[]string{"parser: agents holds 1 entry that is not a JSON object"}},
		{"null members", `"log": {"logs": null}, "agents": null`, nil},
		{"empty members", `"log": {"logs": []}, "agents": []`, nil},
		{"absent members", `"log": {}`, nil},
		{"a telegram bot configuration as a string", `"data": {"telegram_bot": "b", "telegram_bot_cfg": "123:x"}`,
			[]string{"parser: telegram_bot_cfg is not a JSON object"}},
		{"a model override as a string", `"data": {"chat_model_override": "gpt"}`,
			[]string{"parser: chat_model_override is not a JSON object"}},
		{"a model override slot as a list", `"data": {"chat_model_override": {"chat": [1]}}`,
			[]string{"parser: chat_model_override.chat is not a JSON object"}},
		{"data as a list", `"data": [1]`, []string{"parser: data is not a JSON object"}},
		{"null data members", `"data": {"telegram_bot_cfg": null, "chat_model_override": {"chat": null}}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseProblems(t, rel, `{"id": "ctx", "name": "n", `+test.members+`}`); !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}

func TestAToolAccessListThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = "agent-zero/usr/plugins/_tool_access/config.json"
	tests := []struct {
		name, policy string
		want         []string
	}{
		{"allowed as a string", `{"allowed": "code_execution"}`, []string{"parser: allowed is not a JSON array"}},
		{"blocked as an object", `{"blocked": {"a": 1}}`, []string{"parser: blocked is not a JSON array"}},
		{"allowed as a number", `{"allowed": 1}`, []string{"parser: allowed is not a JSON array"}},
		{"null and empty lists", `{"allowed": null, "blocked": []}`, nil},
		{"absent lists", `{"mode": "allow"}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseProblems(t, rel, test.policy); !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}

// An agent's history passes over the entries that are not objects, giving
// one problem row per agent for them all; null and absent members give none.
func TestHistoryEntriesThatAreNotObjectsAreCountedPerAgent(t *testing.T) {
	const rel = "agent-zero/usr/chats/ctx/chat.json"
	message := `{"_cls":"Message","id":"m","ai":false,"content":{"user_message":"from history"}}`
	tests := []struct {
		name, histories string
		want            []string
	}{
		{"scalars among the bulks, topics and messages are counted together",
			`{"_cls":"History","bulks":5,"topics":[1,"x",null],"current":{"_cls":"Topic","messages":[1,"two",[3],` +
				message + `]}}`,
			[]string{"parser: history of agent 0 holds 6 entries that are not JSON objects"}},
		{"one scalar is one entry", `{"_cls":"History","current":{"_cls":"Topic","messages":[true,` + message + `]}}`,
			[]string{"parser: history of agent 0 holds 1 entry that is not a JSON object"}},
		{"null and absent members give nothing", `{"_cls":"History","bulks":null,"current":{"_cls":"Topic",` +
			`"messages":[null,` + message + `]}}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			history := jsonvalue.NewString(test.histories).JSON()
			// The second agent's message has an id of its own, as a
			// message whose id an earlier one has gives a problem of its own.
			second := jsonvalue.NewString(strings.Replace(test.histories, `"id":"m"`, `"id":"m1"`, 1)).JSON()
			chatText := `{"id":"ctx","agents":[{"number":0,"history":` + history + `},{"number":1,"history":` +
				second + `}],"log":{"logs":[]}}`
			want := slices.Clone(test.want)
			for _, text := range test.want {
				want = append(want, strings.Replace(text, "agent 0", "agent 1", 1))
			}
			if got := parseProblems(t, rel, chatText); !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// A history that is itself neither null, an array nor an object, as it is
// written or once a history written as JSON text is decoded, is one entry
// that is not an object; an array's entries are counted one by one, and an
// empty or absent history gives nothing.
func TestAHistoryThatIsItselfAScalarIsOneEntry(t *testing.T) {
	const rel = "agent-zero/usr/chats/ctx/chat.json"
	one := []string{"parser: history of agent 0 holds 1 entry that is not a JSON object"}
	tests := []struct {
		name, history string
		want          []string
	}{
		{"a number", `5`, one},
		{"a boolean", `true`, one},
		{"text that decodes to a number", `"7"`, one},
		{"text that decodes to text", `"\"words\""`, one},
		{"text that decodes to an array of scalars", `"[1,2]"`,
			[]string{"parser: history of agent 0 holds 2 entries that are not JSON objects"}},
		{"empty text", `""`, nil},
		{"null", `null`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chatText := `{"id":"ctx","agents":[{"number":0,"history":` + test.history + `}],"log":{"logs":[]}}`
			if got := parseProblems(t, rel, chatText); !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
