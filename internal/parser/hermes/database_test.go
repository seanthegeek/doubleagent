package hermes

import (
	"fmt"
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
	return parseDatabaseAt(t, databaseRel, name)
}

// parseDatabaseAt returns the text of every row the parser gives for the
// named testdata database, placed at rel in a new home.
func parseDatabaseAt(t *testing.T, rel, name string) []string {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return parseFile(t, rel, string(data))
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

// A session id is the sessions table's key, so a repeated one is damage:
// the session's usage rows give their model rows once, after the first row
// of the id in the order the rows are read, and each later row gives its
// own rows, its model from its own model column, and the table one problem
// row, so the rows grow with the table rather than with the square of it.
func TestARepeatedSessionIDGivesItsUsageModelsOnce(t *testing.T) {
	for _, count := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d rows of one id and %d usage rows", count, count), func(t *testing.T) {
			got := parseDatabase(t, fmt.Sprintf("repeated_session_id_%d.db", count))
			want := []string{"session start | copy 0 | source=cli", "model: model-a"}
			for i := 1; i < count; i++ {
				want = append(want, "model: model-"+"ab"[(i-1)%2:(i-1)%2+1]+" -> model-"+"ab"[i%2:i%2+1])
			}
			for i := 1; i < count; i++ {
				want = append(want, fmt.Sprintf("session start | copy %d | source=cli", i))
			}
			want = append(want, fmt.Sprintf("parser: sessions holds %d rows repeating an earlier row's id; "+
				"its usage rows are shown once", count-1))
			if !slices.Equal(got, want) {
				t.Errorf("%d rows %q, want %d rows %q", len(got), got[:min(len(got), 6)], len(want), want[:6])
			}
		})
	}
}

// A response id names one response, so many responses rows of one id are
// damage; the conversation name of that id, written into each of their
// rows, is cut as a carried value, with one problem row, so the output
// grows with the rows rather than with the rows times the name.
func TestAConversationNameIsCutInEveryResponseOfItsID(t *testing.T) {
	for _, count := range []int{10, 40} {
		t.Run(fmt.Sprintf("%d responses of one id", count), func(t *testing.T) {
			rows := parseDatabaseAt(t, ".hermes/response_store.db", fmt.Sprintf("repeated_response_id_%d.db", count))
			whole := strings.Repeat("n", 5000)
			headers, wholeNames, problems := 0, 0, 0
			for _, text := range rows {
				switch {
				case strings.HasPrefix(text, "api server response r1 "):
					headers++
					if len(text) > parser.CarriedLimit+200 {
						t.Errorf("a header of %d bytes", len(text))
					}
				case text == parser.ProblemPrefix+parser.CarriedProblem("conversation name", 1):
					problems++
				}
				wholeNames += strings.Count(text, whole)
			}
			if headers != count || wholeNames != 0 || problems != 1 || len(rows) != count+1 {
				t.Errorf("%d rows: %d headers, %d whole names and %d problems, want %d, 0 and 1",
					len(rows), headers, wholeNames, problems, count)
			}
		})
	}
}
