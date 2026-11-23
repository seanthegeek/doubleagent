package copilotdesktop

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// secretMode is one way a run shows secrets: by default, or withheld as
// the default placeholder, another, or an empty one, with the text a
// stored token is shown as under it.
type secretMode struct {
	name    string
	setting secrets.Setting
	token   func(value string) string
}

// secretModes returns the default and the three ways of withholding.
func secretModes() []secretMode {
	return []secretMode{
		{"shown by default", secrets.Setting{}, func(value string) string { return value }},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			func(string) string { return "REDACTED" }},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			func(string) string { return "[withheld]" }},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, func(string) string { return "" }},
	}
}

func TestAStoredTokenIsShownUnlessWithheldAndAMissingOneIsSaidToBeAbsent(t *testing.T) {
	for _, mode := range secretModes() {
		t.Run(mode.name, func(t *testing.T) {
			got := rowsOfDatabase(t, "secrets.db", mode.setting)
			want := []string{
				"account: ana host=github.com kind=github default=1 token=" + mode.token("FIXTURE-TOKEN"),
				"account: bo host=github.com kind=github default=0 token_present=false",
				"desktop settings: storage_location=/w/store instructions=- github_token=" +
					mode.token("FIXTURE-GITHUB-TOKEN") + " github_token_type=oauth (from .copilot/data.db, file mtime)",
			}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			if mode.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "FIXTURE") {
				t.Errorf("a withheld run shows a token: %q", got)
			}
		})
	}
}
