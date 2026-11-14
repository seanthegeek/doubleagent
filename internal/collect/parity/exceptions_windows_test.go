//go:build parity && windows

package parity

import (
	"strings"
	"testing"
)

// dropWildcardLogError removes from the PowerShell collector's stderr the
// one error record its archiver step writes when the output directory's
// name holds a wildcard character: tar's error output is appended to the log
// with 2>>, which resolves the log's path as a wildcard pattern and fails on
// the [ of "out [1]". That is a defect of the PowerShell collector, not a
// behaviour the binary reproduces. 5.1 wraps the record at the console
// width, so the record runs from its "out-file : " line to the line that
// ends its message and through the position and category lines after it.
// The case fails when the record is not there exactly once.
func dropWildcardLogError(t *testing.T, _ image, want *collectorRun) {
	t.Helper()
	lines := strings.Split(want.stderr, "\n")
	var kept []string
	found := 0
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "out-file : Cannot perform operation because the wildcard path ") {
			kept = append(kept, lines[i])
			continue
		}
		end := i
		for end < len(lines) && !strings.Contains(lines[end], "did not resolve to a file.") {
			end++
		}
		if end == len(lines) {
			break
		}
		found++
		for end+1 < len(lines) && isErrorRecordDetail(lines[end+1]) {
			end++
		}
		i = end
	}
	if found != 1 {
		t.Errorf("expected one wildcard error record from the PowerShell collector's archiver on stderr, found %d:\n%s",
			found, want.stderr)
		return
	}
	want.stderr = strings.Join(kept, "\n")
}

// isErrorRecordDetail reports whether line belongs to the position and
// category part of a PowerShell error record: "At <script>:<line> char:<n>",
// the quoted source lines starting with "+", and the indented "+ CategoryInfo"
// and "+ FullyQualifiedErrorId" lines.
func isErrorRecordDetail(line string) bool {
	trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	return strings.HasPrefix(trimmed, "At ") || strings.HasPrefix(trimmed, "+")
}
