package openclaw

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
// secret is shown as under it.
type secretMode struct {
	name    string
	setting secrets.Setting
	shown   func(secret string) string
}

// secretModes returns the default and the three ways of withholding.
func secretModes() []secretMode {
	return []secretMode{
		{"shown by default", secrets.Setting{}, func(secret string) string { return secret }},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			func(string) string { return "REDACTED" }},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			func(string) string { return "[withheld]" }},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, func(string) string { return "" }},
	}
}

// parseStateDatabase copies the named testdata database into a new home as
// state/openclaw.sqlite and returns the text of every row the parser gives
// for it in a run that shows secrets as setting says.
func parseStateDatabase(t *testing.T, name string, setting secrets.Setting) []string {
	t.Helper()
	const stateRel = ".openclaw/state/openclaw.sqlite"
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(stateRel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + stateRel, Rel: stateRel,
		Agent: "openclaw", Manifest: jsonvalue.NewNull(),
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: setting}}
	var got []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { got = append(got, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestAnExecApprovalsConfigThatIsNotJSONGivesTheReadersDetailUnlessWithheld(t *testing.T) {
	const position = "parser: exec_approvals_config raw_json unparseable (invalid JSON at line 1, column 22"
	details := []string{": invalid character 's' looking for beginning of value)", ": REDACTED)", ": [withheld])", ")"}
	for i, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseStateDatabase(t, "bad_exec_config.db", mode.setting)
			if want := []string{position + details[i]}; !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestTheApprovalsSocketTokenIsShownUnlessWithheld(t *testing.T) {
	const source = " (from state/openclaw.sqlite exec_approvals_config, updated_at_ms)"
	for _, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseStateDatabase(t, "exec_config_socket.db", mode.setting)
			token := " socket.token=" + mode.shown("FIXTURE-SOCKET-TOKEN")
			want := []string{"permission setting: defaults.security=deny" + token + source,
				"permission setting:" + token + source}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			if mode.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "FIXTURE") {
				t.Errorf("a withheld run shows the token: %q", got)
			}
		})
	}
}

func TestTheURLsOfTheStateDatabaseShowTheirCredentialsUnlessWithheld(t *testing.T) {
	schemes := []string{"https://ci:s3cret@", "https://REDACTED@", "https://[withheld]@", "https://"}
	for i, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := parseStateDatabase(t, "state_urls.db", mode.setting)
			want := []string{
				"backup uploaded: git to - key " + schemes[i] + "git.example/o/backup.git namespace - " +
					"(from state/openclaw.sqlite backup_runs, created_at)",
				"source host: github account ana id 7 identity app; publication opened repository o/r branch fix " +
					"pull request " + schemes[i] + "github.example/o/r/pull/7 " +
					"(from state/openclaw.sqlite github_publication_requests, created_at_ms)",
				"git remote: " + schemes[i] + "git.example/o/r.git",
			}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			if mode.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "s3cret") {
				t.Errorf("a withheld run shows the password: %q", got)
			}
		})
	}
}
