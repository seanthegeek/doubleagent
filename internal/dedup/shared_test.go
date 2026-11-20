package dedup

import (
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// sharedRow returns a user row at 10:00:00 whose text is prefix and then
// shared, which it holds as the stand-in "[shared]".
func sharedRow(prefix string, shared *timeline.SharedText) timeline.Row {
	row := timeline.Row{TimestampUTC: "2026-10-01T10:00:00.000Z", Host: "h1", User: "alice", Agent: "plain",
		SessionID: "s1", TurnType: timeline.TurnUser, SourceFile: "/t/history", SourceLine: 1}
	var out timeline.TextBuilder
	out.Write(prefix)
	out.Share(shared, "[shared]", func(*timeline.Row, bool) string { return "[pointer]" })
	out.Set(&row)
	return row
}

func TestARowNamingASharedTextIsMatchedByItsFullText(t *testing.T) {
	long := strings.Repeat("long text ", 10) + "end"
	tests := []struct {
		name, primary string
		shared        string
		dropped       bool
	}{
		{"a short full text equal to the primary row's", "x short", "short", true},
		{"a long full text equal to the primary row's", "x " + long, long, true},
		{"a long full text that differs", "x " + long, long + "!", false},
		{"a primary row holding the stand-in", "x [shared]", long, false},
		{"a short text the length of a digest", "x " + strings.Repeat("y", 30), strings.Repeat("y", 30), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			secondary := sharedRow("x ", timeline.NewSharedText(test.shared))
			primary := secondary
			primary.Text, primary.Shared, primary.SourceFile = test.primary, nil, "/t/transcript"
			kept, dropped := Deduplicate([]timeline.Row{primary}, []Secondary{{Row: secondary, Tolerance: 2}}, nil)
			if got := dropped["plain"] == 1; got != test.dropped || len(kept) != 2-dropped["plain"] {
				t.Errorf("dropped %v (%d rows kept), want %v", got, len(kept), test.dropped)
			}
		})
	}
}
