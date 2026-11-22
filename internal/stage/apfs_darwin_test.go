package stage

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// caseSensitiveVolume attaches a new case-sensitive APFS disk image under a
// test directory and returns its mount point, detached when the test ends;
// the test is skipped where hdiutil cannot make or attach one. Every APFS
// volume, a case-sensitive one included, takes the two Unicode forms of a
// name for one.
func caseSensitiveVolume(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	image, mount := dir+"/volume.dmg", dir+"/volume"
	create := exec.Command("hdiutil", "create", "-size", "16m", "-fs", "Case-sensitive APFS",
		"-volname", "stage", image, "-quiet")
	if output, err := create.CombinedOutput(); err != nil {
		t.Skipf("hdiutil create: %v %s", err, output)
	}
	attach := exec.Command("hdiutil", "attach", "-nobrowse", "-mountpoint", mount, image, "-quiet")
	if output, err := attach.CombinedOutput(); err != nil {
		t.Skipf("hdiutil attach: %v %s", err, output)
	}
	t.Cleanup(func() { _ = exec.Command("hdiutil", "detach", mount, "-force", "-quiet").Run() })
	physical, err := filepath.EvalSymlinks(mount)
	if err != nil {
		t.Fatal(err)
	}
	return physical
}
