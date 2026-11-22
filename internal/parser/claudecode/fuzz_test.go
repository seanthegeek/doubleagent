package claudecode

import (
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A session transcript is evidence: whatever its lines hold, the parser must
// not panic, and every row's text must be valid UTF-8.
func FuzzSessionTranscript(fuzz *testing.F) {
	for _, seed := range []string{
		`{"type":"user","sessionId":"s","cwd":"/srv","timestamp":"2026-10-01T09:00:00Z","message":{"role":"user","content":"hi"}}`,
		`{"type":"assistant","message":{"role":"assistant","model":"m","content":[{"type":"thinking","thinking":"t"},{"type":"tool_use","id":"u","name":"Bash","input":{"command":"ls"}}]}}` + "\n" +
			`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"u","content":"\xff<persisted-output>"}]}}`,
		`{"type":"summary","summary":"s"}` + "\n" + `{bad` + "\n" + `[1]`,
	} {
		fuzz.Add([]byte(seed))
	}
	home := fuzz.TempDir()
	rel := ".claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl"
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		fuzz.Fatal(err)
	}
	artifact := &input.Artifact{Rel: rel, DiskPath: path, Home: &input.Home{DiskPath: home}}
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		if err := osfile.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		err := New().Parse(artifact, parser.Options{IncludeThinking: true}, func(row timeline.Row) {
			if !utf8.ValidString(row.Text) {
				t.Errorf("row text %q is not valid UTF-8", row.Text)
			}
		})
		if err != nil {
			t.Errorf("Parse returned %v", err)
		}
	})
}
