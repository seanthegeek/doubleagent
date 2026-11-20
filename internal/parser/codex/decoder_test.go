package codex

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The compressed rollouts of a run share the parser's one zstd decoder, so
// its buffers are made once: one decoder is made however many files are
// read, and each file after the first allocates its records and rows, not
// a decoder's buffers again (runtime.MemStats.TotalAlloc, not timed).
func TestTheCompressedRolloutsOfARunShareOneDecoder(t *testing.T) {
	frame := readTestdata(t, "two-lines.jsonl.zst")
	const perFile = 32 << 10
	allocated := map[int]uint64{}
	for _, n := range []int{10, 40} {
		home := t.TempDir()
		subject := New()
		subject.Work = parser.CountingWork()
		var artifacts []*input.Artifact
		for i := range n {
			rel := fmt.Sprintf(".codex/sessions/2026/01/02/rollout-2026-01-02T00-00-00-%d.jsonl.zst", i)
			path := filepath.Join(home, filepath.FromSlash(rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(path, frame, 0o600); err != nil {
				t.Fatal(err)
			}
			artifacts = append(artifacts, &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel,
				Home: &input.Home{DiskPath: home}})
		}
		rows := 0
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		for _, artifact := range artifacts {
			if err := subject.Parse(artifact, parser.Options{}, func(timeline.Row) { rows++ }); err != nil {
				t.Fatal(err)
			}
		}
		runtime.ReadMemStats(&after)
		allocated[n] = after.TotalAlloc - before.TotalAlloc
		if made := subject.Work.Count("zstd decoders made"); made != 1 || rows == 0 {
			t.Errorf("%d files gave %d rows after %d decoders were made, want rows after one", n, rows, made)
		}
	}
	if growth := allocated[40] - allocated[10]; growth > 30*perFile {
		t.Errorf("30 more files allocated %d more bytes, more than %d each", growth, perFile)
	}
}
