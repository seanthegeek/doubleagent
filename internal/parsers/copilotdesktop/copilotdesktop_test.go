package copilotdesktop

import (
	"math"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// The rows of data.db and the checkpoint refs are checked by the golden
// output of the analysis commands (internal/analyze), over inputs that hold
// every parsed agent; these tests cover the column conversions and the paths
// the parser wants.

func TestColumnsBecomeRowTextADashOrAFlag(t *testing.T) {
	tests := []struct {
		name  string
		value sqlitefile.Value
		text  string
		dash  string
		flag  string
	}{
		{"NULL", sqlitefile.Value{}, "", "-", "false"},
		{"empty text", sqlitefile.TextValue(""), "", "-", "false"},
		{"text", sqlitefile.TextValue("a b"), "a b", "a b", "true"},
		{"an integer", sqlitefile.IntValue(1), "1", "1", "true"},
		{"zero is not empty", sqlitefile.IntValue(0), "0", "0", "true"},
		{"a float", sqlitefile.FloatValue(1), "1", "1", "true"},
		{"positive infinity", sqlitefile.FloatValue(math.Inf(1)), "Infinity", "Infinity", "true"},
		{"negative infinity", sqlitefile.FloatValue(math.Inf(-1)), "-Infinity", "-Infinity", "true"},
		{"a blob", sqlitefile.BlobValue([]byte("ab")), "x'6162'", "x'6162'", "true"},
		{"an empty blob", sqlitefile.BlobValue(nil), "x''", "x''", "true"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parserkit.ColumnText(test.value); got != test.text {
				t.Errorf("text %q, want %q", got, test.text)
			}
			if got := dash(test.value); got != test.dash {
				t.Errorf("dash %q, want %q", got, test.dash)
			}
			if got := flag(test.value); got != test.flag {
				t.Errorf("flag %q, want %q", got, test.flag)
			}
		})
	}
}

func TestWantsTheDatabaseAndCheckpointRefsOnly(t *testing.T) {
	const ref = ".copilot/repos/r/.git/refs/copilot/checkpoints/"
	const sessionID = "0123456789abcdef0123456789abcdef0123"
	tests := []struct {
		rel  string
		want bool
	}{
		{".copilot/data.db", true},
		{".copilot/data.db-wal", false},
		{"x/.copilot/data.db", false},
		{ref + sessionID + "/1/" + sessionID, true},
		{ref + sessionID + "/12/" + sessionID, true},
		{ref + sessionID + "/x/" + sessionID, false},
		{ref + "short/1/" + sessionID, false},
		{".copilot/repos/a/b/.git/refs/copilot/checkpoints/" + sessionID + "/1/" + sessionID, false},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			if got := New().Wants(&input.Artifact{Rel: test.rel}); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}
