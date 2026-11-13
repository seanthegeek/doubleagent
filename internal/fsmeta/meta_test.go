//go:build unix

package fsmeta

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// statLine runs the system stat for path and returns the eight fields the sh
// collector reads: size mtime atime ctime btime uid gid mode.
func statLine(t *testing.T, path string) string {
	t.Helper()
	args := []string{"-c", "%s %Y %X %Z %W %u %g %a", path}
	if runtime.GOOS == "darwin" {
		args = []string{"-f", "%z %m %a %c %B %u %g %Lp", path}
	}
	output, err := exec.Command("/usr/bin/stat", args...).Output()
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return strings.TrimSpace(string(output))
}

func TestLstatMatchesStat(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "regular")
	setuid := filepath.Join(root, "setuid")
	link := filepath.Join(root, "link")
	write := func(path string, mode os.FileMode) {
		if err := os.WriteFile(path, []byte("hello world"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	write(regular, 0o644)
	write(setuid, os.ModeSetuid|0o755)
	if err := os.Symlink("regular", link); err != nil {
		t.Fatal(err)
	}
	past := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(regular, past, past); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		path string
		kind Kind
		mode string
	}{
		{"a regular file", regular, File, "644"},
		{"a file with the setuid bit", setuid, File, "4755"},
		{"a symlink is described, not followed", link, Symlink, ""},
		{"a directory", root, Dir, ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			meta, err := Lstat(testCase.path)
			if err != nil {
				t.Fatal(err)
			}
			if meta.Kind != testCase.kind {
				t.Errorf("kind = %v, want %v", meta.Kind, testCase.kind)
			}
			if testCase.mode != "" && meta.Mode != testCase.mode {
				t.Errorf("mode = %q, want %q", meta.Mode, testCase.mode)
			}
			mode := meta.Mode
			if runtime.GOOS == "darwin" && len(mode) > 3 {
				mode = mode[len(mode)-3:] // BSD stat %Lp leaves out the special bits
			}
			got := fmt.Sprintf("%d %d %d %d %d %d %d %s", meta.Size, meta.Mtime,
				meta.Atime, meta.Ctime, meta.Btime, meta.UID, meta.GID, mode)
			if want := statLine(t, testCase.path); got != want {
				t.Errorf("fields = %q, stat says %q", got, want)
			}
		})
	}
}

func TestLstatKinds(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if meta, err := Lstat(fifo); err != nil || meta.Kind != Fifo {
		t.Errorf("fifo: kind %v, error %v", meta.Kind, err)
	}
	if meta, err := Lstat("/dev/null"); err != nil || meta.Kind != Device {
		t.Errorf("/dev/null: kind %v, error %v", meta.Kind, err)
	}
	if _, err := Lstat(filepath.Join(root, "missing")); !os.IsNotExist(err) {
		t.Errorf("missing file: error %v, want not-exist", err)
	}
}

func TestKindOf(t *testing.T) {
	cases := []struct {
		name string
		mode uint32
		want Kind
	}{
		{"socket", 0o140755, Socket},
		{"block device", 0o060644, Device},
		{"character device", 0o020644, Device},
		{"unknown bits", 0o110000, Other},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := kindOf(testCase.mode); got != testCase.want {
				t.Errorf("kindOf(%o) = %v, want %v", testCase.mode, got, testCase.want)
			}
		})
	}
}
