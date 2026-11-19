//go:build parity

package replay

import (
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/input"
)

// reportedDifferences returns the differences numbered 170 onward.
func reportedDifferences() []Difference {
	return []Difference{
		{Number: 170, Summary: "a file that cannot be read as its kind is reported, where the " +
			"recording is silent, and so is a record member of the wrong kind: a parse by one of the " +
			"parsers of wrongKindReports may add one system row of the file it parsed, in that parser's wording", Added: wrongKindReport},
	}
}

// entryReportDifferences returns the differences numbered 241 onward.
func entryReportDifferences() []Difference {
	return []Difference{
		{Number: 241, Summary: "a list whose entries are not all of the kind the format gives is " +
			"reported once per list, so a parse that difference 170 lets add one such row may add one " +
			"more for a second list of secondEntryReports, in that parser's wording", Added: secondEntryReport},
	}
}

// secondEntryReports returns the reports difference 241 takes: one entry
// per parser whose recorded cases hold two lists with entries of the wrong
// kind, the agent and the text of the second list's report.
func secondEntryReports() []wrongKindEntry {
	return []wrongKindEntry{
		// A bug report whose diagnostics crashes and assistant turns both
		// hold an entry that is not an object.
		{"pi", func(string) []string {
			return []string{"parser: diagnostics.crashes holds 1 entry that is not a JSON object"}
		}, addedRow{session: parseSession, project: parseProject}},
	}
}

// secondEntryReport takes from got the first system row of the parsed file
// whose text is one of the agent's secondEntryReports texts and that want
// does not hold, as wrongKindReport takes its own.
func secondEntryReport(parse parsedFile, want, got []Row) ([]Row, string) {
	return takeReport(secondEntryReports(), parse, want, got)
}

// wrongKindEntry is one report of a file of the wrong kind that the binary
// gives and the recording lacks: the agent whose parser gives it, the texts
// it gives for a file at a home-relative path, and the rest of its row.
type wrongKindEntry struct {
	agent string
	texts func(rel string) []string
	row   addedRow
}

// label returns the name the run's log and its check of every entry use:
// the agent and the entry's first text.
func (e wrongKindEntry) label() string {
	return e.agent + ": " + e.texts("{rel}")[0]
}

// wrongKindReports returns the reports difference 170 takes, one entry per
// parser and wording; a parser that comes to report another kind of file
// adds an entry, and a run of every case fails for an entry no case used.
func wrongKindReports() []wrongKindEntry {
	return []wrongKindEntry{
		{"pi", func(rel string) []string {
			return []string{"parser: " + rel + " has no session header",
				"parser: " + rel + " member session.jsonl has no session header"}
		}, addedRow{}},
		// A bug report whose diagnostics assistant turns hold a number among
		// the objects the format gives.
		{"pi", func(string) []string {
			return []string{"parser: diagnostics.assistant holds 1 entry that is not a JSON object"}
		}, addedRow{session: parseSession, project: parseProject}},
		{"muse-code", func(string) []string {
			return []string{"parser: not a Muse session log (the first record is neither an envelope nor a " +
				"retained frame)"}
		}, addedRow{line: 1}},
		// A remote session spaces file whose entries hold a number among
		// the objects the format gives.
		{"claude-desktop", func(string) []string {
			return []string{"parser: entries holds 1 entry that is not a JSON object"}
		}, addedRow{line: 1}},
		// A cloud agent list and a plugin list that hold an object where the
		// format gives a list, in two recorded cases.
		{"cursor", func(string) []string {
			return []string{"parser: cloudAgentRepository.agents.auth0%7Cuser_02 is not a JSON array"}
		}, addedRow{line: 13}},
		{"cursor", func(string) []string {
			return []string{"parser: cursor.plugins.installedIds.no-team|no-workspace is not a JSON array"}
		}, addedRow{line: 27}},
	}
}

// wrongKindReport takes from got the first system row of the parsed file
// whose text is one of the agent's wrongKindReports texts and that want
// does not hold, and returns the rest and the agent of the entry it took
// from, or "". A parse may add one such row; a second stays and is
// compared.
func wrongKindReport(parse parsedFile, want, got []Row) ([]Row, string) {
	return takeReport(wrongKindReports(), parse, want, got)
}

