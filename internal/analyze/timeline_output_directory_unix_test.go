//go:build unix

package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

func TestTimelineNeverFollowsASymbolicLinkAsItsOutput(t *testing.T) {
	work := t.TempDir()
	target := filepath.Join(work, "target")
	if err := osfile.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(work, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCommand(testEnvironment(), "timeline", filepath.Join(work, "missing"), link)
	want := "doubleagent timeline: the output directory " + link + " is not a directory\n"
	if code != ExitError || !strings.HasPrefix(stderr, want) {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestTimelineRefusesADirectoryItCannotWriteTo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the superuser can write to any directory")
	}
	work := t.TempDir()
	// A readable input, since the output is made only once the input is read.
	input := filepath.Join(work, "input")
	if err := osfile.MkdirAll(input, 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		output func(readOnly string) string
		want   func(output string) string
	}{
		{"an existing empty directory", func(readOnly string) string { return readOnly },
			func(output string) string {
				return "error: " + filepath.Join(output, timelineFile) + ": permission denied\n"
			}},
		{"a new directory inside a read-only one", func(readOnly string) string {
			return filepath.Join(readOnly, "new")
		}, func(output string) string { return "error: " + output + ": permission denied\n" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			readOnly := filepath.Join(work, strings.ReplaceAll(test.name, " ", "-"))
			if err := osfile.MkdirAll(readOnly, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(readOnly, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(readOnly, 0o755) })
			output := test.output(readOnly)
			wantText := test.want(output)
			code, _, stderr := runCommand(testEnvironment(), "timeline", input, output)
			if code != ExitError || !strings.HasPrefix(stderr, wantText) {
				t.Fatalf("exit %d, stderr %q does not hold %q", code, stderr, wantText)
			}
		})
	}
}

func TestTimelineCleansItsOutputPathByNameBeforeWritingIt(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "lt", "alice", ".claude", "history.jsonl"), historyLine)
	if err := os.MkdirAll(filepath.Join(work, "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(work, "deep"), filepath.Join(work, "link")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)
	code, _, stderr := runCommand(testEnvironment(), "timeline", "lt", filepath.Join("link", "..", "out"))
	if code == ExitError {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if !lstatExists(filepath.Join(work, "out", "timeline.jsonl")) {
		t.Error("the output was not written to out, the name the cleaned path gives")
	}
	if lstatExists(filepath.Join(work, "deep", "timeline.jsonl")) {
		t.Error("the output was written beside the link's target")
	}
}
