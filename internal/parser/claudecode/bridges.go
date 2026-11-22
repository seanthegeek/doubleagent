package claudecode

import (
	"sort"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// bridgeRuns returns, for each bridge-session record of lines (in line
// order), the first and last position of the run of records next to it
// that name the same bridge id; an unnamed record is a run of its own.
// A record names another bridge than a given one exactly when it lies
// outside the given bridge's run, so the nearest other bridge either side
// of a line is one step past a run's end.
func bridgeRuns(lines []bridgeLine) (first, last []int) {
	first, last = make([]int, len(lines)), make([]int, len(lines))
	for i, each := range lines {
		first[i] = i
		if i > 0 && each.named && sameBridge(lines[i-1], each) {
			first[i] = first[i-1]
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		last[i] = i
		if i+1 < len(lines) && lines[i].named && sameBridge(lines[i+1], lines[i]) {
			last[i] = last[i+1]
		}
	}
	return first, last
}

// sameBridge reports whether two records name one bridge id.
func sameBridge(left, right bridgeLine) bool {
	return left.named && right.named && left.bridgeID == right.bridgeID
}

// bridgeURL returns the URL of the bridge's first bridge_status record at
// or after line, else the nearest before it, among those between the
// bridge-session records of other bridges either side (a detach, with no
// id, counting as another bridge), or "-". The records are in line order,
// one per line, so each side is found by a binary search; the records
// looked at are counted in work.
func (s *prescan) bridgeURL(bridge jsonvalue.Value, line int, work parser.Work) string {
	if len(s.runFirst) != len(s.bridgeLines) {
		s.runFirst, s.runLast = bridgeRuns(s.bridgeLines)
	}
	ours := func(i int) bool {
		work.Add("bridge records looked at", 1)
		other := s.bridgeLines[i]
		return other.named && bridge.IsString() && other.bridgeID == bridge.StringValue()
	}
	position := sort.Search(len(s.bridgeLines), func(i int) bool {
		work.Add("bridge records looked at", 1)
		return s.bridgeLines[i].line >= line
	})
	start, end := 0, -1
	if before := position - 1; before >= 0 {
		if ours(before) {
			before = s.runFirst[before] - 1
		}
		if before >= 0 {
			start = max(start, s.bridgeLines[before].line)
		}
	}
	after := position
	if after < len(s.bridgeLines) && s.bridgeLines[after].line == line {
		after++
	}
	if after < len(s.bridgeLines) && ours(after) {
		after = s.runLast[after] + 1
	}
	if after < len(s.bridgeLines) {
		end = s.bridgeLines[after].line
	}
	next := sort.Search(len(s.bridgeURLs), func(i int) bool {
		work.Add("bridge records looked at", 1)
		return s.bridgeURLs[i].line >= line
	})
	if next < len(s.bridgeURLs) && (end < 0 || s.bridgeURLs[next].line < end) {
		return s.bridgeURLs[next].url
	}
	if next > 0 && start < s.bridgeURLs[next-1].line {
		return s.bridgeURLs[next-1].url
	}
	return "-"
}
