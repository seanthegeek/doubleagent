package kilocode

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// write writes text to path, making its directory.
func write(t testing.TB, path, text string) {
	t.Helper()
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// taskArtifact returns the artifact of a pre-fork task's history under home.
func taskArtifact(home, taskID string) *input.Artifact {
	rel := ".config/Code/User/globalStorage/kilocode.kilo-code/tasks/" + taskID + "/api_conversation_history.json"
	return &input.Artifact{Rel: rel, DiskPath: filepath.Join(home, filepath.FromSlash(rel)),
		Home: &input.Home{DiskPath: home}}
}

func TestEveryTaskTakesItsIndexEntryFromOneReadOfTheIndex(t *testing.T) {
	const tasks = 300
	home := t.TempDir()
	entries := []string{`{"id": "t0", "workspace": "/first"}`, `7`, `{"id": 1, "workspace": "/number"}`}
	for i := range tasks {
		entries = append(entries, `{"id": "t`+strconv.Itoa(i)+`", "workspace": "/w`+strconv.Itoa(i)+`"}`)
	}
	tasksDirectory := filepath.Join(home, ".config/Code/User/globalStorage/kilocode.kilo-code/tasks")
	write(t, filepath.Join(tasksDirectory, "_index.json"), `{"entries": [`+strings.Join(entries, ",")+`]}`)
	subject := New()
	for i := range tasks {
		artifact := taskArtifact(home, "t"+strconv.Itoa(i))
		write(t, artifact.DiskPath, `[{"role": "user", "content": "hi"}]`)
		var rows []timeline.Row
		subject.task(artifact, parser.Options{}, func(row timeline.Row) { rows = append(rows, row) })
		want := "/w" + strconv.Itoa(i)
		if i == 0 {
			want = "/first"
		}
		if len(rows) == 0 || rows[0].ProjectPath != want {
			t.Fatalf("task %d gave %+v, want the project %s", i, rows, want)
		}
	}
	if subject.indexed != len(entries) {
		t.Errorf("%d index entries read for %d tasks, want one pass over %d", subject.indexed, tasks,
			len(entries))
	}
}
