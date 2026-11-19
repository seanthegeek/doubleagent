package letta

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/parsertest"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestATranscriptsServerShowsItsCredentialsUnlessWithheld(t *testing.T) {
	const rel = ".letta/transcripts/agent-1/conv-1/transcript.jsonl"
	files := map[string]string{
		settingsRel: `{"agents": [{"agentId": "agent-1", "baseUrl": "https://tok:s3cret@letta.example/"}]}`,
		rel:         `{"captured_at": "2026-10-01T09:00:00Z"}` + "\n",
	}
	tests := []struct {
		name    string
		setting secrets.Setting
		server  string
	}{
		{"shown by default", secrets.Setting{}, "https://tok:s3cret@letta.example/"},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			"https://REDACTED@letta.example/"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			"https://[withheld]@letta.example/"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, "https://letta.example/"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			for name, text := range files {
				path := filepath.Join(home, filepath.FromSlash(name))
				if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)),
				Original: "/home/u/" + rel, Rel: rel, Agent: "letta", Manifest: jsonvalue.NewNull(),
				Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: secrets.NewRecorder(test.setting)}}
			var got []string
			err := parsertest.Parse(t, New(), artifact, parser.Options{}, func(row timeline.Row) { got = append(got, row.Text) })
			if err != nil {
				t.Fatal(err)
			}
			want := "session synced: server conversation conv-1 agent agent-1 server " + test.server + " "
			if len(got) == 0 || !strings.HasPrefix(got[0], want) {
				t.Errorf("got %q, want a first row starting %q", got, want)
			}
			if test.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "s3cret") {
				t.Errorf("a withheld run shows the password: %q", got)
			}
		})
	}
}
