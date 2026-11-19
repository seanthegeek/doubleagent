package opencode

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/secrets"
)

func TestAShareURLAndSecretAreShownUnlessWithheld(t *testing.T) {
	tests := []struct {
		name    string
		setting secrets.Setting
		shown   func(secret string) string
	}{
		{"shown by default", secrets.Setting{}, func(secret string) string { return secret }},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			func(string) string { return "REDACTED" }},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			func(string) string { return "[withheld]" }},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, func(string) string { return "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, text := range parseDatabaseWith(t, "shared_session.db", test.setting) {
				if strings.HasPrefix(text, "session synced: share") {
					got = append(got, text)
				}
			}
			user := test.shown("ci:s3cret") + "@"
			if user == "@" {
				user = ""
			}
			want := []string{
				"session synced: share shr_1 url https://opncd.example/s/shr_1?token=" + test.shown("s3cret") +
					" secret=" + test.shown("FIXTURE-SHARE-SECRET") + " (from opencode.db session_share, time_created)",
				"session synced: share url https://" + user + "opncd.example/s/shr_2 " +
					"(from opencode.db event session.updated.1, info.time.updated)",
			}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			if test.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "s3cret") ||
				test.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "FIXTURE") {
				t.Errorf("a withheld run shows a secret: %q", got)
			}
		})
	}
}
