package cody

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
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
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseFile(t, test.rel, test.data); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}
