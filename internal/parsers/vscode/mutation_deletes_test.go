package vscode

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// deleteLog returns a mutation log that pushes n requests one at a time,
// then deletes them all, alternately from the front and one before the
// end.
func deleteLog(n int) string {
	var log strings.Builder
	log.WriteString(`{"kind":0,"v":{"requests":[]}}` + "\n")
	for i := range n {
		fmt.Fprintf(&log, `{"kind":2,"k":["requests"],"v":[{"n":%d}]}`+"\n", i)
	}
	for i := range n {
		index := 0
		if i%2 == 1 && n-i > 1 {
			index = -2
		}
		fmt.Fprintf(&log, `{"kind":3,"k":["requests",%d]}`+"\n", index)
	}
	return log.String()
}

// A delete leaves the items after it where they are, so a log of n pushes
// and n deletes changes 2n entries, not a number that grows with the
// square of n.
func TestDeletingRequestsMovesNoOtherEntry(t *testing.T) {
	for _, n := range []int{100, 1000} {
		t.Run(fmt.Sprintf("%d pushes and %d deletes change %d entries", n, n, 2*n), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "session.jsonl")
			if err := osfile.WriteFile(path, []byte(deleteLog(n)), 0o600); err != nil {
				t.Fatal(err)
			}
			work := parserkit.CountingWork()
			lines := requestLines{}
			var problems parserkit.Problems
			state := replay(nil, path, lines, &problems, work)
			if state.Get("requests").Len() != 0 || len(problems) != 0 || !maps.Equal(lines, requestLines{sessionStart: 1}) {
				t.Fatalf("replay gave %s, lines %v and problems %v", state.JSON(), lines, problems)
			}
			if got := work.Count("entries changed"); got != 2*n {
				t.Errorf("changed %d entries, want %d", got, 2*n)
			}
		})
	}
}

// randomRecord returns a mutation record of any kind at a path into the
// requests or into one of them, with indexes in range, out of it and
// negative, and pushes with and without a truncation.
func randomRecord(source *rand.Rand) string {
	index := func() string {
		switch source.IntN(6) {
		case 0:
			return fmt.Sprint(-source.IntN(6) - 1)
		case 1:
			return "1.0"
		}
		return fmt.Sprint(source.IntN(8))
	}
	paths := []string{`["requests"]`, `["requests",` + index() + `]`, `["requests",` + index() + `,"a"]`,
		`["other"]`, `[]`}
	path := paths[source.IntN(len(paths))]
	value := []string{`[{}]`, `[{},{"b":1}]`, `{}`, `"x"`, `[]`, `null`}[source.IntN(6)]
	switch kind := source.IntN(5); kind {
	case 0:
		return `{"kind":0,"v":{"requests":` + []string{`[]`, `[{},{}]`, `{}`}[source.IntN(3)] + `}}`
	case 2:
		truncation := ""
		if source.IntN(2) == 0 {
			truncation = `,"i":` + index()
		}
		return `{"kind":2,"k":` + path + truncation + `,"v":` + value + `}`
	case 4:
		return `{"kind":7,"k":` + path + `}`
	default:
		return fmt.Sprintf(`{"kind":%d,"k":%s,"v":%s}`, kind, path, value)
	}
}

// requestMadeInside returns the position of the request a push record
// makes at a path inside the list, ["requests", i] where request i is
// absent (i being the length) or null, which the shifting replay gave no
// line or kept the null request's line for; the replay gives it the line
// of the record that made it, as it does a request set by its index.
func requestMadeInside(state *shiftingNode, record jsonvalue.Value) (int64, bool) {
	path := record.Get("k").Items()
	if !isKind(record.Get("kind"), 2) || len(path) != 2 || !isRequests(path[0]) || state == nil ||
		!state.isObject {
		return 0, false
	}
	requests := state.members["requests"]
	if requests == nil || !requests.isArray {
		return 0, false
	}
	if index, ok := integer(path[1]); ok && index == int64(len(requests.items)) {
		return index, true
	}
	index, ok := listIndex(path[1], len(requests.items))
	if !ok || !requests.items[index].isNull() {
		return 0, false
	}
	return int64(index), true
}

