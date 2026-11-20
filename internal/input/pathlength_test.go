package input

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// renamedEntry is a directory entry listed under another name, as a file
// system that holds longer names would list it.
type renamedEntry struct {
	fs.DirEntry
	name string
}

// Name returns the entry's listed name.
func (r renamedEntry) Name() string { return r.name }

func TestAManifestFileWhosePathIsLongerThanTheLimitIsAProblem(t *testing.T) {
	prefix := "/home/a/.pi/agent/sessions/"
	tests := []struct {
		name    string
		length  int
		refused bool
	}{
		{"a path of the limit is read", MaxPathLength, false},
		{"a path one byte longer is a problem", MaxPathLength + 1, true},
		{"a path of 10,000 bytes is a problem", 10_000, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := prefix + strings.Repeat("a", test.length-len(prefix)-len("/s.jsonl")) + "/s.jsonl"
			// The archive member is short, as a manifest's text may make it.
			writeManifest(t, dir, collectedRow("a", "/home/a", "pi", path, "fs/home/a/s.jsonl"))
			writeFile(t, filepath.Join(dir, "fs", "home", "a", "s.jsonl"), "{}\n")
			collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, "")})
			opened(t, err)
			if refused := len(collection.Artifacts) == 0; refused != test.refused {
				t.Errorf("%d artifacts, want refused %v", len(collection.Artifacts), test.refused)
			}
			if test.refused {
				wantSlice(t, "problems", collection.Problems, []string{path + ": " + errTooLong.Error()})
			}
		})
	}
}

func TestALooseFileWhosePathIsLongerThanTheLimitIsAProblem(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alice", ".claude", "history.jsonl"), "{}")
	writeFile(t, filepath.Join(dir, "alice", ".claude", "long.jsonl"), "{}")
	long := strings.Repeat("l", 10_000) + ".jsonl"
	readDir := func(path string) ([]fs.DirEntry, error) {
		entries, err := osfile.ReadDir(path)
		for i, entry := range entries {
			if entry.Name() == "long.jsonl" {
				entries[i] = renamedEntry{entry, long}
			}
		}
		return entries, err
	}
	collection, err := Open(t.Context(), dir, Options{Matcher: matcherOf(t, ""), ReadDir: readDir})
	opened(t, err)
	wantSlice(t, "artifacts", rels(collection), []string{".claude/history.jsonl"})
	wantSlice(t, "problems", collection.Problems,
		[]string{filepath.Join(dir, "alice", ".claude", long) + ": " + errTooLong.Error()})
}
