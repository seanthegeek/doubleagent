//go:build parity

package replay

import (
	"slices"
	"testing"
)

func TestRowTextFormTellsAFormattedValueFromProse(t *testing.T) {
	all := setOf(rowFormatNumbers())
	tests := []struct {
		name, turnType, text, want string
	}{
		{"a value after = in a system row", "system", "interactive=True pid=4242", "interactive=true pid=4242"},
		{"a value after ': ' in a tool call", "tool_use", "flag: False", "flag: false"},
		{"None after = is compared as written", "system", "reason=None, x", "reason=None, x"},
		{"a float after = is compared as written", "system", "cost=1.0 more", "cost=1.0 more"},
		{"a word inside another word is left", "system", "mode=Truest", "mode=Truest"},
		{"a user's True is the user's", "user", "x=True", "x=True"},
		{"a tool's output is as written", "tool_result", "ok=True", "ok=True"},
		{"embedded JSON in prose is one encoding", "user", `see {"b": 2, "a": [1.0]}`, `see {"a":[1.0],"b":2}`},
		{"a key written twice is kept twice in its order", "system", `args {"b":0, "a": 2, "a": 1}`,
			`args {"a":2,"a":1,"b":0}`},
		{"a Python list repr is compared as written", "system", `pattern=['git', "it's"]`, `pattern=['git', "it's"]`},
		{"a bracket in prose is left", "assistant", "[persisted output] and {x}", "[persisted output] and {x}"},
		{"a separator is compared as written", "user", "\x1c hello", "\x1c hello"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rowTextForm(test.turnType, test.text, all); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestNeededDifferencesLeavesOutWhatACaseDoesNotNeed(t *testing.T) {
	tests := []struct {
		name  string
		needs []int
		want  []int
	}{
		{"a case that agrees as it is needs none", nil, nil},
		{"a case needing one names it", []int{63}, []int{63}},
		{"a case needing two names both", []int{63, 66}, []int{63, 66}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := neededDifferences([]int{60, 63, 66, 69}, func(enabled enabledSet) bool {
				for _, number := range test.needs {
					if !enabled[number] {
						return false
					}
				}
				return true
			})
			if !slices.Equal(got, test.want) {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}
