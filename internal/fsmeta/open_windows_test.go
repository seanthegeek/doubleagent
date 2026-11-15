package fsmeta

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

func TestOpenRegularErrorNamesTheNativePath(t *testing.T) {
	canonical := syspath.FromNative(t.TempDir())
	_, err := (&Opener{}).OpenRegular(canonical)
	if !errors.Is(err, ErrNotRegular) {
		t.Fatalf("OpenRegular(directory) error %v, want ErrNotRegular", err)
	}
	text := err.Error()
	if !strings.Contains(text, syspath.ToNative(canonical)) || strings.Contains(text, "/") ||
		strings.Contains(text, `\\?\`) {
		t.Errorf("error %q, want the backslashed path %q and no slash or \\\\?\\ prefix",
			text, syspath.ToNative(canonical))
	}
}

// tempFile creates a file in a new temporary directory and returns its
// canonical path.
func tempFile(t *testing.T) string {
	t.Helper()
	path := syspath.FromNative(t.TempDir()) + "/file"
	if err := os.WriteFile(path, []byte("evidence"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// mayHoldTimes opens path through opener and reports whether the handle it
// returns holds the right to set the file's times: SetFileTime with the
// hold value is refused with access denied on a handle without it.
func mayHoldTimes(t *testing.T, opener *Opener, path string) bool {
	t.Helper()
	file, err := opener.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	err = HoldTimes(windows.Handle(file.Fd()), false)
	return !errors.Is(err, windows.ERROR_ACCESS_DENIED)
}

// requireLocalNTFS skips the test unless path is on a local NTFS volume.
func requireLocalNTFS(t *testing.T, path string) {
	t.Helper()
	file, err := os.Open(syspath.ToNative(path))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if fileSystem := localFileSystem(windows.Handle(file.Fd())); fileSystem != "NTFS" {
		t.Skipf("%s is on %q, not a local NTFS volume", path, fileSystem)
	}
}

func TestOpenerDecision(t *testing.T) {
	cases := []struct {
		name       string
		opener     *Opener
		wantJudges bool
		wantAccess []uint32
	}{
		{"a nil opener holds", nil, false, holding},
		{"the default opener holds", NewOpener(false, false), false, holding},
		{"--no-preserve-atime reads only", NewOpener(true, false), false, readingOnly},
		{"--no-preserve-atime with updates off reads only", NewOpener(true, true), false, readingOnly},
		{"updates off judges the volume, then holds", NewOpener(false, true), true, holding},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			judges, access := test.opener.judgesVolume(), accessList(test.opener.asksForRight())
			if judges != test.wantJudges || fmt.Sprint(access) != fmt.Sprint(test.wantAccess) {
				t.Errorf("judges %v, access %#x; want %v, %#x", judges, access,
					test.wantJudges, test.wantAccess)
			}
		})
	}
}

// The access lists openHolding tries.
var (
	holding     = []uint32{windows.GENERIC_READ | windows.FILE_WRITE_ATTRIBUTES, windows.GENERIC_READ}
	readingOnly = []uint32{windows.GENERIC_READ}
)

func TestTheHandleReturnedIsTheHandleJudged(t *testing.T) {
	local := tempFile(t)
	requireLocalNTFS(t, local)
	t.Run("updates off returns a local NTFS file on a read-only handle", func(t *testing.T) {
		if mayHoldTimes(t, NewOpener(false, true), local) {
			t.Error("the handle may set the file's times")
		}
	})
	t.Run("updates on returns it on a handle that holds", func(t *testing.T) {
		if !mayHoldTimes(t, NewOpener(false, false), local) {
			t.Error("the handle may not set the file's times")
		}
	})
	t.Run("a junction in the path does not stop the judgment", func(t *testing.T) {
		parent := syspath.FromNative(t.TempDir())
		if err := os.Mkdir(parent+"/target", 0o755); err != nil {
			t.Fatal(err)
		}
		makeJunction(t, syspath.ToNative(parent+"/target"), syspath.ToNative(parent+"/junction"))
		if err := os.WriteFile(parent+"/target/file", []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if mayHoldTimes(t, NewOpener(false, true), parent+"/junction/file") {
			t.Error("a local NTFS file reached through a junction got the right")
		}
	})
}

// throughShare returns the canonical path of the local path reached through
// the administrative share of its drive on this machine (C:/x as
// //localhost/C$/x), a remote volume to the opener whose server reports
// NTFS, or skips the test when the share cannot be reached.
func throughShare(t *testing.T, path string) string {
	t.Helper()
	if len(path) < 3 || path[1] != ':' {
		t.Skipf("%s is not on a drive letter", path)
	}
	shared := "//localhost/" + path[:1] + "$" + path[2:]
	if _, err := os.Lstat(syspath.ToNative(shared)); err != nil {
		t.Skipf("the administrative share is unreachable here: %v", err)
	}
	return shared
}

func TestUpdatesOffStillHoldsAFileOnARemoteVolume(t *testing.T) {
	// A file the test owns, so the right is granted wherever it is asked
	// for, read through the loopback share.
	shared := throughShare(t, tempFile(t))
	if !mayHoldTimes(t, NewOpener(false, true), shared) {
		t.Errorf("%s: opened read-only; a remote volume is not governed by the local setting", shared)
	}
}

func TestUpdatesOffHoldsAFileThroughALinkToARemoteVolume(t *testing.T) {
	file := tempFile(t)
	shared := throughShare(t, file)
	link := syspath.FromNative(t.TempDir()) + "/through"
	directory := shared[:strings.LastIndexByte(shared, '/')]
	if err := os.Symlink(syspath.ToNative(directory), syspath.ToNative(link)); err != nil {
		t.Skipf("cannot create a directory symlink here (the privilege is needed): %v", err)
	}
	if !mayHoldTimes(t, NewOpener(false, true), link+"/file") {
		t.Errorf("a file reached through a symlink to a share was opened read-only")
	}
}
