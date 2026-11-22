package cline

import (
	"path/filepath"
	"slices"
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
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "cline",
		Home:     &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: secrets.NewRecorder(setting)},
		Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	if err := parsertest.Parse(t, New(), artifact, parser.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
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

// rowsHolding returns the texts of the rows whose text holds part.
func rowsHolding(rows []timeline.Row, part string) []string {
	var texts []string
	for _, text := range textsOf(rows) {
		if strings.Contains(text, part) {
			texts = append(texts, text)
		}
	}
	return texts
}

func TestAnUploadsAndASandboxStartsEndpointShowTheirCredentialsUnlessWithheld(t *testing.T) {
	const manifest = `{"session_id": "s1", "started_at": "2026-01-01T00:00:00Z", "metadata": ` +
		`{"enterprise.blobUpload": {"userDistinctId": "u1", "storage": {"adapterType": "s3", "bucket": "b", ` +
		`"endpoint": "https://ci:s3cret@s3.example", "region": "r1", "accountId": "a1"}}}}`
	const pending = `[{"sessionId": "s2", "account": "acct", "endpoint": "https://ci:s3cret@cloud.example/api"}]`
	for _, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := rowsHolding(parseSecrets(t, mode.setting, ".cline/data/sessions/s1/s1.json", manifest),
				"endpoint=")
			want := []string{"session synced: enterprise prompt upload to s3 bucket b object sessions/u1/s1/" +
				"messages.json endpoint=" + mode.scheme + "s3.example region=r1 account=a1 " +
				"(from s1.json metadata enterprise.blobUpload, started_at)"}
			checkSecretRows(t, mode, got, want, "s3cret")
			rows := parseSecrets(t, mode.setting, ".cline/data/cloud-pending-starts/1-ab.json", pending)
			got = rowsHolding(rows, "endpoint=")
			if len(got) == 0 || !strings.Contains(got[0], "sandbox start pending account=acct endpoint="+
				mode.scheme+"cloud.example/api") {
				t.Errorf("pending start rows %q", textsOf(rows))
			}
			checkSecretRows(t, mode, got, got, "s3cret")
		})
	}
}
