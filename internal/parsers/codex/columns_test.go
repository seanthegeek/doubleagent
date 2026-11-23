package codex

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parserkit/parsertest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// threadColumnLength is the length of the cwd and branch of the thread of
// the databases testdata/thread_columns.py writes.
const threadColumnLength = 50_000

// A thread's cwd and branch are stamped on every row of the thread, so
// they are made into strings once per thread and the rows share them:
// twice the feedback records allocate twice the bytes of their rows, not a
// copy of both columns per row (runtime.MemStats.TotalAlloc, not timed).
func TestAThreadsColumnsAreConvertedOncePerThread(t *testing.T) {
	const rel = ".codex/state_5.sqlite"
	allocated := map[int]int64{}
	for _, records := range []int{20, 40} {
		data, err := osfile.ReadFile(filepath.Join("testdata", fmt.Sprintf("thread_columns_%d.db", records)))
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
		artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "codex-cli",
			Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
		branches := 0
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		if err := parsertest.ParseWithoutSecrets(t, New(), artifact, parserkit.Options{}, func(row timeline.Row) {
			if row.GitBranch != "" && len(row.GitBranch) <= parserkit.CarriedLimit {
				branches++
			}
		}); err != nil {
			t.Fatal(err)
		}
		runtime.ReadMemStats(&after)
		allocated[records] = int64(after.TotalAlloc - before.TotalAlloc)
		// The thread's record row and one row per feedback record.
		if branches != records+1 {
			t.Errorf("%d records gave %d rows with the branch cut, want %d", records, branches, records+1)
		}
	}
	if growth := allocated[40] - allocated[20]; growth > 5*threadColumnLength {
		t.Errorf("20 more records allocated %d more bytes, more than five copies of a %d-byte column", growth,
			threadColumnLength)
	}
}
