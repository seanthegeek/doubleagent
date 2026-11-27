package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/input"
)

// openCollection opens the collection at root with the bundled catalog.
func openCollection(t *testing.T, root string) *input.Collection {
	t.Helper()
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	matcher, err := catalog.NewMatcher(tables)
	if err != nil {
		t.Fatal(err)
	}
	collection, err := input.Open(t.Context(), root, input.Options{Matcher: matcher})
	if err != nil {
		t.Fatal(err)
	}
	return collection
}

func TestTheStoresDirectoryOfARolloutLiesBelowItsHome(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "collection")
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "fs", "x.jsonl"), "{}\n")
	write(filepath.Join(dir, "logs_1.sqlite"), "planted outside the collection")
	write(filepath.Join(root, "manifest.jsonl"), `{"path":"/home/u/.codex/sessions/2026/01/02/rollout-x.jsonl",`+
		`"home":"/home/u","user":"u","agent":"codex-cli","archive_path":"fs/x.jsonl","type":"file",`+
		`"status":"collected","size":3}`+"\n")
	collection := openCollection(t, root)
	if len(collection.Artifacts) != 1 {
		t.Fatalf("artifacts %d, problems %q", len(collection.Artifacts), collection.Problems)
	}
	home, ok := New().codexHome(collection.Artifacts[0])
	if !ok {
		t.Fatal("no stores directory")
	}
	if want := filepath.Join(root, "fs", "home", "u", ".codex"); home != want {
		t.Errorf("stores directory %s, want %s", home, want)
	}
	for _, database := range logDatabases(&input.Artifact{}, home) {
		if !strings.HasPrefix(database, root+string(filepath.Separator)) {
			t.Errorf("log database %s lies outside the collection", database)
		}
	}
}
