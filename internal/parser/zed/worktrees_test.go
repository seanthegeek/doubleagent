package zed

import (
	"fmt"
	"path/filepath"
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
// damage: the id's thread links join its first row, in id order, each
// later row gives its own unlinked row, and the table one problem row, so
// the rows grow with the table rather than with the square of it.
func TestARepeatedArchivedWorktreeIDJoinsItsLinksOnce(t *testing.T) {
	for _, count := range []int{10, 40} {
		t.Run(strconv.Itoa(count)+" rows of one id and as many links", func(t *testing.T) {
			var got []string
			for _, row := range sidebarRows(t, "repeated_worktree_"+strconv.Itoa(count)+".db") {
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
