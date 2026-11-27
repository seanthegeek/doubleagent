package claudecode

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
	return parseSecretsBy(t, parsertest.Parse, setting, rel, text)
}

// parseSecretsBy is parseSecrets parsing through parse, a function of
// package parsertest.
func parseSecretsBy(t *testing.T, parse func(testing.TB, parserkit.Parser, *input.Artifact, parserkit.Options,
	parserkit.Emit) error, setting secrets.Setting, rel, text string) []timeline.Row {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "claude-code",
		Home:     &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: secrets.NewRecorder(setting)},
		Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	if err := parse(t, New(), artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
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

// rowsWithPrefix returns the texts of the rows that start with one of the
// prefixes.
func rowsWithPrefix(rows []timeline.Row, prefixes ...string) []string {
	var texts []string
	for _, text := range textsOf(rows) {
		if slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(text, prefix) }) {
			texts = append(texts, text)
		}
	}
	return texts
}

func TestAPullRequestsAndABridgesURLShowTheirCredentialsUnlessWithheld(t *testing.T) {
	const rel = ".claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl"
	const transcript = `{"type":"pr-link","sessionId":"s1","prUrl":"https://ci:s3cret@github.com/o/r/pull/7"}` + "\n" +
		`{"type":"system","subtype":"bridge_status","url":"https://claude.example/code?token=s3cret"}` + "\n" +
		`{"type":"bridge-session","sessionId":"s1","bridgeSessionId":"b1","lastSequenceNum":1}` + "\n"
	tokens := []string{"s3cret", "REDACTED", "[withheld]", ""}
	for i, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := rowsWithPrefix(parseSecrets(t, mode.setting, rel, transcript), "pr-link:", "session synced:")
			want := []string{"pr-link: " + mode.scheme + "github.com/o/r/pull/7",
				"session synced: remote control bridge b1 url https://claude.example/code?token=" + tokens[i]}
			checkSecretRows(t, mode, got, want, "s3cret")
		})
	}
}

func TestTheMachineAndGitHubAccountIdsAreShownWhateverTheSetting(t *testing.T) {
	const config = `{"githubWebConnectionStatusCache": {"accountUuid": "a1b2", "orgUuid": "c3d4", ` +
		`"status": "connected"}, "remoteControlMachineId": "0f0e0d0c-0000-4000-8000-000000000001"}`
	want := []string{"source host: github status=connected account=a1b2 org=c3d4 (from .claude.json, file mtime)",
		"remote control: machine id 0f0e0d0c-0000-4000-8000-000000000001 (from .claude.json, file mtime)"}
	for _, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := textsOf(parseSecretsBy(t, parsertest.ParseWithoutSecrets, mode.setting, ".claude.json", config))
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}
