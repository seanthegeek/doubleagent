//go:build unix

package opencode

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
			if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
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

// A project directory that cannot be listed, whether or not it can be
// searched, holds no session file: it is remembered as failed when the
// session directory is first listed and never looked in again, so the
// lookups of as many sessions as there are such directories look only
// where a listing holds the name.
func TestAProjectDirectoryThatCannotBeListedIsNeverLookedIn(t *testing.T) {
	for _, n := range []int{10, 40} {
		for _, mode := range []os.FileMode{0o100, 0o000} {
			t.Run(fmt.Sprintf("%d directories of mode %03o", n, mode), func(t *testing.T) {
				directory := t.TempDir()
				files := map[string]string{"zz/s.json": `{"directory": "/listed"}`}
				for i := range n {
					files[fmt.Sprintf("p%03d/s.json", i)] = `{"directory": "/hidden"}`
				}
				for rel, text := range files {
					path := filepath.Join(directory, filepath.FromSlash(rel))
					if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				for i := range n {
					hidden := filepath.Join(directory, fmt.Sprintf("p%03d", i))
					if err := os.Chmod(hidden, mode); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.Chmod(hidden, 0o700) })
				}
				if _, err := os.ReadDir(filepath.Join(directory, "p000")); err == nil {
					t.Skip("a directory without read permission can still be listed by this user")
				}
				index, work := newTreeIndex(), parserkit.CountingWork()
				got := index.session(directory, "s", work).JSON()
				for i := range n {
					index.session(directory, fmt.Sprintf("absent%d", i), work)
				}
				candidates, listings := work.Count("session file candidates"), work.Count("project directory listings")
				if got != `{"directory":"/listed"}` || candidates != 1 || listings != n+1 {
					t.Errorf("got %s, %d candidates and %d listings; want the listed file, 1 and %d", got,
						candidates, listings, n+1)
				}
			})
		}
	}
}
