package fsmeta

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// bigFileSize is the size of the file the usage tests place where the walk
// must not reach, far above what everything else in their trees adds up to.
const bigFileSize = 1 << 20

// usageTree makes a tree under a test directory holding a small file and
// the directories named in directories, and a directory outside it holding
// a file of bigFileSize bytes; it returns both paths in canonical form.
func usageTree(t *testing.T, directories ...string) (tree, outside string) {
	t.Helper()
	root := filepath.ToSlash(t.TempDir())
	tree, outside = root+"/tree", root+"/outside"
	for _, directory := range append([]string{outside}, directories...) {
		if !strings.HasPrefix(directory, outside) {
			directory = tree + "/" + directory
		}
		if err := os.MkdirAll(filepath.FromSlash(directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeUsageFile(t, tree+"/small", 10)
	writeUsageFile(t, outside+"/big", bigFileSize)
	return tree, outside
}

// writeUsageFile writes size bytes of text to path, so no file system
// stores it sparsely.
func writeUsageFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(filepath.FromSlash(path), []byte(strings.Repeat("x", size)),
		0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiskUsageBytesListsNoDirectorySwappedForALink(t *testing.T) {
	tree, outside := usageTree(t, "swapped")
	swapped := tree + "/swapped"
	walk := usageWalk{beforeOpen: func(path string) {
		if path != swapped {
			return
		}
		if err := os.Rename(filepath.FromSlash(swapped),
			filepath.FromSlash(tree+"-moved")); err != nil {
			t.Fatal(err)
		}
		replaceWithLink(t, outside, swapped)
	}}
	got, err := walk.size(context.Background(), tree)
	if err != nil || got >= bigFileSize {
		t.Errorf("size = %d, %v; want under %d: a directory swapped for a link to "+
			"another directory after its lstat was listed through the link", got, err,
			bigFileSize)
	}
}

func TestDiskUsageBytesListsNoDirectoryTheOpenerRefuses(t *testing.T) {
	tree, _ := usageTree(t, "refused")
	refused := tree + "/refused"
	writeUsageFile(t, refused+"/big", bigFileSize)
	refusal := errors.New("refused")
	asked := false
	opener := &Opener{Admit: func(path string, _ uintptr) error {
		if path != refused {
			return nil
		}
		asked = true
		return refusal
	}}
	got, err := DiskUsageBytes(context.Background(), opener, tree)
	if err != nil || got >= bigFileSize || !asked {
		t.Errorf("DiskUsageBytes = %d, %v, Admit asked about %s: %v; want under %d, nil, "+
			"asked", got, err, refused, asked, bigFileSize)
	}
}

func TestDiskUsageBytesSumsTheSizesOfTheFilesBelow(t *testing.T) {
	tree, outside := usageTree(t, "a/b", "c")
	writeUsageFile(t, tree+"/a/one", 1)
	writeUsageFile(t, tree+"/a/b/two", 5000)
	writeUsageFile(t, tree+"/c/three", 70000)
	writeUsageFile(t, tree+"/empty", 0)
	if err := os.Link(filepath.FromSlash(tree+"/c/three"),
		filepath.FromSlash(tree+"/a/again")); err != nil {
		t.Fatal(err)
	}
	replaceWithLink(t, outside, tree+"/link")
	cases := []struct {
		name, path string
		want       int64
	}{
		{"each path is counted once and no link is followed", tree, 10 + 1 + 5000 + 70000 + 70000},
		{"a link itself counts nothing", tree + "/link", 0},
		{"a file counts its size", tree + "/c/three", 70000},
		{"a missing path counts nothing", tree + "/missing", 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := DiskUsageBytes(context.Background(), nil, testCase.path)
			if err != nil || got != testCase.want {
				t.Errorf("DiskUsageBytes = %d, %v; want %d", got, err, testCase.want)
			}
		})
	}
}

func TestDiskUsageBytesStopsWhenCancelled(t *testing.T) {
	tree := t.TempDir()
	for i := range 300 {
		path := filepath.Join(tree, strconv.Itoa(i%10), strconv.Itoa(i))
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	got, err := DiskUsageBytes(ctx, nil, filepath.ToSlash(tree))
	if !errors.Is(err, context.Canceled) || got != 0 {
		t.Errorf("DiskUsageBytes = %d, %v; want 0, context.Canceled", got, err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("DiskUsageBytes took %v after the context was cancelled", elapsed)
	}
}
