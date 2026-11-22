package relocation

import (
	"context"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// The expected orders follow the Windows sweep by hand: pop a directory, list
// it (NTFS lists names in case-insensitive order, Get-ChildItem directories
// before files), add each file at once, push each subdirectory; the
// subdirectory pushed last is popped next.
func TestSweepOrderOnWindows(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"a directory's files come before its subdirectories, the last listed first",
			map[string]string{"b.txt": "", "a.txt": "", "zeta.txt": "", "alpha/x.txt": "",
				"alpha/inner/y.txt": "", "beta/z.txt": ""},
			[]string{"a.txt", "b.txt", "zeta.txt", "beta/z.txt", "alpha/x.txt",
				"alpha/inner/y.txt"}},
		{"the fixture image's sweep order",
			map[string]string{"backup/Library/LaunchDaemons/com.example.swept.plist": "",
				"work/once.sh": "", "work/scripts/deep/env-helper.sh": ""},
			[]string{"work/once.sh", "work/scripts/deep/env-helper.sh",
				"backup/Library/LaunchDaemons/com.example.swept.plist"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := syspath.FromNative(t.TempDir())
			writeTree(t, root, test.files)
			walk := sweepWalk{ctx: context.Background(), pruned: func(string) bool { return false }}
			if err := walk.visit(root); err != nil {
				t.Fatal(err)
			}
			want := make([]string, len(test.want))
			for i, name := range test.want {
				want[i] = root + "/" + name
			}
			if !slices.Equal(walk.files, want) {
				t.Errorf("files %q, want %q", walk.files, want)
			}
		})
	}
}

func TestSweepOrderKeepsPruningAndSizeOnWindows(t *testing.T) {
	root := syspath.FromNative(t.TempDir())
	writeTree(t, root, map[string]string{"a/big": "123456", "a/small": "1", "b/kept": "",
		"pruned/hidden": "", "top": ""})
	walk := sweepWalk{ctx: context.Background(), limit: 2,
		pruned: func(path string) bool { return syspath.Equal(path, root+"/PRUNED") }}
	if err := walk.visit(root); err != nil {
		t.Fatal(err)
	}
	want := []string{root + "/top", root + "/b/kept", root + "/a/small"}
	if !slices.Equal(walk.files, want) || walk.skipped != 1 {
		t.Errorf("files %q, skipped %d; want %q, 1", walk.files, walk.skipped, want)
	}
}
