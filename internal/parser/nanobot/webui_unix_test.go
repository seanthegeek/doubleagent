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

// A chat's project comes from the namespaces whose listing holds its
// session file as a regular file, or, for a namespace that cannot be
// listed, from a look at the file itself; two namespaces holding it give
// their project only when they agree.
func TestAWebUIChatsProjectComesFromEveryNamespaceHoldingItsFile(t *testing.T) {
	file := base64.RawURLEncoding.EncodeToString([]byte("websocket:chat-1")) + ".jsonl"
	tests := []struct {
		name     string
		files    map[string]string
		unlisted string
		want     string
	}{
		{"one namespace holding the file gives its project",
			map[string]string{"a/" + file: "", "a/.workspace": "/work/a\n", "b/.workspace": "/work/b\n"}, "", "/work/a"},
		{"two namespaces of one project give it",
			map[string]string{"a/" + file: "", "b/" + file: "", "a/.workspace": "/work/a\n", "b/.workspace": "/work/a\n"},
			"", "/work/a"},
		{"two namespaces of two projects give none",
			map[string]string{"a/" + file: "", "b/" + file: "", "a/.workspace": "/work/a\n", "b/.workspace": "/work/b\n"},
			"", ""},
		{"a namespace that cannot be listed is looked in",
			map[string]string{"b/" + file: "", "a/.workspace": "/work/a\n", "b/.workspace": "/work/b\n"}, "b", "/work/b"},
		{"a namespace that cannot be listed and lacks the file gives nothing",
			map[string]string{"a/" + file: "", "a/.workspace": "/work/a\n", "b/.workspace": "/work/a\n"}, "b", "/work/a"},
		{"a directory of the file's name is no session file",
			map[string]string{"a/" + file + "/x": "", "a/.workspace": "/work/a\n"}, "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{".nanobot/webui/websocket_chat-1.jsonl": ""}
			for rel, text := range test.files {
				files[".nanobot/sessions/"+rel] = text
			}
			home := writeHome(t, files)
			if test.unlisted != "" {
				directory := filepath.Join(home, ".nanobot", "sessions", test.unlisted)
				if err := os.Chmod(directory, 0o300); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
			}
			const rel = ".nanobot/webui/websocket_chat-1.jsonl"
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
				Home: &input.Home{DiskPath: home}}
			transcript := &webui{Parser: New(), artifact: artifact, projects: map[string]string{}}
			if got := transcript.workspace("websocket:chat-1"); got != test.want || len(transcript.problems) > 0 {
				t.Errorf("workspace %q, problems %+v; want %q", got, transcript.problems, test.want)
			}
		})
	}
}
