//go:build unix

package walk

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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

// requireRows fails the test unless each manifest field of rows holds the
// values of want, row by row.
func requireRows(t *testing.T, rows *bytes.Buffer, want map[string][]string) {
	t.Helper()
	for key, values := range want {
		if got := field(rows, key); !reflect.DeepEqual(got, values) {
			t.Errorf("%s %q, want %q", key, got, values)
		}
	}
}

// requireStaged fails the test unless the staging directory holds source's
// content at archivePath as a regular file reached through real directories
// only.
func requireStaged(t *testing.T, stageDir, archivePath, source string) {
	t.Helper()
	for dir := archivePath; dir != "."; dir = filepath.Dir(dir) {
		info, err := os.Lstat(stageDir + "/" + dir)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("staged %s: %s is not a real entry (%v)", archivePath, dir, err)
			return
		}
	}
	staged, err := os.ReadFile(stageDir + "/" + archivePath)
	content, sourceErr := os.ReadFile(source)
	if err != nil || sourceErr != nil || string(staged) != string(content) {
		t.Errorf("staged %s %q (%v), source %q (%v)", archivePath, staged, err, content, sourceErr)
	}
}

func TestProjectThroughALinkOutOfItsHome(t *testing.T) {
	cases := []struct {
		name     string
		absolute bool
		viaLink  bool
	}{
		{"a link with a relative target is resolved to stage the project where it is", false, false},
		{"a link with an absolute target is resolved to stage the project where it is", true, false},
		{"a root given through a link keeps the archive path under the root", true, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := fixture{files: []string{"data/code/proj/AGENTS.md"}, dirs: []string{"home/dot"}}.build(t)
			target := "../../data/code"
			if testCase.absolute {
				target = root + "/data/code"
			}
			mustDo(t, os.Symlink(target, root+"/home/dot/code"))
			if testCase.viaLink {
				mustDo(t, os.Symlink(root, root+"-link"))
				t.Cleanup(func() { _ = os.Remove(root + "-link") })
				root += "-link"
			}
			home := root + "/home/dot"
			collector, rows, log := testCollector(t, root, nil)
			collector.MaxSize = 1 << 20
			collector.Tables = &catalog.Tables{ProjectCatalog: []catalog.Entry{{Agent: "project",
				Glob: "AGENTS.md"}}}
			project := home + "/code/proj"
			mustDo(t, collector.CollectProject(context.Background(), "dot", home, project))
			requireRows(t, rows, map[string][]string{"path": {project + "/AGENTS.md"},
				"archive_path": {"fs/data/code/proj/AGENTS.md"}, "status": {"collected"}})
			want := "Project [dot] " + project + " (through the symlinked directory " + home +
				"/code, archived under " + root + "/data/code/proj)"
			if len(log.lines) == 0 || log.lines[0] != want {
				t.Errorf("log %q, want first %q", log.lines, want)
			}
			requireStaged(t, collector.Stager.Dir, "fs/data/code/proj/AGENTS.md",
				root+"/data/code/proj/AGENTS.md")
		})
	}
}

func TestProjectThatIsItselfALink(t *testing.T) {
	cases := []struct {
		name     string
		absolute bool
		walkHome bool
	}{
		{"a project linked by a relative target the home walk recorded", false, true},
		{"a project linked by an absolute target the home walk recorded", true, true},
		{"a project linked by a relative target no walk recorded gets its row", false, false},
		{"a project linked by an absolute target no walk recorded gets its row", true, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := fixture{files: []string{"home/dot/dotfiles/proj/AGENTS.md"},
				dirs: []string{"home/dot/.claude"}}.build(t)
			home := root + "/home/dot"
			target := "../dotfiles/proj"
			if testCase.absolute {
				target = home + "/dotfiles/proj"
			}
			mustDo(t, os.Symlink(target, home+"/.claude/proj"))
			collector, rows, _ := testCollector(t, root, nil)
			collector.MaxSize = 1 << 20
			collector.Tables = &catalog.Tables{ProjectCatalog: []catalog.Entry{{Agent: "project",
				Glob: "AGENTS.md"}}}
			agent := "shared"
			if testCase.walkHome {
				agent = "claude"
				mustDo(t, collector.CollectCatalog(context.Background(), "dot", home,
					[]catalog.Entry{{Agent: "claude", Glob: ".claude"}}, ""))
			}
			project := home + "/.claude/proj"
			mustDo(t, collector.CollectProject(context.Background(), "dot", home, project))
			requireRows(t, rows, map[string][]string{
				"path":         {project, project + "/AGENTS.md"},
				"archive_path": {"fs/home/dot/.claude/proj", "fs/home/dot/dotfiles/proj/AGENTS.md"},
				"agent":        {agent, "project"},
				"status":       {"symlink", "collected"},
			})
			requireStaged(t, collector.Stager.Dir, "fs/home/dot/dotfiles/proj/AGENTS.md",
				home+"/dotfiles/proj/AGENTS.md")
		})
	}
}

