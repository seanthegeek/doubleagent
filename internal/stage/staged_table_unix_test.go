//go:build darwin || linux

package stage

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNoStagedLinkTargetIsListedCounted(t *testing.T) {
	parent := foldingStage(t)
	outside := outsideTree(t, parent)
	first, second := twoFoldingStagers(parent)
	link := parent + "/one/home/u/.config/cursor"
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
	var listed []string
	system := systemFS()
	second.stagedFS = &stagedFS{lstat: system.lstat, list: func(dir string) ([]fs.DirEntry, error) {
		listed = append(listed, dir)
		return system.list(dir)
	}}
	file := parent + "/two/home/u/.config/Cursor/User/settings.json"
	makeSource(t, file, false)
	_, dest, _ := second.Dest(file)
	if _, _, err := copyFile(second, file); !IsCaseCollision(err) {
		t.Errorf("Copy = %v; want the case collision error: the place is cursor's link", err)
	}
	second.FailedCopyInside(dest)
	staged := parent + "/stage/fs/home/u/.config/cursor"
	if slices.ContainsFunc(listed, func(dir string) bool { return strings.HasPrefix(dir, staged) }) {
		t.Errorf("listed %q, at or below the recreated link %s", listed, staged)
	}
	requireUntouched(t, outside, "after staging Cursor/User/settings.json")
}
