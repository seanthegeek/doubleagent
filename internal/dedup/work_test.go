package dedup

import (
	"fmt"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// workCase builds a row set of size n whose rows all share one identity,
// the shape that makes every lookup see many candidates it may not take.
type workCase struct {
	name  string
	hooks Hooks
	build func(n int) ([]timeline.Row, []Secondary)
}

// repeated returns a row of the repeated identity at a time, or undated for
// a negative offset.
func repeated(offset int, file string, line int) timeline.Row {
	row := timeline.Row{Host: "h1", User: "alice", Agent: "claude-code", TurnType: "user",
		Text: "y", SourceFile: file, SourceLine: line}
	if offset >= 0 {
		row.TimestampUTC = timeutil.FormatUTC(time.UnixMilli(1_790_000_000_000 + int64(offset)))
	}
	return row
}

// secondaryOf wraps rows as secondary rows with the default tolerance.
func secondaryOf(rows []timeline.Row) []Secondary {
	secondary := make([]Secondary, len(rows))
	for i, row := range rows {
		secondary[i] = Secondary{row, ToleranceSeconds}
	}
	return secondary
}

// sameTime returns n rows of the repeated identity at one time, row i from
// file(i).
func sameTime(n int, file func(i int) string) []timeline.Row {
	rows := make([]timeline.Row, n)
	for i := range rows {
		rows[i] = repeated(0, file(i), i)
	}
	return rows
}

// oneFile names the same file for every row.
func oneFile(file string) func(int) string {
	return func(int) string { return file }
}

// workCases are the row sets whose work must grow in step with their size.
func workCases() []workCase {
	return append(otherWorkCases(), []workCase{
		{name: "primary and secondary rows at one time", build: func(n int) ([]timeline.Row, []Secondary) {
			return sameTime(n, oneFile("/t/p.jsonl")), secondaryOf(sameTime(n, oneFile("/h/s.jsonl")))
		}},
		{name: "undated secondary rows against primary rows", build: func(n int) ([]timeline.Row, []Secondary) {
			secondary := secondaryOf(sameTime(n, oneFile("/h/s.jsonl")))
			for i := range secondary {
				secondary[i].Row.TimestampUTC = ""
			}
			return sameTime(n, oneFile("/t/p.jsonl")), secondary
		}},
		{name: "secondary rows of one file at one time", build: func(n int) ([]timeline.Row, []Secondary) {
			return nil, secondaryOf(sameTime(n, oneFile("/h/s.jsonl")))
		}},
		{name: "secondary rows each of its own file", build: func(n int) ([]timeline.Row, []Secondary) {
			return nil, secondaryOf(sameTime(n, func(i int) string { return fmt.Sprint("/h/", i) }))
		}},
		{name: "two rows of one file to every row of another", build: func(n int) ([]timeline.Row, []Secondary) {
			return nil, secondaryOf(sameTime(n, func(i int) string {
				return []string{"/h/a", "/h/a", "/h/b"}[i%3]
			}))
		}},
	}...)
}

// otherWorkCases are the work cases of other shapes: sessions, tool call
// ids, widened windows and the order of the primary rows.
func otherWorkCases() []workCase {
	return []workCase{
		{name: "secondary rows each of its own session", build: func(n int) ([]timeline.Row, []Secondary) {
			rows := sameTime(n, func(i int) string { return []string{"/h/a", "/h/b"}[i%2] })
			for i := range rows {
				rows[i].SessionID = fmt.Sprint("s", i)
			}
			return nil, secondaryOf(rows)
		}},
		{name: "tool calls with an id against primary calls with others", build: func(n int) ([]timeline.Row, []Secondary) {
			primary, secondary := sameTime(n, oneFile("/t/p.jsonl")), sameTime(n, oneFile("/h/s.jsonl"))
			for i := range primary {
				primary[i].TurnType, primary[i].ToolUseID = timeline.TurnToolUse, fmt.Sprint("p", i)
				secondary[i].TurnType, secondary[i].ToolUseID = timeline.TurnToolUse, fmt.Sprint("s", i)
			}
			return primary, secondaryOf(secondary)
		}},
		{name: "widened rows each bounded by the next", hooks: sampleHooks(),
			build: func(n int) ([]timeline.Row, []Secondary) {
				primary, secondary := make([]timeline.Row, n), make([]timeline.Row, n)
				for i := range primary {
					primary[i] = repeated(n+i, "/t/p.jsonl", i)
					secondary[i] = repeated(i, "/h/s.jsonl", i)
					for _, row := range []*timeline.Row{&primary[i], &secondary[i]} {
						row.Agent, row.Text, row.SessionID = "hooked", "/x", "s1"
					}
				}
				return primary, secondaryOf(secondary)
			}},
		{name: "primary rows in descending time", build: func(n int) ([]timeline.Row, []Secondary) {
			rows := make([]timeline.Row, n)
			for i := range rows {
				rows[i] = repeated(10*(n-i), "/t/p.jsonl", i)
			}
			return rows, secondaryOf(sameTime(1, oneFile("/h/s.jsonl")))
		}},
	}
}

func TestDeduplicateWorkGrowsWithTheRows(t *testing.T) {
	const small = 2000
	for _, test := range workCases() {
		t.Run(test.name, func(t *testing.T) {
			measure := func(n int) int {
				primary, secondary := test.build(n)
				_, _, steps := deduplicate(primary, secondary, test.hooks)
				return steps
			}
			smaller, larger := measure(small), measure(4*small)
			t.Logf("%d steps for %d rows, %d for %d", smaller, small, larger, 4*small)
			if larger > 5*smaller {
				t.Fatalf("work grew from %d to %d steps for four times the rows", smaller, larger)
			}
		})
	}
}
