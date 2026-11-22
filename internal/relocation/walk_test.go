package relocation

import (
	"io/fs"
	"slices"
	"testing"
)

// fakeEntry is a directory entry with a name and a directory flag only.
type fakeEntry struct {
	name      string
	directory bool
}

// Name returns the entry's name.
func (e fakeEntry) Name() string { return e.name }

// IsDir reports whether the entry is a directory.
func (e fakeEntry) IsDir() bool { return e.directory }

// Type returns the directory bit for a directory and nothing for a file.
func (e fakeEntry) Type() fs.FileMode {
	if e.directory {
		return fs.ModeDir
	}
	return 0
}

// Info has no file to describe and fails.
func (e fakeEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrInvalid }

func TestStackOrder(t *testing.T) {
	file := func(name string) fs.DirEntry { return fakeEntry{name: name} }
	directory := func(name string) fs.DirEntry { return fakeEntry{name: name, directory: true} }
	cases := []struct {
		name    string
		entries []fs.DirEntry
		want    []string
	}{
		{"an empty directory gives nothing", nil, []string{}},
		{"files keep the listing order", []fs.DirEntry{file("b"), file("a")}, []string{"b", "a"}},
		{"directories come after the files, last listed first",
			[]fs.DirEntry{directory("alpha"), directory("beta"), file("a"), file("z")},
			[]string{"a", "z", "beta", "alpha"}},
		{"directories interleaved with files still follow every file",
			[]fs.DirEntry{file("a"), directory("b"), file("c"), directory("d")},
			[]string{"a", "c", "d", "b"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := stackOrder(test.entries); !slices.Equal(got, test.want) {
				t.Errorf("stackOrder = %q, want %q", got, test.want)
			}
		})
	}
}
