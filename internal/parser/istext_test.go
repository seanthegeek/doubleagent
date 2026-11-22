package parser

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestIsTextMatchesOnlyAStringOfTheText(t *testing.T) {
	tests := []struct {
		name, json, text string
		want             bool
	}{
		{"the same string", `"user"`, "user", true},
		{"another string", `"User"`, "user", false},
		{"the empty string", `""`, "", true},
		{"null is not the empty string", "null", "", false},
		{"a number is not its text", "1", "1", false},
		{"true is not its text", "true", "true", false},
		{"an array holding the string", `["user"]`, "user", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := jsonvalue.DecodeString(test.json)
			if err != nil {
				t.Fatal(err)
			}
			if got := IsText(value, test.text); got != test.want {
				t.Errorf("IsText(%s, %q) = %v, want %v", test.json, test.text, got, test.want)
			}
		})
	}
}
