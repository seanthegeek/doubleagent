package claudecode

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A subagent's file staged where the collection keeps a linked directory
// lies under directories of other names than its path in the home, so the
// walk up to subagents/ meets the home before it finds one: the session's
// tool-results/ is then not there.
func TestASubagentStagedUnderOtherNamesReadsNoSidecar(t *testing.T) {
	root := t.TempDir()
	const rel = ".claude/projects/p/s1/subagents/agent-a.jsonl"
	staged := filepath.Join(root, "staged", "physical")
	stub := "<persisted-output>\nFull output saved to: /x/s1/tool-results/out.txt\n</persisted-output>"
	line := `{"type":"user","timestamp":"2026-10-01T09:00:00.000Z","message":{"role":"user","content":` +
		`[{"type":"tool_result","tool_use_id":"t1","content":"` + strings.ReplaceAll(stub, "\n", `\n`) + `"}]}}` + "\n"
	for path, text := range map[string]string{
		filepath.Join(staged, "agent-a.jsonl"):                   line,
		filepath.Join(staged, "tool-results", "out.txt"):         "outside the session",
		filepath.Join(root, "staged", "tool-results", "out.txt"): "outside the session",
	} {
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	artifact := &input.Artifact{Rel: rel, DiskPath: filepath.Join(staged, "agent-a.jsonl"),
		Original: "/alice/" + rel, Home: &input.Home{DiskPath: filepath.Join(root, "home"),
			StagedPaths: map[string]string{".claude/projects/p/s1/subagents": staged}}}
	if _, found := sessionDir(artifact); found {
		t.Fatal("the walk found a session directory above the staged file")
	}
	var results []string
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
		if row.TurnType == timeline.TurnToolResult {
			results = append(results, row.Text)
		}
	}); err != nil {
		t.Fatal(err)
	}
	want := stub + "\n[persisted output not found: tool-results/out.txt]"
	if len(results) != 1 || results[0] != want {
		t.Errorf("got %q, want the stub with its note", results)
	}
}
