package clinelegacy

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// metadataRows writes text as a task's task_metadata.json under a new home
// and returns the texts of the rows parseMetadata gives it.
func metadataRows(t *testing.T, text string) []string {
	t.Helper()
	home := t.TempDir()
	rel := ".cline/data/tasks/t/task_metadata.json"
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
	reader := NewTaskParser(parser.NewBase("x", "x"), `\.cline/data`, "", nil)
	var texts []string
	reader.parseMetadata(artifact, &TaskContext{TaskID: "t"}, func(row timeline.Row) { texts = append(texts, row.Text) })
	return texts
}

func TestATaskMetadataListThatIsNotAnArrayIsReported(t *testing.T) {
	tests := []struct {
		name, member, value string
		want                []string
	}{
		{"files_in_context as a string", "files_in_context", `"x"`, []string{"parser: files_in_context is not a JSON array"}},
		{"files_in_context as a number", "files_in_context", `4`, []string{"parser: files_in_context is not a JSON array"}},
		{"files_in_context as an object", "files_in_context", `{"a": 1}`, []string{"parser: files_in_context is not a JSON array"}},
		{"model_usage as a string", "model_usage", `"x"`, []string{"parser: model_usage is not a JSON array"}},
		{"model_usage as a number", "model_usage", `4`, []string{"parser: model_usage is not a JSON array"}},
		{"model_usage as an object", "model_usage", `{"a": 1}`, []string{"parser: model_usage is not a JSON array"}},
		{"environment_history as a string", "environment_history", `"x"`, []string{"parser: environment_history is not a JSON array"}},
		{"environment_history as a number", "environment_history", `4`, []string{"parser: environment_history is not a JSON array"}},
		{"environment_history as an object", "environment_history", `{"a": 1}`, []string{"parser: environment_history is not a JSON array"}},
		{"null", "model_usage", `null`, nil},
		{"an empty array", "environment_history", `[]`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := metadataRows(t, `{"`+test.member+`": `+test.value+`}`)
			if !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		if got := metadataRows(t, `{}`); len(got) != 0 {
			t.Errorf("rows %q, want none", got)
		}
	})
}
