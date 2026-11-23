//go:build unix

package parserkit

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// unlistable takes away the modes of a directory that let it be listed,
// keeping mode, and skips the test when it can still be listed, as it can
// by the superuser; the mode is given back when the test ends.
func unlistable(t *testing.T, directory string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(directory, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
	if _, err := os.ReadDir(directory); err == nil {
		t.Skip("a directory without read permission can still be listed by this user")
	}
}

func TestChildListingsNeverFollowALink(t *testing.T) {
	parent := listedParent(t)
	target := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "session.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(parent.Path(), "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(parent.Path(), "a", "session.json"),
		filepath.Join(parent.Path(), "b", "linked.json")); err != nil {
		t.Fatal(err)
	}
	children := ListChildren(parent, []string{"linked", "b"}, nil, Work{}, "listings")
	if got := children.Holders("session.json"); !slices.Equal(got, []int{0}) {
		t.Errorf("session.json is in %v, want only b's place 0", got)
	}
	if got := children.Holders("linked.json"); got != nil {
		t.Errorf("a link to a regular file was taken for one, in %v", got)
	}
}

// A directory that cannot be listed, whether or not it can be searched, is
// remembered as failed: it holds no name, its failure is kept once, and it
// is never listed or looked in again.
func TestADirectoryThatCannotBeListedIsRememberedAsFailed(t *testing.T) {
	for _, mode := range []os.FileMode{0o300, 0o000} {
		t.Run(fmt.Sprintf("mode %03o", mode), func(t *testing.T) {
			parent := listedParent(t)
			unlistable(t, filepath.Join(parent.Path(), "a"), mode)
			work := CountingWork()
			children := ListChildren(parent, []string{"a", "b"}, nil, work, "listings")
			failures := children.Failures()
			if got := children.Holders("session.json"); !slices.Equal(got, []int{1}) {
				t.Errorf("session.json is in %v, want only b's place 1", got)
			}
			if len(failures) != 1 || failures[0].Place != 0 || filepath.Base(failures[0].Path) != "a" ||
				failures[0].Problem().Message != "a could not be read (permission denied)" {
				t.Errorf("failures %+v, want a at place 0, permission denied", failures)
			}
			if listings := work.Count("listings"); listings != 2 {
				t.Errorf("%d listings, want 2", listings)
			}
		})
	}
}

// A directory whose own entry cannot be looked at, since its parent cannot
// be searched, fails as its parent.
func TestADirectoryInAParentThatCannotBeSearchedFailsAsTheParent(t *testing.T) {
	parent := listedParent(t)
	if err := os.Chmod(parent.Path(), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent.Path(), 0o700) })
	if _, err := os.Lstat(filepath.Join(parent.Path(), "a")); err == nil {
		t.Skip("a directory without search permission can still be searched by this user")
	}
	children := ListChildren(parent, []string{"a", "b"}, nil, Work{}, "listings")
	failures := children.Failures()
	if len(failures) != 2 || failures[0].Path != parent.Path() || failures[1].Place != 1 ||
		children.Holders("session.json") != nil {
		t.Errorf("failures %+v and holders %v, want both at the parent and none", failures,
			children.Holders("session.json"))
	}
}
