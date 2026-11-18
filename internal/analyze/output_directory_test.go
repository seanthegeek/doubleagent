package analyze

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// lstatExists reports whether path exists, without following a link.
func lstatExists(path string) bool {
	_, err := osfile.Lstat(path)
	return err == nil
}

func TestTimelineChecksItsOutputDirectoryBeforeReadingTheInput(t *testing.T) {
	work := t.TempDir()
	missingInput := filepath.Join(work, "no-such-input")
	tests := []struct {
		name    string
		prepare func(t *testing.T, output string)
		message string
	}{
		{"a file is not a directory", func(t *testing.T, output string) { writeFile(t, output, "x") },
			"the output directory {OUT} is not a directory"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "out")
			test.prepare(t, output)
			code, stdout, stderr := runCommand(testEnvironment(), "timeline", missingInput, output)
			want := "doubleagent timeline: " + strings.ReplaceAll(test.message, "{OUT}", output) + "\nUsage:"
			if code != ExitError || stdout != "" || !strings.HasPrefix(stderr, want) {
				t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
			}
		})
	}
}

func TestTimelineAcceptsAnEmptyDirectoryAndMakesAMissingOne(t *testing.T) {
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"), []jsonvalue.Member{
		member("hostname", jsonvalue.NewString("h1"))}, timelineFiles)
	empty := filepath.Join(work, "empty")
	if err := osfile.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{"an empty directory": empty,
		"a missing directory and its parents": filepath.Join(work, "new", "deeper")} {
		t.Run(name, func(t *testing.T) {
			code, _, stderr := runCommand(testEnvironment(), "timeline", collection, output)
			if code != ExitFound && code != ExitNothing {
				t.Fatalf("exit %d, stderr %q", code, stderr)
			}
			if !lstatExists(filepath.Join(output, InventoryFile)) {
				t.Errorf("%s was not written", InventoryFile)
			}
		})
	}
}
