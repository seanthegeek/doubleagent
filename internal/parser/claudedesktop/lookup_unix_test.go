//go:build unix

package claudedesktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// linkOutside moves the directory at the slash-separated rel below home to
// a new directory outside it and puts a symbolic link to it in its place.
func linkOutside(t *testing.T, home, rel string) {
	t.Helper()
	inside := filepath.Join(home, filepath.FromSlash(rel))
	outside := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(inside, outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, inside); err != nil {
		t.Fatal(err)
	}
}

func TestASessionIsNeverCountedCollectedThroughALinkedDirectory(t *testing.T) {
	const cliID = "c1"
	tests := []struct {
		name, file, linked string
		want               bool
	}{
		{"an audit log in the session dir counts", session + "/audit.jsonl", "", true},
		{"an audit log in a linked session dir does not", session + "/audit.jsonl", session, false},
		{"a transcript in the session dir counts", session + "/.claude/projects/p/t.jsonl", "", true},
		{"a transcript below a linked session dir does not", session + "/.claude/projects/p/t.jsonl",
			session, false},
		{"a transcript below a linked .claude does not", session + "/.claude/projects/p/t.jsonl",
			session + "/.claude", false},
		{"a transcript below a linked projects does not", session + "/.claude/projects/p/t.jsonl",
			session + "/.claude/projects", false},
		{"a transcript in a linked project slug does not", session + "/.claude/projects/p/t.jsonl",
			session + "/.claude/projects/p", false},
		{"a staged CLI transcript counts", "imported-staging/" + cliID + ".jsonl", "", true},
		{"a CLI transcript in a linked imported-staging does not", "imported-staging/" + cliID + ".jsonl",
			"imported-staging", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			writeEmpty(t, home, cowork+"/"+test.file)
			if test.linked != "" {
				linkOutside(t, home, cowork+"/"+test.linked)
			}
			rel := cowork + "/" + session + ".json"
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
				Home: &input.Home{DiskPath: home}}
			record := jsonvalue.NewObject(
				jsonvalue.Member{Key: "sessionId", Value: jsonvalue.NewString(session)},
				jsonvalue.Member{Key: "cliSessionId", Value: jsonvalue.NewString(cliID)})
			if got := sessionCollected(artifact, record); got != test.want {
				t.Errorf("collected %v, want %v", got, test.want)
			}
		})
	}
}

func TestARecordIsNeverFoundThroughALinkedAgentDirectory(t *testing.T) {
	tests := []struct {
		name, want string
		linked     bool
	}{
		{"a record in agent/ is found", "agent", false},
		{"a record in a linked agent/ is not", "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			writeEmpty(t, home, cowork+"/c0c0c0c0/.keep")
			writeEmpty(t, home, cowork+"/agent/.keep")
			record := filepath.Join(home, filepath.FromSlash(cowork), "agent", session+".json")
			if err := os.WriteFile(record, []byte(`{"title": "agent"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if test.linked {
				linkOutside(t, home, cowork+"/agent")
			}
			found, _ := findRecord(&input.Home{DiskPath: home}, cowork+"/c0c0c0c0")
			if got := found.Get("title").Text(); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
