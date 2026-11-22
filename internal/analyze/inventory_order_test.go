package analyze

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestInventoryProjectsAreWrittenInDisplayPathOrder(t *testing.T) {
	paths := []string{"/srv/b", "C:\\Users\\a", "/srv/a", "/srv/é", "/srv/B", "/srv/a b"}
	entries := make([]projectEntryJSON, len(paths))
	for i, path := range paths {
		entries[i] = projectEntryJSON{path: path, value: newProjectValue(i, 0, nil, nil)}
	}
	var out bytes.Buffer
	if err := writeJSON(&out, projectsWhere(entries, map[string]bool{}, false)); err != nil {
		t.Fatal(err)
	}
	written, err := jsonvalue.Decode(out.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	want := slices.Clone(paths)
	slices.SortFunc(want, strings.Compare)
	if got := written.Keys(); !slices.Equal(got, want) {
		t.Errorf("projects written in the order %q, want %q", got, want)
	}
}
