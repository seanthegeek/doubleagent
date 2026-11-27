package twinny

import (
	"fmt"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// globalTexts returns the text of every row the global state text of
// state.vscdb gives.
func globalTexts(t *testing.T, text string) []string {
	t.Helper()
	state, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	artifact := writeArtifact(t, "User/globalStorage/state.vscdb", "")
	var rows []string
	New().globalRows(artifact, state, parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row.Text) })
	return rows
}

// A team policy key is written before each entry of the list under it, and
// a reply's id before each file the reply changed, so a value over the
// limit read from state.vscdb is cut in every one of those rows, with one
// problem for it: no row holds it whole, and twice the entries cost twice
// the bytes, never the value's length again.
func TestAKeyOrReplyIDOfTheGlobalStateIsCutInEveryRowItStandsIn(t *testing.T) {
	long := "v" + strings.Repeat("a", 5000)
	tests := []struct {
		name, rows string
		state      func(count int) string
		problem    func(count int) string
	}{
		{"a team policy key before each list entry", "permission setting: twinny.teamPolicy.v",
			func(count int) string {
				var entries []string
				for i := range count {
					entries = append(entries, fmt.Sprintf(`"e%d"`, i))
				}
				return `{"twinny.teamPolicy": {"` + long + `": [` + strings.Join(entries, ",") + `]}}`
			}, func(count int) string { return parserkit.CarriedProblem("settings key", count) }},
		{"a reply id before each changed file", "file changed:", func(count int) string {
			var files []string
			for i := range count {
				files = append(files, fmt.Sprintf(`{"status": "M", "path": "p%d"}`, i))
			}
			return `{"twinny.conversations": {"c": {"id": "c", "messages": [{"role": "assistant", "content": "x", ` +
				`"changes": {"id": "` + long + `", "files": [` + strings.Join(files, ",") + `]}}]}}}`
		}, func(int) string { return parserkit.CarriedProblem("reply id", 1) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var sizes []int
			for _, count := range []int{10, 40} {
				rows := globalTexts(t, test.state(count))
				joined, problems, whole, size := 0, 0, 0, 0
				for _, text := range rows {
					size += len(text)
					if strings.HasPrefix(text, test.rows) {
						joined++
					}
					if text == parserkit.ProblemPrefix+test.problem(count) {
						problems++
					}
					if strings.Contains(text, long) {
						whole++
					}
				}
				sizes = append(sizes, size)
				if joined != count || problems != 1 || whole != 0 {
					t.Errorf("%d rows starting %q, %d problems and %d whole values, want %d, 1 and 0", joined,
						test.rows, problems, whole, count)
				}
			}
			if growth := sizes[1] - sizes[0]; growth > 30*(2*parserkit.CarriedLimit+1024) {
				t.Errorf("30 more entries cost %d bytes", growth)
			}
		})
	}
}
