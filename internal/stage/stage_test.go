//go:build darwin || linux

package stage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// newStager returns a stager over a fresh staging directory and a fresh
// source root.
func newStager(t *testing.T) (Stager, string) {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(parent+"/stage/fs", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(parent+"/root/home", 0o755); err != nil {
		t.Fatal(err)
	}
	return Stager{Dir: parent + "/stage", Physical: parent + "/stage", Root: parent + "/root"},
		parent + "/root"
}

func TestCopyFile(t *testing.T) {
	stager, root := newStager(t)
	source := root + "/home/a\"b\nc"
	content := []byte("staged bytes")
	if err := os.WriteFile(source, content, 0o640); err != nil {
		t.Fatal(err)
	}
	mtime := time.Unix(1_700_000_000, 123_000_000)
	if err := os.Chtimes(source, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	hash, written, err := stager.CopyFile(source)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	_, dest, _ := stager.Dest(source)
	info, err := os.Lstat(dest)
	if err != nil {
		t.Fatal(err)
	}
	staged, _ := os.ReadFile(dest)
	switch {
	case hash != hex.EncodeToString(sum[:]):
		t.Errorf("hash %s", hash)
	case written != int64(len(content)):
		t.Errorf("written %d", written)
	case string(staged) != string(content):
		t.Errorf("staged %q", staged)
	case info.Mode().Perm() != 0o640:
		t.Errorf("mode %v", info.Mode())
	case !info.ModTime().Equal(mtime):
		t.Errorf("mtime %v", info.ModTime())
	}
}

func TestCopyFileRefuses(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, stager Stager, source string)
	}{
		{"a source that is a symlink", func(t *testing.T, _ Stager, source string) {
			mustDo(t, os.Symlink("elsewhere", source))
		}},
		{"a source that is a FIFO, without waiting for a writer",
			func(t *testing.T, _ Stager, source string) { mustDo(t, syscall.Mkfifo(source, 0o600)) }},
		{"a staged path that is a recreated symlink", func(t *testing.T, stager Stager, source string) {
			mustDo(t, os.WriteFile(source, []byte("x"), 0o644))
			_, dest, _ := stager.Dest(source)
			mustDo(t, os.MkdirAll(filepath.Dir(dest), 0o700))
			mustDo(t, os.Symlink(source, dest))
		}},
		{"a source that is missing", func(*testing.T, Stager, string) {}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			stager, root := newStager(t)
			source := root + "/home/f"
			test.setup(t, stager, source)
			if hash, _, err := stager.CopyFile(source); err == nil {
				t.Fatalf("copied, hash %s", hash)
			}
			if info, err := os.Lstat(source); err == nil && info.Mode().IsRegular() {
				if data, _ := os.ReadFile(source); string(data) != "x" {
					t.Errorf("source changed: %q", data)
				}
			}
		})
	}
}

func TestSymlinkLeavesAnExistingDestination(t *testing.T) {
	stager, root := newStager(t)
	mustDo(t, stager.MakeParent(root+"/home/l"))
	mustDo(t, stager.Symlink(root+"/home/l", "first", false))
	mustDo(t, stager.Symlink(root+"/home/l", "second", false))
	_, dest, _ := stager.Dest(root + "/home/l")
	if target, _ := os.Readlink(dest); target != "first" {
		t.Errorf("target %q", target)
	}
}

// linkedSource builds a source file under a symlinked directory of a home,
// .claude -> dotfiles/claude by absolute target, and recreates that link in
// the staging tree as the walk does, so the staged directory of the file
// resolves to the source. It returns the source file's path.
func linkedSource(t *testing.T, stager Stager, root string) string {
	t.Helper()
	home := root + "/home/dot"
	mustDo(t, os.MkdirAll(home+"/dotfiles/claude/proj", 0o755))
	mustDo(t, os.WriteFile(home+"/dotfiles/claude/proj/AGENTS.md", []byte("x\n"), 0o644))
	mustDo(t, os.Symlink(home+"/dotfiles/claude", home+"/.claude"))
	mustDo(t, stager.MakeParent(home+"/.claude"))
	mustDo(t, stager.Symlink(home+"/.claude", home+"/dotfiles/claude", false))
	return home + "/.claude/proj/AGENTS.md"
}

// stagedEntries lists every path under the staging directory, relative to it.
func stagedEntries(t *testing.T, stager Stager) []string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(stager.Dir, func(path string, _ fs.DirEntry, err error) error {
		entries = append(entries, strings.TrimPrefix(path, stager.Dir))
		return err
	})
	mustDo(t, err)
	return entries
}

