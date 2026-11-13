//go:build unix

package projects

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/users"
)

func TestDiscoverSkipsAPipeWithoutBlocking(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"home/alice/.local/share/amp/history.jsonl": `{"cwd":"/srv/amp"}`,
	}, "home/alice/.letta", "srv/amp")
	if err := syscall.Mkfifo(root+"/home/alice/.letta/sessions.jsonl", 0o644); err != nil {
		t.Fatal(err)
	}
	homes := []users.Home{{User: "alice", Path: root + "/home/alice"}}
	got, err := Discover(context.Background(), homes, root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Project{{"alice", root + "/srv/amp"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFindNamedAgreesWithFind(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"s/b/r.jsonl": "", "s/a/.dot.jsonl": "", "s/c.jsonl/r.jsonl": "", "s/x.json": "",
		"s/a/deep/er/z.jsonl": "", "elsewhere/y.jsonl": "",
	}, "s/empty")
	if err := os.Symlink(root+"/elsewhere", root+"/s/linked"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root+"/elsewhere/y.jsonl", root+"/s/l.jsonl"); err != nil {
		t.Fatal(err)
	}
	for _, start := range []string{"s", "s/linked"} {
		t.Run("from "+start, func(t *testing.T) {
			dir := filepath.Join(root, start)
			output, err := exec.Command("/usr/bin/find", dir, "-name", "*.jsonl").Output()
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
				if line != "" {
					want = append(want, line)
				}
			}
			got := findNamed(context.Background(), dir, "*.jsonl")
			if len(got) != len(want) || (len(got) > 0 && !reflect.DeepEqual(got, want)) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}
