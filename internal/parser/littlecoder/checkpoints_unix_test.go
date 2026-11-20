//go:build unix

package littlecoder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
)

func TestAPiSessionIsNeverFoundThroughALinkedDirectory(t *testing.T) {
	const name = "2026-01-01T00-00-00-000Z_s1.jsonl"
	tests := []struct {
		name, linked, want string
	}{
		{"a project directory of the sessions gives the header", "", "/work/project"},
		{"a linked sessions directory gives none", "sessions", ""},
		{"a linked project directory gives none", filepath.Join("sessions", "--work-project--"), ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			agent := filepath.Join(home, ".pi", "agent")
			project := filepath.Join(agent, "sessions", "--work-project--")
			if err := os.MkdirAll(project, 0o700); err != nil {
				t.Fatal(err)
			}
			header := `{"type": "session", "id": "s1", "cwd": "/work/project"}` + "\n"
			if err := os.WriteFile(filepath.Join(project, name), []byte(header), 0o600); err != nil {
				t.Fatal(err)
			}
			if test.linked != "" {
				inside := filepath.Join(agent, test.linked)
				outside := filepath.Join(t.TempDir(), "moved")
				if err := os.Rename(inside, outside); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, inside); err != nil {
					t.Fatal(err)
				}
			}
			const rel = ".little-coder/checkpoints/" + name + "/file.py"
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
				Home: &input.Home{DiskPath: home}}
			found, problems := New().sessionHeader(artifact, name)
			if got := found.Get("cwd").Text(); got != test.want || len(problems) > 0 {
				t.Errorf("header cwd %q, problems %+v; want %q", got, problems, test.want)
			}
		})
	}
}

// A directory that cannot be listed, or can be listed but not searched,
// gives what a search of every directory in turn gives: the file it holds
// where it can be looked up, else the problem of the first such directory
// when no directory holds the file.
func TestADirectoryThatCannotBeListedOrSearchedGivesWhatEveryDirectoryInTurnGives(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions do not bind the superuser")
	}
	header := func(cwd string) string { return `{"type": "session", "id": "x", "cwd": "` + cwd + `"}` + "\n" }
	tests := []struct {
		name, locked string
		mode         os.FileMode
	}{
		{"a project directory that cannot be listed", ".pi/agent/sessions/p1", 0o300},
		{"a project directory that cannot be searched", ".pi/agent/sessions/p1", 0o600},
		{"a later project directory that cannot be searched", ".pi/agent/sessions/p3", 0o600},
		{"a sessions directory that cannot be listed", ".pi/agent/sessions", 0o300},
		{"a sessions directory that cannot be searched", ".pi/agent/sessions", 0o600},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := writeHome(t, map[string]string{".pi/agent/sessions/p1/a_s1.jsonl": header("/locked"),
				".pi/agent/sessions/p2/a_s1.jsonl": header("/open"), ".pi/agent/sessions/p2/b_s2.jsonl": header("/b"),
				".pi/agent/sessions/p3/e_s5.jsonl": header("/e")})
			locked := filepath.Join(home.DiskPath, filepath.FromSlash(test.locked))
			if err := os.Chmod(locked, test.mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
			checkSameHeaders(t, home)
		})
	}
}
