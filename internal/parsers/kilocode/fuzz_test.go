package kilocode

import (
	"testing"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A pre-fork task's history is evidence: whatever its array holds, the
// parser must not panic, and every row's text must be valid UTF-8.
func FuzzTaskHistory(fuzz *testing.F) {
	for _, seed := range []string{
		`[{"role":"user","content":"hi","ts":1791021600000},{"role":"assistant","content":[{"type":"text","text":"a"},` +
			`{"type":"tool_use","id":"u","name":"x","input":{"a":1}},{"type":"thinking","thinking":"t"}]}]`,
		`[{"type":"reasoning","summary":[{"text":"s\xff"}]},{"isSummary":true,"content":"c"},{"role":"user","content":[{"type":"tool_result","tool_use_id":"u","is_error":true,"content":"e"}]}]`,
		`[{"role":"user"}, {"a":`,
		`[] x`,
	} {
		fuzz.Add([]byte(seed))
	}
	home := fuzz.TempDir()
	artifact := taskArtifact(home, "1791021600000")
	write(fuzz, artifact.DiskPath, "")
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		if err := osfile.WriteFile(artifact.DiskPath, data, 0o600); err != nil {
			t.Fatal(err)
		}
		err := New().Parse(artifact, parserkit.Options{IncludeThinking: true}, func(row timeline.Row) {
			if !utf8.ValidString(row.Text) {
				t.Errorf("row text %q is not valid UTF-8", row.Text)
			}
		})
		if err != nil {
			t.Errorf("Parse returned %v", err)
		}
	})
}
