//go:build parity

package replay

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Difference is one analysis difference between the Python analyzer and
// the binary, numbered from 101. Text rewrites the recorded shape, and
// Binary the binary's own, into one form, so the binary printing the
// recorded wording still differs; Excludes reports a case no rewriting can
// match, Outputs rewrites a run's outputs, Added takes a
// row the recording lacks, Arguments rewrites a command line; each returns
// what it found. Scope names the texts a difference of format applies to;
// Within names a difference that can lie inside this one's rows, which this
// one applies before. A retired one applies nowhere; Unchecked says why a
// run that never uses one does not fail.
type Difference struct {
	Number    int
	Summary   string
	Text      func(text string) (string, int)
	Binary    func(text string) (string, int)
	Excludes  func(text string) bool
	Outputs   func(want, got map[string]string, held heldRecords) int
	Arguments func(command string, args []string) ([]string, int)
	Scope     string
	Added     func(parse parsedFile, want, got []Row) ([]Row, string)
	Within    int
	Retired   string
	Unchecked string
}

// Differences returns the table of documented differences, a fresh slice.
func Differences() []Difference {
	return append([]Difference{
		{Number: 101, Summary: "usage and help text are the binary's own", Retired: retiredUnused},
		{Number: 102, Summary: "library and operating-system error wording in parser: rows, " +
			"problem: lines and error: messages is the binary's own", Text: libraryText(recordedSide),
			Binary: libraryText(binarySide)},
		{Number: 103, Summary: "--match takes Go regular-expression syntax",
			Excludes: pythonOnlyPattern},
		{Number: 104, Summary: "ISO week dates, ordinal dates and non-ASCII digits are not times",
			Retired: retiredUnused},
		{Number: 105, Summary: "zed archived worktrees and state.vscdb value problems have one order",
			Retired: retiredUnneeded},
		{Number: 106, Summary: "an FTS5 table is always readable", Retired: retiredUnused},
		{Number: 107, Summary: "a loose tree is listed in name order, not the file system's: of two " +
			"files the recording's parses both read the same record from, the run may keep the row " +
			"of the other",
			Outputs: otherOfEqualRows, Unchecked: "whether a case needs it depends on the order " +
				"the recording machine's file system listed the tree in"},
	}, slices.Concat(formatDifferences(), laterDifferences(), reportedDifferences(),
		[]Difference{withheldDifference()})...)
}

// retiredUnneeded is why a difference was retired that recorded cases
// showed the shape of: the replay passes with it turned off.
const retiredUnneeded = "no recorded case needed it: every case passes without it"

// retiredUnused is why a difference was retired: no recorded case shows
// its shape, so it could only let a case pass that should fail.
const retiredUnused = "no recorded case used it"

// laterDifferences returns the differences numbered after the differences
// of format, 120 onward.
func laterDifferences() []Difference {
	return []Difference{
		{Number: 120, Summary: "a value an error: message quotes is in Go's double-quoted form",
			Text: quotedValueText(singleQuoted), Binary: quotedValueText(doubleQuoted)},
		{Number: 121, Summary: "options come before the input, and timeline's output directory is its second " +
			"argument", Arguments: optionsFirst},
		{Number: 122, Summary: "a note counting one manifest row that did not parse says row",
			Text: oneManifestRowText, Binary: asWritten},
		{Number: 130, Summary: "a file that holds secrets gives only where it stopped decoding, " +
			"not the reader's detail: the line and column are compared", Retired: "folded into difference 230, " +
			"since the binary now writes the withheld reader's detail as the placeholder"},
		{Number: 140, Summary: "the accepted forms of a time name epoch seconds or milliseconds, " +
			"both of which are read", Text: epochUnitsText(recordedEpochUnits),
			Binary: epochUnitsText(binaryEpochUnits)},
		{Number: 150, Summary: "a database's -shm file is never read, so it is never a problem: a case " +
			"whose recording reports one is not compared, and since the recording stopped at that row, " +
			"such a case says nothing about how the binary reads a database beside an unreadable -shm",
			Excludes: sharedMemoryProblem},
		{Number: 160, Summary: "a JSON array that is cut off, or has data after it, is reported in " +
			"one wording", Text: arrayDamageText(recordedSide), Binary: arrayDamageText(binarySide), Within: 102},
	}
}

