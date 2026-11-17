package pyjson

import (
	"testing"
)

// FuzzDecode checks that any input decodes to a value or an error without a
// panic, and that a decoded value survives a Compact round trip unchanged.
func FuzzDecode(fuzz *testing.F) {
	seeds := []string{
		`{"a":[1,2.5,-0,1e400,"xé😀\ud83d"],"a":null}`, `[1,]`, `01`, `"\uzzzz"`,
		`NaN`, `-Infinity`, `{"k":{"k":{"k":[[[]]]}}}`, "\"\xff\xed\xa0\x80\"", `[1e-400, 1.50E+3]`,
		`"\"\\\/\b\f\n\r\t"`, "  \t\n[ ] ", `{"b":1,"a":2}`, `123456789012345678901234567890`,
	}
	for _, seed := range seeds {
		fuzz.Add([]byte(seed))
	}
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		if _, end, err := DecodePrefix(string(data), 0); err == nil && (end <= 0 || end > len(data)) {
			t.Fatalf("DecodePrefix ended at %d of %d bytes", end, len(data))
		}
		value, err := Decode(data)
		if err != nil {
			return
		}
		compact := Compact(value)
		again, err := DecodeString(compact)
		if err != nil {
			t.Fatalf("Compact output %q does not decode: %v", compact, err)
		}
		if Compact(again) != compact {
			t.Fatalf("round trip changed %q to %q", compact, Compact(again))
		}
		for _, text := range []string{Spaced(value), Indented(value), Sorted(value)} {
			if _, err := DecodeString(text); err != nil {
				t.Fatalf("encoder output %q does not decode: %v", text, err)
			}
		}
		_ = value.Repr()
	})
}
