//go:build unix

package walk

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
)

// projectFixture builds a home whose .claude links to dotfiles/claude, by a
// relative or an absolute target, holding a project with an AGENTS.md and a
// .git that links to a directory inside the project. It returns the root
// and the home.
func projectFixture(t *testing.T, absolute bool) (root, home string) {
	t.Helper()
	root = fixture{files: []string{"home/dot/dotfiles/claude/proj/AGENTS.md",
		"home/dot/dotfiles/claude/proj/realgit/HEAD"},
		links: [][2]string{{"realgit", "home/dot/dotfiles/claude/proj/.git"}}}.build(t)
	home = root + "/home/dot"
	target := "dotfiles/claude"
	if absolute {
		target = home + "/dotfiles/claude"
	}
	mustDo(t, os.Symlink(target, home+"/.claude"))
	return root, home
}

func TestProjectThroughALinkInsideItsHome(t *testing.T) {
	for _, absolute := range []bool{false, true} {
		name := "a project under a link with a relative target is staged where the home holds it"
		if absolute {
			name = "a project under a link with an absolute target is staged where the home holds it"
		}
		t.Run(name, func(t *testing.T) {
			root, home := projectFixture(t, absolute)
			collector, rows, log := testCollector(t, root, nil)
			collector.MaxSize = 1 << 20
			collector.Tables = &catalog.Tables{ProjectCatalog: []catalog.Entry{
				{Agent: "project", Glob: "AGENTS.md"}, {Agent: "project", Glob: ".git/HEAD"}}}
			project := home + "/.claude/proj"
			mustDo(t, collector.CollectProject(context.Background(), "dot", home, project))
			physical := "fs/home/dot/dotfiles/claude/proj"
			want := map[string][]string{
				"path": {home + "/.claude", project + "/AGENTS.md", project + "/.git",
					project + "/.git/HEAD"},
				"archive_path": {"fs/home/dot/.claude", physical + "/AGENTS.md", physical + "/.git",
					physical + "/realgit/HEAD"},
				"agent":  {"shared", "project", "project", "project"},
				"status": {"symlink", "collected", "symlink", "collected"},
			}
			for key, values := range want {
				if got := field(rows, key); !reflect.DeepEqual(got, values) {
					t.Errorf("%s %q, want %q", key, got, values)
				}
			}
			header := "Project [dot] " + project + " (through the symlinked directory " + home +
				"/.claude, archived under " + home + "/dotfiles/claude/proj)"
			if len(log.lines) == 0 || log.lines[0] != header {
				t.Errorf("log %q", log.lines)
			}
			for _, name := range []string{"AGENTS.md", "realgit/HEAD"} {
				staged, err := os.ReadFile(collector.Stager.Dir + "/" + physical + "/" + name)
				source, sourceErr := os.ReadFile(home + "/dotfiles/claude/proj/" + name)
				if err != nil || sourceErr != nil || string(staged) != string(source) ||
					!strings.HasSuffix(string(source), "proj/"+name) {
					t.Errorf("%s staged %q (%v), source %q (%v)", name, staged, err, source, sourceErr)
				}
			}
		})
	}
}

func TestProjectThroughALinkOutOfItsHome(t *testing.T) {
	root := fixture{files: []string{"outside/pi/proj/AGENTS.md"}, dirs: []string{"home/dot"},
		links: [][2]string{{"../../outside/pi", "home/dot/.pi"}}}.build(t)
	home := root + "/home/dot"
	collector, rows, log := testCollector(t, root, nil)
	collector.Tables = &catalog.Tables{ProjectCatalog: []catalog.Entry{{Agent: "project",
		Glob: "AGENTS.md"}}}
	project := home + "/.pi/proj"
	mustDo(t, collector.CollectProject(context.Background(), "dot", home, project))
	if got := field(rows, "path"); !reflect.DeepEqual(got, []string{home + "/.pi"}) {
		t.Errorf("rows for %q, want the link alone", got)
	}
	want := []string{"Project [dot] " + project + ": under the symlinked directory " + home +
		"/.pi, which leads out of the base; not walked"}
	if !reflect.DeepEqual(log.lines, want) {
		t.Errorf("log %q, want %q", log.lines, want)
	}
}

func TestProjectUnderNoHomeKeepsItsRecordedPath(t *testing.T) {
	root := fixture{files: []string{"srv/real/AGENTS.md"},
		links: [][2]string{{"real", "srv/link"}}}.build(t)
	collector, rows, _ := testCollector(t, root, nil)
	collector.MaxSize = 1 << 20
	collector.Tables = &catalog.Tables{ProjectCatalog: []catalog.Entry{{Agent: "project",
		Glob: "AGENTS.md"}}}
	mustDo(t, collector.CollectProject(context.Background(), "", "", root+"/srv/link"))
	if got := field(rows, "archive_path"); !reflect.DeepEqual(got, []string{"fs/srv/link/AGENTS.md"}) {
		t.Errorf("archive paths %q", got)
	}
}
