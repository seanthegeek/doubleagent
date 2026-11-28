package clinelegacy

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

func TestATaskDirectoryFindsItsIndexEntryInAnyCase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "taskHistory.json")
	text := `[{"id": "ab1", "n": 1}, {"id": "Xy", "n": 2}, {"id": "xY", "n": 3}, {"id": "XY", "n": 4},
		{"id": "q", "n": 5}, {"id": "q", "n": 6}]`
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	reader := NewTaskParser(parserkit.NewBase("x", "x"), "x", "", nil)
	tests := []struct {
		name, directory, want string
		grouped               bool
	}{
		{"the id as the directory spells it", "ab1", `{"id":"ab1","n":1}`, false},
		{"the id in another case", "AB1", `{"id":"ab1","n":1}`, false},
		{"the exact spelling among ids equal ignoring case", "XY", `{"id":"XY","n":4}`, false},
		{"ids equal ignoring case and none exact", "xy", "null", false},
		{"a repeated id in another case has the id's group", "Q", `{"id":"q","n":5}`, true},
		{"an id no entry has in any case", "zz", "null", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := reader.CachedEntry(path, "", test.directory, nil).JSON(); got != test.want {
				t.Errorf("entry %s, want %s", got, test.want)
			}
			group := reader.EntryGroup(path, "", test.directory, nil)
			if (group != nil) != test.grouped {
				t.Fatalf("group %v, want one: %v", group, test.grouped)
			}
			if group != nil && reader.EntryGroup(path, "", "q", nil) != group {
				t.Error("the directory's group is not the id's")
			}
		})
	}
}