func TestNoWriteThroughARecreatedLink(t *testing.T) {
	cases := []struct {
		name  string
		write func(stager Stager, source string) error
	}{
		{"a copy of the file the link leads to is refused", func(stager Stager, source string) error {
			_, _, err := stager.CopyFile(source)
			return err
		}},
		{"the file's staging directory is not made", func(stager Stager, source string) error {
			return stager.MakeParent(source)
		}},
		{"a new directory under the link is not made", func(stager Stager, source string) error {
			return stager.MakeParent(filepath.Dir(source) + "/new/f")
		}},
		{"a link under the link is not created", func(stager Stager, source string) error {
			return stager.Symlink(filepath.Dir(source)+"/new", "target", false)
		}},
	}
	want := []string{"", "/fs", "/fs/home", "/fs/home/dot", "/fs/home/dot/.claude"}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			stager, root := newStager(t)
			source := linkedSource(t, stager, root)
			err := test.write(stager, source)
			var pathError *fs.PathError
			if !errors.As(err, &pathError) || !errors.Is(pathError.Err, outsideStage{}) {
				t.Errorf("error %v, want one saying the staging directory is left", err)
			}
			if data, err := os.ReadFile(source); err != nil || string(data) != "x\n" {
				t.Errorf("source changed: %q, %v", data, err)
			}
			entries, _ := os.ReadDir(filepath.Dir(source))
			if len(entries) != 1 {
				t.Errorf("the source directory holds %d entries, want 1", len(entries))
			}
			if got := stagedEntries(t, stager); !slices.Equal(got, want) {
				t.Errorf("staging tree %q, want %q", got, want)
			}
		})
	}
}

func TestCopyFileWithAnUnresolvedPhysicalPath(t *testing.T) {
	stager, root := newStager(t)
	mustDo(t, os.Symlink(stager.Dir, filepath.Dir(stager.Dir)+"/alias"))
	stager.Physical = filepath.Dir(stager.Dir) + "/alias"
	mustDo(t, os.WriteFile(root+"/home/f", []byte("x"), 0o644))
	if _, _, err := stager.CopyFile(root + "/home/f"); err != nil {
		t.Errorf("CopyFile: %v", err)
	}
}

func TestRemoveFailedCopy(t *testing.T) {
	cases := []struct {
		name        string
		setup       func(t *testing.T, stager Stager, outside string) string
		wantResult  bool
		wantPresent bool
	}{
		{"nothing left behind needs no removal", func(_ *testing.T, stager Stager, _ string) string {
			return stager.Dir + "/fs/gone"
		}, true, false},
		{"a partial copy inside the staging directory is removed",
			func(t *testing.T, stager Stager, _ string) string {
				mustDo(t, os.WriteFile(stager.Dir+"/fs/partial", []byte("x"), 0o600))
				return stager.Dir + "/fs/partial"
			}, true, false},
		{"a file reached through a recreated link out of the staging tree is left",
			func(t *testing.T, stager Stager, outside string) string {
				mustDo(t, os.WriteFile(outside+"/source", []byte("x"), 0o600))
				mustDo(t, os.Symlink(outside, stager.Dir+"/fs/link"))
				return stager.Dir + "/fs/link/source"
			}, false, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			stager, _ := newStager(t)
			dest := test.setup(t, stager, t.TempDir())
			if got := stager.RemoveFailedCopy(dest); got != test.wantResult {
				t.Errorf("RemoveFailedCopy = %v", got)
			}
			if _, err := os.Lstat(dest); (err == nil) != test.wantPresent {
				t.Errorf("present after: %v", err == nil)
			}
		})
	}
}

func TestPhysicalDir(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mustDo(t, os.MkdirAll(parent+"/real/sub", 0o755))
	mustDo(t, os.Symlink("real", parent+"/link"))
	mustDo(t, os.WriteFile(parent+"/file", nil, 0o644))
	cases := []struct {
		name, dir, want string
	}{
		{"a link is resolved", parent + "/link/sub", parent + "/real/sub"},
		{"dot-dot after a link leaves the target", parent + "/link/sub/..", parent + "/real"},
		{"a file is not a directory", parent + "/file", ""},
		{"a missing directory fails", parent + "/missing", ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := PhysicalDir(test.dir)
			if got != test.want || (err == nil) != (test.want != "") {
				t.Errorf("PhysicalDir = %q, %v", got, err)
			}
		})
	}
}

func TestUnderComparesBytes(t *testing.T) {
	cases := []struct {
		name, physical, root, rest string
		inside                     bool
	}{
		{"the root itself is inside with no rest", "C:/img", "C:/img", "", true},
		{"a path below the root gives the rest", "C:/img/a/b", "C:/img", "a/b", true},
		{"a root with a trailing slash still matches", "/a/b", "/", "a/b", true},
		{"a sibling sharing a prefix is outside", "C:/img2/a", "C:/img", "", false},
		{"another ASCII case is another directory", "C:/IMG/a", "C:/img", "", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			rest, inside := Under(test.physical, test.root)
			if rest != test.rest || inside != test.inside {
				t.Errorf("Under(%q, %q) = %q, %t, want %q, %t", test.physical, test.root, rest,
					inside, test.rest, test.inside)
			}
		})
	}
}

func TestCopyFileRefusesAPathWithDotDot(t *testing.T) {
	stager, root := newStager(t)
	if err := os.WriteFile(root+"/home/f", []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root + "/home/../home/f", root + "/home/./f"} {
		if _, _, err := stager.CopyFile(path); err == nil {
			t.Errorf("CopyFile(%s) copied it", path)
		}
		if err := stager.MakeParent(path); err == nil {
			t.Errorf("MakeParent(%s) made its parent", path)
		}
	}
	entries, err := os.ReadDir(stager.Dir + "/fs")
	if err != nil || len(entries) != 0 {
		t.Errorf("the staging tree holds %v (%v), want nothing", entries, err)
	}
	if text, err := os.ReadFile(root + "/home/f"); err != nil || string(text) != "source" {
		t.Errorf("the source reads %q, %v", text, err)
	}
}
