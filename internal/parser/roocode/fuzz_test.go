package roocode

import (
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A task file is evidence: whatever its array holds, the parser must not
// panic, and every row's text must be valid UTF-8.
func FuzzTaskFiles(fuzz *testing.F) {
	for _, seed := range []string{
		`[{"ts":1791021600000,"type":"say","say":"text","text":"hi"},{"ts":1791021601000,"type":"ask","ask":"tool","text":"{\"tool\":\"readFile\",\"path\":\"a\"}"}]`,
		`[{"role":"user","content":[{"type":"text","text":"hi\xff"}]},{"role":"assistant","content":[{"type":"tool_use","id":"u","name":"x","input":{}}]}]`,
		`[{"ts":1,"type":"say","say":"api_req_started","text":"{\"cost\":1}"}, {"a":`,
		`{"not":"an array"}`,
	} {
		fuzz.Add([]byte(seed), false)
		fuzz.Add([]byte(seed), true)
	}
	home := fuzz.TempDir()
	directory := filepath.Join(home, filepath.FromSlash(".vscode-mock/global-storage/tasks/1791021600000"))
	if err := osfile.MkdirAll(directory, 0o700); err != nil {
		fuzz.Fatal(err)
	}
	fuzz.Fuzz(func(t *testing.T, data []byte, api bool) {
		name := "ui_messages.json"
		if api {
			name = "api_conversation_history.json"
		}
		path := filepath.Join(directory, name)
		if err := osfile.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		defer osfile.Remove(path)
		artifact := &input.Artifact{Rel: ".vscode-mock/global-storage/tasks/1791021600000/" + name, DiskPath: path, Home: &input.Home{DiskPath: home}}
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
