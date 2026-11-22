package opencode

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// writeTree writes files, by path under a new home, and returns the home.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	home := t.TempDir()
	for rel, text := range files {
		path := filepath.Join(home, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestASessionsSideFilesAreReadOnceForAllItsMessages(t *testing.T) {
	const data = ".local/share/opencode"
	const messages = 20
	tests := []struct {
		name     string
		side     map[string]string
		messages string
	}{
		{"the old layout's info file", map[string]string{
			data + "/project/p/storage/session/info/ses_x.json": `{"id": "ses_x", "directory": "/work/x"}`},
			data + "/project/p/storage/session/message/ses_x/"},
		{"the new layout's project file", map[string]string{
			data + "/storage/session/proj/ses_x.json": `{"id": "ses_x", "projectID": "proj"}`,
			data + "/storage/project/proj.json":       `{"worktree": "/work/x"}`},
			data + "/storage/message/ses_x/"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{}
			for rel, text := range test.side {
				files[rel] = text
			}
			var rels []string
			for i := range messages {
				rel := fmt.Sprintf("%smsg_%03d.json", test.messages, i)
				rels = append(rels, rel)
				files[rel] = fmt.Sprintf(`{"id": "msg_%03d", "sessionID": "ses_x", "role": "assistant", `+
					`"providerID": "p", "modelID": "m%d"}`, i, i)
			}
			home := writeTree(t, files)
			collectedHome := &input.Home{DiskPath: home}
			subject := New()
			subject.Work = parser.CountingWork()
			rows := 0
			for _, rel := range rels {
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
					Home: collectedHome}
				if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
					rows++
					if row.ProjectPath != "/work/x" {
						t.Errorf("%s: row %q has project %q, want /work/x", rel, row.Text, row.ProjectPath)
					}
				}); err != nil {
					t.Fatal(err)
				}
			}
			if reads := subject.Work.Count("session side reads"); rows == 0 || reads != 1 {
				t.Errorf("%d messages gave %d rows and read the session's side files %d times, want once",
					messages, rows, reads)
			}
		})
	}
}

// Every session file of a project names the project's file, so it is read
// once per state of it for the home, and a damaged one is still reported on
// each session file as a fresh read would report it.
func TestAProjectFileIsReadOnceForAllItsSessions(t *testing.T) {
	const storage = ".local/share/opencode/storage"
	const sessions = 20
	tests := []struct {
		name, project, projectPath, problem string
	}{
		{"a whole project file gives each session its worktree", `{"worktree": "/work/proj"}`, "/work/proj", ""},
		{"a damaged project file is reported on each session", `{"worktree": `, "",
			"parser: project file p1.json could not be read (invalid JSON at line 1, column 14: " +
				"unexpected end of JSON input)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{storage + "/project/p1.json": test.project}
			var rels []string
			for i := range sessions {
				rel := fmt.Sprintf("%s/session/p1/ses_%03d.json", storage, i)
				rels = append(rels, rel)
				files[rel] = fmt.Sprintf(`{"id": "ses_%03d", "projectID": "p1", "title": "t%d"}`, i, i)
			}
			home := writeTree(t, files)
			collectedHome := &input.Home{DiskPath: home}
			subject := New()
			subject.Work = parser.CountingWork()
			for _, rel := range rels {
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
					Home: collectedHome}
				var problems []string
				if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
					if row.ProjectPath != test.projectPath {
						t.Errorf("%s: row %q has project %q, want %q", rel, row.Text, row.ProjectPath, test.projectPath)
					}
					if strings.HasPrefix(row.Text, parser.ProblemPrefix) {
						problems = append(problems, row.Text)
					}
				}); err != nil {
					t.Fatal(err)
				}
				if (test.problem == "") != (len(problems) == 0) || (test.problem != "" && problems[0] != test.problem) {
					t.Errorf("%s: problems %q, want %q", rel, problems, test.problem)
				}
			}
			if reads := subject.Work.Count("project file reads"); reads != 1 {
				t.Errorf("%d sessions read the project file %d times, want once", sessions, reads)
			}
		})
	}
}

// Each session looks for its file in every project directory, so each
// directory is listed once and the session's file name looked up in the
// listing, not looked at once per session and directory.
func TestEachProjectDirectoryIsListedOnceForAllSessions(t *testing.T) {
	const storage = ".local/share/opencode/storage"
	for _, sessions := range []int{4, 32} {
		t.Run(fmt.Sprintf("%d sessions over 6 project directories make 6 listings", sessions), func(t *testing.T) {
			files := map[string]string{}
			for project := range 5 {
				files[fmt.Sprintf("%s/session/p%d/other.json", storage, project)] = `{}`
			}
			var rels []string
			for i := range sessions {
				files[fmt.Sprintf("%s/session/p5/ses_%03d.json", storage, i)] =
					fmt.Sprintf(`{"id": "ses_%03d", "directory": "/work/s%d"}`, i, i)
				rel := fmt.Sprintf("%s/message/ses_%03d/msg_1.json", storage, i)
				files[rel] = fmt.Sprintf(`{"id": "msg_1", "sessionID": "ses_%03d", "role": "user"}`, i)
				rels = append(rels, rel)
			}
			home := writeTree(t, files)
			collectedHome := &input.Home{DiskPath: home}
			subject := New()
			subject.Work = parser.CountingWork()
			for i, rel := range rels {
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
					Home: collectedHome}
				want := fmt.Sprintf("/work/s%d", i)
				if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
					if row.ProjectPath != want {
						t.Errorf("%s: row %q has project %q, want %q", rel, row.Text, row.ProjectPath, want)
					}
				}); err != nil {
					t.Fatal(err)
				}
			}
			if listings := subject.Work.Count("project directory listings"); listings != 6 {
				t.Errorf("%d sessions made %d project directory listings, want 6", sessions, listings)
			}
		})
	}
}
