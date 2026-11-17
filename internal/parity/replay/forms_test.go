//go:build parity

package replay

import (
	"slices"
	"testing"
)

func TestPathlibFormIsPythonsPrintedPath(t *testing.T) {
	// Each wanted form is what str(PurePosixPath(given)) printed.
	tests := map[string]string{"col/": "col", "./col": "col", "a//b": "a/b", "//a/b": "//a/b",
		"///a": "/a", "a/./b/": "a/b", ".": ".", "./": ".", "a/../b": "a/../b", "": "."}
	for given, want := range tests {
		if got := pathlibForm(given); got != want {
			t.Errorf("pathlibForm(%q) = %q, want %q", given, got, want)
		}
	}
}

func TestCanonicalNumberSpellsEachValueOneWay(t *testing.T) {
	tests := []struct{ text, want string }{
		{"1", "1"}, {"1.0", "1"}, {"1.50", "1.5"}, {"1e+16", "10000000000000000"},
		{"1E16", "10000000000000000"}, {"10000000000000000", "10000000000000000"},
		{"12345678901234567890123", "12345678901234567890123"}, {"0.0041", "0.0041"},
		{"-0.0", "0"}, {"1e400", "1e400"}, {"007", "7"},
	}
	for _, test := range tests {
		if got := canonicalNumber(test.text); got != test.want {
			t.Errorf("canonicalNumber(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}

func TestRowTextFormTellsAFormattedValueFromProse(t *testing.T) {
	all := setOf(rowFormatNumbers())
	tests := []struct {
		name, turnType, text, want string
	}{
		{"a value after = in a system row", "system", "interactive=True pid=4242", "interactive=true pid=4242"},
		{"a value after ': ' in a tool call", "tool_use", "flag: False", "flag: false"},
		{"None after = is empty", "system", "reason=None, x", "reason=, x"},
		{"a float after = by value", "system", "cost=1.0 more", "cost=1 more"},
		{"a word inside another word is left", "system", "mode=Truest", "mode=Truest"},
		{"a version is not a number", "system", "os_version=3.40.0", "os_version=3.40.0"},
		{"a user's True is the user's", "user", "x=True", "x=True"},
		{"an assistant's None is the assistant's", "assistant", "result: None", "result: None"},
		{"a tool's output is as written", "tool_result", "ok=True", "ok=True"},
		{"embedded JSON in prose is one encoding", "user", `see {"b": 2, "a": [1.0]}`, `see {"a":[1],"b":2}`},
		{"a Python list repr is JSON", "system", `pattern=['git', "it's"]`, `pattern=["git","it's"]`},
		{"a bracket in prose is left", "assistant", "[persisted output] and {x}", "[persisted output] and {x}"},
		{"separators are trimmed with the space around them", "user", "\x1c hello \x1f\nnext\x1e",
			"hello\nnext"},
		{"a separator inside a text stays", "user", "a\x1cb", "a\x1cb"},
		{"a lone surrogate is U+FFFD", "user", "a\xed\xa0\x80b", "a�b"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rowTextForm(test.turnType, test.text, all); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestSurrogateEscapesAreReadOnlyUnder109(t *testing.T) {
	text := `{"a":"x\ud800y","b":"😀","c":"\\ud800"}`
	tests := []struct {
		name    string
		enabled enabledSet
		want    string
	}{
		{"under 109 a lone surrogate is U+FFFD", setOf([]int{differenceLoneSurrogate}),
			`{"a":"x�y","b":"😀","c":"\\ud800"}`},
		{"without 109 it keeps its own text", enabledSet{},
			`{"a":"x\\ud800y","b":"😀","c":"\\ud800"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := (outputForms{enabled: test.enabled}).surrogates(text); got != test.want {
				t.Errorf("got %s, want %s", got, test.want)
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
