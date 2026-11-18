package parser

import (
	"slices"
	"testing"
)

func TestObjectsFromSalvagesTheObjectsBeforeDamage(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		offset int
		want   []string
	}{
		{"a whole array", `[{"a":1}, {"b":2}]`, 1, []string{`{"a":1}`, `{"b":2}`}},
		{"other values are passed over", `[1, {"a":1}, "x", null]`, 1, []string{`{"a":1}`}},
		{"a cut object ends the walk", `[{"a":1}, {"b":`, 1, []string{`{"a":1}`}},
		{"a bad value ends the walk", `[{"a":1}, @, {"b":2}]`, 1, []string{`{"a":1}`}},
		{"the bracket ends it", `[{"a":1}] {"b":2}`, 1, []string{`{"a":1}`}},
		{"commas and whitespace run together", "[ ,,\r\n{\"a\":1}", 1, []string{`{"a":1}`}},
		{"nothing after the offset", `[`, 1, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, value := range ObjectsFrom(test.text, test.offset) {
				got = append(got, value.JSON())
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
