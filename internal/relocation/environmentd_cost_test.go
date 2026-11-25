package relocation

import (
	"runtime"
	"strings"
	"testing"
)

// allocatedReading returns the bytes allocated while resolveEnvironmentD
// reads text, and the assignments it returned.
func allocatedReading(text string) (int64, []assignment) {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got, _ := resolveEnvironmentD(strings.NewReader(text), testNames(), "")
	runtime.ReadMemStats(&after)
	return int64(after.TotalAlloc - before.TotalAlloc), got
}

// An assignment joined from many lines costs memory in proportion to the
// file: the bytes allocated (runtime.MemStats.TotalAlloc) for more lines
// grow by a few copies of the added text, where rebuilding the pending text
// for each line grows by the square.
func TestEnvironmentDJoinsLinesInLinearCost(t *testing.T) {
	const lineSize = 50
	line := strings.Repeat("a", lineSize-1) + "\n"
	for name, head := range map[string]string{
		"an open quote":     "CODEX_HOME=\"/x\n",
		"a trailing escape": "CODEX_HOME=/x\\\n",
	} {
		t.Run(name, func(t *testing.T) {
			allocated := map[int]int64{}
			for _, lines := range []int{4000, 8000} {
				body := line
				if strings.HasSuffix(head, "\\\n") {
					body = strings.Repeat("a", lineSize-2) + "\\\n"
				}
				size, got := allocatedReading(head + strings.Repeat(body, lines))
				allocated[lines] = size
				if len(got) != 1 || got[0].Reason == "" {
					t.Fatalf("%d lines: got %.200v", lines, got)
				}
			}
			added := int64(4000 * lineSize)
			if growth := allocated[8000] - allocated[4000]; growth > 10*added {
				t.Errorf("4000 more lines of %d bytes allocated %d more bytes, over ten copies", lineSize, growth)
			}
		})
	}
}
