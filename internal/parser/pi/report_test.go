package pi

import (
	"runtime"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// failedTurns returns diagnostics holding n failed assistant turns.
func failedTurns(t *testing.T, n int) jsonvalue.Value {
	t.Helper()
	turns := make([]string, n)
	for i := range turns {
		turns[i] = `{"entryId": "e", "timestamp": "t", "stopReason": "error", "errorMessage": "boom"}`
	}
	diagnostics, err := jsonvalue.DecodeString(`{"assistant": [` + strings.Join(turns, ",") + `]}`)
	if err != nil {
		t.Fatal(err)
	}
	return diagnostics
}

func TestABugReportsTextIsBuiltWithoutCopyingItPerTurn(t *testing.T) {
	const turns = 4000
	diagnostics := failedTurns(t, turns)
	var before, after runtime.MemStats
	var problems parser.Problems
	runtime.ReadMemStats(&before)
	text := bugReportRowText(jsonvalue.NewObject(), diagnostics, "(from r)", &problems)
	runtime.ReadMemStats(&after)
	if got := strings.Count(text, "\nfailed assistant turn e at t: stopReason=error boom"); got != turns {
		t.Fatalf("the text has %d turn lines, want %d", got, turns)
	}
	// The bytes allocated grow with the text, a few times its length;
	// copying the text once per turn would allocate thousands of times it.
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > uint64(20*len(text)) {
		t.Errorf("allocated %d bytes for a text of %d", allocated, len(text))
	}
}
