//go:build unix

package continuedev

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
)

func TestADevDataFileNeverReadsAnIndexThroughALinkedDirectory(t *testing.T) {
	tests := []struct {
		name    string
		linked  bool
		entries int
	}{
		{"a sessions directory beside dev_data gives its index", false, 1},
		{"a sessions directory linked to one outside the home gives none", true, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			const rel = ".continue/dev_data/0.2.0/chatInteraction.jsonl"
			if err := os.MkdirAll(filepath.Join(home, ".continue", "dev_data", "0.2.0"), 0o700); err != nil {
				t.Fatal(err)
			}
			held := filepath.Join(home, ".continue", "sessions")
			if test.linked {
				held = t.TempDir()
				if err := os.Symlink(held, filepath.Join(home, ".continue", "sessions")); err != nil {
					t.Fatal(err)
				}
			} else if err := os.MkdirAll(held, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(held, IndexName), []byte(`[{"sessionId": "a"}]`), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
				Home: &input.Home{DiskPath: home}}
			if got := len(New().devIndex(artifact)); got != test.entries {
				t.Errorf("%d index entries, want %d", got, test.entries)
			}
		})
	}
}
