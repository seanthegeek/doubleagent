package analyze

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

func TestAnOutputInsideADirectoryInputIsRefused(t *testing.T) {
	dir := t.TempDir()
	tree := filepath.Join(dir, "loose")
	writeFile(t, filepath.Join(tree, "home", "a", ".claude", "history.jsonl"), "{}\n")
	archive := filepath.Join(dir, "home.tar")
	writeHomeArchive(t, archive)
	inventories := filepath.Join(dir, "inventories")
	writeFile(t, filepath.Join(inventories, "h.jsonl"), `{"type":"host","host":"h"}`+"\n")
	cases := []struct {
		name    string
		args    []string
		code    int
		message string
	}{
		{"timeline into the directory it reads", []string{"timeline", tree,
			filepath.Join(tree, "home", "a", ".claude", "out")}, ExitError,
			"the output directory " + filepath.Join(tree, "home", "a", ".claude", "out") + " must be outside the input"},
		{"timeline into the input itself", []string{"timeline", tree, tree}, ExitError,
			"is not empty"},
		{"fleet into the directory it reads", []string{"fleet", "-o", filepath.Join(inventories, "f.csv"),
			inventories}, ExitError, "must be outside the input directory " + inventories},
		{"timeline beside an archive it reads", []string{"timeline", archive, filepath.Join(dir, "out")},
			ExitFound, ""},
		{"timeline into a sibling of the input", []string{"timeline", tree, tree + "-out"}, ExitFound, ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), test.args, System(), &stdout, &stderr)
			if code != test.code || !strings.Contains(stderr.String(), test.message) {
				t.Fatalf("exit %d, stderr %q", code, stderr.String())
			}
			if test.code == ExitError && !strings.Contains(stderr.String(), "\nUsage: doubleagent ") {
				t.Errorf("not a usage error: %q", stderr.String())
			}
		})
	}
	for _, unwritten := range []string{filepath.Join(tree, "home", "a", ".claude", "out"),
		filepath.Join(tree, TimelineFile), filepath.Join(inventories, "f.csv")} {
		if _, err := os.Lstat(unwritten); err == nil {
			t.Errorf("%s was written into the input", unwritten)
		}
	}
}

// deepTree makes directories d/d/... below root, one more level each until
// the system refuses a longer path or 2000 levels stand, and returns the
// one halfway down: its path leaves room for the files written in it, and
// climbing from it to root by parent entries, three bytes longer a level,
// passes the longest path the system takes.
func deepTree(t *testing.T, root string) string {
	t.Helper()
	var levels []string
	for current := root; len(levels) < 2000; {
		current = filepath.Join(current, "d")
		if err := osfile.MkdirAll(current, 0o700); err != nil {
			break
		}
		levels = append(levels, current)
	}
	if len(levels) < 100 {
		t.Fatalf("only %d levels could be made", len(levels))
	}
	return levels[len(levels)/2]
}

func TestAnOutputDeepInsideADirectoryInputIsRefused(t *testing.T) {
	// A climb that cannot go on refuses: the check cannot tell where the
	// output lies.
	tree := filepath.Join(t.TempDir(), "loose")
	writeFile(t, filepath.Join(tree, "home", "a", ".claude", "history.jsonl"), "{}\n")
	parent := deepTree(t, tree)
	deep, csv := filepath.Join(parent, "out"), filepath.Join(parent, "f.csv")
	for _, args := range [][]string{{"timeline", tree, deep}, {"fleet", "-o", csv, tree}} {
		var stdout, stderr bytes.Buffer
		code := Run(context.Background(), args, System(), &stdout, &stderr)
		if code != ExitError || !strings.Contains(stderr.String(), "outside the input") {
			t.Errorf("%s exited %d: %q", args[0], code, stderr.String())
		}
	}
	for _, unwritten := range []string{deep, csv} {
		if _, err := osfile.Lstat(unwritten); err == nil {
			t.Errorf("%s was written into the input", unwritten)
		}
	}
}
