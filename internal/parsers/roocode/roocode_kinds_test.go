package roocode

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseFile writes text at rel under a new home and returns the texts of
// the rows the parser gives it.
func parseFile(t *testing.T, rel, text string) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
	var texts []string
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

// The same table drives the workspace-state.json file and, through
// retryQueueRows, the state of a state.vscdb; the databases of
// testdata/gen_state.py check the rows a state.vscdb gives.
func TestARetryQueueThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = ".vscode-mock/workspace-storage/h/workspace-state.json"
	const wrong = "parser: roo.retryQueue is not a JSON array"
	tests := []struct {
		name, value string
		wrong       bool
	}{
		{"a string", `"oops"`, true},
		{"a number", `7`, true},
		{"an object", `{"id": "a"}`, true},
		{"null", `null`, false},
		{"an empty array", `[]`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var want []string
			if test.wrong {
				want = []string{wrong}
			}
			if got := parseFile(t, rel, `{"roo.retryQueue": `+test.value+`}`); !slices.Equal(got, want) {
				t.Errorf("file rows %q, want %q", got, want)
			}
			state, err := jsonvalue.DecodeString(`{"roo.retryQueue": ` + test.value + `}`)
			if err != nil {
				t.Fatal(err)
			}
			problem := New().retryQueueRows(&input.Artifact{Rel: rel}, state, rel, func(timeline.Row) {})
			if (problem != "") != test.wrong || (test.wrong && problem != "roo.retryQueue is not a JSON array") {
				t.Errorf("retryQueueRows problem %q, want wrong=%v", problem, test.wrong)
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		if got := parseFile(t, rel, `{}`); len(got) != 0 {
			t.Errorf("rows %q, want none", got)
		}
	})
}

func TestEntriesOfTheWrongKindAreReportedOncePerList(t *testing.T) {
	tests := []struct {
		name, rel, text, want string
	}{
		{"retry queue requests", ".vscode-mock/workspace-storage/h/workspace-state.json",
			`{"roo.retryQueue": [7]}`, "parser: roo.retryQueue holds 1 entry that is not a JSON object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseFile(t, test.rel, test.text); !slices.Contains(got, test.want) {
				t.Errorf("rows %q lack %q", got, test.want)
			}
		})
	}
}

func TestARetryQueueProblemCarriesTheFilesTime(t *testing.T) {
	const jsonRel = ".vscode-mock/workspace-storage/h/workspace-state.json"
	const databaseRel = ".config/Code/User/workspaceStorage/abc/state.vscdb"
	fixture := func(name string) []byte {
		data, err := osfile.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	const notArray = "parser: roo.retryQueue is not a JSON array"
	const notObjects = "parser: roo.retryQueue holds 1 entry that is not a JSON object"
	tests := []struct {
		name, rel string
		data      []byte
		want      string
	}{
		{"a workspace-state.json retry queue that is a number", jsonRel, []byte(`{"roo.retryQueue": 5}`), notArray},
		{"a state.vscdb retry queue that is a number", databaseRel, fixture("retry-queue-number.vscdb"), notArray},
		{"a workspace-state.json retry queue holding a number", jsonRel, []byte(`{"roo.retryQueue": [7]}`), notObjects},
		{"a state.vscdb retry queue holding a number", databaseRel, fixture("retry-queue-entries.vscdb"), notObjects},
	}
	moment := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, filepath.FromSlash(test.rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(path, test.data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := osfile.Chtimes(path, moment, moment); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: path, Original: "/h/" + test.rel, Rel: test.rel,
				Home: &input.Home{DiskPath: home}}
			var got [][2]string
			if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
				got = append(got, [2]string{row.TimestampUTC, row.Text})
			}); err != nil {
				t.Fatal(err)
			}
			if want := [][2]string{{"2024-01-02T03:04:05.000Z", test.want}}; !slices.Equal(got, want) {
				t.Errorf("rows %q, want %q", got, want)
			}
		})
	}
}
