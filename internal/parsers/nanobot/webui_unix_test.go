//go:build unix

package nanobot

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
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
// session file as a regular file; two namespaces holding it give their
// project only when they agree. A namespace that cannot be listed holds no
// session file, and the transcript that listed it gets its one problem.
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
		{"a namespace that cannot be listed is not looked in",
			map[string]string{"b/" + file: "", "a/.workspace": "/work/a\n", "b/.workspace": "/work/b\n"}, "b", ""},
		{"a namespace that cannot be listed leaves the others' project",
			map[string]string{"a/" + file: "", "b/" + file: "", "a/.workspace": "/work/a\n", "b/.workspace": "/work/b\n"},
			"b", "/work/a"},
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
			var wantProblems []string
			if test.unlisted != "" {
				directory := filepath.Join(home, ".nanobot", "sessions", test.unlisted)
				if err := os.Chmod(directory, 0o300); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
				if _, err := os.ReadDir(directory); err == nil {
					t.Skip("a directory without read permission can still be listed by this user")
				}
				wantProblems = []string{test.unlisted + " could not be read (permission denied)"}
			}
			const rel = ".nanobot/webui/websocket_chat-1.jsonl"
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
				Home: &input.Home{DiskPath: home}}
			transcript := &webui{Parser: New(), artifact: artifact, projects: map[string]string{}}
			got := transcript.workspace("websocket:chat-1")
			var problems []string
			for _, problem := range transcript.problems {
				problems = append(problems, problem.Message)
			}
			if got != test.want || !slices.Equal(problems, wantProblems) {
				t.Errorf("workspace %q, problems %q; want %q, %q", got, problems, test.want, wantProblems)
			}
		})
	}
}

// Every WebUI file of a run looks for its chats' session files in every
// session namespace, so a namespace that cannot be listed is remembered as
// failed when the sessions directory is first listed, with one problem for
// the run: as many chats as such namespaces look only where a listing
// holds their file, not in each of them.
func TestANamespaceThatCannotBeListedIsLookedInByNoChat(t *testing.T) {
	for _, n := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d files beside %d namespaces that cannot be listed", n, n), func(t *testing.T) {
			files := map[string]string{
				".nanobot/sessions/open/" + base64.RawURLEncoding.EncodeToString([]byte("websocket:chat-0")) + ".jsonl": "",
				".nanobot/sessions/open/.workspace": "/work/open\n"}
			for i := range n {
				files[fmt.Sprintf(".nanobot/sessions/locked%03d/.workspace", i)] = "/work/locked\n"
				files[fmt.Sprintf(".nanobot/webui/w%03d.jsonl", i)] =
					fmt.Sprintf(`{"event": "user", "chat_id": "chat-%d", "text": "hi"}`, i) + "\n"
			}
			home := writeHome(t, files)
			for i := range n {
				directory := filepath.Join(home, ".nanobot", "sessions", fmt.Sprintf("locked%03d", i))
				if err := os.Chmod(directory, 0o000); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
			}
			if _, err := os.ReadDir(filepath.Join(home, ".nanobot", "sessions", "locked000")); err == nil {
				t.Skip("a directory without read permission can still be listed by this user")
			}
			subject := New()
			subject.Work = parserkit.CountingWork()
			found, problems := 0, 0
			for i := range n {
				rel := fmt.Sprintf(".nanobot/webui/w%03d.jsonl", i)
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
					Home: &input.Home{DiskPath: home}, Manifest: jsonvalue.NewNull()}
				if err := subject.Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
					switch {
					case strings.HasPrefix(row.Text, parserkit.ProblemPrefix+"locked"):
						problems++
					case row.ProjectPath == "/work/open":
						found++
					}
				}); err != nil {
					t.Fatal(err)
				}
			}
			lookups := subject.Work.Count("namespace lookups")
			if found != 1 || problems != n || lookups != 1 {
				t.Errorf("%d found, %d problems and %d lookups; want 1, %d and 1", found, problems, lookups, n)
			}
		})
	}
}
