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

func TestASideFileThatDoesNotHoldAnObjectIsReported(t *testing.T) {
	const notIndex = "parser: _index.json is not a JSON object"
	const notItem = "parser: history_item.json is not a JSON object"
	tests := []struct {
		name, item, index string
		want              []string
	}{
		{"an index that is not JSON", "", `{"entries": [`, []string{"hi", notIndex}},
		{"an index that is an array", "", `[]`, []string{"hi", notIndex}},
		{"an index that is an object", "", `{}`, []string{"hi"}},
		{"an item that is not JSON", `{"task":`, `{}`, []string{"hi", notItem}},
		{"an item that is an array", `[1]`, `{}`, []string{"hi", notItem}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			write(t, filepath.Join(home, ".config/Code/User/globalStorage/kilocode.kilo-code/tasks/_index.json"),
				test.index)
			artifact := taskArtifact(home, "t")
			write(t, artifact.DiskPath, `[{"role": "user", "content": "hi"}]`)
			if test.item != "" {
				write(t, filepath.Join(filepath.Dir(artifact.DiskPath), "history_item.json"), test.item)
			}
			var got []string
			New().task(artifact, parser.Options{}, func(row timeline.Row) { got = append(got, row.Text) })
			if !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestTaskEntriesThatAreNotObjectsAreReportedOncePerList(t *testing.T) {
	t.Run("index entries", func(t *testing.T) {
		got := taskRows(t, `{"entries": [{"id": "u"}, 7]}`)
		if want := []string{"hi", "parser: entries holds 1 entry that is not a JSON object"}; !slices.Equal(got, want) {
			t.Errorf("rows %q, want %q", got, want)
		}
	})
	t.Run("messages and their content", func(t *testing.T) {
		home := t.TempDir()
		artifact := taskArtifact(home, "t")
		write(t, artifact.DiskPath, `[{"role": "user", "content": [{"type": "text", "text": "hi"}, 7]}, "x"]`)
		var texts []string
		New().task(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) })
		want := []string{"hi", "parser: content holds 1 entry that is not a JSON object",
			"parser: api_conversation_history.json holds 1 entry that is not a JSON object"}
		if !slices.Equal(texts, want) {
			t.Errorf("rows %q, want %q", texts, want)
		}
	})
}
