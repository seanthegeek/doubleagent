//go:build unix

package fsmeta

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestDirReadError(t *testing.T) {
	root := t.TempDir()
	mustMkdir := func(name string, mode os.FileMode) string {
		path := filepath.Join(root, name)
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
		return path
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(root, "dangling")
	if err := os.Symlink("nowhere", dangling); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		path   string
		want   string
		failed bool
		asUser bool // needs a user the permission bits bind
	}{
		{"a readable directory", mustMkdir("good", 0o755), "", false, false},
		{"a file", file, "open " + file + ": not a directory", true, false},
		{"a dangling link", dangling, "open " + dangling + ": not a directory", true, false},
		{"a missing path", root + "/missing", "lstat " + root + "/missing: no such file or directory", true, false},
		{"no read permission", mustMkdir("noread", 0o311), "", true, true},
		{"no search permission", mustMkdir("nosearch", 0o444), "", true, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.asUser && os.Geteuid() == 0 {
				t.Skip("root ignores permission bits")
			}
			reason, failed, _ := DirReadError(nil, testCase.path)
			if failed != testCase.failed {
				t.Fatalf("failed = %v (%q), want %v", failed, reason, testCase.failed)
			}
			if testCase.want != "" && reason != testCase.want {
				t.Errorf("reason = %q, want %q", reason, testCase.want)
			}
			if testCase.asUser && !strings.HasSuffix(reason, "permission denied") {
				t.Errorf("reason = %q, want a permission error", reason)
			}
		})
	}
}

func TestDirReadErrorKeepsFirstLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a\nb")
	reason, failed, _ := DirReadError(nil, path)
	if !failed || strings.Contains(reason, "\n") {
		t.Errorf("reason = %q, failed = %v; want one line", reason, failed)
	}
}

func TestListErrorIsTheErrorOfOpeningAndReadingOneEntry(t *testing.T) {
	root := t.TempDir()
	empty, pipe := filepath.Join(root, "empty"), filepath.Join(root, "pipe")
	if err := os.Mkdir(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skipf("cannot make a named pipe: %v", err)
	}
	t.Run("an empty directory lists", func(t *testing.T) {
		if err, refused := ListError(nil, empty); err != nil || refused != nil {
			t.Errorf("ListError = %v, %v; want nil, nil", err, refused)
		}
	})
	t.Run("a named pipe is refused at once as no directory", func(t *testing.T) {
		if err, _ := ListError(nil, pipe); err == nil || !strings.HasSuffix(err.Error(), "not a directory") {
			t.Errorf("ListError = %v; want the open's not a directory error", err)
		}
	})
}
