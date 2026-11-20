package copilotcli

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/parsertest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// sessionColumnLength is the length of the cwd and branch of the session
// of the databases testdata/session_columns.py writes.
const sessionColumnLength = 50_000

// A session's cwd and branch are stamped on every row of the session in
// session-store.db, so they are made into strings once per session and
// the rows share them: twice the turns allocate twice the bytes of their
// rows, not a copy of both columns per row. The bytes are counted
// (runtime.MemStats.TotalAlloc), not timed.
func TestASessionsColumnsAreConvertedOncePerSession(t *testing.T) {
	const rel = ".copilot/session-store.db"
	allocated := map[int]uint64{}
	for _, turns := range []int{20, 40} {
		data, err := osfile.ReadFile(filepath.Join("testdata", fmt.Sprintf("session_columns_%d.db", turns)))
		if err != nil {
			t.Fatal(err)
		}
		home := t.TempDir()
		path := filepath.Join(home, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "copilot-cli",
			Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
		var rows []timeline.Row
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		if err := parsertest.ParseWithoutSecrets(t, New(), artifact, parser.Options{}, func(row timeline.Row) {
			rows = append(rows, row)
		}); err != nil {
			t.Fatal(err)
		}
		runtime.ReadMemStats(&after)
		allocated[turns] = after.TotalAlloc - before.TotalAlloc
		// Two rows per turn, then one problem row for each cut field.
		if len(rows) != 2*turns+2 || len(rows[0].ProjectPath) > parser.CarriedLimit ||
			rows[0].GitBranch != rows[2*turns-1].GitBranch {
			t.Errorf("%d turns gave %d rows, want %d with the branch cut alike", turns, len(rows), 2*turns+2)
		}
	}
	if growth := allocated[40] - allocated[20]; growth > 5*sessionColumnLength {
		t.Errorf("20 more turns allocated %d more bytes, more than five copies of a %d-byte column", growth,
			sessionColumnLength)
	}
}
