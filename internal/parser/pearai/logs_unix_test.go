//go:build unix

package pearai

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/continuedev"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestADevChatNeverReadsTheIndexThroughALinkedDirectory(t *testing.T) {
	tests := []struct {
		name, want string
		linked     bool
	}{
		{"a sessions directory beside dev_data gives the project", "/work/project", false},
		{"a sessions directory linked to one outside the home gives none", "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			const rel = ".pearai/dev_data/chat.jsonl"
			chat := filepath.Join(home, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(chat), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(chat, []byte(`{"sessionId": "a", "completion": "hi"}`+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			held := filepath.Join(home, ".pearai", "sessions")
			if test.linked {
				held = t.TempDir()
				if err := os.Symlink(held, filepath.Join(home, ".pearai", "sessions")); err != nil {
					t.Fatal(err)
				}
			} else if err := os.MkdirAll(held, 0o700); err != nil {
				t.Fatal(err)
			}
			index := `[{"sessionId": "a", "workspaceDirectory": "/work/project"}]`
			if err := os.WriteFile(filepath.Join(held, continuedev.IndexName), []byte(index), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: chat, Original: "/h/" + rel, Rel: rel,
				Home: &input.Home{DiskPath: home, Original: "/h"}}
			var rows []timeline.Row
			if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || rows[0].ProjectPath != test.want {
				t.Errorf("rows %+v, want one with project %q", rows, test.want)
			}
		})
	}
}
