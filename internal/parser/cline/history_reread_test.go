package cline

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestATaskHistoryIsIndexedAgainOnlyWhenItRepeatsAnID(t *testing.T) {
	tests := []struct {
		name, history string
		indexed       int
	}{
		{"no id repeats", `[{"id":"1","ts":1},{"id":"2","ts":2}]`, 0},
		{"an id repeats", `[{"id":"1","ts":1},{"id":"1","ts":2}]`, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			rel := ".cline/data/state/taskHistory.json"
			path := filepath.Join(home, filepath.FromSlash(rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(path, []byte(test.history), 0o600); err != nil {
				t.Fatal(err)
			}
			subject := New()
			subject.Work = parser.CountingWork()
			artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "cline",
				Home: &input.Home{DiskPath: home, Original: "/home/u"}, Manifest: jsonvalue.NewNull()}
			rows := 0
			if err := subject.Parse(artifact, parser.Options{}, func(timeline.Row) { rows++ }); err != nil {
				t.Fatal(err)
			}
			if indexed := subject.Work.Count("index items read"); indexed != test.indexed || rows != 2 {
				t.Errorf("%d rows, the file indexed again over %d items, want 2 and %d", rows, indexed, test.indexed)
			}
		})
	}
}
