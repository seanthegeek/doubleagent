package dedup

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// sampleHooks returns hooks for the agent "hooked": a system row "cmd: x"
// matches as the user text "/x", a user row starting with a slash looks 30
// seconds ahead, and a row whose model is "never" or "at <time>" is
// anchored there (an undated "at" keeps the row's own time).
func sampleHooks() Hooks {
	return Hooks{"hooked": {
		Form: func(row timeline.Row) (string, string) {
			if row.TurnType == timeline.TurnSystem && strings.HasPrefix(row.Text, "cmd: ") {
				return timeline.TurnUser, "/" + row.Text[5:]
			}
			return row.TurnType, row.Text
		},
		Window: func(row timeline.Row) (float64, bool) {
			return 30, row.TurnType == timeline.TurnUser && strings.HasPrefix(row.Text, "/")
		},
		Anchor: func(row timeline.Row) (AnchorKind, int64) {
			if row.Model == "never" {
				return NeverMatch, 0
			}
			if moment, ok := strings.CutPrefix(row.Model, "at "); ok {
				if millis, dated := timeutil.EpochMillis(moment); dated {
					return AtTime, millis
				}
			}
			return OwnTime, 0
		},
	}}
}

// hookedRow returns a row of the agent "hooked" at a second past 10:00,
// changed by the options: text, turn type, file, line, model, session.
func hookedRow(second int, options map[string]string) timeline.Row {
	row := timeline.Row{TimestampUTC: fmt.Sprintf("2026-10-01T10:%02d:%02d.000Z", second/60,
		second%60), Host: "h1", User: "alice", Agent: "hooked",
		SessionID: "s1", ProjectPath: "/p", TurnType: "user", Text: "/x",
		SourceFile: "/t/a.jsonl", SourceLine: 1}
	for key, value := range options {
		switch key {
		case "text":
			row.Text = value
		case "turn":
			row.TurnType = value
		case "file":
			row.SourceFile = value
		case "line":
			row.SourceLine, _ = strconv.Atoi(value)
		case "model":
			row.Model = value
		case "session":
			row.SessionID = value
		}
	}
	return row
}

func TestHooks(t *testing.T) {
	copyAt := func(second, line int, extra map[string]string) timeline.Row {
		options := map[string]string{"file": "/h/a", "line": strconv.Itoa(line)}
		maps.Copy(options, extra)
		return hookedRow(second, options)
	}
	tests := []struct {
		name      string
		primary   []timeline.Row
		secondary []timeline.Row
		kept      []int // source lines kept, the secondary ones recovered
		dropped   int
	}{
		{"a row in another form matches under the agent's form",
			rows(hookedRow(0, nil)), rows(copyAt(1, 2, map[string]string{"text": "cmd: x",
				"turn": "system"})), []int{1}, 1},
		{"a widened row takes a primary row twenty seconds after it",
			rows(hookedRow(20, nil)), rows(copyAt(0, 2, nil)), []int{1}, 1},
		{"the wider window looks forward only",
			rows(hookedRow(0, nil)), rows(copyAt(20, 2, nil)), []int{1, 2}, 0},
		{"the next row of the same text bounds the window",
			rows(hookedRow(15, nil)), rows(copyAt(0, 2, nil), copyAt(10, 3, nil)), []int{1, 2}, 1},
		{"a row of another session never bounds the window",
			rows(hookedRow(15, nil)), rows(copyAt(0, 2, nil), copyAt(10, 3,
				map[string]string{"session": "s2"})), []int{1, 3}, 1},
		{"a row anchored to never matches no primary row",
			rows(hookedRow(0, nil)), rows(copyAt(0, 2, map[string]string{"model": "never"})),
			[]int{1, 2}, 0},
		{"a row anchored to never matches a copy at its own time",
			nil, rows(copyAt(0, 2, map[string]string{"model": "never"}), copyAt(1, 3,
				map[string]string{"file": "/h/b"})), []int{2}, 1},
		{"a row anchored at a time matches the primary row at that time",
			rows(hookedRow(60, map[string]string{"text": "a"})), rows(copyAt(0, 2,
				map[string]string{"text": "a", "model": "at 2026-10-01T10:01:00.500Z"})),
			[]int{1}, 1},
		{"a row anchored at a time is matched there among copies too",
			nil, rows(copyAt(0, 2, map[string]string{"text": "a",
				"model": "at 2026-10-01T10:01:00.000Z"}), copyAt(60, 3,
				map[string]string{"text": "a", "file": "/h/b"})), []int{2}, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kept, dropped := Deduplicate(test.primary, withTolerance(ToleranceSeconds,
				test.secondary...), sampleHooks())
			var lines []int
			for i, row := range kept {
				lines = append(lines, row.SourceLine)
				want := "yes"
				if i < len(test.primary) {
					want = ""
				}
				if row.Recovered != want {
					t.Errorf("line %d recovered %q, want %q", row.SourceLine, row.Recovered, want)
				}
			}
			if !slices.Equal(lines, test.kept) || dropped["hooked"] != test.dropped {
				t.Errorf("kept lines %v dropping %d, want %v dropping %d", lines,
					dropped["hooked"], test.kept, test.dropped)
			}
		})
	}
}
