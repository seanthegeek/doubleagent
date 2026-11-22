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
		{"toolSteps and changes of the wrong kind in three messages", `{"twinny.conversations": {"c": {"id": "c", ` +
			`"messages": [{"toolSteps": 1, "changes": []}, {"toolSteps": "x", "changes": 2}, {"toolSteps": {}}]}}}`,
			[]string{"parser: toolSteps is not a JSON array in 3 entries", "parser: changes is not a JSON object in 2 entries"}},
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
		{"entries that are not objects", `[{"path": "a"}, "b", 3]`,
			[]string{"parser: files holds 2 entries that are not JSON objects"}},
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

func TestAProviderOrConversationOfTheWrongKindIsReported(t *testing.T) {
	tests := []struct {
		name, state string
		want        []string
	}{
		{"providers as a list", `{"twinny.inference-providers": ["p1"]}`,
			[]string{"parser: twinny.inference-providers is not a JSON object"}},
		{"a provider that is a string", `{"twinny.inference-providers": {"p1": "key-in-a-string", "p2": {}}}`,
			[]string{"parser: twinny.inference-providers holds 1 entry that is not a JSON object"}},
		{"several providers of the wrong kind are one row, an active provider of the wrong kind one row each",
			`{"twinny.inference-providers": {"p1": 1, "p2": "x", "p3": [], "p4": {}}, ` +
				`"twinny.active-chat-provider": 5, "twinny.active-fim-provider": 6}`,
			[]string{"parser: twinny.inference-providers holds 3 entries that are not JSON objects",
				"parser: twinny.active-chat-provider is not a JSON object",
				"parser: twinny.active-fim-provider is not a JSON object"}},
		{"providers null", `{"twinny.inference-providers": null}`, nil},
		{"an active chat provider that is a string", `{"twinny.active-chat-provider": "x"}`,
			[]string{"parser: twinny.active-chat-provider is not a JSON object"}},
		{"an active fim provider that is a list", `{"twinny.active-fim-provider": []}`,
			[]string{"parser: twinny.active-fim-provider is not a JSON object"}},
		{"an active embeddings provider that is null", `{"twinny.active-embeddings-provider": null}`, nil},
		{"conversations holding values that are not objects", `{"twinny.conversations": {"c": "x", "d": 3}}`,
			[]string{"parser: twinny.conversations holds 2 entries that are not JSON objects"}},
		{"a conversation list holding a string", `{"twinny.conversations": ["x", {"id": "c"}]}`,
			[]string{"parser: twinny.conversations holds 1 entry that is not a JSON object"}},
		{"messages, steps and changed files that are not objects", `{"twinny.conversations": {"c": {"id": "c", ` +
			`"messages": [7, {"toolSteps": ["x"], "changes": {"files": [1, 2]}}]}}}`,
			[]string{"parser: messages holds 1 entry that is not a JSON object",
				"parser: toolSteps holds 1 entry that is not a JSON object",
				"parser: changes.files holds 2 entries that are not JSON objects"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := globalProblems(t, test.state); !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}

func TestAProviderRowNamesTheProvidersID(t *testing.T) {
	tests := []struct {
		name, providers, want string
	}{
		{"the id, not the key it is filed under", `{"key-1": {"id": "p1", "apiKey": "k"}}`, "model provider: p1 "},
		{"the key when the provider has no id", `{"key-1": {"apiKey": "k"}}`, "model provider: key-1 "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, err := jsonvalue.DecodeString(`{"twinny.inference-providers": ` + test.providers + `}`)
			if err != nil {
				t.Fatal(err)
			}
			var rows []string
			New().globalRows(writeArtifact(t, "User/globalStorage/state.vscdb", ""), state, parser.Options{},
				func(row timeline.Row) { rows = append(rows, row.Text) })
			if len(rows) != 1 || !strings.HasPrefix(rows[0], test.want) {
				t.Errorf("rows %q, want one starting %q", rows, test.want)
			}
		})
	}
}
