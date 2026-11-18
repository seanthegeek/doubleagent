package kilocode

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// taskRows writes index as the text of the tasks' _index.json, parses a
// task t that has no history_item.json and returns the texts of its rows.
func taskRows(t *testing.T, index string) []string {
	t.Helper()
	home := t.TempDir()
	write(t, filepath.Join(home, ".config/Code/User/globalStorage/kilocode.kilo-code/tasks/_index.json"), index)
	artifact := taskArtifact(home, "t")
	write(t, artifact.DiskPath, `[{"role": "user", "content": "hi"}]`)
	var texts []string
	New().task(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) })
	return texts
}

func TestAnIndexEntriesMemberThatIsNotAnArrayIsReported(t *testing.T) {
	const wrong = "parser: entries is not a JSON array"
	tests := []struct {
		name, index string
		want        []string
	}{
		{"a string", `{"entries": "x"}`, []string{"hi", wrong}},
		{"a number", `{"entries": 7}`, []string{"hi", wrong}},
		{"an object", `{"entries": {"id": "t"}}`, []string{"hi", wrong}},
		{"null", `{"entries": null}`, []string{"hi"}},
		{"an empty array", `{"entries": []}`, []string{"hi"}},
		{"absent", `{}`, []string{"hi"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := taskRows(t, test.index); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}
