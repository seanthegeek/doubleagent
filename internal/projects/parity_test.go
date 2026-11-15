//go:build parity && unix

package projects

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// smokeImage builds the sh smoke test's fake image without running its
// checks and returns the image root.
func smokeImage(t *testing.T) string {
	t.Helper()
	script, err := filepath.Abs("../../collectors/tests/smoke.sh")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("sh", script)
	command.Dir = filepath.Dir(filepath.Dir(filepath.Dir(script)))
	command.Env = append(os.Environ(), "SMOKE_IMAGE_ONLY=1", "TMPDIR="+t.TempDir(),
		"PATH="+systemPath)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("smoke image: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return lines[len(lines)-1] + "/root"
}

func TestDiscoverAgreesWithShCollectorOverSmokeImage(t *testing.T) {
	root := smokeImage(t)
	// The -p sets are those of the smoke test's runs that pass any.
	for _, extra := range [][]string{
		nil,
		{root + "/home/dot/.claude/proj"},
		{root + "/home", root + "/opt/daemonproj"},
		{root + "/srv/ccwork", root + "/srv/cchist"},
		{"/nonexistent-cac/proj"},
	} {
		t.Run("with -p "+strings.Join(extra, " "), func(t *testing.T) {
			if compareWithSh(t, root, extra) == 0 {
				t.Error("the sh collector found no project")
			}
		})
	}
}
