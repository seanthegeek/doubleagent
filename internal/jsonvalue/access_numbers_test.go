package jsonvalue

import (
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"strings"
	"testing"
)

func TestEqualComparesEverySpellingOfANumberByValue(t *testing.T) {
	huge := strings.Repeat("9", 30)
	tests := []struct {
		name        string
		left, right string
		equal       bool
	}{
		{"zero and minus zero", `0`, `-0`, true},
		{"zero and a zero fraction with an exponent", `0`, `-0.000e5`, true},
		{"an integer and its exponent form", `100`, `1e2`, true},
		{"an exponent form with a plus sign", `1e+2`, `100.0`, true},
		{"an integer and a zero fraction", `1`, `1.0`, true},
		{"a fraction and a negative exponent", `0.1`, `1e-1`, true},
		{"trailing zeros of a fraction", `1.50`, `1.5`, true},
		{"a capital E", `1E999`, `1e999`, true},
		{"two spellings beyond the range of a float", `1e999`, `10e998`, true},
		{"two numbers beyond the range of a float", `1e999`, `1e998`, false},
		{"two tiny numbers", `1e-999`, `0.1e-998`, true},
		{"a tiny number and zero", `1e-999`, `0`, false},
		{"an integer beyond 64 bits and its exponent form", huge, `9.` + strings.Repeat("9", 29) + `e29`, true},
		{"integers beyond 64 bits that differ in the last digit", huge, huge[:29] + "8", false},
		{"two numbers one float step apart", `0.1`, `0.10000000000000001`, false},
		{"opposite signs", `-1`, `1`, false},
		{"exponents of many digits that differ by the shift",
			`1e100000000000000000000`, `10e99999999999999999999`, true},
		{"negative exponents of many digits", `1e-100000000000000000000`, `0.1e-99999999999999999999`, true},
		{"exponents of many digits that differ", `1e100000000000000000000`, `1e99999999999999999999`, false},
		{"a borrow through an exponent of many digits",
			`1e999999999999999999999`, `0.0001e1000000000000000000003`, true},
		{"a carry through an exponent of many digits",
			`1000e999999999999999999997`, `1e1000000000000000000000`, true},
		{"exponents with leading zeros", `1e0002`, `100`, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left, leftErr := DecodeString(test.left)
			right, rightErr := DecodeString(test.right)
			if leftErr != nil || rightErr != nil {
				t.Fatalf("decoding: %v, %v", leftErr, rightErr)
			}
			if got := Equal(left, right); got != test.equal {
				t.Errorf("Equal(%s, %s) = %v, want %v", test.left, test.right, got, test.equal)
			}
			if got := Equal(right, left); got != test.equal {
				t.Errorf("Equal(%s, %s) = %v, want %v", test.right, test.left, got, test.equal)
			}
		})
	}
}

func TestEqualReadsALongNumberWithoutWorkThatGrowsWithIt(t *testing.T) {
	for _, digits := range []int{1_000, 100_000} {
		long, err := DecodeString(strings.Repeat("9", digits))
		if err != nil {
			t.Fatal(err)
		}
		allocations := testing.AllocsPerRun(3, func() { Equal(long, NewInt(1)) })
		if allocations > 8 {
			t.Errorf("comparing a %d-digit integer made %v allocations", digits, allocations)
		}
	}
}

func TestFloatRefusesANumberBeyondTheRangeOfAFloat(t *testing.T) {
	for _, text := range []string{`1e999`, `-1e999`} {
		value, _ := DecodeString(text)
		if got, ok := value.Float(); got != 0 || ok {
			t.Errorf("%s read as %v, %v; want 0, false", text, got, ok)
		}
	}
}

// nested returns an array nested levels deep around one number.
func nested(levels int) string {
	return strings.Repeat("[", levels) + "1" + strings.Repeat("]", levels)
}

