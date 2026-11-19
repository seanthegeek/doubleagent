//go:build unix

package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/reason"
)

// belowFileReason is the reason the write of an output below a regular file
// fails with: os.MkdirAll reports ENOTDIR, which is "not a directory" here.
const belowFileReason = reason.NotDirectory

func TestAnInputTheCheckCannotReachGivesTheReadsOwnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the superuser enters any directory")
	}
	work := t.TempDir()
	locked := filepath.Join(work, "locked")
	archive := filepath.Join(locked, "x.tar.gz")
	writeHomeArchive(t, archive)
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	dangling := filepath.Join(work, "dangling")
	if err := os.Symlink(filepath.Join(work, "nothing"), dangling); err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, input, want string }{
		{"an archive in a directory that cannot be entered", archive,
			"error: " + archive + ": permission denied\n"},
		{"a link to nothing", dangling, "error: " + dangling + ": no such file or directory\n"},
		{"a device", os.DevNull, "error: " + os.DevNull + " is neither a regular file nor a directory\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := filepath.Join(work, "out", "below")
			code, stdout, stderr := runCommand(testEnvironment(), "timeline", test.input, output)
			if code != ExitError || stderr != test.want || stdout != "" {
				t.Errorf("exit %d, stdout %q, stderr %q, want %q", code, stdout, stderr, test.want)
			}
			if lstatExists(filepath.Join(work, "out")) {
				t.Error("the output was made before the input was read")
				_ = osfile.RemoveAll(filepath.Join(work, "out"))
			}
		})
	}
}

func TestFleetReadsTheFilesOfTheDirectoryItListed(t *testing.T) {
	// A .. after a link leads up from the link's target, so lsub/.. is inv,
	// and its files are read there, not where the text lsub/.. would clean to.
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "inv", "a.json"), `{"type":"host","host":"listed"}`+"\n")
	writeFile(t, filepath.Join(work, "inv", "sub", "keep"), "")
	writeFile(t, filepath.Join(work, "a.json"), `{"type":"host","host":"beside"}`+"\n")
	if err := os.Symlink(filepath.Join(work, "inv", "sub"), filepath.Join(work, "lsub")); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(work, "out", "f.csv")
	code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", output,
		filepath.Join(work, "lsub")+"/..")
	data, err := osfile.ReadFile(output)
	if code != ExitFound || err != nil || !strings.Contains(string(data), "listed") ||
		strings.Contains(string(data), "beside") {
		t.Errorf("exit %d, stderr %q, CSV %q (%v)", code, stderr, data, err)
	}
}
