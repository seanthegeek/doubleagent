package codex

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// columnCompare returns how a column compares its value with an operand:
// a TEXT column turns a number into its text first, any other column
// compares the operand as given. NULL compares with nothing.
func columnCompare(value sqlitefile.Value, textTimes bool) func(sqlitefile.Value) (int, bool) {
	return func(operand sqlitefile.Value) (int, bool) {
		if textTimes && operand.Type == sqlitefile.Integer {
			operand = sqlitefile.TextValue(strconv.FormatInt(operand.Int, 10))
		}
		return sqlitefile.Compare(value, operand), value.Type != sqlitefile.Null && operand.Type != sqlitefile.Null
	}
}

// randomTime returns a ts of a few close seconds, so times tie often, as
// an integer, a float, text or NULL.
func randomTime(source *rand.Rand) sqlitefile.Value {
	second := int64(source.IntN(8))
	switch source.IntN(5) {
	case 0:
		return sqlitefile.Value{Type: sqlitefile.Float, Float: float64(second) + 0.5}
	case 1:
		return sqlitefile.TextValue(strconv.FormatInt(second, 10))
	case 2:
		return sqlitefile.Value{}
	}
	return sqlitefile.IntValue(second)
}

// randomGuardianLogs returns up to 30 guardian rows in rowid order with
// random times, bodies and spans of two threads and a few calls.
func randomGuardianLogs(source *rand.Rand, textTimes bool) []guardianLog {
	logs := make([]guardianLog, source.IntN(31))
	for i := range logs {
		time := randomTime(source)
		logs[i] = guardianLog{time: time, compare: columnCompare(time, textTimes), readable: source.IntN(8) > 0}
		for range source.IntN(3) {
			logs[i].spans = append(logs[i].spans, logSpan{thread: []string{"p", "q"}[source.IntN(2)],
				call: []string{"a", "b", "c", ""}[source.IntN(4)]})
		}
	}
	return logs
}

// callsByScan returns the distinct calls of the thread's spans in the rows
// from low to high seconds, in rowid order, by looking at every row; false
// when a row of the window is not readable.
func callsByScan(logs []guardianLog, thread string, low, high int64) ([]string, bool) {
	var inTime []guardianLog
	for _, each := range logs {
		lowOrder, lowOK := each.compare(sqlitefile.IntValue(low))
		highOrder, highOK := each.compare(sqlitefile.IntValue(high))
		if lowOK && highOK && lowOrder >= 0 && highOrder <= 0 {
			inTime = append(inTime, each)
		}
	}
	var calls []string
	for _, each := range inTime {
		if !each.readable {
			return nil, false
		}
		for _, span := range each.spans {
			if span.thread == thread && span.call != "" && !slices.Contains(calls, span.call) {
				calls = append(calls, span.call)
			}
		}
	}
	return calls, true
}

func TestAGuardianWindowHasTheCallsAScanOfEveryRowFinds(t *testing.T) {
	for seed := range uint64(300) {
		for _, textTimes := range []bool{false, true} {
			t.Run(fmt.Sprintf("seed %d text times %v", seed, textTimes), func(t *testing.T) {
				source := rand.New(rand.NewPCG(seed, 527))
				logs := randomGuardianLogs(source, textTimes)
				index := newGuardianIndex(slices.Clone(logs), textTimes, parser.Work{})
				for range 20 {
					low := int64(source.IntN(10)) - 1
					high := low + int64(source.IntN(6)) - 1
					thread := []string{"p", "q", "r"}[source.IntN(3)]
					want, wantOK := callsByScan(logs, thread, low, high)
					got, ok := index.windowCalls(thread, low, high)
					// Two calls stand for two or more; each must be one
					// the scan found.
					agrees := ok == wantOK && (len(got) == min(len(want), 2)) &&
						!slices.ContainsFunc(got, func(call string) bool { return !slices.Contains(want, call) }) &&
						(len(want) != 1 || got[0] == want[0])
					if !agrees {
						t.Fatalf("thread %s from %d to %d: got %q %v, want %q %v", thread, low, high, got, ok,
							want, wantOK)
					}
				}
			})
		}
	}
}

// windowWork returns the guardian rows looked at to answer n windows, each
// one second longer, over n rows of one second each naming one call.
func windowWork(n int) int {
	logs := make([]guardianLog, n)
	for i := range logs {
		time := sqlitefile.IntValue(int64(i))
		logs[i] = guardianLog{time: time, compare: columnCompare(time, false), readable: true,
			spans: []logSpan{{thread: "p", call: "call"}}}
	}
	work := parser.CountingWork()
	index := newGuardianIndex(logs, false, work)
	for i := range n {
		if calls, ok := index.windowCalls("p", 0, int64(i)); !ok || len(calls) != 1 {
			return -1
		}
	}
	return work.Count("guardian rows looked at")
}

func TestGuardianWindowsGrowWithTheRowsNotTheirSquare(t *testing.T) {
	small, large := windowWork(200), windowWork(2000)
	// Ten times the rows and windows costs about ten times the binary
	// searches' steps, each a little longer; a scan of every row per
	// window would cost a hundred times as much.
	if small <= 0 || large > 15*small {
		t.Errorf("rows looked at: %d for 200, %d for 2000", small, large)
	}
}

// callsOfThreadByScan returns the calls of a thread's spans in the rows of
// logs, in order, by looking at every row: the rule a reviewed thread's
// lookup is to agree with.
func callsOfThreadByScan(logs []guardianLog, thread string) ([]int, []string) {
	var rows []int
	var calls []string
	for position, each := range logs {
		for _, span := range each.spans {
			if span.thread == thread && span.call != "" {
				rows = append(rows, position)
				calls = append(calls, span.call)
			}
		}
	}
	return rows, calls
}

