//go:build unix

package fsmeta

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
			reason, failed := DirReadError(testCase.path)
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
	reason, failed := DirReadError(path)
	if !failed || strings.Contains(reason, "\n") {
		t.Errorf("reason = %q, failed = %v; want one line", reason, failed)
	}
}

func TestDiskUsageBytesMatchesDu(t *testing.T) {
	root := t.TempDir()
	tree := filepath.Join(root, "tree")
	for _, dir := range []string{"a/b", "c"} {
		if err := os.MkdirAll(filepath.Join(tree, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]int{"a/one": 1, "a/b/two": 5000, "c/three": 70000, "empty": 0}
	for name, size := range files {
		if err := os.WriteFile(filepath.Join(tree, name), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Link(filepath.Join(tree, "c/three"), filepath.Join(tree, "a/again")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("c/three", filepath.Join(tree, "link")); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("/usr/bin/du", "-sk", tree).Output()
	if err != nil {
		t.Fatal(err)
	}
	kibibytes, err := strconv.ParseInt(strings.Fields(string(output))[0], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if got := DiskUsageBytes(tree); got != kibibytes*1024 {
		t.Errorf("DiskUsageBytes = %d, du -sk says %d KiB", got, kibibytes)
	}
	if got := DiskUsageBytes(filepath.Join(root, "missing")); got != 0 {
		t.Errorf("DiskUsageBytes of a missing path = %d, want 0", got)
	}
}
