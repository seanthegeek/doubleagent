package parser

import "testing"

func TestSkipJSONSpaceStopsAtTheFirstOtherByte(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		offset int
		want   int
	}{
		{"each kind of JSON whitespace", " \t\r\n[", 0, 4},
		{"no whitespace", "[1]", 0, 0},
		{"from an offset", "a  b", 1, 3},
		{"a form feed is not JSON whitespace", "\f[", 0, 0},
		{"a no-break space is not JSON whitespace", "\u00a0[", 0, 0},
		{"whitespace to the end", "a  ", 1, 3},
		{"an offset at the end", "ab", 2, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := SkipJSONSpace(test.text, test.offset); got != test.want {
				t.Errorf("SkipJSONSpace(%q, %d) = %d, want %d", test.text, test.offset, got, test.want)
			}
		})
	}
}
