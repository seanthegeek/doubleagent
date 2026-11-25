package relocation

import (
	"context"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

func TestSearchFilesReadsTheFilesInTheOrderGiven(t *testing.T) {
	root := syspath.FromNative(t.TempDir())
	writeTree(t, root, map[string]string{"profile.ps1": "$env:CODEX_HOME = 1\n",
		"b.sh": "export CODEX_HOME=1\n", "a.sh": "export CODEX_HOME=1\n"})
	tables, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	var collected []string
	run := &search{names: variableNames(tables), seen: map[string]bool{},
		deps: Deps{Tables: tables, Log: &lineLog{},
			Collect: func(_, _, path string, _ fsmeta.Meta) error {
				collected = append(collected, path)
				return nil
			}}}
	files := []string{root + "/profile.ps1", root + "/b.sh", root + "/a.sh"}
	if err := run.searchFiles(context.Background(), "", "", root, files); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(collected, files) {
		t.Errorf("files read in the order %q, want %q", collected, files)
	}
}

func TestSweepVisitsEntriesInListingOrder(t *testing.T) {
	root := syspath.FromNative(t.TempDir())
	writeTree(t, root, map[string]string{"top": "", "a/x": "", "a/inner/y": "", "b/z": ""})
	var want []string
	var expect func(directory string)
	expect = func(directory string) {
		for _, entry := range readEntries(nil, directory) {
			if path := directory + "/" + entry.Name(); entry.IsDir() {
				expect(path)
			} else {
				want = append(want, path)
			}
		}
	}
	expect(root)
	walk := sweepWalk{ctx: context.Background(), pruned: func(string) bool { return false }}
	if err := walk.visit(root); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(walk.files, want) {
		t.Errorf("files %q, want %q", walk.files, want)
	}
}
