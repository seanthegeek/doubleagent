package zed

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
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
	if err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) { rows = append(rows, row) }); err != nil {
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
