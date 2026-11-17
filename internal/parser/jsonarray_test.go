package parser

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// The positions, problems and ends are what the Python analyzer's
// iter_array_at gives; the decoder's wording is the binary's own.
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
			Message: "unparseable record 2 (invalid JSON at line 1, column 10: expected a value)",
			Raw:     `{"a":`}}, -1},
		{"a missing comma cuts the array off", `[1 2]`, 0, []string{"1"},
			Problems{{Line: 2, Message: "array cut off after record 1"}}, -1},
		{"a trailing comma is a bad record", `[1,]`, 0, []string{"1"}, Problems{{Line: 2,
			Message: "unparseable record 2 (invalid JSON at line 1, column 4: expected a value)",
			Raw:     "]"}}, -1},
		{"a lone bracket", `[`, 0, nil, Problems{{Line: 1,
			Message: "unparseable record 1 (invalid JSON at line 1, column 2: expected a value)"}}, -1},
		{"an array after an offset", `x [ {"a":1} , "é"] tail`, 2, []string{`{"a":1}`, `"é"`}, nil,
			19},
		{"an array without its bracket", "[1, 2\n", 0, []string{"1", "2"},
			Problems{{Line: 3, Message: "array cut off after record 2"}}, -1},
		{"a bad record's raw text loses its line end", "[1, @\r\n", 0, []string{"1"},
			Problems{{Line: 2, Message: "unparseable record 2 (invalid JSON at line 1, column 5: " +
				"expected a value)", Raw: "@"}}, -1},
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
				items = append(items, pyjson.Compact(value))
			}
			if !slices.Equal(items, test.items) || !slices.Equal(problems, test.problems) ||
				end != test.end {
				t.Errorf("got %q, %+v, end %d; want %q, %+v, end %d", items, problems, end,
					test.items, test.problems, test.end)
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
			Problems{{Line: 3, Message: "unparseable data after the JSON array"}}},
		{"a cut-off array reports only the cut", "[1, 2", 2,
			Problems{{Line: 3, Message: "array cut off after record 2"}}},
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
