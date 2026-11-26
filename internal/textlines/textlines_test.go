package textlines

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// The expected values below are the line and trim rules the package
// comment states.

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

func TestValidUTF8ReplacesEachInvalidByte(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{"valid text is kept", "café 😀", "café 😀"},
		{"a cut sequence gives one replacement per byte", "a\xe2\x82b", "a��b"},
		{"an encoded surrogate gives three", "\xed\xa0\x80", "���"},
		{"a stray byte gives one", "x\xffy", "x�y"},
		{"the replacement character itself is kept", "�", "�"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ValidUTF8([]byte(test.data)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestLowerASCIILowersOnlyAToZ(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"empty", "", ""},
		{"capitals are lowered", "Users/ABC-Z", "users/abc-z"},
		{"lower case and digits are kept", "abc019_@[`{", "abc019_@[`{"},
		{"an invalid byte is kept", "A\xffB\xc3", "a\xffb\xc3"},
		{"the Kelvin sign is kept", "KK", "kK"},
		{"the long s is kept", "Sſ", "sſ"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := LowerASCII(test.text); got != test.want {
				t.Errorf("LowerASCII(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

func TestLowerASCIIAllocatesNothingWhenNothingChanges(t *testing.T) {
	text := strings.Repeat("already lower \xffK", 4)
	allocations := testing.AllocsPerRun(100, func() { _ = LowerASCII(text) })
	if allocations != 0 {
		t.Errorf("got %v allocations, want 0", allocations)
	}
}

func TestLowerByteLowersOnlyAToZ(t *testing.T) {
	for value := range 256 {
		char := byte(value)
		want := char
		if 'A' <= char && char <= 'Z' {
			want = char + 'a' - 'A'
		}
		if got := LowerByte(char); got != want {
			t.Errorf("LowerByte(%#x) = %#x, want %#x", char, got, want)
		}
	}
}

func TestUpperASCIIRaisesOnlyAToZ(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"empty", "", ""},
		{"small letters are raised", "Users/abc-z", "USERS/ABC-Z"},
		{"capitals and digits are kept", "ABC019_@[`{", "ABC019_@[`{"},
		{"an invalid byte is kept", "a\xffb\xc3", "A\xffB\xc3"},
		{"the dotless i is kept", "iı", "Iı"},
		{"the long s is kept", "sſ", "Sſ"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := UpperASCII(test.text); got != test.want {
				t.Errorf("UpperASCII(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

func TestUpperASCIIAllocatesNothingWhenNothingChanges(t *testing.T) {
	text := strings.Repeat("ALREADY UPPER \xffı", 4)
	allocations := testing.AllocsPerRun(100, func() { _ = UpperASCII(text) })
	if allocations != 0 {
		t.Errorf("got %v allocations, want 0", allocations)
	}
}

func TestUpperByteRaisesOnlyAToZ(t *testing.T) {
	for value := range 256 {
		char := byte(value)
		want := char
		if 'a' <= char && char <= 'z' {
			want = char - 'a' + 'A'
		}
		if got := UpperByte(char); got != want {
			t.Errorf("UpperByte(%#x) = %#x, want %#x", char, got, want)
		}
	}
}

func TestEqualFoldASCIIIgnoresOnlyASCIICase(t *testing.T) {
	tests := []struct {
		name          string
		first, second string
		want          bool
	}{
		{"both empty are equal", "", "", true},
		{"ASCII case is ignored", "S-1-5-21-ABC", "s-1-5-21-abc", true},
		{"a different letter differs", "abc", "abd", false},
		{"a different length differs", "abc", "abcd", false},
		{"the Kelvin sign is not k", "K", "k", false},
		{"bytes after A to Z are not folded", "[", "{", false},
		{"invalid bytes compare as bytes", "A\xff", "a\xff", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EqualFoldASCII(test.first, test.second); got != test.want {
				t.Errorf("EqualFoldASCII(%q, %q) = %v, want %v", test.first, test.second, got, test.want)
			}
		})
	}
}

func TestEqualFoldASCIIAllocatesNothing(t *testing.T) {
	first, second := strings.Repeat("Mixed Case ", 4), strings.Repeat("mIXED cASE ", 4)
	allocations := testing.AllocsPerRun(100, func() { _ = EqualFoldASCII(first, second) })
	if allocations != 0 {
		t.Errorf("got %v allocations, want 0", allocations)
	}
}
