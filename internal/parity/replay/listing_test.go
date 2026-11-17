//go:build parity

package replay

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The output files of timeline whose lines a loose tree's listing order
// reaches.
const (
	timelineName = "timeline.jsonl"
	sessionsName = "sessions.jsonl"
)

// holdsTimeline reports whether a run's outputs hold a timeline.jsonl.
func holdsTimeline(outputs map[string]string) bool {
	for path := range outputs {
		if strings.HasSuffix(path, timelineName) {
			return true
		}
	}
	return false
}

// filePair is the source file of a recorded line and of the run's line it
// was paired with.
type filePair struct {
	want, got string
}

// otherOfEqualRows is documented difference 107 for a loose tree's timeline:
// the binary lists the tree in name order and the recording in its file
// system's, so where deduplication kept one of two copies of a row that
// differ only in source_file and source_line, the two may keep different
// copies. It rewrites, in got's outputs, each such row to the recorded
// copy, and each session that differs only in a source_file one of those
// rows moved, and returns how many rows it rewrote.
func otherOfEqualRows(want, got map[string]string) int {
	total := 0
	for path, recorded := range want {
		directory, found := strings.CutSuffix(path, timelineName)
		if !found {
			continue
		}
		var pairs []filePair
		got[path], pairs = rewriteCopies(recorded, got[path], []string{"source_file", "source_line"},
			func(filePair) bool { return true })
		total += len(pairs)
		sessions := directory + sessionsName
		if _, ok := want[sessions]; ok && len(pairs) > 0 {
			got[sessions], _ = rewriteCopies(want[sessions], got[sessions], []string{"source_file"},
				func(pair filePair) bool { return slices.Contains(pairs, pair) })
		}
	}
	return total
}

// rewriteCopies replaces each line of got that want lacks with a line of
// want that got lacks when the two JSON records are equal but for the
// ignored keys, come from different source files and allowed accepts the
// pair. It returns the rewritten text and the pairs it rewrote.
func rewriteCopies(want, got string, ignored []string, allowed func(filePair) bool) (string,
	[]filePair) {
	gotLines := strings.Split(got, "\n")
	wantOnly, gotOnly := unmatchedLines(strings.Split(want, "\n"), gotLines)
	var pairs []filePair
	for _, wanted := range wantOnly {
		wantKey, wantFile, ok := recordKey(wanted, ignored)
		if !ok {
			continue
		}
		for j, index := range gotOnly {
			gotKey, gotFile, ok := recordKey(gotLines[index], ignored)
			pair := filePair{wantFile, gotFile}
			if ok && gotKey == wantKey && gotFile != wantFile && allowed(pair) {
				gotLines[index] = wanted
				gotOnly = slices.Delete(gotOnly, j, j+1)
				pairs = append(pairs, pair)
				break
			}
		}
	}
	return strings.Join(gotLines, "\n"), pairs
}

// unmatchedLines returns the lines of want that got does not hold, counted
// as a multiset, and the indexes in got of the lines want does not hold.
func unmatchedLines(want, got []string) ([]string, []int) {
	counts := map[string]int{}
	for _, line := range got {
		counts[line]++
	}
	var wantOnly []string
	for _, line := range want {
		if counts[line] > 0 {
			counts[line]--
			continue
		}
		wantOnly = append(wantOnly, line)
	}
	var gotOnly []int
	for i := len(got) - 1; i >= 0; i-- {
		if counts[got[i]] > 0 {
			counts[got[i]]--
			gotOnly = append(gotOnly, i)
		}
	}
	slices.Reverse(gotOnly)
	return wantOnly, gotOnly
}

// recordKey returns a JSON record's text with the ignored keys left out,
// keys sorted, and its source_file; false when the line is not a record
// with a source file.
func recordKey(line string, ignored []string) (string, string, bool) {
	var record map[string]json.RawMessage
	if json.Unmarshal([]byte(line), &record) != nil {
		return "", "", false
	}
	var sourceFile string
	if json.Unmarshal(record["source_file"], &sourceFile) != nil || sourceFile == "" {
		return "", "", false
	}
	for _, key := range ignored {
		delete(record, key)
	}
	var key strings.Builder
	for _, name := range slices.Sorted(maps.Keys(record)) {
		key.WriteString(name + "=" + string(record[name]) + "\n")
	}
	return key.String(), sourceFile, true
}

