package nanobot

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser/parsertest"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

func TestAConfigThatIsNotJSONGivesTheReadersDetailUnlessWithheld(t *testing.T) {
	const position = "parser: config.json did not parse: invalid JSON at line 1, column 37"
	tests := []struct {
		name    string
		setting secrets.Setting
		want    string
	}{
		{"shown by default", secrets.Setting{}, position + ": invalid character 's' looking for beginning of value"},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			position + ": REDACTED"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			position + ": [withheld]"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, position},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseWith(t, test.setting, ".nanobot/config.json", `{"providers": {"openai": {"apiKey": sk-SECRET}}}`)
			if !slices.Equal(got, []string{test.want}) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestAnotherFileThatIsNotJSONGivesTheReadersDetailWhateverTheSetting(t *testing.T) {
	const rel = ".nanobot/sessions/0123456789abcdef0123456789abcdef/.webui_session_index.json"
	want := []string{"parser: .webui_session_index.json did not parse: invalid JSON at line 1, column 2: " +
		"invalid character 'x' looking for beginning of value"}
	withheld := secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder}
	for _, setting := range []secrets.Setting{{}, withheld} {
		if got := parseBy(t, parsertest.ParseWithoutSecrets, setting, rel, "{x"); !slices.Equal(got, want) {
			t.Errorf("under %+v got %q, want %q: only config.json's error is withheld", setting, got, want)
		}
	}
}

func TestTheConfigsCredentialsAreShownUnlessWithheld(t *testing.T) {
	const config = `{"providers": {"openai": {"apiKey": "sk-FIXTURE"}, "local": {"apiBase": "http://h"}}, ` +
		`"tools": {"restrictToWorkspace": true, "mcpServers": {"github": {"env": ` +
		`{"GITHUB_PERSONAL_ACCESS_TOKEN": "FIXTURE-PAT"}}}}, ` +
		`"channels": {"feishu": {"appSecret": "FIXTURE-APP", "verification_token": "FIXTURE-VERIFY"}, ` +
		`"linear": {"clientSecret": "FIXTURE-CLIENT"}}}`
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
			got := parseWith(t, test.setting, ".nanobot/config.json", config)
			want := []string{
				"permission setting: tools.restrictToWorkspace=true (from .nanobot/config.json, file mtime)",
				"stored credentials: providers.openai.apiKey=" + test.shown("sk-FIXTURE") +
					" tools.mcpServers.github.env.GITHUB_PERSONAL_ACCESS_TOKEN=" + test.shown("FIXTURE-PAT") +
					" channels.feishu.appSecret=" + test.shown("FIXTURE-APP") +
					" channels.feishu.verification_token=" + test.shown("FIXTURE-VERIFY") +
					" channels.linear.clientSecret=" + test.shown("FIXTURE-CLIENT") +
					" (from .nanobot/config.json, file mtime)",
			}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			if test.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "FIXTURE") {
				t.Errorf("a withheld run shows a secret: %q", got)
			}
		})
	}
}
