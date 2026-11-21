package zed

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The times of the two sidebar rows of the repeated thread key, and a
// bound between them.
const (
	firstUpdated  = "2026-01-01T00:00:00.000Z"
	secondUpdated = "2026-06-01T00:00:00.000Z"
	betweenRows   = "2026-03-01T00:00:00.000Z"
)

// repeatedSidebar returns a db.sqlite whose sidebar_threads holds the
// thread key t1 in two rows, as only damage writes (the column is the
// table's primary key in Zed's schema, left out here): titled old at
// /proj/a with session s-old, then new at /proj/b with session s-new. The
// draft store holds two drafts of t1, and an archived worktree is linked
// to t1. A thread t2 of one row has a draft of its own.
func repeatedSidebar(t *testing.T) []byte {
	t.Helper()
	data, err := sqlitetest.Build(
		sqlitetest.Table{Name: "sidebar_threads", SQL: "CREATE TABLE sidebar_threads (thread_id BLOB, " +
			"session_id TEXT, agent_id TEXT, title TEXT, updated_at TEXT, folder_paths TEXT)", Rows: [][]any{
			{"t1", "s-old", nil, "old", "2026-01-01T00:00:00Z", "/proj/a"},
			{"t1", "s-new", nil, "new", "2026-06-01T00:00:00Z", "/proj/b"},
			{"t2", nil, nil, "only", "2026-02-01T00:00:00Z", "/proj/c"}}},
		sqlitetest.Table{Name: "scoped_kv_store", SQL: "CREATE TABLE scoped_kv_store (namespace TEXT, key TEXT, " +
			"value TEXT)", Rows: [][]any{
			{draftsNamespace, "t1", `[{"type":"text","text":"hello"}]`},
			{draftsNamespace, "t1", `[{"type":"text","text":"again"}]`},
			{draftsNamespace, "t2", `[{"type":"text","text":"alone"}]`}}},
		sqlitetest.Table{Name: "archived_git_worktrees", SQL: "CREATE TABLE archived_git_worktrees (id INTEGER, " +
			"worktree_path TEXT, main_repo_path TEXT)", Rows: [][]any{{int64(7), "/w/tree", "/w/main"}}},
		sqlitetest.Table{Name: "thread_archived_worktrees", SQL: "CREATE TABLE thread_archived_worktrees " +
			"(thread_id TEXT, archived_worktree_id INTEGER)", Rows: [][]any{{"t1", int64(7)}}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// parseDatabase parses data as a channel's db.sqlite and returns its rows.
func parseDatabase(t *testing.T, data []byte) []timeline.Row {
	t.Helper()
	const rel = ".local/share/zed/db/0-stable/db.sqlite"
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

// sidebarFilter is what a test keeps of rows: those inside since and until
// ("" for no bound; an undated row is kept) whose text matches pattern.
type sidebarFilter struct {
	since, until, pattern string
}

// keep returns the rows the filter keeps, in order.
func (f sidebarFilter) keep(rows []timeline.Row) []timeline.Row {
	match := regexp.MustCompile(f.pattern)
	var kept []timeline.Row
	for i := range rows {
		timestamp := rows[i].TimestampUTC
		if timestamp != "" && (f.since != "" && timestamp < f.since || f.until != "" && timestamp > f.until) {
			continue
		}
		if rows[i].Matches(match) {
			kept = append(kept, rows[i])
		}
	}
	return kept
}

// settledSidebar returns the rows of thread t1 the timeline command shows
// of repeatedSidebar under the filter: the stored row the key goes with is
// chosen from what the filter keeps of the parse, then the filter runs over
// the rows as attached. A draft reads "draft <time> <project> <text>", the
// archived worktree "link <session>", any other row of t1 "<time> <session>
// <project> <text>".
func settledSidebar(t *testing.T, rowFilter sidebarFilter) []string {
	t.Helper()
	rows := parseDatabase(t, repeatedSidebar(t))
	var got []string
	for _, row := range rowFilter.keep(timeline.Attach(rows, timeline.Choose(rowFilter.keep(rows)))) {
		switch {
		case row.ProjectPath == "/proj/c":
		case strings.HasPrefix(row.Text, "unsent draft: "):
			got = append(got, "draft "+row.TimestampUTC+" "+row.ProjectPath+" "+row.Text)
		case strings.HasPrefix(row.Text, "archived worktree: "):
			got = append(got, "link "+row.SessionID)
		case strings.HasPrefix(row.SessionID, "s-"):
			got = append(got, row.TimestampUTC+" "+row.SessionID+" "+row.ProjectPath+" "+row.Text)
		}
	}
	return got
}

// A sidebar thread key is the table's primary key, so a repeated one is
// damage: the drafts of the key and the session of a worktree linked to it
// go with the first stored row of the key the filters keep, the first when
// they keep none or none is given, and are shown once whichever it is.
func TestARepeatedSidebarThreadsDraftsAndLinksGoWithTheStoredRowTheFiltersKeep(t *testing.T) {
	const prefix = "parser: sidebar_threads holds 2 rows of this row's thread_id; its drafts and " +
		"archived worktree links are shown once, with "
	oldRow := firstUpdated + " s-old /proj/a "
	newRow := secondUpdated + " s-new /proj/b "
	oldThread := oldRow + "sidebar thread: title=old archived=false interacted_at=- main_worktree=-"
	newThread := newRow + "sidebar thread: title=new archived=false interacted_at=- main_worktree=-"
	oldDrafts := []string{"draft " + firstUpdated + " /proj/a unsent draft: hello",
		"draft " + firstUpdated + " /proj/a unsent draft: again"}
	newDrafts := []string{"draft " + secondUpdated + " /proj/b unsent draft: hello",
		"draft " + secondUpdated + " /proj/b unsent draft: again"}
	tests := []struct {
		name   string
		filter sidebarFilter
		want   []string
	}{
		{"without a filter they go with the first stored row", sidebarFilter{},
			slices.Concat([]string{oldRow + prefix + "this row", oldThread, newRow + prefix + "another of them",
				newThread}, oldDrafts, []string{"link s-old"})},
		{"a window that keeps only the first stored row keeps them with it", sidebarFilter{until: betweenRows},
			slices.Concat([]string{oldRow + prefix + "this row", oldThread}, oldDrafts)},
		{"a window that keeps only the later stored row moves them to it", sidebarFilter{since: betweenRows},
			slices.Concat([]string{newRow + prefix + "this row", newThread}, newDrafts, []string{"link s-new"})},
		{"a match only the later stored row's rows satisfy moves them to it",
			sidebarFilter{pattern: "title=new|unsent|archived worktree: "}, slices.Concat([]string{newThread}, newDrafts,
				[]string{"link s-new"})},
		{"a match only the first stored row's rows satisfy keeps them with it",
			sidebarFilter{pattern: "title=old|unsent|archived worktree: "}, slices.Concat([]string{oldThread}, oldDrafts,
				[]string{"link s-old"})},
		{"a match neither stored row's rows satisfy leaves them with the first",
			sidebarFilter{pattern: "unsent|archived worktree: "}, slices.Concat(oldDrafts, []string{"link s-old"})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := settledSidebar(t, test.filter); !slices.Equal(got, test.want) {
				t.Errorf("got  %q\nwant %q", got, test.want)
			}
		})
	}
}

// A thread key the sidebar holds once gives its draft its row's time and
// project, and no row of its own.
func TestASidebarThreadKeyHeldOnceGivesItsDraftItsRow(t *testing.T) {
	var got []string
	for _, row := range timeline.Attach(parseDatabase(t, repeatedSidebar(t)), nil) {
		if row.ProjectPath == "/proj/c" {
			got = append(got, row.TimestampUTC+" "+row.Text)
		}
	}
	if want := []string{"2026-02-01T00:00:00.000Z unsent draft: alone"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A link of a worktree id the archived table repeats, from a thread key the
// sidebar repeats, joins the worktree id's group, since a row joins one
// group: it is shown once, with the session of the key's first sidebar
// row whichever sidebar row the filters keep.
func TestALinkRepeatedInBothTablesKeepsTheFirstSidebarRowsSession(t *testing.T) {
	data, err := sqlitetest.Build(
		sqlitetest.Table{Name: "sidebar_threads", SQL: "CREATE TABLE sidebar_threads (thread_id BLOB, " +
			"session_id TEXT, title TEXT, updated_at TEXT)", Rows: [][]any{
			{"t1", "s-old", "old", "2026-01-01T00:00:00Z"}, {"t1", "s-new", "new", "2026-06-01T00:00:00Z"}}},
		sqlitetest.Table{Name: "archived_git_worktrees", SQL: "CREATE TABLE archived_git_worktrees (id INTEGER, " +
			"worktree_path TEXT, main_repo_path TEXT)", Rows: [][]any{{int64(1), "/w/copy0", "/w/main"},
			{int64(1), "/w/copy1", "/w/main"}}},
		sqlitetest.Table{Name: "thread_archived_worktrees", SQL: "CREATE TABLE thread_archived_worktrees " +
			"(thread_id TEXT, archived_worktree_id INTEGER)", Rows: [][]any{{"t1", int64(1)}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, rowFilter := range []sidebarFilter{{}, {since: betweenRows}} {
		rows := parseDatabase(t, data)
		var links []string
		for _, row := range rowFilter.keep(timeline.Attach(rows, timeline.Choose(rowFilter.keep(rows)))) {
			if strings.HasPrefix(row.Text, "archived worktree: /w/copy0 ") && row.SessionID != "" {
				links = append(links, row.SessionID)
			}
		}
		if want := []string{"s-old"}; !slices.Equal(links, want) {
			t.Errorf("since %q: links %q, want %q", rowFilter.since, links, want)
		}
	}
}
