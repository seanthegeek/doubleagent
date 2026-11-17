package parser

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestElementsVisitWhatPythonsLoopVisits(t *testing.T) {
	tests := []struct {
		name  string
		json  string
		items []string
	}{
		{"an array's items", `["a", 1, {"k": 2}]`, []string{`"a"`, `1`, `{"k":2}`}},
		{"an object's keys", `{"b": 1, "a": 2}`, []string{`"b"`, `"a"`}},
		{"a string's characters", `"aé😀"`, []string{`"a"`, `"é"`, `"😀"`}},
		{"null", `null`, nil},
		{"false", `false`, nil},
		{"zero", `0`, nil},
		{"an empty string", `""`, nil},
		{"an empty array", `[]`, nil},
		{"an empty object", `{}`, nil},
		{"a number is not iterable", `5`, nil},
		{"true is not iterable", `true`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := jsonvalue.DecodeString(test.json)
			if err != nil {
				t.Fatal(err)
			}
			got := Elements(value)
			if len(got) != len(test.items) {
				t.Fatalf("got %d items, want %d", len(got), len(test.items))
			}
			for i, item := range got {
				if text := item.JSON(); text != test.items[i] {
					t.Errorf("item %d is %s, want %s", i, text, test.items[i])
				}
			}
		})
	}
}

func TestElementsGiveNullForAFieldOfANonObject(t *testing.T) {
	for _, item := range Elements(jsonvalue.NewString("ab")) {
		if !item.Get("name").IsNull() {
			t.Errorf("a character gave a field %v", item.Get("name"))
		}
	}
}
