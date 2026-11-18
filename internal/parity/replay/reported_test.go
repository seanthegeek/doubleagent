//go:build parity

package replay

import (
	"slices"
	"testing"
)

// reportedDifferences returns the differences numbered 170 onward.
func reportedDifferences() []Difference {
	return []Difference{
		{Number: 170, Summary: "a file that cannot be read as its kind is reported, where the " +
			"recording is silent, and so is a record member of the wrong kind: a parse by one of the " +
			"parsers of wrongKindReports may add one system row of the file it parsed, in that parser's wording", Added: wrongKindReport},
	}
}

// wrongKindEntry is one report of a file of the wrong kind that the binary
// gives and the recording lacks: the agent whose parser gives it, and the
// texts it gives for a file at a home-relative path.
type wrongKindEntry struct {
	agent string
	texts func(rel string) []string
}

// wrongKindReports returns the reports difference 170 takes, one entry per
// parser and wording; a parser that comes to report another kind of file
// adds an entry, and a run of every case fails for an entry no case used.
func wrongKindReports() []wrongKindEntry {
	return []wrongKindEntry{
		{"pi", func(rel string) []string {
			return []string{"parser: " + rel + " has no session header",
				"parser: " + rel + " member session.jsonl has no session header"}
		}},
		{"muse-code", func(string) []string {
			return []string{"parser: not a Muse session log (the first record is neither an envelope nor a " +
				"retained frame)"}
		}},
		// A cloud agent list and a plugin list that hold an object where the
		// format gives a list, in two recorded cases.
		{"cursor", func(string) []string {
			return []string{"parser: cloudAgentRepository.agents.auth0%7Cuser_02 is not a JSON array",
				"parser: cursor.plugins.installedIds.no-team|no-workspace is not a JSON array"}
		}},
	}
}

// wrongKindReport takes from got the first system row of the parsed file
// whose text is one of the agent's wrongKindReports texts and that want
// does not hold, and returns the rest and the agent of the entry it took
// from, or "". A parse may add one such row; a second stays and is
// compared.
func wrongKindReport(parse parsedFile, want, got []Row) ([]Row, string) {
	for _, entry := range wrongKindReports() {
		if entry.agent != parse.agent {
			continue
		}
		texts := entry.texts(parse.rel)
		index := slices.IndexFunc(got, func(row Row) bool {
			return row.TurnType == "system" && string(row.SourceFile) == parse.file &&
				slices.Contains(texts, string(row.Text)) && !slices.Contains(want, row)
		})
		if index >= 0 {
			return slices.Delete(slices.Clone(got), index, index+1), entry.agent
		}
	}
	return got, ""
}

// parsedFile names one parse: the agent of its parser, and the artifact's
// original path and home-relative path.
type parsedFile struct {
	agent, file, rel string
}

// addedRows applies each difference that takes an added row to one parse,
// and returns got without those rows, the numbers that took one and the
// entries they took from.
func addedRows(parse parsedFile, want, got []Row) ([]Row, []int, []string) {
	var numbers []int
	var entries []string
	for _, entry := range Differences() {
		if entry.Added == nil {
			continue
		}
		var taken string
		if got, taken = entry.Added(parse, want, got); taken != "" {
			numbers = append(numbers, entry.Number)
			entries = append(entries, taken)
		}
	}
	return got, numbers, entries
}

// checkEveryWrongKindReportUsed fails the test for each entry of
// wrongKindReports that no case of a whole run took a row from.
func checkEveryWrongKindReportUsed(t testing.TB, used map[string]int, whole bool) {
	t.Helper()
	if !whole {
		return
	}
	for _, entry := range wrongKindReports() {
		if used[entry.agent] == 0 {
			t.Errorf("difference 170's report of %s is used by no recorded case: remove it", entry.agent)
			continue
		}
		t.Logf("difference 170's report of %s used by %d cases", entry.agent, used[entry.agent])
	}
}

func TestAReportOfAFileOfTheWrongKindIsTheOnlyRowAParseMayAdd(t *testing.T) {
	const file, rel = "/h/.pi/agent/sessions/--p--/s.jsonl", ".pi/agent/sessions/--p--/s.jsonl"
	piParse, museParse := parsedFile{"pi", file, rel}, parsedFile{"muse-code", file, rel}
	entry := Row{TurnType: "user", Text: "hello", SourceFile: file, SourceLine: 2}
	report := Row{TurnType: "system", Text: "parser: .pi/agent/sessions/--p--/s.jsonl has no session header",
		SourceFile: file}
	second := report
	second.Text = "parser: .pi/agent/sessions/--p--/s.jsonl member session.jsonl has no session header"
	second.SourceLine = 7
	otherName := report
	otherName.Text = "parser: other.jsonl has no session header"
	museReport := Row{TurnType: "system", SourceFile: file, SourceLine: 1,
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
		parse     parsedFile
		want, got []Row
		agree     bool
	}{
		{"an added report of a headerless file passes", piParse, []Row{entry}, []Row{entry, report}, true},
		{"an added report of a log that is not Muse passes", museParse, nil, []Row{museReport}, true},
		{"an added problem row of another kind fails", piParse, []Row{entry}, []Row{entry, other}, false},
		{"an added report of another file fails", piParse, []Row{entry}, []Row{entry, elsewhere}, false},
		{"an added row of the shape that is not a system row fails", piParse, []Row{entry}, []Row{entry, notSystem}, false},
		{"a changed row beside an added report fails", piParse, []Row{entry}, []Row{changed, report}, false},
		{"a dropped row beside an added report fails", piParse, []Row{entry, entry}, []Row{entry, report}, false},
		{"a report the recording holds is still compared", piParse, []Row{entry, report}, []Row{entry}, false},
		{"a second added report of one parse fails", piParse, []Row{entry}, []Row{entry, report, second}, false},
		{"a report naming another file fails", piParse, []Row{entry}, []Row{entry, otherName}, false},
		{"a report of one parser added by another fails", museParse, []Row{entry}, []Row{entry, report}, false},
		{"a parser without an entry adds nothing", parsedFile{"codex-cli", file, rel}, nil, []Row{museReport}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _, _ := addedRows(test.parse, test.want, test.got)
			if agree, _ := rowsAgree(t, test.want, got); agree != test.agree {
				t.Errorf("agree %v, want %v", agree, test.agree)
			}
		})
	}
}
