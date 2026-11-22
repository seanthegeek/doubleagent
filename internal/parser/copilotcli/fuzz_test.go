package copilotcli

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

func FuzzReadFlatYAML(fuzz *testing.F) {
	for _, seed := range []string{"", "id: x\ncwd: '/srv'\n", "q: \"a\\\"b\"\nr: \"bad\n", "\xef\xbb\xbfid: 1", ": :\n"} {
		fuzz.Add(seed)
	}
	directory := fuzz.TempDir()
	fuzz.Fuzz(func(t *testing.T, data string) {
		path := filepath.Join(directory, "workspace.yaml")
		if err := osfile.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		readFlatYAML(path, &parser.Problems{})
	})
}
