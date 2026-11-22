package claudedesktop

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// problemRowsOf writes text at rel under a new home, parses it and returns
// the texts of the rows that start with "parser:".
func problemRowsOf(t *testing.T, rel, text string) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
	var texts []string
	err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) {
		if strings.HasPrefix(row.Text, parser.ProblemPrefix) {
			texts = append(texts, row.Text)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestAMemberOfTheWrongKindIsReported(t *testing.T) {
	tests := []struct {
		name, rel, document, wrong string
	}{
		{"alwaysAllowedReasons", cowork + "/" + session + ".json", `{"alwaysAllowedReasons": %s}`,
			"parser: alwaysAllowedReasons is not a JSON array"},
		{"sessionPermissionUpdates", code + "/" + session + ".json", `{"sessionPermissionUpdates": %s}`,
			"parser: sessionPermissionUpdates is not a JSON array"},
		{"scheduledTasks", cowork + "/scheduled-tasks.json", `{"scheduledTasks": %s}`,
			"parser: scheduledTasks is not a JSON array"},
		{"entries", cowork + "/remote-session-spaces.json", `{"entries": %s}`,
			"parser: entries is not a JSON array"},
		{"entries.folders", cowork + "/remote-session-spaces.json", `{"entries": [{"sessionId": "r", "folders": %s}]}`,
			"parser: entries.folders is not a JSON array"},
	}
	for _, test := range tests {
		for _, value := range []string{`"x"`, `7`, `{"a": 1}`} {
			t.Run(test.name+" as "+value, func(t *testing.T) {
				got := problemRowsOf(t, test.rel, strings.Replace(test.document, "%s", value, 1))
				if want := []string{test.wrong}; !slices.Equal(got, want) {
					t.Errorf("rows %q, want %q", got, want)
				}
			})
		}
		for _, value := range []string{`null`, `[]`} {
			t.Run(test.name+" as "+value, func(t *testing.T) {
				if got := problemRowsOf(t, test.rel, strings.Replace(test.document, "%s", value, 1)); len(got) != 0 {
					t.Errorf("rows %q, want none", got)
				}
			})
		}
	}
	t.Run("absent members", func(t *testing.T) {
		for _, rel := range []string{cowork + "/" + session + ".json", cowork + "/scheduled-tasks.json",
			cowork + "/remote-session-spaces.json", "Library/Application Support/Claude/git-worktrees.json"} {
			if got := problemRowsOf(t, rel, `{"entries": [{}]}`); len(got) != 0 {
				t.Errorf("%s: rows %q, want none", rel, got)
			}
		}
	})
}

func TestWorktreesThatIsNotAnObjectIsReported(t *testing.T) {
	const rel = "Library/Application Support/Claude/git-worktrees.json"
	for _, value := range []string{`"x"`, `7`, `[{"path": "p"}]`} {
		t.Run(value, func(t *testing.T) {
			got := problemRowsOf(t, rel, `{"worktrees": `+value+`}`)
			if want := []string{"parser: worktrees is not a JSON object"}; !slices.Equal(got, want) {
				t.Errorf("rows %q, want %q", got, want)
			}
		})
	}
	for _, value := range []string{`null`, `{}`} {
		t.Run(value, func(t *testing.T) {
			if got := problemRowsOf(t, rel, `{"worktrees": `+value+`}`); len(got) != 0 {
				t.Errorf("rows %q, want none", got)
			}
		})
	}
}

func TestEntriesThatAreNotObjectsAreReportedOncePerList(t *testing.T) {
	tests := []struct {
		name, rel, text, want string
	}{
		{"scheduled tasks", cowork + "/scheduled-tasks.json", `{"scheduledTasks": [{"id": "a"}, 7]}`,
			"parser: scheduledTasks holds 1 entry that is not a JSON object"},
		{"worktrees", "Library/Application Support/Claude/git-worktrees.json",
			`{"worktrees": {"a": {"path": "p"}, "b": "x", "c": 7}}`,
			"parser: worktrees holds 2 entries that are not JSON objects"},
		{"remote session spaces", cowork + "/remote-session-spaces.json", `{"entries": ["x"]}`,
			"parser: entries holds 1 entry that is not a JSON object"},
		{"audit content blocks", cowork + "/" + session + "/audit.jsonl",
			`{"type": "assistant", "message": {"content": [{"type": "text", "text": "ok"}, 7]}}` + "\n",
			"parser: message.content holds 1 entry that is not a JSON object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := problemRowsOf(t, test.rel, test.text); !slices.Equal(got, []string{test.want}) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}
