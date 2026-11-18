package crush

import (
	"slices"
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
