package jsonvalue

import (
	"bytes"
	"encoding/json"
	"testing"
)

// FuzzDecode checks that any input decodes to a value or an error without a
// panic, that a decoded value re-encodes to JSON that decodes to an equal
// value with the same encoding, that its indented form is laid out as
// encoding/json's Indent lays out the compact form, and that a Document's
// note of repeated names keeps at most RepeatedNamesKept of them.
func FuzzDecode(fuzz *testing.F) {
	seeds := []string{
		`{"a":[1,2.5,-0,1e400,"xé😀\ud83d"],"a":null}`, `[1,]`, `01`, `"\uzzzz"`,
		`NaN`, `-Infinity`, `{"k":{"k":{"k":[[[]]]}}}`, "\"\xff\xed\xa0\x80\"", `[1e-400, 1.50E+3]`,
		`"\"\\\/\b\f\n\r\t"`, "  \t\n[ ] ", `{"b":1,"a":2}`, `123456789012345678901234567890`,
		`{"a":1}{"b":2}`, "\ufeff[]",
	}
	for _, seed := range seeds {
		fuzz.Add([]byte(seed))
	}
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		if _, end, err := DecodePrefix(string(data), 0); err == nil && (end <= 0 || end > len(data)) {
			t.Fatalf("DecodePrefix ended at %d of %d bytes", end, len(data))
		}
		value, err := Decode(data)
		document := NewDocument(string(data))
		if _, noted := document.Decode(); (noted == nil) != (err == nil) {
			t.Fatalf("a Document gave %v where Decode gave %v", noted, err)
		}
		if repeats := document.TakeRepeats(); len(repeats.Names()) > RepeatedNamesKept ||
			sumTimes(repeats.Names()) > repeats.Count() || (err != nil && repeats.Count() != 0) {
			t.Fatalf("repeats %d of %v", repeats.Count(), repeats.Names())
		}
		if err != nil {
			return
		}
		compact := value.JSON()
		again, err := DecodeString(compact)
		if err != nil {
			t.Fatalf("JSON output %q does not decode: %v", compact, err)
		}
		if !Equal(value, again) || again.JSON() != compact {
			t.Fatalf("round trip changed %q to %q", compact, again.JSON())
		}
		var indented bytes.Buffer
		if err := json.Indent(&indented, []byte(compact), "", "  "); err != nil {
			t.Fatalf("encoding/json cannot indent %q: %v", compact, err)
		}
		if got := value.IndentedJSON("  "); got != indented.String() {
			t.Fatalf("IndentedJSON gave %q, encoding/json %q", got, indented.String())
		}
		_ = value.Text()
		_ = value.IsEmpty()
	})
}

// FuzzIsEmptyNumber checks that a number is empty exactly when it equals
// zero by value.
func FuzzIsEmptyNumber(fuzz *testing.F) {
	for _, seed := range []string{"0", "-0", "0.0e5", "1e-400", "2e-324", "1", "-0.00",
		"504.90372E-0500367971000030950", "0e-99999999999999999999"} {
		fuzz.Add(seed)
	}
	zero, _ := numberOf("0")
	fuzz.Fuzz(func(t *testing.T, text string) {
		number, ok := numberOf(text)
		if !ok {
			return
		}
		if number.IsEmpty() != Equal(number, zero) {
			t.Fatalf("%s: IsEmpty %v, Equal to 0 %v", text, number.IsEmpty(), Equal(number, zero))
		}
	})
}
