package continuedev

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
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

func TestASessionHistoryThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = ".continue/sessions/s.json"
	const start = "session start: t items=0 dateCreated unknown"
	const wrong = "parser: history is not a JSON array"
	tests := []struct {
		name, history string
		want          []string
	}{
		{"a string", `"oops"`, []string{start, wrong}},
		{"a number", `7`, []string{start, wrong}},
		{"an object", `{"message": {"role": "user", "content": "hi"}}`, []string{start, wrong}},
		{"null", `null`, []string{start}},
		{"an empty array", `[]`, []string{start}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseFile(t, rel, `{"sessionId": "s", "title": "t", "history": `+test.history+`}`)
			if !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		if got, want := parseFile(t, rel, `{"sessionId": "s", "title": "t"}`), []string{start}; !slices.Equal(got, want) {
			t.Errorf("rows %q, want %q", got, want)
		}
	})
}

func TestAHistoryItemMemberThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = ".continue/sessions/s.json"
	const start = "session start: t items=1 dateCreated unknown"
	tests := []struct {
		name, item string
		want       []string
	}{
		{"promptLogs as a string", `"promptLogs": "x", "message": {"role": "user", "content": "hi"}`,
			[]string{start, "hi", "parser: promptLogs is not a JSON array"}},
		{"toolCallStates as an object", `"toolCallStates": {"a": {}}, "message": {"role": "assistant", "content": "ok"}`,
			[]string{start, "ok", "parser: toolCallStates is not a JSON array"}},
		{"toolCalls as a number", `"message": {"role": "assistant", "content": "ok", "toolCalls": 3}`,
			[]string{start, "ok", "parser: toolCalls is not a JSON array"}},
		{"members null or empty", `"promptLogs": null, "toolCallStates": [], "message": {"role": "assistant", "content": "ok", "toolCalls": null}`,
			[]string{start, "ok"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseFile(t, rel, `{"sessionId": "s", "title": "t", "history": [{`+test.item+`}]}`)
			if !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}
