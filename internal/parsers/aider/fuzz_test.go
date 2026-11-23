package aider

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

func FuzzInputHistory(fuzz *testing.F) {
	for _, seed := range []string{"", "# 2026-10-01 08:00:00\n+hi\n", "+x\n\n# 2026-10-01 08:00:00.5  \n", "\xff\n"} {
		fuzz.Add(seed)
	}
	directory := fuzz.TempDir()
	fuzz.Fuzz(func(t *testing.T, data string) {
		path := filepath.Join(directory, "input.history")
		if err := osfile.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		var problems parserkit.Problems
		ReadInputHistory(path, &problems)
		_, _ = chatHeader(data)
		_ = isInputStamp(data)
		_, _, _ = llmMarker(data)
	})
}
