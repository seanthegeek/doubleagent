package collect_test

import (
	"bufio"
	"os"
	"slices"
	"strings"
	"testing"
)

// uncoveredFile lists the table lines the coverage test excuses from
// leaving evidence, with the reason of each.
const uncoveredFile = "testdata/coverage-uncovered.txt"

// uncoveredHeader opens testdata/coverage-uncovered.txt and says what its
// reasons mean.
const uncoveredHeader = `# Table lines the coverage test (coverage_test.go) excuses from leaving
# evidence, one per line as "<reason> <table> <line>", sorted. The reasons:
#
#   none               neither image manifest (imagetest/testdata/) holds a
#                      path the line matches, so no run can show it.
#   unix-only          only image_unix.txt does: Windows runs need no
#                      evidence of the line.
#   windows-only       only image_windows.txt does: Linux and macOS runs
#                      need no evidence of the line.
#   linux-unreached    the image holds a path the line matches, and no run
#                      on Linux reaches it: a more specific nested entry of
#                      another agent claims it, or it lies outside every
#                      catalog entry's walk.
#   darwin-unreached   the same on macOS.
#   windows-unreached  the same on Windows.
#
# The test derives every reason and fails when this file gives another;
# DOUBLEAGENT_UPDATE_GOLDEN=1 rewrites the reasons this platform derives,
# keeping the other platforms' unreached lines. Read what changed before
# committing it: a line that leaves none or unreached is a missing fixture.
`

// readUncovered returns the reasons testdata/coverage-uncovered.txt gives
// each line it lists, keyed by table and line, and fails the test on an
// unknown reason or lines out of order.
func readUncovered(t *testing.T) map[string][]string {
	t.Helper()
	file, err := os.Open(uncoveredFile)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	known := []string{"none", "unix-only", "windows-only", "linux-unreached", "darwin-unreached",
		"windows-unreached"}
	uncovered := map[string][]string{}
	previous := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		reason, key, _ := strings.Cut(line, " ")
		if !slices.Contains(known, reason) {
			t.Errorf("%s: %q gives no known reason", uncoveredFile, line)
		}
		if line <= previous {
			t.Errorf("%s: %q is out of order or repeated", uncoveredFile, line)
		}
		previous, uncovered[key] = line, append(uncovered[key], reason)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return uncovered
}

// writeUncovered rewrites testdata/coverage-uncovered.txt with the reasons
// this platform derived for each line and the listed reasons it does not
// decide (the other platforms' unreached), for lines still in the tables.
func writeUncovered(t *testing.T, derived, listed map[string][]string) {
	t.Helper()
	var lines []string
	for key, reasons := range derived {
		for _, reason := range listed[key] {
			if !decidedHere(reason) {
				reasons = append(reasons, reason)
			}
		}
		for _, reason := range reasons {
			lines = append(lines, reason+" "+key)
		}
	}
	slices.Sort(lines)
	text := uncoveredHeader + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(uncoveredFile, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
