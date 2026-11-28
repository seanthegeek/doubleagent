package pearai

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parserkit/parsertest"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// stateAndTask writes a state.vscdb whose PearAI value is not JSON and
// quotes a key, and one task's ui_messages.json, under one home that
// withholds secrets, and returns the database's and the task file's
// artifacts.
func stateAndTask(t *testing.T) (*input.Artifact, *input.Artifact) {
	t.Helper()
	data, err := sqlitetest.Build(sqlitetest.Table{Name: "ItemTable",
		SQL:  "CREATE TABLE ItemTable (key TEXT, value BLOB)",
		Rows: [][]any{{extensionID, `{"taskHistory":[], "apiKey":"sk-abcdefgh" Zq9secretish}`}}})
	if err != nil {
		t.Fatal(err)
	}
	home := &input.Home{DiskPath: t.TempDir(),
		Secrets: secrets.NewRecorder(secrets.Setting{Withhold: true, Placeholder: secrets.DefaultPlaceholder})}
	artifactAt := func(rel string, content []byte) *input.Artifact {
		path := filepath.Join(home.DiskPath, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		return &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: home}
	}
	return artifactAt("User/globalStorage/state.vscdb", data),
		artifactAt("User/globalStorage/"+extensionID+"/tasks/t1/ui_messages.json", []byte(`[]`))
}

func TestAStateValueThatIsNotJSONIsWithheldWhicheverFileIsParsedFirst(t *testing.T) {
	for _, taskFirst := range []bool{true, false} {
		name := "the database first"
		if taskFirst {
			name = "a task file first"
		}
		t.Run(name, func(t *testing.T) {
			state, task := stateAndTask(t)
			subject := New()
			parseTask := func() {
				_, _ = parserkit.Within(task, func(emit parserkit.Emit) error {
					return subject.Parse(task, parserkit.Options{}, emit)
				}, func(timeline.Row) {})
			}
			if taskFirst {
				parseTask()
			}
			var got []string
			_ = parsertest.Parse(t, subject, state, parserkit.Options{},
				func(row timeline.Row) { got = append(got, row.Text) })
			if !taskFirst {
				parseTask()
			}
			want := "parser: ItemTable value for " + extensionID +
				" is not valid JSON (invalid JSON at line 1, column 43: REDACTED)"
			if len(got) != 1 || got[0] != want {
				t.Errorf("rows %q, want [%q]", got, want)
			}
			if strings.Contains(strings.Join(got, "\n"), "Zq9") {
				t.Errorf("a withheld run shows the value: %q", got)
			}
		})
	}
}
