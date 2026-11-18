//go:build parity

package replay

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

// The numbered differences of format, which the information comparison
// applies where a recorded text and the binary's differ.
const (
	differenceJSONValues   = 108
	differenceCSVRecords   = 110
	differenceBoolean      = 113
	differenceEmbeddedJSON = 116
	differenceRoundedTime  = 119
)

// The scopes a difference of format names: the texts it applies to.
const (
	scopeOutputs = "the JSON outputs and detect --json"
	scopeCSV     = "the fleet CSV"
	scopeRows    = "row texts and the formatted fields of a row"
	scopeTimes   = "a row's timestamp and the timestamps of output lines"
)

// retiredFormat is why a difference of format was retired.
const retiredFormat = "no recorded case needed it, and it loosened every comparison"

// formatDifferences returns the differences of format, 108 onward, each with
// the rule of the plan it lets the binary follow. A retired one keeps its
// number and applies nowhere.
func formatDifferences() []Difference {
	return []Difference{
		{Number: differenceJSONValues, Summary: "JSON output is written by encoding/json: a JSON " +
			"output is compared with each object's members sorted by key, a key written twice kept " +
			"twice, and spacing, escaping and the final newline not compared; numbers are compared " +
			"as written", Scope: scopeOutputs},
		{Number: 109, Summary: "a lone surrogate becomes U+FFFD as the standard library does",
			Retired: retiredFormat},
		{Number: differenceCSVRecords, Summary: "the CSV uses encoding/csv's defaults: it is compared as " +
			"the records encoding/csv reads, its quoting and line ends not compared; a blank line " +
			"is compared", Scope: scopeCSV},
		{Number: 111, Summary: "length and padding count runes: a table is compared by column, not by " +
			"its padding", Retired: retiredFormat},
		{Number: 112, Summary: "a path is printed as given", Retired: retiredFormat},
		{Number: differenceBoolean, Summary: "a boolean is true or false: a formatted field, or a " +
			"value after = or \": \" outside prose, compares True and False with true and false",
			Scope: scopeRows},
		{Number: 114, Summary: "a null is empty", Retired: retiredFormat},
		{Number: 115, Summary: "a number from the evidence keeps its source text: numbers are " +
			"compared by value", Retired: retiredFormat},
		{Number: differenceEmbeddedJSON, Summary: "JSON embedded in a row is compact, keys in the " +
			"evidence's order: a JSON object or array in a row's text is compared with each object's " +
			"members sorted by key, a key written twice kept twice, spacing not compared and numbers " +
			"as written", Scope: scopeRows},
		{Number: 117, Summary: "a list or object in a row is JSON, not Python's repr", Retired: retiredFormat},
		{Number: 118, Summary: "whitespace is unicode.IsSpace: U+001C to U+001F are not trimmed",
			Retired: retiredFormat},
		{Number: differenceRoundedTime, Summary: "a timestamp's fraction is cut to the millisecond, " +
			"never rounded up into the next one: a row's timestamp, or a timestamp that is a whole " +
			"JSON string or a whole word of an output line otherwise equal, may be one millisecond " +
			"earlier when it is the cut time of a source with digits below the millisecond: a file's " +
			"modification time, or a time a file writes as text with more than three fraction digits",
			Scope: scopeTimes, Unchecked: "whether a case needs it depends on the modification times the " +
				"recording's files were given when it ran, which a new recording changes"},
	}
}

// rowFormatNumbers are the differences of format a parser case's rows are
// compared under.
func rowFormatNumbers() []int {
	return []int{differenceBoolean, differenceEmbeddedJSON, differenceRoundedTime}
}

// commandFormatNumbers are the differences of format a command case is
// compared under: all that are not retired.
func commandFormatNumbers() []int {
	var numbers []int
	for _, entry := range formatDifferences() {
		if entry.Retired == "" {
			numbers = append(numbers, entry.Number)
		}
	}
	return numbers
}

// enabledSet holds the differences of format one comparison applies.
type enabledSet map[int]bool

