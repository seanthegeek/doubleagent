package parser

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// valueKeySpellings are JSON values, many of them one number or one object
// spelled several ways, for checking ValueKey against jsonvalue.Equal.
func valueKeySpellings() []string {
	return []string{
		`0`, `-0`, `0.0`, `-0.000`, `0e5`, `0E-7`,
		`1`, `1.0`, `1.00`, `1e0`, `1E+0`, `10e-1`, `0.1e1`, `100e-2`, `-1`, `-1.0`, `-1e0`,
		`2`, `100`, `1e2`, `1E2`, `10e1`, `1000e-1`, `1.5`, `15e-1`, `1.50`, `0.15e1`,
		`0.1`, `0.10000000000000001`, `1e400`, `10e399`, `1e-400`, `-1e400`, `0.01e402`,
		`123456789012345678901234567890`, `1.23456789012345678901234567890e29`,
		`1e1000000000000000000`, `10e999999999999999999`, `1e999999999999999999`,
		`1e-1000000000000000000`, `0.1e-999999999999999999`, `1e-999999999999999999`,
		`"1"`, `"1.0"`, `""`, `"a"`, `"s1:a"`, `null`, `true`, `false`,
		`[]`, `[1]`, `[1.0]`, `[1,2]`, `[2,1]`, `[[1]]`, `["a"]`, `[null]`,
		`{}`, `{"k":1}`, `{"k":1.0}`, `{"k":"1"}`, `{"a":1,"b":2}`, `{"b":2,"a":1}`, `{"a":{"b":[1e0]}}`,
		`{"a":{"b":[1]}}`, `{"s1:a":1}`, `{"a":1,"b":1}`,
	}
}

func TestValueKeyAgreesWithEqualOnEverySpelling(t *testing.T) {
	values := make([]jsonvalue.Value, len(valueKeySpellings()))
	for i, text := range valueKeySpellings() {
		value, err := jsonvalue.DecodeString(text)
		if err != nil {
			t.Fatalf("%s: %v", text, err)
		}
		values[i] = value
	}
	for i, left := range values {
		for j, right := range values {
			name := valueKeySpellings()[i] + " against " + valueKeySpellings()[j]
			t.Run(name, func(t *testing.T) {
				equal, sameKey := jsonvalue.Equal(left, right), ValueKey(left) == ValueKey(right)
				if equal != sameKey {
					t.Errorf("Equal is %v, the keys %q and %q", equal, ValueKey(left), ValueKey(right))
				}
			})
		}
	}
}

func TestValueKeyGivesEqualNumbersOneKey(t *testing.T) {
	tests := []struct {
		name, left, right string
	}{
		{"an integer and its float form", `1`, `1.0`},
		{"an exponent and trailing zeros", `1e2`, `100`},
		{"zeros of either sign", `0`, `-0.0`},
		{"an exponent past the int64 range and a shift into it", `10e999999999999999999`, `1e1000000000000000000`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left, _ := jsonvalue.NewNumber(test.left)
			right, _ := jsonvalue.NewNumber(test.right)
			if ValueKey(left) != ValueKey(right) {
				t.Errorf("%q and %q", ValueKey(left), ValueKey(right))
			}
		})
	}
}

// FuzzValueKeyAgreesWithEqual holds that two decoded values share a key
// exactly when jsonvalue.Equal says they hold the same information.
func FuzzValueKeyAgreesWithEqual(fuzz *testing.F) {
	for _, left := range valueKeySpellings() {
		fuzz.Add(left, `1`)
	}
	fuzz.Add(`1e2`, `100.00`)
	fuzz.Add(`{"a":[1,{"c":-0}],"b":2}`, `{"b":2.0,"a":[1e0,{"c":0}]}`)
	fuzz.Fuzz(func(t *testing.T, leftText, rightText string) {
		left, leftErr := jsonvalue.DecodeString(leftText)
		right, rightErr := jsonvalue.DecodeString(rightText)
		if leftErr != nil || rightErr != nil || len(leftText)+len(rightText) > 4096 {
			return
		}
		if jsonvalue.Equal(left, right) != (ValueKey(left) == ValueKey(right)) {
			t.Errorf("%s and %s: Equal %v, keys %q and %q", leftText, rightText,
				jsonvalue.Equal(left, right), ValueKey(left), ValueKey(right))
		}
	})
}