// sharedMemoryLine is a parser row reporting a database's -shm file as
// unreadable, which the binary never reads.
var sharedMemoryLine = regexp.MustCompile(`(?m)^parser: [^\n]*-shm could not be read \(`)

// sharedMemoryProblem reports whether the rows report an unreadable -shm
// file.
func sharedMemoryProblem(text string) bool {
	return sharedMemoryLine.MatchString(text)
}

// asWritten is the rewrite of the binary's side of a difference whose
// binary form is the compared form: none.
func asWritten(text string) (string, int) {
	return text, 0
}

// oneManifestRow is the note of one manifest row that did not parse, in
// either number as recorded; the binary writes the singular, which is
// compared as written.
var oneManifestRow = regexp.MustCompile(`(^|[^0-9])1 manifest rows? did not parse`)

// oneManifestRowText writes each note of oneManifestRow in the singular and
// returns the count.
func oneManifestRowText(text string) (string, int) {
	found := len(oneManifestRow.FindAllStringIndex(text, -1))
	return oneManifestRow.ReplaceAllString(text, "${1}1 manifest row did not parse"), found
}

// The epoch entry of the accepted forms an error: message lists after a
// time it cannot read: "epoch seconds" as recorded, "epoch seconds or
// milliseconds" as the binary names what it reads.
var (
	recordedEpochUnits = regexp.MustCompile(`(?m)(^error: .*; accepted: .*; 2026-10 or 2026; )epoch seconds(; )`)
	binaryEpochUnits   = regexp.MustCompile(`(?m)(^error: .*; accepted: .*; 2026-10 or 2026; )` +
		`epoch seconds or milliseconds(; )`)
)

// epochUnitsText returns the rewrite that writes the epoch entry of
// pattern as one token, with the count.
func epochUnitsText(pattern *regexp.Regexp) func(text string) (string, int) {
	return func(text string) (string, int) {
		found := len(pattern.FindAllStringIndex(text, -1))
		return pattern.ReplaceAllString(text, "${1}{epoch units}${2}"), found
	}
}

// A value an error: message quotes at the start of its reason: in
// Python's single quotes as recorded, in Go's double quotes as the binary
// writes it. A value holding a quote or a backslash, which the two forms
// escape differently, is left to differ.
var (
	singleQuoted = regexp.MustCompile(`(?m)^(error: (?:cannot read |--match )?)'([^'"\\\n]*)'`)
	doubleQuoted = regexp.MustCompile(`(?m)^(error: (?:cannot read |--match )?)"([^'"\\\n]*)"`)
)

// quotedValueText returns the rewrite that writes each quoted value of
// pattern in one form, with the count.
func quotedValueText(pattern *regexp.Regexp) func(text string) (string, int) {
	return func(text string) (string, int) {
		found := len(pattern.FindAllStringIndex(text, -1))
		return pattern.ReplaceAllString(text, "${1}{quoted ${2}}"), found
	}
}

// difference returns the table entry numbered number.
func difference(t testing.TB, number int) Difference {
	t.Helper()
	for _, entry := range Differences() {
		if entry.Number == number {
			return entry
		}
	}
	t.Fatalf("no documented difference %d", number)
	return Difference{}
}

// applyingOrder returns the stable order the differences of table apply
// in: each that applies before another (Within) first, then the rest.
func applyingOrder(table []Difference) []Difference {
	ordered := slices.Clone(table)
	slices.SortStableFunc(ordered, func(left, right Difference) int {
		return cmp.Compare(min(right.Within, 1), min(left.Within, 1))
	})
	return ordered
}

// named returns the entries of table numbered by numbers, in applying
// order, failing the test for a number the table lacks.
func named(t testing.TB, table []Difference, numbers []int) []Difference {
	t.Helper()
	var entries []Difference
	for _, entry := range applyingOrder(table) {
		if slices.Contains(numbers, entry.Number) {
			entries = append(entries, entry)
		}
	}
	for _, number := range numbers {
		if !slices.ContainsFunc(entries, func(entry Difference) bool { return entry.Number == number }) {
			t.Fatalf("no documented difference %d", number)
		}
	}
	return entries
}

