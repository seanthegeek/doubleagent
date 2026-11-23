//go:build unix

package littlecoder

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestAPiSessionIsNeverFoundThroughALinkedDirectory(t *testing.T) {
	const name = "2026-01-01T00-00-00-000Z_s1.jsonl"
	tests := []struct {
		name, linked, want string
	}{
		{"a project directory of the sessions gives the header", "", "/work/project"},
		{"a linked sessions directory gives none", "sessions", ""},
		{"a linked project directory gives none", filepath.Join("sessions", "--work-project--"), ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			agent := filepath.Join(home, ".pi", "agent")
			project := filepath.Join(agent, "sessions", "--work-project--")
			if err := os.MkdirAll(project, 0o700); err != nil {
				t.Fatal(err)
			}
			header := `{"type": "session", "id": "s1", "cwd": "/work/project"}` + "\n"
			if err := os.WriteFile(filepath.Join(project, name), []byte(header), 0o600); err != nil {
				t.Fatal(err)
			}
			if test.linked != "" {
				inside := filepath.Join(agent, test.linked)
				outside := filepath.Join(t.TempDir(), "moved")
				if err := os.Rename(inside, outside); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, inside); err != nil {
					t.Fatal(err)
				}
			}
			const rel = ".little-coder/checkpoints/" + name + "/file.py"
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
				Home: &input.Home{DiskPath: home}}
			found, problems := New().sessionHeader(artifact, name)
			if got := found.Get("cwd").Text(); got != test.want || len(problems) > 0 {
				t.Errorf("header cwd %q, problems %+v; want %q", got, problems, test.want)
			}
		})
	}
}

// A directory that cannot be listed holds no session file and fails, and
// one that can be listed but not searched fails for the names its listing
// holds: a lookup gives the file from the first directory that holds it,
// else the problem of the first such directory, as listing every directory
// in turn gives.
func TestADirectoryThatCannotBeListedOrSearchedGivesWhatListingEveryDirectoryGives(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions do not bind the superuser")
	}
	header := func(cwd string) string { return `{"type": "session", "id": "x", "cwd": "` + cwd + `"}` + "\n" }
	tests := []struct {
		name, locked string
		mode         os.FileMode
	}{
		{"a project directory that cannot be listed", ".pi/agent/sessions/p1", 0o300},
		{"a project directory that cannot be searched", ".pi/agent/sessions/p1", 0o600},
		{"a later project directory that cannot be searched", ".pi/agent/sessions/p3", 0o600},
		{"a sessions directory that cannot be listed", ".pi/agent/sessions", 0o300},
		{"a sessions directory that cannot be searched", ".pi/agent/sessions", 0o600},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := writeHome(t, map[string]string{".pi/agent/sessions/p1/a_s1.jsonl": header("/locked"),
				".pi/agent/sessions/p2/a_s1.jsonl": header("/open"), ".pi/agent/sessions/p2/b_s2.jsonl": header("/b"),
				".pi/agent/sessions/p3/e_s5.jsonl": header("/e")})
			locked := filepath.Join(home.DiskPath, filepath.FromSlash(test.locked))
			if err := os.Chmod(locked, test.mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
			checkSameHeaders(t, home)
		})
	}
}

// lockAll gives each directory mode, giving them back their modes when the
// test ends, and skips the test when the first can still be listed, as it
// can by the superuser.
func lockAll(t *testing.T, mode os.FileMode, directories ...string) {
	t.Helper()
	for _, directory := range directories {
		if err := os.Chmod(directory, mode); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
	}
	if _, err := os.ReadDir(directories[0]); err == nil {
		t.Skip("a directory without read permission can still be listed by this user")
	}
}

// Every checkpoint set looks for its session file in every project
// directory, so a project directory that cannot be listed is remembered as
// failed when the sessions directory is first listed: as many sets as such
// directories look only where a listing holds their name, not in each of
// them.
func TestAProjectDirectoryThatCannotBeListedIsLookedInByNoSet(t *testing.T) {
	for _, n := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d sets beside %d project directories that cannot be listed", n, n), func(t *testing.T) {
			files := map[string]string{
				".pi/agent/sessions/p/2026_s0.jsonl": `{"type": "session", "id": "s0", "cwd": "/w"}` + "\n"}
			var locked []string
			for i := range n {
				files[fmt.Sprintf(".little-coder/checkpoints/2026_s%d.jsonl/a.py", i)] = "x"
				files[fmt.Sprintf(".pi/agent/sessions/locked%03d/other.jsonl", i)] = ""
			}
			home := writeHome(t, files)
			for i := range n {
				locked = append(locked, filepath.Join(home.DiskPath, ".pi", "agent", "sessions", fmt.Sprintf("locked%03d", i)))
			}
			lockAll(t, 0o000, locked...)
			subject := New()
			subject.Work = parserkit.CountingWork()
			found, problems := 0, 0
			for i := range n {
				rel := fmt.Sprintf(".little-coder/checkpoints/2026_s%d.jsonl/a.py", i)
				artifact := &input.Artifact{DiskPath: filepath.Join(home.DiskPath, filepath.FromSlash(rel)), Rel: rel,
					Home: home}
				if err := subject.Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
					switch {
					case row.ProjectPath == "/w":
						found++
					case row.Text == parserkit.ProblemPrefix+"locked000 could not be read (permission denied)":
						problems++
					}
				}); err != nil {
					t.Fatal(err)
				}
			}
			candidates := subject.Work.Count("pi session file candidates")
			listings := subject.Work.Count("pi project directory listings")
			if found != 1 || problems != n-1 || candidates != 1 || listings != n+1 {
				t.Errorf("%d found, %d problems, %d candidates and %d listings; want 1, %d, 1 and %d",
					found, problems, candidates, listings, n-1, n+1)
			}
		})
	}
}

// A checkpoint directory that cannot be listed gives no row, and its files
// each ask for that one listing: it is made once, not once per file.
func TestACheckpointDirectoryThatCannotBeListedIsListedOnce(t *testing.T) {
	for _, n := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d files", n), func(t *testing.T) {
			const set = ".little-coder/checkpoints/c0ffee12/"
			files := map[string]string{}
			for i := range n {
				files[fmt.Sprintf("%sf%03d.py", set, i)] = "x"
			}
			home := writeHome(t, files)
			lockAll(t, 0o300, filepath.Join(home.DiskPath, filepath.FromSlash(set)))
			subject := New()
			subject.Work = parserkit.CountingWork()
			rows := 0
			for i := range n {
				rel := fmt.Sprintf("%sf%03d.py", set, i)
				artifact := &input.Artifact{DiskPath: filepath.Join(home.DiskPath, filepath.FromSlash(rel)), Rel: rel,
					Home: home}
				if err := subject.Parse(artifact, parserkit.Options{}, func(timeline.Row) { rows++ }); err != nil {
					t.Fatal(err)
				}
			}
			if listings := subject.Work.Count("checkpoint listings"); rows != 0 || listings != 1 {
				t.Errorf("%d rows and %d listings, want 0 and 1", rows, listings)
			}
		})
	}
}