// setOf returns the set of numbers.
func setOf(numbers []int) enabledSet {
	set := enabledSet{}
	for _, number := range numbers {
		set[number] = true
	}
	return set
}

// neededDifferences returns the differences of numbers a comparison needs,
// for a comparison that agrees with all of them: none when it agrees with
// none, else what is left after leaving each out in turn while it still
// agrees.
func neededDifferences(numbers []int, agree func(enabled enabledSet) bool) []int {
	if agree(enabledSet{}) {
		return nil
	}
	enabled := setOf(numbers)
	for _, number := range numbers {
		delete(enabled, number)
		if !agree(enabled) {
			enabled[number] = true
		}
	}
	return slices.Sorted(maps.Keys(enabled))
}

// neededTally counts, by difference of format, the cases that needed it.
type neededTally map[int][]string

// note records the differences one case needed.
func (n neededTally) note(node string, numbers []int) {
	for _, number := range numbers {
		n[number] = append(n[number], node)
	}
}

// logNeeded logs, under label, for each of numbers how many cases needed
// it and the first of them.
func logNeeded(t testing.TB, label string, tally neededTally, numbers []int) {
	const named = 3
	for _, number := range numbers {
		nodes := tally[number]
		shown := ""
		if len(nodes) > 0 {
			shown = ": " + strings.Join(nodes[:min(len(nodes), named)], ", ")
			if len(nodes) > named {
				shown += ", ..."
			}
		}
		t.Logf("%s, difference %d needed by %d cases%s", label, number, len(nodes), shown)
	}
}

// compareRowInformation compares two row lists that differ as information:
// field by field, under the differences of format, cut holding the times
// difference 119 may find one millisecond earlier. It reports rows that
// still differ, in their compared form, and returns the differences the
// case needed.
func compareRowInformation(t testing.TB, node string, want, got []Row, cut *cutTimes) []int {
	t.Helper()
	if slices.Equal(want, got) {
		return nil
	}
	numbers := rowFormatNumbers()
	agree := func(enabled enabledSet) bool {
		formWant, formGot := informationRows(want, got, enabled, cut)
		return slices.Equal(formWant, formGot)
	}
	if !agree(setOf(numbers)) {
		formWant, formGot := informationRows(want, got, setOf(numbers), cut)
		DiffRows(t, node, formWant, formGot)
		return nil
	}
	return neededDifferences(numbers, agree)
}

// informationRows returns the compared form of each pair of rows at one
// index that differ; equal rows, and rows past the shorter list, are kept.
func informationRows(want, got []Row, enabled enabledSet, cut *cutTimes) ([]Row, []Row) {
	want, got = slices.Clone(want), slices.Clone(got)
	for i := range min(len(want), len(got)) {
		if want[i] != got[i] {
			want[i], got[i] = rowForm(want[i], enabled), rowForm(got[i], enabled)
			if enabled[differenceRoundedTime] && sameOrOneLater(string(want[i].TimestampUTC),
				string(got[i].TimestampUTC), cut) {
				got[i].TimestampUTC = want[i].TimestampUTC
			}
		}
	}
	return want, got
}

// rowForm returns a row with its text in the compared form and each
// formatted field as a whole value; the other fields are kept.
func rowForm(row Row, enabled enabledSet) Row {
	whole := func(text Text) Text { return Text(wholeValueForm(string(text), enabled)) }
	row.Host, row.User, row.SessionID, row.ProjectPath = whole(row.Host), whole(row.User),
		whole(row.SessionID), whole(row.ProjectPath)
	row.GitBranch, row.Model, row.ToolName, row.ToolUseID = whole(row.GitBranch), whole(row.Model),
		whole(row.ToolName), whole(row.ToolUseID)
	row.Text = Text(rowTextForm(string(row.TurnType), string(row.Text), enabled))
	return row
}

// formattedFields are the fields of a timeline row or session, beside its
// text, whose value a parser may have formatted from a value of the
// evidence.
func formattedFields() []string {
	return []string{"host", "user", "session_id", "project_path", "git_branch", "model", "tool_name",
		"tool_use_id"}
}
