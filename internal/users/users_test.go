package users

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
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
		err := os.MkdirAll(filepath.Join(root, dir), 0o755)
		if invalidFileName(err) {
			t.Skipf("this platform does not accept the directory name %q: %v", dir, err)
		}
		if err != nil {
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
			t.Skipf("cannot create a symbolic link here: %v", err)
		}
	}
	return syspath.FromNative(root)
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
		// windowsWant replaces want on Windows when set, where there is no
		// passwd file, the -u list ignores case, Users comes before home and
		// a directory's names come in listing order (NTFS: by upper case).
		windowsWant []string
	}{
		{
			name:   "passwd users and glob users in enumeration order",
			tree:   tree{dirs: []string{"home/alice", "home/bob", "Users/carol", "root", "var/root", "usr/home/dave", "export/home/erin", "srv/zed"}},
			passwd: "zed:x:5:5::/srv/zed:/bin/sh\n",
			want: []string{"zed:/srv/zed", "alice:/home/alice", "bob:/home/bob", "carol:/Users/carol",
				"root:/root", "root:/var/root", "dave:/usr/home/dave", "erin:/export/home/erin"},
			windowsWant: []string{"carol:/Users/carol", "alice:/home/alice", "bob:/home/bob",
				"root:/root", "root:/var/root", "dave:/usr/home/dave", "erin:/export/home/erin"},
		},
		{
			name:        "every system and shared profile home is dropped",
			tree:        tree{dirs: append([]string{"home/alice"}, append(systemHomes, "Users/carol")...)},
			passwd:      systemPasswd + "alice:x:1000:1000::/home/alice:/bin/sh\n",
			want:        []string{"alice:/home/alice", "carol:/Users/carol"},
			windowsWant: []string{"carol:/Users/carol", "alice:/home/alice"},
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
			name:        "passwd lines with fewer than six fields or an empty home are skipped",
			tree:        tree{dirs: []string{"srv/a", "srv/b"}},
			passwd:      "short:x:1:1:/srv/a\nempty:x:2:2:::/bin/sh\n\nok:x:3:3::/srv/b\n",
			want:        []string{"ok:/srv/b"},
			windowsWant: []string{},
		},
		{
			name:        "a repeated home keeps the first user",
			tree:        tree{dirs: []string{"home/alice"}},
			passwd:      "al:x:1:1::/home/alice:/bin/sh\nalice:x:2:2::/home/alice:/bin/sh\n",
			want:        []string{"al:/home/alice"},
			windowsWant: []string{"alice:/home/alice"},
		},
		{
			name:        "names sort bytewise, upper case before lower case, but not on Windows",
			tree:        tree{dirs: []string{"home/b", "home/Zed", "home/alice", "home/_u"}},
			passwd:      "",
			want:        []string{"Zed:/home/Zed", "_u:/home/_u", "alice:/home/alice", "b:/home/b"},
			windowsWant: []string{"alice:/home/alice", "b:/home/b", "Zed:/home/Zed", "_u:/home/_u"},
		},
		{
			name:        "the -u list picks users, case-sensitively",
			tree:        tree{dirs: []string{"home/alice", "home/bob", "home/Carol"}},
			passwd:      "",
			filter:      "bob,carol,Carol",
			want:        []string{"Carol:/home/Carol", "bob:/home/bob"},
			windowsWant: []string{"bob:/home/bob", "Carol:/home/Carol"},
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
			tree: tree{dirs: []string{"home/" + bracketName}},
			want: []string{bracketName + ":/home/" + bracketName},
		},
		{
			name: "a home whose name ends in a newline is read as the lines of its name",
			tree: tree{dirs: []string{"home/nl", "home/nl\n", "home/two\nlines"}},
			want: []string{"nl:/home/nl"},
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
			got := Enumerate(context.Background(), false, root, testCase.filter, nil, nil, nil)
			entries := testCase.want
			if onWindows && testCase.windowsWant != nil {
				entries = testCase.windowsWant
			}
			want := expected(root, entries...)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Enumerate = %v, want %v", got, want)
			}
		})
	}
}

func TestEnumerateRootWithBracket(t *testing.T) {
	root := syspath.FromNative(filepath.Join(t.TempDir(), bracketRoot))
	err := os.MkdirAll(filepath.Join(root, "home/alice"), 0o755)
	if invalidFileName(err) {
		t.Skipf("this platform does not accept the directory name %q: %v", root, err)
	}
	if err != nil {
		t.Fatal(err)
	}
	got := Enumerate(context.Background(), false, root, "", nil, nil, nil)
	if want := expected(root, "alice:/home/alice"); !reflect.DeepEqual(got, want) {
		t.Errorf("Enumerate = %v, want %v", got, want)
	}
}

func TestSkipProfile(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"alice", false},
		{"Public", true},
		{"public", true},
		{"DEFAULT", true},
		{"Default User", true},
		{"all users", true},
		{"Shared", true},
		{"defaultUser0", true},
		{"Default2", false},
		{"UMFD-0", true},
		{"UMFD-12.DOMAIN", true},
		{"dwm-3", true},
		{"TEMP", true},
		{"temp.corp.000", true},
		{"UMFD-", false},
		{"DWM-x", false},
		{"TEMPERATURE", false},
		{"my.TEMP", false},
		{"", false},
		{"\u017fhared", false},
		{"Default U\u017fer", false},
		{"ALL USER\u017f", false},
	}
	for _, testCase := range cases {
		t.Run("name "+testCase.name, func(t *testing.T) {
			if got := skipProfile(testCase.name); got != testCase.want {
				t.Errorf("skipProfile(%q) = %v, want %v", testCase.name, got, testCase.want)
			}
		})
	}
}

func TestUserNamesAndUnmatchedUsers(t *testing.T) {
	exact := func(name, user string) bool { return name == user }
	for _, test := range []struct {
		name    string
		matches func(name, user string) bool
		want    []string
	}{
		{"exact names drop only exact repeats", exact, []string{"bob", "alice", " x", "BOB"}},
		{"folded names drop a repeat in another case", strings.EqualFold, []string{"bob", "alice", " x"}},
	} {
		if got := userNames("bob,,alice,bob, x,BOB", test.matches); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: userNames = %q, want %q", test.name, got, test.want)
		}
	}
	found := []string{"alice", "émile"}
	cases := []struct {
		name    string
		names   []string
		matches func(name, user string) bool
		want    []string
	}{
		{"exact names match only their own spelling", []string{"alice", "ALICE", "ÉMILE"}, exact,
			[]string{"ALICE", "ÉMILE"}},
		{"Unicode folding matches every letter's case", []string{"ALICE", "ÉMILE", "nobody"},
			strings.EqualFold, []string{"nobody"}},
		{"no names leave nothing unmatched", nil, exact, nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := unmatchedUsers(test.names, found, test.matches); !reflect.DeepEqual(got, test.want) {
				t.Errorf("unmatchedUsers(%q) = %q, want %q", test.names, got, test.want)
			}
		})
	}
}
