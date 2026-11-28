package clinelegacy

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	reader := NewTaskParser(parserkit.NewBase("x", "x"), `\.cline/data`, "", nil)
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

func TestTaskMetadataEntriesThatAreNotObjectsAreReportedOncePerList(t *testing.T) {
	for _, member := range []string{"files_in_context", "model_usage", "environment_history"} {
		t.Run(member, func(t *testing.T) {
			got := metadataRows(t, `{"`+member+`": [7, "x"]}`)
			if want := []string{"parser: " + member + " holds 2 entries that are not JSON objects"}; !slices.Equal(got, want) {
				t.Errorf("rows %q, want %q", got, want)
			}
		})
	}
}

func TestAModelUsageTimeIsAJSONNumberOnly(t *testing.T) {
	tests := []struct{ name, stamp, want string }{
		{"a number after the first entry", `1767000000000`, "b"},
		{"digits in a string with spaces around them sort first", `" 1767000000000 "`, "a"},
		{"true sorts first", `true`, "a"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata := `{"model_usage": [{"ts": 1, "model_id": "a"}, {"ts": ` + test.stamp + `, "model_id": "b"}]}`
			context := newTaskContext("t", jsonvalue.NewNull(), decode(t, metadata), "")
			if got := context.modelAt(jsonvalue.NewInt(1767000000001)); got != test.want {
				t.Errorf("model %q, want %q", got, test.want)
			}
		})
	}
}

func TestAFileEventOfATrueAndOfAOnePathIsReportedForEach(t *testing.T) {
	entry := `"record_source": "s", "record_state": "r", "read_date": 5`
	got := metadataRows(t, `{"files_in_context": [{"path": 1, `+entry+`}, {"path": true, `+entry+`},
		{"path": 1.0, `+entry+`}, {"path": 2, "read_date": true}]}`)
	want := []string{"file read: 1 (source=s state=r)", "file read: true (source=s state=r)"}
	if !slices.Equal(got, want) {
		t.Errorf("rows %q, want %q", got, want)
	}
}
