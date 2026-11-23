package parserkit

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestPresentIsFalseOnlyForNullAndTheEmptyString(t *testing.T) {
	tests := []struct {
		name, json string
		want       bool
	}{
		{"null", "null", false},
		{"the empty string", `""`, false},
		{"a space", `" "`, true},
		{"false", "false", true},
		{"zero", "0", true},
		{"an empty array", "[]", true},
		{"an empty object", "{}", true},
		{"text", `"a"`, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := jsonvalue.DecodeString(test.json)
			if err != nil {
				t.Fatal(err)
			}
			if got := Present(value); got != test.want {
				t.Errorf("Present(%s) = %v, want %v", test.json, got, test.want)
			}
		})
	}
	if Present(jsonvalue.NewObject().Get("absent")) {
		t.Error("an absent key is present")
	}
}
