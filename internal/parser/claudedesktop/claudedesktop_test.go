package claudedesktop

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// The rows each file gives are proved against the Python analyzer by the
// parser replay (internal/parity/replay); these tests cover the routing and
// the record lookup, with the oracle's results.

// The two session trees of the tests, and a session dir's full name.
const (
	cowork  = "Library/Application Support/Claude/local-agent-mode-sessions/a/o"
	code    = "AppData/Roaming/Claude/claude-code-sessions/a/o"
	session = "local_c0c0c0c0-1111-4222-8333-444444444444"
)

func TestWantsAndSecondaryFollowTheAppDataLayout(t *testing.T) {
	tests := []struct {
		rel             string
		wants, isSecond bool
	}{
		{cowork + "/" + session + "/.claude/projects/session/x.jsonl", true, false},
		{cowork + "/agent/" + session + "/.claude/projects/p/x/subagents/agent-1.jsonl", true, false},
		{cowork + "/" + session + "/.claude/projects/p/x/subagents/d/agent-1.jsonl", true, false},
		{cowork + "/" + session + "/.claude/projects/p/notes.txt", false, false},
		{code + "/imported-staging/x.jsonl", true, false},
		{cowork + "/imported-staging/x.jsonl", false, false},
		{cowork + "/" + session + "/audit.jsonl", true, true},
		{cowork + "/agent/f3f3f3f3/audit.jsonl", true, true},
		{code + "/f3/audit.jsonl", false, false},
		{cowork + "/" + session + ".json", true, true},
		{code + "/agent/" + session + ".json", true, true},
		{cowork + "/local_xyz.json", false, false},
		{cowork + "/scheduled-tasks.json", true, false},
		{code + "/scheduled-tasks.json", true, false},
		{"Library/Application Support/Claude/git-worktrees.json", true, false},
		{".config/Claude-3p/git-worktrees.json", true, false},
		{cowork + "/remote-session-spaces.json", true, false},
		{code + "/remote-session-spaces.json", false, false},
		{"AppData/Local/Claude-Data/x/git-worktrees.json", true, false},
		{"AppData/Local/Packages/AnthropicPBC.Claude_abc/LocalCache/Roaming/Claude/git-worktrees.json", true, false},
		{"AppData/Local/Packages/Claude_abc/LocalCache/Roaming/Claude/git-worktrees.json", true, false},
		{"Claude/git-worktrees.json", false, false},
		{".claude/projects/p/s.jsonl", false, false},
	}
	subject := New()
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			if got := subject.Wants(artifact); got != test.wants {
				t.Errorf("wants: got %v, want %v", got, test.wants)
			}
			if got := subject.Secondary(artifact); got != test.isSecond {
				t.Errorf("secondary: got %v, want %v", got, test.isSecond)
			}
		})
	}
}

func TestTheParserRecordsNoMarksAndDeclaresNoHooks(t *testing.T) {
	subject := New()
	if subject.Agent() != "claude-desktop" || subject.Name() != "claude-desktop" {
		t.Errorf("agent %q, name %q", subject.Agent(), subject.Name())
	}
	if got := parser.MarksOf(subject); got != (parser.Marks{}) {
		t.Errorf("marks %+v", got)
	}
	if !parser.HasSecondary(subject) {
		t.Error("the parser reads secondary sources")
	}
	if _, ok := parser.Parser(subject).(parser.HooksDeclarer); ok {
		t.Error("the desktop's rows are matched under the default form")
	}
}

func TestARecordIsFoundWhereTheCollectionStagedTheSessionsOrg(t *testing.T) {
	root := t.TempDir()
	home, staged := filepath.Join(root, "home"), filepath.Join(root, "staged", "org")
	if err := osfile.MkdirAll(filepath.Join(staged, session), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(filepath.Join(staged, session+".json"), []byte(`{"title": "staged"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	collected := &input.Home{DiskPath: home, StagedPaths: map[string]string{cowork: staged}}
	if record, found := findRecord(collected, cowork+"/"+session); !found || text(record.Get("title")) != "staged" {
		t.Errorf("got %v, %v; want the staged org's record", record, found)
	}
	if _, found := findRecord(&input.Home{DiskPath: home}, cowork+"/"+session); found {
		t.Error("a record was found in a home that holds none")
	}
}

func TestARecordIsFoundByTheSessionDirsFullOrShortName(t *testing.T) {
	short := "c0c0c0c0"
	tests := []struct {
		name, sessionDir, record, content, want string
	}{
		{"a full name beside the record", session, session + ".json", `{"title": "full"}`, "full"},
		{"a short name finds the record", short, session + ".json", `{"title": "short"}`, "short"},
		{"a record in agent/", "agent/" + short, "agent/" + session + ".json", `{"title": "agent"}`, "agent"},
		{"a record that is not an object", short, session + ".json", `[1]`, ""},
		{"a name that is no session's", "other", "local_other.json", `{"title": "x"}`, ""},
		{"a short name of other digits", "c0c0c0c1", session + ".json", `{"title": "x"}`, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			org := t.TempDir()
			for _, rel := range []string{test.sessionDir, test.record} {
				if err := osfile.MkdirAll(filepath.Dir(filepath.Join(org, rel)), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(org, filepath.FromSlash(test.record))
			if err := osfile.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			home := &input.Home{DiskPath: filepath.Dir(org)}
			record, _ := findRecord(home, filepath.Base(org)+"/"+test.sessionDir)
			if got := text(record.Get("title")); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
