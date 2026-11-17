package claudecode

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

func TestAPersistedOutputNameThatIsNotAFileNameIsNotFound(t *testing.T) {
	home := t.TempDir()
	const rel = ".claude/projects/p/s1.jsonl"
	results := filepath.Join(home, ".claude", "projects", "p", "s1", "tool-results")
	if err := osfile.MkdirAll(results, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(filepath.Join(results, "out.txt"), []byte("full output"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
		Home: &input.Home{DiskPath: home}}
	tests := []struct {
		name, saved, want string
	}{
		{"a name of .. is not the session directory", "/x/s1/tool-results/..",
			"stub\n[persisted output not found: tool-results/..]"},
		{"a name of . is not tool-results itself", "/x/s1/tool-results/.",
			"stub\n[persisted output not found: tool-results/.]"},
		{"an empty name after a separator is not found", `C:\x\s1\tool-results\`,
			"stub\n[persisted output not found: tool-results/]"},
		{"a name holding a NUL is not looked for", "/x/s1/tool-results/out.txt\x00",
			"stub\n[persisted output not found: tool-results/out.txt\x00]"},
		{"a plain name is read", `C:\x\s1\tool-results\out.txt`, "full output"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems parser.Problems
			if got := persistedText(artifact, "stub", test.saved, &problems); got != test.want ||
				len(problems) != 0 {
				t.Errorf("got %q, %v; want %q", got, problems, test.want)
			}
		})
	}
}
