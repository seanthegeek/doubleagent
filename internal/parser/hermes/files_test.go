package hermes

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseFile writes text into a new home at rel and returns the text of
// every row the parser gives for it, secrets shown.
func parseFile(t *testing.T, rel, text string) []string {
	t.Helper()
	return parseWith(t, secrets.Setting{}, rel, text)
}

// parseWith writes text into a new home at rel and returns the text of
// every row the parser gives for it under the secrets setting.
func parseWith(t *testing.T, setting secrets.Setting, rel, text string) []string {
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
		Home:     &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: setting},
		Manifest: jsonvalue.NewNull()}
	var texts []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}
