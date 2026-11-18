package opencode

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

// problemTexts returns the texts of the rows that start with "parser:".
func problemTexts(rows []timeline.Row) []string {
	var texts []string
	for _, row := range rows {
		if strings.HasPrefix(row.Text, parser.ProblemPrefix) {
			texts = append(texts, row.Text)
		}
	}
	return texts
}

// assistantProblems returns the "parser:" rows of a 2.x assistant record
// with the given data, which the database fixtures cannot give since a
// session_message table is built by a Python generator.
func assistantProblems(t *testing.T, data string) []string {
	t.Helper()
	value, err := jsonvalue.DecodeString(data)
	if err != nil {
		t.Fatal(err)
	}
	var rows []timeline.Row
	New().recordRows(&input.Artifact{Rel: databaseRel}, newScope("s", ""), "assistant", value, 3, "",
		parser.Options{}, func(row timeline.Row) { rows = append(rows, row) })
	return problemTexts(rows)
}

func TestAnAssistantContentThatIsNotAnArrayIsReported(t *testing.T) {
	const wrong = "parser: content is not a JSON array"
	tests := []struct {
		name, content string
		want          []string
	}{
		{"a string", `"x"`, []string{wrong}},
		{"a number", `7`, []string{wrong}},
		{"an object", `{"type": "text", "text": "hi"}`, []string{wrong}},
		{"null", `null`, nil},
		{"an empty array", `[]`, nil},
		{"items that are not objects", `[{"type": "text", "text": "hi"}, "x", 7]`,
			[]string{"parser: content holds 2 entries that are not JSON objects"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := assistantProblems(t, `{"content": `+test.content+`}`); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		if got := assistantProblems(t, `{}`); len(got) != 0 {
			t.Errorf("rows %q, want none", got)
		}
	})
}

func TestAToolStateContentThatIsNotAnArrayIsReported(t *testing.T) {
	const wrong = "parser: state.content is not a JSON array"
	tests := []struct {
		name, content string
		want          []string
	}{
		{"a string", `"x"`, []string{wrong}},
		{"a number", `7`, []string{wrong}},
		{"an object", `{"text": "out"}`, []string{wrong}},
		{"null", `null`, nil},
		{"an empty array", `[]`, nil},
		{"chunks that are not objects", `[{"text": "out"}, 7]`,
			[]string{"parser: state.content holds 1 entry that is not a JSON object"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := `{"content": [{"type": "tool", "id": "c", "name": "bash", "state": ` +
				`{"status": "completed", "input": {"command": "ls"}, "content": ` + test.content + `}}]}`
			if got := assistantProblems(t, data); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		data := `{"content": [{"type": "tool", "id": "c", "name": "bash", "state": {"status": "completed"}}]}`
		if got := assistantProblems(t, data); len(got) != 0 {
			t.Errorf("rows %q, want none", got)
		}
	})
}

func TestAPinnedListThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = ".local/state/opencode/session.json"
	const wrong = "parser: pinned is not a JSON array"
	tests := []struct {
		name, pinned string
		want         []string
	}{
		{"a string", `"ses_1"`, []string{wrong}},
		{"a number", `7`, []string{wrong}},
		{"an object", `{"id": "ses_1"}`, []string{wrong}},
		{"null", `null`, nil},
		{"an empty array", `[]`, nil},
		{"absent", ``, nil},
		{"ids that are not text", `["ses_1", 7, {}]`, []string{"parser: pinned holds 2 entries that are not JSON strings"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := `{}`
			if test.pinned != "" {
				text = `{"pinned": ` + test.pinned + `}`
			}
			home := t.TempDir()
			path := filepath.Join(home, filepath.FromSlash(rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Agent: "opencode",
				Home: &input.Home{DiskPath: home, Original: "/h"}, Manifest: jsonvalue.NewNull()}
			var rows []timeline.Row
			if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
				t.Fatal(err)
			}
			if got := problemTexts(rows); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}
