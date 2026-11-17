package stage

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

// fakeNode is one entry of a fake staging tree: its type and inode.
type fakeNode struct {
	mode fs.FileMode
	ino  uint64
}

// fakeEntry is a fs.DirEntry of a fake tree.
type fakeEntry struct {
	name string
	mode fs.FileMode
}

func (entry fakeEntry) Name() string               { return entry.name }
func (entry fakeEntry) IsDir() bool                { return entry.mode.IsDir() }
func (entry fakeEntry) Type() fs.FileMode          { return entry.mode.Type() }
func (entry fakeEntry) Info() (fs.FileInfo, error) { return nil, errors.ErrUnsupported }

// fakeTree is a staging tree whose volume folds case and Unicode
// normalisation the way fold says, recording every directory listed.
type fakeTree struct {
	nodes      map[string]fakeNode
	unlistable map[string]bool
	listed     []string
}

// fold is the fake volume's comparison key: ASCII case ignored and the
// decomposed é taken for the composed one, as APFS takes them.
func fold(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "é", "é"))
}

// files returns the fake file system walkStaged asks.
func (tree *fakeTree) files() stagedFS {
	return stagedFS{
		list: func(dir string) ([]fs.DirEntry, error) {
			tree.listed = append(tree.listed, dir)
			if tree.unlistable[dir] {
				return nil, fs.ErrPermission
			}
			var entries []fs.DirEntry
			for path, node := range tree.nodes {
				if parent, name, _ := cutLast(path); parent == dir {
					entries = append(entries, fakeEntry{name, node.mode})
				}
			}
			return entries, nil
		},
		lstat: func(path string) (fsmeta.Meta, error) {
			parent, name, _ := cutLast(path)
			for stored, node := range tree.nodes {
				if storedParent, storedName, _ := cutLast(stored); storedParent == parent &&
					fold(storedName) == fold(name) {
					return fsmeta.Meta{Dev: 1, Ino: node.ino}, nil
				}
			}
			return fsmeta.Meta{}, fs.ErrNotExist
		},
	}
}

// cutLast splits path at its last slash.
func cutLast(path string) (string, string, bool) {
	position := strings.LastIndexByte(path, '/')
	return path[:max(position, 0)], path[position+1:], position >= 0
}

func TestWalkStagedFollowsTheTreeAndNoLink(t *testing.T) {
	directory, file, link := fs.ModeDir, fs.FileMode(0), fs.ModeSymlink
	tree := map[string]fs.FileMode{"/s/fs": directory, "/s/fs/a": directory,
		"/s/fs/a/b": directory, "/s/fs/a/b/f": file, "/s/fs/A2": directory, "/s/fs/A2/b": directory,
		"/s/fs/a/F": file, "/s/fs/a/L": link, "/s/fs/a/D": directory, "/s/fs/a/café": file,
		"/s/fs/locked": directory}
	cases := []struct {
		name, path, spelled string
		respelled           bool
		stop                stagedStop
		stopRespelled       bool
	}{
		{"every component exact", "/s/fs/a/b/f", "/s/fs/a/b/f", false, stagedEnd, false},
		{"a directory respelled", "/s/fs/a2/b", "/s/fs/A2/b", true, stagedEnd, false},
		{"the last respelled to a file", "/s/fs/a/f", "/s/fs/a/F", true, stagedEnd, true},
		{"the last respelled to a link", "/s/fs/a/l", "/s/fs/a/L", true, stagedEnd, true},
		{"the last respelled to a directory", "/s/fs/a/d", "/s/fs/a/D", true, stagedEnd, true},
		{"a link in the middle, exact", "/s/fs/a/L/x", "/s/fs/a/L", false, stagedNotDirectory, false},
		{"a link in the middle, respelled", "/s/fs/a/l/x", "/s/fs/a/L", true, stagedNotDirectory,
			true},
		{"a file in the middle", "/s/fs/a/F/x", "/s/fs/a/F", false, stagedNotDirectory, false},
		{"the first component missing", "/s/fs/q/r/x", "/s/fs", false, stagedMissing, false},
		{"a middle component missing", "/s/fs/a/q/x", "/s/fs/a", false, stagedMissing, false},
		{"the last component missing", "/s/fs/a/q", "/s/fs/a", false, stagedMissing, false},
		{"an unlistable directory", "/s/fs/locked/x", "/s/fs/locked", false, stagedFailed, false},
		{"a decomposed name the volume stores composed", "/s/fs/a/café",
			"/s/fs/a/café", true, stagedEnd, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fake := &fakeTree{nodes: map[string]fakeNode{"/s": {fs.ModeDir, 1}},
				unlistable: map[string]bool{"/s/fs/locked": true}}
			var inode uint64 = 2
			for path, mode := range tree {
				fake.nodes[path] = fakeNode{mode, inode}
				inode++
			}
			got := walkStaged(fake.files(), "/s", testCase.path)
			want := stagedSpelling{testCase.spelled, testCase.respelled, testCase.stop,
				testCase.stopRespelled}
			if got != want {
				t.Errorf("walkStaged(%s) = %+v, want %+v", testCase.path, got, want)
			}
			for _, listed := range fake.listed {
				if node, found := fake.nodes[listed]; !found || node.mode != fs.ModeDir {
					t.Errorf("listed %s, which is no real directory of the tree", listed)
				}
			}
		})
	}
}

func TestMemberIsTheStagedPathWhenTheWalkStopsShort(t *testing.T) {
	fake := &fakeTree{nodes: map[string]fakeNode{"/s": {fs.ModeDir, 1}, "/s/fs": {fs.ModeDir, 2},
		"/s/fs/x": {0, 3}}, unlistable: map[string]bool{"/s/fs": true}}
	files := fake.files()
	stager := Stager{Dir: "/s", FoldsCase: true, stagedFS: &files}
	if got := stager.member("", "/s/fs/x"); got != "fs/x" {
		t.Errorf("member = %q; want fs/x, the staged path as named, when the walk stops short", got)
	}
}
