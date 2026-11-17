package pytext

import (
	"slices"
	"testing"
)

func TestFieldsSplitsAtPythonsWhitespace(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"empty", "", nil},
		{"only spaces", " \t\n ", nil},
		{"words", "a  b\tc\nd", []string{"a", "b", "c", "d"}},
		{"separators U+001C to U+001F split", "a\x1cb\x1dc\x1ed\x1fe", []string{"a", "b", "c", "d", "e"}},
		{"unicode spaces split", "a\u00a0b\u2003c\u3000d\u0085e", []string{"a", "b", "c", "d", "e"}},
		{"zero width space and BOM stay", "a\u200bb\ufeffc", []string{"a\u200bb\ufeffc"}},
		{"leading and trailing", "  x y  ", []string{"x", "y"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Fields(test.text); !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
