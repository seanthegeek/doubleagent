package crush

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
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

func TestACrushJSONMemberOfTheWrongKindIsReported(t *testing.T) {
	const rel = ".config/crush/crush.json"
	tests := []struct {
		name, config string
		want         []string
	}{
		{"lists that are not arrays", `{"permissions": {"allowed_tools": "bash"}, "options": {"disabled_tools": {"x": 1}}, ` +
			`"mcp": {"s": {"enabled_tools": 5}}, "hooks": {"PreToolUse": {"command": "true"}}}`,
			[]string{"parser: permissions.allowed_tools is not a JSON array",
				"parser: options.disabled_tools is not a JSON array",
				"parser: mcp.s.enabled_tools is not a JSON array",
				"parser: hooks.PreToolUse is not a JSON array"}},
		{"blocks that are not objects", `{"permissions": "all", "options": [1], "mcp": [1], "hooks": [1]}`,
			[]string{"parser: permissions is not a JSON object", "parser: options is not a JSON object",
				"parser: mcp is not a JSON object", "parser: hooks is not a JSON object"}},
		{"servers that are not objects", `{"mcp": {"s": "x", "t": 1, "u": {}}}`,
			[]string{"parser: mcp holds 2 entries that are not JSON objects"}},
		{"providers that are not an object", `{"providers": ["sk-1"]}`,
			[]string{"parser: providers is not a JSON object"}},
		{"a provider that is a string", `{"providers": {"openai": "sk-2"}}`,
			[]string{"parser: providers holds 1 entry that is not a JSON object"}},
		{"members null or empty", `{"permissions": {"allowed_tools": null}, "mcp": {}, "hooks": null, "providers": {}}`,
			nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, text := range parseFile(t, rel, []byte(test.config)) {
				if strings.HasPrefix(text, parserkit.ProblemPrefix) {
					got = append(got, text)
				}
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("problems %q, want %q", got, test.want)
			}
		})
	}
}
