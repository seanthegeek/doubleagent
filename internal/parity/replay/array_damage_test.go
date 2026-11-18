//go:build parity

package replay

import (
	"regexp"
	"slices"
	"strconv"
	"testing"
)

// The parser rows of a JSON array that ends where an element is due, as
// each side words them: "array cut off after record <n>", n the elements
// read whole, which both write, or, recorded only, "unparseable record
// <n+1>" with Python's reason for text that ends where a value is due and
// no raw text, since nothing follows. Difference 160 applies before 102,
// so it sees the recorded reason as written.
var (
	recordedCutOff = regexp.MustCompile(`^parser: (?:array cut off after record ([0-9]+)|` +
		`unparseable record ([1-9][0-9]*) \(Expecting value: line [0-9]+ column [0-9]+ \(char [0-9]+\)\))$`)
	binaryCutOff = regexp.MustCompile(`^parser: array cut off after record ([0-9]+)()$`)
)

// The parser rows of data after a JSON array's closing bracket: either of
// two wordings as recorded, the second as the binary writes it.
var (
	recordedDataAfter = regexp.MustCompile(`^parser: (?:unparseable data after the JSON array|` +
		`unexpected data after the array)$`)
	binaryDataAfter = regexp.MustCompile(`^parser: unexpected data after the array$`)
)

// arrayDamageText returns the rewrite of difference 160 for one side: a
// row of a cut-off array or of data after it, in that side's wording, as
// one token per condition keeping the count of elements read whole, with
// the count of rows written.
func arrayDamageText(from side) func(text string) (string, int) {
	cutOff, dataAfter := recordedCutOff, recordedDataAfter
	if from == binarySide {
		cutOff, dataAfter = binaryCutOff, binaryDataAfter
	}
	return func(text string) (string, int) {
		if dataAfter.MatchString(text) {
			return "parser: {data after the array}", 1
		}
		match := cutOff.FindStringSubmatch(text)
		if match == nil {
			return text, 0
		}
		count := match[1]
		if count == "" {
			record, err := strconv.Atoi(match[2])
			if err != nil {
				return text, 0
			}
			count = strconv.Itoa(record - 1)
		}
		return "parser: {array cut off after record " + count + "}", 1
	}
}

func TestArrayDamageHasOneTokenPerCondition(t *testing.T) {
	tests := []struct {
		name  string
		side  side
		text  string
		want  string
		found int
	}{
		{"a recorded cut keeps its count", recordedSide, "parser: array cut off after record 1",
			"parser: {array cut off after record 1}", 1},
		{"the binary's cut keeps its count", binarySide, "parser: array cut off after record 1",
			"parser: {array cut off after record 1}", 1},
		{"a missing record at the end is a cut after the record before", recordedSide,
			"parser: unparseable record 3 (Expecting value: line 1 column 660 (char 659))",
			"parser: {array cut off after record 2}", 1},
		{"a missing first record is a cut after none", recordedSide,
			"parser: unparseable record 1 (Expecting value: line 1 column 2 (char 1))",
			"parser: {array cut off after record 0}", 1},
		{"the binary never words a cut as a bad record", binarySide,
			"parser: unparseable record 3 (Expecting value: line 1 column 660 (char 659))",
			"parser: unparseable record 3 (Expecting value: line 1 column 660 (char 659))", 0},
		{"a bad record after another difference's rewrite is not a cut", recordedSide,
			"parser: unparseable record 3 ({json error})",
			"parser: unparseable record 3 ({json error})", 0},
		{"the recorded data after the array", recordedSide, "parser: unparseable data after the JSON array",
			"parser: {data after the array}", 1},
		{"the other recorded data after the array", recordedSide, "parser: unexpected data after the array",
			"parser: {data after the array}", 1},
		{"the binary's data after the array", binarySide, "parser: unexpected data after the array",
			"parser: {data after the array}", 1},
		{"the binary never writes the first recorded wording", binarySide,
			"parser: unparseable data after the JSON array", "parser: unparseable data after the JSON array", 0},
		{"a bad record with its text is not a cut", recordedSide,
			"parser: unparseable record 3 (Expecting value: line 1 column 2 (char 1)): {,",
			"parser: unparseable record 3 (Expecting value: line 1 column 2 (char 1)): {,", 0},
		{"a bad record for another reason is not a cut", recordedSide,
			"parser: unparseable record 3 (Unterminated string starting at: line 1 column 2 (char 1))",
			"parser: unparseable record 3 (Unterminated string starting at: line 1 column 2 (char 1))", 0},
		{"another parser's reason is not a cut", recordedSide, "parser: unparseable record 3 (not a JSON object)",
			"parser: unparseable record 3 (not a JSON object)", 0},
		{"the words inside another row are left alone", recordedSide,
			"user said parser: unexpected data after the array",
			"user said parser: unexpected data after the array", 0},
		{"a cut quoted inside a longer row is left alone", binarySide, "parser: array cut off after record 1: x",
			"parser: array cut off after record 1: x", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, found := rewriteOf(t, 160, test.side)(test.text)
			if got != test.want || found != test.found {
				t.Errorf("got %q (%d), want %q (%d)", got, found, test.want, test.found)
			}
		})
	}
}

// A changed record count, the other condition or another row still differs
// once both sides are written.
func TestArrayDamageStillFailsAChangedRow(t *testing.T) {
	tests := []struct {
		name string
		want string
		got  string
	}{
		{"a different count", "parser: unparseable record 3 (Expecting value: line 1 column 9 (char 8))",
			"parser: array cut off after record 3"},
		{"a cut for data after", "parser: unparseable data after the JSON array",
			"parser: array cut off after record 1"},
		{"data after for a cut", "parser: array cut off after record 1",
			"parser: unexpected data after the array"},
		{"another row's text", "parser: unexpected data after the array", "parser: not a JSON array"},
		{"a bad record for a cut", "parser: array cut off after record 1",
			"parser: unparseable record 2 (invalid JSON at line 1, column 2: invalid character 'x' looking for " +
				"beginning of value): x"},
		{"a bad record with no text for a cut", "parser: array cut off after record 2",
			"parser: unparseable record 3 (invalid JSON at line 1, column 2: invalid character '}' looking for " +
				"beginning of value)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want, _ := NormaliseRows(&recordingTB{TB: t}, "n", []int{102, 160}, []Row{{Text: Text(test.want)}}, nil)
			_, got := NormaliseRows(&recordingTB{TB: t}, "n", []int{102, 160}, nil, []Row{{Text: Text(test.got)}})
			if want[0] == got[0] {
				t.Errorf("%q and %q both became %q", test.want, test.got, want[0].Text)
			}
		})
	}
}

func TestArrayDamageAppliesBeforeTheLibraryWording(t *testing.T) {
	var order []int
	for _, entry := range applyingOrder(Differences()) {
		if entry.Number == 102 || entry.Number == 130 || entry.Number == 160 {
			order = append(order, entry.Number)
		}
	}
	if !slices.Equal(order, []int{130, 160, 102}) {
		t.Errorf("applying order %v, want 130 and 160 before 102", order)
	}
}
