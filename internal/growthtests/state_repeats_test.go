package growthtests

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parserkit/parsertest"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/vscodestate"
)

// stateItem returns a state.vscdb whose ItemTable holds value under key.
func stateItem(t *testing.T, key, value string) []byte {
	t.Helper()
	return database(t, sqlitetest.Table{Name: "ItemTable", SQL: "CREATE TABLE ItemTable (key TEXT, value BLOB)",
		Rows: [][]any{{key, value}}})
}

func TestAStateDatabaseItemThatRepeatsANameIsReportedOnTheDatabase(t *testing.T) {
	tests := []struct {
		parser, agent, rel, key string
	}{
		{"cody", codyEditorAgent, codyState, "sourcegraph.cody-ai"},
		{"twinny", twinnyEditor, twinnyState, "rjmacarthy.twinny"},
		{"tabby", "vscode", ".config/Code/User/globalStorage/state.vscdb", "TabbyML.vscode-tabby"},
		{"pearai", "", pearaiState, "pearai.pearai-roo-cline"},
	}
	for _, test := range tests {
		t.Run(test.parser, func(t *testing.T) {
			files := one(test.rel, stateItem(t, test.key, `{"x":1,"y":{},"x":2}`))
			got := repeatRowsByFileOf(t, test.parser, test.agent, files)
			if want := map[string]int{"/home/u/" + test.rel: 1}; !maps.Equal(got, want) {
				t.Errorf("repeat rows by file %v, want %v", got, want)
			}
		})
	}
}

// stateValueProblems parses a state.vscdb at rel with the registry's
// parser called name, in a run that shows secrets as setting says, and
// returns the text of its rows about a value that is not JSON, as written.
func stateValueProblems(t *testing.T, name, agent, rel string, data []byte, setting secrets.Setting) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: agent,
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h",
			Secrets: secrets.NewRecorder(setting)}, Manifest: jsonvalue.NewNull()}
	var texts []string
	for _, each := range parsers.All() {
		if each.Name() != name {
			continue
		}
		if !each.Wants(artifact) {
			t.Fatalf("%s does not read %s", name, rel)
		}
		if err := parsertest.Parse(t, each, artifact, parserkit.Options{}, func(row timeline.Row) {
			if strings.Contains(row.Text, vscodestate.ValueProblem) {
				texts = append(texts, row.Text)
			}
		}); err != nil {
			t.Fatal(err)
		}
	}
	return texts
}

// A state value that is not JSON gives one row with the JSON reader's
// reason, whose detail can quote the value and so is withheld with the
// secrets, the position kept.
func TestAStateValueThatIsNotJSONShowsTheReadersReasonUnlessWithheld(t *testing.T) {
	const state = ".config/Code/User/globalStorage/state.vscdb"
	tests := []struct {
		parser, agent, rel, key string
	}{
		{"vscode", "vscode", state, "GitHub.copilot-chat"},
		{"vscode", "vscode", state, "github-octocat-usages"},
		{"cody", codyEditorAgent, codyState, "sourcegraph.cody-ai"},
		{"twinny", twinnyEditor, twinnyState, "rjmacarthy.twinny"},
		{"tabby", "vscode", state, "TabbyML.vscode-tabby"},
		{"roo-code", "vscode", state, "RooVeterinaryInc.roo-cline"},
	}
	modes := []struct {
		name    string
		setting secrets.Setting
		detail  string
	}{
		{"shown by default", secrets.Setting{}, ": invalid character 's' looking for beginning of value)"},
		{"withheld as REDACTED", secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder}, ": REDACTED)"},
		{"withheld as an empty placeholder", secrets.Setting{Withhold: true}, ")"},
	}
	for _, test := range tests {
		data := stateItem(t, test.key, `{"apiKey": sk-SECRET}`)
		for _, mode := range modes {
			t.Run(test.parser+" "+test.key+", "+mode.name, func(t *testing.T) {
				got := stateValueProblems(t, test.parser, test.agent, test.rel, data, mode.setting)
				want := []string{parserkit.ProblemPrefix + vscodestate.ValueProblem + test.key +
					" is not valid JSON (invalid JSON at line 1, column 12" + mode.detail}
				if !slices.Equal(got, want) {
					t.Errorf("got %q, want %q", got, want)
				}
			})
		}
	}
}
