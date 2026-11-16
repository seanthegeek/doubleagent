//go:build darwin || linux

package stage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

// oldAccess is the access time the outside directories are given, long
// before any run.
var oldAccess = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

// outsideTree makes parent/outside holding User/settings.json, both
// directories with oldAccess as their access time and a later modification
// time, and returns its path.
func outsideTree(t *testing.T, parent string) string {
	t.Helper()
	outside := parent + "/outside"
	if err := os.MkdirAll(outside+"/User", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside+"/User/settings.json", []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A modification time after the access time: macOS, like Linux's
	// relatime, then moves the access time on the next listing.
	for _, dir := range []string{outside + "/User", outside} {
		if err := os.Chtimes(dir, oldAccess, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	return outside
}

// requireUntouched fails when either outside directory's access time moved.
func requireUntouched(t *testing.T, outside, when string) {
	t.Helper()
	for _, dir := range []string{outside, outside + "/User"} {
		meta, err := fsmeta.Lstat(dir)
		if err != nil || meta.Atime != oldAccess.Unix() {
			t.Errorf("%s: %s has access time %d (%v), want %d: something listed it", when, dir,
				meta.Atime, err, oldAccess.Unix())
		}
	}
}

func TestNoStagedLinkTargetIsListed(t *testing.T) {
	for _, linkName := range []string{"cursor", "Cursor"} {
		t.Run("the recreated link spelled "+linkName, func(t *testing.T) {
			parent := foldingStage(t)
			outside := outsideTree(t, parent)
			first, second := twoFoldingStagers(parent)
			link := parent + "/one/home/u/.config/" + linkName
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, link); err != nil {
				t.Fatal(err)
			}
			if err := first.MakeParent(link); err != nil {
				t.Fatal(err)
			}
			if err := symlink(first, link, outside, true); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"Cursor", "cursor"} {
				file := parent + "/two/home/u/.config/" + name + "/User/settings.json"
				makeSource(t, file, false)
				_, dest, _ := second.Dest(file)
				_, _, _ = copyFile(second, file)
				second.RemoveFailedCopy(dest)
				requireUntouched(t, outside, "after staging "+name+"/User/settings.json")
			}
		})
	}
}

func TestAFailureWithNoOtherSpellingKeepsItsOwnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes any directory")
	}
	t.Run("permission denied two components short", func(t *testing.T) {
		parent := foldingStage(t)
		_, stager := twoFoldingStagers(parent)
		if err := os.Mkdir(parent+"/stage/fs/a", 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(parent+"/stage/fs/a", 0o755) })
		source := parent + "/two/a/b/c/x"
		makeSource(t, source, false)
		_, _, err := copyFile(stager, source)
		if IsCaseCollision(err) || !errors.Is(err, fs.ErrPermission) {
			t.Errorf("CopyFile = %v; want permission denied, not a case collision", err)
		}
	})
	t.Run("a staged path too long for the system", func(t *testing.T) {
		parent := foldingStage(t)
		deep := parent + "/" + strings.Repeat(strings.Repeat("d", 50)+"/", 10) + "stage"
		if err := os.MkdirAll(deep+"/fs", 0o700); err != nil {
			t.Fatal(err)
		}
		stager := Stager{Dir: deep, Physical: deep, Root: parent + "/two", FoldsCase: true,
			Places: NewPlaces()}
		source := parent + "/two/" + strings.Repeat(strings.Repeat("s", 50)+"/", 9) + "x"
		makeSource(t, source, false)
		_, _, err := copyFile(stager, source)
		if err == nil || IsCaseCollision(err) {
			t.Errorf("CopyFile = %v; want the system's error for a path too long", err)
		}
	})
}

func TestALinkUnderAnotherEntrysFileIsACaseCollision(t *testing.T) {
	parent := foldingStage(t)
	first, second := twoFoldingStagers(parent)
	makeSource(t, parent+"/one/s", false)
	if _, _, err := copyFile(first, parent+"/one/s"); err != nil {
		t.Fatal(err)
	}
	makeSource(t, parent+"/two/S/y", true)
	_ = second.MakeParent(parent + "/two/S/y")
	if err := symlink(second, parent+"/two/S/y", "t", false); !IsCaseCollision(err) {
		t.Errorf("Symlink of S/y under the staged file s = %v; want the case collision error", err)
	}
}
