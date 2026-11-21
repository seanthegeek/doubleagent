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

// tabbyRepeatedUser returns a Tabby db.sqlite whose users table holds the
// id 1 in two rows, as only damage writes: an old one, mallory, and one
// five months later, alice, each with an API token; and a thread, a page
// and a user event of the user, a day after the later row.
func tabbyRepeatedUser(t *testing.T) string {
	t.Helper()
	data, err := sqlitetest.Build(
		sqlitetest.Table{Name: "users", SQL: "CREATE TABLE users (id INTEGER, email TEXT, name TEXT, " +
			"auth_token TEXT, created_at TIMESTAMP)", Rows: [][]any{
			{1, "mallory@example.com", "Mallory", "token-old", "2026-01-01 00:00:00"},
			{1, "alice@example.com", "Alice", "token-new", "2026-06-01 00:00:00"}}},
		sqlitetest.Table{Name: "threads", SQL: "CREATE TABLE threads (id INTEGER, user_id INTEGER, " +
			"created_at TIMESTAMP)", Rows: [][]any{{7, 1, "2026-06-02 00:00:00"}}},
		sqlitetest.Table{Name: "pages", SQL: "CREATE TABLE pages (id INTEGER, author_id INTEGER, title TEXT, " +
			"created_at TIMESTAMP)", Rows: [][]any{{8, 1, "notes", "2026-06-02 00:00:00"}}},
		sqlitetest.Table{Name: "user_events", SQL: "CREATE TABLE user_events (id INTEGER, user_id INTEGER, " +
			"kind TEXT, created_at TIMESTAMP, payload TEXT)", Rows: [][]any{
			{9, 1, "view", "2026-06-02 00:00:00", `{"view":{"completion_id":"c1","choice_index":0}}`}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// tabbyUserRun runs timeline over a collection holding the repeated user
// id's database with the options given, and returns each row as "time
// text".
func tabbyUserRun(t *testing.T, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "tabby", ".tabby/ee/db.sqlite", tabbyRepeatedUser(t)}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline", "--agent", "tabby"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, TimelineFile), "text", "timestamp_utc", "text")
}

// A user's label is text inside the rows of the threads, pages and events
// that name the user, rows of other ids, so a users table that holds the
// user's id in more than one row lends none of its stored rows' labels:
// those rows name the user by id, and each stored row gives a row naming
// its own label, whatever the filters keep.
func TestARepeatedTabbyUserIDLendsNoStoredRowsLabel(t *testing.T) {
	const (
		old     = "2026-01-01T00:00:00.000Z "
		current = "2026-06-01T00:00:00.000Z "
		joined  = "2026-06-02T00:00:00.000Z "
		notice  = "parser: users holds 2 rows of this row's id; the rows of its threads, pages and events " +
			"name the user by id alone, and this row's label is "
	)
	oldRows := []string{old + "tabby user: mallory@example.com (Mallory) id=1 auth_token=token-old " +
		"(from users table, created_at)", old + notice + "mallory@example.com (Mallory)"}
	newRows := []string{current + "tabby user: alice@example.com (Alice) id=1 auth_token=token-new " +
		"(from users table, created_at)", current + notice + "alice@example.com (Alice)"}
	named := []string{joined + "thread start | owner user_id=1", joined + "page start | title notes | author user_id=1"}
	tests := []struct {
		name    string
		options []string
		want    []string
	}{
		{"without a filter", nil, slices.Concat(oldRows, newRows, named)},
		{"with a window that keeps only the later row", []string{"--since", "2026-03-01"},
			slices.Concat(newRows, named)},
		{"with a pattern only the earlier row matches", []string{"--match", "Mallory|start"},
			slices.Concat(oldRows, named)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := tabbyUserRun(t, test.options...)
			var kept []string
			for _, row := range rows {
				if !strings.Contains(row, "view") {
					kept = append(kept, row)
				}
			}
			if !slices.Equal(kept, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(kept, "\n"), strings.Join(test.want, "\n"))
			}
			for _, row := range rows {
				if strings.Contains(row, "view") && !strings.Contains(row, "user=1") {
					t.Errorf("the event row %q names a stored row's label", row)
				}
			}
		})
	}
}
