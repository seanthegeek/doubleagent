package ollama

import (
	"path/filepath"
	"slices"
	"testing"

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
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Agent: "ollama",
		Home: &input.Home{DiskPath: home}, Manifest: jsonvalue.NewNull()}
	var texts []string
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

func TestAnIntegrationsMemberThatIsNotAnObjectIsReported(t *testing.T) {
	const rel = ".ollama/config.json"
	problem := parserkit.ProblemPrefix + parserkit.WrongKind("integrations", jsonvalue.Object)
	tests := []struct {
		name, config string
		want         int
	}{
		{"a string", `{"integrations": "oops"}`, 1},
		{"a number", `{"integrations": 7}`, 1},
		{"an array", `{"integrations": [{"automode": true}]}`, 1},
		{"null", `{"integrations": null}`, 0},
		{"an empty object", `{"integrations": {}}`, 0},
		{"absent", `{}`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := 0
			for _, text := range parseFile(t, rel, test.config) {
				if text == problem {
					got++
				}
			}
			if got != test.want {
				t.Errorf("%d rows %q, want %d", got, problem, test.want)
			}
		})
	}
}

func TestIntegrationsThatAreNotObjectsAreReportedOnce(t *testing.T) {
	got := parseFile(t, ".ollama/config.json", `{"integrations": {"a": {"automode": true}, "b": 7, "c": "x"}}`)
	want := parserkit.ProblemPrefix + parserkit.WrongEntries("integrations", jsonvalue.Object, 2)
	if !slices.Contains(got, want) {
		t.Errorf("rows %q lack %q", got, want)
	}
}
