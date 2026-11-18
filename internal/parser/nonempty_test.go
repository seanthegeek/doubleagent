package parser

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestJoinNonEmptyLeavesOutEmptyParts(t *testing.T) {
	tests := []struct {
		name, separator string
		parts           []string
		want            string
	}{
		{"every part is kept", " | ", []string{"a", "b", "c"}, "a | b | c"},
		{"an empty part leaves no separator", " | ", []string{"a", "", "c"}, "a | c"},
		{"leading and trailing empty parts vanish", " ", []string{"", "a", ""}, "a"},
		{"only empty parts give nothing", "\n", []string{"", ""}, ""},
		{"no parts give nothing", ",", nil, ""},
		{"a part of spaces is not empty", ",", []string{" ", "a"}, " ,a"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := JoinNonEmpty(test.separator, test.parts...); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestFirstNonEmptySkipsEveryEmptyValue(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"the first value wins when it is set", []string{`"a"`, `"b"`}, `"a"`},
		{"null, false, zero and empty values are skipped",
			[]string{`null`, `false`, `0`, `0.0`, `""`, `[]`, `{}`, `"x"`}, `"x"`},
		{"a number keeps its text", []string{`0`, `1.50`}, `1.50`},
		{"all empty values give null", []string{`""`, `0`}, `null`},
		{"no values give null", nil, `null`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values := make([]jsonvalue.Value, 0, len(test.values))
			for _, text := range test.values {
				values = append(values, decoded(t, text))
			}
			if got := FirstNonEmpty(values...).JSON(); got != test.want {
				t.Errorf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestStemDropsTheLastExtension(t *testing.T) {
	tests := []struct{ name, want string }{
		{"a.json", "a"},
		{"a.b.json", "a.b"},
		{"x.", "x"},
		{"noext", "noext"},
		{".json", ".json"},
		{"..json", "..json"},
		{".env.local", ".env"},
		{"...", "..."},
		{"", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Stem(test.name); got != test.want {
				t.Errorf("Stem(%q) = %q, want %q", test.name, got, test.want)
			}
		})
	}
}
