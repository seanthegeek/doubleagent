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
// the binary, numbered from 101 in the milestone plan. Each applies only to
// its shape: Text rewrites it into a form both sides share, Rows puts its
// rows in one order, Excludes reports a case no rewriting can match,
// Outputs rewrites a run's outputs against the recorded ones and the
// records the recorded parses read, Added takes a row the recording lacks
// from a parse, Arguments rewrites a command line; each returns what it
// found. Scope names the texts a difference of format applies to. A retired
// one keeps its number and applies nowhere; Unchecked says why a run that
// never uses one does not fail.
type Difference struct {
	Number    int
	Summary   string
	Text      func(text string) (string, int)
	Rows      func(rows []Row) ([]Row, int)
	Excludes  func(text string) bool
	Outputs   func(want, got map[string]string, held heldRecords) int
	Arguments func(command string, args []string) ([]string, int)
	Scope     string
	Added     func(file string, want, got []Row) ([]Row, int)
	Retired   string
	Unchecked string
}

// Differences returns the table of documented differences, a fresh slice.
func Differences() []Difference {
	return append([]Difference{
		{Number: 101, Summary: "usage and help text are the binary's own", Retired: retiredUnused},
		{Number: 102, Summary: "library and operating-system error wording in parser: rows, " +
			"problem: lines and error: messages is the binary's own", Text: libraryErrorText},
		{Number: 103, Summary: "--match takes Go regular-expression syntax",
			Excludes: pythonOnlyPattern},
		{Number: 104, Summary: "ISO week dates, ordinal dates and non-ASCII digits are not times",
			Retired: retiredUnused},
		{Number: 105, Summary: "zed archived worktrees and state.vscdb value problems have one order",
			Rows: plannerOrder},
		{Number: 106, Summary: "an FTS5 table is always readable", Retired: retiredUnused},
		{Number: 107, Summary: "a loose tree is listed in name order, not the file system's: of two " +
			"files the recording's parses both read the same record from, the run may keep the row " +
			"of the other",
			Outputs: otherOfEqualRows, Unchecked: "whether a case needs it depends on the order " +
				"the recording machine's file system listed the tree in"},
	}, slices.Concat(formatDifferences(), laterDifferences(), reportedDifferences())...)
}

// retiredUnused is why a difference was retired: no recorded case shows
// its shape, so it could only let a case pass that should fail.
const retiredUnused = "no recorded case used it"

