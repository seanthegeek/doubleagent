package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// tabbyRepeatedDatabase returns a Tabby db.sqlite whose pages table
// repeats a page id, as only damage writes: an old row and a new one five
// months later, each with its own title and content, and two sections of
// the page, one with no created_at and one dated a day after the new row.
func tabbyRepeatedDatabase(t *testing.T) string {
	t.Helper()
	data, err := sqlitetest.Build(
		sqlitetest.Table{Name: "pages", SQL: "CREATE TABLE pages (id INTEGER, title TEXT, content TEXT, " +
			"created_at TIMESTAMP)", Rows: [][]any{{2, "old page", "old body", "2026-01-01 00:00:00"},
			{2, "new page", "new body", "2026-06-01 00:00:00"}}},
		sqlitetest.Table{Name: "page_sections", SQL: "CREATE TABLE page_sections (id INTEGER, page_id INTEGER, " +
			"title TEXT, content TEXT, position INTEGER, created_at TIMESTAMP)", Rows: [][]any{
			{1, 2, "undated section", "text", 0, nil}, {2, 2, "dated section", "text", 1, "2026-06-02 00:00:00"}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// tabbyRepeatedRun runs timeline over a collection holding the repeated
// page id's database with the options given, and returns each row with a
// session as "time session text".
func tabbyRepeatedRun(t *testing.T, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "tabby", ".tabby/ee/db.sqlite", tabbyRepeatedDatabase(t)}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, TimelineFile), "session_id", "timestamp_utc", "session_id", "text")
}

func TestTheUndatedSectionsOfARepeatedTabbyPageTakeTheTimeOfTheFirstRowTheFilterKeeps(t *testing.T) {
	const (
		old      = "2026-01-01T00:00:00.000Z page:2 "
		current  = "2026-06-01T00:00:00.000Z page:2 "
		undated  = "undated section\n\ntext"
		dated    = "2026-06-02T00:00:00.000Z page:2 dated section\n\ntext"
		oldStart = old + "page start | title old page"
		newStart = current + "page start | title new page"
	)
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"without a filter it takes the first row's time", nil, []string{
			old + undated, oldStart, old + "old body", newStart, current + "new body", dated}},
		{"a window that keeps only the later row gives it that row's time", []string{"--since", "2026-03-01"},
			[]string{current + undated, newStart, current + "new body", dated}},
		{"a window that keeps only the earlier row keeps its time", []string{"--until", "2026-03-01"},
			[]string{old + undated, oldStart, old + "old body"}},
		{"a pattern only the later row matches gives it that row's time", []string{"--match", "new page|section"},
			[]string{current + undated, newStart, dated}},
		{"a pattern no row of the id matches leaves it with the first", []string{"--match", "section"},
			[]string{old + undated, dated}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := tabbyRepeatedRun(t, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
			if count := strings.Count(strings.Join(rows, "\n"), "undated section"); count != 1 {
				t.Errorf("the undated section is shown %d times, want once", count)
			}
		})
	}
}

