//go:build unix

package collect

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// These tests need POSIX permission bits.
func TestOutputDirectoryThatCannotBeWrittenIsFatal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may write any directory")
	}
	outDir := t.TempDir()
	if err := os.Chmod(outDir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(outDir, 0o700)
	var stdout, stderr bytes.Buffer
	options := testOptions(fixtureImage(t), outDir)
	code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr)
	want := "Output dir not writable: " + outDir + "\n"
	if code != exitFatal || stderr.String() != want {
		t.Errorf("exit code %d, stderr %q; want 2 and %q", code, stderr.String(), want)
	}
}

func TestRunCreatesFilesForTheOwnerOnly(t *testing.T) {
	outDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	options := testOptions(fixtureImage(t), outDir)
	if code := runCollection(context.Background(), options, testHost(t), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	for _, name := range outputNames(t, outDir) {
		info, err := os.Lstat(filepath.Join(outDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s has mode %o, want 600", name, info.Mode().Perm())
		}
	}
}