func TestJSONWritesAValueOfAnyDepth(t *testing.T) {
	deep, err := DecodeString(nested(10_000))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		value Value
		want  string
	}{
		{"a value at the decoder's depth", deep, nested(10_000)},
		{"that value inside an array", NewArray(deep), nested(10_001)},
		{"that value inside an object", NewObject(Member{"k", deep}), `{"k":` + nested(10_000) + `}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.value.JSON(); got != test.want {
				t.Fatalf("JSON gave %d bytes, want %d", len(got), len(test.want))
			}
			if got := test.value.Text(); got != test.want {
				t.Fatalf("Text gave %d bytes, want %d", len(got), len(test.want))
			}
		})
	}
}

// numberOf returns text decoded as a JSON number, and false when text is
// not exactly one number with nothing around it.
func numberOf(text string) (Value, bool) {
	value, err := DecodeString(text)
	return value, err == nil && value.IsNumber() && value.Text() == text
}

// smallExponent reports whether a number's exponent, if any, has at most
// three digits.
func smallExponent(text string) bool {
	position := strings.IndexAny(text, "eE")
	return position < 0 || len(strings.TrimLeft(text[position+1:], "+-")) <= 3
}

// FuzzNumbersEqual checks Equal on two numbers against exact rational
// arithmetic, for numbers whose exponents are small enough for it.
func FuzzNumbersEqual(fuzz *testing.F) {
	for _, pair := range [][2]string{{"0", "-0"}, {"1e2", "100"}, {"1.50", "15e-1"},
		{"0.1", "0.10000000000000001"}, {"1e999", "10e998"}, {"-0.000e5", "0"}} {
		fuzz.Add(pair[0], pair[1])
	}
	fuzz.Fuzz(func(t *testing.T, leftText, rightText string) {
		left, leftOK := numberOf(leftText)
		right, rightOK := numberOf(rightText)
		if !leftOK || !rightOK || !smallExponent(leftText) || !smallExponent(rightText) {
			return
		}
		var leftRational, rightRational big.Rat
		if _, ok := leftRational.SetString(leftText); !ok {
			t.Fatalf("big.Rat refused %q", leftText)
		}
		if _, ok := rightRational.SetString(rightText); !ok {
			t.Fatalf("big.Rat refused %q", rightText)
		}
		if got, want := Equal(left, right), leftRational.Cmp(&rightRational) == 0; got != want {
			t.Fatalf("Equal(%s, %s) = %v, want %v", leftText, rightText, got, want)
		}
	})
}

func TestCanonicalNumberSpellsEachValueOneWay(t *testing.T) {
	tests := []struct {
		name, number, want string
	}{
		{"a zero", `-0.000e5`, "0"},
		{"an integer with trailing zeros", `100`, "1e2"},
		{"a fraction with a trailing zero", `1.50`, "15e-1"},
		{"a negative number with a plus exponent", `-12E+03`, "-12e3"},
		{"a leading zero of a fraction", `0.001`, "1e-3"},
		{"an exponent of many digits with a borrow", `0.0001e1000000000000000000003`, "1e999999999999999999999"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanonicalNumber(test.number); got != test.want {
				t.Errorf("CanonicalNumber(%s) = %s, want %s", test.number, got, test.want)
			}
		})
	}
}

// nestedObjects returns objects nested levels deep around one number,
// each the value of the key "a".
func nestedObjects(levels int) string {
	return strings.Repeat(`{"a":`, levels) + "1" + strings.Repeat("}", levels)
}

// tokenReaderRefusesDepth reports whether encoding/json's token reader
// refuses an array nested one past MaxDepth by itself, as it does in the
// default build; built with GOEXPERIMENT=nojsonv2 it has no limit, and the
// package's own count of levels is what refuses.
func tokenReaderRefusesDepth() bool {
	decoder := json.NewDecoder(strings.NewReader(nested(MaxDepth + 1)))
	for {
		if _, err := decoder.Token(); err != nil {
			return !errors.Is(err, io.EOF)
		}
	}
}

// errorText returns err's text, or "" for no error.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// The error text and place are checked in every build. Only in the build
// where the token reader has no limit (GOEXPERIMENT=nojsonv2, which CI runs
// for this test alone) is the refusal the package's own, errTooDeep; in the
// default build the token reader refuses first with the same words.
func TestArraysAndObjectsNestAtMostTenThousandDeep(t *testing.T) {
	ownCheckFires := !tokenReaderRefusesDepth()
	tests := []struct {
		name, text, want, prefixWant string
	}{
		{"arrays at the limit decode", nested(10_000), "", ""},
		{"objects at the limit decode", nestedObjects(10_000), "", ""},
		{"arrays one past the limit are refused", nested(10_001),
			"invalid JSON at line 1, column 10001: exceeded max depth",
			"invalid JSON at line 1, column 10002: exceeded max depth"},
		{"objects one past the limit are refused", nestedObjects(10_001),
			"invalid JSON at line 1, column 50001: exceeded max depth",
			"invalid JSON at line 1, column 50002: exceeded max depth"},
		{"an object holding arrays to one past the limit is refused", `{"a":` + nested(10_000) + `}`,
			"invalid JSON at line 1, column 10005: exceeded max depth",
			"invalid JSON at line 1, column 10006: exceeded max depth"},
		{"the place counts lines", "\n " + nested(10_001),
			"invalid JSON at line 2, column 10002: exceeded max depth",
			"invalid JSON at line 2, column 10002: exceeded max depth"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeString(test.text)
			if errorText(err) != test.want {
				t.Errorf("DecodeString gave %q, want %q", errorText(err), test.want)
			}
			_, _, prefixErr := NewDocument("x" + test.text).DecodePrefix(1)
			if errorText(prefixErr) != test.prefixWant {
				t.Errorf("DecodePrefix gave %q, want %q", errorText(prefixErr), test.prefixWant)
			}
			if refused := test.want != ""; ownCheckFires && refused &&
				(!errors.Is(err, errTooDeep) || !errors.Is(prefixErr, errTooDeep)) {
				t.Errorf("the package's own depth check did not refuse: %v, %v", err, prefixErr)
			}
		})
	}
}
