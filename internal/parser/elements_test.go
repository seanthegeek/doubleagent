package parser

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestElementsGiveAnArraysItemsOnly(t *testing.T) {
	tests := []struct {
		name  string
		json  string
		items []string
	}{
		{"an array's items", `["a", 1, {"k": 2}]`, []string{`"a"`, `1`, `{"k":2}`}},
		{"an object gives nothing", `{"b": 1, "a": 2}`, nil},
		{"a string gives nothing", `"ab"`, nil},
		{"null", `null`, nil},
		{"an empty array", `[]`, nil},
		{"a number", `5`, nil},
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
	for _, item := range Elements(jsonvalue.NewArray(jsonvalue.NewString("ab"), jsonvalue.NewInt(1))) {
		if !item.Get("name").IsNull() {
			t.Errorf("an item gave a field %v", item.Get("name"))
		}
	}
}
