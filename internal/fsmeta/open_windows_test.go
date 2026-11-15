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

func TestOpenerAccessList(t *testing.T) {
	file := tempFile(t)
	if fileSystem := newVolumeLookup()(file); fileSystem != "NTFS" {
		t.Skipf("the temporary directory is on %q, not a local NTFS volume", fileSystem)
	}
	both := []uint32{windows.GENERIC_READ | windows.FILE_WRITE_ATTRIBUTES, windows.GENERIC_READ}
	readOnly := []uint32{windows.GENERIC_READ}
	cases := []struct {
		name   string
		opener *Opener
		want   []uint32
	}{
		{"a nil opener asks for the right, then reads only", nil, both},
		{"the default opener asks for the right, then reads only", NewOpener(false, false), both},
		{"--no-preserve-atime reads only", NewOpener(true, false), readOnly},
		{"updates off reads a local NTFS file only", NewOpener(false, true), readOnly},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := accessList(test.opener.asksForRight(file))
			if fmt.Sprint(got) != fmt.Sprint(test.want) {
				t.Errorf("access list %#x, want %#x", got, test.want)
			}
		})
	}
}

func TestVolumeLookupAsksTheFileItself(t *testing.T) {
	file := tempFile(t)
	local := newVolumeLookup()(file)
	t.Logf("file system of %s: %q", file, local)
	if local == "" {
		t.Fatalf("no file system read for %s", file)
	}
	t.Run("a file reached through a junction is judged by the volume it is on", func(t *testing.T) {
		parent := syspath.FromNative(t.TempDir())
		if err := os.Mkdir(parent+"/target", 0o755); err != nil {
			t.Fatal(err)
		}
		makeJunction(t, syspath.ToNative(parent+"/target"), syspath.ToNative(parent+"/junction"))
		if err := os.WriteFile(parent+"/target/file", []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := newVolumeLookup()(parent + "/junction/file"); got != local {
			t.Errorf("file system %q through the junction, want %q", got, local)
		}
	})
	t.Run("a missing file is unknown", func(t *testing.T) {
		if got := newVolumeLookup()(file + ".missing"); got != "" {
			t.Errorf("file system %q, want none", got)
		}
	})
}

func TestUpdatesOffStillHoldsAFileOnARemoteVolume(t *testing.T) {
	// The administrative share of the system drive, reached through the
	// SMB redirector: a remote volume whose server reports NTFS.
	share := "//localhost/C$/Windows/win.ini"
	if _, err := os.Lstat(syspath.ToNative(share)); err != nil {
		t.Skipf("the administrative share is unreachable here: %v", err)
	}
	opener := NewOpener(false, true)
	if !opener.asksForRight(share) {
		t.Errorf("%s: opened read-only; a remote volume is not governed by the local setting", share)
	}
	if local := tempFile(t); opener.asksForRight(local) {
		t.Errorf("%s: asked for the right on a local NTFS file with updates off", local)
	}
}

func TestUpdatesOffHoldsAFileThroughALinkToARemoteVolume(t *testing.T) {
	share := `\\localhost\C$\Windows`
	if _, err := os.Lstat(share); err != nil {
		t.Skipf("the administrative share is unreachable here: %v", err)
	}
	link := syspath.FromNative(t.TempDir()) + "/windows"
	if err := os.Symlink(share, syspath.ToNative(link)); err != nil {
		t.Skipf("cannot create a directory symlink here (the privilege is needed): %v", err)
	}
	if !NewOpener(false, true).asksForRight(link + "/win.ini") {
		t.Errorf("a file reached through a symlink to a share was opened read-only")
	}
}
