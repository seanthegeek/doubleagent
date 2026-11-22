package tabby

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// A thread or page id is its table's key, so a repeated one is damage: the
// thread's messages and the page's sections are shown once, with the first
// row of the id in the order the rows are read, and each later row gives
// its start row alone and adds to one problem row for its table, so the
// rows grow with the table rather than with the square of it. The id 1
// and the text '1' are two ids, as SQL's = finds them in a column of no
// declared type (sqlite3: SELECT 1 = '1' between two such columns gives
// 0), so the last thread row, whose id is the text, is a thread of its own
// with no message and no repeat.
func TestARepeatedThreadOrPageIDShowsItsChildrenOnce(t *testing.T) {
	for _, count := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d rows of each id", count), func(t *testing.T) {
			got := parseDatabase(t, fmt.Sprintf("repeated_ids_%d.db", count))
			want := []string{"thread start"}
			for i := range count {
				want = append(want, fmt.Sprintf("message %d", i))
			}
			for range count - 1 {
				want = append(want, "thread start")
			}
			want = append(want, "page start | title copy 0")
			for i := range count {
				want = append(want, fmt.Sprintf("section %d\n\nbody", i))
			}
			for i := 1; i < count; i++ {
				want = append(want, fmt.Sprintf("page start | title copy %d", i))
			}
			want = append(want,
				fmt.Sprintf("parser: threads holds %d rows repeating an earlier row's id; "+
					"its messages are shown once", count-2),
				fmt.Sprintf("parser: pages holds %d rows repeating an earlier row's id; "+
					"its sections are shown once", count-1))
			if !slices.Equal(got, want) {
				t.Errorf("%d rows ending %q, want %d rows ending %q", len(got), got[max(0, len(got)-3):],
					len(want), want[len(want)-3:])
			}
		})
	}
}

// An integration id is its table's key, so a repeated one is damage. Its
// name and kind are text inside the rows of the repositories cached from
// it, rows of other ids, so no stored row lends them: the repositories
// name the integration by id, and each stored row gives its own row and a
// parser: row with its own name and kind.
func TestARepeatedIntegrationIDLendsNoStoredRowsName(t *testing.T) {
	rows := parseTables(t,
		sqlitetest.Table{Name: "integrations", SQL: "CREATE TABLE integrations (id INTEGER, kind TEXT, " +
			"display_name TEXT, api_base TEXT, created_at TEXT)", Rows: [][]any{
			{1, "github", "Planted Hub", nil, "2026-01-01 00:00:00"},
			{1, "gitlab", "Real Lab", nil, "2026-06-01 00:00:00"}}},
		sqlitetest.Table{Name: "provided_repositories", SQL: "CREATE TABLE provided_repositories (id INTEGER, " +
			"integration_id INTEGER, name TEXT, git_url TEXT, active INTEGER, updated_at TEXT)",
			Rows: [][]any{{1, 1, "r", "https://example.com/r.git", 1, "2026-06-02 00:00:00"}}})
	const notice = "parser: integrations holds 2 rows of this row's id; the repositories cached from it name " +
		"the integration by id alone, and this row's name is "
	want := []string{
		"source host: github.com integration Planted Hub kind=github api_base=- (account not recorded; " +
			"from integrations table, created_at)", notice + "Planted Hub and its kind github",
		"source host: gitlab.com integration Real Lab kind=gitlab api_base=- (account not recorded; " +
			"from integrations table, created_at)", notice + "Real Lab and its kind gitlab",
		"source host repository: https://example.com/r.git type=- name=r active=yes (integration " +
			"integration_id=1; from provided_repositories table, updated_at)"}
	if !slices.Equal(rows, want) {
		t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(want, "\n"))
	}
}

// The stored rows of the threads and pages tables are read oldest first by
// created_at, then by id, then in the order the table holds them, so the
// children of a repeated id follow its oldest row, though the table holds
// a newer row first.
func TestTheChildrenOfARepeatedIDFollowItsOldestRow(t *testing.T) {
	rows := parseTables(t,
		sqlitetest.Table{Name: "pages", SQL: "CREATE TABLE pages (id INTEGER, title TEXT, created_at TIMESTAMP)",
			Rows: [][]any{{2, "newer", "2026-06-01 00:00:00"}, {2, "older", "2026-01-01 00:00:00"}}},
		sqlitetest.Table{Name: "page_sections", SQL: "CREATE TABLE page_sections (id INTEGER, page_id INTEGER, " +
			"title TEXT, content TEXT, position INTEGER)", Rows: [][]any{{1, 2, "sec", "body", 0}}})
	want := []string{"page start | title older", "sec\n\nbody", "page start | title newer",
		"parser: pages holds 1 row repeating an earlier row's id; its sections are shown once"}
	if !slices.Equal(rows, want) {
		t.Errorf("rows %q, want %q", rows, want)
	}
}
