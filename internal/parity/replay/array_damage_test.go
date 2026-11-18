//go:build parity

package replay

import (
	"regexp"
	"strconv"
	"testing"
)

// arrayCutOff is a parser row of a JSON array that ends where an element
// is due: "array cut off after record <n>", n the elements read whole, or
// "unparseable record <n+1>" with the JSON decoder's reason for text that
// ends where a value is due (as recorded, or as difference 102 leaves it)
// and no raw text, since nothing follows.
var arrayCutOff = regexp.MustCompile(`^parser: (?:array cut off after record ([0-9]+)|` +
	`unparseable record ([1-9][0-9]*) \((?:Expecting value: line [0-9]+ column [0-9]+ ` +
	`\(char [0-9]+\)|\{json error\})\))$`)

// arrayDataAfter is a parser row of data after a JSON array's closing
// bracket, in either of the two recorded wordings.
var arrayDataAfter = regexp.MustCompile(`^parser: (?:unparseable data after the JSON array|` +
	`unexpected data after the array)$`)

// arrayDamageText writes a row of arrayCutOff or arrayDataAfter as one
// token per condition, keeping the count of elements read whole, and
// returns the count of rows written.
func arrayDamageText(text string) (string, int) {
	if arrayDataAfter.MatchString(text) {
		return "parser: {data after the array}", 1
	}
	match := arrayCutOff.FindStringSubmatch(text)
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

func TestArrayDamageHasOneTokenPerCondition(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		want  string
		found int
	}{
		{"a recorded cut keeps its count", "parser: array cut off after record 1",
			"parser: {array cut off after record 1}", 1},
		{"a missing record at the end is a cut after the record before",
			"parser: unparseable record 3 (Expecting value: line 1 column 660 (char 659))",
			"parser: {array cut off after record 2}", 1},
		{"the same row after difference 102 is the same cut", "parser: unparseable record 3 ({json error})",
			"parser: {array cut off after record 2}", 1},
		{"a missing first record is a cut after none", "parser: unparseable record 1 ({json error})",
			"parser: {array cut off after record 0}", 1},
		{"the recorded data after the array", "parser: unparseable data after the JSON array",
			"parser: {data after the array}", 1},
		{"the other recorded data after the array", "parser: unexpected data after the array",
			"parser: {data after the array}", 1},
		{"a bad record with its text is not a cut",
			"parser: unparseable record 3 (Expecting value: line 1 column 2 (char 1)): {,",
			"parser: unparseable record 3 (Expecting value: line 1 column 2 (char 1)): {,", 0},
		{"a bad record for another reason is not a cut",
			"parser: unparseable record 3 (Unterminated string starting at: line 1 column 2 (char 1))",
			"parser: unparseable record 3 (Unterminated string starting at: line 1 column 2 (char 1))", 0},
		{"another parser's reason is not a cut", "parser: unparseable record 3 (not a JSON object)",
			"parser: unparseable record 3 (not a JSON object)", 0},
		{"the words inside another row are left alone",
			"user said parser: unexpected data after the array",
			"user said parser: unexpected data after the array", 0},
		{"a cut quoted inside a longer row is left alone", "parser: array cut off after record 1: x",
			"parser: array cut off after record 1: x", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, found := difference(t, 160).Text(test.text)
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
			"parser: unparseable record 2 ({json error}): x"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want, _ := arrayDamageText(test.want)
			got, _ := arrayDamageText(test.got)
			if want == got {
				t.Errorf("%q and %q both became %q", test.want, test.got, want)
			}
		})
	}
}
