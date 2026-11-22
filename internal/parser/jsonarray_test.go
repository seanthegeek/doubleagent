package parser

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// ArrayAt yields each element before damage with its position, then the
// damage as a problem.
func TestArrayAtSalvagesTheElementsBeforeDamage(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		offset   int
		items    []string
		problems Problems
		end      int
	}{
		{"a whole array", `[1, 2]`, 0, []string{"1", "2"}, nil, 6},
		{"an empty array after whitespace", `  [ ]`, 0, nil, nil, 5},
		{"an object is not an array", `{}`, 0, nil, Problems{{Line: 1, Message: "not a JSON array"}}, -1},
		{"empty text is not an array", ``, 0, nil, Problems{{Line: 1, Message: "not a JSON array"}}, -1},
		{"a record cut off keeps the ones before", `[1, {"a":`, 0, []string{"1"}, Problems{{Line: 2,
			Message: "unparseable record 2 (invalid JSON at line 1, column 10: unexpected end of JSON input)",
			Raw:     `{"a":`}}, -1},
		{"a missing comma cuts the array off", `[1 2]`, 0, []string{"1"},
			Problems{{Line: 2, Message: "array cut off after record 1"}}, -1},
		{"a trailing comma is a bad record", `[1,]`, 0, []string{"1"}, Problems{{Line: 2,
			Message: "unparseable record 2 (invalid JSON at line 1, column 4: invalid character ']' looking for beginning of value)",
			Raw:     "]"}}, -1},
		{"a lone bracket is cut off after no record", `[`, 0, nil,
			Problems{{Line: 1, Message: "array cut off after record 0"}}, -1},
		{"text that ends after a comma is cut off after the record before", "[1, \n", 0, []string{"1"},
			Problems{{Line: 2, Message: "array cut off after record 1"}}, -1},
		{"an array after an offset", `x [ {"a":1} , "é"] tail`, 2, []string{`{"a":1}`, `"é"`}, nil,
			19},
		{"an array without its bracket", "[1, 2\n", 0, []string{"1", "2"},
			Problems{{Line: 3, Message: "array cut off after record 2"}}, -1},
		{"a bad record's raw text loses its line end", "[1, @\r\n", 0, []string{"1"},
			Problems{{Line: 2, Message: "unparseable record 2 (invalid JSON at line 1, column 5: " +
				"invalid character '@' looking for beginning of value)", Raw: "@"}}, -1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems Problems
			var items []string
			end := 0
			position := 0
			for index, value := range ArrayAt(test.text, test.offset, &problems, &end) {
				position++
				if index != position {
					t.Errorf("position %d, want %d", index, position)
				}
				items = append(items, value.JSON())
			}
			if !slices.Equal(items, test.items) || !slices.Equal(problems, test.problems) ||
				end != test.end {
				t.Errorf("got %q, %+v, end %d; want %q, %+v, end %d", items, problems, end,
					test.items, test.problems, test.end)
			}
		})
	}
}

func TestArrayWalkShowsABadRecordOnlyUpToItsShownEnd(t *testing.T) {
	const text = `[1, {"a": ] next part`
	reason := "unparseable record 2 (invalid JSON at line 1, column 11: invalid character ']' after object key:value pair)"
	tests := []struct {
		name     string
		shownEnd int
		raw      string
	}{
		{"the end of the text shows the rest", len(text), `{"a": ] next part`},
		{"an end inside the record shows its start", 9, `{"a":`},
		{"an end at the record's start shows nothing", 4, ""},
		{"an end before the record shows nothing", 1, ""},
		{"an end before the text shows nothing", -3, ""},
		{"an end past the text shows the rest", len(text) + 5, `{"a": ] next part`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems Problems
			var items []string
			given := -1
			walk := &ArrayWalk{Document: jsonvalue.NewDocument(text), ShownEnd: func(stopped int) int {
				given = stopped
				return test.shownEnd
			}}
			for _, value := range walk.Array(0, &problems, nil) {
				items = append(items, value.JSON())
			}
			want := Problems{{Line: 2, Message: reason, Raw: test.raw}}
			if !slices.Equal(items, []string{"1"}) || !slices.Equal(problems, want) || given != 10 || walk.Stopped != 10 {
				t.Errorf("got %q, %+v, stopped %d (given %d); want [1], %+v, stopped 10", items, problems,
					walk.Stopped, given, want)
			}
		})
	}
}