// laterDifferences returns the differences numbered after the differences
// of format, 120 onward.
func laterDifferences() []Difference {
	return []Difference{
		{Number: 120, Summary: "a value an error: message quotes is in Go's double-quoted form",
			Text: quotedValueText},
		{Number: 121, Summary: "options come before the input", Arguments: optionsFirst},
		{Number: 122, Summary: "a note counting one manifest row that did not parse says row",
			Text: oneManifestRowText},
		{Number: 130, Summary: "a file that holds secrets gives only where it stopped decoding, " +
			"not the reader's detail", Text: secretFileErrorText},
		{Number: 140, Summary: "the accepted forms of a time name epoch seconds or milliseconds, " +
			"both of which are read", Text: epochUnitsText},
		{Number: 150, Summary: "a database's -shm file is never read, so it is never a problem",
			Excludes: sharedMemoryProblem},
		{Number: 160, Summary: "a JSON array that is cut off, or has data after it, is reported in " +
			"one wording", Text: arrayDamageText},
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

// secretFileErrors are the rows of files that hold API keys or tokens whose
// reason the binary gives as a position alone, and what each becomes: the
// recorded reason, or the binary's position, or none, is one token.
var secretFileErrors = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(parser: config\.toml is not valid TOML \()(?:[^()\n]|\([^()\n]*\))*\)`),
		"${1}{position})"},
	{regexp.MustCompile(`(parser: config\.json did not parse: )[^\n]*`), "${1}{position}"},
	{regexp.MustCompile(`(parser: [^\n]*crush\.json is not a JSON object)(?: \(invalid JSON at line [0-9]+, ` +
		`column [0-9]+\))?`), "${1}"},
	{regexp.MustCompile(`(exec_approvals_config raw_json unparseable \()[^;\n]*(; text withheld)`),
		"${1}{position}${2}"},
}

// secretFileErrorText rewrites the reason of each secretFileErrors row and
// returns the count.
func secretFileErrorText(text string) (string, int) {
	found := 0
	for _, shape := range secretFileErrors {
		text = shape.pattern.ReplaceAllStringFunc(text, func(match string) string {
			found++
			return shape.pattern.ReplaceAllString(match, shape.replacement)
		})
	}
	return text, found
}

// oneManifestRow is the note of one manifest row that did not parse, in
// either number.
var oneManifestRow = regexp.MustCompile(`(^|[^0-9])1 manifest rows? did not parse`)

// oneManifestRowText writes each note of oneManifestRow in the singular and
// returns the count.
func oneManifestRowText(text string) (string, int) {
	found := len(oneManifestRow.FindAllStringIndex(text, -1))
	return oneManifestRow.ReplaceAllString(text, "${1}1 manifest row did not parse"), found
}

// epochUnits is the epoch entry of the accepted forms an error: message
// lists after a time it cannot read: "epoch seconds" as recorded, "epoch
// seconds or milliseconds" as the binary names what it reads.
var epochUnits = regexp.MustCompile(`(?m)(^error: .*; accepted: .*; 2026-10 or 2026; )` +
	`epoch seconds(?: or milliseconds)?(; )`)

// epochUnitsText writes the epoch entry of epochUnits as one token and
// returns the count.
func epochUnitsText(text string) (string, int) {
	found := 0
	text = epochUnits.ReplaceAllStringFunc(text, func(match string) string {
		found++
		parts := epochUnits.FindStringSubmatch(match)
		return parts[1] + "{epoch units}" + parts[2]
	})
	return text, found
}

// quotedValue is a value an error: message quotes at the start of its
// reason, in Python's single quotes or Go's double quotes; a value holding a
// quote or a backslash, which the two forms escape differently, is left to
// differ.
var quotedValue = regexp.MustCompile(`(?m)^(error: (?:cannot read |--match )?)` +
	`(?:'([^'"\\\n]*)'|"([^'"\\\n]*)")`)

// quotedValueText writes each quoted value of quotedValue in one form and
// returns the count.
func quotedValueText(text string) (string, int) {
	found := 0
	text = quotedValue.ReplaceAllStringFunc(text, func(match string) string {
		found++
		parts := quotedValue.FindStringSubmatch(match)
		return parts[1] + "{quoted " + parts[2] + parts[3] + "}"
	})
	return text, found
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
	// Messages the analyzer worded through Python's exception names, its
	// UTF-8 codec or its shell splitter, each beside the binary's wording of
	// the same condition. They come first: the JSON detail after
	// "JSONDecodeError: " would otherwise be taken alone below.
	{regexp.MustCompile(`JSONDecodeError: (?:Expecting|Unterminated|Invalid|Extra data|Illegal|Unexpected UTF-8 BOM)` +
		`[^()"\n]*: line [0-9]+ column [0-9]+ \(char [0-9]+\)`), "{json error}"},
	{regexp.MustCompile(`\((?:ValueError: not a JSON object|the thread is not a JSON object)\)`),
		"({thread not an object})"},
	{regexp.MustCompile(`created_git_worktrees: (?:KeyError: 'created_at_seconds'|no created_at_seconds field)\)`),
		"created_git_worktrees: {no creation time})"},
	{regexp.MustCompile(`'utf-8' codec can't decode (?:byte 0x[0-9a-f]{2} in position [0-9]+|` +
		`bytes in position [0-9]+-[0-9]+): (?:invalid start byte|invalid continuation byte|unexpected end of data)` +
		`|not valid UTF-8 at byte offset [0-9]+`), "{utf-8 error}"},
	{regexp.MustCompile(`mutation not applied \(path \[[^()\n]*\] not found\)`),
		"mutation not applied (path {mutation path} not found)"},
	{regexp.MustCompile(`unparseable line \((?:No closing quotation|a quote is not closed)\)`),
		"unparseable line ({unclosed quote})"},
	// Python's messages start with one of a few words, so a record's key or
	// a lead-in such as "JSON: " before one is not taken into the detail.
	{regexp.MustCompile(`(?:Expecting|Unterminated|Invalid|Extra data|Illegal|Unexpected UTF-8 BOM)` +
		`[^()"\n]*: line [0-9]+ column [0-9]+ \(char [0-9]+\)`), "{json error}"},
	// The binary's message holds no semicolon, so a note after one stays.
	{regexp.MustCompile(`invalid JSON at line [0-9]+, column [0-9]+: [^();"\n]*`), "{json error}"},
	{regexp.MustCompile(`[A-Za-z][^()"\n]*(?: \([^()"\n]*\))?: line [0-9]+, column [0-9]+`),
		"{xml error}"},
	{regexp.MustCompile(`[A-Z][^()"\n]* \(at (?:line [0-9]+, column [0-9]+|end of document)\)`),
		"{toml error}"},
	{regexp.MustCompile(`invalid TOML at line [0-9]+, column [0-9]+: [^()"\n]*`), "{toml error}"},
	{regexp.MustCompile(`SQLite error: [^"\n]*`), "SQLite error: {sqlite error}"},
	{regexp.MustCompile(`database could not be read: [^"\n]*`),
		"database could not be read: {sqlite error}"},
	{regexp.MustCompile(`state\.vscdb unreadable: [^"\n]*`), "state.vscdb unreadable: {sqlite error}"},
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
