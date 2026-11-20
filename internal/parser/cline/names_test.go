package cline

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// writeManifest writes a session manifest naming cwd at the slash-separated
// rel below home, making its directories.
func writeManifest(t *testing.T, home, rel, cwd string) {
	t.Helper()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(`{"cwd": "`+cwd+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestASessionIDThatIsNotAFileNameHasNoManifest(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		outside   string
	}{
		{"an id of .. reads nothing beside the sessions", "..", ".cline/data/...json"},
		{"an id climbing out of the sessions reads nothing there", "../x", ".cline/data/x.json"},
		{"an id holding a slash reads nothing below another session", "a/b",
			".cline/data/sessions/a/b/a/b.json"},
		{"an empty id reads nothing", "", ".cline/data/sessions/.json"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			writeManifest(t, home, test.outside, "/outside")
			for _, rel := range []string{sessionsDatabaseRel, searchDatabaseRel} {
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
					Home: &input.Home{DiskPath: home}}
				if manifestPresent(artifact, test.sessionID) {
					t.Errorf("%s: the manifest was found", rel)
				}
				if got := New().searchProject(artifact, test.sessionID, "/workspace", map[parser.EntryKey]manifestProject{}); got != "/workspace" {
					t.Errorf("%s: the project is %q, want the workspace", rel, got)
				}
			}
		})
	}
	home := t.TempDir()
	writeManifest(t, home, ".cline/data/sessions/s1/s1.json", "/srv/proj")
	artifact := &input.Artifact{DiskPath: filepath.Join(home, ".cline", "data", "db", "sessions.db"),
		Rel: sessionsDatabaseRel, Home: &input.Home{DiskPath: home}}
	if !manifestPresent(artifact, "s1") || New().searchProject(artifact, "s1", "/workspace", map[parser.EntryKey]manifestProject{}) != "/srv/proj" {
		t.Error("a plain session id's manifest was not read")
	}
}
