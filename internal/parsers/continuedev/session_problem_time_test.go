package continuedev

import (
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// A problem row of a session file carries its session id and project and no
// time of its own, though a member of a repeated id's group takes its
// parent's; a session whose id the index repeats keeps each such row in the
// group.
func TestASessionsProblemRowsHaveNoTime(t *testing.T) {
	const entry = `{"sessionId":"a","dateCreated":"1767000000000","workspaceDirectory":"/srv/proj"}`
	for _, test := range []struct {
		name, index string
		grouped     bool
	}{
		{"an id held once", "[" + entry + "]", false},
		{"an id held twice", "[" + entry + "," + entry + "]", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := sessionRows(t, test.index, "a.json", `{"sessionId":"a","history":{}}`)
			found, timed := 0, 0
			for _, row := range rows {
				switch {
				case row.Text == parserkit.ProblemPrefix+"history is not a JSON array":
					found++
					if (row.TimestampUTC != "" && !test.grouped) || row.SessionID != "a" || row.ProjectPath != "/srv/proj" ||
						(row.Group != nil) != test.grouped {
						t.Errorf("problem row %+v", row)
					}
				case strings.HasPrefix(row.Text, "session start"):
					if row.TimestampUTC != "" {
						timed++
					}
				}
			}
			if found == 0 || timed == 0 {
				t.Errorf("%d problem rows, %d timed start rows in %+v", found, timed, rows)
			}
		})
	}
}
