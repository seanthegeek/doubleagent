//go:build unix

package openhands

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestAPromptHistoryNeverReadsALinkedConversation(t *testing.T) {
	const project = "/outside/project"
	sum := sha256.Sum256([]byte(project))
	history := ".openhands/projects/" + hex.EncodeToString(sum[:]) + "/prompt_history.json"
	state := `{"workspace": {"working_dir": "` + project + `"}}`
	tests := []struct {
		name, want string
		linked     bool
	}{
		{"a conversation directory gives the project", project, false},
		{"a link to a directory outside the home gives none", "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := writeTree(t, t.TempDir(), map[string]string{history: `[{"text": "hello"}]`})
			conversation := filepath.Join(home, ".openhands", "conversations", "0123456789abcdef0123456789abcdef")
			if test.linked {
				outside := writeTree(t, t.TempDir(), map[string]string{"base_state.json": state})
				if err := os.MkdirAll(filepath.Dir(conversation), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, conversation); err != nil {
					t.Fatal(err)
				}
			} else {
				writeTree(t, conversation, map[string]string{"base_state.json": state})
			}
			rows := parseRel(t, home, history)
			if len(rows) != 1 || rows[0].ProjectPath != test.want {
				t.Errorf("rows %+v, want one with project %q", rows, test.want)
			}
		})
	}
}
