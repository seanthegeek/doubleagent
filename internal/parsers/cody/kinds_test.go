package cody

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseFile writes data at rel under a new home and returns the texts of
// the rows the parser gives it.
func parseFile(t *testing.T, rel string, data []byte) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
	var texts []string
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

// The state.vscdb fixtures under testdata are built by testdata/gen_state.py.
func TestAHistoryThatIsNotAnObjectIsReported(t *testing.T) {
	const jetBrains = "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2"
	const state = "Code/User/globalStorage/state.vscdb"
	fixture := func(name string) []byte {
		data, err := osfile.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	tests := []struct {
		name, rel string
		data      []byte
		want      []string
	}{
		{"a JetBrains history that is a list", jetBrains, []byte(`[1]`),
			[]string{"parser: chat history is not a JSON object"}},
		{"a JetBrains history that is an object", jetBrains, []byte(`{}`), nil},
		{"a state history that is a list", state, fixture("history-list.vscdb"),
			[]string{"parser: cody-local-chatHistory-v2 is not a JSON object"}},
		{"a state history that is text holding a list", state, fixture("history-text.vscdb"),
			[]string{"parser: cody-local-chatHistory-v2 is not a JSON object"}},
		{"a state value that is a number", state, fixture("value-number.vscdb"),
			[]string{"parser: sourcegraph.cody-ai value is not a JSON object"}},
		{"a state with no history", state, fixture("no-history.vscdb"), nil},
		{"a state history that is null", state, fixture("history-null.vscdb"), nil},
		{"a state history that is text holding null", state, fixture("history-text-null.vscdb"),
			[]string{"parser: cody-local-chatHistory-v2 is not a JSON object"}},
		{"a JetBrains history that is null", jetBrains, []byte(`null`),
			[]string{"parser: chat history is not a JSON object"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseFile(t, test.rel, test.data); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

// parserRows returns the rows of texts that are problems.
func parserRows(texts []string) []string {
	var found []string
	for _, text := range texts {
		if strings.HasPrefix(text, "parser: ") {
			found = append(found, text)
		}
	}
	return found
}

// wrongKinds returns the values a member must not have, each with a name:
// a string, a number and the container of the other kind than the member's.
func wrongKinds(array bool) [][2]string {
	other := `[1]`
	if array {
		other = `{"a": 1}`
	}
	return [][2]string{{"a string", `"oops"`}, {"a number", `7`}, {"the other container", other}}
}

func TestAHistoryMemberThatIsNotTheRightKindIsReported(t *testing.T) {
	const jetBrains = "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2"
	const message = `"speaker": "assistant", "text": "ok"`
	tests := []struct {
		name, member, history string
		array                 bool
	}{
		{"chat", "chat", `{"a": {"chat": %s}}`, false},
		{"interactions", "interactions", `{"a": {"chat": {"c": {"id": "c", "interactions": %s}}}}`, true},
		{"content", "content",
			`{"a": {"chat": {"c": {"id": "c", "interactions": [{"assistantMessage": {` + message + `, "content": %s}}]}}}}`, true},
		{"processes", "processes",
			`{"a": {"chat": {"c": {"id": "c", "interactions": [{"assistantMessage": {` + message + `, "processes": %s}}]}}}}`, true},
	}
	for _, test := range tests {
		kind := "object"
		silent := []string{`null`, `{}`}
		if test.array {
			kind, silent = "array", []string{`null`, `[]`}
		}
		for _, wrong := range wrongKinds(test.array) {
			t.Run(test.name+" as "+wrong[0], func(t *testing.T) {
				got := parserRows(parseFile(t, jetBrains, []byte(fmt.Sprintf(test.history, wrong[1]))))
				if want := []string{"parser: " + test.member + " is not a JSON " + kind}; !slices.Equal(got, want) {
					t.Errorf("problems %q, want %q", got, want)
				}
			})
		}
		for _, value := range silent {
			t.Run(test.name+" as "+value, func(t *testing.T) {
				if got := parserRows(parseFile(t, jetBrains, []byte(fmt.Sprintf(test.history, value)))); len(got) != 0 {
					t.Errorf("problems %q, want none", got)
				}
			})
		}
	}
	t.Run("absent members", func(t *testing.T) {
		data := `{"a": {"chat": {"c": {"id": "c", "interactions": [{"assistantMessage": {` + message + `}}]}}}}`
		if got := parserRows(parseFile(t, jetBrains, []byte(data))); len(got) != 0 {
			t.Errorf("problems %q, want none", got)
		}
	})
}

func TestAModelPreferenceMemberThatIsNotAnObjectIsReported(t *testing.T) {
	const rel = "Cody-nodejs/JetBrains-globalState/cody-model-preferences"
	for _, kind := range []string{"selected", "defaults"} {
		for _, wrong := range wrongKinds(false) {
			t.Run(kind+" as "+wrong[0], func(t *testing.T) {
				data := `{"https://s.com/": {"` + kind + `": ` + wrong[1] + `}}`
				want := []string{"parser: " + kind + " is not a JSON object"}
				if got := parserRows(parseFile(t, rel, []byte(data))); !slices.Equal(got, want) {
					t.Errorf("problems %q, want %q", got, want)
				}
			})
		}
		for _, silent := range []string{`null`, `{}`} {
			t.Run(kind+" as "+silent, func(t *testing.T) {
				data := `{"https://s.com/": {"` + kind + `": ` + silent + `}}`
				if got := parserRows(parseFile(t, rel, []byte(data))); len(got) != 0 {
					t.Errorf("problems %q, want none", got)
				}
			})
		}
	}
}

// The state.vscdb fixtures under testdata are built by testdata/gen_state.py.
func TestStateModelPreferencesThatAreNotAnObjectAreReported(t *testing.T) {
	const rel = "Code/User/globalStorage/state.vscdb"
	for _, name := range []string{"preferences-string", "preferences-number", "preferences-list"} {
		t.Run(name, func(t *testing.T) {
			data, err := osfile.ReadFile(filepath.Join("testdata", name+".vscdb"))
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"parser: cody-model-preferences is not a JSON object"}
			if got := parserRows(parseFile(t, rel, data)); !slices.Equal(got, want) {
				t.Errorf("problems %q, want %q", got, want)
			}
		})
	}
}

func TestHistoryEntriesOfTheWrongKindAreReportedOncePerList(t *testing.T) {
	const jetBrains = "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2"
	const message = `"speaker": "assistant", "text": "ok"`
	tests := []struct {
		name, history, want string
	}{
		{"chats", `{"a": {"chat": {"c": {"id": "c"}, "d": "x"}}}`, "parser: chat holds 1 entry that is not a JSON object"},
		{"accounts", `{"a": {"chat": {}}, "b": "x", "c": null}`,
			"parser: chat history holds 2 entries that are not JSON objects"},
		{"interactions", `{"a": {"chat": {"c": {"id": "c", "interactions": [{}, "x", 7]}}}}`,
			"parser: interactions holds 2 entries that are not JSON objects"},
		{"content", `{"a": {"chat": {"c": {"id": "c", "interactions": [{"assistantMessage": {` + message +
			`, "content": ["x"]}}]}}}}`, "parser: content holds 1 entry that is not a JSON object"},
		{"processes", `{"a": {"chat": {"c": {"id": "c", "interactions": [{"assistantMessage": {` + message +
			`, "processes": [7]}}]}}}}`, "parser: processes holds 1 entry that is not a JSON object"},
		{"a message", `{"a": {"chat": {"c": {"id": "c", "interactions": [{"assistantMessage": "x"}]}}}}`,
			"parser: assistantMessage is not a JSON object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parserRows(parseFile(t, jetBrains, []byte(test.history)))
			if !slices.Equal(got, []string{test.want}) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}

func TestModelPreferenceEntriesOfTheWrongKindAreReported(t *testing.T) {
	const rel = "Cody-nodejs/JetBrains-globalState/cody-model-preferences"
	tests := []struct {
		name, data, want string
	}{
		{"an endpoint", `{"https://s.com/": {"selected": {}}, "https://t.com/": 7}`,
			"parser: cody-model-preferences holds 1 entry that is not a JSON object"},
		{"a model", `{"https://s.com/": {"selected": {"chat": "m", "edit": 7, "x": {}}}}`,
			"parser: selected holds 2 entries that are not JSON strings"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parserRows(parseFile(t, rel, []byte(test.data))); !slices.Equal(got, []string{test.want}) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}

// repeatedMembers returns a JSON object of n members, named m0 onward,
// each holding value.
func repeatedMembers(value string, n int) string {
	members := make([]string, n)
	for i := range members {
		members[i] = fmt.Sprintf(`"m%d": %s`, i, value)
	}
	return "{" + strings.Join(members, ", ") + "}"
}

// repeatedItems returns a JSON array of n copies of value.
func repeatedItems(value string, n int) string {
	return "[" + strings.TrimSuffix(strings.Repeat(value+", ", n), ", ") + "]"
}

func TestAMemberOfTheWrongKindInManyEntriesOfAHistoryIsOneProblem(t *testing.T) {
	const jetBrains = "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2"
	const message = `"speaker": "assistant", "text": "ok"`
	tests := []struct {
		name, history, want string
	}{
		{"accounts that are not objects", repeatedMembers(`7`, 4),
			"chat history holds 4 entries that are not JSON objects"},
		{"a chat of every account", repeatedMembers(`{"chat": 7}`, 4), "chat is not a JSON object in 4 entries"},
		{"interactions of every chat", `{"a": {"chat": ` + repeatedMembers(`{"interactions": 7}`, 4) + `}}`,
			"interactions is not a JSON array in 4 entries"},
		{"a message of every interaction", `{"a": {"chat": {"c": {"interactions": ` +
			repeatedItems(`{"humanMessage": 7}`, 4) + `}}}}`, "humanMessage is not a JSON object in 4 entries"},
		{"content of every message", `{"a": {"chat": {"c": {"interactions": ` +
			repeatedItems(`{"assistantMessage": {`+message+`, "content": [7]}}`, 4) + `}}}}`,
			"content holds 4 entries that are not JSON objects"},
		{"processes of every message", `{"a": {"chat": {"c": {"interactions": ` +
			repeatedItems(`{"assistantMessage": {`+message+`, "processes": 7}}`, 4) + `}}}}`,
			"processes is not a JSON array in 4 entries"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parserRows(parseFile(t, jetBrains, []byte(test.history)))
			if want := []string{"parser: " + test.want}; !slices.Equal(got, want) {
				t.Errorf("problems %q, want %q", got, want)
			}
		})
	}
}

func TestAModelListOfTheWrongKindInManyEndpointsIsOneProblem(t *testing.T) {
	const rel = "Cody-nodejs/JetBrains-globalState/cody-model-preferences"
	data := repeatedMembers(`{"selected": 7, "defaults": {"chat": 7}}`, 4)
	want := []string{"parser: selected is not a JSON object in 4 entries",
		"parser: defaults holds 4 entries that are not JSON strings"}
	if got := parserRows(parseFile(t, rel, []byte(data))); !slices.Equal(got, want) {
		t.Errorf("problems %q, want %q", got, want)
	}
}
