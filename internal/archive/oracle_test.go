package archive

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// listMembers returns the sorted member names tar -tzf prints.
func listMembers(t *testing.T, archivePath string) []string {
	t.Helper()
	output, err := exec.Command("/usr/bin/tar", "-tzf", archivePath).Output()
	if err != nil {
		t.Skipf("system tar unavailable: %v", err)
	}
	names := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	for index, name := range names {
		names[index] = strings.TrimSuffix(name, "/")
	}
	slices.Sort(names)
	return names
}

func TestMembersEqualSystemTar(t *testing.T) {
	stage := buildStage(t)
	dir := t.TempDir()
	goArchive, systemArchive := filepath.Join(dir, "go.tar.gz"), filepath.Join(dir, "sys.tar.gz")
	if err := WriteTarGz(stage, goArchive); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("/usr/bin/tar", "-czf", systemArchive, "-C", stage, ".").CombinedOutput(); err != nil {
		t.Skipf("system tar cannot write: %v %s", err, output)
	}
	if got, want := listMembers(t, goArchive), listMembers(t, systemArchive); !slices.Equal(got, want) {
		t.Errorf("members = %v, tar = %v", got, want)
	}
	if _, err := os.Stat(goArchive); err != nil {
		t.Fatal(err)
	}
}
