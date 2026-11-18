package clinelegacy

import (
	"math/bits"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

func TestCachedEntryGivesTheFirstObjectOfTheID(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "index.json")
	text := `{"entries": [7, {"id": "a", "n": 1}, {"id": 1, "n": 2}, {"id": "a", "n": 3}, {"id": "1", "n": 4}, {"n": 5}]}`
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	reader := NewTaskParser(parser.NewBase("x", "x"), "x", "", nil)
	reader.Work = parser.CountingWork()
	tests := []struct {
		name, member, entryID, want string
	}{
		{"the first of two entries with the id", "entries", "a", `{"id":"a","n":1}`},
		{"a number id matches by its text", "entries", "1", `{"id":1,"n":2}`},
		{"an id no entry has", "entries", "b", "null"},
		{"an entry without an id has the empty id", "entries", "", `{"n":5}`},
		{"a document that is not a list has no entries", "", "a", "null"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := reader.CachedEntry(path, test.member, test.entryID).JSON(); got != test.want {
				t.Errorf("got %s, want %s", got, test.want)
			}
		})
	}
	if missing := reader.CachedEntry(filepath.Join(directory, "absent.json"), "", "a"); !missing.IsNull() {
		t.Errorf("an absent file gave %s", missing.JSON())
	}
}

func TestAnIndexOfManyTasksIsReadOncePerFile(t *testing.T) {
	const tasks = 2000
	entries := make([]string, tasks)
	for i := range entries {
		entries[i] = `{"id": "t` + strconv.Itoa(i) + `"}`
	}
	path := filepath.Join(t.TempDir(), "taskHistory.json")
	if err := osfile.WriteFile(path, []byte("["+strings.Join(entries, ",")+"]"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader := NewTaskParser(parser.NewBase("x", "x"), "x", "", nil)
	reader.Work = parser.CountingWork()
	for i := range tasks {
		if entry := reader.CachedEntry(path, "", "t"+strconv.Itoa(i)); !entry.IsObject() {
			t.Fatalf("task %d has no entry", i)
		}
	}
	if read := reader.Work.Count("index items read"); read != tasks {
		t.Errorf("%d items read for %d lookups in an index of %d", read, tasks, tasks)
	}
}

func TestModelAtIsABinarySearch(t *testing.T) {
	const entries = 4096
	usage := make([]string, entries)
	for i := range usage {
		usage[i] = `{"ts": ` + strconv.Itoa(i*10) + `, "model_id": "m` + strconv.Itoa(i) + `"}`
	}
	metadata, err := jsonvalue.DecodeString(`{"model_usage": [` + strings.Join(usage, ",") + `]}`)
	if err != nil {
		t.Fatal(err)
	}
	context := newTaskContext("t", jsonvalue.NewNull(), metadata, "cwd")
	context.work = parser.CountingWork()
	for i := range entries {
		if got := context.ModelAt(jsonvalue.NewInt(int64(i*10 + 5))); got != "m"+strconv.Itoa(i) {
			t.Fatalf("ModelAt(%d) = %q", i*10+5, got)
		}
	}
	compared, limit := context.work.Count("entries compared"), entries*(bits.Len(entries)+1)
	if compared == 0 || compared > limit {
		t.Errorf("%d entries looked at for %d lookups, over %d", compared, entries, limit)
	}
}
