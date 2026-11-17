//go:build parity

package replay

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"
)

// Difference is one numbered documented difference between the Python
// analyzer and the binary (the milestone plan, 51 onward). Each applies only
// to its own shape: Text rewrites each place the shape occurs in a text into
// a form both sides share, Rows puts rows of the shape into one order, and
// Excludes reports that a case shows a shape no rewriting can match, so the
// case is not compared. Each returns how many places it found.
type Difference struct {
	Number   int
	Summary  string
	Text     func(text string) (string, int)
	Rows     func(rows []Row) ([]Row, int)
	Excludes func(text string) bool
}

// Differences returns the table of documented differences, a fresh slice.
func Differences() []Difference {
	return []Difference{
		{Number: 51, Summary: "usage and help text are the binary's own", Text: usageText},
		{Number: 52, Summary: "library and operating-system error wording in parser: rows, " +
			"problem: lines and error: messages is the binary's own", Text: libraryErrorText},
		{Number: 53, Summary: "--match takes Go regular-expression syntax",
			Excludes: pythonOnlyPattern},
		{Number: 54, Summary: "ISO week dates, ordinal dates and non-ASCII digits are not times",
			Excludes: unacceptedTime},
		{Number: 55, Summary: "zed archived worktrees and state.vscdb value problems have one order",
			Rows: plannerOrder},
		{Number: 56, Summary: "an FTS5 table is always readable", Excludes: fts5Unreadable},
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

// NormaliseTexts applies the named differences that rewrite text to both
// texts, and fails the test for a named one that finds nothing in want.
func NormaliseTexts(t testing.TB, node string, numbers []int, want, got string) (string, string) {
	t.Helper()
	for _, number := range numbers {
		entry := difference(t, number)
		if entry.Text == nil {
			continue
		}
		var found int
		want, found = entry.Text(want)
		got, _ = entry.Text(got)
		if found == 0 {
			t.Errorf("%s: difference %d names this case but its text holds nothing to normalise",
				node, number)
		}
	}
	return want, got
}

// NormaliseRows applies the named differences to two row lists: Rows to the
// lists, Text to each row's text. It fails the test for a named one that
// finds nothing in want.
func NormaliseRows(t testing.TB, node string, numbers []int, want, got []Row) ([]Row, []Row) {
	t.Helper()
	want, got = slices.Clone(want), slices.Clone(got)
	for _, number := range numbers {
		entry := difference(t, number)
		found := 0
		switch {
		case entry.Rows != nil:
			want, found = entry.Rows(want)
			got, _ = entry.Rows(got)
		case entry.Text != nil:
			found = rewriteRowTexts(want, entry.Text)
			rewriteRowTexts(got, entry.Text)
		default:
			continue
		}
		if found == 0 {
			t.Errorf("%s: difference %d names this case but its rows hold nothing to normalise",
				node, number)
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

// usageLine starts the usage text of a command-line parser.
var usageLine = regexp.MustCompile(`(?m)^usage: `)

// usageText replaces everything from a usage line to the end of the text.
func usageText(text string) (string, int) {
	location := usageLine.FindStringIndex(text)
	if location == nil {
		return text, 0
	}
	return text[:location[0]] + "{usage}\n", 1
}

// errorMarker finds where a line's error text starts: a parser: row, a
// problem: line, an error: message, or the note a tool result carries in
// place of a persisted output that could not be read.
var errorMarker = regexp.MustCompile(`parser: |^problem: |^error: |\[persisted output `)

// libraryShapes are the library and system error details as Python words
// them, and as the binary words those it words differently, each replaced
// by a token naming its source. A detail inside JSON text stops at a quote.
var libraryShapes = []struct {
	pattern *regexp.Regexp
	token   string
}{
	{regexp.MustCompile(`[A-Z][^()"\n]*: line [0-9]+ column [0-9]+ \(char [0-9]+\)`), "{json error}"},
	{regexp.MustCompile(`invalid JSON at line [0-9]+, column [0-9]+: [^()"\n]*`), "{json error}"},
	{regexp.MustCompile(`[A-Za-z][^()"\n]*(?: \([^()"\n]*\))?: line [0-9]+, column [0-9]+`),
		"{xml error}"},
	{regexp.MustCompile(`[A-Z][^()"\n]* \(at (?:line [0-9]+, column [0-9]+|end of document)\)`),
		"{toml error}"},
	{regexp.MustCompile(`invalid TOML at line [0-9]+, column [0-9]+: [^()"\n]*`), "{toml error}"},
	{regexp.MustCompile(`SQLite error: [^"\n]*`), "SQLite error: {sqlite error}"},
	{regexp.MustCompile(`unreadable \([^()"\n]*\)`), "unreadable ({sqlite error})"},
	{regexp.MustCompile(`zstd: [^"\n]*`), "zstd: {zstd error}"},
	{regexp.MustCompile(`could not be read \([^()"\n]*\)`), "could not be read ({system error})"},
	{regexp.MustCompile(`is not a zip archive \([^()"\n]*\)`), "is not a zip archive ({zip error})"},
	{regexp.MustCompile(`is not base64 \([^()"\n]*\)`), "is not base64 ({base64 error})"},
}

// libraryErrorText rewrites the library error details of each line's error
// text, from its marker to the line's end.
func libraryErrorText(text string) (string, int) {
	lines := strings.SplitAfter(text, "\n")
	total := 0
	for i, line := range lines {
		location := errorMarker.FindStringIndex(line)
		if location == nil {
			continue
		}
		rest := line[location[1]:]
		for _, shape := range libraryShapes {
			found := len(shape.pattern.FindAllStringIndex(rest, -1))
			if found > 0 {
				rest = shape.pattern.ReplaceAllLiteralString(rest, shape.token)
				total += found
			}
		}
		lines[i] = line[:location[1]] + rest
	}
	return strings.Join(lines, ""), total
}

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

// boundArgument finds each --since or --until value, one argument per line.
var boundArgument = regexp.MustCompile(`(?m)^--(?:since|until)(?:=|\n)(.*)$`)

// weekOrOrdinal is an ISO 8601 week date or ordinal date.
var weekOrOrdinal = regexp.MustCompile(`^[0-9]{4}-?(?:W[0-9]{2}(?:-?[1-7])?|[0-9]{3})(?:[T ]|\z)`)

// unacceptedTime reports whether a --since or --until value is a week date,
// an ordinal date, or holds a digit outside ASCII.
func unacceptedTime(text string) bool {
	for _, match := range boundArgument.FindAllStringSubmatch(text, -1) {
		value := strings.TrimSpace(match[1])
		if weekOrOrdinal.MatchString(value) {
			return true
		}
		for _, character := range value {
			if unicode.IsDigit(character) && character > unicode.MaxASCII {
				return true
			}
		}
	}
	return false
}

// plannerOrderPrefixes start the texts of the rows whose order depended on
// SQLite's query planner.
var plannerOrderPrefixes = []string{"archived worktree: ", "ItemTable value for "}

// plannerOrder sorts each run of consecutive rows whose text starts with one
// of the prefixes by text, and returns how many rows it found.
func plannerOrder(rows []Row) ([]Row, int) {
	rows = slices.Clone(rows)
	found := 0
	for start := 0; start < len(rows); {
		end := start
		for end < len(rows) && plannerOrdered(string(rows[end].Text)) {
			end++
		}
		if end == start {
			start++
			continue
		}
		found += end - start
		slices.SortStableFunc(rows[start:end], func(left, right Row) int {
			return cmp.Compare(left.Text, right.Text)
		})
		start = end
	}
	return rows, found
}

// plannerOrdered reports whether a row's text has one of the prefixes; a
// problem row carries its prefix after "parser: ".
func plannerOrdered(text string) bool {
	text = strings.TrimPrefix(text, "parser: ")
	for _, prefix := range plannerOrderPrefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

// fts5Unreadable reports whether the expected text shows Python's SQLite
// without the FTS5 module failing on a table.
func fts5Unreadable(text string) bool {
	return strings.Contains(text, "no such module: fts5")
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
