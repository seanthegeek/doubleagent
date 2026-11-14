//go:build unix

package projects

import (
	"context"
	"reflect"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/users"
)

// This builds slash-separated absolute paths, which Windows spells with a drive letter.

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
