package crush

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseProjectsFile writes text as a projects.json under a new home and
// returns the texts of the rows the parser gives it.
func parseProjectsFile(t *testing.T, text string) []string {
	t.Helper()
	const rel = ".local/share/crush/projects.json"
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
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestAProjectsMemberThatIsNotAnArrayIsReported(t *testing.T) {
	tests := []struct {
		name, document string
		want           []string
	}{
		{"a string", `{"projects": "oops"}`, []string{"parser: projects is not a JSON array"}},
		{"a number", `{"projects": 7}`, []string{"parser: projects is not a JSON array"}},
		{"an object", `{"projects": {"path": "/p"}}`, []string{"parser: projects is not a JSON array"}},
		{"a document that is a list", `[1]`, []string{"parser: projects.json is not a JSON object"}},
		{"null", `{"projects": null}`, nil},
		{"an empty array", `{"projects": []}`, nil},
		{"absent", `{}`, nil},
		{"a project", `{"projects": [{"path": "/p"}]}`, []string{"crush project last accessed"}},
		{"entries that are not objects", `{"projects": [{"path": "/p"}, "x", 7]}`,
			[]string{"parser: projects holds 2 entries that are not JSON objects", "crush project last accessed"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseProjectsFile(t, test.document); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}
