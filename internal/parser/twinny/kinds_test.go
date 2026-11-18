package twinny

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

// writeArtifact writes text at rel under a new home and returns its
// artifact.
func writeArtifact(t *testing.T, rel, text string) *input.Artifact {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
}

// problemTexts returns the texts of the rows that report a problem.
func problemTexts(rows []string) []string {
	var problems []string
	for _, text := range rows {
		if strings.HasPrefix(text, parser.ProblemPrefix) {
			problems = append(problems, text)
		}
	}
	return problems
}

// globalProblems returns the problem rows the global state text gives.
func globalProblems(t *testing.T, text string) []string {
	t.Helper()
	state, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	artifact := writeArtifact(t, "User/globalStorage/state.vscdb", "")
	var rows []string
	New().globalRows(artifact, state, parser.Options{}, func(row timeline.Row) { rows = append(rows, row.Text) })
	return problemTexts(rows)
}

func TestAGlobalStateMemberOfTheWrongKindIsReported(t *testing.T) {
	const conversation = `{"twinny.conversations": {"c": {"id": "c", "messages": [{"role": "assistant", "content": "x", `
	tests := []struct {
		name, state string
		want        []string
	}{
		{"conversations as a string", `{"twinny.conversations": "not-an-object"}`,
			[]string{"parser: twinny.conversations is not a JSON object"}},
		{"conversations as a number", `{"twinny.conversations": 3}`,
			[]string{"parser: twinny.conversations is not a JSON object"}},
		{"conversations as a list", `{"twinny.conversations": []}`, nil},
		{"conversations null", `{"twinny.conversations": null}`, nil},
		{"messages as an object", `{"twinny.conversations": {"c": {"id": "c", "messages": {"role": "user"}}}}`,
			[]string{"parser: messages is not a JSON array"}},
		{"messages as a string", `{"twinny.conversations": {"c": {"id": "c", "messages": "x"}}}`,
			[]string{"parser: messages is not a JSON array"}},
		{"messages empty", `{"twinny.conversations": {"c": {"id": "c", "messages": []}}}`, nil},
		{"toolSteps as an object", conversation + `"toolSteps": {"id": "s"}}]}}}`,
			[]string{"parser: toolSteps is not a JSON array"}},
		{"toolSteps as a number", conversation + `"toolSteps": 1}]}}}`,
			[]string{"parser: toolSteps is not a JSON array"}},
		{"changes as a list", conversation + `"changes": []}]}}}`,
			[]string{"parser: changes is not a JSON object"}},
		{"changes.files as an object", conversation + `"changes": {"files": {"path": "a"}}}]}}}`,
			[]string{"parser: changes.files is not a JSON array"}},
		{"changes.files as a string", conversation + `"changes": {"files": "a"}}]}}}`,
			[]string{"parser: changes.files is not a JSON array"}},
		{"members null or empty", conversation + `"toolSteps": null, "changes": {"files": []}}]}}}`, nil},
		{"team policy as a list", `{"twinny.teamPolicy": [1]}`,
			[]string{"parser: twinny.teamPolicy is not a JSON object"}},
		{"team policy null", `{"twinny.teamPolicy": null}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := globalProblems(t, test.state); !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}

func TestAnAgentChangesFilesThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = "User/globalStorage/rjmacarthy.twinny/agent-changes/a.json"
	tests := []struct {
		name, files string
		want        []string
	}{
		{"an object", `{"path": "a"}`, []string{"parser: files is not a JSON array"}},
		{"a string", `"a"`, []string{"parser: files is not a JSON array"}},
		{"a number", `2`, []string{"parser: files is not a JSON array"}},
		{"null", `null`, nil},
		{"empty", `[]`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact := writeArtifact(t, rel, `{"id": "a", "files": `+test.files+`}`)
			var rows []string
			if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { rows = append(rows, row.Text) }); err != nil {
				t.Fatal(err)
			}
			if got := problemTexts(rows); !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}
