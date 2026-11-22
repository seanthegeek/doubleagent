package ollama

import (
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// longChatLength is the length of the chat id of the databases
// testdata/long_chat.py writes.
const longChatLength = 20_000

// longChatArtifact copies the named testdata database into a new home and
// returns the artifact.
func longChatArtifact(t *testing.T, name string) *input.Artifact {
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
	return &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "ollama",
		Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
}

// heldDuringParse copies the named testdata database into a new home whose
// secrets recorder, when setting is not nil, follows it, parses it as a
// run does (parser.Collect), and returns the most heap bytes in use, after
// a collection, when the parse emits a row, less those in use before it,
// and the rows it gives.
func heldDuringParse(t *testing.T, name string, setting *secrets.Setting) (int64, int) {
	t.Helper()
	artifact := longChatArtifact(t, name)
	inUse := func() int64 {
		var stats runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&stats)
		return int64(stats.HeapAlloc)
	}
	before := inUse()
	if setting != nil {
		artifact.Home.Secrets = secrets.NewRecorder(*setting)
	}
	most, rows := int64(0), 0
	if _, err := parser.Collect(New(), artifact, parser.Options{}, func(timeline.Row) {
		rows++
		most = max(most, inUse()-before)
	}); err != nil {
		t.Fatal(err)
	}
	return most, rows
}

// Each call's tool_use_id starts with its chat's id, so a long chat id
// with many calls could hold a long value per row. The rows are emitted as
// they are made, once every table is read, and the chat id is cut once for
// all its calls, so neither the rows nor the secrets recorder of a run
// that withholds secrets keep a copy of the chat id per call: twice the
// calls keep far less than one more copy per added row. The bytes in use
// are counted (runtime.MemStats.HeapAlloc), not timed.
func TestTheRowsOfACallAreNotHeldUntilTheParseEnds(t *testing.T) {
	tests := []struct {
		name    string
		setting *secrets.Setting
	}{
		{"with no recorder", nil},
		{"with a recorder withholding secrets", &secrets.Setting{Withhold: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var held []int64
			for _, count := range []int{20, 40} {
				bytes, rows := heldDuringParse(t, fmt.Sprintf("long_chat_%d.db", count), test.setting)
				// The message's two rows, a row per call, and the problems of the
				// cut tool_use_id and session_id.
				if rows != count+4 {
					t.Errorf("%d calls gave %d rows, want %d", count, rows, count+4)
				}
				held = append(held, bytes)
			}
			// Holding a copy per row would add 20 copies; a quarter copy per
			// added row is the bound.
			if growth := held[1] - held[0]; growth > 5*longChatLength {
				t.Errorf("20 more calls held %d more bytes, more than a quarter of the %d-byte chat id per row",
					growth, longChatLength)
			}
		})
	}
}

// A call's tool_use_id is its chat's id, a colon and the call's id. A chat
// id too long for the field with the call's id after it is cut once, to
// leave room for the call's id, so each of the 20 calls keeps its own
// tool_use_id within the bound, with one problem row counting the cuts,
// as research/ollama.md says; the chat id in each row's session_id is cut
// as every long field is.
func TestTheCallsOfALongChatKeepDistinctIDsWithinTheBound(t *testing.T) {
	ids := map[string]int{}
	var problems []string
	if _, err := parser.Collect(New(), longChatArtifact(t, "long_chat_20.db"), parser.Options{},
		func(row timeline.Row) {
			if row.ToolUseID != "" {
				ids[row.ToolUseID]++
			}
			if strings.HasPrefix(row.Text, parser.ProblemPrefix) {
				problems = append(problems, row.Text)
			}
		}); err != nil {
		t.Fatal(err)
	}
	mark := " [cut from " + strconv.Itoa(longChatLength) + " bytes]"
	chat := strings.Repeat("c", parser.CarriedLimit-callIDRoom-len(mark)) + mark
	for call := 1; call <= 20; call++ {
		toolUseID := chat + ":" + strconv.Itoa(call)
		if ids[toolUseID] != 1 || len(toolUseID) > parser.CarriedLimit {
			t.Errorf("call %d: its tool_use_id of %d bytes is on %d rows, want one", call, len(toolUseID),
				ids[toolUseID])
		}
	}
	want := []string{parser.ProblemPrefix + parser.CarriedProblem("tool_use_id", 20),
		parser.ProblemPrefix + parser.CarriedProblem("session_id", 22)}
	if len(ids) != 20 || !slices.Equal(problems, want) {
		t.Errorf("%d tool_use_ids and problems %q, want 20 and %q", len(ids), problems, want)
	}
}
