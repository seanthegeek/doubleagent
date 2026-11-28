package tabby

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit/parsertest"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// problemRows parses data stored at rel under a new home, as an artifact of
// agent, and returns the texts of the problem rows it gives.
func problemRows(t *testing.T, rel, agent string, data []byte) []string {
	t.Helper()
	var found []string
	for _, row := range parseRowsAsBy(t, parsertest.Parse, secrets.Setting{}, agent, rel, data) {
		if strings.HasPrefix(row.Text, "parser: ") {
			found = append(found, row.Text)
		}
	}
	return found
}

func TestACompletionChoicesThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = ".tabby/events/2026-01-01.json"
	const before, after = `{"user": "1", "ts": 1, "event": {"completion": {"completion_id": "c", "prompt": "p"`,
		"}}}\n"
	const wrong = "parser: choices is not a JSON array"
	tests := []struct {
		name, member string
		want         []string
	}{
		{"a string", `, "choices": "oops"`, []string{wrong}},
		{"a number", `, "choices": 7`, []string{wrong}},
		{"an object", `, "choices": {"text": "x"}`, []string{wrong}},
		{"null", `, "choices": null`, nil},
		{"an empty array", `, "choices": []`, nil},
		{"absent", ``, nil},
		{"entries that are not objects", `, "choices": [{"text": "x"}, "y", 7]`,
			[]string{"parser: choices holds 2 entries that are not JSON objects"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := problemRows(t, rel, "", []byte(before+test.member+after))
			if !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}

// The state.vscdb fixtures under testdata are built by testdata/gen_state.py.
func TestARecentCommandHistoryThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = ".config/Code/User/globalStorage/state.vscdb"
	const wrong = "parser: edit.recentlyCommand is not a JSON array"
	tests := []struct {
		fixture string
		want    []string
	}{
		{"command-string", []string{wrong}},
		{"command-number", []string{wrong}},
		{"command-object", []string{wrong}},
		{"command-null", nil},
		{"command-empty", nil},
	}
	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			data, err := osfile.ReadFile(filepath.Join("testdata", test.fixture+".vscdb"))
			if err != nil {
				t.Fatal(err)
			}
			if got := problemRows(t, rel, "vscode", data); !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}

func TestAnEventBodyThatIsNotAnObjectIsReported(t *testing.T) {
	const rel = ".tabby/events/2026-01-01.json"
	tests := []struct {
		name, event string
		want        []string
	}{
		{"a completion as a string", `{"completion": "x"}`, []string{"parser: completion is not a JSON object"}},
		{"a view as a number", `{"view": 3}`, []string{"parser: view is not a JSON object"}},
		{"a select as an array", `{"select": [1]}`, []string{"parser: select is not a JSON object"}},
		{"a dismiss as null", `{"dismiss": null}`, nil},
		{"a view as an object", `{"view": {"choice_index": 0}}`, nil},
		{"another event as a string", `{"other": "x"}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := problemRows(t, rel, "", []byte(`{"user": "1", "ts": 1, "event": `+test.event+"}\n"))
			if !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}

// The state.vscdb fixtures under testdata are built by testdata/gen_state.py.
func TestServerRecordsOfTheWrongKindAreReported(t *testing.T) {
	const rel = ".config/Code/User/globalStorage/state.vscdb"
	tests := []struct {
		fixture string
		want    []string
	}{
		{"servers-list", []string{"parser: server.serverRecords is not a JSON object"}},
		{"servers-string", []string{"parser: server.serverRecords holds 1 entry that is not a JSON object"}},
		{"command-empty", nil},
	}
	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			data, err := osfile.ReadFile(filepath.Join("testdata", test.fixture+".vscdb"))
			if err != nil {
				t.Fatal(err)
			}
			if got := problemRows(t, rel, "vscode", data); !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}
