package tabby

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// parseTables builds a server database from tables and returns the texts
// of its rows; the builder writes each value as given, so a number stays a
// number in a column of another type, as only damage leaves one.
func parseTables(t *testing.T, tables ...sqlitetest.Table) []string {
	t.Helper()
	data, err := sqlitetest.Build(tables...)
	if err != nil {
		t.Fatal(err)
	}
	return parseFile(t, ".tabby/ee/db.sqlite", string(data))
}

// An id joins only what SQL's = would join between the two columns: NULL
// joins nothing, a number never its text, and text in an id column
// declared COLLATE NOCASE joins its other cases.
func TestAnIDJoinsOnlyWhatTheColumnsCompareEqual(t *testing.T) {
	const created = "2026-01-01 00:00:00"
	tests := []struct {
		name   string
		tables []sqlitetest.Table
		want   []string
	}{
		{"a message whose thread_id is NULL is not a NULL-id thread's", []sqlitetest.Table{
			{Name: "threads", SQL: "CREATE TABLE threads (id, created_at TIMESTAMP)", Rows: [][]any{{nil, created}}},
			{Name: "thread_messages", SQL: "CREATE TABLE thread_messages (id INTEGER, thread_id, role TEXT, " +
				"content TEXT)", Rows: [][]any{{1, nil, "user", "hi"}}}},
			[]string{"thread start", "thread start | thread row missing", "hi"}},
		{"a user_id that is text finds no user whose id is that number", []sqlitetest.Table{
			{Name: "users", SQL: "CREATE TABLE users (id, email TEXT, name TEXT, auth_token TEXT, created_at TEXT)",
				Rows: [][]any{{1, "a@example.com", "A", nil, created}}},
			{Name: "threads", SQL: "CREATE TABLE threads (id, user_id, created_at TIMESTAMP)",
				Rows: [][]any{{7, "1", created}}}},
			[]string{"thread start | owner user_id=1"}},
		{"a page_id in another case finds the page whose id ignores case", []sqlitetest.Table{
			{Name: "pages", SQL: "CREATE TABLE pages (id TEXT COLLATE NOCASE, title TEXT, created_at TIMESTAMP)",
				Rows: [][]any{{"P", "t", created}}},
			{Name: "page_sections", SQL: "CREATE TABLE page_sections (id INTEGER, page_id TEXT, title TEXT, " +
				"content TEXT, position INTEGER)", Rows: [][]any{{1, "p", "sec", "body", 0}}}},
			[]string{"page start | title t", "sec\n\nbody"}},
		{"a page_id in another case is another page under BINARY", []sqlitetest.Table{
			{Name: "pages", SQL: "CREATE TABLE pages (id TEXT, title TEXT, created_at TIMESTAMP)",
				Rows: [][]any{{"P", "t", created}}},
			{Name: "page_sections", SQL: "CREATE TABLE page_sections (id INTEGER, page_id TEXT, title TEXT, " +
				"content TEXT, position INTEGER)", Rows: [][]any{{1, "p", "sec", "body", 0}}}},
			[]string{"page start | title t", "page start | page row missing", "sec\n\nbody"}},
		{"two cases of a page id ignoring case are one repeated id", []sqlitetest.Table{
			{Name: "pages", SQL: "CREATE TABLE pages (id TEXT COLLATE NOCASE, title TEXT, created_at TIMESTAMP)",
				Rows: [][]any{{"P", "one", created}, {"p", "two", created}}},
			{Name: "page_sections", SQL: "CREATE TABLE page_sections (id INTEGER, page_id TEXT, title TEXT, " +
				"content TEXT, position INTEGER)", Rows: [][]any{{1, "p", "sec", "body", 0}}}},
			[]string{"page start | title one", "sec\n\nbody", "page start | title two",
				"parser: pages holds 1 row repeating an earlier row's id; its sections are shown once"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseTables(t, test.tables...); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

// A repository whose integration_id is NULL names no integration, not one
// whose id is NULL too.
func TestARepositoryWithANullIntegrationIDNamesNoIntegration(t *testing.T) {
	rows := parseTables(t,
		sqlitetest.Table{Name: "integrations", SQL: "CREATE TABLE integrations (id, kind TEXT, display_name TEXT, " +
			"api_base TEXT, created_at TEXT)", Rows: [][]any{{nil, "github", "Hub", nil, "2026-01-01 00:00:00"}}},
		sqlitetest.Table{Name: "provided_repositories", SQL: "CREATE TABLE provided_repositories (id INTEGER, " +
			"integration_id, name TEXT, git_url TEXT, active INTEGER, updated_at TEXT)",
			Rows: [][]any{{1, nil, "r", "https://example.com/r.git", 1, "2026-01-02 00:00:00"}}})
	for _, row := range rows {
		if strings.HasPrefix(row, "source host repository:") && !strings.Contains(row, "(integration integration_id=;") {
			t.Errorf("the repository row %q names an integration", row)
		}
	}
}

// A row naming a parent no stored row holds is shown, even when the
// parent's table is gone: a message under a thread start stand-in saying
// the thread row is missing, a section under a page start stand-in
// saying the same of the page (the database has no pages table), a
// thread naming no user by the user's id, and a repository naming no
// integration by the integration's id (it has no integrations table).
func TestARowNamingAMissingParentIsShown(t *testing.T) {
	const created = "2026-01-01 00:00:00"
	rows := parseTables(t,
		sqlitetest.Table{Name: "users", SQL: "CREATE TABLE users (id INTEGER, email TEXT, name TEXT, " +
			"auth_token TEXT, created_at TEXT)", Rows: [][]any{{1, "a@example.com", "A", nil, created}}},
		sqlitetest.Table{Name: "threads", SQL: "CREATE TABLE threads (id INTEGER, user_id INTEGER, " +
			"created_at TIMESTAMP)", Rows: [][]any{{7, 9, created}}},
		sqlitetest.Table{Name: "thread_messages", SQL: "CREATE TABLE thread_messages (id INTEGER, thread_id INTEGER, " +
			"role TEXT, content TEXT)", Rows: [][]any{{1, 8, "user", "lost thread"}}},
		sqlitetest.Table{Name: "page_sections", SQL: "CREATE TABLE page_sections (id INTEGER, page_id INTEGER, " +
			"title TEXT, content TEXT, position INTEGER)", Rows: [][]any{{1, 3, "sec", "body", 0}}},
		sqlitetest.Table{Name: "provided_repositories", SQL: "CREATE TABLE provided_repositories (id INTEGER, " +
			"integration_id INTEGER, name TEXT, git_url TEXT, active INTEGER, updated_at TEXT)",
			Rows: [][]any{{1, 5, "r", "https://example.com/r.git", 1, created}}})
	want := []string{"thread start | owner user_id=9", "thread start | thread row missing", "lost thread",
		"page start | page row missing", "sec\n\nbody",
		"source host repository: https://example.com/r.git type=- name=r active=yes (integration integration_id=5; " +
			"from provided_repositories table, updated_at)"}
	if !slices.Equal(rows, want) {
		t.Errorf("rows %q, want %q", rows, want)
	}
}

// A message whose database has no threads table is shown under a thread
// start stand-in saying the thread row is missing.
func TestAMessageWithNoThreadsTableIsShown(t *testing.T) {
	rows := parseTables(t, sqlitetest.Table{Name: "thread_messages", SQL: "CREATE TABLE thread_messages (id INTEGER, " +
		"thread_id INTEGER, role TEXT, content TEXT)", Rows: [][]any{{1, 8, "user", "lost thread"}}})
	if want := []string{"thread start | thread row missing", "lost thread"}; !slices.Equal(rows, want) {
		t.Errorf("rows %q, want %q", rows, want)
	}
}
