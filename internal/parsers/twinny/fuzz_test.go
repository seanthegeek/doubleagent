package twinny

import (
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// FuzzGlobalState reads any JSON object as the extension's global state,
// its conversations and standing grants: the parser gives rows, never a
// panic, and every row's text is valid UTF-8.
func FuzzGlobalState(fuzz *testing.F) {
	for _, seed := range []string{
		`{"twinny.conversations": {"c1": {"id": "c1", "title": "t", "updatedAt": 1767225600000, "messages": [` +
			`{"role": "user", "content": "hi", "images": [1]},` +
			`{"role": "assistant", "content": "<think>plan</think>ok", "meta": {"model": "m", "withheld": ["k"]},` +
			` "toolSteps": [{"name": "run", "id": "t1", "command": "ls", "output": "x", "status": "waiting"},` +
			` {"said": true, "summary": {"a": 1}}], "changes": {"id": "r1", "files": [{"status": "M", "path": "a"}]}}]}},` +
			` "twinny-global-context-alwaysRunCommands": ["ls"], "twinny.teamPolicy": {"a": [{"b": 1}, null]}}`,
		`{"twinny.conversations": [{"messages": [{"content": [{"type": "text", "text": "<thinking>x"}]}]}],` +
			` "twinny.active-conversation": {"id": "c2", "messages": [{"role": 5}]}}`,
		`{"twinny.conversations": {"c": {"messages": [{"role": "assistant", "content": "a<think>b\n"}]}}}`,
		`{}`,
	} {
		fuzz.Add(seed)
	}
	subject := New()
	artifact := &input.Artifact{DiskPath: filepath.Join(fuzz.TempDir(), "state.vscdb"), Original: "/h/User/globalStorage/state.vscdb",
		Rel: "User/globalStorage/state.vscdb"}
	fuzz.Fuzz(func(t *testing.T, text string) {
		state, err := jsonvalue.DecodeString(text)
		if err != nil || !state.IsObject() {
			return
		}
		subject.globalRows(artifact, state, parserkit.Options{IncludeThinking: true}, func(row timeline.Row) {
			if !utf8.ValidString(row.Text) {
				t.Errorf("row text %q is not valid UTF-8", row.Text)
			}
		})
	})
}
