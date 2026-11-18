//go:build unix

package nanobot

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
)

func TestAWebUISessionsWorkspaceIsNeverFoundThroughALinkedDirectory(t *testing.T) {
	const sessionID = "websocket:chat-1"
	const namespace = "0123456789abcdef0123456789abcdef"
	tests := []struct {
		name, linked, want string
	}{
		{"a session namespace in the state directory gives its workspace", "", "/work/project"},
		{"a linked sessions directory gives none", "sessions", ""},
		{"a linked namespace directory gives none", filepath.Join("sessions", namespace), ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			stateDirectory := filepath.Join(home, ".nanobot")
			files := map[string]string{
				filepath.Join("webui", "websocket_chat-1.jsonl"):                                                       "",
				filepath.Join("sessions", namespace, base64.RawURLEncoding.EncodeToString([]byte(sessionID))+".jsonl"): "",
				filepath.Join("sessions", namespace, ".workspace"):                                                     "/work/project\n",
			}
			for name, text := range files {
				path := filepath.Join(stateDirectory, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if test.linked != "" {
				inside := filepath.Join(stateDirectory, test.linked)
				outside := filepath.Join(t.TempDir(), "moved")
				if err := os.Rename(inside, outside); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, inside); err != nil {
					t.Fatal(err)
				}
			}
			const rel = ".nanobot/webui/websocket_chat-1.jsonl"
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
				Home: &input.Home{DiskPath: home}}
			transcript := &webui{Parser: New(), artifact: artifact, projects: map[string]string{}}
			if got := transcript.workspace(sessionID); got != test.want || len(transcript.problems) > 0 {
				t.Errorf("workspace %q, problems %+v; want %q", got, transcript.problems, test.want)
			}
		})
	}
}