func TestArrayWalkStopsWhereItStoppedReading(t *testing.T) {
	tests := []struct {
		name, text string
		offset     int
		stopped    int
	}{
		{"past the closing bracket", `x [1, 2] tail`, 1, 8},
		{"past an empty array", `[ ] tail`, 0, 3},
		{"at the end of a cut text", `[1, `, 0, 4},
		{"where a comma is missing", `[1 2]`, 0, 3},
		{"at the start of text that is not an array", `  {}`, 0, 2},
		{"at the error of an element that does not decode", `[{"a": "b" "c"}]`, 0, 11},
		{"at the end of the text inside an element", `[{"a": [1, 2`, 0, 12},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems Problems
			walk := &ArrayWalk{Document: jsonvalue.NewDocument(test.text)}
			for range walk.Array(test.offset, &problems, nil) {
			}
			if walk.Stopped != test.stopped {
				t.Errorf("stopped at %d, want %d", walk.Stopped, test.stopped)
			}
		})
	}
}

func TestJSONArrayReportsDataAfterTheArray(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		count    int
		problems Problems
	}{
		{"whitespace after the array is fine", "\ufeff[1, 2] \r\n", 2, nil},
		{"text after the array is reported after its records", "[1, 2] x", 2,
			Problems{{Line: 3, Message: "unexpected data after the array"}}},
		{"a cut-off array reports only the cut", "[1, 2", 2,
			Problems{{Line: 3, Message: "array cut off after record 2"}}},
		{"text after an empty array is reported", "[] x", 0,
			Problems{{Line: 1, Message: "unexpected data after the array"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeFile(t, "array.json", []byte(test.data))
			var problems Problems
			count := 0
			for range JSONArray(path, &problems) {
				count++
			}
			if count != test.count || !slices.Equal(problems, test.problems) {
				t.Errorf("got %d records and %+v, want %d and %+v", count, problems, test.count,
					test.problems)
			}
		})
	}
}

func FuzzArrayAt(fuzz *testing.F) {
	for _, seed := range []string{`[1, 2]`, `[1, {"a":`, `[1 2]`, ` [ ] x`, `["\ud83d", 1e400]`} {
		fuzz.Add(seed, 0)
	}
	fuzz.Fuzz(func(t *testing.T, text string, offset int) {
		if offset < 0 || offset > len(text) {
			return
		}
		var problems Problems
		end := 0
		for range ArrayAt(text, offset, &problems, &end) {
		}
		if end > len(text) || (end < 0 && len(problems) == 0) {
			t.Errorf("end %d of %d with %d problems", end, len(text), len(problems))
		}
	})
}

