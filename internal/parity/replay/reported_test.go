//go:build parity

package replay

import (
	"cmp"
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
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
			"more for a second list of secondEntryReports, in that parser's wording", Added: secondEntryReport,
			After: 170},
	}
}

// secondListEntry is a report of difference 241: the second list's report,
// and the text of the first list's report, which wrongKindReports holds, it
// comes after.
type secondListEntry struct {
	wrongKindEntry
	after string
}

// secondEntryReports returns the reports difference 241 takes: one entry
// per parser whose recorded cases hold two lists with entries of the wrong
// kind.
func secondEntryReports() []secondListEntry {
	return []secondListEntry{
		// A bug report whose diagnostics crashes and assistant turns both
		// hold an entry that is not an object.
		{wrongKindEntry{"pi", func(string) []string {
			return []string{"parser: diagnostics.crashes holds 1 entry that is not a JSON object"}
		}, addedRow{session: parseSession, project: parseProject}},
			"parser: diagnostics.assistant holds 1 entry that is not a JSON object"},
	}
}

// secondEntryReport takes from got, as wrongKindReport takes its own, the
// first system row of the parsed file whose text is one of the agent's
// secondEntryReports texts and that want does not hold, when got also
// holds the first list's report, which difference 170 takes after it.
func secondEntryReport(parse parsedFile, want, got []Row) ([]Row, string) {
	var reports []wrongKindEntry
	for _, entry := range secondEntryReports() {
		if _, first := takeRow(parse.matches(entry.after, entry.row, want), want, got); first && entry.agent == parse.agent {
			reports = append(reports, entry.wrongKindEntry)
		}
	}
	return takeReport(reports, parse, want, got)
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
// original path, its path on disk and its home-relative path, its home and
// the file's modification time as the binary writes it.
type parsedFile struct {
	agent, file, disk, rel string
	home                   *input.Home
	mtime                  string
}

// The rules of an addedRow that derive a field from the case rather than
// give it: the parsed file's modification time; the home's path; and the
// session id and project of the recorded rows of the parse.
const (
	fileTime     = "{file mtime}"
	homePath     = "{the home's path}"
	parseSession = "{the parse's session}"
	parseProject = "{the parse's project}"
)

// addedRow is the rest of a row a difference lets a parse add, beside its
// text: its time, session id, project and source line, each given or one of
// the rules above ("" for an empty field), or a time the parsed database
// holds (cell). The turn is system, the agent the parser's, the source file
// the parsed file, the host and user its home's, and every other field
// empty, so the row is compared whole.
type addedRow struct {
	time, session, project string
	line                   int
	cell                   tableCell
}

// tableCell names the cell of the parsed database that gives an added row's
// time: the table, the column, and the id of the row.
type tableCell struct {
	table, column, rowID string
}

// matches returns whether a row is the whole row a parse may add with text
// under rule, want being the recorded rows of the parse. A rule whose time
// the parse cannot give matches no row.
func (p parsedFile) matches(text string, rule addedRow, want []Row) func(Row) bool {
	timestamp, ok := p.addedTime(rule)
	if !ok {
		return func(Row) bool { return false }
	}
	expected := Row{TurnType: "system", Agent: Text(p.agent), SourceFile: Text(p.file), Text: Text(text),
		SessionID: Text(rule.session), ProjectPath: Text(rule.project), SourceLine: rule.line,
		TimestampUTC: Text(timestamp)}
	if p.home != nil {
		expected.Host, expected.User = Text(p.home.Host), Text(p.home.User)
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
	return func(row Row) bool { return row == expected }
}

// addedTime returns the time an added row under rule has, and false when
// the rule names a cell the parsed file does not hold.
func (p parsedFile) addedTime(rule addedRow) (string, bool) {
	switch {
	case rule.cell != tableCell{}:
		timestamp := p.cellTime(rule.cell)
		return timestamp, timestamp != ""
	case rule.time == fileTime:
		return p.mtime, true
	}
	return rule.time, true
}

// cellTime returns the time a cell of the parsed database holds, converted
// as the parser kit converts every column time (parser.ColumnTime), or ""
// when the file is not such a database or holds no such row. The database
// is read with the project's own reader; the replay of every recorded row
// of a table checks the conversion.
func (p parsedFile) cellTime(cell tableCell) string {
	database, err := sqlitefile.OpenPath(p.disk)
	if err != nil {
		return ""
	}
	defer func() { _ = database.Close() }()
	table, err := database.Table(cell.table)
	if err != nil || !table.Has("id") || !table.Has(cell.column) {
		return ""
	}
	row, found, err := table.Select("id", cell.column).
		Where(func(row sqlitefile.Row) bool { return row.Get("id").String() == cell.rowID }).First()
	if err != nil || !found {
		return ""
	}
	return parser.ColumnTime(row.Get(cell.column))
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
	table := Differences()
	// A difference whose row comes after another's added row applies first,
	// while that row is still there for it to find.
	slices.SortStableFunc(table, func(left, right Difference) int {
		return cmp.Compare(min(right.After, 1), min(left.After, 1))
	})
	for _, entry := range table {
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
	var second []wrongKindEntry
	for _, entry := range secondEntryReports() {
		second = append(second, entry.wrongKindEntry)
	}
	for number, entries := range map[int][]wrongKindEntry{170: wrongKindReports(), 241: second} {
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
		{"a report of the second list alone fails", piParse, []Row{entry}, []Row{entry, crashes}, false},
		{"a report of the second list before the first's passes", piParse, []Row{entry}, []Row{entry, crashes, assistant},
			true},
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

func TestATableTimeIsTheTimeTheDatabaseHoldsForTheRowsUser(t *testing.T) {
	const file, rel = "/h/.tabby/ee/db.sqlite", ".tabby/ee/db.sqlite"
	database := filepath.Join("testdata", "tabby", "users.db")
	parse := parsedFile{agent: "tabby", file: file, disk: database, rel: rel, mtime: "2026-09-01T09:00:00.152Z"}
	var text string
	for _, entry := range withheldRows() {
		if entry.label() == "tabby user auth token" {
			text = entry.text
		}
	}
	created := Row{TurnType: "system", Agent: "tabby", SourceFile: file, SourceLine: 1, Text: Text(text),
		TimestampUTC: "2026-09-01T09:00:00.000Z"}
	timed := func(timestamp string) Row {
		row := created
		row.TimestampUTC = Text(timestamp)
		return row
	}
	tests := []struct {
		name  string
		parse parsedFile
		row   Row
		agree bool
	}{
		{"the user's created_at passes", parse, created, true},
		{"two seconds before it fails", parse, timed("2026-09-01T08:59:58.000Z"), false},
		{"an hour before it, another user's created_at, fails", parse, timed("2026-09-01T08:00:00.000Z"), false},
		{"the file's modification time fails", parse, timed(parse.mtime), false},
		{"no time fails", parse, timed(""), false},
		{"a file that is not the database adds no row", parsedFile{agent: "tabby", file: file,
			disk: filepath.Join("testdata", "tabby", "users.py"), rel: rel, mtime: parse.mtime}, created, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _, _ := addedRows(test.parse, nil, []Row{test.row})
			if agree, _ := rowsAgree(t, nil, got); agree != test.agree {
				t.Errorf("agree %v, want %v", agree, test.agree)
			}
		})
	}
}
