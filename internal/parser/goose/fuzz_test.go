package goose

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

func FuzzPermissionEntries(fuzz *testing.F) {
	for _, seed := range []string{"user:\n  always_allow:\n  - a\n", "- x\n", "a:\n  b: []\n  - 'c''d'\n"} {
		fuzz.Add(seed)
	}
	directory := fuzz.TempDir()
	fuzz.Fuzz(func(t *testing.T, data string) {
		path := filepath.Join(directory, "permission.yaml")
		if err := osfile.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		var problems parser.Problems
		_ = permissionEntries(path, &problems)
		_ = unescapeHistory(data)
		_ = unquote(data)
	})
}
