package opencode

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// A part of a 1.x message in a session session_message holds is that
// session's 1.x copy, read from session_message alone, so it is not a
// part of no message; a part of no message whose data does not decode is
// its own bad-line row and is counted too.
func TestOnlyAPartOfNoMessageReadIsCounted(t *testing.T) {
	rows := keysRows(t,
		sqlitetest.Table{Name: "session_v2", SQL: "CREATE TABLE session_v2 (id TEXT, directory TEXT, title TEXT, " +
			"time_created INTEGER)", Rows: [][]any{{"s1", "/proj", "T", 1700000000000}}},
		sqlitetest.Table{Name: "session_message", SQL: "CREATE TABLE session_message (id TEXT, session_id TEXT, " +
			"type TEXT, seq INTEGER, time_created INTEGER, data TEXT)", Rows: [][]any{
			{"r1", "s1", "user", 0, 1700000000500, `{"text":"CURRENT"}`}}},
		sqlitetest.Table{Name: "message", SQL: "CREATE TABLE message (id TEXT, session_id TEXT, time_created INTEGER, " +
			"data TEXT)", Rows: [][]any{{"m1", "s1", 1700000000500, `{"role":"user"}`}}},
		sqlitetest.Table{Name: "part", SQL: "CREATE TABLE part (id TEXT, message_id TEXT, session_id TEXT, " +
			"time_created INTEGER, data TEXT)", Rows: [][]any{
			{"p1", "m1", "s1", 1700000000500, `{"type":"text","text":"LEGACY"}`},
			{"p2", "mX", "s1", 1700000000600, `{"type":"text",`}}})
	for _, row := range rows {
		if strings.Contains(row, "LEGACY") {
			t.Errorf("the 1.x copy of a session read from session_message is shown: %q", rows)
		}
	}
	for _, want := range []string{
		`s1 /proj parser: unparseable record 2 (part.data: invalid JSON at line 1, column 16: unexpected end of ` +
			`JSON input): {"type":"text",`,
		"  parser: part holds 1 row naming no message read here; their rows are shown with their own session_id " +
			"and no message"} {
		if !slices.Contains(rows, want) {
			t.Errorf("no row %q in %q", want, rows)
		}
	}
}