func TestProjectStagers(t *testing.T) {
	root := fixture{files: []string{"home/dot/dotfiles/claude/proj/env.sh",
		"home/dot/dotfiles/claude/proj/real/env.sh", "outside/env.sh"},
		links: [][2]string{{"real", "home/dot/dotfiles/claude/proj/inner"},
			{"../../../../../outside", "home/dot/dotfiles/claude/proj/out"}}}.build(t)
	home := root + "/home/dot"
	mustDo(t, os.Symlink(home+"/dotfiles/claude", home+"/.claude"))
	project := home + "/.claude/proj"
	physical := "fs/home/dot/dotfiles/claude/proj"
	cases := []struct {
		name, home, path, want string
		passed                 []string
	}{
		{"a file of a project through its home's link is staged where the home holds it",
			home, project + "/env.sh", physical + "/env.sh", nil},
		{"a link inside the project composes with the home's",
			home, project + "/inner/env.sh", physical + "/real/env.sh", []string{project + "/inner"}},
		{"a link out of the project leaves the file at the project's place",
			home, project + "/out/env.sh", physical + "/out/env.sh", nil},
		{"a project under no home keeps its recorded path",
			"", project + "/env.sh", "fs/home/dot/.claude/proj/env.sh", nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			collector, _, _ := testCollector(t, root, nil)
			stagers := ProjectStagers(collector.Stager, testCase.home, project, testCase.path)
			if got, _ := stagers.File.Dest(testCase.path); got != testCase.want {
				t.Errorf("archive path %q, want %q", got, testCase.want)
			}
			if !reflect.DeepEqual(stagers.Passed, testCase.passed) {
				t.Errorf("passed %q, want %q", stagers.Passed, testCase.passed)
			}
		})
	}
}

func TestProjectOutOfTheImageRootIsNotWalked(t *testing.T) {
	cases := []struct {
		name    string
		withIn  bool // the link lies under a home given to CollectProject
		live    bool
		noEnter bool // the outside target cannot be entered by the analyst
	}{
		{"a project under a home through a link out of the image root", true, false, false},
		{"a project under no home through a link out of the image root", false, false, false},
		{"a live run walks the same project", true, true, false},
		{"an outside target the analyst cannot enter is still outside", true, false, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.noEnter && os.Geteuid() == 0 {
				t.Skip("root enters every directory")
			}
			root := fixture{dirs: []string{"home/dot"}}.build(t)
			outside := fixture{files: []string{"host/proj/AGENTS.md"}}.build(t)
			link := root + "/home/dot/ext"
			mustDo(t, os.Symlink(outside+"/host", link))
			if testCase.noEnter {
				mustDo(t, os.Chmod(outside+"/host/proj", 0o600))
				t.Cleanup(func() { _ = os.Chmod(outside+"/host/proj", 0o755) })
			}
			collector, rows, log := testCollector(t, root, nil)
			collector.MaxSize = 1 << 20
			collector.Tables = &catalog.Tables{ProjectCatalog: []catalog.Entry{{Agent: "project",
				Glob: "AGENTS.md"}}}
			if testCase.live {
				collector.Root, collector.Stager.Root = "", ""
			}
			home := ""
			if testCase.withIn {
				home = root + "/home/dot"
			}
			project := link + "/proj"
			mustDo(t, collector.CollectProject(context.Background(), "dot", home, project))
			if testCase.live {
				requireRows(t, rows, map[string][]string{"path": {project + "/AGENTS.md"},
					"status": {"collected"}})
				return
			}
			want := []string{"Project [dot] " + project + ": under the symlinked directory " + link +
				", which leads out of the image root; not walked"}
			if !reflect.DeepEqual(log.lines, want) {
				t.Errorf("log %q, want %q", log.lines, want)
			}
			if rows.Len() != 0 {
				t.Errorf("rows %q, want none", rows.String())
			}
			if entries, _ := os.ReadDir(collector.Stager.Dir); len(entries) != 0 {
				t.Errorf("staged %d entries, want none", len(entries))
			}
			stagers := ProjectStagers(collector.Stager, home, project, project+"/AGENTS.md")
			if !stagers.Skip {
				t.Errorf("stagers %+v, want the project skipped", stagers)
			}
		})
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
