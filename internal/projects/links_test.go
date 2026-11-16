package projects

import (
	"context"
	"github.com/seanthegeek/doubleagent/internal/follow"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

// linkedSource is one reader of discovery: the home directory made a link,
// the source below it, relative to that directory, and the source's text,
// in which {P} stands for the project directory the source names.
type linkedSource struct {
	name, linked, source, text string
}

// linkedSources are the readers, one per kind, each reached through a link
// at the first directory of its path under the home.
func linkedSources() []linkedSource {
	return []linkedSource{
		{"a JSON value read line by line", ".claude", "history.jsonl", `{"project":"{P}"}`},
		{"a JSON key behind the regular-file probe", ".gemini", "projects.json",
			`{"{P}": "x"}`},
		{"a file copied whole", ".gemini", "tmp/a/.project_root", "{P}\n"},
		{"a session file found below a starting directory", ".codex",
			"sessions/2026/s.jsonl", `{"cwd":"{P}"}`},
		{"an editor's workspace storage", ".config",
			"Code/User/workspaceStorage/w/workspace.json", `{"folder": "file://{P}"}`},
		{"the lines of a file behind the regular-file probe", ".nanobot",
			"sessions/s/.workspace", "{P}\n"},
		{"a project list behind a glob", ".local", "share/goose/projects/p.md",
			"workingDirs:\n- {P}\n"},
	}
}

// linkedImage builds an image root holding the project directory srv/p and
// alice's home, whose source.linked is a link to a directory holding the
// source, inside the image when inside is set, else outside it. The source
// names the project as the image holds it, or by its full path when live.
func linkedImage(t *testing.T, source linkedSource, inside, live bool) (root, home string) {
	root = fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	home = root + "/home/alice"
	target := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir())) + "/state"
	if inside {
		target = root + "/elsewhere/state"
	}
	named := "/srv/p"
	if live {
		named = root + "/srv/p"
	}
	text := strings.ReplaceAll(source.text, "{P}", named)
	writeTree(t, root, nil, "srv/p", "home/alice")
	writeTree(t, target, map[string]string{source.source: text})
	linkDirectory(t, target, home+"/"+source.linked)
	return root, home
}

func TestDiscoverReadsNoSourceThroughALinkOutOfTheImage(t *testing.T) {
	modes := []struct {
		name         string
		inside, live bool
	}{
		{"a link out of the image root is not read", false, false},
		{"a link that stays inside the image root is read", true, false},
		{"a live run reads through the link", false, true},
	}
	for _, source := range linkedSources() {
		for _, mode := range modes {
			t.Run(source.name+": "+mode.name, func(t *testing.T) {
				root, home := linkedImage(t, source, mode.inside, mode.live)
				var outOfScope RootExit
				discoverRoot := root
				if mode.live {
					discoverRoot = ""
				} else {
					outOfScope = walk.NewRootCheck(root).Exit
				}
				homes := []users.Home{{User: "alice", Path: home}}
				got, refused, err := Discover(context.Background(), homes, discoverRoot, nil, nil,
					outOfScope, nil)
				if err != nil {
					t.Fatal(err)
				}
				checkLinkedResult(t, got, refused, root, home+"/"+source.linked,
					!mode.inside && !mode.live)
			})
		}
	}
}

// checkLinkedResult checks a run of the linked-source test: a refused run
// finds no project and refuses link alone, once; any other finds the project
// and refuses nothing.
func checkLinkedResult(t *testing.T, got []Project, refused []Refusal, root, link string,
	wantRefused bool,
) {
	t.Helper()
	if !wantRefused {
		want := []Project{{"alice", root + "/srv/p"}}
		if !reflect.DeepEqual(got, want) || len(refused) != 0 {
			t.Errorf("projects %q, refused %+v; want %q and none refused", got, refused, want)
		}
		return
	}
	want := []Refusal{{User: "alice", Refusal: follow.Refusal{Link: link, Kind: follow.LeadsOut}}}
	if len(got) != 0 ||
		!reflect.DeepEqual(refused, want) {
		t.Errorf("projects %q, refused %+v; want none found and %+v", got, refused, want)
	}
}

func TestDiscoverNamesTheLinkThatClimbsAboveTheRoot(t *testing.T) {
	root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	home := root + "/home/alice"
	writeTree(t, root, nil, "home/alice")
	// .claude climbs out of the root: from home/alice, ../../.. is above it.
	linkRelative(t, "../../../outside", home+"/.claude")
	outOfScope := walk.NewRootCheck(root).Exit
	homes := []users.Home{{User: "alice", Path: home}}
	_, refused, err := Discover(context.Background(), homes, root, nil, nil, outOfScope, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(refused) == 0 {
		t.Fatal("no source was refused")
	}
	for _, refusal := range refused {
		if refusal.Link != home+"/.claude" {
			t.Errorf("refusal %+v, want it to name the link %s, not the root", refusal, home+"/.claude")
		}
	}
}

func TestDiscoverRefusesAChainLongerThanTheCheckFollows(t *testing.T) {
	root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	outside := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	home := root + "/home/alice"
	writeTree(t, root, nil, "home/alice", "chain", "srv/p")
	writeTree(t, outside, map[string]string{"history.jsonl": `{"project":"/srv/p"}`})
	// .claude, then 45 links inside the image, the last leading out: more
	// steps than the check takes, though Windows resolves up to 63.
	const chain = 45
	linkDirectory(t, root+"/chain/l0", home+"/.claude")
	for i := 0; i < chain; i++ {
		target := root + "/chain/l" + strconv.Itoa(i+1)
		if i == chain-1 {
			target = outside
		}
		linkDirectory(t, target, root+"/chain/l"+strconv.Itoa(i))
	}
	outOfScope := walk.NewRootCheck(root).Exit
	homes := []users.Home{{User: "alice", Path: home}}
	got, refused, err := Discover(context.Background(), homes, root, nil, nil, outOfScope, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 || len(refused) == 0 {
		t.Errorf("projects %q, refused %+v; want no project and the source refused", got, refused)
	}
}