// Over seeded logs of every kind of record, the replay that leaves deleted
// items' slots empty gives the state, errors and request lines of the one
// that moved every later item down at each delete.
func TestTheReplayMatchesTheShiftingReplayOverSeededLogs(t *testing.T) {
	for seed := range uint64(300) {
		source := rand.New(rand.NewPCG(seed, 527))
		var state *node
		var reference *shiftingNode
		started, startLine := false, 0
		lines, referenceLines := requestLines{}, requestLines{}
		for step := range 80 {
			text := randomRecord(source)
			record, err := jsonvalue.DecodeString(text)
			if err != nil {
				t.Fatalf("seed %d: %s: %v", seed, text, err)
			}
			referenceBefore := shiftingRequestCount(reference)
			made, makes := requestMadeInside(reference, record)
			nextReference, referenceErr := shiftingApply(reference, record, parserkit.Work{})
			next, err := applyMutation(state, record, step, parserkit.Work{})
			if fmt.Sprint(err) != fmt.Sprint(referenceErr) {
				t.Fatalf("seed %d step %d %s: error %v, the shifting replay's %v", seed, step, text, err, referenceErr)
			}
			if err != nil {
				continue
			}
			state, reference = next, nextReference
			shiftingTrack(record, referenceBefore, shiftingRequestCount(reference), step, referenceLines, parserkit.Work{})
			if makes {
				referenceLines[made] = step
			}
			if isKind(record.Get("kind"), 0) {
				started, startLine = true, step
			}
			trackLines(state, started, startLine, lines)
			if got, want := state.value().JSON(), reference.value().JSON(); got != want {
				t.Fatalf("seed %d step %d %s: state %s, the shifting replay's %s", seed, step, text, got, want)
			}
			if !maps.Equal(lines, referenceLines) {
				t.Fatalf("seed %d step %d %s: lines %v, the shifting replay's %v", seed, step, text, lines,
					referenceLines)
			}
		}
	}
}

// largeValueLog returns a mutation log that makes one request of n
// response parts, then sets a member of each part, pushes n more parts
// and deletes n from the front.
func largeValueLog(n int) string {
	var log strings.Builder
	log.WriteString(`{"kind":0,"v":{"requests":[{"response":[` + strings.Repeat(`{"value":"x"},`, n) + `{}]}]}}` + "\n")
	for i := range n {
		fmt.Fprintf(&log, `{"kind":1,"k":["requests",0,"response",%d,"value"],"v":"y"}`+"\n", i)
	}
	for range n {
		log.WriteString(`{"kind":2,"k":["requests",0,"response"],"v":[{"value":"z"}]}` + "\n")
	}
	for range n {
		log.WriteString(`{"kind":3,"k":["requests",0,"response",0]}` + "\n")
	}
	return log.String()
}

// A log of changes to one large value costs allocations in proportion to
// the log: twice the records allocate at most a little over twice as much.
func TestALongLogOfChangesToOneValueAllocatesInProportionToTheLog(t *testing.T) {
	allocations := map[int]float64{}
	for _, n := range []int{1000, 2000} {
		path := filepath.Join(t.TempDir(), "session.jsonl")
		if err := osfile.WriteFile(path, []byte(largeValueLog(n)), 0o600); err != nil {
			t.Fatal(err)
		}
		allocations[n] = testing.AllocsPerRun(3, func() {
			var problems parserkit.Problems
			if state := replay(nil, path, requestLines{}, &problems, parserkit.Work{}); len(problems) != 0 ||
				state.Get("requests").Index(0).Get("response").Len() != n+1 {
				t.Fatalf("replay gave problems %v", problems)
			}
		})
	}
	if ratio := allocations[2000] / allocations[1000]; ratio > 2.1 {
		t.Errorf("2000 changes allocate %.0f times, %.2f times the %.0f of 1000", allocations[2000], ratio,
			allocations[1000])
	}
}
