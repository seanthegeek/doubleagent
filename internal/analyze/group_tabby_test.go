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