// takeReport takes from got the first system row of the parsed file whose
// text is one of the texts an entry of reports gives for the parse's agent
// and that want does not hold, and returns the rest and that agent, or "".
func takeReport(reports []wrongKindEntry, parse parsedFile, want, got []Row) ([]Row, string) {
	for _, entry := range reports {
		if entry.agent != parse.agent {
			continue
		}
		for _, text := range entry.texts(parse.rel) {
			if rest, taken := takeRow(parse.matches(text, entry.row, want), want, got); taken {
				return rest, entry.label()
			}
		}
	}
	return got, ""
}

// parsedFile names one parse: the agent of its parser, the artifact's
// original path and home-relative path, its home and the file's
// modification time as the binary writes it.
type parsedFile struct {
	agent, file, rel string
	home             *input.Home
	mtime            string
}

// The rules of an addedRow that derive a field from the case rather than
// give it: the parsed file's modification time; a time the file's own table
// gives (a well-formed time no later than the file's modification time and
// no more than a day before it, since the recorder wrote the database the
// day the case ran); the home's path; and the session id and project of
// the recorded rows of the parse.
const (
	fileTime       = "{file mtime}"
	tableTime      = "{a time of the file's table}"
	homePath       = "{the home's path}"
	parseSession   = "{the parse's session}"
	parseProject   = "{the parse's project}"
	timestampShape = `^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{3}Z$`
)

// addedRow is the rest of a row a difference lets a parse add, beside its
// text: its time, session id, project and source line, each given or one of
// the rules above ("" for an empty field). The turn is system, the agent
// the parser's, the source file the parsed file, the host and user its
// home's, and every other field empty, so the row is compared whole.
type addedRow struct {
	time, session, project string
	line                   int
}

// matches returns whether a row is the whole row a parse may add with text
// under rule, want being the recorded rows of the parse.
func (p parsedFile) matches(text string, rule addedRow, want []Row) func(Row) bool {
	expected := Row{TurnType: "system", Agent: Text(p.agent), SourceFile: Text(p.file), Text: Text(text),
		SessionID: Text(rule.session), ProjectPath: Text(rule.project), SourceLine: rule.line,
		TimestampUTC: Text(rule.time)}
	if p.home != nil {
		expected.Host, expected.User = Text(p.home.Host), Text(p.home.User)
	}
	switch rule.time {
	case fileTime:
		expected.TimestampUTC = Text(p.mtime)
	case tableTime:
		expected.TimestampUTC = ""
	}
	session, project := parseSessionOf(want)
	switch rule.project {
	case homePath:
		expected.ProjectPath = ""
		if p.home != nil {
			expected.ProjectPath = Text(p.home.Original)
		}
	case parseProject:
		expected.ProjectPath = project
	}
	if rule.session == parseSession {
		expected.SessionID = session
	}
	return func(row Row) bool {
		if rule.time == tableTime {
			if !p.withinADayBefore(string(row.TimestampUTC)) {
				return false
			}
			row.TimestampUTC = ""
		}
		return row == expected
	}
}

// withinADayBefore reports whether a time has the binary's shape and lies
// no later than the parsed file's modification time and no more than a day
// before it.
func (p parsedFile) withinADayBefore(text string) bool {
	if !regexp.MustCompile(timestampShape).MatchString(text) {
		return false
	}
	written, err := time.Parse(time.RFC3339Nano, text)
	modified, mtimeErr := time.Parse(time.RFC3339Nano, p.mtime)
	if err != nil || mtimeErr != nil {
		return false
	}
	gap := modified.Sub(written)
	return gap >= 0 && gap <= 24*time.Hour
}

// parseSessionOf returns the session id and project of the first recorded
// row of a parse that has a session.
func parseSessionOf(want []Row) (Text, Text) {
	for _, row := range want {
		if row.SessionID != "" {
			return row.SessionID, row.ProjectPath
		}
	}
	return "", ""
}

