//go:build unix

package opencode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestALegacySessionFileIsNeverFoundThroughALinkedDirectory(t *testing.T) {
	tests := []struct {
		name, want string
		linked     bool
	}{
		{"a session directory in the storage gives the project", "/work/x", false},
		{"a session directory linked to one outside the home gives none", "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact := legacySession(t, 1)[0]
			if test.linked {
				sessions := filepath.Join(artifact.Home.DiskPath, ".local", "share", "opencode", "storage", "session")
				outside := filepath.Join(t.TempDir(), "session")
				if err := os.Rename(sessions, outside); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, sessions); err != nil {
					t.Fatal(err)
				}
			}
			var rows []timeline.Row
			if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) {
				rows = append(rows, row)
			}); err != nil {
				t.Fatal(err)
			}
			if len(rows) == 0 || rows[0].ProjectPath != test.want {
				t.Errorf("rows %+v, want rows with project %q", rows, test.want)
			}
		})
	}
}

// A project directory that cannot be listed but can be searched still
// gives the session file at its place in name order.
func TestASessionFileInAProjectDirectoryThatCannotBeListedIsStillFound(t *testing.T) {
	directory := t.TempDir()
	files := map[string]string{"p1/s.json": `{"directory": "/hidden"}`, "p2/s.json": `{"directory": "/listed"}`}
	for rel, text := range files {
		path := filepath.Join(directory, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	hidden := filepath.Join(directory, "p1")
	if err := os.Chmod(hidden, 0o100); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(hidden, 0o700) })
	if _, err := os.ReadDir(hidden); err == nil {
		t.Skip("this user can list a directory without read permission")
	}
	want := sessionByEveryDirectory(directory, "s").JSON()
	got := newTreeIndex().session(directory, "s", parser.Work{}).JSON()
	if got != want || got != `{"directory":"/hidden"}` {
		t.Errorf("got %s, want %s, the first directory's file", got, want)
	}
}
