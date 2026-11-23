package codex

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestSameValueComparesNumbersByValue(t *testing.T) {
	tests := []struct {
		name, left, right string
		want              bool
	}{
		{"an integer and a float of one value", "1", "1.0", true},
		{"true and one", "true", "1", true},
		{"two integers", "2", "3", false},
		{"a number and its text", "1", `"1"`, false},
		{"two strings", `"a"`, `"a"`, true},
		{"a number beyond the float64 range is not zero", "1e999", "0", false},
		{"a number beyond the float64 range is not true", "1e999", "true", false},
		{"a number beyond the float64 range equals itself", "1e999", "10e998", true},
		{"two numbers beyond the range differ", "1e999", "2e999", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left, err := jsonvalue.DecodeString(test.left)
			if err != nil {
				t.Fatal(err)
			}
			right, err := jsonvalue.DecodeString(test.right)
			if err != nil {
				t.Fatal(err)
			}
			if got := sameValue(left, right); got != test.want {
				t.Errorf("sameValue(%s, %s) = %v, want %v", test.left, test.right, got, test.want)
			}
		})
	}
}
