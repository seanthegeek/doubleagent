package pearai

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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

// collect returns an emit that appends each row's text to texts.
func collect(texts *[]string) parserkit.Emit {
	return func(row timeline.Row) { *texts = append(*texts, row.Text) }
}

func TestASessionMemberThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = ".pearai/sessions/s.json"
	tests := []struct {
		name, members string
		want          []string
	}{
		{"history as an object", `"history": {"message": {"role": "user", "content": "hi"}}`,
			[]string{"session start: t | history=0 perplexityHistory=0 | dateCreated unknown",
				"parser: history is not a JSON array"}},
		{"perplexityHistory as a string", `"history": [], "perplexityHistory": "x"`,
			[]string{"session start: t | history=0 perplexityHistory=0 | dateCreated unknown",
				"parser: perplexityHistory is not a JSON array"}},
		{"promptLogs as a number", `"history": [{"promptLogs": 3, "message": {"role": "user", "content": "hi"}}]`,
			[]string{"session start: t | history=1 perplexityHistory=0 | dateCreated unknown", "hi",
				"parser: promptLogs is not a JSON array"}},
		{"null and empty members", `"history": null, "perplexityHistory": []`,
			[]string{"session start: t | history=0 perplexityHistory=0 | dateCreated unknown"}},
		{"absent members", ``,
			[]string{"session start: t | history=0 perplexityHistory=0 | dateCreated unknown"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			separator := ""
			if test.members != "" {
				separator = ", "
			}
			artifact := writeArtifact(t, rel, `{"sessionId": "s", "title": "t"`+separator+test.members+`}`)
			var got []string
			if err := New().Parse(artifact, parserkit.Options{}, collect(&got)); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestATaskHistoryThatIsNotAnArrayIsReported(t *testing.T) {
	tests := []struct {
		name, state string
		want        []string
	}{
		{"an object", `{"taskHistory": {"id": "a", "task": "x", "ts": 1}}`, []string{"parser: taskHistory is not a JSON array"}},
		{"a string", `{"taskHistory": "x"}`, []string{"parser: taskHistory is not a JSON array"}},
		{"null", `{"taskHistory": null}`, nil},
		{"empty", `{"taskHistory": []}`, nil},
		{"absent", `{}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact := writeArtifact(t, "User/globalStorage/state.vscdb", "")
			var got []string
			New().stateRows(artifact, decoded(t, test.state), collect(&got))
			if !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}