// otherByScan returns, for each call, the first later position whose call
// differs, len(calls) when none, by looking at every later call.
func otherByScan(calls []string) []int {
	other := make([]int, len(calls))
	for i := range calls {
		other[i] = len(calls)
		for j := i + 1; j < len(calls); j++ {
			if calls[j] != calls[i] {
				other[i] = j
				break
			}
		}
	}
	return other
}

func TestEachReviewedThreadHasTheCallsAScanOfEveryRowFinds(t *testing.T) {
	for seed := range uint64(300) {
		t.Run(fmt.Sprintf("seed %d", seed), func(t *testing.T) {
			source := rand.New(rand.NewPCG(seed, 96))
			logs := randomGuardianLogs(source, false)
			// Rows written twice, each naming the threads of the first.
			for range source.IntN(4) {
				if len(logs) > 0 {
					logs = append(logs, logs[source.IntN(len(logs))])
				}
			}
			index := newGuardianIndex(logs, false, parser.Work{})
			for _, thread := range []string{"p", "q", "r", ""} {
				rows, calls := callsOfThreadByScan(index.logs, thread)
				found := index.parent(thread)
				if !slices.Equal(found.rows, rows) || !slices.Equal(found.calls, calls) ||
					!slices.Equal(found.other, otherByScan(calls)) {
					t.Fatalf("thread %q: got rows %v calls %q other %v; want rows %v calls %q other %v", thread,
						found.rows, found.calls, found.other, rows, calls, otherByScan(calls))
				}
			}
		})
	}
}

// manyThreadsWork returns the guardian rows looked at to answer one window
// for each of n reviewed threads, over n rows of one second each, row i
// naming thread i alone.
func manyThreadsWork(n int) int {
	logs := make([]guardianLog, n)
	for i := range logs {
		time := sqlitefile.IntValue(int64(i))
		logs[i] = guardianLog{time: time, compare: columnCompare(time, false), readable: true,
			spans: []logSpan{{thread: "p" + strconv.Itoa(i), call: "call"}}}
	}
	work := parser.CountingWork()
	index := newGuardianIndex(logs, false, work)
	for i := range n {
		if calls, ok := index.windowCalls("p"+strconv.Itoa(i), int64(i), int64(i)); !ok || len(calls) != 1 {
			return -1
		}
	}
	return work.Count("guardian rows looked at")
}

func TestManyReviewedThreadsGrowWithTheRowsNotTheirSquare(t *testing.T) {
	small, large := manyThreadsWork(1000), manyThreadsWork(2000)
	// Twice the threads and rows costs a little over twice the binary
	// searches' steps; a scan of every row per thread would cost four
	// times as much.
	if small <= 0 || large > 3*small {
		t.Errorf("rows looked at: %d for 1000 threads, %d for 2000", small, large)
	}
}

// writeFile writes data at rel under home, making its directories.
func writeFile(t *testing.T, home, rel, data string) string {
	t.Helper()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEveryGuardianRolloutSharesOneReadOfTheLogDatabases(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".codex/logs_1.sqlite", "not a database")
	subject := New()
	subject.Work = parser.CountingWork()
	const rollouts = 5
	for i := range rollouts {
		threadID := fmt.Sprintf("0198f0a1-0000-7000-8000-00000000000%d", i)
		rel := ".codex/sessions/2026/01/02/rollout-2026-01-02T00-00-00-" + threadID + ".jsonl"
		path := writeFile(t, home, rel, `{"type":"session_meta","timestamp":"2026-01-02T00:00:00Z","payload":{`+
			`"id":"`+threadID+`","thread_source":"guardian_review","parent_thread_id":"parent",`+
			`"timestamp":"2026-01-02T00:00:00.000Z"}}`+"\n"+
			`{"type":"event_msg","timestamp":"2026-01-02T00:00:05Z","payload":{`+
			`"type":"task_complete","last_agent_message":"{\"outcome\":\"allow\"}"}}`)
		subject.parseRollout(&input.Artifact{Rel: rel, DiskPath: path, Home: &input.Home{DiskPath: home}},
			parser.Options{}, func(timeline.Row) {})
	}
	rolloutReads, databaseReads := subject.Work.Count("guardian log reads"),
		subject.Work.Count("guardian log databases read")
	if rolloutReads != rollouts || databaseReads != 1 {
		t.Errorf("%d rollouts looked for the logs and read %d databases; want %d and 1", rolloutReads,
			databaseReads, rollouts)
	}
}

func TestTheStateDatabaseIsReadOnceForEveryLogDatabaseBesideIt(t *testing.T) {
	home := t.TempDir()
	statePath := writeFile(t, home, ".codex/state_5.sqlite", "not a database")
	subject := New()
	subject.Work = parser.CountingWork()
	for i := range 5 {
		rel := ".codex/logs_" + strconv.Itoa(i) + ".sqlite"
		subject.siblingThreads(&input.Artifact{Rel: rel, DiskPath: filepath.Join(home, filepath.FromSlash(rel))})
	}
	if reads := subject.Work.Count("state databases read"); reads != 1 {
		t.Errorf("read the state database %d times for 5 log databases, want 1", reads)
	}
	if err := osfile.WriteFile(statePath, []byte(strings.Repeat("x", 100)), 0o600); err != nil {
		t.Fatal(err)
	}
	subject.siblingThreads(&input.Artifact{Rel: ".codex/logs_9.sqlite",
		DiskPath: filepath.Join(home, ".codex", "logs_9.sqlite")})
	if reads := subject.Work.Count("state databases read"); reads != 2 {
		t.Errorf("read the changed state database %d times in all, want 2", reads)
	}
}
