package textlines

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// The expected values below are those of the Python analyzer's textlines
// module and its tests (analyzer/tests/test_textlines.py).

func TestSplitEndsLinesAtLFOrCRLFOnly(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"text is split at LF", "a\nb", []string{"a", "b"}},
		{"text is split at CRLF", "a\r\nb\r\n", []string{"a", "b"}},
		{"a lone CR stays inside its line", "a\rb\n", []string{"a\rb"}},
		{"a terminating LF gives no empty line", "a\n", []string{"a"}},
		{"empty text gives no line", "", nil},
		{"a lone LF gives one empty line", "\n", []string{""}},
		{"blank lines inside are kept", "a\n\n\nb\n\n", []string{"a", "", "", "b", ""}},
		{"a byte-order mark is kept", "\U0000FEFFa\n", []string{"\U0000FEFFa"}},
		{"a CR alone at the end is removed", "a\r", []string{"a"}},
		{"only one CR before LF is removed", "a\r\r\n", []string{"a\r"}},
		{"invalid UTF-8 passes through", "\xff\xfe\nb", []string{"\xff\xfe", "b"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := Split(test.input); !slices.Equal(got, test.want) {
				t.Errorf("Split(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestSplitIgnoresOtherSeparators(t *testing.T) {
	for _, separator := range []string{"\U00002028", "\U00002029", "\u0085", "\x0b", "\x0c", "\x1c",
		"\x1d", "\x1e"} {
		name := fmt.Sprintf("separator U+%04X does not end a line", []rune(separator)[0])
		t.Run(name, func(t *testing.T) {
			want := []string{"a" + separator + "b"}
			if got := Split("a" + separator + "b\n"); !slices.Equal(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestWithoutTerminatorDropsLFOrCRLFOnly(t *testing.T) {
	cases := []struct {
		name, input, want string
	}{
		{"CRLF is dropped", "a\r\n", "a"},
		{"LF is dropped", "a\n", "a"},
		{"a line without a terminator is unchanged", "a", "a"},
		{"a lone CR is kept", "a\r", "a\r"},
		{"only the last LF is dropped", "a\n\n", "a\n"},
		{"a line separator before LF is kept", " a\U00002028 \n", " a\U00002028 "},
		{"empty text stays empty", "", ""},
		{"a lone LF becomes empty", "\n", ""},
		{"a lone CRLF becomes empty", "\r\n", ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := WithoutTerminator(test.input); got != test.want {
				t.Errorf("WithoutTerminator(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestTrimSetsKeepUnicodeSpaces(t *testing.T) {
	for _, space := range []string{"\U000000A0", "\U00002028", "\U00003000", "\u0085"} {
		t.Run(fmt.Sprintf("space U+%04X is not trimmed", []rune(space)[0]), func(t *testing.T) {
			padded := " " + space + "a" + space + "\t"
			want := space + "a" + space
			if got := strings.Trim(padded, ASCIISpace); got != want {
				t.Errorf("ASCIISpace trim gave %q, want %q", got, want)
			}
			if got := strings.Trim(padded, SpaceOrTab); got != want {
				t.Errorf("SpaceOrTab trim gave %q, want %q", got, want)
			}
		})
	}
}

func TestTrimSetsTakeTheirASCIICharacters(t *testing.T) {
	cases := []struct {
		name, set, input, want string
	}{
		{"ASCIISpace takes space, tab, CR, form feed and vertical tab", ASCIISpace,
			" \t\r\f\va\v\f\r\t ", "a"},
		{"ASCIISpace keeps LF", ASCIISpace, "\na\n", "\na\n"},
		{"ASCIIWhitespace also takes LF", ASCIIWhitespace, "\n \t\r\f\va\n", "a"},
		{"JSONSpace takes space, tab, LF and CR", JSONSpace, " \t\n\ra\r\n", "a"},
		{"JSONSpace keeps form feed", JSONSpace, "\fa", "\fa"},
		{"SpaceOrTab keeps form feed", SpaceOrTab, " \ta\f", "a\f"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := strings.Trim(test.input, test.set); got != test.want {
				t.Errorf("Trim(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
