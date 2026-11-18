package littlecoder

import (
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// FuzzRecordReaders reads any text as a saved v0.0.x session, the saved
// sessions' history.json and the prompt history: the parser gives rows and
// problem rows, never a panic, and every row's text is valid UTF-8.
func FuzzRecordReaders(fuzz *testing.F) {
	for _, seed := range []string{
		`{"session_id": "s1", "saved_at": "2026-01-01T00:00:00", "messages": [{"role": "user", "content": "hi"},` +
			` {"role": "assistant", "content": "ok", "tool_calls": [{"name": "bash", "id": "c1", "input": {"command": "ls"}}]},` +
			` {"role": "tool", "name": "bash", "tool_call_id": "c1", "content": "Denied: user rejected this operation"}]}`,
		`{"session_id": "s1", "messages": [{"role": "user", "content": "cut`,
		`{"sessions": [{"session_id": "s1", "messages": [{"role": "user", "images": "ab"}]}, 5`,
		`["first", 5, ""]`, "\xff", ``,
	} {
		fuzz.Add(seed)
	}
	subject := New()
	rels := []string{".little-coder/sessions/2026/s1.json", legacyHistoryRel, historyRel}
	fuzz.Fuzz(func(t *testing.T, text string) {
		home := t.TempDir()
		for _, rel := range rels {
			path := filepath.Join(home, filepath.FromSlash(rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
			err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
				if !utf8.ValidString(row.Text) {
					t.Errorf("%s: row text %q is not valid UTF-8", rel, row.Text)
				}
			})
			if err != nil {
				t.Error(err)
			}
		}
	})
}
