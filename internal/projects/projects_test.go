package projects

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/users"
)

// writeTree creates each file under root with its text, and each directory.
func writeTree(t *testing.T, root string, files map[string]string, dirs ...string) {
	t.Helper()
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, text := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestScanLines(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"plain":    "/a\n\n/b",
		"nul late": "/a\n/b\x00\n/c\n",
	}, "dir")
	if err := os.Symlink(filepath.Join(root, "plain"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, file string
		want       []string
		ok         bool
	}{
		{"a last line with no newline counts", "plain", []string{"/a", "", "/b"}, true},
		{"a file holding a NUL byte gives no line", "nul late", nil, false},
		{"a symlink is not followed", "link", nil, false},
		{"a directory is not read", "dir", nil, false},
		{"a missing file gives nothing", "missing", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			ok := scanLines(filepath.Join(root, test.file), func(line []byte) {
				got = append(got, string(line))
			})
			if ok != test.ok || (test.ok && !reflect.DeepEqual(got, test.want)) {
				t.Errorf("got %q, %v, want %q, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestStreamValues(t *testing.T) {
	for _, test := range []struct {
		name, stream string
		want         []string
	}{
		{"an escaped slash is undone after the filter", "/a\\/b\n\\/c\nrel\n", []string{"/a/b"}},
		{"a last line with no newline counts", "/a\n/b", []string{"/a", "/b"}},
		{"an empty stream gives nothing", "", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			scan := &homeScan{}
			scan.stream.WriteString(test.stream)
			if got := scan.values(); !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestDiscoverJoinsACopiedFileToTheNextValue(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"home/alice/.gemini/tmp/a/.project_root":           "/srv/a",
		"home/alice/.gemini/tmp/b/.project_root":           "/srv/b",
		"home/alice/.gemini/antigravity-cli/history.jsonl": `{"workspace":"/c"}`,
	}, "srv/a/srv/b/c")
	homes := []users.Home{{User: "alice", Path: root + "/home/alice"}}
	got, err := Discover(context.Background(), homes, root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// cat copies each project root file whole, and neither ends in a newline.
	if want := []Project{{"alice", root + "/srv/a/srv/b/c"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverInventoryReadsNoSecretFile(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"home/alice/.claude/history.jsonl":          `{"project":"/srv/secret"}`,
		"home/alice/.local/share/amp/history.jsonl": `{"cwd":"/srv/open"}`,
		"home/alice/.gemini/tmp/x/.project_root":    "/srv/catted\n",
	}, "srv/secret", "srv/open", "srv/catted")
	homes := []users.Home{{User: "alice", Path: root + "/home/alice"}}
	dropSecret := func(relative string) bool {
		return relative == ".claude/history.jsonl" || relative == ".gemini/tmp/x/.project_root"
	}
	got, err := Discover(context.Background(), homes, root, nil, dropSecret, nil)
	if err != nil {
		t.Fatal(err)
	}
	// cat reads the project root files directly, so dropSecret never sees them.
	want := []Project{{"alice", root + "/srv/catted"}, {"alice", root + "/srv/open"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverMergesAndFilters(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"home/alice/.claude/history.jsonl": `{"project":"/srv/shared"}` + "\n" +
			`{"project":"/home/bob"}` + "\n" + `{"project":"/srv/file"}` + "\n",
		"home/bob/.claude/history.jsonl": `{"project":"/srv/shared"}` + "\n" +
			`{"project":"/srv/bobs"}` + "\n",
		"srv/file": "",
	}, "srv/shared", "srv/bobs", "srv/given", "home/bob/inside")
	homes := []users.Home{{User: "alice", Path: root + "/home/alice"},
		{User: "bob", Path: root + "/home/bob"}}
	extra := []string{root + "/srv/given", root + "/home/bob/inside", root + "/srv/bobs",
		"relative", root + "/srv/missing"}
	got, err := Discover(context.Background(), homes, root+"/", extra, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []Project{{"alice", root + "/srv/shared"}, {"bob", root + "/srv/bobs"},
		{"", root + "/srv/given"}, {"bob", root + "/home/bob/inside"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestKeepProjectDirsKeepsAPathOutOfScopeUntested(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"srv/file": ""}, "srv/dir")
	outOfScope := func(path string) bool { return strings.HasPrefix(path, root+"/out/") }
	cases := []struct {
		name       string
		outOfScope func(string) bool
		want       []Project
	}{
		{"a path out of scope is kept though no directory is there", outOfScope,
			[]Project{{"", root + "/srv/dir"}, {"", root + "/out/gone"}}},
		{"without a scope test every path is tested", nil, []Project{{"", root + "/srv/dir"}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projects := []Project{{"", root + "/srv/dir"}, {"", root + "/srv/file"},
				{"", root + "/out/gone"}, {"", root + "/srv/missing"}, {"u", root + "/out/home"}}
			got := keepProjectDirs(projects, "u:"+root+"/out/home", testCase.outOfScope)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("got %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestMergeProjectOwnersKeepsFirstNonEmptyOwner(t *testing.T) {
	got := mergeProjectOwners([]Project{{"", "/a"}, {"bob", "/a"}, {"carol", "/a"},
		{"dave", ""}, {"erin", "/b"}})
	want := []Project{{"bob", "/a"}, {"erin", "/b"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProjectOwner(t *testing.T) {
	userList := "alice:/home/alice\nbob:/home/bob\nodd"
	for _, test := range []struct{ path, want string }{
		{"/home/alice/p", "alice"},
		{"/home/alice", ""},
		{"/home/alicex/p", ""},
		{"/home/bob/x/y", "bob"},
		{"odd/x", "odd"},
	} {
		if got := projectOwner(userList, test.path); got != test.want {
			t.Errorf("projectOwner(%q) = %q, want %q", test.path, got, test.want)
		}
	}
}

func TestDiscoverStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	homes := []users.Home{{User: "alice", Path: t.TempDir()}}
	if _, err := Discover(ctx, homes, "", nil, nil, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
