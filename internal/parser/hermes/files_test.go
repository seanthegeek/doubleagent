package hermes

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseFile writes text into a new home at rel and returns the text of
// every row the parser gives for it.
func parseFile(t *testing.T, rel, text string) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "hermes",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var texts []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestAFileHoldingSecretsThatIsNotJSONGivesOnlyThePosition(t *testing.T) {
	tests := []struct {
		name, rel, text, want string
	}{
		{"the desktop app's connections", ".config/Hermes/connections.json",
			`{"connections": [{"kind": "remote", "token": sk-SECRET}]}`,
			"parser: connections.json did not parse (invalid JSON at line 1, column 46)"},
		{"a platform's approved users", ".hermes/pairing/telegram-approved.json",
			`{"12345": {"code": 0123456789}}`,
			"parser: pairing/telegram-approved.json did not parse (invalid JSON at line 1, column 21)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseFile(t, test.rel, test.text); !slices.Equal(got, []string{test.want}) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestARemoteBackendsURLIsShownWithoutItsCredentials(t *testing.T) {
	got := parseFile(t, ".config/Hermes/connections.json", `{"connections": [`+
		`{"kind": "remote", "label": "work", "url": "https://alice:s3cret@hermes.example:9119/"},`+
		`{"kind": "ssh", "label": "box", "host": "box.example", "user": "alice", "port": 22}]}`)
	want := []string{
		"remote backend: remote work https://redacted@hermes.example:9119/ (from connections.json, file mtime)",
		"remote backend: ssh box alice@box.example:22 (from connections.json, file mtime)",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
