package jsonvalue

import (
	"bytes"
	"encoding/json"
	"testing"
)

// FuzzDecode checks that any input decodes to a value or an error without a
// panic, that a decoded value re-encodes to JSON that decodes to an equal
// value with the same encoding, and that its indented form is laid out as
// encoding/json's Indent lays out the compact form.
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
