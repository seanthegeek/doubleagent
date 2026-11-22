package copilotdesktop

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// longPathLength is the length of the project path of the databases
// testdata/long_path.py writes.
const longPathLength = 20_000

// allocatedByParse copies the named testdata database into a new home and
// returns the bytes one parse of it allocates and the rows it gives.
func allocatedByParse(t *testing.T, name string) (uint64, int) {
	t.Helper()
	const rel = ".copilot/data.db"
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
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
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "copilot-desktop",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	rows := 0
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) {
		if row.ProjectPath != "" && len(row.ProjectPath) != longPathLength {
			t.Errorf("a row's project path is %d bytes, want %d", len(row.ProjectPath), longPathLength)
		}
		rows++
	}); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc, rows
}

// A project's path is carried by every workspace, worktree and session row
// of the project, and the rows are held until the parse ends, so the path
// is made into a string once and the rows share it: twice the sessions and
// worktrees cost far less than one more copy of the path per added row.
// The bytes allocated are counted (runtime.MemStats.TotalAlloc), not timed.
func TestAProjectPathIsSharedByTheRowsItIsJoinedTo(t *testing.T) {
	var allocated []uint64
	for _, count := range []int{20, 40} {
		t.Run(fmt.Sprintf("%d sessions and %d worktrees", count, count), func(t *testing.T) {
			bytes, rows := allocatedByParse(t, fmt.Sprintf("long_path_%d.db", count))
			if rows != 2*count+2 {
				t.Errorf("%d rows, want %d", rows, 2*count+2)
			}
			allocated = append(allocated, bytes)
		})
	}
	// A copy per row would add 40 copies; half a copy per added row is the
	// bound, far above what the rows themselves take.
	if growth := int64(allocated[1]) - int64(allocated[0]); growth > 20*longPathLength {
		t.Errorf("40 more rows allocated %d bytes, more than half the %d-byte path per row", growth,
			longPathLength)
	}
}
