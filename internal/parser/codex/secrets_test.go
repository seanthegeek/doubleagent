package codex

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

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
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Agent: "codex-cli",
		Home: &input.Home{DiskPath: home, Secrets: setting}, Manifest: jsonvalue.NewNull()}
	var texts []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestATOMLFileThatDoesNotParseGivesTheReadersDetailUnlessWithheld(t *testing.T) {
	const document = "[mcp_servers.x.env]\nAPI_KEY = sk-SECRET\n"
	settings := []struct {
		name    string
		setting secrets.Setting
		detail  string
	}{
		{"shown by default", secrets.Setting{}, `: expected value but found "sk" instead`},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder}, ": REDACTED"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"}, ": [withheld]"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, ""},
	}
	files := []struct{ rel, label string }{
		{".codex/config.toml", "config.toml"},
		{".codex/automations/a/automation.toml", "automation.toml"},
	}
	for _, file := range files {
		for _, each := range settings {
			t.Run(file.label+" "+each.name, func(t *testing.T) {
				want := []string{"parser: unreadable " + file.label + ": invalid TOML at line 2, column 11" + each.detail}
				if got := parseWith(t, each.setting, file.rel, document); !slices.Equal(got, want) {
					t.Errorf("got %q, want %q", got, want)
				}
			})
		}
	}
}

// secretSchemes are the texts a URL whose credentials are ci:s3cret starts
// with when shown, and when withheld as REDACTED, as another placeholder
// and as an empty one.
func secretSchemes() []struct {
	name    string
	setting secrets.Setting
	scheme  string
} {
	return []struct {
		name    string
		setting secrets.Setting
		scheme  string
	}{
		{"shown by default", secrets.Setting{}, "https://ci:s3cret@"},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			"https://REDACTED@"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			"https://[withheld]@"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, "https://"},
	}
}

// remoteRows returns the texts that are git remote: rows.
func remoteRows(texts []string) []string {
	var remotes []string
	for _, text := range texts {
		if strings.HasPrefix(text, timeline.GitRemotePrefix) {
			remotes = append(remotes, text)
		}
	}
	return remotes
}

func TestARemoteURLShowsItsCredentialsUnlessWithheld(t *testing.T) {
	const rollout = ".codex/sessions/2026/01/02/rollout-2026-01-02T00-00-00-0198f0a1-0000-7000-8000-000000000001.jsonl"
	const meta = `{"timestamp": "2026-01-02T00:00:00Z", "type": "session_meta", "payload": {"id": "s1", ` +
		`"cwd": "/w", "git": {"repository_url": "https://ci:s3cret@github.com/o/r.git"}}}` + "\n"
	database, err := osfile.ReadFile(filepath.Join("testdata", "secrets.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range secretSchemes() {
		t.Run(mode.name, func(t *testing.T) {
			want := []string{timeline.GitRemotePrefix + mode.scheme + "github.com/o/r.git"}
			for _, file := range []struct{ rel, text string }{{rollout, meta},
				{".codex/state_5.sqlite", string(database)}} {
				got := parseWith(t, mode.setting, file.rel, file.text)
				if remotes := remoteRows(got); !slices.Equal(remotes, want) {
					t.Errorf("%s: got %q, want %q", file.rel, remotes, want)
				}
				if mode.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "s3cret") {
					t.Errorf("%s: a withheld run shows the password: %q", file.rel, got)
				}
			}
		})
	}
}