// tabbyProjectDatabase returns a Tabby db.sqlite whose threads and pages
// tables each repeat an id, as only damage writes, an old row and a new
// one five months later: the thread's message and the page's section
// attach code from one repository, the project of the id's rows.
func tabbyProjectDatabase(t *testing.T) string {
	t.Helper()
	const attachment = `{"code":[{"git_url":"https://example.com/repo.git","filepath":"a.go"}]}`
	data, err := sqlitetest.Build(
		sqlitetest.Table{Name: "threads", SQL: "CREATE TABLE threads (id INTEGER, created_at TIMESTAMP)",
			Rows: [][]any{{1, "2026-01-01 00:00:00"}, {1, "2026-06-01 00:00:00"}}},
		sqlitetest.Table{Name: "thread_messages", SQL: "CREATE TABLE thread_messages (id INTEGER, " +
			"thread_id INTEGER, role TEXT, content TEXT, attachment TEXT, created_at TIMESTAMP)",
			Rows: [][]any{{1, 1, "user", "ask", attachment, "2026-01-01 00:00:01"}}},
		sqlitetest.Table{Name: "pages", SQL: "CREATE TABLE pages (id INTEGER, title TEXT, created_at TIMESTAMP)",
			Rows: [][]any{{2, "old page", "2026-01-01 00:00:00"}, {2, "new page", "2026-06-01 00:00:00"}}},
		sqlitetest.Table{Name: "page_sections", SQL: "CREATE TABLE page_sections (id INTEGER, page_id INTEGER, " +
			"title TEXT, content TEXT, position INTEGER, attachment TEXT, created_at TIMESTAMP)",
			Rows: [][]any{{1, 2, "section", "text", 0, attachment, nil}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// tabbyProjectRun runs timeline over tabbyProjectDatabase with the options
// given, and returns each thread and page start row as "time session
// project text".
func tabbyProjectRun(t *testing.T, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "tabby", ".tabby/ee/db.sqlite", tabbyProjectDatabase(t)}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	var starts []string
	for _, row := range recordsOf(t, filepath.Join(output, TimelineFile), "session_id", "timestamp_utc",
		"session_id", "project_path", "text") {
		if strings.Contains(row, " thread start") || strings.Contains(row, " page start") {
			starts = append(starts, row)
		}
	}
	return starts
}

// The project of a thread or page is the repository its rows attach code
// from, so every stored row of a repeated id shows it on its start row,
// whichever row the id's rows are shown with and whatever the filters keep.
func TestEveryStartRowOfARepeatedTabbyIDShowsTheIDsProject(t *testing.T) {
	const (
		project = " https://example.com/repo.git "
		old     = "2026-01-01T00:00:00.000Z "
		current = "2026-06-01T00:00:00.000Z "
	)
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"without a filter", nil, []string{old + "1" + project + "thread start", old + "page:2" + project +
			"page start | title old page", current + "1" + project + "thread start",
			current + "page:2" + project + "page start | title new page"}},
		{"with a window that keeps only the later rows", []string{"--since", "2026-03-01"},
			[]string{current + "1" + project + "thread start", current + "page:2" + project +
				"page start | title new page"}},
		{"with a window that keeps only the earlier rows", []string{"--until", "2026-03-01"},
			[]string{old + "1" + project + "thread start", old + "page:2" + project + "page start | title old page"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if rows := tabbyProjectRun(t, test.options...); !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// tabbyBackup returns a Tabby backup database holding the repeated page's
// later row alone, with its undated section, which a page that is not
// repeated dates at the page's time.
func tabbyBackup(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "pages", SQL: "CREATE TABLE pages (id INTEGER, title TEXT, content TEXT, " +
			"created_at TIMESTAMP)", Rows: [][]any{{2, "new page", "new body", "2026-06-01 00:00:00"}}},
		sqlitetest.Table{Name: "page_sections", SQL: "CREATE TABLE page_sections (id INTEGER, page_id INTEGER, " +
			"title TEXT, content TEXT, position INTEGER, created_at TIMESTAMP)", Rows: [][]any{
			{1, 2, "undated section", "text", 0, nil}}})
}

// Deduplication compares a repeated id's rows as they are shown: with the
// time of the stored row the filter chose, the undated section of the
// database is the backup's copy of it, shown once; without a filter, at
// the first row's time, the backup's later copy is its own row.
func TestDeduplicationComparesARepeatedIDsRowsWithTheChosenParentsTime(t *testing.T) {
	const section = "undated section\n\ntext"
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"without a filter the backup's later copy is recovered", nil, []string{
			"2026-01-01T00:00:00.000Z  " + section, "2026-06-01T00:00:00.000Z yes " + section}},
		{"a window that keeps the later row shows the section once", []string{"--since", "2026-03-01"},
			[]string{"2026-06-01T00:00:00.000Z  " + section}},
		{"a pattern only the later row matches shows the section once", []string{"--match", "new page|section"},
			[]string{"2026-06-01T00:00:00.000Z  " + section}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			work := t.TempDir()
			collection := writeCollection(t, filepath.Join(work, "col"),
				[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, []collectedFile{
					{"alice", "/home/alice", "tabby", ".tabby/ee/db.sqlite", tabbyRepeatedDatabase(t)},
					{"alice", "/home/alice", "tabby", ".tabby/ee/db.backup-2026.sqlite", tabbyBackup(t)}})
			env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
				Parsers: parsers.New, Version: "0.0.0-test"}
			output := filepath.Join(work, "out")
			args := append(append([]string{"timeline"}, test.options...), collection, output)
			if code, _, stderr := runCommand(env, args...); code != ExitFound {
				t.Fatalf("%q exited %d: %s", args, code, stderr)
			}
			var got []string
			for _, row := range recordsOf(t, filepath.Join(output, TimelineFile), "text", "timestamp_utc",
				"recovered", "text") {
				if strings.Contains(row, "undated section") {
					got = append(got, row)
				}
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}
