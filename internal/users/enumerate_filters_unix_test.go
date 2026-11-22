//go:build unix

package users

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestEnumerateFilterOverMixedImage lists the homes of one image that holds
// passwd users, scanned directories, a shared profile, a home that is a file,
// a missing home, an empty field and a short line, for each -u list.
func TestEnumerateFilterOverMixedImage(t *testing.T) {
	t.Parallel()
	spec := tree{
		dirs: []string{"home/alice", "home/bob", "home/.hidden", "Users/carol", "Users/Shared",
			"root", "var/root", "usr/home/dave", "srv/zed", "srv/dup", "usr", "var/empty"},
		files: []string{"home/notes", "srv/file"},
	}
	passwd := "zed:x:5:5::/srv/zed:/bin/sh\nusr:x:6:6::/usr:/bin/false\n" +
		"first:x:7:7::/srv/dup:/bin/sh\nsecond:x:8:8::/srv/dup:/bin/sh\n" +
		"file:x:9:9::/srv/file:/bin/sh\nghost:x:10:10::/srv/ghost:/bin/sh\n" +
		"empty:x:11:11:::/bin/sh\nshort:x:12:12:/srv/zed\n"
	cases := []struct {
		name   string
		filter string
		want   []string
	}{
		{"no list keeps every home in enumeration order", "", []string{
			"zed:/srv/zed", "first:/srv/dup", "alice:/home/alice", "bob:/home/bob",
			"carol:/Users/carol", "root:/root", "root:/var/root", "dave:/usr/home/dave"}},
		{"a list names users, and a name differing in case matches nothing",
			"bob,zed,Carol", []string{"zed:/srv/zed", "bob:/home/bob"}},
		{"a name from passwd and a name from a scan share one list",
			"root,second", []string{"second:/srv/dup", "root:/root", "root:/var/root"}},
		{"a list of only a comma names nobody", ",", nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := spec.build(t)
			if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "etc/passwd"), []byte(passwd), 0o644); err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, home := range Enumerate(context.Background(), false, root, test.filter, nil, nil, nil, nil) {
				got = append(got, home.User+":"+strings.TrimPrefix(home.Path, root))
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("Enumerate with %q = %q, want %q", test.filter, got, test.want)
			}
		})
	}
}
