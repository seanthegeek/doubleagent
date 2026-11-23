package parserkit

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestTextOrGivesTheFallbackForAnEmptyValue(t *testing.T) {
	tests := []struct{ name, json, want string }{
		{"text", `"a"`, "a"},
		{"a number as written", "1.50", "1.50"},
		{"true", "true", "true"},
		{"null", "null", "-"},
		{"the empty string", `""`, "-"},
		{"false", "false", "-"},
		{"zero", "0", "-"},
		{"an empty list", "[]", "-"},
		{"a list", "[1]", "[1]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := jsonvalue.DecodeString(test.json)
			if err != nil {
				t.Fatal(err)
			}
			if got := TextOr(value, "-"); got != test.want {
				t.Errorf("TextOr(%s, \"-\") = %q, want %q", test.json, got, test.want)
			}
		})
	}
}
