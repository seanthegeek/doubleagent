package nanobot

import (
	"slices"
	"testing"

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
		if got := parseWith(t, setting, rel, "{x"); !slices.Equal(got, want) {
			t.Errorf("under %+v got %q, want %q: only config.json's error is withheld", setting, got, want)
		}
	}
}
