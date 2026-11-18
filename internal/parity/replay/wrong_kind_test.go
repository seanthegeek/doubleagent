//go:build parity

package replay

import (
	"slices"
	"testing"
)

// wrongKindDifferences returns the differences numbered 210 onward.
func wrongKindDifferences() []Difference {
	return []Difference{
		{Number: 210, Summary: "a record member of the wrong kind is reported, where the recording " +
			"is silent: a parse may add one system row of the file it parsed whose agent and text are " +
			"an entry of wrongKindMembers", Added: wrongKindReports},
	}
}

// wrongKindMember is an agent and the exact text of the row the binary
// gives a member of the wrong kind in a recorded case that gave no row.
type wrongKindMember struct {
	agent, text Text
}

// wrongKindMembers are the rows the binary adds, one per recorded case: a
// Cursor cloud agent list and a plugin list that hold an object, which the
// analyzer passed over in silence.
var wrongKindMembers = []wrongKindMember{
	{"cursor", "parser: cloudAgentRepository.agents.auth0%7Cuser_02 is not a JSON array"},
	{"cursor", "parser: cursor.plugins.installedIds.no-team|no-workspace is not a JSON array"},
}

// wrongKindReports takes from got each system row of file whose agent and
// text are an entry of wrongKindMembers and that want does not hold, and
// returns the rest and how many it took.
func wrongKindReports(file string, want, got []Row) ([]Row, int) {
	kept := make([]Row, 0, len(got))
	taken := 0
	for _, row := range got {
		member := wrongKindMember{row.Agent, row.Text}
		if row.TurnType == "system" && string(row.SourceFile) == file &&
			slices.Contains(wrongKindMembers, member) && !slices.Contains(want, row) {
			taken++
			continue
		}
		kept = append(kept, row)
	}
	return kept, taken
}

func TestAMemberOfTheWrongKindIsTheOnlyRowAParseMayAdd(t *testing.T) {
	const file = "/h/Library/Application Support/Cursor/User/globalStorage/state.vscdb"
	entry := Row{Agent: "cursor", TurnType: "system", Text: "plugin installed: a", SourceFile: file, SourceLine: 3}
	report := Row{Agent: "cursor", TurnType: "system", SourceFile: file, SourceLine: 27,
		Text: "parser: cursor.plugins.installedIds.no-team|no-workspace is not a JSON array"}
	changed := entry
	changed.Text = "plugin installed: b"
	otherMember := report
	otherMember.Text = "parser: cursor.plugins.installedIds.t|x is not a JSON array"
	otherAgent := report
	otherAgent.Agent = "vscode"
	elsewhere := report
	elsewhere.SourceFile = "/h/other.vscdb"
	notSystem := report
	notSystem.TurnType = "user"
	tests := []struct {
		name      string
		want, got []Row
		agree     bool
	}{
		{"an added report of a listed member passes", []Row{entry}, []Row{entry, report}, true},
		{"an added report before the other rows passes", []Row{entry}, []Row{report, entry}, true},
		{"an added report of another member fails", []Row{entry}, []Row{entry, otherMember}, false},
		{"an added report of another agent fails", []Row{entry}, []Row{entry, otherAgent}, false},
		{"an added report of another file fails", []Row{entry}, []Row{entry, elsewhere}, false},
		{"an added row with the text that is not a system row fails", []Row{entry}, []Row{entry, notSystem}, false},
		{"a changed row beside an added report fails", []Row{entry}, []Row{changed, report}, false},
		{"a dropped row beside an added report fails", []Row{entry, entry}, []Row{entry, report}, false},
		{"a report the recording holds is still compared", []Row{entry, report}, []Row{entry}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := addedRows(file, test.want, test.got)
			if agree, _ := rowsAgree(t, test.want, got); agree != test.agree {
				t.Errorf("agree %v, want %v", agree, test.agree)
			}
		})
	}
}
