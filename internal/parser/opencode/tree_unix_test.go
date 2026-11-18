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
