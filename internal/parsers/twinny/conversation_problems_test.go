package twinny

import (
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A problem row of a conversation carries its session id and no time, while
// the conversation's other rows carry its updatedAt.
func TestAConversationsProblemRowsHaveNoTime(t *testing.T) {
	for _, test := range []struct {
		name, conversation, text string
	}{
		{"messages that is not an array", `{"id":"c","updatedAt":1759999205000,"messages":3}`,
			"parser: messages is not a JSON array"},
		{"toolSteps that is not an array", `{"id":"c","updatedAt":1759999205000,"messages":` +
			`[{"role":"assistant","content":"x","toolSteps":5}]}`, "parser: toolSteps is not a JSON array"},
		{"a reply id over the limit", `{"id":"c","updatedAt":1759999205000,"messages":[{"role":"assistant",` +
			`"content":"x","changes":{"id":"` + strings.Repeat("i", parserkit.CarriedLimit+1) +
			`","files":[{"path":"a"}]}}]}`, parserkit.ProblemPrefix + parserkit.CarriedProblem("reply id", 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			conversation, err := jsonvalue.DecodeString(test.conversation)
			if err != nil {
				t.Fatal(err)
			}
			artifact := writeArtifact(t, "User/globalStorage/state.vscdb", "")
			var rows []timeline.Row
			New().conversation(artifact, conversation, "", parserkit.Options{}, func(row timeline.Row) { rows = append(rows, row) })
			found, timed := 0, 0
			for _, row := range rows {
				switch {
				case row.Text == test.text:
					found++
					if row.TimestampUTC != "" || row.SessionID != "c" {
						t.Errorf("problem row has time %q, session %q", row.TimestampUTC, row.SessionID)
					}
				case row.TimestampUTC != "":
					timed++
				}
			}
			if found != 1 || timed == 0 {
				t.Errorf("%d problem rows and %d timed other rows in %+v", found, timed, rows)
			}
		})
	}
}
