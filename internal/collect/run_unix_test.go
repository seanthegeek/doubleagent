//go:build unix

package collect

import (
	"bytes"
	"context"
	"os"
	"testing"
)

// Mode 0o500 on a directory stops writes only where the platform enforces
// POSIX permission bits.
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
