package pyjson

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// The tables of oracle_cases_test.go were printed by Python 3.14
// (analyzer/.venv) through json.loads, json.dumps, JSONDecoder.raw_decode
// and str(); a lone surrogate appears in its three-byte form, as this
// package holds it.

func TestDecodedValuesEncodeAsPythonWritesThem(t *testing.T) {
	cases := validDocuments()
	for _, test := range cases {
		t.Run("document "+printable(test.input)+" round-trips as Python's", func(t *testing.T) {
			value, err := DecodeString(test.input)
			if err != nil {
				t.Fatalf("DecodeString(%q): %v", test.input, err)
			}
			checks := []struct{ name, got, want string }{
				{"Compact", Compact(value), test.compact},
				{"Sorted", Sorted(value), test.sorted},
				{"Spaced", Spaced(value), test.spaced},
				{"SpacedUnicode", SpacedUnicode(value), test.spacedUnicode},
				{"Indented", Indented(value), test.indented},
				{"Str", value.Str(), test.str},
			}
			for _, check := range checks {
				if check.got != check.want {
					t.Errorf("%s = %q, want %q", check.name, check.got, check.want)
				}
			}
		})
	}
}

func TestSortedSpacedUnicodeMatchesPython(t *testing.T) {
	cases := sortedSpacedDocuments()
	format := Format{SortKeys: true, Spaced: true}
	for _, test := range cases {
		name := "document " + printable(test.input) + " is written with sorted keys and spaces"
		t.Run(name, func(t *testing.T) {
			value, err := DecodeString(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got := format.Encode(value); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestDecodeRejectsWhatPythonRejects(t *testing.T) {
	cases := invalidDocuments()
	for _, input := range cases {
		t.Run("document "+printable(input)+" is an error", func(t *testing.T) {
			value, err := DecodeString(input)
			if err == nil {
				t.Fatalf("DecodeString(%q) = %s, want an error", input, Compact(value))
			}
			if _, ok := err.(*SyntaxError); !ok {
				t.Errorf("error %v is a %T, want *SyntaxError", err, err)
			}
		})
	}
}

func TestDecodePrefixStopsWherePythonStops(t *testing.T) {
	cases := prefixDocuments()
	for _, test := range cases {
		t.Run("text "+printable(test.text)+" is read from its offset", func(t *testing.T) {
			value, end, err := DecodePrefix(test.text, test.start)
			if test.end < 0 {
				if err == nil {
					t.Fatalf("got %s ending at %d, want an error", Compact(value), end)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if end != test.end || pytext.CodePointLen(test.text[:end]) != test.endChars {
				t.Errorf("end = %d, want %d (%d code points)", end, test.end, test.endChars)
			}
			if got := Compact(value); got != test.compact {
				t.Errorf("value = %q, want %q", got, test.compact)
			}
		})
	}
}

func TestDecodeReplacesInvalidUTF8First(t *testing.T) {
	cases := rawDocuments()
	for _, test := range cases {
		t.Run("bytes "+printable(test.input)+" decode as Python decodes them", func(t *testing.T) {
			value, err := Decode([]byte(test.input))
			if test.compact == "" {
				if err == nil {
					t.Fatalf("got %q, want an error", Compact(value))
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := Compact(value); got != test.compact {
				t.Errorf("got %q, want %q", got, test.compact)
			}
		})
	}
}

// printable returns a test name fragment for text that may hold any byte.
func printable(text string) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text) && i < 60; i++ {
		char := text[i]
		if char > ' ' && char < 0x7f {
			out = append(out, char)
		} else {
			out = append(out, 'x', hexDigits[char>>4], hexDigits[char&0xf])
		}
	}
	return "[" + string(out) + "]"
}
