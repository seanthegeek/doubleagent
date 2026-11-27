package shellgpt

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// chatProblems writes text as a chat file under a new home, parses it and
// returns the texts of the rows that report a problem.
func chatProblems(t *testing.T, text string) []string {
	t.Helper()
	const rel = ".config/shell_gpt/chat_cache/c1"
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
		if strings.HasPrefix(row.Text, parserkit.ProblemPrefix) {
			problems = append(problems, row.Text)
		}
	}
	if err := New().Parse(artifact, parserkit.Options{}, emit); err != nil {
		t.Fatal(err)
	}
	return problems
}

func TestAChatElementOrToolCallsOfTheWrongKindIsReported(t *testing.T) {
	const user = `{"role": "user", "content": "hi"}`
	tests := []struct {
		name, chat string
		want       []string
	}{
		{"an element that is a number", `[` + user + `, 5]`,
			[]string{"parser: unparseable record 2 (not a JSON object): 5"}},
		{"an element that is a string", `["x", ` + user + `]`,
			[]string{`parser: unparseable record 1 (not a JSON object): "x"`}},
		{"an element that is a list", `[` + user + `, [1]]`,
			[]string{"parser: unparseable record 2 (not a JSON object): [1]"}},
		{"tool_calls as an object", `[{"role": "assistant", "tool_calls": {"id": "c"}}]`,
			[]string{"parser: tool_calls is not a JSON array"}},
		{"tool_calls as a string", `[{"role": "assistant", "tool_calls": "c"}]`,
			[]string{"parser: tool_calls is not a JSON array"}},
		{"tool_calls null or empty", `[{"role": "assistant", "tool_calls": null}, {"role": "assistant", "tool_calls": []}]`, nil},
		{"every element an object", `[` + user + `]`, nil},
		{"tool_calls as a string in three messages", `[{"role": "assistant", "tool_calls": "c"}, ` +
			`{"role": "assistant", "tool_calls": 1}, {"role": "assistant", "tool_calls": {}}]`,
			[]string{"parser: tool_calls is not a JSON array in 3 entries"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := chatProblems(t, test.chat); !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}
