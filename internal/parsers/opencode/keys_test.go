package opencode

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// keysRows builds a database from tables and returns each row the parser
// gives as "session project text"; the builder writes each value as
// given, so a number stays a number in a TEXT column, as only damage
// leaves one.
func keysRows(t *testing.T, tables ...sqlitetest.Table) []string {
	t.Helper()
	data, err := sqlitetest.Build(tables...)
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, row := range parseBytesRows(t, data, secrets.Setting{}) {
		rows = append(rows, row.SessionID+" "+row.ProjectPath+" "+row.Text)
	}
	return rows
}

// keysSessionTable returns a 1.x session table whose id is declared with
// the collation given, holding the rows (id, directory, title).
func keysSessionTable(collation string, rows ...[]any) sqlitetest.Table {
	return sqlitetest.Table{Name: "session", SQL: "CREATE TABLE session (id TEXT" + collation + ", directory TEXT, " +
		"title TEXT, time_created INTEGER)", Rows: rows}
}

// keysMessage returns the 1.x message and part tables holding one user
// message of the session id given, with the text HELLO.
func keysMessage(sessionID any) []sqlitetest.Table {
	return []sqlitetest.Table{
		{Name: "message", SQL: "CREATE TABLE message (id TEXT, session_id TEXT, time_created INTEGER, data TEXT)",
			Rows: [][]any{{"msg_1", sessionID, 1700000000500, `{"role":"user"}`}}},
		{Name: "part", SQL: "CREATE TABLE part (id TEXT, message_id TEXT, session_id TEXT, time_created INTEGER, " +
			"data TEXT)", Rows: [][]any{{"prt_1", "msg_1", sessionID, 1700000000500, `{"type":"text","text":"HELLO"}`}}},
	}
}

// helloRow returns the row of rows whose text is HELLO, "" when none is.
func helloRow(rows []string) string {
	for _, row := range rows {
		if strings.HasSuffix(row, " HELLO") {
			return row
		}
	}
	return ""
}

// A message's session_id names only the session whose id SQL's = finds
// equal to it in the session table's id column: NULL names none, a number
// never its text, and text in an id declared COLLATE NOCASE its other
// cases.
func TestAMessageJoinsOnlyTheSessionTheIDColumnFindsEqual(t *testing.T) {
	tests := []struct {
		name      string
		collation string
		session   any
		reference any
		want      string
	}{
		{"a NULL session_id names no session whose id is NULL", "", nil, nil, "  HELLO"},
		{"a number names no session whose id is its text", "", "1", int64(1), "1  HELLO"},
		{"another case names no session under BINARY", "", "S1", "s1", "s1  HELLO"},
		{"another case names the session under NOCASE", " COLLATE NOCASE", "S1", "s1", "S1 /proj HELLO"},
		{"the same text names the session", "", "s1", "s1", "s1 /proj HELLO"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := keysRows(t, append([]sqlitetest.Table{keysSessionTable(test.collation,
				[]any{test.session, "/proj", "T", 1700000000000})}, keysMessage(test.reference)...)...)
			if got := helloRow(rows); got != test.want {
				t.Errorf("message row %q, want %q, in %q", got, test.want, rows)
			}
		})
	}
}

// A session id is repeated only when SQL's = finds two stored rows' ids
// equal: a number and its text are two sessions, and so are two cases of
// a text unless the id is declared COLLATE NOCASE; two NULL ids are two.
func TestASessionIDRepeatsOnlyWhenTheColumnComparesItsRowsEqual(t *testing.T) {
	tests := []struct {
		name          string
		collation     string
		first, second any
		repeated      bool
	}{
		{"a number and its text", "", int64(1), "1", false},
		{"two cases under BINARY", "", "S", "s", false},
		{"two cases under NOCASE", " COLLATE NOCASE", "S", "s", true},
		{"two NULL ids", "", nil, nil, false},
		{"the same text", "", "s", "s", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := keysRows(t, keysSessionTable(test.collation, []any{test.first, "/a", "A", 1700000000000},
				[]any{test.second, "/b", "B", 1700000001000}))
			starts := 0
			repeated := slices.ContainsFunc(rows, func(row string) bool {
				return strings.Contains(row, "parser: session holds 2 rows")
			})
			for _, row := range rows {
				if strings.Contains(row, "session start:") {
					starts++
				}
			}
			if repeated != test.repeated || starts != 2 {
				t.Errorf("repeated %v with %d start rows, want %v with 2, in %q", repeated, starts, test.repeated, rows)
			}
		})
	}
}

// A session_part item belongs only to the session_message record whose id
// SQL's = finds equal to its message_id: an item whose message_id is NULL
// belongs to no record, not one whose id is NULL too, and is shown as an
// item of no record, counted in one parser: row.
func TestAnItemWithANullMessageIDBelongsToNoRecord(t *testing.T) {
	rows := keysRows(t,
		sqlitetest.Table{Name: "session_v2", SQL: "CREATE TABLE session_v2 (id TEXT, directory TEXT, title TEXT, " +
			"time_created INTEGER)", Rows: [][]any{{"s1", "/proj", "T", 1700000000000}}},
		sqlitetest.Table{Name: "session_message", SQL: "CREATE TABLE session_message (id TEXT, session_id TEXT, " +
			"type TEXT, seq INTEGER, time_created INTEGER, data TEXT)", Rows: [][]any{
			{nil, "s1", "assistant", 0, 1700000000500, `{"content":[{"type":"text","text":"OWN"}]}`}}},
		sqlitetest.Table{Name: "session_part", SQL: "CREATE TABLE session_part (id TEXT, message_id TEXT, " +
			"session_id TEXT, type TEXT, ordinal INTEGER, data TEXT)", Rows: [][]any{
			{"prt_1", nil, "s1", "text", 0, `{"text":"ITEM"}`}}})
	want := []string{"s1 /proj session start: T directory=/proj", "s1 /proj OWN", "s1 /proj ITEM",
		"  parser: session_part holds 1 row naming no assistant record read here; their rows are shown with " +
			"their own session_id and no record"}
	if !slices.Equal(rows, want) {
		t.Errorf("rows %q, want %q", rows, want)
	}
}