// takeRow takes from got the first row that matches and that want does not
// hold, and reports whether it took one.
func takeRow(matches func(Row) bool, want []Row, got []Row) ([]Row, bool) {
	index := slices.IndexFunc(got, func(row Row) bool { return matches(row) && !slices.Contains(want, row) })
	if index < 0 {
		return got, false
	}
	return slices.Delete(slices.Clone(got), index, index+1), true
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
// wrongKindReports and secondEntryReports that no case of a whole run took
// a row from.
func checkEveryWrongKindReportUsed(t testing.TB, used map[string]int, whole bool) {
	t.Helper()
	if !whole {
		return
	}
	for number, entries := range map[int][]wrongKindEntry{170: wrongKindReports(), 241: secondEntryReports()} {
		for _, entry := range entries {
			if used[entry.label()] == 0 {
				t.Errorf("difference %d's report %s is used by no recorded case: remove it", number, entry.label())
				continue
			}
			t.Logf("difference %d's report %s used by %d cases", number, entry.label(), used[entry.label()])
		}
	}
}

func TestAReportOfAFileOfTheWrongKindIsTheOnlyRowAParseMayAdd(t *testing.T) {
	const file, rel = "/h/.pi/agent/sessions/--p--/s.jsonl", ".pi/agent/sessions/--p--/s.jsonl"
	piParse, museParse := parsedFile{agent: "pi", file: file, rel: rel}, parsedFile{agent: "muse-code", file: file, rel: rel}
	entry := Row{TurnType: "user", Text: "hello", SourceFile: file, SourceLine: 2}
	report := Row{TurnType: "system", Agent: "pi", Text: "parser: .pi/agent/sessions/--p--/s.jsonl has no session header",
		SourceFile: file}
	second := report
	second.Text = "parser: .pi/agent/sessions/--p--/s.jsonl member session.jsonl has no session header"
	second.SourceLine = 7
	otherName := report
	otherName.Text = "parser: other.jsonl has no session header"
	museReport := Row{TurnType: "system", Agent: "muse-code", SourceFile: file, SourceLine: 1,
		Text: "parser: not a Muse session log (the first record is neither an envelope nor a retained frame)"}
	changed := entry
	changed.Text = "hullo"
	other := report
	other.Text = "parser: unparseable line (not a JSON object): [1]"
	elsewhere := report
	elsewhere.SourceFile = "/h/other.jsonl"
	notSystem := report
	notSystem.TurnType = "user"
	atLine := report
	atLine.SourceLine = 77
	dated := report
	dated.TimestampUTC = "1999-01-01T00:00:00.000Z"
	inSession := report
	inSession.SessionID = "WRONG-SESSION"
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
		{"a parser without an entry adds nothing", parsedFile{agent: "codex-cli", file: file, rel: rel}, nil,
			[]Row{museReport}, false},
		{"an added report at another line fails", piParse, []Row{entry}, []Row{entry, atLine}, false},
		{"an added report with a time fails", piParse, []Row{entry}, []Row{entry, dated}, false},
		{"an added report in a session fails", piParse, []Row{entry}, []Row{entry, inSession}, false},
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

func TestDifference241LetsAParseAddASecondListsEntryReportOnly(t *testing.T) {
	const file, rel = "/h/proj/pi-bug-report-r.zip", "proj/pi-bug-report-r.zip"
	piParse := parsedFile{agent: "pi", file: file, rel: rel}
	entry := Row{TurnType: "system", Text: "bug report: r", SourceFile: file, SessionID: "s1", ProjectPath: "/p"}
	assistant := Row{TurnType: "system", Agent: "pi", SourceFile: file, SessionID: "s1", ProjectPath: "/p",
		Text: "parser: diagnostics.assistant holds 1 entry that is not a JSON object"}
	crashes := assistant
	crashes.Text = "parser: diagnostics.crashes holds 1 entry that is not a JSON object"
	inOther := crashes
	inOther.SessionID = "s2"
	twoCrashes := crashes
	twoCrashes.Text = "parser: diagnostics.crashes holds 2 entries that are not JSON objects"
	tests := []struct {
		name      string
		parse     parsedFile
		want, got []Row
		agree     bool
	}{
		{"a report of each list passes", piParse, []Row{entry}, []Row{entry, assistant, crashes}, true},
		{"a report of the second list alone passes", piParse, []Row{entry}, []Row{entry, crashes}, true},
		{"the second list's report twice fails", piParse, []Row{entry}, []Row{entry, crashes, crashes}, false},
		{"a report of another count fails", piParse, []Row{entry}, []Row{entry, twoCrashes}, false},
		{"the report added by another agent's parse fails", parsedFile{agent: "cline", file: file, rel: rel}, []Row{entry},
			[]Row{entry, crashes}, false},
		{"a report in another session fails", piParse, []Row{entry}, []Row{entry, inOther}, false},
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
