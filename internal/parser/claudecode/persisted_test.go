package claudecode

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// persistedStub is the tool result a transcript keeps for an output saved
// to tool-results/<name>.
func persistedStub(name string) string {
	return "<persisted-output>\nOutput too large. Full output saved to: /x/s1/tool-results/" + name +
		"\n</persisted-output>"
}

// toolResultRows writes a session file s1.jsonl of the given tool result
// stubs, one line each, beside s1/tool-results/ holding files, and returns
// the texts of the tool_result rows the parser gives it.
func toolResultRows(t *testing.T, stubs []string, files map[string]string) []string {
	t.Helper()
	home := t.TempDir()
	const rel = ".claude/projects/p/s1.jsonl"
	results := filepath.Join(home, ".claude", "projects", "p", "s1", "tool-results")
	if err := osfile.MkdirAll(results, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, text := range files {
		if err := osfile.WriteFile(filepath.Join(results, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lines := make([]string, len(stubs))
	for i, stub := range stubs {
		block := jsonvalue.NewObject(
			jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("tool_result")},
			jsonvalue.Member{Key: "tool_use_id", Value: jsonvalue.NewString("t" + strconv.Itoa(i))},
			jsonvalue.Member{Key: "content", Value: jsonvalue.NewString(stub)})
		lines[i] = `{"type": "user", "sessionId": "s1", "timestamp": "2026-01-02T03:04:05.000Z", ` +
			`"message": {"role": "user", "content": [` + block.JSON() + `]}}`
	}
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "claude-code",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var texts []string
	emit := func(row timeline.Row) {
		if row.TurnType == timeline.TurnToolResult {
			texts = append(texts, row.Text)
		}
	}
	if err := New().Parse(artifact, parser.Options{}, emit); err != nil {
		t.Fatal(err)
	}
	return texts
}

// persistedPointer is the row of a later reference to tool-results/<name>.
func persistedPointer(name string) string {
	return persistedStub(name) + "\n[persisted output shown in full at its first reference: tool-results/" + name + "]"
}

func TestAPersistedOutputNamedByManyStubsIsWrittenOnce(t *testing.T) {
	for _, size := range []int{100, 400} {
		output := strings.Repeat("z", 10*size)
		stubs := make([]string, size)
		for i := range stubs {
			stubs[i] = persistedStub("a.txt")
		}
		texts := toolResultRows(t, stubs, map[string]string{"a.txt": output})
		got := 0
		for _, text := range texts {
			got += len(text)
		}
		want := len(output) + (size-1)*len(persistedPointer("a.txt"))
		if got != want || len(texts) != size || texts[0] != output || texts[size-1] != persistedPointer("a.txt") {
			t.Errorf("%d stubs naming one file of %d bytes: %d rows of %d bytes, want %d rows of %d bytes",
				size, len(output), len(texts), got, size, want)
		}
	}
}

func TestEachPersistedOutputIsShownInFullAtItsOwnFirstReference(t *testing.T) {
	stubs := []string{persistedStub("a.txt"), persistedStub("b.txt"), persistedStub("a.txt"),
		persistedStub("gone.txt"), persistedStub("gone.txt"), persistedStub("b.txt")}
	texts := toolResultRows(t, stubs, map[string]string{"a.txt": "output A", "b.txt": "output B"})
	missing := persistedStub("gone.txt") + "\n[persisted output not found: tool-results/gone.txt]"
	want := []string{"output A", "output B", persistedPointer("a.txt"), missing, missing, persistedPointer("b.txt")}
	if strings.Join(texts, "\n|\n") != strings.Join(want, "\n|\n") {
		t.Errorf("got %q, want %q", texts, want)
	}
}
