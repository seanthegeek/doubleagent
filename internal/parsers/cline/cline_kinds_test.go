package cline

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestAMessagesMemberThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = ".cline/data/sessions/s/s.messages.json"
	const wrong = "parser: messages is not a JSON array"
	tests := []struct {
		name, value string
		want        []string
	}{
		{"a string", `"oops"`, []string{wrong}},
		{"a number", `7`, []string{wrong}},
		{"an object", `{"role": "user", "content": "hi"}`, []string{wrong}},
		{"null", `null`, nil},
		{"an empty array", `[]`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseFile(t, rel, `{"messages": `+test.value+`}`)
			if !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		if got := parseFile(t, rel, `{}`); len(got) != 0 {
			t.Errorf("rows %q, want none", got)
		}
	})
}

func TestACloudSessionStatusesThatIsNotAnObjectIsReported(t *testing.T) {
	const rel = ".cline/data/globalState.json"
	const wrong = "parser: cloudSessionStatuses is not a JSON object"
	tests := []struct {
		name, value string
		want        []string
	}{
		{"a string", `"oops"`, []string{wrong}},
		{"a number", `7`, []string{wrong}},
		{"an array", `[{"status": "completed"}]`, []string{wrong}},
		{"null", `null`, nil},
		{"an empty object", `{}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseFile(t, rel, `{"cloudSessionStatuses": `+test.value+`}`)
			if !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		if got := parseFile(t, rel, `{}`); len(got) != 0 {
			t.Errorf("rows %q, want none", got)
		}
	})
}

func TestEntriesThatAreNotObjectsAreReportedOncePerFile(t *testing.T) {
	tests := []struct {
		name, rel, text, want string
	}{
		{"SDK messages", ".cline/data/sessions/s/s.messages.json",
			`{"messages": [{"role": "user", "content": "hi"}, 7, "x"]}`,
			"parser: messages holds 2 entries that are not JSON objects"},
		{"task history", ".cline/data/state/taskHistory.json", `[{"id": "1", "task": "hi", "ts": 1}, 7]`,
			"parser: taskHistory.json holds 1 entry that is not a JSON object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseFile(t, test.rel, test.text); !slices.Contains(got, test.want) {
				t.Errorf("rows %q lack %q", got, test.want)
			}
		})
	}
}
