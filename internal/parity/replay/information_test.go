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
	differenceJSONValues    = 108
	differenceLoneSurrogate = 109
	differenceCSVRecords    = 110
	differenceTableColumns  = 111
	differencePathAsGiven   = 112
	differenceBoolean       = 113
	differenceNull          = 114
	differenceNumber        = 115
	differenceEmbeddedJSON  = 116
	differenceListRepr      = 117
	differenceSeparators    = 118
	differenceRoundedTime   = 119
)

// The scopes a difference of format names: the texts it applies to.
const (
	scopeOutputs = "the JSON outputs and detect --json"
	scopeAll     = "row texts and the JSON outputs"
	scopeCSV     = "the fleet CSV"
	scopeTables  = "stdout tables"
	scopeStreams = "stdout, stderr and the outputs"
	scopeRows    = "row texts and the formatted fields of a row"
	scopeTimes   = "timestamps in rows, outputs and tables"
)

// formatDifferences returns the differences of format, 108 onward, each with
// the rule of the plan it lets the binary follow.
func formatDifferences() []Difference {
	return []Difference{
		{Number: differenceJSONValues, Summary: "JSON output is written by encoding/json: it is compared " +
			"as decoded values (escaping, spacing, final newline, key order in an object)", Scope: scopeOutputs},
		{Number: differenceLoneSurrogate, Summary: "a lone surrogate becomes U+FFFD as the standard " +
			"library does", Scope: scopeAll},
		{Number: differenceCSVRecords, Summary: "the CSV uses encoding/csv's defaults: it is compared as " +
			"the records encoding/csv reads", Scope: scopeCSV},
		{Number: differenceTableColumns, Summary: "length and padding count runes: a table is compared " +
			"by column, not by its padding", Scope: scopeTables},
		{Number: differencePathAsGiven, Summary: "a path is printed as given", Scope: scopeStreams},
		{Number: differenceBoolean, Summary: "a boolean is true or false", Scope: scopeRows},
		{Number: differenceNull, Summary: "a null is empty", Scope: scopeRows},
		{Number: differenceNumber, Summary: "a number from the evidence keeps its source text: numbers " +
			"are compared by value", Scope: scopeRows},
		{Number: differenceEmbeddedJSON, Summary: "JSON embedded in a row is compact, keys in the " +
			"evidence's order", Scope: scopeRows},
		{Number: differenceListRepr, Summary: "a list or object in a row is JSON, not Python's repr",
			Scope: scopeRows},
		{Number: differenceSeparators, Summary: "whitespace is unicode.IsSpace: U+001C to U+001F are " +
			"not trimmed", Scope: scopeRows},
		{Number: differenceRoundedTime, Summary: "a timestamp's fraction is cut to the millisecond, " +
			"never rounded up into the next one", Scope: scopeTimes},
	}
}

// rowFormatNumbers are the differences of format a parser case's rows are
// compared under.
func rowFormatNumbers() []int {
	return []int{differenceLoneSurrogate, differenceBoolean, differenceNull, differenceNumber,
		differenceEmbeddedJSON, differenceListRepr, differenceSeparators, differenceRoundedTime}
}

// commandFormatNumbers are the differences of format a command case is
// compared under: all of them.
func commandFormatNumbers() []int {
	var numbers []int
	for _, entry := range formatDifferences() {
		numbers = append(numbers, entry.Number)
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
func logNeeded(t *testing.T, label string, tally neededTally, numbers []int) {
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
// field by field, under the differences of format. It reports rows that
// still differ, in their compared form, and returns the differences the
// case needed.
func compareRowInformation(t testing.TB, node string, want, got []Row) []int {
	t.Helper()
	if slices.Equal(want, got) {
		return nil
	}
	numbers := rowFormatNumbers()
	agree := func(enabled enabledSet) bool {
		formWant, formGot := informationRows(want, got, enabled)
		return slices.Equal(formWant, formGot)
	}
	if !agree(setOf(numbers)) {
		formWant, formGot := informationRows(want, got, setOf(numbers))
		DiffRows(t, node, formWant, formGot)
		return nil
	}
	return neededDifferences(numbers, agree)
}

// informationRows returns the compared form of each pair of rows at one
// index that differ; equal rows, and rows past the shorter list, are kept.
func informationRows(want, got []Row, enabled enabledSet) ([]Row, []Row) {
	want, got = slices.Clone(want), slices.Clone(got)
	for i := range min(len(want), len(got)) {
		if want[i] != got[i] {
			want[i], got[i] = rowForm(want[i], enabled), rowForm(got[i], enabled)
			if enabled[differenceRoundedTime] {
				got[i].TimestampUTC = Text(rolledOver(string(want[i].TimestampUTC), string(got[i].TimestampUTC)))
				got[i].Text = Text(rolledOver(string(want[i].Text), string(got[i].Text)))
			}
		}
	}
	return want, got
}

// rowForm returns a row with its text in the compared form and each
// formatted field as a whole value; the other fields lose only a lone
// surrogate.
func rowForm(row Row, enabled enabledSet) Row {
	whole := func(text Text) Text { return Text(wholeValueForm(string(text), enabled)) }
	plain := func(text Text) Text { return Text(surrogateForm(string(text), enabled)) }
	return Row{TimestampUTC: plain(row.TimestampUTC), Host: whole(row.Host), User: whole(row.User),
		Agent: plain(row.Agent), SessionID: whole(row.SessionID), ProjectPath: whole(row.ProjectPath),
		GitBranch: whole(row.GitBranch), TurnType: plain(row.TurnType), Model: whole(row.Model),
		ToolName: whole(row.ToolName), ToolUseID: whole(row.ToolUseID),
		Text:       Text(rowTextForm(string(row.TurnType), string(row.Text), enabled)),
		SourceFile: plain(row.SourceFile), SourceLine: row.SourceLine, Recovered: plain(row.Recovered)}
}

// formattedFields are the fields of a timeline row or session, beside its
// text, whose value a parser may have formatted from a value of the
// evidence.
func formattedFields() []string {
	return []string{"host", "user", "session_id", "project_path", "git_branch", "model", "tool_name",
		"tool_use_id"}
}
