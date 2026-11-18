//go:build parity

package replay

import (
	"regexp"
	"slices"
	"testing"
)

// reportedDifferences returns the differences numbered 170 onward.
func reportedDifferences() []Difference {
	return []Difference{
		{Number: 170, Summary: "a file that cannot be read as its kind is reported, where the " +
			"recording is silent: a parse may add one system row of the file it parsed whose text is " +
			"one of the binary's own reports of a file of the wrong kind", Added: wrongKindReport},
	}
}

// wrongKindShapes are the texts the binary reports a file of the wrong kind
// in, where the analyzer gave no row: a pi session file with records but no
// header, and a Muse Code session log whose first record is not Muse.
var wrongKindShapes = regexp.MustCompile(`\Aparser: (?:[^\n]+ has no session header|` +
	`not a Muse session log \(the first record is neither an envelope nor a retained frame\))\z`)

// wrongKindReport takes from got each system row of file whose text has a
// shape of wrongKindShapes and that want does not hold, and returns the
// rest and how many it took.
func wrongKindReport(file string, want, got []Row) ([]Row, int) {
	kept := make([]Row, 0, len(got))
	taken := 0
	for _, row := range got {
		if row.TurnType == "system" && string(row.SourceFile) == file &&
			wrongKindShapes.MatchString(string(row.Text)) && !slices.Contains(want, row) {
			taken++
			continue
		}
		kept = append(kept, row)
	}
	return kept, taken
}

// addedRows applies each difference that takes an added row to a parse of
// file, and returns got without those rows and the numbers that took one.
func addedRows(file string, want, got []Row) ([]Row, []int) {
	var numbers []int
	for _, entry := range Differences() {
		if entry.Added == nil {
			continue
		}
		var taken int
		if got, taken = entry.Added(file, want, got); taken > 0 {
			numbers = append(numbers, entry.Number)
		}
	}
	return got, numbers
}

func TestAReportOfAFileOfTheWrongKindIsTheOnlyRowAParseMayAdd(t *testing.T) {
	const file = "/h/.pi/agent/sessions/--p--/s.jsonl"
	entry := Row{TurnType: "user", Text: "hello", SourceFile: file, SourceLine: 2}
	report := Row{TurnType: "system", Text: "parser: .pi/agent/sessions/--p--/s.jsonl has no session header",
		SourceFile: file}
	muse := Row{TurnType: "system", SourceFile: file, SourceLine: 1,
		Text: "parser: not a Muse session log (the first record is neither an envelope nor a retained frame)"}
	changed := entry
	changed.Text = "hullo"
	other := report
	other.Text = "parser: unparseable line (not a JSON object): [1]"
	elsewhere := report
	elsewhere.SourceFile = "/h/other.jsonl"
	notSystem := report
	notSystem.TurnType = "user"
	tests := []struct {
		name      string
		want, got []Row
		agree     bool
	}{
		{"an added report of a headerless file passes", []Row{entry}, []Row{entry, report}, true},
		{"an added report of a log that is not Muse passes", nil, []Row{muse}, true},
		{"an added problem row of another kind fails", []Row{entry}, []Row{entry, other}, false},
		{"an added report of another file fails", []Row{entry}, []Row{entry, elsewhere}, false},
		{"an added row of the shape that is not a system row fails", []Row{entry}, []Row{entry, notSystem}, false},
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