// NormaliseTexts applies the named differences that rewrite text, each
// side's rewrite to its own text, and fails the test for a named one that
// finds nothing in want.
func NormaliseTexts(t testing.TB, node string, numbers []int, want, got string) (string, string) {
	t.Helper()
	return normaliseTextsOf(t, Differences(), node, numbers, want, got)
}

// normaliseTextsOf is NormaliseTexts over the differences of table.
func normaliseTextsOf(t testing.TB, table []Difference, node string, numbers []int, want,
	got string) (string, string) {
	t.Helper()
	for _, entry := range named(t, table, numbers) {
		if entry.Text == nil {
			continue
		}
		var found int
		want, found = entry.Text(want)
		got, _ = entry.Binary(got)
		if found == 0 {
			t.Errorf("%s: difference %d names this case but its text holds nothing to normalise",
				node, entry.Number)
		}
	}
	return want, got
}

// NormaliseRows applies the named differences of text to two row lists,
// each side's rewrite to that side's row texts. It fails the
// test for a named one that finds nothing in want.
func NormaliseRows(t testing.TB, node string, numbers []int, want, got []Row) ([]Row, []Row) {
	t.Helper()
	want, got = slices.Clone(want), slices.Clone(got)
	for _, entry := range named(t, Differences(), numbers) {
		if entry.Text == nil {
			continue
		}
		found := rewriteRowTexts(want, entry.Text)
		rewriteRowTexts(got, entry.Binary)
		if found == 0 {
			t.Errorf("%s: difference %d names this case but its rows hold nothing to normalise",
				node, entry.Number)
		}
	}
	return want, got
}

// rewriteRowTexts rewrites each row's text in place and returns the count of
// places rewritten.
func rewriteRowTexts(rows []Row, rewrite func(string) (string, int)) int {
	total := 0
	for i := range rows {
		text, found := rewrite(string(rows[i].Text))
		rows[i].Text = Text(text)
		total += found
	}
	return total
}

// Excluded reports whether a named excluding difference shows in text (a
// command's arguments, or the expected rows' texts, one per line), so the
// case is not compared. It fails the test when excluding differences are
// named and none shows.
func Excluded(t testing.TB, node string, numbers []int, text string) bool {
	t.Helper()
	named := false
	for _, number := range numbers {
		entry := difference(t, number)
		if entry.Excludes == nil {
			continue
		}
		named = true
		if entry.Excludes(text) {
			return true
		}
	}
	if named {
		t.Errorf("%s: an excluding difference names this case but its text shows none", node)
	}
	return false
}

// errorMarker finds where a line's error text starts: a parser: row, a
// problem: line, an error: message, or the note a tool result carries in
// place of a persisted output that could not be read.
var errorMarker = regexp.MustCompile(`parser: |^problem: |^error: |\[persisted output `)

// matchArgument finds each --match value in a command's arguments, one
// argument per line.
var matchArgument = regexp.MustCompile(`(?m)^--match(?:=|\n)(.*)$`)

// pythonOnlySyntax is syntax Python's re accepts that Go's regexp refuses or
// reads differently: lookaround, backreferences, named backreferences,
// comments, possessive and atomic groups, and \Z.
var pythonOnlySyntax = regexp.MustCompile(`\(\?[=!]|\(\?<[=!]|\\[1-9]|\(\?P=|\(\?#|\(\?>|[*+?}]\+|\\Z`)

// pythonOnlyPattern reports whether a --match value uses Python-only syntax
// or does not compile in Go.
func pythonOnlyPattern(text string) bool {
	for _, match := range matchArgument.FindAllStringSubmatch(text, -1) {
		if pythonOnlySyntax.MatchString(match[1]) {
			return true
		}
		if _, err := regexp.Compile(match[1]); err != nil {
			return true
		}
	}
	return false
}

// ArgumentsText returns a command case's arguments one per line, the text
// Excluded reads for a command.
func ArgumentsText(command CommandCase) string {
	arguments := make([]string, len(command.Argv))
	for i, argument := range command.Argv {
		arguments[i] = string(argument)
	}
	return strings.Join(arguments, "\n")
}

// RowsText returns the rows' texts one per line, the text Excluded reads for
// a parser case.
func RowsText(rows []Row) string {
	texts := make([]string, len(rows))
	for i, row := range rows {
		texts[i] = string(row.Text)
	}
	return strings.Join(texts, "\n")
}
