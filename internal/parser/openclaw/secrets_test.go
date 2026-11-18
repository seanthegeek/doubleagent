package openclaw

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

func TestAnExecApprovalsConfigThatIsNotJSONGivesTheReadersDetailUnlessWithheld(t *testing.T) {
	const stateRel = ".openclaw/state/openclaw.sqlite"
	const position = "parser: exec_approvals_config raw_json unparseable (invalid JSON at line 1, column 22"
	data, err := osfile.ReadFile(filepath.Join("testdata", "bad_exec_config.db"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		setting secrets.Setting
		want    string
	}{
		{"shown by default", secrets.Setting{}, position + ": invalid character 's' looking for beginning of value)"},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder},
			position + ": REDACTED)"},
		{"withheld as another placeholder", secrets.Setting{Withhold: true, Placeholder: "[withheld]"},
			position + ": [withheld])"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, position + ")"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
				Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h", Secrets: test.setting}}
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
