package pytext

import (
	"math"
	"testing"
)

// The tables of pytext_cases_test.go were printed by Python 3.14
// (analyzer/.venv) from the inputs beside them; regenerate them with the
// same interpreter.

func TestFloatMatchesPythonRepr(t *testing.T) {
	cases := floatReprs()
	for _, test := range cases {
		value := math.Float64frombits(test.bits)
		t.Run("repr of "+test.want+" is reproduced", func(t *testing.T) {
			if got := Float(value); got != test.want {
				t.Errorf("Float(%v) = %q, want %q", value, got, test.want)
			}
		})
	}
}

func TestFloatWritesSpecialValuesAsPythonDoes(t *testing.T) {
	cases := specialFloats()
	for _, test := range cases {
		t.Run(test.want+" is written by name", func(t *testing.T) {
			if got := Float(test.value); got != test.want {
				t.Errorf("Float(%v) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestBoolAndNoneUsePythonNames(t *testing.T) {
	if Bool(true) != "True" || Bool(false) != "False" || None != "None" {
		t.Errorf("got %q %q %q", Bool(true), Bool(false), None)
	}
}

func TestDecodeUTF8ReplacesEachMaximalInvalidSequence(t *testing.T) {
	cases := decodeCases()
	for _, test := range cases {
		t.Run("bytes "+printable(test.input)+" decode as Python decodes them", func(t *testing.T) {
			if got := DecodeUTF8([]byte(test.input)); got != test.want {
				t.Errorf("DecodeUTF8(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestStripsMatchPython(t *testing.T) {
	cases := stripCases()
	for _, test := range cases {
		t.Run("text "+printable(test.input)+" is trimmed as Python trims it", func(t *testing.T) {
			if got := Strip(test.input); got != test.strip {
				t.Errorf("Strip(%q) = %q, want %q", test.input, got, test.strip)
			}
			if got := StripLeft(test.input); got != test.left {
				t.Errorf("StripLeft(%q) = %q, want %q", test.input, got, test.left)
			}
			if got := StripRight(test.input); got != test.right {
				t.Errorf("StripRight(%q) = %q, want %q", test.input, got, test.right)
			}
			if got := StripASCII(test.input); got != test.ascii {
				t.Errorf("StripASCII(%q) = %q, want %q", test.input, got, test.ascii)
			}
		})
	}
}

func TestCodePointFunctionsCountAsPythonDoes(t *testing.T) {
	cases := codePointCases()
	for _, test := range cases {
		name := "text " + printable(test.input) + " is sliced, counted and padded by code point"
		t.Run(name, func(t *testing.T) {
			if got := SliceCodePoints(test.input, test.n); got != test.slice {
				t.Errorf("SliceCodePoints(%q, %d) = %q, want %q", test.input, test.n, got, test.slice)
			}
			if got := CodePointLen(test.input); got != test.length {
				t.Errorf("CodePointLen(%q) = %d, want %d", test.input, got, test.length)
			}
			if got := PadRight(test.input, 6); got != test.left {
				t.Errorf("PadRight(%q, 6) = %q, want %q", test.input, got, test.left)
			}
			if got := PadLeft(test.input, 6); got != test.right {
				t.Errorf("PadLeft(%q, 6) = %q, want %q", test.input, got, test.right)
			}
		})
	}
}

func TestNextCodePointReadsSurrogatesAndInvalidBytes(t *testing.T) {
	cases := nextCodePointCases()
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			char, width := NextCodePoint(test.input)
			if char != test.char || width != test.width {
				t.Errorf("NextCodePoint(%q) = %U, %d, want %U, %d",
					test.input, char, width, test.char, test.width)
			}
		})
	}
}

func TestAppendCodePointRoundTripsEverySurrogate(t *testing.T) {
	for char := rune(0xD800); char <= 0xDFFF; char++ {
		encoded := string(AppendCodePoint(nil, char))
		got, width := NextCodePoint(encoded)
		if got != char || width != 3 || !HasLoneSurrogate(encoded) {
			t.Fatalf("%U encodes as %q and reads back as %U, %d", char, encoded, got, width)
		}
	}
	if got := string(AppendCodePoint(nil, 0x20AC)); got != "€" {
		t.Errorf("AppendCodePoint(U+20AC) = %q", got)
	}
}

func TestHasLoneSurrogateFindsOnlySurrogateForms(t *testing.T) {
	cases := surrogateCases()
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := HasLoneSurrogate(test.input); got != test.want {
				t.Errorf("HasLoneSurrogate(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

// printable returns a test name fragment for text that may hold any byte.
func printable(text string) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		char := text[i]
		if char > ' ' && char < 0x7f {
			out = append(out, char)
		} else {
			out = append(out, 'x', hexDigits[char>>4], hexDigits[char&0xf])
		}
	}
	return "[" + string(out) + "]"
}