// rowOrderDiffers reports whether a timeline.jsonl or sessions.jsonl of
// got differs from want's once the documented differences of text are
// applied, so that only such a case is compared as multisets of lines.
func rowOrderDiffers(t *testing.T, node string, want, got map[string]string) bool {
	for path, text := range want {
		if !strings.HasSuffix(path, timelineName) && !strings.HasSuffix(path, sessionsName) {
			continue
		}
		if normalWant, normalGot := normalisedPair(t, node, text, got[path]); normalWant != normalGot {
			return true
		}
	}
	return false
}

// namesOrNone returns names joined by commas, or "none".
func namesOrNone(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// sortRowLines sorts the lines of each timeline.jsonl and sessions.jsonl
// of outputs in place, so that two runs are compared as multisets of rows.
func sortRowLines(outputs map[string]string) {
	for path, text := range outputs {
		if strings.HasSuffix(path, timelineName) || strings.HasSuffix(path, sessionsName) {
			lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
			slices.Sort(lines)
			outputs[path] = strings.Join(lines, "\n") + "\n"
		}
	}
}

// checkDocumentedOrder fails the test when a run's timeline.jsonl or
// sessions.jsonl is not in the order the analyzer sorts them in: rows by
// undated last, time, source file and source line; sessions by first time,
// agent and session id.
func checkDocumentedOrder(t testing.TB, node string, outputs map[string]string) {
	t.Helper()
	for _, path := range slices.Sorted(maps.Keys(outputs)) {
		var compare func(left, right string) int
		switch {
		case strings.HasSuffix(path, timelineName):
			compare = compareRowLines
		case strings.HasSuffix(path, sessionsName):
			compare = compareSessionLines
		default:
			continue
		}
		lines := strings.Split(strings.TrimSuffix(outputs[path], "\n"), "\n")
		for i := 1; i < len(lines); i++ {
			if compare(lines[i-1], lines[i]) > 0 {
				t.Errorf("%s: %s line %d sorts before line %d:\n  %s\n  %s", node, path, i+1, i,
					lines[i], lines[i-1])
				break
			}
		}
	}
}

// sortFields are the fields of a timeline row or session line its order
// is decided by.
type sortFields struct {
	TimestampUTC      string `json:"timestamp_utc"`
	SourceFile        string `json:"source_file"`
	SourceLine        int    `json:"source_line"`
	FirstTimestampUTC string `json:"first_timestamp_utc"`
	Agent             string `json:"agent"`
	SessionID         string `json:"session_id"`
}

// readSortFields decodes the sort fields of one line; a line that does
// not decode gives empty fields.
func readSortFields(line string) sortFields {
	var fields sortFields
	_ = json.Unmarshal([]byte(line), &fields)
	return fields
}

// compareRowLines orders two timeline lines as the timeline sort does.
func compareRowLines(left, right string) int {
	first, second := readSortFields(left), readSortFields(right)
	return timeline.CompareRows(
		timeline.Row{TimestampUTC: first.TimestampUTC, SourceFile: first.SourceFile,
			SourceLine: first.SourceLine},
		timeline.Row{TimestampUTC: second.TimestampUTC, SourceFile: second.SourceFile,
			SourceLine: second.SourceLine})
}

// compareSessionLines orders two session lines as the session summary
// does.
func compareSessionLines(left, right string) int {
	first, second := readSortFields(left), readSortFields(right)
	if order := strings.Compare(first.FirstTimestampUTC, second.FirstTimestampUTC); order != 0 {
		return order
	}
	if order := strings.Compare(first.Agent, second.Agent); order != 0 {
		return order
	}
	return strings.Compare(first.SessionID, second.SessionID)
}
