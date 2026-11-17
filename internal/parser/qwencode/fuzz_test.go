package qwencode

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A chat file or a logs.json is evidence: whatever it holds, reading it
// must not panic, and the text helpers must take any text.
func FuzzChatFileAndLogs(fuzz *testing.F) {
	for _, seed := range []string{
		`{"type":"user","sessionId":"s","message":{"parts":[{"text":"[Chat messages since your ` +
			`last reply - for context]\n- [a] b\n\n[Current message - respond to this]"}]}}`,
		`{"type":"system","subtype":"session_approval_mode","systemPayload":{"mode":"plan"}}` + "\n" +
			`{"type":"tool_result","toolCallResult":{"status":"cancelled"},"message":{"parts":[{}]}}`,
		`[{"type":"user","message":"hi"}, {"type":"model_switch","message":"{\"toModel\":1}"} `,
	} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		home := t.TempDir()
		for _, rel := range []string{".qwen/projects/p/chats/e5f6a7b8-4444-4000-8000-000000000011.jsonl",
			".qwen/tmp/h/logs.json", ".qwen/channels/x-group-history.jsonl", ".qwen/usage_record.jsonl"} {
			disk := filepath.Join(home, filepath.FromSlash(rel))
			if err := osfile.MkdirAll(filepath.Dir(disk), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(disk, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: disk, Rel: rel, Home: &input.Home{DiskPath: home}}
			if err := New().Parse(artifact, parser.Options{IncludeThinking: true}, func(row timeline.Row) {
				MatchForm(row)
				MatchAnchor(row)
			}); err != nil {
				t.Fatal(err)
			}
		}
		quotedGroupMessages(text)
		splitURL(text)
	})
}
