package analyze

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			"must be outside the input"},
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
