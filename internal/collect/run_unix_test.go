//go:build unix

package collect

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/summary"
	"github.com/seanthegeek/doubleagent/internal/volumes"
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

func TestUnixHostDescription(t *testing.T) {
	if got := identityText(summary.Identity{UID: 501, RunAs: "ignored"}); got != "uid=501" {
		t.Errorf("identityText = %q, want uid=501", got)
	}
	if !privileged(summary.Identity{UID: 0}) || privileged(summary.Identity{UID: 501, Admin: true}) {
		t.Error("privileged must follow the user ID alone")
	}
	want := volumes.Bases{System: "/img", ProgramData: "/img/ProgramData"}
	if got := systemBases("/img"); got != want {
		t.Errorf("systemBases = %+v, want %+v", got, want)
	}
	if live := (volumes.Bases{ProgramData: "/ProgramData"}); systemBases("") != live {
		t.Errorf("systemBases of a live run = %+v, want %+v", systemBases(""), live)
	}
	if !liveRoot("/") || liveRoot("/img") || registryReader() != nil {
		t.Error("only / is live and Unix has no registry")
	}
}

// longTempDir returns a new temporary directory as the test framework names
// it, a symlinked parent included, which the resolution of -r goes through.
func longTempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}
