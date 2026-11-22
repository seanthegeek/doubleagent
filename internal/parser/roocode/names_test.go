package roocode

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// writeFile writes text to the slash-separated rel below root, making its
// directories.
func writeFile(t *testing.T, root, rel, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// indexTexts returns the row texts of a task index whose one entry has the
// task id, with history_item.json written at each of the outside paths
// first.
func indexTexts(t *testing.T, taskID string, outside []string) []string {
	t.Helper()
	home := t.TempDir()
	const rel = ".vscode-mock/global-storage/tasks/_index.json"
	writeFile(t, home, rel, `{"entries": [{"id": "`+taskID+`", "task": "fix it", "ts": 1790000000000}]}`)
	for _, name := range outside {
		writeFile(t, home, name, `{"id": "x"}`)
	}
	artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)),
		Original: "/alice/" + rel, Rel: rel, Home: &input.Home{DiskPath: home, User: "alice"}}
	var texts []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) {
		texts = append(texts, row.Text)
	}); err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestATaskIDThatIsNotAFileNameHasNoHistoryItem(t *testing.T) {
	tests := []struct {
		name    string
		taskID  string
		outside []string
	}{
		{"an id of .. reads nothing beside the tasks", "..",
			[]string{".vscode-mock/global-storage/history_item.json"}},
		{"an id climbing out of the tasks reads nothing there", "../x",
			[]string{".vscode-mock/global-storage/x/history_item.json"}},
		{"an id holding a slash reads nothing below another task", "a/b",
			[]string{".vscode-mock/global-storage/tasks/a/b/history_item.json"}},
		{"an empty id reads nothing", "", []string{".vscode-mock/global-storage/tasks/history_item.json"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			present := indexTexts(t, test.taskID, test.outside)
			absent := indexTexts(t, test.taskID, nil)
			if !slices.Equal(present, absent) || len(present) == 0 {
				t.Errorf("with the outside file %q, without it %q", present, absent)
			}
		})
	}
	if got := indexTexts(t, "t1", []string{".vscode-mock/global-storage/tasks/t1/history_item.json"}); len(got) != 0 {
		t.Errorf("a task whose history item is there gave %q", got)
	}
}