// ArrayElements does the same for text already in memory.
func TestArrayElementsSalvagesTheElementsBeforeDamage(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		items    []string
		problems Problems
	}{
		{"elements with spaces around commas", `[1, {"a": 2} ,3]`, []string{"1", `{"a":2}`, "3"}, nil},
		{"an empty array", `[]`, nil, nil},
		{"an empty array with spaces", " [ ]\n", nil, nil},
		{"data after an empty array", `[] x`, nil,
			Problems{{Line: 1, Message: "unexpected data after the array"}}},
		{"empty text is not an array", ``, nil, Problems{{Line: 1, Message: "not a JSON array"}}},
		{"whitespace is not an array", "  ", nil, Problems{{Line: 1, Message: "not a JSON array"}}},
		{"a word is not an array", `x`, nil, Problems{{Line: 1, Message: "not a JSON array"}}},
		{"data after the array", `[1] x`, []string{"1"},
			Problems{{Line: 2, Message: "unexpected data after the array"}}},
		{"a second closing bracket", `[1]]`, []string{"1"},
			Problems{{Line: 2, Message: "unexpected data after the array"}}},
		{"a line end after the array is fine", "[1]\n", []string{"1"}, nil},
		{"a comma at the end of text cuts the array off", `[1,`, []string{"1"},
			Problems{{Line: 2, Message: "array cut off after record 1"}}},
		{"a comma and a space at the end of text", `[1, `, []string{"1"},
			Problems{{Line: 2, Message: "array cut off after record 1"}}},
		{"an opening bracket alone", "[\n", nil,
			Problems{{Line: 1, Message: "array cut off after record 0"}}},
		{"a trailing comma before the bracket is a bad record", `[1,]`, []string{"1"}, Problems{{Line: 2,
			Message: "unparseable record 2 (invalid JSON at line 1, column 4: invalid character ']' looking for beginning of value)",
			Raw:     "]"}}},
		{"a missing comma cuts the array off", `[1 2]`, []string{"1"},
			Problems{{Line: 2, Message: "array cut off after record 1"}}},
		{"no closing bracket", `[1`, []string{"1"},
			Problems{{Line: 2, Message: "array cut off after record 1"}}},
		{"a record cut off keeps the ones before", `[1, {"a"`, []string{"1"}, Problems{{Line: 2,
			Message: "unparseable record 2 (invalid JSON at line 1, column 9: unexpected end of JSON input)",
			Raw:     `{"a"`}}},
		{"a bad third record", `[1,"é", tru]`, []string{"1", `"é"`}, Problems{{Line: 3,
			Message: "unparseable record 3 (invalid JSON at line 1, column 12: invalid character ']' in literal true)",
			Raw:     "tru]"}}},
		{"a comma first", `[,1]`, nil, Problems{{Line: 1,
			Message: "unparseable record 1 (invalid JSON at line 1, column 2: invalid character ',' looking for beginning of value)",
			Raw:     ",1]"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems Problems
			var items []string
			for _, value := range ArrayElements(test.text, &problems) {
				items = append(items, value.JSON())
			}
			if !slices.Equal(items, test.items) || !slices.Equal(problems, test.problems) {
				t.Errorf("got %q, %+v; want %q, %+v", items, problems, test.items, test.problems)
			}
		})
	}
}

func FuzzArrayElements(fuzz *testing.F) {
	for _, seed := range []string{`[1, 2]`, `[1, {"a":`, `[1,`, ` [ ] x`, `["\ud83d", 1e400]`} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		var problems Problems
		elements := ArrayElements(text, &problems)
		if len(problems) > 1 || (len(problems) == 1 && problems[0].Line > len(elements)+1) {
			t.Errorf("%d elements with problems %+v", len(elements), problems)
		}
	})
}

// The three readers describe each condition of a damaged array in the same
// words at the same position.
func TestTheArrayReadersDescribeDamageAlike(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		problems Problems
	}{
		{"cut after a comma", "[1, ", Problems{{Line: 2, Message: "array cut off after record 1"}}},
		{"cut after a record", "[1, 2", Problems{{Line: 3, Message: "array cut off after record 2"}}},
		{"cut after the bracket", "[", Problems{{Line: 1, Message: "array cut off after record 0"}}},
		{"cut inside a record", `[1, {"a":`, Problems{{Line: 2,
			Message: "unparseable record 2 (invalid JSON at line 1, column 10: unexpected end of JSON input)",
			Raw:     `{"a":`}}},
		{"data after the array", "[1] {}", Problems{{Line: 2, Message: "unexpected data after the array"}}},
		{"data after an empty array", "[]]", Problems{{Line: 1, Message: "unexpected data after the array"}}},
		{"a bad record", "[1, @]", Problems{{Line: 2, Message: "unparseable record 2 (invalid JSON at " +
			"line 1, column 5: invalid character '@' looking for beginning of value)", Raw: "@]"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var fromText, fromElements, fromFile Problems
			for range ArrayText(test.text, &fromText) {
			}
			ArrayElements(test.text, &fromElements)
			for range JSONArray(writeFile(t, "array.json", []byte(test.text)), &fromFile) {
			}
			for name, got := range map[string]Problems{"ArrayText": fromText,
				"ArrayElements": fromElements, "JSONArray": fromFile} {
				if !slices.Equal(got, test.problems) {
					t.Errorf("%s gave %+v, want %+v", name, got, test.problems)
				}
			}
		})
	}
}
