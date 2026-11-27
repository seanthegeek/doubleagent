package musecode

import (
	"fmt"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// sessionRecord returns one session log line of the given payload type
// whose record is body.
func sessionRecord(kind, body string) string {
	return `{"stream": {"id": "s1"}, "payload_type": "` + kind + `", "payload": {"record": ` + body +
		`}, "recorded_at": 1700000000000000}` + "\n"
}

// The declared permission format goes into every later permission mode
// row, so a format over the limit is cut where it is read, with one
// problem for the log: no row holds it whole, and twice the mode changes
// cost twice the bytes, never the format's length again.
func TestADeclaredPermissionFormatIsCutInEveryModeRow(t *testing.T) {
	long := "f" + strings.Repeat("a", 50_000)
	var sizes []int
	for _, n := range []int{20, 40} {
		t.Run(fmt.Sprintf("%d mode changes", n), func(t *testing.T) {
			var log strings.Builder
			log.WriteString(sessionRecord("runtime.session.permission_format_declared", `{"format": "`+long+`"}`))
			for i := range n {
				log.WriteString(sessionRecord("runtime.session.permission_profile_committed",
					fmt.Sprintf(`{"resolved_snapshot": {"approval": "a%d"}}`, i%2)))
			}
			rows := parseFile(t, ".local/share/muse/sessions/2026/01/01/s1/session.jsonl", log.String())
			modes, whole, size := 0, 0, 0
			for _, row := range rows {
				size += len(row)
				if strings.HasPrefix(row, "permission mode") && strings.Contains(row, " format=f") {
					modes++
				}
				if strings.Contains(row, long) {
					whole++
				}
			}
			sizes = append(sizes, size)
			problems := countOf(rows, parserkit.ProblemPrefix+parserkit.CarriedProblem("permission format", 1))
			if modes != n || problems != 1 || whole != 0 {
				t.Errorf("%d mode rows, %d problems and %d whole formats, want %d, 1 and 0", modes, problems,
					whole, n)
			}
		})
	}
	if growth := sizes[1] - sizes[0]; growth > 20*(2*parserkit.CarriedLimit+1024) {
		t.Errorf("20 more mode changes cost %d bytes", growth)
	}
}
