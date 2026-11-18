package roocode

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseFile writes text at rel under a new home and returns the texts of
// the rows the parser gives it.
func parseFile(t *testing.T, rel, text string) []string {
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
	var texts []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

// The same table drives the workspace-state.json file and, through
// retryQueueRows, the state of a state.vscdb, which only a Python
// generator could build.
func TestARetryQueueThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = ".vscode-mock/workspace-storage/h/workspace-state.json"
	const wrong = "parser: roo.retryQueue is not a JSON array"
	tests := []struct {
		name, value string
		wrong       bool
	}{
		{"a string", `"oops"`, true},
		{"a number", `7`, true},
		{"an object", `{"id": "a"}`, true},
		{"null", `null`, false},
		{"an empty array", `[]`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var want []string
			if test.wrong {
				want = []string{wrong}
			}
			if got := parseFile(t, rel, `{"roo.retryQueue": `+test.value+`}`); !slices.Equal(got, want) {
				t.Errorf("file rows %q, want %q", got, want)
			}
			state, err := jsonvalue.DecodeString(`{"roo.retryQueue": ` + test.value + `}`)
			if err != nil {
				t.Fatal(err)
			}
			problem := New().retryQueueRows(&input.Artifact{Rel: rel}, state, rel, func(timeline.Row) {})
			if (problem != "") != test.wrong || (test.wrong && problem != "roo.retryQueue is not a JSON array") {
				t.Errorf("retryQueueRows problem %q, want wrong=%v", problem, test.wrong)
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		if got := parseFile(t, rel, `{}`); len(got) != 0 {
			t.Errorf("rows %q, want none", got)
		}
	})
}
