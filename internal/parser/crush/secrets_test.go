package crush

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/secrets"
)

func TestACrushJSONThatIsNotJSONGivesTheReadersDetailUnlessWithheld(t *testing.T) {
	const position = "parser: .config/crush/crush.json is not a JSON object (invalid JSON at line 1, column 38"
	tests := []struct {
		name    string
		setting secrets.Setting
		want    string
	}{
		{"shown by default", secrets.Setting{},
			position + ": invalid character 's' looking for beginning of value)"},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			position + ": REDACTED)"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			position + ": [withheld])"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, position + ")"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseWith(t, test.setting, ".config/crush/crush.json",
				[]byte(`{"providers": {"openai": {"api_key": sk-SECRET}}}`))
			if !slices.Equal(got, []string{test.want}) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestAProvidersAPIKeyIsShownUnlessWithheld(t *testing.T) {
	const config = `{"permissions": {"allowed_tools": ["view"]}, "providers": {"openai": {"api_key": "sk-FIXTURE"}, ` +
		`"local": {"base_url": "http://localhost:11434"}}}`
	tests := []struct {
		name    string
		setting secrets.Setting
		key     string
	}{
		{"shown by default", secrets.Setting{}, "sk-FIXTURE"},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder}, "REDACTED"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"}, "[withheld]"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseWith(t, test.setting, ".config/crush/crush.json", []byte(config))
			want := []string{
				"permission setting: permissions.allowed_tools=view (from .config/crush/crush.json, file mtime)",
				"stored credentials: providers.openai.api_key=" + test.key + " (from .config/crush/crush.json, file mtime)",
			}
			if !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
			if test.setting.Withhold && strings.Contains(strings.Join(got, "\n"), "FIXTURE") {
				t.Errorf("a withheld run shows the key: %q", got)
			}
		})
	}
}
