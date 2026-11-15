package projects

import (
	"context"
	"reflect"
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
					outOfScope = func(path string) (string, bool) { return walk.LeavesRootAt(root, path) }
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
// finds no project and refuses at least one source under link, naming link;
// any other finds the project and refuses nothing.
func checkLinkedResult(t *testing.T, got []Project, refused []Refusal, root, link string,
	wantRefused bool,
) {
	t.Helper()
	if !wantRefused {
		want := []Project{{"alice", root + "/srv/p"}}
		if !reflect.DeepEqual(got, want) || len(refused) != 0 {
			t.Errorf("projects %q, refused %q; want %q and none refused", got, refused, want)
		}
		return
	}
	if len(got) != 0 || len(refused) == 0 {
		t.Fatalf("projects %q, refused %q; want none found and a source refused", got, refused)
	}
	for _, refusal := range refused {
		under := refusal.Source == link || strings.HasPrefix(refusal.Source, link+"/")
		if refusal.User != "alice" || refusal.Link != link || !under {
			t.Errorf("refusal %+v, want alice's source under %s, refused at it", refusal, link)
		}
	}
}
