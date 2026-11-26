package projects

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/follow"

	"github.com/seanthegeek/doubleagent/internal/syspath"
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
	}{
		{"a last line with no newline counts", "plain", []string{"/a", "", "/b"}},
		{"a NUL byte keeps every line", "nul late", []string{"/a", "/b\x00", "/c"}},
		{"a symlink is not followed", "link", nil},
		{"a directory is not read", "dir", nil},
		{"a missing file gives nothing", "missing", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			(&homeScan{}).scanLines(filepath.Join(root, test.file), func(line []byte) {
				got = append(got, string(line))
			})
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestStreamPaths(t *testing.T) {
	for _, test := range []struct {
		name, root, stream string
		want               []string
	}{
		{"an escaped slash is undone after the filter", "", "/a\\/b\n\\/c\nrel\n", []string{"/a/b"}},
		{"a last line with no newline counts", "", "/a\n/b", []string{"/a", "/b"}},
		{"the root is put before each path", "/img", "/a\n", []string{"/img/a"}},
		{"an empty stream gives nothing", "", "", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			scan := &homeScan{root: test.root, rules: pathRules{}}
			scan.stream.WriteString(test.stream)
			if got := scan.paths(); !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestDiscoverKeepsACopiedFileApartFromTheNextValue(t *testing.T) {
	root := syspath.FromNative(t.TempDir())
	writeTree(t, root, map[string]string{
		"home/alice/.gemini/tmp/a/.project_root":           "/srv/a",
		"home/alice/.gemini/tmp/c/.project_root":           "/srv/c\n/srv/d",
		"home/alice/.gemini/history/b/.project_root":       "/srv/b\r\n",
		"home/alice/.gemini/antigravity-cli/history.jsonl": `{"workspace":"/srv/e"}`,
	}, "srv/a", "srv/b", "srv/c", "srv/d", "srv/e", "srv/a/srv/c")
	homes := []users.Home{{User: "alice", Path: root + "/home/alice"}}
	got, _, err := Discover(context.Background(), homes, root, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Each line of a copied file is a value of its own, ending at LF, CRLF
	// or the end of the file, so no file runs into the next value.
	var want []Project
	for _, name := range []string{"a", "c", "d", "b", "e"} {
		want = append(want, Project{"alice", root + "/srv/" + name})
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverInventoryReadsNoSecretFile(t *testing.T) {
	root := syspath.FromNative(t.TempDir())
	writeTree(t, root, map[string]string{
		"home/alice/.claude/history.jsonl":          `{"project":"/srv/secret"}`,
		"home/alice/.local/share/amp/history.jsonl": `{"cwd":"/srv/open"}`,
		"home/alice/.gemini/tmp/x/.project_root":    "/srv/catted\n",
	}, "srv/secret", "srv/open", "srv/catted")
	homes := []users.Home{{User: "alice", Path: root + "/home/alice"}}
	dropSecret := func(_, relative string) bool {
		return relative == ".claude/history.jsonl" || relative == ".gemini/tmp/x/.project_root"
	}
	got, _, err := Discover(context.Background(), homes, root, nil, dropSecret, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// cat reads the project root files directly, so dropSecret never sees them.
	want := []Project{{"alice", root + "/srv/catted"}, {"alice", root + "/srv/open"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverReadsNoneOfTheRunsOwnOutputs(t *testing.T) {
	root := syspath.FromNative(t.TempDir())
	writeTree(t, root, map[string]string{
		"home/alice/.codex/sessions/a.jsonl":                      `{"cwd":"/srv/a"}`,
		"home/alice/.codex/sessions/r.manifest.jsonl":             `{"cwd":"/srv/manifest"}`,
		"home/alice/.codex/sessions/.stage-r/fs/sessions/b.jsonl": `{"cwd":"/srv/staged"}`,
		"home/alice/.qwen/projects/p/chats/c.jsonl":               `{"cwd":"/srv/c"}`,
		"home/alice/.qwen/projects/p/chats/r.log.jsonl":           `{"cwd":"/srv/log"}`,
	}, "srv/a", "srv/manifest", "srv/staged", "srv/c", "srv/log")
	homes := []users.Home{{User: "alice", Path: root + "/home/alice"}}
	runOutput := func(path string) bool {
		return strings.Contains(path, "/.stage-r") || strings.Contains(path, "/r.")
	}
	got, _, err := Discover(context.Background(), homes, root, nil, nil, nil, nil, nil, runOutput)
	if err != nil {
		t.Fatal(err)
	}
	want := []Project{{"alice", root + "/srv/a"}, {"alice", root + "/srv/c"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want only the projects outside the run's outputs, %q", got, want)
	}
}

func TestDiscoverMergesAndFilters(t *testing.T) {
	root := syspath.FromNative(t.TempDir())
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
	got, _, err := Discover(context.Background(), homes, root+"/", extra, nil, nil, nil, nil, nil)
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
	outOfScope := func(path string) (follow.Refusal, bool) {
		return follow.Refusal{Link: root + "/out", Kind: follow.LeadsOut},
			strings.HasPrefix(path, root+"/out/")
	}
	cases := []struct {
		name       string
		outOfScope RootExit
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
			homes := []users.Home{{User: "u", Path: root + "/out/home"}}
			got := keepProjectDirs(projects, homes, testCase.outOfScope, nil)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("got %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestMergeProjectOwnersKeepsFirstNonEmptyOwner(t *testing.T) {
	got := mergeProjectOwners([]Project{{"", "/a"}, {"bob", "/a"}, {"carol", "/a"},
		{"dave", ""}, {"erin", "/b"}}, nil)
	want := []Project{{"bob", "/a"}, {"erin", "/b"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProjectOwner(t *testing.T) {
	homes := []users.Home{{User: "alice", Path: "/home/alice"}, {User: "bob", Path: "/home/bob"},
		{User: "nl\ncarol:/home/dave", Path: "/home/nl\ncarol:/home/dave"}}
	for _, test := range []struct{ path, want string }{
		{"/home/alice/p", "alice"},
		{"/home/alice", ""},
		{"/home/alicex/p", ""},
		{"/home/bob/x/y", "bob"},
		{"/home/nl\ncarol:/home/dave/p", "nl\ncarol:/home/dave"},
		{"/home/dave/p", ""},
		{"carol:/home/dave/p", ""},
	} {
		if got := projectOwner(homes, test.path); got != test.want {
			t.Errorf("projectOwner(%q) = %q, want %q", test.path, got, test.want)
		}
	}
}

func TestDiscoverStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	homes := []users.Home{{User: "alice", Path: t.TempDir()}}
	if _, _, err := Discover(ctx, homes, "", nil, nil, nil, nil, nil, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
