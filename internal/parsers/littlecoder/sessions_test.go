package littlecoder

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// sessionHeaderByEveryDirectory finds a pi session file by the rule stated
// on Parser.sessionHeader, listing every project directory of the home's
// sessions directory in name order: the first whose listing holds the name
// in any ASCII case as a regular file and where the file is found, else the
// first problem in that order of a directory that could not be read or of
// a holder where the file could not be looked at.
func sessionHeaderByEveryDirectory(home *input.Home, name string) (jsonvalue.Value, parserkit.Problems) {
	sessions, _ := home.PathOf(".pi/agent/sessions")
	var directories []string
	info, err := lstatPresent(sessions)
	if err == nil && isDirectory(info) {
		var entries []os.DirEntry
		if entries, err = parserkit.ReadDir(sessions); err == nil {
			for _, entry := range entries {
				directories = append(directories, entry.Name())
			}
		}
	}
	if err != nil {
		return jsonvalue.NewNull(), parserkit.Problems{{Message: reason.Unreadable(sessions, err)}}
	}
	var firstFailure parserkit.Problems
	for _, directory := range directories {
		header, failure, found := listedSession(sessions, filepath.Join(sessions, directory), name)
		if found {
			return header, failure
		}
		if firstFailure == nil {
			firstFailure = failure
		}
	}
	return jsonvalue.NewNull(), firstFailure
}

// listedSession lists one project directory and looks for the session file
// name in it when the listing holds it in any ASCII case as a regular file:
// the file's header and problems and true when found, else the problem of
// a directory that could not be read or a file that could not be looked
// at, and false.
func listedSession(sessions, project, name string) (jsonvalue.Value, parserkit.Problems, bool) {
	info, err := lstatPresent(project)
	switch {
	case err != nil:
		return jsonvalue.NewNull(), parserkit.Problems{{Message: reason.Unreadable(sessions, err)}}, false
	case !isDirectory(info):
		return jsonvalue.NewNull(), nil, false
	}
	entries, err := parserkit.ReadDir(project)
	if err != nil {
		return jsonvalue.NewNull(), parserkit.Problems{{Message: reason.Unreadable(project, err)}}, false
	}
	if !slices.ContainsFunc(entries, func(entry os.DirEntry) bool {
		return lowerASCII(entry.Name()) == lowerASCII(name) && entry.Type().IsRegular()
	}) {
		return jsonvalue.NewNull(), nil, false
	}
	return projectSession(project, sessions, name)
}

// writeHome writes each file at its path relative to a new home and
// returns the home.
func writeHome(t *testing.T, files map[string]string) *input.Home {
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
	return &input.Home{DiskPath: home}
}

// sessionNames are the names the comparisons look up: in one directory,
// in two, in another ASCII case, absent, and a directory's name.
func sessionNames() []string {
	return []string{"a_s1.jsonl", "b_s2.jsonl", "A_S1.JSONL", "c_s3.jsonl", "d_s4.jsonl", "e_s5.jsonl"}
}

// checkSameHeaders looks up every name of sessionNames in home, through
// one parser in turn and by a search of every directory, and fails test
// where the two differ.
func checkSameHeaders(t *testing.T, home *input.Home) {
	t.Helper()
	subject := New()
	for _, name := range sessionNames() {
		artifact := &input.Artifact{Home: home}
		got, gotProblems := subject.sessionHeader(artifact, name)
		want, wantProblems := sessionHeaderByEveryDirectory(home, name)
		if got.JSON() != want.JSON() || !slices.Equal(gotProblems, wantProblems) {
			t.Errorf("%s: got %s %+v, want %s %+v", name, got.JSON(), gotProblems, want.JSON(), wantProblems)
		}
	}
}

