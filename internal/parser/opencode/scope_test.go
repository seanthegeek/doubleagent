package opencode

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// writeTree writes files, by path under a new home, and returns the home.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	home := t.TempDir()
	for rel, text := range files {
		path := filepath.Join(home, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestASessionsSideFilesAreReadOnceForAllItsMessages(t *testing.T) {
	const data = ".local/share/opencode"
	const messages = 20
	tests := []struct {
		name     string
		side     map[string]string
		messages string
	}{
		{"the old layout's info file", map[string]string{
			data + "/project/p/storage/session/info/ses_x.json": `{"id": "ses_x", "directory": "/work/x"}`},
			data + "/project/p/storage/session/message/ses_x/"},
		{"the new layout's project file", map[string]string{
			data + "/storage/session/proj/ses_x.json": `{"id": "ses_x", "projectID": "proj"}`,
			data + "/storage/project/proj.json":       `{"worktree": "/work/x"}`},
			data + "/storage/message/ses_x/"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{}
			for rel, text := range test.side {
				files[rel] = text
			}
			var rels []string
			for i := range messages {
				rel := fmt.Sprintf("%smsg_%03d.json", test.messages, i)
				rels = append(rels, rel)
				files[rel] = fmt.Sprintf(`{"id": "msg_%03d", "sessionID": "ses_x", "role": "assistant", `+
					`"providerID": "p", "modelID": "m%d"}`, i, i)
			}
			home := writeTree(t, files)
			subject := New()
			subject.Work = parser.CountingWork()
			rows := 0
			for _, rel := range rels {
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
					Home: &input.Home{DiskPath: home}}
				if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
					rows++
					if row.ProjectPath != "/work/x" {
						t.Errorf("%s: row %q has project %q, want /work/x", rel, row.Text, row.ProjectPath)
					}
				}); err != nil {
					t.Fatal(err)
				}
			}
			if reads := subject.Work.Count("session side reads"); rows == 0 || reads != 1 {
				t.Errorf("%d messages gave %d rows and read the session's side files %d times, want once",
					messages, rows, reads)
			}
		})
	}
}
