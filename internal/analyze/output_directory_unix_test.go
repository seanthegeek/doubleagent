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