func TestThePiSessionFileIsTheOneEveryProjectDirectoryInTurnWouldGive(t *testing.T) {
	const sessions = ".pi/agent/sessions/"
	header := func(cwd string) string { return `{"type": "session", "id": "x", "cwd": "` + cwd + `"}` + "\n" }
	tests := []struct {
		name  string
		files map[string]string
	}{
		{"a name in one directory", map[string]string{sessions + "p1/a_s1.jsonl": header("/one")}},
		{"a name in two directories is the first one's", map[string]string{
			sessions + "p2/b_s2.jsonl": header("/second"), sessions + "p1/b_s2.jsonl": header("/first")}},
		{"a directory of the name is passed over", map[string]string{
			sessions + "p1/a_s1.jsonl/inner": "", sessions + "p2/a_s1.jsonl": header("/file")}},
		{"a project entry that is a file holds nothing", map[string]string{
			sessions + "a_s1.jsonl": header("/top"), sessions + "p1/other": ""}},
		{"a name no directory holds is null", map[string]string{sessions + "p1/other.jsonl": header("/x")}},
		{"a file that is not a session gives its problems", map[string]string{sessions + "p1/a_s1.jsonl": "["}},
		{"no sessions directory gives nothing", map[string]string{".pi/agent/other": ""}},
		{"a sessions entry that is a file gives nothing", map[string]string{".pi/agent/sessions": ""}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checkSameHeaders(t, writeHome(t, test.files))
		})
	}
}

// Each checkpoint set of a pi session looks for its session file in every
// project directory, so as many sets as directories would cost their
// product if each lookup listed them again: the home's sessions directory
// and each project directory are listed once, and a lookup looks only
// where a listing holds the name.
func TestFindingTheSessionFilesOfManyCheckpointSetsIsLinear(t *testing.T) {
	for _, sets := range []int{10, 80} {
		t.Run(fmt.Sprintf("%d sets beside %d project directories", sets, sets), func(t *testing.T) {
			files := map[string]string{}
			for i := range sets {
				files[fmt.Sprintf(".little-coder/checkpoints/2026_s%d.jsonl/a.py", i)] = "x"
				files[fmt.Sprintf(".pi/agent/sessions/p%d/other_%d.jsonl", i, i)] = ""
			}
			files[".pi/agent/sessions/p0/2026_s0.jsonl"] = `{"type": "session", "id": "s0", "cwd": "/w"}` + "\n"
			home := writeHome(t, files)
			subject := New()
			subject.Work = parserkit.CountingWork()
			rows := 0
			for i := range sets {
				rel := fmt.Sprintf(".little-coder/checkpoints/2026_s%d.jsonl/a.py", i)
				artifact := &input.Artifact{DiskPath: filepath.Join(home.DiskPath, filepath.FromSlash(rel)), Rel: rel,
					Home: home}
				if err := subject.Parse(artifact, parserkit.Options{}, func(timeline.Row) { rows++ }); err != nil {
					t.Fatal(err)
				}
			}
			listings := subject.Work.Count("pi session directory listings") +
				subject.Work.Count("pi project directory listings")
			candidates := subject.Work.Count("pi session file candidates")
			if rows != sets || listings != sets+1 || candidates != 1 {
				t.Errorf("%d rows, %d listings and %d candidates, want %d rows, %d listings and 1 candidate",
					rows, listings, candidates, sets, sets+1)
			}
		})
	}
}

// A checkpoint directory's row comes with its first file by name that is
// read as a checkpoint: a snapshots.json read as v0.0.x snapshots, which
// sorts before a pre-edit copy, does not take the row from the copies.
func TestACheckpointDirectoryGivesItsRowBesideASnapshotsFileThatSortsFirst(t *testing.T) {
	const set = ".little-coder/checkpoints/c0ffee12/"
	tests := []struct {
		name  string
		files []string
		want  []string
	}{
		{"a copy after snapshots.json", []string{"snapshots.json", "x.py"},
			[]string{"little-coder checkpoints for no session file (c0ffee12): 2 pre-edit file(s), " +
				"session file not collected: snapshots.json; x.py"}},
		{"a copy before snapshots.json", []string{"a.py", "snapshots.json"},
			[]string{"little-coder checkpoints for no session file (c0ffee12): 2 pre-edit file(s), " +
				"session file not collected: a.py; snapshots.json"}},
		{"snapshots.json alone", []string{"snapshots.json"}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{}
			for _, name := range test.files {
				files[set+name] = "[]"
			}
			home := writeHome(t, files)
			subject := New()
			var got []string
			for _, name := range test.files {
				artifact := &input.Artifact{DiskPath: filepath.Join(home.DiskPath, filepath.FromSlash(set+name)),
					Rel: set + name, Home: home}
				if err := subject.Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
					if strings.HasPrefix(row.Text, "little-coder checkpoints") {
						got = append(got, row.Text)
					}
				}); err != nil {
					t.Fatal(err)
				}
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
