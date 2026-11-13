package users

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// tree describes a fixture image: directories, files and symlinks (name to
// target), all relative to the image root.
type tree struct {
	dirs  []string
	files []string
	links map[string]string
}

func (spec tree) build(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range spec.dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range spec.files {
		path := filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range spec.links {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// expected turns "user:/relative/home" entries into Homes under root.
func expected(root string, entries ...string) []Home {
	var homes []Home
	for _, entry := range entries {
		user, home, _ := strings.Cut(entry, ":")
		homes = append(homes, Home{user, root + home})
	}
	return homes
}

func TestEnumerateImage(t *testing.T) {
	systemHomes := []string{"bin", "sbin", "usr", "usr/bin", "usr/sbin", "dev", "dev/null",
		"proc", "sys", "nonexistent", "var/empty", "Users/Shared", "Users/Public",
		"Users/Default", "Users/All Users", "Users/Default User"}
	systemPasswd := "sys:x:1:1::/:/bin/false\n"
	for _, home := range systemHomes {
		systemPasswd += "svc:x:2:2::/" + home + ":/bin/false\n"
	}
	cases := []struct {
		name   string
		tree   tree
		passwd string
		filter string
		want   []string
	}{
		{
			name:   "passwd users and glob users in enumeration order",
			tree:   tree{dirs: []string{"home/alice", "home/bob", "Users/carol", "root", "var/root", "usr/home/dave", "export/home/erin", "srv/zed"}},
			passwd: "zed:x:5:5::/srv/zed:/bin/sh\n",
			want: []string{"zed:/srv/zed", "alice:/home/alice", "bob:/home/bob", "carol:/Users/carol",
				"root:/root", "root:/var/root", "dave:/usr/home/dave", "erin:/export/home/erin"},
		},
		{
			name:   "every system and shared profile home is dropped",
			tree:   tree{dirs: append([]string{"home/alice"}, append(systemHomes, "Users/carol")...)},
			passwd: systemPasswd + "alice:x:1000:1000::/home/alice:/bin/sh\n",
			want:   []string{"alice:/home/alice", "carol:/Users/carol"},
		},
		{
			name:   "a home that is missing or a file is dropped",
			tree:   tree{dirs: []string{"home/alice"}, files: []string{"home/afile", "srv/file"}},
			passwd: "ghost:x:1:1::/srv/ghost:/bin/sh\nfile:x:2:2::/srv/file:/bin/sh\n",
			want:   []string{"alice:/home/alice"},
		},
		{
			name:   "a link to a directory is a directory; a hidden name is not a home",
			tree:   tree{dirs: []string{"data/real", "home/.hidden"}, links: map[string]string{"home/linked": "../data/real"}},
			passwd: "",
			want:   []string{"linked:/home/linked"},
		},
		{
			name:   "passwd lines with fewer than six fields or an empty home are skipped",
			tree:   tree{dirs: []string{"srv/a", "srv/b"}},
			passwd: "short:x:1:1:/srv/a\nempty:x:2:2:::/bin/sh\n\nok:x:3:3::/srv/b\n",
			want:   []string{"ok:/srv/b"},
		},
		{
			name:   "a repeated home keeps the first user",
			tree:   tree{dirs: []string{"home/alice"}},
			passwd: "al:x:1:1::/home/alice:/bin/sh\nalice:x:2:2::/home/alice:/bin/sh\n",
			want:   []string{"al:/home/alice"},
		},
		{
			name:   "names sort bytewise, upper case before lower case",
			tree:   tree{dirs: []string{"home/b", "home/Zed", "home/alice", "home/_u"}},
			passwd: "",
			want:   []string{"Zed:/home/Zed", "_u:/home/_u", "alice:/home/alice", "b:/home/b"},
		},
		{
			name:   "the -u list picks users, case-sensitively",
			tree:   tree{dirs: []string{"home/alice", "home/bob", "home/Carol"}},
			passwd: "",
			filter: "bob,carol,Carol",
			want:   []string{"Carol:/home/Carol", "bob:/home/bob"},
		},
		{
			name:   "a -u list naming nobody keeps nobody",
			tree:   tree{dirs: []string{"home/alice"}},
			filter: ",",
		},
		{
			name:   "a user excluded by -u does not claim the home",
			tree:   tree{dirs: []string{"home/alice"}},
			passwd: "al:x:1:1::/home/alice:/bin/sh\n",
			filter: "alice",
			want:   []string{"alice:/home/alice"},
		},
		{
			name: "a root whose name holds a bracket",
			tree: tree{dirs: []string{"home/[x]*"}},
			want: []string{"[x]*:/home/[x]*"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := testCase.tree.build(t)
			if testCase.passwd != "" {
				if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "etc/passwd"), []byte(testCase.passwd), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := Enumerate(false, root, testCase.filter, nil)
			want := expected(root, testCase.want...)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Enumerate = %v, want %v", got, want)
			}
		})
	}
}

func TestEnumerateRootWithBracket(t *testing.T) {
	root := filepath.Join(t.TempDir(), "r[o]*ot")
	if err := os.MkdirAll(filepath.Join(root, "home/alice"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := Enumerate(false, root, "", nil)
	if want := expected(root, "alice:/home/alice"); !reflect.DeepEqual(got, want) {
		t.Errorf("Enumerate = %v, want %v", got, want)
	}
}

func TestEnumerateLive(t *testing.T) {
	root := tree{dirs: []string{"srv/getent", "srv/dscl", "srv/file", "home/alice"}}.build(t)
	if err := os.MkdirAll(filepath.Join(root, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc/passwd"), []byte("file:x:1:1::/srv/file:/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	getent := func(name string, args ...string) ([]byte, error) {
		switch name {
		case "getent":
			return []byte("viagetent:x:1:1::/srv/getent:/bin/sh\n"), nil
		case "dscl":
			return []byte("viadscl  " + root + "/srv/dscl\nlonely\n"), nil
		}
		return nil, errors.New("unexpected " + name)
	}
	failing := func(string, ...string) ([]byte, error) { return nil, errors.New("not found") }
	cases := []struct {
		name string
		run  Runner
		want []Home
	}{
		{"getent replaces the passwd file, dscl adds, globs follow", getent, []Home{
			{"viagetent", root + "/srv/getent"}, {"viadscl", root + "/srv/dscl"},
			{"alice", root + "/home/alice"}}},
		{"programs that fail fall back to the passwd file", failing, []Home{
			{"file", root + "/srv/file"}, {"alice", root + "/home/alice"}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// The fixture root stands in for a live run's empty root: getent's
			// homes are prefixed with it, dscl's are not.
			if got := Enumerate(true, root, "", testCase.run); !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("Enumerate = %v, want %v", got, testCase.want)
			}
		})
	}
}
