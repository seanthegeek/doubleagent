//go:build unix

package archive

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// listMembers returns the sorted member names tar -tzf prints, and fails the
// test with tar's message when tar cannot list the archive.
func listMembers(t *testing.T, archivePath string) []string {
	t.Helper()
	output, err := tarListing(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	names := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	for index, name := range names {
		names[index] = strings.TrimSuffix(name, "/")
	}
	slices.Sort(names)
	return names
}

func TestMembersEqualSystemTar(t *testing.T) {
	requireSystemTar(t)
	stage, _ := buildStage(t)
	dir := t.TempDir()
	goArchive, systemArchive := filepath.Join(dir, "go.tar.gz"), filepath.Join(dir, "sys.tar.gz")
	if err := WriteTarGz(context.Background(), stage, goArchive); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(systemTar, "-czf", systemArchive, "-C", stage, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("tar -czf: %v: %s", err, output)
	}
	if got, want := listMembers(t, goArchive), listMembers(t, systemArchive); !slices.Equal(got, want) {
		t.Errorf("members = %v, tar = %v", got, want)
	}
	if _, err := os.Stat(goArchive); err != nil {
		t.Fatal(err)
	}
}
