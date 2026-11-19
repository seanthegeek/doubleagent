//go:build parity && unix

package replay

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

func TestTheInputOfACaseIsItsRegularFilesAndNotWhatALinkNames(t *testing.T) {
	outside, root := t.TempDir(), t.TempDir()
	const log = `{"type": "network", "payload": {"id": "r1", "method": "GET", "url": "https://h/p?key=AIza"}}` + "\n"
	for _, path := range []string{filepath.Join(outside, "log.jsonl"), filepath.Join(root, "log.jsonl")} {
		if err := osfile.WriteFile(path, []byte(log), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(outside, "log.jsonl"), filepath.Join(root, "link.jsonl")); err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`key=AIza`)
	tests := []struct {
		name string
		file string
		held bool
	}{
		{"a regular file is read", "log.jsonl", true},
		{"a link to a file outside the tree is not", "link.jsonl", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]bool{test.file: true}
			if held := treeHolds(root, files)(pattern); held != test.held {
				t.Errorf("the pattern found %v, want %v", held, test.held)
			}
			if held := len(treeRequests(root, files)()["r1"]) == 1; held != test.held {
				t.Errorf("the request found %v, want %v", held, test.held)
			}
		})
	}
}
