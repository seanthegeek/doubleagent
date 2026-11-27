package zed

import (
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The databases under testdata are written by SQLite from the rows in
// testdata/fixtures.py.

const threadsRel = ".local/share/zed/threads/threads.db"

// parseThreadsFixture copies the named testdata database into a new home as
// its threads.db and returns the rows subject gives for it.
func parseThreadsFixture(t *testing.T, subject *Parser, name string) []timeline.Row {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(threadsRel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + threadsRel, Rel: threadsRel, Agent: "zed",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	var rows []timeline.Row
	if err := subject.Parse(artifact, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
		t.Fatal(err)
	}
	return rows
}

// isSortedByTime reports whether the rows' timestamps never decrease.
func isSortedByTime(rows []timeline.Row) bool {
	return slices.IsSortedFunc(rows, func(left, right timeline.Row) int {
		return strings.Compare(left.TimestampUTC, right.TimestampUTC)
	})
}

func TestThreadsBeforeDamageAreReadInTimeOrder(t *testing.T) {
	whole := parseThreadsFixture(t, New(), "threads.db")
	if len(whole) != 60 || !isSortedByTime(whole) {
		t.Fatalf("the whole database gives %d rows, sorted %v; want 60 in time order", len(whole),
			isSortedByTime(whole))
	}
	damaged := parseThreadsFixture(t, New(), "damaged_threads.db")
	last := len(damaged) - 1
	if last < 1 || !strings.HasPrefix(damaged[last].Text, "parser: SQLite error: ") {
		t.Fatalf("got %d rows ending %+v; want the threads before the damage, then its problem row",
			len(damaged), damaged[last])
	}
	threads := damaged[:last]
	if !isSortedByTime(threads) {
		t.Errorf("the threads read are not in time order: %+v", threads)
	}
	for _, row := range threads {
		if !slices.Contains(whole, row) {
			t.Errorf("%+v is not a row of the whole database", row)
		}
	}
}

func TestTheDecompressionBudgetOfADatabaseIsRatioTimesItsSizeAndAtLeastTheFloor(t *testing.T) {
	tests := []struct {
		name  string
		size  int64
		floor int
		want  int
	}{
		{"an empty database has the floor", 0, 100, 100},
		{"a small database has the floor", 1, 100, 100},
		{"a large database has the ratio times its size", 10, 100, 10 * BudgetRatio},
		{"a size the ratio would overflow has no limit", math.MaxInt64, 100, math.MaxInt},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NewBudget(test.size, test.floor); got.total != test.want || got.left != test.want {
				t.Errorf("got %+v, want %d", got, test.want)
			}
		})
	}
}

// largeFrames returns the zstd blobs of large_threads.db, each of which
// decompresses to 40000 bytes, and the database's size.
func largeFrames(t *testing.T) ([][]byte, int64) {
	t.Helper()
	database, err := sqlitefile.OpenPath(filepath.Join("testdata", "large_threads.db"), osfile.ListSpellings)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	table, err := database.Table("threads")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := table.Select("data").All()
	if err != nil || len(rows) != 3 {
		t.Fatalf("got %d rows, %v; want 3", len(rows), err)
	}
	frames := make([][]byte, len(rows))
	for i, row := range rows {
		frames[i] = row.Get("data").Bytes
	}
	return frames, int64(database.PageCount()) * int64(database.PageSize())
}

func TestABudgetRefusesTheBlobThatWouldPassIt(t *testing.T) {
	const decoded = 40000
	frames, _ := largeFrames(t)
	tests := []struct {
		name    string
		total   int
		refused int
	}{
		{"three blobs fit a budget of their size", 3 * decoded, -1},
		{"the third blob passes a budget one byte smaller", 3*decoded - 1, 2},
		{"after a refusal every blob is refused", decoded - 1, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			budget := &Budget{total: test.total, left: test.total}
			for i, frame := range frames {
				out, err := budget.Decompress(frame)
				switch {
				case i < test.refused || test.refused < 0:
					if err != nil || len(out) != decoded {
						t.Errorf("blob %d: got %d bytes, %v; want %d bytes", i, len(out), err, decoded)
					}
				case err == nil || !strings.Contains(err.Error(), fmt.Sprintf("limit of %d bytes", test.total)):
					t.Errorf("blob %d: got %d bytes, %v; want the budget refused", i, len(out), err)
				}
			}
		})
	}
}

func TestAThreadPastTheDatabasesBudgetIsAProblemRow(t *testing.T) {
	_, size := largeFrames(t)
	total := NewBudget(size, 0).total
	if total >= 3*40000 {
		t.Fatalf("the fixture's budget %d holds all three threads", total)
	}
	subject := New()
	subject.budgetFloor = 0
	rows := parseThreadsFixture(t, subject, "large_threads.db")
	want := fmt.Sprintf("parser: unparseable record 3 (zstd: not decompressed: this database's blobs reached "+
		"the limit of %d bytes decompressed in all; stored zstd blob as hex)", total)
	if len(rows) != 3 || !strings.HasPrefix(rows[2].Text, want) {
		t.Fatalf("got %+v, want two threads and %q", rows, want)
	}
	if rows = parseThreadsFixture(t, New(), "large_threads.db"); len(rows) != 3 ||
		!strings.HasPrefix(rows[2].Text, "thread start: large 2") {
		t.Errorf("under the floor got %+v, want three threads", rows)
	}
}
