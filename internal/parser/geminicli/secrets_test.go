package geminicli

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestAPolicyThatDoesNotParseGivesTheReadersDetailUnlessWithheld(t *testing.T) {
	const rel = ".gemini/policies/sandbox.toml"
	const position = "parser: unreadable .gemini/policies/sandbox.toml: invalid TOML at line 2, column 9"
	tests := []struct {
		name    string
		setting secrets.Setting
		want    string
	}{
		{"shown by default", secrets.Setting{}, position + `: expected value but found "sk" instead`},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			position + ": REDACTED"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			position + ": [withheld]"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, position},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, filepath.FromSlash(rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(path, []byte("[env]\nTOKEN = sk-SECRET\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Agent: "gemini-cli",
				Home: &input.Home{DiskPath: home, Secrets: test.setting}, Manifest: jsonvalue.NewNull()}
			var got []string
			err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { got = append(got, row.Text) })
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, []string{test.want}) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
