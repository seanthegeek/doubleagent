package cline

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// Each messages file of a session takes the project from the session's
// manifest, which one parser value reads once per home however many
// messages files name it, and a missing manifest is looked for once too.
func TestASessionManifestIsReadOncePerHomeForAllItsMessagesFiles(t *testing.T) {
	tests := []struct {
		name, manifest, project string
	}{
		{"a manifest naming the project", `{"cwd":"/p","started_at":"2026-01-02T03:04:05Z"}`, "/p"},
		{"no manifest", "", ""},
	}
	const files = 3
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			directory := filepath.Join(home, ".cline", "data", "sessions", "s1")
			if err := osfile.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			if test.manifest != "" {
				if err := osfile.WriteFile(filepath.Join(directory, "s1.json"), []byte(test.manifest), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			owner := &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}
			subject := New()
			subject.Work = parserkit.CountingWork()
			var projects []string
			for _, name := range []string{"s1.messages.json", "a.messages.json", "b.messages.json"}[:files] {
				path := filepath.Join(directory, name)
				body := `{"messages":[{"role":"user","ts":"2026-01-02T03:04:05Z","content":[{"type":"text","text":"hi"}]}]}`
				if err := osfile.WriteFile(path, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
				rel := ".cline/data/sessions/s1/" + name
				artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "cline",
					Home: owner, Manifest: jsonvalue.NewNull()}
				if err := subject.Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
					projects = append(projects, row.ProjectPath)
				}); err != nil {
					t.Fatal(err)
				}
			}
			if reads := subject.Work.Count("session manifests read"); reads != 1 {
				t.Errorf("%d messages files read the manifest %d times, want once", files, reads)
			}
			if len(projects) != files {
				t.Fatalf("got %d rows, want %d", len(projects), files)
			}
			for _, project := range projects {
				if project != test.project {
					t.Errorf("a row's project is %q, want %q", project, test.project)
				}
			}
		})
	}
}
