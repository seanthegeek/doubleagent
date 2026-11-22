package claudedesktop

import (
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// An audit log is evidence: whatever its lines hold, the parser must not
// panic, and every row's text must be valid UTF-8.
func FuzzAuditLog(fuzz *testing.F) {
	for _, seed := range []string{
		`{"type":"system","subtype":"init","cwd":"/srv","model":"m","_audit_timestamp":"2026-10-01T09:00:00Z"}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"hi\xff"}]}}` + "\n" +
			`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"u","name":"Read","input":{}}]}}`,
		`{"type":"result","subtype":"success","result":"done"}` + "\n" + `{bad`,
	} {
		fuzz.Add([]byte(seed))
	}
	home := fuzz.TempDir()
	rel := "Library/Application Support/Claude/local-agent-mode-sessions/acc/org/local_s/audit.jsonl"
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
