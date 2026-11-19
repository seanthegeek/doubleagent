package claudedesktop

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

// secretMode is one way a run shows secrets: by default, or withheld as
// the default placeholder, another, or an empty one, with the text a
// secret is shown as under it and the text a URL whose credentials are
// ci:s3cret starts with.
type secretMode struct {
	name    string
	setting secrets.Setting
	shown   func(secret string) string
	scheme  string
}

// secretModes returns the default and the three ways of withholding.
func secretModes() []secretMode {
	return []secretMode{
		{"shown by default", secrets.Setting{}, func(secret string) string { return secret }, "https://ci:s3cret@"},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			func(string) string { return "REDACTED" }, "https://REDACTED@"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			func(string) string { return "[withheld]" }, "https://[withheld]@"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, func(string) string { return "" },
			"https://"},
	}
}

// parseSecrets writes text at rel under a new home and returns every row
// the parser gives for it in a run that shows secrets as setting says.
func parseSecrets(t *testing.T, setting secrets.Setting, rel, text string) []timeline.Row {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "claude-desktop",
		Home:     &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: setting},
		Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		t.Fatal(err)
	}
	return rows
}

// textsOf returns the text of each row.
func textsOf(rows []timeline.Row) []string {
	texts := make([]string, 0, len(rows))
	for _, row := range rows {
		texts = append(texts, row.Text)
	}
	return texts
}

// checkSecretRows fails the test when the rows are not want, or when a run
// that withholds secrets gave a row holding secret.
func checkSecretRows(t *testing.T, mode secretMode, got, want []string, secret string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if mode.setting.Withhold && strings.Contains(strings.Join(got, "\n"), secret) {
		t.Errorf("a withheld run shows %q: %q", secret, got)
	}
}

func TestAnAllowedFetchURLShowsItsCredentialsUnlessWithheld(t *testing.T) {
	const rel = ".config/Claude/local-agent-mode-sessions/a/o/local_c0c0c0c0-1111-4222-8333-444444444444.json"
	const record = `{"sessionId": "local_c0c0c0c0-1111-4222-8333-444444444444", "createdAt": 1790845200000, ` +
		`"webFetchAllowedUrls": ["https://ci:s3cret@docs.example/"]}`
	for _, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			var got []string
			for _, text := range textsOf(parseSecrets(t, mode.setting, rel, record)) {
				if strings.HasPrefix(text, "permission grant:") {
					got = append(got, text)
				}
			}
			want := []string{"permission grant: webFetchAllowedUrls " + mode.scheme + "docs.example/ (as of createdAt)"}
			checkSecretRows(t, mode, got, want, "s3cret")
		})
	}
}
