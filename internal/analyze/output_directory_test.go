package analyze

import (
	"os"
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
		{"a directory with a file is not empty", func(t *testing.T, output string) {
			writeFile(t, filepath.Join(output, "keep.txt"), "x")
		}, "the output directory {OUT} is not empty"},
		{"a directory with only a directory is not empty", func(t *testing.T, output string) {
			if err := osfile.MkdirAll(filepath.Join(output, "sub"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "the output directory {OUT} is not empty"},
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
			if !lstatExists(filepath.Join(output, inventoryFile)) {
				t.Errorf("%s was not written", inventoryFile)
			}
		})
	}
}

func TestAFailedTimelineRemovesTheDirectoriesItMade(t *testing.T) {
	work := t.TempDir()
	missingInput := filepath.Join(work, "no-such-input")
	existing := filepath.Join(work, "existing")
	if err := osfile.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	made := filepath.Join(existing, "a", "b")
	for name, args := range map[string][]string{
		"a missing input":     {"timeline", missingInput, made},
		"a bad filter":        {"timeline", "--since", "soon", missingInput, made},
		"an unreadable input": {"timeline", filepath.Join(work, "file-input"), made},
	} {
		t.Run(name, func(t *testing.T) {
			writeFile(t, filepath.Join(work, "file-input"), "not a collection")
			code, _, stderr := runCommand(testEnvironment(), args...)
			if code != ExitError {
				t.Fatalf("exit %d, stderr %q", code, stderr)
			}
			if lstatExists(filepath.Join(existing, "a")) {
				t.Error("a directory the run made was left behind")
			}
			if !lstatExists(existing) {
				t.Error("a directory that existed was removed")
			}
		})
	}
}

func TestAFailedTimelineKeepsAnEmptyDirectoryThatExisted(t *testing.T) {
	work := t.TempDir()
	output := filepath.Join(work, "out")
	if err := osfile.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, _ := runCommand(testEnvironment(), "timeline", filepath.Join(work, "missing"), output)
	if code != ExitError || !lstatExists(output) {
		t.Errorf("exit %d, the directory exists: %v", code, lstatExists(output))
	}
}

func TestTimelineWithTheArgumentsSwappedReadsNothing(t *testing.T) {
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"), []jsonvalue.Member{
		member("hostname", jsonvalue.NewString("h1"))}, timelineFiles)
	before := snapshot(t, collection)
	extractions := filepath.Join(work, "extractions")
	code, stdout, stderr := runCommand(testEnvironment(), "timeline", "--work-dir", extractions,
		"--keep-extracted", filepath.Join(work, "new-output"), collection)
	want := "doubleagent timeline: the output directory " + collection + " is not empty\n"
	if code != ExitError || stdout != "" || !strings.HasPrefix(stderr, want) {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if lstatExists(extractions) {
		t.Error("an extraction directory was made")
	}
	if lstatExists(filepath.Join(work, "new-output")) {
		t.Error("the input was read as far as making its output")
	}
	if after := snapshot(t, collection); after != before {
		t.Errorf("the collection changed:\nbefore %s\nafter  %s", before, after)
	}
}

// snapshot returns the names and contents of every file under directory.
func snapshot(t *testing.T, directory string) string {
	t.Helper()
	var text strings.Builder
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		text.WriteString(path + "\n")
		if !entry.IsDir() {
			data, readErr := osfile.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			text.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return text.String()
}
