package zed

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// worktreeWork parses testdata/worktrees_<n>.db, written by
// testdata/worktrees.py, and returns its scratch directory rows and the
// sidebar folders and archived worktrees the parser indexed.
func worktreeWork(t *testing.T, n int) (rows []string, folders, archived int) {
	t.Helper()
	const rel = ".local/share/zed/db/0-stable/db.sqlite"
	data, err := osfile.ReadFile(filepath.Join("testdata", "worktrees_"+strconv.Itoa(n)+".db"))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "zed",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	subject := New()
	subject.Work = parser.CountingWork()
	err = subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
		if strings.HasPrefix(row.Text, "scratch directory: ") {
			rows = append(rows, row.Text)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	return rows, subject.Work.Count("sidebar folders indexed"), subject.Work.Count("archived worktrees indexed")
}

// The main repository of every created worktree is looked up in an index
// of the sidebar threads and archived worktrees made once, so the work
// grows with the rows, not with the created worktrees times the rows.
func TestCreatedWorktreesFindTheirRepositoryInOneIndex(t *testing.T) {
	for _, n := range []int{8, 64} {
		t.Run(strconv.Itoa(n)+" created worktrees index each row once", func(t *testing.T) {
			rows, folders, archived := worktreeWork(t, n)
			if folders != n || archived != n {
				t.Errorf("indexed %d folders and %d archived worktrees, want %d of each", folders, archived, n)
			}
			if len(rows) != n {
				t.Fatalf("got %d scratch directory rows, want %d", len(rows), n)
			}
			for i, text := range rows {
				if !strings.Contains(text, "(Zed-created git worktree of /w/main)") {
					t.Errorf("row %d is %q, want the sidebar thread's main worktree", i, text)
				}
			}
		})
	}
}

// sidebarRows parses the named testdata db.sqlite and returns its rows.
func sidebarRows(t *testing.T, name string) []timeline.Row {
	t.Helper()
	const rel = ".local/share/zed/db/0-stable/db.sqlite"
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "zed",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		t.Fatal(err)
	}
	return rows
}

// An archived worktree's id is its table's key, so a repeated one is
// damage: without a filter the id's thread links join its first row, in id
// order, each later row gives its own unlinked row, and the table one
// problem row, so the rows grow with the table rather than with the square
// of it.
func TestARepeatedArchivedWorktreeIDJoinsItsLinksOnce(t *testing.T) {
	for _, count := range []int{10, 40} {
		t.Run(strconv.Itoa(count)+" rows of one id and as many links", func(t *testing.T) {
			var got []string
			for _, row := range timeline.Attach(sidebarRows(t, "repeated_worktree_"+strconv.Itoa(count)+".db"), nil) {
				switch {
				case strings.HasPrefix(row.Text, "archived worktree: "):
					path, _, _ := strings.Cut(strings.TrimPrefix(row.Text, "archived worktree: "), " ")
					got = append(got, path+" "+row.SessionID)
				case strings.HasPrefix(row.Text, "parser: "):
					got = append(got, row.Text)
				}
			}
			var want []string
			for i := range count {
				want = append(want, fmt.Sprintf("/w/copy0 s%03d", i))
			}
			for i := 1; i < count; i++ {
				want = append(want, fmt.Sprintf("/w/copy%d ", i))
			}
			want = append(want, fmt.Sprintf("parser: archived_git_worktrees holds %d rows repeating an earlier "+
				"row's id; its linked threads are shown once", count-1))
			if !slices.Equal(got, want) {
				t.Errorf("%d rows %q, want %d rows %q", len(got), got[:min(len(got), 3)], len(want), want[:3])
			}
		})
	}
}

// settledWorktrees returns the archived worktree rows of the named testdata
// db.sqlite as the timeline command shows them under a --match of
// pattern: the stored row each repeated id goes with is chosen from what
// the pattern keeps of the parse, then the pattern runs over the rows as
// attached. Each row reads "<worktree path> <session id>".
func settledWorktrees(t *testing.T, name, pattern string) []string {
	t.Helper()
	match := regexp.MustCompile(pattern)
	keep := func(rows []timeline.Row) []timeline.Row {
		var kept []timeline.Row
		for i := range rows {
			if rows[i].Matches(match) {
				kept = append(kept, rows[i])
			}
		}
		return kept
	}
	rows := sidebarRows(t, name)
	var got []string
	for _, row := range keep(timeline.Attach(rows, timeline.Choose(keep(rows), timeline.TestedText))) {
		if rest, found := strings.CutPrefix(row.Text, "archived worktree: "); found {
			path, _, _ := strings.Cut(rest, " ")
			got = append(got, path+" "+row.SessionID)
		}
	}
	return got
}

// A --match keeping only a later stored row of a repeated id shows the
// id's links as rows of that stored row, once each with its own session,
// and leaves out the first stored row's rows; one keeping only the first
// keeps the links with it; one keeping neither shows none.
func TestARepeatedArchivedWorktreeIDsLinksGoWithTheStoredRowTheMatchKeeps(t *testing.T) {
	links := func(path string) []string {
		var rows []string
		for i := range 10 {
			rows = append(rows, fmt.Sprintf("%s s%03d", path, i))
		}
		return rows
	}
	tests := []struct {
		name, pattern string
		want          []string
	}{
		{"a match on a later stored row moves the links to it", "/w/copy3 ", links("/w/copy3")},
		{"a match on the first stored row keeps the links with it", "/w/copy0 ", links("/w/copy0")},
		{"a match on two later stored rows takes the earlier of them", "/w/copy[47] ",
			append(links("/w/copy4"), "/w/copy7 ")},
		{"a match on no stored row shows none of them", "/w/absent", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := settledWorktrees(t, "repeated_worktree_10.db", test.pattern); !slices.Equal(got, test.want) {
				t.Errorf("got  %q\nwant %q", got, test.want)
			}
		})
	}
}

// A link shown with a stored row of a repeated id names that row's
// source_line, as its text is that row's: the row's own when the group goes
// with it, the first row's without a filter.
func TestARepeatedArchivedWorktreeIDsLinksNameTheLineOfTheStoredRowTheyGoWith(t *testing.T) {
	tests := []struct {
		name, pattern string
		want          int
	}{
		{"without a filter the links name the first stored row", "", 1},
		{"a match on a later stored row gives the links its line", "/w/copy3 ", 4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := sidebarRows(t, "repeated_worktree_10.db")
			match := regexp.MustCompile(test.pattern)
			var kept []timeline.Row
			for i := range rows {
				if rows[i].Matches(match) {
					kept = append(kept, rows[i])
				}
			}
			links := 0
			for _, row := range timeline.Attach(rows, timeline.Choose(kept, timeline.TestedText)) {
				if row.SessionID == "" || !strings.HasPrefix(row.Text, "archived worktree: ") || !row.Matches(match) {
					continue
				}
				links++
				if row.SourceLine != test.want {
					t.Errorf("link of %s at source line %d, want %d", row.SessionID, row.SourceLine, test.want)
				}
			}
			if links != 10 {
				t.Errorf("got %d links, want 10", links)
			}
		})
	}
}
