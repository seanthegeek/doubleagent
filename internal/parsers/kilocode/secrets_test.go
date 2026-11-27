package kilocode

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parserkit/parsertest"
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
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "kilo-code",
		Home:     &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: secrets.NewRecorder(setting)},
		Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	if err := parsertest.Parse(t, New(), artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
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

func TestASessionsShareLinkShowsItsCredentialsUnlessWithheld(t *testing.T) {
	const rel = ".local/share/kilo/storage/session_share/ses_1.json"
	const share = `{"id": "k1", "ingestPath": "/ingest/k1", "url": "https://kilo.example/s/k1?token=s3cret"}`
	tokens := []string{"s3cret", "REDACTED", "[withheld]", ""}
	for i, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := textsOf(parseSecrets(t, mode.setting, rel, share))
			want := []string{"session synced: kilo session k1 ingest /ingest/k1 share https://kilo.example/s/k1?token=" +
				tokens[i] + " (from storage/session_share/ses_1.json, file mtime)"}
			checkSecretRows(t, mode, got, want, "s3cret")
		})
	}
}

func TestAShareMarkerIsNamedAsThePathSpellsIt(t *testing.T) {
	got := textsOf(parseSecrets(t, secrets.Setting{}, ".local/share/kilo/Storage/Session_Share/ses_1.JSON", `{"id": "k1"}`))
	if len(got) != 1 || !strings.HasSuffix(got[0], "(from Storage/Session_Share/ses_1.JSON, file mtime)") {
		t.Errorf("got %q", got)
	}
}

func TestARemoteControlRowNamesItsLogAsThePathSpellsIt(t *testing.T) {
	got := textsOf(parseSecrets(t, secrets.Setting{}, ".local/share/kilo/LOG/A.Log",
		"INFO 2026-10-01T08:00:00 +5ms remote connection disabled\n"))
	if len(got) != 1 || !strings.HasSuffix(got[0], "(from LOG/A.Log, line time)") {
		t.Errorf("got %q", got)
	}
}
