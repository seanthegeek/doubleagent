package ollama

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

// longChatLength is the length of the chat id of the databases
// testdata/long_chat.py writes.
const longChatLength = 20_000

// heldDuringParse copies the named testdata database into a new home and
// returns the most heap bytes in use, after a collection, when the parse
// emits a row, less those in use before it, and the rows it gives.
func heldDuringParse(t *testing.T, name string) (int64, int) {
	t.Helper()
	const rel = "Library/Application Support/Ollama/db.sqlite"
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
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "ollama",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
	inUse := func() int64 {
		var stats runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&stats)
		return int64(stats.HeapAlloc)
	}
	before, most, rows := inUse(), int64(0), 0
	if err := New().Parse(artifact, parser.Options{}, func(timeline.Row) {
		rows++
		most = max(most, inUse()-before)
	}); err != nil {
		t.Fatal(err)
	}
	return most, rows
}

// Each call's tool_use_id is built from its chat's id, so a long chat id
// with many calls makes a long value per row. The rows are emitted as they
// are made, once every table is read, so no row's value is held until the
// parse ends: twice the calls keep far less than one more copy of the chat
// id per added row in use. The bytes in use are counted
// (runtime.MemStats.HeapAlloc), not timed.
func TestTheRowsOfACallAreNotHeldUntilTheParseEnds(t *testing.T) {
	var held []int64
	for _, count := range []int{20, 40} {
		t.Run(fmt.Sprintf("%d calls", count), func(t *testing.T) {
			bytes, rows := heldDuringParse(t, fmt.Sprintf("long_chat_%d.db", count))
			if rows != count+2 {
				t.Errorf("%d rows, want %d", rows, count+2)
			}
			held = append(held, bytes)
		})
	}
	// Holding every row would add 20 copies; a quarter copy per added row
	// is the bound.
	if growth := held[1] - held[0]; growth > 5*longChatLength {
		t.Errorf("20 more calls held %d more bytes, more than a quarter of the %d-byte chat id per row", growth,
			longChatLength)
	}
}
