package hermes

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// The databases under testdata are written by SQLite from the SQL in
// testdata/fixtures.py.

const databaseRel = ".hermes/state.db"

// parseDatabase returns the text of every row the parser gives for the
// named testdata database, placed at databaseRel in a new home.
func parseDatabase(t *testing.T, name string) []string {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return parseFile(t, databaseRel, string(data))
}

// A virtual generated column is computed when read and not stored, so
// reading every column of its table leaves it out and reads the rest.
func TestATableWithAVirtualGeneratedColumnIsRead(t *testing.T) {
	rows := parseDatabase(t, "generated_column.db")
	if !slices.ContainsFunc(rows, func(text string) bool { return strings.Contains(text, "hello") }) ||
		slices.ContainsFunc(rows, func(text string) bool { return strings.HasPrefix(text, parser.ProblemPrefix) }) {
		t.Errorf("got %q, want the message and no problem", rows)
	}
}

// Room events are read with their rowids, so a table without rowids is an
// error, as a query naming the rowid is, not rows numbered 0.
func TestRoomEventsInATableWithoutRowidsAreAnError(t *testing.T) {
	rows := parseDatabase(t, "room_events_without_rowid.db")
	want := []string{parser.ProblemPrefix + "SQLite error: table hosted_room_events: no such column: rowid"}
	if !slices.Equal(rows, want) {
		t.Errorf("got %q, want %q", rows, want)
	}
}
